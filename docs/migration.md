# Migration

This guide collects compatibility changes relevant to clients using early
FlashGate builds. Follow the current [tool reference](tools.md),
[protocol reference](protocol.md), and [changelog](../CHANGELOG.md) when
updating an integration. Internal task renumbering is not a product migration.

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

TOML-enabled builds replace the pre-1.0 runtime environment contract with
one [trusted TOML file](configuration.md#source-and-trust). Use the
[template](flashgate-config.toml) and convert only the settings used by the
affected client entry. Write TOML booleans as unquoted `true` or `false`,
integers as unquoted decimal values, and paths as strings.

| Previous environment setting | TOML key |
|---|---|
| `MCP_ROOT` | `filesystem.root` |
| `MCP_READ_ONLY` | `filesystem.read_only` |
| `MCP_ALLOW_CWD_ROOT` | `filesystem.allow_cwd_root` |
| `MCP_ALLOW_HIDDEN_FILES` | `security.allow_hidden_files` |
| `MCP_ALLOW_UNC_PATHS` | `security.allow_unc_paths` |
| `MCP_FOLLOW_SYMLINKS` | `security.follow_symlinks` |
| `MCP_MAX_FILE_SIZE` | `filesystem.max_file_size` |
| `MCP_MAX_WRITE_BYTES` | `filesystem.max_write_bytes` |
| `MCP_MAX_LIST_ENTRIES` | `filesystem.max_list_entries` |
| `MCP_MAX_COPY_BYTES` | `filesystem.max_copy_bytes` |
| `MCP_MAX_DELETE_ENTRIES` | `filesystem.max_delete_entries` |
| `MCP_MAX_JSONRPC_MESSAGE_BYTES` | `protocol.max_message_bytes` |
| `MCP_MAX_TOOL_ARGUMENT_BYTES` | `protocol.max_argument_bytes` |
| `MCP_MAX_RESPONSE_BYTES` | `protocol.max_response_bytes` |
| `MCP_DEBUG` | `logging.level`: use `"debug"` for true and `"off"` for false |

Use an existing absolute directory for production. Explicit development STDIO
can use `root = "."` with `allow_cwd_root = true` under `[filesystem]`.
The runtime default for `read_only` remains false. The template explicitly
enables read-only for initial setup. BL-101 and BL-103 own named roots and
safe-default profiles.

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
contains eight tools; explicit read-only mode contains three.

## Input and operation semantics

Tool arguments are closed objects and are strictly decoded. Unknown fields,
wrong types, explicit nulls, and invalid blank path values are rejected.
Omitting `list_directory.path` selects `.`; an explicitly blank path does not.
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
`move_path` supports file/directory rename and same-volume moves. Cross-volume
moves fail without copy/delete fallback. The default `overwrite:false`
rejects existing targets. With `overwrite:true`, replacement is restricted to
file-to-existing-file; existing directories and type mismatches are rejected.

Move validation rejects same paths, safely resolved aliases, Windows case
aliases, `os.SameFile` matches, and lexical or effective self-subtrees. Source
and target identities are rechecked before `os.Rename`, without deleting the
target separately. The path-based API cannot condition replacement on the
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
