# ADR-0001: Use Go

## Status

Accepted

## Context

`fileserver-mcp` is intended to be a fast, lightweight, cross-platform MCP server for filesystem operations.

The server must run on:

- Windows
- Linux

The project should produce a single standalone binary with minimal runtime requirements.

The implementation should be easy to test, maintain and deploy.

## Decision

The project will be implemented in Go.

## Rationale

Go is a good fit for this project because it provides:

- native cross-platform support
- simple static binaries
- strong standard library support
- efficient filesystem and I/O primitives
- good concurrency primitives
- fast build times
- excellent testing support
- straightforward deployment without a runtime dependency

Go also fits the intended architecture:

- small command-line application
- STDIO-based protocol server
- strict package boundaries
- interface-based design
- low memory and CPU overhead

## Consequences

### Positive

- The server can be distributed as a single binary.
- Windows and Linux builds can be created from the same codebase.
- The standard library covers most required functionality.
- Tests and benchmarks are built into the Go toolchain.
- The codebase remains relatively simple and maintainable.

### Negative

- Some ecosystem libraries around MCP may be less mature than in TypeScript or Python.
- Low-level protocol handling must be implemented carefully.
- GUI-related functionality is out of scope.

## Alternatives Considered

### TypeScript / Node.js

Rejected because it requires a runtime and generally has a larger dependency footprint.

### Python

Rejected because it requires a runtime and is less suitable for a single lightweight binary distribution.

### Rust

Considered, but Go was chosen because of its simpler development workflow, faster iteration speed and strong standard library support for this type of tool.

## Current Impact

The repository is initialized as the FlashGate MCP Go module:

```text
github.com/thomasweidner/flashgate-mcp
```

The minimum expected Go version is Go 1.26 or newer.

## Amendment - 2026-07-10

The public project name is FlashGate MCP. The repository, Go module, binary, and MCP server implementation name (`serverInfo.name`) retain the technical `fileserver-mcp` identifier until Sprint 3.42. This amendment does not change the historical context or the decision to use Go. See ADR-0006 through ADR-0013 for the current identity, architecture, and MCP compatibility direction.

## Amendment - 2026-07-11

Sprint 3.42 completed the technical rename: the Go module is now `github.com/blacksheepkhan/flashgate-mcp`, the binary is `flashgate-mcp`, and the MCP server implementation name (`serverInfo.name`) is `flashgate`. This amendment preserves the historical context and original Go decision.

## Amendment - 2026-07-20

Repository ownership was migrated to `thomasweidner/flashgate-mcp`. The current Go module is `github.com/thomasweidner/flashgate-mcp`. The former `blacksheepkhan` and `fileserver-mcp` identifiers remain only where they document historical decisions or completed migrations. This amendment does not change the decision to use Go.
