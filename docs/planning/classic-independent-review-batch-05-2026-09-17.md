# FlashGate MCP — Classic Independent Review Batch 05

**Status:** `COMPLETE_FINDINGS_CONFIRMED_NO_FIXES_PERFORMED`  
**Date:** 2026-09-17  
**Review mode:** `INDEPENDENT_REVIEW`  
**Repository:** `thomasweidner/flashgate-mcp`  
**Bound main:** `03a4846e4a910231ef9e7a8e314de09c5f0112b3`  
**Bound main tree:** `c541561e6e6464858936d5b367f255fbc78e7081`  
**RepositoryMutationAllowed:** `false`  
**ExternalMutationAllowed:** `false`  
**FindingFixesPerformed:** `false`  
**ReviewerIndependencePreserved:** `true`

This review was performed in Classic as a separate read-only review activity. GitHub/Codex review comments were treated only as evidence pointers and hypotheses. Every disposition below was independently checked against the reviewed implementation/contracts, current repository behavior, current backlog ownership, ADR-015, the current public configuration contract, and relevant authoritative operating-system API documentation.

No reviewed PR, branch, review thread, repository file, remote state, workflow, issue, or external system was modified during this review.

## Authority bound for this review

- `AGENTS.md` blob: `079fad212fc4843eec70999cb551495b226087e6`
- `BACKLOG.md` blob: `a02db7c09b10dba8c827f62618c7c6eb9f096591`
- `Governance/CLOUD-CODEX-GOVERNANCE.md`: `5a8295d61a0d13a624471fa3438c2e6157d9fb60`
- `Governance/CHANGE-TRIGGER-REVIEW-AND-BACKLOG-STANDARD.md`: `07ac0d53520b70d71c8c96fd419d55134e890692`
- `Governance/FINDING-REMEDIATION-AND-REVIEW-MODE-STANDARD.md`: `89622d7c04af0359b3dcc34e5ee5eeb637ced7f0`
- `Governance/HANDOFF-ARTIFACT-AND-CLASSIC-READINESS-STANDARD.md`: `4bdb44bb160b003969d5ccbc157d2415d9e4ed1e`
- ADR-015 hybrid service execution identity: `82d5b4db7910a50e12e55bb15b0f5eba960865f9`
- ADR-017 host ownership/lifecycle: `248fac8d422fc7fe43367ea48586ecb558bf2b4c`

Relevant backlog ownership:

- BL-226: secure local Windows Named Pipe transport, restrictive ACLs, OS-derived caller identity, bounded framing and cancellation.
- BL-227: secure local Linux Unix Domain Socket transport, restrictive ownership/mode, authoritative peer credentials, bounded framing, cleanup and stale-endpoint handling.
- BL-233: configuration precedence/paths, endpoint discovery, bounded timeouts/fallback and Event Log/journald/**user logs**.
- BL-236: backend-neutral caller/policy/backend/dispatch/OS-adapter execution-identity boundary.
- BL-237: Variant-A dedicated least-privilege service-account backend with root grants, deterministic denial and dual caller/effective audit.

ADR-015 permanently excludes in-process impersonation in the shared multi-threaded Go service unless a later explicit security decision supersedes it. Its accepted execution context includes caller principal, effective profile, root, functional capability, execution backend, service generation, correlation/protocol context, deadline and resource budget.

Current public runtime configuration remains based on `MCP_*` variables such as `MCP_ROOT`, `MCP_READ_ONLY`, `MCP_MAX_*`, and `MCP_DEBUG`.

## Reviewed PR bindings

| PR | BL | Head | Tree | Base/parent |
|---:|---|---|---|---|
| #255 | BL-227 | `e8366ac0ad5506de5944902a7ed9c21639dc6bb8` | `5ad5c1b1f373e6ff3371fdeb1d6a637e7b985591` | PR #210 / BL-225 |
| #256 | BL-226 | `f11b0a427b12410e4660cc420dca1657808fa5d1` | `3838f030930101c791777e6a5039158eb8a0ef98` | PR #210 / BL-225 |
| #205 | BL-233 | `e084895636e92e22f0b85f9e97d6e914f740ce81` | `4b07aa6ea941803ab230ccee2104103c7eabd9e7` | main-based |
| #220 | BL-236 | `57d0b7d04cccf1d99e02679fcba32d1018fc5217` | `e4dcbc939f1df5ee1afdbd4c466711a4f2b882ac` | main-based |
| #257 | BL-237 | `d92002cf7ea4d02e7bf8aedd8385fee1be9fe519` | `bc0ffa77920d534aea4f8d1ae740ee60bdfd8021` | PR #220 / BL-236 |

## External platform evidence consulted read-only

For the Windows Named Pipe findings, Classic consulted Microsoft Learn contracts for Named Pipe Security and Access Rights, File Access Rights Constants, `CreateNamedPipe` / `FILE_FLAG_FIRST_PIPE_INSTANCE`, `ConnectNamedPipe`, Synchronous and Overlapped Pipe I/O, and `CancelSynchronousIo`.

These contracts confirm, among other points, that `FILE_APPEND_DATA` and `FILE_CREATE_PIPE_INSTANCE` share the same value for named pipes; generic write rights therefore grant pipe-instance creation and individual rights should be used when that is not intended; `FILE_FLAG_FIRST_PIPE_INSTANCE` makes later attempts to create the first namespace owner fail; synchronous `ConnectNamedPipe` does not return until a client connects or an error occurs; synchronous pipe operations can block for an indefinite period; and `CancelSynchronousIo` is the documented cancellation mechanism for pending synchronous I/O.

For the Linux stale-socket probe, Classic consulted the POSIX/Linux `connect()` contract: on a connection-mode socket without nonblocking mode, a connect that cannot complete immediately may block for an unspecified timeout interval.

No external mutation occurred.

# Findings — Linux Unix Domain Socket transport

## CR-PR255-001 — P1 — CONFIRMED
### Stale-socket verification and unlink are not atomic

`prepareEndpoint` performs `Lstat(path)` and records the original socket identity; a connection probe; another `Lstat(path)`; `os.SameFile(first, second)`; then `os.Remove(path)` by pathname.

The identity check protects against replacement up to the second `Lstat`, but there is still a race between that check and the pathname unlink. Another FlashGate start/restart under the same service identity can replace the socket after the check; this instance can then unlink the newly created endpoint.

The parent directory is service-owned and not group/world writable, which excludes untrusted ordinary users from that race, but it does not serialize concurrent service instances or restarts running under the same trusted service identity. BL-227 explicitly owns safe stale-socket handling.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required correction:** make stale endpoint replacement atomic with respect to the verified filesystem object or serialize endpoint ownership/startup so the verified object cannot be replaced before unlink. Preserve fail-closed behavior when identity changes.

## CR-PR255-002 — P2 — CONFIRMED
### The socket can be reachable before restrictive endpoint permissions are applied

The implementation calls `net.ListenUnix` first and only afterward runs `os.Chmod(path, mode)`.

The filesystem socket is published by `ListenUnix` before the explicit restrictive mode is applied. If the process umask permits broader access and the service-owned parent directory is searchable, an unauthorized local process can race a connection during that window. A connection already queued before the later `chmod` is not retroactively rejected by the permission change.

BL-227 requires restrictive ownership/mode as part of the local endpoint security boundary.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required correction:** ensure restrictive permissions hold at endpoint publication time, for example through a tightly controlled temporary umask/creation strategy or another reviewed method that does not create a wider-access window. Native Linux evidence is required.

## CR-PR255-003 — P2 — NEW CLASSIC FINDING
### The stale-endpoint liveness probe has no server-owned timeout

`prepareEndpoint` probes an existing socket with `net.DialUnix("unix", nil, &net.UnixAddr{Name: path, Net: "unix"})`. There is no context, deadline, nonblocking connect, or other server-owned bound.

The POSIX connection-mode `connect()` contract permits a blocking connect without `O_NONBLOCK` to wait for an unspecified timeout when the connection cannot be established immediately. A wedged or pathological existing endpoint can therefore make FlashGate startup/stale-endpoint handling wait outside any FlashGate-owned deadline.

That contradicts the project's bounded-work posture and BL-227's requirement for deterministic stale-endpoint handling.

**Required correction:** perform the liveness probe through a server-policy-bounded deadline/context or equivalent nonblocking mechanism and fail closed when liveness cannot be established within the bound.

**Prior Codex signal:** none; this finding was produced by Classic review.

# Findings — Windows Named Pipe transport

## CR-PR256-001 — P1 — CONFIRMED
### Shared-process client impersonation violates ADR-015 and is unsafe for the Go execution model

`peerIdentity` calls `ImpersonateNamedPipeClient`, reads the thread token, then calls `RevertToSelf`.

ADR-015 explicitly and permanently excludes in-process caller impersonation inside the shared multi-threaded Go service. The reason is exactly the thread-affine risk: goroutine scheduling, nested/canceled work and reversion failures can contaminate unrelated requests.

The implementation also does not lock the goroutine to one OS thread around the impersonation window, which makes the thread-affinity problem concrete even before applying the architectural prohibition.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required boundary:** derive authoritative client identity without switching the shared service thread's effective identity, or supersede ADR-015 through a separate explicit security decision. No such decision is made in this review.

## CR-PR256-002 — P1 — CONFIRMED
### The authenticated-users DACL grants pipe-instance creation rights

The endpoint DACL contains `(A;;GRGW;;;AU)` for authenticated users.

Microsoft documents that generic write on a named pipe includes `FILE_APPEND_DATA`; `FILE_APPEND_DATA` and `FILE_CREATE_PIPE_INSTANCE` share the same bit value for named pipes. Consequently the authenticated-user grant allows creation of pipe instances in the namespace, not merely client data read/write.

That is broader than BL-226's client-connect requirement and weakens endpoint ownership.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required correction:** use the necessary individual client data/attribute rights without `FILE_CREATE_PIPE_INSTANCE`, with native ACL verification.

## CR-PR256-003 — P1 — CONFIRMED
### The first server instance does not reserve the named-pipe namespace

`CreateNamedPipeW` is called without `FILE_FLAG_FIRST_PIPE_INSTANCE`.

Microsoft documents this flag specifically so the first namespace owner succeeds and later attempts fail. Without it, another local process can pre-create or race the expected pipe namespace. The intended FlashGate service can then join/fail against an already-owned namespace rather than proving it established the endpoint itself.

BL-226 requires a secure local endpoint whose authority derives from FlashGate/OS policy rather than a preclaimed proxy object.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required correction:** reserve/validate first-instance ownership fail-closed before accepting clients, together with the corrected DACL.

## CR-PR256-004 — P2 — CONFIRMED
### Reserved Version-1 frame flags are accepted and emitted

The Version-1 frame contract reserves the flags field and requires unsupported/unnegotiated bits to be rejected.

The Windows transport copies the frame's `Flags` field through read/write framing without rejecting nonzero reserved bits.

This lets peers establish wire behavior outside the accepted BL-225 Version-1 contract and makes future feature negotiation ambiguous.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required correction:** reject nonzero/unnegotiated flags on both receive and send until an explicit negotiated feature assigns them.

## CR-PR256-005 — P1 — NEW CLASSIC FINDING
### `Accept(ctx)` has no guaranteed bounded cancellation of synchronous `ConnectNamedPipe`

The pipe instance is created in synchronous/blocking mode. `Accept` starts `connectNamedPipe.Call(uintptr(h), 0)` inside a goroutine. On `ctx.Done()` the caller removes the pending handle from the listener, calls `CloseHandle` on the pipe handle, then waits unconditionally on `<-done>`.

The wait itself has no deadline.

Microsoft documents that synchronous `ConnectNamedPipe` does not return until a connection or error, and that synchronous pipe I/O can block indefinitely. Microsoft provides `CancelSynchronousIo` for pending synchronous operations; this implementation does not use the documented synchronous-I/O cancellation mechanism and instead relies on handle close from another goroutine to make the operation return.

Even if that side effect commonly causes an error on specific Windows versions, the current implementation has no contractually bounded cancellation path: if the goroutine does not return promptly, `Accept(ctx)` ignores the canceled context and waits indefinitely. This can stall listener shutdown/restart and violates BL-226's explicit cancellation requirement.

**Required correction:** use a documented bounded cancellation design, preferably overlapped `ConnectNamedPipe` with an explicit completion event/deadline or a correctly thread-bound synchronous design using the documented cancellation primitive. The cancellation path itself must have a server-owned upper bound and native Windows tests.

**Prior Codex signal:** none; this finding was produced by Classic review.

# Findings — BL-233 runtime configuration/discovery

## CR-PR205-001 — P2 — CONFIRMED
### The proposed precedence silently abandons the current public `MCP_*` environment contract

Current FlashGate configuration and README expose stable public variables including `MCP_ROOT`, `MCP_READ_ONLY`, `MCP_ALLOW_CWD_ROOT`, `MCP_MAX_*`, `MCP_ALLOW_HIDDEN_FILES`, `MCP_ALLOW_UNC_PATHS`, `MCP_FOLLOW_SYMLINKS`, and `MCP_DEBUG`.

PR #205 defines client/runtime precedence as `CLI > FLASHGATE_* environment > user config > system config > defaults` and says an implemented source must resolve each setting through that model. It does not define aliases, migration, precedence, or preservation for the existing `MCP_*` names.

An implementation following the normative BL-233 contract can therefore ignore/deprecate currently documented Version-1 configuration without an explicit compatibility decision.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required correction:** specify how existing `MCP_*` settings remain supported, map to the new configuration model, or are deliberately superseded through an explicit compatibility/migration decision. Precedence must be deterministic when old/new aliases coexist.

## CR-PR205-002 — P2 — CONFIRMED
### BL-233's required user-log destination is absent

`BACKLOG.md` explicitly assigns BL-233 Event Log, journald, user logs, and secret-safe diagnostics.

The proposed destination table provides direct STDIO stderr, proxy/auto edge stderr, Windows service Event Log, and Linux service journald. It then explicitly says Version 1.0 accepts no arbitrary client log-file destination.

For a GUI/launcher that does not retain edge-process stderr, there is no bounded durable per-user diagnostic destination at all. The contract therefore does not satisfy its own backlog acceptance scope for user logs.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required correction:** define one bounded, secret-safe, server-owned per-user log destination and lifecycle for the applicable edge/client role, without accepting caller-selected arbitrary paths. The exact location/rotation limits must remain within the configuration/diagnostic security policy.

# Findings — BL-236 backend-neutral execution identity

## CR-PR220-001 — P2 — CONFIRMED
### Registry construction accepts arbitrary backend IDs despite the Version-1 closed backend set

ADR-015 and the execution-identity contract define Version 1.0 backend IDs as `current-process` and `service-account`, with `user-worker` reserved and unsupported.

`NewRegistry` rejects only nil backend, empty ID, `user-worker`, and duplicates. Any custom nonempty ID can therefore be registered and selected if trusted policy returns it, despite the accompanying documentation claiming unknown backend IDs fail closed.

This turns the closed security/backend set into an extension point not owned by BL-236.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required correction:** registry construction must accept only the explicitly supported Version-1 backend IDs; future backend expansion requires its owning reviewed contract.

## CR-PR220-002 — P2 — CONFIRMED
### The immutable execution binding omits the policy-selected resource budget

The accepted execution-identity design defines the server-created execution context with a deadline/resource budget and requires global/per-principal resource limits to be applied before dispatch.

PR #220's `Decision` and `ExecutionContext` carry profile, root, capability, backend, service generation, correlation, and caller, but no resource budget.

The backend/OS adapter therefore cannot receive the immutable policy-selected budget through the backend-neutral boundary; later code would have to re-read policy/configuration or pass an unbound parallel channel, weakening the complete execution binding contract.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required correction:** include the resolved immutable resource budget and its clear ownership in the policy decision/execution binding before backend dispatch.

# Findings — BL-237 Variant-A service-account backend

## CR-PR257-001 — P1 — CONFIRMED
### `service-account` can still be registered as an unsecured passthrough backend

PR #257 adds a validating `ServiceAccountBackend` that enforces non-privileged effective identity, explicit granted roots, dual-identity audit, and OS-permission normalization.

But the inherited `NewPassthroughBackend` still accepts `BackendServiceAccount`, and `NewRegistry` does not require the concrete service-account backend type.

A bootstrap can therefore register a passthrough under the stable `service-account` ID. Policy then selects `service-account`, but dispatch bypasses the Variant-A root-grant, effective-identity and audit checks entirely and calls the OS adapter directly.

That defeats the core BL-237 security boundary while appearing to use the correct backend ID.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required correction:** reserve `service-account` so it can be provided only by the validating Variant-A backend or an equivalently validated implementation; passthrough must be `current-process` only.

## CR-PR257-002 — P2 — CONFIRMED
### Dual-identity audit omits the effective FlashGate profile

ADR-015 defines the execution/audit context to include both identity sides plus the effective profile/root/backend/generation binding.

`AuditEvent` includes caller principal, effective principal, backend, root, capability, operation, correlation, generation and result — but not `Profile`.

Two requests by the same principal to the same root/capability/backend under different profiles can therefore produce audit records that are not sufficient to reconstruct which profile authorized the operation.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required correction:** include the immutable effective profile in the audit event and associated tests/redaction contract.

## CR-PR257-003 — P2 — CONFIRMED
### Caller-controlled correlation is copied into audit without a field-level bound

`Request.Correlation` is caller-influenced and is validated only as nonempty. The dispatcher copies it into the immutable binding, and `ServiceAccountBackend` copies it directly into `AuditEvent`.

The overall JSON-RPC message has a global byte limit, but that still allows a correlation value far larger than an audit-safe identifier. Repeated failures/operations can amplify that caller-controlled string into the audit sink.

PR #257 describes its audit records as bounded. The code does not provide an audit-specific correlation limit or safe normalization.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required correction:** impose a conservative server-owned bound/normalization on the correlation identifier before it enters the audit binding; reject or safely replace overlong/unsafe values without logging the raw input.

# Parent / integration constraints

## BL-225 parent blocks both transport heads

PR #255 and PR #256 are direct children of PR #210 / BL-225. Classic Batch 04 independently found the BL-225 Version-1 IPC contract incomplete in three ways: closed schemas are missing for most envelopes/payloads; cancellation failure semantics have no legal encoding; and post-handshake frame reads have no progress deadline.

Therefore the current transport heads cannot be finalized merely by correcting their local transport findings. The corrected BL-225 contract must be bound first, then both transport children must be rebased/revalidated against it.

## BL-236 parent blocks BL-237

PR #257 is a direct child of PR #220. BL-236's closed backend set and resource-budget binding must be corrected first; then BL-237 must be rebased and its service-account/audit findings corrected on that parent.

# Review summary

```text
ReviewedPRCount=5
ConfirmedPriorCodexSignals=13
RejectedPriorCodexSignals=0
NewClassicFindings=2
P1FindingCount=6
P2FindingCount=9
OpenFindingCount=15
ParentConstraintCount=2
FindingFixesPerformed=false
RepositoryMutationCount=0
ExternalMutationCount=0
ReviewerIndependencePreserved=true
```

Per PR:

```text
PR255 = FAIL_REMEDIATION_REQUIRED   (1x P1, 2x P2; inherits BL225 parent blockers)
PR256 = FAIL_SECURITY_REMEDIATION_REQUIRED (4x P1, 1x P2; inherits BL225 parent blockers)
PR205 = FAIL_CONTRACT_REMEDIATION_REQUIRED (2x P2)
PR220 = FAIL_REMEDIATION_REQUIRED   (2x P2)
PR257 = FAIL_SECURITY_REMEDIATION_REQUIRED (1x P1, 2x P2; inherits PR220)
```

Green historical CI/cross-build evidence does not supersede these findings. Real ACL/token/pipe cancellation behavior requires native Windows evidence; real filesystem publication/ownership and stale-endpoint behavior require native Linux evidence.

## Correction / continuation boundary

This review authorizes **no correction**.

Parent-first order:

1. Repair/review BL-225 before finalizing either transport.
2. Correct PR #255's atomic stale replacement, publication permissions and bounded stale probe.
3. Correct PR #256's no-impersonation identity derivation, least-rights DACL, first-instance namespace ownership, reserved frame flags and bounded cancellation.
4. Repair BL-233 configuration compatibility and required user-log contract before implementing its runtime configuration surface.
5. Correct BL-236's closed backend registry and immutable resource-budget binding.
6. Rebase BL-237 on corrected BL-236, reserve the service-account backend implementation and correct audit profile/correlation bounds.
7. Use Classic read-only focused independent delta review after any authorized correction bundle.

Do not use Codex review as independent-review evidence.

## Post-vacation priority

This batch does not establish the post-vacation work queue.

```text
MobileTaskSelection=OFF
PrimaryWorkSource=BACKLOG.md
```

On return, rebind the then-current normal backlog/governance and apply this review evidence only when its owning normal-backlog work is reached.
