package operation

import (
	"errors"
	"math"
	"sync"
	"testing"
)

func TestProgressTrackerZeroValueAndUpdates(t *testing.T) {
	var tracker ProgressTracker

	if got := tracker.Snapshot(); got != (ProgressSnapshot{}) {
		t.Fatalf("zero-value snapshot = %#v, want zero snapshot", got)
	}

	if err := tracker.AddBytes(ByteCounters{Read: 10, Written: 20, Scanned: 30}); err != nil {
		t.Fatalf("AddBytes() error = %v", err)
	}
	if err := tracker.AddBytes(ByteCounters{Read: 1, Scanned: 2}); err != nil {
		t.Fatalf("second AddBytes() error = %v", err)
	}
	if err := tracker.SetDomainProgress(DomainProgress{Completed: 3, Total: 5}); err != nil {
		t.Fatalf("SetDomainProgress() error = %v", err)
	}

	want := ProgressSnapshot{
		Bytes:  ByteCounters{Read: 11, Written: 20, Scanned: 32},
		Domain: DomainProgress{Completed: 3, Total: 5},
	}
	if got := tracker.Snapshot(); got != want {
		t.Fatalf("Snapshot() = %#v, want %#v", got, want)
	}
}

func TestProgressTrackerRejectsOverflowAtomically(t *testing.T) {
	var tracker ProgressTracker
	initial := ByteCounters{Read: math.MaxUint64, Written: 4, Scanned: 5}
	if err := tracker.AddBytes(initial); err != nil {
		t.Fatalf("initial AddBytes() error = %v", err)
	}

	err := tracker.AddBytes(ByteCounters{Read: 1, Written: 9, Scanned: 9})
	if !errors.Is(err, ErrCounterOverflow) {
		t.Fatalf("AddBytes() error = %v, want ErrCounterOverflow", err)
	}
	if got := tracker.Snapshot().Bytes; got != initial {
		t.Fatalf("overflow changed counters to %#v, want %#v", got, initial)
	}
}

func TestProgressTrackerValidatesDomainBound(t *testing.T) {
	var tracker ProgressTracker
	if err := tracker.SetDomainProgress(DomainProgress{Completed: 2, Total: 2}); err != nil {
		t.Fatalf("SetDomainProgress() error = %v", err)
	}

	err := tracker.SetDomainProgress(DomainProgress{Completed: 3, Total: 2})
	if !errors.Is(err, ErrInvalidProgress) {
		t.Fatalf("SetDomainProgress() error = %v, want ErrInvalidProgress", err)
	}
	if got := tracker.Snapshot().Domain; got != (DomainProgress{Completed: 2, Total: 2}) {
		t.Fatalf("invalid update changed progress to %#v", got)
	}

	if err := tracker.SetDomainProgress(DomainProgress{}); err != nil {
		t.Fatalf("clear progress error = %v", err)
	}
	if got := tracker.Snapshot().Domain; got != (DomainProgress{}) {
		t.Fatalf("cleared progress = %#v, want zero", got)
	}
}

func TestProgressTrackerConcurrentByteUpdates(t *testing.T) {
	var tracker ProgressTracker
	const workers = 64
	const updates = 100

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range updates {
				if err := tracker.AddBytes(ByteCounters{Read: 1, Written: 2, Scanned: 3}); err != nil {
					t.Errorf("AddBytes() error = %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()

	want := ByteCounters{Read: workers * updates, Written: 2 * workers * updates, Scanned: 3 * workers * updates}
	if got := tracker.Snapshot().Bytes; got != want {
		t.Fatalf("concurrent counters = %#v, want %#v", got, want)
	}
}
