# FlashGate MCP — Mobile / Codex Cloud Queue

## 1. Purpose and authority

`MOBILE.md` is a derived execution queue for bounded FlashGate work in Codex Cloud while the Windows development host is unavailable.

It is **not** a second backlog authority.

Canonical authority remains, in this order:

1. the current repository `BACKLOG.md`;
2. `Governance/CHANGE-TRIGGER-REVIEW-AND-BACKLOG-STANDARD.md`;
3. `CONTRIBUTING.md` and the directly affected technical documentation;
4. this file only for mobile task selection, branch isolation, and Cloud/Windows handoff rules.

If this file conflicts with a current canonical source, stop with `STALE_MOBILE_QUEUE` and do not adapt the contract autonomously.

Do not create a new BL identifier from this file. Do not change sprint assignment or milestone semantics from this file.

## 2. Activation gate

The queue was derived from the current local FlashGate post-M3c baseline:

- Required baseline ancestor: `f9383ca664e38b93ac6331b312e9327806be43f7`
- Baseline subject: `Converge FlashGate to slim governance`
- M3b: complete
- M3c: complete
- FlashGate product restart: allowed
- PowerShell target standard for later Windows/native-Linux validation: `pwsh` 7.6.5

At authoring time, GitHub `main` was still `750638676dcfae19cbcf4a79961fa91aa89b8adc` and did not contain the required baseline.

**No queue task may start until all activation checks pass:**

1. `origin/main` contains `f9383ca664e38b93ac6331b312e9327806be43f7` as an ancestor.
2. `MOBILE.md` is present on `origin/main`.
3. The current `BACKLOG.md` still contains the selected BL row exactly once.
4. The selected task is still `Planned`.
5. Its title and acceptance contract have not materially changed from the task summary in this file.
6. No current canonical rule introduces a new hard dependency or stop boundary.

If 1 or 2 fails:

`Status=BLOCKED_MOBILE_BASELINE_NOT_PUBLISHED`

If 3–6 fails:

`Status=STALE_MOBILE_ENTRY`

Do not work around either state.

## 3. Mobile queue base

For the active queue, derive one immutable sibling-branch base:

```text
MOBILE_QUEUE_BASE = latest commit reachable from origin/main that changed MOBILE.md
```

Before the first mobile task, verify that `MOBILE_QUEUE_BASE` contains the required baseline ancestor above.

Every mobile task branch MUST start from exactly `MOBILE_QUEUE_BASE`, not from another mobile task branch and not from a later moving `main`.

If `MOBILE.md` is changed on `main` after mobile work has begun, stop with `STALE_MOBILE_QUEUE` until Classic/Windows reviews the new queue base.

This makes all vacation branches independent siblings and defers their integration order to Windows.

## 4. How to select the next task

Process the queue strictly from top to bottom.

For each row:

1. Check the exact remote branch name read-only.
2. If it is absent, that row is the next task.
3. If it exists, inspect it read-only:
   - it must descend from the active `MOBILE_QUEUE_BASE`;
   - its final task commit must contain the trailers defined below;
   - `Mobile-Task` must equal the row ID;
   - `Mobile-State` must be `CLOUD_IMPLEMENTATION_COMPLETE`;
   - `Mobile-Queue-Base` must equal the active queue base.
4. A valid completed branch is skipped.
5. An existing branch that does not satisfy all checks is a collision:
   `Status=MOBILE_BRANCH_COLLISION`.
   Do not overwrite, amend, force-push, delete, or reuse it.

A pushed valid task branch is therefore the durable mobile completion marker. `MOBILE.md` itself is not edited after each task.

## 5. Per-task authorization boundary

Each task requires a fresh user instruction. A previous task authorization never carries forward.

A minimal mobile authorization is:

> Führe den nächsten Task aus MOBILE.md aus. Implementierung ist für genau diesen Task freigegeben. Lokale Git-Mutationen für genau einen Task-Branch, Stage und einen Commit sind freigegeben. Genau ein nicht-erzwungener Push dieses Task-Branches ist freigegeben. Kein PR, Merge, Tag, Release, Branch-Delete oder Write auf main.

This authorization is task-scoped only.

Before the first Git write, complete all deterministic read-only preflights that can be known in advance.

Before the first remote write, complete the implementation, focused validation, consolidated validation, exact diff/scope review, and staged readback.

Hard-one-shot behavior applies:

- first writable Git action consumes the Git authorization;
- first remote push consumes the remote authorization;
- no blind or automatic retry after a failed Git/remote write;
- report the failure and stop.

No force push.

## 6. Cloud implementation contract

For every selected task:

1. Read the current canonical BL row and affected repository sources before changing anything.
2. Stay inside that BL acceptance contract.
3. Do not silently implement another backlog task.
4. Do not introduce a new product, architecture, security, platform, dependency, release, or scope decision.
5. If such a decision is required, stop with:
   `Status=BLOCKED_DECISION_REQUIRED`.
6. Do not install tooling or dependencies unless separately authorized.
7. Do not use credentials or modify GitHub settings.
8. Do not create a PR, merge, tag, release, or delete a branch.
9. Do not modify `MOBILE.md` from a task branch.
10. Keep the canonical BL status `Planned` during Cloud preparation.
11. Do not mark the task `Done` in `BACKLOG.md`.
12. Do not update completion-only `CHANGELOG.md` state merely because the Cloud candidate is ready.
13. Do not fabricate Windows, WSL, native-Linux, PowerShell, service, SCM, systemd, hardware, credential, or local-governance evidence that the Cloud environment did not actually execute.
14. Repository-visible task artifacts/reports are forbidden unless the canonical BL acceptance explicitly requires them.

### Validation funnel

Use:

1. parser/focused/root-cause tests first;
2. bounded correction cycles;
3. one consolidated repository-appropriate final validation;
4. `git diff --check`;
5. exact scope/readback before Stage;
6. staged readback plus `git diff --cached --check`;
7. one final commit;
8. one non-force push.

For the established FlashGate repository, maximum material self-remediation cycles: `6`.

Use repository-provided tools already available in the Cloud environment. If `pwsh` 7.6.5 or another required local tool is unavailable, do not install a replacement silently; record that validation for Windows finalization.

## 7. Branch and commit contract

Each row owns exactly one branch.

The branch must be created from `MOBILE_QUEUE_BASE` using the exact branch name from the queue.

A successful Cloud candidate has exactly one final task commit above the queue base.

Commit message format:

```text
<BL-ID>: <concise task subject>

Mobile-Task: <BL-ID>
Mobile-State: CLOUD_IMPLEMENTATION_COMPLETE
Mobile-Queue-Base: <full SHA>
Windows-Finalization: REQUIRED
```

Do not amend after a successful push.

Any correction required after the successful push is a Windows/Classic integration concern unless the user separately authorizes a new mobile correction contract.

## 8. Ordered mobile queue

Effort scale:

- `XS` — narrow documentation or mechanical consistency change
- `S` — focused code/test change
- `M` — multi-file implementation/test change
- `M+` — broader contract-alignment work, still bounded
- Open BL dependencies count only **unfinished canonical backlog predecessors required to implement this row**.

| Order | ID | Exact branch | Effort | Open BL dependencies | Cloud class | Windows finalization | Integration collision |
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

The ordering deliberately favors small, already-decided, repository-contained work. The benchmark cluster is later because several branches may touch neighboring test/decoder files; they remain sibling branches and MUST NOT consume each other.

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

Mobile branches are preparation candidates, not canonical completion.

After the vacation, process branches under Windows one at a time, normally in the same queue order:

1. fetch/read all mobile branches without modifying them;
2. verify branch ancestry, one-commit contract, trailers, diff, and task scope;
3. bind the then-current local `AGENTS.md`, `Codex-Work\Governance`, canonical `BACKLOG.md`, leading registers, and current repository state;
4. independently review the candidate;
5. rebase/cherry-pick/integrate only after the applicable explicit Git approval;
6. run the complete task-specific Windows/native-Linux/PowerShell 7.6.5 validation funnel;
7. remediate only directly caused in-scope findings within the local continuation policy;
8. update canonical `BACKLOG.md`, `CHANGELOG.md`, status/docs as required only when integration truth supports it;
9. commit/push/PR/merge only with the then-applicable explicit approvals;
10. delete local/remote mobile branches only after verified integration and separate cleanup authorization.

A Cloud branch must never be treated as proof that Windows finalization is complete.

## 12. Required Cloud final response

For every mobile task, end with:

```text
Status              : CLOUD_IMPLEMENTATION_COMPLETE | CLOUD_NO_CHANGE_REVIEW_REQUIRED | BLOCKED_...
TaskID              : BL-xxx
Branch              : mobile/bl-...
MobileQueueBase     : <sha>
CommitSha           : <sha-or-NONE>
PushState           : PUSHED_ONCE | NOT_PUSHED
CloudValidation     : <concise result>
WindowsFinalization : REQUIRED
WarningCount        : <n>
FailureCount        : <n>
NextAction          : <next exact boundary>
```

`CLOUD_IMPLEMENTATION_COMPLETE` means only that the isolated Cloud candidate passed the Cloud-available checks and was pushed once to its task branch. It never means `Done` in the canonical backlog.
