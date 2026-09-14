package operation

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

func TestNewSchedulerRejectsInvalidLimits(t *testing.T) {
	for _, limits := range []QueueLimits{{}, {Global: 1}, {PerPrincipal: 1}, {Global: 1, PerPrincipal: 2}} {
		if _, err := NewScheduler(limits); err == nil {
			t.Fatalf("NewScheduler(%+v) succeeded, want error", limits)
		}
	}
}

func TestSchedulerRejectsInvalidItemsWithoutMutation(t *testing.T) {
	s := newTestScheduler(t, QueueLimits{Global: 2, PerPrincipal: 1})
	for _, item := range []QueueItem{
		{PrincipalID: "alice", Domain: "search"},
		{ID: "op_one", Domain: "search"},
		{ID: "op_one", PrincipalID: "alice"},
	} {
		if err := s.Enqueue(item); !errors.Is(err, ErrInvalidQueueItem) {
			t.Fatalf("Enqueue(%+v) error = %v, want ErrInvalidQueueItem", item, err)
		}
	}
	if global, principal := s.Len("alice"); global != 0 || principal != 0 {
		t.Fatalf("Len(alice) = (%d, %d), want (0, 0)", global, principal)
	}
}

func TestSchedulerEnforcesQueueCapsAtomically(t *testing.T) {
	s := newTestScheduler(t, QueueLimits{Global: 2, PerPrincipal: 1})
	mustEnqueue(t, s, QueueItem{ID: "op_a", PrincipalID: "alice", Domain: "search"})
	if err := s.Enqueue(QueueItem{ID: "op_b", PrincipalID: "alice", Domain: "filesystem"}); !errors.Is(err, ErrPrincipalQueueFull) {
		t.Fatalf("second alice Enqueue error = %v, want ErrPrincipalQueueFull", err)
	}
	mustEnqueue(t, s, QueueItem{ID: "op_c", PrincipalID: "bob", Domain: "filesystem"})
	if err := s.Enqueue(QueueItem{ID: "op_d", PrincipalID: "carol", Domain: "search"}); !errors.Is(err, ErrGlobalQueueFull) {
		t.Fatalf("third principal Enqueue error = %v, want ErrGlobalQueueFull", err)
	}
	if global, principal := s.Len("alice"); global != 2 || principal != 1 {
		t.Fatalf("Len(alice) = (%d, %d), want (2, 1)", global, principal)
	}
}

func TestSchedulerIsFIFOAndPrincipalFair(t *testing.T) {
	s := newTestScheduler(t, QueueLimits{Global: 6, PerPrincipal: 4})
	for _, item := range []QueueItem{
		{ID: "a1", PrincipalID: "alice", Domain: "search"},
		{ID: "a2", PrincipalID: "alice", Domain: "search"},
		{ID: "b1", PrincipalID: "bob", Domain: "filesystem"},
		{ID: "a3", PrincipalID: "alice", Domain: "search"},
		{ID: "b2", PrincipalID: "bob", Domain: "filesystem"},
	} {
		mustEnqueue(t, s, item)
	}

	for index, want := range []string{"a1", "b1", "a2", "b2", "a3"} {
		item, ok := s.Dequeue()
		if !ok || item.ID != want {
			t.Fatalf("Dequeue %d = (%+v, %t), want ID %q", index, item, ok, want)
		}
	}
	if item, ok := s.Dequeue(); ok || item != (QueueItem{}) {
		t.Fatalf("empty Dequeue = (%+v, %t), want zero, false", item, ok)
	}
}

func TestSchedulerConcurrentEnqueueHonorsGlobalLimit(t *testing.T) {
	const limit = 32
	s := newTestScheduler(t, QueueLimits{Global: limit, PerPrincipal: limit})
	var wg sync.WaitGroup
	results := make(chan error, limit*2)
	for i := 0; i < limit*2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results <- s.Enqueue(QueueItem{ID: fmt.Sprintf("op_%d", i), PrincipalID: "alice", Domain: "search"})
		}(i)
	}
	wg.Wait()
	close(results)

	accepted := 0
	for err := range results {
		switch {
		case err == nil:
			accepted++
		case errors.Is(err, ErrGlobalQueueFull):
		default:
			t.Fatalf("Enqueue error = %v, want nil or ErrGlobalQueueFull", err)
		}
	}
	if accepted != limit {
		t.Fatalf("accepted = %d, want %d", accepted, limit)
	}
	if global, principal := s.Len("alice"); global != limit || principal != limit {
		t.Fatalf("Len(alice) = (%d, %d), want (%d, %d)", global, principal, limit, limit)
	}
}

func newTestScheduler(t *testing.T, limits QueueLimits) *Scheduler {
	t.Helper()
	s, err := NewScheduler(limits)
	if err != nil {
		t.Fatalf("NewScheduler() error = %v", err)
	}
	return s
}

func mustEnqueue(t *testing.T, s *Scheduler, item QueueItem) {
	t.Helper()
	if err := s.Enqueue(item); err != nil {
		t.Fatalf("Enqueue(%+v) error = %v", item, err)
	}
}
