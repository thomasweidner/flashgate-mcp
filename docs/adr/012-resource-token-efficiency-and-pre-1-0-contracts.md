# ADR-012: Resource/Token Efficiency and Pre-1.0 Contracts

## Status

Accepted

## Context

FlashGate MCP aims to reduce local resource cost and model round trips. The current project has no production deployment or external compatibility contract, so preserving redundant early tool names would create avoidable long-term cost.

## Decision

Treat startup time, idle RSS, peak memory, CPU, allocations, p50/p95 latency, byte counters, response size, `tools/list` size, calls per reference workflow, and approximate schema/response tokens as measurable quality goals. Server-side bytes, characters, calls, entries, results, duration, and memory remain primary; token estimates are supplemental.

Prefer local deterministic operations, pagination, limits, filtering, sorting, field selection, ranges, head/tail reads, batch inspection, bounded trees, targeted edits, atomic/conditional writes, dry runs, bounded filesystem plans, and compact structured results. Do not introduce a free-form workflow or shell language.

Before 1.0, justified tool names, parameters, and results may change or be removed. Planned cleanup is `list_files` to `list_directory`, `stat_path` to `get_path_info`, `mkdir` to `create_directory`, removal of `exists_path` in favor of `get_path_info`, and removal of `rename_path` in favor of `move_path`. Other base tools remain, with safe contract enhancements planned.

`get_path_info` is planned to represent a missing path structurally with `exists: false`, while distinct normalized errors remain for access, root, path-type, and I/O failures.

## Rationale

Local server work avoids transferring content through the model. Early contract cleanup is cheaper and clearer than artificial deprecation for unpublished tools.

## Consequences

- Sprint 3.41 changes no schema or tool registration.
- Breaking changes require changelog, tests, tool docs, examples, and smoke-test updates.
- A versioning, deprecation, and migration strategy is required before stable release.
- Benchmarks and snapshot tests become contract gates.

## Security Impact

Bounded inputs/results and local operations reduce data exposure and denial-of-service risk. MCP annotations do not replace authorization.

## Implementation Guidance

Perform the tool cleanup in its dedicated sprint. Record baselines before optimization. Use machine-readable errors and schema snapshots, and test all changes through MCP `tools/call`.

## Deferred Decisions

- final normalized error-code set
- exact range, batch, cursor, and plan schemas
- post-1.0 compatibility and deprecation policy
- model-specific token estimation method

## Implementation Amendment - 2026-07-11

Sprint 3.43 implements the decided pre-1.0 baseline as exactly eight default tools in this order: `list_directory`, `read_file`, `get_path_info`, `write_file`, `create_directory`, `delete_path`, `copy_path`, and `move_path`. The read-only profile exposes the first three read tools. The five replaced or redundant names have no aliases.

Inputs are closed objects and are strictly decoded at runtime. `get_path_info` uses one metadata operation and represents genuine absence structurally; policy denials remain errors. `create_directory` reports actual creation state. `copy_path` remains file-only. `move_path` is the single same-volume Move/Rename contract with pre-replacement SameFile, effective-subtree, identity-revalidation, and overwrite safeguards. Stable machine-readable MCP error payloads, runtime output schemas, structured content, directory copy, and general compatibility infrastructure remain deferred.

## Implementation Amendment - 2026-07-12

Sprint 3.45a chooses MCP `CallToolResult` variant B for every successful call from all eight filesystem tools: one `TextContent` block contains compact deterministic JSON, and `structuredContent` contains the same domain object. Protocol conformance takes precedence over minimizing the duplicated payload. Reproducible benchmarks record response bytes, runtime, and allocations for the historical direct form, text-only form, and selected text-plus-structured form without setting CI budgets from a single machine.

Typed domain results and the filesystem core remain independent of MCP. Runtime `outputSchema` is a separate all-eight-tool gate. The existing safe JSON-RPC tool-error model is not partially migrated; BL-203 remains responsible for a complete normalized `isError=true` contract.

## Implementation Amendment - 2026-07-15

Sprint 3.45d implements BL-189 through BL-199 as a three-layer measurement baseline. Existing serialization fixtures remain authoritative and now also report complete JSON-RPC response bytes. Additional in-process benchmarks cover direct `tools/call` handling and schema-bearing `tools/list` responses for read-only and default profiles.

A development-only runner starts the real built server over STDIO, validates initialization and optional calls, samples process resources while the server remains alive, closes stdin, and requires controlled exit. `first_process_start` means the first process after the local script build step; no operating-system cold-cache guarantee is claimed. The standard run uses one first start plus 30 subsequent starts, and quick mode uses 10 subsequent starts.

Windows uses Win32 current/peak working set and user/kernel CPU time. Linux uses procfs `VmRSS`, `VmHWM`, and user/system CPU ticks. Other platforms explicitly report `not_supported` and omit numeric resource fields. Ten read-only reference workflows define request, response, result, filesystem-byte, entry, call, duration, and approximate-token measurements without adding counters to MCP tool results.

Machine-readable format `flashgate-benchmark/v1` excludes host paths and private environment data. Deterministic tool/schema counts, wire sizes, stable allocation records, workflow calls, and counters are hard local budgets. Startup, p50/p95, working set, and CPU are soft review budgets. Full CI execution and cross-run baseline comparison remain deferred to BL-249 and BL-250.

The sole token orientation is `ceil(UTF-8 bytes / 4)`. It is not model-specific, does not use a tokenizer, and is not suitable for billing.
## Planning Amendment - 2026-07-17

The current all-tool text-plus-`structuredContent` parity remains an implementation fact, but Version 1.0 adopts payload classes rather than applying that duplication to every future result. Small metadata may retain compact parity for client compatibility. Payload-heavy file text, binary/media data, search pages, trees, and process output must appear once, with separate compact structured metadata or an identity-bound resource handle.

Version 1.0 adds hard measurement of wire amplification, useful payload bytes, approximate token cost per useful byte, catalog/instruction size, and proxy/service overhead. It also adds opaque `flashgate://` result handles, deterministic catalog fingerprints, profile-specific instructions, and bounded resource-link/inline fallbacks.

The profile target changes the default from a broad implicit profile to: fail when no root is configured; use safe read-only when roots exist but no explicit profile is selected; require explicit activation for write, process, command, or other high-risk capabilities.

Normal runtime implementation prefers Go standard library and native OS adapters. External native programs require typed no-shell definitions plus benchmark and security evidence. Interpreter-based runtime adapters are excluded from Version 1.0. Conditional read/not-modified contracts are accepted post-Version 1.0.

See [Efficiency Improvement Plan](../efficiency-improvement-plan.md) and `BL-213` through `BL-220`.
