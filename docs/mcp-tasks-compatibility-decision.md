# MCP Tasks Extension compatibility decision packet

## Status and scope

This document is the repository-contained decision preparation for `BL-209`.
It does **not** select or advertise MCP Tasks support. The implemented FlashGate
protocol remains MCP `2025-11-25`, and the runtime exposes no Tasks extension.

The decision owner must choose whether Version 1.0 will expose the final
official Tasks Extension. That choice is intentionally separate from:

- the internal Operations/Job lifecycle mapping owned by `BL-210`;
- the no-Tasks fallback contract owned by `BL-211`; and
- implementation, conformance, client-interoperability, and release evidence.

This packet does not treat the 2025 experimental task lifecycle as compatible
with the final extension, and it does not authorize a custom FlashGate job-tool
protocol.

## Bound repository evidence

The current repository establishes these constraints:

1. Protocol-version and extension DTOs belong at the MCP adapter boundary; the
   Operations/Job core remains protocol-neutral.
2. Extension negotiation is capability discovery, not operation authorization.
   Every task reference must remain opaque, expiring, and bound to the caller,
   profile, capabilities, root, backend, service instance, and operation owner.
3. No extension or newer protocol revision may be advertised before its wire
   contract, negotiation, mappings, downgrade behavior, schemas, and positive
   and negative compatibility tests exist.
4. The experimental 2025 task lifecycle and the final Tasks Extension may not
   be combined.
5. Asynchronous MCP exposure requires both an explicit internal lifecycle
   mapping and a bounded behavior for clients without Tasks support.

The checkout contains no client-support matrix or interoperability evidence
that demonstrates which supported FlashGate clients negotiate the final Tasks
Extension. Publication dates, draft text, or SDK type availability alone are
not client-compatibility evidence.

## Decision options

### Option A — include the final Tasks Extension in Version 1.0

Select this only after the final extension contract is stable and the intended
Version 1.0 client set has demonstrated negotiation and lifecycle compatibility.

Required consequences:

- `BL-210` defines and tests the complete adapter mapping for creation, status,
  result retrieval, cancellation, terminal errors, expiry, and redacted
  client-visible messages;
- `BL-211` defines fail-closed behavior when negotiation is absent or
  incompatible;
- the released protocol matrix declares the exact revision and extension
  identifier;
- schema snapshots, conformance review, downgrade/mismatch tests, task-owner
  isolation tests, expiry tests, and cross-client interoperability evidence are
  release gates; and
- no internal job state or unrestricted diagnostic is exposed merely to match
  an MCP task state.

Impact: this enables negotiated asynchronous MCP operations in Version 1.0,
but adds adapter, interoperability, security, and release-gate scope. It must
not delay bounded synchronous operations by silently turning them into tasks.

### Option B — defer the final Tasks Extension beyond Version 1.0

Keep Version 1.0 on its implemented non-Tasks MCP surface. Internal Operations/
Job support may still exist for domain lifecycle management, but the MCP
adapter does not advertise Tasks or expose an ad hoc public job surface.

Required consequences:

- `BL-211` selects the bounded synchronous/capability-error behavior for work
  that cannot be exposed safely without Tasks;
- the released protocol matrix explicitly records that no Tasks extension is
  advertised; and
- later adoption still requires the complete Option A evidence rather than
  inferring compatibility from the internal job manager.

Impact: this minimizes Version 1.0 wire and client risk, while operations that
cannot finish within the selected synchronous bound remain unavailable through
MCP until a negotiated task contract is implemented.

## Evidence required for selection

The decision owner needs one dated compatibility report containing:

| Evidence | Minimum acceptable proof |
|---|---|
| Final contract identity | Final specification revision, extension identifier, and schemas are immutable or release-pinned |
| Intended client set | Named clients and versions supported for FlashGate Version 1.0 |
| Negotiation | Each intended client demonstrably accepts or safely ignores/rejects the extension according to the final contract |
| Lifecycle behavior | Create, poll, result, cancel, expiry, and terminal-error flows verified against each Tasks-capable client |
| No-Tasks behavior | Intended non-supporting clients exercise the selected `BL-211` fallback without custom job tools |
| Security isolation | Negative cross-principal, stale-handle, expired-handle, capability-change, and service-restart cases fail closed |
| Protocol separation | Tests prove no experimental-2025/final-extension DTO or state-machine mixing |
| Release readiness | Protocol matrix, schema snapshots, conformance result, documentation, and changelog agree |

Missing evidence is recorded as `NOT_VERIFIED`; it is not converted into a
compatibility assumption. A client name without an executed version-bound test
does not satisfy the matrix.

## Smallest required decision

After the evidence table is populated, the authorized product/protocol owner
must choose exactly one:

```text
BL-209 Decision: INCLUDE_FINAL_TASKS_IN_VERSION_1_0
```

or:

```text
BL-209 Decision: DEFER_FINAL_TASKS_BEYOND_VERSION_1_0
```

The decision record must identify the final MCP contract revision, intended
client/version set, evidence location, owner, and date. Until that record
exists, FlashGate remains fail-closed: it advertises no Tasks extension and no
downstream task may infer Option A.
