package u64

import (
	"math/bits"
	"sync/atomic"
)

// calcMask calculates mask for slot = hash & mask.
func calcMask(tableCap uint32) uint32 {
	if tableCap <= neighbour {
		return tableCap - 1
	}
	return tableCap - neighbour // Always has a virtual bucket with neigh slots.
}

// calcTableCap calculates the actual capacity of a table.
// This capacity will add a bit extra slots for improving load factor hugely in some cases:
// If there two keys being hashed to the highest position, the Set will have to be expanded
// if there is no extra space.
func calcTableCap(c int) int {
	if c <= neighbour {
		return c
	}
	return c + neighbour - 1
}

// backToOriginCap calculates the origin capacity by actual capacity.
// The origin capacity will be the visible capacity outside.
func backToOriginCap(c int) int {
	if c <= neighbour {
		return c
	}
	return c + 1 - neighbour
}

func (s *Set) getWritableTable() []uint64 {
	idx := s.getWritableIdx()
	p := atomic.LoadPointer(&s.cycle[idx])
	return *(*[]uint64)(p)
}

// getTblSlot gets writable table and slot
func (s *Set) getTblSlot(key uint64) (idx uint8, tbl []uint64, slot int) {
	idx = getWritableIdxByStatus(atomic.LoadUint64(&s.status))
	tbl, slot = s.getTblSlotByIdx(idx, key)
	return
}

func (s *Set) getTblSlotByIdx(idx uint8, key uint64) (tbl []uint64, slot int) {
	p := atomic.LoadPointer(&s.cycle[idx])
	if p == nil {
		return nil, 0
	}

	tbl = *(*[]uint64)(p)
	h := calcHash(idx, key)
	slotCnt := len(tbl)
	slot = int(h & (calcMask(uint32(slotCnt))))
	return
}

func getTbl(s *Set, idx int) []uint64 {
	p := atomic.LoadPointer(&s.cycle[idx])
	if p == nil {
		return nil
	}

	return *(*[]uint64)(p)
}

func nextPower2(n uint64) uint64 {
	if n <= 1 {
		return 1
	}

	return 1 << (64 - bits.LeadingZeros64(n-1)) // TODO may use BSR instruction.
}
