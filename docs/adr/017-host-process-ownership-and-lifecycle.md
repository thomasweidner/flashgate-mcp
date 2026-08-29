# ADR-017: Host Process Ownership and Lifecycle

## Status

Accepted

## Context

ADR-014 accepts direct STDIO, proxy, auto, and operating-system service roles
in one native executable. The same product can therefore have multiple
legitimate processes whose owners and lifetimes differ. Three previously
observed FlashGate processes were no longer available when diagnosis was
attempted; they are evidence of a credible quality, security, and diagnostic
risk, not proof that those particular processes were orphaned.

A process must not be terminated merely because it is old, idle, uses little
CPU, has no active request, shares an executable name, or appears beside
another instance. PID-only and immediate-parent heuristics are also unsafe:
PIDs are reused, wrappers may outlive logical agents, and retained pipe handles
can delay EOF. The target architecture therefore needs one normative contract
for ownership, definitive shutdown signals, cleanup, diagnostics, and the
boundary between definitely orphaned and only suspected-stale instances.

## Decision

### Mode, role, owner, and lifetime

| Mode or role | Authoritative owner | Expected lifetime | Owner or transport loss |
|---|---|---|---|
| Direct `stdio` | One MCP transport session / client launch | Session-scoped | Bounded shutdown after definitive transport loss or verified owner loss |
| `proxy` or the proxy path of `auto` | One client-side MCP transport session | Session-scoped lightweight edge | Close the service connection, cancel and clean connection-owned work, then exit within the bound |
| Direct fallback of `auto` | One MCP transport session / client launch | Session-scoped | Same contract as direct `stdio` |
| `service` | Windows SCM or Linux systemd | Persistent across client sessions | A normal client disconnect does not stop the service |
| Future user host | User-service/background-host supervisor | Persistent according to user-host policy | Supervisor lifecycle |
| Future `worker` | FlashGate broker/service | Broker-owned | Bounded shutdown after verified control-channel or negotiated lease loss |

Direct mode has a baseline of one full FlashGate process per active MCP client
transport session. Multiple direct processes are therefore legitimate. After
service implementation, the expected shared-service baseline is one persistent
full FlashGate service plus one lightweight edge proxy per active client
transport. Additional processes are legitimate only when they are explicitly
classified and diagnosable as workers or managed children.

Each running host receives one immutable host-instance identity at creation.
It binds the instance ID to mode/role, process start identity, and the verified
owner or control relationship established for that instance. Registry updates,
service or proxy metadata, client/MCP claims, and later observations may enrich
diagnostics, but they cannot mutate that identity or reclassify the running
instance under another owner. A materially different identity or service
generation is a new instance, never a rewrite of the existing one.

### Definitive shutdown signals and precedence

Every session-scoped direct or proxy process has exactly one transport-neutral
process-root lifecycle coordinator. Transport and host adapters report signals
to that coordinator; they do not perform domain cleanup themselves. The first
definitive signal wins, stores one typed shutdown reason, and makes subsequent
signals idempotent observations of the same shutdown:

1. normal STDIN EOF or normal transport closure;
2. unrecoverable read- or write-pipe failure;
3. explicit client/protocol shutdown when supported by the negotiated contract;
4. operating-system stop or termination signal;
5. verified owner-process or control-channel loss when a reliable owner
   relationship was actually established;
6. expiry of an explicitly versioned and negotiated session lease when both
   sides support it.

The coordinator maintains an explicit current lifecycle/shutdown phase and a
monotonic last-completed-shutdown phase. The minimum ordered shutdown phases
are `INTAKE_STOPPED`, `OWNED_WORK_CANCELLED`, `DOMAIN_CLEANUP_COMPLETED`,
`OWNED_CHILD_CLEANUP_COMPLETED`, `TEMPORARY_RESOURCE_CLEANUP_COMPLETED`, and
`EVIDENCE_FINALIZED`; `RUNNING` precedes shutdown and `EXITED` is terminal.
The last-completed value advances only after the corresponding phase succeeds.
If the hard deadline expires, evidence records both the phase in progress and
the last successfully completed phase before forced exit. Deadline expiry can
never be classified as a regularly completed graceful shutdown.

The minimum typed shutdown-reason taxonomy is `TRANSPORT_EOF`,
`TRANSPORT_CLOSED`, `TRANSPORT_BROKEN`, `CLIENT_REQUESTED`, `OS_STOP`,
`OWNER_LOST`, `CONTROL_CHANNEL_LOST`, `LEASE_EXPIRED`, `INTERNAL_FATAL`, and
`SHUTDOWN_DEADLINE_EXCEEDED`. Adapters may add platform detail as bounded
metadata, but they must map to one of these stable reasons. Deadline expiry may
upgrade the final exit classification to `EXITED_FORCED`; it does not replace
the first initiating reason in evidence.

The coordinator then:

1. stops new protocol intake and new connection/session-owned work;
2. cancels connection/session-owned operations;
3. performs bounded drain and cleanup;
4. invokes Operations/Job and Managed Process cleanup through their respective
   owners;
5. terminates only actually owned child trees and only within policy;
6. removes owned temporary resources;
7. emits bounded, secret-safe lifecycle and audit evidence; and
8. exits the session-scoped host within one defined hard upper limit, using a
   typed forced-exit result if graceful completion misses the bound.

Root cancellation is the integration signal. Operations/Jobs remain owned by
their manager, managed children remain owned by the Managed Process component,
and neither becomes the owner of the top-level FlashGate host.

### Ownership and automatic-termination safety boundaries

The following rules are normative:

- no unconditional idle-timeout kill and no singleton assumption;
- process age, low CPU use, and absence of active requests are not orphan proof;
- PID alone is never sufficient owner identity;
- the immediate parent PID is not automatically the logical chat or agent;
- a long-lived ChatGPT/Codex app server or wrapper can outlive a logical agent;
- a retained pipe handle can prevent EOF;
- a live proxy-service channel does not necessarily prove that the original
  logical agent is alive;
- automatic cleanup must never target unrelated external processes;
- registry/state files are evidence, never sole ownership authority; and
- lease/heartbeat semantics are valid only when explicitly negotiated, never
  silently inferred.

Proxy-, client-, and MCP-supplied owner information is claimed metadata, not
ownership authority. It may assist correlation but can never by itself
authorize automatic termination or another security-relevant lifecycle
decision. Such authority requires trusted local evidence defined by this ADR:
a bound local transport/control-channel relationship, verified OS process
identity or process handle plus start identity, SCM/systemd supervision, an
explicitly negotiated and verified FlashGate control/lease contract, or other
local evidence that a later ADR explicitly classifies as authoritative. A
proxy claim cannot create authority over another process or session.

When the process, transport, and apparent owner are still live and no explicit
lease or control contract proves loss, the classification is
`SUSPECTED_STALE`; automatic termination is forbidden. `DEFINITELY_ORPHANED`
requires conclusive ownership and transport evidence.

### Platform mechanisms

Windows adapters may use a verified process handle plus process creation/start
identity, console/service stop integration, and Job Objects only when the
launcher, service, or broker actually owns every included child process.

Linux adapters may use PID plus process start identity, signals and process
groups, systemd/cgroup supervision where appropriate, and a parent-death
signal only when the immediate parent is deliberately and verifiably the real
owner.

Both platforms use one process-root cancellation context, bounded child-tree
cleanup, bounded stream drain, and no PID-only authority. A Job Object or
parent-death signal is not a universal substitute for correct ownership. Direct
STDIO must not gain a second watchdog process solely for owner monitoring
unless a later explicit security and benchmark decision justifies the cost.

### Lifecycle evidence and classification

Secret-safe lifecycle evidence must make simultaneous instances explainable
without recording command lines, secrets, or unnecessary host paths. The
minimum record contains:

- immutable host-instance ID and its creation-time mode/role and owner/control
  binding, plus PID and process start identity;
- parent PID and parent process start identity when available;
- verified owner type and owner identity/liveness state, with claimed
  proxy/client/MCP metadata stored separately from authoritative ownership
  evidence;
- MCP client name/version from `initialize` when available;
- service generation or proxy connection ID when applicable;
- start time, last transport activity, and active request count;
- shutdown initiation time, typed shutdown reason, current lifecycle/shutdown
  phase, and last completed shutdown phase;
- cleanup/owned-child-tree termination result; and
- final exit classification.

The minimum classifications are `ACTIVE_SESSION`,
`EXPECTED_PERSISTENT_SERVICE`, `SHUTTING_DOWN`, `DEFINITELY_ORPHANED`,
`SUSPECTED_STALE`, `EXITED_CLEANLY`, and `EXITED_FORCED`. Stale runtime-registry
entries must be deterministically removable, but registry state cannot replace
operating-system ownership evidence.

### Shared-service model and its limits

The shared-service layer allows one persistent heavy service to serve multiple
clients, leaves only a lightweight STDIO edge proxy per client transport,
binds service resources to connection/principal and releases them on
disconnect, uses SCM/systemd for authoritative service supervision and
recovery, and invalidates stale handles through service generation.

It does not prove the lifetime of a logical agent, prevent a lightweight edge
proxy from leaking, force EOF when another process retains a pipe handle, or
make idle-timeout termination safe. A live proxy-service connection remains
insufficient proof that the original logical agent is alive.

## Task and ownership boundaries

- BL-223 owns CLI mode, role/lifetime, shutdown-reason, and exit contracts.
- BL-224 owns the transport-neutral process-root lifecycle coordinator and
  adapter/domain boundary.
- BL-225 owns connection/session identity, disconnect behavior, and optional
  negotiated FlashGate-controlled lease semantics.
- BL-341 owns technical implementation of top-level host ownership, shutdown,
  diagnostics, and orphan prevention across the implemented modes.
- BL-241 owns the integrated Windows/Linux lifecycle and leak test matrix.
- BL-263 owns the Version 1.0 release gate.
- BL-094 owns Operations/Job controlled shutdown; it does not own the host.
- BL-129 owns FlashGate-managed child cleanup/restart/orphan behavior; it does
  not own the host.
- BL-230 and BL-231 own SCM and systemd service integration.

ADR-015 remains authoritative for execution identity, including the permanent
prohibition on in-process impersonation. This ADR governs top-level host-process
ownership and lifecycle only; it neither replaces ADR-015 nor weakens its
caller/effective-identity and execution-backend boundaries.

The dependency flow is BL-223/BL-224/BL-225 contract definition -> BL-341
implementation together with BL-226 through BL-231 -> BL-241 integrated
validation -> BL-263 release decision.

## Security impact

False-negative cleanup can leave session-scoped hosts, owned children,
connection-owned work, temporary resources, or principal-bound state alive.
False-positive cleanup can terminate legitimate FlashGate sessions or unrelated
external processes. The design therefore requires conclusive identity and
ownership evidence, fail-safe `SUSPECTED_STALE` handling, bounded cleanup,
principal/connection binding, PID-reuse resistance, secret-safe evidence, and
tests for retained handles, live wrappers, service persistence, and stale
registry records.

Version 1.0 acceptance requires that, after conclusive owner or transport loss
and expiration of the bounded shutdown window, no `DEFINITELY_ORPHANED`
session-scoped FlashGate host or owned child process remains.

## Compatibility

- The current no-argument invocation remains direct STDIO.
- Direct STDIO requires no proprietary MCP heartbeat extension.
- EOF and transport closure are process lifecycle events, not public tool
  results.
- Optional leases are internal, versioned, negotiated FlashGate IPC behavior
  and do not change public MCP compatibility.
- A normal client disconnect ends connection-owned work but not the persistent
  service.
- No remote listener, runtime interpreter, hidden installation, automatic
  elevation, or single-instance requirement is introduced.

## Alternatives considered

- **Global singleton:** rejected because multiple direct/proxy sessions are
  legitimate and require independent ownership.
- **Idle/age/CPU/request-count reaper:** rejected because these signals do not
  prove orphaning and can terminate valid long-lived sessions.
- **PID or parent PID as authority:** rejected because of PID reuse, wrappers,
  and mismatch between immediate and logical owners.
- **Universal Job Object or parent-death signal:** rejected because these
  mechanisms are correct only for deliberately established ownership.
- **Always-on watchdog process:** rejected for direct STDIO unless later
  security and benchmark evidence supports the extra process and complexity.

## Consequences

Direct isolated STDIO remains supported and is not replaced by a shared host.
The architecture gains explicit owner adapters, typed lifecycle state,
bounded cleanup coordination, and diagnostic records. Implementation and tests
must cover more platform and failure paths, but accidental service termination,
PID-reuse targeting, unsafe heuristic reaping, and unclassified remnants become
explicitly prohibited and testable.

## Non-goals

- proving that the three historical processes were orphaned;
- terminating unrelated or merely suspected-stale processes;
- implementing BL-341 in this ADR/documentation change;
- redefining Operations/Jobs or Managed Process ownership;
- adding remote transport, automatic installation/elevation, or interpreter
  dependencies;
- committing to user-host or worker runtime implementation in Version 1.0.

## Related documents

- [ADR-003](003-stdio-transport.md)
- [ADR-014](014-native-multi-mode-runtime-and-local-service-deployment.md)
- [ADR-015](015-hybrid-service-execution-identity.md)
- [Architecture](../architecture.md)
- [Native runtime and service plan](../native-multi-mode-runtime-and-service-plan.md)
- [Security](../security.md)
- [Protocol](../protocol.md)
- [Specification](../specification.md)
- [Testing](../testing.md)
- Authoritative backlog: `BACKLOG.md`
