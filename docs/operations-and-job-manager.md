# Operations and Job Manager

This document describes the accepted Version 1.0 target for FlashGate's
Operations/Job Manager. The manager is **planned and not implemented in the
current filesystem-only runtime**. [ADR-010](adr/010-operations-and-job-management.md)
is the architecture authority; the individual `BL-084`–`BL-099` backlog rows
own implementation and validation.

## Role and ownership

The manager is optional shared lifecycle infrastructure for work that needs
queuing, cancellation, a server deadline, progress reporting, bounded retained
results, or deterministic cleanup. Short bounded work may continue to execute
synchronously in its domain service.

Domain services retain validation, authorization inputs, business rules, work
implementation, and result semantics. The manager owns only generic lifecycle
mechanics. It must not interpret filesystem, search, process, execution, or
system-information payloads, and it must not depend on MCP or transport types.

## Handles and ownership checks

Each managed operation receives a server-generated, non-guessable handle with
the shape `op_<opaque-id>`. A handle is a lookup key, not a permission token.
Every status, result, and cancellation request must repeat authorization and
ownership checks against the stored execution context:

```text
caller principal + profile + root + execution backend + service generation
```

The registry also records the operation type and domain. A server restart
changes the service generation, so a handle from an earlier generation is
stale rather than reusable. Handles and diagnostics must not expose host paths,
principal data, process IDs, queue positions, or storage locations.

## Lifecycle

The accepted internal states are:

| State | Meaning | Permitted next state |
|---|---|---|
| `queued` | Accepted within queue limits but not started | `running`, `cancelled`, `timed_out`, `failed` |
| `running` | A worker owns execution | `completed`, `failed`, `cancelled`, `timed_out` |
| `completed` | Work finished and its bounded result metadata is final | terminal |
| `failed` | Work stopped with a bounded categorized failure | terminal |
| `cancelled` | Cancellation won before normal completion | terminal |
| `timed_out` | The server deadline expired and termination/cleanup began | terminal |

Terminal state is immutable. Competing completion, cancellation, timeout, and
shutdown paths must produce exactly one terminal transition. Cancellation is
cooperative for in-process workers: the manager cancels their `context.Context`
and workers check it at bounded intervals. The server owns deadlines and does
not trust a caller or worker to enforce them.

An approved subprocess may be used only when required for an external program,
hard resource or crash isolation, a different execution identity, or work that
cannot be cancelled reliably in-process. Subprocess termination and child-tree
cleanup remain the responsibility of their owning process/execution domain.

## Recorded state and results

Registry entries are bounded and may contain only:

- creation, queue, start, finish, expiry, and deadline timestamps;
- current state and a stable categorized failure;
- bounded domain-defined progress and byte counters;
- a bounded inline result or an opaque reference to separately authorized
  result/resource storage;
- temporary-resource references and cleanup status;
- the ownership context required for authorization and accounting.

Progress is advisory and monotonic within the domain contract; it does not
authorize work or prove completion. Payload-heavy output belongs in bounded
pages or identity-bound result/resource storage rather than registry entries.
Expiry must return a stable unavailable/expired outcome and must not disclose
whether another principal ever owned the handle.

## Limits, queues, and fairness

All configured limits are server-side hard caps. The Version 1.0 implementation
must enforce global, per-domain, and per-principal running-operation limits;
global and per-principal queue limits; runtime deadlines; retained-result and
temporary-data budgets; and bounded retention TTLs. It must reject overload
deterministically before allocating unbounded work.

Scheduling must prevent one principal from monopolizing workers while avoiding
starvation of other admitted principals. Queue position is not a public timing
or authorization promise. Exact default limit values remain owned by their
implementation/configuration backlog tasks and must not be inferred from this
documentation.

## Cleanup and shutdown

Each operation registers temporary resources with its lifecycle owner before
they become externally observable. Success, failure, cancellation, timeout,
result expiry, and shutdown all trigger bounded cleanup. If deletion cannot be
completed safely, the entry records an incomplete-cleanup marker for bounded
retry or leak detection; it must not silently claim success.

Controlled server shutdown stops admission, handles queued work according to
the accepted shutdown policy, signals running workers, waits only for a bounded
grace period, performs owner-specific cleanup, and freezes terminal outcomes.
TTL sweeping and leak detection cover abandoned registry entries, stored
results, and temporary resources without crossing principal or domain
boundaries.

The host lifecycle coordinator supplies root cancellation but does not perform
domain cleanup itself. Operations/Jobs and Managed Process components clean up
through their respective owners.

## MCP boundary

Internal operation state is protocol-independent. An MCP adapter may expose an
eligible operation only through a separately accepted mapping to the negotiated
official Tasks extension. No custom status/result/cancel tool set is assumed by
this target. Tasks compatibility, external-state mapping, and fallback behavior
remain the explicit decisions tracked by `BL-209`–`BL-211`.

## Validation targets

Implementation is not complete without focused tests for:

- state-transition races and exactly one terminal outcome;
- cancellation, deadline expiry, controlled shutdown, and worker leaks;
- opaque-handle guessing and cross-principal status/result/cancel denial;
- global/domain/principal limits, queue overload, fairness, and starvation;
- result TTL, restart-generation invalidation, and bounded storage;
- cleanup after success, failure, cancellation, timeout, and partial cleanup;
- redacted diagnostics, slow readers, and audit/log backpressure;
- Windows and Linux lifecycle behavior plus the Go race detector.

Cloud tests may validate transport-neutral behavior. Windows/native lifecycle,
identity, process-tree, and filesystem cleanup evidence remains required during
platform finalization.
