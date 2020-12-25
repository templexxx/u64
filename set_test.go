package u64

import (
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

// TODO test
// 1. delete check deleted
// 2. range
// 3.

// TODO range test (could steal from sync.Map)
