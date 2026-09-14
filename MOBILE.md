# FlashGate MCP — Mobile / Codex Cloud Work Catalog

## 1. Purpose and authority

`MOBILE.md` defines the FlashGate work that may be prepared in Codex Cloud while the Windows development host is unavailable.

It is a **derived Cloud execution catalog**, not a second backlog authority.

Canonical authority remains:

1. repository root `AGENTS.md`;
2. `Governance/CLOUD-CODEX-GOVERNANCE.md`;
3. `Governance/CHANGE-TRIGGER-REVIEW-AND-BACKLOG-STANDARD.md`;
4. current repository `BACKLOG.md`;
5. `CONTRIBUTING.md`, ADRs, security/testing documentation, and the affected code;
6. this file only for Cloud eligibility, task selection, dependency stacking, publication, and Windows handoff.

`MOBILE.md` cannot weaken or override `AGENTS.md` or the Cloud governance capsule.

If this file conflicts with a canonical source, the canonical source wins. Stop with `STALE_MOBILE_CATALOG` when the conflict affects scope, dependencies, security, acceptance, or publication.

Do not create new BL IDs, change sprint assignment, change `Planned`/`Later` milestone semantics, or mark a task `Done` from the mobile workflow.

### Protocol

```text
Mobile-Queue: FLASHGATE-MOBILE-V3
Publication: CODEX_UI_MANUAL_CREATE_PR
Merge-During-Mobile: NO
Windows-Finalization: REQUIRED
```

## 2. Cloud governance activation

Before task discovery or implementation, verify that the selected repository/base contains:

- root `AGENTS.md`;
- `Governance/CLOUD-CODEX-GOVERNANCE.md`;
- `Governance/CHANGE-TRIGGER-REVIEW-AND-BACKLOG-STANDARD.md`;
- `Governance/MOBILE-CLOUD-HANDOFF.md`;
- current `BACKLOG.md`;
- this `MOBILE.md`.

Read the governance sources before selecting work.

If the tracked Cloud governance router or capsule is absent, unreadable, or internally contradictory:

`Status=BLOCKED_CLOUD_GOVERNANCE_UNAVAILABLE`

Do not compensate by assuming the missing local `Codex-Work/Governance` content from memory.

The Cloud capsule is intentionally a bounded projection. Every candidate is rebound against the complete current local governance during Windows finalization.

## 3. Core model — no mandatory global implementation order

There is **no mandatory global implementation order** for user-named Mobile tasks.

Tasks may be:

- implemented independently from current `main`;
- implemented in parallel;
- prepared before lower-numbered tasks;
- stacked on another open Mobile task when the implementation genuinely consumes that task.

A task is blocked only by a real canonical/technical dependency, missing required source/evidence, a decision boundary, or a Cloud capability boundary.

### Default selection when the user says only "next task"

Automatic selection is governed by root `AGENTS.md` and `Governance/MOBILE-CLOUD-HANDOFF.md`:

1. enumerate the open-PR Vacation Reservation Ledger;
2. exclude every reserved BL identity, including unambiguous provisional UI-generated PRs;
3. inspect `Planned` sprints in ascending `SPR-xxx` order;
4. classify every relevant candidate from the **current checkout** as `INDEPENDENT_FROM_CURRENT_CHECKOUT`, `STACK_BASE_READY`, `STACK_REQUIRED`, or blocked;
5. select from the earliest Planned sprint that contains an executable unreserved candidate;
6. inside that sprint prefer mode `A` before `B` before `C`, then independent before stack-base-ready, then lower effort, lower Windows residual, and lower expected collision;
7. if the earliest relevant sprint has no executable candidate and the next actionable path is one exact predecessor stack, return `STACK_RESTART_REQUIRED` rather than silently skipping to later work;
8. do not auto-select `Later` work while executable unreserved `Planned` work exists.

The user may name any eligible task or epic and override the automatic sprint preference, but a reserved task is never duplicated without explicit authorization for a competing candidate.

## 4. Cloud modes

| Mode | Meaning | Mobile completion |
|---|---|---|
| `A — CLOUD_IMPLEMENTABLE` | Repository-contained code/docs/tests can be substantially implemented and validated in Cloud | Task-pure Cloud commit; manual Create-PR handoff; Windows finalization required |
| `B — CLOUD_IMPLEMENTABLE_PLATFORM_FINAL` | Meaningful implementation is possible in Cloud, but Windows/native-host/service/real-OS validation is essential | Task-pure Cloud commit with explicit deferred validations; manual Create-PR handoff |
| `C — CLOUD_ANALYSIS_OR_CONTRACT` | Cloud can perform architecture, threat-model, design, investigation, or draft work, but an owner/external decision may be required before implementation/final completion | PR only if the canonical task permits a bounded documentation/design delta; otherwise return analysis without PR |
| `D — CONTINUOUS_OR_FINAL_GATE` | Valid Cloud work may contribute, but the BL item is continuous, release-wide, or integration-wide and should not normally own a vacation PR by itself | Consume from other tasks; do not auto-select standalone |
| `X — NOT_AUTONOMOUS_MOBILE` | Required evidence/authority is unavailable or the work is primarily an external/local decision/action | Do not auto-select |

`A` does **not** mean Windows validation can be skipped. It means the Cloud candidate itself can be meaningfully completed.

## 5. Repository and publication boundary

A local Git remote is not required.

`no Git remote configured` is non-blocking when Codex Cloud can:

- read `thomasweidner/flashgate-mcp`;
- read the relevant base branch or predecessor ref selected when the Cloud task starts;
- inspect current GitHub pull-request state through the managed repository integration;
- prepare a task-pure commit and the PR metadata required for the Codex UI **Create PR** action.

Do not configure:

- `git remote`;
- PAT/token credentials;
- SSH keys;
- GitHub Apps;
- repository settings;
- alternate network or credential workarounds.

Do not use an agent-driven GitHub write as a substitute for the Codex UI Create-PR handoff.

### Manual Create-PR handoff

After implementation, validation, and a task-pure Cloud commit, prepare:

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

Obtain `CloudTreeSha` from the committed worktree, for example with:

```text
git rev-parse 'HEAD^{tree}'
```

The user performs the Codex UI **Create PR** action. Do not require a PR number in advance.

### Durable GitHub publication boundary

GitHub open pull requests are the Vacation Reservation Ledger. A Cloud task result, local Cloud commit, diff view, or Cloud-internal branch indication is not a durable reservation by itself.

Use these states:

```text
CLOUD_IMPLEMENTATION_IN_PROGRESS
CLOUD_IMPLEMENTATION_COMPLETE_AWAITING_MANUAL_PR
MANUAL_PR_CREATED_PENDING_GITHUB_VISIBILITY
CLOUD_IMPLEMENTATION_COMPLETE_PR_OPEN
```

After the user reports that the PR was created:

1. enumerate the open GitHub PRs again;
2. resolve exactly one PR to the selected BL identity using the reservation rules in `Governance/MOBILE-CLOUD-HANDOFF.md`;
3. record the reservation before content-identity checking;
4. fetch the GitHub head commit read-only and compare its tree SHA with `CloudTreeSha`;
5. accept commit-SHA rematerialization when the tree SHA matches;
6. verify the expected base, especially for stacked children;
7. keep the task reserved even when content identity is unavailable or mismatched; never auto-reimplement it.

A fully accepted candidate returns `CLOUD_IMPLEMENTATION_COMPLETE_PR_OPEN`. A base mismatch, ambiguous reservation, or content mismatch remains fail-closed and requires a new decision; do not automatically mutate, retarget, close, recreate, or merge the PR.

## 6. Dynamic task discovery and reservation

Before selecting work:

1. confirm `AGENTS.md`, Cloud governance, and `Governance/MOBILE-CLOUD-HANDOFF.md` were read for this task;
2. read current `BACKLOG.md`;
3. consider only canonical rows with status `Planned` or explicitly permitted `Later`;
4. map the BL ID through the capability catalog;
5. enumerate all open GitHub PRs and build the Vacation Reservation Ledger;
6. inspect real dependencies from the backlog, current checkout, contracts, and existing Mobile PRs;
7. determine effort and likely Windows residual from the current repository state;
8. select an eligible candidate using the root `AGENTS.md` sprint-first preference unless the user named one.

### Reservation identity

A PR reserves a BL task when either:

- its body contains an exact standalone `Mobile-Task: BL-xxx` marker; or
- no marker exists but exactly one selected-task BL identity is unambiguous from the title/body according to `Governance/MOBILE-CLOUD-HANDOFF.md`.

Canonical Mobile metadata remains preferred, but UI-generated formatting differences do not unreserve a task when its selected BL identity is unambiguous.

If title/body identity conflicts, or several BL identities are ambiguous, stop with the applicable Mobile correlation/identity status and treat every plausibly implicated task as requiring review.

Reservation is independent of commit SHA, branch name, tree SHA, author, committer, or timestamp. Content identity is a separate post-publication gate.

## 7. Dependency and stacked-PR contract

Dependencies are **not** a reason to omit a task from `MOBILE.md`.

### 7.1 Independent task

If the task does not consume an unintegrated Mobile predecessor:

```text
task branch base = current main
PR base          = main
Mobile-Depends-On: NONE
```

### 7.2 Stack-base-ready task

A dependent task is executable only when the current Cloud checkout already contains the exact predecessor Mobile implementation it consumes. The normal route is to start a **new Codex Cloud task directly from the predecessor PR head branch or head SHA**.

Before child implementation, verify locally that the selected predecessor is present. When the parent SHA is available locally, use:

```text
git merge-base --is-ancestor <ParentHeadSha> HEAD
```

For a valid child:

```text
PR base          = <direct predecessor head branch>
Mobile-Depends-On: <parent BL>
```

### 7.3 Stack required from a main-based task

If a candidate genuinely needs exactly one open predecessor that is not present in the current checkout:

```text
Status=STACK_RESTART_REQUIRED
```

Do not make it executable with `git fetch`, `git pull`, merge, cherry-pick, patch replay, downloaded Git objects, a synthetic merge branch, or credential/remote configuration.

Return the parent PR, head branch, head SHA, child BL, and instruct the user to start a new Codex Cloud task from that predecessor ref.

### 7.4 Longer chain

A chain is allowed:

```text
main -> BL-A -> BL-B -> BL-C
```

Each child task must start from its direct predecessor and each PR must target that direct predecessor branch.

### 7.5 Multiple independent prerequisites

Do not automatically synthesize a merge branch from two unrelated open Mobile PRs.

If a task genuinely requires more than one independent uncombined predecessor:

```text
Status=BLOCKED_MULTIPLE_UNCOMBINED_MOBILE_PREDECESSORS
```

Choose another eligible task or defer combination to Windows unless the user explicitly authorizes another strategy.

### 7.6 Dependency truth

Dependency hints in this file are advisory planning hints. They do not create a Git ancestry requirement. Before stacking, verify the real dependency from the current code/contracts.

## 8. Per-task authorization

Each Mobile task needs a fresh task-scoped instruction.

Preferred automatic phone instruction:

> Führe den nächsten geeigneten Cloud-Task aus `MOBILE.md` V3 aus. Implementierung, task-purer Commit und Cloud-validierbare Tests sind für genau diesen Task freigegeben. Reale Abhängigkeiten dürfen nur über einen neuen Cloud-Task konsumiert werden, der direkt vom Vorgänger-PR-Head gestartet wurde. Nach Abschluss PR-Metadaten für den manuellen Codex-UI-`Create PR`-Schritt vorbereiten und mit `CLOUD_IMPLEMENTATION_COMPLETE_AWAITING_MANUAL_PR` stoppen. Kein Merge, kein PR-Close, kein Branch-Delete und keine manuelle Remote- oder Credential-Konfiguration.

The instruction authorizes for exactly one selected task:

- read-only discovery and dependency analysis;
- implementation or bounded design work allowed by its Cloud mode;
- up to 6 material in-scope correction cycles;
- Cloud-available validation;
- a task-pure Cloud commit;
- PR title/body metadata preparation for the manual Codex UI Create-PR handoff.

It does not authorize:

- merging or closing any Mobile PR;
- deleting branches;
- tags/releases;
- GitHub settings/rules/secrets;
- agent-driven GitHub writes as a substitute for the user's UI Create-PR action;
- manual remote/credential configuration;
- predecessor fetch/pull/merge/cherry-pick;
- installations or dependencies not separately approved;
- implementation of another BL task merely because it is nearby.

## 9. Common implementation contract

For every task:

1. re-read its canonical BL row;
2. inspect affected code, tests, docs, ADRs, security and change-trigger rules;
3. enumerate the current Vacation Reservation Ledger;
4. identify actual prerequisites and classify dependency execution from the current checkout;
5. do not silently cross a product/security/architecture decision boundary;
6. keep the canonical backlog status unchanged;
7. do not mark Cloud work as canonical completion;
8. do not fabricate Windows/WSL/systemd/SCM/native-host evidence;
9. keep the candidate branch task-pure except for directly caused in-scope corrections;
10. run focused tests before broad validation;
11. inspect the complete final diff and `git diff --check` when local Git supports it;
12. commit the task-pure Cloud candidate;
13. record `CloudCommitSha` and `CloudTreeSha`;
14. prepare the canonical PR title/body and expected base;
15. return `CLOUD_IMPLEMENTATION_COMPLETE_AWAITING_MANUAL_PR` and stop for the user's Codex UI Create-PR action;
16. only after the user reports `PR erstellt`, perform the read-only GitHub correlation/content-identity/base checks from `Governance/MOBILE-CLOUD-HANDOFF.md`.

If a product defect is discovered outside direct scope:

```text
Status=BLOCKED_PRODUCT_FINDING_REQUIRED
```

If a new decision is required:

```text
Status=BLOCKED_DECISION_REQUIRED
```

## 10. Complete Cloud capability catalog

Every current `Planned` or `Later` row in the ranges below is represented. `Done` rows are never re-opened.

### 10.1 Filesystem epic — BL-036–BL-067

**All `BL-036–BL-067` are Mobile eligible.**

| IDs | Mode | Notes |
|---|---|---|
| `BL-036–BL-057` | `A` | MCP tests, pagination/cursors, listing, range reads, metadata, streaming/read contracts, batch operations, targeted edits, atomic/conditional/dry-run/append/plans |
| `BL-058` | `B` | Cross-volume semantics can be coded/tested synthetically; real Windows/filesystem finalization required |
| `BL-059` | `A` | Conflict strategy |
| `BL-060–BL-061` | `B` | Directory move/copy/size; long-running/job/platform behavior needs Windows/native finalization |
| `BL-062–BL-065` | `A` | Disk usage, write modes, job integration, plan threat model; `BL-064` may stack on Job Manager |
| `BL-066–BL-067` | `B` | Cross-platform integration tests and representative benchmark corpus need later host validation |

Dependency hints:

- cursor/listing cluster: `BL-038` → `BL-037` → `BL-039/040/049`;
- read cluster: `BL-041/042/043/045` feed `BL-044/046/047/048`;
- write cluster: `BL-050/051/052/053/054/055/059` feed `BL-056/057/060/063/065`;
- long-running filesystem work may consume `BL-084–099` through `BL-064`.

### 10.2 Search epic — BL-068–BL-083

| IDs | Mode | Notes |
|---|---|---|
| `BL-068–BL-080`, `BL-082` | `A` | Entire pure-Go baseline search workstream is Cloud-suitable |
| `BL-081` (`Later`) | `B` | Optional ripgrep adapter; external binary/version/security final validation deferred |
| `BL-083` (`Later`) | `C` | Index decision requires benchmark/privacy/lifecycle evidence |

Dependency hints:

- `BL-068` is the search model/threat-model foundation;
- `BL-082` is the portable pure-Go baseline;
- path/name/metadata/text/regex/filter/limit/context/binary work can be split into separate Mobile branches where contracts permit;
- `BL-079` pagination should consume the stable ordering semantics produced by the search implementation it paginates.

### 10.3 Operations and Job Manager — BL-084–BL-099

**All `BL-084–BL-099` are Mobile eligible.**

| IDs | Mode | Notes |
|---|---|---|
| `BL-084–BL-097` | `A` | Registry, handles, states, cancellation, deadlines, counters, result TTL, cleanup, quotas, fairness, shutdown, leak handling, domain ownership, execution rules |
| `BL-098` | `B` | Cross-platform integration suite; Cloud can implement/run available Linux tests, Windows finalization required |
| `BL-099` | `B` | Security/race coverage can run in Cloud where available; Windows race/lifecycle finalization remains |

Suggested stack spine when useful:

`BL-084/086` → `BL-085` → `BL-087/088/089` → `BL-090/091` → `BL-092/093` → `BL-094/095` → `BL-098/099`.

This is not a mandatory global order.

### 10.4 Named roots, capabilities and profiles — BL-100–BL-112

| IDs | Mode | Notes |
|---|---|---|
| `BL-100–BL-111` | `A` | Capability model, named roots, root IDs, profiles, per-root policy/limits/types/reparse/capabilities/process working directory, dynamic registration, negative tests |
| `BL-112` (`Later`) | `C` | Legacy MCP Roots only after a supported-client compatibility decision |

Dependency hints:

`BL-100/101` are natural foundations; `BL-102–109` layer root/profile policy; `BL-110/111` consume the effective model. Security enforcement/testing connects to `BL-159–161`.

### 10.5 Process epic — BL-113–BL-135

| IDs | Mode | Notes |
|---|---|---|
| `BL-113–BL-126`, `BL-129–BL-133` | `A` | Threat model, observation contracts, registry, handles, lifecycle, start/wait/output/buffers/stop, cleanup, limits and redaction can be developed in Cloud |
| `BL-127–BL-128` (`Later`) | `C` | External PID control / process input require explicit high-risk policy decisions |
| `BL-134–BL-135` | `B` | Windows/Linux adapters and lifecycle/race/restart tests require platform finalization |

Natural foundations:

- observation: `BL-113/162` → `BL-114–118`;
- managed processes: `BL-119–126` → `BL-129–135`;
- process authorization consumes capabilities/roots from `BL-100–111` and security owners.

### 10.6 Command execution — BL-136–BL-152

| IDs | Mode | Notes |
|---|---|---|
| `BL-136–BL-145`, `BL-148–BL-149`, `BL-151` | `A` | Threat model, typed commands, no-shell args, roots, timeouts, output, result schema, managed-engine wrapper, environment/redaction, budgets, identity, single-engine invariant |
| `BL-146` | `B` | Windows isolation requires Windows final validation |
| `BL-147` | `B` | Linux isolation can be developed in Cloud but still needs native/security final validation |
| `BL-150` (`Later`) | `C` | Interactive shell remains a separate high-risk decision |
| `BL-152` | `B` | Complete Windows/Linux execution security matrix requires both platforms |

`BL-143` must consume the Managed Process Engine rather than create a second engine.

### 10.7 System information — BL-153–BL-158

| IDs | Mode | Notes |
|---|---|---|
| `BL-153–BL-157` | `A` | Controlled system info, scoped disk usage, environment filtering, field selection/redaction, `system.read` |
| `BL-158` (`Later`) | `C` | Restricted network information requires privacy-sensitive decision |

`BL-154` consumes `BL-062`.

### 10.8 Security — BL-159–BL-173

| IDs | Mode | Notes |
|---|---|---|
| `BL-159–BL-167`, `BL-170–BL-173` | `A` | Server-side capability enforcement/tests, policy models, job security, threat models, audit, redaction, residual-risk docs, annotation negatives, workflow pinning, security policy |
| `BL-168` | `B` | Least-privilege validation requires real server/child-process platform evidence |
| `BL-169` (`Later`) | `C` | Provider security boundaries depend on the post-1.0 provider contract |

Security rows may stack on the domain implementation they secure.

### 10.9 Open-source governance and provider ecosystem — BL-176–BL-188

| IDs | Mode | Notes |
|---|---|---|
| `BL-176` (`Later`) | `C` | Factual license/distribution compatibility analysis only; no autonomous legal conclusion |
| `BL-177–BL-179` | `C` | Governance, maintainer and contribution rules can be drafted/researched, but owner decisions may be required |
| `BL-180` (`Later`) | `A` | Code of Conduct documentation |
| `BL-181–BL-188` (`Later`) | `C` | Provider contract, identifiers, metadata, capabilities, classifications, trust/distribution and runtime model are architecture/decision work |

These are Cloud-eligible architecture/documentation tasks but should not be silently converted into product implementation.

### 10.10 Efficiency and MCP contracts — BL-202–BL-220

| IDs | Mode | Notes |
|---|---|---|
| `BL-202–BL-203`, `BL-205–BL-206`, `BL-212–BL-214`, `BL-216`, `BL-218–BL-219` | `A` | Repo-contained metadata/error/tests/docs/schema/payload/efficiency/resource/catalog work |
| `BL-204` | `C` | Official conformance tooling may require current external tooling/evidence |
| `BL-207–BL-208` | `C` | Protocol matrix and extension/stateless strategy are contract/architecture work |
| `BL-209` | `C` | Final MCP Tasks compatibility is an explicit decision |
| `BL-210` | `A` | Mapping implementation/design once Operations/Job and Tasks decision are available |
| `BL-211` | `C` | Fallback contract decision |
| `BL-215` | `C` | Catalog/initialization budgets require justified Version-1.0 thresholds |
| `BL-217` (`Later`) | `A` | Conditional/not-modified contracts after fingerprint/cache foundations |
| `BL-220` | `C` | Native adapter/no-interpreter policy; implementation only after contract is clear |

Important cross-dependencies include `BL-084–099`, `BL-048`, and result-resource work.

### 10.11 Native multi-mode runtime and service deployment — BL-221–BL-244

| IDs | Mode | Notes |
|---|---|---|
| `BL-221–BL-223`, `BL-225`, `BL-233`, `BL-235`, `BL-238` | `C` | Architecture, lifecycle, IPC/configuration and execution-identity contracts |
| `BL-224`, `BL-228`, `BL-236`, `BL-239`, `BL-243` | `A` | Transport-neutral refactoring, proxy, interfaces/context binding and documentation are strong Cloud candidates |
| `BL-226` | `B` | Windows Named Pipe implementation; Windows ACL/caller identity finalization required |
| `BL-227` | `B` | Unix socket implementation; native peer-credential/service finalization required |
| `BL-229` | `B` | Auto discovery/fallback implementation with later cross-mode host validation |
| `BL-230` | `B` | Windows SCM service host can be coded/cross-built in Cloud; real SCM/service/identity/Task Manager validation is Windows-only |
| `BL-231` | `B` | systemd assets/host can be prepared; real systemd/account/hardening lifecycle requires native host |
| `BL-232` (`Later`) | `B` | User-scoped Windows/Linux background modes |
| `BL-234`, `BL-237` | `B` | Real OS caller/service-account identity and ACL enforcement need platform validation |
| `BL-240` (`Later`) | `B` | Per-user worker backend needs real OS identity/isolation finalization |
| `BL-241–BL-242` | `B` | Integrated lifecycle/platform matrix and all-mode CI/release validation |
| `BL-244` | `B` | Harness/benchmark work can be prepared, but authoritative thresholds require real modes/hosts |

Natural stack:

`BL-221–225` contracts → transports/proxy/discovery → service hosts/identity → lifecycle matrix/CI/benchmarks.

Parallel work inside that structure is allowed when it does not consume unfinished contracts.

### 10.12 CI, release and quality — BL-245, BL-249–BL-263

| IDs | Mode | Notes |
|---|---|---|
| `BL-245`, `BL-249–BL-250`, `BL-252`, `BL-255–BL-259` | `A` | Workflows, benchmark comparison, race gates, artifact-name/schema/payload/catalog checks, legacy-name scan |
| `BL-253–BL-254` | `B` | Windows/Linux process and Operations/Job CI depend on their implementation suites |
| `BL-260` | `D` | Continuous standard gate; normally changed only when another task legitimately triggers it |
| `BL-261` | `B` | Cross-project benchmark can be prepared, but authoritative identical-host/external-server evidence is deferred |
| `BL-262` | `B` | SBOM/provenance/checksum plans can be prepared; signing/credential/atomic-release evidence remains external/local |
| `BL-263` | `D` | Version 1.0 final release boundary; do not auto-select before the required implementation/integration truth exists |

Completed artifact tasks `BL-246–248` and `BL-251` remain closed.

### 10.13 Documentation and client compatibility — BL-305–BL-315

| IDs | Mode | Notes |
|---|---|---|
| `BL-305–BL-306`, `BL-315` | `D` | Continuous steering-document maintenance; update as triggered by real task truth rather than as arbitrary standalone Mobile work |
| `BL-307–BL-312`, `BL-314` | `A` | Identity, architecture, benchmark, roots/capabilities, Job Manager, process/execution security and smoke-test docs |
| `BL-313` (`Later`) | `A` | Provider ecosystem documentation after the provider contracts it describes exist |

### 10.14 Independent-review follow-ups — BL-317–BL-332

| IDs | Mode | Notes |
|---|---|---|
| `BL-317` | `A` | Workflow useful-output semantics |
| `BL-318–BL-319` | `B` | Corpus-parent and host/build provenance logic can be implemented; authoritative Windows/native-host evidence later |
| `BL-320` | `B` | Native race/cross-platform policy CI can be authored and Hosted-CI-tested; complete platform evidence later |
| `BL-321` | `B` | Linux clock-tick implementation can be developed/tested in Cloud; stop if a new dependency/platform contract decision is required |
| `BL-322` | `X` | Acceptance requires SPR-047/historical evidence not fully repository-contained |
| `BL-323`, `BL-325–BL-327`, `BL-329`, `BL-331` | `A` | Bounded repo-contained review follow-ups |
| `BL-328` | `C` | Resource limits require justified limit decisions before silently selecting concrete thresholds |
| `BL-330` | `C` | Canonical `In Progress` status contract requires owner decision |
| `BL-332` | `B` | Contributor-path cleanup can be coded; Windows/WSL leak-validation finalization required |

### 10.15 Cross-mode host lifecycle — BL-341

| ID | Mode | Notes |
|---|---|---|
| `BL-341` | `B` | Architecture contract is already complete; runtime implementation can be prepared in Cloud, with Windows/Linux owner-loss, shutdown and orphan finalization after return |

### 10.16 Excluded completed work

All `Done` tasks are excluded automatically, including the completed foundations, rename/tool-cleanup/Codex-preparation work and `BL-324`, `BL-333–340`, `BL-342–344`.

No Mobile task may reopen them.

### 10.17 Operational launch matrix — snapshot 2026-09-14

This section is an **operational snapshot**, not a replacement for dynamic discovery. It exists so a phone/Codex Cloud session can start from a known-good topology without reconstructing the whole vacation graph from memory.

Snapshot binding:

```text
Repository : thomasweidner/flashgate-mcp
Main       : 5b851afb3ef3e7de1b49ce5a352c06d40b815f9d
AGENTS     : 73b06e26da08847ebd3c5ee0512c581147eb4548
BACKLOG    : a02db7c09b10dba8c827f62618c7c6eb9f096591
MOBILE     : d9ba2949cb1523d8a5a8dce0b3fa8763bf0cf6b6 (pre-this-update)
Ledger     : open GitHub PRs through #147 at snapshot time
```

**Mandatory refresh:** before any mutation, re-read current `AGENTS.md`, governance, `BACKLOG.md`, this file, and the complete open-PR ledger. If `main`, a listed parent PR, or dependency truth changed, current repository/GitHub truth wins and the snapshot classification must be recomputed.

Operational preclassification codes:

| Code | Meaning | Start rule |
|---|---|---|
| `MAIN_READY` | No mandatory unintegrated predecessor identified at snapshot time | Start a fresh Cloud task from current `main`; reclassify before mutation |
| `STACK_RESTART_READY` | One clear open predecessor is known | Start a **new** Cloud task directly from that PR head branch/SHA; no predecessor import |
| `RECHECK_ON_PARENT` | Likely stackable from the listed parent, but exact dependency truth must be rebound | Start from parent only if local inspection proves `STACK_BASE_READY` |
| `WAIT_SINGLE_PARENT` | Required immediate predecessor has no durable child candidate yet | Prepare/publish predecessor first |
| `WAIT_MULTI` | Multiple independent/uncombined predecessors are implicated | Do not synthesize a Cloud merge branch |
| `ANALYSIS_READY` | Mode C research/contract work is useful | Start from `main`; stop at any new decision boundary |

#### Immediate launch candidates

| Task | Mode | Snapshot state | Start ref / expected base | Depends on | Guidance |
|---|:---:|---|---|---|---|
| `BL-209` | C | `ANALYSIS_READY` | `main` | `NONE` | SPR-048 compatibility decision preparation only; do not invent the decision |
| `BL-211` | C | `ANALYSIS_READY` | `main` | `NONE` | SPR-048 fallback-contract decision preparation only |
| `BL-037` | A | `STACK_RESTART_READY` | PR #75 head `codex/next-suitable-cloud-task-ausfuhren` @ `abe72f8a486aa80ffecf2713769dfad3610f46f4` | `BL-038` | Paginated listing consumes stable cursor semantics |
| `BL-051` | A | `STACK_RESTART_READY` | PR #90 head `codex/fuhre-cloud-task-aus-mobile.md-v3-5ml4q4` @ `d4aee0943a1ac4d47aede5899c196437e019710d` | `BL-050` | Expected-match-count checks extend targeted edits |
| `BL-053` | A | `MAIN_READY` | current `main` | `NONE` | Conditional hash/mtime/path-type preconditions; recheck collisions before mutation |
| `BL-069` | A | `STACK_RESTART_READY` | PR #104 head `codex/fuhre-unabhangigen-cloud-task-aus` @ `0ad566a417118339c44df3a045a30cd652bb9331` | `BL-068` | Recommended first Search implementation child |
| `BL-101` | A | `MAIN_READY` | current `main` | `NONE` | Named-root foundation; do not manufacture a stack solely for conceptual sequencing |
| `BL-120` | A | `RECHECK_ON_PARENT` | PR #123 / `BL-119`, head `codex/fuhre-nachsten-cloud-task-aus-o35p7y` @ `0289117a8d818721d3559d244819c333441c6d0d` | likely `BL-119` | Proceed only if registry is the only required implementation predecessor |
| `BL-137` | A | `STACK_RESTART_READY` | PR #107 head `codex/fuhre-cloud-task-aus-mobile.md-v3-aus-ztndq7` @ `02bbdc19b9f706b7d033e7cc55378f612c1c95f` | `BL-136` | Recommended first command-execution child |
| `BL-154` | A | `STACK_RESTART_READY` | PR #135 head `codex/fuhre-nachsten-geeigneten-cloud-task-aus-58655l` @ `1ebd868f5f20d37be48cdf5df5e0de1b7e470797` | `BL-062` | Canonical backlog explicitly requires reuse of BL-062 |
| `BL-155` | A | `RECHECK_ON_PARENT` | PR #132 / `BL-153`, head `codex/fuhre-nachsten-cloud-task-aus-myz7pk` @ `20e2d97a640879564a428b29726fa9c6646abf69` | likely `BL-153` | Preserve environment allowlist and secret exclusion |
| `BL-156` | A | `RECHECK_ON_PARENT` | PR #132 / `BL-153`, same head | likely `BL-153` | Recheck whether BL-155 became a direct predecessor before mutation |
| `BL-159` | A | `STACK_RESTART_READY` | PR #105 head `codex/fuhre-nachsten-cloud-task-aus-mobile.md-v3-aus` @ `3ca7584e00690b8f09bccb8401e555bc9cf3985f` | `BL-100` | Registration must not be the authorization boundary |
| `BL-177` | C | `ANALYSIS_READY` | `main` | `NONE` | Governance model drafting/research; owner decision may remain |
| `BL-178` | C | `ANALYSIS_READY` | `main` | `NONE` | Maintainer rules; stop at owner decision |
| `BL-179` | C | `ANALYSIS_READY` | `main` | `NONE` | Contribution guidance; DCO/CLA is a decision boundary |
| `BL-328` | C | `ANALYSIS_READY` | `main` | `NONE` | Resource limits require justified thresholds; do not silently select them |
| `BL-330` | C | `ANALYSIS_READY` | `main` | `NONE` | Prepare status-contract options/tests; owner chooses canonical rule |

#### Workstream topology

**Filesystem**

- `BL-037`: stack on BL-038 / PR #75.
- `BL-051`: stack on BL-050 / PR #90.
- `BL-053`: `MAIN_READY` snapshot candidate.
- `BL-049`, `BL-056`, `BL-060`, `BL-061`, `BL-063`, `BL-064`: `WAIT_MULTI` at snapshot time because they intersect several independent open listing/write/job foundations.
- `BL-057`: `WAIT_SINGLE_PARENT` on the bounded-plan implementation from BL-056.

**Search**

- foundation: BL-068 / PR #104.
- `BL-069`: first recommended stack child.
- `BL-070`, `BL-071`, `BL-072`, `BL-074`, `BL-075`, `BL-076`, `BL-078`, `BL-080`, `BL-082`: `RECHECK_ON_PARENT`; use BL-068 only when the task remains task-pure without a newer Search sibling.
- `BL-073`, `BL-077`, `BL-079`: prefer a direct implementation predecessor; `BL-079` must consume the stable ordering semantics of the search implementation it paginates.

**Named roots / capabilities**

- capability foundation: BL-100 / PR #105.
- `BL-101`: `MAIN_READY` snapshot candidate.
- `BL-102`, `BL-104–107`: normally wait for the BL-101 candidate and then reclassify.
- `BL-103`, `BL-108–110`: currently cross capability/root/profile boundaries and should be treated as `WAIT_MULTI` until one ancestry chain exists.
- `BL-111`: run only after the effective dynamic registration/catalog model exists.

**Process**

Open foundations are split across BL-113, BL-119, BL-129 and BL-162. Therefore:

- `BL-120` and `BL-121`: recheck from BL-119 / PR #123.
- `BL-114–118`, `BL-122–126`, `BL-130–135`: treat as `WAIT_MULTI` until current topology proves a single direct parent or Windows integration combines the required foundations.

**Command execution**

- foundation: BL-136 / PR #107.
- `BL-137`: first recommended stack child.
- `BL-138`: normally wait for BL-137.
- `BL-140`, `BL-141`, `BL-144`, `BL-149`: may be separable contract work from BL-136; recheck before mutation.
- `BL-139`, `BL-142`, `BL-143`, `BL-145–148`, `BL-151–152`: currently cross named-root, result-resource, managed-process, policy and/or platform-isolation foundations; default to `WAIT_MULTI`.
- `BL-143` must reuse the Managed Process Engine and must never create a second engine.

**System information**

- `BL-154`: stack on BL-062 / PR #135.
- `BL-155`/`BL-156`: recheck from BL-153 / PR #132.
- `BL-157`: wait for both effective system-info and capability-enforcement truth.

**Security**

- `BL-159`: stack on BL-100 / PR #105.
- `BL-160`: normally follows BL-159.
- `BL-161`, `BL-163`, `BL-164`, `BL-167`, `BL-168`, `BL-171`: cross several independent domain/security foundations; treat as `WAIT_MULTI` until reclassification proves otherwise.

**Multi-mode/service and CI/release**

Most unreserved work in `BL-223–244` and `BL-252–262` remains Cloud-capable by mode, but current implementation topology is split across independent architecture, process, identity and service foundations. Do not infer one parent from the catalog alone. Reclassify a user-named task from the selected checkout and use `WAIT_MULTI` unless a single exact parent or independent implementation is proven.

`BL-261` may prepare Mode-B harness/research work but authoritative identical-host/external-server evidence is deferred. `BL-262` may prepare repository-contained supply-chain plans/evidence, but signing credentials and atomic release actions remain outside Cloud authority. `BL-341` remains Mode B and must be rebound against then-current multi-mode host/runtime truth before mutation.

#### Mode-C analysis queue

Useful Planned analysis/contract tasks when implementation topology is blocked:

`BL-177`, `BL-178`, `BL-179`, `BL-209`, `BL-211`, `BL-328`, `BL-330`.

Codex may investigate repository evidence, prepare options, and write only a bounded contract/documentation delta already permitted by the canonical BL scope. It must stop with `BLOCKED_DECISION_REQUIRED` before making a new product, architecture, security, release, dependency, platform, scope, or owner decision.

#### Post-1.0 work

Do not auto-select while executable unreserved Planned work exists:

`BL-081`, `BL-083`, `BL-112`, `BL-127`, `BL-128`, `BL-150`, `BL-158`, `BL-169`, `BL-176`, `BL-181–188`, `BL-217`, `BL-232`, `BL-240`, `BL-313`.

#### Copy/paste start instructions

Independent task from `main`:

```text
Führe <BL-ID> gemäß `MOBILE.md` V3 als eigenständigen Mobile-Task aus.
Start-Ref ist aktuelles `main`. Lies zuerst `AGENTS.md`, die Cloud-Governance,
`Governance/MOBILE-CLOUD-HANDOFF.md`, `BACKLOG.md`, `MOBILE.md` und alle
scope-relevanten ADR-/Security-/Test-Dokumente. Prüfe vor jeder Mutation den
offenen GitHub-PR-Ledger und klassifiziere die DependencyExecution erneut.

Wenn der Task nicht mehr `INDEPENDENT_FROM_CURRENT_CHECKOUT` ist, stoppe ohne
Mutation und gib den korrekten Dependency-Zustand aus.

Implementierung, task-purer Commit und Cloud-validierbare Tests sind für genau
diesen BL-Task freigegeben. Kein Merge, kein PR-Close, kein Branch-Delete,
kein Tag/Release, kein manuelles Remote-/Credential-Setup und keine Installation
neuer Dependencies. Nach Abschluss PR-Metadaten für den manuellen Codex-UI
`Create PR` Schritt vorbereiten.
```

Stacked child:

```text
Führe <CHILD-BL> gemäß `MOBILE.md` V3 als abhängigen Mobile-Task aus.
Der ausgewählte Start-Ref ist der Vorgänger von <PARENT-BL>:
<ParentHeadBranch>
ParentHeadSha=<ParentHeadSha>

Verifiziere lokal, dass ParentHeadSha im aktuellen Checkout enthalten ist.
Kein Fetch, Pull, Merge oder Cherry-Pick des Vorgängers. Wenn die Ancestry-
Prüfung oder die reale Dependency nicht passt, stoppe ohne Mutation.

Der spätere Child-PR muss `<ParentHeadBranch>` als Base verwenden und
`Mobile-Depends-On: <PARENT-BL>` enthalten. Implementierung, task-purer Commit
und Cloud-validierbare Tests sind nur für <CHILD-BL> freigegeben. Kein Merge,
PR-Close, Branch-Delete, Tag/Release, Remote-/Credential-Setup oder neue
Dependency-Installation.
```

Mode-C analysis/contract:

```text
Bearbeite <BL-ID> gemäß `MOBILE.md` V3 ausschließlich als Mode-C
Analyse-/Contract-Task. Lies aktuellen Repository-, Governance-, Backlog- und
PR-Ledger-Stand. Erstelle die maximal repository-gestützte Analyse bzw. den
bounded Contract-/Dokumentationsdelta, den der kanonische BL-Task bereits
erlaubt. Triff keine neue Produkt-, Architektur-, Security-, Release- oder
Owner-Entscheidung. Sobald eine solche Entscheidung erforderlich wird, stoppe
mit `BLOCKED_DECISION_REQUIRED` und liefere die Optionen, Auswirkungen und die
kleinste notwendige Entscheidung.
```

## 11. Epic-level collision and priority guidance

This is guidance, not ordering.

### Low collision / good parallel starts

Typical candidates include:

- focused documentation tasks;
- independent review follow-ups;
- isolated tests;
- architecture/threat-model contracts;
- distinct domain foundations such as Search model, Job Registry, Capability model, Process threat model, Command threat model;
- System Information work;
- independent benchmark validator corrections.

### Medium collision

Tasks touching:

- common MCP registry/schema files;
- shared filesystem contracts;
- benchmark decoder/types;
- shared policy/configuration;
- common documentation.

Prefer stacking when one candidate directly consumes another.

### High collision

Tasks modifying:

- central server routing;
- shared root/capability model;
- managed process engine;
- execution identity;
- service lifecycle coordinator;
- broad CI workflows.

Avoid unrelated parallel branches that rewrite the same shared abstraction.

## 12. Windows return and integration

After vacation, enumerate all open PRs carrying `Mobile-Queue: FLASHGATE-MOBILE-V3` or otherwise unambiguously reserved by the Mobile handoff rules.

Process dependency roots before stacked children.

For each candidate:

1. verify exact PR base/head and dependency markers or provisional selected-task identity;
2. inspect complete diff and Mobile ancestry;
3. bind current local `AGENTS.md`, governance and local canonical state;
4. independently review the candidate;
5. run complete Windows/native-Linux/PowerShell 7.6.5 validation;
6. correct directly caused in-scope findings;
7. integrate or retarget stacked PRs only with then-applicable Git/remote approval;
8. update `BACKLOG.md`, `CHANGELOG.md`, docs/status only from verified integration truth;
9. merge/close the Mobile PR only after Windows finalization;
10. delete branches only after successful integration and separate cleanup approval.

A Cloud PR is evidence of a candidate, never evidence that the BL item is `Done`.

## 13. Required Mobile PR format

The **GitHub PR metadata is part of the durable Mobile contract**.

### Canonical title — preferred

```text
[MOBILE][BL-xxx] <concise canonical task subject>
```

### Canonical body — preferred

```text
Mobile-Queue: FLASHGATE-MOBILE-V3
Mobile-Task: BL-xxx
Mobile-Mode: A|B|C
Mobile-State: CLOUD_IMPLEMENTATION_COMPLETE
Mobile-Depends-On: NONE|BL-xxx[,BL-yyy...]
Mobile-Base-Ref: <main-or-parent-mobile-branch>
Windows-Finalization: REQUIRED
Merge-Allowed: NO
```

Also state:

- Cloud validations actually run;
- Windows/native validations deferred;
- warnings/findings;
- scope and explicit non-goals.

The Codex UI may rematerialize commit metadata, branch naming, or PR formatting. A UI-generated PR that omits the complete marker block still reserves the BL when exactly one selected-task identity is unambiguous under `Governance/MOBILE-CLOUD-HANDOFF.md`. Record such metadata as `PROVISIONAL_UI_GENERATED`; do not duplicate the task.

### Head branch

Preferred branch naming remains task-specific, for example:

```text
mobile/bl-036-tools-call-filesystem-tests
```

A generated branch name is acceptable when task identity, expected base, and content identity are valid. Branch naming is diagnostic; BL reservation, expected base, and tree identity are authoritative.

The PR remains open. Do not merge or close it during Mobile work.

### Publication validation

After the user performs `Create PR`, read the PR back from GitHub. Apply the reservation, tree-identity, and expected-base rules from `Governance/MOBILE-CLOUD-HANDOFF.md`.

Do not automatically mutate a mismatched PR. Keep the BL reserved and return the exact mismatch for a new decision.

## 14. Required Cloud final responses

### Before manual Create PR

```text
Status              : CLOUD_IMPLEMENTATION_COMPLETE_AWAITING_MANUAL_PR
TaskID              : BL-xxx
CloudMode            : A | B | C
DependencyExecution : INDEPENDENT_FROM_CURRENT_CHECKOUT | STACK_BASE_READY
ActualHeadBranch     : <cloud branch>
CloudCommitSha       : <full Cloud commit SHA>
CloudTreeSha         : <full Cloud tree SHA>
ExpectedPRBase       : <main-or-parent-mobile-branch>
ExpectedPRTitle      : [MOBILE][BL-xxx] <subject>
CloudValidation      : <result>
DeferredValidation   : <Windows/native work>
WindowsFinalization  : REQUIRED
WarningCount         : <n>
FailureCount         : <n>
NextAction           : Use Codex UI Create PR, then reply "PR erstellt"
```

### After GitHub readback

```text
Status                 : CLOUD_IMPLEMENTATION_COMPLETE_PR_OPEN | MANUAL_PR_CREATED_PENDING_GITHUB_VISIBILITY | MOBILE_PR_RESERVED_CONTENT_IDENTITY_UNVERIFIED | MOBILE_PR_CONTENT_MISMATCH | MOBILE_STACK_PR_BASE_MISMATCH | MOBILE_PR_CORRELATION_AMBIGUOUS | BLOCKED_...
TaskID                 : BL-xxx
ReservationState       : VACATION_CANDIDATE_ALREADY_PREPARED
PRMetadataState        : CANONICAL | PROVISIONAL_UI_GENERATED
PullRequest            : <OPEN GitHub #number/url-or-NONE>
ExpectedPRBase         : <main-or-parent-mobile-branch>
ActualPRBase           : <GitHub base-or-NONE>
GitHubHeadSha          : <head SHA-or-NONE>
CloudTreeSha           : <Cloud tree SHA-or-NONE>
GitHubTreeSha          : <GitHub tree SHA-or-NONE>
ContentIdentity        : PASS | FAIL | NOT_AVAILABLE
CommitShaRematerialized: true | false | NOT_AVAILABLE
GitHubReadback         : PASS | NOT_AVAILABLE | FAIL
WindowsFinalization    : REQUIRED
NextAction             : <exact boundary>
```

## 15. Minimal phone prompts

### Automatic candidate

> Führe den nächsten geeigneten Cloud-Task aus `MOBILE.md` V3 aus.

This short prompt is sufficient only because root `AGENTS.md`, `Governance/MOBILE-CLOUD-HANDOFF.md`, and this file define the required discovery, reservation, dependency, validation, and manual Create-PR handoff. Codex must not infer missing authorization for merge, close, branch deletion, remote configuration, credentials, installs, tags, releases, or other external actions.

### Named epic

> Wähle den nächsten geeigneten noch offenen Task aus dem Filesystem-Epic gemäß `MOBILE.md` V3 und führe genau diesen Mobile-Task aus.

Replace `Filesystem` with `Search`, `Operations/Job`, `Process`, `Command Execution`, `MCP Contracts`, `Multi-Mode` or another catalog section.

### Named task

> Führe `BL-xxx` gemäß `MOBILE.md` V3 aus. Prüfe die DependencyExecution aus dem aktuellen Checkout; bei `STACK_REQUIRED` stoppe mit dem exakten Vorgänger-Ref für einen neuen Cloud-Task. Kein Fetch/Pull/Merge/Cherry-Pick des Vorgängers und kein Merge des späteren PRs.
