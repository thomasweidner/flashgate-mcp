package executionidentity

import (
	"fmt"
	"sync"
	"time"
)

// StateBinding is the complete ownership boundary for handles, cached values,
// result resources, temporary state, and cancellation rights. Correlation is
// intentionally excluded so state may be consumed by a later request from the
// same authorized execution context.
type StateBinding struct {
	principal         string
	groups            string
	profile           string
	rootID            string
	backend           BackendID
	serviceInstance   string
	serviceGeneration string
	protocolContext   string
}

// StateBinding returns the immutable ownership identity of an execution
// context. Callers cannot construct or weaken bindings field by field.
func (e ExecutionContext) StateBinding() StateBinding {
	return StateBinding{
		principal:         e.caller.principal,
		groups:            fmt.Sprintf("%q", e.caller.groups),
		profile:           e.profile,
		rootID:            e.rootID,
		backend:           e.backend,
		serviceInstance:   e.serviceInstance,
		serviceGeneration: e.serviceGeneration,
		protocolContext:   e.protocolContext,
	}
}

type stateEntry[T any] struct {
	binding   StateBinding
	expiresAt time.Time
	value     T
}

// BoundStore stores opaque execution state and requires an exact owner match
// on every operation. It is suitable as the common ownership gate beneath
// domain-specific handle, result-resource, and cache implementations.
type BoundStore[T any] struct {
	mu      sync.Mutex
	now     func() time.Time
	entries map[string]stateEntry[T]
}

func NewBoundStore[T any]() *BoundStore[T] {
	return &BoundStore[T]{now: time.Now, entries: make(map[string]stateEntry[T])}
}

// Put creates state under an opaque identifier. Existing identifiers cannot
// be replaced, preventing a caller from taking over another owner's state.
func (s *BoundStore[T]) Put(id string, owner ExecutionContext, value T) error {
	if s == nil || id == "" || !validStateOwner(owner) {
		return ErrInvalidContext
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.entries[id]; exists {
		return ErrStateAccessDenied
	}
	s.entries[id] = stateEntry[T]{binding: owner.StateBinding(), expiresAt: owner.expiresAt, value: value}
	return nil
}

// Get returns state only to the exact execution-context owner and only before
// its expiry. Mismatches deliberately return one safe denial category.
func (s *BoundStore[T]) Get(id string, owner ExecutionContext) (T, error) {
	var zero T
	if s == nil || id == "" || !validStateOwner(owner) {
		return zero, ErrInvalidContext
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, exists := s.entries[id]
	if !exists || entry.binding != owner.StateBinding() {
		return zero, ErrStateAccessDenied
	}
	now := s.now()
	if !now.Before(entry.expiresAt) || !now.Before(owner.expiresAt) {
		delete(s.entries, id)
		return zero, ErrStateExpired
	}
	return entry.value, nil
}

// Delete applies the same ownership and expiry checks as Get. Domain code may
// use it for cancellation, result release, or temporary-state cleanup.
func (s *BoundStore[T]) Delete(id string, owner ExecutionContext) error {
	if _, err := s.Get(id, owner); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, id)
	return nil
}

func validStateOwner(owner ExecutionContext) bool {
	return owner.caller.principal != "" && owner.profile != "" && owner.rootID != "" &&
		owner.backend != "" && owner.serviceInstance != "" && owner.serviceGeneration != "" &&
		owner.protocolContext != "" && !owner.expiresAt.IsZero()
}
