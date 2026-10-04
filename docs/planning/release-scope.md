# Version 1.0 Scope and Release Boundary

## Status

**Accepted planning baseline; implementation remains governed by `BACKLOG.md`.**

`BACKLOG.md` is authoritative. In the canonical catalog:

- `Completed` means finished and retained for traceability;
- `Planned` means accepted work awaiting execution, independent of release placement;
- the sprint sequence identifies Version 1.0 work, and the separate Post-1.0 workstream table identifies later release scope;
- a task may change milestone only through an explicit backlog and documentation decision.

Version 1.0 is reached after `SPR-061` only when `BL-263` passes.

The `SPR-048` prerequisite chain `BL-260 -> BL-245 -> BL-203` is complete:
product-code coverage and canonical product versioning are established before
subsequent functional work. BL-255 now owns current-version candidate artifacts
before the next `VERSION`-changing merge; BL-257 follows BL-204 plus BL-212,
BL-213/214 and BL-215 define payload and catalog contracts in SPR-048. BL-216/219/256 require the later SPR-053 effective profile/catalog/context foundation; BL-218/258 require SPR-049 BL-090 result storage. The final public release boundary remains SPR-061/BL-263. After BL-330, complete BL-315 convergence, BL-255 current-version candidate and exact-byte local promotion, and BL-262 supply-chain foundation. Close BL-204/212/257 and early payload/catalog/native policy contracts, then establish SPR-053 named-root/profile/configuration -> BL-236 context -> BL-159 authorization -> BL-110 catalog -> BL-166 audit -> BL-239 state binding. Finish BL-216/219/256 and BL-209/211 there. In SPR-049, complete BL-084–BL-089 lifecycle -> BL-090 store, then conditional BL-210 Tasks mapping, BL-218 resource handoff and BL-258 payload/resource CI. Operations, filesystem, and search consume the final foundation; the matrix's hard predecessors follow this sprint and within-sprint order. [Architecture traceability](architecture-traceability.md) records hard edges and closure obligations.

## Local pre-release promotion

BL-255 may promote a fully BL-248-verified current-version candidate for local use before Version 1.0. Promotion reuses exactly the verified artifact bytes; it never rebuilds or mutates them. Immutable identity includes `VERSION + SourceCommitSHA + ArtifactSHA256`. An optional mutable `current` convenience alias may select the latest verified local set but never replaces immutable identity or silently overwrites different bytes under one identity. The earlier candidate CI may upload transient verification artifacts as evidence. Local promotion itself creates no additional remote upload, Git tag, GitHub Release, remote publication, or external release claim. Root `VERSION` remains the canonical BL-245 source; no SemVer pre-release suffix is generated merely for local promotion. BL-263 owns public release. BL-243 later owns complete installation, removal, and operation guidance across direct, service, and proxy modes; local promotion does not imply those modes already work.

## Version 1.0 product objective

Version 1.0 is a native, local-first, resource-efficient Windows/Linux MCP server that performs controlled host operations without requiring Python, PHP, Node.js, Java, or another interpreter runtime.

The release must provide one primary native executable per platform and preserve direct STDIO operation for users without administrative rights. A centrally installed operating-system service is optional and uses the same executable and core.

The release objective is not maximum tool count. The objective is a compact, measurable, server-enforced host-operation boundary with:

- low startup latency;
- low idle and peak memory;
- bounded CPU and I/O;
- small profile-specific tool catalogs;
- low response and token amplification;
- strict root, capability, identity, and resource controls;
- reproducible Windows/Linux behavior;
- explicit release and supply-chain evidence.

## Required Version 1.0 capabilities

### Native runtime and deployment

Version 1.0 includes:

- native Windows PE and Linux ELF artifacts;
- direct MCP JSON-RPC over STDIO;
- explicit `stdio`, `proxy`, `auto`, and system `service` roles in the same binary;
- Windows SCM system service with local Named Pipe transport;
- Linux systemd system service with local Unix Domain Socket transport;
- no remote TCP/HTTP listener;
- no automatic elevation or service installation;
- direct STDIO as the non-administrative installation-free path.

User-scoped persistent hosts are not required for Version 1.0 because direct STDIO already provides the non-admin path.

### Service execution identity

Version 1.0 adopts the hybrid execution-identity architecture but implements only Variant A:

- **Variant A — service-account roots:** implemented in Version 1.0;
- **Variant B — per-user worker:** interfaces, configuration contract, threat model, and state binding defined in Version 1.0; worker implementation deferred;
- **Variant C — in-process impersonation:** excluded permanently.

Every system-service request has two identities:

1. the authenticated caller principal used for authorization, quotas, ownership, and audit;
2. the effective execution backend used for operating-system access.

Version 1.0 service roots execute through a dedicated least-privilege service account and require explicit OS ACLs. Direct STDIO executes under the process owner's existing OS identity.

### Core domains

Version 1.0 includes the planned bounded implementations for:

- filesystem inspection and controlled modification;
- path, filename, metadata, literal-text, and bounded regular-expression search;
- process observation;
- server-managed process lifecycle;
- typed allowlisted command execution without a general shell;
- explicitly scoped and redacted system information;
- named roots, capability profiles, risk policies, and dynamic tool registration;
- Operations/Job Manager support for bounded long-running work;
- reusable content fingerprints plus bounded file/tree comparison and batch expected-state verification (`BL-048`, `BL-346`).

### Safe defaults

Version 1.0 defaults to:

- no root: startup failure;
- root configured but no explicit profile: safe read-only profile;
- write, process-management, command, and other higher-risk capabilities: explicit activation only;
- no unrestricted shell;
- no external PID control;
- no interactive process input;
- no network-information exposure;
- no deprecated MCP Roots dependency.

### Efficiency contracts

Version 1.0 includes:

- payload-class result contracts;
- single transmission of payload-heavy text, binary, search, and process output;
- separate compact metadata for large payloads;
- opaque principal-bound result/resource handles;
- bounded inline, paging, streaming, or resource-link fallback behavior;
- profile-specific tool-catalog and initialization budgets;
- compact profile-specific server instructions;
- deterministic tool ordering and catalog fingerprints;
- wire-amplification and useful-byte metrics;
- cursor pagination, field selection, ranges, batching, and bounded results;
- direct/proxy/service and cross-project benchmark gates.

Payload-heavy content is transmitted once. Small metadata results may retain text/structured parity when the measured cost remains within the profile budget. Large payload duplication is not accepted.

### Protocol compatibility

Version 1.0 must publish an explicit supported MCP protocol matrix.

The production support matrix currently lists only `2025-11-25`. The
`2026-07-28` stateless adapter is implemented as a compiled-in candidate and
validated with an internal test policy. Public activation remains dependent on
BL-204, BL-212, and BL-219. The Version 1.0 release matrix targets both exact
revisions.

FlashGate documentation and code name protocol paths by exact revision. A later MCP revision therefore receives its own explicit support-matrix entry, adapter delta, and tests.

Before Version 1.0:

- every advertised revision must have exact opening/dispatch, incompatibility, positive, negative, and cross-revision compatibility tests;
- the `2025-11-25` path must preserve its initialization-based behavior while supported;
- the `2026-07-28` path must implement per-request protocol/capability `_meta`, mandatory `server/discover`, `UnsupportedProtocolVersion`, required result `resultType`, result `serverInfo` metadata, and required list-result `ttlMs`/`cacheScope`;
- client self-reported metadata, discovery, extension declarations, and cached catalogs must never become authorization authority;
- Tasks must use the final negotiated `io.modelcontextprotocol/tasks` extension contract, not a mixture with the 2025 experimental lifecycle;
- deprecated Roots, Sampling, and Logging must not become architectural dependencies;
- JSON Schema 2020-12 validation and deterministic schema snapshots must pass;
- the migration must not add an MCP SDK runtime dependency without a separate dependency/architecture decision.

### Security and multi-client service controls

Version 1.0 requires:

- OS-derived local caller identity;
- server-side authorization independent of proxy claims;
- principal/root/profile/backend-bound handles and cached state;
- global, per-domain, and per-principal concurrency and queue limits;
- fair scheduling and deterministic overload behavior;
- separate stdout/stderr limits for managed processes;
- trace/audit correlation across client, proxy, service, backend, job, and OS operation;
- bounded audit lifecycle, rotation, retention, backpressure, and disk-full behavior;
- service endpoint ACL/ownership hardening;
- explicit residual-risk documentation;
- negative cross-user and privilege-escalation tests.

### Release and supply chain

Version 1.0 requires:

- Windows/Linux build, test, race, smoke, schema, response-size, and benchmark gates;
- version/help and release-asset verification;
- checksums;
- SBOM and dependency inventory;
- build provenance;
- signing plan and implemented signing where release infrastructure permits;
- reproducible-build comparison or documented deterministic limitations;
- atomic update/rollback instructions;
- no silent automatic update;
- public security policy, governance, maintainer, and contribution rules;
- complete installation, removal, rollback, operation, and troubleshooting documentation;
- persistent GitHub Release publication for `v<VERSION>` with the already validated platform archives, checksums, release notes, and available supply-chain evidence; transient GitHub Actions artifacts alone are not the public distribution result.

## Explicit post-Version-1.0 work

The following work is accepted but must not delay Version 1.0:

### User isolation and persistent user hosting

- per-user worker implementation for Variant B;
- Linux `systemd --user` hosting;
- Windows per-user background host;
- conditional read/not-modified optimization.

### Optional accelerators and expanded controls

- ripgrep adapter;
- persistent local search index;
- deprecated MCP Roots compatibility for a demonstrated supported `2025-11-25` client need;
- external PID control;
- process input writing;
- interactive shell support;
- privacy-sensitive network information;
- vendor-neutral local cloud/placeholder semantics, filesystem watch, and bounded archives (`BL-345`, `BL-347`–`BL-348`);
- allowlisted OS-settings reads, a portable FlashGate agent skill, and scoped path compression (`BL-349`–`BL-351`).

These Post-1.0 workstream owners do not enlarge the Version 1.0 release gate. Their candidate tool names and core/adapter boundaries are recorded in the [future tool and adapter plan](tool-adapters.md).

### Provider and community ecosystem

- external FlashGate provider contract;
- provider identifiers, metadata, runtime, classification, signing, distribution, and support policy;
- provider-specific security enforcement and documentation;
- Code of Conduct before broader community governance requires it.

### Separate future architecture decisions

The following remain outside the accepted local Version 1.0 architecture and require separate ADRs and threat models:

- remote TCP/HTTP access;
- cloud-hosted FlashGate;
- automatic privilege elevation;
- independently released portable and service products;
- interpreter-based core adapters;
- arbitrary script or workflow languages;
- unrestricted plugin loading.

## Version 1.0 release gate

`BL-263` must verify at minimum:

1. all tasks assigned to Version 1.0 by the sprint sequence are `Completed` or have a documented explicit waiver approved in the release record, and [architecture traceability](architecture-traceability.md) has no ownerless requirement or unresolved Version 1.0 retrofit except an explicitly approved release waiver;
2. all remaining Post-1.0 workstream tasks are correctly described as outside Version 1.0 and are not required by released contracts;
3. direct STDIO remains functional without administrative installation;
4. system service mode implements Variant A only and rejects unsupported Variant B configuration safely;
5. in-process impersonation does not exist;
6. supported MCP revisions and extensions are explicitly documented and tested;
7. performance, payload, token, memory, CPU, concurrency, and security budgets pass;
8. release artifacts and service assets are reproducible, traceable, and rollback-capable;
9. the release publishes persistent GitHub Release `v<VERSION>` assets using the validated archives, checksums, release notes, and available supply-chain evidence rather than relying only on transient workflow artifacts;
10. user, administrator, security, and developer documentation matches the released implementation;
11. breaking-change, compatibility, deprecation, and migration policy is published for post-1.0 releases.

## Related documents

- [Authoritative backlog](../../BACKLOG.md)
- [Roadmap](../roadmap.md)
- [Architecture](../architecture.md)
- [Security model](../security.md)
- [Efficiency improvement plan](efficiency.md)
- [Execution identity backends](../execution-identity-backends.md)
- [Native runtime and service plan](runtime-modes.md)
- [ADR-015: Hybrid service execution identity](../adr/execution-identity.md)
