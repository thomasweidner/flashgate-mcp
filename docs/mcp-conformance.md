# MCP conformance and schema snapshots

## Scope

This document records the BL-204 evaluation of the official MCP conformance
framework and the repository's deterministic input/output-schema snapshot
gate. It does not advertise another protocol revision, add an HTTP transport,
or replace FlashGate's wire and schema tests.

## Official framework evaluation

The official
[`modelcontextprotocol/conformance`](https://github.com/modelcontextprotocol/conformance)
project was evaluated at release `v0.1.16` on 2026-09-10. The project describes
itself as work in progress and unstable. Its server runner accepts an MCP HTTP
endpoint through `server --url`; its published server scenarios include
initialization, tool listing, tool calls, resources, and prompts. The published
composite action uses Node.js and installs the framework's npm dependency tree.

FlashGate Version 1.0 currently exposes only MCP `2025-11-25` over STDIO. It has
no HTTP server endpoint to which the official server runner can connect.
Introducing an HTTP bridge only for this test would add a transport and
dependency boundary that BL-204 does not authorize, and it could produce
conformance evidence for the bridge rather than the shipped STDIO path.

Therefore the official framework is **evaluated but not adopted as a permanent
gate yet**. Reconsider it when either:

1. the framework supports a directly spawned STDIO server; or
2. FlashGate has an approved, shipped HTTP transport that the server runner can
   exercise without a test-only protocol bridge.

Any later adoption must pin an immutable release or commit, review the Node/npm
supply-chain and license inventory, select scenarios compatible with the
advertised protocol revision, and keep expected failures explicit. An
unpinned `npx` invocation is not an acceptable release gate.

## Deterministic schema snapshot

`internal/mcp/tools/testdata/tool-schemas.json` is the committed snapshot of
every runtime filesystem tool's complete `inputSchema` and `outputSchema`. It
also binds the runtime MCP protocol revision and preserves runtime tool order.

`TestRuntimeToolSchemasMatchSnapshot` constructs the runtime definitions,
serializes the complete snapshot with Go's deterministic JSON object-key
ordering, and compares the bytes exactly. Any tool order, protocol revision,
schema keyword, property, required field, or constraint change must therefore
be reviewed as a public-contract change and update the snapshot deliberately.

The snapshot gate complements, rather than replaces:

- runtime-to-static-catalog parity;
- representative successful-result validation;
- `tools/list` wire tests; and
- future complete JSON Schema 2020-12 metaschema and instance validation under
  BL-212.

The snapshot's `schemaVersion` identifies the snapshot artifact format. It is
not a JSON Schema dialect declaration. BL-212 remains responsible for adding
and validating JSON Schema 2020-12 dialect declarations without conflating
them with this artifact version.
