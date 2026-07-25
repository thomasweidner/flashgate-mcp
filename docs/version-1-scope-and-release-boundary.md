# Version 1.0 Scope and Release Boundary

## Status

**Accepted planning baseline; implementation remains governed by `BACKLOG.md`.**

`BACKLOG.md` is authoritative. In the canonical catalog:

- `Done` means implemented and retained for traceability;
- `Planned` means required for Version 1.0;
- `Later` means accepted post-Version-1.0 work;
- a task may change milestone only through an explicit backlog and documentation decision.

Version 1.0 is reached after `SPR-61` only when `BL-263` passes.

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
- Operations/Job Manager support for bounded long-running work.

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

At this planning date, the implemented revision remains `2025-11-25`. The `2026-07-28` release candidate informs architecture but is not claimed as implemented before the final specification and corresponding FlashGate adapter/tests exist.

Before Version 1.0:

- every advertised revision must have negotiation and compatibility tests;
- the adapter must support the selected stateless-core behavior where applicable;
- Tasks must use the final negotiated extension contract, not a mixture with the 2025 experimental lifecycle;
- deprecated Roots, Sampling, and Logging must not become architectural dependencies;
- JSON Schema 2020-12 validation and deterministic schema snapshots must pass.

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
- complete installation, removal, rollback, operation, and troubleshooting documentation.

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
- legacy MCP Roots compatibility;
- external PID control;
- process input writing;
- interactive shell support;
- privacy-sensitive network information.

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

1. all canonical `Planned` tasks are `Done` or have a documented explicit waiver approved in the release record;
2. all remaining `Later` tasks are correctly described as post-Version-1.0 and are not required by released contracts;
3. direct STDIO remains functional without administrative installation;
4. system service mode implements Variant A only and rejects unsupported Variant B configuration safely;
5. in-process impersonation does not exist;
6. supported MCP revisions and extensions are explicitly documented and tested;
7. performance, payload, token, memory, CPU, concurrency, and security budgets pass;
8. release artifacts and service assets are reproducible, traceable, and rollback-capable;
9. user, administrator, security, and developer documentation matches the released implementation;
10. breaking-change, compatibility, deprecation, and migration policy is published for post-1.0 releases.

## Related documents

- [Authoritative backlog](../BACKLOG.md)
- [Roadmap](roadmap.md)
- [Architecture](architecture.md)
- [Security model](security.md)
- [Efficiency improvement plan](efficiency-improvement-plan.md)
- [Execution identity backends](execution-identity-backends.md)
- [Native runtime and service plan](native-multi-mode-runtime-and-service-plan.md)
- [ADR-0015: Hybrid service execution identity](adr/0015-hybrid-service-execution-identity.md)
