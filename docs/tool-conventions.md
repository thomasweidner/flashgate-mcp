# MCP tool conventions

## Naming and exposure

Tool names use stable lower-case snake case and describe the user-visible operation. The `SPR-043` baseline is:

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

The registry determines deterministic exposure order. `tools/list` uses each implementation's single `Definition()` value for name, title, description, input schema, and output schema. MCP names remain in the adapter layer; the filesystem core keeps domain-oriented Go names such as `List`, `Stat`, and `Mkdir`.

## Arguments

Every tool accepts exactly one JSON object. Runtime decoding uses `json.Decoder`, rejects unknown fields and explicit `null` field values, and requires EOF after the first object. Schemas declare `additionalProperties:false`.

Required path fields must be strings and must not be empty or whitespace-only. Validation does not trim or otherwise alter valid path strings. `list_directory.path` is the only optional path; omission defaults to `.`, but an explicit blank value is invalid.

Security and PathGuard policy are enforced server-side and are not delegated to JSON Schema.

## Definitions and schemas

Each tool owns a compact definition containing:

- name;
- human-readable title;
- model-useful description;
- closed input schema with explicit required fields;
- closed success-only output schema for its `structuredContent` object.

`docs/mcp-tool-catalog.json` is the static contract view. A focused contract test compares runtime and catalog name, title, description, complete input schema, and complete runtime `outputSchema`/catalog `resultSchema` parity without introducing a general schema engine.

The catalog `resultSchema` values and runtime `outputSchema` values describe typed successful domain results only. They do not model the outer `CallToolResult.content[]` or the current JSON-RPC error contract. A tests-only structural checker covers the schema keywords emitted by this project; it is not a general JSON Schema 2020-12 validator.

Every successful `tools/call` uses the central MCP adapter wrapper. The outer result is `CallToolResult` with exactly one `TextContent` block whose text is compact deterministic JSON and `structuredContent` containing the same object. No domain field is placed directly on the outer result. Productive results remain structs; Go's standard `encoding/json` provides deterministic struct-field output and sorted map keys if a map is ever used.

## Path and result conventions

All client paths are relative to the configured root. Results may echo the public relative path supplied by the client; they must never expose the PathGuard-resolved absolute host path.

`get_path_info` reports genuine absence as a successful `{path, exists:false}` domain result inside both `CallToolResult` representations, with no `isError=true`. Existing paths include `name`, `isDir`, and `size`. Policy denials are never converted to absence.

`create_directory.created` is `true` only when the leaf directory was created by that call and `false` for an existing directory.

`copy_path` is file-only. `move_path` is the sole Move/Rename contract and is same-volume only.

## Capability gating

The default profile exposes all eight baseline tools. The read-only profile exposes exactly:

```text
list_directory
read_file
get_path_info
```

`write_file`, `create_directory`, `delete_path`, `copy_path`, and `move_path` are write-gated and absent from the read-only registry.

Client activation must set `MCP_READ_ONLY=true` explicitly; the missing-variable default remains the eight-tool profile. The read-only and negative STDIO smokes require identical generic Invalid params responses for every write-gated and removed legacy name.

## Errors

The adapter retains the existing JSON-RPC architecture:

- parse error `-32700`;
- invalid request `-32600`;
- method not found `-32601`;
- expected argument, path, policy, and filesystem contract failures `-32602`;
- unexpected I/O failures `-32603`.

Internal classification uses `not_found`, `already_exists`, `access_denied`, `invalid_path`, `unsupported_path_type`, `unsupported_operation`, `limit_exceeded`, and `io_error`. Messages are safe and generic. Stable wire-level error objects are deferred.

## Compatibility

FlashGate MCP is pre-1.0 and was not productively deployed when `SPR-043` cleaned the contract. Removed names have no alias or deprecation layer. Clients must update their calls and discovery expectations as described in the dated migration.

## Version 1.0 planned conventions

This section is a target contract. It does not claim that the current eight-tool implementation already follows every rule below.

### Safe profile and catalog exposure

For a new Version 1.0 configuration:

- no valid root causes startup failure;
- valid roots with no explicit profile select the safe read-only profile;
- write, process, command, system-sensitive, and destructive capabilities require explicit profile activation;
- unavailable tools are absent from `tools/list` and direct calls fail with the same generic unavailable-tool contract;
- tool ordering, schemas, annotations, and profile instructions are deterministic;
- every profile has an explicit tool-count, catalog-byte, approximate-token, and instruction budget;
- a catalog fingerprint changes when protocol revision, extension set, profile, capability set, schema, annotations, or relevant configuration changes.

The current `MCP_READ_ONLY` environment switch remains a pre-Version-1.0 compatibility mechanism until the profile configuration migration is implemented.

### Result payload classes

Version 1.0 does not apply text-plus-`structuredContent` duplication indiscriminately to payload-heavy results.

| Class | Examples | Target representation |
|---|---|---|
| Small metadata | path status, operation status, counters | compact structured object; text compatibility parity allowed within budget |
| Structured page | directory, search, process page | structured entries/cursor/counters; optional short text summary only |
| Text payload | file range, process stdout/stderr | content once plus compact metadata |
| Binary/media payload | image, archive, arbitrary binary | bounded inline content only below threshold; otherwise opaque resource/result handle |
| Long-running result | tree, hash batch, copy/move plan | operation/job handle plus bounded pages or result resource |

Every result definition states:

- useful payload field or resource;
- metadata and counters;
- truncation/pagination state;
- cursor or handle TTL where applicable;
- maximum inline bytes;
- fallback for clients lacking the preferred representation;
- authorization binding for later reads.

Opaque handles must not expose absolute host paths. Handles, cursors, jobs, processes, and cached results are bound to caller principal, root, profile, capability set, execution backend, service instance, and expiry.

### Tool annotations

Where the negotiated MCP revision supports them, every tool declares accurate:

- `readOnlyHint`;
- `destructiveHint`;
- `idempotentHint`;
- `openWorldHint`.

Annotations are discovery metadata only. They never replace server-side authorization, path validation, execution-identity selection, or risk policy.

### Partial, batch, and field-bounded operations

New tools prefer one bounded call over repeated scalar calls when this lowers total work and response size. Batch tools:

- enforce per-item and aggregate limits;
- preserve deterministic input/result ordering;
- return safe per-item failures where partial completion is valid;
- expose counters for accepted, completed, skipped, and failed items;
- do not weaken path or capability validation.

Range, page, field-selection, and cursor arguments are explicit and closed. Default values are conservative and server caps always win.

### Typed command tools

Command execution never accepts a shell command string. A command tool selects a server-defined `command_id` and supplies a closed typed argument object. Each definition fixes or constrains:

- resolved executable path and optional identity/hash/publisher requirement;
- subcommand and allowed flags;
- positional argument count and type;
- path arguments and their authorized root binding;
- working-directory policy;
- environment allowlist and fixed values;
- timeout, output, concurrency, and process-tree limits;
- network policy where enforceable;
- response-file, hook, plugin, config-injection, and interpreter restrictions.

The server constructs an argument vector and starts the executable directly. It never reparses a generated shell string.

### Native adapter rule

Runtime implementation preference is:

1. Go standard library;
2. small platform-specific Go adapter;
3. direct operating-system API or stable system interface;
4. allowlisted external native operating-system program only after benchmark and security evidence.

Python, PHP, Node.js, Java, or another interpreter is not a normal runtime dependency. Explicit operating-system scripts may be separate administrator/deployment assets, not hidden runtime requirements of MCP tools.
