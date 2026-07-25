# ADR-0004: Use Dependency Injection and Interfaces

## Status

Accepted

## Context

`fileserver-mcp` must remain testable, maintainable and extensible.

The system contains distinct responsibilities:

- configuration
- security validation
- filesystem access
- MCP transport
- routing
- handlers
- tools

Directly constructing dependencies inside business logic would make testing harder and increase coupling between packages.

## Decision

The project will use explicit dependency injection and interface-based boundaries.

Components receive their dependencies through constructors or function parameters.

Global mutable state should be avoided.

## Rationale

Dependency injection supports:

- simpler unit tests
- mock implementations
- clear package boundaries
- predictable initialization
- easier future replacement of components
- cleaner separation between MCP logic and filesystem logic

For example, MCP tools should depend on the `FileSystem` interface rather than directly constructing a local filesystem implementation.

## Consequences

### Positive

- Components are easier to test in isolation.
- The filesystem can be mocked in MCP tool tests.
- Alternative implementations can be introduced later.
- Startup wiring remains explicit.
- Package coupling is reduced.

### Negative

- Initial code is slightly more verbose.
- Constructors must be maintained carefully.
- Interfaces should be introduced deliberately, not mechanically.

## Implementation Rules

- Application wiring belongs in `cmd/server` or dedicated setup packages.
- Business logic should not construct concrete dependencies unnecessarily.
- MCP tools should depend on interfaces where appropriate.
- Filesystem access should go through `internal/fs.FileSystem`.
- Path validation should go through `internal/security.PathGuard`.

## Current Impact

The project already uses interface-oriented design in:

```text
internal/fs.FileSystem
internal/mcp/handlers.Handler
internal/mcp/tools.Tool
```

Further MCP components will follow the same approach.

## Amendment - 2026-07-10

The public project name is FlashGate MCP; the technical rename remains planned for Sprint 3.42. The dependency-injection decision continues to apply to the domain-separated core, optional Operations/Job Manager, platform adapters, and MCP adapter described by ADR-0007 through ADR-0013.
