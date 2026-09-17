# FlashGate MCP — Wave 1 / Wave 2 Integration Preflight

**Status:** READ_ONLY_PREPARED_NOT_INTEGRATION_READY  
**Date:** 2026-09-17  
**Repository:** `thomasweidner/flashgate-mcp`  
**Bound main:** `e75e2371c0d1ae7c587286e75189f86fc5d3a4c0`  
**Bound main tree:** `018fbca90efdfc27aaeb2deb44a9d6b479be8b66`  
**Source planning document:** `docs/planning/mobile-integration-unblock-plan-2026-09-17.md`  
**Source planning blob:** `01e91def61e36ff547d9e86fa24333935ec9e4e6`  
**Open PR count:** `198`  
**LedgerPaginationComplete:** `true`  
**LedgerSnapshotStable:** `true`  
**MutationCount:** `0`
**AGENTS.md blob:** `079fad212fc4843eec70999cb551495b226087e6`  
**BACKLOG.md blob:** `a02db7c09b10dba8c827f62618c7c6eb9f096591`  
**Cloud governance blob:** `5a8295d61a0d13a624471fa3438c2e6157d9fb60`  
**Change-trigger adapter blob:** `07ac0d53520b70d71c8c96fd419d55134e890692`  
**Finding/review-mode standard blob:** `89622d7c04af0359b3dcc34e5ee5eeb637ced7f0`  
**Classic-readiness standard blob:** `4bdb44bb160b003969d5ccbc157d2415d9e4ed1e`  

This is a read-only vacation preflight. It grants no Git, remote, review-thread-resolution,
merge, branch, install, permission, or irreversible authority.

## Classic review authority

For this vacation analysis, GitHub/Codex review comments are **review signals only**. They are not governance-authoritative independent-review findings and they do not by themselves authorize correction.

When technical risk requires independent review, the binding review is performed in **Classic** as a separate read-only actor under the current governance:

- `INDEPENDENT_REVIEW`: read-only, no repository or external mutation, no correction;
- an implementing/correcting actor may self-validate but may not claim independent approval;
- only a Classic review disposition may promote a signal to a confirmed finding for remediation tracking;
- after a confirmed finding is corrected by the authorized correction actor, any required focused delta review is again Classic and read-only;
- Codex/Cloud comments may be used as evidence pointers or candidate hypotheses, but never as the required independent-review evidence.

On return to Windows/local work, rebind against the then-current central Slim Governance and applicable higher-level/local `AGENTS.md`; stricter/newer local authority wins.

## Vacation exit rule

Mobile/Codex-Cloud selection is temporary vacation workflow only.

When the user returns from vacation:

```text
MobileTaskSelection = OFF
PrimaryWorkSource    = BACKLOG.md
TaskSelection        = normal backlog / current governance
```

Do not continue the Mobile Wave 1–6 ordering as the global work queue after the vacation.
Rebind the repository and select from the normal authoritative backlog sequence. The current
backlog has earlier Planned work beginning at SPR-048; Mobile-prepared PRs remain useful
prepared inputs but do not override normal backlog priority.

The repository planning document should eventually be amended to record this exit rule under
separate Git/remote authorization.

---

## Preflight summary

```text
MainSha=e75e2371c0d1ae7c587286e75189f86fc5d3a4c0
MainTree=018fbca90efdfc27aaeb2deb44a9d6b479be8b66
LedgerPaginationComplete=true
LedgerSnapshotStable=true
OpenPRCount=198
Wave1PRsCurrent=true
Wave2PRsCurrent=true
AncestorTopologyMatch=true
ChangedHeadCount=0
ClosedOrMergedInputCount=0
PowerShellVersion=NOT_PROBED_ON_PHONE
RequiredPowerShellVersion=7.6.5
ConflictProjectionState=NO_CURRENT_MAIN_PRODUCT_COLLISION_BUT_PREPARED_PR_REVIEW_SIGNALS_AND_SIBLING_RECONCILIATION_EXIST
MutationCount=0
```

## Important discovery

Neither Wave 1 nor Wave 2 is currently ready for Windows integration.

Observed GitHub/Codex review signals must first be independently dispositioned in Classic; only Classic-confirmed findings enter remediation:

- PR #105 / BL-100: P1 review signal (scope), pending Classic disposition.
- PR #188 / BL-114: P2 review signal (cursor validation), pending Classic disposition.
- PR #193 / BL-117: P2 review signal (process-tree partial state), pending Classic disposition.
- PR #129 / BL-162: P1 and P2 policy-contract review signals, pending Classic disposition; BL-162 is not a hard runtime parent of BL-118 in the Mobile snapshot, but it is normative policy work and belongs to the earlier normal-backlog sprint SPR-054.

PR #181, #191 and #192 currently have no open inline review threads.

---

# Wave 1 — capability / execution authorization

## Topology

### PR #105 — BL-100

```text
State=open
Base=main @ 5b851afb3ef3e7de1b49ce5a352c06d40b815f9d
Head=3ca7584e00690b8f09bccb8401e555bc9cf3985f
```

### PR #181 — BL-159

```text
State=open
Base=PR #105 head @ 3ca7584e00690b8f09bccb8401e555bc9cf3985f
Head=9a1aef868c2313efece7c5f8d2382d5f89b58990
```

Git-object comparison confirms #181 is exactly one commit ahead of #105.

## Current-main drift

From the original #105 base to current main, repository changes are confined to:

- `AGENTS.md`
- `Governance/MOBILE-CLOUD-HANDOFF.md`
- `MOBILE.md`
- `docs/planning/mobile-integration-unblock-plan-2026-09-17.md`

There is currently no product-code overlap with the Wave-1 implementation files.

## Candidate risk signal: PR #105 P1 — pending Classic independent review

Codex signal: BL-100 owns the functional capability model, but the PR also makes capability presence control the registered `tools/list` catalog. The Codex signal attributes general dynamic capability-driven registration and its negative catalog behavior to BL-110/BL-111.

### Minimal correction candidate

If Classic confirms this finding, the correction should preserve BL-100's functional capability vocabulary while restoring the pre-existing registration ownership boundary:

1. Keep `internal/capability` and the immutable/validated capability set.
2. Keep the compatibility conversion from `MCP_READ_ONLY` into the capability vocabulary where it is only an internal representation of the already-existing read-only behavior.
3. In `createToolRegistry`, continue to register the existing read tools unconditionally, as the pre-BL-100 implementation did.
4. Keep the existing write-tool suppression semantics equivalent to `MCP_READ_ONLY`, without generalizing registration to arbitrary future capability absence.
5. Remove/adjust tests such as "no tools without capabilities" that assert the future dynamic registration behavior.
6. Adjust ADR/docs so BL-100 claims the vocabulary/model only; dynamic profiles and general capability-driven catalog construction remain owned by BL-110/BL-111.
7. Re-run focused server/capability tests, full Go tests, race tests for affected packages, vet/build, documentation consistency and Windows PowerShell 7.6.5 gates.
8. Request a fresh review before any integration.

### Effect on PR #181

PR #181 consumes the BL-100 capability-set types and is structurally a valid stack child. After #105 is corrected, #181 must be rebound/rebased to the corrected #105 head and then revalidated. Its current head must not be merged on top of the superseded #105 content.

## Wave-1 state

```text
Wave1State=PENDING_CLASSIC_REVIEW_PARENT_P1_SIGNAL
NextSafeBoundary=CLASSIC_INDEPENDENT_REVIEW_PR_105_THEN_DECIDE_CORRECTION
```

---

# Wave 2 — process observation

## Topology

```text
#188 BL-114  fe78d918bcfcef9c4108fac0e0fca961212044c5
  ├─ #191 BL-115  8f945ec18d5c5b717eb55bf8e43ef9fb0fedb007
  └─ #192 BL-116  1ae5a4882cb2fa85c65bfc5a0a780575dfaa4390
       └─ #193 BL-117  e0043ac51190f0a239ba9412546fe088226d7d30
```

Git-object comparisons confirm each displayed direct parent relationship exactly.

## Current-main drift

From the original #188 base to current main, repository changes are confined to the same Mobile-governance/planning files listed above. There is currently no product-code overlap with the #188 process implementation.

## Candidate risk signal: PR #188 P2 — pending Classic independent review

Codex signal: `decodeProcessCursor` accepts a valid first JSON object followed by malformed trailing bytes because it rejects only when the second decode succeeds; a syntax error is accidentally accepted.

### Minimal correction candidate

1. Require the second JSON decode to return exactly `io.EOF`.
2. Treat `nil`, syntax errors, or any non-EOF result as invalid cursor data.
3. Add a regression test that constructs canonical base64url payload containing a valid cursor JSON object followed by malformed trailing bytes.
4. Assert rejection occurs before `Lister.List` / native observation.
5. Retain existing canonical base64url, size, version, unknown-field and `LastPID` checks.
6. Re-run tool/process tests, focused race tests, full tests, vet/build and Windows finalization.

## Candidate risk signal: PR #193 P2 — pending Classic independent review

Codex signal: process-tree construction silently discards entries whose PID is valid but name is empty, losing parent/child relationships while incorrectly reporting `partial=false`.

### Minimal correction candidate

1. Preserve topology for every non-zero PID even when display metadata is incomplete.
2. Build child relationships independently from whether the entry is safe/complete enough to emit.
3. Mark the result `partial=true` when an encountered topology node has missing required display metadata.
4. Do not emit a node with an invalid/empty required name.
5. Continue traversal through omitted/incomplete nodes so named descendants are not silently lost; preserve their real relative depth.
6. Determine root existence from the topology snapshot, not solely from the subset of displayable entries.
7. Add tests for an unnamed intermediate child with a named grandchild, root/descendant completeness signalling, cycles, depth/truncation and node bounds.
8. Re-run process/tool unit + race tests and native Windows/Linux observation validation.

## Sibling convergence conflict

#191 and #192 are true siblings from #188. Their direct deltas overlap in exactly these main hotspots:

- `CHANGELOG.md`
- `docs/security.md`
- `docs/tools.md`
- `internal/mcp/tools/output_schemas.go`

The current stored plan proposes `#188 -> #191 -> #192 -> #193`. A lower-rework mechanical sequence after any Classic-confirmed findings are corrected is likely:

```text
#188
  -> #192
  -> #193
  -> converge #191 last
```

Reason: #193 is already a real child of #192. Preserving that chain first leaves only one sibling rebase/convergence (#191), instead of rebasing #192 and then rebasing #193 again after #191. This is a conflict-minimization proposal, not merge authority; it must be re-evaluated from fresh Git state on Windows.

## BL-162 / PR #129 policy contract

PR #129 is currently open and has two unresolved Codex review signals pending Classic disposition:

- P1: it prematurely makes deferred risk-class names normative.
- P2: it requires session ownership while omitting session/connection identity from the managed handle ownership record.

This PR is not treated as a hard runtime predecessor of BL-118 in the Mobile snapshot, but the contract must not be contradicted. Under the normal post-vacation backlog, BL-162 belongs to SPR-054, which precedes the process-observation sprint SPR-055. Therefore the normal backlog flow will naturally require this policy work to be reconciled before later process work is considered complete.

## Wave-2 state

```text
Wave2State=PENDING_CLASSIC_REVIEW_OF_CODEX_SIGNALS
ReviewSignals=#188_P2,#193_P2; ClassicReviewStatus=NOT_RUN
SiblingConvergenceRequired=true
PolicyContractSignals=#129_P1,#129_P2; ClassicReviewStatus=NOT_RUN
NextSafeBoundary=CLASSIC_INDEPENDENT_REVIEW_THEN_SEPARATE_CORRECTION_AUTHORIZATION_IF_NEEDED
```

---

# Integration conflict projection

## Current main versus prepared roots

At the current bound main, both Wave-1 and Wave-2 root branches diverge only because main gained Mobile governance/planning commits. No root implementation file has been changed on main since their preparation bases.

Therefore:

```text
CurrentMainProductConflictRisk=LOW
DocumentationConflictRisk=LOW_BEFORE_FIRST_INTEGRATION
```

## After Wave 1 is integrated

Wave 1 and Wave 2 both touch `docs/architecture.md` and `docs/security.md`. If Wave 1 is integrated first, #188 will need documentation reconciliation when brought onto the new main, but its process/product implementation should remain largely independent.

This still favors Wave 1 before Wave 2 **inside the vacation-unblock preparation context**.

After the vacation, however, the normal backlog order—not this Mobile-specific optimization—is authoritative.

---

# Windows/local gates still required

No local Windows host was available during this phone preflight.

```text
PowerShellVersion=NOT_PROBED
Required=7.6.5
```

Before actual integration:

- fresh repository/HEAD/working-tree binding;
- fresh current governance and BACKLOG;
- inspect whether any relevant PR was updated, closed, merged or superseded;
- perform Classic independent review on the exact heads and disposition all recorded Codex review signals;
- PowerShell 7.6.5;
- scope-triggered Go tests, race tests, vet/build;
- documentation consistency;
- native Windows Toolhelp/process cases for process observation;
- native Linux validation from a copy under `/home`, never `/mnt/c`;
- no claim of completion from hosted Windows CI alone.

---

# Next useful phone work

Recommended read-only sequence while still on vacation:

1. Perform Classic `INDEPENDENT_REVIEW` for PR #105 and disposition its Codex review signal.
2. Perform Classic `INDEPENDENT_REVIEW` for PR #188 and disposition its Codex review signal.
3. Perform Classic `INDEPENDENT_REVIEW` for PR #193 and disposition its Codex review signal.
4. Perform Classic independent review of PR #129 against the current policy/ADR authority.
5. Only after Classic confirms findings, prepare a separate authorized correction assignment; never correct inside the review.
6. Then preflight Wave 3 and Wave 4, followed by Wave 5/6.

No Git mutation is authorized by this document.
