# ADR-008: Core Reuse, Deployment, and FlashGate Module/Provider Model

## Status

Accepted

## Context

The architecture must permit reuse and optional local FlashGate modules/providers without creating an unnecessary distributed system or fixing a module/provider contract before real requirements exist. MCP protocol extensions are a separate negotiated wire-protocol concept.

## Decision

Keep one repository and one primary binary initially. Use packages, interfaces, and capabilities for separation. Local Go consumers reuse the core directly. IPC is introduced only for justified process isolation or cross-language integration.

FlashGate modules/providers may later be public standard, community, vendor-specific, organization-internal, or Voxtronic-specific. Possible metadata includes name, version, vendor, declared capabilities, tools, configuration schema, security classification, platform requirements, and dependencies. No identifier syntax is selected.

Sprint 3.41 neither selects nor implements a FlashGate module/provider runtime model.

## Rationale

A single deployment minimizes startup, memory, release, and operational costs. Evidence-driven split gates avoid speculative microservices while preserving future isolation options.

## Consequences

- Separate binaries require measurable security, deployment, startup, memory, maintenance, platform, or release benefits.
- Benchmarks and threat models must justify a split.
- No MCP-to-MCP internal architecture is introduced.
- FlashGate module/provider contracts remain deferred.

## Security Impact

All FlashGate modules/providers must pass through common capabilities, root policies, limits, path validation, process policies, redaction, audit events, and platform adapters. Provider provenance grants no privilege. MCP protocol-extension negotiation grants no authorization.

## Implementation Guidance

Keep interfaces narrow and internal. Document module/provider needs as use cases until a real external provider reaches the decision gate. Treat MCP protocol extensions only in the MCP adapter under ADR-013.

## Decision Gates

Before the first external FlashGate provider, choose among statically linked extension packages registered at build time, registered in-process providers, or isolated out-of-process providers over local IPC. A Go module is only a possible source/versioning form for statically linked packages, not a runtime model. Before a binary split, record benchmarks and a threat model showing concrete benefit.

## Amendment - 2026-07-17

ADR-014 accepts same-executable process roles for direct STDIO, local proxy/auto operation, and operating-system-managed service hosting. This is not a split into independently released product binaries and does not create an MCP-to-MCP or remote microservice architecture. The local IPC boundary is justified specifically by optional SCM/systemd and user-scoped background hosting and remains subject to service authorization, compatibility, benchmark, and threat-model gates.
