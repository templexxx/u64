package u64

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/templexxx/tsc"
)

func TestSet_AddZero(t *testing.T) {
	s := New(2)
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

func TestSet_Contains(t *testing.T) {

	start := 2
	for n := start; n <= MaxCap; n *= 32 {
		keys := generateKeys(n/2, randomKey)
		s := New(n)
		for _, key := range keys {
			err := s.Add(key)
			if err != nil {
				t.Fatal(err)
			}
			if !s.Contains(key) {
				t.Fatal("should have key")
			}
		}
		for _, key := range keys {
			if !s.Contains(key) {
				t.Fatal("should have key")
			}
		}
		_, usage := s.GetUsage()
		if usage != len(keys) {
			t.Fatal("usage size mismatched")
		}
	}
}

// TODO test
// 1. delete check deleted
// 2. check not contain not has
// 3.

func TestConcurrentPerf(t *testing.T) {
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
	start := tsc.UnixNano()
	for i := 0; i < gn; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < n; j++ {
				_ = s.Contains(uint64(j))
			}
		}()
	}
	wg.Wait()
	end := tsc.UnixNano()
	ops := float64(end-start) / float64(n*gn)
	iops := float64(n*gn) / (float64(end-start) / float64(time.Second))
	t.Logf("total op: %d, cost: %dns, thread: %d;"+
		"index search perf: %.2f ns/op, %.2f op/s", n*gn, end-start, gn, ops, iops)
}

func TestIndexSearchPerf(t *testing.T) {

	n := 1024 * 1024
	s := New(n * 2)
	exp := n
	for i := 1; i < n+1; i++ {
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

	if has != exp {
		t.Fatal("contains mismatch", has, exp, n)
	}
	end := tsc.UnixNano()
	ops := float64(end-start) / float64(exp)
	t.Logf("index search perf: %.2f ns/op, total: %d, failed: %d, ok rate: %.8f", ops, n, n-exp, float64(exp)/float64(n))
}

// TODO range test (could steal from sync.Map)
