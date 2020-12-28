package u64

import "github.com/templexxx/cpu"

var isAtomic256 = false

var atomic256CPUs = map[string]struct{}{
	"06_4EH": {}, "06_5EH": {},
	"06_55H": {},
	"06_6AH": {}, "06_6CH": {},
	"06_8EH": {}, "06_9EH": {},
	"06_66H": {},
	"06_A5H": {}, "06_A6H": {},
	"06_7DH": {}, "06_7EH": {},
}

func init() {
	if _, ok := atomic256CPUs[cpu.X86.Signature]; ok {
		isAtomic256 = true
	}
	// TODO after tuning avx version faster
	isAtomic256 = true
}

//go:noescape
func containsAVX(key uint64, tbl *uint64, n int) (has bool)

func alignSize(n int64, align int64) int64 {
	return (n + align - 1) &^ (align - 1)
}

//go:noescape
func alignTo(n int64) int64
