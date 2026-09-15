# Capabilities, profiles, and named roots

This guide explains the accepted Version 1.0 configuration and security model
for capabilities, profiles, and named roots. It is a target contract, not a
description of functionality already available in the current binary.

## Current implementation and target

The current implementation has one filesystem root configured by `MCP_ROOT`.
It exposes all eight filesystem tools by default, or the three read tools when
`MCP_READ_ONLY=true`. It does not yet implement general profiles, multiple
named roots, root IDs, or capability configuration.

The Version 1.0 target replaces that coarse selection with server-configured
profiles and named roots. The final profile names, configuration format, and
named-root request schema remain owned by their canonical backlog tasks. The
examples below describe behavior and boundaries; they are not configuration
syntax that operators can use today.

## The authorization model

FlashGate separates four related concepts:

- **Capability:** a functional permission such as filesystem reading, process
  observation, or controlled command execution.
- **Profile:** an operator-selected composition of capabilities and policy.
- **Named root:** an operator-configured filesystem boundary identified to MCP
  clients by an opaque, model-visible root ID.
- **Risk policy:** additional conditions for higher-risk or destructive work;
  a risk label is not a universal permission.

Profiles determine which tools appear in `tools/list`, but visibility is only
the first gate. Every `tools/call` is authorized again using the effective
profile, capability, root policy, identity, and limits. A hidden tool cannot be
called through another route, and an MCP annotation never grants permission.

Illustrative functional capabilities are:

```text
filesystem.read
filesystem.write
search.execute
process.observe
process.manage
process.control.external
command.execute
system.read
```

These names are architectural examples until the capability-model backlog work
finalizes the public configuration contract.

## Safe default behavior

The Version 1.0 target is fail-closed:

```text
no valid root                    -> startup failure
valid root, no explicit profile  -> safe read-only profile
higher-risk profile              -> explicit validated activation
```

Write, managed-process, command, destructive, and other higher-risk behavior
is never enabled merely because the binary or an MCP client supports it.
Unknown profiles, capabilities, root IDs, policies, and reserved execution
backends fail closed. Configuration errors must not silently fall back to a
broader profile.

The safe default reduces both authority and catalog size. It does not weaken
the existing requirement for an explicitly configured, valid root.

## Named-root model

Named roots are authoritative server configuration. Clients receive a root ID
and address content relative to that root; public tool results and resource
handles do not reveal the absolute host path.

Each root can define:

- read and write permission;
- file, result, scan, temporary-data, and concurrency limits;
- allowed file or content types;
- symlink and Windows reparse-point policy;
- capability mapping;
- permission to use the root as a process working directory;
- the effective service execution backend.

Root selection narrows authority; it cannot add a capability missing from the
effective profile. Conversely, a capability does not authorize access outside
the selected root or override that root's policy and limits.

For system-service deployments, Version 1.0 supports the `service-account`
backend for administratively granted roots. The `user-worker` backend is
reserved for post-Version 1.0 and must fail closed until implemented. Tool
input cannot choose or override the execution backend.

## Example authorization flow

The following is conceptual rather than final configuration syntax:

```text
operator configuration
  -> validate profile and named roots
  -> calculate effective capabilities
  -> register only eligible tools
  -> receive tools/call with root ID and relative path
  -> resolve authenticated principal and configured execution backend
  -> recheck capability, root policy, path, limits, and operation conditions
  -> execute or return a safe denial
```

For example, `filesystem.read` may expose bounded read and metadata tools for a
root while its root policy still rejects a disallowed file type or reparse
boundary. `filesystem.write` may make a write tool visible while create-only,
replace-only, size, or destructive-operation checks still deny a particular
call.

## Stateful objects and changes

Authorization-sensitive handles, cursors, cached results, temporary data, and
cancellation rights are bound to the caller principal, profile, root,
capability set, execution backend, service instance or generation, and expiry.
They cannot be reused across a configuration or identity boundary. A service
restart invalidates stale state.

Catalogs are deterministic for the effective protocol, profile, capability
set, and relevant configuration. A material policy or configuration change
must invalidate affected catalog/cache state rather than leaving previously
visible tools or handles with broader authority.

## MCP Roots and annotations

FlashGate named roots are not MCP Roots supplied by a client. Deprecated MCP
Roots is not an architectural dependency and, if later supported for legacy
compatibility, cannot override server configuration or grant access.

Likewise, MCP tool annotations describe behavior for clients but are never an
authorization source. The server-side capability and root checks are
authoritative even when catalog metadata is missing, stale, or manipulated.

## Operator and reviewer checklist

Until the target implementation is complete, operators must continue using
`MCP_ROOT` and, when desired, the exact `MCP_READ_ONLY=true` setting. Do not use
the examples in this guide as deployable configuration.

When the target is implemented and finalized, verify at minimum:

1. absent or invalid roots stop startup;
2. no explicit profile selects the safe read-only default;
3. unknown or reserved values fail closed without fallback;
4. `tools/list` matches effective capabilities;
5. direct `tools/call` attempts cannot bypass hidden-tool authorization;
6. root IDs resolve only to server-configured roots and relative paths;
7. root-specific path, type, limit, and reparse policies remain enforced;
8. annotations and client-provided MCP Roots cannot grant authority;
9. state and handles cannot cross principal, profile, root, backend, generation,
   or expiry boundaries; and
10. Windows reparse/ACL and Linux identity behavior is validated on real native
    hosts before release claims are made.

See [ADR-009](adr/009-capability-profiles-and-tool-exposure.md) for the accepted
architecture decision, [Security](security.md) for the complete trust model,
and [Tool conventions](tool-conventions.md) for tool and result contracts.
