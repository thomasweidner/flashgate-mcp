#requires -Version 7.6
[CmdletBinding()]
param(
    [Parameter(Mandatory)][string]$TaskRoot,
    [Parameter(Mandatory)][string]$WorkingPath
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

Import-Module (Join-Path $PSScriptRoot 'TaskBoundWorkRoot.psm1') -Force

$resolvedTaskRoot = [System.IO.Path]::GetFullPath($TaskRoot)
$resolvedWorkingPath = [System.IO.Path]::GetFullPath($WorkingPath)
$original = @{}
foreach ($name in @('FLASHGATE_TASK_ROOT', 'FLASHGATE_WORK_ROOT', 'TEMP', 'TMP', 'TMPDIR')) {
    $original[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
}

$checkCount = 0
$failures = [System.Collections.Generic.List[string]]::new()
function Test-Condition {
    param([Parameter(Mandatory)][bool]$Condition, [Parameter(Mandatory)][string]$Name)
    $script:checkCount++
    if (-not $Condition) { [void]$script:failures.Add($Name) }
}

$scratch = $null
$reparseScratch = $null
try {
    [Environment]::SetEnvironmentVariable('FLASHGATE_TASK_ROOT', $null, 'Process')
    [Environment]::SetEnvironmentVariable('FLASHGATE_WORK_ROOT', $null, 'Process')
    $missingRejected = $false
    try { $null = Resolve-FlashGateWorkRoot } catch { $missingRejected = $true }
    Test-Condition $missingRejected 'missing root fails closed'

    $relativeRejected = $false
    try { $null = Resolve-FlashGateWorkRoot -WorkingPath 'relative-work-root' } catch { $relativeRejected = $true }
    Test-Condition $relativeRejected 'relative root fails closed'

    [Environment]::SetEnvironmentVariable('FLASHGATE_TASK_ROOT', $resolvedTaskRoot, 'Process')
    $foreignRejected = $false
    try { $null = Resolve-FlashGateWorkRoot -WorkingPath $PSHOME } catch { $foreignRejected = $true }
    Test-Condition $foreignRejected 'foreign root fails closed'

    $resolved = Resolve-FlashGateWorkRoot -WorkingPath $resolvedWorkingPath
    Test-Condition ($resolved -ceq $resolvedWorkingPath) 'explicit work root resolves exactly'
    foreach ($name in @('TEMP', 'TMP', 'TMPDIR')) {
        Test-Condition (
            [Environment]::GetEnvironmentVariable($name, 'Process') -ceq $resolvedWorkingPath
        ) "$name is process-bound to work root"
    }

    $scratch = New-FlashGateScratchDirectory -WorkingPath $resolvedWorkingPath -Prefix 'contract-test'
    Test-Condition (Test-FlashGatePathWithinRoot -Path $scratch -Root $resolvedWorkingPath) 'scratch stays inside work root'
    Test-Condition (Test-Path -LiteralPath $scratch -PathType Container) 'scratch directory exists'

    $reparseScratch = New-FlashGateScratchDirectory -WorkingPath $resolvedWorkingPath -Prefix 'contract-reparse'
    $target = Join-Path $reparseScratch 'target'
    $link = Join-Path $reparseScratch 'link'
    [void][System.IO.Directory]::CreateDirectory($target)
    $linkType = if ($IsWindows) { 'Junction' } else { 'SymbolicLink' }
    $null = New-Item -ItemType $linkType -Path $link -Target $target
    $reparseRejected = $false
    try { Assert-FlashGatePathChainSafe -Path $link } catch { $reparseRejected = $true }
    Test-Condition $reparseRejected 'linked ancestor fails closed'
}
finally {
    if ($null -ne $reparseScratch -and (Test-Path -LiteralPath $reparseScratch)) {
        $link = Join-Path $reparseScratch 'link'
        if (Test-Path -LiteralPath $link) { Remove-Item -LiteralPath $link -Force }
        Remove-FlashGateScratchDirectory -Path $reparseScratch -WorkingPath $resolvedWorkingPath -Prefix 'contract-reparse'
    }
    if ($null -ne $scratch -and (Test-Path -LiteralPath $scratch)) {
        Remove-FlashGateScratchDirectory -Path $scratch -WorkingPath $resolvedWorkingPath -Prefix 'contract-test'
    }
    foreach ($name in $original.Keys) {
        [Environment]::SetEnvironmentVariable($name, $original[$name], 'Process')
    }
}

if ($failures.Count -gt 0) {
    throw ($failures -join [Environment]::NewLine)
}
[pscustomobject]@{ Status = 'PASS'; CheckCount = $checkCount; FailureCount = 0 }
