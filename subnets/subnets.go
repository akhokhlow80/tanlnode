package subnets

import (
	"akhokhlow80/tanlnode/db"
	"akhokhlow80/tanlnode/nettree"
	"akhokhlow80/tanlnode/utils"
	"context"
	"errors"
	"fmt"
	"net/netip"
)

type Service struct {
	netTrees []nettree.IPTree
}

func NewService(ctx context.Context, allocNets []netip.Prefix, db *db.DB) (Service, error) {
	var service Service
	for _, net := range allocNets {
		var tree nettree.IPTree
		if net.Addr().Is4() {
			tree4, err := nettree.NewAddrTree4(net)
			if err != nil {
				return Service{},
					fmt.Errorf("Error creating tree for alloc net %s: %s", net.String(), err)
			}
			tree = &tree4
		} else if net.Addr().Is6() {
			tree6, err := nettree.NewAddrTree6(net)
			if err != nil {
				return Service{},
					fmt.Errorf("Error creating tree for alloc net %s: %s", net.String(), err)
			}
			tree = &tree6
		} else {
			panic("never")
		}
		service.netTrees = append(service.netTrees, tree)
	}

	defer db.RUnlock()
	db.RLock()
	subnets, err := db.GetAllSubnets(ctx)
	if err != nil {
		return Service{}, err
	}
	for _, subnet := range subnets {
		if subnet.MayOverlap {
			continue
		}
		pref, err := netip.ParsePrefix(subnet.Prefix)
		if err != nil {
			return Service{},
				fmt.Errorf("Invalid prefix from DB: %s, subnet id: %d, error: %s", subnet.Prefix, subnet.ID, err)
		}
		_, err = service.Reserve(pref)
		if err != nil {
			return Service{},
				fmt.Errorf("Failed to reserve subnet %s from DB with id: %d, error: %s", subnet.Prefix, subnet.ID, err)
		}
	}

	return service, nil
}

// Errors:
//
//   - ErrOverlaps if some net was found with overlapping subnet added.
//   - ErrOutOfNet if no net that fits the given subnet is among the alloc nets.
func (s *Service) Reserve(pref netip.Prefix) (nettree.IPTree, error) {
	for _, tree := range s.netTrees {
		if (tree.Version() == 4) != pref.Addr().Is4() {
			continue
		}
		err := tree.Put(pref)
		if err == nil {
			return tree, nil
		}
		if !errors.Is(err, nettree.ErrOutOfNet) {
			return nil, err
		}
	}
	return nil, nettree.ErrOutOfNet
}

func (s *Service) Delete(pref netip.Prefix) bool {
	for _, tree := range s.netTrees {
		ok := tree.Delete(pref)
		if ok {
			return true
		}
	}
	return false
}

// Deletes assigned addresses on failure.
func (s *Service) AssignRandomInEachNet() ([]netip.Addr, error) {
	var err error
	addrs := make([]netip.Addr, 0, len(s.netTrees))
	for _, tree := range s.netTrees {
		var addr netip.Addr
		addr, err = tree.AllocateRandom()
		if err != nil {
			goto error
		}
		addrs = append(addrs, addr)
	}
	return addrs, nil

error:
	for i := range addrs {
		s.netTrees[i].Delete(utils.MakePrefixFromAddr(addrs[i]))
	}
	return nil, err
}
