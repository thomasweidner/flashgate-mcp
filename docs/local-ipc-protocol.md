# FlashGate Local IPC Protocol and Compatibility Handshake

## Status and scope

This document defines the Version 1.0 contract for the private, local-only
protocol between a FlashGate proxy and a FlashGate service. It also applies to
future FlashGate-controlled broker/worker channels only when those channels
explicitly negotiate the relevant features.

The protocol is not MCP, is not a public remote API, and is not available over
TCP, HTTP, or a network socket. Windows Named Pipes and Linux Unix Domain
Sockets provide the transport. The service authenticates the peer from
operating-system transport information; no identity field in a frame is
authoritative.

This contract does not implement the transports, proxy, service, or future
worker. Their backlog owners remain responsible for implementation and native
Windows/Linux validation.

The Linux transport owner provides a filesystem-backed Unix Domain Socket
listener in `internal/ipc/unixsocket`. It accepts only a clean absolute path,
applies owner read/write with optional group read/write and no permissions for
other users, refuses to replace a non-socket or an answering socket, and
removes a stale socket only after a failed local connection probe and a stable
filesystem-identity check. Accepted connections expose credentials captured
from Linux `SO_PEERCRED` before application dispatch. Listener shutdown removes
only the same socket object that the listener created. Endpoint selection,
runtime-directory provisioning/ownership, protocol framing, authorization,
systemd hosting, and service lifecycle remain with their respective owners.

## Security invariants

Every implementation must preserve these invariants:

- authenticate and authorize a connection before accepting work;
- derive the caller principal from OS peer information, never proxy claims;
- bind connection-owned state to a fresh, unguessable session identifier and
  the current service generation;
- bind durable handles additionally to their principal, profile, root,
  capability set, execution backend, and expiry;
- enforce frame, connection, in-flight request, queue, deadline, result, and
  buffered-byte limits before resource allocation;
- reject unknown message kinds, flags, features, versions, and fields when the
  negotiated protocol does not define them;
- use normalized errors without host paths, credentials, raw command lines,
  environment dumps, OS error strings, or internal security identifiers;
- cancel and clean partial connection-owned work after disconnect without
  stopping the persistent service; and
- never make `auto` mode fall back to direct STDIO after access denial,
  authorization or policy rejection, incompatibility, or another fail-closed
  managed-endpoint result.

## Transport preconditions

Before protocol bytes are processed, the transport adapter must establish:

1. the endpoint is the configured local Named Pipe or Unix Domain Socket;
2. endpoint ACL or ownership/mode policy passed;
3. the OS peer identity was obtained and mapped to a candidate principal; and
4. the configured connection and pre-handshake byte/time limits are available.

Failure closes the connection without application dispatch. Transport setup,
peer authentication, handshake parsing, and authorization all remain bounded
and independently fail closed.

## Binary framing

Each frame uses this fixed 16-byte network-byte-order header followed by an
opaque payload:

| Offset | Size | Field | Contract |
|---:|---:|---|---|
| 0 | 4 | `magic` | ASCII `FGIP` (`0x46 0x47 0x49 0x50`) |
| 4 | 1 | `framing_version` | `1` for this framing contract |
| 5 | 1 | `kind` | Message kind from the closed table below |
| 6 | 2 | `flags` | Network-order bit set; all unnegotiated/reserved bits are zero |
| 8 | 8 | `payload_length` | Unsigned network-order length of the following payload |

The receiver reads exactly one header, validates it, checks
`payload_length` against its configured hard maximum, reserves bounded storage,
and then reads exactly that payload. It must not allocate from an unchecked
length or scan for delimiters. EOF inside a header or payload is a truncated
frame and closes the connection after connection-owned cleanup.

Version 1.0 defines these frame kinds:

| Value | Kind | Direction | Payload |
|---:|---|---|---|
| `1` | `client_hello` | client to service | strict JSON handshake object |
| `2` | `server_hello` | service to client | strict JSON handshake result |
| `3` | `request` | client to service | negotiated application envelope |
| `4` | `response` | service to client | negotiated application envelope |
| `5` | `cancel` | either | strict JSON cancellation envelope |
| `6` | `event` | service to client | negotiated bounded event envelope |
| `7` | `close` | either | strict JSON close envelope |
| `8` | `heartbeat` | either | strict JSON lease envelope; negotiated only |

Handshake, cancel, close, and heartbeat payloads are UTF-8 JSON objects using
strict decoding: duplicate keys, unknown keys, invalid UTF-8, trailing values,
non-integer numbers, and explicit `null` where a value is required are rejected.
Application payload encoding is selected by the handshake. Version 1.0
requires the `json` encoding and permits later encodings only through a new
protocol version or an explicitly defined optional feature.

No compression flag exists in Version 1.0. Fragmentation is not implicit: one
frame contains one complete envelope. Large results use bounded pages or
identity-bound resource handles rather than oversized or unbounded frames.

## Compatibility handshake

The client sends exactly one `client_hello` as its first frame. No other frame
may be processed until the service returns one `server_hello`. A handshake
timeout, malformed hello, or extra pre-handshake frame closes the connection.

The Version 1.0 `client_hello` fields are:

| Field | Type | Meaning |
|---|---|---|
| `protocol_min` | positive integer | Oldest IPC protocol revision accepted by the client |
| `protocol_max` | positive integer | Newest IPC protocol revision accepted by the client |
| `client_role` | string | Closed value: `proxy` in Version 1.0; later revisions may define `worker` |
| `client_build` | string | Bounded diagnostic build identity; never authorization input |
| `encodings` | array of strings | Ordered supported encodings; Version 1.0 includes `json` |
| `features` | array of strings | Sorted, unique optional features the client offers |
| `required_features` | array of strings | Sorted, unique subset required for this connection |
| `limits` | object | Client receive ceilings for frames and concurrent requests |

The service selects the greatest protocol revision in the inclusive
intersection of the client range and its configured supported range. It never
selects a revision outside either range. It selects one mutually supported
encoding and only features both peers support. Every required feature must be
selected. Build identity, executable version, and branch/commit equality are
diagnostic and never replace protocol/feature negotiation.

The Version 1.0 `server_hello` fields are:

| Field | Type | Meaning |
|---|---|---|
| `accepted` | boolean | Whether application frames may follow |
| `protocol` | positive integer, conditional | Selected revision when accepted |
| `encoding` | string, conditional | Selected payload encoding when accepted |
| `features` | array of strings | Sorted, unique selected optional features |
| `service_build` | string | Bounded diagnostic build identity |
| `service_generation` | string, conditional | Fresh opaque identifier for this service lifetime |
| `session_id` | string, conditional | Fresh opaque identifier for this connection |
| `limits` | object, conditional | Effective send/receive and concurrency ceilings |
| `error` | object, conditional | One normalized handshake error when rejected |

When `accepted` is true, every conditional success field is present and
`error` is absent. When it is false, `error` is present and the conditional
success fields are absent. A rejection uses a stable category such as
`incompatible_version`, `missing_feature`,
`unsupported_role`, `unauthorized`, `overloaded`, or `protocol_error`, then the
service closes the connection. The client treats every rejection other than a
proved `endpoint_absent` result from endpoint discovery as fail closed.

Negotiated limits are the lower applicable ceiling advertised by the two
peers and the service's administrative policy. Negotiation can only reduce a
server policy limit. It cannot grant authorization, increase a quota, or
weaken a root/profile/capability policy.

## Version and feature rules

Protocol revisions are positive integers. Revision `1` is the contract in this
document. Implementations may support a bounded contiguous compatibility
window, but they must advertise the real minimum and maximum and select one
revision explicitly. There is no optimistic use of an unknown higher version.

Optional features use stable lowercase dotted identifiers. Version 1.0
reserves these feature contracts:

| Feature | Meaning |
|---|---|
| `cancel.v1` | Correlated cancellation frames |
| `events.v1` | Bounded service event frames |
| `resources.v1` | Identity-bound paged/resource-handle results |
| `lease.v1` | Explicit heartbeat/lease exchange for controlled peers |

`cancel.v1` is required for proxy/service operation. Other features are
optional unless configuration or the requested role requires them. Receiving
a frame that depends on an unselected feature is a protocol error.

Adding a message kind, changing field meaning, weakening strict validation, or
changing an authorization or lifecycle invariant requires a new protocol
revision. An additive optional behavior with a safe absence path may use a new
feature identifier. Unknown optional features are not selected; unknown
required features reject the handshake.

## Application envelopes and correlation

Every `request`, `response`, `cancel`, and request-related `event` carries:

- `request_id`: a nonzero unsigned 64-bit integer chosen uniquely by the
  initiating peer for the lifetime of the session;
- `correlation_id`: a bounded opaque identifier propagated into diagnostics
  and audit without granting access;
- `service_generation`: the negotiated generation identifier; and
- `session_id`: the negotiated connection identifier.

Requests additionally carry an operation name, a relative deadline budget in
milliseconds measured against a monotonic clock, and an operation payload.
They never carry an authoritative principal, profile, capability, root, or
backend selection.
Those values come from authenticated server-side connection and policy state.

Responses contain exactly one success result or normalized error. A response
must match one currently in-flight request from the same session and service
generation. Duplicate, zero, unknown, completed, cross-session, or stale-
generation request identifiers are rejected without dispatch or information
disclosure. Correlation identifiers are not lookup keys and are never reused
as authorization tokens.

Peers apply explicit per-session and per-principal in-flight limits before
dispatch. A request that exceeds a negotiated limit receives a bounded
`overloaded` or `limit_exceeded` result when safe; otherwise the connection is
closed. Slow readers have bounded output queues and cannot cause unbounded
retention.

## Cancellation and deadlines

With `cancel.v1`, `cancel` names an in-flight `request_id`, generation, and
session. Cancellation is idempotent and scoped to the authenticated owner. An
unknown, completed, stale-generation, or foreign request is reported only as a
generic unavailable result so ownership is not disclosed.

The service derives the effective deadline as the earliest applicable client
budget, server operation maximum, administrative policy deadline, transport
shutdown deadline, or parent context deadline. A client cannot extend a
server-owned maximum. Deadline and cancellation propagate through the
application layer to the owning domain. Operations/Job and Managed Process
owners retain responsibility for their own terminal state and cleanup.

Cancellation acceptance does not promise instantaneous termination. The final
response is the first authoritative terminal outcome accepted by the domain;
races between completion, cancellation, timeout, disconnect, and shutdown do
not replace an already accepted terminal outcome.

## Disconnects, close, and partial work

A `close` frame is advisory graceful shutdown of the connection, not authority
to stop the service. The peer stops accepting new requests, completes or
cancels work according to the close reason and bounded drain policy, and then
closes the transport.

Transport EOF, truncation, broken pipe, authentication loss, or protocol error
is an abrupt disconnect. On either graceful or abrupt disconnect the service:

1. rejects new work for the session;
2. cancels partial and connection-owned work;
3. invokes bounded cleanup through the owning domain;
4. discards unsent partial frames and connection-owned buffers; and
5. records a bounded, redacted outcome.

A normal proxy disconnect never stops the SCM/systemd service. Explicitly
durable work may outlive a connection only when its contract, authorization,
principal-bound handle, expiry, and reconnect semantics say so. Merely having
a handle or request ID does not make work durable.

Service restart creates a new `service_generation`. Requests, cancellation,
handles, resources, and buffered results bound to an older generation fail as
unavailable unless a separate durable contract explicitly defines validated
recovery. No PID, endpoint name, or elapsed time substitutes for generation
identity.

## Lease and heartbeat

Direct STDIO never uses this protocol and requires no proprietary heartbeat.
Proxy/service connections do not infer death from idle time, request count, or
CPU use.

`lease.v1` may be negotiated only between FlashGate-controlled peers. The
handshake supplies bounded `interval_ms` and `expiry_ms` values, with expiry
strictly greater than the interval and both constrained by server policy.
Heartbeat frames carry the session and generation plus a monotonically
increasing sequence. They carry no application data or credentials.

Lease expiry is a definitive disconnect signal only after successful feature
negotiation and the complete negotiated expiry interval. Reconnect performs a
new handshake and receives a new session; it cannot revive old connection-
owned work. Unsupported, malformed, stale-session, or stale-generation
heartbeats fail closed.

## Error model

Handshake and application errors use a closed object with:

- `category`: stable machine-readable category;
- `message`: short safe text;
- `retryable`: boolean bounded by the category; and
- `details`: optional bounded non-sensitive structured fields defined by the
  selected revision.

Version 1.0 categories include `protocol_error`, `incompatible_version`,
`missing_feature`, `unsupported_role`, `unauthenticated`, `unauthorized`,
`policy_denied`, `not_found`, `conflict`, `cancelled`, `deadline_exceeded`,
`limit_exceeded`, `overloaded`, `backend_unavailable`, `service_shutting_down`,
and `internal_error`.

`retryable` is never permission to switch to direct mode or bypass the managed
endpoint. Authentication, authorization, policy, compatibility, and protocol
errors are not retryable on the same connection. Backoff and reconnect limits
remain client policy and must be bounded.

## Upgrade and mismatch behavior

Upgrade order must preserve an overlapping protocol revision between proxy and
service. Administrators upgrade the persistent service first only when the old
proxy revision remains inside the new service compatibility window; otherwise
they update compatible proxy artifacts before activating the service. Rollback
uses the same overlap rule.

There is no build-number equality requirement. If no protocol intersection or
required-feature set exists, the handshake returns an incompatibility error,
the connection closes, and `auto` fails closed because a managed endpoint was
present but incompatible. Repeated blind reconnects are prohibited.

## Validation obligations

Implementations must add focused unit/contract tests plus real Windows Named
Pipe and Linux Unix Domain Socket integration evidence for:

- fragmented reads and multiple sequential frames;
- every malformed header, length overflow, truncation, strict-JSON failure,
  unknown kind/flag/field, and pre-handshake ordering error;
- version intersection, required/optional feature negotiation, limit
  reduction, role rejection, and build-version independence;
- OS-derived identity and rejection of payload identity claims;
- correlation uniqueness, cross-session and stale-generation rejection;
- cancellation/deadline races and first-terminal-outcome preservation;
- per-connection, per-principal, queue, buffer, and slow-reader limits;
- normal and abrupt disconnect cleanup while the persistent service remains;
- service restart, proxy/service upgrade/rollback, and mismatch failure;
- optional lease timing, reconnect, stale heartbeat, and long legitimate idle;
- safe normalized errors and audit/correlation redaction; and
- proof that no network listener, implicit fallback, or stdout corruption was
  introduced.

Native transport, peer-credential, ACL/ownership, lifecycle, race, and service
evidence remains required during Windows/Linux finalization; documentation or
loopback fakes cannot replace it.

## Related documents

- [ADR-014: Native Multi-Mode Runtime and Local Service Deployment](adr/014-native-multi-mode-runtime-and-local-service-deployment.md)
- [ADR-017: Host Process Ownership and Lifecycle](adr/017-host-process-ownership-and-lifecycle.md)
- [Architecture](architecture.md)
- [Protocol architecture](protocol.md)
- [Security model](security.md)
- [Native multi-mode runtime and service plan](native-multi-mode-runtime-and-service-plan.md)
- [Testing](testing.md)
