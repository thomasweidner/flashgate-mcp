# Filesystem MCP tools

FlashGate MCP exposes eight filesystem tools in the default profile, in this exact order:

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

The read-only profile exposes only `list_directory`, `read_file`, and `get_path_info`. Write-capable tools are not registered in read-only mode, and calls to unavailable or unknown names return generic JSON-RPC Invalid params.

For initial read-only client setup, set `files.read_only = true` and configure
an existing absolute data directory under a root ID such as `work`.
See [client setup](client-setup.md).

Every operand supplies an exact configured root ID and a nonblank path
relative to that root. Clients provide the ID even for a single-root
configuration. Absolute paths, traversal and disallowed filesystem aliases
are rejected by the server. Inputs are closed JSON objects with exact
property names. Unknown or duplicate properties, malformed or trailing JSON,
wrong types, explicit `null` and missing required fields are rejected.

## Successful MCP result envelope

Every successful `tools/call` uses an MCP `CallToolResult`. A small metadata
result can carry a compact JSON compatibility copy:

```json
{
  "content": [
    { "type": "text", "text": "{\"path\":\"missing.txt\",\"exists\":false}" }
  ],
  "structuredContent": {
    "path": "missing.txt",
    "exists": false
  }
}
```

The central adapter places each payload in one location. `list_directory`
returns entries in `structuredContent.entries` and a fixed text summary.
`read_file` returns valid UTF-8 file text in `content[0].text` and source byte size
in `structuredContent.size`. The other six tools return metadata in
`structuredContent`, with a compact JSON text copy for objects up to 8 KiB.
Larger metadata objects within the result limit receive the same fixed text
summary as structured collections.

`tools/list` exposes an `outputSchema` for every registered tool: three schemas in the read-only profile and eight in the default profile. Each schema describes only the successful domain object in `structuredContent`; it does not describe the outer `CallToolResult.content[]`. Runtime schemas are deeply matched to catalog `resultSchema` by a contract test. Expected tool failures use the separate machine-readable error contract below.

The same current MCP `2025-11-25` definitions expose all four annotation members explicitly:

| Tool | `readOnlyHint` | `destructiveHint` | `idempotentHint` | `openWorldHint` |
|---|---:|---:|---:|---:|
| `list_directory` | `true` | `false` | `true` | `false` |
| `read_file` | `true` | `false` | `true` | `false` |
| `get_path_info` | `true` | `false` | `true` | `false` |
| `write_file` | `false` | `true` | `false` | `false` |
| `create_directory` | `false` | `false` | `true` | `false` |
| `delete_path` | `false` | `true` | `true` | `false` |
| `copy_path` | `false` | `true` | `false` | `false` |
| `move_path` | `false` | `true` | `true` | `false` |

`write_file` and `copy_path` are not tool-wide idempotent because `overwrite:true` can replace an existing target. Annotations are client-facing discovery hints only. Registration, read-only/profile gating, capabilities, root/path/security checks, and execution identity remain authoritative server-side decisions.

The deterministic UTF-8 JSONL `tools/list` response, including the trailing
newline, is 2517 response bytes and 2482 result bytes for read-only, and 6808
response bytes and 6773 result bytes for default.

## `list_directory`

Requires `root_id` and `path`. Use `.` to list the selected root directory.

```json
{
  "root_id": "work",
  "path": "docs"
}
```

```json
{
  "entries": [
    { "name": "tools.md", "isDir": false, "size": 123 }
  ]
}
```

The configured entry limit counts every entry scanned, including entries
filtered by policy. Returned entries are also subject to a result byte limit.
Exceeding either limit returns `limit_exceeded` without a partial listing.

## `read_file`

Requires `root_id` and `path`. Optional `maxBytes` has a minimum of 1 and defaults to the server limit. A larger client value is capped at that limit.

The file must contain valid UTF-8. After JSON decoding, the text encodes to
the original bytes, including any BOM, line endings, or control characters.
An empty file returns an empty string and size zero. Invalid UTF-8 returns
`invalid_text_encoding` without file data.

```json
{
  "root_id": "work",
  "path": "README.md",
  "maxBytes": 4096
}
```

```json
{
  "content": [{"type": "text", "text": "file text"}],
  "structuredContent": {"size": 9}
}
```

Range reads are not implemented.

## `get_path_info`

Requires `root_id` and `path`. A single metadata lookup provides both existence and metadata without an existence pre-check.

Existing path:

```json
{
  "path": "README.md",
  "exists": true,
  "name": "README.md",
  "isDir": false,
  "size": 123
}
```

Missing path:

```json
{
  "path": "missing.txt",
  "exists": false
}
```

Only genuine missing-path errors become `exists:false`. Security and policy denials remain errors, and no absolute host path is returned.

## `write_file`

Requires `root_id` and `path`. Optional `content` defaults to empty text and optional `overwrite` defaults to `false`. Limits and root security apply.

```json
{
  "root_id": "work",
  "path": "output.txt",
  "content": "text",
  "overwrite": false
}
```

```json
{
  "path": "output.txt",
  "size": 4,
  "written": true
}
```

## `create_directory`

Requires `root_id` and `path`. Missing parents are created. `created` describes the actual leaf state.

```json
{ "root_id": "work", "path": "output/archive" }
```

New leaf:

```json
{ "path": "output/archive", "created": true }
```

Existing directory:

```json
{ "path": "output/archive", "created": false }
```

An existing file at the target is a path-type error.

## `delete_path`

Requires `root_id` and `path`. Optional `recursive` defaults to `false`. Non-empty directories require explicit recursive deletion and remain bounded by the configured deletion limit.

```json
{ "root_id": "work", "path": "output/archive", "recursive": true }
```

```json
{ "path": "output/archive", "deleted": true }
```

## `copy_path`

Requires `source_root_id` and `source` plus `target_root_id` and `target`.
The root IDs must match. Optional `overwrite` defaults to `false`.

```json
{ "source_root_id": "work", "source": "a.txt", "target_root_id": "work", "target": "b.txt", "overwrite": false }
```

```json
{ "source": "a.txt", "target": "b.txt", "copied": true }
```

This contract copies files only. Directory copy and recursive copy are not supported.

## `move_path`

Requires `source_root_id` and `source` plus `target_root_id` and `target`.
The root IDs must match. Optional `overwrite` defaults to `false`.
The same tool handles moves and renames.

```json
{ "source_root_id": "work", "source": "old.txt", "target_root_id": "work", "target": "new.txt", "overwrite": false }
```

```json
{ "source": "old.txt", "target": "new.txt", "moved": true }
```

Files and directories may be renamed or moved on the same volume. Cross-volume moves are rejected without copy/delete fallback. With `overwrite:true`, only file-to-existing-file replacement is allowed. Existing directory targets and all directory replacement combinations are rejected. Same-path, same-file (including detectable hardlinks and Windows case aliases), and lexical or symlink-resolved directory-into-own-subtree operations are rejected before replacement.

The source and target identities are revalidated immediately before the operating-system rename, and existing files are replaced through `os.Rename` without a separate target deletion. The remaining race is limited to a concurrent change at the already authorized target path after final revalidation; the path-based cross-platform API cannot condition replacement on the previously observed file identity. Such a race cannot trigger a directory-removal fallback, and this behavior is narrower than the previous explicit remove-then-rename sequence.

## Machine-readable tool errors

Registered tools return expected argument and domain failures as MCP
`CallToolResult` with `isError:true`, exactly one text block, and no
`structuredContent`. Text is compact deterministic JSON containing only
`category` and a safe `message`, for example:

```json
{"content":[{"type":"text","text":"{\"category\":\"invalid_path\",\"message\":\"filesystem error: invalid path\"}"}],"isError":true}
```

Active categories are `invalid_arguments`, `invalid_path`, `not_found`,
`already_exists`, `access_denied`, `unsupported_path_type`,
`unsupported_operation`, `invalid_text_encoding`, and `limit_exceeded`. The future vocabulary reserves
`unsupported_capability`, `content_not_local`, `indeterminate`, `canceled`,
and `deadline_exceeded`; their domain functions are not implemented.

Missing, invalid or unknown root IDs produce `invalid_arguments`. Copy and
move across distinct configured root IDs produce `unsupported_operation`.

Malformed requests and outer `tools/call` params remain JSON-RPC errors.
Unknown, removed, and profile-hidden tool names share generic Invalid params
(`-32602`, `invalid params`). After registry resolution, expected failures use
the MCP result above. Unclassified I/O, internal, panic, and serialization
failures use generic JSON-RPC Internal error (`-32603`, `internal error`).
No public `io_error`, raw OS cause, host root, provider identity, or diagnostic
fields are included. Error responses contain either a JSON-RPC `error` or an
MCP `result`, never both. No `_meta` is added.

The success `outputSchema` describes `structuredContent`. Missing
`get_path_info` remains successful with `exists:false`.

## Version 1.0 target contract direction

The sections above describe the current eight-tool implementation. Version 1.0 expands the catalog only through capability profiles and retains a compact safe read-only default.

Planned contract changes include:

- paginated and field-bounded directory results;
- line, byte, head, and tail file ranges;
- batch reads, metadata, and hashing with bounded partial failure;
- text/media/binary classification and explicit transfer modes;
- targeted edits, conditional/atomic writes, dry-run previews, and bounded filesystem plans;
- search, process, typed command, system-information, and job tools in separate capabilities;
- deterministic profile catalogs and compact initialization instructions;
- apply the current single-transmission contract to new payload-producing tools;
- opaque principal-bound result/resource handles for content that should not be embedded in one tool response.

Version 1.0 tool names, schemas, annotations, limits, profiles, and error payloads become stable only after their canonical backlog tasks and the release gate are complete. The machine-readable catalog remains a current-implementation snapshot until replaced by generated Version 1.0 profile catalogs.
