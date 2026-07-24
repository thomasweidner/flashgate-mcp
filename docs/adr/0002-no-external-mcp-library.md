# ADR-0002: Do Not Use an External MCP Library

## Status

Accepted

## Context

`fileserver-mcp` implements a Model Context Protocol (MCP) server.

The project aims to be:

- fast
- lightweight
- predictable
- easy to debug
- independent from third-party MCP implementation decisions

MCP libraries exist in different languages and maturity levels, but relying on an external MCP library may introduce unwanted dependencies, abstractions or compatibility constraints.

## Decision

The project will implement the MCP protocol directly instead of using an external MCP-specific library.

This applies specifically to the MCP protocol implementation.

General-purpose standard library packages and carefully selected non-MCP dependencies may still be considered if justified in future decisions.

## Rationale

Direct MCP implementation provides:

- full control over protocol behavior
- fewer dependencies
- easier debugging
- predictable performance
- better long-term maintainability
- no dependency on the lifecycle of an external MCP library

The MCP protocol is JSON-RPC based and small enough to implement directly in Go.

## Consequences

### Positive

- The server behavior is fully controlled by this project.
- Protocol bugs can be fixed directly.
- Dependency footprint remains minimal.
- The implementation can be optimized for filesystem use cases.
- Compatibility decisions remain explicit.

### Negative

- More protocol code must be maintained by this project.
- The implementation must track future MCP specification changes.
- Some features provided by external libraries must be implemented manually.

## Alternatives Considered

### Use an Existing Go MCP Library

Rejected for now because the project requires full control over behavior, performance and protocol handling.

### Wrap Another MCP Server

Rejected because the goal is to replace slower existing filesystem-oriented MCP servers, not to wrap them.

## Current Impact

The MCP runtime is structured under:

```text
internal/mcp
```

Protocol data structures are kept under:

```text
internal/protocol
```

The MCP implementation will be tested against MCP-compatible clients such as Claude Desktop and Codex-compatible tooling once the server core is complete.

## Amendment - 2026-07-10

The public project name is FlashGate MCP; the technical rename remains planned for Sprint 3.42. The MCP core is now covered by unit tests and Windows/Linux STDIO smoke tests, while client-specific activation preparation remains planned. Direct protocol implementation remains the decision, but protocol-version and extension compatibility is now governed by ADR-0013. See ADR-0006 through ADR-0013 for the current architecture.
