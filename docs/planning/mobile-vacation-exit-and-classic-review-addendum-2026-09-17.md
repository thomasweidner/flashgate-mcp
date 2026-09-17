# FlashGate MCP — Mobile Vacation Exit and Classic Review Addendum

**Status:** READ_ONLY_PLANNING_ADDENDUM  
**Date:** 2026-09-17  
**Repository:** `thomasweidner/flashgate-mcp`  
**Bound main:** `e75e2371c0d1ae7c587286e75189f86fc5d3a4c0`  
**Bound main tree:** `018fbca90efdfc27aaeb2deb44a9d6b479be8b66`  
**AGENTS.md blob:** `079fad212fc4843eec70999cb551495b226087e6`  
**BACKLOG.md blob:** `a02db7c09b10dba8c827f62618c7c6eb9f096591`  
**Cloud governance blob:** `5a8295d61a0d13a624471fa3438c2e6157d9fb60`  
**Change-trigger adapter blob:** `07ac0d53520b70d71c8c96fd419d55134e890692`  
**Finding/review-mode standard blob:** `89622d7c04af0359b3dcc34e5ee5eeb637ced7f0`  
**Classic-readiness standard blob:** `4bdb44bb160b003969d5ccbc157d2415d9e4ed1e`  
**Open PR ledger:** `198`  
**LedgerPaginationComplete:** `true`  
**LedgerSnapshotStable:** `true`

This addendum records two temporary-workflow boundaries. It is planning evidence, not canonical backlog status and not mutation authority.

## 1. Vacation exit rule

The Mobile/Vacation selector exists only for the period in which normal Windows/local development is unavailable. When the user returns from vacation:

```text
MobileTaskSelection = OFF
PrimaryWorkSource    = BACKLOG.md
TaskSelection        = current normal governance + current BACKLOG.md
```

`MOBILE.md` and the dated Mobile planning artifacts remain historical/preparation evidence. They do not override normal backlog sprint/task priority. Before selecting the first normal task, rebind current `main`, local/central governance, applicable higher-level/local `AGENTS.md`, current `BACKLOG.md`, working-tree state, toolchain/platform state, and all relevant open PRs.

## 2. Classic review authority

For review-sensitive FlashGate work, the governance-required independent review is performed in Classic as a separate read-only actor.

- `INDEPENDENT_REVIEW` is read-only: no repository mutation, external mutation, correction, or commit.
- The actor that implemented or corrected a change may self-validate but may not claim independent approval of that work.
- A GitHub/Codex review comment is a **review signal**, not the required independent-review evidence.
- A review signal may become a confirmed finding only after independent Classic review reproduces or otherwise substantiates it against the bound repository state and applicable contract.
- Corrections are performed only in an authorized non-independent correction assignment and remain subject to scope, decision, remediation-budget, platform and validation boundaries.
- When a focused independent delta review is required after correction, it is again Classic and read-only.
- On return to Windows/local work, bind the then-current central Slim Governance. Any stricter/newer local authority wins.

## 3. Effect on the vacation preflight artifacts

The dated Wave 1/2, Wave 3/4, and Prepared-PR Remediation artifacts inventory Codex/GitHub review comments as candidate technical risks. They must not be interpreted as completed governance reviews. Their candidate correction notes are hypotheses for Classic review and later authorized remediation, not correction authority.

## 4. Next read-only activity

Continue by performing independent Classic review of prepared PRs in bounded batches, starting with the normal-backlog-relevant roots and their direct children. Each Classic review records:

```text
Repository/Main binding
PR number / BL / base / head / tree
Review mode = INDEPENDENT_REVIEW
RepositoryMutationAllowed=false
ExternalMutationAllowed=false
FindingFixesPerformed=false
ReviewerIndependencePreserved=true
Scope reviewed
Confirmed findings
Rejected/stale Codex signals
Decision boundaries
Native evidence still required
Next authorized boundary
```

No correction is performed inside the review.
