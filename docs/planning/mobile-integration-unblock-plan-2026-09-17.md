# FlashGate MCP — Mobile Integration / Unblock Plan

**Status:** versioned read-only integration planning handoff  
**Date:** 2026-09-17  
**Repository:** `thomasweidner/flashgate-mcp`  
**Repository path:** `docs/planning/mobile-integration-unblock-plan-2026-09-17.md`  
**Bound main:** `7cc15f37671e22fa52b8241d293d8d7220478abb`  
**Bound main tree:** `5f6262ff27f57593ecb71a1642bcac2e8837e2d3`  
**AGENTS.md blob:** `079fad212fc4843eec70999cb551495b226087e6`  
**BACKLOG.md blob:** `a02db7c09b10dba8c827f62618c7c6eb9f096591`  
**MOBILE.md blob:** `f2695dddc3ead56ca016e1fde8faf155fdd64ce0`  
**Governance/MOBILE-CLOUD-HANDOFF.md blob:** `209bcae942e512ef7ca00ac16b89b1354fb5c2e2`  
**Open PR ledger:** `198` open PRs  
**LedgerPaginationComplete:** `true`  
**LedgerSnapshotStable:** `true`  
**Source artifact SHA-256:** `3b5ff2c85cdd772445ccb8706c52b339b3208e435fafcce9992a5b24f482c34b`

This file is a durable planning artifact. It is **not** canonical backlog status and grants **no Git/remote/merge authority**. Before any integration or new Mobile implementation, bind the then-current repository, `main`, working tree, authoritative governance, `BACKLOG.md`, `MOBILE.md`, directly affected ADRs/docs and a fresh selector-stable open-PR ledger.

This version was rebound from the prior durable artifact immediately before repository publication. The bound repository state and ledger facts above are the publication preimage; future execution still requires a fresh rebind.

---

## 1. Current conclusion

The remaining active Mobile queue contains 21 `WAIT_MULTI` topics. The current state is an **integration bottleneck**, not a lack of useful work. Existing open PRs contain most prerequisite implementation pieces, but the pieces are split across independent PR lineages. The next productive Windows/Classic work is to converge a small number of shared prerequisite lines. After each convergence wave, rerun Mobile selection from fresh repository and PR-ledger truth.

Current Mobile snapshot:

```text
MAIN_EXECUTABLE     : 0
PR_STACK_CANDIDATE  : 0
WAIT_MULTI          : 21
```

Do not interpret `WAIT_MULTI` as a permanent classification. It must be recomputed after every material topology change.

---

## 2. Mandatory fresh-rebind gate before every integration wave

Before any write, retarget, rebase, merge, or new child implementation:

1. Bind repository, branch, `main` HEAD/tree and working tree.
2. Read current `AGENTS.md`, `Governance/MOBILE-CLOUD-HANDOFF.md`, `BACKLOG.md`, `MOBILE.md`, and affected ADR/security/testing docs.
3. Build the complete Vacation Reservation Ledger with **two immediately consecutive complete pagination passes**. Require:

```text
LedgerPaginationComplete=true
LedgerSnapshotStable=true
```

4. Re-read every affected PR's current base/head SHA and changed-file set.
5. Verify ancestry from Git objects; do not infer it from titles or old handoff text.
6. Finalize prerequisite roots before stacked children.
7. Use PowerShell 7.6.5 for Windows/WSL validation. Native Linux validation follows `docs/testing.md` from `/home`, never `/mnt/c`.
8. Run only scope-triggered gates plus directly caused corrections.
9. Git mutations, remote updates, branch deletion, force operations and merges remain separate authorization boundaries.

---

## 3. Integration Wave A — shared authorization foundation

This is the highest-reuse prerequisite because both `BL-118` and `BL-157` consume server-side capability authorization, and later security work (`BL-160` and parts of `BL-163`) also depends on it.

### A1. Functional capability model

- **PR #105 — BL-100**
- Head branch: `codex/fuhre-nachsten-cloud-task-aus-mobile.md-v3-aus`
- Head SHA: `3ca7584e00690b8f09bccb8401e555bc9cf3985f`
- Base at preparation: `main`
- Purpose: closed Version-1.0 functional capability vocabulary including `process.observe` and `system.read`.

### A2. Execution-time authorization

- **PR #181 — BL-159**
- Head branch: `codex/fuhre-bl-159-gema-mobile.md-v3-aus`
- Head SHA: `9a1aef868c2313efece7c5f8d2382d5f89b58990`
- Direct base: PR #105 head (`3ca7584e...`)
- Purpose: authorization immediately before `tools/call` execution; tool registration/catalog visibility is not the authority boundary.

### Integration intent

Finalize and integrate the BL-100 → BL-159 ancestry as one coherent line. Do not merge the child without first proving the BL-100 content is present in the integration candidate.

### Windows/Classic validation focus

- capability-set construction and fail-closed unknown identifiers;
- `tools/list` visibility versus independent `tools/call` authorization;
- direct/crafted calls cannot bypass authorization;
- standard Go tests/race/vet/build affected by router/tool registry;
- PowerShell 7.6.5 documentation/contract gates;
- no stdout diagnostics regression.

### Expected unlock effect

After this line is on current `main`, rerun classification for at least:

`BL-118`, `BL-157`, `BL-160`, `BL-163`.

It is not sufficient by itself to make those tasks executable; it removes their shared authorization-side split.

---

## 4. Integration Package B — unlock BL-118 (`process.observe`)

### 4.1 Process observation topology

The observation work is **not one linear tip**. There are sibling branches.

#### Root

- **PR #188 — BL-114**
- Head: `codex/execute-next-cloud-task-from-mobile.md-v3-uo7g1e`
- SHA: `fe78d918bcfcef9c4108fac0e0fca961212044c5`
- Prepared from `main`.
- Implements bounded unregistered `list_processes`.

#### Sibling A

- **PR #191 — BL-115**
- Head: `codex/fuhre-bl-115-gema-mobile.md-v3-aus`
- SHA: `8f945ec18d5c5b717eb55bf8e43ef9fb0fedb007`
- Base: PR #188 head.
- Adds process field selection.

#### Sibling B / tree chain

- **PR #192 — BL-116**
- Head: `codex/fuhre-cloud-task-aus-mobile.md-v3-aus-mfvj86`
- SHA: `1ae5a4882cb2fa85c65bfc5a0a780575dfaa4390`
- Base: PR #188 head.
- Adds process details.

- **PR #193 — BL-117**
- Head: `codex/fuhre-bl-117-gema-mobile.md-v3-aus`
- SHA: `e0043ac51190f0a239ba9412546fe088226d7d30`
- Base: PR #192 head.
- Adds bounded process tree.

**Important:** PR #193 does **not** contain BL-115 merely because it is the numerically latest process-observation PR. BL-115 and BL-116/117 diverge from BL-114 and must be converged explicitly.

### 4.2 Additional authorization line

Requires the shared Wave-A authorization foundation:

- PR #105 / BL-100
- PR #181 / BL-159

At the final rebind, also inspect **PR #129 / BL-162 Process Policy Model** as the normative policy contract. Current Mobile topology did not treat BL-162 as a separate implementation predecessor for `BL-118`, but its contract must not be contradicted.

### 4.3 Preferred Windows integration sequence

1. Finalize/integrate PR #188 (`BL-114`).
2. Rebind and converge PR #191 (`BL-115`) onto the new main.
3. Rebind PR #192 (`BL-116`) onto the new main.
4. Rebind/integrate PR #193 (`BL-117`) after #192.
5. Ensure Wave A (`BL-100 → BL-159`) is present.
6. Rerun the Mobile concrete-delta test for `BL-118`.

### 4.4 Native/focused validation

Windows:

- real Toolhelp process listing/details/tree;
- access-denied versus missing behavior;
- bounded page/depth/node behavior;
- no command line/environment/user/path leakage;
- field-selection and serialization/schema parity;
- process observation remains inaccessible until `process.observe` registration/authorization exists.

Portable:

- `go test ./internal/process ./internal/mcp/tools ./cmd/server`;
- race tests for process/tool packages;
- `go vet ./...`, build, diff checks;
- affected schema/catalog/smoke gates.

### 4.5 Expected next Cloud task

If fresh post-integration analysis confirms no additional hard predecessor:

```text
ExpectedCandidate = BL-118
ExpectedClass     = MAIN_EXECUTABLE or STACK_BASE_READY
```

`BL-118` then owns registration plus execution-time authorization of the process-observation tools under `process.observe`.

---

## 5. Integration Package C — unlock BL-157 (`system.read`)

### 5.1 System-information topology

This family is also not a single linear branch.

#### System info root

- **PR #132 — BL-153**
- Head: `codex/fuhre-nachsten-cloud-task-aus-myz7pk`
- SHA: `20e2d97a640879564a428b29726fa9c6646abf69`
- Base: `main` at preparation.
- Adds privacy-safe `system_info`.

#### Siblings on BL-153

- **PR #213 — BL-155**
- Head: `codex/starte-neuen-codex-cloud-task-jixm80`
- SHA: `5b0481344dd978d49a13f56c130767b352231c2f`
- Base: PR #132 head.
- Adds filtered environment information.

- **PR #215 — BL-156**
- Head: `codex/fuhre-bl-156-als-abhangigen-task-aus`
- SHA: `4313299a3ac2b83e154c79d6d1a1dd600dd67c9c`
- Base: PR #132 head.
- Adds field selection/redaction.

PR #213 and PR #215 are siblings; neither should be assumed to contain the other.

#### Separate disk-usage line

- **PR #211 — BL-154**
- Head: `codex/starte-neuen-codex-cloud-task`
- SHA: `f4be9937ae78cf1f82191488f9ec0bdd02d27cce`
- Consumes the root-scoped disk-usage provider (`BL-062`) on a separate ancestry.

### 5.2 Authorization line

Requires Wave A:

- PR #105 / BL-100 (`system.read` exists in capability vocabulary)
- PR #181 / BL-159 (execution-time authorization)

### 5.3 Preferred Windows integration sequence

1. Finalize/integrate BL-153 root (#132).
2. Rebind and converge BL-155 (#213).
3. Rebind and converge BL-156 (#215).
4. Independently finalize BL-154 (#211) with its actual BL-062 ancestry; integrate it when its root-scoped dependency is clean.
5. Ensure BL-100→BL-159 is present.
6. Rerun `BL-157` concrete-delta feasibility. Do **not** assume BL-154 is a hard implementation predecessor if the exact BL-157 delta only gates `system_info`; conversely, do not omit it if `system.read` is defined over the complete exposed system-information surface.

### 5.4 Validation focus

Windows/Linux:

- real system-info provider behavior;
- field omission/selective disclosure;
- filtered environment allowlist and secret exclusion;
- disk usage remains root-scoped and path/volume-identity safe;
- `system.read` catalog registration and post-resolution authorization;
- negative direct-call bypass tests;
- schema/catalog/wire-size parity after convergence.

### 5.5 Expected next Cloud task

After fresh reclassification:

```text
ExpectedCandidate = BL-157
```

The expected implementation is registration plus server-side execution checks for `system.read`, not a new system-info provider.

---

## 6. Integration Package D — unlock BL-139 (command working-directory policy)

### 6.1 Command ancestry

- **PR #107 — BL-136** Command Execution Threat Model
  - Head `codex/fuhre-cloud-task-aus-mobile.md-v3-aus-ztndq7`
  - SHA `02bbdc19b9f7067b7d033e7cc55378f612c1c95f`

- **PR #186 — BL-137** Typed command definitions
  - Head `codex/fuhre-bl-137-gema-mobile.md-v3-aus-cual5t`
  - SHA `760b83d5e97da5cb8031c9051e3e7fab939fe361`
  - Base: PR #107 head.

- **PR #198 — BL-138** No-shell typed argument construction
  - Head `codex/fuhre-bl-138-gema-mobile.md-v3-aus`
  - SHA `ef8bf0d567055423baa3397452f726479d9fcb87`
  - Base: PR #186 head.

This is a genuine cumulative command line through BL-138.

### 6.2 Named-root working-directory ancestry

The immediate required tip is:

- **PR #189 — BL-109** Process working-directory permission per root
  - Head `codex/fuhre-bl-109-als-mobile-task-aus`
  - SHA `23b237c452c60bc0ad1dc90fef30186deefec9d7`
  - Base: PR #187 (`BL-108`) head.

Observed preceding segment:

- PR #185 — BL-107, head `2a9ca62e7098c2f5bef9e6b263a42a8dcb89cb1a`
- PR #187 — BL-108, head `64fd0d7b1ca5ffb10db2a0ab1c027e071e3e9e20`
- PR #189 — BL-109, head `23b237c...`

Before integration, enumerate the **complete** ancestry of PR #189 back to its root; do not use only the last three PRs as a substitute for ancestry proof.

### 6.3 Why BL-139 is blocked today

`BL-139` must connect typed command path/argument handling with the independent named-root `WorkingDirectoryRoot` authorization gate. Neither PR #198 nor PR #189 contains the other line.

### 6.4 Preferred integration sequence

1. Finalize the complete named-root ancestry through PR #189.
2. Finalize the command ancestry #107 → #186 → #198.
3. Bring both onto current `main` with full conflict/review/test gates.
4. Rerun Mobile classification for `BL-139`.

### 6.5 Validation focus

- native Windows absolute/relative working-directory semantics;
- symlink/junction/reparse and effective-root containment;
- explicit allow/deny roots, unknown root, default fail-closed behavior;
- command argument path handling and traversal rejection;
- no request-supplied executable/shell/interpreter path;
- command package + roots package race tests;
- documentation/security/testing consistency.

### 6.6 Expected next Cloud task

```text
ExpectedCandidate = BL-139
```

Its delta should be narrow: bind command working-directory selection to the already integrated named-root working-directory authorization contract.

---

## 7. Integration Package E — unlock BL-341 (largest leverage)

`BL-341` is the strategic integration hinge for later multi-mode runtime work. Its planning, architecture contract and documentation review are already complete; runtime implementation remains `NOT_STARTED / PLANNED`.

### 7.1 Three currently independent prerequisites

#### Operations/Job shutdown

- **PR #81 — BL-094**
- Head: `codex/nachsten-cloud-task-aus-mobile.md-v3-ausfuhren`
- SHA: `b6e95a4417874708a68c3e69f366e26030bcee0b`
- Base: `main` at preparation.
- Provides bounded two-phase Operations/Job shutdown owned by the operation domain.

#### Managed Child lifecycle contract

- **PR #134 — BL-129**
- Head: `codex/fuhre-nachsten-cloud-task-aus-mobile.md-v3-aus-g7bviy`
- SHA: `ee0016c32c5c8b31545ab6f6398db13ca4ea21e0`
- Base: `main` at preparation.
- Defines authoritative managed-child ownership, PID/start identity, bounded cleanup, restart generation and orphan rules.

#### Process-root coordinator / transport-neutral runtime

- **PR #214 — BL-224**
- Head: `codex/fuhre-nachsten-cloud-task-aus-1i1kcm`
- SHA: `6179bf1ad97147d314a16f4f523769ccae45326d`
- Base: `main` at preparation.
- Adds one process-root lifecycle coordinator and transport-neutral MCP runtime.

These are three independent main-based lines. No existing open head contains all three.

### 7.2 Preferred Windows/Classic integration order

1. BL-094 / PR #81 — establish the concrete Operations/Job shutdown owner.
2. BL-129 / PR #134 — establish managed-child lifecycle/ownership contract.
3. BL-224 / PR #214 — integrate the process-root coordinator against the then-current repository and confirm the owner-registration boundary remains compatible.
4. Rerun `BL-341` feasibility from fresh main.

This order is semantic, not authority to merge. Fresh conflict analysis may justify a different mechanical order, but the final state must contain all three contracts without weakening domain ownership.

### 7.3 BL-341 implementation target after convergence

The eventual BL-341 implementation owns:

- Direct STDIO and proxy/auto-edge host ownership;
- definitive EOF/transport failure/OS stop/verified owner loss handling;
- optional explicitly negotiated lease signals where applicable;
- bounded deterministic process-root shutdown;
- Operations/Job cleanup via the BL-094 owner;
- Managed Child cleanup via the BL-129 owner;
- PID + process-start identity / verified OS handle, never PID-only authority;
- secret-safe diagnostics and typed exit classification;
- stale runtime-registry cleanup;
- native Windows/Linux behavior;
- no idle/age/CPU/request-count/singleton heuristic termination.

### 7.4 Native validation focus

Windows:

- EOF and broken-pipe paths;
- owner death and duplicated/retained pipe handles;
- process-start identity/PID reuse protection;
- bounded shutdown with operation + managed-child cleanup;
- no orphaned session-scoped hosts/owned children after conclusive loss + shutdown window;
- real Job Object/child cleanup as applicable;
- no false-positive termination during legitimate idle.

Native Linux:

- same ownership/lifecycle matrix under `/home` copy;
- signal/transport close/owner-loss behavior;
- process-tree cleanup and PID reuse assumptions;
- no `/mnt/c` native validation.

Portable:

- lifecycle coordinator idempotence/order/error aggregation;
- Operations shutdown concurrency/race coverage;
- managed child lifecycle/race coverage;
- diagnostics redaction and stable exit categories.

### 7.5 Expected unlock effect

After BL-341 itself is implemented/integrated, rerun at least:

`BL-228`, `BL-229`, `BL-230`, `BL-231`, `BL-234`, `BL-241`, `BL-242`, `BL-244`.

ADR-014 makes BL-341 a mandatory lifecycle dependency for BL-226 through BL-231.

---

## 8. Multi-mode follow-on lines already prepared

These should **not** be combined synthetically in Cloud today, but they are valuable ready inputs after BL-341 convergence.

| Function | PR / BL | Head SHA | Topology note |
|---|---|---|---|
| IPC contract | #210 / BL-225 | `2964450a943907beb0cde0847dfe89a245d264c6` | main-based contract |
| Linux UDS transport | #255 / BL-227 | `e8366ac0ad5506de5944902a7ed9c21639dc6bb8` | child of BL-225 |
| Windows Named Pipe transport | #256 / BL-226 | `f11b0a427b12410e4660cc420dca1657808fa5d1` | child of BL-225 |
| runtime config / discovery | #205 / BL-233 | `e084895636e92e22f0b85f9e97d6e914f740ce81` | independent main-based line |
| execution identity interface | #220 / BL-236 | `57d0b7d04cccf1d99e02679fcba32d1018fc5217` | main-based line |
| Variant A service account | #257 / BL-237 | `d92002cf7ea4d02e7bf8aedd8385fee1be9fe519` | child of BL-236 |

After BL-341 integration, these lines become candidates for structured convergence toward proxy/auto/service work. `BL-229` still must not be started from BL-233 alone; it needs the lifecycle dependency too.

---

## 9. Recommended actual integration queue on return to Windows

This ordering maximizes reuse and early Mobile reactivation.

### Wave 1 — authorization shared by multiple domains

1. PR #105 / BL-100
2. PR #181 / BL-159
3. Fresh reclassification

### Wave 2 — process observation

1. PR #188 / BL-114
2. PR #191 / BL-115
3. PR #192 / BL-116
4. PR #193 / BL-117
5. Fresh reclassification → target `BL-118`

### Wave 3 — system information

1. PR #132 / BL-153
2. PR #213 / BL-155
3. PR #215 / BL-156
4. PR #211 / BL-154 with its own BL-062 ancestry when ready
5. Fresh reclassification → target `BL-157`

### Wave 4 — command working directory

1. complete named-root ancestry through PR #189 / BL-109
2. command ancestry #107 → #186 → #198
3. Fresh reclassification → target `BL-139`

### Wave 5 — lifecycle hinge

1. PR #81 / BL-094
2. PR #134 / BL-129
3. PR #214 / BL-224
4. Fresh reclassification → target `BL-341`

### Wave 6 — Multi-mode consolidation

After BL-341 implementation/integration:

- BL-225 transport contract + BL-226/227 transports;
- BL-233 discovery/configuration;
- BL-236→237 execution identity;
- then re-evaluate BL-228–231 and BL-234;
- only after integrated runtime exists, re-evaluate BL-241/242/244.

---

## 10. Mapping of all 21 WAIT_MULTI topics to likely unlock waves

| Topic | Primary unlock dependency / wave |
|---|---|
| BL-064 | filesystem BL-060/061 + Operations/Job integration |
| BL-118 | Wave 1 + Wave 2 |
| BL-139 | Wave 4 |
| BL-143 | command core + Managed Process convergence (separate later package) |
| BL-146 | command + Managed Process + execution identity/native isolation |
| BL-147 | command + Managed Process + execution identity/native isolation |
| BL-152 | integrated command/root/env/output/timeout/isolation surfaces |
| BL-157 | Wave 1 + Wave 3 |
| BL-160 | Wave 1 + integrated root/domain policy surfaces |
| BL-163 | command/root/environment/limits/isolation convergence |
| BL-167 | process + execution + system + jobs + audit convergence |
| BL-168 | execution identity/service account + child/platform isolation |
| BL-228 | BL-341 + IPC/runtime/transport convergence |
| BL-229 | BL-341 + BL-233 discovery/config contract |
| BL-230 | BL-341 + Windows transport + Variant A execution identity |
| BL-231 | BL-341 + Linux transport + Variant A execution identity |
| BL-234 | transport caller identity + roots/capabilities + execution identity + quotas + audit |
| BL-241 | integrated multi-mode runtime first |
| BL-242 | integrated modes/tests first |
| BL-244 | integrated modes first; benchmark only after runnable modes coexist |
| BL-341 | Wave 5 |

---

## 11. Do not lose this work — persistence contract

This planning output is retained in the FlashGate repository and must not rely on chat memory. Keep the original external Markdown artifact and SHA-256 digest as source provenance; the repository copy is the durable project-visible handoff.

When this file is refreshed or superseded, first use a fresh rebind and update the header with:

- then-current `main` SHA/tree;
- then-current `BACKLOG.md`, `MOBILE.md`, `AGENTS.md`, and Mobile-governance blob hashes;
- current complete/stable PR-ledger count;
- whether any listed PR was merged, closed, retargeted, superseded, or changed head SHA;
- a short supersession note rather than silently overwriting historical assumptions.

Prefer a dated successor or explicit supersession note when the integration plan materially changes; do not silently rewrite historical assumptions.

Do not treat this file as canonical status. `BACKLOG.md`, governance, ADRs and current repository truth remain authoritative.

---

## 12. Immediate next action when Windows is available

Run a **read-only fresh integration preflight** for Wave 1 and Wave 2 before authorizing any Git mutation. The preflight should return:

```text
MainSha=
MainTree=
LedgerPaginationComplete=true|false
LedgerSnapshotStable=true|false
OpenPRCount=
Wave1PRsCurrent=true|false
Wave2PRsCurrent=true|false
AncestorTopologyMatch=true|false
ChangedHeadCount=
ClosedOrMergedInputCount=
ConflictProjectionState=
PowerShellVersion=
NextSafeIntegrationBoundary=
MutationCount=0
```

Only after that evidence should a separate integration authorization select the first actual Git operation.
