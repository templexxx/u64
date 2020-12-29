// Key Concepts:
//
// 1. Slot:
// Entry container.
//
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
// 64 could reach high load factor(e.g. 0.9) and the performance is good.
//
// If there is no place to set key, try to resize to another bucket until meet MaxCap.
const neighbour = 64

const (
	// defaultMaxCap is the default maximum capacity of Set.
	defaultMaxCap = 1 << 25 // 32Mi * 8 Byte = 256MB, big enough for most cases. Avoiding unexpected memory usage.
	// Start with a minCap, saving memory.
	minCap = 2
	// MaxCap is the maximum capacity of Set.
	// The real max number of keys may be around 0.9 * MaxCap.
	MaxCap = defaultMaxCap
)

// Set is unsigned 64-bit integer set.
// Providing Lock-free Write & Wait-free Read.
type Set struct {
	// status is a set of flags of Set, see status.go for more details.
	status uint64
	// cycle is the container of tables,
	// it's made of two uint64 slices.
	// only the one could be inserted at a certain time.
	cycle [2]unsafe.Pointer
}

// New creates a new Set.
// cap is the set capacity at the beginning,
// Set will grow if no bucket to add until meet MaxCap.
//
// If cap is zero, using minCap.
func New(cap int) (*Set, error) {

	// if !isAtomic256 {
	// 	return nil, ErrUnsupported
	// }

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
	}, nil
}

// Close closes Set and release the resource.
func (s *Set) Close() {
	s.close()
	atomic.StorePointer(&s.cycle[0], nil)
	atomic.StorePointer(&s.cycle[1], nil)
}

var (
	ErrUnsupported = errors.New("unsupported platform, need AVX atomic supports")
	ErrIsClosed    = errors.New("is closed")
	ErrAddTooFast  = errors.New("add too fast") // Cycle being caught up.
	ErrIsFull      = errors.New("set is full")
	ErrIsSealed    = errors.New("is sealed")
	ErrExisted     = errors.New("existed")
)

// Add adds key into Set.
// Return nil if succeed.
//
// P.S.:
// It's better to use only one goroutine to Add at the same time,
// it'll be more friendly for optimistic lock used by Set.
func (s *Set) Add(key uint64) error {

	if !s.IsRunning() {
		return ErrIsClosed
	}

	err := s.tryAdd(key, false)
	switch err {

	case nil:
		if key != 0 {
			s.addCnt()
		}
		s.unlock()
		return nil
	case ErrExisted:
		s.unlock()
		return nil

	case ErrIsFull:
		if s.isScaling() {
			s.unlock()
			// In practice, it's rare to have such fast adding.
			// Which means the caller's speed if fast than 'sequential traverse'
			return ErrAddTooFast
		}

		// Last writable table is full, try to expand to new table.
		idx := s.getWritableIdx()
		p := atomic.LoadPointer(&s.cycle[idx])
		tbl := *(*[]uint64)(p)
		oc := backToOriginCap(len(tbl))
		if oc*2 > MaxCap {
			s.unlock()
			return ErrIsFull // Already MaxCap.
		}

		s.scale()
		next := idx ^ 1
		newTbl := make([]uint64, calcTableCap(oc*2))
		atomic.StorePointer(&s.cycle[next], unsafe.Pointer(&newTbl))
		s.setWritable(next)
		_ = s.tryAdd(key, true) // First insert must be succeed.
		go s.expand(int(idx))
		s.addCnt()
		s.unlock()
		return nil

	default:
		s.unlock()
		return err
	}
}

// Contains returns the key in set or not.
func (s *Set) Contains(key uint64) bool {

	if key == 0 {
		return s.hasZero()
	}

	widx := s.getWritableIdx()
	next := widx ^ 1
	wt := getTbl(s, int(widx))
	nt := getTbl(s, int(next))

	// 1. Search writable table first.
	slot := getSlot(widx, wt, key)
	if wt != nil {
		slotCnt := len(wt)
		n := neighbour
		if slot+neighbour >= slotCnt {
			n = slotCnt - slot
		}

		// if containsAVX(key, &wt[slot], n) {
		// 	return true
		// }

		for i := 0; i < n; i++ {
			k := atomic.LoadUint64(&wt[slot+i])
			if k == key {
				return true
			}
		}
	}

	// 2. If is scaling, searching next table.
	slot = getSlot(next, nt, key)
	if nt != nil {
		slotCnt := len(nt)
		n := neighbour
		if slot+neighbour >= slotCnt {
			n = slotCnt - slot
		}

		// if containsAVX(key, &nt[slot], n) {
		// 	return true
		// }

		for i := 0; i < n; i++ {
			k := atomic.LoadUint64(&nt[slot+i])
			if k == key {
				return true
			}
		}
	}
	return false
}

// GetUsage returns Set capacity & usage.
func (s *Set) GetUsage() (total, usage int) {
	total = 0
	tbl := s.getWritableTable()
	if tbl != nil { // In case.
		total = backToOriginCap(len(tbl))
	}
	return total, int(s.getCnt())
}

// Remove removes key in Set.
func (s *Set) Remove(key uint64) {
	if !s.IsRunning() {
		return
	}
	s.tryRemove(key)
}

// Range calls f sequentially for each key present in the Set.
// If f returns false, range stops the iteration.
//
// Range does not necessarily correspond to any consistent snapshot of the Set's
// contents: no key will be visited more than once, but if the value for any key
// is Added or Removed concurrently, Range may reflect any mapping for that key
// from any point during the Range call.
//
// Range may be O(N) with the number of elements in the set even if f returns
// false after a constant number of calls.
func (s *Set) Range(f func(key uint64) bool) {

	widx := s.getWritableIdx()
	wt := getTbl(s, int(widx))

	next := widx ^ 1
	nt := getTbl(s, int(next))

	if wt != nil {
		for i := len(wt) - 1; i >= 0; i-- { // DESC for avoiding visiting the same key twice caused by swap in Add process.

			k := atomic.LoadUint64(&wt[i])
			if k == 0 {
				continue
			}

			if !f(k) {
				break
			}
		}
	}

	if nt != nil {
		for i := len(nt) - 1; i >= 0; i-- {
			k := atomic.LoadUint64(&nt[i])
			if k == 0 {
				continue
			}

			if wt != nil {
				slot := getSlot(widx, wt, k)
				slotCnt := len(wt)
				n := neighbour
				if slot+neighbour >= slotCnt {
					n = slotCnt - slot
				}

				has := false
				for j := 0; j < n; j++ {
					wk := atomic.LoadUint64(&wt[slot+j])
					if k == wk {
						has = true
						break
					}
				}
				if has {
					continue
				}
			}

			if !f(k) {
				break
			}
		}
	}

	if s.hasZero() {
		if !f(0) {
			return
		}
	}
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
			runtime.Gosched() // Let potential 'func Add' run.
		}

	restart:
		if !s.lock() {
			pause()
			goto restart
		}

		k := atomic.LoadUint64(&src[i])
		if k != 0 {
			err := s.tryAdd(k, true)
			if err == ErrIsFull {
				s.seal()
				s.unlock()
				return
			}

			if err == ErrExisted {
				s.delCnt()
			}

			cnt++
		}
		if i == n-1 { // Last one is finished.
			atomic.StorePointer(&s.cycle[ri], unsafe.Pointer(nil))
			s.unScale()
			s.unlock()
			return
		}
		s.unlock()
	}
}

// getPosition gets key's position in tbl if has.
func getPosition(tbl []uint64, slot int, key uint64) (has bool, pos int) {
	if tbl != nil {
		slotCnt := len(tbl)
		n := neighbour
		if slot+neighbour >= slotCnt {
			n = slotCnt - slot
		}
		for i := 0; i < n; i++ {
			k := atomic.LoadUint64(&tbl[slot+i])
			if k == key {
				return true, slot + i
			}
		}
	}
	return false, 0
}

func (s *Set) tryRemove(key uint64) {

restart:

	if !s.lock() {
		pause()
		goto restart
	}

	if key == 0 {
		s.removeZero()
		s.unlock()
		return
	}

	idx, tbl, slot := s.getTblSlot(key)
	has, pos := getPosition(tbl, slot, key)
	if has {
		atomic.StoreUint64(&tbl[pos], 0)
		s.delCnt()
		s.unlock()
		return
	}

	tbl, slot = s.getTblSlotByIdx(idx, key)
	has, pos = getPosition(tbl, slot, key)
	if has {
		atomic.StoreUint64(&tbl[pos], 0)
		s.delCnt()
		s.unlock()
		return
	}
	s.unlock()
	return
}

func (s *Set) tryAdd(key uint64, isLocked bool) (err error) {

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
	tbl := getTbl(s, int(idx))

	// 1. Ensure key is unique. And try to find free slot within neighbourhood.
	slotOff := neighbour // slotOff is the distance between avail slot from hashed slot.
	slot := getSlot(idx, tbl, key)
	if tbl != nil {
		slotCnt := len(tbl)
		n := neighbour
		if slot+neighbour >= slotCnt {
			n = slotCnt - slot
		}
		for i := 0; i < n; i++ {
			k := atomic.LoadUint64(&tbl[slot+i])
			if k == key {
				return ErrExisted
			}
			if k == 0 && i < slotOff {
				slotOff = i
			}
		}
	}

	// 2. Try to Add within neighbour.
	if slotOff < neighbour {
		atomic.StoreUint64(&tbl[slot+slotOff], key)
		return nil
	}

	// 3. Linear probe to find an empty slot and swap.
	j := slot + neighbour
	for { // Closer and closer.
		free, status := s.swap(j, len(tbl), tbl, idx)
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
					atomic.StoreUint64(&tbl[j], 0)
					atomic.StoreUint64(&tbl[i], k)
					return j, swapOK
				}
			}
			return 0, swapFull // Can't find slot for swapping. Table is full.
		}
	}
	return 0, swapFull
}
