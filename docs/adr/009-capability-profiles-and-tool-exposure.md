# ADR-009: Capability Profiles and Tool Exposure

## Status

Accepted

## Context

As host functionality expands, exposing every tool in every deployment would increase attack surface, `tools/list` size, and operational ambiguity.

## Decision

Functional capabilities and profiles determine which tools are registered. Illustrative functional capabilities are `filesystem.read`, `filesystem.write`, `search.execute`, `process.observe`, `process.manage`, `process.control.external`, `command.execute`, and `system.read`.

Possible future profiles include `safe-read`, `filesystem-write`, and `admin`. `high-risk`, `destructive`, and `interactive` are risk classifications or additional policy conditions rather than universal rights. Final capability, profile, and risk-classification names are deferred to Sprint 3.50. Existing `MCP_READ_ONLY` behavior is the first restricted-profile case.

Server-side checks remain authoritative. MCP tool annotations are not authorization.

The target supports multiple named roots addressed by explicit root ID and relative path, based on authoritative FlashGate server configuration. Each root may have read/write, size, result, file-type, capability, symlink/reparse, and process-working-directory rules. Deprecated MCP Roots is not an architectural dependency and may be evaluated only as optional legacy-client compatibility.

## Rationale

Least-privilege tool exposure reduces risk and schema/token overhead while giving operators understandable configurations.

## Consequences

- `tools/list` varies by effective profile and capabilities.
- Negative capability tests are required.
- Direct tool calls must still be authorized server-side.
- The current single-root configuration remains compatible during migration.

## Security Impact

Capabilities, profiles, and roots form a server-enforced gate. Hidden tools and annotations alone never provide enforcement.

## Implementation Guidance

Centralize effective-capability calculation and registration. Apply the same decision during execution. Avoid leaking unavailable tool existence in error details.

## Deferred Decisions

- final profile names and configuration format
- exact named-root request schema
- whether a supported legacy client justifies optional deprecated MCP Roots compatibility

## Implementation Amendment - 2026-07-11

Sprint 3.44 prepares the existing restricted-profile case for later client activation without introducing final profile names or a general profile framework. A Codex read-only example must set `MCP_READ_ONLY=true` explicitly and then exposes exactly `list_directory`, `read_file`, and `get_path_info`. All five write names and all five removed legacy names return the same generic Invalid params contract in STDIO smokes. No client configuration is activated by this amendment.
