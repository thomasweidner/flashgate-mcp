# FlashGate MCP — Classic Independent Review Batch 03

**Status:** `COMPLETE_FINDINGS_CONFIRMED_NO_FIXES_PERFORMED`  
**Date:** 2026-09-17  
**Review mode:** `INDEPENDENT_REVIEW`  
**Repository:** `thomasweidner/flashgate-mcp`  
**Bound main:** `1c6fcc121e7ee1c782134fec28d3bfa3b4326e4a`  
**Bound main tree:** `1dd4061ec4df7e3f8e622ef94a19ed17eddfdd5e`  
**RepositoryMutationAllowed:** `false`  
**ExternalMutationAllowed:** `false`  
**FindingFixesPerformed:** `false`  
**ReviewerIndependencePreserved:** `true`

This review was performed in Classic as a separate read-only review activity. GitHub/Codex review comments were treated only as evidence pointers and hypotheses. Every disposition below was independently checked against the reviewed PR code/diffs, current `BACKLOG.md`, and the current security/architecture authority.

No reviewed PR, branch, review thread, repository file, remote state, workflow, issue, or external system was modified during this review.

## Authority bound for this review

- `AGENTS.md` blob: `079fad212fc4843eec70999cb551495b226087e6`
- `BACKLOG.md` blob: `a02db7c09b10dba8c827f62618c7c6eb9f096591`
- `Governance/CLOUD-CODEX-GOVERNANCE.md`: `5a8295d61a0d13a624471fa3438c2e6157d9fb60`
- `Governance/CHANGE-TRIGGER-REVIEW-AND-BACKLOG-STANDARD.md`: `07ac0d53520b70d71c8c96fd419d55134e890692`
- `Governance/FINDING-REMEDIATION-AND-REVIEW-MODE-STANDARD.md`: `89622d7c04af0359b3dcc34e5ee5eeb637ced7f0`
- `Governance/HANDOFF-ARTIFACT-AND-CLASSIC-READINESS-STANDARD.md`: `4bdb44bb160b003969d5ccbc157d2415d9e4ed1e`
- ADR-009 capability/profile authority: `82607310523d4c44c3a5530acefc617819b18cf5`
- ADR-011 managed-process/command authority: `3e64a0c4cfbb0223110cf57a107002d49b63e5f1`

Relevant current backlog ownership:

- `BL-107`: per-root symlink/reparse rules
- `BL-108`: per-root capability mapping
- `BL-109`: process working-directory permission per root
- `BL-136`: command-execution threat model
- `BL-137`: typed command definitions and executable IDs
- `BL-138`: no-shell typed argument construction
- `BL-139`: working directories restricted to allowed roots

`BL-100` separately owns the functional capability vocabulary that BL-108 must consume rather than redefine.

## Reviewed PR bindings

| PR | BL | Head | Tree | Parent/base |
|---:|---|---|---|---|
| #107 | BL-136 | `02bbdc19b9f7067b7d033e7cc55378f612c1c95f` | `4c4d40ef33c8192941d6845299eb20fc88988c9c` | `5b851afb3ef3e7de1b49ce5a352c06d40b815f9d` |
| #186 | BL-137 | `760b83d5e97da5cb8031c9051e3e7fab939fe361` | `2851726c904e8971e0b42a1efe5ef8a9b0662d6f` | PR #107 |
| #198 | BL-138 | `ef8bf0d567055423baa3397452f726479d9fcb87` | `4e431a8abc4cba0f5721cd4ef3b3d9fa7e35262e` | PR #186 |
| #185 | BL-107 | `2a9ca62e7098c2f5bef9e6b263a42a8dcb89cb1a` | `a1f3bf660f0af944df74d1c58684d4a6689ebe6a` | BL-106 head `357ae231...` |
| #187 | BL-108 | `64fd0d7b1ca5ffb10db2a0ab1c027e071e3e9e20` | `64ae30679da55099b0ba76cc45341d87fc551885` | PR #185 |
| #189 | BL-109 | `23b237c452c60bc0ad1dc90fef30186deefec9d7` | `1dcbbb47db2549917b8c4704fe1086e4145eb9ac` | PR #187 |

# Findings — command execution line

## CR-PR107-001 — P1 — CONFIRMED
### The threat model permits unpinned executables without another mandatory provenance control

The BL-136 threat model requires a configured executable to resolve to an approved absolute native program path, but makes identity/fingerprint verification conditional: `When configured, verify its file identity or content fingerprint.` Its executable-substitution threat row similarly lists identity/fingerprint revalidation only "where configured".

An absolute path plus race-safe launch prevents request-controlled lookup and TOCTOU retargeting, but it does not prove that the executable at that path is trusted. If a less-trusted principal can replace the file or a writable ancestor controls replacement, the system can faithfully open and launch the attacker's replacement.

The same document promises that executable substitution is rejected. The contract therefore lacks a mandatory provenance rule for the case where the optional binary identity pin is absent.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required boundary:** BL-136 must require at least one fail-closed provenance mechanism for every approved executable, for example trusted ownership/write-permission validation, mandatory identity/publisher/content pinning for that definition, or another explicitly reviewed equivalent.

Because current BL-137 explicitly describes binary identity pinning as optional, choosing the exact Version-1 provenance rule may be an architecture/security decision boundary. This review does not choose that mechanism.

No correction was performed.

## CR-PR186-001 — P2 — CONFIRMED
### Fixed `--` can terminate option parsing before generated flagged rules

BL-137 permits arbitrary non-empty/NUL-free fixed arguments. A definition may therefore contain `"--"` in `FixedArguments` and also declare later flagged `ArgumentRules`.

BL-138 emits fixed arguments before all generated rules. For conventional option parsers, `--` terminates option parsing, so the supposedly typed flags generated afterward become positional operands. That violates the command definition's declared argument semantics.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required correction:** reject definitions where a fixed option terminator precedes any generated flagged rule, or represent an explicitly reviewed boundary that makes the later arguments unambiguously positional.

## CR-PR186-002 — P2 — NEW CLASSIC FINDING
### `ArgumentRule.Flag == "--"` is accepted as a normal flag

At the BL-138 head the flag validator rejects values that do not start with `-`, exactly `"-"`, flags containing `=`, whitespace, CR/LF, or NUL, and selected injection names. It does **not** reject exactly `"--"`.

`BuildInvocation` then handles `"--"` like any other flag: append `"--"`, then append the encoded value. For standard option parsing this is not a named flag at all; it is the option terminator. A rule declared as a flagged string/integer/enum/path can therefore silently turn its value into a positional operand. For a boolean rule, `true` emits only the terminator.

This is distinct from `CR-PR186-001`: that finding concerns a trusted fixed terminator before other flags, whereas this finding concerns the rule's own flag token being semantically invalid.

**Required correction:** reserve/reject `"--"` as an `ArgumentRule.Flag` at registry construction and add focused tests.

**Prior Codex signal:** none; this finding was produced by Classic review.

## CR-PR198-001 — P1 — CONFIRMED
### Interpreter-backed definitions can recreate free shell/program-text execution

The BL-136 threat model says Version 1.0 executable IDs cannot resolve to an interpreter. But the registry permits any clean absolute executable path and BL-138 does not reject interpreter executables or execution modes.

A server definition can therefore use, for example, `/bin/sh -c <request-controlled string>` or PowerShell `-Command <request-controlled string>`. The argv array remains structurally separate from the executable path, but the selected interpreter re-parses the request-controlled argument as program text. That recreates the free-shell/interpreter gateway BL-138 is meant to exclude.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required correction:** enforce the accepted Version-1 no-interpreter boundary at catalog validation using a closed, reviewable mechanism. An exception requires a separate explicit security/product decision.

## CR-PR198-002 — P1 — CONFIRMED
### Inline prohibited selectors bypass the fixed-argument injection check

`isInjectionSelector` strips leading hyphens and checks exact names or `fragment-` / `fragment_` prefixes for `config`, `hook`, `plugin`, `loader`, `interpreter`, `executable`, and `exec-path`.

A common fixed option such as `--config=/tmp/tool.conf` or `--plugin=/tmp/ext` normalizes to `config=/tmp/...` / `plugin=/tmp/...` and matches none of those conditions. The definition is accepted even though the option still selects configuration/plugin loading surfaces that the contract says the generic validation rejects.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required correction:** parse or reject inline option assignments before selector matching and add negative fixtures for every prohibited selector family.

## CR-PR198-003 — P2 — CONFIRMED
### Negative positional integers can be reinterpreted as options

`ValueInteger` emits the decimal integer directly. If a rule is positional (`Flag == ""`) and its validated range includes negative values, `-1` becomes a standalone argv token beginning with `-`.

Unlike positional string/enum/path values, integer values bypass `beginsOptionOrResponse`. For programs using option parsing, a declared positional integer can therefore become an option token.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required correction:** either prohibit negative positional integer ranges or require an explicit definition-level safe terminator/boundary that is validated in conjunction with the positional rules. Flagged negative values can be handled separately where the target parser's value boundary is explicit.

# Findings — named-root line

## CR-PR185-001 — P2 — CONFIRMED
### Single-root constructors can panic on a nil filesystem before `New` rejects it

`New` explicitly rejects `entry.FileSystem == nil`. But the `Single*` compatibility constructors derive link rules first via `LinkRules: linkRulesFor(filesystem)` and `linkRulesFor` immediately calls `filesystem.PathPolicy()`.

A nil filesystem therefore panics before the existing fail-closed `ErrInvalidEntry` validation can run.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required correction:** validate nil before deriving link rules or perform rule derivation only after the entry's filesystem has passed validation. Add tests for each affected compatibility constructor.

## CR-PR187-001 — P1 — CONFIRMED
### BL-108 is built on the wrong ancestry and duplicates the BL-100 capability foundation

Current `BACKLOG.md` assigns the functional capability model to BL-100 and per-root capability mapping to BL-108.

The BL-100 prepared implementation defines the closed Version-1 vocabulary in `internal/capability`, including the stable string identifiers `filesystem.read`, `filesystem.write`, and the other Version-1 rights.

PR #187 is stacked only on BL-107 and independently introduces another capability model inside the roots package using a `uint8` bitset with `FilesystemRead` and `FilesystemWrite`.

That creates two functional-capability types/owners for the same rights and allows the per-root mapping to evolve independently from the canonical capability vocabulary.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required correction:** rebuild/rebase BL-108 from an ancestry containing the corrected BL-100 capability foundation plus the named-root ancestry through BL-107. The root policy should consume the canonical capability identifiers/model rather than define a parallel one.

PR #189 must then be rebased on that corrected parent.

## CR-PR187-002 — P2 — CONFIRMED
### `RootWithCapability` does not bind the requested capability to the requested access

Registry construction correctly prevents a root's configured capabilities from exceeding its coarse `Access`.

But resolution checks `required` and `requiredCapability` independently and never requires `requiredCapability` to equal or cover the capability implied by `required`.

For example, an entry with `Access=ReadWrite` and `Capabilities=FilesystemRead` can satisfy `RootWithCapability(id, Write, FilesystemRead)`: the write access check passes and the separate read-capability check passes, so the root is returned even though the requested operation is write and the mapped functional capability is read.

Current filesystem tool callers construct the matching pair, so this does not demonstrate a present tool-level bypass. But `RootWithCapability` is itself the security API BL-108 introduces for current/future resolver callers, and its contract says both access and functional capability authorize the requested operation.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required correction:** derive/validate the functional capability from the requested access inside the authoritative resolver rather than trusting callers to provide a semantically matching pair.

# PR #189 / BL-109 disposition

No direct security/correctness finding was identified in the isolated BL-109 delta during this Classic review.

`ProcessWorkingDirectory` is intentionally an independent per-root permission; the current delta denies by default for the compatible single-root bootstrap, returns `ErrUnknownRoot` for unknown IDs, returns `ErrWorkingDirectoryDenied` when the permission is absent, and does not claim that filesystem read/write capability grants imply working-directory permission.

However, the PR is stacked on PR #187. Because BL-108 has a confirmed P1 ancestry/model defect and a confirmed resolver-contract defect, the current PR #189 head cannot be integrated as-is.

```text
PR189DirectFindingCount=0
PR189State=BLOCKED_PARENT_REBUILD_REQUIRED
```

After BL-108 is rebuilt on the corrected capability foundation, BL-109 must be rebased and revalidated, including the then-current root/capability types.

# Review summary

```text
ReviewedPRCount=6
ConfirmedPriorCodexSignals=8
RejectedPriorCodexSignals=0
NewClassicFindings=1
P1FindingCount=4
P2FindingCount=5
OpenFindingCount=9
ParentBlockedNoDirectFindingCount=1
FindingFixesPerformed=false
RepositoryMutationCount=0
ExternalMutationCount=0
ReviewerIndependencePreserved=true
```

Per PR:

```text
PR107 = FAIL_REMEDIATION_OR_DECISION_REQUIRED  (1x P1)
PR186 = FAIL_REMEDIATION_REQUIRED              (2x P2; inherits PR107)
PR198 = FAIL_REMEDIATION_REQUIRED              (2x P1, 1x P2; inherits PR107/PR186)
PR185 = FAIL_REMEDIATION_REQUIRED              (1x P2)
PR187 = FAIL_REBUILD_REMEDIATION_REQUIRED      (1x P1, 1x P2; requires corrected BL100 ancestry)
PR189 = BLOCKED_PARENT_REBUILD_REQUIRED        (0 direct findings)
```

Green historical CI/cross-build evidence does not supersede these findings, and native Windows/Linux security evidence remains separate.

## Correction / continuation boundary

This review authorizes **no correction**.

Parent-first correction order:

1. Resolve the BL-136 executable-provenance contract/decision before treating BL-137/138 as security-complete.
2. Correct PR #186's option-terminator validation, including both fixed-argument and rule-flag cases.
3. Rebase/correct PR #198 on the corrected BL-137 parent and address interpreter-backed definitions, inline selector assignment, and negative positional integers.
4. Correct PR #185's nil-filesystem constructor path.
5. Use the corrected BL-100 capability foundation plus corrected named-root ancestry through BL-107 to rebuild PR #187 / BL-108.
6. Rebase PR #189 / BL-109 on the corrected BL-108 parent and revalidate its isolated delta.
7. BL-139 working-directory integration remains later work and must consume the corrected command and root lines rather than synthesize the current flawed heads.

After any authorized correction bundle, use the required **Classic read-only focused independent delta review**. Do not use Codex review as independent-review evidence.

## Post-vacation priority

This batch does not establish the post-vacation work queue.

```text
MobileTaskSelection=OFF
PrimaryWorkSource=BACKLOG.md
```

On return, rebind the current normal backlog/governance and apply this review evidence only when its owning normal-backlog work is reached.
