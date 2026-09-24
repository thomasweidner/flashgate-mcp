# Changelog

All notable changes to this project will be documented in this file.

The format follows the spirit of [Keep a Changelog](https://keepachangelog.com/), and this project uses semantic versioning once releases begin.

## [Unreleased]

### Added

- Added explicit MCP `2025-11-25` discovery annotations for all eight filesystem tools. The exact annotation matrix includes explicit `false` members; annotations do not grant authorization or change server-side checks.
- Added task-bound scratch routing for PowerShell and Python validation producers, with explicit working roots, fail-closed path validation, and portable Windows/Linux checks.
- Added deterministic Windows and Linux shell-script validation, including PowerShell parsing, Bash syntax, encoding, line-ending, bounded-process, and cleanup checks.
- Added canonical build identity, embedded build manifests, Windows resources, Linux build metadata, and deterministic Windows ZIP and Linux TAR.GZ release artifacts for x64 and ARM64. Release validation checks contents, hashes, reproducibility, and host/credential leakage.
- Added Go coverage reports and separate Windows/Linux coverage gates, plus resource, latency, payload, catalog, and token-efficiency benchmarks.
- Added MCP `CallToolResult` wrapping and `structuredContent` for successful filesystem tool results, with runtime `outputSchema` for all eight tools.
- Added strict startup root validation and a read-only activation/rollback guide for local STDIO clients.
- Established a public, self-contained repository boundary for build, test, release, contribution, and product documentation.

### Changed

- Switched coverage to the deterministic production-server package graph and raised the separate Windows/Linux hard gates to 90.0%, with an exact unrounded threshold check and package inventories.
- Converged public documentation and the documentation gate around current filesystem capabilities, Version 1.0 targets, portable contributor guidance, and durable repository-boundary checks.
- Kept the current MCP runtime at `2025-11-25`; Version 1.0 planning targets a separate `2026-07-28` path without advertising it before implementation and compatibility tests.
- Maintained the eight-tool filesystem baseline: `list_directory`, `read_file`, `get_path_info`, `write_file`, `create_directory`, `delete_path`, `copy_path`, and `move_path`.
- Clarified the Version 1.0 boundary: `Planned` tasks are required for the initial stable release, while `Later` tasks are accepted post-Version-1.0 work.
- Kept source and build tooling in the public checkout without requiring a private development control plane.

### Planning

- Planned reusable content identities under BL-048 as non-authorizing evidence, Version 1.0 `compare_paths`/`verify_paths` under BL-346, and post-Version-1.0 conditional retrieval and optional transparent caches under BL-217.
- Planned Version 1.0 named-root discovery, command discovery, bounded search, managed processes, typed command execution, system information, safe capability profiles, payload-efficient results, and service-account-root operation.
- Documented future Platform Adapter and Native Tool Adapter boundaries, cloud/placeholder storage behavior, transparent path compression/encryption, content compression, extended metadata, and one optional portable FlashGate agent skill.
- Kept cloud/placeholder storage, conditional retrieval, transparent path compression/encryption, and the portable agent skill post-Version-1.0.
- Planned one canonical product-version source under BL-245 so feature merges carry their SemVer change in the same product change; documented post-merge closure of superseded PRs and safe obsolete-branch cleanup without deleting historical merged PR records.
- Moved BL-260 and BL-245 from the final release-gate sprint to the start of SPR-048 and bound the prerequisite order `BL-260 -> BL-245 -> BL-203`, so product-code coverage and canonical versioning are established before further functional implementation.

### Fixed

- Corrected successful tool responses to use MCP `CallToolResult` after strict clients rejected unwrapped domain objects.
- Improved root confinement, traversal and reparse handling, JSON-RPC validation, hard limits, redacted diagnostics, and read-only tool registration.
- Corrected coverage summary status on failed thresholds and kept benchmark soft-budget messages out of general warnings.

### Security

- Startup rejects missing, relative, nonexistent, or disallowed roots before tool registration; development use of the current directory requires explicit opt-in.
- Filesystem access remains root-confined, with hidden, UNC, symlink, junction, and reparse policies enforced server-side. Standard output remains reserved for JSON-RPC.
