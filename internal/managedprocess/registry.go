// Package managedprocess owns the lifecycle identity of processes started by
// FlashGate. Platform adapters and protocol handlers build on this package;
// the registry itself is transport- and platform-neutral.
package managedprocess

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math"
	"sync"
)

const (
	handleEntropyBytes = 32
	handleAttempts     = 8
)

var (
	// ErrIdentifierExhausted is returned rather than reusing a process instance
	// identifier. Reuse could associate a stale reference with a different
	// process.
	ErrIdentifierExhausted = errors.New("managed process identifier space exhausted")
	// ErrInvalidPrincipal is returned when a process has no owner identity.
	ErrInvalidPrincipal = errors.New("managed process principal must not be empty")
	// ErrHandleUnavailable is returned when a unique opaque handle cannot be
	// generated. The registry is not mutated in this case.
	ErrHandleUnavailable = errors.New("managed process handle unavailable")
)

// Handle is the opaque public identity of one managed process. Its value does
// not contain a PID, internal instance identifier, owner, or other metadata.
type Handle string

// PrincipalID identifies the authenticated connection or principal that owns
// a managed process. Callers must derive this value from trusted local state,
// never from an unverified process-tool request field.
type PrincipalID string

// InstanceID identifies one registry membership for a server-started process.
// It is deliberately kept inside registry entries: callers use Handle as the
// primary identity, while the never-reused ID prevents lifecycle aliasing.
type InstanceID uint64

type entry[T any] struct {
	instanceID InstanceID
	principal  PrincipalID
	process    T
}

// Registry tracks processes started and owned by this FlashGate instance.
// Registry is safe for concurrent use. T is normally a pointer to the managed
// process state owned by the future process engine.
type Registry[T any] struct {
	mu           sync.RWMutex
	entries      map[Handle]entry[T]
	lastID       InstanceID
	randomSource io.Reader
}

// NewRegistry creates an empty managed-process registry.
func NewRegistry[T any]() *Registry[T] {
	return &Registry[T]{
		entries:      make(map[Handle]entry[T]),
		randomSource: rand.Reader,
	}
}

// Register records a newly server-started process and returns its opaque,
// principal-bound handle. Neither handles nor internal instance identifiers
// are reused during the lifetime of a Registry.
func (r *Registry[T]) Register(principal PrincipalID, process T) (Handle, error) {
	if principal == "" {
		return "", ErrInvalidPrincipal
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.lastID == InstanceID(math.MaxUint64) {
		return "", ErrIdentifierExhausted
	}
	if r.entries == nil {
		r.entries = make(map[Handle]entry[T])
	}
	if r.randomSource == nil {
		r.randomSource = rand.Reader
	}

	handle, err := r.newUniqueHandle()
	if err != nil {
		return "", err
	}

	r.lastID++
	r.entries[handle] = entry[T]{
		instanceID: r.lastID,
		principal:  principal,
		process:    process,
	}
	return handle, nil
}

func (r *Registry[T]) newUniqueHandle() (Handle, error) {
	var entropy [handleEntropyBytes]byte
	for range handleAttempts {
		if _, err := io.ReadFull(r.randomSource, entropy[:]); err != nil {
			return "", fmt.Errorf("%w: %v", ErrHandleUnavailable, err)
		}
		handle := Handle(base64.RawURLEncoding.EncodeToString(entropy[:]))
		if _, exists := r.entries[handle]; !exists {
			return handle, nil
		}
	}
	return "", ErrHandleUnavailable
}

// Get returns the process only when handle exists and belongs to principal.
// Unknown, removed, and wrong-principal handles intentionally have the same
// result so the registry does not disclose another principal's handles.
func (r *Registry[T]) Get(principal PrincipalID, handle Handle) (T, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	registered, ok := r.entries[handle]
	if !ok || registered.principal != principal {
		var zero T
		return zero, false
	}
	return registered.process, true
}

// Remove atomically releases registry ownership of handle and returns its
// process only to the owning principal. A handle is never reused after removal.
func (r *Registry[T]) Remove(principal PrincipalID, handle Handle) (T, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	registered, ok := r.entries[handle]
	if !ok || registered.principal != principal {
		var zero T
		return zero, false
	}
	delete(r.entries, handle)
	return registered.process, true
}

// Len returns the number of currently registered processes.
func (r *Registry[T]) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.entries)
}
