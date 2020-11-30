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

	"github.com/templexxx/cpu"
	"github.com/zeebo/xxh3"
)

const (
	defaultShrinkRatio    = 0.25
	defaultShrinkInterval = 2 * time.Minute
	// defaultMaxCap is the default maximum capacity of Set.
	defaultMaxCap = 2 ^ 25 // 32Mi * 8 Byte = 256MB, big enough for most cases. Avoiding unexpected memory usage.
)

var (
	// ShrinkRatio is the avail_keys/total_capacity ratio,
	// when the ratio < ShrinkRatio, indicating Set may need to shrink.
	ShrinkRatio = defaultShrinkRatio
	// ShrinkDuration is the minimum duration between last add and Set shrink.
	// When now - last_add > ShrinkDuration & avail_keys/total_capacity < ShrinkRatio & it's the writable slice,
	// shrink will happen.
	ShrinkDuration = defaultShrinkInterval
	// MaxCap is the maximum capacity of Set.
	// The real max number of keys may be around 0.8 * MaxCap.
	MaxCap = defaultMaxCap
)

// neighbour is the hopscotch hash neighbourhood size.
//
// P is the probability a hopscotch hash table with load factor 0.75
// and the neighborhood size 32 must be rehashed:
// 3.81e-40 < P < 1e-4
//
// If there is no place to set key, try to resize to another bucket until meet MaxCap.
const (
	neighbour = 32
)

// Set is unsigned 64-bit integer set.
// Lock-free Write & Wait-free Read.
type Set struct {
	// add_status struct(uint64):
	// 64                       0
	// <-------------------------
	// | cnt(32) | last_add(32) |
	//
	// cnt: count of added keys.
	// last_add: timestamp of last add.
	addStatus uint64
	// _padding here for avoiding false share.
	// cycle which under the addStatus won't be modified frequently, but read frequently.
	_padding [cpu.X86FalseSharingRange]byte

	// cycle is the container of tables,
	// it's made of two uint64 slices,
	// only the cycle[0] is writable at a certain time.
	cycle [2]unsafe.Pointer
}

const (
	// Start with a minCap,
	// saving memory:
	// total usage = Set struct + Set pointer + minCap * 8bytes = 152 + 8 + minCap * 8 = 176bytes
	minCap = 2
)

// New creates a new Set.
// cap is the set capacity at the beginning,
// Set will grow if no bucket to add until meet MaxCap.
//
// If cap is zero, using minCap.
func New(cap int) *Set {

	if cap < minCap {
		cap = minCap
	}
	if cap > MaxCap {
		cap = MaxCap
	}

	cap = int(nextPower2(uint64(cap)))

	bkt0 := make([]uint64, cap, cap) // Create one bucket at the beginning.

	return &Set{
		cycle: [2]unsafe.Pointer{unsafe.Pointer(&bkt0)},
	}
}

func nextPower2(n uint64) uint64 {
	if n <= 1 {
		return 1
	}

	return 1 << (64 - bits.LeadingZeros64(n-1)) // TODO may use BSR instruction.
}

const (
	// epoch is an Unix time.
	// 2020-06-03T08:39:34.000+0800.
	epoch     int64 = 1591144774
	epochNano       = epoch * int64(time.Second)
	// doom is the zai's max Unix time.
	// It will reach the end after 136 years from epoch.
	doom int64 = 5880040774 // epoch + 136 years (about 2^32 seconds).
	// maxTS is the zai's max timestamp.
	maxTS = uint32(doom - epoch)
)

// Add adds key into Set.
// Return nil if succeed.
//
// Warning:
// There must be only one goroutine tries to Add at the same time
// (both of insert and Remove must use the same goroutine).
func (s *Set) Add(key uint64) error {
	return s.tryInsert(key)
}

func (s *Set) Contains(key uint64) bool {

	return false
}

func (s *Set) Remove(key uint64) {

}

var (
	ErrNoNeigh  = errors.New("no neighbour for insertion")
	ErrIsFull   = errors.New("set is full")
	ErrNotFound = errors.New("not found")
)

var hashFunc = func(k uint64) uint64 {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, k)
	return xxh3.Hash(b)
}

func (s *Set) tryInsert(key uint64) (err error) {

restart:
	p := atomic.LoadPointer(&s.cycle[0])
	bkts := *(*[]uint64)(p)
	bktCnt := len(bkts)
	mask := uint64(len(bkts) - 1)

	h := hashFunc(key)

	bkt := int(h & mask)

	// 1. Ensure key is unique.
	bktOff := neighbour // bktOff is the distance between avail bucket from bkt.
	// TODO use SIMD
	for i := 0; i < neighbour && bkt+i < bktCnt; i++ {
		entry := atomic.LoadUint64(&bkts[bkt+i])
		if entry == key {
			return nil // If already contains, do nothing.
		}
		if entry == 0 {
			if i < bktOff {
				bktOff = i
			}
			continue
		}
	}

	// 2. Try to Add within neighbour
	if bktOff < neighbour { // There is bktOff bucket within neighbour.
		if !atomic.CompareAndSwapUint64(&bkts[bkt+bktOff], 0, key) {
			goto restart
		}
		return nil
	}

	// 3. Linear probe to find an empty bucket and swap.
	j := bkt + neighbour
	for {
		free, ok := s.swap(j, bktCnt)
		if !ok {
			return ErrIsFull
		}
		if free-bkt < neighbour {
			entry := (free-bkt)<<neighOffShift | digest<<digestShift | addr
			atomic.StoreUint64(&s.cycle[free], entry)
			return nil
		}
		j = free
	}
}

// swap exchanges the free bucket and the another one (within the neighbourhood with the bucket we want).
// Return true if find one.
func (s *Set) swap(start, bktCnt int, bkts []uint64) (int, bool) {

	for i := start; i < bktCnt; i++ {
		if atomic.LoadUint64(&bkts[i]) == 0 { // Find a free one.
			for j := i - neighbour + 1; j < i; j++ { // Search forward.
				entry := atomic.LoadUint64(&bkts[j])
				if entry>>neighOffShift&neighOffMask+i-j < neighbour {
					atomic.StoreUint64(&s.cycle[i], entry)
					atomic.StoreUint64(&s.cycle[j], 0)

					return j, true
				}
			}
			return 0, false // Can't find bucket for swapping. Table is full.
		}
	}
	return 0, false
}

// TODO Contains logic:
// 1. VPBBROADCASTQ 8byte->32byte 1
// 2. VPCMPEQQ	2
// 3. VPTEST Y0, Y0	3
//
//
// // Has returns the key in set or not.
// // There are multi goroutines try to Has.
// func (s *Set) Has(key uint64) bool {
//
// 	bkt := uint64(digest) & bktMask
//
// 	for i := 0; i < neighbour && i+int(bkt) < bktCnt; i++ {
//
// 		entry := atomic.LoadUint64(&s.cycle[bkt+uint64(i)])
//
// 		if entry>>digestShift&keyMask == uint64(digest) {
// 			deleted := entry >> deletedShift & deletedMask
// 			if deleted == 1 { // Deleted.
// 				return 0, ErrNotFound
// 			}
// 			// entry maybe modified after atomic load.
// 			// Check it after read from disk.
// 			return uint32(entry & addrMask), nil
// 		}
// 	}
//
// 	return 0, ErrNotFound
// }
//
// // Remove removes key in set.
// func (s *Set) Remove(key uint64) {
// 	bkt := uint64(digest) & bktMask
//
// 	for i := 0; i < neighbour && i+int(bkt) < bktCnt; i++ {
//
// 		entry := atomic.LoadUint64(&s.cycle[bkt+uint64(i)])
// 		if entry>>digestShift&keyMask == uint64(digest) {
// 			deleted := entry >> deletedShift & deletedMask
// 			if deleted == 1 { // Deleted.
// 				return
// 			}
// 			a := uint64(1) << deletedShift
// 			entry = entry | a
// 			atomic.StoreUint64(&s.cycle[bkt+uint64(i)], entry)
// 		}
// 	}
// }
//
// // List lists all keys in set.
// func (s *Set) List() []uint64 {
//
// }
