#requires -Version 7.6

[CmdletBinding()]
param(
    [Parameter(Mandatory)][ValidateSet('GENERIC_COMMIT_PREPARATION', 'IMPLEMENTATION_TO_INDEPENDENT_FULL_REVIEW', 'FINDING_CORRECTION')][string]$Profile,
    [Parameter(Mandatory)][ValidateSet('COMMIT_PREPARATION_TO_COMMIT_APPROVAL', 'IMPLEMENTATION_TO_INDEPENDENT_FULL_REVIEW', 'BUNDLED_CORRECTION_TO_FOCUSED_DELTA_REVIEW')][string]$TransitionType,
    [Parameter(Mandatory)][ValidatePattern('^BL-[0-9]{3}$')][string]$TaskId,
    [Parameter(Mandatory)][string]$SourceDirectory,
    [Parameter(Mandatory)][AllowEmptyCollection()][string[]]$AllowedDeltaPath,
    [Parameter(Mandatory)][string]$PackagePath,
    [Parameter(Mandatory)][string]$AuthoritativeRepositoryRoot,
    [switch]$PreflightOnly,
    [switch]$FinalPackageContentOnly,
    [string]$StagingDirectory
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$exitCode = 2
$failureMessage = $null

try {
    $resolvedResultPath = [IO.Path]::GetFullPath($PackagePath)
    $resultParent = [IO.Path]::GetDirectoryName($resolvedResultPath)
    if (-not (Test-Path -LiteralPath $resultParent -PathType Container)) {
        throw "Capture result parent does not exist: $resultParent"
    }
    if (Test-Path -LiteralPath $resolvedResultPath) {
        throw "Capture result path must be new: $resolvedResultPath"
    }
    $payload = [ordered]@{
        schemaVersion = 1
        profile = $Profile
        transitionType = $TransitionType
        taskId = $TaskId
        sourceDirectory = $SourceDirectory
        allowedDeltaPath = @($AllowedDeltaPath)
        packagePath = $PackagePath
        authoritativeRepositoryRoot = $AuthoritativeRepositoryRoot
    }
    if ($PreflightOnly) {
        $payload['preflightOnly'] = $true
        $payload['stagingDirectory'] = $StagingDirectory
    }
    elseif ($FinalPackageContentOnly) {
        $payload['finalPackageContentOnly'] = $true
        $payload['stagingDirectory'] = $StagingDirectory
    }
    [IO.File]::WriteAllText(
        $resolvedResultPath,
        (($payload | ConvertTo-Json -Depth 8) + [Environment]::NewLine),
        [Text.UTF8Encoding]::new($false)
    )
    $exitCode = if ($TaskId -ceq 'BL-998') { 7 } else { 0 }
}
catch {
    $failureMessage = $_.Exception.Message
    $exitCode = 2
}
finally {
    if (-not [string]::IsNullOrWhiteSpace($failureMessage)) {
        [Console]::Error.WriteLine("Generator binding capture failed: $failureMessage")
    }
}

exit $exitCode
