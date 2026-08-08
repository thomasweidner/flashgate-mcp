# FlashGate Finding Remediation and Review-Mode Standard

**Status:** Binding
**Tasks:** BL-333 foundation, BL-334 enforcement, and BL-336 handoff-profile generalization

## Modes

| Mode | Independent | Repository mutation | External mutation | Review scope | Commit allowed | Correction allowed | Prior review | Focused delta review |
|---|---:|---:|---:|---|---:|---:|---:|---:|
| `INDEPENDENT_REVIEW` | Yes | No | No | `FULL_INTEGRATION` exactly once | No | No | No | No |
| `BUNDLED_CORRECTION` | No | Yes, within approved scope | Yes, within approved scope | `APPROVED_CORRECTION_SCOPE` | No | Yes | Yes | Required afterward |
| `FOCUSED_INDEPENDENT_DELTA_REVIEW` | Yes | No | No | `CORRECTION_DELTA`, findings, direct interfaces, and regression evidence | No | No | Yes | This is the focused review |
| `COMMIT_PREPARATION` | No | No | No | `COMMIT_SCOPE` only | No | No | Yes | Must already be complete |

An actor that implements or corrects work may not claim independent approval
of that work. The full review and focused delta review are read-only.
The properties above are immutable control semantics. The machine-readable
catalog repeats them for discoverability, but the validator compares the
catalog and every record against its own canonical constants. Neither the
catalog nor an assignment record is the authority for mutation, independence,
correction, or commit permission. `COMMIT_PREPARATION` never creates a commit.

## Same-run remediation

During `BUNDLED_CORRECTION`, correct every newly discovered issue when it is:

- directly caused by or directly adjacent to the approved change;
- within the approved repository/external-writing boundary;
- testable with the current toolchain and permanent project evidence; and
- free of a mandatory decision boundary.

Record such an issue as `DISCOVERED_AND_CORRECTED_IN_RUN`, include its
evidence, and add success, error, boundary, security, and platform regression
coverage as applicable. Do not return one directly related defect at a time.
Every discovered-in-run finding has exactly one ID-identical correction entry.
Both entries state cause, evidence, affected paths or an explicit non-file
boundary, correction, and permanent regression-evidence IDs. Missing, duplicate,
or orphaned partners fail closed.

### Adaptive bounded remediation budgets

The remediation budget is selected once from the artifact state at assignment
start:

- a new or materially rebuilt artifact permits at most 12 correction and
  revalidation cycles;
- an established, previously validated artifact permits at most 6 cycles; and
- after the first productive write-capable operation has been invoked,
  automatic retry or automatic full-assignment re-execution has a budget of 0.

A cycle is one directly related correction followed by the smallest sufficient
focused validation. The budget is a safety ceiling, not a target. Stop earlier
when validation passes or a mandatory decision or authorization boundary is
reached. A material rebuild means that prior validation evidence no longer
represents the artifact's relevant control flow or structure; normal focused
maintenance of an established artifact does not reset its budget to 12.

Directly caused, isolated harness, fixture, parser, instrumentation,
diagnostic, or classification defects remain in the same authorized assignment
while budget and scope remain. They are not returned individually to Classic.
Classic or the owner is required only for a binding decision, authorization,
scope expansion, exhausted budget, or an independently required review.

Material correction cycles, validation executions, and infrastructure or
invocation failures are different counters. Parser reruns, read-only diagnosis,
and unchanged focused checks do not become material correction cycles. Reports
also separate observed, resolved, and open warnings and enforce
`ObservedWarningCount = ResolvedWarningCount + OpenWarningCount`.

`FailureCount` counts only factual or technical `FAIL` results. `BLOCKED`
records an unavailable prerequisite or authorization boundary in `Status` and
does not increment `FailureCount`. Infrastructure and invocation failures use
only `InfrastructureOrInvocationFailureCount`; no redundant blocker or
open/resolved invocation summary fields are added.

Validation uses a funnel: root-cause evidence, directly affected components,
documentation convergence, and exactly one final complete run. A repeated full
run needs a documented new technical cause. The first independent review covers
the complete approved risk space; after a stable core, findings are triaged and
corrections receive only a focused independent delta review unless a documented
full-review trigger applies.

For recurring long runs, missing numeric completed/selected progress with a
named unit and phase is an instrumentation finding. Status updates occur only
when phase, progress, warning, or result changes.

### Productive mutation-attempt boundary

The productive mutation attempt begins immediately before invoking the first
operation that is capable of changing productive or otherwise authorization-
bound state. This includes create, replace, move, delete, rename, reference
rewrite, productive backup creation, remote write, or an equivalent write-
capable API request. The attempt counts even when that operation returns
without a confirmed state change.

Read-only inspection, parsing, dry runs, isolated fixtures, and writes confined
to an expressly isolated disposable validation root do not cross this boundary.
Preparation that creates or changes a productive backup, quarantine, source,
target, active reference, repository, remote, or other authorization-bound
object does cross it. After the boundary, do not automatically launch another
productive attempt. Only an explicitly preauthorized compensating action
inside the same attempt is allowed; a new productive attempt requires a new
explicit authorization.

### Final activity-gate isolation

Before a final activity gate, external monitors and control processes must
either be ended or operate without the monitored plaintext roots, paths, and
object names in their command line, environment, temporary files, or other
gate-visible state. A control process is not exempt merely because it is
read-only. Between the final passing gate and the first productive
write-capable operation, perform no external, filesystem, process, task,
shortcut, network, or other time-varying check.

## Mandatory stop conditions

Stop rather than infer a decision when correction requires a new product,
architecture, security, platform, dependency, release, or scope decision.
Also stop when correction requires unapproved credentials, production data,
remote mutation, installation, destructive cleanup, or a write outside the
approved boundary.

Each stop condition records its decision-boundary type, evidence, impact,
owner, proposed target backlog/register, and exact next action.
Every deferred finding names exactly one `boundaryId`. That boundary exists,
is blocking, states a concrete cause and next action, and uses the same
decision class required by the finding. Unrelated, non-blocking, missing, or
ambiguous boundaries fail closed.

## Review sequence

1. Implement and self-validate the complete approved scope.
2. Run exactly one `INDEPENDENT_REVIEW`.
3. Correct all findings in one `BUNDLED_CORRECTION`.
4. Run one `FOCUSED_INDEPENDENT_DELTA_REVIEW`.
5. If residual direct findings remain, perform one further bundled correction
   and another focused delta review. Do not repeat the full review unless the
   feature scope materially changes and the reason is recorded.
6. Enter `COMMIT_PREPARATION` only after all findings are closed.

## Required record fields

Correction records contain:

- `OriginalFindings`
- `DiscoveredInRunFindings`
- `CorrectedInRunFindings`
- `DeferredFindings`
- `StopConditionsEncountered`
- `SelfReviewIterations`
- `PermanentRegressionEvidence`
- `FocusedValidationResult`
- `IndependentDeltaReviewRequired`

Independent review records also state:

- `RepositoryMutationAllowed=false`
- `FindingFixesPerformed=false`
- `ReviewerIndependencePreserved=true`

`COMMIT_PREPARATION` records prove that no open finding or decision boundary
remains and that the independent delta review is complete.

Finding-free assignments satisfy `allFindingsClosed` with an empty finding set;
they do not create placeholder findings, correction matrices, or regression
matrices. The generic commit-preparation profile binds the external independent
review directly to the reviewed paths and hashes. Finding-correction artifacts
remain exclusive to the `FINDING_CORRECTION` profile.

Focused delta records additionally bind the prior review package and hash,
correction-start commit, `correction-only.patch` and its byte-exact SHA-256,
`current-delta.patch` and its byte-exact SHA-256, correction-only paths, direct
interface paths, reviewed finding IDs, regression evidence IDs, allowed delta
paths, and reference-only paths. The productive validator calculates both
patch hashes from the unmodified package-entry bytes and compares them with
separately supplied expected values and every manifest, assignment, focused,
completion, and report declaration. Changed paths must be a subset of
correction plus direct-interface paths and must not contain reference-only or
unrelated paths.

For a finding correction, the strict correction matrix, regression matrix,
focused-delta record, completion report, and bounded machine-readable report
contract contain exactly the same finding IDs. Severity, pending-delta status,
disposition, correction evidence, affected paths, regression tests, and
evidence references agree per finding. `CORRECTED_PENDING_DELTA` is the only
correction-run terminal status; `CLOSED` requires the later independent delta
review.

## Backlog interaction

Finding origin does not change the registration rule. Review, Hosted CI,
incident, and field findings are corrected in the approved scope or
deduplicated and registered canonically. A completed backlog item is not
reopened merely because its permanent gate detects a new problem.
