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
	"encoding/binary"
	"errors"
	"math/bits"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/cespare/xxhash/v2"
	"github.com/zeebo/xxh3"
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
const neighbour = 32

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

var ErrIsClosed = errors.New("is closed")

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

	err := s.tryInsert(key)
	switch err {
	case ErrIsFull:
		idx, tCap, ok := s.couldScale()
		if !ok {
			return ErrIsFull
		}
		if tCap*2 > MaxCap {
			return ErrIsFull
		}
		next := idx ^ 1
		newTbl := make([]uint64, tCap*2)
		atomic.StorePointer(&s.cycle[next], unsafe.Pointer(&newTbl))
		_ = s.tryInsert(key) // First insert can't fail.
		go s.expand(idx)
		return nil
	default:
		return err
	}
}

//func (s *Set) expand(ri int) {
//	rp := atomic.LoadPointer(&s.cycle[ri])
//	src := *(*[]uint64)(rp)
//	for i := range src {
//		v := atomic.LoadUint64(&src[i])
//		if v != 0 {
//			s.Lock()
//			err := s.tryInsert(v)
//			if err == ErrIsFull {
//				s.Unlock()
//				return
//			}
//			s.Unlock()
//		}
//	}
//	s.release(ri)
//}

// TODO Contains logic:
// 1. VPBBROADCASTQ 8byte->32byte 1
// 2. VPCMPEQQ	2
// 3. VPTEST Y0, Y0	3
//
//
// Contains returns the key in set or not.
func (s *Set) Contains(key uint64) bool {

	p0 := atomic.LoadPointer(&s.cycle[0])
	tbl0 := *(*[]uint64)(p0)

	if s.searchTbl(key, tbl0) {
		return true
	}
	p1 := atomic.LoadPointer(&s.cycle[1])
	if p1 != nil {
		tbl1 := *(*[]uint64)(p1)
		if s.searchTbl(key, tbl1) {
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
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, k)
	return xxh3.Hash(b) // xxh3 is prefect bijective for 8bytes and blazing fast.
}

// hash function for cycle[1]
var hashFunc1 = func(k uint64) uint64 {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, k)
	return xxhash.Sum64(b) // xxhash is prefect bijective for 8bytes and blazing fast.
}

var ErrIsSealed = errors.New("is sealed")

func (s *Set) tryInsert(key uint64) (err error) {

	defer func() {
		if err == nil {
			s.addCnt()
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
	}

	// 3. Linear probe to find an empty slot and swap.
	j := slot + neighbour
	for { // Closer and closer.
		free, status := s.swap(j, slotCnt, tbl)
		if status == swapFull {
			return ErrIsFull
		}
		if status == swapCASFailed {
			goto restart
		}
		if free-slot < neighbour {
			atomic.StoreUint64(&tbl[free], key)
		}
		j = free
	}
}

const (
	swapOK = iota
	swapFull
	swapCASFailed
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
