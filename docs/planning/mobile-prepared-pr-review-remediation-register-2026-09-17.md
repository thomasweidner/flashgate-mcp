# FlashGate MCP — Prepared PR Review Remediation Register

**Status:** READ_ONLY_VACATION_REMEDIATION_REGISTER  
**Date:** 2026-09-17  
**Repository:** `thomasweidner/flashgate-mcp`  
**Bound main:** `e75e2371c0d1ae7c587286e75189f86fc5d3a4c0`  
**Bound main tree:** `018fbca90efdfc27aaeb2deb44a9d6b479be8b66`  
**Open PR count:** `198`  
**LedgerPaginationComplete:** `true`  
**LedgerSnapshotStable:** `true`  
**MutationCount:** `0`
**AGENTS.md blob:** `079fad212fc4843eec70999cb551495b226087e6`  
**BACKLOG.md blob:** `a02db7c09b10dba8c827f62618c7c6eb9f096591`  
**Cloud governance blob:** `5a8295d61a0d13a624471fa3438c2e6157d9fb60`  
**Change-trigger adapter blob:** `07ac0d53520b70d71c8c96fd419d55134e890692`  
**Finding/review-mode standard blob:** `89622d7c04af0359b3dcc34e5ee5eeb637ced7f0`  
**Classic-readiness standard blob:** `4bdb44bb160b003969d5ccbc157d2415d9e4ed1e`  

This register is a read-only vacation artifact. It is not backlog authority and grants no mutation authority.

## Classic review authority

For this vacation analysis, GitHub/Codex review comments are **review signals only**. They are not governance-authoritative independent-review findings and they do not by themselves authorize correction.

When technical risk requires independent review, the binding review is performed in **Classic** as a separate read-only actor under the current governance:

- `INDEPENDENT_REVIEW`: read-only, no repository or external mutation, no correction;
- an implementing/correcting actor may self-validate but may not claim independent approval;
- only a Classic review disposition may promote a signal to a confirmed finding for remediation tracking;
- after a confirmed finding is corrected by the authorized correction actor, any required focused delta review is again Classic and read-only;
- Codex/Cloud comments may be used as evidence pointers or candidate hypotheses, but never as the required independent-review evidence.

On return to Windows/local work, rebind against the then-current central Slim Governance and applicable higher-level/local `AGENTS.md`; stricter/newer local authority wins.

## Vacation exit rule

When the user returns from vacation:

```text
MobileTaskSelection = OFF
PrimaryWorkSource    = BACKLOG.md
```

The Mobile waves remain useful integration/remediation evidence, but they stop being the work queue. Normal repository governance and the then-current `BACKLOG.md` determine priority. The current normal Planned sequence begins at SPR-048.

---

## Executive finding

The prepared-PR landscape is not blocked only by multi-PR topology. A material number of the prepared inputs have unresolved, non-outdated Codex review signals. These are candidate risks only until independently dispositioned in Classic.

Do not begin a Windows merge campaign by mechanically merging the oldest roots. First perform a fresh Classic independent-review pass on each PR reached by normal backlog priority.

### Codex review signals observed in the Wave 1–6 preflight — not yet Classic findings

| PR | BL | Codex signal severity/count | Candidate risk type |
|---:|---|---|---|
| #105 | BL-100 | 1×P1 | scope ownership / capability registration |
| #181 | BL-159 | none observed | downstream of #105 |
| #188 | BL-114 | 1×P2 | cursor strict decoding |
| #191 | BL-115 | none observed | sibling convergence |
| #192 | BL-116 | none observed | sibling convergence |
| #193 | BL-117 | 1×P2 | partial/topology semantics |
| #129 | BL-162 | 1×P1, 1×P2 | policy taxonomy + session binding |
| #132 | BL-153 | 2×P1 | Windows version API + hard budgets |
| #213 | BL-155 | 1×P2 | unsafe env-value release |
| #215 | BL-156 | 1×P2 | ineffective schema test |
| #211 | BL-154 | 1×P1, 1×P2 | Windows file path + ADR/profile conflict |
| #107 | BL-136 | 1×P1 | executable provenance security contract |
| #186 | BL-137 | 1×P2 | `--` terminator/catalog validation |
| #198 | BL-138 | 2×P1, 1×P2 | interpreter bypass, inline selectors, negative positional ints |
| #185 | BL-107 | 1×P2 | nil filesystem panic |
| #187 | BL-108 | 1×P1, 1×P2 | invalid ancestry + access/capability mismatch |
| #189 | BL-109 | none observed | downstream of invalid #187 ancestry |
| #195 | BL-223 | 1×P2 | preserve established `-h` CLI alias |
| #214 | BL-224 | 1×P2 | cancellation cannot unblock read loop / cleanup ordering |
| #210 | BL-225 | 2×P1, 1×P2 | incomplete wire schemas, cancellation result, frame read bounds |
| #81 | BL-094 | 1×P1, 2×P2 | bounded callbacks, completion race, stable report retrieval |
| #134 | BL-129 | 1×P1, 1×P2 | write-once ownership finalization + expiry binding |
| #255 | BL-227 | 1×P1, 1×P2 | atomic UDS endpoint replacement + publication permissions |
| #256 | BL-226 | 3×P1, 1×P2 | impersonation, DACL rights, namespace reservation, reserved flags |
| #205 | BL-233 | 2×P2 | legacy env compatibility + required per-user logs |
| #220 | BL-236 | 2×P2 | backend allowlist + budget in execution binding |
| #257 | BL-237 | 1×P1, 2×P2 | service-account ID bypass + audit profile + bounded correlation |

This table records only the PRs inspected during the current vacation preflight. It is not a claim that unlisted PRs are clean.

---

# Candidate remediation classes — apply only after Classic confirmation

## A. Invalid scope / wrong ancestry signals — Classic review first

### PR #105 / BL-100

The branch owns the capability vocabulary but also generalizes dynamic catalog registration that the Codex signal attributes to BL-110/BL-111.

**Candidate direction if Classic confirms:** retain the functional capability model; preserve only existing `MCP_READ_ONLY` behavior; do not consume general dynamic-profile/catalog ownership; then rebase/revalidate PR #181.

### PR #187 / BL-108

The branch was stacked on BL-107 only, but it consumes/duplicates the BL-100 capability foundation.

**Candidate direction if Classic confirms:** correct BL-100 first; combine corrected BL-100 with the named-root ancestry through BL-107; rebuild/rebase BL-108 on that ancestry; then rebase BL-109.

These are candidate topology corrections if Classic confirms the underlying signals; they are not yet authorized remediation.

---

# B. Local bounded code/test fixes

- **PR #188 / BL-114:** require cursor second decode to return exactly EOF; malformed trailing bytes must be rejected before native observation; add regression test.
- **PR #193 / BL-117:** preserve process topology across incomplete metadata, set `partial=true`, do not emit invalid empty-name nodes, and continue traversal through omitted nodes.
- **PR #213 / BL-155:** validate allowlisted environment **values**, not only keys; keep narrow key-specific formats/limits and never return arbitrary token/path/control data.
- **PR #215 / BL-156:** validate `system_info` representative results against the actual system-info output schema, not a nil/default filesystem schema.
- **PR #186 / BL-137:** reject command definitions where a fixed `--` terminator precedes generated flagged rules.
- **PR #198 / BL-138:** reject interpreter-backed program-text modes; normalize and reject prohibited inline selectors; prevent negative positional integers from being reinterpreted as options.
- **PR #185 / BL-107:** fail closed on nil filesystem before deriving path/link rules; no panic.
- **PR #195 / BL-223:** include the established public `-h` help alias in the normative CLI grammar and validation matrix.
- **PR #220 / BL-236:** registration must accept only supported Version-1 backend IDs; carry the immutable policy-selected resource budget through Decision -> ExecutionContext -> backend.
- **PR #257 / BL-237:** reserve `service-account` ID for the validating ServiceAccountBackend; add effective profile to audit events; bound caller-controlled correlation/audit fields before recording.

---

# C. Platform/native correctness fixes

- **PR #132 / BL-153:** replace manifest-sensitive `syscall.GetVersion` with a manifest-independent source such as `RtlGetVersion`, without introducing a new dependency; regenerate deterministic hard catalog/workflow budget fields coherently after the corrected catalog is final.
- **PR #211 / BL-154:** preserve the authoritative public contract around Windows `GetDiskFreeSpaceExW`; if files are valid inputs, query their validated containing directory; if directory-only, narrow schema/tests consistently.
- **PR #214 / BL-224:** parent/root cancellation must initiate cleanup and unblock/close transport intake even when a reader is stuck because STDIN/pipe remains open.
- **PR #81 / BL-094:** callbacks must be subject to configured shutdown phase bounds; completion must be rechecked at timer boundaries; completed report retrieval must win deterministically over a later canceled caller context.
- **PR #255 / BL-227:** stale Unix-socket check/remove must be atomically bound to the verified endpoint or serialized across competing starts; restrictive permissions must hold before publication.
- **PR #256 / BL-226:** high-security Windows-native signals cover shared-process impersonation, overly broad DACL rights, missing first-instance namespace reservation, and reserved/unnegotiated frame flags. Hosted Windows compilation alone is not sufficient evidence.

---

# D. Contract/security decisions or normative contract gaps

- **PR #129 / BL-162:** risk-class taxonomy may conflict with deferred ADR-009 ownership; session-scoped managed handles/work need immutable owning session/connection identity in addition to principal/profile/root and generation.
- **PR #107 / BL-136:** absolute executable paths and race-safe launch do not prove provenance if the executable or an ancestor is writable by a less-trusted principal; the threat model needs a fail-closed provenance control or an explicit security/architecture decision.
- **PR #210 / BL-225:** IPC contract signals concern closed schemas for every Version-1 envelope/payload/nested object, cancellation acknowledgment/result semantics, and a bounded post-handshake frame read/progress deadline.
- **PR #205 / BL-233:** new `FLASHGATE_*` runtime settings must not silently break established `MCP_*` configuration; BL-233 also needs the required bounded per-user log destination/lifecycle if launcher stderr may be unavailable.
- **PR #211 / BL-154:** accepted ADR-009 still describes the current restricted profile as exactly the existing three read tools; registration ownership must be resolved rather than leaving contradictory normative contracts.

---

# E. Lifecycle ownership contract correction candidates — Classic review first

### PR #134 / BL-129

Candidate contract direction if Classic confirms: pre-launch ownership reservation; process creation; atomic/write-once finalization of PID/start identity/verified handle/tree ownership before exposing the public handle; immutable resolved expiry/TTL bound into the ownership record.

---

# Wave 5 / 6 integration implication

The previously stored vacation plan correctly identifies the high-level dependencies but must not be read as "these PRs are ready to merge".

### Wave 5

All five inputs have Codex review signals pending Classic disposition:

```text
#195 BL-223  -> P2
#214 BL-224  -> P2
#210 BL-225  -> P1,P1,P2
#81  BL-094  -> P1,P2,P2
#134 BL-129  -> P1,P2
```

`BL-341` must not be reclassified from these current heads.

### Wave 6

All inspected prepared follow-on lines have Codex review signals pending Classic disposition:

```text
#255 BL-227 -> P1,P2
#256 BL-226 -> P1,P1,P1,P2
#205 BL-233 -> P2,P2
#220 BL-236 -> P2,P2
#257 BL-237 -> P1,P2,P2
```

In addition, #255/#256 depend on the currently incomplete BL-225 contract and #257 depends on BL-236. Correct parents before children only after Classic review and separate correction authorization.

---

# Recommended phone-only work from here

1. Finish the Review Remediation Register for the remaining prepared PRs outside Waves 1–6.
2. For every Codex P1/P2 signal, perform Classic independent disposition first, then classify any Classic-confirmed finding as bounded local fix, stale/outdated after later governance, parent-first fix, invalid ancestry/rebuild, explicit architecture/security/product decision, or native Windows/Linux evidence.
3. After Classic confirmation only, prepare exact correction instructions for the first normal-backlog PRs that will be encountered after vacation; execution remains a separate authorized correction assignment.
4. Keep the repository-versioned **Vacation Exit Rule / Classic Review Addendum** current as dated planning evidence; do not treat it as backlog authority.
5. On return, discard Mobile priority and rebind from normal `BACKLOG.md` starting with the then-current earliest relevant Planned work.

---

# Suggested normal-backlog transition check

When vacation ends, run read-only:

```text
MainSha=
MainTree=
WorkingTree=
CurrentGovernanceHashes=
BacklogHash=
EarliestPlannedSprint=
OpenPreparedPRCount=
ClassicConfirmedOpenFindingCount=
SupersededPRCount=
DecisionBlockedCount=
NativeFinalizationPendingCount=
MobileSelectionDisabled=true
MutationCount=0
```

Only then choose the next normal backlog assignment.
