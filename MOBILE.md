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
Publication: CODEX_CLOUD_MANAGED_OPEN_PR
Merge-During-Mobile: NO
Windows-Finalization: REQUIRED
```

## 2. Cloud governance activation

Before task discovery or implementation, verify that the selected repository/base contains:

- root `AGENTS.md`;
- `Governance/CLOUD-CODEX-GOVERNANCE.md`;
- `Governance/CHANGE-TRIGGER-REVIEW-AND-BACKLOG-STANDARD.md`;
- current `BACKLOG.md`;
- this `MOBILE.md`.

Read the first three governance sources before selecting work.

If the tracked Cloud governance router or capsule is absent, unreadable, or internally contradictory:

`Status=BLOCKED_CLOUD_GOVERNANCE_UNAVAILABLE`

Do not compensate by assuming the missing local `Codex-Work/Governance` content from memory.

The Cloud capsule is intentionally a bounded projection. Every candidate is rebound against the complete current local governance during Windows finalization.

## 3. Core model — no global task order

There is **no mandatory global execution order**.

The numerical order in `BACKLOG.md`, sprint order, epic order, and the order of tables in this file are not a Mobile execution queue.

Tasks may be:

- implemented independently from current `main`;
- implemented in parallel;
- prepared before lower-numbered tasks;
- stacked on another open Mobile task when the implementation genuinely consumes that task.

A task is blocked only by a real canonical/technical dependency, missing required source/evidence, a decision boundary, or a Cloud capability boundary.

### Default selection when the user says only "next task"

Enumerate all eligible open tasks and choose the best candidate using these preferences, not hard gates:

1. `Planned` before `Later`;
2. Cloud mode `A` before `B` before `C`;
3. no prerequisite before a stack-ready prerequisite before a blocked prerequisite;
4. lower effort before higher effort;
5. lower expected Windows residual before higher residual;
6. lower expected branch/diff collision before higher collision.

Do **not** choose by BL number alone.

The user may name any eligible task or epic and override this preference order.

## 4. Cloud modes

| Mode | Meaning | Mobile completion |
|---|---|---|
| `A — CLOUD_IMPLEMENTABLE` | Repository-contained code/docs/tests can be substantially implemented and validated in Cloud | Open PR candidate; Windows finalization still required |
| `B — CLOUD_IMPLEMENTABLE_PLATFORM_FINAL` | Meaningful implementation is possible in Cloud, but Windows/native-host/service/real-OS validation is essential | Open PR candidate with explicit deferred validations |
| `C — CLOUD_ANALYSIS_OR_CONTRACT` | Cloud can perform architecture, threat-model, design, investigation, or draft work, but an owner/external decision may be required before implementation/final completion | Open PR only if the canonical task itself permits a bounded documentation/design delta; otherwise return analysis without PR |
| `D — CONTINUOUS_OR_FINAL_GATE` | Valid Cloud work may contribute, but the BL item is continuous, release-wide, or integration-wide and should not normally own a vacation PR by itself | Consume from other tasks; do not auto-select standalone |
| `X — NOT_AUTONOMOUS_MOBILE` | Required evidence/authority is unavailable or the work is primarily an external/local decision/action | Do not auto-select |

`A` does **not** mean Windows validation can be skipped. It means the Cloud candidate itself can be meaningfully completed.

## 5. Repository and publication boundary

A local Git remote is not required.

`no Git remote configured` is non-blocking when Codex Cloud can:

- read `thomasweidner/flashgate-mcp`;
- read the relevant base branch;
- inspect current GitHub pull-request state through the managed repository integration;
- publish the completed candidate through the managed open-PR action.

Do not configure:

- `git remote`;
- PAT/token credentials;
- SSH keys;
- GitHub Apps;
- repository settings;
- alternate network or credential workarounds.

If managed PR publication is unavailable after implementation:

```text
Status=BLOCKED_CLOUD_PR_PUBLICATION_UNAVAILABLE
```

Do not fall back to manual push.

### Durable GitHub publication boundary

A Codex Cloud task result, diff view, local Cloud commit, Cloud-internal branch view, or
Cloud-internal "PR" indication is **not** a durable Mobile completion marker by itself.

Use these states:

```text
CLOUD_IMPLEMENTATION_IN_PROGRESS
CLOUD_IMPLEMENTATION_COMPLETE_UNPUBLISHED
CLOUD_PR_PUBLICATION_PENDING
CLOUD_IMPLEMENTATION_COMPLETE_PR_OPEN
```

`CLOUD_IMPLEMENTATION_COMPLETE_PR_OPEN` may be claimed only after the managed publication
flow has created a real GitHub pull request and Codex has read that pull request back from
GitHub.

Required GitHub readback:

- repository is `thomasweidner/flashgate-mcp`;
- PR state is `OPEN`;
- PR title starts with `[MOBILE][<BL-ID>]`;
- PR body contains the exact Mobile V3 identity fields from section 13;
- `Mobile-Task` equals the selected canonical BL ID;
- `Mobile-State` is `CLOUD_IMPLEMENTATION_COMPLETE`;
- `Windows-Finalization` is `REQUIRED`;
- `Merge-Allowed` is `NO`;
- PR base equals the expected `main` or direct predecessor Mobile branch;
- PR head resolves to a durable GitHub branch and commit;
- the GitHub PR diff remains task-pure.

If implementation is complete but the real GitHub PR has not been created or cannot be
read back:

```text
Status=CLOUD_IMPLEMENTATION_COMPLETE_UNPUBLISHED
```

Do not archive, discard, or treat the Cloud task as completed. Do not advance to another
task under the same BL ID. The exact next action is the managed Codex Cloud `Create PR`
publication step.

If managed publication was attempted but GitHub readback is unavailable or invalid:

```text
Status=CLOUD_PR_PUBLICATION_PENDING
```

Do not retry automatically and do not fall back to manual Git remote or credential
configuration.

GitHub is the durable Mobile candidate ledger. Codex Cloud is the execution workspace.

## 6. Dynamic task discovery

Before selecting work:

1. confirm `AGENTS.md` and `Governance/CLOUD-CODEX-GOVERNANCE.md` were read for this task;
2. read current `BACKLOG.md`;
3. consider only canonical rows with status `Planned` or `Later`;
4. map the BL ID through the capability catalog;
5. inspect existing Mobile V3 PRs;
6. inspect real dependencies from the backlog, code, contracts, and existing Mobile PRs;
7. determine effort and likely Windows residual from the current repository state;
8. select an eligible candidate using the preference model unless the user named one.

A valid Mobile V3 candidate is a **real GitHub pull request**, read back from GitHub,
whose title starts with:

```text
[MOBILE][BL-xxx]
```

and whose body contains:

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

A Cloud task history entry, Cloud-only diff, or Cloud-internal PR indication is not
sufficient discovery evidence.

An open, GitHub-read-back, metadata-valid PR means that task already has a durable Cloud
candidate and is not selected again.

If a matching task has:

- multiple competing Mobile PRs → `MOBILE_PR_COLLISION`;
- a closed unmerged Mobile PR → `MOBILE_PR_CLOSED_REVIEW_REQUIRED`;
- a merged Mobile PR before Windows finalization → `MOBILE_POLICY_VIOLATION_MERGED_PR`.

## 7. Dependency and stacked-PR contract

Dependencies are **not** a reason to omit a task from `MOBILE.md`.

### 7.1 Independent task

If the task does not consume an unintegrated Mobile predecessor:

```text
task branch base = current main
PR base          = main
```

### 7.2 Task consuming one Mobile predecessor

If BL-B genuinely requires the unintegrated implementation of BL-A:

```text
main
  \
   BL-A branch ---- open PR A
          \
           BL-B branch ---- open stacked PR B
```

For BL-B:

```text
task branch base = current head of BL-A
PR base          = BL-A head branch
Mobile-Depends-On: BL-A
```

This keeps BL-B's PR diff limited to BL-B rather than duplicating BL-A.

Neither PR is merged during Mobile work.

### 7.3 Longer chain

A chain is allowed:

```text
main -> BL-A -> BL-B -> BL-C
```

Each PR targets its direct predecessor branch and records the complete dependency list relevant to Windows integration.

### 7.4 Multiple independent prerequisites

Do not automatically synthesize a merge branch from two unrelated open Mobile PRs.

If BL-C requires BL-A and BL-B and those heads are not already in one ancestry chain:

```text
Status=BLOCKED_MULTIPLE_UNCOMBINED_MOBILE_PREDECESSORS
```

Choose another eligible task. Windows can combine the prerequisites later, or the user can separately authorize a synthesis strategy.

### 7.5 Dependency truth

The dependency hints in this file are advisory Mobile planning hints. They do not create new canonical backlog dependencies.

Before stacking, verify the dependency from the current code/contracts.

## 8. Per-task authorization

Each Mobile task needs a fresh task-scoped instruction.

Preferred short instruction:

> Führe einen geeigneten Cloud-Task aus `MOBILE.md` V3 aus. Implementierung und genau ein Cloud-gemanagter offener GitHub-PR sind für genau diesen Task freigegeben. Reale Abhängigkeiten dürfen über einen gestapelten Mobile-Branch konsumiert werden. Der Task gilt erst nach erfolgreichem GitHub-Readback mit `[MOBILE][BL-xxx]`-Titel und vollständigen V3-Markern als Cloud-fertig. Kein Merge, kein PR-Close, kein Branch-Delete und keine manuelle Remote- oder Credential-Konfiguration.

The instruction authorizes for exactly one selected task:

- read-only discovery and dependency analysis;
- implementation or bounded design work allowed by its Cloud mode;
- up to 6 material in-scope correction cycles;
- Cloud-available validation;
- one managed publication attempt producing one real open GitHub PR plus read-only
  GitHub readback of its identity, state, base, head, title, markers, and task-pure scope.

It does not authorize:

- merging or closing any Mobile PR;
- deleting branches;
- tags/releases;
- GitHub settings/rules/secrets;
- manual remote/credential configuration;
- installations or dependencies not separately approved;
- implementation of another BL task merely because it is nearby.

## 9. Common implementation contract

For every task:

1. re-read its canonical BL row;
2. inspect affected code, tests, docs, ADRs, security and change-trigger rules;
3. identify actual prerequisites;
4. do not silently cross a product/security/architecture decision boundary;
5. keep the canonical backlog status unchanged;
6. do not mark Cloud work as canonical completion;
7. do not fabricate Windows/WSL/systemd/SCM/native-host evidence;
8. keep the candidate branch task-pure except for directly caused in-scope corrections;
9. run focused tests before broad validation;
10. inspect the complete final diff and `git diff --check` when local Git supports it;
11. publish one open PR only after the Cloud candidate is internally coherent;
12. after managed publication, read the PR back from GitHub and validate the section 5
    durability gate;
13. report `CLOUD_IMPLEMENTATION_COMPLETE_PR_OPEN` only after that GitHub readback passes;
14. if only the Codex Cloud result/diff exists, report
    `CLOUD_IMPLEMENTATION_COMPLETE_UNPUBLISHED` and preserve the Cloud task until
    publication succeeds or the user makes a new decision.

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

After vacation, enumerate all open PRs carrying:

```text
Mobile-Queue: FLASHGATE-MOBILE-V3
```

Process dependency roots before stacked children.

For each candidate:

1. verify exact PR base/head and dependency markers;
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

### Title — mandatory

Exact prefix:

```text
[MOBILE][BL-xxx]
```

Preferred complete title:

```text
[MOBILE][BL-xxx] <concise canonical task subject>
```

The BL ID must be present in this machine-recognizable prefix. A generic title derived
from the phone prompt, such as `Implementiere den nächsten mobile.md task`, is invalid
even when the Cloud task itself selected the correct BL item.

### Body — mandatory

The PR body must contain these exact identity fields as standalone lines:

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

### Head branch

Preferred branch naming remains task-specific, for example:

```text
mobile/bl-036-tools-call-filesystem-tests
```

If Codex Cloud controls branch naming and generates another unique branch, keep that
single generated branch rather than manufacturing a second branch solely for naming.

A generated branch name is acceptable only when:

- the branch is dedicated to exactly one Mobile task;
- the GitHub PR title and body satisfy the mandatory BL identity contract above;
- GitHub readback proves the actual durable head branch and commit;
- the final response records `GeneratedBranchNameAccepted: true`.

Branch naming is therefore advisory; PR title/body identity and GitHub readback are
mandatory.

The PR remains open. Do not merge or close it during Mobile work.

### Publication validation

After `Create PR`, read the PR back from GitHub. If any mandatory title/body/state/base/
head identity check fails:

```text
Status=CLOUD_PR_METADATA_INVALID
```

Do not claim Mobile completion and do not automatically mutate the PR a second time.
Return the exact mismatch for a new decision.

## 14. Required Cloud final response

```text
Status                    : CLOUD_IMPLEMENTATION_COMPLETE_PR_OPEN | CLOUD_IMPLEMENTATION_COMPLETE_UNPUBLISHED | CLOUD_PR_PUBLICATION_PENDING | CLOUD_ANALYSIS_COMPLETE | CLOUD_NO_CHANGE_REVIEW_REQUIRED | BLOCKED_...
TaskID                    : BL-xxx
CloudMode                  : A | B | C
PreferredBranch            : mobile/bl-xxx-...
ActualHeadBranch           : <durable-GitHub-branch-or-NONE>
GeneratedBranchNameAccepted: true | false
PullRequest                : <OPEN GitHub #number/url-or-NONE>
PRTitle                    : <exact title-or-NONE>
PRBase                     : <main-or-parent-mobile-branch>
PRHeadSha                  : <GitHub head SHA-or-NONE>
MobileDependsOn            : NONE | BL-...
GitHubReadback             : PASS | NOT_AVAILABLE | FAIL
CloudValidation            : <result>
DeferredValidation         : <Windows/native work>
RemoteConfiguration        : NOT_REQUIRED_UNCHANGED
WindowsFinalization        : REQUIRED
WarningCount               : <n>
FailureCount               : <n>
NextAction                 : <exact boundary>
```

## 15. Minimal phone prompts

### Automatic candidate

> Führe einen geeigneten Cloud-Task aus `MOBILE.md` V3 aus. Implementierung und genau ein Cloud-gemanagter offener GitHub-PR sind für genau diesen Task freigegeben. Reale Abhängigkeiten dürfen über einen gestapelten Mobile-Branch konsumiert werden. Der Task gilt erst nach erfolgreichem GitHub-Readback mit `[MOBILE][BL-xxx]`-Titel und vollständigen V3-Markern als Cloud-fertig. Kein Merge, kein PR-Close, kein Branch-Delete und keine manuelle Remote- oder Credential-Konfiguration.

### Named epic

> Wähle einen geeigneten noch offenen Task aus dem Filesystem-Epic gemäß `MOBILE.md` V3 und führe genau diesen Mobile-Task aus.

Replace `Filesystem` with `Search`, `Operations/Job`, `Process`, `Command Execution`, `MCP Contracts`, `Multi-Mode` or another catalog section.

### Named task

> Führe `BL-xxx` gemäß `MOBILE.md` V3 aus. Nutze bei echter technischer Abhängigkeit einen gestapelten Mobile-PR. Kein Merge.
