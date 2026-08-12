#requires -Version 7.6
[CmdletBinding()]
param(
    [Parameter(Mandatory)][string]$RequestPath,
    [Parameter(Mandatory)][string]$ResultPath,
    [string]$RepositoryRoot = (Split-Path -Parent $PSScriptRoot)
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$status = 'FAIL'
$exitCode = 2
$failureMessage = $null
$resolvedResultPath = $null
$resultHash = $null
$temporaryResultPath = $null

try {
    $resolvedRepositoryRoot = [System.IO.Path]::GetFullPath($RepositoryRoot)
    $resolvedResultPath = [System.IO.Path]::GetFullPath($ResultPath)
    if (Test-Path -LiteralPath $resolvedResultPath) {
        throw "ResultPath must be new: $resolvedResultPath"
    }
    $resultDirectory = Split-Path -Parent $resolvedResultPath
    if (-not (Test-Path -LiteralPath $resultDirectory -PathType Container)) {
        [void][System.IO.Directory]::CreateDirectory($resultDirectory)
    }

    $modulePath = Join-Path $PSScriptRoot 'GovernanceValidationOrchestration.psm1'
    Import-Module -Name $modulePath -Force -ErrorAction Stop
    $request = Read-GovernanceJsonContract `
        -LiteralPath ([System.IO.Path]::GetFullPath($RequestPath)) `
        -SchemaPath (Join-Path $resolvedRepositoryRoot 'Governance/governance-validation-request.schema.json') `
        -ExpectedSchemaVersion 1
    $result = Invoke-GovernanceValidationOrchestration `
        -Request $request `
        -RepositoryRoot $resolvedRepositoryRoot

    $temporaryResultPath = "$resolvedResultPath.$PID.$([guid]::NewGuid().ToString('N')).tmp"
    [System.IO.File]::WriteAllText(
        $temporaryResultPath,
        (($result | ConvertTo-Json -Depth 100) + "`n"),
        [System.Text.UTF8Encoding]::new($false)
    )
    $validatedResult = Read-GovernanceTypedResult `
        -LiteralPath $temporaryResultPath `
        -SchemaPath (Join-Path $resolvedRepositoryRoot 'Governance/governance-validation-result.schema.json') `
        -ExpectedProfile ([string]$request.profile)
    [System.IO.File]::Move($temporaryResultPath, $resolvedResultPath)
    $resultHash = Get-GovernanceLowerSha256 -LiteralPath $resolvedResultPath
    $status = [string]$validatedResult.status
    $exitCode = switch ($status) {
        'PASS' { 0 }
        'BLOCKED' { 2 }
        'CANCELLED' { 2 }
        default { 1 }
    }
}
catch {
    $failureMessage = $_.Exception.Message
}
finally {
    if ($null -ne $temporaryResultPath -and (Test-Path -LiteralPath $temporaryResultPath -PathType Leaf)) {
        Remove-Item -LiteralPath $temporaryResultPath -Force
    }
    [pscustomobject]@{
        Status = $status
        ResultPath = $resolvedResultPath
        ResultSHA256 = $resultHash
        FailureMessage = $failureMessage
        NextAction = if ($status -ceq 'PASS') {
            'Reuse the hash-bound typed result at the next applicable validation stage.'
        }
        elseif ($status -ceq 'BLOCKED') {
            'Provide the missing typed subordinate result without rerunning unchanged PASS evidence.'
        }
        else {
            'Correct the failed cheap gate or typed-result contract and rerun the selected profile.'
        }
    } | Format-List
}

exit $exitCode
