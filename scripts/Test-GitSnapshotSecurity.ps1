[CmdletBinding()]
param([string]$WorkingPath)

$ErrorActionPreference = 'Stop'
Import-Module (Join-Path $PSScriptRoot 'TaskBoundWorkRoot.psm1') -Force
$WorkRoot = Resolve-FlashGateWorkRoot -WorkingPath $WorkingPath
$SnapshotScript = Join-Path $PSScriptRoot 'New-GitSnapshot.ps1'
$TestRoot = New-FlashGateScratchDirectory -WorkingPath $WorkRoot -Prefix 'git-snapshot-test'
$Repository = Join-Path $TestRoot 'repository'
$Output = Join-Path $TestRoot 'output'
$Errors = [Collections.Generic.List[string]]::new()
$ExitCode = 1

function Invoke-GitRequired {
    param([Parameter(Mandatory)] [string[]] $Arguments)

    $Result = @(& git.exe -C $Repository @Arguments 2>&1)
    if ($LASTEXITCODE -ne 0) {
        throw ('git {0} failed: {1}' -f ($Arguments -join ' '), ($Result -join ' '))
    }
}

function Invoke-Snapshot {
    param([Parameter(Mandatory)] [string] $Name)

    New-FlashGateGitSnapshot `
        -RootPath $Repository `
        -SnapshotPath (Join-Path $Output "$Name.tar") `
        -ManifestPath (Join-Path $Output "$Name.json")
}

try {
    . $SnapshotScript
    $null = New-Item -ItemType Directory -Path $Repository, $Output
    Invoke-GitRequired -Arguments @('init', '--quiet')

    [IO.File]::WriteAllText(
        (Join-Path $Repository '.gitignore'),
        "ignored.bin`n.env`n",
        [Text.UTF8Encoding]::new($false)
    )
    [IO.File]::WriteAllText(
        (Join-Path $Repository 'tracked.txt'),
        'tracked',
        [Text.UTF8Encoding]::new($false)
    )
    [IO.File]::WriteAllText(
        (Join-Path $Repository 'untracked.txt'),
        'untracked',
        [Text.UTF8Encoding]::new($false)
    )
    [IO.File]::WriteAllText(
        (Join-Path $Repository 'ignored.bin'),
        '[REDACTED]',
        [Text.UTF8Encoding]::new($false)
    )
    Invoke-GitRequired -Arguments @('add', '.gitignore', 'tracked.txt')

    $null = Invoke-Snapshot -Name 'valid'
    $Manifest = Get-Content -LiteralPath (Join-Path $Output 'valid.json') -Raw |
        ConvertFrom-Json -Depth 5
    $ManifestPaths = @($Manifest.files.Path)
    foreach ($Expected in @('.gitignore', 'tracked.txt', 'untracked.txt')) {
        if ($Expected -notin $ManifestPaths) {
            $Errors.Add("Expected snapshot file missing: $Expected")
        }
    }
    foreach ($Forbidden in @('ignored.bin')) {
        if ($Forbidden -in $ManifestPaths) {
            $Errors.Add("Ignored file entered the snapshot: $Forbidden")
        }
    }

    [IO.File]::WriteAllText(
        (Join-Path $Repository '.env'),
        '[REDACTED]',
        [Text.UTF8Encoding]::new($false)
    )
    try {
        $null = Invoke-Snapshot -Name 'ignored-sensitive'
        $Errors.Add('Ignored sensitive filename unexpectedly passed.')
    }
    catch {
    }
    Remove-Item -LiteralPath (Join-Path $Repository '.env')

    [IO.File]::WriteAllText(
        (Join-Path $Repository 'credentials-local.txt'),
        '[REDACTED]',
        [Text.UTF8Encoding]::new($false)
    )
    try {
        $null = Invoke-Snapshot -Name 'sensitive'
        $Errors.Add('Sensitive filename unexpectedly entered a snapshot.')
    }
    catch {
    }
    Remove-Item -LiteralPath (Join-Path $Repository 'credentials-local.txt')

    $HardLink = Join-Path $Repository 'hardlink.txt'
    $null = New-Item `
        -ItemType HardLink `
        -Path $HardLink `
        -Target (Join-Path $Repository 'tracked.txt')
    try {
        $null = Invoke-Snapshot -Name 'hardlink'
        $Errors.Add('Hard-linked file unexpectedly entered a snapshot.')
    }
    catch {
    }
    Remove-Item -LiteralPath $HardLink

    $OutsideFile = Join-Path $TestRoot 'outside.txt'
    [IO.File]::WriteAllText(
        $OutsideFile,
        'outside',
        [Text.UTF8Encoding]::new($false)
    )
    $LinkPath = Join-Path $Repository 'linked.txt'
    $null = New-Item -ItemType SymbolicLink -Path $LinkPath -Target $OutsideFile
    try {
        $null = Invoke-Snapshot -Name 'reparse'
        $Errors.Add('Reparse-point file unexpectedly entered a snapshot.')
    }
    catch {
    }

    if (
        (Get-Content -LiteralPath $OutsideFile -Raw) -cne 'outside'
    ) {
        $Errors.Add('File outside the snapshot repository was modified.')
    }

    if ($Errors.Count -eq 0) {
        $ExitCode = 0
    }
}
catch {
    $Errors.Add($_.Exception.Message)
}
finally {
    if (Test-Path -LiteralPath $TestRoot -PathType Container) {
        Remove-FlashGateScratchDirectory -Path $TestRoot -WorkingPath $WorkRoot -Prefix 'git-snapshot-test'
    }

    [pscustomobject]@{
        Status       = if ($Errors.Count -eq 0) { 'PASS' } else { 'FAIL' }
        TestRoot     = $TestRoot
        WarningCount = 0
        ErrorCount   = $Errors.Count
        Warnings     = $null
        Errors       = if ($Errors.Count -gt 0) {
            $Errors -join [Environment]::NewLine
        } else {
            $null
        }
    } | Format-List
}

exit $ExitCode
