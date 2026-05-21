package main

import (
	"akhokhlow80/tanlnode/peerstats"
	"akhokhlow80/tanlnode/sqlgen"
	"akhokhlow80/tanlnode/tx"
	"akhokhlow80/tanlnode/wg"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/netip"
	"sync"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type peerStatCache struct {
	peerID    int64
	totalCalc peerstats.TotalStatCalc
}

type peerStatsCache struct {
	sync.RWMutex
	m map[wgtypes.Key]peerStatCache
}

func newPeerStatsCache() *peerStatsCache {
	return &peerStatsCache{
		m: make(map[wgtypes.Key]peerStatCache),
	}
}

func (cache *peerStatsCache) PutNew(publicKey wgtypes.Key, dbID int64) {
	defer cache.Unlock()
	cache.Lock()
	cache.m[publicKey] = peerStatCache{
		peerID:    dbID,
		totalCalc: peerstats.MakeTotalStatsCalc(0, 0),
	}
}

func (cache *peerStatsCache) Remove(publicKey wgtypes.Key) {
	defer cache.Unlock()
	cache.Lock()
	delete(cache.m, publicKey)
}

func (node *node) prepareWGAndPeerStatsCache(ctx context.Context) error {
	// Parse peers from db
	type parsedPeer struct {
		id      int64
		wgCfg   wg.PeerConfig
		totalRx int64
		totalTx int64
	}
	peers, err := func() ([]parsedPeer, error) {
		defer node.db.RUnlock()
		node.db.RLock()

		dbPeers, err := node.db.GetPeers(ctx, nil)
		if err != nil {
			return nil, err
		}
		peers := make([]parsedPeer, 0, len(dbPeers))
		for _, dbPeer := range dbPeers {
			// parse peer
			publicKey, err := wgtypes.ParseKey(dbPeer.PublicKeyBase64)
			if err != nil {
				return nil, fmt.Errorf("Invalid peer public key %s: %s", dbPeer.PublicKeyBase64, err)
			}
			var psk *wgtypes.Key
			if len(dbPeer.PresharedKeyBase64) != 0 {
				psk = new(wgtypes.Key)
				*psk, err = wgtypes.ParseKey(dbPeer.PresharedKeyBase64)
				if err != nil {
					return nil, fmt.Errorf(
						"Invalid peer preshared key %s (pubkey=%s): %s",
						dbPeer.PresharedKeyBase64,
						dbPeer.PublicKeyBase64,
						err,
					)
				}
			}

			// load and parse subnets
			dbSubnets, err := node.db.GetPeerSubnets(ctx, &dbPeer.ID)
			if err != nil {
				return nil, fmt.Errorf("Failed to load peer %s subnets: %s", dbPeer.PublicKeyBase64, err)
			}
			allowedIPs := make([]netip.Prefix, 0, len(dbSubnets))
			for _, dbSubnet := range dbSubnets {
				subnet, err := netip.ParsePrefix(dbSubnet.Prefix)
				if err != nil {
					return nil, fmt.Errorf("Invalid subnet %s (id=%d): %s", dbSubnet.Prefix, dbSubnet.ID, err)
				}
				allowedIPs = append(allowedIPs, subnet)
			}

			// load and parse stats
			var tx, rx int64
			dbStats, err := node.db.GetLastPeerStat(ctx, sqlgen.GetLastPeerStatParams{PeerID: dbPeer.ID})
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					tx = 0
					rx = 0
				} else {
					return nil, fmt.Errorf("Failed to load peer %s stat: %s", dbPeer.PublicKeyBase64, err)
				}
			} else {
				tx = dbStats.Tx
				rx = dbStats.Rx
			}

			peers = append(peers, parsedPeer{
				id: dbPeer.ID,
				wgCfg: wg.PeerConfig{
					PublicKey:           publicKey,
					PresharedKey:        psk,
					Endpoint:            dbPeer.Endpoint,
					PersistentKeepalive: dbPeer.PersistentKeepalive,
					AllowedIPs:          allowedIPs,
				},
				totalTx: tx,
				totalRx: rx,
			})
		}
		return peers, nil
	}()
	if err != nil {
		return fmt.Errorf("Failed to parse peers from db: %s", err)
	}

	// Prepare WG and cache
	err, errAt := func() (err error, errAt int) {
		defer node.peerStatsCache.Unlock()
		node.peerStatsCache.Lock()
		for i, peer := range peers {
			// Reset peer stat by removing from then adding to wg
			if err := node.wg.RemovePeer(peer.wgCfg.PublicKey); err != nil {
				return fmt.Errorf("Error removing peer from wg: %s", err), i
			}
			if err := node.wg.PutPeer(&peer.wgCfg); err != nil {
				return fmt.Errorf("Error adding peer to wg: %s", err), i
			}
			node.peerStatsCache.m[peer.wgCfg.PublicKey] = peerStatCache{
				peerID:    peer.id,
				totalCalc: peerstats.MakeTotalStatsCalc(peer.totalTx, peer.totalRx),
			}
		}
		return nil, 0
	}()
	if err != nil {
		for i := range errAt + 1 {
			node.wg.RemovePeer(peers[i].wgCfg.PublicKey)
			delete(node.peerStatsCache.m, peers[i].wgCfg.PublicKey)
		}
		return fmt.Errorf("Failed to prepare peer %s: %s, rolled back", peers[errAt].wgCfg.PublicKey, err)
	}

	return nil
}

func (node *node) collectStats(ctx context.Context) error {
	wgStats, err := node.wg.GetPeerStats()
	if err != nil {
		return err
	}

	now := time.Now()

	node.peerStatsCache.Lock()
	defer node.peerStatsCache.Unlock()
	node.db.Lock()
	defer node.db.Unlock()
	for _, wgStat := range wgStats {
		cachedStat, present := node.peerStatsCache.m[wgStat.PublicKey]
		if !present {
			continue
		}

		total, newPeriod := cachedStat.totalCalc.Calculate(wgStat.Stat)
		if newPeriod {
			log.Printf(
				"Peer %s was recreated without tanlnode being restarted; stats may be incomplete",
				wgStat.PublicKey.String(),
			)
		}

		node.db.PutPeerStat(ctx, sqlgen.PutPeerStatParams{
			TimestampMs: now.UnixMilli(),
			PeerID:      cachedStat.peerID,
			Tx:          total.Tx,
			Rx:          total.Rx,
		})
		node.db.UpdatePeerHandshakeData(ctx, sqlgen.UpdatePeerHandshakeDataParams{
			LatestHandshakeAt: &total.LatestHandshake,
			LatestEndpoint:    &total.LatestEndpoint,
			ID:                cachedStat.peerID,
		})
	}
	return nil
}

func (node *node) collectStatsRoutine(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(node.cfg.StatCollectIntervalSecs) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		err := node.collectStats(ctx)
		if err != nil {
			log.Printf("Failed to collect stats: %s", err)
		}
	}
}

func (node *node) rollupOldStats(ctx context.Context) error {
	defer node.db.Unlock()
	node.db.Lock()

	peers, err := node.db.GetPeers(ctx, nil)
	if err != nil {
		return err
	}

	oldest := time.Now().Add(-time.Duration(node.cfg.StatRollupAfterSecs) * time.Second)

	dbTx, err := node.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	return tx.Transactional{
		Commit: func(ctx context.Context) error {
			return dbTx.Commit()
		},
		Rollback: func(ctx context.Context) error {
			return dbTx.Rollback()
		},
		Action: func(ctx context.Context) error {
			stats := make([]sqlgen.PeerStat, 0, len(peers))
			for _, peer := range peers {
				ms := oldest.UnixMilli()
				stat, err := node.db.WithTx(dbTx).GetLastPeerStat(ctx, sqlgen.GetLastPeerStatParams{
					PeerID:  peer.ID,
					UntilMs: &ms,
				})
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						stat.PeerID = peer.ID
						stat.Tx = 0
						stat.Rx = 0
					} else {
						return err
					}
				}
				stats = append(stats, stat)
			}

			if err := node.db.WithTx(dbTx).RemoveOldStats(ctx, oldest.UnixMilli()); err != nil {
				return err
			}

			for _, stat := range stats {
				_, err := node.db.WithTx(dbTx).PutPeerStat(ctx, sqlgen.PutPeerStatParams{
					TimestampMs: oldest.UnixMilli(),
					PeerID:      stat.PeerID,
					Rx:          stat.Rx,
					Tx:          stat.Tx,
				})
				if err != nil {
					return err
				}
			}

			return nil
		},
	}.Do(ctx)
}

func (node *node) rollupOldStatsRoutine(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(node.cfg.StatRollupIntervalSecs) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		err := node.rollupOldStats(ctx)
		if err != nil {
			log.Printf("Failed to rollup stats: %s", err)
		}
	}
}
