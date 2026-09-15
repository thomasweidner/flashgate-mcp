# Stateful Domain Threat Models

This document is the Version 1.0 threat-model index for FlashGate components
that retain state across a request, expose resumable state, or coordinate host
resources. It records the minimum security properties that implementations and
reviews must preserve. It does not claim that the planned domains are already
implemented.

The current runtime implements the filesystem tools described in the README.
Search, Operations/Jobs, managed processes, typed command execution, result
resources, multi-mode hosting, and external providers remain planned unless a
more specific current document says otherwise.

## Shared trust boundaries and invariants

State crosses four distinct boundaries:

1. untrusted MCP input enters a version-specific protocol adapter;
2. server-derived principal, profile, capability, root, backend, and service
   generation establish the effective execution context;
3. domain owners create and mutate their own state under central limits; and
4. platform adapters interact with files, processes, IPC endpoints, and the
   operating system.

Client input, tool visibility, MCP annotations, opaque identifiers, cached
state, registry files, and proxy-supplied identity are never authorization.
Every state lookup and mutation must repeat server-side authorization and bind
the object to the complete execution context that is relevant to it. Public
identifiers must be unpredictable and must not disclose a PID, host path,
principal, or other sensitive implementation detail.

All stateful domains must:

- bound creation, concurrency, queue depth, retained bytes, result size, and
  lifetime with server-controlled limits;
- reject cross-principal, cross-root, cross-profile, cross-backend, and stale
  service-generation access without revealing whether another object exists;
- define cancellation, deadline, partial-result, expiry, restart, and shutdown
  behavior before exposing resumable state;
- make cleanup idempotent and preserve the last proven lifecycle transition;
- emit bounded, structured, secret-safe diagnostics and audit correlation;
- fail closed when ownership, identity, generation, or cleanup state is
  ambiguous; and
- have race, negative-authorization, exhaustion, restart, and cleanup tests.

## Filesystem and search state

| Asset or state | Principal threats | Required controls and evidence |
|---|---|---|
| Root-confined paths and filesystem plans | traversal; symlink/reparse and TOCTOU escape; stale preconditions; partial destructive execution | authoritative named-root resolution; relative model-facing paths; effective-path revalidation; bounded preflight/runtime accounting; explicit partial-state results; Windows/Linux path-race tests |
| Listing and search cursors | cursor guessing or tampering; reuse after policy/root/content changes; cross-principal disclosure; unstable ordering | opaque execution-context-bound cursor; stable portable ordering; explicit invalidation; page/entry/byte/depth limits; forged, expired, cross-context, and mutation tests |
| Search scans and context | regex/recursion exhaustion; binary or encoding confusion; ignored-path disclosure; excessive context | bounded pattern complexity, files, depth, scanned bytes, matches, context, and response; explicit binary/encoding policy; deterministic include/exclude precedence |
| Hashes and conditional state | treating a fingerprint as authorization; stale write acceptance; unbounded hashing | bounded algorithms and byte accounting; authorization on every use; explicit snapshot/precondition semantics; mutation-race tests |

Filesystem state remains owned by the filesystem domain. Search may consume
filesystem access through that boundary but must not weaken root policy or
invent a second path-authority model.

## Operations, jobs, and result resources

| Asset or state | Principal threats | Required controls and evidence |
|---|---|---|
| Operation records and handles | enumeration; cross-principal status/cancel/result access; stale generation; state-transition races | cryptographically unpredictable handles; immutable owner/context binding; validated transition graph; indistinguishable unauthorized/not-found behavior; race tests |
| Queues and workers | global or per-principal starvation; queue exhaustion; cancellation/deadline races; goroutine or subprocess leaks | global/domain/principal caps; fair scheduling; deterministic overload; server deadlines; bounded worker ownership; shutdown and leak tests |
| Retained results and temporary data | unbounded retention; host-path disclosure; read-after-expiry; incomplete cleanup | byte/count/TTL quotas; opaque references; owner checks on retrieval; incomplete markers; idempotent sweeps across success, failure, cancellation, timeout, and restart |
| `flashgate://` result resources | bearer-token authorization; URI/path leakage; stale cache or service generation; oversized fallback | handle is reference only; authorization on every retrieval; MIME/size/hash metadata; execution-context and expiry binding; bounded negotiated fallback |

The Operations/Job Manager coordinates lifecycle only. Filesystem, search,
process, execution, and system-information components retain business-domain
ownership and perform their own authorization and cleanup.

## Managed processes and typed commands

| Asset or state | Principal threats | Required controls and evidence |
|---|---|---|
| Managed process identity | PID reuse; adoption of external processes; cross-principal control; orphaned child trees | opaque handle plus verified creation identity; never PID-only authority; explicit ownership record; bounded owner-only wait/stop/cleanup; PID-reuse and restart tests |
| Output buffers and cursors | secret or command-line disclosure; unbounded stdout/stderr; cursor replay; slow-reader exhaustion | separate bounded streams; truncation markers; redaction before persistence/exposure; execution-context-bound cursors; slow-reader and expiry tests |
| Typed command definitions | executable substitution; shell/argument/response-file/config/hook/plugin injection; root escape | server-resolved executable ID and structured arguments; no shell; allowlisted flags and values; root-confined paths; rejected injection corpus |
| Execution environment and isolation | inherited credentials/loaders/plugins; privilege confusion; network escape; incomplete platform isolation | minimal allowlisted or fixed environment; separate caller and effective backend identities; least privilege; documented fail-closed platform limits; Windows/Linux native evidence |

The synchronous command adapter must use the Managed Process Engine rather
than create a second execution engine. External PID control, process input,
and interactive shells remain separate high-risk decisions.

## Host, service, and connection state

| Asset or state | Principal threats | Required controls and evidence |
|---|---|---|
| Local IPC connection/session | endpoint spoofing; peer-identity substitution; cross-connection state reuse; downgrade or mismatch | restrictive OS endpoint policy; OS-derived peer identity; versioned bounded handshake; connection ownership; fail-closed mismatch and disconnect cleanup |
| Direct/proxy host instance | orphaned host; retained pipe; false owner attribution; PID reuse; unsafe idle or singleton reaping | immutable instance identity; definitive transport/owner evidence; PID plus start identity or verified handle; one bounded root coordinator; `SUSPECTED_STALE` prevents automatic termination |
| Persistent service | client disconnect terminating service; service-account privilege expansion; stale generation | SCM/systemd supervision; connection cleanup separate from service lifetime; dedicated least-privilege identity; generation-bound state; native lifecycle tests |
| Audit and runtime registry | secret/log injection; forged authority; disk-full deadlock; stale records | structured encoding and redaction; bounded queue/backpressure; rotation/retention policy; evidence-only registry; safe deterministic stale-record removal |

ADR-017 is authoritative for host ownership and lifecycle. A normal client
disconnect ends connection-owned work but does not stop a persistent service.
No age, idle-time, CPU, request-count, singleton, PID-only, or registry-only
heuristic authorizes process termination.

## Protocol, caches, and future providers

Protocol negotiation and catalog/result caching must be isolated by effective
profile, configuration, protocol revision, execution context, and service
generation. Downgrade, cache collision, stale TTL, and unsupported-extension
paths fail closed and never alter authorization. MCP Tasks mapping must not
introduce a competing state or cancellation authority.

External providers are post-Version 1.0. Before any provider runtime is
accepted, its threat model must cover capability inflation, policy bypass,
dependency and update compromise, state isolation, lifecycle ownership,
distribution trust, and in-process versus IPC failure containment. Providers
must use the same central authorization, limits, redaction, audit, and
execution-identity boundaries as built-in domains.

## Required review and validation

Each implementation change must identify the rows it affects and add the
smallest permanent tests that prove the accepted contract. At minimum, the
complete Version 1.0 evidence set includes:

- forged, malformed, expired, stale-generation, and cross-context identifiers;
- concurrent create/read/update/cancel/expire/shutdown execution under the Go
  race detector;
- quota, fairness, slow-reader, oversized-input/result, and cleanup pressure;
- restart, abrupt disconnect, deadline, partial failure, and idempotent retry;
- redaction and log-injection negatives with no host path or credential leak;
- Windows and native Linux identity, path, IPC, process, and lifecycle cases
  where platform behavior is authoritative; and
- traceability from every accepted residual risk to an ADR, backlog owner, or
  explicit post-Version-1.0 decision boundary.

Cloud tests may validate repository-contained code and portable behavior, but
they do not replace Windows, native Linux, SCM, systemd, ACL, peer-identity, or
real process-lifecycle evidence.

## Related documents

- [Security model](security.md)
- [Architecture](architecture.md)
- [Testing](testing.md)
- [Capability profiles and tool exposure](adr/009-capability-profiles-and-tool-exposure.md)
- [Operations and Job Manager](adr/010-operations-and-job-management.md)
- [Managed process and command execution](adr/011-managed-process-and-command-execution.md)
- [Native multi-mode runtime](adr/014-native-multi-mode-runtime-and-local-service-deployment.md)
- [Execution identity](adr/015-hybrid-service-execution-identity.md)
- [Host ownership and lifecycle](adr/017-host-process-ownership-and-lifecycle.md)
