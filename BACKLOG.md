# FlashGate MCP Backlog

This is the authoritative planning and steering document for FlashGate MCP.

FlashGate MCP uses repository `thomasweidner/flashgate-mcp`, local directory `flashgate-mcp`, Go module `github.com/thomasweidner/flashgate-mcp`, binary `flashgate-mcp`, and MCP server implementation name (`serverInfo.name`) `flashgate`.

## Working rules

- Keep `README.md`, `CHANGELOG.md`, and `BACKLOG.md` current at every backlog-item completion; sprint-level review is an additional aggregate check, not the first reconciliation point.
- Define every canonical task exactly once in the task catalog; sprint tables reference IDs only.
- Keep dangerous capabilities disabled or restricted by default.
- Keep protocol output on stdout and diagnostics on stderr.
- Mark target and planned behavior explicitly; do not present it as implemented.
- Preserve completed tasks with status `Completed`.
- Keep changes bound to their canonical backlog owner, reuse completed product gates without reopening their owners, and register genuinely new work only through an explicit backlog update.
- Before a task becomes `Completed`, satisfy its complete acceptance scope or bind every intentionally deferred deliverable to one concrete existing successor BL and add that deliverable to the successor's own acceptance scope. A predecessor-only note is not a handoff, and unowned residual work blocks completion.
- At every task completion, explicitly disposition affected documentation, `CHANGELOG.md`/`VERSION`, tests/CI, candidate artifacts, release/distribution, migration, and security impacts. An explicit not-applicable disposition is acceptable only when the area is genuinely unaffected.
- Every new or materially changed technical BL consumes [the implementation contract](docs/planning/implementation-contract.md) and updates [architecture traceability](docs/planning/architecture-traceability.md) in the same planning change. Resolve its architecture role, authorities, prerequisites, reuse, retrofit targets, public contract delta, invariants, forbidden alternatives, validation, and closure before implementation. An architecture change names affected existing controls and retrofit paths; ownerless requirements and unbound residuals block readiness and closure.
- For accepted future public tool names, adapter taxonomy, cross-cutting security/efficiency rules, and per-capability implementation detail, [the future tool and adapter plan](docs/planning/tool-adapters.md) is the required companion contract. `BACKLOG.md` remains authoritative for task owner, status, and milestone; the planning document supplies the detailed implementation contract and must stay consistent with those owners.
- Write maintained documentation in English and follow `docs/documentation-style.md`. Keep assigned task and sprint IDs stable; personal execution reports and internal ID-migration evidence stay outside the public documentation tree.

> New backlog items receive the next available BL number. Assigned BL IDs remain stable; table position and ID are independent.

## Status legend

| Status | Meaning |
|---|---|
| **Planned** | Accepted work awaiting execution |
| **In Progress** | Work has started and is not terminally closed |
| **Blocked** | Waiting for a decision or dependency |
| Completed | Finished and retained for traceability |
| Rejected | Considered and declined |
| Superseded | Replaced by another accepted work item or decision |

Status describes the execution lifecycle of a human work item or sprint. Sprint, release, and post-Version-1.0 placement are separate planning properties; `Planned` alone does not imply Version 1.0 scope. The sprint sequence and Post-1.0 workstream table below are the canonical placement records.
Non-terminal status values in canonical BL and sprint status cells must be Markdown bold; terminal values must be plain. Markdown decoration does not change the semantic status identity.

## Completed sprint baseline

### SPR-040 - Windows/Linux test matrix and smoke tests

Backlog IDs: `BL-023`, `BL-024`, `BL-025`.

The completed work through `SPR-040` is retained as a concise development index.
Assigned task and sprint IDs remain stable.

## Sprint sequence and status

The sprint sequence defines Version 1.0 scope. The Post-1.0 workstream table defines accepted work beyond the initial stable release. Cross-cutting security, CI, release, governance, and documentation gates apply throughout the implementation sprints and are complete only when their canonical tasks are `Completed`.

Every sprint has one stable standalone positive-integer identifier. Table
position does not change an assigned identifier.

| Sprint | Status | Backlog IDs | Scope |
|---|---|---|---|
| SPR-041 | Completed | BL-026–BL-035 | FlashGate architecture baseline and backlog consolidation |
| SPR-042 | Completed | BL-264–BL-280 | Technical project rename to FlashGate MCP |
| SPR-043 | Completed | BL-281–BL-294 | Pre-1.0 filesystem tool contract cleanup |
| SPR-044 | Completed | BL-174, BL-295–BL-303 | Codex read-only activation preparation |
| SPR-045 | Completed | BL-201 | MCP `CallToolResult` foundation and `structuredContent` |
| SPR-046 | Completed | BL-200 | MCP runtime `outputSchema` integration and parity |
| SPR-047 | Completed | BL-189–BL-199 | Resource, latency, payload, catalog, workflow, and baseline benchmarking |
| SPR-048 | **Planned** | BL-202–BL-208, BL-212–BL-215, BL-220, BL-245, BL-255, BL-257, BL-260, BL-262, BL-363 | Release and contract definitions: current-version candidate/local promotion, supply chain, persistent public pre-1.0 GitHub prereleases, MCP conformance/schema, payload classes and metrics, catalog budgets, native-adapter policy |
| SPR-053 | **Planned** | BL-100–BL-111, BL-159–BL-161, BL-166, BL-171, BL-209, BL-211, BL-216, BL-219, BL-233, BL-236, BL-239, BL-256, BL-352 | Core architecture and effective MCP contracts: named roots, profiles/capabilities, context, authorization, catalog, audit/correlation, state binding, fingerprints, instructions and catalog CI |
| SPR-049 | **Planned** | BL-084–BL-099, BL-164, BL-210, BL-218, BL-258 | Operations/Job Manager and shared result store, then MCP Tasks mapping, resource handoff and dependent payload CI |
| SPR-050 | **Planned** | BL-036–BL-049, BL-346 | Efficient filesystem listing, reading, batch inspection, MIME/binary handling, compare/verify, and large-result handoff |
| SPR-051 | **Planned** | BL-050–BL-061, BL-063–BL-067 | Targeted edits, conditional writes, bounded filesystem plans, and filesystem integration benchmarks |
| SPR-052 | **Planned** | BL-068–BL-080, BL-082 | Filesystem and text search |
| SPR-054 | **Planned** | BL-113, BL-129, BL-162, BL-165 | Process architecture, execution identity, and stateful security model |
| SPR-055 | **Planned** | BL-114–BL-118 | Process observation |
| SPR-056 | **Planned** | BL-119–BL-126, BL-130–BL-135, BL-252–BL-254 | Managed process execution, output cursors, resource control, race tests, and CI jobs |
| SPR-057 | **Planned** | BL-136–BL-149, BL-151–BL-152, BL-163, BL-167–BL-168, BL-170, BL-353 | Typed allowlisted command execution, command discovery, OS isolation, redaction, and security tests |
| SPR-058 | **Planned** | BL-062, BL-153–BL-157 | Scoped and redacted system information |
| SPR-059 | **Planned** | BL-221–BL-225, BL-234–BL-235, BL-237–BL-238 | Multi-mode and IPC architecture, remaining service/runtime contracts, and Variant A security |
| SPR-060 | **Planned** | BL-226–BL-231, BL-341 | Named Pipe/Unix socket transports, proxy/auto modes, Windows SCM service, Linux systemd service, Variant A service-account execution, cross-mode host-process ownership, deterministic shutdown, diagnostics, and orphan prevention |
| SPR-061 | **Planned** | BL-172–BL-173, BL-177–BL-179, BL-241–BL-244, BL-246–BL-251, BL-259, BL-261, BL-263, BL-305–BL-312, BL-314–BL-340, BL-342–BL-344 | Version 1.0 validation, packaging, cross-project benchmarks, governance, documentation, Dependabot maintenance, PR #15/#16/#21 review follow-up, reference-bound legacy Temp cleanup, slim project-governance convergence, and task-bound validation scratch routing |

**SPR-048 quality/version prerequisite order — 2026-09-24.** The prerequisite chain `BL-260 -> BL-245 -> BL-203` is complete: production-server coverage and canonical product versioning are established before subsequent functional work.

**Release and contract gates.** `BL-245 + BL-248 -> BL-255 -> BL-262 -> BL-363 -> further pre-1.0 development -> BL-263` separates canonical versioning and verification, candidate/local promotion, supply-chain evidence, public prerelease publication, and the later stable Version 1.0 gate. `BL-255` is the immediate candidate prerequisite before the next merge that changes `VERSION`; `BL-262` follows its verified local promotion and `BL-363` consumes both without rebuilding. In `SPR-048`, `BL-257` follows `BL-204` plus `BL-212`, and `BL-213` plus `BL-214` establish single-transmission contracts and metrics without requiring the later result store. Profile-conditioned `BL-216`, effective-catalog `BL-219`, and `BL-256` follow the `SPR-053` policy/context foundation. `BL-218`, `BL-210`, and `BL-258` follow the `SPR-049` Operations/result-store prerequisites. These gates complete before dependent domains or public revision/extension support rely on them.

**Post-BL-330 executable lane.** Rebind local `VERSION`, `main`, and current authorities; close BL-315 planning convergence. The near-term order is BL-255 candidate plus BL-248 verification and exact-byte local promotion -> BL-262 supply-chain foundation -> BL-363 public pre-1.0 prerelease publication -> BL-204 + BL-212 -> BL-257 -> BL-213 -> BL-214 -> BL-215 -> BL-220 -> SPR-053. Rebind dependencies, then execute SPR-053 in this order: BL-233 configuration plus BL-100–BL-109 named roots/profile contracts -> BL-236 backend-neutral request/execution context -> BL-159 central authorization -> BL-110 effective catalog -> BL-166 audit/correlation -> BL-239 reusable state binding; finish focused BL-111/160/161/171/352 gates. BL-216 follows profile/catalog contracts; BL-219 follows the effective catalog and state-binding contracts; BL-256 follows BL-215/216/219. BL-209 then decides final Tasks compatibility and BL-211 defines the no-Tasks fallback. In SPR-049, BL-084–BL-089 establish Operations lifecycle, BL-090 adds the one result store, and remaining Operations work follows; BL-210 maps eligible jobs only if selected by BL-209, BL-218 layers MCP resource handoff over BL-090, and BL-258 enforces the completed payload/resource contract. BL-305/306/315 apply at every owner closure. Only then build stateful/public filesystem, search, process and later domains on the final root/profile/backend/context interfaces. Completed historical owners remain closed.

Version 1.0 is reached only after `SPR-061` and the release gate in `BL-263`. The following accepted work is intentionally post-Version 1.0 and has no committed implementation sprint before that release:

| Post-1.0 workstream | Backlog IDs | Direction |
|---|---|---|
| Efficiency and user-isolated hosting | BL-217, BL-232, BL-240 | Conditional reads, user-scoped persistent hosts, and Variant B user-worker implementation |
| Optional accelerators and expanded control | BL-081, BL-083, BL-112, BL-127–BL-128, BL-150, BL-158, BL-360 | Ripgrep/index, legacy Roots, external PID/input, interactive shell, bounded network observation/probes, and the first-party read-only Git Native Tool adapter |
| Provider/community ecosystem | BL-169, BL-176, BL-180–BL-188, BL-313 | External provider security, licensing, governance extensions, provider contracts/runtime, and related documentation |
| Future filesystem and storage capabilities | BL-345, BL-347–BL-348, BL-351, BL-354, BL-359 | Local cloud-placeholder semantics, filesystem watch, archives, content/path compression, encryption state, path security, and extended metadata |
| Future system and platform capabilities | BL-349, BL-355–BL-358, BL-361–BL-362 | Platform configuration, system logs, service/scheduler control, audit integrity/query, and user-session clipboard/notification capabilities |
| Future agent guidance | BL-350 | Portable public FlashGate agent skill |

`SPR-044` replaces the former `SPR-041` Codex preparation plan and must use the FlashGate technical names established in `SPR-042` and the cleaned tool names created in `SPR-043`.

The next free sprint identifier is `SPR-062`. It is reserved as the next
available number only and is not an assigned sprint.

## Canonical task catalog

### Completed foundation and current implementation

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-001 | Completed | Establish Go project foundation | Module, package layout, configuration, security, and filesystem abstraction |
| BL-002 | Completed | Implement root-confined filesystem tools | Current list/read/info/write/create/delete/copy/move implementation |
| BL-003 | Completed | Implement MCP server foundation | JSON-RPC, initialize, `tools/list`, `tools/call`, routing, and server loop |
| BL-004 | Completed | Add package and tool documentation | README, package docs, human and machine-readable tool references |
| BL-005 | Completed | Add CI pipeline | Formatting, vet, separate Windows/Linux coverage-gated tests, lint, build validation, and per-platform coverage artifacts |
| BL-006 | Completed | Add release build workflow | Windows and Linux artifacts under current technical names |
| BL-007 | Completed | Add version and help CLI modes | `--version`, `--help`, and argument validation |
| BL-008 | Completed | Add Windows JSON-RPC smoke script | Real STDIO path on Windows |
| BL-009 | Completed | Run Windows JSON-RPC smoke in CI | Windows CI integration |
| BL-010 | Completed | Document JSON-RPC smoke testing | README usage and validation description |
| BL-011 | Completed | Add Linux/macOS JSON-RPC smoke script | Bash STDIO validation |
| BL-012 | Completed | Update GitHub Actions major versions | Node-24-compatible action versions |
| BL-013 | Completed | Update artifact upload action | Current artifact workflow version |
| BL-014 | Completed | Add optional read-only mode | `MCP_READ_ONLY=true` restricted registration |
| BL-015 | Completed | Enforce filesystem write capability gating | Read-only mode exposes only current read tools |
| BL-016 | Completed | Harden effective-root and traversal validation | Real-path validation for existing paths and create parents |
| BL-017 | Completed | Enforce hidden, UNC, symlink, junction, and reparse policy | Deny-by-default cross-platform path policy |
| BL-018 | Completed | Harden JSON-RPC validation and error behavior | Envelopes, IDs, notifications, batches, params, unknown tools, and panic boundary |
| BL-019 | Completed | Enforce protocol message and tool-argument limits | Bounded JSON-RPC and `tools/call` input |
| BL-020 | Completed | Enforce filesystem operation and response limits | Bounded reads, writes, listing, copy, recursive delete, and responses |
| BL-021 | Completed | Add centralized redaction and safe stderr diagnostics | Secret/host-path redaction, debug gating, and no stdout diagnostics |
| BL-022 | Completed | Add safe defaults for non-developer users | Deny-by-default limits and conservative behavior |
| BL-023 | Completed | Run Linux smoke test in Ubuntu CI | `SPR-040` real STDIO validation |
| BL-024 | Completed | Run JSON-RPC smoke matrix on Windows and Linux | Default, read-only, and negative variants |
| BL-025 | Completed | Isolate smoke JSONL artifacts per run | Unique files and deterministic cleanup |

### Project identity and architecture baseline

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-026 | Completed | Adopt FlashGate MCP public identity | Name, tagline, meaning, scope, non-goals, and technical-name transition |
| BL-027 | Completed | Document current and accepted target architecture | Diagram and explicit current/planned/deferred separation |
| BL-028 | Completed | Define domain-separated local system core | Filesystem, search, process, execution, system, jobs, policy, limits, diagnostics, adapters, MCP |
| BL-029 | Completed | Define core reuse and deployment baseline | Direct Go reuse, one repository/binary, evidence gates for IPC or split |
| BL-030 | Completed | Define vendor-neutral open-source core | The core is vendor-neutral and has no organization-specific infrastructure, credential, permission, or proprietary dependency. |
| BL-031 | Completed | Define FlashGate module/provider direction and decision gate | Shared controls, no identifier or runtime model yet; distinct from MCP protocol extensions |
| BL-032 | Completed | Define Operations/Job Manager architecture | Handles, states, deadlines, cancellation, resources, cleanup, goroutine/process gates |
| BL-033 | Completed | Define capability profiles and named-root direction | Server-side enforcement, profile examples, root policy model |
| BL-034 | Completed | Define managed process and execution architecture | Handles, PID rules, allowlists, no-shell default, single engine |
| BL-035 | Completed | Define efficiency and pre-1.0 contract policy | Metrics, local work, planned cleanup, no artificial compatibility |

### Filesystem epic

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-036 | **Planned** | Add filesystem tests through MCP `tools/call` | Read, write, list, info, missing path, and security cases |
| BL-037 | **Planned** | Add paginated `list_directory` | Bounded pages rather than fail-or-return-all behavior |
| BL-038 | **Planned** | Define stable cursor semantics | Opaque cursor, invalidation, ordering, and policy changes |
| BL-039 | **Planned** | Add listing sort and filters | Portable name/type/metadata behavior |
| BL-040 | **Planned** | Add listing field selection | Return only requested portable fields |
| BL-041 | **Planned** | Add line-range reads | Bounded text line windows |
| BL-042 | **Planned** | Add byte-range and head/tail reads | Bounded binary-safe offsets and edge semantics |
| BL-043 | **Planned** | Expand portable path metadata | Keep `get_path_info` as the single path-metadata query; add field selection for portable type/size/time/permission facts and leave future storage, cloud availability, identity/link, security, and platform-specific fields to BL-345/351/359 under the detailed future-tool/adapter contract |
| BL-044 | **Planned** | Add large-file streaming strategy | Avoid whole-file memory loading and unbounded responses |
| BL-045 | **Planned** | Define text, media, and binary read behavior | MIME detection, explicit modes, inline thresholds, bounded encoding, resource-handoff rules, and client-compatible fallbacks |
| BL-046 | **Planned** | Add batch `read_files` | Per-item bounded results and partial-failure model |
| BL-047 | **Planned** | Add batch `get_paths_info` | Reduce repeated stat/existence round trips |
| BL-048 | **Planned** | Add batch hashing and content fingerprints | Bounded algorithms, byte accounting, reusable content identities/change identifiers, and job handoff. Fingerprints are integrity/change evidence, never authorization tokens, and must remain usable by BL-346 expected-state verification and later BL-217 conditional retrieval without requiring a second hash engine. |
| BL-049 | **Planned** | Add bounded recursive `list_directory` | Extend `list_directory` with `recursive`, `max_depth`, type/name filters, field selection, deterministic sort, cursor, and page limits; prefer flat root-relative entries and do not add a separate `get_directory_tree` tool |
| BL-050 | **Planned** | Add exact targeted file changes | Explicit ranges or match-based edits without model retransmission |
| BL-051 | **Planned** | Add expected-match-count checks | Reject ambiguous or stale targeted edits |
| BL-052 | **Planned** | Add atomic writes | Same-filesystem replace and deterministic cleanup semantics |
| BL-053 | **Planned** | Add conditional write preconditions | Hash, modified-time, and path-type checks |
| BL-054 | **Planned** | Add dry-run support | Structured preview for destructive or multi-step changes |
| BL-055 | **Planned** | Add bounded append mode to `write_file` | Implement append through `write_file(mode="append")` with explicit bounds and existing path/capability checks; do not add a separate public `append_file` tool |
| BL-056 | **Planned** | Add bounded filesystem plans | Limited known operations; no free-form workflow language |
| BL-057 | **Planned** | Enforce plan operation, entry, and byte limits | Preflight and runtime accounting |
| BL-058 | **Planned** | Define cross-volume move behavior | Copy/verify/delete, cancellation, and partial-state rules |
| BL-059 | **Planned** | Define conflict strategy | Fail, skip, replace, and explicit per-operation rules |
| BL-060 | **Planned** | Support directory copy and move | Bounded traversal, jobs, conflicts, and cleanup |
| BL-061 | **Planned** | Add directory-size operation | Streaming scan, limits, progress, and jobs |
| BL-062 | **Planned** | Add scoped disk-usage operation | Root-scoped capacity and privacy-safe results |
| BL-063 | **Planned** | Extend `write_file` safe modes | Canonical modes are `create_only`, `replace_only`, `upsert`, and `append`; retain explicit atomic/conditional behavior and do not add redundant `create_file`, `replace_file`, or `append_file` aliases |
| BL-064 | **Planned** | Integrate long filesystem work with jobs | Preserve filesystem domain ownership |
| BL-065 | **Planned** | Threat-model bounded filesystem plans | Path races, rollback limits, conflicts, and partial completion |
| BL-066 | **Planned** | Add Windows/Linux filesystem MCP integration tests | Exercise real `tools/call` contracts and path behavior |
| BL-067 | **Planned** | Create representative filesystem benchmark corpus | Small/large files, deep/wide trees, binary data, and cross-volume cases |
| BL-346 | **Planned** | Add bounded `compare_paths` and `verify_paths` | Version 1.0 deterministic comparison/revalidation: file↔file and directory↔directory metadata/hash/content/text comparison plus batch expected-state verification for one or more paths using existence/type/size/mtime/hash/content identity/fingerprint/tree inventory and selected metadata. Default results are compact counts plus bounded mismatch/indeterminate deltas; a caller-requested fresh/strong verification must re-evaluate the current required metadata/content proof instead of satisfying the proof solely from a reusable cached result. Reuse BL-048/049 primitives, cheapest safe equality proof first, no second hash engine, and no workflow-/handoff-specific public API. Detailed contract: `docs/planning/tool-adapters.md`. |

### Search epic

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-068 | **Planned** | Define search model and threat model | Scope, recursion, data exposure, resource budgets, and errors; define the two accepted public families `search_paths` and `search_text`, with no per-row public tools |
| BL-069 | **Planned** | Add path search | Relative root-scoped paths; extend `search_paths` under the shared named-root and context contract |
| BL-070 | **Planned** | Add filename search | Literal and pattern matching |
| BL-071 | **Planned** | Add metadata filters | Type, size, and time with portable semantics |
| BL-072 | **Planned** | Add literal text search | Bounded content scanning; extend `search_text` under the shared named-root and context contract |
| BL-073 | **Planned** | Add regular-expression search | Complexity and scan limits |
| BL-074 | **Planned** | Add include/exclude patterns | Deterministic precedence and relative matching |
| BL-075 | **Planned** | Enforce search depth, file, and scanned-byte limits | Server-side hard caps and counters |
| BL-076 | **Planned** | Enforce total and per-file match limits | Bounded results and diagnostics |
| BL-077 | **Planned** | Add bounded context lines | Per-match and aggregate response limits |
| BL-078 | **Planned** | Define binary detection and encoding behavior | Skip/error/explicit modes and reporting |
| BL-079 | **Planned** | Add search pagination | Opaque cursors and stable ordering |
| BL-080 | **Planned** | Add ignore-file support | Explicit policy and optional gitignore-compatible behavior |
| BL-081 | **Planned** | Evaluate optional cross-platform ripgrep accelerator | Pure Go remains the normative `search_text` baseline. A Windows/Linux ripgrep Native Tool adapter may accelerate only equivalent semantics after benchmark/security evidence, executable-identity validation, typed no-shell invocation, deterministic-result handling, and rejection of raw CLI/preprocessor/helper-spawning archive-search behavior. |
| BL-082 | **Planned** | Provide pure-Go search fallback | Portable baseline without external dependency |
| BL-083 | **Planned** | Decide on local search index after benchmarks | No index without measured need and privacy/lifecycle design |

### Operations and Job Manager epic

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-084 | **Planned** | Implement Operation Registry | Thread-safe ownership and lifecycle |
| BL-085 | **Planned** | Generate opaque identity-bound operation handles | `op_<opaque-id>` without guessable internals; bind owner principal, root, profile, execution backend, and service generation |
| BL-086 | **Planned** | Implement operation status model | queued/running/completed/failed/cancelled/timed_out |
| BL-087 | **Planned** | Add context cancellation | Cooperative cancellation contract |
| BL-088 | **Planned** | Add server deadlines and watchdog | Server-controlled timeout enforcement |
| BL-089 | **Planned** | Define progress and byte counters | Read/written/scanned bytes and bounded domain progress |
| BL-090 | **Planned** | Add bounded identity-bound result storage and TTL | Expiry, retrieval, resource handles, owner checks, and cleanup behavior; own one protocol-neutral bounded result store and lifecycle; BL-218 adds MCP resource handoff over this store rather than creating another |
| BL-091 | **Planned** | Add temporary-resource cleanup | Success, failure, cancellation, timeout, and incomplete markers |
| BL-092 | **Planned** | Limit global, per-domain, and per-principal parallel jobs | Configurable safe defaults that prevent one caller from exhausting shared service capacity |
| BL-093 | **Planned** | Limit queues and provide fair scheduling | Global/per-principal queue caps, deterministic overload responses, and starvation resistance |
| BL-094 | **Planned** | Define controlled server shutdown | Cancellation, grace period, worker termination, final state |
| BL-095 | **Planned** | Prevent and detect job leaks | TTL sweep, ownership checks, and metrics |
| BL-096 | **Planned** | Preserve domain ownership | Jobs execute work without becoming its business domain |
| BL-097 | **Planned** | Implement goroutine/subprocess decision rules | External, isolation, cancellation, identity, and platform gates |
| BL-098 | **Planned** | Add cross-platform Operations/Job integration tests | Go package and integration tests for deadline, cancellation, cleanup, shutdown, and temporary-resource behavior on Windows and Linux; this task does not define CI workflow jobs |
| BL-099 | **Planned** | Add job security tests and race detector | Handles, limits, lifecycle races, and leak checks |

### Named roots, capabilities, and profiles epic

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-100 | **Planned** | Implement functional capability model | Functional rights kept separate from profiles and risk classifications |
| BL-101 | **Planned** | Support multiple named roots | Preserve compatible single-root migration path; migrate the current `MCP_ROOT` single-root bootstrap through a documented compatibility path, reusing existing path security controls |
| BL-102 | **Planned** | Use root ID and relative path in target contracts | Avoid model-visible absolute host paths; retrofit current path inputs and `internal/security` confinement to `root_id` plus relative path without a second validator |
| BL-103 | **Planned** | Implement profile and risk-policy configuration with read-only default | If roots exist but no profile is selected, expose only the safe read profile; write/process/command profiles require explicit activation; migrate `MCP_READ_ONLY` tool exposure to safe-default profile/capability semantics with a compatibility path and no parallel authorization target |
| BL-104 | **Planned** | Add per-root read/write policy | Independent permissions per root |
| BL-105 | **Planned** | Add per-root size and result limits | File, scan, response, and temporary data policies |
| BL-106 | **Planned** | Add per-root allowed file types | Explicit portable matching |
| BL-107 | **Planned** | Add per-root symlink/reparse rules | Preserve root confinement and platform semantics |
| BL-108 | **Planned** | Add per-root capability mapping | Tool and operation authorization |
| BL-109 | **Planned** | Add process working-directory permission per root | Execution policy integration |
| BL-110 | **Planned** | Implement dynamic tool registration | Effective profile/capability controls `tools/list`; maintain one stable master catalog and per-request/per-principal effective views; never switch clients by mutating a global registry unsafely |
| BL-111 | **Planned** | Add negative capability and `tools/list` catalog tests | Verify profile-driven tool visibility, dynamic registration, deterministic catalog output, and denial when a tool is absent from the effective catalog; generic server-side authorization bypass tests belong to `BL-160` |
| BL-112 | **Planned** | Evaluate deprecated MCP Roots only for supported `2025-11-25` client compatibility | No architectural dependency; server configuration and explicit root IDs remain authoritative; implement only for a demonstrated supported `2025-11-25` client need; document deprecation and exact protocol-revision scope |
| BL-352 | **Planned** | Add named-root discovery | Add compact `list_roots` and `get_root_info` contracts so agents can discover authorized root IDs, safe labels, effective read/write availability, capabilities and selected limits without deprecated MCP Roots or default absolute-host-path disclosure. Field-selectable results and profile/capability enforcement follow the detailed future-tool/adapter contract. |

### Process epic

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-113 | **Planned** | Create process threat model | Observation, identity, control, disclosure, races, and cleanup |
| BL-114 | **Planned** | Add paginated process list | Bounded stable results |
| BL-115 | **Planned** | Add process field selection | Minimize sensitive output |
| BL-116 | **Planned** | Add process details | Explicit fields and access errors |
| BL-117 | **Planned** | Add process tree | Bounded depth and partial platform data |
| BL-118 | **Planned** | Enforce `process.observe` | Registration and execution checks |
| BL-119 | **Planned** | Implement Managed Process Registry | Thread-safe server-started process ownership |
| BL-120 | **Planned** | Generate opaque process handles and prevent PID reuse errors | Handles are primary identity; PID is diagnostic only |
| BL-121 | **Planned** | Define managed process status | Starting/running/exited/failed/stopped/timed-out states |
| BL-122 | **Planned** | Add `start_process` | Start only server-defined typed `command_id` definitions through the shared Managed Process/Typed Command engine; no second arbitrary exec interface, raw command line, or shell path |
| BL-123 | **Planned** | Add `wait_process` | Context, timeout, and final result semantics |
| BL-124 | **Planned** | Add `read_process_output` | Cursor/bounded incremental output |
| BL-125 | **Planned** | Add separate stdout/stderr ring buffers | Size limits, truncation markers, and cleanup |
| BL-126 | **Planned** | Add `stop_process` | Managed handles by default |
| BL-127 | **Planned** | Evaluate external PID control | Candidate public name is `stop_external_process`; require `process.control.external` or equivalent, high-risk policy, strong PID/process identity and reuse protection, explicit authorization/audit, and absence from standard profiles. |
| BL-128 | **Planned** | Evaluate `write_process_input` | Separate policy and lifecycle gate |
| BL-129 | **Planned** | Define process cleanup, restart, and orphan behavior | Shutdown, server crash/restart, TTL, and diagnostics |
| BL-130 | **Planned** | Limit managed process count and concurrency | Global and profile budgets |
| BL-131 | **Planned** | Enforce process runtime limits | Defaults, maxima, cancellation, and termination |
| BL-132 | **Planned** | Define CPU and RAM enforcement strategy | Distinguish hard enforcement from observed-only/unsupported platform outcomes; policy that requires stronger enforcement than the active backend can prove fails closed instead of silently degrading |
| BL-133 | **Planned** | Redact process command lines and environments | Minimize output and audit data |
| BL-134 | **Planned** | Implement Windows and Linux process adapters | Equivalent policy outcomes with platform-specific internals |
| BL-135 | **Planned** | Add process race, lifecycle, and restart tests | Registry races, PID reuse assumptions, cleanup, and limits |

### Command execution epic

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-136 | **Planned** | Create command-execution threat model | Executable substitution, injection, environment, roots, output, isolation |
| BL-137 | **Planned** | Add typed command definitions with executable IDs | Resolve IDs server-side to approved canonical executables plus fixed subcommands, typed flag/value/path rules, working directory, timeout/output/concurrency/network requirements, and policy-selectable executable identity evidence (reparse/path state, stable file identity, hash/version/publisher/signature where available); no raw adapter options |
| BL-138 | **Planned** | Enforce no-shell default and typed argument objects | No free shell string; server generates argv only from validated structured fields and rejects response-file, hook, plugin, and configuration injection paths |
| BL-139 | **Planned** | Restrict working directories to allowed roots | Named-root process permission integration |
| BL-140 | **Planned** | Define timeout defaults and maxima | Server-enforced deadlines |
| BL-141 | **Planned** | Limit stdout and stderr separately | Bounded buffers/results and truncation markers |
| BL-142 | **Planned** | Define stable command result schema | Exit, output references, timeout, status, and bounded diagnostics |
| BL-143 | **Planned** | Implement `run_command` over Managed Process Engine | Synchronous wrapper over the same typed `command_id` Managed Process engine used by `start_process`; no second execution engine |
| BL-144 | **Planned** | Add environment allowlist and propagation rules | Minimal explicit environment; no unreviewed inherited hooks, loaders, plugin paths, credentials, or interpreter controls |
| BL-145 | **Planned** | Add execution environment and output redaction | Results, diagnostics, and audit events |
| BL-146 | **Planned** | Implement Windows execution isolation | Least privilege plus truthful machine-readable enforcement strength for network/resource/process isolation; required-but-unavailable hard enforcement fails closed |
| BL-147 | **Planned** | Implement Linux execution isolation | Least privilege plus truthful machine-readable enforcement strength for network/resource/process isolation; required-but-unavailable hard enforcement fails closed |
| BL-148 | **Planned** | Limit parallel command processes | Profile/global resource budgets |
| BL-149 | **Planned** | Define least-privilege execution identity | Avoid inherited privileges where practical |
| BL-150 | **Planned** | Keep interactive shell disabled | Separate interactive/high-risk policy decision and threat model |
| BL-151 | **Planned** | Prevent a second execution engine | Enforce that `start_process`, `run_command`, and Native Tool adapters reuse the same typed execution/managed-process policy boundaries rather than exposing an arbitrary exec path |
| BL-152 | **Planned** | Add Windows/Linux execution security tests | Cover allowlist/typed args, roots, env, output, timeout, executable substitution/identity, injection vectors, enforcement-strength mismatch/downgrade, network policy, and platform isolation |
| BL-353 | **Planned** | Add typed-command discovery | Add compact `list_commands` and `get_command_info` contracts for commands authorized by the effective profile/capability context; expose typed argument/limit metadata without default absolute executable-path or adapter-identity disclosure. Detailed contract: `docs/planning/tool-adapters.md`. |

### System information epic

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-153 | **Planned** | Add `get_system_info` | Controlled, field-selectable OS/architecture/version/allowed host facts using the canonical `get_*` metadata naming rule; default output minimizes host/platform disclosure |
| BL-154 | **Planned** | Expose scoped disk usage | Reuse root-scoped operation `BL-062` |
| BL-155 | **Planned** | Add filtered environment information | Allowlist and secret exclusion |
| BL-156 | **Planned** | Add field selection, redaction, and platform-disclosure minimization | Minimize results and host identifiers; portable/default profiles omit unnecessary OS/provider/adapter detail, while platform-sensitive information requires explicit fields/capabilities and diagnostics never become authorization |
| BL-157 | **Planned** | Enforce `system.read` capability | Registration and server-side execution checks |
| BL-158 | **Planned** | Add bounded network observation and probe decision gate | Separate `network.observe`, `network.probe`, and typed-command egress policy. Plan `get_network_info`, `list_network_connections`, `resolve_host`, and `test_tcp_connection` with privacy/SSRF/DNS-rebinding/time/resource controls, explicit host/suffix/port and private/link-local/loopback destination policy, connect-time address validation, and small bounded results; no generic HTTP client or mandatory ICMP/ping in core. |

### Post-Version-1.0 future capability epic

These Post-1.0 workstream owners are accepted future work, not current MCP tools or Version 1.0 release requirements. The shared detailed implementation contract is [the future tool/adapter plan](docs/planning/tool-adapters.md); `BL-215` keeps its names, adapter taxonomy, security/efficiency rules, and owner/milestone mapping consistent with this backlog. Each row below still owns its implementation acceptance boundary. `BL-346` was promoted to Version 1.0 and is therefore listed in the Filesystem epic instead of this Post-1.0 section.

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-345 | **Planned** | Define cloud-backed and placeholder filesystem semantics | Vendor-neutral local-file model; OneDrive is a Windows validation case, not a dependency. Separate reparse classification from materialization policy; default content access to `local_only`, bound optional hydration by files/bytes/time, distinguish local/cloud-sync/network backing, add explicit sync-backed-mutation policy, and expose requested normalized `get_path_info` availability fields such as content state, pin/in-sync state, on-disk/validated/modified-not-synced sizes, and recall-on-open/data-access where reliably available. Preserve indeterminate compare/verify semantics, no watch-triggered hydration, truthful local-write-vs-external-sync results, and a later `set_path_availability` candidate. Windows Cloud Files/CFAPI remains an internal adapter. |
| BL-347 | **Planned** | Add root-confined filesystem watch | Canonical public family is `start_path_watch`, `read_path_watch_events`, and `stop_path_watch` for a file or directory. Require root/profile/principal-bound opaque handles, TTL, bounded queue, debounce/coalescing, sequence/cursor, overflow/resync, global/per-principal limits, cleanup/restart semantics, no content in events by default, no cloud hydration, and Windows ReadDirectoryChangesW-class/Linux inotify-class adapters. |
| BL-348 | **Planned** | Add bounded archive inspection, reading, verification, creation, and extraction | Canonical family: `list_archive_formats`, `get_archive_info`, `list_archive_entries`, `read_archive_entry`, `verify_archive`, `create_archive`, `extract_archive`. Use one format-neutral contract with strict root/staging/path/link/special-file/collision/duplicate/entry/depth/expanded-byte/compression-ratio/conflict/partial-failure bounds. Maintain a capability-advertised candidate format matrix covering ZIP/ZIP64, TAR and common TAR+compression forms, optional 7z/XZ/Zstd/RAR/CPIO, and ISO/WIM inspection only when safe/justified; each format advertises only supported operations. Prefer built-in Go/OS paths; optional 7-Zip/libarchive-class Native Tool adapters get no raw options or extra authority and must satisfy BL-220. |
| BL-349 | **Planned** | Define platform-sensitive configuration reads | Do not invent a generic `get_os_settings` store. Under explicit platform-sensitive capabilities, Windows exposes bounded allowlisted `list_registry_keys`, `list_registry_values`, `get_registry_value`; Linux exposes bounded allowlisted `list_sysctls`, `get_sysctl`. Registry and sysctl remain distinct semantics; xattr belongs to BL-359. Portable semantic settings get a shared domain only where meaning is genuinely equivalent. |
| BL-350 | **Planned** | Publish one portable FlashGate agent skill | Publish one optional portable FlashGate skill with progressive capability-oriented references for filesystem, identity/verification, search, process/execution, platform storage, and efficiency recipes. Teach batch-first selection, fields/ranges/pages, verify-before-reread, content identities, conditional retrieval when available, large-result handoff, roots/profiles/capabilities, annotation-versus-authorization, sync/jobs, and protocol/feature detection with graceful fallback without duplicating tool schemas. Reuse `BL-216` server instructions without treating them as the full skill. The skill has no organization-specific infrastructure, private workflow, or particular agent-product prerequisite and grants no authorization. |
| BL-351 | **Planned** | Extend path storage metadata, compression, and encryption | Keep `get_path_info(fields=["storage"])` as the query owner. Where reliably supported, normalized requested fields include `compression_supported`, `compression_enabled`, `compression_inherited_default`, `encryption_supported`, `encryption_enabled`, `encryption_inherited_default`, sparse state, logical size, and allocated/physical size. An inherited default describes directory/child behavior, not the enabled state of the current path. Add scoped `set_path_compression` for files/directories with an explicit distinction between directory default/inheritance and recursive mutation, plus separate high-risk `set_path_encryption`; never silently transform one storage state to enable the other. Exclude volume/partition compression or encryption administration. |
| BL-354 | **Planned** | Add content compression and codec adapters | Keep content compression distinct from archives and transparent path compression. Canonical family: `list_compression_formats`, `compress_file`, `decompress_file`; plan DEFLATE/GZIP/ZLIB/BZIP2/XZ-LZMA/Zstandard/LZ4/Brotli through standard-library-first bounded codec interfaces and optional reviewed typed adapters, with semantic compression presets rather than raw backend flags. |
| BL-355 | **Planned** | Add platform-sensitive configuration mutation | Separate high-risk write owner from BL-349 reads. Under strict configured allowlists/capabilities/audit, Windows candidates are `create_registry_key`, `set_registry_value`, `delete_registry_value`, `delete_registry_key`; Linux candidate is `set_sysctl`. No general Registry editor, sysctl browser, raw backend arguments, or portable false equivalence. |
| BL-356 | **Planned** | Add bounded system-log query | Canonical family: `list_system_log_sources`, `query_system_logs`; Windows Event Log and Linux journald are internal adapters. Require time/source/severity/event/field filters, cursor/page limits, compact default messages, redaction, and no raw backend/provider leakage. |
| BL-357 | **Planned** | Add generic OS-service observation and control | Separate management of other OS services from FlashGate's own host lifecycle. Canonical family: `list_services`, `get_service_info`, `start_service`, `stop_service`, `restart_service`; Windows SCM and Linux systemd are internal adapters, observation/control capabilities are separate, mutations are high-risk and audited. |
| BL-358 | **Planned** | Add scheduled-job and timer management | Canonical family: `list_scheduled_jobs`, `get_scheduled_job_info`, `create_scheduled_job`, `update_scheduled_job`, `delete_scheduled_job`, `enable_scheduled_job`, `disable_scheduled_job`, `run_scheduled_job`. Windows Task Scheduler and Linux systemd timers are baseline adapters; cron is optional. Persistence/late execution requires explicit high-risk capability, bounded definitions, and audit. |
| BL-359 | **Planned** | Add path security and extended metadata | Portable `get_path_security` plus high-risk `set_path_security`; Windows ADS family `list/read/write/delete_file_stream`; Linux xattr family `list/read/write/delete_extended_attribute`. Do not pretend ADS and xattr are identical; expose platform-sensitive capabilities only explicitly and preserve root/identity/audit boundaries. |
| BL-360 | **Planned** | Add first-party read-only Git Native Tool adapter | Canonical local read-only family: `get_git_status`, `get_git_diff`, `list_git_branches`, `list_git_commits`, `get_git_commit`, `get_git_blame`. Repository must be inside an authorized root; reject raw CLI/options, arbitrary `-c`, hooks, pager/external diff, uncontrolled env, and implicit remote/credential activity; bind Git binary identity through BL-137/220. |
| BL-361 | **Planned** | Add audit integrity and administrative audit query | Extend BL-166 with `get_audit_status`, `query_audit_events`, `verify_audit_integrity` and an explicit optional `audit_integrity = none | hash_chain` mode carrying sequence/previous-event/event/rotation-link evidence when enabled. Keep results bounded/redacted and document hash chaining as tamper evidence/defense in depth, not protection from a fully privileged local attacker. |
| BL-362 | **Planned** | Add user-session clipboard and notification capabilities | Only through a verified user-session backend, never simulated by a system service account. Canonical family: `list_clipboard_formats`, `read_clipboard`, `write_clipboard`, `clear_clipboard`, `send_desktop_notification`; define payload/size/session/privacy bounds. Broad UI automation remains provider/future-architecture work. |

### Security epic

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-159 | **Planned** | Enforce capabilities server-side | Registration is not the authorization boundary; make one authoritative server-side decision before domain/backend dispatch; annotations and catalog visibility never grant authority |
| BL-160 | **Planned** | Add server-side authorization bypass tests | Verify capability enforcement after tool resolution, including crafted direct calls, stale catalog assumptions, root/domain policy bypass attempts, and every high-risk operation; catalog visibility tests belong to `BL-111` |
| BL-161 | **Planned** | Add per-root policy security tests | Read/write, types, limits, links/reparse, working directory |
| BL-162 | **Planned** | Define process policy model | Observe/manage/external-control separation, managed identity, risk classifications, and policy inputs; enforcement remains in concrete implementation tasks |
| BL-163 | **Planned** | Define and enforce execution policies | Executables, args, roots, environment, limits, isolation |
| BL-164 | **Planned** | Add Operations/Job limit security tests | Queue, concurrency, time, temporary data, cleanup, handles |
| BL-165 | **Planned** | Maintain stateful domain threat models | Filesystem, search, processes, execution, FlashGate modules/providers, and MCP protocol extensions |
| BL-166 | **Planned** | Define structured audit lifecycle and trace correlation | Bounded redacted decisions/outcomes, immutable event IDs, end-to-end correlation, rotation, retention, disk-full/backpressure behavior, and log-injection protection; post-Version-1.0 integrity/query extensions are owned by BL-361; establish the structured audit/correlation contract before later domains depend on it, retaining rotation, retention, and backpressure in this owner |
| BL-167 | **Planned** | Extend secret redaction across new domains | Process, execution, system, jobs, audit, and module/provider outputs |
| BL-168 | **Planned** | Validate least-privilege execution | Server and child process permissions |
| BL-169 | **Planned** | Enforce FlashGate module/provider security boundaries | Post-1.0 provider work may not bypass policies, functional capabilities, roots, limits, audit, execution identity, or adapters |
| BL-170 | **Planned** | Document sandbox boundaries and residual risk | OS, process, filesystem, link/reparse, and configuration limits |
| BL-171 | **Planned** | Verify MCP annotations are never authorization | Documentation and negative tests |
| BL-172 | **Planned** | Review and enforce workflow pinning strategy | Version 1.0 supply-chain hardening, SHA-pin tradeoffs, update process, and automated validation |
| BL-173 | **Planned** | Maintain public security policy | Reporting, supported versions, disclosure, and release gate |
| BL-174 | Completed | Fail closed when no root is explicitly configured | Root is required; missing/empty/whitespace fail closed; production roots are absolute; `.` requires explicit `MCP_ALLOW_CWD_ROOT=true`; other relative roots are denied; root must exist and resolve to a permitted directory; safe categorized stderr and exit codes precede Registry/STDIO; Windows/Linux startup smokes cover the contract |

### Open source and FlashGate modules/providers epic

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-175 | Completed | Confirm current open-source license | Repository and README currently declare GNU GPL v3.0; no license change in `SPR-041` |
| BL-176 | **Planned** | Review license and distribution compatibility before external module contract | Post-1.0 factual compatibility gate before the first external provider; no legal conclusions in backlog |
| BL-177 | **Planned** | Define governance model | Decision authority, releases, and stewardship |
| BL-178 | **Planned** | Define maintainer rules | Roles, review, security, and succession |
| BL-179 | **Planned** | Expand contribution guidelines | Development, testing, documentation, DCO/CLA decision, and durable post-merge repository hygiene: after canonical merge/main readback close superseded PRs for the same owner and remove obsolete task branches only when no required stacked child, worktree, or active reference depends on them; merged PR records remain history |
| BL-180 | **Planned** | Add Code of Conduct before public community release | Community expectations and enforcement |
| BL-181 | **Planned** | Hold FlashGate module/provider contract decision gate | Post-1.0 and required before first external provider |
| BL-182 | **Planned** | Decide FlashGate module/provider identifier rules | Post-1.0; no concrete syntax before the contract decision |
| BL-183 | **Planned** | Define FlashGate module/provider metadata | Post-1.0 name, version, vendor, tools, config, platforms, dependencies |
| BL-184 | **Planned** | Define module/provider capability declarations | Post-1.0 required and optional functional capabilities with least privilege |
| BL-185 | **Planned** | Define module/provider security classification | Post-1.0 risk categories and review requirements |
| BL-186 | **Planned** | Distinguish public, community, vendor, and internal providers | Post-1.0 distribution and support labels do not change security |
| BL-187 | **Planned** | Define official versus community provider policy | Post-1.0 trust language, signing/update expectations, and support |
| BL-188 | **Planned** | Decide FlashGate provider runtime model | Prefer built-in/static reviewed components or isolated out-of-process providers for larger optional systems; do not turn the trusted service into an arbitrary DLL/SO/Go-plugin loader. Any in-process model requires an explicit later security decision, and no provider may bypass roots, capabilities, principal binding, limits, execution identity, audit, or redaction. |

### Efficiency and MCP contract foundation

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-189 | Completed | Add startup benchmark | Real binary over STDIO; one `first_process_start` after build plus 30 new subsequent processes by default, 10 in quick mode; no OS cold-cache claim |
| BL-190 | Completed | Measure idle RSS | Windows idle Working Set and native Linux `VmRSS` measurements are retained in the versioned platform baselines with their measurement and revision provenance. |
| BL-191 | Completed | Measure peak memory, CPU time, and allocations | Win32 peak working set/user/kernel time, Linux `VmHWM`/user/system time, existing and direct-handler Go allocation benchmarks, representative single and multi-operation workflows |
| BL-192 | Completed | Measure p50 and p95 latency | Nearest-rank p50/p95 for startup and all ten real-process reference workflows |
| BL-193 | Completed | Record scanned, read, and written bytes | Runner-side counters have explicit semantics and remain outside public MCP results |
| BL-194 | Completed | Measure serialized result sizes | Existing six-fixture historical/text/text-plus-structured benchmark retained; deterministic result and complete response bytes are pinned |
| BL-195 | Completed | Measure `tools/list` size | Read-only/default tool count, schema count, request/result/response bytes, and approximate tokens are deterministic gates |
| BL-196 | Completed | Measure calls per reference workflow | Machine-readable workflows record actual `tools/call` counts, including ten-call independent path/read cases |
| BL-197 | Completed | Add optional schema/response token approximation | `approx_tokens_bytes4 = ceil(UTF-8 bytes / 4)` is clearly non-model-specific and unsuitable for billing |
| BL-198 | Completed | Establish benchmark baselines | Versioned Windows and native Linux baselines, including measurement and revision provenance, are in `benchmarks/baseline.windows-amd64.json` and `benchmarks/baseline.linux-amd64.json`. |
| BL-199 | Completed | Define CI regression budgets | Machine-readable hard deterministic and soft noise-sensitive budgets with local evaluation; full CI execution/comparison remains BL-249/BL-250 |
| BL-200 | Completed | Add MCP `outputSchema` | All eight runtime filesystem tools expose success-only schemas matching catalog `resultSchema` and successful `structuredContent`; no error migration or complete general JSON Schema validation |
| BL-201 | Completed | Add MCP `CallToolResult` foundation and `structuredContent` | All eight successful filesystem tools use one central text-plus-structured wrapper with deterministic parity, strict decoder/wire tests, corrected smokes, and no runtime `outputSchema` |
| BL-202 | Completed | Review MCP tool annotations | Implemented accurate MCP `2025-11-25` discovery annotations for all eight filesystem tools with explicit read-only, destructive, idempotent, and open-world members including `false`; runtime/catalog/wire parity is enforced and annotations remain non-authoritative for registration, profiles, capabilities, path/security checks, execution identity, and authorization. |
| BL-203 | Completed | Define normalized machine-readable errors | Stable categories including safe unsupported-capability/content-not-local/indeterminate outcomes where needed, without raw OS/backend/provider leakage; diagnostic detail remains separately gated |
| BL-204 | **Planned** | Evaluate official MCP conformance testing and add schema snapshots | Add conformance/schema coverage for every advertised revision; `SPR-046` already provides runtime/catalog output-schema parity and `tools/list` wire coverage, while final `2026-07-28` conformance plus complete input/output snapshots remain required before support is advertised; preserve exact-revision conformance ownership and keep unsupported revision state unadvertised until BL-212/219 gates pass |
| BL-205 | **Planned** | Add response-size regression tests | `SPR-046` measures `tools/list` payload impact without setting a persistent budget; success/error regression gates remain planned |
| BL-206 | Completed | Document local deterministic work principle | Documented the canonical rule in `docs/planning/efficiency.md`: prefer bounded typed local copy/edit/hash/search and related deterministic work over model retransmission when the applicable contract exists; distinguish implemented primitives from accepted Version 1.0 operations and preserve authorization and boundedness |
| BL-207 | Completed | Define Version 1.0 MCP protocol matrix | Keep the core version-independent and publish an exact revision matrix. Version 1.0 target: preserve the initialization-based `2025-11-25` path and add the final `2026-07-28` revision only after implementation; every advertised revision gets its own wire-path tests, negotiation/error contract, and breaking-upgrade coverage. Use exact revision identifiers rather than generic generation labels so later revisions can be added explicitly. |
| BL-208 | Completed | Define MCP extension-negotiation and stateless-adapter strategy | Implement a revision dispatcher with a `2025-11-25` initialization path and a `2026-07-28` stateless path. The `2026-07-28` path covers per-request `_meta`, mandatory `server/discover`, `UnsupportedProtocolVersion`, response `serverInfo`, required result `resultType`, required `ttlMs`/`cacheScope` on cacheable list/read results including `resources/read`, and `subscriptions/listen` for opted-in list/resource change notifications when those capabilities are exposed. Any `resources/subscribe`/unsolicited-notification compatibility behavior remains confined to the `2025-11-25` initialization path. Include official extension identifiers/capabilities, downgrade/mismatch/cache-invalidation tests, and zero authorization authority from client metadata, discovery, subscriptions, cache hints, extensions, or cached results. Future revisions require explicit new matrix/path entries rather than implicit inheritance. |
| BL-209 | **Planned** | Decide final MCP Tasks Extension compatibility | Evaluate the final `io.modelcontextprotocol/tasks` extension against the supported Version 1.0 client set and the `2026-07-28` revision after BL-219 effective-catalog/revision readiness; do not mix the 2025 experimental lifecycle with the final extension; do not expose asynchronous MCP behavior until BL-210/211 mapping/fallback and interoperability evidence pass |
| BL-210 | **Planned** | Map internal operation lifecycle to MCP Tasks | Define tested state, result, error, cancellation, TTL, and redaction mapping; execute after BL-209 selects Tasks and BL-090 establishes the internal result lifecycle; internal states may be more detailed |
| BL-211 | **Planned** | Decide fallback when MCP Tasks is unavailable | Bounded synchronous result or explicit capability error; follow BL-209's decision; no ad hoc custom job-tool contract |
| BL-212 | **Planned** | Validate all input/output schemas as JSON Schema 2020-12 | Complete standard-conformant validation, dialect declarations, deterministic property ordering, snapshots, and protocol-version compatibility; coordinate schema dialect and revision snapshots with BL-204/219 and BL-257 drift enforcement |
| BL-213 | **Planned** | Define payload-class result contracts and single-transmission rules | Small metadata may retain text/structured parity; heavy text, binary, search, and process payloads appear once with separate compact metadata and bounded compatibility fallback; retrofit existing payload-heavy results, explicitly including `read_file`, to single transmission; retain small-metadata parity only where the final contract allows |
| BL-214 | **Planned** | Add wire-amplification and useful-byte efficiency metrics | Record response bytes versus useful payload, approximate token cost per useful byte, serialization copies, and hard regression budgets |
| BL-215 | **Planned** | Define profile-specific tool-catalog and initialization budgets | Set Version 1.0 limits for tool count, schema bytes/tokens, descriptions, server instructions, and optional profile composition; maintain `docs/planning/tool-adapters.md` as the required detailed contract for accepted future public names, adapter taxonomy, cross-cutting security/efficiency rules, and owner/milestone parity, without advertising unimplemented tools |
| BL-216 | **Planned** | Add compact profile-specific server instructions | Guide clients to batch, paginate, request fields/ranges, use dry-run, avoid redundant stat calls, and resume cursor output within a bounded instruction budget; implement profile-conditioned instructions only after BL-103/110 effective-profile/catalog contracts exist |
| BL-217 | **Planned** | Add conditional read and not-modified contracts | Post-1.0 conditional retrieval accepts a caller-known content identity/fingerprint/snapshot for files, ranges, lists, searches, system facts, and process output and may return compact `not_modified` metadata without retransmitting unchanged payload. Every request re-runs current root/profile/capability/principal/path authorization; knowledge of an identity never grants access. Any server-side identity/content cache is optional, bounded, execution-context scoped, invalidated safely, and semantically transparent when disabled. Provide an explicit refresh/fresh-proof path that does not substitute cached proof where current bytes/state are required, and measure avoided physical reads/result bytes/model-visible payload without exposing private cache internals. |
| BL-218 | **Planned** | Add opaque large-result and resource-handoff abstraction | Principal-bound `flashgate://` handles, MIME/size/hash metadata, TTL, streaming or paging, negotiated resource links, and bounded inline fallback without host-path leakage; execute after BL-090 and BL-213; consume BL-090 result storage/lifecycle for negotiated MCP resource handoff; do not expose host paths or create another store |
| BL-219 | **Planned** | Define deterministic catalog fingerprints and cache semantics | Stable tool ordering, exact-revision/profile/config fingerprint, invalidation rules, and revision-specific list-result caching. For `2026-07-28`, bind required `ttlMs`/`cacheScope`; default to non-shared/private caching unless the complete result is proven principal/profile-independent; implement only after BL-103/110/239 effective profile, catalog and state-binding contracts; bind fingerprints/cache scope to exact revision and effective principal/profile/config; do not share a result without proof of independence |
| BL-220 | **Planned** | Define native OS-adapter selection and no-interpreter gate | Prefer Go standard library, platform Go adapters, and direct OS APIs. External native programs use a typed no-shell Native Tool adapter with approved executable identity, minimal environment, bounded resources, no raw CLI passthrough, and benchmark/security evidence; interpreter-based adapters are excluded from Version 1.0; this early owner defines the reusable policy/no-interpreter gate, while BL-163 and typed-execution owners later implement and test commands and adapters. |

**MCP `2026-07-28` planning convergence — 2026-09-19.** The specification is final. Open Mobile PRs #60 (BL-207), #69 (BL-208), and #149 (BL-209) are retained only as prepared implementation/decision inputs that predate this convergence. They are not current support authority and must be freshly rebased/reviewed against `main` and the revised BL-207/208/209 acceptance contracts before any later merge. This planning convergence changes no runtime protocol support and does not introduce an MCP SDK dependency.

**Authoritative MCP revision implementation order and support gate.** After the completed `BL-260 -> BL-245 -> BL-203` chain, finish the SPR-048 conformance/schema owners, then the SPR-053 effective-profile/catalog/state-binding fingerprint owner before advertising the target revision. Tasks mapping waits for SPR-049 Operations; each owner follows the explicit hard prerequisites in [architecture traceability](docs/planning/architecture-traceability.md):

1. **BL-207** — define the exact supported-revision matrix and revision-dispatch contract while continuing to advertise only revisions already implemented and validated.
2. **BL-208** — implement the `2026-07-28` stateless adapter path, including per-request metadata, `server/discover`, unsupported-version/result/cache semantics, subscriptions, and exact extension negotiation without weakening the `2025-11-25` initialization path.
3. **BL-204 / BL-212, then BL-219 after SPR-053 context/catalog integration** — complete final-spec conformance/schema coverage, JSON Schema 2020-12 validation, deterministic exact-revision fingerprints, cache invalidation, and safe cache-scope isolation. **FlashGate must not advertise `2026-07-28` as supported until BL-207, BL-208, and these required gates pass.**
4. **BL-209 / BL-211, then BL-210 after SPR-049 BL-090 if Tasks is selected** — decide final Tasks support and bounded no-Tasks fallback; map eligible internal Operations/Jobs only after their lifecycle exists.

The open prepared PRs #60, #69, and #149 remain inputs to these later tasks only; each must be freshly rebound/rebased and reviewed against the then-current `main` and this authoritative order before integration.

### Native multi-mode runtime and local service deployment epic

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-221 | **Planned** | Define native multi-mode runtime architecture and threat model | One Windows PE/Linux ELF binary; explicit current/planned boundaries; no interpreter, remote listener, or implicit privilege escalation |
| BL-222 | **Planned** | Preserve one self-contained binary across runtime modes | Shared core and executable for `stdio`, `proxy`, `auto`, system service, and user-scoped host; split only through a separate evidence-backed ADR |
| BL-223 | **Planned** | Define CLI mode and lifecycle contract | Preserve no-argument STDIO compatibility; specify `--mode stdio`, `--mode proxy`, `--mode auto`, `--mode service`, management commands, exit codes, and shutdown behavior. Define the authoritative owner and expected lifetime for direct STDIO, proxy, auto direct fallback, persistent service, future user host, and future worker roles; signal precedence and typed shutdown/exit reasons; session-scoped edge versus persistent OS-service behavior; and an explicit prohibition on idle-timeout or singleton inference. |
| BL-224 | **Planned** | Separate MCP/core runtime from transport and host lifecycle | Transport-neutral server/core wiring with exactly one process-root lifecycle coordinator and no business logic in STDIO, IPC, SCM, or systemd adapters. Transport/host adapters report lifecycle signals but do not perform domain cleanup; root cancellation invokes Operations/Job and Managed Process cleanup through their respective owners, while platform-specific owner monitoring remains behind adapters. |
| BL-225 | **Planned** | Define versioned local IPC protocol and compatibility handshake | Framing, protocol version, feature negotiation, correlation, cancellation, errors, limits, disconnects, and proxy/service version mismatch behavior. Bind every client to a connection/session ownership ID; cancel partial connection-owned work on disconnect; optionally negotiate versioned lease/heartbeat semantics only for FlashGate-controlled proxy-service or broker-worker channels; keep the persistent service alive after a normal client disconnect; and require no proprietary MCP heartbeat for direct STDIO. |
| BL-226 | **Planned** | Implement Windows Named Pipe transport | Local-only pipe, restrictive ACLs, caller identity from the OS, bounded framing, cancellation, and no trust in proxy-supplied identity |
| BL-227 | **Planned** | Implement Linux Unix Domain Socket transport | Local-only socket, restrictive ownership/mode, peer UID/GID/PID credentials, bounded framing, cleanup, and stale-socket handling |
| BL-228 | **Planned** | Implement STDIO proxy mode | Present normal MCP STDIO to the client and forward safely to the local service without corrupting stdout or changing public tool contracts |
| BL-229 | **Planned** | Implement automatic service discovery and safe STDIO fallback | Prefer explicitly configured/system/user endpoints; no elevation or installation; fallback only when no managed endpoint is present, never after authorization, policy, or compatibility rejection |
| BL-230 | **Planned** | Implement Windows SCM service host, identity, and management | Real Windows Service Control Manager lifecycle, restricted service identity, install/uninstall/start/stop/status, graceful shutdown, and recovery policy. Register the user-facing SCM display name exactly as `FlashGate MCP`; the binding spelling uses uppercase `F` and `G`, one space between `FlashGate` and `MCP`, and all-uppercase `MCP`. Keep the executable filename `flashgate-mcp.exe` and explicitly distinguish the SCM service name, SCM display name, SCM service description, executable image name, PE `ProductName`, PE `FileDescription`, and MCP `serverInfo.name` (`flashgate`). Do not unintentionally change the Go module, repository name, MCP implementation name, configuration names, endpoint or protocol identifiers, installation paths, or release artifact names. Install the SCM service only from a controlled Windows artifact carrying the existing canonical product metadata. Validate the visible and technical identities in Services, PowerShell/SCM queries, and the relevant Task Manager views, and document which Windows surface displays which identity. Reuse the completed BL-246 metadata gates; do not rename technical identifiers unless separately required and approved. **Pre-implementation contract complete:** PR #32 merged the SCM identity/documentation contract and PR #33 merged the post-BL-230 governance registration. Those documentation and coordination completions did not implement the Windows service host, lifecycle, restricted identity, management commands, recovery policy, or platform validation; BL-230 therefore remains `Planned`. |
| BL-231 | **Planned** | Implement Linux systemd system service | Unit, dedicated restricted account, Unix socket/runtime directories, journald, hardening directives, install/uninstall/start/stop/status, and graceful shutdown |
| BL-232 | **Planned** | Add user-scoped background modes | Post-1.0 Linux `systemd --user` service and Windows per-user host; direct STDIO remains the non-admin Version 1.0 path |
| BL-233 | **Planned** | Define configuration precedence, endpoint discovery, and log destinations | CLI/environment/config precedence, system/user paths, endpoint names, timeouts, fallback policy, stdout purity, journald/Event Log/user logs, and secret-safe diagnostics; own one configuration and precedence model consumed by direct, service, and proxy modes; migrate current environment-only configuration |
| BL-234 | **Planned** | Enforce service-side authorization, identity dispatch, and policy | OS-derived caller identity, user/group mapping, roots, profiles, capabilities, Variant A backend selection, per-principal limits, audit, least privilege, and no local privilege-escalation path |
| BL-235 | **Planned** | Adopt hybrid per-root service execution identity | Version 1.0 uses service-account roots; user-worker roots are architected now and implemented later; in-process impersonation is permanently excluded |
| BL-236 | **Planned** | Implement backend-neutral execution-identity interfaces | Separate authenticated caller, policy decision, effective identity backend, operation dispatch, and OS adapter so Variant B can be added without changing domain/MCP contracts; retrofit current minimal handler/request context and direct MCP-to-filesystem wiring to a backend-neutral application/execution-identity boundary |
| BL-237 | **Planned** | Implement Variant A service-account root backend | Dedicated least-privilege account, explicitly ACL-granted roots, no LocalSystem/root convenience default, deterministic denial, and dual caller/effective-identity audit fields |
| BL-238 | **Planned** | Define Variant B user-worker contract and threat model | Specify worker launch/token or UID model, same-binary internal worker mode, broker IPC, environment/groups, lifecycle, quotas, crash recovery, and Windows/Linux differences without implementing it in Version 1.0 |
| BL-239 | **Planned** | Bind state, caches, and result resources to execution context | Bind principal, groups, profile, root, backend, service instance/generation, protocol context, and expiry; prohibit cross-principal cache/handle reuse; apply one reusable context-binding rule to handles, cursors, resources, and caches; prohibit cross-principal/root/profile/backend/generation reuse |
| BL-240 | **Planned** | Implement Variant B per-user worker backend | Post-1.0 broker-managed worker processes under the real user identity with OS resource isolation, native audit attribution, and no shared-process impersonation |
| BL-241 | **Planned** | Add multi-client, lifecycle, compatibility, and denial tests | Retain the independent pre-existing acceptance scope for concurrent clients; disconnects; service and host restart; stale transport endpoints; shutdown; general IPC, proxy, and service version mismatch; unauthorized access; and fail-closed `auto` behavior. Add, without substituting for any of those cases, the integrated Windows/Linux host-lifecycle matrix: normal STDIN EOF and transport close; broken pipe; client crash/kill; verified owner death; retained or duplicated pipe handle; long legitimate idle without false positive; OS stop/signal; optional lease expiry plus lease reconnect/version mismatch where relevant; proxy death cleanup of connection-owned service state while the persistent service survives normal client disconnect; bounded Operations/Job cleanup; bounded Managed Child cleanup; PID reuse; stale instance/runtime-registry state; repeated parallel starts/closes; and return to the documented Direct and Service process baselines. After conclusive owner/transport loss and the bounded shutdown window, require zero remaining `DEFINITELY_ORPHANED` session-scoped hosts or owned children. |
| BL-242 | **Planned** | Add Windows/Linux CI and release validation for all modes | Build native artifacts, test STDIO/proxy/service adapters where CI permits, verify no interpreter dependency, validate service assets, and retain existing gates |
| BL-243 | **Planned** | Document installation, removal, operation, and non-admin deployment | System service, user service/host, portable STDIO, proxy/auto configuration, troubleshooting, rollback, permissions, and explicit current-versus-planned status |
| BL-244 | **Planned** | Benchmark direct, proxy, and service modes and define release gate | Startup, steady-state latency, memory, CPU, payload overhead, concurrency, and evidence-based acceptance thresholds before recommending managed mode broadly |

### CI, release, and quality epic

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-245 | Completed | Define product versioning, release notes, and tag-based release workflow | Establish one canonical repository SemVer source consumed by build/CLI/native metadata/artifacts; before 1.0 new externally observable product capability increments minor and compatible fixes increment patch, while docs/tests/planning/behavior-neutral refactors do not bump; from 1.0 use normal SemVer; functional version bump is part of the same merge, release tag `v<VERSION>` must match, and changelog/release notes remain coordinated |
| BL-246 | Completed | Embed native Windows file and product metadata | Generate and embed deterministic Windows `VERSIONINFO` and the FlashGate application icon in x64 and ARM64 binaries; expose product name, description, company, copyright, original filename, internal name, numeric file version, and full SemVer product version from the canonical build-information source; validate PE architecture, Explorer properties, exact release-archive contents, checksums, reproducibility, and host/credential leak absence. |
| BL-247 | Completed | Define and publish native Linux binary and package metadata | Expose the shared canonical identity through compact/verbose CLI output, Go build/VCS information, ELF headers and Go build IDs for native x64 and cross-built ARM64; publish deterministic TAR.GZ artifacts with exact contents and checksums; keep extended attributes non-authoritative and defer `.deb`, `.rpm`, and systemd metadata until those distribution assets are approved. |
| BL-248 | Completed | Add artifact verification | Reuse the canonical `internal/version`, build, manifest, icon, releaseaudit, and platform-verifier paths; validate real Windows/Linux x64 and cross-built ARM64 binaries and ZIP/TAR.GZ packages, including version/help, product/platform/architecture metadata, exact inventories, checksums, two-build reproducibility, leak scans, structured results, and focused negative cases. |
| BL-249 | **Planned** | Run benchmark suite in CI | Stable selection and artifacted results |
| BL-250 | **Planned** | Compare benchmark baselines in CI | Budgets from `BL-199` |
| BL-251 | Completed | Validate PowerShell and Bash scripts | Completed deterministic Windows and native Linux validation for PowerShell and Bash scripts, including parser, syntax, encoding, line-ending, shebang, bounded-process, and cleanup checks. |
| BL-252 | **Planned** | Run race detector for stateful components | Execute Go race detection against jobs, process registry, output buffers, cancellation, and shutdown; provide the reusable race-test command and failure gate consumed by CI tasks such as `BL-254` |
| BL-253 | **Planned** | Add Windows/Linux process CI jobs | Dedicated CI matrix for process observation and managed lifecycle behavior on supported Windows and Linux runners; reuse implementation tests from the process packages rather than redefining them |
| BL-254 | **Planned** | Add Operations/Job CI jobs | Dedicated CI execution for the Operations/Job integration suite from `BL-098`, including cancellation, timeout, cleanup, leak checks, and the race gate from `BL-252`; this task owns workflow orchestration, not duplicate test implementation |
| BL-255 | **Planned** | Build and verify current-version candidate artifacts | Before the next merge that changes root `VERSION`, make CI build controlled Windows/Linux x64/ARM64 candidate artifacts from that exact repository version, reuse the completed BL-248 artifact-validation contract for metadata/inventory/checksum/reproducibility/leak checks, verify canonical archive/summary names, and upload transient verification artifacts. This is per-version candidate evidence, not public release publication; after BL-248 verifies the candidate, support local pre-release promotion of exactly those bytes without rebuild; bind immutable `VERSION + SourceCommitSHA + ArtifactSHA256`, allow only an optional mutable `current` alias, reject different bytes under one identity, and create no additional upload, tag, GitHub Release, or public-release claim during local promotion; the earlier transient CI verification-artifact upload remains part of candidate evidence |
| BL-256 | **Planned** | Enforce profile-specific catalog and initialization budgets | Execute after BL-215 budgets and BL-216/219 effective instructions and fingerprints exist; enforce `tools/list`, tool count, schema bytes/tokens, server instructions, deterministic ordering, and fingerprint regression in CI before later catalog work relies on them. |
| BL-257 | **Planned** | Run schema snapshot checks in CI | Execute after BL-204 and BL-212 provide the conformance/schema contracts; require explicit review for contract changes and fail CI on unaccepted snapshot drift before later implementation relies on the schemas; enforce accepted BL-204/212 schema snapshots only after both owner contracts are ready |
| BL-258 | **Planned** | Run payload and response-efficiency tests in CI | Execute after BL-213/214 contracts and BL-218 resource handoff are implemented; prevent unbounded contracts, duplicate heavy payloads, excessive wire amplification, and result-resource regressions before later implementation relies on them; enforce BL-213/214 single-transmission and wire-efficiency contracts, including retrofitted heavy results |
| BL-259 | **Planned** | Search repository for legacy names after `SPR-042` | Allow only migration/history exceptions |
| BL-260 | Completed | Keep standard test/vet/lint/build gates and raise product-code coverage | The runner derives the current module's transitive `./cmd/server` package closure, records a sorted per-platform inventory, and gates the unrounded product statement ratio. The active Windows and Linux CI minima are 90.0%. The project target remains at least 95.0%, with 100% pursued where meaningful for bounded core logic. |
| BL-261 | **Planned** | Add reproducible cross-project efficiency benchmark | Compare pinned FlashGate, official Node.js filesystem, selected native Rust filesystem, and selected Go filesystem servers on identical host/corpus/workflows without claiming unmeasured superiority |
| BL-262 | **Planned** | Add native release supply-chain evidence | Checksums, Windows signing plan, Linux artifact/package signing plan, SBOM, build provenance, dependency inventory, reproducible-build comparison, and atomic rollback; no silent auto-update; execute after BL-255 local candidate/promotion foundation and before BL-363 public prerelease publication; retain provenance and rollback controls. Own evidence, not publication; BL-363 and later BL-263 consume it without duplicating its authority |
| BL-363 | **Planned** | Publish verified pre-1.0 GitHub prereleases | SPR-048 release/validation owner; hard prerequisites BL-245, BL-248, BL-255, BL-262. Publish persistent public GitHub Releases only for canonical versions <1.0.0 with mandatory `prerelease=true`, through an explicit safe publication boundary, never automatically on each normal main merge. Consume only BL-255/BL-248-verified Windows/Linux x64/ARM64 ZIP/TAR.GZ archives and sibling checksums; no publication-time build, rebuild, repackaging, or byte mutation. Bind immutable `VERSION + SourceCommitSHA + ArtifactSHA256`; enforce BL-245 version/tag parity with `v<VERSION>` pointing exactly to the candidate SourceCommitSHA. Finalize release-relevant source bytes, including VERSION and the dated CHANGELOG version section, before freezing a candidate intended for public publication; identity-relevant edits require a newly built and verified candidate. Reject identity/hash/tag/asset/permission drift, unverified candidates, missing required BL-262 evidence, and different bytes under one identity fail-closed; never silently replace, overwrite, or delete published assets. Consume BL-262 supply-chain/rollback evidence and carry or unambiguously bind its public-distribution evidence artifacts. Derive notes solely from CHANGELOG.md; ordinary 0.x.y remains unsuffixed, and only an intentionally separate canonical candidate version may use a SemVer prerelease suffix; the publisher invents no version. Use a separate minimally privileged release-write path without broadening build/verification permissions, `write-all`, unnecessary issues/PR/admin rights, or an assumed new long-lived credential mechanism. Bind each remote write attempt and verify remote release identity, exact tag target, prerelease flag, evidence, asset inventory and hashes by readback; ambiguous or failed writes require reconciliation before a separately authorized attempt, never a blind retry. Add focused positive/negative tests and public download/prerelease guidance distinguishing planned from available releases. Retrofit current tag-gated validation and transient Actions-artifact handoff using the existing implementation-contract family and traceability row. Prerelease publication implies no stable/latest claim and does not satisfy the stable Version 1.0 gate; BL-263 remains its sole gate and stable-publication owner |
| BL-263 | **Planned** | Define and enforce Version 1.0 release boundary | Verify every task assigned to Version 1.0 by the sprint sequence or documented waiver, stable protocol/tool contracts, migration/deprecation policy, Variant A-only service identity, performance/security budgets, supported platforms, and post-1.0 deferrals; only after the stable Version 1.0 gate passes, publish the stable persistent GitHub Release `v<VERSION>` assets using the already validated Windows/Linux archives, checksums, canonical changelog notes, and available BL-262 supply-chain evidence. Reuse the safe BL-363 publication foundation; a successful pre-1.0 prerelease does not satisfy this gate. Transient GitHub Actions artifacts alone do not satisfy public distribution; require architecture-traceability closure with no ownerless Version 1.0 requirement or unresolved retrofit except an explicit approved waiver |

BL-255 local implementation uses the candidate workflow and `releaseaudit` record,
manifest, and promotion commands. Its status remains **Planned** until the real
hosted four-target candidate run and independent terminal review pass. BL-262
owns supply-chain and rollback evidence, BL-363 owns public pre-1.0
distribution, and BL-263 owns the stable Version 1.0 gate. No existing public
BL-255 contract needs migration.


BL-251, BL-324, and BL-333 through BL-335 remain completed historical work. Their validation and integration records are retained as provenance, without creating a current private infrastructure dependency.


### SPR-042 technical identity

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-264 | Completed | Rename local folder to `flashgate-mcp` | Manually completed before `SPR-042` implementation |
| BL-265 | Completed | Rename GitHub repository to `flashgate-mcp` | Manually completed before `SPR-042` implementation |
| BL-266 | Completed | Update Git remote URL | Remote verified with fetch and redirect checks |
| BL-267 | Completed | Update Go module and imports | Current module `github.com/thomasweidner/flashgate-mcp`; the former owner path is retained only in historical migration records |
| BL-268 | Completed | Rename binary to `flashgate-mcp` | Windows/Linux build and usage updated |
| BL-269 | Completed | Change MCP server implementation name (`serverInfo.name`) to `flashgate` | Initialize response and smoke tests updated |
| BL-270 | Completed | Review package and command paths | `cmd/server` retained as a generic internal command path |
| BL-271 | Completed | Update README, changelog, and documentation names | Migration/history context preserved |
| BL-272 | Completed | Update PowerShell and Bash scripts | Paths, errors, examples, and smoke expectations updated |
| BL-273 | Completed | Update CI and release artifact names | Workflows updated without unrelated modernization |
| BL-274 | Completed | Update installation and configuration examples | New folder and binary names documented |
| BL-275 | Completed | Update smoke tests | Implementation name (`serverInfo.name`), binary, paths, and current tool names validated |
| BL-276 | Completed | Search all files for legacy names | Remaining occurrences classified as historical or migration guidance |
| BL-277 | Completed | Write technical rename migration note | Old/new repository, module, binary, implementation name (`serverInfo.name`), local folder |
| BL-278 | Completed | Verify GitHub redirect behavior | New path, remote, fetch, and legacy URL redirect verified |
| BL-279 | Completed | Document manual repository rename action | Completed repository-rename procedure retained in external execution evidence; current consumer identity and clone guidance is in `docs/migration.md`. |
| BL-280 | Completed | Keep rename sprint functionally neutral | No feature or tool-contract changes mixed in |

### SPR-043 pre-1.0 tool contract cleanup

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-281 | Completed | Rename `list_files` to `list_directory` | Code, closed schema, runtime validation, tests, docs, catalog, examples, and smoke updated |
| BL-282 | Completed | Rename `stat_path` to `get_path_info` | Single-stat existing/missing contract implemented without exposing host paths |
| BL-283 | Completed | Rename `mkdir` to `create_directory` | Parent creation retained; `created` now reflects the actual leaf state |
| BL-284 | Completed | Remove `exists_path` | MCP tool and redundant core method removed; no compatibility alias |
| BL-285 | Completed | Remove `rename_path` | MCP tool and redundant core alias removed; `move_path` covers rename and movement |
| BL-286 | Completed | Define `move_path` rename/move semantics | Same-path/SameFile, overwrite type combinations, same-volume/cross-volume, Windows case aliases, hardlinks, and self-subtree covered; no copy/delete fallback |
| BL-287 | Completed | Define missing-path `get_path_info` result | Genuine missing returns `{ "path": ..., "exists": false }`; all policy denials remain errors |
| BL-288 | Completed | Define normalized filesystem error codes | Safe sprint-local categories map expected failures to `-32602` and unexpected I/O to `-32603`; stable wire objects remain later work |
| BL-289 | Completed | Review input/output schemas and required fields | Strict object/EOF/unknown-field validation, non-blank paths, optional list path, and `maxBytes >= 1` implemented |
| BL-290 | Completed | Optimize tool descriptions | Titles exposed via shared definitions; descriptions are compact and `copy_path` is explicitly file-only |
| BL-291 | Completed | Update JSON schema snapshots and unit tests | Runtime/catalog contract test covers names, titles, descriptions, required/property fields, and `additionalProperties` |
| BL-292 | Completed | Update smoke tests and MCP `tools/call` tests | Registry/router/call plus default, read-only, negative, Existing/Missing, and Move-as-Rename smoke contracts updated |
| BL-293 | Completed | Update tool docs, client examples, and catalog | README, current architecture/security/testing docs, tool docs, conventions, catalog, ADR amendments, and migration coordinated |
| BL-294 | Completed | Document breaking changes in changelog | Breaking pre-1.0 cleanup documented with no alias or artificial deprecation compatibility |

### SPR-044 Codex read-only activation preparation

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-295 | Completed | Add Codex read-only configuration example | Prepared, not applied: verified binary, implementation name (`serverInfo.name`), cleaned tools, explicit root/read-only environment, and unconfirmed fields clearly marked |
| BL-296 | Completed | Add Claude Desktop configuration example | Prepared Windows-oriented example plus Linux perspective with renamed artifact; no client changed |
| BL-297 | Completed | Add general MCP client examples | Minimal local STDIO contract uses absolute binary/root and explicit read-only/CWD policy |
| BL-298 | Completed | Add read-only troubleshooting guide | Safe categories cover root, permissions/policy, JSON-RPC, binary, and profile/tool-list issues without raw host details |
| BL-299 | Completed | Create activation checklist | Covers commit/binary/hash, root, read-only profile, exact tools/list, positive/negative smokes, backup, acceptance, and rollback |
| BL-300 | Completed | Update read-only smoke test | Exact three read-only tools; all five write and five legacy names negative; Windows/Ubuntu startup smokes verify stdout/stderr, root failures, and cleanup |
| BL-301 | Completed | Validate new documentation paths and links | Current FlashGate paths used; legacy installation paths remain only in immutable migration/history context |
| BL-302 | Completed | Document non-developer read-only validation | Script-based binary/hash/root/tool-list/negative/rollback checklist requires no Go build at client start |
| BL-303 | Completed | Keep activation external to preparation sprint | No real Codex configuration, MCP entry, or auth file changed; activation remains a separately confirmed post-merge step |

### Documentation and client compatibility epic

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-304 | Completed | Link planning and history from README | Backlog, roadmap, changelog, architecture, security, code coverage, and ADRs |
| BL-305 | **Planned** | Keep CHANGELOG current at every backlog-item completion | Continuous completion obligation. At each BL closure, disposition notable Added/Changed/Fixed/Security, breaking, CI, coverage, and release-note impact. Every `VERSION` change must create or update exactly one nonempty `## [<VERSION>]` section in the same BL; version-specific notes must not remain only under `[Unreleased]`. An introduced product version may use an undated section while no candidate is frozen for public publication. Before freezing a public candidate, finalize its release notes and date as `## [<VERSION>] - YYYY-MM-DD` in the canonical candidate source commit. Publication verifies that dated section together with the exact `v<VERSION>` tag; it does not add or change the date after candidate freeze. Later identity-relevant notes or date changes require a new candidate build and renewed verification. The initial reconciliation establishes explicit `0.1.0`, `0.2.0`, and `0.3.0` sections; preserve that version history on later updates. |
| BL-306 | **Planned** | Keep BACKLOG current at every backlog-item completion | Continuous completion obligation. Update the owning BL status in the same closure change, preserve stable IDs/sprint/milestone history, and require complete acceptance or an explicit successor handoff: every deferred deliverable must name one existing successor BL and also appear in that successor's acceptance scope. A predecessor-only deferral is insufficient; completed owners are not reopened merely to execute correctly transferred successor work. |
| BL-307 | **Planned** | Maintain FlashGate project identity reference | Name, tagline, scope, transition, planned identifiers |
| BL-308 | **Planned** | Maintain architecture and ADRs | Current/target/planned/deferred separation |
| BL-309 | **Planned** | Document benchmark method and baselines | `SPR-045` documents the tool-result-contract subset and single-machine noise limits; broader benchmark documentation remains planned |
| BL-310 | **Planned** | Document capabilities, profiles, and named roots | Configuration and security model |
| BL-311 | **Planned** | Document Operations/Job Manager | Handles, states, limits, lifecycle, cleanup |
| BL-312 | **Planned** | Document process and execution security | Handles, PIDs, allowlists, isolation, redaction |
| BL-313 | **Planned** | Document external module/provider ecosystem | Post-1.0 provider contract, runtime, security classification, distribution, support, and separation from negotiated MCP extensions |
| BL-314 | **Planned** | Maintain non-developer smoke-test documentation | PowerShell/Bash and expected results |
| BL-315 | **Planned** | Review documentation and closure parity at every backlog-item completion | Continuous semantic completion review. Before an owning BL becomes `Completed`, verify affected README/CHANGELOG/BACKLOG/release-scope and all directly affected architecture, roadmap, specification, security, testing, coverage, migration, tool/protocol, and ADR material; explicitly disposition `VERSION`, changelog section, tests/CI, current-version candidate artifact, release/distribution, migration, and security impact; require every intentional residual to be dual-bound to its concrete successor BL. Sprint-level parity review remains an aggregate backstop. Maintained documentation uses English prose and stable topic paths, covers every first-party Markdown file in naming/language/link checks, and keeps personal execution evidence outside the public tree; maintain implementation-contract and architecture-traceability parity for new/materially changed technical BLs; plan machine checks for missing family coverage, ownerless rows, invalid references, and Completed items with unresolved retrofit not dual-bound to a successor |

`BL-305`, `BL-306`, and `BL-315` are continuous obligations from this point forward. Their final task status remains open until Version 1.0 closure, but each completed BL must consume their applicable checks immediately.

### Benchmark validation follow-up

These tasks originate in the independent review of PR #15. They are intentionally not implemented by the blocker-fix commit and are scheduled as separate work after PR #15 is merged.

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-316 | Completed | Make the benchmark artifact validator authoritative and schema-strict | **Completed:** the validator strictly decodes each Windows/Linux artifact and canonical budget/workflow source, rejects unknown, duplicate, missing, mistyped, null, and trailing JSON content, enforces the typed schema invariants, independently recomputes hard and soft budgets, and compares the complete result exactly with embedded `budget_evaluation`. Canonical hard/soft key sets are complete, every required soft limit is positive, recomputed hard failures remain fatal, and matching soft warnings remain review-only through the full platform path. Runner result construction keeps budget messages exclusively in `budget_evaluation`; general process/stderr warnings remain separate and fatal. Cross-platform comparison follows independent artifact validation. End-to-end mutations cover runner-produced single-/dual-platform soft results, general warnings, identical dual-platform attacks, stale/fabricated evaluation, matching hard results, invalid soft definitions, and structural attacks. **Scope:** BL-317 through BL-323 remain unchanged; |
| BL-317 | **Planned** | Gate deterministic workflow semantics independently of output size | **Origin/severity:** independent review of PR #15, Major. **Components:** `benchmarks/workflows.json`, runner workflow checks, budgets and artifact tests. **Risk:** truncated or missing output can look more efficient and still satisfy byte/counter ceilings. **Acceptance:** validate every deterministic minimum/exact contract, including `expected_read_bytes` and `expected_entries`, reject absent or reduced useful output, and add negative artifacts proving output loss fails. **Timing:** separate work after merge of PR #15; not fixed by the blocker change. |
| BL-318 | **Planned** | Require an isolated explicit corpus parent for authoritative runs | **Origin/severity:** independent review of PR #15, Major. **Components:** authoritative benchmark controller, `scripts/benchmark*.ps1`, `scripts/benchmark*.sh`, benchmark workspace policy. **Risk:** an implicitly chosen temporary corpus can leave the measured filesystem, synchronization, and storage provenance uncontrolled. **Acceptance:** require and validate an explicit corpus parent below the fixed local Windows benchmark workspace and, for native Linux, native ext4 below `/home`; reject reparse, mounted Windows, synchronized, network, or unresolved parents fail-closed. **Timing:** separate work after merge of PR #15; not fixed by the blocker change. |
| BL-319 | **Planned** | Record verifiable benchmark build and host provenance | **Origin/severity:** independent review of PR #15, Major. **Components:** baseline schema, authoritative controller, build preparation, host gate and reports. **Risk:** committed measurements cannot be tied cryptographically to the measured binary, source/build inputs, controller, workspace, or accepted host-load interval. **Acceptance:** record and validate binary, source/build-input, controller and workspace identities/hashes plus preparation and measurement timestamps, authoritative preflight evidence, and final host-gate evidence; reject incomplete or mismatched provenance. **Timing:** separate work after merge of PR #15; not fixed by the blocker change. |
| BL-320 | **Planned** | Add native Linux race and cross-platform benchmark-policy CI gates | **Origin/severity:** independent review of PR #15, Major. **Components:** GitHub Actions, Windows/Linux policy scripts, window tests, native Linux `go test -race`. **Risk:** platform-specific symlink, reparse, policy-window, and race regressions may merge without executing the relevant native coverage. **Acceptance:** CI runs native Linux race coverage, Windows and Linux output/baseline policy tests, and both measurement-window suites with deterministic pass/fail artifacts; documented exceptions must fail the release gate rather than silently skip required coverage. **Timing:** separate work after merge of PR #15; not fixed by the blocker change. |
| BL-321 | **Planned** | Derive Linux clock ticks instead of assuming 100 Hz | **Origin/severity:** independent review of PR #15, Minor. **Components:** Linux process/resource collector in `internal/benchmark`. **Risk:** CPU-time values are wrong on systems whose `_SC_CLK_TCK` differs from 100. **Acceptance:** obtain the platform value through a supported native mechanism without a shell, handle lookup failure explicitly, and test conversion with non-100 values. **Timing:** separate work after merge of PR #15; not fixed by the blocker change. |
| BL-322 | **Planned** | Reconcile BL-190 and BL-198 final status documentation | **Origin/severity:** independent review of PR #15, Minor. **Components:** `BACKLOG.md`, `SPR-047` report, related benchmark history/current-state documentation. **Risk:** steering state and sprint evidence disagree about whether baseline collection and validation are complete. **Acceptance:** after the authoritative merge decision, establish one canonical current status for BL-190 and BL-198, update current steering documents consistently, and preserve historical evidence and any dated correction outside the active public tree instead of rewriting the original record. **Timing:** separate work after merge of PR #15; not fixed by the blocker change. |
| BL-323 | **Planned** | Correct benchmark coverage claims for copy and search | **Origin/severity:** independent review of PR #15, Minor. **Components:** `docs/testing.md`, benchmark inventory and future filesystem/search benchmark work. **Risk:** testing documentation claims Copy and Search benchmarks that do not yet exist, overstating coverage. **Acceptance:** inventory implemented benchmark cases, make current documentation match that inventory, and describe copy/search only as planned until executable coverage and tests exist. **Timing:** separate work after merge of PR #15; not fixed by the blocker change. |

### GitHub dependency maintenance follow-up

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-324 | Completed | Configure Dependabot security and version updates | Completed Dependabot security and weekly version-update configuration for Go modules and GitHub Actions. |

### Metadata validation follow-up

These tasks originate in the independent security and release review of PR #16. They are intentionally not implemented by the two-Major-finding correction and are scheduled as separate work after PR #16 is merged.

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-325 | **Planned** | Align benchmark JSON Schema and Go representations | **Origin/severity:** independent review of PR #16, Minor. **Components:** `benchmarks/baseline.schema.json`, Go benchmark types, strict decoder, schema-drift tests. **Risk:** `exit_statuses`, numeric representation/range rules, and future nested schema changes can differ between published schema consumers and the Go release gate. **Acceptance:** define equivalent exit-status value constraints and numeric representation/ranges in schema and Go, inventory all current nested required/type/enum/pattern/minimum/additional-properties rules, and add deterministic drift tests proving schema and runtime acceptance remain aligned. **Timing:** separate work after merge of PR #16; not fixed by the Major-finding correction. |
| BL-326 | **Planned** | Reject malformed Unicode in strict benchmark JSON | **Origin/severity:** independent review of PR #16, Minor. **Components:** strict JSON decoder and mutation tests for artifacts, budgets, and workflow catalogs. **Risk:** invalid UTF-8 bytes and unpaired UTF-16 surrogate escapes are silently replaced with U+FFFD instead of being rejected as malformed input. **Acceptance:** reject invalid raw UTF-8 and unpaired surrogate escapes without rejecting legitimate U+FFFD text, retain escaped-property duplicate detection, and add positive/negative Unicode fixtures at relevant nesting levels. **Timing:** separate work after merge of PR #16; not fixed by the Major-finding correction. |
| BL-327 | **Planned** | Make all benchmark hard-failure diagnostics deterministic | **Origin/severity:** independent review of PR #16, Minor. **Components:** hard measurement/budget set validation and artifact diagnostic aggregation. **Risk:** remaining Go-map iteration in measurement failure paths can reorder otherwise identical hard-failure messages across processes, destabilizing CI diagnostics and exact failure signatures. **Acceptance:** deterministically order every missing/unknown hard measurement and budget diagnostic plus final aggregated messages, with multi-error tests across fresh evaluations. The PR #16 Major correction sorts its new soft-definition set errors only and does not claim this task complete. **Timing:** separate work after merge of PR #16; not fixed by the Major-finding correction. |
| BL-328 | **Planned** | Bound strict benchmark JSON resource consumption | **Origin/severity:** independent review of PR #16, Note. **Components:** artifact/budget/catalog file loading and strict JSON shape/typed decoding. **Risk:** repository-controlled oversized JSON is fully read and materialized in an untyped tree before a second typed decode, allowing disproportionate memory/CPU use. **Acceptance:** define justified per-file and relevant collection/string limits, reject oversize inputs before full materialization, preserve duplicate/shape guarantees, and add bounded large/deep negative tests without performance baseline work. **Timing:** separate work after merge of PR #16; not fixed by the Major-finding correction. |
| BL-329 | **Planned** | Bind platform baseline filenames to embedded identity | **Origin/severity:** independent review of PR #16, Note. **Components:** required platform baseline loader and swap/identity tests. **Risk:** complete Windows/Linux file contents can be swapped because the loader reindexes only by embedded `os`, leaving repository filenames mislabeled. **Acceptance:** derive the expected OS/architecture from each fixed filename, compare it directly with embedded identity before map insertion, and add swapped-content negative coverage without remeasuring or modifying baseline data. **Timing:** separate work after merge of PR #16; not fixed by the Major-finding correction. |

### Release validation follow-up

These tasks originate in the final independent review of PR #21. They are accepted as non-blocking Minor follow-ups and were not implemented in PR #21.

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-330 | Completed | Apply the canonical work-item status model to FlashGate | **Origin/severity:** final independent review of PR #21 plus subsequent accepted status convergence, Minor. **Components:** `BACKLOG.md`, directly affected public status/sprint documentation, `scripts/Test-DocumentationConsistency.ps1`, sprint ownership checks, and documentation-quality-gate tests. **Acceptance:** migrate FlashGate human work-item and sprint status to `Planned`, `In Progress`, `Blocked`, `Completed`, `Rejected`, and `Superseded`; map active `Ready` to `Planned`, `Done` to `Completed`, and `Later` to `Planned` while preserving post-Version-1.0 milestone placement separately; use `In Progress` for started partial work; keep technical result/review/authorization/artifact machine states in their existing technical contracts; add positive and negative validation for the documented assignment semantics. FlashGate remains self-contained and must not depend on private governance files or paths. |
| BL-331 | **Planned** | Align ARM64 validation documentation with the implemented runner model | **Origin/severity:** final independent review of PR #21, Minor. **Components:** `docs/adr/product-metadata.md`, build/release metadata documentation, manual validation guidance, and related PR/release wording. **Risk:** one decision passage can be read as evidence that native Windows ARM64 and Ubuntu ARM64 runners already execute tests, while the implemented and validated state is cross-compilation plus static ARM64 validation on x64 hosts. **Acceptance:** clearly distinguish current implementation from target state; document x64-hosted cross-compilation and static ARM64 validation as current behavior, label native ARM64 execution as future/conditional where applicable, and verify consistency across decision, build, testing, and manual-validation documents. **Timing:** separate documentation follow-up after merge of PR #21; no build or CI matrix rerun is required unless implementation behavior changes. |
| BL-332 | **Planned** | Remove contributor-specific path markers from native leak validation | **Origin/severity:** final independent review of PR #21, Minor. **Components:** `scripts/linux-native-driver.sh`, Windows-to-WSL orchestration inputs, leak-scan fixtures, and focused native validation tests. **Risk:** fixed contributor and synchronized-company path markers make local validation host-specific and reduce portability even though they are negative scan values rather than credentials and do not enter release artifacts. **Acceptance:** derive forbidden host/user/synchronized-root values at runtime or pass them explicitly from the orchestrator; keep deterministic generic fixtures for stable regression coverage; prove contributor-specific paths are still detected without embedding personal or organization-specific literals in repository scripts; retain fail-closed leak scanning and add focused Windows/WSL tests. **Timing:** separate local-validation hygiene follow-up after merge of PR #21; release artifacts and product code remain unchanged. |

### Completed repository governance history

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-333 | Completed | Establish change-trigger, review-mode, and handoff governance | Completed historical repository change-trigger, review, and handoff foundation. |
| BL-334 | Completed | Enforce change-trigger, finding-remediation, and handoff governance | Completed historical repository review and handoff validation. |
| BL-335 | Completed | Migrate FlashGate reference-bound legacy Temp objects to local Temp | Completed the historical local temporary-object migration and verified source removal and target parity. No open project finding remains; this migration defines no current contributor setup requirement. |

### Completed repository validation history

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-336 | Completed | Generalize governance handoff contracts and commit-preparation validation | Completed historical repository handoff and commit-preparation validation. |

### Repository validation and host-lifecycle follow-up

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-337 | Completed | Isolate governance fixture execution in one controlled runner process | Completed by supersession under BL-343. The earlier large fixture-runner platform is not part of the current project architecture; no further work is open. |
| BL-338 | Completed | Add canonical governance case metadata and deterministic selection | Completed deterministic case metadata, selection, platform/capability preflight, and repository-only portability coverage. This historical validation machinery is not a public product dependency. |
| BL-339 | Completed | Provide reusable focused and full governance validation orchestration | Completed the historical focused and full validation orchestration. |
| BL-340 | Completed | Complete governance generator/profile migration | Completed the historical generator/profile migration through |
| BL-341 | **Planned** | Implement cross-mode host-process ownership, deterministic shutdown, diagnostics, and orphan prevention | Implement Direct STDIO and proxy/auto-edge lifecycle through one process-root coordinator; platform owner adapters; definitive EOF, transport failure, OS stop, verified owner loss, and explicitly negotiated lease signals; bounded deterministic shutdown; Operations/Job (`BL-094`) and Managed Child (`BL-129`) cleanup through their respective owners; secret-safe instance diagnostics and typed exit classification; PID plus process-start identity or verified OS handle, never PID-only authority; safe stale runtime-registry cleanup; and Windows/Linux behavior. Multiple direct/proxy instances remain legitimate; age, idle time, CPU, request count, or singleton assumptions never authorize termination, and ambiguous live-owner/live-transport cases are `SUSPECTED_STALE`. BL-241 owns complete integrated testability and BL-263 the Version 1.0 release gate. Add no remote listener, interpreter dependency, hidden installation, or automatic elevation. |
| BL-342 | Completed | Bind validation scratch producers to explicit task work roots | Completed explicit task-bound scratch routing for active PowerShell and Python validation producers. Windows and native Linux gates passed with no open project finding; validation output no longer defaults to OS Temp. |
| BL-343 | Completed | Converge FlashGate to the Slim Governance project adapter | Completed the public repository boundary and Slim project adapter. Active product, build, security, shell, and documentation gates remain in place; obsolete private orchestration is not a CI or contributor prerequisite. |
| BL-344 | Completed | Integrate Slim Governance poststate and establish clean local baseline | Completed local integration of the Slim project baseline. The repository remains self-contained for public build, test, release, contribution, and product understanding; no open project finding remains. |


BL-333 through BL-340 and BL-342 through BL-344 remain `Completed`. BL-344 is `Completed`. BL-341 remains `Planned` in SPR-060 for host-process lifecycle implementation; its planning completion does not imply runtime implementation. Historical execution and review evidence is kept outside the active public
documentation tree. Task status is not inferred from retiring a report.

The highest assigned backlog identifier is `BL-363`.

## Cross-epic rules

- Human status describes execution state. The sprint sequence and Post-1.0 workstream table determine release placement independently of status. A task changes milestone only through an explicit backlog and documentation decision.
- Security tasks apply to their domain tasks without duplicating canonical definitions.
- FlashGate public capabilities, wire contracts, documentation, and agent guidance are consumer-independent. They must not require organization-specific governance, private task identifiers, local control-plane paths, a consumer `AGENTS.md`, or a particular agent/orchestrator product. Consumer-specific workflows may compose FlashGate capabilities above the product boundary; historical private-development references are provenance only and have no normative product authority.
- New installations with configured roots but no explicit profile default to the safe read-only profile; higher-risk profiles require explicit activation.
- Payload-heavy content is transferred once. Structured metadata, resource handles, and compatibility fallbacks must not duplicate large file, process, search, or binary payloads.
- Every service request has both an authenticated caller identity and an effective execution backend. Version 1.0 implements the service-account backend only; the user-worker backend is interface-compatible but post-1.0. In-process impersonation is prohibited.
- Handles, jobs, caches, result resources, temporary files, cancellation, and audit correlation are bound to principal, profile, root, execution backend, and service generation.
- Global limits are insufficient for service mode; per-principal quotas and fair scheduling are mandatory.
- Native Go/OS APIs are preferred. External programs require typed no-shell definitions, approved executable identity, minimal environment, bounded resources, no raw CLI passthrough, and evidence; interpreter-based adapters are excluded from Version 1.0.
- Public tool names use `verb_object`; `get` means structured state/metadata, `read` means payload/content, `set` means structured-state mutation, and `write` means payload mutation. Adapter or provider names do not become public tool names. The detailed accepted naming/options/adapter contract is `docs/planning/tool-adapters.md`.
- Portable/default profiles minimize OS/provider/adapter fingerprinting; platform-specific Registry/sysctl/ADS/xattr capabilities are opt-in, but FlashGate never invents false semantic equivalence merely to hide the OS.
- Execution policy must distinguish requested policy from proven enforcement strength; required network/resource/isolation enforcement that the active backend cannot prove fails closed rather than silently degrading.
- Cloud-backed paths never trigger implicit hydration under the default `local_only` policy, and local success in a sync-backed root never implies external synchronization success.
- Operations/jobs are optional lifecycle infrastructure; short synchronous operations may run directly and domain logic/ownership stays outside the manager.
- Benchmarks and threat models must justify separate product binaries, indexes, or external adapters/providers. The same-binary local service IPC accepted by ADR-014 still requires its defined security, compatibility, and benchmark release gates.
- FlashGate modules/providers and MCP protocol extensions are separate concepts and contracts.
- Deprecated MCP Roots is never the foundation of named-root authorization.
- MCP annotations never replace server-side authorization.
- Planned tool cleanup and technical rename occur only in their dedicated sprints.
- Before Version 1.0, breaking changes are allowed but require coordinated tests, documentation, examples, smoke tests, and changelog entries. Version 1.0 requires a documented compatibility, deprecation, and migration policy.
