// Package lifecycle coordinates process-root cancellation and domain cleanup.
package lifecycle

import (
	"context"
	"errors"
	"sync"
)

// Owner owns resources that must be released during process-root shutdown.
// Implementations remain responsible for their domain-specific cleanup.
type Owner interface {
	Shutdown(context.Context) error
}

// Coordinator provides the single process-root cancellation and cleanup path.
// It does not contain domain cleanup logic; it only invokes registered owners.
type Coordinator struct {
	ctx    context.Context
	cancel context.CancelFunc
	owners []Owner
	once   sync.Once
	err    error
}

// New creates a process-root coordinator. Owners are shut down in reverse
// registration order so dependants can be registered after their dependencies.
func New(parent context.Context, owners ...Owner) *Coordinator {
	ctx, cancel := context.WithCancel(parent)
	return &Coordinator{ctx: ctx, cancel: cancel, owners: append([]Owner(nil), owners...)}
}

// Context returns the process-root context shared by runtime adapters.
func (c *Coordinator) Context() context.Context {
	return c.ctx
}

// Shutdown cancels the process root and invokes every resource owner exactly
// once. Repeated calls return the result of the first cleanup pass.
func (c *Coordinator) Shutdown(ctx context.Context) error {
	c.once.Do(func() {
		c.cancel()
		for index := len(c.owners) - 1; index >= 0; index-- {
			c.err = errors.Join(c.err, c.owners[index].Shutdown(ctx))
		}
	})
	return c.err
}
