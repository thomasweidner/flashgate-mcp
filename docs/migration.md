# Migration

This guide collects compatibility changes relevant to clients using early
FlashGate builds. Follow the current [tool reference](tools.md),
[protocol reference](protocol.md), and [changelog](../CHANGELOG.md) when
updating an integration. Internal task renumbering is not a product migration.

## Per-root path exclusions

Existing configurations can omit `excluded`. To restrict paths, add it
under a named policy or a root in the active TOML file and restart the
server. A root inherits only when its own field is absent. Replace the
whole list to change a root's rules, or use `[]` to clear its inherited
list. Other overlapping roots retain their own exclusions.

Check the [supported native paths and pattern grammar](configuration.md#path-exclusions)
before enabling the setting. Verify one denied path and one allowed sibling
under each relevant root ID, including any configured alias. Clients can
use their existing tools and arguments. Excluded paths produce
`access_denied`, and parent listings omit the excluded names.

An older server rejects the new TOML field. To roll back to an older version,
restore the prior configuration and use that version's existing access and
root boundaries for any paths that must remain private.

## Per-root link policy

Remove `security.follow_symlinks` before starting this build. That global
field is rejected even when false. Configure `follow_symlinks` in each root
that needs traversal or in its referenced named policy; an explicit root
boolean replaces the inherited boolean. Missing values default to false.
There is no alias, automatic conversion or configuration rewrite.

After upgrading, only supported relative targets inside the selected root
may be followed. Replace absolute symlinks deliberately if traversal is
required; Windows junctions, unsupported reparse objects, nested Linux mounts
and unproved relative-link relocation remain denied. Link permission does not
override access, file types, limits or selected-configuration protection.
Public tool arguments are unchanged. Validate allowed reads and writes and
denials under every configured root ID before restoring normal clients.

The bound root itself cannot be removed through `delete_path` with `path:"."`.
That unsupported namespace operation is denied before changing any children;
select an authorized child path instead.

Back up the selected configuration and retain its paired old binary before
the cutover. Rollback restores that pair deliberately; the old loader rejects
the new per-root/named-policy field. Builds require Go 1.27.4 or newer, and
native Linux runtime filesystem enforcement requires `openat2` support.

## File-type restrictions

Existing configurations without `allowed_extensions` or `allow_extensionless`
retain unrestricted extension matching. There is no mandatory configuration
rewrite or automatic migration. Opt into a restriction at a named policy or
root using the [file-type fields](configuration.md#file-types). Older binaries
reject these new fields; remove them only as an explicit rollback decision,
since doing so removes their restriction.

Absent or unrestricted extensions with extensionless names allowed preserve
previously addressable native Linux basenames, including literal colons,
backslashes and trailing dots/spaces. Portable basename rules apply when an
effective file-type restriction is present. Existing PathGuard and configuration
protection still apply; unrestricted matching never permits ambiguous Windows
device/ADS mappings or unsupported native objects.

Do not use `[]` to mean unrestricted: it denies all suffixed names. A separate
true extensionless boolean permits only extensionless names with that empty
list. To remove an inherited restriction for all names, set the extension
field to `"unrestricted"` and the extensionless field to true. An inherited
boolean is not implicitly reset by changing the extension field.

All current tools preserve their names, schemas and result envelopes. A denied
file operand returns `access_denied`, including metadata queries; it is not
reported as missing. Listings omit denied file entries but charge their scan
cost. Directory operations stay structural. Unsafe native mappings, Windows
streams, special objects and unproved restricted link projections are denied;
do not rely on suffix normalization or an allowed alias to reach a denied target. File-type permission
never overrides selected-configuration protection or other security checks.

## Resource size settings

Replace `files.max_write_bytes` with `files.max_write_size` and remove
`files.max_copy_bytes`: the single write size now governs complete writes and
file copies. Existing `files.max_file_size` also bounds written/copied content.
Choose one write ceiling deliberately if the former write/copy values differ.
Replace `protocol.max_message_bytes`, `max_argument_bytes` and
`max_response_bytes` with their `_size` spellings. Likewise replace
`endpoints.max_frame_bytes` and `logging.max_record_bytes` with `_size`.
Removed keys are rejected; there are no aliases or automatic file rewrites.

All size settings accept positive byte integers and exact English SI/IEC
strings. Add optional `max_scan_size`, `max_result_size` and `max_temp_size`
globally, in named policies or at each root. Root/policy values must stay
within global ceilings. See [resource semantics](configuration.md#resource-limits)
for exact units, conservative fractional rounding, canonical formatting,
scan accounting and serialized result bounds. Entry counts, timeouts and
the existing `read_file.maxBytes` request argument retain their units/names.

## Product identity

The current repository is `thomasweidner/flashgate-mcp`, the Go module is
`github.com/thomasweidner/flashgate-mcp`, the executable is `flashgate-mcp`
(`flashgate-mcp.exe` on Windows), and MCP `serverInfo.name` is `flashgate`.

Older integrations may use `fileserver-mcp` or the former repository owner
`blacksheepkhan`. Update launch paths, module imports, and identity checks to
the current identifiers. A client does not need to rename the internal
`cmd/server` source directory. The technical rename itself did not rename
`MCP_*` environment variables.

For an existing source clone, inspect its remotes before applying a separately
authorized local remote correction:

```text
git remote -v
git remote set-url origin https://github.com/thomasweidner/flashgate-mcp.git
```

New clones use the current repository directly. Do not repeat historical
repository-rename operations, rewrite Git history, or treat an old redirect as
the canonical installation path.

## Root configuration

The named-root schema replaces the former `[filesystem]` table. Before
upgrading a TOML deployment, back up the selected file and retain the
verified old binary. Convert the configuration manually while the
client session is stopped:

1. Choose a logical ID such as `work`. Set `default_id = "work"` in `[files]`.
2. Move the old root value to `path` under `[files.work]`. Replace `.` with
   the absolute directory intended for the client.
3. Move the five existing filesystem limits to `[files]`. Remove the old
   table, `allow_cwd_root`, and `read_only`. See the profile cutover below.
4. Restart with the same selected file and run the
   [acceptance checks](client-setup.md#acceptance-checks).

The loader rejects the former table and has no automatic conversion.
Rollback restores the previous binary and its backed-up TOML together.
The current MCP tool inputs require explicit root IDs and relative paths.

## Flat TOML and root capabilities

Before upgrading, stop the client and retain the previous binary and a backup
of the selected TOML. Edit the selected file manually:

1. Move each `[files.roots.<id>]` table to `[files.<id>]`. Keep
   `files.default_id` and global file settings in `[files]`.
2. Move each `[profiles.definitions.<id>]` table to `[profiles.<id>]`.
   Keep `active`, `include`, and `max_risk` in `[profiles]`.
3. In each root and named policy `access` list, change `"read"` to
   `"files.read"` and `"write"` to `"files.write"`. Keep omitted and empty
   lists as they are to preserve inheritance and explicit denial.
4. Restart and run the [acceptance checks](client-setup.md#acceptance-checks)
   with the intended profile and root IDs.

The loader rejects the former table layouts and short access values. It
does not rewrite the configuration. For rollback restore the paired binary
and backup together.

## Root access

After upgrading to a build with per-root access, roots without explicit
access still allow reads. A writable profile by itself leaves mutations
denied. Stop the client, back up the selected TOML, and give only the roots
that need writes an explicit grant. For example, add
`access = ["files.read", "files.write"]` to `[files.work]` or reference a
[named policy](configuration.md#named-policies) that supplies those rights.
Keep the profile grants required by those operations. Restart and check a
read and an authorized mutation on test data under the intended root ID.
Check that other IDs still deny writes. The MCP arguments remain the same.

For rollback, stop the client and restore the paired prior binary and
backed-up TOML. The prior strict loader rejects the new policy keys.

## Capability profile cutover

Remove `files.read_only` before starting a build with capability profiles.
Omit `profiles.active` for default read access. Set `active = "write"` in
`[profiles]` to restore access to the writable catalog. Configure [root access](#root-access)
as well to permit mutations. Use the current
[configuration reference](configuration.md#profiles) for custom grants.

| Retired ID | Current ID |
|---|---|
| `filesystem.read` | `files.read` |
| `filesystem.write` | `files.write` |
| `search.execute` | `search.run` |
| `process.observe` | `processes.read` |
| `process.manage` | `processes.manage` |
| `process.control.external` | `processes.control` |
| `command.execute` | `commands.run` |

`system.read` keeps its spelling. The loader rejects retired keys and IDs.

### From runtime environment variables

TOML-enabled builds replace the pre-1.0 runtime environment contract with
one [trusted TOML file](configuration.md#source-and-trust). Use the
[template](flashgate-config.toml) and convert only the settings used by the
affected client entry. Write TOML booleans as unquoted `true` or `false`,
integers as unquoted decimal values, and paths as strings.

| Previous environment setting | TOML key |
|---|---|
| `MCP_ROOT` | Set `files.default_id = "work"` and `files.work.path` to an absolute directory. |
| `MCP_READ_ONLY` | Remove the environment setting and use the current profile configuration. |
| `MCP_ALLOW_CWD_ROOT` | Removed. Use the intended absolute directory. |
| `MCP_ALLOW_HIDDEN_FILES` | `security.allow_hidden_files` |
| `MCP_ALLOW_UNC_PATHS` | `security.allow_unc_paths` |
| `MCP_FOLLOW_SYMLINKS` | Set `follow_symlinks` deliberately per root or named policy; no global replacement. |
| `MCP_MAX_FILE_SIZE` | `files.max_file_size` |
| `MCP_MAX_WRITE_BYTES` | `files.max_write_size` |
| `MCP_MAX_LIST_ENTRIES` | `files.max_list_entries` |
| `MCP_MAX_COPY_BYTES` | `files.max_write_size` (shared with writes) |
| `MCP_MAX_DELETE_ENTRIES` | `files.max_delete_entries` |
| `MCP_MAX_JSONRPC_MESSAGE_BYTES` | `protocol.max_message_size` |
| `MCP_MAX_TOOL_ARGUMENT_BYTES` | `protocol.max_argument_size` |
| `MCP_MAX_RESPONSE_BYTES` | `protocol.max_response_size` |
| `MCP_DEBUG` | `logging.level`: use `"debug"` for true and `"off"` for false |

Before switching binaries, back up the client entry and retain the verified
previous release binary, its checksum, and its launch settings. Update the
affected entry to point to the TOML-enabled binary. For a non-default TOML
path, add `--config` and the absolute path as two arguments. Remove the old
`MCP_*` settings from that entry. The TOML-enabled binary ignores them and
fails closed when the selected file is unavailable or invalid. Restart the
client and perform the [acceptance checks](client-setup.md#acceptance-checks).

To roll back, stop the affected client session, restore the verified prior
binary and its backed-up entry together, and restore that release's
`MCP_*` values. Remove the `--config` arguments from the restored entry.
Restart and verify binary identity, initialization, and read-only exposure
under the restored configuration. Keep the TOML file separately for the next
migration attempt.

### Configuration protection rollout

Builds with configuration protection can use an overlapping root when the
[source and root checks](configuration.md#source-and-trust) establish native
protection. An overlapping root grants no ordinary access to the
selected configuration's contents, including through copy, and no
permission to mutate the protected resource.
Before downgrading from a build that accepts an overlapping root, move the
configuration to a location the older build accepts, update the explicit
launch path if needed, and verify startup using the paired client entry.
Role defaults and the TOML schema remain compatible.

## Tool names

| Removed name | Current operation |
|---|---|
| `list_files` | `list_directory` |
| `stat_path` | `get_path_info` |
| `mkdir` | `create_directory` |
| `exists_path` | Use `get_path_info` and its `exists` field. |
| `rename_path` | Use `move_path` for rename and same-volume movement. |

The removed names have no aliases and return generic Invalid params. Refresh
`tools/list`, replace old calls, and update expected results. Default discovery
contains three tools. The explicit `write` profile contains eight.

## Input and operation semantics

Update all filesystem calls to include explicit root IDs. Use `root_id` and
`path` for single-path tools, including `{"root_id":"work","path":"."}` to
list the configured `work` root. Copy and move require `source_root_id` and
`source` plus `target_root_id` and `target`, with identical configured IDs.
Replace `work` with the applicable configured ID. Send the ID even when it
matches `files.default_id` or is the sole configured root.

Tool arguments are closed objects. Unknown or duplicate fields, wrong types,
explicit nulls, and blank or missing paths are rejected.
Omitting `read_file.maxBytes` uses the server limit. An explicit value must be
at least 1 and is capped at the server maximum.

`get_path_info` uses one metadata operation rather than an existence pre-check.
A genuinely absent path returns `path` and `exists:false`. Root, traversal,
hidden, UNC, symlink, reparse, and other policy denials remain errors and are
never converted to absence. Results do not expose resolved absolute host paths.

`create_directory` still creates missing parents. The leaf result is
`created:true` only when newly created; an existing directory yields
`created:false`, and an existing file is a path-type error.

`copy_path` copies files only. Directory or recursive copy is not emulated.
`move_path` supports file/directory rename and same-volume moves within one
root ID. Both copy and move reject distinct IDs. Cross-volume
moves fail without copy/delete fallback. The default `overwrite:false`
rejects existing targets. With `overwrite:true`, replacement is restricted to
file-to-existing-file; existing directories and type mismatches are rejected.

Move validation rejects same paths, safely resolved aliases, Windows case
aliases, `os.SameFile` matches, and lexical or effective self-subtrees. Source
and target identities are rechecked before a native confined rename, without deleting the
target separately. The namespace API cannot condition replacement on the
previously observed target identity, so a concurrent replacement at that
already authorized target retains a residual race window. This is not a claim
of race-free identity-conditional replacement. See [security](security.md).

## Successful tool results

Early builds returned domain objects directly as JSON-RPC `result`. Clients
must now decode the MCP `CallToolResult` envelope:

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "content": [{"type": "text", "text": "file text"}],
    "structuredContent": {"size": 9}
  }
}
```

BL-213 replaces the duplicated `read_file` object with one plain text block.
Read `result.content[0].text` directly as file text and
`result.structuredContent.size` as source byte size. Earlier builds placed the
text in `structuredContent.content` and a JSON-encoded object in the text block.
Clients supporting both wire shapes can use the advertised `read_file`
`outputSchema` to select the decoder.

Handle `invalid_text_encoding` as a tool failure when the input file is not
valid UTF-8. Earlier text serialization could replace malformed bytes.
Valid text retains its original UTF-8 bytes after JSON decoding.

`list_directory` clients read `result.structuredContent.entries`. Its text
block gives a short pointer to the structured data. The server's
list-entry limit now counts filtered entries too. A directory can
return `limit_exceeded` even if few entries would be visible. Small metadata
receipts still offer a compact JSON text copy within the 8 KiB compatibility budget.
Read authoritative metadata from `structuredContent`, including
`exists:false` for a genuinely missing path. Successful calls omit `isError`.

Runtime `outputSchema` is implemented for all eight tools. The catalog's
`resultSchema` describes the same domain result, not the entire JSON-RPC or
MCP wrapper. Version 0.2.0 moves expected failures of registered tools from JSON-RPC
Invalid params into MCP `result` with `isError:true`. Decode the text JSON
`category` and `message`; error results omit `structuredContent`. Request-level
errors and unavailable names remain JSON-RPC errors. Unexpected failures return
generic Internal error. See the [error contract](tools.md#machine-readable-tool-errors).

Validate each payload location, metadata compatibility, missing-path behavior,
discovery, and negative cases through the Windows/Linux smoke tests and the
actual client.
