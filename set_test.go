package u64

import (
	"runtime"
	"sync"
	"testing"
)

func TestSet_AddZero(t *testing.T) {
	if !isAtomic256 {
		t.Skip(ErrUnsupported.Error())
	}
	s, _ := New(2)
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

	if !isAtomic256 {
		t.Skip(ErrUnsupported.Error())
	}

	start := 2
	for n := start; n <= MaxCap; n *= 32 {
		keys := generateKeys(n, randomKey)
		s, _ := New(n)

		wg := new(sync.WaitGroup) // Using sync.WaitGroup for ensuring the order.
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, key := range keys {
				err := s.Add(key)
				if err != nil {
					t.Fatal(err)
				}
				if !s.Contains(key) {
					t.Fatal("should have key")
				}
			}
		}()
		wg.Wait()

		for _, key := range keys {
			if !s.Contains(key) {
				t.Fatal("should have key")
			}
		}
	}
}

func TestSet_Remove(t *testing.T) {

	if !isAtomic256 {
		t.Skip(ErrUnsupported.Error())
	}

	start := 2
	for n := start; n <= MaxCap; n *= 32 {
		keys := generateKeys(n/2, randomKey)
		s, _ := New(n)
		for _, key := range keys {
			err := s.Add(key)
			if err != nil {
				t.Fatal(err)
			}
			s.Remove(key)
			if s.Contains(key) {
				t.Fatal("should not have key")
			}
		}
		for _, key := range keys {
			if s.Contains(key) {
				t.Fatal("should not have key")
			}
		}
		_, usage := s.GetUsage()
		if usage != 0 {
			t.Fatal("usage size mismatched")
		}
	}
}

// Add & Remove concurrently, checking dead lock or not.
func TestSet_UpdateConcurrent(t *testing.T) {

	if !isAtomic256 {
		t.Skip(ErrUnsupported.Error())
	}

	n := 1024 * 4
	s, _ := New(n)
	for i := 0; i < 1024; i++ {
		err := s.Add(uint64(i))
		if err != nil {
			t.Fatal(err)
		}
	}

	wg := new(sync.WaitGroup)
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 1024; i < 1024*2; i++ {
			err := s.Add(uint64(i))
			if err != nil {
				t.Fatal(err)
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 1024; i++ {
			s.Remove(uint64(i))
		}
	}()
	wg.Wait()

	_, usage := s.GetUsage()
	if usage != 1024 {
		t.Fatal("usage mismatched")
	}

	for i := 0; i < 1024; i++ {
		if s.Contains(uint64(i)) {
			t.Fatal("should not have key")
		}
	}
}

func TestSet_GetUsage(t *testing.T) {

	if !isAtomic256 {
		t.Skip(ErrUnsupported.Error())
	}

	n := 2048
	s, _ := New(n * 4)
	for j := 0; j < 16; j++ {
		for i := 1; i < n+1; i++ {
			err := s.Add(uint64(i))
			if err != nil {
				t.Fatal(err)
			}
		}
	}

	_, usage := s.GetUsage()
	if usage != n {
		t.Fatal("usage mismatched", usage)
	}

	cnt := 0
	for i := 1; i < (n+1)/2; i++ {
		cnt++
		s.Remove(uint64(i))
	}

	_, usage = s.GetUsage()
	if usage != n-(n+1)/2+1 {
		t.Fatal("usage mismatched")
	}
}

func TestSet_Range(t *testing.T) {

	if !isAtomic256 {
		t.Skip(ErrUnsupported.Error())
	}

	n := 1 << 12
	s, _ := New(n * 4)

	for i := 1; i <= n; i++ {
		err := s.Add(uint64(i))
		if err != nil {
			t.Fatal(err)
		}
	}

	seen := make(map[uint64]bool, n)
	s.Range(func(k uint64) bool {
		if seen[k] {
			t.Fatalf("Range visited key %v twice", k)
		}
		seen[k] = true
		return true
	})
	if len(seen) != n {
		t.Fatalf("Range visited %v elements of %v-element Map", len(seen), n)
	}
}

func TestSet_RangeWithExpand(t *testing.T) {

	if !isAtomic256 {
		t.Skip(ErrUnsupported.Error())
	}

	cnt := 1 << 13
	s, _ := New(cnt / 2) // Not enough capacity, must trigger expand.

	for i := 1; i <= cnt; i++ {
		err := s.Add(uint64(i))
		if err != nil {
			t.Fatal(err)
		}
	}

	iters := 1024
	for n := iters; n > 0; n-- {
		seen := make(map[uint64]bool, cnt)

		s.Range(func(k uint64) bool {

			if seen[k] {
				t.Fatalf("Range visited key %v twice", k)
			}
			seen[k] = true
			return true
		})
	}
}

func TestConcurrentRange(t *testing.T) {

	if !isAtomic256 {
		t.Skip(ErrUnsupported.Error())
	}

	const cnt = 1 << 12

	s, _ := New(cnt)
	for n := uint64(1); n <= cnt; n++ {
		err := s.Add(n)
		if err != nil {
			t.Fatal(err)
		}
	}

	done := make(chan struct{})
	var wg sync.WaitGroup
	defer func() {
		close(done)
		wg.Wait()
	}()
	for g := int64(runtime.GOMAXPROCS(0)); g > 0; g-- {
		wg.Add(1)
		go func(g int64) {
			defer wg.Done()
			for i := int64(0); ; i++ {
				select {
				case <-done:
					return
				default:
				}
				for n := uint64(1); n < cnt; n++ {
					err := s.Add(n)
					if err != nil {
						t.Fatal(err)
					}
				}
			}
		}(g)
	}

	iters := 1 << 10
	if testing.Short() {
		iters = 16
	}
	for n := iters; n > 0; n-- {
		seen := make(map[uint64]bool, cnt)

		s.Range(func(k uint64) bool {

			if seen[k] {
				t.Fatalf("Range visited key %v twice", k)
			}
			seen[k] = true
			return true
		})

		if n == 1 {
			if len(seen) != cnt {
				t.Logf("In last iter, Range visited %v elements of %v-element Map", len(seen), cnt)
			}
		}
	}
}
