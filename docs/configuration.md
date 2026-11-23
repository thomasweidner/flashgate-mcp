# Runtime configuration

FlashGate reads one UTF-8 TOML file at startup. Use the [complete template](flashgate-config.toml)
and set `files.work.path` to an existing absolute data directory that you
intend to expose. The template uses the default `read` profile.
In a Windows basic TOML string, use forward slashes or
escape each backslash. A TOML literal string such as
`'C:\FlashGateData\ReadOnlyRoot'` also preserves backslashes.

## Named roots

Configure one table per root and select the default explicitly.

```toml
[files]
default_id = "work"

[files.work]
path = 'C:\FlashGate\Work'

[files.archive]
path = 'D:\Archive'
```

Root IDs are case-sensitive and match `^[a-z][a-z0-9_]{0,63}$`. The keys
`default_id` and the global size and entry settings below are reserved,
as is the former `roots` namespace. The default
must name a configured ID exactly. Up to 256 roots fit within the configuration
contract, subject to the 64 KiB document limit. Every path must be an
existing absolute directory that passes PathGuard and native protection
checks. An invalid secondary root prevents startup.

The current MCP tools require an exact configured ID and a relative path for
each operand, even with one configured root. See the [tool reference](tools.md).
The configured default ID is a TOML requirement and clients still send the ID
explicitly. Named-root discovery belongs to BL-352.
The [migration guide](migration.md#flat-toml-and-root-capabilities) covers
upgrading an existing TOML deployment.

## Named policies

Use `[policy.<name>]` to define a reusable policy in the same TOML file.
Names match `^[a-z][a-z0-9_]{0,63}$` and are referenced exactly. A definition
grants nothing by itself. A resource can reference one policy with `policy` and
applies the fields supported by its resource type. Its explicit values override
the referenced fields individually. Lists replace the whole value. References
between policies are unsupported.

Filesystem roots support [link policy](#symlinks-and-reparse-points), [file-type restrictions](#file-types), the five [resource limits](#resource-limits) and
`access`, a list of unique full capability IDs that apply to filesystem
roots. Omit the field at both levels for `["files.read"]`. Specify
`[]` to grant no root rights, including when overriding a named policy. Unknown
references or fields, duplicate definitions or rights, and invalid types or
values fail startup, including in unused policies.

```toml
[policy.workspace]
access = ["files.read", "files.write"]

[files]
default_id = "documents"

[files.documents]
path = 'C:\Documents'

[files.projects]
path = 'C:\Projects'
policy = "workspace"

[files.archive]
path = 'D:\Archive'
policy = "workspace"
access = ["files.read"]
```

The filesystem access list accepts `files.read`, `files.write` and
`search.run`. The latter is reserved for approved search consumers and
enables no current tool. Global capabilities such as `processes.read`
are profile grants and are rejected in filesystem access lists.
Each operand uses the rights of its explicit root ID. Two IDs can point to
overlapping paths and grant different access to them. The
[tool reference](tools.md) lists each operation's required root rights.
Writing requires a root grant and the applicable profile grants and risk
ceiling. Existing write deployments can follow the
[root access migration](migration.md#root-access).

## Symlinks and reparse points

`follow_symlinks` is a strict boolean supported only by named policies and
root tables. An explicit root value replaces the inherited value, including
false replacing true. Omission at both levels means false. Resolve each
effective value once during startup; even unused policy definitions must be
valid. There is no global link setting and no implicit grant of access or
functional capabilities.

```toml
[policy.workspace]
access = ["files.read", "files.write"]
follow_symlinks = false

[files.projects]
path = 'C:\Projects'
policy = "workspace"

[files.archive]
path = 'D:\Archive'
policy = "workspace"
follow_symlinks = true
```

False denies link traversal, including linked parents. Listings omit denied
links; stat reports a denial rather than target metadata or absence. True
permits supported relative symlinks only when their entire resolution stays
inside that root. Absolute targets, escaping links, cycles, excessive chains,
Windows junctions and unknown/unsupported reparse objects remain denied.
Root-path ancestors must also have supported resolution. Explicit root aliases
may use supported relative links when enabled; they bind one effective root
and cannot change that root during operation.

Native Linux effects require `openat2` support and reject nested mount/volume
transitions, including same-device bind mounts. There is no weaker fallback.
Windows opens content through a bound parent and rejects a final reparse
object before content I/O or truncation. All eight operations retain identity,
configuration protection, access, extension and resource checks. Unproved
relative-link relocation is denied; a same-parent leaf-link rename remains
supported. Recursive scans never descend through directory links.

See [tool semantics](tools.md#link-and-native-effect-semantics), the
[security boundary](security.md#symlinks) and the explicit
[pre-1.0 cutover](migration.md#per-root-link-policy).

## Process working directories

Set `allow_process_cwd` only in a named policy or a root table. It accepts
the TOML booleans `true` and `false` and defaults to `false`. A root inherits
the named-policy value when its field is absent. An explicit root boolean
overrides it independently of link policy, exclusions, access and limits.

```toml
[policy.execution_workspace]
allow_process_cwd = true

[files.work]
path = "/srv/flashgate/work"
policy = "execution_workspace"

[files.archive]
path = "/srv/flashgate/archive"
policy = "execution_workspace"
allow_process_cwd = false
```

An internal working-directory operand names the exact root and a relative
directory. Use `.` for that root, or a path to an existing eligible
subdirectory. PathGuard, configuration protection, link rules and applicable
exclusions still apply. The setting grants directory selection only. The
[security contract](security.md#process-working-directory-and-delegated-effects)
defines the independent profile, resource and native-effect requirements.
The current tools have no process-launch route.

## File types

Set `allowed_extensions` and `allow_extensionless` only in a named policy or
a root table. They are additional restrictions, independent of root access,
profile grants, PathGuard, configuration protection and resource limits.
With both fields absent at both levels, existing configurations have no
additional extension restriction, including for extensionless files.

```toml
[policy.documents]
allowed_extensions = [".txt", ".md", ".tar.gz"]
allow_extensionless = false

[files.work]
path = "/srv/flashgate/work"
policy = "documents"

[files.other]
path = "/srv/flashgate/other"
policy = "documents"
allowed_extensions = "unrestricted"
allow_extensionless = true
```

Each absent root field independently inherits its policy field. A root list
replaces the entire inherited list, including `[]`. The exact scalar
`"unrestricted"` removes the suffix restriction; it does not erase an inherited
extensionless boolean. Resolve both fields before applying these defaults:

| Effective extension setting | Extensionless boolean absent at both levels | Effect of an explicit/inherited boolean |
|---|---|---|
| Absent or `"unrestricted"` | True | False denies extensionless names only |
| Any list, including `[]` | False | True also permits extensionless names |

An empty list denies every suffixed name. `[]` with true permits only
extensionless names. No wildcard or other scalar string is supported.
Unused policies are validated too; malformed/duplicate tokens, wrong types,
unknown fields or references and duplicate definitions reject startup before
exposure. The effective policy is immutable and startup does not rewrite TOML.

Tokens start with a dot and contain nonempty dot-separated ASCII segments
matching `[a-z0-9][a-z0-9_-]*`. Require lowercase tokens; filenames use ASCII-only
case folding. `.txt` permits `A.TXT`, not `a.txt.exe`. `.tar.gz` permits
`a.tar.gz`, not `a.gz`; `.gz` also permits `a.tar.gz`. `.env` is extensionless,
while `.env.txt` is suffixed. No MIME token, wildcard, separator, whitespace,
control character or Unicode lookalike is repaired into an allowed token.
With an effective restriction, portable basename checks reject trailing
dots/spaces, stream syntax and other ambiguous spellings rather than repairing
them into a match. With absent or `"unrestricted"` extensions and extensionless
names allowed, BL-106 adds no portable name restriction. Existing controls can
still address native Linux names such as `colon:token.txt`, `tail.txt.`,
`tail.txt ` and `back\slash.txt`. A colon in a Linux basename is not a Windows
alternate data stream. Native Windows device, ADS and trailing-dot/space
mappings remain denied independently in every mode. File effects always
require a provable regular-file type; a suffix never classifies contents or
grants access to a device or special object. Structural directory operations
do not require a filename suffix.

Both the supplied name and resolved native name are checked, including allowed
links and Windows short-name aliases. Under a restriction, moving a relative
link to another parent or moving a directory containing relative links is
denied when its projected target cannot be established. Same-parent link
renames and absolute links still require every other check. File effects,
overwrites and bounded recursive preflights use the same policy below MCP;
see [tool semantics](tools.md#file-type-enforcement).

## Path exclusions

```toml
[files]
default_id = "work"

[policy.workspace]
excluded = ["A/X", "logs/my*.log", "reports/report-202?.pdf"]

[files.work]
path = "/srv/flashgate/work"
policy = "workspace"

[files.other]
path = "/srv/flashgate/other"
policy = "workspace"
excluded = []
```

Set `excluded` in a named policy or a root table. Omit it on a root to
inherit the policy's list, with an empty list when no policy value applies.
An explicit root list replaces the whole inherited list. In the example,
`files.other` clears its inherited list. Any effective exclusion from
another applicable root still applies to overlapping paths.

Patterns are root-relative and match complete paths or their ancestors.
`A/X` excludes `A/X` and paths such as `A/X/y.z` and `A/X/Z/file.txt`.
`A/X2`, `A/XY` and `B/file.txt` remain eligible under this rule.
Use `/` as the separator on both platforms. Within a component, `*`
matches zero or more Unicode characters and `?` matches one. Other
accepted characters are literal. The grammar rejects recursive `**`,
character classes, brace expansion, rooted or traversing patterns and
ambiguous components such as trailing dots or whitespace. The same list applies to files,
directories, symlinks and paths that have yet to be created. Matching
uses the native filesystem's verified case semantics.

Each list accepts up to 64 patterns. A pattern accepts up to 1,024 UTF-8
bytes and 64 components, with up to 255 bytes per component. The 64 KiB
configuration ceiling still applies. An active policy bounds an operand
route to 4,096 bytes, 128 components and 65,536 component-matching steps
across all its applicable rules and spellings.
The server denies unproved mappings or work beyond those limits.

Use local NTFS paths on Windows. Case-sensitive lookup uses exact spelling.
Case-insensitive lookup supports provable ASCII folding and exact Unicode
characters. A comparison that would require an unproved Unicode fold
denies access. On Linux use the native ext filesystem of the root mount,
without casefolding, bind mounts or additional mount transitions in data
routes. The server rejects configuration if it cannot bind the configured
roots when any effective exclusion list is active. A configuration with
only empty effective lists retains the existing filesystem behavior.

See [ADR-005](adr/filesystem-abstraction.md#bl-375-path-exclusions)
for link and overlap semantics and the
[security boundary](security.md#path-exclusion-boundary) for
interaction with active configuration protection.

## Resource limits

`files.max_file_size`, `max_write_size`, `max_scan_size`, `max_result_size`
and `max_temp_size` are global hard ceilings. A named policy can supply any of
these fields. An explicit `files.<id>` field replaces that policy field;
an absent field inherits the policy or global value. Every effective value
must stay within its global ceiling. Invalid or expanding unused policy
definitions also prevent startup. These immutable values grant no access or
capabilities. Two root IDs may have different caps even for the same directory.

| Dimension | Current measurement |
|---|---|
| File | Content bytes read, written or copied, per file. Metadata-only stat and rename do not read file contents. |
| Write | Complete new content for write and file copy, using the same ceiling. |
| Scan | Content bytes read by read/copy; raw name bytes visited by list, recursive delete and directory-move inspection, including filtered entries. Listing/deletion entry ceilings remain independent. |
| Result | Complete serialized MCP success result, including JSON escaping, content blocks, structured metadata and revision-owned fields. The protocol response ceiling additionally counts JSON-RPC framing, request ID and newline. Mutation receipts are admitted before side effects. |
| Temp | Aggregate live temporary-data reservations per exact root ID. Current filesystem operations create no scratch files; future consumers use `Registry.ReserveTemp` before allocating and release on every exit, also enforcing tighter operation/request budgets. This is distinct from heap and transport-buffer bounds. |

Size settings accept positive TOML integers as bytes or English strings such as
`"1 234"`, `"1,234"`, `"8.3 GB"`, `"8.3 GiB"` and `"10MiB"`. Strings use
a decimal point and either correctly grouped commas or single spaces for
thousands. There is no locale inference. Units are case sensitive: `B` is bytes,
`b` is bits; SI prefixes are `k`, `M`, `G`, `T`, `P`, `E`, and IEC prefixes are
`Ki`, `Mi`, `Gi`, `Ti`, `Pi`, `Ei`. Units are optional for byte strings. One
optional space separates the number from its unit. `"8,3 GB"`, `"8.388.608"`,
mixed grouping, unknown units, zero, negatives, floats, overflow and fractional
values below one byte are invalid. Exact rational arithmetic floors fractional
bytes for hard ceilings. Strings are limited to 128 bytes; the normalized
configuration ceiling must be smaller than the signed 64-bit maximum.

`internal/resource.ParseSize` and `FormatSize` are the shared pure helpers.
The formatter chooses the largest IEC byte unit that divides the value exactly,
otherwise bytes. Thus all spellings of 10485760 bytes become `10 MiB`, and
1536 bytes becomes `1536 B`. Its output roundtrips exactly. BL-373 must reuse
this formatter. Startup normalizes in memory and never rewrites the file.

Client `read_file.maxBytes` can only tighten the global, root, file and scan
bounds. A result that cannot fit returns the existing safe `limit_exceeded`
error; mandatory error envelopes are outside the success-result budget and
remain subject to the existing protocol error contract. No partial success
payload is emitted. Future batch, archive and job owners must additionally
declare aggregate effects and hard backend enforcement before activation.

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

[profiles.dev]
grants = ["files.read"]

[profiles.changes]
grants = ["files.write"]
```

Profile IDs match `^[a-z][a-z0-9_]{0,63}$`. The built-ins `read` and `write`,
the options `active`, `include`, `max_risk`, and the former `definitions`
namespace are reserved.
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

TOML booleans use `true` or `false`. Size fields accept positive byte integers
or the exact strings above. Entry/record counts remain integers and timeouts
remain milliseconds.

| Key | Type and runtime default | Effect |
|---|---|---|
| `runtime.mode` | String, host-selected (`"stdio"` in the current executable) | Must match the bound runtime role. |
| `files.default_id` | Required string | Must name an existing root ID. Tool calls provide their own explicit IDs. |
| `files.<id>.path` | Required string per root | Existing absolute data directory, subject to PathGuard. |
| `files.<id>.policy` | Optional policy name | Reference one named policy. |
| `files.<id>.access` | Optional capability ID array | Replace the referenced access list. Missing access at both levels defaults to `["files.read"]`. |
| `policy.<name>.access` | Optional rights array | Supply shared filesystem rights for roots that reference the policy. |
| `policy.<name>.allowed_extensions`, `files.<id>.allowed_extensions` | Optional extension array or exact `"unrestricted"` | Independent per-field inheritance; absent is unrestricted, `[]` denies suffixed names. |
| `policy.<name>.allow_extensionless`, `files.<id>.allow_extensionless` | Optional boolean | Inherit independently; default true with absent/unrestricted extensions, false with a list. |
| `profiles.active` | String, `"read"` | Select a built-in or validated custom base profile. |
| `profiles.include` | Array of strings, empty | Explicit composition with additional distinct grant sets. Requires explicit `active`. |
| `profiles.max_risk` | String, `"mutation"` | Operation risk ceiling. Accepts `"read"`, `"mutation"`, or explicit `"high"`. |
| `profiles.<id>.grants` | Required array of capability IDs per definition | Declare the custom profile's exact grants. |
| `files.max_file_size` | Size, 10485760 | Maximum file content for read, write and copy. |
| `files.max_write_size` | Size, 10485760 | Maximum complete write or copied content. |
| `files.max_scan_size` | Size, 10485760 | Maximum inspected content/name bytes per operation. |
| `files.max_result_size` | Size, 16777216 | Maximum serialized MCP success result. |
| `files.max_temp_size` | Size, 10485760 | Maximum aggregate live temporary-data reservations per exact root ID across concurrent operations. |
| `policy.<name>.max_*_size`, `files.<id>.max_*_size` | Optional size for the five resource fields above | Inherit and replace per field, within global hard ceilings. |
| `files.max_list_entries` | Integer, 1000 | Maximum scanned directory entries, including filtered entries. |
| `files.max_delete_entries` | Integer, 1000 | Maximum entries for recursive deletion. |
| `security.allow_hidden_files` | Boolean, `false` | Allows dot-prefixed data paths subject to other checks. |
| `security.allow_unc_paths` | Boolean, `false` | Allows a UNC data root subject to other checks. |
| `policy.<name>.follow_symlinks`, `files.<id>.follow_symlinks` | Boolean, `false` | Root override replaces the named-policy value; supported relative in-root links only. |
| `policy.<name>.allow_process_cwd`, `files.<id>.allow_process_cwd` | Boolean, `false` | Independently inherited root permission to select an eligible process working directory. |
| `protocol.max_message_size` | Size, 16777216 | Maximum inbound JSON-RPC message, including newline. |
| `protocol.max_argument_size` | Size, 12582912 | Maximum tool call params or arguments. |
| `protocol.max_response_size` | Size, 16777216 | Maximum serialized JSON-RPC response, including newline. |
| `logging.level` | String, `"off"` | Select `"debug"` for optional diagnostics. |
| `logging.sink` | String, `"stderr"` | The current supported sink. Stdout carries MCP only. |
| `logging.format` | String, `"text"` | Select `"json"` for one JSON object per diagnostic line. |
| `logging.max_record_size` | Size, 4096 | 128 to 65536 bytes per emitted record, including its newline. |
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
| `max_frame_size` | Size, 16777216 | Frame limit, 1 to 67108864 bytes. |
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
