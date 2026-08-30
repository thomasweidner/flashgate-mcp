# ADR-015: Hybrid Service Execution Identity

## Status

Accepted

## Context

ADR-014 accepts a local multi-client FlashGate system service. A system service may have different operating-system permissions from the authenticated client, so FlashGate must decide under which OS identity each requested operation runs.

Three models were considered:

- service-owned roots accessed by a dedicated FlashGate service account;
- separate worker processes running under the authenticated user's identity;
- caller impersonation inside the shared multi-threaded service process.

Service-account roots provide the lowest resource cost and simplest cross-platform implementation but do not automatically inherit personal user permissions. User workers provide native user permissions and OS audit attribution but add process, token/session, IPC, lifecycle, and memory complexity. Shared-process impersonation creates thread/goroutine correctness and privilege-reversion risks and has no clean equivalent Linux model.

FlashGate must preserve its efficiency objective while keeping a future path for user-specific filesystem and credential access.

## Decision

FlashGate adopts a hybrid per-root execution-identity architecture.

### Version 1.0

Version 1.0 implements **Variant A: service-account roots**.

- The system service runs under a dedicated least-privilege service identity.
- Administrators explicitly grant that identity access to configured roots.
- Caller identity is still derived from the local IPC peer and used for FlashGate authorization, profiles, capabilities, quotas, ownership, and audit.
- OS operations execute under the service account's existing identity.
- `LocalSystem` or `root` is not an unreviewed default.
- Native OS audit may show the service account; FlashGate audit records both the requesting caller and effective backend identity.

Version 1.0 also defines and implements backend-neutral interfaces, configuration validation, execution-context binding, and threat-model contracts required for **Variant B: per-user workers**, but does not implement the worker runtime.

### Post-Version 1.0

Variant B may add broker-managed worker processes using the same native FlashGate binary under the authenticated user's OS identity. It requires a separate implementation gate, platform threat models, benchmark evidence, and cross-user security tests.

### Excluded

**Variant C: in-process impersonation is prohibited.** The shared service process will not switch caller credentials around individual operations.

### Per-root selection

The system-service root policy selects the backend administratively. Tool calls cannot choose or change the backend.

Version 1.0 supports `service-account`. A reserved `user-worker` configuration value must fail closed until the later backend is implemented and explicitly enabled.

## Rationale

The hybrid architecture preserves the low RAM, CPU, and latency advantages of one service process for normal shared roots while avoiding an architectural dead end for personal user resources.

Implementing backend boundaries immediately prevents domain, tool, and IPC contracts from becoming coupled to the service account. Deferring worker implementation prevents Version 1.0 from taking on the highest-complexity Windows/Linux identity and process-management work before the primary service path is stable.

Excluding shared-process impersonation provides a clearer cross-platform security model and avoids thread-affine credential leakage across goroutines, callbacks, cleanup, and concurrent requests.

## Consequences

### Positive

- Version 1.0 retains the most resource-efficient service model.
- The same core and tool contracts support direct STDIO and service-account service execution.
- Variant B can be added without redesigning MCP tools or domain services.
- Authorization and OS execution identity are explicit separate concepts.
- Per-root policies make security and administrator expectations visible.
- Cross-principal handles, caches, results, and quotas can be enforced consistently.
- Shared-process impersonation risks are eliminated.

### Negative

- Version 1.0 cannot transparently access arbitrary personal user files or user-bound credentials through the system service.
- Administrators must grant ACL access to the service account for every service root.
- Native filesystem audit attributes Variant A operations to the service account.
- The backend abstraction adds design and test work before only one backend exists.
- Future Variant B remains a substantial Windows/Linux implementation project.

## Security impact

The implementation must:

- derive caller identity from OS local IPC peer credentials;
- reject payload-supplied identity claims;
- authorize caller/root/profile/capability before dispatch;
- select the backend from trusted root configuration;
- bind operations, handles, caches, resources, temporary data, cancellation, and audit to principal, profile, root, backend, and service generation;
- enforce global and per-principal resource limits and fair scheduling;
- use a dedicated restricted service account with minimal ACLs;
- reject unsupported `user-worker` roots in Version 1.0;
- avoid password storage or credential collection for future workers;
- add negative cross-user and privilege-escalation tests;
- keep all service endpoints local-only.

## Compatibility

- Direct STDIO behavior remains unchanged and runs under the launching user's process identity.
- Existing MCP tool contracts do not expose OS account names or backend selection.
- Root IDs and relative paths remain the model-facing path contract.
- Adding Variant B later must not change domain result schemas merely because the execution backend changes.
- Unsupported backend configuration is a startup/configuration failure, not a silent fallback.

## Implementation guidance

1. Define immutable caller and execution context types below the transport adapters.
2. Add a backend registry/selector controlled by root policy.
3. Implement `current-process` for direct mode and `service-account` for Version 1.0 service mode.
4. Bind all stateful identifiers and caches to the execution context.
5. Add per-principal scheduling and quotas before multi-client service release.
6. Document administrator ACL and native-audit implications.
7. Specify the future worker protocol, identity acquisition, lifecycle, and resource isolation without shipping it.
8. Reject all in-process impersonation proposals unless this ADR is explicitly superseded.

## Decision gates for Variant B

- Windows user token/session acquisition without stored passwords;
- Linux UID/GID/supplementary-group and session environment model;
- worker startup and reuse policy;
- Windows Job Object and Linux cgroup/systemd-scope limits;
- broker-worker authentication and compatibility;
- measured per-worker RAM/CPU/start cost;
- cross-user isolation and crash-recovery tests;
- administrator/user documentation and rollback.

## Related documents

- [ADR-014: Native multi-mode runtime](014-native-multi-mode-runtime-and-local-service-deployment.md)
- [Execution identity backends](../execution-identity-backends.md)
- [Native runtime and service plan](../native-multi-mode-runtime-and-service-plan.md)
- [Security model](../security.md)
- [Version 1.0 scope](../version-1-scope-and-release-boundary.md)
- [Authoritative backlog](../../BACKLOG.md)
