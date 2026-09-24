# Testing

## Portable project validation

FlashGate's public checkout contains the product validation needed by ordinary
contributors and CI. Start with focused checks for the changed component, then
run the applicable complete quality chain below. Build, test, security, release,
shell, and documentation checks must not depend on private workflow files or a
machine-specific workspace.

Use PowerShell 7.6.x for repository PowerShell scripts. Commands that create
temporary evidence require an explicit caller-provided work root through
`FLASHGATE_WORK_ROOT`; the root must be outside the source tree and satisfy the
path constraints documented by the invoked script.

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

### Shell script validation

BL-251 validates the complete repository shell-entry-point inventory rather
than a fixed file list. The Windows gate requires PowerShell 7.6.x (Major 7, Minor 6) and
`C:\Program Files\Git\bin\bash.exe`, parses every `.ps1` and `.psm1`, runs
Git Bash syntax checks for every `.sh`, validates strict UTF-8, line endings,
final newlines and supported Bash shebangs, and proves that validation did not
change repository status or script bytes:

```powershell
& {
    $workRoot = $env:FLASHGATE_WORK_ROOT
    if ([string]::IsNullOrWhiteSpace($workRoot)) { throw 'Bind FLASHGATE_WORK_ROOT first.' }
    .\scripts\Test-ShellScripts.ps1
    .\scripts\Test-ShellScripts.Tests.ps1 -WorkingPath $workRoot
}
```

The native Ubuntu gate uses only `/usr/bin/bash` and native standard tools.
Bind an existing caller-controlled directory on the native filesystem before
running the harness; the harness creates and removes only its unique scratch
subdirectory below that root:

```bash
work_root="$HOME/.cache/flashgate-mcp-shell-validation"
mkdir -p -- "$work_root"
export FLASHGATE_WORK_ROOT="$work_root"
/usr/bin/bash scripts/test-shell-scripts.sh
/usr/bin/bash scripts/test-shell-scripts.tests.sh
```

The persistent negative matrices cover an empty inventory, missing files,
invalid PowerShell and Bash syntax, wrong runtime paths or versions, invalid
repository roots, paths with spaces, stable inventory ordering, mutation
detection, bounded subprocess exit and timeout behavior, unconfirmed
termination without an unbounded stream wait, deterministic failure
classification, cleanup failure reporting, and a symlink shell entry point.
The bounded-process result records the PID immediately after a successful
process start. Its timeout regression validates the positive direct PID,
exit code 124, confirmed tree termination, absence of that concrete process,
and the case where timeout occurs before an optional child-authored PID file.
The PowerShell matrix currently passes 25/25 cases. The Bash cleanup-negative
probe uses only its task-local fixture root and requires `Status: FAIL`,
`Cleanup: FAIL`, a nonzero failure count, a nonzero exit code, and exactly one
terminal status block. CI
runs both Windows commands after binding a verified PowerShell 7.6.x
runtime, and both native Bash commands on Ubuntu. PSScriptAnalyzer and
ShellCheck remain optional local enrichments: when they are unavailable and
installation is not authorized, the parser, native syntax checks, structural
policy checks, and positive/negative harnesses are the approved deterministic
alternative. Script changes still require the controlled native Linux gate
below before completion.

## Required Quality Checks

Before committing, run:

```bash
go fmt ./...
go vet ./...
go test ./...
golangci-lint run
go build -o build/flashgate-mcp ./cmd/server
```

Run the applicable Windows or native Linux shell-validation pair above when
PowerShell, Bash, CI, build, release, smoke, or validation scripts change.

On Windows, the build command is usually:

```powershell
go build -o build/flashgate-mcp.exe ./cmd/server
```

## Native Linux validation

Run Linux validation from a native Linux checkout on a native filesystem. A
checkout below a Windows mount such as `/mnt/c` does not provide authoritative
Linux filesystem, permission, symlink, signal, or cleanup evidence.

The portable native sequence is:

```bash
work_root="$HOME/.cache/flashgate-mcp-shell-validation"
mkdir -p -- "$work_root"
export FLASHGATE_WORK_ROOT="$work_root"
/usr/bin/bash scripts/test-shell-scripts.sh
/usr/bin/bash scripts/test-shell-scripts.tests.sh
test -z "$(gofmt -l cmd internal)"
go vet ./...
go test ./...
go test -race ./...
golangci-lint run
go build -o build/flashgate-mcp ./cmd/server
```

Run this sequence when a change affects Go or shell sources, CI, build or
release behavior, filesystem/security boundaries, platform-specific code,
tooling versions, or native process lifecycle. Purely editorial changes do not
require native execution unless they alter an executable instruction.

Record the actual tool versions and results. A failed technical check is
blocking; do not substitute Windows-mounted or synthetic evidence for a native
Linux result.

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

Focused contract tests compare runtime tool definitions with `docs/mcp-tool-catalog.json` for name, title, description, complete input schema, deeply equal runtime `outputSchema`/catalog `resultSchema`, and the exact eight-tool annotation matrix. The catalog gate parses annotations structurally, requires all four members even when their value is `false`, and includes a negative mutation proving that deletion of one explicit-false member fails. Targeted tests require exactly eight runtime output schemas, object roots, valid required/property relationships, expected project property types, representative successful `structuredContent`, both `get_path_info` variants, the `read_file` outer-array/inner-string distinction, fail-closed unknown annotation lookup, and explicit false serialization. The tests-only structural checker covers only `type`, `properties`, `required`, `additionalProperties`, `items`, `oneOf`, and `const` as currently emitted; it is not a complete JSON Schema 2020-12 validator.

The `tools/list` JSON-RPC wire test checks schema and annotation exposure for both profiles, requires all four annotation members in the real wire object, verifies the exact annotation values, and records deterministic UTF-8 JSONL response/result sizes. The current BL-202 measurements are 2446/2411 bytes for read-only and 6492/6457 bytes for default (response/result). The test and `benchmarks/budgets.json` pin them as deterministic contract values; no timing, CPU, RAM, or performance baseline is recalibrated.

### MCP Compatibility Testing

The implemented protocol remains MCP `2025-11-25`. Explicit `CallToolResult` DTO tests, a strict project-local decoder, pre-`CallToolResult` unwrapped negative fixtures, all-eight-tool adapter coverage, and full JSON-RPC wire tests cover success and the unchanged error contract. The decoder intentionally validates the exact FlashGate-emitted `2025-11-25` subset (one text block, required object `structuredContent`, optional boolean `isError`, no `_meta`) rather than claiming to decode every standard-conformant MCP result. Windows and Bash positive smokes enforce the same shape.

Version 1.0 adds a separate `2026-07-28` compatibility matrix rather than mutating the `2025-11-25` expectations in place. Required coverage includes `server/discover`; per-request protocol/capability `_meta`; optional self-reported `clientInfo` without authorization effect; response `serverInfo`; `UnsupportedProtocolVersion`; required result `resultType`; list `ttlMs`/`cacheScope`; extension downgrade/mismatch; no connection-history authority; and cross-revision opening/probe behavior on STDIO. Future protocol revisions require their own explicit matrix/path delta. Complete JSON Schema 2020-12 validation and official MCP conformance tooling remain planned.

### Benchmarks

`SPR-047` benchmarks performance-sensitive operations including:

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

### Filesystem identity, compare, and verification tests

- BL-048 batch hashes/content identities are deterministic for the defined input and never accepted as authorization evidence;
- `compare_paths` file/file and directory/directory metadata/hash/content/text modes, type mismatch, bounded differences, paging, cheapest-safe-proof behavior, and cloud/local-only indeterminate outcomes;
- `verify_paths` one/many expected-state records, compact count-only success, bounded mismatch/indeterminate deltas, and manifest-like batch use without workflow-specific semantics;
- fresh/strong verification re-evaluates the required current metadata/content proof and does not pass solely from reusable cached evidence;
- current root/profile/capability/principal/path checks run regardless of supplied fingerprint/hash/snapshot identity;
- no second hashing/tree implementation diverges from BL-048/049 primitives.

### Payload and catalog tests

- payload-class selection for metadata, structured pages, heavy text, media/binary, and large results;
- heavy payload appears only once across MCP result fields;
- deterministic useful-byte, wire-amplification, approximate-token-per-useful-byte,
  and serialization-copy budgets for the existing result fixtures; metadata-only
  fixtures use absolute byte budgets and do not manufacture ratios from zero useful bytes;
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

### BL-241 additive multi-client and host-process lifecycle matrix

BL-241 retains its independent pre-existing multi-client, lifecycle,
compatibility, and denial contract. The following cases remain separately
required and discoverable:

- general concurrent-client behavior;
- normal and abrupt disconnects;
- service and host restart;
- stale transport endpoints;
- shutdown;
- general IPC, proxy, and service version mismatch;
- unauthorized access; and
- fail-closed `auto` behavior.

The ADR-017 and BL-341 integrated Windows/Linux host-lifecycle matrix below is
additive. Retained-pipe, lease, PID-reuse, and parallel-start cases do not
replace any pre-existing case above. The additional required cases are:

- normal STDIN EOF and normal transport closure;
- unrecoverable read/write pipe failure and broken pipe;
- client crash or kill;
- verified owner-process or control-channel death;
- retained or duplicated pipe handles that delay EOF;
- long legitimate idle with no false-positive termination;
- operating-system stop/termination signal;
- optional negotiated lease expiry, including relevant reconnect and version
  mismatch behavior;
- proxy death cleans connection-owned service state;
- normal client disconnect leaves the persistent service active;
- bounded Operations/Job cleanup through its owner;
- bounded Managed Child cleanup through its owner;
- PID reuse and process-start-identity mismatch;
- stale instance/runtime-registry state and deterministic evidence-only cleanup;
- repeated parallel starts and closes;
- return to the expected process baseline; and
- after conclusive owner/transport loss plus the bounded shutdown window, zero
  remaining `DEFINITELY_ORPHANED` session-scoped hosts or owned children.

The direct-mode oracle is one full FlashGate process per active MCP client
transport session. Multiple direct processes are valid. The shared-service
oracle is one persistent full FlashGate service plus one lightweight edge proxy
per active client transport; further processes must be explicitly classified
as workers or managed children. Ambiguous live-owner/live-transport cases are
`SUSPECTED_STALE` and the test must prove no heuristic age, idle, CPU,
request-count, singleton, PID-only, or registry-only termination.
### Protocol compatibility tests

Before Version 1.0, publish and test the supported MCP revision matrix:

- exact current `2025-11-25` initialization behavior, preserved while that revision is supported;
- final `2026-07-28` stateless behavior only after implementation, including `server/discover`, per-request metadata, supported-version errors, result typing, server identity metadata, required cache hints on cacheable list/read results including `resources/read`, and `subscriptions/listen` for opted-in list/resource change notifications;
- cross-revision STDIO opening/probe/fallback behavior without ambiguous connection-state authority;
- deterministic exact-revision catalog fingerprint and cache invalidation, including safe `cacheScope`, resource-read invalidation, cross-principal/private-cache isolation, and confirmation that `2025-11-25` `resources/subscribe` semantics are not used on the `2026-07-28` stateless path;
- final Tasks Extension mapping without mixing the 2025 experimental lifecycle;
- extension downgrade/mismatch and missing-required-capability cases;
- JSON Schema 2020-12 validation;
- deprecated Roots never overrides server roots;
- any future revision only through a new explicit matrix/path entry and directly affected tests.

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

an explicit caller-provided task-bound workspace on local nonsynchronized storage

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

## PowerShell 7.6 LTS patch contract

Compatibility requires PowerShell major version 7 and minor version 6. `ObservedPowerShellVersion` records the actual patch. `MinimumPowerShellVersion` is `null` unless a specific fix establishes and justifies a minimum patch. `ServicingTarget=LatestServicedPatchWithin7.6` is the maintenance target. Exact patch, path, or hash bindings are allowed only for historical evidence, bug reproduction, installer/download/SBOM/supply-chain provenance, or a documented minimum-patch fix; each exception must be explicitly classified.
