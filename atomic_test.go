package u64

import (
	"testing"
)

func TestContainsAVX2(t *testing.T) {

	cnt := 256
	tbl := generateKeys(cnt, randomKey)
	for _, key := range tbl {
		if !containsAVX(key, &tbl[0], 256) {
			t.Fatal("should have")
		}
	}

}

func TestAlignSize(t *testing.T) {
	var align int64 = 64
	var i int64
	for i = 1; i <= align; i++ {
		n := alignSize(i, align)
		if n != align {
			t.Fatal("align mismatch", n, i)
		}
		if n != alignTo(i) {
			t.Fatal("alignTo mismatch", alignTo(i), n, i)
		}
	}
	for i = align + 1; i < align*2; i++ {
		n := alignSize(i, align)
		if n != align*2 {
			t.Fatal("align mismatch")
		}
		if n != alignTo(i) {
			t.Fatal("alignTo mismatch")
		}
	}
}
