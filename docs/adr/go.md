# ADR-001: Use Go

## Status

Accepted

## Context

FlashGate MCP is intended to be a fast, lightweight, cross-platform MCP server for filesystem operations.

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

## Implementation

The Go module is `github.com/thomasweidner/flashgate-mcp`; the executable is
`flashgate-mcp`, and MCP `serverInfo.name` is `flashgate`. The minimum
supported Go toolchain is declared in [`go.mod`](../../go.mod).
[Project identity](../project-identity.md) is authoritative for current names;
[migration](../migration.md#product-identity) explains early identifiers.
