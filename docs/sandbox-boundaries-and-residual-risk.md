# Sandbox Boundaries and Residual Risk

This document defines the security boundary that FlashGate calls a sandbox and
records the risk that remains after its controls are applied. It is a security
contract and operator disclosure, not a claim that the current process runs in
a complete operating-system sandbox.

## Current implementation

The implemented server is a local JSON-RPC/MCP process over standard input and
standard output. Its current sandbox is a **root-confined filesystem policy**:

- startup requires one configured filesystem root;
- tool paths are relative to that root;
- lexical and effective-path validation reject traversal and escapes;
- symlink, Windows reparse-point, UNC, and hidden-file policies are enforced;
- filesystem input, traversal, output, and protocol sizes are bounded; and
- client-visible failures and diagnostics avoid raw host paths and operating-
  system errors.

These controls limit which paths the filesystem tools may address. They do not
change the operating-system identity, privileges, network access, namespace,
token, job object, cgroup, container, or mandatory-access-control domain of the
FlashGate process. A direct STDIO server has the permissions and environmental
exposure of the account that launched it.

## Boundary model

FlashGate relies on several nested boundaries. No single layer substitutes for
the others.

| Boundary | Security outcome | Explicit limitation |
|---|---|---|
| MCP and JSON-RPC adapter | Validates message shape, methods, arguments, and bounded wire data | A syntactically valid request is not authorized merely because the adapter accepts it |
| Gate and policy | Applies server-side capabilities, profile, root, domain, limit, and risk-policy decisions | Tool visibility, annotations, client claims, and extension negotiation are not authority |
| Filesystem root | Confines accepted tool paths to an effective configured root and applies link/reparse/hidden/UNC policy | It is not an OS filesystem namespace, chroot, container, or ACL boundary |
| Domain limits | Bounds bytes, entries, recursion, results, time, concurrency, and retained resources | Limits reduce denial-of-service impact but cannot promise zero CPU, memory, disk, or scheduling contention |
| Execution policy | Allows only server-defined executables and typed arguments in the Version 1.0 target | Validation alone does not remove the privileges or ambient access of a child process |
| OS isolation adapter | Applies platform-specific process identity and resource/isolation controls where implemented and proven | Windows and Linux mechanisms differ; unsupported controls must fail closed or be disclosed, never described as equivalent without native evidence |
| Service endpoint | Restricts local IPC and derives caller identity from operating-system peer information | Local-only transport is not authorization, and a shared service remains a local privilege boundary |

## Filesystem residual risk

Root confinement substantially narrows filesystem access, but the standard Go
filesystem interface is path based. Validation and use are separate operating-
system actions, so another process with sufficient access may race by replacing
a validated path component or target. Sensitive operations may add focused
revalidation, and unsupported link/reparse behavior is denied, but FlashGate
does not claim race-free directory-handle-relative traversal on every platform.

The configured root is also not a substitute for host permissions:

- the launching identity may retain access outside the root through code paths
  other than the exposed filesystem tools;
- another local process with sufficient permissions may alter root contents or
  policy-relevant metadata concurrently;
- filesystem ACLs, mounts, network shares, case rules, and reparse behavior vary
  by host and require native validation; and
- allowing symlinks or UNC roots intentionally expands the attack surface even
  though effective-root checks still apply.

Operators should grant the FlashGate identity access only to the required data,
use a dedicated root, keep link/UNC/hidden access disabled unless needed, and
avoid placing secrets in an otherwise shared writable root.

## Process and command residual risk

Process observation, managed processes, and typed command execution are target
Version 1.0 capabilities rather than current filesystem-server behavior. When
implemented, they remain higher-risk operations:

- executable allowlisting and typed arguments reduce injection but cannot make
  a vulnerable approved executable safe;
- a child may read or modify anything allowed to its effective OS identity
  unless a proven platform control removes that access;
- environment variables, inherited handles or descriptors, configuration
  search paths, plugins, response files, and executable replacement are ambient
  inputs that must be restricted explicitly;
- termination of one PID does not prove descendant cleanup, and PID reuse makes
  raw PIDs unsuitable as authorization handles;
- timeout, output, process-count, CPU, and memory controls bound impact but do
  not guarantee immediate cancellation or recovery from kernel and device
  stalls; and
- a no-network policy is only a claim when the selected native isolation
  mechanism enforces it and platform tests prove the result.

No standard profile exposes a general shell. Interactive input, arbitrary
external PID control, and interpreter-backed runtime adapters remain outside
the Version 1.0 boundary.

## Configuration and deployment residual risk

Configuration is trusted operator input, but malformed or unsafe configuration
must fail closed. An operator can nevertheless grant a root, capability,
executable, environment field, or service identity more authority than
intended. Configuration review and least-privilege host permissions are
therefore part of deployment security.

Direct STDIO mode does not authenticate a separate remote caller: its trust
boundary is the local client/process relationship and the launching account.
The planned local service modes add OS-authenticated peer identity and a
dedicated service account, but also create a multi-client privilege boundary.
Restrictive pipe/socket permissions, caller-specific authorization, per-
principal quotas, execution-backend binding, and audit correlation are all
required; proxy-supplied identity is never trusted.

FlashGate does not include a remote TCP/HTTP listener. Local-only operation
reduces remote exposure but does not protect against a malicious or compromised
local principal that already has access to the process or service endpoint.

## Platform assurance boundary

Portable unit tests and cross-builds do not prove native sandbox behavior.
Claims involving the following require tests on the real target platform:

- Windows ACLs, access tokens, reparse points, Named Pipes, Job Objects, SCM,
  service identity, process trees, and restricted-child behavior;
- Linux ownership/mode, Unix peer credentials, `/proc`, cgroups, namespaces,
  resource limits, signals, process trees, and systemd identity/hardening; and
- filesystem-specific atomicity, case handling, mount/share behavior, and race
  outcomes on representative local and network filesystems.

When a requested isolation outcome is unavailable, unimplemented, or cannot be
verified, FlashGate must reject the affected mode or document the weaker
outcome. It must not silently fall back after authorization, policy, identity,
or compatibility rejection.

## Required validation and evidence

Changes to a sandbox boundary require directly affected negative tests and a
review of this residual-risk disclosure. Evidence must distinguish:

1. portable policy and parser tests;
2. simulated adapter tests;
3. cross-build or static artifact checks; and
4. native Windows/Linux execution evidence.

Security acceptance requires attempts to bypass capability and root policy,
escape through links/reparse points, exceed resource limits, inject command or
environment behavior, cross caller/handle ownership, exploit PID reuse, leave
descendants or temporary resources, and trigger unsafe fallback. Logs and
errors must remain bounded, injection-safe, and free of secrets and unnecessary
host paths during those failures.

## Operator summary

FlashGate is a defense-in-depth local gate. Root confinement, server-side
policy, bounded resources, typed execution, least-privilege identities, and
native isolation each reduce a different class of risk. They do not turn an
approved operation into harmless data, eliminate hostile same-host actors, or
replace operating-system permissions and deployment hardening.

Deploy only the required capabilities and roots, run under the least privileged
identity that can perform the approved work, and treat every unproven platform
isolation claim as unavailable.
