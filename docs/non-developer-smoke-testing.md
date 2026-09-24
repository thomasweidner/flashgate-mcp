# Non-Developer Smoke Testing

This guide verifies a prebuilt FlashGate MCP binary through its public STDIO
boundary. It is intended for operators and reviewers who do not need to edit or
compile Go source. The binary must already exist at `build/flashgate-mcp.exe`
on Windows or `build/flashgate-mcp` on Linux.

Run the commands from the repository root. Use PowerShell 7.6.5 on Windows and
Bash plus Python 3 on Linux. The scripts set the test root themselves, create
only isolated fixtures below `build/`, and remove their per-run request,
response, and fixture files on exit.

## Windows

```powershell
.\scripts\smoke-jsonrpc.ps1

$env:MCP_READ_ONLY = "true"
try {
    .\scripts\smoke-jsonrpc.ps1
} finally {
    Remove-Item Env:\MCP_READ_ONLY -ErrorAction SilentlyContinue
}

.\scripts\smoke-jsonrpc-negative.ps1
.\scripts\smoke-startup-negative.ps1
```

Each command must exit with code `0` and print, respectively:

```text
JSON-RPC smoke test passed.
JSON-RPC smoke test passed.
Negative JSON-RPC smoke test passed.
Startup negative smoke test passed.
```

## Linux

Before running the suite, confirm that the binary is executable:

```bash
test -x build/flashgate-mcp
bash scripts/smoke-jsonrpc.sh
MCP_READ_ONLY=true bash scripts/smoke-jsonrpc.sh
bash scripts/smoke-jsonrpc-negative.sh
bash scripts/smoke-startup-negative.sh
```

The four scripts must exit with code `0` and print the same four pass messages
shown for Windows.

## What the results prove

- The default smoke validates initialization, the exact default tool catalog,
  representative read operations, missing-path behavior, and a write-profile
  move through real JSON-RPC STDIO.
- The read-only smoke validates the exact read-only catalog and confirms that
  all write-capable tool names are rejected without changing its fixtures.
- The negative smoke validates parse, unknown-method, invalid-parameter,
  notification, and removed-tool behavior without exposing internal details.
- The startup-negative smoke validates fail-closed root and configuration
  handling, safe categorized stderr, expected exit codes, and empty protocol
  stdout on startup failures.

A pass is focused local evidence for the tested binary and host only. It does
not establish release approval, native coverage for another operating system,
or completion of Windows finalization.

## Failure handling

Treat any nonzero exit, missing pass message, unexpected file mutation, protocol
output on startup-failure stdout, or host-path detail in a diagnostic as a
failure. Preserve the terminal output and the binary identity (path and
checksum) for diagnosis. Do not weaken a check or retry with a broader root to
make a failure pass.

If a script reports that the binary is missing, obtain the already approved
artifact for the current source revision. Building from source is a developer
workflow and is intentionally outside this non-developer procedure.

See [Testing](testing.md) for the complete test strategy and
[Security](security.md) for the boundaries these smokes exercise.
