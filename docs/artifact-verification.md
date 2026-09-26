# Artifact verification

FlashGate verifies release artifacts against the same canonical build identity
used by the executable, Windows resources, Linux build information, archive
names, and release workflows. BL-248 does not introduce a second version or
product-metadata source.

## Canonical expectation sources

The root `VERSION` file supplies the canonical controlled product version;
`internal/version` owns the runtime identity model and development defaults.
Controlled build inputs are validated by
`scripts/Build-InputValidation.ps1` and
`scripts/build-input-validation.sh`. The Windows resource generator, embedded
manifest verifier, icon verifier, release builders, and archive audit consume
those values instead of a separately maintained expected-metadata document.

Release validation uses `VERSION`, its exact `v<VERSION>` tag, the clean source
commit, and source time resolved by the controlled build or workflow. A passed
release version can only assert equality with `VERSION`. Public `x64` maps to Go
`amd64`; public `arm64` maps to Go `arm64`.

## Verification layers

| Layer | Canonical implementation | Contract |
|---|---|---|
| Windows binary | `scripts/Test-WindowsMetadata.ps1`, `scripts/VerifierProcess.ps1`, `cmd/versionmanifest`, `cmd/iconverify` | PE x64/ARM64 machine type, `VERSIONINFO`, canonical product fields, embedded build manifest, icon identity, Go target, and bounded compact/verbose/help execution only after all static checks pass and the target matches the actual Windows OS architecture |
| Linux binary | `scripts/Test-LinuxMetadata.sh`, `scripts/verifier-process.sh`, `scripts/bounded-process-runner.py`, `cmd/versionmanifest` | ELF x64/ARM64 machine type, static linkage, Go build ID, Go/VCS provenance, canonical build manifest, and bounded compact/verbose/help execution only after all static checks pass and the target matches the intrinsic `uname -m` host architecture |
| Release package | `scripts/Test-ReleaseArtifact.ps1`, `scripts/test-release-artifact.sh`, `cmd/releaseaudit` | exact ZIP/TAR.GZ names and inventory, regular-file entry types, checksum match, controlled extraction, and rejection of traversal, links, special entries, duplicates, missing files, or unexpected files |
| Reproducibility and leaks | `cmd/releaseaudit compare`, `scan`, and `scan-file` | two-build binary/archive/checksum/inventory identity plus machine-readable host-path, credential-pattern, private-key, and caller-supplied forbidden-value findings |
| Orchestration | `.github/workflows/metadata-regression.yml`, `.github/workflows/release-build.yml`, and the controlled native Linux validation driver | Windows and Linux x64/ARM64 build and static validation; native execution on matching x64 hosts; deterministic reports and aggregated nonzero failure status |

The verifier, rather than its caller, determines whether execution is native.
Windows uses the actual OS architecture, not the PowerShell process
architecture. Linux normalizes the controlled `uname -m` result. Nonmatching
targets are automatically `SKIPPED`; `--skip-execution` is only an additional
Linux restriction and can never enable a cross-built target. ARM64 artifacts
remain cross-built and statically inspected on current x64 runners; native
ARM64 execution is not claimed by the current runner model.

Every child process has a deterministic execution timeout and separate bounded
stdout and stderr capture. Both streams are drained concurrently. Windows uses
a second bounded cleanup deadline and reports whether the process was already
exited, tree-killed, fallback-killed, or could not be terminated; Linux uses a
bounded process-group cleanup path. No exit, drain, kill, or cleanup path waits
without a deadline. Start failure, nonzero exit, timeout, output overflow,
termination failure, or cleanup failure is `FAIL`. Static failure prevents
launch and records `StaticValidationFailed`; a nonnative target records
`NonNativeTarget`.

## Structured evidence

Platform validators emit one compact structured result block with status,
artifact identity, platform metadata, warning count, error count,
`RuntimeExecution`, `RuntimeFailureReason`, `HelpContract`,
`HelpSkipReason`, and `HelpFailureReason`. `cmd/releaseaudit` writes strict
typed JSON reports for archive inventory, reproducibility comparison, and leak
scans.
These Go report types and their focused tests are the canonical report
contract. A parallel JSON expected-metadata file or a second report schema
would duplicate the current typed implementation and is therefore not used.

Unknown CLI arguments and missing required values fail closed. Shared build
fixtures cover valid and invalid SemVer and source-time values. Embedded
manifest tests reject missing, duplicate, and unknown fields. Archive tests
cover traversal, links, special entry types, duplicate paths, incomplete
packages, checksum mismatch, reproducibility mismatch, and leak findings.
`scripts/Test-VerifierProcess.ps1` and
`scripts/test-verifier-process.sh` permanently drive controlled nonzero
compact/verbose/help, empty/partial help, timeout, stdout/stderr-limit, start,
and cleanup failures through the complete production verifiers. They assert
structured state, error aggregation, and final exit behavior. Static-failure
cases assert both runtime and help skip states and marker absence. Child
cleanup requires an explicit `READY` marker and confirms process exit plus the
continued absence of the delayed survivor marker.

## Acceptance evidence

Validate each applicable binary and archive against the current build identity.
Record native execution separately from cross-build and static inspection; a
cross-built ARM64 binary is not evidence of native ARM64 validation. Keep
run-specific results and independent review records as external evidence, not
as a second current task queue in this document.

Generated binaries, archives, extracted trees, and validation reports are not
repository source. The authoritative task status remains in `BACKLOG.md`.

## Related documentation

- [Build and release metadata](build-metadata.md)
- [Manual metadata validation](metadata-validation.md)
- [Testing](testing.md)
