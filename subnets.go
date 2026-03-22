package main

import (
	"akhokhlow80/tanlnode/nettree"
	"akhokhlow80/tanlnode/sqlgen"
	"akhokhlow80/tanlnode/tx"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"strconv"
)

func (node *node) registerSubnetHandlers(mux *http.ServeMux) {
	mux.HandleFunc("POST /subnets/reserved", node.apiReserveSubnet)
	mux.HandleFunc("GET /subnets/reserved", node.apiGetReservedSubnets)
	mux.HandleFunc("DELETE /subnets/{id}", node.apiDeleteSubnet)
}

type ReserveSubnetRequest struct {
	Prefix     string `json:"prefix"`
	Comment    string `json:"comment"` // Can be empty
	MayOverlap bool   `json:"may_overlap"`
}

type SubnetResponse struct {
	ID         int64  `json:"id"`
	Prefix     string `json:"prefix"`
	Comment    string `json:"comment"`
	MayOverlap bool   `json:"may_overlap"`
}

func (resp *SubnetResponse) fromDB(dbSubnet *sqlgen.Subnet) {
	resp.ID = dbSubnet.ID
	resp.Prefix = dbSubnet.Prefix
	resp.Comment = dbSubnet.Comment
	resp.MayOverlap = dbSubnet.MayOverlap
}

// @Summary	Reserve a subnet
// @Tags		subnets
// @Accept		json
// @Produce	json
// @Param		subnet	body		ReserveSubnetRequest	true	"subnet"
// @Success	201		{object}	SubnetResponse
// @Failure	400		{object}	APIError	"Bad request"
// @Failure	409		{object}	APIError	"Subnet overlaps with existing"
// @Router		/api/v1/subnets/reserved [post]
func (node *node) apiReserveSubnet(w http.ResponseWriter, r *http.Request) {
	var req ReserveSubnetRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	prefix, err := netip.ParsePrefix(req.Prefix)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid prefix")
		return
	}
	if prefix.Masked().Addr().Compare(prefix.Addr()) != 0 {
		respondError(w, http.StatusBadRequest, "Prefix has non-zero host bits")
		return
	}

	var tree nettree.IPTree
	var reservedSubnet SubnetResponse
	err = tx.Transactional{
		Action: func(ctx context.Context) error {
			node.db.Lock()

			if !req.MayOverlap {
				tree, err = node.subnets.Reserve(prefix)
				if err != nil {
					return err
				}
			}
			dbSubnet, err := node.db.AddSubnet(r.Context(), sqlgen.AddSubnetParams{
				Prefix:     fmt.Sprintf("%s/%d", prefix.Addr().StringExpanded(), prefix.Bits()),
				PeerID:     nil,
				Comment:    req.Comment,
				MayOverlap: req.MayOverlap,
			})
			if err != nil {
				return err
			}
			reservedSubnet.fromDB(&dbSubnet)
			return nil
		},
		Rollback: func(ctx context.Context) error {
			defer node.db.Unlock()

			if tree != nil {
				tree.Delete(prefix)
			}
			return nil
		},
		Commit: func(ctx context.Context) error {
			node.db.Unlock()
			return nil
		},
	}.Do(r.Context())
	if err != nil {
		if errors.Is(err, nettree.ErrOverlaps) {
			respondError(w, http.StatusConflict, "Subnet overlaps with the existing one")
			return
		} else if errors.Is(err, nettree.ErrOutOfNet) {
			respondError(w, http.StatusBadRequest, "Subnet is out of any configured alloc net")
			return
		} else {
			internalServerError(w, r, err)
			return
		}
	}
	respondJSON(w, http.StatusCreated, reservedSubnet)
}

// @Summary	Get reserved subnets
// @Tags		subnets
// @Accept		json
// @Produce	json
// @Success	200	{object}	[]SubnetResponse
// @Router		/api/v1/subnets/reserved [get]
func (node *node) apiGetReservedSubnets(w http.ResponseWriter, r *http.Request) {
	dbSubnets, err := func() ([]sqlgen.Subnet, error) {
		node.db.RLock()
		defer node.db.RUnlock()
		return node.db.GetReservedSubnets(r.Context())
	}()
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	subnets := make([]SubnetResponse, 0, len(dbSubnets))
	for _, dbSubnet := range dbSubnets {
		var subnet SubnetResponse
		subnet.fromDB(&dbSubnet)
		subnets = append(subnets, subnet)
	}
	respondJSON(w, http.StatusOK, subnets)
}

// @Summary	Delete a subnet
// @Tags		subnets
// @Produce	json
// @Param		key	path	string	true	"Subnet id"
// @Success	204	"No content"
// @Failure	404	{object}	APIError	"Not found"
// @Router		/api/v1/subnets/{key} [delete]
func (node *node) apiDeleteSubnet(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid id")
		return
	}

	node.db.Lock()
	defer node.db.Unlock()

	subnet, err := node.db.GetSubnetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "Subnet not found")
			return
		} else {
			internalServerError(w, r, err)
		}
	}

	prefix, err := netip.ParsePrefix(subnet.Prefix)
	if err != nil {
		internalServerError(w, r,
			fmt.Errorf(
				"Failed to parse subnet prefix %s from DB, id %d, err: %s",
				subnet.Prefix,
				subnet.ID,
				err,
			),
		)
		return
	}
	if !subnet.MayOverlap {
		if !node.subnets.Delete(prefix) {
			logReqPrintf(
				r,
				"Warning: subnet %s (id=%d) was found in DB, but is not presented in net trees",
				subnet.Prefix,
				subnet.ID,
			)
		}
	}

	_, err = node.db.DeleteSubnet(r.Context(), id)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
