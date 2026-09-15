# Large-result and resource-handoff contract

FlashGate Version 1.0 uses transport-neutral result descriptors when a bounded
payload should not be embedded in one tool response. The core descriptor is
implemented by `internal/resultresource`; it does not make the core depend on
MCP resource types and does not yet advertise an MCP resources capability.

## Descriptor and URI

A descriptor contains:

- an unpredictable `flashgate://result/<opaque-id>` URI;
- MIME type, raw byte size, and a `sha256:<lowercase-hex>` content digest;
- an absolute UTC expiry time;
- principal, root, profile, effective-capability, execution-backend,
  service-generation, and operation bindings.

The URI contains no host path, user name, root name, MIME type, digest, or
authorization data. Resource references are bearer-resistant, not bearer
tokens: knowing a URI never authorizes access.

## Access and lifetime

Every read, page, stream, metadata lookup, or deletion repeats the complete
binding comparison from trusted request context. A mismatch returns only a
normalized access denial. Expired references return an expiry result and must
be removed by the storage owner. A service restart changes the generation and
therefore invalidates every prior reference even if backing data remains.

Storage remains owned by the producing domain or Operations/Job result store.
The descriptor retains no content. Storage implementations must enforce their
own byte, item, lifetime, and concurrency limits and deterministic cleanup on
success, failure, cancellation, expiry, and shutdown.

## Transfer and fallback

The adapter chooses among bounded inline content, an MCP resource link, paging,
or streaming only after negotiated client capabilities and server policy are
known. The order is:

1. return inline content only when its payload class and configured byte limit
   permit it;
2. otherwise return a resource link when that feature was negotiated;
3. otherwise use a bounded page/stream contract supported by the tool;
4. otherwise return a normalized capability/limit error.

The adapter must not Base64-embed an oversized payload, increase a limit, or
expose a host path merely because a client lacks resource-link support.

## Scope boundary

This contract supplies the shared descriptor, safe URI, metadata, expiry, and
authorization checks. MCP resource endpoint registration, negotiated resource
links, domain-specific backing stores, paging protocols, audit events, and
Operations/Job integration remain with their existing backlog owners.
