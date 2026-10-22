# Security policy

FlashGate MCP is a local host-operation gateway. Security reports are welcome,
especially for authorization bypasses, filesystem-root escapes, unsafe link or
reparse handling, secret or host-path disclosure, denial of service, and
release or supply-chain integrity failures.

## Supported versions

FlashGate MCP has not released Version 1.0 and does not currently have a
supported stable release line.

| Version | Security fixes |
|---|---|
| Unreleased development on `main` | Best effort during development |
| Tagged pre-1.0 versions | Not supported unless a release note says otherwise |
| Version 1.0 and later | Policy will be defined before the first stable release |

Do not treat the development branch as production-ready. The repository
backlog and release documentation, rather than this table, remain authoritative
for implementation and release status.

## Reporting a vulnerability

Please report a suspected vulnerability privately by email to
`thomas.weidner@gmx.at` with the subject `FlashGate MCP security report`.

Include, when available:

- the affected commit, tag, platform, and configuration;
- the security boundary or component involved;
- reproducible steps or a minimal proof of concept;
- the observed and expected behavior;
- likely impact and any known mitigations; and
- whether the report or details have already been shared publicly.

Do not include secrets, credentials, personal data, or unrelated host data.
Use synthetic fixtures and redact host paths where possible. Do not open a
public issue for an unpatched vulnerability.

Reports are handled on a best-effort basis while the project is pre-1.0. The
maintainer will try to acknowledge a report, reproduce and assess it, and
coordinate remediation and disclosure, but no response or remediation service
level is promised. If email delivery fails, open a public issue containing
only a request for a private reporting channel and no vulnerability details.

## Coordinated disclosure

Please allow time to investigate and prepare a fix before publishing details.
The reporter and maintainer should coordinate a disclosure date based on the
severity, exploitability, affected versions, mitigation availability, and
release readiness. There is no automatic disclosure deadline.

When a fix is released, the project should document the affected versions,
impact, mitigation or upgrade path, and appropriate credit if the reporter
wants attribution. Operational secrets, unsafe exploit detail, and private
reporter information must not be published.

## Scope and security boundary

The current implementation is a root-confined filesystem MCP server over local
JSON-RPC STDIO. Planned process, command, service, system-information, and
provider features are not implemented security boundaries merely because they
appear in planning documents. See [the security model](docs/security.md) for
implemented controls, target controls, residual risks, and deferred decisions.

Reports about dependencies, build workflows, release artifacts, documentation
that could cause unsafe deployment, or a credible bypass of an intended
server-side control are in scope. General support requests, feature requests,
and findings that require already-authorized local access without crossing a
documented security boundary should use a normal issue.

## Release gate

Before the first stable release, maintainers must revalidate this policy and:

- define supported release lines and security-fix lifetimes;
- verify that the private reporting contact or platform channel works;
- align disclosure and advisory handling with the release process;
- document affected-version, mitigation, upgrade, and rollback guidance; and
- keep security fixes subject to the same Windows/Linux, regression,
  provenance, and artifact-integrity gates as other release changes.

Publishing this policy does not itself satisfy the Version 1.0 security or
release gates.
