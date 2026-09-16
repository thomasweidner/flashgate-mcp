package operation

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"
)

var (
	ErrInvalidLeakRecord = errors.New("operation: invalid leak record")
	ErrDuplicateJob      = errors.New("operation: duplicate job")
	ErrUnknownJob        = errors.New("operation: unknown job")
	ErrJobOwnerMismatch  = errors.New("operation: job owner mismatch")
	ErrCleanupPanic      = errors.New("operation: cleanup callback panicked")
)

// LeakRecord is the minimum domain-owned contract needed to reap an expired
// job. Owner is an opaque, server-derived principal identity. Cleanup must be
// idempotent because a failed cleanup remains eligible for a later sweep.
type LeakRecord struct {
	ID        string
	Owner     string
	ExpiresAt time.Time
	Cleanup   func(context.Context) error
}

// LeakOutcome records one expired job observed by a sweep.
type LeakOutcome struct {
	ID      string
	Cleaned bool
	Error   error
}

// LeakMetrics is a race-safe snapshot. Counters are monotonic for the lifetime
// of the registry; Active and Leaked are current gauges.
type LeakMetrics struct {
	Registered       uint64
	Active           uint64
	Sweeps           uint64
	ExpiredDetected  uint64
	Cleaned          uint64
	CleanupFailures  uint64
	OwnershipDenials uint64
	Leaked           uint64
}

type leakEntry struct {
	record   LeakRecord
	cleaning bool
	leaked   bool
}

// LeakRegistry provides TTL reaping and ownership checks without taking over
// domain result semantics. Its methods are safe for concurrent use.
type LeakRegistry struct {
	mu      sync.Mutex
	entries map[string]*leakEntry
	metrics LeakMetrics
}

func NewLeakRegistry() *LeakRegistry { return &LeakRegistry{entries: make(map[string]*leakEntry)} }

// Register begins tracking a domain-owned job until it is removed or reaped.
func (r *LeakRegistry) Register(record LeakRecord) error {
	if record.ID == "" || record.Owner == "" || record.ExpiresAt.IsZero() || record.Cleanup == nil {
		return ErrInvalidLeakRecord
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.entries[record.ID]; exists {
		return ErrDuplicateJob
	}
	r.entries[record.ID] = &leakEntry{record: record}
	r.metrics.Registered++
	return nil
}

// CheckOwner verifies that a tracked job belongs to owner. Adapters should map
// both errors to the same external response when job existence is sensitive.
func (r *LeakRegistry) CheckOwner(id, owner string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, exists := r.entries[id]
	if !exists {
		return ErrUnknownJob
	}
	if entry.record.Owner != owner {
		r.metrics.OwnershipDenials++
		return ErrJobOwnerMismatch
	}
	return nil
}

// Remove stops tracking a job after verifying its owner. Cleanup remains the
// owning domain's responsibility for jobs removed before expiry.
func (r *LeakRegistry) Remove(id, owner string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, exists := r.entries[id]
	if !exists {
		return ErrUnknownJob
	}
	if entry.record.Owner != owner {
		r.metrics.OwnershipDenials++
		return ErrJobOwnerMismatch
	}
	delete(r.entries, id)
	return nil
}

// Sweep claims jobs whose TTL is at or before now and invokes cleanup without
// holding the lock. Results are ordered by job ID for deterministic audit.
func (r *LeakRegistry) Sweep(ctx context.Context, now time.Time) []LeakOutcome {
	r.mu.Lock()
	r.metrics.Sweeps++
	ids := make([]string, 0)
	for id, entry := range r.entries {
		if !entry.cleaning && !entry.record.ExpiresAt.After(now) {
			entry.cleaning = true
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	r.mu.Unlock()

	outcomes := make([]LeakOutcome, 0, len(ids))
	for _, id := range ids {
		r.mu.Lock()
		entry, exists := r.entries[id]
		if !exists || !entry.cleaning {
			r.mu.Unlock()
			continue
		}
		cleanup := entry.record.Cleanup
		if !entry.leaked {
			entry.leaked = true
			r.metrics.ExpiredDetected++
		}
		r.mu.Unlock()

		err := runCleanup(ctx, cleanup)
		r.mu.Lock()
		entry, exists = r.entries[id]
		if exists {
			if err == nil {
				delete(r.entries, id)
				r.metrics.Cleaned++
			} else {
				entry.cleaning = false
				r.metrics.CleanupFailures++
			}
		}
		r.mu.Unlock()
		outcomes = append(outcomes, LeakOutcome{ID: id, Cleaned: err == nil, Error: err})
	}
	return outcomes
}

func runCleanup(ctx context.Context, cleanup func(context.Context) error) (err error) {
	defer func() {
		if recover() != nil {
			err = ErrCleanupPanic
		}
	}()
	return cleanup(ctx)
}

// Metrics returns a consistent snapshot without exposing job identities.
func (r *LeakRegistry) Metrics() LeakMetrics {
	r.mu.Lock()
	defer r.mu.Unlock()
	metrics := r.metrics
	metrics.Active = uint64(len(r.entries))
	for _, entry := range r.entries {
		if entry.leaked {
			metrics.Leaked++
		}
	}
	return metrics
}
