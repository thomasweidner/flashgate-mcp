# FlashGate Codex Cloud Governance Capsule

**Status:** Binding for repository-contained Codex Cloud execution  
**Model:** `SLIM_CLOUD_PROJECTION`  
**Project:** `thomasweidner/flashgate-mcp`  
**Local authority retained:** current central Slim Governance under `Codex-Work/Governance`  
**Purpose:** make the minimum security, authorization, continuation, review, change-trigger, and handoff boundaries available when the local Windows governance tree is not mounted.

## 1. Authority boundary

This file is a Cloud execution projection. It is not a second global governance authority and it does not supersede the central local Slim Governance.

For Codex Cloud, use this repository authority order:

1. repository root `AGENTS.md`;
2. this `Governance/CLOUD-CODEX-GOVERNANCE.md`;
3. `Governance/CHANGE-TRIGGER-REVIEW-AND-BACKLOG-STANDARD.md`;
4. current `BACKLOG.md`;
5. `MOBILE.md` when the assignment is Mobile work;
6. directly affected technical/security/testing/ADR documentation.

When Windows/local governance is available again, rebind the candidate against the then-current central governance and applicable higher-level/local `AGENTS.md`. Any stricter or newer local rule wins during finalization.

Never claim that a Cloud PR proves local-governance, Windows, WSL, SCM, systemd, ACL, identity, credential, or native-host completion.

## 2. Slim model and legacy boundary

Normal FlashGate product work uses Slim Governance.

- reuse current technical gates and unchanged evidence;
- validate directly affected areas first;
- risk determines review/validation depth;
- completed backlog owners remain `Done`;
- Heavy Generic Handoff, Commit Preparation, Governance Publication, V3/V4 meta-orchestration, and historical fixture contracts are `LEGACY_COMPATIBILITY_ONLY` unless a current repository consumer explicitly requires them;
- do not create a new consumer of a Heavy/legacy governance contract merely because the file still exists.

Security, identity, credentials, external/remote mutation, destructive action, and technical reproducibility boundaries remain fully active.

## 3. Change-trigger review

At assignment start, on material scope change, before candidate publication, and at any release boundary:

1. read the current canonical BL row;
2. classify affected domains, public contracts, security boundaries, platforms, artifacts, dependencies, documentation, and operational assumptions;
3. map the change to existing permanent gates and open backlog work;
4. search for duplicate/equivalent backlog coverage;
5. reuse existing gates when the accepted contract is unchanged;
6. keep new requirements inside the current task only when they remain within its accepted coherent scope;
7. stop rather than silently create or broaden work when a new independent decision or owner is required.

Codex Cloud does not autonomously assign a new BL identifier. If genuinely new independently owned work is required, return:

`Status=BLOCKED_NEW_BACKLOG_REGISTRATION_REQUIRED`

Include a proposed scope and evidence, but leave canonical registration to an explicitly authorized repository/governance action.

## 4. Mandatory decision boundaries

Stop with `BLOCKED_DECISION_REQUIRED` when proceeding requires a new:

- `PRODUCT_DECISION`;
- `ARCHITECTURE_DECISION`;
- `SECURITY_DECISION`;
- `PLATFORM_DECISION`;
- `DEPENDENCY_DECISION`;
- `RELEASE_DECISION`;
- `SCOPE_DECISION`.

A decision boundary is not an implementation assumption.

## 5. Same-assignment continuation

A task-scoped implementation authorization includes directly caused, directly adjacent corrections that:

- stay inside the accepted BL outcome;
- remain inside the authorized repository/write boundary;
- are testable with the current toolchain;
- do not cross a mandatory decision boundary.

Correct and revalidate those findings in the same assignment rather than returning one correctable defect at a time.

Remediation ceilings:

- established, previously validated artifact: maximum 6 material correction/revalidation cycles;
- new or materially rebuilt artifact: maximum 12 cycles.

A cycle is one material correction followed by the smallest sufficient focused validation. The ceiling is not a target.

Stop on:

- validation success;
- exhausted budget;
- repeated root cause/no progress;
- scope expansion;
- new decision/authorization boundary;
- ambiguous mutation state;
- unproven rollback/preimage parity after a canonical write failure.

## 6. Validation funnel

Prefer:

1. parser/static/focused root-cause checks;
2. directly affected unit/integration/security tests;
3. affected documentation/contract checks;
4. one consolidated repository-appropriate final validation;
5. complete final diff/path review;
6. `git diff --check` when local Git is available;
7. Hosted CI after managed PR publication.

Do not rerun large unaffected matrices only for ceremony. Reuse unchanged valid evidence.

PowerShell target standard is 7.6.5 through `pwsh`. If a required Windows/native validation environment is unavailable in Cloud, do not fabricate or silently replace it; defer that validation explicitly.

## 7. Hard-One-Shot boundary

The following are Hard-One-Shot unless a later explicit authority says otherwise:

- external/remote/GitHub/provider writes;
- credentials, PATs, tokens, SSH keys, auth configuration;
- separately approval-bound Git mutations;
- installs/dependency installation outside the accepted task contract;
- permission/ACL/owner changes;
- system/service/scheduled-task/automation changes;
- irreversible or non-exactly-reversible production/security actions.

Before the first write-capable invocation:

- complete every deterministic read-only/prewrite check that can be known;
- bind the intended action, target, current state, and task scope.

The first write-capable attempt consumes the corresponding authorization even when it fails.

After a failed write-capable attempt:

- no automatic retry;
- no provider switch;
- no manual credential/remote fallback;
- no blind identical attempt;
- report the terminal state and return for a new decision.

After a successful write-capable external action, read back the resulting state and verify the intended transition.

## 8. Codex Cloud GitHub publication

For Mobile work, a task prompt may authorize exactly one managed open-PR publication attempt.

The managed publication path may create the task branch/commit/PR as required by the platform. Codex must not configure a manual Git remote or credentials as a substitute.

Before publication verify:

- selected BL task is still open (`Planned` or explicitly permitted `Later`);
- current base/parent Mobile PR is valid;
- final diff is task-pure;
- no `MOBILE.md`, governance-capsule, or unrelated steering mutation is mixed into a product task;
- required Cloud-available validations have passed;
- deferred Windows/native validations are listed.

On success:

- PR remains open;
- read back PR base/head/state/scope when available;
- `Merge-Allowed: NO`;
- Windows finalization required.

Prohibited during Mobile work:

- merge;
- PR close;
- branch delete;
- force update;
- tag/release;
- repository settings/rules/secrets mutation.

## 9. Backlog and documentation truth

Cloud candidates are not canonical completion.

- do not change a task to `Done`;
- do not claim local integration;
- do not add completion-only changelog/status wording;
- update technical docs/tests within the task when required for internal consistency;
- preserve historical evidence as history;
- if the task exposes required work outside scope, report the boundary instead of hiding it.

## 10. Platform and security truth

Never claim evidence that was not executed in the actual environment.

Examples that require later real/local validation when applicable:

- Windows filesystem/reparse/ACL behavior;
- WSL routing;
- Windows SCM;
- Task Manager / Windows service identity;
- systemd/service accounts;
- OS caller identity and peer credentials;
- native ARM64 execution;
- real signing credentials;
- release/provider credentials;
- productive permissions or service installation.

Cloud may implement code, tests, documentation, fixtures, and cross-build logic for those domains when the BL task permits it, but the final result remains `Windows-Finalization: REQUIRED`.

## 11. Independent review

An implementing/correcting actor may validate its work but may not claim independent approval of it.

Where an independent review is required by the current task or Windows finalization process, it remains read-only and separate.

Do not resurrect legacy Heavy independent-review packaging as a default Mobile requirement.

## 12. Windows rebind

Every open Mobile PR must be rebound on return to Windows against:

- current central `Codex-Work/Governance`;
- applicable higher-level/local `AGENTS.md`;
- current `BACKLOG.md`;
- current `main`;
- current platform/toolchain state;
- all real prerequisite Mobile branches/PRs.

Only Windows/local finalization may establish canonical task completion and authorize integration/cleanup.
