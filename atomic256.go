package u64

import (
	"github.com/templexxx/cpu"
)

var isAtomic256 = false

var isAtomic256CPUs = map[string]struct{}{
	"06_4EH": {}, "06_5EH": {},
	"06_55H": {},
	"06_6AH": {}, "06_6CH": {},
	"06_8EH": {}, "06_9EH": {},
	"06_66H": {},
	"06_A5H": {}, "06_A6H": {},
	"06_7DH": {}, "06_7EH": {},
}

func init() {
	if _, ok := isAtomic256CPUs[cpu.X86.Signature]; ok {
		isAtomic256 = true
		contains = containsAtomic256
	}
}

func containsAtomic256(key uint64, tbl []uint64, slot, n int) bool {
	return containsGeneric(key, tbl, slot, n)
	// if n < 8 {
	// 	return containsGeneric(key, tbl, slot, n)
	// }
	// return containsAVX(key, &tbl[slot], n)
}

//go:noescape
func containsAVX(key uint64, tbl *uint64, n int) bool
