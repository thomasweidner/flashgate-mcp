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
**ADR-017 blob:** `248fac8d422fc7fe43367ea48586ecb558bf2b4c`  
**Open PR ledger:** `198` open PRs  
**LedgerPaginationComplete:** `true`  
**LedgerSnapshotStable:** `true`  
**Source artifact SHA-256:** `3b5ff2c85cdd772445ccb8706c52b339b3208e435fafcce9992a5b24f482c34b`

This file is a durable planning artifact. It is **not** canonical backlog status and grants **no Git/remote/merge authority**. Before any integration or new Mobile implementation, bind the then-current repository, `main`, working tree, authoritative governance, `BACKLOG.md`, `MOBILE.md`, directly affected ADRs/docs and a fresh selector-stable open-PR ledger.

This version was rebound from the prior durable artifact immediately before repository publication. The bound repository state and ledger facts above are the publication preimage; future execution still requires a fresh rebind.

## 1. Current conclusion

The remaining active Mobile queue contains 21 `WAIT_MULTI` topics. The current state is an **integration bottleneck**, not a lack of useful work. Existing open PRs contain most prerequisite implementation pieces, but the pieces are split across independent PR lineages.

Current Mobile snapshot:

```text
MAIN_EXECUTABLE     : 0
PR_STACK_CANDIDATE  : 0
WAIT_MULTI          : 21
```

`WAIT_MULTI` is a snapshot annotation, not a permanent classification. Recompute it after every material topology change.

## 2. Mandatory fresh-rebind gate before every integration wave

Before any write, retarget, rebase, merge, or new child implementation:

1. Bind repository, branch, `main` HEAD/tree and working tree.
2. Read current `AGENTS.md`, `Governance/MOBILE-CLOUD-HANDOFF.md`, `BACKLOG.md`, `MOBILE.md`, affected ADRs, security and testing guidance.
3. Build the complete Vacation Reservation Ledger with **two immediately consecutive complete pagination passes** and require:

```text
LedgerPaginationComplete=true
LedgerSnapshotStable=true
```

4. Re-read every affected PR's current base/head SHA and changed-file set.
5. Verify ancestry from Git objects; never infer it from PR number, title, or stale handoff text.
6. Finalize prerequisite roots before stacked children.
7. Use PowerShell 7.6.5 for Windows/WSL validation. Native Linux validation follows `docs/testing.md` from `/home`, never `/mnt/c`.
8. Run only scope-triggered gates plus directly caused corrections.
9. Git mutations, remote updates, branch deletion, force operations and merges remain separate authorization boundaries.

## 3. Wave A — shared authorization foundation

This is the highest-reuse prerequisite because `BL-118`, `BL-157`, `BL-160`, and parts of `BL-163` consume server-side capability authorization.

### 3.1 Functional capability model

- **PR #105 — BL-100**
- Head: `codex/fuhre-nachsten-cloud-task-aus-mobile.md-v3-aus`
- SHA: `3ca7584e00690b8f09bccb8401e555bc9cf3985f`
- Base at preparation: `main`
- Provides the closed Version-1.0 capability vocabulary, including `process.observe` and `system.read`.

### 3.2 Execution-time authorization

- **PR #181 — BL-159**
- Head: `codex/fuhre-bl-159-gema-mobile.md-v3-aus`
- SHA: `9a1aef868c2313efece7c5f8d2382d5f89b58990`
- Direct base: PR #105 head.
- Enforces authorization immediately before `tools/call` execution; catalog visibility is not the authorization boundary.

### 3.3 Integration intent and validation

Finalize and integrate BL-100 → BL-159 as one coherent ancestry. Do not merge the child without proving BL-100 content is present.

Validate capability construction, fail-closed unknown identifiers, `tools/list` visibility versus post-resolution `tools/call` authorization, crafted direct-call denial, affected Go/race/vet/build gates, PowerShell 7.6.5 documentation gates, and stdout purity.

After this line reaches `main`, rerun classification for `BL-118`, `BL-157`, `BL-160`, and `BL-163`.

## 4. Package B — unlock BL-118 (`process.observe`)

### 4.1 Actual process-observation topology

The observation work is **not one linear tip**.

- **PR #188 — BL-114**
  - Head `codex/execute-next-cloud-task-from-mobile.md-v3-uo7g1e`
  - SHA `fe78d918bcfcef9c4108fac0e0fca961212044c5`
  - Root of the current observation implementation; bounded unregistered `list_processes`.

- **PR #191 — BL-115**
  - Head `codex/fuhre-bl-115-gema-mobile.md-v3-aus`
  - SHA `8f945ec18d5c5b717eb55bf8e43ef9fb0fedb007`
  - Sibling child of PR #188; adds process field selection.

- **PR #192 — BL-116**
  - Head `codex/fuhre-cloud-task-aus-mobile.md-v3-aus-mfvj86`
  - SHA `1ae5a4882cb2fa85c65bfc5a0a780575dfaa4390`
  - Sibling child of PR #188; adds process details.

- **PR #193 — BL-117**
  - Head `codex/fuhre-bl-117-gema-mobile.md-v3-aus`
  - SHA `e0043ac51190f0a239ba9412546fe088226d7d30`
  - Child of PR #192; adds bounded process tree.

PR #193 does **not** contain BL-115 merely because it is the latest numbered process-observation PR. BL-115 and BL-116/117 diverge from BL-114 and must be converged explicitly.

### 4.2 Additional policy/authorization line

Requires Wave A (PR #105 / BL-100 and PR #181 / BL-159). At final rebind also inspect **PR #129 / BL-162 Process Policy Model** as the normative policy contract; current topology did not classify it as an additional runtime implementation predecessor, but the integration must not contradict it.

### 4.3 Preferred Windows integration sequence

1. Finalize/integrate PR #188 (`BL-114`).
2. Rebind and converge PR #191 (`BL-115`) onto current main.
3. Rebind PR #192 (`BL-116`) onto current main.
4. Rebind/integrate PR #193 (`BL-117`) after #192.
5. Ensure Wave A (`BL-100 → BL-159`) is present.
6. Rerun the concrete-delta test for `BL-118`.

### 4.4 Validation focus

Windows: real Toolhelp list/details/tree behavior, access-denied versus missing cases, bounded page/depth/node behavior, no command-line/environment/user/path leakage, field-selection and schema parity, and proof that observation stays inaccessible until `process.observe` is registered and authorized.

Portable: `go test ./internal/process ./internal/mcp/tools ./cmd/server`, relevant race tests, `go vet ./...`, build/diff checks, affected schema/catalog/smoke gates.

Expected post-convergence candidate:

```text
ExpectedCandidate = BL-118
```

## 5. Package C — unlock BL-157 (`system.read`)

### 5.1 Actual system-information topology

- **PR #132 — BL-153**
  - Head `codex/fuhre-nachsten-cloud-task-aus-myz7pk`
  - SHA `20e2d97a640879564a428b29726fa9c6646abf69`
  - Root system-info implementation.

- **PR #213 — BL-155**
  - Head `codex/starte-neuen-codex-cloud-task-jixm80`
  - SHA `5b0481344dd978d49a13f56c130767b352231c2f`
  - Sibling child of BL-153; filtered environment information.

- **PR #215 — BL-156**
  - Head `codex/fuhre-bl-156-als-abhangigen-task-aus`
  - SHA `4313299a3ac2b83e154c79d6d1a1dd600dd67c9c`
  - Sibling child of BL-153; field selection/redaction.

PR #213 and PR #215 are siblings; neither contains the other.

Separate line:

- **PR #211 — BL-154**
  - Head `codex/starte-neuen-codex-cloud-task`
  - SHA `f4be9937ae78cf1f82191488f9ec0bdd02d27cce`
  - Consumes the root-scoped disk-usage provider (`BL-062`) on another ancestry.

### 5.2 Authorization line and integration sequence

Requires Wave A (`BL-100 → BL-159`). Then:

1. integrate BL-153 root (#132);
2. rebind/converge BL-155 (#213);
3. rebind/converge BL-156 (#215);
4. independently finalize BL-154 (#211) with its actual BL-062 ancestry;
5. rerun `BL-157` concrete-delta feasibility.

Do not assume BL-154 is a hard implementation predecessor if the exact BL-157 delta only gates `system_info`; conversely, do not omit it if the effective `system.read` surface includes `get_disk_usage`.

Validate real system-info behavior, selective disclosure, environment allowlist/secret exclusion, root-scoped disk usage, post-resolution authorization, direct-call bypass negatives, and schema/catalog/wire parity.

Expected post-convergence candidate: `BL-157`.

## 6. Package D — unlock BL-139 (command working-directory policy)

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

Immediate required tip:

- **PR #189 — BL-109**
  - Head `codex/fuhre-bl-109-als-mobile-task-aus`
  - SHA `23b237c452c60bc0ad1dc90fef30186deefec9d7`
  - Base: PR #187 / BL-108.

Observed preceding segment:

- PR #185 / BL-107 — `2a9ca62e7098c2f5bef9e6b263a42a8dcb89cb1a`
- PR #187 / BL-108 — `64fd0d7b1ca5ffb10db2a0ab1c027e071e3e9e20`
- PR #189 / BL-109 — `23b237c452c60bc0ad1dc90fef30186deefec9d7`

Before integration enumerate the **complete** ancestry of PR #189 back to its root; this observed segment is not a substitute for ancestry proof.

### 6.3 Why BL-139 is blocked and how to unlock it

`BL-139` connects typed command path/argument handling with the independent named-root `WorkingDirectoryRoot` gate. Neither PR #198 nor PR #189 contains the other line.

Preferred sequence:

1. finalize the complete named-root ancestry through PR #189;
2. finalize command ancestry #107 → #186 → #198;
3. converge both on current `main` with conflict/review/test gates;
4. rerun Mobile classification for `BL-139`.

Validate native Windows working-directory semantics, symlink/junction/reparse/effective-root confinement, explicit allow/deny/default behavior, argument path/traversal rejection, no request-selected executable/shell/interpreter path, and command/roots race tests.

Expected post-convergence candidate: `BL-139`.

## 7. Package E — unlock BL-341 (largest leverage)

`BL-341` is the strategic hinge for later multi-mode runtime work. Planning, architecture and documentation contract work is already complete; runtime implementation remains `NOT_STARTED / PLANNED`.

### 7.1 Required contract and cleanup prerequisites

ADR-017 defines the implementation dependency flow as:

```text
BL-223 / BL-224 / BL-225 contract definition
    -> BL-341 implementation together with BL-226 through BL-231
    -> BL-241 integrated validation
    -> BL-263 release decision
```

The BL-341 runtime delta also consumes domain-owned cleanup contracts from `BL-094` and `BL-129`. Therefore all five contract/owner inputs below must be present before BL-341 is reclassified as executable.

#### CLI mode / role / lifetime contract

- **PR #195 — BL-223**
- Head: `codex/fuhre-nachsten-cloud-task-aus-mobile.md-v3-aus-cbj3s9`
- SHA: `3d687729af9e15c29c38ac9c5250a3d924fc228f`
- Base: `main` at preparation.
- Defines CLI grammar, runtime role/lifetime semantics, shutdown reasons, exit categories and validation obligations.

#### Process-root coordinator / adapter-domain boundary

- **PR #214 — BL-224**
- Head: `codex/fuhre-nachsten-cloud-task-aus-1i1kcm`
- SHA: `6179bf1ad97147d314a16f4f523769ccae45326d`
- Base: `main` at preparation.
- Adds the transport-neutral MCP runtime and one process-root lifecycle coordinator.

#### Local IPC session/lifecycle contract

- **PR #210 — BL-225**
- Head: `codex/fuhre-nachsten-cloud-task-aus-mobile.md-v3-aus-arnxhb`
- SHA: `2964450a943907beb0cde0847dfe89a245d264c6`
- Base: `main` at preparation.
- Defines IPC framing/handshake plus connection/session identity, disconnect behavior, cancellation, compatibility and optional negotiated lease semantics.

#### Operations/Job shutdown owner

- **PR #81 — BL-094**
- Head: `codex/nachsten-cloud-task-aus-mobile.md-v3-ausfuhren`
- SHA: `b6e95a4417874708a68c3e69f366e26030bcee0b`
- Base: `main` at preparation.
- Provides bounded two-phase Operations/Job shutdown while preserving domain ownership.

#### Managed Child lifecycle owner

- **PR #134 — BL-129**
- Head: `codex/fuhre-nachsten-cloud-task-aus-mobile.md-v3-aus-g7bviy`
- SHA: `ee0016c32c5c8b31545ab6f6398db13ca4ea21e0`
- Base: `main` at preparation.
- Defines managed-child ownership, PID/start identity, bounded cleanup, restart generation and orphan rules.

The five prerequisites are currently split across independent prepared lines; no existing open head contains the complete BL-341 predecessor set.

### 7.2 Preferred Windows/Classic integration order

1. BL-223 / PR #195 — establish CLI mode, role/lifetime, shutdown-reason and exit contracts.
2. BL-224 / PR #214 — integrate the process-root coordinator against that host-role contract.
3. BL-225 / PR #210 — establish connection/session identity, disconnect behavior and optional negotiated lease semantics.
4. BL-094 / PR #81 — establish the concrete Operations/Job shutdown owner.
5. BL-129 / PR #134 — establish managed-child lifecycle/ownership contract.
6. Rerun `BL-341` feasibility only after all five inputs are present.

This is a semantic plan, not merge authority. Fresh conflict and ancestry analysis may change the mechanical order, but the final pre-BL-341 state must contain all five inputs without weakening domain ownership.

### 7.3 BL-341 implementation target after convergence

The eventual implementation owns:

- Direct STDIO and proxy/auto-edge host ownership;
- definitive EOF/transport failure/OS stop/verified owner loss handling;
- explicitly negotiated lease signals where applicable;
- bounded deterministic process-root shutdown;
- Operations/Job cleanup through BL-094;
- Managed Child cleanup through BL-129;
- PID plus process-start identity or verified OS handle, never PID-only authority;
- secret-safe diagnostics and typed exit classification;
- stale runtime-registry cleanup;
- native Windows/Linux behavior;
- no idle/age/CPU/request-count/singleton heuristic termination.

### 7.4 Native validation focus

Windows: EOF/broken-pipe paths, owner death, retained/duplicated handles, PID reuse/process-start identity, bounded job + child cleanup, zero definitely orphaned session-scoped hosts/owned children after conclusive loss + shutdown window, relevant Job Object behavior, and no false-positive termination during legitimate idle.

Native Linux: same ownership/lifecycle matrix under `/home`, signal/transport/owner-loss behavior, process-tree cleanup and PID reuse; never validate natively from `/mnt/c`.

Portable: coordinator idempotence/order/error aggregation, Operations shutdown race coverage, managed-child lifecycle/race coverage, diagnostics redaction and stable exit categories.

### 7.5 Expected unlock effect

After BL-341 itself is implemented/integrated, rerun at least:

`BL-228`, `BL-229`, `BL-230`, `BL-231`, `BL-234`, `BL-241`, `BL-242`, `BL-244`.

## 8. Multi-mode follow-on lines already prepared

`BL-225` is deliberately **not** a follow-on: it belongs in Wave 5 before BL-341 reclassification.

Remaining prepared inputs for post-BL-341 convergence:

| Function | PR / BL | Head SHA | Topology note |
|---|---|---|---|
| Linux UDS transport | #255 / BL-227 | `e8366ac0ad5506de5944902a7ed9c21639dc6bb8` | child of BL-225 |
| Windows Named Pipe transport | #256 / BL-226 | `f11b0a427b12410e4660cc420dca1657808fa5d1` | child of BL-225 |
| runtime config / discovery | #205 / BL-233 | `e084895636e92e22f0b85f9e97d6e914f740ce81` | independent main-based line |
| execution identity interface | #220 / BL-236 | `57d0b7d04cccf1d99e02679fcba32d1018fc5217` | main-based line |
| Variant A service account | #257 / BL-237 | `d92002cf7ea4d02e7bf8aedd8385fee1be9fe519` | child of BL-236 |

After BL-341 integration these lines can converge toward proxy/auto/service work. `BL-229` must not start from BL-233 alone; it still needs the integrated lifecycle line.

## 9. Recommended integration queue on return to Windows

### Wave 1 — shared authorization

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

1. PR #195 / BL-223
2. PR #214 / BL-224
3. PR #210 / BL-225
4. PR #81 / BL-094
5. PR #134 / BL-129
6. Fresh reclassification → target `BL-341`

### Wave 6 — multi-mode convergence

After BL-341 implementation/integration:

- converge BL-226/227 transports with integrated BL-225 lifecycle contract;
- converge BL-233 discovery/configuration;
- converge BL-236→237 execution identity;
- re-evaluate BL-228–231 and BL-234;
- after runnable modes coexist, re-evaluate BL-241/242/244.

## 10. Mapping of all 21 WAIT_MULTI topics

| Topic | Primary unlock dependency / wave |
|---|---|
| BL-064 | filesystem BL-060/061 + Operations/Job integration |
| BL-118 | Wave 1 + Wave 2 |
| BL-139 | Wave 4 |
| BL-143 | command core + Managed Process convergence |
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
| BL-244 | integrated runnable modes first; benchmark afterward |
| BL-341 | Wave 5: BL-223 + BL-224 + BL-225 contracts plus BL-094 + BL-129 cleanup owners |

## 11. Persistence contract

This planning output is retained in the FlashGate repository and must not rely on chat memory. Keep the original external Markdown artifact and its SHA-256 digest as source provenance; the repository copy is the durable project-visible handoff.

When this file is refreshed or superseded, first use a fresh rebind and update the header with:

- then-current `main` SHA/tree;
- then-current `BACKLOG.md`, `MOBILE.md`, `AGENTS.md`, Mobile-governance and directly affected ADR blob hashes;
- current complete/stable PR-ledger count;
- whether any listed PR was merged, closed, retargeted, superseded, or changed head SHA;
- a dated supersession note rather than silently rewriting historical assumptions.

Do not treat this file as canonical status. `BACKLOG.md`, governance, ADRs and current repository truth remain authoritative.

## 12. Immediate next action when Windows is available

Run a **read-only fresh integration preflight** for Wave 1 and Wave 2 before authorizing any Git mutation. Return at least:

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
