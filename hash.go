package u64

import "math/bits"

func calcHash(idx uint8, key uint64) uint32 {
	if idx == 0 {
		return hash32(key, 0)
	}
	return hash32(key, 1)
}

func hash32(key uint64, seed uint32) uint32 {
	var a, b, c, d uint32
	a = 8
	b = 40
	c = 9
	d = b + seed
	a += uint32(key << 32 >> 32)
	b += uint32(key >> 32)
	c += uint32(key >> 32)
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
