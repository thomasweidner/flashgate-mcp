# FlashGate MCP Security Model

FlashGate MCP is designed as a secure-by-default local host-operations MCP server. It currently exposes only the filesystem functionality described below.

Filesystem access is security-sensitive because MCP clients may request operations on local files. For this reason, all filesystem operations are restricted to a configured sandbox root.

## Core Security Principles

### Sandbox Root

All filesystem paths are resolved relative to a configured root directory.

The root is configured through:

```text
MCP_ROOT
```

`MCP_ROOT` is required. Production roots must be absolute, exist, be accessible under the current policy, and resolve to a directory. Missing, empty, whitespace-only, relative, non-existent, file, and policy-denied roots stop startup before Filesystem, Registry, Router or JSON-RPC processing.

Expected configuration/root failures use safe categories on stderr and exit code `3`; stdout remains empty. Unexpected bootstrap failures use `startup_failed` and exit code `1`. Raw roots and operating-system errors are not emitted.

The process working directory is available only for explicit development use:

```text
MCP_ROOT=.
MCP_ALLOW_CWD_ROOT=true
```

The opt-in accepts only lowercase `true` or `false`, never supplies a missing root, and never enables other relative roots. A successful CWD-development start emits one safe stderr warning. Production and Codex examples set `MCP_ALLOW_CWD_ROOT=false`.

### No Direct Filesystem Access Outside `internal/fs`

Production code outside `internal/fs` must not directly call filesystem APIs such as:

- `os.ReadFile`
- `os.WriteFile`
- `os.ReadDir`
- `os.Remove`
- `os.Rename`
- `os.Open`
- `os.Stat`

All filesystem access must go through the `FileSystem` abstraction.

### Central Path Validation

Path validation is centralized in:

```text
internal/security/PathGuard
```

No other package should implement independent path validation.

## PathGuard

`PathGuard` validates and resolves user-provided paths against the sandbox root.

It protects against:

- empty root paths
- absolute user paths
- parent directory traversal
- resolved paths outside the sandbox root

Path validation is intentionally layered:

1. Lexical validation normalizes the user path and rejects absolute paths and leading parent traversal.
2. Effective path validation uses evaluated filesystem paths to confirm the final existing path, or the nearest existing parent for create targets, remains inside the evaluated sandbox root.

The configured root must exist, pass policy, and resolve effectively to a directory when the server starts. This keeps root comparisons based on the effective filesystem location rather than only string-cleaned paths.

## SafePath

`SafePath` represents a path that has passed validation.

Only `PathGuard` creates `SafePath` values.

This prevents unvalidated user input from being passed directly to filesystem operations.

## Blocked Path Types

### Absolute Paths

User-provided absolute paths are rejected.

Examples:

```text
C:\Windows\System32
/etc/passwd
```

### Parent Traversal

Parent traversal is rejected.

Examples:

```text
..
../secret.txt
../../outside-root
```

### Outside Root Resolution

Even after cleaning and resolving a path, the final effective path must remain inside the configured sandbox root.

Existing paths are evaluated directly. Create targets that do not exist yet are checked by evaluating the nearest existing parent directory before the operation is allowed.

## Destructive Operations

Destructive operations are intentionally conservative.

### Write

`Write()` does not overwrite existing files unless `overwrite=true`.

### Delete

`Delete()` does not delete non-empty directories unless `recursive=true`.

### Move

`Move()` does not overwrite existing targets unless `overwrite=true`. Replacement is restricted to file-to-file, revalidates the observed source and target identities immediately before the rename, and uses `os.Rename` without a separate target deletion. Directory targets are never explicitly removed.

The standard cross-platform API remains path-based: a concurrent writer could exchange the file at the already authorized target path after final revalidation. That residual race is bounded to the target path, cannot invoke directory removal or copy/delete fallback, and is narrower than the previous remove-then-rename sequence.

### Copy

`Copy()` does not overwrite existing targets unless `overwrite=true`.

Directory copy is currently unsupported by design.

## Symlinks

`SPR-036` rejects symlink-based escapes where an existing path, or the nearest existing parent for a create target, resolves outside the configured root.

`SPR-037` adds explicit symlink policy enforcement.

Configuration:

```text
MCP_FOLLOW_SYMLINKS
```

Default: `false`.

When `MCP_FOLLOW_SYMLINKS=false`, existing symlink path components are denied before filesystem operations. Create targets are denied when the nearest existing parent contains a symlink. `list_directory` filters symlink entries instead of exposing them.

When `MCP_FOLLOW_SYMLINKS=true`, classic symlinks may be followed only if the effective target remains inside the effective root. Symlink escapes are still denied. Windows junctions and non-symlink reparse points remain denied by default.

## UNC Paths

UNC path policy is enforced for configured roots and user paths.

Configuration:

```text
MCP_ALLOW_UNC_PATHS
```

Default: `false`.

When `MCP_ALLOW_UNC_PATHS=false`, UNC roots and UNC-style user paths are rejected. When `MCP_ALLOW_UNC_PATHS=true`, UNC roots do not fail solely because they are UNC paths, but the root must still exist and all root containment, hidden-file, symlink, and reparse policies still apply.

## Hidden Files

Hidden file policy is enforced for lexical dot-paths and Windows hidden attributes where available through the standard library.

Configuration:

```text
MCP_ALLOW_HIDDEN_FILES
```

Default: `false`.

When `MCP_ALLOW_HIDDEN_FILES=false`, path components whose names start with `.` are denied, except for `.` itself. Examples include `.git/config`, `.codex/settings`, and `dir/.secret`. Create targets with hidden names are denied. `list_directory` filters hidden entries instead of failing the parent directory.

When `MCP_ALLOW_HIDDEN_FILES=true`, hidden and dotfile paths are allowed if all other policies pass.

## JSON-RPC Boundary

`SPR-038` adds JSON-RPC request validation before MCP dispatch.

Requests must be object-shaped JSON-RPC 2.0 messages. Invalid JSON, invalid request envelopes, unsupported batch requests, invalid IDs, missing methods, and malformed method params are rejected with generic JSON-RPC errors.

Responses for parse errors or invalid requests without a valid request ID include:

```json
{"jsonrpc":"2.0","id":null,"error":{"code":-32600,"message":"invalid request"}}
```

Notifications do not receive JSON-RPC responses. `notifications/initialized` is accepted as a no-op lifecycle notification. Other notifications are not executed, so `tools/call` without an `id` cannot trigger filesystem operations.

Unexpected handler panics are contained at the request boundary and returned as generic Internal error responses when the request requires a response.

Every successful filesystem `tools/call` now crosses one central adapter boundary into MCP `CallToolResult`. The required outer `content` is a text-block array, and `structuredContent` repeats the same already-serialized domain object. The wrapper adds no resolved host paths and leaves the filesystem core protocol-independent. Existing safe JSON-RPC error classification is intentionally unchanged in `SPR-045`; BL-203 owns a later complete `isError=true` migration.

## Limits and Redaction

`SPR-039` adds configurable hard limits for protocol input, tool arguments, filesystem payloads, and response size.

| Environment variable | Default | Scope |
|---|---:|---|
| `MCP_MAX_FILE_SIZE` | `10485760` | Hard cap for `read_file`; client `maxBytes` can only lower it. |
| `MCP_MAX_JSONRPC_MESSAGE_BYTES` | `16777216` | Maximum single JSON-RPC message read from stdin. |
| `MCP_MAX_TOOL_ARGUMENT_BYTES` | `12582912` | Maximum `tools/call` params or arguments payload. |
| `MCP_MAX_WRITE_BYTES` | `10485760` | Maximum `write_file` content size. |
| `MCP_MAX_LIST_ENTRIES` | `1000` | Maximum policy-visible `list_directory` entries. |
| `MCP_MAX_COPY_BYTES` | `10485760` | Maximum `copy_path` source file size. |
| `MCP_MAX_DELETE_ENTRIES` | `1000` | Maximum entries for recursive `delete_path`. |
| `MCP_MAX_RESPONSE_BYTES` | `16777216` | Maximum serialized JSON-RPC response size safety net. |

Limit violations use generic client-visible messages. Filesystem limit denials are mapped to Invalid params with `filesystem error: limit exceeded`. JSON-RPC messages above the configured message cap are rejected as Invalid Request with `id:null`.

`MCP_DEBUG=true` enables minimal stderr diagnostics. Diagnostics are redacted for common authorization headers, token/password/API-key/secret assignments, private-key markers, connection strings with credentials, and absolute host paths. Redaction is a diagnostic safeguard; client-visible security and protocol errors are still built generically instead of exposing raw OS errors.

## Security Testing

Security tests currently cover:

- empty root rejection
- missing, whitespace, relative and development-CWD root contracts
- root existence and directory type
- categorized startup errors, exit codes, empty startup stdout and host-path redaction
- absolute path rejection
- path traversal rejection
- root normalization
- effective root validation
- symlink escape rejection
- symlink deny/follow policy
- Windows reparse point deny behavior where safely detectable
- UNC root and user path denial
- hidden dot-path and Windows hidden-attribute denial
- create-target parent validation
- safe path metadata
- filesystem traversal rejection across list/read/info/write/create/delete/copy/move
- JSON-RPC envelope validation
- explicit `id:null` error responses
- notification no-response and no tool execution behavior
- generic protocol error messages
- JSON-RPC message and tool argument limits
- filesystem read/write/list/copy/delete limits
- response-size safety net
- strict successful `CallToolResult` envelope and text/structured parity without host-path additions
- diagnostics redaction

Startup preflight completes before any tool Registry, Router or MCP server is created. Normal starts remain silent; diagnostics never share JSON-RPC stdout.

## Future Security Work

Version 1.0 security work includes:

- named roots and safe read-only default profile;
- server-side capability enforcement and negative tests;
- payload-class and large-result security;
- Operations/Job Manager quotas and identity-bound state;
- typed command policy and platform isolation;
- local multi-client service threat model;
- hybrid execution-identity backend design;
- Variant A service-account root implementation;
- per-principal fairness and resource limits;
- audit lifecycle and end-to-end correlation;
- supply-chain and release evidence.

Post-Version-1.0 security work includes the Variant B user-worker implementation, user-scoped persistent hosts, conditional read cache semantics, optional accelerators, and any external provider ecosystem.

## Accepted Target Security Architecture

Except where the current-state sections above state otherwise, the controls below are planned and are not implemented yet.

### Gate as a server-enforced boundary

The “Gate” is the authoritative server-side boundary. Tool visibility, descriptions, annotations, profile names, proxy payload claims, and extension negotiation are not authorization.

Every operation passes:

- authenticated caller/principal resolution;
- profile and functional capability checks;
- root policy and execution-backend selection;
- path/process/command/system policy;
- global, domain, and principal resource limits;
- redaction and audit/trace handling;
- platform-adapter validation.

External providers, when later implemented, cannot bypass these controls.

### Safe default profiles and capability enforcement

Version 1.0 target behavior:

```text
no valid root                    -> fail closed
valid root, no explicit profile  -> safe read-only profile
higher-risk profile              -> explicit validated activation
```

Tool registration reflects effective capability but execution checks remain authoritative. Direct calls cannot bypass hidden/unregistered tools.

MCP annotations are accurate hints only and never grant permission.

### Per-root policies and execution backend

Named roots use authoritative FlashGate configuration and root IDs plus relative paths. Each root may define:

- read/write permission;
- file/result/scan/temp limits;
- allowed file types;
- symlink/reparse policy;
- capability mapping;
- process working-directory permission;
- service execution backend.

Version 1.0 system-service roots support `service-account`. A reserved `user-worker` root must fail closed until the post-Version-1.0 backend exists. Tool input cannot choose the backend.

Deprecated MCP Roots is never authoritative and is post-Version-1.0 compatibility work only.

### Operations, handles, resources, quotas, and fairness

Operations/jobs use opaque non-guessable handles. All stateful objects are bound to:

```text
principal + profile + root + execution backend + service generation + expiry
```

This applies to:

- operation/job handles;
- managed process handles;
- process/search cursors;
- large-result/resource handles;
- temporary data;
- cancellation rights;
- authorization-sensitive caches.

Resource control combines:

- global limits;
- per-domain limits;
- per-principal concurrency and queue limits;
- fair scheduling and starvation prevention;
- stored-result/temp-data/process/output limits;
- deterministic overload behavior;
- TTL cleanup and leak detection;
- slow-reader/backpressure handling.

A service restart changes the generation and invalidates stale state.

### Payload and resource boundary

Payload-heavy content must not be duplicated between MCP result fields or IPC layers.

Large text, binary, media, search, tree, and process output uses bounded pages, streams, or identity-bound resource handles. Resource URIs are opaque and never contain raw host paths.

The server enforces:

- MIME/content-class validation;
- raw and encoded byte limits;
- TTL and owner checks;
- safe expiry/restart errors;
- no cross-principal resource access;
- bounded inline compatibility fallback;
- no unrestricted base64 output.

### Managed process identity and control

Server-started processes receive opaque handles. PIDs are diagnostic only and cannot be the sole authority because PID reuse may target the wrong process.

Default control is limited to server-managed processes. External PID control is post-Version 1.0 and requires a separate high-risk capability and threat model.

stdout and stderr are separately bounded. Command lines, environments, and output are minimized and redacted.

### Typed command execution boundary

Version 1.0 command execution uses server-defined command IDs. A definition fixes or constrains:

- absolute executable path and optional binary identity;
- fixed subcommand;
- allowed flags and typed values;
- path arguments bound to named roots;
- working directory;
- environment;
- timeout;
- stdout/stderr limits;
- concurrency;
- network policy;
- OS isolation.

The server constructs argv. Standard profiles do not accept a free shell string, response files, arbitrary config overrides, unapproved hooks/plugins/loaders, or uncontrolled environment inheritance.

Interactive shell and process input remain post-Version 1.0.

### Native OS and interpreter boundary

Normal Version 1.0 runtime prefers:

1. Go standard library;
2. platform-specific Go adapter;
3. direct OS API or stable OS virtual filesystem;
4. allowlisted native OS program without a shell only after benchmark/security evidence.

Python, PHP, Node.js, Java, PowerShell, Bash, or another interpreter is not a required runtime layer for the FlashGate core/service. Scripts may remain development, installation, smoke, or administrator tooling.

### Hybrid service execution identity

A multi-client system service is a local privilege boundary. Every request has:

- an authenticated caller principal;
- an effective execution backend.

Version 1.0 implements Variant A service-account roots:

- dedicated restricted Windows/Linux service identity;
- explicit ACLs for configured service roots;
- no LocalSystem/root convenience default;
- caller-specific FlashGate authorization and quotas;
- OS operation under the service account;
- audit records include caller and effective backend identity.

Version 1.0 defines interfaces and the threat model for Variant B user workers, but the worker is post-Version 1.0.

Variant C shared-process impersonation is prohibited. FlashGate does not switch caller credentials inside the shared Go service process.

The service derives caller identity from Named Pipe or Unix socket peer information. Proxy-supplied identity claims are never authoritative.

### Local IPC and auto-mode boundary

Windows uses a local Named Pipe with restrictive ACLs. Linux uses a local Unix Domain Socket with restrictive ownership/mode and OS peer credentials.

The IPC contract includes framing, size limits, compatibility handshake, correlation, cancellation, overload, resource handles, disconnect behavior, and service generation.

`auto` may fall back to direct STDIO only when no managed endpoint is present or configured. It fails closed after:

- access denial;
- caller authorization failure;
- policy/root/profile denial;
- protocol incompatibility;
- required managed endpoint failure.

It never installs, elevates, or changes service state.

No remote TCP/HTTP listener is included.

### Audit lifecycle and trace correlation

Version 1.0 audit defines bounded structured events with:

- immutable event and correlation IDs;
- caller principal/policy mapping;
- effective profile, capability, root, and execution backend;
- operation/tool and decision;
- result category, counters, and duration;
- proxy/service/backend/job/process correlation.

It also defines:

- rotation and retention;
- maximum queue/buffer/disk use;
- slow/full sink and disk-full behavior;
- log-injection protection;
- redaction before output;
- shutdown flush/drop policy;
- no raw secrets, credentials, full file contents, unrestricted environments, or unnecessary absolute paths.

A heavy telemetry dependency is not required. Optional standard trace-context propagation may build on the compact internal correlation model.

### MCP version and extension compatibility

The implemented revision remains MCP `2025-11-25` until another revision is finalized, implemented, and tested.

Version 1.0 protocol security includes:

- explicit supported-revision matrix;
- adapter-only version/extension logic;
- stateless-core and list-cache/TTL review for the 2026 line;
- final Tasks Extension mapping without mixing the 2025 experimental lifecycle;
- JSON Schema 2020-12 validation;
- capability downgrade/mismatch tests;
- no authorization implication from extension support;
- no architectural dependency on deprecated Roots, Sampling, or Logging.

### Release and supply-chain security

Version 1.0 requires:

- pinned/validated workflow strategy;
- checksums;
- SBOM and dependency inventory;
- build provenance;
- Windows/Linux signing plan and configured signing where available;
- reproducible-build comparison or documented limitations;
- artifact identity/version verification;
- atomic update/rollback guidance;
- no silent automatic update;
- public security policy and supported-version statement.

Build identity is provenance metadata, not a code signature. Release mode
fails closed unless the exact version tag is checked out and the tree is
clean. Controlled builds use canonical source time and `-trimpath`. Every
release matrix path performs two independent builds and rejects differing
binary, archive, checksum, or inventory hashes before upload. A
machine-readable pre-upload scan rejects workspace/home/temp/repository paths,
supplied usernames and hostnames, GitHub/AWS credential shapes,
credential-bearing URLs, and complete private-key blocks in the produced
binaries and all archive files. Bare private-key header literals used by
redaction code are narrowly reported as allowed markers rather than hidden.

Windows and Linux binaries contain the same static canonical build manifest.
Validation binds version, full commit, source time, modified state, GOOS,
GOARCH and public architecture directly to the binary. Windows ARM64 therefore
has an equivalent static provenance gate without relying on native execution
or an unauthenticated sidecar.

Native WSL validation accepts only a restricted direct-child run ID below the
canonical `$HOME/.cache/flashgate-mcp-validation` base. Creation and recursive
cleanup reject symbolic links and descriptor identity changes. Source
snapshots are made from explicit Git tracked/nonignored-untracked inventory;
ignored sensitive names, reparse points, links and special files fail closed.
The native extractor rejects traversal, duplicate names, links and special TAR
types, then verifies every extracted length and SHA-256 against the manifest.

The vendored Windows resource generator and committed icon source are covered by dependency inventory and third-party notices. Generated `.syso` files are temporary, ignored, rejected when stale, and removed on every build exit path.

## Threat Model Workstreams

| Workstream | Minimum concerns |
|---|---|
| Filesystem | traversal, symlink/reparse escape, TOCTOU, overwrite/delete semantics, MIME/binary transfer, result handles, exhaustion, disclosure |
| Search | recursion/regex cost, scanned bytes, encoding, ignores, context/result leakage, cursor ownership |
| Operations/jobs | handle guessing, cross-principal access, fairness, queues, TTL, cleanup, restart, slow readers |
| Processes | PID reuse, lifecycle races, command/environment disclosure, output cursors, orphan cleanup, child limits |
| Typed commands | executable substitution, argument/config/hook/plugin injection, env, roots, output, network, OS isolation |
| System service | endpoint spoofing, peer identity, service-account ACLs, policy/OS permission mismatch, privilege escalation, auto fallback |
| MCP host lifecycle | orphaned direct/proxy hosts, retained pipes, long-lived wrappers, PID reuse, false owner attribution, stale/manipulated instance records, unsafe idle/singleton heuristics, owned-child/job survival, connection-owned state after proxy loss, unrelated-process targeting |
| Future user workers | token/UID/session acquisition, groups/env, broker IPC, worker reuse, cross-user state, resource/crash isolation |
| Payload/resources | amplification, base64 cost, host-path URI leak, owner/TTL checks, stale generation, compatibility fallback |
| MCP versions/extensions | downgrade/mismatch, stateless routing, cache invalidation, Tasks lifecycle, deprecated capability confusion |
| Supply chain | dependency/workflow compromise, artifact substitution, unsigned updates, provenance, rollback |
| Future providers | policy bypass, capability inflation, dependency/update risk, in-process versus IPC isolation |

Stateful components require race-detector coverage, restart/shutdown analysis, negative capability tests, quota/fairness tests, and cleanup verification.

### MCP host lifecycle

This planned Version 1.0 workstream treats top-level host lifecycle separately
from Operations/Jobs and Managed Process ownership. It covers orphaned direct
and proxy-edge hosts; retained or duplicated pipes; long-lived ChatGPT/Codex
app servers and wrapper parents; PID reuse and false owner attribution;
manipulated or stale instance records; unsafe age/idle/CPU/request-count or
singleton reaping; owned child/job survival after host shutdown;
cross-principal or connection-owned service state after proxy loss; and any
attempt to target unrelated processes.

Automatic termination requires conclusive transport and ownership evidence.
PID alone and registry/state records are never authority. Ambiguous live
process/transport/owner cases are `SUSPECTED_STALE`, remain diagnostic, and are
not killed. Instance evidence is bounded and secret-safe; it excludes command
lines, credentials, and unnecessary host paths. See ADR-017 and BL-341.
## Version 1.0 Security Acceptance

Version 1.0 cannot release until:

- safe read-only is the default profile;
- all higher-risk capabilities require explicit activation;
- heavy payloads are bounded and not duplicated;
- service-account roots and endpoint ACLs pass Windows/Linux tests;
- caller and effective execution identity are separate and auditable;
- unsupported user-worker configuration fails closed;
- no in-process impersonation path exists;
- handles/caches/resources/cancellation are execution-context bound;
- per-principal quotas and fairness pass;
- typed command injection and environment tests pass;
- audit lifecycle and disk-full behavior are defined/tested;
- supported MCP versions/extensions are explicit;
- release supply-chain evidence is complete.
- after conclusive owner or transport loss and expiration of the bounded
  shutdown window, zero `DEFINITELY_ORPHANED` session-scoped FlashGate hosts or
  owned child processes remain.

## Deferred Security Decisions

Post-Version-1.0 decisions include:

- Variant B user-worker implementation details;
- user-scoped persistent hosting;
- conditional read/cache authorization semantics;
- external PID control;
- interactive process input/shell;
- privacy-sensitive network information;
- provider runtime and isolation;
- optional accelerators and indexes.

Remote access or a product/binary split requires a separate ADR and threat model.

## Related Documents

- [Architecture](architecture.md)
- [Execution identity backends](execution-identity-backends.md)
- [Native runtime and service plan](native-multi-mode-runtime-and-service-plan.md)
- [Efficiency improvement plan](efficiency-improvement-plan.md)
- [Version 1.0 scope](version-1-scope-and-release-boundary.md)
- [ADR-014](adr/014-native-multi-mode-runtime-and-local-service-deployment.md)
- [ADR-015](adr/015-hybrid-service-execution-identity.md)
- [ADR-017](adr/017-host-process-ownership-and-lifecycle.md)
