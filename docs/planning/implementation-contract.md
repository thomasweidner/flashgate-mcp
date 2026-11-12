# Technical Backlog Implementation Contract

## Authority and use

This stable contract applies to every technical FlashGate backlog item, including
protocol, security, domain, runtime, release, validation, and accepted Post-1.0
work. [BACKLOG.md](../../BACKLOG.md) owns the ID, status, sprint or Post-1.0
placement, and task-specific acceptance. [Architecture](../architecture.md),
[security](../security.md), the relevant [ADRs](../adr/README.md), and detailed
plans own their respective designs. The [architecture traceability matrix](architecture-traceability.md)
records changing ownership, retrofit, and validation state. A short backlog row
inherits this contract and its family binding; it does not waive them.

Before implementation, the owning BL must resolve and record, directly or by
precise links to its family and traceability rows:

1. **Target architecture role:** MCP adapter, transport, application and
   authorization, domain, jobs and state, execution backend, platform or Native
   Tool adapter, host and runtime, release and validation, or an explicitly
   defined additional layer.
2. **Authoritative sources:** the maintained architecture, security,
   specification, planning, ADR, and public-contract sections governing it.
3. **Hard prerequisites:** only decisions and technical owners whose completed contracts must exist before this owner's implementation can start. Keep later consumers, same-family work, validation owners, and retrofit targets out of this field. Planned predecessors must occur earlier in the documented sprint and within-sprint order; completed historical prerequisites may be reused without reopening them.
4. **Must reuse:** retained packages, completed BL contracts, stores, guards,
   validators, and interfaces to extend instead of rebuilding.
5. **Retrofit targets:** existing implementation, configuration, and public
   contracts that must migrate when the new architecture lands; write `none`
   only after inspecting the current implementation.
6. **Public contract delta:** tool names, schemas, config, error, protocol,
   version, and release effects, including an explicit `none` where true.
7. **Security and resource invariants:** authorization order, principal, group,
   profile, root, capability, risk, backend and generation binding; fail-closed
   behavior, bounds, audit/redaction, stdout purity, and platform parity as
   applicable.
8. **Forbidden alternatives:** second result stores, parallel authorization or
   root validators, global registry mutation for per-client catalogs, ad hoc
   public tools, raw-shell or native passthrough, and duplicate dispatch paths.
9. **Acceptance and validation:** focused positive/negative/boundary tests,
   current [testing requirements](../testing.md), and successor CI owners.
10. **Completion state:** the old path to migrate or retire, and any residual
    explicitly dual-bound in both the closing BL and one concrete successor BL.

A BL is not implementation-ready while its architecture role, hard prerequisites,
reuse, retrofit obligations, or acceptance path are unresolved. The hard-prerequisite graph must be acyclic and have no planned predecessor scheduled after its consumer. A later consumer never becomes a prerequisite merely because it reuses this owner's output. A new or
materially changed technical BL updates its family/task binding here and its
row in [architecture traceability](architecture-traceability.md) in the same
planning change. An architecture change identifies affected current controls,
public contracts, and retrofit targets. `BL-315` owns continuing consistency
review and planned machine checks; `BL-305` and `BL-306` remain completion
controls, and `BL-308` remains the architecture and ADR owner.

Any owner that adds or materially changes a public tool, schema, effective
profile or capability composition, protocol revision, entry instructions, or
adapter that affects the public wire contract inherits
[BL-215 catalog acceptance](#bl-215-catalog-and-entry-budget-binding), including
Post-1.0 owners. An internal-only adapter records that its catalog and entry
delta is absent and measures its own affected cost under its existing contract.

## Foundation and consumer order

`SPR-048` defines release/protocol/payload/native policy contracts that can be built before profiles and Jobs. `SPR-053` establishes configuration and named-root/profile contracts, then BL-236 context, BL-159 authorization, BL-110 effective catalog, BL-166 audit/correlation, and BL-239 state binding; it completes BL-216/219/256 against those contracts. `SPR-049` builds BL-084–BL-089 lifecycle, then BL-090 storage, then dependent Operations, BL-210 Tasks mapping, BL-218 MCP resource handoff, and BL-258 payload/resource CI. Later domains consume these foundations. BL-213 single-transmission work does not require BL-090; BL-218 does. BL-220 defines early native/no-interpreter policy; BL-163 and typed-execution owners implement it later. Exact hard edges and validation/retrofit owners are maintained in [architecture traceability](architecture-traceability.md).

## BL-220 adapter acceptance binding

For every new tool, platform or Native Tool adapter, execution backend,
process-launch path, host, transport or provider, name the first owning BL
and classify the affected code under the [adapter selection
policy](tool-adapters.md#bl-220-adapter-selection). Bind the applicable
[successor acceptance](tool-adapters.md#bl-220-successor-acceptance) directly
from the canonical backlog row and include the owner in that family's
applicability declaration. Record the selected native path, alternatives
considered, fallback or unsupported result, security and resource bounds,
reuse and retrofit targets, and any required later activation decision in
the existing planning and traceability sources. Public tools also update
the [public inventory](tool-adapters.md#20-public-tool-inventory-by-target)
with the required family for the surface. The doccheck gate compares the
inventory, family applicability and canonical backlog acceptance.

Before activation, exercise authorization before dispatch, backend and
platform parity, the affected negative and unsupported cases, enforcement
strength, cancellation and cleanup, and operation-specific efficiency
budgets. The static source baseline is reusable for unchanged paths. Actual
new call sites, behavior, deployment modes and binaries need owner-specific
runtime evidence. Update the [source gate](../testing.md#native-runtime-source-gate)
with any new shipped entry point, build variant or reviewed centralized
launch site and include bypass fixtures. A separate approved security and
architecture decision remains required for a provider, dependency or runtime
model outside the existing contract.

BL-220 supplies the policy, canonical successor bindings and source gate.
BL-163 owns executable runtime policy. BL-134 owns the platform components.
BL-137/138/144/146/147 own their identity, argument, environment and isolation
contracts. BL-151 owns productive integration and the central launch-site
gate under the [staged activation contract](#managed-process-activation).
BL-152 owns runtime execution negatives. BL-242 owns artifact/mode verification. The
operation owners implement and test their own adapters without reopening BL-220.

### Managed Process activation

SPR-056 delivers the Managed Process registry, identity, typed interfaces,
bounded output and lifecycle contracts, and reusable CI for the non-launching
paths. BL-129 and the earlier Operations owners retain their contracts and
the evidence they can prove independently of productive children. Use
injected outcomes and safe isolated platform or observation tests. Keep
`start_process` and unsupported managed-child routes out of the effective
catalog. Reject direct calls and leave raw or alternate process-launch
paths blocked. Earlier owner closure requires proof of its revised scope
and the two-sided launch transfer in BACKLOG.md.

In SPR-057, establish BL-163 policy and the applicable typed, identity, root,
environment, limit, redaction and platform predecessors in the
[architecture traceability](architecture-traceability.md#domains-runtime-and-release-validation)
table. BL-151 then integrates the BL-122 request/dispatch foundation, the
BL-134 platform components and the shared registry and lifecycle into one
reviewable native launcher. Build and test the candidate with its public
routes unavailable. Any source-gate exception must bind the exact reviewed
central launch path. All other launch paths remain blocked.

Before activation, BL-151 owns the native Windows/Linux execution and
denial, lifecycle, restart, race, resource and cleanup evidence transferred
by the earlier owners. Reuse the BL-135 tests, BL-252 race commands and
BL-253/254 CI harnesses. Run the real managed-child and Operations scenarios
in them without a separate launcher or duplicate CI harness. BL-152 and
BL-168 retain ownership of security negatives and least-privilege evidence
respectively. They may develop fixtures independently and run native
scenarios against the unadvertised BL-151 candidate. Their passing evidence
is required before productive activation. This evidence gate is separate
from the hard prerequisites for starting BL-151 implementation.

BL-151 enables the reviewed routes only when policy is effective before
every dispatch and native enforcement meets the requested strength. Reuse
the same boundary for BL-143 and applicable Native Tool adapters. The
synchronous wrapper and its result schema are separate from the prerequisites
for managed start. Later discovery and deployment owners expose only verified,
authorized support under their own acceptance.

## BL-204 conformance binding

BL-204 owns test and validation code at the MCP adapter and STDIO boundary.
It reuses completed BL-200/201/202 output schemas, result contracts and explicit
annotations, BL-207/208 revision policy and dispatch, the runtime registry,
native filesystem guards and existing wire harnesses. The current controls in
`cmd/server`, `internal/mcp`, `internal/protocol` and `internal/mcptest` remain
authoritative; this task adds no transport, runtime dependency or product tool.

Acceptance covers native contracts and deterministic complete snapshots for
`2025-11-25` and the test-policy-only `2026-07-28` candidate, each under default
and read-only profiles. Registry, profile, capability and revision inventories
must have explicit executable coverage; mutations must prove drift detection.
The official suite is evaluated at a pinned upstream commit, with scored and
not-scored server scenarios mapped individually and execution limits stated.
The native validation report is reproducible from the focused tests and their
JSON output. Snapshot updates require an explicit test flag and review.

The public protocol matrix remains unchanged. BL-212 retains full JSON Schema
2020-12 validation; BL-219 retains fingerprints/cache implementation; BL-257
retains dedicated CI drift enforcement. BL-369 may consume this task's focused
report later; BL-370 retains HTTP transport. Neither is implemented here.
No release, distribution or migration behavior changes; test-only work leaves
`VERSION` unchanged. Independent terminal technical review remains required.

## BL-212 schema conformance binding

BL-212 extends the MCP adapter's test and validation boundary. It reuses
BL-200/201 schemas and success results, BL-204 registration, revision/profile
coverage, snapshots and report, and BL-289/291 strict arguments and catalog
parity. The BL-262 inventory records the pinned validator and its transitive
module as test dependencies while preserving verification of earlier bundles.

The existing gate compiles registered input and output schemas as Draft
2020-12 with offline reference resolution, validates successful wire instances,
and tests schema, fixture, catalog and snapshot failures. Tool owners supply
functional cases. BL-110 extends the shared matrix when effective profiles are
implemented, and BL-257 owns dedicated CI drift enforcement. This change adds
no wire fields, runtime checks, product dependencies, activation, or migration.
Independent terminal review is required before closure.

## BL-371 MCP `ping` receiver for `2025-11-25`

**Role and authority.** BL-371 is a revision-specific MCP-adapter implementation
owner in SPR-048, separate from the BL-204 conformance/snapshot candidate.
Use the official `2025-11-25` ping utility and request/schema specification,
ADR-013 and the final `2026-07-28` changelog removing `ping`. Completed
BL-207/208 are the hard predecessors; BL-204 gap evidence is coordination,
not an implementation or hard-prerequisite owner. No BL-257 hard edge is added.

**Reuse and retrofit.** Extend the existing Go `internal/mcp/router`,
`internal/mcp/server`, request validation and exact `internal/mcp/revision`
policy. The accepted design registers a dedicated stateless `ping` handler
on the initialization-based `2025-11-25` path and returns JSON-RPC `result: {}`
with the unchanged request ID. The server owns parameter validation, and the
handler has no tool registry or domain dependency.
Leave `2026-07-28` disabled in production and keep valid enabled-revision
`ping` calls rejected as method-not-found (`-32601`). Do not add any
`tools/list` entry, network/ICMP probe, automatic heartbeat, external MCP SDK,
new dependency or protocol matrix entry. Record the receiver under Unreleased
and keep `VERSION` at the current release version. The
[BL-372 binding](#bl-372-release-oriented-product-versioning) governs later
release preparation and verification.

**Safety, tests and closure.** Honor the real revision request-params contract,
including valid optional metadata; reject malformed inputs through the existing
JSON-RPC boundary without leaking internal errors. Preserve the existing
limits, cancellation, notification no-response, diagnostics redaction and
stdout purity. Exercise actual STDIO positive and negative wire cases, IDs,
invalid params, notifications, unknown methods, ordinary post-initialize
behavior and injected `2026-07-28` revision isolation. Complete applicable
Windows/native Linux, CI/coverage, documentation and independent terminal
review gates. Record the support decision and actual result in BL-371; do not
claim full official conformance or change BL-204's already reviewed bytes.

## BL-257 CI enforcement binding

BL-257 owns the blocking test and release-quality gate in SPR-048. It
consumes the completed BL-204 and BL-212 contracts through the existing
`cmd/mcpcheck` runner, server registry, catalog, and conformance matrix.
Retrofit the Windows and Linux CI test job and its workflow policy tests.
Keep the broad test and coverage jobs in place for their separate guarantees.

Require executed snapshot evidence for every declared revision/profile
combination, complete tool success and denial cases, schema and catalog
parity, and no missing, failed, or skipped required evidence. The feature
owner extends these contracts for future revisions, profiles, capabilities,
tools, and parameters while the CI command stays stable. The gate tests
implemented contracts. Coverage of specification requirements still needs
owner assertions and independent review of intentional contract diffs.

Acceptance requires positive and negative tests, both platform quality
chains, and independent terminal review before Git integration. Snapshot
updates remain explicit. This gate makes no product, activation, version,
migration, or security-policy change and produces only temporary CI evidence.

## BL-213 tool result contract

BL-213 owns the shared tool result contract at the MCP adapter boundary. It
reuses BL-201 result and error envelopes, BL-204 exact-revision wire coverage,
BL-212 schema and catalog parity, and BL-257's existing CI gate. The security
model, [efficiency ADR](../adr/efficiency-contracts.md), and
[protocol guide](../protocol.md#tool-result-classes) govern the public contract.

Each tool declares its result variants and returns typed results to the central
adapter. Payload kind, transfer mode, and result state are independent. Inline
results can combine adapter-derived metadata with ordered payload parts.
Registration rejects missing or unsupported declarations. The adapter accepts
closed object shapes and checks their encoded sizes before serialization.
Payload-bearing results derive their size metadata from the parts. The adapter
rejects malformed UTF-8 text parts before serialization. File text must
round-trip byte for byte. Directory scans count filtered entries toward the
configured entry limit and fail without a partial result. Small metadata can
retain compatibility parity within its budget. File text and directory entries each
have one primary wire location.

Validate all registered variants on the default and read-only paths for each
executable revision. Cover single transmission, malformed contracts, limits,
schema instances, catalog parity, normalized errors, and intentional snapshot
changes through the existing native CI gate. Keep client migration guidance and
Unreleased notes with the wire delta. Require independent review before closure.
The successor owners retain paging, resource delivery, notifications, jobs,
streaming, and efficiency budgets.

## BL-214 wire measurement and coverage

BL-214 is the SPR-048 test and benchmark contract for the MCP adapter and
transport boundary. It builds on BL-189–BL-199 benchmark infrastructure,
BL-213 typed variants and single transmission, and the BL-204/212/257
registry, revision, profile, schema and wire tests. Use the existing server
test harness to capture actual frames and compare their decoded content with
independent fixture data. Measure explicit conversion and serialization sites
through a request-scoped observer used by tests. Production passes no observer.

The ordinary Go suite matches each executable registry variant to a semantic
case and each measured fixture to a hard budget for its actual revision and
profile. The benchmark package measures ordered payload locations and aggregates
the frames of a logical delivery. A producer can add fixture references,
placement and budget bindings without changing the measurement algorithm.
Retain the historical benchmark and allocation contracts. Synthetic cases
exercise future delivery shapes without enabling registration, routes or
capabilities. Preserve schema, redaction, limits, profile enforcement and the
single transmitted representation for heavy payloads.

The [successor matrix](architecture-traceability.md#result-delivery-successors)
binds later producers, profile and catalog owners, transports and CI consumers
to their existing milestones. A non-tool owner must identify its actual route,
prove the negotiated profile/revision/transport and feature availability, and
exercise real output through the common measurement and budget gate. Closure
requires executable coverage mutations, unchanged historical artifact semantics,
applicable native gates and independent review before integration.

## BL-215 catalog and entry budget binding

BL-215 is the SPR-048 MCP adapter and validation contract for bounded effective
catalogs and revision-specific entry. Reuse the BL-189–BL-199 benchmark
references, BL-207/208 exact revisions, BL-204/212/257 registry and schema
conformance, BL-213 result contracts, and the available BL-214 wire harness
and fixtures. The SPR-048 order in [BACKLOG.md](../../BACKLOG.md) allows BL-215
to proceed independently of BL-214's formal closure. Retrofit the existing wire
gate and [tool planning inventory](tool-adapters.md#20-public-tool-inventory-by-target)
in place. The public runtime contract stays byte-compatible. The budget gate
consumes the server-authorized effective registry and does not confer visibility
or execution rights.

Before advertisement or activation, the responsible owner must:

1. Bind the canonical name, milestone, exposure prerequisites and budget class
   in the [tool and adapter plan](tool-adapters.md) for each affected public
   family. Enumerate supported effective compositions, including platform and
   policy differences that change output.
2. Measure actual catalog and entry responses for every affected executable
   exact revision, effective profile and transport. Extend the existing
   conformance snapshots, schema and registry parity, wire fixture and budget
   bindings. Include entry instructions when that revision supports them. For
   every new tool or result variant, preserve the separate BL-214 call budgets.
3. Apply the [effective limits and composition rules](efficiency.md#tool-catalog-and-initialization-budgets)
   to the full catalog. Record field, schema, instruction, frame, JSON-RPC and
   revision costs. Test missing or stale bindings, oversize output, unsupported
   compositions and capability denials as applicable. Keep planned fixtures
   separate from executable production and candidate-policy paths.
4. Show that all activated combinations fit. If they do not, reduce safe
   representational overhead or narrow the authorized composition under the
   owning contract. A change to an accepted ceiling requires an explicit
   decision backed by measurements and compatibility evidence.

New functions consume the accepted BL-215 contract and extend their fixtures,
budgets and tests under their own backlog owner. This does not reopen BL-215.
BL-103/110 own effective composition, BL-216 owns instructions, BL-219 owns
fingerprints and cache behavior, and BL-256 owns comprehensive CI and runtime
enforcement. These are successor consumers, not prerequisites for the current
executable budget gate.

## BL-372 release-oriented product versioning

BL-372 owns release governance and test controls in SPR-048. It reuses the
BL-245 VERSION, changelog and tag contract, BL-248 artifact verification,
BL-255 candidate and promotion identity, BL-262 supply-chain evidence, and
BL-363/364 publication and distribution controls.

Retrofit both build scripts, the candidate and tagged build workflows and
their releaseaudit gates. Ordinary development builds use `0.0.0-dev` with
existing provenance. Notable changes accumulate in Unreleased. A separately
authorized freeze changes VERSION and adds dated, nonempty notes on the
exact candidate commit. A PATCH from an older release uses an isolated
maintenance line when main contains new public features.

Acceptance covers consecutive development changes, MINOR and PATCH freezes,
malformed notes, development artifact rejection, and published identity
reuse, with Windows and native Linux validation and independent review.
VERSION remains 0.3.0. Publication, tag creation and provider writes require
separate authorization. The published v0.3.0 identity is retained.

## Inherited contract families

| Family and BL binding | Required reuse, target boundary, and retrofit |
|---|---|
| MCP, protocol, and results — BL-090, BL-201, BL-204, BL-207–BL-219, BL-257–BL-258, BL-371 | Keep domain results protocol-neutral. BL-090 owns one bounded result lifecycle; BL-218 adapts it to negotiated MCP resource handoff. Preserve exact revision negotiation and conformance, JSON Schema 2020-12, fingerprint/cache isolation, and payload single-transmission rules. Retrofit existing `read_file` and other heavy responses; do not advertise a revision before its gates pass. |
| Policy, execution context, named roots, configuration, and audit — BL-101–BL-111, BL-159–BL-166, BL-233, BL-236, BL-239, BL-352, BL-373 | Build one immutable request/execution context, one server-side authorization decision before dispatch, one configuration model, and one correlation lifecycle. Extend `filesystem.root`, `filesystem.read_only` and current path controls in BL-233's model. Migrate direct MCP-to-filesystem dispatch. BL-101–BL-110 and future domains extend and consume the central model for their own fields and acceptance. Bind handles, cursors, caches, and resources to the effective context; exposure does not authorize. |
| Capability and authorization verification — BL-100, BL-171 | Keep functional rights distinct from profile and risk classification; verify MCP annotations never grant authority. Reuse the central BL-159 decision and its negative tests. |
| Filesystem — BL-036–BL-067, BL-346 | Extend `internal/fs` and the existing `internal/security` path guard. Use named root IDs and relative paths, bounded reads/writes/trees, existing hash and tree primitives, and the shared result lifecycle. Migrate the current eight-tool contracts where their owners require it. |
| Search — BL-068–BL-082 | Extend the accepted `search_paths` and `search_text` public families over confined roots, bounded scanning, pages, and context-bound cursors. Pure Go is the portable baseline. Do not add a public tool per backlog row. |
| Operations and jobs — BL-084–BL-099, BL-164 | Own one generic bounded lifecycle, quotas, cancellation, TTL, cleanup, and fair scheduling. Consume policy and execution context before creating state. Keep domain meaning in domains; reuse BL-090 storage. |
| Process — BL-113–BL-135, BL-162, BL-165, BL-252–BL-254 | Reuse execution context and Operations for owned handles, bounded output, cancellation, and cleanup. External PID control is separate Post-1.0 scope. Do not use a PID alone as authority. |
| Typed execution and Native Tool adapters — BL-136–BL-152, BL-167–BL-170, BL-220, BL-353 | Resolve server-approved command IDs to executables and typed argv, with limits, redaction, and OS isolation. Native Tool adapters are optional, policy-constrained accelerators, never public aliases or interpreter/raw-shell passthrough. |
| System information — BL-062, BL-153–BL-158 | Expose only scoped, explicitly released facts through domain and OS adapters; filter environment and identity data, bound results, and retain Post-1.0 network/administrative decisions outside Version 1.0. |
| Runtime and service modes — BL-221–BL-232, BL-234–BL-238, BL-240–BL-244, BL-341, BL-370 | Keep one core behind direct STDIO and local proxy/service transports. Consume BL-233's validated configuration per runtime role. BL-236 owns backend-neutral dispatch. Variant A service account is the Version 1.0 backend; unsupported user-worker selection fails closed. Preserve host ownership and stdout purity. |
| Release, artifacts, and validation — BL-203–BL-204, BL-215–BL-216, BL-241–BL-263, BL-305–BL-315, BL-363, BL-364, BL-366, BL-367, BL-369 | Reuse root `VERSION`, BL-245 changelog/tag semantics, BL-248 verification and current CI gates. BL-255 builds/verifies candidates, then promotes the exact bytes locally with immutable identity and no additional remote publication. BL-262 supplies evidence/rollback controls. BL-363 consumes verified candidates and evidence for explicit persistent public pre-1.0 GitHub prereleases with `prerelease=true`, exact version/source/artifact identity, no publication-time rebuild or silent asset replacement, minimal separate release-write permissions, and remote readback. Finalize release-relevant source and dated changelog notes before public candidate freeze. BL-263 separately requires stable Version 1.0 traceability/gate closure and reuses the publication foundation; prerelease success never passes that gate. Retrofit the tag-gated build/validation path, transient Actions-artifact handoff and public download guidance. BL-364 owns the accepted five-asset distribution target for all releases including v0.3.0: four original archives and shared SHA256SUMS, preserving complete verification and required temporary evidence within regular Actions retention. Release-unassigned BL-366 owns SBOM security management; release-unassigned BL-367 owns GitHub-native signed attestations. Neither blocks a release or is implicitly implemented here; see the task bindings below. |
| Response and benchmark quality — BL-172–BL-173, BL-177–BL-179, BL-205, BL-317–BL-323, BL-325–BL-329, BL-331–BL-332 | Reuse current benchmark schemas, deterministic workflow/corpus controls, host/platform evidence, and documentation gates. Strengthen checks at their existing owners without weakening result, supply-chain, platform, or public documentation truth; no benchmark result itself authorizes product behavior. |
| Accepted Post-1.0 capabilities — Post-1.0 workstream BLs in BACKLOG.md | Inherit the same context, root, authorization, domain, state, result, adapter, and validation contracts. A later milestone does not relax them or imply current implementation. Detailed names and adapter boundaries live in [tool adapters](tool-adapters.md). |
| Documentation navigation tooling — BL-368 | Independent release-unassigned development tooling, outside the product runtime and release-security owners. Reuse completed documentation validation and extend its local/read-only CI consumers under the [BL-368 binding](#bl-368-documentation-navigation); no hard dependency on BL-365 or release obligation. |

## BL-233 central runtime configuration

**Role and prerequisites.** SPR-053 bootstrap and core configuration owner.
Implement the [ADR-014 configuration contract](../adr/runtime-modes.md#configuration-contract).
The traceability row has no hard BL predecessor. Selecting and adopting a
reviewed TOML library requires a separate dependency decision. BL-223 and the
runtime, policy, and domain owners consume the model.

**Reuse and retrofit.** Reuse existing configuration and startup code where
it fits. BL-233 may reorganize the internals when the implementation shows
a simpler design, better maintainability, and lower resource use. Preserve
PathGuard, root confinement, existing limits and read-only controls,
fail-closed startup, safe startup errors, and protocol and compatibility
guarantees. Migrate the current `MCP_*` runtime inputs, including path policy,
payload limits, and diagnostics. BL-101/103 retain their named-root and profile
semantics. BL-166 consumes the logging path and owns the audit lifecycle.
Apply the shared source-separation check after PathGuard resolves each data
root. BL-101 reuses this check for each named root before exposing it.

**Public delta and invariants.** The future startup configuration contract
changes from environment inputs to one trusted TOML file per instance.
Publish the actual compatibility, conversion, and rollback behavior in
[migration](../migration.md#root-configuration) and the affected client
guidance. Keep STDIO invocation and protocol output compatible under the
new configuration contract. Apply the same source trust, immutable effective
state, rejection, redaction, and no-startup-write rules for every role.
BL-233 owns a complete, valid, documented TOML configuration template for
its supported settings. BL-223 owns explicit CLI creation through
`config init`, consumes that template, and uses the same schema and validator
for creation and any CLI validation. BL-373 owns later MCP management. This
binding defines no new MCP schema or tool and assigns no product version or
release.

**Acceptance.** Test role and identity selection, secure default and absolute
override, absent, invalid, redirected and untrusted inputs, invalid security
fields, current-policy migration, validation before initialization, immutable
consumer state, log limits/redaction, stdout purity, and absence of startup
writes on native Windows and Linux. Prove that configuration denial cannot
trigger endpoint or weaker-source fallback. Verify that the documented TOML
template covers the supported settings and passes the shared validator.
BL-241/242 validate the enabled managed-mode integrations, and BL-243
documents the delivered paths.

## BL-373 authorized MCP configuration management

**Role and prerequisites.** Post-1.0 application and authorization owner.
Implementation requires BL-233 (typed model and validation), BL-236 (trusted
caller and execution context), BL-159 (central authorization), BL-110
(effective catalog), and BL-166 (audit and correlation). Reuse named roots,
profiles, capabilities, and identity bindings from those foundations.
BL-370 is a later transport consumer. Each supported transport must meet
its own activation conditions before it exposes management.

**Reuse and retrofit.** Extend BL-233's model and effective-state access
with one transport-neutral management function in the shared Go core.
Route enabled direct, proxy, and service paths to it through the existing
dispatch, authorization, effective catalog, and audit contracts. The service
keeps its own configuration authority. A compatible MCP Apps client may
provide a graphical interface through the same management API and checks.
Any later Streamable HTTP exposure uses the same core functions after
BL-370's authentication, security, and activation gates. There is no second
configuration engine or client-side mutation path.

**Public delta and invariants.** Provide a bounded, redacted view of effective
configuration and typed change preparation, validation, and a readable
preview before any mutation. Require explicit server-side authorization for
each step, including current authorization at commit time. Protect secrets,
sensitive paths, and administrative policy from exposure or unauthorized
change. Only an explicitly authorized effective tool catalog can expose
management. Discovery, transport availability, and client or agent
confirmation confer no permission. The implementation owner decides the
public tool names, schemas, and errors before activation.

**Acceptance.** Define version/concurrency conflicts, compare-and-swap (CAS),
safe persistence and postwrite readback, failure and recovery behavior,
and when restart makes a persisted change effective. Preserve BL-233's
immutable running configuration. Prove native Windows/Linux behavior with
positive, security, negative, consistency, and integration tests across the
authorized paths. Cover stale or competing proposals, denied catalog access
and direct calls, authorization changes before commit, redaction, persistence
failures, and restart. Verify optional UI and later transport exposure
against the same server-side policy before enabling them.

## BL-369 human-readable release quality reporting

**Architecture role and prerequisites.** Independent release-unassigned
presentation/aggregation owner; reuse completed BL-248/255/262 verification and
canonical VERSION/source/asset identity. MCP BL-204/212/257, security BL-366,
future attestation BL-367 and benchmark/CI owners remain evidence producers, not
a hard prerequisite graph or a replacement release gate. Architecture and status
follow the actual producer contracts and [release scope](release-scope.md).

**Target and boundaries.** Render an actionable human-readable Markdown report
and GitHub Actions Job Summary for a verified candidate, with optional later
version-specific public GitHub Markdown after separately confirmed publication.
Show a release summary, version/commit/artifact IDs, platform and MCP coverage,
quality/security/SBOM/advisory/reproducibility/performance findings, outstanding
exceptions, differentiated pass/fail/not-run/not-applicable/unverified states,
links to real producer evidence and its retention, and the actual decision owner.
The report never invents PASS on absent or expired evidence, silently overrides
BL-366 informational findings, creates its own vulnerability/SBOM/conformance
engine, or grants release authority. Use deterministic redacted bounded rendering
and exact source/attempt bindings; test missing, stale, contradictory, and
misbound inputs. Review whether Markdown under docs/releases/ is an appropriate
durable public location during implementation; do not add a sixth Release asset
or change the existing five-asset distribution and CHANGELOG release notes.

**Scope/closure.** Reuse the existing CI and verified producer outputs, with no
product runtime, VERSION, frozen artifact or new GitHub write/permission effect.
Publishing a version-specific permanent report is a separate authorized Git
operation with poststate readback. Until a separate release-gate decision, this
release-unassigned owner does not block existing prereleases or stable BL-263.

## BL-370 future Streamable HTTP MCP transport

**Architecture role and prerequisites.** Separate accepted Post-1.0
transport/host/security implementation owner. Reuse BL-224 transport/core
separation, BL-233 effective configuration, BL-204 exact-revision
conformance/snapshot tests, BL-212 schema
validation and, when effective state is involved, BL-219/239 bindings. Before
implementation, explicitly decide listener/network exposure, endpoint,
authentication and principal binding, TLS/origin/DNS-rebinding/CSRF,
limits/stream lifetime, host lifecycle and platform threat model through its
own ADR and security review. No Version 1.0 release prerequisite or premature
HTTP support advertisement.

**Target and acceptance.** Implement the actual supported Streamable HTTP MCP
transport directly against the existing Go core. HTTP transport availability
does not authorize BL-373 management. Expose management only if it is
implemented, explicitly authorized in the effective catalog, and covered
by the HTTP authentication, security and activation gates.
Extend the existing conformance/snapshot contract for each exposed pair of
exact protocol revision and HTTP transport, including official HTTP server
conformance against the real isolated test listener plus negative security,
protocol, lifecycle and Windows/Linux tests. Require deterministic automatic
missing-coverage detection and explicit public revision/transport support
matrix updates; a missing/failed gate prevents HTTP activation. No production
HTTP-to-STDIO bridge, uncontrolled remote listener, new dependency, permission
or service change without its own decision and authorization. Future additions
belong to this transport owner, not a reopening of BL-204.

## BL-368 documentation navigation

**Role and authorities.** BL-368 owns future deterministic README navigation
generation and drift validation in development tooling, outside `cmd/server`.
[BACKLOG.md](../../BACKLOG.md#release-unassigned-documentation-tooling) owns its
Planned status and release-unassigned placement; the
[traceability row](architecture-traceability.md#release-unassigned-documentation-tooling)
maps reuse, retrofits and closure. The [documentation quality gate](../documentation-quality-gate.md),
[documentation style](../documentation-style.md) and [testing requirements](../testing.md)
govern public-document rules and validation. This registration implements nothing.

**Prerequisites and reuse.** The existing documentation-validation foundation
is available; there is no outstanding hard prerequisite.
Inspect and reuse `cmd/doccheck` heading, fence, duplicate-anchor, explicit-anchor
and local-link handling, its fixtures, and `scripts/Test-DocumentationConsistency.ps1`
before adding behavior. Extend these owners rather than creating a second
general documentation engine. BL-365 is the separate onboarding owner in SPR-048;
its manual README navigation can change without this tooling. Neither BL is a
hard predecessor of the other, and neither may block the other's completion.
No sprint, Version 1.0 or Post-1.0
commitment or release gate is introduced.

**Retrofits and public contract.** Later separately authorized implementation
must integrate a reusable local generation command with one clearly marked
README ToC region and extend the existing documentation/CI gate and contributor
guidance. Reconcile the then-current README shape, including any BL-365 changes,
without taking over its onboarding scope. Navigation derives from actual Markdown
headings in stable document order. Resolve GitHub-compatible heading and anchor
rules consistently with existing validators, including normalization, Unicode,
punctuation, duplicate headings, explicit anchors and fenced code. No MCP tool,
schema, configuration, runtime, binary, product-version or release delta is planned.

**Representation and safety.** The local generator may change only one
unambiguously delimited managed region. Preserve every byte outside it and the
existing UTF-8 encoding, BOM state, LF/CRLF representation and final newline.
Repeated generation must produce identical bytes; no silent whole-file rewrite
or data loss. Missing, duplicate, malformed or ambiguous markers fail closed
before writing. Bound document/input size and processing resources using the
existing checker limits; preserve local-first behavior and public portability
without private workflow dependencies, credentials, installations or permissions.

**CI and excluded alternatives.** The existing documentation gate must compute
the expected bytes without mutating the checkout and fail with actionable
diagnostics on drift, invalid anchors, missing/invalid links or unexpected changes
outside the managed region. A targeted local command may update that region.
CI commits, pushes or automatic PR creation require a separate later architecture
decision and authorization; no extra GitHub write rights are implied. Exclude
manual link maintenance as the permanent solution, a competing documentation
engine, a documentation website and new external dependencies without a later
decision. Registration changes no README, generator, executable CI or permissions.

**Validation and completion.** Later implementation requires focused positive
and negative fixtures for normal/normalized headings and GitHub anchors,
duplicates, Unicode and punctuation, fenced code, explicit anchors, valid/invalid
links and targets, added/removed/renamed headings, missing/duplicate/malformed
markers, stable order, repeated byte-identical generation, LF/CRLF and UTF-8/BOM/
final-newline preservation, and unchanged bytes outside the managed region.
Prove Windows/Linux and CI parity, read-only drift failure and the applicable
documentation and implementation gates. Reconcile backlog, this binding and the
traceability row with actual generator, validator, README and guidance retrofits;
disposition documentation, CHANGELOG/VERSION, tests/CI, candidates,
release/distribution, migration and security, with any residual dual-bound to one
concrete successor. The present registration closes only its planning action;
BL-368 remains Planned until separately authorized implementation and validation
complete. It creates no product/release artifact, migration or active security
change and leaves BL-365 and completed owners unchanged.

## BL-363 implementation binding

Architecture role: release and validation, with a narrow provider adapter.
The publication correction keeps that role: read a positively identified draft
by its immutable provider ID, and add a separately authorized empty-draft
continuation using the unchanged original publication manifest. Reuse the
existing evidence verifier, upload/readback sequence, consumed-approval history
and exclusive journal. Retrofit the GitHub adapter, manual workflow and
publication tests; do not add a recovery framework or product dependency.
Acceptance additionally covers list-invisible drafts, direct-ID errors and
contradictions, exact empty-draft prestate, fresh continuation approval,
unchanged manifest bytes, and credential-free public download verification.
Upload acceptance also binds response negotiation independently from the raw
request media type: JSON responses, octet-stream bodies and exact lengths.
Retrofit the request helper, upload adapter and existing execution error
boundary so only a typed numeric HTTP status survives a failed upload; never
forward arbitrary provider or file-close errors. Adapter and complete
continuation tests must prove one write, no redirect/retry and no sensitive
diagnostic output. This is the same BL-363 correction scope and adds no
dependency, release identity or recovery path.
Hard prerequisites remain completed BL-245/248/255/262. Extend cmd/releaseaudit
and consume verifySupply and candidate semantics without a second verifier.
The separate manual publish-prerelease.yml job owns minimal release-write
rights; release-build.yml and current-version-candidate.yml remain read-only
build/verification consumers. Public delta is prerelease distribution tooling,
with no MCP/runtime or product VERSION change. Retrofit download guidance and
the complete transient evidence handoff through the
[publication contract](../prerelease-publication.md).

Acceptance includes anchored four-target evidence, committed source/notes/tag
parity, one-shot draft/upload/publication, full downloaded byte readback,
permission/rerun/conflict/path negatives and applicable Windows/Linux gates.
The local BL-255/262 retrofit shares only stage-directory promotion: bounded
Windows Win32 5/32 follow-ups require stable directory/ancestor identities,
target absence and the existing complete verifier. Eight calls and a two-second
retry-control window are limits, not a blocking-call deadline. Non-Windows,
current-alias operations and remote one-shot publication retain their policies.
Acceptance also requires deterministic drift/budget tests, ownership-checked
cleanup, candidate rollback parity and native Linux single-call regression.
The first public v0.3.0 prerelease and all 40 original asset downloads passed
independent publication acceptance on 2026-10-25. The publication contract now
records the workflow-token capability condition and the separate exact manual
publish-only boundary, with no new credential mechanism or automatic recovery.
Formal BL-363 closure requires review/integration of this documentation
and the canonical full-work-item completion transition. No product,
VERSION, CHANGELOG, candidate or binary bytes change in this closure; accepted
technical/platform evidence remains applicable. Historical failed-attempt causes
remain unproven without leaving a current publication-acceptance failure.
This prerelease does not satisfy BL-263; stable publication remains BL-263.

## BL-364 planning binding

Architecture role: release/distribution and validation, extending the existing
publication adapter. Completed in SPR-048 after completed BL-363; hard prerequisites
are completed BL-245, BL-248, BL-255, BL-262 and BL-363. Reuse root VERSION,
canonical CHANGELOG notes/tag parity, internal/version archive names, BL-248
verification, BL-255 immutable candidate bytes, BL-262 complete externally anchored
evidence and BL-363 authorization, provider journal, reconciliation and public
download readback. Completed owners remain closed; BL-263 retains stable release
closure and is a later consumer, not an implementation prerequisite.

Authorities: [release architecture](../architecture.md#release-and-supply-chain-architecture),
[security](../security.md#release-and-supply-chain-security),
[artifact verification](../artifact-verification.md#native-supply-chain-evidence),
[publication](../prerelease-publication.md), [release scope](release-scope.md),
[testing](../testing.md#bl-364-five-asset-distribution-target) and
[DEC-FP-003](../adr/product-metadata.md#dec-fp-003---metadata-build-and-architecture-implementation-strategy).

### Accepted distribution target

Every FlashGate GitHub Release, including existing v0.3.0 and later stable
releases, targets exactly five uploaded assets: the four original verified
Windows x64/ARM64 ZIP and Linux x64/ARM64 TAR.GZ archives plus one shared
`SHA256SUMS`. Preserve canonical archive names and bytes. GitHub-generated source
downloads are excluded. No separate `.sha256` files or public build, test,
provenance, SBOM or other evidence assets belong in this list.

`SHA256SUMS` uses the four original verified archive SHA-256 values, with exactly
one entry per canonical archive basename, ordinal filename ordering, lowercase
hexadecimal hashes, GNU-compatible `<sha256><two spaces><filename>` records,
UTF-8 without BOM, LF and a final LF. It is generated from verified identities,
never from untrusted provider metadata. A consumer or installation script must
be able to download only its selected platform archive and `SHA256SUMS`, match
exactly that archive entry and verify the bytes without the other three archives.
This is a distribution capability requirement, not an installer implementation
under this planning task.

Original archive hashes, actual GitHub asset SHA-256 digests and credential-free
downloaded archive bytes must agree in the release pipeline. Also bind and
verify the generated `SHA256SUMS` bytes through provider digest and downloaded
readback. Missing, malformed or conflicting digests fail closed; provider metadata
alone is neither original verification evidence nor builder authentication.
CHANGELOG.md remains the sole narrative release-notes source from the frozen
commit. Add no separate SHA-256 section to release notes, no additional public
release manifest and no separate permanent release ledger. Preserve the existing
`VERSION + SourceCommitSHA + ArtifactSHA256` identity. Internal manifests, sibling
checksums and the existing one-shot journal retain their existing roles.

Complete BL-248/255/262 verification and required temporary evidence remain
mandatory. Evidence stays outside the public release assets and is available
for the verification, publication, independent review and readback that consume
it. There is no duty to retain raw evidence beyond regular GitHub Actions
retention, no required new storage service and no historical evidence dataset
as a permanent acceptance deliverable. Missing evidence needed for an actual
operation still stops that operation; an explicitly bound retained original
copy remains an existing option, never an untrusted fallback or later rebuild.
BL-262 remains Completed with its original unsigned evidence scope.

Lightweight tags remain standard with exact tag/commit/VERSION parity. BL-364
adds no mandatory signed provenance, signing keys or signing infrastructure.
It activates neither GitHub Artifact Attestations nor Immutable Releases.
Separate future security owners BL-366 and BL-367 are release-unassigned,
non-blocking work; neither is a BL-364 prerequisite.

### Historical v0.3.0 acceptance

Closure includes actual v0.3.0 normalization. Its original publication had 40
uploaded assets; preserve that true history and completed BL-363. Before any
mutation, independently verify the exact original inventory, asset IDs, lengths,
original bytes/hashes, candidate/evidence identity bindings and provider state.
Derive the new `SHA256SUMS` only from the four verified original archive hashes.
Keep the evidence needed for the operation and acceptance temporarily; no
permanent historical raw-evidence dataset is required.

Uploading the one new `SHA256SUMS` and deleting exactly the 36 existing additional
assets each require separate explicit external authorization with exact repository,
release ID, asset names/IDs/hashes, retained archive IDs/hashes, executor and
expected provider state. Bind the upload to its exact generated bytes/hash.
Implementation approval grants neither action. Each mutation is one-shot with
fresh prestate and readback. Reconcile failed or ambiguous operations read-only
without blind retry, compensation or implicit asset recreation. Any other
release or notes mutation also needs its own explicit authorization.

After the authorized operations, independently read back the complete paginated
inventory: the four original archive IDs/bytes and one new `SHA256SUMS`, exactly
five assets. Prove unchanged version, tag, source commit and prerelease identity,
original-hash/provider-digest/credential-free-download parity and checksum-file
content/byte parity. Provider restrictions, identity drift or unavailable required
evidence stop the transition without bypass. Preserve the original 40-asset
publication statement as history.

### Retrofit, validation and closure

Retrofit cmd/releaseaudit publication inventory/authorization/manifest, checksum
aggregation, provider digest/readback and historical disposition tests; the manual
publish-prerelease.yml consumer; public download/verification/build guidance;
release/security/testing contracts and DEC-FP-003. The current all-file upload
and public publication-manifest path must yield to the five-asset contract;
internal authorization and identity bindings remain. Preserve complete candidate
and evidence generation/verification, fail-closed checks, prerelease/stable
separation, minimal rights, explicit authorization, one-shot reconciliation and
secret-safe diagnostics. No second verifier, publication-time rebuild/repackaging,
silent replacement or new permanent ledger.

Acceptance covers exact five-asset and four-archive/content inventories,
deterministic GNU/LF checksum records, selected-platform-only consumer verification,
original/digest/download parity and unchanged identities. Test missing, extra,
duplicate, swapped or tampered assets, checksum entries or required evidence;
malformed digests, CRLF/BOM/unsafe names, checksum/notes/identity drift, unavailable
provider data, unsafe deletion subjects and failed or ambiguous operations.
Reuse BL-248/255/262/363 tests and applicable [testing gates](../testing.md), with
independent review of this material release/security plan before technical
implementation, and independent review of the later implementation.

The publisher retrofit, selected-platform verification, required independent
technical review and actual historical v0.3.0 normalization are accepted. BL-364
is Completed: the current public release has four unchanged original archives
and shared SHA256SUMS, with independently verified asset identities/digests and
user-confirmed public-download hash parity. The original 40-asset publication
remains true history. The later, explicitly approved cumulative notes contain
the frozen CHANGELOG sections 0.1.0, 0.2.0 and 0.3.0 and have a distinct
[notes identity](../prerelease-publication.md#implementation-and-availability).
Guidance, ADRs and status are reconciled; existing tests/CI and candidate
identities remain valid, with no product migration or deferred deliverable.
This developer-tooling retrofit changes no product runtime or existing artifact
identity and requires no VERSION increment or historical changelog rewrite.
Git, provider/release/tag/asset, signing, permission, credential, installation and
cleanup actions retain separate authorization boundaries.

## Closure

At each BL closure, reconcile the traceability row, current documentation,
`CHANGELOG.md` and `VERSION`, tests and CI, candidate artifacts,
release/distribution, migration, and security. Mark retrofit obligations
completed only with implementation and validation evidence. A deferred item
must appear in the predecessor and one concrete successor acceptance scope.
The Version 1.0 `BL-263` gate requires no ownerless architecture requirement
or unresolved retrofit obligation in its release scope except an explicit,
approved waiver recorded by that release gate.

## BL-262 task binding

Architecture role: release and validation. Hard prerequisite: completed BL-255;
reuse completed BL-248 verification, reproduction and leak contracts, BL-245
VERSION semantics, internal/version and the existing releaseaudit CLI. Retrofit
current-version-candidate.yml at its complete manifest consumer. The tag-gated
release-build.yml stays a validation path; BL-363 owns its later publication
consumer retrofit and must use this evidence authority.

Acceptance is the [native supply-chain evidence contract](../artifact-verification.md#native-supply-chain-evidence):
one deterministic source SBOM, four unsigned in-toto Candidate-Verification statements, compact index, external
trust anchors, complete portable bundle, strict readback, leak rejection and
synthetic rollback through BL-255. No MCP/runtime contract or VERSION delta,
new dependency, second CLI/matrix/verifier, credentials or productive signing.
Build tools and third-party assets are distinguished from runtime dependencies;
source inventory never implies complete platform binary composition. BL-363
consumes the complete anchored set within retention or from a bound local copy;
BL-263 owns stable release closure. Closure requires local platform gates and
independent review; publication and provider/signing decisions remain separate.

## Release-unassigned security task bindings

BL-366 and BL-367 are independent owners with no sprint, release milestone
or release-blocking obligation. They may be scheduled whenever their prerequisites
and a separate implementation authorization are satisfied. Their registration
activates no tool, external provider, workflow permission or credential change.

### BL-366 SBOM security management

Architecture role: dependency security management and validation. Authorities are
[security](../security.md#release-and-supply-chain-security),
[SPDX inventory semantics](../artifact-verification.md#inventory-and-provenance-semantics),
[testing](../testing.md#release-unassigned-security-validation) and the traceability
row. Hard prerequisites: completed BL-262 inventory and BL-324 dependency-maintenance
configuration. Reuse their SPDX 2.3 source inventory and existing Dependabot/GitHub
security functions. Retrofit the existing releaseaudit inventory consumer,
dependency-security triage, relevant CI/security gates and documentation.

Acceptance: correlate inventory dependencies with known advisories/CVEs, distinguish
direct/transitive dependencies and actual product exposure, and assess/prioritize
findings with remediation or documented exceptions. Include security/negative
tests for missing/stale or mismatched inventory/advisory data, classification
errors and unjustified exceptions. Do not claim the existing source inventory
proves full platform binary composition. The authorized local implementation
extends `cmd/releaseaudit` with a verified inventory reader, bounded read-only
OSV transport/snapshot capture, deterministic offline assessment and a report/
exception consumer. Reuse `verifySupply` with independent version, commit,
manifest and index anchors; reconcile carried `source/go.mod` indirect markers
with SPDX dependency and use relationships. Preserve UNKNOWN/INCOMPLETE on
contradictions or missing evidence. OSV querybatch is positional and paginated;
ID detail reads are separate. No provider call is authorized by local development.
Use offline fixtures and local HTTP servers for validation. Existing Dependabot
remains complementary without an alert API, new scopes or credentials.

Candidate verification and confirmed successful prerelease publication each
invoke a separate informative assessment. The existing candidate dispatch and
local CLI are the manual routes. Findings of every severity and provider errors
create no commit, PR, merge or release gate; existing integrity/build/test gates
remain blocking. Assessment errors are visible as INCOMPLETE/ERROR and cannot
reclassify successful publication. Outputs remain temporary run evidence and
Actions summaries, outside the five public assets. No workflow permission or
credential forwarding change, dependency mutation, new runtime dependency,
database or persistent finding lifecycle. Versioned exact-identity triage retains
findings and flags invalid/expired exceptions. Add no second SBOM generator, mandatory
public SBOM asset or runtime/MCP contract. Close only with accepted triage,
gates, tests and current guidance; disposition VERSION/CHANGELOG, CI, artifacts,
release, migration and security under the shared closure contract.

### BL-367 GitHub Artifact Attestations

Architecture role: release supply-chain authentication and validation. Authorities
are [DEC-FP-003](../adr/product-metadata.md#accepted-public-distribution-target),
[security](../security.md#release-and-supply-chain-security),
[testing](../testing.md#release-unassigned-security-validation) and the traceability
row. Hard prerequisites: completed BL-248/255/262/363. Coordinate with BL-364
distribution without a hard edge in either direction. Reuse the four verified
immutable archive identities, existing release workflows and unsigned evidence
without relabeling BL-262 as signed provenance.
Retrofit only later authorized workflow attestation generation/verification,
minimal permission declarations, security/negative tests and public guidance.

Acceptance: automatically create signed GitHub-native build attestations for
exactly the four verified platform archives in later authorized release workflows,
using Actions OIDC and short-lived workflow identity; verify with GitHub-native
methods bound to source/workflow/subject identity. Test tampering, missing/swapped
subjects, wrong repository/workflow/source identity, unavailable verification and
excess permission. No permanent signing keys, long-term credentials or additional
release-download asset. Provider/permission decisions and activation require the
later implementation authorization. Close with accepted generation, verification,
permissions, negative tests and guidance plus the shared closure dispositions.
