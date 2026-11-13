# Bounded Filesystem Plans: Threat Model

**Status:** Version 1.0 security contract; implementation remains planned under
`BL-056` and `BL-057`.

## Purpose and boundary

A bounded filesystem plan groups a small, typed sequence of filesystem-domain
operations so the server can validate and execute deterministic local work
without retransmitting file contents through the client. A plan is not a shell,
script, transaction language, or general workflow engine. It cannot invoke
commands, processes, network access, providers, or another MCP tool.

The filesystem domain owns path validation, operation semantics, and results.
The optional Operations/Job Manager may supply cancellation, deadlines,
progress, and bounded result storage, but it does not authorize or reinterpret
plan steps.

## Protected assets and attacker capabilities

The contract protects:

- data outside the configured root and paths denied by root policy;
- files and directories inside the root from unintended replacement, deletion,
  disclosure, or partial mutation;
- server availability, memory, temporary storage, and worker capacity;
- result confidentiality and the integrity of audit and diagnostic records.

Treat every plan, path, precondition, conflict choice, and limit request as
untrusted. An attacker may race filesystem objects between validation and use,
construct aliases through links or platform path rules, submit overlapping
steps, exhaust limits, disconnect or cancel during execution, and trigger I/O
or process termination at any step boundary.

## Closed operation model

The implementation must use an allowlist of versioned, typed operations. Each
step has a closed argument object and names only relative paths under one
explicit configured root. The initial operation set must be assembled from
individually implemented filesystem primitives; adding a new step kind requires
its own reviewed semantics and tests.

Plans must reject:

- unknown fields, operation kinds, or plan versions;
- absolute paths, traversal, unauthorized hidden paths, or disallowed
  symlink/reparse traversal;
- cross-root references and implicit current-working-directory resolution;
- steps whose source and destination alias or whose destination is inside a
  moved directory's source subtree;
- command strings, environment interpolation, glob expansion, callbacks,
  loops, conditionals, variables, or references to arbitrary prior output.

## Mandatory limits

The server, not the request, owns hard maxima. Before mutation, enforce plan
byte size, operation count, distinct path count, and requested result/preview
size. During execution, enforce cumulative visited-entry, bytes read, bytes
written, temporary-byte, result-byte, and duration/deadline limits. A caller
may lower but never raise a server maximum.

Preflight estimates do not replace runtime accounting. Directory growth,
sparse files, concurrent changes, encoding, metadata, and platform allocation
can make an estimate stale. Reaching a runtime limit stops before the next
step, cancels active work where supported, and returns bounded completion data.

## Validation and path-race controls

Validation has two layers:

1. **Plan preflight:** strictly decode the complete request, check structural
   limits and step compatibility, resolve every currently existing path through
   the central `PathGuard`, evaluate preconditions, and compute a conservative
   resource estimate without mutating the filesystem.
2. **Per-step execution:** immediately before each operation, repeat the
   applicable path-policy, type, identity, conflict, and precondition checks.
   The operation must then use the existing filesystem primitive rather than a
   second path implementation in the MCP adapter or plan runner.

Preflight success is not an authorization lease. Authorization and root policy
must still be valid when each step starts. When a path identity or relevant
precondition changed, the step fails closed as a conflict; the executor must not
silently retarget the replacement object.

Portable path-based APIs cannot remove every time-of-check/time-of-use race.
Implementations must preserve the narrow residual-race contracts of their
underlying primitives, avoid check-then-delete replacement, and document any
platform-specific stronger guarantee. No plan-level wording may claim atomicity
for a multi-step plan.

## Conflicts and overlapping steps

Every mutating step uses an explicit conflict behavior supported by its
underlying primitive. Omission means `fail`; there is no inherited global
"replace everything" default. `skip` and `replace` are valid only after their
individual semantics are implemented and reviewed. A conflict never falls back
to a more destructive behavior.

Preflight must reject ambiguous overlap, including a later step that addresses
an object or subtree moved or deleted by an earlier step, unless that exact
ordered composition has a separately defined typed contract. Duplicate targets
and source/target cycles are rejected. This keeps execution deterministic and
prevents earlier attacker-controlled mutations from changing the meaning of
later paths.

## Failure, cancellation, and rollback limits

Version 1.0 plans are ordered, bounded sequences, **not transactions**. A
success result means every step completed. Once a step reports completion, its
effects may remain if a later step fails, reaches a limit, times out, or is
cancelled. The server must never promise general rollback.

Automatic cleanup is limited to temporary artifacts created and exclusively
owned by the active step. It must not restore user data from an unverified path,
reverse a completed delete, or overwrite a concurrently changed destination.
Any future compensating operation must be explicit, identity/precondition
checked, independently bounded, and must report its own failure; it cannot turn
partial completion into an atomicity claim.

The result must identify the terminal state and a bounded ordered summary for
each accepted step: `pending`, `completed`, `skipped`, `failed`, or `cancelled`.
It must include the first terminal failure category and whether exclusively
owned temporary cleanup completed, while excluding absolute host paths and raw
operating-system errors. Steps after the terminal failure remain `pending` and
are not started.

## Dry-run semantics

Dry-run performs validation and produces a bounded preview without mutation. It
is advisory: it does not reserve paths, identities, capacity, authorization, or
conflict outcomes for a later execution. Executing a previously previewed plan
repeats all checks against current state. Preview output must distinguish known
facts from estimates and must not expose content or metadata the caller could
not obtain through the effective filesystem policy.

## Required security and correctness gates

Implementation is not complete until tests cover:

- strict decoding, unknown versions/steps, and every hard preflight/runtime
  limit, including estimates that become stale;
- traversal, absolute paths, hidden paths, symlink/reparse aliases, cross-root
  references, subtree moves, duplicate targets, and step overlap;
- identity/precondition changes between preflight and every mutating step;
- fail/skip/replace behavior without destructive fallback;
- failure, cancellation, deadline, disconnect, and server shutdown at each step
  boundary, with truthful partial-completion and temporary-cleanup reporting;
- dry-run non-mutation and mandatory revalidation at execution;
- deterministic ordering and bounded/redacted results through MCP `tools/call`;
- race-detector coverage plus Windows and native-Linux filesystem behavior.

Windows reparse points, Windows replacement behavior, and native Linux
symlink/filesystem behavior require real platform finalization; cross-builds or
synthetic tests do not replace that evidence.
