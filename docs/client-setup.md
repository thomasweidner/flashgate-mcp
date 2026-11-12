# Client setup

This guide connects an MCP client to the implemented FlashGate STDIO server.
It uses an explicitly read-only root for initial acceptance. Client-specific
configuration syntax is not part of the FlashGate protocol contract; use the
client's documented launch configuration rather than copying another client's
configuration file wholesale.

## Prerequisites

Use a verified TOML-enabled release binary, or build from a known source commit using the
[controlled build procedure](build-metadata.md). Check binary identity,
`--version`, `--help`, and SHA-256 before configuring a client. Do not run an
automatic build or `go run` at each MCP startup. Keep the binary outside the
data root.

Choose a small, existing, absolute directory that the FlashGate process can
access. Do not begin with an entire drive, home directory, build directory, or
synchronized storage tree. Cloud-backed paths must be locally available and
satisfy the current hidden, UNC, symlink, junction, and reparse policy. Cloud
placeholder support is separate planned work, not implied by client setup.

Back up the client's existing configuration without exposing credentials in
reports. Record the intended change and rollback, leave unrelated MCP entries
unchanged, and apply the configuration only with the operator's authorization.

## STDIO launch contract

| Setting | Windows example | Linux example |
|---|---|---|
| Executable | `C:\Program Files\FlashGate\flashgate-mcp.exe` | `/opt/flashgate/current/flashgate-mcp` |
| Arguments | Empty argument array | Empty argument array |
| Root | `C:\FlashGateData\ReadOnlyRoot` | `/home/example/flashgate-readonly-root` |
| Transport | STDIO | STDIO |

The paths are examples, not required installation locations. The Linux binary
needs execute permission. Pass paths with spaces as single configuration
values. Do not mix Windows paths with native Linux or WSL paths. WSL is not a
FlashGate prerequisite.

Copy the [template](flashgate-config.toml) to the
[trusted user default](configuration.md#source-and-trust) and set
`filesystem.root` to the data directory. For example:

```toml
[filesystem]
root = 'C:\FlashGateData\ReadOnlyRoot'
read_only = true
```

For Linux, use a Linux absolute data path. To use an alternate configuration
file, set the client's argument array to `["--config", "<absolute-path>"]`.
Verify client-specific startup and call timeouts for the client version.
See [runtime configuration](configuration.md) for all supported settings.

## Current read-only behavior

Set `filesystem.read_only = true` explicitly. The current default without that setting
exposes eight tools; the planned safe-by-default profile system is not yet the
implemented behavior.

Read-only discovery returns exactly these three tools, in this order:

1. `list_directory`
2. `read_file`
3. `get_path_info`

The current mutation names are `write_file`, `create_directory`, `delete_path`,
`copy_path`, and `move_path`. In read-only mode they are unavailable. Removed
names `list_files`, `stat_path`, `exists_path`, `mkdir`, and `rename_path` have
no aliases. The current error contract for calls to any of these unavailable
names is the same generic Invalid params result. Do not infer authorization
from tool annotations alone.

## Acceptance checks

Run the applicable [smoke tests](testing.md) against the verified binary before
activating the client entry. Use a strict MCP result decoder: syntactically
valid JSON and domain fields alone are not sufficient.

After activation, verify initialization, `serverInfo.name=flashgate`, the
three-tool discovery list, a directory listing, ordinary file reads, names with
spaces or Unicode characters, and existing and missing path metadata. Verify
that out-of-root traversal is denied, and use the provided negative tests to
check every unavailable mutation and removed tool name. Compare root contents
before and after testing.

Read UTF-8 file text from MCP `content[0].text` and its byte size from
`structuredContent.size`. Invalid UTF-8 returns a tool error. Read directory entries from
`structuredContent.entries`. A missing `get_path_info` path returns a
successful `exists:false` metadata result. See the
[protocol reference](protocol.md) and [migration guide](migration.md).

Stdout must contain only JSON-RPC. Stderr must use safe categories without raw
OS errors, secrets, or absolute root paths. Confirm the client handles the
actual server results rather than relying only on a direct command-line test.

## Startup troubleshooting

| Category | Meaning | Action |
|---|---|---|
| `missing_config` | Selected file is absent | Create the file at the selected trusted path. |
| `untrusted_config` | Path, ownership, access, or file type fails source checks | Use a trusted absolute path under the executing account. |
| `invalid_config` | Syntax, fields, types, or limits fail validation | Compare the file against the template and supported settings. |
| `unsupported_runtime` | The configured mode cannot run under this host | Use `runtime.mode = "stdio"` for this client. |
| `missing_root` | `filesystem.root` is absent | Set an explicit absolute root. |
| `invalid_root` | Empty, whitespace, relative, or disallowed `.` | Correct the root; do not rely on implicit CWD. |
| `root_not_found` | Root does not exist | Check the path and local availability. |
| `root_not_directory` | Root is a file | Select an existing directory. |
| `root_not_allowed` | Root policy denies access or bootstrap cannot prove separation from the TOML file | Select a separate local data root and keep the configuration outside its resolved path. |
| `startup_failed` | Unexpected bootstrap failure | Inspect safe diagnostics and roll back the configuration. |

Expected root/configuration failures use exit code 3; unexpected failures use
exit code 1. Failed startup leaves stdout empty.

## Rollback

Disable the affected FlashGate entry using the client's supported method, or
restore its verified configuration backup. Leave other MCP entries unchanged.
Restart the client as required and confirm that it no longer exposes the
removed entry. Confirm that the transport session launched by that entry has
ended; do not terminate unrelated FlashGate sessions by executable name.
Inspect the data root for unexpected changes. Removing an installed binary or
changing its permissions is a separate operator action, not an automatic
side effect of configuration rollback.

For a rollback to an environment-based binary, restore its paired client entry
as described in [migration](migration.md#root-configuration). A replacement
binary requires a fresh identity and acceptance check. Setup examples do not attest that any particular user's client has
been configured or validated.
