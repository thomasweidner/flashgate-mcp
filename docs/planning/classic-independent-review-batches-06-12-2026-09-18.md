# FlashGate MCP — Classic Independent Review Batches 06–12

**Status:** PASS_READ_ONLY_REVIEW_CONSOLIDATED  
**Date:** 2026-09-18  
**Repository:** `thomasweidner/flashgate-mcp`  
**Bound main:** `f615d6f54c967b1fae04c5378f32af0b648b40f7`  
**Bound main tree:** `477fb951fa9ac5b27e051a8d320ca440e473eb9d`  
**BACKLOG.md blob:** `a02db7c09b10dba8c827f62618c7c6eb9f096591`  
**Open PR count:** `198`  
**LedgerPaginationComplete:** `true`  
**LedgerSnapshotStable:** `true`  
**ReviewMode:** `INDEPENDENT_REVIEW`  
**RepositoryMutationCountDuringReviews:** `0`  
**FindingFixesPerformed:** `false`

This consolidated artifact preserves the read-only Classic review results produced after
`classic-independent-review-batch-05-2026-09-17.md`. It is review evidence, not backlog authority
and not correction/merge authority.

## Vacation exit rule

When vacation ends:

```text
MobileTaskSelection=OFF
PrimaryWorkSource=BACKLOG.md
```

The Mobile-prepared PRs remain useful implementation evidence, but the normal current backlog and
governance determine priority.

# Batch 06 — SPR-048 BL-202 through BL-207

## BL-202 — MCP tool annotations

Preferred candidate: PR #158. PR #52 is superseded.

Confirmed on #158:
- overwrite-capable `write_file` and `copy_path` are incorrectly marked `idempotentHint=true`;
- false-valued annotation members are not presence-checked by the static catalog parity test;
- current documentation is contradictory about whether annotations are implemented or planned.

Result: `FAIL_REMEDIATION_REQUIRED`.

## BL-203 — normalized machine-readable errors

PR #56 is main-based but turns unavailable/read-only-gated tools into MCP `CallToolResult` errors,
conflicting with the active JSON-RPC invalid-params boundary.

PR #167 preserves the better semantic split—unavailable/malformed calls remain JSON-RPC errors and
execution failures become MCP `isError` results—but is built on unrelated BL-073 Search ancestry.

Result: `REBUILD_REQUIRED` using the narrow #167 semantics on a clean normal-backlog base.

## BL-204 — MCP conformance evaluation/schema snapshots

PR #65: `CLEAN_CANDIDATE_FOR_LATER_FINALIZATION`.

The earlier snapshot concern is corrected: snapshot construction now uses the production
`createToolRegistry(...)`, binding real membership/order and complete input/output schemas.

## BL-205 — response-size regression tests

PR #55 is the preferred main-based candidate but must measure complete framed JSONL bytes without
trimming and use tight deterministic limits.

PR #165 is noncanonical because it is stacked on Search ancestry.

## BL-206 — local deterministic work principle

PR #161: `CLEAN_CANDIDATE_FOR_LATER_FINALIZATION`.

The statements are explicitly framed as a Version-1 planning baseline, not current implementation
claims. PR #54 is superseded.

## BL-207 — Version-1 MCP protocol matrix

PR #60: `CLEAN_CANDIDATE_FOR_LATER_FINALIZATION`.

The matrix matches current runtime behavior: protocol `2025-11-25`, STDIO only, no advertised
extensions, valid foreign proposals answered with the supported server revision, malformed/missing
versions rejected.

# Batch 07 — SPR-048 BL-208 through BL-214

## BL-208
PR #69 preferred but P1: cache context omits caller/root/backend/service/auth-generation dimensions.
Either bind them or prove identity-independent caching with mandatory policy revalidation.

## BL-209
PR #149 decision packet needs correction:
- circular prerequisite with BL-210 lifecycle evidence;
- defer-beyond-1.0 option conflicts with unconditional permanent Tasks gates.

## BL-210
No dedicated open PR found. Decision-dependent on corrected BL-209.

## BL-211
PR #150 is a clean decision packet but blocked on BL-209. It defines bounded synchronous,
explicit-capability-error, or class-specific fallback choices without selecting one.

## BL-212
PR #71 preferred main-based:
- stale hard tools/list budgets;
- manually maintained schema inventory instead of production registry.

PR #216 is noncanonical because it is stacked on later BL-153 System Information.

## BL-213
PR #58 preferred main-based but:
- prematurely makes lifecycle fallback/operation handles normative before BL-209–211;
- metadata useful-byte definition conflicts with benchmark contract.

PR #168 is noncanonical Search ancestry and has additional binding/fallback-order issues.

## BL-214
PR #63 preferred main-based but Classic independently confirmed a P2: serialization-copy count is a
constant (`2`), not measured from the real pipeline, so an added serialization cannot trip the gate.

# Batch 08 — SPR-048 BL-215/216/218/219/220

## BL-215
PR #95:
- P1 consumes BL-256 enforcement scope although BL-215 owns budget definition;
- P2 ceilings are not reconciled with accepted token goals;
- duplicate JSON keys are not detected by its strict loader.

## BL-216
PR #57 preferred; minor P2: instruction text should say batch only when supported/available.
PR #175 is noncanonical Search ancestry.

## BL-218
PR #61 preferred but requires substantial correction:
- P1 descriptor creation materializes entire payload in `[]byte`, defeating streaming;
- P2 MIME parameters can leak path-like values;
- P2 expiry is checked before binding mismatch, exposing lifecycle state;
- P2 claimed fixed-work binding comparison short-circuits.

PR #235 is noncanonical Managed-Process ancestry and has additional monotonic-TTL/scaling issues.

## BL-219
PR #59 is main-based but contractually incomplete/circular.
PR #177 has stronger semantics—domain-separated SHA-256, production registry definitions, explicit
protocol/extensions/profile/capabilities/risk-policy/schema/config generation and no current wire
publication—but is built on Search ancestry.

Result: rebuild the useful #177 semantics on a clean base after corrected BL-208 context rules.

## BL-220
PR #62 preferred but:
- full forbidden-interpreter list is not pinned;
- source admission is not bound to concrete executables/invocations.

# Batch 09 — SPR-049 Operations/Job

## Clean candidates

- PR #76 / BL-085 — opaque 192-bit random handles with server-side principal/root/profile/backend/service-generation binding.
- PR #70 / BL-086 — internal operation status model.
- PR #77 / BL-087 — operation-scoped cooperative cancellation.
- PR #82 / BL-090 — bounded identity-bound result storage/TTL.
- PR #84 / BL-093 — global/per-principal bounded principal-fair scheduler.

## Bounded remediation

- PR #64 / BL-084 — registration duplicate error can probe candidate IDs; narrow anti-probing claim or bind registration to generated/namespaced IDs.
- PR #78 / BL-088 — parent cancellation can later be misclassified as timeout.
- PR #74 / BL-089 — stale absolute progress updates can move counters backwards.
- PR #80 / BL-091 — path-bearing temporary IDs leak and callback panic can strand cleanup state.
- PR #72 / BL-096 — permanent domain-boundary test guesses future package paths.
- PR #68 / BL-097 — implementation conflicts with ADR-010's still-deferred selector decision.

## Integration/rebuild

- PR #79 / BL-092 — limiter not wired into production operation admission.
- PR #81 / BL-094 — reuse Batch-04 shutdown findings.
- PR #238 / BL-095 — inherits BL-094 and has stale-cleanup ABA/reused-ID deletion race.
- PR #241 / BL-098 — tests lifecycle behavior in helpers rather than production wiring; rebuild after primitives integrate.
- BL-099 — #242 incomplete; #243 contains a useful BL-095 ABA fix but still does not cover all required handles/limits.
- PR #240 / BL-164 — rebuild after integrated handles/limits/queue/deadline/cleanup foundations.

# Batch 10 — SPR-050 Filesystem BL-036 through BL-049

## Clean candidates

- PR #83 / BL-041 — bounded one-based line-window reads with separate selected-byte/scan limits.
- PR #103 / BL-044 — open-handle bounded read using `io.LimitReader(max+1)` and same-handle stat.

## Remediation/parent constraints

- PR #156 / BL-036 — Windows path-leak assertion compares unescaped path against escaped JSON and can false-pass.
- PR #75 / BL-038 — append-only process-output cursor strategy missing; wrong-owner versus invalid cursor leaks existence.
- PR #153 / BL-037 — full directory rescanned per page; wall-clock cursor TTL.
- PR #96 / BL-039 — filtering happens after unfiltered list limit; nested null filter fields treated as omitted.
- PR #87 / BL-040 — stale composite hard budget and severe per-entry allocation regression.
- PR #85 / BL-042 — byte ranges corrupt invalid UTF-8/binary and mishandle readable size-zero virtual files.
- PR #86 / BL-043 — affected workflow budgets not recalibrated.
- PR #91 / BL-045 — binary raw cap ignores base64 + duplicated MCP envelope.
- PR #89 / BL-046 — historical baseline provenance was synthetically rewritten; batch cap ignores envelope; branch predates bounded BL-044 reader.
- PR #88 / BL-047 — exact-count budgets conflict with versioned 3/8 baselines.
- PR #92 / BL-048 — public docs contradict tool/profile/limit state.
- PR #169 / BL-049 — parent-blocked by BL-038/037 and has wide-directory, depth-zero-validation, truncation-fingerprint and page-ceiling defects.

# Batch 11 — SPR-051 Filesystem targeted/long work

No fully clean prepared PR remains.

Key confirmed issues:
- BL-050/#90 — TOCTOU after stat can panic/range-overrun.
- BL-051/#159 — schema branches do not mutually exclude edit modes; parent BL-050 blocked.
- BL-052/#94 — atomic replace loses existing Unix permissions.
- BL-053/#151 — no-overwrite race lacks authoritative O_EXCL; unbounded SHA precondition; O_RDWR breaks writable-unreadable overwrite.
- BL-054/#100 — public dryRun contract missing Changelog entry.
- BL-055/#97 — hard tools/list budgets stale after append tool.
- BL-056/#231 — plan path/policy “prevalidation” is not whole-plan.
- BL-057/#232 — copy budget uses stale pre-Stat rather than streamed bytes; parent blocked.
- BL-058/#99 — published target not rebound/revalidated before destructive source delete.
- BL-059/#98 — machine error encoding preempts BL-203 and append semantics are duplicated/ambiguous.
- BL-060/#250 — directory-copy path-swap/root-escape race, preflight-only byte cap, read-only directory modes applied before population.
- BL-061/#237 — `WalkDir` is unbounded for very wide dirs and symlink scan-root semantics are wrong.
- BL-063/#233 — explicit empty mode treated as omitted; schema permits mode+overwrite although runtime rejects.
- BL-064 — no dedicated prepared PR; later integration point with Operations/Job.
- BL-065/#93 — active-step budgets/partial-effect truth/skip outcome/disconnect semantics incomplete.
- BL-066/#102 — Windows path-leak test can false-pass on JSON escaping.
- BL-067/#101 — corpus path traversal, incorrect deep-tree recipe and missing deterministic payload recipes.

# Batch 12 — SPR-052 Search

No clean final Search head exists. The complete stack is parent-blocked from BL-068.

## Root BL-068 / PR #104
- P1 pre-open path validation leaves link/reparse TOCTOU escape;
- P1 cursor binding omits backend and service generation;
- P2 final search capability name is prematurely fixed.

## Stack findings
- BL-069/#154 — allowed directory symlinks not traversed; POSIX backslashes rewritten.
- BL-070/#155 — no independent visited-entry bound; oversized patterns; invalid UTF-8 selector replacement.
- BL-071/#157 — RFC3339 schema/runtime mismatch.
- BL-072/#160 — unbounded/nonregular reads, unbounded queued directories, stale metadata around read.
- BL-073/#162 — regex size/compile budget missing.
- BL-074/#164 — absolute/dot-segment patterns accepted; byte-limit/schema maxLength mismatch; stale payload docs.
- BL-075/#166 — entry budget checked after eager directory listing.
- BL-076/#170 — truncation before logical ordering.
- BL-077/#171 — newline-ending match returns excess context; contradictory testing docs.
- BL-078/#172 — combined response budget bug, binaryMode/context ordering bug, invalid-UTF8 regex semantics, stale docs.
- BL-079/#173 — cached pages not rebound to policy/file state, cursor overhead after size check, no independent TTL reclamation.
- BL-080/#174 — ignored start directory traversed; caret-negated class semantics wrong.
- BL-082/#176 — no-external-deps gate misses transitive internal imports and false-passes under `-trimpath`.

Required remediation topology:

```text
BL-068 #104
 -> BL-069 #154
 -> BL-070 #155
 -> BL-071 #157
 -> BL-072 #160
 -> BL-073 #162
 -> BL-074 #164
 -> BL-075 #166
 -> BL-076 #170
 -> BL-077 #171
 -> BL-078 #172
 -> BL-079 #173
 -> BL-080 #174
 -> BL-082 #176
```

# Overall normal-backlog readiness through SPR-052

Clean prepared candidates found so far:

```text
SPR-048:
  BL-204 #65
  BL-206 #161
  BL-207 #60

SPR-049:
  BL-085 #76
  BL-086 #70
  BL-087 #77
  BL-090 #82
  BL-093 #84

SPR-050:
  BL-041 #83
  BL-044 #103

SPR-051:
  none

SPR-052:
  none
```

“Clean candidate” means no in-scope defect was found in this read-only review of that prepared delta.
It is **not** merge authority and does not replace fresh repository/governance binding, native
Windows/Linux evidence, conflict reconciliation or required focused gates.

# Continuation boundary

No finding was corrected by these reviews.

Next normal-backlog review target after this consolidated artifact: **SPR-053**.

The separate source snapshots for Batches 06–12 were retained externally with SHA-256 digests during
the review session. This consolidated repository artifact is the durable project-visible summary.
