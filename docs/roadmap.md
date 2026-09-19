# FlashGate MCP Roadmap

`BACKLOG.md` is the only authoritative planning and steering document. This roadmap summarizes sequence and release boundaries without duplicating every canonical task.

## Current direction

FlashGate MCP is the binding project name. The current implementation is a native Go filesystem MCP over STDIO. Version 1.0 expands that core into a bounded local host-operation platform while preserving low startup latency, low RAM/CPU use, compact tool catalogs, and no interpreter runtime.

`Planned` backlog tasks are required for Version 1.0. `Later` tasks are accepted post-Version-1.0 work.

## Version 1.0 sequence

| Phase | Sprint IDs | Direction |
|---|---|---|
| Architecture and identity | `SPR-041` | FlashGate identity, ADR baseline, authoritative backlog consolidation |
| Technical transition | `SPR-042`–`SPR-044` | Technical rename, pre-1.0 filesystem contract cleanup, read-only client preparation |
| Efficiency and shared runtime | `SPR-045`–`SPR-049` | Payload/result architecture, explicit MCP revision matrix (`2025-11-25` initialization path plus final `2026-07-28` stateless path), tool/token/cache budgets, native adapter policy, Operations/Job Manager, quotas, identity-bound state |
| Filesystem and search | `SPR-050`–`SPR-052` | Efficient inspection, MIME/binary/large-result handling, safe edits/plans, bounded search |
| Policy model | `SPR-053` | Named roots, read-only safe default, capabilities, profiles, dynamic tool registration, negative authorization tests |
| Process and execution | `SPR-054`–`SPR-057` | Threat models, observation, managed processes, typed allowlisted commands, OS isolation, cursor output |
| System information | `SPR-058` | Scoped and redacted host information |
| Service architecture | `SPR-059` | Multi-mode/IPC contracts, hybrid execution identity, Variant A design, Variant B interfaces, audit lifecycle |
| Native system services | `SPR-060` | Named Pipe/Unix socket, proxy/auto, Windows SCM, Linux systemd, service-account root backend |
| Version 1.0 release gate | `SPR-061` | Multi-client/security validation, CI, cross-project benchmarks, supply-chain evidence, governance, documentation, packaging, rollback |

### MCP revision migration sequence (`SPR-048`)

The protocol migration is intentionally split so each implementation chat has a bounded owner and the current `2025-11-25` runtime stays truthful until the new path is complete:

1. **BL-207 — revision matrix and dispatch contract:** rebase the prepared protocol-matrix work against this planning; define exact revision entries and dispatch invariants while continuing to advertise only already implemented support.
2. **BL-208 — `2026-07-28` stateless path:** implement per-request metadata, `server/discover`, unsupported-version/result/cache semantics, and exact extension negotiation without weakening the `2025-11-25` path.
3. **BL-204 / BL-212 / BL-219 — conformance, schemas, and cache gates:** complete final-spec conformance, JSON Schema 2020-12, deterministic fingerprints, and safe cache-scope coverage; only then may the support matrix advertise `2026-07-28`.
4. **BL-209 / BL-210 / BL-211 — Tasks decision and mapping:** decide final Tasks support, map internal Operations/Jobs if selected, and define the bounded no-Tasks fallback.

Prepared Mobile PRs #60, #69, and #149 are inputs to those later chats, not merge-ready authority after this convergence.

Version 1.0 implements service execution Variant A. Variant B's backend boundary and threat model are included so later user workers do not require public tool or domain redesign. Shared-process impersonation is excluded.

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

See [Version 1.0 Scope and Release Boundary](version-1-scope-and-release-boundary.md).

## Post-Version-1.0 direction

Accepted post-Version-1.0 work includes:

- per-user worker execution backend;
- Linux user service and Windows per-user persistent host;
- conditional read/not-modified optimization;
- optional ripgrep adapter and search index;
- legacy MCP Roots compatibility only for demonstrated client need;
- external PID control, process input, and interactive-shell decision gates;
- restricted network information;
- external FlashGate provider/community ecosystem.

No post-Version-1.0 item may be pulled into Version 1.0 without an explicit backlog/milestone change and corresponding risk, resource, and documentation review.

## Architectural anchors

- [Architecture](architecture.md)
- [Security model](security.md)
- [Efficiency improvement plan](efficiency-improvement-plan.md)
- [Execution identity backends](execution-identity-backends.md)
- [Native runtime and service plan](native-multi-mode-runtime-and-service-plan.md)
- [Authoritative backlog](../BACKLOG.md)
