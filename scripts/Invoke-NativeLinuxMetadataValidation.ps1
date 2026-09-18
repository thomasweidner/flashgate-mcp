[CmdletBinding()]
param(
    [string] $RootPath = (Split-Path -Parent $PSScriptRoot),

    [string] $DistroName = 'Ubuntu-24.04',

    [Parameter(Mandatory)]
    [string] $OutputRoot,

    [string] $Version = '1.2.3-rc.1',

    [string] $WorkingPath
)

$ErrorActionPreference = 'Stop'
Import-Module (Join-Path $PSScriptRoot 'TaskBoundWorkRoot.psm1') -Force
$WorkRoot = Resolve-FlashGateWorkRoot -WorkingPath $WorkingPath

$Timestamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$WorkingDirectory = New-FlashGateScratchDirectory -WorkingPath $WorkRoot -Prefix 'native-linux'
$SnapshotPath = Join-Path $WorkingDirectory 'working-tree.tar'
$SnapshotManifestPath = Join-Path $WorkingDirectory 'working-tree.manifest.json'
$DriverPath = Join-Path $WorkingDirectory 'native-driver.sh'
$DriverTemplatePath = Join-Path $PSScriptRoot 'linux-native-driver.sh'
$SnapshotScriptPath = Join-Path $PSScriptRoot 'New-GitSnapshot.ps1'
$SnapshotValidatorTemplatePath = Join-Path $PSScriptRoot 'validate-snapshot.py'
$SnapshotValidatorPath = Join-Path $WorkingDirectory 'validate-snapshot.py'
$SafetyLibraryTemplatePath = Join-Path $PSScriptRoot 'native-validation-safety.sh'
$SafetyLibraryPath = Join-Path $WorkingDirectory 'native-validation-safety.sh'
$SafetyHelperTemplatePath = Join-Path $PSScriptRoot 'safe-work-root.py'
$SafetyHelperPath = Join-Path $WorkingDirectory 'safe-work-root.py'
$SummaryPath = Join-Path $OutputRoot 'native-summary.env'

$Warnings = [System.Collections.Generic.List[string]]::new()
$Errors = [System.Collections.Generic.List[string]]::new()

$Result = [ordered]@{
    Status            = 'FAIL'
    RootPath          = $RootPath
    DistroName        = $DistroName
    OutputRoot        = $OutputRoot
    SummaryPath       = $SummaryPath
    LinuxX64Binary    = $null
    LinuxARM64Binary  = $null
    NativeRoot        = $null
    GoVersion         = $null
    LinuxX64Sha256    = $null
    LinuxARM64Sha256  = $null
    LinuxCoverage     = $null
    LinuxX64Archive   = $null
    LinuxARM64Archive = $null
    LinuxX64ArchiveSha256 = $null
    LinuxARM64ArchiveSha256 = $null
    WarningCount      = 0
    ErrorCount        = 0
    Warnings          = $null
    Errors            = $null
}

function Invoke-ExternalRequired {
    param(
        [Parameter(Mandatory)]
        [string] $FilePath,

        [Parameter(Mandatory)]
        [string[]] $Arguments,

        [string] $WorkingDirectory = $RootPath
    )

    $PreviousLocation = Get-Location
    try {
        Set-Location -LiteralPath $WorkingDirectory
        $Output = @(& $FilePath @Arguments 2>&1)
        $ExitCode = $LASTEXITCODE
    }
    finally {
        Set-Location -LiteralPath $PreviousLocation
    }

    if ($ExitCode -ne 0) {
        throw ('{0} {1} failed with exit code {2}: {3}' -f
            $FilePath, ($Arguments -join ' '), $ExitCode, ($Output -join ' '))
    }

    return $Output
}

function ConvertTo-WslPath {
    param(
        [Parameter(Mandatory)]
        [string] $WindowsPath
    )

    $FullWindowsPath = [System.IO.Path]::GetFullPath($WindowsPath)
    $PathRoot = [System.IO.Path]::GetPathRoot($FullWindowsPath)
    $DirectorySeparator = [System.IO.Path]::DirectorySeparatorChar

    $IsLocalDriveRoot = (
        $PathRoot.Length -eq 3 -and
        [char]::IsLetter($PathRoot[0]) -and
        $PathRoot[1] -eq ':' -and
        $PathRoot[2] -eq $DirectorySeparator
    )

    if (-not $IsLocalDriveRoot) {
        throw "Only local drive-letter paths can be mapped to WSL: $FullWindowsPath"
    }

    $DriveLetter = [char]::ToLowerInvariant($PathRoot[0])
    $RelativePath = $FullWindowsPath.Substring($PathRoot.Length)
    $RelativeWslPath = $RelativePath.Replace(
        $DirectorySeparator,
        [char]'/'
    )

    if ([string]::IsNullOrEmpty($RelativeWslPath)) {
        return "/mnt/$DriveLetter"
    }

    return "/mnt/$DriveLetter/$RelativeWslPath"
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
            ) `
            -WorkingDirectory $RootPath |
            Out-String
    ).Trim()
    $HeadCommit = (
        Invoke-ExternalRequired `
            -FilePath 'git.exe' `
            -Arguments @('-C', $RootPath, 'rev-parse', 'HEAD') `
            -WorkingDirectory $RootPath |
            Out-String
    ).Trim()
    if (-not (Test-Path -LiteralPath $GitCommonDirectory -PathType Container)) {
        throw ('Git common directory not found: {0}' -f $GitCommonDirectory)
    }
    if ($HeadCommit -notmatch '^[0-9a-f]{40}$') {
        throw ('Unexpected Git HEAD format: {0}' -f $HeadCommit)
    }

    $RootWslPreflight = ConvertTo-WslPath -WindowsPath $RootPath
    $DriverTemplateWslPreflight = ConvertTo-WslPath `
        -WindowsPath $DriverTemplatePath
    $ExpectedDriverTemplateWslPreflight = (
        $RootWslPreflight.TrimEnd('/') +
        '/scripts/linux-native-driver.sh'
    )

    if (
        $DriverTemplateWslPreflight -ne
        $ExpectedDriverTemplateWslPreflight
    ) {
        throw (
            "WSL path mapping preflight failed. Expected: " +
            "$ExpectedDriverTemplateWslPreflight; found: " +
            "$DriverTemplateWslPreflight"
        )
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
        -Arguments @(
            '-d'
            $DistroName
            '--'
            'true'
        ) `
        -WorkingDirectory $RootPath

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
        @{
            Type = '-d'
            Path = $RootWslPath
            Name = 'repository root'
        }
        @{
            Type = '-f'
            Path = $SnapshotWslPath
            Name = 'working-tree snapshot'
        }
        @{
            Type = '-f'
            Path = $SnapshotManifestWslPath
            Name = 'snapshot manifest'
        }
        @{
            Type = '-d'
            Path = $OutputWslPath
            Name = 'native output directory'
        }
        @{
            Type = '-f'
            Path = $DriverWslPath
            Name = 'native driver'
        }
        @{
            Type = '-f'
            Path = $SnapshotValidatorWslPath
            Name = 'snapshot validator'
        }
        @{
            Type = '-f'
            Path = $SafetyLibraryWslPath
            Name = 'safety library'
        }
        @{
            Type = '-f'
            Path = $SafetyHelperWslPath
            Name = 'work-root helper'
        }
        @{
            Type = '-d'
            Path = $GitCommonDirectoryWslPath
            Name = 'Git common directory'
        }
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
                ) `
                -WorkingDirectory $RootPath
        }
        catch {
            throw "Converted WSL path is invalid for $($PathCheck.Name): $($PathCheck.Path)"
        }
    }

    if (-not $DriverWslPath.EndsWith(
        '/native-driver.sh',
        [System.StringComparison]::Ordinal
    )) {
        throw "Unexpected converted native-driver path: $DriverWslPath"
    }

    $RunId = "flashgate-file-properties-$Timestamp"
    $WindowsUserPath = [Environment]::GetFolderPath('UserProfile')
    if ([string]::IsNullOrWhiteSpace($WindowsUserPath)) {
        throw 'The Windows user-profile path could not be resolved for leak validation.'
    }
    $WindowsUserWslPath = ConvertTo-WslPath -WindowsPath $WindowsUserPath
    $SyncRoots = @(
        'OneDrive'
        'OneDriveCommercial'
        'OneDriveConsumer'
    ) | ForEach-Object { [Environment]::GetEnvironmentVariable($_) } |
        Where-Object { -not [string]::IsNullOrWhiteSpace($_) } |
        Select-Object -Unique

    $WslOutput = @(
        Invoke-ExternalRequired `
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
                "FG_VERSION=$Version"
                "FG_FORBIDDEN_WINDOWS_USER=$WindowsUserPath"
                "FG_FORBIDDEN_WSL_USER=$WindowsUserWslPath"
                ('FG_FORBIDDEN_SYNC_ROOTS={0}' -f ($SyncRoots -join "`n"))
                'bash'
                $DriverWslPath
            ) `
            -WorkingDirectory $RootPath
    )

    if (-not (Test-Path -LiteralPath $SummaryPath -PathType Leaf)) {
        throw "Native summary was not created: $SummaryPath"
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
        throw "Native validation did not report PASS: $($Summary['ERROR'])"
    }

    foreach ($RequiredKey in @(
        'NATIVE_ROOT'
        'GO_VERSION'
        'LINUX_X64_SHA256'
        'LINUX_ARM64_SHA256'
        'LINUX_X64_BUILD_ID'
        'LINUX_ARM64_BUILD_ID'
        'LINUX_COVERAGE'
        'LINUX_X64_ARCHIVE_SHA256'
        'LINUX_ARM64_ARCHIVE_SHA256'
    )) {
        if ([string]::IsNullOrWhiteSpace($Summary[$RequiredKey])) {
            throw "Native summary value is missing: $RequiredKey"
        }
    }

    $Result.Status = if ($Warnings.Count -gt 0) {
        'PASS_WITH_WARNINGS'
    }
    else {
        'PASS'
    }

    $Result.LinuxX64Binary = Join-Path $OutputRoot "flashgate-mcp_$Version`_linux_x64"
    $Result.LinuxARM64Binary = Join-Path $OutputRoot "flashgate-mcp_$Version`_linux_arm64"
    $Result.NativeRoot = $Summary['NATIVE_ROOT']
    $Result.GoVersion = $Summary['GO_VERSION']
    $Result.LinuxX64Sha256 = $Summary['LINUX_X64_SHA256']
    $Result.LinuxARM64Sha256 = $Summary['LINUX_ARM64_SHA256']
    $Result.LinuxCoverage = $Summary['LINUX_COVERAGE']
    $Result.LinuxX64Archive = Join-Path $OutputRoot "flashgate-mcp_$Version`_linux_x64.tar.gz"
    $Result.LinuxARM64Archive = Join-Path $OutputRoot "flashgate-mcp_$Version`_linux_arm64.tar.gz"
    $Result.LinuxX64ArchiveSha256 = $Summary['LINUX_X64_ARCHIVE_SHA256']
    $Result.LinuxARM64ArchiveSha256 = $Summary['LINUX_ARM64_ARCHIVE_SHA256']
}
catch {
    $Errors.Add($_.Exception.Message)
}
finally {
    if (Test-Path -LiteralPath $WorkingDirectory -PathType Container) {
        try {
            Remove-FlashGateScratchDirectory -Path $WorkingDirectory -WorkingPath $WorkRoot -Prefix 'native-linux'
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
    }
    else {
        $null
    }
    $Result.Errors = if ($Errors.Count -gt 0) {
        $Errors -join [Environment]::NewLine
    }
    [pscustomobject]$Result | Format-List
}

if ($Errors.Count -gt 0) {
    exit 1
}
