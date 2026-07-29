# Testing

## Governance enforcement

BL-333 supplies the change-trigger, finding-remediation/review-mode, and
handoff-readiness foundation. BL-334 enforces it through
`scripts/Test-GovernanceConsistency.ps1`.

Parse every new or changed PowerShell source with PowerShell 7.6.4 before its
first execution. Then run:

```powershell
& {
    .\scripts\Test-GovernanceConsistency.ps1
    .\scripts\Test-GovernanceConsistencyFixtures.ps1
    .\scripts\Test-DocumentationConsistency.ps1
}
```

`scripts/Test-ClassicReviewArtifact.ps1` is the versioned Hosted-CI execution
mirror of the external canonical Classic artifact validator. Its integration
baseline is 32719 bytes with SHA-256
`c6da4524c881d339bdccfde5894277ddc549fae898348e538956302949518c51`.
The mirror is not an independent governance source. The fixture runner resolves
it relative to its own script by default and still accepts an explicit
`-CanonicalArtifactValidatorPath`. When the external canonical validator exists
locally, the runner compares both files byte-for-byte by SHA-256 and fails
before creating fixtures on any divergence. Hosted Windows CI explicitly passes
the repository mirror and therefore requires no contributor-local path. Fixture
record current-commit and Hosted-CI head provenance are bound to the checked-out
repository HEAD so the unchanged matrix remains valid on each committed CI head.

The fixture matrix exercises the production validator with positive and
negative canonical paths, every immutable mode flag, real tracked domains,
same-run and deferred-finding bindings, byte-exact correction/current-delta
hashes, strict focused-delta and finding matrices, complete per-finding
completion parity, exact narrative/report/repository/external/status sets,
the five canonical external path-to-scope mappings, the strict bounded
`HANDOFF.md` JSON contract, its exactly-16-key typed visible status block,
independent counts for all four status/contract markers, rejection of reserved
control lines outside the visible block, and complete visible/JSON parity,
actual valid and re-manifested corrupt ZIPs, external before/after payloads,
complete tracked-path coverage, and the real CI/release parameter binding.
Negative packages alter bytes, self-reported hashes, findings, evidence, paths,
interfaces, report and handoff blocks, external path/scope mappings, status,
counts, duplicate/unknown visible keys, independently added or reversed
markers, reserved outside-block lines, and queue through the same productive
validator path. Each re-manifested negative case must fail its expected
specific productive gate. A workflow checkpoint
record passes only with an authoritative
immutable repository, commit, event, ref, run, and Hosted CI source identity.

The current BL-333/BL-334 governance matrix contains 198 cases. The count is
reported by the fixture runner and must change together with its permanent
case inventory.

FlashGate MCP uses Go's standard testing framework and the `flashgate-mcp` binary.

The project aims for high test coverage in security-sensitive and filesystem-related code.

## Test Commands

Run all tests:

```bash
go test ./...
```

Run tests with the race detector:

```bash
go test -race ./...
```

The authoritative race gate runs on a platform with a supported race toolchain.
The current Windows host has no CGO/GCC race toolchain, so Windows race is reported
as an infrastructure limitation rather than worked around. Native Linux
`go test -race ./...` is required. Functional serialization, payload, fixture, and
budget-contract tests remain active under race; only the `testing.AllocsPerRun`
budget assertion is skipped because race instrumentation changes allocation
behavior. Ordinary non-race tests continue to enforce the unchanged allocation
budgets.

Run tests for a specific package:

```bash
go test ./internal/fs
```

Run focused MCP protocol tests:

```bash
go test -v ./internal/protocol ./internal/mcp/server ./internal/mcp/router ./internal/mcp/tools ./internal/mcp/initialize
```

Run tests with coverage:

```bash
go test -cover ./...
```

## Required Quality Checks

Before committing, run:

```bash
go fmt ./...
go vet ./...
go test ./...
golangci-lint run
go build -o build/flashgate-mcp ./cmd/server
```

On Windows, the build command is usually:

```powershell
go build -o build/flashgate-mcp.exe ./cmd/server
```

## Controlled native Linux validation

Windows remains the leading FlashGate development environment. WSL2/Linux is
only a native build, test, and validation environment. The canonical runner
copies the Windows source one way into a new ext4 test copy below `/home`; no
source is synchronized back, no native gate runs below `/mnt/c`, and no other
MCP project is activated by this workflow.

### Canonical entry points

Run PowerShell 7.6.4 through the Windows orchestrator:

```text
C:\Users\ThomasW\OneDrive - VOXTRONIC\Desktop\Voxtronic\Scripts\Invoke-FlashGateLinuxValidation.ps1
```

The orchestrator uses:

```text
Runner  /home/weidnerthomas/voxtronic/tools/mcp-linux-validation/mcp-linux-validate
Profile /home/weidnerthomas/voxtronic/tools/mcp-linux-validation/profiles/flashgate-mcp.json
```

`quick`, `security`, and `release` use class-specific companion profiles in the
same managed profile directory. All runner and profile paths are absolute; PATH
fallback is forbidden for the runner, managed Go toolchain, and managed
`golangci-lint`.

Example:

```powershell
& {
    $result = & "C:\Users\ThomasW\OneDrive - VOXTRONIC\Desktop\Voxtronic\Scripts\Invoke-FlashGateLinuxValidation.ps1" `
        -RunId "manual-validation-20260724-130500" `
        -ValidationClass "standard" `
        -ReportContext "Manual validation after filesystem changes"

    $result | Format-List
}
```

Run IDs must match `<context>-<YYYYMMDD-HHMMSS>` and contain only lowercase
ASCII letters, digits, and hyphens. Spaces, separators, `..`, shell
metacharacters, control characters, and existing validation, log, or snapshot
paths are rejected.

### Mandatory triggers

Native Linux validation is mandatory when a change affects:

- `cmd/**`, `internal/**`, any `*.go` or `*.sh` file, `go.mod`, or `go.sum`;
- `.golangci.yml`, `.gitattributes`, `.editorconfig`, or
  `.github/workflows/**`;
- the Linux validation runner, a FlashGate validation profile, or
  build/test/lint/smoke scripts;
- filesystem, path, permission, symlink, junction, or reparse-point behavior;
- build tags, `runtime.GOOS`, `runtime.GOARCH`, platform-specific processes,
  signals, or cleanup;
- Go, linter, or build-tool versions;
- central security or platform boundaries, CI, release behavior, or completion
  of a platform-relevant sprint or PR diff.

It is not automatic for spelling, non-technical planning, or purely editorial
changes without a technical instruction or runtime effect. The gate remains
available for such changes.

### Validation classes

| Class | Scope | Closure use |
|---|---|---|
| `quick` | `go test ./...` and `go vet ./...` using the managed toolchain | Local intermediate check only; never replaces a commit, sprint, PR, or release gate |
| `standard` | Environment and package discovery, module download, formatting, vet, tests, managed lint, and native build | Normal technical sprint and PR validation |
| `security` | `standard` plus existing JSON-RPC, read-only, negative, startup-negative, and native validation-safety smokes | Security-, filesystem-, path-, or platform-boundary changes |
| `release` | `standard` plus existing build-input, repository-build, protocol, negative, and startup smokes | Release preparation; it does not publish a release |

The runner validates copy integrity, permissions, line endings, the managed
linter identity, expected artifacts, unexpected changes, and an unchanged
Windows source before returning a result.

The tracked FlashGate source baseline expects zero `MIXED` and zero
`NO_FINAL_NEWLINE` classifications. Intentional LF or CRLF files retain their
established style; corrections are limited to terminator normalization or one
matching final newline and must preserve encoding, BOM state, text, and
whitespace.

### Coverage-aware rules

Changes to executable Go production logic must add or update tests for relevant
success, failure, boundary, security-rejection, platform, resource, and cleanup
paths in the same implementation or correction run. Run targeted coverage for
the directly affected packages first.

Run the complete project coverage matrix only after a coherent implementation
or finding-correction package, for central/shared production logic, when a
minimum-threshold regression is possible, before the final committed PR review,
or when targeted coverage exposes additional risk. The current Windows and
Linux minimums remain authoritative in `.github/workflows/ci.yml`.

PowerShell, Python, and Bash components without percentage coverage use a
documented branch matrix covering success, input validation, failures, timeout,
protection, cleanup/finally, negative security, and platform/context paths.

### Results and blocking

- `PASS` requires successful copy, permission, line-ending, command, artifact,
  unexpected-change, and Windows-source protection gates.
- `PASS_WITH_WARNINGS` is allowed only for individually documented,
  non-blocking warnings such as one controlled `0x800705b4` retry, an optional
  Codex gate failure, or a non-functional runtime deviation. Sprint closure
  requires an explicit justification.
- `FAIL` blocks sprint, integration, commit, PR, and release closure. Build,
  test, vet, lint, coverage, integrity, source-protection, `/mnt`, result-schema,
  or infrastructure-control failures are blocking. A technical test failure is
  not retried automatically.

The orchestrator reads and validates the native `result.json`, binds status to
the runner exit code, and stops Ubuntu only when it started the distribution.
It never deletes validation copies.

### Codex, retention, reporting, and review

A WSL Codex read-only smoke is an additional gate for changes to `AGENTS.md`,
the validation runner or profiles, sandbox/path/context rules, unclear
Linux-specific errors, or an explicit request. It is not required for every
ordinary run. A token, capacity, or external-service failure is non-blocking
only when Codex was optional and all local technical gates passed.

Before that smoke, resolve the native binary below `/home` and require its
`codex-cli` version to match the active Windows version. Linux uses its own
configuration and authentication storage; a Windows binary or profile below
`/mnt/c` is never a substitute.

Validation copies are classified as `REFERENCE_BASELINE`,
`ACTIVE_VALIDATION`, `FAILED_VALIDATION`, or `DISPOSABLE_VALIDATION`.
References and failed runs are retained; successful normal runs may be marked
as later cleanup candidates. No copy is deleted automatically. Any later
cleanup requires an exact allowlist path, ignore/tracked/untracked and reparse
checks, preserved evidence, and a bounded dry run.

When Linux validation is required, the completion report records:

```text
LinuxValidationRequired
LinuxValidationClass
LinuxValidationRunId
LinuxValidationResult
LinuxValidationPath
LinuxValidationDuration
LinuxWarningCount
LinuxFailureCount
WindowsSourceChanged
UnexpectedChanges
CoverageRequired
TargetedCoverageResult
FullCoverageMatrixRequired
FullCoverageMatrixResult
ReviewType
ReportOrResultPath
SprintClosureAllowed
```

Use exactly one full independent review for a larger implementation or
integration state. Bundle Blocker and Major corrections, then review only the
changed delta, regression tests, coverage impact, and directly affected
interfaces. Do not repeat unchanged INF-095 or project-wide matrices without a
concrete technical reason. The next full review is performed on the final
committed PR state after successful hosted CI.

## Test Strategy

### Unit Tests

Unit tests are required for:

- configuration loading
- path validation
- filesystem operations
- MCP protocol routing
- tool execution

### Filesystem Tests

Filesystem tests use:

```go
t.TempDir()
```

This ensures that tests do not modify real user data.

Test helper functions may use `os.WriteFile`, `os.ReadFile`, `os.MkdirAll` and `os.Stat` to create and verify test fixtures.

This is allowed in tests.

Production code outside `internal/fs` must not use direct filesystem operations.

### Security Tests

Security tests must cover:

- path traversal
- absolute path rejection
- sandbox escape attempts
- destructive operation defaults
- overwrite behavior
- recursive delete behavior
- JSON-RPC message and tool argument limits
- filesystem read, write, list, copy, and recursive delete limits
- diagnostics redaction

### Integration Tests

JSON-RPC smoke tests exercise the built server binary over STDIO.

On Windows:

```powershell
.\scripts\smoke-jsonrpc.ps1
$env:MCP_READ_ONLY = "true"
.\scripts\smoke-jsonrpc.ps1
Remove-Item Env:\MCP_READ_ONLY
.\scripts\smoke-jsonrpc-negative.ps1
```

On Linux:

```bash
bash scripts/smoke-jsonrpc.sh
MCP_READ_ONLY=true bash scripts/smoke-jsonrpc.sh
bash scripts/smoke-jsonrpc-negative.sh
```

Run fail-closed startup validation on Windows:

```powershell
.\scripts\smoke-startup-negative.ps1
```

On Linux:

```bash
bash scripts/smoke-startup-negative.sh
```

The default smoke test validates `initialize`, the exact eight-tool `tools/list`, `list_directory`, `read_file`, `get_path_info` for existing and missing paths, and `move_path` rename behavior. Every positive result must have only the `CallToolResult` envelope fields, exactly one text block with only `type` and `text`, valid compact object JSON, and a deeply equal `structuredContent` object. The read-only variant verifies the exact three-tool profile and invokes all five write-capable names, requiring the same generic Invalid params response without filesystem changes. The negative smoke validates all five removed legacy names in addition to malformed JSON, unknown methods, invalid `tools/call` params, and notification no-response behavior.

The startup-negative smoke covers missing/empty/whitespace/relative roots, `.` with and without the development opt-in, invalid development/read-only values, missing and file roots, a valid absolute root, exit codes, empty stdout, safe stderr categories and cleanup.

GitHub Actions runs default, read-only, negative JSON-RPC, and startup-negative smoke variants on both `windows-latest` and `ubuntu-latest`. The smoke scripts create per-run artifacts under `build/` and clean them before exit. Script output is CI diagnostic output; server stdout remains reserved for redirected JSON-RPC protocol messages.

Limit and redaction behavior is primarily covered by Go unit tests. Additional limit-negative smoke coverage can be added later if it can be done without broad smoke-script refactoring.

Focused contract tests compare runtime tool definitions with `docs/mcp-tool-catalog.json` for name, title, description, complete input schema, and deeply equal runtime `outputSchema`/catalog `resultSchema`. Targeted tests require exactly eight runtime output schemas, object roots, valid required/property relationships, expected project property types, representative successful `structuredContent`, both `get_path_info` variants, and the `read_file` outer-array/inner-string distinction. The tests-only structural checker covers only `type`, `properties`, `required`, `additionalProperties`, `items`, `oneOf`, and `const` as currently emitted; it is not a complete JSON Schema 2020-12 validator.

The `tools/list` JSON-RPC wire test checks schema exposure for both profiles and records deterministic UTF-8 JSONL sizes with and without output schemas. `SPR-46` records 1239/2134 bytes for read-only and 3850/5657 bytes for default; no regression budget is enforced.

### MCP Compatibility Testing

The implemented protocol remains MCP `2025-11-25`. Explicit `CallToolResult` DTO tests, a strict project-local decoder, legacy unwrapped negative fixtures, all-eight-tool adapter coverage, and full JSON-RPC wire tests cover success and the unchanged error contract. The decoder intentionally validates the exact FlashGate-emitted subset (one text block, required object `structuredContent`, optional boolean `isError`, no `_meta`) rather than claiming to decode every standard-conformant MCP result. Windows and Bash positive smokes enforce the same shape.

Future protocol or extension support still requires version-negotiation, extension-negotiation, client fallback, and compatibility tests before it is advertised. Complete JSON Schema 2020-12 validation and official MCP conformance tooling remain planned.

### Benchmarks

`SPR-47` benchmarks performance-sensitive operations including:

- directory listing
- file reading
- file copying
- tool-result wrapping and serialized payload forms
- search

Benchmark command:

```bash
go test -bench=. ./...
```

The deterministic benchmark tests also enforce the exact tool-profile and workflow measurement sets, the six payload/allocation budgets from `benchmarks/budgets.json`, initialized-notification framing, initialization-result validation, zero `scanned_bytes` for ordinary reads, partial Linux procfs metrics, host-path redaction, and clean versioned artifacts. Windows and Linux artifacts are each strictly decoded and independently reevaluated only after the canonical hard/soft workflow sets and all positive soft limits have been validated fail-closed. Their complete resource, sample, exit-status, stderr, general warning, unsupported-metric, provenance, and deterministic cross-platform parity is then compared. The validator rejects unknown or duplicate fields, missing required fields, wrong types, nulls, trailing values, stale or fabricated embedded budget results, incomplete/unknown/zero soft budgets, hard-budget manipulation of either or both artifacts, and schema-invariant violations. Runner result construction stores soft budget messages only in `budget_evaluation`; production-path tests cover dual-platform, Windows-only, Linux-only, and no-soft-warning results through the complete platform gate. General result `warnings` remain reserved for non-budget runtime warnings and fatal in clean versioned artifacts, while matching soft budget warnings remain review-only and recomputed hard failures remain fatal. Platform baseline generation is a separate two-phase operation after the implementation commit. The diagnostic wrappers recognize `-RecordBaseline` and `--record-baseline` only to reject them fail-closed before any Go invocation or write. All explicit and Linux-default output names are physically checked; protected-directory aliases and existing final symlink/reparse targets are rejected. The runner additionally binds the validated parent with `os.Root`, writes through one exclusively opened temporary-file handle, and publishes by handle-relative rename without following the final target. Tests cover protected parent aliases, noncanonical final links to both platform baselines, broken links, hard-link aliases, late target/output-parent/protected-directory exchanges, regular diagnostic output, and invariant baseline hashes.

Functional gates such as tests, builds, vet, lint, protocol smokes, and parser checks are independent of the host measurement window. Their timing and resource consumption are not performance evidence. Performance gates are valid only when the entire measurement series runs outside the primary development host's scheduled-load block from 19:00 inclusive until 04:00 exclusive in `Europe/Vienna`; the preferred safety-margin window is 04:15–18:45, and the series must finish before 19:00. The blocked interval is a formal baseline blocker.

Every performance measurement report records the `Europe/Vienna` time window, start and end times, and whether known or unusual additional host load was present. A baseline is rejected if such load is known or observed even inside the nominally allowed interval. Contaminated runs are retained as diagnosis evidence without being approved, compared for regression, or used to tune budgets. Ordinary wrapper runs may continue but are marked contaminated; they cannot record a baseline.

## Current Tested Packages

Currently tested:

- `internal/config`
- `internal/diagnostics`
- `internal/security`
- `internal/fs`
- `internal/protocol`
- `internal/mcp/server`
- `internal/mcp/router`
- `internal/mcp/transport`
- `internal/mcp/initialize`
- `internal/mcp/tools`

## Version 1.0 Planned Validation Matrix

The current tests above describe the implemented filesystem baseline. Version 1.0 adds the following required gates.

### Payload and catalog tests

- payload-class selection for metadata, structured pages, heavy text, media/binary, and large results;
- heavy payload appears only once across MCP result fields;
- wire-amplification and useful-byte budgets;
- bounded base64 thresholds;
- opaque resource handles contain no host path and enforce owner/TTL/service-generation checks;
- fallback behavior for clients without resource-link support;
- deterministic tool ordering and catalog fingerprint;
- profile-specific `tools/list`, schema, description, and initialization-instruction budgets;
- safe read-only catalog when roots exist and no explicit profile is selected.

### Operations and multi-principal tests

- opaque handles bound to principal, profile, root, execution backend, and service generation;
- cross-principal status/result/cancel/cache/resource denial;
- global, per-domain, and per-principal concurrency limits;
- global/per-principal queue caps and fair scheduling;
- deterministic overload behavior;
- TTL cleanup, restart invalidation, shutdown, and leak detection;
- slow-reader and audit/log backpressure behavior.

### Typed command tests

- executable ID resolves only to approved absolute binary;
- fixed subcommand and allowed flags/value rules;
- path arguments remain under allowed roots;
- no shell interpretation;
- response files, hooks, plugins, loaders, config overrides, and unapproved environment are rejected;
- stdout/stderr, runtime, process count, and network policy limits;
- Windows/Linux isolation outcomes and redaction.

### System service and execution-identity tests

Version 1.0 tests Variant A only:

- Windows SCM and Linux systemd lifecycle;
- Named Pipe ACL and Unix socket ownership/mode;
- OS-derived peer identity cannot be overridden by payload;
- caller authorization independent of service-account filesystem permission;
- allowed FlashGate policy plus denied service-account ACL fails safely;
- denied caller plus available service-account ACL fails before execution;
- service-account root backend and dedicated identity;
- no LocalSystem/root convenience default;
- unsupported `user-worker` configuration fails closed;
- no in-process impersonation path;
- caller and effective backend identity both appear in bounded audit events;
- service restart invalidates generation-bound handles/resources;
- `auto` never falls back after managed denial or incompatibility;
- proxy/client stdout remains MCP-only.

Variant B worker tests are post-Version 1.0 and require a separate implementation gate.

### Protocol compatibility tests

Before Version 1.0, publish and test the supported MCP revision matrix:

- current `2025-11-25` behavior;
- any later final revision only after implementation;
- stateless-core behavior where selected;
- deterministic list cache/TTL invalidation;
- final Tasks Extension mapping without mixing the 2025 experimental lifecycle;
- extension downgrade/mismatch;
- JSON Schema 2020-12 validation;
- deprecated Roots never overrides server roots.

### Audit and failure-path tests

- immutable event/correlation IDs;
- proxy/service/backend/job/process correlation;
- redaction before output;
- rotation and retention;
- slow sink and bounded buffering;
- disk-full behavior;
- log-injection handling;
- shutdown flush/drop policy;
- no secret, full payload, unrestricted environment, or unnecessary host-path leakage.

### Release and supply-chain tests

- artifact version/help/platform/name checks;
- compact and verbose CLI identity checks;
- Windows x64/ARM64 `VERSIONINFO`, PE architecture, icon, and Explorer property checks;
- Linux x64 native and ARM64 cross-build Go/VCS, ELF-header, ELF-note, and Go build-ID checks;
- exact ZIP/TAR.GZ inventory and SHA-256 verification;
- repeated byte-for-byte binary and archive reproducibility checks;
- host-path, username, hostname, and credential-shaped-value leak checks;
- shared Go/PowerShell/Bash SemVer and `SOURCE_DATE_EPOCH` fixtures;
- fail-closed Git snapshot inventory, ignored/sensitive-file, TAR traversal,
  link, special-type, duplicate, missing, length, and hash fixtures;
- descriptor-bound native work-root traversal, symlink, component-exchange,
  length, and whitespace fixtures;
- static canonical build-manifest validation on all four targets;
- normalized embedded-icon frame identity and manipulation fixtures;
- persistent Windows/Linux verifier-process contracts for native-host
  selection, static-failure-before-launch, runtime/help status, timeout,
  bounded stdout/stderr, deterministic Windows termination/cleanup deadlines,
  complete verifier error aggregation, and READY-confirmed
  child-tree/process-group cleanup;
- regular-clone, linked-worktree, nonrepository, and damaged-Git fixtures;
- no interpreter runtime dependency;
- service asset syntax and install/remove dry validation;
- checksums;
- SBOM and dependency inventory;
- build provenance;
- signing verification where configured;
- reproducible-build comparison;
- pinned/validated workflow policy;
- atomic rollback documentation and smoke procedure.

The controlled commands and expected fields are documented in [Build and
release metadata](build-and-release-metadata.md), [Artifact
verification](artifact-verification.md), and [Manual metadata
validation](manual-metadata-validation.md). Native Linux validation verifies a
manifest-bound Git inventory in a new controlled extraction directory before
copying it into an ext4 clone under `/home`; Windows-mounted paths are
orchestration inputs only and are never the native build directory.

### Cross-project benchmark

The Version 1.0 benchmark compares pinned FlashGate, official Node.js filesystem, selected native Rust filesystem, and selected Go filesystem MCP versions on the same host and corpus. The report must separate feature/security differences from measured performance and must not claim results for unmeasured operations.

See [Efficiency Improvement Plan](efficiency-improvement-plan.md), [Execution Identity Backends](execution-identity-backends.md), and [Version 1.0 Scope](version-1-scope-and-release-boundary.md).

<!-- FLASHGATE_PERFORMANCE_WORKSPACE_POLICY_START -->
## Authoritative benchmark workspace gate

On the primary Windows development host, an authoritative benchmark attempt is
blocked unless its Windows working area is below:

`C:\Voxtronic\Codex\Temp\Benchmarks`

Before the attempt, verify that the root is a fixed local NTFS path, contains no
reparse point, and is not below OneDrive, Dropbox, a redirected user directory,
a network share, or other synchronized storage.

All source bundles, isolated Windows checkouts, prepared binaries, measurement
outputs, logs, verification files, and controller data stay in that local area
through the final host-load gate. Archival copying to synchronized storage occurs
only afterward and is not part of the measured phase.

The native Linux checkout and temporary output remain on the distribution's
native ext4 filesystem under `/home`. A path below `/mnt` or `/media` is a formal
baseline blocker.

All validation, test, vet, lint, build, linker, and parser work finishes before
the authoritative host gate. After the last such operation, wait at least 180
seconds without Git, Go, scan, archive, or analysis activity. Run the authoritative
three-block CPU/disk/RAM/per-process-delta preflight exactly once. If it passes,
invoke the prepared binaries directly without rebuilding. A 15-second intermediate
gate precedes native Linux measurement and a final host gate precedes any result
copy, hash scan, JSON verification, report, archive, or OneDrive access.

`scripts/benchmark.ps1 -RecordBaseline` and
`scripts/benchmark.sh --record-baseline` are deliberately blocked compatibility
flags, not an authoritative workflow. A separately prepared controller implements
the two-phase attempt; no wrapper-side shortcut or time override is permitted.
<!-- FLASHGATE_PERFORMANCE_WORKSPACE_POLICY_END -->
