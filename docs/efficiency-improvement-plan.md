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

## Local deterministic work principle

When FlashGate can express a bounded operation against an authorized local
root, the client should ask FlashGate to perform that operation locally instead
of reading the complete input into the model and sending a rewritten copy back.
This applies especially to:

- copying or moving an already authorized path;
- exact range- or match-based edits with explicit preconditions;
- hashing files and comparing content fingerprints;
- filtering and searching paths or file content; and
- sorting, selecting fields, paginating, or batching local results.

The principle reduces model round trips, token use, wire bytes, and avoidable
serialization while preserving the original bytes that are outside the
requested transformation. Clients should transfer only the selectors,
preconditions, bounded replacement data, and result fields needed for the
operation. A preview, digest, compact summary, cursor, or opaque result handle
should be preferred to retransmitting an unchanged or payload-heavy object.

Local execution is not additional authority. Every operation still passes the
same server-side principal, profile, capability, named-root, path, policy,
limit, and redaction checks as an equivalent direct request. The server owns
resource budgets and validates paths and preconditions at execution time;
client-side inspection, a prior hash, tool visibility, or an MCP annotation
never authorizes a later operation. Ambiguous targets, stale preconditions,
unsupported content, and exceeded limits fail closed.

This principle does not require FlashGate to infer edits, execute a free-form
workflow or shell language, expose host paths, or retain unbounded state. When
no reviewed bounded local operation exists, the client must use the existing
safe primitive or receive an explicit unsupported/capability error rather than
silently expanding the server's behavior.

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

Initial planning budgets are deliberately conservative and must be confirmed by `SPR-047` benchmarks:

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
- revision-specific list-result cache semantics; on `2026-07-28`, return the required `ttlMs` and `cacheScope` fields and keep `cacheScope` non-shared/private unless the complete result is proven independent of principal/profile-sensitive state;
- no reuse of a catalog fingerprint across incompatible protocol revisions or security contexts.

This supports client caching without exposing sensitive configuration details. On the `2026-07-28` path, the same cache-hint model also applies to cacheable resource reads when FlashGate exposes MCP resources: `resources/read` carries `ttlMs`/`cacheScope`, and change invalidation is delivered only through a client-opened `subscriptions/listen` stream for the requested notifications. `2025-11-25` `resources/subscribe` behavior is not copied into the `2026-07-28` stateless path.

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

Development and validation tooling remains outside the FlashGate runtime. The
portable contributor and CI commands are defined in `CONTRIBUTING.md` and
`docs/testing.md`.

## Content identity, verification, and conditional retrieval

Version 1.0 `BL-048` supplies reusable content identities/fingerprints and `BL-346` consumes them for compact batch expected-state verification. A fingerprint identifies observed content/state for equality/change reasoning; it is never an access token and never bypasses current root/profile/capability/principal/path checks.

`verify_paths` can verify one or more expected-state records in one bounded call. The default response reports compact checked/mismatch/indeterminate counts and only the requested bounded mismatch details. This is the portable primitive for any consumer that already holds a manifest-like set of path+identity expectations; FlashGate does not add chat-, handoff-, task-, or workflow-specific verification APIs.

A caller-requested fresh/strong verification re-evaluates the current metadata/content evidence required by the request. Reusable identity evidence or an internal cache may accelerate ordinary work, but a cached proof must not silently satisfy a request whose contract requires current bytes/state to be rechecked.

Conditional read/not-modified behavior remains accepted post-Version 1.0 under `BL-217` because it is an optimization rather than a functional prerequisite. The later design may accept content identities or snapshot IDs for:

- full files and file ranges;
- directory pages;
- search results;
- process output;
- system information.

When the supplied identity still matches, FlashGate may return compact `not_modified` metadata and omit the payload. When it does not match, the current bounded payload/result is returned under the normal contract.

Any server-side identity/content cache is optional and semantically transparent when disabled. Cache entries are bounded and partitioned by the applicable principal/profile/root/execution-backend/service-generation/protocol context and expiry; current authorization is always re-evaluated before a cached result can influence an externally visible response. Knowing a content hash never grants access. A fresh/strong request bypasses reusable proof as required by its contract.

Efficiency validation should record physical-read/hash work, cache/identity reuse, payload bytes returned, and payload bytes avoided so the optimization is measurable without exposing private cache keys or implementation details.

## Portable agent guidance

Post-Version-1.0 `BL-350` publishes one optional FlashGate skill, not separate competing file/cache/search/process skills. Its small core teaches selection strategy—batch before scalar repetition, fields/ranges/pages before broad retrieval, verify-before-reread, conditional retrieval when supported, and resource handoff for large results—while progressive capability references hold filesystem, verification, search, process/execution, platform-storage, and recipe detail. Tool schemas remain the API authority and are not copied into the skill.

The skill detects released protocol/features and degrades to older supported behavior when an optimization is unavailable. It has no dependency on private governance, consumer `AGENTS.md`, task/handoff conventions, local organization paths, or a particular agent/orchestrator product, and it never grants authorization.

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
- [ADR-012](adr/012-resource-token-efficiency-and-pre-1-0-contracts.md)
- [Authoritative backlog](../BACKLOG.md)
