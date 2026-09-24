# Codex read-only activation preparation

This guide prepares a later FlashGate activation. `SPR-044` did not activate
FlashGate in Codex. None of the examples on this page has been applied to a
real Codex, Claude Desktop, or other client configuration.

Activate only after review, commit, pull request, merge, and post-merge checks,
with a separate approval for the client configuration change.

## 1. Prerequisites

- A merged and verified FlashGate commit.
- A reproducibly built Windows or Linux binary.
- A small, explicitly approved absolute root.
- Passing default, read-only, negative, and startup-negative smoke tests whose
  positive responses validate `CallToolResult.content[]` and `structuredContent`.
- A backup of the existing client configuration.
- A documented rollback.

The first activation uses neither `go run` nor an automatic build at MCP startup.

## 2. Build and verify the binary

Build the first activation binary with `-trimpath` from the exact merged `main`
commit. Before any later configuration change, verify at least:

```powershell
git rev-parse HEAD
go build -trimpath -o build\flashgate-mcp.exe ./cmd/server
.\build\flashgate-mcp.exe --help
.\build\flashgate-mcp.exe --version
Get-FileHash -Algorithm SHA256 .\build\flashgate-mcp.exe
```

With separate approval, copy the verified binary to a versioned path outside
the repository, for example:

```text
C:\Program Files\FlashGate\flashgate-mcp.exe
```

The binary path must be outside the approved data root. Remove temporary
build files after acceptance. A verified release binary is preferred for later
production use.

## 3. Choose a safe root

`MCP_ROOT` is required. The production root must be an absolute, existing,
accessible directory. Use a small dedicated root for first acceptance, for
example:

```text
C:\FlashGateData\ReadOnlyRoot
```

Do not use:

- An entire drive or home directory.
- An entire synchronized storage tree, such as OneDrive.
- A relative root or `.` without explicit development opt-in.
- A binary or build directory as the data root.

A synchronized storage root must be locally available and pass hidden-file,
UNC, symlink, junction, and reparse policy checks.

## 4. Required read-only profile

Set this explicitly for the first Codex activation:

```text
MCP_READ_ONLY=true
```

Without it, the normal default profile exposes eight tools. That remains the
current implementation behavior and is unsuitable for the first read-only
Codex activation.

Additional safe settings:

```text
MCP_ALLOW_CWD_ROOT=false
MCP_ALLOW_HIDDEN_FILES=false
MCP_ALLOW_UNC_PATHS=false
MCP_FOLLOW_SYMLINKS=false
MCP_DEBUG=false
```

The read-only profile exposes exactly these tools in this order:

1. `list_directory`
2. `read_file`
3. `get_path_info`

## 5. Windows Codex example — do not apply automatically

`command`, `args`, environment values, `startup_timeout_sec`, and
`codex mcp add` were verified locally during `SPR-044`. The manual `cwd` syntax
and `tool_timeout_sec` were not conclusively verified then; check both against
the installed Codex version immediately before actual activation.

```toml
# EXAMPLE ONLY — NOT APPLIED BY SPR-044
[mcp_servers.flashgate_readonly]
command = 'C:\Program Files\FlashGate\flashgate-mcp.exe'
args = []
startup_timeout_sec = 10

# Verify before use with the installed Codex version:
# cwd = 'C:\Program Files\FlashGate'
# tool_timeout_sec = 30

[mcp_servers.flashgate_readonly.env]
MCP_ROOT = 'C:\FlashGateData\ReadOnlyRoot'
MCP_READ_ONLY = 'true'
MCP_ALLOW_CWD_ROOT = 'false'
MCP_ALLOW_HIDDEN_FILES = 'false'
MCP_ALLOW_UNC_PATHS = 'false'
MCP_FOLLOW_SYMLINKS = 'false'
MCP_DEBUG = 'false'
```

`codex mcp add` is a verified mechanism, but `SPR-044` did not run it. Before
later use, check the backup, exact entry name, environment, timeouts, and
resulting configuration diff.

## 6. Linux and possible later WSL use

A Linux binary needs execute permission and an absolute Linux root:

```text
/opt/flashgate/current/flashgate-mcp
/home/example/flashgate-readonly-root
```

Pass paths with spaces as single configuration values. Symlinks remain
disabled by default. WSL2 was not installed by `SPR-044` and is not a
prerequisite. Any later WSL activation needs its own path, permission, binary,
and rollback checks; Windows and WSL paths must not be mixed implicitly.

## 7. Claude Desktop and general STDIO examples

These examples are preparation only. Verify the client's current syntax
before applying one.

Claude Desktop oriented Windows example:

```json
{
  "mcpServers": {
    "flashgate_readonly": {
      "command": "C:\\Path\\To\\Verified\\flashgate-mcp.exe",
      "args": [],
      "env": {
        "MCP_ROOT": "C:\\Path\\To\\Small\\Approved\\Root",
        "MCP_READ_ONLY": "true",
        "MCP_ALLOW_CWD_ROOT": "false",
        "MCP_ALLOW_HIDDEN_FILES": "false",
        "MCP_ALLOW_UNC_PATHS": "false",
        "MCP_FOLLOW_SYMLINKS": "false"
      }
    }
  }
}
```

Claude Desktop oriented Linux example:

```json
{
  "mcpServers": {
    "flashgate_readonly": {
      "command": "/opt/flashgate/current/flashgate-mcp",
      "args": [],
      "env": {
        "MCP_ROOT": "/home/example/flashgate-readonly-root",
        "MCP_READ_ONLY": "true",
        "MCP_ALLOW_CWD_ROOT": "false",
        "MCP_ALLOW_HIDDEN_FILES": "false",
        "MCP_ALLOW_UNC_PATHS": "false",
        "MCP_FOLLOW_SYMLINKS": "false"
      }
    }
  }
}
```

General local STDIO contract:

```text
transport = stdio
command = absolute path to verified flashgate-mcp binary
args = empty
environment.MCP_ROOT = absolute approved directory
environment.MCP_READ_ONLY = true
environment.MCP_ALLOW_CWD_ROOT = false
```

No example contains tokens, passwords, or authentication values.

## 8. Activation and acceptance tests

After a separately approved activation:

1. Before changing configuration, run a direct STDIO preflight with a strict
   `CallToolResult` decoder; valid JSON and domain fields alone are insufficient.
2. Check MCP discovery and `serverInfo.name=flashgate`.
3. Check exactly three tools in the documented order.
4. List the root and a subdirectory.
5. Read ordinary files and files with spaces or Unicode names.
6. Check file, directory, and missing-path metadata.
7. Confirm that traversal and absolute paths outside the root are denied.
8. Negatively test every write and removed legacy tool name.
9. Confirm that stdout contains only JSON-RPC.
10. Check stderr for safe categories and absence of host paths.
11. Check the root and repository for new files.

Write tool names:

```text
write_file
create_directory
delete_path
copy_path
move_path
```

Removed legacy names:

```text
list_files
stat_path
exists_path
mkdir
rename_path
```

All ten names must return the same generic Invalid params contract.

## 9. Read-only validation without a Go workflow

For reviewers who do not use Go tooling:

1. Compare binary origin, path, `--version`, and SHA-256 with the approval record.
2. Limit the root to the approved small directory.
3. Confirm `MCP_READ_ONLY=true` and `MCP_ALLOW_CWD_ROOT=false` visibly.
4. Run the provided PowerShell or Bash smoke tests.
5. Confirm the result lists contain exactly three tools.
6. Run a write attempt only through the negative tests and expect a generic error.
7. Compare the root before and after testing.
8. Confirm rollback capability.

This review does not require `go run` or a build at every client startup.

## 10. Safe startup categories and troubleshooting

| Category | Meaning | Safe response |
|---|---|---|
| `missing_root` | `MCP_ROOT` is absent | Configure an explicit absolute root. |
| `invalid_root` | Empty, whitespace, relative, or disallowed `.` | Correct the root; do not use implicit CWD. |
| `root_not_found` | The root does not exist | Check path and local availability. |
| `root_not_directory` | The root is a file | Choose an existing directory. |
| `root_not_allowed` | Permission or root policy rejects it | Check policy and root choice; do not bypass policy. |
| `invalid_profile` | Invalid `MCP_READ_ONLY` | Set a valid Boolean value; use `true` for Codex. |
| `invalid_development_option` | Invalid `MCP_ALLOW_CWD_ROOT` | Use exactly lowercase `true` or `false`. |
| `startup_failed` | Unexpected bootstrap failure | Check safe logs and environment, then roll back. |

Expected configuration/root failures use exit code 3; unexpected failures use
exit code 1. Stdout stays empty, and stderr contains neither raw OS errors nor
absolute root paths.

## 11. Backup and rollback

Before a later change:

1. Make a timestamped backup of the client configuration.
2. Record existing MCP entries and versions.
3. Keep unprotected secrets out of reports and backups.

Rollback:

1. Disable or remove the FlashGate entry using the client's supported method,
   or restore the verified backup.
2. Leave other MCP entries unchanged.
3. Restart the client completely.
4. Confirm FlashGate is absent from the active MCP list.
5. Confirm no `flashgate-mcp` process remains.
6. Check the root and repository for artifacts.
7. Remove the binary only with separate approval.

`SPR-044` ran neither `codex mcp add` nor `codex mcp remove` and changed no
real `config.toml` or authentication file.

## 12. CallToolResult and canary gate

The first Codex canary passed a JSON-RPC preflight but failed in the real
client with `Unexpected response type` because successful domain objects were
unwrapped. `SPR-045` corrected the server contract but did not reactivate the
canary.

Reactivate only after review, commit, PR, passing Windows/Ubuntu CI, merge,
post-merge gates, a new versioned binary, strict direct preflight, separate
user approval, a complete Codex restart, and a successful repeat of the
model-assisted end-to-end test. The existing canary remains disabled until then.
