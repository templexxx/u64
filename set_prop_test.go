package u64

import (
	"testing"
)

// TODO when reach the first full
func TestMitFull(t *testing.T) {
	n := 1024 * 1024
	s := New(n)
	s.scale() // Forbidden expanding.
	cnt := 0
	for i := 0; i < n; i++ {
		// TODO try to use a pseudo-random number
		// TODO two version, seq number & random number
		err := s.Add(uint64(i))
		if err == ErrAddTooFast {
			t.Logf("total: %d, first full: %d, load factor: %.2f", n, cnt, float64(cnt)/float64(n))
			break
		}
		cnt++
	}
}
