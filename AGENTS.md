# FlashGate MCP — Agent Router

This file is the repository-visible router for Codex and other repository agents.

## Authority and required reading

Before implementation, review, or Mobile task selection, read the current repository state and then:

1. `Governance/CLOUD-CODEX-GOVERNANCE.md`
2. `Governance/CHANGE-TRIGGER-REVIEW-AND-BACKLOG-STANDARD.md`
3. `BACKLOG.md`
4. `MOBILE.md` for Codex Cloud / Mobile work
5. directly affected ADRs, security documents, testing guidance, and technical documentation

Do not infer unavailable local files from memory.

The tracked Cloud governance is a bounded projection for repository-contained Cloud work. It does not replace the central local Slim Governance authority under `Codex-Work/Governance` or higher-level local `AGENTS.md` files. During Windows finalization, bind and apply the then-current local authorities in addition to the repository rules.

## Assignment start

For every material assignment:

- read the exact current BL row and status from `BACKLOG.md`;
- perform the change-trigger review;
- identify affected public contracts, security boundaries, platforms, dependencies, release impact, documentation, and permanent gates;
- identify real prerequisites and mandatory decision boundaries before implementation;
- preserve completed (`Done`) work as completed;
- do not broaden a BL task beyond one coherent reviewable acceptance boundary.

## Continuation and remediation

Continue independently through directly caused, in-scope corrections while the approved goal, authorization boundary, and remediation budget remain valid.

- established validated artifacts: at most 6 material correction/revalidation cycles;
- new or materially rebuilt artifacts: at most 12 cycles;
- validate parser/focused root cause first, then the smallest sufficient focused checks, then one consolidated final validation;
- stop earlier on success, no progress, exhausted budget, ambiguous mutation state, or a mandatory decision/authorization boundary.

Do not claim independent approval of work you implemented or corrected.

## Mandatory decision stops

Do not convert an unresolved decision into an implementation assumption. Stop when proceeding requires a new:

- product decision;
- architecture decision;
- security decision;
- platform decision;
- dependency decision;
- release decision;
- scope decision.

## External and security boundary

External, Remote/GitHub/provider, credential, separately approval-bound Git, installation, permission/ACL/owner, system/service, scheduled-task/automation, irreversible, and non-exactly-reversible production/security actions are Hard-One-Shot boundaries.

Before a write-capable external action, complete every deterministic read-only/prewrite check that can be known in advance. The first write-capable attempt consumes that action authorization even if it fails. Do not automatically retry, switch provider, or fall back to a manual credential/remote path.

Force operations are prohibited unless a later explicit contract says otherwise.

## Codex Cloud / Mobile

For Mobile work, `MOBILE.md` defines task eligibility, dependency stacking, and the managed-open-PR handoff.

- no manual `git remote` configuration;
- no PAT/token/SSH/credential workaround;
- exactly one managed open-PR publication attempt when the task prompt authorizes it;
- no PR merge or close;
- no branch deletion;
- no tag or release;
- no repository-settings or secrets changes;
- do not mark the BL task `Done`;
- do not fabricate Windows, WSL, SCM, systemd, native-host, hardware, identity, permission, or credential evidence.

Windows finalization remains required for every Mobile PR.

## Toolchain

PowerShell target standard for FlashGate is PowerShell 7.6.5 via `pwsh`.

If required tooling is unavailable in Codex Cloud, do not silently install or substitute a different authoritative environment. Run the checks that are genuinely available and record the remainder for Windows/native finalization.
