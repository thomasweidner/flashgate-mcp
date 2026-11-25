package execution

import (
	"context"
	"runtime"
	"slices"

	"github.com/thomasweidner/flashgate-mcp/internal/capability"
)

type operationObserverKey struct{}

// ObserveOperations attaches an in-process observer for operations that reached
// the authorized backend. The backend may return an ordinary domain error.
func ObserveOperations(ctx context.Context, observe func(OperationKey)) context.Context {
	return context.WithValue(ctx, operationObserverKey{}, observe)
}

// CatalogComposition identifies an immutable set of execution routes for a
// protocol and ingress origin. It contains no principal or per-request state.
type CatalogComposition struct {
	routes   *CatalogRoutes
	protocol Protocol
	origin   Origin
}

func (c CatalogComposition) Protocol() Protocol { return c.protocol }
func (c CatalogComposition) Origin() Origin     { return c.origin }
func (c CatalogComposition) Platform() string   { return runtime.GOOS }

func (c CatalogComposition) Policy() capability.Policy {
	return c.routes.policy.profile.Policy()
}

type CatalogRouteID string

// CatalogRoute identifies a selected typed dispatcher and its complete
// resolver, policy and backend binding without exposing its command type.
type CatalogRoute struct{ binding *dispatchBinding }

func (r CatalogRoute) Backend() BackendID               { return r.binding.backend }
func (r CatalogRoute) Representation() RepresentationID { return r.binding.representation }

func (r CatalogRoute) Declared(key OperationKey) bool {
	if r.binding == nil {
		return false
	}
	_, ok := r.binding.declarations.byKey[key]
	return ok
}

func (r CatalogRoute) Eligible(key OperationKey) bool {
	if r.binding == nil {
		return false
	}
	declaration, declared := r.binding.declarations.byKey[key]
	if !declared || !r.binding.policy.profile.Policy().Allows(declaration.Operation) {
		return false
	}
	_, supported := r.binding.operations[key]
	return supported
}

// CatalogRoutes is built once at trusted composition and reused for each
// request assigned to its policy. Route IDs are local to the composition.
type CatalogRoutes struct {
	policy *PolicyBinding
	routes map[CatalogRouteID]CatalogRoute
}

func BindCatalogRoutes(routes map[CatalogRouteID]CatalogRoute) (*CatalogRoutes, error) {
	if len(routes) == 0 {
		return nil, ErrMismatch
	}
	bound := &CatalogRoutes{routes: make(map[CatalogRouteID]CatalogRoute, len(routes))}
	for id, route := range routes {
		if route.binding == nil || (bound.policy != nil && route.binding.policy != bound.policy) {
			return nil, ErrMismatch
		}
		bound.policy = route.binding.policy
		bound.routes[id] = route
	}
	return bound, nil
}

func (r *CatalogRoutes) Context(request Request) (CatalogContext, error) {
	if r == nil || !r.policy.accepts(request) || request.invocation != nil {
		return CatalogContext{}, ErrMismatch
	}
	return CatalogContext{request: request, composition: CatalogComposition{
		routes: r, protocol: request.Protocol(), origin: request.Caller().Origin(),
	}}, nil
}

func (c CatalogComposition) Route(id CatalogRouteID) (CatalogRoute, bool) {
	if c.routes == nil {
		return CatalogRoute{}, false
	}
	route, ok := c.routes.routes[id]
	return route, ok
}

// CatalogContext is issued by the route set for an authenticated, assigned
// request. It carries the exact request separately from its composition.
type CatalogContext struct {
	request     Request
	composition CatalogComposition
}

func (c CatalogContext) Composition() CatalogComposition { return c.composition }
func (c CatalogContext) Valid() bool {
	return c.composition.routes != nil
}

func (c CatalogContext) SameScope(other CatalogContext) bool {
	return c.Valid() && c.composition == other.composition && c.request.correlation == other.request.correlation
}

func (c CatalogContext) Declared(key OperationKey) bool {
	route, _ := c.composition.Route("")
	return route.Declared(key)
}

func (c CatalogContext) Eligible(key OperationKey) bool {
	route, _ := c.composition.Route("")
	return route.Eligible(key)
}

type catalogInvocationKey struct{}

type catalogInvocationBinding struct {
	route    CatalogRoute
	variants []OperationKey
}

// CatalogInvocation carries a fresh request restricted to the exact route and
// variants admitted for one tool entry.
type CatalogInvocation struct{ request Request }

func (c CatalogContext) BindInvocation(id CatalogRouteID, variants []OperationKey) (CatalogInvocation, error) {
	route, ok := c.composition.Route(id)
	if !ok || len(variants) == 0 {
		return CatalogInvocation{}, ErrMismatch
	}
	for _, key := range variants {
		if !route.Eligible(key) {
			return CatalogInvocation{}, ErrMismatch
		}
	}
	request := c.request
	request.invocation = &catalogInvocationBinding{route, slices.Clone(variants)}
	return CatalogInvocation{request}, nil
}

func (i CatalogInvocation) Request() Request { return i.request }

func (i CatalogInvocation) Context(ctx context.Context) context.Context {
	if ctx == nil || i.request.invocation == nil || ctx.Value(catalogInvocationKey{}) != nil {
		return ctx
	}
	return context.WithValue(ctx, catalogInvocationKey{}, i.request.invocation)
}

func (i CatalogInvocation) Matches(route CatalogRoute) bool {
	return i.request.invocation != nil && i.request.invocation.route == route
}

func (i CatalogInvocation) Allows(key OperationKey) bool {
	return i.request.invocation != nil && slices.Contains(i.request.invocation.variants, key)
}

func (d *Dispatcher[C, R]) CatalogRoute() CatalogRoute { return CatalogRoute{d.binding} }

func (d *Dispatcher[C, R]) acceptsInvocation(ctx context.Context, request Request) bool {
	if ctx == nil {
		return false
	}
	if bound, _ := ctx.Value(catalogInvocationKey{}).(*catalogInvocationBinding); bound != nil && bound != request.invocation {
		return false
	}
	return request.invocation == nil || (CatalogInvocation{request}).Matches(d.CatalogRoute())
}

func (d *Dispatcher[C, R]) CatalogContext(request Request) (CatalogContext, error) {
	if d == nil {
		return CatalogContext{}, ErrMismatch
	}
	return d.catalog.Context(request)
}

func (d *Dispatcher[C, R]) OperationKeys(name OperationID) []OperationKey {
	var keys []OperationKey
	for _, key := range d.authorizer.declarations.keys {
		if key.Name == name {
			keys = append(keys, key)
		}
	}
	return keys
}
