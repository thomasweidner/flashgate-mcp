#Requires -Version 7.6

[CmdletBinding()]
param(
    [Parameter()]
    [string]$RepositoryRoot = (Split-Path -Parent $PSScriptRoot)
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$resolvedRoot = (Resolve-Path -LiteralPath $RepositoryRoot -ErrorAction Stop).Path
if (-not (Test-Path -LiteralPath $resolvedRoot -PathType Container)) {
    throw 'RepositoryRoot must identify an existing directory.'
}

$checks = [System.Collections.Generic.List[object]]::new()
$documents = [System.Collections.Generic.Dictionary[string, string]]::new(
    [System.StringComparer]::Ordinal
)

function Add-Check {
    param(
        [Parameter(Mandatory)]
        [string]$Id,

        [Parameter(Mandatory)]
        [bool]$Passed,

        [Parameter(Mandatory)]
        [string]$Message
    )

    $checks.Add([pscustomobject]@{
            id      = $Id
            passed  = $Passed
            message = $Message
        })
}

function Read-StrictUtf8 {
    param(
        [Parameter(Mandatory)]
        [string]$RelativePath
    )

    $fullPath = Join-Path $resolvedRoot $RelativePath
    if (-not (Test-Path -LiteralPath $fullPath -PathType Leaf)) {
        Add-Check -Id ('FILE-{0}' -f $RelativePath) -Passed $false -Message 'Required file is missing.'
        return $null
    }

    try {
        $bytes = [System.IO.File]::ReadAllBytes($fullPath)
        $encoding = [System.Text.UTF8Encoding]::new($false, $true)
        $text = $encoding.GetString($bytes)
        Add-Check -Id ('FILE-{0}' -f $RelativePath) -Passed (-not $text.Contains([char]0)) -Message 'Strict UTF-8 and NUL-free.'
        return $text
    }
    catch {
        Add-Check -Id ('FILE-{0}' -f $RelativePath) -Passed $false -Message $_.Exception.Message
        return $null
    }
}

function Get-BacklogHighestIdentifier {
    param(
        [Parameter(Mandatory)]
        [string]$BacklogText
    )

    $diagnostics = [System.Collections.Generic.List[string]]::new()
    $workItemMatches = [regex]::Matches(
        $BacklogText,
        '(?m)^\| BL-(?<Number>[0-9]{3}) \| (?<Status>[^|\r\n]+) \| (?<Task>[^|\r\n]+) \|'
    )
    $identifiers = [string[]]@(
        $workItemMatches | ForEach-Object {
            'BL-{0:D3}' -f [int]$_.Groups['Number'].Value
        }
    )
    $duplicates = [string[]]@(
        $identifiers |
            Group-Object |
            Where-Object { $_.Count -gt 1 } |
            ForEach-Object { $_.Name }
    )

    $computedIdentifier = $null
    if ($identifiers.Count -eq 0) {
        $diagnostics.Add('No canonical backlog work-item rows were found.')
    }
    else {
        $highestNumber = [int](
            $workItemMatches |
                ForEach-Object { [int]$_.Groups['Number'].Value } |
                Measure-Object -Maximum
        ).Maximum
        $computedIdentifier = 'BL-{0:D3}' -f $highestNumber
    }

    if ($duplicates.Count -ne 0) {
        $duplicateJson = ConvertTo-Json -InputObject ([string[]]$duplicates) -Compress
        $diagnostics.Add('Duplicate canonical work-item identifiers: {0}.' -f $duplicateJson)
    }

    $declarationMatches = [regex]::Matches(
        $BacklogText,
        '(?im)^The highest assigned backlog identifier is (?<Token>[^.\r\n]+)\.'
    )
    $declaredIdentifier = $null
    if ($declarationMatches.Count -eq 1) {
        $declaredMatch = [regex]::Match(
            $declarationMatches[0].Groups['Token'].Value.Trim(),
            '^`(?<Identifier>BL-[0-9]{3})`$'
        )
        if ($declaredMatch.Success) {
            $declaredIdentifier = $declaredMatch.Groups['Identifier'].Value
        }
        else {
            $diagnostics.Add('The declared highest assigned backlog identifier has an invalid format.')
        }
    }
    elseif ($declarationMatches.Count -eq 0) {
        $diagnostics.Add('The highest assigned backlog identifier declaration is missing.')
    }
    else {
        $diagnostics.Add('Multiple highest assigned backlog identifier declarations were found.')
    }

    if ($null -ne $computedIdentifier -and
        $null -ne $declaredIdentifier -and
        $computedIdentifier -cne $declaredIdentifier) {
        $diagnostics.Add((
                'Declared identifier {0} does not match computed identifier {1}.' -f
                $declaredIdentifier,
                $computedIdentifier
            ))
    }

    [pscustomobject]@{
        Passed                                   = $diagnostics.Count -eq 0
        ComputedHighestAssignedBacklogIdentifier = $computedIdentifier
        DeclaredHighestAssignedBacklogIdentifier = $declaredIdentifier
        Diagnostics                              = [string[]]$diagnostics
    }
}

function Get-InternalReferenceFindings {
    param(
        [Parameter(Mandatory)]
        [System.Collections.Generic.Dictionary[string, string]]$TextByPath,

        [Parameter(Mandatory)]
        [object[]]$Patterns
    )

    $findings = [System.Collections.Generic.List[object]]::new()
    foreach ($path in [string[]]@($TextByPath.Keys | Sort-Object -CaseSensitive)) {
        $text = $TextByPath[$path]
        foreach ($pattern in $Patterns) {
            $matches = [regex]::Matches($text, [string]$pattern.expression)
            if ($matches.Count -ne 0) {
                $findings.Add([pscustomobject]@{
                        path       = $path
                        patternId  = [string]$pattern.id
                        matchCount = $matches.Count
                    })
            }
        }
    }

    return [object[]]$findings
}

$requiredFiles = [string[]]@(
    '.github/workflows/ci.yml',
    '.github/workflows/metadata-regression.yml',
    '.github/workflows/release-build.yml',
    'AGENTS.md',
    'BACKLOG.md',
    'CHANGELOG.md',
    'CONTRIBUTING.md',
    'README.md',
    'benchmarks/README.md',
    'docs/architecture.md',
    'docs/codex-read-only-activation.md',
    'docs/documentation-quality-gate.md',
    'docs/efficiency-improvement-plan.md',
    'docs/security.md',
    'docs/technical-rename-to-flashgate-2026-07-11.md',
    'docs/testing.md'
)

foreach ($relativePath in $requiredFiles) {
    $text = Read-StrictUtf8 -RelativePath $relativePath
    if ($null -ne $text) {
        $documents.Add($relativePath, [string]$text)
    }
}

$backlog = [string]$documents['BACKLOG.md']
$ci = [string]$documents['.github/workflows/ci.yml']
$release = [string]$documents['.github/workflows/release-build.yml']

Add-Check -Id 'BACKLOG-BL343' -Passed ($backlog -match '(?m)^\| BL-343 \| Done \|') -Message 'BL-343 remains historical Done work.'
Add-Check -Id 'BACKLOG-BL344' -Passed ($backlog -match '(?m)^\| BL-344 \| Done \|') -Message 'BL-344 remains historical Done work.'
$backlogHighest = Get-BacklogHighestIdentifier -BacklogText $backlog
$backlogDiagnosticsJson = ConvertTo-Json -InputObject ([string[]]$backlogHighest.Diagnostics) -Compress
Add-Check -Id 'BACKLOG-HIGHEST' -Passed $backlogHighest.Passed -Message (
    'Computed={0}; Declared={1}; Diagnostics={2}' -f
    $backlogHighest.ComputedHighestAssignedBacklogIdentifier,
    $backlogHighest.DeclaredHighestAssignedBacklogIdentifier,
    $backlogDiagnosticsJson
)

Add-Check -Id 'CI-PRODUCT-GATES' -Passed (
    $ci.Contains('go vet ./...') -and
    $ci.Contains('Test-GoCoverage.ps1') -and
    $ci.Contains('golangci-lint run') -and
    $ci.Contains('go build')
) -Message 'Go vet, coverage, lint, and build remain active.'
Add-Check -Id 'CI-SCRIPT-GATES' -Passed (
    $ci.Contains('Test-DocumentationConsistency.ps1') -and
    $ci.Contains('Test-ShellScripts.Tests.ps1')
) -Message 'Documentation and shell gates remain active.'
Add-Check -Id 'CI-NO-INTERNAL-ORCHESTRATION' -Passed (
    -not ($ci -match 'Legacy governance orchestration|New-GovernanceWorkflowRecord|Test-GovernanceConsistency|Test-ClassicReviewArtifact')
) -Message 'Product CI has no private workflow-orchestration block.'
Add-Check -Id 'RELEASE-TECHNICAL' -Passed (
    $release.Contains('Resolve and validate release values') -and
    $release.Contains('Test-ReleaseArtifact.ps1')
) -Message 'Technical release validation remains active.'

$expectedDeletedPaths = [string[]]@(
    'Governance/assignment-governance-record.schema.json',
    'Governance/change-trigger-catalog.json',
    'Governance/CHANGE-TRIGGER-REVIEW-AND-BACKLOG-STANDARD.md',
    'Governance/CLOUD-CODEX-GOVERNANCE.md',
    'Governance/completion-report.schema.json',
    'Governance/finding-correction-assignment.schema.json',
    'Governance/finding-correction-completion.schema.json',
    'Governance/finding-correction-matrix.schema.json',
    'Governance/finding-correction-report-contract.schema.json',
    'Governance/finding-ledger.schema.json',
    'Governance/finding-regression-matrix.schema.json',
    'Governance/FINDING-REMEDIATION-AND-REVIEW-MODE-STANDARD.md',
    'Governance/focused-delta-review-record.schema.json',
    'Governance/generic-assignment-record.schema.json',
    'Governance/generic-completion-report.schema.json',
    'Governance/generic-handoff-contract.schema.json',
    'Governance/generic-independent-review-evidence.schema.json',
    'Governance/generic-package-inventory.schema.json',
    'Governance/generic-pre-review-validation-evidence.schema.json',
    'Governance/generic-report-contract.schema.json',
    'Governance/generic-scope-inventory.schema.json',
    'Governance/generic-validation-summary.schema.json',
    'Governance/governance-case-metadata.json',
    'Governance/governance-case-metadata.schema.json',
    'Governance/governance-handoff-contract.schema.json',
    'Governance/governance-report-contract.schema.json',
    'Governance/governance-validation-request.schema.json',
    'Governance/governance-validation-result.schema.json',
    'Governance/HANDOFF-ARTIFACT-AND-CLASSIC-READINESS-STANDARD.md',
    'Governance/MOBILE-CLOUD-HANDOFF.md',
    'Governance/previous-review-binding.schema.json',
    'Governance/publication-regression-evidence.schema.json',
    'Governance/publication-regression-expected-execution-input-binding.schema.json',
    'Governance/publication-regression-matrix-catalog.json',
    'Governance/publication-regression-matrix-catalog.schema.json',
    'Governance/publication-regression-result-v1.schema.json',
    'Governance/publication-regression-result.schema.json',
    'MOBILE.md',
    'docs/adr/016-governance-fixture-harness-execution-architecture.md',
    'scripts/GenericGovernanceGitEvidence.ps1',
    'scripts/GovernanceCaseSelection.psm1',
    'scripts/GovernanceHandoffPublication.psm1',
    'scripts/GovernanceValidationOrchestration.psm1',
    'scripts/Invoke-GenericGovernanceHandoffGeneratorChild.ps1',
    'scripts/Invoke-GovernanceValidation.ps1',
    'scripts/New-GovernanceHandoff.ps1',
    'scripts/New-GovernanceWorkflowRecord.ps1',
    'scripts/Test-ClassicReviewArtifact.ps1',
    'scripts/Test-FindingCorrectionHandoff.ps1',
    'scripts/Test-FindingCorrectionHandoffFixtures.ps1',
    'scripts/Test-GenericGovernanceHandoff.ps1',
    'scripts/Test-GenericGovernanceHandoffFixtures.ps1',
    'scripts/Test-GovernanceCaseSelectionFixtures.ps1',
    'scripts/Test-GovernanceConsistency.ps1',
    'scripts/Test-GovernanceConsistencyFixtures.ps1',
    'scripts/Test-GovernanceHandoffPublicationFixtures.ps1',
    'scripts/Test-GovernanceHostedCiPortabilityFixtures.ps1',
    'scripts/Test-GovernanceValidationOrchestration.ps1',
    'scripts/Test-ImplementationReviewHandoffFixtures.ps1',
    'scripts/testdata/Capture-GenericGovernanceHandoffGeneratorBinding.ps1',
    'scripts/testdata/Invoke-GovernanceHandoffCandidateDrift.ps1',
    'scripts/testdata/Invoke-GovernanceHandoffPublicationFixtureChild.ps1',
    'scripts/testdata/Invoke-GovernancePublicationInputDrift.ps1'
)

$duplicateDeletedPaths = [string[]]@(
    $expectedDeletedPaths |
        Group-Object -CaseSensitive |
        Where-Object { $_.Count -gt 1 } |
        ForEach-Object { $_.Name }
)
$deletedPathsStillPresent = [string[]]@(
    $expectedDeletedPaths |
        Where-Object { Test-Path -LiteralPath (Join-Path $resolvedRoot $_) }
)
Add-Check -Id 'BOUNDARY-DELETE-INVENTORY' -Passed (
    $expectedDeletedPaths.Count -eq 63 -and
    $duplicateDeletedPaths.Count -eq 0
) -Message ('ExpectedDeleted={0}; Duplicates={1}.' -f $expectedDeletedPaths.Count, $duplicateDeletedPaths.Count)
Add-Check -Id 'BOUNDARY-DELETED-PATHS-ABSENT' -Passed (
    $deletedPathsStillPresent.Count -eq 0
) -Message ('StillPresent={0}.' -f (ConvertTo-Json -InputObject ([string[]]$deletedPathsStillPresent) -Compress))

$operativePaths = [string[]]@(
    '.github/workflows/ci.yml',
    '.github/workflows/metadata-regression.yml',
    '.github/workflows/release-build.yml',
    'AGENTS.md',
    'CONTRIBUTING.md',
    'README.md',
    'docs/architecture.md',
    'docs/codex-read-only-activation.md',
    'docs/documentation-quality-gate.md',
    'docs/efficiency-improvement-plan.md',
    'docs/security.md',
    'docs/testing.md'
)
$operativeDocuments = [System.Collections.Generic.Dictionary[string, string]]::new(
    [System.StringComparer]::Ordinal
)
foreach ($path in $operativePaths) {
    if ($documents.ContainsKey($path)) {
        $operativeDocuments.Add($path, $documents[$path])
    }
}

$catalogMarker = '## Canonical task catalog'
$catalogIndex = $backlog.IndexOf($catalogMarker, [System.StringComparison]::Ordinal)
if ($catalogIndex -lt 0) {
    Add-Check -Id 'BACKLOG-OPERATIVE-PROJECTION' -Passed $false -Message 'Canonical task catalog marker is missing.'
}
else {
    $operativeDocuments.Add('BACKLOG.md#operative-preamble', $backlog.Substring(0, $catalogIndex))
    Add-Check -Id 'BACKLOG-OPERATIVE-PROJECTION' -Passed $true -Message 'Backlog history is separated from the operative preamble.'
}

$forbiddenPatterns = [object[]]@(
    [pscustomobject]@{ id = 'CODEX_WORK'; expression = 'Codex-Work' },
    [pscustomobject]@{ id = 'CLASSIC_REVIEW'; expression = 'ChatGPT Classic|Classic Review|ClassicReviewReady' },
    [pscustomobject]@{ id = 'FINDING_CORRECTION'; expression = 'FINDING_CORRECTION' },
    [pscustomobject]@{ id = 'INFRASTRUCTURE_REGISTER'; expression = 'INFRASTRUCTURE-WORK-REGISTER' },
    [pscustomobject]@{ id = 'PRIVATE_USER_PATH'; expression = '(?i)C:\\Users\\[^\\\r\n]+' },
    [pscustomobject]@{ id = 'PRIVATE_WORKSPACE_PATH'; expression = '(?i)C:\\Voxtronic\\Codex-Work|OneDrive\s+-\s+VOXTRONIC' },
    [pscustomobject]@{ id = 'REMOVED_GOVERNANCE_PATH'; expression = 'Governance/' },
    [pscustomobject]@{ id = 'REMOVED_MOBILE_ROUTER'; expression = 'MOBILE\.md' },
    [pscustomobject]@{ id = 'INTERNAL_TASK_ID'; expression = '\bINF-[0-9]{3}\b' },
    [pscustomobject]@{ id = 'REMOVED_WORKFLOW_SCRIPT'; expression = '(?:New|Invoke|Test)-(?:Generic)?Governance|Test-ClassicReviewArtifact|FindingCorrectionHandoff' }
)

$operativeFindings = [object[]]@(
    Get-InternalReferenceFindings -TextByPath $operativeDocuments -Patterns $forbiddenPatterns
)
$operativeFindingMessage = if ($operativeFindings.Count -eq 0) {
    'OperativeInternalReferenceCount=0.'
}
else {
    $operativeFindings | ConvertTo-Json -Compress -Depth 4
}
Add-Check -Id 'BOUNDARY-OPERATIVE-INTERNAL-REFERENCES' -Passed (
    $operativeFindings.Count -eq 0
) -Message $operativeFindingMessage

$runtimeFiles = [System.Collections.Generic.Dictionary[string, string]]::new(
    [System.StringComparer]::Ordinal
)
foreach ($relativeRoot in [string[]]@('cmd', 'internal')) {
    $fullRoot = Join-Path $resolvedRoot $relativeRoot
    foreach ($file in Get-ChildItem -LiteralPath $fullRoot -File -Recurse -Filter '*.go') {
        if ($file.Name.EndsWith('_test.go', [System.StringComparison]::Ordinal)) {
            continue
        }
        $relativePath = [System.IO.Path]::GetRelativePath($resolvedRoot, $file.FullName).Replace('\', '/')
        $runtimeFiles.Add($relativePath, [System.IO.File]::ReadAllText($file.FullName))
    }
}
$runtimeFindings = [object[]]@(
    Get-InternalReferenceFindings -TextByPath $runtimeFiles -Patterns $forbiddenPatterns
)
Add-Check -Id 'BOUNDARY-RUNTIME-DEPENDENCY' -Passed ($runtimeFindings.Count -eq 0) -Message (
    'RuntimeInternalReferenceCount={0}.' -f $runtimeFindings.Count
)

$planningRoot = Join-Path $resolvedRoot 'docs/planning'
$planningFiles = [object[]]@(
    Get-ChildItem -LiteralPath $planningRoot -File -Recurse -ErrorAction Stop
)
Add-Check -Id 'HISTORICAL-NON-OPERATIVE' -Passed (
    $planningFiles.Count -eq 11 -and
    (Test-Path -LiteralPath (Join-Path $resolvedRoot 'CHANGELOG.md') -PathType Leaf)
) -Message (
    'PlanningArtifactCount={0}; CHANGELOG retained; classification=HISTORICAL_NON_OPERATIVE.' -f
    $planningFiles.Count
)

$failedChecks = [object[]]@($checks | Where-Object { -not $_.passed })
$result = [pscustomobject]@{
    schemaVersion                            = 2
    status                                   = if ($failedChecks.Count -eq 0) { 'PASS' } else { 'FAIL' }
    checkCount                               = $checks.Count
    passedCount                              = $checks.Count - $failedChecks.Count
    failureCount                             = $failedChecks.Count
    deletedInternalPathCount                 = $expectedDeletedPaths.Count
    deletedInternalPathsExistAfter           = $deletedPathsStillPresent.Count -ne 0
    operativeInternalReferenceCount          = $operativeFindings.Count
    buildDependencyOnInternalInfrastructure  = $operativeFindings.Count -ne 0
    runtimeDependencyOnInternalInfrastructure = $runtimeFindings.Count -ne 0
    releaseDependencyOnInternalInfrastructure = $operativeFindings.Count -ne 0
    contributorDependencyOnInternalInfrastructure = $operativeFindings.Count -ne 0
    historicalClassification                 = 'HISTORICAL_NON_OPERATIVE'
    computedHighestAssignedBacklogIdentifier = $backlogHighest.ComputedHighestAssignedBacklogIdentifier
    declaredHighestAssignedBacklogIdentifier = $backlogHighest.DeclaredHighestAssignedBacklogIdentifier
    operativeFindings                        = [object[]]$operativeFindings
    runtimeFindings                          = [object[]]$runtimeFindings
    checks                                   = [object[]]$checks
}

$result | ConvertTo-Json -Depth 6
if ($failedChecks.Count -ne 0) {
    exit 1
}
