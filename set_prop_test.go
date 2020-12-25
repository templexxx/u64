package u64

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"sync"
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

func TestContainsPerfConcurrent(t *testing.T) {

	if !IsPropEnabled() {
		t.Skip("skip perf testing")
	}

	n := 1024 * 1024
	s := New(n * 2) // Ensure there is enough space for Adding, avoiding scaling.
	for i := 0; i < n; i++ {
		err := s.Add(uint64(i))
		if err != nil {
			t.Fatal(err)
		}
	}

	gn := runtime.NumCPU()
	wg := new(sync.WaitGroup)
	wg.Add(gn)
	start := time.Now().UnixNano()
	for i := 0; i < gn; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < n; j++ {
				_ = s.Contains(uint64(j))
			}
		}()
	}
	wg.Wait()
	end := time.Now().UnixNano()
	ops := float64(end-start) / float64(n*gn)
	iops := float64(n*gn) / (float64(end-start) / float64(time.Second))
	t.Logf("total op: %d, cost: %dns, thread: %d;"+
		"index search perf: %.2f ns/op, %.2f op/s", n*gn, end-start, gn, ops, iops)
}

func TestContainsPerf(t *testing.T) {

	if !IsPropEnabled() {
		t.Skip("skip perf testing")
	}

	n := 1024 * 1024
	s := New(n * 2)
	exp := n
	for i := 1; i < n+1; i++ {
		err := s.Add(uint64(i))
		if err != nil {
			exp--
		}
	}

	start := time.Now().UnixNano()
	has := 0
	for i := 1; i < n+1; i++ {
		if s.Contains(uint64(i)) {
			has++
		}
	}

	if has != exp {
		t.Fatal("contains mismatch", has, exp, n)
	}
	end := time.Now().UnixNano()
	ops := float64(end-start) / float64(exp)
	t.Logf("index search perf: %.2f ns/op, total: %d, failed: %d, ok rate: %.8f", ops, n, n-exp, float64(exp)/float64(n))
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
