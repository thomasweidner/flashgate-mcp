# FlashGate MCP — Wave 3 / Wave 4 Integration Preflight

**Status:** READ_ONLY_PREPARED_NOT_INTEGRATION_READY  
**Date:** 2026-09-17  
**Repository:** `thomasweidner/flashgate-mcp`  
**Bound main:** `e75e2371c0d1ae7c587286e75189f86fc5d3a4c0`  
**Bound main tree:** `018fbca90efdfc27aaeb2deb44a9d6b479be8b66`  
**Source planning document:** `docs/planning/mobile-integration-unblock-plan-2026-09-17.md`  
**Source planning blob:** `01e91def61e36ff547d9e86fa24333935ec9e4e6`  
**Open PR count:** `198`  
**LedgerPaginationComplete:** `true`  
**LedgerSnapshotStable:** `true`  
**MutationCount:** `0`
**AGENTS.md blob:** `079fad212fc4843eec70999cb551495b226087e6`  
**BACKLOG.md blob:** `a02db7c09b10dba8c827f62618c7c6eb9f096591`  
**Cloud governance blob:** `5a8295d61a0d13a624471fa3438c2e6157d9fb60`  
**Change-trigger adapter blob:** `07ac0d53520b70d71c8c96fd419d55134e890692`  
**Finding/review-mode standard blob:** `89622d7c04af0359b3dcc34e5ee5eeb637ced7f0`  
**Classic-readiness standard blob:** `4bdb44bb160b003969d5ccbc157d2415d9e4ed1e`  

This is read-only vacation preparation. It grants no Git/remote/merge/review-thread authority.

## Classic review authority

For this vacation analysis, GitHub/Codex review comments are **review signals only**. They are not governance-authoritative independent-review findings and they do not by themselves authorize correction.

When technical risk requires independent review, the binding review is performed in **Classic** as a separate read-only actor under the current governance:

- `INDEPENDENT_REVIEW`: read-only, no repository or external mutation, no correction;
- an implementing/correcting actor may self-validate but may not claim independent approval;
- only a Classic review disposition may promote a signal to a confirmed finding for remediation tracking;
- after a confirmed finding is corrected by the authorized correction actor, any required focused delta review is again Classic and read-only;
- Codex/Cloud comments may be used as evidence pointers or candidate hypotheses, but never as the required independent-review evidence.

On return to Windows/local work, rebind against the then-current central Slim Governance and applicable higher-level/local `AGENTS.md`; stricter/newer local authority wins.

## Vacation exit rule

The Mobile/Vacation workflow ends when the user returns from vacation.

```text
AfterVacation:
    MobileTaskSelection = OFF
    PrimaryWorkSource    = BACKLOG.md
    PrioritySource       = normal current backlog/governance
```

The Wave labels below are therefore **integration-preparation labels only**. They are not the normal post-vacation work queue. Current `BACKLOG.md` has Planned work beginning at SPR-048, well before the Mobile-derived packages below.

---

# Wave 3 preparation — system information

## Topology

### Root

- PR #132 — BL-153
- State: open
- Head: `20e2d97a640879564a428b29726fa9c6646abf69`
- Base at preparation: `main`
- Purpose: privacy-safe `system_info` provider/tool.

### Sibling A

- PR #213 — BL-155
- State: open
- Head: `5b0481344dd978d49a13f56c130767b352231c2f`
- Base: PR #132 head.
- Purpose: filtered environment information.

### Sibling B

- PR #215 — BL-156
- State: open
- Head: `4313299a3ac2b83e154c79d6d1a1dd600dd67c9c`
- Base: PR #132 head.
- Purpose: field selection/redaction.

Git-object comparisons confirm both #213 and #215 are exactly one commit ahead of #132. They are siblings and must later be converged deliberately.

### Separate disk-usage line

- PR #211 — BL-154
- State: open
- Head: `f4be9937ae78cf1f82191488f9ec0bdd02d27cce`
- Separate ancestry consuming BL-062 root-scoped disk usage.

## Observed GitHub/Codex review signals — pending Classic independent review

### PR #132 / BL-153 — P1: Windows version source is virtualized

Current implementation uses `syscall.GetVersion()`. The Codex signal alleges that on modern Windows this can return compatibility-shimmed version data when the executable lacks the appropriate supported-OS manifest.

**Candidate correction package if Classic confirms:**

1. Replace `syscall.GetVersion` with a manifest-independent Windows source, preferably `ntdll!RtlGetVersion` through existing Go/standard-library Windows syscall mechanisms.
2. Keep the output contract as bounded `major.minor.build`; do not add machine identifiers.
3. Treat API resolution/call failure as provider failure and preserve the public redacted error.
4. Add a unit seam around the native probe where practical, but require native Windows evidence that the returned build matches the running Windows version.
5. No new third-party dependency.

### PR #132 / BL-153 — P1: hard catalog budgets stale

At the PR head the hard benchmark budget still expects:

```text
read_only tool/schema count = 3
default   tool/schema count = 8
read_only max response      = 2134
default   max response      = 5657
```

The new `system_info` registration changes the catalog and wire size. The Codex signal reports hard budget failures.

**Candidate correction package if Classic confirms:**

1. Do not merely raise one byte ceiling.
2. Re-run the deterministic tools-list/workflow measurements on the corrected candidate.
3. Update `benchmarks/budgets.json` coherently for tool counts, schema counts, tools/list response/result bytes, `initialize_tools_list` response/result ceilings, and every other deterministic hard field actually changed.
4. Keep hard budgets tight to the corrected measured contract rather than adding arbitrary slack.
5. Re-run the permanent budget evaluator and all catalog/schema parity tests.
6. Performance-baseline work remains subject to the project's authoritative Windows benchmark path/time restrictions; wire-size contract regeneration is not a substitute for performance baselining.

### PR #213 / BL-155 — P2: environment values are allowlisted only by key

The branch allows only `COLORTERM`, `LANG`, `LC_ALL`, `LC_CTYPE`, and `TERM`, but copies the values verbatim. A permitted key can therefore still transport a token, path, username-like value, control data, or other sensitive content.

**Candidate correction package if Classic confirms:**

1. Keep the key allowlist closed.
2. Add conservative per-key value validation and strict byte/character length limits.
3. Unsafe values must be omitted or redacted/fail-safe; never returned verbatim merely because the key is allowlisted.
4. Reject/omit control characters, multiline data, path-like/URL-like or otherwise clearly non-locale/non-terminal forms according to a narrow documented policy.
5. Add negative tests with token-like strings, absolute paths, control characters, oversized values and malformed locale/terminal forms.
6. Preserve secret-safe diagnostics; never log rejected raw values.

If the exact accepted locale/terminal syntax cannot be defined without a policy decision, stop at that decision boundary rather than inventing a broad regex.

### PR #215 / BL-156 — P2: schema test does not test system-info schema

The new structured-result case passes `systemInfoToolName` through the filesystem schema helper, which returns no schema, so the validator accepts the result without meaningful checking.

**Candidate correction package if Classic confirms:**

1. Dispatch `system_info` cases to `systemInfoOutputSchema()`.
2. Prefer a common `toolOutputSchema()` dispatch if the repository's current abstraction supports all tool families.
3. Add negative schema-conformance cases so a missing/extra/wrong-typed selected field actually fails.
4. Keep catalog and runtime schemas deeply equal after sibling convergence.

### PR #211 / BL-154 — P1: Windows disk API receives file path

The Windows adapter passes a file path directly to `GetDiskFreeSpaceExW`, which requires a directory path; the PR's own Windows test includes a file input.

**Candidate correction if Classic confirms, subject to current public contract rebind:**

1. Preserve the intended root-scoped path contract.
2. If an accepted input resolves to a file, query disk capacity using its containing directory after the same root/effective-path validation.
3. If the target is a directory, use the directory itself.
4. Never widen the path beyond the validated root or expose volume names/host paths.
5. Add native Windows file + directory + missing/access-denied tests.

If the current authoritative contract instead says "directory only", then the public schema/tests must be narrowed consistently rather than silently normalizing files. Rebind before coding.

### PR #211 / BL-154 — P2: read-only profile conflicts with ADR-009

The PR exposes `get_disk_usage` in the existing read-only catalog while accepted ADR-009 still records the current restricted profile as exactly `list_directory`, `read_file`, `get_path_info`.

The better ownership boundary must be re-evaluated against BL-154/BL-157 before integration. Two possible outcomes exist: keep BL-154 implemented but unregistered until BL-157 owns `system.read` registration and execution authorization; or intentionally amend the profile/ADR as part of the owning capability-registration task. Do not silently leave two conflicting normative contracts.

## Wave 3 convergence risk

#213 and #215 overlap heavily in `README.md`, `cmd/server/tools_list_output_schema_test.go`, `docs/mcp-tool-catalog.json`, `docs/testing.md`, `docs/tools.md`, `internal/mcp/tools/output_schemas.go`, `internal/mcp/tools/output_schemas_test.go`, `internal/mcp/tools/tool_system_info.go`, and `internal/mcp/tools/tool_system_info_test.go`.

#213 additionally changes `internal/systeminfo`; #215 primarily changes field-selection/tool/schema logic.

**Proposed conflict-minimizing order after any Classic-confirmed root findings are corrected:**

```text
#132 corrected
    -> choose one sibling as convergence base
    -> integrate/rebase the other sibling explicitly
```

No current evidence makes one sibling universally safer. Select the order from a fresh diff once #132 is corrected, because both touch the same public schema. Do not assume PR number order.

## Wave 3 state

```text
Wave3State=PENDING_CLASSIC_REVIEW_OF_CODEX_SIGNALS_AND_SIBLING_CONVERGENCE
RootSignals=#132_P1,#132_P1
SiblingSignals=#213_P2,#215_P2
SeparateLineSignals=#211_P1,#211_P2
```

---

# Wave 4 preparation — command + named-root working-directory policy

## Command ancestry

```text
#107 BL-136  02bbdc19b9f7067b7d033e7cc55378f612c1c95f
  -> #186 BL-137 760b83d5e97da5cb8031c9051e3e7fab939fe361
       -> #198 BL-138 ef8bf0d567055423baa3397452f726479d9fcb87
```

Git-object comparisons confirm #186 is exactly one commit on #107 and #198 exactly one commit on #186.

## Named-root observed segment

```text
#185 BL-107 2a9ca62e7098c2f5bef9e6b263a42a8dcb89cb1a
  -> #187 BL-108 64fd0d7b1ca5ffb10db2a0ab1c027e071e3e9e20
       -> #189 BL-109 23b237c452c60bc0ad1dc90fef30186deefec9d7
```

Git-object comparisons confirm both displayed parent relations. The complete ancestry before #185 still must be re-enumerated before integration.

## Observed command review signals — pending Classic independent review

### PR #107 / BL-136 — P1: executable provenance

The threat model permits identity pinning only "where configured", yet a server-owned absolute path in a caller-writable location can still be replaced before launch.

**Candidate correction package if Classic confirms:**

The threat model must require one fail-closed provenance mechanism for every approved executable, not optional best effort. Acceptable architecture may include trusted ownership/write-permission validation for the executable and every relevant ancestor; mandatory identity/hash/publisher pinning appropriate to the platform; or another explicitly reviewed equivalent provenance control.

A server-owned absolute path and race-safe argv construction alone are not provenance. If the repository has not selected which provenance mechanism is mandatory for Version 1.0, stop at the security/architecture decision boundary instead of coding an assumption.

### PR #186 / BL-137 — P2: fixed `--` before declared flags

Definitions can contain `--` in fixed arguments and still declare flagged argument rules. Standard option parsers then interpret later flags as positional data.

**Candidate correction package if Classic confirms:**

1. Reject definitions where a fixed option terminator precedes any generated flagged argument.
2. Prefer constructor-time/catalog rejection, not request-time guessing.
3. Add tests for exact `--`, mixed fixed/flagged rules, and valid purely positional definitions.
4. Preserve deterministic argv ordering.

### PR #198 / BL-138 — P1: interpreter-backed definitions

The catalog can approve `/bin/sh -c`, PowerShell `-Command`, etc., which recreates free shell execution despite separate executable/argv fields.

**Candidate correction package if Classic confirms:**

1. Add a closed prohibited-interpreter identity/mode check during definition validation.
2. Cover Windows and Unix interpreter identities and execution modes used to evaluate program text.
3. Reject before any request value reaches invocation construction.
4. Tests must include shell/interpreter aliases/path forms relevant to supported platforms.
5. Do not introduce an interpreter exception without a separate explicit security decision.

### PR #198 / BL-138 — P1: inline prohibited selectors

Checks miss forms such as `--config=/tmp/x` or `--plugin=/tmp/y`.

**Candidate correction package if Classic confirms:**

1. Normalize option names before validation by separating the selector from inline `=value`.
2. Apply the existing prohibited-selector policy to both split and inline forms.
3. Cover hyphen/underscore normalizations already intended by the contract.
4. Add negative tests for config/plugin/loader-style inline forms.

### PR #198 / BL-138 — P2: negative positional integers can become options

A positional integer such as `-1` can be parsed by the target executable as an option.

**Candidate correction package if Classic confirms:**

1. For positional integer rules, disallow negative ranges unless the definition provides an explicit safe boundary mechanism that prevents option interpretation.
2. Constructor-time rejection is preferable.
3. Add tests for negative min/range, zero/positive valid values, and flagged negative values where the flag syntax makes interpretation unambiguous.

## Observed named-root review signals — pending Classic independent review

### PR #185 / BL-107 — P2: nil filesystem panic

`Single*` helpers derive link rules from a nil filesystem before the existing validation can return `ErrInvalidEntry`.

**Candidate correction package if Classic confirms:**

- nil-check before `linkRulesFor`, or move link-rule derivation behind validated construction;
- retain fail-closed typed error instead of panic;
- add nil tests for every affected constructor.

### PR #187 / BL-108 — P1: wrong ancestry, duplicates capability foundation

This is the most important Wave-4 structural Codex signal; Classic must independently verify the ancestry and ownership conflict.

BL-108 was prepared only on BL-107, but it consumes/duplicates functional-capability concepts owned by BL-100. The branch therefore cannot be treated as a valid independent named-root chain.

**Candidate correction topology if Classic confirms:**

```text
corrected BL-100 capability foundation
        +
corrected named-root ancestry through BL-107
        -> rebuild/rebase BL-108
             -> rebase BL-109
```

Do not solve this by keeping a second capability type/model inside the roots package. This directly couples the future BL-139 package to the Wave-1 BL-100 remediation.

### PR #187 / BL-108 — P2: requested access/capability mismatch

The resolver checks access and capability independently, allowing a write-capable root to be returned while only the read capability is supplied.

**Candidate correction package if Classic confirms:**

1. Derive the capability required by the requested access (`Read` -> read capability, `Write`/read-write operation -> write capability as appropriate).
2. Require the entry's capability set to satisfy that implied capability.
3. Reject mismatched caller-supplied combinations fail-closed.
4. Tests must cover read/read, write/write, read-write combinations and deliberate mismatches.

### PR #189 / BL-109

No current open inline Codex review signal was found on #189 itself, but it is downstream of the potentially invalid #187 ancestry. It must therefore be rebased/revalidated after #187 is reconstructed on the correct capability foundation.

## Wave 4 state

```text
Wave4State=PENDING_CLASSIC_SECURITY_REVIEW_AND_ANCESTRY_REVIEW
CommandSignals=#107_P1,#186_P2,#198_P1,#198_P1,#198_P2
RootSignals=#185_P2,#187_P1,#187_P2
BL109Signal=none_direct_but_parent_requires_Classic_disposition
CrossWaveDependency=corrected_BL100_required_for_BL108
```

---

# Post-vacation meaning

None of the above means "do Wave 3 then Wave 4" after the vacation.

The correct post-vacation model is:

1. Fresh repository/governance/backlog rebind.
2. Disable Mobile/Vacation selection.
3. Use normal `BACKLOG.md` priority/sprint flow.
4. Treat these prepared PRs, Classic review dispositions, and any later authorized correction packages as reusable evidence/input when their normal backlog owners are reached.
5. Reconcile old Mobile PRs with the then-current backlog instead of forcing the backlog to follow the vacation topology.

Current normal Planned sequence starts at SPR-048, not SPR-053/055/057/058.

---

# Recommended remaining phone work

Still read-only, high-value:

1. Build a **Review Remediation Register** for all currently prepared PRs, not only Waves 1–4.
2. Categorize each review signal after Classic disposition as local code correction, stale finding, invalid ancestry requiring rebuild, architecture/security decision, native Windows evidence, or duplicate/superseded PR.
3. Preflight Wave 5 / lifecycle and Wave 6 / multi-mode in the same way.
4. Prepare the vacation-exit amendment for the repository planning document.
5. Do not mutate/merge old PRs from the phone without a separate explicit Git authorization and, where required, native Windows finalization.
