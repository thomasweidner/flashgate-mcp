package operation

import "testing"

func TestSelectExecutionUnitDefaultsToGoroutine(t *testing.T) {
	got := SelectExecutionUnit(ExecutionRequirements{ReliableInProcessCancellation: true})
	want := ExecutionSelection{Unit: ExecutionUnitGoroutine}
	if got != want {
		t.Fatalf("SelectExecutionUnit() = %#v, want %#v", got, want)
	}
}

func TestSelectExecutionUnitSubprocessGates(t *testing.T) {
	tests := []struct {
		name         string
		requirements ExecutionRequirements
		reason       SubprocessReason
	}{
		{"external executable", ExecutionRequirements{ExternalExecutable: true, ReliableInProcessCancellation: true}, SubprocessReasonExternalExecutable},
		{"hard resource isolation", ExecutionRequirements{HardResourceIsolation: true, ReliableInProcessCancellation: true}, SubprocessReasonHardResourceIsolation},
		{"crash isolation", ExecutionRequirements{CrashIsolation: true, ReliableInProcessCancellation: true}, SubprocessReasonCrashIsolation},
		{"unreliable cancellation", ExecutionRequirements{}, SubprocessReasonUnreliableCancellation},
		{"different identity", ExecutionRequirements{ReliableInProcessCancellation: true, DifferentExecutionIdentity: true}, SubprocessReasonDifferentIdentity},
		{"platform mechanism", ExecutionRequirements{ReliableInProcessCancellation: true, PlatformExternalMechanism: true}, SubprocessReasonPlatformExternalMechanism},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := SelectExecutionUnit(test.requirements)
			want := ExecutionSelection{Unit: ExecutionUnitSubprocess, Reason: test.reason}
			if got != want {
				t.Fatalf("SelectExecutionUnit() = %#v, want %#v", got, want)
			}
		})
	}
}

func TestSelectExecutionUnitUsesStableGatePrecedence(t *testing.T) {
	got := SelectExecutionUnit(ExecutionRequirements{
		ExternalExecutable:            true,
		HardResourceIsolation:         true,
		CrashIsolation:                true,
		ReliableInProcessCancellation: false,
		DifferentExecutionIdentity:    true,
		PlatformExternalMechanism:     true,
	})
	if got.Reason != SubprocessReasonExternalExecutable {
		t.Fatalf("reason = %q, want %q", got.Reason, SubprocessReasonExternalExecutable)
	}
}
