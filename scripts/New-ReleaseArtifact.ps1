[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [string] $Version,

    [Parameter(Mandatory)]
    [ValidateSet('amd64', 'arm64')]
    [string] $GOARCH,

    [Parameter(Mandatory)]
    [string] $OutputDirectory,

    [switch] $Release
)

$ErrorActionPreference = 'Stop'

$RootPath = Split-Path -Parent $PSScriptRoot
$BuildScript = Join-Path $RootPath 'scripts\build.ps1'
$InputValidationScript = Join-Path `
    $RootPath `
    'scripts\Build-InputValidation.ps1'
$RequiredFiles = @(
    'LICENSE'
    'README.md'
    'THIRD-PARTY-NOTICES.md'
)
$PublicArch = if ($GOARCH -eq 'amd64') { 'x64' } else { 'arm64' }
$ArtifactBaseName = "flashgate-mcp_${Version}_windows_${PublicArch}"
$ExitCode = 1
$Warnings = [System.Collections.Generic.List[string]]::new()
$Errors = [System.Collections.Generic.List[string]]::new()

$Result = [ordered]@{
    Status       = 'FAIL'
    Version      = $Version
    GOARCH       = $GOARCH
    PublicArch   = $PublicArch
    ArchivePath  = $null
    ChecksumPath = $null
    Sha256       = $null
    WarningCount = 0
    ErrorCount   = 0
    Warnings     = $null
    Errors       = $null
}

function Invoke-ProcessRequired {
    param(
        [Parameter(Mandatory)]
        [string] $FilePath,

        [Parameter(Mandatory)]
        [string[]] $Arguments,

        [Parameter(Mandatory)]
        [string] $WorkingDirectory
    )

    $StartInfo = [System.Diagnostics.ProcessStartInfo]::new()
    $StartInfo.FileName = $FilePath
    $StartInfo.WorkingDirectory = $WorkingDirectory
    $StartInfo.UseShellExecute = $false
    $StartInfo.RedirectStandardOutput = $true
    $StartInfo.RedirectStandardError = $true
    $StartInfo.CreateNoWindow = $true

    foreach ($Argument in $Arguments) {
        $null = $StartInfo.ArgumentList.Add($Argument)
    }

    $Process = [System.Diagnostics.Process]::new()
    $Process.StartInfo = $StartInfo

    try {
        if (-not $Process.Start()) {
            throw "Unable to start process: $FilePath"
        }

        $StandardOutput = $Process.StandardOutput.ReadToEnd()
        $StandardError = $Process.StandardError.ReadToEnd()
        $Process.WaitForExit()
        $ProcessExitCode = $Process.ExitCode
    }
    finally {
        $Process.Dispose()
    }

    if ($ProcessExitCode -ne 0) {
        throw (
            '{0} {1} failed with exit code {2}: {3} {4}' -f
            $FilePath, ($Arguments -join ' '), $ProcessExitCode,
            $StandardError, $StandardOutput
        )
    }

    return $StandardOutput
}

function Resolve-SourceTimestamp {
    if (-not [string]::IsNullOrWhiteSpace($env:SOURCE_DATE_EPOCH)) {
        $Canonical = ConvertFrom-FlashGateSourceDateEpoch `
            -Value $env:SOURCE_DATE_EPOCH
        return [DateTimeOffset]::ParseExact(
            $Canonical,
            'yyyy-MM-ddTHH:mm:ssZ',
            [Globalization.CultureInfo]::InvariantCulture,
            [Globalization.DateTimeStyles]::AssumeUniversal
        )
    }

    $EpochText = (
        & git -C $RootPath show -s --format=%ct HEAD 2>&1 |
            Out-String
    ).Trim()
    if ($LASTEXITCODE -ne 0) {
        throw "Unable to resolve the Git commit epoch: $EpochText"
    }

    $Canonical = ConvertFrom-FlashGateSourceDateEpoch -Value $EpochText
    return [DateTimeOffset]::ParseExact(
        $Canonical,
        'yyyy-MM-ddTHH:mm:ssZ',
        [Globalization.CultureInfo]::InvariantCulture,
        [Globalization.DateTimeStyles]::AssumeUniversal
    )
}

try {
    foreach ($RequiredScript in @($BuildScript, $InputValidationScript)) {
        if (-not (Test-Path -LiteralPath $RequiredScript -PathType Leaf)) {
            throw "Required script not found: $RequiredScript"
        }
    }

    . $InputValidationScript
    $null = Get-FlashGateSemanticVersion -Value $Version
    if ($Release) {
        $RepositoryVersion = Get-FlashGateRepositoryVersion -RootPath $RootPath
        if ($Version -cne $RepositoryVersion) {
            throw "Release version assertion '$Version' differs from VERSION '$RepositoryVersion'."
        }
    }

    foreach ($RequiredFile in $RequiredFiles) {
        $RequiredPath = Join-Path $RootPath $RequiredFile
        if (-not (Test-Path -LiteralPath $RequiredPath -PathType Leaf)) {
            throw "Required release file not found: $RequiredPath"
        }
        if ((Get-Item -LiteralPath $RequiredPath).Length -eq 0) {
            throw "Required release file is empty: $RequiredPath"
        }
    }

    if (-not [IO.Path]::IsPathRooted($OutputDirectory)) {
        $OutputDirectory = Join-Path $RootPath $OutputDirectory
    }
    $OutputDirectory = [IO.Path]::GetFullPath($OutputDirectory)
    New-Item -ItemType Directory -Path $OutputDirectory -Force | Out-Null

    $SourceTimestamp = Resolve-SourceTimestamp
    if ($SourceTimestamp.Year -lt 1980) {
        throw 'The source timestamp is too old for the ZIP file format.'
    }

    $WorkDirectory = Join-Path $OutputDirectory ".${ArtifactBaseName}.work"
    $StageDirectory = Join-Path $WorkDirectory $ArtifactBaseName
    $BinaryPath = Join-Path $StageDirectory 'flashgate-mcp.exe'
    $ArchivePath = Join-Path $OutputDirectory "$ArtifactBaseName.zip"
    $ChecksumPath = "$ArchivePath.sha256"

    if (Test-Path -LiteralPath $WorkDirectory) {
        Remove-Item -LiteralPath $WorkDirectory -Recurse -Force
    }
    foreach ($ExistingPath in @($ArchivePath, $ChecksumPath)) {
        if (Test-Path -LiteralPath $ExistingPath) {
            Remove-Item -LiteralPath $ExistingPath -Force
        }
    }

    New-Item -ItemType Directory -Path $StageDirectory -Force | Out-Null

    $BuildArguments = @(
        '-NoLogo'
        '-NoProfile'
        '-File'
        $BuildScript
        '-GOOS'
        'windows'
        '-GOARCH'
        $GOARCH
        '-Version'
        $Version
        '-OutputPath'
        $BinaryPath
    )
    if ($Release) {
        $BuildArguments += '-Release'
    }

    $null = Invoke-ProcessRequired `
        -FilePath (Get-Process -Id $PID).Path `
        -Arguments $BuildArguments `
        -WorkingDirectory $RootPath

    if (-not (Test-Path -LiteralPath $BinaryPath -PathType Leaf)) {
        throw "Windows binary was not created: $BinaryPath"
    }

    foreach ($RequiredFile in $RequiredFiles) {
        Copy-Item `
            -LiteralPath (Join-Path $RootPath $RequiredFile) `
            -Destination (Join-Path $StageDirectory $RequiredFile) `
            -Force
    }

    Add-Type -AssemblyName System.IO.Compression
    Add-Type -AssemblyName System.IO.Compression.FileSystem

    $ArchiveStream = [IO.File]::Open(
        $ArchivePath,
        [IO.FileMode]::CreateNew,
        [IO.FileAccess]::ReadWrite,
        [IO.FileShare]::None
    )
    try {
        $Archive = [IO.Compression.ZipArchive]::new(
            $ArchiveStream,
            [IO.Compression.ZipArchiveMode]::Create,
            $false,
            [Text.Encoding]::UTF8
        )
        try {
            foreach ($RelativePath in @(
                'LICENSE'
                'README.md'
                'THIRD-PARTY-NOTICES.md'
                'flashgate-mcp.exe'
            )) {
                $SourcePath = Join-Path $StageDirectory $RelativePath
                $EntryName = "$ArtifactBaseName/$RelativePath"
                $Entry = $Archive.CreateEntry(
                    $EntryName,
                    [IO.Compression.CompressionLevel]::Optimal
                )
                $Entry.LastWriteTime = $SourceTimestamp

                $EntryStream = $Entry.Open()
                $SourceStream = [IO.File]::OpenRead($SourcePath)
                try {
                    $SourceStream.CopyTo($EntryStream)
                }
                finally {
                    $SourceStream.Dispose()
                    $EntryStream.Dispose()
                }
            }
        }
        finally {
            $Archive.Dispose()
        }
    }
    finally {
        $ArchiveStream.Dispose()
    }

    $Hash = (
        Get-FileHash -LiteralPath $ArchivePath -Algorithm SHA256
    ).Hash.ToLowerInvariant()
    $ChecksumLine = "$Hash  $([IO.Path]::GetFileName($ArchivePath))`n"
    [IO.File]::WriteAllText(
        $ChecksumPath,
        $ChecksumLine,
        [Text.UTF8Encoding]::new($false)
    )

    $Result.Status = 'PASS'
    $Result.ArchivePath = $ArchivePath
    $Result.ChecksumPath = $ChecksumPath
    $Result.Sha256 = $Hash
    $ExitCode = 0
}
catch {
    $Errors.Add($_.Exception.Message)
}
finally {
    if (
        -not [string]::IsNullOrWhiteSpace($WorkDirectory) -and
        (Test-Path -LiteralPath $WorkDirectory)
    ) {
        try {
            Remove-Item -LiteralPath $WorkDirectory -Recurse -Force
        }
        catch {
            $Errors.Add("Failed to remove staging directory: $($_.Exception.Message)")
            $Result.Status = 'FAIL'
            $ExitCode = 1
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
    else {
        $null
    }

    [pscustomobject]$Result | Format-List
}

exit $ExitCode
