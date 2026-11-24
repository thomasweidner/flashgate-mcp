package execution

import (
	"context"
	"crypto/rand"
	"reflect"
	"slices"
	"time"

	"github.com/thomasweidner/flashgate-mcp/internal/capability"
	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/resource"
)

type OperationID string
type PolicyID string
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
	ID       BackendID
	Identity Identity
	Backend  Backend[C, R]
}

type Dispatcher[C, R any] struct {
	authorizer authorizer[C]
	policyID   PolicyID
	selected   Registration[C, R]
}

func NewDispatcher[C, R any](runtime *Runtime, profile config.EffectiveProfile, selected BackendID, routes Routes[C], registrations ...Registration[C, R]) (*Dispatcher[C, R], error) {
	authorizer, err := newAuthorizer(runtime, profile, routes)
	if err != nil {
		return nil, err
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
	return &Dispatcher[C, R]{authorizer: authorizer, policyID: PolicyID(rand.Text()), selected: backend}, nil
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
	return ctx.Err()
}

type Completion[R any] struct {
	binding *Binding
	value   R
}

func Complete[R any](binding *Binding, value R) Completion[R] {
	return Completion[R]{binding: binding, value: value}
}

func (d *Dispatcher[C, R]) Dispatch(ctx context.Context, request Request, command C, admission func() error) (R, error) {
	var zero R
	if d == nil {
		return zero, ErrMismatch
	}
	declaration, resources, err := d.authorizer.authorize(ctx, request, command)
	if err != nil {
		return zero, err
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
		request: request, profile: d.authorizer.profile, policyID: d.policyID,
		key: declaration.Key, operation: declaration.Operation, resources: resources,
		backend: backend.ID, effective: backend.Identity, deadline: deadline,
	}
	if err := binding.Verify(ctx, backend.ID, backend.Identity, d.authorizer.runtime.generation); err != nil {
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
	completion, err := backend.Backend.Execute(ctx, Invocation[C]{binding: binding, command: command, admission: admission})
	if err != nil {
		return zero, err
	}
	if completion.binding != binding {
		return zero, ErrMismatch
	}
	if err := binding.Verify(ctx, backend.ID, backend.Identity, d.authorizer.runtime.generation); err != nil {
		return zero, err
	}
	return completion.value, nil
}
