package operation_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/thomasweidner/flashgate-mcp/internal/operation"
)

func TestLeakSecurityExpiryAndTemporaryDataCleanup(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_800_000_000, 0)
	temporary := filepath.Join(t.TempDir(), "job-result")
	if err := os.WriteFile(temporary, []byte("sensitive result"), 0o600); err != nil {
		t.Fatal(err)
	}

	registry := operation.NewLeakRegistry()
	if err := registry.Register(operation.LeakRecord{
		ID:        "op_expired",
		Owner:     "principal-a",
		ExpiresAt: now,
		Cleanup: func(context.Context) error {
			return os.Remove(temporary)
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(operation.LeakRecord{
		ID:        "op_live",
		Owner:     "principal-a",
		ExpiresAt: now.Add(time.Nanosecond),
		Cleanup:   func(context.Context) error { return errors.New("live cleanup must not run") },
	}); err != nil {
		t.Fatal(err)
	}

	outcomes := registry.Sweep(context.Background(), now)
	if len(outcomes) != 1 || outcomes[0].ID != "op_expired" || !outcomes[0].Cleaned || outcomes[0].Error != nil {
		t.Fatalf("Sweep() outcomes = %+v", outcomes)
	}
	if _, err := os.Stat(temporary); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary result still accessible after expiry: %v", err)
	}
	if err := registry.CheckOwner("op_expired", "principal-a"); !errors.Is(err, operation.ErrUnknownJob) {
		t.Fatalf("expired handle remains usable: %v", err)
	}
	if err := registry.CheckOwner("op_live", "principal-a"); err != nil {
		t.Fatalf("unexpired handle rejected: %v", err)
	}
}

func TestLeakSecurityHandleOwnershipCannotRemoveAnotherPrincipalsJob(t *testing.T) {
	t.Parallel()

	registry := operation.NewLeakRegistry()
	if err := registry.Register(operation.LeakRecord{
		ID:        "op_opaque",
		Owner:     "principal-a",
		ExpiresAt: time.Now().Add(time.Hour),
		Cleanup:   func(context.Context) error { return nil },
	}); err != nil {
		t.Fatal(err)
	}

	if err := registry.CheckOwner("op_opaque", "principal-b"); !errors.Is(err, operation.ErrJobOwnerMismatch) {
		t.Fatalf("cross-owner lookup error = %v", err)
	}
	if err := registry.Remove("op_opaque", "principal-b"); !errors.Is(err, operation.ErrJobOwnerMismatch) {
		t.Fatalf("cross-owner removal error = %v", err)
	}
	if err := registry.CheckOwner("op_opaque", "principal-a"); err != nil {
		t.Fatalf("cross-owner removal changed owner access: %v", err)
	}
	metrics := registry.Metrics()
	if metrics.Active != 1 || metrics.OwnershipDenials != 2 {
		t.Fatalf("Metrics() = %+v", metrics)
	}
}

func TestLeakSecurityConcurrentSweepDoesNotDuplicateCleanup(t *testing.T) {
	t.Parallel()

	registry := operation.NewLeakRegistry()
	now := time.Now()
	started := make(chan struct{})
	release := make(chan struct{})
	firstDone := make(chan []operation.LeakOutcome)
	var cleanupCalls atomic.Int32
	if err := registry.Register(operation.LeakRecord{
		ID:        "op_expired",
		Owner:     "principal-a",
		ExpiresAt: now,
		Cleanup: func(context.Context) error {
			cleanupCalls.Add(1)
			close(started)
			<-release
			return nil
		},
	}); err != nil {
		t.Fatal(err)
	}

	go func() { firstDone <- registry.Sweep(context.Background(), now) }()
	<-started
	const competingSweeps = 32
	results := make(chan []operation.LeakOutcome, competingSweeps)
	for range competingSweeps {
		go func() { results <- registry.Sweep(context.Background(), now) }()
	}
	for range competingSweeps {
		if outcomes := <-results; len(outcomes) != 0 {
			t.Fatalf("competing Sweep() claimed cleanup: %+v", outcomes)
		}
	}
	close(release)
	if outcomes := <-firstDone; len(outcomes) != 1 || !outcomes[0].Cleaned {
		t.Fatalf("first Sweep() outcomes = %+v", outcomes)
	}
	if calls := cleanupCalls.Load(); calls != 1 {
		t.Fatalf("cleanup calls = %d, want 1", calls)
	}
}

func TestLeakSecurityFailedCleanupRetainsHandleForBoundedRetry(t *testing.T) {
	t.Parallel()

	registry := operation.NewLeakRegistry()
	now := time.Now()
	cleanupFailure := errors.New("bounded cleanup failure")
	var attempts atomic.Int32
	if err := registry.Register(operation.LeakRecord{
		ID:        "op_expired",
		Owner:     "principal-a",
		ExpiresAt: now,
		Cleanup: func(context.Context) error {
			if attempts.Add(1) == 1 {
				return cleanupFailure
			}
			return nil
		},
	}); err != nil {
		t.Fatal(err)
	}

	first := registry.Sweep(context.Background(), now)
	if len(first) != 1 || first[0].Cleaned || !errors.Is(first[0].Error, cleanupFailure) {
		t.Fatalf("first Sweep() outcomes = %+v", first)
	}
	if err := registry.CheckOwner("op_expired", "principal-a"); err != nil {
		t.Fatalf("failed cleanup lost owner-bound retry state: %v", err)
	}
	metrics := registry.Metrics()
	if metrics.Active != 1 || metrics.Leaked != 1 || metrics.CleanupFailures != 1 {
		t.Fatalf("Metrics() after cleanup failure = %+v", metrics)
	}

	second := registry.Sweep(context.Background(), now)
	if len(second) != 1 || !second[0].Cleaned || second[0].Error != nil {
		t.Fatalf("second Sweep() outcomes = %+v", second)
	}
	if err := registry.CheckOwner("op_expired", "principal-a"); !errors.Is(err, operation.ErrUnknownJob) {
		t.Fatalf("cleaned handle remains usable: %v", err)
	}
}
