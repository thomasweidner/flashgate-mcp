# Documentation quality gate

This gate checks the public FlashGate checkout. It supplements `BACKLOG.md`,
accepted decisions, product code, tests, and release evidence; it does not
replace editorial or semantic review.

## Run the gate

Use the Go toolchain declared in `go.mod`, Git, and PowerShell 7.6.x:

```powershell
& {
    & ./scripts/Test-DocumentationConsistency.ps1
    if ($LASTEXITCODE -ne 0) { throw 'Documentation consistency failed.' }
}
```

The native inventory, naming, prose-candidate, and link checker can also run
independently on Windows or Linux:

```bash
go run ./cmd/doccheck
go test ./cmd/doccheck
```

Both commands emit or test deterministic results. The checker emits one JSON
report and exits nonzero on a finding; the composite PowerShell gate adds the
product/backlog/security/CI consistency checks. Source files are not changed.
Normal Go compilation may use the caller's build cache and temporary directory.
The PowerShell gate uses the installed toolchain and does not install one.

## Coverage and rules

Follow [documentation style](documentation-style.md). The inventory comes from
Git's tracked files plus nonignored new files, not a fixed documentation list.
Deleted files are absent from the candidate inventory. The composite gate also
requires the canonical public entry points, so deleting an authority cannot
silently shrink the required set.

All first-party Markdown files are inspected, including ADRs, planning,
metadata decisions, and documentation outside `docs/`. Vendored upstream
Markdown is counted separately and left unchanged. Ignored generated files
are not source documentation. The checker requires regular readable UTF-8
files without NUL bytes, safe repository-relative paths, at most 4 MiB per
file and 32 MiB in total, and a bounded Git inventory.

The automated rules cover:

- stable lower-kebab-case topic filenames and the explicit conventional names;
- no dated, PR-, sprint-, mobile-, or execution-review filenames;
- suspicious non-English prose and comments in fenced examples;
- relative inline and reference links, target path spelling and case, and
  Markdown heading or explicit HTML-anchor destinations;
- complete public-document coverage, stable backlog and sprint IDs, the six canonical semantic human-status values, exact Markdown status-cell rendering (bold for non-terminal values and plain for terminal values), and separate Post-1.0 placement parity;
- current versus planned product/protocol behavior and release boundaries;
- existing Windows/Linux build, coverage, lint, metadata, shell, documentation,
  release, and security gates;
- no private host paths, task infrastructure, or private control-plane
  dependencies in public guidance or implementation sources.

Link validation is local: it does not contact external sites or claim that an
external URL is live. Repository-root escapes and absolute local links fail.
Code examples are not rendered links. Heading fragments are checked against
ATX/setext headings, duplicate-heading suffixes, and explicit HTML anchors.

## Editorial review

The language rule is a heuristic vocabulary scan, not language identification
or proof of complete English prose. Inspect every maintained document and
review each finding; do not merely remove a matching word. Preserve intentional
Unicode examples, API identifiers, proper names, licenses, and third-party
attribution. Code payloads are not translated to satisfy a prose rule.

Confirm current implementation claims against source and tests. Keep accepted
future contracts explicitly separate from available features. Verify moved
path references in scripts, CI, plain code spans, and generators as well as
rendered Markdown links. Unchanged release/security/compatibility requirements
must not be weakened to pass the checker.

Retain original execution evidence outside the public tree before retiring a
historical report. Consolidate applicable contracts and retain unresolved work
under its existing owner. Removing a report never resolves its findings.
`CHANGELOG.md` remains the narrative release-notes source.

## Backlog-item completion review

Before a backlog item is marked complete, perform a semantic closure review in
addition to the automated documentation checks:

- update the owning `BACKLOG.md` row in the same closure change;
- verify the BL's [implementation-contract](planning/implementation-contract.md) family and [architecture-traceability](planning/architecture-traceability.md) row, including architecture role, prerequisites/reuse, retrofit targets, validation, and successor-bound residuals; new or materially changed technical BLs update both authorities together;
- satisfy the complete acceptance scope, or bind every intentionally deferred
  deliverable to one concrete existing successor BL and add that deliverable to
  the successor's own acceptance scope;
- review all directly affected current documentation and remove stale
  current/planned/implemented claims;
- explicitly disposition `VERSION`, `CHANGELOG.md`, tests/CI, current-version
  candidate artifacts, release/distribution, migration, and security impact;
- whenever `VERSION` changes, require one nonempty `## [<VERSION>]` changelog
  section in the same change. The section may be undated before publication; a
  public release adds ` - YYYY-MM-DD` only with the matching `v<VERSION>` tag.

A predecessor-only note such as "document later" is not a valid handoff. The
successor must carry the obligation in its own canonical backlog scope so a
future implementation chat can discover it without reading historical work.
These are semantic completion obligations; the current checker does not infer
all of them from arbitrary diffs. Sprint-level documentation review remains an
aggregate backstop rather than the first reconciliation point.

## Regression and CI boundary

`cmd/doccheck` is a development command using only Go's standard library; the
product server does not import it. Its fixtures cover filename exceptions,
Unicode and prose boundaries, malformed text, symlinks, missing/duplicate
anchors, reference links, case mismatches, path escapes, ignored/vendor files,
and failed Git inventory. The normal full Go tests include these regressions.

The composite PowerShell gate runs a deterministic positive and negative
fixture matrix for human status and Post-1.0 placement, then checks the real
backlog and sprint tables. It keeps technical result and protocol states outside
the human-status contract.

CI runs the checker on Windows and Ubuntu and runs the composite gate on the
exact PR head. Changes to PowerShell or CI additionally require the shell gates
in [testing](testing.md). Public CI consumes only checkout files and standard
runner tools, never personal paths, credentials, or contributor-local reports.

## PowerShell compatibility

Compatibility requires PowerShell major 7 and minor 6. Record the observed
patch separately and maintain against the latest serviced 7.6 patch unless a
documented fix establishes a higher minimum.
