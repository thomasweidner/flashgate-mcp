# Architecture Traceability

## Authority and dependency semantics

This living matrix maps accepted Version 1.0, Post-1.0, and release-unassigned
architecture requirements to implementation, retrofit, validation, and closure
owners. [BACKLOG.md](../../BACKLOG.md)
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
| Exact revision conformance and schema snapshots (A, R, ADR MCP compatibility) | BL-204 | BL-200, BL-201 | Reuse completed BL-202/207/208; BL-257 consumes snapshots; BL-219 later consumes the revision contract | Runtime registry, `internal/protocol` and `cmd/server` default/read-only wire paths; no unsupported advertisement | [BL-204 binding](implementation-contract.md#bl-204-conformance-binding), native wire/snapshot/completeness mutations and pinned official applicability | Reviewed complete snapshots per revision/profile; official execution limitations explicit; candidate remains disabled until BL-212/219 pass |
| MCP `ping` receiver behavior for `2025-11-25` (A, T, ADR-013; official ping utility) | BL-371 | BL-207, BL-208 | Reuse BL-204 gap evidence and existing router/server/revision protocol contracts; BL-257 remains independently gated by BL-204/212 | Existing server validation and exact-revision router, with one dedicated `ping` handler in the STDIO bootstrap | Official request/result schema; positive and negative ID/params/notification/revision wire tests; Windows/native Linux, focused CI/coverage and review | Completed with verified `2025-11-25` empty-result response and `2026-07-28` method-not-found, unchanged tool catalog, and native conformance evidence |
| JSON Schema 2020-12 validation (A, R, ADR MCP compatibility) | BL-212 | BL-200, BL-201, BL-204 | Reuse BL-289/291 arguments/catalog and BL-262 source inventory; BL-110 extends the profile matrix; BL-257 consumes evidence | BL-204 registry, wire snapshots and report; BL-262 dependency classification | [BL-212 binding](implementation-contract.md#bl-212-schema-conformance-binding) | Offline Draft 2020-12 and instance checks, drift mutations, pinned test-only dependencies, unchanged wire snapshots |
| MCP contract enforcement in CI (T, R) | BL-257 | BL-204, BL-212 | Reuse `mcpcheck`, registry and catalog parity, schema validation, and the owner-maintained conformance matrix | Existing Windows/Linux test job and workflow policy checks | [BL-257 binding](implementation-contract.md#bl-257-ci-enforcement-binding) | Missing executable coverage, failed/skipped checks, or unaccepted contract drift blocks CI; intentional diffs require independent review |
| Typed result contracts and single transmission (A, R, ADR efficiency contracts) | BL-213 | BL-201, BL-204, BL-212, BL-257 | BL-214 metrics and BL-218 handoff consume the contract. BL-090 supplies storage to BL-218 | Tool interface, registry, result adapter, `read_file`, `list_directory`, and six metadata tools | [BL-213 binding](implementation-contract.md#bl-213-tool-result-contract), per-variant wire coverage and existing CI gate | Adapter-derived payload metadata, lossless UTF-8, pre-serialization bounds, bounded metadata parity, and fail-closed registration |
| Useful-byte and wire-amplification metrics (A, T) | BL-214 | BL-213 | Reuse BL-189–BL-199 and BL-204/212/257. BL-258 extends enforcement to resource delivery | Existing MCP conformance harness, serializers and benchmarks | BL-214, BL-258 | Exact executable variant, profile, revision, fixture and budget parity in ordinary Go tests |
| Catalog and entry budgets (A, R, TA) | BL-215 | — | Reuse the available BL-214 wire harness, existing benchmark references and BL-204/207/208/212/213/257 contracts. BL-103/110 compositions, BL-216 instructions, BL-219 metadata/fingerprints, BL-256 comprehensive enforcement, and each later public wire owner consume the budgets | Extend existing catalog, initialize and discover wire gate and planned public inventory | BL-215, then the owning BL | Actual path and field budgets, finite effective composition, owner/milestone parity and fail-closed fixture coverage |
| Native adapter policy and no-interpreter source gate (A, S, T, TA; [binding](implementation-contract.md#bl-220-adapter-acceptance-binding)) | BL-220 | — | Existing Go filesystem, PathGuard, domain interfaces and doccheck/CI. [Successor families](tool-adapters.md#bl-220-successor-acceptance) bind actual owners in BACKLOG.md. | Check current product dependency graphs and exact family applicability against all bound backlog owners and the public inventory. BL-102/107/224/236 retain their runtime retrofits. | Source graph, wrong-family, missing internal acceptance and inventory drift negatives, native Windows/Linux gates and independent review. BL-151/152 add launch-specific coverage and BL-242 adds artifact/mode evidence. | Policy and static evidence delivered early. BL-163 and the Managed Process/typed execution owners implement effective runtime policy. Native Tool, platform and provider owners prove their own activation. |
| Current-version candidate and exact-byte local promotion (R, T) | BL-255 | BL-245, BL-248 | BL-262 later consumes verified identity; `current` is an optional alias | `current-version-candidate.yml`, existing BL-248 verifiers and `cmd/releaseaudit` record/manifest/promotion commands; release workflow naming parity | Focused source, matrix, report-binding and exact-byte tests; local quality gates; hosted four-target run and independent review passed for PR #303 | Verified VERSION/commit/hash bytes promoted unchanged; local step makes no additional remote/public upload |
| Release-oriented product versioning (R, T) | BL-372 | BL-245, BL-248, BL-255, BL-262, BL-363, BL-364 | Reuse VERSION, changelog, build provenance and verified version/source/hash identity | Build scripts, candidate and release workflows, releaseaudit source and candidate validators | [BL-372 binding](implementation-contract.md#bl-372-release-oriented-product-versioning), development and freeze regressions, both platform chains and independent terminal review | Completed. Development is distinct from frozen releases. Candidate and publication controls prevent reuse of published identity. |
| Supply-chain provenance and rollback foundation (A, R, T) | BL-262 | BL-255 | Reuse internal/version, BL-248 hashes/verifier/reproduction/leak checks and BL-255 manifest/promotion; BL-363 consumes pinned portable evidence; BL-263 later verifies release closure | cmd/releaseaudit supply-chain commands and current-version-candidate.yml manifest job; release-build.yml remains tag validation with publication retrofit owned by BL-363 | Generator/verifier determinism, SPDX/module/asset taxonomy, four subjects, exact versioned Candidate-Verification predicate and old-SLSA rejection, trust anchors, collision/path/leak negatives and synthetic prior-identity rollback; revalidate affected platform gates; local and native validation completed; the historical Windows staging-rename diagnostic residual was independently accepted, with its cause remaining unproven; the separate investigation is not a deferred FlashGate product deliverable, and any subsequently proven FlashGate product defect requires a new finding under its appropriate owner; terminal independent review and corrected hosted CI passed; integrated through PR #306 | One unsigned contract-version-1 index, one SPDX 2.3 SBOM and four in-toto Candidate-Verification statements in a separate complete bundle; no builder, SLSA-level, build-attestation, promotion-byte mutation, publication or deployment-atomicity claim |
| Persistent public pre-1.0 GitHub prereleases (A, S, R, T, DEC-FP-002) | BL-363 | BL-245, BL-248, BL-255, BL-262 | Reuse root VERSION, BL-245 CHANGELOG/tag contract, BL-248 verifier, BL-255 candidate identity/verified bytes, and BL-262 supply-chain/rollback evidence; BL-263 later consumes the safe publication foundation | releaseaudit publication commands and separate manual publish-prerelease.yml consume the complete transient evidence bundle; release-build.yml stays read-only; public download guidance is updated; local candidate/evidence stages share bounded Windows promotion with identity-bound revalidation and cleanup | BL-363 mock, identity, permissions, one-shot/rerun, source/notes, safe-path and downloaded-byte readback gates plus applicable platform regression; deterministic stage drift/budget tests, candidate alias rollback and native single-call regression | Completed. Public v0.3.0 and all 40 original public asset bytes passed independent publication acceptance on 2026-10-25. Exact immutable bytes, no rebuild/replacement, no stable/latest claim; BL-263 remains separate |
| Five-asset distribution for all releases, including historical v0.3.0 (A, S, R, T, DEC-FP-002/003; [BL-364 binding](implementation-contract.md#bl-364-planning-binding)) | BL-364 | BL-245, BL-248, BL-255, BL-262, BL-363 | Reuse VERSION/CHANGELOG/tag semantics, internal/version names, BL-248 verifier, BL-255 immutable candidates, BL-262 complete anchored verification and BL-363 publication; BL-263 is a later consumer | releaseaudit inventory/authorization/manifest, SHA256SUMS aggregation and provider digest/readback; manual publish-prerelease.yml consumer; affected download/build/release/security/testing guidance and DEC-FP-003; retain existing evidence producers/verifier | Exact five-asset/four-archive inventory; deterministic GNU/LF four-entry SHA256SUMS; selected-platform-only verification; original/provider-digest/download parity; unchanged identity; complete historical prestate before separately authorized checksum upload and deletion of 36 extras, with one-shot/readback and tampering/drift/unavailable/ambiguous-operation negatives; independent release/security plan review before implementation and later implementation review | Completed SPR-048. Four original Windows/Linux x64/ARM64 archives plus shared SHA256SUMS; exclude GitHub source downloads. No sibling checksum, evidence or extra manifest asset, notes checksum section or permanent release ledger. Complete verification/required temporary evidence within regular Actions retention; no long-term raw-evidence duty. Lightweight tags and original VERSION/source/artifact identity remain. Publisher/consumer retrofit and independent technical review are accepted. Actual v0.3.0 normalization passed independent GitHub identity/digest acceptance and user public-download hash readback: exactly five assets with the four original archive IDs/bytes and unchanged tag/source/prerelease. Preserve true original 40-asset history and distinguish the separately approved cumulative frozen CHANGELOG notes from the original notes identity. No Git/provider/permission/cleanup authority |

BL-363's publication correction retrofits the same adapter and manual workflow:
positive-ID draft readback, explicit continuation of one authorized empty draft
with the original manifest, and credential-free public download verification.
Its acceptance adds hidden-list/direct-ID fixtures, conflicting identities,
nonempty-draft rejection and fresh consumed-approval checks to the row above.
The same adapter retrofit separates upload response Accept from body
Content-Type and retains only typed numeric upload status through the CLI HOLD
boundary. Full-path negatives cover rejected/ambiguous writes, suppressed
provider and close-error text, and unchanged no-retry/redirect controls.
No product rebuild, new dependency, tag change or general recovery engine is
introduced. The first public v0.3.0 release and its original public asset bytes
passed independent acceptance. Public guidance now records the workflow-token
capability condition and a separately authorized manual publish-only boundary
using existing CLI authority; no automatic fallback or permission upgrade is
introduced. Historical failure causes remain unproven; their publication HOLD
is resolved. BL-363 is Completed. SPR-048 and stable BL-263 closure remain separate.

## Core policy, context, catalog, and state foundation

| Requirement / authority | Owner BL | Hard predecessors | Reuse / later consumers | Current retrofit target | Validation / gate | Target closure |
|---|---|---|---|---|---|---|
| [Single-file typed runtime configuration](implementation-contract.md#bl-233-central-runtime-configuration) (A, S, ADR runtime modes) | BL-233 | — | BL-101–BL-110 and future domain owners extend the model; BL-223 consumes the TOML template and schema; BL-228–BL-232, BL-236, BL-370 and BL-373 consume the model | `internal/config` environment loader, `cmd/server` bootstrap and current public `MCP_*` settings | BL-233, BL-241/242 | Trusted role-bound TOML, validation before initialization, immutable effective state, safe diagnostics, no startup writes, documented migration and a complete, valid, documented TOML template |
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
| Managed Process foundations and observation (A, S, ADR process execution) | BL-113–BL-126, BL-129–BL-135, BL-162, BL-165 | BL-090, BL-159, BL-239 | Reuse Operations, context and output bounds. BL-151 consumes the foundations and owns transferred productive acceptance. | No current managed-process domain. Build registry, lifecycle, typed dispatch and native components with launch unavailable. | BL-135 and BL-252–BL-254 fixture or observation coverage. BL-151 runs their real launch cases. | SPR-056 foundations and retained acceptance with a two-sided transfer to BL-151. Stage 1 creates no executable public route. |
| Typed execution policy enforcement (A, S, TA) | BL-163 | BL-159, BL-166, BL-220, BL-236, BL-239 | Typed commands and Native Tool adapters consume one policy | No current command engine. Apply the policy before central Managed Process dispatch. | BL-163, BL-151/152, BL-167–BL-170 | Deny unproved identity, roots, argv, environment, limits or enforcement before launch |
| Typed launch prerequisites (A, S, TA, ADR process execution) | BL-136–BL-141, BL-144–BL-149 | BL-163, BL-220, BL-239 | Extend typed policy, isolation and native components independently of public launch. BL-151 consumes effective controls. | No current command engine | Identity and typed argv, roots, time/output, environment, redaction, isolation and resource controls. | Reviewed execution prerequisites for the unadvertised BL-151 candidate. |
| Productive Managed Process activation ([contract](implementation-contract.md#managed-process-activation)) | BL-151 | BL-094, BL-097, BL-109, BL-110, BL-119–BL-126, BL-129–BL-134, BL-136–BL-141, BL-144–BL-149, BL-163, BL-220, BL-239 | Inherit the authorization, audit and execution context requirements through BL-163. Reuse BL-135 and BL-252–BL-254 evidence infrastructure. BL-143 consumes the activated boundary. | Integrate the BL-122 typed interface and BL-134 native components via one reviewed launch path. Keep product routes unavailable during candidate verification. | Native Windows/Linux start, denial, lifecycle, cleanup, parity, race and CI for the transferred acceptance. Consume BL-152 and BL-168 candidate evidence before activation. | SPR-057 activation only after required controls, tests and review. No second execution engine or raw launch fallback. |
| Typed execution security evidence (S, T, TA) | BL-152 | BL-163, BL-220, BL-239 | Own independent security negatives and run them against the unadvertised BL-151 candidate. BL-168 retains least-privilege evidence. | No current command engine | Native identity, argv, environment, isolation, denial and lifecycle negatives | BL-151 consumes passing evidence before advertisement. The evidence does not block construction of the candidate it tests. |
| Typed result and command discovery contracts (A, S, TA) | BL-142, BL-353 | BL-163, BL-220, BL-239 | Define result schema and authorized discovery without requiring a managed start to implement the contracts. | No current command engine | Schema, authorized exposure and truthful availability | Discover only supported commands and avoid raw executable identity disclosure. |
| Synchronous command wrapper (A, S, TA) | BL-143 | BL-142, BL-151, BL-163, BL-220, BL-239 | Reuse the one activated launch boundary | No current command engine | Synchronous results, matching denial and lifecycle | Productive wrapper over Managed Process. |
| Scoped system information (A, S, TA) | BL-062, BL-153–BL-157 | BL-159, BL-236 | Reuse domain/context/redaction | No current system-information domain | BL-158, BL-241 | Explicit fields and bounded redacted results |
| Direct/proxy/service host modes (A, R, ADR runtime modes/host lifecycle) | BL-221–BL-231, BL-234–BL-235, BL-237–BL-238, BL-341 | BL-166, BL-233, BL-236, BL-239 | One core behind STDIO/local IPC; BL-223 explicit CLI creation using the BL-233 template and shared schema; BL-228/229 endpoint policy; BL-230/231 trusted installation paths; Variant A only for Version 1.0 | Current `cmd/server` direct STDIO | BL-241–BL-244, BL-263 | Local-only modes, trusted peer identity, stdout purity and cleanup |
| User-scoped persistent host (A, S, R, ADR runtime modes) | BL-232 | BL-233 | Reuse native host/lifecycle contracts and load under the executing user's identity | Current direct user STDIO bootstrap; no persistent user host | BL-232 native identity, lifecycle, configuration and orphan tests | Post-1.0 host with the user-role default or an explicitly bound trusted path |
| Multi-client security/lifecycle integration (A, S, T) | BL-241 | BL-090, BL-159, BL-166, BL-239, BL-341 | Validates integrated host and domains | Direct-mode smoke baseline | BL-241, BL-263 | Cross-client, ownership and host-lifecycle evidence |
| Windows/Linux mode release validation (R, T) | BL-242 | BL-241 | Consumes direct/proxy/service artifacts | Platform CI and service asset plans | BL-242, BL-263 | Native mode parity, no interpreter and assets verified |
| Installation/removal/operation documentation (R, T) | BL-243 | BL-241, BL-242 | Documents implemented modes and actual CLI creation, configuration selection, installation and migration paths. BL-255 promotion does not imply service availability. | Current direct-mode guidance | BL-243, BL-263 | Portable user/admin instructions with current/planned status |
| Direct/proxy/service benchmarks (R, T) | BL-244 | BL-241, BL-242 | Consumes modes and measurement baseline | Current direct-mode baseline | BL-244, BL-263 | Evidence-based managed-mode thresholds |
| Benchmark CI execution (R, T) | BL-249 | — | BL-250 consumes suite | Current benchmark runner | BL-249, BL-263 | Stable CI selection and artifacted measurements |
| Benchmark baseline comparison (R, T) | BL-250 | BL-249 | BL-261 consumes comparable evidence | BL-199 budgets and baseline schema | BL-250, BL-263 | CI fails on hard budget regression |
| Cross-project efficiency evidence (R, T) | BL-261 | BL-249, BL-250 | BL-263 consumes report | FlashGate benchmark and pinned peers | BL-261, BL-263 | Same-host/corpus/workflow comparison without unsupported claims |
| Response-size/benchmark integrity follow-up (R, T) | BL-205, BL-317–BL-323, BL-325–BL-329 | — | Reuse schemas, controllers and deterministic corpus | Benchmark loaders, host evidence and docs | Owning BLs, BL-263 | Bounded, deterministic and truthful measurements |
| Public release governance/validation portability (R, T) | BL-172–BL-173, BL-177–BL-179, BL-331–BL-332 | — | Reuse contribution, workflow and artifact gates | Workflow pins, release/security guidance, ARM64/host-path docs | Owning BLs, BL-263 | Portable, truthful release controls |
| Stable Version 1.0 architecture/release closure (R, T) | BL-263 | BL-242, BL-243, BL-244, BL-249, BL-250, BL-261, BL-262, BL-363, BL-364 | Audit every Version 1.0 row plus BL-305/306/308/315 controls; reuse BL-363 safe publication and accepted BL-364 five-asset distribution/temporary-evidence contract | Every retained implementation/public contract in matrix | BL-263, including BL-364 implementation and historical v0.3.0 acceptance | No ownerless requirement or retrofit except approved waiver; stable Version 1.0 publication only after its gate with four original archive assets plus shared SHA256SUMS, original/digest/download parity and required verification evidence within regular Actions retention; BL-363 prerelease success does not satisfy it; release-unassigned BL-366/367 do not block it |


## Release-unassigned human-readable release reporting

BL-369 is a separate Planned presentation owner with no sprint/release
assignment or automatic release-blocking effect. Its production inputs retain
their original authorities; it does not create a second verifier or a sixth
release asset.

| Requirement / authority | Owner BL | Hard predecessors | Reuse / later consumers | Current retrofit target | Validation / gate | Target closure |
|---|---|---|---|---|---|---|
| Readable candidate/release quality report (R, T; [binding](implementation-contract.md#bl-369-human-readable-release-quality-reporting)) | BL-369 | BL-248, BL-255, BL-262 | Reuse BL-204/212/257 MCP, BL-366 advisory, BL-367 attestation when available, existing CI/performance/security, source and published release identities | Existing machine-only/ephemeral CI evidence and summary fields, not a new test or release authority | Fail on incorrect/stale/missing identity and unresolved required evidence; visible pass/warn/fail/not-run/not-applicable/unverified distinctions, redaction, links and deterministic Markdown output | Readable Actions candidate summary and separately authorized optional persistent public version report; no sixth asset, raw-evidence archive, new release blocker or silent publication |

## Future HTTP MCP transport

BL-370 is explicitly Post-Version-1.0. Its later implementation must decide
HTTP-facing architecture and security separately, reuse the conformance test
contract and block activation until every advertised revision/transport pair
passes.

| Requirement / authority | Owner BL | Hard predecessors | Reuse / later consumers | Current retrofit target | Validation / gate | Target closure |
|---|---|---|---|---|---|---|
| Streamable HTTP MCP transport, conformance and activation (A, S, T, R; [binding](implementation-contract.md#bl-370-future-streamable-http-mcp-transport)) | BL-370 | BL-204, BL-212, BL-219, BL-224, BL-233 | Existing Go core, exact revision policy, schema/snapshot tests, BL-239 context binding and protocol/host-lifecycle testing; conditional BL-373 consumer | No current HTTP MCP listener; later direct core adapter and explicit public transport/revision support matrix | Independent ADR/threat-model decision; HTTP security and Windows/Linux negative tests; official server conformance against real HTTP endpoint; automatic coverage completeness and activation denial on drift | Only explicitly authorized HTTP support after every technical and publication gate; administrative exposure separately gated; BL-204 stays closed |

## Future MCP configuration management

| Requirement / authority | Owner BL | Hard predecessors | Reuse / later consumers | Current retrofit target | Validation / gate | Target closure |
|---|---|---|---|---|---|---|
| [Authorized MCP configuration management](implementation-contract.md#bl-373-authorized-mcp-configuration-management) (A, S, T, TA) | BL-373 | BL-110, BL-159, BL-166, BL-233, BL-236 | Extend the one typed model, trusted context, root/profile/capability policy, effective catalog and audit; BL-370 may consume the management API after HTTP gates | Extend BL-233's startup-only configuration and the existing authorized MCP dispatch; no current management API | [BL-220 context and policy](tool-adapters.md#bl-220-context-and-policy); native Windows/Linux security, negatives, conflict/CAS and readback, failure/restart consistency, authorized transport integration | Post-1.0 bounded views and previewed, authorized changes through one core; optional MCP Apps UI uses the same policy |

## Release-unassigned security contracts

These independent owners have no sprint or release assignment and block no release.
BL-366 is Completed. BL-367 is Planned and requires separate implementation
authorization.

| Requirement / authority | Owner BL | Hard predecessors | Reuse / later consumers | Current retrofit target | Validation / gate | Target closure |
|---|---|---|---|---|---|---|
| SBOM security management (A, S, T; [binding](implementation-contract.md#bl-366-sbom-security-management)) | BL-366 | BL-262, BL-324 | Reuse verifySupply, externally anchored SPDX/source inputs and existing Dependabot configuration; later BL-263 can reuse the same assessment CLI | releaseaudit verified reader, bounded OSV snapshot transport, deterministic offline assessment, report/triage consumer; candidate and confirmed-success publisher summaries | Exact module/version and positional/paginated advisory binding; direct/indirect and declared build/runtime/UNKNOWN roles; freshness, aliases, exceptions, bounded/malformed/provider negatives; nonblocking workflow regressions and independent security review | Completed with local implementation, validation, and independent review accepted; no second SBOM generator, dependency, public asset, runtime delta, permissions or credentials; all findings/provider failures informative; no Version 1.0 or other release blocker |
| GitHub Artifact Attestations (A, S, T, DEC-FP-003; [binding](implementation-contract.md#bl-367-github-artifact-attestations)) | BL-367 | BL-248, BL-255, BL-262, BL-363 | Reuse four verified immutable archive identities, Actions workflows and existing unsigned evidence; coordinate with BL-364 without a hard edge | Later authorized release-workflow attestation generation/verification, minimal permissions, negative tests and guidance | GitHub-native OIDC/short-lived-identity signing and verification; subject/source/repository/workflow identity, missing/tampered/swapped/unavailable and excess-permission negatives; independent review | Signed GitHub-native build attestations for four archives, no permanent keys/credentials or extra download asset; no activation/permission change in this plan; no release blocker |

## Release-unassigned documentation tooling

BL-368 is an independent Planned development-tooling owner, with no sprint,
Version 1.0 or Post-1.0 assignment and no release-blocking role. BL-365 remains
the separate onboarding owner in SPR-048; neither is a hard predecessor of the other. Registration
implements no generator, README change or CI behavior.

| Requirement / authority | Owner BL | Hard predecessors | Reuse / later consumers | Current retrofit target | Validation / gate | Target closure |
|---|---|---|---|---|---|---|
| Deterministic README navigation and drift validation (T; [binding](implementation-contract.md#bl-368-documentation-navigation), [documentation gate](../documentation-quality-gate.md)) | BL-368 | None outstanding; existing documentation validation is available | Reuse cmd/doccheck heading/fence/duplicate/explicit-anchor/link rules and fixtures, Test-DocumentationConsistency.ps1, documentation style and testing; accommodate later BL-365 README shape without a hard edge | One explicitly managed README ToC region, reusable local generation command, existing read-only documentation/CI gate and contributor guidance | Normalized/duplicate/Unicode/punctuation headings, GitHub/explicit anchors, fences, links/targets, added/removed/renamed headings, missing/duplicate/malformed markers; stable order, byte-identical idempotence, UTF-8/BOM/LF/CRLF/final-newline preservation and unchanged outside bytes; Windows/Linux/CI parity and read-only drift/invalid-link/unexpected-delta failure | Separately authorized generator and existing-validator/CI/guidance retrofits accepted under the binding's closure dispositions; bounded processing and no competing engine, private dependency, runtime/version/release change or extra GitHub write rights. CI writes require a separate architecture decision. Registration leaves BL-368 Planned and BL-365 unchanged, with no release obligation |

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

### Result delivery successors

The completed [BL-213 result contract](implementation-contract.md#bl-213-tool-result-contract)
defines bounded variants and single transmission. The following owners add the
delivery mechanisms and their acceptance in the backlog.

| Responsibility | Implementation and validation owners |
|---|---|
| Directory paging and file streaming/media | BL-037, BL-044, BL-045 |
| Operation lifecycle, storage and context binding | BL-084–BL-090, BL-239 |
| Resource handoff, subscriptions and bounded fallback | BL-218 |
| Catalog notifications and cache invalidation | BL-219 |
| Tasks, optional task notifications and synchronous fallback | BL-209, BL-210, BL-211 |
| Negotiated event/stream forwarding and transport tests | BL-225, BL-228, BL-241, BL-370 |
| Watch domain events | BL-347 |
| Useful-byte measurement and resource efficiency CI | BL-214, BL-258 |

Each successor integrates the [BL-214 measurement contract](implementation-contract.md#bl-214-wire-measurement-and-coverage)
for the routes or outputs it actually implements. These are acceptance bindings
to the existing sprint and release placements in the backlog. Early or
independently scheduled owners can provide the bindings within their own scope.

| Owner | Measurement acceptance |
|---|---|
| BL-037–BL-039, BL-049, BL-079 | Per-page and complete retrieval, cursor overhead, continuation order, projections and single transmission |
| BL-044, BL-045 | Range bytes, chunks, bounded copies, text/media encoding, MIME, inline thresholds and fallback |
| BL-089, BL-090 | Distinct domain/I/O and delivery counters, stored content identity and retrieval sizes |
| BL-103, BL-110 | Effective profile composition and executable catalog/variant coverage |
| BL-215, BL-216, BL-219, BL-256 | Discovery, initialization, instructions and catalog budgets with revision, profile, cache and fingerprint parity |
| BL-209, BL-210, BL-211 | Selected task/status/event routes, synchronous fallback and capability errors |
| BL-218, BL-258 | Distinct link/read accounting, full retrieval, bounded fallback and integrated CI across activated delivery paths |
| BL-239 | Effective context binding and zero useful bytes on cross-context denial |
| BL-225, BL-228, BL-241, BL-370 | Measured IPC, proxy, stream/event and HTTP framing and payload forwarding on exercised routes |
| BL-249, BL-250 | Deterministic gates before host-sensitive CI and comparisons using compatible measurement versions |
| BL-347 | Actual watch event routes, batch/overflow/resume frames and distinct delivered event bytes |
| BL-356, BL-359 | Per-platform log pages and cursor costs, and actual ADS/xattr payload representations |

For each completed row, the owning BL records focused test/CI evidence,
migrated paths/contracts, and any residual dual-bound to one successor. BL-315
reviews parity and plans machine checks for missing coverage, ownerless rows,
invalid references, and unresolved retrofit on Completed tasks. BL-308
maintains architecture and ADRs. BL-263 enumerates and audits these rows
before stable Version 1.0 publication; completed historical prerequisites are reuse evidence,
not proof that later targets have closed.
