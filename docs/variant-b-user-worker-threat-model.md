# Variant B User-Worker Contract and Threat Model

## Status and scope

This document defines the Version 1.0 architecture and security contract owned
by `BL-238`. [ADR-015](adr/015-hybrid-service-execution-identity.md) is the
binding execution-identity decision: Variant A (`service-account`) is the only
system-service backend implemented for Version 1.0, Variant B (`user-worker`)
is reserved for later implementation, and shared-process impersonation is
prohibited.

This contract does **not** enable a worker mode, promise a public worker CLI,
select a Windows token API or Linux session manager, or claim native-platform
validation. An enabled `user-worker` root must fail startup until the later
implementation, security, platform, dependency, and release gates are closed.

## Security objective

Variant B may allow an already authenticated and authorized local caller to
execute an operation in a separate FlashGate worker under that caller's OS
identity. It must not let a caller:

- choose an identity, backend, root, profile, or capability in tool input;
- acquire another principal's worker, state, environment, credentials, or
  filesystem access;
- cause the broker to impersonate inside its shared process;
- expand local IPC into a remote execution surface; or
- retain useful authority after logout, revocation, broker restart, worker
  replacement, or execution-context expiry.

The broker remains the policy authority. The worker is an execution boundary,
not an authorization authority.

## Trust boundaries and identities

```text
MCP client
    |
    | local client IPC; OS-authenticated peer
    v
broker service
    | authenticate -> authorize -> reserve quota -> select trusted root policy
    |
    | private, authenticated broker-worker channel
    v
same-binary worker process
    |
    | OS operations under one verified user identity
    v
configured root / OS resources
```

Four identities must remain distinct:

1. **Caller principal**: derived by the broker from local IPC peer credentials.
2. **Policy principal**: the server-created principal/group mapping used for
   roots, profiles, capabilities, ownership, quotas, and audit.
3. **Worker OS identity**: obtained through a platform-native mechanism and
   verified before the worker accepts work.
4. **Broker identity**: the restricted service identity that creates and
   supervises workers but does not become the caller's effective identity.

Payload-supplied identity, group, home, session, backend, and environment
claims are non-authoritative. Every dispatch carries an immutable,
broker-created execution context bound at least to principal, policy mapping,
profile, root, capability, backend, broker generation, worker instance,
request/correlation ID, deadline, and resource budget.

## Worker acquisition contract

### Windows

A later implementation must obtain a real user token through an approved
native Windows mechanism without collecting or storing the user's password.
Before use, it must validate the token's user SID, session and token type,
expected logon state, and required group/restriction attributes against the
broker-authenticated principal and current policy. Token duplication and
inheritance must use the least rights needed. No client-supplied token handle,
SID, session ID, profile path, or environment block is trusted.

The implementation decision must explicitly define behavior for disconnected,
locked, logged-off, elevated, filtered, remote, service, and multiple
concurrent user sessions. Until that platform decision and its native tests
exist, acquisition fails closed rather than using the broker token or a stored
credential.

### Linux

A later implementation must resolve the authenticated local peer through a
trusted account source and launch a separate process with the intended UID,
primary GID, and freshly resolved supplementary groups. It must define whether
a valid login session is required and how account/group changes invalidate a
worker. The broker must not accept UID, GID, group, home, shell, or environment
values from the request.

Dropping from a privileged broker must be ordered and irreversible for the
worker: initialize approved resources, set groups, set GID, set UID, apply
resource/sandbox controls, verify the resulting identity, and only then accept
dispatch. Failure at any step terminates the worker. The later platform design
must decide whether a systemd scope or another native containment mechanism is
required; this contract does not silently select one.

## Environment and credential contract

Workers start from a server-built allowlist, not an inherited broker or client
environment. At minimum:

- remove loader, runtime-hook, proxy, tracing, language-toolchain, temporary
  directory, and search-path variables unless explicitly required and safely
  reconstructed;
- derive home and user-directory values from the verified OS account source;
- use broker-created private runtime and temporary directories with restrictive
  permissions and worker-instance binding;
- close all non-required handles/file descriptors and disable unintended
  inheritance;
- never forward broker secrets, service credentials, auth tokens, arbitrary
  client variables, or client-selected working directories; and
- treat access to user credential stores, network credentials, agents, desktop
  sessions, and interactive UI as separate capabilities requiring an explicit
  later security decision.

The worker binary and configuration are selected by the broker from trusted
installation state. No shell, interpreter, user startup file, plugin search,
response file, or PATH lookup participates in worker launch.

## Private broker-worker protocol

The worker channel is local, private to one broker generation, bounded, and
versioned. It reuses the internal compatibility principles of the local IPC
contract but is a distinct role with mutual endpoint authentication.

The handshake must bind:

- broker generation and unpredictable launch nonce;
- worker instance and expected OS identity;
- protocol version, build identity, and feature set;
- policy principal and backend (`user-worker`);
- maximum frame, concurrency, output, and lifetime limits; and
- both endpoints of the broker-created channel.

The nonce is single-use, short-lived, transmitted through a protected launch
channel rather than command-line arguments or logs, and discarded after the
handshake. A worker accepts commands only from its launching broker. The broker
accepts a worker only after OS identity and handshake verification. Version,
feature, identity, nonce, framing, or generation mismatch closes the channel
and worker without dispatch or fallback.

Messages use correlation IDs and explicit operation, cancellation, result, and
shutdown frames. They carry root IDs and server-resolved operation data, never
model-visible absolute-path authority or reusable credentials. Frames, queued
requests, output, errors, and partial results are bounded. Unknown message or
field semantics fail closed.

## Lifecycle and reuse

A worker belongs to exactly one policy principal and verified OS identity. It
must never be shared across principals, even if numeric IDs, group sets, or
root permissions appear equal.

The future implementation may choose one-operation workers or bounded
same-principal reuse only after benchmark and platform evidence. Reuse must be
keyed by the complete security context, including identity attributes, policy
mapping, backend configuration, broker generation, and compatibility version.
Root changes, group/policy change, logout/token invalidation, broker restart,
worker failure, compatibility change, or expiry makes the worker unavailable;
it is never rebound in place.

Required lifecycle rules:

- the worker becomes dispatchable only after identity, containment, and
  handshake verification;
- startup has a bounded deadline and no automatic privilege-changing fallback;
- normal idle expiry is bounded and graceful, then forcibly contained if the
  deadline expires;
- broker loss closes the private channel and causes bounded worker/descendant
  termination;
- cancellation is scoped to the bound request and cannot address another
  principal's work;
- partial results and temporary resources remain instance-bound and are
  cleaned or marked incomplete after failure; and
- crash restart creates a new instance identity and does not replay mutating
  work automatically.

Only explicitly idempotent, read-only work may ever be retried, and only after
a later contract defines proof that no result or side effect was accepted.

## Resource isolation and quotas

Broker admission control precedes worker dispatch. Global, per-principal,
per-root, per-domain, and per-worker limits remain effective simultaneously.
At minimum the design accounts for process count, queued and running work, CPU,
memory, open handles/file descriptors, child processes, temporary/storage
bytes, result/output bytes, and wall-clock deadlines.

The broker must place workers and descendants in a containment boundary that
supports bounded cleanup. The implementation gate must select and validate the
native mechanism (for example Windows Job Objects or an approved Linux cgroup
or systemd-scope design) rather than claiming limits from Go counters alone.
Containment failure prevents dispatch. Worker pooling must not turn one user's
resource exhaustion into another user's starvation or bypass caller-specific
accounting.

## Failure and audit behavior

Safe external categories distinguish unavailable backend, authentication or
authorization denial, identity acquisition failure, compatibility failure,
capacity exhaustion, timeout/cancellation, worker loss, and internal failure
without exposing tokens, SIDs, group lists, environment values, endpoints,
non-public paths, or raw OS errors.

Audit correlates broker and worker events and records bounded identifiers for:

- caller/policy principal and effective worker identity;
- root, profile, capability, backend, broker generation, and worker instance;
- launch, handshake, dispatch, cancellation, shutdown, crash, and cleanup;
- authorization and resource-limit outcomes; and
- normalized result category and resource counters.

The worker cannot authoritatively rewrite caller identity or authorization
outcomes. Log fields are structured, injection-safe, redacted before emission,
and subject to bounded buffering and retention.

## Threat-to-control matrix

| Threat | Required control |
|---|---|
| Caller selects another user or backend | OS-derived peer identity; trusted root policy; no identity/backend tool fields |
| Broker launches the wrong account | Platform-native acquisition plus post-launch identity verification |
| Stolen or spoofed worker channel | Broker-created private endpoint, single-use nonce, mutual identity/generation handshake |
| Cross-user worker reuse | One-principal ownership and complete-context reuse key; no rebinding |
| Broker secret or environment disclosure | Server-built allowlist, closed inheritance, no client environment forwarding |
| Worker escapes its root/capabilities | Broker authorization plus worker-side execution-context/root enforcement and OS permissions |
| Compromised worker attacks broker | Minimal private protocol, bounded frames, normalized results, restricted broker endpoint |
| Worker or descendants survive broker loss | Native containment, owner-channel loss detection, bounded termination |
| Crash duplicates a write | No automatic mutation replay; incomplete-state recording and reconciliation |
| PID/UID/session reuse aliases stale state | Opaque instance/generation identity plus current OS credential validation |
| Resource exhaustion | Admission control, per-principal fairness, native containment, bounded queues/results |
| Shared-process credential leakage | Separate worker process; Variant C remains prohibited |

Defense in depth requires both broker-side authorization and worker-side
validation of the broker-created execution context. Worker validation does not
make a compromised broker trustworthy; service hardening and least privilege
remain necessary.

## Required validation before implementation can ship

The post-Version-1.0 implementation gate requires native Windows and Linux
evidence for:

- correct and incorrect identity acquisition, session changes, group changes,
  logout/revocation, and unavailable identity sources;
- handshake spoofing, replay, wrong generation/build/version, endpoint theft,
  malformed/oversized frames, and broker/worker crash;
- cross-user denial for roots, handles, cursors, caches, results, temporary
  data, cancellation, logs, and environment;
- sanitized environment and handle/file-descriptor inheritance;
- CPU, memory, process, output, storage, and deadline enforcement plus complete
  descendant cleanup;
- startup, steady-state, per-worker resource, reuse, and crash-recovery costs;
- audit attribution and redaction; and
- confirmation that enabling or requesting an unsupported worker still fails
  closed and never falls back to service-account execution.

Security review must approve the concrete Windows token/session design, Linux
identity/session design, containment mechanisms, reuse policy, and broker-worker
protocol before runtime implementation. Release documentation must describe
platform support, administrative prerequisites, residual risks, rollback, and
the distinction between FlashGate audit attribution and native OS audit.

## Explicit non-goals

- implementing or enabling `user-worker` in Version 1.0;
- remote workers, network listeners, containers, or distributed scheduling;
- password, key, token, or credential collection by FlashGate;
- shared-process impersonation or process-wide credential switching;
- arbitrary command/shell execution through the worker protocol;
- guaranteeing access to a user's GUI desktop, network session, credential
  vault, agents, or interactive applications; and
- changing public MCP tools or result schemas solely for the backend.

## Related documents

- [ADR-015: Hybrid service execution identity](adr/015-hybrid-service-execution-identity.md)
- [Service execution identity backends](execution-identity-backends.md)
- [Native runtime and service plan](native-multi-mode-runtime-and-service-plan.md)
- [Security model](security.md)
- [Testing strategy](testing.md)
- [Authoritative backlog](../BACKLOG.md)
