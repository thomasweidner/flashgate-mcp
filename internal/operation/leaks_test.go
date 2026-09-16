package operation

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLeakRegistrySweepsExpiredJobsInStableOrder(t *testing.T) {
	registry := NewLeakRegistry()
	now := time.Unix(100, 0)
	var cleaned []string
	var mu sync.Mutex
	for _, record := range []LeakRecord{
		{ID: "later", Owner: "alice", ExpiresAt: now.Add(time.Second), Cleanup: func(context.Context) error { return nil }},
		{ID: "b", Owner: "alice", ExpiresAt: now, Cleanup: appendCleanup(&mu, &cleaned, "b")},
		{ID: "a", Owner: "bob", ExpiresAt: now.Add(-time.Second), Cleanup: appendCleanup(&mu, &cleaned, "a")},
	} {
		if err := registry.Register(record); err != nil {
			t.Fatal(err)
		}
	}
	outcomes := registry.Sweep(context.Background(), now)
	if len(outcomes) != 2 || outcomes[0].ID != "a" || outcomes[1].ID != "b" {
		t.Fatalf("outcomes = %+v", outcomes)
	}
	if len(cleaned) != 2 || cleaned[0] != "a" || cleaned[1] != "b" {
		t.Fatalf("cleanup order = %v", cleaned)
	}
	metrics := registry.Metrics()
	if metrics.Registered != 3 || metrics.Active != 1 || metrics.Sweeps != 1 || metrics.ExpiredDetected != 2 || metrics.Cleaned != 2 || metrics.Leaked != 0 {
		t.Fatalf("metrics = %+v", metrics)
	}
}

func TestLeakRegistryEnforcesOwnership(t *testing.T) {
	registry := NewLeakRegistry()
	record := LeakRecord{ID: "job", Owner: "alice", ExpiresAt: time.Now().Add(time.Hour), Cleanup: func(context.Context) error { return nil }}
	if err := registry.Register(record); err != nil {
		t.Fatal(err)
	}
	if err := registry.CheckOwner("job", "mallory"); !errors.Is(err, ErrJobOwnerMismatch) {
		t.Fatalf("CheckOwner() error = %v", err)
	}
	if err := registry.Remove("job", "mallory"); !errors.Is(err, ErrJobOwnerMismatch) {
		t.Fatalf("Remove() error = %v", err)
	}
	if err := registry.Remove("job", "alice"); err != nil {
		t.Fatal(err)
	}
	if err := registry.CheckOwner("job", "alice"); !errors.Is(err, ErrUnknownJob) {
		t.Fatalf("CheckOwner() after removal error = %v", err)
	}
	if metrics := registry.Metrics(); metrics.Active != 0 || metrics.OwnershipDenials != 2 {
		t.Fatalf("metrics = %+v", metrics)
	}
}

func TestLeakRegistryRetainsAndRetriesFailedCleanup(t *testing.T) {
	registry := NewLeakRegistry()
	now := time.Now()
	var calls atomic.Int32
	cleanupErr := errors.New("cleanup failed")
	if err := registry.Register(LeakRecord{ID: "job", Owner: "alice", ExpiresAt: now, Cleanup: func(context.Context) error {
		if calls.Add(1) == 1 {
			return cleanupErr
		}
		return nil
	}}); err != nil {
		t.Fatal(err)
	}
	first := registry.Sweep(context.Background(), now)
	if len(first) != 1 || !errors.Is(first[0].Error, cleanupErr) || first[0].Cleaned {
		t.Fatalf("first sweep = %+v", first)
	}
	if metrics := registry.Metrics(); metrics.Active != 1 || metrics.Leaked != 1 || metrics.ExpiredDetected != 1 || metrics.CleanupFailures != 1 {
		t.Fatalf("metrics after failure = %+v", metrics)
	}
	second := registry.Sweep(context.Background(), now)
	if len(second) != 1 || !second[0].Cleaned || second[0].Error != nil {
		t.Fatalf("second sweep = %+v", second)
	}
	if metrics := registry.Metrics(); metrics.Active != 0 || metrics.Leaked != 0 || metrics.ExpiredDetected != 1 || metrics.Cleaned != 1 {
		t.Fatalf("metrics after retry = %+v", metrics)
	}
}

func TestLeakRegistryContainsCleanupPanic(t *testing.T) {
	registry := NewLeakRegistry()
	now := time.Now()
	if err := registry.Register(LeakRecord{ID: "job", Owner: "alice", ExpiresAt: now, Cleanup: func(context.Context) error { panic("secret") }}); err != nil {
		t.Fatal(err)
	}
	outcomes := registry.Sweep(context.Background(), now)
	if len(outcomes) != 1 || !errors.Is(outcomes[0].Error, ErrCleanupPanic) {
		t.Fatalf("outcomes = %+v", outcomes)
	}
}

func TestLeakRegistryConcurrentSweepsClaimOnce(t *testing.T) {
	registry := NewLeakRegistry()
	now := time.Now()
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	if err := registry.Register(LeakRecord{ID: "job", Owner: "alice", ExpiresAt: now, Cleanup: func(context.Context) error {
		calls.Add(1)
		close(started)
		<-release
		return nil
	}}); err != nil {
		t.Fatal(err)
	}
	done := make(chan []LeakOutcome)
	go func() { done <- registry.Sweep(context.Background(), now) }()
	<-started
	if outcomes := registry.Sweep(context.Background(), now); len(outcomes) != 0 {
		t.Fatalf("concurrent outcomes = %+v", outcomes)
	}
	close(release)
	<-done
	if calls.Load() != 1 {
		t.Fatalf("cleanup calls = %d", calls.Load())
	}
}

func TestLeakRegistryRejectsInvalidAndDuplicateRecords(t *testing.T) {
	registry := NewLeakRegistry()
	if err := registry.Register(LeakRecord{}); !errors.Is(err, ErrInvalidLeakRecord) {
		t.Fatalf("Register(empty) error = %v", err)
	}
	record := LeakRecord{ID: "job", Owner: "alice", ExpiresAt: time.Now(), Cleanup: func(context.Context) error { return nil }}
	if err := registry.Register(record); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(record); !errors.Is(err, ErrDuplicateJob) {
		t.Fatalf("Register(duplicate) error = %v", err)
	}
}

func appendCleanup(mu *sync.Mutex, cleaned *[]string, id string) func(context.Context) error {
	return func(context.Context) error { mu.Lock(); defer mu.Unlock(); *cleaned = append(*cleaned, id); return nil }
}
