// Key Concepts:
// 1. Slot:
// Entry container.
// Neighbourhood:
// Key could be found in slot which hashed to or next Neighbourhood - 1 slots.
// Bucket:
// It's a virtual struct made of neighbourhood slots.
// Table:
// An array of buckets.
package u64

import (
	"errors"
	"math/bits"
	"runtime"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/templexxx/xxh3"
)

const (
	defaultShrinkRatio    = 0.25
	defaultShrinkInterval = 2 * time.Minute
	// defaultMaxCap is the default maximum capacity of Set.
	defaultMaxCap = 1 << 25 // 32Mi * 8 Byte = 256MB, big enough for most cases. Avoiding unexpected memory usage.
)

var (
	// ShrinkRatio is the avail_keys/total_capacity ratio,
	// when the ratio < ShrinkRatio, indicating Set may need to shrink.
	// TODO implement ratio checking, too big ratio is meaningless.
	ShrinkRatio = defaultShrinkRatio
	// ShrinkDuration is the minimum duration between last add and Set shrink.
	// When now - last_add > ShrinkDuration & avail_keys/total_capacity < ShrinkRatio & it's the writable slice,
	// shrink will happen.
	// TODO implement duration checking, too big or too small duration is meaningless.
	ShrinkDuration = defaultShrinkInterval
)

// MaxCap is the maximum capacity of Set.
// The real max number of keys may be around 0.8~0.9 * MaxCap.
const MaxCap = defaultMaxCap

// neighbour is the hopscotch hash neighbourhood size.
//
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
	// TODO may don't need it because the the write operations aren't many, few cache miss is okay.
	// remove it could save 128 bytes, it's attractive for application which want to save every bit.
	// _padding [cpu.X86FalseSharingRange]byte

	// cycle is the container of tables,
	// it's made of two uint64 slices.
	// only the one could be inserted at a certain time.
	cycle [2]unsafe.Pointer
}

const (
	// Start with a minCap, saving memory.
	minCap = 2
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

	bkt0 := make([]uint64, cap, cap) // Create one bucket at the beginning.
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

func nextPower2(n uint64) uint64 {
	if n <= 1 {
		return 1
	}

	return 1 << (64 - bits.LeadingZeros64(n-1)) // TODO may use BSR instruction.
}

var (
	ErrIsClosed   = errors.New("is closed")
	ErrAddTooFast = errors.New("add too fast")
)

// Add adds key into Set.
// Return nil if succeed.
//
// Warning:
// There must be only one goroutine tries to Add at the same time
// (both of insert and Remove must use the same goroutine).
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

		// Try to expand.
		idx := s.getWritableTable()
		p := atomic.LoadPointer(&s.cycle[idx])
		tbl := *(*[]uint64)(p)
		if len(tbl)*2 > MaxCap {
			s.unlock()
			return ErrIsFull
		}

		next := idx ^ 1
		newTbl := make([]uint64, len(tbl)*2)
		atomic.StorePointer(&s.cycle[next], unsafe.Pointer(&newTbl))
		s.setWritable(next)
		s.scale()
		_ = s.tryInsert(key, true) // First insert can't fail.
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

// TODO GetUsage returns capacity & usage.
func (s *Set) GetUsage() {

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

	// 1. Search writable table first.
	idx := s.getWritableTable()
	p := atomic.LoadPointer(&s.cycle[idx])
	tbl := *(*[]uint64)(p)

	//if s.searchTbl(key, tbl) {
	//	return true
	//}
	h := getHash(idx, key)
	slotCnt := len(tbl)
	slot := int(h & uint64(len(tbl)-1))
	n := neighbour
	if slot+neighbour >= slotCnt {
		n = slotCnt - slot
	}
	if contains(key, &tbl[slot], uint8(n)) {
		return true
	}

	// 2. If is scaling, searching next table.
	next := idx ^ 1
	nextP := atomic.LoadPointer(&s.cycle[next])
	if nextP == nil {
		return false
	}
	// TODO replace with contains
	nextT := *(*[]uint64)(nextP)
	if s.searchTbl(key, nextT) {
		return true
	}

	return false
}

func getHash(idx uint8, key uint64) uint64 {
	if idx == 0 {
		return hashFunc0(key)
	}
	return hashFunc1(key)
}

func contains(key uint64, start *uint64, n uint8) bool {
	var i uint8
	for ; i < n; i++ {
		k := atomic.LoadUint64((*uint64)(unsafe.Pointer(uintptr(unsafe.Pointer(start)) + uintptr(i*8))))
		if k == key {
			return true
		}
	}
	return false
}

func (s *Set) searchTbl(key uint64, tbl []uint64) bool {

	h := hashFunc0(key)
	slotCnt := len(tbl)
	slot := int(h & uint64(len(tbl)-1))

	for i := 0; i < neighbour && i+slot < slotCnt; i++ {
		k := atomic.LoadUint64(&tbl[slot+i])
		if k == key {
			return true
		}
	}

	return false
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

var (
	ErrIsFull   = errors.New("set is full")
	ErrNotFound = errors.New("not found")
)

// TODO check escape
// hash function for cycle[0]
var hashFunc0 = func(k uint64) uint64 {
	return uint64(farm32(k, 0))
	//return xxh3.HashU64(k, 0) // xxh3 is prefect bijective for 8bytes and blazing fast.
}

func farm32(fid uint64, seed uint32) uint32 {
	var a, b, c, d uint32
	a = 8
	b = 40
	c = 9
	d = b + seed
	a += uint32(fid << 32 >> 32)
	b += uint32(fid >> 32)
	c += uint32(fid >> 32)
	return fmix(seed ^ mur(c, mur(b, mur(a, d))))
}

// Magic numbers for 32-bit hashing.  Copied from Murmur3.
const c1 uint32 = 0xcc9e2d51
const c2 uint32 = 0x1b873593

func mur(a, h uint32) uint32 {
	// Helper from Murmur3 for combining two 32-bit values.
	a *= c1
	a = bits.RotateLeft32(a, -17)
	a *= c2
	h ^= a
	h = bits.RotateLeft32(h, -19)
	return h*5 + 0xe6546b64
}

// A 32-bit to 32-bit integer hash copied from Murmur3.
func fmix(h uint32) uint32 {
	h ^= h >> 16
	h *= 0x85ebca6b
	h ^= h >> 13
	h *= 0xc2b2ae35
	h ^= h >> 16
	return h
}

// hash function for cycle[1]
var hashFunc1 = func(k uint64) uint64 {
	return xxh3.HashU64(k, 1)
}

var ErrIsSealed = errors.New("is sealed")

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

	idx := s.getWritableTable()

	p := atomic.LoadPointer(&s.cycle[idx])
	tbl := *(*[]uint64)(p)
	slotCnt := len(tbl)
	mask := uint64(len(tbl) - 1)

	var hashFunc = hashFunc0
	if idx == 1 {
		hashFunc = hashFunc1
	}

	h := hashFunc(key)

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
		free, status := s.swap(j, slotCnt, tbl)
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
func (s *Set) swap(start, bktCnt int, tbl []uint64) (int, uint8) {

	mask := uint64(len(tbl) - 1)
	for i := start; i < bktCnt; i++ {
		if atomic.LoadUint64(&tbl[i]) == 0 { // Find a free one.
			j := i - neighbour + 1
			if j < 0 {
				j = 0
			}
			for ; j < i; j++ { // Search start at the closet position.
				k := atomic.LoadUint64(&tbl[j])
				slot := int(hashFunc0(k) & mask)
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
