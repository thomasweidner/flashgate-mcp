# Artifact verification

FlashGate verifies release artifacts against the same canonical build identity
used by the executable, Windows resources, Linux build information, archive
names, and release workflows. BL-248 does not introduce a second version or
product-metadata source.

## Linux verifier process cleanup

The bounded Linux verifier starts a new session and binds its direct child's
PID, process start ticks, session and process group. It observes parent exit
with `waitid(WNOWAIT)` and retains the waitable leader until group cleanup has
finished, preventing reuse of the numeric group identity. Stream capture
threads do not reap processes or send signals.

Every terminal path kills only that identity-checked group, including after
leader exit and child reparenting. A bounded procfs identity readback confirms
that no live members remain in the pinned session/group; it does not discover
ownership by searching for arbitrary PIDs or ancestry. Unexpected group changes,
unreadable state, signal failure or unconfirmed termination fail closed.
Parent reaping, group termination and stream closure are checked separately.
A three-second cleanup budget is followed by at most 200 ms of capture-thread
cancellation; no wait, join or drain is unbounded.
Existing validation reasons are retained and cleanup failures are appended.

This is a verifier process-group boundary, not a general process sandbox.
An unrelated session is never signalled. Retained pipe writers outside the
bound group cause a bounded stream-cleanup failure. The native process contract
includes uncooperative children and foreign-session pipe writers. Its intentional
parent-exit-before-release fixture accepts either parent-dead or child-ancestry
rejection only with confirmed child exit and no timeout activation or survivor.

## Canonical expectation sources

The root `VERSION` file supplies the canonical controlled product version;
`internal/version` owns the runtime identity model and development defaults.
Controlled build inputs are validated by
`scripts/Build-InputValidation.ps1` and
`scripts/build-input-validation.sh`. The Windows resource generator, embedded
manifest verifier, icon verifier, release builders, and archive audit consume
those values instead of a separately maintained expected-metadata document.

Release validation uses `VERSION`, its exact `v<VERSION>` tag, the clean source
commit, and source time resolved by the controlled build or workflow. A passed
release version can only assert equality with `VERSION`. Public `x64` maps to Go
`amd64`; public `arm64` maps to Go `arm64`.

## Verification layers

| Layer | Canonical implementation | Contract |
|---|---|---|
| Windows binary | `scripts/Test-WindowsMetadata.ps1`, `scripts/VerifierProcess.ps1`, `cmd/versionmanifest`, `cmd/iconverify` | PE x64/ARM64 machine type, `VERSIONINFO`, canonical product fields, embedded build manifest, icon identity, Go target, and bounded compact/verbose/help execution only after all static checks pass and the target matches the actual Windows OS architecture |
| Linux binary | `scripts/Test-LinuxMetadata.sh`, `scripts/verifier-process.sh`, `scripts/bounded-process-runner.py`, `cmd/versionmanifest` | ELF x64/ARM64 machine type, static linkage, Go build ID, Go/VCS provenance, canonical build manifest, and bounded compact/verbose/help execution only after all static checks pass and the target matches the intrinsic `uname -m` host architecture |
| Release package | `scripts/Test-ReleaseArtifact.ps1`, `scripts/test-release-artifact.sh`, `cmd/releaseaudit` | exact ZIP/TAR.GZ names and inventory, regular-file entry types, checksum match, controlled extraction, and rejection of traversal, links, special entries, duplicates, missing files, or unexpected files |
| Reproducibility and leaks | `cmd/releaseaudit compare`, `scan`, and `scan-file` | two-build binary/archive/checksum/inventory identity plus machine-readable host-path, credential-pattern, private-key, and caller-supplied forbidden-value findings |
| Orchestration | `.github/workflows/metadata-regression.yml`, `.github/workflows/release-build.yml`, and the controlled native Linux validation driver | Windows and Linux x64/ARM64 build and static validation; native execution on matching x64 hosts; deterministic reports and aggregated nonzero failure status |

The verifier, rather than its caller, determines whether execution is native.
Windows uses the actual OS architecture, not the PowerShell process
architecture. Linux normalizes the controlled `uname -m` result. Nonmatching
targets are automatically `SKIPPED`; `--skip-execution` is only an additional
Linux restriction and can never enable a cross-built target. ARM64 artifacts
remain cross-built and statically inspected on current x64 runners; native
ARM64 execution is not claimed by the current runner model.

Every child process has a deterministic execution timeout and separate bounded
stdout and stderr capture. Both streams are drained concurrently. Windows uses
a second bounded cleanup deadline and reports whether the process was already
exited, tree-killed, fallback-killed, or could not be terminated; Linux uses a
bounded process-group cleanup path. No exit, drain, kill, or cleanup path waits
without a deadline. Start failure, nonzero exit, timeout, output overflow,
termination failure, or cleanup failure is `FAIL`. Static failure prevents
launch and records `StaticValidationFailed`; a nonnative target records
`NonNativeTarget`.

## Structured evidence

Platform validators emit one compact structured result block with status,
artifact identity, platform metadata, warning count, error count,
`RuntimeExecution`, `RuntimeFailureReason`, `HelpContract`,
`HelpSkipReason`, and `HelpFailureReason`. `cmd/releaseaudit` writes strict
typed JSON reports for archive inventory, reproducibility comparison, and leak
scans.
These Go report types and their focused tests are the canonical report
contract. A parallel JSON expected-metadata file or a second report schema
would duplicate the current typed implementation and is therefore not used.

Unknown CLI arguments and missing required values fail closed. Shared build
fixtures cover valid and invalid SemVer and source-time values. Embedded
manifest tests reject missing, duplicate, and unknown fields. Archive tests
cover traversal, links, special entry types, duplicate paths, incomplete
packages, checksum mismatch, reproducibility mismatch, and leak findings.
`scripts/Test-VerifierProcess.ps1` and
`scripts/test-verifier-process.sh` permanently drive controlled nonzero
compact/verbose/help, empty/partial help, timeout, stdout/stderr-limit, start,
and cleanup failures through the complete production verifiers. They assert
structured state, error aggregation, and final exit behavior. Static-failure
cases assert both runtime and help skip states and marker absence. Child
cleanup requires an explicit `READY` marker and confirms process exit plus the
continued absence of the delayed survivor marker.

## Acceptance evidence

### Public publication handoff

Current implementation: BL-363 uploads the full verified bundle plus its
publication manifest; v0.3.0 was accepted with 40 assets. Accepted future target:
Planned BL-364 changes every release, including v0.3.0, to exactly five uploaded
assets, four unchanged original verified archives and shared `SHA256SUMS`.
Deterministic GNU-compatible LF records use the four original archive hashes,
one entry each. A consumer downloads only its platform archive and this file.
Actual GitHub digests and credential-free downloaded bytes must match original
hashes; checksum-file byte parity is also required. No sibling `.sha256`, extra
notes checksum section, public evidence assets, additional public release
manifest or permanent release ledger. Full BL-248/255/262 verification and
required temporary evidence remain, without raw-evidence retention beyond regular
Actions retention. BL-262 stays Completed; no mandatory signed provenance, keys
or implicit Attestations/Immutable Releases activation. Historical normalization
needs exact original/provider checks, separate one-shot checksum upload and
36-asset deletion authorizations and readback; retain four original archive IDs/
bytes and tag/source/prerelease identity, then prove five-asset parity. Preserve
the original 40-asset history and completed BL-363; no permanent historical
dataset is required. The procedure below remains the current implementation
until the separately authorized and independently reviewed retrofit is accepted.
See the [BL-364 binding](planning/implementation-contract.md#bl-364-planning-binding).

The planned lane is `BL-248 verified artifact -> BL-255 immutable candidate
identity -> BL-262 evidence -> BL-363 publication`. BL-248 remains the sole
artifact-verification authority; publication adds no competing verifier.
BL-255 binds `VERSION + SourceCommitSHA + ArtifactSHA256` and promotes those
bytes locally without an additional remote upload. BL-262 owns supply-chain
and rollback evidence; BL-363 consumes and carries or unambiguously binds the
public-distribution evidence alongside the verified archives and checksums.

BL-363 implements the manual path for persistent GitHub prereleases below 1.0.0 with
`prerelease=true`. It must publish exactly the verified Windows/Linux x64/ARM64
archive bytes, without build, rebuild, repackaging, or silent asset replacement.
For a public candidate, release-relevant source and the dated canonical
changelog entry are final before freeze; the exact `v<VERSION>` tag must target
its SourceCommitSHA. Changed source/notes/date requires a new candidate and
verification. Identity, hash, tag, asset, permission or required-evidence drift
fails closed; remote readback verifies the published identity and bytes.
Stable Version 1.0 remains a separate BL-263 gate and publication.

The [publication consumer](prerelease-publication.md) reruns the existing
supply-chain verifier with external manifest/index anchors. All original bundle
files become individually uploaded assets; publication-manifest.json maps their
original paths to public names and binds sizes and SHA-256. Consumers reconstruct
the original bundle and reuse supply-chain-verify. The publisher never edits
accepted evidence. Source archives generated by GitHub are separate from these
verified uploaded assets. Local validation establishes no hosted publication.

### Current validation evidence

BL-255 adds a current-version candidate workflow on pull requests to `main`.
It compares root `VERSION` at the exact PR head with the event-bound base and
skips the matrix when unchanged only if the candidate workflow already exists
in that exact base commit. A missing base workflow enables the bootstrap matrix
for the first PR introducing this lane; an existence-query error fails closed.
An input-free `workflow_dispatch` on `refs/heads/main` always reverifies root
`VERSION` at the validated current main commit, so finalized notes/date can be
verified again without changing the version. Non-main branch and tag dispatches
are skipped before checkout. Every dispatch job explicitly checks out main;
a main SHA change between jobs fails closed. Both paths share the same
record/manifest and BL-248 gates. For later BL-363 reverification, the finalized
source must first be the current main state. A changed PR version, bootstrap PR or dispatch requires the four Windows/
Linux x64/ARM64 targets. Each target runs the existing BL-248 platform verifier,
independent second build, `releaseaudit compare`, and `releaseaudit scan` before
uploading its archive, sibling checksum, verifier output, audit reports, and
strict target record as transient Actions evidence. A final job assembles a
four-target `flashgate-current-candidate/v1` manifest only when every target
passes. The verifier output is bound to the same archive name, version and
architecture, and must contain exactly one canonical `Sha256` field: a
64-character lowercase hexadecimal SHA-256 of the controlled archive copy
actually checked by the platform verifier. All four targets require this hash
to equal the candidate record's `artifactSha256`. Missing, empty, malformed,
duplicate, noncanonical or differing hash fields fail closed, even when the
checksum, reproducibility and leak reports otherwise pass. Report and artifact
hashes are checked again when assembling the manifest and during local
promotion; evidence from another archive with the same version, source commit
and architecture cannot authorize the candidate bytes.

`releaseaudit candidate-promote` takes a local bundle, that manifest, expected
version and full source commit, and a local destination. It rehashes all source
files, checks the four BL-248 evidence records, copies the exact bytes through
a temporary sibling directory, and reads back every promoted file before the
immutable directory becomes visible. A present identity with different bytes
fails closed. The optional `current` file is a mutable pointer to that directory
and grants no identity authority. Promotion checks its prestate before exposing
a new identity. A failed alias update or cleanup restores the original alias
bytes/mode or absence and removes only an identity created by that invocation;
preexisting verified identities are retained. Unrecoverable filesystem errors
are reported with rollback parity unresolved and retained recovery evidence.

Candidate and supply-chain stage promotion on Windows permits at most eight
rename calls, retrying only actual Win32 error codes 5 or 32. The two-second
retry-control window does not bound blocking operating-system calls. Immediate
success does not sleep. Before a follow-up, the original stage/ancestor identities,
absence of the sibling target and the complete caller-specific stage verification
must still hold. Identity handles close before rename. After success, the target
must be the bound stage and the old stage absent, followed by full content
readback. Ambiguous states retain recovery evidence and stop; no unknown target
is overwritten or deleted. Other platforms use one rename; current-alias and
remote publication operations never acquire this retry policy.

This local operation builds, repacks and
uploads nothing; BL-262 owns supply-chain/rollback evidence and BL-363 owns
public publication. The initial hosted four-target candidate run passed for BL-255 in PR #303
before its 2026-10-11 merge. Future candidate identities still require hosted
Windows/Linux evidence; local tests do not substitute for it.

Validate each applicable binary and archive against the current build identity.
Record native execution separately from cross-build and static inspection; a
cross-built ARM64 binary is not evidence of native ARM64 validation. Keep
run-specific results and independent review records as external evidence, not
as a second current task queue in this document.

Generated binaries, archives, extracted trees, and validation reports are not
repository source. The authoritative task status remains in `BACKLOG.md`.

## Related documentation

- [Build and release metadata](build-metadata.md)
- [Manual metadata validation](metadata-validation.md)
- [Testing](testing.md)

## Native supply-chain evidence

BL-262 extends cmd/releaseaudit using the complete BL-255 four-target manifest
after every BL-248 verification/reproduction/leak gate has passed. Evidence is
created in a new directory outside immutable promotion directories. No archive,
checksum, record, existing manifest or promoted identity is changed.

The complete portable bundle contains six evidence documents at its root:

- sbom.spdx.json: SPDX 2.3 source dependency inventory;
- provenance-windows-x64.json and provenance-windows-arm64.json;
- provenance-linux-x64.json and provenance-linux-arm64.json;
- supply-chain-index.json: schema flashgate-supply-chain-evidence, contractVersion 1.

The candidate/ directory carries the original manifest, all four archive/checksum
pairs, target records, verifier reports, BL-248 reproduction reports and leak
reports unchanged. source/ carries the committed VERSION, go.mod, go.sum,
vendor/modules.txt, third-party notices and branding inputs. These carried files
make the bundle independently consumable from a bound local copy after Actions
retention expires. The compact index binds the manifest rather than copying its
full semantics; it records VERSION, source commit and commit time, source input
hashes, the four artifact/verifier/reproduction identities, SBOM/provenance hashes
and unsigned signing status. Paths are relative to this one evidence set.

Linux artifact verification invokes `Test-LinuxMetadata.sh` with `--portable-report`
before candidate-record hashes are bound. `BinaryPath` then names the checked
archive-relative `flashgate-mcp_<VERSION>_linux_<arch>/flashgate-mcp` entry.
The metadata verifier still uses the actual local binary path for every static
and runtime check; direct diagnostic invocation without this option retains its
normal `BinaryPath` output. Portable mode rejects a mismatched release layout.
Reports are never redacted or rewritten after their hashes have been bound.

Generation requires an exact source commit checkout, source input parity with
that commit, expected VERSION/commit and an externally bound candidate manifest
SHA-256. Text checkout EOL conversion is permitted, but carried source inputs use
committed bytes. A new output directory is required; an existing output is a
collision even for identical content. A private sibling stage is re-read and
verified before directory publication, then verified again. Failure never grants
consumer readiness. Caller-controlled roots must not be modified concurrently. Candidate archives
are copied through bounded streams and are not loaded into memory for bundle
inventory checks. Metadata reads are limited to 2 MiB; archive reads/copies are
bounded at four BL-248 maximum entry sizes plus 64 KiB of container overhead
(268,500,992 bytes). Oversize or growing inputs fail closed before consumer
readiness, and destination files are created exclusively.

Run the existing CLI with explicit paths and independently bound identities:

~~~text
releaseaudit supply-chain-generate --root <checkout> --dir <candidate-directory> --output <new-evidence-directory> --version <VERSION> --commit <SourceCommitSHA> --manifest-sha256 <candidate-manifest-SHA256>
releaseaudit supply-chain-verify --dir <evidence-directory> --version <VERSION> --commit <SourceCommitSHA> --manifest-sha256 <candidate-manifest-SHA256> --index-sha256 <trusted-index-SHA256>
~~~

Both commands accept repeated --forbidden values for host-specific leak checks.
The verifier requires external index and manifest digest anchors; reading a hash
from an untrusted bundle and trusting it is not authenticity verification. It
reuses BL-255 candidate semantics and rechecks archive, checksum, record,
verification, reproduction and leak bindings. It regenerates the expected six
canonical documents from carried inputs and independently reads and compares
their exact bytes, then rejects extra/missing files or directories, duplicates,
links, traversal, alternate file types, swapped subjects and identity drift.
Hashes establish integrity under trusted anchors; unsigned evidence provides no
cryptographic author or builder authentication.

### Inventory and provenance semantics

The SBOM describes the current source dependency set, not an exhaustive inventory
of each platform binary. goversioninfo and rsrc are build-only Windows resource
generators. The current server has no external Go runtime module dependency.
The pinned Font Awesome Free SVG and derived ICO are separately identified
third-party branding assets with actual file SHA-256 and CC-BY-4.0 attribution.
Go standard library, compiler, OS libraries, runner tools and their transitive
supply chains are outside the claimed inventory. Retained go.sum test/historical
entries are carried as checksum evidence and are not misrepresented as active
vendored or shipped modules. Unsupported replacement/workspace/module-purpose
changes fail closed for a reviewed inventory-contract update.

Go h1 values identify normalized module directory or go.mod checksums; they are
recorded as Go-specific facts and never labeled ordinary archive SHA-256.
Active module requirements, versions, both h1 entries, vendor module headers,
explicit markers and package ownership must agree. SPDX relationships bind the
application, both build dependencies, their dependency edge and branding
derivation. filesAnalyzed=false limits the package claim to source metadata.

The four unsigned in-toto Statement/v1 subjects each name exactly one platform
archive with its exact SHA-256. Their project-specific Candidate-Verification
predicate type is the stable versioned URI
`https://github.com/thomasweidner/flashgate-mcp/attestations/candidate-verification/v1`.
This URI identifies the contract defined here; it is not a build service or
an additional candidate authority. The predicate records bindings recovered
from an independently reverified BL-255 candidate:

| Required predicate field | Exact meaning |
|---|---|
| version, platform, architecture | Candidate VERSION and the subject's target from its manifest record |
| source | uri is git+https://github.com/thomasweidner/flashgate-mcp; digest.gitCommit is the full candidate source commit |
| candidateManifest | path is candidate/candidate-manifest.json; sha256 is the externally anchored original manifest digest |
| verification | path is candidate/ followed by the record's verificationEvidence; sha256 is its verificationSha256 |
| reproduction | path is candidate/ followed by the record's compareReport; sha256 is its compareSha256 |

All fields are required; paths are relative to the portable bundle. The verifier
requires the exact canonical statement bytes, field set, subject and bindings,
including when statement and index digests are rebound. It rejects the former
SLSA predicate type and shape, unknown versions/fields, missing fields, nulls,
duplicate keys and trailing JSON. The unpublished initial evidence contract
remains version 1; no accepted external consumer uses the superseded draft.
The four provenance-*.json filenames and index provenance fields remain the
transport names for these Candidate-Verification statements.

This is verification evidence, with no SLSA level, build-attestation, builder
identity, invocation timestamp or signature claim. An evidence reconstruction
tool is not the artifact builder. sourceCommitTime and the deterministic SBOM
created field use the source commit time, explicitly not an observed
build-invocation or evidence-generation clock.

Format references: [SPDX package information](https://spdx.github.io/spdx-spec/v2.3.1/package-information/),
[in-toto statement](https://github.com/in-toto/attestation/blob/main/spec/v1/statement.md).

### Consumer and transient CI handoff

current-version-candidate.yml runs releaseaudit tests independently of the optional
VERSION-change matrix, then generates and verifies evidence from the complete
four-target manifest without a second matrix. Before upload, generated evidence, carried verifier/record reports
and source inputs pass existing leak rules plus runner-specific forbidden
values. Build/verification permissions remain contents: read. No OIDC,
attestation, release publishing or new service permission is added.

candidate-evidence-<commit>-<run-id>-<attempt> is the complete Actions bundle with
14-day retention; the step summary records index and manifest SHA-256. BL-363
must bind the exact run/attempt, VERSION, source commit, index/manifest digests
and four artifact hashes, securely copy the complete set within retention, or
consume an explicitly bound local copy. Missing/expired/unbound evidence blocks
publication. Never rebuild or regenerate from unrelated source after expiry.
The existing release-build.yml remains tag-gated validation and transient upload;
BL-363 owns its later safe publication retrofit, consuming this single authority.

### Signing boundaries

Current evidence and candidate bytes are unsigned. Windows Authenticode would
change PE bytes and must be a separately authorized build/sign/verify pipeline
before candidate freeze, followed by fresh archive hashes and reproduction policy.
Do not append embedded signing to already verified binaries or promotion bytes.
Certificates, hardware/provider selection, timestamp services, identities and
permissions require a separate decision; no key or credential is provisioned here.

Linux detached signing may later bind an exact archive and/or index digest with
a separate signature, approved algorithm, trusted key identity and independent
verification. Detached signatures do not alter archives, but signer authenticity,
rotation/revocation and provider permissions still require separate authorization.
No .deb/.rpm package-signing path or productive signer is activated by BL-262.

### Controlled rollback smoke

Rollback selects a previously verified immutable candidate using existing
candidate-promote --current with the prior candidate directory, manifest,
VERSION and source commit. Revalidate its evidence and all hashes before selection;
reuse its original bytes without rebuilding. Keep the previous identity and
bound current pointer for recovery, then read back alias bytes and candidate
parity. Failure must preserve the previous alias and immutable bytes; existing
BL-255 injection tests cover update/cleanup failures and restoration parity.
The focused BL-262 synthetic smoke returns to a verified prior identity and
rejects a tampered prior artifact without changing the pointer.

A pointer/file rename is only a filesystem selection operation. The existing
backup/install sequence can expose a brief absent-current interval; it is not a
transaction across running processes, services, configuration or data. Already
running processes retain their old image. Service stop/start, installation,
schema/data migration, updater behavior and productive deployment rollback are
outside this tooling contract and need their own authorized procedure.
