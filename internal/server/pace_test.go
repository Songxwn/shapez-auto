package server

import (
	"testing"
	"time"
)

func TestSlowProgress(t *testing.T) {
	if slowProgress(false, 100, 0, 0, 10*time.Second, 0) {
		t.Fatal("too soon")
	}
	if slowProgress(false, 100, 0, 0, 25*time.Second, 3) {
		t.Fatal("retry cap")
	}
	if !slowProgress(false, 100, 0, 0, 25*time.Second, 0) {
		t.Fatal("stuck count goal")
	}
	if slowProgress(false, 40, 0, 30, 25*time.Second, 0) {
		t.Fatal("count goal is fast enough")
	}
	if !slowProgress(false, 1000, 0, 2, 25*time.Second, 0) {
		t.Fatal("count goal eta is too long")
	}
	if !slowProgress(true, 10, 0, 2, 25*time.Second, 1) {
		t.Fatal("throughput is far below the goal")
	}
	if slowProgress(true, 10, 0, 8, 25*time.Second, 0) {
		t.Fatal("throughput is close enough")
	}
}
