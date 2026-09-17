# FlashGate MCP — Classic Independent Review Batch 01

**Status:** `COMPLETE_FINDINGS_CONFIRMED_NO_FIXES_PERFORMED`  
**Date:** 2026-09-17  
**Review mode:** `INDEPENDENT_REVIEW`  
**Repository:** `thomasweidner/flashgate-mcp`  
**Bound main:** `2770f0b1c0415bd5fcfef718a48e58ec29c2f7fc`  
**Bound main tree:** `adf7b2e248daabd42fff43831d7b94d9bb1aa0e2`  
**RepositoryMutationAllowed:** `false`  
**ExternalMutationAllowed:** `false`  
**FindingFixesPerformed:** `false`  
**ReviewerIndependencePreserved:** `true`

This review was performed in Classic as a separate read-only review activity. GitHub/Codex review comments were treated only as evidence pointers and hypotheses; every disposition below was reached from the reviewed PR diff/code plus current repository authority.

No reviewed PR, branch, review thread, repository file, remote state, workflow, issue, or external system was modified during this review.

## Authority bound for this review

- `AGENTS.md` blob: `079fad212fc4843eec70999cb551495b226087e6`
- `BACKLOG.md` blob: `a02db7c09b10dba8c827f62618c7c6eb9f096591`
- `Governance/CLOUD-CODEX-GOVERNANCE.md`: `5a8295d61a0d13a624471fa3438c2e6157d9fb60`
- `Governance/CHANGE-TRIGGER-REVIEW-AND-BACKLOG-STANDARD.md`: `07ac0d53520b70d71c8c96fd419d55134e890692`
- `Governance/FINDING-REMEDIATION-AND-REVIEW-MODE-STANDARD.md`: `89622d7c04af0359b3dcc34e5ee5eeb637ced7f0`
- `Governance/HANDOFF-ARTIFACT-AND-CLASSIC-READINESS-STANDARD.md`: `4bdb44bb160b003969d5ccbc157d2415d9e4ed1e`
- ADR-009 capability/profile authority blob: `82607310523d4c44c3a5530acefc617819b18cf5`
- ADR-011 managed-process authority blob: `3e64a0c4cfbb0223110cf57a107002d49b63e5f1`
- ADR-017 host ownership/lifecycle blob: `248fac8d422fc7fe43367ea48586ecb558bf2b4c`

## Reviewed PR bindings

| PR | BL | Head | Tree | Parent/base relevant to review |
|---:|---|---|---|---|
| #105 | BL-100 | `3ca7584e00690b8f09bccb8401e555bc9cf3985f` | `2ba117fd882b3008cab0e6644a96a29355354db9` | `5b851afb3ef3e7de1b49ce5a352c06d40b815f9d` |
| #188 | BL-114 | `fe78d918bcfcef9c4108fac0e0fca961212044c5` | `73c9314d1b529342e10703a4593196944c5f1f89` | `8de5e5bb9f03a96b0202b1df421d3f6812a716f2` |
| #193 | BL-117 | `e0043ac51190f0a239ba9412546fe088226d7d30` | `0a77eacd83f1ce3e400efa691ba7db76e368fc8d` | `1ae5a4882cb2fa85c65bfc5a0a780575dfaa4390` / PR #192 |
| #129 | BL-162 | `a835bcf2e81658461341ddb43a73ca9bade087bd` | `2103fc1bcd6ebe7d85192bcc851ba3f63d9f65b9` | `5b851afb3ef3e7de1b49ce5a352c06d40b815f9d` |

# Findings

## CR-PR105-001 — P1 — CONFIRMED
### BL-100 consumes BL-110/BL-111 dynamic-registration acceptance scope

`BL-100` owns the functional capability model: functional rights are kept separate from profiles and risk classifications. `BL-110` separately owns dynamic tool registration and `BL-111` owns negative capability/catalog tests.

PR #105 does more than introduce the vocabulary/set. In `cmd/server/tools_boot.go`, `createToolRegistry` now suppresses read tools unless `filesystem.read` is in the capability set, and `TestCreateToolRegistryOmitsToolsWithoutCapabilities` makes that generalized catalog behavior a tested contract. The PR's ADR/testing text also says effective capabilities drive catalog registration.

That is the architecture intended for the later capability/profile stack, but it is not task-pure for BL-100. Merging this head as BL-100 would consume separately owned BL-110/BL-111 acceptance scope and make backlog completion/ownership ambiguous.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required boundary:** separate authorized correction; no fix performed in this review.

**Correction intent to validate later:** keep the closed functional vocabulary and validated set, but preserve only the already-existing `MCP_READ_ONLY` compatibility behavior until the owning dynamic-registration task is integrated. Then rebase/revalidate PR #181 on the corrected parent.

## CR-PR188-001 — P2 — CONFIRMED
### Malformed bytes after a valid cursor JSON object are accepted

`decodeProcessCursor` performs one strict JSON decode and then:

```go
if decoder.Decode(&struct{}{}) == nil {
    return 0, errors.New("invalid cursor")
}
```

This rejects a second valid JSON value, but it accepts any non-`nil` second-decode result. A payload containing a valid first cursor object followed by malformed JSON therefore returns a syntax error from the second decode and is incorrectly accepted.

Because cursor validation occurs before `Lister.List`, this is exactly the boundary where malformed input should fail closed before native process observation.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required correction:** require the second decode to return exactly `io.EOF`, with a regression case proving malformed trailing data is rejected before the lister is called.

## CR-PR188-002 — P2 — NEW CLASSIC FINDING
### Page-size bounds do not bound native work or memory

The public `pageSize` is bounded to 200, but the implementation obtains the complete native process snapshot before paging it:

- Linux uses `os.ReadDir("/proc")`, materializing the complete directory and then builds an entry slice for all observed processes.
- Windows walks the complete Toolhelp snapshot and appends all process entries.
- `LocalLister.List` sorts the complete result before `Execute` slices one page.

A request for one item therefore still performs and allocates work proportional to the entire host process table. This conflicts with FlashGate's resource-control posture and with the PR's claim of bounded process observation: the **response** is bounded, but the operation itself is not server-budget bounded.

This matters on hosts with very large process tables and permits repeated observation calls to amplify CPU/allocation work.

**Required boundary:** correction design must define a deterministic server-side observation budget (entry/time/allocation or equivalent fail-closed bound) without weakening deterministic PID ordering. No fix performed.

**Prior Codex signal:** none; this finding was produced by Classic review.

## CR-PR188-003 — P2 — NEW CLASSIC FINDING
### Linux silently treats unexpected `/proc/<pid>/comm` errors as successful omission

The Linux adapter has:

```go
name, err := os.ReadFile(...)
if err != nil {
    if errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrPermission) {
        continue
    }
    continue
}
```

The second unconditional `continue` suppresses every other error as though the process were merely gone/inaccessible. I/O errors and other unexpected failures therefore produce an apparently successful process list with silently omitted entries and no partial/error indication.

Expected process-exit and access-denial races may be safely omitted, but unexpected platform errors must not be silently converted into a complete-looking successful observation.

**Required correction:** distinguish expected disappearance/access denial from unexpected failures; either fail safely with the generic observation error or introduce an explicitly reviewed partial contract. No fix performed.

**Prior Codex signal:** none; this finding was produced by Classic review.

## CR-PR193-001 — P2 — CONFIRMED
### Incomplete entries can erase descendant topology while `partial=false`

`buildProcessTree` discards entries when `entry.Name == ""` before adding their parent relation:

```go
if entry.PID == 0 || entry.Name == "" {
    continue
}
```

An unnamed but otherwise valid process therefore disappears from both `byPID` and `children`. Named descendants beneath that process are never queued, and the result can still report `partial=false`.

The documented meaning of `partial` is violated: the result appears complete while an entire subtree may be absent.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required correction:** preserve nonzero-PID topology independently from displayable metadata; mark partial when required display metadata is missing and continue traversal through omitted intermediate nodes without emitting invalid nodes.

## CR-PR193-002 — P2 — NEW CLASSIC FINDING
### The 200-node/depth limits do not bound process-tree observation work

`maximumProcessTreeNodes=200` and `maxDepth<=8` bound only the emitted tree. Before those limits are applied:

1. `processTree` enumerates the complete process table;
2. Linux calls `processDetails` for every process;
3. the full `[]Details` snapshot is retained;
4. `buildProcessTree` constructs `byPID` and `children` maps for the full snapshot.

Thus even a shallow one-root request performs host-wide work proportional to the entire process table. The output bound does not protect observation CPU, syscall count, or memory.

**Required boundary:** define and enforce a host-observation budget appropriate for tree discovery, including a safe outcome when a complete parent map cannot be built within the budget. No fix performed.

**Prior Codex signal:** none; this finding was produced by Classic review.

## CR-PR129-001 — P1 — CONFIRMED
### PR #129 makes deferred global risk-class labels normative

The accepted ADR-009 separates functional capabilities, profiles, and risk classifications and explicitly defers final names for those classifications to the owning capability/profile planning work.

PR #129's normative Version-1 process policy table introduces the concrete labels:

- `Sensitive read`
- `Stateful execution`
- `High risk, post-Version 1.0`

BL-162 owns the process policy model and its risk-classification relationship, but this branch is prepared independently of the still-unintegrated capability/risk-policy owner. Making those global labels normative now creates a second security contract that can conflict with the later owning classification.

The process policy can state required security properties and relative risk without finalizing the global taxonomy first.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required boundary:** either integrate the owning classification decision first or make the labels descriptive/non-normative until that owner is resolved. No fix performed.

## CR-PR129-002 — P2 — CONFIRMED
### Session ownership is required by policy but absent from the managed-process ownership record

The PR says policy evaluation uses:

> authenticated caller principal and connection/session ownership

and says lifecycle operations repeat all applicable ownership checks. But the mandatory managed process registry binding lists principal, profile/capability, root, execution backend/generation, process identity, command identity, timing/lifecycle and output/cleanup state — **not the owning connection/session identity**.

Two connections with the same principal/profile/root/backend/generation can therefore be indistinguishable from the record even though the policy later requires session ownership to be checked.

Current ADR-017 reinforces the same security boundary: connection-owned work is bound to connection/principal and released on disconnect.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required correction:** bind immutable connection/session identity whenever a managed handle is session-scoped, or explicitly define the handle as principal-scoped and remove contradictory session-ownership requirements. No fix performed.

# Review summary

```text
ReviewedPRCount=4
ConfirmedPriorCodexSignals=5
RejectedPriorCodexSignals=0
NewClassicFindings=3
P1FindingCount=2
P2FindingCount=6
OpenFindingCount=8
FindingFixesPerformed=false
RepositoryMutationCount=0
ExternalMutationCount=0
ReviewerIndependencePreserved=true
```

Per PR:

```text
PR105 = FAIL_REMEDIATION_REQUIRED   (1x P1)
PR188 = FAIL_REMEDIATION_REQUIRED   (3x P2)
PR193 = FAIL_REMEDIATION_REQUIRED   (2x P2)
PR129 = FAIL_REMEDIATION_REQUIRED   (1x P1, 1x P2)
```

Green historical CI on a reviewed head does not supersede these review findings. Native evidence requirements also remain separate from code/contract review.

## Correction / continuation boundary

This review authorizes **no correction**.

The next permissible step for any confirmed finding is a separate, explicitly authorized non-independent correction assignment under the remediation budget. Parent-first rules apply:

1. BL-100 / PR #105 must be corrected before rebasing/finalizing PR #181 or any child that consumes the capability foundation.
2. BL-114 / PR #188 must be corrected before converging #191/#192/#193.
3. PR #193 receives its own focused correction only after the corrected ancestor stack is bound.
4. BL-162 / PR #129 is a policy-contract correction and must remain consistent with the owning capability/risk decision and current lifecycle/session authority.

After correction, use the governance-required **Classic read-only focused independent delta review** where applicable. Do not use Codex review as the independent-review evidence.

## Post-vacation priority

This batch does not establish the post-vacation work queue. When vacation ends:

```text
MobileTaskSelection=OFF
PrimaryWorkSource=BACKLOG.md
```

Rebind the then-current normal backlog/governance and use these findings only when their owning normal-backlog work is reached.
