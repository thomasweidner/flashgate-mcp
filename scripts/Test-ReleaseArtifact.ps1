[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [string] $ArchivePath,

    [Parameter(Mandatory)]
    [string] $ChecksumPath,

    [Parameter(Mandatory)]
    [string] $ExpectedVersion,

    [Parameter(Mandatory)]
    [ValidateSet('x64', 'arm64')]
    [string] $ExpectedPublicArch,

    [Parameter(Mandatory)]
    [ValidateSet('true', 'false')]
    [string] $ExpectedModified,

    [Parameter(Mandatory)]
    [ValidatePattern('^[0-9a-f]{40}$')]
    [string] $ExpectedCommit,

    [Parameter(Mandatory)]
    [string] $ExpectedSourceTime,

    [Parameter(Mandatory)]
    [ValidateSet('windows')]
    [string] $ExpectedPlatform,

    [Parameter(Mandatory)]
    [ValidateSet('amd64', 'arm64')]
    [string] $ExpectedGOARCH,

    [Parameter(Mandatory)]
    [ValidateRange(1970, 9999)]
    [int] $ExpectedCopyrightYear,

    [string] $WorkingPath
)

$ErrorActionPreference = 'Stop'
Import-Module (Join-Path $PSScriptRoot 'TaskBoundWorkRoot.psm1') -Force
$WorkRoot = Resolve-FlashGateWorkRoot -WorkingPath $WorkingPath
$RootPath = Split-Path -Parent $PSScriptRoot
$MetadataScript = Join-Path $RootPath 'scripts\Test-WindowsMetadata.ps1'
$InputValidationScript = Join-Path `
    $RootPath `
    'scripts\Build-InputValidation.ps1'
$ExitCode = 1
$ExtractionRoot = $null
$Warnings = [System.Collections.Generic.List[string]]::new()
$Errors = [System.Collections.Generic.List[string]]::new()

$Result = [ordered]@{
    Status       = 'FAIL'
    ArchivePath  = $ArchivePath
    ChecksumPath = $ChecksumPath
    Version      = $ExpectedVersion
    PublicArch   = $ExpectedPublicArch
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
        throw ('{0} {1} failed with exit code {2}: {3} {4}' -f
            $FilePath, ($Arguments -join ' '), $ProcessExitCode, $StandardError, $StandardOutput)
    }

    return $StandardOutput
}

try {
    foreach ($RequiredPath in @(
        $ArchivePath
        $ChecksumPath
        $MetadataScript
        $InputValidationScript
    )) {
        if (-not (Test-Path -LiteralPath $RequiredPath -PathType Leaf)) {
            throw "Required file not found: $RequiredPath"
        }
    }
    . $InputValidationScript
    $VersionIdentity = Get-FlashGateSemanticVersion -Value $ExpectedVersion
    $ParsedSourceTime = [DateTimeOffset]::MinValue
    if (
        -not [DateTimeOffset]::TryParseExact(
            $ExpectedSourceTime,
            'yyyy-MM-ddTHH:mm:ssZ',
            [Globalization.CultureInfo]::InvariantCulture,
            [Globalization.DateTimeStyles]::AssumeUniversal,
            [ref]$ParsedSourceTime
        )
    ) {
        throw "Expected source time is not canonical RFC3339 UTC: $ExpectedSourceTime"
    }
    $MappedGOARCH = if ($ExpectedPublicArch -eq 'x64') {
        'amd64'
    } else {
        'arm64'
    }
    if ($ExpectedGOARCH -cne $MappedGOARCH) {
        throw (
            "Architecture mapping mismatch. '$ExpectedPublicArch' requires " +
            "'$MappedGOARCH', not '$ExpectedGOARCH'."
        )
    }

    $ExpectedBaseName =
        "flashgate-mcp_${ExpectedVersion}_windows_${ExpectedPublicArch}"
    $ExpectedArchiveName = "$ExpectedBaseName.zip"
    if ([IO.Path]::GetFileName($ArchivePath) -cne $ExpectedArchiveName) {
        throw (
            "Archive name mismatch. Expected '$ExpectedArchiveName'; found " +
            "'$([IO.Path]::GetFileName($ArchivePath))'."
        )
    }
    if (
        [IO.Path]::GetFileName($ChecksumPath) -cne
        "$ExpectedArchiveName.sha256"
    ) {
        throw 'Checksum filename does not match the release contract.'
    }

    $ExtractionRoot = New-FlashGateScratchDirectory -WorkingPath $WorkRoot -Prefix 'release-validate'
    $ArchiveUnderTest = Join-Path $ExtractionRoot $ExpectedArchiveName
    $ChecksumUnderTest = "$ArchiveUnderTest.sha256"
    Copy-Item -LiteralPath $ArchivePath -Destination $ArchiveUnderTest
    Copy-Item -LiteralPath $ChecksumPath -Destination $ChecksumUnderTest

    $ActualHash = (
        Get-FileHash -LiteralPath $ArchiveUnderTest -Algorithm SHA256
    ).Hash.ToLowerInvariant()
    $ChecksumText = [IO.File]::ReadAllText($ChecksumUnderTest).Trim()
    $ExpectedChecksumText = "$ActualHash  $ExpectedArchiveName"
    if ($ChecksumText -cne $ExpectedChecksumText) {
        throw 'Checksum file content does not match the archive.'
    }
    $Result.Sha256 = $ActualHash

    Add-Type -AssemblyName System.IO.Compression
    Add-Type -AssemblyName System.IO.Compression.FileSystem

    $ExpectedEntries = @(
        "$ExpectedBaseName/LICENSE"
        "$ExpectedBaseName/README.md"
        "$ExpectedBaseName/THIRD-PARTY-NOTICES.md"
        "$ExpectedBaseName/flashgate-mcp.exe"
    ) | Sort-Object

    $Archive = [IO.Compression.ZipFile]::OpenRead($ArchiveUnderTest)
    try {
        $ActualEntries = @(
            $Archive.Entries |
                ForEach-Object {
                    if (
                        $_.FullName.StartsWith('/') -or
                        $_.FullName.Contains('..') -or
                        $_.FullName.Contains('\')
                    ) {
                        throw "Unsafe ZIP entry: $($_.FullName)"
                    }
                    $_.FullName
                } |
                Sort-Object
        )
    }
    finally {
        $Archive.Dispose()
    }

    if (($ActualEntries -join "`n") -cne ($ExpectedEntries -join "`n")) {
        throw ('Unexpected ZIP content. Expected: {0}; found: {1}' -f
            ($ExpectedEntries -join ', '), ($ActualEntries -join ', '))
    }

    try {
        $InventoryReport = Join-Path $ExtractionRoot 'inventory.json'
        $null = Invoke-ProcessRequired `
            -FilePath 'go.exe' `
            -Arguments @(
                '-C'
                $RootPath
                'run'
                '-mod=vendor'
                './cmd/releaseaudit'
                'inventory'
                '--artifact'
                $ArchiveUnderTest
                '--report'
                $InventoryReport
            ) `
            -WorkingDirectory $RootPath

        $Inventory = Get-Content -LiteralPath $InventoryReport -Raw |
            ConvertFrom-Json -Depth 10
        if ($Inventory.status -cne 'PASS') {
            throw 'ZIP entry path/type audit did not report PASS.'
        }
        $ExpectedTypes = @(
            $ExpectedEntries |
                ForEach-Object { "$_`tfile" } |
                Sort-Object
        )
        $ActualTypes = @(
            $Inventory.entries |
                ForEach-Object { "$($_.path)`t$($_.type)" } |
                Sort-Object
        )
        if (
            ($ExpectedTypes -join "`n") -cne
            ($ActualTypes -join "`n")
        ) {
            throw 'ZIP entry types do not match the release contract.'
        }

        $ExtractedRoot = Join-Path $ExtractionRoot 'extracted'
        [IO.Compression.ZipFile]::ExtractToDirectory(
            $ArchiveUnderTest,
            $ExtractedRoot
        )
        $ArtifactRoot = Join-Path $ExtractedRoot $ExpectedBaseName
        $BinaryPath = Join-Path $ArtifactRoot 'flashgate-mcp.exe'

        foreach ($RequiredFile in @(
            'LICENSE'
            'README.md'
            'THIRD-PARTY-NOTICES.md'
            'flashgate-mcp.exe'
        )) {
            $Path = Join-Path $ArtifactRoot $RequiredFile
            if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
                throw "Extracted release file not found: $Path"
            }
            if ((Get-Item -LiteralPath $Path).Length -eq 0) {
                throw "Extracted release file is empty: $Path"
            }
        }

        $ExpectedFileVersion = $VersionIdentity.FileVersion

        $MetadataArguments = @(
            '-NoLogo'
            '-NoProfile'
            '-File'
            $MetadataScript
            '-BinaryPath'
            $BinaryPath
            '-ExpectedProductVersion'
            $ExpectedVersion
            '-ExpectedFileVersion'
            $ExpectedFileVersion
            '-ExpectedPublicArch'
            $ExpectedPublicArch
            '-ExpectedGOARCH'
            $ExpectedGOARCH
            '-ExpectedCommit'
            $ExpectedCommit
            '-ExpectedSourceTime'
            $ExpectedSourceTime
            '-ExpectedModified'
            $ExpectedModified
            '-ExpectedCopyrightYear'
            $ExpectedCopyrightYear.ToString(
                [Globalization.CultureInfo]::InvariantCulture
            )
        )

        $null = Invoke-ProcessRequired `
            -FilePath (Get-Process -Id $PID).Path `
            -Arguments $MetadataArguments `
            -WorkingDirectory $RootPath

        if ($ExpectedPublicArch -eq 'x64') {
            $CompactOutput = @(& $BinaryPath --version 2>&1)
            if ($LASTEXITCODE -ne 0) {
                throw 'Extracted x64 binary failed its compact version command.'
            }
            if (($CompactOutput -join "`n").Trim() -cne "flashgate-mcp $ExpectedVersion") {
                throw 'Extracted x64 binary returned an unexpected compact version.'
            }

            $VerboseOutput = @(& $BinaryPath --version --verbose 2>&1)
            if ($LASTEXITCODE -ne 0) {
                throw 'Extracted x64 binary failed its verbose version command.'
            }
            $VerboseText = $VerboseOutput -join "`n"
            foreach ($ExpectedLine in @(
                "Version:      $ExpectedVersion"
                "Commit:       $ExpectedCommit"
                "Source time:  $ExpectedSourceTime"
                "Modified:     $ExpectedModified"
                "Platform:     $ExpectedPlatform/$ExpectedPublicArch"
                "Go target:    $ExpectedPlatform/$ExpectedGOARCH"
            )) {
                if ($VerboseText -notmatch [regex]::Escape($ExpectedLine)) {
                    throw "Verbose release output is missing: $ExpectedLine"
                }
            }
        }
    }
    finally {
        if (
            -not [string]::IsNullOrWhiteSpace($ExtractionRoot) -and
            (Test-Path -LiteralPath $ExtractionRoot)
        ) {
            Remove-FlashGateScratchDirectory -Path $ExtractionRoot -WorkingPath $WorkRoot -Prefix 'release-validate'
        }
    }

    $Result.Status = 'PASS'
    $ExitCode = 0
}
catch {
    $Errors.Add($_.Exception.Message)
}
finally {
    if (
        -not [string]::IsNullOrWhiteSpace($ExtractionRoot) -and
        (Test-Path -LiteralPath $ExtractionRoot)
    ) {
        $Errors.Add("Controlled validation directory remains after fail-closed cleanup: $ExtractionRoot")
        $ExitCode = 1
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
