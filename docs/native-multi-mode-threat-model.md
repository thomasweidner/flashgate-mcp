# Native Multi-Mode Runtime Threat Model

## Status and scope

This document is the Version 1.0 threat model for the planned native
multi-mode runtime. It completes the security-analysis boundary of `BL-221`;
it does not claim that proxy, auto, service, Named Pipe, Unix socket, or
service-account execution is implemented.

The accepted architecture remains one native FlashGate executable with direct
`stdio`, `proxy`, `auto`, and operating-system-managed `service` roles. The
current implementation is direct STDIO only. Remote transport, automatic
elevation, user-scoped persistent hosts, and user-worker execution are outside
this Version 1.0 boundary.

## Assets and security objectives

The runtime must protect:

- configured roots and their contents;
- caller identity, profile, capabilities, quotas, and policy decisions;
- operation, process, cursor, cache, result, and temporary-resource ownership;
- local IPC availability and message integrity;
- service-account credentials and effective OS permissions;
- protocol-clean MCP output and bounded, redacted diagnostics;
- executable, configuration, endpoint, and service-generation identity; and
- host availability during malformed, oversized, concurrent, or abandoned
  work.

The central objective is outcome parity across modes: routing a request through
a proxy or service must not grant access, weaken limits, change public tool
authorization, or expose more host information than direct execution under the
same effective policy.

## Trust boundaries and authoritative inputs

| Boundary | Trusted authority | Untrusted or non-authoritative input |
|---|---|---|
| MCP client to direct STDIO | configured local process identity and server policy | MCP payloads, annotations, requested roots or capabilities |
| MCP client to proxy | proxy framing and bounded transport state only | all client payloads and claimed identity |
| Proxy to system service | OS-authenticated local connection and versioned handshake | proxy-supplied user, group, profile, backend, or authorization claims |
| Service to filesystem/process OS APIs | selected service-account backend and explicit OS grants | caller-provided host paths, executable paths, environment, or identity |
| Runtime to configuration | administrator-controlled source selected by defined precedence | ambiguous, writable, malformed, or conflicting configuration |
| Runtime to stored state | principal/profile/root/backend/generation-bound records | handles, cursors, cache keys, or resource IDs presented by clients |
| Service to logs/audit sinks | bounded redaction and sink policy | payload text, control characters, secrets, host paths, and sink availability |

Named Pipe ACLs and Unix socket ownership/mode are admission controls, not the
complete authorization decision. Every accepted request is authorized again
server-side using OS-derived caller identity. MCP annotations, proxy fields,
endpoint possession, and successful protocol negotiation never authorize an
operation.

## Threats and required controls

### Endpoint spoofing and transport confusion

An attacker may create a look-alike endpoint, race stale endpoint cleanup, or
connect a proxy to an incompatible service. Endpoint discovery must use fixed,
validated local names; platform adapters must reject remote endpoints and
unsafe ownership or ACLs; and the handshake must bind protocol version,
features, framing limits, and service generation. A mismatch fails closed.

The `auto` mode may use direct STDIO only when no managed endpoint is present
or configured and policy permits fallback. Access denial, authentication or
policy failure, incompatible protocol, malformed handshake, and a required but
unhealthy endpoint must not fall through to a less restrictive mode.

### Identity injection and confused deputy behavior

A proxy or request may claim another user, group, profile, root, or execution
backend. The service ignores such claims for authority, derives the caller
from Named Pipe or Unix socket peer information, maps that principal through
server-owned policy, and chooses the execution backend itself. Audit records
distinguish the authenticated caller from the effective service-account
backend.

Version 1.0 never uses LocalSystem or root as a convenience default and never
performs shared-process impersonation. A configured `user-worker` backend
fails closed until the post-Version-1.0 implementation and its separate gates
exist.

### Cross-principal and stale-state reuse

Opaque identifiers can be replayed, guessed, retained across reconnects, or
presented after a service restart. Handles, cursors, caches, cancellation
rights, result resources, and temporary data must be bound to the caller,
profile, root, execution backend, connection where applicable, service
generation, and expiry. Missing or mismatched bindings produce a bounded
denial without revealing whether another principal's object exists.

### Framing, amplification, and resource exhaustion

Local-only IPC is still attacker-controlled input. Framing must reject invalid
lengths, truncated messages, trailing data, unsupported features, duplicate or
unknown correlation identifiers, and messages over the configured limit
before unbounded allocation. Request, response, queue, concurrency, temporary
storage, and per-principal budgets apply before and during work. Cancellation,
deadlines, disconnect cleanup, slow readers, and backpressure have bounded
behavior and cannot leave indefinitely retained connection-owned state.

Payload-heavy content must not be duplicated merely because a proxy boundary
exists. Public MCP limits and result semantics remain consistent across direct
and service paths.

### Lifecycle, disconnect, and orphan hazards

Direct and proxy-edge processes are session-scoped; the system service is
persistent. A normal service-client disconnect cancels connection-owned work
but does not stop the service. PID, age, idle time, request count, a registry
entry, or an apparent singleton is not ownership proof. Automatic termination
requires conclusive transport and ownership evidence and must target only
owned children. Ambiguous cases remain `SUSPECTED_STALE` and are not killed.

The process-root lifecycle coordinator owns bounded shutdown sequencing, while
Operations/Job and Managed Process domains retain their own cleanup authority.
Detailed ownership and orphan controls remain governed by ADR-017 and BL-341.

### Configuration, executable, and environment substitution

Configuration precedence, endpoint names, executable identity, service assets,
and log destinations must be deterministic and validated before use. The
runtime rejects ambiguous or unsafe writable sources and does not search for
or invoke an interpreter. Child execution uses server-resolved allowlisted
native binaries, structured arguments, bounded environments, and root policy;
client-controlled shell strings, loader/plugin paths, and credential-bearing
environment inheritance are excluded.

### Information disclosure and audit abuse

Errors, logs, audit events, endpoint diagnostics, and mismatch responses must
not disclose credentials, unrestricted environments, full payloads,
unnecessary absolute host paths, another principal's state, or sensitive group
membership. Redaction occurs before enqueue or output. Control characters and
untrusted fields are encoded to prevent log injection. Slow, full, or failing
sinks follow explicit bounded backpressure and drop/fail behavior rather than
growing memory or blocking shutdown indefinitely.

## Fail-closed requirements

The runtime must deny or stop safely when it cannot establish any of the
following:

- local endpoint identity and restrictive platform access controls;
- OS-derived caller identity;
- compatible IPC version, framing, and feature negotiation;
- unambiguous configuration and execution-backend selection;
- server-side capability, profile, root, and resource-limit authorization;
- ownership and service-generation binding for stateful identifiers; or
- bounded allocation, queueing, cancellation, cleanup, and diagnostic output.

A denial must not trigger installation, elevation, endpoint replacement,
remote fallback, policy bypass, or a retry through direct mode.

## Required validation evidence

Version 1.0 implementation and finalization must cover at least:

- Named Pipe ACL and Unix socket ownership/mode negatives;
- peer-identity derivation and forged proxy-identity rejection;
- absent endpoint versus denial/incompatibility `auto` fallback cases;
- framing truncation, oversized input, unsupported version, slow reader, queue
  saturation, and cancellation/disconnect cleanup;
- cross-principal, cross-profile, cross-root, cross-backend, expired, and stale
  generation handle/resource rejection;
- service-account OS denial even when FlashGate policy allows, and FlashGate
  denial even when the OS account could access the target;
- proxy/direct public-contract parity and protocol-clean stdout;
- service restart, stale endpoint, concurrent client, bounded shutdown, and
  orphan/lifecycle cases required by ADR-017 and BL-241; and
- redaction, log-injection, audit backpressure, and disk-full behavior.

Real Windows SCM, Named Pipe ACL/caller identity, Linux systemd, Unix peer
credentials, service-account permissions, and native lifecycle evidence cannot
be replaced by cross-builds or synthetic Cloud tests.

## Residual risks and decision boundaries

Even with these controls, a local service concentrates access behind a
long-lived process, platform ACL configuration can be wrong, kernel or OS API
defects remain possible, and bounded denial-of-service within configured limits
cannot be eliminated. These risks require native tests, least-privilege
deployment guidance, observable bounded failures, and explicit release review.

Remote access, a second executable/product, in-process impersonation, or
enabling user workers changes this threat model and requires a separate
architecture and security decision. Concrete endpoint names, IPC framing,
service-account forms, and compatibility windows remain owned by their
specific backlog contracts; this document does not choose them implicitly.

## Related documents

- [ADR-014: Native multi-mode runtime](adr/014-native-multi-mode-runtime-and-local-service-deployment.md)
- [ADR-015: Hybrid service execution identity](adr/015-hybrid-service-execution-identity.md)
- [ADR-017: Host process ownership and lifecycle](adr/017-host-process-ownership-and-lifecycle.md)
- [Native runtime and service plan](native-multi-mode-runtime-and-service-plan.md)
- [Security model](security.md)
- [Testing strategy](testing.md)
- [Authoritative backlog](../BACKLOG.md)
