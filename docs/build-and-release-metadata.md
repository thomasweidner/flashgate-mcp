# Build and release metadata

## Purpose

FlashGate MCP derives product, version, platform, and source metadata from one canonical build-information model. The same values are used by the CLI, Windows `VERSIONINFO`, Linux Go/ELF inspection, release artifact names, checksums, and CI validation.

This metadata identifies the software build. It is not a code signature and does not replace Authenticode, package signatures, repository signatures, or transport security.

## Product identity

| Field | Value |
|---|---|
| Product | `FlashGate MCP` |
| Binary | `flashgate-mcp` |
| MCP implementation name | `flashgate` |
| File description | `FlashGate MCP Server` |
| Company/manufacturer | `Thomas Weidner` |
| License | `GNU General Public License v3.0` |
| Project | `https://github.com/thomasweidner/flashgate-mcp` |

The canonical constants and runtime representation are maintained in `internal/version`.

## Canonical build values

Controlled builds resolve these values once:

| Value | Source |
|---|---|
| Product version | exact `v<SemVer>` Git tag for releases, including any approved prerelease suffix; explicit SemVer for validation; `0.0.0-dev` for development |
| Windows file version | `MAJOR.MINOR.PATCH.0` |
| Commit | full 40-character Git revision |
| Source time | `SOURCE_DATE_EPOCH`, otherwise Git commit time |
| Time format | UTC RFC 3339 with `Z` |
| Modified state | Git working-tree state |
| Go version | running/building Go toolchain |
| Platform | public `windows/x64`, `windows/arm64`, `linux/x64`, or `linux/arm64` |
| Go target | internal `GOOS/GOARCH`, such as `windows/amd64` |

Release builds fail closed unless the exact version tag is checked out and the working tree is clean. Local validation builds may be dirty and report `Modified: true`.

SemVer and `SOURCE_DATE_EPOCH` use one versioned fixture contract in
`internal/version/testdata/build-input-validation-fixtures.json`. Numeric
prerelease identifiers with leading zeroes and version components above
`65535` are rejected. `SOURCE_DATE_EPOCH` accepts decimal digits only and is
bounded to `0..253402300799`, the canonical UTC range through
`9999-12-31T23:59:59Z`; signs, whitespace and overflow fail closed. Go,
PowerShell, Bash and workflow input validation execute the same fixtures.

## CLI metadata

Compact output is stable and script-friendly:

```text
flashgate-mcp 1.2.3
```

Detailed output is available through:

```text
flashgate-mcp --version --verbose
```

It reports product version, numeric file version, full commit, canonical source time, modified state, Go version, public platform, and Go target. Version commands terminate before MCP STDIO processing and do not contaminate the JSON-RPC stream.

## Windows metadata

Windows x64 and ARM64 binaries receive generated `VERSIONINFO` and the FlashGate application icon immediately before compilation.

Required fields include:

- `FileDescription`
- `FileVersion`
- `ProductName`
- `ProductVersion`
- `CompanyName`
- `LegalCopyright`
- `OriginalFilename`
- `InternalName`
- `Comments`

The resource generator is the pinned, vendored `github.com/josephspurrier/goversioninfo v1.7.0`. Generated `resource_windows_*.syso` files are temporary build products, are never committed, and are removed after the build, including error paths. Existing or stale resource files are rejected.

The application icon is derived from Font Awesome Free `bolt-lightning`, Classic Solid, version `7.2.0`. Attribution is retained in `THIRD-PARTY-NOTICES.md` and `assets/branding/README.md`.

Automated validation uses `scripts/Test-WindowsMetadata.ps1`. It parses PE
architecture and `VERSIONINFO`, reads the Go target from the binary, validates
the embedded canonical build manifest, and compares every normalized embedded
icon frame with the committed ICO by SHA-256. This static path covers ARM64
without execution on x64. Native execution starts only after every static
check passes and the target matches the actual Windows OS architecture.
Runtime and help subprocesses have deterministic time and output bounds,
concurrent stream draining, a separate bounded cleanup deadline, structured
tree/fallback termination outcomes, and fail-closed status. Static ARM64
validation does not execute an ARM64 binary on an x64 runner and does not use
Explorer thumbnail caches. A final Explorer property-sheet check is documented
separately.

The embedded build manifest is generated from the same resolved values as the
linker fields and contains version, numeric file version, full commit, source
time, modified state, GOOS, GOARCH and public architecture. It is validated
directly from each binary and is not an external trust sidecar.

## Linux metadata

Linux binaries use the same CLI values and retain standard Go build information and ELF notes.

Validation covers:

- the embedded canonical build manifest
- `go version -m`
- `go tool buildid`
- `file`
- `readelf -h`
- `readelf -n`
- `.note.go.buildid`
- VCS revision, time, and modified state
- target architecture and public architecture mapping

FlashGate does not add proprietary ELF metadata sections or extended attributes. Linux desktop file managers do not provide a portable equivalent of Windows Explorer `VERSIONINFO`. Product identity is therefore verified through the CLI, Go build information, ELF inspection, release archive name, README, license, and checksum.

`.deb`, `.rpm`, and systemd metadata remain intentionally deferred until those packaging or deployment formats are approved.

## Controlled builds

### PowerShell

```powershell
& {
    & ./scripts/build.ps1 `
        -GOOS windows `
        -GOARCH amd64 `
        -Version 1.2.3 `
        -OutputPath build/windows_x64/flashgate-mcp.exe
}
```

Linux cross-build from Windows:

```powershell
& {
    & ./scripts/build.ps1 `
        -GOOS linux `
        -GOARCH amd64 `
        -Version 1.2.3 `
        -OutputPath build/linux_x64/flashgate-mcp
}
```

### Bash

```bash
bash scripts/build.sh   --goos linux   --goarch amd64   --version 1.2.3   --output build/linux_x64/flashgate-mcp
```

Direct `go build` remains valid for development, but controlled scripts are required when release-grade metadata and validation are expected.

## Release artifacts

Release artifacts use public architecture names:

```text
flashgate-mcp_<version>_windows_x64.zip
flashgate-mcp_<version>_windows_arm64.zip
flashgate-mcp_<version>_linux_x64.tar.gz
flashgate-mcp_<version>_linux_arm64.tar.gz
```

Each archive contains exactly one top-level directory with:

- the platform binary
- `LICENSE`
- `README.md`
- `THIRD-PARTY-NOTICES.md`

Each archive has a sibling `.sha256` file. Archive creation and validation are implemented in:

```text
scripts/New-ReleaseArtifact.ps1
scripts/Test-ReleaseArtifact.ps1
scripts/new-release-artifact.sh
scripts/test-release-artifact.sh
```

Release archives are published only after filename, content, checksum, version, architecture, Windows resource, Linux build-information, and ELF checks pass.

ZIP and TAR.GZ validators reject absolute, traversal, backslash, duplicate and
normalization-colliding names before extraction. Only the exact required
directories and regular files are accepted; symlinks, hardlinks, devices,
FIFOs, sockets and other special types fail closed. Validation uses a private
temporary copy so a caller-controlled archive path cannot be exchanged between
checksum verification and extraction.

## Continuous validation

`.github/workflows/metadata-regression.yml` validates Windows and Linux for x64
and ARM64. Every matrix binary is built independently twice, compared
byte-for-byte and leak-scanned before its first build and JSON audit report are
uploaded. The existing CI workflow continues to run formatting, vet, unit
tests, coverage, lint, and JSON-RPC smoke tests.

`.github/workflows/release-build.yml` builds and validates tagged release
archives with canonical source values. Before any upload, each matrix path
performs build 1, metadata and archive validation, build 2, binary/archive/
checksum/inventory reproducibility comparison, and a release-content leak
scan. Machine-readable reproducibility and leak reports are uploaded with the
validated archive and checksum.

Stable, prerelease, and development expectations are stored in `internal/version/testdata/build-metadata-fixtures.json` and exercised by `internal/version/fixtures_test.go`.

The shared valid/invalid SemVer and epoch matrix is stored in
`internal/version/testdata/build-input-validation-fixtures.json`.

The canonical acceptance layers, report contracts, and fail-closed negative
cases are mapped in [Artifact verification](artifact-verification.md).

## Reproducibility and privacy

Controlled builds use `-trimpath`, canonical source time, and deterministic resource/archive generation. Release metadata must not contain:

- local absolute paths
- usernames
- hostnames
- tokens or credentials
- local build time

The Go and GNU build IDs are retained as standard toolchain metadata. Repeated
builds with identical inputs must have identical binary, archive, checksum and
normalized archive-inventory hashes.

`cmd/releaseaudit` scans binaries and every regular archive entry for
workspace/home/temp/repository paths, supplied usernames and hostnames,
GitHub-token and AWS-key shapes, credential-bearing URLs, and complete private
key blocks. The literal private-key header strings used by redaction logic are
reported as narrowly allowed static markers; only a complete key-shaped block
is a finding.

## Native WSL snapshot boundary

The Windows orchestrators create native-validation snapshots only from
`git ls-files -z` plus untracked, nonignored
`git ls-files --others --exclude-standard -z` entries. Ignored files are never
archived, and sensitive filenames cause failure even when ignored. Every input
must be a regular, single-link, non-reparse file inside the worktree. The
manifest records relative path, type, length and SHA-256.

The Ubuntu driver validates TAR paths, entry types, lengths and hashes before
copying the snapshot into the ext4 clone. Extraction uses file-descriptor-
relative, no-follow operations in a new private directory and verifies the
complete manifest afterward.

Native work roots are direct children of
`$HOME/.cache/flashgate-mcp-validation`. `FG_RUN_ID` uses the restricted
`^[A-Za-z0-9][A-Za-z0-9._-]{0,79}$` form. Creation and cleanup reject symlink
components and use descriptor-bound identity checks; cleanup cannot target the
base, `$HOME`, `/home` or `/`.
