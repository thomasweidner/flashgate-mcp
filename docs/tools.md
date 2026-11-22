# Filesystem MCP tools

The explicit `write` profile exposes eight filesystem tools in this order:

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

The default `read` profile requires an existing absolute data directory
under a root ID such as `work`.
See [client setup](client-setup.md).

Every operand supplies an exact configured root ID and a nonblank path
relative to that root. Clients provide the ID even for a single-root
configuration. Absolute paths, traversal and disallowed filesystem aliases
are rejected by the server. Inputs are closed JSON objects with exact
property names. Unknown or duplicate properties, malformed or trailing JSON,
wrong types, explicit `null` and missing required fields are rejected.

The [root's effective access](configuration.md#named-policies) must allow
each operand in addition to the applicable profile grants and risk ceiling.

| Tools | Required root rights |
|---|---|
| `list_directory`, `read_file`, `get_path_info` | `files.read` |
| `write_file`, `create_directory`, `delete_path` | `files.write` |
| `copy_path` | `files.read` on source, `files.write` on target |
| `move_path` | `files.write` on source and target |

Copy and move require matching root IDs. An insufficient root grant yields
`access_denied` before any filesystem operation. Tool exposure still follows
the active profile.

## Link and native effect semantics

Every operation consumes the selected root's immutable
[`follow_symlinks`](configuration.md#symlinks-and-reparse-points) value.
The false default denies linked operands and parents. Listings omit them;
stat returns the existing `invalid_path` denial rather than following a target or reporting it
missing. True permits supported relative in-root links while absolute links,
junctions, unsupported reparse objects, escaping chains and unsafe mount
transitions remain denied. Metadata permission never implies content access.

Read and copy sources are checked before content I/O. Existing output handles
are opened without truncation; object/protection checks and result admission
precede destructive effects. Native parent bindings protect create, mkdir,
unlink and rename effects. All roots share the existing effect coordinator.
Observable root, parent, link, source or target drift is denied. These checks
also apply when an extension is allowed and through overlapping root IDs.

Deleting an allowed leaf link removes the link entry. Copy reads the allowed
target; overwriting through a link checks the actual output object. Rename
acts on the source entry rather than moving a link's target. Relative leaf
links can be renamed within the same parent; relocating them or moving a tree
with unproved relative-link projections is denied. Recursive scans do not
descend through directory links. The [security model](security.md#configuration-trust-and-protection)
states the independent-process final namespace residual; these operations do
not claim atomic identity-conditional rename or unlink.

Content-target and namespace-entry parents are checked independently, including
leaf links into another directory. Removing a leaf directory link leaves all
target children intact, with either recursive flag. Directory-move preflight
and native-boundary revalidation share the selected root's single SCAN budget;
both actual scans consume that same remaining allowance.

## File-type enforcement

The [effective root file-type policy](configuration.md#file-types) is enforced
by the filesystem core, including direct domain calls. `list_directory` omits
denied file entries while still charging scan bytes and entry limits.
`read_file` denies before opening contents. `get_path_info` returns
`access_denied` for a denied file rather than `exists:false`. Authorized
content-independent configuration metadata remains available only when the
file-type and other applicable policies permit it.

Writes and file copies check prospective names and existing overwrite objects;
copy also checks the source before producing output. Move/rename checks both
names and objects and inspects projected descendant names for directory moves.
Recursive deletion preflights the complete bounded tree before any effect.
An already detectable denial leaves the tree and destination unchanged.
`create_directory` retains its structural behavior, including directory names
that resemble a denied file suffix. Directory copy and cross-root copy/move
remain unsupported. No public tools or schemas are added.

Native identity and alias checks are reapplied at the existing effect boundary.
An allowed link name cannot hide a denied resolved name; unavailable
classification or enforcement denies the route. Relative-link relocation
with an unproved future target is denied independently of file-type restrictions.
These rules do not inspect MIME or contents, hydrate placeholders or override
PathGuard, configuration protection, grants, root access, risk or limits.

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

`tools/list` exposes an `outputSchema` for each registered tool. The `read`
profile has three schemas and `write` has eight. Each schema describes the
successful domain object in `structuredContent`. A contract test matches
the runtime schemas to catalog `resultSchema`. Expected tool failures follow
the machine-readable error contract below.

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
newline, is 2517 response bytes and 2482 result bytes for `read`, and 6808
response bytes and 6773 result bytes for `write`.

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

Deleting the bound root itself (`path:"."`) is denied before any child
effect. Delete an authorized child path instead. No root-parent access is
introduced to emulate an unsupported root-self namespace operation.

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

The source and target identities are revalidated immediately before the confined native rename, without a separate target deletion. The namespace API cannot condition replacement on the previously observed file identity. Independent OS changes after the final check retain the documented residual race; they cannot trigger a directory-removal or copy/delete fallback. See the [security model](security.md#configuration-trust-and-protection).

## Resource limits

All eight tools consume the selected root's immutable
[resource policy](configuration.md#resource-limits). Read uses the minimum
of request, file and scan ceilings. Write and file copy share one write ceiling
and also enforce file size; copy additionally consumes source scan bytes.
Directory listing charges all visited name bytes and entries, including filtered
entries. Recursive delete and directory-move inspection bound traversal before
effects. Metadata-only stat, mkdir, delete and rename do not read file contents.
Every success result is checked after serialization, including escaping and
metadata. Mutating tools admit the receipt before changing files. The global
response ceiling independently includes JSON-RPC framing and the newline.
Limit errors contain no partial success. No current tool creates temporary
scratch data; future owners must consume the same reservation contract.

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
