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

This file intentionally lists only **new Mobile work that remains Cloud-preparable without first resolving a known open Classic/owner decision**. "Cloud-preparable" means the task-pure delta can be implemented and meaningfully validated in Cloud; it does **not** mean all later integration/native evidence already exists or that the BL can be marked `Done`.

A BL task is excluded from the active catalog when any of these is true:

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

`BL-036`, `BL-202`, `BL-203`, `BL-205`, `BL-206`, `BL-212`, `BL-213`, `BL-214`, `BL-216`, `BL-218`, `BL-219`.

Do not create another candidate for a collision. Reconcile competing candidates during Windows/Classic finalization.

## 5. Automatic task selection

When the user says only “next task”:

1. read `AGENTS.md`, Mobile governance, current `BACKLOG.md`, this file and relevant technical contracts;
2. build the current Vacation Reservation Ledger and exclude all reserved BLs;
3. exclude every task with a known unresolved Classic/owner decision and every task directly blocked by one;
4. inspect `Planned` sprints in ascending order defined by root `AGENTS.md`;
5. for each serious candidate, run the **concrete-delta feasibility test** from `Governance/MOBILE-CLOUD-HANDOFF.md` before assigning a dependency state;
6. classify candidates from the **current checkout** as:
   - `INDEPENDENT_FROM_CURRENT_CHECKOUT`;
   - `STACK_BASE_READY`;
   - `STACK_REQUIRED`;
   - `BLOCKED_DEPENDENCY_DECISION`; or
   - `BLOCKED_MULTIPLE_UNCOMBINED_MOBILE_PREDECESSORS`;
7. select from the earliest sprint with an executable unreserved candidate;
8. inside that sprint prefer mode A before B before C, independent before stack-base-ready, then lower effort, lower Windows residual and lower collision risk;
9. a candidate-local `BLOCKED_MULTIPLE_UNCOMBINED_MOBILE_PREDECESSORS` does not end the scan; continue with other candidates and later Planned sprints;
10. return a global no-executable Planned result only after the complete allowed Planned scan, subject to the single-predecessor `STACK_RESTART_REQUIRED` rule;
11. do not auto-select `Later` while executable unreserved `Planned` work exists.

The user may explicitly name a task, but naming a reserved or decision-blocked task does not make it executable.

## 6. Dependency and stack contract

Dependency hints are planning hints only. Real code/contracts determine ancestry.

### Concrete-delta feasibility test

Before treating an open PR as a hard predecessor, identify the exact task-pure delta owned by the candidate. A predecessor is hard only when that delta actually consumes missing code, types, schemas, contracts or runtime behavior.

Do **not** create a Stack/Block dependency merely because the candidate mentions future consumers, future packages, later CI/end-to-end coverage, later integration validation, Windows/native finalization or release evidence. A generic repository-wide gate may be prepared independently when its own implementation and focused validation are correct now and it will automatically cover future packages after integration.

If the candidate's own delta is complete now but some acceptance evidence is necessarily deferred, record that evidence as deferred. Deferred evidence does not make the candidate blocked and does not permit claiming the BL `Done`.

### Independent

If the candidate's concrete task-pure delta does not consume an unintegrated predecessor:

```text
DependencyExecution=INDEPENDENT_FROM_CURRENT_CHECKOUT
ExpectedPRBase=main
Mobile-Depends-On: NONE
```

This includes generic tests/CI gates whose future coverage grows automatically when later packages integrate.

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

If exactly one genuinely required predecessor is open but absent from the checkout, do not import it with fetch/pull/merge/cherry-pick/patch replay/downloaded Git objects/credential setup. Return:

```text
Status=STACK_RESTART_REQUIRED
TaskID=BL-xxx
DependsOn=BL-yyy
ParentPR=#n
ParentHeadBranch=<branch>
ParentHeadSha=<sha>
MutationCount=0
```

If the concrete delta genuinely requires multiple independent uncombined predecessors, classify that candidate as `BLOCKED_MULTIPLE_UNCOMBINED_MOBILE_PREDECESSORS`, exclude it, and continue scanning. Never synthesize a Cloud merge branch.

## 7. Snapshot binding — 2026-09-17

```text
Repository : thomasweidner/flashgate-mcp
Main       : c761fed9a1543467f37208bc3352e564d61f193c
AGENTS     : 0e686cbbd305204cb077771bd0f88a1a15d164d2
BACKLOG    : a02db7c09b10dba8c827f62618c7c6eb9f096591
MOBILE     : 21200e7f851d06df81862ef192d25050b337bc47 (pre-this-update)
Ledger     : open GitHub PRs through #238 at snapshot time
```

Since the preceding Mobile refresh, these formerly active topics are now prepared/reserved and are removed from new Mobile selection:

`BL-056`, `BL-057`, `BL-061`, `BL-063`, `BL-095`, `BL-134`, `BL-135`, `BL-142`, `BL-256`.

Reservation evidence for this refresh:

- PR #231 prepares `BL-056` from `main`, and PR #232 prepares `BL-057` as its stack child.
- PR #233 prepares the `BL-063` safe-write-mode delta. Its UI-generated title/body omit the BL marker, but the selected task is unambiguous from the explicit `write_file` create-only/replace-only/upsert scope; record this reservation as `PROVISIONAL_UI_GENERATED`.
- PR #237 prepares `BL-061` directly from `main`.
- PR #238 prepares `BL-095` on the cumulative Operations/Job chain through controlled shutdown.
- PR #230 prepares `BL-134` from the managed-process chain through `BL-133`, and PR #234 prepares `BL-135` as its child.
- PR #236 prepares `BL-142` from the cumulative command chain through `BL-141`.
- PR #229 prepares `BL-256` directly from `main`.
- PR #235 creates an additional open reservation for already-reserved `BL-218`; it is therefore added to the duplicate-reservation collision set and does not change the active count.

The complete current Search implementation queue represented by `BL-068–BL-080` and `BL-082` remains reserved by open PRs. No new Version-1.0 Search BL remains in the active Mobile list at this snapshot.

The filesystem plan/safe-mode work now leaves only `BL-060` and `BL-064` active in that epic. The process-observation chain still leaves `BL-118` unreserved, while the managed-process implementation chain through `BL-135` is now reserved. The command result foundation through `BL-142` is reserved, so later command/runtime tasks must be classified from their concrete deltas against the current open stacks.

## 8. Classic/owner decision exclusions

The following Planned tasks are intentionally absent from the active queue until the required Classic/owner decision is recorded in canonical truth:

| IDs | Boundary |
|---|---|
| `BL-177–BL-179` | Governance authority, maintainer rules and contribution/DCO/CLA decisions |
| `BL-209` | Final MCP Tasks Extension compatibility decision; decision packet exists in PR #149 |
| `BL-210` | Mapping remains blocked until Tasks compatibility/fallback decisions are resolved |
| `BL-211` | MCP Tasks fallback decision; decision packet exists in PR #150 |
| `BL-261` | Cross-project benchmark requires canonical selection/pinning of the native Rust and Go comparison filesystem servers; no such selection is recorded in current repository truth |
| `BL-328` | Strict-JSON resource ceilings require justified limit decisions |
| `BL-330` | Canonical `In Progress` status contract requires owner decision |

Decision-gated post-1.0 work is also excluded from the current queue, including `BL-083`, `BL-112`, `BL-127–BL-128`, `BL-150`, `BL-158`, `BL-169`, `BL-176`, `BL-181–BL-188`, `BL-217`, `BL-232`, `BL-240` and `BL-313`.

There are no active Mode-C rows at this snapshot because the four previously active bounded-contract tasks (`BL-225`, `BL-233`, `BL-235`, `BL-238`) are reserved by open PRs. This does **not** change the A→B→C selection contract or make Mode C categorically ineligible.

## 9. Active Planned candidates

These IDs were unreserved at the snapshot, are `Planned`, and are not blocked by a known unresolved Classic/owner decision. Re-run concrete-delta feasibility and real dependency classification before every mutation.

| Epic | Mode A | Mode B | Mode C |
|---|---|---|---|
| Filesystem | `BL-064` | `BL-060` | — |
| Operations / Job | — | `BL-098–BL-099` | — |
| Named roots / capabilities | `BL-103` | — | — |
| Process | `BL-118` | — | — |
| Command execution | `BL-139`, `BL-143–BL-145`, `BL-148–BL-149`, `BL-151` | `BL-146–BL-147`, `BL-152` | — |
| System information | `BL-157` | — | — |
| Security | `BL-160`, `BL-163–BL-164`, `BL-167` | `BL-168` | — |
| Native multi-mode / service | `BL-228` | `BL-226–BL-227`, `BL-229–BL-231`, `BL-234`, `BL-237`, `BL-241–BL-242`, `BL-244` | — |
| CI / release quality | — | `BL-253–BL-254` | — |
| Cross-mode host lifecycle | — | `BL-341` | — |

Snapshot totals:

```text
Mode A active : 16
Mode B active : 20
Mode C active : 0
Total active  : 36
```

`Later`, completed, reserved, known decision-blocked, Mode-D and Mode-X rows are intentionally omitted.

## 10. Immediate launch topology

These are current **preclassifications** only. Fresh inspection always wins.

The feasibility rule distinguishes a hard code/contract dependency from merely deferred coverage. The newly prepared PRs remove the previously advertised `BL-134`, `BL-142` and `BL-256` launch paths; remaining candidates must be rebound from their concrete deltas.

| Task | Mode | Snapshot state | Start ref / reason |
|---|:---:|---|---|
| `BL-060` | B | `RECHECK_FEASIBILITY` | Determine whether the bounded directory copy/move delta can be prepared against current filesystem contracts without consuming the still-unintegrated Job Manager; mentions of jobs alone are not a hard dependency. |
| `BL-064` | A | `WAIT_MULTI` | Its owned delta is explicitly the integration of long filesystem work with Operations/Job, so it must consume both filesystem and Job Manager implementations rather than merely defer their evidence. |
| `BL-098–BL-099` | B | `RECHECK_ON_PR238` | PR #238 now carries the cumulative Operations/Job chain through `BL-095`; determine from the concrete test delta whether that single ancestry is sufficient or whether additional sibling contracts are genuinely consumed. |
| `BL-103` | A | `RECHECK_FEASIBILITY` | Profile/risk-policy configuration may be Cloud-preparable from current contracts; do not require every named-root/capability PR merely because later runtime consumers use the profile. |
| `BL-118` | A | `WAIT_MULTI` | Its concrete runtime registration/authorization delta consumes the process-observation stack and capability/authorization foundations. |
| `BL-139` | A | `WAIT_MULTI` | Its concrete working-directory enforcement consumes both the command stack and named-root working-directory policy. |
| `BL-143` | A | `WAIT_MULTI` | `run_command` must consume both the command/result stack through `BL-142` and the separately prepared Managed Process Engine chain; no single current ancestry represents both. |
| `BL-157` | A | `WAIT_MULTI` | Its concrete `system.read` registration/execution delta consumes both the system-information chain and server-side capability/authorization work. |
| `BL-164` | A | `RECHECK_ON_PR238` | Job-limit security tests may be able to use the cumulative Operations/Job chain through PR #238; verify the concrete test targets before declaring additional hard predecessors. |
| `BL-228` | A | `WAIT_MULTI` | Proxy-mode runtime implementation consumes lifecycle/IPC plus platform transport behavior that is not represented by one current ancestry. |
| `BL-253–BL-254` | B | `RECHECK_FEASIBILITY` | CI orchestration can be independent when the gate itself is complete and later packages/tests are only deferred coverage; classify from the concrete workflow delta rather than future consumers. |
| `BL-341` | B | `WAIT_MULTI` | Host-process lifecycle integration spans Operations/Job cleanup and Managed Process cleanup/lifecycle foundations that are currently represented by separate open ancestries. |

The automatic selector must inspect the concrete owned delta before treating missing future components as ancestry. A task is not blocked merely because complete integration/native evidence is deferred.

### Workstream notes

- **Filesystem:** `BL-056`, `BL-057`, `BL-061` and provisional `BL-063` are now reserved by PRs #231, #232, #237 and #233. Only `BL-060` and `BL-064` remain active; apply the concrete-delta test before deciding whether Job Manager ancestry is hard.
- **Search:** all current Version-1.0 Search candidates remain reserved. Do not start another Search BL from this catalog.
- **Named roots:** `BL-103` is the only unreserved Planned row; determine whether its profile/risk-policy configuration delta can be prepared against current contracts before treating all named-root/capability PRs as hard ancestry.
- **Operations/Job:** `BL-095` is now reserved by PR #238. Remaining `BL-098–099` should first be tested against that cumulative chain before assuming multiple predecessors.
- **Process:** `BL-134` and `BL-135` are now reserved by PRs #230 and #234, completing the currently planned managed-process implementation/testing chain. `BL-118` remains a separate cross-foundation process-observation authorization case.
- **Command:** `BL-142` is now reserved by PR #236. `BL-143` still crosses the command and Managed Process ancestries; reclassify `BL-144–149` and `BL-151–152` from their actual owned deltas rather than sequence alone.
- **System:** `BL-154–156` remain reserved. `BL-157` is the sole unreserved system-information row and concretely crosses system-info plus capability enforcement.
- **Security:** `BL-161` remains reserved in addition to `BL-159`/`BL-171`. `BL-160` still crosses server authorization and root/domain bypass coverage; `BL-164` should be rebound against PR #238, while `BL-163`, `BL-167` and `BL-168` require concrete-delta classification.
- **Multi-mode / service:** `BL-224–225`, `BL-233`, `BL-235–236`, `BL-238–239` remain reserved. No Mode-C row remains active. Classify the remaining transport/service/test work from the concrete runtime delta; do not synthesize a Cloud merge when multiple implementations are actually required.
- **CI / release:** `BL-252`, `BL-256`, `BL-258` and `BL-262` are reserved. `BL-253–254` remain active under the concrete-delta feasibility rule. `BL-261` is excluded pending canonical selection/pinning of the external Rust/Go comparison servers.

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

Cloud work must not mark `BACKLOG.md` `Done` or claim local integration. A task may report deferred integration/native validation without becoming blocked when its own Cloud delta is complete.

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
ActualHeadBranch=<cloud-branch>
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

> Führe `BL-xxx` gemäß `MOBILE.md` V3 aus. Prüfe zuerst den konkreten task-puren Delta und erst dann die DependencyExecution aus dem aktuellen Checkout; bei echtem `STACK_REQUIRED` stoppe mit dem exakten Vorgänger-Ref für einen neuen Cloud-Task. Kein Fetch/Pull/Merge/Cherry-Pick des Vorgängers und kein Merge des späteren PRs.
