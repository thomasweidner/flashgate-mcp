# Search Model and Threat Model

## Status and scope

This document defines the Version 1.0 contract for `BL-068`. It is the foundation for the planned search tasks; it does not advertise a search tool in the current runtime.

Search is a separate read-only domain that consumes the central filesystem abstraction. It finds relative paths and bounded content below one server-configured root. It does not provide an index, invoke an external search program, interpret an ignore file by default, or bypass the central filesystem and path-policy boundary.

## Request model

A search request selects exactly one authorized root and one search kind:

- relative path or filename;
- portable metadata;
- literal text; or
- a regular expression accepted by the bounded pure-Go engine.

The request may narrow the search with a relative start path, deterministic include/exclude patterns, file types, portable metadata filters, recursion depth, context lines, and result fields. All paths and returned paths are root-relative. Patterns apply to normalized slash-separated relative paths; exclusions win over inclusions. Search never accepts an absolute host path, a shell command, an executable, or interpreter input.

Every request is normalized and validated completely before traversal. Unknown fields, empty search expressions where a term is required, invalid UTF-8 text selectors, invalid patterns, unsupported metadata, and limits outside server maxima fail before scanning.

## Traversal and ordering

Traversal starts only after the requested root and start path pass the same central path policy used by filesystem tools. Each discovered entry is rechecked before metadata access or content opening. Symlink, junction, reparse-point, hidden-path, and UNC behavior follows the configured filesystem policy; a denied entry is neither traversed nor disclosed.

Results use one portable deterministic order: normalized relative path by UTF-8 byte order, then match byte offset within a file. Implementations may scan concurrently, but concurrency cannot change the emitted order. Pagination resumes that logical order through an opaque, principal- and request-bound cursor. A cursor is not a path or authorization token and fails closed after expiry or relevant root, policy, request, or file-snapshot invalidation.

Files that disappear or change during scanning are reported only through bounded counters or safe per-item status when the later result contract permits it. Search does not silently claim a stable snapshot. A page must never mix results from a cursor whose binding has become invalid.

## Resource budgets

Server configuration sets positive maxima; a client may request only equal or smaller values. Before work begins, search binds effective budgets for:

- recursion depth;
- visited entries and opened files;
- bytes scanned globally and per file;
- matches globally and per file;
- context lines and aggregate context bytes;
- response bytes and page entries;
- elapsed time and cancellation checks; and
- concurrent search work globally and per principal.

Accounting uses bytes actually read for content scanning and counts entries before expensive inspection. Reaching a limit stops additional work and returns an explicit bounded/truncated outcome with counters; it never falls back to an unbounded scan. Cancellation and deadlines are checked during traversal and content reading, not only between files. Long-running searches may later use Operations/Job handoff, but Search retains matching, traversal, and accounting semantics.

## Content, encoding, and errors

The pure-Go baseline is mandatory. Literal matching operates on a documented text decoding mode; regular expressions use Go's bounded RE2-style engine and never a backtracking engine. Binary detection, supported encodings, context extraction, and explicit skip/error modes are finalized by `BL-078`; until then, no implementation may guess or silently decode binary content.

Errors use stable safe categories rather than raw operating-system strings. The contract distinguishes invalid input, authorization/policy denial, unavailable capability, unsupported content or encoding, resource limit, cancellation/deadline, stale cursor, path changed during scan, and internal I/O failure. Host absolute paths, file contents outside requested result fields, credentials, and raw OS errors are never included in results or diagnostics.

## Threats and required controls

| Threat | Required control |
|---|---|
| Root escape through traversal, links, junctions, or reparse points | Central root resolution plus policy recheck before traversal and open; fail closed on ambiguity or path change |
| Resource exhaustion from deep/wide trees or large files | Server-owned depth, entry, file, byte, time, match, context, response, and concurrency limits with incremental accounting |
| Regular-expression denial of service | Pure-Go RE2-style syntax, expression-size limits, cancellation, deadlines, and scan budgets |
| Result or context data leakage | Root-relative minimal fields, explicit field/context selection, bounded snippets, redacted diagnostics, and no denied-entry disclosure |
| Binary or malformed-encoding confusion | Explicit detection and mode contract; no implicit lossy decode or binary context |
| Ignore/include pattern bypass | Deterministic normalized matching, exclusion precedence, bounded pattern count/length, and tests for separator/case behavior |
| Cursor guessing, replay, or cross-principal use | Opaque random cursor bound to principal, root, profile, capability set, query, ordering, policy generation, and TTL |
| TOCTOU and unstable results | Revalidation before access, safe changed-path outcome, no stable-snapshot claim, and cursor invalidation |
| Unauthorized direct invocation | Server-side `search.read` capability and root-policy checks after tool resolution; catalog visibility is not authorization |
| Accelerator command injection or policy drift | No accelerator in the baseline; any later adapter uses no shell and must prove equivalent policy, limits, accounting, ordering, and redaction |

## Trust boundaries and non-goals

Search trusts only server-resolved principal, root, profile, capability, limits, and policy state. Client-supplied root labels, cursor contents, counters, path classifications, or accelerator claims are untrusted. Filesystem access remains behind `internal/fs`; the MCP adapter performs DTO mapping but owns no search policy.

Persistent indexing and a ripgrep accelerator are post-Version 1.0 and require their separate privacy, lifecycle, dependency, and equivalence decisions. Search does not add remote access, unrestricted globbing, arbitrary query languages, shell execution, hidden permission elevation, or a second filesystem abstraction.

## Permanent validation requirements

Focused unit, integration, security, and fuzz/property tests must cover:

- absolute/traversal paths and symlink/reparse/junction changes;
- deterministic ordering across concurrency and platform separators;
- depth, entry, file, byte, match, context, response, time, cancellation, and concurrency limits at and beyond boundaries;
- literal and regular-expression positives plus rejected syntax and oversized expressions;
- include/exclude precedence and case rules;
- binary, encoding, malformed-content, disappearing-file, and changed-file outcomes;
- cursor tampering, expiry, request/policy/root drift, and cross-principal replay;
- server-side capability bypass attempts and host-path/secret redaction; and
- native Windows and Linux behavior for path policy, metadata, cancellation, and platform-specific filesystems.

Later search tasks may refine their owned wire schemas and implementation details, but they must preserve these boundaries or record an explicit architecture/security decision.
