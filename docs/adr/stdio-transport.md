# ADR-003: Use STDIO Transport

## Status

Accepted

## Context

MCP servers commonly communicate with clients through standard input and standard output.

The target clients for FlashGate MCP include:

- Claude Desktop
- Codex-compatible tooling
- other MCP-compatible clients

The server should not require opening a network port for normal operation.

An HTTP health-check listener is not part of the STDIO transport architecture.

## Decision

FlashGate MCP uses STDIO as its primary and currently implemented transport.

The server reads JSON-RPC messages from `stdin` and writes JSON-RPC responses to `stdout`.

Diagnostic output must go to `stderr`.

## Rationale

STDIO is appropriate because:

- it matches common MCP client integration patterns
- it avoids exposing a network port
- it avoids Windows Firewall prompts
- it keeps deployment simple
- it works on Windows and Linux
- it is easy to launch as a child process from MCP clients

## Consequences

### Positive

- No TCP listener is required.
- No firewall exception is required.
- The server is simple to run from desktop MCP clients.
- STDIO is easy to test with controlled input/output streams.
- Protocol traffic remains isolated from diagnostic logs.

### Negative

- The server is process-bound to the launching client.
- Remote use requires a separate wrapper or transport layer.
- Long-running diagnostics must not write to `stdout`, otherwise JSON-RPC communication may break.

## Implementation Rules

- `stdout` is reserved for MCP protocol messages.
- `stderr` is used for logs and diagnostics.
- The transport layer must not contain business logic.
- The transport layer must not directly execute tools.
- The transport layer must only read and write protocol messages.

## Current implementation and accepted targets

`cmd/server` wires the server to `internal/mcp/transport`. The no-argument
invocation is direct STDIO.

[ADR-014](runtime-modes.md) accepts optional native runtime modes in the same
executable. STDIO remains the MCP-client-facing transport for direct and
future proxy operation. A future service may use Windows Named Pipes or Linux
Unix Domain Sockets internally. This does not authorize remote TCP/HTTP
transport or imply that the service modes are already implemented.

[ADR-017](host-lifecycle.md) defines the target lifecycle: one session-scoped
process per direct client transport, bounded shutdown after definitive EOF or
transport loss, and verified ownership rather than PID-only inference.
Multiple processes serving different active transports are valid. The backlog
owns lifecycle implementation status; acceptance of the contract is not proof
that every target shutdown mechanism exists today.
