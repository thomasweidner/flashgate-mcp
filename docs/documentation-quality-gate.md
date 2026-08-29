# Documentation quality gate

This document defines the repeatable documentation checks for FlashGate MCP. It supplements, but does not replace, the authority of `BACKLOG.md`, accepted ADRs, source code, tests, Git history, and CI evidence.

## Purpose

The gate prevents structurally inconsistent documentation from being committed or released. It focuses on checks that can be evaluated deterministically and keeps semantic review explicit instead of approximating it with broad keyword searches.

The earlier temporary `CUR-010` audit finding demonstrated why this distinction matters: the word `search` appeared in a section that explicitly described planned behavior and was incorrectly treated as an implemented-state claim. The permanent gate therefore verifies section boundaries and required statements rather than forbidding planned-domain words throughout mixed explanatory prose.

## Command

Use PowerShell 7.6.5:

```powershell
& {
    try {
        $result = & pwsh -NoLogo -NoProfile -File .\scripts\Test-DocumentationConsistency.ps1 2>&1
        $exitCode = $LASTEXITCODE
    }
    catch {
        $result = $_.Exception.Message
        $exitCode = 2
    }
    finally {
        [pscustomobject]@{
            Status     = if ($exitCode -eq 0) { 'PASS' } else { 'FAIL' }
            ExitCode   = $exitCode
            Result     = ($result -join [Environment]::NewLine)
            NextAction = if ($exitCode -eq 0) { 'Complete the manual review checklist.' } else { 'Open the generated report and correct the reported failure.' }
        } | Format-List
    }
}
```

Governance source changes additionally require:

```powershell
& {
    .\scripts\Test-GovernanceConsistency.ps1
    .\scripts\Test-GovernanceConsistencyFixtures.ps1
    .\scripts\Test-GenericGovernanceHandoffFixtures.ps1
}
```

This gate checks the binding standards, catalog/schema parity, complete tracked
path coverage, assignment checkpoints, immutable mode semantics, same-run and
deferred-finding bindings, focused-delta scope, actual correction/current-delta
byte hashes, strict finding, report, and bounded handoff contracts, exact
finding/repository/external/status parity, exact case-insensitive Windows
path-to-scope mapping for the five external governance entries, strict parsing
of the exactly-16-key visible HANDOFF status block, independent counting and
ordering of every status/contract marker, rejection of duplicate, unknown, or
outside-block reserved control lines, complete typed visible-HANDOFF/JSON
parity, actual-package handoff readiness, commit preparation, and trusted
Git/Hosted CI provenance. Re-manifesting altered bytes or rewriting
self-reported hashes cannot replace the separately supplied expected hashes.
BL-336 adds an explicit transition/profile discriminator: the generic
commit-preparation path requires task, review, patch, scope, validation,
inventory, manifest, and hash parity while accepting zero findings; the
isolated correction profile retains all historical BL-333/BL-334 checks. Mixed
profiles fail closed. The authoritative handoff gate remains the canonical external
`Test-ClassicReviewArtifact.ps1` validator named by the handoff standard.
Hosted CI executes the byte-bound repository mirror at
`scripts/Test-ClassicReviewArtifact.ps1`; the mirror is an execution source,
not an independent governance source. The fixture runner fails closed on
mirror/canonical byte divergence whenever the external validator is locally
available.

The Windows governance job checks out the exact pull-request head independently
from the merge-result Go matrix. It compares the event-loaded workflow blob with
the exact-head workflow blob before accepting workflow provenance. It then uses
the official PowerShell 7.6.5 Windows x64 ZIP only after the pinned SHA-256
`32EB8F6CDCE08F86E987D625A2733E54AC3E289AE7E1621B14C0B5BCEC2434EA`
passes. Only after the hash, extraction, and executable-existence gates does it
publish the verified extraction directory through `GITHUB_PATH`; the
governance step uses the static `shell: pwsh` and requires its actual process
path to equal the expected hash-gated executable under ordinal,
case-insensitive Windows semantics after full-path normalization. Casing-only
differences are accepted; changed drives, directories, subdirectories,
filenames, relative paths, and empty paths remain fail-closed. Both the
productive validator and fixture harness fail closed on a different
interpreter version; the productive validator also rechecks the downloaded
package digest. The focused workflow-binding fixture rejects missing path
publication, missing expected path binding, the obsolete dynamic shell form,
and case-sensitive path comparisons without changing the permanent 198-case
inventory.

PR #27 merged through regular merge commit
`e42d57d57ea075640c9b123a533057bcac3861b8`. Its two-parent structure,
second-parent six-commit reachability, merge/head tree equality, and 26/26 path
parity passed. Post-merge CI, Metadata Regression, and CodeQL completed
successfully; exact-head and workflow-source parity, PowerShell 7.6.4, 1,051
governance checks with zero errors, and 198/198 fixtures passed.
`PR27-EXACT-REV-001`, `PR27-EXACT-REV-002`, and `PR27-EXACT-REV-003` remain
`CLOSED_BY_INDEPENDENT_EXACT_COMMIT_REVIEW`, all earlier findings remain
closed, and both review queues are empty. BL-333 and BL-334 are `Done`.
BL-335 remains `Planned` and not begun as the next queue step.

The post-merge finalization is documentation-only. Its exact allowlist is
`BACKLOG.md`, `CHANGELOG.md`, `docs/testing.md`, and
`docs/documentation-quality-gate.md`; runtime, workflows, validators, fixtures,
INF-121, and BL-335 remain unchanged. Local fixture reuse is allowed only
because validator and fixture bytes are unchanged and the exact post-merge
Hosted run binds 198/198; the Finalization PR must execute the full matrix.

The script writes its detailed report to:

```text
build/reports/documentation-consistency.md
```

An alternate repository root or report path can be supplied through `-RepositoryRoot` and `-ReportPath`.

## Exit codes

| Exit code | Meaning |
|---:|---|
| `0` | Automated gate passed; warnings may still require review |
| `1` | Documentation inconsistency detected |
| `2` | Script or infrastructure failure prevented a valid audit |

## Automated checks

The script checks the following areas:

| Area | Required result |
|---|---|
| Repository structure | Required documentation and script paths exist |
| Encoding | Every Markdown file is valid UTF-8 |
| Duplication | No Markdown files have identical content |
| Relative links | Inline relative Markdown links resolve inside the repository |
| Backlog catalog | Canonical IDs are unique and continuous from `BL-001` through the current highest ID |
| Sprint identifiers and assignments | Canonical rows use unique `SPR-<positive integer>` IDs without legacy `Sprint 3.x` rows; no unknown or duplicate backlog references; every `Planned` task is assigned exactly once |
| Milestone separation | No `Later` task is assigned to a Version 1.0 sprint |
| Sprint status | `Done` sprint rows contain only `Done` tasks; early completion inside a planned cross-cutting sprint is reported for manual review |
| Project identity | Active identity documents use `thomasweidner/flashgate-mcp` and `github.com/thomasweidner/flashgate-mcp` |
| Current-state boundary | Architecture identifies the eight current tools and explicitly lists major planned domains as not implemented |
| Tool catalog parity | README, `docs/tools.md`, and `docs/mcp-tool-catalog.json` contain the exact eight current tools in canonical order |

The highest backlog ID is discovered dynamically. The gate remains valid when the canonical catalog is extended without renumbering existing tasks.

## Mandatory manual review

Automation cannot reliably prove semantic correctness. Before commit or release, review and record all applicable items:

- current implementation, accepted Version 1.0 target, and post-Version-1.0 scope remain clearly separated;
- task status is supported by merged code, tests, CI, reports, and Git history;
- historical ADRs and migration documents are not rewritten; later corrections use dated amendments or new migration records;
- README, changelog, backlog, architecture, security, protocol, specification, tool, testing, benchmark, and coverage documents affected by the change are synchronized;
- current repository, module, binary, server name, protocol revision, tool count, profile behavior, coverage thresholds, and release claims match authoritative implementation or CI sources;
- no local credentials, private host paths, transient branch state, or temporary report paths are introduced;
- `git diff --check` passes after the documentation changes are applied to the intended target-branch state.

## Gate timing

Run the gate:

1. after changing canonical documentation;
2. after rebasing or reapplying documentation over a changed target branch;
3. before committing the consolidated documentation;
4. before a release candidate or Version 1.0 release decision;
5. whenever backlog IDs, sprint assignments, project identity, tool contracts, or current-state claims change.

## CI integration boundary

The repository script is the permanent local gate. The dedicated Hosted
governance job calls the same productive script and fixture matrix under the
pinned PowerShell 7.6.5 package without duplicating their validation logic.
Merge-result Go testing remains a separate CI responsibility.

CI integration must not silently convert exit code `2` into a documentation failure or ignore it. Infrastructure failure is separately actionable and blocks the gate because no trustworthy audit result was produced.
