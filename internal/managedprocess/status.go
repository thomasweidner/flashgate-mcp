package managedprocess

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Status describes the lifecycle state of a process owned by the managed
// process engine. These values are domain-specific and intentionally differ
// from the Operations/Job Manager states.
type Status string

const (
	StatusStarting Status = "starting"
	StatusRunning  Status = "running"
	StatusExited   Status = "exited"
	StatusFailed   Status = "failed"
	StatusStopped  Status = "stopped"
	StatusTimedOut Status = "timed_out"
)

var (
	// ErrInvalidStatus is returned for a status outside the managed-process
	// lifecycle contract.
	ErrInvalidStatus = errors.New("invalid managed process status")
	// ErrInvalidStatusTransition is returned when a transition would move a
	// process backwards or replace a terminal outcome.
	ErrInvalidStatusTransition = errors.New("invalid managed process status transition")
)

// Valid reports whether status is part of the managed-process contract.
func (status Status) Valid() bool {
	switch status {
	case StatusStarting, StatusRunning, StatusExited, StatusFailed, StatusStopped, StatusTimedOut:
		return true
	default:
		return false
	}
}

// Terminal reports whether status is a final lifecycle outcome.
func (status Status) Terminal() bool {
	switch status {
	case StatusExited, StatusFailed, StatusStopped, StatusTimedOut:
		return true
	default:
		return false
	}
}

// State stores the current managed-process status and serializes competing
// lifecycle updates. Its zero value represents StatusStarting and is usable.
type State struct {
	mu       sync.RWMutex
	status   Status
	done     chan struct{}
	doneOnce sync.Once
}

// NewState creates a lifecycle in the only valid initial state.
func NewState() *State {
	return &State{status: StatusStarting, done: make(chan struct{})}
}

// Status returns a consistent snapshot of the current lifecycle state.
func (state *State) Status() Status {
	state.mu.RLock()
	defer state.mu.RUnlock()

	return state.currentStatus()
}

// Transition atomically advances the lifecycle. Starting may become running
// or any terminal outcome; running may become any terminal outcome. Terminal
// outcomes cannot be replaced, including by another terminal outcome.
func (state *State) Transition(next Status) error {
	if !next.Valid() {
		return fmt.Errorf("%w: %q", ErrInvalidStatus, next)
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	current := state.currentStatus()
	if !validStatusTransition(current, next) {
		return fmt.Errorf("%w: %q to %q", ErrInvalidStatusTransition, current, next)
	}
	state.status = next
	if next.Terminal() {
		state.doneOnce.Do(func() { close(state.doneChannel()) })
	}
	return nil
}

// Wait blocks until the lifecycle reaches its first terminal outcome or ctx
// ends. Cancellation only stops this observer; it never changes process state.
func (state *State) Wait(ctx context.Context) (Status, error) {
	state.mu.Lock()
	done := state.doneChannel()
	status := state.currentStatus()
	state.mu.Unlock()

	if status.Terminal() {
		return status, nil
	}
	select {
	case <-done:
		return state.Status(), nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (state *State) doneSignal() <-chan struct{} {
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.doneChannel()
}

// doneChannel must be called while state.mu is held. It keeps the zero value
// usable without racing concurrent waiters and transitions.
func (state *State) doneChannel() chan struct{} {
	if state.done == nil {
		state.done = make(chan struct{})
	}
	return state.done
}

func (state *State) currentStatus() Status {
	if state.status == "" {
		return StatusStarting
	}
	return state.status
}

func validStatusTransition(current, next Status) bool {
	switch current {
	case StatusStarting:
		return next == StatusRunning || next.Terminal()
	case StatusRunning:
		return next.Terminal()
	default:
		return false
	}
}
