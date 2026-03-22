package subnets

import (
	"akhokhlow80/tanlnode/nettree"
	"akhokhlow80/tanlnode/sqlgen"
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sync"
)

// Kepp lock while using!
type SyncedIPTree struct {
	sync.Mutex
	nettree.IPTree
}

type Service struct {
	queries *struct {
		sync.RWMutex
		*sqlgen.Queries
	}
	netTrees []*SyncedIPTree
}

func NewService(ctx context.Context, allocNets []netip.Prefix, queries *struct {
	sync.RWMutex
	*sqlgen.Queries
}) (Service, error) {
	var service Service
	for _, net := range allocNets {
		tree := new(SyncedIPTree)
		if net.Addr().Is4() {
			tree4, err := nettree.NewAddrTree4(net)
			if err != nil {
				return Service{},
					fmt.Errorf("Error creating tree for alloc net %s: %s", net.String(), err)
			}
			tree.IPTree = &tree4
		} else if net.Addr().Is6() {
			tree6, err := nettree.NewAddrTree6(net)
			if err != nil {
				return Service{},
					fmt.Errorf("Error creating tree for alloc net %s: %s", net.String(), err)
			}
			tree.IPTree = &tree6
		} else {
			panic("never")
		}
		service.netTrees = append(service.netTrees, tree)
	}

	queries.RLock()
	subnets, err := queries.GetAllSubnets(ctx)
	queries.RUnlock()
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
func (s *Service) Reserve(pref netip.Prefix) (*SyncedIPTree, error) {
	if pref.Addr().Is4() {
		for _, tree := range s.netTrees {
			err := func() error {
				defer tree.Unlock()
				tree.Lock()
				return tree.Put(pref)
			}()
			if err == nil {
				return tree, nil
			}
			if !errors.Is(err, nettree.ErrOutOfNet) {
				return nil, err
			}
		}
	}
	return nil, nettree.ErrOutOfNet
}

func (s *Service) Delete(pref netip.Prefix) bool {
	for _, tree := range s.netTrees {
		ok := func() bool {
			defer tree.Unlock()
			tree.Lock()
			return tree.Delete(pref)
		}()
		if ok {
			return true
		}
	}
	return false
}
