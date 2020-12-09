package u64

import (
	"fmt"
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
		err := s.Add(uint64(i * 2)) // i*2 in hash32 is 0.36, when n = 1024 * 1024.
		if err == ErrAddTooFast {
			slot := hash32(uint64(i*2), 0) & uint32(n-1)
			fmt.Println(i*2, slot)
			table := *(*[]uint64)(s.cycle[0])
			n2 := int(slot) + neighbour
			if int(slot)+neighbour > len(table) {
				n2 = len(table)
			}
			fmt.Println(table[slot-neighbour : slot])
			fmt.Println(table[slot:n2])
			t.Logf("total: %d, first full: %d, load factor: %.2f", n, cnt, float64(cnt)/float64(n))
			break
		}
		cnt++
	}
}
