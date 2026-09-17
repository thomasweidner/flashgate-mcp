# FlashGate MCP — Agent Router

This file is the repository-visible router for Codex and other repository agents.

## Authority and required reading

Before implementation, review, or Mobile task selection, read the current repository state and then:

1. `Governance/CLOUD-CODEX-GOVERNANCE.md`
2. `Governance/CHANGE-TRIGGER-REVIEW-AND-BACKLOG-STANDARD.md`
3. `Governance/MOBILE-CLOUD-HANDOFF.md` for Codex Cloud / Mobile work
4. `BACKLOG.md`
5. `MOBILE.md` for Codex Cloud / Mobile work
6. directly affected ADRs, security documents, testing guidance, and technical documentation

Do not infer unavailable local files from memory.

The tracked Cloud governance is a bounded projection for repository-contained Cloud work. It does not replace the central local Slim Governance authority under `Codex-Work/Governance` or higher-level local `AGENTS.md` files. During Windows finalization, bind and apply the then-current local authorities in addition to the repository rules.

## Assignment start

For every material assignment:

- read the exact current BL row and status from `BACKLOG.md`;
- perform the change-trigger review;
- identify affected public contracts, security boundaries, platforms, dependencies, release impact, documentation, and permanent gates;
- identify real prerequisites and mandatory decision boundaries before implementation;
- preserve completed (`Done`) work as completed;
- do not broaden a BL task beyond one coherent reviewable acceptance boundary.

## Continuation and remediation

Continue independently through directly caused, in-scope corrections while the approved goal, authorization boundary, and remediation budget remain valid.

- established validated artifacts: at most 6 material correction/revalidation cycles;
- new or materially rebuilt artifact: at most 12 cycles;
- validate parser/focused root cause first, then the smallest sufficient focused checks, then one consolidated final validation;
- stop earlier on success, no progress, exhausted budget, ambiguous mutation state, or a mandatory decision/authorization boundary.

Do not claim independent approval of work you implemented or corrected.

## Mandatory decision stops

Do not convert an unresolved decision into an implementation assumption. Stop when proceeding requires a new:

- product decision;
- architecture decision;
- security decision;
- platform decision;
- dependency decision;
- release decision;
- scope decision.

## External and security boundary

External, Remote/GitHub/provider, credential, separately approval-bound Git, installation, permission/ACL/owner, system/service, scheduled-task/automation, irreversible, and non-exactly-reversible production/security actions are Hard-One-Shot boundaries.

Before a write-capable external action, complete every deterministic read-only/prewrite check that can be known in advance. The first write-capable attempt consumes that action authorization even if it fails. Do not automatically retry, switch provider, or fall back to a manual credential/remote path.

Force operations are prohibited unless a later explicit contract says otherwise.

## Codex Cloud / Mobile

For Mobile work, `Governance/MOBILE-CLOUD-HANDOFF.md` defines the vacation reservation ledger, dependency execution states, stack restart, and the manual Create-PR handoff. `MOBILE.md` defines task eligibility, dependency hints, and Windows finalization.

### Automatic Mobile task selection

When the user asks for the **next suitable** Mobile task without naming a BL task or epic, do not choose an arbitrary Cloud-suitable item from the full backlog.

1. Read the open-PR Vacation Reservation Ledger from GitHub exactly as defined in `Governance/MOBILE-CLOUD-HANDOFF.md`.
2. Exclude every task ID that the ledger marks reserved, including metadata-valid Mobile PRs and unambiguous provisional UI-generated PRs.
3. Read the current `Sprint sequence and status` table in `BACKLOG.md`.
4. Inspect `Planned` sprints in ascending `SPR-xxx` order.
5. For each candidate, first run the concrete-delta feasibility test from `Governance/MOBILE-CLOUD-HANDOFF.md`. A dependency hint in `MOBILE.md` is advisory and does not by itself make a task stack-required.
6. Within each sprint, first classify from the **current checkout**. If at least one unreserved candidate is `INDEPENDENT_FROM_CURRENT_CHECKOUT` or `STACK_BASE_READY`, select from that sprint and prefer mode `A` before `B` before `C`, then independent before stack-base-ready, then lower effort, lower Windows residual, and lower expected branch/diff collision.
7. If that sprint has no current-checkout executable candidate, perform the read-only **open-PR-head restart pass** from `Governance/MOBILE-CLOUD-HANDOFF.md` before advancing to the next sprint. For each remaining candidate, determine its complete hard-predecessor delta set, collect every open PR head ancestry that already contains all of it, collapse comparable heads in the same ancestry lineage, and retain the minimal sufficient head in each lineage. One cumulative PR ancestry may satisfy several logical prerequisite BLs.
8. A candidate is unambiguously restartable when exactly one minimal sufficient lineage head remains after that reduction. If one or more candidates in the sprint are unambiguously restartable, choose among those candidates using the same mode/effort/Windows-residual/collision preferences and return `STACK_RESTART_REQUIRED` with the selected predecessor PR, head branch, head SHA, child BL, and exact restart instruction. Do **not** require the sprint to contain only one restartable candidate.
9. If a candidate has multiple incomparable minimal sufficient heads and each independently contains the complete hard-predecessor set, its `DependencyExecution` remains `STACK_REQUIRED`; classify only the restart-base choice as `STACK_RESTART_BASE_AMBIGUOUS`, exclude that candidate from automatic choice, and continue. This is not `BLOCKED_MULTIPLE_UNCOMBINED_MOBILE_PREDECESSORS` because no branch combination is required. If the earliest sprint has no executable or unambiguous restartable candidate but still has such ambiguity, report the ambiguity and candidate heads instead of a global no-executable result.
10. If a candidate genuinely requires two or more independent uncombined PR heads because no single head contains the complete hard-predecessor set, classify it `BLOCKED_MULTIPLE_UNCOMBINED_MOBILE_PREDECESSORS`, exclude only that candidate, and continue. If the sprint has neither a current-checkout executable candidate, an unambiguous single-line restart candidate, nor a restart-base ambiguity requiring user choice, continue with the next Planned sprint.
11. Do not attempt `git fetch`, manual remote configuration, merge, cherry-pick, patch replay, or synthetic predecessor import to make a `STACK_REQUIRED` candidate executable.
12. Return a global no-executable Planned result only after every allowed Planned sprint has completed both the current-checkout pass and, where needed, the open-PR-head restart pass, with no unresolved restart-base ambiguity that should be surfaced instead.
13. Do not auto-select `Later` work while any executable or unambiguously restartable unreserved `Planned` Mobile task exists.
14. A user-named task or epic explicitly overrides this automatic sprint preference, but a user-named task that is already reserved must be reported rather than duplicated unless the user explicitly authorizes a competing candidate.

A Mobile dependency is hard only when the candidate's concrete task-pure delta actually consumes unintegrated predecessor code, types, schemas, contracts, or runtime behavior. Do **not** infer a hard dependency merely because the BL acceptance notes mention future consumers, later CI coverage, later end-to-end integration, Windows/native finalization, or evidence that will only become complete after other PRs integrate. A generic gate that automatically covers future packages may be prepared independently when its own implementation and focused validation are complete against the current checkout. Record the remaining integration/native evidence as deferred; do not use that deferral to claim the BL `Done`.

`BLOCKED_MULTIPLE_UNCOMBINED_MOBILE_PREDECESSORS` is candidate-local. Exclude that candidate and continue evaluating other candidates and later Planned sprints. A reserved predecessor PR may still be a valid read-only-discovered start ref for an unreserved child; reservation prevents duplicate implementation of the predecessor, not legitimate stacking on its head. Multiple sufficient heads in one ancestry lineage are not an ambiguity after reduction: keep the earliest/minimal sufficient head whose ancestry already contains the complete hard-predecessor set. Multiple incomparable sufficient heads leave the candidate `STACK_REQUIRED`; they make only the restart-base selection ambiguous.

Multiple independent tasks from the selected sprint may still be prepared in parallel. Real dependency chains use a new Codex Cloud task started directly from the predecessor PR head branch or commit supplied by `STACK_RESTART_REQUIRED`; they are not assembled by fetching the predecessor into a `main`-based task.

For the default phone prompt, use **"Führe den nächsten geeigneten Cloud-Task aus `MOBILE.md` V3 aus"** rather than "einen geeigneten".

### Manual PR handoff

Codex Cloud is not required to create the real GitHub PR itself.

After implementation, validation, and commit:

- prepare the PR title/body metadata with the selected `BL-xxx` identity and the Mobile V3 markers;
- report `CLOUD_IMPLEMENTATION_COMPLETE_AWAITING_MANUAL_PR`;
- provide `TaskID`, `ActualHeadBranch`, `CloudCommitSha`, `CloudTreeSha`, expected PR base, and prepared PR title;
- obtain `CloudTreeSha` from the committed Cloud worktree, for example with `git rev-parse 'HEAD^{tree}'`;
- for a stacked child, `ExpectedPRBase` is the direct predecessor Mobile branch from which the Cloud task was started;
- instruct the user to use the Codex UI **Create PR** action;
- do not require the PR number before the user performs that action.

When the user replies that the PR was created, perform the read-only GitHub correlation defined in `Governance/MOBILE-CLOUD-HANDOFF.md`.

- First resolve exactly one open PR to the selected `TaskID` using its Mobile marker or unambiguous BL identity. That immediately reserves the BL task for the vacation workflow.
- Do **not** require the GitHub PR head commit SHA to equal `CloudCommitSha`; the UI publication may rematerialize an equivalent commit.
- Then fetch the GitHub head commit and compare its `tree.sha` with `CloudTreeSha` to validate content identity.
- A matching tree with a different commit SHA is accepted with `CommitShaRematerialized=true`.
- A tree mismatch or unavailable tree proof blocks content acceptance but does **not** remove the reservation and must never cause the same BL task to be selected again automatically.
- Branch-name differences are diagnostic unless they create ambiguous task identity or an invalid stacked base.

The PR number is discovered from GitHub after publication; it is not a prerequisite supplied by the user.

- no manual `git remote` configuration;
- no PAT/token/SSH/credential workaround;
- no agent-driven GitHub write as a substitute for the UI Create-PR action;
- no predecessor `git fetch`, merge, cherry-pick, or synthetic stack construction inside a Mobile task;
- no PR merge or close;
- no branch deletion;
- no tag or release;
- no repository-settings or secrets changes;
- do not mark the BL task `Done`;
- do not fabricate Windows, WSL, SCM, systemd, native-host, hardware, identity, permission, or credential evidence.

Windows finalization remains required for every Mobile PR.

## Toolchain

PowerShell target standard for FlashGate is PowerShell 7.6.5 via `pwsh`.

If required tooling is unavailable in Codex Cloud, do not silently install or substitute a different authoritative environment. Run the checks that are genuinely available and record the remainder for Windows/native finalization.
