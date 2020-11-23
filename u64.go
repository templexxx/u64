package u64

import (
	"errors"
	"sync/atomic"

	"github.com/templexxx/cpu"
)

const (
	maxEntries = 2 ^ 28 // 256M * 8 Byte = 2GB.
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

	// insertOnly is the Set global insert configuration.
	// When it's true, new object with the same hashing will be failed to insert.
	insertOnly bool

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
	// It's good enough, almost impossible.
	// If there is no place to set key, try to resize to another bucket.
	buckets [2][]uint64
}

// New creates a new Set.
// sizeHint gives the hint of set capacity.
func New(sizeHint int) *Set {

	size := sizeHint / 3 * 4 // load factor 0.75.
	size = size >> 6 << 6    // Multiple of 64(neighborhood size).

	if size < 256 {
		size = 256
	}
	if size > maxEntries {
		size = maxEntries
	}

	h := uint64(size) << 0
	bkt0 := make([]uint64, size)

	return &Set{
		header:  h,
		buckets: [2][]uint64{bkt0},
	}
}

// insert inserts entry to index.
// Return nil if succeed.
//
// There will be only one goroutine tries to insert.
// (both of insert and delete use the same goroutine)
func (ix *Set) insert(digest, addr uint32) error {

	return ix.tryInsert(uint64(digest), uint64(addr), ix.insertOnly)
}

var (
	ErrDigestConflict = errors.New("digest conflicted")
	ErrIndexFull      = errors.New("index is full")
	ErrNotFound       = errors.New("not found")
)

// tryInsert tries to insert entry to index.
// Set insertOnly false if you want to replace the older entry,
// it's useful in test and extent GC process.
func (ix *Set) tryInsert(digest, addr uint64, insertOnly bool) (err error) {

	bkt := digest & bktMask

	// 1. Ensure digest is unique.
	bktOff := neighbour // Bucket offset: free_bucket - hash_bucket.

	// TODO use SIMD
	for i := 0; i < neighbour && bkt+uint64(i) < bktCnt; i++ {
		entry := atomic.LoadUint64(&ix.buckets[bkt+uint64(i)])
		if entry == 0 {
			if i < bktOff {
				bktOff = i
			}
			continue
		}
		d := entry >> digestShift & keyMask
		if d == digest {
			if insertOnly {
				return ErrDigestConflict
			} else {
				bktOff = i
				break
			}
		}
	}

	// 2. Try to insert within neighbour
	if bktOff < neighbour { // There is bktOff bucket within neighbour.
		entry := uint64(bktOff)<<neighOffShift | digest<<digestShift | addr
		atomic.StoreUint64(&ix.buckets[bkt+uint64(bktOff)], entry)
		return nil
	}

	// 3. Linear probe to find an empty bucket and swap.
	j := bkt + neighbour
	for {
		free, ok := ix.exchange(j)
		if !ok {
			return ErrIndexFull
		}
		if free-bkt < neighbour {
			entry := (free-bkt)<<neighOffShift | digest<<digestShift | addr
			atomic.StoreUint64(&ix.buckets[free], entry)
			return nil
		}
		j = free
	}
}

// exchange exchanges the empty slot and the another one (closer to the bucket we want).
func (ix *Set) exchange(start uint64) (uint64, bool) {

	for i := start; i < bktCnt; i++ {
		if atomic.LoadUint64(&ix.buckets[i]) == 0 { // Find a free one.
			for j := i - neighbour + 1; j < i; j++ { // Search forward.
				entry := atomic.LoadUint64(&ix.buckets[j])
				if entry>>neighOffShift&neighOffMask+i-j < neighbour {
					atomic.StoreUint64(&ix.buckets[i], entry)
					atomic.StoreUint64(&ix.buckets[j], 0)

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
func (ix *Set) search(digest uint32) (addr uint32, err error) {

	bkt := uint64(digest) & bktMask

	for i := 0; i < neighbour && i+int(bkt) < bktCnt; i++ {

		entry := atomic.LoadUint64(&ix.buckets[bkt+uint64(i)])

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

func (ix *Set) delete(digest uint32) {
	bkt := uint64(digest) & bktMask

	for i := 0; i < neighbour && i+int(bkt) < bktCnt; i++ {

		entry := atomic.LoadUint64(&ix.buckets[bkt+uint64(i)])
		if entry>>digestShift&keyMask == uint64(digest) {
			deleted := entry >> deletedShift & deletedMask
			if deleted == 1 { // Deleted.
				return
			}
			a := uint64(1) << deletedShift
			entry = entry | a
			atomic.StoreUint64(&ix.buckets[bkt+uint64(i)], entry)
		}
	}
}
