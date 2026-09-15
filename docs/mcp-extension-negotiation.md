# MCP extension negotiation and stateless adapter contract

## Status and scope

This document defines the accepted Version 1.0 adapter contract for MCP
extension negotiation and stateless operation. It does not advertise an
extension or a protocol revision. The released protocol matrix remains the
authority for what FlashGate actually supports.

The contract applies only at the MCP adapter boundary. FlashGate modules and
providers are a separate product concept and do not inherit MCP extension
identifiers or negotiation rules.

## Extension identity and admission

An extension is supported only when all of the following are present in one
released protocol-matrix entry:

- the exact official, case-sensitive extension identifier;
- the compatible MCP protocol revision or revision range;
- the extension contract version, when the specification defines one;
- the server capability used to offer it and the client capability used to
  accept it;
- implementation, positive/negative compatibility tests, and downgrade rules.

Aliases, shortened names, prefix matching, and identifiers inferred from tool
names are invalid. An unknown identifier is never mapped to a FlashGate module
or provider. Experimental and final forms of an extension are distinct
contracts and are not combined by name similarity.

## Negotiation

Negotiation is deterministic for a selected protocol revision:

1. Parse the client declaration as a closed, bounded capability object.
2. Reject malformed, duplicate, conflicting, or unsupported required
   declarations with a compatibility error.
3. Intersect optional client declarations with the extensions admitted by the
   released protocol matrix for that revision.
4. Return only the exact accepted identifiers and contract versions. Omission
   means not negotiated.
5. Bind the negotiated set to the request's adapter context and derive tool
   discovery, schemas, result representations, and cache identity from it.

Server support alone does not activate an extension. Client declaration alone
does not activate one either. A request must not use extension fields, methods,
or result forms unless the applicable adapter context contains that negotiated
extension.

The server fails closed when a client marks an unsupported extension as
required. Optional unsupported extensions are omitted, after which only the
documented non-extension behavior is available. FlashGate does not silently
substitute an experimental contract, a custom job-tool surface, or another
extension.

## Stateless adapter context

Core domain services do not depend on an MCP connection or protocol revision.
For every adapter request, the MCP boundary constructs an immutable context
containing at least:

- selected protocol revision;
- negotiated extension identifiers and versions;
- principal and authentication generation;
- active profile, roots, effective capabilities, and risk-policy generation;
- relevant configuration and schema generation;
- service/backend generation and request correlation.

Every request is authorized from current server state and that complete
context. Connection history, a previous initialization exchange, a catalog
fingerprint, a cursor, or an opaque handle is not sufficient authorization.
Stateful resources may exist behind the adapter, but each access revalidates
the resource's principal, root, profile, capability, backend/service,
generation, and expiry bindings.

Implementations may retain a connection-local copy of the immutable context as
an optimization for a protocol revision that defines connection-scoped
initialization. The copy is not an authority: policy/configuration generation
changes invalidate it, and no domain object may rely solely on connection
identity for ownership.

## Catalog cache and TTL semantics

Catalog caching is an optimization, not capability delegation. A cache key
includes the full public-contract tuple:

```text
protocol revision
negotiated extension identifiers and versions
profile and effective capabilities
risk-policy generation
schema/catalog generation
relevant configuration generation
```

TTL is an upper bound on reuse, never a promise that a cached catalog remains
valid for that duration. A generation or tuple change invalidates the entry
immediately. Expiry causes regeneration; it does not extend authorization or
the lifetime of handles/results. Cache responses and fingerprints reveal no
secret configuration values or internal identity values.

A client presenting an unknown, stale, differently scoped, or
protocol-incompatible fingerprint receives a current catalog (or the
protocol-defined cache miss response), never an authorization bypass and never
data from another context. If the selected protocol has no compatible cache or
TTL field, FlashGate omits it rather than inventing a wire extension.

## Downgrade and mismatch behavior

Negotiation and request handling must cover these cases with deterministic
tests:

| Case | Required behavior |
|---|---|
| Supported optional extension | Activate the exact common contract |
| Unknown optional extension | Omit it and use documented base behavior |
| Unsupported required extension | Fail with a compatibility error |
| Extension valid for another revision | Treat as unsupported for this revision |
| Extension use without negotiation | Reject; do not infer negotiation |
| Experimental/final identifier mismatch | Reject; do not translate |
| Stale or cross-context cache identity | Miss/regenerate without data reuse |
| Policy/configuration generation change | Invalidate context and cached catalog |

Errors are bounded and redacted. They may identify the incompatible public
protocol revision or extension identifier, but do not expose host paths,
credentials, principal identifiers, policy internals, or configuration values.

## Security boundary

Protocol negotiation answers only which wire contract both peers may use. It
never grants a filesystem, process, execution, system, job, root, or provider
capability. Tool exposure and every operation remain subject to current
server-side identity, profile, root, capability, policy, limit, redaction, and
audit enforcement.

Tasks support, fallback behavior when Tasks is unavailable, and the mapping of
internal Operations/Job states are owned by their separate decision and
implementation tasks. This contract does not resolve those decisions or
advertise Tasks.

## Permanent validation gate

For each released protocol/extension combination, automated tests must cover:

- exact identifier and protocol-matrix parity;
- successful negotiation and deterministic accepted-set ordering;
- malformed, duplicate, unknown, wrong-revision, and required-extension cases;
- unnegotiated use and experimental/final mismatch rejection;
- stateless reconstruction and generation invalidation;
- catalog fingerprint/TTL isolation and stale-cache behavior;
- proof that negotiation and cache artifacts do not authorize operations.

The gate runs for every advertised protocol revision. A documentation entry or
published upstream specification is insufficient to advertise support.
