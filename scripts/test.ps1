#requires -Version 7.6
<#
.SYNOPSIS
Runs FlashGate development, candidate or post-review validation.
.DESCRIPTION
Composes existing project gates. See docs/testing.md for evidence and platform
requirements. Temporary output requires an existing external work root.
.PARAMETER Mode
FAST selects affected development tests. CANDIDATE runs platform gates.
PUBLISH verifies unchanged candidate and independent review evidence.
.PARAMETER WorkingPath
Existing task-owned output and cache root outside the checkout.
#>
[CmdletBinding(PositionalBinding = $false)]
param(
    [ValidateSet('FAST', 'CANDIDATE', 'PUBLISH')][string]$Mode = 'FAST',
    [Parameter(Mandatory)][string]$WorkingPath,
    [string]$ResourceRoot,
    [string[]]$ChangedPath,
    [string]$EvidencePath,
    [string]$EvidenceSHA256,
    [string]$ReviewPath,
    [string]$ReviewSHA256,
    [string]$ReviewManifestPath,
    [string]$NativeConfig,
    [switch]$PlanOnly
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$exitCode = 2
$process = $null
try {
    $python = @(Get-Command python -CommandType Application -ErrorAction Stop)[0].Source
    $start = [Diagnostics.ProcessStartInfo]::new()
    $start.FileName = $python
    $start.UseShellExecute = $false
    $start.CreateNoWindow = $true
    $start.RedirectStandardOutput = $true
    $start.RedirectStandardError = $true
    $start.WorkingDirectory = Split-Path -Parent $PSScriptRoot
    foreach ($value in @('-B', (Join-Path $PSScriptRoot 'validation.py'), '--mode', $Mode, '--work-root', $WorkingPath)) {
        $start.ArgumentList.Add($value)
    }
    foreach ($pair in @(
        @('--resource-root', $ResourceRoot), @('--evidence', $EvidencePath),
        @('--evidence-sha256', $EvidenceSHA256), @('--review', $ReviewPath),
        @('--review-sha256', $ReviewSHA256), @('--native-config', $NativeConfig),
        @('--review-manifest', $ReviewManifestPath)
    )) {
        if (-not [string]::IsNullOrWhiteSpace($pair[1])) {
            $start.ArgumentList.Add($pair[0])
            $start.ArgumentList.Add($pair[1])
        }
    }
    foreach ($path in @($ChangedPath)) {
        if (-not [string]::IsNullOrWhiteSpace($path)) {
            $start.ArgumentList.Add('--changed')
            $start.ArgumentList.Add($path)
        }
    }
    if ($PlanOnly) { $start.ArgumentList.Add('--plan-only') }
    $process = [Diagnostics.Process]::Start($start)
    $stderrTask = $process.StandardError.ReadToEndAsync()
    while ($null -ne ($line = $process.StandardOutput.ReadLine())) {
        Write-Output $line
    }
    $process.WaitForExit()
    $stderr = $stderrTask.GetAwaiter().GetResult()
    if (-not [string]::IsNullOrEmpty($stderr)) { [Console]::Error.Write($stderr) }
    $exitCode = $process.ExitCode
}
catch {
    Write-Error -ErrorRecord $_ -ErrorAction Continue
}
finally {
    if ($null -ne $process) { $process.Dispose() }
}
exit $exitCode
