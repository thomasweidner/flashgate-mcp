# Sprint 3.45d resource, latency, and payload baseline

Date: 2026-07-17

## Scope

Sprint 3.45d implements the benchmark code for BL-189 through BL-199 without changing public tool names, parameters, successful results, or tool-error behavior. BL-190 and BL-198 remain `Planned` until clean Windows and native Linux baselines exist on the same implementation commit. The existing six-fixture serialization benchmark remains the source for historical direct, text-only, and text-plus-structured comparisons.

The new implementation adds direct in-process handler measurement, deterministic wire-size gates, a real STDIO process runner, ten reference workflows, machine-readable result schema/baseline/budgets, and Windows/Linux resource collectors using only the Go standard library.

## Measurement architecture

### In-process

`BenchmarkCallToolResultSerialization` still reports `ns/op`, `B/op`, `allocs/op`, and `payload-bytes` for all existing variants and now also reports full JSON-RPC `response-bytes`. `BenchmarkCallToolHandlerProcessing` measures validation, tool execution against deterministic fakes, result construction, and wrapping. `BenchmarkToolsListWireSerialization` covers read-only and default profiles.

Tests pin all six result sizes and their response-envelope sizes. The existing `tools/list` test now pins these schema-bearing responses:

| Profile | Tools | Schemas | Request bytes | Result bytes | Response bytes |
|---|---:|---:|---:|---:|---:|
| Read-only | 3 | 3 | 59 | 2,099 | 2,134 |
| Default | 8 | 8 | 59 | 5,622 | 5,657 |

### End-to-end process

Each sample starts the real built server, sends `initialize`, validates the negotiated protocol version plus present `serverInfo` and `capabilities`, sends `notifications/initialized`, optionally performs the remaining workflow requests, samples resources while the process is alive, closes stdin, and requires controlled exit status zero. The notification is included in workflow request bytes but has no response and is not a `tools/call`.

The standard run uses one `first_process_start` sample and 30 `subsequent_process_start` samples. Quick mode uses one first sample and 10 subsequent samples. `first_process_start` means first process after the script's build step; it is not a claim that an OS cold cache was forced.

### Reference workflows

The ten workflows are versioned in `benchmarks/workflows.json`. Multiple-operation workflows use ten independent calls, so their MCP call-count benefit can be compared with future batch operations without conflating MCP exchanges with model turns.

Filesystem byte and entry counters are derived from successful structured results and the known operation semantics. They remain private to the benchmark runner.

## Clean baseline workflow

The preliminary dirty Windows artifact was removed. Baseline creation has two commits: first the corrected implementation commit without platform baselines, then a separate artifact commit containing clean Windows and native Linux baselines generated from that same implementation commit. The legacy wrapper record flags now fail closed before any Go command or write. Authoritative attempts use the documented two-phase prebuilt workflow with a minimum 180-second quiet period, one three-block preflight, prepared binaries, intermediate and final host gates, local Windows workspace isolation, and native Linux files under `/home`. Ordinary dirty-tree development runs remain supported and accurately record `working_tree_dirty: true`.

The corrected deterministic workflow budgets are:

| Workflow | Calls | Request bytes | Response bytes | Result bytes | Read bytes | Entries |
|---|---:|---:|---:|---:|---:|---:|
| initialize | 0 | 221 | 151 | 116 | 0 | 0 |
| initialize → tools/list | 0 | 280 | 2,285 | 2,215 | 0 | 0 |
| get_path_info existing | 1 | 338 | 426 | 356 | 0 | 0 |
| get_path_info missing | 1 | 337 | 326 | 256 | 0 | 0 |
| read_file small | 1 | 331 | 355 | 285 | 26 | 0 |
| read_file 64 KiB | 1 | 336 | 131,378 | 131,308 | 65,536 | 0 |
| list_directory small | 1 | 336 | 538 | 468 | 0 | 3 |
| list_directory 500 | 1 | 336 | 53,680 | 53,610 | 0 | 500 |
| multiple path checks | 10 | 1,513 | 3,123 | 2,736 | 0 | 0 |
| multiple file reads | 10 | 1,463 | 2,193 | 1,806 | 260 | 0 |

Every workflow above has `written_bytes = 0` and `scanned_bytes = 0`. The `read_bytes` values count returned file content only; search, hashing, classification, or comparable content inspection would count separately as `scanned_bytes` in later workflows.

The 64-KiB result remains approximately doubled by the selected text-plus-structured MCP contract, consistent with the Sprint 3.45a serialization baseline.

## Platform differences

Windows measurements use current and peak working set plus user/kernel process time from Win32. Linux reads `VmRSS`, `VmHWM`, and procfs user/system CPU ticks independently, preserving every available metric and explicitly naming missing ones. Linux source compatibility is validated by cross-building the benchmark command; a native Linux run must record its own `baseline.linux-<arch>.json` rather than reuse Windows soft measurements.

Other platforms explicitly use `not_supported` and omit resource numbers. They do not serialize zero placeholders as if measurements succeeded.

## Budgets

Hard budgets equal the deterministic wire/counter contracts and reject missing, duplicate, unknown, or incomplete tool-profile/workflow measurements. All six text-plus-structured fixtures load payload and allocation limits from `benchmarks/budgets.json`; the measured supported Go path remains within six allocations per operation. Timing and resource budgets are intentionally broad soft review limits to avoid normal runner noise failing builds.

The local runner evaluates budgets now. Adding full benchmark execution and baseline comparison to CI is intentionally deferred to BL-247 and BL-248.

## Token orientation

The only token field is `approx_tokens_bytes4 = ceil(UTF-8 bytes / 4)`. It is approximate, not model-specific, uses no tokenizer library, and is unsuitable for billing.

## Security and release boundary

Results exclude binary paths, corpus roots, user names, raw environment variables, and secrets. Known local paths are replaced in captured stderr before serialization. The corpus is deleted after the run.

`cmd/benchmark` is not added to release workflows. Release artifacts remain server binaries built from `cmd/server`.

## Backlog ID correction - 2026-07-20

This sprint report retains the identifiers valid when it was written.
The deferred benchmark CI tasks referenced above moved as follows:

- historical `BL-247` -> current `BL-249`
- historical `BL-248` -> current `BL-250`

The authoritative mapping is recorded in `docs/backlog-id-migration-2026-07-20.md`.
