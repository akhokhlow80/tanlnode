package addr

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"math/big"
	"net/netip"
)

var ErrOutOfNet = errors.New("Address is out of the net")
var ErrOverlaps = errors.New("Address overlaps with the already assigned ones")
var ErrNetTooSmall = errors.New("Network is too small")
var ErrBigNet6NotSupported = errors.New("IPv6 net with more than 2^64 hosts is not supported")
var ErrNoFreeAddr = errors.New("No free address in a network")

type IPTree interface {
	// Errors: ErrOutOfNet, ErrOverlaps
	Put(pref netip.Prefix) error
	// Returns true iff subnet was found and removed.
	Delete(pref netip.Prefix) bool
	// Errors: ErrNoFreeAddr
	AllocateRandom() (netip.Addr, error)
}

type IP4Tree struct {
	Net netip.Prefix // do not change after tree is created
	uintTree[uint32]
}

var _ IPTree = (*IP4Tree)(nil)

// Errors: ErrNetTooSmall
func NewAddrTree4(net netip.Prefix) (IP4Tree, error) {
	if !net.Addr().Is4() {
		panic("expected IP4 net")
	}
	if 32-net.Bits() < 2 {
		return IP4Tree{}, ErrNetTooSmall
	}
	tree := IP4Tree{
		net,
		uintTree[uint32]{
			msBit:       1 << (32 - net.Bits() - 1),
			maxUint:     ^uint32(0) >> net.Bits(),
			maxPrefBits: 32 - net.Bits(),
		},
	}
	tree.uintTree.Put(0, tree.maxPrefBits)
	tree.uintTree.Put(^uint32(0)&tree.maxUint, tree.maxPrefBits)
	return tree, nil
}

func (tree *IP4Tree) Put(pref netip.Prefix) error {
	if !pref.Addr().Is4() {
		panic("expected IP4 net")
	}
	if !tree.Net.Contains(pref.Addr()) || tree.Net.Bits() > pref.Bits() {
		return ErrOutOfNet
	}

	addr := pref.Addr().As4()
	if tree.uintTree.Put(binary.BigEndian.Uint32(addr[:])&tree.maxUint, pref.Bits()-tree.Net.Bits()) {
		return nil
	} else {
		return ErrOverlaps
	}
}

func (tree *IP4Tree) Delete(pref netip.Prefix) bool {
	if !tree.Net.Contains(pref.Addr()) || tree.Net.Bits() > pref.Bits() {
		return false
	}
	addr := pref.Addr().As4()
	return tree.uintTree.Delete(binary.BigEndian.Uint32(addr[:])&tree.maxUint, pref.Bits()-tree.Net.Bits())
}

func (tree *IP4Tree) AllocateRandom() (netip.Addr, error) {
	size := uint64(tree.maxUint) - tree.Size().Uint64() + 1
	if size == 0 {
		return netip.Addr{}, ErrNoFreeAddr
	}
	sizeBig := new(big.Int)
	sizeBig.SetUint64(size)

	nBig, err := rand.Int(rand.Reader, sizeBig)
	if err != nil {
		panic(err)
	}

	offset := tree.PutNthFree(uint32(nBig.Uint64()))
	netAddr := tree.Net.Addr().As4()
	addr := binary.BigEndian.AppendUint32(nil, binary.BigEndian.Uint32(netAddr[:])+offset)

	return netip.AddrFrom4([4]byte(addr)), nil
}

type IP6Tree struct {
	net netip.Prefix
	uintTree[uint64]
}

var _ IPTree = (*IP6Tree)(nil)

// Errors: ErrNetTooSmall, ErrBigNet6NotSupported
func NewAddrTree6(net netip.Prefix) (IP6Tree, error) {
	if !net.Addr().Is6() {
		panic("expected IP6 net")
	}
	if 128-net.Bits() < 2 {
		return IP6Tree{}, ErrNetTooSmall
	}
	if 128-net.Bits() > 64 {
		return IP6Tree{}, ErrBigNet6NotSupported
	}
	tree := IP6Tree{
		net,
		uintTree[uint64]{
			msBit:       1 << (128 - net.Bits() - 1),
			maxUint:     ^uint64(0) >> (net.Bits() - 64),
			maxPrefBits: 128 - net.Bits(),
		},
	}
	tree.uintTree.Put(0, tree.maxPrefBits)
	return tree, nil
}

// Errors: ErrOutOfNet, ErrOverlaps
func (tree *IP6Tree) Put(pref netip.Prefix) error {
	if !pref.Addr().Is6() {
		panic("expected IP6 net")
	}
	if !tree.net.Contains(pref.Addr()) || tree.net.Bits() > pref.Bits() {
		return ErrOutOfNet
	}

	addr := pref.Addr().As16()
	if tree.uintTree.Put(binary.BigEndian.Uint64(addr[8:])&tree.maxUint, pref.Bits()-tree.net.Bits()) {
		return nil
	} else {
		return ErrOverlaps
	}
}

// Returns true iff subnet was found and removed.
// Do not delete host 0.
func (tree *IP6Tree) Delete(pref netip.Prefix) bool {
	if !tree.net.Contains(pref.Addr()) || tree.net.Bits() > pref.Bits() {
		return false
	}
	addr := pref.Addr().As16()
	return tree.uintTree.Delete(binary.BigEndian.Uint64(addr[8:])&tree.maxUint, pref.Bits()-tree.net.Bits())
}

// Errors: ErrNoFreeAddr
func (tree *IP6Tree) AllocateRandom() (netip.Addr, error) {
	// NewAddrTree6 always creates host 0, thus saving us from overflow.
	size := uint64(tree.maxUint) - tree.Size().Uint64() + 1
	if size == 0 {
		return netip.Addr{}, ErrNoFreeAddr
	}
	sizeBig := new(big.Int)
	sizeBig.SetUint64(size)

	nBig, err := rand.Int(rand.Reader, sizeBig)
	if err != nil {
		panic(err)
	}

	offset := tree.PutNthFree(nBig.Uint64())
	addr16 := tree.net.Addr().As16()
	addr8 := binary.BigEndian.AppendUint64(nil, binary.BigEndian.Uint64(addr16[8:])+offset)

	copy(addr16[8:], addr8)

	return netip.AddrFrom16([16]byte(addr16)), nil
}
