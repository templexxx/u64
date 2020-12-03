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

func TestCreateStatus(t *testing.T) {
	sa := createStatus()

}
