# Process Observation and Managed-Process Threat Model

**Status:** Normative Version 1.0 target contract

**Owner:** BL-113
**Applies to:** process observation and server-managed process lifecycle

## Scope and trust boundaries

FlashGate treats process information and process control as separate security
surfaces. Process observation reads operating-system state that may disclose
users, executable locations, command lines, working directories, parent/child
relationships, or timing information. Managed-process operations create and
control only processes started by FlashGate through the Managed Process Engine.

The MCP client and every value supplied by it are untrusted. The operating
system process APIs and the server's own registry records are authoritative
only after platform errors, partial results, lifecycle races, and identifier
reuse have been handled. Tool visibility and MCP annotations are never an
authorization decision.

Version 1.0 explicitly excludes:

- control of arbitrary external PIDs;
- interactive process input and interactive shells;
- unrestricted command lines, environment access, or process dumps;
- treating a client-supplied PID, parent PID, executable path, user name, or
  process title as identity or authorization evidence.

Those exclusions cannot be enabled by a request flag. External-process control
and interactive input remain separate post-Version-1.0 decision gates.

## Protected assets

- process existence, ownership, topology, executable identity, command lines,
  environments, working directories, start times, and resource data;
- opaque managed-process handles and the registry records they address;
- captured stdout/stderr, output cursors, exit status, and diagnostics;
- caller principal, effective profile/capabilities, root, execution backend,
  service generation, and audit correlation;
- server availability, child-process capacity, storage, file descriptors, and
  cleanup capacity;
- unrelated operating-system processes, which FlashGate must never signal or
  wait on because of stale state or PID reuse.

## Authorization model

Observation requires the functional capability `process.observe`. Starting,
waiting for, reading output from, and stopping server-started processes require
`process.manage` plus the applicable executable, argument, root, environment,
limit, and execution-backend policy. Authorization is checked at execution
time, not only while constructing `tools/list`.

Every stateful operation revalidates the caller against the registry record.
A managed-process handle is opaque and bound to at least:

```text
caller principal + profile/capability context + root + execution backend
+ service generation + process-instance identity + expiry
```

Possession of a handle does not grant access. Cross-principal reads, waits,
stops, and output access fail without confirming whether a guessed handle is
valid for another caller.

## Threats and required controls

### Observation disclosure and enumeration

Unbounded process lists and rich default fields can expose other users,
credentials embedded in arguments, sensitive paths, service topology, or
security tooling. Observation therefore uses bounded pages with deterministic
ordering, explicit field selection, server-side redaction, and a safe minimal
default field set. Environment values are not returned by the observation
surface. Access-denied or vanished processes produce bounded per-entry
diagnostics or an aggregate partial-result marker; raw OS errors and absolute
host paths are not returned.

Filters reduce result volume but do not expand visibility. Pagination cursors
are opaque, expire, and bind the caller, effective policy, requested fields and
filters, ordering contract, and snapshot/generation semantics. A cursor from a
different context fails closed.

### PID reuse and false association

PIDs are reusable diagnostic values, never managed-process handles. A registry
record binds a process instance using the strongest available creation-time or
native process identity in addition to its PID. Before a signal, wait, or
destructive lifecycle action, the platform adapter revalidates that identity.
If identity cannot be proved, mismatches, or the process has already exited,
FlashGate returns a stable stale/not-found lifecycle result and does not target
the current PID occupant.

Observation pagination must tolerate processes exiting and PIDs being reused
between page construction and detail reads. It must not merge fields gathered
from two different process instances into one result.

### Lifecycle and control races

Start, exit, wait, timeout, cancellation, stop, output drain, and server
shutdown may race. Registry state transitions are monotonic and idempotent;
exactly one terminal state is published. Repeated wait or stop requests return
the recorded outcome without signalling an unrelated process. Cancellation of
a request does not silently transfer ownership or abandon required cleanup.

Stopping targets the managed process instance and its owned process tree under
the documented platform strategy. Graceful termination has a bounded grace
period before any permitted escalation. Platform inability to prove tree
membership fails safely and is surfaced as a bounded partial-cleanup result;
it never authorizes broad name-, parent-PID-, or session-based killing.

### Orphans, restart, and cleanup

The process-root lifecycle coordinator cancels managed work on controlled
shutdown and invokes the Managed Process domain owner for bounded cleanup.
Service generation changes invalidate all prior handles and cursors. Persisted
diagnostics must not make stale processes controllable after restart.

Crash/orphan handling is platform-specific and must use explicit ownership
mechanisms where available. Startup may detect and report prior-generation
survivors, but Version 1.0 must not adopt or kill a process merely because its
PID, executable, parent, command line, or record resembles an old child.

### Output, command-line, and environment disclosure

stdout and stderr are separate bounded streams with independent truncation
markers and monotonic cursors. Limits apply while reading from the child, not
only when serializing an MCP response. Slow clients cannot cause unbounded
memory, goroutine, descriptor, or temporary-file growth.

Command lines, environments, executable paths, working directories, output,
errors, and audit events pass through field-specific minimization and
redaction. Secrets are redacted before persistence and before diagnostic or
audit emission. Truncation and redaction are explicit; omitted sensitive data
must not be recoverable through error detail, cursor metadata, or resource
identifiers.

### Resource exhaustion and denial of service

The server enforces global and per-principal limits for observed entries,
detail lookups, concurrent managed processes, queued starts, runtime, stdout,
stderr, stored results, temporary bytes, and handle/cursor lifetime. Counters
are charged incrementally, including failed or redacted reads where work was
performed. Limit exhaustion produces deterministic bounded errors and does not
disable cleanup.

Child inheritance is minimized: no shell by default, explicit environment,
root-confined working directory, closed or deliberately inherited handles, and
platform-appropriate least privilege. Managed-process execution reuses the
single engine required by ADR-011; adapters may not bypass its limits.

### Platform variance and partial visibility

Windows and Linux expose different process fields and access guarantees.
Public fields have portable semantics; unsupported or inaccessible optional
fields are omitted or represented with a stable availability state rather than
guessed. Platform adapters normalize access-denied, vanished, unsupported, and
stale-identity outcomes without leaking raw native errors.

## Error and audit contract

Process failures use stable categories such as invalid request, unauthorized,
not found/stale, access denied, limit exceeded, timed out, cancelled,
unsupported field, and internal failure. Responses do not expose raw OS error
strings, secrets, unrestricted command lines, absolute paths, or whether a
cross-principal handle exists.

Audit events record the caller, effective authorization context, operation,
managed handle or redacted observation scope, decision, bounded counters,
result category, and correlation ID. They do not record unrestricted process
output, command lines, or environments. Audit backpressure must remain bounded
and must not prevent process cleanup.

## Required validation

Version 1.0 validation must cover:

- server-side `process.observe` and `process.manage` checks, including crafted
  calls that bypass catalog discovery;
- bounded pagination, field selection, redaction, partial visibility, cursor
  expiry, and policy/context mismatch;
- PID reuse and start-identity mismatch immediately before wait/stop actions;
- concurrent exit/wait/stop/timeout/cancellation and exactly one terminal
  state under the race detector;
- separate stdout/stderr bounds, truncation, slow readers, cursor misuse, and
  cleanup after success, failure, cancellation, timeout, and shutdown;
- cross-principal and prior-service-generation handle denial without existence
  disclosure;
- child-tree cleanup and orphan diagnostics without unrelated-process
  targeting;
- global/per-principal process and storage exhaustion, deterministic overload,
  TTL cleanup, and leak checks;
- Windows and Linux native adapter behavior, including access denial, partial
  fields, identity revalidation, tree cleanup, and restart behavior.

Cloud/unit tests may validate models, races, fakes, and parsers. They are not a
substitute for native Windows/Linux process identity, ownership, signal,
handle, job/process-group, service shutdown, or orphan evidence.

## Residual risks

Authorized observation can still reveal sensitive host metadata after
minimization. OS process snapshots are inherently time-varying, and abrupt
server or host failure may leave work whose ownership cannot later be proven.
Platform isolation and tree-control mechanisms differ. These risks require
least-privilege deployment, conservative fields, bounded retention, explicit
native validation, and safe refusal whenever process-instance identity or
ownership cannot be established.
