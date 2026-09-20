# FlashGate MCP — Classic Independent Review Batches 15–16

**Status:** PASS_READ_ONLY_REVIEW_CONSOLIDATED
**Date:** 2026-09-18
**Bound main:** `f615d6f54c967b1fae04c5378f32af0b648b40f7`
**Bound tree:** `477fb951fa9ac5b27e051a8d320ca440e473eb9d`
**BACKLOG.md:** `a02db7c09b10dba8c827f62618c7c6eb9f096591`
**ReviewMode:** `INDEPENDENT_REVIEW`
**FindingFixesPerformed:** `false`

Review evidence only; no backlog status, correction or merge authority.

## Vacation exit

After vacation: `MobileTaskSelection=OFF`, normal current `BACKLOG.md` and governance resume.

## Batch 15 — SPR-055 Process Observation

- **BL-114/#188:** reuse Batch-01 P2; cursor parser must require trailing decode = `io.EOF`.
- **BL-115/#191:** narrow field-selection delta is acceptable; parent BL-114 remains blocked.
- **BL-116/#192:** new Classic P2 — Linux parser rejects empty `comm`, so a valid process cannot
  return requested `parentPid`/`threadCount` when name is empty and not requested. Parse topology
  fields independently of display-name completeness.
- **BL-117/#193:** reuse Batch-01 P2; dropping incomplete/empty-name entries can hide descendants
  without `partial=true`.
- **BL-118:** no dedicated current PR; later registration + `process.observe` execution-authorization
  integration join after corrected process and SPR-053/054 foundations.

## Batch 16 — SPR-056 Managed Process / Process CI

### Clean/reusable primitives
- **BL-119/#123:** clean registry primitive; monotonic internal never-reused InstanceID, not public
  authority.
- **BL-121/#199:** state primitive reusable after parents.
- **BL-123/#204:** wait delta reusable after parents.
- **BL-124/#206:** bounded incremental combined-capture delta reusable after parents.
- **BL-133/#226:** closed Status+PID evidence/stable start-stop error categories reusable after parents.

### Required remediation
- **BL-120/#197:** removed public handle values can be reused and stale requests can alias a new
  process.
- **BL-122/#202:** empty authorized Environment becomes nil `Cmd.Env` and inherits FlashGate server
  environment.
- **BL-125/#207:** full output ring shifts nearly entire buffer per small write; use true circular
  storage.
- **BL-126/#209:** Stop Kill races reaper; successful Stop can become Failed.
- **BL-130/#217:** concurrency slot released before killed OS process actually exits.
- **BL-131/#219:** failed deadline Kill has no retry/escalation; successful Kill publishes timeout
  and releases slot before confirmed exit.
- **BL-132/#224:** portable CPU rate lacks unit/topology/range/rounding semantics.
- **BL-134/#230:** workload can escape delegated cgroup; failed tree termination can be discarded;
  Linux CPU mapping reflects unresolved BL-132 semantics.
- **BL-135/#234:** docs overclaim Linux adapter lifecycle/race evidence; tests use fake processes.

### CI owners
- **BL-252/#227:** reusable race script is groundwork only at its prepared base; rebase after
  stateful packages integrate and prove coverage.
- **BL-253/#247:** rebase after Process integration; Windows leg does not bind/verify required
  PowerShell 7.6.5.
- **BL-254/#251:** parent BL-098 incomplete; Windows leg lacks 7.6.5 binding; workflow duplicates
  race command instead of consuming BL-252 reusable gate.

No complete SPR-056 integration line is ready. No findings were fixed.
