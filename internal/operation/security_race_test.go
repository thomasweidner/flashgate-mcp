package operation

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestLeakRegistryConcurrentSecurityBoundary exercises the owner boundary and
// expiry path together. It is intentionally useful under go test -race: every
// record is attacked, checked by its owner, and swept by competing reapers.
func TestLeakRegistryConcurrentSecurityBoundary(t *testing.T) {
	const jobCount = 64

	registry := NewLeakRegistry()
	now := time.Unix(100, 0)
	cleanupCalls := make([]atomic.Int32, jobCount)
	for i := range jobCount {
		id := fmt.Sprintf("job-%03d", i)
		if err := registry.Register(LeakRecord{
			ID:        id,
			Owner:     "owner",
			ExpiresAt: now,
			Cleanup: func(context.Context) error {
				cleanupCalls[i].Add(1)
				return nil
			},
		}); err != nil {
			t.Fatalf("Register(%q) error = %v", id, err)
		}
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range jobCount {
		id := fmt.Sprintf("job-%03d", i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if err := registry.CheckOwner(id, "attacker"); !errors.Is(err, ErrJobOwnerMismatch) {
				t.Errorf("CheckOwner(%q, attacker) error = %v", id, err)
			}
			if err := registry.Remove(id, "attacker"); !errors.Is(err, ErrJobOwnerMismatch) {
				t.Errorf("Remove(%q, attacker) error = %v", id, err)
			}
			if err := registry.CheckOwner(id, "owner"); err != nil {
				t.Errorf("CheckOwner(%q, owner) error = %v", id, err)
			}
		}()
	}
	close(start)
	wg.Wait()

	// All owner checks complete before expiry processing so an unknown record
	// cannot be mistaken for a successful authorization check.
	const sweepers = 16
	outcomes := make(chan []LeakOutcome, sweepers)
	for range sweepers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			outcomes <- registry.Sweep(context.Background(), now)
		}()
	}
	wg.Wait()
	close(outcomes)

	cleaned := 0
	for result := range outcomes {
		cleaned += len(result)
		for _, outcome := range result {
			if !outcome.Cleaned || outcome.Error != nil {
				t.Errorf("cleanup outcome = %+v", outcome)
			}
		}
	}
	if cleaned != jobCount {
		t.Fatalf("cleaned outcomes = %d, want %d", cleaned, jobCount)
	}
	for i := range jobCount {
		if calls := cleanupCalls[i].Load(); calls != 1 {
			t.Errorf("cleanup calls for job-%03d = %d, want 1", i, calls)
		}
	}

	metrics := registry.Metrics()
	if metrics.Registered != jobCount || metrics.Active != 0 || metrics.ExpiredDetected != jobCount || metrics.Cleaned != jobCount || metrics.Leaked != 0 {
		t.Errorf("lifecycle metrics = %+v", metrics)
	}
	if metrics.OwnershipDenials != 2*jobCount {
		t.Errorf("ownership denials = %d, want %d", metrics.OwnershipDenials, 2*jobCount)
	}
}

func TestLeakRegistryFailedCleanupDoesNotLeakSweepClaim(t *testing.T) {
	registry := NewLeakRegistry()
	now := time.Unix(100, 0)
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	var calls atomic.Int32
	wantErr := errors.New("temporary cleanup failure")
	if err := registry.Register(LeakRecord{
		ID:        "job",
		Owner:     "owner",
		ExpiresAt: now,
		Cleanup: func(context.Context) error {
			if calls.Add(1) == 1 {
				close(firstStarted)
				<-releaseFirst
				return wantErr
			}
			return nil
		},
	}); err != nil {
		t.Fatal(err)
	}

	firstDone := make(chan []LeakOutcome, 1)
	go func() { firstDone <- registry.Sweep(context.Background(), now) }()
	<-firstStarted

	// A claimed cleanup is invisible to another sweeper rather than executed
	// twice. Once the failure releases the claim, a later sweep may retry it.
	if result := registry.Sweep(context.Background(), now); len(result) != 0 {
		t.Fatalf("competing sweep result = %+v", result)
	}
	close(releaseFirst)
	first := <-firstDone
	if len(first) != 1 || !errors.Is(first[0].Error, wantErr) || first[0].Cleaned {
		t.Fatalf("first sweep result = %+v", first)
	}
	second := registry.Sweep(context.Background(), now)
	if len(second) != 1 || !second[0].Cleaned || second[0].Error != nil {
		t.Fatalf("retry sweep result = %+v", second)
	}
	if calls.Load() != 2 {
		t.Fatalf("cleanup calls = %d, want 2", calls.Load())
	}
	if metrics := registry.Metrics(); metrics.Active != 0 || metrics.Leaked != 0 || metrics.ExpiredDetected != 1 || metrics.CleanupFailures != 1 || metrics.Cleaned != 1 {
		t.Fatalf("metrics = %+v", metrics)
	}
}
