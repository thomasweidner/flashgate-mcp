# Architecture decisions

These decisions retain their stable IDs in their headings while filenames use
short topic names. File renaming does not renumber a decision. The absence of
an ID is not an invitation to reuse it.

An accepted decision records a product contract, not implementation completion.
Consult [architecture](../architecture.md), [planning](../planning/README.md),
and the [backlog](../../BACKLOG.md) for the current-versus-target boundary.

| Decision | Document |
|---|---|
| ADR-001: Use Go | [go](go.md) |
| ADR-002: Do Not Use an External MCP Library | [mcp implementation](mcp-implementation.md) |
| ADR-003: Use STDIO Transport | [stdio transport](stdio-transport.md) |
| ADR-004: Use Dependency Injection and Interfaces | [dependency injection](dependency-injection.md) |
| ADR-005: Filesystem abstraction | [filesystem abstraction](filesystem-abstraction.md) |
| ADR-006: FlashGate Project Identity and Open-Source Scope | [project identity](project-identity.md) |
| ADR-007: Domain-Separated Local System Core | [domain core](domain-core.md) |
| ADR-008: Core Reuse, Deployment, and FlashGate Module/Provider Model | [core reuse](core-reuse.md) |
| ADR-009: Capability Profiles and Tool Exposure | [capability profiles](capability-profiles.md) |
| ADR-010: Operations and Job Management | [operations and jobs](operations-and-jobs.md) |
| ADR-011: Managed Process and Command Execution | [process execution](process-execution.md) |
| ADR-012: Resource/Token Efficiency and Pre-1.0 Contracts | [efficiency contracts](efficiency-contracts.md) |
| ADR-013: MCP Version and Extension Compatibility | [mcp compatibility](mcp-compatibility.md) |
| ADR-014: Native Multi-Mode Runtime and Local Service Deployment | [runtime modes](runtime-modes.md) |
| ADR-015: Hybrid Service Execution Identity | [execution identity](execution-identity.md) |
| ADR-017: Host Process Ownership and Lifecycle | [host lifecycle](host-lifecycle.md) |
| File and product metadata decisions | [product metadata](product-metadata.md) |

The product metadata document retains its `DEC-FP-001` through `DEC-FP-003` identifiers and machine-readable section markers.
