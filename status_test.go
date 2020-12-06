package u64

import "testing"

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
	if s.getWritableTable() != 0 {
		t.Fatal("writable table mismatched")
	}
}

func TestSetWritable(t *testing.T) {

	s := New(0)
	s.setWritable(1)
	if s.getWritableTable() != 1 {
		t.Fatal("writable table mismatched")
	}
	s.setWritable(0)
	if s.getWritableTable() != 0 {
		t.Fatal("writable table mismatched")
	}
}
