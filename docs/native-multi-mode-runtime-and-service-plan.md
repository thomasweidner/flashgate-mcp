# Native Multi-Mode Runtime and Local Service Plan

## Document status

**Accepted Version 1.0 target architecture; implementation not started.**

The authoritative service tasks are `BL-221` through `BL-244` in [BACKLOG.md](../BACKLOG.md), with post-Version-1.0 tasks `BL-232` and `BL-240`. ADR-0014 defines the multi-mode runtime. ADR-0015 defines the hybrid execution-identity architecture.

Version 1.0 delivers direct STDIO and system-service operation with Variant A service-account roots. It includes the interfaces and threat model for Variant B but does not implement user workers or persistent user-scoped hosts.

## Objective

FlashGate MCP is delivered as one native compiled Go executable per platform:

```text
Windows: flashgate-mcp.exe   native PE executable
Linux:   flashgate-mcp       native ELF executable
```

Normal operation requires no Python, PHP, Node.js, Java, or another interpreter layer. The same executable supports direct MCP operation, a client-facing proxy, automatic local-service discovery, and operating-system-managed service hosting without duplicating the FlashGate core.

## Current state

The implemented runtime is:

```text
MCP client -> child process -> JSON-RPC over STDIO -> FlashGate core
```

Current properties:

- one native Go binary;
- direct STDIO only;
- one process per MCP client launch;
- no service installation;
- no Named Pipe or Unix Domain Socket;
- no remote listener;
- current filesystem operations run under the launching process identity.

All service, proxy, auto, identity-backend, and large-result behavior in this document is planned.

## Version 1.0 runtime modes

### Direct STDIO

```text
flashgate-mcp
flashgate-mcp --mode stdio
```

Requirements:

- no administrative installation;
- no persistent background host;
- MCP JSON-RPC on stdin/stdout;
- diagnostics only on stderr;
- core runs in the same process;
- OS access uses the launching process's current identity;
- current no-argument behavior remains compatible.

Direct STDIO is the Version 1.0 non-admin path.

### Proxy

```text
flashgate-mcp --mode proxy
```

```text
MCP client
    |
MCP JSON-RPC over STDIO
    v
FlashGate proxy
    |
versioned local IPC
    v
FlashGate system service
    |
authorization + execution backend + core
```

Requirements:

- proxy stdout remains MCP-only;
- proxy contains no domain authorization decision;
- caller identity comes from the service's OS peer inspection, not a proxy payload claim;
- request/response sizes, cancellation, deadlines, and reconnect behavior are bounded;
- service version/capability mismatch produces an explicit safe failure;
- public tool contracts do not differ solely because proxy mode is used.

### Automatic mode

```text
flashgate-mcp --mode auto
```

Version 1.0 discovery order:

1. explicitly configured local endpoint;
2. platform system-service endpoint;
3. direct STDIO fallback when permitted and safe.

The post-Version-1.0 user-host endpoint may later be inserted before direct fallback.

Safe fallback rules:

| Condition | Version 1.0 behavior |
|---|---|
| No configured or present service endpoint | Direct STDIO fallback allowed |
| Endpoint path/pipe absent | Direct STDIO fallback allowed unless managed mode is required |
| Compatible endpoint accepts connection | Use proxy path |
| Access denied or caller unauthorized | Fail closed; no direct fallback |
| Service policy denies profile/root/capability | Return denial; no direct fallback |
| Proxy/service protocol incompatible | Fail closed; no direct fallback |
| Required managed endpoint is unhealthy | Fail closed unless an explicit administrator policy permits fallback |
| Service installation/elevation would be required | Never install or elevate; use permitted direct fallback or fail |

`auto` never triggers UAC, sudo, service installation, scheduled-task creation, or another state-changing privilege workflow.

### System service

```text
flashgate-mcp --mode service
```

The operating system starts and supervises the service. It accepts multiple authorized local clients through platform IPC and dispatches every request through the same domains, policies, limits, and OS adapters used by direct mode.

Version 1.0 service execution uses Variant A only:

- dedicated least-privilege service identity;
- administratively configured service roots;
- explicit OS ACL grants to that identity;
- caller-specific FlashGate authorization and quotas;
- no shared-process impersonation;
- unsupported user-worker roots fail closed.

## Post-Version-1.0 runtime modes

### User-scoped persistent host

Accepted later work:

- Linux `systemd --user` service;
- Windows per-user background host using an explicit user-context startup mechanism;
- owner-only local endpoint by default.

This is not required for Version 1.0 because direct STDIO already supports non-admin users. The Windows host must not be described as a Windows Service unless it is registered with SCM.

### User-worker execution backend

Accepted later work uses the same binary in an internal worker role:

```text
flashgate-mcp --mode worker
```

This role is not a public Version 1.0 CLI commitment. Version 1.0 defines only its backend boundary and security contract.

## Platform integration

### Windows system service

Version 1.0 components:

- Service Control Manager lifecycle adapter;
- explicit install, uninstall, start, stop, and status operations;
- local Named Pipe;
- restrictive pipe security descriptor and OS-derived peer identity;
- dedicated service account or reviewed virtual service account;
- explicit service-root ACL documentation;
- graceful bounded drain/cancel on stop;
- recovery policy without restart loops;
- Windows Event Log or approved service-safe destination;
- optional Windows Job Objects for child-process limits.

The service must not run as LocalSystem merely because it simplifies access.

Shared-process caller impersonation is prohibited. Future user identity execution uses a separate worker process if implemented.

### Linux system service

Version 1.0 components:

- systemd unit and optional socket unit only if justified;
- dedicated `flashgate` service user/group;
- Unix Domain Socket in an approved runtime directory;
- restrictive ownership and mode;
- peer UID/GID/PID acquisition;
- stale-socket and single-instance handling;
- journald integration;
- graceful bounded stop;
- hardening such as `NoNewPrivileges`, `PrivateTmp`, `ProtectSystem`, `ProtectHome`, capability minimization, and restricted writable paths;
- cgroup/systemd limits for service and child processes where applicable.

The executable remains a native ELF binary. systemd is supervision, not an interpreter dependency.

## Local IPC contract

The proxy/service protocol is internal, local-only, bounded, and versioned.

It defines:

- framing and maximum frame size;
- handshake and internal protocol version;
- build/service identity and compatible version window;
- feature negotiation;
- request/response correlation;
- cancellation and deadlines;
- streaming, paging, or resource-handle frames;
- per-connection and per-request concurrency;
- overload and quota responses;
- disconnect and partial-result behavior;
- safe normalized errors;
- proxy/service upgrade order;
- audit/trace correlation identifiers;
- graceful shutdown indication;
- service generation used to invalidate stale handles.

The IPC protocol is not a second public MCP contract and does not become a remote RPC framework.

Payload-heavy responses follow the single-transmission rule. The proxy must not deserialize and reserialize large content unnecessarily when a bounded forwarding path is possible.

## Core and adapter boundaries

```text
CLI / STDIO / SCM / systemd / Named Pipe / Unix socket
                         |
                         v
              MCP and local IPC adapters
                         |
                         v
       authentication and principal resolution
                         |
                         v
       policy/profile/root/capability evaluation
                         |
                         v
             execution-backend selector
                |                 |
                v                 v
      current/service account   future user worker
                |                 |
                +--------+--------+
                         v
               domain services and jobs
                         |
                         v
             Windows and Linux OS adapters
```

Rules:

- transport and host adapters contain no domain business logic;
- domain services do not know whether a request arrived through STDIO or IPC;
- MCP types stay above the core boundary;
- local IPC types stay above domain services;
- authorization occurs before backend dispatch;
- backend selection comes from trusted root policy;
- execution context is immutable once dispatch starts;
- no duplicated domain implementation by runtime mode or backend;
- adding Variant B later does not change public tools solely because identity handling changes.

## Hybrid execution identity

### Version 1.0 decision

- Variant A service-account backend: implemented;
- Variant B user-worker backend: interface and threat model only;
- Variant C in-process impersonation: excluded.

Every request has:

- authenticated caller principal;
- groups/policy mapping;
- effective profile and capability;
- root ID;
- selected execution backend;
- service generation;
- correlation ID;
- resource budget and deadline.

### Root policy

Illustrative service configuration:

```toml
[[roots]]
id = "shared-projects"
path = "/srv/flashgate/projects"
execution_backend = "service-account"
profile = "filesystem-write"
```

A future `user-worker` value is reserved but rejected as unsupported when enabled in Version 1.0. Tool input cannot select the backend.

### Variant A request flow

1. Authenticate the local IPC peer through OS credentials.
2. Map the principal to allowed profiles and roots.
3. Resolve the root ID and service-account backend.
4. Apply capability, risk, path, and resource policy.
5. Reserve per-principal and global capacity.
6. Execute under the dedicated service identity.
7. Return bounded result or identity-bound handle.
8. Audit caller and effective backend identity separately.

### Identity-bound state

The following are bound to principal, profile, root, backend, service generation, and expiry:

- operation/job handles;
- managed process handles;
- process/search cursors;
- large-result/resource handles;
- temporary data;
- cancellation rights;
- authorization-sensitive caches;
- audit correlation.

A service restart invalidates stale generation-bound handles.

## Authorization and resource governance

Minimum Version 1.0 requirements:

- derive caller identity from the OS;
- reject identity payload claims;
- allowed users/groups and policy mapping;
- read-only default when no explicit profile is selected;
- server-side capability/root/policy checks;
- per-root execution backend fixed by configuration;
- global, per-domain, and per-principal concurrency limits;
- global and per-principal queue limits;
- fair scheduling and starvation prevention;
- per-principal stored-result, temporary-data, process, and output limits;
- safe overload and rate-limit errors;
- cross-principal handle/cache/resource denial;
- endpoint replacement, race, symlink, and stale-object protection;
- no auto fallback after managed denial;
- no local privilege-escalation path.

## Configuration

Version 1.0 defines:

- CLI, environment, user config, and system config precedence;
- explicit endpoint override and discovery;
- managed-required versus fallback-allowed policy;
- connection, request, shutdown, and retry timeouts;
- system service account/runtime directories;
- allowed users/groups and profile mapping;
- root execution backend;
- root/capability/limit policy;
- log destinations and verbosity;
- proxy/service compatibility window;
- update and rollback behavior.

Secrets do not appear in command-line arguments or logs. Validation finishes before the service accepts clients.

## Logging, audit, and tracing

| Mode | Protocol output | Diagnostics/audit |
|---|---|---|
| Direct STDIO | MCP on stdout | redacted stderr |
| Proxy/auto client process | MCP on stdout | redacted stderr/client log |
| Windows service | local IPC only | Event Log or approved service log |
| Linux service | local IPC only | journald |

Version 1.0 audit lifecycle defines:

- immutable event and correlation IDs;
- caller and effective backend identity fields;
- rotation and retention;
- maximum buffer/disk use;
- behavior when the sink is slow or full;
- log-injection prevention;
- redaction before emission;
- correlation across proxy, service, backend, job, child process, and OS operation.

A mandatory heavy tracing runtime is not required. A compact internal trace context is sufficient; optional standard trace-context propagation may be added without becoming a core dependency.

## CLI planning baseline

Version 1.0 runtime forms:

```text
flashgate-mcp
flashgate-mcp --mode stdio
flashgate-mcp --mode proxy
flashgate-mcp --mode auto
flashgate-mcp --mode service
```

Management covers explicit system scope:

```text
install
uninstall
start
stop
status
```

Exact syntax remains a CLI contract task. State-changing commands are explicit, never auto-elevate, never print secrets, and return stable exit categories.

User-scope management and internal worker syntax are post-Version-1.0.

## Packaging and supply chain

Version 1.0 release outputs include native binaries and applicable service assets:

- systemd unit templates;
- Windows service management support;
- example system configuration;
- uninstall and rollback instructions;
- checksums;
- version/build metadata;
- SBOM and dependency inventory;
- build provenance;
- signing plan and configured signing where available;
- reproducible-build comparison;
- no silent auto-update.

Portable extraction and direct STDIO remain possible without an installer.

## Validation strategy

### Unit and contract tests

- mode parsing and lifecycle;
- endpoint discovery/fallback matrix;
- IPC framing, limits, handshake, and version mismatch;
- principal resolution and payload-identity rejection;
- root/backend selection;
- service-account backend behavior;
- identity-bound handles and resources;
- per-principal quotas and fairness;
- config precedence and redaction;
- shutdown and cancellation;
- unsupported user-worker configuration.

### Windows/Linux integration tests

- Named Pipe ACL and Unix socket ownership/mode;
- OS peer identity;
- service start/stop/restart/status;
- stale endpoint cleanup;
- concurrent clients;
- client disconnect during work;
- service restart and stale handle denial;
- unauthorized and cross-user access;
- FlashGate policy allow but OS service-account ACL deny;
- no elevation from client modes;
- protocol-clean stdout;
- child-process resource limits where supported;
- no in-process impersonation path.

### Compatibility tests

- equal and supported-skew proxy/service versions;
- unsupported version mismatch;
- optional features absent;
- direct/proxy public contract equivalence;
- selected MCP protocol matrix;
- final Tasks Extension behavior if Version 1.0 exposes asynchronous MCP tasks;
- bounded fallback for clients without optional resource-link support.

### Benchmarks

Compare direct STDIO, proxy, and service for:

- startup and first response;
- p50/p95 latency;
- idle and peak memory;
- CPU time and allocations where measurable;
- request/response/IPC bytes;
- useful payload and wire amplification;
- concurrent-client behavior;
- fairness and overload;
- restart/reconnect cost;
- service-account backend overhead.

Version 1.0 also includes the pinned cross-project filesystem benchmark.

## Delivery sequence

### SPR-59 — architecture, contracts, identity, and security

Backlog: `BL-221`–`BL-225`, `BL-233`–`BL-239`, and `BL-166`.

Deliverables:

- multi-mode threat model;
- CLI/lifecycle and IPC specifications;
- configuration/discovery/logging contract;
- hybrid execution-identity architecture;
- backend-neutral interfaces;
- Variant A account/root model;
- Variant B contract and threat model;
- identity-bound state model;
- audit lifecycle and correlation.

### SPR-60 — transports and Version 1.0 system hosts

Backlog: `BL-226`–`BL-231`.

Deliverables:

- Windows Named Pipe;
- Linux Unix Domain Socket;
- proxy mode;
- safe auto mode;
- Windows SCM system service;
- Linux systemd system service;
- Variant A service-account root execution.

`BL-232` user-scoped hosts and `BL-240` user workers remain post-Version-1.0.

### SPR-61 — validation and release readiness

Backlog: `BL-241`–`BL-244` plus applicable CI/release/documentation tasks through `BL-263` and `BL-315`.

Deliverables:

- multi-client/security/lifecycle/compatibility test suite;
- CI and release validation;
- installation/removal/operation/rollback documentation;
- direct/proxy/service and cross-project benchmarks;
- supply-chain evidence;
- Version 1.0 release decision.

## Version 1.0 acceptance criteria

The service work package is Version 1.0-ready only when:

- Windows/Linux artifacts are native binaries with no interpreter runtime;
- one executable supports direct, proxy, auto, and system-service roles;
- no-argument direct STDIO remains compatible;
- direct STDIO works without admin installation;
- Windows SCM and Linux systemd services operate under dedicated restricted identities;
- Named Pipe/Unix socket endpoints enforce restrictive access and OS peer identity;
- Variant A service-account roots are implemented and documented;
- Variant B interfaces/threat model exist but enabled user-worker configuration fails safely;
- shared-process impersonation is absent;
- caller authorization and effective OS execution identity are separate and auditable;
- state, caches, resources, and cancellation are execution-context bound;
- per-principal quotas and fairness pass;
- proxy/service compatibility and fail-closed auto behavior pass;
- payload-heavy results are not duplicated;
- stdout remains protocol-clean;
- packaging, rollback, checksums, SBOM/provenance, and administrator documentation exist;
- direct/proxy/service benchmarks pass accepted budgets.

## Out of scope for Version 1.0

- user-scoped persistent hosts;
- Variant B worker implementation;
- in-process impersonation;
- remote TCP/HTTP access;
- cloud-hosted service;
- automatic elevation or installation;
- interpreter runtime dependency;
- separate independently released service/portable products;
- general plugin IPC framework;
- arbitrary shell/script execution.

Any remote transport, product split, or broader provider-isolation mechanism requires a separate ADR, threat model, backlog package, and release decision.

## Related documents

- [ADR-0014](adr/0014-native-multi-mode-runtime-and-local-service-deployment.md)
- [ADR-0015](adr/0015-hybrid-service-execution-identity.md)
- [Execution identity backends](execution-identity-backends.md)
- [Efficiency improvement plan](efficiency-improvement-plan.md)
- [Version 1.0 scope](version-1-scope-and-release-boundary.md)
- [Security model](security.md)
- [Authoritative backlog](../BACKLOG.md)
