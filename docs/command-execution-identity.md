# Command Execution Identity

This document defines the Version 1.0 least-privilege identity contract for
typed commands. It specializes the execution-identity decision in ADR-015 for
children started by the Managed Process Engine; it does not introduce another
execution backend or an impersonation mechanism.

## Identity inheritance by runtime mode

FlashGate starts a typed command with the effective operating-system identity
of the selected execution backend:

| Runtime mode | Child identity |
|---|---|
| Direct STDIO | The existing identity of the FlashGate process launched by the MCP client |
| Version 1.0 system service | The dedicated, restricted FlashGate service account |
| Future user-worker backend | The worker identity, only after the separate Variant B implementation gate |

The authenticated caller principal remains an authorization, ownership,
quota, and audit identity. It is not automatically an operating-system process
identity. A request cannot select an account, supply credentials, request
elevation, or change the configured backend.

## Least-privilege requirements

Before a command can be admitted, FlashGate must authorize the caller and bind
the request to its profile, capability, named root, backend, and service
generation. Command policy must then constrain the executable, arguments,
working directory, environment, runtime, output, concurrency, network policy,
and platform isolation. An operating-system permission never substitutes for
FlashGate authorization.

The child receives only the access already held by the selected backend and
the explicitly allowlisted execution environment. FlashGate does not:

- elevate the child or request an administrative token;
- accept a username, password, token, UID, GID, group list, or equivalent
  credential in a command request;
- switch credentials inside the shared process;
- silently fall back to a more privileged identity when launch or isolation
  fails;
- expose account names, credentials, raw group membership, or token details in
  MCP results or diagnostics.

Direct STDIO cannot remove privileges already held by the launching process.
Operators therefore remain responsible for launching it under an appropriately
restricted account. Service mode provides the Version 1.0 stronger boundary:
the service must not use `LocalSystem` or `root` as a convenience default, and
administrators grant its dedicated account only the roots and operating-system
rights needed by the configured policy.

Platform isolation can reduce a child's usable rights and resources, but a
missing or failed required isolation control is a denial, not permission to run
with broader access. Windows token/Job Object behavior and Linux
UID/GID/cgroup or service-manager behavior require native validation; Cloud or
cross-build evidence cannot establish those outcomes.

## Audit and failure behavior

Every launch decision records bounded, redacted identifiers for both the
authenticated caller and the effective execution backend. It also records the
selected command ID and policy outcome without recording credentials, a raw
environment, or an unrestricted command line.

Identity/backend mismatch, unsupported `user-worker` configuration, requested
identity material, unavailable required isolation, and operating-system access
denial all fail closed. Errors returned over MCP remain categorized and do not
reveal host account or credential details.

## Required validation

The permanent command and execution-identity matrix must prove that:

- direct-mode children retain the deliberately selected launcher identity and
  do not elevate;
- service-mode children run as the configured dedicated service account, not
  the authenticated caller and not `LocalSystem`/`root`;
- payload identity or credential fields cannot influence child creation;
- authorization denial happens before process creation even if the effective
  account has operating-system access;
- required isolation or identity setup failures create no child and do not
  trigger a privileged fallback;
- audit evidence distinguishes caller from effective backend while all public
  results and diagnostics remain secret-safe.

The Windows and Linux assertions above require native finalization on the
supported hosts.

## Related decisions

- [ADR-011: Managed Process and Command Execution](adr/011-managed-process-and-command-execution.md)
- [ADR-015: Hybrid Service Execution Identity](adr/015-hybrid-service-execution-identity.md)
- [Execution Identity Backends](execution-identity-backends.md)
- [Security Model](security.md)

