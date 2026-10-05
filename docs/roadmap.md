# FlashGate MCP Roadmap

`BACKLOG.md` is the only authoritative planning and steering document. This roadmap summarizes sequence and release boundaries without duplicating every canonical task.

## Current direction

FlashGate MCP is the binding project name. The current implementation is a native Go filesystem MCP over STDIO. Version 1.0 expands that core into a bounded local host-operation platform while preserving low startup latency, low RAM/CPU use, compact tool catalogs, and no interpreter runtime.

Backlog status describes execution state. The sprint sequence identifies Version 1.0 work, while the separate Post-1.0 workstream table identifies accepted work beyond Version 1.0.

## Version 1.0 sequence

| Phase | Sprint IDs | Direction |
|---|---|---|
| Architecture and identity | `SPR-041` | FlashGate identity, ADR baseline, authoritative backlog consolidation |
| Technical transition | `SPR-042`–`SPR-044` | Technical rename, pre-1.0 filesystem contract cleanup, read-only client preparation |
| Release and contract definitions | `SPR-045`–`SPR-048` | Coverage/version baseline, exact-byte local candidate promotion, supply-chain foundation, planned persistent public pre-1.0 GitHub prereleases, MCP conformance/schema, payload classes and metrics, catalog budgets and native/no-interpreter policy |
| Core policy and effective MCP contracts | `SPR-053` | Named roots, safe-default profiles/capabilities, backend-neutral context, central authorization, effective catalog, audit/correlation, state binding, fingerprints, instructions and catalog CI |
| Operations, Jobs and dependent MCP gates | `SPR-049` | Generic bounded lifecycle and one result store, then Tasks mapping, resource handoff and payload/resource CI |
| Filesystem and search | `SPR-050`–`SPR-052` | Efficient inspection, hashes/content identities, bounded file/tree compare and expected-state verification, MIME/binary/large-result handling, safe edits/plans, bounded search built on final root/profile/backend contracts |
| Process and execution | `SPR-054`–`SPR-057` | Threat models, observation, managed processes, typed allowlisted commands, OS isolation, cursor output |
| System information | `SPR-058` | Scoped and redacted host information |
| Service architecture | `SPR-059` | Multi-mode/IPC contracts, Variant A design, Variant B interfaces, and reuse of the SPR-053 execution-context and audit contracts |
| Native system services | `SPR-060` | Named Pipe/Unix socket, proxy/auto, Windows SCM, Linux systemd, service-account root backend |
| Version 1.0 release gate | `SPR-061` | Multi-client/security validation, CI, cross-project benchmarks, verification and consumption of BL-262 supply-chain evidence, governance, documentation, packaging and rollback checks |

### SPR-048 quality and version prerequisites

The prerequisite chain `BL-260 -> BL-245 -> BL-203` is complete: the
production-server coverage gate and canonical product-version source are active,
and subsequent functional work carries the required SemVer change. Before the
next merge that changes `VERSION`, BL-255 adds controlled candidate artifacts for
the exact repository version and BL-262 follows verified local promotion.
BL-363 then plans persistent public pre-1.0 GitHub prereleases of those exact
verified bytes with `prerelease=true`, a separate explicit publication boundary
and no publication-time rebuild. Stable Version 1.0 gate/publication remains
exclusively BL-263; a prerelease does not imply stable/latest readiness.
BL-257 follows BL-204/212 in SPR-048. BL-213/214 and BL-215 establish payload
and catalog contracts there; dependent BL-216/219/256 run after the SPR-053
profile/context/catalog foundation, while BL-218/258 run after SPR-049 BL-090.

### MCP revision migration sequence (SPR-048 and SPR-053)

The protocol migration is intentionally split so each implementation chat has a bounded owner and the current `2025-11-25` runtime stays truthful until the new path is complete:

1. **BL-207 — completed revision matrix and dispatch contract:** the exact revision matrix and dispatch invariants are defined while production continues to advertise only enabled support.
2. **BL-208 — completed `2026-07-28` stateless candidate path:** per-request metadata, `server/discover`, unsupported-version/result/cache semantics, and exact extension negotiation are implemented behind the production policy without weakening the `2025-11-25` path.
3. **BL-204 / BL-212, then BL-219 after SPR-053:** complete final-spec conformance and JSON Schema 2020-12 first; after effective catalog/context binding, complete fingerprints and safe cache-scope coverage. Only then may the support matrix advertise `2026-07-28`.
4. **BL-209 / BL-211, then conditional BL-210 after SPR-049:** decide final Tasks support and bounded fallback; map internal Operations/Jobs only if selected and after their lifecycle exists.

Prepared Mobile PRs #60, #69, and #149 are inputs to those later chats, not merge-ready authority after this convergence.

Version 1.0 implements service execution Variant A. Variant B's backend boundary and threat model are included so later user workers do not require public tool or domain redesign. Shared-process impersonation is excluded.

After BL-330, rebind local version/main and complete BL-315 convergence. The near-term order is `BL-255 -> BL-262 -> BL-363`, then BL-204 + BL-212 -> BL-257 -> BL-213 -> BL-214 -> BL-215 -> BL-220 -> SPR-053. BL-255 reuses BL-248 verification and exact-byte local promotion; BL-262 supplies evidence and BL-363 publishes the verified immutable candidate through the planned safe publication boundary. SPR-053 establishes roots/profiles/configuration -> BL-236 context -> BL-159 authorization -> BL-110 effective catalog -> BL-166 audit -> BL-239 state binding; then BL-216/219/256 and BL-209/211. SPR-049 establishes BL-084–BL-089 lifecycle -> BL-090 result store -> remaining Operations, conditional BL-210 Tasks mapping, BL-218 handoff and BL-258 payload/resource CI. BL-305/306/315 apply at every closure. Later domains consume these final contracts. [Implementation contract](planning/implementation-contract.md) and [architecture traceability](planning/architecture-traceability.md) carry hard edges and retrofit owners.

## Version 1.0 release boundary

Version 1.0 requires:

- native Windows/Linux artifacts with no interpreter runtime;
- direct STDIO for non-admin use;
- optional local system service using the same binary;
- safe read-only default when no higher-risk profile is selected;
- bounded filesystem/search/process/execution/system domains;
- typed command definitions without a general shell;
- single-transmission payload contracts and large-result handles;
- per-principal quotas and fair service scheduling;
- supported MCP protocol/extension matrix with revision-specific `2025-11-25` and final `2026-07-28` adapter tests, plus schema compatibility tests;
- direct/proxy/service and cross-project efficiency benchmarks;
- checksums, SBOM, provenance, signing plan, rollback, and complete documentation.

See [Version 1.0 Scope and Release Boundary](planning/release-scope.md).

## Post-Version-1.0 direction

Accepted post-Version-1.0 work includes:

- per-user worker execution backend;
- Linux user service and Windows per-user persistent host;
- conditional read/not-modified optimization;
- optional ripgrep adapter and search index;
- deprecated MCP Roots compatibility only for a demonstrated supported `2025-11-25` client need;
- external PID control, process input, and interactive-shell decision gates;
- restricted network information;
- external FlashGate provider/community ecosystem;
- local cloud/placeholder semantics, filesystem watch, and archives (`BL-345`, `BL-347`–`BL-348`);
- allowlisted OS-settings reads, a portable public agent skill, and scoped path compression (`BL-349`–`BL-351`).

The [future tool and adapter plan](planning/tool-adapters.md) records candidate names, domain/core/MCP/OS-adapter ownership, risk, execution form, platform, and milestone. `BL-215` maintains that catalog planning input; it does not promote future tools into the Version 1.0 release.

No post-Version-1.0 item may be pulled into Version 1.0 without an explicit backlog/milestone change and corresponding risk, resource, and documentation review.

## Architectural anchors

- [Architecture](architecture.md)
- [Security model](security.md)
- [Efficiency improvement plan](planning/efficiency.md)
- [Execution identity backends](execution-identity-backends.md)
- [Native runtime and service plan](planning/runtime-modes.md)
- [Authoritative backlog](../BACKLOG.md)
