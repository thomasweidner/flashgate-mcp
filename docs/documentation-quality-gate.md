# Documentation quality gate

This document defines the repeatable documentation checks for FlashGate MCP. It supplements, but does not replace, the authority of `BACKLOG.md`, accepted ADRs, source code, tests, Git history, and CI evidence.

## Purpose

The gate prevents structurally inconsistent documentation from being committed or released. It focuses on checks that can be evaluated deterministically and keeps semantic review explicit instead of approximating it with broad keyword searches.

The earlier temporary `CUR-010` audit finding demonstrated why this distinction matters: the word `search` appeared in a section that explicitly described planned behavior and was incorrectly treated as an implemented-state claim. The permanent gate therefore verifies section boundaries and required statements rather than forbidding planned-domain words throughout mixed explanatory prose.

## Command

Use PowerShell 7.6.4:

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
The authoritative handoff gate remains the canonical external
`Test-ClassicReviewArtifact.ps1` validator named by the handoff standard.
Hosted CI executes the byte-bound repository mirror at
`scripts/Test-ClassicReviewArtifact.ps1`; the mirror is an execution source,
not an independent governance source. The fixture runner fails closed on
mirror/canonical byte divergence whenever the external validator is locally
available.

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

The repository script is the permanent local gate. GitHub Actions integration should be added only after the script has passed under PowerShell 7.6.4 on the integrated target-branch state. A dedicated workflow or CI job should call the same script without duplicating its validation logic.

CI integration must not silently convert exit code `2` into a documentation failure or ignore it. Infrastructure failure is separately actionable and blocks the gate because no trustworthy audit result was produced.
