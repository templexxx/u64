// Key Concepts:
// 1. Slot:
// Entry container.
// Neighbourhood:
// Key could be found in slot which hashed to or next Neighbourhood - 1 slots.
//
// 2. Bucket:
// It's a virtual struct made of neighbourhood slots.
//
// 3. Table:
// An array of buckets.
package u64

import (
	"errors"
	"runtime"
	"sync/atomic"
	"unsafe"
)

// neighbour is the hopscotch hash neighbourhood size.
//
// TODO testing the load factor
// P is the probability a hopscotch hash table with load factor 0.75
// and the neighborhood size 32 must be rehashed:
// 3.81e-40 < P < 1e-4
//
// If there is no place to set key, try to resize to another bucket until meet MaxCap.
const neighbour = 64

// Set is unsigned 64-bit integer set.
// Lock-free Write & Wait-free Read.
type Set struct {
	status uint64
	// _padding here for avoiding false share.
	// cycle which under the status won't be modified frequently, but read frequently.
	//
	// may don't need it because the the write operations aren't many, few cache miss is okay.
	// remove it could save 128 bytes, it's attractive for application which want to save every bit.
	// _padding [cpu.X86FalseSharingRange]byte

	// cycle is the container of tables,
	// it's made of two uint64 slices.
	// only the one could be inserted at a certain time.
	cycle [2]unsafe.Pointer
}

const (
	// defaultMaxCap is the default maximum capacity of Set.
	defaultMaxCap = 1 << 25 // 32Mi * 8 Byte = 256MB, big enough for most cases. Avoiding unexpected memory usage.
	// Start with a minCap, saving memory.
	minCap = 2
	// MaxCap is the maximum capacity of Set.
	// The real max number of keys may be around 0.8~0.9 * MaxCap.
	MaxCap = defaultMaxCap
)

// New creates a new Set.
// cap is the set capacity at the beginning,
// Set will grow if no bucket to add until meet MaxCap.
//
// If cap is zero, using minCap.
func New(cap int) *Set {

	cap = int(nextPower2(uint64(cap)))

	if cap < minCap {
		cap = minCap
	}
	if cap > MaxCap {
		cap = MaxCap
	}

	cap = calcTableCap(cap)
	bkt0 := make([]uint64, cap, cap) // Create one table at the beginning.
	return &Set{
		status: createStatus(),
		cycle:  [2]unsafe.Pointer{unsafe.Pointer(&bkt0)},
	}
}

// Close closes Set and release the resource.
func (s *Set) Close() {
	s.close()
	atomic.StorePointer(&s.cycle[0], nil)
	atomic.StorePointer(&s.cycle[1], nil)
}

var (
	ErrIsClosed   = errors.New("is closed")
	ErrAddTooFast = errors.New("add too fast") // Cycle being caught up.
	ErrIsFull     = errors.New("set is full")
	ErrIsSealed   = errors.New("is sealed")
)

// Add adds key into Set.
// Return nil if succeed.
//
// P.S.:
// It's better to use only one goroutine to Add at the same time.
func (s *Set) Add(key uint64) error {

	if !s.IsRunning() {
		return ErrIsClosed
	}

	err := s.tryInsert(key, false)
	switch err {
	case ErrIsFull:
		if s.isScaling() {
			s.unlock()
			return ErrAddTooFast
		}

		// Last writable table is full, try to expand to new table.
		idx := s.getWritableIdx()
		p := atomic.LoadPointer(&s.cycle[idx])
		tbl := *(*[]uint64)(p)
		oc := backToOriginCap(len(tbl))
		if oc*2 > MaxCap {
			s.unlock()
			return ErrIsFull
		}

		next := idx ^ 1
		newTbl := make([]uint64, calcTableCap(oc*2))
		atomic.StorePointer(&s.cycle[next], unsafe.Pointer(&newTbl))
		s.setWritable(next)
		s.scale()
		_ = s.tryInsert(key, true) // First insert must be succeed.
		go s.expand(int(idx))
		s.unlock()
		return nil
	default:
		s.unlock()
		return err
	}
}

// TODO Restart restarts Set, replay scaling
func (s *Set) Restart() {

}

// GetUsage returns Set capacity & usage.
func (s *Set) GetUsage() (total, usage int) {
	return len(s.getWritableTable()), int(s.getCnt())
}

const (
	defaultShrinkRatio = 0.25
	// ShrinkRatio is the avail_keys/total_capacity ratio,
	// when the ratio < ShrinkRatio, indicating Set may need to shrink.
	ShrinkRatio = defaultShrinkRatio
)

// TODO
// Shrink tries to shrink Set if could.
func (s *Set) Shrink() {

}

func (s *Set) expand(ri int) {
	rp := atomic.LoadPointer(&s.cycle[ri])
	src := *(*[]uint64)(rp)

	n, cnt := len(src), 0
	for i := range src {

		if !s.IsRunning() {
			return
		}

		if cnt >= 10 {
			cnt = 0
			runtime.Gosched()
		}
		s.lock()
		v := atomic.LoadUint64(&src[i])
		if v != 0 {
			err := s.tryInsert(v, true)
			if err == ErrIsFull {
				s.seal()
				s.unlock()
				return
			}
			cnt++
		}
		if i == n-1 { // Last one is finished.
			atomic.StorePointer(&s.cycle[ri], unsafe.Pointer(nil))
			s.unScale()
		}
		s.unlock()
	}
}

// TODO Contains logic:
// 1. VPBBROADCASTQ 8byte->32byte 1
// 2. VPCMPEQQ	2
// 3. VPTEST Y0, Y0	3
//
//
// Contains returns the key in set or not.
func (s *Set) Contains(key uint64) bool {

	if key == 0 {
		return s.hasZero()
	}

	sa := atomic.LoadUint64(&s.status)

	// 1. Search writable table first.
	idx, tbl, slot := s.getTblSlot(sa, key)
	if tbl != nil {
		slotCnt := len(tbl)
		n := neighbour
		if slot+neighbour >= slotCnt {
			n = slotCnt - slot
		}
		for i := 0; i < n; i++ {
			k := atomic.LoadUint64(&tbl[slot+i])
			if k == key {
				return true
			}
		}
		// if contains(key, tbl, slot, n) {
		// 	return true
		// }
	}

	// 2. If is scaling, searching next table.
	if !s.isScaling() {
		return false
	}
	next := idx ^ 1
	tbl, slot = s.getTblSlotByIdx(next, key)
	if tbl == nil {
		return false
	}
	slotCnt := len(tbl)
	n := neighbour
	if slot+neighbour >= slotCnt {
		n = slotCnt - slot
	}
	return contains(key, tbl, slot, n)
}

var contains = containsGeneric

func containsGeneric(key uint64, tbl []uint64, slot, n int) bool {
	for i := 0; i < n; i++ {
		k := atomic.LoadUint64(&tbl[slot+i])
		if k == key {
			return true
		}
	}
	return false
}

// Remove removes key in Set.
func (s *Set) Remove(key uint64) {
	if !s.IsRunning() {
		return
	}
	_ = s.tryRemove(key)
}

func (s *Set) tryRemove(key uint64) (err error) {

	defer func() {
		if err == nil {
			s.delCnt()
		}
	}()

restart:

	if !s.lock() {
		pause()
		goto restart
	}

	if s.isSealed() {
		return ErrIsSealed
	}

	if key == 0 {
		s.delZero()
		return nil
	}

	idx := s.getWritableIdx()

	p := atomic.LoadPointer(&s.cycle[idx])
	tbl := *(*[]uint64)(p)
	slotCnt := len(tbl)
	mask := calcMask(uint32(slotCnt))

	h := calcHash(idx, key)

	slot := int(h & mask)

	// 1. Ensure key is unique.
	slotOff := neighbour // slotOff is the distance between avail slot from hashed slot.
	for i := 0; i < neighbour && slot+i < slotCnt; i++ {
		k := atomic.LoadUint64(&tbl[slot+i])
		if k == key {
			return nil // If already contains, do nothing.
		}
		if k == 0 {
			if i < slotOff {
				slotOff = i
			}
			continue // Go on trying to find the same key.
		}
	}

	// 2. Try to Add within neighbour
	if slotOff < neighbour {
		atomic.StoreUint64(&tbl[slot+slotOff], key)
		return nil
	}

	// 3. Linear probe to find an empty slot and swap.
	j := slot + neighbour
	for { // Closer and closer.
		free, status := s.swap(j, slotCnt, tbl, idx)
		if status == swapFull {
			return ErrIsFull
		}

		if free-slot < neighbour {
			atomic.StoreUint64(&tbl[free], key)
			return nil
		}
		j = free
	}
}

// Remove removes key in set.
//func (s *Set) Remove(key uint64) {
//
//	bkt := uint64(digest) & bktMask
//
//	for i := 0; i < neighbour && i+int(bkt) < bktCnt; i++ {
//
//		entry := atomic.LoadUint64(&s.cycle[bkt+uint64(i)])
//		if entry>>digestShift&keyMask == uint64(digest) {
//			deleted := entry >> deletedShift & deletedMask
//			if deleted == 1 { // Deleted.
//				return
//			}
//			a := uint64(1) << deletedShift
//			entry = entry | a
//			atomic.StoreUint64(&s.cycle[bkt+uint64(i)], entry)
//		}
//	}
//}

func (s *Set) tryInsert(key uint64, isLocked bool) (err error) {

	defer func() {
		if err == nil {
			s.addCnt()
		}
	}()

restart:

	if !isLocked {
		if !s.lock() {
			pause()
			goto restart
		}
	}

	if s.isSealed() {
		return ErrIsSealed
	}

	if key == 0 {
		s.addZero()
		return nil
	}

	idx := s.getWritableIdx()

	p := atomic.LoadPointer(&s.cycle[idx])
	tbl := *(*[]uint64)(p)
	slotCnt := len(tbl)
	mask := calcMask(uint32(slotCnt))

	h := calcHash(idx, key)

	slot := int(h & mask)

	// 1. Ensure key is unique.
	slotOff := neighbour // slotOff is the distance between avail slot from hashed slot.
	for i := 0; i < neighbour && slot+i < slotCnt; i++ {
		k := atomic.LoadUint64(&tbl[slot+i])
		if k == key {
			return nil // If already contains, do nothing.
		}
		if k == 0 {
			if i < slotOff {
				slotOff = i
			}
			continue // Go on trying to find the same key.
		}
	}

	// 2. Try to Add within neighbour
	if slotOff < neighbour {
		atomic.StoreUint64(&tbl[slot+slotOff], key)
		return nil
	}

	// 3. Linear probe to find an empty slot and swap.
	j := slot + neighbour
	for { // Closer and closer.
		free, status := s.swap(j, slotCnt, tbl, idx)
		if status == swapFull {
			return ErrIsFull
		}

		if free-slot < neighbour {
			atomic.StoreUint64(&tbl[free], key)
			return nil
		}
		j = free
	}
}

const (
	swapOK = iota
	swapFull
)

// swap swaps the free slot and the another one (closer to the hashed slot).
// Return position & swapOK if find one.
func (s *Set) swap(start, slotCnt int, tbl []uint64, idx uint8) (int, uint8) {

	mask := calcMask(uint32(slotCnt))
	for i := start; i < slotCnt; i++ {
		if atomic.LoadUint64(&tbl[i]) == 0 { // Find a free one.
			j := i - neighbour + 1
			if j < 0 {
				j = 0
			}
			for ; j < i; j++ { // Search start at the closet position.
				k := atomic.LoadUint64(&tbl[j])
				slot := int(calcHash(idx, k) & mask)
				if i-slot < neighbour {
					atomic.StoreUint64(&tbl[i], k)
					atomic.StoreUint64(&tbl[j], 0)
					return j, swapOK
				}
			}
			return 0, swapFull // Can't find slot for swapping. Table is full.
		}
	}
	return 0, swapFull
}

//
// // List lists all keys in set.
// func (s *Set) List() []uint64 {
//
// }
