# Runtime configuration

FlashGate reads one UTF-8 TOML file at startup. Use the [complete template](flashgate-config.toml)
and set `filesystem.root` to an existing absolute data directory that you
intend to expose. For a first connection, retain the template's
`read_only = true`. In a Windows basic TOML string, use forward slashes or
escape each backslash. A TOML literal string such as
`'C:\FlashGateData\ReadOnlyRoot'` also preserves backslashes.

## Source and trust

| Executing host | Default |
|---|---|
| Direct STDIO or a future user-scoped host | `~/.flashgate/flashgate_config.toml` under the executing user's OS profile |
| Future Windows system service | `C:\ProgramData\FlashGate\flashgate_config.toml` |
| Future Linux system service | `/etc/flashgate-mcp/flashgate_config.toml` |

Run `flashgate-mcp` for the user default, or pass `--config <absolute-path>`
to select one alternate file. Pass the flag and path as separate arguments.
An override replaces the default even when the selected file is absent.
The executable directory, project directory, working directory, and
environment supply no alternative sources. The operating system account
mapping supplies the user home. The host binds its role before the file opens.

Store the configuration in a locally owned, protected directory outside the
MCP data root. The file must be a regular file owned by the executing user
for user roles, with no second hard link and no write access for other
untrusted identities. Each parent must prevent untrusted replacement. Linux
uses descriptor-relative lookup on supported local filesystems and checks
ownership and group/other write permissions. Windows checks a fixed local
drive, handles, owner, and DACLs. Config paths through symbolic links,
junctions, reparse points, or remote filesystems are denied. System roles use
root ownership on Linux and trusted system or administrator ownership on
Windows. The filesystem tool policy below applies to the data root, independent
of the configuration file's trust rules.

Startup also requires the authoritative TOML file to be outside every
effective MCP data root. This applies to the role default and `--config`,
including read-only profiles, and regardless of hidden-file policy.
FlashGate checks resolved roots, local directory identities, and relevant
Linux mount mappings for aliases. It reports `root_not_allowed` if a root
can expose the file or the separation cannot be established. Before enabling
tools, FlashGate securely reopens the source and confirms that it still
identifies the trusted file loaded at startup and still has exactly one
hard link. A source that fails this recheck also reports `root_not_allowed`.
Windows network roots and Linux mounts that cannot be compared to the trusted
source are examples of unprovable separation. Choose a separate local
data directory and keep its mount namespace stable while the server runs.

The file is limited to 64 KiB. Use the exact lowercase key names in the
template. Unknown keys, duplicate keys, wrong types,
unsupported settings, and invalid limits fail startup. The running process
uses a validated snapshot and reads the file again on restart. Startup
performs no configuration writes. Failed startup leaves stdout empty and
reports a safe category on stderr. See [startup troubleshooting](client-setup.md#startup-troubleshooting).

## Supported settings

TOML booleans use `true` or `false`. Limits are positive integers, in bytes
unless the field names or table indicate another unit.

| Key | Type and runtime default | Effect |
|---|---|---|
| `runtime.mode` | String, host-selected (`"stdio"` in the current executable) | Must match the bound runtime role. |
| `filesystem.root` | Required string | Existing absolute data directory, subject to PathGuard. |
| `filesystem.read_only` | Boolean, `false` | When true, register only the three current read tools. The template explicitly sets true. |
| `filesystem.allow_cwd_root` | Boolean, `false` | Development STDIO only. Allows an explicitly configured `root = "."` and emits one stderr warning. |
| `filesystem.max_file_size` | Integer, 10485760 | Maximum file read. |
| `filesystem.max_write_bytes` | Integer, 10485760 | Maximum write content. |
| `filesystem.max_copy_bytes` | Integer, 10485760 | Maximum copied file size. |
| `filesystem.max_list_entries` | Integer, 1000 | Maximum scanned directory entries, including filtered entries. |
| `filesystem.max_delete_entries` | Integer, 1000 | Maximum entries for recursive deletion. |
| `security.allow_hidden_files` | Boolean, `false` | Allows dot-prefixed data paths subject to other checks. |
| `security.allow_unc_paths` | Boolean, `false` | Allows a UNC data root subject to other checks. |
| `security.follow_symlinks` | Boolean, `false` | Allows classic symlinks inside the effective data root. |
| `protocol.max_message_bytes` | Integer, 16777216 | Maximum inbound JSON-RPC message. |
| `protocol.max_argument_bytes` | Integer, 12582912 | Maximum tool call params or arguments. |
| `protocol.max_response_bytes` | Integer, 16777216 | Maximum serialized JSON-RPC response. |
| `logging.level` | String, `"off"` | Select `"debug"` for optional diagnostics. |
| `logging.sink` | String, `"stderr"` | The current supported sink. Stdout carries MCP only. |
| `logging.format` | String, `"text"` | Select `"json"` for one JSON object per diagnostic line. |
| `logging.max_record_bytes` | Integer, 4096 | 128 to 65536 bytes per emitted record, including its newline. |
| `logging.max_records` | Integer, 10000 | 1 to 1000000 diagnostic records per process. |

Optional diagnostics are redacted before emission and omit messages that
exceed the output bounds. Safe startup failures and the development-root
warning use their own fixed stderr messages.

## Managed runtime contract

The central model reserves the role values `proxy`, `auto`, `service`, and
`user-host` for future hosts. The current executable starts direct STDIO.
Keep the `endpoints` defaults in a direct STDIO configuration. Future hosts
bind their role and consume these typed values from the same configuration:

| Endpoint key | Type and default | Validated scope |
|---|---|---|
| `windows_pipe_name` | String, empty | Local pipe name of up to 128 ASCII letters, digits, periods, underscores, or hyphens. |
| `linux_socket_path` | String, empty | Clean absolute local socket path of up to 107 bytes. |
| `connect_timeout_ms` | Integer, 5000 | Connection timeout, 1 to 3600000 milliseconds. |
| `request_timeout_ms` | Integer, 30000 | Request timeout, 1 to 3600000 milliseconds. |
| `shutdown_timeout_ms` | Integer, 5000 | Shutdown timeout, 1 to 3600000 milliseconds. |
| `max_frame_bytes` | Integer, 16777216 | Frame limit, 1 to 67108864 bytes. |
| `ipc_version` | Integer, 1 | The only supported contract version. |
| `fallback` | String, `"disabled"` | `"direct-if-absent"` is accepted for a future auto host only. Configuration failures are terminal. |

Endpoint discovery belongs to BL-229. Transport, service, and HTTP owners
implement activation separately. Future service logging targets such as
Windows Event Log and journald require native integration. BL-166 owns
structured audit and its lifecycle. The other planned configuration domains
are defined in the [runtime modes ADR](adr/runtime-modes.md#configuration-contract).

See [migration](migration.md#root-configuration) for conversion and rollback
from environment-based releases, and [client setup](client-setup.md) for
launch and acceptance.
