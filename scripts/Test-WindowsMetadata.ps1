[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [string] $BinaryPath,

    [Parameter(Mandatory)]
    [string] $ExpectedProductVersion,

    [Parameter(Mandatory)]
    [string] $ExpectedFileVersion,

    [Parameter(Mandatory)]
    [ValidateSet('x64', 'arm64')]
    [string] $ExpectedPublicArch,

    [Parameter(Mandatory)]
    [ValidateSet('amd64', 'arm64')]
    [string] $ExpectedGOARCH,

    [Parameter(Mandatory)]
    [ValidatePattern('^[0-9a-f]{40}$')]
    [string] $ExpectedCommit,

    [Parameter(Mandatory)]
    [string] $ExpectedSourceTime,

    [Parameter(Mandatory)]
    [ValidateSet('true', 'false')]
    [string] $ExpectedModified,

    [Parameter(Mandatory)]
    [ValidateRange(1970, 9999)]
    [int] $ExpectedCopyrightYear,

    [ValidateRange(1, 60)]
    [int] $ArtifactTimeoutSeconds = 10,

    [ValidateRange(10, 300)]
    [int] $ToolTimeoutSeconds = 120,

    [ValidateRange(1024, 1048576)]
    [int] $MaximumOutputCharacters = 65536,

    [Parameter(DontShow)]
    [string] $TestLaunchMarkerPath
)

function Invoke-FlashGateWindowsMetadataMain {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string] $BinaryPath,

        [Parameter(Mandatory)]
        [string] $ExpectedProductVersion,

        [Parameter(Mandatory)]
        [string] $ExpectedFileVersion,

        [Parameter(Mandatory)]
        [ValidateSet('x64', 'arm64')]
        [string] $ExpectedPublicArch,

        [Parameter(Mandatory)]
        [ValidateSet('amd64', 'arm64')]
        [string] $ExpectedGOARCH,

        [Parameter(Mandatory)]
        [ValidatePattern('^[0-9a-f]{40}$')]
        [string] $ExpectedCommit,

        [Parameter(Mandatory)]
        [string] $ExpectedSourceTime,

        [Parameter(Mandatory)]
        [ValidateSet('true', 'false')]
        [string] $ExpectedModified,

        [Parameter(Mandatory)]
        [ValidateRange(1970, 9999)]
        [int] $ExpectedCopyrightYear,

        [ValidateRange(1, 60)]
        [int] $ArtifactTimeoutSeconds = 10,

        [ValidateRange(10, 300)]
        [int] $ToolTimeoutSeconds = 120,

        [ValidateRange(1024, 1048576)]
        [int] $MaximumOutputCharacters = 65536,

        [string] $TestLaunchMarkerPath,

        [Parameter(DontShow)]
        [scriptblock] $RuntimeProcessInvoker
    )

$ErrorActionPreference = 'Stop'
$RootPath = Split-Path -Parent $PSScriptRoot
$ExpectedIconPath = Join-Path $RootPath 'assets\branding\flashgate.ico'
$InputValidationScript = Join-Path `
    $RootPath `
    'scripts\Build-InputValidation.ps1'
$VerifierProcessScript = Join-Path `
    $RootPath `
    'scripts\VerifierProcess.ps1'

$Warnings = [System.Collections.Generic.List[string]]::new()
$Errors = [System.Collections.Generic.List[string]]::new()
$ExitCode = 1

$Result = [ordered]@{
    Status                 = 'FAIL'
    BinaryPath             = $BinaryPath
    ExpectedProductVersion = $ExpectedProductVersion
    ExpectedFileVersion    = $ExpectedFileVersion
    ExpectedPublicArch     = $ExpectedPublicArch
    FileDescription        = $null
    FileVersion            = $null
    ProductName            = $null
    ProductVersion         = $null
    CompanyName            = $null
    LegalCopyright         = $null
    OriginalFilename       = $null
    InternalName           = $null
    Comments               = $null
    Machine                = $null
    IconFrameCount         = $null
    IconFrameIdentity      = $null
    HostArchitecture       = $null
    TargetArchitecture     = $ExpectedPublicArch
    NativeExecutionEligible = $false
    ExecutionSkipReason    = 'NotEvaluated'
    RuntimeExecution       = 'SKIPPED'
    RuntimeFailureReason   = $null
    HelpContract           = 'SKIPPED'
    HelpSkipReason         = 'NotEvaluated'
    HelpFailureReason      = $null
    WarningCount           = 0
    ErrorCount             = 0
    Warnings               = $null
    Errors                 = $null
}

function Invoke-ProcessRequired {
    param(
        [Parameter(Mandatory)]
        [string] $FilePath,

        [Parameter(Mandatory)]
        [string[]] $Arguments,

        [Parameter(Mandatory)]
        [string] $WorkingDirectory,

        [Parameter(Mandatory)]
        [int] $TimeoutMilliseconds
    )

    $ProcessResult = Invoke-FlashGateBoundedProcess `
        -FilePath $FilePath `
        -Arguments $Arguments `
        -WorkingDirectory $WorkingDirectory `
        -TimeoutMilliseconds $TimeoutMilliseconds `
        -MaximumCharactersPerStream $MaximumOutputCharacters
    if ($ProcessResult.Status -cne 'PASS') {
        throw (
            "Required process failed safely. Reason=$($ProcessResult.FailureReason); " +
            "ExitCode=$($ProcessResult.ExitCode); " +
            "TimedOut=$($ProcessResult.TimedOut); " +
            "OutputLimitExceeded=$($ProcessResult.OutputLimitExceeded)."
        )
    }
    return $ProcessResult.Stdout
}

function Invoke-ArtifactProcess {
    param(
        [Parameter(Mandatory)]
        [string] $FilePath,

        [Parameter(Mandatory)]
        [string[]] $Arguments,

        [Parameter(Mandatory)]
        [string] $WorkingDirectory,

        [Parameter(Mandatory)]
        [int] $TimeoutMilliseconds
    )

    if ($null -ne $RuntimeProcessInvoker) {
        return & $RuntimeProcessInvoker `
            $FilePath `
            ([string[]]$Arguments) `
            $WorkingDirectory `
            $TimeoutMilliseconds `
            $MaximumOutputCharacters
    }
    return Invoke-FlashGateBoundedProcess `
        -FilePath $FilePath `
        -Arguments $Arguments `
        -WorkingDirectory $WorkingDirectory `
        -TimeoutMilliseconds $TimeoutMilliseconds `
        -MaximumCharactersPerStream $MaximumOutputCharacters
}

function Assert-Equal {
    param(
        [Parameter(Mandatory)]
        [string] $Name,

        [AllowNull()]
        [object] $Actual,

        [AllowNull()]
        [object] $Expected
    )

    if ([string]$Actual -cne [string]$Expected) {
        $Errors.Add(
            "$Name mismatch. Expected '$Expected'; found '$Actual'."
        )
    }
}

try {
    if (-not (Test-Path -LiteralPath $VerifierProcessScript -PathType Leaf)) {
        throw "Bounded process helper not found: $VerifierProcessScript"
    }
    . $VerifierProcessScript
    $ExecutionDecision = Get-FlashGateNativeExecutionDecision `
        -HostArchitecture (
            [Runtime.InteropServices.RuntimeInformation]::OSArchitecture
        ) `
        -TargetArchitecture $ExpectedPublicArch
    $Result.HostArchitecture = $ExecutionDecision.HostArchitecture
    $Result.TargetArchitecture = $ExecutionDecision.TargetArchitecture
    $Result.NativeExecutionEligible =
        $ExecutionDecision.NativeExecutionEligible
    $Result.ExecutionSkipReason = $ExecutionDecision.SkipReason
    $Result.HelpSkipReason = $ExecutionDecision.SkipReason
    if ($ExecutionDecision.HostArchitecture -ceq 'unsupported') {
        $Errors.Add('The Windows OS architecture is unsupported.')
    }

    if (-not (Test-Path -LiteralPath $BinaryPath -PathType Leaf)) {
        throw "Binary not found: $BinaryPath"
    }

    $ResolvedBinaryPath = (Resolve-Path -LiteralPath $BinaryPath).Path
    if (-not (Test-Path -LiteralPath $ExpectedIconPath -PathType Leaf)) {
        throw "Canonical icon not found: $ExpectedIconPath"
    }
    if (-not (Test-Path -LiteralPath $InputValidationScript -PathType Leaf)) {
        throw "Input validation helper not found: $InputValidationScript"
    }
    . $InputValidationScript
    if (
        -not [string]::IsNullOrWhiteSpace($TestLaunchMarkerPath) -and
        $env:FLASHGATE_VERIFIER_TEST_MODE -cne '1'
    ) {
        throw 'The launch marker is available only in verifier test mode.'
    }
    $VersionIdentity = Get-FlashGateSemanticVersion `
        -Value $ExpectedProductVersion
    if ($VersionIdentity.FileVersion -cne $ExpectedFileVersion) {
        throw (
            "Expected product/file version mapping is inconsistent: " +
            "$ExpectedProductVersion -> $ExpectedFileVersion"
        )
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

    $Stream = [IO.File]::OpenRead($ResolvedBinaryPath)
    try {
        $Reader = [IO.BinaryReader]::new($Stream)
        try {
            if ($Reader.ReadUInt16() -ne 0x5A4D) {
                throw 'Binary does not contain a valid DOS MZ header.'
            }

            $Stream.Position = 0x3C
            $PEOffset = $Reader.ReadInt32()
            $Stream.Position = $PEOffset

            if ($Reader.ReadUInt32() -ne 0x00004550) {
                throw 'Binary does not contain a valid PE signature.'
            }

            $Machine = $Reader.ReadUInt16()
        }
        finally {
            $Reader.Dispose()
        }
    }
    finally {
        $Stream.Dispose()
    }

    $ExpectedMachine = if ($ExpectedPublicArch -eq 'x64') {
        0x8664
    }
    else {
        0xAA64
    }

    if ($Machine -ne $ExpectedMachine) {
        $Errors.Add(
            ('PE machine mismatch. Expected 0x{0:X4}; found 0x{1:X4}.' -f
                $ExpectedMachine,
                $Machine)
        )
    }

    $Result.Machine = '0x{0:X4}' -f $Machine

    $VersionInfo = [System.Diagnostics.FileVersionInfo]::GetVersionInfo(
        $ResolvedBinaryPath
    )

    $ExpectedCopyright =
        "Copyright © $ExpectedCopyrightYear Thomas Weidner"

    Assert-Equal `
        -Name 'FileDescription' `
        -Actual $VersionInfo.FileDescription `
        -Expected 'FlashGate MCP Server'

    Assert-Equal `
        -Name 'FileVersion' `
        -Actual $VersionInfo.FileVersion `
        -Expected $ExpectedFileVersion

    Assert-Equal `
        -Name 'ProductName' `
        -Actual $VersionInfo.ProductName `
        -Expected 'FlashGate MCP'

    Assert-Equal `
        -Name 'ProductVersion' `
        -Actual $VersionInfo.ProductVersion `
        -Expected $ExpectedProductVersion

    Assert-Equal `
        -Name 'CompanyName' `
        -Actual $VersionInfo.CompanyName `
        -Expected 'Thomas Weidner'

    Assert-Equal `
        -Name 'LegalCopyright' `
        -Actual $VersionInfo.LegalCopyright `
        -Expected $ExpectedCopyright

    Assert-Equal `
        -Name 'OriginalFilename' `
        -Actual $VersionInfo.OriginalFilename `
        -Expected 'flashgate-mcp.exe'

    Assert-Equal `
        -Name 'InternalName' `
        -Actual $VersionInfo.InternalName `
        -Expected 'flashgate-mcp'

    Assert-Equal `
        -Name 'Comments' `
        -Actual $VersionInfo.Comments `
        -Expected 'Native Model Context Protocol server for controlled local system access.'

    $GoBuildInfo = Invoke-ProcessRequired `
        -FilePath 'go.exe' `
        -Arguments @('version', '-m', $ResolvedBinaryPath) `
        -WorkingDirectory $RootPath `
        -TimeoutMilliseconds ($ToolTimeoutSeconds * 1000)
    foreach ($ExpectedBuildSetting in @(
        "path`tgithub.com/thomasweidner/flashgate-mcp/cmd/server"
        "build`tGOOS=windows"
        "build`tGOARCH=$ExpectedGOARCH"
        "build`tCGO_ENABLED=0"
        "build`t-trimpath=true"
    )) {
        if (-not $GoBuildInfo.Contains(
            $ExpectedBuildSetting,
            [StringComparison]::Ordinal
        )) {
            $Errors.Add(
                "Go build information is missing: $ExpectedBuildSetting"
            )
        }
    }

    $ManifestOutput = Invoke-ProcessRequired `
        -FilePath 'go.exe' `
        -Arguments @(
            '-C'
            $RootPath
            'run'
            '-mod=vendor'
            './cmd/versionmanifest'
            '--binary'
            $ResolvedBinaryPath
            '--expected-version'
            $ExpectedProductVersion
            '--expected-file-version'
            $ExpectedFileVersion
            '--expected-commit'
            $ExpectedCommit
            '--expected-source-time'
            $ExpectedSourceTime
            '--expected-modified'
            $ExpectedModified
            '--expected-goos'
            'windows'
            '--expected-goarch'
            $ExpectedGOARCH
            '--expected-public-arch'
            $ExpectedPublicArch
        ) `
        -WorkingDirectory $RootPath `
        -TimeoutMilliseconds ($ToolTimeoutSeconds * 1000)
    if ($ManifestOutput -notmatch '(?m)^Status: PASS$') {
        $Errors.Add('Static build-manifest verifier did not report PASS.')
    }

    $IconOutput = Invoke-ProcessRequired `
        -FilePath 'go.exe' `
        -Arguments @(
            '-C'
            $RootPath
            'run'
            '-mod=vendor'
            './cmd/iconverify'
            '--binary'
            $ResolvedBinaryPath
            '--icon'
            $ExpectedIconPath
        ) `
        -WorkingDirectory $RootPath `
        -TimeoutMilliseconds ($ToolTimeoutSeconds * 1000)
    if ($IconOutput -notmatch '(?m)^Status: PASS$') {
        $Errors.Add('Icon identity verifier did not report PASS.')
    }
    if ($IconOutput -match '(?m)^FrameCount: (?<count>[0-9]+)$') {
        $Result.IconFrameCount = [int]$Matches['count']
    }
    else {
        $Errors.Add('Icon identity verifier did not report a frame count.')
    }
    if (
        $IconOutput -match
        '(?m)^FrameIdentitySHA256: (?<hash>[0-9a-f]{64})$'
    ) {
        $Result.IconFrameIdentity = $Matches['hash']
    }
    else {
        $Errors.Add('Icon identity verifier did not report its frame hash.')
    }

    $Result.FileDescription = $VersionInfo.FileDescription
    $Result.FileVersion = $VersionInfo.FileVersion
    $Result.ProductName = $VersionInfo.ProductName
    $Result.ProductVersion = $VersionInfo.ProductVersion
    $Result.CompanyName = $VersionInfo.CompanyName
    $Result.LegalCopyright = $VersionInfo.LegalCopyright
    $Result.OriginalFilename = $VersionInfo.OriginalFilename
    $Result.InternalName = $VersionInfo.InternalName
    $Result.Comments = $VersionInfo.Comments

    if ($Errors.Count -gt 0) {
        $Result.ExecutionSkipReason = 'StaticValidationFailed'
        $Result.HelpSkipReason = 'StaticValidationFailed'
    }
    elseif (-not $ExecutionDecision.NativeExecutionEligible) {
        $Result.ExecutionSkipReason = $ExecutionDecision.SkipReason
        $Result.HelpSkipReason = $ExecutionDecision.SkipReason
    }
    else {
        $Result.ExecutionSkipReason = $null
        $Result.HelpSkipReason = $null
        $RuntimeErrorCount = $Errors.Count
        $Result.RuntimeExecution = 'FAIL'
        if (-not [string]::IsNullOrWhiteSpace($TestLaunchMarkerPath)) {
            [IO.File]::WriteAllText(
                $TestLaunchMarkerPath,
                'runtime-attempted',
                [Text.UTF8Encoding]::new($false)
            )
        }
        $CompactResult = Invoke-ArtifactProcess `
            -FilePath $ResolvedBinaryPath `
            -Arguments @('--version') `
            -WorkingDirectory $RootPath `
            -TimeoutMilliseconds ($ArtifactTimeoutSeconds * 1000)
        $Result.RuntimeExecution = ConvertTo-FlashGateExecutionState `
            -Attempted $CompactResult.Attempted `
            -ProcessStatus $CompactResult.Status
        if ($CompactResult.Status -cne 'PASS') {
            $Result.RuntimeFailureReason = $CompactResult.FailureReason
            $Errors.Add(
                "Compact version execution failed safely: " +
                "$($CompactResult.FailureReason)."
            )
        }
        else {
            $CompactOutput = $CompactResult.Stdout.Trim()
            Assert-Equal `
                -Name 'Compact version output' `
                -Actual $CompactOutput `
                -Expected "flashgate-mcp $ExpectedProductVersion"
            if ($Errors.Count -gt $RuntimeErrorCount) {
                $Result.RuntimeFailureReason = 'CompactOutputMismatch'
            }
        }

        if ($Errors.Count -eq $RuntimeErrorCount) {
            $VerboseResult = Invoke-ArtifactProcess `
                -FilePath $ResolvedBinaryPath `
                -Arguments @('--version', '--verbose') `
                -WorkingDirectory $RootPath `
                -TimeoutMilliseconds ($ArtifactTimeoutSeconds * 1000)
            if ($VerboseResult.Status -cne 'PASS') {
                $Result.RuntimeFailureReason = $VerboseResult.FailureReason
                $Errors.Add(
                    "Verbose version execution failed safely: " +
                    "$($VerboseResult.FailureReason)."
                )
            }
            else {
                foreach ($ExpectedLine in @(
                    'Product:      FlashGate MCP'
                    "Version:      $ExpectedProductVersion"
                    "File version: $ExpectedFileVersion"
                    "Commit:       $ExpectedCommit"
                    "Source time:  $ExpectedSourceTime"
                    "Modified:     $ExpectedModified"
                    "Platform:     windows/$ExpectedPublicArch"
                    "Go target:    windows/$ExpectedGOARCH"
                )) {
                    if (-not $VerboseResult.Stdout.Contains(
                        $ExpectedLine,
                        [StringComparison]::Ordinal
                    )) {
                        $Errors.Add(
                            "Verbose version output is missing: $ExpectedLine"
                        )
                        $Result.RuntimeFailureReason =
                            'VerboseOutputMismatch'
                    }
                }
            }
        }
        if ($Errors.Count -eq $RuntimeErrorCount) {
            $Result.RuntimeExecution = 'PASS'
        }
        else {
            $Result.RuntimeExecution = 'FAIL'
        }

        if ($Result.RuntimeExecution -ceq 'PASS') {
            $HelpErrorCount = $Errors.Count
            $Result.HelpContract = 'FAIL'
            $HelpResult = Invoke-ArtifactProcess `
                -FilePath $ResolvedBinaryPath `
                -Arguments @('--help') `
                -WorkingDirectory $RootPath `
                -TimeoutMilliseconds ($ArtifactTimeoutSeconds * 1000)
            $Result.HelpContract = ConvertTo-FlashGateExecutionState `
                -Attempted $HelpResult.Attempted `
                -ProcessStatus $HelpResult.Status
            if ($HelpResult.Status -cne 'PASS') {
                $Result.HelpFailureReason = $HelpResult.FailureReason
                $Errors.Add(
                    "Help execution failed safely: " +
                    "$($HelpResult.FailureReason)."
                )
            }
            else {
                foreach (
                    $MissingHelpLine in
                    @(Get-FlashGateMissingHelpLines -Output $HelpResult.Stdout)
                ) {
                    $Errors.Add(
                        "Help output is missing: $MissingHelpLine"
                    )
                    $Result.HelpFailureReason = 'HelpContractMismatch'
                }
            }
            if ($Errors.Count -eq $HelpErrorCount) {
                $Result.HelpContract = 'PASS'
            }
            else {
                $Result.HelpContract = 'FAIL'
            }
        }
        else {
            $Result.HelpSkipReason = 'RuntimeValidationFailed'
        }
    }

    if ($Errors.Count -eq 0) {
        $Result.Status = if ($Warnings.Count -gt 0) {
            'PASS_WITH_WARNINGS'
        }
        else {
            'PASS'
        }
        $ExitCode = 0
    }
}
catch {
    $Errors.Add($_.Exception.Message)
    if ($Result.RuntimeExecution -ceq 'SKIPPED') {
        $Result.ExecutionSkipReason = 'StaticValidationFailed'
        $Result.HelpSkipReason = 'StaticValidationFailed'
    }
}
finally {
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

    $FinalResult = [pscustomobject]$Result
}

return [pscustomobject]@{
    ExitCode = $ExitCode
    Result   = $FinalResult
}
}

if ($MyInvocation.InvocationName -cne '.') {
    $InvocationResult = Invoke-FlashGateWindowsMetadataMain @PSBoundParameters
    $InvocationResult.Result | Format-List
    exit $InvocationResult.ExitCode
}
