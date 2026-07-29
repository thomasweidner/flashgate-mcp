#requires -Version 7.6
[CmdletBinding()]
param(
    [Parameter(Mandatory)][string]$OutputPath,
    [Parameter(Mandatory)][string]$StatusSourcePath
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$status = 'FAIL'
$resolvedOutputPath = $null
$outputHash = $null
$failureMessage = $null

try {
    $strictUtf8 = [System.Text.UTF8Encoding]::new($false, $true)
    $utf8 = [System.Text.UTF8Encoding]::new($false)
    $source = [System.IO.File]::ReadAllText(
        [System.IO.Path]::GetFullPath($StatusSourcePath),
        $strictUtf8
    ) | ConvertFrom-Json -Depth 100 -DateKind String

    $requiredProperties = @(
        'schemaVersion',
        'taskId',
        'correctionMode',
        'status',
        'classicReviewReady',
        'targetFindings',
        'pendingFindings',
        'closedFindings',
        'run007Status',
        'commitPreparationApproved',
        'commitAuthorized',
        'requiredReviewMode',
        'targetFindingCount',
        'correctedFindingCount',
        'pendingDeltaFindingCount',
        'closedFindingCount',
        'openFindingCount',
        'nextAction'
    )
    $missing = @($requiredProperties | Where-Object {
            $_ -notin @($source.PSObject.Properties.Name)
        })
    if ($missing.Count -gt 0) {
        throw "Status source is missing required properties: $($missing -join ', ')"
    }

    $contract = [ordered]@{}
    foreach ($property in $requiredProperties) {
        $contract[$property] = $source.$property
    }
    $contractJson = $contract | ConvertTo-Json -Depth 20
    $handoff = @"
# BL-333/BL-334 fourth bundled correction handoff

<!-- BEGIN GOVERNANCE-HANDOFF-STATUS -->
Status: $($contract.status)
CorrectionMode: $($contract.correctionMode)
TargetFindingCount: $($contract.targetFindingCount)
CorrectedFindingCount: $($contract.correctedFindingCount)
PendingDeltaFindingCount: $($contract.pendingDeltaFindingCount)
ClosedFindingCount: $($contract.closedFindingCount)
OpenFindingCount: $($contract.openFindingCount)
ClassicReviewReady: $(([string]$contract.classicReviewReady).ToLowerInvariant())
TargetFindings: $(@($contract.targetFindings) -join ',')
PendingFindings: $(@($contract.pendingFindings) -join ',')
ClosedFindings: $(@($contract.closedFindings) -join ',')
Run007Status: $($contract.run007Status)
CommitPreparationApproved: $(([string]$contract.commitPreparationApproved).ToLowerInvariant())
CommitAuthorized: $(([string]$contract.commitAuthorized).ToLowerInvariant())
RequiredReviewMode: $($contract.requiredReviewMode)
NextAction: $($contract.nextAction)
<!-- END GOVERNANCE-HANDOFF-STATUS -->

<!-- BEGIN GOVERNANCE-HANDOFF-CONTRACT -->
$contractJson
<!-- END GOVERNANCE-HANDOFF-CONTRACT -->
"@
    $resolvedOutputPath = [System.IO.Path]::GetFullPath($OutputPath)
    $outputDirectory = Split-Path -Parent $resolvedOutputPath
    if (-not (Test-Path -LiteralPath $outputDirectory -PathType Container)) {
        [void][System.IO.Directory]::CreateDirectory($outputDirectory)
    }
    [System.IO.File]::WriteAllText($resolvedOutputPath, $handoff, $utf8)
    $outputHash = (
        Get-FileHash -LiteralPath $resolvedOutputPath -Algorithm SHA256
    ).Hash.ToLowerInvariant()
    $status = 'PASS'
}
catch {
    $failureMessage = $_.Exception.Message
}
finally {
    [pscustomobject]@{
        Status = $status
        HandoffPath = $resolvedOutputPath
        HandoffSHA256 = $outputHash
        FailureMessage = $failureMessage
        NextAction = if ($status -ceq 'PASS') {
            'Validate the generated HANDOFF contract with Test-GovernanceConsistency.ps1.'
        }
        else {
            'Correct the typed status source and regenerate HANDOFF.md.'
        }
    } | Format-List
}

if ($status -ceq 'PASS') {
    exit 0
}
exit 1
