package u64

import (
	"sync"
	"testing"
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

		_, usage := s.GetUsage()
		if usage != len(keys) {
			t.Fatal("usage mismatched")
		}

		for _, key := range keys {
			if !s.Contains(key) {
				t.Fatal("should have key")
			}
		}
	}
}

func TestSet_Remove(t *testing.T) {

	start := 2
	for n := start; n <= MaxCap; n *= 32 {
		keys := generateKeys(n/2, randomKey)
		s := New(n)
		for _, key := range keys {
			err := s.Add(key)
			if err != nil {
				t.Fatal(err)
			}
			s.Remove(key)
			if s.Contains(key) {
				t.Fatal("should not have key", n, key)
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

	n := 1024 * 4
	s := New(n)
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

// TODO test
// 1. concurrent range (sync.Map)
// 2. expand background
