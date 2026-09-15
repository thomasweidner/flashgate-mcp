# Manual metadata validation

## Scope

Most file and product metadata checks are automated. This document defines the remaining visual or operator-facing checks that cannot be represented completely by unit tests or binary parsers.

## Windows Explorer property sheet

Use an x64 validation or release binary produced by the controlled build path.

1. Open the directory containing `flashgate-mcp.exe`.
2. Open **Properties**.
3. Select **Details**.
4. Confirm these values:

| Explorer field | Expected value |
|---|---|
| File description | `FlashGate MCP Server` |
| Type | Application |
| File version | numeric `MAJOR.MINOR.PATCH.0` |
| Product name | `FlashGate MCP` |
| Product version | complete SemVer, including prerelease suffix |
| Copyright | `Copyright © <source-year> Thomas Weidner` |
| Company | `Thomas Weidner` |
| Original filename | `flashgate-mcp.exe` |
| Internal name | `flashgate-mcp` |
| Comments | `Native Model Context Protocol server for controlled local system access.` |

Also confirm that Explorer displays the FlashGate lightning icon for the executable.

The automated companion check is:

```powershell
& {
    & ./scripts/Test-WindowsMetadata.ps1 `
        -BinaryPath <path-to-flashgate-mcp.exe> `
        -ExpectedProductVersion <version> `
        -ExpectedFileVersion <major.minor.patch.0> `
        -ExpectedPublicArch x64 `
        -ExpectedGOARCH amd64 `
        -ExpectedCommit <full-40-character-sha> `
        -ExpectedSourceTime <yyyy-MM-ddTHH:mm:ssZ> `
        -ExpectedModified false `
        -ExpectedCopyrightYear <year>
}
```

The automated test also validates the embedded build manifest and the
normalized identity of every icon frame against the committed ICO. The
completion script opens the generated validation binary, prints its complete
absolute path immediately before the confirmation prompt, and records the
operator confirmation in its report. The visual confirmation is not a
substitute for the automated test.

## Linux presentation

Linux desktop file managers do not expose a standard, portable product-version property sheet comparable to Windows Explorer. No visual desktop-file-manager field is therefore a release requirement.

Use these commands instead:

```bash
./flashgate-mcp --version
./flashgate-mcp --version --verbose
go version -m ./flashgate-mcp
go tool buildid ./flashgate-mcp
file ./flashgate-mcp
readelf -h ./flashgate-mcp
readelf -n ./flashgate-mcp
```

Expected evidence includes:

- `FlashGate MCP` product identity through verbose CLI output
- complete SemVer and numeric file-version mapping
- public `linux/x64` or `linux/arm64`
- internal `linux/amd64` or `linux/arm64`
- full Git revision and canonical UTC source time
- Go module and VCS settings
- `.note.go.buildid`
- matching ELF machine architecture

Run the version and help commands only when the artifact architecture matches
the native host architecture. On the currently implemented x64 validation
hosts, ARM64 artifacts are cross-built and inspected statically; the commands
above are not ARM64 execution evidence. Native Windows ARM64 and Linux ARM64
execution remains a future, conditional check on matching ARM64 hosts.

Extended attributes are intentionally not used because they are not portable and are frequently lost during copying or archive extraction.

Package-manager and systemd presentation checks are not applicable while `.deb`, `.rpm`, and systemd integration remain deferred.

## Recording the result

The manual validation record must include:

- binary path
- tested product version
- tested architecture
- automated metadata-test result
- operator confirmation
- date
- remaining limitations

Repeat the visual check against the final x64 integration artifact after all automated validation gates pass.
