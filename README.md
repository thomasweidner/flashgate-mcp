# FlashGate MCP

**Fast, secure and local-first host operations for MCP.**

FlashGate MCP is a resource-efficient, cross-platform MCP server for controlled local host operations. Today it provides secure, root-confined filesystem access; Version 1.0 extends that foundation to bounded search, managed processes, typed command execution, and system information. Deterministic work stays local to reduce latency, payload size, model round trips, and token use.

FlashGate MCP uses repository `thomasweidner/flashgate-mcp`, Go module
`github.com/thomasweidner/flashgate-mcp`, binary `flashgate-mcp`, and MCP server
implementation name (`serverInfo.name`) `flashgate`.

It exposes secure filesystem operations to MCP-compatible clients through JSON-RPC over STDIO. The server is designed for predictable behavior, low operational overhead, clear security boundaries, and maintainable, auditable code.

## Status

The project currently implements the core MCP server loop, JSON-RPC routing and request validation, tool discovery, tool execution, MCP-conformant `CallToolResult` wrapping, filesystem abstraction, root-confined path handling, read-only tool gating, canonical build identity, native Windows/Linux metadata, deterministic release archives, reproducible resource/latency/payload benchmarks, tests, and documentation.

The current implemented scope is filesystem operations. Version 1.0 plans bounded search, process observation/management, typed allowlisted command execution, controlled system information, named roots, safe-default capability profiles, the Operations/Job Manager, payload-efficient large-result handling, reusable content fingerprints with bounded file/tree compare and expected-state verification, and optional local system-service deployment. These remain planned work.

Implemented tools:

```text
list_directory
read_file
get_path_info
write_file
create_directory
delete_path
copy_path
move_path
```

## Design Goals

- Secure-by-default filesystem access
- Root-confined path handling
- Deterministic MCP tool discovery
- Cross-platform support for Windows and Linux
- Single self-contained Go binary
- Minimal runtime dependencies
- Clear package boundaries
- Comprehensive unit tests
- Professional documentation for users and developers
- No external MCP framework dependency
- Measurable CPU, RAM, startup, latency, response-size, and token efficiency
- Local deterministic operations instead of transferring file content through the model
- Payload-heavy content transferred once with bounded result/resource handling
- Reusable content identities and compact verification before unnecessary retransmission
- Safe read-only profile as the Version 1.0 default when no higher-risk profile is selected
- Separate caller authorization and effective service execution identity
- No interpreter runtime for normal Windows/Linux operation

## FlashGate Security Gate

“Gate” means a server-enforced boundary: policies, capabilities, roots, limits, path and process validation, redaction, and audit events control access below the MCP adapter. Tool annotations and tool visibility are not authorization.

The current implementation enforces one configured root, optional read-only registration, path policies, hard limits, and redacted diagnostics. The Version 1.0 target adds multiple named roots and capability-based profiles while retaining server-side checks as authoritative. With valid roots but no explicit profile, the target default is safe read-only; write, process, and command capabilities require explicit activation.

The [capabilities, profiles, and named roots guide](docs/capabilities-profiles-and-named-roots.md)
separates current configuration from the accepted Version 1.0 target and
documents how tool exposure, call-time authorization, root policy, and
execution identity compose without turning client metadata into authority.

## Target Domains and Runtime

Accepted future domains are filesystem, search, process, execution, and system information. An optional shared Operations/Job Manager is planned for bounded long-running or managed work, cancellation, deadlines, progress, TTL, cleanup, and leak protection. Short synchronous work may remain directly in domain services; the manager is not currently implemented and does not own domain logic.

FlashGate MCP remains one repository and one primary native binary per platform unless benchmarks and threat models demonstrate a concrete benefit from splitting it. Version 1.0 keeps direct STDIO for non-admin users and adds optional Windows/Linux system-service roles through the same binary. The system service implements service-account roots; per-user workers are designed but deferred.

Version 1.0 runtime roles:

```text
flashgate-mcp [no mode / --mode stdio]  direct MCP over STDIO
flashgate-mcp --mode proxy              STDIO proxy to local service
flashgate-mcp --mode auto               safe service discovery with fail-closed fallback
flashgate-mcp --mode service            Windows SCM or Linux systemd host
```

The system service separates the authenticated caller from the effective OS execution backend. Version 1.0 uses a dedicated service-account backend for administratively granted roots. A later per-user worker backend is planned; in-process impersonation is excluded.

Host-process lifecycle is also planned, not implemented. Direct and proxy/auto
edge processes are session-scoped, while SCM/systemd services persist across
client disconnects. ADR-017 defines the owner matrix, one bounded process-root
shutdown coordinator, secret-safe instance/exit diagnostics, and the strict
`SUSPECTED_STALE` versus `DEFINITELY_ORPHANED` boundary. BL-341 owns the later
runtime implementation and BL-241 its integrated Windows/Linux validation.
## Open-Source, Modules, and Protocol Extensions

FlashGate MCP is a general, vendor-neutral open-source project. Its core, public contracts, documentation, and optional agent guidance have no organization-specific infrastructure, private control-plane, machine-local workflow, proprietary credential or permission, or particular agent-product prerequisite. Consumer policies may compose FlashGate capabilities externally without becoming runtime or authorization dependencies.

Optional FlashGate modules/providers from public, community, vendor, or organization-internal sources are post-Version-1.0 work. No module/provider contract or runtime model is part of the initial stable release, and future providers may not bypass central security or execution-identity controls.

MCP protocol extensions are separate negotiated wire-protocol features. The implemented protocol remains MCP `2025-11-25`. Version 1.0 targets explicit support for the final `2026-07-28` revision alongside the `2025-11-25` initialization path, but no revision is advertised until its adapter path and compatibility tests are implemented. FlashGate names protocol paths by exact revision so future revisions remain explicit. The project keeps its own Go MCP adapter; official MCP SDKs/specification artifacts are interoperability references, not a new runtime dependency. Deprecated MCP Roots is not the basis of FlashGate named roots.

## Protocol and Transport

`flashgate-mcp` communicates through MCP using JSON-RPC over STDIO.

Standard output is reserved for protocol messages. Diagnostic output must not be written to standard output because it would corrupt the JSON-RPC stream.

The server currently supports MCP methods such as:

```text
initialize
tools/list
tools/call
```

The planned `2026-07-28` path is separate: it removes the initialization handshake, requires per-request protocol/capability metadata, implements `server/discover`, and returns revision-specific result/cache metadata. Those behaviors are target architecture only until BL-207/208 are implemented and validated.

Filesystem operations are exposed as MCP tools and invoked through `tools/call`. Every currently implemented successful filesystem call is wrapped centrally as MCP `CallToolResult`: `content` contains one text block with compact JSON and `structuredContent` contains the same domain object. This is the present eight-tool contract. Version 1.0 will retain compact parity only for small metadata where justified; payload-heavy file, binary, search, and process content will be transmitted once with separate metadata or an opaque result/resource handle.

Runtime `outputSchema` is exposed for all eight tools and remains deeply equal to the catalog `resultSchema` values. These schemas describe successful `structuredContent`; the current safe JSON-RPC tool-error contract remains unchanged pending BL-203.

JSON-RPC request envelopes are validated before dispatch. Unsupported batch requests, invalid protocol versions, missing or invalid methods, invalid IDs, and malformed method params are rejected with generic JSON-RPC errors. Parse errors and invalid requests without a valid request ID serialize `id:null`. Notifications do not receive responses; `notifications/initialized` is accepted as a no-op, and other notifications are not executed.

JSON-RPC messages, tool arguments, filesystem payloads, and response sizes are bounded by configurable hard limits. A message that exceeds `MCP_MAX_JSONRPC_MESSAGE_BYTES` is rejected as an invalid request with `id:null` before tool dispatch. Oversized `tools/call` arguments return generic Invalid params errors, and filesystem limit violations return generic limit errors without host paths.

## Security Model

All filesystem operations are constrained to a configured root directory.

The root directory is configured through the `MCP_ROOT` environment variable:

```text
MCP_ROOT
```

`MCP_ROOT` is required. Production roots must be absolute, exist, satisfy the current root policy, and resolve to a directory. Missing, empty, whitespace-only, relative, non-existent, file, or policy-denied roots fail before tool registration and JSON-RPC processing. Expected configuration failures exit with code `3`, leave stdout empty, and report only a safe category on stderr.

The process working directory is available for development only when both `MCP_ROOT=.` and `MCP_ALLOW_CWD_ROOT=true` are set exactly. This opt-in never supplies a missing root and never enables other relative roots. It emits one safe warning on stderr.

Tool arguments use relative paths below this root. Absolute paths, path traversal outside the configured root, and unsafe path forms are rejected by the filesystem and security layers.

Path validation uses two stages:

1. Lexical validation rejects absolute user paths and leading parent traversal such as `..` or `../secret.txt`.
2. Effective path validation evaluates existing paths, and the nearest existing parent for create targets, to confirm the resulting filesystem location remains inside the configured root.

Individual tools do not bypass the filesystem abstraction and do not call host filesystem APIs directly. This keeps path validation centralized and testable.

`SPR-037` adds deny-by-default policy enforcement for hidden paths, UNC paths, symlinks, and Windows reparse points:

```text
MCP_ALLOW_HIDDEN_FILES=false
MCP_ALLOW_UNC_PATHS=false
MCP_FOLLOW_SYMLINKS=false
```

When hidden files are not allowed, dot-prefixed path components such as `.git/config` and Windows hidden-attribute paths are denied. `list_directory` filters hidden entries instead of failing the whole parent listing.

When UNC paths are not allowed, UNC roots and UNC-style user paths are rejected. When symlink following is not enabled, existing symlink components are denied and `list_directory` filters symlink or reparse entries. When symlink following is enabled, symlink targets are still constrained by effective root containment, and Windows junctions or non-symlink reparse points remain denied.

Security and path denials are mapped to generic invalid-path tool errors without exposing host absolute paths.

### Limits and diagnostics

`SPR-039` adds conservative hard limits:

| Environment variable | Default | Purpose |
|---|---:|---|
| `MCP_MAX_FILE_SIZE` | `10485760` | Maximum `read_file` bytes. Client `maxBytes` can only lower this cap. |
| `MCP_MAX_JSONRPC_MESSAGE_BYTES` | `16777216` | Maximum single JSON-RPC stdin message. |
| `MCP_MAX_TOOL_ARGUMENT_BYTES` | `12582912` | Maximum `tools/call` params or arguments payload. |
| `MCP_MAX_WRITE_BYTES` | `10485760` | Maximum `write_file` content bytes. |
| `MCP_MAX_LIST_ENTRIES` | `1000` | Maximum policy-visible entries returned by `list_directory`. |
| `MCP_MAX_COPY_BYTES` | `10485760` | Maximum `copy_path` source file size. |
| `MCP_MAX_DELETE_ENTRIES` | `1000` | Maximum entries allowed for recursive `delete_path`. |
| `MCP_MAX_RESPONSE_BYTES` | `16777216` | Safety net for serialized JSON-RPC responses. |

All limit values must be positive integers.

`MCP_DEBUG=true` enables minimal diagnostics on stderr. Diagnostics are redacted for common credentials, tokens, private-key markers, connection strings, and host paths. Normal MCP operation still writes only JSON-RPC protocol messages to stdout. No persistent logfiles are created.

### Read-only mode

`SPR-035` adds read-only enforcement for MCP tool discovery and direct tool calls.

Enable read-only mode with:

```text
MCP_READ_ONLY=true
```

When read-only mode is enabled, only these tools are registered and returned by `tools/list`:

```text
list_directory
read_file
get_path_info
```

Write-capable tools are not registered in read-only mode, so direct `tools/call` requests for `write_file`, `create_directory`, `delete_path`, `copy_path`, or `move_path` are rejected with a generic Invalid params error without revealing whether the tool exists in another mode.

## Tool Documentation

The MCP tool interface is documented in:

```text
docs/tools.md
```

Tool implementation and naming conventions are documented in:

```text
docs/tool-conventions.md
```

A machine-readable MCP tool catalog is available at:

```text
docs/mcp-tool-catalog.json
```

Preparation for a later, separately approved Codex read-only activation is documented in [docs/codex-read-only-activation.md](docs/codex-read-only-activation.md). `SPR-044` does not modify Codex configuration or register FlashGate as an MCP server.

The catalog contains tool names, descriptions, input schemas, domain `resultSchema` values, the central `CallToolResult` envelope description, and common error behavior. Runtime output schemas are exposed for the current tools. Version 1.0 also adds profile-specific catalog/instruction budgets, deterministic ordering, and catalog fingerprints.

## Project Planning

Planned work is tracked in:

```text
BACKLOG.md
```

The backlog is the authoritative planning document for upcoming filesystem, process, command execution, system information, security, CI, release, and documentation work.

Project history is tracked in:

```text
CHANGELOG.md
```

`README.md`, `CHANGELOG.md`, and `BACKLOG.md` should be kept current as part of the normal sprint workflow.

Architecture and security references:

- [Architecture](docs/architecture.md)
- [Version 1.0 scope and release boundary](docs/version-1-scope-and-release-boundary.md)
- [Efficiency improvement plan](docs/efficiency-improvement-plan.md)
- [Execution identity backends](docs/execution-identity-backends.md)
- [Native runtime and service plan](docs/native-multi-mode-runtime-and-service-plan.md)
- [Comparative MCP review](docs/comparative-mcp-review-2026-07-17.md)
- [Security model](docs/security.md)
- [Code coverage](docs/development/code-coverage.md)
- [Protocol and local transport](docs/protocol.md)
- [Version 1.0 product and technical specification](docs/specification.md)
- [Architecture decisions](docs/adr/)
- [Authoritative backlog](BACKLOG.md)
- [High-level roadmap](docs/roadmap.md)
- [Project identity](docs/project-identity.md)
- [Documentation quality gate](docs/documentation-quality-gate.md)

## Project Structure

```text
cmd/
  server/               Application entry point

internal/
  config/               Runtime configuration
  diagnostics/          Redacted stderr diagnostics helpers
  fs/                   Filesystem abstraction and implementation
  mcp/                  MCP server, router, handlers, transport, and tools
  protocol/             JSON-RPC and MCP protocol types
  security/             Root-confined path validation
  version/              Build and release metadata

docs/
  architecture.md       Current and Version 1.0 target architecture
  version-1-scope-and-release-boundary.md
                        Version 1.0 versus post-Version-1.0 scope
  efficiency-improvement-plan.md
                        Payload, token, RAM, CPU, and native-adapter plan
  execution-identity-backends.md
                        Service-account and future user-worker architecture
  native-multi-mode-runtime-and-service-plan.md
                        STDIO/proxy/auto/system-service plan
  protocol.md           MCP and local IPC protocol architecture
  specification.md      Consolidated Version 1.0 requirements
  coding-style.md       Go, native adapter, lifecycle, and payload rules
  development/
    code-coverage.md  Windows/Linux coverage gates and maintenance rules
  mcp-tool-catalog.json Machine-readable MCP tool catalog
  tool-conventions.md   Tool implementation and interface conventions
  tools.md              Human-readable MCP tool reference
```

## Requirements

- Go version required by `go.mod` (currently Go 1.26.4 or newer)

## Building

Clone the repository:

```bash
git clone https://github.com/thomasweidner/flashgate-mcp.git
cd flashgate-mcp
```

Build the server:

```bash
go build -o build/flashgate-mcp ./cmd/server
```

On Windows, the common build command is:

```powershell
go build -o build/flashgate-mcp.exe ./cmd/server
```

Direct `go build` is intended for development. Controlled metadata and release builds use `scripts/build.ps1` or `scripts/build.sh`; see [Build and release metadata](docs/build-and-release-metadata.md).

Version identity is available without starting the MCP STDIO loop:

```text
flashgate-mcp --version
flashgate-mcp --version --verbose
```

## Testing and Quality Checks

FlashGate's public checkout contains the normal product, security, release,
metadata, platform, shell, and documentation validation gates. Start with the
directly affected checks and finish with the applicable repository-wide chain
below. No private development environment is required.

Run the documentation consistency gate with PowerShell 7.6.x (Major 7, Minor 6):

```powershell
.\scripts\Test-DocumentationConsistency.ps1
```

The detailed checklist and exit-code contract are documented in [docs/documentation-quality-gate.md](docs/documentation-quality-gate.md).

Validate the complete PowerShell and Bash entry-point inventory on Windows
with the required PowerShell 7.6 LTS line and Git Bash runtimes:

```powershell
& {
    $workRoot = $env:FLASHGATE_WORK_ROOT
    if ([string]::IsNullOrWhiteSpace($workRoot)) { throw 'Bind FLASHGATE_WORK_ROOT first.' }
    .\scripts\Test-ShellScripts.ps1
    .\scripts\Test-ShellScripts.Tests.ps1 -WorkingPath $workRoot
}
```

Native Ubuntu CI runs the matching `/usr/bin/bash`
`scripts/test-shell-scripts.sh` and `scripts/test-shell-scripts.tests.sh`
gates. The full policy, negative-test matrix, and fallback when optional
linters are unavailable are documented in [Testing](docs/testing.md#shell-script-validation).
The bounded subprocess helper verifies termination after a timeout and never
waits indefinitely for redirected streams after an unconfirmed process-tree
termination. The persistent PowerShell harness covers 21 focused cases; the
native Bash harness also verifies fail-closed cleanup reporting with exactly
one terminal result block.

Run the standard validation chain:

```bash
go fmt ./...
go vet ./...
go test ./...
golangci-lint run
```

Build after validation:

```bash
go build -o build/flashgate-mcp ./cmd/server
```

On Windows:

```powershell
go build -o build/flashgate-mcp.exe ./cmd/server
```

Relevant platform, build, test, lint, workflow, filesystem, and security-boundary
changes require the native Linux checks described in
[Testing](docs/testing.md#native-linux-validation). Run those checks in a native
Linux checkout; do not treat a Windows-mounted filesystem as native Linux
evidence.

### Code Coverage

FlashGate enforces separate production-server Go statement-coverage gates for Windows and Linux. The current minimum values are:

| Platform | Minimum |
|---|---:|
| Windows | 90.0% |
| Linux | 90.0% |

Run the Windows coverage gate with PowerShell 7.6.x (Major 7, Minor 6):

```powershell
.\scripts\Test-GoCoverage.ps1 -PlatformName windows -MinimumCoverage 90.0
```

Run the Linux gate with PowerShell 7 on the native Linux validation environment:

```bash
pwsh -NoLogo -NoProfile -File ./scripts/Test-GoCoverage.ps1 -PlatformName linux -MinimumCoverage 90.0
```

Each run writes `coverage.out`, `coverage.txt`, `coverage.html`, `test.log`, `product-packages.txt`, and `summary.json` below `build/coverage/<platform>/`. GitHub Actions uploads separate Windows and Linux artifacts for 14 days.

The technically authoritative thresholds are maintained in `.github/workflows/ci.yml`. Windows and Linux values are evaluated independently and must not be averaged. Detailed operation and baseline-maintenance rules are documented in [docs/development/code-coverage.md](docs/development/code-coverage.md).

Run the JSON-RPC smoke test on Windows:

```powershell
.\scripts\smoke-jsonrpc.ps1
```

Run the same smoke test in Windows read-only mode:

```powershell
$env:MCP_READ_ONLY = "true"
.\scripts\smoke-jsonrpc.ps1
Remove-Item Env:\MCP_READ_ONLY
```

On Linux, build the non-`.exe` binary and run:

```bash
bash scripts/smoke-jsonrpc.sh
MCP_READ_ONLY=true bash scripts/smoke-jsonrpc.sh
```

The default smoke test starts the built server binary and sends JSON-RPC requests through STDIO. It verifies that the server responds to:

```text
initialize
tools/list
```

The script also validates the negotiated protocol version, the registered MCP tool list, and every positive `tools/call` result as text-plus-`structuredContent` with deterministic semantic parity.

Run the negative JSON-RPC smoke test on Windows:

```powershell
.\scripts\smoke-jsonrpc-negative.ps1
```

Run fail-closed startup validation:

```powershell
.\scripts\smoke-startup-negative.ps1
```

On Linux:

```bash
bash scripts/smoke-jsonrpc-negative.sh
```

Linux startup validation uses:

```bash
bash scripts/smoke-startup-negative.sh
```

The negative smoke test verifies malformed JSON, unknown methods, invalid `tools/call` params, and notification no-response behavior.

The smoke scripts create per-run JSONL request and response files under `build/` and remove them before exit. Script status output goes to the shell or CI log; the server process still writes only JSON-RPC protocol data to its redirected stdout stream.

## Resource, Latency, and Payload Benchmarks

`SPR-047` adds a versioned local benchmark system for process startup, end-to-end workflow latency, idle and peak working set, process CPU time, Go allocations, request/result/response sizes, filesystem counters, `tools/list` size, MCP call counts, and a coarse byte-based token orientation.

Run the standard Windows benchmark after normal validation:

```powershell
& ".\scripts\benchmark.ps1"
```

Use the 10-repetition quick mode during development:

```powershell
& ".\scripts\benchmark.ps1" -Quick
```

On Linux:

```bash
bash scripts/benchmark.sh
bash scripts/benchmark.sh --quick
```

The standard diagnostic mode records one `first_process_start` and 30 `subsequent_process_start` samples. Quick mode records one first and 10 subsequent samples. The first label means the first process started by that benchmark command; it does not claim an enforced OS cold cache.

Detailed counter semantics, platform behavior, reference workflows, result schema, baseline, and hard-versus-soft budgets are documented in [`benchmarks/README.md`](benchmarks/README.md). The approximation `approx_tokens_bytes4 = ceil(UTF-8 bytes / 4)` is not model-specific and is not suitable for billing.

Versioned baselines are recorded only from clean isolated checkouts of the same implementation commit using the documented two-phase prebuilt workflow. Validation and builds finish before a minimum 180-second quiet period, one authoritative three-block host preflight, direct measurement with prepared binaries, an intermediate gate, and a final host gate. The caller supplies an explicit task-bound Windows workspace on local nonsynchronized storage; native Linux work remains under `/home`. No personal global benchmark path is project authority. The legacy `-RecordBaseline` and `--record-baseline` wrapper flags fail closed and cannot create baselines. Ordinary local runs may use a dirty tree and record that provenance explicitly.

`cmd/benchmark` is development-only. Diagnostic wrappers build it locally; authoritative runs invoke a separately prepared binary. It is not included in release artifacts.

## Release Builds

Release tags use `v<SemVer>`. The leading `v` belongs to the Git tag but is not embedded in product versions or artifact names. Release mode requires the exact tag and a clean tree; local integration validation may use an explicit SemVer and records `Modified: true`.

The tag-gated `.github/workflows/release-build.yml` workflow builds and validates:

```text
flashgate-mcp_<version>_windows_x64.zip
flashgate-mcp_<version>_windows_arm64.zip
flashgate-mcp_<version>_linux_x64.tar.gz
flashgate-mcp_<version>_linux_arm64.tar.gz
```

Each archive has a sibling `.sha256` file and exactly one top-level directory containing the platform binary, `LICENSE`, `README.md`, and `THIRD-PARTY-NOTICES.md`. Windows artifacts carry `VERSIONINFO`, an embedded machine-readable build manifest, and the canonical FlashGate icon; Linux artifacts carry matching CLI, embedded-manifest, Go/VCS, ELF architecture, and Go build-ID metadata. Before upload, every matrix path builds independently twice, validates metadata and exact archive contents, compares binary, archive, checksum-file, and inventory identities, and performs a machine-readable host/credential leak scan.

Detailed build inputs, architecture naming, reproducibility rules, local commands, and manual Explorer validation are documented in [Build and release metadata](docs/build-and-release-metadata.md), [Artifact verification](docs/artifact-verification.md), and [Manual metadata validation](docs/manual-metadata-validation.md).

Historical project and release milestones remain available in `BACKLOG.md` and
`CHANGELOG.md`; they do not add private prerequisites to the current public
build and release process.

## Basic Usage

Set the filesystem root:

```powershell
$env:MCP_ROOT = "C:\Path\To\Allowed\Root"
```

The value must be an explicit absolute directory. For development only, `MCP_ROOT=.` additionally requires `MCP_ALLOW_CWD_ROOT=true`; client activation examples do not use that opt-in.

For read-only operation:

```powershell
$env:MCP_READ_ONLY = "true"
```

Run the server:

```powershell
.\build\flashgate-mcp.exe
```

The process expects JSON-RPC messages on standard input and writes JSON-RPC responses to standard output.

## CLI Help and Version Information

The binary supports a dedicated help mode:

```powershell
.\build\flashgate-mcp.exe --help
```

Short form:

```powershell
.\build\flashgate-mcp.exe -h
```

Example output:

```text
flashgate-mcp

Usage:
  flashgate-mcp
  flashgate-mcp --version
  flashgate-mcp --help

Environment:
  MCP_ROOT             Required absolute root directory exposed to MCP clients
  MCP_READ_ONLY        Set to true to expose only read-only filesystem tools
  MCP_ALLOW_CWD_ROOT   Development only: set to true with MCP_ROOT=.
```

The binary also supports a dedicated version mode:

```powershell
.\build\flashgate-mcp.exe --version
```

A local development build without embedded release metadata prints default values:

```text
flashgate-mcp
version: dev
commit: unknown
date: unknown
```

Release builds embed the version label, Git commit SHA, and UTC build date:

```text
flashgate-mcp
version: v0.1.0-test
commit: d9342ef1f1c4ebf03c2716f11d10b7fdb8dd316a
date: 2026-07-05T20:02:54Z
```

The `--version` mode is intended for diagnostics and artifact traceability. Normal MCP operation still communicates exclusively through JSON-RPC over STDIO.

## Example JSON-RPC Request

Example `tools/list` request:

```json
{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}
```

On Windows, a simple smoke test can be executed with a JSONL file:

```powershell
$json = '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'
[System.IO.File]::WriteAllText("$PWD\request-tools-list.jsonl", $json + "`n", [System.Text.UTF8Encoding]::new($false))
cmd /c ".\build\flashgate-mcp.exe < request-tools-list.jsonl"
Remove-Item request-tools-list.jsonl
```

## Development Workflow

The project uses small, focused feature branches.

Typical workflow:

```bash
git checkout -b feature-sprint-name
go fmt ./...
go vet ./...
go test ./...
golangci-lint run
go build -o build/flashgate-mcp ./cmd/server
git add .
git commit -m "Describe focused change"
git checkout main
git merge feature-sprint-name
git branch -d feature-sprint-name
git push
```

Each feature should include:

- focused implementation
- unit tests
- documentation updates when interfaces change
- successful formatting, vetting, tests, linting, and build

## Implemented MCP Tools

| Tool | Description |
|---|---|
| `list_directory` | Lists files and directories below the configured filesystem root. |
| `read_file` | Reads a text file below the configured filesystem root. |
| `get_path_info` | Returns existence and metadata; missing paths return `exists:false`. |
| `write_file` | Writes a text file. |
| `create_directory` | Creates a directory and reports whether it was newly created. |
| `delete_path` | Deletes a file or directory. |
| `copy_path` | Copies a file. Directory copy is currently unsupported. |
| `move_path` | Moves or renames a file or directory on the same volume. |

When `MCP_READ_ONLY=true`, only `list_directory`, `read_file`, and `get_path_info` are exposed.

## Roadmap

Planned work is maintained authoritatively in [BACKLOG.md](BACKLOG.md). [docs/roadmap.md](docs/roadmap.md) contains only the high-level sequence.

The backlog covers Version 1.0 filesystem, search, process, typed command, system, security, payload-efficiency, hybrid identity, service, CI, supply-chain, release, and documentation work. Tasks marked `Later` are accepted post-Version-1.0 work and do not delay the initial stable release.

## License

This project is licensed under the GNU General Public License v3.0.

See `LICENSE` for details.

## PowerShell 7.6 LTS patch contract

Compatibility requires PowerShell major version 7 and minor version 6. `ObservedPowerShellVersion` records the actual patch. `MinimumPowerShellVersion` is `null` unless a specific fix establishes and justifies a minimum patch. `ServicingTarget=LatestServicedPatchWithin7.6` is the maintenance target. Exact patch, path, or hash bindings are allowed only for historical evidence, bug reproduction, installer/download/SBOM/supply-chain provenance, or a documented minimum-patch fix; each exception must be explicitly classified.
