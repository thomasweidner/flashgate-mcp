# Documentation quality gate

This document defines the public documentation checks for FlashGate. It
supplements `BACKLOG.md`, accepted ADRs, product code, tests, Git history, and
CI evidence.

## Purpose

The gate prevents current documentation from contradicting product, test,
release, security, backlog, or repository-boundary state. Historical planning
and changelog material remains non-operative evidence and does not define a
current contributor workflow.

## Command

Use PowerShell 7.6.x (Major 7, Minor 6):

```powershell
& pwsh -NoLogo -NoProfile -File .\scripts\Test-DocumentationConsistency.ps1
```

The script is read-only. It emits one JSON result and returns a nonzero exit
code when a focused check fails.

## Automated checks

The focused gate verifies:

- strict UTF-8 readability of required public project documents;
- backlog identifier/status consistency without reopening completed work;
- active Windows/Linux Go, coverage, lint, build, release, metadata, shell,
  PowerShell 7.6 LTS-line, documentation, and security gates;
- absence of operational dependencies on private development infrastructure;
- absence of private host paths from active public guidance;
- the explicit classification of historical planning and changelog content as
  non-operative.

## Manual review

Confirm that documentation still matches product and test behavior, links
resolve, historical text is not presented as a current instruction, and no new
product, architecture, dependency, release, credential, remote, or destructive
decision was introduced.

## CI boundary

Hosted CI consumes only files in the public checkout and standard runner tools.
It must not read contributor-local workflow files, personal paths, credentials,
or task directories.

## PowerShell 7.6 LTS patch contract

Compatibility requires PowerShell major version 7 and minor version 6. Record
the observed patch separately; maintain against the latest serviced 7.6 patch
unless a documented fix establishes a higher minimum.
