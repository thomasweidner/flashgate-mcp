# Backlog ID Migration - 2026-08-12

## Status

Canonical planning migration fully integrated. PR #34 merged through
`26734c333341455a63f79c0f1a956309e54177e0`, and all post-merge CI, Metadata
Regression, and CodeQL checks passed. BL-339 is `Done`; BL-340 and BL-341 remain
`Planned`.

## Reason

The fully implemented, validated, and independently reviewed governance
orchestration scope is the complete canonical BL-339 task. The remaining
workflow-generator/profile migration is independently implementable and is
therefore extracted as BL-340. The former BL-340 host-process ownership and
lifecycle task moves to BL-341 without changing its scope.

## Canonical mapping

| Previous current-state identity | Canonical identity | Status | Meaning |
|---|---|---|---|
| BL-339 Phase A | BL-339 | Done | Reusable focused/full governance validation orchestration |
| Residual scope within BL-339 | BL-340 | Planned | Complete governance generator/profile migration |
| BL-340 | BL-341 | Planned | Define cross-mode host-process ownership and lifecycle |

BL-340 reuses the completed BL-339 orchestration contract. It owns full
workflow-generator/profile migration, current `currentStateGate` generation,
and explicit schema-version-1 historical-read compatibility. It does not
reopen BL339-REV findings and is not a new prerequisite for resuming BL-324.

BL-341 retains the former BL-340 title, scope, SPR-59 assignment, and ADR-0017
binding. Only its canonical current ID changes.

## Preservation boundary

Historical migration records, changelog entries describing their contemporary
state, reports, immutable review packages, manifests, evidence paths, hashes,
and finding IDs are not rewritten. Current steering sources use the canonical
BL-339/BL-340/BL-341 mapping above.

## Canonical range

The active backlog is continuous and unique through `BL-341`.
