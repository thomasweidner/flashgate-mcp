# FlashGate MCP benchmarks

`SPR-47` provides one reproducible benchmark system with three layers. It extends the existing `tools/call` serialization fixtures instead of creating a competing serialization suite.

## Layers

1. In-process Go benchmarks measure direct `tools/call` handler work, result construction, JSON serialization, allocations, result bytes, response bytes, and `tools/list` wire output for read-only and default profiles.
2. `cmd/benchmark` starts the real previously built FlashGate binary and exchanges JSON-RPC over STDIO.
3. Ten reference workflows use a deterministic temporary corpus and report calls, wire sizes, result sizes, duration, filesystem counters, entry counts, resources, and the optional byte-based token approximation.

`cmd/benchmark` is a development tool. Release workflows continue to build and publish only `cmd/server`.

## Commands

Windows standard run (one first process plus 30 subsequent processes and 30 repetitions per workflow):

```powershell
& ".\scripts\benchmark.ps1"
```

Windows quick run (one first process plus 10 subsequent processes and 10 repetitions per workflow):

```powershell
& ".\scripts\benchmark.ps1" -Quick
```

Linux equivalents:

```bash
bash scripts/benchmark.sh
bash scripts/benchmark.sh --quick
```

The legacy `-RecordBaseline` and `--record-baseline` flags remain recognized for
compatibility but fail closed before any Go command, build, directory creation,
benchmark execution, or output write. These wrappers are diagnostic development
entry points, not an authoritative baseline-recording path. Every explicit or
default non-authoritative output name is checked physically. The wrappers reject
alias, junction, symlink, and other reparse parents into the repository baseline
directory, as well as existing final file symlinks/reparse points and unresolved
canonical baseline targets. The Linux default output is subject to the same
checks as an explicit path, and both wrappers repeat their policy check
immediately before starting the benchmark binary.

The runner then enforces the write boundary independently of the wrapper. It
opens the protected baseline directory and validated output parent as stable
`os.Root` handles (a directory handle on Windows and dirfd-based root on Linux),
compares their physical identities, and never opens an existing output target
for writing. The result is written and synced to a new exclusive temporary file
through the bound output root and published through a handle-relative rename,
which replaces rather than follows a final symlink introduced after validation.
The published directory entry must still identify the same opened file. An
output-parent or protected-directory path that changes after validation causes
fail-closed cleanup through the bound root. Normal diagnostic output therefore
cannot write through an alias into a versioned baseline; authoritative baseline
recording remains a separate process.

## Measurement environment

The primary Windows development host has a known resource-intensive scheduled workload every day from 19:00 inclusive until 04:00 exclusive in the `Europe/Vienna` time zone. Performance, latency, startup, CPU, memory, resource, cross-platform, and regression baselines must not be recorded, approved, compared, or used to tune budgets during that blocked window. The preferred measurement window with safety margin is 04:15–18:45 `Europe/Vienna`, and a complete measurement series must finish before 19:00.

The time window is necessary but not sufficient: known or unusual additional host load invalidates a baseline even during otherwise allowed hours. The measurement report must record the time zone, start/end window, and host-load status. Quick runs are diagnostic observations, not automatic release baselines. Contaminated runs remain diagnosis evidence but are not approval artifacts, and they must never be replaced or selectively cherry-picked in favor of more favorable individual measurements.

Functional tests, builds, vet, and lint remain valid during the blocked window, but any incidental timing or resource values from those commands are not performance evidence. Normal benchmark runs may execute during the blocked window and are explicitly marked contaminated. The diagnostic wrappers cannot record or publish a baseline.

Run only the in-process benchmarks:

```bash
go test -run '^$' -bench 'Benchmark(CallToolResultSerialization|CallToolHandlerProcessing.*|ToolsListWireSerialization)$' -benchmem ./internal/mcp/tools ./cmd/server
```

## Start and resource semantics

`first_process_start` is the first new server process started by the benchmark command. In a diagnostic wrapper run it follows the wrapper build; in an authoritative run it uses the previously prepared binary after the required quiet period and host gate. `subsequent_process_start` contains later new processes in the same run. These labels do not claim that an operating-system cold filesystem or executable cache was guaranteed or cleared.

Startup duration begins immediately before the operating-system process start call and ends when a valid `initialize` response has been received. The runner validates the negotiated protocol version, `serverInfo`, and `capabilities`, then sends the no-ID `notifications/initialized` notification before any later request or controlled exit. Workflow duration begins at process start and ends when the final valid response has been received. Controlled stdin closure and process exit validation happen after the measured response interval.

On Windows, the runner uses `OpenProcess`, `GetProcessMemoryInfo`, and `GetProcessTimes` to read current/peak working set plus user/kernel CPU time. On Linux, it reads `VmRSS` and `VmHWM` from `/proc/<pid>/status` and user/system CPU ticks from `/proc/<pid>/stat` independently. Available Linux values remain present when another metric is missing; each missing value is named in `unsupported_metrics`. Other operating systems report resource status `not_supported`, omit unsupported numeric metrics, and never emit plausible zero placeholders.

Idle working set is sampled immediately after `initialize`. Peak working set and CPU time are sampled after the final workflow response while the server is still alive.

## Counter definitions

- `request_bytes`: complete UTF-8 JSON-RPC request including its JSONL newline.
- `response_bytes`: complete UTF-8 JSON-RPC response including its JSONL newline.
- `result_bytes`: only the serialized JSON value in the JSON-RPC `result` member.
- `read_bytes`: content bytes successfully returned by `read_file`.
- `written_bytes`: content bytes successfully written or copied; all `SPR-47` read-only reference workflows correctly report zero.
- `scanned_bytes`: bytes actually inspected for search, hashing, classification, or comparable content analysis. All `SPR-47` read-only workflows report zero; ordinary `read_file` return bytes are not scans.
- `entries`: directory entries actually returned by successful reference calls.
- `calls`: `tools/call` requests actually executed successfully. `initialize` and `tools/list` are not counted.

These benchmark counters are runner-side measurements only. `SPR-47` does not add them to public MCP tool results.

Workflow request byte counts include the initialization request and the 55-byte `notifications/initialized` JSONL notification. The notification has no response and never increments `calls`. The separate `tools_list_measurements` entries contain only the `tools/list` request and response.

## Reference workflows

The machine-readable catalog is `workflows.json`. It covers initialize, initialize plus `tools/list`, existing and missing `get_path_info`, small and 64-KiB reads, small and 500-entry directory listings, ten independent path checks, and ten independent file reads. Every repetition starts a new read-only server process.

The corpus is created below the operating-system temporary directory, is removed after the run, and is never serialized into results.

## Token approximation

`approx_tokens_bytes4` is exactly:

```text
ceil(utf8_bytes / 4)
```

It is an orientation only, is not model-specific, does not use a tokenizer, and is not suitable for billing. Workflow values approximate complete response bytes; `tools/list` values approximate its complete response bytes.

## Baselines and budgets

`baseline.schema.json` defines result format `flashgate-benchmark/v1`. A result records project, commit, whether the binary came from a dirty working tree, Go version, OS, architecture, repetitions, starts, resources, `tools/list`, workflows, warnings, budget evaluation, unsupported metrics, and stable suite/catalog/corpus plus runtime/transport/backend/profile/parallelism provenance.

Versioned baseline creation is deliberately two-stage. First complete tests, vet,
lint, parser validation, and builds in isolated Windows and native Linux checkouts
from the same clean implementation commit. After the final preparation operation,
wait at least 180 seconds without further build, Git, scan, archive, or analysis
work. Then run exactly one authoritative three-block host preflight that records
CPU, disk activity, memory, and per-process CPU deltas. Only a passed preflight may
be followed by direct invocation of the already prepared benchmark and server
binaries. Run a 15-second intermediate gate before Linux and a final host gate
before copying, hashing, verifying, or archiving results.

The Windows source bundle, checkout, prepared binaries, output, logs, verification,
and controller stay below `C:\Voxtronic\Codex\Temp\Benchmarks` on local fixed NTFS
storage without reparse points. OneDrive, Dropbox, redirected folders, network
shares, and other synchronized paths are prohibited during the measured phase.
The Linux checkout and temporary output stay on native ext4 under `/home`, never
under `/mnt` or `/media`. OneDrive archival occurs only after the final host gate.
The legacy wrapper record flags are intentionally blocked; a policy-compliant
controller is prepared and independently reviewed for each authoritative attempt.

Ordinary local benchmark results remain allowed on dirty trees and record
`working_tree_dirty: true` accurately, but results collected during the scheduled
host-load window are contaminated diagnosis only.

Versioned results must not contain absolute host paths, user names, secrets, temporary directory names, or raw private environment variables. The runner never serializes its binary path, corpus root, or environment and replaces known paths in captured stderr.

Repository artifact tests load the Windows and Linux baselines together, enforce
complete provenance, budget, resource, sample, exit-status, stderr, warning, and
unsupported-metric gates, and compare an explicit deterministic cross-platform
projection while excluding only time, memory, CPU, generation time, and platform
identity fields.

`budgets.json` separates deterministic hard contracts from noisy soft review limits:

- Hard: complete and exact tool-profile/workflow measurement sets, tool/schema counts, wire/result byte maxima, reference workflow calls/counters, and all six selected-result allocation/payload records loaded from `budgets.json`.
- Soft: startup p95, workflow p95, idle/peak working set, and CPU time.

The versioned-artifact gate does not trust embedded `budget_evaluation`. It strictly
decodes each platform artifact and the canonical budget/workflow definitions,
rejecting unknown, duplicate, missing, mistyped, null, or trailing JSON content.
Typed Go invariants cover the complete current `baseline.schema.json` contract
without a third-party schema dependency. Hard and soft results are recomputed from
the loaded measurements only after exact hard/soft workflow key sets and positive
soft limits have been validated. The result is compared exactly with the embedded
evaluation, and any hard failure is rejected before the independent Windows/Linux
consistency check. Matching soft warnings retain their review-only meaning through
the complete platform gate and are excluded from deterministic cross-platform
projection. Runner result construction records budget messages only in
`budget_evaluation`; general result `warnings` contain only non-budget runtime
warnings and remain forbidden in clean versioned baselines.

Payload and allocation contracts are both validated in ordinary tests. Under race
instrumentation the functional serialization, payload, fixture, and budget-contract
checks still run, while only the `testing.AllocsPerRun` assertion is skipped because
the race detector changes allocation behavior. The authoritative race gate runs on
a supported race platform; for the current Windows host, missing CGO/GCC is an
infrastructure limitation and does not justify relaxing allocation budgets. Native
Linux `go test -race ./...` remains required.

A hard failure makes the local benchmark command fail after writing its JSON result. A soft excess is recorded as a warning for review. `SPR-47` does not add the full process benchmark to CI; cross-run baseline comparison and CI enforcement remain BL-249 and BL-250.

## Version 1.0 benchmark expansion

`SPR-47` baselines are created only after the corrected implementation commit is clean. They are not retroactively rewritten when Version 1.0 contracts change.

Version 1.0 extends the benchmark system with the following measurements:

- useful payload bytes distinct from response and result bytes;
- wire amplification factor: `response_bytes / useful_payload_bytes` for payload-bearing operations;
- approximate token cost per useful byte;
- initialization instructions bytes and approximate tokens;
- per-profile `tools/list` bytes, approximate tokens, tool count, and deterministic catalog fingerprint;
- direct STDIO, proxy-to-service, and service-backend latency/CPU/memory overhead;
- per-principal queue, concurrency, fairness, and overload behavior;
- large-result inline, page, cursor, stream, and resource-handle behavior;
- text, media, binary, directory, search, process-output, and system-information payload classes;
- audit/logging overhead under bounded normal load;
- native adapter versus any proposed external native-program adapter before adoption.

Payload-heavy content must be counted once as useful payload even when a client-compatibility fallback causes additional wire bytes. Metadata-only operations report zero useful payload and are evaluated through absolute response/catalog budgets rather than division by zero.

## Runtime-mode benchmark matrix

Version 1.0 records at least:

| Mode | Required measurements |
|---|---|
| Direct STDIO | startup, initialize, catalog, workflow latency, CPU, RSS/working set, wire/result/useful bytes |
| Proxy plus local service | proxy startup, connection/handshake, end-to-end latency, proxy/service CPU and memory, IPC bytes |
| System service backend | idle service cost, per-client cost, concurrency, queueing, caller authorization, service-account backend cost |
| Auto mode | successful service discovery and no-service direct fallback without elevation |

Windows and Linux results remain separate. A platform result must not substitute unsupported metrics with plausible zero values.

## Cross-project comparison

`BL-261` adds a reproducible comparison against pinned versions or commits of:

1. FlashGate MCP;
2. the official Node.js filesystem reference server;
3. one selected native Rust filesystem MCP;
4. one selected Go filesystem MCP.

The comparison uses the same host, corpus, requested functionality, warm-up policy, repetitions, payload definitions, and reporting format. Missing functionality is reported as `not_supported`; it is not emulated through unmeasured wrappers. The report must archive exact versions, configurations, commands, and raw machine-readable results and must not claim superiority outside the measured workflows.

## Version 1.0 release use

The Version 1.0 gate in `BL-263` requires approved hard budgets for deterministic protocol/catalog/payload contracts and reviewed soft budgets for host-sensitive latency, CPU, and memory. New optional accelerators or external programs are not accepted into the initial release unless they demonstrate a material benefit and pass the same security and portability review.

<!-- FLASHGATE_PERFORMANCE_WORKSPACE_POLICY_START -->
## Primary Windows host workspace isolation

Authoritative performance and baseline attempts on the primary Windows
development host use the local workspace:

`C:\Voxtronic\Codex\Temp\Benchmarks`

The source bundle, Windows checkout, prepared binaries, output JSON files, logs,
verification evidence, and measurement controller must remain below that local
root until both platform measurements and the final host-load gate are complete.

OneDrive, Dropbox, redirected profile folders, network shares, and other
synchronized locations are not valid measurement workspaces. Final reports and
review bundles may be copied to archival synchronized storage only after the
measured phase has ended.

Native Linux measurements continue to use the distribution's native ext4
filesystem under `/home`, never a Windows-mounted path such as `/mnt`.
<!-- FLASHGATE_PERFORMANCE_WORKSPACE_POLICY_END -->
