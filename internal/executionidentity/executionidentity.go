// Package executionidentity separates caller authorization from the operating
// system identity used to execute an operation.
package executionidentity

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sort"
)

// BackendID identifies an execution backend selected by trusted policy.
type BackendID string

const (
	BackendCurrentProcess BackendID = "current-process"
	BackendServiceAccount BackendID = "service-account"
	BackendUserWorker     BackendID = "user-worker"
)

var (
	ErrDenied             = errors.New("execution denied")
	ErrInvalidContext     = errors.New("invalid execution context")
	ErrBackendUnavailable = errors.New("execution backend unavailable")
)

// Caller is an authenticated principal derived by a trusted transport.
type Caller struct {
	principal string
	groups    []string
}

// NewCaller constructs a caller and takes a copy of groups.
func NewCaller(principal string, groups []string) (Caller, error) {
	if principal == "" {
		return Caller{}, fmt.Errorf("%w: empty principal", ErrInvalidContext)
	}
	groupsCopy := append([]string(nil), groups...)
	sort.Strings(groupsCopy)
	for i, group := range groupsCopy {
		if group == "" || (i > 0 && group == groupsCopy[i-1]) {
			return Caller{}, fmt.Errorf("%w: invalid caller groups", ErrInvalidContext)
		}
	}
	return Caller{principal: principal, groups: groupsCopy}, nil
}

func (c Caller) Principal() string { return c.principal }

// Groups returns a copy so the authenticated identity cannot be mutated.
func (c Caller) Groups() []string { return append([]string(nil), c.groups...) }

// Request contains only caller-selectable operation inputs. Backend selection
// is deliberately absent and remains an authorization-policy decision.
type Request struct {
	RootID      string
	Capability  string
	Operation   string
	Correlation string
}

// Decision is the trusted policy output used to build an ExecutionContext.
type Decision struct {
	Allowed           bool
	Profile           string
	RootID            string
	Capability        string
	Backend           BackendID
	ServiceGeneration string
}

// ExecutionContext is the immutable identity and policy binding passed below
// the authorization boundary.
type ExecutionContext struct {
	caller            Caller
	profile           string
	rootID            string
	capability        string
	backend           BackendID
	serviceGeneration string
	correlation       string
}

func (e ExecutionContext) Caller() Caller            { return e.caller }
func (e ExecutionContext) Profile() string           { return e.profile }
func (e ExecutionContext) RootID() string            { return e.rootID }
func (e ExecutionContext) Capability() string        { return e.capability }
func (e ExecutionContext) Backend() BackendID        { return e.backend }
func (e ExecutionContext) ServiceGeneration() string { return e.serviceGeneration }
func (e ExecutionContext) Correlation() string       { return e.correlation }

// Policy authorizes an authenticated caller and selects a backend from trusted
// root configuration. Implementations must not use payload-supplied identity.
type Policy interface {
	Authorize(context.Context, Caller, Request) (Decision, error)
}

// Operation is a protocol-independent domain operation.
type Operation interface {
	Name() string
}

// OSAdapter owns the actual platform operation. It receives the complete
// execution binding and must preserve cancellation and deadlines.
type OSAdapter interface {
	Execute(context.Context, ExecutionContext, Operation) (any, error)
}

// Backend dispatches an operation under one effective execution identity.
type Backend interface {
	ID() BackendID
	Dispatch(context.Context, ExecutionContext, Operation, OSAdapter) (any, error)
}

// EffectiveIdentity identifies the operating-system account under which an
// operation is executed. Privileged must be supplied by trusted service
// bootstrap code, never inferred from a caller-controlled name.
type EffectiveIdentity struct {
	Principal  string
	Privileged bool
}

// AuditEvent records both sides of the service-account identity boundary.
// It deliberately contains stable IDs rather than host paths or credentials.
type AuditEvent struct {
	CallerPrincipal    string
	EffectivePrincipal string
	Backend            BackendID
	RootID             string
	Capability         string
	Operation          string
	Correlation        string
	ServiceGeneration  string
	Result             string
}

// AuditSink receives bounded service-account dispatch records.
type AuditSink interface {
	Record(context.Context, AuditEvent)
}

// Registry is an immutable backend selector after construction.
type Registry struct{ backends map[BackendID]Backend }

// NewRegistry validates and registers supported backends. The reserved
// user-worker backend always fails closed in Version 1.0.
func NewRegistry(backends ...Backend) (*Registry, error) {
	registered := make(map[BackendID]Backend, len(backends))
	for _, backend := range backends {
		if backend == nil {
			return nil, fmt.Errorf("%w: nil backend", ErrBackendUnavailable)
		}
		id := backend.ID()
		if id == "" || id == BackendUserWorker {
			return nil, fmt.Errorf("%w: %q", ErrBackendUnavailable, id)
		}
		if _, exists := registered[id]; exists {
			return nil, fmt.Errorf("%w: duplicate %q", ErrBackendUnavailable, id)
		}
		registered[id] = backend
	}
	return &Registry{backends: registered}, nil
}

func (r *Registry) selectBackend(id BackendID) (Backend, error) {
	if r == nil || id == BackendUserWorker {
		return nil, fmt.Errorf("%w: %q", ErrBackendUnavailable, id)
	}
	backend, ok := r.backends[id]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrBackendUnavailable, id)
	}
	return backend, nil
}

// Dispatcher enforces authorization before selecting an execution backend.
type Dispatcher struct {
	policy   Policy
	registry *Registry
	adapter  OSAdapter
}

func NewDispatcher(policy Policy, registry *Registry, adapter OSAdapter) (*Dispatcher, error) {
	if policy == nil || registry == nil || adapter == nil {
		return nil, fmt.Errorf("%w: incomplete dispatcher", ErrInvalidContext)
	}
	return &Dispatcher{policy: policy, registry: registry, adapter: adapter}, nil
}

// Dispatch authorizes, validates the policy binding, and invokes exactly the
// backend chosen by policy. Denials and invalid bindings never reach a backend.
func (d *Dispatcher) Dispatch(ctx context.Context, caller Caller, request Request, operation Operation) (any, error) {
	if caller.principal == "" || request.RootID == "" || request.Capability == "" || request.Operation == "" || request.Correlation == "" || operation == nil || operation.Name() != request.Operation {
		return nil, ErrInvalidContext
	}
	decision, err := d.policy.Authorize(ctx, caller, request)
	if err != nil {
		return nil, err
	}
	if !decision.Allowed {
		return nil, ErrDenied
	}
	if decision.Profile == "" || decision.RootID != request.RootID || decision.Capability != request.Capability || decision.Backend == "" || decision.ServiceGeneration == "" {
		return nil, ErrInvalidContext
	}
	backend, err := d.registry.selectBackend(decision.Backend)
	if err != nil {
		return nil, err
	}
	binding := ExecutionContext{
		caller: caller, profile: decision.Profile, rootID: decision.RootID,
		capability: decision.Capability, backend: decision.Backend,
		serviceGeneration: decision.ServiceGeneration, correlation: request.Correlation,
	}
	return backend.Dispatch(ctx, binding, operation, d.adapter)
}

// PassthroughBackend represents current-process and service-account execution.
// It never impersonates: the OS adapter runs under the process's existing identity.
type PassthroughBackend struct{ id BackendID }

func NewPassthroughBackend(id BackendID) (PassthroughBackend, error) {
	if id != BackendCurrentProcess && id != BackendServiceAccount {
		return PassthroughBackend{}, fmt.Errorf("%w: %q", ErrBackendUnavailable, id)
	}
	return PassthroughBackend{id: id}, nil
}

func (b PassthroughBackend) ID() BackendID { return b.id }

func (b PassthroughBackend) Dispatch(ctx context.Context, binding ExecutionContext, operation Operation, adapter OSAdapter) (any, error) {
	if binding.Backend() != b.id {
		return nil, ErrInvalidContext
	}
	return adapter.Execute(ctx, binding, operation)
}

// ServiceAccountBackend executes only roots explicitly granted to a dedicated,
// non-privileged service identity. Native service bootstrap remains responsible
// for creating the account and granting the corresponding OS ACLs.
type ServiceAccountBackend struct {
	identity EffectiveIdentity
	roots    map[string]struct{}
	audit    AuditSink
}

// NewServiceAccountBackend constructs the Variant A backend. An empty root
// list and privileged convenience identities fail closed at startup.
func NewServiceAccountBackend(identity EffectiveIdentity, grantedRoots []string, audit AuditSink) (*ServiceAccountBackend, error) {
	if identity.Principal == "" || identity.Privileged || audit == nil || len(grantedRoots) == 0 {
		return nil, fmt.Errorf("%w: invalid service-account configuration", ErrBackendUnavailable)
	}
	roots := make(map[string]struct{}, len(grantedRoots))
	for _, root := range grantedRoots {
		if root == "" {
			return nil, fmt.Errorf("%w: invalid service-account root", ErrBackendUnavailable)
		}
		if _, exists := roots[root]; exists {
			return nil, fmt.Errorf("%w: duplicate service-account root", ErrBackendUnavailable)
		}
		roots[root] = struct{}{}
	}
	return &ServiceAccountBackend{identity: identity, roots: roots, audit: audit}, nil
}

func (*ServiceAccountBackend) ID() BackendID { return BackendServiceAccount }

func (b *ServiceAccountBackend) Dispatch(ctx context.Context, binding ExecutionContext, operation Operation, adapter OSAdapter) (any, error) {
	if b == nil || binding.Backend() != BackendServiceAccount || operation == nil || adapter == nil {
		return nil, ErrInvalidContext
	}
	event := AuditEvent{
		CallerPrincipal: binding.Caller().Principal(), EffectivePrincipal: b.identity.Principal,
		Backend: BackendServiceAccount, RootID: binding.RootID(), Capability: binding.Capability(),
		Operation: operation.Name(), Correlation: binding.Correlation(), ServiceGeneration: binding.ServiceGeneration(),
	}
	if _, granted := b.roots[binding.RootID()]; !granted {
		event.Result = "denied_root"
		b.audit.Record(ctx, event)
		return nil, ErrDenied
	}
	result, err := adapter.Execute(ctx, binding, operation)
	switch {
	case err == nil:
		event.Result = "allowed"
	case errors.Is(err, fs.ErrPermission):
		event.Result = "denied_os_permission"
		result = nil
		err = ErrDenied
	default:
		event.Result = "operation_failed"
	}
	b.audit.Record(ctx, event)
	return result, err
}
