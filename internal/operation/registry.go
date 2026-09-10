// Package operation provides transport-neutral lifecycle infrastructure for
// bounded, domain-owned operations.
package operation

import (
	"errors"
	"sync"
)

var (
	// ErrInvalidIdentity indicates that an operation ID or domain is empty.
	ErrInvalidIdentity = errors.New("operation identity is incomplete")
	// ErrAlreadyRegistered indicates that an operation ID is already live.
	ErrAlreadyRegistered = errors.New("operation is already registered")
	// ErrNotFound intentionally covers both an unknown operation and a domain
	// mismatch so callers cannot use the registry to probe other domains.
	ErrNotFound = errors.New("operation not found")
)

// Record is one domain-owned value in a Registry. ID is an internal registry
// key, not a public operation handle. Public opaque, identity-bound handles are
// added by the handle layer rather than inferred from this record.
type Record[T any] struct {
	ID     string
	Domain string
	Value  T
}

// Registry stores live operations and serializes their lifecycle transitions.
// Values are returned by value; callers that use reference-like T values remain
// responsible for synchronizing mutations inside those values.
type Registry[T any] struct {
	mu      sync.RWMutex
	records map[string]Record[T]
}

// NewRegistry creates an empty operation registry.
func NewRegistry[T any]() *Registry[T] {
	return &Registry[T]{records: make(map[string]Record[T])}
}

// Register atomically adds a live operation. Reusing an ID is rejected even
// when the proposed domain differs.
func (r *Registry[T]) Register(record Record[T]) error {
	if record.ID == "" || record.Domain == "" {
		return ErrInvalidIdentity
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.records[record.ID]; exists {
		return ErrAlreadyRegistered
	}
	r.records[record.ID] = record
	return nil
}

// Get returns an operation only to its owning domain. Unknown IDs and domain
// mismatches deliberately have the same result.
func (r *Registry[T]) Get(domain, id string) (Record[T], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	record, exists := r.records[id]
	if !exists || domain == "" || record.Domain != domain {
		return Record[T]{}, ErrNotFound
	}
	return record, nil
}

// Replace atomically changes the stored value without allowing identity or
// domain ownership to change.
func (r *Registry[T]) Replace(domain, id string, value T) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	record, exists := r.records[id]
	if !exists || domain == "" || record.Domain != domain {
		return ErrNotFound
	}
	record.Value = value
	r.records[id] = record
	return nil
}

// Remove atomically ends the registry lifecycle and returns the last value to
// the owning domain. Unknown IDs and domain mismatches do not mutate state.
func (r *Registry[T]) Remove(domain, id string) (Record[T], error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	record, exists := r.records[id]
	if !exists || domain == "" || record.Domain != domain {
		return Record[T]{}, ErrNotFound
	}
	delete(r.records, id)
	return record, nil
}

// Len returns the number of live operations.
func (r *Registry[T]) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.records)
}
