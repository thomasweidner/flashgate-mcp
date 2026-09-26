# ADR-007: Domain-Separated Local System Core

## Status

Accepted

## Context

The implemented filesystem server is expected to grow into search, process, execution, and system-information functionality. Coupling those domains to MCP or JSON-RPC types would make reuse, testing, and later extraction difficult.

## Decision

Separate the local system core conceptually from the MCP adapter. Core packages must not depend on MCP, JSON-RPC, tool schema, or tool response types.

The accepted domains are filesystem, search, process, execution, system information, policy/capabilities, limits, diagnostics/audit, Windows adapters, Linux adapters, and the MCP adapter. Operations/jobs are an optional shared runtime service rather than a competing business domain or mandatory layer. Files and directories remain one filesystem domain.

The MCP adapter owns transport, tool registration, tool schemas, input validation, error translation, and structured serialization. Operating-system logic remains below it.

## Rationale

Domain ownership and inward dependency direction reduce duplication, keep policy enforcement consistent, and allow Go callers to reuse core behavior without MCP-to-MCP calls.

## Consequences

- Existing code remains valid current state; restructuring is planned, not performed here.
- Go components in the same codebase call the core directly.
- Future MCPs do not chain MCP calls internally.
- The core remains extractable without promising a stable public Go API prematurely.

## Security Impact

Policies, limits, validation, audit, and redaction remain shared boundaries below tool serialization. Domain adapters cannot bypass them.

## Implementation Guidance

Introduce package boundaries incrementally. Keep DTO translation at adapter edges. Test domain behavior without MCP protocol types and test MCP translation separately.

## Deferred Decisions

- exact package names and interface shapes
- timing of any extracted core package or library
- public Go API and compatibility commitment
