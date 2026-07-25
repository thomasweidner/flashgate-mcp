# Sprint ID Migration — 2026-07-25

## Purpose

This governance migration replaces the former `Sprint 3.x` identifiers with
standalone `SPR-N` identifiers. It changes no backlog ID, task scope, task
status, priority, dependency, or release assignment.

The former suffix and grouped-scope model is retired. Each former
evidence-bearing or planned entry has one independent positive-integer ID, and
all subsequent entries shift forward without collisions.

This file is the authoritative legacy-to-canonical mapping. After the migration
is merged, it becomes immutable history under the project documentation rules.

## Complete mapping

| Legacy sprint ID | Canonical sprint ID | Status |
|---|---|---|
| Sprint 3.35 | `SPR-35` | Done |
| Sprint 3.36 | `SPR-36` | Done |
| Sprint 3.37 | `SPR-37` | Done |
| Sprint 3.38 | `SPR-38` | Done |
| Sprint 3.39 | `SPR-39` | Done |
| Sprint 3.40 | `SPR-40` | Done |
| Sprint 3.41 | `SPR-41` | Done |
| Sprint 3.42 | `SPR-42` | Done |
| Sprint 3.43 | `SPR-43` | Done |
| Sprint 3.44 | `SPR-44` | Done |
| Sprint 3.45a | `SPR-45` | Done |
| Sprint 3.45b | `SPR-46` | Done |
| Sprint 3.45d | `SPR-47` | Done |
| Sprint 3.45 remaining | `SPR-48` | Planned |
| Sprint 3.46 | `SPR-49` | Planned |
| Sprint 3.47 | `SPR-50` | Planned |
| Sprint 3.48 | `SPR-51` | Planned |
| Sprint 3.49 | `SPR-52` | Planned |
| Sprint 3.50 | `SPR-53` | Planned |
| Sprint 3.51 | `SPR-54` | Planned |
| Sprint 3.52 | `SPR-55` | Planned |
| Sprint 3.53 | `SPR-56` | Planned |
| Sprint 3.54 | `SPR-57` | Planned |
| Sprint 3.55 | `SPR-58` | Planned |
| Sprint 3.56 | `SPR-59` | Planned |
| Sprint 3.57 | `SPR-60` | Planned |
| Sprint 3.58 | `SPR-61` | Planned |

The canonical sequence is continuous from `SPR-35` through `SPR-61`.
`SPR-62` is the next free identifier; it is not an assigned sprint.

## Current steering result

- `BACKLOG.md` uses only canonical IDs in its sprint sequence and task
  references.
- No sprint has status `In Progress`.
- Completed entries remain `Done`.
- Planned Version 1.0 entries remain `Planned`.
- Post-Version-1.0 work remains outside an assigned sprint.
- BL-248, BL-251, BL-324, and BL-173 retain their existing status, text, and
  assignment; no implementation work was started.

## Historical exceptions

Earlier merged ADRs, dated migration records, and dated evidence reports retain
their original identifiers as immutable historical text. They do not define the
current sprint model. Current steering, entry-point, roadmap, security, testing,
development, and benchmark guidance uses `SPR-N`.

The historical benchmark report path was normalized without rewriting its
content:

`docs/benchmarks/sprint-045d-resource-latency-baseline.md`
→ `docs/benchmarks/spr-47-resource-latency-baseline.md`

## Validation contract

- canonical IDs are positive integers without suffixes or decimals;
- the current status table contains one row each for `SPR-41` through `SPR-61`;
- the mapping contains one entry each for `SPR-35` through `SPR-61`;
- no canonical ID is duplicated;
- backlog assignments and task definitions are unchanged;
- relative Markdown links resolve;
- the documentation consistency gate accepts the canonical row format.
