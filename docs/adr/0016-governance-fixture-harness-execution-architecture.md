# ADR-0016: Governance Fixture Harness Execution Architecture

## Status

Accepted

## Context

FlashGate governance validation contains long PowerShell fixture matrices and
package validators. Existing case-level checks are deterministic, but process
ownership, timeout cleanup, progress semantics, case metadata, and reusable
orchestration are not yet one explicit architecture contract. Unbounded waits,
task-specific launchers, or separately maintained case lists can leave a run
without trustworthy terminal evidence.

## Decision

Each fixture-matrix invocation owns exactly one controlled runner process. The
controller captures its PID and start identity immediately after launch and
uses bounded timeout, termination, wait, and stdout/stderr drain operations.
Terminal evidence records the concrete process identity, exit state, cleanup
result, survivor scan, and repository-state parity. A timeout or unverifiable
cleanup fails closed.

One canonical case inventory owns every case name, group, tag, supported
platform, required capability, and Windows-only dependency marker. `-CaseName`
remains compatible during migration, while `-ListGroups`, `-ListTags`,
`-ListCases`, group selection, tag selection, and platform/capability filtering
are derived from the same inventory. Its deterministic serialized form and
SHA-256 bind selection evidence; no Shell/PowerShell array or second manually
maintained list is permitted as a canonical source. Selector resolution
completes before the runner process starts. Unknown, duplicate, ambiguous,
platform-incompatible, or capability-incomplete selections stop fail-closed,
identify the affected case IDs in structured diagnostics, and execute zero
fixtures. A native subset may contain only cases whose canonical metadata
supports Linux, whose required capabilities are present, and whose Windows-only
dependency set is empty.

Long runs emit machine-readable progress and heartbeat events. Progress is
`completed/selected` with a named unit and phase. Completed means terminal
`PASS`, `FAIL`, `SKIPPED`, `BLOCKED`, or `CANCELLED`; `PENDING` and `NOT_RUN`
remain non-completed. Status counts, warning counts, and execution counters use
the canonical governance contract. Missing progress instrumentation on a
recurring long run is a governance finding.

Every terminal case has exactly one terminal ProgressEvent. Event fields are
limited to technical identity, sequence, event type, status, completed and
selected counts, unit, phase, and elapsed time. Identical events are
suppressed. A heartbeat is explicitly typed, occurs only after the configured
interval, and is deduplicated for the same interval and state. An event is
emitted only for progress, phase change, status change, or heartbeat.

Reusable orchestration performs current-state, toolchain, parameter, Temp,
sandbox, and harness preflights, then follows the validation funnel: root-cause
checks, directly affected components, documentation convergence, and exactly
one final full run. Resume/checkpoint evidence is accepted only when the
orchestrator binds immutable inputs, selection inventory, completed cases,
outputs, and repository state; otherwise the run starts fresh.

A hash-verified task-local native toolchain is assignment-scoped rather than
attempt-scoped. It is retained and reused through directly caused fixture,
parser, schema, or full-run corrections and is removed only at a terminal
handoff or an external authorization boundary. No correction implicitly grants
another download. When any natively relevant source hash changes, prior native
evidence is invalidated and the retained runtime must validate the new source
binding. External local documents use optimistic concurrency: bind their hashes
before mutation, preserve non-overlapping foreign changes, and stop before any
write when the task-owned section is missing or overlaps a foreign delta.

All pre-execution gates complete before mutation. The source repository and
worktree are explicit parameters; no active or main repository copy is
hard-coded. The execution context is selected before launch. Branch, commit,
tree, and selected file hashes bind the exact source, and isolated native
evidence must validate the later delta worktree. Standard Git and PowerShell
probes use helper names that do not shadow external commands or cmdlets,
deterministic detached-HEAD detection, direct exit-code evaluation, and a
timeout budget derived from probe count and measured runtime. A known required
normal-user context is chosen directly. Scope overruns stop fail-closed.

`Status` carries the running or terminal state. `FailureCount` counts only
factual or technical `FAIL`; `BLOCKED` does not increase it. Infrastructure and
invocation failures remain one total counter. Warning counters keep their
existing invariant. Historical and current contract versions remain separate,
and no redundant blocker or invocation summary fields are introduced.

## Consequences

- BL-337 implements process isolation and bounded cleanup.
- BL-338 implements canonical metadata, listing, and selection.
- BL-339 implements reusable focused/full orchestration and telemetry.
- Until BL-338 supplies the complete reusable metadata layer, the current
  governance runner derives its active count, ordered metadata-inventory
  SHA-256, platform/capability-valid resolved selection, and completion parity
  from one runtime inventory assembled from its canonical case definitions; no
  fixed current count or separately maintained native case list is
  authoritative.
- Until BL-339 Phase A updates the workflow generator, schema-version-1 workflow
  records whose readiness fields remain pending or false may omit
  `currentStateGate`. Current readiness claims and the versioned
  `GENERIC_COMMIT_PREPARATION` profile continue to require the typed gate.
- Existing historical fixture results remain evidence; their runners migrate
  without silently changing case identity or count semantics.
- This ADR changes governance tooling only, not FlashGate runtime behavior.

## Validation requirements

- positive completion and heartbeat evidence;
- timeout, kill, wait, stream-drain, survivor, and repository-mutation negatives;
- deterministic inventory/listing and group/tag/legacy-selection parity;
- progress, status, warning-invariant, and execution-counter negatives;
- blocked-without-failure, selector-preflight, duplicate-event/heartbeat,
  explicit-source/worktree, helper-shadowing, detached-HEAD, exit-code, and
  scope-overrun positives or fail-closed negatives;
- Windows PowerShell 7.6.4 and native Linux PowerShell validation.

## Related documents

- [Testing](../testing.md)
- [Efficiency improvement plan](../efficiency-improvement-plan.md)
- [Change-trigger standard](../../Governance/CHANGE-TRIGGER-REVIEW-AND-BACKLOG-STANDARD.md)
- [Finding and review-mode standard](../../Governance/FINDING-REMEDIATION-AND-REVIEW-MODE-STANDARD.md)
