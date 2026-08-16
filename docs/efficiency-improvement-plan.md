# Version 1.0 Efficiency Improvement Plan

## Status

**Accepted planning baseline; not an implementation claim.**

FlashGate optimizes for fast responses, low model-token use, low RAM, low CPU, and minimal runtime dependencies. This plan incorporates the useful patterns found in comparable filesystem, shell, and desktop MCP projects without turning FlashGate into a broad desktop automation framework.

## Efficiency principles

1. Perform deterministic work locally.
2. Expose only tools permitted by the active profile.
3. Transfer payload-heavy content once.
4. Return only requested fields, ranges, pages, or batches.
5. Keep memory bounded and stream or page large data.
6. Prefer native Go and operating-system APIs.
7. Use external programs only through typed no-shell definitions and only with evidence.
8. Measure wire, token, CPU, memory, latency, and call-count cost before and after changes.
9. Preserve server-side authorization regardless of tool visibility or annotations.
10. Defer optional acceleration until benchmarks prove value.

## Adopted improvements from comparative review

### Partial and batch operations

Version 1.0 plans:

- line, byte, head, and tail reads;
- paginated directory listings and search;
- batch reads, metadata, and hashing;
- partial failure for batch inspection where safe;
- field selection and deterministic ordering;
- bounded directory trees;
- incremental process output through cursors.

These reduce tool calls, repeated schema overhead, model round trips, and unnecessary data transfer.

### Dry-run and precise edits

Version 1.0 plans:

- exact targeted changes;
- expected-match-count checks;
- atomic and conditional writes;
- dry-run structured previews;
- bounded filesystem plans rather than a free-form workflow language.

This retains the useful preview/diff behavior of mature filesystem MCPs without requiring full-file retransmission through the model.

### MIME, media, and binary handling

Version 1.0 distinguishes:

- text content;
- media content;
- arbitrary binary content;
- metadata-only results;
- large result resources.

The server records MIME type, raw size, content hash where requested, and safe transfer mode. Base64 is allowed only below explicit thresholds or when a client contract requires it. Large binary content is not embedded by default.

### Tool annotations

Every Version 1.0 tool receives accurate MCP annotations where supported:

- `readOnlyHint`;
- `destructiveHint`;
- `idempotentHint`;
- `openWorldHint`.

Annotations improve client behavior but are never authorization.

## Payload-class result architecture

The current implementation returns compact JSON as both one text block and `structuredContent` for every successful filesystem call. This remains the current state, but it is not the final contract for payload-heavy operations.

Version 1.0 defines result classes:

| Class | Examples | Primary content | Structured metadata |
|---|---|---|---|
| Small metadata | path info, create result, status | compact JSON/text compatibility form | same small object permitted |
| Structured collections | directory page, process page, search page | optional short summary | entries, cursor, counters |
| Heavy text | file range, process output | text once | path/root ID, range, encoding, truncation, hash |
| Binary/media | image, audio, blob | bounded media block or resource handle | MIME, raw size, hash, transfer mode |
| Large asynchronous result | tree, scan, hash batch, long output | resource link or page reference | handle, TTL, progress, size, owner |

### Single-transmission rule

The full heavy payload must not be repeated in both `content` and `structuredContent`.

Compatibility options are evaluated per supported client:

- structured metadata plus one text/media content block;
- resource link plus compact metadata;
- bounded inline fallback when resource links are unsupported;
- explicit capability error when the requested payload cannot be returned safely within the client-compatible limit.

### Wire efficiency metrics

The benchmark suite adds:

```text
wire_amplification = complete_response_bytes / useful_payload_bytes
```

and:

```text
approx_token_cost_per_useful_byte = approx_tokens_bytes4 / useful_payload_bytes
```

For metadata-only operations, useful payload is the canonical compact domain result. For content operations, it is the requested raw text/binary range before transport encoding.

The benchmark also records:

- serialization count and avoidable copies where measurable;
- base64 amplification;
- result/response bytes;
- useful bytes;
- cursor/page overhead;
- proxy/service IPC overhead.

## Large-result and resource abstraction

Version 1.0 introduces server-owned opaque handles such as:

```text
flashgate://result/<opaque-id>
flashgate://process/<opaque-id>/stdout
```

Host paths are never exposed in the URI.

Each handle is bound to:

- caller principal;
- profile;
- root;
- execution backend;
- service generation;
- MIME/content class;
- size and TTL;
- operation/job ownership.

The abstraction supports:

- paged retrieval;
- bounded streaming/chunking;
- negotiated MCP resource links;
- expiry and deterministic cleanup;
- cancellation;
- safe error behavior after restart or expiration.

Resource support is an adapter feature. Core domains return local result references without depending on MCP resource types.

## Tool catalog and initialization budgets

Profiles minimize both privilege and model context.

Initial planning budgets are deliberately conservative and must be confirmed by `SPR-47` benchmarks:

| Profile | Planning target |
|---|---:|
| Safe read | no more than approximately 750 `tools/list` tokens |
| Standard filesystem | no more than approximately 2,000 `tools/list` tokens |
| Process | separate opt-in profile/addition within measured aggregate budget |
| Command | separate opt-in profile/addition within measured aggregate budget |
| Diagnostics | absent unless explicitly enabled |

Budgets cover:

- tool count;
- name/title/description bytes;
- input/output schema bytes;
- approximate tokens;
- server instructions;
- deterministic ordering and catalog fingerprint.

A profile must not expose tools that cannot succeed under its effective capabilities.

## Compact server instructions

The MCP adapter may publish short profile-specific instructions within a fixed token budget. They should tell compatible clients to:

- batch independent reads and metadata requests;
- request ranges instead of complete files;
- request only needed fields;
- paginate directories, search, and process lists;
- use dry-run before destructive plans;
- avoid a separate `get_path_info` call when the target operation already performs the necessary safe validation;
- resume process/search output with the returned cursor;
- use resource handles for large results.

Instructions are guidance, not policy and not a substitute for server limits.

## Deterministic catalogs and cache behavior

Version 1.0 defines:

- deterministic tool ordering;
- deterministic schema property ordering where the serializer permits;
- a catalog fingerprint derived from protocol revision, active profile, effective capabilities, schema versions, and relevant configuration generation;
- invalidation when the effective catalog changes;
- list-result TTL/cache semantics compatible with supported MCP revisions;
- no reuse of a catalog fingerprint across incompatible protocol contracts.

This supports client caching without exposing sensitive configuration details.

## Read-only safe default

The current environment flag remains a compatible implementation input during migration. The target profile system changes the default behavior:

```text
no valid root                    -> fail closed
valid root, no explicit profile  -> safe read-only profile
explicit higher-risk profile     -> validate and enable only its capabilities
```

This reduces both attack surface and `tools/list` cost.

## Typed command efficiency and safety

A command definition is a compact server-side contract, not a shell template.

Illustrative definition:

```toml
[commands.git-status]
executable = "C:\\Program Files\\Git\\cmd\\git.exe"
fixed_arguments = ["status"]
allowed_flags = ["--short", "--branch"]
working_directory_root_required = true
timeout_ms = 10000
max_stdout_bytes = 262144
max_stderr_bytes = 65536
network = false
```

Tool input uses typed fields or enumerations. The server constructs argv and rejects:

- shell metacharacter interpretation;
- response files;
- unapproved configuration overrides;
- hook/plugin/loader injection;
- unapproved environment variables;
- executable-path replacement.

This keeps the tool catalog smaller than one tool per flag combination while avoiding an unrestricted command line.

## Native OS adapter policy

Version 1.0 implementation preference:

1. Go standard library;
2. small platform-specific Go adapter;
3. direct Windows/Linux OS API or documented virtual filesystem such as `/proc` or `/sys`;
4. allowlisted native OS program invoked without a shell only when benchmark and security evidence justify it;
5. interpreter-based scripts are excluded from the Version 1.0 product path.

PowerShell or shell scripts may remain development, installation, smoke, or administrator tooling. They are not runtime dependencies of the FlashGate core/service.

Governance validation follows ADR-0016. Reusable orchestration performs cheap
current-state, toolchain, parser, parameter, Temp, sandbox, and harness
preflights before expensive matrices. It converges with focused root-cause and
component checks, completes documentation, and then performs exactly one full
final run. Progress always means completed selected units, never a pass/fail
pair, and repeated long runs without numeric progress are instrumentation
findings.

BL-339 standardizes that workflow, and BL-340 completes it as nine cataloged data profiles and
one strict request/result contract. The ordered cheap prefix prevents expensive
work after parser, text/EOL, diff, closed external-input classification,
toolchain/context, exact source, or canonical selector-resolution failures.
Task-local generated controllers are exceptional and measured from a declared
inventory; the normal count is zero because the thin runner composes permanent
helpers and typed subordinate results. Unknown exceptions or counter drift fail
closed. Exact PASS/dependency hashes and an eight-component state map allow
unchanged evidence reuse and selective invalidation rather than prophylactic
full reruns. Its reusable `IMPLEMENTATION_TO_INDEPENDENT_FULL_REVIEW` handoff
profile carries that hash-bound full-completion evidence directly into the first
independent review without requiring a review artifact that cannot yet exist.
The same generator supports evidence-only focused review and post-merge closure
as nine-member empty-delta packages, without artificial patch generation.

The correction handoff uses the same generator in directory-first
`PreflightOnly` mode. This performs schema, snapshot, patch, scope, finding,
inventory, and manifest validation once before any productive ZIP open. A
passing preflight exposes `ReadyToExecute` and preserves the validated staging
bytes. A separate ZIP-free `FinalPackageContentOnly` directory then validates
the exact final lifecycle semantics before authorization. This keeps the
runtime package-attempt counter at zero, eliminates trial ZIP writes and
in-place package repair, and lets the later one-shot package operation consume
already validated final-state bytes.

The cheap source gate hashes the complete raw porcelain-v2 status and compares
expected with actual before expensive work. Exact, case-safe equality between
scope and file-hash paths prevents partial bindings. Separately protected
worktrees have independent bindings rather than implicit exclusions.

The canonical workflow metrics are `validationExecutionCount`,
`infrastructureOrInvocationFailureCount`, `fullMatrixRunCount`,
`packageWriteAttemptCount`, `generatedTaskControllerFileCount`,
`generatedTaskControllerLineCount`, and `readOnlyProbeCount`. A completed change
package permits exactly one full-completion run and one final package write.
Directory validation precedes ZIP creation so known content errors cost zero
package-write attempts. The attempt counter becomes one immediately before the
first write-capable candidate open after target preconditions pass, including a
failed candidate open. An existing file or directory target fails before that
attempt; automatic retry is zero.

## Conditional reads

Conditional read/not-modified behavior is accepted post-Version 1.0 because it is an optimization rather than a functional prerequisite.

The later design may use content hashes or snapshot IDs for:

- file ranges;
- directory pages;
- search results;
- process output;
- system information.

The Version 1.0 hashing and fingerprint foundation must not block this later addition.

## Cross-project benchmark

Before Version 1.0, FlashGate will compare pinned versions/commits of:

- FlashGate MCP;
- the official Node.js filesystem reference server;
- one maintained native Rust filesystem MCP;
- one maintained Go filesystem MCP.

The benchmark uses the same host, filesystem corpus, operations, repetitions, and measurement method. It records:

- install/runtime dependency footprint;
- artifact and runtime size;
- cold-labelled first and repeated process starts;
- idle and peak memory;
- CPU time and allocations where comparable;
- p50/p95 latency;
- `tools/list` bytes/tokens;
- request/response bytes;
- useful payload and wire amplification;
- call count for reference workflows;
- behavior under errors and root denial.

Results must not claim superiority for unmeasured functionality. Feature breadth and security model differences are documented separately from performance values.

## Version 1.0 gates

The efficiency architecture is accepted only when:

- no interpreter runtime is required for normal operation;
- heavy payload duplication is eliminated;
- profile catalog and initialization budgets pass;
- large results remain bounded and identity-bound;
- proxy/service overhead is measured against direct STDIO;
- memory and CPU remain within approved Windows/Linux budgets;
- cross-project results are reproducible and source versions are pinned;
- safe read-only is the default profile;
- optional post-Version-1.0 accelerators are not pulled into the initial release without evidence.

## Related documents

- [Comparative MCP review](comparative-mcp-review-2026-07-17.md)
- [Benchmark documentation](../benchmarks/README.md)
- [Architecture](architecture.md)
- [Version 1.0 scope](version-1-scope-and-release-boundary.md)
- [ADR-0012](adr/0012-resource-token-efficiency-and-pre-1-0-contracts.md)
- [Authoritative backlog](../BACKLOG.md)
