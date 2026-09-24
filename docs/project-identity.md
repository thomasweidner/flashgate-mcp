# FlashGate MCP Project Identity

## Name

The binding public project name is **FlashGate MCP**.

Tagline: **Fast, secure and local-first host operations for MCP.**

Working description: FlashGate MCP is a resource-efficient, cross-platform MCP server for controlled local host operations. Today it provides secure, root-confined filesystem access; Version 1.0 targets bounded search, managed processes, typed command execution, and system information.

## Meaning

- Flash means low latency, efficient local processing, and compact responses.
- Gate means controlled access through policies, capabilities, roots, limits, redaction, and audit events.

FlashGate MCP is not a web-hosting service and not a remote-shell replacement. It is a controlled local boundary between MCP clients and explicitly enabled operating-system functions.

## Scope and non-goals

The current implemented scope is a root-confined filesystem MCP server. Accepted future domains include search, managed processes, controlled execution, and system information.

Unrestricted host access, free-form remote shell behavior, implicit network exposure, and bypasses around central policies are non-goals.

## Open-source direction

FlashGate is a general, vendor-neutral open-source project. Its core has no organization-specific infrastructure, private control-plane, machine-local workflow, proprietary credential or permission, or particular agent-product prerequisite.

Public, community, vendor, and organization-internal **FlashGate modules/providers** may be considered later as optional local project extensions. Provider origin never changes the central security boundary. `SPR-041` defines no module/provider contract, identifier syntax, or runtime model.

An **MCP protocol extension** is a separate negotiated addition to the MCP wire protocol and follows the official vendor-prefix/slash identifier contract, for example `io.modelcontextprotocol/tasks`. FlashGate modules/providers do not automatically define or implement MCP protocol extensions.

## Technical identifiers

Current identifiers are:

| Item | Current value |
|---|---|
| Repository | `thomasweidner/flashgate-mcp` |
| Binary | `flashgate-mcp` |
| MCP server implementation name (`serverInfo.name`) | `flashgate` |
| Go module | `github.com/thomasweidner/flashgate-mcp` |
| Short name | FlashGate |

Historical rename and owner-migration records remain project history; they do
not define current setup or compatibility requirements.
