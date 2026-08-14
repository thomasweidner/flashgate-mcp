# FlashGate MCP Backlog

This is the authoritative planning and steering document for FlashGate MCP.

FlashGate MCP uses repository `thomasweidner/flashgate-mcp`, local directory `flashgate-mcp`, Go module `github.com/thomasweidner/flashgate-mcp`, binary `flashgate-mcp`, and MCP server implementation name (`serverInfo.name`) `flashgate`.

## Working rules

- Keep `README.md`, `CHANGELOG.md`, and `BACKLOG.md` current in every sprint.
- Define every canonical task exactly once in the task catalog; sprint tables reference IDs only.
- Keep dangerous capabilities disabled or restricted by default.
- Keep protocol output on stdout and diagnostics on stderr.
- Mark target and planned behavior explicitly; do not present it as implemented.
- Preserve completed tasks with status `Done`.
- Apply the binding change-trigger review in `Governance/CHANGE-TRIGGER-REVIEW-AND-BACKLOG-STANDARD.md`. Reuse completed gates, update coherent open work, and register new independently reviewable work after duplicate search.
- Complete the current sprint's ID migration document before merge. After merge, dated migration files are immutable history; any later full renumbering creates a new dated migration file and may add a small migration index.

> Neue Aufgaben werden an der fachlich richtigen Position eingefügt. Danach werden alle nachfolgenden BL-IDs repositoryweit fortlaufend umnummeriert.

## Status legend

| Status | Meaning |
|---|---|
| Ready | Clear and ready for implementation |
| Planned | Accepted work required for Version 1.0 unless its task text explicitly defines a continuous gate |
| Later | Accepted post-Version-1.0 work; not required for the initial stable release |
| Blocked | Waiting for a decision or dependency |
| Done | Completed and retained for traceability |

## Completed sprint baseline

### SPR-40 - Windows/Linux test matrix and smoke tests

Backlog IDs: `BL-023`, `BL-024`, `BL-025`.

The completed work through `SPR-40` remains historical. The 2026-07-25
governance migration assigned canonical `SPR-N` identifiers without changing
task IDs, scope, or status.

## Sprint sequence and status

`Planned` tasks form the Version 1.0 scope. `Later` tasks are accepted post-Version-1.0 work and must not delay the initial stable release. Cross-cutting security, CI, release, governance, and documentation gates apply throughout the implementation sprints and are complete only when their canonical tasks are `Done`.

Every sprint has one standalone positive-integer identifier. The former
suffix-based planning entries were assigned independent IDs, and every
subsequent sprint was shifted forward without changing backlog assignments.
The complete legacy mapping is recorded in
[the 2026-07-25 sprint-ID migration](docs/sprint-id-migration-2026-07-25.md).

| Sprint | Status | Backlog IDs | Scope |
|---|---|---|---|
| SPR-41 | Done | BL-026–BL-035 | FlashGate architecture baseline and backlog consolidation |
| SPR-42 | Done | BL-264–BL-280 | Technical project rename to FlashGate MCP |
| SPR-43 | Done | BL-281–BL-294 | Pre-1.0 filesystem tool contract cleanup |
| SPR-44 | Done | BL-174, BL-295–BL-303 | Codex read-only activation preparation |
| SPR-45 | Done | BL-201 | MCP `CallToolResult` foundation and `structuredContent` |
| SPR-46 | Done | BL-200 | MCP runtime `outputSchema` integration and parity |
| SPR-47 | Done | BL-189–BL-199 | Resource, latency, payload, catalog, workflow, and baseline benchmarking |
| SPR-48 | Planned | BL-202–BL-216, BL-218–BL-220 | Remaining MCP contracts, payload/result architecture, catalog budgets, and native-adapter policy |
| SPR-49 | Planned | BL-084–BL-099, BL-164 | Operations/Job Manager, identity-bound handles, quotas, fairness, and cleanup |
| SPR-50 | Planned | BL-036–BL-049 | Efficient filesystem listing, reading, batch inspection, MIME/binary handling, and large-result handoff |
| SPR-51 | Planned | BL-050–BL-061, BL-063–BL-067 | Targeted edits, conditional writes, bounded filesystem plans, and filesystem integration benchmarks |
| SPR-52 | Planned | BL-068–BL-080, BL-082 | Filesystem and text search |
| SPR-53 | Planned | BL-100–BL-111, BL-159–BL-161, BL-171 | Named roots, read-only safe default, capabilities, dynamic tool profiles, and negative authorization tests |
| SPR-54 | Planned | BL-113, BL-129, BL-162, BL-165 | Process architecture, execution identity, and stateful security model |
| SPR-55 | Planned | BL-114–BL-118 | Process observation |
| SPR-56 | Planned | BL-119–BL-126, BL-130–BL-135, BL-252–BL-254 | Managed process execution, output cursors, resource control, race tests, and CI jobs |
| SPR-57 | Planned | BL-136–BL-149, BL-151–BL-152, BL-163, BL-167–BL-168, BL-170 | Typed allowlisted command execution, OS isolation, redaction, and security tests |
| SPR-58 | Planned | BL-062, BL-153–BL-157 | Scoped and redacted system information |
| SPR-59 | Planned | BL-221–BL-225, BL-233–BL-239, BL-166, BL-341 | Multi-mode architecture, IPC/configuration contracts, hybrid execution-identity backend design, audit lifecycle, host-process ownership/lifecycle, and Variant A security |
| SPR-60 | Planned | BL-226–BL-231 | Named Pipe/Unix socket transports, proxy/auto modes, Windows SCM service, Linux systemd service, and Variant A service-account execution |
| SPR-61 | Planned | BL-172–BL-173, BL-177–BL-179, BL-241–BL-251, BL-255–BL-263, BL-305–BL-312, BL-314–BL-340 | Version 1.0 validation, packaging, cross-project benchmarks, supply-chain evidence, governance, documentation, Dependabot maintenance, PR #15/#16/#21 review follow-up, reference-bound legacy Temp cleanup, task-neutral governance handoffs, reusable fixture/validation orchestration, and governance generator/profile migration |

Version 1.0 is reached only after `SPR-61` and the release gate in `BL-263`. The following accepted work is intentionally post-Version 1.0 and has no committed implementation sprint before that release:

| Post-1.0 workstream | Backlog IDs | Direction |
|---|---|---|
| Efficiency and user-isolated hosting | BL-217, BL-232, BL-240 | Conditional reads, user-scoped persistent hosts, and Variant B user-worker implementation |
| Optional accelerators and expanded control | BL-081, BL-083, BL-112, BL-127–BL-128, BL-150, BL-158 | Ripgrep/index, legacy Roots, external PID/input, interactive shell, and network information decision gates |
| Provider/community ecosystem | BL-169, BL-176, BL-180–BL-188, BL-313 | External provider security, licensing, governance extensions, provider contracts/runtime, and related documentation |

`SPR-44` replaces the former `SPR-41` Codex preparation plan and must use the FlashGate technical names established in `SPR-42` and the cleaned tool names created in `SPR-43`.

The next free sprint identifier is `SPR-62`. It is reserved as the next
available number only and is not an assigned sprint.

## Canonical task catalog

### Completed foundation and current implementation

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-001 | Done | Establish Go project foundation | Module, package layout, configuration, security, and filesystem abstraction |
| BL-002 | Done | Implement root-confined filesystem tools | Current list/read/info/write/create/delete/copy/move implementation |
| BL-003 | Done | Implement MCP server foundation | JSON-RPC, initialize, `tools/list`, `tools/call`, routing, and server loop |
| BL-004 | Done | Add package and tool documentation | README, package docs, human and machine-readable tool references |
| BL-005 | Done | Add CI pipeline | Formatting, vet, separate Windows/Linux coverage-gated tests, lint, build validation, and per-platform coverage artifacts |
| BL-006 | Done | Add release build workflow | Windows and Linux artifacts under current technical names |
| BL-007 | Done | Add version and help CLI modes | `--version`, `--help`, and argument validation |
| BL-008 | Done | Add Windows JSON-RPC smoke script | Real STDIO path on Windows |
| BL-009 | Done | Run Windows JSON-RPC smoke in CI | Windows CI integration |
| BL-010 | Done | Document JSON-RPC smoke testing | README usage and validation description |
| BL-011 | Done | Add Linux/macOS JSON-RPC smoke script | Bash STDIO validation |
| BL-012 | Done | Update GitHub Actions major versions | Node-24-compatible action versions |
| BL-013 | Done | Update artifact upload action | Current artifact workflow version |
| BL-014 | Done | Add optional read-only mode | `MCP_READ_ONLY=true` restricted registration |
| BL-015 | Done | Enforce filesystem write capability gating | Read-only mode exposes only current read tools |
| BL-016 | Done | Harden effective-root and traversal validation | Real-path validation for existing paths and create parents |
| BL-017 | Done | Enforce hidden, UNC, symlink, junction, and reparse policy | Deny-by-default cross-platform path policy |
| BL-018 | Done | Harden JSON-RPC validation and error behavior | Envelopes, IDs, notifications, batches, params, unknown tools, and panic boundary |
| BL-019 | Done | Enforce protocol message and tool-argument limits | Bounded JSON-RPC and `tools/call` input |
| BL-020 | Done | Enforce filesystem operation and response limits | Bounded reads, writes, listing, copy, recursive delete, and responses |
| BL-021 | Done | Add centralized redaction and safe stderr diagnostics | Secret/host-path redaction, debug gating, and no stdout diagnostics |
| BL-022 | Done | Add safe defaults for non-developer users | Deny-by-default limits and conservative behavior |
| BL-023 | Done | Run Linux smoke test in Ubuntu CI | `SPR-40` real STDIO validation |
| BL-024 | Done | Run JSON-RPC smoke matrix on Windows and Linux | Default, read-only, and negative variants |
| BL-025 | Done | Isolate smoke JSONL artifacts per run | Unique files and deterministic cleanup |

### Project identity and architecture baseline

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-026 | Done | Adopt FlashGate MCP public identity | Name, tagline, meaning, scope, non-goals, and technical-name transition |
| BL-027 | Done | Document current and accepted target architecture | Diagram and explicit current/planned/deferred separation |
| BL-028 | Done | Define domain-separated local system core | Filesystem, search, process, execution, system, jobs, policy, limits, diagnostics, adapters, MCP |
| BL-029 | Done | Define core reuse and deployment baseline | Direct Go reuse, one repository/binary, evidence gates for IPC or split |
| BL-030 | Done | Define vendor-neutral open-source core | No mandatory Voxtronic assumptions, secrets, paths, or proprietary dependencies |
| BL-031 | Done | Define FlashGate module/provider direction and decision gate | Shared controls, no identifier or runtime model yet; distinct from MCP protocol extensions |
| BL-032 | Done | Define Operations/Job Manager architecture | Handles, states, deadlines, cancellation, resources, cleanup, goroutine/process gates |
| BL-033 | Done | Define capability profiles and named-root direction | Server-side enforcement, profile examples, root policy model |
| BL-034 | Done | Define managed process and execution architecture | Handles, PID rules, allowlists, no-shell default, single engine |
| BL-035 | Done | Define efficiency and pre-1.0 contract policy | Metrics, local work, planned cleanup, no artificial compatibility |

### Filesystem epic

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-036 | Planned | Add filesystem tests through MCP `tools/call` | Read, write, list, info, missing path, and security cases |
| BL-037 | Planned | Add paginated `list_directory` | Bounded pages rather than fail-or-return-all behavior |
| BL-038 | Planned | Define stable cursor semantics | Opaque cursor, invalidation, ordering, and policy changes |
| BL-039 | Planned | Add listing sort and filters | Portable name/type/metadata behavior |
| BL-040 | Planned | Add listing field selection | Return only requested portable fields |
| BL-041 | Planned | Add line-range reads | Bounded text line windows |
| BL-042 | Planned | Add byte-range and head/tail reads | Bounded binary-safe offsets and edge semantics |
| BL-043 | Planned | Expand portable path metadata | Modified time, permissions where portable, type, and size |
| BL-044 | Planned | Add large-file streaming strategy | Avoid whole-file memory loading and unbounded responses |
| BL-045 | Planned | Define text, media, and binary read behavior | MIME detection, explicit modes, inline thresholds, bounded encoding, resource-handoff rules, and client-compatible fallbacks |
| BL-046 | Planned | Add batch `read_files` | Per-item bounded results and partial-failure model |
| BL-047 | Planned | Add batch `get_paths_info` | Reduce repeated stat/existence round trips |
| BL-048 | Planned | Add batch hashing and content fingerprints | Bounded algorithms, byte accounting, reusable change identifiers, and job handoff |
| BL-049 | Planned | Add bounded directory tree | Depth, entry, byte, field, and pagination controls |
| BL-050 | Planned | Add exact targeted file changes | Explicit ranges or match-based edits without model retransmission |
| BL-051 | Planned | Add expected-match-count checks | Reject ambiguous or stale targeted edits |
| BL-052 | Planned | Add atomic writes | Same-filesystem replace and deterministic cleanup semantics |
| BL-053 | Planned | Add conditional write preconditions | Hash, modified-time, and path-type checks |
| BL-054 | Planned | Add dry-run support | Structured preview for destructive or multi-step changes |
| BL-055 | Planned | Add append-file support | Explicit bounded append distinct from overwrite |
| BL-056 | Planned | Add bounded filesystem plans | Limited known operations; no free-form workflow language |
| BL-057 | Planned | Enforce plan operation, entry, and byte limits | Preflight and runtime accounting |
| BL-058 | Planned | Define cross-volume move behavior | Copy/verify/delete, cancellation, and partial-state rules |
| BL-059 | Planned | Define conflict strategy | Fail, skip, replace, and explicit per-operation rules |
| BL-060 | Planned | Support directory copy and move | Bounded traversal, jobs, conflicts, and cleanup |
| BL-061 | Planned | Add directory-size operation | Streaming scan, limits, progress, and jobs |
| BL-062 | Planned | Add scoped disk-usage operation | Root-scoped capacity and privacy-safe results |
| BL-063 | Planned | Extend `write_file` safe modes | Create-only, replace-only, and explicit atomic/conditional behavior |
| BL-064 | Planned | Integrate long filesystem work with jobs | Preserve filesystem domain ownership |
| BL-065 | Planned | Threat-model bounded filesystem plans | Path races, rollback limits, conflicts, and partial completion |
| BL-066 | Planned | Add Windows/Linux filesystem MCP integration tests | Exercise real `tools/call` contracts and path behavior |
| BL-067 | Planned | Create representative filesystem benchmark corpus | Small/large files, deep/wide trees, binary data, and cross-volume cases |

### Search epic

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-068 | Planned | Define search model and threat model | Scope, recursion, data exposure, resource budgets, and errors |
| BL-069 | Planned | Add path search | Relative root-scoped paths |
| BL-070 | Planned | Add filename search | Literal and pattern matching |
| BL-071 | Planned | Add metadata filters | Type, size, and time with portable semantics |
| BL-072 | Planned | Add literal text search | Bounded content scanning |
| BL-073 | Planned | Add regular-expression search | Complexity and scan limits |
| BL-074 | Planned | Add include/exclude patterns | Deterministic precedence and relative matching |
| BL-075 | Planned | Enforce search depth, file, and scanned-byte limits | Server-side hard caps and counters |
| BL-076 | Planned | Enforce total and per-file match limits | Bounded results and diagnostics |
| BL-077 | Planned | Add bounded context lines | Per-match and aggregate response limits |
| BL-078 | Planned | Define binary detection and encoding behavior | Skip/error/explicit modes and reporting |
| BL-079 | Planned | Add search pagination | Opaque cursors and stable ordering |
| BL-080 | Planned | Add ignore-file support | Explicit policy and optional gitignore-compatible behavior |
| BL-081 | Later | Evaluate optional ripgrep adapter | Allowlisted local adapter with version/security checks |
| BL-082 | Planned | Provide pure-Go search fallback | Portable baseline without external dependency |
| BL-083 | Later | Decide on local search index after benchmarks | No index without measured need and privacy/lifecycle design |

### Operations and Job Manager epic

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-084 | Planned | Implement Operation Registry | Thread-safe ownership and lifecycle |
| BL-085 | Planned | Generate opaque identity-bound operation handles | `op_<opaque-id>` without guessable internals; bind owner principal, root, profile, execution backend, and service generation |
| BL-086 | Planned | Implement operation status model | queued/running/completed/failed/cancelled/timed_out |
| BL-087 | Planned | Add context cancellation | Cooperative cancellation contract |
| BL-088 | Planned | Add server deadlines and watchdog | Server-controlled timeout enforcement |
| BL-089 | Planned | Define progress and byte counters | Read/written/scanned bytes and bounded domain progress |
| BL-090 | Planned | Add bounded identity-bound result storage and TTL | Expiry, retrieval, resource handles, owner checks, and cleanup behavior |
| BL-091 | Planned | Add temporary-resource cleanup | Success, failure, cancellation, timeout, and incomplete markers |
| BL-092 | Planned | Limit global, per-domain, and per-principal parallel jobs | Configurable safe defaults that prevent one caller from exhausting shared service capacity |
| BL-093 | Planned | Limit queues and provide fair scheduling | Global/per-principal queue caps, deterministic overload responses, and starvation resistance |
| BL-094 | Planned | Define controlled server shutdown | Cancellation, grace period, worker termination, final state |
| BL-095 | Planned | Prevent and detect job leaks | TTL sweep, ownership checks, and metrics |
| BL-096 | Planned | Preserve domain ownership | Jobs execute work without becoming its business domain |
| BL-097 | Planned | Implement goroutine/subprocess decision rules | External, isolation, cancellation, identity, and platform gates |
| BL-098 | Planned | Add cross-platform Operations/Job integration tests | Go package and integration tests for deadline, cancellation, cleanup, shutdown, and temporary-resource behavior on Windows and Linux; this task does not define CI workflow jobs |
| BL-099 | Planned | Add job security tests and race detector | Handles, limits, lifecycle races, and leak checks |

### Named roots, capabilities, and profiles epic

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-100 | Planned | Implement functional capability model | Functional rights kept separate from profiles and risk classifications |
| BL-101 | Planned | Support multiple named roots | Preserve compatible single-root migration path |
| BL-102 | Planned | Use root ID and relative path in target contracts | Avoid model-visible absolute host paths |
| BL-103 | Planned | Implement profile and risk-policy configuration with read-only default | If roots exist but no profile is selected, expose only the safe read profile; write/process/command profiles require explicit activation |
| BL-104 | Planned | Add per-root read/write policy | Independent permissions per root |
| BL-105 | Planned | Add per-root size and result limits | File, scan, response, and temporary data policies |
| BL-106 | Planned | Add per-root allowed file types | Explicit portable matching |
| BL-107 | Planned | Add per-root symlink/reparse rules | Preserve root confinement and platform semantics |
| BL-108 | Planned | Add per-root capability mapping | Tool and operation authorization |
| BL-109 | Planned | Add process working-directory permission per root | Execution policy integration |
| BL-110 | Planned | Implement dynamic tool registration | Effective profile/capability controls `tools/list` |
| BL-111 | Planned | Add negative capability and `tools/list` catalog tests | Verify profile-driven tool visibility, dynamic registration, deterministic catalog output, and denial when a tool is absent from the effective catalog; generic server-side authorization bypass tests belong to `BL-160` |
| BL-112 | Later | Evaluate deprecated MCP Roots only for legacy client compatibility | No architectural dependency; server configuration and explicit root IDs remain authoritative; implement only for a supported legacy client; document deprecation and protocol-version scope |

### Process epic

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-113 | Planned | Create process threat model | Observation, identity, control, disclosure, races, and cleanup |
| BL-114 | Planned | Add paginated process list | Bounded stable results |
| BL-115 | Planned | Add process field selection | Minimize sensitive output |
| BL-116 | Planned | Add process details | Explicit fields and access errors |
| BL-117 | Planned | Add process tree | Bounded depth and partial platform data |
| BL-118 | Planned | Enforce `process.observe` | Registration and execution checks |
| BL-119 | Planned | Implement Managed Process Registry | Thread-safe server-started process ownership |
| BL-120 | Planned | Generate opaque process handles and prevent PID reuse errors | Handles are primary identity; PID is diagnostic only |
| BL-121 | Planned | Define managed process status | Starting/running/exited/failed/stopped/timed-out states |
| BL-122 | Planned | Add `start_process` | Managed engine only, with policy checks |
| BL-123 | Planned | Add `wait_process` | Context, timeout, and final result semantics |
| BL-124 | Planned | Add `read_process_output` | Cursor/bounded incremental output |
| BL-125 | Planned | Add separate stdout/stderr ring buffers | Size limits, truncation markers, and cleanup |
| BL-126 | Planned | Add `stop_process` | Managed handles by default |
| BL-127 | Later | Evaluate external PID control | Requires `process.control.external` or equivalent plus high-risk policy; absent from standard profiles |
| BL-128 | Later | Evaluate `write_process_input` | Separate policy and lifecycle gate |
| BL-129 | Planned | Define process cleanup, restart, and orphan behavior | Shutdown, server crash/restart, TTL, and diagnostics |
| BL-130 | Planned | Limit managed process count and concurrency | Global and profile budgets |
| BL-131 | Planned | Enforce process runtime limits | Defaults, maxima, cancellation, and termination |
| BL-132 | Planned | Define CPU and RAM limit strategy | Windows/Linux capabilities and fallbacks |
| BL-133 | Planned | Redact process command lines and environments | Minimize output and audit data |
| BL-134 | Planned | Implement Windows and Linux process adapters | Equivalent policy outcomes with platform-specific internals |
| BL-135 | Planned | Add process race, lifecycle, and restart tests | Registry races, PID reuse assumptions, cleanup, and limits |

### Command execution epic

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-136 | Planned | Create command-execution threat model | Executable substitution, injection, environment, roots, output, isolation |
| BL-137 | Planned | Add typed command definitions with executable IDs | Resolve IDs server-side to approved absolute paths plus fixed subcommands, allowed flags/value rules, path-argument binding, timeout/output/network policy, and optional binary identity pinning |
| BL-138 | Planned | Enforce no-shell default and typed argument objects | No free shell string; server generates argv only from validated structured fields and rejects response-file, hook, plugin, and configuration injection paths |
| BL-139 | Planned | Restrict working directories to allowed roots | Named-root process permission integration |
| BL-140 | Planned | Define timeout defaults and maxima | Server-enforced deadlines |
| BL-141 | Planned | Limit stdout and stderr separately | Bounded buffers/results and truncation markers |
| BL-142 | Planned | Define stable command result schema | Exit, output references, timeout, status, and bounded diagnostics |
| BL-143 | Planned | Implement `run_command` over Managed Process Engine | Synchronous wrapper only |
| BL-144 | Planned | Add environment allowlist and propagation rules | Minimal explicit environment; no unreviewed inherited hooks, loaders, plugin paths, credentials, or interpreter controls |
| BL-145 | Planned | Add execution environment and output redaction | Results, diagnostics, and audit events |
| BL-146 | Planned | Implement Windows execution isolation | Least privilege and documented platform limits |
| BL-147 | Planned | Implement Linux execution isolation | Least privilege and documented platform limits |
| BL-148 | Planned | Limit parallel command processes | Profile/global resource budgets |
| BL-149 | Planned | Define least-privilege execution identity | Avoid inherited privileges where practical |
| BL-150 | Later | Keep interactive shell disabled | Separate interactive/high-risk policy decision and threat model |
| BL-151 | Planned | Prevent a second execution engine | Registry and policy invariants/tests |
| BL-152 | Planned | Add Windows/Linux execution security tests | Allowlist, args, roots, env, output, timeout, and isolation |

### System information epic

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-153 | Planned | Add `system_info` | Controlled OS, architecture, version, and allowed host facts |
| BL-154 | Planned | Expose scoped disk usage | Reuse root-scoped operation `BL-062` |
| BL-155 | Planned | Add filtered environment information | Allowlist and secret exclusion |
| BL-156 | Planned | Add field selection and redaction | Minimize results and host identifiers |
| BL-157 | Planned | Enforce `system.read` capability | Registration and server-side execution checks |
| BL-158 | Later | Evaluate restricted network information | Separate privacy-sensitive decision gate |

### Security epic

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-159 | Planned | Enforce capabilities server-side | Registration is not the authorization boundary |
| BL-160 | Planned | Add server-side authorization bypass tests | Verify capability enforcement after tool resolution, including crafted direct calls, stale catalog assumptions, root/domain policy bypass attempts, and every high-risk operation; catalog visibility tests belong to `BL-111` |
| BL-161 | Planned | Add per-root policy security tests | Read/write, types, limits, links/reparse, working directory |
| BL-162 | Planned | Define process policy model | Observe/manage/external-control separation, managed identity, risk classifications, and policy inputs; enforcement remains in concrete implementation tasks |
| BL-163 | Planned | Define and enforce execution policies | Executables, args, roots, environment, limits, isolation |
| BL-164 | Planned | Add Operations/Job limit security tests | Queue, concurrency, time, temporary data, cleanup, handles |
| BL-165 | Planned | Maintain stateful domain threat models | Filesystem, search, processes, execution, FlashGate modules/providers, and MCP protocol extensions |
| BL-166 | Planned | Define structured audit lifecycle and trace correlation | Bounded redacted decisions/outcomes, immutable event IDs, end-to-end correlation, rotation, retention, disk-full/backpressure behavior, and log-injection protection |
| BL-167 | Planned | Extend secret redaction across new domains | Process, execution, system, jobs, audit, and module/provider outputs |
| BL-168 | Planned | Validate least-privilege execution | Server and child process permissions |
| BL-169 | Later | Enforce FlashGate module/provider security boundaries | Post-1.0 provider work may not bypass policies, functional capabilities, roots, limits, audit, execution identity, or adapters |
| BL-170 | Planned | Document sandbox boundaries and residual risk | OS, process, filesystem, link/reparse, and configuration limits |
| BL-171 | Planned | Verify MCP annotations are never authorization | Documentation and negative tests |
| BL-172 | Planned | Review and enforce workflow pinning strategy | Version 1.0 supply-chain hardening, SHA-pin tradeoffs, update process, and automated validation |
| BL-173 | Planned | Maintain public security policy | Reporting, supported versions, disclosure, and release gate |
| BL-174 | Done | Fail closed when no root is explicitly configured | Root is required; missing/empty/whitespace fail closed; production roots are absolute; `.` requires explicit `MCP_ALLOW_CWD_ROOT=true`; other relative roots are denied; root must exist and resolve to a permitted directory; safe categorized stderr and exit codes precede Registry/STDIO; Windows/Linux startup smokes cover the contract |

### Open source and FlashGate modules/providers epic

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-175 | Done | Confirm current open-source license | Repository and README currently declare GNU GPL v3.0; no license change in `SPR-41` |
| BL-176 | Later | Review license and distribution compatibility before external module contract | Post-1.0 factual compatibility gate before the first external provider; no legal conclusions in backlog |
| BL-177 | Planned | Define governance model | Decision authority, releases, and stewardship |
| BL-178 | Planned | Define maintainer rules | Roles, review, security, and succession |
| BL-179 | Planned | Expand contribution guidelines | Development, testing, documentation, and DCO/CLA decision |
| BL-180 | Later | Add Code of Conduct before public community release | Community expectations and enforcement |
| BL-181 | Later | Hold FlashGate module/provider contract decision gate | Post-1.0 and required before first external provider |
| BL-182 | Later | Decide FlashGate module/provider identifier rules | Post-1.0; no concrete syntax before the contract decision |
| BL-183 | Later | Define FlashGate module/provider metadata | Post-1.0 name, version, vendor, tools, config, platforms, dependencies |
| BL-184 | Later | Define module/provider capability declarations | Post-1.0 required and optional functional capabilities with least privilege |
| BL-185 | Later | Define module/provider security classification | Post-1.0 risk categories and review requirements |
| BL-186 | Later | Distinguish public, community, vendor, and internal providers | Post-1.0 distribution and support labels do not change security |
| BL-187 | Later | Define official versus community provider policy | Post-1.0 trust language, signing/update expectations, and support |
| BL-188 | Later | Decide FlashGate provider runtime model | Choose among statically linked packages registered at build time, registered in-process providers, or isolated out-of-process providers over local IPC; a Go module is source/versioning only, not a runtime model |

### Efficiency and MCP contract foundation

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-189 | Done | Add startup benchmark | Real binary over STDIO; one `first_process_start` after build plus 30 new subsequent processes by default, 10 in quick mode; no OS cold-cache claim |
| BL-190 | Done | Measure idle RSS | Clean 30-repetition Windows idle Working Set and native Linux `VmRSS` baselines from implementation commit `cfd211fa81cc48ee1dc463966718442f2ab5223c` are versioned by baseline commit `0f022648f6e30a37db53f457a920693904962f1e`; resources are supported in `benchmarks/baseline.windows-amd64.json` and `benchmarks/baseline.linux-amd64.json` |
| BL-191 | Done | Measure peak memory, CPU time, and allocations | Win32 peak working set/user/kernel time, Linux `VmHWM`/user/system time, existing and direct-handler Go allocation benchmarks, representative single and multi-operation workflows |
| BL-192 | Done | Measure p50 and p95 latency | Nearest-rank p50/p95 for startup and all ten real-process reference workflows |
| BL-193 | Done | Record scanned, read, and written bytes | Runner-side counters have explicit semantics and remain outside public MCP results |
| BL-194 | Done | Measure serialized result sizes | Existing six-fixture historical/text/text-plus-structured benchmark retained; deterministic result and complete response bytes are pinned |
| BL-195 | Done | Measure `tools/list` size | Read-only/default tool count, schema count, request/result/response bytes, and approximate tokens are deterministic gates |
| BL-196 | Done | Measure calls per reference workflow | Machine-readable workflows record actual `tools/call` counts, including ten-call independent path/read cases |
| BL-197 | Done | Add optional schema/response token approximation | `approx_tokens_bytes4 = ceil(UTF-8 bytes / 4)` is clearly non-model-specific and unsuitable for billing |
| BL-198 | Done | Establish benchmark baselines | Versioned v1 Windows and native Linux baselines from implementation commit `cfd211fa81cc48ee1dc463966718442f2ab5223c` are recorded by baseline commit `0f022648f6e30a37db53f457a920693904962f1e` in `benchmarks/baseline.windows-amd64.json` and `benchmarks/baseline.linux-amd64.json` with complete provenance, zero hard/soft budget findings, and passed artifact/platform checks |
| BL-199 | Done | Define CI regression budgets | Machine-readable hard deterministic and soft noise-sensitive budgets with local evaluation; full CI execution/comparison remains BL-249/BL-250 |
| BL-200 | Done | Add MCP `outputSchema` | All eight runtime filesystem tools expose success-only schemas matching catalog `resultSchema` and successful `structuredContent`; no error migration or complete general JSON Schema validation |
| BL-201 | Done | Add MCP `CallToolResult` foundation and `structuredContent` | All eight successful filesystem tools use one central text-plus-structured wrapper with deterministic parity, strict decoder/wire tests, corrected smokes, and no runtime `outputSchema` |
| BL-202 | Planned | Review MCP tool annotations | Accurate metadata, never authorization |
| BL-203 | Planned | Define normalized machine-readable errors | Stable categories without raw OS leakage |
| BL-204 | Planned | Evaluate official MCP conformance testing and add schema snapshots | `SPR-46` adds full runtime/catalog output-schema parity and `tools/list` wire coverage; official tooling plus full input/output snapshots remain planned |
| BL-205 | Planned | Add response-size regression tests | `SPR-46` measures `tools/list` payload impact without setting a persistent budget; success/error regression gates remain planned |
| BL-206 | Planned | Document local deterministic work principle | Prefer local copy/edit/hash/search over model retransmission |
| BL-207 | Planned | Define Version 1.0 MCP protocol matrix | Keep core version-independent; retain `2025-11-25` until newer revisions are final and implemented; test every advertised revision and breaking upgrade path |
| BL-208 | Planned | Define MCP extension-negotiation and stateless-adapter strategy | Official identifiers, capability negotiation, 2026 stateless-core adaptation, cache/TTL semantics, downgrade/mismatch tests, and no authorization implication |
| BL-209 | Planned | Decide final MCP Tasks Extension compatibility | Do not mix the 2025 experimental lifecycle with the final extension; evaluate supported clients before asynchronous MCP exposure |
| BL-210 | Planned | Map internal operation lifecycle to MCP Tasks | Define tested state, result, error, cancellation, TTL, and redaction mapping; internal states may be more detailed |
| BL-211 | Planned | Decide fallback when MCP Tasks is unavailable | Bounded synchronous result or explicit capability error; no ad hoc custom job-tool contract |
| BL-212 | Planned | Validate all input/output schemas as JSON Schema 2020-12 | Complete standard-conformant validation, dialect declarations, deterministic property ordering, snapshots, and protocol-version compatibility |
| BL-213 | Planned | Define payload-class result contracts and single-transmission rules | Small metadata may retain text/structured parity; heavy text, binary, search, and process payloads appear once with separate compact metadata and bounded compatibility fallback |
| BL-214 | Planned | Add wire-amplification and useful-byte efficiency metrics | Record response bytes versus useful payload, approximate token cost per useful byte, serialization copies, and hard regression budgets |
| BL-215 | Planned | Define profile-specific tool-catalog and initialization budgets | Set Version 1.0 limits for tool count, schema bytes/tokens, descriptions, server instructions, and optional profile composition |
| BL-216 | Planned | Add compact profile-specific server instructions | Guide clients to batch, paginate, request fields/ranges, use dry-run, avoid redundant stat calls, and resume cursor output within a bounded instruction budget |
| BL-217 | Later | Add conditional read and not-modified contracts | Post-1.0 content fingerprints/snapshot IDs for files, lists, searches, system facts, and process output to avoid retransmitting unchanged data |
| BL-218 | Planned | Add opaque large-result and resource-handoff abstraction | Principal-bound `flashgate://` handles, MIME/size/hash metadata, TTL, streaming or paging, negotiated resource links, and bounded inline fallback without host-path leakage |
| BL-219 | Planned | Define deterministic catalog fingerprints and cache semantics | Stable tool ordering, profile/config/protocol fingerprint, invalidation rules, and compatible list-result TTL behavior |
| BL-220 | Planned | Define native OS-adapter selection and no-interpreter gate | Prefer Go standard library, platform Go adapters, and direct OS APIs; external OS programs require no-shell allowlisting plus benchmark/security evidence; interpreter-based adapters are excluded from Version 1.0 |

### Native multi-mode runtime and local service deployment epic

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-221 | Planned | Define native multi-mode runtime architecture and threat model | One Windows PE/Linux ELF binary; explicit current/planned boundaries; no interpreter, remote listener, or implicit privilege escalation |
| BL-222 | Planned | Preserve one self-contained binary across runtime modes | Shared core and executable for `stdio`, `proxy`, `auto`, system service, and user-scoped host; split only through a separate evidence-backed ADR |
| BL-223 | Planned | Define CLI mode and lifecycle contract | Preserve no-argument STDIO compatibility; specify `--mode stdio`, `--mode proxy`, `--mode auto`, `--mode service`, management commands, exit codes, and shutdown behavior |
| BL-224 | Planned | Separate MCP/core runtime from transport and host lifecycle | Transport-neutral server/core wiring with no business logic in STDIO, IPC, SCM, or systemd adapters |
| BL-225 | Planned | Define versioned local IPC protocol and compatibility handshake | Framing, protocol version, feature negotiation, correlation, cancellation, errors, limits, disconnects, and proxy/service version mismatch behavior |
| BL-226 | Planned | Implement Windows Named Pipe transport | Local-only pipe, restrictive ACLs, caller identity from the OS, bounded framing, cancellation, and no trust in proxy-supplied identity |
| BL-227 | Planned | Implement Linux Unix Domain Socket transport | Local-only socket, restrictive ownership/mode, peer UID/GID/PID credentials, bounded framing, cleanup, and stale-socket handling |
| BL-228 | Planned | Implement STDIO proxy mode | Present normal MCP STDIO to the client and forward safely to the local service without corrupting stdout or changing public tool contracts |
| BL-229 | Planned | Implement automatic service discovery and safe STDIO fallback | Prefer explicitly configured/system/user endpoints; no elevation or installation; fallback only when no managed endpoint is present, never after authorization, policy, or compatibility rejection |
| BL-230 | Planned | Implement Windows SCM service host, identity, and management | Real Windows Service Control Manager lifecycle, restricted service identity, install/uninstall/start/stop/status, graceful shutdown, and recovery policy. Register the user-facing SCM display name exactly as `FlashGate MCP`; the binding spelling uses uppercase `F` and `G`, one space between `FlashGate` and `MCP`, and all-uppercase `MCP`. Keep the executable filename `flashgate-mcp.exe` and explicitly distinguish the SCM service name, SCM display name, SCM service description, executable image name, PE `ProductName`, PE `FileDescription`, and MCP `serverInfo.name` (`flashgate`). Do not unintentionally change the Go module, repository name, MCP implementation name, configuration names, endpoint or protocol identifiers, installation paths, or release artifact names. Install the SCM service only from a controlled Windows artifact carrying the existing canonical product metadata. Validate the visible and technical identities in Services, PowerShell/SCM queries, and the relevant Task Manager views, and document which Windows surface displays which identity. Reuse the completed BL-246 metadata gates; do not rename technical identifiers unless separately required and approved. |
| BL-231 | Planned | Implement Linux systemd system service | Unit, dedicated restricted account, Unix socket/runtime directories, journald, hardening directives, install/uninstall/start/stop/status, and graceful shutdown |
| BL-232 | Later | Add user-scoped background modes | Post-1.0 Linux `systemd --user` service and Windows per-user host; direct STDIO remains the non-admin Version 1.0 path |
| BL-233 | Planned | Define configuration precedence, endpoint discovery, and log destinations | CLI/environment/config precedence, system/user paths, endpoint names, timeouts, fallback policy, stdout purity, journald/Event Log/user logs, and secret-safe diagnostics |
| BL-234 | Planned | Enforce service-side authorization, identity dispatch, and policy | OS-derived caller identity, user/group mapping, roots, profiles, capabilities, Variant A backend selection, per-principal limits, audit, least privilege, and no local privilege-escalation path |
| BL-235 | Planned | Adopt hybrid per-root service execution identity | Version 1.0 uses service-account roots; user-worker roots are architected now and implemented later; in-process impersonation is permanently excluded |
| BL-236 | Planned | Implement backend-neutral execution-identity interfaces | Separate authenticated caller, policy decision, effective identity backend, operation dispatch, and OS adapter so Variant B can be added without changing domain/MCP contracts |
| BL-237 | Planned | Implement Variant A service-account root backend | Dedicated least-privilege account, explicitly ACL-granted roots, no LocalSystem/root convenience default, deterministic denial, and dual caller/effective-identity audit fields |
| BL-238 | Planned | Define Variant B user-worker contract and threat model | Specify worker launch/token or UID model, same-binary internal worker mode, broker IPC, environment/groups, lifecycle, quotas, crash recovery, and Windows/Linux differences without implementing it in Version 1.0 |
| BL-239 | Planned | Bind state, caches, and result resources to execution context | Bind principal, groups, profile, root, backend, service instance/generation, protocol context, and expiry; prohibit cross-principal cache/handle reuse |
| BL-240 | Later | Implement Variant B per-user worker backend | Post-1.0 broker-managed worker processes under the real user identity with OS resource isolation, native audit attribution, and no shared-process impersonation |
| BL-241 | Planned | Add multi-client, lifecycle, compatibility, and denial tests | Windows/Linux unit and integration coverage for concurrent clients, disconnects, restart, stale endpoints, shutdown, version mismatch, unauthorized access, and fail-closed auto behavior |
| BL-242 | Planned | Add Windows/Linux CI and release validation for all modes | Build native artifacts, test STDIO/proxy/service adapters where CI permits, verify no interpreter dependency, validate service assets, and retain existing gates |
| BL-243 | Planned | Document installation, removal, operation, and non-admin deployment | System service, user service/host, portable STDIO, proxy/auto configuration, troubleshooting, rollback, permissions, and explicit current-versus-planned status |
| BL-244 | Planned | Benchmark direct, proxy, and service modes and define release gate | Startup, steady-state latency, memory, CPU, payload overhead, concurrency, and evidence-based acceptance thresholds before recommending managed mode broadly |

### CI, release, and quality epic

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-245 | Planned | Add release notes or tag-based release workflow | Release maturity after technical rename |
| BL-246 | Done | Embed native Windows file and product metadata | Generate and embed deterministic Windows `VERSIONINFO` and the FlashGate application icon in x64 and ARM64 binaries; expose product name, description, company, copyright, original filename, internal name, numeric file version, and full SemVer product version from the canonical build-information source; validate PE architecture, Explorer properties, exact release-archive contents, checksums, reproducibility, and host/credential leak absence. |
| BL-247 | Done | Define and publish native Linux binary and package metadata | Expose the shared canonical identity through compact/verbose CLI output, Go build/VCS information, ELF headers and Go build IDs for native x64 and cross-built ARM64; publish deterministic TAR.GZ artifacts with exact contents and checksums; keep extended attributes non-authoritative and defer `.deb`, `.rpm`, and systemd metadata until those distribution assets are approved. |
| BL-248 | Done | Add artifact verification | Reuse the canonical `internal/version`, build, manifest, icon, releaseaudit, and platform-verifier paths; validate real Windows/Linux x64 and cross-built ARM64 binaries and ZIP/TAR.GZ packages, including version/help, product/platform/architecture metadata, exact inventories, checksums, two-build reproducibility, leak scans, structured results, and focused negative cases. PR #25 was merged on 2026-07-26 at `a30d3ab4958af6c1df5015300817aac1b692fde9`; CI Run 82 and Metadata Regression Run 11 succeeded. The final Windows contract suite passed `201/201`, the native Linux contract suite passed `206/206`, all six original findings including `BL248-REV-004` are closed, and no BL-248 finding remains open. Durable evidence is preserved and the local preparation workspace is removed. |
| BL-249 | Planned | Run benchmark suite in CI | Stable selection and artifacted results |
| BL-250 | Planned | Compare benchmark baselines in CI | Budgets from `BL-199` |
| BL-251 | Done | Validate PowerShell and Bash scripts | Deterministic Windows validation is complete for the dynamic 54-file inventory (32 PowerShell, 22 Bash): PowerShell 7.6.4 parser, exact Git Bash syntax, UTF-8/line-ending/shebang rules, no-mutation checks, 21/21 persistent PowerShell harness cases, and Windows/native-Ubuntu CI wiring. The bounded-process result carries the PID captured directly after process start, so the timeout regression proves termination of the concrete process without depending on child-authored PID-file timing; process-tree termination and stream draining remain bounded and fail closed. The native Bash harness reports cleanup truthfully and includes a controlled cleanup-negative case. Native `standard` run `bl251-review-findings-closure-20260802-100200` passed all twelve ordered commands with 0 warnings and 0 failures. Independent review `BL-251-focused-independent-delta-review-20260802-102407.md` passed with 0 warnings/0 failures and closed `BL251-REV-001` through `BL251-REV-004` as `CLOSED_BY_INDEPENDENT_DELTA_REVIEW`. Independent PID review `BL-251-pid-focused-independent-delta-review-20260802-121605.md` also passed with 0 warnings/0 failures and closed `BL251-PRECOMMIT-REV-001` as `CLOSED_BY_INDEPENDENT_DELTA_REVIEW`; the earlier BL-251 review findings remain closed. The exact commit, remote push, Draft PR, Hosted CI, and focused independent review of the documentation correction are complete. The final independent PR review found the documentation-only Minor finding `PR30-REV-001`; it is `CLOSED_BY_INDEPENDENT_DELTA_REVIEW`. Merge remains separately authorized. INF-122 and INF-129 remain closed, BL-324 remains planned, the focused wrapper validation passed `12/12`, and the reusable terminal governance evidence remains `225/225 PASS`. |
| BL-252 | Planned | Run race detector for stateful components | Execute Go race detection against jobs, process registry, output buffers, cancellation, and shutdown; provide the reusable race-test command and failure gate consumed by CI tasks such as `BL-254` |
| BL-253 | Planned | Add Windows/Linux process CI jobs | Dedicated CI matrix for process observation and managed lifecycle behavior on supported Windows and Linux runners; reuse implementation tests from the process packages rather than redefining them |
| BL-254 | Planned | Add Operations/Job CI jobs | Dedicated CI execution for the Operations/Job integration suite from `BL-098`, including cancellation, timeout, cleanup, leak checks, and the race gate from `BL-252`; this task owns workflow orchestration, not duplicate test implementation |
| BL-255 | Planned | Verify FlashGate release artifact names | After `SPR-42`, including archives and summaries |
| BL-256 | Planned | Enforce profile-specific catalog and initialization budgets | `tools/list`, tool count, schema bytes/tokens, server instructions, deterministic ordering, and fingerprint regression |
| BL-257 | Planned | Run schema snapshot checks in CI | Contract changes require explicit review |
| BL-258 | Planned | Run payload and response-efficiency tests in CI | Prevent unbounded contracts, duplicate heavy payloads, excessive wire amplification, and result-resource regressions |
| BL-259 | Planned | Search repository for legacy names after `SPR-42` | Allow only migration/history exceptions |
| BL-260 | Planned | Keep standard test/vet/lint/build gates | Preserve formatting, vet, tests, separate Windows/Linux coverage gates and artifacts, lint, and build validation; active thresholds remain authoritative in `.github/workflows/ci.yml` |
| BL-261 | Planned | Add reproducible cross-project efficiency benchmark | Compare pinned FlashGate, official Node.js filesystem, selected native Rust filesystem, and selected Go filesystem servers on identical host/corpus/workflows without claiming unmeasured superiority |
| BL-262 | Planned | Add native release supply-chain evidence | Checksums, Windows signing plan, Linux artifact/package signing plan, SBOM, build provenance, dependency inventory, reproducible-build comparison, and atomic rollback; no silent auto-update |
| BL-263 | Planned | Define and enforce Version 1.0 release boundary | Verify every Planned task or documented waiver, stable protocol/tool contracts, migration/deprecation policy, Variant A-only service identity, performance/security budgets, supported platforms, and post-1.0 deferrals |

BL-333/BL-334, BL-335, and BL-251 are complete. The twelve-command native
standard profile, the final native run, the runspace-free wrapper validation,
and the terminally persisted 225-case fixture replacement all pass. INF-122
and INF-129 are closed. BL-324 is `Done` with no remaining task work.

Binding remaining queue after BL-324 completion:
`schedule BL-340 independently in SPR-61 -> final documentation convergence -> Local Work Register dissolution audit -> separately authorized Local Work Register removal`.

### SPR-42 technical identity

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-264 | Done | Rename local folder to `flashgate-mcp` | Manually completed before `SPR-42` implementation |
| BL-265 | Done | Rename GitHub repository to `flashgate-mcp` | Manually completed before `SPR-42` implementation |
| BL-266 | Done | Update Git remote URL | Remote verified with fetch and redirect checks |
| BL-267 | Done | Update Go module and imports | Current module `github.com/thomasweidner/flashgate-mcp`; the former owner path is retained only in historical migration records |
| BL-268 | Done | Rename binary to `flashgate-mcp` | Windows/Linux build and usage updated |
| BL-269 | Done | Change MCP server implementation name (`serverInfo.name`) to `flashgate` | Initialize response and smoke tests updated |
| BL-270 | Done | Review package and command paths | `cmd/server` retained as a generic internal command path |
| BL-271 | Done | Update README, changelog, and documentation names | Migration/history context preserved |
| BL-272 | Done | Update PowerShell and Bash scripts | Paths, errors, examples, and smoke expectations updated |
| BL-273 | Done | Update CI and release artifact names | Workflows updated without unrelated modernization |
| BL-274 | Done | Update installation and configuration examples | New folder and binary names documented |
| BL-275 | Done | Update smoke tests | Implementation name (`serverInfo.name`), binary, paths, and current tool names validated |
| BL-276 | Done | Search all files for legacy names | Remaining occurrences classified as historical or migration guidance |
| BL-277 | Done | Write technical rename migration note | Old/new repository, module, binary, implementation name (`serverInfo.name`), local folder |
| BL-278 | Done | Verify GitHub redirect behavior | New path, remote, fetch, and legacy URL redirect verified |
| BL-279 | Done | Document manual repository rename action | See dated migration note: clean-main/origin/auth/target/remote preconditions; GitHub rename, remote/default/folder actions and separate technical branch; view/remote/fetch/main/reachability/redirect/history/path verification; failure/rollback without force-pushes or old-name reuse |
| BL-280 | Done | Keep rename sprint functionally neutral | No feature or tool-contract changes mixed in |

### SPR-43 pre-1.0 tool contract cleanup

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-281 | Done | Rename `list_files` to `list_directory` | Code, closed schema, runtime validation, tests, docs, catalog, examples, and smoke updated |
| BL-282 | Done | Rename `stat_path` to `get_path_info` | Single-stat existing/missing contract implemented without exposing host paths |
| BL-283 | Done | Rename `mkdir` to `create_directory` | Parent creation retained; `created` now reflects the actual leaf state |
| BL-284 | Done | Remove `exists_path` | MCP tool and redundant core method removed; no compatibility alias |
| BL-285 | Done | Remove `rename_path` | MCP tool and redundant core alias removed; `move_path` covers rename and movement |
| BL-286 | Done | Define `move_path` rename/move semantics | Same-path/SameFile, overwrite type combinations, same-volume/cross-volume, Windows case aliases, hardlinks, and self-subtree covered; no copy/delete fallback |
| BL-287 | Done | Define missing-path `get_path_info` result | Genuine missing returns `{ "path": ..., "exists": false }`; all policy denials remain errors |
| BL-288 | Done | Define normalized filesystem error codes | Safe sprint-local categories map expected failures to `-32602` and unexpected I/O to `-32603`; stable wire objects remain later work |
| BL-289 | Done | Review input/output schemas and required fields | Strict object/EOF/unknown-field validation, non-blank paths, optional list path, and `maxBytes >= 1` implemented |
| BL-290 | Done | Optimize tool descriptions | Titles exposed via shared definitions; descriptions are compact and `copy_path` is explicitly file-only |
| BL-291 | Done | Update JSON schema snapshots and unit tests | Runtime/catalog contract test covers names, titles, descriptions, required/property fields, and `additionalProperties` |
| BL-292 | Done | Update smoke tests and MCP `tools/call` tests | Registry/router/call plus default, read-only, negative, Existing/Missing, and Move-as-Rename smoke contracts updated |
| BL-293 | Done | Update tool docs, client examples, and catalog | README, current architecture/security/testing docs, tool docs, conventions, catalog, ADR amendments, and migration coordinated |
| BL-294 | Done | Document breaking changes in changelog | Breaking pre-1.0 cleanup documented with no alias or artificial deprecation compatibility |

### SPR-44 Codex read-only activation preparation

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-295 | Done | Add Codex read-only configuration example | Prepared, not applied: verified binary, implementation name (`serverInfo.name`), cleaned tools, explicit root/read-only environment, and unconfirmed fields clearly marked |
| BL-296 | Done | Add Claude Desktop configuration example | Prepared Windows-oriented example plus Linux perspective with renamed artifact; no client changed |
| BL-297 | Done | Add general MCP client examples | Minimal local STDIO contract uses absolute binary/root and explicit read-only/CWD policy |
| BL-298 | Done | Add read-only troubleshooting guide | Safe categories cover root, permissions/policy, JSON-RPC, binary, and profile/tool-list issues without raw host details |
| BL-299 | Done | Create activation checklist | Covers commit/binary/hash, root, read-only profile, exact tools/list, positive/negative smokes, backup, acceptance, and rollback |
| BL-300 | Done | Update read-only smoke test | Exact three read-only tools; all five write and five legacy names negative; Windows/Ubuntu startup smokes verify stdout/stderr, root failures, and cleanup |
| BL-301 | Done | Validate new documentation paths and links | Current FlashGate paths used; legacy installation paths remain only in immutable migration/history context |
| BL-302 | Done | Document non-developer read-only validation | Script-based binary/hash/root/tool-list/negative/rollback checklist requires no Go build at client start |
| BL-303 | Done | Keep activation external to preparation sprint | No real Codex configuration, MCP entry, or auth file changed; activation remains a separately confirmed post-merge step |

### Documentation and client compatibility epic

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-304 | Done | Link planning and history from README | Backlog, roadmap, changelog, architecture, security, code coverage, and ADRs |
| BL-305 | Planned | Keep CHANGELOG updated each sprint | Added/Changed/Fixed/Security, breaking changes, and CI or coverage-policy changes |
| BL-306 | Planned | Keep BACKLOG updated each sprint | Canonical IDs, sprint refs, migration rule, completed status |
| BL-307 | Planned | Maintain FlashGate project identity reference | Name, tagline, scope, transition, planned identifiers |
| BL-308 | Planned | Maintain architecture and ADRs | Current/target/planned/deferred separation |
| BL-309 | Planned | Document benchmark method and baselines | `SPR-45` documents the tool-result-contract subset and single-machine noise limits; broader benchmark documentation remains planned |
| BL-310 | Planned | Document capabilities, profiles, and named roots | Configuration and security model |
| BL-311 | Planned | Document Operations/Job Manager | Handles, states, limits, lifecycle, cleanup |
| BL-312 | Planned | Document process and execution security | Handles, PIDs, allowlists, isolation, redaction |
| BL-313 | Later | Document external module/provider ecosystem | Post-1.0 provider contract, runtime, security classification, distribution, support, and separation from negotiated MCP extensions |
| BL-314 | Planned | Maintain non-developer smoke-test documentation | PowerShell/Bash and expected results |
| BL-315 | Planned | Review README, CHANGELOG, and BACKLOG each sprint | Prevent stale identity, implementation, planning, CI, and coverage claims; include `docs/development/code-coverage.md` when thresholds or measurement rules change |

### PR #15 independent-review follow-up

These tasks originate in the independent review of PR #15. They are intentionally not implemented by the blocker-fix commit and are scheduled as separate work after PR #15 is merged.

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-316 | Done | Make the benchmark artifact validator authoritative and schema-strict | **Completed:** the validator strictly decodes each Windows/Linux artifact and canonical budget/workflow source, rejects unknown, duplicate, missing, mistyped, null, and trailing JSON content, enforces the typed schema invariants, independently recomputes hard and soft budgets, and compares the complete result exactly with embedded `budget_evaluation`. Canonical hard/soft key sets are complete, every required soft limit is positive, recomputed hard failures remain fatal, and matching soft warnings remain review-only through the full platform path. Runner result construction keeps budget messages exclusively in `budget_evaluation`; general process/stderr warnings remain separate and fatal. Cross-platform comparison follows independent artifact validation. End-to-end mutations cover runner-produced single-/dual-platform soft results, general warnings, identical dual-platform attacks, stale/fabricated evaluation, matching hard results, invalid soft definitions, and structural attacks. **Scope:** BL-317 through BL-323 remain unchanged; PR #16 Minor/Note follow-ups are BL-325 through BL-329. |
| BL-317 | Planned | Gate deterministic workflow semantics independently of output size | **Origin/severity:** independent review of PR #15, Major. **Components:** `benchmarks/workflows.json`, runner workflow checks, budgets and artifact tests. **Risk:** truncated or missing output can look more efficient and still satisfy byte/counter ceilings. **Acceptance:** validate every deterministic minimum/exact contract, including `expected_read_bytes` and `expected_entries`, reject absent or reduced useful output, and add negative artifacts proving output loss fails. **Timing:** separate work after merge of PR #15; not fixed by the blocker change. |
| BL-318 | Planned | Require an isolated explicit corpus parent for authoritative runs | **Origin/severity:** independent review of PR #15, Major. **Components:** authoritative benchmark controller, `scripts/benchmark*.ps1`, `scripts/benchmark*.sh`, benchmark workspace policy. **Risk:** an implicitly chosen temporary corpus can leave the measured filesystem, synchronization, and storage provenance uncontrolled. **Acceptance:** require and validate an explicit corpus parent below the fixed local Windows benchmark workspace and, for native Linux, native ext4 below `/home`; reject reparse, mounted Windows, synchronized, network, or unresolved parents fail-closed. **Timing:** separate work after merge of PR #15; not fixed by the blocker change. |
| BL-319 | Planned | Record verifiable benchmark build and host provenance | **Origin/severity:** independent review of PR #15, Major. **Components:** baseline schema, authoritative controller, build preparation, host gate and reports. **Risk:** committed measurements cannot be tied cryptographically to the measured binary, source/build inputs, controller, workspace, or accepted host-load interval. **Acceptance:** record and validate binary, source/build-input, controller and workspace identities/hashes plus preparation and measurement timestamps, authoritative preflight evidence, and final host-gate evidence; reject incomplete or mismatched provenance. **Timing:** separate work after merge of PR #15; not fixed by the blocker change. |
| BL-320 | Planned | Add native Linux race and cross-platform benchmark-policy CI gates | **Origin/severity:** independent review of PR #15, Major. **Components:** GitHub Actions, Windows/Linux policy scripts, window tests, native Linux `go test -race`. **Risk:** platform-specific symlink, reparse, policy-window, and race regressions may merge without executing the relevant native coverage. **Acceptance:** CI runs native Linux race coverage, Windows and Linux output/baseline policy tests, and both measurement-window suites with deterministic pass/fail artifacts; documented exceptions must fail the release gate rather than silently skip required coverage. **Timing:** separate work after merge of PR #15; not fixed by the blocker change. |
| BL-321 | Planned | Derive Linux clock ticks instead of assuming 100 Hz | **Origin/severity:** independent review of PR #15, Minor. **Components:** Linux process/resource collector in `internal/benchmark`. **Risk:** CPU-time values are wrong on systems whose `_SC_CLK_TCK` differs from 100. **Acceptance:** obtain the platform value through a supported native mechanism without a shell, handle lookup failure explicitly, and test conversion with non-100 values. **Timing:** separate work after merge of PR #15; not fixed by the blocker change. |
| BL-322 | Planned | Reconcile BL-190 and BL-198 final status documentation | **Origin/severity:** independent review of PR #15, Minor. **Components:** `BACKLOG.md`, `SPR-47` report, related benchmark history/current-state documentation. **Risk:** steering state and sprint evidence disagree about whether baseline collection and validation are complete. **Acceptance:** after the authoritative merge decision, establish one canonical current status for BL-190 and BL-198, update current steering documents consistently, and preserve historical documents or append a dated correction instead of rewriting history. **Timing:** separate work after merge of PR #15; not fixed by the blocker change. |
| BL-323 | Planned | Correct benchmark coverage claims for copy and search | **Origin/severity:** independent review of PR #15, Minor. **Components:** `docs/testing.md`, benchmark inventory and future filesystem/search benchmark work. **Risk:** testing documentation claims Copy and Search benchmarks that do not yet exist, overstating coverage. **Acceptance:** inventory implemented benchmark cases, make current documentation match that inventory, and describe copy/search only as planned until executable coverage and tests exist. **Timing:** separate work after merge of PR #15; not fixed by the blocker change. |

### GitHub dependency maintenance follow-up

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-324 | Done | Configure Dependabot security and version updates | **Completed:** PR #36 merged exact reviewed head `eb35b059793eb1327d0570c62174e6090e57a14e` as merge commit `396bfa7720bdd11c8baf3bf133096f911174b303`, changing exactly `.github/dependabot.yml` and `Governance/change-trigger-catalog.json`. Exact-commit/push-scope independent review passed with `OpenFindingCount=0`; pre-merge CI and Metadata Regression passed. Post-merge CI #116, Metadata Regression #45, and CodeQL actions/Go/Python analyses passed; initial GitHub processing for both `gomod` and `github-actions` completed successfully. Dependabot Alerts and Dependabot Security Updates are enabled, security updates are not paused, weekly version updates, routine non-security grouping, and concurrent-PR limits are active, and automatic merging remains disabled. No BL-324 task work remains. |

### PR #16 independent-review follow-up

These tasks originate in the independent security and release review of PR #16. They are intentionally not implemented by the two-Major-finding correction and are scheduled as separate work after PR #16 is merged.

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-325 | Planned | Align benchmark JSON Schema and Go representations | **Origin/severity:** independent review of PR #16, Minor. **Components:** `benchmarks/baseline.schema.json`, Go benchmark types, strict decoder, schema-drift tests. **Risk:** `exit_statuses`, numeric representation/range rules, and future nested schema changes can differ between published schema consumers and the Go release gate. **Acceptance:** define equivalent exit-status value constraints and numeric representation/ranges in schema and Go, inventory all current nested required/type/enum/pattern/minimum/additional-properties rules, and add deterministic drift tests proving schema and runtime acceptance remain aligned. **Timing:** separate work after merge of PR #16; not fixed by the Major-finding correction. |
| BL-326 | Planned | Reject malformed Unicode in strict benchmark JSON | **Origin/severity:** independent review of PR #16, Minor. **Components:** strict JSON decoder and mutation tests for artifacts, budgets, and workflow catalogs. **Risk:** invalid UTF-8 bytes and unpaired UTF-16 surrogate escapes are silently replaced with U+FFFD instead of being rejected as malformed input. **Acceptance:** reject invalid raw UTF-8 and unpaired surrogate escapes without rejecting legitimate U+FFFD text, retain escaped-property duplicate detection, and add positive/negative Unicode fixtures at relevant nesting levels. **Timing:** separate work after merge of PR #16; not fixed by the Major-finding correction. |
| BL-327 | Planned | Make all benchmark hard-failure diagnostics deterministic | **Origin/severity:** independent review of PR #16, Minor. **Components:** hard measurement/budget set validation and artifact diagnostic aggregation. **Risk:** remaining Go-map iteration in measurement failure paths can reorder otherwise identical hard-failure messages across processes, destabilizing CI diagnostics and exact failure signatures. **Acceptance:** deterministically order every missing/unknown hard measurement and budget diagnostic plus final aggregated messages, with multi-error tests across fresh evaluations. The PR #16 Major correction sorts its new soft-definition set errors only and does not claim this task complete. **Timing:** separate work after merge of PR #16; not fixed by the Major-finding correction. |
| BL-328 | Planned | Bound strict benchmark JSON resource consumption | **Origin/severity:** independent review of PR #16, Note. **Components:** artifact/budget/catalog file loading and strict JSON shape/typed decoding. **Risk:** repository-controlled oversized JSON is fully read and materialized in an untyped tree before a second typed decode, allowing disproportionate memory/CPU use. **Acceptance:** define justified per-file and relevant collection/string limits, reject oversize inputs before full materialization, preserve duplicate/shape guarantees, and add bounded large/deep negative tests without performance baseline work. **Timing:** separate work after merge of PR #16; not fixed by the Major-finding correction. |
| BL-329 | Planned | Bind platform baseline filenames to embedded identity | **Origin/severity:** independent review of PR #16, Note. **Components:** required platform baseline loader and swap/identity tests. **Risk:** complete Windows/Linux file contents can be swapped because the loader reindexes only by embedded `os`, leaving repository filenames mislabeled. **Acceptance:** derive the expected OS/architecture from each fixed filename, compare it directly with embedded identity before map insertion, and add swapped-content negative coverage without remeasuring or modifying baseline data. **Timing:** separate work after merge of PR #16; not fixed by the Major-finding correction. |

### PR #21 final-review follow-up

These tasks originate in the final independent review of PR #21. They are accepted as non-blocking Minor follow-ups and were not implemented in PR #21.

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-330 | Planned | Define the canonical `In Progress` backlog status contract | **Origin/severity:** final independent review of PR #21, Minor. **Components:** `BACKLOG.md`, `scripts/Test-DocumentationConsistency.ps1`, sprint-assignment validation, and documentation-quality-gate tests. **Risk:** the gate accepts `In Progress` although the canonical status legend does not define it, and current assignment validation applies only to `Planned`; future active tasks could therefore bypass intended sprint-assignment semantics. **Acceptance:** decide whether `In Progress` is a permanent canonical status or a temporary implementation-only state; either document its meaning and required sprint-assignment rules consistently in backlog and gate tests, or remove its acceptance from the gate; add positive and negative tests proving all accepted statuses follow the documented assignment contract. **Timing:** separate governance/documentation work after merge of PR #21; not fixed in PR #21. |
| BL-331 | Planned | Align ARM64 validation documentation with the implemented runner model | **Origin/severity:** final independent review of PR #21, Minor. **Components:** `docs/decisions/file-and-product-metadata-decisions.md`, build/release metadata documentation, manual validation guidance, and related PR/release wording. **Risk:** one decision passage can be read as evidence that native Windows ARM64 and Ubuntu ARM64 runners already execute tests, while the implemented and validated state is cross-compilation plus static ARM64 validation on x64 hosts. **Acceptance:** clearly distinguish current implementation from target state; document x64-hosted cross-compilation and static ARM64 validation as current behavior, label native ARM64 execution as future/conditional where applicable, and verify consistency across decision, build, testing, and manual-validation documents. **Timing:** separate documentation follow-up after merge of PR #21; no build or CI matrix rerun is required unless implementation behavior changes. |
| BL-332 | Planned | Remove contributor-specific path markers from native leak validation | **Origin/severity:** final independent review of PR #21, Minor. **Components:** `scripts/linux-native-driver.sh`, Windows-to-WSL orchestration inputs, leak-scan fixtures, and focused native validation tests. **Risk:** fixed contributor and synchronized-company path markers make local validation host-specific and reduce portability even though they are negative scan values rather than credentials and do not enter release artifacts. **Acceptance:** derive forbidden host/user/synchronized-root values at runtime or pass them explicitly from the orchestrator; keep deterministic generic fixtures for stable regression coverage; prove contributor-specific paths are still detected without embedding personal or organization-specific literals in repository scripts; retain fail-closed leak scanning and add focused Windows/WSL tests. **Timing:** separate local-validation hygiene follow-up after merge of PR #21; release artifacts and product code remain unchanged. |

### Governance integration and dependent INF-121 follow-up

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-333 | Done | Establish change-trigger, review-mode, and handoff governance | **Completed:** the canonical change-trigger, review-mode, finding-remediation, readiness, report, and handoff foundation is implemented and independently reviewed. PR #27 merged through regular merge commit `e42d57d57ea075640c9b123a533057bcac3861b8`; its second parent preserves all six PR commits at `c9b54c9be0cc96d9fc7f81841e28dc7a9b89fc74`, and merge/head trees plus all 26/26 paths are identical. REV-007, REV-008, REV-010, REV-013, REV-015, `PR27-EXACT-REV-001`, `PR27-EXACT-REV-002`, and `PR27-EXACT-REV-003` are closed with no remaining BL-333/BL-334 or PR27-EXACT finding. Post-merge CI, Metadata Regression, and CodeQL passed; exact-head/workflow-source parity, PowerShell 7.6.4, 1,051 governance checks with zero errors, and 198/198 fixtures passed. Durable evidence is retained and the five complete temporary review packages remain preserved because none is a redundant disposable copy. No runtime or product logic changed. |
| BL-334 | Done | Enforce change-trigger, finding-remediation, and handoff governance | **Completed:** fail-closed enforcement covers diff-derived triggers, checkpoints, immutable review modes, actual correction/current-delta bytes, exact finding and status parity, strict schemas, external path mappings, bounded handoff/readiness, commit preparation, and authoritative Hosted-CI provenance. PR #27 merged at `e42d57d57ea075640c9b123a533057bcac3861b8` with two-parent, tree, six-commit, and 26/26 path parity. REV-007, REV-008, REV-010, REV-013, and REV-015 remain `CLOSED_BY_INDEPENDENT_REVIEW`; RUN-007 remains `CORRECTED_AND_INDEPENDENTLY_REVIEWED`; `PR27-EXACT-REV-001`, `PR27-EXACT-REV-002`, and `PR27-EXACT-REV-003` remain `CLOSED_BY_INDEPENDENT_EXACT_COMMIT_REVIEW`. The finding and exact-review queues are empty. Post-merge Hosted CI passed with PowerShell 7.6.4, 1,051/0 governance, and 198/198 fixtures. Durable evidence is secured, controlled cleanup removed no sole-copy review package, and runtime/product logic remain unchanged. |
| BL-335 | Done | Migrate FlashGate reference-bound legacy Temp objects to local Temp | **Completed:** `TMP-001`, `TMP-004`, `TMP-016`, and `TMP-059` were copied to `<CodexTempRoot>` and `REF-005` through `REF-007` were atomically rebound with a complete productive reference backup. Productive migration, focused function validation, and the concrete isolated rollback rehearsal passed. The four original source objects were then removed as one controlled source-removal assignment; final read-only evidence confirms `SourceObjectCountAfter=0`, four unchanged productive targets, three unchanged active target references, an unchanged productive backup, no quarantine remainder, parser/hash/manifest parity, and a clean repository before this governance convergence. `BL335-D-VAL-012` is `CLOSED_BY_INDEPENDENT_REVIEW_VALIDATION_CONTROL_INTERFERENCE`: the only failed post-removal activity gate observed the concurrently running plaintext Codex monitor, while the identical post-monitor diagnostic was processes/tasks/shortcuts/exclusive-probe failures `0/0/0/0`. No BL-335 finding remains open. Durable evidence is in the BL-335 Freigabe B, Freigabe C, VAL-008, and Freigabe D reports under `Codex-Work\Reports`. |

### Governance handoff generalization follow-up

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-336 | Done | Generalize governance handoff contracts and commit-preparation validation | **Completed:** explicit task-neutral `GENERIC_COMMIT_PREPARATION` and isolated `FINDING_CORRECTION` profiles now bind complete repository, scope, patch, inventory, manifest, validation, review, and report evidence while preserving the historical BL-333/BL-334 contracts and all fail-closed path, UTF-8, independence, and readiness gates. Windows and native Linux validation under PowerShell 7.6.4 passed, including 31/31 focused and 85/85 generic cases; the historical 225-case matrix remains compatible. The focused independent Exact-Head Delta Review of technical head `8f29ee8e0b8c841b204506b32fbb617648f5bf4b` passed with no new findings, warnings, or failures. `BL336-PR31-REV-001` through `BL336-PR31-REV-004` are `CLOSED_BY_FOCUSED_INDEPENDENT_EXACT_HEAD_DELTA_REVIEW`; `BL336-PR31-WARN-001` is `CLOSED_BY_PR_METADATA_CONVERGENCE`; `OpenFindingCount: 0`. The final status commit is documentation-only and introduces no MCP runtime or product behavior change. **Sequence:** BL-230 integration may resume before BL-324. |

### Post-BL-230 governance and host-lifecycle follow-up

| ID | Status | Task | Scope and acceptance notes |
|---|---|---|---|
| BL-337 | Planned | Isolate governance fixture execution in one controlled runner process | Each fixture run owns exactly one controlled runner process identified by PID and start identity; deterministic timeouts use bounded kill, wait, and stream drain; terminal evidence proves cleanup, no surviving fixture/validator process, and unchanged repository state. Every terminal case emits one machine-readable per-case ProgressEvent with only technically required fields. Identical events are suppressed; a heartbeat is explicitly typed, emitted only after the configured interval, and never duplicated for the same interval/state. Output occurs only for progress, phase change, status change, or heartbeat. Missing progress instrumentation on a recurring long run is a finding. |
| BL-338 | Done | Add canonical governance case metadata and deterministic selection | One machine-readable leading inventory owns every case ID, group, tag, supported platform, required capability, and Windows-only dependency marker; no Shell/PowerShell array or second maintained list is canonical. `-ListGroups`, `-ListTags`, `-ListCases`, group/tag selection, and the compatible `-CaseName` path derive from that source. All selectors resolve completely before runner-process start and each selected token resolves to exactly one canonical case. Unknown, duplicate, ambiguous, platform-incompatible, or capability-incomplete selections stop fail-closed with structured diagnostics containing the affected IDs and no redundant summary fields. The deterministic metadata inventory and resolved selection are SHA-256-bound. **Completed:** the 277-case JSON inventory, closed schema, shared resolver, all three list interfaces, CaseName/Group/Tag preflight, platform/capability gating, semantic hashes, structured diagnostics, and permanent focused fixture matrix are implemented and locally validated. The independent full review plus focused delta review closed BL338-REV-001..004. Classic subsequently raised `BL338-CLASSIC-REV-001` because the Windows fixture harness depended on contributor-local Codex-Work/INF-160 files. The bundled correction removes that repository/Hosted-CI dependency, keeps INF-160 at the local Codex caller boundary, and adds permanent repository-only portability coverage. The focused independent Classic delta reviews closed `BL338-CLASSIC-REV-001` and `BL338-CLASSIC-REV-002` with no new finding; `BL338ClassicPreCommitQualityGate=PASS`. Implementation commit `a28953b19d1e3ff51ffdbd12262964787a9252dc` was merged through PR #40 as `b72c29e5d65803b11463f8d6b3d6f304cf510bf6`. The prior native Linux `standard` evidence remains valid because the corrected boundary does not change case semantics or Linux execution. Post-merge CI #122, Metadata Regression #51, and CodeQL #68 passed; BL-338 is `Done` with `OpenFindingCount=0`. BL-337 consumes only the resolved set and remains `Planned`; BL-340 remains unimplemented and `Planned`. |
| BL-339 | Done | Provide reusable focused and full governance validation orchestration | **Completed:** implementation and Full Completion passed, and the final independent closure review passed against immutable 19-member package SHA-256 `425A8B4E3D5497C40119E58291E773B25CF02675084653A4C73E685F6ABFB119` (154308 bytes) with no new finding. REV-001 through REV-014 are closed; REV-001/002 retain their prior independent-delta closure and REV-003..014 are `CLOSED_BY_INDEPENDENT_DELTA_REVIEW`. `OpenFindingCount=0`; no further BL-339 correction or review cycle is required. PR #34 merged through `26734c333341455a63f79c0f1a956309e54177e0`, and all post-merge CI, Metadata Regression, and CodeQL checks passed. BL-339 has no remaining gate or work. |
| BL-340 | Planned | Complete governance generator/profile migration | Migrate the unchanged workflow generator and all current reusable governance profiles to the complete BL-339 orchestration contract. New generated records must bind a valid `currentStateGate`; stored schema-version-1 records remain explicitly readable under their historical schema without becoming current readiness evidence. Reuse BL-339 orchestration rather than duplicating its implementation or reopening any BL339-REV finding. **Acceptance:** generator, profile, schema/catalog, transition, compatibility, and documentation triggers are identified; focused positive and fail-closed fixtures cover current-record generation, absent/stale state binding, profile parity, and historical-v1 reads; directly affected governance and documentation gates pass; the task is independently implementable and reviewable. BL-340 is an independent SPR-61 task and is not a new prerequisite for resuming BL-324. |
| BL-341 | Planned | Define cross-mode host-process ownership and lifecycle | ADR-0017 binds direct STDIO, proxy-edge, and persistent service process owners; connection ownership; owner/transport loss; PID plus start identity; bounded shutdown; orphan classification; instance diagnostics; service persistence; separation from Managed Process and Operations/Jobs; and Windows/Linux lifecycle-test ownership. BL-241 retains integrated multi-client/lifecycle tests and BL-129 retains managed-child cleanup. |

BL-339 is terminal `Done` with Full Completion and final independent review
`PASS`.
All REV-001..014 findings are closed and `OpenFindingCount=0`. Its productive
`FINDING_CORRECTION` / `BUNDLED_CORRECTION_TO_FOCUSED_DELTA_REVIEW` contract,
per-finding parity gates, permanent focused evidence, full-completion evidence,
and immutable final package remain authoritative. No further BL-339 correction
or review cycle is required. PR #34 merged through
`26734c333341455a63f79c0f1a956309e54177e0`; its post-merge workflows passed.
BL-339 has no remaining gate.

The Post-BL-230 compatibility correction did not itself complete BL-338 or the
residual migration now registered as BL-340.
It removes the current runner's fixed leading count in favor of its derived
ordered inventory and SHA-256, and keeps pending schema-version-1 records from
the unchanged workflow generator compatible. Any current readiness claim and
the versioned `GENERIC_COMMIT_PREPARATION` profile still require a valid
`currentStateGate`. Full generator/profile migration is BL-340; group/tag
metadata and reusable selection remain BL-338.

Completed BL-339 was the required enabler for the now-completed BL-324 work.
Its scope was limited to explicit isolated source/worktree selection; exact HEAD,
tree, and file-hash binding; toolchain/platform, execution-context, and selector
preflights; and standardized Git/PowerShell probes. It uses BL-339 and creates
no new ID.

BL-341 is the unchanged former BL-340 host-process ownership/lifecycle task
under ADR-0017. The renumbering and residual-scope extraction are recorded in
`docs/backlog-id-migration-2026-08-12.md`; historical evidence retains the IDs
that were canonical when it was produced.

BL-336 PRE_COMMIT checkpoint: `BL336-VAL-001` and `BL336-VAL-002` remain
`CLOSED_BY_IMPLEMENTATION_AND_FULL_REVALIDATION`; `BL336-REV-001`,
`BL336-REV-002`, and `BL336-REV-003` remain
`CLOSED_BY_INDEPENDENT_DELTA_REVIEW`. The private and undeclared host-path
negatives now use one closed seven-class factory and assemble deterministic
synthetic values only at fixture runtime. All seven re-signed packages reach
the unchanged `GENERIC-CLASSIFIED-HOST-PATH-POLICY` gate and fail there with
the expected check exactly once. No complete private synthetic path literal
remains in the reviewed delta. The generic matrix is 85/85 PASS, the historical
matrix remains 225/225 PASS, and the complete static governance,
documentation, schema, parser, deterministic-generation, BL-230,
finding-bearing BL-336, full-text-patch, inventory, manifest, authoritative
scope, internal-validator, and unchanged external-validator gates pass.

The trusted repository, baseline object, current HEAD, branch, complete
relevant Git status, tracked/staged state, mode, length, SHA-256, and
INCLUDE/EXCLUDE decisions remain bound to the isolated worktree. The earlier
BL-336 findings remain closed. PR #31 findings `BL336-PR31-REV-001`,
`BL336-PR31-REV-002`, `BL336-PR31-REV-003`, and
`BL336-PR31-REV-004` are
`CLOSED_BY_FOCUSED_INDEPENDENT_EXACT_HEAD_DELTA_REVIEW`; therefore
`OpenFindingCount: 0` and BL-336 is `Done`. The corrections add
baseline-bound tracked deletions, source/target-bound tracked renames,
platform-classified untracked modes, complete binary patch parity, isolated
temporary object writes, real-object inventory parity, literal pathspecs,
actual NUL-separated delta parity, EXCLUDE prohibition, and the corresponding
positive and fail-closed negative packages. Alternate and fixture-divergence
paths are assembled component by component with no compound backslash child
literal. `BL336-PR31-WARN-001` is `CLOSED_BY_PR_METADATA_CONVERGENCE`. The
PRE_COMMIT revalidation passes the 85/85 generic fixtures on Windows and
natively on Linux under PowerShell 7.6.4, including the real Unix executable
end-to-end case; the 31/31 focused matrix passes on both platforms. Historical
fixtures remain 225/225 PASS, with complete static governance and documentation
checks, zero parser, JSON, strict-UTF-8, diff,
object-inventory, or parallel-worktree errors, and no warnings. The directly
caused seven-case historical check-ID delta also passes 7/7. The PRE_COMMIT
change-trigger result is `EXISTING_GATES_REQUIRED`; triggered domains are
governance schema, handoff validation, fixtures, and documentation; existing
backlog coverage is BL-336; no new backlog item or product/runtime decision is
introduced. The independent technical review of exact head
`8f29ee8e0b8c841b204506b32fbb617648f5bf4b` is `PASS`, with review warnings,
review failures, new findings, and open findings all `0`. The current
documentation/status closure changes no technical path or previously reviewed
technical byte. Its resulting new Exact Head requires only the separately
packaged final read-only review gate. Ready-for-Review, reviewer requests,
other PR metadata changes, merge, rebase, and force-push remain prohibited;
the persistent catalog keeps general remote actions closed.

The highest assigned backlog identifier is `BL-341`. BL-339 is `Done`; BL-340
and BL-341 are distinct `Planned` tasks.
BL-333, BL-334, BL-335, and BL-336 remain `Done`.

PR #27 merged at `e42d57d57ea075640c9b123a533057bcac3861b8`.
Its first parent is `537ea1c1660cddfde5aace1888242d80a6be77bf`;
its second parent is `c9b54c9be0cc96d9fc7f81841e28dc7a9b89fc74`.
The second-parent history retains the six PR commits
`b97cb1bf40399e2489bdacc1487038bfdba46506`,
`981c9be89b5fb8eca21ea1003758dede7695ef8d`,
`4afd08dad8a1544b4e3b2cede0d9c037bd90d132`,
`63824dd2b90da4cbc4d91322a5d5fea00d892d76`,
`ecbd8dc61905c82cfdcb9386c0587c1089635f47`, and
`c9b54c9be0cc96d9fc7f81841e28dc7a9b89fc74`. Merge/head tree parity and
26/26 path parity passed. Post-merge CI, Metadata Regression, and CodeQL are
terminal successful; exact-head and workflow-source parity, PowerShell 7.6.4,
1,051/0 governance, and 198/198 fixtures passed. All earlier findings and all
three PR27-EXACT findings are closed. BL-333, BL-334, and BL-335 are `Done`.
The binding remaining queue is:
`schedule BL-340 independently in SPR-61 → final documentation convergence → Local Work Register dissolution audit → separately authorized Local Work Register removal`.

## Cross-epic rules

- `Planned` means required for Version 1.0; `Later` means accepted post-Version-1.0 work. A task changes milestone only through an explicit backlog and documentation decision.
- Security tasks apply to their domain tasks without duplicating canonical definitions.
- New installations with configured roots but no explicit profile default to the safe read-only profile; higher-risk profiles require explicit activation.
- Payload-heavy content is transferred once. Structured metadata, resource handles, and compatibility fallbacks must not duplicate large file, process, search, or binary payloads.
- Every service request has both an authenticated caller identity and an effective execution backend. Version 1.0 implements the service-account backend only; the user-worker backend is interface-compatible but post-1.0. In-process impersonation is prohibited.
- Handles, jobs, caches, result resources, temporary files, cancellation, and audit correlation are bound to principal, profile, root, execution backend, and service generation.
- Global limits are insufficient for service mode; per-principal quotas and fair scheduling are mandatory.
- Native Go/OS APIs are preferred. External programs require typed no-shell definitions and evidence; interpreter-based adapters are excluded from Version 1.0.
- Operations/jobs are optional lifecycle infrastructure; short synchronous operations may run directly and domain logic/ownership stays outside the manager.
- Benchmarks and threat models must justify separate product binaries, indexes, or external adapters/providers. The same-binary local service IPC accepted by ADR-0014 still requires its defined security, compatibility, and benchmark release gates.
- FlashGate modules/providers and MCP protocol extensions are separate concepts and contracts.
- Deprecated MCP Roots is never the foundation of named-root authorization.
- MCP annotations never replace server-side authorization.
- Planned tool cleanup and technical rename occur only in their dedicated sprints.
- Before Version 1.0, breaking changes are allowed but require coordinated tests, documentation, examples, smoke tests, and changelog entries. Version 1.0 requires a documented compatibility, deprecation, and migration policy.
