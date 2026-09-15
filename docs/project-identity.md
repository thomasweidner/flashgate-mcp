# FlashGate MCP Project Identity

## Name

The binding public project name is **FlashGate MCP**.

Tagline: **Fast, secure and local-first host operations for MCP.**

Working description: FlashGate MCP is a fast, secure, resource-efficient, local-first MCP server for controlled filesystem, process, and operating-system operations.

## Meaning

- Flash means low latency, efficient local processing, and compact responses.
- Gate means controlled access through policies, capabilities, roots, limits, redaction, and audit events.

FlashGate MCP is not a web-hosting service and not a remote-shell replacement. It is a controlled local boundary between MCP clients and explicitly enabled operating-system functions.

## Scope and non-goals

The current implemented product is a root-confined filesystem MCP server over
JSON-RPC STDIO. The Version 1.0 scope also includes accepted but not necessarily
implemented work for search, managed processes, typed allowlisted command
execution, scoped system information, and local multi-mode hosting. The
[Version 1.0 scope](version-1-scope-and-release-boundary.md) and
[canonical backlog](../BACKLOG.md) are authoritative for the status of those
domains; inclusion here is not an implementation or release claim.

Unrestricted host access, free-form remote shell behavior, implicit network exposure, and bypasses around central policies are non-goals.

## Open-source direction

The project is intended to be a general, vendor-neutral open-source project. Its core has no mandatory Voxtronic-specific paths, tools, proprietary systems, permissions, secrets, or infrastructure values.

Public, community, vendor, organization-internal, and Voxtronic-specific
**FlashGate modules/providers** may be considered later as optional local
project extensions. Provider origin never changes the central security
boundary. Provider contracts, identifier syntax, metadata, trust labels, and a
runtime model are post-Version-1.0 decisions (`BL-181` through `BL-188`); this
identity reference does not preselect them.

An **MCP protocol extension** is a separate negotiated addition to the MCP wire protocol and follows the official vendor-prefix/slash identifier contract, for example `io.modelcontextprotocol/tasks`. FlashGate modules/providers do not automatically define or implement MCP protocol extensions.

## Current technical identifiers

These values identify the current repository and executable contract. The
canonical build metadata values are recorded in the
[file and product metadata decisions](decisions/file-and-product-metadata-decisions.md).

| Item | Current value |
|---|---|
| Repository | `thomasweidner/flashgate-mcp` |
| Binary | `flashgate-mcp` |
| MCP server implementation name (`serverInfo.name`) | `flashgate` |
| Go module | `github.com/thomasweidner/flashgate-mcp` |
| Short name | FlashGate |
| Windows original filename | `flashgate-mcp.exe` |
| Windows PE product name | `FlashGate MCP` |
| Windows PE file description | `FlashGate MCP Server` |

The public product name, executable name, and MCP implementation name are
separate contracts. They must not be substituted for one another in runtime,
packaging, service, or client configuration.

## Planned identifiers and reserved decisions

Accepted architecture introduces local runtime modes named `stdio`, `proxy`,
`auto`, and `service`. Those mode names describe planned Version 1.0 behavior;
only the current implementation status in the backlog and product documentation
may be used to claim that a mode is available.

The Windows SCM **display name** is reserved as `FlashGate MCP`. It does not
rename the executable, MCP implementation, repository, or Go module. Exact SCM
service names, local endpoint names, configuration paths, and post-Version-1.0
provider identifiers remain owned by their respective open backlog decisions
and must not be inferred from the public name. See the
[native runtime and service plan](native-multi-mode-runtime-and-service-plan.md)
for the current/planned boundary.

## Transition record

The public identity was adopted before the technical rename. The repository,
binary, and module then moved to the FlashGate names, followed by the repository
owner migration to `thomasweidner`. The dated
[technical rename](technical-rename-to-flashgate-2026-07-11.md) and
[backlog migration](backlog-id-migration-2026-07-17.md) documents preserve that
history; historical identifiers are not aliases or current configuration
values.

Historical rename and owner-migration records remain project history; they do
not define current setup or compatibility requirements.
