# FlashGate MCP — Classic Independent Review Batches 21–27

**Status:** PASS_READ_ONLY_REVIEW_CONSOLIDATED  
**Date:** 2026-09-18  
**Repository:** `thomasweidner/flashgate-mcp`  
**Bound main:** `f615d6f54c967b1fae04c5378f32af0b648b40f7`  
**Bound main tree:** `477fb951fa9ac5b27e051a8d320ca440e473eb9d`  
**BACKLOG.md blob:** `a02db7c09b10dba8c827f62618c7c6eb9f096591`  
**Open PR count at persistence preflight:** `200`  
**LedgerPaginationComplete:** `true`  
**LedgerSnapshotStable:** `true`  
**ReviewMode:** `INDEPENDENT_REVIEW`  
**FindingFixesPerformed:** `false`

This artifact closes the read-only Classic review of the remaining SPR-061 groups. It is review
evidence only: no canonical backlog status changes, corrections, merges, branch deletions or release
actions are authorized by this file.

## Vacation exit

After vacation:

```text
MobileTaskSelection=OFF
PrimaryWorkSource=BACKLOG.md
```

The then-current normal backlog/governance controls work selection.

# Batch 21 — BL-172/173/177–179

- **BL-172/#125:** P1 — workflow pinning gate scans lines and recognizes only trimmed lines that
  begin exactly with `uses:`; valid YAML formatting such as spaced/quoted keys or flow mappings can
  carry floating actions while the gate remains green. Parse workflow YAML structurally.
- **BL-173/#115:** P2 — README links to SECURITY.md but Windows/Linux release archives package only
  LICENSE, README, THIRD-PARTY-NOTICES and the binary. Include/link the security policy durably and
  update exact release inventories.
- **BL-177/178/179:** no dedicated open PRs; governance/maintainer/DCO-CLA decisions remain
  decision-gated.

# Batch 22 — BL-241–251

- **BL-241:** no PR; integrated multi-client/lifecycle/security matrix.
- **BL-242:** no PR; all-mode Windows/Linux CI/release validation.
- **BL-243/#142:** two P2 documentation errors: exit code 3 has more configuration causes than root
  config; exit code 1 can occur before logger creation so MCP_DEBUG cannot diagnose every case.
- **BL-244:** no PR; direct/proxy/service benchmark gate after runnable integrated modes coexist.
- **BL-245/#141:** P2 — workflow permission test checks only top-level `contents: read`; job-level
  permissions can escalate while the gate stays green.
- **BL-246/247/248:** canonical Done; reuse evidence.
- **BL-249/#116:** two P2 — Windows benchmark job does not provision/verify PowerShell 7.6.5;
  contamination/window warning is not preserved in the uploaded JSON artifact.
- **BL-250/#145:** P2 — `go test -run` succeeds with “[no tests to run]” if the named test is
  removed/renamed, silently disabling the permanent baseline-comparison gate.
- **BL-251:** canonical Done.

# Batch 23 — BL-255–263

- **BL-255:** prefer #140 over older #117. #140 P2: expected release names are hard-coded and not
  bound to the actual four workflow matrix rows.
- **BL-256/#229:** P2 — initialize budget uses hard-coded server identity `budget-test` rather than
  production name/version; also depends on corrected BL-215 budget definitions/catalog path.
- **BL-257/#146:** new Classic P2 — exact `go test -run` selector can disappear and still pass;
  rebase on corrected BL-204/#65 production-registry snapshot instead of older manual inventory.
- **BL-258/#225:** same `go test -run` false-pass; must follow corrected BL-205/213/214/218 payload
  contracts.
- **BL-259/#119:** repository gate scans ignored/untracked local files and does not inspect tracked
  path names for legacy identifiers.
- **BL-260:** no standalone PR; continuous final standard-gate validation.
- **BL-261:** no PR; cross-project benchmark remains decision-gated on pinned servers/host/window.
- **BL-262/#218:** P1 SBOM uses go.sum hashes while release builds with `-mod=vendor`, so patched
  vendor bytes can enter the binary while upstream checksums remain advertised; P2 source/commit
  timestamp is incorrectly published as SLSA `startedOn`.
- **BL-263:** no PR; final Version-1 release gate.

# Batch 24 — BL-305–312

- **BL-305/306:** continuous per-sprint CHANGELOG/BACKLOG gates, no standalone PR.
- **BL-307/#110:** clean documentation candidate; current technical identities, planned identifiers
  and historical names are separated correctly.
- **BL-308/#121:** P2 — ADR horizon `Historical compatibility` is used but undefined in the index
  legend.
- **BL-309/#120:** three P1 overclaims: reduced output not yet hard-gated (BL-317), benchmark corpus
  placement not yet enforced (BL-318), and provenance currently validates declared metadata rather
  than verifiable measured-input identities (BL-319).
- **BL-310/#122:** two P2 — service catalog/capabilities derived before caller authentication in the
  documented flow; state binding omits groups and protocol context.
- **BL-311/#124:** new Classic P2 — documented Operations ownership tuple omits canonical BL-239
  groups and negotiated protocol-context binding.
- **BL-312/#126:** P2 — audit redaction is described as applying “by default”, implying an
  unapproved escape hatch; secret redaction before persistence must be unconditional absent a
  separate decision.

# Batch 25 — BL-314–320

- **BL-314/#118:** P2 — Windows smoke instructions remove rather than restore an existing
  MCP_READ_ONLY setting, and can leave later invocations unexpectedly write-capable.
- **BL-315:** continuous docs review gate.
- **BL-316:** canonical Done.
- **BL-317/#108:** P1 — useful-output validation trusts response-reported read `size` rather than
  actual content bytes; truncated content can pass.
- **BL-318/#133:** P1 validated corpus parent is never consumed by benchmark corpus creation;
  P2 Linux tests try to create temp dirs directly under non-writable `/home`.
- **BL-319/#143:** three P1 — provenance not wired into recording/admission; several hashes only
  syntax-checked; required 180-second quiet interval not enforced.
- **BL-320/#131:** P1 — Windows policy/window scripts receive no WorkingPath/FLASHGATE_WORK_ROOT and
  fail before assertions.

# Batch 26 — BL-321–333

Clean candidates:
- **BL-321/#139:** reads kernel AT_CLKTCK from `/proc/self/auxv`, supports non-100 values and
  explicit failure.
- **BL-323/#73:** benchmark docs match executable inventory and mark copy/search planned.
- **BL-326/#109:** strict JSON prevalidation rejects invalid raw UTF-8 and unpaired surrogates while
  accepting legitimate U+FFFD/valid pairs.
- **BL-329/#137:** binds fixed baseline filename to embedded OS/arch and tests real committed
  artifact swaps; supersedes weaker #111.

Requires correction:
- **BL-325/#113:** Draft-2020 integer wire encodings and Go int decoding still diverge; nested
  schema drift tests do not pin exact constraints.
- **BL-327/#112:** unknown measurement diagnostics retain input order rather than deterministic
  lexical order.
- **BL-331/#50:** metadata-regression docs incorrectly claim archive-content validation belonging
  to release-build.
- **BL-332/#130:** tests do not exercise the actual PowerShell→WSL marker transport and binary/
  archive leak-failure boundary.

Not prepared:
- **BL-322** status-document reconciliation.
- **BL-328** strict-JSON resource ceilings.
- **BL-330** canonical In-Progress status decision.

Done/reused:
- **BL-324**, **BL-333**.

# Batch 27 — BL-334–340 and BL-342–344

All are canonical Done in current BACKLOG.md and have no remaining open task work:

- BL-334 enforcement governance
- BL-335 reference-bound legacy Temp migration
- BL-336 generic handoff/commit-preparation governance
- BL-337 superseded fixture-runner disposition
- BL-338 canonical governance case metadata
- BL-339 focused/full governance validation orchestration
- BL-340 generator/profile migration
- BL-342 task-bound validation scratch producers
- BL-343 Slim Governance adapter convergence
- BL-344 Slim Governance poststate/local baseline integration

Reuse their durable evidence and do not reopen absent a new scope-triggered regression.

# Milestone

With these batches, the current normal Version-1 Planned sprint sequence **SPR-048 through SPR-061**
has been independently reviewed read-only against the prepared PR landscape.

No findings were corrected during review.

Next high-value read-only activity is to derive a **post-vacation normal-backlog remediation and
integration order** from the confirmed findings, dependency topology and clean reusable prepared
deltas. That order must be rebound against then-current repository truth before execution.
