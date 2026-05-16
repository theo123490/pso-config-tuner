package controller

import (
	"testing"
	"time"
)

func TestIterBarrier_AllReport(t *testing.T) {
	ids := []string{"p1", "p2", "p3"}
	b := newIterBarrier(ids)

	for _, id := range ids {
		b.Report(id)
	}

	select {
	case <-b.readyCh:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("readyCh not closed after all particles reported")
	}
}

func TestIterBarrier_Idempotent(t *testing.T) {
	b := newIterBarrier([]string{"p1", "p2"})

	if b.Report("p1") {
		t.Fatal("first report should not be a duplicate")
	}
	if !b.Report("p1") {
		t.Fatal("second report should be a duplicate")
	}

	// only p1 reported — readyCh must still be open
	select {
	case <-b.readyCh:
		t.Fatal("readyCh closed prematurely")
	default:
	}
}

func TestIterBarrier_Unreported(t *testing.T) {
	ids := []string{"p1", "p2", "p3"}
	b := newIterBarrier(ids)
	b.Report("p2")

	unreported := b.Unreported(ids)
	if len(unreported) != 2 {
		t.Fatalf("want 2 unreported, got %d: %v", len(unreported), unreported)
	}
	for _, id := range unreported {
		if id == "p2" {
			t.Fatalf("p2 reported but listed as unreported")
		}
	}
}

func TestIterBarrier_Empty(t *testing.T) {
	b := newIterBarrier(nil)
	select {
	case <-b.readyCh:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("empty barrier readyCh should be closed immediately")
	}
}
