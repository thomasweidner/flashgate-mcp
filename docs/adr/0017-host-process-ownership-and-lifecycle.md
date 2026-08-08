# ADR-0017: Host Process Ownership and Lifecycle

## Status

Accepted

## Context

ADR-0014 accepts direct STDIO, proxy, auto, and operating-system service roles
in one native executable. Existing backlog items own CLI lifecycle, IPC
disconnect behavior, SCM/systemd hosting, managed child processes, Operations/
Jobs, and integrated lifecycle tests. A single cross-mode rule is still needed
for who owns each FlashGate host process and what happens when its owner or
transport disappears.

## Decision

Every FlashGate host instance has an immutable instance identity consisting of
its process ID, process start identity, runtime role, start time, and service
generation where applicable. A PID alone is never sufficient after possible
reuse.

- A direct STDIO host is owned by its launching client connection. EOF,
  transport failure, or owner loss initiates bounded graceful shutdown.
- A proxy-edge process is owned by its launching client connection. Its loss
  closes only that client edge and does not terminate the persistent service.
- An SCM/systemd service is owned by the operating-system service manager and
  persists across individual client disconnects. Only an authorized service
  lifecycle transition initiates service shutdown.

Shutdown has explicit phases and deadlines: stop intake, cancel or drain owned
requests, close transport, reconcile Operations/Jobs and managed children,
flush bounded audit/diagnostic state, and exit. Timeout handling records the
last completed phase and fails closed. Orphans are classified as host orphan,
managed-child orphan, operation/job remainder, or external process; each class
has a distinct owner and cleanup policy.

Connection identity, host instance identity, service generation, active owners,
shutdown phase, and orphan classification are available through bounded,
redacted instance diagnostics. Proxy claims never establish ownership; local
transport and operating-system evidence do.

## Scope boundaries

- ADR-0014 remains the authority for runtime modes and shared local service use.
- ADR-0015 remains the authority for execution identity and prohibits
  in-process impersonation.
- BL-129 owns cleanup/restart semantics for FlashGate-managed child processes.
- BL-223 owns CLI mode and exit contracts; BL-225 owns IPC disconnect behavior.
- BL-230 and BL-231 own SCM and systemd integration.
- BL-241 owns Windows/Linux multi-client and lifecycle integration tests.
- Operations/Jobs do not become the owner of the FlashGate host process.

## Consequences

Direct isolated STDIO remains supported and is not replaced by a shared host.
Shared-service persistence is explicit, while each edge connection remains
independently bounded. Implementations must prevent PID-reuse confusion,
unbounded shutdown, accidental service termination on client loss, and
unclassified process remnants.

## Related documents

- [ADR-0014](0014-native-multi-mode-runtime-and-local-service-deployment.md)
- [ADR-0015](0015-hybrid-service-execution-identity.md)
- [Native runtime and service plan](../native-multi-mode-runtime-and-service-plan.md)
- [Security](../security.md)
- [Testing](../testing.md)
