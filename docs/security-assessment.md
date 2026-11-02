# Dependency security assessment

BL-366 extends the development command `releaseaudit`; it changes no MCP
runtime contract. Local implementation remains subject to independent review
and integration. This release-unassigned work blocks no release, including
Version 1.0. Findings of every severity, unavailable fixes and assessment errors
are informative. Existing integrity, build, test and supply-chain gates remain
mandatory.

## Verified inputs and limits

`security-assess` calls the complete BL-262 verifier before correlation or any
provider request. Supply independently trusted product version, source commit,
candidate-manifest SHA-256 and index SHA-256. The reader then rechecks the
carried SPDX and `source/go.mod` hashes. Use one caller-controlled bundle without
concurrent writes. A hash read from an untrusted bundle is not a trust anchor.
See [artifact verification](artifact-verification.md#native-supply-chain-evidence).

The inventory covers active source Go modules. It excludes retained historical
`go.sum` entries. Direct/indirect markers come from the unchanged hash-bound
`go.mod`; indirect reachability must agree with SPDX dependency edges. Declared
SPDX build/runtime relationships determine exposure. Missing declarations stay
UNKNOWN; contradictions make the assessment incomplete. Build-only findings
still require triage. A runtime declaration is not symbol reachability, and an
absent reachability proof never means not affected. The current two modules are
build dependencies; the server has no external Go runtime module dependency.
Go toolchain, standard library, OS libraries, CI tools and branding are not
covered by this Go advisory assessment. No full platform-binary CVE claim follows.

## Local execution

Compile the existing command using the pinned vendored inputs, with task-local
Go cache and temporary paths. Bind `FLASHGATE_WORK_ROOT` to an existing external
task directory. Every result must be a new directory below it, outside source
checkouts and the evidence bundle. No Git or dependency mutation is performed.

```text
releaseaudit security-assess --dir <verified-bundle> --version <VERSION> --commit <SourceCommitSHA> --manifest-sha256 <trusted-manifest-SHA256> --index-sha256 <trusted-index-SHA256> --snapshot <offline-snapshot.json> --triage docs/security-triage.json --output <task-root/new-result>
```

Exactly one of `--snapshot` and `--online` is required. Real OSV queries need
their own external-action authorization; implementing or testing this command
does not perform or authorize them. The online form replaces `--snapshot ...`
with `--online`. It writes a sanitized `snapshot.json` alongside the assessment.
Offline `--at <UTC-RFC3339>` fixes the assessment clock for reproducible replay;
it is unavailable online. Freshness is measured against that explicit clock,
or the current UTC time when omitted, with a 24-hour maximum snapshot age.
This is a point-in-time assessment, not continuous monitoring.

Outputs are `assessment.json` and `summary.md`, plus `snapshot.json` online.
Status COMPLETE means the bounded source-module assessment completed with fresh,
consistent data; it does not mean vulnerability-free or product-safe. Findings
alone return exit code zero, including Critical/High and missing fixes.
INCOMPLETE or ERROR returns nonzero and a safe quality code; data/provider
failure cannot produce a findings-free PASS. Argument/output failures return
nonzero and may have no report. No private paths or raw HTTP error text appear
in reports. Preserve these temporary files only as needed within normal retention.

## OSV snapshots and correlation

The [OSV querybatch contract](https://google.github.io/osv.dev/post-v1-querybatch/)
returns IDs and modification timestamps in request order. The adapter uses exact
Go module/version identities, batches of at most 16, at most 128 modules,
16 pages per batch and 256 distinct advisory details. It follows per-query
pagination while retaining original module positions, then separately reads
[ID details](https://google.github.io/osv.dev/get-v1-vulns/). Repeated tokens,
duplicate IDs, wrong counts, null results, conflicting detail IDs/timestamps,
missing data and invalid shapes are incomplete. Requests use 15-second timeouts,
a two-minute overall budget and 1 MiB response limits. No endpoint override,
credential/cookie forwarding, redirect, retry or provider fallback is exposed.

Snapshots bind the inventory hashes, capture time, exact ordered queries and
their completeness, minimal consumed advisory fields and safe error codes.
The snapshot format is `flashgate-security-snapshot/v1`; offline input is limited
to 1 MiB, with duplicate keys, unknown properties and trailing JSON rejected.
Snapshots are externally supplied evidence, not signed OSV attestations.
The report binds their exact SHA-256 and the triage input hash.

Affected versions require an exact Go package match and either an explicit
affected version or a supported SEMVER interval in the
[OSV schema](https://ossf.github.io/osv-schema/). Unsupported or contradictory
data remains UNKNOWN/INCOMPLETE. Fixes come only from the matching affected
interval; a fix is an upgrade candidate requiring compatibility review, not an
automatically selected safe release. Explicit database severity labels map to
Critical/High/Medium/Low. Missing labels, unsupported scales and vector-only
CVSS data remain UNKNOWN; no score is invented. Priority follows severity and
retains unknown-severity review. Exposure never downgrades severity.

Declared aliases are format-checked. Records consolidate only on reciprocal
ID aliases and consistent impact/disposition for the same module/version.
A shared CVE alone does not justify merging records; conflicting/ambiguous
claims remain separate with a quality finding. Withdrawn records stay visible.
Raw descriptions, reference URLs and HTTP bodies are not copied into reports.

## Triage and exceptions

[security-triage.json](security-triage.json) is the small versioned source;
initially it contains no exceptions. Each exception requires exact `advisory`,
`module`, `version`, a nonempty `reason`, accountable `owner`, and a future UTC
`reviewBy` timestamp. Example entry in the `exceptions` array:

```json
{
  "advisory": "GO-2026-1000",
  "module": "example.org/build-tool",
  "version": "v1.2.3",
  "reason": "Review the fixed tool against the controlled build inputs.",
  "owner": "maintainer",
  "reviewBy": "2026-12-01T00:00:00Z"
}
```

This example is synthetic, not a project exception. Wildcards, missing reasons
or owners, invalid/expired review times, duplicates, conflicting entries and
unmatched identities create quality findings. Exceptions change presentation
only: the finding, its severity and required review remain visible. They grant
no release permission. Review upgrade compatibility or record a bounded,
justified exception/mitigation when a fix is unavailable; never mutate dependencies
automatically. Dependabot/GitHub Security remains a complementary maintenance
signal with unchanged configuration and no added alert API or credentials.

## Candidate and publication

The existing candidate workflow evaluates after full four-target generation and
independent verification, including its manual `workflow_dispatch` route. No
second matrix, SBOM or required check is introduced. It passes producer-bound
anchors into a separate step and retains temporary results for 14 days.

The existing publisher evaluates only after `publication-publish` returns
success following final public-byte readback and journal completion. Its
separate step consumes the already approved evidence identities, forwards no
publication credential and performs no release write. Assessment and summary/
retention steps use `continue-on-error: true`; provider failure is visible in
the Actions summary without changing candidate or successful publication status.
No tag, notes, asset bytes, five-asset inventory or permissions change. A manual
publish-only reconciliation outside that workflow may invoke the same local CLI
after confirmed success with separate provider authorization. No existing
publication or Actions dispatch is repeated for this feature. Stable publication
remains BL-263; its publisher can reuse this assessment call unchanged.

No separate security-only workflow or added GitHub capability is needed; the
existing dispatch and local CLI cover the requested manual routes. Hosted
activation and actual provider results require later authorized execution;
offline fixtures and local HTTP servers establish implementation behavior only.
