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
The immutable `internal/roots.Registry` implements the interface. It checks
the [effective profile, risk ceiling and access of each exact root ID](capability-profiles.md)
before dispatch to the bound filesystem.
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

### BL-375 path exclusions

FlashGate must prove that its own and delegated effects comply with the
exclusion policy, including compound preflight and rechecks before native
effects. Unproved bindings and detectable drift deny the effect. Independently
authorized OS processes can change the namespace or the contents of an open
object under ordinary OS semantics. Each new FlashGate operation checks the
current namespace. BL-375 provides no transactional isolation, snapshots,
or content-version checks. The existing BL-107 and BL-374 checks still apply.

BL-375 adds one server-side `excluded` list of root-relative patterns to
named policies and root overrides. Bootstrap validates all definitions,
including unused policies, and publishes immutable effective lists. An
omitted root field inherits its policy's list. With neither field set, the
list is empty. A present root list replaces the inherited list in full,
including an explicit `[]`.

Match whole paths or any ancestor by component. Within one component,
`*` matches zero or more characters and `?` matches exactly one character.
Neither crosses a path separator. Recursive `**`, character classes, brace
expansion, general glob engines and regular expressions are outside this
grammar. Patterns use unambiguous root-relative components subject to
PathGuard. Reject unsupported or ambiguous patterns, absolute paths and
traversal at bootstrap. A match applies whether the path names a file, directory
or symlink, or does not yet exist. Filename suffixes, trailing separators
and current object types supply no rule classification.

Evaluate affected paths relative to every applicable configured root,
including their verified native names. Each root's effective list applies
independently of the requested root ID. A nested or aliased root whose path
lies in an excluded subtree cannot expose that subtree. Native path
matching follows the actual directory's case semantics.
Typical Windows directories are case-insensitive and typical Linux ext4
directories are case-sensitive. Windows directories with case sensitivity
enabled and other nonstandard filesystems require native verification. Deny
the affected access when the relevant path interpretation cannot be proved.
Root-ID and TOML-key syntax remain independent of native matching.

An existing hardlink is evaluated under the name used to access it.
FlashGate may read or write the shared file content through a name that
passes all applicable controls, even when another name is excluded.
Any future hardlink creation must separately authorize its explicit source
and target. An excluded or out-of-root source cannot be used. The active
configuration retains BL-374's path and object-identity protection,
including content and mutation denials through hardlinks.

Evaluate a symlink entry under its own path and also check the resolved
target path whenever the operation resolves a supported link. Preserve
BL-107's link, reparse, junction and mount rules. The shared filesystem
boundary enforces exclusions for all eight operations and their results,
including recursive effects, before access or mutation. The exclusion list
can only restrict access. Exact-root access, effective profile and risk
checks, resource limits and BL-374 protection each remain in force.

The enforcement boundary reuses the native root, parent and object bindings,
effect coordination and pre-effect checks. Windows binds local NTFS drive
roots by volume GUID, uses verified directory-entry spellings, and queries
case sensitivity on the lookup directory. Linux uses confined native opens
in the ext filesystem of the namespace root and checks the directory
casefold flag. Mount aliases or a directory with unproved comparisons fail
closed. Later consumers must prove the same boundary before activation.

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
access. Public calls carry the ID and path for each operand. Currently, copy
and move require the source and target IDs to match. The registry checks
their combined required rights for that ID before dispatch. The same operation
definitions drive catalog admission and the registry's profile and access
checks. Path confinement and selected-source protection remain independent.

### Planned explicit cross-root operations

[BL-376](../planning/implementation-contract.md#bl-376-explicit-cross-root-verified-file-copy)
adds an explicit, default-deny server policy for a directed root pair and
operation. The first planned consumer extends the existing `copy_path`
route for verified single-file copies while preserving the source. The
registry authorizes the caller, effective profile, risk, and functionality
and checks `files.read` at the source and `files.write` at the target
independently. Both paths keep the existing PathGuard, native effect,
configuration-protection, and exclusion checks, including staging,
verification, and cleanup.

The planned route uses the common filesystem boundary for bounded local byte
transfer, create-only publication, and independent target verification.
Move requires its own permission and verified adoption before source
deletion under BL-058. Directory traversal and compound copy or move remain
with BL-060. Until a variant passes its activation gates, distinct root IDs
continue to be denied for that variant.

## Current implementation

The [Linux openat2 contract](https://man7.org/linux/man-pages/man2/openat2.2.html)
defines no-cross-mount/no-link resolution and unavailable-interface failure.
The [Windows reparse data contract](https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/ntifs/ns-ntifs-_reparse_data_buffer)
defines the actual substitute name and relative flag; a human-readable print
name supplies no traversal authority. [Windows no-follow handle semantics](https://learn.microsoft.com/en-us/windows/win32/fileio/symbolic-link-effects-on-file-systems-functions)
explain the final reparse-point open. The implementation also binds these
choices to the tested Go 1.27.4 standard-library behavior.

BL-107 adds the immutable per-root link boolean to named-policy composition
and removes the global field without a compatibility path. PathGuard owns
relative-chain classification and effective-root confinement. Native execution
binds the root/parent/object before each operation. `os.Root` confines paths
but permits Linux bind mounts, and its ordinary opens follow supported links.
Linux therefore requires targeted `openat2` no-follow/no-cross-mount binding.
Windows opens final objects through an opened parent with the reparse-point
flag and classifies the handle before content I/O or truncation. There is no
weaker fallback, new dependency, second registry or generic execution engine.

Every current open/effect route, recursive inspection and compound operand
uses the binding. Raw link, source/target and parent drift are rechecked at
native boundaries. Configuration protection, root rights, grants, suffix
matching and limits remain separate. Native metadata snapshots preserve the
selected hidden-file policy.
Only implicit tree descendants use structural-name handling; Windows hidden
attribute drift and changed opened-root metadata are denied before I/O/effects.
The fixed Windows metadata procedure and Linux syscall operations have exact
path/platform/operation exceptions
in the existing runtime source gate; generic loading and syscall function
values remain denied. The [BL-107 implementation contract](../planning/implementation-contract.md#bl-107-per-root-symlink-and-reparse-policy)
binds acceptance, unsupported projections and future consumer ownership.

Directory selection for process preflight reuses the guarded existing-path,
native object and parent identity, exclusion, and configuration protection
checks. The [delegated-effects contract](../security.md#process-working-directory-and-delegated-effects)
requires independent enforcement before actual execution.

BL-106 binds the immutable root file-type policy below MCP through this same
interface. Bootstrap resolves its typed fields once; the native filesystem
checks lexical and resolved file names, source/target and recursive effects.
Missing fields preserve unrestricted extension matching; explicit lists and
extensionless overrides retain independent inheritance. Names do not classify
contents or bypass authorization. Unknown object types and unproved link
projections deny safely. See [configuration](../configuration.md#file-types)
and [tool semantics](../tools.md#file-type-enforcement).

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
operations. File replacement uses a confined native rename without a separate target deletion;
there is no cross-volume copy/delete fallback. The residual target-identity
race of the path-based rename API is documented in [security](../security.md)
and [migration](../migration.md#input-and-operation-semantics), not presented
as a stronger atomic identity guarantee.

The [BL-102 consumer contract](../planning/implementation-contract.md#bl-102-filesystem-resource-contract)
applies to every new or materially changed local filesystem consumer,
including filesystem effects of search, managed execution, Native Tools and
providers. Owners prove the same safeguards at their actual open, effect and
result boundaries. The [backlog](../../BACKLOG.md) owns acceptance and status.
