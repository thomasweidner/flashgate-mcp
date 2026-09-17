# FlashGate MCP — Classic Independent Review Batch 04

**Status:** `COMPLETE_FINDINGS_CONFIRMED_NO_FIXES_PERFORMED`  
**Date:** 2026-09-17  
**Review mode:** `INDEPENDENT_REVIEW`  
**Repository:** `thomasweidner/flashgate-mcp`  
**Bound main:** `98556d359ab77e4eb0c84af721ec0d0b379cad2d`  
**Bound main tree:** `9779488d3e91b40739ea7eb230691c0dca03ae4e`  
**RepositoryMutationAllowed:** `false`  
**ExternalMutationAllowed:** `false`  
**FindingFixesPerformed:** `false`  
**ReviewerIndependencePreserved:** `true`

This review was performed in Classic as a separate read-only review activity. GitHub/Codex review comments were treated only as evidence pointers and hypotheses. Every disposition below was independently checked against the reviewed code/contracts, current backlog ownership, ADR-017, the current CLI behavior, and the current stateful-object security contract.

No reviewed PR, branch, review thread, repository file, remote state, workflow, issue, or external system was modified during this review.

## Authority bound for this review

- `AGENTS.md` blob: `079fad212fc4843eec70999cb551495b226087e6`
- `BACKLOG.md` blob: `a02db7c09b10dba8c827f62618c7c6eb9f096591`
- `Governance/CLOUD-CODEX-GOVERNANCE.md`: `5a8295d61a0d13a624471fa3438c2e6157d9fb60`
- `Governance/CHANGE-TRIGGER-REVIEW-AND-BACKLOG-STANDARD.md`: `07ac0d53520b70d71c8c96fd419d55134e890692`
- `Governance/FINDING-REMEDIATION-AND-REVIEW-MODE-STANDARD.md`: `89622d7c04af0359b3dcc34e5ee5eeb637ced7f0`
- `Governance/HANDOFF-ARTIFACT-AND-CLASSIC-READINESS-STANDARD.md`: `4bdb44bb160b003969d5ccbc157d2415d9e4ed1e`
- ADR-017 host ownership/lifecycle: `248fac8d422fc7fe43367ea48586ecb558bf2b4c`
- current security contract: `ae0fb75511aff853dd7a779d1c41339918bc2a22`

Relevant backlog ownership:

- BL-223 owns CLI mode, role/lifetime, shutdown reason, and exit contract.
- BL-224 owns the transport-neutral process-root lifecycle coordinator and adapter/domain boundary.
- BL-225 owns connection/session identity, disconnect behavior, IPC framing/version/cancellation/limits/compatibility, and optional negotiated FlashGate leases.
- BL-094 owns Operations/Job controlled shutdown.
- BL-129 owns managed-child cleanup/restart/orphan behavior.
- BL-341 later owns technical top-level host ownership/shutdown/orphan prevention.
- BL-241 later owns integrated native lifecycle/leak validation.

ADR-017 requires one process-root cancellation context, bounded cleanup, principal/connection binding, and explicit separation between host ownership, Operations/Job ownership, and Managed Process child ownership.

## Reviewed PR bindings

| PR | BL | Head | Tree | Base/parent |
|---:|---|---|---|---|
| #195 | BL-223 | `3d687729af9e15c29c38ac9c5250a3d924fc228f` | `c318d8e3c7926d5d95fd43802b9ac37f4a160730` | `8de5e5bb9f03a96b0202b1df421d3f6812a716f2` |
| #214 | BL-224 | `6179bf1ad97147d314a16f4f523769ccae45326d` | `24821df076f95bebe727b0c2f4e87c6cd0fbafa3` | `c8a08c57042fd4e5604eb46a6ebe19e8cdd918ba` |
| #210 | BL-225 | `2964450a943907beb0cde0847dfe89a245d264c6` | `c4d13c87918c85d0dc5805d9fff850735b6ab877` | `c8a08c57042fd4e5604eb46a6ebe19e8cdd918ba` |
| #81 | BL-094 | `b6e95a4417874708a68c3e69f366e26030bcee0b` | `563a165277ebc62e2ad61404354410d10161aca2` | `5b851afb3ef3e7de1b49ce5a352c06d40b815f9d` |
| #134 | BL-129 | `ee0016c32c5c8b31545ab6f6398db13ca4ea21e0` | `eace5cd93bb31a0f965bc7c4dcf12ba00b05ff6d` | `5b851afb3ef3e7de1b49ce5a352c06d40b815f9d` |

# Findings

## CR-PR195-001 — P2 — CONFIRMED
### The normative BL-223 grammar drops the already-supported `-h` help form

Current `cmd/server/cli.go` accepts both `--help` and `-h`. PR #195's new normative grammar lists only `--help` and states that options/forms not listed in the grammar are usage errors.

If implemented literally, the already-supported and tested `-h` alias becomes a breaking usage error even though BL-223's acceptance scope says to preserve existing no-argument STDIO compatibility and does not authorize removal of established informational CLI behavior.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required correction:** include `-h` in the normative informational grammar and parser/validation matrix, or make an explicit separately reviewed compatibility-breaking decision. No correction was performed.

## CR-PR214-001 — P2 — CONFIRMED
### Parent/root cancellation can leave `Server.Run` blocked before lifecycle cleanup starts

The bootstrap creates a coordinator, runs the MCP server with the coordinator context, and only after `Run` returns invokes `processRoot.Shutdown(context.Background())`.

However, `Server.Run` can remain blocked inside the transport `ReadMessage` while STDIN/pipe remains open. Canceling the parent/process-root context does not itself close or unblock that read.

A retained pipe handle or another still-open writer can therefore prevent `Run` from returning, which prevents the coordinator's `Shutdown` call and any future registered domain-owner cleanup from running.

This directly violates BL-224/ADR-017's model in which root cancellation is the integration signal and adapters report lifecycle signals while one process-root coordinator drives cleanup.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required correction:** root cancellation/lifecycle shutdown must be able to stop protocol intake and unblock/close the transport wait path before waiting for `Run` to naturally return. Exact transport-close ownership must remain compatible with later BL-341 host-lifecycle implementation.

## CR-PR210-001 — P1 — CONFIRMED
### Version-1 strict decoding lacks closed schemas for most wire payloads

The protocol says handshake, cancel, close, and heartbeat JSON use strict decoding and says unknown keys/fields are rejected. But the document does not define one implementable closed schema for cancel, close, heartbeat, request, response, event, the hello `limits` object, nested normalized error/details objects, lease timing fields, or operation/result payload envelopes.

For many fields it states semantic requirements but not exact key names, requiredness, nullability, bounds, array limits, nested structure, or conditional shape.

Independent proxy and service implementations can therefore choose incompatible JSON forms while both claiming Version-1 compliance, and conformance tests have no single byte-level target.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required correction:** publish closed, bounded schemas/tables for every Version-1 envelope and nested object, including exact conditional-field rules.

## CR-PR210-002 — P1 — CONFIRMED
### The contract requires a cancellation failure result that the frame model cannot represent

For an unknown/completed/stale/foreign request the cancellation section requires a generic `unavailable` result.

But there is no cancel-ack/cancel-result frame kind; a normal `response` is restricted to one currently in-flight request, so those cases cannot use that response ID without violating the correlation rule; `events.v1` is optional and cannot be required for cancellation; and the closed error taxonomy does not contain a generic `unavailable` category.

The protocol therefore mandates an outcome with no defined legal encoding.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required correction:** define an explicit cancellation acknowledgement/result envelope and stable error mapping, or normatively define the relevant cancellation cases as silent/idempotent. Keep ownership non-disclosure intact.

## CR-PR210-003 — P2 — CONFIRMED
### Post-handshake frame reads have byte bounds but no time/progress bound

The framing algorithm checks `payload_length`, allocates bounded storage, and reads exactly that payload. The explicit connection byte/time preconditions are pre-handshake.

After a successful handshake, a peer can send a valid maximum-size header and then stop or drip the payload indefinitely. Each such connection can retain its bounded allocation and connection slot without an explicit per-frame receive/progress deadline.

This contradicts the stated invariant that connection, buffer, deadline, and resource usage are bounded before allocation/dispatch.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required correction:** define a server-policy-bounded post-handshake frame receive/progress deadline or equivalent bounded mechanism for every frame.

## CR-PR81-001 — P1 — CONFIRMED
### `Cancel` and `Force` callbacks execute outside the configured shutdown phase bounds

The shutdown contract says `Cancel` and `Force` must return promptly, but the coordinator invokes all `Cancel` callbacks synchronously before starting the grace wait timer and all `Force` callbacks synchronously before starting the force wait timer.

A single blocked callback can therefore prevent finalization forever. Even multiple merely slow callbacks can make total shutdown exceed the configured grace/force periods.

The caller-visible timeout configuration does not actually bound the complete shutdown phases.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required correction:** callback invocation itself must be inside/subject to the server-owned phase/hard deadline, with a design that still preserves deterministic participant accounting and does not leak unbounded goroutines/resources.

## CR-PR81-002 — P2 — CONFIRMED
### Timer expiry can race a just-completed participant and cause false escalation/incomplete state

`waitForParticipants` scans each `Done` channel nonblocking, then waits on either the phase timer or a short sleep. If a participant completes after the last scan but before the timer case wins, the function returns with the cached `Terminated=false`.

Consequences: after grace, `Force` can be invoked for work that already completed; after force, the immutable final report can incorrectly publish `Terminated=false` / `Complete=false`.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required correction:** resample completion at the deadline boundary and make escalation/final classification refuse or account for already-completed work deterministically.

## CR-PR81-003 — P2 — CONFIRMED
### An already-completed shutdown can nondeterministically return a caller context error

`Run` always selects between `<-done` and `<-ctx.Done()>`. When shutdown has already completed and a later caller supplies an already-canceled/expired context, both cases are ready. Go may choose either, so repeated calls can alternate between the immutable cached report and a context error.

That contradicts the method contract that concurrent/later callers receive the same final report.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required correction:** check/publish the completed cached report before entering the context-sensitive wait path.

## CR-PR134-001 — P1 — CONFIRMED
### The contract requires final OS ownership identity to exist before process creation

The new lifecycle contract says that before a child is started the component creates an immutable ownership record already bound to PID plus process-start identity or verified OS handle and launcher-established child-tree ownership.

Those values do not exist until the OS has successfully created the process. An implementation must therefore either mutate a record declared immutable or leave the newly created process temporarily outside the supposedly completed ownership binding.

That undermines the very PID-reuse/cleanup safety the contract is intended to establish.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required correction:** define a pre-launch immutable reservation/policy binding followed by atomic/write-once OS-identity finalization immediately after creation and before exposing the public handle or permitting lifecycle actions.

## CR-PR134-002 — P2 — CONFIRMED
### The ownership record does not bind the resolved handle/result expiry

The contract later says TTL governs retained handles/output/result metadata and repeatedly refers to the object's TTL/expiry, but the immutable ownership record does not contain the resolved expiry.

Current `docs/security.md` explicitly requires all stateful objects, including managed process handles, to be bound to `principal + profile + root + execution backend + service generation + expiry`.

Without an immutable resolved expiry, a later profile/configuration change can cause retention or cleanup to be recomputed from a different policy than the one that authorized creation.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required correction:** bind the resolved expiry/TTL policy into the immutable ownership reservation/record.

## CR-PR134-003 — P2 — NEW CLASSIC FINDING
### Session-owned managed children have no immutable owning connection/session identity

The BL-129 contract explicitly includes cleanup on owning connection/session loss when policy makes the child session-owned and requires handle/lifecycle access to use the bound execution context.

But its ownership-record field list contains principal/profile/root/backend, generations, command decision, OS process identity, and child-tree ownership — not the owning connection/session ID.

For two simultaneous connections with the same principal/profile/root/backend/generation, the record therefore cannot distinguish which connection owns a session-scoped child. On disconnect, the Managed Process owner cannot safely select only the children owned by that session from the declared record.

ADR-017 independently requires the shared service to bind service resources to connection/principal and release connection-owned work on disconnect, and its security impact calls out principal/connection binding.

This is a lifecycle-contract defect in BL-129 itself, independent of the earlier BL-162 policy finding.

**Required correction:** when policy makes a managed child session/connection-owned, bind an immutable authoritative session/connection identity into the ownership reservation/record. If a managed child is instead principal-scoped or durable, that scope must be explicit and its disconnect semantics must not claim session ownership.

**Prior Codex signal:** none; this finding was produced by Classic review.

# Review summary

```text
ReviewedPRCount=5
ConfirmedPriorCodexSignals=10
RejectedPriorCodexSignals=0
NewClassicFindings=1
P1FindingCount=4
P2FindingCount=7
OpenFindingCount=11
FindingFixesPerformed=false
RepositoryMutationCount=0
ExternalMutationCount=0
ReviewerIndependencePreserved=true
```

Per PR:

```text
PR195 = FAIL_REMEDIATION_REQUIRED   (1x P2)
PR214 = FAIL_REMEDIATION_REQUIRED   (1x P2)
PR210 = FAIL_CONTRACT_REMEDIATION_REQUIRED (2x P1, 1x P2)
PR81  = FAIL_REMEDIATION_REQUIRED   (1x P1, 2x P2)
PR134 = FAIL_CONTRACT_REMEDIATION_REQUIRED (1x P1, 2x P2)
```

Green historical unit/CI evidence does not supersede these contract/security findings. Native Windows/Linux lifecycle evidence remains separate.

## Correction / continuation boundary

This review authorizes no correction.

Parent/contract-first correction order:

1. Correct BL-223 grammar compatibility (`-h`) before treating the CLI/lifecycle contract as normative.
2. Correct BL-224 so process-root cancellation can stop intake and actually initiate coordinator cleanup without depending on the read loop returning.
3. Repair BL-225 as a complete Version-1 wire contract: closed schemas, legal cancellation-result semantics, and post-handshake frame progress deadlines.
4. Correct BL-094 shutdown mechanics before BL-341 can safely depend on Operations/Job cleanup.
5. Correct BL-129 ownership creation/finalization, immutable expiry, and session/connection binding before Managed Process cleanup is consumed by BL-341.
6. Only after these corrected contracts/owners are integrated and independently delta-reviewed may BL-341 feasibility be reconsidered.

After any authorized correction bundle, use the governance-required Classic read-only focused independent delta review. Do not use Codex review as independent-review evidence.

## Post-vacation priority

This batch does not establish the post-vacation work queue.

```text
MobileTaskSelection=OFF
PrimaryWorkSource=BACKLOG.md
```

On return, rebind the current normal backlog/governance and apply this review evidence only when its owning normal-backlog work is reached.
