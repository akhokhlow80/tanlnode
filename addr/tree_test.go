package addr

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"
)

func (tree *uintTree[T]) dump(t *testing.T) {
	var dumpRec func(node *uintNode[T], level int, wentRight bool, addr T, bit T)
	dumpRec = func(node *uintNode[T], level int, wentRight bool, addr T, bit T) {
		if node == nil {
			return
		}
		var sb strings.Builder
		for range level {
			sb.WriteRune(' ')
		}
		if wentRight {
			sb.WriteString("R")
		} else {
			sb.WriteString("L")
		}
		t.Logf("%s %x/%d %d [%p]\n", sb.String(), addr, level, node.childrenCnt, node)
		dumpRec(node.left, level+1, false, addr, bit>>1)
		dumpRec(node.right, level+1, true, addr|bit, bit>>1)
	}
	t.Log("DUMP")
	dumpRec(tree.root, 0, false, 0, tree.msBit)
}

func (tree *uintTree[T]) checkInvariant(t *testing.T) {
	var checkInvRec func(node *uintNode[T], level int) T
	checkInvRec = func(node *uintNode[T], level int) T {
		if node == nil {
			return 0
		}
		leftChildren := checkInvRec(node.left, level-1)
		rightChildren := checkInvRec(node.right, level-1)

		var cnt T
		if node.left == nil && node.right == nil {
			cnt = T(1) << level
		} else {
			cnt = leftChildren + rightChildren
		}
		if cnt != node.childrenCnt {
			tree.dump(t)
			t.Fatalf("Invariant check failed at node: %p %d != %d", node, cnt, node.childrenCnt)
		}

		return cnt
	}
	checkInvRec(tree.root, tree.maxPrefBits)
}

func TestUintTree(t *testing.T) {
	{
		tree := uintTree[uint16]{msBit: 1 << 15, maxUint: 65535, maxPrefBits: 16}

		if !tree.Put(0x0000, 0) {
			t.Fatalf("Invalid Put return")
		}
		tree.checkInvariant(t)
		if tree.Put(0x0000, 0) {
			t.Fatalf("Invalid Put return")
		}
		tree.checkInvariant(t)
		if tree.Put(0x0000, 1) {
			t.Fatalf("Invalid Put return")
		}
		tree.checkInvariant(t)
		if tree.Put(0xffff, 16) {
			t.Fatalf("Invalid Put return")
		}
		tree.checkInvariant(t)
		if !tree.Overlaps(0x0000, 0) {
			t.Fatalf("Invalid Overlaps return")
		}
		if !tree.Overlaps(0x0000, 1) {
			t.Fatalf("Invalid Overlaps return")
		}
		if !tree.Overlaps(0x8000, 1) {
			t.Fatalf("Invalid Overlaps return")
		}
		if !tree.Overlaps(0x1234, 11) {
			t.Fatalf("Invalid Overlaps return")
		}
	}
	{
		tree := uintTree[uint16]{msBit: 1 << 15, maxUint: 65535, maxPrefBits: 16}
		if !tree.Put(0x0000, 2) {
			t.Fatalf("Invalid Put return")
		}
		tree.checkInvariant(t)
		if tree.Put(0x3000, 4) {
			t.Fatalf("Invalid Put return")
		}
		tree.checkInvariant(t)
		if !tree.Put(0xa000, 3) {
			t.Fatalf("Invalid Put return")
		}
		tree.checkInvariant(t)
		if !tree.Put(0xe000, 3) {
			t.Fatalf("Invalid Put return")
		}
		tree.checkInvariant(t)

		if !tree.Overlaps(0x0000, 1) {
			t.Fatalf("Invalid Overlaps return")
		}
		if tree.Overlaps(0x4000, 2) {
			t.Fatalf("Invalid Overlaps return")
		}
		if !tree.Overlaps(0x3000, 1) {
			t.Fatalf("Invalid Overlaps return")
		}
		if !tree.Overlaps(0xa000, 3) {
			t.Fatalf("Invalid Overlaps return")
		}
		if !tree.Overlaps(0xe000, 3) {
			t.Fatalf("Invalid Overlaps return")
		}
		if !tree.Overlaps(0x8000, 1) {
			t.Fatalf("Invalid Overlaps return")
		}

		if tree.Delete(0x0000, 0) {
			t.Fatalf("Invalid Delete return")
		}
		tree.checkInvariant(t)
		if !tree.Delete(0xa000, 3) {
			t.Fatalf("Invalid Delete return")
		}
		tree.checkInvariant(t)
		if tree.Overlaps(0xa000, 3) {
			t.Fatalf("Invalid Overlaps return")
		}
		tree.checkInvariant(t)
		if !tree.Delete(0xe000, 3) {
			t.Fatalf("Invalid Delete return")
		}
		tree.checkInvariant(t)
		if tree.Overlaps(0xe000, 3) {
			t.Fatalf("Invalid Overlaps return")
		}
		if tree.Overlaps(0x8000, 1) {
			t.Fatalf("Invalid Overlaps return")
		}
		if !tree.Put(0xf000, 4) {
			t.Fatalf("Invalid Put return")
		}
		tree.checkInvariant(t)

		if !tree.Delete(0xf000, 4) {
			t.Fatalf("Invalid Delete return")
		}
		tree.checkInvariant(t)
		if !tree.Delete(0x0000, 2) {
			t.Fatalf("Invalid Delete return")
		}
		tree.checkInvariant(t)
		if tree.Overlaps(0x0000, 1) {
			t.Fatalf("Invalid Overlaps return")
		}
		if !tree.Put(0x0000, 1) {
			t.Fatalf("Invalid Put return")
		}
		tree.checkInvariant(t)
	}
}

type dumbUintTree struct {
	els    [65536]bool
	ranges []struct {
		removed bool
		start   uint16
		hosts   uint16
	}
}

func (tree *dumbUintTree) Put(addr uint16, prefBits int) bool {
	hosts := (1 << (16 - prefBits)) - 1
	if tree.Overlaps(addr, prefBits) {
		return false
	}
	for i := 0; i <= hosts; i++ {
		tree.els[int(addr)+i] = true
	}
	tree.ranges = append(tree.ranges, struct {
		removed bool
		start   uint16
		hosts   uint16
	}{false, addr, uint16(hosts)})
	return true
}

func (tree *dumbUintTree) Overlaps(addr uint16, prefBits int) bool {
	hosts := (1 << (16 - prefBits)) - 1
	for i := 0; i <= hosts; i++ {
		if tree.els[int(addr)+i] {
			return true
		}
	}
	return false
}

func (tree *dumbUintTree) Delete(addr uint16, prefBits int) bool {
	hosts := uint16((1 << (16 - prefBits)) - 1)
	for i, rng := range tree.ranges {
		if rng.removed {
			continue
		}
		if rng.start == addr && rng.hosts == hosts {
			for j := 0; j <= int(hosts); j++ {
				tree.els[int(addr)+j] = false
			}
			tree.ranges[i].removed = true
			return true
		}
	}
	return false
}

func (tree *dumbUintTree) PutNthFree(n uint16) uint16 {
	for i := uint16(0); ; i++ {
		if !tree.els[i] {
			if n == 0 {
				tree.els[i] = true
				tree.ranges = append(tree.ranges, struct {
					removed bool
					start   uint16
					hosts   uint16
				}{
					false, i, 0,
				})
				return i
			} else {
				n--
			}
		}
		if i == 65535 {
			panic("never")
		}
	}
}

func TestUintTreeRandom(t *testing.T) {
	const N = 200
	const OPS = 500

	for n := range N {
		t.Run(fmt.Sprintf("TestUintTreeRandom_%d", n), func(t *testing.T) {
			t.Parallel()
			rnd := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))

			tree := uintTree[uint16]{msBit: 1 << 15, maxUint: 65535, maxPrefBits: 16}
			dumbTree := new(dumbUintTree)

			for range OPS {
				addr := uint16(rnd.Uint32())
				prefBits := rnd.Uint32N(16 + 1)
				addr &= ^uint16((1 << (16 - prefBits)) - 1)
				switch rnd.Int32N(4) {
				case 0:
					treePut := tree.Put(addr, int(prefBits))
					dumbTreePut := dumbTree.Put(addr, int(prefBits))
					t.Logf("tree.Put(0x%x, %d) = %t", addr, prefBits, treePut)
					t.Logf("dumbTree.Put(0x%x, %d) = %t", addr, prefBits, dumbTreePut)
					if treePut != dumbTreePut {
						tree.dump(t)
						t.FailNow()
					}
				case 1:
					treeOverlaps := tree.Overlaps(addr, int(prefBits))
					dumbOverlaps := dumbTree.Overlaps(addr, int(prefBits))
					if treeOverlaps != dumbOverlaps {
						t.Logf("tree.Overlaps(0x%x, %d) = %t", addr, int(prefBits), treeOverlaps)
						t.Logf("dumbTree.Overlaps(0x%x, %d) = %t", addr, int(prefBits), dumbOverlaps)
						tree.dump(t)
						t.FailNow()
					}
				case 2:
					treeDelete := tree.Delete(addr, int(prefBits))
					dumbTreeDelete := dumbTree.Delete(addr, int(prefBits))
					t.Logf("tree.Delete(0x%x, %d) = %t", addr, prefBits, treeDelete)
					t.Logf("dumbTree.Delete(0x%x, %d) = %t", addr, prefBits, dumbTreeDelete)
					if treeDelete != dumbTreeDelete {
						tree.dump(t)
						t.FailNow()
					}
				case 3:
					nMax := 65536 - tree.Size().Uint64()
					if nMax == 0 {
						continue
					}
					n := uint16(rnd.Uint64N(nMax))
					t.Logf("tree.PutNthFree(%d)", n)
					t.Logf("dumbTree.PutNthFree(%d)", n)
					tree.dump(t)
					nthAddr := tree.PutNthFree(n)
					nthDumbAddr := dumbTree.PutNthFree(n)
					t.Logf("tree.PutNthFree(%d) = %x", n, nthAddr)
					t.Logf("dumbTree.PutNthFree(%d) = %x", n, nthDumbAddr)
					if nthAddr != nthDumbAddr {
						tree.dump(t)
						t.FailNow()
					}
				}
				tree.checkInvariant(t)
			}
		})
	}
}
