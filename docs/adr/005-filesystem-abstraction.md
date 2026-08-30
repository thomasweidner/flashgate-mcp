# ADR-005: Filesystem Abstraction

## Status

Accepted

## Context

`fileserver-mcp` exposes filesystem operations to MCP clients.

Direct filesystem access is security-sensitive. If filesystem APIs are used across multiple packages, security checks may become inconsistent or be bypassed accidentally.

The project must support:

- Windows
- Linux
- sandboxed filesystem access
- future read-only mode
- future audit logging
- future search functionality
- unit testing without touching real user files

## Decision

All production filesystem operations must go through a central `FileSystem` abstraction in:

```text
internal/fs
```

The main interface is:

```go
type FileSystem interface {
    List(path string) ([]Entry, error)
    Read(path string, maxBytes int64) ([]byte, error)
    Stat(path string) (Metadata, error)
    Exists(path string) (bool, error)
    Write(path string, content []byte, overwrite bool) error
    Mkdir(path string) error
    Delete(path string, recursive bool) error
    Move(source string, target string, overwrite bool) error
    Copy(source string, target string, overwrite bool) error
    Rename(source string, target string, overwrite bool) error
}
```

`internal/fs` is the only package that may directly use operating system filesystem calls in production code.

All paths are validated through `internal/security.PathGuard`.

## Consequences

### Positive

- security checks are centralized
- MCP tools remain simple
- filesystem behavior is easier to test
- future implementations can provide an in-memory filesystem or mock filesystem
- audit logging can be added in one place
- read-only mode can be enforced centrally

### Negative

- initial implementation requires more structure
- simple tools must depend on an abstraction rather than calling `os.*` directly
- some operations require careful policy decisions, such as overwrite and recursive delete behavior

## Security Impact

This decision significantly reduces the risk of accidental sandbox bypasses.

All MCP tools must use the `FileSystem` interface instead of direct filesystem access.

## Current Implementation

The current local filesystem implementation is:

```text
internal/fs/LocalFileSystem
```

It supports:

- list
- read
- stat
- exists
- write
- mkdir
- delete
- move
- copy
- rename

Directory copy is intentionally unsupported in the initial implementation.

## Amendment - 2026-07-10

The public project name is FlashGate MCP; the technical rename remains planned for Sprint 3.42. Read-only tool registration, centralized limits, and redacted stderr diagnostics are now implemented. The current `copy_path` tool copies files only; directory copy remains planned work. See ADR-006 through ADR-013 for the current architecture, security, and MCP compatibility direction.

## Amendment - 2026-07-11

Sprint 3.43 removes the redundant `FileSystem.Exists` method and the `FileSystem.Rename` alias. A single `Stat` path normalizes genuine absence for `get_path_info`, avoiding an Exists-then-Stat race, while `Move` is the domain operation for both move and rename.

Directory creation now reports whether the leaf was actually created while preserving parent creation. Move validates same path, effective path, `os.SameFile`, Windows case aliases, overwrite type combinations, directory self-subtrees, changed path identities, and same-volume support before replacement. Existing files are replaced by rename without a separate target deletion. Cross-volume moves are rejected without copy/delete fallback. These changes retain centralized PathGuard enforcement and do not add directory copy.

## Implementation Amendment - 2026-07-11

Sprint 3.44 makes `MCP_ROOT` mandatory and requires absolute production roots. Missing, empty, whitespace-only and general relative roots fail closed. `MCP_ROOT=.` is development-only and requires exact `MCP_ALLOW_CWD_ROOT=true`; no other relative root is enabled.

Root preflight verifies existence, current policy, canonical/effective resolution and directory type before constructing the Filesystem, tool Registry, Router or STDIO server. Expected root/configuration failures leave stdout empty, use safe stderr categories and exit code 3. This implements the existing centralized PathGuard decision without introducing named roots or a second filesystem boundary.
