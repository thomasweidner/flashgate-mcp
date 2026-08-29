# Service Execution Identity Backends

## Status

**Accepted architecture. Version 1.0 implements Variant A; Variant B is designed but deferred.**

This document refines the native service architecture. ADR-015 is binding. `BACKLOG.md` remains authoritative for implementation status.

## Decision summary

FlashGate uses a hybrid per-root execution-identity architecture:

| Variant | Description | Version 1.0 |
|---|---|---|
| A | OS access through a dedicated FlashGate service account and explicitly granted service roots | Implemented |
| B | Broker-managed worker process under the authenticated user's OS identity | Interface and threat model only |
| C | In-process impersonation inside the shared service process | Permanently excluded |

The hybrid decision separates **caller authorization** from **effective OS execution identity**. A caller may be authorized to use a root even though the operating system operation is performed by the service account. Both identities are visible in audit records.

## Identity model

Every request carries an immutable server-created execution context:

```text
ExecutionContext
├── caller principal ID
├── caller group/security attributes
├── effective FlashGate profile
├── root ID
├── functional capability
├── risk classification
├── execution backend ID
├── service instance/generation
├── request/correlation ID
├── protocol/transport context
└── deadline and resource budget
```

Identity values come from trusted server configuration and OS peer information. They are not accepted from ordinary proxy payload fields.

### Caller principal

The caller principal answers:

- who connected;
- which groups or mapped policy identities apply;
- which profiles, roots, and capabilities are authorized;
- which quotas and ownership rules apply;
- who requested the action for FlashGate audit purposes.

### Effective execution backend

The execution backend answers:

- under which OS identity the operation runs;
- which OS permissions are actually evaluated;
- how process, file, temporary-resource, and child-process limits are applied;
- how the operation is isolated and terminated.

## Runtime-mode matrix

| Runtime path | Caller identity | Effective OS identity | Backend |
|---|---|---|---|
| Direct STDIO | Local process owner | Local process owner | `current-process` |
| Proxy to Version 1.0 system service | Named Pipe/Unix socket peer | Dedicated FlashGate service account | `service-account` |
| Future service user-worker root | Named Pipe/Unix socket peer | Worker launched under that user's identity | `user-worker` |
| Future per-user host | Owning user | Owning user | `current-process` |

`current-process` is not service Variant C. It is ordinary direct execution under the process's existing identity and performs no impersonation.

## Root configuration model

Each system-service root has an administratively controlled execution backend.

Illustrative configuration:

```toml
[[roots]]
id = "shared-projects"
path = "D:\\SharedProjects"
execution_backend = "service-account"
profile = "filesystem-write"

[[roots]]
id = "future-user-home"
path_template = "${USER_HOME}"
execution_backend = "user-worker"
profile = "safe-read"
enabled = false
```

Version 1.0 behavior:

- `service-account` is supported;
- `user-worker` is a reserved planned value and must be rejected as unsupported if enabled;
- callers cannot select or override the backend in a tool call;
- backend changes require administrative configuration and validation;
- root IDs and relative paths remain the public tool contract.

## Backend-neutral architecture

The core must not know Windows tokens, Linux UIDs, SCM, systemd, Named Pipes, or Unix sockets.

Conceptual dependency flow:

```text
OS peer authentication
        |
        v
Principal resolver
        |
        v
Authorization and root/profile policy
        |
        v
Execution-backend selector
        |
        +-----------------------+
        |                       |
        v                       v
Service-account backend     User-worker backend
Version 1.0                 post-Version 1.0
        |                       |
        +-----------+-----------+
                    v
             Domain operation
                    v
             OS platform adapter
```

The exact Go interfaces remain implementation details, but the separation must support:

- backend registration by stable internal ID;
- startup validation of configured roots;
- capability checks before backend dispatch;
- context, deadline, cancellation, and budget propagation;
- normalized safe errors;
- identity-bound handles, result resources, temporary paths, and audit records;
- no domain or MCP contract changes when Variant B is later added.

## Variant A — service-account roots

### Required behavior

The system service runs under a dedicated restricted account:

```text
Windows: dedicated service account or virtual service account
Linux:   dedicated flashgate user/group
```

The account receives OS access only to explicitly configured service roots and required runtime/log directories.

The service must not use `LocalSystem` or `root` as an unreviewed convenience default.

### Authorization flow

```text
1. Derive caller from local IPC peer credentials.
2. Map caller to FlashGate policy and profile.
3. Resolve root ID and verify caller authorization.
4. Verify root uses service-account backend.
5. Apply per-principal and global quotas.
6. Execute under the service account's existing OS identity.
7. Record requested caller and effective service identity separately.
8. Bind any handles/results to the complete execution context.
```

### Advantages retained

- lowest memory and CPU overhead;
- one service process and shared immutable configuration;
- straightforward Windows/Linux implementation;
- simple service hardening;
- strong fit for shared project and application data;
- stable behavior without user login tokens or PAM/session dependencies.

### Known limitations

- personal user files require explicit ACL access for the service account;
- native filesystem audit attributes changes to the service account, not the requesting user;
- user-specific network credentials, certificate stores, encrypted files, and profiles are generally unavailable;
- broad service-account ACLs increase impact if the service is compromised.

These limitations are accepted for Version 1.0 and must be documented for administrators.

## Variant B — per-user worker

Variant B is architected in Version 1.0 but implemented later.

### Planned topology

```text
MCP client
    |
local IPC
    v
FlashGate broker service
    |
internal authenticated worker IPC
    v
flashgate-mcp --mode worker
    |
OS operations under user identity
```

The worker remains the same native FlashGate binary in an internal non-public role. No interpreter is introduced.

### Contract to define in Version 1.0

The Version 1.0 design must specify:

- worker identity and principal binding;
- Windows token/session acquisition constraints;
- Linux UID/GID, supplementary groups, environment, home, and session behavior;
- broker-to-worker authentication and framing;
- worker startup, reuse, idle expiry, restart, and shutdown;
- per-user process, CPU, memory, file, handle, and output quotas;
- child-process containment through Windows Job Objects and Linux cgroups/systemd scopes where available;
- credential and environment minimization;
- crash and partial-operation behavior;
- version and feature compatibility;
- worker log and audit correlation;
- prohibition of cross-user worker reuse;
- failure behavior when no valid user session/token can be obtained.

### Post-Version-1.0 implementation gate

Variant B may be implemented only after:

- Version 1.0 service-account mode is stable and benchmarked;
- the Windows and Linux threat models are approved;
- native user identity acquisition is proven without storing user passwords;
- resource cost per worker is measured;
- worker pooling/reuse does not weaken identity isolation;
- tests demonstrate no cross-user handle, cache, output, environment, or filesystem access.

## Variant C exclusion

FlashGate does not impersonate a caller inside the shared multi-threaded Go service process.

Reasons:

- thread-affine OS impersonation is difficult to reason about with goroutine scheduling;
- reversion failures can contaminate unrelated requests;
- cancellation, nested calls, callbacks, logging, and asynchronous cleanup complicate correctness;
- Linux credential switching in a shared process is not an equivalent safe cross-platform model;
- worker processes provide a clearer OS security and failure boundary.

No future implementation may add shared-process impersonation without replacing ADR-015 through a separate explicit security decision.

## State and cache binding

The following objects must be bound to the full execution context:

- operation/job handles;
- managed process handles;
- process-output cursors;
- search cursors;
- large-result/resource handles;
- temporary files and staging directories;
- cached metadata or authorization-sensitive results;
- cancellation requests;
- audit correlation.

At minimum, ownership includes:

```text
principal + profile + root + execution backend + service generation
```

A service restart changes the generation. Stale handles fail safely. A cache entry created for one principal or backend must not be reused for another unless the data is explicitly classified as identity-independent and the policy decision is revalidated.

## Quotas and fairness

Version 1.0 system-service mode combines:

- global limits;
- per-domain limits;
- per-principal concurrent-operation limits;
- per-principal queue limits;
- per-principal stored-result and temporary-byte limits;
- process and command limits;
- rate limits for repeated denials or malformed requests;
- fair scheduling that prevents a single caller from consuming all workers.

Service-account execution does not make all callers one resource owner. Quotas remain caller-specific.

## Audit model

Every security-relevant event records bounded fields such as:

```text
caller_principal
caller_groups_hash or policy mapping ID
effective_profile
root_id
execution_backend
service_effective_identity
operation/tool
capability decision
request/correlation ID
result category
resource counters
duration
```

Sensitive raw group memberships, tokens, full paths, command lines, file contents, and credentials remain redacted or omitted.

For Variant A, administrators must understand that native OS filesystem auditing sees the service account. FlashGate audit supplies the requesting caller attribution.

## Testing requirements

Version 1.0 tests include:

- service-account root allow and deny cases;
- caller allowed/denied independently of service-account ACL access;
- service-account ACL denied even when FlashGate policy allows;
- cross-principal handle/result/cache denial;
- service restart invalidates stale state;
- per-principal quota and fairness tests;
- audit contains both caller and effective backend identity;
- unsupported `user-worker` configuration fails closed;
- no impersonation APIs or shared-process credential switching path exists;
- Windows and Linux endpoint identity cannot be spoofed by payload fields.

Post-Version-1.0 Variant B tests additionally cover worker identity, session/token behavior, groups, environment, crash isolation, resource isolation, and cross-user denial.

## Related documents

- [ADR-015](adr/015-hybrid-service-execution-identity.md)
- [Native runtime and service plan](native-multi-mode-runtime-and-service-plan.md)
- [Security model](security.md)
- [Architecture](architecture.md)
- [Version 1.0 scope](version-1-scope-and-release-boundary.md)
- [Authoritative backlog](../BACKLOG.md)
