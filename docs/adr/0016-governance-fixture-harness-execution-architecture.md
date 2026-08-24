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

BL-338 realizes this inventory as the closed, strict-UTF-8
`Governance/governance-case-metadata.json` contract with
`Governance/governance-case-metadata.schema.json`. Its contiguous `Order`
preserves the established BL-339 execution order; groups and tags are derived
ordinally. `scripts/GovernanceCaseSelection.psm1` is the single reader,
semantic validator, canonicalizer, list provider, and selector resolver. The
fixture harness assigns executable descriptor identity from that canonical
order; descriptor code owns only execution inputs. Supplemental routes consume
ordered metadata subsets, so no second maintained CaseId/group/tag list exists.
The repository fixture harness is contributor- and Hosted-CI-portable. It has
no runtime dependency on a local Codex-Work tree, contributor-specific absolute
path, or external validator copy. It performs its own fail-closed platform and
capability preflight from explicitly supplied executable paths/capabilities or
from deterministic repository-host process discovery. Local Codex callers bind
the `pwsh-governance` and `git-local-readonly` INF-160 routes at their caller
boundary and may pass those validated executables into the harness; INF-160 is
not a repository or Hosted-CI installation requirement.

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

BL-339 realizes that decision as one strict request/result pair, one
permanent PowerShell module, and one thin runner. BL-340 completes the migration;
the catalog owns exactly nine
orchestration profiles. Their common cheap-gate prefix is parser/syntax, leading
`.gitattributes`/`.editorconfig` text policy, `git diff --check`, complete
VERSIONED/IGNORED/GIT_EXCLUDED/EXTERNAL input existence, classification, roots,
links, and hashes, toolchain/platform/execution context, and
repository/source/worktree/HEAD/tree/file-hash plus canonical selector
resolution.
The first failed cheap gate marks later cheap gates `NOT_RUN` and prevents typed
subordinate evidence from being consumed.

The BL-339 correction handoff uses the same sole producer with the explicit
`FINDING_CORRECTION` discriminator. Its previous-review state is either a
commit or an immutable review package; package mode reconstructs the reviewed
tree and binds every relevant previous postimage before a correction patch may
be accepted. Directory-first preflight is a validation state, not a second
producer and not authorization to write a ZIP.

The same producer also creates a fresh ZIP-free final-content staging directory
whose internal bytes already describe the immutable review package. The
preflight and final-content states are discriminated and independently
validated; only a later authorization may perform the single `CreateNew`.
Product validation derives correction inventory from isolated Git plumbing,
binds rename source and target, rejects actual/reference-only overlap, and
task-binds every finding set. Commit and immutable-package previous-review modes
are both executed by the permanent functional fixture matrix.

The same report survives both directory states as a complete correction
artifact. A canonical embedded JSON contract binds findings/dispositions,
previous review, original patch bytes and path counts, scope semantics,
regression evidence, and focused validation. Final-content derivation changes
only its six lifecycle/package fields; the productive validator reparses and
cross-checks all subject fields instead of accepting narrative token presence.

The complete source working-tree state is an immutable request binding. The
orchestrator hashes the unmodified stdout bytes of exactly
`git status --porcelain=v2 --untracked-files=all`, compares the actual SHA-256
with `expectedStatusSha256`, and reports both values. Scope and file-hash paths
must be equal canonical sets with no exact duplicates or Windows case
collisions. A protected foreign worktree is never an implicit exception: it is
modeled independently with root, HEAD, tree, branch/detached state, and its own
raw-status hash.

External inputs use platform-native path comparison, include the declared
source root itself in link/reparse checks, and derive `GIT_EXCLUDED` only from
the actual `.git/info/exclude` or resolved `core.excludesFile` provenance
reported by Git.

For a separately declared publication matrix, the controller also owns an
Expected Execution Input Binding created before runner start from the canonical
catalog, runner, and dependency bytes. It passes both the strict binding
artifact and its SHA-256 to the child. The runner uses independent .NET
primitives to verify that contract before every module import and immediately
before the first case, then rechecks it before recording any case result.
Result V2 preserves exactly the controller's binding. Runner or dependency
drift in either pre-start or post-import window therefore terminates with zero
accepted cases rather than evidence from an unbound loaded code version.

Task assignments are data. Existing fixture selectors remain the only dynamic
case inventory and are bound by inventory and selection hashes; BL-340 invokes
that resolver rather than hashing a selector object alone and does not add case,
group, tag, platform, or capability metadata. One canonical
reader rejects invalid UTF-8, BOMs, duplicate properties, schema/version/profile
mismatches, non-terminal status, trailing JSON, counter violations, and
`BLOCKED`/failure-count contradictions. Unchanged PASS evidence is reusable only
when its own hash and every dependency hash still match. Relevant changes
invalidate only dependent evidence.

`New-GovernanceHandoff.ps1` remains the sole package/manifest generator. It
validates fresh staging as a directory after inventory/manifest generation,
writes the final ZIP once with `CreateNew`, and reopens the archive through the
same validator. Directory failure writes no ZIP; any invalid attempted ZIP is
discarded. The package-attempt counter changes from `0` to `1` immediately
before the first write-capable open of the final path, including a failed open;
there is no automatic retry. This decision does not change BL-337 process-control ownership or
BL-338 dynamic listing/selection ownership.

The same producer supports four explicitly discriminated generic transitions.
`IMPLEMENTATION_TO_INDEPENDENT_FULL_REVIEW` packages the complete implementation
and hash-bound reusable full-completion evidence before any independent review;
its review status is `NOT_PERFORMED` and it cannot contain or depend on an
independent-review artifact. `GENERIC_COMMIT_PREPARATION` remains the later
review-dependent transition. Both use directory-first validation and the same
exactly-once final ZIP boundary; their evidence members and schema fields are
mutually exclusive.

`EVIDENCE_ONLY_FOCUSED_REVIEW` packages only an empty repository scope and one
hash-verified external/read-only evidence artifact; it has no patch members.
`POST_MERGE_CLOSURE` adds real packaged merge-state and live-readback bytes
whose hashes are verified by producer and validator, and explicitly records
unchanged matrices as `NOT_RUN`. Both retain
the same directory-first, manifest, one-write, reopen, and collision/link safety
contract as the two patch-bearing profiles.

For exact-commit preparation, the result binds intended base, merge base,
effective PR scope and patch hash, isolated integration-projection tree,
authorized write set, staged-state prohibition, and every protected foreign
worktree. The request also carries eight state components (commit, tree,
working status, scope, selector, package, external inputs, and evidence).
Current hashes are derived from the validated actual state rather than trusted
from the request; only components with changed hashes become `INVALIDATED`,
with a deterministic reason. Executable task-controller inventory is scanned
from the fixed root derived from the bound worktree and compared with request
expectations and actual file/line counters; omissions, outside-root paths, and
unknown exceptions fail before runner execution.

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
- BL-340 completes the workflow-generator and reusable-profile migration onto
  the BL-339 contract, including current `currentStateGate` generation and
  explicit schema-version-1 historical-read compatibility. Historical records
  remain readable but cannot become current readiness evidence without the
  current readiness discriminator and state gate.
- BL-338 supplies the complete reusable metadata layer. The governance runner
  derives its active count, ordered metadata-inventory SHA-256,
  platform/capability-valid resolved selection, and completion parity from the
  JSON inventory. No fixed current count, runtime-assembled metadata catalog,
  or separately maintained native case list is authoritative.
- The currently migrated path accepts a hash-bound BL-339 orchestration request
  and result. BL-340 owns complete workflow-generator/profile migration: new
  generated records bind `currentStateGate`, while historical stored
  schema-version-1 records remain governed by their versioned schema.
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
- Windows PowerShell 7.6.5 and native Linux PowerShell 7.6.4 validation.

## Related documents

- [Testing](../testing.md)
- [Efficiency improvement plan](../efficiency-improvement-plan.md)
- [Change-trigger standard](../../Governance/CHANGE-TRIGGER-REVIEW-AND-BACKLOG-STANDARD.md)
- [Finding and review-mode standard](../../Governance/FINDING-REMEDIATION-AND-REVIEW-MODE-STANDARD.md)
