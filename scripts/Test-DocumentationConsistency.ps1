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

$publicAuthorityPaths = [string[]]@(
    'AGENTS.md',
    'BACKLOG.md',
    'CHANGELOG.md',
    'CONTRIBUTING.md',
    'README.md',
    'benchmarks/README.md',
    'docs/architecture.md',
    'docs/codex-read-only-activation.md',
    'docs/development/code-coverage.md',
    'docs/documentation-quality-gate.md',
    'docs/efficiency-improvement-plan.md',
    'docs/planning/README.md',
    'docs/planning/future-tool-adapter-plan.md',
    'docs/project-identity.md',
    'docs/protocol.md',
    'docs/roadmap.md',
    'docs/security.md',
    'docs/specification.md',
    'docs/testing.md',
    'docs/tool-conventions.md',
    'docs/tools.md',
    'docs/version-1-scope-and-release-boundary.md'
)
$supportingPaths = [string[]]@(
    '.github/workflows/ci.yml',
    '.github/workflows/metadata-regression.yml',
    '.github/workflows/release-build.yml',
    'docs/technical-rename-to-flashgate-2026-07-11.md'
)
$requiredFiles = [string[]]@($publicAuthorityPaths + $supportingPaths)

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
$bl344ContradictoryStatusCount = [regex]::Matches(
    $backlog,
    '(?i)\bBL-344\s+is\s+`?In Progress`?'
).Count
$bl344DoneSummaryCount = [regex]::Matches(
    $backlog,
    '(?i)\bBL-344\s+(?:is|are)\s+`?Done`?'
).Count
Add-Check -Id 'BACKLOG-BL344-STATUS-PARITY' -Passed (
    $bl344ContradictoryStatusCount -eq 0 -and
    $bl344DoneSummaryCount -gt 0
) -Message (
    'DoneSummaryCount={0}; ContradictoryStatusCount={1}.' -f
    $bl344DoneSummaryCount,
    $bl344ContradictoryStatusCount
)
$backlogHighest = Get-BacklogHighestIdentifier -BacklogText $backlog
$backlogDiagnosticsJson = ConvertTo-Json -InputObject ([string[]]$backlogHighest.Diagnostics) -Compress
Add-Check -Id 'BACKLOG-HIGHEST' -Passed $backlogHighest.Passed -Message (
    'Computed={0}; Declared={1}; Diagnostics={2}' -f
    $backlogHighest.ComputedHighestAssignedBacklogIdentifier,
    $backlogHighest.DeclaredHighestAssignedBacklogIdentifier,
    $backlogDiagnosticsJson
)

$expectedMilestones = [object[]]@(
    [pscustomobject]@{ id = 'BL-217'; status = 'Later' },
    [pscustomobject]@{ id = 'BL-345'; status = 'Later' },
    [pscustomobject]@{ id = 'BL-346'; status = 'Planned' },
    [pscustomobject]@{ id = 'BL-350'; status = 'Later' },
    [pscustomobject]@{ id = 'BL-351'; status = 'Later' },
    [pscustomobject]@{ id = 'BL-352'; status = 'Planned' },
    [pscustomobject]@{ id = 'BL-353'; status = 'Planned' }
)
$milestoneErrors = [System.Collections.Generic.List[string]]::new()
foreach ($item in $expectedMilestones) {
    $rowPattern = '(?m)^\| {0} \| {1} \|' -f $item.id, $item.status
    if (-not [regex]::IsMatch($backlog, $rowPattern)) {
        $milestoneErrors.Add('{0}:{1}' -f $item.id, $item.status)
    }
}
Add-Check -Id 'BACKLOG-MILESTONE-PARITY' -Passed ($milestoneErrors.Count -eq 0) -Message (
    'MissingExpectedRows={0}.' -f (ConvertTo-Json -InputObject ([string[]]$milestoneErrors) -Compress)
)

$readme = [string]$documents['README.md']
$architecture = [string]$documents['docs/architecture.md']
$identity = [string]$documents['docs/project-identity.md']
$protocol = [string]$documents['docs/protocol.md']
$scope = [string]$documents['docs/version-1-scope-and-release-boundary.md']
Add-Check -Id 'CURRENT-AND-V1-SCOPE-PARITY' -Passed (
    $readme.Contains('Today it provides secure, root-confined filesystem access') -and
    $readme.Contains('Version 1.0 extends that foundation') -and
    $architecture.Contains('Its current implementation provides root-confined filesystem access') -and
    $identity.Contains('Today it provides secure, root-confined filesystem access') -and
    $scope.Contains('This is a release target, not a claim that `2026-07-28` is implemented today.')
) -Message 'Current filesystem implementation and Version 1.0 targets agree across public authorities.'
Add-Check -Id 'MCP-REVISION-PARITY' -Passed (
    $readme.Contains('The implemented protocol remains MCP `2025-11-25`') -and
    $architecture.Contains('Current implementation remains only `2025-11-25`') -and
    $protocol.Contains('advertises MCP revision `2025-11-25`') -and
    $scope.Contains('The implemented revision remains `2025-11-25`') -and
    $scope.Contains('The final `2026-07-28` specification is now an accepted Version 1.0 implementation target')
) -Message 'Current MCP runtime and later Version 1.0 revision target remain distinct.'

$storagePlan = [string]$documents['docs/planning/future-tool-adapter-plan.md']
$storageNames = [string[]]@(
    'compression_supported', 'compression_enabled', 'compression_inherited_default',
    'encryption_supported', 'encryption_enabled', 'encryption_inherited_default'
)
$storageTermsAgree = $true
foreach ($name in $storageNames) {
    if (-not $backlog.Contains($name) -or -not $storagePlan.Contains($name)) {
        $storageTermsAgree = $false
    }
}
Add-Check -Id 'STORAGE-PLANNING-TERMINOLOGY' -Passed (
    $storageTermsAgree -and
    $backlog.Contains('An inherited default describes directory/child behavior, not the enabled state of the current path.') -and
    $storagePlan.Contains('An inherited default describes directory/child behavior, not the enabled state of the current path.')
) -Message 'Planned storage names and inherited-default semantics agree.'

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

$obsoleteTopLevelPaths = [string[]]@('Governance', 'MOBILE.md')
$obsoletePresent = [string[]]@(
    $obsoleteTopLevelPaths |
        Where-Object { Test-Path -LiteralPath (Join-Path $resolvedRoot $_) }
)
Add-Check -Id 'BOUNDARY-OBSOLETE-TOP-LEVEL' -Passed ($obsoletePresent.Count -eq 0) -Message (
    'Present={0}.' -f (ConvertTo-Json -InputObject ([string[]]$obsoletePresent) -Compress)
)

$operativePaths = [string[]]@(
    $publicAuthorityPaths |
        Where-Object { $_ -cne 'docs/planning/README.md' }
)
$operativeDocuments = [System.Collections.Generic.Dictionary[string, string]]::new(
    [System.StringComparer]::Ordinal
)
foreach ($path in $operativePaths) {
    if ($documents.ContainsKey($path)) {
        $operativeDocuments.Add($path, $documents[$path])
    }
}
$publicAuthorityMissing = [string[]]@(
    $publicAuthorityPaths | Where-Object { -not $documents.ContainsKey($_) }
)
Add-Check -Id 'PUBLIC-AUTHORITY-COVERAGE' -Passed (
    $publicAuthorityMissing.Count -eq 0 -and
    $operativeDocuments.Count -eq ($publicAuthorityPaths.Count - 1)
) -Message (
    'Authorities={0}; ActiveWholeDocuments={1}; HistoricalIndex=docs/planning/README.md; Missing={2}.' -f
    $publicAuthorityPaths.Count,
    $operativeDocuments.Count,
    (ConvertTo-Json -InputObject ([string[]]$publicAuthorityMissing) -Compress)
)

$technicalRenamePath = 'docs/technical-rename-to-flashgate-2026-07-11.md'
$technicalRename = [string]$documents[$technicalRenamePath]
$currentIdentifiersMarker = '## Current identifiers'
$existingClonesMarker = '## Existing clones and GitHub redirects'
$ownerAmendmentMarker = '## Owner migration amendment - 2026-07-20'
$compatibilityMarker = '## Compatibility and scope'
$currentIdentifiersIndex = $technicalRename.IndexOf($currentIdentifiersMarker, [System.StringComparison]::Ordinal)
$existingClonesIndex = $technicalRename.IndexOf($existingClonesMarker, [System.StringComparison]::Ordinal)
$ownerAmendmentIndex = $technicalRename.IndexOf($ownerAmendmentMarker, [System.StringComparison]::Ordinal)
$compatibilityIndex = $technicalRename.IndexOf($compatibilityMarker, [System.StringComparison]::Ordinal)
if ($currentIdentifiersIndex -lt 0 -or
    $existingClonesIndex -le $currentIdentifiersIndex -or
    $ownerAmendmentIndex -lt 0 -or
    $compatibilityIndex -le $ownerAmendmentIndex) {
    Add-Check -Id 'TECHNICAL-RENAME-OPERATIVE-PROJECTION' -Passed $false -Message 'Current technical-rename sections cannot be projected safely.'
}
else {
    $technicalRenameOperativeProjection =
        $technicalRename.Substring($currentIdentifiersIndex, $existingClonesIndex - $currentIdentifiersIndex) +
        $technicalRename.Substring($ownerAmendmentIndex, $compatibilityIndex - $ownerAmendmentIndex)
    $operativeDocuments.Add(
        'docs/technical-rename-to-flashgate-2026-07-11.md#current-sections',
        $technicalRenameOperativeProjection
    )
    Add-Check -Id 'TECHNICAL-RENAME-OPERATIVE-PROJECTION' -Passed $true -Message 'Only current identifier and owner-amendment sections are operative.'
}

# The complete canonical task catalog is current public authority, including Done rows.
$operativeDocuments['BACKLOG.md'] = $backlog

# This dated index points to non-operative migration and review provenance.
# Only its current public capability-plan link is active project guidance.
$planningIndex = [string]$documents['docs/planning/README.md']
$planningCurrentMarker = '[Future tool and adapter plan](future-tool-adapter-plan.md)'
$planningCurrentStart = $planningIndex.IndexOf(
    $planningCurrentMarker,
    [System.StringComparison]::Ordinal
)
$planningCurrentEnd = $planningIndex.IndexOf(
    '## Interpretation',
    [System.StringComparison]::Ordinal
)
if ($planningCurrentStart -lt 0 -or $planningCurrentEnd -le $planningCurrentStart) {
    Add-Check -Id 'HISTORICAL-INDEX-PROJECTION' -Passed $false -Message 'Planning index current link cannot be projected.'
}
else {
    $operativeDocuments.Add(
        'docs/planning/README.md#current-public-link',
        $planningIndex.Substring(
            $planningCurrentStart,
            $planningCurrentEnd - $planningCurrentStart
        )
    )
    Add-Check -Id 'HISTORICAL-INDEX-PROJECTION' -Passed $true -Message 'Dated planning/review entries remain non-operative; current public link is scanned.'
}

$privateHostPathPatterns = [object[]]@(
    [pscustomobject]@{ id = 'PRIVATE_USER_PATH'; expression = '(?i)C:\\Users\\[^\\\r\n]+' },
    [pscustomobject]@{ id = 'PRIVATE_WORK_ROOT'; expression = '(?i)Codex-Work|<CodexTempRoot>' },
    [pscustomobject]@{ id = 'PRIVATE_SYNC_PATH'; expression = '(?i)OneDrive\s+-\s+[^\\/\r\n]+(?:\\[^\\\r\n]+)?' }
)
$forbiddenPatterns = [object[]]@(
    $privateHostPathPatterns + [object[]]@(
        [pscustomobject]@{ id = 'INTERNAL_TASK_ID'; expression = '(?i)\bINF-?[0-9]{3}(?:-[A-Z0-9]+)?\b' },
        [pscustomobject]@{ id = 'RETIRED_WORKFLOW_PATH'; expression = '(?-i:Governance[/\\])|(?i:MOBILE\.md|(?:New|Invoke|Test)-(?:Generic)?Governance|Test-ClassicReviewArtifact|FindingCorrectionHandoff|FINDING_CORRECTION)' },
        [pscustomobject]@{ id = 'PRIVATE_REVIEW_WORKFLOW'; expression = '(?i)ChatGPT Classic|Classic Review|ClassicReviewReady|Classic independent review' },
        [pscustomobject]@{ id = 'PRIVATE_CONTROL_PLANE_DEPENDENCY'; expression = '(?i)\b(?:requires?|depends?\s+on)\s+(?:access\s+to\s+)?(?:a\s+|the\s+)?(?:private|internal)\s+(?:development\s+)?control[- ]plane\b' }
    )
)

$privateHostPathFindings = [object[]]@(
    Get-InternalReferenceFindings -TextByPath $operativeDocuments -Patterns $privateHostPathPatterns
)
$privateActiveHostPathCount = 0
foreach ($finding in $privateHostPathFindings) {
    $privateActiveHostPathCount += [int]$finding.matchCount
}
Add-Check -Id 'BOUNDARY-PRIVATE-ACTIVE-HOST-PATHS' -Passed (
    $privateActiveHostPathCount -eq 0
) -Message ('PrivateActiveHostPathCount={0}.' -f $privateActiveHostPathCount)

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
$planningClassificationValid =
    $planningIndex.Contains('non-canonical planning/review evidence') -and
    $planningIndex.Contains('historical evidence') -and
    $planningIndex.Contains('None of these files authorize Git, remote, correction, integration, branch deletion or release actions.')
Add-Check -Id 'HISTORICAL-NON-OPERATIVE' -Passed (
    $planningClassificationValid -and
    (Test-Path -LiteralPath (Join-Path $resolvedRoot 'CHANGELOG.md') -PathType Leaf)
) -Message (
    'PlanningArtifactCount={0}; PlanningIndexClassificationValid={1}; CHANGELOG retained; classification=HISTORICAL_NON_OPERATIVE.' -f
    $planningFiles.Count,
    $planningClassificationValid
)

$failedChecks = [object[]]@($checks | Where-Object { -not $_.passed })
$result = [pscustomobject]@{
    schemaVersion                            = 2
    status                                   = if ($failedChecks.Count -eq 0) { 'PASS' } else { 'FAIL' }
    checkCount                               = $checks.Count
    passedCount                              = $checks.Count - $failedChecks.Count
    failureCount                             = $failedChecks.Count
    operativeInternalReferenceCount          = $operativeFindings.Count
    buildDependencyOnInternalInfrastructure  = $operativeFindings.Count -ne 0
    runtimeDependencyOnInternalInfrastructure = $runtimeFindings.Count -ne 0
    releaseDependencyOnInternalInfrastructure = $operativeFindings.Count -ne 0
    contributorDependencyOnInternalInfrastructure = $operativeFindings.Count -ne 0
    historicalClassification                 = 'HISTORICAL_NON_OPERATIVE'
    computedHighestAssignedBacklogIdentifier = $backlogHighest.ComputedHighestAssignedBacklogIdentifier
    declaredHighestAssignedBacklogIdentifier = $backlogHighest.DeclaredHighestAssignedBacklogIdentifier
    bl344ContradictoryStatusCount             = $bl344ContradictoryStatusCount
    privateActiveHostPathCount                = $privateActiveHostPathCount
    privateHostPathFindings                   = [object[]]$privateHostPathFindings
    operativeFindings                        = [object[]]$operativeFindings
    runtimeFindings                          = [object[]]$runtimeFindings
    checks                                   = [object[]]$checks
}

$result | ConvertTo-Json -Depth 6
if ($failedChecks.Count -ne 0) {
    exit 1
}
