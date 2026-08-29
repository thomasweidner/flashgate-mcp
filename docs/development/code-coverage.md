# Code Coverage

## Purpose

FlashGate MCP uses repository-wide Go statement coverage as a regression gate for both supported development and validation platforms. Coverage is a quality signal, not a substitute for security review, behavioral tests, race detection, linting, or native operating-system validation.

Windows remains the leading development platform. Linux coverage is measured in the controlled native Linux validation environment.

## Authoritative Sources

The following repository files are authoritative:

- `.github/workflows/ci.yml` defines the active platform matrix and minimum coverage values.
- `scripts/Test-GoCoverage.ps1` runs tests, creates reports, evaluates the minimum, and writes a machine-readable summary.
- This document explains operation and maintenance but does not override the workflow values.

The initial dual-OS gates were merged through PR #18 in commit `a23b7a6b542e6cc3af77e881c8970f9b02821b79`.

## Current Gates

| Platform | Minimum statement coverage | Validation context |
|---|---:|---|
| Windows | 71.4% | Windows CI job and local PowerShell 7.6.5 validation |
| Linux | 70.6% | Native Ubuntu GitHub Actions job |

The values are intentionally separate. Operating-system-specific source files and build behavior can produce different totals. The two percentages must not be averaged or replaced by one cross-platform value.

## Measurement Model

The script executes the repository-wide Go test suite with:

```text
go test -covermode=atomic -coverpkg=./... -coverprofile=<platform-output>/coverage.out ./...
```

This measures statement coverage across all repository packages included by `./...`. Benchmark and development-only packages remain part of the repository-wide value unless a separate reviewed decision changes the measurement model.

The total is read from:

```text
go tool cover -func=<coverage-profile>
```

The CI gate fails when the measured value is below the platform-specific minimum.

## Local Execution

### Windows

Run from the repository root with PowerShell 7.6.5:

```powershell
.\scripts\Test-GoCoverage.ps1 -PlatformName windows -MinimumCoverage 71.4
```

### Linux

Run from the native Linux checkout with PowerShell 7:

```bash
pwsh -NoLogo -NoProfile -File ./scripts/Test-GoCoverage.ps1 -PlatformName linux -MinimumCoverage 70.6
```

Linux validation must follow the project policy: native Linux filesystem paths, no productive development in WSL2, and no bidirectional synchronization.

## Generated Outputs

Each platform writes to:

```text
build/coverage/<platform>/
```

Generated files:

| File | Purpose |
|---|---|
| `coverage.out` | Go coverage profile |
| `coverage.txt` | Per-function text report |
| `coverage.html` | Human-readable HTML report |
| `test.log` | Complete `go test` output |
| `summary.json` | Machine-readable status, platform, coverage, minimum, paths, and error |

`summary.json` reports `PASS` only after the minimum has been satisfied. Failures, including a coverage shortfall or report-generation error, are represented as `FAIL`.

## GitHub Actions

The existing CI test matrix runs the same script separately for:

- `windows-latest`
- `ubuntu-latest`

Each job uploads its own artifact:

```text
go-coverage-<platform>-<run-id>-<attempt>
```

Artifacts are retained for 14 days. Windows and Linux artifacts remain independent so platform regressions are visible and auditable.

## Changing a Minimum

A minimum may be changed only through a focused pull request. The change must include:

1. a reproducible platform measurement from the current implementation;
2. an explanation for raising or lowering the value;
3. successful Windows and Linux CI jobs;
4. confirmation that `summary.json` reports the correct status in both positive and negative gate tests;
5. an update to this document and the README when the documented values change.

A temporary reduction must not be used to hide a regression. A real regression requires either additional tests, a justified scope change, or a separately approved measurement-model change.

## Interpreting Coverage

Coverage indicates which statements were executed; it does not prove correctness. Review should prioritize meaningful tests in security- and filesystem-sensitive packages rather than trivial execution paths added only to increase the percentage.

The following checks remain separate and mandatory where applicable:

- formatting and linting;
- `go vet`;
- unit and integration tests;
- JSON-RPC and startup smoke tests;
- Windows/Linux native validation;
- Linux race detection;
- security and release review.
