# Future Tool and Adapter Plan

**Status:** Accepted planning inventory; `BACKLOG.md` owns task status and implementation. All future names below are candidates until their owners define and validate contracts. The eight baseline MCP filesystem tools remain the current implementation. This plan adds no runtime tool, profile, capability, adapter, or release requirement.

## Ownership and milestone

`BL-215` maintains this cross-domain inventory as input to catalog size, profile composition, deterministic ordering, and naming review. Each implementation owner retains its own acceptance boundary. All seven new owners are `Later` (post-Version-1.0). Existing Version 1.0 primitives remain governed by their own `Planned` rows. A new capability cannot be advertised until its owner, security gate, protocol/schema tests, docs, and measured catalog budget pass.

## Capability and naming matrix

`Read/write` describes operation effect, not authorization. Capabilities below are semantic candidates, subject to `BL-100`/`BL-108` profile design. `Sync/job` indicates expected bounded execution form; a job uses the shared Operations/Job lifecycle without transferring business ownership. `Win/Linux` means both platforms are in scope; OS-specific fields may be absent with an explicit supported/unsupported result.

| Tool name or candidate | Domain | Core operation | MCP tool | Adapter | Read/write; capability/risk | Sync/job | Platform | Milestone; owner |
|---|---|---|---|---|---|---|---|---|
| `list_directory` | Filesystem | List | Current | MCP filesystem | Read; filesystem.read | Sync/page | Win/Linux | Current |
| `read_file` | Filesystem | Read | Current | MCP filesystem | Read; filesystem.read | Sync/range or resource | Win/Linux | Current |
| `get_path_info` | Filesystem | Stat | Current; extend fields | MCP filesystem | Read; filesystem.read | Sync | Win/Linux | Current; BL-043, BL-345, BL-351 |
| `write_file`, `create_directory`, `delete_path`, `copy_path`, `move_path` | Filesystem | Write/Mkdir/Delete/Copy/Move | Current | MCP filesystem | Write; filesystem.write, destructive where applicable | Sync, later job for bounded long work | Win/Linux | Current |
| `get_paths_info` | Filesystem | Batch Stat | Candidate | MCP filesystem | Read; filesystem.read | Sync/page | Win/Linux | Version 1.0; BL-047 |
| `hash_paths` | Filesystem | Batch Hash/Fingerprint | Candidate | MCP filesystem | Read; filesystem.read, I/O cost | Sync/job | Win/Linux | Version 1.0; BL-048 |
| `get_directory_tree` | Filesystem | Tree inventory | Candidate | MCP filesystem | Read; filesystem.read, traversal cost | Sync/job/page | Win/Linux | Version 1.0; BL-049 |
| `edit_file`, `append_file` | Filesystem | Targeted edit/append | Candidates | MCP filesystem | Write; filesystem.write, conflict risk | Sync | Win/Linux | Version 1.0; BL-050–BL-055 |
| `apply_filesystem_plan` | Filesystem | Bounded operation plan | Candidate | MCP filesystem | Write; filesystem.write, destructive/multi-step risk | Sync/job | Win/Linux | Version 1.0; BL-056–BL-060 |
| `get_directory_size`, `get_disk_usage` | Filesystem/system | Scoped size/capacity | Candidates | MCP filesystem/system | Read; filesystem.read/system.read, scan/host-detail risk | Sync/job | Win/Linux | Version 1.0; BL-061–BL-062, BL-154 |
| `search_paths`, `search_content` | Search | Bounded path/name/metadata or text search | Candidates | MCP search | Read; search.execute, scan/disclosure risk | Sync/job/page | Win/Linux | Version 1.0; BL-068–BL-080, BL-082 |
| `list_processes`, `get_process_info`, `get_process_tree` | Process | Observe process inventory/state/tree | Candidates | MCP process | Read; process.observe, privacy risk | Sync/page | Win/Linux | Version 1.0; BL-114–BL-118 |
| `start_process`, `wait_process`, `read_process_output`, `stop_process` | Process | Managed lifecycle/output | Planned names for start/wait/read/stop | MCP process | Manage/read; process.manage, execution risk | Job/handle/page | Win/Linux | Version 1.0; BL-119–BL-126 |
| `run_command` | Execution | Run typed allowlisted command | Planned name | MCP execution | Execute; command.execute, high risk | Sync over managed engine | Win/Linux | Version 1.0; BL-136–BL-152 |
| `system_info`, filtered environment query | System information | Scoped host facts | `system_info` planned; other name undecided | MCP system | Read; system.read, privacy risk | Sync/fields | Win/Linux | Version 1.0; BL-153–BL-157 |
| `compare_paths` | Filesystem | Compare file/file or tree/tree | Candidate | MCP filesystem | Read; filesystem.read, scan cost | Sync/job/page | Win/Linux | Later; BL-346 |
| `verify_path` | Filesystem | Check expected state/hash/inventory | Candidate | MCP filesystem | Read; filesystem.read, scan cost | Sync/job/page | Win/Linux | Later; BL-346 |
| `watch_paths` | Filesystem | Watch root-confined events | Candidate; distinct from process watch | MCP filesystem | Read; explicit watch capability, state/resource risk | Job/handle/page | Win/Linux | Later; BL-347 |
| `inspect_archive` | Filesystem | List archive entries safely | Candidate | MCP filesystem; format adapter | Read; filesystem.read, expansion risk | Sync/job/page | Win/Linux | Later; BL-348 |
| `create_archive` | Filesystem | Package selected paths | Candidate | MCP filesystem; format adapter | Write; filesystem.write, resource risk | Job | Win/Linux | Later; BL-348 |
| `extract_archive` | Filesystem | Extract into scoped root | Candidate | MCP filesystem; format adapter | Write; filesystem.write, destructive/escape risk | Job | Win/Linux | Later; BL-348 |
| `get_os_settings` | System information | Read allowlisted semantic keys | Candidate | MCP system; Registry/sysctl/xattr backends | Read; system.read plus field policy | Sync/page | Win/Linux where defined | Later; BL-349 |
| `set_path_compression` | Filesystem | Change scoped path compression | Candidate pending naming/security decision | MCP filesystem; platform adapter | Write; explicit filesystem mutation capability, high risk | Sync/job | Supported Win/Linux semantics only | Later; BL-351 |

The portable FlashGate agent skill (`BL-350`) is a public documentation/skill artifact, not an MCP tool or core operation. Cloud/placeholder semantics (`BL-345`) primarily extend existing path inspection and operation policy; they do not imply a cloud API tool or a dependency on any vendor. Process watch, if ever proposed, belongs to the Process domain and must use a name that cannot be confused with `watch_paths`.

## Naming and boundary rules

- Use short, agent-readable snake-case verbs and nouns. Use `get` for metadata or state and `read` for file content; retain `get_path_info` as the one path-metadata query. Do not restore `get_directory` or `get_directories` aliases for `list_directory`.
- `compare_paths` must handle both files and trees; `verify_path` must express expected hash, size/type, changed state, tree parity, and optional metadata without creating another hash engine. Exact pluralization is settled with schemas by `BL-346`.
- Filesystem `watch_paths` names events on paths. Process observation/monitoring uses an explicit Process-domain name if later planned; neither name is interchangeable.
- Archive verbs are `inspect`, `create`, and `extract`, with one format-neutral model and bounded adapters. Do not expose raw archive entry paths as authorized destination paths.
- Avoid vague context names such as `get_access_context`; state exactly which allowlisted setting or metadata is returned. Do not add a general settings editor.
- Keep core methods and OS backends free of MCP/JSON-RPC types. MCP names, schemas, profile exposure and feature detection stay in the adapter; OS differences stay in platform/format adapters.

## Cross-cutting acceptance before a future tool ships

Every owner must bind root-relative inputs, effective path confinement, server-side capability checks, execution identity, limits, redaction, deterministic cleanup, and negative tests. Results use bounded fields, ranges/pages/cursors or identity-bound resource/job handles. Catalog/schema/annotation descriptions are discovery hints, never authorization. Measure tool-list, instruction and wire cost; preserve current/planned/later truth. Native Go/OS implementation has priority; an external program needs the existing typed no-shell security and benchmark gate, and an interpreter is not a normal runtime dependency.

## Related authority

- [Backlog](../../BACKLOG.md)
- [Tool conventions](../tool-conventions.md)
- [Roadmap](../roadmap.md)
- [Architecture](../architecture.md)
- [Security model](../security.md)
- [Version 1.0 boundary](../version-1-scope-and-release-boundary.md)
