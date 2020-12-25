package u64

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"
)

func TestMain(m *testing.M) {

	flag.Parse()

	rand.Seed(time.Now().UnixNano())

	os.Exit(m.Run())
}

func TestMitFull(t *testing.T) {

	if !IsPropEnabled() {
		t.Skip("skip testing, because it may take too long time")
	}

	start := 64 * 1024 // Too small is meaningless.
	end := MaxCap

	sortRets := make(map[int]int)
	randRets := make(map[int]int)

	for n := start; n <= end; n *= 2 {
		ok := testMitFull(n, sortKey)
		sortRets[n] = ok

		ok = testMitFull(n, randomKey)
		randRets[n] = ok
	}

	printRets(sortRets, sortKey)
	printRets(randRets, randomKey)
}

func testMitFull(cnt, keyType int) int {
	s := New(cnt)
	s.scale()
	keys := generateKeys(cnt, keyType)
	for i, key := range keys {
		err := s.Add(key)
		if err == ErrAddTooFast {
			return i
		}
	}
	return cnt
}

const (
	sortKey = iota
	randomKey
)

func generateKeys(cnt, keyType int) []uint64 {
	keys := make([]uint64, cnt)
	for i := range keys {
		keys[i] = uint64(i + 1)
	}
	switch keyType {
	case randomKey:
		kk := rand.Perm(cnt)
		for i := range kk {
			keys[i] = uint64(kk[i])
		}
		return keys
	default:
		return keys
	}
}

func printRets(rets map[int]int, keyType int) {
	var avg, min, max float64
	min = 1
	max = 0
	var minN, maxN int
	for k, v := range rets {
		lf := float64(v) / float64(k)
		avg += lf
		if lf < min {
			min = lf
			minN = k
		}
		if lf > max {
			max = lf
			maxN = k
		}
	}
	avg = avg / float64(len(rets))

	fmt.Printf("keyType: %s, load_factor: avg: %.2f, min: %.2f(n: %d), max: %.2f(n: %d)\n",
		keyTypeToStr(keyType), avg, min, minN, max, maxN)
}

func keyTypeToStr(keyType int) string {
	switch keyType {
	case randomKey:
		return "random"
	default:
		return "order"
	}
}

var _propEnabled = flag.Bool("prop", false, "enable properties testing or not")

// IsPropEnabled returns enable properties testing or not.
// Default is false.
//
// e.g.
// no properties testing: go test -prop=false -v or go test -v
// run properties testing: go test -prop=true -v
func IsPropEnabled() bool {
	if !flag.Parsed() {
		flag.Parse()
	}

	return *_propEnabled
}
