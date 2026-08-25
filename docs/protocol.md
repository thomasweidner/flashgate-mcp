# MCP Protocol and Local Transport Architecture

## Status

This document distinguishes the current implemented protocol from the accepted Version 1.0 target. It does not advertise a protocol revision or extension until implementation and compatibility tests pass.

## Current implementation

The current server:

- communicates through UTF-8 JSON-RPC messages over STDIO;
- implements `initialize`, `tools/list`, and `tools/call` plus the accepted initialized notification behavior;
- advertises MCP revision `2025-11-25`;
- uses deterministic tool ordering;
- validates request envelopes and method parameters before dispatch;
- writes protocol messages only to stdout;
- returns successful filesystem results as one compact JSON text block plus the same object in `structuredContent`;
- exposes successful `outputSchema` definitions for the current eight tools;
- retains safe generic JSON-RPC errors for current tool failures.

`flashgate-mcp --version` and `flashgate-mcp --version --verbose` are pre-protocol CLI exits. They print build identity and terminate before reading MCP input. During normal server operation, stdout remains reserved exclusively for JSON-RPC protocol messages.

## Version 1.0 runtime transports

The same native binary supports four public roles:

| Mode | External MCP side | Internal side |
|---|---|---|
| `stdio` | JSON-RPC/MCP over stdin/stdout | direct in-process core |
| `proxy` | JSON-RPC/MCP over stdin/stdout | authenticated local service protocol |
| `auto` | JSON-RPC/MCP over stdin/stdout | service when safely compatible; otherwise direct STDIO only for absence/unavailability cases allowed by policy |
| `service` | Named Pipe on Windows or Unix Domain Socket on Linux | core through selected execution backend |

The local service transport is not a remote MCP endpoint. It is a versioned local IPC contract with OS-derived peer identity, endpoint ACLs/ownership, bounded framing, handshake, feature negotiation, deadlines, cancellation, and explicit error mapping.

## Auto-mode fallback

Auto mode must distinguish:

- no compatible service endpoint present;
- endpoint transiently unavailable;
- authorization denied;
- policy denied;
- protocol/feature incompatible;
- service configuration invalid.

Only explicitly allowed absence/unavailability cases may fall back to direct STDIO. Authorization, policy, identity, or compatibility rejection must fail closed and must never be bypassed by direct fallback. Auto mode never installs a service, requests elevation, or changes policy.

## Initialization and negotiation

Version 1.0 initialization includes:

- explicit supported MCP revision selection;
- negotiated extension set;
- deterministic server information;
- active profile/capability-derived tool catalog;
- compact profile-specific server instructions;
- catalog fingerprint and cache invalidation inputs where supported by the selected protocol contract.

The server instructions prioritize efficient usage: batch rather than repeated scalar calls, ranges/pages rather than unbounded content, exact field selection, dry-run before risky multi-step work, and cursor continuation for process/search results. Instructions are bounded and benchmarked.

## Tool discovery

`tools/list` is deterministic for a fixed tuple of:

```text
protocol revision
extension set
profile
capabilities
risk policy
schema version
relevant configuration
```

The catalog fingerprint changes whenever that tuple changes. Tool annotations aid clients but do not authorize operations.

## Tool result classes

Version 1.0 defines representation by payload class instead of applying one envelope pattern to all data:

- small metadata may use compact text/structured parity within budget;
- structured pages use typed entries, counters, and cursors with no duplicated page payload;
- text payload appears once plus compact metadata;
- binary/media payload uses bounded inline representation or an opaque result/resource handle;
- long-running work returns an operation/job handle and bounded status/result pages.

All handles are opaque, random, expiring, and bound to principal, root, profile, capability set, execution backend, service instance, and operation ownership. Host absolute paths are never encoded into public URIs.

## Large-result resources

Version 1.0 may expose identity-bound URIs such as:

```text
flashgate://result/<opaque-id>
flashgate://operation/<opaque-id>/result
flashgate://process/<opaque-id>/stdout
```

The exact URI and MCP resource mapping are finalized by the relevant contract task. Reading a resource repeats authorization and state-binding checks. Inline thresholds, TTL, paging, deletion, and client fallback are explicit. Large binary data is not Base64-embedded by default.

## Errors

The protocol adapter maps domain errors without exposing host paths, raw OS errors, command lines, credentials, or internal identifiers. Version 1.0 defines stable machine-readable tool-error data while preserving negotiated-client compatibility.

Errors distinguish at least:

- invalid arguments;
- unavailable capability/tool;
- authentication or authorization denial;
- path/root/policy denial;
- not found or conflict;
- unsupported operation or backend;
- resource/limit/queue exhaustion;
- cancellation/deadline;
- incompatible protocol/extension/service version;
- unexpected internal I/O failure.

No error permits auto-mode fallback after an authorization, policy, identity, or compatibility rejection.

## Stateless-core and Tasks planning

Version 1.0 architecture does not bind durable authorization or operation ownership solely to one transport connection. Server-side state is addressed through identity-bound handles and survives only within explicit TTL and lifecycle rules.

The final supported protocol matrix decides whether and how the MCP Tasks Extension is exposed. FlashGate must not combine the experimental 2025 task lifecycle with a later final extension contract. Internal Operations/Job Manager semantics remain protocol-independent and are adapted only after negotiation.

Deprecated MCP Roots, Sampling, and Logging are not architectural dependencies. FlashGate named roots are server configuration and authorization objects, not client-provided trust roots.

## Host lifecycle and connection ownership

EOF or transport close is a host-lifecycle event, not a public tool result.
Direct STDIO needs no proprietary MCP heartbeat: normal EOF, definitive
transport failure, supported explicit shutdown, or an OS stop signal enters
the process-root bounded shutdown path defined by ADR-0017.

Lease or heartbeat behavior is permitted only as internal, versioned, and
explicitly negotiated IPC between FlashGate-controlled proxy/service or future
broker/worker peers. It is never silently inferred from public MCP traffic.
Every proxy connection has a connection/session ownership identity. Disconnect
cancels and cleans connection-owned work and partial results, but does not stop
the persistent SCM/systemd service.

## Limits and backpressure

Every transport enforces bounded:

- frame/message and argument bytes;
- response and inline payload bytes;
- connection count;
- per-principal and global concurrency;
- queue depth and wait time;
- cursor/result/job/process handles;
- buffered stdout/stderr and IPC data;
- deadlines and cancellation propagation.

Slow readers cannot cause unbounded memory growth. Overload behavior is deterministic, auditable, and fair across principals.

## Compatibility and release rule

The supported protocol/extension matrix is a released artifact. A specification publication date alone does not change FlashGate behavior. Adding or removing a revision, extension, tool schema, representation, or error contract requires implementation, snapshots, positive and negative compatibility tests, documentation, and migration policy.

## Related documents

- [Architecture](architecture.md)
- [Version 1.0 scope](version-1-scope-and-release-boundary.md)
- [Efficiency plan](efficiency-improvement-plan.md)
- [Runtime and service plan](native-multi-mode-runtime-and-service-plan.md)
- [Execution identity backends](execution-identity-backends.md)
- [ADR-0013](adr/0013-mcp-version-and-extension-compatibility.md)
- [ADR-0014](adr/0014-native-multi-mode-runtime-and-local-service-deployment.md)
