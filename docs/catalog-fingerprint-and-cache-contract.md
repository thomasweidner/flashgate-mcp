# Tool Catalog Fingerprint and Cache Contract

## Status and scope

This document defines the Version 1.0 target contract for deterministic
`tools/list` ordering, catalog fingerprints, and compatible list-result cache
semantics. It does not change the currently advertised MCP revision, add a
field to the current `tools/list` result, or make cached catalog data an
authorization decision.

The MCP adapter owns this contract. Core filesystem, search, process, and
execution domains do not depend on catalog fingerprints or MCP cache metadata.

## Canonical catalog representation

The effective catalog is computed only after the server has evaluated the
protocol revision, active profile, functional capabilities, and relevant
configuration. Tools that are not authorized for that effective context are
absent rather than described and rejected later.

The adapter constructs the canonical representation as follows:

1. Sort tools by the UTF-8 bytes of their unique `name`, in ascending order.
2. Serialize each complete public tool definition, including annotations and
   input and output schemas, as JSON with object member names sorted by UTF-8
   bytes, no insignificant whitespace, and ordinary JSON array order
   preserved.
3. Reject duplicate tool names and values that cannot be represented as JSON.
4. Treat schema array order as contract data. Do not reorder `required`, enum,
   union, example, or tuple-like arrays while canonicalizing.

The same effective context therefore produces byte-identical canonical
catalog JSON regardless of Go map iteration or tool registration order. The
wire response uses the canonical tool order. A serializer may use a different
insignificant object-member layout on the wire, but the fingerprint input is
always the canonical representation above.

## Fingerprint input and format

The fingerprint input is one canonical JSON object with these members:

```json
{
  "capabilities": ["filesystem.read"],
  "catalog": {"tools": []},
  "configurationGeneration": "opaque-generation",
  "profile": "safe-read",
  "protocolVersion": "2025-11-25",
  "schemaVersion": "flashgate-tool-catalog/v1"
}
```

The example values are illustrative. The input rules are normative:

- `protocolVersion` is the negotiated MCP revision used to encode the result;
- `profile` is the effective profile identifier, not a client claim;
- `capabilities` is the sorted, duplicate-free set of effective functional
  capabilities that can affect catalog visibility or definitions;
- `configurationGeneration` is an opaque, non-secret identifier that changes
  whenever relevant configuration can affect the catalog;
- `schemaVersion` identifies this fingerprint-input format; and
- `catalog` is the complete canonical `tools/list` result.

Compute SHA-256 over the UTF-8 bytes of that canonical object and expose the
identifier as lowercase `sha256:<64 hexadecimal digits>`. The digest is a
change identifier, not a secret, signature, authorization token, or proof that
the caller may invoke a listed tool.

Root paths, principal identifiers, credentials, environment values, policy
reasons, and other host-sensitive data must not appear in the fingerprint
input. When one of those values changes the effective catalog, the server
changes the opaque configuration generation instead of hashing the sensitive
value.

## Invalidation

A catalog fingerprint and any cached result are invalid immediately when any
of the following can change the effective catalog or a public tool definition:

- negotiated protocol revision or negotiated catalog-affecting extension;
- effective profile, capabilities, or risk policy;
- tool name, title, description, annotations, input schema, or output schema;
- root or configuration policy that affects tool availability or definition;
- server configuration, implementation, or schema generation relevant to the
  catalog; or
- a supported MCP revision's list-cache contract requires invalidation.

Changes that provably cannot affect visibility or public definitions need not
change the fingerprint. Implementations must prefer a false invalidation over
reusing a possibly stale or overprivileged catalog.

Fingerprints are scoped to one protocol contract. Identical tool JSON under an
incompatible protocol revision does not reuse the earlier fingerprint because
`protocolVersion` is part of the digest input.

## Cache and TTL behavior

The server may cache canonical catalog bytes and the corresponding
fingerprint. A cache entry is keyed by all fingerprint inputs and by any
execution-context identity needed to prevent authorization-sensitive reuse.
It may be shared only when the server has proved the catalog data
identity-independent and has re-evaluated the current caller's policy before
returning it.

A TTL is an upper bound on reuse, not a promise that an entry remains valid for
that duration. Invalidation evicts or bypasses an entry immediately even when
its TTL has not elapsed. Expiry requires regeneration; it does not imply a
catalog change, so regeneration may produce the same fingerprint.

For the implemented `2025-11-25` adapter, FlashGate returns a normal
`tools/list` result on every request and advertises no cache extension or
fingerprint field. An implementation may reuse a validated in-process cached
serialization internally, but it must preserve the same wire result and must
perform current policy evaluation first.

For a future MCP revision with negotiated list caching, the adapter maps the
fingerprint and TTL only through that revision's official fields and
semantics. Clients that did not negotiate the feature receive the complete
bounded result. FlashGate does not add ad hoc fields to older protocol
revisions.

## Security and failure behavior

- Tool visibility and a matching fingerprint never authorize `tools/call`.
  Invocation performs current server-side capability and policy checks.
- Cache lookup must fail closed across principal, profile, capability, root,
  backend, service generation, and configuration boundaries wherever those
  boundaries can affect visibility or definitions.
- Client-supplied fingerprints are untrusted cache validators, not selectors
  for another context's entry.
- Unknown, malformed, stale, or context-mismatched validators cause a complete
  bounded catalog response or the official negotiated-protocol error; they do
  not reveal whether another context's fingerprint exists.
- Diagnostics may record a redacted cache outcome and fingerprint prefix, but
  not sensitive fingerprint inputs.

## Required validation

Implementation of this target contract must cover:

- registration-order and map-order independence;
- stable canonical bytes and fingerprint for repeated identical input;
- changed fingerprints for every catalog-affecting input class;
- preserved array order and deterministic object-member ordering;
- no cross-profile, cross-capability, cross-principal, cross-root,
  cross-backend, or cross-generation cache reuse;
- immediate invalidation before TTL expiry and safe regeneration after expiry;
- downgrade behavior for clients without negotiated cache support; and
- parity between the fingerprinted catalog and the catalog returned on the
  wire for every advertised protocol/profile combination.

Windows finalization must revalidate platform-specific configuration and
identity-generation boundaries when those inputs are implemented.
