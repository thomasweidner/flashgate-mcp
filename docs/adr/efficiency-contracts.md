# ADR-012: Resource/Token Efficiency and Pre-1.0 Contracts

## Status

Accepted

## Context

FlashGate MCP aims to reduce local resource cost and model round trips. The current project has no production deployment or external compatibility contract, so preserving redundant early tool names would create avoidable long-term cost.

## Decision

Treat startup time, idle RSS, peak memory, CPU, allocations, p50/p95 latency, byte counters, response size, `tools/list` size, calls per reference workflow, and approximate schema/response tokens as measurable quality goals. Server-side bytes, characters, calls, entries, results, duration, and memory remain primary; token estimates are supplemental.

Prefer local deterministic operations, pagination, limits, filtering, sorting, field selection, ranges, head/tail reads, batch inspection, bounded trees, targeted edits, atomic/conditional writes, dry runs, bounded filesystem plans, and compact structured results. Do not introduce a free-form workflow or shell language.

Before 1.0, justified tool names, parameters, and results may change or be removed with coordinated tests, documentation, and migration guidance. The early filesystem name cleanup is implemented; see [migration](../migration.md#tool-names). Further contract enhancements remain owned by the backlog.

`get_path_info` represents genuine absence structurally with `exists: false`; access, root, path-type, and I/O failures remain distinct errors.

## Rationale

Local server work avoids transferring content through the model. Early contract cleanup is cheaper and clearer than artificial deprecation for unpublished tools.

## Consequences

- Breaking changes require changelog, tests, tool docs, examples, and smoke-test updates.
- A versioning, deprecation, and migration strategy is required before stable release.
- Benchmarks and snapshot tests become contract gates.

## Security Impact

Bounded inputs/results and local operations reduce data exposure and denial-of-service risk. MCP annotations do not replace authorization.

## Implementation Guidance

Record baselines before optimization. Use machine-readable errors and schema snapshots, and test all changes through MCP `tools/call`.

## Deferred Decisions

- final normalized error-code set
- exact range, batch, cursor, and plan schemas
- post-1.0 compatibility and deprecation policy
- model-specific token estimation method

## Current filesystem baseline

The implemented pre-1.0 baseline has exactly eight default tools in this order: `list_directory`, `read_file`, `get_path_info`, `write_file`, `create_directory`, `delete_path`, `copy_path`, and `move_path`. The read-only profile exposes the first three read tools. The five replaced or redundant names have no aliases.

Inputs are closed objects and are strictly decoded at runtime. `get_path_info` uses one metadata operation and represents genuine absence structurally; policy denials remain errors. `create_directory` reports actual creation state. `copy_path` remains file-only. `move_path` is the single same-volume Move/Rename contract with pre-replacement SameFile, effective-subtree, identity-revalidation, and overwrite safeguards. Directory copy remains deferred. BL-203 implements normalized errors for expected failures of registered tools; later error categories and capabilities remain planned. Successful result wrapping and runtime output schemas are implemented as described below.

## Current successful result contract

BL-213 replaces universal text parity with registered result variants. The
contract separates four payload kinds, transfer, and lifecycle. The central
adapter supports ordered parts with derived metadata for complete inline
results. UTF-8 file text and directory entries have one wire location.
Standalone object sizes are checked before serialization. Small metadata retains bounded
compatibility parity. Reproducible benchmarks preserve the historical direct,
text-only, and text-plus-structured comparison fixtures.

Typed domain results and the filesystem core remain independent of MCP.
Runtime `outputSchema` describes the successful structured object for each
tool. BL-203's [machine-readable tool-error contract](../tools.md#machine-readable-tool-errors)
separates normalized expected failures from JSON-RPC request and internal
errors.

## Measurement contract

The benchmark components introduced under BL-189 through BL-199 provide a three-layer measurement baseline. Current completion status is recorded in the backlog. Existing serialization fixtures remain authoritative and now also report complete JSON-RPC response bytes. Additional in-process benchmarks cover direct `tools/call` handling and schema-bearing `tools/list` responses for read-only and default profiles.

A development-only runner starts the real built server over STDIO, validates initialization and optional calls, samples process resources while the server remains alive, closes stdin, and requires controlled exit. `first_process_start` means the first process after the local script build step; no operating-system cold-cache guarantee is claimed. The standard run uses one first start plus 30 subsequent starts, and quick mode uses 10 subsequent starts.

Windows uses Win32 current/peak working set and user/kernel CPU time. Linux uses procfs `VmRSS`, `VmHWM`, and user/system CPU ticks. Other platforms explicitly report `not_supported` and omit numeric resource fields. Ten read-only reference workflows define request, response, result, filesystem-byte, entry, call, duration, and approximate-token measurements without adding counters to MCP tool results.

Machine-readable format `flashgate-benchmark/v1` excludes host paths and private environment data. Deterministic tool/schema counts, wire sizes, stable allocation records, workflow calls, and counters are hard local budgets. Startup, p50/p95, working set, and CPU are soft review budgets. Full CI execution and cross-run baseline comparison remain deferred to BL-249 and BL-250.

The sole token orientation is `ceil(UTF-8 bytes / 4)`. It is not model-specific, does not use a tokenizer, and is not suitable for billing.

BL-214 checks actual framed responses against independent semantic references
and versioned wire budgets in the ordinary Go suite. The gate derives its
current variant inventory from the executable registry and uses the existing
conformance paths for exact revision and effective profile coverage. It records
bounded metadata parity separately from useful bytes and observes explicit
response operations without adding wire fields. The
[benchmark contract](../../benchmarks/README.md#deterministic-wire-efficiency)
defines accounting and the producer integration rule.

## Accepted Version 1.0 target

The [catalog and entry contract](../planning/efficiency.md#tool-catalog-and-initialization-budgets)
binds budgets to exact executable paths and caps the entire effective
composition. Later public wire producers extend the existing wire gate under
their own backlog owner.

Search pages, trees, process output, and media use the same single-transmission
rule as the current filesystem payloads. Paged and resource delivery, jobs,
notifications, and streaming are separate mechanisms owned by their feature
and transport contracts.

Version 1.0 extends wire measurement to active delivery routes, effective
catalogs and instructions, and proxy/service overhead. It also adds opaque
`flashgate://` result handles, deterministic catalog fingerprints,
profile-specific instructions, and bounded resource-link/inline fallbacks.

The profile target changes the default from a broad implicit profile to: fail when no root is configured; use safe read-only when roots exist but no explicit profile is selected; require explicit activation for write, process, command, or other high-risk capabilities.

Normal runtime implementation prefers Go standard library and native OS adapters. External native programs require typed no-shell definitions plus benchmark and security evidence. Interpreter-based runtime adapters are excluded from Version 1.0. Conditional read/not-modified contracts are accepted post-Version 1.0.

See [Efficiency Improvement Plan](../planning/efficiency.md) and `BL-213` through `BL-220`.
