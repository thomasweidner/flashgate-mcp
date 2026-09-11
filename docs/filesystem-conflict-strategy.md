# Filesystem conflict strategy

## Status and scope

This document defines the Version 1.0 conflict contract for filesystem
operations. It standardizes how a requested operation behaves when its target
already exists or when an observed path changes during execution. It does not
add a new filesystem tool, a general transaction mechanism, or a free-form
workflow language.

The server, rather than the client, remains responsible for path validation,
authorization, type checks, limits, and final conflict detection. Conflict
options never weaken those checks.

## Closed strategy set

Filesystem operations that expose a conflict option use this closed set:

| Strategy | Meaning |
|---|---|
| `fail` | Do not replace or ignore a conflicting target. Return a conflict error without performing the requested target mutation. |
| `skip` | Leave the conflicting target unchanged and report the operation as skipped. This is valid only for batch or bounded-plan items whose result can report an item outcome. |
| `replace` | Replace an existing target only when the operation-specific type, identity, atomicity, and policy rules permit it. |

`fail` is the default whenever the caller omits a strategy. Unknown values are
invalid arguments. A Boolean `overwrite` field in the current pre-1.0 tools is
a compatibility spelling: `false` maps to `fail` and `true` maps to `replace`.
It does not create a fourth strategy or imply unconditional replacement.

`skip` is deliberately absent from scalar operations. A scalar call that did
nothing would otherwise be easy to mistake for success. Batch and plan results
must identify every skipped item and its safe conflict category.

## Operation matrix

| Operation class | `fail` | `skip` | `replace` |
|---|---|---|---|
| Create file or `write_file` | Reject an existing leaf | Not supported | Replace an existing regular file only; never a directory or unsupported path type |
| Append file | Append to an existing regular file or create an absent leaf; a directory/type conflict fails | Not supported | Not applicable because append does not replace the target |
| Create directory | An existing directory is an idempotent success with `created:false`; a non-directory conflicts | Not supported | Not supported |
| Copy file | Reject an existing target | Scalar: not supported; batch/plan item: leave unchanged and report skipped | Replace an existing regular file only |
| Move file | Reject an existing target | Scalar: not supported; batch/plan item: leave source and target unchanged and report skipped | Replace an existing regular file only, subject to same-volume and identity revalidation |
| Move directory | Reject an existing target | Scalar: not supported; batch/plan item: leave source and target unchanged and report skipped | Not supported; directory merge or replacement is never implied |
| Targeted edit or append | A stale precondition or incompatible path type fails | Not supported | Not a conflict strategy; these operations mutate the authorized existing file under their own preconditions |
| Bounded plan item | Fail the item and apply the plan's explicit stop/continue rule | Leave unchanged and record a skipped item | Apply only the corresponding scalar operation's permitted replacement rules |

Delete has no target-conflict strategy. Its recursive flag controls traversal,
not replacement. A missing delete target and conditional-write precondition
failures retain their operation-specific contracts rather than being treated
as `skip`.

## Conflict classification and results

Conflict detection uses stable machine-readable categories rather than raw
operating-system errors. At minimum, implementations distinguish:

- target already exists;
- source or target type is incompatible;
- source and target identify the same path or filesystem object;
- an observed source or target changed before mutation;
- a directory move targets its own subtree;
- the operation would cross a volume where its contract disallows that move;
- a conditional precondition is stale.

Scalar conflicts return an error and never report success. Batch or plan items
may return `skipped` only when the request explicitly selected `skip`; the
aggregate result records completed, skipped, and failed counts and preserves
deterministic input order. Client-visible details may echo authorized relative
paths but never resolved host paths or raw OS errors.

## Validation and race rules

Preflight validation is not a reservation and cannot guarantee later success.
Immediately before the target mutation, the implementation revalidates the
source and target identities and applies the selected strategy to the current
state. A target appearing after preflight is a conflict; it is not silently
replaced unless `replace` was explicitly selected and all replacement rules
still pass.

Replacement must use the strongest operation-specific publication primitive
available without a delete-then-create window. Temporary files remain inside
the authorized target filesystem and are cleaned up on failure where their
identity is still provable. Cross-volume copy/verify/delete behavior, directory
copy, atomic writes, conditional writes, and bounded plans retain their own
backlog owners and must consume this conflict contract rather than redefine it.

## Required validation

Implementations that consume this contract must test:

- omission and explicit `fail`, plus rejection of unknown strategies;
- every supported and unsupported strategy/operation combination;
- file/directory/unsupported-type conflicts;
- same-path, hard-link or equivalent same-object, and directory-subtree cases;
- target appearance, disappearance, or identity change between preflight and
  mutation;
- preservation of source and target on rejected or skipped operations;
- deterministic per-item outcomes and counters for batch or plan operations;
- safe error categories without absolute paths or raw OS error text;
- Windows and Linux replacement behavior on real filesystems during native
  finalization.
