#requires -Version 7.6
<#
.SYNOPSIS
Runs one validation process using FlashGate's existing bounded process owner.
.DESCRIPTION
Internal JSON bridge. Request and result files belong to the caller's work root.
The process receives the bridge's already-bound environment.
.PARAMETER RequestPath
Absolute path to a JSON process request.
.PARAMETER ResultPath
New absolute result path.
#>
[CmdletBinding(PositionalBinding = $false)]
param(
    [Parameter(Mandatory)][string]$RequestPath,
    [Parameter(Mandatory)][string]$ResultPath
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$exitCode = 1
try {
    if ([IO.File]::Exists($ResultPath)) { throw 'ResultAlreadyExists' }
    $request = Get-Content -LiteralPath $RequestPath -Raw -Encoding utf8 | ConvertFrom-Json
    if ($request.PSObject.Properties.Name -contains 'operation') {
        if ($request.operation -cne 'snapshot') { throw 'UnknownBridgeOperation' }
        . (Join-Path $PSScriptRoot 'New-GitSnapshot.ps1')
        New-FlashGateGitSnapshot -RootPath $request.directory -SnapshotPath $request.archive -ManifestPath $request.manifest | Out-Null
        [IO.File]::WriteAllText($ResultPath, '{"status":"PASS"}' + "`n", [Text.UTF8Encoding]::new($false))
        exit 0
    }
    . (Join-Path $PSScriptRoot 'VerifierProcess.ps1')
    # The existing process owner is reused with a suite wall-clock budget.
    # This does not alter Go's per-package test timeout.
    $clock = [Diagnostics.Stopwatch]::StartNew()
    $result = [FlashGate.Validation.BoundedProcessRunner]::Run(
        [string]$request.executable,
        [string[]]$request.arguments,
        [string]$request.directory,
        [int]$request.timeout_ms,
        33554432,
        5000
    )
    $clock.Stop()
    [IO.File]::WriteAllText([string]$request.stdout, $result.Stdout, [Text.UTF8Encoding]::new($false))
    [IO.File]::WriteAllText([string]$request.stderr, $result.Stderr, [Text.UTF8Encoding]::new($false))
    $record = [ordered]@{
        status = $result.Status
        exit_code = $result.ExitCode
        timed_out = $result.TimedOut
        failure_reason = $result.FailureReason
        seconds = $clock.Elapsed.TotalSeconds
        output_limit = $result.OutputLimitExceeded
    }
    [IO.File]::WriteAllText($ResultPath, ($record | ConvertTo-Json -Compress) + "`n", [Text.UTF8Encoding]::new($false))
    $exitCode = 0
}
catch {
    Write-Error -ErrorRecord $_ -ErrorAction Continue
}
finally {
    $request = $null
}
exit $exitCode
