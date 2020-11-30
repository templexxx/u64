package u64

import (
	"errors"
	"math/bits"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/templexxx/cpu"
	"github.com/zeebo/xxh3"
)

// TODO how to shrink space? (GC)

// hash function for bucket0.
var hashFunc0 = func(b []byte) uint64 {
	return xxh3.Hash(b) // xxh3 is prefect bijective for 8bytes and blazing fast.
}

// hash function for bucket1.
var hashFunc1 = func(b []byte) uint64 {
	return xxhash.Sum64(b) // xxhash is prefect bijective for 8bytes and blazing fast.
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

// neighbour is the hopscotch hash neighbour size.
// loadFactor is the hash load factor.
//
// P is the probability a hopscotch hash table with load factor 0.75
// and the neighborhood size 64 must be rehashed:
// 7.95e-98 < P < 1e-8
//
// If there is no place to set key, try to resize to another bucket.
const (
	neighbour  = 64
	loadFactor = 0.75
)

// TODO block until expand/shrink finishing.
// Set is unsigned 64-bit integer set.
// Supports one write goroutine and multi read goroutines at the same time.
// Read is wait-free.
type Set struct {
	// add_status struct(uint64):
	// 64                       0
	// <-------------------------
	// | cnt(32) | last_add(32) |
	//
	// cnt: count of available keys
	// last_add: timestamp of last add
	addStatus uint64
	// There is only one padding for avoiding false share.
	// The memory area which needs to reduce cache pollution around it should have its own false sharing range,
	// this padding just has responsibility of taking care of the cache next to it.
	// Only one padding could save 128byte memory usage.
	_padding [cpu.X86FalseSharingRange]byte

	// cycle is the container of entries,
	// it's made of two uint64 slices,
	// only one slice is writable at a certain time.
	//
	// It won't change frequently, no need to put a padding to avoid false sharing.
	cycle [2]*[]uint64
}

const (
	// Start with a minCap,
	// saving memory:
	// total usage = Set struct + Set pointer + minCap * 8bytes = 152 + 8 + minCap * 8 = 176bytes
	minCap = 2
)

// TODO aligned to 64bytes(cache line)
// TODO no shrink option

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

	h := uint64(n) << 0
	bkt0 := make([]uint64, n) // Create one bucket at the beginning.

	return &Set{
		cycleStatus: h,
		cycle:       [2][]uint64{bkt0},
	}
}
func nextPower2(n uint64) uint64 {
	if n <= 1 {
		return 1
	}

	return 1 << (64 - bits.LeadingZeros64(n-1)) // TODO may use BSR instruction.
}

// Add inserts entry to index.
// Return nil if succeed.
//
// There will be only one goroutine tries to insert.
// (both of insert and Remove use the same goroutine)
func (s *Set) Add(key uint64) error {
	return nil
	// return s.tryAdd(key)
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

// // tryAdd tries to add entry to specific bucket.
// func (s *Set) tryAdd(key uint64) (err error) {
//
// 	bkt := digest & bktMask
//
// 	// 1. Ensure digest is unique.
// 	bktOff := neighbour // Bucket offset: free_bucket - hash_bucket.
//
// 	// TODO use SIMD
// 	for i := 0; i < neighbour && bkt+uint64(i) < bktCnt; i++ {
// 		entry := atomic.LoadUint64(&s.cycle[bkt+uint64(i)])
// 		if entry == 0 {
// 			if i < bktOff {
// 				bktOff = i
// 			}
// 			continue
// 		}
// 		d := entry >> digestShift & keyMask
// 		if d == digest {
// 			if insertOnly {
// 				return ErrNoNeigh
// 			} else {
// 				bktOff = i
// 				break
// 			}
// 		}
// 	}
//
// 	// 2. Try to Add within neighbour
// 	if bktOff < neighbour { // There is bktOff bucket within neighbour.
// 		entry := uint64(bktOff)<<neighOffShift | digest<<digestShift | addr
// 		atomic.StoreUint64(&s.cycle[bkt+uint64(bktOff)], entry)
// 		return nil
// 	}
//
// 	// 3. Linear probe to find an empty bucket and swap.
// 	j := bkt + neighbour
// 	for {
// 		free, ok := s.exchange(j)
// 		if !ok {
// 			return ErrIsFull
// 		}
// 		if free-bkt < neighbour {
// 			entry := (free-bkt)<<neighOffShift | digest<<digestShift | addr
// 			atomic.StoreUint64(&s.cycle[free], entry)
// 			return nil
// 		}
// 		j = free
// 	}
// }
//
// // exchange exchanges the empty slot and the another one (closer to the bucket we want).
// func (s *Set) exchange(start uint64) (uint64, bool) {
//
// 	for i := start; i < bktCnt; i++ {
// 		if atomic.LoadUint64(&s.cycle[i]) == 0 { // Find a free one.
// 			for j := i - neighbour + 1; j < i; j++ { // Search forward.
// 				entry := atomic.LoadUint64(&s.cycle[j])
// 				if entry>>neighOffShift&neighOffMask+i-j < neighbour {
// 					atomic.StoreUint64(&s.cycle[i], entry)
// 					atomic.StoreUint64(&s.cycle[j], 0)
//
// 					return j, true
// 				}
// 			}
// 			return 0, false // Can't find bucket for swapping. Table is full.
// 		}
// 	}
// 	return 0, false
// }

// TODO Contains logic:
// 1. VPBBROADCASTQ 8byte->32byte
// 2. VPCMPEQQ
// 3. VPTEST Y0, Y0
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
