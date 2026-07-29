# FlashGate Finding Remediation and Review-Mode Standard

**Status:** Binding
**Tasks:** BL-333 foundation and BL-334 enforcement

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
