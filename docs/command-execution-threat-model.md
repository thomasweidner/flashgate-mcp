# Command Execution Threat Model

**Status:** Normative Version 1.0 security contract

**Owner:** BL-136
**Scope:** typed, allowlisted command execution through FlashGate

## Security objective

FlashGate may start a configured native executable only when the effective
caller, profile, capability, root, command definition, arguments, environment,
limits, and execution backend all authorize the same operation. Command
execution must not become a general shell, an interpreter gateway, or a way to
escape filesystem and process policy.

This document threat-models the command boundary. It does not implement
`run_command`, choose platform sandbox mechanisms, set final resource-limit
values, or authorize external PID control or interactive input.

## Trust boundaries and protected assets

The following inputs are untrusted even when they arrive in a valid MCP
request:

- the command identifier and every typed argument value;
- paths, working-directory selections, environment values, timeouts, and
  output requests;
- files reachable below an allowed root, including files changed after
  validation;
- executable lookup state, inherited process state, and operating-system error
  text; and
- subprocess stdout, stderr, exit status, and descendants.

Server-owned command definitions, effective authorization policy, configured
root bindings, executable identity pins, limit maxima, and backend selection
are trusted only after startup validation. The protected assets are host files
and credentials, caller and service identities, policy/configuration, process
and job state, audit records, server availability, and data belonging to other
principals or roots.

## Authorization and execution invariants

Every invocation must satisfy all of these invariants immediately before the
process is created:

1. Resolve a closed `command_id` through server configuration. Never accept an
   executable path, shell text, or interpreter program from tool input.
2. Enforce the execution capability and risk policy server-side after tool
   resolution. Catalog visibility and MCP annotations are not authorization.
3. Resolve the configured executable to an approved absolute native-program
   path. When configured, verify its file identity or content fingerprint and
   fail closed if identity cannot be established.
4. Construct `argv` from a closed typed argument schema. Reject unknown fields,
   positional spillover, response-file syntax, and values that can select
   configuration, hooks, plugins, loaders, interpreters, or alternate
   executables unless that exact behavior is explicitly defined and reviewed.
5. Bind every path argument and working directory to an authorized named root
   and relative path. Revalidate effective containment at use time.
6. Build a minimal environment from an allowlist. Do not inherit credentials,
   loader variables, search-path overrides, startup hooks, proxy settings, or
   interpreter controls by default.
7. Apply server-owned runtime, concurrency, output, and temporary-data limits.
   A request may select only values at or below validated maxima.
8. Start the process through the single Managed Process Engine and bind its
   opaque handle and result to principal, profile, root, backend, and service
   generation. PID is diagnostic, not authorization identity.
9. Keep stdout and stderr separate, bounded, and redacted. Process output is
   untrusted data and must never be copied to the MCP transport as protocol
   framing or to diagnostics without safe encoding and redaction.
10. Deny network access when the command definition says it is disabled; never
    infer isolation merely because a command is expected not to use the
    network.

Validation and process creation are one security-sensitive transaction. If an
executable, root, working directory, or policy binding changes between checks
and use, the invocation fails closed; an implementation must not silently
retry against a different target.

## Threats and required controls

| Threat | Required control | Failure outcome |
|---|---|---|
| Executable substitution through `PATH`, aliases, links, replacement, or writable parents | Server-owned absolute path; no request-controlled lookup; native identity/fingerprint revalidation where configured; platform-safe open/launch design | Reject before start |
| Shell or interpreter injection | No shell command string; no implicit shell; executable IDs cannot resolve to an interpreter for Version 1.0 | Invalid arguments or policy denial |
| Argument-boundary injection | Closed typed schema and server-generated `argv`; preserve argument boundaries; reject unknown or ambiguous forms | Invalid arguments |
| Response-file, config, hook, plugin, or loader injection | Definition-specific deny rules for prefixes, flags, file arguments, environment, and implicit discovery locations | Invalid arguments or policy denial |
| Root or working-directory escape | Root ID plus relative path; lexical and effective containment; symlink/reparse policy; use-time revalidation | Root/policy denial |
| Environment or credential disclosure | Minimal allowlist; no ambient secret propagation; centralized redaction for results, diagnostics, and audit | Policy denial or redacted failure |
| Output flooding, blocking, or control-sequence abuse | Separate bounded streams, truncation markers, slow-reader-independent draining, safe serialization, cancellation | Bounded/truncated result or controlled termination |
| Runtime, fork, memory, CPU, or concurrency exhaustion | Server deadlines; process/descendant accounting; global and per-principal budgets; platform isolation with documented fallback | Overload rejection or controlled termination |
| Network access or data exfiltration | Per-definition network policy backed by an enforceable platform mechanism when denial is claimed; otherwise disclose the residual risk and do not claim isolation | Policy denial when required isolation is unavailable |
| Privilege escalation or confused deputy | Explicit effective backend, least privilege, no request-selected identity, capability/root checks under caller identity | Authorization denial |
| PID reuse, cross-principal control, or stale handle | Opaque identity-bound handles, process-start identity, service generation, owner checks | Unavailable/authorization denial |
| Timeout/cancellation leaves descendants or temporary data | One managed engine, bounded termination escalation, descendant cleanup, incomplete markers, TTL cleanup | Typed terminal state with bounded cleanup |
| Error or audit leakage | Stable categories, secret/host-path redaction, bounded fields, no raw OS error on public channels | Safe categorized failure |

## Platform-isolation boundary

Windows and Linux may use different native mechanisms, but must achieve the
same policy outcomes. Unsupported controls must fail closed when a command
definition requires them. A timeout alone is not CPU or memory isolation, a
working directory is not filesystem isolation, and a service account is not a
sandbox. Cross-building or synthetic tests do not prove native identity,
descendant cleanup, resource enforcement, or network isolation.

The concrete Windows and Linux isolation choices remain owned by BL-146 and
BL-147. Least-privilege execution identity remains owned by BL-149. This model
therefore defines required outcomes without selecting a new platform
mechanism.

## Result, error, and audit contract

Public results expose only the stable command definition identity, managed
status, bounded output or result references, exit information permitted by the
definition, timeout/cancellation state, and safe truncation or cleanup markers.
They do not expose absolute executable paths, host root paths, raw environment,
credentials, platform handles, or raw operating-system errors.

Security-relevant audit events record a safe correlation identifier, caller
principal, effective profile/capability decision, root and command IDs,
execution backend, policy outcome, managed handle, terminal state, applied
limits, and redacted failure category. Audit data follows the same ownership,
retention, size, redaction, and backpressure controls as other stateful domains.

## Required validation

Before Version 1.0 command execution is accepted, tests must cover:

- unknown and unauthorized command IDs, direct-handler/catalog bypass, and
  stale-policy calls;
- executable replacement, links/reparse points, writable-parent and identity
  mismatch cases on native Windows and Linux;
- quoting and argument-boundary fixtures, empty/Unicode values, unknown fields,
  response files, config/plugin/hook/loader flags, and shell metacharacters;
- absolute, traversal, cross-root, symlink/reparse, and working-directory race
  cases;
- environment allowlist behavior and credential/loader/proxy/interpreter
  exclusion plus redaction in results, diagnostics, and audit;
- independent stdout/stderr limits, slow readers, binary/control bytes,
  truncation, cancellation, timeouts, and descendant cleanup;
- global/per-principal concurrency and runtime exhaustion, temporary-data TTL,
  restart, PID reuse, ownership, and race-detector coverage;
- enforceable network-denied and least-privilege cases for definitions that
  claim those properties; and
- proof that `run_command` uses the Managed Process Engine rather than a second
  launch path.

Native Windows/Linux validation is mandatory for OS identity and isolation
claims. Cloud/unit tests may validate parsers, policy composition, schemas,
state transitions, and synthetic failure paths but cannot replace that proof.

## Residual risks and non-goals

An approved native program can contain vulnerabilities, interpret data files,
spawn descendants, or access resources allowed to its effective OS identity.
Allowlisting reduces reach; it does not make arbitrary programs safe. Native
sandbox controls vary by platform and may not provide perfect filesystem,
network, CPU, or memory isolation. These limits must be documented per command
definition and platform.

Version 1.0 does not provide arbitrary executable paths, free-form shell
commands, interpreter adapters, interactive shells, stdin streaming, external
PID control, request-selected users, or a claim of container-grade isolation.
