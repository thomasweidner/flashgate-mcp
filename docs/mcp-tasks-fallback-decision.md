# MCP Tasks fallback decision packet

## Status and decision boundary

This document is the repository-contained decision packet for `BL-211`. It
does **not** select a fallback and does not advertise or implement the MCP Tasks
Extension. FlashGate currently implements MCP `2025-11-25`; synchronous tool
calls remain the only implemented MCP execution behavior.

The product/protocol owner must choose the Version 1.0 behavior for a client
that has not negotiated the final Tasks Extension before FlashGate exposes any
operation asynchronously over MCP. The choice must be recorded together with
the supported protocol and client matrix. An adapter must never infer Tasks
support from a client name, protocol revision, or transport.

## Invariants shared by every option

Whichever fallback is selected:

- extension negotiation is not authorization;
- the MCP adapter, not a domain service or the Operations/Job Manager, owns the
  wire-level fallback;
- no custom FlashGate status, result, or cancellation tools become the primary
  substitute for MCP Tasks;
- a request is never detached into background work unless the client has
  negotiated a contract through which it can securely address and control that
  work;
- authorization, root, profile, capability, principal, execution-backend,
  service-generation, quota, deadline, cancellation, and redaction checks are
  identical on negotiated and fallback paths;
- authorization, policy, identity, limit, and protocol rejections fail closed;
  they cannot be converted into another execution mode;
- payload and stored-result limits remain enforceable before work begins and
  while it runs; and
- errors contain stable safe categories, never raw OS errors, secrets, host
  paths, command lines, or enumerable internal handles.

The decision concerns MCP representation only. It does not alter the accepted
internal Operations/Job states or move domain ownership into the MCP adapter.

## Decision options

### Option A — bounded synchronous fallback when eligible

Execute an operation synchronously only when its domain contract can prove,
before acceptance, that it fits the configured synchronous execution and
response budgets. The request retains its normal deadline and cancellation
path. If eligibility cannot be proven, return the explicit capability error
from Option B without starting work.

This option preserves useful behavior for clients without Tasks, but it
requires a deterministic, testable eligibility predicate per operation class.
It must not wait optimistically and switch to an asynchronous handle after a
timeout or budget overrun.

### Option B — explicit capability error

Reject every operation that requires Tasks when the final Tasks Extension was
not negotiated. The response uses a stable machine-readable category and may
state, within the redaction and response-size budgets, that negotiated Tasks
support is required.

This option has the smallest compatibility and resource-risk surface, but
clients without Tasks cannot invoke those operation classes through MCP. Short
operations that are independently defined as synchronous remain unaffected;
the absence of Tasks must not disable ordinary bounded synchronous tools.

### Option C — class-specific combination of A and B

Select Option A only for explicitly enumerated operation classes with proven
synchronous bounds and Option B for all other Tasks-requiring classes. The
matrix is closed: an unlisted or newly added operation defaults to the explicit
capability error until reviewed.

This option can preserve more compatibility than Option B while avoiding a
global synchronous assumption. It also has the greatest ongoing contract and
test cost because every eligible class and budget must remain synchronized
across schemas, documentation, and supported protocol revisions.

## Required decision evidence

The owner decision must bind all of the following:

| Evidence | Required conclusion |
|---|---|
| Final Tasks contract and FlashGate support decision | The extension identifier, negotiation semantics, and lifecycle are final and are not mixed with the experimental 2025 lifecycle. |
| Supported-client matrix | Clients without Tasks that matter for Version 1.0 are named by tested capability behavior, not guessed identity. |
| Operation classification | Each exposed operation is short synchronous, Tasks-required, or (for Option C) explicitly synchronous-fallback eligible. |
| Resource budgets | Admission, runtime, response, stored-result, queue, and temporary-data limits are defined without choosing thresholds implicitly in this packet. |
| Cancellation and disconnect behavior | Synchronous cancellation is bounded and no unreachable background operation survives a fallback request. |
| Error compatibility | The no-Tasks error category and client-visible message are stable, bounded, redacted, and schema-valid. |
| Payload behavior | Inline, page, stream, or resource representation obeys the payload-class and single-transmission contracts. |
| Security review | Principal/handle binding, authorization parity, downgrade resistance, quotas, audit, and restart generation are covered. |

If this evidence is incomplete, the safe current behavior is to expose neither
asynchronous MCP work nor a new fallback surface.

## Required validation after the decision

Implementation of the selected option must add:

1. initialization tests for Tasks negotiated, not offered, declined, and
   mismatched-version cases;
2. positive and negative tests for every operation classification;
3. proof that an ineligible fallback performs no work and creates no handle,
   result, temporary resource, or audit-success event;
4. deadline, cancellation, disconnect, shutdown, TTL, and cleanup tests;
5. per-principal quota/fairness and cross-principal handle-denial tests;
6. response-size, payload-amplification, schema-snapshot, and redaction tests;
7. compatibility tests for every advertised MCP revision and supported client
   behavior; and
8. Windows and native-Linux lifecycle finalization where platform behavior is
   involved.

Release documentation must then state the selected behavior and its limits.
Until those gates pass, FlashGate continues to advertise only its implemented
MCP `2025-11-25` synchronous surface.

## Smallest required owner decision

Choose exactly one of `BOUNDED_SYNCHRONOUS_WHEN_ELIGIBLE`,
`EXPLICIT_CAPABILITY_ERROR`, or `CLASS_SPECIFIC_COMBINATION`; for the first or
third choice, approve the closed operation-class eligibility matrix and its
already-justified budgets. This is a product/protocol decision. This packet
intentionally does not make it.

## Related documents

- [ADR-010: Operations and Job Management](adr/010-operations-and-job-management.md)
- [ADR-012: Resource/Token Efficiency and Pre-1.0 Contracts](adr/012-resource-token-efficiency-and-pre-1-0-contracts.md)
- [ADR-013: MCP Version and Extension Compatibility](adr/013-mcp-version-and-extension-compatibility.md)
- [MCP protocol and local transport architecture](protocol.md)
- [Security](security.md)
- [Testing](testing.md)
- [Version 1.0 scope and release boundary](version-1-scope-and-release-boundary.md)
