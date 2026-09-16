package managedprocess

import (
	"errors"
	"io"
)

const MaxCPURate = 10_000

var (
	ErrInvalidResourceLimits      = errors.New("invalid managed process resource limits")
	ErrInvalidProcessAdapter      = errors.New("invalid managed process adapter")
	ErrResourceControlUnavailable = errors.New("managed process resource control unavailable")
)

// ResourceLimits is the portable policy output consumed by native adapters.
// CPURate is measured in basis points of host CPU capacity (1..10,000);
// MemoryBytes is an aggregate hard ceiling for the complete process tree.
// Zero omits a control.
type ResourceLimits struct {
	CPURate     uint32
	MemoryBytes int64
}

func (limits ResourceLimits) validate() error {
	if limits.CPURate > MaxCPURate || limits.MemoryBytes < 0 {
		return ErrInvalidResourceLimits
	}
	return nil
}

func (limits ResourceLimits) required() bool {
	return limits.CPURate != 0 || limits.MemoryBytes != 0
}

// ProcessAdapter is the trusted platform boundary used by Engine. Adapters
// must establish every requested tree control before untrusted code executes.
type ProcessAdapter interface {
	Start(Launch, io.Writer, io.Writer) (startedProcess, error)
}

type processAdapterFunc func(Launch, io.Writer, io.Writer) (startedProcess, error)

func (adapter processAdapterFunc) Start(launch Launch, stdout, stderr io.Writer) (startedProcess, error) {
	return adapter(launch, stdout, stderr)
}
