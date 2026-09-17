# FlashGate MCP — Codex Cloud Mobile Handoff

**Status:** Binding for FlashGate Mobile / Codex Cloud vacation work  
**Protocol:** `FLASHGATE-MOBILE-V3`  
**Purpose:** make already prepared Mobile work discoverable across independent Codex Cloud tasks without relying on agent memory or a mutable repository state ledger.

## 1. Core model

GitHub open pull requests are the **Vacation Reservation Ledger**.

Codex Cloud task memory is advisory only. Do not use memory as the authority for whether a BL task has already been prepared.

Do not create `MOBILE-STATE.md` or another repository file merely to record vacation reservations.

A BL task is reserved for the vacation workflow when an open GitHub PR can be associated unambiguously with that task according to section 3. Reservation is established from PR/task identity and does **not** depend on commit-SHA or tree-SHA equality. Reserved tasks are excluded from later automatic Mobile selection until Windows finalization or an explicit user decision changes that state.

Content identity is a separate post-publication check. A failed or unavailable content-identity check may block acceptance of the PR as the exact Cloud candidate, but it must never unreserve the BL task or cause automatic duplicate implementation.

## 2. Read-only GitHub access

Before every automatic Mobile task selection, query the public GitHub API for open pull requests in:

`thomasweidner/flashgate-mcp`

Canonical endpoint:

```text
GET https://api.github.com/repos/thomasweidner/flashgate-mcp/pulls?state=open&per_page=100
```

The Mobile read path is intentionally narrow:

```text
AllowedHost    : api.github.com
AllowedMethods : GET, HEAD, OPTIONS
Credentials    : NONE
Authentication : NONE
```

Do not send PATs, tokens, SSH credentials, cookies, or other secrets.

If the Codex Cloud environment does not allow this read-only request, stop automatic task selection with:

```text
Status=BLOCKED_MOBILE_GITHUB_READ_ACCESS_REQUIRED
RequiredHost=api.github.com
RequiredMethods=GET,HEAD,OPTIONS
CredentialsRequired=false
```

Do not fall back to memory, guessed prior-task state, manual Git remotes, or credentials.

## 3. Reservation identity

For every open PR returned by GitHub, determine its Mobile task identity using this precedence.

### 3.1 Canonical identity

If the PR body contains an exact standalone marker:

```text
Mobile-Task: BL-xxx
```

that BL ID is reserved immediately.

If the title and body disagree on the selected task identity, return:

```text
Status=MOBILE_PR_IDENTITY_CONFLICT
```

and do not automatically select any task whose identity is implicated by the conflict.

### 3.2 Provisional UI-generated identity

The Codex UI may create a GitHub PR with a title or body that does not preserve the complete Mobile V3 marker block.

When no `Mobile-Task` marker exists, a PR is still a valid vacation reservation when exactly one selected-task BL identity is unambiguous from the PR metadata.

Preferred fallback order:

1. exactly one `BL-xxx` in the PR title;
2. an explicit body statement such as `Complete BL-xxx`, `TaskID: BL-xxx`, `Mobile task BL-xxx`, or equivalent unambiguous selected-task wording;
3. exactly one distinct `BL-xxx` anywhere in title/body when there is no conflicting dependency or historical-task context.

Record such a PR as:

```text
PRMetadataState=PROVISIONAL_UI_GENERATED
```

Do not reject or duplicate the task merely because the UI changed the PR title format.

If multiple BL IDs are present and the selected task cannot be distinguished safely:

```text
Status=MOBILE_PR_IDENTITY_AMBIGUOUS
```

Treat the ambiguous PR as requiring review and do not automatically select any BL ID that could plausibly be its selected task.

### 3.3 Reservation is independent of commit identity

Commit SHA, branch name, tree SHA, commit message, author, committer, and timestamp are **not reservation keys**.

Once an open PR resolves unambiguously to `BL-xxx`, record:

```text
ReservationState=VACATION_CANDIDATE_ALREADY_PREPARED
```

and exclude that BL ID from later automatic Mobile selection even when:

- the GitHub head commit SHA differs from the Cloud commit SHA;
- the GitHub branch name differs from the preferred or Cloud-reported name;
- the tree/content identity check is pending, unavailable, or fails.

## 4. Dependency execution model

Dependency hints in `MOBILE.md` are planning hints only. They do not establish a Git ancestry requirement.

For each candidate, inspect the current repository contracts and code and classify execution from the **current checkout**:

```text
INDEPENDENT_FROM_CURRENT_CHECKOUT
STACK_BASE_READY
STACK_REQUIRED
BLOCKED_DEPENDENCY_DECISION
BLOCKED_MULTIPLE_UNCOMBINED_MOBILE_PREDECESSORS
```

### 4.0 Concrete-delta feasibility test

Before assigning any dependency state, identify the candidate's **concrete task-pure delta** and answer these questions from the current checkout:

1. What code, documentation, tests, workflow or schema delta does this BL itself own?
2. Which APIs, types, schemas, contracts or runtime behavior must that delta consume **now**?
3. Can the owned delta be implemented coherently and meaningfully validated now without importing an open predecessor?
4. Are missing predecessor changes required to implement the owned delta, or are they only future consumers, later integration coverage, later end-to-end evidence, Windows/native finalization, or later release evidence?
5. Would the candidate remain correct when later packages/features integrate because its gate/contract is generic or automatically expands over the repository?

A dependency is hard only when the concrete owned delta actually requires unintegrated predecessor code, types, schemas, contracts, or runtime behavior. Do **not** infer `STACK_REQUIRED` or a blocked state merely because acceptance notes mention:

- future consumers;
- future packages that a generic test or CI gate will automatically cover;
- later CI or end-to-end coverage;
- later integration validation;
- Windows/native/platform finalization;
- release evidence produced only after other work integrates;
- documentation of an already-authorized target contract whose runtime implementation is separately owned.

If the task-pure delta can be implemented and focused validation can pass now, classify it from the current checkout accordingly and record remaining integration/native evidence as deferred. Deferred evidence never permits claiming the BL `Done`; it also does not by itself create a Git dependency.

### 4.1 Independent from current checkout

Use `INDEPENDENT_FROM_CURRENT_CHECKOUT` when the candidate's concrete task-pure delta can be implemented correctly against the current checkout without consuming unintegrated predecessor changes.

This remains true even when `MOBILE.md` names an earlier conceptual or sequencing hint, when later components will consume the result, or when the candidate's generic tests/CI gates will gain additional coverage after future packages integrate.

For a task started from `main`:

```text
ExpectedPRBase=main
Mobile-Depends-On: NONE
```

Do not manufacture a stack solely to mirror a dependency hint or future integration relationship.

### 4.2 Stack base ready

Use `STACK_BASE_READY` only when the current Codex Cloud checkout already contains every hard predecessor delta that the child genuinely consumes.

A child task intentionally started from one sufficient predecessor PR head branch or commit is the normal way to reach this state. That one head may itself be the tip of a cumulative Mobile ancestry and therefore contain several logical prerequisite BLs.

Before child implementation, verify locally that the selected predecessor commit is present in the current checkout. When the predecessor SHA is available as a local object, use:

```text
git merge-base --is-ancestor <ParentHeadSha> HEAD
```

or an equivalent local ancestry check.

No network fetch is required or permitted for this verification.

For a valid stack child:

```text
ExpectedPRBase=<ParentHeadBranch>
Mobile-Depends-On: <direct-parent-BL>[,<contained-prerequisite-BL>...]
```

The direct parent branch remains the later PR base even when that parent ancestry contains earlier prerequisites.

### 4.3 Stack required

Use `STACK_REQUIRED` only when:

- the candidate's concrete task-pure delta genuinely consumes hard predecessor changes that are not already present in the current checkout; and
- exactly one open Mobile PR head ancestry contains **all** hard predecessor deltas required to implement the candidate.

The candidate may have several logical prerequisite BLs when they are already cumulative in that one head ancestry. `STACK_REQUIRED` becomes `BLOCKED_MULTIPLE_UNCOMBINED_MOBILE_PREDECESSORS` only when no single open PR head contains the complete hard-predecessor set and two or more independent heads would have to be combined.

Do not use `STACK_REQUIRED` for a future consumer relationship, deferred integration/CI/native evidence, or a generic gate that can be implemented correctly against the current checkout.

A `STACK_REQUIRED` task is **not executable in the current Cloud task**.

Do not make it executable by:

- `git fetch`;
- `git pull`;
- adding/changing a remote;
- downloading Git pack/object data;
- merge;
- cherry-pick;
- patch replay;
- synthetic merge branch;
- manual credential or token configuration.

The public GitHub API may be used read-only to identify candidate predecessor PRs, branches, head SHAs, bases, and ancestry relationships. It is not a transport for importing predecessor Git history.

A predecessor BL being reserved is expected and does not disqualify its PR head as a stack start. Reservation prevents reimplementing that predecessor; it does not prohibit an unreserved child from starting from the predecessor's existing head.

### 4.4 Open-PR-head restart pass

Automatic selection is two-tiered **inside each Planned sprint**.

1. First run the normal current-checkout feasibility/dependency pass.
2. If that sprint contains an unreserved `INDEPENDENT_FROM_CURRENT_CHECKOUT` or `STACK_BASE_READY` candidate, select from those executable candidates and do not prefer a restart merely because one exists.
3. If the sprint has no current-checkout executable candidate, perform a read-only restart pass over its remaining unreserved candidates before advancing to the next sprint.
4. For each such candidate, identify the complete hard-predecessor delta set and inspect the open Vacation Reservation Ledger plus public PR/base/head/compare metadata to determine whether one open PR head ancestry contains that complete set.
5. If exactly one sufficient head exists for a candidate, that candidate is a single-head restart candidate. `MOBILE.md` may describe this snapshot condition as `PR_STACK_CANDIDATE`; the canonical dependency state remains `STACK_REQUIRED` until a new Cloud task actually starts from that head and locally verifies `STACK_BASE_READY`.
6. If two or more candidate tasks in the same sprint each have one sufficient head, that is valid. Choose among the tasks with the normal mode, effort, Windows-residual, and collision preferences. The word **single** applies to the sufficient head for each candidate, not to the number of restartable candidates in the sprint.
7. If no single head contains a candidate's complete hard-predecessor set, classify only that candidate as `BLOCKED_MULTIPLE_UNCOMBINED_MOBILE_PREDECESSORS` when multiple independent heads are genuinely required.
8. Advance to the next Planned sprint only when the current sprint has neither a current-checkout executable candidate nor a single-head restart candidate.

For the selected single-head restart candidate, stop before implementation with:

```text
Status              : STACK_RESTART_REQUIRED
TaskID              : BL-xxx
DependsOn           : BL-yyy
ParentPR             : #<number>
ParentHeadBranch     : <GitHub predecessor branch>
ParentHeadSha        : <GitHub predecessor head SHA>
CurrentCheckoutBase  : <current branch/ref>
MutationCount        : 0
NextAction           : Start a new Codex Cloud task from ParentHeadBranch (or ParentHeadSha) and execute TaskID as the stack child
```

`DependsOn` identifies the selected direct-parent PR's Mobile task. Any earlier hard prerequisites already contained in that head ancestry remain represented by the ancestry rather than requiring another import.

Also provide this exact short restart instruction with the actual values filled in:

```text
Führe BL-xxx gemäß `MOBILE.md` V3 als abhängigen Mobile-Task aus. Der ausgewählte Start-Ref ist der Vorgänger von BL-yyy. Verifiziere lokal, dass ParentHeadSha im aktuellen Checkout enthalten ist. Kein Fetch, Pull, Merge oder Cherry-Pick des Vorgängers. Der spätere Child-PR muss den direkten Vorgängerbranch als Base verwenden.
```

The failed attempt to import a predecessor is not a reason to request reauthorization for the same network operation. The correct remedy is a new Cloud task whose initial checkout is already bound to the predecessor.

### 4.5 Multiple predecessors

Use `BLOCKED_MULTIPLE_UNCOMBINED_MOBILE_PREDECESSORS` only when the candidate's concrete task-pure delta genuinely consumes more than one independent uncombined open Mobile predecessor and the complete required changes are not already represented by any one open PR-head ancestry.

Do not create this state from future consumers, deferred integration evidence, or multiple components that a generic repository-wide gate can cover later without changing the gate implementation.

```text
Status=BLOCKED_MULTIPLE_UNCOMBINED_MOBILE_PREDECESSORS
```

This state is **candidate-local**. Do not synthesize a merge branch in Cloud. Exclude the blocked candidate and continue evaluating other candidates in the sprint and later Planned sprints. Defer combination to Windows unless the user explicitly authorizes another strategy.

## 5. Automatic task selection

Before selecting the next Mobile task:

1. enumerate open GitHub PRs through section 2;
2. build `ReservedTaskIDs` through section 3;
3. exclude `ReservedTaskIDs`;
4. read current Planned sprints in the order defined by root `AGENTS.md`;
5. process each Planned sprint in ascending order;
6. for every serious candidate in that sprint, run the concrete-delta feasibility test in section 4.0 before assigning an ancestry/dependency state;
7. run the current-checkout pass through sections 4.1–4.3 and select only `INDEPENDENT_FROM_CURRENT_CHECKOUT` or `STACK_BASE_READY` candidates when at least one exists;
8. within that sprint apply the mode/effort/Windows-residual/collision preferences from root `AGENTS.md`;
9. when the sprint has no current-checkout executable candidate, run the open-PR-head restart pass from section 4.4 before considering a later sprint;
10. for every restart-pass candidate, accept one cumulative open PR ancestry as sufficient when it contains the candidate's entire hard-predecessor set; do not require every logical prerequisite to be a separate direct parent;
11. when one or more candidates in the sprint each have one sufficient head, choose among them with the normal preferences and return `STACK_RESTART_REQUIRED` for the selected candidate;
12. do not attempt network Git operations to promote `STACK_REQUIRED` into an executable state;
13. a candidate-local `BLOCKED_MULTIPLE_UNCOMBINED_MOBILE_PREDECESSORS` does not end the automatic scan; continue with other candidates, then later Planned sprints only if this sprint has no restartable path;
14. return a global no-executable Planned result only after every allowed Planned sprint has completed both the current-checkout pass and, where needed, the open-PR-head restart pass;
15. do not jump to `Later` work while executable or single-head-restartable unreserved `Planned` work exists.

An existing reservation does not mean the BL task is `Done`. It means only:

```text
VACATION_CANDIDATE_ALREADY_PREPARED
```

and prevents duplicate Cloud implementation during the vacation workflow.

## 6. Codex implementation completion

Codex Cloud is responsible for:

- task discovery;
- dependency execution classification;
- implementation only when the task is executable from the current checkout;
- Cloud-available validation;
- a task-pure commit;
- preserving the actual Cloud branch for diagnostics;
- recording the Cloud commit SHA for diagnostics;
- recording the Cloud tree SHA for later content-identity validation;
- preparing PR metadata for the Codex UI.

Before asking the user to create the PR, prepare:

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

Obtain the committed Cloud tree identity with:

```text
git rev-parse 'HEAD^{tree}'
```

For an independent candidate:

```text
ExpectedPRBase=main
```

For a stack child:

```text
ExpectedPRBase=<direct predecessor Mobile branch>
```

The task must have started from that predecessor branch/commit and locally verified that the predecessor ancestry is present before implementation.

If the available Codex PR-metadata preparation facility has its own naming behavior, still include the BL ID explicitly in the prepared title and selected-task description.

At this point return:

```text
Status              : CLOUD_IMPLEMENTATION_COMPLETE_AWAITING_MANUAL_PR
TaskID              : BL-xxx
DependencyExecution : INDEPENDENT_FROM_CURRENT_CHECKOUT | STACK_BASE_READY
ActualHeadBranch     : <cloud-branch>
CloudCommitSha       : <full Cloud commit SHA>
CloudTreeSha         : <full Cloud tree SHA>
ExpectedPRBase       : <main-or-parent-mobile-branch>
ExpectedPRTitle      : [MOBILE][BL-xxx] <subject>
WindowsFinalization  : REQUIRED
NextAction           : Use Codex UI Create PR, then reply "PR erstellt"
```

Do not require or invent a PR number at this stage.

## 7. Manual Create-PR handoff

The user performs the Codex UI **Create PR** action.

The user does not need to copy the GitHub PR number back into the Codex task.

After the user replies that the PR was created, enumerate the open GitHub PRs again through section 2.

### 7.1 First establish reservation by BL identity

Find open PRs whose selected task resolves to the current `TaskID` through section 3.

Exactly one open PR resolves to the expected `TaskID`:

```text
ReservationState=VACATION_CANDIDATE_ALREADY_PREPARED
```

The task is now reserved regardless of commit-SHA equality. Do not implement this BL again automatically.

No open PR resolves to the expected task:

```text
Status=MANUAL_PR_CREATED_PENDING_GITHUB_VISIBILITY
```

Do not claim durable completion and do not create another implementation of the same BL. The current task remains locally reserved until the user explicitly decides otherwise.

More than one open PR resolves to the same task, or task identity is contradictory:

```text
Status=MOBILE_PR_CORRELATION_AMBIGUOUS
```

Treat the BL as reserved and stop for review. Do not select it again automatically.

### 7.2 Then validate content identity

For the single correlated PR, record:

```text
GitHubHeadSha = pr.head.sha
```

Fetch the GitHub head commit read-only and record its Git tree:

```text
GET https://api.github.com/repos/thomasweidner/flashgate-mcp/git/commits/<GitHubHeadSha>
GitHubTreeSha = response.tree.sha
```

Compare:

```text
CloudTreeSha == GitHubTreeSha
```

This is the authoritative content-identity check for the manual UI handoff.

The commit SHA itself is diagnostic only. Git commit IDs may differ when publication rematerializes the same tree with different commit metadata.

### 7.3 Content-identity outcomes

Tree SHA matches:

```text
ContentIdentity=PASS
CommitShaRematerialized=<CloudCommitSha != GitHubHeadSha>
GitHubReadback=PASS
```

A different `GitHubHeadSha` is accepted when the tree matches.

Tree SHA differs:

```text
Status=MOBILE_PR_CONTENT_MISMATCH
ContentIdentity=FAIL
ReservationState=VACATION_CANDIDATE_ALREADY_PREPARED
```

Stop content acceptance and report both tree SHAs. **Do not unreserve or reimplement the BL automatically.**

Cloud tree SHA or GitHub tree SHA cannot be obtained:

```text
Status=MOBILE_PR_RESERVED_CONTENT_IDENTITY_UNVERIFIED
ContentIdentity=NOT_AVAILABLE
ReservationState=VACATION_CANDIDATE_ALREADY_PREPARED
```

Keep the BL reserved. Do not substitute commit-SHA equality as a hard gate.

### 7.4 Base and branch checks

The expected PR base remains a hard topology check:

```text
pr.base.ref == ExpectedPRBase
```

For a stacked candidate, the direct predecessor branch must match the expected Mobile predecessor.

`pr.head.ref == ActualHeadBranch` is diagnostic only. A UI-generated or rematerialized branch name is acceptable when task identity, expected base, and content identity are valid.

If the Codex UI creates a stacked child PR against `main` instead of `ExpectedPRBase`, return:

```text
Status=MOBILE_STACK_PR_BASE_MISMATCH
ReservationState=VACATION_CANDIDATE_ALREADY_PREPARED
```

Keep the child BL reserved and stop. Do not retarget, close, recreate, merge, or otherwise mutate the PR during vacation work without a separate explicit authorization.

## 8. Post-publication validation

After correlation, read the discovered PR number, URL, state, base, head ref, head SHA, head tree SHA, title, and body from GitHub.

A fully accepted durable vacation candidate requires:

- repository `thomasweidner/flashgate-mcp`;
- PR state `OPEN`;
- selected BL task resolvable through section 3;
- expected base or direct stacked predecessor;
- task-pure PR scope;
- `CloudTreeSha == GitHubTreeSha`.

The following do **not** invalidate an otherwise accepted candidate:

- `CloudCommitSha != GitHubHeadSha` when tree SHA matches;
- a generated GitHub head branch name different from the preferred Cloud branch name;
- UI-generated title/body formatting when section 3 still resolves the BL unambiguously.

Metadata may be:

```text
CANONICAL
```

when the full V3 marker block and `[MOBILE][BL-xxx]` title are present, or:

```text
PROVISIONAL_UI_GENERATED
```

when the UI altered formatting but the selected BL task remains unambiguous.

Both states reserve the task and prevent duplicate vacation work.

On full acceptance return:

```text
Status                    : CLOUD_IMPLEMENTATION_COMPLETE_PR_OPEN
TaskID                    : BL-xxx
ReservationState          : VACATION_CANDIDATE_ALREADY_PREPARED
DependencyExecution       : INDEPENDENT_FROM_CURRENT_CHECKOUT | STACK_BASE_READY
PullRequest               : #<number>
PRUrl                      : <GitHub URL>
PRMetadataState           : CANONICAL | PROVISIONAL_UI_GENERATED
ActualHeadBranch          : <GitHub head ref>
CloudCommitSha            : <Cloud commit SHA>
GitHubHeadSha             : <GitHub commit SHA>
CommitShaRematerialized   : true | false
CloudTreeSha              : <Cloud tree SHA>
GitHubTreeSha             : <GitHub tree SHA>
ContentIdentity           : PASS
GitHubReadback            : PASS
WindowsFinalization       : REQUIRED
NextAction                : Leave PR open; select a different unreserved Mobile task
```

Do not merge, close, delete, retarget, or rewrite the PR merely to normalize UI-generated metadata during vacation work.

## 9. Security and mutation boundary

The public GitHub API use defined here is read-only discovery.

It does not authorize:

- GitHub writes from the Codex agent;
- `git fetch`/`git pull` of predecessor branches or commits;
- manual `git remote` configuration;
- PAT/token/SSH setup;
- repository settings;
- merge/cherry-pick/synthetic stack construction;
- PR retargeting, close, or merge;
- branch deletion;
- force operations;
- release or tag creation.

The only Mobile publication write in the phone workflow is the user's explicit Codex UI **Create PR** action.

## 10. Windows finalization

Open vacation PRs remain candidates only.

On return to Windows, rebind each candidate against:

- current central local Slim Governance;
- current root/local `AGENTS.md`;
- current `BACKLOG.md`;
- current `main`;
- real dependency ancestry;
- Windows/native validation requirements.

Only that later finalization may establish canonical BL completion or authorize integration/cleanup.
