#requires -Version 7.6
[CmdletBinding(DefaultParameterSetName = 'LegacyDocument')]
param(
    [Parameter(Mandatory, ParameterSetName = 'LegacyDocument')][string]$OutputPath,
    [Parameter(Mandatory, ParameterSetName = 'LegacyDocument')][string]$StatusSourcePath,

    [Parameter(Mandatory, ParameterSetName = 'GenericPackage')]
    [ValidateSet('GENERIC_COMMIT_PREPARATION', 'IMPLEMENTATION_TO_INDEPENDENT_FULL_REVIEW', 'FINDING_CORRECTION')][string]$Profile,
    [Parameter(Mandatory, ParameterSetName = 'GenericPackage')]
    [ValidateSet('COMMIT_PREPARATION_TO_COMMIT_APPROVAL', 'IMPLEMENTATION_TO_INDEPENDENT_FULL_REVIEW', 'BUNDLED_CORRECTION_TO_FOCUSED_DELTA_REVIEW')][string]$TransitionType,
    [Parameter(Mandatory, ParameterSetName = 'GenericPackage')]
    [ValidatePattern('^BL-[0-9]{3}$')][string]$TaskId,
    [Parameter(Mandatory, ParameterSetName = 'GenericPackage')][string]$SourceDirectory,
    [Parameter(Mandatory, ParameterSetName = 'GenericPackage')][string[]]$AllowedDeltaPath,
    [Parameter(Mandatory, ParameterSetName = 'GenericPackage')][string]$PackagePath,
    [Parameter(ParameterSetName = 'GenericPackage')][string]$AuthoritativeRepositoryRoot,
    [Parameter(ParameterSetName = 'GenericPackage')][string]$OrchestrationResultPath,
    [Parameter(ParameterSetName = 'GenericPackage')][switch]$PreflightOnly,
    [Parameter(ParameterSetName = 'GenericPackage')][switch]$FinalPackageContentOnly,
    [Parameter(ParameterSetName = 'GenericPackage')][string]$StagingDirectory
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$status = 'FAIL'
$resolvedOutputPath = $null
$outputHash = $null
$failureMessage = $null
$stagingRoot = $null
$directoryValidationStatus = 'NOT_RUN'
$reopenValidationStatus = 'NOT_RUN'
$packageWriteAttemptCount = 0
$validationExecutionCount = 0
$readyToExecute = $false

function Read-StrictUtf8Json {
    param(
        [Parameter(Mandatory)][string]$LiteralPath,
        [string]$SchemaPath,
        [int]$ExpectedSchemaVersion = 0
    )
    $arguments = @{ LiteralPath = $LiteralPath }
    if (-not [string]::IsNullOrWhiteSpace($SchemaPath)) { $arguments.SchemaPath = $SchemaPath }
    if ($ExpectedSchemaVersion -gt 0) { $arguments.ExpectedSchemaVersion = $ExpectedSchemaVersion }
    return Read-GovernanceJsonContract @arguments
}

function Get-LowerSha256 {
    param([Parameter(Mandatory)][string]$LiteralPath)
    return (Get-FileHash -LiteralPath $LiteralPath -Algorithm SHA256).Hash.ToLowerInvariant()
}

function Assert-JsonSchema {
    param(
        [Parameter(Mandatory)][string]$LiteralPath,
        [Parameter(Mandatory)][string]$SchemaPath
    )

    $text = [System.IO.File]::ReadAllText($LiteralPath, [System.Text.UTF8Encoding]::new($false, $true))
    if (-not ($text | Test-Json -SchemaFile $SchemaPath -ErrorAction Stop)) {
        throw "Schema validation failed: $([System.IO.Path]::GetFileName($LiteralPath))"
    }
}

function Read-FindingCorrectionReportContract {
    param(
        [Parameter(Mandatory)][string]$Text,
        [Parameter(Mandatory)][string]$SchemaPath
    )

    $beginMarker = '<!-- BEGIN FINDING-CORRECTION-REPORT-CONTRACT -->'
    $endMarker = '<!-- END FINDING-CORRECTION-REPORT-CONTRACT -->'
    if ([regex]::Matches($Text, [regex]::Escape($beginMarker)).Count -ne 1 -or
        [regex]::Matches($Text, [regex]::Escape($endMarker)).Count -ne 1) {
        throw 'Finding-correction report must contain exactly one embedded report contract.'
    }
    $pattern = '(?s)' + [regex]::Escape($beginMarker) + '\r?\n(?<json>.*?)\r?\n' + [regex]::Escape($endMarker)
    $match = [regex]::Match($Text, $pattern)
    if (-not $match.Success) {
        throw 'Finding-correction report contract markers are not canonical.'
    }
    $jsonBytes = [System.Text.UTF8Encoding]::new($false, $true).GetBytes($match.Groups['json'].Value)
    return Read-GovernanceJsonContract -Bytes $jsonBytes -Label 'report.md embedded contract' `
        -SchemaPath $SchemaPath -ExpectedSchemaVersion 1
}

function Set-SingleReportStatusLine {
    param(
        [Parameter(Mandatory)][string]$Text,
        [Parameter(Mandatory)][string]$Label,
        [Parameter(Mandatory)][string]$Value
    )

    $pattern = '(?m)^' + [regex]::Escape($Label) + ':.*\r?$'
    if ([regex]::Matches($Text, $pattern).Count -ne 1) {
        throw "Finding-correction report must contain exactly one '$Label' status line."
    }
    return [regex]::Replace(
        $Text,
        $pattern,
        [System.Text.RegularExpressions.MatchEvaluator]{ param($match) "$Label`: $Value" },
        1
    )
}

function Set-FindingCorrectionReportLifecycle {
    param(
        [Parameter(Mandatory)][string]$Text,
        [Parameter(Mandatory)][object]$Contract
    )

    $beginMarker = '<!-- BEGIN FINDING-CORRECTION-REPORT-CONTRACT -->'
    $endMarker = '<!-- END FINDING-CORRECTION-REPORT-CONTRACT -->'
    $pattern = '(?s)' + [regex]::Escape($beginMarker) + '\r?\n(?<json>.*?)\r?\n' + [regex]::Escape($endMarker)
    $replacement = $beginMarker + "`n" + ($Contract | ConvertTo-Json -Depth 100) + "`n" + $endMarker
    $updated = [regex]::Replace(
        $Text,
        $pattern,
        [System.Text.RegularExpressions.MatchEvaluator]{ param($match) $replacement },
        1
    )
    foreach ($field in @(
            [ordered]@{ Label = 'ArtifactLifecycleState'; Value = [string]$Contract.artifactLifecycleState },
            [ordered]@{ Label = 'Status'; Value = [string]$Contract.status },
            [ordered]@{ Label = 'ReadyToExecute'; Value = ([string][bool]$Contract.readyToExecute).ToLowerInvariant() },
            [ordered]@{ Label = 'ClassicReviewReady'; Value = ([string][bool]$Contract.classicReviewReady).ToLowerInvariant() },
            [ordered]@{ Label = 'PackageWriteAttemptCount'; Value = [string][int]$Contract.packageWriteAttemptCount },
            [ordered]@{ Label = 'NextAction'; Value = [string]$Contract.nextAction }
        )) {
        $updated = Set-SingleReportStatusLine -Text $updated -Label $field.Label -Value $field.Value
    }
    return $updated
}

try {
    $utf8 = [System.Text.UTF8Encoding]::new($false)
    Import-Module (Join-Path $PSScriptRoot 'GovernanceValidationOrchestration.psm1') -Force
    Import-Module (Join-Path $PSScriptRoot 'GovernanceHandoffPublication.psm1') -Force

    if ($PSCmdlet.ParameterSetName -ceq 'LegacyDocument') {
        $source = Read-GovernanceJsonContract -LiteralPath ([System.IO.Path]::GetFullPath($StatusSourcePath))
        $requiredProperties = @(
            'schemaVersion', 'taskId', 'correctionMode', 'status',
            'classicReviewReady', 'targetFindings', 'pendingFindings',
            'closedFindings', 'run007Status', 'commitPreparationApproved',
            'commitAuthorized', 'requiredReviewMode', 'targetFindingCount',
            'correctedFindingCount', 'pendingDeltaFindingCount',
            'closedFindingCount', 'openFindingCount', 'nextAction'
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
    }
    else {
        $resolvedSourceDirectory = [System.IO.Path]::GetFullPath($SourceDirectory)
        $resolvedOutputPath = [System.IO.Path]::GetFullPath($PackagePath)
        $repositoryRoot = [System.IO.Path]::GetFullPath((Split-Path -Parent $PSScriptRoot))
        $resolvedAuthoritativeRepositoryRoot = if ([string]::IsNullOrWhiteSpace($AuthoritativeRepositoryRoot)) {
            $repositoryRoot
        }
        else {
            [System.IO.Path]::GetFullPath($AuthoritativeRepositoryRoot)
        }
        if (-not (Test-Path -LiteralPath $resolvedSourceDirectory -PathType Container)) {
            throw "Source directory does not exist: $resolvedSourceDirectory"
        }
        $isImplementationReview = $Profile -ceq 'IMPLEMENTATION_TO_INDEPENDENT_FULL_REVIEW'
        $isFindingCorrection = $Profile -ceq 'FINDING_CORRECTION'
        $expectedTransition = switch ($Profile) {
            'IMPLEMENTATION_TO_INDEPENDENT_FULL_REVIEW' { 'IMPLEMENTATION_TO_INDEPENDENT_FULL_REVIEW' }
            'GENERIC_COMMIT_PREPARATION' { 'COMMIT_PREPARATION_TO_COMMIT_APPROVAL' }
            'FINDING_CORRECTION' { 'BUNDLED_CORRECTION_TO_FOCUSED_DELTA_REVIEW' }
            default { throw 'Unknown explicit handoff profile.' }
        }
        if ($TransitionType -cne $expectedTransition) {
            throw 'Profile and transition type are not a supported explicit pair.'
        }
        if ($isFindingCorrection) {
            if ($PreflightOnly -and $FinalPackageContentOnly) {
                throw 'PreflightOnly and FinalPackageContentOnly are mutually exclusive.'
            }
            $directoryOnly = [bool]$PreflightOnly -or [bool]$FinalPackageContentOnly
            $contentLifecycleState = if ($PreflightOnly) {
                'ZIP_FREE_READY_TO_EXECUTE'
            }
            else { 'FINAL_REVIEW_PACKAGE' }
            $requiredSourceNames = @(
                'assignment-record.json', 'completion-report.json',
                'correction-only.patch', 'correction-scope-inventory.json',
                'current-delta.patch', 'external-governance-manifest.json',
                'finding-correction-matrix.json', 'finding-ledger.json',
                'finding-regression-matrix.json', 'focused-delta-review-record.json',
                'previous-review-binding.json', 'readiness-evidence.json',
                'report.md', 'scope-inventory.json', 'trusted-expected-hashes.json',
                'validation-summary.json'
            )
            $actualSourceNames = @(
                Get-ChildItem -LiteralPath $resolvedSourceDirectory -File |
                    ForEach-Object Name | Sort-Object
            )
            $hasPublicationEvidence = $actualSourceNames -ccontains 'publication-regression-evidence.json'
            $hasPublicationResult = $actualSourceNames -ccontains 'publication-regression-result.json'
            if ($hasPublicationEvidence -ne $hasPublicationResult) {
                throw 'Publication regression evidence and result artifacts must be present together.'
            }
            if ($hasPublicationEvidence) {
                $requiredSourceNames += @('publication-regression-evidence.json', 'publication-regression-result.json')
            }
            if (($actualSourceNames -join "`n") -cne (($requiredSourceNames | Sort-Object) -join "`n")) {
                throw 'Finding-correction source directory must contain the canonical source artifacts and only the optional publication-regression evidence artifact.'
            }

            $governanceRoot = Join-Path (Split-Path -Parent $PSScriptRoot) 'Governance'
            $schemaMap = [ordered]@{
                'assignment-record.json' = 'finding-correction-assignment.schema.json'
                'completion-report.json' = 'finding-correction-completion.schema.json'
                'finding-correction-matrix.json' = 'finding-correction-matrix.schema.json'
                'finding-ledger.json' = 'finding-ledger.schema.json'
                'finding-regression-matrix.json' = 'finding-regression-matrix.schema.json'
                'focused-delta-review-record.json' = 'focused-delta-review-record.schema.json'
                'previous-review-binding.json' = 'previous-review-binding.schema.json'
            }
            if ($hasPublicationEvidence) {
                $schemaMap['publication-regression-evidence.json'] = 'publication-regression-evidence.schema.json'
                $schemaMap['publication-regression-result.json'] = 'publication-regression-result.schema.json'
            }
            $versionMap = [ordered]@{
                'assignment-record.json' = 2
                'completion-report.json' = 2
                'correction-scope-inventory.json' = 1
                'external-governance-manifest.json' = 1
                'finding-correction-matrix.json' = 2
                'finding-ledger.json' = 1
                'finding-regression-matrix.json' = 2
                'focused-delta-review-record.json' = 3
                'previous-review-binding.json' = 3
                'readiness-evidence.json' = 2
                'scope-inventory.json' = 1
                'trusted-expected-hashes.json' = 1
                'validation-summary.json' = 1
            }
            if ($hasPublicationEvidence) {
                $versionMap['publication-regression-evidence.json'] = 2
                $versionMap['publication-regression-result.json'] = 2
            }
            $sourceContracts = @{}
            foreach ($name in $versionMap.Keys) {
                $schemaPath = if ($schemaMap.Contains($name)) { Join-Path $governanceRoot $schemaMap[$name] } else { '' }
                $sourceContracts[$name] = Read-StrictUtf8Json `
                    -LiteralPath (Join-Path $resolvedSourceDirectory $name) `
                    -SchemaPath $schemaPath -ExpectedSchemaVersion $versionMap[$name]
            }
            $assignment = $sourceContracts['assignment-record.json']
            $completion = $sourceContracts['completion-report.json']
            $sourceReadiness = $sourceContracts['readiness-evidence.json']
            $sourceReportText = [System.IO.File]::ReadAllText(
                (Join-Path $resolvedSourceDirectory 'report.md'),
                [System.Text.UTF8Encoding]::new($false, $true)
            )
            $sourceReportContract = Read-FindingCorrectionReportContract -Text $sourceReportText `
                -SchemaPath (Join-Path $governanceRoot 'finding-correction-report-contract.schema.json')
            if ([string]$assignment.taskId -cne $TaskId -or [string]$completion.taskId -cne $TaskId -or
                [string]$assignment.profile -cne $Profile -or [string]$completion.profile -cne $Profile -or
                [string]$assignment.transitionType -cne $TransitionType -or
                [string]$completion.transitionType -cne $TransitionType -or
                [string]$sourceReportContract.taskId -cne $TaskId -or
                [string]$sourceReportContract.profile -cne $Profile -or
                [string]$sourceReportContract.transitionType -cne $TransitionType) {
                throw 'Finding-correction task/profile/transition discriminator mismatch.'
            }
            if ([int]$assignment.schemaVersion -ne 2 -or [int]$completion.schemaVersion -ne 2 -or
                [int]$sourceReadiness.schemaVersion -ne 2 -or
                [string]$assignment.artifactLifecycleState -cne 'ZIP_FREE_READY_TO_EXECUTE' -or
                [string]$completion.artifactLifecycleState -cne 'ZIP_FREE_READY_TO_EXECUTE' -or
                [string]$sourceReadiness.artifactLifecycleState -cne 'ZIP_FREE_READY_TO_EXECUTE' -or
                -not [bool]$assignment.readyToExecute -or -not [bool]$completion.readyToExecute -or
                -not [bool]$sourceReadiness.readyToExecute -or [bool]$assignment.classicReviewReady -or
                [bool]$completion.classicReviewReady -or [bool]$sourceReadiness.classicReviewReady -or
                [int]$completion.packageWriteAttemptCount -ne 0 -or
                [int]$sourceReadiness.packageWriteAttemptCount -ne 0 -or
                [string]$assignment.nextAction -cne 'READY_FOR_SINGLE_DELTA_REVIEW_PACKAGE_WRITE_AUTHORIZATION' -or
                [string]$completion.nextAction -cne 'READY_FOR_SINGLE_DELTA_REVIEW_PACKAGE_WRITE_AUTHORIZATION' -or
                [string]$sourceReportContract.artifactLifecycleState -cne 'ZIP_FREE_READY_TO_EXECUTE' -or
                -not [bool]$sourceReportContract.readyToExecute -or [bool]$sourceReportContract.classicReviewReady -or
                [int]$sourceReportContract.packageWriteAttemptCount -ne 0 -or
                [string]$sourceReportContract.nextAction -cne 'READY_FOR_SINGLE_DELTA_REVIEW_PACKAGE_WRITE_AUTHORIZATION') {
                throw 'Finding-correction source must be the canonical ZIP-free preflight lifecycle state.'
            }
            $scope = $sourceContracts['scope-inventory.json']
            $scopePaths = @(@(foreach ($entry in @($scope.entries)) {
                        if ([string]$entry.gitStatus -ceq 'TRACKED_RENAMED') {
                            [string]$entry.previousPath
                        }
                        [string]$entry.path
                    }) | Sort-Object -Unique)
            if ((($scopePaths | Sort-Object) -join "`n") -cne (($AllowedDeltaPath | Sort-Object) -join "`n")) {
                throw 'Allowed delta paths do not match the finding-correction scope inventory.'
            }

            if ($directoryOnly) {
                if ([string]::IsNullOrWhiteSpace($StagingDirectory)) {
                    throw 'A directory-only finding-correction run requires an explicit fresh StagingDirectory.'
                }
                $stagingRoot = [System.IO.Path]::GetFullPath($StagingDirectory)
                if (Test-Path -LiteralPath $stagingRoot) {
                    throw "Finding-correction staging directory must be new: $stagingRoot"
                }
            }
            else {
                $stagingRoot = Join-Path ([System.IO.Path]::GetTempPath()) ('flashgate-finding-correction-handoff-' + [guid]::NewGuid().ToString('N'))
            }
            [void][System.IO.Directory]::CreateDirectory($stagingRoot)
            foreach ($name in $requiredSourceNames) {
                [System.IO.File]::Copy((Join-Path $resolvedSourceDirectory $name), (Join-Path $stagingRoot $name), $false)
            }

            if ($contentLifecycleState -ceq 'FINAL_REVIEW_PACKAGE') {
                $assignment.schemaVersion = 2
                $assignment | Add-Member -NotePropertyName artifactLifecycleState -NotePropertyValue 'FINAL_REVIEW_PACKAGE' -Force
                $assignment | Add-Member -NotePropertyName readyToExecute -NotePropertyValue $false -Force
                $assignment.classicReviewReady = $true
                $assignment.nextAction = 'FOCUSED_INDEPENDENT_DELTA_REVIEW'
                [System.IO.File]::WriteAllText(
                    (Join-Path $stagingRoot 'assignment-record.json'),
                    (($assignment | ConvertTo-Json -Depth 100) + "`n"), $utf8
                )

                $completion.schemaVersion = 2
                $completion | Add-Member -NotePropertyName artifactLifecycleState -NotePropertyValue 'FINAL_REVIEW_PACKAGE' -Force
                $completion.status = 'FINAL_REVIEW_PACKAGE'
                $completion.readyToExecute = $false
                $completion.classicReviewReady = $true
                $completion.packageWriteAttemptCount = 1
                $completion.nextAction = 'FOCUSED_INDEPENDENT_DELTA_REVIEW'
                [System.IO.File]::WriteAllText(
                    (Join-Path $stagingRoot 'completion-report.json'),
                    (($completion | ConvertTo-Json -Depth 100) + "`n"), $utf8
                )

                $readiness = $sourceReadiness
                $readiness.schemaVersion = 2
                $readiness | Add-Member -NotePropertyName artifactLifecycleState -NotePropertyValue 'FINAL_REVIEW_PACKAGE' -Force
                $readiness.readyToExecute = $false
                $readiness.classicReviewReady = $true
                $readiness.packageWriteAttemptCount = 1
                $readiness | Add-Member -NotePropertyName nextAction -NotePropertyValue 'FOCUSED_INDEPENDENT_DELTA_REVIEW' -Force
                [System.IO.File]::WriteAllText(
                    (Join-Path $stagingRoot 'readiness-evidence.json'),
                    (($readiness | ConvertTo-Json -Depth 100) + "`n"), $utf8
                )

                $sourceReportContract.artifactLifecycleState = 'FINAL_REVIEW_PACKAGE'
                $sourceReportContract.status = 'FINAL_REVIEW_PACKAGE'
                $sourceReportContract.readyToExecute = $false
                $sourceReportContract.classicReviewReady = $true
                $sourceReportContract.packageWriteAttemptCount = 1
                $sourceReportContract.nextAction = 'FOCUSED_INDEPENDENT_DELTA_REVIEW'
                $finalReport = Set-FindingCorrectionReportLifecycle -Text $sourceReportText `
                    -Contract $sourceReportContract
                [System.IO.File]::WriteAllText(
                    (Join-Path $stagingRoot 'report.md'),
                    $finalReport,
                    $utf8
                )
            }

            $contract = [ordered]@{
                schemaVersion = 2
                taskId = $TaskId
                profile = $Profile
                transitionType = $TransitionType
                artifactLifecycleState = $contentLifecycleState
                status = [string]$completion.status
                readyToExecute = [bool]$completion.readyToExecute
                classicReviewReady = [bool]$completion.classicReviewReady
                findingIds = @($completion.findingIds)
                reviewStatus = [string]$completion.reviewStatus
                commitAuthorized = $false
                packageWriteAttemptCount = [int]$completion.packageWriteAttemptCount
                nextAction = [string]$completion.nextAction
            }
            $contractJson = $contract | ConvertTo-Json -Depth 20
            $handoff = @"
# $TaskId finding-correction handoff

<!-- BEGIN GOVERNANCE-HANDOFF-STATUS -->
TaskId: $TaskId
TransitionType: $TransitionType
Profile: $Profile
ArtifactLifecycleState: $($contract.artifactLifecycleState)
Status: $($contract.status)
ReadyToExecute: $(([string]$contract.readyToExecute).ToLowerInvariant())
ClassicReviewReady: $(([string]$contract.classicReviewReady).ToLowerInvariant())
FindingCount: $(@($contract.findingIds).Count)
ReviewStatus: $($contract.reviewStatus)
PackageWriteAttemptCount: $($contract.packageWriteAttemptCount)
CommitAuthorized: false
NextAction: $($contract.nextAction)
<!-- END GOVERNANCE-HANDOFF-STATUS -->

<!-- BEGIN GOVERNANCE-HANDOFF-CONTRACT -->
$contractJson
<!-- END GOVERNANCE-HANDOFF-CONTRACT -->
"@
            [System.IO.File]::WriteAllText((Join-Path $stagingRoot 'HANDOFF.md'), $handoff, $utf8)

            $inventoryEntries = @(
                Get-ChildItem -LiteralPath $stagingRoot -File | Sort-Object Name | ForEach-Object {
                    [ordered]@{ path = $_.Name; sha256 = Get-LowerSha256 $_.FullName; length = [int64]$_.Length }
                }
            )
            $inventory = [ordered]@{
                schemaVersion = 1; taskId = $TaskId; profile = $Profile
                transitionType = $TransitionType; entries = $inventoryEntries
            }
            [System.IO.File]::WriteAllText(
                (Join-Path $stagingRoot 'package-inventory.json'),
                ($inventory | ConvertTo-Json -Depth 20), $utf8
            )
            [string[]]$manifestNames = @(Get-ChildItem -LiteralPath $stagingRoot -File | ForEach-Object Name)
            [array]::Sort($manifestNames, [System.StringComparer]::Ordinal)
            $manifestLines = @($manifestNames | ForEach-Object {
                    $file = Get-Item -LiteralPath (Join-Path $stagingRoot $_)
                    "$(Get-LowerSha256 $file.FullName)  $($file.Length)  $($file.Name)"
                })
            [System.IO.File]::WriteAllText(
                (Join-Path $stagingRoot 'MANIFEST.sha256'),
                (($manifestLines -join "`n") + "`n"), $utf8
            )

            $validatorPath = Join-Path $PSScriptRoot 'Test-FindingCorrectionHandoff.ps1'
            $directoryValidationOutput = @(& $validatorPath -PackagePath $stagingRoot `
                    -RepositoryRoot $repositoryRoot `
                    -AuthoritativeRepositoryRoot $resolvedAuthoritativeRepositoryRoot `
                    -ReturnInsteadOfExit)
            $directoryValidationExitCode = $LASTEXITCODE
            $validationExecutionCount++
            if ($directoryValidationExitCode -ne 0) {
                throw "Finding-correction staging validation failed: $(($directoryValidationOutput | Out-String).Trim())"
            }
            $directoryValidationStatus = 'PASS'
            $readyToExecute = $true

            if (-not $directoryOnly) {
                $outputDirectory = Split-Path -Parent $resolvedOutputPath
                if (-not (Test-Path -LiteralPath $outputDirectory -PathType Container)) {
                    [void][System.IO.Directory]::CreateDirectory($outputDirectory)
                }
                $candidateValidator = {
                    param($candidatePath)
                    $candidateOutput = @(& $validatorPath -PackagePath $candidatePath `
                            -RepositoryRoot $repositoryRoot `
                            -AuthoritativeRepositoryRoot $resolvedAuthoritativeRepositoryRoot `
                            -ReturnInsteadOfExit)
                    $script:validationExecutionCount++
                    if ($LASTEXITCODE -ne 0) {
                        throw "Finding-correction candidate validation failed: $(($candidateOutput | Out-String).Trim())"
                    }
                }
                $publication = Publish-GovernanceHandoffPackage -StagingDirectory $stagingRoot `
                    -FinalPath $resolvedOutputPath -CandidateValidator $candidateValidator `
                    -PackageWriteAttemptCount ([ref]$packageWriteAttemptCount)
                $outputHash = $publication.Sha256
                $reopenValidationStatus = 'PASS'
            }
        }
        else {
        $evidenceName = if ($isImplementationReview) {
            'pre-review-validation-evidence.json'
        }
        else {
            'independent-review-evidence.json'
        }
        $evidenceSchemaName = if ($isImplementationReview) {
            'generic-pre-review-validation-evidence.schema.json'
        }
        else {
            'generic-independent-review-evidence.schema.json'
        }
        $requiredSourceNames = @(
            'assignment-record.json', 'completion-report.json',
            'current-delta.patch', $evidenceName,
            'report.md', 'scope-inventory.json', 'task.patch',
            'validation-summary.json'
        )
        $actualSourceNames = @(
            Get-ChildItem -LiteralPath $resolvedSourceDirectory -File |
                ForEach-Object Name | Sort-Object
        )
        if (($actualSourceNames -join "`n") -cne (($requiredSourceNames | Sort-Object) -join "`n")) {
            throw 'Generic source directory must contain exactly the eight canonical source artifacts.'
        }

        $governanceRoot = Join-Path (Split-Path -Parent $PSScriptRoot) 'Governance'
        $schemaMap = [ordered]@{
            'assignment-record.json' = 'generic-assignment-record.schema.json'
            'completion-report.json' = 'generic-completion-report.schema.json'
            $evidenceName = $evidenceSchemaName
            'scope-inventory.json' = 'generic-scope-inventory.schema.json'
            'validation-summary.json' = 'generic-validation-summary.schema.json'
        }
        foreach ($name in $schemaMap.Keys) {
            $sourcePath = Join-Path $resolvedSourceDirectory $name
            $typedSource = Read-GovernanceJsonContract `
                -LiteralPath $sourcePath `
                -SchemaPath (Join-Path $governanceRoot $schemaMap[$name]) `
                -ExpectedProfile $Profile
            if ([string]$typedSource.taskId -cne $TaskId -or
                [string]$typedSource.profile -cne $Profile) {
                throw "Task/profile mismatch in $name"
            }
        }

        if (-not [string]::IsNullOrWhiteSpace($OrchestrationResultPath)) {
            $orchestrationResult = Read-GovernanceTypedResult `
                -LiteralPath $OrchestrationResultPath `
                -SchemaPath (Join-Path $governanceRoot 'governance-validation-result.schema.json')
            if ([string]$orchestrationResult.taskId -cne $TaskId -or
                [string]$orchestrationResult.status -cne 'PASS' -or
                [int]$orchestrationResult.packageWriteAttemptCount -ne 0) {
                throw 'Orchestration result is not a PASS bound to this task before package creation.'
            }
        }

        $assignment = Read-StrictUtf8Json -LiteralPath (Join-Path $resolvedSourceDirectory 'assignment-record.json')
        $completion = Read-StrictUtf8Json -LiteralPath (Join-Path $resolvedSourceDirectory 'completion-report.json')
        $review = Read-StrictUtf8Json -LiteralPath (Join-Path $resolvedSourceDirectory $evidenceName)
        $scope = Read-StrictUtf8Json -LiteralPath (Join-Path $resolvedSourceDirectory 'scope-inventory.json')
        $scopeInventorySha256 = Get-LowerSha256 -LiteralPath (Join-Path $resolvedSourceDirectory 'scope-inventory.json')
        $taskPatchSha256 = Get-LowerSha256 -LiteralPath (Join-Path $resolvedSourceDirectory 'task.patch')
        $currentDeltaSha256 = Get-LowerSha256 -LiteralPath (Join-Path $resolvedSourceDirectory 'current-delta.patch')
        $fullCompletionEvidenceSha256 = if ($isImplementationReview) {
            Get-LowerSha256 -LiteralPath (Join-Path $resolvedSourceDirectory $evidenceName)
        }
        else { $null }
        if ([string]$assignment.transitionType -cne $TransitionType -or
            [string]$completion.transitionType -cne $TransitionType) {
            throw 'Transition discriminator mismatch.'
        }
        $scopePaths = @(
            $scope.entries |
                Where-Object { [string]$_.inclusionDecision -ceq 'INCLUDE' } |
                ForEach-Object {
                    if ([string]$_.gitStatus -ceq 'TRACKED_RENAMED') {
                        [string]$_.previousPath
                    }
                    [string]$_.path
                }
        )
        if ((($scopePaths | Sort-Object) -join "`n") -cne
            (($AllowedDeltaPath | Sort-Object) -join "`n")) {
            throw 'Allowed delta paths do not match the scope inventory.'
        }
        $bindingSources = @($assignment, $completion, $review)
        foreach ($binding in $bindingSources) {
            if ([string]$binding.repository -cne [string]$scope.repository -or
                [string]$binding.baselineCommit -cne [string]$scope.baselineCommit -or
                [string]$binding.currentCommit -cne [string]$scope.currentCommit -or
                [string]$binding.branch -cne [string]$scope.branch -or
                [string]$binding.scopeInventorySha256 -cne $scopeInventorySha256 -or
                ((@($binding.allowedDeltaPaths | Sort-Object) -join "`n") -cne (@($scope.allowedDeltaPaths | Sort-Object) -join "`n")) -or
                ((@($binding.excludedDeltaPaths | Sort-Object) -join "`n") -cne (@($scope.excludedDeltaPaths | Sort-Object) -join "`n"))) {
                throw 'Repository, current-state, scope-hash, or delta-path binding mismatch.'
            }
        }
        foreach ($binding in @($assignment, $completion)) {
            if ([string]$binding.taskPatchSha256 -cne $taskPatchSha256 -or
                [string]$binding.currentDeltaSha256 -cne $currentDeltaSha256) {
                throw 'Patch hash binding mismatch.'
            }
        }
        if ($isImplementationReview) {
            foreach ($binding in @($assignment, $completion)) {
                if ([string]$binding.fullCompletionEvidenceSha256 -cne $fullCompletionEvidenceSha256 -or
                    [string]$binding.fullCompletionResultSha256 -cne [string]$review.fullCompletionResultSha256 -or
                    [string]$binding.executionEnvelopeSha256 -cne [string]$review.executionEnvelopeSha256) {
                    throw 'Full-completion evidence hash binding mismatch.'
                }
            }
            if ([string]$review.independentReviewStatus -cne 'NOT_PERFORMED' -or
                -not [bool]$review.fullCompletionEvidenceReused -or
                [bool]$review.fullCompletionReexecuted -or
                [bool]$review.externalArtifactRequired) {
                throw 'Pre-review evidence semantics are invalid.'
            }
        }
        else {
            foreach ($reviewed in @($review.reviewedArtifacts)) {
                $expectedHash = if ([string]$reviewed.path -ceq 'task.patch') {
                    $taskPatchSha256
                }
                else {
                    $currentDeltaSha256
                }
                if ([string]$reviewed.sha256 -cne $expectedHash) {
                    throw 'Independent-review patch hash binding mismatch.'
                }
            }
        }
        $assignmentFindingJson = @($assignment.findingIds) | ConvertTo-Json -Compress
        $completionFindingJson = @($completion.findingIds) | ConvertTo-Json -Compress
        $reviewFindingJson = @($review.findingIds) | ConvertTo-Json -Compress
        if ($assignmentFindingJson -cne $completionFindingJson -or
            $assignmentFindingJson -cne $reviewFindingJson) {
            throw 'Finding IDs do not agree across generic evidence.'
        }

        $contract = [ordered]@{
            schemaVersion = 1
            taskId = $TaskId
            repository = [string]$assignment.repository
            baselineCommit = [string]$assignment.baselineCommit
            currentCommit = [string]$assignment.currentCommit
            branch = [string]$assignment.branch
            transitionType = $TransitionType
            profile = $Profile
            status = [string]$completion.status
            classicReviewReady = [bool]$completion.classicReviewReady
            findingIds = @($completion.findingIds)
            reviewStatus = if ($isImplementationReview) { [string]$review.independentReviewStatus } else { [string]$review.result }
            commitAuthorized = $false
            scopeInventorySha256 = $scopeInventorySha256
            taskPatchSha256 = $taskPatchSha256
            currentDeltaSha256 = $currentDeltaSha256
            allowedDeltaPaths = @($AllowedDeltaPath)
            excludedDeltaPaths = @($scope.excludedDeltaPaths)
            nextAction = [string]$completion.nextAction
        }
        if ($isImplementationReview) {
            $contract.fullCompletionEvidenceSha256 = $fullCompletionEvidenceSha256
            $contract.fullCompletionResultSha256 = [string]$review.fullCompletionResultSha256
            $contract.executionEnvelopeSha256 = [string]$review.executionEnvelopeSha256
        }
        $contractJson = $contract | ConvertTo-Json -Depth 20
        if (-not ($contractJson | Test-Json -SchemaFile (Join-Path $governanceRoot 'generic-handoff-contract.schema.json'))) {
            throw 'Generated generic handoff contract failed its schema.'
        }

        if ($PreflightOnly) {
            if ([string]::IsNullOrWhiteSpace($StagingDirectory)) {
                throw 'PreflightOnly requires an explicit fresh StagingDirectory.'
            }
            $stagingRoot = [System.IO.Path]::GetFullPath($StagingDirectory)
            if (Test-Path -LiteralPath $stagingRoot) {
                throw "Preflight staging directory must be new: $stagingRoot"
            }
        }
        else {
            $stagingRoot = Join-Path ([System.IO.Path]::GetTempPath()) ('flashgate-generic-handoff-' + [guid]::NewGuid().ToString('N'))
        }
        [void][System.IO.Directory]::CreateDirectory($stagingRoot)
        foreach ($name in $requiredSourceNames) {
            [System.IO.File]::Copy((Join-Path $resolvedSourceDirectory $name), (Join-Path $stagingRoot $name), $false)
        }

        $handoff = @"
# $TaskId governance handoff

<!-- BEGIN GOVERNANCE-HANDOFF-STATUS -->
TaskId: $TaskId
TransitionType: $TransitionType
Profile: $Profile
Status: $($contract.status)
ClassicReviewReady: $(([string]$contract.classicReviewReady).ToLowerInvariant())
FindingCount: $(@($contract.findingIds).Count)
ReviewStatus: $($contract.reviewStatus)
CommitAuthorized: false
AllowedDeltaPaths: $(@($contract.allowedDeltaPaths) -join ',')
NextAction: $($completion.nextAction)
<!-- END GOVERNANCE-HANDOFF-STATUS -->

<!-- BEGIN GOVERNANCE-HANDOFF-CONTRACT -->
$contractJson
<!-- END GOVERNANCE-HANDOFF-CONTRACT -->
"@
        [System.IO.File]::WriteAllText((Join-Path $stagingRoot 'HANDOFF.md'), $handoff, $utf8)

        $inventoryEntries = @(
            Get-ChildItem -LiteralPath $stagingRoot -File |
                Sort-Object Name |
                ForEach-Object {
                    [ordered]@{
                        path = $_.Name
                        sha256 = Get-LowerSha256 -LiteralPath $_.FullName
                        length = [int64]$_.Length
                    }
                }
        )
        $inventory = [ordered]@{
            schemaVersion = 1
            taskId = $TaskId
            profile = $Profile
            transitionType = $TransitionType
            entries = $inventoryEntries
        }
        $inventoryPath = Join-Path $stagingRoot 'package-inventory.json'
        [System.IO.File]::WriteAllText($inventoryPath, ($inventory | ConvertTo-Json -Depth 20), $utf8)
        Assert-JsonSchema -LiteralPath $inventoryPath -SchemaPath (Join-Path $governanceRoot 'generic-package-inventory.schema.json')

        [string[]]$manifestNames = @(
            Get-ChildItem -LiteralPath $stagingRoot -File | ForEach-Object Name
        )
        [array]::Sort($manifestNames, [System.StringComparer]::Ordinal)
        $manifestLines = @(
            $manifestNames | ForEach-Object {
                $manifestFile = Get-Item -LiteralPath (Join-Path $stagingRoot $_)
                "$(Get-LowerSha256 -LiteralPath $manifestFile.FullName)  $($manifestFile.Length)  $($manifestFile.Name)"
            }
        )
        [System.IO.File]::WriteAllText(
            (Join-Path $stagingRoot 'MANIFEST.sha256'),
            (($manifestLines -join "`n") + "`n"),
            $utf8
        )

        $genericValidatorPath = Join-Path $PSScriptRoot 'Test-GenericGovernanceHandoff.ps1'
        $directoryValidationOutput = @(
            & $genericValidatorPath `
                -PackagePath $stagingRoot `
                -RepositoryRoot $repositoryRoot `
                -AuthoritativeRepositoryRoot $resolvedAuthoritativeRepositoryRoot `
                -ReturnInsteadOfExit
        )
        $directoryValidationExitCode = $LASTEXITCODE
        $validationExecutionCount++
        if ($directoryValidationExitCode -ne 0) {
            throw "Staging directory validation failed before ZIP write: $(($directoryValidationOutput | Out-String).Trim())"
        }
        $directoryValidationStatus = 'PASS'

        if (-not $PreflightOnly) {
        $outputDirectory = Split-Path -Parent $resolvedOutputPath
        if (-not (Test-Path -LiteralPath $outputDirectory -PathType Container)) {
            [void][System.IO.Directory]::CreateDirectory($outputDirectory)
        }
        $candidateValidator = {
            param($candidatePath)
            $candidateOutput = @(& $genericValidatorPath -PackagePath $candidatePath `
                    -RepositoryRoot $repositoryRoot `
                    -AuthoritativeRepositoryRoot $resolvedAuthoritativeRepositoryRoot `
                    -ReturnInsteadOfExit)
            $script:validationExecutionCount++
            if ($LASTEXITCODE -ne 0) {
                throw "Generic governance candidate validation failed: $(($candidateOutput | Out-String).Trim())"
            }
        }
        $publication = Publish-GovernanceHandoffPackage -StagingDirectory $stagingRoot `
            -FinalPath $resolvedOutputPath -CandidateValidator $candidateValidator `
            -PackageWriteAttemptCount ([ref]$packageWriteAttemptCount)
        $outputHash = $publication.Sha256
        $reopenValidationStatus = 'PASS'
        }
        else {
            $readyToExecute = $true
        }
        }
    }

    if (-not $PreflightOnly -and -not $FinalPackageContentOnly) {
        $outputHash = Get-LowerSha256 -LiteralPath $resolvedOutputPath
    }
    $status = 'PASS'
}
catch {
    $failureMessage = $_.Exception.Message
}
finally {
    if ($null -ne $stagingRoot -and (Test-Path -LiteralPath $stagingRoot -PathType Container) -and
        (-not ($PreflightOnly -or $FinalPackageContentOnly) -or $status -cne 'PASS')) {
        Remove-Item -LiteralPath $stagingRoot -Recurse -Force
    }
    [pscustomobject]@{
        Status = $status
        HandoffPath = $resolvedOutputPath
        HandoffSHA256 = $outputHash
        StagingPath = if (($PreflightOnly -or $FinalPackageContentOnly) -and $status -ceq 'PASS') { $stagingRoot } else { $null }
        PreflightOnly = [bool]$PreflightOnly
        FinalPackageContentOnly = [bool]$FinalPackageContentOnly
        ReadyToExecute = $status -ceq 'PASS' -and ($readyToExecute -or -not $PreflightOnly)
        DirectoryValidationStatus = $directoryValidationStatus
        PackageWriteAttemptCount = $packageWriteAttemptCount
        ReopenValidationStatus = $reopenValidationStatus
        ValidationExecutionCount = $validationExecutionCount
        InfrastructureOrInvocationFailureCount = 0
        FullMatrixRunCount = 0
        GeneratedTaskControllerFileCount = 0
        GeneratedTaskControllerLineCount = 0
        ReadOnlyProbeCount = 0
        FailureMessage = $failureMessage
        NextAction = if ($status -ceq 'PASS' -and ($PreflightOnly -or $FinalPackageContentOnly)) {
            'READY_FOR_SINGLE_PACKAGE_WRITE_AUTHORIZATION'
        }
        elseif ($status -ceq 'PASS') {
            'Validate the generated handoff with Test-GovernanceConsistency.ps1.'
        }
        else {
            'Correct the typed source artifacts and generate one new package path.'
        }
    } | Format-List
}

if ($status -ceq 'PASS') { exit 0 }
exit 1
