# Prerelease publication

## Implementation and availability

BL-363 provides `releaseaudit publication-inventory`, `publication-preflight`,
`publication-download` and `publication-publish`, plus the manually dispatched
`publish-prerelease.yml` workflow. Build and candidate workflows retain
`contents: read`. Only the publication job has `contents: write` and
`actions: read`. Provider steps use the short-lived job token, subject to the
[workflow permission condition](#workflow-permission-condition-and-manual-publish-only-reconciliation).
No new long-lived credential mechanism is introduced. Compiling releaseaudit is
development tooling, not a product rebuild. Publication never builds or
repackages binaries.

[Version 0.3.0](https://github.com/thomasweidner/flashgate-mcp/releases/tag/v0.3.0)
is a public prerelease, published on 2026-10-25 from frozen source
`fa4082ce575f43b4b43c9aaa9e9fe71e3019bdb5`. Independent publication acceptance
confirmed all 40 uploaded assets: the original 39 files plus the unchanged
publication manifest. Credential-free public downloads matched every original
size and SHA-256, totaling 4,866,296 bytes. This establishes availability for
that release; each future publication still requires its own authorization and
readback. Stable Version 1.0 and its release gate remain BL-263.

## Prepare the frozen candidate

The procedures below describe the current BL-363 publisher. The accepted BL-364
target is not implemented: every release, including v0.3.0, targets exactly five
uploaded assets, the four original verified platform archives plus shared
`SHA256SUMS`. GitHub-generated source downloads are excluded. Generate four
deterministic GNU-compatible LF checksum records from original verified hashes,
one per canonical archive name. A consumer can verify only its selected platform
archive plus `SHA256SUMS` without the other three archives. Original hashes,
actual GitHub asset digests and credential-free downloaded bytes must agree in
the pipeline; read back the checksum file bytes too. No sibling `.sha256` files,
extra release-notes SHA-256 section, public evidence assets, additional public
release manifest or permanent release ledger. CHANGELOG.md alone owns narrative
notes; VERSION/source/artifact identity stays unchanged.

Complete BL-248/255/262 verification and required temporary evidence remain
mandatory, without a raw-evidence retention duty beyond regular Actions retention.
BL-262 remains Completed and unsigned. Lightweight tags retain exact version/
source parity. BL-364 adds no mandatory signed provenance/keys and activates no
Artifact Attestations or Immutable Releases. Independent release-unassigned
security owners BL-366/367 block no release and authorize no current activation.
Independently review this material plan before separately authorized technical
implementation and review the changed publisher before use. See the
[BL-364 binding](planning/implementation-contract.md#bl-364-planning-binding).

Historical v0.3.0 normalization is required for BL-364 closure. Independently
verify exact original inventory/IDs/bytes/hashes, required identity anchors and
provider state before separately authorizing upload of new `SHA256SUMS` and
deletion of exactly the 36 existing additional assets. Each mutation is one-shot
with fresh prestate and readback. Retain the four original archive IDs/bytes,
tag/version/source/prerelease identity and prove complete final five-asset
provider/digest/credential-free download parity. No permanent historical
raw-evidence dataset is required. The original 40-asset publication remains
history; BL-363 stays Completed. The current publisher implements no deletion
path. This plan authorizes no provider action and BL-364 remains Planned until
technical implementation, independent review and historical normalization pass.

1. Finalize canonical VERSION and its nonempty dated CHANGELOG section before
   candidate freeze. The date records intended publication, not its completion.
   Under BL-245, release tooling alone does not increment the product version.
2. Integrate those source bytes through the separately authorized Git process.
   Reverify the exact current main commit through current-version-candidate.yml.
   Obtain the complete four-target evidence bundle and independently trusted
   manifest/index SHA-256 values from its reviewed run evidence. Never derive
   trust solely from hashes read from an untrusted downloaded bundle.
3. Bind VERSION, full source commit, candidate run ID and attempt, and evidence
   artifact ID. The expected artifact name is
   `candidate-evidence-<commit>-<run>-<attempt>`. Missing, expired or unbound
   evidence stops publication. It is never replaced by a later build.
4. Separately authorize and create exact `v<VERSION>` on that source commit.
   The publisher only resolves existing tags, including annotated tags to their
   commit; it provides no tag-creation operation. Keep source and provider state
   stable during the operation. A changed tag or concurrent privileged provider
   mutation fails readback; preflight does not lock GitHub against administrators.
5. Verify the complete bundle and write its deterministic asset inventory:

~~~text
releaseaudit publication-inventory --dir <bundle> --version <VERSION> --commit <SourceCommitSHA> --manifest-sha256 <trusted-manifest-SHA256> --index-sha256 <trusted-index-SHA256> --output <new-assets.json>
~~~

Hash the exact generated inventory bytes for assetsSha256. Obtain the committed
CHANGELOG bytes using `git show <SourceCommitSHA>:CHANGELOG.md` and hash them for
changelogSha256. Generate notes through `releaseaudit source --root <clean-source>
--release --notes-output <new-notes.md>` and hash those exact generated bytes for
notesSha256. Preflight independently derives notes from the committed source;
local edits, undated/missing sections and differing notes fail closed. A changed
date, version, notes or release-relevant source requires a new candidate freeze.

## Explicit authorization

Prepare and review one UTF-8 JSON document with exactly these fields:

~~~json
{
  "schema": "flashgate-publication-authorization/v1",
  "action": "CREATE_DRAFT_UPLOAD_READBACK_PUBLISH_PRERELEASE",
  "repository": "thomasweidner/flashgate-mcp",
  "version": "<canonical pre-1.0 VERSION>",
  "sourceCommit": "<40 lowercase hexadecimal characters>",
  "manifestSha256": "<independently trusted original manifest SHA-256>",
  "indexSha256": "<independently trusted original index SHA-256>",
  "changelogSha256": "<committed CHANGELOG SHA-256>",
  "notesSha256": "<canonical generated notes SHA-256>",
  "assetsSha256": "<canonical generated assets.json SHA-256>",
  "candidateRunId": 123,
  "candidateRunAttempt": 1,
  "evidenceArtifactId": 456,
  "evidenceOrigin": "ACTIONS_ARTIFACT",
  "preState": "RELEASE_ABSENT",
  "originalApproval": "approval-unique-public-reference"
}
~~~

Numbers above are placeholders. Digest values are 64 lowercase hexadecimal
characters. The authorization binds the complete action sequence and exact
asset inventory, not merely a version label. It is a human-governed assertion;
a JSON file or its checksum cannot authenticate human approval. Keep references
public-safe: the publication manifest carries the binding, not secrets or private
paths. originalApproval is a unique 1-96 character reference using ASCII letters,
digits, dots, underscores and hyphens, beginning with a letter or digit. It
identifies the original human approval across JSON representations and must
never be reused after dispatch. Hash the exact externally approved bytes independently.
Whitespace and its final newline are part of that transport digest.

For an explicitly retained local copy after original Actions retention expires,
keep the original verified bundle bytes and trusted anchors unchanged. A
separately authorized transport may place that complete copy in an Actions
artifact named `retained-candidate-evidence-<indexSha256>`. Set evidenceOrigin
to `BOUND_LOCAL_COPY` and evidenceArtifactId to that copy's exact artifact ID;
retain the original candidate run/attempt as provenance. This is an explicit
new binding, never an automatic fallback or new build. The publisher verifies
the copy's availability and the full anchored original bundle rather than
claiming the expired original artifact is available.

Run the local source/evidence preflight without credentials:

~~~text
releaseaudit publication-preflight --root <clean-source> --dir <bundle> --authorization <approved.json> --authorization-sha256 <trusted-authorization-SHA256> --output <new-plan.json>
~~~

The plan is reviewable before any external action. Dispatch is a separate
authorization boundary. Only after that approval, dispatch the publication
workflow on main with authorization and authorization_sha256 containing those
exact approved bytes and digest. Ordinary main pushes cannot publish. Normal
`0.x.y` stays unsuffixed; prerelease classification does not invent `-rc.N`.

## One-shot execution and reconciliation

The workflow accepts only its first run attempt and serializes publication
without cancelling an active attempt. It checks its run identity and prior
workflow history for the same originalApproval reference. Reformatting JSON or
changing its transport digest cannot reset that consumption. A previous dispatch is
consumed even if its outcome was unclear. Unavailable or incomplete history
fails closed at the pagination bound. Do not delete publication run history
or reuse a consumed approval. This relies on GitHub retaining that provider
history; it is not an independent cryptographic replay ledger.

PREFLIGHT verifies the source, notes, complete BL-248/255/262 evidence, external
hash anchors, existing tag target and absent release for a create authorization.
DRAFT STAGING creates a prerelease draft. Its positive response ID is then read
directly through `GET /releases/{release_id}`. The paginated release list still
detects visible conflicts, but omission from that list is not treated as proof
that a known draft is absent. A failed direct-ID read never triggers another
create or an alternate read route. EXACT ASSET TRANSFER uploads every original file once.
INDEPENDENT READBACK downloads all assets and checks names, sizes and SHA-256
before PUBLICATION sets draft=false, prerelease=true and make_latest=false.
FINAL READBACK repeats tag/release/asset parity, requires a valid publication
timestamp and rejects a latest-release claim. Every asset is then downloaded
through its constructed public release URL without API credentials or cookies
and checked again for exact size and SHA-256. Another provider readback follows
those public downloads. Public download failure is HOLD even after publication;
it never causes rollback or a second publish.

An exclusively created local journal records and fsyncs each write intent before
the request. The workflow retains the plan and journal as diagnostic artifacts.
Transport failures, ambiguous responses, conflicts, incomplete drafts, upload
failures and readback failures all HOLD. No write retry, provider fallback,
automatic rollback, asset replacement or deletion is implemented. A workflow
rerun cannot resume a partial attempt. Final readback failure may occur after
publication; HOLD then means the actual remote state must be reconciled, not
that publication was rolled back.

Use read-only provider inspection to establish the actual release, tag and asset
state after HOLD. Preserve the original approval, plan, journal and run evidence.
Any repair, cleanup or further write requires a separately authorized exact
target state and fresh preflight. The automated continuation below supports
only an explicitly bound, entirely empty draft. The CLI and workflow cannot
resume partially uploaded or published releases. An already complete draft has
the separate human-governed publish-only reconciliation boundary described
below; it is never selected automatically.

## Workflow permission condition and manual publish-only reconciliation

GitHub evaluates release-write capability against workflow differences between
the resolved target commit and the default branch. Where that target introduces
workflow changes, the [update-release API](https://docs.github.com/en/rest/releases/releases#update-a-release)
requires workflow-write authority in addition to release-write access. Classic
OAuth/PAT authentication needs the existing `workflow` scope; other token types
need the corresponding repository permission. Actions `GITHUB_TOKEN` cannot
receive that authority. Check this condition before authorizing a frozen
candidate; `contents: write` alone does not guarantee publication capability.
Do not alter the frozen source or tag to bypass the condition. If the selected
executor lacks capability, stop for an explicit executor/release decision.

When a consumed attempt has left an entirely uploaded, unpublished draft,
publication may proceed only through a separately authorized manual operation:

1. Preserve the consumed approval, plan, journal and failure. Reconcile the exact
   release ID, existing tag/source, name, notes, draft/prerelease flags and latest
   state. Download and verify the entire asset inventory against the unchanged
   original manifest, including the original publication manifest itself. Any
   missing, extra or mismatching asset prevents this path.
2. Verify that an already authorized normal CLI session has the required provider
   capability. Obtain a new human decision binding the exact release ID, executor,
   payload bytes/digest and one publish attempt, including ambiguous-response and
   public-download risks. A prior upload approval cannot cover this operation.
   This procedure grants no new credentials or permissions and provides no
   automatic fallback when the existing session lacks authority.
3. After fresh read-only preflight, send exactly one `PATCH /repos/{owner}/{repo}/releases/{release_id}`
   using a reviewed payload containing only `draft=false`, `prerelease=true`
   and `make_latest="false"`. No upload, overwrite, deletion, tag/source change,
   asset replacement or workflow rerun is part of this operation. Persist the
   attempt intent and safe outcome evidence without credential material.
4. Read back release/tag/asset parity and the publication timestamp. Download
   every public asset without credentials or cookies, verify its original length
   and SHA-256, then repeat provider readback. Failure or ambiguity is HOLD and
   requires read-only reconciliation; it never authorizes a retry or rollback.

This is a manual recovery boundary, not a full-draft resume feature in
`releaseaudit`. It succeeded for v0.3.0 after independent full-draft byte checks.
The earlier workflow failure did not expose its HTTP status; the documented
capability condition does not prove that historical response or root cause.

## Explicit empty-draft continuation

Preserve the failed attempt's original authorization, exact generated plan,
journal and run evidence. Obtain a new human approval for one positive release
ID and the independently verified SHA-256 of that original plan. The new UTF-8
authorization has exactly these seven fields:

~~~json
{
  "schema": "flashgate-publication-continuation-authorization/v1",
  "action": "COMPLETE_EMPTY_DRAFT_UPLOAD_READBACK_PUBLISH_PRERELEASE",
  "repository": "thomasweidner/flashgate-mcp",
  "releaseId": 123,
  "preState": "EXACT_EMPTY_DRAFT",
  "originalApproval": "new-unique-human-continuation-approval",
  "planSha256": "<independently trusted SHA-256 of the exact original plan bytes>"
}
~~~

The example is not an authorization. The approved plan digest binds its full
original authorization, five evidence anchors, candidate run/attempt/artifact,
frozen source, tag, notes and complete asset inventory. Reusing the original
approval reference is rejected. The existing history gate also rejects any
earlier dispatch of the new reference, including after JSON reformatting.
Neither file content nor a local preflight authenticates human approval.

Use the existing commands with the additional
`--publication-manifest <unchanged-original-plan.json>` parameter. Without this
file a continuation authorization is rejected; a create authorization cannot
be used as a continuation. Local `publication-preflight` repeats the original
source/evidence verification and reconstructs the original plan byte for byte.
Its new output is a `flashgate-publication-continuation-plan/v1` containing the
new authorization and its transport SHA-256. This execution plan is distinct
from the unchanged original `publication-manifest.json` uploaded as the final
asset. No new product build, inventory replacement or plan rebinding occurs.

After independent review and separate dispatch approval, supply the exact
original plan bytes in the workflow's `publication_manifest` input as well as
the new authorization and its independently approved transport digest. The
workflow hashes both inputs before checking out the exact frozen source. It
retains the new execution plan, authorization, original plan and journal.

Before opening the new exclusive journal, the continuation reads the exact
release ID, tag target, latest state and dedicated asset endpoint. ID, tag,
source, name, notes, draft/prerelease flags and unpublished state must agree;
the asset inventory must be empty. A visible conflicting release or
contradictory list/direct-ID response fails closed. A missing, forbidden,
malformed or timed-out direct-ID response stops before upload, with no fallback.
The journal binds the new authorization digest and unchanged original-plan
digest. The continuation never calls CreateDraft. The existing upload-once,
full downloaded-byte parity, publish-once and final/public-readback sequence
then applies. Any unexpected asset or ambiguous write stops the sequence.

## Upload media negotiation and failure diagnostics

The release-asset POST sends `Accept: application/vnd.github+json` and
`Content-Type: application/octet-stream` independently, with the unchanged
original asset bytes and exact Content-Length. A non-201 response is a consumed,
non-retryable write attempt, including 415, 422, 502 and redirects. A typed
numeric HTTP status is retained through the publication CLI's HOLD boundary;
untyped provider errors and joined file-close details remain suppressed.
Response bodies, headers, query URLs and credential material are never copied
into diagnostic output. Transport failures without an observed HTTP response
retain the generic consumed-write HOLD and do not invent a status.
The actual provider cause of any historical failed upload is not inferred
from a subsequently observed empty asset inventory.

## Provider reads and diagnostics

Provider requests use the [GitHub Releases API](https://docs.github.com/en/rest/releases/releases),
[asset API](https://docs.github.com/en/rest/releases/assets) and
[tag API](https://docs.github.com/en/rest/git/tags). Provider errors are generic;
response bodies, credentials and signed download URLs are not printed. A terminal
download response other than HTTP 200 reports only its numeric status and a fixed
phase (`api_response` or `redirect_response`), including during release-asset
readback. These diagnostics identify the failing response boundary, not its cause;
other provider/transport failures remain generic. No additional request, retry or
fallback is performed for diagnosis. Writes
never follow redirects; read-only download redirects omit the job credential.

Release-state parity failures retain the stable prefix
`remote release parity failed: ` followed by one bounded JSON object (at most
4096 bytes including the prefix). Its `codes` array lists every observed failed
condition in this fixed order: `TAG_COMMIT`, `RELEASE_PRESENT`, `RELEASE_ID`,
`RELEASE_TAG`, `RELEASE_TARGET`, `RELEASE_NAME`, `RELEASE_BODY`, `RELEASE_DRAFT`,
`RELEASE_PRERELEASE`, `RELEASE_NOT_LATEST`, `ASSET_COUNT`. With no observed release,
release-dependent conditions are unavailable and omitted; independent tag/latest
conditions are still reported. Multiple codes describe simultaneous mismatches,
not one asserted root cause.

The object binds `createDraftID`, `expectedTagCommit`, `observedTagCommit`,
`expectedRelease`, `observedRelease` and `latestID`. Release projections contain
ID, tag, target, name, body, draft/prerelease flags and asset count. Every string,
including expected strings and tag/commit values, is represented only by its
decoded byte `length` and `sha256`. A missing release is `observedRelease=null`.
Direct-ID reads require explicit non-null boolean draft and prerelease flags;
missing flags stop as provider-read failures. Other scalar decoding does not
preserve whether an individual field was absent or JSON null; diagnostics must
not claim that distinction.
`latestID` must differ from `createDraftID`; the existing latest-endpoint 404
handling remains unchanged. The create ID is the existing readback argument,
not a new request or journal entry.

No raw response, free text, release notes, URL/query, header, cookie, credential,
or wrapped provider error is emitted. Inspect/API-read failures remain the
separate generic `provider readback failed` class; asset conflicts, download
failures and byte-parity failures retain their existing classes. A mismatch
code records the immediate comparison result, not proof of its external cause
or an explanation of an earlier attempt whose responses were not retained.
These diagnostics add no provider call, retry, fallback or acceptance tolerance,
and do not authorize reuse of a consumed approval or repair of an existing draft.

Actions artifact ZIP downloads request `application/vnd.github+json` from the
initial artifact API endpoint, which returns the archive redirect. Release-asset
binary downloads request `application/octet-stream` and accept a direct binary
response or an allowed redirect. Redirect requests carry neither the initial
credential nor its API headers or cookies. Offline media-type and redirect tests
verify this contract; they do not establish hosted download availability or the
historical cause of a provider HTTP 415 response.

## Download and verify

After an actual verified publication, select its exact `v<VERSION>` on the
repository's [Releases page](https://github.com/thomasweidner/flashgate-mcp/releases).
Choose the Windows x64/ARM64 ZIP or Linux x64/ARM64 TAR.GZ and its sibling
`.sha256`. GitHub's generated Source code ZIP/TAR.GZ files are separate source
archives, not the verified uploaded platform assets.

publication-manifest.json maps all original evidence paths to public asset names,
sizes and hashes. Candidate files keep their basenames; source paths become
`evidence--source--...`, and collisions fail closed. Download all mapped assets
to reconstruct the original complete bundle, rejecting unsafe paths, links,
duplicates and extra files. Verify every mapped hash against independently
trusted publication/evidence bindings, then run the existing
`releaseaudit supply-chain-verify` with independently trusted manifest/index
anchors and expected VERSION/commit. An untrusted publication manifest alone
is not authenticity proof. Evidence remains explicitly unsigned, with no SLSA
or build-attestation claim. See [artifact verification](artifact-verification.md).
