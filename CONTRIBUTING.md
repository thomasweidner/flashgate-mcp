# Contributing to FlashGate MCP

## Governance checkpoints

Before implementing, after a material scope change, before commit, at sprint
close, and for release candidate/stable release work, create or update an
assignment governance record according to
[the change-trigger standard](Governance/CHANGE-TRIGGER-REVIEW-AND-BACKLOG-STANDARD.md).
CI and release records are generated ephemerally by
`scripts/New-GovernanceWorkflowRecord.ps1`. Run
`scripts/Test-GovernanceConsistency.ps1` and
`scripts/Test-GovernanceConsistencyFixtures.ps1` when catalog, schema,
governance source, validator, or workflow behavior changes. Independent full
and focused delta reviews are read-only; correction happens only in
`BUNDLED_CORRECTION`.

Same-assignment remediation is bounded to 12 correction/revalidation cycles
for new or materially rebuilt artifacts and 6 for established validated
artifacts. Automatic retries after the first productive write-capable
operation are forbidden. Directly caused isolated harness, fixture, parser,
instrumentation, diagnostic, and classification defects stay in the same
authorized correction assignment unless a decision, authorization, scope, or
budget boundary is reached. End or redact external monitors before final
activity gates; no time-varying check may occur between the final gate and the
first productive write.

Classic receives exactly one handoff file. One required file may be transferred
directly; multiple required files must be rebuilt and validated as one ZIP,
never as separately uploaded package members. See
[the handoff standard](Governance/HANDOFF-ARTIFACT-AND-CLASSIC-READINESS-STANDARD.md).
The reusable `IMPLEMENTATION_TO_INDEPENDENT_FULL_REVIEW` profile carries a
complete implementation patch, scope and hash-bound passing full-completion
evidence into the first independent full review. It requires no prior review
artifact. `GENERIC_COMMIT_PREPARATION` remains a later transition and still
requires independent-review evidence.

`FINDING_CORRECTION` is the separate correction-to-focused-review profile. It
requires the explicit `BUNDLED_CORRECTION_TO_FOCUSED_DELTA_REVIEW` transition,
exact finding parity, a commit or immutable-package previous-review state, and
directory-first validation. Use `-PreflightOnly` with a fresh staging directory
to prove `ReadyToExecute` while keeping `PackageWriteAttemptCount=0`; this state
does not make the payload Classic-review-ready. Use
`-FinalPackageContentOnly` to materialize and validate a second fresh directory
whose bytes already carry `FINAL_REVIEW_PACKAGE`, `ClassicReviewReady=true`,
and contractual attempt count `1`. That mode still performs no real package
write; a later separately authorized call serializes one candidate with
`CreateNew`, binds and product-validates its identity, rejects later drift, and
atomically publishes the same object by no-overwrite hard link. It never repairs
the ZIP in place. `report.md` must retain the
complete correction evidence and exactly one embedded contract conforming to
`Governance/finding-correction-report-contract.schema.json`. The sole producer
may change only its lifecycle/status/readiness/Classic/attempt/next-action
fields for final content; finding, disposition, previous-review, patch, scope,
and evidence bindings remain byte-value equivalent and are reparsed by the
productive validator.
If a finding declares a separate publication regression matrix, include both
optional artifacts: the runner-written `publication-regression-result.json`
and Evidence V2. Evidence binds the result SHA-256 and canonical matrix-catalog
SHA-256; the catalog owns the exact case/source/dependency sets. Bind the
evidence SHA-256, matrix ID, finding ID, and result-derived case IDs in the
ledger, finding regression matrix, focused record, and embedded report
contract. Do not substitute narrative, global counts, or self-authored result
claims for the persisted runner result.

Before architecture, backlog, or implementation decisions, bind the current
repository identity, baseline/current commits, branch, complete relevant status,
scope, IDs, and parallel-worktree state. New or materially rebuilt validators
require a failure-mode matrix before implementation and cheap parser, parameter,
Temp, sandbox, and harness gates before expensive matrices.

The complete status binding is the SHA-256 of the unmodified bytes from exactly
`git status --porcelain=v2 --untracked-files=all`. Scope and file-hash paths are
equal, duplicate-free, and Windows-case-safe sets. Protected foreign worktrees
require separate explicit status bindings. A final ZIP attempt is counted
immediately before its first write-capable candidate open, even if that open
fails, and is never retried automatically.

Validation follows one funnel: root-cause checks, directly affected components,
documentation convergence, and exactly one complete final run. Long runs report
`completed/selected` with a named unit and phase. They separately report
`PASS`, `FAIL`, `SKIPPED`, `BLOCKED`, `CANCELLED`, `PENDING`, and `NOT_RUN`;
`X/Y` never means pass/fail. Observed, resolved, and open warnings plus material
correction cycles, validation executions, and infrastructure/invocation failures
are separate counters. Recurring long runs without numeric progress are
instrumentation findings.

Governance work uses `scripts/Invoke-GovernanceValidation.ps1` with a task-data
request conforming to
`Governance/governance-validation-request.schema.json`. The permanent profiles
are `documentation-registration`, `governance-schema-change`,
`fixture-harness-change`, `finding-correction`, `commit-preparation`,
`focused-revalidation`, `evidence-only-focused-review`, `post-merge-closure`,
and `full-completion`. Do not generate a task-specific controller when the
request and an existing permanent helper can express the work; declared
controller inventory and its file/line counters must agree and unknown
exceptions fail closed. Every profile runs parser/syntax, repository text
policy, `git diff --check`, VERSIONED/IGNORED/GIT_EXCLUDED/EXTERNAL input
binding, toolchain/context binding, and actual canonical
source/worktree/selector resolution in that order before consuming an expensive
subordinate result.

## Scope

FlashGate is a native, local-first MCP server optimized for low latency, low token use, low RAM/CPU consumption, and strict server-side security. Contributions must preserve those objectives and must not silently introduce an interpreter, remote listener, broad shell, unbounded operation, or security bypass.

`BACKLOG.md` is the authoritative implementation plan. `docs/version-1-scope-and-release-boundary.md` defines which accepted work is required before Version 1.0 and which work is deliberately deferred.

## Before changing code or contracts

1. Identify the canonical backlog task and sprint.
2. Read the relevant ADRs, architecture, security, tool conventions, and testing plan.
3. For a new architecture or security decision, add or amend an ADR before implementation.
4. Keep current implementation facts separate from target Version 1.0 behavior.
5. Do not promote a `Later` item into Version 1.0 without an explicit backlog and documentation decision.

## Design requirements

- Prefer Go standard library and existing project abstractions.
- Add no external dependency without review of necessity, license, supply-chain impact, binary size, startup, memory, CPU, and portability.
- Keep MCP/protocol types at adapter boundaries; domain services must remain transport-independent.
- Route filesystem, process, execution, system, job, identity, and audit behavior through their owning services.
- Use direct OS APIs or small platform adapters before external programs.
- Start external programs only through typed allowlisted definitions and direct argument vectors; never through a generated shell string.
- Do not add Python, PHP, Node.js, Java, or another interpreter as a normal runtime dependency.
- Keep stdout protocol-clean in STDIO and proxy modes.

## Security requirements

Every change must preserve or strengthen:

- fail-closed root configuration;
- lexical and effective path confinement;
- capability and risk-policy enforcement;
- caller authentication and authorization;
- separation of caller identity from effective execution backend;
- Version 1.0 service-account-root behavior;
- prohibition of in-process impersonation;
- principal/root/profile/backend binding of handles and state;
- global and per-principal resource limits;
- redacted errors, diagnostics, and audit events;
- deterministic cleanup, cancellation, and expiry.

Security-relevant changes require negative tests for bypass, cross-user access, unsafe fallback, and information disclosure.

## Efficiency requirements

New or changed tools must document and test:

- tool-list and instruction size;
- response, result, useful-payload, and wire-amplification bytes;
- pagination/range/batch behavior;
- maximum allocations and bounded buffering where relevant;
- latency, CPU, and memory impact for material operations;
- direct/proxy/service overhead when the change crosses runtime modes.

Payload-heavy content must not be duplicated merely for convenience. Optional accelerators and external programs require measured evidence against the native baseline.

## Documentation changes

Review and update all affected documents, including as applicable:

- `README.md`;
- `CHANGELOG.md`;
- `BACKLOG.md`;
- `docs/architecture.md`;
- `docs/security.md`;
- `docs/protocol.md`;
- `docs/specification.md`;
- `docs/tools.md` and `docs/tool-conventions.md`;
- `docs/testing.md` and `benchmarks/README.md`;
- the relevant ADR and migration document.

Do not rewrite historical migration documents. Add a new dated migration when canonical IDs or released contracts change.

Run `scripts/Test-DocumentationConsistency.ps1` for documentation changes and complete the manual checklist in [docs/documentation-quality-gate.md](docs/documentation-quality-gate.md). The automated gate does not replace validation of implementation and status claims against code, CI, reports, and Git history.

## Validation

Run the checks relevant to the change. The normal code chain is:

```bash
go fmt ./...
go vet ./...
go test ./...
golangci-lint run
go build -o build/flashgate-mcp ./cmd/server
```

Changes to PowerShell, Bash, CI, build, release, smoke, or validation scripts
also require the deterministic shell gates. On Windows, use PowerShell 7.6.4:

```powershell
& {
    .\scripts\Test-ShellScripts.ps1
    .\scripts\Test-ShellScripts.Tests.ps1
}
```

On native Ubuntu, use the fixed Bash entry point:

```bash
/usr/bin/bash scripts/test-shell-scripts.sh
/usr/bin/bash scripts/test-shell-scripts.tests.sh
```

Also run affected Windows/Linux smoke, race, schema, response-size, security, service, and benchmark checks. State explicitly when a platform-specific check could not be executed.

## Git and review

Keep changes focused and reviewable. Do not combine unrelated cleanup with a functional task. Commits, pushes, pull requests, merges, branch deletion, or remote changes require the applicable project authorization. Never commit secrets, private host paths, local credentials, generated benchmark corpora, or unreviewed release keys.
