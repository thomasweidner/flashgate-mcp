// Package managedprocess owns the lifecycle identity of processes started by
// FlashGate. Platform adapters and protocol handlers build on this package;
// the registry itself is transport- and platform-neutral.
package managedprocess

import (
	"errors"
	"math"
	"sync"
)

// ErrIdentifierExhausted is returned rather than reusing a process instance
// identifier. Reuse could associate a stale reference with a different
// process.
var ErrIdentifierExhausted = errors.New("managed process identifier space exhausted")

// InstanceID identifies one registry membership for a server-started process.
//
// InstanceID is an internal lifecycle key, not a public process handle. The
// opaque, principal-bound handle contract is owned by BL-120.
type InstanceID uint64

// Registry tracks processes started and owned by this FlashGate instance.
// Registry is safe for concurrent use. T is normally a pointer to the managed
// process state owned by the future process engine.
type Registry[T any] struct {
	mu      sync.RWMutex
	entries map[InstanceID]T
	lastID  InstanceID
}

// NewRegistry creates an empty managed-process registry.
func NewRegistry[T any]() *Registry[T] {
	return &Registry[T]{entries: make(map[InstanceID]T)}
}

// Register records a newly server-started process and assigns a never-reused
// internal lifecycle identifier.
func (r *Registry[T]) Register(process T) (InstanceID, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.lastID == InstanceID(math.MaxUint64) {
		return 0, ErrIdentifierExhausted
	}

	r.lastID++
	if r.entries == nil {
		r.entries = make(map[InstanceID]T)
	}
	r.entries[r.lastID] = process

	return r.lastID, nil
}

// Get returns the process registered for id. Unknown and removed identifiers
// have the same result.
func (r *Registry[T]) Get(id InstanceID) (T, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	process, ok := r.entries[id]
	return process, ok
}

// Remove atomically releases registry ownership of id and returns its process.
// An identifier is never reused after removal.
func (r *Registry[T]) Remove(id InstanceID) (T, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	process, ok := r.entries[id]
	if ok {
		delete(r.entries, id)
	}
	return process, ok
}

// Len returns the number of currently registered processes.
func (r *Registry[T]) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.entries)
}
