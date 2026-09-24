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
- exposes explicit `readOnlyHint`, `destructiveHint`, `idempotentHint`, and `openWorldHint` annotations for every current tool;
- retains safe generic JSON-RPC errors for current tool failures.

The implemented `2025-11-25` annotation values are:

| Tool | Read-only | Destructive | Idempotent | Open world |
|---|---:|---:|---:|---:|
| `list_directory` | `true` | `false` | `true` | `false` |
| `read_file` | `true` | `false` | `true` | `false` |
| `get_path_info` | `true` | `false` | `true` | `false` |
| `write_file` | `false` | `true` | `false` | `false` |
| `create_directory` | `false` | `false` | `true` | `false` |
| `delete_path` | `false` | `true` | `true` | `false` |
| `copy_path` | `false` | `true` | `false` | `false` |
| `move_path` | `false` | `true` | `true` | `false` |

These values are discovery metadata, not authorization. They do not affect registration, profile/capability decisions, root or path checks, security policy, or execution identity. `write_file` and `copy_path` remain non-idempotent at the tool level because their `overwrite:true` variants can replace existing targets.

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

## Revision-specific protocol entry and negotiation

Version 1.0 uses exact revision-specific paths rather than one permanent generation abstraction.

For the `2025-11-25` initialization path:

- retain the `initialize`/`notifications/initialized` contract while that revision is supported;
- select only behavior defined and implemented for that revision;
- preserve current clients until an explicit later deprecation/removal decision.

For the `2026-07-28` stateless path:

- do not use `initialize`, `notifications/initialized`, or a protocol-level MCP session;
- require the request's `_meta` protocol version and client capabilities, with client information treated only as self-reported compatibility/diagnostic metadata;
- implement `server/discover` as required by the revision, while allowing clients to invoke ordinary RPCs without a prior discovery call;
- return `UnsupportedProtocolVersion` for unsupported requested revisions with the exact supported revision set;
- stamp server identity in result `_meta` as defined by the revision;
- emit the revision-required `ttlMs`/`cacheScope` on cacheable list/read results, including `resources/read` when resources are exposed;
- deliver opted-in tool/prompt/resource list-change and `resources/updated` notifications only through a client-opened `subscriptions/listen` stream on this revision; `resources/subscribe`/unsolicited-notification compatibility behavior remains confined to the `2025-11-25` initialization path;
- negotiate only explicitly supported extensions.

The adapter derives the active profile/capability tool catalog, compact server instructions, exact-revision catalog fingerprint, and cache invalidation inputs from current server state. None of these artifacts grants authorization.

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

The catalog fingerprint changes whenever that tuple changes. On the `2026-07-28` path, cacheable list/read results carry the revision-required `ttlMs` and `cacheScope`; this includes `resources/read` for any exposed FlashGate resource. FlashGate plans non-shared/private caching unless the complete result is proven independent of principal/profile-sensitive state. Change notifications can invalidate cacheable state through the negotiated `subscriptions/listen` stream, but cache hints, notifications, annotations, and client cache contents aid efficiency only and never authorize operations.

## Tool result classes

Version 1.0 defines representation by payload class instead of applying one envelope pattern to all data:

- small metadata may use compact text/structured parity within budget;
- structured pages use typed entries, counters, and cursors with no duplicated page payload;
- text payload appears once plus compact metadata;
- binary/media payload uses bounded inline representation or an opaque result/resource handle;
- long-running work returns an operation/job handle and bounded status/result pages.

Wire encoding is revision-specific. The current `2025-11-25` result envelope remains unchanged until its own contract changes. Every successful `2026-07-28` result carries the required `resultType` (normally `"complete"`; `"input_required"` only for the revision's Multi Round-Trip Requests pattern), plus revision-defined result `_meta` such as server identity. Shared domain result objects do not acquire protocol-version fields.

All handles are opaque, random, expiring, and bound to principal, root, profile, capability set, execution backend, service instance, and operation ownership. Host absolute paths are never encoded into public URIs.

Cursor-producing results follow the [stable cursor contract](cursor-semantics.md):
continuations are bound to one deterministic sequence and the current security
context, and source or policy changes invalidate rather than approximately
resume that sequence.

## Large-result resources

Version 1.0 may expose identity-bound URIs such as:

```text
flashgate://result/<opaque-id>
flashgate://operation/<opaque-id>/result
flashgate://process/<opaque-id>/stdout
```

The exact URI and MCP resource mapping are finalized by the relevant contract task. Reading a resource repeats authorization and state-binding checks. Inline thresholds, TTL, paging, deletion, and client fallback are explicit. On `2026-07-28`, any cacheable `resources/read` result carries the revision cache hints and any `resources/updated` delivery uses `subscriptions/listen`; a `2025-11-25` `resources/subscribe` contract is not silently reused on the `2026-07-28` stateless path. Large binary data is not Base64-embedded by default.

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

## `2026-07-28` stateless path and Tasks planning

Version 1.0 architecture does not bind durable authorization or operation ownership solely to one transport connection. The `2026-07-28` path must process each request from its own metadata and current server state; an STDIO process or IPC connection is not a conversation/session authority. Server-side state is addressed through identity-bound handles and survives only within explicit TTL and lifecycle rules.

The supported protocol matrix decides whether and how the final `io.modelcontextprotocol/tasks` Extension is exposed. FlashGate must not combine the experimental 2025 task lifecycle with that final extension contract. Internal Operations/Job Manager semantics remain protocol-independent and are adapted only after exact-revision/extension negotiation. If Multi Round-Trip Requests are used, they are implemented only on revision paths that define them rather than by reintroducing server-initiated JSON-RPC requests.

Deprecated MCP Roots, Sampling, and Logging are not architectural dependencies. FlashGate named roots are server configuration and authorization objects, not client-provided trust roots.

## Host lifecycle and connection ownership

EOF or transport close is a host-lifecycle event, not a public tool result.
Direct STDIO needs no proprietary MCP heartbeat: normal EOF, definitive
transport failure, supported explicit shutdown, or an OS stop signal enters
the process-root bounded shutdown path defined by ADR-017.

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
- [Stable cursor semantics](cursor-semantics.md)
- [Runtime and service plan](native-multi-mode-runtime-and-service-plan.md)
- [Execution identity backends](execution-identity-backends.md)
- [ADR-013](adr/013-mcp-version-and-extension-compatibility.md)
- [ADR-014](adr/014-native-multi-mode-runtime-and-local-service-deployment.md)
