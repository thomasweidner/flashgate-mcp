// Package operation provides transport-neutral lifecycle mechanics for
// long-running, domain-owned work.
package operation

import "context"

// Cancellation owns the cooperative cancellation signal for one operation.
// Domain workers receive Context and remain responsible for checking it at
// bounded intervals and returning promptly when it is cancelled.
type Cancellation struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// NewCancellation derives an operation-scoped cancellation signal from parent.
// Parent cancellation propagates to the operation, while cancelling the
// operation does not affect its parent.
func NewCancellation(parent context.Context) *Cancellation {
	ctx, cancel := context.WithCancel(parent)
	return &Cancellation{ctx: ctx, cancel: cancel}
}

// Context returns the operation-scoped context supplied to domain workers.
func (c *Cancellation) Context() context.Context {
	return c.ctx
}

// Cancel requests cooperative cancellation. Cancel is safe to call
// concurrently and more than once.
func (c *Cancellation) Cancel() {
	c.cancel()
}
