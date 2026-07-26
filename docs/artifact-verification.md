# Artifact verification

FlashGate verifies release artifacts against the same canonical build identity
used by the executable, Windows resources, Linux build information, archive
names, and release workflows. BL-248 does not introduce a second version or
product-metadata source.

## Canonical expectation sources

The authoritative product constants and embedded build identity live in
`internal/version`. Controlled build inputs are validated by
`scripts/Build-InputValidation.ps1` and
`scripts/build-input-validation.sh`. The Windows resource generator, embedded
manifest verifier, icon verifier, release builders, and archive audit consume
those values instead of a separately maintained expected-metadata document.

Release validation uses the requested SemVer plus the source commit and source
time resolved by the controlled build or workflow. Public `x64` maps to Go
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

## Current review state

BL-248 is `Done`. Its one independent Full Review and the required focused
Delta Reviews are complete. All six original Full-Review findings are closed;
`BL248-REV-004` is `CLOSED`, and no BL-248 finding remains open. The final
Windows verifier-process contract suite passed `201/201`, the native Linux
suite passed `206/206`, and the independent focused review of the final
six-file correction returned `PASS`. CI Run 82 and Metadata Regression Run 11
both completed with `success`.

The completed acceptance evidence proves:

1. native Windows x64 metadata, version, and help validation;
2. Windows ARM64 cross-build and static PE/resource/manifest/icon validation;
3. native Linux x64 ELF, provenance, version, and help validation;
4. Linux ARM64 cross-build and static ELF/provenance validation;
5. exact ZIP and TAR.GZ inventory and checksums;
6. two-build reproducibility and leak gates;
7. focused negative cases for Windows, Linux, and archive inputs;
8. bounded execution, architecture, timeout, output-limit, cleanup, parser,
   shell syntax, targeted test, documentation, and diff gates.

Generated binaries, archives, extracted trees, and local verification reports
remain ignored validation output. They are not repository source.

PR #25 was merged on 2026-07-26 at
`a30d3ab4958af6c1df5015300817aac1b692fde9`. The final post-merge chain is
complete: review and CI correction passed, the pull request was merged,
durable evidence was preserved with a verified manifest and ZIP inventory,
and the local preparation workspace was removed only after that evidence
passed validation. BL-333/BL-334 are the next functional queue step; BL-251
and BL-324 remain not begun.

## Related documentation

- [Build and release metadata](build-and-release-metadata.md)
- [Manual metadata validation](manual-metadata-validation.md)
- [Testing](testing.md)
