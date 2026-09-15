# FlashGate MCP — Mobile / Codex Cloud Work Catalog

## 1. Purpose and authority

`MOBILE.md` is the active FlashGate Codex Cloud execution catalog for periods when the Windows development host is unavailable. It is derived execution guidance, not a second backlog or governance authority.

Authority order:

1. root `AGENTS.md`;
2. `Governance/CLOUD-CODEX-GOVERNANCE.md`;
3. `Governance/CHANGE-TRIGGER-REVIEW-AND-BACKLOG-STANDARD.md`;
4. `Governance/MOBILE-CLOUD-HANDOFF.md`;
5. current `BACKLOG.md`;
6. directly affected ADRs, architecture, security, testing and release documentation;
7. this file for Mobile eligibility, task selection, dependency execution and handoff.

If this file conflicts with a higher authority, current repository truth wins. Do not create BL IDs, change sprint assignment or milestone semantics, or mark a task `Done` from Mobile work.

```text
Mobile-Queue: FLASHGATE-MOBILE-V3
Publication: CODEX_UI_MANUAL_CREATE_PR
Merge-During-Mobile: NO
Windows-Finalization: REQUIRED
```

## 2. Active-catalog rule

This file intentionally lists only **new Mobile work that remains actionable without first resolving a known open Classic/owner decision**. A BL task is excluded from the active catalog when any of these is true:

- an open GitHub PR already reserves that BL identity;
- a Classic/owner product, architecture, security, platform, dependency, release, scope, governance or policy decision is known to be still open;
- the task directly depends on such an unresolved decision;
- it is `D — CONTINUOUS_OR_FINAL_GATE` or `X — NOT_AUTONOMOUS_MOBILE` work;
- it is `Later` work outside the current Version-1.0 Mobile execution queue.

Mode `C` is **not** excluded categorically. A bounded architecture/contract task remains Mobile-eligible when current canonical sources already provide enough authority to do useful work without inventing a new decision. If fresh inspection reaches a real decision boundary, stop with `BLOCKED_DECISION_REQUIRED`.

Exclusion from `MOBILE.md` does not change canonical `BACKLOG.md` status. GitHub open PRs remain the Vacation Reservation Ledger and override this snapshot immediately. A prepared/reserved PR remains excluded even when its review or validation still has findings; reservation prevents duplicate implementation but does not imply merge readiness or completion.

Refresh this catalog when a Mobile PR is created/merged/closed/retargeted, Classic records an unblock decision, or `main`, governance, backlog or dependency topology materially changes.

## 3. Cloud modes

| Mode | Meaning | Active Mobile treatment |
|---|---|---|
| `A — CLOUD_IMPLEMENTABLE` | Repository-contained code/docs/tests can be substantially implemented and validated in Cloud | Active when unreserved and dependency-executable |
| `B — CLOUD_IMPLEMENTABLE_PLATFORM_FINAL` | Meaningful Cloud implementation is possible; native Windows/Linux/service validation remains essential | Active when unreserved and dependency-executable; native finalization remains required |
| `C — CLOUD_ANALYSIS_OR_CONTRACT` | Cloud can perform bounded architecture, threat-model, design, investigation or contract work; a new owner/external decision may still become necessary | Active when the current contract allows bounded work without inventing a decision; stop at a new decision boundary |
| `D — CONTINUOUS_OR_FINAL_GATE` | Release/integration-wide gate | Not auto-selected standalone |
| `X — NOT_AUTONOMOUS_MOBILE` | Required authority/evidence is unavailable | Not auto-selected |

All active Mobile work retains `Windows-Finalization: REQUIRED`.

## 4. Reservation ledger

Before every task selection, enumerate all open PRs in `thomasweidner/flashgate-mcp` and resolve each selected BL identity according to `Governance/MOBILE-CLOUD-HANDOFF.md`.

A PR reserves a task when:

- its body has exact `Mobile-Task: BL-xxx`; or
- the selected BL is otherwise unambiguous from title/body under the provisional UI-generated rules.

Reservation is independent of commit SHA, branch name, mergeability, review result or tree identity. Content identity and review findings are later acceptance gates.

Known duplicate reservation collisions at this snapshot include:

`BL-036`, `BL-202`, `BL-203`, `BL-205`, `BL-206`, `BL-213`, `BL-216`, `BL-219`.

Do not create another candidate for a collision. Reconcile competing candidates during Windows/Classic finalization.

## 5. Automatic task selection

When the user says only “next task”:

1. read `AGENTS.md`, Mobile governance, current `BACKLOG.md`, this file and relevant technical contracts;
2. build the current Vacation Reservation Ledger and exclude all reserved BLs;
3. exclude every task with a known unresolved Classic/owner decision and every task directly blocked by one;
4. inspect `Planned` sprints in ascending order defined by root `AGENTS.md`;
5. classify serious candidates from the **current checkout** as:
   - `INDEPENDENT_FROM_CURRENT_CHECKOUT`;
   - `STACK_BASE_READY`;
   - `STACK_REQUIRED`;
   - `BLOCKED_DEPENDENCY_DECISION`; or
   - `BLOCKED_MULTIPLE_UNCOMBINED_MOBILE_PREDECESSORS`;
6. select from the earliest sprint with an executable unreserved candidate;
7. inside that sprint prefer mode A before B before C, independent before stack-base-ready, then lower effort, lower Windows residual and lower collision risk;
8. do not auto-select `Later` while executable unreserved `Planned` work exists.

The user may explicitly name a task, but naming a reserved or decision-blocked task does not make it executable.

## 6. Dependency and stack contract

Dependency hints are planning hints only. Real code/contracts determine ancestry.

### Independent

If the task does not consume an unintegrated predecessor:

```text
DependencyExecution=INDEPENDENT_FROM_CURRENT_CHECKOUT
ExpectedPRBase=main
Mobile-Depends-On: NONE
```

### Stack base ready

A dependent task is executable only when the current checkout already contains the exact predecessor it consumes. The normal route is a **new Codex Cloud task started directly from the predecessor PR head branch/SHA**.

Verify locally, when possible:

```text
git merge-base --is-ancestor <ParentHeadSha> HEAD
```

Then:

```text
DependencyExecution=STACK_BASE_READY
ExpectedPRBase=<ParentHeadBranch>
Mobile-Depends-On: <ParentBL>
```

### Stack required

If exactly one required predecessor is open but absent from the checkout, do not import it with fetch/pull/merge/cherry-pick/patch replay/downloaded Git objects/credential setup. Return:

```text
Status=STACK_RESTART_REQUIRED
TaskID=BL-xxx
DependsOn=BL-yyy
ParentPR=#n
ParentHeadBranch=<branch>
ParentHeadSha=<sha>
MutationCount=0
```

If multiple independent uncombined predecessors are genuinely required, return `BLOCKED_MULTIPLE_UNCOMBINED_MOBILE_PREDECESSORS`. Never synthesize a Cloud merge branch.

## 7. Snapshot binding — 2026-09-15

```text
Repository : thomasweidner/flashgate-mcp
Main       : 42cd2d9e12f2e812eeeaafb7c7221c4910a4bb9f
AGENTS     : 73b06e26da08847ebd3c5ee0512c581147eb4548
BACKLOG    : a02db7c09b10dba8c827f62618c7c6eb9f096591
MOBILE     : 1a85e8c5761eba9cc96086a17a1f26f8359e35db (pre-this-update)
Ledger     : open GitHub PRs through #178 at snapshot time
```

Since the preceding Mobile refresh, the following formerly active topics are now prepared/reserved and therefore removed from new Mobile selection:

`BL-049`, `BL-074–BL-080`, `BL-082`, `BL-171`, `BL-203`, `BL-205`, `BL-213`, `BL-216`, `BL-219`.

The complete current Search implementation queue represented by `BL-068–BL-080` and `BL-082` is now reserved by open PRs. No new Version-1.0 Search BL remains in the active Mobile list at this snapshot.

## 8. Classic/owner decision exclusions

The following Planned tasks are intentionally absent from the active queue until the required Classic/owner decision is recorded in canonical truth:

| IDs | Boundary |
|---|---|
| `BL-177–BL-179` | Governance authority, maintainer rules and contribution/DCO/CLA decisions |
| `BL-209` | Final MCP Tasks Extension compatibility decision; decision packet exists in PR #149 |
| `BL-210` | Mapping remains blocked until Tasks compatibility/fallback decisions are resolved |
| `BL-211` | MCP Tasks fallback decision; decision packet exists in PR #150 |
| `BL-328` | Strict-JSON resource ceilings require justified limit decisions |
| `BL-330` | Canonical `In Progress` status contract requires owner decision |

Decision-gated post-1.0 work is also excluded from the current queue, including `BL-083`, `BL-112`, `BL-127–BL-128`, `BL-150`, `BL-158`, `BL-169`, `BL-176`, `BL-181–BL-188`, `BL-217`, `BL-232`, `BL-240` and `BL-313`.

`BL-235` is not decision-blocked: accepted ADR-015 already selects the Version-1.0 hybrid per-root execution-identity architecture and Variant A while reserving Variant B. It therefore remains eligible Mode-C work while its backlog row is `Planned`.

## 9. Active Planned candidates

These IDs were unreserved at the snapshot, are `Planned`, and are not blocked by a known unresolved Classic/owner decision. Reclassify real dependencies before every mutation.

| Epic | Mode A | Mode B | Mode C |
|---|---|---|---|
| Filesystem | `BL-056–BL-057`, `BL-063–BL-064` | `BL-060–BL-061` | — |
| Operations / Job | `BL-095` | `BL-098–BL-099` | — |
| Named roots / capabilities | `BL-102–BL-111` | — | — |
| Process | `BL-114–BL-118`, `BL-120–BL-126`, `BL-130–BL-133` | `BL-134–BL-135` | — |
| Command execution | `BL-137–BL-145`, `BL-148–BL-149`, `BL-151` | `BL-146–BL-147`, `BL-152` | — |
| System information | `BL-154–BL-157` | — | — |
| Security | `BL-159–BL-161`, `BL-163–BL-164`, `BL-167` | `BL-168` | — |
| Native multi-mode / service | `BL-224`, `BL-228`, `BL-236`, `BL-239` | `BL-226–BL-227`, `BL-229–BL-231`, `BL-234`, `BL-237`, `BL-241–BL-242`, `BL-244` | `BL-223`, `BL-225`, `BL-233`, `BL-235`, `BL-238` |
| CI / release quality | `BL-252`, `BL-256`, `BL-258` | `BL-253–BL-254`, `BL-261–BL-262` | — |
| Cross-mode host lifecycle | — | `BL-341` | — |

Snapshot totals:

```text
Mode A active : 60
Mode B active : 25
Mode C active : 5
Total active  : 90
```

`Later`, completed, reserved, known decision-blocked, Mode-D and Mode-X rows are intentionally omitted.

## 10. Immediate launch topology

These are current **preclassifications** only. Fresh inspection always wins.

| Task | Mode | Snapshot state | Start ref / expected base | Parent |
|---|:---:|---|---|---|
| `BL-102` | A | `RECHECK_ON_PARENT` | PR #152 `codex/wahle-offenen-flashgate-mobile-task-aus` @ `5f99cf1493a6c5c7788e7d88b8e22ccdf95cd750` | likely `BL-101` |
| `BL-120` | A | `RECHECK_ON_PARENT` | PR #123 `codex/fuhre-nachsten-cloud-task-aus-o35p7y` @ `0289117a8d818721d3559d244819c333441c6d0d` | likely `BL-119` |
| `BL-137` | A | `STACK_RESTART_READY` | PR #107 `codex/fuhre-cloud-task-aus-mobile.md-v3-aus-ztndq7` @ `02bbdc19b9f7067b7d033e7cc55378f612c1c95f` | `BL-136` |
| `BL-154` | A | `STACK_RESTART_READY` | PR #135 `codex/fuhre-nachsten-geeigneten-cloud-task-aus-58655l` @ `1ebd868f5f20d37be48cdf5df5e0de1b7e470797` | `BL-062` |
| `BL-155` | A | `RECHECK_ON_PARENT` | PR #132 `codex/fuhre-nachsten-cloud-task-aus-myz7pk` @ `20e2d97a640879564a428b29726fa9c6646abf69` | likely `BL-153` |
| `BL-156` | A | `RECHECK_ON_PARENT` | PR #132, same head | likely `BL-153` |
| `BL-159` | A | `STACK_RESTART_READY` | PR #105 `codex/fuhre-nachsten-cloud-task-aus-mobile.md-v3-aus` @ `3ca7584e00690b8f09bccb8401e555bc9cf3985f` | `BL-100` |

Earlier-sprint work does not become executable merely because its BL/sprint number is lower. The remaining `SPR-049` and filesystem work is currently dominated by multiple independent predecessors or later native integration. If fresh dependency inspection confirms those blocks, `BL-102` is the first known clean candidate to inspect from the next actionable foundation.

### Workstream notes

- **Filesystem:** `BL-049` is now reserved by PR #169. `BL-056`, `BL-060–061`, `BL-063–064` currently intersect multiple open write/list/job foundations and default to `WAIT_MULTI`; `BL-057` normally follows `BL-056`.
- **Search:** all current Version-1.0 Search candidates are reserved. Do not start another Search BL from this catalog.
- **Named roots:** BL-101 is reserved by PR #152; BL-100 capability foundation is PR #105. `BL-102` is the cleanest next named-root child to recheck. Root/profile/capability crossing tasks may become `WAIT_MULTI`.
- **Operations/Job:** required primitives are distributed across independent open PRs; `BL-095`, `BL-098–099` default to `WAIT_MULTI` until current topology proves otherwise.
- **Process:** BL-113, BL-119, BL-129 and BL-162 foundations are independent. `BL-120/121` may follow BL-119 if it is the only real prerequisite; otherwise process work defaults to `WAIT_MULTI`.
- **Command:** BL-136 / PR #107 is the threat-model foundation; `BL-137` is the preferred first child. `BL-143` must reuse the Managed Process Engine.
- **System:** `BL-154` consumes BL-062; `BL-155/156` should be rebound from BL-153. `BL-157` crosses system-info and capability enforcement.
- **Security:** `BL-171` is now reserved by PR #178. `BL-159` consumes BL-100; `BL-160` normally follows it. Other active security rows frequently cross multiple domain foundations.
- **Multi-mode / service:** bounded Mode-C contract work remains selectable under the root A→B→C preference; do not invent a decision when current ADR/backlog authority is insufficient. `BL-235` has its architecture choice already fixed by ADR-015. Reclassify downstream A/B work from the selected checkout.
- **CI / release:** `BL-261` authoritative benchmark evidence and `BL-262` signing/atomic-release evidence remain deferred; repository-contained preparation stays active when dependency-executable.

## 11. Implementation and validation contract

For exactly one selected task:

- read the canonical BL row and directly affected technical/security/testing documentation;
- keep the task branch task-pure except directly caused in-scope corrections;
- prefer Go standard library and existing project patterns;
- add no dependency without separate approval;
- never corrupt MCP stdout with logs;
- preserve root/realpath/symlink/reparse/UNC, read-only, capability, caller/backend, principal, quota/fairness, resource-budget and safe-error boundaries;
- stop on a new product/architecture/security/platform/dependency/release/scope decision instead of inventing one.

Validation follows `DIRECTLY_AFFECTED_FIRST`: focused tests/static checks first, then the smallest sufficient consolidated gate. Use relevant `gofmt`, `go test`, race tests, `go vet`, build, schema/docs checks and `git diff --check`. PowerShell target is 7.6.5. Never claim unavailable Windows/WSL/SCM/systemd/ACL/native-host evidence.

Cloud work must not mark `BACKLOG.md` `Done` or claim local integration.

## 12. Manual Create-PR handoff

After implementation, validation and a task-pure commit, record:

```text
CloudCommitSha=<full SHA>
CloudTreeSha=<git rev-parse 'HEAD^{tree}'>
```

Prepare:

```text
ExpectedPRTitle: [MOBILE][BL-xxx] <concise task subject>

Mobile-Queue: FLASHGATE-MOBILE-V3
Mobile-Task: BL-xxx
Mobile-Mode: A|B|C
Mobile-State: CLOUD_IMPLEMENTATION_COMPLETE
Mobile-Depends-On: NONE|BL-xxx[,BL-yyy...]
Mobile-Base-Ref: <main-or-parent-mobile-branch>
Windows-Finalization: REQUIRED
Merge-Allowed: NO
```

Then stop with:

```text
Status=CLOUD_IMPLEMENTATION_COMPLETE_AWAITING_MANUAL_PR
TaskID=BL-xxx
DependencyExecution=INDEPENDENT_FROM_CURRENT_CHECKOUT|STACK_BASE_READY
ActualHeadBranch=<cloud branch>
CloudCommitSha=<sha>
CloudTreeSha=<tree sha>
ExpectedPRBase=<main-or-parent-branch>
ExpectedPRTitle=[MOBILE][BL-xxx] <subject>
WindowsFinalization=REQUIRED
NextAction=Use Codex UI Create PR, then reply "PR erstellt"
```

The user performs Codex UI **Create PR**. Do not substitute agent-driven GitHub writes, manual remotes or credentials.

After “PR erstellt”, enumerate open PRs read-only, establish reservation, compare Cloud/GitHub tree SHA, verify expected base and task-pure scope. Commit-SHA rematerialization is acceptable when tree SHA matches. Keep the BL reserved on visibility delay, ambiguity, tree mismatch or base mismatch; never auto-reimplement it.

## 13. Hard boundaries

Mobile task authorization does **not** authorize:

- merge or PR close;
- branch deletion or force update;
- tags/releases;
- repository settings/rules/secrets;
- manual Git remote, PAT/token/SSH setup;
- predecessor fetch/pull/merge/cherry-pick;
- dependency/tool installation without separate approval;
- synthetic combination of independent Mobile predecessors.

Every candidate requires Windows/local finalization before canonical completion.

## 14. Windows return

For every open Mobile PR on return:

1. bind current local governance, `AGENTS.md`, `BACKLOG.md`, `main` and toolchain;
2. process dependency roots before stacked children;
3. inspect complete diff/ancestry and perform independent review where required;
4. run real Windows/native Linux/PowerShell 7.6.5 validation;
5. correct directly caused findings;
6. integrate/retarget/merge/cleanup only under then-applicable Git/remote approvals;
7. update canonical backlog/status only from verified integration truth.

A Cloud PR is evidence of a candidate, never evidence that its BL is `Done`.

## 15. Minimal phone prompts

### Automatic

> Führe den nächsten geeigneten Cloud-Task aus `MOBILE.md` V3 aus.

### Named epic

> Wähle den nächsten geeigneten noch offenen Task aus dem Filesystem-Epic gemäß `MOBILE.md` V3 und führe genau diesen Mobile-Task aus.

Replace `Filesystem` with `Operations/Job`, `Named roots`, `Process`, `Command Execution`, `System Information`, `Security`, `Multi-Mode` or `CI/Release`.

### Named task

> Führe `BL-xxx` gemäß `MOBILE.md` V3 aus. Prüfe die DependencyExecution aus dem aktuellen Checkout; bei `STACK_REQUIRED` stoppe mit dem exakten Vorgänger-Ref für einen neuen Cloud-Task. Kein Fetch/Pull/Merge/Cherry-Pick des Vorgängers und kein Merge des späteren PRs.
