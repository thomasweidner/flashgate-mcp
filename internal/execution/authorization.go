package execution

import (
	"context"
	"slices"
	"strings"

	"github.com/thomasweidner/flashgate-mcp/internal/capability"
)

type policyDenial struct {
	risk bool
}

func (policyDenial) Error() string { return ErrDenied.Error() }
func (policyDenial) Unwrap() error { return ErrDenied }

type VariantID string

const DefaultVariant VariantID = "default"

type OperationKey struct {
	Name    OperationID
	Variant VariantID
}

type OperandRequirement struct {
	Kind        ResourceKind
	Role        OperandRole
	Requirement capability.Requirement
}

// Declaration is supplied by the owning domain at trusted composition.
// Each required resource has a fixed position, kind, role and requirement.
type Declaration struct {
	Key           OperationKey
	Operation     capability.Operation
	Operands      []OperandRequirement
	DurableIntent bool
}

// Routes derives a declaration key and resource bindings from a typed command.
// Key is a pure selection. Resolve may consult the trusted policy snapshot.
type Routes[C any] struct {
	Audit        *Auditing
	Declarations []Declaration
	Shared       *Declarations
	Key          func(C) OperationKey
	Resolve      func(context.Context, C) ([]Resource, error)
}

// OperationKeys returns the declarations known to this composition. Backend
// composition selects the operations it has established it can implement.
func (r Routes[C]) OperationKeys() []OperationKey {
	if r.Shared != nil {
		return slices.Clone(r.Shared.keys)
	}
	keys := make([]OperationKey, 0, len(r.Declarations))
	for _, declaration := range r.Declarations {
		keys = append(keys, declaration.Key)
	}
	return keys
}

type authorizer[C any] struct {
	policy       *PolicyBinding
	declarations *Declarations
	key          func(C) OperationKey
	resolve      func(context.Context, C) ([]Resource, error)
}

// Declarations holds validated operations independently of any command type.
// Multiple domain dispatchers can use the same trusted declaration source.
type Declarations struct {
	catalog capability.Catalog
	byKey   map[OperationKey]Declaration
	keys    []OperationKey
}

func BindDeclarations(catalog capability.Catalog, definitions ...Declaration) (*Declarations, error) {
	if len(definitions) == 0 {
		return nil, ErrMismatch
	}
	declarations := &Declarations{
		catalog: catalog,
		byKey:   make(map[OperationKey]Declaration, len(definitions)),
		keys:    make([]OperationKey, 0, len(definitions)),
	}
	for _, declaration := range definitions {
		if _, duplicate := declarations.byKey[declaration.Key]; duplicate ||
			strings.TrimSpace(string(declaration.Key.Name)) == "" ||
			strings.TrimSpace(string(declaration.Key.Variant)) == "" {
			return nil, ErrMismatch
		}
		operands := make([]capability.Requirement, len(declaration.Operands))
		for i, operand := range declaration.Operands {
			if operand.Kind == "" || operand.Role == "" {
				return nil, ErrMismatch
			}
			operands[i] = operand.Requirement
		}
		operation, err := catalog.CombineOperation(declaration.Operation, operands...)
		if err != nil {
			return nil, ErrMismatch
		}
		declaration.Operation = operation
		declaration.Operands = slices.Clone(declaration.Operands)
		declarations.byKey[declaration.Key] = declaration
		declarations.keys = append(declarations.keys, declaration.Key)
	}
	return declarations, nil
}

func newAuthorizer[C any](policy *PolicyBinding, routes Routes[C]) (authorizer[C], error) {
	if policy == nil || policy.runtime == nil ||
		routes.Key == nil || routes.Resolve == nil {
		return authorizer[C]{}, ErrMismatch
	}
	declarations := routes.Shared
	if declarations == nil {
		var err error
		declarations, err = BindDeclarations(policy.profile.Catalog(), routes.Declarations...)
		if err != nil {
			return authorizer[C]{}, err
		}
	} else if len(routes.Declarations) != 0 || len(declarations.keys) == 0 || declarations.catalog != policy.profile.Catalog() {
		return authorizer[C]{}, ErrMismatch
	}
	return authorizer[C]{
		policy: policy, declarations: declarations,
		key: routes.Key, resolve: routes.Resolve,
	}, nil
}

func (a authorizer[C]) accepts(request Request) bool {
	return a.policy.accepts(request)
}

func (p *PolicyBinding) accepts(request Request) bool {
	return p != nil && request.session.runtime == p.runtime && request.session.policy == p &&
		request.correlation != 0 && request.Caller().ID() != "" && request.Caller().Origin() != ""
}

func (a authorizer[C]) authorize(ctx context.Context, request Request, command C, support map[OperationKey]struct{}) (Declaration, []BoundResource, error) {
	if ctx == nil || !a.accepts(request) {
		return Declaration{}, nil, ErrMismatch
	}
	if err := ctx.Err(); err != nil {
		return Declaration{}, nil, err
	}
	declaration, known := a.declarations.byKey[a.key(command)]
	if !known {
		return Declaration{}, nil, ErrDenied
	}
	if request.invocation != nil && !(CatalogInvocation{request}).Allows(declaration.Key) {
		return Declaration{}, nil, ErrDenied
	}
	policy := a.policy.profile.Policy()
	if !policy.Allows(declaration.Operation) {
		return declaration, nil, policyDenial{risk: declaration.Operation.Risk() > policy.Ceiling()}
	}
	if _, supported := support[declaration.Key]; !supported {
		return declaration, nil, ErrUnsupported
	}
	resources, err := a.resolve(ctx, command)
	if err != nil {
		return declaration, nil, err
	}
	if len(resources) != len(declaration.Operands) {
		return declaration, nil, ErrDenied
	}
	bound := make([]BoundResource, len(resources))
	for i, resource := range resources {
		operand := declaration.Operands[i]
		if resource.Kind != operand.Kind || resource.Role != operand.Role || resource.ID == "" ||
			!operand.Requirement.SatisfiedBy(resource.Access) {
			return declaration, nil, ErrDenied
		}
		bound[i] = BoundResource{Resource: resource, Requirement: operand.Requirement}
	}
	return declaration, bound, nil
}
