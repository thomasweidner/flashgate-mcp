# Process and Execution Security

This document describes the accepted Version 1.0 security contract for process
observation, managed processes, and typed command execution. It is an
implementation and review target, not a statement that these planned features
are available today. The current runtime exposes filesystem tools only; see
[`BACKLOG.md`](../BACKLOG.md) for delivery status.

The architecture authority is
[ADR-011](adr/011-managed-process-and-command-execution.md). Capability and
profile rules are defined by
[ADR-009](adr/009-capability-profiles-and-tool-exposure.md), and domain
separation is defined by
[ADR-007](adr/007-domain-separated-local-system-core.md).

## Security invariants

Every process or command request is authorized and validated server-side.
Tool visibility, MCP annotations, caller-supplied identifiers, and knowledge of
a handle or PID never grant permission. The effective policy binds the caller,
profile, capability, named root, execution backend, service generation, and
applicable limits before work starts and whenever state is read or controlled.

The process and command domains share these invariants:

- opaque server-issued handles are the authority for server-managed process
  state; a PID is diagnostic data only;
- observation, management, and external-process control are separate
  capabilities;
- command IDs resolve server-side to approved absolute executables and typed
  argument rules;
- the server constructs the argument vector without a shell;
- working directories and path arguments are relative to authorized named
  roots and are revalidated at execution time;
- environment variables use an explicit allowlist and secret-bearing values
  are neither inherited nor returned by default;
- stdout and stderr remain separate, bounded, cursor-addressed streams with
  explicit truncation and expiry;
- runtime, output, process-count, concurrency, and temporary-data limits are
  enforced by the server; and
- status, output, wait, stop, cleanup, and audit access repeat ownership and
  policy checks rather than trusting prior discovery.

## Handles, PIDs, and lifecycle

A managed-process handle identifies one server-started process instance and is
bound to its owner and execution context. Handles are opaque, non-guessable,
not derived from a PID, never reused, and do not reveal host paths or process
identifiers. Unknown, stale, expired, cross-principal, cross-profile, and
cross-generation handles fail with the same safe category where distinguishing
them would disclose state.

PID reuse must not associate a handle with a different operating-system
process. Implementations therefore retain and validate platform process
identity in addition to the PID. A PID may appear in explicitly selected
diagnostic output only after redaction and policy checks. It is never accepted
as a substitute for a managed handle.

Normal control is restricted to processes started by FlashGate. External PID
control remains post-Version 1.0 and requires its own high-risk capability,
policy, threat model, and explicit enablement. Server shutdown, timeout,
cancellation, caller disconnect, process exit, and restart have deterministic
final states and bounded cleanup. Ambiguous identity must fail closed rather
than signal an unrelated process.

## Typed command definitions

The client supplies a configured command ID and structured values, not an
executable path or command line. A command definition constrains:

- the canonical executable and, where required, its binary identity;
- fixed subcommands and allowed flags;
- each value's type, range, length, and placement;
- path arguments, named-root binding, and working directory;
- permitted environment names and fixed values;
- timeout, output, concurrency, network, and platform-isolation policy; and
- the profile and functional capability allowed to invoke it.

Unknown command IDs and invalid values fail before process creation. Standard
profiles reject shell strings, response files, executable search through
`PATH`, arbitrary configuration injection, uncontrolled environment
inheritance, hooks, plugins, loaders, and interpreter fallback. A synchronous
`run_command` is a bounded wrapper over the Managed Process Engine, not a
second execution path.

Interactive input and interactive shells are outside Version 1.0. They must not
be enabled by treating an argument, environment value, or executable as an
implicit shell escape.

## Isolation and platform boundary

Windows and Linux adapters may use different native mechanisms but must
produce equivalent policy outcomes. Least-privilege identity, process-tree
termination, CPU and memory controls, signal semantics, and orphan prevention
require native platform validation. An unavailable hard isolation control must
produce the configured fail-closed result; it must not silently degrade to an
unrestricted process.

The normal runtime does not require PowerShell, Bash, Python, Node.js, Java, or
another interpreter. An approved native external program is invoked directly,
without a shell, only through a typed command definition. Development and
administrator scripts are not runtime command adapters.

## Output, errors, redaction, and audit

Command lines, working directories, environment values, output, and platform
errors can disclose secrets or host details. Client-visible results expose only
requested, policy-permitted fields and stable error categories. They do not
return raw OS errors, absolute host paths, environment dumps, credentials, or
unredacted command lines.

stdout and stderr have independent byte limits. Reads report cursor position,
truncation, expiry, and final-state information without replaying already
consumed data implicitly. Redaction occurs before client results, diagnostics,
or audit persistence; truncation must not bypass redaction at chunk boundaries.

Audit events correlate the authorization decision, command definition,
managed handle, caller, effective execution backend, lifecycle transition, and
bounded outcome. Audit data uses stable identifiers and redacted metadata, not
raw arguments or environment values by default. Audit failure follows the
configured backpressure/fail-closed policy and never spills records to MCP
stdout.

## Required negative validation

Implementation acceptance requires focused unit tests plus Windows and native
Linux lifecycle/security coverage for at least:

- direct calls when a tool is absent from the effective catalog or capability;
- guessed, malformed, stale, expired, cross-owner, and cross-generation
  handles;
- PID reuse and process-identity mismatch without unrelated-process control;
- unknown command IDs, executable substitution, `PATH` lookup, shell
  metacharacters, response files, and argument-rule bypasses;
- root escape through path arguments, working directories, links, or reparse
  points;
- environment injection and redaction of secrets across chunk boundaries;
- separate stdout/stderr limits, truncation, cursors, slow readers, and expiry;
- timeout, cancellation, stop races, server shutdown, crash/restart, and orphan
  cleanup;
- process-count, runtime, output, concurrency, CPU, and memory limit failures;
  and
- unavailable platform isolation controls producing deterministic fail-closed
  outcomes.

Race-enabled tests must cover handle ownership, lifecycle transitions, output
buffers, cancellation, and cleanup. Cross-platform tests must record the native
mechanism actually exercised and must not present cross-build or synthetic
checks as native isolation evidence.

## Explicit non-goals

This contract does not authorize external PID control, process input,
interactive shells, arbitrary executable paths, remote execution, implicit
privilege elevation, a second command engine, or silent isolation fallback.
Those boundaries require their separately tracked implementation and decision
work.
