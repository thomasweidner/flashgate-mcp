package router

import (
	"sync"

	"github.com/thomasweidner/flashgate-mcp/internal/mcp/discovery"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/handlers"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/revision"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

// Router dispatches method calls to registered handlers.
type Router struct {
	handlers map[string]handlers.Handler
	mu       sync.RWMutex
	policy   revision.Policy
	name     string
	version  string
}

// New creates a new router.
func New() *Router {
	return NewWithPolicy("flashgate", "0.0.0-dev", revision.ProductionPolicy())
}

// NewWithPolicy injects an exact revision set into the same production router.
func NewWithPolicy(name, version string, policy revision.Policy) *Router {
	return &Router{
		handlers: make(map[string]handlers.Handler),
		policy:   policy,
		name:     name,
		version:  version,
	}
}

// Register registers a handler.
func (r *Router) Register(handler handlers.Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.handlers[handler.Method()] = handler
}

// Dispatch dispatches a method call.
func (r *Router) Dispatch(method string, ctx handlers.Context, params []byte) (any, *protocol.Error) {
	return r.DispatchRevision(revision.Request{Version: protocol.ProtocolVersion, Params: params}, method, ctx)
}

// DispatchRevision selects the exact wire contract without changing handlers.
func (r *Router) DispatchRevision(request revision.Request, method string, ctx handlers.Context) (any, *protocol.Error) {
	version := request.Version
	if method == "server/discover" {
		if !r.policy.Enabled(revision.Stateless) {
			return nil, &protocol.Error{Code: protocol.ErrMethodNotFound, Message: "method not found"}
		}
		if !request.StatelessEnvelope {
			return nil, &protocol.Error{Code: protocol.ErrInvalidParams, Message: "invalid params"}
		}
	}
	if request.StatelessEnvelope && version != revision.Stateless {
		return nil, revision.Unsupported(version, r.policy)
	}
	if !request.StatelessEnvelope && version != protocol.ProtocolVersion {
		return nil, revision.Unsupported(version, r.policy)
	}
	if !r.policy.Enabled(version) {
		return nil, revision.Unsupported(version, r.policy)
	}
	if version == revision.Stateless {
		if method == "server/discover" {
			return revision.Complete(discovery.Result(r.policy), method, r.name, r.version)
		}
		if method != "tools/list" && method != "tools/call" {
			return nil, &protocol.Error{Code: protocol.ErrMethodNotFound, Message: "method not found"}
		}
	}
	r.mu.RLock()
	handler, ok := r.handlers[method]
	r.mu.RUnlock()

	if !ok {
		return nil, &protocol.Error{
			Code:    protocol.ErrMethodNotFound,
			Message: "method not found",
		}
	}

	result, rpcErr := handler.Handle(ctx, request.Params)
	if rpcErr != nil || version == protocol.ProtocolVersion {
		return result, rpcErr
	}
	return revision.Complete(result, method, r.name, r.version)
}
