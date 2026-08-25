# FlashGate MCP Version 1.0 Product and Technical Specification

## Document role

This specification consolidates the accepted Version 1.0 requirements. `BACKLOG.md` remains authoritative for task status and sequencing. ADRs remain authoritative for individual architecture decisions.

## Product objective

FlashGate MCP is a native Windows/Linux local host-operation gateway for MCP clients. It minimizes response latency, model-token consumption, RAM, CPU, runtime dependencies, and attack surface while providing controlled filesystem, search, process, command, system-information, and long-operation capabilities.

## Mandatory properties

Version 1.0 shall:

1. ship one primary native PE/ELF executable per supported platform;
2. require no Python, PHP, Node.js, Java, or other interpreter for normal runtime;
3. support direct non-admin STDIO operation;
4. optionally support Windows SCM and Linux systemd system-service operation through the same binary;
5. enforce authorization, roots, profiles, risk policy, limits, execution identity, and audit server-side;
6. remain local-only with no remote TCP/HTTP listener;
7. provide deterministic bounded contracts and reproducible performance/security evidence;
8. publish an explicit protocol and extension compatibility matrix;
9. meet the release gate in `BL-263`.

## Host-lifecycle nonfunctional requirements

The accepted target shall assign an explicit authoritative owner and expected
lifetime to every direct, proxy/auto edge, system service, future user host,
and future worker role. Session-scoped hosts shall use one process-root
coordinator and complete deterministic bounded shutdown after a definitive
signal, including cancellation, drain, owned-child/domain cleanup, temporary
resource cleanup, and a typed final result.

Instance and exit diagnostics shall be bounded and secret-safe and shall bind
PID to process start identity or a verified operating-system handle. PID alone
is not authoritative. Age, idle time, CPU use, active-request count, and
singleton assumptions shall never independently authorize termination.
Ambiguous live-owner/live-transport cases shall be classified
`SUSPECTED_STALE`, not automatically killed. After conclusive ownership or
transport loss plus the bounded shutdown window, zero
`DEFINITELY_ORPHANED` session-scoped hosts or owned children may remain.

## Runtime modes

- `stdio`: direct MCP server in the caller process identity.
- `proxy`: STDIO-compatible MCP facade to an installed local service.
- `auto`: compatible service discovery with strictly limited safe fallback to direct mode.
- `service`: system host managed by SCM/systemd.
- internal backend/worker roles: not directly exposed as normal client entry points.

Service installation may require administrative rights; using a correctly installed service does not require the client to run as administrator when endpoint and policy rules authorize it.

## Execution identity

The architecture is hybrid per root:

- `current-process`: direct mode executes under the process identity;
- `service-account`: Version 1.0 system-service backend for administratively ACL-granted roots;
- `user-worker`: reserved backend contract, rejected safely in Version 1.0, implemented post-Version-1.0;
- in-process impersonation: prohibited.

Every request carries an authenticated caller principal separately from its effective execution backend. Authorization, quotas, state ownership, and audit use the caller principal; OS access uses the selected backend. Backend selection is administrative configuration, never a normal tool argument.

## Functional domains

### Filesystem

Required bounded capabilities include metadata, pages, ranges, text/media/binary reads, batches, hashes/fingerprints, trees, targeted edits, atomic/conditional writes, dry-run, append, bounded plans, cross-volume semantics, directory copy/move/size, and job handoff.

### Search

Required bounded capabilities include path/name/metadata/literal/regex search, include/exclude patterns, depth/file/scanned-byte/match/context limits, binary behavior, deterministic ordering, pagination, and a pure-Go baseline. Optional ripgrep and persistent indexing are post-Version-1.0.

### Operations and jobs

Required capabilities include opaque handles, lifecycle, status, cancellation, deadlines, progress, bounded results/pages, TTL, cleanup, ownership, per-principal limits, and domain-neutral orchestration. Domain services retain domain logic.

### Process observation and management

Required capabilities include bounded observation, process identity/start-time protection, managed process startup without shell, separate stdout/stderr ring buffers, cursor reads, process-tree cleanup, environment/working-directory/timeout/output controls, and job integration. External PID control and interactive input are post-Version-1.0.

### Typed command execution

Commands use server-defined IDs and closed typed arguments. Executable resolution, subcommands, flags, positionals, paths, environment, working directory, network policy, timeout, output, concurrency, process tree, identity pinning, and injection protections are explicit. Arbitrary shell strings and interpreter hosting are not provided.

### System information

Only explicit, bounded, redacted, capability-gated facts are exposed. Privacy-sensitive network information is post-Version-1.0.

## Security requirements

Version 1.0 shall provide:

- fail-closed roots and safe read-only default profile;
- named roots and explicit profile/capability/risk policy;
- lexical and effective path containment including symlink/reparse policy;
- OS-derived local service caller identity;
- endpoint ACL/ownership hardening;
- dedicated least-privilege service account, not root/LocalSystem by default;
- principal/root/profile/backend/service-instance binding for handles and cached state;
- global, per-domain, and per-principal quotas plus fair scheduling;
- safe auto-mode fallback rules;
- normalized redacted errors and diagnostics;
- bounded correlated audit lifecycle;
- negative cross-user, traversal, race, command-injection, fallback-bypass, and privilege-escalation tests.

## Efficiency requirements

Version 1.0 shall:

- expose only profile-authorized tools;
- budget tool count, catalog bytes/tokens, and initialization instructions;
- keep tool ordering and fingerprints deterministic;
- use ranges, pages, batches, field selection, cursors, and jobs to avoid repeated/unbounded transfer;
- transmit payload-heavy content once;
- separate compact metadata from text/binary/process/search payload;
- provide bounded opaque result/resource handoff;
- record useful payload, result, response, request, IPC, and approximate-token bytes;
- measure wire amplification and token cost per useful byte;
- measure direct/proxy/service latency, CPU, memory, and concurrency;
- retain hard deterministic budgets and reviewed host-sensitive soft budgets;
- complete a pinned cross-project comparison without unmeasured performance claims.

## Native dependency rule

Implementation order is Go standard library, platform-specific Go adapter, direct OS API/stable OS interface, then an external native program only after measured need and security review. Runtime interpreter dependencies are excluded. Administrator deployment scripts may use the target OS scripting environment when clearly separated from the MCP runtime.

## Protocol requirements

- protocol/extension negotiation is explicit;
- advertised revisions are implemented and tested;
- tool schemas use the selected JSON Schema requirements and deterministic snapshots;
- result representation follows payload classes;
- annotations are accurate but not authorization;
- long-lived state uses opaque principal-bound handles;
- internal jobs remain independent from negotiated MCP Tasks;
- deprecated MCP Roots/Sampling/Logging are not core dependencies;
- service IPC is local, versioned, authenticated, bounded, and fail-closed.

## Audit and observability

Every security-relevant request records a safe correlation chain across client, proxy, service, backend, domain operation/job, and OS result. The lifecycle defines rotation, retention, access, redaction, queue/backpressure, disk-full behavior, and shutdown flush limits. Metrics and traces are optional/exportable without making a heavy telemetry runtime mandatory.

## Packaging and supply chain

Release evidence includes Windows/Linux assets, checksums, dependency inventory, SBOM, provenance, signing plan and available signatures, reproducibility comparison or limitations, service assets, installation/removal/rollback instructions, and no silent automatic update.

Current native artifact identity uses one canonical
SemVer/build-information model and an equivalent statically inspectable build
manifest in all binaries. Windows x64/ARM64 binaries expose `VERSIONINFO` and
the exact committed application icon; Linux x64/ARM64 binaries expose matching
CLI, Go/VCS, ELF architecture, and Go build-ID metadata. Release archives use
public `x64`/`arm64` names, exact documented regular-file/directory types, and
sibling SHA-256 files. Upload requires independent two-build reproduction and
a machine-readable leak scan. `.deb`, `.rpm`, and systemd metadata remain
deferred until those distribution assets are approved.

## Version 1.0 exclusions

Version 1.0 does not implement:

- per-user worker backend;
- Linux user service or Windows per-user persistent host;
- conditional read/not-modified optimization;
- ripgrep acceleration or persistent index;
- external PID control or interactive shell/input;
- network-information capability;
- external provider/plugin ecosystem;
- remote transport, cloud hosting, or automatic elevation.

These exclusions do not permit Version 1.0 interfaces to block the accepted later architecture.

## Acceptance

Release is accepted only when all `Planned` tasks are complete or explicitly waived in a reviewed release record, every public contract matches documentation, security and efficiency budgets pass, Variant B configuration is rejected safely, Variant C is absent, and the release artifacts are reproducible, traceable, and rollback-capable.

## Related documents

- [Authoritative backlog](../BACKLOG.md)
- [Version 1.0 scope](version-1-scope-and-release-boundary.md)
- [Architecture](architecture.md)
- [Security](security.md)
- [Protocol](protocol.md)
- [Efficiency plan](efficiency-improvement-plan.md)
- [Execution identity backends](execution-identity-backends.md)
- [Testing](testing.md)
