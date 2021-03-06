package u64

import (
	"testing"
)

func TestCalcCap(t *testing.T) {
	for i := 2; i <= MaxCap; i *= 2 {
		if backToOriginCap(calcTableCap(i)) != i {
			t.Fatal("calc cap mismatched")
		}
	}
}

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

	if !isAtomic256 {
		b.Skip(ErrUnsupported.Error())
	}

	s, err := New(1024)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = s.getTblSlot(uint64(i))
	}
}
