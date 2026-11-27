// Package fsapp adapts typed filesystem commands to the common execution
// boundary. The root registry owns native confinement and policy enforcement.
package fsapp

import (
	"context"
	"crypto/rand"
	"errors"
	"math"

	"github.com/thomasweidner/flashgate-mcp/internal/audit"
	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/execution"
	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/resource"
	"github.com/thomasweidner/flashgate-mcp/internal/roots"
)

type Service struct {
	roots      *roots.Registry
	session    execution.Session
	dispatcher *execution.Dispatcher[command, result]
}

// NewDirect composes the only active product backend with its protected roots.
func NewDirect(registry *roots.Registry, profile config.EffectiveProfile) (*Service, error) {
	return newDirect(registry, profile, "", nil, nil)
}

func NewAuditedDirect(registry *roots.Registry, profile config.EffectiveProfile, generation string, recorder *audit.Recorder, selected []string) (*Service, error) {
	return newDirect(registry, profile, generation, recorder, selected)
}

func newDirect(registry *roots.Registry, profile config.EffectiveProfile, generation string, recorder *audit.Recorder, selected []string) (*Service, error) {
	if registry == nil || !registry.MatchesProfile(profile.Policy()) {
		return nil, execution.ErrMismatch
	}
	if generation == "" {
		generation = rand.Text()
	}
	runtime, err := execution.NewRuntime(generation)
	if err != nil {
		return nil, err
	}
	session, identity, err := execution.BindDirectSTDIO(runtime)
	if err != nil {
		return nil, err
	}
	policy, err := execution.NewPolicyBinding(runtime, profile)
	if err != nil {
		return nil, err
	}
	session, err = session.WithPolicy(policy)
	if err != nil {
		return nil, err
	}
	operations, err := fs.BindOperations(profile.Catalog())
	if err != nil {
		return nil, err
	}
	routes, err := bindRoutes(registry, operations)
	if err != nil {
		return nil, err
	}
	routes.Audit, err = execution.BindAuditing(recorder, selected, routes.Declarations)
	if err != nil {
		return nil, config.NewError(config.CategoryInvalidConfig, err)
	}
	backend := currentProcess{roots: registry, operations: operations, identity: identity, generation: generation}
	dispatcher, err := execution.NewDispatcher(policy, execution.CurrentProcess, routes,
		execution.Registration[command, result]{
			ID: execution.CurrentProcess, Identity: identity, Backend: backend, Operations: routes.OperationKeys(),
			Counters: auditCounters,
		})
	if err != nil {
		return nil, err
	}
	return &Service{roots: registry, session: session, dispatcher: dispatcher}, nil
}

func auditCounters(c command, value result) audit.Counters {
	switch c.name {
	case readFile:
		return audit.Counters{RequestedBytes: uint64(max(0, c.maxBytes)), ReadBytes: value.readBytes}
	case listDirectory:
		return audit.Counters{Entries: uint64(len(value.entries))}
	case writeFile:
		return audit.Counters{RequestedBytes: uint64(len(c.content)), WrittenBytes: value.writtenBytes}
	case copyPath:
		return audit.Counters{ReadBytes: value.readBytes, WrittenBytes: value.writtenBytes}
	case makeDirectory:
		if value.created {
			return audit.Counters{Entries: 1}
		}
	}
	return audit.Counters{}
}

func bindRoutes(registry *roots.Registry, operations fs.Operations) (execution.Routes[command], error) {
	variants := [...]command{
		{name: listDirectory}, {name: readFile}, {name: pathInfo},
		{name: writeFile}, {name: writeFile, overwrite: true}, {name: makeDirectory},
		{name: deletePath}, {name: deletePath, recursive: true},
		{name: copyPath}, {name: copyPath, overwrite: true},
		{name: movePath}, {name: movePath, overwrite: true},
	}
	declarations := make([]execution.Declaration, 0, len(variants))
	for _, variant := range variants {
		action, requested, err := variant.declaration()
		if err != nil {
			return execution.Routes[command]{}, err
		}
		operands := make([]execution.OperandRequirement, 0, len(requested))
		for _, operand := range requested {
			operands = append(operands, execution.OperandRequirement{
				Kind: execution.Filesystem, Role: operand.role, Requirement: operations[operand.action].Requirement(),
			})
		}
		declarations = append(declarations, execution.Declaration{
			Key: variant.key(), Operation: operations[action], Operands: operands,
		})
	}
	return execution.Routes[command]{Declarations: declarations, Key: command.key, Resolve: registryResources(registry)}, nil
}

func registryResources(registry *roots.Registry) func(context.Context, command) ([]execution.Resource, error) {
	return func(_ context.Context, c command) ([]execution.Resource, error) {
		_, requested, err := c.declaration()
		if err != nil {
			return nil, err
		}
		resources := make([]execution.Resource, 0, len(requested))
		for _, operand := range requested {
			access, limits, err := registry.AccessForRoot(operand.root)
			if err != nil {
				return nil, err
			}
			resources = append(resources, execution.Resource{
				Kind: execution.Filesystem, Role: operand.role, ID: string(operand.root), Relative: operand.path,
				Access: access, Limits: limits,
			})
		}
		if len(resources) == 2 && resources[0].ID != resources[1].ID {
			return nil, fs.ErrCrossRootOperation
		}
		return resources, nil
	}
}

// ForRequest binds the accepted ingress protocol and response allowance once.
// The returned interface runs every operation through the bound dispatcher.
func (s *Service) ForRequest(ctx context.Context, protocol execution.Protocol) (fs.FileSystem, error) {
	request, err := s.request(ctx, protocol)
	if err != nil {
		return nil, err
	}
	return filesystem{service: s, ctx: ctx, request: request}, nil
}

func (s *Service) request(ctx context.Context, protocol execution.Protocol) (execution.Request, error) {
	if ctx == nil {
		return execution.Request{}, execution.ErrMismatch
	}
	return s.session.Request(protocol, resource.ResultLimit(ctx, math.MaxInt64))
}

func (s *Service) CatalogContext(ctx context.Context, protocol execution.Protocol) (execution.CatalogContext, error) {
	request, err := s.request(ctx, protocol)
	if err != nil {
		return execution.CatalogContext{}, err
	}
	return s.dispatcher.CatalogContext(request)
}

func (s *Service) OperationKeys(operation execution.OperationID) []execution.OperationKey {
	return s.dispatcher.OperationKeys(operation)
}

// ForTool confines this handler to the variants admitted for its catalog entry.
// Every concrete operation still crosses Dispatch and the native guards.
func (s *Service) ForTool(ctx context.Context, invocation execution.CatalogInvocation) (fs.FileSystem, error) {
	if ctx == nil || !invocation.Matches(s.dispatcher.CatalogRoute()) {
		return nil, execution.ErrMismatch
	}
	return filesystem{service: s, ctx: ctx, request: invocation.Request()}, nil
}

type filesystem struct {
	service *Service
	ctx     context.Context
	request execution.Request
}

func (f filesystem) dispatch(c command, admission func() error) (result, error) {
	value, err := f.service.dispatcher.Dispatch(f.ctx, f.request, c, admission)
	if errors.Is(err, execution.ErrDenied) {
		err = fs.ErrRootAccessDenied
	}
	return value, err
}

func (f filesystem) LimitsForRoot(id config.RootID) (resource.Limits, error) {
	return f.service.roots.LimitsForRoot(id)
}

func (f filesystem) List(path fs.Operand) ([]fs.Entry, error) {
	value, err := f.dispatch(command{name: listDirectory, path: fileOperand(path)}, nil)
	return value.entries, err
}

func (f filesystem) Read(path fs.Operand, maxBytes int64) ([]byte, error) {
	value, err := f.dispatch(command{name: readFile, path: fileOperand(path), maxBytes: maxBytes}, nil)
	return value.content, err
}

func (f filesystem) Stat(path fs.Operand) (fs.Metadata, error) {
	value, err := f.dispatch(command{name: pathInfo, path: fileOperand(path)}, nil)
	if err == nil && value.missing {
		return fs.Metadata{}, fs.ErrNotFound
	}
	return value.info, err
}

func (f filesystem) Write(path fs.Operand, content []byte, overwrite bool) error {
	_, err := f.dispatch(command{name: writeFile, path: fileOperand(path), content: content, overwrite: overwrite}, path.AdmitEffect)
	return err
}

func (f filesystem) Mkdir(path fs.Operand) (bool, error) {
	value, err := f.dispatch(command{name: makeDirectory, path: fileOperand(path)}, path.AdmitEffect)
	return value.created, err
}

func (f filesystem) Delete(path fs.Operand, recursive bool) error {
	_, err := f.dispatch(command{name: deletePath, path: fileOperand(path), recursive: recursive}, path.AdmitEffect)
	return err
}

func (f filesystem) Move(source, target fs.Operand, overwrite bool) error {
	_, err := f.dispatch(command{name: movePath, path: fileOperand(source), target: fileOperand(target), overwrite: overwrite}, target.AdmitEffect)
	return err
}

func (f filesystem) Copy(source, target fs.Operand, overwrite bool) error {
	_, err := f.dispatch(command{name: copyPath, path: fileOperand(source), target: fileOperand(target), overwrite: overwrite}, target.AdmitEffect)
	return err
}
