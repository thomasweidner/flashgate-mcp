# ADR-011: Managed Process and Command Execution

## Status

Accepted

## Context

Process observation and program execution are useful host operations but expose command lines, environments, identities, and powerful control surfaces.

## Decision

Use a Managed Process Registry. Every server-started process receives an opaque process handle used for status, output, waiting, and stopping. PIDs may be diagnostic fields but are not the sole security identity; PID reuse must not cause false association.

Stopping defaults to server-managed processes. External PID control requires a distinct functional capability such as `process.control.external` plus a high-risk policy classification and is excluded from standard profiles. Process data, command lines, and environments are filtered and redacted.

Command execution uses configured executable IDs resolved server-side to allowed absolute program paths. Arguments are separate arrays. Working directories must be within allowed roots. Environment propagation is allowlisted or explicitly defined. stdout and stderr are separate and bounded. Runtime, output, and concurrency are limited.

The Managed Process Engine does not choose or elevate an execution identity on
behalf of a tool caller. It receives the server-created execution context and
launches the child through that context's already-authorized backend:

- direct STDIO uses the FlashGate process's current OS identity;
- the Version 1.0 system service uses its dedicated restricted service account;
- the reserved `user-worker` backend remains unavailable in Version 1.0; and
- shared-process impersonation, caller-supplied credentials, and per-call
  identity selection are prohibited.

Command authorization remains bound to the authenticated caller even when the
child runs as the service account. The executable, working directory, root,
profile, capability, limits, handles, output, cancellation rights, and audit
records remain bound to the same immutable execution context. A child process
must not receive broader filesystem, environment, group, token, capability, or
network access than its approved command definition and execution backend
provide. Platform adapters may use different isolation mechanisms, but they
must fail closed when the configured identity or required restrictions cannot
be established; they must not retry with the server's ambient privileges.

A future `run_command` is a synchronous wrapper over the Managed Process Engine. No second execution engine is permitted. Free shell strings and interactive shells are disabled by default.

## Rationale

Opaque handles provide stronger lifecycle identity than reusable PIDs. A single execution engine prevents divergent policy and cleanup behavior.

## Consequences

- Process observation and control use separate capabilities.
- Windows and Linux adapters must implement equivalent policy outcomes.
- Output needs bounded ring buffers or equivalent streaming storage.
- Server restart behavior must be defined.
- Command execution cannot be used as an identity-selection or elevation API.

## Security Impact

Executable allowlisting, argument separation, root-confined working directories, environment allowlisting, output limits, redaction, and least privilege are mandatory boundaries.

## Implementation Guidance

Threat-model observation and execution separately. Implement registry identity and cleanup before control tools. Test PID reuse assumptions, races, timeouts, redaction, platform isolation, rejection of caller-selected identity data, context-bound ownership, and fail-closed launch behavior when the required backend identity or isolation cannot be established.

## Decision Gates

- external PID control remains separately capable, high-risk classified, and opt-in
- interactive input requires a later risk decision
- interactive shell support requires separate interactive/high-risk policy decisions
- CPU/RAM isolation mechanisms are selected per platform after research
