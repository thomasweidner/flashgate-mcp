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
	Usage   PlanUsage
}

// PlanLimits bounds both the plan request and the work it may perform. Entries
// count path operands (one for path operations and two for copy/move), while
// bytes count write payloads and copied source bytes. Moves do not copy bytes.
type PlanLimits struct {
	MaxOperations int
	MaxEntries    int64
	MaxBytes      int64
}

// PlanUsage reports accounting for a completed operation or a failed runtime
// limit check. Operations is zero until the operation has completed.
type PlanUsage struct {
	Operations int
	Entries    int64
	Bytes      int64
}

// PlanExecutionError identifies the failed operation without exposing paths or
// operating-system details. The underlying error remains available to trusted callers.
type PlanExecutionError struct {
	Index int
	Usage PlanUsage
	Err   error
}

func (e *PlanExecutionError) Error() string { return "filesystem plan operation failed" }
func (e *PlanExecutionError) Unwrap() error { return e.Err }

// PlanExecutor executes a bounded, prevalidated sequence through FileSystem.
// It stops on cancellation or the first error. Completed operations are not rolled back.
type PlanExecutor struct {
	filesystem FileSystem
	limits     PlanLimits
}

// NewPlanExecutor creates an executor with a caller-selected positive operation cap.
func NewPlanExecutor(filesystem FileSystem, maxOperations int) (*PlanExecutor, error) {
	maxEntries := int64(maxOperations)
	if maxEntries > int64(^uint64(0)>>1)/2 {
		maxEntries = int64(^uint64(0) >> 1)
	} else {
		maxEntries *= 2
	}
	return NewPlanExecutorWithLimits(filesystem, PlanLimits{
		MaxOperations: maxOperations,
		MaxEntries:    maxEntries,
		MaxBytes:      int64(^uint64(0) >> 1),
	})
}

// NewPlanExecutorWithLimits creates an executor with positive operation,
// path-entry, and byte caps.
func NewPlanExecutorWithLimits(filesystem FileSystem, limits PlanLimits) (*PlanExecutor, error) {
	if filesystem == nil || limits.MaxOperations <= 0 || limits.MaxEntries <= 0 || limits.MaxBytes <= 0 {
		return nil, ErrInvalidPlan
	}
	return &PlanExecutor{filesystem: filesystem, limits: limits}, nil
}

// Execute prevalidates and then executes a plan sequentially.
func (e *PlanExecutor) Execute(ctx context.Context, operations []PlanOperation) ([]PlanResult, error) {
	if len(operations) == 0 || len(operations) > e.limits.MaxOperations {
		return nil, ErrInvalidPlan
	}
	var preflight PlanUsage
	for _, operation := range operations {
		if !validPlanOperation(operation) {
			return nil, ErrInvalidPlan
		}
		cost := staticPlanCost(operation)
		if !addPlanUsageWithinLimits(&preflight, cost, e.limits) {
			return nil, ErrInvalidPlan
		}
	}

	results := make([]PlanResult, 0, len(operations))
	var usage PlanUsage
	for index, operation := range operations {
		if err := ctx.Err(); err != nil {
			return results, &PlanExecutionError{Index: index, Usage: usage, Err: err}
		}
		cost, err := e.runtimePlanCost(operation)
		if err != nil {
			return results, &PlanExecutionError{Index: index, Usage: usage, Err: err}
		}
		prospective := usage
		if !addPlanUsageWithinLimits(&prospective, cost, e.limits) {
			return results, &PlanExecutionError{Index: index, Usage: usage, Err: ErrLimitExceeded}
		}
		result, err := e.execute(operation)
		if err != nil {
			return results, &PlanExecutionError{Index: index, Usage: usage, Err: err}
		}
		usage = prospective
		result.Usage = cost
		results = append(results, result)
	}
	return results, nil
}

func staticPlanCost(operation PlanOperation) PlanUsage {
	entries := int64(1)
	if operation.Kind == PlanCopyPath || operation.Kind == PlanMovePath {
		entries = 2
	}
	bytes := int64(0)
	if operation.Kind == PlanWriteFile {
		bytes = int64(len(operation.Content))
	}
	return PlanUsage{Operations: 1, Entries: entries, Bytes: bytes}
}

func (e *PlanExecutor) runtimePlanCost(operation PlanOperation) (PlanUsage, error) {
	cost := staticPlanCost(operation)
	if operation.Kind != PlanCopyPath {
		return cost, nil
	}
	metadata, err := e.filesystem.Stat(operation.Source)
	if err != nil {
		return PlanUsage{}, err
	}
	if metadata.IsDir {
		return PlanUsage{}, ErrCopyDirectoryUnsupported
	}
	if metadata.Size < 0 {
		return PlanUsage{}, ErrInvalidPlan
	}
	cost.Bytes = metadata.Size
	return cost, nil
}

func addPlanUsageWithinLimits(total *PlanUsage, addition PlanUsage, limits PlanLimits) bool {
	if addition.Operations < 0 || addition.Entries < 0 || addition.Bytes < 0 ||
		total.Operations > limits.MaxOperations-addition.Operations ||
		total.Entries > limits.MaxEntries-addition.Entries ||
		total.Bytes > limits.MaxBytes-addition.Bytes {
		return false
	}
	total.Operations += addition.Operations
	total.Entries += addition.Entries
	total.Bytes += addition.Bytes
	return true
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
