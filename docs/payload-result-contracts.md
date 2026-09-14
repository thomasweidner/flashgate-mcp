# Payload-Class Result Contracts

**Status:** Version 1.0 contract for `BL-213`; implementation remains incremental.

FlashGate classifies every successful domain result before the MCP adapter serializes
it. The class controls where the useful payload appears, which compact metadata is
required, and whether paging or a resource handoff is mandatory. Classification does
not change authorization: the server still enforces the effective principal, profile,
capability, named root, policy, and limits before producing any result.

## Common rules

1. A payload-heavy byte sequence appears **exactly once** in an MCP result. It must
   not be repeated in both `content` and `structuredContent`.
2. `structuredContent` contains bounded metadata and structured collections, not a
   second encoding of heavy text, binary, media, or process output.
3. Raw host paths never appear in content, metadata, cursors, or resource URIs.
4. Size, truncation, range/page boundaries, encoding, MIME type, hash, cursor, and
   resource metadata describe only values the server actually returned or measured.
5. Server-owned response, item, byte, scan, and time limits apply before client
   compatibility fallbacks. A fallback may reduce a result; it may not raise a limit.
6. Errors use the normalized tool-error contract. Partial results are returned only
   when the owning tool explicitly defines a partial-result model.

The current filesystem and search adapters still use the pre-Version 1.0 compact
text-plus-`structuredContent` parity wrapper. That is an implementation fact, not an
exception to this target contract; each tool migrates only with its affected schema,
wire, compatibility, and regression tests.

## Result classes

| Class | Examples | MCP primary representation | Required compact metadata | Mandatory boundary |
|---|---|---|---|---|
| Small metadata | path info, mutation receipt, operation status | `structuredContent`; one compact text compatibility form is permitted | stable domain fields and relevant counters | configured response limit |
| Structured collection | directory page, search page, process-event page | `structuredContent`; optional bounded summary in `content` | returned count, truncation state, page/cursor data, applied limits | page item and byte limits |
| Heavy text | file range, match context, process stdout/stderr | one text content block, or a resource link when not safely inline | UTF-8 byte size, encoding, range, truncation, optional hash | inline threshold, then page/resource |
| Binary or media | image, audio, arbitrary bytes | one bounded media block or resource link; never a base64 copy in metadata | MIME type, raw byte size, transfer mode, optional hash | base64/inline threshold, then resource |
| Large asynchronous result | tree, scan, hash batch, long output | opaque resource link or page reference | handle, content class, size when known, TTL, progress and completion state | job/result-store quotas and TTL |

Small metadata may retain text/structured parity only while the complete canonical
domain object remains below the configured small-result limit. A collection is not
small metadata merely because a particular page happens to contain one entry.

## Class-specific contracts

### Structured collections

Ordering and cursor semantics are part of the producing tool contract. A page reports
the number of entries actually returned and whether more data exists. Search pages
keep match data in the structured collection; file contents that were scanned but not
returned are never counted or emitted as useful payload. Cursor continuation remains
bound to the original query and effective security context.

### Heavy text

Text is UTF-8 unless the result explicitly names another supported encoding. Byte or
line boundaries describe the source range, while the returned-byte count describes
the single emitted payload. Truncation is explicit. Match context and process output
use the same rule and may not be copied into a structured transcript.

### Binary and media

Metadata records raw size independently from encoded wire size. Inline base64 is a
compatibility transport, not structured metadata, and is allowed only under its
server-owned threshold. Unsupported or oversized inline requests select a negotiated
resource handoff or fail with a normalized capability/limit error; they are never
silently embedded.

### Large asynchronous results

The core domain returns a local result reference without importing MCP resource types.
The adapter may expose an opaque `flashgate://` URI after binding it to the principal,
profile, root, execution backend, service generation, content class, ownership, and
TTL. Expired, restarted, unauthorized, or unknown handles fail closed without
revealing whether another principal's result exists.

## Client compatibility and fallback

The negotiated client feature set selects among equivalent bounded representations:

1. resource link plus compact metadata;
2. structured page plus cursor;
3. one bounded inline text/media payload plus compact metadata;
4. a normalized capability or limit error.

Selection is deterministic for the same effective request, policy, negotiated
features, and server configuration. Clients cannot request duplicate representations,
and unsupported resource links never cause an unbounded inline fallback.

## Permanent validation gate

Every new or migrated payload-producing tool must test:

- its class selection and deterministic fallback order;
- byte-exact proof that heavy payload occurs once in the complete JSON-RPC response;
- metadata accuracy for raw/returned size, encoding, MIME type, range/page, and
  truncation fields that apply;
- inline, page, cursor, response, and resource limits at and beyond boundaries;
- clients with and without negotiated resource-link support;
- normalized failures for unsupported and oversized representations;
- absence of absolute host paths and cross-principal resource information; and
- useful-payload and wire-amplification accounting without counting scanned-only or
  duplicated compatibility bytes as useful payload.

Schema, catalog snapshot, documentation, smoke, and response-size gates change in the
same task that migrates a public tool. This contract alone does not migrate an existing
wire result or advertise resource support.
