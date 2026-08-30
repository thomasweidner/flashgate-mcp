# Handoff Artifact and Classic Review Readiness Standard

**Status:** Binding
**Tasks:** BL-333 foundation, BL-334 enforcement, BL-336 handoff-profile generalization, and BL-340 profile migration

## Principle

A handoff is complete only when the next actor's required inputs exist,
identify one authoritative source state, and pass integrity and scope checks.
A report that names an uncreated or inaccessible artifact is incomplete.

The cross-project storage rule is authoritative. FlashGate produces exactly
one handoff file below `<CodexTempRoot>` or an explicitly authorized
assignment-specific override. When exactly one payload file is required, that
file may be transferred directly without a ZIP. When more than one payload
file is required, the handoff is exactly one ZIP archive. Package members are
never requested, uploaded, or transferred separately. Task-related names
begin with the canonical identifier confirmed in `BACKLOG.md` and the current assignment scope.

Before any ZIP is created, a ZIP-free machine-readable readiness gate must pass
against the converged fachliche, documentation, Windows/Linux (when required),
scope, patch, inventory, review, and warning state. Package generation is not a
substitute for readiness.

The sole generic package producer is `scripts/New-GovernanceHandoff.ps1`. It
assembles fresh staging, generates the complete inventory and manifest, and
passes the same semantic validator against that directory before any package
write. Only then may it create one fresh same-parent candidate with `CreateNew`.
The producer serializes the candidate exactly once, closes it, binds its
SHA-256/length identity, and product-validates those bytes. After validation it
reopens the candidate with a write/delete-excluding read lease, rejects identity
drift, and atomically creates the no-overwrite final hard link to that same
validated filesystem object. A pre-publication failure or interruption never
exposes a partial final ZIP; the diagnostic candidate may remain. A second
manifest or package generator is prohibited.

Historical immutable-package scope `pathCount` is the canonical entry count.
Rename source and target paths form a separately expanded canonical path set and
are compared with the reconstructed historical patch inventory.
`previousReviewedPathCount` retains the historical entry-count meaning.

`PackageWriteAttemptCount` is set to `1` immediately before the first
write-capable `FileStream(CreateNew, Write)` call on the candidate path. It
remains `1` when that call fails because the target exists, is locked, is a
directory, is denied, races, or returns another I/O error. Failures before that
boundary remain `0`; no automatic second final-package attempt is permitted.

The current-state request binds the raw stdout bytes from exactly
`git status --porcelain=v2 --untracked-files=all` with an explicit
`expectedStatusSha256`. Validation recollects and hashes those bytes without
text, newline, culture, or object-serialization conversion and compares
expected with actual. `scopePaths` and `fileHashes.path` are duplicate-free,
free of Windows case collisions, and equal as exact canonical sets. Every
legitimate foreign or protected worktree exception is separately and
explicitly bound by root, commit, tree, branch/detached state, and its own raw
status hash; there is no implicit status exception.

A final ZIP is immutable and already product-valid when atomically published.
It is never opened for in-place correction, member replacement, manifest repair,
or re-signing. A rejected candidate is never published; corrected content is
assembled in a wholly fresh staging root and the manifest is regenerated.
Classic receives exactly that one new complete manifest- and SHA-256-validated
ZIP, never repaired or separately transferred members.

## Required payload by transition

| Transition | Required payload |
|---|---|
| Implementation to independent full review | Handoff README, report, complete feature patch, scope inventory, focused evidence, and manifest. |
| Bundled correction to focused delta review | Handoff README, correction report, byte-exact `correction-only.patch` and `current-delta.patch`, separately trusted expected hashes, assignment/completion records, strict correction and regression matrices, strict focused-delta record, bounded report contract, external before/after deltas, scope inventory, validation evidence, and manifest. |
| Commit preparation to commit approval | Exact staged/unstaged scope, commit-preparation report, patch, assignment record, validation evidence, and manifest. |
| Exact commit to push approval | Commit SHA, commit patch, exact-commit validation, scope inventory, and manifest. |
| Push/PR to Hosted CI review | PR identity, head SHA, changed paths, workflow/run identities, and authoritative source URLs or immutable identifiers. |

Ignored build output and unrelated preparation workspaces are excluded unless
the next step explicitly requires them.

## Explicit transition and profile contract

Every newly produced multi-file handoff declares both `transitionType` and
`profile`; neither value is inferred from a task ID, filename, or incidental
package member. The supported pairs are:

| Transition type | Profile | Purpose |
|---|---|---|
| `IMPLEMENTATION_TO_INDEPENDENT_FULL_REVIEW` | `IMPLEMENTATION_TO_INDEPENDENT_FULL_REVIEW` | Complete implementation and reused full-completion evidence for the first independent full review; no prior independent-review evidence exists or is required. |
| `COMMIT_PREPARATION_TO_COMMIT_APPROVAL` | `GENERIC_COMMIT_PREPARATION` | Task-neutral commit preparation, including finding-free work. |
| `EVIDENCE_ONLY_TO_FOCUSED_REVIEW` | `EVIDENCE_ONLY_FOCUSED_REVIEW` | Independent focused review over newly bound read-only evidence with an empty repository delta and no synthetic patch. |
| `POST_MERGE_TO_DOCUMENTATION_CLOSURE` | `POST_MERGE_CLOSURE` | Post-merge documentation/status closure with reused merge state, a live external readback, and unchanged matrices explicitly `NOT_RUN`. |
| `BUNDLED_CORRECTION_TO_FOCUSED_DELTA_REVIEW` | `FINDING_CORRECTION` | Strict finding correction and focused delta review. |

`FINDING_CORRECTION` supports a ZIP-free `PreflightOnly` execution of the sole
productive generator. It materializes a fresh flat directory containing the
complete correction payload, package inventory, and manifest, then runs the
productive semantic validator. `ReadyToExecute=true` means only that the
separately authorized final package write remains. The generator must not open,
probe, create, or replace the final ZIP path in this mode, and
`PackageWriteAttemptCount` remains zero. `ClassicReviewReady` remains false
until the one final package exists and passes reopen validation.

Before that separately authorized write, `-FinalPackageContentOnly` builds a
second fresh flat directory from the same canonical source. Its internal
contract is already `ArtifactLifecycleState=FINAL_REVIEW_PACKAGE`,
`ReadyToExecute=false`, `ClassicReviewReady=true`, contractual
`PackageWriteAttemptCount=1`, and `NextAction=FOCUSED_INDEPENDENT_DELTA_REVIEW`.
The generator runtime attempt counter remains zero because no final path is
opened. Both directories pass the same productive validator. The later package
write serializes final-content bytes once; no preflight-state ZIP and no
in-place metadata repair are permitted.

The correction `report.md` is not a lifecycle summary. It contains exactly one
machine-readable contract between
`BEGIN/END FINDING-CORRECTION-REPORT-CONTRACT` markers and conforms to
`finding-correction-report-contract.schema.json`. It binds task, profile,
transition, lifecycle, previous-review state and SHA-256, the previous-binding
artifact and SHA-256, byte-exact current/correction patch hashes and path
counts, exact findings and dispositions, correction/direct-interface/
reference-only semantics, permanent regression evidence, focused validation,
the required independent delta review, Classic readiness, package attempt
count, and next action. Final-content generation preserves every subject field
and changes only `artifactLifecycleState`, `status`, `readyToExecute`,
`classicReviewReady`, `packageWriteAttemptCount`, and `nextAction`.

When a finding declares a separate publication regression matrix, the
correction payload additionally contains both
`publication-regression-result.json` and
`publication-regression-evidence.json`. The calling process derives a canonical
Expected Execution Input Binding from the catalog before it starts the matrix
runner and binds the expected-binding artifact itself by SHA-256. Using only
independent .NET file, UTF-8, JSON, and hashing primitives, the runner verifies
that binding before each module import and again immediately before Case 1.
Every later case-record operation rechecks it, so changed runner or dependency
bytes cannot become PASS evidence. Result V2 contains exactly that parent-bound
pre-execution binding. Evidence V2 binds the result bytes and independently
rebinds those same inputs. The catalog owns the fixed case set, runner, result schema, and complete
source/dependency path sets. The productive validator derives result counts and
case IDs from `results[]`, requires exact Result/Evidence/catalog/current-byte
and leading-contract parity, and verifies every current source/dependency
SHA-256. It rejects a
missing or fabricated result, consistently reduced case sets, incomplete
provenance, a stale PASS result reattached to fresh evidence, stale bytes,
non-PASS results, and cross-contract drift. Corrections
without a declared publication matrix omit both optional artifacts and remain
compatible.

Historical BL-333/BL-334 packages without explicit discriminators remain valid
only through the isolated legacy compatibility path. New correction packages
declare the correction profile explicitly. Mixing generic and correction-only
members or typed correction fields fails closed. Ordinary patch, report,
backlog, changelog, or documentation text may name historical findings or
correction artifacts without changing the profile; narrative tokens are never
used as profile discriminators.

The generic commit-preparation profile contains exactly `HANDOFF.md`, `assignment-record.json`,
`completion-report.json`, `current-delta.patch`,
`independent-review-evidence.json`, `package-inventory.json`, `report.md`,
`scope-inventory.json`, `task.patch`, `validation-summary.json`, and the root
`MANIFEST.sha256`. It permits zero real findings and forbids correction
matrices, `correction-only.patch`, focused-finding records, fixed queues, and
BL-333/BL-334-specific status or path assumptions.

The evidence-only and post-merge profiles use the same generator, directory
validator, inventory, manifest, ordinal ordering, strict UTF-8, single-write,
and reopen lifecycle. Their payloads omit `task.patch` and
`current-delta.patch`; both hashes are JSON `null` and the authoritative scope
must be empty. Each evidence binding names a unique canonical package member
whose real non-link bytes match the declared SHA-256. Evidence-only therefore
has ten members and exactly one `EXTERNAL_READ_ONLY` artifact. Post-merge has
eleven members and exactly one real `MERGE_STATE` plus one real
`LIVE_EXTERNAL_READBACK` artifact, plus
`matrixDisposition=UNCHANGED_MATRICES_NOT_RUN`.

The correction profile retains the existing correction/current-delta,
finding-matrix, regression-matrix, focused-record, external-delta, trusted-hash,
readiness, status, queue, and parity gates without weakening them. Its
productive validator parses the embedded report contract and requires parity
with assignment, completion, correction matrix, regression matrix, focused
record, ledger, previous-review binding, readiness, HANDOFF, actual patch
bytes, and Git-derived scope/patch inventories.

## Patch and scope requirements

The patch must:

- use unified Git format and the declared baseline;
- include tracked changes, intended untracked source, modes, renames, and
  binary-safe data when applicable;
- exclude unintended generated output and secrets;
- parse and preferably pass `git apply --check` against an isolated baseline.

The inventory records repository, baseline, branch, HEAD, complete relevant Git
status, path, tracked/staged state, mode, byte size, SHA-256, inclusion decision,
and nonempty inclusion or exclusion reason. It separately declares allowed
delta paths and explicitly excluded task-unrelated paths. Assignment, completion
report, independent-review evidence, task/current-delta patches, generated
HANDOFF, and report contract bind the same repository, baseline, current commit,
branch, scope-inventory SHA-256, patch SHA-256 values, allowed paths, and
excluded paths. Every target path has `staged=false` before commit approval.
Repository status, report paths, patch paths, inventory paths, and manifest
paths must agree; any stale or mismatched field fails closed.

## Manifest and package validation

The root `MANIFEST.sha256` is ordinally sorted and covers every payload file
exactly once while excluding itself. Each line contains lowercase SHA-256,
decimal byte size, and safe relative path. The final ZIP is reopened and
validated independently. Absolute/traversing ZIP paths, duplicate entries,
case-colliding paths, symlink/junction/reparse entries or source targets,
invalid UTF-8, control characters, unresolved placeholders, secret material,
and unintended host/user paths fail closed.

Host-path validation is artifact-specific and classification-driven. Every
scanned artifact is either declared host-path-free or has exact structured
references classified as `CANONICAL_INFRASTRUCTURE`, `DOCUMENTED_EXAMPLE`, or
`SYNTHETIC_FIXTURE`. Private Windows user-profile paths, private unapproved UNC
paths, `/home/<name>`, `/Users/<name>`, `/tmp`, undeclared absolute host paths,
unused allowlist entries, and wildcard-style allowances fail closed. Declared
canonical, documented example, and synthetic fixture paths are permitted only
in the bound artifact. A package-wide narrative text regex is not profile or
host-path classification evidence.

Any payload-byte, required-file, path, manifest, readiness, or instruction
change invalidates the previous package. Rebuild the complete package from a
fresh staging root, regenerate the full manifest, reopen the resulting ZIP,
and validate every entry hash and length. Incremental archive updates and a
manifest carried forward from older payload bytes are forbidden. Extra
unmanifested objects and missing required objects fail closed.

The authoritative external artifact gate is:

```text
C:\Users\ThomasW\OneDrive - VOXTRONIC\Desktop\Voxtronic\Codex-Work\Scripts\Test-ClassicReviewArtifact.ps1
```

Binding invocation under PowerShell 7.6.5:

```powershell
& {
    $result = & 'C:\Users\ThomasW\OneDrive - VOXTRONIC\Desktop\Voxtronic\Codex-Work\Scripts\Test-ClassicReviewArtifact.ps1' `
        -ArtifactPath '<absolute-package-path>' `
        -ReadinessRequirement RequireTrue
    $exitCode = $LASTEXITCODE
}
```

Exit code `0` is PASS, `1` is a content or readiness failure, and `2` is an
invocation or infrastructure error. The validator path and SHA-256 used by the
producer are recorded in the assignment and completion records. The repository
validator is a prerequisite static/semantic gate; it is not an alternative to
this authoritative external artifact gate.

`HANDOFF.md` contains exactly one visible status block between
`<!-- BEGIN GOVERNANCE-HANDOFF-STATUS -->` and
`<!-- END GOVERNANCE-HANDOFF-STATUS -->`, followed by exactly one strict JSON
contract between `<!-- BEGIN GOVERNANCE-HANDOFF-CONTRACT -->` and
`<!-- END GOVERNANCE-HANDOFF-CONTRACT -->`. The productive handoff producer
generates both blocks from one typed status source.

For the correction profile, the validator counts each of the four marker lines independently and requires
exactly one of each in strict status-before-contract order. The visible block
contains exactly these case-sensitive keys, once each and in canonical order:
`Status`, `CorrectionMode`, `TargetFindingCount`, `CorrectedFindingCount`,
`PendingDeltaFindingCount`, `ClosedFindingCount`, `OpenFindingCount`,
`ClassicReviewReady`, `TargetFindings`, `PendingFindings`, `ClosedFindings`,
`Run007Status`, `CommitPreparationApproved`, `CommitAuthorized`,
`RequiredReviewMode`, and `NextAction`. Unknown keys, duplicate keys, empty or
mistyped values, non-canonical lists, and reserved `Key: value` control lines
outside the visible block fail closed. Every visible value is parsed by type
and compared exactly with the JSON field or its bounded array-derived count;
substring checks are not parity evidence. The generic profile instead uses the
ordered visible keys `TaskId`, `TransitionType`, `Profile`, `Status`,
`ClassicReviewReady`, `FindingCount`, `ReviewStatus`, `CommitAuthorized`,
`AllowedDeltaPaths`, and `NextAction`; every value must equal the strict JSON
contract generated from the same typed source.

For `FINDING_CORRECTION`, every finding-bearing package contract is also
validated per finding rather than only by overall finding or regression sets.
Severity, previous/current status, disposition, correction text,
affected/correction paths, permanent regression IDs, evidence references, and
direct producer/reviewer disposition mappings must remain exactly equal across
the correction matrix, regression matrix, ledger, and embedded report. A
union-preserving reassignment of evidence or regression IDs between findings is
invalid.

The JSON contract is validated by
`Governance/governance-handoff-contract.schema.json` and must agree with the
assignment, completion report, correction and regression matrices, focused
record, readiness evidence, report contract, validation summary, Local Work
Register delta, and every parsed visible handoff value. Status parity is true
only when the contract, visible-key, visible-value, independent-marker,
reserved-control-line, finding, count, queue, and package-contract gates all
pass. Its discriminated schema keeps the historical correction contract intact
and defines the task-neutral generic contract separately.

## ClassicReviewReady boundary

`ClassicReviewReady=true` is permitted only when:

- every transition-required artifact exists;
- the actual correction/current-delta entry bytes match separately supplied
  expected hashes and every manifest and record declaration;
- patch and inventory scope agree exactly with assignment, focused record,
  completion report, correction matrix, report contract, and repository status;
- all current correction/review semantic contracts contain the same unique
  target findings, per-finding
  status/evidence, direct interfaces, regression tests, and no unknown fields;
- external manifest, payloads, completion report, report contract, and
  narrative path section contain exactly four canonical external paths and
  scopes with the required one-to-one mapping. Windows-normalized,
  case-insensitive duplicate paths, scope swaps, missing/extra scopes, and
  alternate spellings fail closed;
- manifest and ZIP reopen checks pass;
- the assignment governance record passes;
- the strict completion-report contract passes;
- every registered external change has a manifest-bound, hash-matching review
  delta;
- no open decision boundary or required artifact remains; and
- the package remains outside the active repository.

The finding-, external-governance-, correction-patch-, focused-record-, queue-,
and correction-status bullets apply only to `FINDING_CORRECTION`. For
`IMPLEMENTATION_TO_INDEPENDENT_FULL_REVIEW`, readiness requires exact parity
among the assignment, completion report, `pre-review-validation-evidence.json`,
task and current-delta patches, scope inventory, validation summary, package
inventory, manifest, and generic HANDOFF/report contracts. The pre-review
evidence must state `independentReviewStatus=NOT_PERFORMED`, must package every
required review input without a mutable external dependency, and may reuse only
hash-bound passing full-completion evidence without reexecution. For
`GENERIC_COMMIT_PREPARATION`, readiness instead requires exact parity among the
assignment, completion report, external independent-review evidence, task and
current-delta patches, scope inventory, validation summary, package inventory,
manifest, and generic HANDOFF/report contracts. The generic profile always
requires `commitAuthorized=false` and never treats Classic readiness as commit
authorization.

For `EVIDENCE_ONLY_FOCUSED_REVIEW` and `POST_MERGE_CLOSURE`, readiness replaces
patch parity with exact empty-delta parity and the profile-specific packaged
evidence bytes above. Missing, hash-drifted, tampered, duplicate/case-colliding
evidence, artificial patch members, nonempty authoritative status, stale
merge/readback bindings, or any profile/transition mismatch fail closed before
the sole package-write attempt.

The generic implementation-to-review and commit-preparation payloads each have
exactly eleven members. They differ by one evidence member:
`pre-review-validation-evidence.json` is mandatory and
`independent-review-evidence.json` is forbidden for the pre-review profile; the
inverse applies to commit preparation. Profile and transition are explicit
discriminators and are never inferred from missing fields or narrative text.

The generic scope inventory is not self-authenticating. Before readiness, the
validator must resolve the trusted isolated worktree and verify its repository
Origin, the bound baseline object, exact current HEAD and branch, and the
complete relevant Git status. Every INCLUDE and EXCLUDE entry must match that
authoritative state for path, status class, tracked/staged state, mode, byte
length, and SHA-256. INCLUDE entries equal the patch path set; EXCLUDE entries
must be absent from the patch. A semantic negative fixture is valid evidence
only after every dependent scope hash, typed contract, embedded HANDOFF/report
contract, package inventory, and manifest has been regenerated and the
intended named gate is the observed failure.

Generic scope entries use disjoint `gitStatus` forms:

- ordinary tracked postimages require current `path`, Git-derived `mode`,
  `modeSource=GIT_WORKTREE`, byte length, and SHA-256;
- untracked regular files require current bytes and an explicit platform mode
  classification: `WINDOWS_REGULAR_FILE_NORMALIZED` with `100644`, or
  `UNIX_EXECUTABLE_BIT_NORMALIZED` with normalized `100644`/`100755`;
- `TRACKED_DELETED` requires a binary-safe baseline `preimage` with commit,
  `BASELINE_TREE` mode source, Git mode, byte length, and SHA-256, plus
  `postimageAbsent=true`;
- `TRACKED_RENAMED` requires distinct `previousPath` and `path`, binds the
  baseline preimage to `previousPath`, and binds the current postimage to
  `path`.

The real NUL-separated Porcelain-v2 status remains complete evidence and every
entry remains `staged=false`. Because an unstaged filesystem rename may appear
there as a delete plus an untracked target, the validator may additionally use
a temporary alternate index with object-isolated source semantics and
`git diff --name-status -z --find-renames` to pair source and target. That
temporary index never replaces or mutates the real index. Authoritative patch
generation uses the bound baseline in another temporary index, includes both
rename paths, preserves binary bytes, and must equal `task.patch` and
`current-delta.patch` byte-for-byte. Baseline identity is validated before any
baseline-dependent plumbing command.

Every temporary-index operation that can create Git objects also uses a unique
temporary `GIT_OBJECT_DIRECTORY`. Its `info/alternates` contains exactly one
canonical absolute binding to the real object directory resolved through the
worktree's Git common directory. Every filesystem child below the object
directory is constructed component by component; a compound child literal
with a platform-specific separator is prohibited in both productive evidence
and its fixtures. New blobs and trees are written only to that temporary
database. A deterministic inventory of every real object-directory
file and directory, including loose objects, pack/index files, commit graphs,
multi-pack indexes, and `info` content when present, must be byte-identical
before and after evidence generation. Temporary index and object directories
are removed in `finally`; inventory divergence or cleanup failure fails
`GENERIC-REAL-OBJECT-DATABASE-IMMUTABILITY`.

All path-bound Git plumbing receives `GIT_LITERAL_PATHSPECS=1` and passes each
validated repository-relative path as a separate argument after `--`. The
isolated index stages only the exact INCLUDE path set. Its NUL-separated actual
`--name-status --find-renames` inventory is parsed fail-closed and must match
the declared INCLUDE entries exactly, including deletion and rename tuples.
No actual or package-patch delta may contain an EXCLUDE path. These contracts
are enforced by `GENERIC-LITERAL-PATHSPEC-BINDING`,
`GENERIC-ACTUAL-DELTA-INVENTORY-PARITY`, and
`GENERIC-EXCLUDED-DELTA-PATH-PROHIBITION`.

Changes to this cross-platform Git-evidence helper require real end-to-end
execution of the generic fixture matrix on Windows and native Linux under the
project PowerShell version. A platform-gated synthetic result for the opposite
operating system does not replace that platform's native run.

False, absent, mistyped, or conflicting readiness is never promoted to true.
An instruction to transfer individual members of a multi-file package also
fails readiness, even if the ZIP itself is technically valid.

## Required report fields

- `HandoffRequired`
- `HandoffPackage`
- `HandoffArtifacts`
- `ScopeInventoryResult`
- `PatchCompletenessResult`
- `HandoffManifestResult`
- `HostedCISourceVerification`
- `MissingHandoffArtifacts`
- `ClassicReviewReady`

`NextAction` must not instruct the user to pass an artifact that the producing
assignment did not create.
