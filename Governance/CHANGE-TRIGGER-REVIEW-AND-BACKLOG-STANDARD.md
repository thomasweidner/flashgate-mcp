# FlashGate Slim Change-Trigger and Backlog Adapter

**Status:** Binding
**Owner:** BL-343
**Global authority:** `Codex-Work/Governance/CHANGE-TRIGGER-REVIEW-AND-BACKLOG-STANDARD.md`
**Adapter model:** `SLIM_PROJECT_ADAPTER`

## Purpose

This file contains only FlashGate-specific trigger, owner, validation, and
backlog rules. It inherits the central Slim Governance security,
authorization, continuation, review, and registration-truth boundaries without
copying their workflow state machines or handoff contracts.

## Project checkpoints

Apply this adapter at assignment start, after a material scope change, before a
local Git action, at sprint close, and for release-candidate or stable-release
work. Each checkpoint reads the current repository, relevant project
authorities, and `BACKLOG.md` before deciding.

A new backlog item is required only for genuinely new, independently owned
product, architecture, security, platform, release, dependency, or durable
project work. Same-owner correction, report or handoff repair, focused
revalidation, and lifecycle progress do not create another item.

`NEW_WORK_REGISTERED` may be claimed only after the canonical `BACKLOG.md` write
has succeeded and the identifier, title, scope, status, and owner have been read
back. A planned candidate or narrative statement is not registration evidence.
Completed owners remain `Done` and are not reopened merely because their active
enforcement artifacts are later simplified or superseded.

## FlashGate validation adapter

Validation follows `DIRECTLY_AFFECTED_FIRST`. Reuse valid unchanged evidence
and run only affected project gates:

- Go formatting, vet, unit/integration tests, Windows/Linux coverage, lint and
  build;
- product, MCP, protocol, filesystem and security contracts;
- platform-specific Windows/Linux behavior;
- release, metadata, shell and PowerShell 7.6.5 checks when their sources are
  affected;
- focused documentation consistency for active project statements.

Product or integration work is blocked only by a real correctness, security,
data-integrity, credential, external/remote mutation, destructive-action,
technical-reproducibility, scope, architecture, or authorization boundary.
Meta, report, history, presentation, handoff, lifecycle, unchanged-evidence, or
time-only deviations are not product blockers unless the artifact itself is
the current technical authority.

## Git and external boundary

Normal Local Git uses a read-only repository/branch/scope check, relevant
project gates, explicit action-scoped user approval, exact paths, staged
readback and a separate commit approval. It does not require a V3/V4 state
binding, Target-State Envelope, Derived Grant, Stage Receipt, Local Prep
Handoff, or Heavy Authorization Package.

Push, PR or merge writes, remote cleanup, credentials, and other external
actions remain separately authorized and retain their applicable fail-closed
security contracts. Force remains prohibited.

## Legacy compatibility

The repository may continue to contain historical Generic Handoff, Finding
Correction, Commit Preparation, publication, orchestration, and V3/V4 fixtures.
They are `LEGACY_COMPATIBILITY_ONLY`, receive no new project consumers, and are
not normal Product-CI or development blockers. Their physical retirement is a
later consumer-cutover decision.

BL-337 is terminally superseded by this adapter. BL-330 remains `Planned` for
the small FlashGate-specific status-legend and validator-parity decision; that
project detail is not silently claimed as resolved by central governance.
