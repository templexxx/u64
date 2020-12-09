package u64

import (
	"sync/atomic"
	"testing"
)

func TestNextPower2(t *testing.T) {
	for i := 0; i <= 1025; i++ {
		p := nextPower2(uint64(i))
		if p != slowNextPower2(uint64(i)) {
			t.Fatal("power2 mismatch", p, i)
		}
	}
}

func slowNextPower2(n uint64) uint64 {
	var p uint64 = 1
	for {
		if p < n {
			p *= 2
		} else {
			break
		}
	}
	return p
}

func BenchmarkSet_getTblSlot(b *testing.B) {
	s := New(1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = s.getTblSlot(atomic.LoadUint64(&s.status), uint64(i))
	}
}
