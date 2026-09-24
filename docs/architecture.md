# FlashGate MCP Architecture

FlashGate MCP is a resource-efficient, cross-platform MCP server for controlled local host operations. Its current implementation provides root-confined filesystem access; Version 1.0 targets bounded search, managed processes, typed command execution, and system information.

## Project identity

FlashGate MCP is the public project name. **Flash** represents low latency, efficient local processing, compact catalogs, and bounded results. **Gate** represents the server-enforced boundary formed by identities, profiles, capabilities, roots, policies, limits, redaction, audit, and OS adapters.

Current identifiers:

| Item | Value |
|---|---|
| Repository | `thomasweidner/flashgate-mcp` |
| Binary | `flashgate-mcp` |
| MCP server implementation name (`serverInfo.name`) | `flashgate` |
| Go module | `github.com/thomasweidner/flashgate-mcp` |
| Short name | FlashGate |

FlashGate is not a remote-shell replacement, web-hosting service, cloud agent, or broad desktop-automation suite.

## Architectural goals

- predictable, testable, fail-closed behavior;
- low startup latency, CPU, memory, I/O, response size, and model-token use;
- deterministic local work instead of model retransmission;
- native Windows and Linux operation without an interpreter runtime;
- direct non-admin STDIO use plus optional local system-service hosting;
- explicit caller identity and effective execution identity;
- strict root, capability, profile, limit, and audit boundaries;
- one repository, one domain core, and one primary binary per platform;
- vendor-neutral open-source core;
- no external MCP framework dependency;
- protocol-version independence below the MCP adapter;
- measurable release and supply-chain quality gates.

## Version 1.0 planning rule

`BACKLOG.md` is authoritative:

- `Planned` tasks are required for Version 1.0;
- `Later` tasks are accepted post-Version-1.0 work;
- Version 1.0 release requires the explicit gate in `BL-263`.

See [Version 1.0 Scope and Release Boundary](version-1-scope-and-release-boundary.md).

## Current state

The current implementation is a layered Go application using MCP JSON-RPC over STDIO. It provides:

- environment-based configuration;
- JSON-RPC validation, routing, initialization, `tools/list`, and `tools/call`;
- eight filesystem tools;
- one required root through `MCP_ROOT`;
- optional read-only tool registration through `MCP_READ_ONLY`;
- central path validation and filesystem abstraction;
- hard protocol, argument, filesystem, and response limits;
- redacted diagnostics on stderr;
- Windows/Linux tests and smoke tests;
- MCP protocol revision `2025-11-25`;
- benchmark measurement for startup, memory, CPU, latency, payload, catalog size, calls, and approximate tokens.

Current successful filesystem tool results are wrapped as one compact JSON text block plus the same object in `structuredContent`. This is the implemented contract, not the final Version 1.0 payload-heavy result architecture. Version 1.0 plans payload classes so large text, binary, search, and process output is transferred once.

Current dependency path:

```text
MCP Client
    |
STDIO
    |
JSON-RPC / MCP server -> router -> handlers -> tools
    |
Filesystem abstraction
    |
PathGuard and current policies
    |
Operating-system filesystem
```

Startup fails closed before runtime exposure. Production roots must be explicit absolute directories and pass existence/type/effective-path policy. `MCP_ROOT=.` remains development-only and requires `MCP_ALLOW_CWD_ROOT=true`.

Not yet implemented:

- multiple named roots;
- general profiles/capabilities;
- safe read-only as the profile-system default;
- search;
- Operations/Job Manager;
- process observation/management;
- typed command execution;
- system-information tools;
- payload-class and large-result resources;
- proxy/auto/system-service modes;
- hybrid execution-identity backends;
- explicit `2026-07-28` stateless revision support alongside the `2025-11-25` initialization path;
- external provider system.

## Accepted Version 1.0 target architecture

```text
                         MCP Adapter
       Version/Extension Negotiation, Schemas, Catalog Budget
                              |
                     Transport Adapter
               STDIO or local proxy/service IPC
                              |
                Authentication / Principal Resolver
                              |
             Profiles, Capabilities, Root Authorization
                              |
           Policy, Limits, Fair Scheduling, Audit/Redaction
                              |
                  Execution Backend Selector
                  |                         |
                  v                         v
       Current/Service Account       Future User Worker
                  |                         |
                  +------------+------------+
                               v
 ┌────────────┬──────────┬─────────┬───────────┬─────────┐
 │ Filesystem │  Search  │ Process │ Execution │ System  │
 └────────────┴──────────┴─────────┴───────────┴─────────┘
           |                optional lifecycle             |
           +-----------------------> Operations / Jobs <---+
                               |
                 Windows and Linux OS Adapters
```

The user-worker backend is represented because interfaces are Version 1.0 work. Its runtime implementation is post-Version 1.0.

## Dependency direction

The local system core is independent of MCP, JSON-RPC, STDIO, Named Pipes, Unix sockets, SCM, systemd, and protocol-specific resource types.

The MCP adapter owns:

- protocol revision and extension negotiation;
- tool registration, schemas, annotations, and instructions;
- MCP-specific input/output DTOs;
- error translation;
- resource-link and Tasks mapping;
- catalog fingerprints and compatible cache/TTL behavior.

Transport/host adapters own:

- STDIO framing;
- local IPC framing and handshake;
- SCM/systemd lifecycle;
- endpoint setup and teardown;
- OS peer identity extraction.

The application layer owns:

- principal mapping;
- profile/root/capability decisions;
- execution-backend selection;
- resource reservations and quotas;
- audit correlation;
- dispatch to domains/jobs.

Domains own their business rules. Platform adapters own OS-specific implementation. Operations/jobs own generic lifecycle only.

Forbidden dependencies:

```text
platform adapters -X-> MCP adapter
domain core       -X-> JSON-RPC/MCP DTOs
domain core       -X-> SCM/systemd/STDIO/IPC
jobs              -X-> domain business meaning
proxy             -X-> authorization authority
client payload    -X-> authoritative identity/backend selection
```

Go components in this repository reuse the core directly. Future MCPs built on FlashGate reuse Go packages/interfaces, not MCP-to-MCP calls. A stable public Go API is not promised before an explicit decision.

## Domain model

### Filesystem

Owns files, directories, metadata, ranged reads, text/media/binary classification, writes, edits, copying, moving, deletion, hashing, fingerprints/content identities, directory size, bounded plans, and Version 1.0 deterministic file/tree comparison plus batch expected-state verification through `BL-346`. BL-346 reuses the `BL-048` hashing/fingerprint and `BL-049` bounded-tree primitives and adds no second hash engine. Accepted post-Version-1.0 work adds local placeholder state (`BL-345`), filesystem event watch (`BL-347`), archive operations (`BL-348`), and scoped path compression (`BL-351`). Shared hashing/tree primitives and Operations/Job lifecycle are reused without transferring filesystem business ownership.

### Search

Owns root-scoped path/name/metadata/content search, include/exclude rules, bounded recursion, pagination, context, and optional later accelerators.

### Process

Owns observation, details, trees, managed process instances, opaque handles, status, stdout/stderr cursors, stop/wait, and lifecycle.

### Execution

Owns typed allowlisted commands, executable identity/path resolution, structured argument validation, working-directory/environment policy, resource limits, and platform isolation.

Execution does not expose a general shell. A command definition can fix subcommands, allow named flags and typed values, bind path arguments to roots, disable network use, and constrain timeout/output.

### System information

Owns only explicitly released OS/architecture/resource facts, scoped disk use, filtered environment fields, and redaction. Network information remains post-Version 1.0. `BL-349` separately owns future allowlisted semantic OS-settings reads with platform backends; a general Registry/sysctl editor or settings mutation is outside that read contract.

### Operations and jobs

Owns generic queued/running/completed/failed/cancelled/timed-out lifecycle, deadlines, cancellation, progress, bounded result storage, TTL, cleanup, and leak protection.

Operations/jobs do not own filesystem, search, process, execution, or system semantics.

### Cross-cutting components

- authentication and principal mapping;
- named roots;
- profiles, capabilities, and risk policy;
- execution-identity backends;
- global/domain/principal limits and fair scheduling;
- diagnostics, audit, trace correlation, and redaction;
- result/resource storage;
- Windows/Linux adapters;
- MCP and IPC adapters.

## Operations and Job Manager

Long work may use an opaque handle such as `op_<opaque-id>`. The handle is server-generated and bound to:

- caller principal;
- profile;
- root;
- execution backend;
- service generation;
- domain/type;
- expiry.

Accepted internal statuses:

- `queued`;
- `running`;
- `completed`;
- `failed`;
- `cancelled`;
- `timed_out`.

The registry retains bounded timestamps, deadline, progress, byte counters, result/resource reference, error category, temporary resources, TTL, and cleanup status.

The transport-neutral operation result store keeps immutable byte snapshots behind
opaque `op_` handles. Every insertion is bounded by explicit per-result, aggregate-byte,
entry-count, and maximum-TTL policy; the store deliberately supplies no implicit
defaults. Retrieval and deletion repeat the complete principal, profile, root,
execution-backend, service-generation, and domain binding check. Missing, expired,
and mismatched results share one unavailable outcome, retrieval never extends expiry,
and expiry sweeps reclaim both entry and byte capacity. Domains remain responsible for
validating and interpreting the stored bytes.

Resource governance includes:

- global concurrency;
- per-domain concurrency;
- per-principal concurrency;
- global/per-principal queue caps;
- fair scheduling;
- runtime/result/temp-data limits;
- shutdown drain/cancel;
- leak detection;
- slow-reader and backpressure handling.

The normal execution unit is a cancellable Go goroutine. A subprocess is justified for an approved external program, hard resource/crash isolation, a different OS identity, or work that cannot be reliably cancelled in-process.

## Managed processes and typed command execution

Managed process handles are the primary identity; PIDs are diagnostic only because of reuse risk.

Process output uses separate bounded stdout/stderr buffers, truncation markers, and cursors. Status, wait, output, and stop operations require the owning execution context.

Typed command definitions resolve a command ID to a server-approved executable path and contract. Tool input is structured; the server creates argv. Standard profiles reject:

- shell command strings;
- response files;
- arbitrary config overrides;
- unapproved hooks/plugins/loaders;
- uncontrolled environment inheritance;
- executable substitution;
- unbounded output/runtime/concurrency.

A future synchronous `run_command` remains a wrapper over the Managed Process Engine, not a second engine.

## Profiles, capabilities, and named roots

Profiles determine tool exposure and policy composition; server-side authorization remains authoritative.

Illustrative capabilities:

```text
filesystem.read
filesystem.write
search.execute
process.observe
process.manage
process.control.external
command.execute
system.read
```

Version 1.0 safe default:

```text
no valid root                    -> startup failure
valid root, no explicit profile  -> safe read-only profile
higher-risk profile              -> explicit validated activation
```

The target uses multiple named roots and model-visible root IDs plus relative paths. Each root can define:

- read/write access;
- size/result/scan/temp limits;
- allowed file types;
- symlink/reparse policy;
- capability mapping;
- process working-directory permission;
- service execution backend.

For Version 1.0 system services, supported backend is `service-account`. A reserved `user-worker` selection fails closed until post-Version-1.0 implementation. Tool calls cannot choose the backend.

Deprecated MCP Roots is not authoritative. Optional compatibility for a demonstrated supported `2025-11-25` client need is post-Version 1.0 and can never override server configuration.

## Hybrid execution identity

Every system-service request distinguishes:

1. **caller principal** — authenticated from OS local IPC and used for authorization, quotas, state ownership, and audit;
2. **effective execution backend** — used for OS operations.

Version 1.0:

- direct STDIO: current process identity;
- system service: dedicated service-account backend;
- user-worker backend: interface/threat model only;
- in-process impersonation: prohibited.

The backend-neutral boundary is implemented before the service so Variant B can be added later without changing domains or MCP tools.

All authorization-sensitive handles, cursors, caches, resources, temporary data, and cancellation rights are bound to principal, profile, root, backend, and service generation.

See [Execution Identity Backends](execution-identity-backends.md) and ADR-015.

## MCP compatibility boundary

The core is protocol independent. The MCP adapter owns all revision-specific wire behavior.

Current implementation remains only `2025-11-25`. The final `2026-07-28` specification is an accepted Version 1.0 target, not an implementation claim.

FlashGate names protocol paths by exact revision:

- **`2025-11-25` initialization path:** `initialize`/`notifications/initialized` behavior and the current wire/result contracts;
- **`2026-07-28` stateless path:** no initialization handshake or protocol-level session; each request carries protocol version/capabilities in `_meta`, the server implements `server/discover`, and results/cache metadata follow the final revision;
- **future revisions:** each receives an explicit protocol-matrix entry and adapter/test delta. No future revision implicitly inherits every `2026-07-28` behavior merely because it is newer.

One binary/process may support multiple exact revisions concurrently while all domain services remain protocol-neutral. Revision dispatch must never become an authorization boundary: OS/transport identity, server policy, principal/root/profile/backend bindings, and current generation remain authoritative.

The migration keeps FlashGate's own Go MCP adapter. Official MCP SDKs and schemas are interoperability/conformance references; adopting a runtime SDK dependency requires a separate dependency/architecture decision.

Version 1.0 protocol work includes:

- an exact supported-revision matrix covering `2025-11-25` and, after implementation, `2026-07-28`;
- revision-specific opening/dispatch and incompatibility behavior;
- `2026-07-28` per-request `_meta`, mandatory `server/discover`, `UnsupportedProtocolVersion`, result `resultType`, response `serverInfo`, and list `ttlMs`/`cacheScope`;
- deterministic tool catalogs and exact-revision fingerprints;
- full JSON Schema 2020-12 validation;
- final Tasks Extension evaluation/mapping without mixing the 2025 experimental lifecycle;
- bounded fallback for clients without optional extensions/resources;
- deprecation awareness for Roots, Sampling, and Logging.

MCP annotations, discovery, client metadata, extension negotiation, and cache state never grant authorization.

## Payload, resource, and token efficiency

Efficiency is a release quality attribute, not informal optimization.

Primary measurements:

- startup and p50/p95 latency;
- idle/peak memory;
- CPU time and allocations;
- calls per workflow;
- bytes scanned/read/written;
- useful payload bytes;
- result and complete response bytes;
- wire amplification;
- approximate tokens per useful byte;
- tool/schema/instruction bytes and tokens;
- proxy/service IPC overhead.

Version 1.0 result architecture uses payload classes:

- small metadata may use compact text/structured parity;
- structured pages use compact metadata and requested fields;
- heavy text appears once plus metadata;
- binary/media uses bounded content or an opaque result/resource handle;
- large/asynchronous results use identity-bound handles, paging/streaming, TTL, and negotiated resource links with bounded fallback.

The server must avoid unnecessary parse/serialize/copy cycles across proxy/service boundaries.

Efficiency mechanisms include pagination, filtering, sorting, field selection, ranges, batching, bounded trees, targeted edits, dry-run, atomic/conditional writes, output cursors, and compact profile-specific server instructions.

Version 1.0 fingerprints/content identities are compact integrity/change evidence, not authorization tokens. `verify_paths` accepts one or more expected-state records and normally returns compact checked/mismatch/indeterminate counts plus bounded requested deltas. When a caller explicitly requires a fresh/strong proof, the filesystem domain re-evaluates the current required metadata/content evidence rather than satisfying that proof solely from a reusable cached result.

Conditional read/not-modified contracts remain post-Version 1.0 under `BL-217`. They may accept a prior content identity/snapshot and omit unchanged payload, but every call still repeats current principal/profile/root/capability/path checks. An internal identity/content cache is an optional bounded implementation optimization: it is scoped to the relevant execution context, safely invalidated, semantically transparent when disabled, and never turns knowledge of a hash into access.

See [Efficiency Improvement Plan](efficiency-improvement-plan.md).

## Native OS adapter policy

Version 1.0 implementation priority:

1. Go standard library;
2. small platform-specific Go adapters;
3. direct Windows/Linux APIs or stable OS virtual filesystems;
4. an allowlisted native OS program invoked without a shell only when security and benchmark evidence justify it;
5. no interpreter-based runtime adapter.

PowerShell and shell scripts may be development, installation, validation, or administrator tooling. They are not normal FlashGate runtime dependencies.

The [future tool and adapter plan](planning/future-tool-adapter-plan.md) is a planning inventory under `BL-215`. Its candidate MCP names stay in the adapter, core operations stay in the owning domain, and Windows/Linux or archive-format details stay in platform adapters. The optional public agent skill (`BL-350`) is one portable guidance surface with a small strategy core and progressively loaded capability references; it is not a runtime dependency, does not duplicate tool schemas, and cannot become an authorization mechanism.

## Runtime and deployment model

One repository and one primary binary remain the baseline.

```text
MCP client -> flashgate-mcp [stdio] -> local core

MCP client -> flashgate-mcp [proxy/auto]
           -> Named Pipe / Unix socket
           -> flashgate-mcp [system service]
           -> principal/policy/backend/core
```

Version 1.0 modes:

- `stdio`;
- `proxy`;
- `auto`;
- system `service`.

Post-Version-1.0 modes:

- Linux user service;
- Windows per-user host;
- internal per-user worker.

The endpoint is local-only. No TCP/HTTP/remote-host listener is included. `auto` never installs, elevates, or falls back after managed authorization/policy/version denial.

## Host-process ownership and lifecycle

This section is an accepted Version 1.0 target; BL-341 remains `Planned` and
the behavior is not implemented yet.

| Mode or role | Authoritative owner | Lifetime | Loss behavior |
|---|---|---|---|
| Direct `stdio` | MCP transport session / client launch | Session-scoped | Bounded shutdown after definitive transport or verified owner loss |
| `proxy` / proxy path of `auto` | Client-side MCP transport session | Session-scoped lightweight edge | Close service connection, cancel connection-owned work, bounded exit |
| Direct fallback of `auto` | MCP transport session / client launch | Session-scoped | Same as direct `stdio` |
| `service` | Windows SCM or Linux systemd | Persistent across clients | Normal client disconnect does not stop the service |
| Future user host | User-host supervisor | Policy-persistent | Supervisor lifecycle |
| Future worker | FlashGate broker/service | Broker-owned | Bounded shutdown after verified control-channel or negotiated lease loss |

Exactly one transport-neutral process-root lifecycle coordinator receives
definitive EOF/transport-close, broken-pipe, negotiated protocol shutdown, OS
stop, verified owner/control-channel loss, and explicitly negotiated lease
expiry signals. The first signal stores a typed reason, stops intake, cancels
session/connection-owned work, and performs bounded drain and cleanup. Host
adapters report signals; Operations/Jobs and Managed Process cleanup remain
with their respective domain owners and are invoked after root cancellation.

The direct baseline is one full FlashGate process per active MCP client
transport. The shared-service baseline is one persistent full service plus one
lightweight edge proxy per active client transport. Multiple processes are not
an error by themselves; workers and managed children must be explicitly
classified.

Lifecycle evidence is bounded and secret-safe. It records instance/mode/PID
plus process-start identity, verified owner state, client identity where
available, service generation or connection ID, activity/request counts,
shutdown time/reason, cleanup result, and one of `ACTIVE_SESSION`,
`EXPECTED_PERSISTENT_SERVICE`, `SHUTTING_DOWN`, `DEFINITELY_ORPHANED`,
`SUSPECTED_STALE`, `EXITED_CLEANLY`, or `EXITED_FORCED`. PID alone, process
age, idle/CPU/request-count heuristics, or singleton assumptions never
authorize termination. Ambiguous live-owner/live-transport cases remain
`SUSPECTED_STALE` and are not automatically killed.

See [ADR-017](adr/017-host-process-ownership-and-lifecycle.md) for the
normative contract and BL-341 in [BACKLOG.md](../BACKLOG.md) for the later
technical implementation.
## Audit and trace architecture

Audit events are bounded, redacted, and correlation-aware. They include both requested caller and effective backend identity where relevant.

Version 1.0 defines:

- immutable event/correlation IDs;
- proxy/service/backend/job/process correlation;
- rotation and retention;
- maximum buffer/disk use;
- slow/full sink behavior;
- log-injection protection;
- redaction before output;
- privacy-safe group/policy identifiers;
- no full file content, credential, token, unredacted command line, or unnecessary host path.

A heavy telemetry runtime is not mandatory. Optional standard trace-context propagation may be supported behind the compact internal correlation model.

## Release and supply-chain architecture

Version 1.0 release evidence includes:

- native Windows/Linux artifacts;
- version/help and platform verification;
- checksums;
- SBOM and dependency inventory;
- build provenance;
- signing plan and configured signing where available;
- reproducible-build comparison;
- pinned/validated CI workflows;
- no silent automatic update;
- atomic update/rollback guidance;
- direct/proxy/service and cross-project benchmark artifacts.

### Build identity and native metadata

`internal/version` is the canonical runtime build-information model.
Controlled builds resolve SemVer, numeric Windows file version, commit,
canonical source time, modified state, Go toolchain, target, and public
architecture once and inject the same values into CLI output, platform
metadata, and a statically inspectable in-binary build manifest.

Windows builds generate a temporary architecture-specific `.syso` resource
through `cmd/versioninfo`, then remove it on success or failure. Linux retains
standard Go/VCS information and Go/ELF build IDs. Static manifest and
normalized icon-frame validation are repository build tools, not runtime
dependencies. Deterministic ZIP or TAR.GZ construction, exact-content/type
verification, checksums, two-build comparison, and leak checks are build-time
concerns and add no runtime interpreter dependency.

See [Build and release metadata](build-and-release-metadata.md) and [File and product metadata decisions](decisions/file-and-product-metadata-decisions.md).

## Open-source, providers, and protocol extensions

The Version 1.0 core and all public FlashGate contracts remain vendor and consumer neutral. They have no organization-specific infrastructure, private control-plane, machine-local workflow, proprietary credential or permission, or particular agent-product prerequisite. Consumer policies may use FlashGate above this boundary but have no product authorization role; historical development provenance is non-operative.

External FlashGate providers/modules are post-Version 1.0. Their future origin or support label never weakens central identity, capability, root, limit, audit, and adapter controls.

MCP protocol extensions are separate negotiated wire features and do not imply a FlashGate provider runtime.

## Planned work

Authoritative tasks and status are in [BACKLOG.md](../BACKLOG.md).

Near-term sequence:

1. complete efficiency/payload/catalog/native-adapter foundations;
2. implement jobs, bounded domains, profiles, and typed execution;
3. define hybrid identity and service contracts;
4. implement Variant A system services;
5. pass Version 1.0 security, performance, supply-chain, compatibility, and documentation gates.

## Post-Version-1.0 decisions

- conditional read/not-modified optimization;
- per-user worker implementation;
- persistent user-scoped hosts;
- ripgrep/index accelerators;
- deprecated MCP Roots compatibility for a demonstrated supported `2025-11-25` client need;
- external PID/input and interactive shell decisions;
- network information;
- provider ecosystem.

Remote transport, product splitting, interpreter-based core operation, or unrestricted dynamic plugins require separate ADRs, threat models, and backlog packages.

## Related documents

- [Version 1.0 scope](version-1-scope-and-release-boundary.md)
- [Roadmap](roadmap.md)
- [Security model](security.md)
- [Efficiency improvement plan](efficiency-improvement-plan.md)
- [Execution identity backends](execution-identity-backends.md)
- [Native runtime and service plan](native-multi-mode-runtime-and-service-plan.md)
- [ADR directory](adr/)
- [ADR-017: Host process ownership and lifecycle](adr/017-host-process-ownership-and-lifecycle.md)
