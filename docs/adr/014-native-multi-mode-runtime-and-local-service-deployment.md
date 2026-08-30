# ADR-014: Native Multi-Mode Runtime and Local Service Deployment

## Status

Accepted

## Context

FlashGate MCP is currently a native Go executable launched by an MCP client and connected through JSON-RPC over STDIO. This portable model requires no service installation and is suitable for users without administrative rights. Centrally managed environments also need an optional long-running operating-system service for shared lifecycle control, consistent policy, local multi-client access, recovery, and platform logging.

A service-only design would exclude non-administrative users. Separate portable and service products would duplicate bootstrap, configuration, security, release, and maintenance work. A network listener would enlarge the threat surface and is unnecessary for local host operations.

## Decision

FlashGate MCP will retain one repository, one shared local core, and one primary native executable per platform. The same executable is planned to support distinct process roles selected at startup:

- `stdio`: direct MCP JSON-RPC over STDIO with the core in the same process;
- `proxy`: MCP JSON-RPC over STDIO at the client edge, forwarded to a local FlashGate service;
- `auto`: connect to an authorized compatible managed endpoint when present, otherwise use direct STDIO only under the defined safe-fallback rules;
- `service`: operating-system-managed local host serving authorized local clients through platform IPC.

The current no-argument invocation remains direct STDIO for backward compatibility. Future client templates may recommend an explicit `--mode auto` only after the service deployment passes its security, compatibility, test, packaging, and benchmark gates.

Windows system hosting will integrate with the Service Control Manager and use a local Named Pipe. Linux system hosting will integrate with systemd and use a local Unix Domain Socket. Linux `systemd --user` hosting and a Windows per-user background host are accepted post-Version-1.0 options. They are not required for Version 1.0 because direct STDIO already provides the non-administrative path. A Windows per-user host must not be described as a Windows Service unless registered with SCM.

The local service transport is an internal versioned IPC protocol. It may carry MCP-compatible messages, but it must define framing, handshake, compatibility, limits, cancellation, errors, disconnect behavior, and feature negotiation independently from an implicit unversioned byte stream. It is local-only and does not add TCP, HTTP, remote-host, or firewall-listener support.

`auto` performs no installation, privilege elevation, UAC prompt, or sudo request. It may fall back to direct STDIO when no managed endpoint is present. It must not silently bypass a present endpoint after authorization denial, policy rejection, protocol incompatibility, or a configured fail-closed condition.

The service derives caller identity from operating-system peer information. Proxy-supplied identity claims are never authoritative. The service enforces capabilities, roots, read/write policy, limits, concurrency, operation ownership, redaction, and audit policy for every client. ADR-015 resolves service execution identity: Version 1.0 uses dedicated service-account roots, backend interfaces reserve a future per-user worker path, and in-process impersonation is prohibited. Endpoint ACLs, caller authorization, per-principal quotas, and execution-context binding require approved threat models before implementation.

ADR-017 defines process ownership and bounded lifecycle behavior for direct
STDIO, proxy-edge, and persistent service hosts. It refines lifecycle ownership
without changing the runtime modes accepted by this ADR.

Direct STDIO, proxy edges, and the direct fallback path of `auto` are
session-scoped. SCM/systemd services are persistent and operating-system owned;
a normal client disconnect cancels connection-owned work but does not stop the
service. The shared-service layer reduces repeated heavy FlashGate starts by
leaving one lightweight STDIO edge per active client transport, but it cannot
by itself detect the end of a logical agent, force EOF past retained pipe
handles, or make idle-timeout cleanup safe. ADR-017 and BL-341 are mandatory
lifecycle dependencies of the later BL-226 through BL-231 multi-mode
implementation.
## Rationale

One executable minimizes release and operational complexity while preserving a portable path for every user. Explicit process modes allow platform service integration without coupling the local core to SCM, systemd, Named Pipes, Unix sockets, or STDIO. A local-only proxy keeps compatibility with MCP clients that expect to launch a STDIO server.

The fail-closed fallback rule prevents `auto` from becoming a policy-bypass mechanism. Operating-system-derived peer identity prevents a compromised or user-controlled proxy from asserting privileges it does not have.

## Consequences

### Positive

- Users without administrative rights retain direct portable STDIO operation.
- Managed installations can use SCM/systemd lifecycle, recovery, and platform logging.
- MCP clients continue to see a normal STDIO process in direct, proxy, and auto modes.
- One core and executable reduce duplicated code, release artifacts, and configuration drift.
- Local IPC enables multi-client service use without a remote network listener.
- Direct STDIO preserves non-admin use; persistent user-scoped hosting remains an accepted post-Version-1.0 extension.

### Negative

- The binary gains multiple bootstrap and lifecycle paths.
- Local IPC introduces framing, compatibility, authorization, concurrency, and denial-of-service risks.
- Service installation, upgrade, rollback, and endpoint cleanup require platform-specific implementation and tests.
- A privileged service could become a local privilege-escalation boundary if service-account ACLs, caller authorization, backend selection, or principal-bound state are incorrect.
- Proxy and service versions require an explicit compatibility contract.

## Security Impact

The service boundary is security-sensitive. The implementation must:

- use restrictive Named Pipe ACLs or Unix socket ownership/mode;
- derive caller identity from the operating system;
- reject unauthorized clients before tool dispatch;
- enforce server-side capabilities, roots, limits, concurrency, and operation ownership;
- use Variant A dedicated service-account roots for Version 1.0 and avoid LocalSystem/root as an unreviewed convenience default;
- keep secrets and host paths out of errors and logs;
- bind handles, caches, cancellation, results, resource handles, and temporary resources to caller, profile, root, execution backend, and service generation;
- treat access denial, policy rejection, and version mismatch as fail-closed outcomes;
- keep all endpoints local and add no remote transport;
- require a separate ADR for remote access or a split product architecture.

## Compatibility

- Existing no-argument invocation remains direct STDIO.
- Existing MCP tool contracts are not changed by this decision.
- `stdout` remains exclusively MCP protocol output in `stdio`, `proxy`, and `auto` client-facing operation.
- The internal IPC protocol has its own version handshake and must not assume identical proxy/service builds.
- Managed-mode recommendation is gated by tests and benchmarks against direct STDIO.

## Implementation Guidance

1. Refactor bootstrap so the MCP adapter and core can run behind STDIO or local IPC without duplicating domain logic.
2. Define the CLI, configuration precedence, endpoint discovery, exit codes, and graceful-shutdown contract.
3. Specify a bounded versioned local IPC protocol before implementing platform transports.
4. Implement and test Windows Named Pipes and Linux Unix Domain Sockets with OS-derived peer identity.
5. Implement proxy and auto behavior, including the fail-closed fallback matrix.
6. Add SCM and systemd adapters using the Version 1.0 service-account backend.
7. Keep user-scoped hosting and per-user workers post-Version 1.0.
8. Complete security, multi-client, lifecycle, compatibility, packaging, rollback, supply-chain, and benchmark gates before broad recommendation.

## Decision Gates

The following details remain implementation decisions within this accepted direction:

- exact CLI subcommand syntax for service/user-host management;
- exact endpoint names and configuration paths;
- internal IPC framing/envelope format;
- exact dedicated Windows/Linux service-account form and ACL deployment model within ADR-015;
- future per-user worker launch/session/resource-isolation model;
- post-Version-1.0 Windows per-user host startup mechanism;
- proxy/service compatibility window and upgrade order;
- benchmark thresholds for recommending `auto` or managed mode by default.

## Related Documents

- [ADR-003: Use STDIO Transport](003-stdio-transport.md)
- [ADR-008: Core Reuse, Deployment, and FlashGate Module/Provider Model](008-core-reuse-deployment-and-extension-model.md)
- [ADR-015: Hybrid Service Execution Identity](015-hybrid-service-execution-identity.md)
- [ADR-017: Host Process Ownership and Lifecycle](017-host-process-ownership-and-lifecycle.md)
- [Architecture](../architecture.md)
- [Security model](../security.md)
- [Native multi-mode runtime and service plan](../native-multi-mode-runtime-and-service-plan.md)
- [Authoritative backlog](../../BACKLOG.md)
