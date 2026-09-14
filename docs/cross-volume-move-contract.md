# Cross-volume move contract

## Status and scope

This document defines the Version 1.0 target behavior for `move_path` when the source and target are on different filesystem volumes. The current runtime still rejects such requests with `unsupported_operation`; implementation of the copy/verify/delete workflow, directory traversal, and job integration remains separate work.

The operation is a bounded, resumable **copy, verify, then delete** workflow. It is not an atomic rename and must never be reported as one. The source remains authoritative until verification succeeds and source deletion starts.

## Preconditions and policy

Before copying any content, the server must:

1. resolve and authorize source and target through the same root, profile, capability, path-type, symlink/reparse, and execution-identity rules used by other filesystem operations;
2. reject same-path, same-file, directory-into-own-subtree, unsupported target type, and disallowed overwrite cases;
3. determine that a same-volume rename is unavailable because the endpoints are on different volumes;
4. preflight server-owned entry, byte, depth, temporary-space, duration, and result limits; and
5. capture source identities and metadata needed for later revalidation.

Client estimates cannot raise server limits. A preflight estimate is not a reservation and does not replace runtime accounting or revalidation.

## Staging and publication

Content is copied into a uniquely named, operation-owned staging location on the **target volume** and below the authorized target root. Staging names and operation handles are opaque and must not expose host paths. The server writes regular files without following a replacement symlink or reparse point and accounts for every entry and byte while traversing.

After the staged copy is complete, the server verifies it against the captured source state. Regular-file verification requires byte length and a server-selected cryptographic content fingerprint. Directory verification requires the complete bounded entry inventory, entry types, relative paths, file lengths, and per-file fingerprints. Portable metadata is applied and verified only where the active policy and platform contract support it.

Only a fully verified staging tree may be published to the target. Publication uses the same target-conflict and overwrite rules as other filesystem writes, with immediate target revalidation. If target publication cannot be atomic on the target volume, the operation must expose that limitation and must not claim an all-or-nothing result.

## Source deletion and partial completion

The source is revalidated immediately before deletion. A changed source stops the operation without deleting it. Deletion proceeds only after target publication and verification have succeeded. For directories, deletion is bottom-up and each entry is revalidated before removal.

Once source deletion begins, rollback is not promised. Failure or cancellation therefore returns a terminal partial-completion result containing bounded, machine-readable phase and counters, including whether the target was published, whether source deletion started, and whether source entries remain. The result must never reduce this state to a generic success or imply that the source is intact when deletion may have begun.

## Cancellation, cleanup, and recovery

Cancellation is checked during traversal, copying, verification, publication, and deletion. Before publication, cancellation preserves the source and removes only staging resources proven to belong to the operation. After publication, the published target is retained and the result reports the exact phase and remaining source state; automatic deletion does not continue in the background.

Cleanup is bounded and identity-checked. The server must not recursively remove an unverified path merely because its name resembles a staging path. Failed cleanup is recorded as an incomplete temporary resource for the Operations/Job cleanup owner; it is not hidden by the primary operation error.

Process or service restart does not silently resume destructive work. A later resume facility may continue only from identity-bound, versioned state after revalidating the caller, root, profile, execution backend, source, target, staging data, and limits. Otherwise the operation remains terminal and requires an explicit new request.

## Result and error requirements

The eventual result contract must distinguish at least:

- `preflight`, `copying`, `verifying`, `publishing`, and `deleting_source`;
- completed, failed, cancelled, and partial completion;
- copied, verified, and deleted entry/byte counters;
- target publication and source-remains indicators; and
- bounded cleanup state without absolute host paths.

Limit exhaustion, verification mismatch, source/target drift, cancellation, cleanup failure, and unsupported platform behavior remain distinct normalized failure categories. A cross-volume workflow reports `moved:true` only after the target is published and the complete source has been removed.

## Required validation

Implementation acceptance requires focused unit and integration coverage for files and directories; empty, deep, wide, and large inputs; overwrite and type conflicts; fingerprint mismatch; source and target races; every limit; cancellation in every phase; cleanup failure; restart; and injected failures before and after publication and during source deletion. Tests must prove that pre-publication failures preserve the source and that post-publication failures report partial state truthfully.

Windows finalization must exercise distinct local volumes, reparse behavior, replacement semantics, cleanup, and interruption. Native Linux finalization must exercise distinct mounted filesystems, symlink policy, rename/publication semantics, cleanup, and interruption. Synthetic Cloud tests are useful but are not evidence for either native platform boundary.
