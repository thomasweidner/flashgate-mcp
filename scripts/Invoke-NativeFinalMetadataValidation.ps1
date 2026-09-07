[CmdletBinding()]
param(
    [string] $RootPath = (Split-Path -Parent $PSScriptRoot),
    [string] $DistroName = 'Ubuntu-24.04',
    [Parameter(Mandatory)]
    [string] $OutputRoot,
    [string] $WorkingPath
)

$ErrorActionPreference = 'Stop'
Import-Module (Join-Path $PSScriptRoot 'TaskBoundWorkRoot.psm1') -Force
$WorkRoot = Resolve-FlashGateWorkRoot -WorkingPath $WorkingPath

$Timestamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$WorkingDirectory = New-FlashGateScratchDirectory -WorkingPath $WorkRoot -Prefix 'native-final'
$SnapshotPath = Join-Path $WorkingDirectory 'working-tree.tar'
$SnapshotManifestPath = Join-Path $WorkingDirectory 'working-tree.manifest.json'
$DriverPath = Join-Path $WorkingDirectory 'native-final-driver.sh'
$DriverTemplatePath = Join-Path $PSScriptRoot 'final-metadata-native-driver.sh'
$SnapshotScriptPath = Join-Path $PSScriptRoot 'New-GitSnapshot.ps1'
$SnapshotValidatorTemplatePath = Join-Path $PSScriptRoot 'validate-snapshot.py'
$SnapshotValidatorPath = Join-Path $WorkingDirectory 'validate-snapshot.py'
$SafetyLibraryTemplatePath = Join-Path $PSScriptRoot 'native-validation-safety.sh'
$SafetyLibraryPath = Join-Path $WorkingDirectory 'native-validation-safety.sh'
$SafetyHelperTemplatePath = Join-Path $PSScriptRoot 'safe-work-root.py'
$SafetyHelperPath = Join-Path $WorkingDirectory 'safe-work-root.py'
$SummaryPath = Join-Path $OutputRoot 'native-final-summary.env'

$Warnings = [System.Collections.Generic.List[string]]::new()
$Errors = [System.Collections.Generic.List[string]]::new()

$Result = [ordered]@{
    Status       = 'FAIL'
    RootPath     = $RootPath
    DistroName   = $DistroName
    OutputRoot   = $OutputRoot
    SummaryPath  = $SummaryPath
    NativeRoot   = $null
    GoVersion    = $null
    FixtureCount = $null
    WarningCount = 0
    ErrorCount   = 0
    Warnings     = $null
    Errors       = $null
}

function Invoke-ExternalRequired {
    param(
        [Parameter(Mandatory)]
        [string] $FilePath,
        [Parameter(Mandatory)]
        [string[]] $Arguments,
        [string] $WorkingDirectory = $RootPath
    )

    $Output = @(& $FilePath @Arguments 2>&1)
    $ExitCode = $LASTEXITCODE
    if ($ExitCode -ne 0) {
        throw ('{0} {1} failed with exit code {2}: {3}' -f
            $FilePath, ($Arguments -join ' '), $ExitCode, ($Output -join ' '))
    }
    return $Output
}

function ConvertTo-WslPath {
    param([Parameter(Mandatory)] [string] $WindowsPath)

    $FullWindowsPath = [IO.Path]::GetFullPath($WindowsPath)
    $PathRoot = [IO.Path]::GetPathRoot($FullWindowsPath)
    $Separator = [IO.Path]::DirectorySeparatorChar
    $Valid = (
        $PathRoot.Length -eq 3 -and
        [char]::IsLetter($PathRoot[0]) -and
        $PathRoot[1] -eq ':' -and
        $PathRoot[2] -eq $Separator
    )
    if (-not $Valid) {
        throw "Only local drive-letter paths can be mapped to WSL: $FullWindowsPath"
    }

    $Drive = [char]::ToLowerInvariant($PathRoot[0])
    $Relative = $FullWindowsPath.Substring($PathRoot.Length).Replace(
        $Separator,
        [char]'/'
    )
    if ([string]::IsNullOrEmpty($Relative)) {
        return "/mnt/$Drive"
    }
    return "/mnt/$Drive/$Relative"
}

try {
    foreach ($RequiredPath in @(
        $RootPath
        (Join-Path $RootPath '.git')
        $DriverTemplatePath
        $SnapshotScriptPath
        $SnapshotValidatorTemplatePath
        $SafetyLibraryTemplatePath
        $SafetyHelperTemplatePath
    )) {
        if (-not (Test-Path -LiteralPath $RequiredPath)) {
            throw "Required path not found: $RequiredPath"
        }
    }

    foreach ($Command in @('wsl.exe', 'git.exe')) {
        if (-not (Get-Command $Command -ErrorAction SilentlyContinue)) {
            throw "Required command not found: $Command"
        }
    }

    $GitCommonDirectory = (
        Invoke-ExternalRequired `
            -FilePath 'git.exe' `
            -Arguments @(
                '-C'
                $RootPath
                'rev-parse'
                '--path-format=absolute'
                '--git-common-dir'
            ) |
            Out-String
    ).Trim()
    $HeadCommit = (
        Invoke-ExternalRequired `
            -FilePath 'git.exe' `
            -Arguments @('-C', $RootPath, 'rev-parse', 'HEAD') |
            Out-String
    ).Trim()
    if (-not (Test-Path -LiteralPath $GitCommonDirectory -PathType Container)) {
        throw ('Git common directory not found: {0}' -f $GitCommonDirectory)
    }
    if ($HeadCommit -notmatch '^[0-9a-f]{40}$') {
        throw ('Unexpected Git HEAD format: {0}' -f $HeadCommit)
    }

    New-Item -ItemType Directory -Path $OutputRoot -Force | Out-Null
    New-Item -ItemType Directory -Path $WorkingDirectory -Force | Out-Null
    Copy-Item -LiteralPath $DriverTemplatePath -Destination $DriverPath -Force
    Copy-Item `
        -LiteralPath $SnapshotValidatorTemplatePath `
        -Destination $SnapshotValidatorPath `
        -Force
    Copy-Item `
        -LiteralPath $SafetyLibraryTemplatePath `
        -Destination $SafetyLibraryPath `
        -Force
    Copy-Item `
        -LiteralPath $SafetyHelperTemplatePath `
        -Destination $SafetyHelperPath `
        -Force

    $null = Invoke-ExternalRequired `
        -FilePath 'wsl.exe' `
        -Arguments @('-d', $DistroName, '--', 'true')

    . $SnapshotScriptPath
    $null = New-FlashGateGitSnapshot `
        -RootPath $RootPath `
        -SnapshotPath $SnapshotPath `
        -ManifestPath $SnapshotManifestPath

    foreach ($SnapshotOutput in @($SnapshotPath, $SnapshotManifestPath)) {
        if (-not (Test-Path -LiteralPath $SnapshotOutput -PathType Leaf)) {
            throw "Working-tree snapshot output was not created: $SnapshotOutput"
        }
    }

    $RootWslPath = ConvertTo-WslPath -WindowsPath $RootPath
    $GitCommonDirectoryWslPath = ConvertTo-WslPath `
        -WindowsPath $GitCommonDirectory
    $SnapshotWslPath = ConvertTo-WslPath -WindowsPath $SnapshotPath
    $SnapshotManifestWslPath = ConvertTo-WslPath `
        -WindowsPath $SnapshotManifestPath
    $OutputWslPath = ConvertTo-WslPath -WindowsPath $OutputRoot
    $DriverWslPath = ConvertTo-WslPath -WindowsPath $DriverPath
    $SnapshotValidatorWslPath = ConvertTo-WslPath `
        -WindowsPath $SnapshotValidatorPath
    $SafetyLibraryWslPath = ConvertTo-WslPath `
        -WindowsPath $SafetyLibraryPath
    $SafetyHelperWslPath = ConvertTo-WslPath -WindowsPath $SafetyHelperPath

    foreach ($PathCheck in @(
        @{ Type = '-d'; Path = $RootWslPath; Name = 'repository root' }
        @{ Type = '-f'; Path = $SnapshotWslPath; Name = 'working-tree snapshot' }
        @{ Type = '-f'; Path = $SnapshotManifestWslPath; Name = 'snapshot manifest' }
        @{ Type = '-d'; Path = $OutputWslPath; Name = 'native output directory' }
        @{ Type = '-f'; Path = $DriverWslPath; Name = 'native driver' }
        @{ Type = '-f'; Path = $SnapshotValidatorWslPath; Name = 'snapshot validator' }
        @{ Type = '-f'; Path = $SafetyLibraryWslPath; Name = 'safety library' }
        @{ Type = '-f'; Path = $SafetyHelperWslPath; Name = 'work-root helper' }
        @{ Type = '-d'; Path = $GitCommonDirectoryWslPath; Name = 'Git common directory' }
    )) {
        try {
            $null = Invoke-ExternalRequired `
                -FilePath 'wsl.exe' `
                -Arguments @(
                    '-d'
                    $DistroName
                    '--'
                    'test'
                    $PathCheck.Type
                    $PathCheck.Path
                )
        }
        catch {
            throw "Converted WSL path is invalid for $($PathCheck.Name): $($PathCheck.Path)"
        }
    }

    $RunId = "flashgate-file-properties-final-$Timestamp"
    $null = Invoke-ExternalRequired `
        -FilePath 'wsl.exe' `
        -Arguments @(
            '-d'
            $DistroName
            '--'
            'env'
            "FG_WINDOWS_REPO=$RootWslPath"
            ('FG_GIT_COMMON_DIR={0}' -f $GitCommonDirectoryWslPath)
            ('FG_HEAD_COMMIT={0}' -f $HeadCommit)
            "FG_SNAPSHOT_TAR=$SnapshotWslPath"
            "FG_SNAPSHOT_MANIFEST=$SnapshotManifestWslPath"
            "FG_SNAPSHOT_VALIDATOR=$SnapshotValidatorWslPath"
            "FG_NATIVE_SAFETY=$SafetyLibraryWslPath"
            "FG_NATIVE_SAFETY_HELPER=$SafetyHelperWslPath"
            ('FG_OUTPUT_DIR={0}' -f $OutputWslPath)
            "FG_RUN_ID=$RunId"
            "FG_DISTRO_NAME=$DistroName"
            'bash'
            $DriverWslPath
        )

    if (-not (Test-Path -LiteralPath $SummaryPath -PathType Leaf)) {
        throw "Native final summary was not created: $SummaryPath"
    }

    $Summary = [ordered]@{}
    foreach ($Line in Get-Content -LiteralPath $SummaryPath) {
        if ([string]::IsNullOrWhiteSpace($Line) -or $Line -notmatch '=') {
            continue
        }
        $Parts = $Line.Split('=', 2)
        $Summary[$Parts[0]] = $Parts[1]
    }

    if ($Summary['STATUS'] -ne 'PASS') {
        throw "Native final validation did not report PASS: $($Summary['ERROR'])"
    }

    foreach ($RequiredKey in @(
        'NATIVE_ROOT'
        'GO_VERSION'
        'FIXTURE_COUNT'
        'SMOKE_DEFAULT'
        'SMOKE_READONLY'
        'SMOKE_NEGATIVE'
        'SMOKE_STARTUP_NEGATIVE'
    )) {
        if ([string]::IsNullOrWhiteSpace($Summary[$RequiredKey])) {
            throw "Native final summary value is missing: $RequiredKey"
        }
    }

    $Result.Status = 'PASS'
    $Result.NativeRoot = $Summary['NATIVE_ROOT']
    $Result.GoVersion = $Summary['GO_VERSION']
    $Result.FixtureCount = [int]$Summary['FIXTURE_COUNT']
}
catch {
    $Errors.Add($_.Exception.Message)
}
finally {
    if (Test-Path -LiteralPath $WorkingDirectory -PathType Container) {
        try {
            Remove-FlashGateScratchDirectory -Path $WorkingDirectory -WorkingPath $WorkRoot -Prefix 'native-final'
        }
        catch {
            $Errors.Add(
                "Failed to remove orchestration directory: $($_.Exception.Message)"
            )
        }
    }
    $Result.WarningCount = $Warnings.Count
    $Result.ErrorCount = $Errors.Count
    $Result.Warnings = if ($Warnings.Count -gt 0) {
        $Warnings -join [Environment]::NewLine
    } else {
        $null
    }
    $Result.Errors = if ($Errors.Count -gt 0) {
        $Errors -join [Environment]::NewLine
    } else {
        $null
    }
    [pscustomobject]$Result
}

if ($Errors.Count -gt 0) {
    exit 1
}
