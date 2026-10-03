package operation

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestNewDeadlineRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		parent  context.Context
		timeout time.Duration
	}{
		{name: "nil parent", timeout: time.Second},
		{name: "zero timeout", parent: context.Background()},
		{name: "negative timeout", parent: context.Background(), timeout: -time.Second},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewDeadline(test.parent, test.timeout); !errors.Is(err, ErrInvalidDeadline) {
				t.Fatalf("NewDeadline() error = %v, want %v", err, ErrInvalidDeadline)
			}
		})
	}
}

func TestDeadlineExpiresAndCancelsWorker(t *testing.T) {
	t.Parallel()

	watchdog, err := NewDeadline(context.Background(), 20*time.Millisecond)
	if err != nil {
		t.Fatalf("NewDeadline() error = %v", err)
	}

	select {
	case <-watchdog.Context().Done():
	case <-time.After(time.Second):
		t.Fatal("worker context was not cancelled by deadline")
	}
	if cause := context.Cause(watchdog.Context()); !errors.Is(cause, context.DeadlineExceeded) {
		t.Fatalf("context cause = %v, want %v", cause, context.DeadlineExceeded)
	}
	if !watchdog.TimedOut() {
		t.Fatal("TimedOut() = false, want true")
	}
	if watchdog.Stop() {
		t.Fatal("Stop() = true after expiry, want false")
	}
}

func TestDeadlineStopIsIdempotent(t *testing.T) {
	t.Parallel()

	watchdog, err := NewDeadline(context.Background(), time.Hour)
	if err != nil {
		t.Fatalf("NewDeadline() error = %v", err)
	}
	if !watchdog.Stop() {
		t.Fatal("first Stop() = false, want true")
	}
	if watchdog.Stop() {
		t.Fatal("second Stop() = true, want false")
	}
	if watchdog.TimedOut() {
		t.Fatal("TimedOut() = true after Stop, want false")
	}
	if cause := context.Cause(watchdog.Context()); !errors.Is(cause, context.Canceled) {
		t.Fatalf("context cause = %v, want %v", cause, context.Canceled)
	}
}

func TestDeadlineParentCancellationIsNotTimeout(t *testing.T) {
	t.Parallel()

	parent, cancelParent := context.WithCancelCause(context.Background())
	watchdog, err := NewDeadline(parent, time.Hour)
	if err != nil {
		t.Fatalf("NewDeadline() error = %v", err)
	}
	defer watchdog.Stop()

	parentCause := errors.New("server shutdown")
	cancelParent(parentCause)
	<-watchdog.Context().Done()
	if cause := context.Cause(watchdog.Context()); !errors.Is(cause, parentCause) {
		t.Fatalf("context cause = %v, want %v", cause, parentCause)
	}
	if watchdog.TimedOut() {
		t.Fatal("TimedOut() = true after parent cancellation, want false")
	}
}

func TestDeadlineConcurrentStop(t *testing.T) {
	t.Parallel()

	watchdog, err := NewDeadline(context.Background(), time.Hour)
	if err != nil {
		t.Fatalf("NewDeadline() error = %v", err)
	}

	const callers = 64
	results := make(chan bool, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- watchdog.Stop()
		}()
	}
	wg.Wait()
	close(results)

	stopped := 0
	for result := range results {
		if result {
			stopped++
		}
	}
	if stopped != 1 {
		t.Fatalf("successful Stop() calls = %d, want 1", stopped)
	}
}

func TestDeadlineTimeIsFixed(t *testing.T) {
	t.Parallel()

	before := time.Now()
	watchdog, err := NewDeadline(context.Background(), time.Hour)
	if err != nil {
		t.Fatalf("NewDeadline() error = %v", err)
	}
	defer watchdog.Stop()
	after := time.Now()

	wantEarliest := before.Add(time.Hour)
	wantLatest := after.Add(time.Hour)
	if got := watchdog.Time(); got.Before(wantEarliest) || got.After(wantLatest) {
		t.Fatalf("Time() = %v, want between %v and %v", got, wantEarliest, wantLatest)
	}
	if got := watchdog.Time(); !got.Equal(watchdog.Time()) {
		t.Fatalf("Time() changed: first %v, second %v", got, watchdog.Time())
	}
	contextDeadline, ok := watchdog.Context().Deadline()
	if !ok {
		t.Fatal("Context().Deadline() is not set")
	}
	if !contextDeadline.Equal(watchdog.Time()) {
		t.Fatalf("Context().Deadline() = %v, want %v", contextDeadline, watchdog.Time())
	}
}
