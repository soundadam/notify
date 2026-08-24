package server

import (
	"testing"
	"time"
)

func TestWindowReserveAndForget(t *testing.T) {
	t.Parallel()
	w := NewWindow(50 * time.Millisecond)
	if w.Reserve("k") {
		t.Fatal("first reserve should not be cooldown")
	}
	if !w.Reserve("k") {
		t.Fatal("second reserve should be cooldown")
	}
	w.Forget("k")
	if w.Reserve("k") {
		t.Fatal("after forget, reserve should not be cooldown")
	}
}

func TestWindowExpiry(t *testing.T) {
	t.Parallel()
	w := NewWindow(20 * time.Millisecond)
	if w.Reserve("k") {
		t.Fatal("first reserve should not be cooldown")
	}
	time.Sleep(30 * time.Millisecond)
	if w.Reserve("k") {
		t.Fatal("after ttl, reserve should not be cooldown")
	}
}
