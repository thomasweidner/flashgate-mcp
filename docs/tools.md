# Filesystem MCP tools

FlashGate MCP exposes one search tool and eight filesystem tools in the default profile, in this exact order:

```text
search_paths
list_directory
read_file
get_path_info
write_file
create_directory
delete_path
copy_path
move_path
```

The read-only profile exposes `search_paths`, `list_directory`, `read_file`, and `get_path_info`. Write-capable tools are not registered in read-only mode, and calls to unavailable or unknown names return generic JSON-RPC Invalid params.

For later Codex activation, `MCP_READ_ONLY=true` must be explicit and `MCP_ROOT` must be an absolute preflighted directory. See [Codex read-only activation preparation](codex-read-only-activation.md). `SPR-044` does not activate a client.

All paths are relative to the configured root. Absolute paths, traversal, denied hidden/UNC paths, and denied symlink, junction, or reparse access remain server-side errors. Inputs are strict JSON objects: unknown properties, malformed JSON, trailing JSON values, wrong field types, explicit `null` field values, missing required fields, and blank required paths are rejected.

## Successful MCP result envelope

The result examples below are domain objects. Every successful `tools/call` places that object inside an MCP `CallToolResult`:

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

The central adapter serializes the typed domain result once with `encoding/json`. The compact bytes become both the text and `structuredContent`, so decoding the text is deeply equal to the structured object. All nine tools use the same wrapper. For `read_file`, outer `content` is the MCP array while `structuredContent.content` remains the file-text string.

`tools/list` exposes an `outputSchema` for every registered tool: four schemas in the read-only profile and nine in the default profile. Each schema describes only the successful domain object in `structuredContent`; it does not describe the outer `CallToolResult.content[]`. Runtime schemas are deeply matched to catalog `resultSchema` by a contract test. Tool failures retain the existing safe JSON-RPC contract until BL-203.

The current deterministic UTF-8 JSONL `tools/list` response, including its trailing newline, is 5074 bytes for read-only and 8597 bytes for default. The older 1239/2134-byte, 3850/5657-byte, 3046/6569-byte, 4271/7794-byte, and 4894/8417-byte measurements remain historical snapshots, not persistent payload budgets.

## `search_paths`

Recursively enumerates policy-visible descendants below a relative start directory. `path` is optional; omission means `.`, while an explicitly empty or whitespace-only value is invalid. The start directory itself is not returned. Results are normalized with `/`, remain relative to the configured root, and use deterministic UTF-8 byte ordering.

An optional `name` selects an exact, case-sensitive base filename. Alternatively, `namePattern` uses Go `path.Match` syntax (`*`, `?`, and character classes) case-sensitively against each complete base filename. The two selectors are mutually exclusive, must be nonblank, and cannot contain `/`; malformed patterns fail before traversal. Both files and directories can match, while unmatched directories are still traversed so matching descendants remain discoverable. Omitting both selectors retains the path-search behavior.

Portable metadata filters can be combined with filename, path-only, literal-content, or regular-expression search. `type` accepts only `file` or `directory`. Inclusive `minSizeBytes` and `maxSizeBytes` bounds use the file byte size reported by the central filesystem abstraction; directories never match when either size bound is present. Inclusive `modifiedNotBefore` and `modifiedNotAfter` bounds accept RFC 3339 instants, compare the filesystem modification instant independent of its displayed time-zone offset, and apply to files and directories. Negative or reversed bounds, unsupported types, and malformed timestamps fail before traversal.

```json
{
  "path": "docs"
}
```

```json
{
  "paths": [
    { "path": "docs/adr", "isDir": true },
    { "path": "docs/adr/001-use-go.md", "isDir": false },
    { "path": "docs/tools.md", "isDir": false }
  ]
}
```

An optional `text` selects case-sensitive literal UTF-8 content search. Alternatively, mutually exclusive `regex` selects a Go RE2-style regular expression; empty or syntactically invalid expressions fail before traversal, and the runtime does not use a backtracking engine. It is mutually compatible with filename and file metadata filters, but `type: "directory"` is rejected. Content results contain the root-relative file `path` and zero-based UTF-8 `byteOffset` for each deterministic, non-overlapping occurrence. Optional `contextLines` from 1 through 10 adds a `context` string containing the complete line or lines spanned by the match plus that many complete lines before and after, preserving original line endings. The baseline matches UTF-8 selector bytes directly and does not guess an alternate encoding or classify binary data; those modes remain owned by BL-078. Content searches may request positive `maxMatchesPerFile` and `maxMatches` values no greater than the server maxima. Match and context limits are rejected on path-only searches, and out-of-range values fail before traversal. Context is returned only for valid UTF-8 content; it never guesses an encoding or replaces malformed bytes, leaving binary detection and encoding modes to BL-078.

Every search uses fixed server-owned traversal caps of 64 descendant levels and 10,000 visited entries. Entry accounting occurs before result filtering or content reads, so a selective query cannot turn a wide tree into an unbounded scan. Content scanning additionally permits at most 1,000 opened files, 1 MiB per file, 10 MiB scanned across the request, 256 matches per file, 1,000 matches total, 10 context lines on each side, 16 KiB of context per match, 256 KiB of aggregate context, and 1 MiB for the encoded result. The operation fails closed with a safe limit error before crossing a traversal, scan, or response cap and returns no partial result for those failures. Reaching an effective total or per-file match limit instead returns the bounded deterministic prefix with `truncated: true` and a `limit` diagnostic containing the limit kind, configured maximum, returned count, and the root-relative path for a per-file limit. A complete content result has `truncated: false` and omits `limit`. Cancellation is checked during traversal and between returned matches. The same caps and outcome contract apply to literal and regular-expression search; context byte limits fail closed before returning partial results, and pagination remains owned by BL-079.

Path search also has a server-owned cap of 1,000 returned paths and fails with the same safe limit category before exceeding it. Only entries matching every supplied filename and metadata filter count against this result cap, while every visited entry counts against the traversal cap. Every traversal, metadata read, and content read goes through the central filesystem and path-policy boundary.

## `list_directory`

Lists one directory. `path` is optional; omission means `.`, while an explicitly empty or whitespace-only value is invalid.

```json
{
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

No pagination, filtering, recursion, or batch behavior is provided.

## `read_file`

Required: `path`. Optional: `maxBytes` with a minimum of 1. When omitted, the configured server limit is used; a larger client value is capped at that limit.

```json
{
  "path": "README.md",
  "maxBytes": 4096
}
```

```json
{
  "content": "...",
  "size": 123
}
```

Range reads are not implemented.

## `get_path_info`

Required: `path`. A single metadata lookup provides both existence and metadata without an existence pre-check.

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

Required: `path`. Optional: `content` (empty is allowed) and `overwrite` (default `false`). Existing limits and root/security enforcement apply.

```json
{
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

Required: `path`. Missing parents are created. `created` describes the actual leaf state.

```json
{ "path": "output/archive" }
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

Required: `path`. Optional: `recursive`, default `false`. Non-empty directories require explicit recursive deletion and remain bounded by the configured deletion limit.

```json
{ "path": "output/archive", "recursive": true }
```

```json
{ "path": "output/archive", "deleted": true }
```

## `copy_path`

Required: `source` and `target`. Optional: `overwrite`, default `false`.

```json
{ "source": "a.txt", "target": "b.txt", "overwrite": false }
```

```json
{ "source": "a.txt", "target": "b.txt", "copied": true }
```

This contract copies files only. Directory copy and recursive copy are not supported.

## `move_path`

Required: `source` and `target`. Optional: `overwrite`, default `false`. This is the single contract for both move and rename.

```json
{ "source": "old.txt", "target": "new.txt", "overwrite": false }
```

```json
{ "source": "old.txt", "target": "new.txt", "moved": true }
```

Files and directories may be renamed or moved on the same volume. Cross-volume moves are rejected without copy/delete fallback. With `overwrite:true`, only file-to-existing-file replacement is allowed. Existing directory targets and all directory replacement combinations are rejected. Same-path, same-file (including detectable hardlinks and Windows case aliases), and lexical or symlink-resolved directory-into-own-subtree operations are rejected before replacement.

The source and target identities are revalidated immediately before the operating-system rename, and existing files are replaced through `os.Rename` without a separate target deletion. The remaining race is limited to a concurrent change at the already authorized target path after final revalidation; the path-based cross-platform API cannot condition replacement on the previously observed file identity. Such a race cannot trigger a directory-removal fallback, and this behavior is narrower than the previous explicit remove-then-rename sequence.

## Errors

Parse, invalid-request, and method errors use the standard JSON-RPC codes. Expected argument, path, policy, not-found, already-exists, path-type, unsupported-operation, and limit failures use `-32602`. Unexpected I/O failures use `-32603`. Error messages are normalized and do not expose absolute host paths or raw operating-system details.

Runtime `outputSchema` and `structuredContent` cover successful calls only. Stable machine-readable MCP tool-error payloads remain separate work.

The previous pre-1.0 contract and required client changes are documented in [filesystem tool contract cleanup](filesystem-tool-contract-cleanup-2026-07-11.md).

## Version 1.0 target contract direction

The sections above describe the current one-search-tool and eight-filesystem-tool implementation. Version 1.0 expands the catalog only through capability profiles and retains a compact safe read-only default.

Planned contract changes include:

- paginated and field-bounded directory results;
- line, byte, head, and tail file ranges;
- batch reads, metadata, and hashing with bounded partial failure;
- text/media/binary classification and explicit transfer modes;
- targeted edits, conditional/atomic writes, dry-run previews, and bounded filesystem plans;
- search, process, typed command, system-information, and job tools in separate capabilities;
- accurate MCP tool annotations;
- deterministic profile catalogs and compact initialization instructions;
- payload classes that transmit large content once instead of duplicating it in text and structured fields;
- opaque principal-bound result/resource handles for content that should not be embedded in one tool response.

Version 1.0 tool names, schemas, annotations, limits, profiles, and error payloads become stable only after their canonical backlog tasks and the release gate are complete. The machine-readable catalog remains a current-implementation snapshot until replaced by generated Version 1.0 profile catalogs.
