package nettree

import (
	"math/big"
)

type uintNode[T ~uint32 | ~uint64 | ~uint16] struct {
	parent, left, right *uintNode[T]
	childrenCnt         T // 0 in root always means tree is full (due to overflow). Empty tree has nil root
}

type uintTree[T ~uint32 | ~uint64 | ~uint16] struct {
	root        *uintNode[T]
	msBit       T
	maxUint     T
	maxPrefBits int
}

func (tree *uintTree[T]) Overlaps(addr T, prefbits int) bool {
	var prev *uintNode[T]
	node := tree.root
	for {
		if node == nil {
			if prev != nil {
				return prev.left == nil && prev.right == nil
			} else {
				return false
			}
		}

		if prefbits == 0 {
			return true
		}

		prev = node
		if addr&tree.msBit == 0 {
			node = node.left
		} else {
			node = node.right
		}
		prefbits--
		addr <<= 1
	}
}

// Returns true iff a non-overlapping subnet was added
func (tree *uintTree[T]) Put(addr T, prefbits int) bool {
	var prev *uintNode[T]
	var wentRight bool
	childrenCntDiff := T(1) << (tree.maxPrefBits - prefbits)
	node := tree.root
	for {
		// TODO: split in two loops for better readability & performance?

		newNode := false
		if node == nil {
			newNode = true
			node = &uintNode[T]{}
			if prev != nil {
				if wentRight {
					prev.right = node
				} else {
					prev.left = node
				}
				node.parent = prev
			} else {
				tree.root = node
			}
		} else {
			if node.left == nil && node.right == nil {
				return false
			}
		}

		if prefbits == 0 {
			if newNode {
				goto updateParents
			} else {
				return false
			}
		}

		prev = node
		if addr&tree.msBit == 0 {
			node = node.left
			wentRight = false
		} else {
			node = node.right
			wentRight = true
		}

		addr <<= 1
		prefbits--
	}

updateParents:
	for node != nil {
		node.childrenCnt += childrenCntDiff
		node = node.parent
	}
	return true
}

func (tree *uintTree[T]) Delete(addr T, prefbits int) bool {
	node := tree.root
	childrenCntDiff := T(1 << (tree.maxPrefBits - prefbits))
	for {
		if node == nil {
			return false
		}

		if prefbits == 0 {
			if node.left == nil && node.right == nil {
				goto remove
			} else {
				return false
			}
		}

		if addr&tree.msBit == 0 {
			node = node.left
		} else {
			node = node.right
		}

		prefbits--
		addr <<= 1
	}

remove:
	for node != nil {
		node.childrenCnt -= childrenCntDiff

		if node.left == nil && node.right == nil {
			if node.parent != nil {
				if node.parent.left == node {
					node.parent.left = nil
				} else {
					node.parent.right = nil
				}
			} else {
				tree.root = nil
				break
			}
		}

		node = node.parent
	}
	return true
}

// Call only if there's at least n free addresses.
func (tree *uintTree[T]) PutNthFree(n T) T {
	netStart := T(0)
	node := tree.root
	bit := tree.msBit
	for {
		if node == nil {
			if !tree.Put(netStart+n, tree.maxPrefBits) {
				panic("never")
			}
			return netStart + n
		}

		leftFreeElsCnt := bit
		if node.left != nil {
			leftFreeElsCnt -= node.left.childrenCnt
		}

		if n < leftFreeElsCnt {
			node = node.left
		} else {
			node = node.right
			netStart |= bit
			n -= leftFreeElsCnt
		}

		bit >>= 1
	}
}

func (tree *uintTree[T]) Size() *big.Int {
	if tree.root == nil {
		return big.NewInt(0)
	}
	size := new(big.Int).SetUint64(uint64(tree.root.childrenCnt))
	if tree.root.childrenCnt == 0 {
		size.SetUint64(uint64(tree.maxUint))
		size = size.Add(size, big.NewInt(1))
	}
	return size
}
