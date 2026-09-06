#requires -Version 7.6

Set-StrictMode -Version Latest

function Test-FlashGatePathWithinRoot {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][string]$Path,
        [Parameter(Mandatory)][string]$Root,
        [switch]$AllowRoot
    )

    $comparison = if ($IsWindows) {
        [System.StringComparison]::OrdinalIgnoreCase
    }
    else {
        [System.StringComparison]::Ordinal
    }
    $resolvedPath = [System.IO.Path]::GetFullPath($Path).TrimEnd('\', '/')
    $resolvedRoot = [System.IO.Path]::GetFullPath($Root).TrimEnd('\', '/')
    if ($AllowRoot -and $resolvedPath.Equals($resolvedRoot, $comparison)) {
        return $true
    }
    $prefix = $resolvedRoot + [System.IO.Path]::DirectorySeparatorChar
    return $resolvedPath.StartsWith($prefix, $comparison)
}

function Assert-FlashGatePathChainSafe {
    [CmdletBinding()]
    param([Parameter(Mandatory)][string]$Path)

    $resolved = [System.IO.Path]::GetFullPath($Path)
    $root = [System.IO.Path]::GetPathRoot($resolved)
    if ([string]::IsNullOrWhiteSpace($root) -or $resolved.TrimEnd('\', '/') -ceq $root.TrimEnd('\', '/')) {
        throw 'A filesystem root is not a permitted FlashGate work root.'
    }
    $relative = $resolved.Substring($root.Length)
    $current = $root
    foreach ($component in @($relative -split '[\\/]' | Where-Object { $_ })) {
        $current = Join-Path $current $component
        if (-not (Test-Path -LiteralPath $current)) {
            throw "FlashGate work-root component does not exist: $current"
        }
        $item = Get-Item -LiteralPath $current -Force
        if (($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0 -or
            -not [string]::IsNullOrWhiteSpace([string]$item.LinkType)) {
            throw "FlashGate work-root path crosses a link or reparse point: $current"
        }
    }
}

function Resolve-FlashGateWorkRoot {
    [CmdletBinding()]
    param([string]$WorkingPath)

    $candidate = $WorkingPath
    if ([string]::IsNullOrWhiteSpace($candidate)) {
        $candidate = [Environment]::GetEnvironmentVariable('FLASHGATE_WORK_ROOT', 'Process')
    }
    if ([string]::IsNullOrWhiteSpace($candidate)) {
        throw 'WorkingPath is required unless FLASHGATE_WORK_ROOT is explicitly bound in the process environment.'
    }
    if (-not [System.IO.Path]::IsPathFullyQualified($candidate)) {
        throw 'WorkingPath must be an absolute path.'
    }

    $resolved = [System.IO.Path]::GetFullPath($candidate)
    if (-not (Test-Path -LiteralPath $resolved -PathType Container)) {
        throw "FlashGate work root must be an existing directory: $resolved"
    }
    Assert-FlashGatePathChainSafe -Path $resolved

    $taskRoot = [Environment]::GetEnvironmentVariable('FLASHGATE_TASK_ROOT', 'Process')
    if (-not [string]::IsNullOrWhiteSpace($taskRoot)) {
        if (-not [System.IO.Path]::IsPathFullyQualified($taskRoot)) {
            throw 'FLASHGATE_TASK_ROOT must be an absolute path.'
        }
        $resolvedTaskRoot = [System.IO.Path]::GetFullPath($taskRoot)
        if (-not (Test-Path -LiteralPath $resolvedTaskRoot -PathType Container)) {
            throw "FLASHGATE_TASK_ROOT must identify an existing directory: $resolvedTaskRoot"
        }
        Assert-FlashGatePathChainSafe -Path $resolvedTaskRoot
        if (-not (Test-FlashGatePathWithinRoot -Path $resolved -Root $resolvedTaskRoot)) {
            throw "WorkingPath must be a child of FLASHGATE_TASK_ROOT: $resolved"
        }
    }

    [Environment]::SetEnvironmentVariable('FLASHGATE_WORK_ROOT', $resolved, 'Process')
    foreach ($name in @('TEMP', 'TMP', 'TMPDIR')) {
        [Environment]::SetEnvironmentVariable($name, $resolved, 'Process')
    }
    [Environment]::SetEnvironmentVariable('POWERSHELL_TELEMETRY_OPTOUT', '1', 'Process')
    return $resolved
}

function New-FlashGateScratchDirectory {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][string]$WorkingPath,
        [Parameter(Mandatory)][ValidatePattern('^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$')][string]$Prefix
    )

    $root = Resolve-FlashGateWorkRoot -WorkingPath $WorkingPath
    $path = Join-Path $root ($Prefix + '-' + [guid]::NewGuid().ToString('N'))
    if (-not (Test-FlashGatePathWithinRoot -Path $path -Root $root)) {
        throw 'Generated scratch path escaped the validated work root.'
    }
    [void][System.IO.Directory]::CreateDirectory($path)
    Assert-FlashGatePathChainSafe -Path $path
    return $path
}

function Remove-FlashGateScratchDirectory {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][string]$Path,
        [Parameter(Mandatory)][string]$WorkingPath,
        [Parameter(Mandatory)][ValidatePattern('^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$')][string]$Prefix
    )

    $root = Resolve-FlashGateWorkRoot -WorkingPath $WorkingPath
    $resolved = [System.IO.Path]::GetFullPath($Path)
    $parent = [System.IO.Path]::GetDirectoryName($resolved)
    $leaf = [System.IO.Path]::GetFileName($resolved)
    $comparison = if ($IsWindows) {
        [System.StringComparison]::OrdinalIgnoreCase
    }
    else {
        [System.StringComparison]::Ordinal
    }
    if (-not $parent.Equals($root.TrimEnd('\', '/'), $comparison) -or
        -not $leaf.StartsWith($Prefix + '-', [System.StringComparison]::Ordinal)) {
        throw "Refusing cleanup outside the validated direct scratch child contract: $resolved"
    }
    if (Test-Path -LiteralPath $resolved) {
        Assert-FlashGatePathChainSafe -Path $resolved
        Remove-Item -LiteralPath $resolved -Recurse -Force
    }
}

Export-ModuleMember -Function @(
    'Test-FlashGatePathWithinRoot',
    'Assert-FlashGatePathChainSafe',
    'Resolve-FlashGateWorkRoot',
    'New-FlashGateScratchDirectory',
    'Remove-FlashGateScratchDirectory'
)
