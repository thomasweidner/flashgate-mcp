# FlashGate MCP

**Fast, secure and local-first host operations for MCP.**

FlashGate MCP is an open-source, portable, native Go MCP server for controlled
local host operations. Today it provides secure, root-confined filesystem access:
connect an MCP client, choose a directory, and let the client list files, read
text, or inspect metadata within that boundary.

Native Windows and Linux binaries run directly over STDIO without Node.js, npm,
or a Go installation. FlashGate enforces configured path and operation boundaries
on the server, with limits on requests, file operations, and responses. Its
vendor-neutral core has no proprietary client or infrastructure dependency.

**Start with the [public prerelease downloads](https://github.com/thomasweidner/flashgate-mcp/releases).**
[Version 0.3.0](https://github.com/thomasweidner/flashgate-mcp/releases/tag/v0.3.0)
is an available prerelease, not a stable Version 1.0 release. The examples below
use that exact version. Set `MCP_READ_ONLY=true` explicitly for initial use;
the current default otherwise exposes write-capable tools too.

## Contents

- [Download and verify](#download-and-verify)
- [Quickstart for Windows and Linux](#quickstart-for-windows-and-linux)
- [Connect your MCP client](#connect-your-mcp-client)
- [Try your first task](#try-your-first-task)
- [Security by design](#security-by-design)
- [Current capabilities](#current-capabilities)
- [Documentation and roadmap](#documentation-and-roadmap)
- [Development, testing, and contributing](#development-testing-and-contributing)
- [License](#license)

## Download and verify

Choose a release on the [Releases page](https://github.com/thomasweidner/flashgate-mcp/releases).
Pre-1.0 releases are prereleases. Download **your platform archive and
`SHA256SUMS` from the same release**, keeping their original filenames in one
directory. You do not need the other three platform archives.

For [v0.3.0](https://github.com/thomasweidner/flashgate-mcp/releases/tag/v0.3.0),
the five uploaded assets are:

| Platform | Archive |
|---|---|
| Windows x64 | `flashgate-mcp_0.3.0_windows_x64.zip` |
| Windows ARM64 (Preview) | `flashgate-mcp_0.3.0_windows_arm64.zip` |
| Linux x64 | `flashgate-mcp_0.3.0_linux_x64.tar.gz` |
| Linux ARM64 (Preview) | `flashgate-mcp_0.3.0_linux_arm64.tar.gz` |
| Shared checksums | `SHA256SUMS` |

GitHub-generated **Source code** downloads are separate source archives.
There are no individual public `.sha256` files or extra public manifest/SBOM
assets. ARM64 archives are available, but cross-build/static verification does
not establish native execution on an ARM64 host; see the
[architecture validation targets](docs/adr/product-metadata.md#target-architectures).

Verify the **archive bytes before extraction or execution**. SHA-256 checks
that the archive matches the reference hash you use. A checksum downloaded
alongside an archive is not a signature or independent publisher authentication;
establish trust in the release channel separately.

### Windows verification

In PowerShell, from the directory containing the selected ZIP and `SHA256SUMS`:

```powershell
& {
    $archive = 'flashgate-mcp_0.3.0_windows_x64.zip'
    $pattern = '^[0-9a-f]{64}  ' + [regex]::Escape($archive) + '$'
    $records = @(Get-Content -LiteralPath '.\SHA256SUMS' |
        Where-Object { $_ -cmatch $pattern })
    if ($records.Count -ne 1) { throw 'Expected exactly one selected-archive checksum.' }
    $expected = $records[0].Substring(0, 64)
    $actual = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -cne $expected) { throw 'SHA-256 mismatch. Do not extract or run this archive.' }
    "SHA-256 OK: $archive"
}
```

For Windows ARM64, change only the archive name to the matching table entry.
Continue only after `SHA-256 OK`.

### Linux verification

With GNU `sha256sum`, from a directory containing only your selected TAR.GZ
and `SHA256SUMS`:

```bash
(
    set -eu
    archive='flashgate-mcp_0.3.0_linux_x64.tar.gz'
    test -f "$archive"
    awk -v name="$archive" '$2 == name { count++ } END { exit(count != 1) }' SHA256SUMS
    sha256sum --ignore-missing --check SHA256SUMS
)
```

For Linux ARM64, change only the archive name. Expect the selected archive's
name followed by `OK` and exit status 0. Stop on a missing/duplicate selected
record, missing archive, malformed checksums, or mismatch.

Contributors can also use the existing
[selected-platform verifier](docs/prerelease-publication.md#selected-platform-download-verification).
The [publication guide](docs/prerelease-publication.md#download-and-verify)
explains current downloads and preserves the original publication history.

## Quickstart for Windows and Linux

You need a matching verified binary and a local MCP client that can launch a
STDIO server. The server itself needs no Node.js/npm runtime, interpreter,
network listener, WSL, or administrator session. Building from source has
[separate developer requirements](#development-testing-and-contributing).

Each archive contains one versioned directory with the binary, `LICENSE`,
`README.md`, and `THIRD-PARTY-NOTICES.md`. Extract to a user-writable location
**outside the data directory**. The paths below are examples, not installation
defaults; replace them with your actual absolute paths. Choose a small existing
data directory containing a harmless text file such as `hello.txt` for the first
test. Avoid granting an entire drive, home directory, or synchronized storage
tree. See [root prerequisites](docs/client-setup.md#prerequisites).

### Windows quickstart

After verifying the Windows x64 ZIP, choose an absent destination under a
directory you own. Replace the example destination below. Existing destinations
are rejected, including a tree containing an older binary. Do not modify the
destination concurrently while these checks run.

```powershell
& {
    $destination = 'C:\Tools\FlashGate'
    $process = $null
    try {
        if (Test-Path -LiteralPath $destination -ErrorAction Stop) {
            throw 'Destination already exists. Choose a new, absent directory.'
        }
        $archive = (Get-Item -LiteralPath '.\flashgate-mcp_0.3.0_windows_x64.zip' -ErrorAction Stop).FullName
        New-Item -ItemType Directory -Path $destination -ErrorAction Stop | Out-Null
        Expand-Archive -LiteralPath $archive -DestinationPath $destination -ErrorAction Stop
        $binary = Join-Path $destination 'flashgate-mcp_0.3.0_windows_x64\flashgate-mcp.exe'
        $binary = (Get-Item -LiteralPath $binary -ErrorAction Stop).FullName
        foreach ($argument in @('--version', '--help')) {
            $startInfo = [System.Diagnostics.ProcessStartInfo]::new()
            $startInfo.FileName = $binary
            $startInfo.UseShellExecute = $false
            $startInfo.ArgumentList.Add($argument)
            $process = [System.Diagnostics.Process]::new()
            $process.StartInfo = $startInfo
            if (-not $process.Start()) { throw 'Binary could not be started.' }
            $process.WaitForExit()
            $exitCode = $process.ExitCode
            $process.Dispose()
            $process = $null
            if ($exitCode -ne 0) { throw "Binary $argument failed with exit code $exitCode." }
        }
        'Quickstart checks passed.'
    }
    catch {
        throw
    }
    finally {
        if ($null -ne $process) { $process.Dispose() }
    }
}
```

Continue only after `Quickstart checks passed.`, following `flashgate-mcp 0.3.0`
and CLI help. Any error stops the block before later checks; a partial extraction
must not be reused. The block adds no cleanup or permission command: inspect any
remaining output and choose a different absent destination before retrying.
For ARM64, use the ARM64 archive
and versioned directory instead. No installer or service registration is needed.

Use that executable path and an existing data directory, for example
`C:\FlashGateData\ReadOnlyRoot`, in a [client example](#connect-your-mcp-client).
To inspect full build identity, add `--version --verbose`. Version/help modes
exit without starting an MCP session.

### Linux quickstart

After verifying the Linux x64 TAR.GZ, replace `/home/example/tools` with an
absent directory under an existing parent you own. `mkdir` intentionally has no
`-p`: an existing destination, even one containing a binary, must stop the block.
Do not modify the destination concurrently.

```bash
(
    set -eu
    archive='flashgate-mcp_0.3.0_linux_x64.tar.gz'
    destination='/home/example/tools'
    test -f "$archive"
    mkdir -- "$destination"
    tar -xzf "$archive" -C "$destination"
    "$destination/flashgate-mcp_0.3.0_linux_x64/flashgate-mcp" --version
    "$destination/flashgate-mcp_0.3.0_linux_x64/flashgate-mcp" --help
    printf '%s\n' 'Quickstart checks passed.'
)
```

Continue only after `Quickstart checks passed.`, following `flashgate-mcp 0.3.0`
and CLI help. Extraction or binary failure aborts the subshell, with no later
binary call or success message. Partial output is retained for inspection;
choose a different absent destination before retrying. No cleanup or permission
command is run. For ARM64, use its archive and directory. The extracted binary
needs execute permission on a filesystem that
allows execution; see [client setup](docs/client-setup.md#stdio-launch-contract).

For the Linux client examples below, the extracted directory is assumed to be
at `/home/example/tools/flashgate-mcp_0.3.0_linux_x64`, and the existing data
directory at `/home/example/flashgate-readonly-root`. Substitute your real
absolute paths. These are native Linux examples; keep Windows paths separate.

## Connect your MCP client

The client launches the binary with **no arguments** and supplies its environment.
Normal invocation waits for newline-delimited JSON-RPC on stdin; an idle terminal
with no banner does not prove a successful MCP connection. Stdout is reserved
for protocol messages; diagnostics use stderr.

Use one platform example for your client. Back up its existing configuration,
add or update only the `flashgate` entry, and preserve unrelated settings and
servers. The examples describe an operator-applied change, not an automatic
registration. Never put the binary inside `MCP_ROOT`, use `MCP_ROOT=.`, or
recommend a build/`go run` on every startup. Detailed acceptance and rollback
remain in [client setup](docs/client-setup.md).

### Codex: config.toml

Add the entry to your user `~/.codex/config.toml` (Windows:
`%USERPROFILE%\.codex\config.toml`). Follow the official
[Codex MCP guide](https://developers.openai.com/codex/mcp) and
[configuration reference](https://developers.openai.com/codex/config-reference).
The `command` is one absolute executable path; environment values are strings.

**Windows x64 example** (TOML literal strings preserve backslashes):

```toml
[mcp_servers.flashgate]
command = 'C:\Tools\FlashGate\flashgate-mcp_0.3.0_windows_x64\flashgate-mcp.exe'
args = []

[mcp_servers.flashgate.env]
MCP_ROOT = 'C:\FlashGateData\ReadOnlyRoot'
MCP_READ_ONLY = "true"
MCP_ALLOW_CWD_ROOT = "false"
MCP_ALLOW_HIDDEN_FILES = "false"
MCP_ALLOW_UNC_PATHS = "false"
MCP_FOLLOW_SYMLINKS = "false"
MCP_DEBUG = "false"
```

**Linux x64 example** (alternative entry for a native Linux client):

```toml
[mcp_servers.flashgate]
command = "/home/example/tools/flashgate-mcp_0.3.0_linux_x64/flashgate-mcp"
args = []

[mcp_servers.flashgate.env]
MCP_ROOT = "/home/example/flashgate-readonly-root"
MCP_READ_ONLY = "true"
MCP_ALLOW_CWD_ROOT = "false"
MCP_ALLOW_HIDDEN_FILES = "false"
MCP_ALLOW_UNC_PATHS = "false"
MCP_FOLLOW_SYMLINKS = "false"
MCP_DEBUG = "false"
```

Reload/restart the client as required by its version, then inspect its MCP
server/tool list. In Codex CLI, `/mcp` shows active MCP servers. Continue with
[the first task](#try-your-first-task) after `flashgate` connects.

### Claude Desktop: claude_desktop_config.json

Open **Settings → Developer → Edit Config** to locate
`claude_desktop_config.json`. On Windows, the documented location is
`%APPDATA%\Claude\claude_desktop_config.json`; see the official
[local-server integration guide](https://modelcontextprotocol.io/docs/2026-07-28/develop/connect-local-servers).
**Claude Desktop for Linux is in beta**, as documented in Anthropic's
[installation and availability guide](https://support.claude.com/en/articles/10065433-install-claude-desktop).
On Linux, use the file opened by **Edit Config** in your actual client version
instead of assuming a fixed configuration path. These configuration examples
have not been tested in a real Claude Desktop session.
The MCP guide's Node.js-based filesystem-server example is a different server; FlashGate
runs its own native executable.

**Windows x64 example** (JSON backslashes must be escaped):

```json
{
  "mcpServers": {
    "flashgate": {
      "command": "C:\\Tools\\FlashGate\\flashgate-mcp_0.3.0_windows_x64\\flashgate-mcp.exe",
      "args": [],
      "env": {
        "MCP_ROOT": "C:\\FlashGateData\\ReadOnlyRoot",
        "MCP_READ_ONLY": "true",
        "MCP_ALLOW_CWD_ROOT": "false",
        "MCP_ALLOW_HIDDEN_FILES": "false",
        "MCP_ALLOW_UNC_PATHS": "false",
        "MCP_FOLLOW_SYMLINKS": "false",
        "MCP_DEBUG": "false"
      }
    }
  }
}
```

**Linux x64 example** (Claude Desktop for Linux beta):

```json
{
  "mcpServers": {
    "flashgate": {
      "command": "/home/example/tools/flashgate-mcp_0.3.0_linux_x64/flashgate-mcp",
      "args": [],
      "env": {
        "MCP_ROOT": "/home/example/flashgate-readonly-root",
        "MCP_READ_ONLY": "true",
        "MCP_ALLOW_CWD_ROOT": "false",
        "MCP_ALLOW_HIDDEN_FILES": "false",
        "MCP_ALLOW_UNC_PATHS": "false",
        "MCP_FOLLOW_SYMLINKS": "false",
        "MCP_DEBUG": "false"
      }
    }
  }
}
```

These are complete minimal examples for an otherwise empty configuration; add
the entry to your existing `mcpServers` object rather than replacing the file.
Completely quit and restart Claude Desktop, then inspect FlashGate's available
tools in its connector controls.

## Try your first task

A successful connection completes MCP `initialize`, reports
`serverInfo.name=flashgate` and negotiated revision `2025-11-25`, and lets the
client discover exactly these three tools in read-only mode, in order:

1. `list_directory`
2. `read_file`
3. `get_path_info`

A client may display a server prefix on tool names. If write tools appear,
check that this server entry explicitly sets `MCP_READ_ONLY=true` and reload it.

Ask your client:

> Use FlashGate to list the files in the configured directory and show
> information about one file. Do not modify anything.

For a harmless text file named `hello.txt` already inside the root, follow with:

> Read hello.txt through FlashGate, with a maximum of 4096 bytes.
> Do not modify anything.

The corresponding current tool calls are:

| Tool | Arguments | Check |
|---|---|---|
| `list_directory` | `{"path":"."}` | `entries` lists policy-visible names relative to the root. |
| `get_path_info` | `{"path":"hello.txt"}` | `exists:true`, filename, type, and size match your file. |
| `read_file` | `{"path":"hello.txt","maxBytes":4096}` | `content` matches the text; `size` reports returned bytes. |

Choose a small file no larger than 4096 bytes for this example. All arguments are
relative to `MCP_ROOT`, not absolute host paths. In a protocol trace, successful
`tools/call` results have one JSON text block in MCP `content[]` and the same
domain object in `structuredContent`. A missing metadata path returns
`exists:false`; that alone does not verify a real-file read.

Confirm the client actually used FlashGate and that the data directory is
unchanged. A direct CLI/version check alone is not an end-to-end client test.
For startup categories, missing tools, or path denials, use
[startup troubleshooting](docs/client-setup.md#startup-troubleshooting),
[acceptance checks](docs/client-setup.md#acceptance-checks), and
[rollback](docs/client-setup.md#rollback). Tool arguments and errors are detailed
in the [tool reference](docs/tools.md).

## Security by design

- **Explicit root:** `MCP_ROOT` must be an existing absolute directory that
  passes policy. Missing, relative, nonexistent, file, or denied roots fail
  before tool registration. Expected configuration failures exit with code 3,
  leave stdout empty, and report a safe category on stderr.
- **Server-side confinement:** relative tool paths pass lexical and effective
  containment checks. Traversal, absolute tool paths, and out-of-root resolution
  are rejected. Tools use the central filesystem abstraction and path guard.
- **Explicit read-only operation:** `MCP_READ_ONLY=true` registers only the
  three read tools. Write-capable names are unavailable even when called
  directly. Tool annotations and visibility are not authorization.
- **Restrictive path policy:** the examples deny hidden files, UNC paths,
  symlinks, junctions, and other reparse access. Listing filters denied entries.
  Enabling symlink following still cannot authorize an out-of-root target.
- **Bounded work and disclosure:** configurable limits cap JSON-RPC messages,
  arguments, reads/writes, listings, copies, recursive deletes, and responses.
  Errors are generic; optional diagnostics are redacted and use stderr.
  `MCP_DEBUG=false` keeps initial operation quiet. Current diagnostics create no
  persistent logfiles.

FlashGate runs under the launching OS account. Root and operation checks are
application controls, not OS process isolation or protection from a compromised
client/account. Grant only the data access you intend. Read the
[security model](docs/security.md) for limits, policy details, residual races,
and current-versus-target controls.

## Current capabilities

The available filesystem tools are:

| Tool | Behavior | In read-only mode |
|---|---|---|
| `list_directory` | List one directory; no recursive search or pagination. | Yes |
| `read_file` | Read bounded text; no range-read interface. | Yes |
| `get_path_info` | Inspect existence, name, type, and size. | Yes |
| `write_file` | Write text; overwrite requires `overwrite:true`. | No |
| `create_directory` | Create a directory and missing parents. | No |
| `delete_path` | Delete a file/directory; nonempty directories require `recursive:true`. | No |
| `copy_path` | Copy a file; directory copy is unsupported. | No |
| `move_path` | Move/rename on the same volume; no cross-volume copy/delete fallback. | No |

All tools remain subject to path policies and resource limits. See
[tools](docs/tools.md), the [machine-readable catalog](docs/mcp-tool-catalog.json),
and [tool conventions](docs/tool-conventions.md) for exact schemas and behavior.

The production policy enables and advertises only MCP `2025-11-25` over direct
STDIO. The compiled-in `2026-07-28` adapter is an internally validated candidate,
disabled and unadvertised pending BL-204, BL-212, and BL-219. See
[the protocol reference](docs/protocol.md) and
[advertised revision matrix](docs/mcp-protocol-matrix.json).

**Planned, not currently available:** Version 1.0 extends that foundation with
bounded search, managed processes, typed allowlisted execution, system
information, named roots/profiles, Operations/Jobs, payload-efficient large
results, and optional local service/proxy/auto roles. Generic shell/terminal
execution, remote MCP, installers, and automatic updates are not provided.
Future modules/providers are post-Version-1.0 work, separate from MCP protocol
extensions. Planned runtime flags are not current CLI arguments.

## Documentation and roadmap

The [documentation index](docs/README.md) is the main route to detailed guidance.

| Topic | Reference |
|---|---|
| Client acceptance, troubleshooting, rollback | [Client setup](docs/client-setup.md) |
| Tool input/output and errors | [Tools](docs/tools.md) and [catalog](docs/mcp-tool-catalog.json) |
| MCP and transport | [Protocol](docs/protocol.md) |
| Roots, policies, limits, threat models | [Security](docs/security.md) |
| Compatibility and early integrations | [Migration](docs/migration.md) |
| Architecture and rationale | [Architecture](docs/architecture.md) and [ADRs](docs/adr/README.md) |
| Product identifiers and vendor-neutral scope | [Project identity](docs/project-identity.md) |
| Release history | [Changelog](CHANGELOG.md) |
| Authoritative work status and sequence | [Backlog](BACKLOG.md) and [roadmap](docs/roadmap.md) |
| Initial stable release boundary | [Version 1.0 scope](docs/planning/release-scope.md) |
| Payload/resource targets and measurement | [Efficiency plan](docs/planning/efficiency.md) |
| Future runtime and execution identity | [Runtime plan](docs/planning/runtime-modes.md) and [identity backends](docs/execution-identity-backends.md) |
| Consolidated target requirements | [Specification](docs/specification.md) |

The backlog owns task status and milestones. Planned features and accepted ADRs
do not establish current availability. Stable Version 1.0 publication has its
own BL-263 gate; a prerelease does not pass it.

## Development, testing, and contributing

To build from source, use the Go version declared in [go.mod](go.mod)
(currently Go 1.26.4 or newer). Go is a developer requirement, not a requirement
for running verified binaries.

```bash
git clone https://github.com/thomasweidner/flashgate-mcp.git
cd flashgate-mcp
go build -o build/flashgate-mcp ./cmd/server
```

On Windows, use `go build -o build/flashgate-mcp.exe ./cmd/server`.
Direct development builds report `0.0.0-dev`. Controlled builds and release
identity use root [VERSION](VERSION); see [build metadata](docs/build-metadata.md).
The repository is `thomasweidner/flashgate-mcp`, the Go module is
`github.com/thomasweidner/flashgate-mcp`, the executable is `flashgate-mcp`,
and MCP `serverInfo.name` is `flashgate`. The project maintains its own Go MCP
adapter without an external MCP framework.

The application entry point is `cmd/server`. Runtime packages separate
configuration (`internal/config`), redacted diagnostics (`internal/diagnostics`),
filesystem operations (`internal/fs`), path enforcement (`internal/security`),
MCP routing/tools/transport (`internal/mcp`), wire types (`internal/protocol`),
and build identity (`internal/version`). `docs/` contains the references above.

Start validation in the affected area. For documentation changes:

```bash
go test ./cmd/doccheck
go run ./cmd/doccheck
```

Then run `scripts/Test-DocumentationConsistency.ps1` with PowerShell 7.6.x.
The compatibility gate checks major 7/minor 6; record the actual patch and
maintain the latest serviced patch in that line. See
[documentation quality](docs/documentation-quality-gate.md) for automated and
editorial checks.

The normal code chain is `go fmt ./...`, `go vet ./...`, `go test ./...`,
`golangci-lint run`, and the platform build above. Follow
[Contributing](CONTRIBUTING.md) and [Testing](docs/testing.md) for applicable
Windows/Linux smoke, race, coverage, shell, and CI gates. Shell changes require
the documented deterministic shell-validation pair. Test scratch requires a
caller-bound `FLASHGATE_WORK_ROOT` outside the source tree.

Further contributor references:

- [Coding style](docs/coding-style.md) and [tool conventions](docs/tool-conventions.md).
- [Product coverage](docs/development/code-coverage.md): separate Windows/Linux
  hard gates of at least 90.0%, with a 95.0% target.
- [Benchmarks](benchmarks/README.md): startup, CPU/RAM, latency, payload/catalog,
  and approximate-token methodology; diagnostic runs and authoritative baselines
  have different workspace/preflight requirements.
- [Artifact verification](docs/artifact-verification.md) and
  [manual metadata checks](docs/metadata-validation.md): deterministic archives,
  repeated builds, native metadata, hashes, and leak checks.
- [Prerelease publication](docs/prerelease-publication.md): frozen version/source/
  artifact identity, five public assets, complete internal unsigned evidence,
  and separately authorized publication/recovery.

Release workflows and immutable candidates are contributor controls, not extra
end-user runtime prerequisites. No automatic update or installation is performed.

## License

FlashGate MCP is licensed under the GNU General Public License v3.0.
See [LICENSE](LICENSE) and [third-party notices](THIRD-PARTY-NOTICES.md).
