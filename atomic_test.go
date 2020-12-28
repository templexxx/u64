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
