# ADR-0003: Use STDIO Transport

## Status

Accepted

## Context

MCP servers commonly communicate with clients through standard input and standard output.

The target clients for `fileserver-mcp` include:

- Claude Desktop
- Codex-compatible tooling
- other MCP-compatible clients

The server should not require opening a network port for normal operation.

During early development an HTTP health-check server was used only to verify the Go toolchain. That HTTP approach is not part of the final transport architecture.

## Decision

`fileserver-mcp` will use STDIO as its primary and initially only transport.

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

## Current Impact

Transport code is located under:

```text
internal/mcp/transport
```

The current `cmd/server/main.go` is only a bootstrap placeholder until the real MCP server wiring is completed.

## Amendment - 2026-07-10

The public project name is FlashGate MCP; the technical rename remains planned for Sprint 3.42. Server wiring is now implemented under `cmd/server` and uses the STDIO transport. This updated implementation state does not change the original STDIO decision. See ADR-0006 through ADR-0013 for the current architecture and compatibility strategy.

## Amendment - 2026-07-11

Sprint 3.42 completed the technical rename. The binary is `flashgate-mcp` and the MCP server implementation name (`serverInfo.name`) is `flashgate`; `cmd/server` remains the generic command path. This does not change the original STDIO transport decision.

## Amendment - 2026-07-17

ADR-0014 accepts an optional native multi-mode deployment in the same executable. STDIO remains the implemented transport and the required MCP-client-facing transport for direct and proxy operation. A future service process may use Windows Named Pipes or Linux Unix Domain Sockets as an internal local transport between FlashGate processes. This does not authorize remote TCP/HTTP transport and does not change the current no-argument STDIO behavior.

## Amendment - 2026-08-13

Direct STDIO is one session-scoped FlashGate process per client launch and MCP
transport session. Normal EOF or definitive transport loss initiates the one
bounded process-root shutdown path. Parent or logical-agent liveness is never
inferred from PID alone, and multiple simultaneous direct processes are valid
when they belong to distinct active transports. ADR-0017 is authoritative for
top-level host ownership, typed lifecycle evidence, safe stale/orphan
classification, and orphan prevention.
