package operation

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestCancellationCancelSignalsWorker(t *testing.T) {
	t.Parallel()

	cancellation := NewCancellation(context.Background())
	cancellation.Cancel()

	select {
	case <-cancellation.Context().Done():
	case <-time.After(time.Second):
		t.Fatal("operation context was not cancelled")
	}
	if !errors.Is(cancellation.Context().Err(), context.Canceled) {
		t.Fatalf("context error = %v, want context.Canceled", cancellation.Context().Err())
	}
}

func TestCancellationPropagatesParentCancellation(t *testing.T) {
	t.Parallel()

	parent, cancelParent := context.WithCancel(context.Background())
	cancellation := NewCancellation(parent)
	cancelParent()

	select {
	case <-cancellation.Context().Done():
	case <-time.After(time.Second):
		t.Fatal("parent cancellation was not propagated")
	}
}

func TestCancellationDoesNotCancelParent(t *testing.T) {
	t.Parallel()

	parent, cancelParent := context.WithCancel(context.Background())
	t.Cleanup(cancelParent)
	cancellation := NewCancellation(parent)
	cancellation.Cancel()

	select {
	case <-parent.Done():
		t.Fatal("operation cancellation propagated to its parent")
	default:
	}
}

func TestCancellationIsConcurrentAndIdempotent(t *testing.T) {
	t.Parallel()

	cancellation := NewCancellation(context.Background())
	const callers = 32
	var callersDone sync.WaitGroup
	callersDone.Add(callers)
	for range callers {
		go func() {
			defer callersDone.Done()
			cancellation.Cancel()
		}()
	}
	callersDone.Wait()

	if !errors.Is(cancellation.Context().Err(), context.Canceled) {
		t.Fatalf("context error = %v, want context.Canceled", cancellation.Context().Err())
	}
}
