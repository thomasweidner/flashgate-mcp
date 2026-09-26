# Migration

This guide collects compatibility changes relevant to clients using early
FlashGate builds. It describes the current implementation, not an internal
sprint procedure. Follow the current [tool reference](tools.md),
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

Older builds could fall back to the process working directory when `MCP_ROOT`
was absent or empty. That implicit fallback is no longer supported.
`MCP_ROOT` must be absolute, existing, accessible under policy, and a directory.

| Configuration | Current startup result |
|---|---|
| Missing `MCP_ROOT` | `missing_root`, exit 3 |
| Empty, whitespace, general relative root, or `..` | `invalid_root`, exit 3 |
| Nonexistent root | `root_not_found`, exit 3 |
| File instead of a directory | `root_not_directory`, exit 3 |
| Permission or policy denial | `root_not_allowed`, exit 3 |

Expected failures produce no JSON-RPC output and only a safe stderr category.
Do not restore an implicit root or weaken PathGuard policy to make an old
configuration start.

The process working directory is available only for explicit development use:

```text
MCP_ROOT=.
MCP_ALLOW_CWD_ROOT=true
```

`MCP_ALLOW_CWD_ROOT` accepts only lowercase `true` and `false`. It neither
supplies a missing root nor enables any other relative root. A successful
opt-in emits one safe stderr warning. Production client setup uses an absolute
root and `MCP_ALLOW_CWD_ROOT=false`.

For initial read-only acceptance, also set `MCP_READ_ONLY=true`. The current
implicit profile is not read-only; changing that default remains planned
work. Back up and update the affected client entry, then run the checks in
[client setup](client-setup.md). Roll back the entry rather than bypassing
security if acceptance fails.

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
    "content": [{"type": "text", "text": "{\"entries\":[]}"}],
    "structuredContent": {"entries": []}
  }
}
```

The text block is compact deterministic JSON; it and `structuredContent`
decode to the same domain object. Domain fields are not placed directly on
the outer result. Successful calls omit `isError`.

For `read_file`, the domain object remains `{"content":"file text","size":9}`:
outer MCP `content` is an array, while `structuredContent.content` is a string.
Genuine missing-path metadata remains successful in both representations.

Runtime `outputSchema` is implemented for all eight tools. The catalog's
`resultSchema` describes the same domain result, not the entire JSON-RPC or
MCP wrapper. The complete normalized `isError:true` tool-error migration is
still owned by `BL-203`; it is not claimed by this documentation cleanup.
Current expected tool and argument failures use JSON-RPC Invalid params
(`-32602`), while unexpected I/O uses Internal error (`-32603`) with safe,
normalized messages.

Validate envelope shape, semantic parity, missing-path behavior, discovery,
and negative cases through the supported Windows/Linux smoke tests and the
actual client. Accepting valid JSON alone is insufficient. Payload-reduction
planning does not authorize reverting to the invalid unwrapped result form.
