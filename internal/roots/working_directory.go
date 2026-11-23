package roots

import (
	"errors"

	"github.com/thomasweidner/flashgate-mcp/internal/capability"
	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/fs"
)

var (
	ErrWorkingDirectoryDenied     = errors.New("process working directory denied")
	ErrProcessDispatchUnavailable = errors.New("process effect enforcement unavailable")
)

// WorkingDirectoryOperand selects a root by exact ID and supplies a relative
// directory path. "." explicitly selects that root.
type WorkingDirectoryOperand struct {
	RootID       config.RootID
	RelativePath string
}

// WorkingDirectory is a confined selection bound to the exact authorized root,
// effective policy and observed directory identity. It carries no launch grant.
type WorkingDirectory struct {
	operand WorkingDirectoryOperand
	binding fs.DirectoryBinding
}

func (d WorkingDirectory) Operand() WorkingDirectoryOperand { return d.operand }

// Revalidate checks that the bound directory is still the authorized object.
// A later execution owner must prove enforcement at the native effect boundary.
func (d WorkingDirectory) Revalidate() error {
	if err := d.binding.Revalidate(); err != nil {
		return ErrWorkingDirectoryDenied
	}
	return nil
}

// CheckWorkingDirectory applies the caller's declared functional operation and
// per-operand resource requirements in this registry's capability snapshot.
// Neither process rights nor filesystem rights imply permission to select a CWD.
func (r *Registry) CheckWorkingDirectory(operand WorkingDirectoryOperand, operation capability.Operation, access capability.Requirement) (WorkingDirectory, error) {
	root, ok := r.entries[operand.RootID]
	if !ok {
		return WorkingDirectory{}, fs.ErrUnknownRoot
	}
	required := operation.Requirement()
	if !root.allowProcessCWD ||
		!(required.Requires(capability.CommandsRun) || required.Requires(capability.ProcessesManage)) ||
		!r.profile.Allows(operation) || !access.SatisfiedBy(r.profile.Grants()) || !access.SatisfiedBy(root.access) {
		return WorkingDirectory{}, fs.ErrRootAccessDenied
	}
	binding, err := root.backend.BindDirectory(operand.RelativePath)
	if err != nil {
		return WorkingDirectory{}, ErrWorkingDirectoryDenied
	}
	return WorkingDirectory{operand: operand, binding: binding}, nil
}

// PreflightProcessDispatch is a non-executing dispatch boundary. The trusted
// caller supplies its enforcement check and a non-executing sink. A future
// execution owner must enforce each actual effect through its native backend.
func (r *Registry) PreflightProcessDispatch(operand WorkingDirectoryOperand, operation capability.Operation, access capability.Requirement,
	proveEnforcement func(WorkingDirectory) bool, dispatch func(WorkingDirectory) error,
) error {
	directory, err := r.CheckWorkingDirectory(operand, operation, access)
	if err != nil {
		return err
	}
	if proveEnforcement == nil || dispatch == nil || !proveEnforcement(directory) {
		return ErrProcessDispatchUnavailable
	}
	if err := directory.Revalidate(); err != nil {
		return err
	}
	return dispatch(directory)
}
