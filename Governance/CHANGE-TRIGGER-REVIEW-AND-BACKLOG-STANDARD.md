# FlashGate Change-Trigger Review and Backlog Registration Standard

**Status:** Binding
**Tasks:** BL-333 foundation, BL-334 enforcement, BL-336 handoff-profile generalization, and BL-340 profile migration
**Machine-readable source:** [change-trigger-catalog.json](change-trigger-catalog.json)

## Purpose

Every FlashGate assignment classifies material change at the checkpoints
defined below. Completed backlog items remain `Done`; their permanent gates
are reused. Existing open work is extended only while the acceptance boundary
remains coherent. New independently reviewable work is registered without
waiting for a user reminder.

## Required checkpoints

| Checkpoint | Required review |
|---|---|
| `ASSIGNMENT_START` | Classify intended scope, affected domains, existing backlog coverage, continuous gates, and decision boundaries before implementation. |
| `MATERIAL_SCOPE_CHANGE` | Repeat classification when a changed requirement, implementation fact, or finding materially expands the scope. |
| `PRE_COMMIT` | Prove gate disposition, duplicate search, backlog/status/documentation consistency, handoff readiness, and commit-preparation state. |
| `SPRINT_CLOSE` | Reconcile acceptance evidence and continuous governance obligations without repeating unaffected matrices. |
| `RELEASE_CANDIDATE` | Review every release-relevant trigger and verify the authoritative Hosted CI sources. |
| `STABLE_RELEASE` | Reconfirm release-wide trigger coverage against the exact candidate commit and accepted CI evidence. |

An assignment record conforming to
[assignment-governance-record.schema.json](assignment-governance-record.schema.json)
is mandatory at each applicable checkpoint. A material scope change creates a
new record; it does not rewrite the earlier checkpoint record.

### Current-state gate and assignment separation

Before architecture, backlog, ADR, or implementation decisions, the assignment
records a current-state gate. It binds repository identity, baseline/current
commit, branch, complete relevant index/working/untracked state, authorized
scope, ID inventory, and parallel-worktree state. A stale or incomplete binding
blocks dependent decisions instead of being filled by an implementation
assumption.

The main assignment and a reusable generic enabler are separate records when
they have different acceptance criteria, owners, schedules, or review evidence.
The main assignment may depend on an existing enabler; it does not silently
absorb or mark that enabler complete.

## Trigger handling

The catalog is exhaustive for the current governance contract. A trigger may
be derived from repository paths, an explicit event, or both. Diff-derived
triggers are the minimum set: assignments must add event-derived triggers
that are not visible from paths alone.

Changed paths are canonical repository-relative forward-slash paths. Empty
entries, absolute Windows or Unix paths, UNC paths, drive prefixes, `.` or `..`
segments, doubled separators, backslashes, control characters, and any input
whose normalized representation differs from the supplied value fail closed.
Every relevant tracked production, workflow, governance, script, security,
build, release, and documentation path maps to at least one DIFF trigger.
Explicit exclusions are machine-readable, narrowly scoped, and justified.
`NO_TRIGGER` is permitted only for an actually empty changed-path set or a
cataloged non-material exclusion; an unknown non-empty path set fails.

For each observed trigger:

1. identify the affected continuous gates;
2. search `BACKLOG.md` and current decision/security/testing sources for a
   duplicate;
3. reuse an existing gate when the accepted contract is unchanged;
4. update an open backlog item when the new requirement remains within its
   coherent acceptance boundary;
5. register a new item when it introduces a separately reviewable capability,
   contract, platform, security boundary, artifact/release obligation,
   persistent gate, or deferred risk;
6. record release impact and all deliberately omitted checks.

Review, CI, incident, and field findings use the same procedure. A finding is
not exempt merely because it appeared late in the lifecycle.

## Decision boundaries and mandatory stop

Stop and request a separate decision when proceeding requires a new:

- `PRODUCT_DECISION`
- `ARCHITECTURE_DECISION`
- `SECURITY_DECISION`
- `PLATFORM_DECISION`
- `DEPENDENCY_DECISION`
- `RELEASE_DECISION`
- `SCOPE_DECISION`

The assignment record names the boundary, evidence, blocking effect, owner,
and next action. A boundary may not be converted into an implementation
assumption. Directly related issues that do not cross one of these boundaries
are handled under the same-run remediation standard.

## Backlog registration

Before adding work:

1. read the current canonical catalog in `BACKLOG.md`;
2. search titles, acceptance notes, related ADRs, security documents, testing
   guidance, and recent findings for equivalent scope;
3. use the current sequential-numbering rule rather than remembered IDs;
4. record origin, risk, affected components, acceptance criteria,
   dependencies, platform/release evidence, and blocking effect;
5. update sprint mapping and `BACKLOG.md` when applicable.

Routine revalidation does not create backlog work. Unfinished independently
reviewable work must not be hidden inside a completed item.

## Assignment and report contract

Every applicable record and completion report provides:

- `RecordedAt`, `Repository`, `BaselineCommit`, and `Branch`
- `ExecutionMode`
- `Checkpoint`
- `ChangeTriggerReviewResult`
- `TriggeredDomains`
- `ObservedTriggers`
- `AffectedContinuousGates`
- `ExistingBacklogCoverage`
- `DuplicateSearch`
- `RepeatedChecks`
- `ChecksNotRequired`
- `NewBacklogItems`
- `UpdatedBacklogOrRegisterEntries`
- `DeferredTriggerItems`
- `DecisionBoundaries`
- `ReleaseImpact`
- `DocumentationConsistencyResult`
- the review/remediation fields from the finding standard;
- the readiness fields from the handoff standard.

Assignment records conform to
[assignment-governance-record.schema.json](assignment-governance-record.schema.json).
The machine-readable completion report conforms to
[completion-report.schema.json](completion-report.schema.json). Correction
handoffs additionally conform to
[finding-correction-matrix.schema.json](finding-correction-matrix.schema.json),
[finding-regression-matrix.schema.json](finding-regression-matrix.schema.json),
[publication-regression-evidence.schema.json](publication-regression-evidence.schema.json),
[publication-regression-expected-execution-input-binding.schema.json](publication-regression-expected-execution-input-binding.schema.json),
[publication-regression-result.schema.json](publication-regression-result.schema.json),
[publication-regression-matrix-catalog.schema.json](publication-regression-matrix-catalog.schema.json),
[focused-delta-review-record.schema.json](focused-delta-review-record.schema.json),
and [governance-report-contract.schema.json](governance-report-contract.schema.json).
A narrative Markdown report contains exactly one bounded strict JSON contract;
it is not a substitute for the machine-readable records. Records and reports
bind repository, baseline, current commit, actual correction/current-delta
bytes, workflow commit, run identity, run attempt, event, ref, and head SHA to
trusted expected values supplied outside the data being validated.

`ChangeTriggerReviewResult` is one of `NO_TRIGGER`,
`EXISTING_GATES_REQUIRED`, `EXISTING_BACKLOG_UPDATED`,
`NEW_BACKLOG_REGISTERED`, or `BLOCKED_PENDING_DECISION`.

BL-336 is registered in the machine-readable catalog as the accepted follow-up
for task-neutral handoff profiles. The pre-registration classification
`NEW_BACKLOG_REQUIRED` becomes the canonical persisted result
`NEW_BACKLOG_REGISTERED` once the backlog and catalog entries exist.

BL-340 adds no new trigger vocabulary. Current generated records use the same
catalog and must carry `recordReadinessClass=CURRENT` plus a passing
`currentStateGate`. Historical schema-version-1 records remain readable through
their versioned schema, but absence of that current readiness class prevents
their reuse as current checkpoint evidence.

## Enforcement

[`../scripts/Test-GovernanceConsistency.ps1`](../scripts/Test-GovernanceConsistency.ps1)
fails closed when the catalog, standards, schema, assignment record, derived
diff triggers, backlog continuity, handoff state, or Hosted CI source evidence
is inconsistent. Its focused fixture matrix includes positive and negative
mode, trigger, boundary, checkpoint, finding, and handoff cases.
