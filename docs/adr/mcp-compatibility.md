# ADR-013: MCP Version and Extension Compatibility

## Status

Accepted

## Context

FlashGate MCP currently implements MCP protocol version `2025-11-25`. The final MCP `2026-07-28` specification is now published and changes the base-protocol model materially: it removes the initialization/session handshake, makes requests self-contained through per-request `_meta`, requires `server/discover`, adds required result typing and list cache hints, formalizes Extensions including Tasks, and deprecates Roots, Sampling, and Logging. JSON Schema 2020-12 is the default schema dialect. FlashGate needs a compatibility strategy that does not couple its local system core to one wire revision or incorrectly claim support for later protocol features.

FlashGate modules/providers are optional local project extensions. MCP protocol extensions are separately specified, identified, and negotiated additions to the MCP wire protocol. They are not the same concept.

## Decision

The local FlashGate core remains independent of MCP protocol versions. The MCP adapter owns protocol-version negotiation, extension negotiation, revision-specific DTOs, and mapping between internal domain results and MCP wire contracts. FlashGate retains its own Go MCP adapter; official MCP SDKs/schemas are interoperability and conformance references, not runtime dependencies. Adopting an MCP SDK later requires a separate dependency/architecture decision.

The implemented protocol remains `2025-11-25`. Version 1.0 targets support for the final `2026-07-28` revision alongside the `2025-11-25` initialization path, but `2026-07-28` is not supported until FlashGate has the revision-specific adapter implementation, compatibility/conformance tests, and changelog/documentation evidence. Future revisions are added only through explicit matrix entries and adapter/test deltas.

New MCP features must not be inserted directly into core domains. The internal Operations/Job Manager will be designed so the MCP adapter can map eligible internal jobs to the final official MCP Tasks Extension `io.modelcontextprotocol/tasks`. The 2025 experimental task lifecycle and the final extension lifecycle must not be combined. No custom operation status/result/cancel tools are accepted as the primary MCP job contract while the Tasks-extension and client-compatibility decision remains open.

Internal operation state may be more detailed than the external MCP Task state. The adapter therefore requires an explicit, tested mapping for status, result, cancellation, errors, TTL, and client-visible messages. A bounded fallback for clients without Tasks support must be decided before asynchronous MCP behavior is exposed.

MCP Roots is deprecated and is not a foundation of FlashGate named roots. Named roots are authoritative server configuration addressed by explicit root IDs and relative tool paths. Compatibility with deprecated Roots behavior from initialization-based revisions may be evaluated only for a demonstrated supported-client need, with no architectural dependency.

MCP protocol extensions use the official vendor-prefix/slash identifier and capability-negotiation contract, for example `io.modelcontextprotocol/tasks`. FlashGate module/provider identifiers and runtime loading rules remain undecided and must not reuse MCP extension rules by implication.

All future tool input and output schemas will be validated against JSON Schema 2020-12. Any protocol upgrade requires protocol-version and extension negotiation tests, stateless-core behavior where applicable, deterministic catalog/cache TTL semantics, client compatibility review, schema/conformance checks, and changelog documentation.

## Rationale

Keeping revision-specific protocol details at the adapter boundary preserves a stable local core and permits deliberate support for multiple exact protocol revisions. Aligning future asynchronous behavior with the official Tasks Extension avoids creating a competing FlashGate-specific MCP contract. Server-configured named roots remain enforceable even when client support for deprecated MCP Roots is absent.

## Consequences

- MCP `2025-11-25` remains the only implemented protocol revision.
- The final `2026-07-28` revision is an accepted Version 1.0 target but is not claimed as supported before its revision-specific implementation and validation pass.
- The Operations/Job Manager has no dependency on MCP types.
- The MCP adapter needs explicit internal-job-to-Task mapping and fallback decisions.
- Named roots have no dependency on MCP Roots.
- FlashGate modules/providers and MCP protocol extensions use separate terminology and contracts.
- JSON Schema 2020-12 becomes a validation requirement for future tool contracts.

## Security Impact

Extension negotiation is not authorization. Tasks and every supported revision path must still enforce capabilities, root policies, caller/handle binding, limits, redaction, and audit controls. Opaque task or operation identifiers must not permit enumeration or cross-caller access. Deprecated MCP Roots must never weaken authoritative server root configuration.

## Implementation Guidance

- Keep version negotiation, stateless request routing, extension declarations, and list-cache TTL behavior in the MCP adapter.
- Generate deterministic tool ordering and a catalog fingerprint for each protocol/profile/configuration generation.
- Add version-specific compatibility tests before advertising a protocol revision or extension.
- Map internal states to official Task states deliberately; do not leak unrestricted internal diagnostics.
- Return synchronous results when supported and appropriate until Tasks compatibility is decided.
- For clients without a required extension, define a bounded synchronous or explicit capability-error fallback rather than an ad hoc job-tool surface.
- Validate all future input/output schemas as JSON Schema 2020-12 and evaluate official MCP conformance tooling.

## Decision Gates

- Version 1.0 supported protocol matrix with the `2025-11-25` initialization path plus final `2026-07-28` support
- Tasks Extension support and internal lifecycle mapping
- bounded behavior for clients without Tasks support
- optional deprecated MCP Roots compatibility for a demonstrated client need
- FlashGate module/provider contract and runtime model

## Official References

- [MCP 2025-11-25 schema](https://modelcontextprotocol.io/specification/2025-11-25/schema)
- [SEP-1613: JSON Schema 2020-12](https://modelcontextprotocol.io/seps/1613-establish-json-schema-2020-12-as-default-dialect-f)
- [SEP-2133: Extensions](https://modelcontextprotocol.io/seps/2133-extensions)
- [SEP-2577: Deprecate Roots, Sampling, and Logging](https://modelcontextprotocol.io/seps/2577-deprecate-roots-sampling-and-logging)
- [SEP-2663: Tasks Extension](https://modelcontextprotocol.io/seps/2663-tasks-extension)
- [MCP 2026-07-28 specification](https://modelcontextprotocol.io/specification/2026-07-28)
- [MCP 2026-07-28 key changes](https://modelcontextprotocol.io/specification/2026-07-28/changelog)
- [MCP 2026-07-28 final release announcement](https://blog.modelcontextprotocol.io/posts/2026-07-28/)

## Current result adapter

The implemented adapter uses MCP `2025-11-25` with explicit `TextContent` and `CallToolResult` DTOs. All eight current tools expose runtime `outputSchema` for the typed domain result. These features do not add protocol-version or extension negotiation.
## Version 1.0 adapter requirements

Version 1.0 must publish a concrete protocol matrix. Support for the accepted `2026-07-28` target is conditional on completed implementation and tests; the date alone does not change the advertised revision.

The Version 1.0 adapter design must be able to operate without connection-bound authorization or ownership assumptions. FlashGate state remains server-side but is addressed through principal-bound operation, process, cursor, and result handles. `tools/list` ordering and fingerprints must be deterministic, and cache/TTL use must invalidate on protocol, profile, capability, schema, or relevant configuration changes.

## Accepted protocol revision target

The final `2026-07-28` specification supersedes the earlier pre-final assumption in this ADR. FlashGate planning now names protocol paths by exact revision: the `2025-11-25` initialization path and the `2026-07-28` stateless path. Future protocol revisions receive explicit matrix entries and adapter/test deltas rather than inheriting a generic generation label.

The `2026-07-28` adapter path owns per-request protocol/capability metadata, `server/discover`, unsupported-version errors, required result typing, server identity metadata, list cache hints, and extension negotiation. Self-reported client information, discovery results, extension capabilities, and cache state are explicitly non-authoritative for FlashGate security decisions. Implementations owned by BL-207/208/209 must satisfy this final-specification planning contract before integration.
