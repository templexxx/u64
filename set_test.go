package u64

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/templexxx/tsc"
)

func TestIndexSearchPerf(t *testing.T) {

	n := 1024 * 1024
	// m := make(map[uint64]struct{}, n)
	s := New(n * 2)
	exp := n
	for i := 1; i < n+1; i++ {
		err := s.Add(uint64(i))
		if err != nil {
			exp--
		}
		// m[uint64(i)] = struct{}{}
	}

	start := tsc.UnixNano()
	has := 0
	for i := 1; i < n+1; i++ {
		if s.Contains(uint64(i)) {
			has++
		}
		// if _, ok := m[uint64(i)]; ok {
		// 	has++
		// }
	}
	// for j := 0; j < 10; j++ {
	// 	for i := 1; i < n+1; i++ {
	// 		s.Contains(uint64(i))
	// 	}
	// }

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

func TestConcurrentPerf(t *testing.T) {
	n := 1024 * 1024
	s := New(n * 2)
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
