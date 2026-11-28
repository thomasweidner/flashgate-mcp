package execution

import (
	"context"
	"crypto/rand"
	"reflect"
	"slices"
	"time"

	"github.com/thomasweidner/flashgate-mcp/internal/audit"
	"github.com/thomasweidner/flashgate-mcp/internal/capability"
	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/resource"
)

type OperationID string
type PolicyID string
type RepresentationID string
type ResourceKind string
type OperandRole string

const (
	Filesystem       ResourceKind = "filesystem"
	WorkingDirectory ResourceKind = "working-directory"
	Single           OperandRole  = "single"
	Source           OperandRole  = "source"
	Target           OperandRole  = "target"
)

// Proof revalidates native identity or confinement on an applicable operand.
// It is supplied by the trusted domain and is never inferred from its path.
type Proof interface{ Revalidate() error }

type Resource struct {
	Kind     ResourceKind
	Role     OperandRole
	ID       string
	Relative string
	Access   capability.Set
	Limits   resource.Limits
	Proof    Proof
}

// BoundResource holds the centrally approved operand and its declared right.
type BoundResource struct {
	Resource
	Requirement capability.Requirement
}

// Backend is registered at trusted composition. Its command and result are
// domain types. A future transport must authenticate the returned binding.
type Backend[C, R any] interface {
	Execute(context.Context, Invocation[C]) (Completion[R], error)
}

// WorkingDirectoryBackend requires a separate execution-side proof. A CWD
// operand establishes an authorized directory, not native launch capability.
type WorkingDirectoryBackend interface {
	ProveWorkingDirectory(context.Context, *Binding) error
}

type Registration[C, R any] struct {
	ID             BackendID
	Identity       Identity
	Backend        Backend[C, R]
	Representation RepresentationID
	Operations     []OperationKey
	Counters       func(C, R) audit.Counters
}

type Dispatcher[C, R any] struct {
	authorizer authorizer[C]
	selected   Registration[C, R]
	binding    *dispatchBinding
	catalog    CatalogRoutes
	auditing   *Auditing
	counters   func(C, R) audit.Counters
}

// Each dispatcher owns one resource resolver and selected execution route.
// Its identity distinguishes resource/support bindings with identical grants.
type dispatchBinding struct {
	id             PolicyID
	policy         *PolicyBinding
	declarations   *Declarations
	backend        BackendID
	representation RepresentationID
	operations     map[OperationKey]struct{}
	auditing       *Auditing
}

func NewDispatcher[C, R any](policy *PolicyBinding, selected BackendID, routes Routes[C], registrations ...Registration[C, R]) (*Dispatcher[C, R], error) {
	authorizer, err := newAuthorizer(policy, routes)
	if err != nil {
		return nil, err
	}
	for _, declaration := range authorizer.declarations.byKey {
		if declaration.DurableIntent && (routes.Audit == nil || !routes.Audit.required[declaration.Key]) {
			return nil, ErrMismatch
		}
	}
	registered := make(map[BackendID]Registration[C, R], len(registrations))
	for _, registration := range registrations {
		switch registration.ID {
		case CurrentProcess, ServiceAccount, UserWorker:
		default:
			return nil, ErrUnsupported
		}
		if _, duplicate := registered[registration.ID]; duplicate || nilBackend(registration.Backend) || !registration.Identity.valid() {
			return nil, ErrMismatch
		}
		registered[registration.ID] = registration
	}
	backend, found := registered[selected]
	if !found {
		return nil, ErrUnsupported
	}
	operations := make(map[OperationKey]struct{}, len(backend.Operations))
	for _, key := range backend.Operations {
		_, declared := authorizer.declarations.byKey[key]
		if _, duplicate := operations[key]; !declared || duplicate {
			return nil, ErrMismatch
		}
		operations[key] = struct{}{}
	}
	backend.Operations = nil
	binding := &dispatchBinding{
		id: PolicyID(rand.Text()), policy: policy, declarations: authorizer.declarations,
		backend: backend.ID, representation: backend.Representation, operations: operations,
		auditing: routes.Audit,
	}
	return &Dispatcher[C, R]{
		authorizer: authorizer, selected: backend, binding: binding,
		catalog:  CatalogRoutes{policy: policy, routes: map[CatalogRouteID]CatalogRoute{"": {binding}}},
		auditing: routes.Audit, counters: backend.Counters,
	}, nil
}

func nilBackend(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

// Binding carries separate caller, policy, operation, backend and generation
// identities. Pointer identity is the private in-process completion binding.
// BL-238 will define its authenticated equivalent for a worker process.
type Binding struct {
	request   Request
	profile   config.EffectiveProfile
	policyID  PolicyID
	key       OperationKey
	operation capability.Operation
	resources []BoundResource
	backend   BackendID
	effective Identity
	deadline  time.Time
}

func (b *Binding) Request() Request                 { return b.request }
func (b *Binding) Profile() config.EffectiveProfile { return b.profile }
func (b *Binding) Operation() capability.Operation  { return b.operation }
func (b *Binding) Key() OperationKey                { return b.key }
func (b *Binding) Resources() []BoundResource       { return slices.Clone(b.resources) }
func (b *Binding) Backend() BackendID               { return b.backend }
func (b *Binding) EffectiveIdentity() Identity      { return b.effective }
func (b *Binding) Generation() string               { return b.request.session.runtime.generation }
func (b *Binding) PolicyID() PolicyID               { return b.policyID }
func (b *Binding) Deadline() time.Time              { return b.deadline }

// Verify checks the execution-side binding and revalidates native proofs.
// An unsupported native proof must deny before the effect or result release.
func (b *Binding) Verify(ctx context.Context, backend BackendID, identity Identity, generation string) error {
	if b == nil || ctx == nil || b.request.session.runtime == nil || backend != b.backend ||
		identity != b.effective || generation != b.Generation() {
		return ErrMismatch
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !b.deadline.IsZero() && !time.Now().Before(b.deadline) {
		return context.DeadlineExceeded
	}
	for _, operand := range b.resources {
		if operand.Kind == WorkingDirectory && operand.Proof == nil {
			return ErrUnsupported
		}
		if operand.Proof != nil {
			if err := operand.Proof.Revalidate(); err != nil {
				return err
			}
		}
	}
	return nil
}

type Invocation[C any] struct {
	binding   *Binding
	command   C
	admission func() error
	audit     *auditSpan
}

func (i Invocation[C]) Binding() *Binding { return i.binding }
func (i Invocation[C]) Command() C        { return i.command }

// AdmitEffect is the in-process result-budget guard at the native mutation
// boundary. Future delegated mutation requires its own enforced handshake.
func (i Invocation[C]) AdmitEffect(ctx context.Context) error {
	if i.binding == nil {
		return ErrMismatch
	}
	if err := i.binding.Verify(ctx, i.binding.backend, i.binding.effective, i.binding.Generation()); err != nil {
		return err
	}
	if i.admission != nil {
		if err := i.admission(); err != nil {
			return err
		}
	}
	if err := i.audit.start(ctx, i.binding); err != nil {
		return err
	}
	return i.binding.Verify(ctx, i.binding.backend, i.binding.effective, i.binding.Generation())
}

type Completion[R any] struct {
	binding *Binding
	value   R
}

func Complete[R any](binding *Binding, value R) Completion[R] {
	return Completion[R]{binding: binding, value: value}
}

func (d *Dispatcher[C, R]) Dispatch(ctx context.Context, request Request, command C, admission func() error) (returned R, failure error) {
	var zero R
	if d == nil || d.binding == nil || !d.acceptsInvocation(ctx, request) {
		return zero, ErrMismatch
	}
	if request.invocation != nil && d.auditing != nil {
		request.invocation.attempted.Store(true)
	}
	declaration, resources, err := d.authorizer.authorize(ctx, request, command, d.binding.operations)
	span := d.auditing.decision(d.binding, request, declaration, resources, err)
	if err != nil {
		return zero, err
	}
	var observedResult R
	var completed bool
	if span != nil {
		defer func() {
			var counters audit.Counters
			if d.counters != nil {
				counters = d.counters(command, observedResult)
			}
			reason := auditReason(failure, false)
			if failure == nil && !completed {
				reason = audit.Uncertain
			}
			span.finish(reason, counters)
		}()
	}
	var requiresCWD bool
	for _, operand := range resources {
		requiresCWD = requiresCWD || operand.Kind == WorkingDirectory
	}
	backend := d.selected
	if admission != nil && backend.ID != CurrentProcess {
		return zero, ErrUnsupported
	}
	deadline, _ := ctx.Deadline()
	binding := &Binding{
		request: request, profile: d.binding.policy.profile, policyID: d.binding.id,
		key: declaration.Key, operation: declaration.Operation, resources: resources,
		backend: backend.ID, effective: backend.Identity, deadline: deadline,
	}
	if err := binding.Verify(ctx, backend.ID, backend.Identity, d.binding.policy.runtime.generation); err != nil {
		return zero, err
	}
	if requiresCWD {
		prover, ok := backend.Backend.(WorkingDirectoryBackend)
		if !ok {
			return zero, ErrUnsupported
		}
		if err := prover.ProveWorkingDirectory(ctx, binding); err != nil {
			return zero, err
		}
	}
	completion, err := backend.Backend.Execute(ctx, Invocation[C]{binding: binding, command: command, admission: admission, audit: span})
	if completion.binding == binding {
		observedResult = completion.value
	}
	if observe, _ := ctx.Value(operationObserverKey{}).(func(OperationKey)); observe != nil {
		observe(binding.Key())
	}
	if err != nil {
		return zero, err
	}
	if completion.binding != binding {
		return zero, ErrMismatch
	}
	if err := binding.Verify(ctx, backend.ID, backend.Identity, d.binding.policy.runtime.generation); err != nil {
		return zero, err
	}
	completed = true
	return completion.value, nil
}
