# ADR-006: FlashGate Project Identity and Open-Source Scope

## Status

Accepted

## Context

The project needs a durable public identity and a scope that does not confuse the current technical repository identifier with the intended product direction.

## Decision

The public name is **FlashGate MCP**. The tagline is “Fast, secure and local-first host operations for MCP.” Flash represents low latency, efficient local work, and compact responses. Gate represents controlled access through policies, capabilities, roots, limits, redaction, and audit.

The project is a vendor-neutral open-source MCP server for controlled local filesystem, process, execution, and operating-system operations. It is neither web hosting nor a remote-shell replacement.

The current repository, module, binary, and MCP server implementation name (`serverInfo.name`) remain `fileserver-mcp` until Sprint 3.42. Target identifiers are repository and binary `flashgate-mcp`, implementation name `flashgate`, and module `github.com/blacksheepkhan/flashgate-mcp`.

The core must not require Voxtronic-specific paths, tool names, proprietary dependencies, product permissions, secrets, internal URLs, or infrastructure values.

## Rationale

A public name can be adopted without mixing high-risk mechanical renames into the architecture sprint. A vendor-neutral core supports public reuse while allowing separately governed FlashGate modules/providers later.

## Consequences

- Documentation must explain the temporary dual-name state.
- Technical identifiers remain unchanged in Sprint 3.41.
- After Sprint 3.42, the old identifier remains only in migration and history.
- Public, community, vendor, internal, and Voxtronic-specific FlashGate modules/providers may be considered without changing core neutrality.

## Security Impact

Module/provider origin never weakens the shared security model. No company secret, path, permission, or infrastructure value belongs in the core. MCP protocol extensions are a separate negotiated wire-protocol concept governed by ADR-013.

## Implementation Guidance

Use FlashGate MCP for the public project. Use `fileserver-mcp` only when describing current technical artifacts or history until the rename sprint. Keep future features explicitly marked as planned.

## Deferred Decisions

- final public-release governance details
- FlashGate module/provider contract and runtime model
- MCP protocol-extension compatibility, governed by ADR-013
- technical rename execution, reserved for Sprint 3.42

## Amendment - 2026-07-11

Sprint 3.42 completed the technical rename. The repository is `blacksheepkhan/flashgate-mcp`, the Go module is `github.com/blacksheepkhan/flashgate-mcp`, the binary is `flashgate-mcp`, and the MCP server implementation name (`serverInfo.name`) is `flashgate`. The original context, decision, and transition rationale remain historical records.

## Amendment - 2026-07-20

Repository ownership was migrated from `blacksheepkhan/flashgate-mcp` to `thomasweidner/flashgate-mcp`. The current Go module is `github.com/thomasweidner/flashgate-mcp`; the binary and MCP server implementation identifiers remain `flashgate-mcp` and `flashgate`. Earlier owner references in this ADR record the state at the time and are retained as historical context.
