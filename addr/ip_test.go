package addr

import (
	"errors"
	"net/netip"
	"testing"
)

func mustCreateAddrTree4(prefix string) *IP4Tree {
	net := netip.MustParsePrefix(prefix)
	tree, err := NewAddrTree4(net)
	if err != nil {
		panic(err)
	}
	return tree
}

func TestNewAddrTree4(t *testing.T) {
	net24 := netip.MustParsePrefix("192.168.0.0/24")
	_, err := NewAddrTree4(net24)
	if err != nil {
		t.Fatalf("NewAddrTree4(/24) failed: %v", err)
	}

	net31 := netip.MustParsePrefix("192.168.0.0/31")
	_, err = NewAddrTree4(net31)
	if !errors.Is(err, ErrNetTooSmall) {
		t.Errorf("expected ErrNetToSmall for /31, got %v", err)
	}
}

func TestAddrTree4Put(t *testing.T) {
	tree := mustCreateAddrTree4("192.168.0.0/24")
	rootNet := netip.MustParsePrefix("192.168.0.0/24")

	err := tree.Put(rootNet)
	if !errors.Is(err, ErrOverlaps) {
		t.Errorf("Put(root) expected ErrOverlaps, got %v", err)
	}

	longer := netip.MustParsePrefix("192.168.0.0/25")
	err = tree.Put(longer)
	if !errors.Is(err, ErrOverlaps) {
		t.Errorf("Put(/25) expected ErrOverlaps, got %v", err)
	}

	shorter := netip.MustParsePrefix("192.168.0.0/23")
	err = tree.Put(shorter)
	if !errors.Is(err, ErrOutOfNet) {
		t.Errorf("Put(/23) expected ErrOutOfNet: %v", err)
	}

	validSubnet := netip.MustParsePrefix("192.168.0.4/30")
	err = tree.Put(validSubnet)
	if err != nil {
		t.Errorf("Put(valid /30) failed: %v", err)
	}

	outside := netip.MustParsePrefix("192.168.1.0/24")
	err = tree.Put(outside)
	if !errors.Is(err, ErrOutOfNet) {
		t.Errorf("Put(outside) expected ErrOutOfNet, got %v", err)
	}
}

func TestAddrTree4Delete(t *testing.T) {
	tree := mustCreateAddrTree4("192.168.0.0/24")
	subnet := netip.MustParsePrefix("192.168.0.4/30")

	removed := tree.Delete(subnet)
	if removed {
		t.Error("Delete(nonexistent) should return false")
	}

	err := tree.Put(subnet)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}
	removed = tree.Delete(subnet)
	if !removed {
		t.Error("Delete(existing) should return true")
	}

	removed = tree.Delete(subnet)
	if removed {
		t.Error("Delete(already removed) should return false")
	}
}

func TestAddrTree4AllocateRandom(t *testing.T) {
	tree := mustCreateAddrTree4("192.168.0.0/29")
	tree.dump(t)
	for range 6 {
		randomAddr, err := tree.AllocateRandom()
		if err != nil {
			t.Errorf("AllocateRandom returned error: %s", err)
			break
		}
		tree.dump(t)
		err = tree.Put(netip.PrefixFrom(randomAddr, 32))
		if !errors.Is(err, ErrOverlaps) {
			t.Error("Put(randomAddr) should return ErrOverlaps")
		}
	}
	_, err := tree.AllocateRandom()
	if !errors.Is(err, ErrNoFreeAddr) {
		t.Errorf("AllocateRandom expected to return ErrNoFreeAddr, got %s", err)
	}
}

func mustCreateAddrTree6(prefix string) *IP6Tree {
	net := netip.MustParsePrefix(prefix)
	tree, err := NewAddrTree6(net)
	if err != nil {
		panic(err)
	}
	return tree
}

func TestNewAddrTree6(t *testing.T) {
	net120 := netip.MustParsePrefix("fd62:4245:7b96::/120")
	_, err := NewAddrTree6(net120)
	if err != nil {
		t.Fatalf("NewAddrTree6(/120) failed: %v", err)
	}

	net127 := netip.MustParsePrefix("fd62:4245:7b96::/127")
	_, err = NewAddrTree6(net127)
	if !errors.Is(err, ErrNetTooSmall) {
		t.Errorf("expected ErrNetToSmall for /127, got %v", err)
	}
	net64 := netip.MustParsePrefix("fd62:4245:7b96::/63")
	_, err = NewAddrTree6(net64)
	if !errors.Is(err, ErrBigNet6NotSupported) {
		t.Errorf("expected ErrBigNet6NotSupported for /64, got %v", err)
	}
}

func TestAddrTree6Put(t *testing.T) {
	tree := mustCreateAddrTree6("fd62:4245:7b96::/120")
	rootNet := netip.MustParsePrefix("fd62:4245:7b96::/120")

	err := tree.Put(rootNet)
	if !errors.Is(err, ErrOverlaps) {
		t.Errorf("Put(root) expected ErrOverlaps, got %v", err)
	}

	longer := netip.MustParsePrefix("fd62:4245:7b96::/121")
	err = tree.Put(longer)
	if !errors.Is(err, ErrOverlaps) {
		t.Errorf("Put(/121) expected ErrOverlaps, got %v", err)
	}

	shorter := netip.MustParsePrefix("fd62:4245:7b96::/119")
	err = tree.Put(shorter)
	if !errors.Is(err, ErrOutOfNet) {
		t.Errorf("Put(/119) expected ErrOutOfNet: %v", err)
	}

	validSubnet := netip.MustParsePrefix("fd62:4245:7b96::6/127")
	tree.dump(t)
	t.Logf("%t", tree.uintTree.Put(4, 8))
	err = tree.Put(validSubnet)
	if err != nil {
		t.Errorf("Put(valid /122) failed: %v", err)
	}

	outside := netip.MustParsePrefix("fd62:4245:7b96:1::/24")
	err = tree.Put(outside)
	if !errors.Is(err, ErrOutOfNet) {
		t.Errorf("Put(outside) expected ErrOutOfNet, got %v", err)
	}
}

func TestAddrTree6Delete(t *testing.T) {
	tree := mustCreateAddrTree6("fd62:4245:7b96::/120")
	subnet := netip.MustParsePrefix("fd62:4245:7b96::6/127")

	removed := tree.Delete(subnet)
	if removed {
		t.Error("Delete(nonexistent) should return false")
	}

	err := tree.Put(subnet)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}
	removed = tree.Delete(subnet)
	if !removed {
		t.Error("Delete(existing) should return true")
	}

	removed = tree.Delete(subnet)
	if removed {
		t.Error("Delete(already removed) should return false")
	}
}

func TestAddrTree6AllocateRandom(t *testing.T) {
	tree := mustCreateAddrTree6("fd62:4245:7b96::/125")
	tree.dump(t)
	for range 7 {
		randomAddr, err := tree.AllocateRandom()
		if err != nil {
			t.Errorf("AllocateRandom returned error: %s", err)
			break
		}
		tree.dump(t)
		err = tree.Put(netip.PrefixFrom(randomAddr, 128))
		if !errors.Is(err, ErrOverlaps) {
			t.Error("Put(randomAddr) should return ErrOverlaps")
		}
	}
	_, err := tree.AllocateRandom()
	if !errors.Is(err, ErrNoFreeAddr) {
		t.Errorf("AllocateRandom expected to return ErrNoFreeAddr, got %s", err)
	}

	tree = mustCreateAddrTree6("fd62:4245:7b96:3531::/64")
	_, err = tree.AllocateRandom()
	if err != nil {
		t.Errorf("AllocateRandom returned error: %s", err)
	}
}
