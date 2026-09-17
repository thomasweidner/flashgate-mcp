package operation_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/thomasweidner/flashgate-mcp/internal/operation"
)

// TestLifecycleIntegration exercises the public operation contracts with real
// goroutines, contexts, and filesystem resources. It intentionally uses only
// portable Go and filesystem APIs so the same suite runs on Windows and Linux.
func TestLifecycleIntegration(t *testing.T) {
	t.Run("caller cancellation cleans temporary resources", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		resource := newTemporaryResource(t)
		registry := registryForResource(t, resource)
		done := startCleanupWorker(ctx, registry)

		cancel()
		waitForWorker(t, done)
		assertResourceRemoved(t, resource)
		assertCleanupMetrics(t, registry)
	})

	t.Run("server deadline cleans temporary resources", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		resource := newTemporaryResource(t)
		registry := registryForResource(t, resource)
		done := startCleanupWorker(ctx, registry)

		waitForWorker(t, done)
		if !errors.Is(context.Cause(ctx), context.DeadlineExceeded) {
			t.Fatalf("context cause = %v, want deadline exceeded", context.Cause(ctx))
		}
		assertResourceRemoved(t, resource)
		assertCleanupMetrics(t, registry)
	})

	t.Run("shutdown cancellation cleans temporary resources", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		resource := newTemporaryResource(t)
		registry := registryForResource(t, resource)
		done := startCleanupWorker(ctx, registry)
		shutdown, err := operation.NewShutdown(operation.ShutdownConfig{
			GracePeriod: time.Second,
			ForcePeriod: time.Second,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := shutdown.Register(operation.ShutdownParticipant{
			ID: "integration-job", Cancel: cancel, Done: done,
		}); err != nil {
			t.Fatal(err)
		}

		report, err := shutdown.Run(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !report.Complete || len(report.Outcomes) != 1 || !report.Outcomes[0].Terminated || report.Outcomes[0].Forced {
			t.Fatalf("shutdown report = %+v", report)
		}
		assertResourceRemoved(t, resource)
		assertCleanupMetrics(t, registry)
	})
}

func newTemporaryResource(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "operation-partial.tmp")
	if err := os.WriteFile(path, []byte("partial result"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func registryForResource(t *testing.T, resource string) *operation.LeakRegistry {
	t.Helper()
	registry := operation.NewLeakRegistry()
	err := registry.Register(operation.LeakRecord{
		ID:        "integration-job",
		Owner:     "integration-principal",
		ExpiresAt: time.Unix(1, 0),
		Cleanup: func(context.Context) error {
			return os.Remove(resource)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func startCleanupWorker(ctx context.Context, registry *operation.LeakRegistry) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		<-ctx.Done()
		registry.Sweep(context.Background(), time.Now())
	}()
	return done
}

func waitForWorker(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("operation worker did not terminate")
	}
}

func assertResourceRemoved(t *testing.T, resource string) {
	t.Helper()
	if _, err := os.Stat(resource); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary resource stat error = %v, want not exist", err)
	}
}

func assertCleanupMetrics(t *testing.T, registry *operation.LeakRegistry) {
	t.Helper()
	metrics := registry.Metrics()
	if metrics.Registered != 1 || metrics.Active != 0 || metrics.ExpiredDetected != 1 || metrics.Cleaned != 1 || metrics.Leaked != 0 {
		t.Fatalf("cleanup metrics = %+v", metrics)
	}
}
