# Local deterministic work principle

FlashGate performs deterministic host work inside the trusted server boundary
instead of asking an MCP client or model to reproduce that work from transferred
content. This principle reduces payload size, model round trips, token use, and
the chance that host data is disclosed or changed unintentionally.

## Decision rule

An operation belongs in FlashGate when all of the following are true:

1. the requested result follows from explicit, typed inputs and local state;
2. the server can enforce roots, capabilities, limits, cancellation, and audit
   requirements before doing the work;
3. behavior and ordering can be defined consistently on supported platforms;
4. inputs, work, and results can be bounded and covered by contract tests; and
5. performing the operation locally avoids transferring data merely so a client
   or model can compute the same deterministic result.

If those conditions are not met, FlashGate must not turn the request into a
free-form workflow, shell command, or implicit policy decision. A separate
typed contract and its required product, architecture, security, platform, or
dependency decision must come first.

## Preferred local operations

Use server-side primitives for work such as:

- copying and moving files without returning their contents;
- exact or conditional edits without retransmitting a complete file;
- hashing and metadata inspection without returning source bytes;
- bounded path, name, metadata, text, and regular-expression search;
- filtering, field selection, stable sorting, pagination, and batch inspection;
- range, head, or tail reads when only part of a file is needed; and
- dry-run previews and bounded plans composed from typed operations.

Return the smallest sufficient result: compact metadata for small operations,
requested ranges or pages for bounded content, and opaque result resources for
large content. Payload-heavy data must not be duplicated across text,
structured content, audit records, caches, or proxy/service messages.

## Security and correctness boundaries

Local execution is an efficiency rule, not additional authority. Every
operation remains subject to server-side root confinement, capability checks,
input validation, resource limits, redaction, cancellation, and audit policy.
Tool visibility, annotations, client instructions, and a request to avoid
retransmission never grant access.

The server must also avoid hidden work. It must not scan, read, hash, copy, or
modify more data than the typed request and enforced bounds require. Partial
results, truncation, conflicts, and per-item failures must be explicit in the
operation contract rather than concealed by a best-effort response.

## Client guidance

Clients should prefer one bounded server operation over a read-transform-write
sequence when FlashGate exposes the required typed operation. They should use
ranges, filters, selected fields, batches, cursors, and dry-run modes instead of
requesting complete content by default. Clients remain responsible for semantic
or creative decisions; FlashGate performs only the deterministic mechanics
expressed by the selected tool contract.

## Examples

| Goal | Prefer | Avoid |
|---|---|---|
| Duplicate a file | `copy_path` with relative source and destination paths | Read the file through MCP and send all bytes back to `write_file` |
| Determine whether content changed | A bounded server-side hash operation | Transfer the complete file solely to hash it elsewhere |
| Update known text | An exact edit with expected-match or fingerprint checks | Round-trip the complete file for a small replacement |
| Inspect many paths | A bounded batch with per-item outcomes | Repeated metadata calls when one typed batch is available |
| Find matching content | Bounded local search with filters and pagination | Read an entire tree through the model and search client-side |

The examples describe the Version 1.0 direction. They do not claim that every
listed operation is implemented by the current eight-tool filesystem profile;
the current tool set and implemented limits remain documented in
[Filesystem MCP tools](tools.md).
