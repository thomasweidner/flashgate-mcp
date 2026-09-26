# ADR-005: Filesystem abstraction

## Status

Accepted.

## Context

FlashGate exposes security-sensitive filesystem operations to MCP clients.
Distributing filesystem access across tool handlers risks inconsistent or
bypassed policy enforcement. Windows and Linux need one testable domain
boundary that also supports read-only exposure, resource limits, and future
search and audit integration.

## Decision

Production tool filesystem operations go through `internal/fs.FileSystem`.
`internal/fs.LocalFileSystem` implements that interface and delegates path
validation to `internal/security.PathGuard`. Tools depend on the interface,
not direct `os.*` calls. Security-specific path resolution remains owned by
PathGuard; this decision does not move security logic into MCP handlers.

The current interface is defined in
[`internal/fs/filesystem.go`](../../internal/fs/filesystem.go). It exposes
`List`, `Read`, `Stat`, `Write`, `Mkdir`, `Delete`, `Move`, and `Copy`.
`Stat` is the single metadata and missing-path operation; there is no separate
`Exists` method. `Move` covers rename, without a redundant `Rename` alias.
`Mkdir` returns whether the leaf was newly created.

## Consequences

Centralized policy and operation semantics keep tools small, support isolated
unit tests through fake implementations, and provide one integration point
for later filesystem capabilities. The cost is explicit constructor and
interface wiring, plus deliberate overwrite and recursive-operation policy.
Do not introduce interfaces mechanically or move transport types into the
filesystem core.

## Security impact

Every public filesystem tool uses the domain boundary and current PathGuard
policy. Read-only registration, operation limits, redacted errors, and path
confinement remain separate enforced controls. No direct tool-level OS call
may bypass them.

Bootstrap requires an explicit absolute `MCP_ROOT`, validates root existence,
policy, effective resolution, and directory type before constructing the
filesystem, registry, router, and STDIO server, and fails closed for expected
root/configuration failures. The only relative-root exception is the explicit
development opt-in `MCP_ROOT=.` with `MCP_ALLOW_CWD_ROOT=true`.

## Current implementation

The current tools are listed in the [tool reference](../tools.md).
Directory copy is not implemented; `copy_path` copies files only. Directory
creation retains parent creation and reports actual leaf creation state.

`move_path` supports same-volume file and directory movement and rename.
It rejects same-path and same-file aliases, effective self-subtrees, unsafe
overwrite type combinations, changed path identities, and cross-volume
operations. File replacement uses rename without a separate target deletion;
there is no cross-volume copy/delete fallback. The residual target-identity
race of the path-based rename API is documented in [security](../security.md)
and [migration](../migration.md#input-and-operation-semantics), not presented
as a stronger atomic identity guarantee.

Future filesystem, search, audit, and profile changes must retain this domain
boundary. Their acceptance and implementation status are owned by the
[backlog](../../BACKLOG.md), not inferred from this accepted decision.
