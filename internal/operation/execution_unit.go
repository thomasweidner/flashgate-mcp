// Package operation provides transport-neutral operation lifecycle primitives.
package operation

// ExecutionUnit identifies the mechanism used to run operation work.
type ExecutionUnit string

const (
	// ExecutionUnitGoroutine is the safe default for bounded in-process work.
	ExecutionUnitGoroutine ExecutionUnit = "goroutine"
	// ExecutionUnitSubprocess isolates work in a separately managed process.
	ExecutionUnitSubprocess ExecutionUnit = "subprocess"
)

// SubprocessReason records the first deterministic gate requiring a subprocess.
// The order of the constants is also the selection precedence.
type SubprocessReason string

const (
	SubprocessReasonNone                      SubprocessReason = ""
	SubprocessReasonExternalExecutable        SubprocessReason = "external_executable"
	SubprocessReasonHardResourceIsolation     SubprocessReason = "hard_resource_isolation"
	SubprocessReasonCrashIsolation            SubprocessReason = "crash_isolation"
	SubprocessReasonUnreliableCancellation    SubprocessReason = "unreliable_in_process_cancellation"
	SubprocessReasonDifferentIdentity         SubprocessReason = "different_execution_identity"
	SubprocessReasonPlatformExternalMechanism SubprocessReason = "platform_external_mechanism"
)

// ExecutionRequirements contains facts established by the owning domain and
// policy layers. Selection does not authorize an executable, identity, or
// platform mechanism; those checks must complete before dispatch.
type ExecutionRequirements struct {
	ExternalExecutable            bool
	HardResourceIsolation         bool
	CrashIsolation                bool
	ReliableInProcessCancellation bool
	DifferentExecutionIdentity    bool
	PlatformExternalMechanism     bool
}

// ExecutionSelection is the deterministic execution-unit decision.
type ExecutionSelection struct {
	Unit   ExecutionUnit
	Reason SubprocessReason
}

// SelectExecutionUnit chooses a subprocess when any accepted subprocess gate
// applies. Otherwise it selects the controlled-goroutine default.
func SelectExecutionUnit(requirements ExecutionRequirements) ExecutionSelection {
	switch {
	case requirements.ExternalExecutable:
		return ExecutionSelection{Unit: ExecutionUnitSubprocess, Reason: SubprocessReasonExternalExecutable}
	case requirements.HardResourceIsolation:
		return ExecutionSelection{Unit: ExecutionUnitSubprocess, Reason: SubprocessReasonHardResourceIsolation}
	case requirements.CrashIsolation:
		return ExecutionSelection{Unit: ExecutionUnitSubprocess, Reason: SubprocessReasonCrashIsolation}
	case !requirements.ReliableInProcessCancellation:
		return ExecutionSelection{Unit: ExecutionUnitSubprocess, Reason: SubprocessReasonUnreliableCancellation}
	case requirements.DifferentExecutionIdentity:
		return ExecutionSelection{Unit: ExecutionUnitSubprocess, Reason: SubprocessReasonDifferentIdentity}
	case requirements.PlatformExternalMechanism:
		return ExecutionSelection{Unit: ExecutionUnitSubprocess, Reason: SubprocessReasonPlatformExternalMechanism}
	default:
		return ExecutionSelection{Unit: ExecutionUnitGoroutine}
	}
}
