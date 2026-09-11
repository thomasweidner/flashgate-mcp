package operation

import (
	"errors"
	"sync"
)

var (
	// ErrInvalidQueueItem indicates that an item is missing an identity needed
	// for deterministic scheduling and accounting.
	ErrInvalidQueueItem = errors.New("invalid queue item")
	// ErrGlobalQueueFull indicates that the global queue capacity is exhausted.
	ErrGlobalQueueFull = errors.New("global operation queue is full")
	// ErrPrincipalQueueFull indicates that a principal's queue capacity is exhausted.
	ErrPrincipalQueueFull = errors.New("principal operation queue is full")
)

// QueueLimits bounds retained work globally and for each principal.
type QueueLimits struct {
	Global       int
	PerPrincipal int
}

// QueueItem is the scheduler-owned identity for queued domain work. ID is an
// opaque operation handle; Domain identifies the owner that will execute it.
type QueueItem struct {
	ID          string
	PrincipalID string
	Domain      string
}

// Scheduler is a bounded, concurrency-safe, principal-fair FIFO queue.
//
// Items are FIFO within a principal. Dequeue serves active principals in
// round-robin order, preventing a continuously busy principal from starving
// another principal that has queued work.
type Scheduler struct {
	mu         sync.Mutex
	limits     QueueLimits
	queues     map[string][]QueueItem
	principals []string
	size       int
}

// NewScheduler constructs an empty scheduler with explicit positive limits.
func NewScheduler(limits QueueLimits) (*Scheduler, error) {
	if limits.Global <= 0 || limits.PerPrincipal <= 0 || limits.PerPrincipal > limits.Global {
		return nil, errors.New("operation queue limits must be positive and per-principal must not exceed global")
	}
	return &Scheduler{limits: limits, queues: make(map[string][]QueueItem)}, nil
}

// Enqueue retains item or returns a deterministic capacity error without
// partially mutating the queue.
func (s *Scheduler) Enqueue(item QueueItem) error {
	if item.ID == "" || item.PrincipalID == "" || item.Domain == "" {
		return ErrInvalidQueueItem
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.size >= s.limits.Global {
		return ErrGlobalQueueFull
	}
	queue := s.queues[item.PrincipalID]
	if len(queue) >= s.limits.PerPrincipal {
		return ErrPrincipalQueueFull
	}
	if len(queue) == 0 {
		s.principals = append(s.principals, item.PrincipalID)
	}
	s.queues[item.PrincipalID] = append(queue, item)
	s.size++
	return nil
}

// Dequeue returns the next fairly scheduled item. The bool is false when the
// scheduler is empty.
func (s *Scheduler) Dequeue() (QueueItem, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.size == 0 {
		return QueueItem{}, false
	}

	principal := s.principals[0]
	queue := s.queues[principal]
	item := queue[0]
	queue = queue[1:]
	s.size--

	if len(queue) == 0 {
		delete(s.queues, principal)
		s.principals = s.principals[1:]
	} else {
		s.queues[principal] = queue
		copy(s.principals, s.principals[1:])
		s.principals[len(s.principals)-1] = principal
	}
	return item, true
}

// Len reports the current global and principal-specific queue lengths.
func (s *Scheduler) Len(principalID string) (global, principal int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.size, len(s.queues[principalID])
}
