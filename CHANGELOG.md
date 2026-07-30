# Changelog

All notable changes to this project will be documented in this file.

The format follows the spirit of [Keep a Changelog](https://keepachangelog.com/), and this project uses semantic versioning once releases begin.

## [Unreleased]

### Planning

- Defined the Version 1.0 release boundary: canonical `Planned` backlog tasks are required for the initial stable release and `Later` tasks are accepted post-Version-1.0 work.
- Adopted hybrid per-root service execution identity. Version 1.0 implements dedicated service-account roots (Variant A), defines backend-neutral contracts for later per-user workers (Variant B), and permanently excludes in-process impersonation (Variant C).
- Added the native multi-mode runtime plan for direct STDIO, proxy, auto, Windows SCM service, and Linux systemd service operation in one native binary. User-scoped persistent hosts and the per-user worker runtime remain post-Version-1.0.
- Added payload-class result contracts, single-transmission rules for large content, opaque identity-bound result resources, wire-amplification metrics, profile catalog/instruction budgets, deterministic catalog fingerprints, and bounded server instructions.
- Added Version 1.0 plans for per-principal quotas and fair scheduling, typed no-shell command definitions, native OS adapter precedence, audit lifecycle/correlation, MCP 2026 compatibility preparation, supply-chain evidence, and a pinned cross-project efficiency benchmark.
- Added ADR-0015, Version 1.0 scope, execution-identity backend, efficiency-improvement, comparative-review, runtime/service, protocol, specification, and roadmap documentation.
- Expanded the canonical backlog to the continuous range `BL-001` through `BL-335`; the current renumbering through `BL-324` is documented in [Backlog ID migration - 2026-07-20](docs/backlog-id-migration-2026-07-20.md), layered over the immutable earlier migration records. PR #16 adds `BL-325` through `BL-329`, the final PR #21 review adds `BL-330` through `BL-332`, BL-333/BL-334 provide governance foundation and enforcement, and INF-121 registers BL-335 without changing existing canonical IDs.
- Migrated current sprint identifiers to standalone `SPR-N` values. Former suffix-based entries now have independent IDs, subsequent entries shift without changing backlog assignments, and the complete mapping is recorded in [Sprint ID migration - 2026-07-25](docs/sprint-id-migration-2026-07-25.md).

### Migration

- See [CallToolResult contract migration](docs/call-tool-result-contract-2026-07-12.md). Clients must read successful domain values from `structuredContent` or parse the compact JSON text block; direct domain fields no longer occupy the outer JSON-RPC `result`.

- See [fail-closed root configuration](docs/fail-closed-root-configuration-2026-07-11.md). Clients must set an explicit absolute root; development CWD use requires the new opt-in.
- See [filesystem tool contract cleanup](docs/filesystem-tool-contract-cleanup-2026-07-11.md). The old pre-1.0 tool names were removed without aliases; clients and smoke tests must use the eight-tool baseline.
- `SPR-43` does not change `MCP_*` names, root defaults, capability policy, or the MCP protocol revision.
- See [technical rename to FlashGate](docs/technical-rename-to-flashgate-2026-07-11.md).
- No MCP tool-contract or runtime-security change was made; existing `MCP_*` variables remain unchanged.

### Added

- Added the BL-333 governance foundation: canonical change-trigger,
  finding-remediation/review-mode, and Classic-readiness standards, a complete
  machine-readable trigger catalog, and an assignment-record schema.
- Added BL-334 fail-closed enforcement with diff-derived triggers, checkpoint,
  boundary, same-run finding, focused-delta, actual-package handoff,
  commit-preparation, trusted Git/workflow provenance, strict assignment and
  completion-report schemas, ephemeral CI/release records, and a 198-case
  production-path positive/negative fixture matrix. The second focused
  correction binds the actual `correction-only.patch` and
  `current-delta.patch` bytes to separately trusted expected hashes, adds
  strict correction/regression/focused/report schemas, and enforces exact
  finding, repository, external-governance, status, and queue parity. The third
  bundled correction closes the remaining REV-013/REV-015 implementation gap:
  exact canonical Windows external path-to-scope mappings, a schema-bound
  HANDOFF contract produced from one typed status source, visible/JSON status
  parity, and re-manifested negative fixtures for every added boundary. The
  fourth bundled correction makes visible HANDOFF parity complete: it strictly
  parses exactly 16 typed keys from the shared producer source, independently
  counts and orders all four status/contract markers, rejects reserved control
  lines outside the block, binds focused-review commit authorization to false,
  and adds 18 re-manifested productive negative cases that fail their specific
  gates. The subsequent focused independent Delta Review passed, closes
  REV-013 and REV-015 alongside the three previously closed findings, records
  RUN-007 as independently reviewed, and makes BL-333/BL-334 technically ready
  for separately authorized bounded Commit Preparation without authorizing
  Commit Preparation or commit. The focused Hosted-CI correction adds a
  byte-bound repository execution mirror of the external Classic artifact
  validator, makes the fixture default script-relative with fail-closed local
  canonical-byte parity, and passes the mirror explicitly in Windows CI without
  changing runtime or product logic. The exact-head correction moves governance
  into a separate Windows job that checks out the pull-request head SHA instead
  of the synthetic merge commit, proves workflow-source blob parity before
  binding workflow provenance, and downloads the official PowerShell 7.6.4
  Windows x64 ZIP only from its versioned release URL after verifying SHA-256
  `80832551C52809301E6071C8BAC977BEB5A2F1EC953EB4DB9F94DEB953333793`.
  The productive validator and the unchanged 198-case fixture matrix fail
  closed on a different interpreter version or package digest; the existing
  Windows/Linux Go matrix continues to test the merge result.
- Added one canonical build-information model and embedded machine-readable manifest for compact and verbose CLI identity, Windows resources, Linux Go/ELF metadata, and release artifacts.
- Added deterministic Windows x64/ARM64 `VERSIONINFO` and the Font Awesome-based FlashGate application icon, with vendored resource tooling and third-party notices.
- Added native Linux x64 and cross-compiled Linux ARM64 metadata validation, including Go/VCS information, ELF headers and Go build IDs.
- Added deterministic Windows ZIP and Linux TAR.GZ artifacts for x64 and ARM64; every release matrix path now builds twice and gates upload on exact contents, SHA-256, binary/archive/checksum/inventory reproducibility, and machine-readable host/credential leak checks.
- Added tag-gated release and metadata-regression workflows plus permanent build, validation, and decision documentation.
- Added one shared SemVer/`SOURCE_DATE_EPOCH` fixture contract, linked-worktree-aware Bash builds, fail-closed native validation roots, and manifest-verified Git inventory snapshots with traversal- and special-entry-safe TAR extraction.
- Completed and merged BL-248 artifact verification through PR #25 on 2026-07-26 at `a30d3ab4958af6c1df5015300817aac1b692fde9`. CI Run 82 and Metadata Regression Run 11 succeeded; the final Windows contract suite passed `201/201`, the final native Linux contract suite passed `206/206`, and all six original Full-Review findings, including `BL248-REV-004`, are closed with no open BL-248 finding. The completed contracts are documented in [Artifact verification](docs/artifact-verification.md).
- Added `scripts/Test-GoCoverage.ps1`, separate Windows/Linux text, HTML, profile, log, and JSON coverage reports, 14-day per-platform CI artifacts, README guidance, and `docs/development/code-coverage.md`.
- `SPR-47` adds the development-only `cmd/benchmark` STDIO runner, Win32 and Linux procfs process metrics, ten machine-readable reference workflows, result schema `flashgate-benchmark/v1`, local budget evaluation, and diagnostic PowerShell/Bash launch scripts. Authoritative platform baselines use a separate two-phase prebuilt controller after Windows and native Linux preparation on the same clean implementation commit.
- `SPR-45` adds explicit `TextContent` and `CallToolResult` protocol DTOs, a strict project-local decoder with legacy unwrapped negative fixtures, full success/error wire tests, and reproducible tool-result serialization benchmarks.
- A dated benchmark baseline records historical, text-only, and text-plus-structured payload/runtime/allocation costs without CI budgets.

- `SPR-44` categorized startup configuration errors, exit-code tests, Windows/Linux startup-negative smokes, and a Codex read-only activation/rollback guide.
- FlashGate MCP project identity and transition documentation.
- `SPR-41` architecture baseline.
- Vendor-neutral open-source and FlashGate module/provider direction, separated from MCP protocol extensions.
- Accepted Operations and Job Manager target architecture.
- ADRs for project identity, domain-separated core, deployment/modules/providers, capability profiles, operations/jobs, managed processes/commands, resource/token efficiency, and MCP version/extension compatibility.
- Backlog ID migration documentation for the continuous `BL-001` sequence.
- `SPR-38` JSON-RPC request validation and error behavior hardening.
- `SPR-39` configurable hard limits, redacted diagnostics, and secrets-aware behavior.
- `SPR-40` Windows/Linux JSON-RPC smoke-test matrix coverage.
- `SPR-37` hidden, UNC, symlink, junction, and reparse policy enforcement.
- `SPR-36` root, realpath, and traversal hardening for filesystem access.
- `SPR-35` read-only tool capability gating for filesystem MCP tools.
- Initial Go module setup.
- Project structure for a professional MCP server implementation.
- Immutable configuration package.
- Environment-based configuration loading.
- Secure path validation with `PathGuard`.
- `SafePath` abstraction for validated filesystem paths.
- Local filesystem abstraction.
- Filesystem support for:
  - list
  - read
  - stat
  - exists
  - write
  - mkdir
  - delete
  - move
  - copy
  - rename
- MCP protocol types for JSON-RPC and MCP messages.
- MCP server loop for JSON-RPC over STDIO.
- MCP initialize handler.
- MCP router and handler abstraction.
- MCP tool registry.
- MCP `tools/list` support.
- MCP `tools/call` support.
- JSON-RPC envelope validation for protocol version, method shape, IDs, notifications, unsupported batches, and method-specific params.
- Configurable limits for JSON-RPC messages, tool arguments, write payloads, list entries, copy source size, recursive delete entries, and response size.
- Central diagnostics redaction for common tokens, credentials, private-key markers, connection strings, and host paths.
- Negative JSON-RPC smoke test coverage for malformed JSON, unknown methods, invalid `tools/call` params, and notification no-response behavior.
- Bash negative JSON-RPC smoke test script for Linux CI.
- Deterministic MCP tool discovery order.
- Filesystem MCP tools:
  - `list_files`
  - `read_file`
  - `stat_path`
  - `exists_path`
  - `write_file`
  - `mkdir`
  - `delete_path`
  - `move_path`
  - `copy_path`
  - `rename_path`
- Unit tests for configuration, security, filesystem, protocol, router, transport, server, initialize, tools, CLI, version, and bootstrap packages.
- Package documentation files.
- Human-readable MCP tool documentation in `docs/tools.md`.
- Tool implementation conventions in `docs/tool-conventions.md`.
- Machine-readable MCP tool catalog in `docs/mcp-tool-catalog.json`.
- PowerShell scripts for build, lint, and test workflows.
- Windows JSON-RPC smoke test script in `scripts/smoke-jsonrpc.ps1`.
- Linux/macOS JSON-RPC smoke test script in `scripts/smoke-jsonrpc.sh`.
- GitHub Actions CI workflow for formatting, vetting, tests, linting, and build validation.
- Windows and Ubuntu CI JSON-RPC smoke test execution for default, read-only, and negative smoke variants.
- Manual GitHub Actions release build workflow.
- Release build artifacts for Windows and Linux.
- Release artifact summary and retention configuration.
- Version metadata package.
- Build metadata embedding through linker flags.
- `--version` CLI mode.
- `--help` and `-h` CLI modes.
- CLI argument validation with dedicated invalid-argument exit behavior.
- GNU General Public License v3.0 license file.
- Project backlog in `BACKLOG.md`.

### Changed

- Registered the three accepted non-blocking Minor findings from the final independent PR #21 review as canonical follow-up tasks `BL-330` through `BL-332`; the findings remain unfixed and no follow-up implementation is included in this registration-only change.
- BL-316 makes versioned benchmark artifact validation authoritative: Windows and Linux artifacts are strictly decoded, independently reevaluated against complete fail-closed hard/soft budget definitions, compared exactly with embedded budget results, and rejected before cross-platform comparison on any hard failure; matching soft warnings remain review-only through the complete platform gate, while general runner warnings remain separately fatal.
- Registered the three Minor findings and two Notes from the independent PR #16 review as canonical post-merge tasks `BL-325` through `BL-329` without implementing them in the Major-finding correction.
- CI now enforces separate repository-wide Go statement-coverage gates of 71.4% on Windows and 70.6% on Linux. The values are evaluated independently, and `summary.json` reports threshold failures consistently as `FAIL`.
- Non-authoritative benchmark output now binds the validated parent through `os.Root`, writes and syncs an exclusive temporary file through that stable handle, and publishes with a handle-relative rename that never follows the final target; all diagnostic names, final symlinks/reparse points, protected-directory aliases, and late output/protected-directory exchanges fail closed.
- Registered the eight Major/Minor findings deferred from the independent PR #15 review as canonical post-merge tasks `BL-316` through `BL-323` without implementing them in the blocker fix.
- `SPR-47` keeps functional serialization coverage active under race while evaluating allocation budgets only without race instrumentation; legacy baseline-record flags now fail closed in favor of the authoritative two-phase prebuilt workflow.
- `SPR-47` physically resolves Windows diagnostic output parents to block junction/reparse aliases into versioned baseline paths and validates both platform artifacts together across provenance, budgets, resources, process outcomes, and deterministic fields.
- `SPR-47` review corrections enforce clean-only versioned baselines, complete MCP initialization with `notifications/initialized`, validate exact profile/workflow measurement sets, activate all six serialization budgets, preserve partial Linux metrics, harden host-path redaction and cleanup, and define ordinary reads as `read_bytes` without `scanned_bytes`.
- `SPR-47` establishes a reproducible Windows/Linux resource, startup, latency, payload, filesystem-counter, call-count, and approximate-token benchmark baseline without changing public MCP tool contracts.
- Existing tool-result serialization fixtures now retain their historical/text/text-plus-structured measurements while also pinning full JSON-RPC response bytes; direct `tools/call` and both `tools/list` profiles have dedicated in-process benchmarks.
- Deterministic wire/counter regression budgets are hard local gates, while startup, latency, RSS/working-set, and CPU budgets remain soft review warnings; full CI benchmark execution and comparison stay deferred to BL-249 and BL-250.
- `SPR-46` exposes runtime `outputSchema` for all eight filesystem tools, with deep parity to catalog `resultSchema` and no changes to tool names or successful domain results.
- `SPR-45` wraps every successful filesystem `tools/call` result in an MCP 2025-11-25 `CallToolResult` with one compact JSON `TextContent` block and the same domain object in `structuredContent`.
- All eight filesystem tools use one central adapter wrapper; internal filesystem and domain result types remain protocol-independent.
- Windows and Bash positive STDIO smokes now strictly validate the outer `CallToolResult` and assert domain values through `structuredContent`.
- The existing safe JSON-RPC tool-error contract remains unchanged; normalized `isError=true` migration stays planned under BL-203.
- Runtime output schemas describe successful `structuredContent` only; the existing JSON-RPC error contract remains unchanged.
- `SPR-44` requires an explicit absolute `MCP_ROOT`; missing, empty, whitespace-only, and general relative roots now fail closed before tool registration or JSON-RPC processing.
- `MCP_ROOT=.` now requires the explicit lowercase `MCP_ALLOW_CWD_ROOT=true` development opt-in and emits one safe stderr warning.
- Root startup validates existence, policy, canonical resolution, and directory type before exposing tools.
- Codex, Claude Desktop, and general STDIO read-only activation examples now require `MCP_READ_ONLY=true` and remain preparation only.
- `SPR-43` replaces the pre-1.0 filesystem tool contract with the exact baseline `list_directory`, `read_file`, `get_path_info`, `write_file`, `create_directory`, `delete_path`, `copy_path`, and `move_path`.
- Tool arguments are decoded strictly: unknown properties, trailing JSON values, wrong types, missing required fields, and blank required paths are rejected.
- `get_path_info` reports genuine missing paths as successful `exists:false` results, `create_directory` reports the actual `created` state, and `move_path` safely covers same-volume move and rename.
- `copy_path` is explicitly file-only; directory copy remains planned work.
- Repository renamed to `flashgate-mcp`; the later owner migration moved the active repository to `thomasweidner/flashgate-mcp`.
- Go module and internal imports now use `github.com/thomasweidner/flashgate-mcp`; former owner paths remain only in historical migration context.
- Binary renamed to `flashgate-mcp`; scripts, workflows, release artifacts, tests, and documentation updated.
- MCP server implementation name (`serverInfo.name`) changed to `flashgate`.
- Local project folder, clone instructions, and remote instructions updated.
- Public project name changed from Fileserver MCP to FlashGate MCP; the technical repository, module, binary, MCP server implementation name (`serverInfo.name`), scripts, workflows, and catalog remained unchanged until `SPR-42`.
- Backlog consolidated into one canonical continuously numbered task catalog without a separate `BL-D` series.
- Former `SPR-41` Codex read-only preparation shifted to `SPR-44`.
- Architecture expanded toward a domain-separated local system core while clearly distinguishing current and planned components.
- Long-running or managed work is planned to use an optional shared Operations/Job runtime service without changing domain ownership; short synchronous domain operations do not require it.
- Pre-1.0 tool contracts are explicitly allowed to change before a stable external contract exists.
- Vendor-neutral open-source core and optional FlashGate module/provider direction documented without selecting a runtime model; MCP protocol extensions are treated separately.
- `SPR-41` review corrections align planning with SEP-1613, SEP-2133, SEP-2577, SEP-2663, and MCP `2025-11-25` implementation-name semantics.
- Roadmap clarified as a high-level pointer to authoritative `BACKLOG.md` planning.
- Unknown tool names in `tools/call`, including read-only-gated write tools, now return generic JSON-RPC Invalid params errors instead of Method not found.
- `MCP_MAX_FILE_SIZE` is now a hard server cap for `read_file`; client `maxBytes` can reduce but not increase it.
- Minimal debug diagnostics are now gated by `MCP_DEBUG` and written only to stderr after redaction.
- Smoke scripts now use per-run JSONL request/response files under `build/` and clean them up before exit.
- JSON-RPC protocol errors now use generic messages such as `parse error`, `invalid request`, `invalid params`, `method not found`, and `internal error`.
- `PathGuard` now accepts an explicit filesystem security policy while keeping the default constructor compatible.
- `list_files` now filters hidden and denied link/reparse entries according to policy.
- Moved protocol definitions from `pkg/protocol` to `internal/protocol`.
- Removed the public `pkg` package layout in favor of internal packages.
- Refactored filesystem operations into focused files.
- Updated supported MCP protocol version from `2025-06-18` to `2025-11-25`.
- Split CLI and server bootstrap responsibilities.
- Extracted tool registry bootstrap.
- Extracted router bootstrap.
- Updated README with build, usage, CLI, release, tool, and smoke-test information.
- Updated roadmap handling so planned work is tracked in `BACKLOG.md`.
- Updated GitHub Actions to the active Node-24-compatible major versions:
  - CI: `actions/checkout@v6`, `actions/setup-go@v6`, `actions/upload-artifact@v7`
  - Release Build: `actions/checkout@v7`, `actions/setup-go@v6`, `actions/upload-artifact@v6`

### Fixed

- Removed the five tracked `MIXED` and seven `NO_FINAL_NEWLINE` line-ending cases through byte-preserving mechanical corrections; encoding, BOM state, text, and whitespace remain unchanged.
- Fixed benchmark runner result construction so soft budget messages remain exclusively in `budget_evaluation` instead of being duplicated into general `warnings`.
- Fixed coverage summaries so a failed minimum-coverage gate produces `status: FAIL` instead of a misleading successful artifact.
- Fixed successful FlashGate tool responses that strict MCP clients such as Codex rejected as `Unexpected response type` because domain objects were returned directly instead of inside `CallToolResult.content[]`.

- Stabilized golangci-lint execution in CI by installing the expected linter version.
- Removed Node.js 20 deprecation annotations from CI and release workflows.
- Improved release artifact visibility in GitHub Actions.
- Validated JSON-RPC smoke-test behavior locally and in CI.

### Security

- `SPR-44` prevents implicit process-working-directory exposure, rejects file roots, keeps startup stdout empty, and redacts startup failures to safe categories.
- Read-only STDIO smokes reject all five write tool names, and negative smokes reject all five removed legacy names with the same generic Invalid params contract.
- `SPR-38` validates JSON-RPC envelopes before dispatch, rejects unsupported batches, suppresses responses for notifications, prevents `tools/call` notification execution, serializes unknown IDs as `id:null`, and converts handler panics to generic Internal error responses.
- `SPR-39` bounds JSON-RPC messages, tool arguments, filesystem operation payloads, recursive delete scope, and serialized responses with generic limit errors.
- `SPR-39` adds centralized redaction before debug diagnostics reach stderr.
- `SPR-36` adds effective path validation through `PathGuard` using evaluated existing paths and evaluated nearest existing parents for create targets.
- `SPR-36` rejects symlink-based filesystem escapes that resolve outside the configured root.
- `SPR-36` maps security/path denials to generic invalid-params tool errors without exposing host paths.
- `SPR-35` enforces `MCP_READ_ONLY=true` at tool registration time by exposing only `list_files`, `read_file`, `stat_path`, and `exists_path`.
- `SPR-35` prevents direct `tools/call` execution of write-capable tools in read-only mode because those tools are not registered.
- Filesystem paths are resolved through a sandbox root.
- Absolute user paths are rejected.
- Parent directory traversal is rejected.
- Destructive operations use conservative defaults.
- Tool implementations do not bypass the filesystem abstraction.
- Normal MCP operation reserves standard output for JSON-RPC protocol messages.
- Diagnostic output is kept separate from the JSON-RPC stream.
