# FlashGate architecture decision index

This index makes the boundary between an accepted decision and delivered
runtime behavior explicit. Every ADR listed here is `Accepted`, but that status
means only that the architectural decision is authoritative. It does not mean
that every target described by the ADR is implemented, platform-validated, or
released.

For delivery truth, read the exact task row in
[`BACKLOG.md`](../../BACKLOG.md). For implemented behavior, use product code,
tests, and the [Current state](../architecture.md#current-state) section. The
[Version 1.0 target](../architecture.md#accepted-version-10-target-architecture)
and `Planned` backlog rows describe accepted work that is not yet canonical
completion. `Later` rows and the
[post-Version-1.0 list](../architecture.md#post-version-10-decisions) are
deferred.

## Decision records

The **horizon** column is a reading aid, not a second delivery tracker.
`Current foundation` means the decision underpins the implemented baseline;
`current and target` means the record contains both established constraints
and planned expansion; `accepted target` means its principal delivery remains
tracked as planned work. Backlog status always wins if the two appear to
conflict.

| ADR | Decision | Horizon |
|---|---|---|
| [ADR-001](001-use-go.md) | Use Go | Current foundation |
| [ADR-002](002-no-external-mcp-library.md) | No external MCP library | Current foundation |
| [ADR-003](003-stdio-transport.md) | STDIO transport | Current foundation |
| [ADR-004](004-dependency-injection.md) | Dependency injection | Current foundation |
| [ADR-005](005-filesystem-abstraction.md) | Filesystem abstraction | Current foundation |
| [ADR-006](006-flashgate-project-identity-and-open-source-scope.md) | Project identity and open-source scope | Current and target |
| [ADR-007](007-domain-separated-local-system-core.md) | Domain-separated local system core | Accepted target |
| [ADR-008](008-core-reuse-deployment-and-extension-model.md) | Core reuse, deployment, and module/provider model | Current and target |
| [ADR-009](009-capability-profiles-and-tool-exposure.md) | Capability profiles and tool exposure | Accepted target |
| [ADR-010](010-operations-and-job-management.md) | Operations and Job Manager | Accepted target |
| [ADR-011](011-managed-process-and-command-execution.md) | Managed process and command execution | Accepted target |
| [ADR-012](012-resource-token-efficiency-and-pre-1-0-contracts.md) | Resource, token-efficiency, and pre-1.0 contracts | Current and target |
| [ADR-013](013-mcp-version-and-extension-compatibility.md) | MCP version and extension compatibility | Current and target |
| [ADR-014](014-native-multi-mode-runtime-and-local-service-deployment.md) | Native multi-mode runtime and local service deployment | Accepted target |
| [ADR-015](015-hybrid-service-execution-identity.md) | Hybrid service execution identity | Accepted target |
| [ADR-016](016-governance-fixture-harness-execution-architecture.md) | Governance fixture harness execution architecture | Historical compatibility |
| [ADR-017](017-host-process-ownership-and-lifecycle.md) | Host-process ownership and lifecycle | Accepted target |

## Status rules

- Do not infer implementation from an ADR's `Accepted` status.
- Do not infer current product behavior from a target diagram or future-facing
  contract.
- Do not mark a backlog task complete from documentation wording or a Cloud
  candidate.
- When code, tests, an ADR, and the backlog disagree materially, stop and
  resolve the inconsistency rather than silently choosing one interpretation.
- Historical decisions remain records; superseding or narrowing one requires
  an explicit decision and must preserve the historical context.
