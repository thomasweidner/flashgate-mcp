# Contributing to FlashGate MCP

## Before contributing

Read the current repository state, identify the owning `BACKLOG.md` task, and
review the directly affected architecture, security, testing, and ADR material.
Keep the change focused and preserve completed backlog entries as history.

Validation is directly affected first: run focused tests while developing,
then the complete applicable quality chain in this document and
`docs/testing.md`. Do not weaken product, security, release, metadata,
platform, shell, or documentation gates merely because a change is small.

Stop for an explicit decision when a change introduces a new product,
architecture, security, platform, dependency, release, or scope choice. Do not
commit secrets, credentials, private host paths, or dependencies on a private
development environment.

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

## Product versioning

A merge that adds a new externally observable FlashGate product capability, tool,
protocol behavior, or other user-visible function must carry the corresponding
product-version change once the canonical repository version source defined by
`BL-245` is implemented. Before Version 1.0, new product capability increments
the SemVer minor component and resets patch to zero; a compatible product bug fix
increments patch. A deliberate breaking pre-1.0 product change uses a new minor
version together with the required changelog and migration notes. After Version
1.0, normal SemVer major/minor/patch rules apply.

Documentation-only, test-only, planning-only, and behavior-neutral refactoring
changes do not require a product-version increment. Version numbers must come
from one canonical source; do not maintain competing manual version values.
Until `BL-245` completes that source, the current tag/explicit-build version
mechanism remains authoritative and no repository version bump is falsely
claimed.


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
also require the deterministic shell gates. On Windows, use PowerShell 7.6.x (Major 7, Minor 6):

```powershell
& {
    $workRoot = $env:FLASHGATE_WORK_ROOT
    if ([string]::IsNullOrWhiteSpace($workRoot)) { throw 'Bind FLASHGATE_WORK_ROOT first.' }
    .\scripts\Test-ShellScripts.ps1
    .\scripts\Test-ShellScripts.Tests.ps1 -WorkingPath $workRoot
}
```

On native Ubuntu, bind an existing caller-controlled work root on the native
filesystem and use the fixed Bash entry point:

```bash
work_root="$HOME/.cache/flashgate-mcp-shell-validation"
mkdir -p -- "$work_root"
export FLASHGATE_WORK_ROOT="$work_root"
/usr/bin/bash scripts/test-shell-scripts.sh
/usr/bin/bash scripts/test-shell-scripts.tests.sh
```

Also run affected Windows/Linux smoke, race, schema, response-size, security, service, and benchmark checks. State explicitly when a platform-specific check could not be executed.

### Product code coverage

Coverage is product-quality evidence, not a substitute for semantic, security,
boundary, race, or integration tests. Every product-code change must add or
update tests for the changed behavior and must not reduce coverage below the
active CI gate.

`BL-260` owns migration to a production-code coverage contract: at least
90.0% statement coverage as the hard merge gate on each supported CI platform,
a project target of at least 95.0%, and 100% where practical for bounded core
logic. The measured product scope is the FlashGate module package closure
actually linked into `./cmd/server`; development-only commands, vendored code,
generated artifacts, and test helpers are not allowed to distort product
coverage. Ordinary tests for the complete repository remain required.

The current lower Windows/Linux thresholds in `.github/workflows/ci.yml`
remain the active enforced values until `BL-260` raises real product coverage
and the gate together. Do not document the 90% target as already enforced.


## Git and review

Keep changes focused and reviewable. Do not combine unrelated cleanup with a functional task. Commits, pushes, pull requests, merges, branch deletion, or remote changes require the applicable project authorization. Never commit secrets, private host paths, local credentials, generated benchmark corpora, or unreviewed release keys.

After a canonical pull request is merged and `main` is read back successfully,
close every still-open superseded pull request for the same backlog owner or
implementation candidate. Remove obsolete task branches once no open child pull
request, required worktree, or other active reference depends on them. Never
delete a parent branch that is still the base of required stacked work. The
merged pull-request record remains historical evidence; it is not deleted.
Completed or superseded pull requests must not remain in the active work queue.

## PowerShell 7.6 LTS patch contract

Compatibility requires PowerShell major version 7 and minor version 6. `ObservedPowerShellVersion` records the actual patch. `MinimumPowerShellVersion` is `null` unless a specific fix establishes and justifies a minimum patch. `ServicingTarget=LatestServicedPatchWithin7.6` is the maintenance target. Exact patch, path, or hash bindings are allowed only for historical evidence, bug reproduction, installer/download/SBOM/supply-chain provenance, or a documented minimum-patch fix; each exception must be explicitly classified.
