# Code Coverage

## Purpose

FlashGate MCP uses production-server Go statement coverage as a regression gate for both supported development and validation platforms. Coverage is a quality signal, not a substitute for security review, behavioral tests, race detection, linting, or native operating-system validation.

Windows remains the leading development platform. Linux coverage is measured in the controlled native Linux validation environment.

## Authoritative Sources

The following repository files are authoritative:

- `.github/workflows/ci.yml` defines the active platform matrix and minimum coverage values.
- `scripts/Test-GoCoverage.ps1` runs tests, creates reports, evaluates the minimum, and writes a machine-readable summary.
- This document explains operation and maintenance but does not override the workflow values.


## Current Gates

| Platform | Minimum statement coverage | Validation context |
|---|---:|---|
| Windows | 90.0% | Windows CI job and local PowerShell 7.6.x validation |
| Linux | 90.0% | Native Ubuntu GitHub Actions job |

The values are intentionally separate. Operating-system-specific source files and build behavior can produce different totals. The two percentages must not be averaged or replaced by one cross-platform value.

## Measurement Model

The script obtains the current module path with `go list -m` and determines the
active platform's transitive dependencies for `./cmd/server` with `go list
-deps`. It keeps only packages in the current module, sorts and deduplicates
them, and rejects an empty or incomplete product graph. The full repository
test suite then runs with:

```text
go test -covermode=atomic -coverpkg=<sorted-product-packages> -coverprofile=<platform-output>/coverage.out ./...
```

The product denominator includes server-linked FlashGate packages only. Tests
still run across `./...`, including development-only commands, but those
commands, external modules, vendored code, generated artifacts, and test
helpers do not enter the product percentage. `product-packages.txt` records the
exact platform package closure. Windows and Linux may differ because of build
tags; each platform is measured and gated independently.

The total is read from:

```text
go tool cover -func=<coverage-profile>
```

The CI gate compares the unrounded ratio of covered to total unique product
statements against the platform-specific minimum. Repeated coverage blocks
from different test packages are combined before counting. The per-function
text report retains Go's one-decimal display.

## Local Execution

### Windows

Run from the repository root with PowerShell 7.6.x (Major 7, Minor 6):

```powershell
.\scripts\Test-GoCoverage.ps1 -PlatformName windows -MinimumCoverage 90.0
```

### Linux

Run from the native Linux checkout with PowerShell 7.6.x:

```bash
pwsh -NoLogo -NoProfile -File ./scripts/Test-GoCoverage.ps1 -PlatformName linux -MinimumCoverage 90.0
```

Linux validation uses a native Linux filesystem as described in [testing](../testing.md#native-linux-validation). Do not treat a Windows-mounted checkout as native validation or synchronize an active validation tree bidirectionally.

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
| `product-packages.txt` | Sorted product package inventory for the active platform |
| `summary.json` | Machine-readable status, scope, root, package count and inventory path, exact coverage and statement counts, minimum, report paths, and error |

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

1. a reproducible product-scope measurement from the current implementation on both platforms;
2. an explanation for raising or lowering the value;
3. successful Windows and Linux CI jobs;
4. confirmation that `summary.json` reports the correct status in both positive and negative gate tests;
5. an update to this document and the README when the documented values change.

A temporary reduction must not be used to hide a regression. A real regression requires either additional tests, a justified scope change, or a separately approved measurement-model change.

## Interpreting Coverage

Coverage indicates which statements were executed; it does not prove correctness. Review should prioritize meaningful tests in security- and filesystem-sensitive packages rather than trivial execution paths added only to increase the percentage.

The project target is at least 95.0%. Pursue 100% for bounded core logic where
meaningful tests can cover it without artificial assertions.

The following checks remain separate and mandatory where applicable:

- formatting and linting;
- `go vet`;
- unit and integration tests;
- JSON-RPC and startup smoke tests;
- Windows/Linux native validation;
- Linux race detection;
- security and release review.

## PowerShell 7.6 LTS patch contract

Compatibility requires PowerShell major version 7 and minor version 6. `ObservedPowerShellVersion` records the actual patch. `MinimumPowerShellVersion` is `null` unless a specific fix establishes and justifies a minimum patch. `ServicingTarget=LatestServicedPatchWithin7.6` is the maintenance target. Exact patch, path, or hash bindings are allowed only for historical evidence, bug reproduction, installer/download/SBOM/supply-chain provenance, or a documented minimum-patch fix; each exception must be explicitly classified.
