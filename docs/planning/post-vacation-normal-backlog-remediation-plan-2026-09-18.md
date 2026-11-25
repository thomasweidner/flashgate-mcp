# FlashGate MCP — Post-Vacation Normal Backlog Remediation / Integration Plan

Status: READ_ONLY_PLANNING_COMPLETE
Date: 2026-09-18
Bound main: f615d6f54c967b1fae04c5378f32af0b648b40f7
Bound tree: 477fb951fa9ac5b27e051a8d320ca440e473eb9d
BACKLOG.md: a02db7c09b10dba8c827f62618c7c6eb9f096591
Review basis: Classic Independent Review Batches 01–27
Ledger preflight: 202 open PRs, complete pagination, two-pass stable

This is noncanonical planning evidence. It grants no correction, integration, merge, branch deletion or release authority.

## Vacation exit

After vacation:
- MobileTaskSelection = OFF
- PrimaryWorkSource = BACKLOG.md

Rebind current repository, working tree, governance, BACKLOG, PR heads, toolchain and platform before selecting work. Required PowerShell baseline is 7.6.5.

## Normal sprint order

SPR-048 -> SPR-049 -> SPR-050 -> SPR-051 -> SPR-052 -> SPR-053 -> SPR-054 -> SPR-055 -> SPR-056 -> SPR-057 -> SPR-058 -> SPR-059 -> SPR-060 -> SPR-061.

Within one sprint, dependency order may override numeric BL order. A clean child never bypasses an uncorrected parent.

# SPR-048 — first return sprint

1. BL-202 / PR #158: correct overwrite idempotence hints, false-valued annotation member presence checks, and contradictory docs.
2. BL-203: rebuild the narrow semantics from #167 on a clean base. Keep malformed/unavailable calls as JSON-RPC errors; MCP isError only after tool execution begins.
3. BL-204 / #65: clean candidate; fresh finalization/rebase.
4. BL-205 / #55: measure complete framed JSONL bytes with tight deterministic bounds.
5. BL-206 / #161: clean candidate.
6. BL-207 / #60: clean candidate.
7. BL-208 / #69: correct caller/root/backend/service/auth-generation cache binding.
8. BL-209 / #149: repair the decision packet, then make the Tasks decision.
9. BL-210 only if BL-209 selects final Tasks support.
10. BL-211 / #150 decision only after BL-209.
11. BL-212 / #71: production-registry schema inventory and regenerated budgets.
12. BL-213 / #58: remove premature lifecycle ownership and align useful-byte contract.
13. BL-214 / #63: replace literal serialization-copy count with real pipeline evidence.
14. BL-215 / #95: budget definition only; leave enforcement to BL-256.
15. BL-216 / #57: batch guidance only when supported.
16. BL-218 / #61: streaming construction, safe media type and non-oracular binding checks.
17. BL-219: rebuild useful #177 fingerprint semantics after corrected BL-208.
18. BL-220 / #62: pin forbidden interpreters and concrete executable admissions.

# SPR-049 — Operations/Job

Dependency order:
BL-084 -> BL-085 -> BL-086 -> BL-087 -> BL-088 -> BL-089 -> BL-090 -> BL-091 -> BL-092 -> BL-093 -> BL-094 -> BL-095 -> BL-096 -> BL-097 -> BL-098 -> BL-099 -> BL-164.

Use clean #76, #70, #77, #82 and #84 after their parents. Correct admission, deadline, progress, cleanup, shutdown and ABA findings first. Rebuild BL-098/099/164 only after production controls are integrated.

# SPR-050 — Filesystem read/list

Correct BL-038 cursor semantics before BL-037 paging. Then BL-039/040, clean BL-041/#83, BL-042/043, clean BL-044/#103, BL-045, BL-046 on BL-044, BL-047/048, and BL-049 only after corrected BL-038/037.

# SPR-051 — targeted/long filesystem work

Dependency groups:
- BL-050 -> BL-051
- BL-052, BL-053, BL-054, BL-055
- BL-065 threat model -> BL-056 -> BL-057
- BL-058 + BL-059 -> BL-060
- BL-061, BL-063
- corrected SPR-049 Operations + BL-060/061 -> BL-064
- then BL-066 integration tests -> BL-067 corpus

BL-060/061 require substantive I/O/traversal hardening.

# SPR-052 — Search

Treat Search as one parent-first stack:
BL-068/#104 -> BL-069/#154 -> BL-070/#155 -> BL-071/#157 -> BL-072/#160 -> BL-073/#162 -> BL-074/#164 -> BL-075/#166 -> BL-076/#170 -> BL-077/#171 -> BL-078/#172 -> BL-079/#173 -> BL-080/#174 -> BL-082/#176.

Correct and independently review each parent before rebasing the next child.

# SPR-053 — capabilities/profiles/named roots

Required convergence:
- correct BL-100;
- correct BL-103;
- correct named-root line BL-101 through BL-107;
- combine corrected BL-100 with roots to rebuild BL-108 -> BL-109;
- then rebuild BL-110 -> BL-111 -> BL-161;
- corrected BL-100 -> rebase BL-159 -> implement BL-160;
- corrected BL-202 -> rebase BL-171.

Do not preserve the duplicate root-local capability model.

# SPR-054 / SPR-055 — Process

SPR-054 policy-first: BL-113, BL-162, BL-165, then BL-129 lifecycle/ownership.

SPR-055: correct BL-114, then BL-116 -> BL-117, converge/rebase BL-115, then BL-118 registration plus process.observe execution authorization.

# SPR-056 — Managed Process

Core: BL-119 -> BL-120 -> BL-121 -> BL-122 -> BL-123 -> BL-124 -> BL-125 -> BL-126.

Then BL-130, BL-131, and BL-132 -> BL-133 -> BL-134 -> BL-135.

Release process slots/budgets only after confirmed OS exit. Resolve portable CPU semantics before BL-134. Finish with BL-252 after integrated stateful packages, BL-253 after Process integration and PowerShell 7.6.5, and BL-254 after corrected BL-098 plus reusable BL-252 gate.

# SPR-057 — Typed Command Execution

Correct BL-136 -> BL-137 -> BL-138, then combine with corrected WorkingDirectoryRoot for BL-139. Continue BL-140 -> BL-141 -> BL-142, combine with corrected Managed Process for BL-143, then BL-144, corrected BL-145, BL-146/147, BL-148 integration, BL-149, BL-151, BL-152, BL-163, BL-167, BL-168, and final BL-170 residual-risk docs.

# SPR-058 — System Information

Converge BL-062 -> BL-154 with BL-153 -> BL-155 -> BL-156, then implement BL-157 system.read registration/execution authorization using corrected SPR-053 capabilities.

# SPR-059 — Multi-mode architecture/execution identity

Recommended:
1. BL-221 threat model
2. BL-222 single-binary gate
3. BL-223 CLI/lifetime
4. BL-224 process-root coordinator
5. BL-225 IPC/session contract
6. BL-233 config/discovery/logs
7. BL-235 owner matrix/status wording
8. BL-236 backend-neutral identity
9. BL-166 audit contract
10. BL-237 Variant A
11. BL-238 Variant B contract/decision
12. BL-239 state/cache/resource binding
13. BL-234 service authorization/identity/policy integration

# SPR-060 — native runtime/service

After prior contracts are clean:
1. BL-226 Windows Named Pipe
2. BL-227 Linux UDS
3. BL-341 host ownership/deterministic shutdown
4. BL-228 proxy
5. BL-229 auto/discovery fallback
6. BL-230 Windows service
7. BL-231 Linux systemd service

BL-341 consumes corrected BL-223/224/225 plus BL-094 and BL-129.

# SPR-061 — final validation/release

First make evidence trustworthy:
- BL-172/173 and governance decisions BL-177–179;
- benchmark/evidence hardening BL-317–332 as applicable;
- continuous/current-state documentation BL-305–315.

Then integrated validation/release:
- BL-241/242/243/244/245;
- reuse Done BL-246–248/251;
- BL-249/250;
- BL-255–260;
- BL-261 after benchmark-server decision/pinning;
- BL-262 exact-input supply-chain evidence;
- BL-263 final Version-1 release boundary.

# First actionable return queue

Assuming a fresh rebind does not materially change topology:

1. BL-202 / PR #158 correction
2. BL-203 clean rebuild from #167 semantics
3. BL-204 / PR #65 finalization
4. BL-205 / PR #55 correction
5. BL-206 / PR #161 finalization
6. BL-207 / PR #60 finalization
7. BL-208 / PR #69 correction
8. BL-209 / PR #149 packet correction plus decision
9. BL-210/211 according to BL-209
10. continue remaining SPR-048 in the order above

For every owner: fresh binding, scoped correction, focused portable/native gates, correction-actor self-validation, separate read-only Classic delta review where required, and explicit Git/remote authority before commit/push/merge. Canonical backlog status changes only after integrated completion/readback.
