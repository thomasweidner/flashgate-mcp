# CLI Mode and Host Lifecycle Contract

## Status and scope

This document is the normative Version 1.0 CLI mode and host-lifecycle
contract owned by `BL-223`. It specifies the target interface; it does not
claim that proxy, auto, service management, or the process-root lifecycle
coordinator is implemented. The current executable continues to support only
the no-argument STDIO form plus `--help` and `--version` until the owning
implementation tasks land.

ADR-014 fixes the four public runtime modes. ADR-017 fixes host ownership,
shutdown reasons, phases, evidence, and orphan-safety rules. This contract
binds those decisions to one unambiguous command-line and exit interface.

## Invocation grammar

The Version 1.0 public grammar is:

```text
flashgate-mcp [--mode stdio|proxy|auto|service]
flashgate-mcp service install
flashgate-mcp service uninstall
flashgate-mcp service start
flashgate-mcp service stop
flashgate-mcp service status
flashgate-mcp --version [--verbose]
flashgate-mcp --help
```

No arguments is exactly equivalent to `--mode stdio`. Mode names and service
verbs are lowercase ASCII and case-sensitive. Options and operands not shown
above, repeated `--mode`, mixed runtime and management forms, and trailing
arguments are usage errors. `--help` and `--version` are informational forms,
not combinable with runtime or management forms except for the documented
`--version --verbose` form.

`service` always means the operating-system system service. Version 1.0 does
not expose user-host or worker management through this grammar. Internal
worker syntax, if added for the post-Version 1.0 backend, is not a public
client entry point and requires its own contract and validation.

## Runtime-mode behavior

| Form | Role and owner | Required behavior |
|---|---|---|
| no arguments / `--mode stdio` | Session-scoped direct host owned by the MCP transport session | Run MCP JSON-RPC over STDIO under the current process identity. EOF or definitive transport loss initiates bounded shutdown. |
| `--mode proxy` | Session-scoped lightweight edge owned by the client transport session | Present MCP STDIO to the client and connect to an authorized compatible local service. Never fall back to direct execution. |
| `--mode auto` | Session-scoped proxy edge or direct fallback | Apply the documented discovery and fail-closed fallback matrix. Absence may permit direct STDIO; denial, policy rejection, or incompatibility never does. |
| `--mode service` | Persistent system host owned by SCM or systemd | Run only in the platform service-host context. A client disconnect cleans connection-owned work but does not terminate the service. |

Runtime forms never install, uninstall, start, or reconfigure a service. In
particular, `auto` performs no elevation, installation, or repair.

MCP-facing STDOUT is protocol-only in `stdio`, `proxy`, and `auto`. Service
mode has no MCP STDOUT contract; it uses the configured platform diagnostic
and audit sinks. Human diagnostics remain bounded, redacted, and free of
credentials and unnecessary host paths.

## System-service management

The service verbs are explicit one-shot administrative requests:

| Command | Contract |
|---|---|
| `service install` | Install the platform service from the invoked, validated artifact and configured system scope. Do not start it implicitly. |
| `service uninstall` | Remove a stopped service and its owned service metadata. Refuse while running; do not imply `stop`. |
| `service start` | Ask SCM/systemd to start an installed service and wait only for the bounded management result. |
| `service stop` | Ask SCM/systemd for bounded graceful stop. A forced termination is not reported as clean success. |
| `service status` | Read service installation and lifecycle state without mutating it. |

Management commands never self-elevate, prompt for UAC or `sudo`, install a
scheduled task, change unrelated ACLs, or silently switch scope. Insufficient
privilege is a stable denial, not a reason to retry through another mechanism.
Detailed platform installation, identity, ACL, rollback, and asset behavior
remains owned by `BL-230` and `BL-231`.

## Exit contract

The process returns one of these stable categories. Platform adapters map
native errors into the category; native numeric status values are not exposed
as the FlashGate exit code.

| Code | Symbolic category | Meaning |
|---:|---|---|
| `0` | `SUCCESS` | Informational/management request succeeded, or a runtime host exited cleanly after its accepted lifecycle. |
| `1` | `INTERNAL_FAILURE` | Unexpected internal, adapter, or startup failure not covered by a safer category. |
| `2` | `USAGE_ERROR` | Invalid command-line grammar, option, mode, or verb. |
| `3` | `CONFIGURATION_ERROR` | Invalid or unsafe FlashGate configuration/root preflight. This preserves the implemented startup contract. |
| `4` | `UNAVAILABLE_OR_INCOMPATIBLE` | Required endpoint, installed service, platform facility, or compatible peer is unavailable. |
| `5` | `ACCESS_DENIED` | OS authorization, endpoint authorization, service policy, or required administrative privilege denied the request. |
| `6` | `STATE_CONFLICT` | The requested transition conflicts with current state, such as install when installed or uninstall while running. |
| `7` | `SHUTDOWN_INCOMPLETE` | The bounded shutdown/management deadline expired or required owned cleanup did not complete. |

Expected failures emit a single safe category on STDERR where a console is
attached. They do not emit raw OS errors, secrets, endpoint credentials, or
unnecessary paths. Service-host reporting additionally follows the platform
service-manager contract. A lifecycle deadline expiry records the initiating
shutdown reason and `SHUTDOWN_DEADLINE_EXCEEDED`, returns code `7`, and must
not be represented as `SUCCESS`.

## Shutdown and ownership binding

Every runtime instance has the immutable mode/role, owner relationship, and
expected lifetime defined by ADR-017. Exactly one process-root lifecycle
coordinator accepts adapter signals. The first definitive signal wins; later
signals are idempotent observations. Stable shutdown reasons and ordered
cleanup phases are those in ADR-017 and are not redefined by CLI parsing.

The coordinator stops intake, cancels owned work, invokes cleanup through the
Operations/Job and Managed Process owners, removes owned temporary resources,
and finalizes bounded evidence before exit. Only `EXITED_CLEANLY` maps to exit
code `0`; deadline expiry or incomplete required cleanup maps to code `7`.

No CLI mode or management verb creates authority to terminate a process based
only on PID, parent PID, age, idle time, CPU use, request count, executable
name, or a singleton assumption. `SUSPECTED_STALE` instances are never
automatically terminated. Automatic cleanup requires the conclusive ownership
and transport evidence necessary for `DEFINITELY_ORPHANED`.

## Validation obligations

The implementing tasks must add table-driven parser tests for every accepted
form and representative rejected combinations, plus tests that bind each
runtime form to its role without starting an unintended mode. Platform tests
must prove management exit-category mapping, no implicit elevation or state
transition, STDOUT purity, bounded shutdown, clean versus forced exit, and the
complete ADR-017 lifecycle matrix assigned to `BL-241`.

Cross-compilation does not establish SCM, systemd, Named Pipe, Unix socket,
ACL, peer-identity, signal, or native process-lifecycle behavior. Those gates
require native Windows/Linux finalization.

## Related documents

- [ADR-014: Native multi-mode runtime and local service deployment](adr/014-native-multi-mode-runtime-and-local-service-deployment.md)
- [ADR-017: Host process ownership and lifecycle](adr/017-host-process-ownership-and-lifecycle.md)
- [Native multi-mode runtime and service plan](native-multi-mode-runtime-and-service-plan.md)
- [Security model](security.md)
- [Testing](testing.md)
- Authoritative backlog: `BACKLOG.md`
