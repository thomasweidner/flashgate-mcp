# Runtime configuration

FlashGate reads one UTF-8 TOML file at startup. Use the [complete template](flashgate-config.toml)
and set `files.roots.work.path` to an existing absolute data directory that you
intend to expose. The template uses the default `read` profile.
In a Windows basic TOML string, use forward slashes or
escape each backslash. A TOML literal string such as
`'C:\FlashGateData\ReadOnlyRoot'` also preserves backslashes.

## Named roots

Configure one table per root and select the default explicitly.

```toml
[files]
default_id = "work"

[files.roots.work]
path = 'C:\FlashGate\Work'

[files.roots.archive]
path = 'D:\Archive'
```

Root IDs are case-sensitive and match `^[a-z][a-z0-9_]{0,63}$`. The default
must name a configured ID exactly. Up to 256 roots fit within the configuration
contract, subject to the 64 KiB document limit. Every path must be an
existing absolute directory that passes PathGuard and native protection
checks. An invalid secondary root prevents startup.

The current MCP tools require an exact configured ID and a relative path for
each operand, even with one configured root. See the [tool reference](tools.md).
The configured default ID is a TOML requirement and clients still send the ID
explicitly. Named-root discovery belongs to BL-352.
The [migration guide](migration.md#root-configuration) covers conversion
from the former table.

## Named policies

Use `[policy.<name>]` to define a reusable policy in the same TOML file.
Names match `^[a-z][a-z0-9_]{0,63}$` and are referenced exactly. A definition
grants nothing by itself. A resource can reference one policy with `policy` and
applies the fields supported by its resource type. Its explicit values override
the referenced fields individually. Lists replace the whole value. References
between policies are unsupported.

Filesystem roots currently support `access`, a list of unique `"read"` and
`"write"` rights. Omit the field at both levels for read-only access. Specify
`[]` to deny both rights, including when overriding a named policy. Unknown
references or fields, duplicate definitions or rights, and invalid types or
values fail startup, including in unused policies.

```toml
[policy.workspace]
access = ["read", "write"]

[files]
default_id = "documents"

[files.roots.documents]
path = 'C:\Documents'

[files.roots.projects]
path = 'C:\Projects'
policy = "workspace"

[files.roots.archive]
path = 'D:\Archive'
policy = "workspace"
access = ["read"]
```

Each operand uses the rights of its explicit ID. Two IDs can point to
overlapping paths and grant different access to them. The
[tool reference](tools.md) lists each operation's required root rights.
Writing requires a root grant and the applicable profile grants and risk
ceiling. Existing write deployments can follow the
[root access migration](migration.md#root-access).

## Profiles

`read` grants `files.read` and exposes the three current read tools. Select
`write` explicitly to grant `files.read` and `files.write` and expose all
eight filesystem tools. Root rights govern each call independently of the
profile's tool exposure. Path and configuration-protection checks also apply.

```toml
[profiles]
active = "dev"
include = ["changes"]
max_risk = "mutation"

[profiles.definitions.dev]
grants = ["files.read"]

[profiles.definitions.changes]
grants = ["files.write"]
```

Profile IDs match `^[a-z][a-z0-9_]{0,63}$`. `read` and `write` are reserved.
Each definition requires an explicit array of exact, unique [capability IDs](adr/capability-profiles.md#functional-capability-contract).
An empty array contributes zero rights. Every definition is validated,
including inactive definitions. `include` selects distinct additional
profiles only when `active` is explicit. The union is independent of order.
Definitions contain grants only and cannot include other profiles.

`max_risk = "read"` restricts an otherwise writable configuration to read
operations. The default ceiling is `mutation`. An operation derives its risk
from reviewed capability definitions and its own minimum risk. The TOML
ceiling adds no grants and cannot change an operation's risk classification.
An explicit `max_risk = "high"` is valid. High-risk operations still require
their exact grants, a measured executable view, and any separate security
policy. The current STDIO host exposes only the same measured filesystem
catalogs with this ceiling.

The current STDIO host admits the measured `files.read` and
`files.read` plus `files.write` combinations, including equivalent custom
profiles and explicit compositions. Selecting an unmeasured combination
fails startup with `invalid_profile`. Known capabilities for future tools may
appear in valid inactive definitions. The host supports one or more named
roots under either measured catalog.

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

Provision the configuration with OS permissions appropriate to the executing
account. FlashGate requires a readable regular file that it can open
consistently. Linux opens components relative to verified directories.
Windows opens the source with short-lived handles and restricted sharing.
Source paths through symbolic links, junctions or other redirects fail
source validation.

Before enabling tools, FlashGate rechecks the source identity and verifies
that it can bind one protection snapshot to all resolved data roots.
Overlapping roots are allowed when native identity and mount checks support
the binding.
A failed binding or changed source identity reports `root_not_allowed`.

The file is limited to 64 KiB. Use the exact lowercase key names in the
template. Unknown keys, duplicate keys, wrong types,
unsupported settings, and invalid limits fail startup. The running process
uses a validated snapshot and reads the file again on restart. Startup
performs no configuration writes. Failed startup leaves stdout empty and
reports a safe category on stderr. See [startup troubleshooting](client-setup.md#startup-troubleshooting).

## Configuration protection

The operating system and its administrators control permissions and
provisioning. FlashGate protects the selected resource from ordinary tool
reads, disclosure and mutation.
Contents remain protected under ordinary root read grants, including when
the selected file is used as a copy source. Ordinary listing and stat can
still return allowed content-independent metadata. BL-373 owns future
server-authorized redacted configuration views. The snapshot stays effective
after independent file changes until a validated restart or a later
authorized configuration mechanism. See the [security contract](security.md#configuration-trust-and-protection).

## Supported settings

TOML booleans use `true` or `false`. Limits are positive integers, in bytes
unless the field names or table indicate another unit.

| Key | Type and runtime default | Effect |
|---|---|---|
| `runtime.mode` | String, host-selected (`"stdio"` in the current executable) | Must match the bound runtime role. |
| `files.default_id` | Required string | Must name an existing root ID. Tool calls provide their own explicit IDs. |
| `files.roots.<id>.path` | Required string per root | Existing absolute data directory, subject to PathGuard. |
| `files.roots.<id>.policy` | Optional policy name | Reference one named policy. |
| `files.roots.<id>.access` | Optional rights array | Replace the referenced access list. Missing access at both levels defaults to `["read"]`. |
| `policy.<name>.access` | Optional rights array | Supply shared filesystem rights for roots that reference the policy. |
| `profiles.active` | String, `"read"` | Select a built-in or validated custom base profile. |
| `profiles.include` | Array of strings, empty | Explicit composition with additional distinct grant sets. Requires explicit `active`. |
| `profiles.max_risk` | String, `"mutation"` | Operation risk ceiling. Accepts `"read"`, `"mutation"`, or explicit `"high"`. |
| `profiles.definitions.<id>.grants` | Required array of capability IDs per definition | Declare the custom profile's exact grants. |
| `files.max_file_size` | Integer, 10485760 | Maximum file read. |
| `files.max_write_bytes` | Integer, 10485760 | Maximum write content. |
| `files.max_copy_bytes` | Integer, 10485760 | Maximum copied file size. |
| `files.max_list_entries` | Integer, 1000 | Maximum scanned directory entries, including filtered entries. |
| `files.max_delete_entries` | Integer, 1000 | Maximum entries for recursive deletion. |
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
exceed the output bounds. Startup failures use fixed stderr messages.

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
