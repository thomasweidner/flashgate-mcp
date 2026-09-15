# Payload-class result contract

**Status:** Accepted Version 1.0 target contract

**Owner:** `BL-213`

**Current-runtime impact:** None; the current eight filesystem tools retain their
documented pre-Version-1.0 envelopes until their owning implementation tasks migrate
them.

## Purpose

FlashGate classifies every successful tool result before choosing its MCP
representation. The classification prevents payload-heavy data from being copied into
both `content` and `structuredContent`, while allowing small compatibility results to
remain easy for clients to consume.

Classification is based on the semantic result, not its incidental size in one test.
A server may lower an inline threshold, but it must not reclassify a payload to evade
the rules below. Error representation is owned by `BL-203` and is not a payload class.

## Result classes

| Class | Typical results | Authoritative payload location | Required compact metadata |
|---|---|---|---|
| `metadata` | path status, mutation acknowledgement, operation status, counters | `structuredContent`; one compact text compatibility copy is permitted only within the active profile budget | result-specific identifiers, state, and counters |
| `structured_page` | directory, search, process, hash, or tree page | typed entries in `structuredContent` exactly once | returned count, limit, truncation state, and next cursor when more data exists |
| `text` | file range or process output | one MCP text content item, or an opaque result handle when it exceeds the inline limit | content type, encoding, raw byte count, returned range, truncation state, and optional hash |
| `binary_media` | image, audio, archive, or arbitrary bytes | one bounded MCP media/content item below the inline limit; otherwise an opaque result handle | MIME type, raw and encoded byte counts, transfer mode, and optional hash |
| `long_running` | scan, large tree, hash batch, copy/move plan, or managed process result | operation handle followed by bounded status/pages or an opaque result handle | operation state, progress/counters, expiry, and result availability |

The output schema for each tool identifies its class and the fields that carry useful
payload, metadata, counters, truncation, pagination, and expiry. Optional fields are
omitted rather than populated with misleading zero values.

## Single-transmission invariant

Useful payload bytes appear in exactly one result location. In particular:

- a structured page does not repeat serialized entries in a text summary;
- text is not copied into `structuredContent`; its metadata may record range and size;
- binary bytes are not repeated as Base64 in metadata;
- a handle response contains metadata about the referenced result, not an inline copy;
- a proxy or service transport forwards the same logical payload without adding a
  second protocol representation.

Short summaries may describe a result but must not reproduce its useful payload.
Hashes, counts, ranges, MIME values, and cursors are metadata and are not duplication.

## Bounds and fallback

Every payload-bearing tool defines server-enforced per-item and aggregate bounds. The
effective bound is the minimum of the configured server maximum, root/profile policy,
operation budget, and negotiated client capability. Client-requested limits may only
reduce that bound.

When a preferred representation is unavailable, the adapter uses this order:

1. a negotiated resource/result link;
2. bounded paging or streaming supported by the tool contract;
3. bounded inline content when the complete payload fits the effective inline limit;
4. a stable capability or limit error.

The adapter never silently truncates a complete-result contract. A contract that
permits truncation reports it explicitly and supplies a cursor or handle when further
retrieval is available. Base64 is not a fallback for data above the binary inline
limit.

## Authorization and lifetime

Cursors and opaque result handles are random, non-enumerable identifiers. They contain
no absolute host path, principal name, or other host detail. Creation and every later
read are bound to and revalidate:

- caller principal;
- root and effective profile/capabilities;
- execution backend and service generation;
- owning operation where applicable;
- expiry and remaining byte/page limits.

Expiry, cancellation, service restart, ownership mismatch, and policy change fail with
safe normalized errors. Stored payloads and temporary resources have bounded TTL and
deterministic cleanup; a handle is not an authorization capability by possession.

## Useful-byte accounting

Each result class has one deterministic useful-byte definition:

- `metadata`: UTF-8 bytes of the canonical compact domain object;
- `structured_page`: UTF-8 bytes of the canonical entries array, excluding envelope,
  cursor, counters, and summaries;
- `text`: raw bytes in the requested returned text range before JSON escaping;
- `binary_media`: raw bytes before Base64 or transport framing;
- `long_running`: useful bytes of the retrieved page or final referenced result; the
  initial handle/status response is accounted as metadata.

Empty useful payloads record zero useful bytes and no ratio; implementations must not
divide by zero. `BL-214` owns measurement and regression budgets for wire
amplification, approximate token cost per useful byte, serialization copies, and IPC
overhead.

## Compatibility and implementation boundary

Clients discover the available representation through negotiated MCP capabilities and
the tool's schema; annotations never authorize access. A client without resource-link
support receives only the bounded fallbacks above, not an unbounded compatibility
copy.

This contract does not by itself migrate `read_file` or add pages, cursors, operations,
resource storage, or resource endpoints. Those changes remain with their owning
filesystem, process, Operations/Job, resource-handoff, schema, and protocol tasks. A
migration must update the tool schema, wire tests, documentation, and efficiency gates
together.
