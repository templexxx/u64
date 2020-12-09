package u64

import "testing"

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
