# File and product metadata decisions

## Purpose

This document defines the durable product, architecture, and implementation
decisions for FlashGate MCP file and product metadata on Windows and Linux.
It is separate from the implementation backlog and remains product
documentation after the feature is completed.

<!-- FP-DECISION-PRODUCT-IDENTITY:BEGIN -->
## DEC-FP-001 - Publisher, copyright, and application icon

**Status:** Accepted
**Date:** 2026-07-21

### Decision

| Property | Required value |
|---|---|
| Publisher / `CompanyName` | `Thomas Weidner` |
| Copyright holder | `Thomas Weidner` |
| Application icon | Font Awesome `bolt-lightning` |
| Font Awesome style | Classic Solid |
| Icon usage | Windows application icon and other technically suitable release presentations |

### Copyright format

Release builds use this base format:

```text
Copyright © <year> Thomas Weidner
```

The year or year range is derived from reproducible source or release time,
not from the local build clock. DEC-FP-002 and DEC-FP-003 define the time-source
and build implementation rules.

### Icon source and processing

- Use the Font Awesome `bolt-lightning` icon in the Classic Solid style.
- Pin the Font Awesome version before adding the source asset.
- Use a version-pinned vector source.
- Do not include font files in the repository or release artifacts.
- Convert the vector image deterministically to the required Windows ICO sizes.
- Document the source, version, license, and any required attribution.
- Normal and reproducible builds must not require network access to generate the icon.

### Exclusions

This decision does not introduce Authenticode signing, trademark registration,
a change to the MCP implementation name, a change to the binary name, or a
change to the software license.

### Backlog ownership

Primary owner: `BL-246`.
<!-- FP-DECISION-PRODUCT-IDENTITY:END -->

<!-- FP-DECISION-IDENTITY-VERSION:BEGIN -->
## DEC-FP-002 - Product identity, versioning, and reproducible build time

**Status:** Accepted
**Date:** 2026-07-21

### Product identity

| Property | Required value |
|---|---|
| Product name | `FlashGate MCP` |
| Binary name | `flashgate-mcp` |
| Windows original filename | `flashgate-mcp.exe` |
| Internal name | `flashgate-mcp` |
| MCP implementation name | `flashgate` |
| File description | `FlashGate MCP Server` |
| Publisher | `Thomas Weidner` |
| Copyright holder | `Thomas Weidner` |
| License | `GNU General Public License v3.0` |
| Project URL | `https://github.com/thomasweidner/flashgate-mcp` |
| Comments | `Native Model Context Protocol server for controlled local system access.` |
| Windows resource language | English - United States (`0x0409`) |
| Windows character set | Unicode (`1200`) |

This feature does not change the MCP implementation name `flashgate` or the
binary name `flashgate-mcp`.

### Canonical version source

- Root `VERSION` is the only editable canonical product-version source in the repository. It contains exactly one SemVer value without a leading `v`.
- Release tags use `v<VERSION>` and must point to the exact commit being built. Tags establish parity; they do not supply the product version.
- Prerelease tags may contain a SemVer suffix, for example `v0.5.0-rc.1`.
- The leading `v` belongs to the Git tag, not the embedded product version.
- Ordinary uncontrolled builds use `0.0.0-dev` regardless of tags. Controlled builds use `VERSION`.
- Explicit SemVer overrides are restricted to controlled metadata or regression-test inputs. For a release, an override may at most assert the expected value of `VERSION`.
- `CHANGELOG.md` is the only manually maintained narrative release-notes source. Published release notes are derived from it.
- Freely entered release-version values are not a canonical source.

### Semantic Versioning

Product and release versions follow Semantic Versioning. Examples of complete
product versions are `0.5.0`, `0.5.0-rc.1`, and `0.5.0+build.1`. Reject invalid
SemVer values before a release build.

### Windows file version

The numeric Windows file version contains four components:
`Major.Minor.Patch.0`. Product version `0.5.0-rc.1`, for example, maps to
numeric file version `0.5.0.0`.

Prerelease and build suffixes appear only in `ProductVersion` and other
applicable textual version fields. The fourth numeric component remains `0`
for this feature.

### Git revision and dirty state

Use the full 40-character Git commit SHA internally. Compact human-readable
output may show the first twelve hexadecimal characters.

Represent dirty state in a separate Boolean field named `Modified` or its
equivalent. Dirty state must never become part of the numeric Windows file
version. A release build with unexpected dirty state must fail closed.

### Canonical time source

- All embedded build and source timestamps use UTC.
- The canonical text format is RFC 3339 with a `Z` suffix, for example `2026-07-21T16:38:27Z`.
- When `SOURCE_DATE_EPOCH` is set, it is authoritative.
- Otherwise, use the Git commit time of the commit being built.
- Do not use the build host's local clock as the canonical binary timestamp.
- Do not embed a fixed `GMT+1` or `CET` offset; it does not represent daylight-saving periods reliably.
- Human-readable reports may additionally show local time when the time zone and UTC offset are explicit.

### Reproducibility and privacy

Identical source, version, and time inputs must produce identical product
metadata. Do not embed hostnames, usernames, local working-directory paths,
OneDrive paths, or other machine-specific values in release metadata.

Identical rebuilds must not obtain new build timestamps from the local clock.
Windows resources, CLI output, Go build information, release artifact names,
and future package metadata must use the same version.

### Technical decisions completed by DEC-FP-003

DEC-FP-003 resolves the following implementation questions that remained open
when DEC-FP-002 was accepted:

- the Windows resource generator;
- whether `.syso` files are committed or generated only;
- the relationship between linker values and Go `-buildvcs`;
- the Go and ELF build-ID strategy;
- the scope of `.deb`, `.rpm`, and systemd artifacts;
- the CLI format for verbose or JSON output.

### Backlog ownership

Primary owner: `BL-246`. Shared supporting scope: `BL-247`.
<!-- FP-DECISION-IDENTITY-VERSION:END -->

<!-- FP-DECISION-TECHNICAL-IMPLEMENTATION:BEGIN -->
## DEC-FP-003 - Metadata, build, and architecture implementation strategy

**Status:** Accepted
**Date:** 2026-07-21

### Single source of truth

Resolve all build parameters once, centrally. Canonical values include the
product version, Windows file version, full commit SHA, UTC source time,
dirty state, GOOS, and GOARCH. These values supply Go linker parameters, CLI
output, Windows resources, release artifact names, and future package metadata.

Go VCS data is an independent provenance and consistency source. Controlled
release builds must reject contradictions between explicit build values and
Go VCS data.

A machine-readable build manifest embedded directly in the binary statically
binds the same values. It is also the architecture-independent provenance
source for Windows ARM64 when the binary cannot run on the validating x64 host.

### Build and runtime information

Product identity, product version, commit, and source time are build data.
Read the Go version, GOOS, and GOARCH from the Go runtime or
`debug.ReadBuildInfo`. Do not embed hostnames, usernames, local paths, or other
host information.

### Shared build-information schema

| Field | Meaning |
|---|---|
| `ProductName` | `FlashGate MCP` |
| `BinaryName` | `flashgate-mcp` |
| `Version` | Complete SemVer product version |
| `FileVersion` | Four-component numeric Windows version |
| `Commit` | Full Git SHA |
| `SourceTime` | RFC 3339 timestamp in UTC |
| `Modified` | Dirty state |
| `GoVersion` | Go toolchain used |
| `GOOS` | Technical Go target operating system |
| `GOARCH` | Technical Go target architecture |
| `PublicArch` | User-facing architecture identifier |
| `BuildManifest` | Static binding of canonical build values in the binary |

### Development defaults

- Version: `0.0.0-dev`.
- Windows file version: `0.0.0.0`.
- Commit: `unknown` when no linker or Go VCS data is available.
- Source time: `unknown` when no linker or Go VCS data is available.
- Determine the platform and Go version at runtime.

### Windows resource generator

Use `github.com/josephspurrier/goversioninfo`, pinned to `v1.7.0`. Do not use
`@latest` in build or release workflows.

A repository-owned generator wrapper creates the resource from centrally
resolved build parameters. Neither `rc.exe` nor GNU `windres` is mandatory.
Normal builds do not require network access.

### Windows resource fields

| Field | Value |
|---|---|
| `FileDescription` | `FlashGate MCP Server` |
| `FileVersion` | Numeric four-component version |
| `ProductName` | `FlashGate MCP` |
| `ProductVersion` | Complete SemVer version |
| `CompanyName` | `Thomas Weidner` |
| `LegalCopyright` | Reproducible copyright text |
| `OriginalFilename` | `flashgate-mcp.exe` |
| `InternalName` | `flashgate-mcp` |
| `Comments` | `Native Model Context Protocol server for controlled local system access.` |

Do not use `PrivateBuild`, `SpecialBuild`, `LegalTrademarks`, or similar
additional fields without a concrete requirement.

### Icon and branding artifacts

Commit the version-pinned Font Awesome SVG and the ICO generated from it as
verified branding artifacts. Do not include font files. Document the source,
version, license, attribution, and hashes.

The ICO is version-independent and is not regenerated during every normal
build. Verification normalizes every embedded icon frame and compares its
descriptors and SHA-256 values with the committed ICO.

### `.syso` lifecycle

Do not commit `.syso` files. Generate them immediately before a Windows build.
Filenames include the target operating system and technical Go architecture,
for example `resource_windows_amd64.syso` or `resource_windows_arm64.syso`.

Reject unexpected or stale `.syso` files. Remove generated files even when a
build fails. Exclude generated `.syso` files through `.gitignore`.

### Go VCS strategy

Direct local developer builds use `-buildvcs=auto`. Controlled script, CI, and
release builds use `-buildvcs=true`. Reject release builds with dirty state.

Explicit build values are the user-facing source. Use Go VCS data for fallback,
provenance, and consistency checks. Generate the static build manifest from the
same validated values as CLI and linker fields, and inspect it directly in
Windows and Linux binaries. An unverified text sidecar is not a provenance
source.

### Go and ELF build IDs

Retain Go's default build ID. Do not supply a custom `-buildid` string or use
`go tool buildid -w` in the release process.

Inspect Windows and Linux artifacts with `go tool buildid`. Additionally run
`readelf -n` natively on Linux. Do not force an additional GNU build ID.

### CLI

`flashgate-mcp --version` returns `flashgate-mcp <version>`.

`flashgate-mcp --version --verbose` returns the product, version, file version,
commit, source time, dirty state, Go version, public platform, and technical
Go target. This feature does not implement JSON version output.

Example for x64:

```text
Platform:  windows/x64
Go target: windows/amd64
```

Example for ARM64:

```text
Platform:  windows/arm64
Go target: windows/arm64
```

### Internal and public architecture identifiers

| GOARCH | Public identifier | User-facing description |
|---|---|---|
| `amd64` | `x64` | 64-bit Intel or AMD processors |
| `arm64` | `arm64` | 64-bit ARM processors |

Internal Go code, build scripts, and `.syso` filenames retain Go's `amd64` and
`arm64` values. Public filenames, download descriptions, and user-facing output
use `x64` and `arm64`. Do not use `x86_64` or `aarch64` as public FlashGate
identifiers.

### Release artifacts

| Target | Artifact name |
|---|---|
| Windows x64 | `flashgate-mcp_<version>_windows_x64.zip` |
| Linux x64 | `flashgate-mcp_<version>_linux_x64.tar.gz` |
| Windows ARM64 | `flashgate-mcp_<version>_windows_arm64.zip` |
| Linux ARM64 | `flashgate-mcp_<version>_linux_arm64.tar.gz` |

Do not include the tag's leading `v` in the product version or archive name.
Archives contain the binary, `LICENSE`, `README.md`, and
`THIRD-PARTY-NOTICES.md`. Publish SHA-256 checksums alongside the archives.

Before every upload, compare two independent builds, including the binary,
archive, checksum file, and exact inventory. A machine-readable leak scan of
all binary and archive contents must pass before upload.

### Linux packaging scope

Provide the native ELF binary and a versioned tarball. Defer `.deb` and `.rpm`
until there is an actual distribution requirement. Implement systemd metadata
with the later systemd service implementation. Extended attributes are excluded
from this packaging scope.

### Target architectures

The following table defines validation targets, not evidence that native ARM64
runs have already taken place. Current x64 validation can cross-build and
statically inspect ARM64 artifacts; only execution on an ARM64 host qualifies
as native validation. See [artifact verification](../artifact-verification.md)
for the current acceptance distinction.

| Go target | Public artifact | Native validation target | Initial status |
|---|---|---|---|
| `windows/amd64` | `windows_x64` | Windows x64 runner | Stable |
| `linux/amd64` | `linux_x64` | Ubuntu x64 runner | Stable |
| `windows/arm64` | `windows_arm64` | Windows ARM64 runner | Preview |
| `linux/arm64` | `linux_arm64` | Ubuntu ARM64 runner | Preview |

ARM64 artifacts may be cross-compiled without local ARM hardware. A successful
cross-build alone is not native validation. Use suitable ARM64 runners for
native ARM64 tests.

Promote ARM64 to Stable only after repeated successful release cycles and
stable runner availability. Add other architectures only for a concrete need,
with a separate native-validation strategy. Reassess cross-compilation if cgo
or native libraries are introduced later.

### Test matrix

- Windows x64 Stable and Linux x64 Stable.
- Windows ARM64 Preview and Linux ARM64 Preview.
- Uncontrolled development build with `0.0.0-dev`.
- Stable and prerelease SemVer.
- Clean and dirty working trees.
- Windows file-version mapping.
- Short and verbose CLI output.
- Windows `VERSIONINFO`.
- Go VCS and build-ID consistency.
- Native `readelf -n` on Linux.
- Shared SemVer and `SOURCE_DATE_EPOCH` fixture matrix.
- Tampered TAR type, traversal, and inventory fixtures.
- Tampered icon-identity fixture.
- Ordinary repository and linked worktree.

### Backlog ownership

Primary owner: `BL-246`. Shared supporting scope: `BL-247`.
<!-- FP-DECISION-TECHNICAL-IMPLEMENTATION:END -->
