# Architecture Traceability

## Authority and dependency semantics

This living matrix maps accepted Version 1.0 architecture requirements to
implementation, retrofit, validation, and closure owners. [BACKLOG.md](../../BACKLOG.md)
owns BL status, sprint placement, and task acceptance. The [implementation
contract](implementation-contract.md) defines shared readiness and closure
rules. These rows describe targets, not current implementation claims.

`Hard predecessors` contains only BL contracts that must exist before the row's
owner starts implementation. An owner with multiple predecessors needs all of
them. `—` means no BL predecessor beyond maintained architecture/ADR sources.
Completed historical predecessors are reusable evidence and are not reopened.
`Reuse / later consumers` names retained code and downstream work without
making a consumer a predecessor. `Validation / gate` names closure evidence,
not a hard implementation dependency unless it also appears in `Hard
predecessors`. Within a sprint, follow hard-edge order. The graph must be
acyclic and no Planned predecessor may be scheduled after its consumer.

Abbreviations: `A` = [architecture](../architecture.md), `S` =
[security](../security.md), `T` = [testing](../testing.md), `R` =
[release scope](release-scope.md), `TA` = [tool adapters](tool-adapters.md).
The [ADR index](../adr/README.md) identifies maintained decisions. Every row
also inherits its owner's focused tests and `T`.

## Release and protocol contracts

| Requirement / authority | Owner BL | Hard predecessors | Reuse / later consumers | Current retrofit target | Validation / gate | Target closure |
|---|---|---|---|---|---|---|
| Exact revision conformance and schema snapshots (A, R, ADR MCP compatibility) | BL-204 | BL-200, BL-201 | BL-257 consumes snapshots; BL-219 later consumes the revision contract | `internal/protocol` and `cmd/server` candidate path; no unsupported advertisement | BL-204, BL-257 | Final-spec conformance and accepted snapshots per advertised revision |
| JSON Schema 2020-12 validation (A, R, ADR MCP compatibility) | BL-212 | BL-200, BL-201 | BL-257 consumes schema contract | Current tool schemas and protocol adapter | BL-212, BL-257 | Dialect, deterministic schemas and negative tests |
| Schema drift enforcement (T, R) | BL-257 | BL-204, BL-212 | CI consumes both accepted owner contracts | Existing snapshot/CI path | BL-257 | Unaccepted schema drift fails CI |
| Payload classes and single transmission (A, R, ADR efficiency contracts) | BL-213 | BL-201 | BL-214 metrics and BL-218 handoff consume it; BL-090 is not required for inline heavy-payload retrofit | Existing `read_file` and other duplicated heavy results | BL-213, BL-214, BL-258 | Heavy bytes appear once; allowed small metadata parity remains bounded |
| Useful-byte and wire-amplification metrics (A, T) | BL-214 | BL-213 | BL-258 later enforces measurements | Existing response serialization and benchmarks | BL-214, BL-258 | Hard budgets and useful-byte measurements exist |
| Catalog and initialization budget definitions (A, R, TA) | BL-215 | — | BL-216 instructions and BL-256 CI consume budgets | `tools/list` and initialization-size baseline | BL-215, BL-256 | Version 1.0 profile-specific limits defined |
| Early Native Tool/no-interpreter policy (A, S, TA) | BL-220 | — | BL-163 and typed-command owners later implement policy; no early command engine | Current native Go runtime and future adapter selection | BL-220, BL-163 | Typed no-shell selection, executable identity and evidence gate defined |
| Current-version candidate and exact-byte local promotion (R, T) | BL-255 | BL-245, BL-248 | BL-262 later consumes verified identity; `current` is an optional alias | Existing CI candidate/verification artifacts | BL-248, BL-255 | Verified VERSION/commit/hash bytes promoted unchanged; local step makes no additional remote/public upload |
| Supply-chain provenance and rollback foundation (A, R, T) | BL-262 | BL-255 | BL-263 later verifies evidence | Current build/release audit and archive paths | BL-262, BL-263 | Provenance, signing plan, SBOM, checksums and atomic rollback evidence |

## Core policy, context, catalog, and state foundation

| Requirement / authority | Owner BL | Hard predecessors | Reuse / later consumers | Current retrofit target | Validation / gate | Target closure |
|---|---|---|---|---|---|---|
| One canonical configuration and precedence model (A, S, ADR runtime modes) | BL-233 | — | BL-101/103/236 and later direct/service/proxy modes consume it | Environment-only `internal/config` | BL-233, BL-241 | One precedence model with mode parity and safe diagnostics |
| Functional rights distinct from risk/profile (A, S, ADR capability profiles) | BL-100 | BL-233 | BL-103 and BL-159 consume rights | Current read-only exposure | BL-100, BL-160 | Functional capability inputs separate from profile and risk |
| Named-root configuration and single-root migration (A, S, ADR filesystem abstraction) | BL-101 | BL-233 | BL-102/103/236 and domains consume roots; reuse current path guard | `MCP_ROOT` bootstrap in `internal/config` | BL-101, BL-161 | Compatible migration and multiple validated roots |
| `root_id` plus relative paths (A, S, ADR filesystem abstraction) | BL-102 | BL-101 | BL-236 and domains consume path contract; reuse realpath/reparse controls | Current tool paths and `internal/security/path.go` | BL-102, BL-161 | No model-visible host path or second validator |
| Safe-default profile/risk configuration (A, S, ADR capability profiles) | BL-103 | BL-100, BL-101, BL-233 | BL-159/110/216/219 consume effective profile | `MCP_READ_ONLY` exposure and server tool boot | BL-103, BL-161 | Compatible safe read-only default; no parallel authorization target |
| Per-root policy and capability composition (A, S) | BL-104–BL-109 | BL-101, BL-102, BL-103 | BL-159/110 and domains consume policies | Current root/path limits and checks | BL-111, BL-160, BL-161 | Root rights, limits, types, reparse and execution policy |
| Immutable principal/groups/profile/root/capability/risk/backend/generation/correlation context and neutral dispatch (A, S, ADR execution identity/domain core) | BL-236 | BL-101, BL-102, BL-103, BL-233 | BL-159 authorizes against it; BL-166/239 and domains consume it | Minimal `internal/mcp/handlers` context and direct MCP-to-`internal/fs` wiring | BL-236, BL-241 | Context built once below adapter; trusted backend selection |
| Central server-side authorization before dispatch (A, S, ADR capability profiles) | BL-159 | BL-100, BL-102, BL-103, BL-104, BL-108, BL-236 | BL-110 catalog and BL-166 audit consume decision; exposure/annotations do not authorize | Current read-only registration and direct handlers | BL-160, BL-161, BL-241 | One authoritative decision before domain/backend dispatch |
| Stable master catalog and request/principal effective view (A, S, ADR capability profiles) | BL-110 | BL-103, BL-159 | BL-216/219 consume catalog; no unsafe global registry mutation | `internal/mcp/tools/registry.go`, `tools_list.go`, server tool boot | BL-111, BL-256 | Visibility follows policy while authorization remains separate |
| Structured audit and end-to-end correlation (A, S, ADR execution identity) | BL-166 | BL-159, BL-236 | BL-239 and domains consume audit; Post-1.0 BL-361 extends integrity/query | `cmd/server` diagnostics and future proxy/job/process paths | BL-166, BL-241 | Bounded redacted events, rotation, retention, backpressure and correlation |
| Reusable binding of state, handles, cursors, results and caches (A, S, ADR operations/jobs) | BL-239 | BL-159, BL-166, BL-236 | BL-090/219 and stateful domains consume rule; BL-090 is not a predecessor | Future and retained state across tools/jobs/processes/resources | BL-239, BL-241 | No cross-principal/root/profile/backend/generation reuse |
| Catalog visibility negatives (A, S, T) | BL-111 | BL-110, BL-159 | Validates effective catalog; no new authorization engine | Catalog tests and direct calls | BL-111 | Hidden tools unavailable; deterministic catalog |
| Authorization bypass negatives (A, S, T) | BL-160 | BL-159 | Validates central decision, including direct calls | Current registration-only assumptions | BL-160 | Crafted calls and stale catalogs cannot bypass policy |
| Root/profile/path negatives (A, S, T) | BL-161 | BL-101, BL-102, BL-103, BL-159 | Validates root and capability controls | Current single-root tool paths | BL-161 | Cross-root, unsafe path and profile denials proven |
| Annotation-is-not-authorization negatives (A, S, T) | BL-171 | BL-159 | Validates central decision | Current MCP annotations | BL-171 | Metadata grants no authority |
| Named-root discovery (A, S, TA) | BL-352 | BL-101, BL-102, BL-159 | Later domains consume root IDs | Current single-root client configuration | BL-352, BL-161 | Bounded root discovery without host-path leakage |
| Compact effective-profile server instructions (A, R) | BL-216 | BL-103, BL-110, BL-215 | BL-256 enforces budget | Current initialization instructions | BL-216, BL-256 | Profile-conditioned instructions reflect effective catalog |
| Exact-revision/profile/config fingerprints and cache scope (A, R, ADR MCP compatibility) | BL-219 | BL-204, BL-212, BL-103, BL-110, BL-239 | BL-209 decision and BL-256 CI consume it | Current private zero-TTL hints in `internal/protocol`/`cmd/server` | BL-219, BL-256, BL-263 | Principal-safe cache scope, invalidation and fingerprint before revision advertisement |
| Catalog/fingerprint/instruction CI (A, T) | BL-256 | BL-215, BL-216, BL-219 | Later catalog work consumes gate | Existing catalog tests and CI | BL-256 | Profile budgets, ordering, instructions and fingerprints fail on regression |
| Final Tasks compatibility decision (A, R, ADR MCP compatibility) | BL-209 | BL-204, BL-212, BL-219 | BL-211 fallback and conditional BL-210 mapping consume decision | Current non-Tasks MCP adapter | BL-209, BL-263 | Final extension/client matrix selected without mixing 2025 experimental lifecycle |
| Bounded no-Tasks fallback (A, R, ADR MCP compatibility) | BL-211 | BL-209 | BL-210 mapping conditional; no custom job-tool contract | Current synchronous result path | BL-211, BL-263 | Bounded synchronous result or explicit capability error |

## Operations and dependent MCP contracts

| Requirement / authority | Owner BL | Hard predecessors | Reuse / later consumers | Current retrofit target | Validation / gate | Target closure |
|---|---|---|---|---|---|---|
| Generic registry, ownership, state, cancellation, deadlines and counters (A, S, ADR operations/jobs) | BL-084–BL-089 | BL-239 | BL-090 storage and later Operations consume lifecycle | No current job manager | BL-098, BL-099 | One bounded context-bound lifecycle foundation |
| Protocol-neutral bounded result store, TTL and cleanup (A, S, ADR operations/jobs) | BL-090 | BL-084–BL-089, BL-239 | BL-218 consumes store; no second store | Inline-only result path and future job state | BL-090, BL-098, BL-258 | One owner for bounds, retrieval, expiry, cleanup and ownership |
| Remaining Operations limits, fairness, cleanup and domain separation (A, S, ADR operations/jobs) | BL-091–BL-099, BL-164 | BL-090 | Later domains/processes/service modes consume lifecycle | No current manager | BL-098, BL-099, BL-164, BL-241 | Context-bound quotas, fairness and deterministic cleanup |
| Official MCP Tasks mapping (A, R, ADR MCP compatibility) | BL-210 | BL-090, BL-209, BL-211 | Uses one Operations lifecycle; extension negotiation grants no authority | Current synchronous MCP adapter | BL-210, BL-263 | State/result/error/cancel/TTL/redaction mapping only if BL-209 selects Tasks |
| Negotiated MCP `flashgate://` handoff (A, R, ADR efficiency contracts) | BL-218 | BL-090, BL-213, BL-239 | Consumes BL-090 store; no second lifecycle or host-path URI | `internal/mcp/tools` heavy result wrapping | BL-218, BL-258, BL-263 | Owner-bound links, MIME/hash/size, TTL and bounded fallback |
| Payload/resource response-efficiency CI (A, T) | BL-258 | BL-213, BL-214, BL-218 | Later domains consume gate | Existing heavy-response tests and CI | BL-258 | Single transmission, wire budgets and handoff regressions enforced |

## Domains, runtime, and release validation

| Requirement / authority | Owner BL | Hard predecessors | Reuse / later consumers | Current retrofit target | Validation / gate | Target closure |
|---|---|---|---|---|---|---|
| Filesystem domain/path safety (A, S, TA, ADR filesystem abstraction) | BL-036–BL-061, BL-063–BL-067, BL-346 | BL-102, BL-159, BL-239, BL-090, BL-218 | Reuse `internal/fs`, `internal/security`, BL-048 hash and BL-049 tree | Eight current tools and single-root paths | BL-161, BL-241, BL-263 | Bounded operations on shared guard/context/result architecture |
| Search `search_paths` and `search_text` (A, S, TA) | BL-068–BL-080, BL-082 | BL-102, BL-159, BL-239, BL-090 | Pure Go baseline; no tool per backlog row | No current search domain | BL-082, BL-241, BL-263 | Bounded confined search and context-bound cursors |
| Managed process ownership/output (A, S, ADR process execution) | BL-113–BL-126, BL-129–BL-135, BL-162, BL-165 | BL-090, BL-159, BL-239 | Reuse Operations, context and output bounds | No current managed-process domain | BL-252–BL-254, BL-241 | Owned handles, limits and safe termination |
| Typed execution policy enforcement (A, S, TA) | BL-163 | BL-159, BL-166, BL-220, BL-236, BL-239 | Typed execution owners and Native Tool adapters consume one policy | No current command engine | BL-163, BL-167–BL-170 | Allowlisted IDs, roots, argv, env and limits; no raw shell |
| Typed commands/platform adapters (A, S, TA, ADR process execution) | BL-136–BL-149, BL-151–BL-152, BL-353 | BL-163, BL-220, BL-239 | Extend policy and OS adapter; no public adapter aliases | No current command engine | BL-167–BL-170, BL-241 | Typed argv, identity, OS isolation and platform parity |
| Scoped system information (A, S, TA) | BL-062, BL-153–BL-157 | BL-159, BL-236 | Reuse domain/context/redaction | No current system-information domain | BL-158, BL-241 | Explicit fields and bounded redacted results |
| Direct/proxy/service host modes (A, R, ADR runtime modes/host lifecycle) | BL-221–BL-231, BL-234–BL-235, BL-237–BL-238, BL-341 | BL-166, BL-233, BL-236, BL-239 | One core behind STDIO/local IPC; Variant A only for Version 1.0 | Current `cmd/server` direct STDIO | BL-241–BL-244, BL-263 | Local-only modes, trusted peer identity, stdout purity and cleanup |
| Multi-client security/lifecycle integration (A, S, T) | BL-241 | BL-090, BL-159, BL-166, BL-239, BL-341 | Validates integrated host and domains | Direct-mode smoke baseline | BL-241, BL-263 | Cross-client, ownership and host-lifecycle evidence |
| Windows/Linux mode release validation (R, T) | BL-242 | BL-241 | Consumes direct/proxy/service artifacts | Platform CI and service asset plans | BL-242, BL-263 | Native mode parity, no interpreter and assets verified |
| Installation/removal/operation documentation (R, T) | BL-243 | BL-241, BL-242 | Documents implemented modes; BL-255 promotion does not imply service availability | Current direct-mode guidance | BL-243, BL-263 | Portable user/admin instructions with current/planned status |
| Direct/proxy/service benchmarks (R, T) | BL-244 | BL-241, BL-242 | Consumes modes and measurement baseline | Current direct-mode baseline | BL-244, BL-263 | Evidence-based managed-mode thresholds |
| Benchmark CI execution (R, T) | BL-249 | — | BL-250 consumes suite | Current benchmark runner | BL-249, BL-263 | Stable CI selection and artifacted measurements |
| Benchmark baseline comparison (R, T) | BL-250 | BL-249 | BL-261 consumes comparable evidence | BL-199 budgets and baseline schema | BL-250, BL-263 | CI fails on hard budget regression |
| Cross-project efficiency evidence (R, T) | BL-261 | BL-249, BL-250 | BL-263 consumes report | FlashGate benchmark and pinned peers | BL-261, BL-263 | Same-host/corpus/workflow comparison without unsupported claims |
| Response-size/benchmark integrity follow-up (R, T) | BL-205, BL-317–BL-323, BL-325–BL-329 | — | Reuse schemas, controllers and deterministic corpus | Benchmark loaders, host evidence and docs | Owning BLs, BL-263 | Bounded, deterministic and truthful measurements |
| Public release governance/validation portability (R, T) | BL-172–BL-173, BL-177–BL-179, BL-331–BL-332 | — | Reuse contribution, workflow and artifact gates | Workflow pins, release/security guidance, ARM64/host-path docs | Owning BLs, BL-263 | Portable, truthful release controls |
| Version 1.0 architecture/release closure (R, T) | BL-263 | BL-242, BL-243, BL-244, BL-249, BL-250, BL-261, BL-262 | Audit every Version 1.0 row plus BL-305/306/308/315 continuous controls | Every retained implementation/public contract in matrix | BL-263 | No ownerless requirement or retrofit except approved waiver; public release only after gate |


## Coverage and future work

Every material Version 1.0 architecture/contract/gate has an explicit row or
validation/closure reference. Documentation-only and continuous maintenance
BLs still follow BACKLOG.md; mere appearance in a contract family does not
prove an architecture requirement is traced. BL-315 task-local validation
checks hard-edge cycles and sprint inversions separately from exact sprint
ownership, contract-family coverage, and material matrix coverage.

Accepted Post-1.0 workstream owners inherit the same context, root,
authorization, state, result, adapter, and validation contracts when
scheduled. They remain outside BL-263's Version 1.0 feature scope and are not
hard predecessors in the Version 1.0 graph.

For each completed row, the owning BL records focused test/CI evidence,
migrated paths/contracts, and any residual dual-bound to one successor. BL-315
reviews parity and plans machine checks for missing coverage, ownerless rows,
invalid references, and unresolved retrofit on Completed tasks. BL-308
maintains architecture and ADRs. BL-263 enumerates and audits these rows
before public release; completed historical prerequisites are reuse evidence,
not proof that later targets have closed.
