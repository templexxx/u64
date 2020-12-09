package u64

import (
	"fmt"
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
		fmt.Println(s.getWritableIdx(), s.isScaling())
		t.Fatal("contains mismatch", has, exp, n)
	}
	end := tsc.UnixNano()
	ops := float64(end-start) / float64(exp)
	t.Logf("index search perf: %.2f ns/op, total: %d, failed: %d, ok rate: %.8f", ops, n, n-exp, float64(exp)/float64(n))
}

func TestSet_AddZero(t *testing.T) {
	s := New(0)
	if s.Contains(0) {
		t.Fatal("should not have 0")
	}
	err := s.Add(0)
	if err != nil {
		t.Fatal(err)
	}
	if !s.Contains(0) {
		t.Fatal("should have 0")
	}
}
