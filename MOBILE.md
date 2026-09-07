# FlashGate MCP — Mobile / Codex Cloud Queue

## 1. Purpose and authority

`MOBILE.md` is a derived execution queue for bounded FlashGate work in Codex Cloud while the Windows development host is unavailable.

It is **not** a second backlog authority.

Canonical authority remains, in this order:

1. the current repository `BACKLOG.md`;
2. `Governance/CHANGE-TRIGGER-REVIEW-AND-BACKLOG-STANDARD.md`;
3. `CONTRIBUTING.md` and the directly affected technical documentation;
4. this file only for mobile task selection, branch isolation, Cloud publication, and Cloud/Windows handoff rules.

If this file conflicts with a current canonical source, stop with `STALE_MOBILE_QUEUE` and do not adapt the contract autonomously.

Do not create a new BL identifier from this file. Do not change sprint assignment or milestone semantics from this file.

### Mobile publication model

- Queue protocol: `FLASHGATE-MOBILE-V2`
- Publication model: `CODEX_CLOUD_MANAGED_OPEN_PR`
- One mobile task = one isolated Cloud candidate = one dedicated head branch = one **open** pull request.
- Mobile pull requests are never merged, closed, rebased onto another mobile branch, or deleted during the vacation workflow.
- Windows finalization remains mandatory before integration.

## 2. Activation gate

The queue requires the validated FlashGate slim-governance baseline:

- Required baseline ancestor: `f9383ca664e38b93ac6331b312e9327806be43f7`
- Baseline subject: `Converge FlashGate to slim governance`
- M3b: complete
- M3c: complete
- FlashGate product restart: allowed
- PowerShell target standard for later Windows/native-Linux validation: `pwsh` 7.6.5

A local Git remote is **not** an activation requirement for Codex Cloud.

The message `no Git remote configured` is non-blocking when all of the following are true:

1. the Cloud task is attached to repository `thomasweidner/flashgate-mcp`;
2. the repository-integrated source can read the current default branch `main`;
3. the current `main` contains the required baseline above;
4. this `MOBILE.md` revision is visible from `main`;
5. the current `BACKLOG.md` still contains the selected BL row exactly once;
6. the selected task is still `Planned`;
7. its title and acceptance contract have not materially changed from the task summary in this file;
8. no current canonical rule introduces a new hard dependency or stop boundary;
9. Codex Cloud exposes its repository-integrated action to publish the completed candidate as a pull request.

If 1–4 fails:

`Status=BLOCKED_MOBILE_REPOSITORY_OR_BASELINE`

If 5–8 fails:

`Status=STALE_MOBILE_ENTRY`

If 9 fails:

`Status=BLOCKED_CLOUD_PR_PUBLICATION_UNAVAILABLE`

Do not configure a Git remote, token, credential, SSH key, PAT, GitHub App, or network workaround to make an activation check pass.

## 3. Mobile base and isolation contract

Every mobile task must start from the repository-integrated `main` state presented to the Cloud task.

Rules:

1. Never base a mobile task on another `mobile/*` branch or another mobile pull request.
2. Never merge another mobile pull request into the task branch.
3. Mobile pull requests remain open and unmerged during the vacation workflow.
4. Therefore mobile tasks remain mutually independent even when they are created at different times.
5. If `main` advances for unrelated work, re-read the current canonical sources before starting the next task. Continue only if the selected task contract is still valid.
6. The pull request's GitHub metadata is the authoritative base/head binding for later Windows review.
7. If Codex Cloud exposes an exact base SHA before publication, record it in the final response. Do not invent one if the platform does not expose it.

`main` is the only allowed PR base for this mobile queue.

## 4. How to select the next task

Process the queue strictly from top to bottom.

Use the **Codex Cloud / GitHub repository integration**, not a local Git remote, to inspect existing mobile pull requests.

A valid completed Cloud candidate is an open pull request whose body contains all of:

```text
Mobile-Queue: FLASHGATE-MOBILE-V2
Mobile-Task: <BL-ID>
Mobile-State: CLOUD_IMPLEMENTATION_COMPLETE
Windows-Finalization: REQUIRED
Merge-Allowed: NO
```

For each row:

1. Query repository-integrated pull-request state read-only.
2. If exactly one valid open mobile PR exists for the row, skip that row.
3. If no mobile PR exists for the row, that row is the next task.
4. If more than one candidate PR exists for the same row, stop:
   `Status=MOBILE_PR_COLLISION`.
5. If a matching PR is closed without merge, stop:
   `Status=MOBILE_PR_CLOSED_REVIEW_REQUIRED`.
6. If a matching PR was merged during the mobile workflow, stop:
   `Status=MOBILE_POLICY_VIOLATION_MERGED_PR`.
7. If Codex Cloud cannot inspect repository-integrated PR state, stop:
   `Status=CLOUD_PR_STATE_UNAVAILABLE`.
   Do not guess task completion from local branches or task history.

An open, correctly marked PR is the durable mobile completion marker. `MOBILE.md` itself is not edited after each task.

## 5. Per-task authorization boundary

Each task requires a fresh user instruction. A previous task authorization never carries forward.

The preferred minimal mobile authorization is:

> Führe den nächsten Task aus `MOBILE.md` nach Variante B aus. Implementierung und genau ein Cloud-gemanagter offener PR sind für genau diesen Task freigegeben. Kein Merge, kein PR-Close, kein Branch-Delete und keine manuelle Remote- oder Credential-Konfiguration.

This authorization is task-scoped only.

It authorizes:

- implementation of exactly the selected BL task in the Cloud workspace;
- directly caused in-scope correction cycles within the bounded remediation budget;
- Cloud-available validation;
- exactly one Codex-Cloud-managed publication attempt that creates one open PR for the completed candidate.

It does **not** authorize:

- `git remote add`, remote URL changes, PAT/token/SSH configuration, or manual credential handling;
- a manual `git push` workaround;
- PR merge or close;
- branch deletion;
- tags, releases, repository settings, GitHub rules, secrets, permissions, or other external changes.

The first write-capable Cloud publication attempt consumes the publication authorization. If PR creation fails, do not retry automatically and do not fall back to manual Git remote/push configuration.

## 6. Cloud implementation contract

For every selected task:

1. Read the current canonical BL row and affected repository sources before changing anything.
2. Stay inside that BL acceptance contract.
3. Do not silently implement another backlog task.
4. Do not introduce a new product, architecture, security, platform, dependency, release, or scope decision.
5. If such a decision is required, stop with:
   `Status=BLOCKED_DECISION_REQUIRED`.
6. Do not install tooling or dependencies unless separately authorized.
7. Do not configure credentials, Git remotes, SSH keys, tokens, GitHub Apps, or repository settings.
8. Do not modify `MOBILE.md` from a mobile task.
9. Keep the canonical BL status `Planned` during Cloud preparation.
10. Do not mark the task `Done` in `BACKLOG.md`.
11. Do not update completion-only `CHANGELOG.md` state merely because the Cloud candidate is ready.
12. Do not fabricate Windows, WSL, native-Linux, PowerShell, service, SCM, systemd, hardware, credential, or local-governance evidence that the Cloud environment did not actually execute.
13. Repository-visible task artifacts/reports are forbidden unless the canonical BL acceptance explicitly requires them.
14. A Cloud candidate is never canonical completion.

### Validation funnel

Use:

1. parser/focused/root-cause tests first;
2. bounded correction cycles;
3. one consolidated repository-appropriate final validation;
4. inspect the complete final diff and changed-path inventory;
5. run `git diff --check` when local Git is available;
6. confirm no unrelated task or completion-only steering change is present;
7. publish exactly one open PR through the Codex Cloud / GitHub integration.

For the established FlashGate repository, maximum material self-remediation cycles: `6`.

Use repository-provided tools already available in the Cloud environment. If `pwsh` 7.6.5 or another required local tool is unavailable, do not install a replacement silently; record that validation for Windows finalization.

Local Git staging or committing is an implementation detail, not a publication requirement. Do not fail merely because no Git remote exists. The authoritative Cloud handoff is the GitHub pull request and its base/head/diff metadata.

If the final candidate has no repository delta, stop with:

`Status=CLOUD_NO_CHANGE_REVIEW_REQUIRED`

Do not create an empty PR and do not advance to the next queue item automatically.

## 7. Branch and pull-request contract

Each task owns exactly one dedicated head branch and exactly one open pull request.

### Branch naming

The queue provides a preferred branch name.

- If Codex Cloud lets the task choose the branch name, use the exact preferred name.
- If Codex Cloud assigns a generated branch name, that is acceptable only when it is a unique branch dedicated to this one task.
- Never reuse another task branch.
- Never create a second branch merely to obtain the preferred spelling.
- Record the actual PR head branch in the final response.

### Pull request

The PR must:

- target `main`;
- contain only the selected task's candidate delta;
- remain open;
- not be marked as canonical task completion;
- not merge or close itself;
- not include `MOBILE.md` changes;
- use title format:

```text
[MOBILE][<BL-ID>] <canonical task title>
```

The PR body must contain:

```text
Mobile-Queue: FLASHGATE-MOBILE-V2
Mobile-Task: <BL-ID>
Mobile-State: CLOUD_IMPLEMENTATION_COMPLETE
Mobile-Preferred-Branch: <preferred branch from the queue>
Windows-Finalization: REQUIRED
Merge-Allowed: NO
```

Also summarize:

- Cloud validations actually executed;
- validations deferred to Windows;
- known warnings or findings;
- exact scope/non-goals.

After successful PR creation, perform read-only PR readback when the Cloud surface supports it. Verify base `main`, task identity, open state, and selected-task-only scope. Do not mutate the branch or PR further in the same task.

## 8. Ordered mobile queue

Effort scale:

- `XS` — narrow documentation or mechanical consistency change
- `S` — focused code/test change
- `M` — multi-file implementation/test change
- `M+` — broader contract-alignment work, still bounded
- Open BL dependencies count only **unfinished canonical backlog predecessors required to implement this row**.

| Order | ID | Preferred branch | Effort | Open BL dependencies | Cloud class | Windows finalization | Integration collision |
|---:|---|---|---|---:|---|---|---|
| 1 | BL-206 | `mobile/bl-206-local-deterministic-work` | XS | 0 | Docs | Required | Low |
| 2 | BL-323 | `mobile/bl-323-benchmark-coverage-claims` | XS | 0 | Docs/inventory | Required | Medium |
| 3 | BL-331 | `mobile/bl-331-arm64-runner-documentation` | S | 0 | Docs/current-state verification | Required | Medium |
| 4 | BL-202 | `mobile/bl-202-mcp-tool-annotations` | S | 0 | Metadata/tests | Required | Low |
| 5 | BL-327 | `mobile/bl-327-deterministic-benchmark-diagnostics` | S | 0 | Go/tests | Required | Medium |
| 6 | BL-329 | `mobile/bl-329-baseline-filename-identity` | S | 0 | Go/tests | Required | Medium |
| 7 | BL-036 | `mobile/bl-036-tools-call-filesystem-tests` | M | 0 | Go tests | Required | Low |
| 8 | BL-326 | `mobile/bl-326-strict-benchmark-unicode` | M | 0 | Go/decoder tests | Required | Medium |
| 9 | BL-325 | `mobile/bl-325-benchmark-schema-alignment` | M+ | 0 | Schema/Go/tests | Required | High |
| 10 | BL-317 | `mobile/bl-317-workflow-output-semantics` | M+ | 0 | Benchmark contract/tests | Required | Medium |

The ordering deliberately favors small, already-decided, repository-contained work. The benchmark cluster is later because several PRs may touch neighboring test/decoder files; they remain independent because no mobile PR is merged into another mobile task.

## 9. Task contracts

### 1 — BL-206 — Document local deterministic work principle

**Canonical intent:** Prefer local copy/edit/hash/search over model retransmission.

Cloud scope:

- identify the current canonical contributor/development documentation owner;
- add the smallest clear documentation delta expressing the existing deterministic-local-work principle;
- keep it guidance only;
- do not change product behavior, security policy, protocol contracts, or tooling dependencies;
- avoid spreading duplicate wording across many documents.

If no single clear documentation owner can be derived from current repository structure, stop with `BLOCKED_DECISION_REQUIRED`.

Cloud completion requires a coherent documentation diff and available repository documentation checks.

Windows finalization: run current local documentation/governance gates and integrate the branch.

---

### 2 — BL-323 — Correct benchmark coverage claims for copy and search

**Canonical intent:** Current documentation must not claim Copy/Search benchmark coverage that does not exist.

Cloud scope:

- inventory executable benchmark cases from the repository;
- make `docs/testing.md` and directly affected current benchmark documentation match the executable inventory;
- describe Copy/Search as planned unless executable coverage actually exists;
- do not implement Copy/Search benchmarks in this task;
- do not rewrite immutable historical evidence.

Cloud completion requires documentation to match the current executable benchmark inventory.

Windows finalization: local documentation validation and integration.

---

### 3 — BL-331 — Align ARM64 validation documentation with the implemented runner model

**Canonical intent:** Current behavior is x64-hosted cross-compilation plus static ARM64 validation; native ARM64 execution is future/conditional unless current repository evidence proves otherwise.

Cloud scope:

- reconcile the current decision, build/release metadata, testing, and manual-validation documentation;
- clearly distinguish current from target behavior;
- do not change build or CI behavior;
- do not claim native ARM64 runner execution without repository evidence;
- no build/CI matrix rerun is required for the Cloud candidate unless behavior is unexpectedly changed, in which case stop rather than expand scope.

Windows finalization: documentation/governance checks and integration.

---

### 4 — BL-202 — Review MCP tool annotations

**Canonical intent:** Tool annotations must accurately describe behavior and must never act as authorization.

Cloud scope:

- inventory current tool annotations and actual tool behavior;
- correct only demonstrably inaccurate annotation metadata;
- add/update focused tests proving metadata consistency where the current test structure supports it;
- do not alter authorization, capability, root, or risk-policy behavior;
- do not invent new annotation semantics.

If accurate annotation values require a new policy or protocol decision not already determined by current code/contracts, stop with `BLOCKED_DECISION_REQUIRED`.

Windows finalization: full relevant MCP/catalog validation and integration.

---

### 5 — BL-327 — Make benchmark hard-failure diagnostics deterministic

**Canonical intent:** All missing/unknown hard measurement and budget diagnostics plus final aggregation must be deterministically ordered.

Cloud scope:

- remove remaining Go-map-order nondeterminism from hard-failure diagnostics;
- preserve diagnostic meaning and failure semantics;
- add multi-error tests that evaluate fresh instances repeatedly;
- do not change benchmark budgets, measurements, or pass/fail policy.

Cloud validation: focused benchmark tests followed by consolidated Go validation available in the environment.

Windows finalization: current Windows/native-Linux benchmark regression gates and integration.

---

### 6 — BL-329 — Bind platform baseline filenames to embedded identity

**Canonical intent:** Fixed baseline filenames must agree with embedded OS/architecture identity.

Cloud scope:

- derive expected OS/architecture from each fixed required platform baseline filename;
- compare expected identity with embedded identity before map insertion;
- add swapped-content negative coverage;
- do not remeasure or modify baseline measurement data;
- do not broaden platform support.

Cloud validation: focused loader/identity tests followed by consolidated Go validation.

Windows finalization: current platform/baseline validation and integration.

---

### 7 — BL-036 — Add filesystem tests through MCP `tools/call`

**Canonical intent:** Add `tools/call` coverage for read, write, list, info, missing-path, and security cases.

Cloud scope:

- test the current implemented filesystem contracts through the real MCP `tools/call` path;
- reuse current tool names/contracts established by the completed pre-1.0 cleanup;
- include positive and negative/security cases required by BL-036;
- this is a test task, not authorization to change filesystem product behavior.

If a new test exposes a product defect whose correction exceeds the direct test-enablement scope, preserve the failing evidence and stop with `BLOCKED_PRODUCT_FINDING_REQUIRED`.

Cloud validation: focused MCP tests, then consolidated Go validation.

Windows finalization: Windows/Linux MCP/path/security revalidation and integration.

---

### 8 — BL-326 — Reject malformed Unicode in strict benchmark JSON

**Canonical intent:** Invalid raw UTF-8 and unpaired UTF-16 surrogate escapes are malformed; legitimate U+FFFD text remains valid.

Cloud scope:

- reject invalid raw UTF-8;
- reject unpaired surrogate escapes;
- retain legitimate U+FFFD;
- retain escaped-property duplicate detection;
- add positive and negative fixtures at relevant nesting levels;
- do not add an external dependency without a separate dependency decision.

Cloud validation: focused strict-decoder/mutation tests, then consolidated Go validation.

Windows finalization: full benchmark validation and integration.

---

### 9 — BL-325 — Align benchmark JSON Schema and Go representations

**Canonical intent:** Published schema and Go/runtime acceptance must be equivalent for exit statuses, numeric representation/ranges, and current nested constraints.

Cloud scope:

- inventory current schema and runtime constraints;
- align mechanically where the current authoritative runtime/schema contract makes the intended rule unambiguous;
- add deterministic drift tests for required/type/enum/pattern/minimum/additional-properties and relevant nested rules;
- do not silently choose between materially conflicting plausible contracts.

If equivalence requires a new contract choice rather than mechanical reconciliation, stop with `BLOCKED_DECISION_REQUIRED` and identify the exact mismatch.

Cloud validation: schema/decoder drift tests, then consolidated Go validation.

Windows finalization: full schema, benchmark, release-gate and platform validation plus integration.

---

### 10 — BL-317 — Gate deterministic workflow semantics independently of output size

**Canonical intent:** Missing or reduced useful output must fail even if byte/counter ceilings appear more efficient.

Cloud scope:

- validate every existing deterministic minimum/exact workflow contract;
- include `expected_read_bytes` and `expected_entries`;
- reject absent or reduced useful output;
- add negative artifacts/fixtures proving output loss fails;
- do not redefine benchmark budgets or create new benchmark domains.

Cloud validation: focused workflow/artifact tests, then consolidated benchmark/Go validation.

Windows finalization: authoritative Windows/native-Linux benchmark gates and integration.

## 10. Explicitly excluded from autonomous mobile execution

These are intentionally not in the queue even when some code could technically be written in Cloud.

### Missing repository-local authority

- `BL-322` — requires the SPR-047 report and related historical/current-state evidence not fully present in the GitHub repository.

### New decision required

- `BL-330` — requires an owner decision on the canonical `In Progress` status contract.
- `BL-328` — requires justified new resource limits.
- `BL-321` — requires a supported Linux clock-tick mechanism/platform choice.
- `BL-205` — response-size regression policy is not sufficiently bounded here to invent a threshold autonomously.
- architecture/threat-model/policy tasks throughout Search, Process, Command Execution, Service/IPC, provider/runtime, and release design.

### Local/platform evidence is central

- `BL-318`–`BL-320` — authoritative benchmark workspace/provenance/native policy and CI gates.
- `BL-332` — explicitly Windows/WSL native-validation hygiene.
- `BL-341` — Windows/Linux host-process ownership/lifecycle integration.
- Windows SCM, systemd, native service identity, permissions, ACL, owner, installation, credential, and signing work.

### Dependency-chain or closure-gate work

- CI tasks whose required implementation owners are not yet complete.
- Version 1.0 release boundary work such as `BL-263`.
- continuous documentation/status gates such as `BL-305`, `BL-306`, and `BL-315`.
- verification-only closure tasks that may produce no independent implementation diff are not used as vacation branches.

Anything not listed in section 8 is **out of scope for autonomous execution through this file**.

## 11. Windows return and integration contract

Mobile PRs are preparation candidates, not canonical completion.

After the vacation, process the open mobile PRs under Windows one at a time, normally in queue order:

1. enumerate open PRs carrying `Mobile-Queue: FLASHGATE-MOBILE-V2`;
2. verify PR base/head, task marker, branch identity, exact diff, and selected-task-only scope;
3. bind the then-current local `AGENTS.md`, `Codex-Work\Governance`, canonical `BACKLOG.md`, leading registers, and current repository state;
4. independently review the Cloud candidate;
5. run the complete task-specific Windows/native-Linux/PowerShell 7.6.5 validation funnel;
6. remediate only directly caused in-scope findings within the local continuation policy;
7. decide the integration method against the then-current `main`;
8. update canonical `BACKLOG.md`, `CHANGELOG.md`, status/docs only when local integration truth supports it;
9. merge/rebase/cherry-pick/update the mobile PR or use a separate integration branch only with the then-applicable explicit Git/remote approvals;
10. close or merge the mobile PR only after successful Windows finalization;
11. delete local/remote mobile branches only after verified integration and separate cleanup authorization.

A Cloud-open PR must never be treated as proof that Windows finalization is complete.

## 12. Required Cloud final response

For every mobile task, end with:

```text
Status              : CLOUD_IMPLEMENTATION_COMPLETE_PR_OPEN | CLOUD_NO_CHANGE_REVIEW_REQUIRED | BLOCKED_...
TaskID              : BL-xxx
PreferredBranch     : mobile/bl-...
ActualHeadBranch    : <branch-or-NONE>
PullRequest         : <OPEN #number/url-or-NONE>
PRBase              : main
CloudValidation     : <concise result>
RemoteConfiguration : NOT_REQUIRED_UNCHANGED
WindowsFinalization : REQUIRED
WarningCount        : <n>
FailureCount        : <n>
NextAction          : <next exact boundary>
```

`CLOUD_IMPLEMENTATION_COMPLETE_PR_OPEN` means only that the isolated Cloud candidate passed the Cloud-available checks and exactly one open PR was created through the managed repository integration. It never means `Done` in the canonical backlog.

A successful mobile task ends with an **open PR** and no merge.
