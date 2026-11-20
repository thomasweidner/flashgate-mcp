# Changelog

All notable changes to this project will be documented in this file.

The format follows the spirit of [Keep a Changelog](https://keepachangelog.com/).
Notable development changes accumulate under Unreleased. Release preparation
assigns the product version and adds its dated notes in the same freeze commit.
Historical undated sections record prior repository versions. A date records
the intended release date. Publication evidence is separate.

## [Unreleased]

### Breaking Changes

- Replace `security.follow_symlinks` with a strict boolean in named policies
  and individual roots. The default remains false; only supported relative
  in-root links can be followed. Absolute links, junctions, unsupported reparse
  objects and unsafe mount transitions are denied. There are no legacy aliases
  or automatic rewrites. See [link-policy migration](docs/migration.md#per-root-link-policy).

- Use `_size` configuration keys for file writes, protocol messages/arguments/
  responses, IPC frames and logging records. Remove `max_copy_bytes`; one
  `max_write_size` bounds writes and copies. Old byte keys are rejected.
  See [resource migration](docs/migration.md#resource-size-settings).

- Filesystem roots default to read access. A write-capable profile still
  exposes mutation tools, and calls require explicit write access on the
  selected root. See [root access migration](docs/migration.md#root-access).

- STDIO starts with the `read` profile. Select `profiles.active = "write"`
  explicitly for the writable catalog. `files.read_only` and the seven
  retired capability spellings are rejected. See the [profile cutover](docs/migration.md#capability-profile-cutover).

- Configure named roots in `[files.roots.<id>]` and set `files.default_id`.
  Startup validates every root before enabling the filesystem tools.
  Replace the former `[filesystem]` table and CWD opt-in using the
  [migration guide](docs/migration.md#root-configuration).

- All eight filesystem tools require an explicit configured root ID for each
  relative path. Pass `root_id` and `path` for one operand, and
  `source_root_id`/`source` and `target_root_id`/`target` for copy or move.
  Copy and move require matching root IDs. Pass `.` explicitly to list the
  root. See [client input migration](docs/migration.md#input-and-operation-semantics).

### Changed

- Configure immutable file, write, scan, serialized-result and temporary-data
  limits globally, in named policies and per root. Size fields accept exact
  English SI/IEC byte/bit strings. Shared conversion and budget helpers are
  available to future consumers; startup never rewrites the configuration.
  Scan-bounded reads reject unproven completion, including virtual files whose
  reported size cannot establish EOF, rather than returning a truncated prefix.

- Define reusable named policies in TOML and override access per root.
  An empty access list denies reads and writes for the selected root ID.

- Resolve built-in and custom profiles to immutable grants and apply a
  separate restrictive risk ceiling. Admit effective catalogs through their
  measured wire bindings and preserve the selected configuration's protection.

- Protect the selected TOML configuration and its aliases from ordinary
  filesystem tool reads, copies and mutations. Bootstrap can accept a safely
  enforceable overlapping data root. The OS governs source permissions and
  provisioning.

- Define the internal functional capability model and successor acceptance
  for profiles, root policy, effective catalogs, domains, and providers.

- Runtime startup now requires one trusted TOML configuration file. This
  pre-1.0 breaking change replaces the old `MCP_*` runtime environment
  settings. Use the user default or pass `--config <absolute-path>`. See the
  [template](docs/flashgate-config.toml) and
  [migration and rollback guide](docs/migration.md#root-configuration).

### Validation

- Require exact framed MCP response sizes for deterministic success and error
  fixtures, with explicit error applicability and authorization coverage
  under the existing wire budget gate.

- Check shipped Go runtime dependency graphs for uncontrolled process launch
  and interpreter adapter paths. Require canonical backlog acceptance for
  future public tool adapters. Record the typed no-shell contract and the
  runtime enforcement obligations of its successors.

- Check effective tool count, name/title/description and schema costs on
  captured catalogs, and instruction bounds on MCP initialize and discovery
  responses. Bind future tool and profile owners to the same catalog contract.

- Check exact MCP wire bytes, semantic payload placement, observed response
  operations and hard budgets for registered variants in the ordinary Go
  test suite. Include candidate-policy and synthetic delivery fixtures.

- Validate registered MCP tool inputs and successful structured results offline
  with JSON Schema Draft 2020-12 in the existing BL-204 conformance gate. The
  test-only validator adds no runtime checks or wire changes.

- Add native MCP revision/profile conformance contracts, complete runtime/wire
  schema snapshots, completeness mutations and a focused reproducible report.
  The official suite remains unexecuted; the candidate revision stays disabled
  and product behavior and VERSION are unchanged.

### Release tooling

- Assign product versions at release preparation. Default script builds use
  `0.0.0-dev`. Candidate verification requires a new version, dated notes,
  and an exact freeze commit.

- Added an explicit manual pre-1.0 publication workflow and releaseaudit adapter consuming the existing verified four-target candidate and complete supply-chain bundle. Publication binds externally approved source, notes, evidence and asset hashes, stages a draft, transfers exact bytes once, downloads every asset before publishing, and confirms the final prerelease. Conflicts and failed or ambiguous writes hold for read-only reconciliation. This records local implementation, not an actual public release; product VERSION is unchanged.

### Added

- Bind link policy and native open/effect identity checks below MCP across all
  eight filesystem operations. Preserve configuration protection, per-root
  access, file types and resource limits. Linux uses `openat2` without a weaker
  fallback; Windows uses root/parent-relative content opens with no final
  reparse traversal. Resolve relative targets from their actual parents before
  applying `..`, and retain the complete identity chain behind directory aliases.
  The project minimum Go version is 1.27.4.

- Configure immutable per-root file-type restrictions with `allowed_extensions`
  and `allow_extensionless`, inherited independently from named policies.
  Missing settings preserve unrestricted extension matching; empty lists and
  explicit unrestricted overrides retain distinct semantics. All eight
  filesystem tools enforce the same policy below MCP, including native aliases,
  overwrite operands and bounded recursive preflights. Denied metadata is not
  reported as missing. No tool schemas, VERSION or release assets change.

- Respond to MCP `ping` on the `2025-11-25` STDIO path with an empty result
  and the original request ID under both tool profiles.

- Extend releaseaudit with deterministic unsigned SPDX source inventory, four
  artifact-bound provenance statements and a compact verified supply-chain index;
  reuse the candidate workflow and exact-byte promotion for consumer handoff and
  controlled rollback validation. Product VERSION remains unchanged.

- Added a read-only current-version candidate CI gate for Windows and Linux x64/ARM64, a strict four-target verification manifest, and local exact-byte promotion with immutable version/source/artifact identity. Hosted candidate evidence and public publication remain separate gates.
- Added an exact machine-readable MCP support matrix for currently advertised revisions; it lists only `2025-11-25` and does not activate `2026-07-28`.

### Changed

- Enforce typed MCP tool result contracts. `read_file` sends file text once in
  `content[0].text` and file size in `structuredContent.size`. It rejects
  invalid UTF-8 with `invalid_text_encoding`.
  `list_directory` sends entries once in `structuredContent.entries` and
  counts filtered entries toward `MCP_MAX_LIST_ENTRIES`.
  See the [client migration](docs/migration.md#successful-tool-results).

- Reworked README onboarding with prerelease downloads, selected-archive SHA-256 verification, Windows/Linux quickstarts, explicit read-only Codex and Claude Desktop configuration, and a first filesystem task. Corrected the public download procedure to use four platform archives plus shared SHA256SUMS while preserving publication history; product VERSION and release bytes are unchanged.
- Migrated human backlog and sprint status to the canonical six-status lifecycle while retaining Post-1.0 placement in a separate workstream table; added deterministic status, Markdown status-cell rendering, and placement validation.
- Standardized public documentation in English with stable topic-based filenames, consolidated client and migration guidance, and separated historical execution reports from product documentation.

### Planning

- Converged the inherited technical-BL implementation contract and living architecture traceability under BL-315; reordered the release and core-foundation lane, and defined exact-byte local pre-release promotion without changing product behavior or `VERSION`.
- Made backlog-item completion, rather than sprint end, the primary reconciliation point for backlog status, changelog/version, affected documentation, tests/CI, candidate artifacts, release/distribution, migration, and security impact. Intentionally deferred work must be carried by one concrete successor BL in both the closing and successor scopes.
- Moved current-version candidate-artifact and schema/payload/catalog CI owners `BL-255`–`BL-258` into `SPR-048`, with dependency-bound execution before later implementation relies on their contracts.
- Registered BL-363 in SPR-048 for planned persistent public pre-1.0 GitHub prereleases after BL-255 verified candidates and BL-262 supply-chain evidence. Publication preserves exact verified bytes, canonical version/tag/source/hash identity and frozen changelog notes, with `prerelease=true`, a separate minimal-permission publication boundary and remote readback. Stable Version 1.0 publication remains exclusively BL-263; transient Actions artifacts remain current validation evidence. This planning decision implements no publisher and changes no product version.
- Updated BL-330 from an already-resolved status decision to the remaining FlashGate-specific application and validation of the canonical project work-item status model.

## [0.3.0] - 2026-10-23

### Public prerelease scope

- The local STDIO server supports MCP `2025-11-25` with the eight existing filesystem tools: `list_directory`, `read_file`, `get_path_info`, `write_file`, `create_directory`, `delete_path`, `copy_path`, and `move_path`. Access remains subject to the configured root, path restrictions, read-only mode, and hard resource limits.
- The existing release-artifact formats target Windows and Linux x64/ARM64. Search, managed processes, system-service operation, and public activation of MCP `2026-07-28` are not part of this prerelease.

### Added

- Implemented an internal MCP `2026-07-28` stateless adapter candidate with per-request metadata validation, `server/discover`, revision-specific errors and results, and conservative private cache hints. The existing `2025-11-25` initialization path remains the production default. The new path is not publicly enabled or advertised; activation awaits BL-204, BL-212, and BL-219.

## [0.2.0]

### Changed

- Expected registered filesystem-tool failures now return MCP `isError:true` with compact `category`/`message` text JSON and no `structuredContent`. Request errors and unavailable tools remain JSON-RPC; unexpected failures use generic Internal error. Success output schemas and results are unchanged.

## [0.1.0]

### Added

- Established root `VERSION` as the sole editable product SemVer source (`0.1.0` seed), with exact release-tag parity and generated notes audited from this changelog. This entry does not mark `0.1.0` as released.
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
