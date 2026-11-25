# Technical Implementation Contract

## Authority and use

This stable contract applies to every technical FlashGate backlog item, including
protocol, security, domain, runtime, release, validation, and accepted Post-1.0
work. [BACKLOG.md](../../BACKLOG.md) owns the ID, status, sprint or Post-1.0
placement, and task-specific acceptance. [Architecture](../architecture.md),
[security](../security.md), the relevant [ADRs](../adr/README.md), and detailed
plans own their respective designs. The [architecture traceability matrix](architecture-traceability.md)
records prerequisites, retrofit targets and closure evidence. Every backlog
row inherits this contract and its applicable technical family.

Before implementation, the owning backlog item must resolve and record, directly or by
precise links to its family and traceability rows:

1. **Target architecture role:** MCP adapter, transport, application and
   authorization, domain, jobs and state, execution backend, platform or Native
   Tool adapter, host and runtime, release and validation, or an explicitly
   defined additional layer.
2. **Authoritative sources:** the maintained architecture, security,
   specification, planning, ADR, and public-contract sections governing it.
3. **Hard prerequisites:** only decisions and technical owners whose completed contracts must exist before this owner's implementation can start. Keep later consumers, same-family work, validation owners, and retrofit targets out of this field. Planned predecessors must occur earlier in the documented sprint and within-sprint order; completed historical prerequisites may be reused without reopening them.
4. **Must reuse:** retained packages, established contracts, stores, guards,
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

A backlog item is not implementation-ready while its architecture role, hard prerequisites,
reuse, retrofit obligations, or acceptance path are unresolved. The hard-prerequisite graph must be acyclic and have no planned predecessor scheduled after its consumer. A later consumer never becomes a prerequisite merely because it reuses this owner's output. A new or
materially changed technical BL updates its family/task binding here and its
row in [architecture traceability](architecture-traceability.md) in the same
planning change. An architecture change identifies affected current controls,
public contracts, and retrofit targets. `BL-315` owns continuing consistency
review and planned machine checks; `BL-305` and `BL-306` remain completion
controls, and `BL-308` remains the architecture and ADR owner.

Any owner that adds or materially changes a public tool, schema, effective
profile or capability composition, protocol revision, entry instructions, or
adapter that affects the public wire contract inherits
[BL-215 catalog acceptance](#bl-215-catalog-and-entry-budget-binding), including
Post-1.0 owners. An internal-only adapter records that its catalog and entry
delta is absent and measures its own affected cost under its existing contract.

## Foundation and consumer order

`SPR-048` defines release/protocol/payload/native policy contracts that can be built before profiles and Jobs. `SPR-053` establishes configuration and named-root/profile contracts, then BL-236 context, BL-159 authorization, BL-110 effective catalog, BL-166 audit/correlation, and BL-239 state binding; it completes BL-216/219/256 against those contracts. `SPR-049` builds BL-084–BL-089 lifecycle, then BL-090 storage, then dependent Operations, BL-210 Tasks mapping, BL-218 MCP resource handoff, and BL-258 payload/resource CI. Later domains consume these foundations. BL-213 single-transmission work does not require BL-090; BL-218 does. BL-220 defines early native/no-interpreter policy; BL-163 and typed-execution owners implement it later. Exact hard edges and validation/retrofit owners are maintained in [architecture traceability](architecture-traceability.md).

## BL-236 execution identity

BL-236 supplies a transport-neutral application boundary after BL-100–BL-109,
BL-233 and BL-374/375. It replaces the direct MCP-to-root-registry execution
path of the eight filesystem tools. The existing catalog, effective profile,
explicit grants, risk classifications, resource limits, root registry,
PathGuard and protected native filesystems remain authoritative at their
respective boundaries.

Bind the OS-derived direct caller to the inherited STDIO session. Carry its
principal, permitted security attributes, trusted origin, server-generated
request and correlation ID, accepted protocol binding, cancellation, deadline
and response budget. Bind the full immutable effective policy snapshot and
runtime generation separately from the selected backend and effective OS
identity. The caller and public tool arguments cannot select the backend.

Dispatch typed operations and results through one application boundary. A
reviewed declaration binds the functional operation, risk and independently
ordered resource operands, including their roles, access requirements and
limits. Check the effective profile and each resource grant before backend
execution. Validate the same binding at the effect and result boundaries.
Carry the BL-109 working-directory proof when applicable and deny a process
effect without native enforcement. Preserve BL-374 protection and BL-375
exclusions on actual resources, aliases, indirect effects and content-derived
results. Keep the existing pre-effect mutation receipt budget.

Each successor consumes the shared context and its task-specific Backlog
acceptance. The owner declares the operation and resource rights and validates
the actual effect, result release or delegation under the selected OS identity.
Include an allowed case and a denied or stale binding case that demonstrates no
effect or useful result. Retained state uses BL-239. New transports must supply
trusted ingress identity and use the same dispatch. Capability discovery,
installed adapters, transport metadata and caller claims confer no authority.
There is no automatic backend fallback. Activation is gated by each successor's
native enforcement and security acceptance.

BL-159 owns the later common central policy decision. BL-237 implements the
dedicated service-account backend. BL-238 specifies the worker protocol and
BL-240 implements the isolated native workers. BL-236 activates the direct route
only. It has no public config, tool, schema or wire delta. Verify the existing
wire, result budget and catalog fixtures, native Windows and Linux protection,
race safety, and typed backend and operation extension with test fixtures.

## BL-159 central authorization

BL-159 owns the shared policy decision at `internal/execution.Dispatcher.Dispatch`.
BL-236 already routes all eight production STDIO filesystem operations through
this typed transition. BL-159 supplies the common Authorizer there. Trusted
composition registers immutable operation and variant declarations. The
Authorizer checks the selected declaration, effective grants and derived risk
before resolving operands, then proves each ordered operand's access from the
same snapshot. The production MCP registry is sealed after request-bound
registration. Native object, identity and PathGuard checks, result-budget
admission before mutation, and bound completion and result release remain
independent checks. BL-160 owns the broader denial matrix and BL-263 checks every
shipped route against its owner's evidence.

Every current or newly activated productive tool operation uses this transition.
This includes resource-free operations, changed variants under an existing
name, and independently dispatched effects and delivery actions without a
public tool name. The domain supplies a trusted declaration of the operation,
variant, required rights, risk and ordered resources with their roles. The
Authorizer uses the trusted caller, effective profile, risk ceiling and operand
rights from one validated policy snapshot. Reject missing, unknown or
incompatible declarations. Catalog visibility, annotations and technical
support confer no grants. A denied operation reaches neither a backend nor
its fallback.

The direct STDIO implementation preserves these declarations and access
requirements. Overwrite and recursive deletion select reviewed variants.
The same Dispatcher supports a resource-free declaration, covered at BL-159
acceptance by a test-only operation.

| Current operations | Required access |
|---|---|
| `list_directory`, `read_file`, `get_path_info` | `files.read` on the selected operand |
| `write_file`, `create_directory`, `delete_path` | `files.write` on the selected operand |
| `copy_path` | `files.read` on the source and `files.write` on the target |
| `move_path` | `files.write` on both operands |

Current copy and move require the same root ID for both operands. The separate
[BL-376 cross-root contract](#bl-376-explicit-cross-root-verified-file-copy)
governs directed cross-root copy and the grants reused by later directed variants.

### Authorization units and verification outcomes

- **Action and independent items.** Check the general action requirement before
  domain execution. If it is denied, reject the entire call. For admitted
  `read_files`, `get_paths_info`, `hash_paths` and `verify_paths` calls, authorize
  each input independently. Preserve its input position, return a safe per-item
  failure on denial and deliver allowed siblings within the shared response
  budget. Denied items produce no protected reads, hashes, cache entries or
  content-derived results.
- **Composite assertions.** Authorize all contributing operands before dispatch
  or release of a shared assertion such as `compare_paths(A, B)`. Apply this
  rule when a cached hash or other retained evidence is available. Suppress
  equality, difference and derived evidence when a contributing operand is
  denied.
- **Multi-step effects.** Require all determinable rights before the first
  effect. Revalidate applicable rights and bindings at each relevant effect
  boundary, including derived operands as they become known. On late denial,
  stop dependent effects and report only proven partial state visible to the
  caller. Before source deletion in a move, verify source identity, current
  deletion authority and verified destination adoption. A partial copy confers
  no authority to remove the source.

The planned `verify_paths` contract accepts one or more independent
expected-state records. A checked record has a confirmed match or mismatch.
Denied and otherwise unresolved records contribute neither a match nor a
mismatch. The aggregate follows the complete submitted set.

| Aggregate | Required outcome |
|---|---|
| `verified` | Every submitted record was checked and matched. |
| `mismatch` | Every submitted record was checked and at least one differed. |
| `partially_verified` | At least one record has a confirmed outcome and at least one remains unresolved. Keep confirmed mismatches visible in their corresponding item results. |
| `unverified` | No record has a confirmed outcome. |

The aggregate boolean `verified` is true only for the `verified` outcome. When
the caller lacks authority to check a name's existence, return the same safe
per-item `access_denied` whether the object exists or is absent, including for
an expected absence. Authorized, reliably established absence can confirm that
expectation. The BL-160 and BL-346 acceptance uses a match, a mismatch and a
denied item to require `partially_verified`, `verified: false`, and a visible
mismatch at its original position. Repeat with the denied name present and
absent and compare the full semantic client result.

### Protected fields and later actions

Apply field requirements before protected acquisition and at result projection.
Preserve them in predicates, counts, inferred statements, retained data,
alternate result representations, errors, diagnostics and audit. A denied
field contributes no caller-visible derived information.

In resolved FlashGate tool calls, a malformed opaque handle argument yields
`invalid_arguments`. A valid argument with a missing general action right
yields `access_denied` before lookup. With that right, a syntactically valid
unknown, foreign, expired, stale or currently invisible handle yields the same
safe `not_found`. Independent MCP protocol methods use the error contract of
their negotiated revision.

Under BL-239, match the originating binding and authorize the current action
before a job runs, a later status or control action takes effect, or a result,
page, resumed stream or buffered notification is released. A scheduled run
checks the rights of its bound identity at execution time. Server-owned cleanup
is restricted to resources with proven ownership. A provider callback that
adds rights, resources or effects requires a fresh BL-159 decision before the
effect. Even within the original scope, enforce the native effect and bound
result checks of the selected backend.

## BL-102 filesystem resource contract

BL-102 owns transport-neutral local filesystem addressing in SPR-053. Reuse
the completed BL-101 registry and its BL-233 configuration, BL-374 source
protection, shared effect coordination, and existing `internal/fs` and
PathGuard. Retrofit the eight current filesystem tool schemas, adapters and
bootstrap together. The [filesystem ADR](../adr/filesystem-abstraction.md)
and [security model](../security.md) govern confinement and native effects.

The filesystem interface receives an exact configured `RootID` and a relative
path together for each operand. The registry binds each ID to its validated
root backend, and that backend uses PathGuard for path resolution. Copy and
move receive separate source and target operands and reject distinct root IDs
before invoking native operations. The public fields are defined in the
[tool reference](../tools.md). The configured default ID remains part of the
TOML contract. Tool callers supply every ID and path explicitly.

Every new or materially changed local filesystem consumer must use this
boundary for direct and indirect filesystem operands, including working
directories, executables, inputs, outputs, archives, staging and cleanup.
Domain, Native Tool, execution and provider owners must prove that the native
route and its results preserve confinement, configuration protection and
limits. Each operand retains its own root binding. Root selection, capability
grants, per-root policy, principal authorization and backend selection are
separate controls. Registry keys, sysctls, services, the clipboard and other
non-filesystem resources use their own resource contracts.

Acceptance covers closed schemas, explicit selection in single and multiple
roots, aliases, redaction, native open and mutation checks, sibling effects,
and the existing catalog and platform gates. The [migration guide](../migration.md)
owns the client input change. Record it under Unreleased with the product
version assigned through release preparation. BL-103–BL-110, BL-159, BL-236,
BL-239 and BL-352 retain their respective policy, execution, state and
discovery scope. Independent terminal review is required before closure.

## BL-104 named policies and root access

BL-104 owns the global named policy definitions in BL-233's typed TOML model
and the first consumer in SPR-053. Reuse BL-100 functional grants, BL-101's
immutable root registry, BL-102's exact operands, BL-103 profiles and risk,
and BL-374 configuration protection. Retrofit the config loader and root
registry used by server bootstrap and all eight filesystem operations. The
[configuration guide](../configuration.md#named-policies) defines the public
TOML delta. Tool names, schemas, and profile inventory stay stable.

A policy definition is inert until a resource references it. A consumer
projects the typed fields supported for its resource. Explicit resource fields
override matching fields individually, including an empty replacement list.
The filesystem consumer defaults missing access to read and checks each exact
root ID before backend dispatch. List, read, and stat require read. Write,
mkdir, and delete require write. Copy requires source read and target write.
Move requires write on both operands. The existing cross-root prohibition,
PathGuard, limits, selected-configuration protection, and profile checks
remain mandatory. Use the shared registry for current dispatch and preserve
the binding for future domain, Native Tool, and provider routes.

BL-105–BL-109 and later domain owners extend the same typed policy model with
their own supported fields and tests. BL-236 carries the effective policy in
context, BL-159 composes it with authorization, and BL-352 owns discovery.
BL-373 owns the dedicated authorized configuration route. Each later backend
owner proves the selected source and all delegated effects preserve BL-374.

Acceptance covers schema rejection, replacement and safe default semantics,
exact and overlapping roots, all eight operations, pre-dispatch denial and
redaction, independent profile gates, and native Windows/Linux configuration
protection and wire checks. Use the existing CI matrix and coverage gate.
The [migration guide](../migration.md#root-access) covers explicit write
opt-in for existing configurations. Record the change under Unreleased and
include the intended backlog closure in the independent terminal review.

## BL-108 per-root capability mapping

BL-108 extends the shared functional capability model in SPR-053. Reuse the
BL-100 catalog, sets and operation requirements; bind the BL-101/102 root IDs
and operands, BL-103 measured profile and risk policy, and BL-104 named policy
inheritance. Retrofit the typed TOML loader, bootstrap, root registry,
filesystem dispatch and all eight current filesystem tools.

Code declares operation requirements and their resource applicability. A
profile must grant every required capability within its risk ceiling. Each
applicable operand must also have the required capability in the selected
resource's access set from the same catalog snapshot. Copy needs source read
and target write. Move needs write on both operands. Both remain same-root
only. Backend or adapter availability conveys no authority.

Profile definitions use `[profiles.<id>]` and root definitions use
`[files.<id>]`. Keep profile selection, named policies and independent field
overrides. Root access accepts only known, filesystem-applicable capability IDs
and defaults to `files.read` if absent at both levels. An explicit list replaces
the inherited list. Reject the old nesting and short access values.

Preserve BL-105 limits, BL-106 file-type checks, BL-107 native identity and
link checks, and BL-374 configuration protection. BL-375 adds an independent
exclusion check that also applies with full functional grants.
Exercise the cutover, exact operands, profile/root intersection, risk,
pre-effect denial, active and inactive definitions, alias protection and
native platform gates. Tools, wire schemas, VERSION and dependencies remain
stable. Record the configuration break under Unreleased and in migration
guidance. Complete independent review and verify integration before closure.

Future consumers declare reviewed requirements and applicable resource
operands, reuse the same grants and access sets, and test denial independently
of technical availability. See the backlog's cross-epic rules.

## BL-109 process working-directory policy

BL-109 supplies a shared non-executing domain policy boundary. It reuses the
BL-101 exact root registry, BL-102 PathGuard and operand bindings, BL-104
named policies, BL-107 native identity, BL-108 resource capabilities,
BL-233 typed configuration, BL-374 protection and BL-375 exclusions.
Extend the typed TOML loader, immutable root definitions and existing
registry backends. Preserve independent profile and risk checks. Bind the
selected directory, root and effective policy without exposing a native path.

Validate strict defaults and overrides, exact root and directory identity,
protected effects, overlapping roots and exclusions, missing grants and
enforcement, and denial before the non-executing dispatch boundary on native
Windows and Linux. Keep MCP schemas, advertised tools, VERSION and dependencies
stable. BL-139 integrates the operand into typed execution and BL-151 owns
productive activation. Execution owners apply the durable
[working-directory and delegated-effects contract](../security.md#process-working-directory-and-delegated-effects)
at their effect boundary and test the actual enforcement.

## BL-105 resource limits

BL-105 is a shared domain policy foundation, with configuration and MCP result
adapters. Its hard prerequisites are completed BL-233/374 and BL-100–104.
Extend the typed TOML model, named-policy composition, immutable root registry,
`internal/fs` and the existing single-transmission result and transport paths.
Retrofit every current filesystem tool, protocol limit, logging record bound,
IPC frame setting, bootstrap, fixtures and maintained configuration examples.

Global `files` limits are hard ceilings. A named policy supplies optional
per-field limits; explicit root fields replace those fields. Reject attempts
to exceed global ceilings, including unused policy definitions. Request and
operation bounds may only tighten the effective root snapshot. Access,
capabilities, risk, PathGuard and selected-configuration protection remain
independent gates. Copy stays within one exact root ID and consumes the same
write bound as a complete write, plus its source file/scan bounds.

Use positive byte integers or unambiguous English SI/IEC byte/bit strings for
size fields. Remove public byte-key aliases, including copy's separate bound.
One bounded exact parser and one canonical lossless formatter serve future
BL-373 configuration views. Startup normalizes only in memory and never writes
the configuration. Preserve tool schemas, revisions, error categories, wire
budgets and product VERSION; document breaking TOML migration in Unreleased.

FILE bounds content handled by read, write and copy. WRITE bounds new content
for write and copy. SCAN bounds bytes actually read and directory names visited,
including filtered entries; entry limits independently bound traversal.
RESULT bounds the serialized complete MCP result, including escaping and
revision-owned metadata. Adapters reserve their exact envelope additions with
the shared result allowance before effects; the protocol response bound includes JSON-RPC framing
and the transport newline. Mutating tools must admit their bounded receipt
before side effects. TEMP bounds aggregate live reservations per exact root
through `Registry.ReserveTemp`, plus tighter operation/request allowances; no current
filesystem operation creates scratch files. Memory and wire buffering retain
their separate existing bounds. Future scratch consumers reserve before
allocation and release on success, denial, cancellation and failure.

Small typed meters and root snapshots must be usable below MCP by Go and
simulated external backends. Missing necessary hard enforcement denies dispatch;
a backend configuration claim alone is insufficient. No external process is
activated. BL-151/152 retain actual OS enforcement, BL-090 retains result storage,
and BL-236/239 retain context and lifecycle extensions. Each future tool owner
declares applicable dimensions, measurement units, aggregate effects, hard
enforcement and cancellation/cleanup evidence in the existing BL-220 and
BL-205/214/215 contracts before activation. No second registry or quota engine.

Acceptance includes exact parser/formatter boundaries and roundtrips, unused
policy rejection, immutable and overlapping roots, all eight tools, no grants
from limits, protected operands, scan/filter exhaustion, safe serialized result
denial before mutations, full wire limits, concurrent reservations, adapter
parity and missing-enforcement negatives. Run the applicable native Windows and
Linux, race, coverage, conformance, documentation and source-hygiene gates in
the testing guide before independent terminal review.

## BL-106 per-root file-type policy

BL-106 supplies the shared filesystem file-type policy foundation in SPR-053.
It is a name restriction, not a MIME classifier. BACKLOG.md remains its sole task
and acceptance authority. The [filesystem ADR](../adr/filesystem-abstraction.md),
[capability ADR](../adr/capability-profiles.md),
[runtime configuration ADR](../adr/runtime-modes.md#configuration-contract),
[execution-identity ADR](../adr/execution-identity.md) and
[configuration protection contract](../security.md#configuration-trust-and-protection)
govern its boundary. Its hard prerequisites are completed BL-100–BL-104,
BL-233 and BL-374. BL-105 is not a hard prerequisite; its existing resource
controls remain applicable. Later domain, transport, Native Tool and provider
owners are consumers, not prerequisites or unfinished BL-106 deliverables.

Extend `internal/config`, BL-104 named-policy inheritance and per-field root
overrides, `internal/roots` immutable snapshots, `internal/fs`, PathGuard and
the shared configuration protection/effect coordination. Resolve and validate
once before exposure; pass an immutable effective policy below MCP. Do not
add runtime TOML parsers, a second root validator, security registry, policy
engine or adapter-specific allowlist.

### TOML and matching contract

The typed bootstrap model supports the following fields under
`[policy.<name>]` and `[files.<id>]` only:

```toml
[policy.documents]
allowed_extensions = [".txt", ".md", ".tar.gz"]
allow_extensionless = false

[files.work]
path = "/srv/flashgate/work"
policy = "documents"
allowed_extensions = [".txt", ".md"]

[files.unrestricted]
path = "/srv/flashgate/unrestricted"
policy = "documents"
allowed_extensions = "unrestricted"
allow_extensionless = true
```

An absent root field inherits that field from its referenced policy. An
explicit root list replaces the entire inherited list, including `[]`.
The exact scalar `allowed_extensions = "unrestricted"` explicitly removes
an inherited restriction on suffixed names; no other scalar string is valid.
An explicit boolean replaces the inherited boolean independently. Resolve
both fields before applying defaults; omission at the root is inheritance,
not a reset. If `allowed_extensions` is absent at both levels, there is no
additional extension restriction. Preserve this distinction in the typed
model: absence and an explicit empty list must never collapse to one value.

If `allow_extensionless` is absent at both levels, its effective default is
true for absent or explicit unrestricted extensions, and false for an
explicit extension list, including `[]`. An explicit or inherited boolean
always takes precedence over this mode-dependent default. Thus a root that
removes a restrictive policy for all names sets both fields as in the
`unrestricted` example above; the scalar alone does not erase an inherited
`allow_extensionless = false`. A root can likewise replace an unrestricted
extension mode with a list while independently retaining or overriding the
inherited extensionless setting.

| Effective extension setting | Effective extensionless setting | Additional name restriction |
|---|---|---|
| Absent or `"unrestricted"` | No explicit/inherited boolean: true | None, including extensionless files |
| Absent or `"unrestricted"` | Explicit/inherited false | Suffixed names only |
| `[]` | No explicit/inherited boolean, or false | No regular-file names admitted |
| `[]` | Explicit/inherited true | Extensionless names only |
| `[".txt"]` | No explicit/inherited boolean, or false | Matching `.txt` names only |
| `[".txt"]` | Explicit/inherited true | Matching `.txt` and extensionless names |

A validated empty list always denies all suffixed files; it never means
unrestricted access. Missing configuration adds no file-type restriction
and does not require existing configurations to opt back into file access.
Resolve these semantics once into each immutable effective root policy. Validate
even unused policy definitions. Unknown fields/references, wrong types,
duplicate keys or entries, malformed tokens and unsupported policy forms
fail startup without partial activation or fallback. Unrestricted matching
removes only this name restriction; all other security controls still apply.

Each extension token starts with a literal dot followed by one or more
nonempty dot-separated ASCII segments matching `[a-z0-9][a-z0-9_-]*`.
Require lowercase canonical tokens and reject whitespace, empty segments,
wildcards, MIME names, path/stream separators, control characters and
non-ASCII lookalikes. Extension lists contain no wildcard or allow-all token;
the distinct scalar `"unrestricted"` is the explicit unrestricted mode.
For a restricted list, match only complete terminal suffixes of a regular
file's final basename using ASCII
case folding, independent of OS locale and filesystem case behavior. An
allowed `.gz` also matches `a.tar.gz`; `.tar.gz` alone admits `a.tar.gz`,
not `a.gz`. `.txt` admits `A.TXT`, not `a.txt.exe`. Never trim trailing dots
or spaces, decode escapes, normalize lookalikes or remove a final suffix
to obtain a match. Deny ambiguous native name mappings that cannot preserve
this interpretation. A name with no dot after its first character, including
`.env`, is extensionless and requires `allow_extensionless = true`;
`.env.txt` is suffixed. A trailing dot or empty final segment is not an
extensionless escape. An extension requires a nonempty basename before its
leading dot.

Keep name matching, content/MIME classification and native object type
separate. An allowed suffix neither proves contents nor turns a directory,
link, stream or special object into a regular file. Directory traversal and
creation use their existing structural contract, not a suffix exemption
for descendant files. Resolve permitted links through PathGuard; check the
applicable operand name and resolved regular-file name without allowing a
renamed link to launder a denied target type. Unsupported or unprovable
object/name classification denies the affected operation without content
sniffing, hydration or weaker backend fallback. Denials use the existing
redacted domain/MCP error mapping consistently across backends.

### Retrofit, invariants and acceptance

Retrofit all eight existing operations: `List`, `Read`, `Stat`, `Write`,
`Mkdir`, `Delete`, `Move` (including rename) and `Copy`. Bind each exact root
operand to its effective policy before backend dispatch and recheck at the
actual open/effect/result boundary as required by identity protection.
Listing omits denied file entries; stat and reads deny disallowed file
operands. Preserve permitted content-independent configuration metadata only
when all applicable policies allow it. Check prospective file names before
creation and source and destination names before copy, rename, move or
overwrite, including the existing destination object. For directory effects,
boundedly inspect every affected descendant file and projected destination;
deny a recursive mutation before effects if any required check fails. Include
parent creation, indirect effects, staging and cleanup where applicable.
Do not add directory copy, cross-root movement or another public operation.

Overlapping roots retain their exact selected ID and own effective policy;
do not borrow another root's permissions or retry under a more permissive
ID. Preserve shared alias/object checks and effect coordination, including
the selected runtime configuration path, loaded/current objects and protected
parents. An allowed extension, MIME claim or available backend grants no
authority and never overrides PathGuard, configuration protection,
capabilities, root access, principal restrictions or resource limits.
BL-107 retains native link/reparse semantics. No new protection guarantee
supersedes the documented independent-OS final-window residual; detectable
drift and FlashGate-started or delegated effects remain hard denials.

Acceptance requires native Windows and Linux positive, negative, boundary,
alias, deterministic race and unsupported cases through all eight operations.
Cover inheritance and replacement, absent/empty/invalid/duplicate/unsupported
policies, unrestricted overrides and independent boolean inheritance. Prove
absent configuration preserves access subject to existing controls, `[]`
denies suffixed names, and the table's extensionless cases remain distinct.
Cover immutable snapshots, case and compound suffixes, extensionless and
hidden names, deceptive/trailing/stream suffixes, non-regular objects, both
operands and overwrite, recursive pre-effect denial, overlapping roots and
protected aliases with otherwise complete grants. Prove no content open or
effect before denial and successful allowed siblings. Contract tests must
prove identical decisions and redacted failures across supported backends,
and denial when required classification or enforcement is unavailable.
Reuse existing MCP, BL-220 adapter, security, BL-105 resource, BL-205/214/215
wire/catalog, conformance, coverage, race and CI gates from the
[testing guide](../testing.md); do not introduce parallel gates.

Before eventual BL-106 closure, satisfy its full backlog acceptance and this
binding, reconcile configuration/template, migration, ADR/security/tool/test
documentation and Unreleased notes, and explicitly disposition VERSION,
tests/CI, candidate artifacts, release/distribution and security impact.
Require independent terminal review and verified integration. After full
implementation acceptance, BL-106
stays closed: later consumers extend their own acceptance and native/contract
tests for each applicable source, destination, backend, result, fallback and
delegated effect. BL-045 owns MIME/representations, BL-068 and its implementing
search BLs own search, BL-107 owns symlink/reparse behavior, BL-345, BL-347,
BL-348, BL-354, BL-359 and BL-360 own their platform and adapter semantics,
BL-181/188 own provider
decisions and runtime, and BL-373 owns authorized configuration management.
These consumers reuse the effective policy without creating another owner
authority or implicitly reopening BL-106.

The native implementation checks the original basename before OS path
normalization as well as the resolved name and object type. Relative symlink
relocation to another parent and directory moves containing relative links
remain denied under a restriction when their projected native target is not
proved. Same-parent link renames and absolute links still pass normal guards.
No productive external backend, new tool or deferred BL-106 feature is implied
by these explicit fail-closed unsupported routes.

## BL-107 per-root symlink and reparse policy

BL-107 extends the existing configuration, PathGuard and native filesystem
boundary. Its hard prerequisites are completed BL-104, BL-105, BL-106 and
BL-374 and their named-root/configuration foundations. Reuse the immutable
root registry, per-field policy inheritance, shared cross-root effect lock,
file-type checks and selected-configuration identity protection. Retrofit all
eight existing operations, their recursive/compound effects and bootstrap.
There is no second registry, policy engine or generic execution engine.

Only named policies and root tables accept `follow_symlinks`, a strict
boolean. Resolve root override, inherited value and default false once at
bootstrap. Validate unused policies. Remove `security.follow_symlinks` without
an alias or automatic migration. A link setting adds no capability or access.
Preserve all independent access, resource, extension, risk and protection
checks. Tool names, arguments, result schemas, VERSION and dependencies stay
unchanged; Go's project minimum becomes 1.27.4.

PathGuard owns link classification, bounded relative-link resolution and
effective-root confinement. Deny absolute targets, Windows junctions and
unknown/unsupported reparse objects. Native execution binds the authorized
root and parent/object identities rather than reopening an unchecked absolute
pathname. Use `os.Root` for the root handle and confined standard operations;
add only operation-specific native no-follow and mount/volume checks where
the standard library cannot establish the required boundary. Linux must not
traverse nested mounts, including same-device bind aliases. Unknown native
enforcement fails closed. Symlink metadata never implies target permission.

Open existing output handles without truncation. Check identity, protection,
both operands and result admission before destructive effects, and recheck
after deterministic test interception. Bind parent effects for create,
mkdir, unlink and rename; preserve the shared effect lock across overlapping
root IDs. Recursive scans do not descend into links and preflight all effects.
Do not claim identity-conditional namespace mutation or absolute race freedom
where the OS cannot prove it. The narrowly accepted independent-OS-process
final-window residual in the BL-374 security contract remains explicit.

Acceptance covers defaults and mixed roots, inheritance and explicit false,
strict schema and retired global fields, relative/absolute links, chains,
cycles, native junction/reparse/mount cases, hardlinks, overlapping IDs,
protected read/copy sources and mutation destinations, deterministic late
link/parent/source/target drift, extension-policy independence, unchanged
siblings, redaction and native/MCP parity. Run the complete applicable Windows
and native Linux testing, race, coverage, conformance, source and budget gates.
Privileged unavailable cases are reported as unexecuted, never PASS.

Future filesystem, search, operations, process, Native Tool and provider
owners consume this existing boundary for every operand and effect and prove
their own routes before activation. They do not reopen BL-107. BL-108/109 and
other independent product tasks remain separate. Documentation, migration,
Unreleased, traceability and reviewed backlog closure belong to this delivery;
release/distribution and taskroot cleanup are outside it. Independent terminal
review and actual integration/main readback govern completion.

## BL-375 per-root file and directory exclusions

BL-375 is release-unassigned. The [backlog](../../BACKLOG.md#named-roots-capabilities-and-profiles-epic)
owns acceptance and scheduling. The [traceability row](architecture-traceability.md#core-policy-context-catalog-and-state-foundation)
records the same dependencies and retrofit.

1. **Target architecture role.** Add a server-enforced restriction to the
   configuration, named-root and native filesystem policy boundary below MCP.
   An immutable bootstrap snapshot binds an effective `excluded` list to each
   named root. A component pattern match on a whole path or ancestor denies
   that path regardless of object type or existence. Apply every applicable
   root's exclusions at the shared resolution and effect boundary.
2. **Authoritative sources.** Follow the [named-root architecture](../architecture.md#profiles-capabilities-and-named-roots),
   [per-root security policy](../security.md#per-root-policies-and-execution-backend),
   [configuration protection](../security.md#configuration-trust-and-protection),
   [typed configuration](../configuration.md#path-exclusions),
   [ADR-005 exclusion decision](../adr/filesystem-abstraction.md#bl-375-path-exclusions),
   [ADR-009](../adr/capability-profiles.md),
   [ADR-010](../adr/operations-and-jobs.md) for retained and deferred work, and
   the [adapter policy](tool-adapters.md#bl-220-adapter-selection) as
   applicable to consumer backends.
3. **Hard prerequisites.** The completed BL-101/102 registry and operands,
   BL-104 named policies and root access, BL-105 resource limits, BL-106 file-type
   policy, BL-107 native link/effect safeguards, BL-108 capability intersection,
   BL-233 typed configuration and BL-374 configuration protection are the
   required foundations. Later search, jobs, context, authorization, Native
   Tools and providers are consumers. Each must prove the effect and result
   boundary for its own routes before activation.
4. **Must reuse.** Extend `internal/config`, `cmd/server` bootstrap,
   `internal/roots`, PathGuard and `internal/fs`. Reuse shared cross-root
   effect coordination, native root/parent/object binding and revalidation,
   recursive effect checks, resource meters and redacted denial handling.
   Preserve the independent protection of the active configuration.
5. **Retrofit targets.** Bind validated exclusion rules at bootstrap and
   enforce them at the shared resolution, traversal, native open/effect and
   result boundaries. Cover list, stat, read, write, mkdir, delete, move and
   copy, including parent listings, recursive scans, implicit and projected
   paths, and source, target and replacement effects. A compound mutation
   that would reach an excluded path must deny before modifying an
   otherwise allowed destination or ancestor. Check original and projected
   paths for directory effects, including descendants of allowed parents.
   Later filesystem and search routes (including BL-068), Operations and
   retained state (BL-084/239),
   context and authorization (BL-236/159), root discovery (BL-352), Native
   Tools (BL-360), providers (BL-181/188) and authorized configuration
   management (BL-373) must demonstrate the boundary for their own effects
   and results when applicable.
6. **Public contract delta.** The
   [server-side `excluded` list](../configuration.md#path-exclusions) applies
   to named policies and root overrides. Omission inherits or defaults to
   empty. An explicit root list replaces the inherited list, including
   `[]`. Only `*` and `?` have wildcard meaning within a component. Existing
   operations gain redacted denials and listing omission. Tool names,
   operands, schemas and transport contracts stay unchanged.
7. **Security and resource invariants.** Exclusions remain authoritative
   with full root read/write rights, profile and capability grants, and with
   permissive file-type or link policy. Apply them after admission by both
   independent BL-108 capability layers. Preserve all existing authorization
   checks and scan, staging and result limits. Apply the same denial below
   every transport to direct, indirect, staged, delegated and job effects,
   including revalidation of deferred work before reuse and publication.
   Hide excluded entries in parent listings and prevent protected metadata,
   names or content from leaking through errors, logs, counts, hashes, search
   snippets, previews, receipts, caches or retained result delivery through
   excluded paths. Apply native path semantics to alternate root IDs,
   overlap and case aliases. Evaluate each hardlink under its own name.
   Allowed aliases can read or write shared file contents. Preserve BL-374's
   object-identity denials through every hardlink to the active configuration.
   Check symlink entries and supported resolved targets through BL-107 and
   deny unsupported links, reparse points, mount paths and unproved bindings.
   Preserve allowed sibling access, native identity and drift checks, and the
   existing parent and ancestor protections. The scope covers FlashGate and
   delegated effects.
8. **Forbidden alternatives.** Reject a second root registry, path
   validator, or adapter-specific policy engine. BL-074 search patterns and
   BL-080 ignore files remain query controls. They cannot supply server-side
   denial. Exclusions introduce no glob support in public tool filters.
   Reject lexical-only checks, post-effect filtering, general glob or regex
   engines, blanket lowercase normalization, global file-identity exclusion
   and weaker backend fallback.
9. **Acceptance and validation.** Prove native Windows and Linux positive
   and negative cases for component patterns, forbidden syntax, omitted
   versus empty and replacement lists, and immutable bootstrap. Cover full
   grants, allowed siblings, listing omission and direct denial, missing
   paths and creation, overlapping roots and IDs, native aliases and case
   modes, rejected symlink targets, allowed ordinary hardlink aliases with
   shared reads and writes, denied BL-374 hardlink aliases, deterministic
   substitution and unsupported bindings. Exercise source, target and
   recursive effects. Prove denial before side effects,
   bounded scans and redacted results through domain and MCP paths. Cover
   staged, deferred and provider paths when present and require each later
   owner to test its own paths before activation. Apply the relevant
   [quality gates](../testing.md#required-quality-checks), native race and
   coverage evidence, and independent terminal review.
10. **Completion state.** Migrate all current filesystem operations to the
    same enforceable exclusion boundary before enabling the setting. Publish
    the resolved configuration, behavior, security limits, compatibility and
    test guidance, and reconcile changelog/version, migration, tests/CI,
    candidate, release/distribution and security dispositions. Require reviewed
    integration and main readback before closing the implementation. Any
    deferred BL-375 deliverable must be accepted by one concrete successor
    and recorded in both backlog items. Completed foundations remain closed.

Prove effect binding or fail closed at each affected native and delegated
boundary. FlashGate checks every applicable path before its own effects and
rejects detectable drift. Independent OS actors continue to operate under
native namespace and handle semantics. BL-107 and BL-374 retain their
existing bindings and residuals. BL-375 adds neither snapshot isolation nor
content-version checks.

## BL-376 explicit cross-root verified file copy

BL-376 is the release-unassigned owner for the directed cross-root policy and
its first consumer, verified single-file copy through `copy_path`. The current
registry rejects distinct root IDs for copy and move. The planned variant
preserves same-root behavior and adds no public tool, second copy engine, or
release prerequisite.

### Prerequisites and retrofit

The hard predecessors are the immutable typed configuration and named-root
operand contracts (BL-233/101/102/104), the root-specific controls
(BL-105/106/107/108/374/375), caller context and central authorization
(BL-236/159), and the shared content fingerprint, staging, precondition, and
conflict contracts (BL-048/052/053/059). Reuse their primitives under the
existing registry, PathGuard, and `internal/fs` native effect coordination.
BL-161 provides integrated policy validation and consumes the active variant
only when it is available.

Retrofit the typed configuration loader and immutable registry snapshot,
the `copy_path` dispatch and result declarations, filesystem copy and native
effect paths, and the corresponding configuration, security, tool, migration,
and test guidance. Define the validated configuration fields and the bounded
success, conflict, failure, and uncertain-outcome contracts before enabling
the variant. Use the existing exact root IDs and relative operands and the
central typed MCP result/error adapter. The filesystem core stays independent
of MCP and the policy supplies no new capability ID or protocol revision.

### Authorization and native effects

- The authoritative server configuration must explicitly allow the operation
  for the exact ordered pair of distinct root IDs. Omission denies. Reject
  invalid rules before publishing the immutable configuration. Granting one
  direction, operation, or pair confers no access to another. Root access,
  tool visibility, and native OS permissions confer no cross-root grant.
- Before any content I/O or staging, authorize the caller, effective profile,
  risk ceiling, effective backend, and every declared functional requirement.
  Require `files.read` under the exact source root and `files.write` under
  the exact target root, each with its own profile and root checks. Permissions
  do not carry across roots, including overlapping or aliased roots. Recheck
  the applicable policy and native bindings before each observable effect.
- Confine each path through the existing lexical and effective PathGuard,
  exact root, parent, and object bindings, shared effect coordination, and
  native alias, reparse, link, and mount rules. Preserve file-type controls,
  BL-374 configuration protection, and all applicable BL-375 exclusions for
  operands, staging, target publication, readback, and cleanup. Reject
  unsupported objects, effective self-copies, known aliases of protected
  objects, unproved bindings, and detectable drift. Separate source and
  target volumes may be supported with confinement within each bound root.
- Stream a single regular file through bounded Go buffers. Preserve source
  bytes and namespace and send no file content over MCP. Account for source
  and target resource limits, including file size, aggregate scanned and
  written bytes, readback, staging space, buffers, handles, and elapsed time.
  Enforce the relevant limits through success, cancellation, and cleanup.
  Copy grants confer no permission to hydrate cloud placeholders, switch to
  a user host, or synchronize with a provider. Each has a separate policy,
  support check, and validation contract.
- Stage within the bound target filesystem, verify the staged bytes, and
  publish with a native no-clobber operation that binds the destination and
  preconditions at the effect. Create-only is the safe default for this
  variant. Overwrite requires a later, separately proved contract. Do not
  fall back to an unconfined path or weaker publication primitive. Cleanup
  may touch only staging objects whose identity and ownership are proved.

### Verification and acceptance

- Compute SHA-256 and count the bytes streamed. Prove source stability
  separately with native source identity and change checks around the stream
  and before publication. Compare the digest and byte count with an independent
  read from the bound published destination and verify its identity. Internal
  verification is confined to the operation's own bytes and grants no general
  target read or equality oracle. An unproved check cannot support a verified
  result. Keep receipts, errors, and audit events bounded and redact host paths
  and native error details.
- Define bounded, distinguishable outcomes for verified publication,
  confirmed nonpublication, known partial completion or failed verification,
  and `UNKNOWN` when actual effect, identity, or cleanup cannot be proved.
  An exhausted verification budget after publication cannot report success.
  Specify cancellation boundaries, conflict and cleanup behavior, residual
  state, and safe reconciliation after a possible effect. No blind retry or
  deletion of an unverified object. Preserve the source in every outcome.
  Verification establishes the observed copy outcome under the supported
  native checks. It supplies no snapshot or crash-durability guarantee.
- On native Windows and Linux, test allowed and denied root pairs and their
  reverse directions, partial operand rights, caller/profile/risk denials,
  alias and protected-configuration cases, exclusions, same-root regressions,
  supported cross-volume copies, unsupported paths, resource limits,
  concurrent source/target changes, verification mismatch, cancellation,
  conflicts, safe publication, and partial or uncertain outcomes. Cover
  the actual filesystem and MCP routes, including failure and cleanup.
- Extend BL-205/214/215 for the changed schema, result variants, negative
  authorization and error fixtures, and response, wire, and catalog budgets
  before enabling the variant. Reuse existing coverage, conformance, source,
  native race, and CI gates. Bind the actual Windows/Linux evidence to the
  candidate and obtain independent terminal review before closure.

BL-058 owns cross-volume move and the separate directed permission and
source deletion for its cross-root variant. BL-060 owns directory traversal
and directory copy/move and reuses the verified copy building blocks for
its cross-root variants. BL-161 owns integrated positive and negative policy
tests. Existing release-bound acceptance stays independent of the optional
cross-root variants. At implementation closure, reconcile documentation,
changelog/version, tests/CI, candidate evidence, release/distribution,
migration, and security. Any deferred delivery needs a two-sided binding to
its concrete successor under the shared [closure contract](#closure).

## BL-220 adapter acceptance binding

Every new or changed consumer inherits the [BL-105 resource contract](#bl-105-resource-limits).
Declare applicable dimensions and units, root and aggregate budgets, the exact
hard enforcement boundary, result serialization and temporary-data lifecycle.
Missing enforceability denies dispatch. Test exhaustion and no-effect denials
on the actual backend before activation; a configured limit is not proof.

For every new tool, platform or Native Tool adapter, execution backend,
process-launch path, host, transport or provider, name the first owning BL
and classify the affected code under the [adapter selection
policy](tool-adapters.md#bl-220-adapter-selection). Bind the applicable
[successor acceptance](tool-adapters.md#bl-220-successor-acceptance) directly
from the canonical backlog row and include the owner in that family's
applicability declaration. Record the selected native path, alternatives
considered, fallback or unsupported result, security and resource bounds,
reuse and retrofit targets, and any required later activation decision in
the existing planning and traceability sources. Public tools also update
the [public inventory](tool-adapters.md#20-public-tool-inventory-by-target)
with the required family for the surface. The doccheck gate compares the
inventory, family applicability and canonical backlog acceptance.

Before activation, exercise authorization before dispatch, backend and
platform parity, the affected negative and unsupported cases, enforcement
strength, cancellation and cleanup, and operation-specific efficiency
budgets. The static source baseline is reusable for unchanged paths. Actual
new call sites, behavior, deployment modes and binaries need owner-specific
runtime evidence. Update the [source gate](../testing.md#native-runtime-source-gate)
with any new shipped entry point, build variant or reviewed centralized
launch site and include bypass fixtures. A separate approved security and
architecture decision remains required for a provider, dependency or runtime
model outside the existing contract.

BL-220 supplies the policy, canonical successor bindings and source gate.
BL-163 owns executable runtime policy. BL-134 owns the platform components.
BL-137/138/144/146/147 own their identity, argument, environment and isolation
contracts. BL-151 owns productive integration and the central launch-site
gate under the [staged activation contract](#managed-process-activation).
BL-152 owns runtime execution negatives. BL-242 owns artifact/mode verification. The
operation owners implement and test their own adapters without reopening BL-220.

### Managed Process activation

SPR-056 delivers the Managed Process registry, identity, typed interfaces,
bounded output and lifecycle contracts, and reusable CI for the non-launching
paths. BL-129 and the earlier Operations owners retain their contracts and
the evidence they can prove independently of productive children. Use
injected outcomes and safe isolated platform or observation tests. Keep
`start_process` and unsupported managed-child routes out of the effective
catalog. Reject direct calls and leave raw or alternate process-launch
paths blocked. Earlier owner closure requires proof of its revised scope
and the two-sided launch transfer in BACKLOG.md.

In SPR-057, establish BL-163 policy and the applicable typed, identity, root,
environment, limit, redaction and platform predecessors in the
[architecture traceability](architecture-traceability.md#domains-runtime-and-release-validation)
table. BL-151 then integrates the BL-122 request/dispatch foundation, the
BL-134 platform components and the shared registry and lifecycle into one
reviewable native launcher. Build and test the candidate with its public
routes unavailable. Any source-gate exception must bind the exact reviewed
central launch path. All other launch paths remain blocked.

Before activation, BL-151 owns the native Windows/Linux execution and
denial, lifecycle, restart, race, resource and cleanup evidence transferred
by the earlier owners. Reuse the BL-135 tests, BL-252 race commands and
BL-253/254 CI harnesses. Run the real managed-child and Operations scenarios
in them without a separate launcher or duplicate CI harness. BL-152 and
BL-168 retain ownership of security negatives and least-privilege evidence
respectively. They may develop fixtures independently and run native
scenarios against the unadvertised BL-151 candidate. Their passing evidence
is required before productive activation. This evidence gate is separate
from the hard prerequisites for starting BL-151 implementation.

BL-151 enables the reviewed routes only when policy is effective before
every dispatch and native enforcement meets the requested strength. Reuse
the same boundary for BL-143 and applicable Native Tool adapters. The
synchronous wrapper and its result schema are separate from the prerequisites
for managed start. Later discovery and deployment owners expose only verified,
authorized support under their own acceptance.

## BL-204 conformance binding

BL-204 owns test and validation code at the MCP adapter and STDIO boundary.
It reuses completed BL-200/201/202 output schemas, result contracts and explicit
annotations, BL-207/208 revision policy and dispatch, the runtime registry,
native filesystem guards and existing wire harnesses. The current controls in
`cmd/server`, `internal/mcp`, `internal/protocol` and `internal/mcptest` remain
authoritative; this task adds no transport, runtime dependency or product tool.

Acceptance covers native contracts and deterministic complete snapshots for
`2025-11-25` and the test-policy-only `2026-07-28` candidate, each under default
and read-only profiles. Registry, profile, capability and revision inventories
must have explicit executable coverage; mutations must prove drift detection.
The official suite is evaluated at a pinned upstream commit, with scored and
not-scored server scenarios mapped individually and execution limits stated.
The native validation report is reproducible from the focused tests and their
JSON output. Snapshot updates require an explicit test flag and review.

The public protocol matrix remains unchanged. BL-212 retains full JSON Schema
2020-12 validation; BL-219 retains fingerprints/cache implementation; BL-257
retains dedicated CI drift enforcement. BL-369 may consume this task's focused
report later; BL-370 retains HTTP transport. Neither is implemented here.
No release, distribution or migration behavior changes; test-only work leaves
`VERSION` unchanged. Independent terminal technical review remains required.

## BL-212 schema conformance binding

BL-212 extends the MCP adapter's test and validation boundary. It reuses
BL-200/201 schemas and success results, BL-204 registration, revision/profile
coverage, snapshots and report, and BL-289/291 strict arguments and catalog
parity. The BL-262 inventory records the pinned validator and its transitive
module as test dependencies while preserving verification of earlier bundles.

The existing gate compiles registered input and output schemas as Draft
2020-12 with offline reference resolution, validates successful wire instances,
and tests schema, fixture, catalog and snapshot failures. Tool owners supply
functional cases. BL-110 extends the shared matrix when effective profiles are
implemented, and BL-257 owns dedicated CI drift enforcement. This change adds
no wire fields, runtime checks, product dependencies, activation, or migration.
Independent terminal review is required before closure.

<a id="bl-371-mcp-ping-receiver-for-2025-11-25"></a>

## MCP `ping` receiver for `2025-11-25`

Backlog applicability: BL-371.

**Role and authority.** BL-371 is a revision-specific MCP-adapter implementation
owner in SPR-048, separate from the BL-204 conformance/snapshot candidate.
Use the official `2025-11-25` ping utility and request/schema specification,
ADR-013 and the final `2026-07-28` changelog removing `ping`. Completed
BL-207/208 are the hard predecessors; BL-204 gap evidence is coordination,
not an implementation or hard-prerequisite owner. No BL-257 hard edge is added.

**Reuse and retrofit.** Extend the existing Go `internal/mcp/router`,
`internal/mcp/server`, request validation and exact `internal/mcp/revision`
policy. The accepted design registers a dedicated stateless `ping` handler
on the initialization-based `2025-11-25` path and returns JSON-RPC `result: {}`
with the unchanged request ID. The server owns parameter validation, and the
handler has no tool registry or domain dependency.
Leave `2026-07-28` disabled in production and keep valid enabled-revision
`ping` calls rejected as method-not-found (`-32601`). Do not add any
`tools/list` entry, network/ICMP probe, automatic heartbeat, external MCP SDK,
new dependency or protocol matrix entry. Record the receiver under Unreleased
and keep `VERSION` at the current release version. The
[BL-372 binding](#bl-372-release-oriented-product-versioning) governs later
release preparation and verification.

**Safety, tests and closure.** Honor the real revision request-params contract,
including valid optional metadata; reject malformed inputs through the existing
JSON-RPC boundary without leaking internal errors. Preserve the existing
limits, cancellation, notification no-response, diagnostics redaction and
stdout purity. Exercise actual STDIO positive and negative wire cases, IDs,
invalid params, notifications, unknown methods, ordinary post-initialize
behavior and injected `2026-07-28` revision isolation. Complete applicable
Windows/native Linux, CI/coverage, documentation and independent terminal
review gates. Record the support decision and actual result in BL-371; do not
claim full official conformance or change BL-204's already reviewed bytes.

<a id="bl-257-ci-enforcement-binding"></a>

## MCP contract enforcement in CI

Backlog applicability: BL-257.

BL-257 owns the blocking test and release-quality gate in SPR-048. It
consumes the completed BL-204 and BL-212 contracts through the existing
`cmd/mcpcheck` runner, server registry, catalog, and conformance matrix.
Retrofit the Windows and Linux CI test job and its workflow policy tests.
Keep the broad test and coverage jobs in place for their separate guarantees.

Require executed snapshot evidence for every declared revision/profile
combination, complete tool success and denial cases, schema and catalog
parity, and no missing, failed, or skipped required evidence. The feature
owner extends these contracts for future revisions, profiles, capabilities,
tools, and parameters while the CI command stays stable. The gate tests
implemented contracts. Coverage of specification requirements still needs
owner assertions and independent review of intentional contract diffs.

Acceptance requires positive and negative tests, both platform quality
chains, and independent terminal review before Git integration. Snapshot
updates remain explicit. This gate makes no product, activation, version,
migration, or security-policy change and produces only temporary CI evidence.

## BL-213 tool result contract

BL-213 owns the shared tool result contract at the MCP adapter boundary. It
reuses BL-201 result and error envelopes, BL-204 exact-revision wire coverage,
BL-212 schema and catalog parity, and BL-257's existing CI gate. The security
model, [efficiency ADR](../adr/efficiency-contracts.md), and
[protocol guide](../protocol.md#tool-result-classes) govern the public contract.

Each tool declares its result variants and returns typed results to the central
adapter. Payload kind, transfer mode, and result state are independent. Inline
results can combine adapter-derived metadata with ordered payload parts.
Registration rejects missing or unsupported declarations. The adapter accepts
closed object shapes and checks their encoded sizes before serialization.
Payload-bearing results derive their size metadata from the parts. The adapter
rejects malformed UTF-8 text parts before serialization. File text must
round-trip byte for byte. Directory scans count filtered entries toward the
configured entry limit and fail without a partial result. Small metadata can
retain compatibility parity within its budget. File text and directory entries each
have one primary wire location.

Validate all registered variants on the default and read-only paths for each
executable revision. Cover single transmission, malformed contracts, limits,
schema instances, catalog parity, normalized errors, and intentional snapshot
changes through the existing native CI gate. Keep client migration guidance and
Unreleased notes with the wire delta. Require independent review before closure.
The successor owners retain paging, resource delivery, notifications, jobs,
streaming, and efficiency budgets.

## BL-214 wire measurement and coverage

BL-214 is the SPR-048 test and benchmark contract for the MCP adapter and
transport boundary. It builds on BL-189–BL-199 benchmark infrastructure,
BL-213 typed variants and single transmission, and the BL-204/212/257
registry, revision, profile, schema and wire tests. Use the existing server
test harness to capture actual frames and compare their decoded content with
independent fixture data. Measure explicit conversion and serialization sites
through a request-scoped observer used by tests. Production passes no observer.

The ordinary Go suite matches each executable registry variant to a semantic
case and each measured fixture to a hard budget for its actual revision and
profile. The benchmark package measures ordered payload locations and aggregates
the frames of a logical delivery. A producer can add fixture references,
placement and budget bindings without changing the measurement algorithm.
Retain the historical benchmark and allocation contracts. Synthetic cases
exercise future delivery shapes without enabling registration, routes or
capabilities. Preserve schema, redaction, limits, profile enforcement and the
single transmitted representation for heavy payloads.

The [successor matrix](architecture-traceability.md#result-delivery-successors)
binds later producers, profile and catalog owners, transports and CI consumers
to their existing milestones. A non-tool owner must identify its actual route,
prove the negotiated profile/revision/transport and feature availability, and
exercise real output through the common measurement and budget gate. Closure
requires executable coverage mutations, unchanged historical artifact semantics,
applicable native gates and independent review before integration.

## BL-205 response-size regressions

Resource-producing owners reuse [BL-105 limits](#bl-105-resource-limits),
including root success-result bounds and the independently framed global
response bound. Add exact/one-byte-over and escaping cases before activation.

BL-205 extends the existing MCP adapter validation for STDIO under
[ADR-003](../adr/stdio-transport.md), [ADR-012](../adr/efficiency-contracts.md)
and [ADR-013](../adr/mcp-compatibility.md). Reuse the BL-203
[error contract](../tools.md#machine-readable-tool-errors), BL-213 result
variants, BL-214 measurement and budgets, BL-215 catalog budgets and the
BL-204/212/257 conformance and CI inventory. Retrofit the test harness,
semantic fixtures and existing budget bindings. The public tool, protocol,
configuration and runtime contracts stay byte-compatible.

Capture the actual UTF-8 response including its JSON-RPC envelope and
transport framing. Validate frame count and boundaries before decoding.
Compare decoded semantics, payload placement and redaction independently of
exact deterministic size. Document any justified minimal tolerance and keep
the hard wire ceilings and runtime safety limits separate. Reject missing or
stale fixtures and budgets through the shared coverage gate. Preserve
single transmission and explicit authorization checks.

Every owning BL applies the [operation and delivery acceptance gate](tool-adapters.md#22-implementation-and-activation-gate-for-productive-routes)
to its executable revisions, effective profiles, platforms, capabilities and
transports before activation. Bind applicable errors and explicit exclusions
to the actual operation. Unresolved behavior and reserved categories remain
with their implementing owners. Extend the existing registry, conformance
and wire tests and the same budget authority. BL-205 does not add a parallel
inventory, measurement engine or production dispatch path.

Acceptance requires positive, negative and boundary tests and mutations for
framing, bytes, semantics, registration, result variants, applicability and
budget parity, followed by the applicable native quality chain and independent
review. This contract remains in force after BL-205 closes.

<a id="bl-215-catalog-and-entry-budget-binding"></a>

## Catalog and entry budgets

Backlog applicability: BL-215.

New tools and adapters declare their applicable [resource dimensions](#bl-105-resource-limits)
in the existing contract and prove limits under the same executable revision,
profile and delivery fixtures. BL-105 remains closed when later consumers add
their own operation dimensions or lifecycle integrations.

BL-215 is the SPR-048 MCP adapter and validation contract for bounded effective
catalogs and revision-specific entry. Reuse the BL-189–BL-199 benchmark
references, BL-207/208 exact revisions, BL-204/212/257 registry and schema
conformance, BL-213 result contracts, and the available BL-214 wire harness
and fixtures. The SPR-048 order in [BACKLOG.md](../../BACKLOG.md) allows BL-215
to proceed independently of BL-214's formal closure. Retrofit the existing wire
gate and [tool planning inventory](tool-adapters.md#20-public-tool-inventory-by-target)
in place. The public runtime contract stays byte-compatible. The budget gate
consumes the server-authorized effective registry and does not confer visibility
or execution rights.

Before advertisement or activation, the responsible owner must:

1. Bind the canonical name, milestone, exposure prerequisites and budget class
   in the [tool and adapter plan](tool-adapters.md) for each affected public
   family. Enumerate supported effective compositions, including platform and
   policy differences that change output.
2. Measure actual catalog and entry responses for every affected executable
   exact revision, effective profile and transport. Extend the existing
   conformance snapshots, schema and registry parity, wire fixture and budget
   bindings. Include entry instructions when that revision supports them. For
   every new tool or result variant, preserve the separate BL-214 call budgets.
3. Apply the [effective limits and composition rules](efficiency.md#tool-catalog-and-initialization-budgets)
   to the full catalog. Record field, schema, instruction, frame, JSON-RPC and
   revision costs. Test missing or stale bindings, oversize output, unsupported
   compositions and capability denials as applicable. Keep planned fixtures
   separate from executable production and candidate-policy paths.
4. Show that all activated combinations fit. If they do not, reduce safe
   representational overhead or narrow the authorized composition under the
   owning contract. A change to an accepted ceiling requires an explicit
   decision backed by measurements and compatibility evidence.

New functions consume the accepted BL-215 contract and extend their fixtures,
budgets and tests under their own backlog owner. This does not reopen BL-215.
BL-103/110 own effective composition, BL-216 owns instructions, BL-219 owns
fingerprints and cache behavior, and BL-256 owns comprehensive CI and runtime
enforcement. These are successor consumers, not prerequisites for the current
executable budget gate.

## BL-372 release-oriented product versioning

BL-372 owns release governance and test controls in SPR-048. It reuses the
BL-245 VERSION, changelog and tag contract, BL-248 artifact verification,
BL-255 candidate and promotion identity, BL-262 supply-chain evidence, and
BL-363/364 publication and distribution controls.

Retrofit both build scripts, the candidate and tagged build workflows and
their releaseaudit gates. Ordinary development builds use `0.0.0-dev` with
existing provenance. Notable changes accumulate in Unreleased. A separately
authorized freeze changes VERSION and adds dated, nonempty notes on the
exact candidate commit. A PATCH from an older release uses an isolated
maintenance line when main contains new public features.

Acceptance covers consecutive development changes, MINOR and PATCH freezes,
malformed notes, development artifact rejection, and published identity
reuse, with Windows and native Linux validation and independent review.
VERSION remains 0.3.0. Publication, tag creation and provider writes require
separate authorization. The published v0.3.0 identity is retained.

## Inherited contract families

| Family and BL binding | Required reuse, target boundary, and retrofit |
|---|---|
| MCP, protocol, and results — BL-090, BL-201, BL-204, BL-207–BL-219, BL-257–BL-258, BL-371 | Keep domain results protocol-neutral. BL-090 owns one bounded result lifecycle; BL-218 adapts it to negotiated MCP resource handoff. Preserve exact revision negotiation and conformance, JSON Schema 2020-12, fingerprint/cache isolation, and payload single-transmission rules. Retrofit existing `read_file` and other heavy responses; do not advertise a revision before its gates pass. |
| Policy, execution context, named roots, configuration, and audit — BL-101–BL-111, BL-159–BL-166, BL-233, BL-236, BL-239, BL-352, BL-373 | Build one immutable request/execution context, one server-side authorization decision before dispatch, one configuration model, and one correlation lifecycle. Reuse named roots, effective profiles and current path controls in the typed TOML model. Migrate direct MCP-to-filesystem dispatch. BL-101–BL-110 and future domains extend and consume the central model for their own fields and acceptance. Bind handles, cursors, caches, and resources to the effective context. Enforce authorization at dispatch. |
| Capability and authorization verification — BL-100, BL-171 | Use the [BL-100 functional model](#bl-100-functional-capability-model) for explicit functional requirements. BL-171 verifies that MCP annotations confer no grants through the central BL-159 decision. |
| Filesystem — BL-036–BL-067, BL-346 | Extend `internal/fs` and the existing `internal/security` path guard. Use named root IDs and relative paths, bounded reads/writes/trees, existing hash and tree primitives, and the shared result lifecycle. Migrate the current eight-tool contracts where their owners require it. |
| Search — BL-068–BL-082 | Extend the accepted `search_paths` and `search_text` public families over confined roots, bounded scanning, pages, and context-bound cursors. Pure Go is the portable baseline. Do not add a public tool per backlog row. |
| Operations and jobs — BL-084–BL-099, BL-164 | Own one generic bounded lifecycle, quotas, cancellation, TTL, cleanup, and fair scheduling. Consume policy and execution context before creating state. Keep domain meaning in domains; reuse BL-090 storage. |
| Process — BL-113–BL-135, BL-162, BL-165, BL-252–BL-254 | Reuse execution context and Operations for owned handles, bounded output, cancellation, and cleanup. External PID control is separate Post-1.0 scope. Do not use a PID alone as authority. |
| Typed execution and Native Tool adapters — BL-136–BL-152, BL-167–BL-170, BL-220, BL-353 | Resolve server-approved command IDs to executables and typed argv, with limits, redaction, and OS isolation. Native Tool adapters are optional, policy-constrained accelerators, never public aliases or interpreter/raw-shell passthrough. |
| System information — BL-062, BL-153–BL-158 | Expose only scoped, explicitly released facts through domain and OS adapters; filter environment and identity data, bound results, and retain Post-1.0 network/administrative decisions outside Version 1.0. |
| Runtime and service modes — BL-221–BL-232, BL-234–BL-238, BL-240–BL-244, BL-341, BL-370 | Keep one core behind direct STDIO and local proxy/service transports. Consume BL-233's validated configuration per runtime role. BL-236 owns backend-neutral dispatch. Variant A service account is the Version 1.0 backend; unsupported user-worker selection fails closed. Preserve host ownership and stdout purity. |
| Release, artifacts, and validation — BL-203–BL-204, BL-215–BL-216, BL-241–BL-263, BL-305–BL-315, BL-363, BL-364, BL-366, BL-367, BL-369 | Reuse root `VERSION`, BL-245 changelog/tag semantics, BL-248 verification and current CI gates. BL-255 builds/verifies candidates, then promotes the exact bytes locally with immutable identity and no additional remote publication. BL-262 supplies evidence/rollback controls. BL-363 consumes verified candidates and evidence for explicit persistent public pre-1.0 GitHub prereleases with `prerelease=true`, exact version/source/artifact identity, no publication-time rebuild or silent asset replacement, minimal separate release-write permissions, and remote readback. Finalize release-relevant source and dated changelog notes before public candidate freeze. BL-263 separately requires stable Version 1.0 traceability/gate closure and reuses the publication foundation; prerelease success never passes that gate. Retrofit the tag-gated build/validation path, transient Actions-artifact handoff and public download guidance. BL-364 owns the accepted five-asset distribution target for all releases including v0.3.0: four original archives and shared SHA256SUMS, preserving complete verification and required temporary evidence within regular Actions retention. Release-unassigned BL-366 owns SBOM security management; release-unassigned BL-367 owns GitHub-native signed attestations. Neither blocks a release or is implicitly implemented here; see the task bindings below. |
| Response and benchmark quality — BL-172–BL-173, BL-177–BL-179, BL-205, BL-317–BL-323, BL-325–BL-329, BL-331–BL-332 | Reuse current benchmark schemas, deterministic workflow/corpus controls, host/platform evidence, and documentation gates. Strengthen checks at their existing owners without weakening result, supply-chain, platform, or public documentation truth; no benchmark result itself authorizes product behavior. |
| Accepted Post-1.0 capabilities — Post-1.0 workstream BLs in BACKLOG.md | Inherit the same context, root, authorization, domain, state, result, adapter, and validation contracts. A later milestone does not relax them or imply current implementation. Detailed names and adapter boundaries live in [tool adapters](tool-adapters.md). |
| Documentation navigation tooling — BL-368 | Independent release-unassigned development tooling, outside the product runtime and release-security owners. Reuse completed documentation validation and extend its local/read-only CI consumers under the [BL-368 binding](#bl-368-documentation-navigation); no hard dependency on BL-365 or release obligation. |

<a id="bl-100-functional-capability-model"></a>

## BL-110 effective tool catalog

**Role and prerequisites.** Apply ADR-009 at the MCP composition boundary.
Reuse the BL-100 capability catalog, BL-103 immutable effective profile,
BL-108 ordered resource requirements, BL-236 trusted request and backend
binding, and BL-159 validated operation declarations. Reuse the current
configuration, native guards, result contracts, and BL-205/212/214/215/220
activation evidence under their existing owners.

**Retrofit.** Bind all eight filesystem tools and all twelve execution
variants in the server bootstrap. `internal/fsapp` supplies references to the
same validated declarations that `internal/execution` enforces. The MCP
registry owns the immutable public definitions and their trusted handlers.
Bootstrap/profile admission, discovery and direct calls consume effective
views of that catalog. Typed domain dispatchers may share the same validated
declaration set while retaining their own command and result types. Domain
and execution packages remain transport-neutral.

**Effective view.** The trusted ingress assigns the authenticated principal
to an immutable effective policy for its runtime. One composition can bind
several typed domain dispatchers with that policy. Each route retains its
resource resolver, selected backend and confirmed operation support. The
tool invocation carries the exact route and admitted operations to dispatch.
For each declared variant, apply the complete functional
requirement and derived risk before support. Validate the exact composition,
including its platform and protocol representation, against activation
evidence. Derive each request's view from its assigned composition, including
for a principal authenticated after activation. A tool can expose its
authorized and supported variants under the bound representation. BL-159,
native policy and confinement, configuration protection, pre-effect result
admission and bound release govern concrete operands. Invalid and stale
bindings fail closed. Direct calls need no prior discovery.

**Public contracts.** Preserve current tool names, order, schemas, results,
generic unresolved-tool errors and the exact production revision. Preserve
the three-tool read and eight-tool write views for their measured profiles.
Deduplicate references to the same canonical registration and reject distinct
definitions sharing a public name. Catalog readers cannot alter definitions
or replace handlers. No root-grant aggregation, global profile switch, second
authorization table, dynamic loader, cache activation or new protocol grant.

**Validation and closure.** Prove the actual bootstrap, discovery, direct
invocation and dispatcher paths on Windows and native Linux. Bind each
exposed operation key and observed result variant to its executable fixture,
revision-specific schema and exact budget. Cover stale or misassigned policy,
partial grants, missing support, a late principal, independent native
denials, conflicting definitions, concurrency and isolated synthetic future
extensions. Complete the existing
candidate, coverage, wire, security and independent review gates. Record
documentation, version, migration, security, CI, release and candidate
dispositions in the BL-110 closure. Confirm completed status on `main`.

**Consumer acceptance.** Every new or materially changed technical owner
states its own BL-110 disposition in the backlog. For applicable work, name
the concrete declaration, exposure, activation, invocation or evidence it
adds. Explain non-applicability for independent work. BL-111 owns the wider
negative catalog matrix, BL-216 uses the effective view for instructions,
BL-219 owns fingerprints and caching, BL-239 owns retained state, and BL-256
owns the expanded CI policy. New tool and adapter families extend the common
contract without reopening the foundation.

## Functional capability model

Backlog applicability: BL-100.

The transport-neutral policy foundation implements the
[ADR-009 functional capability contract](../adr/capability-profiles.md#functional-capability-contract)
in `internal/capability`. BL-233 is the completed configuration baseline.
Capability definitions are reviewed data. A catalog snapshot validates
definitions, grants and requirements before use. The evaluator checks all
explicitly required IDs in that snapshot and takes no transport, backend,
profile, principal or resource inputs.

Use the common catalog and evaluator for future functional rights. The
implementing owner declares each new operation or variant and any necessary
new definitions, exercises granted and denied calls including partial grants
for composed operations, and binds executable coverage to the existing
authorization, catalog and CI gates. Backend selection and provider trust
remain independent. See [backlog ownership](../../BACKLOG.md) for the concrete
requirements of each successor and the
[traceability matrix](architecture-traceability.md#core-policy-context-catalog-and-state-foundation)
for integration boundaries.

BL-100 originally preserved the `files.read_only` setting and server
bootstrap. BL-103 replaces that setting and binds the current registry to
explicit grants. BL-108 applies the same capability catalog to per-root
access, BL-110 owns effective
registration, and BL-159 owns functional enforcement alongside all other
checks before dispatch. BL-236 carries the inputs across execution backends,
and BL-239 applies the effective context to state and handles. BL-100 adds
the internal model and focused fixtures. The current configuration, registry,
tool contracts, protocol, dependencies, version and distribution stay as
implemented.

Validate construction failures, deterministic set operations, all-rights
matching, concurrent read-only use, declarative extension and backend parity
with synthetic inputs. Run the existing Windows and native Linux product
coverage, STDIO, source and documentation gates. BL-111, BL-160 and BL-161
own runtime visibility, direct-call and root negatives. BL-256 owns
effective-composition CI. Independent terminal review is required before
closing BL-100.

<a id="bl-103-capability-profiles-and-risk-policy"></a>

## Capability profiles and risk policy

Backlog applicability: BL-103.

**Role and prerequisites.** Apply [ADR-009](../adr/capability-profiles.md)
using the BL-100 catalog and all-rights evaluator, BL-101/102 root binding
and operands, BL-233 typed TOML, BL-374 configuration protection, and the
BL-205/214/215 executable wire and catalog gates.

**Retrofit.** Replace `files.read_only` and the boolean tool selection in
`internal/config` and `cmd/server`. Use one immutable policy snapshot for the
existing STDIO registry and dispatch. Replace the retired capability IDs at
all active consumers. Preserve tool names, schemas, root and protection
checks, generic unavailable-tool errors, and protocol revision eligibility.

**Acceptance.** Validate all definitions and the explicit composition before
startup. Require both the operation's complete grants and a risk class within
the ceiling. Resolve an explicit high-risk ceiling in the generic model. Admit
only measured effective combinations for the current runtime and require all
separate operation security controls. Exercise real catalog and direct-call
parity, missing bindings, protected resources and ordinary siblings on Windows
and Linux. Later owners
declare their own operation variants, risk floors, context and negative
fallback tests, and extend the executable fixtures and budgets before
activation. BL-110/159/236/239 own the later catalog, authorization, context,
and state boundaries respectively.

## BL-101 named-root registry

**Role and prerequisites.** BL-101 owns root configuration and the bootstrap
registry. Reuse the BL-233 typed TOML model, BL-374 configuration protection,
BL-100 capability boundary and the [filesystem abstraction ADR](../adr/filesystem-abstraction.md).

**Retrofit.** Replace the previous TOML filesystem table in `internal/config`.
Build the immutable `internal/roots` registry through `cmd/server` bootstrap.
Construct each root through the current PathGuard and local filesystem
adapter. Bind the same loaded config identity, selected name and original
ancestors to every root, with per-root identity and native alias checks.
Serialize ordinary filesystem effects across the instance.

**Contract.** Require a configured default ID and up to 256 roots within the
existing 64 KiB document limit. IDs match `^[a-z][a-z0-9_]{0,63}$`, each path is
an existing absolute directory, and every root must pass validation before the
registry can be used. Reject the former table and development CWD setting.
Keep global limits and read-only exposure. Registry access grants no
functional rights. The original BL-101 tools targeted the default root.
[BL-102](#bl-102-filesystem-resource-contract) owns explicit addressing.
BL-103 owns profiles, BL-104–BL-109 per-root policy,
BL-159 authorization, BL-236 execution context, and BL-352 discovery.

**Acceptance and closure.** Exercise strict TOML decoding, isolated snapshots,
concurrent lookup, default MCP parity, native startup and overlapping-root
protection for loaded and replacement objects, aliases and ancestors. Preserve
allowed sibling and metadata access, redaction and the BL-374 external
final-window boundary. Run Windows and native Linux gates and review the full
candidate independently. Update public setup, migration and Unreleased notes
with `VERSION` unchanged. Release candidates and publication use their
existing separate owners.

<a id="bl-233-central-runtime-configuration"></a>

## Typed runtime configuration

Backlog applicability: BL-233.

**Role and prerequisites.** SPR-053 bootstrap and core configuration owner.
Implement the [ADR-014 configuration contract](../adr/runtime-modes.md#configuration-contract).
The traceability row has no hard BL predecessor. Selecting and adopting a
reviewed TOML library requires a separate dependency decision. BL-223 and the
runtime, policy, and domain owners consume the model.

**Reuse and retrofit.** Reuse existing configuration and startup code where
it fits. BL-233 may reorganize the internals when the implementation shows
a simpler design, better maintainability, and lower resource use. Preserve
PathGuard, root confinement, existing limits and read-only controls,
fail-closed startup, safe startup errors, and protocol and compatibility
guarantees. Migrate the current `MCP_*` runtime inputs, including path policy,
payload limits, and diagnostics. BL-101/103 retain their named-root and profile
semantics. BL-166 consumes the logging path and owns the audit lifecycle.
The delivered baseline checks separation after PathGuard resolves the data
root. BL-374 owns the planned replacement. Keep the historical BL-233
acceptance and the current protection until that replacement is proven.

**Public delta and invariants.** The future startup configuration contract
changes from environment inputs to one trusted TOML file per instance.
Publish the actual compatibility, conversion, and rollback behavior in
[migration](../migration.md#root-configuration) and the affected client
guidance. Keep STDIO invocation and protocol output compatible under the
new configuration contract. Apply the same source trust, immutable effective
state, rejection, redaction, and no-startup-write rules for every role.
BL-233 owns a complete, valid, documented TOML configuration template for
its supported settings. BL-223 owns explicit CLI creation through
`config init`, consumes that template, and uses the same schema and validator
for creation and any CLI validation. BL-373 owns later MCP management. This
binding defines no new MCP schema or tool and assigns no product version or
release.

**Acceptance.** Test role and identity selection, secure default and absolute
override, absent, invalid, redirected and untrusted inputs, invalid security
fields, current-policy migration, validation before initialization, immutable
consumer state, log limits/redaction, stdout purity, and absence of startup
writes on native Windows and Linux. Prove that configuration denial cannot
trigger endpoint or weaker-source fallback. Verify that the documented TOML
template covers the supported settings and passes the shared validator.
BL-241/242 validate the enabled managed-mode integrations, and BL-243
documents the delivered paths.

<a id="bl-374-configuration-trust-and-protection"></a>

## Configuration trust and protection

Backlog applicability: BL-374.

**Role and prerequisites.** SPR-053 bootstrap and filesystem retrofit. The
completed BL-233 typed model, ADR-014 role selection, PathGuard and the
existing Go filesystem are prerequisites. BL-374 can run on today's direct
STDIO path before later authorization or service integration. The
[ADR-014 amendment](../adr/runtime-modes.md#configuration-trust-and-protection-amendment)
and [security contract](../security.md#configuration-trust-and-protection)
are the authorities for the change.

**Reuse and retrofit.** Replace administrative trust checks in
`internal/config` on Windows and Linux while preserving role-bound selection
and stable, bounded reads. Reuse the strict typed decoder and immutable
accessors. Update `cmd/server` bootstrap, the current separation check,
native fixture helpers, PathGuard resolution and `internal/fs` read, create,
write, copy, move and delete. Guard `read_file` and both `copy_path`
operands. Protect the selected path, relevant parents and file identity.
Check opened objects before content access or truncation, coordinate
FlashGate operations through the effect, and revalidate effect operands for
detectable path, identity and root drift. Reject a protected copy
source before creating, truncating or modifying the destination. Ordinary
listing and stat retain authorized content-independent metadata.
Review destructive opens before truncation, rename/replacement, the complete
recursive-delete effect, implicit parent creation and temporary output or
cleanup wherever applicable. The accepted final namespace window covers only
independent adversarial OS actors where native APIs cannot make the check and
effect atomic. Preserve and report the four ext4 reproductions separately
from hard acceptance. FlashGate-initiated helpers and adapters inherit the
hard requirements. Cover a
read or copy source swapped after resolution and verify bytes cannot escape
through partial results, diagnostics or outputs. Retain
current root and capability controls. Bind a safe typed denial through
existing MCP and bootstrap error paths.

**Public delta and scope.** Keep role defaults, the absolute override, the
schema, limits, current MCP tool names and STDIO framing. Allow a root that
contains the selected source only with proved protection. Keep no-startup-write,
secret-safe diagnostics and empty stdout on startup failure. Use the existing
Go enforcement points. BL-374 adds no configuration-management API,
process isolation engine, runtime dependency or tool.

**Acceptance and closure.** Run the native Windows/Linux
[acceptance matrix](../testing.md#configuration-protection-acceptance) and
regressions for the affected loader, bootstrap, filesystem and MCP surfaces.
Prove the normal unprivileged test path and explain any unavailable native
fixture or operation. Mark the owning backlog row according to actual
implementation and independent review. Reconcile affected ADRs, guides,
tests and CI, migration and rollback, security, changelog and version
disposition, current-version candidate artifacts and release/distribution.
Bind any accepted deferred deliverable in both this owner and its accepting
successor.

**Successor rule.** All new or changed tools and adapters screen their
actual side effects under [BL-220 successor acceptance](tool-adapters.md#bl-220-successor-acceptance).
Each relevant owner binds denial and positive evidence to its actual dispatch,
read, mutation and disclosure paths in
[traceability](architecture-traceability.md#configuration-protection-consumers).
Cover content-derived results, selected fields, continuations and retained
payloads wherever they can expose protected content.
Keep BL-220 and completed BL-374 closed for ordinary extensions. A change
to the shared protection boundary requires an explicit security decision
and an identified owner. BL-373 owns authorized configuration changes.

<a id="bl-373-authorized-mcp-configuration-management"></a>

## Authorized MCP configuration management

Backlog applicability: BL-373.

**Role and prerequisites.** Post-1.0 application and authorization owner.
Implementation requires BL-233 (typed model and validation), BL-374
(protected source binding), BL-236 (trusted
caller and execution context), BL-159 (central authorization), BL-110
(effective catalog), and BL-166 (audit and correlation). Reuse named roots,
profiles, capabilities, and identity bindings from those foundations.
BL-370 is a later transport consumer. Each supported transport must meet
its own activation conditions before it exposes management.

**Reuse and retrofit.** Extend BL-233's model and effective-state access
with one transport-neutral management function in the shared Go core.
Reuse BL-105's exact `internal/resource.ParseSize` and canonical `FormatSize`
for size input and display; do not create a second locale or unit authority.
Route enabled direct, proxy, and service paths to it through the existing
dispatch, authorization, effective catalog, and audit contracts. The service
keeps its own configuration authority. A compatible MCP Apps client may
provide a graphical interface through the same management API and checks.
Any later Streamable HTTP exposure uses the same core functions after
BL-370's authentication, security, and activation gates. There is no second
configuration engine or client-side mutation path.

**Public delta and invariants.** The dedicated, server-authorized management
route owns bounded, redacted views of effective configuration. Ordinary
read or copy operations cannot provide a raw-source view. Provide typed
change preparation, validation, and a readable
preview before any mutation. Require explicit server-side authorization for
each step, including current authorization at commit time. Protect secrets,
sensitive paths, and administrative policy from exposure or unauthorized
change. Only an explicitly authorized effective tool catalog can expose
management. Discovery, transport availability, and client or agent
confirmation confer no permission. The implementation owner decides the
public tool names, schemas, and errors before activation.

**Acceptance.** Define version/concurrency conflicts, compare-and-swap (CAS),
safe persistence and postwrite readback, failure and recovery behavior,
and when restart makes a persisted change effective. Atomically publish
one fully validated new snapshot with its corresponding protection binding
under the existing authorization and generation controls. Where persistence
is authorized, bind the actual target and validate the bytes and filesystem
permissions before publication, with CAS, readback and defined recovery.
Define temporary versus permanent changes during BL-373 design. Prove native Windows/Linux behavior with
positive, security, negative, consistency, and integration tests across the
authorized paths. Cover stale or competing proposals, denied catalog access
and direct calls, authorization changes before commit, redaction, persistence
failures, and restart. Verify optional UI and later transport exposure
against the same server-side policy before enabling them.

<a id="bl-369-human-readable-release-quality-reporting"></a>

## Human-readable release quality reporting

Backlog applicability: BL-369.

**Architecture role and prerequisites.** Independent release-unassigned
presentation/aggregation owner; reuse completed BL-248/255/262 verification and
canonical VERSION/source/asset identity. MCP BL-204/212/257, security BL-366,
future attestation BL-367 and benchmark/CI owners remain evidence producers, not
a hard prerequisite graph or a replacement release gate. Architecture and status
follow the actual producer contracts and [release scope](release-scope.md).

**Target and boundaries.** Render an actionable human-readable Markdown report
and GitHub Actions Job Summary for a verified candidate, with optional later
version-specific public GitHub Markdown after separately confirmed publication.
Show a release summary, version/commit/artifact IDs, platform and MCP coverage,
quality/security/SBOM/advisory/reproducibility/performance findings, outstanding
exceptions, differentiated pass/fail/not-run/not-applicable/unverified states,
links to real producer evidence and its retention, and the actual decision owner.
The report never invents PASS on absent or expired evidence, silently overrides
BL-366 informational findings, creates its own vulnerability/SBOM/conformance
engine, or grants release authority. Use deterministic redacted bounded rendering
and exact source/attempt bindings; test missing, stale, contradictory, and
misbound inputs. Review whether Markdown under docs/releases/ is an appropriate
durable public location during implementation; do not add a sixth Release asset
or change the existing five-asset distribution and CHANGELOG release notes.

**Scope/closure.** Reuse the existing CI and verified producer outputs, with no
product runtime, VERSION, frozen artifact or new GitHub write/permission effect.
Publishing a version-specific permanent report is a separate authorized Git
operation with poststate readback. Until a separate release-gate decision, this
release-unassigned owner does not block existing prereleases or stable BL-263.

<a id="bl-370-future-streamable-http-mcp-transport"></a>

## Future Streamable HTTP MCP transport

Backlog applicability: BL-370.

**Architecture role and prerequisites.** Separate accepted Post-1.0
transport/host/security implementation owner. Reuse BL-224 transport/core
separation, BL-233 effective configuration, BL-204 exact-revision
conformance/snapshot tests, BL-212 schema
validation and, when effective state is involved, BL-219/239 bindings. Before
implementation, explicitly decide listener/network exposure, endpoint,
authentication and principal binding, TLS/origin/DNS-rebinding/CSRF,
limits/stream lifetime, host lifecycle and platform threat model through its
own ADR and security review. No Version 1.0 release prerequisite or premature
HTTP support advertisement.

**Target and acceptance.** Implement the actual supported Streamable HTTP MCP
transport directly against the existing Go core. HTTP transport availability
does not authorize BL-373 management. Expose management only if it is
implemented, explicitly authorized in the effective catalog, and covered
by the HTTP authentication, security and activation gates.
Extend the existing conformance/snapshot contract for each exposed pair of
exact protocol revision and HTTP transport, including official HTTP server
conformance against the real isolated test listener plus negative security,
protocol, lifecycle and Windows/Linux tests. Require deterministic automatic
missing-coverage detection and explicit public revision/transport support
matrix updates; a missing/failed gate prevents HTTP activation. No production
HTTP-to-STDIO bridge, uncontrolled remote listener, new dependency, permission
or service change without its own decision and authorization. Future additions
belong to this transport owner, not a reopening of BL-204.

<a id="bl-368-documentation-navigation"></a>

## Documentation navigation

Backlog applicability: BL-368.

**Role and authorities.** BL-368 owns future deterministic README navigation
generation and drift validation in development tooling, outside `cmd/server`.
[BACKLOG.md](../../BACKLOG.md#release-unassigned-documentation-tooling) owns its
Planned status and release-unassigned placement; the
[traceability row](architecture-traceability.md#release-unassigned-documentation-tooling)
maps reuse, retrofits and closure. The [documentation quality gate](../documentation-quality-gate.md),
[documentation style](../documentation-style.md) and [testing requirements](../testing.md)
govern public-document rules and validation. This registration implements nothing.

**Prerequisites and reuse.** The existing documentation-validation foundation
is available; there is no outstanding hard prerequisite.
Inspect and reuse `cmd/doccheck` heading, fence, duplicate-anchor, explicit-anchor
and local-link handling, its fixtures, and `scripts/Test-DocumentationConsistency.ps1`
before adding behavior. Extend these owners rather than creating a second
general documentation engine. BL-365 is the separate onboarding owner in SPR-048;
its manual README navigation can change without this tooling. Neither BL is a
hard predecessor of the other, and neither may block the other's completion.
No sprint, Version 1.0 or Post-1.0
commitment or release gate is introduced.

**Retrofits and public contract.** Later separately authorized implementation
must integrate a reusable local generation command with one clearly marked
README ToC region and extend the existing documentation/CI gate and contributor
guidance. Reconcile the then-current README shape, including any BL-365 changes,
without taking over its onboarding scope. Navigation derives from actual Markdown
headings in stable document order. Resolve GitHub-compatible heading and anchor
rules consistently with existing validators, including normalization, Unicode,
punctuation, duplicate headings, explicit anchors and fenced code. No MCP tool,
schema, configuration, runtime, binary, product-version or release delta is planned.

**Representation and safety.** The local generator may change only one
unambiguously delimited managed region. Preserve every byte outside it and the
existing UTF-8 encoding, BOM state, LF/CRLF representation and final newline.
Repeated generation must produce identical bytes; no silent whole-file rewrite
or data loss. Missing, duplicate, malformed or ambiguous markers fail closed
before writing. Bound document/input size and processing resources using the
existing checker limits; preserve local-first behavior and public portability
without private workflow dependencies, credentials, installations or permissions.

**CI and excluded alternatives.** The existing documentation gate must compute
the expected bytes without mutating the checkout and fail with actionable
diagnostics on drift, invalid anchors, missing/invalid links or unexpected changes
outside the managed region. A targeted local command may update that region.
CI commits, pushes or automatic PR creation require a separate later architecture
decision and authorization; no extra GitHub write rights are implied. Exclude
manual link maintenance as the permanent solution, a competing documentation
engine, a documentation website and new external dependencies without a later
decision. Registration changes no README, generator, executable CI or permissions.

**Validation and completion.** Later implementation requires focused positive
and negative fixtures for normal/normalized headings and GitHub anchors,
duplicates, Unicode and punctuation, fenced code, explicit anchors, valid/invalid
links and targets, added/removed/renamed headings, missing/duplicate/malformed
markers, stable order, repeated byte-identical generation, LF/CRLF and UTF-8/BOM/
final-newline preservation, and unchanged bytes outside the managed region.
Prove Windows/Linux and CI parity, read-only drift failure and the applicable
documentation and implementation gates. Reconcile backlog, this binding and the
traceability row with actual generator, validator, README and guidance retrofits;
disposition documentation, CHANGELOG/VERSION, tests/CI, candidates,
release/distribution, migration and security, with any residual dual-bound to one
concrete successor. The present registration closes only its planning action;
BL-368 remains Planned until separately authorized implementation and validation
complete. It creates no product/release artifact, migration or active security
change and leaves BL-365 and completed owners unchanged.

<a id="bl-363-implementation-binding"></a>

## Persistent public prerelease publication

Backlog applicability: BL-363.

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
publication adapter. Completed in SPR-048 after completed BL-363; hard prerequisites
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

The publisher retrofit, selected-platform verification, required independent
technical review and actual historical v0.3.0 normalization are accepted. BL-364
is Completed: the current public release has four unchanged original archives
and shared SHA256SUMS, with independently verified asset identities/digests and
user-confirmed public-download hash parity. The original 40-asset publication
remains true history. The later, explicitly approved cumulative notes contain
the frozen CHANGELOG sections 0.1.0, 0.2.0 and 0.3.0 and have a distinct
[notes identity](../prerelease-publication.md#implementation-and-availability).
Guidance, ADRs and status are reconciled; existing tests/CI and candidate
identities remain valid, with no product migration or deferred deliverable.
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

BL-366 and BL-367 are independent owners with no sprint, release milestone
or release-blocking obligation. They may be scheduled whenever their prerequisites
and a separate implementation authorization are satisfied. Their registration
activates no tool, external provider, workflow permission or credential change.

<a id="bl-366-sbom-security-management"></a>

### SBOM security management

Backlog applicability: BL-366.

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
proves full platform binary composition. The authorized local implementation
extends `cmd/releaseaudit` with a verified inventory reader, bounded read-only
OSV transport/snapshot capture, deterministic offline assessment and a report/
exception consumer. Reuse `verifySupply` with independent version, commit,
manifest and index anchors; reconcile carried `source/go.mod` indirect markers
with SPDX dependency and use relationships. Preserve UNKNOWN/INCOMPLETE on
contradictions or missing evidence. OSV querybatch is positional and paginated;
ID detail reads are separate. No provider call is authorized by local development.
Use offline fixtures and local HTTP servers for validation. Existing Dependabot
remains complementary without an alert API, new scopes or credentials.

Candidate verification and confirmed successful prerelease publication each
invoke a separate informative assessment. The existing candidate dispatch and
local CLI are the manual routes. Findings of every severity and provider errors
create no commit, PR, merge or release gate; existing integrity/build/test gates
remain blocking. Assessment errors are visible as INCOMPLETE/ERROR and cannot
reclassify successful publication. Outputs remain temporary run evidence and
Actions summaries, outside the five public assets. No workflow permission or
credential forwarding change, dependency mutation, new runtime dependency,
database or persistent finding lifecycle. Versioned exact-identity triage retains
findings and flags invalid/expired exceptions. Add no second SBOM generator, mandatory
public SBOM asset or runtime/MCP contract. Close only with accepted triage,
gates, tests and current guidance; disposition VERSION/CHANGELOG, CI, artifacts,
release, migration and security under the shared closure contract.

<a id="bl-367-github-artifact-attestations"></a>

### GitHub Artifact Attestations

Backlog applicability: BL-367.

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
