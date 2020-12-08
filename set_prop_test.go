package u64

import "testing"

// TODO when reach the first full
func TestMitFull(t *testing.T) {
	n := 1024 * 256
	s := New(n)
	s.scale() // Forbidden expanding.
	cnt := 0
	for i := 0; i < n; i++ {
		err := s.Add(uint64(i))
		if err == ErrAddTooFast {
			t.Logf("total: %d, first full: %d, load factor: %.2f", n, cnt, float64(cnt)/float64(n))
			break
		}
		cnt++
	}
}
