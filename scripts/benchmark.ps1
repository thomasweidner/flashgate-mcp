[CmdletBinding()]
param(
    [switch]$Quick,
    [switch]$RecordBaseline,
    [string]$OutputPath
)

if ($RecordBaseline) {
    throw 'Authoritative baseline recording is not supported by scripts/benchmark.ps1. Use the documented two-phase prebuilt workflow in an explicitly bound task-local benchmark workspace.'
}

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $true

if (-not ('FlashGate.BenchmarkPathNative' -as [type])) {
    Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
using Microsoft.Win32.SafeHandles;

namespace FlashGate
{
    public static class BenchmarkPathNative
    {
        [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
        public static extern SafeFileHandle CreateFileW(
            string fileName,
            uint desiredAccess,
            uint shareMode,
            IntPtr securityAttributes,
            uint creationDisposition,
            uint flagsAndAttributes,
            IntPtr templateFile);

        [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
        public static extern uint GetFinalPathNameByHandleW(
            SafeFileHandle file,
            System.Text.StringBuilder path,
            uint pathLength,
            uint flags);
    }
}
'@
}

function ConvertFrom-ExtendedWindowsPath {
    [CmdletBinding()]
    param([Parameter(Mandatory)][string]$Path)

    if ($Path.StartsWith('\\?\UNC\', [StringComparison]::OrdinalIgnoreCase)) {
        return '\\' + $Path.Substring(8)
    }
    if ($Path.StartsWith('\\?\', [StringComparison]::OrdinalIgnoreCase)) {
        return $Path.Substring(4)
    }
    return $Path
}

function Get-PhysicalExistingPath {
    [CmdletBinding()]
    param([Parameter(Mandatory)][string]$Path)

    $NormalizedPath = [IO.Path]::GetFullPath($Path)
    $Handle = [FlashGate.BenchmarkPathNative]::CreateFileW(
        $NormalizedPath,
        0,
        7,
        [IntPtr]::Zero,
        3,
        0x02000000,
        [IntPtr]::Zero
    )
    if ($Handle.IsInvalid) {
        $NativeError = [Runtime.InteropServices.Marshal]::GetLastWin32Error()
        $Handle.Dispose()
        throw [ComponentModel.Win32Exception]::new($NativeError, "Cannot resolve physical path: $NormalizedPath")
    }

    try {
        $Capacity = 512
        while ($true) {
            $Buffer = [Text.StringBuilder]::new($Capacity)
            $Length = [FlashGate.BenchmarkPathNative]::GetFinalPathNameByHandleW($Handle, $Buffer, [uint32]$Buffer.Capacity, 0)
            if ($Length -eq 0) {
                $NativeError = [Runtime.InteropServices.Marshal]::GetLastWin32Error()
                throw [ComponentModel.Win32Exception]::new($NativeError, "Cannot resolve final physical path: $NormalizedPath")
            }
            if ($Length -lt $Buffer.Capacity) {
                return [IO.Path]::TrimEndingDirectorySeparator((ConvertFrom-ExtendedWindowsPath -Path $Buffer.ToString()))
            }
            $Capacity = [int]$Length + 1
        }
    }
    finally {
        $Handle.Dispose()
    }
}

function Assert-NonAuthoritativeOutputPath {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][string]$CandidatePath,
        [Parameter(Mandatory)][string]$RepositoryRoot
    )

    $OutputFullPath = [IO.Path]::GetFullPath($CandidatePath)
    $FileName = [IO.Path]::GetFileName($OutputFullPath)
    $IsCanonicalName = $FileName -like 'baseline.*-*.json'
    $ParentPath = [IO.Path]::GetDirectoryName($OutputFullPath)
    $PhysicalBenchmarkDirectory = Get-PhysicalExistingPath -Path (Join-Path $RepositoryRoot 'benchmarks')
    $PhysicalParent = $null
    try {
        $PhysicalParent = Get-PhysicalExistingPath -Path $ParentPath
    }
    catch {
        if ($IsCanonicalName) {
            throw 'Non-authoritative benchmark runs must not use a canonical baseline name below an unresolved physical parent path.'
        }
    }

    if ($null -ne $PhysicalParent -and $PhysicalParent.Equals($PhysicalBenchmarkDirectory, [StringComparison]::OrdinalIgnoreCase)) {
        throw 'Non-authoritative benchmark runs must not write a canonical versioned baseline path.'
    }

    $PhysicalTarget = $null
    $ExistingTarget = Get-Item -LiteralPath $OutputFullPath -Force -ErrorAction SilentlyContinue
    if ($null -ne $ExistingTarget) {
        if (($ExistingTarget.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
            throw 'Non-authoritative benchmark runs must not use a final symbolic link or reparse-point output target.'
        }
        if ($ExistingTarget.PSIsContainer) {
            throw 'Non-authoritative benchmark output targets must be regular files.'
        }
        try {
            $PhysicalTarget = Get-PhysicalExistingPath -Path $OutputFullPath
        }
        catch {
            throw 'Non-authoritative benchmark runs must not use an unresolved reparse target with a canonical baseline name.'
        }
        $PhysicalTargetParent = [IO.Path]::GetDirectoryName($PhysicalTarget)
        if ($PhysicalTargetParent.Equals($PhysicalBenchmarkDirectory, [StringComparison]::OrdinalIgnoreCase)) {
            throw 'Non-authoritative benchmark runs must not write a canonical versioned baseline path.'
        }
    }

    return [pscustomobject]@{
        OutputFullPath = $OutputFullPath
        IsCanonicalName = $true
        PhysicalParent = $PhysicalParent
        PhysicalTarget = $PhysicalTarget
    }
}

function Assert-OutputPathUnchanged {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]$InitialState,
        [Parameter(Mandatory)]$CurrentState
    )

    $Comparer = [StringComparer]::OrdinalIgnoreCase
    $ParentChanged = $null -ne $InitialState.PhysicalParent -and
        -not $Comparer.Equals([string]$InitialState.PhysicalParent, [string]$CurrentState.PhysicalParent)
    $TargetChanged = -not $Comparer.Equals([string]$InitialState.PhysicalTarget, [string]$CurrentState.PhysicalTarget)
    if ($ParentChanged -or $TargetChanged) {
        throw 'Non-authoritative benchmark output path changed after physical validation; refusing to start the benchmark.'
    }
}

$RepoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$BuildDirectory = Join-Path $RepoRoot 'build'
$ServerBinary = Join-Path $BuildDirectory 'flashgate-mcp.exe'
$BenchmarkBinary = Join-Path $BuildDirectory 'flashgate-benchmark.exe'
$ProtectedBaselineDirectory = Join-Path $RepoRoot 'benchmarks'
$BudgetPath = Join-Path $RepoRoot 'benchmarks\budgets.json'

if ([string]::IsNullOrWhiteSpace($OutputPath)) {
    $OutputPath = Join-Path $BuildDirectory 'benchmark-current.windows-amd64.json'
}
elseif (-not [IO.Path]::IsPathRooted($OutputPath)) {
    $OutputPath = Join-Path $RepoRoot $OutputPath
}
$InitialOutputPolicyState = Assert-NonAuthoritativeOutputPath -CandidatePath $OutputPath -RepositoryRoot $RepoRoot
$OutputPath = $InitialOutputPolicyState.OutputFullPath

. (Join-Path $PSScriptRoot 'benchmark-window.ps1')

$Status = 'FAIL'
$WarningCount = 0
$FailureCount = 0
$NextAction = 'Inspect the reported failure.'
$ReportPath = $null
$FailureMessage = $null
$RunOutputPath = $null
$PerformanceContaminated = $false
$MeasurementWarning = $null

try {
    $PerformanceContaminated = (Get-EuropeViennaMeasurementWindowStatus).IsBlocked

    $InitialStatus = @(& git status --porcelain --untracked-files=all)
    $WorkingTreeDirty = $InitialStatus.Count -gt 0

    New-Item -ItemType Directory -Path $BuildDirectory -Force | Out-Null
    & go build -o $ServerBinary ./cmd/server 2>&1 | Out-Null
    & go build -o $BenchmarkBinary ./cmd/benchmark 2>&1 | Out-Null
    $Commit = (& git rev-parse HEAD).Trim()
    $RunOutputPath = $OutputPath

    $Arguments = @(
        '-binary', $ServerBinary,
        '-output', $RunOutputPath,
        '-commit', $Commit,
        '-budgets', $BudgetPath,
        '-protected-baseline-dir', $ProtectedBaselineDirectory
    )
    if ($WorkingTreeDirty) {
        $Arguments += '-working-tree-dirty'
    }
    if ($Quick) {
        $Arguments += '-quick'
    }

    $CurrentOutputPolicyState = Assert-NonAuthoritativeOutputPath -CandidatePath $RunOutputPath -RepositoryRoot $RepoRoot
    Assert-OutputPathUnchanged -InitialState $InitialOutputPolicyState -CurrentState $CurrentOutputPolicyState
    $BenchmarkOutput = @(& $BenchmarkBinary @Arguments 2>&1)
    $Result = Get-Content -LiteralPath $RunOutputPath -Raw | ConvertFrom-Json
    $WarningCount = @($Result.warnings).Count
    $FailureCount = [int]$Result.budget_evaluation.hard_failures
    if ($FailureCount -gt 0) {
        throw "Benchmark reported $FailureCount hard budget failure(s)."
    }

    if ((Get-EuropeViennaMeasurementWindowStatus).IsBlocked) {
        $PerformanceContaminated = $true
    }

    if ($PerformanceContaminated) {
        $MeasurementWarning = Get-ContaminatedPerformanceWarning
        $WarningCount++
    }

    $Status = 'PASS'
    $NextAction = 'Compare the current result with the versioned baseline.'
}
catch {
    $FailureCount = [Math]::Max(1, $FailureCount)
    $FailureMessage = $_.Exception.Message
}
finally {
    $ReportPath = if ([string]::IsNullOrWhiteSpace($OutputPath)) { '[not-created]' } else { $OutputPath }
    [pscustomobject]@{
        Status         = $Status
        ReportPath     = $ReportPath
        WarningCount   = $WarningCount
        FailureCount   = $FailureCount
        NextAction     = $NextAction
        MeasurementWarning = $MeasurementWarning
        FailureMessage = $FailureMessage
    } | Format-List
}

if ($FailureCount -gt 0) {
    exit 1
}
