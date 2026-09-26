# ADR-010: Operations and Job Management

## Status

Accepted

## Context

Large filesystem operations, search, hashing, managed processes, and commands need bounded asynchronous execution without losing their business-domain ownership.

## Decision

Introduce a shared, optional Operations/Job Manager as planned runtime infrastructure. It is not a mandatory layer between every domain and platform adapter. Short synchronous work may execute directly in a domain service; long-running or managed work may use opaque internal handles shaped like `op_<opaque-id>`. Accepted internal states are `queued`, `running`, `completed`, `failed`, `cancelled`, and `timed_out`.

The manager is planned to track handle, operation type, domain, timestamps, deadline, state, bounded progress and byte counters, bounded error/result data, temporary resources, TTL, and cleanup status.

Domain services retain operation logic, validation, result semantics, and ownership. The generic manager owns lifecycle mechanics and knows neither MCP tool types nor domain-specific response contracts. The MCP adapter may later map eligible jobs to the negotiated official Tasks Extension `io.modelcontextprotocol/tasks`. Internal states may be more detailed than external Task states and require an explicit mapping.

The server sets and watches deadlines. Workers receive cancellable contexts and check them regularly. On expiry the server cancels work, terminates external workers if necessary, cleans or marks temporary resources, records `timed_out`, and keeps diagnostics/results bounded.

The default execution unit is a controlled goroutine using `context.Context`, bounded buffers and concurrency, streaming, and deterministic cleanup. A separate process is allowed only for an external executable, hard CPU/memory or crash isolation, unreliable in-process cancellation, another security identity, or a platform-only external mechanism.

## Rationale

A common lifecycle engine prevents every domain from implementing cancellation, progress, resource accounting, and cleanup differently.

## Consequences

- Domain ownership is preserved even when a job or subprocess performs the work.
- Global and per-domain concurrency, queue, runtime, result, and temporary-data limits are required.
- Shutdown and job-leak behavior become explicit contracts.
- Custom operation status/result/cancel tools are not the accepted primary MCP contract while Tasks and client fallback compatibility remain undecided.

## Security Impact

The server, not an untrusted worker, controls deadlines. Handles are opaque. Results, errors, queues, temporary data, and concurrency are bounded and auditable.

## Implementation Guidance

Build registry and lifecycle semantics without moving domain logic into the manager. Add race, cancellation, timeout, shutdown, Windows, Linux, and security tests. Keep MCP mapping in the adapter and follow ADR-013.

## Deferred Decisions

- MCP Tasks mapping and bounded fallback for clients without Tasks support
- persistence across restart
- exact progress schema and TTL defaults
- per-operation subprocess selection
