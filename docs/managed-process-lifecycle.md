# Managed Process Cleanup, Restart, and Orphan Contract

## Status and scope

This document defines the planned Version 1.0 lifecycle contract owned by
BL-129 for processes started by FlashGate. It refines ADR-011 without
implementing the Managed Process Registry or platform adapters.

This contract applies only to FlashGate-managed child processes. Top-level
FlashGate host ownership and shutdown remain governed by ADR-017. External PID
control remains outside Version 1.0 and is never acquired as a cleanup
fallback.

## Ownership record

Before a child is started, the Managed Process component creates an immutable,
opaque process handle and an ownership record bound to:

- principal, profile, allowed root, and effective execution backend;
- service generation and managed-process instance generation;
- the approved command definition and policy decision;
- platform process identity, including PID plus process start identity or a
  verified OS process handle; and
- parent/child-tree ownership established by the launcher.

PID, executable name, parent PID, command line, a registry file, or an
untrusted client claim is never sufficient ownership authority. A PID that no
longer matches the recorded start identity is treated as reused and must not be
signalled. Handle access, wait, output, stop, and cleanup require the bound
execution context.

## Lifecycle and terminal states

Managed process state follows the status contract owned by BL-121. Cleanup
adds an idempotent internal phase model:

1. `RUNNING`: the child may produce bounded stdout and stderr.
2. `STOP_REQUESTED`: no new input or child work is accepted and the initiating
   reason is fixed.
3. `GRACEFUL_STOP`: the platform adapter sends only the policy-approved
   graceful signal and drains output within a bound.
4. `FORCED_STOP`: after the grace bound, the adapter terminates only the
   verified owned process tree.
5. `OUTPUT_FINALIZED`: bounded buffers and truncation markers are sealed.
6. `RESOURCES_RELEASED`: handles, process objects, temporary resources, and
   accounting slots are released.
7. `CLEANUP_COMPLETE`: the final status and cleanup evidence are immutable.

Repeated cancellation, stop, timeout, TTL sweep, disconnect, or host-shutdown
signals observe the same cleanup operation. They do not signal a process a
second time, extend deadlines, replace the first cleanup reason, or rewrite a
terminal status. A failed cleanup records the last completed phase and remains
eligible for bounded leak detection; it is not reported as clean completion.

## Cleanup triggers and precedence

The Managed Process component initiates bounded cleanup for:

- explicit authorized `stop_process`;
- operation cancellation or a server-enforced runtime deadline;
- process exit, startup failure, or output-pump failure;
- owning connection/session loss when policy makes the child session-owned;
- process-handle or retained-result TTL expiry;
- process-root shutdown delegated by the host lifecycle coordinator; and
- registry recovery after restart when conclusive ownership evidence remains.

The first trigger fixes a typed reason. A server timeout or shutdown hard
deadline may strengthen the final outcome to forced or incomplete, but it does
not erase that initiating reason. Host adapters request cleanup through the
Managed Process owner; they never enumerate or kill children directly.

Cleanup is bounded by server-configured grace and hard deadlines. Missing or
ambiguous ownership evidence fails closed: FlashGate records a diagnostic
finding and does not target the process. Age, inactivity, low CPU use, matching
executable names, or a stale record cannot authorize termination.

## Normal exit and explicit stop

On normal child exit, FlashGate captures the platform exit result, drains the
already-open stdout/stderr streams within the output bound, seals buffers,
releases OS resources, and retains only bounded result metadata until its TTL.
Output drain cannot keep a process handle or concurrency slot alive without a
deadline.

An explicit stop first revalidates the caller, handle context, service
generation, and platform process identity. The adapter attempts the configured
graceful stop and escalates only after the grace period. Tree termination is
limited to descendants whose ownership was established by the launcher and
preserved by the platform adapter. Unrelated descendants or external
processes are never adopted by name or PID ancestry alone.

## Host shutdown

The process-root lifecycle coordinator stops intake and cancels owned work,
then invokes the Managed Process component through its owning interface. The
component snapshots its owned children, prevents new starts, cleans children
concurrently only within a fixed bound, and returns a structured aggregate
result before the host hard deadline.

Host shutdown does not silently detach managed children. If the deadline is
exhausted, evidence identifies every incomplete handle and the last completed
cleanup phase. Forced host exit is not represented as successful managed-child
cleanup. Platform containment such as a Windows Job Object, Linux process
group, or cgroup may enforce the established ownership boundary, but it cannot
replace identity checks or broaden ownership.

## Crash and restart recovery

Version 1.0 does not promise that process handles or buffered output survive a
FlashGate crash or restart. Every restart creates a new service and
managed-process generation. Requests using an earlier-generation handle fail
with a stable stale-handle error and never attach to a PID observed after
restart.

Durable recovery metadata, when enabled, is bounded, integrity-checked,
secret-safe, and diagnostic rather than sole termination authority. Recovery
may clean a surviving child only when the new instance can prove the original
FlashGate ownership and the same platform start identity through authoritative
local evidence. Otherwise the record is classified as unresolved and no
automatic signal is sent. Stale records are removable after their diagnostic
retention TTL without implying that the referenced process was terminated.

Automatic restart of a managed command is not part of Version 1.0. A client
must submit a new authorized start request and receives a new handle. This
prevents hidden retry loops, duplicated side effects, and reuse of stale policy
decisions.

## TTL and retention

Runtime deadlines govern live child execution. TTLs govern retained handles,
sealed output, result metadata, and recovery diagnostics; a result TTL is not
silently converted into an unbounded process runtime.

If policy permits a live child to outlast its handle-retention window, expiry
first initiates the same bounded owned-child cleanup and releases the handle
only after the cleanup outcome is recorded. The implementation must not erase
the only ownership record before a live owned child is resolved. Terminal
metadata and output expire deterministically, release their byte accounting,
and become inaccessible through a stable expired-handle result.

TTL sweepers have bounded batch size and work duration. Sweep failure cannot
extend storage or process lifetime indefinitely; it produces leak accounting
and is retried only by the normal bounded maintenance schedule.

## Diagnostics and audit evidence

Lifecycle evidence is structured, bounded, and redacted. At minimum it records:

- opaque handle correlation, principal/policy correlation, and generation;
- platform identity as a protected diagnostic field, not public authority;
- start, exit, cleanup-initiation, phase-completion, and expiry timestamps;
- initiating reason, terminal status, exit classification, and last completed
  cleanup phase;
- graceful/forced/tree-cleanup outcomes and bounded duration;
- stdout/stderr byte counts and truncation state; and
- unresolved-child, PID-reuse, deadline, and resource-release findings.

Public results expose stable categories and only the minimum safe diagnostics.
They do not expose raw command lines, environments, credentials, absolute host
paths, unsafe OS error strings, or process-tree inventories. Audit failure or
backpressure follows the fail-safe audit policy and cannot silently disable
cleanup.

## Required validation

Implementation acceptance requires focused unit, race, and native Windows and
Linux tests for:

- simultaneous stop/cancel/timeout/exit signals and idempotent cleanup;
- PID reuse and process-start-identity mismatch without signalling;
- graceful stop, bounded escalation, owned-tree cleanup, and unrelated-process
  protection;
- stdout/stderr drain, truncation, blocked readers, and cleanup deadlines;
- host shutdown with zero, one, and many managed children;
- crash/restart generation invalidation and stale-handle denial;
- authoritative recovery, ambiguous recovery, corrupt/stale records, and
  diagnostic-record expiry;
- live-child, output, result, and handle TTL behavior plus bounded sweepers;
- resource/accounting release after every terminal and incomplete path; and
- redacted public errors, diagnostics, and audit events.

Cross-builds and fake adapters can validate portable state transitions but do
not replace native process-tree, signal, Job Object, process-group/cgroup, PID
reuse, crash, and restart evidence.

## Non-goals and ownership boundaries

- BL-119 owns the Managed Process Registry implementation.
- BL-120 owns opaque handle generation and PID-reuse protection mechanisms.
- BL-121 owns the public managed-process status model.
- BL-122 through BL-126 own start, wait, output, buffering, and stop tools.
- BL-130 through BL-132 own count, concurrency, runtime, CPU, and RAM limits.
- BL-134 and BL-135 own platform adapters and lifecycle/race tests.
- BL-224 and BL-341 own the top-level host coordinator and host lifecycle.
- BL-094 owns Operations/Job shutdown; jobs do not own managed children.

This contract adds no external PID control, automatic command restart,
persistent public handles across service generations, unbounded detached
processes, shell execution, new platform dependency, or host-process reaper.

## Related documents

- [ADR-011: Managed Process and Command Execution](adr/011-managed-process-and-command-execution.md)
- [ADR-017: Host Process Ownership and Lifecycle](adr/017-host-process-ownership-and-lifecycle.md)
- [Architecture](architecture.md)
- [Security](security.md)
- [Testing](testing.md)
