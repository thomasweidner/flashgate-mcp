// Package operation provides transport-neutral lifecycle primitives for
// long-running, domain-owned work.
package operation

// Status is the internal lifecycle state of an operation.
//
// Status values are stable internal contract values. Mapping them to an MCP
// Tasks extension is owned by the MCP adapter and is deliberately not defined
// here.
type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
	StatusTimedOut  Status = "timed_out"
)

// Valid reports whether status is one of the accepted lifecycle states.
func (status Status) Valid() bool {
	switch status {
	case StatusQueued, StatusRunning, StatusCompleted, StatusFailed, StatusCancelled, StatusTimedOut:
		return true
	default:
		return false
	}
}

// Terminal reports whether status represents a completed lifecycle.
func (status Status) Terminal() bool {
	switch status {
	case StatusCompleted, StatusFailed, StatusCancelled, StatusTimedOut:
		return true
	default:
		return false
	}
}

// CanTransitionTo reports whether an operation may move directly from status
// to next. Repeating a state is not a transition, and terminal states are
// immutable.
func (status Status) CanTransitionTo(next Status) bool {
	if !status.Valid() || !next.Valid() || status == next {
		return false
	}

	switch status {
	case StatusQueued:
		return next == StatusRunning || next == StatusFailed || next == StatusCancelled || next == StatusTimedOut
	case StatusRunning:
		return next.Terminal()
	default:
		return false
	}
}
