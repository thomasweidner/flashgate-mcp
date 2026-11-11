# Future Tool and Adapter Plan

**Status:** Accepted planning contract. `BACKLOG.md` owns canonical task status, milestone, and implementation ownership. This document owns the detailed public naming, capability, adapter, security, and efficiency contract referenced by those owners. Implementations must read both their canonical backlog row and the relevant sections of this document. Nothing in this document advertises an unimplemented MCP tool. Technical owners inherit the [implementation contract](implementation-contract.md) and update [architecture traceability](architecture-traceability.md) when names, adapter boundaries, reuse, or retrofit obligations change. Search owners extend the accepted `search_paths` and `search_text` families rather than adding a public tool per backlog row.

## 1. Purpose and scope

FlashGate expands host functionality only when the result remains consistent with the project principles:

- fail-closed security and server-side authorization;
- minimal model-token, wire, CPU, memory, I/O, and process cost;
- Go standard library and direct/native OS APIs before external programs;
- bounded results, pagination/ranges/field selection, and deterministic behavior;
- one domain engine per semantic operation; no shell or duplicate execution engines;
- public tool names describe FlashGate semantics, never the selected backend;
- optional adapters add implementation capability, not authorization authority;
- no mandatory interpreter runtime, remote listener, implicit elevation, or arbitrary plugin loading.

The current eight filesystem tools remain the implemented baseline until their respective owners change runtime behavior. Future names below are accepted target names under their canonical backlog owners and remain absent from `tools/list` until implemented and authorized by the effective profile. The backlog's sprint sequence and Post-1.0 workstream table determine release placement independently of owner status.

## 2. Public naming contract

Public MCP tool names use lower-case snake case and the pattern:

```text
<verb>_<object>[_<qualifier>]
```

`verb_object` is preferred over `object_verb` because the verb makes the requested action immediately unambiguous to an agent. Domain grouping is provided by deterministic catalog ordering and profile-specific exposure rather than by reversing public names.

### 2.1 Canonical verb semantics

| Verb | Meaning |
|---|---|
| `list` | Enumerate a bounded collection |
| `get` | Return structured state, metadata, or a single value |
| `read` | Return payload/content/stream data |
| `set` | Mutate structured state or configuration |
| `write` | Create, replace, or append payload content |
| `edit` | Apply a targeted delta to existing content |
| `create` | Create a new persistent object |
| `delete` | Remove an object |
| `copy` / `move` | Copy or move an existing object |
| `search` | Search a bounded data space |
| `compare` | Compare two current states |
| `query` | Filter/query history, log, or event data |
| `start` / `stop` / `restart` / `wait` | Lifecycle operations |
| `run` | Execute a bounded synchronous operation |
| `resolve` | Resolve a name or identity |
| `test` | Perform an active bounded probe |
| `verify` | Assert current state against expected state or integrity constraints |
| `compress` / `decompress` | Transform file content using a compression codec |

Additional rules:

- `path` means file **or** directory.
- `get` is not an alias for `read`: `get_path_info` returns metadata; `read_file` returns content.
- `set` is not an alias for `write`: `set_sysctl` changes structured configuration; `write_file` changes payload bytes.
- adapter names do not become MCP tool names;
- OS names do not appear in portable tool names unless the semantic object itself is platform-specific, for example `registry` or `sysctl`;
- ambiguous names such as `get_access_context` are prohibited;
- no redundant aliases are added merely for grouping.

### 2.2 Rejected redundant public tools

The target contract intentionally has no separate public:

- `get_directory` or `get_directories` — directory metadata remains `get_path_info`; directory contents remain `list_directory`;
- `get_directory_tree` — recursive traversal is `list_directory(recursive=true, ...)`;
- `list_directories` — use `list_directory(types=["directory"])`;
- `create_file` — use `write_file(mode="create_only")`;
- `replace_file` — use `write_file(mode="replace_only")`;
- `append_file` — use `write_file(mode="append")`;
- `read_binary_file` — use `read_file` with explicit content/range mode;
- adapter-branded aliases such as `ripgrep_search`, `7zip_extract`, `onedrive_pin`, `journalctl_query`, or `scm_start`.

## 3. Adapter and provider taxonomy

### BL-220 adapter selection

Use the first option that meets the operation's semantics and effective policy
on the supported platform:

1. Go standard library and existing FlashGate abstractions.
2. A small platform-specific Go adapter behind the existing domain interface.
3. Direct OS APIs or a stable native interface such as procfs or sysfs.
4. An approved external native executable through the shared typed execution
   boundary, with evidence that the earlier options cannot meet the need.

Record portability, availability, maintenance, dependency cost, effective
security and runtime cost for the chosen path. A linked native library is a
platform dependency that needs its own review, including load-time trust and
distribution. It does not qualify as an approved executable or authorize
dynamic loading into the core. No new dependency is authorized by this policy.

Keep one domain implementation of policy-visible semantics and public results.
The application authorizes each call under the authenticated caller's profile,
root and capability before selecting a backend. Installation and discovery
grant no rights. The effective execution identity may differ from the caller
and must stay explicit in dispatch and audit. Use a fallback only when it can
honor the same authorization, semantics, limits and enforcement. A denial,
identity mismatch or missing required protection cannot trigger fallback.
Return a normalized unsupported or denied outcome when appropriate and keep
unavailable operations out of the effective advertised surface.

### 3.1 Core domain operation

Owns semantic behavior, policy inputs, result shape, limits, and domain errors. Core code remains independent of MCP/JSON-RPC and of a specific OS backend.

### 3.2 Platform adapter

Implements equivalent FlashGate semantics using OS-specific APIs or stable system interfaces. Examples: filesystem, process, Windows Event Log, journald, SCM, systemd, Registry, sysctl, ReadDirectoryChangesW, inotify, Cloud Files, Windows security descriptors, Linux xattr.

### 3.3 Native Tool adapter

Uses an explicitly approved external native binary behind a closed typed FlashGate contract. It never accepts raw command-line fragments or gains authority beyond the calling FlashGate capability. Examples: ripgrep, Git, 7-Zip/7zz, or a reviewed archive/codec utility.

Native Tool adapters use the same policy and Managed Process lifecycle as
typed commands. A server-owned definition selects the executable identity,
fixed operation and closed argument schema. The caller cannot select arbitrary
programs, options or working directories. Build separate argv values from
validated typed fields, handle option terminators and platform argument
encoding, and apply named-root/path policy to every path operand and the
working directory. Start directly under the effective identity with an
explicit minimal environment and safe handle/descriptor inheritance.

Prohibit shells, interpreters, scripts, implicit OS associations and raw CLI
fragments in Version 1.0 product execution. In particular, rejecting known
interpreter names alone is insufficient. Reject unapproved response files,
preprocessors, hooks, plugins, loaders, pagers, external diff, configuration,
helper processes and credential/network access, including defaults loaded
from the repository, user home or environment. A required helper needs an
explicit modeled policy and lifecycle under the same enforcement boundary.
Adapter diagnostics must never enter MCP stdout. Normalize errors and redact
executable paths, identities, raw backend errors and credentials from normal
client results.

Development, build, installation, smoke, validation and administrator scripts
may use interpreters outside the shipped runtime path. Installation scripts
can register a native image. Product requests and service lifecycles cannot
invoke such scripts to implement features.

### 3.4 Provider boundary

Larger optional systems that introduce their own credentials, remote access, process model, or complex dependency surface belong behind the future provider contract rather than inside the trusted core. Candidate families include SSH, HTTP/Web, Docker/Podman, package managers, desktop automation, cloud APIs, and databases.

The provider model must not become arbitrary DLL/SO/Go-plugin loading into the trusted FlashGate process. Built-in/static reviewed packages and isolated out-of-process providers are preferred; any in-process model requires an explicit later security decision and must never bypass roots, capabilities, principal binding, limits, execution identity, audit, or redaction.

## 4. Cross-cutting security and efficiency contract

### 4.1 Profile-specific exposure and platform disclosure

The normal/default portable profile exposes only tools permitted by the effective profile and capability set. Platform-sensitive tools such as Registry, sysctl, ADS, or xattr appear only when the corresponding platform-sensitive capability/profile is explicitly enabled.

OS/provider/adapter detail is minimized rather than treated as an authorization boundary:

- `get_system_info` returns only requested and allowed fields;
- provider name, adapter binary path/version/hash, raw reparse tags, native error codes, and backend identity are diagnostic/admin data unless needed by the operation contract;
- normal errors use normalized machine-readable categories such as `unsupported_capability`, never raw Win32/errno/backend text;
- FlashGate does not invent false cross-platform abstractions merely to hide the OS.

### 4.2 Enforcement truth

A requested policy is not considered enforced merely because it appears in configuration. Execution backends must expose the effective enforcement strength needed by policy evaluation, including equivalent states for:

- network: `denied`, `destination_allowlist`, `unrestricted`, `unsupported`;
- CPU: `hard`, `observed_only`, `unsupported`;
- memory: `hard`, `observed_only`, `unsupported`;
- process isolation: `hard`, `partial`, `unsupported`.

If a command definition requires stronger enforcement than the active backend can prove, execution fails closed. No silent best-effort downgrade is permitted.

### 4.3 Executable identity

Typed commands and Native Tool adapters may require policy-selectable executable identity evidence, including where available:

- canonical absolute path;
- reparse/path safety state;
- stable file identity;
- length/size;
- SHA-256;
- version metadata;
- publisher/code-signature identity;
- parent-path ownership/writability trust;
- post-start executable identity where the OS makes this reliable.

BL-137 selects the exact policy classes. Resolve from a controlled server-side
definition, verify parent trust and substitution resistance for the selected
class, and bind the evidence to the executable actually dispatched. Where a
prelaunch check alone leaves an executable replacement race, require an
OS-supported identity-bound launch or an equivalent proof. Post-start evidence
cannot undo an unsafe launch. A path allowlist provides location evidence.
Cryptographic identity requires independently trusted hash or signing evidence
as specified by the policy. Deny when the active platform cannot establish
the required identity or safe execution.

### 4.4 Resource bounds

Every expensive operation defines server maxima for the dimensions it can amplify: entries, depth, scanned bytes, read bytes, written bytes, expanded bytes, compression ratio, events, matches, output, runtime, concurrent work, temporary data, handles, and result TTL. Client-requested limits can only reduce, never increase, server caps.

Process-backed work also accounts for CPU/RAM, children and descendants,
network access, stdout and stderr separately, process startup, and inherited
handles. Reuse Operations for request cancellation, quotas and temporary state
and Managed Process for launch, termination, reaping, timeout and descendant
cleanup. Where hard enforcement is required, observed use and partial
isolation cannot satisfy it. Record effective strength for each dimension and
test unavailability and downgrade before advertising support.

An optional accelerator needs reproducible differential security and semantic
tests against the Go baseline, plus the applicable
[benchmark workspace and measurement gates](../testing.md#authoritative-benchmark-workspace-gate).
Include process startup in latency and account for CPU, memory, allocations,
copies, I/O, scanned data, output, wire/token costs and dependency overhead as
applicable. Compare deterministic results and errors, including adverse
inputs, cancellation and the fallback case. Activate the accelerator only
within the proved policy and semantic envelope.

### 4.5 Stateful handles

Watch handles, process handles, jobs, cursors, result resources, caches, and other state are bound to the applicable caller principal, profile, root, execution backend, service generation, protocol context, and expiry. A service restart invalidates stale generation-bound state.

### 4.6 Content identity, cache, and consumer independence

Content hashes/fingerprints/snapshot identifiers are evidence about observed state, not bearer capabilities. Every externally visible read/verify/conditional result still passes current authorization and root/path policy. Optional server-side identity/content caches are bounded, context-partitioned, safely invalidated, and semantically transparent when absent; a request for fresh/strong proof cannot be satisfied solely by a reusable cached proof when its contract requires current state to be re-evaluated.

Public FlashGate contracts and guidance remain consumer-independent. They have no organization-specific task or infrastructure, private control-plane, machine-local workflow, or particular agent-product prerequisite. A consumer may layer its own workflow or manifest policy over generic FlashGate primitives without transferring that policy into the product.

## 5. Version 1.0 filesystem and discovery corrections

### 5.1 Named-root discovery — `BL-352`

Public tools:

- `list_roots`
- `get_root_info`

Purpose: let an agent discover root IDs and the bounded properties it needs without depending on deprecated MCP Roots or on absolute host paths.

Default fields are compact and may include root ID, safe label, read/write availability, and a compact capability summary. Optional `fields` can request relevant limits, allowed file types, or other policy-visible properties. Absolute host paths, raw ACLs, principal internals, and backend implementation details are not returned by default.

### 5.2 Path metadata — `BL-043`, `BL-345`, `BL-351`, `BL-359`

`get_path_info` remains the single path-metadata query owner. Field selection prevents the baseline call from becoming a large platform dump.

Field groups may include:

- portable basics: exists, type, name, size, modified time, portable permission summary;
- `storage`: `compression_supported`, `compression_enabled`, `compression_inherited_default`, `encryption_supported`, `encryption_enabled`, `encryption_inherited_default`, sparse state, logical/allocated size where reliable;
- `availability`: cloud-backed/placeholder state, local/partial/online-only state, pin state, in-sync state, safe provider-independent byte counters;
- `identity`: bounded stable file identity where policy permits;
- `link`: symlink/reparse classification without unnecessary target/host disclosure;
- `security`: normalized security summary where authorized.

### 5.3 Directory listing — `BL-037`–`BL-040`, `BL-049`

`list_directory` owns both shallow and recursive directory enumeration. Target options include:

- `root_id`, `path`;
- `recursive=false`;
- `max_depth` when recursive;
- `types` such as file/directory;
- bounded name/filter rules;
- `fields`;
- deterministic sort;
- `cursor` and `limit`.

Recursive output should prefer flat root-relative entries that can be paged deterministically instead of an unbounded deeply nested JSON tree.

### 5.4 File write modes — `BL-050`–`BL-055`, `BL-063`

`write_file` owns complete-content writes and append behavior. Canonical modes are:

- `create_only`;
- `replace_only`;
- `upsert`;
- `append`.

Atomic replacement and conditional preconditions remain explicit options where supported. `edit_file` remains the targeted-delta operation with exact range/match contracts, expected-match-count checks, and dry-run support.

### 5.5 Compare and verify — `BL-346` (Version 1.0)

Public tools:

- `compare_paths`
- `verify_paths`

`compare_paths` supports file↔file and directory↔directory. File↔directory produces an explicit type mismatch. Candidate options include:

- `mode=auto|metadata|content|text`;
- `recursive` and `max_depth`;
- include/exclude filters with an explicit filtered marker;
- bounded diff-entry and diff-byte limits;
- cursor/paging for large difference sets.

The implementation uses the cheapest safe equality proof first and reuses the canonical hashing/tree primitives; it does not create a second hash engine.

`verify_paths` checks one or more paths against a supplied expected state. Supported expectations include, where applicable:

- exists/absent;
- type;
- size;
- modified time;
- SHA-256/content fingerprint;
- deterministic tree fingerprint/inventory;
- entry count;
- selected metadata expectations.

The input may contain one or many expected-state records so a caller can verify a manifest-like path set in one bounded request. This is generic path verification, not a chat/task/handoff-specific API. Default results remain compact (`verified`, checked count, mismatch count, indeterminate count). Mismatch details are bounded and returned only as needed/requested. A statement that something "changed" requires a supplied or server-owned baseline; absence of a baseline is not inferred as change.

An explicit fresh/strong verification mode re-evaluates the current metadata and, where required by the requested proof, current content/hash evidence instead of accepting a reusable cached result as the proof. Ordinary `auto` verification may reuse safe lower-cost evidence when the semantics remain equivalent. Content identities never authorize access.

For cloud/local-only content, `unknown`/`indeterminate` must not be silently converted to `different` when equality cannot be proven without forbidden hydration.

## 6. Version 1.0 search and execution corrections

### 6.1 Search — `BL-068`–`BL-083`

Canonical public tools:

- `search_paths` — path/name/metadata search;
- `search_text` — literal/regular-expression content search.

Pure Go is the normative portable baseline (`BL-082`). The optional ripgrep adapter (`BL-081`) is cross-platform and may accelerate `search_text` only when security, semantics, deterministic result requirements, and benchmark evidence permit it. It never creates a public `ripgrep_*` tool. Raw ripgrep options, preprocessors, and helper-spawning archive search are outside the adapter contract unless separately modeled and approved.

### 6.2 Managed process and typed commands — `BL-119`–`BL-152`, `BL-353`

`start_process(command_id, typed_args)` and `run_command(command_id, typed_args)` use the same Managed Process / Typed Command engine. `run_command` is only the synchronous wrapper; FlashGate has no second arbitrary execution engine.

The [staged activation contract](implementation-contract.md#managed-process-activation)
keeps SPR-056 non-launching. BL-151 owns productive native launch and
activation of the dependent managed-child routes in SPR-057. Those routes
remain unavailable to direct calls and absent from the effective catalog
until the applicable controls and activation evidence pass.

Public command discovery:

- `list_commands`
- `get_command_info`

Only commands available to the effective profile/capability context are exposed. Discovery returns compact typed argument/limit information and does not reveal absolute executable paths or adapter identity by default.

### 6.3 System information — `BL-153`–`BL-158`

The canonical planned public name is `get_system_info`, not `system_info`. Related V1 candidates remain `get_disk_usage` and `get_environment_info` with strict field allowlists/redaction.

`BL-158` owns post-Version-1.0 network observation/probe planning and must separate:

- `network.observe`;
- `network.probe`;
- command egress policy, which remains part of typed execution.

Candidate tools:

- `get_network_info`;
- `list_network_connections`;
- `resolve_host`;
- `test_tcp_connection`.

Network probes use explicit policy rather than generic outbound authority. The policy must support bounded host/exact-name or suffix allowlists, port allowlists, and explicit treatment of loopback, private, link-local, multicast, unspecified, and other non-public destination classes. DNS names are resolved under policy and the actual address used for connection is validated again so rebinding or multi-answer resolution cannot bypass destination rules. Timeouts, attempted addresses, returned fields, and diagnostic text stay small and bounded. There is no generic HTTP client in the core. ICMP/ping is not a baseline requirement unless a later native/security decision justifies it.

## 7. Cloud-backed and placeholder paths — `BL-345`

The model is vendor-neutral. OneDrive is a required Windows validation case, not a FlashGate dependency or a public tool prefix.

### 7.1 Reparse classification

Do not model all Windows reparse points as one security class. Internal classification must distinguish at least equivalent states for:

- normal;
- name-surrogate/link-like;
- cloud placeholder;
- other known reparse;
- unknown reparse.

A cloud placeholder is not automatically a path escape. Unknown/unsafe semantics fail closed. Reparse policy and content-materialization policy are separate decisions.

### 7.2 Root backing and materialization policy

Roots may be classified internally as local, cloud-sync, network, or unknown where this can be determined safely.

Per-root/profile content materialization policy:

```text
local_only
allow_hydration
```

Default is `local_only`. A metadata operation may inspect a placeholder without downloading content. A content operation that needs unavailable bytes under `local_only` returns a safe `content_not_local`/equivalent category instead of causing implicit network I/O.

When hydration is allowed, FlashGate enforces server-side maxima for hydrated files, bytes, duration, and concurrent work.

### 7.3 Sync-backed mutations

Per-root policy:

```text
sync_backed_mutation = deny | allow
```

A local write inside a sync root may later propagate externally. FlashGate may report local write completion and that external synchronization is possible; it must never claim cloud synchronization completion unless a future explicit provider contract can prove it.

### 7.4 Availability mutation

Post-1.0 candidate public tool:

- `set_path_availability`

Provider-neutral semantic states may include `always_local`, `automatic`, and `online_only` only when the platform adapter can truthfully implement them. No `onedrive_*` public aliases.

### 7.5 Compare/verify/watch behavior

- `compare_paths` and `verify_paths` can return incomplete/indeterminate when local-only policy prevents content proof;
- `start_path_watch` events never hydrate content;
- sync-origin classification is emitted only when FlashGate can actually prove it;
- provider identity remains diagnostic/admin data by default.

When explicitly requested through `get_path_info(fields=["availability"])`, normalized fields may include `content_state`, `pin_state`, `in_sync`, `on_disk_size`, `validated_size`, `modified_not_synced_size`, `recall_on_open`, and `recall_on_data_access` where the platform can determine them reliably. Unsupported/unknown values remain explicit rather than inferred. Provider name/version, raw reparse tags, native identifiers, and adapter details remain diagnostic/admin data by default.

Windows Cloud Files/CFAPI is an internal Platform Adapter for this domain.

## 8. Filesystem watch — `BL-347`

Canonical public family:

- `start_path_watch`
- `read_path_watch_events`
- `stop_path_watch`

The watched target may be one file or a directory. The watch handle is opaque and context-bound. Required controls include:

- root confinement;
- recursive flag and event-type filtering;
- TTL;
- bounded queue;
- debounce/coalescing;
- monotonically useful sequence/cursor semantics;
- overflow marker and explicit resync requirement;
- global/per-principal watch limits;
- cancellation/cleanup/service-restart behavior;
- no file content in events by default.

Expected internal adapters: Windows ReadDirectoryChangesW-class implementation and Linux inotify-class implementation. `watch` alone and `watch_paths` are rejected as public names because FlashGate can later watch other domains.

## 9. Archives — `BL-348`

Archive is a format-neutral domain with the public family:

- `list_archive_formats`
- `get_archive_info`
- `list_archive_entries`
- `read_archive_entry`
- `verify_archive`
- `create_archive`
- `extract_archive`

A format advertises the operations it actually supports; FlashGate does not assume every format can be created and extracted.

Candidate format coverage is capability-based rather than a promise that every backend implements every item:

- ZIP/ZIP64 — inspect/list/read/verify/create/extract where the selected built-in/adapter path supports it;
- TAR — inspect/list/read/verify/create/extract;
- TAR.GZ/TGZ and TAR.BZ2 — composed archive+compression support where available;
- TAR.XZ and TAR.ZST — optional codec/Native Tool adapter support;
- 7z — optional adapter, operation set advertised at runtime;
- RAR — primarily inspect/list/read/verify/extract; creation is not assumed;
- CPIO — optional adapter when justified;
- ISO and WIM — inspection/read-only candidates only when a safe bounded implementation is justified.

`list_archive_formats` reports the actual operation matrix for the active build/profile/adapters; unsupported operations are never guessed from a filename extension.

Security requirements for **every** archive backend:

- sources and destination are confined to allowed Named Roots;
- reject absolute-path and `..` escapes;
- reject or explicitly policy-gate symlinks, junctions, reparse points, device nodes, FIFOs, and other special types;
- Windows reserved names, ADS syntax, case collisions, and duplicate entries are handled deterministically;
- bounded entry count, per-entry expanded size, total expanded size, nesting depth, runtime, and compression ratio;
- explicit overwrite/conflict strategy;
- no self-extracting payload execution;
- no raw backend arguments;
- controlled staging and post-extraction realpath/identity validation where required;
- archive passwords/keys are not exposed as unsafe external-tool command-line arguments;
- partial failure leaves deterministic residue/cleanup semantics.

Prefer built-in Go and OS paths. Optional 7-Zip/7zz or bsdtar executables may
extend supported formats after the [Native Tool acceptance](#bl-220-native-tools)
passes. A linked libarchive integration needs separate dependency, loading
and distribution approval.

## 10. Content compression vs transparent path compression

FlashGate distinguishes three concepts:

1. **Archive** — multiple objects in a container, for example ZIP or TAR.
2. **Content compression** — transform a file's bytes into a compressed representation, for example GZIP or Zstandard.
3. **Transparent filesystem/path compression** — the logical file/directory remains the same object while the filesystem stores it compressed.

### 10.1 Content compression — `BL-354`

Public tools:

- `list_compression_formats`
- `compress_file`
- `decompress_file`

The codec interface plans support for the useful portable format set, subject to implementation evidence:

- DEFLATE;
- GZIP;
- ZLIB;
- BZIP2;
- XZ/LZMA;
- Zstandard;
- LZ4;
- Brotli.

Go standard-library support is used first. Optional codec/native adapters are allowed only through the common bounded typed contract. Compression levels are semantic presets or bounded enums, not raw backend flags.

### 10.2 Transparent path compression/encryption — `BL-351`

Query state through `get_path_info(fields=["storage"])`; there is no separate `get_path_storage_info`. Where reliably supported, normalized requested storage fields may include `compression_supported`, `compression_enabled`, `compression_inherited_default`, `encryption_supported`, `encryption_enabled`, `encryption_inherited_default`, sparse state, logical size, and allocated/physical size. An inherited default describes directory/child behavior, not the enabled state of the current path. Unknown/unsupported fields are explicit and do not fabricate cross-filesystem equivalence.

Canonical mutation:

- `set_path_compression`

For directories, `recursive=false` changes the directory's default/inheritance behavior where the platform supports it and does not silently rewrite existing descendants. `recursive=true` explicitly requests bounded descendant mutation and returns counts/counters.

Separate high-risk mutation:

- `set_path_encryption`

Compression and encryption are independent operations. FlashGate does not silently decompress to encrypt or decrypt to compress; conflicting states return an explicit error/decision result. Volume/partition compression or encryption administration is outside this owner.

## 11. Platform configuration — `BL-349` and `BL-355`

Registry and sysctl are not artificially mapped to one generic settings store.

### 11.1 Read-only platform configuration — `BL-349`

Windows candidates:

- `list_registry_keys`
- `list_registry_values`
- `get_registry_value`

Linux candidates:

- `list_sysctls`
- `get_sysctl`

These tools are absent from portable/default profiles unless an explicit platform-sensitive capability is enabled. Registry access is constrained to configured key roots/namespaces. sysctl access is constrained to an allowlisted key set/namespace. xattr does not belong to this owner.

### 11.2 Platform configuration mutation — `BL-355`

Windows candidates:

- `create_registry_key`
- `set_registry_value`
- `delete_registry_value`
- `delete_registry_key`

Linux candidate:

- `set_sysctl`

Mutation is separate high-risk work with explicit capability, allowlist, audit, negative tests, and no general Registry editor/sysctl browser.

## 12. System logs — `BL-356`

Public tools:

- `list_system_log_sources`
- `query_system_logs`

Internal adapters:

- Windows Event Log API;
- Linux journald.

The query contract uses bounded time range, source/provider/unit, severity/event ID, field selection, cursor, and limit. Default output is compact and does not render large full event payloads unless requested and within budget.

## 13. OS service observation/control — `BL-357`

This domain controls **other operating-system services** and is separate from FlashGate's own SCM/systemd host lifecycle.

Public tools:

- `list_services`
- `get_service_info`
- `start_service`
- `stop_service`
- `restart_service`

Internal adapters: Windows SCM and Linux systemd. Observation and mutation have separate capabilities; service control is high-risk and requires explicit activation/audit.

## 14. Scheduled jobs/timers — `BL-358`

Public tools:

- `list_scheduled_jobs`
- `get_scheduled_job_info`
- `create_scheduled_job`
- `update_scheduled_job`
- `delete_scheduled_job`
- `enable_scheduled_job`
- `disable_scheduled_job`
- `run_scheduled_job`

Internal adapters: Windows Task Scheduler and Linux systemd timers. Cron may be evaluated as an optional limited adapter; it is not the portable baseline. Creation/update/delete/run are high-risk because they can create persistence or execute later without the initiating MCP session.

## 15. Path security and extended metadata — `BL-359`

Portable security surface:

- `get_path_security`
- `set_path_security` (high-risk)

Windows Alternate Data Streams:

- `list_file_streams`
- `read_file_stream`
- `write_file_stream`
- `delete_file_stream`

Linux extended attributes:

- `list_extended_attributes`
- `read_extended_attribute`
- `write_extended_attribute`
- `delete_extended_attribute`

ADS and xattr are not described as semantically identical. Platform-sensitive profile/disclosure rules apply. Security mutations require explicit capability and must preserve root/identity/audit boundaries.

## 16. Git first-party Native Tool adapter — `BL-360`

Initial scope is read-only and local:

- `get_git_status`
- `get_git_diff`
- `list_git_branches`
- `list_git_commits`
- `get_git_commit`
- `get_git_blame`

The repository must be within an authorized Named Root. The adapter must disable or reject hooks, arbitrary `-c`, external diff/pager, raw CLI options, uncontrolled environment, and implicit remote/credential activity. Remote fetch/push/pull is not part of this owner. Git binary identity is governed by the shared executable-identity contract.

## 17. Audit integrity and query — `BL-166`, `BL-361`

`BL-166` remains the Version 1.0 owner for bounded structured audit events, immutable event/correlation IDs, rotation, retention, disk-full/backpressure behavior, redaction, and log-injection protection.

The optional integrity mode is explicit:

```text
audit_integrity = none | hash_chain
```

`none` preserves the base audit lifecycle without chain evidence. `hash_chain` adds the integrity evidence described below without changing authorization or making the audit store immutable.

Post-Version-1.0 candidates:

- `get_audit_status`
- `query_audit_events`
- `verify_audit_integrity`

Optional hash-chain evidence may include sequence, previous-event hash, event hash, and rotation-link hash. It is documented as tamper evidence/defense in depth, not as protection against a fully privileged local attacker who can replace all audit storage.

## 18. User-session capabilities — `BL-362`

These tools are only available through a valid user-session backend and are not simulated through a system service account:

- `list_clipboard_formats`
- `read_clipboard`
- `write_clipboard`
- `clear_clipboard`
- `send_desktop_notification`

Clipboard/notification permissions, payload types, size limits, and session identity require explicit platform/user-session contracts. Broad UI automation, screenshots, accessibility input, and desktop control remain provider/future-architecture work rather than this owner.

## 19. Portable public agent skill — `BL-350`

Publish exactly one canonical optional FlashGate skill rather than separate cache/file/search/process skills. The skill has a small always-loaded decision core and progressively loaded capability references, for example filesystem, identity-and-verification, search, process/execution, platform-storage, and efficiency recipes. This grouping is an authoring/layout choice, not a new public API taxonomy.

The core teaches portable strategy: prefer batch calls over repeated scalars; request only needed fields/ranges/pages; verify known identities before rereading unchanged payload; use conditional/not-modified retrieval only when the active server advertises it; use resource handles for large results; understand roots/profiles/capabilities and annotation-vs-authorization; distinguish synchronous work from jobs; and perform protocol/feature discovery with graceful fallback.

The skill explains how to choose tools but does not duplicate their schemas or become the API authority. It reuses `BL-216` compact server instructions without treating them as the full skill. It never becomes an authorization mechanism.

The skill and its examples are consumer-independent: no organization-specific identifiers or paths, private workflow or control-plane prerequisites, or assumed agent product. Consumer policies may compose FlashGate externally; the skill grants no authorization.

## 20. Public tool inventory by target

Each row assigns the first public implementation owner. Later retrofit owners
inherit the same [catalog acceptance](implementation-contract.md#bl-215-catalog-and-entry-budget-binding)
for the existing names they change. For example, BL-043 changes path metadata,
BL-037–040/049 change directory listing, and BL-052/053/055/063 change file writes.

`IMPLEMENTED` means present in the current registry where authorized.
`PLANNED` means an accepted name with no executable measurement.
`CANDIDATE` means a planned, decision-gated surface with no executable
measurement or activation. The milestone is independent of execution status.

An owner cell with `/ activation BL-151` retains the first implementation
owner and identifies the separate owner of productive availability. A
completed foundation can therefore retain a planned surface. The activation
owner is responsible for changing the surface state after acceptance.

**Exposure and budget classes.** R is safe-read eligible, including in broader
authorized catalogs. F is standard filesystem eligible. O requires an
explicit opt-in composition. The capabilities below describe eligibility.
The profile and policy owners define their final IDs and decide effective
registration. Each class follows the
[whole-catalog composition rule](efficiency.md#tool-catalog-and-initialization-budgets).
Windows and Linux are the intended platforms unless the row specifies a
narrower gate. All planned platform availability remains subject to proof.

| Family | Public names | Owner | Milestone / state | Eligible capability / class | Adapter acceptance |
|---|---|---|---|---|---|
| Root and command discovery | `list_roots`, `get_root_info` | BL-352 | 1.0 / PLANNED | Named-root read / R | [Acceptance](#bl-220-context-and-policy) |
| Root and command discovery | `list_commands`, `get_command_info` | BL-353 | 1.0 / PLANNED | Authorized typed-command discovery / O | [Acceptance](#bl-220-managed-execution) |
| Path/filesystem | `get_path_info` | BL-002 | 1.0 / IMPLEMENTED | Filesystem read / R | — |
| Path/filesystem | `copy_path`, `move_path`, `delete_path` | BL-002 | 1.0 / IMPLEMENTED | Filesystem write / F | — |
| Path/filesystem | `get_paths_info` | BL-047 | 1.0 / PLANNED | Filesystem read / F | [Acceptance](#bl-220-filesystem) |
| Path/filesystem | `hash_paths` | BL-048 | 1.0 / PLANNED | Filesystem read / F | [Acceptance](#bl-220-filesystem) |
| Path/filesystem | `compare_paths`, `verify_paths` | BL-346 | 1.0 / PLANNED | Filesystem read / F | [Acceptance](#bl-220-filesystem) |
| Path/filesystem | `apply_filesystem_plan` | BL-056 | 1.0 / PLANNED | Bounded filesystem plan and operation-specific permissions / F | [Acceptance](#bl-220-filesystem) |
| Path/filesystem | `get_path_security`, `set_path_security` | BL-359 | Post-1.0 / PLANNED | Separate security inspection and high-risk mutation / O | [Acceptance](#bl-220-system-and-platform) |
| Directory | `list_directory` | BL-002 | 1.0 / IMPLEMENTED | Filesystem read / R | — |
| Directory | `create_directory` | BL-002 | 1.0 / IMPLEMENTED | Filesystem write / F | — |
| Directory | `get_directory_size` | BL-061 | 1.0 / PLANNED | Filesystem read / F | [Acceptance](#bl-220-filesystem) |
| File | `read_file` | BL-002 | 1.0 / IMPLEMENTED | Filesystem read / R | — |
| File | `write_file` | BL-002 | 1.0 / IMPLEMENTED | Filesystem write / F | — |
| File | `read_files` | BL-046 | 1.0 / PLANNED | Filesystem read / F | [Acceptance](#bl-220-filesystem) |
| File | `edit_file` | BL-050 | 1.0 / PLANNED | Filesystem write / F | [Acceptance](#bl-220-filesystem) |
| Search | `search_paths` | BL-069 | 1.0 / PLANNED | Root-scoped search / O | [Acceptance](#bl-220-search) |
| Search | `search_text` | BL-072 | 1.0 / PLANNED | Root-scoped content search / O | [Acceptance](#bl-220-search) |
| Path watch | `start_path_watch`, `read_path_watch_events`, `stop_path_watch` | BL-347 | Post-1.0 / PLANNED | Root-confined watch and handle ownership / O | [Acceptance](#bl-220-system-and-platform) |
| Archive | `list_archive_formats`, `get_archive_info`, `list_archive_entries`, `read_archive_entry`, `verify_archive`, `create_archive`, `extract_archive` | BL-348 | Post-1.0 / PLANNED | Format and operation capability, including destination write for mutations / O | [Acceptance](#bl-220-native-tools) |
| Content compression | `list_compression_formats`, `compress_file`, `decompress_file` | BL-354 | Post-1.0 / PLANNED | Codec capability and operation-specific read/write / O | [Acceptance](#bl-220-native-tools) |
| Storage/path attributes | `set_path_compression`, `set_path_encryption` | BL-351 | Post-1.0 / PLANNED | Filesystem support and explicit mutation, with separate high-risk encryption / O | [Acceptance](#bl-220-system-and-platform) |
| Storage/path attributes | `set_path_availability` | BL-345 | Post-1.0 / CANDIDATE | Cloud availability mutation with proven platform/provider support / O | [Acceptance](#bl-220-system-and-platform) |
| Process observation | `list_processes` | BL-114 | 1.0 / PLANNED | Process observation / O | [Acceptance](#bl-220-managed-execution) |
| Process observation | `get_process_info` | BL-116 | 1.0 / PLANNED | Process observation / O | [Acceptance](#bl-220-managed-execution) |
| Process observation | `get_process_tree` | BL-117 | 1.0 / PLANNED | Process observation / O | [Acceptance](#bl-220-managed-execution) |
| Managed process | `start_process` | BL-122 / activation BL-151 | 1.0 / PLANNED | Managed process and typed allowlist / O | [Acceptance](#bl-220-managed-execution) |
| Managed process | `wait_process` | BL-123 / activation BL-151 | 1.0 / PLANNED | Managed process and handle ownership / O | [Acceptance](#bl-220-managed-execution) |
| Managed process | `read_process_output` | BL-124 / activation BL-151 | 1.0 / PLANNED | Managed process and handle ownership / O | [Acceptance](#bl-220-managed-execution) |
| Managed process | `stop_process` | BL-126 / activation BL-151 | 1.0 / PLANNED | Managed process and handle ownership / O | [Acceptance](#bl-220-managed-execution) |
| Managed process | `write_process_input` | BL-128 | Post-1.0 / CANDIDATE | Explicit process input policy and handle ownership / O | [Acceptance](#bl-220-managed-execution) |
| Managed process | `stop_external_process` | BL-127 | Post-1.0 / CANDIDATE | High-risk external PID control / O | [Acceptance](#bl-220-managed-execution) |
| Typed execution | `run_command` | BL-143 | 1.0 / PLANNED | Typed command execution over managed process / O | [Acceptance](#bl-220-managed-execution) |
| System | `get_system_info` | BL-153 | 1.0 / PLANNED | Allowlisted system read / O | [Acceptance](#bl-220-system-and-platform) |
| System | `get_disk_usage` | BL-154 | 1.0 / PLANNED | Scoped system read using BL-062 / O | [Acceptance](#bl-220-system-and-platform) |
| System | `get_environment_info` | BL-155 | 1.0 / PLANNED | Allowlisted and redacted system read / O | [Acceptance](#bl-220-system-and-platform) |
| Network | `get_network_info`, `list_network_connections` | BL-158 | Post-1.0 / CANDIDATE | Network observation / O | [Acceptance](#bl-220-system-and-platform) |
| Network | `resolve_host`, `test_tcp_connection` | BL-158 | Post-1.0 / CANDIDATE | Explicit bounded network probe / O | [Acceptance](#bl-220-system-and-platform) |
| System logs | `list_system_log_sources`, `query_system_logs` | BL-356 | Post-1.0 / PLANNED | Bounded log read with Event Log or journald / O | [Acceptance](#bl-220-system-and-platform) |
| Services | `list_services`, `get_service_info`, `start_service`, `stop_service`, `restart_service` | BL-357 | Post-1.0 / PLANNED | Separate service observation and high-risk control with SCM or systemd / O | [Acceptance](#bl-220-system-and-platform) |
| Scheduled jobs | `list_scheduled_jobs`, `get_scheduled_job_info`, `create_scheduled_job`, `update_scheduled_job`, `delete_scheduled_job`, `enable_scheduled_job`, `disable_scheduled_job`, `run_scheduled_job` | BL-358 | Post-1.0 / PLANNED | Separate schedule observation and persistence/control with Task Scheduler or systemd timers / O | [Acceptance](#bl-220-system-and-platform) |
| Windows Registry | `list_registry_keys`, `list_registry_values`, `get_registry_value` | BL-349 | Post-1.0 / PLANNED | Windows allowlisted Registry read / O | [Acceptance](#bl-220-system-and-platform) |
| Windows Registry | `create_registry_key`, `set_registry_value`, `delete_registry_value`, `delete_registry_key` | BL-355 | Post-1.0 / PLANNED | Windows high-risk allowlisted Registry mutation / O | [Acceptance](#bl-220-system-and-platform) |
| Linux sysctl | `list_sysctls`, `get_sysctl` | BL-349 | Post-1.0 / PLANNED | Linux allowlisted sysctl read / O | [Acceptance](#bl-220-system-and-platform) |
| Linux sysctl | `set_sysctl` | BL-355 | Post-1.0 / PLANNED | Linux high-risk allowlisted sysctl mutation / O | [Acceptance](#bl-220-system-and-platform) |
| Windows ADS | `list_file_streams`, `read_file_stream`, `write_file_stream`, `delete_file_stream` | BL-359 | Post-1.0 / PLANNED | Windows stream read/write with root and identity checks / O | [Acceptance](#bl-220-system-and-platform) |
| Linux xattr | `list_extended_attributes`, `read_extended_attribute`, `write_extended_attribute`, `delete_extended_attribute` | BL-359 | Post-1.0 / PLANNED | Linux attribute read/write with root and identity checks / O | [Acceptance](#bl-220-system-and-platform) |
| Git first-party Native Tool adapter | `get_git_status`, `get_git_diff`, `list_git_branches`, `list_git_commits`, `get_git_commit`, `get_git_blame` | BL-360 | Post-1.0 / PLANNED | Local authorized repository read and approved executable / O | [Acceptance](#bl-220-native-tools) |
| Audit administration | `get_audit_status`, `query_audit_events`, `verify_audit_integrity` | BL-361 | Post-1.0 / PLANNED | Administrative audit access / O | [Acceptance](#bl-220-system-and-platform) |
| User session | `list_clipboard_formats`, `read_clipboard`, `write_clipboard`, `clear_clipboard`, `send_desktop_notification` | BL-362 | Post-1.0 / PLANNED | Separate clipboard and notification permissions in a verified user session / O | [Acceptance](#bl-220-system-and-platform) |

Operations and Jobs (BL-084–099) provide internal lifecycle and storage.
BL-209–211 own MCP Tasks selection, mapping and fallback. Neither defines extra
public MCP tool names. Provider families such as SSH, HTTP/Web, containers,
desktop automation, cloud APIs and databases have no approved public names.
BL-181–188 must resolve provider contracts before any registration. A provider
that later proposes public tools supplies the same owner, milestone, platform,
capability and budget bindings as a built-in family.

## 21. Internal adapter inventory

### Protocol adapters

- MCP `2025-11-25`
- MCP `2026-07-28`
- future exact revisions as explicit new entries

### Transport adapters

- STDIO uses the current Go transport. BL-224 retains it during runtime wiring.
- Windows Named Pipe (BL-226) and Unix Domain Socket (BL-227) bind OS peer identity.
- Streamable HTTP (BL-370) has a separate Post-1.0 transport decision and
  connects directly to the shared Go core and BL-233 configuration.

Post-1.0 [BL-373](implementation-contract.md#bl-373-authorized-mcp-configuration-management)
owns configuration management across explicitly authorized direct, proxy, and
service transports. The eventual HTTP adapter may expose the same core
operations only after its own activation gates and an explicit effective
catalog decision. Optional MCP Apps clients use the same authorized API.
Public tool names and schemas belong to BL-373's later implementation planning.

### Host adapters

- Windows SCM host (BL-230) and Linux systemd host (BL-231).
- Per-user host (BL-232), with process-root ownership and shutdown under BL-341.

### Execution backends

- Current process and service-account backends use BL-236 context and BL-237
  least-privilege service identity.
- User-worker contract (BL-238) and Post-1.0 implementation (BL-240) use
  identity-bound native launch and supervised broker IPC.

### Platform adapters

- Windows/Linux filesystem through the existing Go filesystem and PathGuard
  interfaces. BL-102/107 own the root/link retrofit, with BL-066 native tests.
- Windows/Linux process APIs (BL-134) and execution isolation (BL-146/147).
- Windows ReadDirectoryChangesW-class / Linux inotify-class watch (BL-347).
- Windows/Linux system information (BL-062, BL-153–157) and network (BL-158).
- Windows Event Log / Linux journald (BL-356).
- Windows SCM / Linux systemd service control of other services (BL-357).
- Windows Task Scheduler / Linux systemd timers (BL-358).
- Windows Registry / Linux sysctl (BL-349/355).
- Windows security descriptors/ACL / Unix permission/ACL and Windows ADS /
  Linux xattr (BL-359).
- Windows Cloud Files/placeholder adapter (BL-345) and platform storage
  attribute support (BL-351).

### Built-in capability components

- Pure Go search baseline (BL-082) and optional root-scoped index decision (BL-083).
- Hashing/fingerprints (BL-048) and path compare/verify (BL-346).
- ZIP/TAR and standard-library compression codecs (BL-348/354).

### Native Tool adapters

- ripgrep accelerator (BL-081).
- Git read-only adapter (BL-360).
- 7-Zip/7zz or bsdtar-class candidates (BL-348), with any linked libarchive
  integration subject to its own dependency and native library review.
- Optional native codec adapters (BL-354).

### Provider and module candidates

BL-181–188 own the separate contract for SSH, Web/API, containers, package
managers, desktop automation, cloud and databases. BL-169 owns the security
boundary. None is an approved core backend, public tool or plugin today.

## BL-220 successor acceptance

Each canonical backlog row binds the applicable family below and its own
behavior. Each family's **Applicable owners** line records the scope of that
existing architecture contract. Use comma-separated BL IDs and inclusive
ranges. The public inventory records the required family for each planned
public surface. Keep these classifications consistent with the backlog's
acceptance links. The backlog owns task status, milestone and acceptance.
The owner may reuse the unchanged baseline source gate and fixtures.
The owner must add executable evidence for its own selection, policy,
platforms, negative cases and resource behavior before activation. Test
unavailable support, normalized failures, deterministic behavior and absence
of new authorization for all affected backend choices. The
[implementation binding](implementation-contract.md#bl-220-adapter-acceptance-binding)
specifies when to update these contracts and the source gate.

### BL-220 filesystem

**Applicable owners** BL-036–BL-061, BL-063–BL-067, BL-346

Use the Go filesystem abstraction and PathGuard for every path. Keep platform
identity, reparse and storage details behind that boundary. Test changed
operations against applicable path escapes, denied roots, races, partial
failure and local-only data. Exercise Windows/Linux native filesystem behavior
and bounded traversal, bytes, temporary state or result costs as applicable.
Do not delegate normal filesystem operations to OS commands.

### BL-220 search

**Applicable owners** BL-068–BL-080, BL-082–BL-083

Use BL-082 as the portable reference and PathGuard for traversed paths. Prove
equivalent filters, encoding and regex semantics, ordering and pagination,
permission failures, scan limits and cancellation before enabling an
accelerator. Use Go where equivalence is unproved, subject to the shared
fallback rule. Keep index state within its authorized context and lifecycle.

### BL-220 operations

**Applicable owners** BL-084–BL-099, BL-164, BL-210–BL-211, BL-218, BL-254

Run cancellable bounded Go work in-process where identity and enforcement allow.
When work requires a subprocess, dispatch through the single managed launch
boundary. Preserve domain authorization and lifecycle ownership. Exercise
limit exhaustion, denial, timeout, cancellation, result expiry, shutdown and
cleanup for affected work and prove bounded queues and temporary resources.

### BL-220 context and policy

**Applicable owners** BL-100–BL-111, BL-159–BL-161, BL-165–BL-168, BL-170, BL-217, BL-236, BL-239, BL-256, BL-352, BL-373

Reuse BL-159 authorization, BL-236 caller/backend separation, BL-166 audit,
BL-239 state binding and the applicable root/profile contracts. Keep discovery,
backend availability and cached results independent of authorization. Test the
affected direct calls, backend parity and context mismatch, including denial
before dispatch and secret-safe errors and audit.

### BL-220 managed execution

**Applicable owners** BL-113–BL-152, BL-162–BL-163, BL-252–BL-253, BL-353

Keep observation and control in bounded platform Go/API adapters and launch in
the centralized Managed Process and typed execution boundary. BL-163 owns
runtime policy before dispatch. BL-137/138/144 bind executable identity, argv
and environment. BL-134 owns the native components and BL-146/147 own the
platform enforcement controls. BL-151 integrates them and activates real
launch under the [staged activation contract](implementation-contract.md#managed-process-activation).
BL-151 confines launch to the exact reviewed central symbol, package,
platform and dependency boundary and adds bypass fixtures. BL-152 proves runtime rejection, effective
limits, cancellation and cleanup on both OSs. No allowlist flag, package-wide
exemption or second execution engine can substitute for those controls.

### BL-220 system and platform

**Applicable owners** BL-062, BL-153–BL-158, BL-345, BL-347, BL-349, BL-351, BL-355–BL-359, BL-361–BL-362

Use scoped Go/OS queries or direct native interfaces for supported semantics.
Use the existing authorization/context and per-operation bounds. Test absent or
denied APIs, safe redaction, native return/error normalization and unsupported
platform behavior. Separate observation from mutation, user-session authority
from service identity, and probe destinations from generic command egress.
Native executables, where separately justified, inherit the complete launch
boundary. Managed account, service and scheduler commands have no shell escape.

### BL-220 host and transport

**Applicable owners** BL-221–BL-235, BL-237–BL-238, BL-240–BL-244, BL-341, BL-370

Reuse the single native binary, transport-neutral core, process-root lifecycle
and OS-derived caller identity for local managed transports. Apply the
[BL-233 configuration contract](../adr/runtime-modes.md#configuration-contract)
at bootstrap and keep file selection separate from endpoint discovery.
Use the applicable native IPC, service-manager
and supervision APIs. Test unauthorized peers, absent endpoints, restart,
shutdown and effective identity; auto fallback cannot follow policy rejection.
Any same-binary worker launch must use an exact reviewed host role and image
binding under central lifecycle/launch control, with no caller-controlled
argv or alternate engine. BL-242 validates the actual artifact and enabled
runtime modes without installed interpreters, including service assets and
platform-specific paths, and extends the gate matrix when build inputs change.

### BL-220 native tools

**Applicable owners** BL-081, BL-348, BL-354, BL-360

Apply sections 3 and 4 and the operation-specific contract. Use the centralized
typed launch policy, enforce effective limits and preserve the Go/domain
baseline. Prove executable substitution, argument/config injection, helper
behavior, credential/network effects, unsupported behavior and fallback
before activation. The option matrix, differential tests and performance
evidence belong to each implementing owner.

### BL-220 provider boundary

**Applicable owners** BL-169, BL-176, BL-181–BL-188, BL-313

Retain the separate provider architecture and review decision. Treat provider
metadata, installation and trust labels as inputs to approval and never as
permissions. Define isolation, identity, resource caps, trust and credential
scope before loading or dispatch. Prove failures, denials and cleanup at the
boundary. No arbitrary scripts, dynamic plugins or new interpreter dependency
enter the trusted core through the provider mechanism.

## 22. Implementation gate for every new public tool

Use the shared [BL-213 result contract](implementation-contract.md#bl-213-tool-result-contract).
Each registered variant needs an executable wire fixture and a matching
structured schema. Bind it to the [BL-214 gate](implementation-contract.md#bl-214-wire-measurement-and-coverage)
for each effective profile and exact revision where it can execute.
The owning BL also applies the [BL-215 catalog and entry acceptance](implementation-contract.md#bl-215-catalog-and-entry-budget-binding)
to each affected effective catalog and route before activation.

Before a future tool is advertised, its owner must provide:

- final name/schema/result contract;
- capability/profile registration and negative direct-call authorization tests;
- root/path/identity policy where applicable;
- platform/adapter support and truthful unsupported behavior;
- hard operation/result/resource limits;
- redaction and platform-disclosure review;
- deterministic ordering/cursor behavior where applicable;
- state ownership/TTL/cleanup for handles;
- MCP annotations as hints only;
- actual whole-catalog, field, schema and entry budget measurements;
- semantic reference payloads, observed placement and hard BL-214 budgets for
  every material result variant or delivery change;
- Windows/Linux tests where both are in scope;
- [BL-220 adapter acceptance](implementation-contract.md#bl-220-adapter-acceptance-binding);
- documentation and current-vs-planned truth.

## Related authority

- [Backlog](../../BACKLOG.md)
- [Tool conventions](../tool-conventions.md)
- [Roadmap](../roadmap.md)
- [Architecture](../architecture.md)
- [Security model](../security.md)
- [Efficiency improvement plan](efficiency.md)
- [Version 1.0 boundary](release-scope.md)
