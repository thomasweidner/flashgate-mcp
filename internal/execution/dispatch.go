package execution

import (
	"context"
	"crypto/rand"
	"reflect"
	"slices"
	"strings"
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
	Kind        ResourceKind
	Role        OperandRole
	ID          string
	Relative    string
	Requirement capability.Requirement
	Access      capability.Set
	Limits      resource.Limits
	Proof       Proof
}

// Plan is derived from the typed command by its trusted declaration. The
// application cannot take a capability declaration from the wire.
type Plan struct {
	Operation   OperationID
	Requirement capability.Operation
	Resources   []Resource
}

// Authorizer is the single pre-dispatch transition. The domain resolves
// declarations and operand grants from this runtime's effective snapshots.
// BL-159 supplies the shared central policy decision at this boundary.
type Authorizer[C any] func(context.Context, Request, C) (Plan, error)

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
	runtime   *Runtime
	profile   config.EffectiveProfile
	policyID  PolicyID
	selected  Registration[C, R]
	authorize Authorizer[C]
}

func NewDispatcher[C, R any](runtime *Runtime, profile config.EffectiveProfile, selected BackendID, authorize Authorizer[C], registrations ...Registration[C, R]) (*Dispatcher[C, R], error) {
	if runtime == nil || runtime.generation == "" || profile.ID() == "" || authorize == nil {
		return nil, ErrMismatch
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
	return &Dispatcher[C, R]{runtime: runtime, profile: profile, policyID: PolicyID(rand.Text()), selected: backend, authorize: authorize}, nil
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
	plan      Plan
	backend   BackendID
	effective Identity
	deadline  time.Time
}

func (b *Binding) Request() Request                 { return b.request }
func (b *Binding) Profile() config.EffectiveProfile { return b.profile }
func (b *Binding) Operation() capability.Operation  { return b.plan.Requirement }
func (b *Binding) Name() OperationID                { return b.plan.Operation }
func (b *Binding) Resources() []Resource            { return slices.Clone(b.plan.Resources) }
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
	for _, operand := range b.plan.Resources {
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
	if d == nil || ctx == nil || request.session.runtime != d.runtime || request.correlation == 0 {
		return zero, ErrMismatch
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	plan, err := d.authorize(ctx, request, command)
	if err != nil {
		return zero, err
	}
	policy := d.profile.Policy()
	if strings.TrimSpace(string(plan.Operation)) == "" || !policy.Allows(plan.Requirement) {
		return zero, ErrDenied
	}
	var requiresCWD bool
	for _, operand := range plan.Resources {
		if operand.Kind == "" || operand.Role == "" || operand.ID == "" ||
			!operand.Requirement.SatisfiedBy(policy.Grants()) || !operand.Requirement.SatisfiedBy(operand.Access) {
			return zero, ErrDenied
		}
		requiresCWD = requiresCWD || operand.Kind == WorkingDirectory
	}
	backend := d.selected
	if admission != nil && backend.ID != CurrentProcess {
		return zero, ErrUnsupported
	}
	plan.Resources = slices.Clone(plan.Resources)
	deadline, _ := ctx.Deadline()
	binding := &Binding{request: request, profile: d.profile, policyID: d.policyID, plan: plan, backend: backend.ID, effective: backend.Identity, deadline: deadline}
	if err := binding.Verify(ctx, backend.ID, backend.Identity, d.runtime.generation); err != nil {
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
	if err := binding.Verify(ctx, backend.ID, backend.Identity, d.runtime.generation); err != nil {
		return zero, err
	}
	return completion.value, nil
}
