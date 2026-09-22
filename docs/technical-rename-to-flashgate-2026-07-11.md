# Technical Rename to FlashGate MCP

Date: 2026-07-11
Sprint: 3.42 – Technical project rename to FlashGate MCP
Status: Historical rename record with current-owner amendment dated 2026-07-20

> This document records the technical rename performed on 2026-07-11. Repository ownership was later migrated from `blacksheepkhan` to `thomasweidner`. Owner-specific commands and paths in the historical procedure below are evidence of the completed migration, not current setup instructions.

The public project name was established as FlashGate MCP in Sprint 3.41. The GitHub repository rename, Git remote update, and local folder rename were completed manually before this code rename. Sprint 3.42 completes the corresponding repository-internal technical rename.

| Area | Old | New |
| --- | --- | --- |
| Public project | Fileserver MCP | FlashGate MCP |
| GitHub repository | `blacksheepkhan/fileserver-mcp` | `blacksheepkhan/flashgate-mcp` |
| Local folder | `fileserver-mcp` | `flashgate-mcp` |
| Go module | `github.com/blacksheepkhan/fileserver-mcp` | `github.com/blacksheepkhan/flashgate-mcp` |
| Binary | `fileserver-mcp` | `flashgate-mcp` |
| Windows binary | `fileserver-mcp.exe` | `flashgate-mcp.exe` |
| MCP implementation name | `fileserver-mcp` | `flashgate` |
| Release artifacts | legacy fileserver names | FlashGate artifact names |

## Current identifiers

- Public project name: FlashGate MCP
- Binary: `flashgate-mcp`
- MCP server implementation name (`serverInfo.name`): `flashgate`
- Go module: `github.com/thomasweidner/flashgate-mcp`
- Repository: `thomasweidner/flashgate-mcp`
- Active local project path: `C:\Voxtronic\MCP\flashgate-mcp`

## Existing clones and GitHub redirects

The former repository paths are historical identifiers and must not be used as current installation paths. Existing clones should use the current repository owner:

```text
git remote set-url origin https://github.com/thomasweidner/flashgate-mcp.git
```

Current clones use:

```text
git clone https://github.com/thomasweidner/flashgate-mcp.git
cd flashgate-mcp
```

## Manual GitHub rename procedure

The GitHub repository, Git remote, GitHub CLI default repository, and local folder were already successfully renamed before the Sprint 3.42 internal technical rename. This section records the completed 2026-07-11 migration as historical evidence; it is not a current repeatable procedure or an outstanding task. The internal technical rename was then performed separately on the Sprint 3.42 feature branch. Neither this procedure nor that internal rename changes MCP tool names or `MCP_*` environment variables.

### Historical prerequisites - 2026-07-11

- Start from a clean working tree on `main` at the current `origin/main` state.
- Ensure GitHub CLI is authenticated, the target `blacksheepkhan/flashgate-mcp` has been checked, and the current repository name and remote URL have been recorded.
- Do not proceed while critical work is running or local changes are open.

### Historical recorded commands and action

The following commands record the successfully executed rename sequence and its verification:

```powershell
gh repo view blacksheepkhan/flashgate-mcp
gh repo rename flashgate-mcp --yes
git remote set-url origin https://github.com/blacksheepkhan/flashgate-mcp.git
gh repo set-default blacksheepkhan/flashgate-mcp
git remote -v
gh repo view --json nameWithOwner,url,defaultBranchRef
git fetch origin
git status --short --branch
```

At that time, the local project folder was renamed or moved to:

```text
<local-workspace>\flashgate-mcp
```

After those manual steps, create or use the dedicated Sprint 3.42 feature branch for the separate internal technical rename.

### Historical verification - 2026-07-11

- `gh repo view` reports `blacksheepkhan/flashgate-mcp` and default branch `main`.
- `git remote -v` reports `https://github.com/blacksheepkhan/flashgate-mcp.git`, and `git fetch origin` succeeds.
- `main` follows `origin/main`; the new Git path is reachable and the former Git path redirects through GitHub.
- Commit, pull request, and Actions history remain available, and the local path matches the target path above.

### Failure handling and rollback

- If the GitHub rename fails, do not begin local technical rename steps.
- If only the local remote URL is wrong, correct it to the URL GitHub reports.
- If the local folder cannot be moved, close applications and terminals holding handles, then retry.
- For a serious problem, rename the GitHub repository back in its repository settings while the former name is still available. Then restore the remote URL, local folder, and documentation consistently.
- Do not use force-pushes, history rewrites, or replacement repositories as rollback. After a successful migration, do not reuse the former repository name for another repository, because doing so can impair GitHub redirect behavior.

## Owner migration amendment - 2026-07-20

Repository ownership was migrated from `blacksheepkhan/flashgate-mcp` to `thomasweidner/flashgate-mcp`. The Go module was correspondingly migrated to `github.com/thomasweidner/flashgate-mcp`, and the active Windows project path is `C:\Voxtronic\MCP\flashgate-mcp`.

The owner migration did not change the binary name, `serverInfo.name`, MCP tool contracts, `MCP_*` environment variables, commit history, pull-request history, or release history. Earlier owner-specific commands and paths in this document remain as historical evidence of the 2026-07-11 rename.

## Compatibility and scope

This sprint makes no functional change. The existing `MCP_*` environment variables remain unchanged, including `MCP_ROOT` and `MCP_READ_ONLY`. Existing tool names and schemas remain unchanged until Sprint 3.43. Root-default behavior and all security policies are unchanged.

`cmd/server` remains the generic internal command path. It is not a legacy product identifier and was intentionally not moved.

Historical ADRs and changelog entries retain the old name where it records the context at the time. Those historical references, this migration note, and redirect guidance are the only intended remaining references to the legacy identifier.
