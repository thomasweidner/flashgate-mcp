# Testing

## Governance enforcement

BL-333 supplies the change-trigger, finding-remediation/review-mode, and
handoff-readiness foundation. BL-334 enforces it through
`scripts/Test-GovernanceConsistency.ps1`.

Parse every new or changed PowerShell source with PowerShell 7.6.4 before its
first execution. Then run:

```powershell
& {
    .\scripts\Test-GovernanceConsistency.ps1
    .\scripts\Test-GovernanceCaseSelectionFixtures.ps1
    .\scripts\Test-GovernanceConsistencyFixtures.ps1
    .\scripts\Test-GenericGovernanceHandoffFixtures.ps1
    .\scripts\Test-ImplementationReviewHandoffFixtures.ps1
    .\scripts\Test-DocumentationConsistency.ps1
}
```

ADR-0016 and BL-337 through BL-340 define the governance-harness layer.

### Governance validation orchestration profiles

BL-339 uses `scripts/Invoke-GovernanceValidation.ps1` and the strict
request/result schemas in `Governance/governance-validation-*.schema.json`.
Exactly seven permanent profiles are cataloged:

1. `documentation-registration`;
2. `governance-schema-change`;
3. `fixture-harness-change`;
4. `finding-correction`;
5. `commit-preparation`;
6. `focused-revalidation`;
7. `full-completion`.

### Canonical governance case selection

BL-338 stores the complete ordered 277-case inventory in
`Governance/governance-case-metadata.json`; its closed JSON schema is
`Governance/governance-case-metadata.schema.json`. The shared
`scripts/GovernanceCaseSelection.psm1` reader rejects unreadable, malformed,
non-UTF-8, unknown-property, duplicate, unordered, platform-inconsistent,
capability-incomplete, and unresolvable dependency metadata before any case
execution. The canonicalized catalog and resolved case objects receive
independent semantic SHA-256 bindings.

`scripts/Test-GovernanceConsistencyFixtures.ps1` exposes `-ListGroups`,
`-ListTags`, `-ListCases`, `-CaseName`, `-Group`, and `-Tag`. Exactly one
selector class may be active; multiple values within that class retain the
existing semantics (CaseName/Group union, Tag conjunction). Unknown,
duplicate, mixed-class, zero-result, platform-incompatible, or
capability-incomplete requests return structured diagnostics with
`RunnerProcessStartCount=0` and `ValidationExecutionCount=0`. The executable
descriptors contain execution inputs only: their IDs are assigned directly
from the canonical ordered inventory, while supplemental routes consume
ordered metadata subsets rather than a second maintained CaseId list. The
repository harness has no runtime dependency on Codex-Work or another
contributor-local infrastructure tree. It binds platform and capabilities
fail-closed from explicit validated caller inputs or deterministic local
process discovery. A local Codex caller applies the INF-160
`pwsh-governance`/`git-local-readonly` routes at its own boundary and may pass
the selected executable paths; Hosted CI uses only repository files and its
prepared PowerShell/Git runtime.

`scripts/Test-GovernanceCaseSelectionFixtures.ps1` is the permanent focused
38-case matrix for list, selector, metadata-schema, deterministic hash/order,
mixed-valid/invalid, platform, capability, and zero-execution behavior.
`scripts/Test-GovernanceHostedCiPortabilityFixtures.ps1` adds repository-only
regressions for all list operations, a canonical case selection and preflight,
invalid-selector zero execution, the absence of hard-coded contributor paths,
and the actual CI invocation contract.
BL-337 later consumes the single resolved set; it does not reselect. BL-340
remains responsible for generator/profile migration and does not own this
metadata source.

Every profile runs the same fail-fast cheap sequence before any subordinate
matrix result is read: PowerShell parser/syntax; `.gitattributes` and
`.editorconfig` EOL/UTF-8/trailing-whitespace/single-final-newline rules;
`git diff --check`; required external and ignored input existence/hash binding;
PowerShell/Git/platform/execution-context binding; then explicit source
repository, isolated worktree, HEAD, tree, file-hash and dynamic selector
binding. A failed gate stops the sequence and later expensive stages remain
`NOT_RUN`.

Source binding hashes the raw bytes produced by exactly
`git status --porcelain=v2 --untracked-files=all` and compares them with the
request's `expectedStatusSha256`. The result records expected and actual hashes.
The scope and file-hash path sets must be exactly equal and reject duplicates or
Windows case collisions. Explicit protected-worktree bindings independently
verify root, HEAD, tree, branch/detached state, and raw status hash.

The request carries task data, not generated controller code. Subordinate
results are strict UTF-8 and schema/version/profile/hash bound before use.
Unchanged PASS evidence is reused only when every declared dependency hash
matches; changed source, toolchain, selector, or other dependency invalidates
only its dependants. The result reports `validationExecutionCount`,
`infrastructureOrInvocationFailureCount`, `fullMatrixRunCount`,
`packageWriteAttemptCount`, `generatedTaskControllerFileCount`,
`generatedTaskControllerLineCount`, and `readOnlyProbeCount`. `BLOCKED` is not a
technical failure, and only `full-completion` may record one full-matrix PASS.

Generic handoff tests validate the staging directory before one same-parent
candidate write, product-validate the closed candidate, and atomically publish
the same identity-bound object by hard link without overwrite. Directory,
serialization, validation, candidate mutation/replacement, race, publication,
and pre-publication interruption failures leave the final path absent. The
write-attempt counter is set before the first write-capable candidate open; no
second attempt occurs. Post-publication interruption preserves the valid ZIP. BL-338 remains the
exclusive owner of dynamic case/group/tag/platform/capability listing and
selection; the BL-339 request only binds that interface and its hashes. BL-340
owns complete workflow-generator/profile migration, including current
`currentStateGate` generation and explicit historical schema-version-1 reads.

`Test-ImplementationReviewHandoffFixtures.ps1` is the focused permanent matrix
for the pre-review profile. It covers a valid directory and ZIP, absent prior
review evidence, full-completion reuse, explicit discriminator failures,
patch/scope failures, directory-first no-write behavior, exactly one write,
failed first opens against an existing file and a directory target, no retry,
reopen/manifest/SHA parity, readiness, and positive/negative compatibility with
`GENERIC_COMMIT_PREPARATION`. The profile requires
`pre-review-validation-evidence.json`; commit preparation still requires
`independent-review-evidence.json`.

`Test-FindingCorrectionHandoffFixtures.ps1` is the focused permanent matrix for
the productive correction-to-focused-review contract. It covers task-neutral
BL-339 and historical BL-333/BL-334 finding IDs, commit and immutable-package
previous states, mutual exclusion and incomplete snapshot negatives, exact
finding/path parity, generator discriminators, ZIP-free preflight semantics,
the complete embedded report contract, and compatibility with both generic
profiles. The productive
`Test-FindingCorrectionHandoff.ps1` additionally reconstructs the historical
tree, verifies per-path postimages, applies both patches in isolated Git object
and index storage, parses the report JSON against
`finding-correction-report-contract.schema.json`, and validates report parity,

The focused matrix additionally includes a valid two-finding contract and an
adversarial union-preserving regression-ID swap. Product validation must reject
that swap even though the aggregate test-ID union is unchanged. Independent
negative cases cover per-finding severity, previous/current status,
disposition, correction text, affected paths, evidence references, and
producer/reviewer status so that cross-contract parity is functional evidence,
not schema-presence or substring coverage.
directory inventory, and manifest coverage.
The matrix is execution-based rather than source-presence-based: it runs the
productive generator and validator for preflight, final-content directory,
synthetic final ZIP and reopen, commit and immutable-package previous states,
historical composite task IDs, unchanged reference-only paths, foreign-task
finding mutation across all seven finding-bearing contracts, and modification/add/delete/mode/
unchanged-rename/modified-rename inventory. Targeted negatives cover lifecycle,
mixed/missing previous state, reference duplicates/case collisions/overlap,
foreign correction paths, historical declared entry counts, historical rename
source/target patch-inventory parity, and rename/preimage/postimage binding. Case count is
derived from the executed result and is not fixed in governance. Permanent
report cases cover complete preflight and final reports, missing contract,
wrong current/correction patch hashes, missing/foreign report findings, wrong
previous binding, cross-lifecycle report substitution, subject-data
preservation, and synthetic final-ZIP reopen with the full contract.

`Test-GovernanceHandoffPublicationFixtures.ps1` covers all producer profiles
through the shared publication primitive. Explicit cross-process handshakes
cover interruption during validation, after validation, and after publication;
separate drift processes actually mutate and replace a validated candidate.
CreateNew, serialization, validator, wrong-parent, race, and publication-I/O
failures remain deterministic product cases.

For a finding that declares this separate matrix, the calling process captures
a canonical Expected Execution Input Binding before child/runner start. It
contains the exact catalog, runner, and complete dependency SHA-256 values, and
the parent also supplies the SHA-256 of the binding artifact. Before every
module import, and again immediately before Case 1, the runner uses independent
.NET primitives to compare all current bytes with that parent binding. Each
case-record operation rechecks the same contract. Deterministic separate-process
handshakes mutate the runner and a loaded dependency both between parent binding
and runner start and between import and Case 1; each path fails with zero
executed or accepted PASS cases. The runner persists
`publication-regression-result.json`; its `results[]` is the leading source for
executed `GHP-*` IDs and PASS counts. Result V2 carries exactly the parent-bound
`executionInputBinding`.
Evidence V2 binds those exact result bytes and independently binds
`publication-regression-matrix-catalog.json`, whose fixed matrix definition
owns the complete required case, source, and dependency sets.
`Test-FindingCorrectionHandoffFixtures.ps1 -PublicationEvidenceOnly` exercises
the productive 21-member contract without rerunning the unchanged complete
finding-correction matrix. It rejects result absence/hash/JSON/count failures,
missing/foreign/consistently reduced cases, incomplete/extra/stale provenance,
matrix-definition drift, stale PASS results paired with fresh evidence after
source/dependency/catalog mutation, and leading-contract mismatch. Synthetic positive
result data is labelled as contract-fixture data and is never BL-339 execution
evidence. A correction without a declared publication matrix remains a
positive compatibility case.

The BL339-REV-001 focused source-binding matrix has 21 cases and covers exact
and empty status PASS, added tracked/untracked state, missing and status-changed
paths, content-hash drift, duplicate/case-colliding paths, scope/hash-set
mismatch, and an independently protected foreign worktree. The BL339-REV-002
handoff matrix has 19 cases and preserves the existing implementation-review
and commit-preparation compatibility coverage.
Their pre-execution contract resolves all selectors before process start,
binds an explicit source/worktree by branch, commit, tree, and file hashes, and
selects toolchain, platform, and execution context before mutation. Progress is
per-case and event-based: terminal cases have a terminal event, identical
events are suppressed, and explicitly typed heartbeats respect their interval.
`BLOCKED` is not a failure and does not increment `FailureCount`.
Before mutation, a current-state gate binds repository, baseline/current commit,
branch, complete relevant status, authorized scope, IDs, and parallel worktrees.
Toolchain, parser, parameter, Temp, sandbox, and harness preflights run before
expensive fixtures. A new or materially rebuilt validator starts from a written
failure-mode matrix covering success, rejection, timeout, process cleanup,
platform behavior, malformed evidence, and repository-mutation detection.

Validation is a funnel: reproduce/root-cause, validate directly affected
components, converge documentation, and perform exactly one complete final
governance/documentation run. A second full run requires a documented new root
cause. Resume/checkpoint data is valid only when immutable inputs, case
inventory, completed cases, outputs, and repository state are hash-bound.

### Progress, result, warning, and execution telemetry

`X/Y` means `X` completed units from `Y` selected units and always names the
unit. It never represents successful and failed results. Long-run events include
the phase, `ProgressUnit`, and separate counts for `PASS`, `FAIL`, `SKIPPED`,
`BLOCKED`, `CANCELLED`, `PENDING`, and `NOT_RUN`. Terminal completed states are
PASS, FAIL, SKIPPED, BLOCKED, and CANCELLED. PENDING and NOT_RUN are not
completed. While no terminal failure is known, intermediate text uses
“Bisher keine Fehler; Gesamtergebnis noch offen” rather than a success claim.
Status is emitted only for a phase, progress, warning, or result change.

Every governed validation summary and completion report separately records:

- `ObservedWarningCount`;
- `ResolvedWarningCount`;
- `OpenWarningCount`;
- `MaterialCorrectionCycleCount`;
- `ValidationExecutionCount`;
- `InfrastructureOrInvocationFailureCount`.

The invariant is `ObservedWarningCount = ResolvedWarningCount +
OpenWarningCount`. A recurring long run without numeric progress is an
instrumentation finding. The generic JSON contracts use lower-camel-case field
names with the same semantics.

The historical governance fixture matrix contains 225 result cases, and the
accepted BL-336 evidence contains 85 generic-handoff cases. Those values remain
unchanged historical evidence; neither is the leading count for a current full
run.
Long local runs may pass `-ProgressPath <new-jsonl-path>` to record fixture ID,
sequence, completion time, and result as JSONL. They may additionally pass
`-ResultPath <new-json-path>` to atomically persist the typed terminal result,
including the canonical fixture count, ordered metadata inventory, inventory
SHA-256, resolved selection and its metadata, progress record count, and
progress SHA-256 before normal process exit. Every canonical descriptor binds
its case ID, group, tags, supported platforms, required capabilities, and
Windows-only dependencies. The current governance runner derives its full
count and focused platform subset only from that source. `-Tag`,
`-TargetPlatform`, and platform capability inputs resolve before the first
fixture process. Local Codex orchestration consumes INF-160 executable,
version, hash, realpath, environment, owner, and working-root evidence before
calling the portable harness; repository and Hosted-CI execution do not import
INF-160 files. Other callers may provide already validated executable paths and
capabilities, or use the harness's deterministic process preflight.
Unknown, duplicate, ambiguous, platform-incompatible, or
capability-incomplete selections execute zero cases. Full and focused runs fail
closed unless the resolved selection and completed IDs match that inventory
exactly, none are unexpectedly skipped, cleanup succeeds, and repository status
is unchanged.
Historical 198- and 225-case results below remain evidence for their earlier
committed governance states rather than the current working-tree count.

Assignment-schema version 1 retains a compatibility bridge for pending records
emitted by the unchanged `New-GovernanceWorkflowRecord.ps1`: those records may
omit `currentStateGate` while every readiness field remains pending or false.
The gate remains typed and becomes mandatory when the record claims current
documentation/validation readiness, a required or ready handoff, or any
commit-preparation readiness transition. The explicit
`GENERIC_COMMIT_PREPARATION` profile continues to require the gate without an
exception. Positive completion fixtures receive all current telemetry, warning
invariant, ZIP-free readiness, and package-generation defaults from
`New-CompletionFixture`.

The generic matrix includes positive end-to-end packages for tracked deletion,
unchanged and content-modified renames, Windows-normalized and Unix-normalized
untracked modes, and their closed negative matrix. Delete preimages come from
the exact baseline blob; rename preimages bind `previousPath`, postimages bind
`path`, and both rename paths participate in binary patch parity. Semantic
negative packages regenerate every dependent contract, inventory, and manifest
and must fail their named check exactly once. A restored deletion is rejected
at `GENERIC-PATCH-SCOPE-PARITY` before later authoritative-scope binding.

The Git-evidence matrix additionally proves that every temporary index that can
write objects uses an isolated temporary object database with an exact
read-only alternate to the real object directory. Canonical real-object
inventories must remain identical before and after the operation, new fixture
blobs must occur only in the temporary database, and all temporary index/object
artifacts must be removed. Literal pathspec packages cover `docs/[a].md` and a
leading `!` filename while similarly matching paths remain explicit EXCLUDE.
The actual NUL-separated delta inventory and the package-patch inventory must
equal INCLUDE exactly; EXCLUDE leaks, missing INCLUDE entries, invalid literal
environment, bad alternates, leftover temp state, inventory divergence,
unknown or duplicate status records, and CR/LF/NUL paths fail their named gate.
The `info/alternates` and fixture-only object-divergence paths are constructed
component by component so the same child hierarchy is exercised on Windows and
Unix filesystems. A change to this shared Git-evidence helper requires the full
generic fixture matrix under PowerShell 7.6.4 on Windows and on a native Linux
copy below `/home`; the real Unix executable package must run on Linux, and a
platform-gated synthetic result does not satisfy that end-to-end gate.

`scripts/Test-ClassicReviewArtifact.ps1` is the versioned Hosted-CI execution
mirror of the external canonical Classic artifact validator. Its integration
baseline is 32719 bytes with SHA-256
`c6da4524c881d339bdccfde5894277ddc549fae898348e538956302949518c51`.
The mirror is not an independent governance source. The fixture runner resolves
it relative to its own script by default and still accepts an explicit
`-CanonicalArtifactValidatorPath`. When the external canonical validator exists
locally, the runner compares both files byte-for-byte by SHA-256 and fails
before creating fixtures on any divergence. Hosted Windows CI explicitly passes
the repository mirror and therefore requires no contributor-local path.

The dedicated Windows governance job selects
`github.event.pull_request.head.sha` for pull requests and `github.sha` for
pushes, checks out that exact commit, and logs the expected head, checked-out
commit, current commit, and validator expectations. Pull-request governance
therefore validates the exact PR head rather than GitHub's synthetic merge
commit. Because a pull-request workflow can be loaded through a different event
commit, the job separately compares the `.github/workflows/ci.yml` Git blob at
`github.workflow_sha` with the exact-head blob and binds workflow provenance to
the head only after byte parity passes. The Windows/Linux Go matrix retains its
normal merge checkout and continues to validate the prospective merge result.

The governance job downloads
`PowerShell-7.6.4-win-x64.zip` only from the official versioned PowerShell
release URL and verifies SHA-256
`80832551C52809301E6071C8BAC977BEB5A2F1EC953EB4DB9F94DEB953333793`
before extraction. After the hash, extraction, and executable-existence gates
pass, it publishes only that extraction directory through `GITHUB_PATH` and
uses the static `shell: pwsh`. The governance step compares the running
PowerShell process path with the expected hash-gated executable path using
explicit ordinal, case-insensitive equality after both absolute paths are
normalized. Casing-only differences are accepted; different drives,
directories, subdirectories, filenames, relative paths, and empty values
remain fail-closed, with no global installation, latest-version resolution, or
fallback. The productive validator gates the exact `7.6.4` interpreter and
downloaded package digest; the fixture runner also requires 7.6.4 and launches
child validators from its current `$PSHOME`.

The fixture matrix exercises the production validator with positive and
negative canonical paths, every immutable mode flag, real tracked domains,
the immutable 12/6/0 remediation, productive-mutation, final-activity-gate,
and single-package policies, same-run and deferred-finding bindings,
byte-exact correction/current-delta
hashes, strict focused-delta and finding matrices, complete per-finding
completion parity, exact narrative/report/repository/external/status sets,
the five canonical external path-to-scope mappings, the strict bounded
`HANDOFF.md` JSON contract, its exactly-16-key typed visible status block,
independent counts for all four status/contract markers, rejection of reserved
control lines outside the visible block, and complete visible/JSON parity,
actual valid and re-manifested corrupt ZIPs, external before/after payloads,
complete tracked-path coverage, and the real CI/release parameter binding.
Negative packages alter bytes, self-reported hashes, findings, evidence, paths,
interfaces, report and handoff blocks, external path/scope mappings, status,
counts, duplicate/unknown visible keys, independently added or reversed
markers, reserved outside-block lines, and queue through the same productive
validator path. Each re-manifested negative case must fail its expected
specific productive gate. A workflow checkpoint
record passes only with an authoritative
immutable repository, commit, event, ref, run, and Hosted CI source identity.
Focused runtime fixtures additionally prove that a wrong PowerShell version and
a wrong package digest fail their specific productive checks. The real-CI
workflow-binding fixture also semantically requires the post-verification
`GITHUB_PATH` publication, static governance shell, expected path binding, and
expected/actual process-path diagnostics. In-memory negative variants prove
that removing either binding, restoring the obsolete dynamic shell, or using a
case-sensitive path comparison fails closed. The same focused check accepts
the Hosted casing-only `.exe`/`.EXE` variant and rejects changed directories
and filenames without adding permanent fixture cases.

Classic handoff policy fixtures additionally accept one direct file, accept
one complete multi-file ZIP, verify manifest SHA-256/length coverage and a
fresh full rebuild after payload change, and reject missing files, stale
manifests, wrong hashes, exact or case-colliding ZIP paths, absolute/traversing
paths, link/junction/reparse entries, unmanifested objects, incomplete
`ClassicReviewReady=true`, and instructions to transfer package members
separately.

Current fixture counts are reported by their owning runner and current evidence;
the primary governance runner also reports the canonical ordered inventory and
its SHA-256. No separately maintained numeric constant is authoritative.
Historical 225-case BL-333/BL-334 and 85-case BL-336 evidence remains unchanged.
Generic cases cover
finding-free BL-230 commit preparation, a real finding, external independent
review, both readiness states, successful Classic readiness with commit still
unauthorized, narrative historical-ID and correction-artifact mentions,
missing/unknown/mixed profiles, actual correction-only members and typed fields,
complete repository/baseline/HEAD/branch/status/tracked/staged/mode/length/hash/
inclusion scope binding, additional delta paths, classified canonical/example/
synthetic host paths, private Windows/UNC/Linux/macOS/temp path leaks, undeclared
absolute Windows and Unix paths, and the closed typed runtime factory for all
seven negative host-path classes; review
drift, unauthorized commit, unknown JSON fields, invalid UTF-8, and ZIP/
inventory/manifest divergence. `-CaseName` resolves against the same canonical
primary inventory, rejects unknown or duplicate IDs before execution, and does
not redefine the full inventory.

The generic validator receives the schema repository and the authoritative
isolated worktree as distinct roots. It verifies the trusted Origin, existence
of the bound baseline commit, exact current HEAD and branch, and the complete
Porcelain-v2 changed/untracked set. Each scope entry is then compared to the
worktree for status class, tracked/staged state, Git mode, byte length, and
SHA-256. INCLUDE entries must exactly match the patch paths; EXCLUDE entries
must be relevant Git-status paths and absent from the patch. The semantic
baseline, current-commit, branch, repository, staged, tracked, length, mode,
and hash negatives regenerate every dependent scope hash, typed JSON record,
embedded HANDOFF/report contract, package inventory, and manifest before
asserting one named failure gate rather than accepting an arbitrary nonzero
exit.

PR #27 merged through `e42d57d57ea075640c9b123a533057bcac3861b8`.
The merge has first parent `537ea1c1660cddfde5aace1888242d80a6be77bf`,
second parent `c9b54c9be0cc96d9fc7f81841e28dc7a9b89fc74`, identical merge/head
trees, all six PR commits, and exact 26/26 path parity. Post-merge CI,
Metadata Regression, and CodeQL completed successfully. The governance job
bound exact head and workflow-source parity to the merge commit, used
PowerShell 7.6.4, completed 1,051 checks with zero errors, and ran 198/198
fixtures successfully. All BL-333/BL-334 and PR27-EXACT findings are closed.

The post-merge finalization changes only `BACKLOG.md`, `CHANGELOG.md`,
`docs/testing.md`, and `docs/documentation-quality-gate.md`. Validator,
workflow, and fixture bytes are unchanged. Therefore the local 198-case run is
reused from the exact post-merge Hosted evidence instead of being redundantly
repeated; the Finalization PR must run the complete Hosted matrix again.
This reuse contract does not waive productive governance or documentation
consistency checks and introduces no runtime, product, INF-121, or BL-335
change.

FlashGate MCP uses Go's standard testing framework and the `flashgate-mcp` binary.

The project aims for high test coverage in security-sensitive and filesystem-related code.

## Test Commands

Run all tests:

```bash
go test ./...
```

Run tests with the race detector:

```bash
go test -race ./...
```

The authoritative race gate runs on a platform with a supported race toolchain.
The current Windows host has no CGO/GCC race toolchain, so Windows race is reported
as an infrastructure limitation rather than worked around. Native Linux
`go test -race ./...` is required. Functional serialization, payload, fixture, and
budget-contract tests remain active under race; only the `testing.AllocsPerRun`
budget assertion is skipped because race instrumentation changes allocation
behavior. Ordinary non-race tests continue to enforce the unchanged allocation
budgets.

Run tests for a specific package:

```bash
go test ./internal/fs
```

Run focused MCP protocol tests:

```bash
go test -v ./internal/protocol ./internal/mcp/server ./internal/mcp/router ./internal/mcp/tools ./internal/mcp/initialize
```

Run tests with coverage:

```bash
go test -cover ./...
```

### Shell script validation

BL-251 validates the complete repository shell-entry-point inventory rather
than a fixed file list. The Windows gate requires PowerShell 7.6.4 and
`C:\Program Files\Git\bin\bash.exe`, parses every `.ps1` and `.psm1`, runs
Git Bash syntax checks for every `.sh`, validates strict UTF-8, line endings,
final newlines and supported Bash shebangs, and proves that validation did not
change repository status or script bytes:

```powershell
& {
    .\scripts\Test-ShellScripts.ps1
    .\scripts\Test-ShellScripts.Tests.ps1
}
```

The native Ubuntu gate uses only `/usr/bin/bash` and native standard tools:

```bash
/usr/bin/bash scripts/test-shell-scripts.sh
/usr/bin/bash scripts/test-shell-scripts.tests.sh
```

The persistent negative matrices cover an empty inventory, missing files,
invalid PowerShell and Bash syntax, wrong runtime paths or versions, invalid
repository roots, paths with spaces, stable inventory ordering, mutation
detection, bounded subprocess exit and timeout behavior, unconfirmed
termination without an unbounded stream wait, deterministic failure
classification, cleanup failure reporting, and a symlink shell entry point.
The bounded-process result records the PID immediately after a successful
process start. Its timeout regression validates the positive direct PID,
exit code 124, confirmed tree termination, absence of that concrete process,
and the case where timeout occurs before an optional child-authored PID file.
The PowerShell matrix currently passes 21/21 cases. The Bash cleanup-negative
probe uses only its task-local fixture root and requires `Status: FAIL`,
`Cleanup: FAIL`, a nonzero failure count, a nonzero exit code, and exactly one
terminal status block. CI
runs both Windows commands after binding the verified PowerShell 7.6.4
runtime, and both native Bash commands on Ubuntu. PSScriptAnalyzer and
ShellCheck remain optional local enrichments: when they are unavailable and
installation is not authorized, the parser, native syntax checks, structural
policy checks, and positive/negative harnesses are the approved deterministic
alternative. Script changes still require the controlled native Linux gate
below before completion.

## Required Quality Checks

Before committing, run:

```bash
go fmt ./...
go vet ./...
go test ./...
golangci-lint run
go build -o build/flashgate-mcp ./cmd/server
```

Run the applicable Windows or native Linux shell-validation pair above when
PowerShell, Bash, CI, build, release, smoke, or validation scripts change.

On Windows, the build command is usually:

```powershell
go build -o build/flashgate-mcp.exe ./cmd/server
```

## Controlled native Linux validation

Windows remains the leading FlashGate development environment. WSL2/Linux is
only a native build, test, and validation environment. The canonical runner
copies the Windows source one way into a new ext4 test copy below `/home`; no
source is synchronized back, no native gate runs below `/mnt/c`, and no other
MCP project is activated by this workflow.

### Canonical entry points

Run PowerShell 7.6.4 through the Windows orchestrator:

```text
C:\Users\ThomasW\OneDrive - VOXTRONIC\Desktop\Voxtronic\Scripts\Invoke-FlashGateLinuxValidation.ps1
```

The orchestrator uses:

```text
Runner  /home/weidnerthomas/voxtronic/tools/mcp-linux-validation/mcp-linux-validate
Profile /home/weidnerthomas/voxtronic/tools/mcp-linux-validation/profiles/flashgate-mcp.json
```

`quick`, `security`, and `release` use class-specific companion profiles in the
same managed profile directory. All runner and profile paths are absolute; PATH
fallback is forbidden for the runner, managed Go toolchain, and managed
`golangci-lint`.

Example:

```powershell
& {
    $result = & "C:\Users\ThomasW\OneDrive - VOXTRONIC\Desktop\Voxtronic\Scripts\Invoke-FlashGateLinuxValidation.ps1" `
        -RunId "manual-validation-20260724-130500" `
        -ValidationClass "standard" `
        -ReportContext "Manual validation after filesystem changes"

    $result | Format-List
}
```

Run IDs must match `<context>-<YYYYMMDD-HHMMSS>` and contain only lowercase
ASCII letters, digits, and hyphens. Spaces, separators, `..`, shell
metacharacters, control characters, and existing validation, log, or snapshot
paths are rejected.

### Mandatory triggers

Native Linux validation is mandatory when a change affects:

- `cmd/**`, `internal/**`, any `*.go` or `*.sh` file, `go.mod`, or `go.sum`;
- `.golangci.yml`, `.gitattributes`, `.editorconfig`, or
  `.github/workflows/**`;
- the Linux validation runner, a FlashGate validation profile, or
  build/test/lint/smoke scripts;
- filesystem, path, permission, symlink, junction, or reparse-point behavior;
- build tags, `runtime.GOOS`, `runtime.GOARCH`, platform-specific processes,
  signals, or cleanup;
- Go, linter, or build-tool versions;
- central security or platform boundaries, CI, release behavior, or completion
  of a platform-relevant sprint or PR diff.

It is not automatic for spelling, non-technical planning, or purely editorial
changes without a technical instruction or runtime effect. The gate remains
available for such changes.

### Validation classes

| Class | Scope | Closure use |
|---|---|---|
| `quick` | `go test ./...` and `go vet ./...` using the managed toolchain | Local intermediate check only; never replaces a commit, sprint, PR, or release gate |
| `standard` | Environment and package discovery, module download, formatting, vet, tests, managed lint, and native build | Normal technical sprint and PR validation |
| `security` | `standard` plus existing JSON-RPC, read-only, negative, startup-negative, and native validation-safety smokes | Security-, filesystem-, path-, or platform-boundary changes |
| `release` | `standard` plus existing build-input, repository-build, protocol, negative, and startup smokes | Release preparation; it does not publish a release |

The runner validates copy integrity, permissions, line endings, the managed
linter identity, expected artifacts, unexpected changes, and an unchanged
Windows source before returning a result.

The tracked FlashGate source baseline expects zero `MIXED` and zero
`NO_FINAL_NEWLINE` classifications. Intentional LF or CRLF files retain their
established style; corrections are limited to terminator normalization or one
matching final newline and must preserve encoding, BOM state, text, and
whitespace.

### Coverage-aware rules

Changes to executable Go production logic must add or update tests for relevant
success, failure, boundary, security-rejection, platform, resource, and cleanup
paths in the same implementation or correction run. Run targeted coverage for
the directly affected packages first.

Run the complete project coverage matrix only after a coherent implementation
or finding-correction package, for central/shared production logic, when a
minimum-threshold regression is possible, before the final committed PR review,
or when targeted coverage exposes additional risk. The current Windows and
Linux minimums remain authoritative in `.github/workflows/ci.yml`.

PowerShell, Python, and Bash components without percentage coverage use a
documented branch matrix covering success, input validation, failures, timeout,
protection, cleanup/finally, negative security, and platform/context paths.

### Results and blocking

- `PASS` requires successful copy, permission, line-ending, command, artifact,
  unexpected-change, and Windows-source protection gates.
- `PASS_WITH_WARNINGS` is allowed only for individually documented,
  non-blocking warnings such as one controlled `0x800705b4` retry, an optional
  Codex gate failure, or a non-functional runtime deviation. Sprint closure
  requires an explicit justification.
- `FAIL` blocks sprint, integration, commit, PR, and release closure. Build,
  test, vet, lint, coverage, integrity, source-protection, `/mnt`, result-schema,
  or infrastructure-control failures are blocking. A technical test failure is
  not retried automatically.

The orchestrator reads and validates the native `result.json`, binds status to
the runner exit code, and stops Ubuntu only when it started the distribution.
It never deletes validation copies.

### Codex, retention, reporting, and review

A WSL Codex read-only smoke is an additional gate for changes to `AGENTS.md`,
the validation runner or profiles, sandbox/path/context rules, unclear
Linux-specific errors, or an explicit request. It is not required for every
ordinary run. A token, capacity, or external-service failure is non-blocking
only when Codex was optional and all local technical gates passed.

Before that smoke, resolve the native binary below `/home` and require its
`codex-cli` version to match the active Windows version. Linux uses its own
configuration and authentication storage; a Windows binary or profile below
`/mnt/c` is never a substitute.

Validation copies are classified as `REFERENCE_BASELINE`,
`ACTIVE_VALIDATION`, `FAILED_VALIDATION`, or `DISPOSABLE_VALIDATION`.
References and failed runs are retained; successful normal runs may be marked
as later cleanup candidates. No copy is deleted automatically. Any later
cleanup requires an exact allowlist path, ignore/tracked/untracked and reparse
checks, preserved evidence, and a bounded dry run.

When Linux validation is required, the completion report records:

```text
LinuxValidationRequired
LinuxValidationClass
LinuxValidationRunId
LinuxValidationResult
LinuxValidationPath
LinuxValidationDuration
LinuxWarningCount
LinuxFailureCount
WindowsSourceChanged
UnexpectedChanges
CoverageRequired
TargetedCoverageResult
FullCoverageMatrixRequired
FullCoverageMatrixResult
ReviewType
ReportOrResultPath
SprintClosureAllowed
```

Use exactly one full independent review for a larger implementation or
integration state. Bundle Blocker and Major corrections, then review only the
changed delta, regression tests, coverage impact, and directly affected
interfaces. Do not repeat unchanged INF-095 or project-wide matrices without a
concrete technical reason. The next full review is performed on the final
committed PR state after successful hosted CI.

## Test Strategy

### Unit Tests

Unit tests are required for:

- configuration loading
- path validation
- filesystem operations
- MCP protocol routing
- tool execution

### Filesystem Tests

Filesystem tests use:

```go
t.TempDir()
```

This ensures that tests do not modify real user data.

Test helper functions may use `os.WriteFile`, `os.ReadFile`, `os.MkdirAll` and `os.Stat` to create and verify test fixtures.

This is allowed in tests.

Production code outside `internal/fs` must not use direct filesystem operations.

### Security Tests

Security tests must cover:

- path traversal
- absolute path rejection
- sandbox escape attempts
- destructive operation defaults
- overwrite behavior
- recursive delete behavior
- JSON-RPC message and tool argument limits
- filesystem read, write, list, copy, and recursive delete limits
- diagnostics redaction

### Integration Tests

JSON-RPC smoke tests exercise the built server binary over STDIO.

On Windows:

```powershell
.\scripts\smoke-jsonrpc.ps1
$env:MCP_READ_ONLY = "true"
.\scripts\smoke-jsonrpc.ps1
Remove-Item Env:\MCP_READ_ONLY
.\scripts\smoke-jsonrpc-negative.ps1
```

On Linux:

```bash
bash scripts/smoke-jsonrpc.sh
MCP_READ_ONLY=true bash scripts/smoke-jsonrpc.sh
bash scripts/smoke-jsonrpc-negative.sh
```

Run fail-closed startup validation on Windows:

```powershell
.\scripts\smoke-startup-negative.ps1
```

On Linux:

```bash
bash scripts/smoke-startup-negative.sh
```

The default smoke test validates `initialize`, the exact eight-tool `tools/list`, `list_directory`, `read_file`, `get_path_info` for existing and missing paths, and `move_path` rename behavior. Every positive result must have only the `CallToolResult` envelope fields, exactly one text block with only `type` and `text`, valid compact object JSON, and a deeply equal `structuredContent` object. The read-only variant verifies the exact three-tool profile and invokes all five write-capable names, requiring the same generic Invalid params response without filesystem changes. The negative smoke validates all five removed legacy names in addition to malformed JSON, unknown methods, invalid `tools/call` params, and notification no-response behavior.

The startup-negative smoke covers missing/empty/whitespace/relative roots, `.` with and without the development opt-in, invalid development/read-only values, missing and file roots, a valid absolute root, exit codes, empty stdout, safe stderr categories and cleanup.

GitHub Actions runs default, read-only, negative JSON-RPC, and startup-negative smoke variants on both `windows-latest` and `ubuntu-latest`. The smoke scripts create per-run artifacts under `build/` and clean them before exit. Script output is CI diagnostic output; server stdout remains reserved for redirected JSON-RPC protocol messages.

Limit and redaction behavior is primarily covered by Go unit tests. Additional limit-negative smoke coverage can be added later if it can be done without broad smoke-script refactoring.

Focused contract tests compare runtime tool definitions with `docs/mcp-tool-catalog.json` for name, title, description, complete input schema, and deeply equal runtime `outputSchema`/catalog `resultSchema`. Targeted tests require exactly eight runtime output schemas, object roots, valid required/property relationships, expected project property types, representative successful `structuredContent`, both `get_path_info` variants, and the `read_file` outer-array/inner-string distinction. The tests-only structural checker covers only `type`, `properties`, `required`, `additionalProperties`, `items`, `oneOf`, and `const` as currently emitted; it is not a complete JSON Schema 2020-12 validator.

The `tools/list` JSON-RPC wire test checks schema exposure for both profiles and records deterministic UTF-8 JSONL sizes with and without output schemas. `SPR-46` records 1239/2134 bytes for read-only and 3850/5657 bytes for default; no regression budget is enforced.

### MCP Compatibility Testing

The implemented protocol remains MCP `2025-11-25`. Explicit `CallToolResult` DTO tests, a strict project-local decoder, legacy unwrapped negative fixtures, all-eight-tool adapter coverage, and full JSON-RPC wire tests cover success and the unchanged error contract. The decoder intentionally validates the exact FlashGate-emitted subset (one text block, required object `structuredContent`, optional boolean `isError`, no `_meta`) rather than claiming to decode every standard-conformant MCP result. Windows and Bash positive smokes enforce the same shape.

Future protocol or extension support still requires version-negotiation, extension-negotiation, client fallback, and compatibility tests before it is advertised. Complete JSON Schema 2020-12 validation and official MCP conformance tooling remain planned.

### Benchmarks

`SPR-47` benchmarks performance-sensitive operations including:

- directory listing
- file reading
- file copying
- tool-result wrapping and serialized payload forms
- search

Benchmark command:

```bash
go test -bench=. ./...
```

The deterministic benchmark tests also enforce the exact tool-profile and workflow measurement sets, the six payload/allocation budgets from `benchmarks/budgets.json`, initialized-notification framing, initialization-result validation, zero `scanned_bytes` for ordinary reads, partial Linux procfs metrics, host-path redaction, and clean versioned artifacts. Windows and Linux artifacts are each strictly decoded and independently reevaluated only after the canonical hard/soft workflow sets and all positive soft limits have been validated fail-closed. Their complete resource, sample, exit-status, stderr, general warning, unsupported-metric, provenance, and deterministic cross-platform parity is then compared. The validator rejects unknown or duplicate fields, missing required fields, wrong types, nulls, trailing values, stale or fabricated embedded budget results, incomplete/unknown/zero soft budgets, hard-budget manipulation of either or both artifacts, and schema-invariant violations. Runner result construction stores soft budget messages only in `budget_evaluation`; production-path tests cover dual-platform, Windows-only, Linux-only, and no-soft-warning results through the complete platform gate. General result `warnings` remain reserved for non-budget runtime warnings and fatal in clean versioned artifacts, while matching soft budget warnings remain review-only and recomputed hard failures remain fatal. Platform baseline generation is a separate two-phase operation after the implementation commit. The diagnostic wrappers recognize `-RecordBaseline` and `--record-baseline` only to reject them fail-closed before any Go invocation or write. All explicit and Linux-default output names are physically checked; protected-directory aliases and existing final symlink/reparse targets are rejected. The runner additionally binds the validated parent with `os.Root`, writes through one exclusively opened temporary-file handle, and publishes by handle-relative rename without following the final target. Tests cover protected parent aliases, noncanonical final links to both platform baselines, broken links, hard-link aliases, late target/output-parent/protected-directory exchanges, regular diagnostic output, and invariant baseline hashes.

Functional gates such as tests, builds, vet, lint, protocol smokes, and parser checks are independent of the host measurement window. Their timing and resource consumption are not performance evidence. Performance gates are valid only when the entire measurement series runs outside the primary development host's scheduled-load block from 19:00 inclusive until 04:00 exclusive in `Europe/Vienna`; the preferred safety-margin window is 04:15–18:45, and the series must finish before 19:00. The blocked interval is a formal baseline blocker.

Every performance measurement report records the `Europe/Vienna` time window, start and end times, and whether known or unusual additional host load was present. A baseline is rejected if such load is known or observed even inside the nominally allowed interval. Contaminated runs are retained as diagnosis evidence without being approved, compared for regression, or used to tune budgets. Ordinary wrapper runs may continue but are marked contaminated; they cannot record a baseline.

## Current Tested Packages

Currently tested:

- `internal/config`
- `internal/diagnostics`
- `internal/security`
- `internal/fs`
- `internal/protocol`
- `internal/mcp/server`
- `internal/mcp/router`
- `internal/mcp/transport`
- `internal/mcp/initialize`
- `internal/mcp/tools`

## Version 1.0 Planned Validation Matrix

The current tests above describe the implemented filesystem baseline. Version 1.0 adds the following required gates.

### Payload and catalog tests

- payload-class selection for metadata, structured pages, heavy text, media/binary, and large results;
- heavy payload appears only once across MCP result fields;
- wire-amplification and useful-byte budgets;
- bounded base64 thresholds;
- opaque resource handles contain no host path and enforce owner/TTL/service-generation checks;
- fallback behavior for clients without resource-link support;
- deterministic tool ordering and catalog fingerprint;
- profile-specific `tools/list`, schema, description, and initialization-instruction budgets;
- safe read-only catalog when roots exist and no explicit profile is selected.

### Operations and multi-principal tests

- opaque handles bound to principal, profile, root, execution backend, and service generation;
- cross-principal status/result/cancel/cache/resource denial;
- global, per-domain, and per-principal concurrency limits;
- global/per-principal queue caps and fair scheduling;
- deterministic overload behavior;
- TTL cleanup, restart invalidation, shutdown, and leak detection;
- slow-reader and audit/log backpressure behavior.

### Typed command tests

- executable ID resolves only to approved absolute binary;
- fixed subcommand and allowed flags/value rules;
- path arguments remain under allowed roots;
- no shell interpretation;
- response files, hooks, plugins, loaders, config overrides, and unapproved environment are rejected;
- stdout/stderr, runtime, process count, and network policy limits;
- Windows/Linux isolation outcomes and redaction.

### System service and execution-identity tests

Version 1.0 tests Variant A only:

- Windows SCM and Linux systemd lifecycle;
- Named Pipe ACL and Unix socket ownership/mode;
- OS-derived peer identity cannot be overridden by payload;
- caller authorization independent of service-account filesystem permission;
- allowed FlashGate policy plus denied service-account ACL fails safely;
- denied caller plus available service-account ACL fails before execution;
- service-account root backend and dedicated identity;
- no LocalSystem/root convenience default;
- unsupported `user-worker` configuration fails closed;
- no in-process impersonation path;
- caller and effective backend identity both appear in bounded audit events;
- service restart invalidates generation-bound handles/resources;
- `auto` never falls back after managed denial or incompatibility;
- proxy/client stdout remains MCP-only.

Variant B worker tests are post-Version 1.0 and require a separate implementation gate.

### Protocol compatibility tests

Before Version 1.0, publish and test the supported MCP revision matrix:

- current `2025-11-25` behavior;
- any later final revision only after implementation;
- stateless-core behavior where selected;
- deterministic list cache/TTL invalidation;
- final Tasks Extension mapping without mixing the 2025 experimental lifecycle;
- extension downgrade/mismatch;
- JSON Schema 2020-12 validation;
- deprecated Roots never overrides server roots.

### Audit and failure-path tests

- immutable event/correlation IDs;
- proxy/service/backend/job/process correlation;
- redaction before output;
- rotation and retention;
- slow sink and bounded buffering;
- disk-full behavior;
- log-injection handling;
- shutdown flush/drop policy;
- no secret, full payload, unrestricted environment, or unnecessary host-path leakage.

### Release and supply-chain tests

- artifact version/help/platform/name checks;
- compact and verbose CLI identity checks;
- Windows x64/ARM64 `VERSIONINFO`, PE architecture, icon, and Explorer property checks;
- Linux x64 native and ARM64 cross-build Go/VCS, ELF-header, ELF-note, and Go build-ID checks;
- exact ZIP/TAR.GZ inventory and SHA-256 verification;
- repeated byte-for-byte binary and archive reproducibility checks;
- host-path, username, hostname, and credential-shaped-value leak checks;
- shared Go/PowerShell/Bash SemVer and `SOURCE_DATE_EPOCH` fixtures;
- fail-closed Git snapshot inventory, ignored/sensitive-file, TAR traversal,
  link, special-type, duplicate, missing, length, and hash fixtures;
- descriptor-bound native work-root traversal, symlink, component-exchange,
  length, and whitespace fixtures;
- static canonical build-manifest validation on all four targets;
- normalized embedded-icon frame identity and manipulation fixtures;
- persistent Windows/Linux verifier-process contracts for native-host
  selection, static-failure-before-launch, runtime/help status, timeout,
  bounded stdout/stderr, deterministic Windows termination/cleanup deadlines,
  complete verifier error aggregation, and READY-confirmed
  child-tree/process-group cleanup;
- regular-clone, linked-worktree, nonrepository, and damaged-Git fixtures;
- no interpreter runtime dependency;
- service asset syntax and install/remove dry validation;
- checksums;
- SBOM and dependency inventory;
- build provenance;
- signing verification where configured;
- reproducible-build comparison;
- pinned/validated workflow policy;
- atomic rollback documentation and smoke procedure.

The controlled commands and expected fields are documented in [Build and
release metadata](build-and-release-metadata.md), [Artifact
verification](artifact-verification.md), and [Manual metadata
validation](manual-metadata-validation.md). Native Linux validation verifies a
manifest-bound Git inventory in a new controlled extraction directory before
copying it into an ext4 clone under `/home`; Windows-mounted paths are
orchestration inputs only and are never the native build directory.

### Cross-project benchmark

The Version 1.0 benchmark compares pinned FlashGate, official Node.js filesystem, selected native Rust filesystem, and selected Go filesystem MCP versions on the same host and corpus. The report must separate feature/security differences from measured performance and must not claim results for unmeasured operations.

See [Efficiency Improvement Plan](efficiency-improvement-plan.md), [Execution Identity Backends](execution-identity-backends.md), and [Version 1.0 Scope](version-1-scope-and-release-boundary.md).

<!-- FLASHGATE_PERFORMANCE_WORKSPACE_POLICY_START -->
## Authoritative benchmark workspace gate

On the primary Windows development host, an authoritative benchmark attempt is
blocked unless its Windows working area is below:

`C:\Voxtronic\Codex\Temp\Benchmarks`

Before the attempt, verify that the root is a fixed local NTFS path, contains no
reparse point, and is not below OneDrive, Dropbox, a redirected user directory,
a network share, or other synchronized storage.

All source bundles, isolated Windows checkouts, prepared binaries, measurement
outputs, logs, verification files, and controller data stay in that local area
through the final host-load gate. Archival copying to synchronized storage occurs
only afterward and is not part of the measured phase.

The native Linux checkout and temporary output remain on the distribution's
native ext4 filesystem under `/home`. A path below `/mnt` or `/media` is a formal
baseline blocker.

All validation, test, vet, lint, build, linker, and parser work finishes before
the authoritative host gate. After the last such operation, wait at least 180
seconds without Git, Go, scan, archive, or analysis activity. Run the authoritative
three-block CPU/disk/RAM/per-process-delta preflight exactly once. If it passes,
invoke the prepared binaries directly without rebuilding. A 15-second intermediate
gate precedes native Linux measurement and a final host gate precedes any result
copy, hash scan, JSON verification, report, archive, or OneDrive access.

`scripts/benchmark.ps1 -RecordBaseline` and
`scripts/benchmark.sh --record-baseline` are deliberately blocked compatibility
flags, not an authoritative workflow. A separately prepared controller implements
the two-phase attempt; no wrapper-side shortcut or time override is permitted.
<!-- FLASHGATE_PERFORMANCE_WORKSPACE_POLICY_END -->
