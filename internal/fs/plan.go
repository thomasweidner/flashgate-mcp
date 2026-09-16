package fs

import (
	"context"
	"errors"
)

// ErrInvalidPlan reports a structurally invalid or over-limit plan.
var ErrInvalidPlan = errors.New("invalid filesystem plan")

// PlanOperationKind identifies an operation in the closed filesystem-plan set.
type PlanOperationKind string

const (
	PlanCreateDirectory PlanOperationKind = "create_directory"
	PlanWriteFile       PlanOperationKind = "write_file"
	PlanCopyPath        PlanOperationKind = "copy_path"
	PlanMovePath        PlanOperationKind = "move_path"
	PlanDeletePath      PlanOperationKind = "delete_path"
)

// PlanOperation describes one operation from the closed filesystem-plan set.
// Only fields used by Kind may be set, preventing ambiguous requests.
type PlanOperation struct {
	Kind      PlanOperationKind
	Path      string
	Source    string
	Target    string
	Content   []byte
	Overwrite bool
	Recursive bool
}

// PlanResult reports the outcome of one completed plan operation.
type PlanResult struct {
	Kind    PlanOperationKind
	Created bool
}

// PlanExecutionError identifies the failed operation without exposing paths or
// operating-system details. The underlying error remains available to trusted callers.
type PlanExecutionError struct {
	Index int
	Err   error
}

func (e *PlanExecutionError) Error() string { return "filesystem plan operation failed" }
func (e *PlanExecutionError) Unwrap() error { return e.Err }

// PlanExecutor executes a bounded, prevalidated sequence through FileSystem.
// It stops on cancellation or the first error. Completed operations are not rolled back.
type PlanExecutor struct {
	filesystem    FileSystem
	maxOperations int
}

// NewPlanExecutor creates an executor with a caller-selected positive operation cap.
func NewPlanExecutor(filesystem FileSystem, maxOperations int) (*PlanExecutor, error) {
	if filesystem == nil || maxOperations <= 0 {
		return nil, ErrInvalidPlan
	}
	return &PlanExecutor{filesystem: filesystem, maxOperations: maxOperations}, nil
}

// Execute prevalidates and then executes a plan sequentially.
func (e *PlanExecutor) Execute(ctx context.Context, operations []PlanOperation) ([]PlanResult, error) {
	if len(operations) == 0 || len(operations) > e.maxOperations {
		return nil, ErrInvalidPlan
	}
	for _, operation := range operations {
		if !validPlanOperation(operation) {
			return nil, ErrInvalidPlan
		}
	}

	results := make([]PlanResult, 0, len(operations))
	for index, operation := range operations {
		if err := ctx.Err(); err != nil {
			return results, &PlanExecutionError{Index: index, Err: err}
		}
		result, err := e.execute(operation)
		if err != nil {
			return results, &PlanExecutionError{Index: index, Err: err}
		}
		results = append(results, result)
	}
	return results, nil
}

func (e *PlanExecutor) execute(operation PlanOperation) (PlanResult, error) {
	result := PlanResult{Kind: operation.Kind}
	var err error
	switch operation.Kind {
	case PlanCreateDirectory:
		result.Created, err = e.filesystem.Mkdir(operation.Path)
	case PlanWriteFile:
		err = e.filesystem.Write(operation.Path, operation.Content, operation.Overwrite)
	case PlanCopyPath:
		err = e.filesystem.Copy(operation.Source, operation.Target, operation.Overwrite)
	case PlanMovePath:
		err = e.filesystem.Move(operation.Source, operation.Target, operation.Overwrite)
	case PlanDeletePath:
		err = e.filesystem.Delete(operation.Path, operation.Recursive)
	}
	return result, err
}

func validPlanOperation(operation PlanOperation) bool {
	switch operation.Kind {
	case PlanCreateDirectory:
		return operation.Path != "" && operation.Source == "" && operation.Target == "" && len(operation.Content) == 0 && !operation.Overwrite && !operation.Recursive
	case PlanWriteFile:
		return operation.Path != "" && operation.Source == "" && operation.Target == "" && !operation.Recursive
	case PlanCopyPath, PlanMovePath:
		return operation.Path == "" && operation.Source != "" && operation.Target != "" && len(operation.Content) == 0 && !operation.Recursive
	case PlanDeletePath:
		return operation.Path != "" && operation.Source == "" && operation.Target == "" && len(operation.Content) == 0 && !operation.Overwrite
	default:
		return false
	}
}
