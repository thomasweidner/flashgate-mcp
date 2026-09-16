package managedprocess

import (
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
	mu     sync.RWMutex
	status Status
}

// NewState creates a lifecycle in the only valid initial state.
func NewState() *State {
	return &State{status: StatusStarting}
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
	return nil
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
