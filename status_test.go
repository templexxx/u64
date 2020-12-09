package u64

import (
	"sync/atomic"
	"testing"
)

func TestSet_IsRunning(t *testing.T) {
	s := New(0)
	if !s.IsRunning() {
		t.Fatal("should be running")
	}
}

func TestSet_Close(t *testing.T) {
	s := New(0)
	s.Close()
	if s.IsRunning() {
		t.Fatal("should be closed")
	}
}

func TestCreateStatusWritable(t *testing.T) {
	s := New(0)
	if s.getWritableIdx() != 0 {
		t.Fatal("writable table mismatched")
	}
}

func TestSetWritable(t *testing.T) {

	s := New(0)
	s.setWritable(1)
	if s.getWritableIdx() != 1 {
		t.Fatal("writable table mismatched")
	}
	s.setWritable(0)
	if s.getWritableIdx() != 0 {
		t.Fatal("writable table mismatched")
	}
}

func TestSetLock(t *testing.T) {
	s := New(0)
	if !s.lock() {
		t.Fatal("lock should be succeed")
	}

	if s.lock() {
		t.Fatal("should be locked")
	}

	sa := atomic.LoadUint64(&s.status)
	if !isLocked(sa) {
		t.Fatal("should be locked")
	}

	s.unlock()

	sa = atomic.LoadUint64(&s.status)
	if isLocked(sa) {
		t.Fatal("should be unlocked")
	}
}

func TestSet_Seal(t *testing.T) {
	s := New(0)
	s.seal()
	if !s.isSealed() {
		t.Fatal("should be sealed")
	}
}

func TestSet_Scale(t *testing.T) {
	s := New(0)
	s.scale()
	if !s.isScaling() {
		t.Fatal("should be scaling")
	}
	s.unScale()
	if s.isScaling() {
		t.Fatal("should be scalable")
	}
}

func TestSet_Zero(t *testing.T) {
	s := New(0)
	if s.hasZero() {
		t.Fatal("should not have zero")
	}
	s.addZero()
	if !s.hasZero() {
		t.Fatal("should have zero")
	}
	s.delZero()
	if s.hasZero() {
		t.Fatal("should not have zero")
	}
}

func TestSet_Cnt(t *testing.T) {
	s := New(0)
	for i := 0; i < 1024; i++ {
		if s.getCnt() != uint64(i) {
			t.Fatal("add count mismatch")
		}
		s.addCnt()
	}

	for i := 1024; i > 0; i-- {
		if s.getCnt() != uint64(i) {
			t.Fatal("del count mismatch")
		}
		s.delCnt()
	}
}
