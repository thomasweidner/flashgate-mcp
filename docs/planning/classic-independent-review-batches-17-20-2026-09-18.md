# FlashGate MCP — Classic Independent Review Batches 17–20

**Status:** PASS_READ_ONLY_REVIEW_CONSOLIDATED  
**Date:** 2026-09-18  
**Bound main:** `f615d6f54c967b1fae04c5378f32af0b648b40f7`  
**Bound tree:** `477fb951fa9ac5b27e051a8d320ca440e473eb9d`  
**BACKLOG.md:** `a02db7c09b10dba8c827f62618c7c6eb9f096591`  
**ReviewMode:** `INDEPENDENT_REVIEW`  
**FindingFixesPerformed:** `false`

Review evidence only. No backlog status, correction, integration, merge, branch-deletion or release authority.

## Vacation exit

After vacation: `MobileTaskSelection=OFF`; normal current `BACKLOG.md` and governance resume.

# Batch 17 — SPR-057 Typed Command Execution

Reused Batch-03: BL-136/#107 executable provenance; BL-137/#186 `--`/flag ordering;
BL-138/#198 interpreter escape, inline selector bypass and negative positional integer ambiguity.

- **BL-139:** no PR; later command + named-root integration join.
- **BL-140/#203:** timeout default/max policy delta reusable after corrected BL-138.
- **BL-141/#212:** separate bounded stdout/stderr primitive reusable after parents.
- **BL-142/#236:** failed signal termination must allow no exit code; diagnostics must be validated
  against actual stream truncation and terminal status.
- **BL-143:** no PR; synchronous Managed Process wrapper integration join.
- **BL-144/#246:** Windows os/exec can reintroduce SYSTEMROOT; hard deny misses multiple
  interpreter/plugin injection environment families.
- **BL-145:** prefer #249 over #248, but both share two security defects: common credential names
  such as PGPASSWORD/MYSQL_PWD can evade sensitive-name detection, and literal secret replacement
  can destroy sensitive field labels before pattern redaction.
- **BL-146/147:** no PR.
- **BL-148/#252:** clean concurrency-limiter primitive, but not wired to actual command/process
  admission; BL-148 not complete.
- **BL-149:** prefer #254; clarify that `user-worker` fails closed when enabled/used while a
  forward-compatible `enabled=false` declaration remains valid. #253 superseded.
- **BL-151/#245:** single-engine gate misses direct APIs such as `syscall.Exec` /
  `syscall.CreateProcess`.
- **BL-152, BL-163, BL-167, BL-168:** no dedicated PRs.
- **BL-170/#114:** clean residual-risk/sandbox-boundary documentation candidate.

# Batch 18 — SPR-058 System Information

- **BL-062/#135:** Windows file input must be converted to validated containing directory before
  `GetDiskFreeSpaceExW`; quota-scoped total must not be mixed with volume-wide free bytes.
- **BL-153/#132:** reuse Batch-02 manifest-sensitive Windows version + stale hard budgets.
- **BL-154/#211:** reuse Batch-02 Windows file-input issue + read-only-profile contract conflict.
- **BL-155/#213:** allowlisted environment names still need narrow value validation/limits.
- **BL-156/#215:** representative result test validates through wrong/nil schema path.
- **BL-157:** no PR; later `system.read` registration/execution-authorization join.

# Batch 19 — SPR-059 Multi-Mode Architecture / Execution Identity

- **BL-221/#136:** require native rejection of remotely-originating Named Pipe clients independent
  of ACL authorization.
- **BL-222/#144:** single-binary permanent gate blacklists only guessed mode names; enumerate
  executable cmd targets or actual build/archive outputs.
- **BL-223/#195:** reuse Batch-04 `-h` contract finding.
- **BL-224/#214:** reuse Batch-04 cancellation/read-loop/cleanup ordering finding.
- **BL-225/#210:** reuse Batch-04 missing closed wire schemas, cancellation-result semantics and
  post-handshake progress/read deadline.
- **BL-233/#205:** reuse Batch-05 MCP_*/FLASHGATE_* compatibility and per-user log lifecycle gaps.
- **BL-234:** no PR; service authorization/identity/policy integration join.
- **BL-235/#201:** use “accepted architecture/decision”, not “task complete”, while canonical
  backlog row remains Planned.
- **BL-236/#220:** reuse Batch-05 closed backend set + immutable resource-budget binding.
- **BL-237/#257:** reuse Batch-05 service-account passthrough bypass + audit profile/correlation.
- **BL-238/#208:** normative worker contract leaves Windows session/token and Linux login-session
  acquisition decisions unresolved; complete decision or mark blocked.
- **BL-239/#222:** exact expiry missing from binding; Delete has validate/unlock/delete ABA;
  expired abandoned entries lack bounded independent reclamation.
- **BL-166/#127:** define audit reader/admin least privilege; include effective backend identity;
  disambiguate parent references across generations; mandatory events cannot silently disappear on
  redaction failure; exact-one-terminal rule needs unclean-shutdown exception/recovery-gap semantics.

# Batch 20 — SPR-060 Native Runtime / Service

- **BL-226/#256:** reuse Batch-05 Windows transport security findings; parent BL-225 blocked.
- **BL-227/#255:** reuse Batch-05 Unix-socket stale replacement/publication/bounded-probe findings;
  parent BL-225 blocked.
- **BL-228:** no PR; proxy integration join.
- **BL-229:** no PR; requires corrected BL-233 **and** BL-341.
- **BL-230:** no PR; Windows service integration/native evidence.
- **BL-231:** no PR; Linux systemd service integration/native evidence.
- **BL-341:** no PR; requires corrected BL-223, BL-224, BL-225, BL-094 and BL-129 before
  reclassification.

# Readiness through SPR-060

Clean prepared candidates or narrow reusable primitives found since Batch 17:

```text
SPR-057:
  BL-170 #114 clean documentation candidate
  BL-140 #203 narrow parent-blocked delta
  BL-141 #212 narrow parent-blocked delta
  BL-148 #252 clean limiter primitive, integration still required

SPR-058:
  none complete

SPR-059:
  none complete

SPR-060:
  none complete
```

No finding was corrected in these reviews.

## Continuation

Next normal-backlog review target: **SPR-061**.
