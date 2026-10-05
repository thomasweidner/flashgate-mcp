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

## Foundation and consumer order

`SPR-048` defines release/protocol/payload/native policy contracts that can be built before profiles and Jobs. `SPR-053` establishes configuration and named-root/profile contracts, then BL-236 context, BL-159 authorization, BL-110 effective catalog, BL-166 audit/correlation, and BL-239 state binding; it completes BL-216/219/256 against those contracts. `SPR-049` builds BL-084–BL-089 lifecycle, then BL-090 storage, then dependent Operations, BL-210 Tasks mapping, BL-218 MCP resource handoff, and BL-258 payload/resource CI. Later domains consume these foundations. BL-213 single-transmission work does not require BL-090; BL-218 does. BL-220 defines early native/no-interpreter policy; BL-163 and typed-execution owners implement it later. Exact hard edges and validation/retrofit owners are maintained in [architecture traceability](architecture-traceability.md).

## Inherited contract families

| Family and BL binding | Required reuse, target boundary, and retrofit |
|---|---|
| MCP, protocol, and results — BL-090, BL-201, BL-204, BL-207–BL-219, BL-257–BL-258 | Keep domain results protocol-neutral. BL-090 owns one bounded result lifecycle; BL-218 adapts it to negotiated MCP resource handoff. Preserve exact revision negotiation and conformance, JSON Schema 2020-12, fingerprint/cache isolation, and payload single-transmission rules. Retrofit existing `read_file` and other heavy responses; do not advertise a revision before its gates pass. |
| Policy, execution context, named roots, configuration, and audit — BL-101–BL-111, BL-159–BL-166, BL-233, BL-236, BL-239, BL-352 | Build one immutable request/execution context, one server-side authorization decision before dispatch, one configuration model, and one correlation lifecycle. Migrate `MCP_ROOT`, `MCP_READ_ONLY`, current path controls, and direct MCP-to-filesystem dispatch. Bind handles, cursors, caches, and resources to the effective context; exposure does not authorize. |
| Capability and authorization verification — BL-100, BL-171 | Keep functional rights distinct from profile and risk classification; verify MCP annotations never grant authority. Reuse the central BL-159 decision and its negative tests. |
| Filesystem — BL-036–BL-067, BL-346 | Extend `internal/fs` and the existing `internal/security` path guard. Use named root IDs and relative paths, bounded reads/writes/trees, existing hash and tree primitives, and the shared result lifecycle. Migrate the current eight-tool contracts where their owners require it. |
| Search — BL-068–BL-082 | Extend the accepted `search_paths` and `search_text` public families over confined roots, bounded scanning, pages, and context-bound cursors. Pure Go is the portable baseline. Do not add a public tool per backlog row. |
| Operations and jobs — BL-084–BL-099, BL-164 | Own one generic bounded lifecycle, quotas, cancellation, TTL, cleanup, and fair scheduling. Consume policy and execution context before creating state. Keep domain meaning in domains; reuse BL-090 storage. |
| Process — BL-113–BL-135, BL-162, BL-165, BL-252–BL-254 | Reuse execution context and Operations for owned handles, bounded output, cancellation, and cleanup. External PID control is separate Post-1.0 scope. Do not use a PID alone as authority. |
| Typed execution and Native Tool adapters — BL-136–BL-152, BL-167–BL-170, BL-220, BL-353 | Resolve server-approved command IDs to executables and typed argv, with limits, redaction, and OS isolation. Native Tool adapters are optional, policy-constrained accelerators, never public aliases or interpreter/raw-shell passthrough. |
| System information — BL-062, BL-153–BL-158 | Expose only scoped, explicitly released facts through domain and OS adapters; filter environment and identity data, bound results, and retain Post-1.0 network/administrative decisions outside Version 1.0. |
| Runtime and service modes — BL-221–BL-231, BL-234–BL-238, BL-240–BL-244, BL-341 | Keep one core behind direct STDIO and local proxy/service transports. BL-233 owns configuration precedence; BL-236 owns backend-neutral dispatch. Variant A service account is the Version 1.0 backend; unsupported user-worker selection fails closed. Preserve host ownership and stdout purity. |
| Release, artifacts, and validation — BL-203–BL-204, BL-215–BL-216, BL-241–BL-263, BL-305–BL-315, BL-363 | Reuse root `VERSION`, BL-245 changelog/tag semantics, BL-248 verification and current CI gates. BL-255 builds/verifies candidates, then promotes the exact bytes locally with immutable identity and no additional remote publication. BL-262 supplies evidence/rollback controls. BL-363 consumes verified candidates and evidence for explicit persistent public pre-1.0 GitHub prereleases with `prerelease=true`, exact version/source/artifact identity, no publication-time rebuild or silent asset replacement, minimal separate release-write permissions, and remote readback. Finalize release-relevant source and dated changelog notes before public candidate freeze. BL-263 separately requires stable Version 1.0 traceability/gate closure and reuses the publication foundation; prerelease success never passes that gate. Retrofit the tag-gated build/validation path, transient Actions-artifact handoff and public download guidance. |
| Response and benchmark quality — BL-172–BL-173, BL-177–BL-179, BL-205, BL-317–BL-323, BL-325–BL-329, BL-331–BL-332 | Reuse current benchmark schemas, deterministic workflow/corpus controls, host/platform evidence, and documentation gates. Strengthen checks at their existing owners without weakening result, supply-chain, platform, or public documentation truth; no benchmark result itself authorizes product behavior. |
| Accepted Post-1.0 capabilities — Post-1.0 workstream BLs in BACKLOG.md | Inherit the same context, root, authorization, domain, state, result, adapter, and validation contracts. A later milestone does not relax them or imply current implementation. Detailed names and adapter boundaries live in [tool adapters](tool-adapters.md). |

## Closure

At each BL closure, reconcile the traceability row, current documentation,
`CHANGELOG.md` and `VERSION`, tests and CI, candidate artifacts,
release/distribution, migration, and security. Mark retrofit obligations
completed only with implementation and validation evidence. A deferred item
must appear in the predecessor and one concrete successor acceptance scope.
The Version 1.0 `BL-263` gate requires no ownerless architecture requirement
or unresolved retrofit obligation in its release scope except an explicit,
approved waiver recorded by that release gate.
