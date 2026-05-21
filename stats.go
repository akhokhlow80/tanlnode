package main

import (
	"akhokhlow80/tanlnode/sqlgen"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
)

func (node *node) registerStatsHandlers(mux *http.ServeMux) {
	mux.HandleFunc("GET /peers/{pubkey}/stat", node.apiGetPeerTotalStatByPublicKey)
	mux.HandleFunc("GET /peers/stat", node.apiGetPeersTotalStat)
}

// DB must be rlocked during call
//
// toMs == 0 means no upper limit
func (node *node) getPeerStats(
	ctx context.Context,
	peerID int64,
	fromMs int64,
	toMs int64,
) (tx int64, rx int64, err error) {
	untilMs := fromMs - 1
	stat1, err := node.db.GetLastPeerStat(ctx, sqlgen.GetLastPeerStatParams{
		PeerID:  peerID,
		UntilMs: &untilMs,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			stat1.Tx = 0
			stat1.Rx = 0
		} else {
			return 0, 0, err
		}
	}

	var toMsPtr *int64
	if toMs != 0 {
		toMsPtr = new(int64)
		*toMsPtr = toMs
	}
	stat2, err := node.db.GetLastPeerStat(ctx, sqlgen.GetLastPeerStatParams{
		PeerID:  peerID,
		UntilMs: toMsPtr,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			stat2.Tx = 0
			stat2.Rx = 0
		} else {
			return 0, 0, err
		}
	}

	return stat2.Tx - stat1.Tx, stat2.Rx - stat1.Rx, nil
}

type TransferStatResponse struct {
	PublicKeyBase64 string `json:"public_key_base64"`
	Tx              int64  `json:"tx"`
	Rx              int64  `json:"rx"`
}

func parsePeriod(fromMs string, toMs string) (fromMsParsed int64, toMsParsed int64, err error) {
	if fromMs != "" {
		fromMsParsed, err = strconv.ParseInt(fromMs, 10, 64)
		if err != nil {
			err = fmt.Errorf("Invalid from_ms")
			return
		}
	}
	if toMs != "" {
		toMsParsed, err = strconv.ParseInt(toMs, 10, 64)
		if err != nil {
			err = fmt.Errorf("Invalid to_ms")
			return
		}
	}
	return
}

// @Summary	Get total stat for peer
// @Tags		stats
// @Produce	json
// @Param		from_ms	query		number	false	"start of the period"
// @Param		to_ms	query		number	false	"end of the period; 0 means no limit"
// @Param		pubkey	path		string	true	"Peer key"
// @Success	200		{object}	TransferStatResponse
// @Failure	400		{object}	APIError	"Invalid period"
// @Failure	404		{object}	APIError	"Peer not found"
// @Router		/api/v1/peers/{pubkey}/stat [get]
func (node *node) apiGetPeerTotalStatByPublicKey(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	pubkey := r.PathValue("pubkey")
	fromMsParsed, toMsParsed, err := parsePeriod(
		query.Get("from_ms"),
		query.Get("to_ms"),
	)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	tx, rx, err := func() (tx int64, rx int64, err error) {
		node.db.RLock()
		defer node.db.RUnlock()

		peer, err := node.db.GetPeerByPublicKey(r.Context(), pubkey)
		if err != nil {
			return 0, 0, err
		}

		return node.getPeerStats(r.Context(), peer.ID, fromMsParsed, toMsParsed)
	}()
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "Peer not found")
			return
		} else {
			internalServerError(w, r, err)
			return
		}
	}

	respondJSON(w, http.StatusOK, TransferStatResponse{
		PublicKeyBase64: pubkey,
		Tx:              tx,
		Rx:              rx,
	})
}

// @Summary	Get total stat for multiple peers
// @Tags		stats
// @Produce	json
// @Param		from_ms	query		number	false	"start of the period"
// @Param		to_ms	query		number	false	"end of the period; 0 means no limit"
// @Param		owner	query		string	false	"owner"
// @Success	200		{object}	[]TransferStatResponse
// @Failure	400		{object}	APIError	"Invalid period"
// @Router		/api/v1/peers/stat [get]
func (node *node) apiGetPeersTotalStat(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	owner := query.Get("owner")
	fromMsParsed, toMsParsed, err := parsePeriod(
		query.Get("from_ms"),
		query.Get("to_ms"),
	)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	stats, err := func() ([]TransferStatResponse, error) {
		defer node.db.RUnlock()
		node.db.RLock()

		var ownerPtr *string
		if owner != "" {
			ownerPtr = &owner
		}
		peers, err := node.db.GetPeers(r.Context(), ownerPtr)
		if err != nil {
			return nil, err
		}

		stats := make([]TransferStatResponse, 0, len(peers))
		for _, peer := range peers {
			tx, rx, err := node.getPeerStats(r.Context(), peer.ID, fromMsParsed, toMsParsed)
			if err != nil {
				return nil, err
			}
			stats = append(stats, TransferStatResponse{
				PublicKeyBase64: peer.PublicKeyBase64,
				Tx:              tx,
				Rx:              rx,
			})
		}

		return stats, nil
	}()
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, stats)
	return
}
