package main

import (
	"akhokhlow80/tanlnode/db"
	"akhokhlow80/tanlnode/nettree"
	"akhokhlow80/tanlnode/sqlgen"
	"akhokhlow80/tanlnode/tx"
	"akhokhlow80/tanlnode/wg"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/netip"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func (node *node) registerPeerHandlers(mux *http.ServeMux) {
	mux.HandleFunc("POST /peers", node.apiAddPeer)
	mux.HandleFunc("GET /peers", node.listPeers)
	mux.HandleFunc("DELETE /peers/{pubkey}", node.apiDeletePeer)
	mux.HandleFunc("GET /peers/{pubkey}", node.apiGetPeerByPubkey)
	mux.HandleFunc("PUT /peers/{pubkey}", node.apiUpdatePeer)
}

type PeerResponse struct {
	PublicKeyBase64     string  `json:"public_key_base64"`
	IsEnabled           bool    `json:"is_enabled"`
	PresharedKeyBase64  *string `json:"preshared_key_base64"`
	Endpoint            *string `json:"endpoint"`
	PersistentKeepalive *int64  `json:"persistent_keepalive"`
	Owner               *string `json:"owner"`
}

func (resp *PeerResponse) fromDB(p *sqlgen.Peer) {
	resp.PublicKeyBase64 = p.PublicKeyBase64
	resp.IsEnabled = p.IsEnabled
	resp.PresharedKeyBase64 = p.PresharedKeyBase64
	resp.Endpoint = p.Endpoint
	resp.PersistentKeepalive = p.PersistentKeepalive
	resp.Owner = p.Owner
}

type AddPeerRequest struct {
	// omit to generate new random private key (not saved on the node)
	PublicKeyBase64    *string `json:"public_key_base64"`
	PresharedKeyBase64 *string `json:"preshared_key_base64"`
	// If set, then a new random preshared key will be generated.
	// This flag beign set is mutually exclusive with `preshared_key_base64` beign non-null.
	RandomPresharedKey  bool    `json:"random_preshared_key"`
	Endpoint            *string `json:"endpoint"`
	PersistentKeepalive *int64  `json:"persistent_keepalive"`
	Owner               *string `json:"owner"`
	// List of CIDR subnets that are going to be assigned to the created peer.
	// If null, then one address (/32 for v4, /128 v6) per each configured node's net
	// will be assigned randomly.
	AllowedAddresses *[]struct {
		Subnet string
		// Subnet is not treated as exclusive for one peer, so it may overlap with other subnets,
		// and it is not taken into account by random address allocation.
		MayOverlap bool
	} `json:"allowed_addresses"`
}

type WGQuickConfig struct {
	Interface struct {
		// is set to a newly generated one only if no public key was provided in the request
		PrivateKey *string `json:"private_key"`
		// CIDRs
		Addresses []string `json:"addresses"`
		// based on configuration
		DNS *string `json:"dns"`
		// based on configuration
		MTU *int `json:"mtu"`
	} `json:"interface"`
	NodePeer struct {
		PublicKey string `json:"public_key"`
		// set only if preshared key was given, or random preshared key generation was requested
		PresharedKey *string `json:"preshared_key"`
		Endpoint     string  `json:"endpoint"`
		// based on configuration
		PersistentKeepalive *int `json:"persistent_keepalive"`
	} `json:"node_peer"`
}

type NewPeerResponse struct {
	Peer   PeerResponse  `json:"peer"`
	Config WGQuickConfig `json:"config"`
}

// @Summary		Add a new peer
// @Description	Creates a new peer
// @Tags			peers
// @Accept			json
// @Produce		json
// @Param			peer	body		AddPeerRequest	true	"peer"
// @Success		201		{object}	NewPeerResponse
// @Failure		400		{object}	APIError
// @Router			/api/v1/peers [post]
func (node *node) apiAddPeer(w http.ResponseWriter, r *http.Request) {
	var req AddPeerRequest

	// Parse

	// TODO: parse endpoint

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	type ParsedSubnet struct {
		Prefix     netip.Prefix
		MayOverlap bool
	}
	var parsedSubnets *[]ParsedSubnet
	if req.AllowedAddresses != nil {
		parsedSubnetsSlice := make([]ParsedSubnet, 0, len(*req.AllowedAddresses))
		for _, subnet := range *req.AllowedAddresses {
			prefix, err := netip.ParsePrefix(subnet.Subnet)
			if err != nil {
				respondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid subnet %s", subnet.Subnet))
				return
			}
			parsedSubnetsSlice = append(parsedSubnetsSlice, ParsedSubnet{
				Prefix:     prefix,
				MayOverlap: subnet.MayOverlap,
			})
		}
		parsedSubnets = &parsedSubnetsSlice
	}

	// Generate keys if needed

	var (
		err                      error
		privateKey, presharedKey *wgtypes.Key
		publicKey                wgtypes.Key
	)

	if req.PublicKeyBase64 == nil {
		newPrivateKey, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			internalServerError(w, r, err)
			return
		}
		privateKey = &newPrivateKey
		publicKey = privateKey.PublicKey()
	} else {
		publicKey, err = wgtypes.ParseKey(*req.PublicKeyBase64)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid public key")
			return
		}
	}

	if req.PresharedKeyBase64 != nil {
		parsedPresharedKey, err := wgtypes.ParseKey(*req.PresharedKeyBase64)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid preshared key")
			return
		}
		presharedKey = &parsedPresharedKey
	} else {
		if req.RandomPresharedKey {
			randomPresharedKey, err := wgtypes.GeneratePrivateKey()
			if err != nil {
				internalServerError(w, r, err)
				return
			}
			presharedKey = &randomPresharedKey
		}
	}

	// Add to DB and nettrees

	var dbTx *sql.Tx
	var dbPeer sqlgen.Peer
	err = tx.Transactional{
		Commit: func(ctx context.Context) error {
			if dbTx != nil {
				return dbTx.Commit()
			} else {
				return nil
			}
		},
		Rollback: func(ctx context.Context) error {
			var err error
			if dbTx != nil {
				err = dbTx.Rollback()
			}

			// Remove subnets that possibly were added to netrees
			if parsedSubnets != nil {
				for _, subnet := range *parsedSubnets {
					node.subnets.Delete(subnet.Prefix)
				}
			}
			return err
		},
		Action: func(ctx context.Context) error {
			node.db.Lock()
			defer node.db.Unlock()

			dbTx, err = node.db.BeginTx(r.Context(), nil)
			if err != nil {
				return err
			}

			// Add peer to DB

			var addPeer sqlgen.AddPeerParams
			addPeer.PublicKeyBase64 = publicKey.String()
			addPeer.IsEnabled = true
			if presharedKey != nil {
				addPeer.PresharedKeyBase64 = new(string)
				*addPeer.PresharedKeyBase64 = presharedKey.String()
			}
			addPeer.Endpoint = req.Endpoint
			addPeer.PersistentKeepalive = req.PersistentKeepalive
			addPeer.Owner = req.Owner
			dbPeer, err = node.db.AddPeer(r.Context(), addPeer)
			if err != nil {
				return err
			}

			// Reserve or allocate subnets in nettree

			if parsedSubnets != nil {
				for _, subnet := range *parsedSubnets {
					if subnet.MayOverlap {
						continue
					}
					_, err := node.subnets.Reserve(subnet.Prefix)
					if err != nil {
						return err
					}
				}
			} else {
				randomAddrs, err := node.subnets.AssignRandomInEachNet()
				if err != nil {
					return err
				}
				parsedSubnetsSlice := make([]ParsedSubnet, 0, len(randomAddrs))
				parsedSubnets = &parsedSubnetsSlice
				for _, randomAddr := range randomAddrs {
					var maskBits int
					if randomAddr.Is4() {
						maskBits = 32
					} else if randomAddr.Is6() {
						maskBits = 128
					} else {
						panic("never")
					}
					parsedSubnetsSlice = append(parsedSubnetsSlice, ParsedSubnet{
						Prefix:     netip.PrefixFrom(randomAddr, maskBits),
						MayOverlap: false,
					})
				}
			}

			// Put addresses to DB

			for _, pSubnet := range *parsedSubnets {
				_, err := node.db.AddSubnet(ctx, sqlgen.AddSubnetParams{
					Prefix: fmt.Sprintf(
						"%s/%d",
						pSubnet.Prefix.Addr().StringExpanded(),
						pSubnet.Prefix.Bits(),
					),
					PeerID:     &dbPeer.ID,
					Comment:    "",
					MayOverlap: pSubnet.MayOverlap,
				})
				if err != nil {
					return err
				}
			}

			// Add peer to wg
			// TODO: endpoint should be parsed to avoid 500 error on malformed endpoint
			// TODO: also providing domain name as endpoint causes wg to perform DNS lookup

			wgAllowedIPs := make([]netip.Prefix, 0, len(*parsedSubnets))
			for _, subnet := range *parsedSubnets {
				wgAllowedIPs = append(wgAllowedIPs, subnet.Prefix)
			}
			err = node.wg.PutPeer(&wg.Peer{
				PublicKey:           publicKey,
				PresharedKey:        presharedKey,
				Endpoint:            req.Endpoint,
				PersistentKeepalive: req.PersistentKeepalive,
				AllowedIPs:          wgAllowedIPs,
			})
			if err != nil {
				return err
			}

			return nil
		},
	}.Do(r.Context())
	if err != nil {
		if db.IsConstraintErr(err) {
			respondError(w, http.StatusConflict, "Peer with such public key already exists")
			return
		} else if errors.Is(err, nettree.ErrNoFreeAddr) {
			respondError(w, http.StatusBadRequest, "No free addresses left for random allocation")
			return
		} else {
			internalServerError(w, r, err)
			return
		}
	}

	var resp NewPeerResponse
	resp.Peer.fromDB(&dbPeer)

	// [Interface]
	if privateKey != nil {
		resp.Config.Interface.PrivateKey = new(string)
		*resp.Config.Interface.PrivateKey = privateKey.String()
	}
	resp.Config.Interface.Addresses = make([]string, 0, len(*parsedSubnets))
	for _, subnet := range *parsedSubnets {
		resp.Config.Interface.Addresses = append(resp.Config.Interface.Addresses, subnet.Prefix.String())
	}
	if len(node.cfg.WGDNS) != 0 {
		resp.Config.Interface.DNS = &node.cfg.WGDNS
	}
	if node.cfg.WGMTU != 0 {
		resp.Config.Interface.MTU = &node.cfg.WGMTU
	}

	// [Peer]
	resp.Config.NodePeer.PublicKey = node.cfg.WGPublicKey
	if presharedKey != nil {
		resp.Config.NodePeer.PresharedKey = new(string)
		*resp.Config.NodePeer.PresharedKey = presharedKey.String()
	}
	resp.Config.NodePeer.Endpoint = node.cfg.WGEndpoint
	if node.cfg.WGPersistentKeepalive != 0 {
		resp.Config.NodePeer.PersistentKeepalive = &node.cfg.WGPersistentKeepalive
	}

	respondJSON(w, http.StatusCreated, resp)
}

// @Summary		List peers
// @Description	Returns a list of peers, optionally filtered by owner
// @Tags			peers
// @Produce		json
// @Param			owner	query		string	false	"Filter by owner"
// @Success		200		{array}		PeerResponse
// @Failure		400		{object}	APIError
// @Router			/api/v1/peers [get]
func (node *node) listPeers(w http.ResponseWriter, r *http.Request) {
	var owner *string
	if r.URL.Query().Has("owner") {
		owner = new(string)
		*owner = r.URL.Query().Get("owner")
	}

	sqlPeers, err := func() ([]sqlgen.Peer, error) {
		defer node.db.RUnlock()
		node.db.RLock()
		return node.db.GetPeers(r.Context(), owner)
	}()
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	var peers []PeerResponse
	for _, sqlPeer := range sqlPeers {
		var peer PeerResponse
		peer.fromDB(&sqlPeer)
		peers = append(peers, peer)
	}

	respondJSON(w, http.StatusOK, peers)
}

// @Summary		Delete a peer
// @Description	Removes the peer identified by the given key
// @Tags			peers
// @Produce		json
// @Param			pubkey	path	string	true	"Peer key"
// @Success		204		"No content"
// @Failure		404		{object}	APIError
// @Router			/api/v1/peers/{pubkey} [delete]
func (node *node) apiDeletePeer(w http.ResponseWriter, r *http.Request) {
	publicKey, err := wgtypes.ParseKey(r.PathValue("pubkey"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid public key")
		return
	}

	var dbTx *sql.Tx
	err = tx.Transactional{
		Commit: func(ctx context.Context) error {
			if dbTx != nil {
				return dbTx.Commit()
			} else {
				return nil
			}
		},
		Rollback: func(ctx context.Context) error {
			if dbTx != nil {
				return dbTx.Rollback()
			} else {
				return nil
			}
		},
		Action: func(ctx context.Context) error {
			node.db.Lock()
			defer node.db.Unlock()

			rows, err := node.db.RemovePeer(ctx, publicKey.String())
			if err != nil {
				return err
			}
			if rows == 0 {
				return sql.ErrNoRows
			}

			if err := node.wg.RemovePeer(publicKey); err != nil {
				return err
			}

			return nil
		},
	}.Do(r.Context())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondJSON(w, http.StatusNotFound, "Peer not found")
			return
		} else {
			internalServerError(w, r, err)
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// @Summary		Get a peer by key
// @Description	Returns the peer identified by the given key
// @Tags			peers
// @Produce		json
// @Param			pubkey	path		string	true	"Peer key"
// @Success		200		{object}	PeerResponse
// @Failure		404		{object}	APIError
// @Router			/api/v1/peers/{pubkey} [get]
func (node *node) apiGetPeerByPubkey(w http.ResponseWriter, r *http.Request) {
	pubkey := r.PathValue("pubkey")

	dbPeer, err := func() (sqlgen.Peer, error) {
		node.db.Lock()
		defer node.db.Unlock()
		return node.db.GetPeerByPublicKey(r.Context(), pubkey)
	}()
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondJSON(w, http.StatusBadRequest, "Peer not found")
			return
		} else {
			internalServerError(w, r, err)
			return
		}
	}

	var peerResp PeerResponse
	peerResp.fromDB(&dbPeer)
	respondJSON(w, http.StatusOK, &peerResp)
}

type UpdatePeerRequest struct {
	IsEnabled           bool    `json:"is_enabled"`
	PresharedKeyBase64  *string `json:"preshared_key_base64"`
	Endpoint            *string `json:"endpoint"`
	PersistentKeepalive *int64  `json:"persistent_keepalive"`
	Owner               *string `json:"owner"`
}

// @Summary	Update a peer
// @Tags		peers
// @Accept		json
// @Produce	json
// @Param		pubkey	path	string				true	"Peer key"
// @Param		peer	body	UpdatePeerRequest	true	"Updated peer object"
// @Success	204		"No content"
// @Failure	400		{object}	APIError
// @Failure	404		{object}	APIError
// @Router		/api/v1/peers/{pubkey} [put]
func (node *node) apiUpdatePeer(w http.ResponseWriter, r *http.Request) {
	var err error

	publicKeyBase64 := r.PathValue("pubkey")
	publicKey, err := wgtypes.ParseKey(publicKeyBase64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid public key")
		return
	}

	var req UpdatePeerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid json")
		return
	}

	var presharedKey *wgtypes.Key
	if req.PresharedKeyBase64 != nil {
		presharedKey = new(wgtypes.Key)
		*presharedKey, err = wgtypes.ParseKey(*req.PresharedKeyBase64)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid preshared key")
			return
		}
	}

	var dbTx *sql.Tx
	err = tx.Transactional{
		Commit: func(ctx context.Context) error {
			if dbTx != nil {
				return dbTx.Commit()
			} else {
				return nil
			}
		},
		Rollback: func(ctx context.Context) error {
			if dbTx != nil {
				return dbTx.Rollback()
			} else {
				return nil
			}
		},
		Action: func(ctx context.Context) error {
			node.db.Lock()
			defer node.db.Unlock()

			// Update in DB

			var updatePeer sqlgen.UpdatePeerParams
			updatePeer.IsEnabled = req.IsEnabled
			if presharedKey != nil {
				updatePeer.PresharedKeyBase64 = new(string)
				*updatePeer.PresharedKeyBase64 = presharedKey.String()
			}
			updatePeer.Endpoint = req.Endpoint
			updatePeer.PersistentKeepalive = req.PersistentKeepalive
			updatePeer.Owner = req.Owner
			updatePeer.PublicKeyBase64 = publicKeyBase64
			dbPeer, err := node.db.UpdatePeer(r.Context(), updatePeer)
			if err != nil {
				return err
			}

			// Get last subnets

			subnets, err := node.db.GetPeerSubnets(r.Context(), &dbPeer.ID)
			if err != nil {
				return err
			}
			wgAllowedIPs := make([]netip.Prefix, 0, len(subnets))
			for _, subnet := range subnets {
				pref, err := netip.ParsePrefix(subnet.Prefix)
				if err != nil {
					return fmt.Errorf("Invalid subnet %s (id=%d) in DB: %s", subnet.Prefix, subnet.ID, err)
				}
				wgAllowedIPs = append(wgAllowedIPs, pref)
			}

			// Remove old peer from wg
			// Unfortunately, there's no sane way to do this transactionaly

			if err := node.wg.RemovePeer(publicKey); err != nil {
				return err
			}

			if updatePeer.IsEnabled {
				// Add new to wg
				if err := node.wg.PutPeer(&wg.Peer{
					PublicKey:           publicKey,
					PresharedKey:        presharedKey,
					Endpoint:            req.Endpoint,
					PersistentKeepalive: req.PersistentKeepalive,
					AllowedIPs:          wgAllowedIPs,
				}); err != nil {
					return err
				}
			}

			return nil
		},
	}.Do(r.Context())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "Peer not found")
			return
		} else {
			internalServerError(w, r, err)
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}
