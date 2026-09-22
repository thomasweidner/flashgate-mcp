# FlashGate MCP — Agent Guide

This repository contains the public FlashGate product, its tests, and its
portable contributor documentation.

## Required reading

Before a material change, inspect the current repository state and read:

1. `BACKLOG.md` for the owning task and status;
2. `CONTRIBUTING.md` for contributor and validation requirements;
3. `docs/architecture.md`, `docs/security.md`, and `docs/testing.md`;
4. directly affected ADRs and technical documentation.

Do not infer unavailable files or project state from memory.

## Working rules

- Keep changes inside one coherent, reviewable acceptance boundary.
- Preserve completed backlog work as history; do not reopen it implicitly.
- Stop for a new product, architecture, security, platform, dependency,
  release, or scope decision instead of converting it into an assumption.
- Validate directly affected areas first, then run the repository-wide gates
  required by `docs/testing.md`.
- Preserve FlashGate's local-first, least-privilege, fail-closed design.
- Do not add secrets, private host paths, credential material, or machine-local
  workflow dependencies to the repository.
- Treat external writes, credentials, permissions, services, releases, and
  destructive operations as explicit authorization boundaries.
- Never force-push unless an explicitly approved recovery procedure requires it.

The public repository must remain sufficient to build, test, release,
contribute to, and understand FlashGate without access to a private development
control plane.
