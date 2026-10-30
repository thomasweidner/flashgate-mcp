# Technical Backlog Implementation Contract

## Authority and use

This stable contract applies to every technical FlashGate backlog item, including
protocol, security, domain, runtime, release, validation, and accepted Post-1.0
work. [BACKLOG.md](../../BACKLOG.md) owns the ID, status, sprint or Post-1.0
placement, and task-specific acceptance. [Architecture](../architecture.md),
[security](../security.md), the relevant [ADRs](../adr/README.md), and detailed
plans own their respective designs. The [architecture traceability matrix](architecture-traceability.md)
records changing ownership, retrofit, and validation state. A short backlog row
inherits this contract and its family binding; it does not waive them.

Before implementation, the owning BL must resolve and record, directly or by
precise links to its family and traceability rows:

1. **Target architecture role:** MCP adapter, transport, application and
   authorization, domain, jobs and state, execution backend, platform or Native
   Tool adapter, host and runtime, release and validation, or an explicitly
   defined additional layer.
2. **Authoritative sources:** the maintained architecture, security,
   specification, planning, ADR, and public-contract sections governing it.
3. **Hard prerequisites:** only decisions and technical owners whose completed contracts must exist before this owner's implementation can start. Keep later consumers, same-family work, validation owners, and retrofit targets out of this field. Planned predecessors must occur earlier in the documented sprint and within-sprint order; completed historical prerequisites may be reused without reopening them.
4. **Must reuse:** retained packages, completed BL contracts, stores, guards,
   validators, and interfaces to extend instead of rebuilding.
5. **Retrofit targets:** existing implementation, configuration, and public
   contracts that must migrate when the new architecture lands; write `none`
   only after inspecting the current implementation.
6. **Public contract delta:** tool names, schemas, config, error, protocol,
   version, and release effects, including an explicit `none` where true.
7. **Security and resource invariants:** authorization order, principal, group,
   profile, root, capability, risk, backend and generation binding; fail-closed
   behavior, bounds, audit/redaction, stdout purity, and platform parity as
   applicable.
8. **Forbidden alternatives:** second result stores, parallel authorization or
   root validators, global registry mutation for per-client catalogs, ad hoc
   public tools, raw-shell or native passthrough, and duplicate dispatch paths.
9. **Acceptance and validation:** focused positive/negative/boundary tests,
   current [testing requirements](../testing.md), and successor CI owners.
10. **Completion state:** the old path to migrate or retire, and any residual
    explicitly dual-bound in both the closing BL and one concrete successor BL.

A BL is not implementation-ready while its architecture role, hard prerequisites,
reuse, retrofit obligations, or acceptance path are unresolved. The hard-prerequisite graph must be acyclic and have no planned predecessor scheduled after its consumer. A later consumer never becomes a prerequisite merely because it reuses this owner's output. A new or
materially changed technical BL updates its family/task binding here and its
row in [architecture traceability](architecture-traceability.md) in the same
planning change. An architecture change identifies affected current controls,
public contracts, and retrofit targets. `BL-315` owns continuing consistency
review and planned machine checks; `BL-305` and `BL-306` remain completion
controls, and `BL-308` remains the architecture and ADR owner.

## Foundation and consumer order

`SPR-048` defines release/protocol/payload/native policy contracts that can be built before profiles and Jobs. `SPR-053` establishes configuration and named-root/profile contracts, then BL-236 context, BL-159 authorization, BL-110 effective catalog, BL-166 audit/correlation, and BL-239 state binding; it completes BL-216/219/256 against those contracts. `SPR-049` builds BL-084–BL-089 lifecycle, then BL-090 storage, then dependent Operations, BL-210 Tasks mapping, BL-218 MCP resource handoff, and BL-258 payload/resource CI. Later domains consume these foundations. BL-213 single-transmission work does not require BL-090; BL-218 does. BL-220 defines early native/no-interpreter policy; BL-163 and typed-execution owners implement it later. Exact hard edges and validation/retrofit owners are maintained in [architecture traceability](architecture-traceability.md).

## Inherited contract families

| Family and BL binding | Required reuse, target boundary, and retrofit |
|---|---|
| MCP, protocol, and results — BL-090, BL-201, BL-204, BL-207–BL-219, BL-257–BL-258 | Keep domain results protocol-neutral. BL-090 owns one bounded result lifecycle; BL-218 adapts it to negotiated MCP resource handoff. Preserve exact revision negotiation and conformance, JSON Schema 2020-12, fingerprint/cache isolation, and payload single-transmission rules. Retrofit existing `read_file` and other heavy responses; do not advertise a revision before its gates pass. |
| Policy, execution context, named roots, configuration, and audit — BL-101–BL-111, BL-159–BL-166, BL-233, BL-236, BL-239, BL-352 | Build one immutable request/execution context, one server-side authorization decision before dispatch, one configuration model, and one correlation lifecycle. Migrate `MCP_ROOT`, `MCP_READ_ONLY`, current path controls, and direct MCP-to-filesystem dispatch. Bind handles, cursors, caches, and resources to the effective context; exposure does not authorize. |
| Capability and authorization verification — BL-100, BL-171 | Keep functional rights distinct from profile and risk classification; verify MCP annotations never grant authority. Reuse the central BL-159 decision and its negative tests. |
| Filesystem — BL-036–BL-067, BL-346 | Extend `internal/fs` and the existing `internal/security` path guard. Use named root IDs and relative paths, bounded reads/writes/trees, existing hash and tree primitives, and the shared result lifecycle. Migrate the current eight-tool contracts where their owners require it. |
| Search — BL-068–BL-082 | Extend the accepted `search_paths` and `search_text` public families over confined roots, bounded scanning, pages, and context-bound cursors. Pure Go is the portable baseline. Do not add a public tool per backlog row. |
| Operations and jobs — BL-084–BL-099, BL-164 | Own one generic bounded lifecycle, quotas, cancellation, TTL, cleanup, and fair scheduling. Consume policy and execution context before creating state. Keep domain meaning in domains; reuse BL-090 storage. |
| Process — BL-113–BL-135, BL-162, BL-165, BL-252–BL-254 | Reuse execution context and Operations for owned handles, bounded output, cancellation, and cleanup. External PID control is separate Post-1.0 scope. Do not use a PID alone as authority. |
| Typed execution and Native Tool adapters — BL-136–BL-152, BL-167–BL-170, BL-220, BL-353 | Resolve server-approved command IDs to executables and typed argv, with limits, redaction, and OS isolation. Native Tool adapters are optional, policy-constrained accelerators, never public aliases or interpreter/raw-shell passthrough. |
| System information — BL-062, BL-153–BL-158 | Expose only scoped, explicitly released facts through domain and OS adapters; filter environment and identity data, bound results, and retain Post-1.0 network/administrative decisions outside Version 1.0. |
| Runtime and service modes — BL-221–BL-231, BL-234–BL-238, BL-240–BL-244, BL-341 | Keep one core behind direct STDIO and local proxy/service transports. BL-233 owns configuration precedence; BL-236 owns backend-neutral dispatch. Variant A service account is the Version 1.0 backend; unsupported user-worker selection fails closed. Preserve host ownership and stdout purity. |
| Release, artifacts, and validation — BL-203–BL-204, BL-215–BL-216, BL-241–BL-263, BL-305–BL-315, BL-363, BL-364, BL-366, BL-367 | Reuse root `VERSION`, BL-245 changelog/tag semantics, BL-248 verification and current CI gates. BL-255 builds/verifies candidates, then promotes the exact bytes locally with immutable identity and no additional remote publication. BL-262 supplies evidence/rollback controls. BL-363 consumes verified candidates and evidence for explicit persistent public pre-1.0 GitHub prereleases with `prerelease=true`, exact version/source/artifact identity, no publication-time rebuild or silent asset replacement, minimal separate release-write permissions, and remote readback. Finalize release-relevant source and dated changelog notes before public candidate freeze. BL-263 separately requires stable Version 1.0 traceability/gate closure and reuses the publication foundation; prerelease success never passes that gate. Retrofit the tag-gated build/validation path, transient Actions-artifact handoff and public download guidance. BL-364 owns the accepted five-asset distribution target for all releases including v0.3.0: four original archives and shared SHA256SUMS, preserving complete verification and required temporary evidence within regular Actions retention. Release-unassigned BL-366 owns SBOM security management; release-unassigned BL-367 owns GitHub-native signed attestations. Neither blocks a release or is implicitly implemented here; see the task bindings below. |
| Response and benchmark quality — BL-172–BL-173, BL-177–BL-179, BL-205, BL-317–BL-323, BL-325–BL-329, BL-331–BL-332 | Reuse current benchmark schemas, deterministic workflow/corpus controls, host/platform evidence, and documentation gates. Strengthen checks at their existing owners without weakening result, supply-chain, platform, or public documentation truth; no benchmark result itself authorizes product behavior. |
| Accepted Post-1.0 capabilities — Post-1.0 workstream BLs in BACKLOG.md | Inherit the same context, root, authorization, domain, state, result, adapter, and validation contracts. A later milestone does not relax them or imply current implementation. Detailed names and adapter boundaries live in [tool adapters](tool-adapters.md). |

## BL-363 implementation binding

Architecture role: release and validation, with a narrow provider adapter.
The publication correction keeps that role: read a positively identified draft
by its immutable provider ID, and add a separately authorized empty-draft
continuation using the unchanged original publication manifest. Reuse the
existing evidence verifier, upload/readback sequence, consumed-approval history
and exclusive journal. Retrofit the GitHub adapter, manual workflow and
publication tests; do not add a recovery framework or product dependency.
Acceptance additionally covers list-invisible drafts, direct-ID errors and
contradictions, exact empty-draft prestate, fresh continuation approval,
unchanged manifest bytes, and credential-free public download verification.
Upload acceptance also binds response negotiation independently from the raw
request media type: JSON responses, octet-stream bodies and exact lengths.
Retrofit the request helper, upload adapter and existing execution error
boundary so only a typed numeric HTTP status survives a failed upload; never
forward arbitrary provider or file-close errors. Adapter and complete
continuation tests must prove one write, no redirect/retry and no sensitive
diagnostic output. This is the same BL-363 correction scope and adds no
dependency, release identity or recovery path.
Hard prerequisites remain completed BL-245/248/255/262. Extend cmd/releaseaudit
and consume verifySupply and candidate semantics without a second verifier.
The separate manual publish-prerelease.yml job owns minimal release-write
rights; release-build.yml and current-version-candidate.yml remain read-only
build/verification consumers. Public delta is prerelease distribution tooling,
with no MCP/runtime or product VERSION change. Retrofit download guidance and
the complete transient evidence handoff through the
[publication contract](../prerelease-publication.md).

Acceptance includes anchored four-target evidence, committed source/notes/tag
parity, one-shot draft/upload/publication, full downloaded byte readback,
permission/rerun/conflict/path negatives and applicable Windows/Linux gates.
The local BL-255/262 retrofit shares only stage-directory promotion: bounded
Windows Win32 5/32 follow-ups require stable directory/ancestor identities,
target absence and the existing complete verifier. Eight calls and a two-second
retry-control window are limits, not a blocking-call deadline. Non-Windows,
current-alias operations and remote one-shot publication retain their policies.
Acceptance also requires deterministic drift/budget tests, ownership-checked
cleanup, candidate rollback parity and native Linux single-call regression.
The first public v0.3.0 prerelease and all 40 original asset downloads passed
independent publication acceptance on 2026-10-25. The publication contract now
records the workflow-token capability condition and the separate exact manual
publish-only boundary, with no new credential mechanism or automatic recovery.
Formal BL-363 closure requires review/integration of this documentation
and the canonical full-work-item completion transition. No product,
VERSION, CHANGELOG, candidate or binary bytes change in this closure; accepted
technical/platform evidence remains applicable. Historical failed-attempt causes
remain unproven without leaving a current publication-acceptance failure.
This prerelease does not satisfy BL-263; stable publication remains BL-263.

## BL-364 planning binding

Architecture role: release/distribution and validation, extending the existing
publication adapter. Planned in SPR-048 after completed BL-363; hard prerequisites
are completed BL-245, BL-248, BL-255, BL-262 and BL-363. Reuse root VERSION,
canonical CHANGELOG notes/tag parity, internal/version archive names, BL-248
verification, BL-255 immutable candidate bytes, BL-262 complete externally anchored
evidence and BL-363 authorization, provider journal, reconciliation and public
download readback. Completed owners remain closed; BL-263 retains stable release
closure and is a later consumer, not an implementation prerequisite.

Authorities: [release architecture](../architecture.md#release-and-supply-chain-architecture),
[security](../security.md#release-and-supply-chain-security),
[artifact verification](../artifact-verification.md#native-supply-chain-evidence),
[publication](../prerelease-publication.md), [release scope](release-scope.md),
[testing](../testing.md#bl-364-five-asset-distribution-target) and
[DEC-FP-003](../adr/product-metadata.md#dec-fp-003---metadata-build-and-architecture-implementation-strategy).

### Accepted distribution target

Every FlashGate GitHub Release, including existing v0.3.0 and later stable
releases, targets exactly five uploaded assets: the four original verified
Windows x64/ARM64 ZIP and Linux x64/ARM64 TAR.GZ archives plus one shared
`SHA256SUMS`. Preserve canonical archive names and bytes. GitHub-generated source
downloads are excluded. No separate `.sha256` files or public build, test,
provenance, SBOM or other evidence assets belong in this list.

`SHA256SUMS` uses the four original verified archive SHA-256 values, with exactly
one entry per canonical archive basename, ordinal filename ordering, lowercase
hexadecimal hashes, GNU-compatible `<sha256><two spaces><filename>` records,
UTF-8 without BOM, LF and a final LF. It is generated from verified identities,
never from untrusted provider metadata. A consumer or installation script must
be able to download only its selected platform archive and `SHA256SUMS`, match
exactly that archive entry and verify the bytes without the other three archives.
This is a distribution capability requirement, not an installer implementation
under this planning task.

Original archive hashes, actual GitHub asset SHA-256 digests and credential-free
downloaded archive bytes must agree in the release pipeline. Also bind and
verify the generated `SHA256SUMS` bytes through provider digest and downloaded
readback. Missing, malformed or conflicting digests fail closed; provider metadata
alone is neither original verification evidence nor builder authentication.
CHANGELOG.md remains the sole narrative release-notes source from the frozen
commit. Add no separate SHA-256 section to release notes, no additional public
release manifest and no separate permanent release ledger. Preserve the existing
`VERSION + SourceCommitSHA + ArtifactSHA256` identity. Internal manifests, sibling
checksums and the existing one-shot journal retain their existing roles.

Complete BL-248/255/262 verification and required temporary evidence remain
mandatory. Evidence stays outside the public release assets and is available
for the verification, publication, independent review and readback that consume
it. There is no duty to retain raw evidence beyond regular GitHub Actions
retention, no required new storage service and no historical evidence dataset
as a permanent acceptance deliverable. Missing evidence needed for an actual
operation still stops that operation; an explicitly bound retained original
copy remains an existing option, never an untrusted fallback or later rebuild.
BL-262 remains Completed with its original unsigned evidence scope.

Lightweight tags remain standard with exact tag/commit/VERSION parity. BL-364
adds no mandatory signed provenance, signing keys or signing infrastructure.
It activates neither GitHub Artifact Attestations nor Immutable Releases.
Separate future security owners BL-366 and BL-367 are release-unassigned,
non-blocking work; neither is a BL-364 prerequisite.

### Historical v0.3.0 acceptance

Closure includes actual v0.3.0 normalization. Its original publication had 40
uploaded assets; preserve that true history and completed BL-363. Before any
mutation, independently verify the exact original inventory, asset IDs, lengths,
original bytes/hashes, candidate/evidence identity bindings and provider state.
Derive the new `SHA256SUMS` only from the four verified original archive hashes.
Keep the evidence needed for the operation and acceptance temporarily; no
permanent historical raw-evidence dataset is required.

Uploading the one new `SHA256SUMS` and deleting exactly the 36 existing additional
assets each require separate explicit external authorization with exact repository,
release ID, asset names/IDs/hashes, retained archive IDs/hashes, executor and
expected provider state. Bind the upload to its exact generated bytes/hash.
Implementation approval grants neither action. Each mutation is one-shot with
fresh prestate and readback. Reconcile failed or ambiguous operations read-only
without blind retry, compensation or implicit asset recreation. Any other
release or notes mutation also needs its own explicit authorization.

After the authorized operations, independently read back the complete paginated
inventory: the four original archive IDs/bytes and one new `SHA256SUMS`, exactly
five assets. Prove unchanged version, tag, source commit and prerelease identity,
original-hash/provider-digest/credential-free-download parity and checksum-file
content/byte parity. Provider restrictions, identity drift or unavailable required
evidence stop the transition without bypass. Preserve the original 40-asset
publication statement as history.

### Retrofit, validation and closure

Retrofit cmd/releaseaudit publication inventory/authorization/manifest, checksum
aggregation, provider digest/readback and historical disposition tests; the manual
publish-prerelease.yml consumer; public download/verification/build guidance;
release/security/testing contracts and DEC-FP-003. The current all-file upload
and public publication-manifest path must yield to the five-asset contract;
internal authorization and identity bindings remain. Preserve complete candidate
and evidence generation/verification, fail-closed checks, prerelease/stable
separation, minimal rights, explicit authorization, one-shot reconciliation and
secret-safe diagnostics. No second verifier, publication-time rebuild/repackaging,
silent replacement or new permanent ledger.

Acceptance covers exact five-asset and four-archive/content inventories,
deterministic GNU/LF checksum records, selected-platform-only consumer verification,
original/digest/download parity and unchanged identities. Test missing, extra,
duplicate, swapped or tampered assets, checksum entries or required evidence;
malformed digests, CRLF/BOM/unsafe names, checksum/notes/identity drift, unavailable
provider data, unsafe deletion subjects and failed or ambiguous operations.
Reuse BL-248/255/262/363 tests and applicable [testing gates](../testing.md), with
independent review of this material release/security plan before technical
implementation, and independent review of the later implementation.

The local implementation retrofits the existing publisher to project five
assets, verify selected-platform downloads and prepare separately authorized
historical upload/deletion. The old 40-asset publication remains true history;
no historical provider mutation is implied. BL-364 remains Planned until the
implementation, required independent review and actual historical normalization
are fully accepted. At closure reconcile guidance, ADRs, tests/CI, candidate
identities, distribution, migration, security and VERSION/CHANGELOG disposition.
This developer-tooling retrofit changes no product runtime or existing artifact
identity and requires no VERSION increment or historical changelog rewrite.
Git, provider/release/tag/asset, signing, permission, credential, installation and
cleanup actions retain separate authorization boundaries.

## Closure

At each BL closure, reconcile the traceability row, current documentation,
`CHANGELOG.md` and `VERSION`, tests and CI, candidate artifacts,
release/distribution, migration, and security. Mark retrofit obligations
completed only with implementation and validation evidence. A deferred item
must appear in the predecessor and one concrete successor acceptance scope.
The Version 1.0 `BL-263` gate requires no ownerless architecture requirement
or unresolved retrofit obligation in its release scope except an explicit,
approved waiver recorded by that release gate.

## BL-262 task binding

Architecture role: release and validation. Hard prerequisite: completed BL-255;
reuse completed BL-248 verification, reproduction and leak contracts, BL-245
VERSION semantics, internal/version and the existing releaseaudit CLI. Retrofit
current-version-candidate.yml at its complete manifest consumer. The tag-gated
release-build.yml stays a validation path; BL-363 owns its later publication
consumer retrofit and must use this evidence authority.

Acceptance is the [native supply-chain evidence contract](../artifact-verification.md#native-supply-chain-evidence):
one deterministic source SBOM, four unsigned in-toto Candidate-Verification statements, compact index, external
trust anchors, complete portable bundle, strict readback, leak rejection and
synthetic rollback through BL-255. No MCP/runtime contract or VERSION delta,
new dependency, second CLI/matrix/verifier, credentials or productive signing.
Build tools and third-party assets are distinguished from runtime dependencies;
source inventory never implies complete platform binary composition. BL-363
consumes the complete anchored set within retention or from a bound local copy;
BL-263 owns stable release closure. Closure requires local platform gates and
independent review; publication and provider/signing decisions remain separate.

## Release-unassigned security task bindings

BL-366 and BL-367 are independent Planned owners with no sprint, release milestone
or release-blocking obligation. They may be scheduled whenever their prerequisites
and a separate implementation authorization are satisfied. Their registration
activates no tool, external provider, workflow permission or credential change.

### BL-366 SBOM security management

Architecture role: dependency security management and validation. Authorities are
[security](../security.md#release-and-supply-chain-security),
[SPDX inventory semantics](../artifact-verification.md#inventory-and-provenance-semantics),
[testing](../testing.md#release-unassigned-security-validation) and the traceability
row. Hard prerequisites: completed BL-262 inventory and BL-324 dependency-maintenance
configuration. Reuse their SPDX 2.3 source inventory and existing Dependabot/GitHub
security functions. Retrofit the existing releaseaudit inventory consumer,
dependency-security triage, relevant CI/security gates and documentation.

Acceptance: correlate inventory dependencies with known advisories/CVEs, distinguish
direct/transitive dependencies and actual product exposure, and assess/prioritize
findings with remediation or documented exceptions. Include security/negative
tests for missing/stale or mismatched inventory/advisory data, classification
errors and unjustified exceptions. Do not claim the existing source inventory
proves full platform binary composition. Select tools and external providers
only in a later implementation task. Add no second SBOM generator, mandatory
public SBOM asset or runtime/MCP contract. Close only with accepted triage,
gates, tests and current guidance; disposition VERSION/CHANGELOG, CI, artifacts,
release, migration and security under the shared closure contract.

### BL-367 GitHub Artifact Attestations

Architecture role: release supply-chain authentication and validation. Authorities
are [DEC-FP-003](../adr/product-metadata.md#accepted-public-distribution-target),
[security](../security.md#release-and-supply-chain-security),
[testing](../testing.md#release-unassigned-security-validation) and the traceability
row. Hard prerequisites: completed BL-248/255/262/363. Coordinate with BL-364
distribution without a hard edge in either direction. Reuse the four verified
immutable archive identities, existing release workflows and unsigned evidence
without relabeling BL-262 as signed provenance.
Retrofit only later authorized workflow attestation generation/verification,
minimal permission declarations, security/negative tests and public guidance.

Acceptance: automatically create signed GitHub-native build attestations for
exactly the four verified platform archives in later authorized release workflows,
using Actions OIDC and short-lived workflow identity; verify with GitHub-native
methods bound to source/workflow/subject identity. Test tampering, missing/swapped
subjects, wrong repository/workflow/source identity, unavailable verification and
excess permission. No permanent signing keys, long-term credentials or additional
release-download asset. Provider/permission decisions and activation require the
later implementation authorization. Close with accepted generation, verification,
permissions, negative tests and guidance plus the shared closure dispositions.
