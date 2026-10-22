#requires -Version 7.6

Set-StrictMode -Version Latest

function Assert-AuthoritativeBenchmarkCorpusParent {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][string]$CorpusParent,
        [Parameter(Mandatory)][string]$WorkspaceRoot,
        [string[]]$SynchronizedRoots = @()
    )

    if (-not $IsWindows) { throw 'The Windows corpus-parent gate requires Windows.' }
    if (-not [System.IO.Path]::IsPathFullyQualified($CorpusParent)) {
        throw 'Benchmark corpus parent must be an absolute path.'
    }
    Import-Module (Join-Path $PSScriptRoot 'TaskBoundWorkRoot.psm1') -Force
    $workspace = Resolve-FlashGateWorkRoot -WorkingPath $WorkspaceRoot
    $parent = [System.IO.Path]::GetFullPath($CorpusParent)
    if (-not (Test-Path -LiteralPath $parent -PathType Container)) {
        throw 'Benchmark corpus parent must be an existing directory.'
    }
    Assert-FlashGatePathChainSafe -Path $parent
    if (-not (Test-FlashGatePathWithinRoot -Path $parent -Root $workspace)) {
        throw 'Benchmark corpus parent must be strictly below the task-bound workspace.'
    }
    if ($parent.StartsWith('\\')) { throw 'Network benchmark corpus parents are prohibited.' }
    $drive = [System.IO.DriveInfo]::new([System.IO.Path]::GetPathRoot($parent))
    if ($drive.DriveType -ne [System.IO.DriveType]::Fixed -or $drive.DriveFormat -cne 'NTFS') {
        throw 'Benchmark corpus parent must be on a fixed local NTFS volume.'
    }
    $knownSynchronizedRoots = @(
        $SynchronizedRoots
        [Environment]::GetEnvironmentVariable('OneDrive', 'Process')
        [Environment]::GetEnvironmentVariable('OneDriveCommercial', 'Process')
        [Environment]::GetEnvironmentVariable('OneDriveConsumer', 'Process')
        [Environment]::GetEnvironmentVariable('Dropbox', 'Process')
    ) | Where-Object { -not [string]::IsNullOrWhiteSpace($_) }
    foreach ($root in $knownSynchronizedRoots) {
        $resolvedRoot = [System.IO.Path]::GetFullPath($root)
        if ((Test-FlashGatePathWithinRoot -Path $parent -Root $resolvedRoot -AllowRoot)) {
            throw 'Benchmark corpus parent must not be below synchronized storage.'
        }
    }
    return $parent
}

Export-ModuleMember -Function 'Assert-AuthoritativeBenchmarkCorpusParent'
