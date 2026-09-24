// Package operation provides transport-neutral lifecycle mechanics for long-running work.
package operation

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrInvalidDeadline indicates that a watchdog cannot be created from the
// supplied parent context and timeout.
var ErrInvalidDeadline = errors.New("operation deadline requires a parent context and positive timeout")

// Deadline is a server-controlled watchdog for one operation. Domain workers
// observe Context; they do not control or extend the deadline.
type Deadline struct {
	context  context.Context
	cancel   context.CancelCauseFunc
	deadline time.Time
	timer    *time.Timer

	mu       sync.Mutex
	active   bool
	timedOut bool
}

type deadlineContext struct {
	context.Context
	deadline time.Time
}

func (c deadlineContext) Deadline() (time.Time, bool) {
	return c.deadline, true
}

// NewDeadline starts a watchdog using the supplied server-selected timeout.
func NewDeadline(parent context.Context, timeout time.Duration) (*Deadline, error) {
	if parent == nil || timeout <= 0 {
		return nil, ErrInvalidDeadline
	}

	base, cancel := context.WithCancelCause(parent)
	deadline := time.Now().Add(timeout)
	d := &Deadline{
		context:  deadlineContext{Context: base, deadline: deadline},
		cancel:   cancel,
		deadline: deadline,
		active:   true,
	}
	d.timer = time.AfterFunc(timeout, d.expire)
	return d, nil
}

// Context returns the operation-scoped context watched by domain workers.
func (d *Deadline) Context() context.Context {
	return d.context
}

// Time returns the immutable server-selected deadline.
func (d *Deadline) Time() time.Time {
	return d.deadline
}

// TimedOut reports whether this watchdog, rather than its parent, cancelled the
// operation after the server-selected deadline elapsed.
func (d *Deadline) TimedOut() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.timedOut
}

// Stop releases the watchdog after an operation completes. It returns true
// only when it stopped an active watchdog before expiry. Stop is idempotent.
func (d *Deadline) Stop() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.active {
		return false
	}

	d.active = false
	d.timer.Stop()
	d.cancel(context.Canceled)
	return true
}

func (d *Deadline) expire() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.active {
		return
	}

	d.active = false
	d.timedOut = true
	d.cancel(context.DeadlineExceeded)
}
