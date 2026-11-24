package execution

import (
	"context"
	"slices"
	"strings"

	"github.com/thomasweidner/flashgate-mcp/internal/capability"
	"github.com/thomasweidner/flashgate-mcp/internal/config"
)

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
	Key       OperationKey
	Operation capability.Operation
	Operands  []OperandRequirement
}

// Routes derives a declaration key and resource bindings from a typed command.
// Key is a pure selection. Resolve may consult the trusted policy snapshot.
type Routes[C any] struct {
	Declarations []Declaration
	Key          func(C) OperationKey
	Resolve      func(context.Context, C) ([]Resource, error)
}

type authorizer[C any] struct {
	runtime      *Runtime
	profile      config.EffectiveProfile
	declarations map[OperationKey]Declaration
	key          func(C) OperationKey
	resolve      func(context.Context, C) ([]Resource, error)
}

func newAuthorizer[C any](runtime *Runtime, profile config.EffectiveProfile, routes Routes[C]) (authorizer[C], error) {
	if runtime == nil || runtime.generation == "" || profile.ID() == "" ||
		routes.Key == nil || routes.Resolve == nil || len(routes.Declarations) == 0 {
		return authorizer[C]{}, ErrMismatch
	}
	declarations := make(map[OperationKey]Declaration, len(routes.Declarations))
	for _, declaration := range routes.Declarations {
		if _, duplicate := declarations[declaration.Key]; duplicate ||
			strings.TrimSpace(string(declaration.Key.Name)) == "" ||
			strings.TrimSpace(string(declaration.Key.Variant)) == "" {
			return authorizer[C]{}, ErrMismatch
		}
		operands := make([]capability.Requirement, len(declaration.Operands))
		for i, operand := range declaration.Operands {
			if operand.Kind == "" || operand.Role == "" {
				return authorizer[C]{}, ErrMismatch
			}
			operands[i] = operand.Requirement
		}
		operation, err := profile.Catalog().CombineOperation(declaration.Operation, operands...)
		if err != nil {
			return authorizer[C]{}, ErrMismatch
		}
		declaration.Operation = operation
		declaration.Operands = slices.Clone(declaration.Operands)
		declarations[declaration.Key] = declaration
	}
	return authorizer[C]{
		runtime: runtime, profile: profile, declarations: declarations,
		key: routes.Key, resolve: routes.Resolve,
	}, nil
}

func (a authorizer[C]) authorize(ctx context.Context, request Request, command C) (Declaration, []BoundResource, error) {
	if ctx == nil || request.session.runtime != a.runtime || request.correlation == 0 ||
		request.Caller().ID() == "" || request.Caller().Origin() == "" {
		return Declaration{}, nil, ErrMismatch
	}
	if err := ctx.Err(); err != nil {
		return Declaration{}, nil, err
	}
	declaration, known := a.declarations[a.key(command)]
	if !known || !a.profile.Policy().Allows(declaration.Operation) {
		return Declaration{}, nil, ErrDenied
	}
	resources, err := a.resolve(ctx, command)
	if err != nil {
		return Declaration{}, nil, err
	}
	if len(resources) != len(declaration.Operands) {
		return Declaration{}, nil, ErrDenied
	}
	bound := make([]BoundResource, len(resources))
	for i, resource := range resources {
		operand := declaration.Operands[i]
		if resource.Kind != operand.Kind || resource.Role != operand.Role || resource.ID == "" ||
			!operand.Requirement.SatisfiedBy(resource.Access) {
			return Declaration{}, nil, ErrDenied
		}
		bound[i] = BoundResource{Resource: resource, Requirement: operand.Requirement}
	}
	return declaration, bound, nil
}
