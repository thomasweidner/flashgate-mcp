# Audit Lifecycle and Trace Correlation Contract

**Status:** Accepted Version 1.0 target contract

**Owner:** BL-166

**Implementation state:** Planned; this document does not claim that the current filesystem-only runtime emits these records.

## Purpose

FlashGate audit records explain security-relevant decisions and their outcomes without becoming a second application log, a payload archive, or an authorization mechanism. The contract applies consistently to direct STDIO and future local proxy/service modes.

Audit data is diagnostic security evidence. It must not grant access, extend the lifetime of a request, or be required to reconstruct mutable domain state.

## Event identity and correlation

Every audit event has a server-generated, immutable `event_id`. Identifiers are opaque, unique within a server generation, and never derived from a principal, host path, process ID, tool argument, or secret.

Each accepted request receives one server-generated `trace_id`. All policy decisions, domain dispatches, operation lifecycle events, and terminal outcomes caused by that request carry the same trace ID. A child event additionally names its immediate `parent_event_id`; the first event has no parent. Clients may supply protocol correlation values, but those values are recorded only as bounded, redacted attributes and never replace a server-generated identifier.

Future proxy/service hops preserve the server-generated trace ID after the service has authenticated the connection. Proxy-supplied principal, trace, event, or execution-backend values are not authoritative. If an incoming trace value is absent, malformed, oversized, or conflicts with an established connection context, the receiving server starts a new trace and records a safe correlation failure.

## Required event envelope

The durable logical envelope is independent of a particular log encoding:

| Field | Contract |
|---|---|
| `event_id` | Opaque immutable identifier, unique within `service_generation` |
| `trace_id` | Opaque identifier shared by causally related events |
| `parent_event_id` | Immediate causal parent when one exists |
| `timestamp` | UTC event time; ordering never relies on wall-clock time alone |
| `sequence` | Monotonic sequence within the service generation |
| `service_generation` | Changes after restart and scopes identifiers/state |
| `event_type` | Stable bounded category, not a free-form message |
| `phase` | `decision`, `start`, `progress`, or `outcome` |
| `outcome` | Stable result category such as `allowed`, `denied`, `completed`, `failed`, `cancelled`, or `timed_out` |
| `principal_ref` | Server-derived opaque or redacted principal reference when identity exists |
| `profile_ref` / `root_ref` | Stable configured identifiers when relevant; never a raw host path |
| `capability` | Evaluated functional capability when relevant |
| `domain` / `operation` | Stable bounded domain and operation names |
| `policy_reason` | Stable reason code for a decision or outcome |
| `attributes` | Allowlisted, typed, size-bounded supplemental facts |

An implementation may add schema/version and integrity metadata. Unknown fields must not weaken strict limits, and consumers must treat unknown event types as non-authoritative diagnostics rather than authorization evidence.

## Lifecycle

For a request that reaches authorization, the minimum useful sequence is:

1. a policy `decision` event;
2. a `start` event only after authorization succeeds and work is accepted;
3. zero or more bounded `progress` events for long-running work;
4. exactly one terminal `outcome` event for every accepted start.

A denial has a decision event and no start event. Validation failures before identity or policy evaluation may emit a bounded rejection event without inventing principal or root data. Cancellation, timeout, overload, cleanup failure, disconnect, and process/service shutdown use distinct stable outcome or reason codes.

Audit emission does not change the domain transaction result. The audit subsystem receives immutable event values after policy evaluation; it cannot call domains, repeat operations, or mutate their outcomes.

## Redaction and minimization

Audit fields use an allowlist. The following are prohibited by default:

- secrets, credentials, authorization headers, environment values, or file contents;
- raw command lines, tool argument bodies, process environments, or MCP payloads;
- absolute host paths, user home paths, and unredacted transport endpoints;
- client-supplied labels promoted to trusted principal, root, capability, or backend identity;
- raw operating-system errors or stack traces.

Identifiers and stable categories are preferred over values. When a value is necessary for an approved diagnostic purpose, it is length-bounded and passed through centralized redaction before serialization. Redaction failure drops the optional field or event; it never falls back to raw content.

## Capacity, backpressure, and disk-full behavior

Audit memory queues, individual records, attribute counts, serialized bytes, files, and retained bytes are server-bounded. Progress records are coalesced or sampled under the configured policy; decision and terminal records are not silently replaced by progress noise.

When a sink is slow, unavailable, or full:

- no unbounded queue or goroutine may be created;
- the server reports a safe health/diagnostic state outside MCP stdout;
- the failure is counted and, when capacity permits, represented by one bounded synthetic gap record;
- write-capable or otherwise high-risk work fails closed if the active policy requires durable audit evidence before execution;
- lower-risk work follows an explicit configured fail-open or fail-closed policy, never an implicit fallback;
- the runtime never redirects audit output to MCP stdout or an unapproved path.

The selection of which operations require durable pre-execution evidence belongs to capability/risk policy. This contract does not silently assign risk classes or choose deployment-specific storage.

## Rotation and retention

Rotation is atomic at a record boundary. A rotated segment is immutable; failed rotation keeps the current bounded failure state and does not overwrite an unverified segment. Retention is configured by both age and total bytes, with deletion limited to verified audit segments owned by the configured sink.

Retention cleanup is deterministic, oldest-first, bounded per pass, and separately observable. It never follows links/reparse points, broadens directory permissions, deletes an active segment, or treats audit retention as domain temporary-resource cleanup. Concrete paths, ACLs/owners, retention periods, and byte limits are deployment policy and require native platform validation.

## Log-injection resistance

Structured serialization is authoritative. Untrusted strings cannot select field names, event types, severity, record delimiters, or output destinations. Text renderers escape control characters and line breaks so one logical event remains one record. Parsers use strict decoding, reject duplicate security-significant keys, and enforce limits before allocation or persistence.

## Failure and restart semantics

A restart creates a new service generation and sequence. An unclean shutdown may leave an accepted start without a terminal event; the next generation may emit a bounded recovery finding that references only validated prior-generation identifiers. It must not fabricate a successful or failed domain outcome.

Sink writes use deterministic error categories. Automatic retries, if enabled for a local sink, are bounded and idempotent by event ID. Remote audit export is outside this Version 1.0 contract and requires a separate security and external-provider decision.

## Validation requirements

Permanent tests must cover:

- immutable event IDs, per-generation uniqueness, sequence ordering, and parent/trace propagation;
- a decision and exactly one terminal outcome for accepted work;
- denial without dispatch, plus cancellation, timeout, overload, disconnect, and shutdown outcomes;
- field allowlisting, centralized redaction, control-character escaping, duplicate-key rejection, and record-size limits;
- bounded queue/backpressure behavior and configured fail-open/fail-closed policy;
- disk-full, partial-write, rotation, retention, restart, and recovery-gap behavior;
- absence of secrets, raw payloads, absolute host paths, and raw OS errors;
- proxy/service correlation that rejects client- or proxy-asserted identity authority.

Windows finalization must exercise the selected file/Event Log destination, ACL/owner, rotation, disk-full, and service-identity behavior. Linux finalization must exercise the selected file/journald destination, ownership/mode, rotation, disk-full, and service-account behavior. Until those implementations and native checks exist, this remains a target contract rather than runtime evidence.
