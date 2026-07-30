#requires -Version 7.6
[CmdletBinding()]
param(
    [Parameter(Mandatory)][string]$OutputPath,
    [string]$RepositoryRoot = (Split-Path -Parent $PSScriptRoot),
    [ValidateSet('INDEPENDENT_REVIEW', 'BUNDLED_CORRECTION', 'FOCUSED_INDEPENDENT_DELTA_REVIEW', 'COMMIT_PREPARATION')]
    [string]$ExecutionMode = 'INDEPENDENT_REVIEW',
    [Parameter(Mandatory)]
    [ValidateSet('ASSIGNMENT_START', 'MATERIAL_SCOPE_CHANGE', 'PRE_COMMIT', 'SPRINT_CLOSE', 'RELEASE_CANDIDATE', 'STABLE_RELEASE')]
    [string]$Checkpoint,
    [AllowEmptyCollection()][string[]]$ChangedPath = @(),
    [Parameter(Mandatory)][string]$Repository,
    [Parameter(Mandatory)][ValidatePattern('^[0-9a-f]{40}$')][string]$BaselineCommit,
    [Parameter(Mandatory)][ValidatePattern('^[0-9a-f]{40}$')][string]$CurrentCommit,
    [Parameter(Mandatory)][ValidatePattern('^[0-9a-f]{40}$')][string]$WorkflowCommit,
    [Parameter(Mandatory)][ValidatePattern('^[1-9][0-9]*$')][string]$RunId,
    [Parameter(Mandatory)][ValidateRange(1, [int]::MaxValue)][int]$RunAttempt,
    [Parameter(Mandatory)][ValidateNotNullOrEmpty()][string]$Event,
    [Parameter(Mandatory)][ValidateNotNullOrEmpty()][string]$Ref,
    [Parameter(Mandatory)][ValidatePattern('^[0-9a-f]{40}$')][string]$HeadSha,
    [string]$TaskId = 'BL-334'
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Convert-GlobToRegex {
    param([Parameter(Mandatory)][string]$Pattern)

    $escaped = [regex]::Escape($Pattern)
    $escaped = $escaped.Replace('\*\*', '.*')
    $escaped = $escaped.Replace('\*', '[^/]*')
    return '^' + $escaped + '$'
}

function Test-CanonicalRepositoryPath {
    param([Parameter(Mandatory)][AllowEmptyString()][string]$Path)

    if ([string]::IsNullOrWhiteSpace($Path) -or
        $Path.Contains('\') -or
        $Path.StartsWith('/', [System.StringComparison]::Ordinal) -or
        $Path.Contains('//', [System.StringComparison]::Ordinal) -or
        $Path -match '^[A-Za-z]:' -or
        $Path -match '[\x00-\x1f\x7f]' -or
        -not [string]::Equals(
            $Path,
            $Path.Normalize([System.Text.NormalizationForm]::FormC),
            [System.StringComparison]::Ordinal
        )) {
        return $false
    }

    $segments = @($Path.Split('/'))
    return (
        $segments.Count -gt 0 -and
        @($segments | Where-Object { $_ -in @('', '.', '..') }).Count -eq 0 -and
        [string]::Equals(
            ($segments -join '/'),
            $Path,
            [System.StringComparison]::Ordinal
        )
    )
}

$status = 'FAIL'
$failureMessage = $null
$resolvedOutputPath = $null
$recordHash = $null
$changedPathCount = 0

try {
    $resolvedRepositoryRoot = [System.IO.Path]::GetFullPath($RepositoryRoot)
    $catalogPath = Join-Path $resolvedRepositoryRoot 'Governance/change-trigger-catalog.json'
    if (-not (Test-Path -LiteralPath $catalogPath -PathType Leaf)) {
        throw "Governance catalog does not exist: $catalogPath"
    }
    if (@($ChangedPath | Where-Object { -not (Test-CanonicalRepositoryPath -Path $_) }).Count -gt 0) {
        throw 'ChangedPath contains a non-canonical repository-relative path.'
    }

    $catalog = Get-Content -LiteralPath $catalogPath -Raw -Encoding UTF8 |
        ConvertFrom-Json -Depth 100 -DateKind String
    $observedTriggers = [System.Collections.Generic.List[string]]::new()
    foreach ($path in @($ChangedPath)) {
        foreach ($trigger in @($catalog.triggers)) {
            if ('DIFF' -notin @($trigger.sources)) {
                continue
            }
            if (@($trigger.pathPatterns | Where-Object {
                        $path -match (Convert-GlobToRegex -Pattern ([string]$_)
                        )
                    }).Count -gt 0 -and [string]$trigger.id -notin $observedTriggers) {
                [void]$observedTriggers.Add([string]$trigger.id)
            }
        }
    }
    $observed = @($observedTriggers | Sort-Object -Unique)
    if (@($ChangedPath).Count -gt 0 -and $observed.Count -eq 0) {
        throw 'A non-empty workflow diff contains no cataloged trigger.'
    }
    $triggeredDomains = @(
        $catalog.triggers |
            Where-Object { [string]$_.id -in $observed } |
            ForEach-Object { [string]$_.domain } |
            Sort-Object -Unique
    )
    $affectedGates = @(
        $catalog.triggers |
            Where-Object { [string]$_.id -in $observed } |
            ForEach-Object { @($_.continuousGates) } |
            Sort-Object -Unique
    )
    $branch = [string](& git -C $resolvedRepositoryRoot rev-parse --abbrev-ref HEAD)
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($branch)) {
        throw 'Unable to resolve the checked-out branch.'
    }

    $repositorySlug = $Repository -replace '^https://github\.com/', '' -replace '\.git$', ''
    $mutationAllowed = $ExecutionMode -ceq 'BUNDLED_CORRECTION'
    $independent = $ExecutionMode -in @('INDEPENDENT_REVIEW', 'FOCUSED_INDEPENDENT_DELTA_REVIEW')
    $record = [ordered]@{
        schemaVersion = 1
        recordedAt = [datetimeoffset]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ')
        taskId = $TaskId
        repository = $Repository
        baselineCommit = $BaselineCommit
        currentCommit = $CurrentCommit
        branch = $branch
        executionMode = $ExecutionMode
        checkpoint = $Checkpoint
        changeTriggerReviewResult = if ($observed.Count -eq 0) { 'NO_TRIGGER' } else { 'EXISTING_GATES_REQUIRED' }
        triggeredDomains = $triggeredDomains
        observedTriggers = $observed
        affectedContinuousGates = $affectedGates
        existingBacklogCoverage = @('BL-333', 'BL-334')
        duplicateSearch = [ordered]@{
            performed = $observed.Count -gt 0
            sources = if ($observed.Count -gt 0) {
                @('BACKLOG.md', 'Governance/change-trigger-catalog.json')
            }
            else {
                @()
            }
            result = if ($observed.Count -gt 0) { 'EXISTING_ITEM_REUSED' } else { 'NOT_REQUIRED' }
        }
        repeatedChecks = @()
        checksNotRequired = @('performance baseline')
        newBacklogItems = @()
        updatedBacklogOrRegisterEntries = @()
        deferredTriggerItems = @()
        decisionBoundaries = @()
        releaseImpact = 'Workflow governance validation only; release impact is determined by the checked diff.'
        documentationConsistencyResult = 'PENDING'
        review = [ordered]@{
            repositoryMutationAllowed = $mutationAllowed
            externalMutationAllowed = $mutationAllowed
            findingFixesPerformed = $false
            reviewerIndependencePreserved = $independent
            originalFindings = @()
            discoveredInRunFindings = @()
            correctedInRunFindings = @()
            deferredFindings = @()
            stopConditionsEncountered = @()
            selfReviewIterations = 0
            permanentRegressionEvidence = @()
            focusedValidationResult = 'PENDING'
            independentDeltaReviewRequired = $ExecutionMode -ceq 'BUNDLED_CORRECTION'
        }
        handoff = [ordered]@{
            required = $false
            package = $null
            artifacts = @()
            scopeInventoryResult = 'NOT_APPLICABLE'
            patchCompletenessResult = 'NOT_APPLICABLE'
            manifestResult = 'NOT_APPLICABLE'
            missingArtifacts = @()
            classicReviewReady = $false
            canonicalValidatorPath = $null
            canonicalValidatorSha256 = $null
            canonicalValidatorExitCode = $null
            canonicalValidatorResult = 'NOT_APPLICABLE'
        }
        hostedCI = [ordered]@{
            required = $true
            sourceVerified = $true
            sources = @(
                "$repositorySlug@$HeadSha"
                "github-actions-run:$RunId"
            )
            workflowCommit = $WorkflowCommit
            runId = $RunId
            runAttempt = $RunAttempt
            event = $Event
            ref = $Ref
            headSha = $HeadSha
        }
        commitPreparation = [ordered]@{
            allFindingsClosed = $false
            independentDeltaReviewComplete = $false
            scopeVerified = $false
            validationPassed = $false
            commitAuthorized = $false
        }
        focusedDelta = [ordered]@{
            previousReviewPackage = $null
            priorReviewBaselineSha256 = $null
            correctionStartCommit = $null
            correctionPatchArtifact = $null
            correctionPatchSha256 = $null
            currentDeltaArtifact = $null
            currentDeltaSha256 = $null
            correctionOnlyPaths = @()
            reviewedFindingIds = @()
            directInterfacePaths = @()
            regressionEvidenceIds = @()
            allowedDeltaPaths = @()
            referenceOnlyPaths = @()
        }
    }

    $resolvedOutputPath = [System.IO.Path]::GetFullPath($OutputPath)
    $outputDirectory = Split-Path -Parent $resolvedOutputPath
    if (-not (Test-Path -LiteralPath $outputDirectory -PathType Container)) {
        [void][System.IO.Directory]::CreateDirectory($outputDirectory)
    }
    [System.IO.File]::WriteAllText(
        $resolvedOutputPath,
        ($record | ConvertTo-Json -Depth 100),
        [System.Text.UTF8Encoding]::new($false)
    )
    $recordHash = (Get-FileHash -LiteralPath $resolvedOutputPath -Algorithm SHA256).Hash.ToLowerInvariant()
    $changedPathCount = @($ChangedPath).Count
    $status = 'PASS'
}
catch {
    $failureMessage = $_.Exception.Message
}
finally {
    [pscustomobject]@{
        Status = $status
        RecordPath = $resolvedOutputPath
        RecordSHA256 = $recordHash
        ChangedPathCount = $changedPathCount
        FailureMessage = $failureMessage
        NextAction = if ($status -eq 'PASS') {
            'Pass the ephemeral record and trusted workflow context to Test-GovernanceConsistency.ps1.'
        }
        else {
            'Correct the workflow context or catalog coverage and rerun record generation.'
        }
    } | Format-List
}

if ($status -eq 'PASS') {
    exit 0
}
exit 1
