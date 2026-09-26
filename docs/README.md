# FlashGate documentation

FlashGate currently provides root-confined filesystem tools over local STDIO.
Accepted future capabilities are explicitly separated into planning documents.
Start with the root [README](../README.md) for the current product baseline.

## Use and integrate

| Document | Purpose |
|---|---|
| [Client setup](client-setup.md) | Verified binary, explicit read-only root, client acceptance, troubleshooting, and rollback. |
| [Tools](tools.md) | Current discovery, arguments, and tool behavior. |
| [Protocol](protocol.md) | Implemented MCP revision, JSON-RPC, initialization, and result contracts. |
| [Security](security.md) | Root, path, policy, disclosure, and resource boundaries. |
| [Migration](migration.md) | Compatibility guidance for early names, root configuration, and result envelopes. |
| [Changelog](../CHANGELOG.md) | The single narrative release-notes source. |

## Understand the design

| Document | Purpose |
|---|---|
| [Architecture](architecture.md) | Current structure and accepted target boundaries. |
| [Specification](specification.md) | Functional requirements and current-versus-planned scope. |
| [Project identity](project-identity.md) | Public identifiers and vendor-neutral scope. |
| [Architecture decisions](adr/README.md) | Stable decision IDs, rationale, and security consequences. |
| [Execution identity](execution-identity-backends.md) | Caller and execution-backend separation. |

## Build, test, and contribute

| Document | Purpose |
|---|---|
| [Contributing](../CONTRIBUTING.md) | Scope, change, validation, and release requirements. |
| [Coding style](coding-style.md) | Implementation conventions. |
| [Tool conventions](tool-conventions.md) | Public tool and schema contracts. |
| [Testing](testing.md) | Focused, platform, shell, and complete quality gates. |
| [Coverage](development/code-coverage.md) | Product coverage scope and policy. |
| [Benchmarks](../benchmarks/README.md) | Reproducible methodology, baselines, budgets, and workspace constraints. |
| [Build metadata](build-metadata.md) | Canonical product/build identity and release generation. |
| [Artifact verification](artifact-verification.md) | Binary, archive, provenance, and leak checks. |
| [Metadata validation](metadata-validation.md) | Remaining operator-facing checks. |
| [Documentation style](documentation-style.md) | English prose, stable names, ownership, and evidence separation. |
| [Documentation quality](documentation-quality-gate.md) | Automated and manual documentation checks. |

## Follow accepted plans

The [planning index](planning/README.md) groups accepted tool, runtime,
efficiency, and release targets. The [roadmap](roadmap.md) summarizes direction;
[BACKLOG.md](../BACKLOG.md) owns IDs, statuses, and milestones. Planned behavior
must not be advertised as already implemented.
