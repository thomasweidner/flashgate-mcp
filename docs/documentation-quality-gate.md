# Documentation quality gate

This document defines the focused documentation checks for the FlashGate Slim
Governance project adapter. It supplements `BACKLOG.md`, accepted ADRs, product
code, tests, Git history, and CI evidence.

## Purpose

The gate prevents current project documentation from contradicting product,
test, release, security, backlog, or Slim Governance adapter state. Historical
Heavy-Governance material may remain explicitly marked
`LEGACY_COMPATIBILITY_ONLY`; it is not a normal development blocker.

## Command

Use PowerShell 7.6.5:

```powershell
& pwsh -NoLogo -NoProfile -File .\scripts\Test-DocumentationConsistency.ps1
```

The script is read-only. It emits one JSON result and returns a nonzero exit
code when a focused check fails.

## Automated checks

The focused gate verifies:

- strict UTF-8 readability and required active project documents;
- BL-343 registration/completion, BL-337 superseded closure, BL-330 retained
  project scope, and highest-ID truth;
- presence of the thin project adapter and `DIRECTLY_AFFECTED_FIRST` policy;
- active Windows/Linux Go, coverage, lint, build, release, metadata, shell,
  PowerShell 7.6.5 and security gates;
- disabled Heavy-Governance orchestration in normal CI and its absence from the
  active release-preparation path;
- absence of contributor-local Codex-Work or stale personal benchmark paths in
  active repository guidance;
- all ten INF168-REV-007 dispositions recorded by BL-343.

The gate does not run Generic Handoff, Finding Correction, Commit Preparation,
publication, orchestration, or V3/V4 fixture matrices. Those remain historical
compatibility assets until a separately authorized consumer cutover.

## Manual review

Confirm that documentation still matches the actual product and test behavior,
that no fachlich relevant FlashGate statement was discarded as meta-governance,
and that no new product, architecture, dependency, release, credential, remote,
or destructive decision was introduced.

## CI boundary

Hosted CI consumes only repository files and prepared runner tools. It must not
read contributor-local `<CodexPersistentRoot>`, personal benchmark paths, or
task directories. Central governance is validated centrally when it changes.
