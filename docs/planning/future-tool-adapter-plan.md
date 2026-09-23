# Future Tool and Adapter Plan

**Status:** Accepted planning contract. `BACKLOG.md` owns canonical task status, milestone, and implementation ownership. This document owns the detailed public naming, capability, adapter, security, and efficiency contract referenced by those owners. Implementations must read both their canonical backlog row and the relevant sections of this document. Nothing in this document advertises an unimplemented MCP tool.

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

The current eight filesystem tools remain the implemented baseline until their respective owners change runtime behavior. Future names below are accepted target names only when their backlog owner is `Planned` or `Later` and remain absent from `tools/list` until implemented and authorized by the effective profile.

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

### 3.1 Core domain operation

Owns semantic behavior, policy inputs, result shape, limits, and domain errors. Core code remains independent of MCP/JSON-RPC and of a specific OS backend.

### 3.2 Platform adapter

Implements equivalent FlashGate semantics using OS-specific APIs or stable system interfaces. Examples: filesystem, process, Windows Event Log, journald, SCM, systemd, Registry, sysctl, ReadDirectoryChangesW, inotify, Cloud Files, Windows security descriptors, Linux xattr.

### 3.3 Native Tool adapter

Uses an explicitly approved external native binary behind a closed typed FlashGate contract. It never accepts raw command-line fragments or gains authority beyond the calling FlashGate capability. Examples: ripgrep, Git, 7-Zip/7zz, or a reviewed archive/codec utility.

Every Native Tool adapter requires:

- an approved executable identity and known resolution path;
- no shell;
- typed arguments generated by FlashGate;
- minimal environment;
- bounded timeout, output, concurrency, and working directory;
- root binding for path arguments;
- rejection of response-file, hook, plugin, config, loader, pager, external-diff, or equivalent injection unless explicitly modeled and approved;
- benchmark/security evidence against the native baseline where it acts as an accelerator;
- no raw adapter errors, binary path, provider identity, or version leakage in normal client results.

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

The exact policy classes are an implementation decision, but path allowlisting alone must not be described as cryptographic binary identity.

### 4.4 Resource bounds

Every expensive operation defines server maxima for the dimensions it can amplify: entries, depth, scanned bytes, read bytes, written bytes, expanded bytes, compression ratio, events, matches, output, runtime, concurrent work, temporary data, handles, and result TTL. Client-requested limits can only reduce, never increase, server caps.

### 4.5 Stateful handles

Watch handles, process handles, jobs, cursors, result resources, caches, and other state are bound to the applicable caller principal, profile, root, execution backend, service generation, protocol context, and expiry. A service restart invalidates stale generation-bound state.

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
- `storage`: compression supported/enabled/default, encryption supported/enabled/default, sparse state, logical/allocated size where reliable;
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

Default results remain compact (`verified`, checked count, mismatch count, indeterminate count). Mismatch details are bounded and returned only as needed/requested. A statement that something "changed" requires a supplied or server-owned baseline; absence of a baseline is not inferred as change.

For cloud/local-only content, `unknown`/`indeterminate` must not be silently converted to `different` when equality cannot be proven without forbidden hydration.

## 6. Version 1.0 search and execution corrections

### 6.1 Search — `BL-068`–`BL-083`

Canonical public tools:

- `search_paths` — path/name/metadata search;
- `search_text` — literal/regular-expression content search.

Pure Go is the normative portable baseline (`BL-082`). The optional ripgrep adapter (`BL-081`) is cross-platform and may accelerate `search_text` only when security, semantics, deterministic result requirements, and benchmark evidence permit it. It never creates a public `ripgrep_*` tool. Raw ripgrep options, preprocessors, and helper-spawning archive search are outside the adapter contract unless separately modeled and approved.

### 6.2 Managed process and typed commands — `BL-119`–`BL-152`, `BL-353`

`start_process(command_id, typed_args)` and `run_command(command_id, typed_args)` use the same Managed Process / Typed Command engine. `run_command` is only the synchronous wrapper; FlashGate has no second arbitrary execution engine.

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

Later candidate public tool:

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

Built-in Go/OS paths are preferred. Optional 7-Zip/7zz or libarchive/bsdtar-class Native Tool adapters may extend supported formats only after the common contract and `BL-220` gate pass.

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

Query state through `get_path_info(fields=["storage"])`; there is no separate `get_path_storage_info`. Where reliably supported, normalized requested storage fields may include compression supported/enabled/inherited-default, encryption supported/enabled/inherited-default, sparse state, logical size, and allocated/physical size. Unknown/unsupported fields are explicit and do not fabricate cross-filesystem equivalence.

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

The optional public FlashGate skill teaches portable use of the released contracts without private governance dependencies. It should cover batch-first calls, ranges/fields/pages, fingerprints/verify, avoiding redundant metadata calls, resource handles, roots/profiles/capabilities, annotation-vs-authorization, synchronous/jobs behavior, and protocol/feature discovery. It never becomes an authorization mechanism or a replacement for concise server instructions.

## 20. Public tool inventory by target

This inventory is a planning contract, not a current-runtime claim.

### Root and command discovery

- `list_roots`
- `get_root_info`
- `list_commands`
- `get_command_info`

### Path/filesystem

- `get_path_info`
- `get_paths_info`
- `hash_paths`
- `compare_paths`
- `verify_paths`
- `copy_path`
- `move_path`
- `delete_path`
- `apply_filesystem_plan`
- `get_path_security`
- `set_path_security`

### Directory

- `list_directory`
- `get_directory_size`
- `create_directory`

### File

- `read_file`
- `read_files`
- `write_file`
- `edit_file`

### Search

- `search_paths`
- `search_text`

### Path watch

- `start_path_watch`
- `read_path_watch_events`
- `stop_path_watch`

### Archive

- `list_archive_formats`
- `get_archive_info`
- `list_archive_entries`
- `read_archive_entry`
- `verify_archive`
- `create_archive`
- `extract_archive`

### Content compression

- `list_compression_formats`
- `compress_file`
- `decompress_file`

### Storage/path attributes

- `set_path_compression`
- `set_path_encryption`
- later cloud candidate `set_path_availability`

### Process observation

- `list_processes`
- `get_process_info`
- `get_process_tree`

### Managed process

- `start_process`
- `wait_process`
- `read_process_output`
- `stop_process`
- later `write_process_input`
- later high-risk `stop_external_process` candidate under `BL-127`

### Typed execution

- `run_command`

### System

- `get_system_info`
- `get_disk_usage`
- `get_environment_info`

### Network

- `get_network_info`
- `list_network_connections`
- `resolve_host`
- `test_tcp_connection`

### System logs

- `list_system_log_sources`
- `query_system_logs`

### Services

- `list_services`
- `get_service_info`
- `start_service`
- `stop_service`
- `restart_service`

### Scheduled jobs

- `list_scheduled_jobs`
- `get_scheduled_job_info`
- `create_scheduled_job`
- `update_scheduled_job`
- `delete_scheduled_job`
- `enable_scheduled_job`
- `disable_scheduled_job`
- `run_scheduled_job`

### Windows Registry

- `list_registry_keys`
- `list_registry_values`
- `get_registry_value`
- `create_registry_key`
- `set_registry_value`
- `delete_registry_value`
- `delete_registry_key`

### Linux sysctl

- `list_sysctls`
- `get_sysctl`
- `set_sysctl`

### Windows ADS

- `list_file_streams`
- `read_file_stream`
- `write_file_stream`
- `delete_file_stream`

### Linux xattr

- `list_extended_attributes`
- `read_extended_attribute`
- `write_extended_attribute`
- `delete_extended_attribute`

### Git first-party Native Tool adapter

- `get_git_status`
- `get_git_diff`
- `list_git_branches`
- `list_git_commits`
- `get_git_commit`
- `get_git_blame`

### Audit administration

- `get_audit_status`
- `query_audit_events`
- `verify_audit_integrity`

### User session

- `list_clipboard_formats`
- `read_clipboard`
- `write_clipboard`
- `clear_clipboard`
- `send_desktop_notification`

## 21. Internal adapter inventory

### Protocol adapters

- MCP `2025-11-25`
- MCP `2026-07-28`
- future exact revisions as explicit new entries

### Transport adapters

- STDIO
- Windows Named Pipe
- Unix Domain Socket

### Host adapters

- Windows SCM host
- Linux systemd host

### Execution backends

- current-process
- service-account
- future user-worker

### Platform adapters

- Windows/Linux filesystem
- Windows/Linux process
- Windows/Linux execution isolation/resource enforcement
- Windows ReadDirectoryChangesW-class / Linux inotify-class path watch
- Windows/Linux system information
- Windows Event Log / Linux journald
- Windows/Linux network
- Windows SCM / Linux systemd service control
- Windows Task Scheduler / Linux systemd timers
- Windows Registry / Linux sysctl
- Windows security descriptors/ACL / Unix permission/ACL
- Windows ADS / Linux xattr
- Windows Cloud Files/placeholder adapter

### Built-in capability components

- Pure Go search baseline
- hashing/fingerprints
- path compare/verify
- ZIP/TAR and standard-library archive/compression codecs where applicable

### Native Tool adapters

- ripgrep accelerator
- Git read-only adapter
- 7-Zip/7zz archive/codec adapter
- libarchive/bsdtar-class candidate where evidence justifies it

## 22. Implementation gate for every new public tool

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
- catalog/schema/instruction budget measurement;
- wire/payload efficiency measurement for material results;
- Windows/Linux tests where both are in scope;
- no-interpreter/native-adapter gate;
- documentation and current-vs-planned truth.

## Related authority

- [Backlog](../../BACKLOG.md)
- [Tool conventions](../tool-conventions.md)
- [Roadmap](../roadmap.md)
- [Architecture](../architecture.md)
- [Security model](../security.md)
- [Efficiency improvement plan](../efficiency-improvement-plan.md)
- [Version 1.0 boundary](../version-1-scope-and-release-boundary.md)
