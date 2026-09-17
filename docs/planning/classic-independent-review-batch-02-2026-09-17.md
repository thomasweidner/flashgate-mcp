# FlashGate MCP — Classic Independent Review Batch 02

**Status:** `COMPLETE_FINDINGS_CONFIRMED_NO_FIXES_PERFORMED`  
**Date:** 2026-09-17  
**Review mode:** `INDEPENDENT_REVIEW`  
**Repository:** `thomasweidner/flashgate-mcp`  
**Bound main:** `d0aa7dda7e2cbd9d1e1f13abd918e0ac335ccc97`  
**Bound main tree:** `a4ce47801a4ea0a08d8a959975e3ee4501488fc6`  
**RepositoryMutationAllowed:** `false`  
**ExternalMutationAllowed:** `false`  
**FindingFixesPerformed:** `false`  
**ReviewerIndependencePreserved:** `true`

This review was performed in Classic as a separate read-only review activity. GitHub/Codex comments were used only as candidate evidence pointers. Every disposition below was independently checked against the reviewed code/diff, current repository authority, and where needed the authoritative platform API contract.

No reviewed PR, branch, review thread, repository file, remote state, workflow, issue, or external system was modified during this review.

## Authority bound for this review

- `AGENTS.md` blob: `079fad212fc4843eec70999cb551495b226087e6`
- `BACKLOG.md` blob: `a02db7c09b10dba8c827f62618c7c6eb9f096591`
- `Governance/CLOUD-CODEX-GOVERNANCE.md`: `5a8295d61a0d13a624471fa3438c2e6157d9fb60`
- `Governance/CHANGE-TRIGGER-REVIEW-AND-BACKLOG-STANDARD.md`: `07ac0d53520b70d71c8c96fd419d55134e890692`
- `Governance/FINDING-REMEDIATION-AND-REVIEW-MODE-STANDARD.md`: `89622d7c04af0359b3dcc34e5ee5eeb637ced7f0`
- `Governance/HANDOFF-ARTIFACT-AND-CLASSIC-READINESS-STANDARD.md`: `4bdb44bb160b003969d5ccbc157d2415d9e4ed1e`
- ADR-009 capability/profile authority: `82607310523d4c44c3a5530acefc617819b18cf5`

Relevant current backlog ownership:

- `BL-153`: add controlled `system_info`
- `BL-154`: expose scoped disk usage by reusing BL-062
- `BL-155`: filtered environment information, allowlist and secret exclusion
- `BL-156`: field selection and redaction
- `BL-157`: **enforce `system.read` capability — registration and server-side execution checks**

ADR-009's current implementation amendment says `MCP_READ_ONLY=true` exposes exactly `list_directory`, `read_file`, and `get_path_info` until the later capability/profile work owns a new exposure decision.

## Reviewed PR bindings

| PR | BL | Head | Tree | Parent/base relevant to review |
|---:|---|---|---|---|
| #132 | BL-153 | `20e2d97a640879564a428b29726fa9c6646abf69` | `5c3d4fb0ba6e21063f99a001103ec1f17976cf16` | `5b851afb3ef3e7de1b49ce5a352c06d40b815f9d` |
| #213 | BL-155 | `5b0481344dd978d49a13f56c130767b352231c2f` | `8d000e76702700ca28a4626b1c103f886aaae7c7` | PR #132 / `20e2d97a...` |
| #215 | BL-156 | `4313299a3ac2b83e154c79d6d1a1dd600dd67c9c` | `9318ae84490ea46aff5f3bbcdfcb7c3b44e941e3` | PR #132 / `20e2d97a...` |
| #211 | BL-154 | `f4be9937ae78cf1f82191488f9ec0bdd02d27cce` | `0a68233e5f37db241206d665aecb62b429754573` | PR #135 / BL-062 head `1ebd868f...` |

PR #213 and PR #215 are true siblings on PR #132.

## External platform evidence consulted read-only

Microsoft's `GetDiskFreeSpaceExW` contract states that `lpDirectoryName` identifies a directory, that `lpTotalNumberOfBytes` may be quota-limited for the calling user, and that `lpTotalNumberOfFreeBytes` is total free space on the disk.

Microsoft's Windows version-targeting documentation states that `GetVersion`/`GetVersionEx` returns manifest-dependent compatibility values on Windows 8.1+; an application not manifested for newer Windows can receive Windows 8 version 6.2.

No external mutation occurred.

# Findings

## CR-PR132-001 — P1 — NEW CLASSIC FINDING
### BL-153 exposes `system_info` before the BL-157 registration/authorization owner

PR #132 unconditionally registers `system_info` before the existing read-only return:

```go
toolRegistry.Register(tools.NewSystemInfoTool(systeminfo.NewProvider()))

if !capabilities.filesystemWrite {
    return toolRegistry
}
```

The branch therefore changes both default and `MCP_READ_ONLY=true` public catalogs and makes `system_info` callable without a `system.read` execution check.

That crosses two current authority boundaries:

1. `BL-157` explicitly owns `system.read` **registration and server-side execution checks**.
2. ADR-009's current restricted-profile amendment still says read-only exposes exactly the three existing filesystem read tools.

This is not only a documentation mismatch. The runtime tool is exposed before the planned functional capability that is supposed to authorize it.

The process-observation work demonstrates the intended ownership pattern: implementation tasks may prepare tool/domain code while the later capability owner controls runtime registration/exposure.

**Required correction boundary:** BL-153 should leave the provider/tool implementation prepared but unregistered until the BL-157 owner integrates registration and execution authorization, unless a separate current architecture/security decision explicitly reassigns that ownership.

No correction was performed.

## CR-PR132-002 — P1 — CONFIRMED
### Windows OS version can be compatibility-virtualized

`internal/systeminfo/version_windows.go` uses:

```go
version, err := syscall.GetVersion()
```

Current `cmd/versioninfo/main.go` creates the Windows resource configuration but does not bind a supported-OS application manifest.

Microsoft documents that on Windows 8.1 and later the value returned by `GetVersion`/`GetVersionEx` depends on how the executable is manifested; applications not manifested for Windows 8.1/10 can receive the Windows 8 value `6.2`.

`BL-153` explicitly promises controlled OS/version facts. Returning a compatibility version instead of the running OS version breaks the primary tool fact on a supported platform.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required correction:** use a manifest-independent authoritative version source appropriate to the supported Windows runtime, with native Windows evidence. Do not add a dependency without separate approval.

## CR-PR132-003 — P1 — CONFIRMED
### The permanent hard catalog budget gate cannot pass on this head

PR #132's own deterministic tools-list test changes the catalog from:

```text
read-only: 3 -> 4 tools
default:   8 -> 9 tools
```

and expects tools-list response sizes around 2564 / 6087 bytes.

But `benchmarks/budgets.json` at the same head still hard-caps:

```text
read-only tool/schema count = 3
default   tool/schema count = 8
read-only max response      = 2134
default   max response      = 5657
```

The benchmark command explicitly exits with code `3` whenever `BudgetEvaluation.HardFailures > 0`.

Thus the permanent benchmark gate is internally inconsistent with the proposed head.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Correction ordering note:** first resolve `CR-PR132-001`. If BL-153 correctly stops registering the tool, the current catalog budgets should normally remain unchanged and this failure can disappear without widening them. Only the eventual owning registration task should regenerate catalog/wire hard budgets if the public catalog actually changes.

No benchmark baseline was generated during this review.

## CR-PR213-001 — P2 — CONFIRMED
### Environment key allowlisting does not make the value safe

PR #213 restricts keys to:

```text
COLORTERM, LANG, LC_ALL, LC_CTYPE, TERM
```

but `filteredEnvironment` copies every non-empty allowed value verbatim, and `releasedEnvironment` validates only that the key is allowed and the value is non-empty.

A permitted key can therefore carry an arbitrary token-like value, host path, username-like data, control/newline content, or an excessively large value and still be returned by `system_info`. This contradicts BL-155's explicit secret-exclusion requirement and the tool description's claim that the result excludes secrets.

The existing tests prove only that disallowed **keys** such as `PATH` and `API_TOKEN` are filtered; they do not prove that an allowed key has a safe value.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required correction:** define narrow per-field value syntax/length policy or a fail-safe redaction/omission rule, add negative tests for token/path/control/oversized values, and never log rejected raw values. If the accepted syntax itself requires a product/security decision, stop at that decision boundary rather than inventing a permissive regex.

## CR-PR215-001 — P2 — CONFIRMED
### The new structured-result test never validates `system_info`

PR #215 adds:

```go
{systemInfoToolName, map[string]string{"os": "linux"}}
```

to `TestFilesystemStructuredResultsMatchOutputSchemas`, but the loop always resolves the schema with:

```go
filesystemOutputSchema(toolName)
```

`filesystemOutputSchema(systemInfoToolName)` returns `nil`. The local test schema validator treats a schema without a `type` as success, so the new case passes without checking any system-info property, type, or `additionalProperties` behavior.

**Disposition of prior Codex signal:** `CONFIRMED`.

**Required correction:** dispatch the case to `systemInfoOutputSchema()` (or one unified output schema dispatcher) and add a negative case that demonstrably fails on a malformed selected result.

# Integration constraint — PR #215 / BL-156

PR #213 / BL-155 and PR #215 / BL-156 are siblings on BL-153. This is not counted as a separate severity finding against the isolated Mobile-preparation delta, but it is a mandatory convergence boundary before BL-156 can be treated as complete under the normal backlog.

BL-155 adds the public `environment` field. BL-156's field allowlist currently contains only:

```text
os, architecture, version
```

After BL-155 is integrated, BL-156 must be rebased/reconciled so that the final field-selection and redaction contract deliberately handles `environment`; it must not accidentally drop it or leave it outside the field-selection owner.

## CR-PR211-001 — P1 — NEW CLASSIC FINDING
### BL-154 exposes `get_disk_usage` before the BL-157 registration/authorization owner

PR #211 unconditionally registers:

```go
toolRegistry.Register(tools.NewGetDiskUsageTool(filesystem))
```

before the read-only return and changes the documented read-only catalog to four tools.

This conflicts with:

- ADR-009's current exact three-tool read-only profile; and
- `BL-157`, which explicitly owns `system.read` **registration and server-side execution checks**.

The branch contains no `system.read` authorization boundary, so the scoped system-information surface becomes callable before its owning capability task.

**Disposition of prior Codex ADR/profile signal:** `CONFIRMED_AND_SUBSUMED`.

This same premature registration also makes the permanent hard catalog budgets stale: PR #211's tools-list test expects 4/9 tools and approximately 2813/6336 bytes while the hard budget still allows 3/8 and 2134/5657. That is treated here as a direct consequence of the same out-of-owner catalog expansion rather than a separate finding.

**Required correction boundary:** keep the BL-154 adapter/tool implementation unregistered until the BL-157 owner integrates `system.read` registration and execution authorization, unless current governance explicitly reassigns ownership.

## CR-PR211-002 — P1 — CONFIRMED
### Valid file inputs fail on Windows because the native API requires a directory

The BL-062 parent contract explicitly accepts an existing relative path, not directories only. `LocalFileSystem.DiskUsage` resolves that path and passes the resolved string unchanged to `diskUsageForPath`.

The Windows implementation then passes the same string to `GetDiskFreeSpaceExW`.

Microsoft documents the API's first parameter as **a directory on the disk**. PR #211's own tool/wire behavior uses file-shaped inputs such as `data/file.txt` / `read file.txt`.

Therefore a valid cross-platform contract input can succeed on Unix and fail on Windows.

**Disposition of prior PR #211 Codex signal:** `CONFIRMED`.  
**Related parent PR #135 Codex signal:** independently confirmed as the same root defect.

**Owner/correction order:** the root defect is in BL-062 / PR #135 and must be corrected there first, then PR #211 rebased/revalidated. Under the current "existing path" contract, a file should be mapped to its validated containing directory for the capacity query; alternatively narrowing the public contract to directories would require coordinated cross-platform/schema/documentation changes and must not be inferred silently.

Native Windows validation remains mandatory.

## CR-PR211-003 — P2 — CONFIRMED INHERITED PARENT FINDING
### Windows mixes quota-scoped total bytes with volume-wide free bytes

The Windows BL-062 adapter requests all three values from `GetDiskFreeSpaceExW` and computes:

```go
UsedBytes: total - free
```

Microsoft's API contract distinguishes the scopes:

- `lpTotalNumberOfBytes` is the total available to the calling user and can be reduced by a per-user quota.
- `lpFreeBytesAvailableToCaller` is also caller/quota scoped.
- `lpTotalNumberOfFreeBytes` is total free space on the disk.

With per-user quotas, `free` can exceed the caller-scoped `total`. Unsigned subtraction can then underflow, after which the caller rejects the result as a limit violation.

Even when no underflow occurs, the reported "used" value mixes two different accounting scopes.

**Disposition of the prior PR #135 Codex signal:** `CONFIRMED` by Classic using the native adapter and Microsoft API contract.

**Owner/correction order:** correct BL-062 / PR #135 first using values with one coherent accounting scope (or a separately reviewed volume-wide total source), then rebase/revalidate PR #211.

# Review summary

```text
ReviewedPRCount=4
ReviewedInheritedParentCount=1
ConfirmedPriorCodexSignals=7
RejectedPriorCodexSignals=0
NewClassicFindings=2
P1FindingCount=5
P2FindingCount=3
OpenFindingCount=8
IntegrationConstraintCount=1
FindingFixesPerformed=false
RepositoryMutationCount=0
ExternalMutationCount=0
ReviewerIndependencePreserved=true
```

Per reviewed PR:

```text
PR132 = FAIL_REMEDIATION_REQUIRED   (3x P1)
PR213 = FAIL_REMEDIATION_REQUIRED   (1x P2, inherits PR132 blockers)
PR215 = FAIL_REMEDIATION_REQUIRED   (1x P2, inherits PR132 blockers, sibling convergence required)
PR211 = FAIL_REMEDIATION_REQUIRED   (2x P1, 1x P2; native root defects owned by PR135)
```

## Correction / continuation boundary

This review authorizes **no correction**.

Parent-first correction order is mandatory:

1. PR #132 / BL-153: resolve registration ownership, Windows version source, and permanent hard-gate consistency.
2. Rebase/revalidate #213 and #215 only after the corrected BL-153 parent is bound.
3. Converge #213 and #215 deliberately; the final BL-156 contract must account for the BL-155 environment field.
4. PR #135 / BL-062: correct Windows path handling and quota-consistent accounting first.
5. Rebase/revalidate PR #211 / BL-154 after its corrected parent.
6. The eventual public registration/execution authorization of `system_info` and `get_disk_usage` remains with BL-157 / `system.read` unless current governance explicitly changes that ownership.

After any authorized bundled correction, use the required **Classic read-only focused independent delta review**. Do not use Codex review as independent-review evidence.

## Post-vacation priority

This batch does not establish the post-vacation work queue. When vacation ends:

```text
MobileTaskSelection=OFF
PrimaryWorkSource=BACKLOG.md
```

Rebind the then-current normal backlog/governance and use this review evidence when its owning normal-backlog work is reached.
