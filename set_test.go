package u64

import (
	"testing"

	"github.com/templexxx/tsc"
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

func TestIndexSearchPerf(t *testing.T) {

	n := 1024 * 1024
	s := New(n * 2)
	exp := n
	for i := 1; i < n+1; i++ { // TODO add a flag has 0
		err := s.Add(uint64(i))
		if err != nil {
			exp--
		}
	}

	start := tsc.UnixNano()
	has := 0
	for i := 1; i < n+1; i++ {
		if s.Contains(uint64(i)) {
			has++
		}
	}
	//for j := 0; j < 10; j++ {
	//	for i := 1; i < n+1; i++ {
	//		s.Contains(uint64(i))
	//	}
	//}

	if has != exp {
		t.Fatal("contains mismatch", has, exp)
	}
	end := tsc.UnixNano()
	ops := float64(end-start) / float64(exp)
	t.Logf("index search perf: %.2f ns/op, total: %d, failed: %d, ok rate: %.8f", ops, n, n-exp, float64(exp)/float64(n))
}
