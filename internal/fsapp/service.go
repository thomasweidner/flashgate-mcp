// Package fsapp adapts typed filesystem commands to the common execution
// boundary. The root registry owns native confinement and policy enforcement.
package fsapp

import (
	"context"
	"crypto/rand"
	"errors"
	"math"

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
	if registry == nil || !registry.MatchesProfile(profile.Policy()) {
		return nil, execution.ErrMismatch
	}
	generation := rand.Text()
	runtime, err := execution.NewRuntime(generation)
	if err != nil {
		return nil, err
	}
	session, identity, err := execution.BindDirectSTDIO(runtime)
	if err != nil {
		return nil, err
	}
	operations, err := fs.BindOperations(profile.Catalog())
	if err != nil {
		return nil, err
	}
	backend := currentProcess{roots: registry, operations: operations, identity: identity, generation: generation}
	dispatcher, err := execution.NewDispatcher(runtime, profile, execution.CurrentProcess, authorize(registry, operations),
		execution.Registration[command, result]{ID: execution.CurrentProcess, Identity: identity, Backend: backend})
	if err != nil {
		return nil, err
	}
	return &Service{roots: registry, session: session, dispatcher: dispatcher}, nil
}

func authorize(registry *roots.Registry, operations fs.Operations) execution.Authorizer[command] {
	return func(_ context.Context, _ execution.Request, command command) (execution.Plan, error) {
		action, requested, err := command.declaration()
		if err != nil {
			return execution.Plan{}, err
		}
		resources := make([]execution.Resource, 0, len(requested))
		for _, operand := range requested {
			access, limits, err := registry.AccessForRoot(operand.root)
			if err != nil {
				return execution.Plan{}, err
			}
			resources = append(resources, execution.Resource{
				Kind: execution.Filesystem, Role: operand.role, ID: string(operand.root), Relative: operand.path,
				Requirement: operations[operand.action].Requirement(), Access: access, Limits: limits,
			})
		}
		if len(resources) == 2 && resources[0].ID != resources[1].ID {
			return execution.Plan{}, fs.ErrCrossRootOperation
		}
		return execution.Plan{Operation: command.name, Requirement: operations[action], Resources: resources}, nil
	}
}

// ForRequest binds the accepted ingress protocol and response allowance once.
// The returned interface runs every operation through the bound dispatcher.
func (s *Service) ForRequest(ctx context.Context, protocol execution.Protocol) (fs.FileSystem, error) {
	if ctx == nil {
		return nil, execution.ErrMismatch
	}
	request, err := s.session.Request(protocol, resource.ResultLimit(ctx, math.MaxInt64))
	if err != nil {
		return nil, err
	}
	return filesystem{service: s, ctx: ctx, request: request}, nil
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
