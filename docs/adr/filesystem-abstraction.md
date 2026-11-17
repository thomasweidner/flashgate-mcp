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

Production filesystem consumers use `internal/fs.FileSystem`. Each operation
accepts an `fs.Operand` with an exact configured root ID and a relative path.
The immutable `internal/roots.Registry` implements the interface and checks
the effective access of each exact ID before dispatch to the bound filesystem.
The selected `internal/fs.LocalFileSystem`
delegates path resolution to `internal/security.PathGuard`.

The current interface is defined in
[`internal/fs/filesystem.go`](../../internal/fs/filesystem.go). It exposes
`List`, `Read`, `Stat`, `Write`, `Mkdir`, `Delete`, `Move`, and `Copy`.
`Stat` is the single metadata and missing-path operation; there is no separate
`Exists` method. `Move` covers rename, without a redundant `Rename` alias.
`Mkdir` returns whether the leaf was newly created.
`LimitsForRoot` returns an independent effective resource value. A caller's
optional result-admission hook runs after domain validation and before the
first effect; it grants no rights and cannot replace native safeguards.

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

The [named-root contract](../planning/implementation-contract.md#bl-101-named-root-registry)
uses one immutable registry built from the authoritative TOML file. Each
configured root has a stable logical ID and an explicit absolute directory.
Bootstrap validates every root with PathGuard and binds one selected-source
protection to the entire registry before enabling the router and STDIO server.
Overlapping roots share effect coordination. They retain separate effective
access. Public calls carry the ID and path for each operand. Copy and move
require the source and target IDs to match. The registry checks their combined
required rights for that ID before dispatch. Capability checks, path
confinement and selected-source protection remain independent.

## Current implementation

BL-105 extends the same root snapshots with typed resource limits, using
`internal/resource` for exact size conversion and operation budgets. Native
content and scan limits live below MCP; serialized result admission belongs
to the existing result adapter and transport. Mutation receipts are bounded
before effects. Copy and write share one content ceiling. Future backends
prove the same effective limits and hard enforcement before activation; no
productive external process or second filesystem registry is introduced.

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

The [BL-102 consumer contract](../planning/implementation-contract.md#bl-102-filesystem-resource-contract)
applies to every new or materially changed local filesystem consumer,
including filesystem effects of search, managed execution, Native Tools and
providers. Owners prove the same safeguards at their actual open, effect and
result boundaries. The [backlog](../../BACKLOG.md) owns acceptance and status.
