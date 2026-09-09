# FlashGate MCP — Codex Cloud Mobile Handoff

**Status:** Binding for FlashGate Mobile / Codex Cloud vacation work  
**Protocol:** `FLASHGATE-MOBILE-V3`  
**Purpose:** make already prepared Mobile work discoverable across independent Codex Cloud tasks without relying on agent memory or a mutable repository state ledger.

## 1. Core model

GitHub open pull requests are the **Vacation Reservation Ledger**.

Codex Cloud task memory is advisory only. Do not use memory as the authority for whether a BL task has already been prepared.

Do not create `MOBILE-STATE.md` or another repository file merely to record vacation reservations.

A BL task is reserved for the vacation workflow when an open GitHub PR can be associated with that task according to section 3. Reserved tasks are excluded from later automatic Mobile selection until Windows finalization or an explicit user decision changes that state.

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

that BL ID is reserved.

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

## 4. Automatic task selection

Before selecting the next Mobile task:

1. enumerate open GitHub PRs through section 2;
2. build `ReservedTaskIDs` through section 3;
3. exclude `ReservedTaskIDs` from automatic selection;
4. apply the Planned-sprint priority in root `AGENTS.md`;
5. apply `MOBILE.md` Cloud-mode, dependency, and stacked-PR rules.

An existing reservation does not mean the BL task is `Done`. It means only:

```text
VACATION_CANDIDATE_ALREADY_PREPARED
```

and prevents duplicate Cloud implementation during the vacation workflow.

## 5. Codex implementation completion

Codex Cloud is responsible for:

- task discovery;
- implementation;
- Cloud-available validation;
- a task-pure commit;
- preserving the actual head branch and head SHA;
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

If the available Codex PR-metadata preparation facility has its own naming behavior, still include the BL ID explicitly in the prepared title and selected-task description.

At this point return:

```text
Status              : CLOUD_IMPLEMENTATION_COMPLETE_AWAITING_MANUAL_PR
TaskID              : BL-xxx
ActualHeadBranch     : <branch>
HeadSha              : <full SHA>
ExpectedPRBase       : <main-or-parent-mobile-branch>
ExpectedPRTitle      : [MOBILE][BL-xxx] <subject>
WindowsFinalization  : REQUIRED
NextAction           : Use Codex UI Create PR, then reply "PR erstellt"
```

Do not require or invent a PR number at this stage.

## 6. Manual Create-PR handoff

The user performs the Codex UI **Create PR** action.

The user does not need to copy the GitHub PR number back into the Codex task.

After the user replies that the PR was created, enumerate the open GitHub PRs again through section 2.

Find the publication primarily by:

```text
pr.head.sha == HeadSha
```

Secondary consistency checks:

```text
pr.head.ref == ActualHeadBranch
pr.base.ref == ExpectedPRBase
```

The exact head SHA is the primary correlation key because the PR number does not exist until after the manual UI publication.

### Correlation outcomes

Exactly one open PR with the expected head SHA:

```text
GitHubReadback=PASS
```

No matching open PR:

```text
Status=MANUAL_PR_CREATED_PENDING_GITHUB_VISIBILITY
```

Do not claim durable completion and do not create another implementation.

More than one matching PR or contradictory base/head identity:

```text
Status=MOBILE_PR_CORRELATION_AMBIGUOUS
```

Stop for user review.

## 7. Post-publication validation

After correlation, read the discovered PR number, URL, state, base, head ref, head SHA, title, and body from GitHub.

A durable vacation reservation requires:

- repository `thomasweidner/flashgate-mcp`;
- PR state `OPEN`;
- exact expected `head.sha`;
- expected base or direct stacked predecessor;
- task-pure PR scope;
- selected BL task resolvable through section 3.

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

Return:

```text
Status                    : CLOUD_IMPLEMENTATION_COMPLETE_PR_OPEN
TaskID                    : BL-xxx
PullRequest               : #<number>
PRUrl                      : <GitHub URL>
PRMetadataState           : CANONICAL | PROVISIONAL_UI_GENERATED
ActualHeadBranch          : <GitHub head ref>
HeadSha                   : <GitHub head SHA>
GitHubReadback            : PASS
WindowsFinalization       : REQUIRED
NextAction                : Leave PR open; select a different unreserved Mobile task
```

Do not merge, close, delete, or rewrite the PR merely to normalize UI-generated metadata during vacation work.

## 8. Security and mutation boundary

The public GitHub API use defined here is read-only discovery.

It does not authorize:

- GitHub writes from the Codex agent;
- manual `git remote` configuration;
- PAT/token/SSH setup;
- repository settings;
- branch deletion;
- PR close or merge;
- force operations;
- release or tag creation.

The only Mobile publication write in the phone workflow is the user's explicit Codex UI **Create PR** action.

## 9. Windows finalization

Open vacation PRs remain candidates only.

On return to Windows, rebind each candidate against:

- current central local Slim Governance;
- current root/local `AGENTS.md`;
- current `BACKLOG.md`;
- current `main`;
- real dependency ancestry;
- Windows/native validation requirements.

Only that later finalization may establish canonical BL completion or authorize integration/cleanup.
