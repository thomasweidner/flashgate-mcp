# ADR-006: FlashGate Project Identity and Open-Source Scope

## Status

Accepted

## Context

The project needs a durable public identity and a scope that does not confuse the current technical repository identifier with the intended product direction.

## Decision

The public name is **FlashGate MCP**. The tagline is “Fast, secure and local-first host operations for MCP.” Flash represents low latency, efficient local work, and compact responses. Gate represents controlled access through policies, capabilities, roots, limits, redaction, and audit.

The project is a vendor-neutral open-source MCP server for controlled local filesystem, process, execution, and operating-system operations. It is neither web hosting nor a remote-shell replacement.

Current identifiers are repository `thomasweidner/flashgate-mcp`, Go module
`github.com/thomasweidner/flashgate-mcp`, executable `flashgate-mcp`, and MCP
implementation name (`serverInfo.name`) `flashgate`.

The core must not require organization-specific paths, tool names, proprietary dependencies, product permissions, secrets, internal URLs, or infrastructure values.

## Rationale

A durable identity separates product contracts from contributor-specific infrastructure. A vendor-neutral core supports public reuse while allowing separately governed FlashGate modules/providers later.

## Consequences

- Use the current product identifiers consistently.
- Explain early identifiers only where an external consumer needs migration guidance.
- Public, community, vendor, and organization-internal FlashGate modules/providers may be considered without changing core neutrality.

## Security Impact

Module/provider origin never weakens the shared security model. No company secret, path, permission, or infrastructure value belongs in the core. MCP protocol extensions are a separate negotiated wire-protocol concept governed by ADR-013.

## Implementation Guidance

Use FlashGate MCP and the current technical identifiers for active guidance. Keep future features explicitly marked as planned. See [migration](../migration.md#product-identity) for early integrations.

## Deferred Decisions

- final public-release governance details
- FlashGate module/provider contract and runtime model
- MCP protocol-extension compatibility, governed by ADR-013
