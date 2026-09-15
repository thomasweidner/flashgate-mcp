# Process Policy Model

## Status and scope

This document defines the accepted Version 1.0 policy model for process
observation and server-managed processes. The current runtime remains
filesystem-only; this contract does not expose process tools or implement an OS
process adapter.

The model separates three authority classes:

1. **observe** — inspect policy-approved process facts without changing process
   state;
2. **manage** — start and control only processes created by FlashGate and owned
   by the caller's effective execution context;
3. **external control** — affect an operating-system process that FlashGate did
   not start.

The classes are not interchangeable. Observation never implies management, and
management of a FlashGate-owned process never implies authority over an
external PID.

## Required capabilities and risk classes

| Authority class | Capability | Version 1.0 profile | Risk class |
|---|---|---|---|
| Observe | `process.observe` | Explicitly configurable | Sensitive read |
| Manage | `process.manage` | Explicitly configurable | Stateful execution |
| External control | `process.control.external` | Excluded | High risk, post-Version 1.0 |

Tool visibility and MCP annotations are advisory discovery metadata. Every
request is authorized again server-side against the effective profile,
capability, principal, root, execution backend, service generation, and current
policy. A client-supplied capability, profile, principal, backend, or risk label
has no authority.

## Process identity

### Observed processes

An observed process is identified by a platform adapter using a PID together
with process-start identity or an equivalent verified OS handle. PID-only
identity is insufficient because PIDs can be reused. Before returning details
from multiple OS reads, the adapter verifies that the process identity has not
changed; a missing, inaccessible, exited, or changed identity produces a safe
typed outcome rather than partial data about another process.

Observation results use a request-scoped identity. They do not create a
management handle and do not confer later control rights.

### Managed processes

A managed process receives an opaque, unguessable public handle distinct from
its PID. The registry record binds at least:

- caller principal;
- effective profile and `process.manage` capability;
- applicable named root and working-directory policy;
- effective execution backend and service generation;
- immutable process-start identity or verified OS handle;
- command-definition identity;
- creation time, deadline, lifecycle state, and expiry;
- stdout/stderr ownership and cleanup state.

Status, output, wait, stop, cancellation, and result retrieval repeat all
applicable ownership and policy checks. A service restart invalidates prior
handles. Handle possession alone is never authorization, and a PID is never a
substitute for the handle.

## Policy inputs

The policy evaluator consumes server-derived and validated inputs only:

- authenticated caller principal and connection/session ownership;
- active profile and its effective capabilities;
- operation class and requested fields;
- named-root identity and root policy where paths are involved;
- command definition for managed process startup;
- effective execution backend and service generation;
- global, per-principal, and per-domain quotas;
- platform adapter facts, including stable process identity and access outcome.

For managed startup, the command definition additionally constrains the
absolute executable identity, fixed or typed arguments, working directory,
environment, network behavior, timeout, output, concurrency, and process-tree
cleanup. The server constructs the argument vector; free shell strings,
response files, arbitrary environment inheritance, and client-selected
executables are rejected.

Unknown capabilities, profiles, fields, policy values, command IDs, backends,
or process identities fail closed. Configuration cannot silently fall back to a
broader profile or a more privileged backend.

## Observation policy

Observation exposes only an explicit portable field set. A profile may narrow
the visible process population and fields, but it cannot widen the server's
compiled safety boundary. Command lines, executable paths, usernames,
environments, working directories, and process relationships are sensitive and
are omitted unless the released field contract and policy both permit them.
Secrets and host-specific paths are minimized and redacted before entering a
result, diagnostic, or audit event.

Enumeration and detail reads are bounded by entry, byte, field, time, and
concurrency limits. Per-item disappearance or access denial is represented by a
safe deterministic category. Raw OS errors, unrestricted environment values,
and cross-principal managed-process data are never returned.

## Managed-process policy

Only FlashGate-created processes registered to the requester's complete owning
context are manageable in Version 1.0. Lifecycle operations are idempotent only
where their specific contract says so; races with exit, timeout, cancellation,
restart, and cleanup produce deterministic typed states.

Cancellation and stop target the verified managed instance and its owned
process tree, never an unverified current occupant of a reused PID. Runtime,
output, retained results, process count, queue use, and cleanup are bounded
before allocation and throughout execution. stdout and stderr remain separate,
bounded streams with explicit truncation and cursor semantics.

## External-control boundary

External PID control is not part of Version 1.0 and is absent from standard
profiles. Adding it requires the distinct `process.control.external`
capability, an explicit high-risk policy decision, platform-specific process
identity and authorization rules, a dedicated threat model, and native
Windows/Linux validation. Implementations must not approximate external control
by accepting a PID in a managed-process operation.

Interactive process input and interactive shells remain separate post-Version
1.0 decisions and are not implied by any process capability in this model.

## Audit, errors, and validation

Authorization decisions and lifecycle changes emit bounded, redacted audit
events with correlation, caller principal, operation class, policy outcome,
effective backend, and safe reason category. Audit records do not include raw
command lines, environments, output, host paths, or reusable authorization
material.

Permanent validation must cover:

- denial when each required capability or ownership binding is absent;
- observation/management separation and absence of external control;
- forged handles, cross-principal handles, restart-stale handles, and PID reuse;
- process exit or identity change between platform reads;
- field filtering, redaction, bounded enumeration, and safe OS-error mapping;
- command, argument, environment, working-directory, quota, and backend denial;
- cancellation/exit/timeout/stop races and verified process-tree cleanup;
- equivalent fail-closed policy outcomes on native Windows and Linux.

Fakes may validate pure policy transitions, but they do not replace native
evidence for OS process identity, access checks, process trees, or isolation.

## Related authority

- [ADR-011: Managed Process and Command Execution](adr/011-managed-process-and-command-execution.md)
- [Architecture](architecture.md)
- [Security](security.md)
- [Testing](testing.md)
- [Execution Identity Backends](execution-identity-backends.md)
