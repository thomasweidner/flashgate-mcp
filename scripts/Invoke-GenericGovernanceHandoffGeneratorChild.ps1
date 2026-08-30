#requires -Version 7.6

[CmdletBinding()]
param(
    [Parameter(Mandatory)][string]$GeneratorPath,
    [Parameter(Mandatory)][ValidateSet('GENERIC_COMMIT_PREPARATION', 'IMPLEMENTATION_TO_INDEPENDENT_FULL_REVIEW', 'EVIDENCE_ONLY_FOCUSED_REVIEW', 'POST_MERGE_CLOSURE', 'FINDING_CORRECTION')][string]$Profile,
    [Parameter(Mandatory)][ValidateSet('COMMIT_PREPARATION_TO_COMMIT_APPROVAL', 'IMPLEMENTATION_TO_INDEPENDENT_FULL_REVIEW', 'EVIDENCE_ONLY_TO_FOCUSED_REVIEW', 'POST_MERGE_TO_DOCUMENTATION_CLOSURE', 'BUNDLED_CORRECTION_TO_FOCUSED_DELTA_REVIEW')][string]$TransitionType,
    [Parameter(Mandatory)][ValidatePattern('^BL-[0-9]{3}$')][string]$TaskId,
    [Parameter(Mandatory)][string]$SourceDirectory,
    [Parameter(Mandatory)][string]$AllowedDeltaPathBindingPath,
    [Parameter(Mandatory)][string]$PackagePath,
    [Parameter(Mandatory)][string]$AuthoritativeRepositoryRoot,
    [switch]$PreflightOnly,
    [switch]$FinalPackageContentOnly,
    [string]$StagingDirectory
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$document = $null
$exitCode = 2

try {
    $powerShellVersion = $PSVersionTable.PSVersion.ToString()
    if ($IsWindows) {
        if ($powerShellVersion -cne '7.6.5') {
            throw "Windows PowerShell 7.6.5 is required; actual=$powerShellVersion"
        }
    }
    elseif ($IsLinux) {
        $expectedLinuxPowerShellPath = '/home/weidnerthomas/voxtronic/tools/powershell/7.6.5/pwsh'
        $expectedLinuxPowerShellSha256 = 'D989CD1AB2EAD1BE3331DB2EEF38D209759128981873E6300653DF27BC7246C5'
        if ($powerShellVersion -cne '7.6.5') {
            throw "Native Linux PowerShell 7.6.5 is required; actual=$powerShellVersion"
        }
        $actualLinuxPowerShellPath = (Get-Process -Id $PID).Path
        if ($actualLinuxPowerShellPath -cne $expectedLinuxPowerShellPath) {
            throw "Native Linux PowerShell must use the managed executable; expected=$expectedLinuxPowerShellPath; actual=$actualLinuxPowerShellPath"
        }
        $actualLinuxPowerShellSha256 = (Get-FileHash -LiteralPath $actualLinuxPowerShellPath -Algorithm SHA256).Hash
        if ($actualLinuxPowerShellSha256 -cne $expectedLinuxPowerShellSha256) {
            throw "Managed native Linux PowerShell SHA-256 mismatch; expected=$expectedLinuxPowerShellSha256; actual=$actualLinuxPowerShellSha256"
        }
    }
    else {
        throw 'The generic governance generator child supports only Windows and native Linux.'
    }

    $resolvedGeneratorPath = [IO.Path]::GetFullPath($GeneratorPath)
    $resolvedBindingPath = [IO.Path]::GetFullPath($AllowedDeltaPathBindingPath)
    if (-not (Test-Path -LiteralPath $resolvedGeneratorPath -PathType Leaf)) {
        throw "Generator script does not exist: $resolvedGeneratorPath"
    }
    if (-not (Test-Path -LiteralPath $resolvedBindingPath -PathType Leaf)) {
        throw "AllowedDeltaPath binding does not exist: $resolvedBindingPath"
    }

    $strictUtf8 = [Text.UTF8Encoding]::new($false, $true)
    $bindingText = $strictUtf8.GetString([IO.File]::ReadAllBytes($resolvedBindingPath))
    $document = [Text.Json.JsonDocument]::Parse($bindingText)
    $root = $document.RootElement
    if ($root.ValueKind -cne [Text.Json.JsonValueKind]::Object) {
        throw 'AllowedDeltaPath binding must be exactly one JSON object.'
    }
    $propertyNames = @($root.EnumerateObject() | ForEach-Object Name)
    if (@($propertyNames).Count -ne 2 -or
        'schemaVersion' -cnotin $propertyNames -or
        'allowedDeltaPath' -cnotin $propertyNames) {
        throw 'AllowedDeltaPath binding properties must be exactly schemaVersion and allowedDeltaPath.'
    }
    if ($root.GetProperty('schemaVersion').ValueKind -cne [Text.Json.JsonValueKind]::Number -or
        $root.GetProperty('schemaVersion').GetInt32() -ne 1) {
        throw 'AllowedDeltaPath binding schemaVersion must be 1.'
    }
    $allowedElement = $root.GetProperty('allowedDeltaPath')
    if ($allowedElement.ValueKind -cne [Text.Json.JsonValueKind]::Array) {
        throw 'allowedDeltaPath must be a JSON array.'
    }
    $allowedDeltaPaths = [Collections.Generic.List[string]]::new()
    foreach ($element in $allowedElement.EnumerateArray()) {
        if ($element.ValueKind -cne [Text.Json.JsonValueKind]::String) {
            throw 'Every allowedDeltaPath element must be a JSON string.'
        }
        $allowedDeltaPaths.Add($element.GetString())
    }

    $boundParameters = [ordered]@{
        Profile = $Profile
        TransitionType = $TransitionType
        TaskId = $TaskId
        SourceDirectory = $SourceDirectory
        AllowedDeltaPath = [string[]]$allowedDeltaPaths.ToArray()
        PackagePath = $PackagePath
        AuthoritativeRepositoryRoot = $AuthoritativeRepositoryRoot
    }
    if ($PreflightOnly) {
        $boundParameters.PreflightOnly = $true
        $boundParameters.StagingDirectory = $StagingDirectory
    }
    elseif ($FinalPackageContentOnly) {
        $boundParameters.FinalPackageContentOnly = $true
        $boundParameters.StagingDirectory = $StagingDirectory
    }

    $global:LASTEXITCODE = 0
    & $resolvedGeneratorPath @boundParameters
    $exitCode = [int]$global:LASTEXITCODE
}
catch {
    [Console]::Error.WriteLine("Structured governance generator child failed: $($_.Exception.Message)")
    $exitCode = 2
}
finally {
    if ($null -ne $document) {
        $document.Dispose()
    }
}

exit $exitCode
