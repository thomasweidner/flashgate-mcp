// Package operation provides transport-neutral lifecycle infrastructure for
// long-running, domain-owned work.
package operation

import (
	"errors"
	"math"
	"sync"
)

var (
	// ErrCounterOverflow reports that adding a byte count would exceed uint64.
	ErrCounterOverflow = errors.New("operation byte counter overflow")
	// ErrInvalidProgress reports a domain-progress update outside its declared
	// bound.
	ErrInvalidProgress = errors.New("operation progress exceeds total")
)

// ByteCounters records useful I/O performed by an operation. The values are
// cumulative and intentionally separate so callers do not need to infer work
// from a domain-specific result.
type ByteCounters struct {
	Read    uint64
	Written uint64
	Scanned uint64
}

// DomainProgress is bounded, domain-defined progress. Total zero means that
// the domain has not declared a measurable total; in that case Completed must
// also be zero.
type DomainProgress struct {
	Completed uint64
	Total     uint64
}

// ProgressSnapshot is an immutable point-in-time view of operation progress.
type ProgressSnapshot struct {
	Bytes  ByteCounters
	Domain DomainProgress
}

// ProgressTracker safely accumulates byte counters and bounded domain progress
// for one operation. Its zero value is ready for use.
type ProgressTracker struct {
	mu       sync.RWMutex
	bytes    ByteCounters
	progress DomainProgress
}

// AddBytes adds cumulative byte counts. It rejects the entire update if any
// counter would overflow, leaving the tracker unchanged.
func (t *ProgressTracker) AddBytes(delta ByteCounters) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if wouldOverflow(t.bytes.Read, delta.Read) ||
		wouldOverflow(t.bytes.Written, delta.Written) ||
		wouldOverflow(t.bytes.Scanned, delta.Scanned) {
		return ErrCounterOverflow
	}

	t.bytes.Read += delta.Read
	t.bytes.Written += delta.Written
	t.bytes.Scanned += delta.Scanned
	return nil
}

// SetDomainProgress replaces the domain-defined progress. Updates must remain
// within their declared total. A zero total clears domain progress.
func (t *ProgressTracker) SetDomainProgress(progress DomainProgress) error {
	if progress.Completed > progress.Total {
		return ErrInvalidProgress
	}

	t.mu.Lock()
	t.progress = progress
	t.mu.Unlock()
	return nil
}

// Snapshot returns a consistent copy of all current counters.
func (t *ProgressTracker) Snapshot() ProgressSnapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return ProgressSnapshot{
		Bytes:  t.bytes,
		Domain: t.progress,
	}
}

func wouldOverflow(current, delta uint64) bool {
	return delta > math.MaxUint64-current
}
