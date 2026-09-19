[CmdletBinding()]
param(
    [ValidateSet('windows', 'linux')]
    [string] $GOOS = 'windows',

    [ValidateSet('amd64', 'arm64')]
    [string] $GOARCH = 'amd64',

    [string] $Version = '',

    [string] $OutputPath = '',

    [switch] $Release
)

$ErrorActionPreference = 'Stop'

$RootPath = Split-Path -Parent $PSScriptRoot
$InputValidationScript = Join-Path $PSScriptRoot 'Build-InputValidation.ps1'
$VersionInfoCommand = Join-Path $RootPath 'cmd\versioninfo'
$ServerDirectory = Join-Path $RootPath 'cmd\server'
$IconPath = Join-Path $RootPath 'assets\branding\flashgate.ico'
$VendorPath = Join-Path $RootPath 'vendor'

$Warnings = [System.Collections.Generic.List[string]]::new()
$Errors = [System.Collections.Generic.List[string]]::new()

$ResourcePath = $null
$Commit = $null
$SourceTime = $null
$Modified = $false
$FileVersion = $null
$PublicArch = if ($GOARCH -eq 'amd64') { 'x64' } else { 'arm64' }
$ExitCode = 1

$OriginalGOOS = $env:GOOS
$OriginalGOARCH = $env:GOARCH
$OriginalCGOEnabled = $env:CGO_ENABLED

$Result = [ordered]@{
    Status       = 'FAIL'
    RootPath     = $RootPath
    GOOS         = $GOOS
    GOARCH       = $GOARCH
    PublicArch   = $PublicArch
    Version      = $null
    FileVersion  = $null
    Commit       = $null
    SourceTime   = $null
    Modified     = $null
    OutputPath   = $null
    ResourcePath = $null
    WarningCount = 0
    ErrorCount   = 0
    Warnings     = $null
    Errors       = $null
}

function Invoke-GitRequired {
    param(
        [Parameter(Mandatory)]
        [string[]] $Arguments
    )

    $Output = @(& git -C $RootPath @Arguments 2>&1)
    if ($LASTEXITCODE -ne 0) {
        throw ('git {0} failed with exit code {1}: {2}' -f ($Arguments -join ' '), $LASTEXITCODE, ($Output -join ' '))
    }

    return $Output
}

function Invoke-GoRequired {
    param(
        [Parameter(Mandatory)]
        [string[]] $Arguments
    )

    $Output = @(& go -C $RootPath @Arguments 2>&1)
    if ($LASTEXITCODE -ne 0) {
        throw ('go {0} failed with exit code {1}: {2}' -f ($Arguments -join ' '), $LASTEXITCODE, ($Output -join ' '))
    }

    return $Output
}

function ConvertTo-CanonicalUtc {
    param(
        [Parameter(Mandatory)]
        [string] $Value
    )

    $Parsed = [DateTimeOffset]::Parse(
        $Value,
        [Globalization.CultureInfo]::InvariantCulture,
        [Globalization.DateTimeStyles]::RoundtripKind
    )

    return $Parsed.UtcDateTime.ToString(
        'yyyy-MM-ddTHH:mm:ssZ',
        [Globalization.CultureInfo]::InvariantCulture
    )
}

function Resolve-Version {
    param(
        [string] $RequestedVersion
    )

    $ExactTag = $null
    $TagOutput = @(& git -C $RootPath describe --tags --exact-match HEAD 2>$null)
    if ($LASTEXITCODE -eq 0) {
        $ExactTag = ($TagOutput | Out-String).Trim()
    }

    if (-not [string]::IsNullOrWhiteSpace($RequestedVersion)) {
        $Resolved = $RequestedVersion
    }
    elseif ($ExactTag -match '^v(.+)$') {
        $Resolved = $Matches[1]
    }
    else {
        $Resolved = '0.0.0-dev'
    }

    $null = Get-FlashGateSemanticVersion -Value $Resolved

    if ($Release) {
        $ExpectedTag = "v$Resolved"
        if ($ExactTag -ne $ExpectedTag) {
            throw "Release builds require exact tag '$ExpectedTag'; current exact tag is '$ExactTag'."
        }
    }

    return $Resolved
}

try {
    if ($PSVersionTable.PSVersion.Major -ne 7 -or $PSVersionTable.PSVersion.Minor -ne 6) {
        $Warnings.Add(
            "PowerShell $($PSVersionTable.PSVersion) is in use; PowerShell 7.6.x is expected."
        )
    }

    foreach ($RequiredPath in @(
        $InputValidationScript
        $VersionInfoCommand
        $ServerDirectory
        $IconPath
        $VendorPath
    )) {
        if (-not (Test-Path -LiteralPath $RequiredPath)) {
            throw "Required build input not found: $RequiredPath"
        }
    }

    . $InputValidationScript

    $ExistingResources = @(
        Get-ChildItem `
            -LiteralPath $ServerDirectory `
            -Filter 'resource_windows_*.syso' `
            -File `
            -ErrorAction Stop
    )

    if ($ExistingResources.Count -gt 0) {
        throw ('Refusing to build with pre-existing Windows resource files: {0}' -f ($ExistingResources.FullName -join ', '))
    }

    $Version = Resolve-Version -RequestedVersion $Version
    $VersionIdentity = Get-FlashGateSemanticVersion -Value $Version
    $FileVersion = $VersionIdentity.FileVersion

    $Commit = (
        Invoke-GitRequired -Arguments @('rev-parse', 'HEAD') |
            Out-String
    ).Trim()

    if ($Commit -notmatch '^[0-9a-f]{40}$') {
        throw ('Unexpected Git commit format: {0}' -f [string]$Commit)
    }

    if (-not [string]::IsNullOrWhiteSpace($env:SOURCE_DATE_EPOCH)) {
        $SourceTime = ConvertFrom-FlashGateSourceDateEpoch `
            -Value $env:SOURCE_DATE_EPOCH
    }
    else {
        $CommitTime = (
            Invoke-GitRequired -Arguments @('show', '-s', '--format=%cI', 'HEAD') |
                Out-String
        ).Trim()

        $SourceTime = ConvertTo-CanonicalUtc -Value $CommitTime
    }

    $StatusLines = @(
        Invoke-GitRequired -Arguments @('status', '--porcelain=v1', '--untracked-files=normal')
    )
    $Modified = $StatusLines.Count -gt 0

    if ($Release -and $Modified) {
        throw 'Release builds require a clean working tree.'
    }

    if ([string]::IsNullOrWhiteSpace($OutputPath)) {
        $Extension = if ($GOOS -eq 'windows') { '.exe' } else { '' }
        $OutputPath = Join-Path `
            (Join-Path $RootPath "build\$($GOOS)_$PublicArch") `
            "flashgate-mcp$Extension"
    }
    elseif (-not [IO.Path]::IsPathRooted($OutputPath)) {
        $OutputPath = Join-Path $RootPath $OutputPath
    }

    New-Item `
        -ItemType Directory `
        -Path (Split-Path -Parent $OutputPath) `
        -Force |
        Out-Null

    if ($GOOS -eq 'windows') {
        $env:GOOS = $null
        $env:GOARCH = $null
        $env:CGO_ENABLED = $null

        $ResourcePath = Join-Path $ServerDirectory "resource_windows_$GOARCH.syso"

        $GeneratorArguments = @(
            'run'
            '-mod=vendor'
            './cmd/versioninfo'
            '-version'
            $Version
            '-source-time'
            $SourceTime
            '-goarch'
            $GOARCH
            '-output'
            $ResourcePath
            '-icon'
            $IconPath
        )

        $null = Invoke-GoRequired -Arguments $GeneratorArguments

        if (-not (Test-Path -LiteralPath $ResourcePath -PathType Leaf)) {
            throw "Windows resource was not generated: $ResourcePath"
        }
    }

    $LinkerPrefix = 'github.com/thomasweidner/flashgate-mcp/internal/version'
    $BuildManifest = @(
        'FLASHGATE_BUILD_MANIFEST_V1'
        "version=$Version"
        "fileVersion=$FileVersion"
        ('commit={0}' -f [string]$Commit)
        "sourceTime=$SourceTime"
        "modified=$($Modified.ToString().ToLowerInvariant())"
        "goos=$GOOS"
        "goarch=$GOARCH"
        "publicArch=$PublicArch"
        'END_FLASHGATE_BUILD_MANIFEST_V1'
    ) -join '|'
    $Ldflags = @(
        '-s'
        '-w'
        "-X $LinkerPrefix.version=$Version"
        "-X $LinkerPrefix.fileVersion=$FileVersion"
        ('-X {0}.commit={1}' -f $LinkerPrefix, [string]$Commit)
        "-X $LinkerPrefix.date=$SourceTime"
        "-X $LinkerPrefix.modified=$($Modified.ToString().ToLowerInvariant())"
        "-X $LinkerPrefix.buildManifest=$BuildManifest"
    ) -join ' '

    $env:GOOS = $GOOS
    $env:GOARCH = $GOARCH
    $env:CGO_ENABLED = '0'

    $BuildArguments = @(
        'build'
        '-mod=vendor'
        '-trimpath'
        '-buildvcs=true'
        '-ldflags'
        $Ldflags
        '-o'
        $OutputPath
        './cmd/server'
    )

    $null = Invoke-GoRequired -Arguments $BuildArguments

    if (-not (Test-Path -LiteralPath $OutputPath -PathType Leaf)) {
        throw "Build output was not created: $OutputPath"
    }

    $Result.Status = if ($Warnings.Count -gt 0) {
        'PASS_WITH_WARNINGS'
    }
    else {
        'PASS'
    }
    $ExitCode = 0
}
catch {
    $Errors.Add($_.Exception.Message)
}
finally {
    $env:GOOS = $OriginalGOOS
    $env:GOARCH = $OriginalGOARCH
    $env:CGO_ENABLED = $OriginalCGOEnabled

    if (
        -not [string]::IsNullOrWhiteSpace($ResourcePath) -and
        (Test-Path -LiteralPath $ResourcePath -PathType Leaf)
    ) {
        try {
            Remove-Item -LiteralPath $ResourcePath -Force
        }
        catch {
            $Errors.Add("Failed to remove generated resource '$ResourcePath': $($_.Exception.Message)")
            $Result.Status = 'FAIL'
            $ExitCode = 1
        }
    }

    $Result.Version = $Version
    $Result.FileVersion = $FileVersion
    $Result.Commit = $Commit
    $Result.SourceTime = $SourceTime
    $Result.Modified = $Modified
    $Result.OutputPath = $OutputPath
    $Result.ResourcePath = $ResourcePath
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
    else {
        $null
    }

    [pscustomobject]$Result | Format-List
}

exit $ExitCode
