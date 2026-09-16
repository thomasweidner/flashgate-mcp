# Runtime Configuration, Endpoint Discovery, and Logging Contract

**Status:** Version 1.0 contract; implementation remains planned
**Owner:** BL-233
**Applies to:** `stdio`, `proxy`, `auto`, and system `service` runtime roles

This document fixes the configuration precedence, local endpoint names,
discovery outcomes, timeout categories, and diagnostic destinations for the
native multi-mode runtime accepted by ADR-014. It does not claim that those
runtime modes are implemented.

## Security invariants

- Configuration never grants authority by itself. The service still derives
  the caller identity from the operating system and applies server-side root,
  profile, capability, quota, and execution-backend policy.
- An endpoint is local or it is rejected. TCP, HTTP, UNC paths, remote named
  pipes, abstract Unix sockets, and user-supplied file descriptors are outside
  the Version 1.0 contract.
- Command-line arguments, environment variables, diagnostics, and errors must
  not contain credentials or configuration values classified as secrets.
- Invalid explicit configuration fails closed. It is never treated as if the
  setting were absent, and `auto` never falls back after a managed endpoint
  rejects authorization, policy, or protocol compatibility.
- Standard output remains protocol-only for `stdio`, `proxy`, and `auto`.

## Sources and precedence

For client-facing `stdio`, `proxy`, and `auto` processes, each setting is
resolved independently in this order, from highest to lowest precedence:

1. an explicit CLI option;
2. a `FLASHGATE_*` environment variable;
3. the per-user configuration file;
4. the system configuration file;
5. the compiled safe default.

An absent value falls through; an empty or invalid value at any present source
is an error. Configuration files are read once during startup. Duplicate keys,
unknown keys, malformed UTF-8, unsupported schema versions, and files that
exceed the implementation's documented bounded size are rejected.

The system `service` role has a deliberately narrower trust boundary:

1. service-management CLI options supplied during explicit installation or
   validation;
2. the system configuration file;
3. compiled service-safe defaults.

The running service ignores per-user configuration and `FLASHGATE_*`
variables. Its service manager provides the system configuration path; the
path cannot be selected by an IPC client. Root, profile, identity, ACL, and
authorization policy are service-owned settings and are never overridden by a
client CLI or environment variable.

### Configuration paths

| Scope | Windows | Linux |
|---|---|---|
| System | `%ProgramData%\FlashGate MCP\config.toml` | `/etc/flashgate-mcp/config.toml` |
| User | `%AppData%\FlashGate MCP\config.toml` | `${XDG_CONFIG_HOME:-$HOME/.config}/flashgate-mcp/config.toml` |

Environment expansion in a configuration-file value is prohibited. Relative
configuration paths and paths containing an unresolved variable are rejected.
Missing optional client configuration files are equivalent to an absent
source; an unreadable or malformed existing file is fatal. A missing service
configuration is fatal unless installation generated and validated the
documented minimal configuration.

The Version 1.0 configuration format is TOML with a required top-level
`schema_version = 1`. The eventual implementation must publish the complete
closed key catalog and parser limits with its configuration package; this
contract does not make currently unimplemented keys user-facing.

## Local endpoint identity

The canonical Version 1.0 system-service endpoints are:

| Platform | Canonical endpoint |
|---|---|
| Windows | `\\.\pipe\flashgate-mcp-v1` |
| Linux | `/run/flashgate-mcp/flashgate-mcp-v1.sock` |

The `v1` suffix versions the transport endpoint namespace, not MCP. A future
incompatible local transport uses a new endpoint name and must not silently
reuse this one.

An explicit endpoint may be supplied only to `proxy` or `auto`, via
`--endpoint`, `FLASHGATE_ENDPOINT`, or the resolved client configuration
`endpoint` key. `stdio` rejects endpoint settings as inapplicable. `service`
uses the canonical platform endpoint from system configuration and rejects a
client-provided override.

Endpoint validation occurs before connection:

- Windows accepts only a local `\\.\pipe\` name with a single bounded leaf;
- Linux accepts only an absolute filesystem Unix-socket path;
- both reject traversal, control characters, empty components, remote forms,
  and values over the platform-specific documented bound;
- Linux discovery rejects symlinks and non-socket filesystem objects;
- endpoint ACL/owner and OS peer identity checks remain mandatory after a
  syntactically valid endpoint is found.

## Discovery and fallback

`proxy` requires one resolved managed endpoint. Absence, connection failure,
authorization denial, policy denial, and compatibility failure all terminate
with a typed non-zero outcome; direct execution is never attempted.

`auto` follows exactly this state machine:

1. Resolve and validate an explicit endpoint when configured; otherwise use
   the canonical platform endpoint.
2. Classify discovery as `absent`, `present`, or `invalid` without starting,
   installing, repairing, or elevating a service.
3. Fall back to direct STDIO only when the canonical, non-explicit endpoint is
   conclusively `absent` and fallback policy is `allowed`.
4. Treat an invalid endpoint object, ACL/owner mismatch, connection error to a
   present endpoint, authentication or authorization denial, policy rejection,
   overload, and protocol/version incompatibility as terminal managed-mode
   failures. None permits fallback.

An explicitly configured endpoint expresses managed intent: its absence is a
terminal configuration/discovery failure even when general fallback policy is
`allowed`. `managed-required` policy makes every absence terminal. Discovery
does not scan directories, enumerate alternate pipes/sockets, or retry another
endpoint after a failure.

## Timeouts and retries

Configuration exposes separate bounded durations for `connect`, `request`,
`shutdown`, and (where the IPC contract permits it) `retry_backoff`. Each has a
nonzero compiled default and a documented maximum. Zero, negative, overflowed,
or over-maximum values fail validation.

Startup discovery performs one connection attempt. Automatic retries are not
part of Version 1.0 startup because they blur `absent` and `present but
unhealthy`. Retry metadata may be returned to callers for an overload result,
but a client action initiates any later attempt. Request expiry cancels only
the bound request/connection-owned work; service lifetime remains independent.
Shutdown uses the bounded lifecycle contract in ADR-017 and never converts an
expired graceful window into success.

## Diagnostics, audit, and log destinations

| Runtime role | Destination | Required behavior |
|---|---|---|
| Direct `stdio` | process `stderr` | redacted diagnostics; never stdout |
| `proxy` / `auto` edge | process `stderr` | redacted connection diagnostics; never stdout |
| Windows system service | Windows Event Log | source `FlashGate MCP`; no client-selected file path |
| Linux system service | journald | service unit identity; no client-selected file path |

Version 1.0 does not accept an arbitrary log-file destination from a client.
Verbosity is bounded to `error`, `warn`, `info`, and `debug`; the default is
`info`. `debug` does not relax redaction and must not emit request payloads,
file contents, command arguments, environment values, credentials, raw peer
tokens, or absolute root paths. Audit records follow their separate lifecycle
contract and cannot be disabled by client configuration.

Before emitting a diagnostic, implementations normalize untrusted text to
prevent log injection and attach only bounded, safe identifiers such as event,
correlation, endpoint category, runtime role, and typed outcome. Failure to
open a mandatory service diagnostic/audit sink is a startup failure unless a
separately accepted audit-lifecycle contract defines a bounded safe-degraded
state.

## Required contract tests

The implementation must provide table-driven tests for:

- every precedence pair, absent-source fallthrough, and invalid-highest-source
  failure;
- the service role's rejection of user configuration and environment input;
- exact platform defaults and rejection of remote, relative, traversal,
  symlink, non-socket, malformed, and overlong endpoint values;
- every `auto` discovery/fallback cell, including the rule that only a
  non-explicit canonical endpoint's conclusive absence may fall back;
- timeout lower/upper bounds and the single-attempt startup rule;
- stdout purity and destination selection for all runtime roles;
- redaction and log-injection cases at every verbosity level.

Native Windows pipe ACL/identity and Event Log evidence, and native Linux Unix
socket ownership/peer-credential and journald evidence, remain platform
finalization gates. Unit tests cannot substitute for them.

## Related contracts

- [ADR-014: Native Multi-Mode Runtime and Local Service Deployment](adr/014-native-multi-mode-runtime-and-local-service-deployment.md)
- [ADR-017: Host Process Ownership and Lifecycle](adr/017-host-process-ownership-and-lifecycle.md)
- [Native multi-mode runtime and service plan](native-multi-mode-runtime-and-service-plan.md)
- [Security model](security.md)
- [Testing](testing.md)
