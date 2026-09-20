# FlashGate MCP — Classic Independent Review Batches 13–14

**Status:** PASS_READ_ONLY_REVIEW_CONSOLIDATED
**Date:** 2026-09-18
**Bound main:** `f615d6f54c967b1fae04c5378f32af0b648b40f7`
**Bound tree:** `477fb951fa9ac5b27e051a8d320ca440e473eb9d`
**BACKLOG.md:** `a02db7c09b10dba8c827f62618c7c6eb9f096591`
**ReviewMode:** `INDEPENDENT_REVIEW`
**FindingFixesPerformed:** `false`

Review evidence only; no backlog status, correction or merge authority.

## Vacation exit

After vacation: `MobileTaskSelection=OFF`, normal current `BACKLOG.md` and governance resume.

## Batch 13 — SPR-053

Reused prior Classic findings: BL-100/#105; BL-107/#185; BL-108/#187; BL-109/#189.

- **BL-101/#152:** production creates a one-root registry then unwraps it to the legacy filesystem,
  so multi-root is not actually retained/configurable; typed-nil filesystem values can pass
  interface-nil validation.
- **BL-102/#180:** explicit empty `rootId` defaults instead of failing; Changelog claims completion
  before finalization.
- **BL-103/#244:** safe-read default breaks permanent default smoke/native metadata path; CI masks it
  with `MCP_PROFILE=filesystem-write`. Structurally the branch maps profiles to the old Boolean
  capability model rather than canonical BL-100 functional capabilities.
- **BL-104/#182:** copy permits write-only root although source content is read.
- **BL-105/#183:** MaxResultBytes limits raw bytes, not serialized duplicated CallToolResult.
- **BL-106/#184:** file-type policy can be bypassed via copy/move source/target extension changes.
- **BL-110/#190:** narrow catalog mechanics are reusable, but ancestry is invalid for canonical
  integration: root-local capability types, no BL-100 canonical model, no BL-103 profile/risk config.
- **BL-111/#194:** negative tests bypass production `capabilitiesFromRoots`.
- **BL-159/#181:** execution-time authorizer is a reusable narrow delta, but must rebase on corrected
  BL-100.
- **BL-160:** no dedicated open PR.
- **BL-161/#221:** move-denial test grants the exact Write permission consumed by MovePathTool;
  also inherits BL-111 ancestry.
- **BL-171/#178:** parent BL-202 needs correction; own test does not assert generic
  `"invalid params"` message.

Required convergence:

```text
corrected BL-100 + corrected BL-103 + corrected BL-101..109
    -> rebuild BL-110 -> BL-111 -> BL-161
corrected BL-100 -> rebase BL-159 -> implement BL-160
corrected BL-202 -> rebase BL-171
```

## Batch 14 — SPR-054

No clean candidate.

- **BL-113/#106:** already-recorded exited child must return terminal outcome, not stale/not-found.
- **BL-129/#134:** reuse Batch-04 ownership-finalization, expiry and session-binding findings.
- **BL-162/#129:** reuse Batch-01 premature risk taxonomy and missing session binding.
- **BL-165/#138:** repeated authorization applies to externally initiated access/mutations, not
  server-owned timeout/disconnect/shutdown transitions under immutable bound context.

No findings were fixed.
