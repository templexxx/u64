package u64

import (
	"errors"
	"sync/atomic"

	"github.com/cespare/xxhash/v2"
	"github.com/templexxx/cpu"
	"github.com/zeebo/xxh3"
)

const (
	MaxEntries = 2 ^ 25 // 32M * 8 Byte = 256MB.
)

const (
	neighOffBits = 6
	keyBits      = 58
)

const (
	neighbour     = 1 << neighOffBits
	neighOffShift = keyBits
	keyMask       = 1<<keyBits - 1
	neighOffMask  = 1<<neighOffBits - 1
)

// hash function for bucket0.
var hashFunc0 = func(b []byte) uint64 {
	return xxh3.Hash(b) // xxh3 is prefect bijective for 8bytes and blazing fast.
}

// hash function for bucket1.
var hashFunc1 = func(b []byte) uint64 {
	return xxhash.Sum64(b) // xxhash is prefect bijective for 8bytes and blazing fast.
}

// Set is unsigned 64-bit integer set.
// Supports one write goroutine and multi read goroutines at the same time.
// Read is wait-free.
type Set struct {
	_padding0 [cpu.X86FalseSharingRange]byte
	// Header Struct(uint64):
	// 64                                          0
	// <--------------------------------------------
	// | bkt1_size(31) | bkt0_size(31) | status(2) |
	header    uint64
	_padding1 [cpu.X86FalseSharingRange]byte

	// buckets are the containers of entries.
	//
	// Entry Struct(uint64):
	// 64                       0
	// <-------------------------
	// | neigh_off(6) | key(58) |
	//
	// neigh_off: hopscotch hashing neighborhood offset
	// P is the probability a hopscotch hash table with load factor 0.75
	// and the neighborhood size 64 must be rehashed:
	// 7.95e-98 < P < 1e-8
	// It's good enough, almost impossible for MaxEntries.
	// If there is no place to set key, try to resize to another bucket.
	buckets [2][]uint64
}

// New creates a new Set.
// size gives the hint size of set capacity, it's maximum size of the set,
// insertion will grow the size if needed.
func New(size int) *Set {

	n := size / 3 * 4 // load factor 0.75.
	n = n >> 6 << 6   // Multiple of 64(neighborhood size).

	if n < 256 {
		n = 256
	}
	if n > MaxEntries {
		n = MaxEntries
	}

	h := uint64(n) << 0
	bkt0 := make([]uint64, n) // Create one bucket at the beginning.

	return &Set{
		header:  h,
		buckets: [2][]uint64{bkt0},
	}
}

// Insert inserts entry to index.
// Return nil if succeed.
//
// There will be only one goroutine tries to insert.
// (both of insert and delete use the same goroutine)
func (s *Set) Insert(key uint64) error {

	return s.tryInsert(key)
}

var (
	ErrNoNeigh  = errors.New("no neighbour for insertion")
	ErrIsFull   = errors.New("set is full")
	ErrNotFound = errors.New("not found")
)

// tryInsert tries to insert entry.
func (s *Set) tryInsert(key uint64) (err error) {

	bkt := digest & bktMask

	// 1. Ensure digest is unique.
	bktOff := neighbour // Bucket offset: free_bucket - hash_bucket.

	// TODO use SIMD
	for i := 0; i < neighbour && bkt+uint64(i) < bktCnt; i++ {
		entry := atomic.LoadUint64(&s.buckets[bkt+uint64(i)])
		if entry == 0 {
			if i < bktOff {
				bktOff = i
			}
			continue
		}
		d := entry >> digestShift & keyMask
		if d == digest {
			if insertOnly {
				return ErrNoNeigh
			} else {
				bktOff = i
				break
			}
		}
	}

	// 2. Try to Insert within neighbour
	if bktOff < neighbour { // There is bktOff bucket within neighbour.
		entry := uint64(bktOff)<<neighOffShift | digest<<digestShift | addr
		atomic.StoreUint64(&s.buckets[bkt+uint64(bktOff)], entry)
		return nil
	}

	// 3. Linear probe to find an empty bucket and swap.
	j := bkt + neighbour
	for {
		free, ok := s.exchange(j)
		if !ok {
			return ErrIsFull
		}
		if free-bkt < neighbour {
			entry := (free-bkt)<<neighOffShift | digest<<digestShift | addr
			atomic.StoreUint64(&s.buckets[free], entry)
			return nil
		}
		j = free
	}
}

// exchange exchanges the empty slot and the another one (closer to the bucket we want).
func (s *Set) exchange(start uint64) (uint64, bool) {

	for i := start; i < bktCnt; i++ {
		if atomic.LoadUint64(&s.buckets[i]) == 0 { // Find a free one.
			for j := i - neighbour + 1; j < i; j++ { // Search forward.
				entry := atomic.LoadUint64(&s.buckets[j])
				if entry>>neighOffShift&neighOffMask+i-j < neighbour {
					atomic.StoreUint64(&s.buckets[i], entry)
					atomic.StoreUint64(&s.buckets[j], 0)

					return j, true
				}
			}
			return 0, false // Can't find bucket for swapping. Table is full.
		}
	}
	return 0, false
}

// TODO add cuckoo filter.
// There are multi goroutines try to search.
func (s *Set) search(digest uint32) (addr uint32, err error) {

	bkt := uint64(digest) & bktMask

	for i := 0; i < neighbour && i+int(bkt) < bktCnt; i++ {

		entry := atomic.LoadUint64(&s.buckets[bkt+uint64(i)])

		if entry>>digestShift&keyMask == uint64(digest) {
			deleted := entry >> deletedShift & deletedMask
			if deleted == 1 { // Deleted.
				return 0, ErrNotFound
			}
			// entry maybe modified after atomic load.
			// Check it after read from disk.
			return uint32(entry & addrMask), nil
		}
	}

	return 0, ErrNotFound
}

func (s *Set) delete(digest uint32) {
	bkt := uint64(digest) & bktMask

	for i := 0; i < neighbour && i+int(bkt) < bktCnt; i++ {

		entry := atomic.LoadUint64(&s.buckets[bkt+uint64(i)])
		if entry>>digestShift&keyMask == uint64(digest) {
			deleted := entry >> deletedShift & deletedMask
			if deleted == 1 { // Deleted.
				return
			}
			a := uint64(1) << deletedShift
			entry = entry | a
			atomic.StoreUint64(&s.buckets[bkt+uint64(i)], entry)
		}
	}
}
