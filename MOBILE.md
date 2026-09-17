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

`BL-036`, `BL-099`, `BL-149`, `BL-202`, `BL-203`, `BL-205`, `BL-206`, `BL-212`, `BL-213`, `BL-214`, `BL-216`, `BL-218`, `BL-219`.

Do not create another candidate for a collision. Reconcile competing candidates during Windows/Classic finalization.

## 5. Automatic task selection

When the user says only “next task”:

1. read `AGENTS.md`, Mobile governance, current `BACKLOG.md`, this file and relevant technical contracts;
2. build the current Vacation Reservation Ledger and exclude all reserved BLs;
3. exclude every task with a known unresolved Classic/owner decision and every task directly blocked by one;
4. inspect `Planned` sprints in ascending order defined by root `AGENTS.md`;
5. for each serious candidate, run the **concrete-delta feasibility test** from `Governance/MOBILE-CLOUD-HANDOFF.md` before assigning a dependency state;
6. inside each sprint, first run the **current-checkout pass** and classify candidates as:
   - `INDEPENDENT_FROM_CURRENT_CHECKOUT`;
   - `STACK_BASE_READY`;
   - `STACK_REQUIRED`;
   - `BLOCKED_DEPENDENCY_DECISION`; or
   - `BLOCKED_MULTIPLE_UNCOMBINED_MOBILE_PREDECESSORS`;
7. if that sprint contains an unreserved `INDEPENDENT_FROM_CURRENT_CHECKOUT` or `STACK_BASE_READY` candidate, select from it and prefer mode A before B before C, independent before stack-base-ready, then lower effort, lower Windows residual and lower collision risk;
8. if the sprint has no current-checkout executable candidate, run the read-only **open-PR-head restart pass** before advancing to a later sprint: for each remaining candidate, determine whether exactly one open PR head ancestry contains every hard predecessor delta it needs;
9. when one or more candidates in the sprint each have one sufficient open PR head, choose among those candidates with the same mode/effort/Windows-residual/collision preferences and return `STACK_RESTART_REQUIRED` with the selected parent PR, branch and SHA;
10. `PR_STACK_CANDIDATE` in this file is only a snapshot hint for that restart pass; the canonical state remains `STACK_REQUIRED` until a new Cloud task starts from the parent and verifies `STACK_BASE_READY` locally;
11. if a candidate genuinely requires several independent uncombined PR heads, classify it `BLOCKED_MULTIPLE_UNCOMBINED_MOBILE_PREDECESSORS`, exclude only that candidate and continue;
12. advance to the next Planned sprint only when the current sprint has neither a current-checkout executable candidate nor a single-head restart candidate;
13. return a global no-executable Planned result only after every allowed Planned sprint has completed both passes;
14. do not auto-select `Later` while executable or single-head-restartable unreserved `Planned` work exists.

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

A dependent task is executable only when the current checkout already contains every hard predecessor delta it consumes. The normal route is a **new Codex Cloud task started directly from one sufficient predecessor PR head branch/SHA**. That head may itself contain a cumulative chain of earlier Mobile prerequisites.

Verify locally, when possible:

```text
git merge-base --is-ancestor <ParentHeadSha> HEAD
```

Then:

```text
DependencyExecution=STACK_BASE_READY
ExpectedPRBase=<ParentHeadBranch>
Mobile-Depends-On: <direct-parent-BL>[,<contained-prerequisite-BL>...]
```

### Stack required / restart candidate

If the hard predecessor delta set is absent from the current checkout but exactly one open PR head ancestry contains all of it, do not import that head with fetch/pull/merge/cherry-pick/patch replay/downloaded Git objects/credential setup. Classify the candidate canonically as `STACK_REQUIRED`; this catalog may additionally label the current snapshot as `PR_STACK_CANDIDATE`.

Return:

```text
Status=STACK_RESTART_REQUIRED
TaskID=BL-xxx
DependsOn=BL-yyy
ParentPR=#n
ParentHeadBranch=<branch>
ParentHeadSha=<sha>
MutationCount=0
```

A reserved predecessor PR is a valid stack start. Reservation prevents duplicate implementation of the predecessor; it does not prohibit an unreserved child from using that predecessor head as its initial checkout.

If the concrete delta genuinely requires multiple independent uncombined predecessors and no one open PR head ancestry contains the complete required set, classify that candidate as `BLOCKED_MULTIPLE_UNCOMBINED_MOBILE_PREDECESSORS`, exclude it, and continue scanning. Never synthesize a Cloud merge branch.

## 7. Snapshot binding — 2026-09-17

```text
Repository : thomasweidner/flashgate-mcp
Main       : 383714cf77a816f0427b7a5e842139351b96b2d5
AGENTS     : 0e686cbbd305204cb077771bd0f88a1a15d164d2 (pre-this-update)
MobileGov  : dd0e55f7251ab9c97f8b777fd574561b5134044b (pre-this-update)
BACKLOG    : a02db7c09b10dba8c827f62618c7c6eb9f096591
MOBILE     : 85ad498761f45849e749d85c19fa822c2d9a3600 (pre-this-update)
Ledger     : 198 open GitHub PRs, through #257 at snapshot time
```

No new reservation was created between the preceding queue refresh and this topology correction. The most recent reservation delta remains:

`BL-060`, `BL-098`, `BL-099`, `BL-103`, `BL-144`, `BL-145`, `BL-148`, `BL-149`, `BL-151`, `BL-164`, `BL-226`, `BL-227`, `BL-237`, `BL-253`, `BL-254`.

Reservation evidence remains:

- PR #240 prepares `BL-164` on the Operations/Job chain.
- PR #241 prepares `BL-098`; PRs #242 and #243 both reserve `BL-099` and therefore form a duplicate reservation collision.
- PR #244 prepares `BL-103`.
- PR #245 prepares `BL-151`; PR #246 prepares `BL-144`; PR #248 prepares `BL-145`; PR #252 prepares `BL-148`.
- PRs #253 and #254 both reserve `BL-149` and therefore form a duplicate reservation collision.
- PR #247 prepares `BL-253`.
- PR #250 prepares `BL-060`.
- PR #251 is a provisional UI-generated reservation for `BL-254`: its Operations/Job CI orchestration scope maps unambiguously to the canonical `BL-254` row even though its title omits the BL marker.
- PR #255 prepares `BL-227`; PR #256 prepares `BL-226`.
- PR #257 prepares `BL-237` as the Variant A service-account backend.

PR #249 overlaps the already-reserved execution-redaction candidate but does not create a new active BL identity; `BL-145` is already durably reserved by PR #248.

The complete Version-1.0 Search queue remains reserved by open PRs. The active catalog still contains 21 unreserved, non-decision-bound Planned topics. The important correction in this snapshot is that **absence from current `main` is not the end of selection**: after the current-checkout pass, open PR heads must be evaluated as possible stack restart bases.

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

There are no active Mode-C rows at this snapshot. This does **not** change the A→B→C selection contract or make Mode C categorically ineligible after a future refresh.

## 9. Active Planned candidates

These IDs are unreserved at the snapshot, are `Planned`, and are not blocked by a known unresolved Classic/owner decision.

| Epic | Mode A | Mode B | Mode C |
|---|---|---|---|
| Filesystem | `BL-064` | — | — |
| Process | `BL-118` | — | — |
| Command execution | `BL-139`, `BL-143` | `BL-146–BL-147`, `BL-152` | — |
| System information | `BL-157` | — | — |
| Security | `BL-160`, `BL-163`, `BL-167` | `BL-168` | — |
| Native multi-mode / service | `BL-228` | `BL-229–BL-231`, `BL-234`, `BL-241–BL-242`, `BL-244` | — |
| Cross-mode host lifecycle | — | `BL-341` | — |

Snapshot totals:

```text
Mode A active : 9
Mode B active : 12
Mode C active : 0
Total active  : 21
```

`Later`, completed, reserved, known decision-blocked, Mode-D and Mode-X rows are intentionally omitted.

## 10. Current execution-entry topology

The current `main` checkout still has no task-pure executable unreserved candidate among these 21. That result is only the **first pass** and must no longer be returned as the final selector outcome without the PR-head restart pass.

Snapshot entry classes:

```text
MAIN_EXECUTABLE     : none
PR_STACK_CANDIDATE  : BL-229
WAIT_MULTI          : 20 topics
```

### 10.1 Single-head restart candidate

`BL-229` is currently the earliest verified single-head restart path.

Its concrete owned delta is automatic managed-endpoint discovery and safe STDIO fallback. Open PR #205 (`BL-233`) already contains the accepted runtime configuration, canonical endpoint, discovery, timeout/retry, `proxy`/`auto` fallback, diagnostics and redaction contract that `BL-229` must consume. The `BL-229` selection/decision logic can be implemented and meaningfully validated from that one contract head without importing the Windows Named Pipe, Unix-socket, service-host or execution-identity sibling PRs; integration with those later runtime components remains deferred evidence rather than a reason to combine their branches into this Cloud task.

Fresh selection must re-check that the PR is still open and its head identity is unchanged. Snapshot start data:

```text
CatalogEntryClass   : PR_STACK_CANDIDATE
CanonicalState      : STACK_REQUIRED
TaskID              : BL-229
DependsOn           : BL-233
ParentPR             : #205
ParentHeadBranch     : codex/fuhre-cloud-task-aus-mobile.md-v3-aus-djhu71
ParentHeadSha        : e084895636e92e22f0b85f9e97d6e914f740ce81
CurrentMain          : 383714cf77a816f0427b7a5e842139351b96b2d5
ExpectedSelector     : STACK_RESTART_REQUIRED
```

Expected restart instruction after fresh read-only revalidation:

```text
Führe BL-229 gemäß `MOBILE.md` V3 als abhängigen Mobile-Task aus. Der ausgewählte Start-Ref ist der Vorgänger von BL-233. Verifiziere lokal, dass ParentHeadSha e084895636e92e22f0b85f9e97d6e914f740ce81 im aktuellen Checkout enthalten ist. Kein Fetch, Pull, Merge oder Cherry-Pick des Vorgängers. Der spätere Child-PR muss den direkten Vorgängerbranch `codex/fuhre-cloud-task-aus-mobile.md-v3-aus-djhu71` als Base verwenden.
```

### 10.2 Remaining multi-line candidates

The other 20 active topics still require more than one independent prepared line, or integrated runtime surfaces that are currently split across those lines. They are therefore snapshot `WAIT_MULTI` candidates, not global queue blockers:

| Tasks | Why one current PR head is not sufficient at this snapshot |
|---|---|
| `BL-064` | Integrates long filesystem operations with Operations/Job; filesystem and Operations/Job implementations are on independent lines. |
| `BL-118` | Must register and execute-authorize `process.observe`; process-observation work through PR #193 and capability/authorization work are on separate lines. |
| `BL-139` | Connects typed commands to named-root process-working-directory policy; command and named-root/capability lines are independent. |
| `BL-143` | `run_command` must be the synchronous wrapper over the Managed Process Engine; command and managed-process lines remain independent. |
| `BL-146–BL-147` | Windows/Linux execution isolation consumes command policy plus Managed Process/execution-identity/platform controls that are not cumulative in one current head. |
| `BL-152` | Cross-platform execution security tests need allowlist/args/roots/environment/output/timeout/isolation surfaces that are split across current command, roots and platform lines. |
| `BL-157` | `system.read` requires the System Information line plus server-side capability/authorization enforcement. |
| `BL-160` | Authorization-bypass coverage requires execution-time authorization together with named-root/domain policy surfaces; the relevant open lines diverge. |
| `BL-163` | Execution policy enforcement spans command definitions/arguments, roots, environment, limits and isolation, currently split across multiple PR lines. |
| `BL-167` | Cross-domain secret redaction must cover process, execution, system, jobs and audit owners; those domains are not represented by one current ancestry. |
| `BL-168` | Least-privilege validation needs execution identity/service-account behavior plus child/platform-isolation behavior from separate lines. |
| `BL-228` | STDIO proxy mode needs transport-neutral lifecycle/runtime plus local IPC protocol/transport behavior that is currently split. |
| `BL-230–BL-231` | Windows SCM/Linux systemd hosting needs lifecycle, platform transport and service execution-identity foundations from separate lines. |
| `BL-234` | Service-side authorization combines OS-derived transport caller identity, roots/profiles/capabilities, execution-identity dispatch, per-principal limits and audit; no one head contains all of those foundations. |
| `BL-241–BL-242`, `BL-244` | Integrated multi-mode tests, CI/release validation and benchmarks require several proxy/service/transport/lifecycle implementations to exist together first. |
| `BL-341` | Canonically consumes process-root lifecycle plus Operations/Job (`BL-094`) and Managed Child (`BL-129`) cleanup owners, which remain independent prepared lines. |

`WAIT_MULTI` is a snapshot catalog annotation, not a new governance state. Fresh task selection must still run the concrete-delta test and public-PR ancestry check because a later stacked PR may consolidate the required lines and turn one of these topics into a single-head restart candidate.

The default phone prompt should therefore currently return `STACK_RESTART_REQUIRED` for `BL-229` after fresh validation, rather than `NO_EXECUTABLE_UNRESERVED_PLANNED_MOBILE_TASK`.

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
