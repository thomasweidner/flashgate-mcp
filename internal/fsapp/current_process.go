package fsapp

import (
	"context"

	"github.com/thomasweidner/flashgate-mcp/internal/execution"
	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/roots"
)

type currentProcess struct {
	roots      *roots.Registry
	operations fs.Operations
	identity   execution.Identity
	generation string
}

func (b currentProcess) Execute(ctx context.Context, invocation execution.Invocation[command]) (execution.Completion[result], error) {
	binding := invocation.Binding()
	if err := binding.Verify(ctx, execution.CurrentProcess, b.identity, b.generation); err != nil {
		return execution.Completion[result]{}, err
	}
	if err := execution.CheckCurrentProcess(b.identity); err != nil {
		return execution.Completion[result]{}, err
	}
	c := invocation.Command()
	if !b.matches(binding, c) || !b.roots.MatchesProfile(binding.Profile().Policy()) {
		return execution.Completion[result]{}, execution.ErrMismatch
	}
	value, err := b.execute(c, func() error {
		if err := execution.CheckCurrentProcess(b.identity); err != nil {
			return err
		}
		return invocation.AdmitEffect(ctx)
	})
	if err != nil {
		return execution.Completion[result]{}, err
	}
	if err := execution.CheckCurrentProcess(b.identity); err != nil {
		return execution.Completion[result]{}, err
	}
	return execution.Complete(binding, value), nil
}

func (b currentProcess) matches(binding *execution.Binding, c command) bool {
	action, requested, err := c.declaration()
	bound := binding.Resources()
	if err != nil || binding.Name() != c.name || len(bound) != len(requested) {
		return false
	}
	operation := b.operations[action]
	if binding.Operation().Risk() != operation.Risk() || !binding.Operation().Requirement().Equal(operation.Requirement()) {
		return false
	}
	for i, operand := range requested {
		resource := bound[i]
		access, limits, err := b.roots.AccessForRoot(operand.root)
		if resource.Kind != execution.Filesystem || resource.Role != operand.role ||
			resource.ID != string(operand.root) || resource.Relative != operand.path ||
			err != nil || !resource.Access.Equal(access) || resource.Limits != limits ||
			!resource.Requirement.Equal(b.operations[operand.action].Requirement()) {
			return false
		}
	}
	return true
}

func (b currentProcess) execute(c command, admit func() error) (value result, err error) {
	path := fs.Operand{RootID: c.path.root, Path: c.path.path, AdmitEffect: admit}
	target := fs.Operand{RootID: c.target.root, Path: c.target.path, AdmitEffect: admit}
	switch c.name {
	case listDirectory:
		value.entries, err = b.roots.List(path)
	case readFile:
		value.content, err = b.roots.Read(path, c.maxBytes)
	case pathInfo:
		value.info, err = b.roots.Stat(path)
	case writeFile:
		err = b.roots.Write(path, c.content, c.overwrite)
	case makeDirectory:
		value.created, err = b.roots.Mkdir(path)
	case deletePath:
		err = b.roots.Delete(path, c.recursive)
	case movePath:
		err = b.roots.Move(path, target, c.overwrite)
	case copyPath:
		err = b.roots.Copy(path, target, c.overwrite)
	default:
		err = execution.ErrUnsupported
	}
	return value, err
}
