# Benchmark method and baseline interpretation

This document is the durable reader's guide to FlashGate's benchmark method and
committed baselines. It describes the current `SPR-047` evidence without turning
diagnostic runs into release evidence or extending the Version 1.0 benchmark
scope.

## What is measured

The benchmark system deliberately separates three layers:

1. Go benchmarks measure in-process `tools/call` handling, result serialization,
   allocations, payload bytes, response bytes, and `tools/list` serialization.
2. `cmd/benchmark` starts a previously built `flashgate-mcp` process and measures
   JSON-RPC over STDIO, including initialization, workflow latency, and supported
   process resource counters.
3. The ten scenarios in
   [`../benchmarks/workflows.json`](../benchmarks/workflows.json) exercise a
   deterministic read-only corpus. Each repetition uses a new server process.

This division matters when interpreting a number. An in-process allocation count
is not an end-to-end process cost, and an end-to-end workflow duration includes
process startup and initialization. `first_process_start` means only the first
process started by that benchmark invocation; it is not evidence of a cleared OS
cache or a physical cold start.

The counter definitions, sampling points, supported platform metrics, token
orientation, and exact commands are maintained in the
[benchmark implementation reference](../benchmarks/README.md). The byte-based
token value is an orientation (`ceil(UTF-8 bytes / 4)`), not a tokenizer result or
billing estimate.

## Reproducible inputs

The following versioned files define the reproducible contract:

| Input | Purpose |
|---|---|
| [`workflows.json`](../benchmarks/workflows.json) | Ordered scenarios, steps, call counts, and deterministic expected counters |
| [`budgets.json`](../benchmarks/budgets.json) | Deterministic hard limits and host-sensitive soft review thresholds |
| [`baseline.schema.json`](../benchmarks/baseline.schema.json) | Machine-readable `flashgate-benchmark/v1` result shape |
| [`baseline.windows-amd64.json`](../benchmarks/baseline.windows-amd64.json) | Committed Windows AMD64 measurement |
| [`baseline.linux-amd64.json`](../benchmarks/baseline.linux-amd64.json) | Committed Linux AMD64 measurement |

The two committed platform artifacts identify the same clean source commit,
`cfd211fa81cc48ee1dc463966718442f2ab5223c`, use Go 1.26.4, contain 30
repetitions, and report no hard failures, soft warnings, or general warnings.
Those statements describe the checked-in artifacts; they are not a claim that a
future checkout has reproduced them.

Tests strictly decode the artifacts and canonical inputs, recompute budget
evaluation, validate complete workflow/profile sets and provenance, and compare
only the deterministic cross-platform projection. Platform identity, timestamps,
duration, CPU, and memory are intentionally not required to be byte-identical
between Windows and Linux.

## Diagnostic runs versus authoritative baselines

The repository wrappers are diagnostic development entry points:

```powershell
& ".\scripts\benchmark.ps1" -Quick
& ".\scripts\benchmark.ps1"
```

```bash
bash scripts/benchmark.sh --quick
bash scripts/benchmark.sh
```

They build local binaries, record whether the working tree is dirty, and write a
non-authoritative result outside the protected baseline directory. Their legacy
recording flags fail closed. A passing diagnostic run is useful for investigation
but cannot replace either committed platform artifact or establish a release
baseline.

An authoritative baseline is a separately controlled, two-platform measurement:

- Windows and native Linux use isolated clean checkouts of the same commit and
  prepared binaries from that commit.
- Validation and build work finishes before the required quiet period and host
  preflight.
- Windows uses a caller-bound workspace on local nonsynchronized NTFS storage;
  native Linux uses native storage below `/home`.
- Host-load gates bracket measurement and evidence handling. Contaminated runs
  remain diagnostic evidence and are not selectively replaced with favorable
  samples.
- The resulting artifacts must pass schema, provenance, budget, warning,
  supported-metric, and cross-platform consistency gates before they may replace
  versioned baselines.

The complete operational constraints remain normative in the
[benchmark implementation reference](../benchmarks/README.md). This guide does
not authorize baseline replacement, external publication, or release action.

## Reading budgets correctly

Hard budgets cover deterministic contract properties such as exact workflow and
profile membership, tool/schema counts, byte ceilings, call and filesystem
counters, and serialization payload/allocation records. A hard failure invalidates
the candidate artifact.

Soft budgets cover host-sensitive startup, workflow duration, CPU, and memory.
An excess is retained as a review warning rather than silently discarded. It
requires investigation and cannot be interpreted as either a deterministic
product failure or a passing release decision by itself.

Missing metrics are named explicitly. Unsupported CPU or memory measurements must
not be replaced with plausible zero values. Likewise, Windows and Linux baselines
are separate evidence; one platform cannot stand in for the other.

## Historical evidence and future work

The [SPR-047 report](benchmarks/spr-47-resource-latency-baseline.md) records the
historical implementation and measurement event. Historical values and decisions
remain evidence for that exact source state and are not rewritten when current
contracts change.

The current baseline covers the implemented read-only filesystem workflows. It
does not prove planned search, process, command, system-information, Operations/Job,
proxy, service, concurrency, large-result, or cross-project performance. Those
measurements remain Version 1.0 or later work under their canonical backlog
owners. Release use remains subject to the
[Version 1.0 release boundary](version-1-scope-and-release-boundary.md).
