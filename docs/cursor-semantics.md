# Stable cursor semantics

## Status and scope

This document defines the Version 1.0 contract for cursors returned by bounded
FlashGate collection and stream operations. It applies to directory and search
pages and to incremental process output. It does not add pagination to the
current `list_directory` implementation or define a new MCP tool.

A cursor is an opaque continuation capability for one logical result sequence.
It is neither a path, an offset supplied by the caller, an authorization token,
nor proof that the caller remains authorized.

## Public representation

Cursors are non-empty, server-generated strings. Clients must preserve the
value exactly and may only return it in the continuation field of the operation
that produced it. The value has no client-visible subfields, ordering meaning,
or stable encoding contract. It must not contain an absolute host path, caller
identity, configuration value, credential, or other sensitive plaintext.

The server may use authenticated self-contained state or a random lookup key.
In either case, a cursor has enough server-validated state to bind it to:

- the operation and result kind;
- caller principal and authorization context;
- root and normalized relative request scope;
- profile, capability set, risk policy, and execution backend;
- service generation;
- the complete normalized query, filters, field selection, and ordering;
- the sequence snapshot or invalidation state;
- expiry and implementation format version.

Cursor entropy, integrity protection, storage, and rotation are implementation
details. A client cannot construct, edit, combine, or transfer cursors.

## Ordering and page boundary

Every cursor-producing operation defines one total deterministic order before
issuing a cursor. Directory listing uses a platform-neutral ordinal ordering of
the public relative entry name, comparing UTF-8 bytes after the operation's
documented name normalization. A deterministic type or metadata filter is
applied before page boundaries are chosen. If a primary key can collide, the
operation defines an additional stable tie-breaker so that two distinct
results never compare equal.

The first request omits the cursor and supplies the complete query. A response
returns at most the effective server-capped page size and may return a cursor
only when more results remain. Absence of a cursor means the sequence is
complete. An empty page may carry a cursor only when the underlying contract
explicitly permits progress without a visible item; directory and search
pages must not do so.

A continuation request supplies the cursor rather than restating mutable query
parameters. Except for an optional requested page size no larger than the
original effective size, query, ordering, filter, field, root, or path arguments
must not accompany a cursor. The server rejects conflicting continuation
arguments instead of silently starting or reshaping a sequence.

## Snapshot and mutation behavior

Each cursor-producing operation must document one of these strategies:

1. **Materialized snapshot:** the server retains a bounded immutable result or
   stable result reference until completion or expiry.
2. **Validated continuation:** the server retains or encodes a bounded
   snapshot identity and verifies before every page that the observed sequence
   still represents the same logical source.

Directory pagination uses validated continuation. The snapshot identity must
detect changes that can alter membership, visible fields, filtering, or order.
It must not rely only on a client-visible last name or byte offset. If the
implementation cannot prove that the directory sequence is unchanged, it
invalidates the cursor; it never resumes approximately, skips silently, or
returns duplicates as if they belonged to one stable snapshot.

This contract intentionally chooses invalidation rather than best-effort
continuation for a changed source. A client that receives invalidation may
restart from the first page and must treat the restarted sequence as new.

## Invalidation and expiry

The server rejects a cursor when any bound input or security context changes,
including:

- source content or metadata relevant to membership, fields, or ordering;
- root mapping, normalized scope, profile, capabilities, or risk policy;
- caller principal, execution backend, or service generation;
- query, filter, field selection, ordering, or result schema;
- cursor format/version, integrity validation, retained state, or TTL;
- a server limit change that makes safe continuation impossible.

Expiry is based on a server-controlled monotonic elapsed-time decision where
available. Wall-clock timestamps may be reported for diagnostics but cannot
extend a cursor after rollback. Cursor TTLs and retained cursor count/bytes are
bounded by server policy. Use does not extend expiry unless the operation's
published contract explicitly says so; Version 1.0 directory cursors use an
absolute lifetime from creation.

Restart invalidates cursors by changing the service generation. Persistence of
cursor authority across restart is not part of Version 1.0.

## Errors and retry behavior

Cursor failures are machine-readable and safe. The public categories distinguish:

- `invalid_cursor`: malformed, tampered, wrong operation, or unsupported format;
- `cursor_forbidden`: caller or current authorization context does not own it;
- `cursor_expired`: TTL or retained state ended;
- `cursor_invalidated`: source, query contract, policy, or service generation changed;
- `limit_exceeded`: continuation cannot satisfy current bounded limits.

Diagnostics never echo the cursor, its decoded state, host paths, identities,
or policy details. Wrong-owner use must not reveal whether a cursor is otherwise
valid. Cursor errors do not authorize automatic fallback to another root,
profile, backend, or service. Retrying the same expired or invalidated cursor
cannot make it valid; the client starts a new sequence when policy permits.

## Resource and security requirements

Cursor creation, lookup, continuation, completion, expiry, and invalidation are
bounded and auditable. Implementations enforce per-principal and global limits
for live cursor count, retained bytes, page size, and lifetime. Completing the
sequence permits immediate state deletion. Expiry, cancellation, disconnect
cleanup where ownership requires it, and shutdown delete retained state
deterministically.

Every continuation repeats current authentication, authorization, root-policy,
and capability checks before reading retained or live data. Cursor possession
never bypasses those checks. Comparisons of random identifiers or integrity
tags use timing-resistant primitives where applicable, and logs record only a
safe correlation identifier rather than the public cursor value.

## Required validation

Each cursor implementation requires focused tests for:

- deterministic ordering, exact page boundaries, completion, and no duplicates;
- mutation between pages and fail-closed invalidation;
- malformed, tampered, wrong-operation, expired, and restarted-service cursors;
- cross-principal, cross-root, cross-profile, and changed-capability rejection;
- conflicting continuation arguments and server-limit reductions;
- bounded cursor count, retained bytes, TTL cleanup, cancellation, and shutdown;
- safe errors and logs without cursor, identity, or host-path disclosure;
- equivalent contract outcomes on Windows and Linux.

Tests that require real Windows reparse behavior, native host identity, or
service restart remain part of Windows/native finalization rather than Cloud
evidence.
