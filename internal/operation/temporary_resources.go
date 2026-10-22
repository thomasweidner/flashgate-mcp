package operation

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// CleanupOutcome records why temporary-resource cleanup was requested.
type CleanupOutcome string

const (
	CleanupSucceeded CleanupOutcome = "succeeded"
	CleanupFailed    CleanupOutcome = "failed"
	CleanupCancelled CleanupOutcome = "cancelled"
	CleanupTimedOut  CleanupOutcome = "timed_out"
)

// Valid reports whether the outcome is part of the operation cleanup contract.
func (o CleanupOutcome) Valid() bool {
	switch o {
	case CleanupSucceeded, CleanupFailed, CleanupCancelled, CleanupTimedOut:
		return true
	default:
		return false
	}
}

// TemporaryResource identifies domain-owned temporary data without exposing a
// host path. Cleanup must be bounded by the supplied context and may be called
// only once by a TemporaryResources collection.
type TemporaryResource struct {
	ID      string
	Cleanup func(context.Context) error
}

// CleanupMarker is the durable, path-free cleanup state for one resource.
type CleanupMarker struct {
	ID         string
	Outcome    CleanupOutcome
	Incomplete bool
}

// CleanupReport summarizes a cleanup pass. Markers are returned in registration
// order so callers can persist deterministic lifecycle evidence.
type CleanupReport struct {
	Outcome CleanupOutcome
	Markers []CleanupMarker
}

// TemporaryResources tracks cleanup for the temporary data of one operation.
// Its zero value is ready for use.
type TemporaryResources struct {
	mu        sync.Mutex
	resources []TemporaryResource
	ids       map[string]struct{}
	cleaning  bool
	done      chan struct{}
	report    *CleanupReport
	err       error
}

// Register adds a resource before cleanup starts.
func (r *TemporaryResources) Register(resource TemporaryResource) error {
	if resource.ID == "" {
		return errors.New("temporary resource ID is required")
	}
	if resource.Cleanup == nil {
		return errors.New("temporary resource cleanup function is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cleaning || r.report != nil {
		return errors.New("temporary resource cleanup has already started")
	}
	if r.ids == nil {
		r.ids = make(map[string]struct{})
	}
	if _, exists := r.ids[resource.ID]; exists {
		return fmt.Errorf("temporary resource %q is already registered", resource.ID)
	}
	r.ids[resource.ID] = struct{}{}
	r.resources = append(r.resources, resource)
	return nil
}

// Cleanup invokes every registered cleanup function exactly once. A failed or
// context-interrupted callback is marked incomplete, while cleanup continues so
// later resources are not leaked. Concurrent and later calls receive a copy of
// the first completed report and never invoke callbacks again.
func (r *TemporaryResources) Cleanup(ctx context.Context, outcome CleanupOutcome) (CleanupReport, error) {
	if ctx == nil {
		return CleanupReport{}, errors.New("cleanup context is required")
	}
	if !outcome.Valid() {
		return CleanupReport{}, errors.New("valid cleanup outcome is required")
	}

	r.mu.Lock()
	if r.report != nil {
		report, err := cloneCleanupReport(*r.report), r.err
		r.mu.Unlock()
		return report, err
	}
	if r.cleaning {
		done := r.done
		r.mu.Unlock()
		select {
		case <-done:
			r.mu.Lock()
			report, err := cloneCleanupReport(*r.report), r.err
			r.mu.Unlock()
			return report, err
		case <-ctx.Done():
			return CleanupReport{}, ctx.Err()
		}
	}
	r.cleaning = true
	r.done = make(chan struct{})
	resources := append([]TemporaryResource(nil), r.resources...)
	r.mu.Unlock()

	report := CleanupReport{Outcome: outcome, Markers: make([]CleanupMarker, 0, len(resources))}
	var cleanupErrors []error
	for _, resource := range resources {
		err := resource.Cleanup(ctx)
		marker := CleanupMarker{ID: resource.ID, Outcome: outcome, Incomplete: err != nil}
		report.Markers = append(report.Markers, marker)
		if err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("temporary resource %q: %w", resource.ID, err))
		}
	}
	cleanupErr := errors.Join(cleanupErrors...)
	r.mu.Lock()
	r.report = &report
	r.err = cleanupErr
	close(r.done)
	r.mu.Unlock()
	return cloneCleanupReport(report), cleanupErr
}

func cloneCleanupReport(report CleanupReport) CleanupReport {
	cloned := report
	cloned.Markers = append([]CleanupMarker(nil), report.Markers...)
	return cloned
}
