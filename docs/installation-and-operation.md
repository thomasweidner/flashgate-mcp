# Installation and operation

This guide separates the FlashGate behavior available in the current checkout
from the accepted multi-mode target. Do not use a target-state example as an
implemented command. The binary currently supports direct STDIO only; system
service, proxy, and automatic-discovery modes are not implemented.

## Current supported deployment: portable direct STDIO

1. Obtain the native `flashgate-mcp` binary for the host platform and verify
   the archive and checksum using [artifact verification](artifact-verification.md).
2. Extract the binary into a user-controlled local directory. No installer,
   administrator rights, service account, interpreter, or network listener is
   required.
3. Configure an absolute existing directory in `MCP_ROOT`. Set
   `MCP_READ_ONLY=true` when only read access is required. Relative paths are
   rejected except for the explicit development-only combination
   `MCP_ROOT=.` and `MCP_ALLOW_CWD_ROOT=true`.
4. Configure the MCP client to start `flashgate-mcp` with no arguments and to
   communicate over STDIO. Standard output is reserved for JSON-RPC.
5. Confirm the selected binary with `flashgate-mcp --version --verbose` and
   inspect supported invocation forms with `flashgate-mcp --help`.

Removing a portable deployment means stopping its MCP client session and then
removing the extracted files. Preserve configuration or logs only when local
policy requires them. FlashGate currently creates no service registration,
scheduled task, background host, persistent logfile, or network endpoint.

Direct STDIO is the supported non-administrator path for Version 1.0. Each MCP
client transport owns its launched FlashGate process. Normal STDIN EOF or
transport closure ends that session; multiple simultaneous direct processes
can therefore be legitimate.

## Current diagnostics

| Symptom | Check | Safe response |
|---|---|---|
| Exit code `2` | The command line contains an unknown or unsupported argument. | Run `flashgate-mcp --help`; do not substitute a planned `--mode` or service-management command. |
| Exit code `3` before protocol input | Root configuration is missing, invalid, or denied by policy. | Supply an absolute existing `MCP_ROOT` and review the root policy. Do not weaken path policy merely to make startup succeed. |
| Exit code `1` | Startup or runtime failed outside the expected configuration categories. | Enable `MCP_DEBUG=true` only for bounded redacted stderr diagnostics; keep stdout untouched. |
| Client receives no valid JSON-RPC | The client may not be using STDIO or another program may be writing to stdout. | Start the binary directly as the MCP STDIO command and keep wrappers from emitting stdout. |
| Access is denied | Root confinement, read-only registration, or filesystem policy rejected the request. | Correct the requested relative path or the explicitly administered policy; never bypass the denial through another runtime mode. |
| A process appears idle | Idle time alone does not prove an orphan. | Close the owning MCP session. Do not terminate by executable name, age, PID alone, or a singleton assumption. |

Errors and debug output must remain free of credentials and unnecessary host
paths. Do not post configuration, filesystem roots, or raw logs publicly
without reviewing and redacting them.

## Planned system-service deployment: not operational

The accepted target uses the same native binary for explicit `stdio`, `proxy`,
`auto`, and operating-system `service` roles. Windows SCM plus a local Named
Pipe and Linux systemd plus a local Unix Domain Socket are planned. They do not
exist in the current CLI, release assets, or supported operational surface.

Consequently, the following are **not current instructions**:

```text
flashgate-mcp --mode stdio
flashgate-mcp --mode proxy
flashgate-mcp --mode auto
flashgate-mcp --mode service
flashgate-mcp install|uninstall|start|stop|status
```

Do not manufacture an SCM service, systemd unit, endpoint, account, ACL,
configuration path, or management wrapper from these examples. Exact command
syntax, endpoint names, configuration precedence, service assets, accounts,
permissions, logging destinations, and recovery policy remain owned by their
implementation tasks. `auto` will never install software, elevate privileges,
or fall back after a present managed endpoint denies authorization, policy, or
protocol compatibility.

The planned system service is persistent across client disconnects and uses a
dedicated least-privilege service identity with explicitly granted roots. It
must not default to LocalSystem or root. A Windows per-user background host and
Linux `systemd --user` host are post-Version-1.0 plans; direct STDIO remains the
installation-free user path.

## Future upgrade, rollback, and removal gate

Before managed deployment can be documented as operational, its release must
provide and validate all of the following on the real target platform:

- exact install, status, start, stop, restart, removal, and exit contracts;
- signed or checksummed native artifacts and the applicable service assets;
- dedicated account, endpoint ownership/ACL, configured-root, and log policy;
- configuration validation before accepting clients;
- explicit proxy/service compatibility and upgrade order;
- a rollback procedure that restores the previous binary, configuration,
  service definition, endpoint ownership, and availability without weakening
  authorization; and
- removal verification covering the service registration or unit, endpoint,
  runtime state, temporary resources, and intentionally retained logs/config.

Rollback must be explicit and local: there is no silent automatic update. A
failed managed authorization or compatibility check is not permission to fall
back to direct mode. Until those gates pass, use the portable direct-STDIO
procedure above rather than extrapolating service commands from target-state
architecture documents.

## Related contracts

- [Native multi-mode runtime and service plan](native-multi-mode-runtime-and-service-plan.md)
- [ADR-014: Native multi-mode runtime and local service deployment](adr/014-native-multi-mode-runtime-and-local-service-deployment.md)
- [ADR-015: Hybrid service execution identity](adr/015-hybrid-service-execution-identity.md)
- [ADR-017: Host process ownership and lifecycle](adr/017-host-process-ownership-and-lifecycle.md)
- [Security model](security.md)
- [Testing](testing.md)
