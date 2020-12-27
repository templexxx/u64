package u64

import (
	"sync/atomic"
)

// TODO shrink should be manually, because it's heavy & the last_add is too slow because need to get timestamp
// TODO add is_scaling flag
// TODO remove last_add

// status struct(uint64):
// 64                                                                                  58
// <------------------------------------------------------------------------------------
// | is_running(1) | locked(1) | sealed(1) | is_scaling(1) | writable(1) | has_zero(1) |
// 58                       0
// <------------------------
// | padding(26) | cnt(32) |
//
// is_running: [63], is running or not.
// locked: [62], is locked or not.
// sealed: [61], seal Set when there is an unexpected failure.
// is_scaling: [60], Set is expanding/shrinking.
// writable: [59], writable table index.
// has_zero: [58], has 0 as key or not.
// cnt: [0,32), count of added keys.

// IsRunning returns Set is running or not.
func (s *Set) IsRunning() bool {
	sa := atomic.LoadUint64(&s.status)
	return bitOne(sa, 63)
}

// close sets status closed.
func (s *Set) close() {
	sa := atomic.LoadUint64(&s.status)
	sa = clrBit(sa, 63)
	atomic.StoreUint64(&s.status, sa)
}

// lock tries to lock Set, return true if succeed.
func (s *Set) lock() bool {
	sa := atomic.LoadUint64(&s.status)
	if isLocked(sa) {
		return false // locked.
	}

	nsa := setBit(sa, 62)
	return atomic.CompareAndSwapUint64(&s.status, sa, nsa)
}

// unlock unlocks Set, Set must be locked.
func (s *Set) unlock() {
	sa := atomic.LoadUint64(&s.status)
	sa = clrBit(sa, 62)
	atomic.StoreUint64(&s.status, sa)
}

func isLocked(sa uint64) bool {
	return bitOne(sa, 62)
}

// create status when New a Set.
func createStatus() uint64 {

	return setBit(0, 63) // set isRunning.
}

// TODO how to deal with sealed. Should pause make bigger table and transfer all data
// seal seals Set.
// When there is no writable table setting Set sealed.
func (s *Set) seal() {
	sa := atomic.LoadUint64(&s.status)
	sa = setBit(sa, 61)
	atomic.StoreUint64(&s.status, sa)
}

// isSealed returns Set is sealed or not.
func (s *Set) isSealed() bool {
	sa := atomic.LoadUint64(&s.status)
	return bitOne(sa, 61)
}

// scale sets Set sealed.
// When Set is expanding/shrinking setting Set scaling.
func (s *Set) scale() {
	sa := atomic.LoadUint64(&s.status)
	sa = setBit(sa, 60)
	atomic.StoreUint64(&s.status, sa)
}

// isScaling returns Set is scaling or not.
func (s *Set) isScaling() bool {
	sa := atomic.LoadUint64(&s.status)
	return bitOne(sa, 60)
}

// unScale sets Set scalable.
func (s *Set) unScale() {
	sa := atomic.LoadUint64(&s.status)
	sa = clrBit(sa, 60)
	atomic.StoreUint64(&s.status, sa)
}

// getWritableIdx gets writable table in Set.
// 0 or 1.
func (s *Set) getWritableIdx() uint8 {
	sa := atomic.LoadUint64(&s.status)
	return uint8((sa >> 59) & 1)
}

func getWritableIdxByStatus(sa uint64) uint8 {
	return uint8((sa >> 59) & 1)
}

// setWritable sets writable table index.
func (s *Set) setWritable(idx uint8) {
	sa := atomic.LoadUint64(&s.status)
	if idx == 0 {
		sa = clrBit(sa, 59)
	} else {
		sa = setBit(sa, 59)
	}
	atomic.StoreUint64(&s.status, sa)
}

func (s *Set) addZero() {
	sa := atomic.LoadUint64(&s.status)
	sa = setBit(sa, 58)
	atomic.StoreUint64(&s.status, sa)
}

func (s *Set) removeZero() {
	sa := atomic.LoadUint64(&s.status)
	sa = clrBit(sa, 58)
	atomic.StoreUint64(&s.status, sa)
}

func (s *Set) hasZero() bool {
	sa := atomic.LoadUint64(&s.status)
	return bitOne(sa, 58)
}

// addCnt adds Set count.
func (s *Set) addCnt() {
	atomic.AddUint64(&s.status, 1) // cnt is the lowest bits, just +1.
}

// delCnt minutes Set count.
func (s *Set) delCnt() {
	atomic.AddUint64(&s.status, ^uint64(0))
}

const cntMask = (1 << 32) - 1

func (s *Set) getCnt() uint64 {
	sa := atomic.LoadUint64(&s.status)
	return sa & cntMask
}

// Set x[off] to 1.
func setBit(x uint64, off uint64) uint64 {
	x |= 1 << off
	return x
}

// Return x[off] is 1 or not.
func bitOne(x, off uint64) bool {
	return (x>>off)&1 == 1
}

// Set x[off] to 0.
func clrBit(x uint64, off uint64) uint64 {
	x &= ^(1 << off)
	return x
}
