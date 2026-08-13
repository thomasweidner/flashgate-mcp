[CmdletBinding()]
param(
    [string]$RepositoryRoot = (Split-Path -Parent $PSScriptRoot),
    [string]$CanonicalArtifactValidatorPath = (Join-Path $PSScriptRoot 'Test-ClassicReviewArtifact.ps1'),
    [string[]]$CaseName = @(),
    [string[]]$Tag = @(),
    [string]$TargetPlatform,
    [string[]]$AvailableCapability = @(),
    [string]$ProgressPath,
    [string]$ResultPath
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$repositoryArtifactValidatorMirrorPath = Join-Path $PSScriptRoot 'Test-ClassicReviewArtifact.ps1'
$externalCanonicalArtifactValidatorPath = 'C:\Users\ThomasW\OneDrive - VOXTRONIC\Desktop\Voxtronic\Codex-Work\Scripts\Test-ClassicReviewArtifact.ps1'

function Copy-Record {
    param([Parameter(Mandatory)][object]$Record)

    return ($Record | ConvertTo-Json -Depth 100 | ConvertFrom-Json -Depth 100 -DateKind String)
}

function New-Finding {
    param(
        [Parameter(Mandatory)][string]$Id,
        [Parameter(Mandatory)][string]$Disposition,
        [string]$BoundaryId,
        [string]$BoundaryType
    )

    [string[]]$regressionEvidenceIds = @()
    if ($Disposition -in @('CORRECTED', 'DISCOVERED_AND_CORRECTED_IN_RUN')) {
        $regressionEvidenceIds = [string[]]@('REG-FIXTURE')
    }
    [string[]]$affectedPaths = @('Governance/change-trigger-catalog.json')

    return [ordered]@{
        id = $Id
        severity = 'MAJOR'
        disposition = $Disposition
        evidence = 'fixture evidence'
        cause = 'fixture cause'
        correction = if ($Disposition -in @('CORRECTED', 'DISCOVERED_AND_CORRECTED_IN_RUN')) { 'fixture correction' } else { '' }
        regressionEvidenceIds = $regressionEvidenceIds
        affectedPaths = $affectedPaths
        nonFileBoundary = $null
        boundaryId = if ([string]::IsNullOrWhiteSpace($BoundaryId)) { $null } else { $BoundaryId }
        boundaryType = if ([string]::IsNullOrWhiteSpace($BoundaryType)) { $null } else { $BoundaryType }
    }
}

function New-BaseRecord {
    param(
        [Parameter(Mandatory)][string]$Mode,
        [Parameter(Mandatory)][string]$Checkpoint,
        [string[]]$ObservedTriggers = @(),
        [string[]]$TriggeredDomains = @(),
        [string[]]$AffectedGates = @()
    )

    $mutationAllowed = $Mode -eq 'BUNDLED_CORRECTION'
    $independent = $Mode -in @('INDEPENDENT_REVIEW', 'FOCUSED_INDEPENDENT_DELTA_REVIEW')

    return [ordered]@{
        schemaVersion = 1
        recordedAt = '2026-07-29T00:00:00+02:00'
        taskId = 'BL-333/BL-334'
        repository = 'https://github.com/thomasweidner/flashgate-mcp.git'
        baselineCommit = $actualHead
        currentCommit = $actualHead
        branch = 'docs/bl-333-bl-334-change-trigger-governance'
        executionMode = $Mode
        checkpoint = $Checkpoint
        changeTriggerReviewResult = if ($ObservedTriggers.Count -eq 0) { 'NO_TRIGGER' } else { 'EXISTING_GATES_REQUIRED' }
        currentStateGate = [ordered]@{
            result = 'PASS'
            repositoryIdentityBound = $true
            commitAndBranchBound = $true
            completeStatusBound = $true
            scopeAndIdsBound = $true
            parallelWorktreesBound = $true
        }
        triggeredDomains = @($TriggeredDomains)
        observedTriggers = @($ObservedTriggers)
        affectedContinuousGates = @($AffectedGates)
        existingBacklogCoverage = @('BL-333', 'BL-334')
        duplicateSearch = [ordered]@{
            performed = $ObservedTriggers.Count -gt 0
            sources = if ($ObservedTriggers.Count -gt 0) { @('BACKLOG.md', 'Governance/change-trigger-catalog.json') } else { @() }
            result = if ($ObservedTriggers.Count -gt 0) { 'EXISTING_ITEM_REUSED' } else { 'NOT_REQUIRED' }
        }
        repeatedChecks = @()
        checksNotRequired = @('performance baseline')
        newBacklogItems = @()
        updatedBacklogOrRegisterEntries = @()
        deferredTriggerItems = @()
        decisionBoundaries = @()
        releaseImpact = 'No release artifact or runtime behavior change.'
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
            independentDeltaReviewRequired = $Mode -eq 'BUNDLED_CORRECTION'
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
            required = $false
            sourceVerified = $false
            sources = @()
            workflowCommit = $null
            runId = $null
            runAttempt = $null
            event = $null
            ref = $null
            headSha = $null
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
}

function Get-LowerSha256 {
    param([Parameter(Mandatory)][byte[]]$Bytes)

    return [Convert]::ToHexString(
        [System.Security.Cryptography.SHA256]::HashData($Bytes)
    ).ToLowerInvariant()
}

function ConvertTo-PowerShellSingleQuotedLiteral {
    param([Parameter(Mandatory)][AllowEmptyString()][string]$Value)

    return "'" + $Value.Replace("'", "''") + "'"
}

function ConvertTo-PowerShellArrayLiteral {
    param([AllowEmptyCollection()][string[]]$Value)

    return '@(' + ((@($Value) | ForEach-Object {
                    ConvertTo-PowerShellSingleQuotedLiteral -Value $_
                }) -join ',') + ')'
}

function Convert-GlobToRegex {
    param([Parameter(Mandatory)][string]$Pattern)

    $escaped = [regex]::Escape($Pattern)
    $escaped = $escaped.Replace('\*\*', '.*')
    $escaped = $escaped.Replace('\*', '[^/]*')
    return '^' + $escaped + '$'
}

function New-MinimalClassicReviewPackage {
    param(
        [Parameter(Mandatory)][string]$Root,
        [Parameter(Mandatory)][string]$Name,
        [Parameter(Mandatory)][string]$ReadmeText,
        [ValidateSet('NONE', 'DUPLICATE_PATH', 'CASE_COLLISION', 'ABSOLUTE_PATH', 'TRAVERSAL_PATH', 'REPARSE_ENTRY')]
        [string]$Mutation = 'NONE'
    )

    $utf8 = [System.Text.UTF8Encoding]::new($false)
    $packagePath = Join-Path $Root ($Name + '.zip')
    $readmeBytes = $utf8.GetBytes($ReadmeText)
    $handoffBytes = $utf8.GetBytes(
        '{"schemaVersion":1,"classicReviewReady":true,"transferUnit":"single-package"}' + "`n"
    )
    $manifestObject = [ordered]@{
        schemaVersion = 1
        classicReviewReady = $true
        entries = @(
            [ordered]@{
                path = 'README.md'
                length = $readmeBytes.LongLength
                sha256 = Get-LowerSha256 -Bytes $readmeBytes
            },
            [ordered]@{
                path = 'handoff.json'
                length = $handoffBytes.LongLength
                sha256 = Get-LowerSha256 -Bytes $handoffBytes
            }
        )
    }
    $manifestJsonBytes = $utf8.GetBytes(
        (($manifestObject | ConvertTo-Json -Depth 20) + "`n")
    )

    $payloads = [System.Collections.Generic.List[object]]::new()
    foreach ($payload in @(
            [pscustomobject]@{ Path = 'README.md'; Bytes = $readmeBytes; ExternalAttributes = 0 },
            [pscustomobject]@{ Path = 'handoff.json'; Bytes = $handoffBytes; ExternalAttributes = 0 },
            [pscustomobject]@{ Path = 'manifest.json'; Bytes = $manifestJsonBytes; ExternalAttributes = 0 }
        )) {
        [void]$payloads.Add($payload)
    }

    $manifestRecords = @(
        $payloads |
            ForEach-Object {
                [pscustomobject]@{
                    Path = [string]$_.Path
                    Line = "$(Get-LowerSha256 -Bytes ([byte[]]$_.Bytes))  $($_.Bytes.LongLength)  $($_.Path)"
                }
            }
    )
    $manifestPaths = [string[]]@($manifestRecords | ForEach-Object Path)
    [array]::Sort($manifestPaths, [System.StringComparer]::Ordinal)
    $manifestLineByPath = @{}
    foreach ($record in $manifestRecords) {
        $manifestLineByPath[$record.Path] = $record.Line
    }
    $manifestBytes = $utf8.GetBytes(
        ((@($manifestPaths | ForEach-Object { $manifestLineByPath[$_] }) -join "`n") + "`n")
    )
    [void]$payloads.Add([pscustomobject]@{
            Path = 'MANIFEST.sha256'
            Bytes = $manifestBytes
            ExternalAttributes = 0
        })

    switch ($Mutation) {
        'DUPLICATE_PATH' {
            [void]$payloads.Add([pscustomobject]@{
                    Path = 'README.md'
                    Bytes = $utf8.GetBytes("duplicate`n")
                    ExternalAttributes = 0
                })
        }
        'CASE_COLLISION' {
            [void]$payloads.Add([pscustomobject]@{
                    Path = 'readme.md'
                    Bytes = $utf8.GetBytes("case collision`n")
                    ExternalAttributes = 0
                })
        }
        'ABSOLUTE_PATH' {
            [void]$payloads.Add([pscustomobject]@{
                    Path = '/absolute.txt'
                    Bytes = $utf8.GetBytes("unsafe`n")
                    ExternalAttributes = 0
                })
        }
        'TRAVERSAL_PATH' {
            [void]$payloads.Add([pscustomobject]@{
                    Path = '../traversal.txt'
                    Bytes = $utf8.GetBytes("unsafe`n")
                    ExternalAttributes = 0
                })
        }
        'REPARSE_ENTRY' {
            [void]$payloads.Add([pscustomobject]@{
                    Path = 'linked.txt'
                    Bytes = $utf8.GetBytes("link target`n")
                    ExternalAttributes = [int][System.IO.FileAttributes]::ReparsePoint
                })
        }
    }

    Add-Type -AssemblyName System.IO.Compression
    $stream = [System.IO.File]::Open(
        $packagePath,
        [System.IO.FileMode]::CreateNew,
        [System.IO.FileAccess]::ReadWrite,
        [System.IO.FileShare]::None
    )
    $archive = $null
    try {
        $archive = [System.IO.Compression.ZipArchive]::new(
            $stream,
            [System.IO.Compression.ZipArchiveMode]::Create,
            $true
        )
        foreach ($payload in $payloads) {
            $entry = $archive.CreateEntry(
                [string]$payload.Path,
                [System.IO.Compression.CompressionLevel]::Optimal
            )
            $entry.ExternalAttributes = [int]$payload.ExternalAttributes
            $entryStream = $entry.Open()
            try {
                $entryStream.Write([byte[]]$payload.Bytes, 0, [int]$payload.Bytes.LongLength)
            }
            finally {
                $entryStream.Dispose()
            }
        }
    }
    finally {
        if ($null -ne $archive) {
            $archive.Dispose()
        }
        $stream.Dispose()
    }

    return [pscustomobject]@{
        PackagePath = $packagePath
        PackageSha256 = (Get-FileHash -LiteralPath $packagePath -Algorithm SHA256).Hash.ToLowerInvariant()
        ManifestJsonSha256 = Get-LowerSha256 -Bytes $manifestJsonBytes
    }
}

function Test-ClassicTransferPlan {
    param(
        [Parameter(Mandatory)][ValidateRange(1, [int]::MaxValue)][int]$RequiredFileCount,
        [Parameter(Mandatory)][ValidateRange(1, [int]::MaxValue)][int]$TransferFileCount,
        [Parameter(Mandatory)][string]$TransferFileName,
        [Parameter(Mandatory)][string]$Instruction
    )

    if ($RequiredFileCount -eq 1) {
        return $TransferFileCount -eq 1
    }

    $forbiddenMemberInstruction = $Instruction -match (
        '(?i)\b(separate|separately|individual|individually|einzeln|Paketmitglieder)\b'
    )
    return (
        $TransferFileCount -eq 1 -and
        [System.IO.Path]::GetExtension($TransferFileName) -ieq '.zip' -and
        -not $forbiddenMemberInstruction
    )
}

function New-ClassicFixturePackage {
    param(
        [Parameter(Mandatory)][string]$Root,
        [Parameter(Mandatory)][object]$Record,
        [Parameter(Mandatory)][string[]]$ChangedPaths,
        [Parameter(Mandatory)][string]$ValidatorPath
    )

    $Record = Copy-Record -Record $Record
    $packagePath = Join-Path $Root 'positive-classic-handoff.zip'
    $staging = Join-Path $Root 'positive-classic-handoff'
    [void][System.IO.Directory]::CreateDirectory($staging)
    $utf8 = [System.Text.UTF8Encoding]::new($false)
    $validatorHash = (
        Get-FileHash -LiteralPath $ValidatorPath -Algorithm SHA256
    ).Hash.ToLowerInvariant()
    $expectedFindingIds = @(
        'BL333-BL334-REV-013',
        'BL333-BL334-REV-015'
    )
    $closedFindingIds = @(
        'BL333-BL334-REV-007',
        'BL333-BL334-REV-008',
        'BL333-BL334-REV-010'
    )
    $handoffStatus = 'FOURTH_BUNDLED_CORRECTION_COMPLETE_AWAITING_FOCUSED_DELTA_REVIEW'
    $nextAction = 'Perform only the focused independent delta review of BL333-BL334-REV-013 and BL333-BL334-REV-015 with the new verified review package.'
    $queue = 'BL-333/BL-334 -> BL-335 -> BL-251 -> BL-324 -> final documentation convergence -> remove Local Work Register'
    $referenceOnlyPaths = @('README.md')
    $directInterfacePaths = @($ChangedPaths | Select-Object -First 1)
    $previousReviewPackage = Join-Path $Root 'previous-focused-review.zip'
    [System.IO.File]::WriteAllBytes(
        $previousReviewPackage,
        $utf8.GetBytes('fixture previous focused review package')
    )
    $previousReviewHash = (
        Get-FileHash -LiteralPath $previousReviewPackage -Algorithm SHA256
    ).Hash.ToLowerInvariant()

    $correctionPatchText = @(
        $ChangedPaths | ForEach-Object {
            "diff --git a/$_ b/$_`n--- a/$_`n+++ b/$_`n@@ -1 +1 @@`n-fixture`n+fixture corrected`n"
        }
    ) -join ''
    $currentDeltaText = $correctionPatchText + ((@(
                $referenceOnlyPaths | ForEach-Object {
                    "diff --git a/$_ b/$_`n--- a/$_`n+++ b/$_`n@@ -1 +1 @@`n-reference before`n+reference after`n"
                }
            )) -join '')
    $correctionPatchPath = Join-Path $staging 'correction-only.patch'
    $currentDeltaPath = Join-Path $staging 'current-delta.patch'
    [System.IO.File]::WriteAllText($correctionPatchPath, $correctionPatchText, $utf8)
    [System.IO.File]::WriteAllText($currentDeltaPath, $currentDeltaText, $utf8)
    $correctionPatchHash = (
        Get-FileHash -LiteralPath $correctionPatchPath -Algorithm SHA256
    ).Hash.ToLowerInvariant()
    $currentDeltaHash = (
        Get-FileHash -LiteralPath $currentDeltaPath -Algorithm SHA256
    ).Hash.ToLowerInvariant()

    $regressionIdsByFinding = @{}
    $allRegressionIds = [System.Collections.Generic.List[string]]::new()
    for ($index = 0; $index -lt $expectedFindingIds.Count; $index++) {
        $findingId = $expectedFindingIds[$index]
        $testId = 'REG-SECOND-' + $findingId
        $regressionIdsByFinding[$findingId] = @($testId)
        [void]$allRegressionIds.Add($testId)
    }
    $findingRecords = @(
        foreach ($findingId in $expectedFindingIds) {
            $finding = New-Finding -Id $findingId -Disposition 'CORRECTED'
            $finding.evidence = "fixture:$findingId"
            $finding.cause = "Fixture cause for $findingId."
            $finding.correction = "Fixture correction for $findingId."
            $finding.regressionEvidenceIds = @($regressionIdsByFinding[$findingId])
            $finding.affectedPaths = @($ChangedPaths)
            $finding
        }
    )

    $externalDefinitions = @(
        [pscustomobject]@{
            Slug = 'global-codex-agents'
            Scope = 'GLOBAL_CODEX_RULES'
            ActivePath = 'C:\Users\ThomasW\.codex\AGENTS.md'
        },
        [pscustomobject]@{
            Slug = 'workspace-agents'
            Scope = 'VOXTRONIC_WORKSPACE_RULES'
            ActivePath = 'C:\Voxtronic\AGENTS.md'
        },
        [pscustomobject]@{
            Slug = 'project-wide-root-standard'
            Scope = 'PROJECT_WIDE_ROOT_STANDARD'
            ActivePath = 'C:\Users\ThomasW\OneDrive - VOXTRONIC\Desktop\Voxtronic\Codex-Work\PROJECT-WIDE-REVIEW-AND-VALIDATION-STANDARD.md'
        },
        [pscustomobject]@{
            Slug = 'project-wide-governance-standard'
            Scope = 'PROJECT_WIDE_GOVERNANCE_STANDARD'
            ActivePath = 'C:\Users\ThomasW\OneDrive - VOXTRONIC\Desktop\Voxtronic\Codex-Work\Governance\PROJECT-WIDE-REVIEW-AND-VALIDATION-STANDARD.md'
        },
        [pscustomobject]@{
            Slug = 'local-work-register'
            Scope = 'FLASHGATE_LOCAL_WORK_REGISTER'
            ActivePath = 'C:\Voxtronic\MCP\flashgate-mcp-local-work-register.md'
        }
    )
    $externalChanges = @(
        foreach ($definition in $externalDefinitions) {
            $relativeRoot = 'external/' + $definition.Slug
            [void][System.IO.Directory]::CreateDirectory((Join-Path $staging $relativeRoot))
            $beforeText = "fixture before $($definition.Scope)`n"
            $afterText = if ($definition.Scope -ceq 'FLASHGATE_LOCAL_WORK_REGISTER') {
                @"
Status: FOURTH_BUNDLED_CORRECTION_COMPLETE_AWAITING_FOCUSED_DELTA_REVIEW
$queue
ClosedFindingCount: 3
PendingDeltaFindingCount: 2
ClosedFindings: BL333-BL334-REV-007,BL333-BL334-REV-008,BL333-BL334-REV-010
PendingFindings: BL333-BL334-REV-013,BL333-BL334-REV-015
Commit Preparation remains blocked pending the focused independent delta review.
BL-335 remains blocked.
"@
            }
            else {
                "fixture after $($definition.Scope)`n"
            }
            $beforePath = Join-Path $staging ($relativeRoot + '/before.txt')
            $afterPath = Join-Path $staging ($relativeRoot + '/after.txt')
            $diffPath = Join-Path $staging ($relativeRoot + '/change.patch')
            [System.IO.File]::WriteAllText($beforePath, $beforeText, $utf8)
            [System.IO.File]::WriteAllText($afterPath, $afterText, $utf8)
            [System.IO.File]::WriteAllText(
                $diffPath,
                "--- before.txt`n+++ after.txt`n@@ -1 +1 @@`n-$($beforeText.TrimEnd())`n+$($afterText.TrimEnd())`n",
                $utf8
            )
            [ordered]@{
                activePath = $definition.ActivePath
                backupPath = $definition.ActivePath + '.backup'
                scope = $definition.Scope
                beforeSha256 = (
                    Get-FileHash -LiteralPath $beforePath -Algorithm SHA256
                ).Hash.ToLowerInvariant()
                afterSha256 = (
                    Get-FileHash -LiteralPath $afterPath -Algorithm SHA256
                ).Hash.ToLowerInvariant()
                beforeSize = (Get-Item -LiteralPath $beforePath).Length
                afterSize = (Get-Item -LiteralPath $afterPath).Length
                beforePayload = $relativeRoot + '/before.txt'
                afterPayload = $relativeRoot + '/after.txt'
                diffPayload = $relativeRoot + '/change.patch'
            }
        }
    )
    $externalManifest = [ordered]@{
        schemaVersion = 1
        changes = $externalChanges
    }
    [System.IO.File]::WriteAllText(
        (Join-Path $staging 'external-governance-manifest.json'),
        ($externalManifest | ConvertTo-Json -Depth 100),
        $utf8
    )
    $externalPaths = @($externalChanges | ForEach-Object { [string]$_.activePath })

    $scopeInventory = [ordered]@{
        schemaVersion = 1
        entries = @($ChangedPaths | ForEach-Object { [ordered]@{ path = $_ } })
        referenceOnlyPaths = @($referenceOnlyPaths)
    }
    $scopePath = Join-Path $staging 'scope-inventory.json'
    [System.IO.File]::WriteAllText(
        $scopePath,
        ($scopeInventory | ConvertTo-Json -Depth 100),
        $utf8
    )

    $Record.taskId = 'BL-333/BL-334'
    $Record.executionMode = 'BUNDLED_CORRECTION'
    $Record.checkpoint = 'SPRINT_CLOSE'
    $Record.review.repositoryMutationAllowed = $true
    $Record.review.externalMutationAllowed = $true
    $Record.review.findingFixesPerformed = $true
    $Record.review.reviewerIndependencePreserved = $false
    $Record.review.originalFindings = @($findingRecords)
    $Record.review.permanentRegressionEvidence = @($allRegressionIds)
    $Record.review.focusedValidationResult = 'PASS'
    $Record.review.independentDeltaReviewRequired = $true
    $Record.commitPreparation.allFindingsClosed = $false
    $Record.commitPreparation.independentDeltaReviewComplete = $false
    $Record.commitPreparation.scopeVerified = $false
    $Record.commitPreparation.validationPassed = $false
    $Record.commitPreparation.commitAuthorized = $false
    $Record.focusedDelta.previousReviewPackage = $previousReviewPackage
    $Record.focusedDelta.priorReviewBaselineSha256 = $previousReviewHash
    $Record.focusedDelta.correctionStartCommit = $Record.currentCommit
    $Record.focusedDelta.correctionPatchArtifact = 'correction-only.patch'
    $Record.focusedDelta.correctionPatchSha256 = $correctionPatchHash
    $Record.focusedDelta.currentDeltaArtifact = 'current-delta.patch'
    $Record.focusedDelta.currentDeltaSha256 = $currentDeltaHash
    $Record.focusedDelta.correctionOnlyPaths = @($ChangedPaths)
    $Record.focusedDelta.reviewedFindingIds = @($expectedFindingIds)
    $Record.focusedDelta.directInterfacePaths = @($directInterfacePaths)
    $Record.focusedDelta.regressionEvidenceIds = @($allRegressionIds)
    $Record.focusedDelta.allowedDeltaPaths = @($ChangedPaths)
    $Record.focusedDelta.referenceOnlyPaths = @($referenceOnlyPaths)

    $artifactNames = @(
        'HANDOFF.md',
        'assignment-record.json',
        'completion-report.json',
        'correction-only.patch',
        'current-delta.patch',
        'external-governance-manifest.json',
        'finding-correction-matrix.json',
        'finding-regression-matrix.json',
        'focused-delta-review-record.json',
        'MANIFEST.sha256',
        'readiness-evidence.json',
        'report.md',
        'scope-inventory.json',
        'trusted-expected-hashes.json',
        'validation-summary.json'
    ) + @(
        $externalChanges |
            ForEach-Object { @($_.beforePayload, $_.afterPayload, $_.diffPayload) }
    )
    $Record.handoff.required = $true
    $Record.handoff.package = $packagePath
    $Record.handoff.artifacts = @($artifactNames | Sort-Object)
    $Record.handoff.scopeInventoryResult = 'PASS'
    $Record.handoff.patchCompletenessResult = 'PASS'
    $Record.handoff.manifestResult = 'PASS'
    $Record.handoff.missingArtifacts = @()
    $Record.handoff.classicReviewReady = $true
    $Record.handoff.canonicalValidatorPath = $ValidatorPath
    $Record.handoff.canonicalValidatorSha256 = $validatorHash
    $Record.handoff.canonicalValidatorExitCode = 0
    $Record.handoff.canonicalValidatorResult = 'PASS'

    $assignmentPath = Join-Path $staging 'assignment-record.json'
    [System.IO.File]::WriteAllText(
        $assignmentPath,
        ($Record | ConvertTo-Json -Depth 100),
        $utf8
    )
    $assignmentHash = (
        Get-FileHash -LiteralPath $assignmentPath -Algorithm SHA256
    ).Hash.ToLowerInvariant()
    $scopeHash = (
        Get-FileHash -LiteralPath $scopePath -Algorithm SHA256
    ).Hash.ToLowerInvariant()

    $completionReport = Copy-Record -Record $Record
    $completionReport.PSObject.Properties.Remove('focusedDelta')
    $completionReport | Add-Member -NotePropertyName repositoryArtifacts -NotePropertyValue @($ChangedPaths)
    $completionReport | Add-Member -NotePropertyName externalGovernanceChanges -NotePropertyValue @($externalPaths)
    $completionReport | Add-Member -NotePropertyName correctionPatchArtifact -NotePropertyValue 'correction-only.patch'
    $completionReport | Add-Member -NotePropertyName correctionPatchSha256 -NotePropertyValue $correctionPatchHash
    $completionReport | Add-Member -NotePropertyName currentDeltaArtifact -NotePropertyValue 'current-delta.patch'
    $completionReport | Add-Member -NotePropertyName currentDeltaSha256 -NotePropertyValue $currentDeltaHash
    $completionReport | Add-Member -NotePropertyName reviewStatus -NotePropertyValue 'AWAITING_FOCUSED_DELTA_REVIEW'
    $completionReport | Add-Member -NotePropertyName run007Status -NotePropertyValue 'CORRECTED_PENDING_DELTA'
    $completionReport | Add-Member -NotePropertyName queue -NotePropertyValue $queue
    $completionReport | Add-Member -NotePropertyName scopeInventorySha256 -NotePropertyValue $scopeHash
    $completionReport | Add-Member -NotePropertyName assignmentRecordSha256 -NotePropertyValue $assignmentHash
    $completionReport | Add-Member -NotePropertyName findingStatus -NotePropertyValue @(
        foreach ($finding in $findingRecords) {
            [ordered]@{
                id = $finding.id
                severity = $finding.severity
                status = 'CORRECTED_PENDING_DELTA'
                disposition = 'CORRECTED'
                correctionEvidence = $finding.correction
                affectedPaths = @($finding.affectedPaths)
                regressionTestIds = @($finding.regressionEvidenceIds)
                evidenceReferences = @($finding.evidence)
            }
        }
    )
    $completionReport | Add-Member -NotePropertyName materialCorrectionCycleCount -NotePropertyValue 1
    $completionReport | Add-Member -NotePropertyName validationExecutionCount -NotePropertyValue 1
    $completionReport | Add-Member -NotePropertyName infrastructureOrInvocationFailureCount -NotePropertyValue 0
    $completionReport | Add-Member -NotePropertyName observedWarningCount -NotePropertyValue 0
    $completionReport | Add-Member -NotePropertyName resolvedWarningCount -NotePropertyValue 0
    $completionReport | Add-Member -NotePropertyName openWarningCount -NotePropertyValue 0
    $completionReport | Add-Member -NotePropertyName warningCount -NotePropertyValue 0
    $completionReport | Add-Member -NotePropertyName failureCount -NotePropertyValue 0
    $completionReport | Add-Member -NotePropertyName zipFreeReadinessPassed -NotePropertyValue $true
    $completionReport | Add-Member -NotePropertyName packageGeneration -NotePropertyValue ([ordered]@{
        freshStaging = $true
        finalZipWriteCount = 1
        inPlaceRepairPerformed = $false
    })
    $completionReport | Add-Member -NotePropertyName nextAction -NotePropertyValue $nextAction
    [System.IO.File]::WriteAllText(
        (Join-Path $staging 'completion-report.json'),
        ($completionReport | ConvertTo-Json -Depth 100),
        $utf8
    )

    $correctionMatrix = [ordered]@{
        schemaVersion = 2
        mode = 'BUNDLED_CORRECTION'
        previousReviewPackage = $previousReviewPackage
        previousReviewSha256 = $previousReviewHash
        correctedFindingCount = 2
        repositoryCorrectionPaths = @($ChangedPaths)
        findings = @(
            foreach ($finding in $findingRecords) {
                [ordered]@{
                    id = $finding.id
                    severity = $finding.severity
                    previousStatus = 'PARTIALLY_CLOSED_CORRECTION_REQUIRED'
                    status = 'CORRECTED_PENDING_DELTA'
                    disposition = 'CORRECTED'
                    correction = $finding.correction
                    affectedPaths = @($finding.affectedPaths)
                    regressionTestIds = @($finding.regressionEvidenceIds)
                    evidenceReferences = @($finding.evidence)
                }
            }
        )
    }
    [System.IO.File]::WriteAllText(
        (Join-Path $staging 'finding-correction-matrix.json'),
        ($correctionMatrix | ConvertTo-Json -Depth 100),
        $utf8
    )
    $regressionMatrix = [ordered]@{
        schemaVersion = 2
        fixtureCount = 198
        fixtureResult = 'PASS'
        validatorPath = 'scripts/Test-GovernanceConsistency.ps1'
        findings = @(
            foreach ($correctionFinding in $correctionMatrix.findings) {
                [ordered]@{
                    id = $correctionFinding.id
                    severity = $correctionFinding.severity
                    previousStatus = $correctionFinding.previousStatus
                    status = $correctionFinding.status
                    disposition = $correctionFinding.disposition
                    correction = $correctionFinding.correction
                    affectedPaths = @($correctionFinding.affectedPaths)
                    evidenceReferences = @($correctionFinding.evidenceReferences)
                    regressionTests = @(
                        foreach ($testId in $correctionFinding.regressionTestIds) {
                            [ordered]@{
                                id = $testId
                                status = 'PASS'
                                validatorPath = 'scripts/Test-GovernanceConsistency.ps1'
                                evidence = 'Productive package validator fixture.'
                            }
                        }
                    )
                }
            }
        )
    }
    [System.IO.File]::WriteAllText(
        (Join-Path $staging 'finding-regression-matrix.json'),
        ($regressionMatrix | ConvertTo-Json -Depth 100),
        $utf8
    )
    $focusedRecord = [ordered]@{
        schemaVersion = 2
        mode = 'FOCUSED_INDEPENDENT_DELTA_REVIEW'
        previousReviewPackage = $previousReviewPackage
        previousReviewSha256 = $previousReviewHash
        correctionStartCommit = $Record.currentCommit
        correctionPatchArtifact = 'correction-only.patch'
        correctionPatchSha256 = $correctionPatchHash
        currentDeltaArtifact = 'current-delta.patch'
        currentDeltaSha256 = $currentDeltaHash
        correctionOnlyPaths = @($ChangedPaths)
        reviewedFindingIds = @($expectedFindingIds)
        directInterfacePaths = @($directInterfacePaths)
        regressionTestIds = @($allRegressionIds)
        allowedDeltaPaths = @($ChangedPaths)
        referenceOnlyPaths = @($referenceOnlyPaths)
        fullReviewRepeatAuthorized = $false
        commitPreparationApproved = $false
        commitAuthorized = $false
    }
    [System.IO.File]::WriteAllText(
        (Join-Path $staging 'focused-delta-review-record.json'),
        ($focusedRecord | ConvertTo-Json -Depth 100),
        $utf8
    )
    $trustedHashes = [ordered]@{
        schemaVersion = 1
        previousReviewPackage = $previousReviewPackage
        previousReviewSha256 = $previousReviewHash
        correctionPatchArtifact = 'correction-only.patch'
        correctionPatchSha256 = $correctionPatchHash
        currentDeltaArtifact = 'current-delta.patch'
        currentDeltaSha256 = $currentDeltaHash
    }
    [System.IO.File]::WriteAllText(
        (Join-Path $staging 'trusted-expected-hashes.json'),
        ($trustedHashes | ConvertTo-Json -Depth 20),
        $utf8
    )

    $reportContract = [ordered]@{
        schemaVersion = 1
        taskId = 'BL-333/BL-334'
        status = $handoffStatus
        executionMode = 'BUNDLED_CORRECTION'
        reviewStatus = 'AWAITING_FOCUSED_DELTA_REVIEW'
        findingIds = @($expectedFindingIds)
        closedFindings = @($closedFindingIds)
        findingStatus = 'CORRECTED_PENDING_DELTA'
        run007Status = 'CORRECTED_PENDING_DELTA'
        requiredReviewMode = 'FOCUSED_INDEPENDENT_DELTA_REVIEW'
        targetFindingCount = 2
        correctedFindingCount = 2
        pendingDeltaFindingCount = 2
        closedFindingCount = 3
        openFindingCount = 0
        nextAction = $nextAction
        repositoryPaths = @($ChangedPaths)
        referenceOnlyPaths = @($referenceOnlyPaths)
        directInterfacePaths = @($directInterfacePaths)
        regressionTestIds = @($allRegressionIds)
        externalPaths = @($externalPaths)
        correctionPatchArtifact = 'correction-only.patch'
        correctionPatchSha256 = $correctionPatchHash
        currentDeltaArtifact = 'current-delta.patch'
        currentDeltaSha256 = $currentDeltaHash
        classicReviewReady = $true
        commitPreparationApproved = $false
        commitAuthorized = $false
        queue = $queue
    }
    $narrativePaths = @(@($ChangedPaths) + @($externalPaths)) -join "`n"
    $reportText = @"
Status: $handoffStatus
Repository and external correction paths:
$narrativePaths
Commit Preparation remains blocked.
BEGIN_GOVERNANCE_REPORT_CONTRACT
$($reportContract | ConvertTo-Json -Depth 100)
END_GOVERNANCE_REPORT_CONTRACT
"@
    [System.IO.File]::WriteAllText((Join-Path $staging 'report.md'), $reportText, $utf8)
    $handoffContract = [ordered]@{
        schemaVersion = 1
        taskId = 'BL-333/BL-334'
        correctionMode = 'BUNDLED_CORRECTION'
        status = $handoffStatus
        classicReviewReady = $true
        targetFindings = @($expectedFindingIds)
        pendingFindings = @($expectedFindingIds)
        closedFindings = @($closedFindingIds)
        run007Status = 'CORRECTED_PENDING_DELTA'
        commitPreparationApproved = $false
        commitAuthorized = $false
        requiredReviewMode = 'FOCUSED_INDEPENDENT_DELTA_REVIEW'
        targetFindingCount = 2
        correctedFindingCount = 2
        pendingDeltaFindingCount = 2
        closedFindingCount = 3
        openFindingCount = 0
        nextAction = $nextAction
    }
    $handoffText = @"
# BL-333/BL-334 fourth bundled correction handoff

<!-- BEGIN GOVERNANCE-HANDOFF-STATUS -->
Status: $handoffStatus
CorrectionMode: BUNDLED_CORRECTION
TargetFindingCount: 2
CorrectedFindingCount: 2
PendingDeltaFindingCount: 2
ClosedFindingCount: 3
OpenFindingCount: 0
ClassicReviewReady: true
TargetFindings: $($expectedFindingIds -join ',')
PendingFindings: $($expectedFindingIds -join ',')
ClosedFindings: $($closedFindingIds -join ',')
Run007Status: CORRECTED_PENDING_DELTA
CommitPreparationApproved: false
CommitAuthorized: false
RequiredReviewMode: FOCUSED_INDEPENDENT_DELTA_REVIEW
NextAction: $nextAction
<!-- END GOVERNANCE-HANDOFF-STATUS -->

<!-- BEGIN GOVERNANCE-HANDOFF-CONTRACT -->
$($handoffContract | ConvertTo-Json -Depth 20)
<!-- END GOVERNANCE-HANDOFF-CONTRACT -->
"@
    [System.IO.File]::WriteAllText((Join-Path $staging 'HANDOFF.md'), $handoffText, $utf8)
    $readinessEvidence = [ordered]@{
        schemaVersion = 1
        status = $handoffStatus
        classicReviewReady = $true
        targetFindings = @($expectedFindingIds)
        closedFindings = @($closedFindingIds)
        commitPreparationApproved = $false
        commitAuthorized = $false
    }
    [System.IO.File]::WriteAllText(
        (Join-Path $staging 'readiness-evidence.json'),
        ($readinessEvidence | ConvertTo-Json -Depth 20),
        $utf8
    )
    [System.IO.File]::WriteAllText(
        (Join-Path $staging 'validation-summary.json'),
        "{`"status`":`"PASS`",`"fixtureCount`":198,`"handoffContractGatePassed`":true,`"visibleHandoffKeyGatePassed`":true,`"visibleHandoffParityGatePassed`":true,`"handoffMarkerCountGatePassed`":true,`"reservedControlLineGatePassed`":true,`"externalPathScopeMappingGatePassed`":true}`n",
        $utf8
    )

    $manifestRecords = @(
        Get-ChildItem -LiteralPath $staging -File -Recurse |
            ForEach-Object {
                $relative = [System.IO.Path]::GetRelativePath($staging, $_.FullName).Replace('\', '/')
                if ($relative -cne 'MANIFEST.sha256') {
                    $hash = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
                    [pscustomobject]@{
                        Path = $relative
                        Line = "$hash  $($_.Length)  $relative"
                    }
                }
            }
    )
    $manifestPaths = [string[]]@($manifestRecords | ForEach-Object Path)
    [array]::Sort($manifestPaths, [System.StringComparer]::Ordinal)
    $manifestLineByPath = @{}
    foreach ($manifestRecord in $manifestRecords) {
        $manifestLineByPath[$manifestRecord.Path] = $manifestRecord.Line
    }
    $manifestLines = @($manifestPaths | ForEach-Object { $manifestLineByPath[$_] })
    [System.IO.File]::WriteAllText(
        (Join-Path $staging 'MANIFEST.sha256'),
        (($manifestLines -join "`n") + "`n"),
        $utf8
    )

    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [System.IO.Compression.ZipFile]::CreateFromDirectory(
        $staging,
        $packagePath,
        [System.IO.Compression.CompressionLevel]::Optimal,
        $false
    )

    return [pscustomobject]@{
        PackagePath = $packagePath
        AssignmentPath = $assignmentPath
        CompletionReportPath = Join-Path $staging 'completion-report.json'
        ScopeInventoryPath = $scopePath
        ValidatorHash = $validatorHash
        CorrectionPatchHash = $correctionPatchHash
        CurrentDeltaHash = $currentDeltaHash
    }
}

function Update-FixtureManifest {
    param([Parameter(Mandatory)][string]$Staging)

    $utf8 = [System.Text.UTF8Encoding]::new($false)
    $manifestPath = Join-Path $Staging 'MANIFEST.sha256'
    $records = @(
        Get-ChildItem -LiteralPath $Staging -File -Recurse |
            Where-Object Name -cne 'MANIFEST.sha256' |
            ForEach-Object {
                $relative = [System.IO.Path]::GetRelativePath(
                    $Staging,
                    $_.FullName
                ).Replace('\', '/')
                [pscustomobject]@{
                    Path = $relative
                    Line = "$((Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant())  $($_.Length)  $relative"
                }
            }
    )
    $paths = [string[]]@($records | ForEach-Object Path)
    [array]::Sort($paths, [System.StringComparer]::Ordinal)
    $lineByPath = @{}
    foreach ($record in $records) {
        $lineByPath[$record.Path] = $record.Line
    }
    [System.IO.File]::WriteAllText(
        $manifestPath,
        ((@($paths | ForEach-Object { $lineByPath[$_] }) -join "`n") + "`n"),
        $utf8
    )
}

function Update-FixtureReportContract {
    param(
        [Parameter(Mandatory)][string]$ReportPath,
        [Parameter(Mandatory)][scriptblock]$Mutation
    )

    $utf8 = [System.Text.UTF8Encoding]::new($false)
    $reportText = Get-Content -LiteralPath $ReportPath -Raw -Encoding UTF8
    $contractMatch = [regex]::Match(
        $reportText,
        '(?ms)^BEGIN_GOVERNANCE_REPORT_CONTRACT\r?\n(?<json>\{.*?\})\r?\nEND_GOVERNANCE_REPORT_CONTRACT$'
    )
    if (-not $contractMatch.Success) {
        throw "Fixture report contract is missing: $ReportPath"
    }
    $contract = $contractMatch.Groups['json'].Value |
        ConvertFrom-Json -Depth 100 -DateKind String
    & $Mutation $contract
    $replacement = $contract | ConvertTo-Json -Depth 100
    $updatedText = $reportText.Remove(
        $contractMatch.Groups['json'].Index,
        $contractMatch.Groups['json'].Length
    ).Insert($contractMatch.Groups['json'].Index, $replacement)
    [System.IO.File]::WriteAllText($ReportPath, $updatedText, $utf8)
}

function Update-FixtureHandoffContract {
    param(
        [Parameter(Mandatory)][string]$HandoffPath,
        [Parameter(Mandatory)][scriptblock]$Mutation
    )

    $utf8 = [System.Text.UTF8Encoding]::new($false)
    $handoffText = Get-Content -LiteralPath $HandoffPath -Raw -Encoding UTF8
    $contractMatch = [regex]::Match(
        $handoffText,
        '(?ms)^<!-- BEGIN GOVERNANCE-HANDOFF-CONTRACT -->\r?\n(?<json>\{.*?\})\r?\n<!-- END GOVERNANCE-HANDOFF-CONTRACT -->\r?$'
    )
    if (-not $contractMatch.Success) {
        throw "Fixture handoff contract is missing: $HandoffPath"
    }
    $contract = $contractMatch.Groups['json'].Value |
        ConvertFrom-Json -Depth 100 -DateKind String
    & $Mutation $contract
    $replacement = $contract | ConvertTo-Json -Depth 100
    $updatedText = $handoffText.Remove(
        $contractMatch.Groups['json'].Index,
        $contractMatch.Groups['json'].Length
    ).Insert($contractMatch.Groups['json'].Index, $replacement)
    [System.IO.File]::WriteAllText($HandoffPath, $updatedText, $utf8)
}

function New-MutatedFixturePackage {
    param(
        [Parameter(Mandatory)][string]$Root,
        [Parameter(Mandatory)][string]$SourcePackage,
        [Parameter(Mandatory)]
        [ValidateSet(
            'MISSING_FILE',
            'WRONG_HASH',
            'WRONG_SIZE',
            'SCOPE_INVENTORY_WRONG_SCOPE',
            'WRONG_SCOPE',
            'DUPLICATE_SCOPE',
            'PATH_SCOPE_SWAP',
            'EXTRA_SCOPE',
            'PATH_CASE_DUPLICATE',
            'HANDOFF_STALE_STATUS',
            'HANDOFF_TARGET_MISMATCH',
            'HANDOFF_CLOSED_MISMATCH',
            'HANDOFF_RUN_MISMATCH',
            'HANDOFF_COMMIT_PREP_MISMATCH',
            'HANDOFF_COMMIT_AUTH_MISMATCH',
            'HANDOFF_REVIEW_MODE_MISMATCH',
            'HANDOFF_NEXT_ACTION_MISMATCH',
            'HANDOFF_NARRATIVE_CONTRACT_MISMATCH',
            'HANDOFF_MISSING_BLOCK',
            'HANDOFF_DUPLICATE_BLOCK',
            'HANDOFF_INVALID_JSON',
            'HANDOFF_VISIBLE_PENDING_MISMATCH',
            'HANDOFF_VISIBLE_COMMIT_PREP_MISMATCH',
            'HANDOFF_VISIBLE_COMMIT_AUTH_MISMATCH',
            'HANDOFF_VISIBLE_TARGET_COUNT_MISMATCH',
            'HANDOFF_VISIBLE_CORRECTED_COUNT_MISMATCH',
            'HANDOFF_VISIBLE_PENDING_COUNT_MISMATCH',
            'HANDOFF_VISIBLE_CLOSED_COUNT_MISMATCH',
            'HANDOFF_VISIBLE_OPEN_COUNT_MISMATCH',
            'HANDOFF_VISIBLE_DUPLICATE_KEY_SAME',
            'HANDOFF_VISIBLE_DUPLICATE_KEY_CONFLICT',
            'HANDOFF_VISIBLE_UNKNOWN_KEY',
            'HANDOFF_RESERVED_KEY_OUTSIDE',
            'HANDOFF_EXTRA_STATUS_BEGIN',
            'HANDOFF_EXTRA_STATUS_END',
            'HANDOFF_EXTRA_CONTRACT_BEGIN',
            'HANDOFF_EXTRA_CONTRACT_END',
            'HANDOFF_REVERSED_STATUS_MARKERS',
            'HANDOFF_REVERSED_CONTRACT_MARKERS',
            'MISSING_ASSIGNMENT',
            'MISSING_REPORT',
            'MISSING_EXTERNAL',
            'EXTRA_MEMBER',
            'PATCH_BYTES_STALE_MANIFEST',
            'PATCH_BYTES_REMANIFESTED',
            'PATCH_BYTES_RECORDS_REHASHED',
            'CURRENT_DELTA_BYTES_REMANIFESTED',
            'PATCHES_SWAPPED',
            'TRUSTED_WRONG_PATCH_HASH',
            'FOCUSED_WRONG_PATCH_HASH',
            'FOCUSED_WRONG_CURRENT_HASH',
            'FOCUSED_MISSING_FINDING',
            'FOCUSED_EXTRA_PATH',
            'FOCUSED_MISSING_INTERFACE',
            'FOCUSED_WRONG_REFERENCE',
            'CORRECTION_MISSING_FINDING',
            'CORRECTION_EXTRA_FINDING',
            'CORRECTION_DUPLICATE_FINDING',
            'CORRECTION_MISSING_PATH',
            'CORRECTION_FOREIGN_PATH',
            'CORRECTION_MISSING_TEST',
            'REGRESSION_UNKNOWN_FINDING',
            'REGRESSION_UNKNOWN_TEST',
            'COMPLETION_MISSING_FINDING',
            'COMPLETION_WRONG_STATUS',
            'COMPLETION_WRONG_SEVERITY',
            'COMPLETION_EMPTY_FINDINGS',
            'COMPLETION_WRONG_PATH',
            'COMPLETION_FOREIGN_TEST',
            'COMPLETION_EMPTY_EVIDENCE',
            'REPORT_MISSING_REPOSITORY_PATH',
            'REPORT_EXTRA_REPOSITORY_PATH',
            'REPORT_MISSING_EXTERNAL_PATH',
            'REPORT_EXTRA_EXTERNAL_PATH',
            'REPORT_STALE_STATUS',
            'REPORT_CONFLICTING_QUEUE',
            'REPORT_MISSING_BLOCK',
            'REPORT_DOUBLE_BLOCK',
            'REPORT_DUMMY_JSON',
            'LOCAL_REGISTER_STALE_STATUS'
        )]
        [string]$Mutation
    )

    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $utf8 = [System.Text.UTF8Encoding]::new($false)
    $slug = $Mutation.ToLowerInvariant()
    $staging = Join-Path $Root ('mutated-' + $slug)
    $packagePath = Join-Path $Root ('mutated-' + $slug + '.zip')
    $externalAssignmentPath = Join-Path $Root ('mutated-' + $slug + '-assignment.json')
    [System.IO.Compression.ZipFile]::ExtractToDirectory($SourcePackage, $staging)

    $assignmentPath = Join-Path $staging 'assignment-record.json'
    $completionPath = Join-Path $staging 'completion-report.json'
    $scopePath = Join-Path $staging 'scope-inventory.json'
    $manifestPath = Join-Path $staging 'MANIFEST.sha256'
    $reportPath = Join-Path $staging 'report.md'
    $handoffPath = Join-Path $staging 'HANDOFF.md'
    $externalManifestPath = Join-Path $staging 'external-governance-manifest.json'
    $correctionPatchPath = Join-Path $staging 'correction-only.patch'
    $currentDeltaPath = Join-Path $staging 'current-delta.patch'
    $focusedPath = Join-Path $staging 'focused-delta-review-record.json'
    $correctionMatrixPath = Join-Path $staging 'finding-correction-matrix.json'
    $regressionMatrixPath = Join-Path $staging 'finding-regression-matrix.json'
    $trustedHashesPath = Join-Path $staging 'trusted-expected-hashes.json'

    $assignment = Get-Content -LiteralPath $assignmentPath -Raw -Encoding UTF8 |
        ConvertFrom-Json -Depth 100 -DateKind String
    $assignment.handoff.package = $packagePath
    [System.IO.File]::WriteAllText(
        $assignmentPath,
        ($assignment | ConvertTo-Json -Depth 100),
        $utf8
    )
    Copy-Item -LiteralPath $assignmentPath -Destination $externalAssignmentPath
    $completion = Get-Content -LiteralPath $completionPath -Raw -Encoding UTF8 |
        ConvertFrom-Json -Depth 100 -DateKind String
    $completion.assignmentRecordSha256 = (
        Get-FileHash -LiteralPath $assignmentPath -Algorithm SHA256
    ).Hash.ToLowerInvariant()
    [System.IO.File]::WriteAllText(
        $completionPath,
        ($completion | ConvertTo-Json -Depth 100),
        $utf8
    )
    Update-FixtureManifest -Staging $staging

    $refreshManifest = $true
    switch ($Mutation) {
        'MISSING_FILE' {
            Remove-Item -LiteralPath (Join-Path $staging 'validation-summary.json') -Force
            $refreshManifest = $false
        }
        'WRONG_HASH' {
            $manifest = Get-Content -LiteralPath $manifestPath -Raw -Encoding UTF8
            $manifest = [regex]::Replace($manifest, '^[0-9a-f]{64}', ('0' * 64), 1)
            [System.IO.File]::WriteAllText($manifestPath, $manifest, $utf8)
            $refreshManifest = $false
        }
        'WRONG_SIZE' {
            $manifest = Get-Content -LiteralPath $manifestPath -Raw -Encoding UTF8
            $manifest = [regex]::Replace(
                $manifest,
                '^(?<hash>[0-9a-f]{64})  (?<size>[0-9]+)',
                '${hash}  999999',
                1
            )
            [System.IO.File]::WriteAllText($manifestPath, $manifest, $utf8)
            $refreshManifest = $false
        }
        'SCOPE_INVENTORY_WRONG_SCOPE' {
            $scope = [ordered]@{
                schemaVersion = 1
                entries = @([ordered]@{ path = 'README.md' })
                referenceOnlyPaths = @()
            }
            [System.IO.File]::WriteAllText(
                $scopePath,
                ($scope | ConvertTo-Json -Depth 20),
                $utf8
            )
            $completion = Get-Content -LiteralPath $completionPath -Raw -Encoding UTF8 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $completion.scopeInventorySha256 = (
                Get-FileHash -LiteralPath $scopePath -Algorithm SHA256
            ).Hash.ToLowerInvariant()
            [System.IO.File]::WriteAllText(
                $completionPath,
                ($completion | ConvertTo-Json -Depth 100),
                $utf8
            )
        }
        'WRONG_SCOPE' {
            $external = Get-Content -LiteralPath $externalManifestPath -Raw -Encoding UTF8 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $external.changes[0].scope = 'WRONG_SCOPE'
            [System.IO.File]::WriteAllText(
                $externalManifestPath,
                ($external | ConvertTo-Json -Depth 100),
                $utf8
            )
        }
        'DUPLICATE_SCOPE' {
            $external = Get-Content -LiteralPath $externalManifestPath -Raw -Encoding UTF8 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $external.changes[1].scope = [string]$external.changes[0].scope
            [System.IO.File]::WriteAllText(
                $externalManifestPath,
                ($external | ConvertTo-Json -Depth 100),
                $utf8
            )
        }
        'PATH_SCOPE_SWAP' {
            $external = Get-Content -LiteralPath $externalManifestPath -Raw -Encoding UTF8 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $firstScope = [string]$external.changes[0].scope
            $external.changes[0].scope = [string]$external.changes[1].scope
            $external.changes[1].scope = $firstScope
            [System.IO.File]::WriteAllText(
                $externalManifestPath,
                ($external | ConvertTo-Json -Depth 100),
                $utf8
            )
        }
        'EXTRA_SCOPE' {
            $external = Get-Content -LiteralPath $externalManifestPath -Raw -Encoding UTF8 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $source = $external.changes[0] | ConvertTo-Json -Depth 100 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $source.activePath = 'C:\Voxtronic\EXTRA-GOVERNANCE.md'
            $source.backupPath = 'C:\Voxtronic\EXTRA-GOVERNANCE.md.backup'
            $source.scope = 'EXTRA_SCOPE'
            $source.beforePayload = 'external/extra-scope/before.txt'
            $source.afterPayload = 'external/extra-scope/after.txt'
            $source.diffPayload = 'external/extra-scope/change.patch'
            [void][System.IO.Directory]::CreateDirectory((Join-Path $staging 'external/extra-scope'))
            Copy-Item -LiteralPath (Join-Path $staging 'external/global-codex-agents/before.txt') `
                -Destination (Join-Path $staging $source.beforePayload)
            Copy-Item -LiteralPath (Join-Path $staging 'external/global-codex-agents/after.txt') `
                -Destination (Join-Path $staging $source.afterPayload)
            Copy-Item -LiteralPath (Join-Path $staging 'external/global-codex-agents/change.patch') `
                -Destination (Join-Path $staging $source.diffPayload)
            $external.changes = @($external.changes) + @($source)
            [System.IO.File]::WriteAllText(
                $externalManifestPath,
                ($external | ConvertTo-Json -Depth 100),
                $utf8
            )
        }
        'PATH_CASE_DUPLICATE' {
            $external = Get-Content -LiteralPath $externalManifestPath -Raw -Encoding UTF8 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $external.changes[4].activePath = 'C:\USERS\THOMASW\.CODEX\AGENTS.MD'
            [System.IO.File]::WriteAllText(
                $externalManifestPath,
                ($external | ConvertTo-Json -Depth 100),
                $utf8
            )
        }
        'HANDOFF_STALE_STATUS' {
            Update-FixtureHandoffContract -HandoffPath $handoffPath -Mutation {
                param($contract)
                $contract.status = 'SECOND_BUNDLED_CORRECTION_COMPLETE_AWAITING_FOCUSED_DELTA_REVIEW'
            }
        }
        'HANDOFF_TARGET_MISMATCH' {
            Update-FixtureHandoffContract -HandoffPath $handoffPath -Mutation {
                param($contract)
                $contract.targetFindings = @('BL333-BL334-REV-013')
            }
        }
        'HANDOFF_CLOSED_MISMATCH' {
            Update-FixtureHandoffContract -HandoffPath $handoffPath -Mutation {
                param($contract)
                $contract.closedFindings = @('BL333-BL334-REV-007')
            }
        }
        'HANDOFF_RUN_MISMATCH' {
            Update-FixtureHandoffContract -HandoffPath $handoffPath -Mutation {
                param($contract)
                $contract.run007Status = 'CLOSED'
            }
        }
        'HANDOFF_COMMIT_PREP_MISMATCH' {
            Update-FixtureHandoffContract -HandoffPath $handoffPath -Mutation {
                param($contract)
                $contract.commitPreparationApproved = $true
            }
        }
        'HANDOFF_COMMIT_AUTH_MISMATCH' {
            Update-FixtureHandoffContract -HandoffPath $handoffPath -Mutation {
                param($contract)
                $contract.commitAuthorized = $true
            }
        }
        'HANDOFF_REVIEW_MODE_MISMATCH' {
            Update-FixtureHandoffContract -HandoffPath $handoffPath -Mutation {
                param($contract)
                $contract.requiredReviewMode = 'INDEPENDENT_REVIEW'
            }
        }
        'HANDOFF_NEXT_ACTION_MISMATCH' {
            Update-FixtureHandoffContract -HandoffPath $handoffPath -Mutation {
                param($contract)
                $contract.nextAction = 'Enter Commit Preparation.'
            }
        }
        'HANDOFF_NARRATIVE_CONTRACT_MISMATCH' {
            $handoffText = Get-Content -LiteralPath $handoffPath -Raw -Encoding UTF8
            $handoffText = $handoffText.Replace(
                'Status: FOURTH_BUNDLED_CORRECTION_COMPLETE_AWAITING_FOCUSED_DELTA_REVIEW',
                'Status: SECOND_BUNDLED_CORRECTION_COMPLETE_AWAITING_FOCUSED_DELTA_REVIEW'
            )
            [System.IO.File]::WriteAllText($handoffPath, $handoffText, $utf8)
        }
        'HANDOFF_MISSING_BLOCK' {
            $handoffText = Get-Content -LiteralPath $handoffPath -Raw -Encoding UTF8
            $handoffText = [regex]::Replace(
                $handoffText,
                '(?ms)^<!-- BEGIN GOVERNANCE-HANDOFF-CONTRACT -->\r?\n.*?\r?\n<!-- END GOVERNANCE-HANDOFF-CONTRACT -->\r?$',
                ''
            )
            [System.IO.File]::WriteAllText($handoffPath, $handoffText, $utf8)
        }
        'HANDOFF_DUPLICATE_BLOCK' {
            $handoffText = Get-Content -LiteralPath $handoffPath -Raw -Encoding UTF8
            $block = [regex]::Match(
                $handoffText,
                '(?ms)^<!-- BEGIN GOVERNANCE-HANDOFF-CONTRACT -->\r?\n.*?\r?\n<!-- END GOVERNANCE-HANDOFF-CONTRACT -->\r?$'
            ).Value
            [System.IO.File]::WriteAllText(
                $handoffPath,
                $handoffText + "`n" + $block,
                $utf8
            )
        }
        'HANDOFF_INVALID_JSON' {
            $handoffText = Get-Content -LiteralPath $handoffPath -Raw -Encoding UTF8
            $handoffText = [regex]::Replace(
                $handoffText,
                '(?ms)(^<!-- BEGIN GOVERNANCE-HANDOFF-CONTRACT -->\r?\n)\{.*?\}(\r?\n<!-- END GOVERNANCE-HANDOFF-CONTRACT -->\r?$)',
                '${1}{invalid json}${2}'
            )
            [System.IO.File]::WriteAllText($handoffPath, $handoffText, $utf8)
        }
        'HANDOFF_VISIBLE_PENDING_MISMATCH' {
            $handoffText = Get-Content -LiteralPath $handoffPath -Raw -Encoding UTF8
            $handoffText = $handoffText.Replace(
                'PendingFindings: BL333-BL334-REV-013,BL333-BL334-REV-015',
                'PendingFindings: BL333-BL334-REV-013'
            )
            [System.IO.File]::WriteAllText($handoffPath, $handoffText, $utf8)
        }
        'HANDOFF_VISIBLE_COMMIT_PREP_MISMATCH' {
            $handoffText = Get-Content -LiteralPath $handoffPath -Raw -Encoding UTF8
            $handoffText = $handoffText.Replace(
                'CommitPreparationApproved: false',
                'CommitPreparationApproved: true'
            )
            [System.IO.File]::WriteAllText($handoffPath, $handoffText, $utf8)
        }
        'HANDOFF_VISIBLE_COMMIT_AUTH_MISMATCH' {
            $handoffText = Get-Content -LiteralPath $handoffPath -Raw -Encoding UTF8
            $handoffText = $handoffText.Replace(
                'CommitAuthorized: false',
                'CommitAuthorized: true'
            )
            [System.IO.File]::WriteAllText($handoffPath, $handoffText, $utf8)
        }
        'HANDOFF_VISIBLE_TARGET_COUNT_MISMATCH' {
            $handoffText = Get-Content -LiteralPath $handoffPath -Raw -Encoding UTF8
            $handoffText = $handoffText.Replace(
                'TargetFindingCount: 2',
                'TargetFindingCount: 1'
            )
            [System.IO.File]::WriteAllText($handoffPath, $handoffText, $utf8)
        }
        'HANDOFF_VISIBLE_CORRECTED_COUNT_MISMATCH' {
            $handoffText = Get-Content -LiteralPath $handoffPath -Raw -Encoding UTF8
            $handoffText = $handoffText.Replace(
                'CorrectedFindingCount: 2',
                'CorrectedFindingCount: 1'
            )
            [System.IO.File]::WriteAllText($handoffPath, $handoffText, $utf8)
        }
        'HANDOFF_VISIBLE_PENDING_COUNT_MISMATCH' {
            $handoffText = Get-Content -LiteralPath $handoffPath -Raw -Encoding UTF8
            $handoffText = $handoffText.Replace(
                'PendingDeltaFindingCount: 2',
                'PendingDeltaFindingCount: 1'
            )
            [System.IO.File]::WriteAllText($handoffPath, $handoffText, $utf8)
        }
        'HANDOFF_VISIBLE_CLOSED_COUNT_MISMATCH' {
            $handoffText = Get-Content -LiteralPath $handoffPath -Raw -Encoding UTF8
            $handoffText = $handoffText.Replace(
                'ClosedFindingCount: 3',
                'ClosedFindingCount: 2'
            )
            [System.IO.File]::WriteAllText($handoffPath, $handoffText, $utf8)
        }
        'HANDOFF_VISIBLE_OPEN_COUNT_MISMATCH' {
            $handoffText = Get-Content -LiteralPath $handoffPath -Raw -Encoding UTF8
            $handoffText = $handoffText.Replace(
                'OpenFindingCount: 0',
                'OpenFindingCount: 1'
            )
            [System.IO.File]::WriteAllText($handoffPath, $handoffText, $utf8)
        }
        'HANDOFF_VISIBLE_DUPLICATE_KEY_SAME' {
            $handoffText = Get-Content -LiteralPath $handoffPath -Raw -Encoding UTF8
            $handoffText = $handoffText.Replace(
                '<!-- END GOVERNANCE-HANDOFF-STATUS -->',
                "CommitAuthorized: false`n<!-- END GOVERNANCE-HANDOFF-STATUS -->"
            )
            [System.IO.File]::WriteAllText($handoffPath, $handoffText, $utf8)
        }
        'HANDOFF_VISIBLE_DUPLICATE_KEY_CONFLICT' {
            $handoffText = Get-Content -LiteralPath $handoffPath -Raw -Encoding UTF8
            $handoffText = $handoffText.Replace(
                '<!-- END GOVERNANCE-HANDOFF-STATUS -->',
                "CommitAuthorized: true`n<!-- END GOVERNANCE-HANDOFF-STATUS -->"
            )
            [System.IO.File]::WriteAllText($handoffPath, $handoffText, $utf8)
        }
        'HANDOFF_VISIBLE_UNKNOWN_KEY' {
            $handoffText = Get-Content -LiteralPath $handoffPath -Raw -Encoding UTF8
            $handoffText = $handoffText.Replace(
                '<!-- END GOVERNANCE-HANDOFF-STATUS -->',
                "DeploymentAuthorized: true`n<!-- END GOVERNANCE-HANDOFF-STATUS -->"
            )
            [System.IO.File]::WriteAllText($handoffPath, $handoffText, $utf8)
        }
        'HANDOFF_RESERVED_KEY_OUTSIDE' {
            $handoffText = Get-Content -LiteralPath $handoffPath -Raw -Encoding UTF8
            $handoffText = $handoffText.Replace(
                '<!-- BEGIN GOVERNANCE-HANDOFF-CONTRACT -->',
                "Status: PASS`n<!-- BEGIN GOVERNANCE-HANDOFF-CONTRACT -->"
            )
            [System.IO.File]::WriteAllText($handoffPath, $handoffText, $utf8)
        }
        'HANDOFF_EXTRA_STATUS_BEGIN' {
            $handoffText = Get-Content -LiteralPath $handoffPath -Raw -Encoding UTF8
            [System.IO.File]::WriteAllText(
                $handoffPath,
                "<!-- BEGIN GOVERNANCE-HANDOFF-STATUS -->`n" + $handoffText,
                $utf8
            )
        }
        'HANDOFF_EXTRA_STATUS_END' {
            $handoffText = Get-Content -LiteralPath $handoffPath -Raw -Encoding UTF8
            [System.IO.File]::WriteAllText(
                $handoffPath,
                "<!-- END GOVERNANCE-HANDOFF-STATUS -->`n" + $handoffText,
                $utf8
            )
        }
        'HANDOFF_EXTRA_CONTRACT_BEGIN' {
            $handoffText = Get-Content -LiteralPath $handoffPath -Raw -Encoding UTF8
            [System.IO.File]::WriteAllText(
                $handoffPath,
                "<!-- BEGIN GOVERNANCE-HANDOFF-CONTRACT -->`n" + $handoffText,
                $utf8
            )
        }
        'HANDOFF_EXTRA_CONTRACT_END' {
            $handoffText = Get-Content -LiteralPath $handoffPath -Raw -Encoding UTF8
            [System.IO.File]::WriteAllText(
                $handoffPath,
                "<!-- END GOVERNANCE-HANDOFF-CONTRACT -->`n" + $handoffText,
                $utf8
            )
        }
        'HANDOFF_REVERSED_STATUS_MARKERS' {
            $handoffText = Get-Content -LiteralPath $handoffPath -Raw -Encoding UTF8
            $handoffText = $handoffText.Replace(
                '<!-- BEGIN GOVERNANCE-HANDOFF-STATUS -->',
                '<!-- TEMP GOVERNANCE-HANDOFF-STATUS -->'
            ).Replace(
                '<!-- END GOVERNANCE-HANDOFF-STATUS -->',
                '<!-- BEGIN GOVERNANCE-HANDOFF-STATUS -->'
            ).Replace(
                '<!-- TEMP GOVERNANCE-HANDOFF-STATUS -->',
                '<!-- END GOVERNANCE-HANDOFF-STATUS -->'
            )
            [System.IO.File]::WriteAllText($handoffPath, $handoffText, $utf8)
        }
        'HANDOFF_REVERSED_CONTRACT_MARKERS' {
            $handoffText = Get-Content -LiteralPath $handoffPath -Raw -Encoding UTF8
            $handoffText = $handoffText.Replace(
                '<!-- BEGIN GOVERNANCE-HANDOFF-CONTRACT -->',
                '<!-- TEMP GOVERNANCE-HANDOFF-CONTRACT -->'
            ).Replace(
                '<!-- END GOVERNANCE-HANDOFF-CONTRACT -->',
                '<!-- BEGIN GOVERNANCE-HANDOFF-CONTRACT -->'
            ).Replace(
                '<!-- TEMP GOVERNANCE-HANDOFF-CONTRACT -->',
                '<!-- END GOVERNANCE-HANDOFF-CONTRACT -->'
            )
            [System.IO.File]::WriteAllText($handoffPath, $handoffText, $utf8)
        }
        'MISSING_ASSIGNMENT' {
            Remove-Item -LiteralPath $assignmentPath -Force
            $refreshManifest = $false
        }
        'MISSING_REPORT' {
            Remove-Item -LiteralPath $completionPath -Force
            $refreshManifest = $false
        }
        'MISSING_EXTERNAL' {
            Remove-Item -LiteralPath (
                Join-Path $staging 'external/local-work-register/after.txt'
            ) -Force
            $refreshManifest = $false
        }
        'EXTRA_MEMBER' {
            [System.IO.File]::WriteAllText(
                (Join-Path $staging 'unexpected.txt'),
                "unexpected`n",
                $utf8
            )
            $refreshManifest = $false
        }
        'PATCH_BYTES_STALE_MANIFEST' {
            [System.IO.File]::AppendAllText(
                $correctionPatchPath,
                "# stale manifest mutation`n",
                $utf8
            )
            $refreshManifest = $false
        }
        'PATCH_BYTES_REMANIFESTED' {
            [System.IO.File]::AppendAllText(
                $correctionPatchPath,
                "# re-manifested mutation`n",
                $utf8
            )
        }
        'PATCH_BYTES_RECORDS_REHASHED' {
            [System.IO.File]::AppendAllText(
                $correctionPatchPath,
                "# rehashed self-report mutation`n",
                $utf8
            )
            $newPatchHash = (
                Get-FileHash -LiteralPath $correctionPatchPath -Algorithm SHA256
            ).Hash.ToLowerInvariant()
            $assignment = Get-Content -LiteralPath $assignmentPath -Raw -Encoding UTF8 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $assignment.focusedDelta.correctionPatchSha256 = $newPatchHash
            [System.IO.File]::WriteAllText(
                $assignmentPath,
                ($assignment | ConvertTo-Json -Depth 100),
                $utf8
            )
            Copy-Item -LiteralPath $assignmentPath -Destination $externalAssignmentPath -Force
            $focused = Get-Content -LiteralPath $focusedPath -Raw -Encoding UTF8 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $focused.correctionPatchSha256 = $newPatchHash
            [System.IO.File]::WriteAllText($focusedPath, ($focused | ConvertTo-Json -Depth 100), $utf8)
            $trusted = Get-Content -LiteralPath $trustedHashesPath -Raw -Encoding UTF8 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $trusted.correctionPatchSha256 = $newPatchHash
            [System.IO.File]::WriteAllText(
                $trustedHashesPath,
                ($trusted | ConvertTo-Json -Depth 100),
                $utf8
            )
            $completion = Get-Content -LiteralPath $completionPath -Raw -Encoding UTF8 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $completion.correctionPatchSha256 = $newPatchHash
            $completion.assignmentRecordSha256 = (
                Get-FileHash -LiteralPath $assignmentPath -Algorithm SHA256
            ).Hash.ToLowerInvariant()
            [System.IO.File]::WriteAllText(
                $completionPath,
                ($completion | ConvertTo-Json -Depth 100),
                $utf8
            )
            Update-FixtureReportContract -ReportPath $reportPath -Mutation {
                param($contract)
                $contract.correctionPatchSha256 = $newPatchHash
            }
        }
        'CURRENT_DELTA_BYTES_REMANIFESTED' {
            [System.IO.File]::AppendAllText(
                $currentDeltaPath,
                "# re-manifested current delta mutation`n",
                $utf8
            )
        }
        'PATCHES_SWAPPED' {
            $correctionBytes = [System.IO.File]::ReadAllBytes($correctionPatchPath)
            $currentBytes = [System.IO.File]::ReadAllBytes($currentDeltaPath)
            [System.IO.File]::WriteAllBytes($correctionPatchPath, $currentBytes)
            [System.IO.File]::WriteAllBytes($currentDeltaPath, $correctionBytes)
        }
        'TRUSTED_WRONG_PATCH_HASH' {
            $trusted = Get-Content -LiteralPath $trustedHashesPath -Raw -Encoding UTF8 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $trusted.correctionPatchSha256 = 'd' * 64
            [System.IO.File]::WriteAllText(
                $trustedHashesPath,
                ($trusted | ConvertTo-Json -Depth 100),
                $utf8
            )
        }
        'FOCUSED_WRONG_PATCH_HASH' {
            $focused = Get-Content -LiteralPath $focusedPath -Raw -Encoding UTF8 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $focused.correctionPatchSha256 = 'd' * 64
            [System.IO.File]::WriteAllText($focusedPath, ($focused | ConvertTo-Json -Depth 100), $utf8)
        }
        'FOCUSED_WRONG_CURRENT_HASH' {
            $focused = Get-Content -LiteralPath $focusedPath -Raw -Encoding UTF8 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $focused.currentDeltaSha256 = 'd' * 64
            [System.IO.File]::WriteAllText($focusedPath, ($focused | ConvertTo-Json -Depth 100), $utf8)
        }
        'FOCUSED_MISSING_FINDING' {
            $focused = Get-Content -LiteralPath $focusedPath -Raw -Encoding UTF8 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $focused.reviewedFindingIds = @($focused.reviewedFindingIds | Select-Object -Skip 1)
            [System.IO.File]::WriteAllText($focusedPath, ($focused | ConvertTo-Json -Depth 100), $utf8)
        }
        'FOCUSED_EXTRA_PATH' {
            $focused = Get-Content -LiteralPath $focusedPath -Raw -Encoding UTF8 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $focused.correctionOnlyPaths = @($focused.correctionOnlyPaths) + @('docs/foreign.md')
            [System.IO.File]::WriteAllText($focusedPath, ($focused | ConvertTo-Json -Depth 100), $utf8)
        }
        'FOCUSED_MISSING_INTERFACE' {
            $focused = Get-Content -LiteralPath $focusedPath -Raw -Encoding UTF8 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $focused.directInterfacePaths = @()
            [System.IO.File]::WriteAllText($focusedPath, ($focused | ConvertTo-Json -Depth 100), $utf8)
        }
        'FOCUSED_WRONG_REFERENCE' {
            $focused = Get-Content -LiteralPath $focusedPath -Raw -Encoding UTF8 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $focused.referenceOnlyPaths = @('docs/foreign-reference.md')
            [System.IO.File]::WriteAllText($focusedPath, ($focused | ConvertTo-Json -Depth 100), $utf8)
        }
        'CORRECTION_MISSING_FINDING' {
            $matrix = Get-Content -LiteralPath $correctionMatrixPath -Raw -Encoding UTF8 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $matrix.findings = @($matrix.findings | Select-Object -Skip 1)
            [System.IO.File]::WriteAllText($correctionMatrixPath, ($matrix | ConvertTo-Json -Depth 100), $utf8)
        }
        'CORRECTION_EXTRA_FINDING' {
            $matrix = Get-Content -LiteralPath $correctionMatrixPath -Raw -Encoding UTF8 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $extraFinding = $matrix.findings[0] | ConvertTo-Json -Depth 100 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $extraFinding.id = 'BL333-BL334-REV-999'
            $matrix.findings = @($matrix.findings) + @($extraFinding)
            [System.IO.File]::WriteAllText($correctionMatrixPath, ($matrix | ConvertTo-Json -Depth 100), $utf8)
        }
        'CORRECTION_DUPLICATE_FINDING' {
            $matrix = Get-Content -LiteralPath $correctionMatrixPath -Raw -Encoding UTF8 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $matrix.findings = @($matrix.findings) + @($matrix.findings[0])
            [System.IO.File]::WriteAllText($correctionMatrixPath, ($matrix | ConvertTo-Json -Depth 100), $utf8)
        }
        'CORRECTION_MISSING_PATH' {
            $matrix = Get-Content -LiteralPath $correctionMatrixPath -Raw -Encoding UTF8 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $matrix.findings[0].affectedPaths = @($matrix.findings[0].affectedPaths | Select-Object -Skip 1)
            [System.IO.File]::WriteAllText($correctionMatrixPath, ($matrix | ConvertTo-Json -Depth 100), $utf8)
        }
        'CORRECTION_FOREIGN_PATH' {
            $matrix = Get-Content -LiteralPath $correctionMatrixPath -Raw -Encoding UTF8 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $matrix.repositoryCorrectionPaths = @($matrix.repositoryCorrectionPaths) + @('docs/foreign.md')
            [System.IO.File]::WriteAllText($correctionMatrixPath, ($matrix | ConvertTo-Json -Depth 100), $utf8)
        }
        'CORRECTION_MISSING_TEST' {
            $matrix = Get-Content -LiteralPath $correctionMatrixPath -Raw -Encoding UTF8 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $matrix.findings[0].regressionTestIds = @()
            [System.IO.File]::WriteAllText($correctionMatrixPath, ($matrix | ConvertTo-Json -Depth 100), $utf8)
        }
        'REGRESSION_UNKNOWN_FINDING' {
            $matrix = Get-Content -LiteralPath $regressionMatrixPath -Raw -Encoding UTF8 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $unknown = $matrix.findings[0] | ConvertTo-Json -Depth 100 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $unknown.id = 'BL333-BL334-REV-999'
            $matrix.findings = @($matrix.findings) + @($unknown)
            [System.IO.File]::WriteAllText($regressionMatrixPath, ($matrix | ConvertTo-Json -Depth 100), $utf8)
        }
        'REGRESSION_UNKNOWN_TEST' {
            $matrix = Get-Content -LiteralPath $regressionMatrixPath -Raw -Encoding UTF8 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $unknown = $matrix.findings[0].regressionTests[0] | ConvertTo-Json -Depth 100 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $unknown.id = 'REG-UNKNOWN'
            $matrix.findings[0].regressionTests = @($matrix.findings[0].regressionTests) + @($unknown)
            [System.IO.File]::WriteAllText($regressionMatrixPath, ($matrix | ConvertTo-Json -Depth 100), $utf8)
        }
        'COMPLETION_MISSING_FINDING' {
            $completion = Get-Content -LiteralPath $completionPath -Raw -Encoding UTF8 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $completion.findingStatus = @($completion.findingStatus | Select-Object -Skip 1)
            [System.IO.File]::WriteAllText($completionPath, ($completion | ConvertTo-Json -Depth 100), $utf8)
        }
        'COMPLETION_WRONG_STATUS' {
            $completion = Get-Content -LiteralPath $completionPath -Raw -Encoding UTF8 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $completion.findingStatus[0].status = 'CLOSED'
            [System.IO.File]::WriteAllText($completionPath, ($completion | ConvertTo-Json -Depth 100), $utf8)
        }
        'COMPLETION_WRONG_SEVERITY' {
            $completion = Get-Content -LiteralPath $completionPath -Raw -Encoding UTF8 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $completion.findingStatus[0].severity = 'MINOR'
            [System.IO.File]::WriteAllText($completionPath, ($completion | ConvertTo-Json -Depth 100), $utf8)
        }
        'COMPLETION_EMPTY_FINDINGS' {
            $completion = Get-Content -LiteralPath $completionPath -Raw -Encoding UTF8 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $completion.findingStatus = @()
            [System.IO.File]::WriteAllText($completionPath, ($completion | ConvertTo-Json -Depth 100), $utf8)
        }
        'COMPLETION_WRONG_PATH' {
            $completion = Get-Content -LiteralPath $completionPath -Raw -Encoding UTF8 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $completion.findingStatus[0].affectedPaths = @('docs/foreign.md')
            [System.IO.File]::WriteAllText($completionPath, ($completion | ConvertTo-Json -Depth 100), $utf8)
        }
        'COMPLETION_FOREIGN_TEST' {
            $completion = Get-Content -LiteralPath $completionPath -Raw -Encoding UTF8 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $completion.findingStatus[0].regressionTestIds = @(
                @($completion.findingStatus[0].regressionTestIds) + @('REG-UNKNOWN')
            )
            [System.IO.File]::WriteAllText($completionPath, ($completion | ConvertTo-Json -Depth 100), $utf8)
        }
        'COMPLETION_EMPTY_EVIDENCE' {
            $completion = Get-Content -LiteralPath $completionPath -Raw -Encoding UTF8 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $completion.findingStatus[0].evidenceReferences = @()
            [System.IO.File]::WriteAllText($completionPath, ($completion | ConvertTo-Json -Depth 100), $utf8)
        }
        'REPORT_MISSING_REPOSITORY_PATH' {
            $firstPath = [string]$assignment.focusedDelta.correctionOnlyPaths[0]
            $reportText = Get-Content -LiteralPath $reportPath -Raw -Encoding UTF8
            $reportText = [regex]::Replace(
                $reportText,
                '(?m)^' + [regex]::Escape($firstPath) + '\r?\n',
                '',
                1
            )
            [System.IO.File]::WriteAllText($reportPath, $reportText, $utf8)
        }
        'REPORT_EXTRA_REPOSITORY_PATH' {
            $reportText = Get-Content -LiteralPath $reportPath -Raw -Encoding UTF8
            $reportText = $reportText.Replace(
                'Commit Preparation remains blocked.',
                "docs/foreign.md`nCommit Preparation remains blocked."
            )
            [System.IO.File]::WriteAllText($reportPath, $reportText, $utf8)
        }
        'REPORT_MISSING_EXTERNAL_PATH' {
            $externalManifest = Get-Content -LiteralPath (
                Join-Path $staging 'external-governance-manifest.json'
            ) -Raw -Encoding UTF8 | ConvertFrom-Json -Depth 100 -DateKind String
            $firstPath = [string]$externalManifest.changes[0].activePath
            $reportText = Get-Content -LiteralPath $reportPath -Raw -Encoding UTF8
            $reportText = [regex]::Replace(
                $reportText,
                '(?m)^' + [regex]::Escape($firstPath) + '\r?\n',
                '',
                1
            )
            [System.IO.File]::WriteAllText($reportPath, $reportText, $utf8)
        }
        'REPORT_EXTRA_EXTERNAL_PATH' {
            $reportText = Get-Content -LiteralPath $reportPath -Raw -Encoding UTF8
            $reportText = $reportText.Replace(
                'Commit Preparation remains blocked.',
                "C:\Voxtronic\foreign-governance.md`nCommit Preparation remains blocked."
            )
            [System.IO.File]::WriteAllText($reportPath, $reportText, $utf8)
        }
        'REPORT_STALE_STATUS' {
            $reportText = Get-Content -LiteralPath $reportPath -Raw -Encoding UTF8
            $reportText = $reportText.Replace(
                'FOURTH_BUNDLED_CORRECTION_COMPLETE_AWAITING_FOCUSED_DELTA_REVIEW',
                'independent Full Review, possible bundled correction and focused Delta'
            )
            [System.IO.File]::WriteAllText($reportPath, $reportText, $utf8)
        }
        'REPORT_CONFLICTING_QUEUE' {
            Update-FixtureReportContract -ReportPath $reportPath -Mutation {
                param($contract)
                $contract.queue = 'BL-335 -> BL-333/BL-334'
            }
        }
        'REPORT_MISSING_BLOCK' {
            $reportText = Get-Content -LiteralPath $reportPath -Raw -Encoding UTF8
            $reportText = [regex]::Replace(
                $reportText,
                '(?ms)^BEGIN_GOVERNANCE_REPORT_CONTRACT\r?\n.*?\r?\nEND_GOVERNANCE_REPORT_CONTRACT$',
                ''
            )
            [System.IO.File]::WriteAllText($reportPath, $reportText, $utf8)
        }
        'REPORT_DOUBLE_BLOCK' {
            $reportText = Get-Content -LiteralPath $reportPath -Raw -Encoding UTF8
            $match = [regex]::Match(
                $reportText,
                '(?ms)^BEGIN_GOVERNANCE_REPORT_CONTRACT\r?\n.*?\r?\nEND_GOVERNANCE_REPORT_CONTRACT$'
            )
            [System.IO.File]::WriteAllText(
                $reportPath,
                $reportText + "`n" + $match.Value,
                $utf8
            )
        }
        'REPORT_DUMMY_JSON' {
            $reportText = Get-Content -LiteralPath $reportPath -Raw -Encoding UTF8
            $reportText = [regex]::Replace(
                $reportText,
                '(?ms)(^BEGIN_GOVERNANCE_REPORT_CONTRACT\r?\n)\{.*?\}(\r?\nEND_GOVERNANCE_REPORT_CONTRACT$)',
                '${1}{}${2}'
            )
            [System.IO.File]::WriteAllText($reportPath, $reportText, $utf8)
        }
        'LOCAL_REGISTER_STALE_STATUS' {
            $localPath = Join-Path $staging 'external/local-work-register/after.txt'
            $localText = Get-Content -LiteralPath $localPath -Raw -Encoding UTF8
            $localText = $localText.Replace(
                'FOURTH_BUNDLED_CORRECTION_COMPLETE_AWAITING_FOCUSED_DELTA_REVIEW',
                'independent Full Review, possible bundled correction and focused Delta'
            )
            [System.IO.File]::WriteAllText($localPath, $localText, $utf8)
        }
    }

    if ($refreshManifest) {
        Update-FixtureManifest -Staging $staging
    }
    [System.IO.Compression.ZipFile]::CreateFromDirectory(
        $staging,
        $packagePath,
        [System.IO.Compression.CompressionLevel]::Optimal,
        $false
    )
    return [pscustomobject]@{
        PackagePath = $packagePath
        AssignmentPath = $externalAssignmentPath
        CompletionReportPath = $completionPath
        ScopeInventoryPath = $scopePath
    }
}

function New-CompletionFixture {
    param(
        [Parameter(Mandatory)][string]$Root,
        [Parameter(Mandatory)][string]$Name,
        [Parameter(Mandatory)][object]$Record,
        [Parameter(Mandatory)][string]$RecordPath,
        [Parameter(Mandatory)][string[]]$ChangedPaths,
        [Parameter(Mandatory)]
        [ValidateSet('VALID', 'MISSING_FIELD', 'EXTRA_FIELD', 'WRONG_TYPE', 'INVALID_ENUM', 'INVALID_NESTED', 'ASSIGNMENT_CONTRADICTION', 'SCOPE_CONTRADICTION', 'INCOMPLETE_FINDING', 'READINESS_CONTRADICTION')]
        [string]$Mutation
    )

    $utf8 = [System.Text.UTF8Encoding]::new($false)
    $scopePath = Join-Path $Root ($Name + '-scope.json')
    $scope = [ordered]@{
        schemaVersion = 1
        entries = @($ChangedPaths | ForEach-Object { [ordered]@{ path = $_ } })
    }
    [System.IO.File]::WriteAllText($scopePath, ($scope | ConvertTo-Json -Depth 100), $utf8)

    $report = Copy-Record -Record $Record
    $report.PSObject.Properties.Remove('focusedDelta')
    $report | Add-Member -NotePropertyName repositoryArtifacts -NotePropertyValue @($ChangedPaths)
    $report | Add-Member -NotePropertyName externalGovernanceChanges -NotePropertyValue @()
    $report | Add-Member -NotePropertyName correctionPatchArtifact -NotePropertyValue 'correction-only.patch'
    $report | Add-Member -NotePropertyName correctionPatchSha256 -NotePropertyValue ('b' * 64)
    $report | Add-Member -NotePropertyName currentDeltaArtifact -NotePropertyValue 'current-delta.patch'
    $report | Add-Member -NotePropertyName currentDeltaSha256 -NotePropertyValue ('c' * 64)
    $report | Add-Member -NotePropertyName reviewStatus -NotePropertyValue 'AWAITING_FOCUSED_DELTA_REVIEW'
    $report | Add-Member -NotePropertyName run007Status -NotePropertyValue 'CORRECTED_PENDING_DELTA'
    $report | Add-Member -NotePropertyName queue -NotePropertyValue (
        'BL-333/BL-334 -> BL-335 -> BL-251 -> BL-324 -> final documentation convergence -> remove Local Work Register'
    )
    $report | Add-Member -NotePropertyName scopeInventorySha256 -NotePropertyValue (
        (Get-FileHash -LiteralPath $scopePath -Algorithm SHA256).Hash.ToLowerInvariant()
    )
    $report | Add-Member -NotePropertyName assignmentRecordSha256 -NotePropertyValue (
        (Get-FileHash -LiteralPath $RecordPath -Algorithm SHA256).Hash.ToLowerInvariant()
    )
    $report | Add-Member -NotePropertyName findingStatus -NotePropertyValue @(
        [ordered]@{
            id = 'BL333-BL334-REV-007'
            severity = 'MAJOR'
            status = 'CORRECTED_PENDING_DELTA'
            disposition = 'CORRECTED'
            correctionEvidence = 'Fixture correction evidence.'
            affectedPaths = @('Governance/change-trigger-catalog.json')
            regressionTestIds = @('FIXTURE-REGRESSION-001')
            evidenceReferences = @('fixture:evidence')
        }
    )
    $report | Add-Member -NotePropertyName materialCorrectionCycleCount -NotePropertyValue 1
    $report | Add-Member -NotePropertyName validationExecutionCount -NotePropertyValue 1
    $report | Add-Member -NotePropertyName infrastructureOrInvocationFailureCount -NotePropertyValue 0
    $report | Add-Member -NotePropertyName observedWarningCount -NotePropertyValue 0
    $report | Add-Member -NotePropertyName resolvedWarningCount -NotePropertyValue 0
    $report | Add-Member -NotePropertyName openWarningCount -NotePropertyValue 0
    $report | Add-Member -NotePropertyName warningCount -NotePropertyValue 0
    $report | Add-Member -NotePropertyName failureCount -NotePropertyValue 0
    $report | Add-Member -NotePropertyName zipFreeReadinessPassed -NotePropertyValue $true
    $report | Add-Member -NotePropertyName packageGeneration -NotePropertyValue ([ordered]@{
        freshStaging = $true
        finalZipWriteCount = 1
        inPlaceRepairPerformed = $false
    })
    $report | Add-Member -NotePropertyName nextAction -NotePropertyValue 'Run the next authorized governance checkpoint.'

    switch ($Mutation) {
        'MISSING_FIELD' { $report.PSObject.Properties.Remove('nextAction') }
        'EXTRA_FIELD' { $report | Add-Member -NotePropertyName unexpected -NotePropertyValue 'rejected' }
        'WRONG_TYPE' { $report.warningCount = 'zero' }
        'INVALID_ENUM' { $report.executionMode = 'UNSAFE_MODE' }
        'INVALID_NESTED' { $report.handoff = 'PASS' }
        'ASSIGNMENT_CONTRADICTION' { $report.repository = 'https://github.com/example/other.git' }
        'SCOPE_CONTRADICTION' { $report.repositoryArtifacts = @('README.md') }
        'INCOMPLETE_FINDING' {
            $report.findingStatus = @([ordered]@{ id = 'FIX-INCOMPLETE' })
        }
        'READINESS_CONTRADICTION' { $report.handoff.classicReviewReady = $true }
    }

    $reportPath = Join-Path $Root ($Name + '-completion.json')
    [System.IO.File]::WriteAllText($reportPath, ($report | ConvertTo-Json -Depth 100), $utf8)
    return [pscustomobject]@{
        ReportPath = $reportPath
        ScopePath = $scopePath
    }
}

function New-CanonicalFixtureDescriptor {
    param(
        [Parameter(Mandatory)][string]$CaseId,
        [string]$Group,
        [string[]]$Tags = @(),
        [string[]]$SupportedPlatforms = @('windows', 'linux'),
        [string[]]$RequiredCapabilities = @('git', 'powershell-7.6.4'),
        [string[]]$WindowsOnlyDependencies = @()
    )

    if ([string]::IsNullOrWhiteSpace($Group)) {
        $Group = switch -Regex ($CaseId) {
            'workflow-binding' { 'workflow-binding'; break }
            '^report-contract-' { 'completion-report'; break }
            'package-' { 'handoff-package'; break }
            'tracked-path|scope-' { 'scope-inventory'; break }
            'runtime-hosted-ci' { 'runtime-contract'; break }
            default { 'governance-record' }
        }
    }

    $polarityTag = if ($CaseId.StartsWith('positive-', [StringComparison]::Ordinal)) {
        'positive'
    }
    elseif ($CaseId.StartsWith('negative-', [StringComparison]::Ordinal)) {
        'negative'
    }
    else {
        'contract'
    }
    $normalizedTags = @(@($polarityTag) + @($Tags) | Sort-Object -Unique -CaseSensitive)

    return [pscustomobject][ordered]@{
        CaseId = $CaseId
        Group = $Group
        Tags = $normalizedTags
        SupportedPlatforms = @($SupportedPlatforms | Sort-Object -Unique -CaseSensitive)
        RequiredCapabilities = @($RequiredCapabilities | Sort-Object -Unique -CaseSensitive)
        WindowsOnlyDependencies = @($WindowsOnlyDependencies | Sort-Object -Unique -CaseSensitive)
    }
}

function Get-CaseStringArrayProperty {
    param(
        [Parameter(Mandatory)][object]$Case,
        [Parameter(Mandatory)][string]$PropertyName,
        [string[]]$Default = @()
    )

    if ($PropertyName -in @($Case.PSObject.Properties.Name)) {
        return [string[]]@($Case.$PropertyName)
    }
    return [string[]]@($Default)
}

$status = 'FAIL'
$failureMessage = $null
$results = [System.Collections.Generic.List[object]]::new()
$startedAt = [DateTimeOffset]::Now
$canonicalFixtureCount = 0
$fixtureInventorySHA256 = $null
$canonicalFixtureNames = @()
$canonicalFixtureInventory = @()
$selectedFixtureNames = @()
$selectedFixtureMetadata = @()
$selectionIsFullInventory = $false
$selectionResolved = $false
$cleanupErrors = [System.Collections.Generic.List[string]]::new()
$cleanupStatus = 'NOT_RUN'
$repositoryMutationDetected = $false
$repositoryStatusBefore = $null
$lastCompletedFixture = $null
$temporaryRoot = $null
$repositoryInternalPackagePath = $null
$resolvedRepositoryRoot = $null
$actualHead = $null
$resolvedResultPath = $null

try {
    $resolvedRepositoryRoot = [System.IO.Path]::GetFullPath($RepositoryRoot)
    $repositoryStatusBefore = @(& git -C $resolvedRepositoryRoot status --porcelain=v1 --untracked-files=all)
    if ($LASTEXITCODE -ne 0) {
        throw "Cannot capture the repository status before fixture execution: $resolvedRepositoryRoot"
    }
    if (-not [string]::IsNullOrWhiteSpace($ProgressPath)) {
        $resolvedProgressPath = [System.IO.Path]::GetFullPath($ProgressPath)
        $progressParent = [System.IO.Path]::GetDirectoryName($resolvedProgressPath)
        if (-not (Test-Path -LiteralPath $progressParent -PathType Container)) {
            throw "Progress path parent does not exist: $progressParent"
        }
        if (Test-Path -LiteralPath $resolvedProgressPath) {
            throw "Progress path must be new: $resolvedProgressPath"
        }
        $ProgressPath = $resolvedProgressPath
    }
    if (-not [string]::IsNullOrWhiteSpace($ResultPath)) {
        $resolvedResultPath = [System.IO.Path]::GetFullPath($ResultPath)
        $resultParent = [System.IO.Path]::GetDirectoryName($resolvedResultPath)
        if (-not (Test-Path -LiteralPath $resultParent -PathType Container)) {
            throw "Result path parent does not exist: $resultParent"
        }
        if (Test-Path -LiteralPath $resolvedResultPath) {
            throw "Result path must be new: $resolvedResultPath"
        }
        $ResultPath = $resolvedResultPath
    }
    $actualHead = [string](& git -C $resolvedRepositoryRoot rev-parse HEAD)
    if ($LASTEXITCODE -ne 0 -or $actualHead -notmatch '^[0-9a-f]{40}$') {
        throw "Cannot resolve the repository HEAD for fixture binding: $resolvedRepositoryRoot"
    }
    $validatorPath = Join-Path $resolvedRepositoryRoot 'scripts/Test-GovernanceConsistency.ps1'
    if (-not (Test-Path -LiteralPath $validatorPath -PathType Leaf)) {
        throw "Validator does not exist: $validatorPath"
    }
    if (-not (Test-Path -LiteralPath $repositoryArtifactValidatorMirrorPath -PathType Leaf)) {
        throw "Repository artifact validator mirror does not exist: $repositoryArtifactValidatorMirrorPath"
    }
    if (Test-Path -LiteralPath $externalCanonicalArtifactValidatorPath -PathType Leaf) {
        $repositoryArtifactValidatorMirrorBytes = [System.IO.File]::ReadAllBytes(
            $repositoryArtifactValidatorMirrorPath
        )
        $externalCanonicalArtifactValidatorBytes = [System.IO.File]::ReadAllBytes(
            $externalCanonicalArtifactValidatorPath
        )
        $repositoryArtifactValidatorMirrorHash = (
            Get-FileHash -LiteralPath $repositoryArtifactValidatorMirrorPath -Algorithm SHA256
        ).Hash.ToLowerInvariant()
        $externalCanonicalArtifactValidatorHash = (
            Get-FileHash -LiteralPath $externalCanonicalArtifactValidatorPath -Algorithm SHA256
        ).Hash.ToLowerInvariant()
        $artifactValidatorBytesMatch = (
            $repositoryArtifactValidatorMirrorBytes.Length -eq $externalCanonicalArtifactValidatorBytes.Length -and
            [System.Linq.Enumerable]::SequenceEqual(
                [byte[]]$repositoryArtifactValidatorMirrorBytes,
                [byte[]]$externalCanonicalArtifactValidatorBytes
            )
        )
        if (
            -not $artifactValidatorBytesMatch -or
            $repositoryArtifactValidatorMirrorHash -cne $externalCanonicalArtifactValidatorHash
        ) {
            throw (
                'Repository artifact validator mirror does not match the available external ' +
                'canonical validator byte-for-byte.'
            )
        }
    }
    if (-not (Test-Path -LiteralPath $CanonicalArtifactValidatorPath -PathType Leaf)) {
        throw "Canonical artifact validator does not exist: $CanonicalArtifactValidatorPath"
    }

    $temporaryRoot = Join-Path ([System.IO.Path]::GetTempPath()) ('flashgate-governance-fixtures-' + [guid]::NewGuid().ToString('N'))
    [void][System.IO.Directory]::CreateDirectory($temporaryRoot)
    $requiredPowerShellVersion = '7.6.4'
    $actualPowerShellVersion = $PSVersionTable.PSVersion.ToString()
    if ($actualPowerShellVersion -cne $requiredPowerShellVersion) {
        throw "PowerShell $requiredPowerShellVersion is required; actual=$actualPowerShellVersion"
    }
    $pwshExecutableName = if ($IsWindows) { 'pwsh.exe' } else { 'pwsh' }
    $pwsh = Join-Path $PSHOME $pwshExecutableName
    if (-not (Test-Path -LiteralPath $pwsh -PathType Leaf)) {
        throw "Current PowerShell executable does not exist: $pwsh"
    }

    $documentationGates = @('documentation-consistency', 'backlog-continuity', 'status', 'links', 'strict-utf8')
    $filesystemGates = @('root-confinement', 'symlink-reparse', 'limits', 'windows-linux')
    $findingGates = @('finding-remediation', 'regression-evidence', 'independent-delta-review')
    $platformGates = @('windows', 'native-linux', 'architecture', 'artifact-metadata')
    $protocolGates = @('schema', 'catalog', 'wire', 'smoke', 'compatibility')
    $releaseGates = @('metadata', 'inventory', 'checksums', 'reproducibility', 'leak-scan', 'provenance')
    $workflowGates = @('workflow-source', 'hosted-ci', 'permissions', 'action-pinning')

    $cases = [System.Collections.Generic.List[object]]::new()

    $bundled = New-BaseRecord -Mode BUNDLED_CORRECTION -Checkpoint ASSIGNMENT_START -ObservedTriggers DOCUMENTATION_GOVERNANCE_LIFECYCLE -TriggeredDomains documentation-governance -AffectedGates $documentationGates
    [void]$cases.Add([pscustomobject]@{ Name = 'positive-bundled-start'; ExpectedExit = 0; ChangedPaths = @('Governance/change-trigger-catalog.json'); Record = $bundled; Tags = @('post-bl230-native', 'legacy-compatibility') })

    $currentStateGatePositive = Copy-Record -Record $bundled
    [void]$cases.Add([pscustomobject]@{
            Name = 'positive-current-state-gate'
            ExpectedExit = 0
            ChangedPaths = @('Governance/change-trigger-catalog.json')
            Record = $currentStateGatePositive
            Tags = @('post-bl230-native', 'current-state-gate')
        })

    $independent = New-BaseRecord -Mode INDEPENDENT_REVIEW -Checkpoint SPRINT_CLOSE -ObservedTriggers REVIEW_FINDING -TriggeredDomains finding -AffectedGates $findingGates
    [void]$cases.Add([pscustomobject]@{ Name = 'positive-independent-review'; ExpectedExit = 0; ChangedPaths = @(); Record = $independent })

    $delta = New-BaseRecord -Mode FOCUSED_INDEPENDENT_DELTA_REVIEW -Checkpoint SPRINT_CLOSE -ObservedTriggers @('DOCUMENTATION_GOVERNANCE_LIFECYCLE', 'REVIEW_FINDING') -TriggeredDomains @('documentation-governance', 'finding') -AffectedGates @($documentationGates + $findingGates | Sort-Object -Unique)
    $delta.focusedDelta.priorReviewBaselineSha256 = 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
    $delta.focusedDelta.correctionStartCommit = '537ea1c1660cddfde5aace1888242d80a6be77bf'
    $delta.focusedDelta.correctionPatchSha256 = 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb'
    $delta.focusedDelta.currentDeltaSha256 = 'cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc'
    $delta.focusedDelta.correctionOnlyPaths = @('Governance/change-trigger-catalog.json')
    $delta.focusedDelta.allowedDeltaPaths = @('Governance/change-trigger-catalog.json')
    $delta.focusedDelta.reviewedFindingIds = @('BL333-BL334-REV-001')
    $delta.focusedDelta.regressionEvidenceIds = @('REG-FOCUSED-001')
    $delta.review.originalFindings = @(
        New-Finding -Id 'BL333-BL334-REV-001' -Disposition 'CORRECTED'
    )
    [void]$cases.Add([pscustomobject]@{
            Name = 'positive-focused-delta'
            ExpectedExit = 0
            ChangedPaths = @('Governance/change-trigger-catalog.json')
            Record = $delta
            ExpectedPriorReviewBaselineSha256 = 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
            ExpectedCorrectionPatchSha256 = 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb'
            ExpectedCurrentDeltaSha256 = 'cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc'
            ExpectedCorrectionStartCommit = '537ea1c1660cddfde5aace1888242d80a6be77bf'
        })

    $preCommit = New-BaseRecord -Mode COMMIT_PREPARATION -Checkpoint PRE_COMMIT -ObservedTriggers DOCUMENTATION_GOVERNANCE_LIFECYCLE -TriggeredDomains documentation-governance -AffectedGates $documentationGates
    $preCommit.documentationConsistencyResult = 'PASS'
    $preCommit.review.focusedValidationResult = 'PASS'
    $preCommit.review.independentDeltaReviewRequired = $false
    $preCommit.handoff.required = $true
    $preCommit.handoff.package = 'BL-333-BL-334-handoff.zip'
    $preCommit.handoff.artifacts = @('HANDOFF.md', 'report.md', 'changes.patch', 'scope-inventory.json', 'MANIFEST.sha256')
    $preCommit.handoff.scopeInventoryResult = 'PASS'
    $preCommit.handoff.patchCompletenessResult = 'PASS'
    $preCommit.handoff.manifestResult = 'PASS'
    $preCommit.handoff.classicReviewReady = $true
    $preCommit.commitPreparation.allFindingsClosed = $true
    $preCommit.commitPreparation.independentDeltaReviewComplete = $true
    $preCommit.commitPreparation.scopeVerified = $true
    $preCommit.commitPreparation.validationPassed = $true
    $classicFixture = New-ClassicFixturePackage -Root $temporaryRoot -Record $preCommit `
        -ChangedPaths @('BACKLOG.md') -ValidatorPath $CanonicalArtifactValidatorPath
    [void]$cases.Add([pscustomobject]@{
            Name = 'positive-commit-preparation'
            ExpectedExit = 0
            ChangedPaths = @('BACKLOG.md')
            Record = $preCommit
            RecordPath = $classicFixture.AssignmentPath
            CompletionReportPath = $classicFixture.CompletionReportPath
            ScopeInventoryPath = $classicFixture.ScopeInventoryPath
            HandoffPackagePath = $classicFixture.PackagePath
            CanonicalArtifactValidatorPath = $CanonicalArtifactValidatorPath
            ExpectedCanonicalArtifactValidatorSha256 = $classicFixture.ValidatorHash
            ExpectedCorrectionPatchSha256 = $classicFixture.CorrectionPatchHash
            ExpectedCurrentDeltaSha256 = $classicFixture.CurrentDeltaHash
            SupportedPlatforms = @('windows')
            RequiredCapabilities = @('git', 'powershell-7.6.4', 'external-windows-governance-backups')
            WindowsOnlyDependencies = @('external-governance-backup-paths')
        })

    $missingRequiredCurrentStateGate = Copy-Record -Record $preCommit
    $missingRequiredCurrentStateGate.PSObject.Properties.Remove('currentStateGate')
    [void]$cases.Add([pscustomobject]@{
            Name = 'negative-required-current-state-gate-missing'
            ExpectedExit = 1
            ChangedPaths = @('BACKLOG.md')
            Record = $missingRequiredCurrentStateGate
            Tags = @('post-bl230-native', 'current-state-gate')
        })

    $release = New-BaseRecord -Mode INDEPENDENT_REVIEW -Checkpoint RELEASE_CANDIDATE -ObservedTriggers WORKFLOW_CI -TriggeredDomains workflow-ci -AffectedGates $workflowGates
    $release.hostedCI.required = $true
    $release.hostedCI.sourceVerified = $true
    $release.hostedCI.sources = @(
        "thomasweidner/flashgate-mcp@$actualHead",
        'github-actions-run:123'
    )
    $release.hostedCI.workflowCommit = '537ea1c1660cddfde5aace1888242d80a6be77bf'
    $release.hostedCI.runId = '123'
    $release.hostedCI.runAttempt = 1
    $release.hostedCI.event = 'workflow_dispatch'
    $release.hostedCI.ref = 'refs/tags/v1.0.0'
    $release.hostedCI.headSha = $actualHead
    [void]$cases.Add([pscustomobject]@{
            Name = 'positive-release-hosted-ci'
            ExpectedExit = 0
            ChangedPaths = @()
            Record = $release
            ExpectedWorkflowCommit = '537ea1c1660cddfde5aace1888242d80a6be77bf'
            ExpectedRunId = '123'
            ExpectedRunAttempt = 1
            ExpectedEvent = 'workflow_dispatch'
            ExpectedRef = 'refs/tags/v1.0.0'
            ExpectedHeadSha = $actualHead
        })

    $filesystem = New-BaseRecord -Mode BUNDLED_CORRECTION -Checkpoint MATERIAL_SCOPE_CHANGE -ObservedTriggers FILESYSTEM_SEMANTICS -TriggeredDomains filesystem -AffectedGates $filesystemGates
    [void]$cases.Add([pscustomobject]@{ Name = 'positive-real-filesystem-path'; ExpectedExit = 0; ChangedPaths = @('internal/fs/read_file.go'); Record = $filesystem })

    $protocol = New-BaseRecord -Mode BUNDLED_CORRECTION -Checkpoint MATERIAL_SCOPE_CHANGE -ObservedTriggers PUBLIC_MCP_CONTRACT -TriggeredDomains protocol -AffectedGates $protocolGates
    [void]$cases.Add([pscustomobject]@{ Name = 'positive-real-mcp-path'; ExpectedExit = 0; ChangedPaths = @('internal/mcp/server/server.go'); Record = $protocol })

    $releaseWorkflow = New-BaseRecord -Mode BUNDLED_CORRECTION -Checkpoint MATERIAL_SCOPE_CHANGE -ObservedTriggers @('PLATFORM_ARCHITECTURE', 'ARTIFACT_RELEASE_CONTRACT', 'WORKFLOW_CI') -TriggeredDomains @('platform', 'release', 'workflow-ci') -AffectedGates @($platformGates + $releaseGates + $workflowGates | Sort-Object -Unique)
    [void]$cases.Add([pscustomobject]@{ Name = 'positive-real-release-workflow-path'; ExpectedExit = 0; ChangedPaths = @('.github/workflows/release-build.yml'); Record = $releaseWorkflow })

    $missingDiffTrigger = Copy-Record -Record $bundled
    $missingDiffTrigger.observedTriggers = @()
    [void]$cases.Add([pscustomobject]@{ Name = 'negative-missing-diff-trigger'; ExpectedExit = 1; ChangedPaths = @('Governance/change-trigger-catalog.json'); Record = $missingDiffTrigger })

    $duplicateNotPerformed = Copy-Record -Record $bundled
    $duplicateNotPerformed.duplicateSearch.performed = $false
    [void]$cases.Add([pscustomobject]@{ Name = 'negative-duplicate-search'; ExpectedExit = 1; ChangedPaths = @('Governance/change-trigger-catalog.json'); Record = $duplicateNotPerformed })

    $independentMutation = Copy-Record -Record $independent
    $independentMutation.review.repositoryMutationAllowed = $true
    $independentMutation.review.findingFixesPerformed = $true
    [void]$cases.Add([pscustomobject]@{ Name = 'negative-independent-mutation'; ExpectedExit = 1; ChangedPaths = @(); Record = $independentMutation })

    $invalidSameRun = Copy-Record -Record $bundled
    $invalidSameRun.review.discoveredInRunFindings = @(New-Finding -Id 'FIX-001' -Disposition 'DISCOVERED_AND_CORRECTED_IN_RUN')
    [void]$cases.Add([pscustomobject]@{ Name = 'negative-same-run-disposition'; ExpectedExit = 1; ChangedPaths = @('Governance/change-trigger-catalog.json'); Record = $invalidSameRun })

    $deferredWithoutBoundary = Copy-Record -Record $bundled
    $deferredWithoutBoundary.review.deferredFindings = @(New-Finding -Id 'FIX-002' -Disposition 'DEFERRED_WITH_BOUNDARY' -BoundaryId 'BOUNDARY-404' -BoundaryType 'SECURITY_DECISION')
    [void]$cases.Add([pscustomobject]@{ Name = 'negative-deferred-without-boundary'; ExpectedExit = 1; ChangedPaths = @('Governance/change-trigger-catalog.json'); Record = $deferredWithoutBoundary })

    $incompleteHandoff = Copy-Record -Record $preCommit
    $incompleteHandoff.handoff.artifacts = @('HANDOFF.md')
    [void]$cases.Add([pscustomobject]@{ Name = 'negative-classic-readiness'; ExpectedExit = 1; ChangedPaths = @('BACKLOG.md'); Record = $incompleteHandoff })

    $falseClassicReadiness = Copy-Record -Record $preCommit
    $falseClassicReadiness.handoff.classicReviewReady = $false
    [void]$cases.Add([pscustomobject]@{ Name = 'negative-precommit-classic-readiness-false'; ExpectedExit = 1; ChangedPaths = @('BACKLOG.md'); Record = $falseClassicReadiness })

    $unblockedBoundary = Copy-Record -Record $bundled
    $unblockedBoundary.decisionBoundaries = @([ordered]@{
        id = 'BOUNDARY-001'
        type = 'SECURITY_DECISION'
        cause = 'fixture cause'
        evidence = 'fixture'
        blocking = $true
        owner = 'project owner'
        nextAction = 'decide'
    })
    [void]$cases.Add([pscustomobject]@{ Name = 'negative-boundary-result'; ExpectedExit = 1; ChangedPaths = @('Governance/change-trigger-catalog.json'); Record = $unblockedBoundary })

    $openPreCommitFinding = Copy-Record -Record $preCommit
    $openPreCommitFinding.review.originalFindings = @(New-Finding -Id 'FIX-003' -Disposition 'OPEN')
    [void]$cases.Add([pscustomobject]@{
            Name = 'negative-open-precommit-finding'
            ExpectedExit = 1
            ChangedPaths = @('BACKLOG.md')
            Record = $openPreCommitFinding
            HandoffPackagePath = $classicFixture.PackagePath
            CanonicalArtifactValidatorPath = $CanonicalArtifactValidatorPath
            ExpectedCanonicalArtifactValidatorSha256 = $classicFixture.ValidatorHash
        })

    $unverifiedRelease = Copy-Record -Record $release
    $unverifiedRelease.hostedCI.sourceVerified = $false
    [void]$cases.Add([pscustomobject]@{ Name = 'negative-release-ci-source'; ExpectedExit = 1; ChangedPaths = @(); Record = $unverifiedRelease })

    $incompleteReleaseSources = Copy-Record -Record $release
    $incompleteReleaseSources.hostedCI.sources = @('github-actions-run:123')
    [void]$cases.Add([pscustomobject]@{ Name = 'negative-release-ci-source-incomplete'; ExpectedExit = 1; ChangedPaths = @(); Record = $incompleteReleaseSources })

    $wrongBooleanType = Copy-Record -Record $bundled
    $wrongBooleanType.duplicateSearch.performed = 'true'
    [void]$cases.Add([pscustomobject]@{ Name = 'negative-schema-boolean-type'; ExpectedExit = 1; ChangedPaths = @('Governance/change-trigger-catalog.json'); Record = $wrongBooleanType })

    $unknownNestedProperty = Copy-Record -Record $bundled
    $unknownNestedProperty.review | Add-Member -NotePropertyName unexpected -NotePropertyValue 'rejected'
    [void]$cases.Add([pscustomobject]@{ Name = 'negative-schema-nested-property'; ExpectedExit = 1; ChangedPaths = @('Governance/change-trigger-catalog.json'); Record = $unknownNestedProperty })

    foreach ($invalidPathCase in @(
            [pscustomobject]@{ Name = 'parent-traversal'; Path = '../x' },
            [pscustomobject]@{ Name = 'absolute-unix'; Path = '/etc/x' },
            [pscustomobject]@{ Name = 'absolute-windows'; Path = 'C:\x' },
            [pscustomobject]@{ Name = 'unc'; Path = '\\server\share' },
            [pscustomobject]@{ Name = 'dot-segment'; Path = './x' },
            [pscustomobject]@{ Name = 'backslash'; Path = 'internal\fs\x.go' },
            [pscustomobject]@{ Name = 'double-separator'; Path = 'internal//fs/x.go' },
            [pscustomobject]@{ Name = 'empty'; Path = '' },
            [pscustomobject]@{ Name = 'control-character'; Path = ('docs/x' + [char]0x001f + '.md') },
            [pscustomobject]@{ Name = 'unicode-nfd'; Path = ('docs/e' + [char]0x0301 + '.md') },
            [pscustomobject]@{ Name = 'unknown-repository-relative'; Path = 'unclassified-production/x.go' }
        )) {
        $invalidPathProperties = [ordered]@{
                Name = 'negative-path-' + $invalidPathCase.Name
                ExpectedExit = 1
                ChangedPaths = @($invalidPathCase.Path)
                Record = $bundled
        }
        if ($invalidPathCase.Name -ceq 'unicode-nfd') {
            $invalidPathProperties.ChangedPathExpression = "@([string]::Concat('docs/e',[char]0x0301,'.md'))"
        }
        [void]$cases.Add([pscustomobject]$invalidPathProperties)
    }

    $nonEmptyNoTrigger = New-BaseRecord -Mode INDEPENDENT_REVIEW -Checkpoint SPRINT_CLOSE
    [void]$cases.Add([pscustomobject]@{
            Name = 'negative-nonempty-no-trigger'
            ExpectedExit = 1
            ChangedPaths = @('README.md')
            Record = $nonEmptyNoTrigger
        })

    $sameRunDiscovery = New-Finding -Id 'FIX-SAME-RUN-001' -Disposition 'DISCOVERED_AND_CORRECTED_IN_RUN'
    $sameRunCorrection = New-Finding -Id 'FIX-SAME-RUN-001' -Disposition 'CORRECTED'
    $validSameRun = Copy-Record -Record $bundled
    $validSameRun.review.findingFixesPerformed = $true
    $validSameRun.review.discoveredInRunFindings = @($sameRunDiscovery)
    $validSameRun.review.correctedInRunFindings = @($sameRunCorrection)
    $validSameRun.review.permanentRegressionEvidence = @('REG-FIXTURE')
    [void]$cases.Add([pscustomobject]@{
            Name = 'positive-same-run-pair'
            ExpectedExit = 0
            ChangedPaths = @('Governance/change-trigger-catalog.json')
            Record = $validSameRun
        })

    $missingSameRunPartner = Copy-Record -Record $validSameRun
    $missingSameRunPartner.review.correctedInRunFindings = @()
    [void]$cases.Add([pscustomobject]@{ Name = 'negative-same-run-missing-partner'; ExpectedExit = 1; ChangedPaths = @('Governance/change-trigger-catalog.json'); Record = $missingSameRunPartner })

    $duplicateSameRunPartner = Copy-Record -Record $validSameRun
    $duplicateSameRunPartner.review.correctedInRunFindings = @(
        $sameRunCorrection,
        $sameRunCorrection
    )
    [void]$cases.Add([pscustomobject]@{ Name = 'negative-same-run-duplicate-partner'; ExpectedExit = 1; ChangedPaths = @('Governance/change-trigger-catalog.json'); Record = $duplicateSameRunPartner })

    $sameRunMissingEvidence = Copy-Record -Record $validSameRun
    $sameRunMissingEvidence.review.discoveredInRunFindings[0].regressionEvidenceIds = @()
    [void]$cases.Add([pscustomobject]@{ Name = 'negative-same-run-missing-evidence'; ExpectedExit = 1; ChangedPaths = @('Governance/change-trigger-catalog.json'); Record = $sameRunMissingEvidence })

    $correctionWithoutDiscovery = Copy-Record -Record $validSameRun
    $correctionWithoutDiscovery.review.discoveredInRunFindings = @()
    [void]$cases.Add([pscustomobject]@{ Name = 'negative-same-run-correction-without-discovery'; ExpectedExit = 1; ChangedPaths = @('Governance/change-trigger-catalog.json'); Record = $correctionWithoutDiscovery })

    $foreignCorrectionId = Copy-Record -Record $validSameRun
    $foreignCorrectionId.review.correctedInRunFindings[0].id = 'FIX-SAME-RUN-FOREIGN'
    [void]$cases.Add([pscustomobject]@{ Name = 'negative-same-run-foreign-id'; ExpectedExit = 1; ChangedPaths = @('Governance/change-trigger-catalog.json'); Record = $foreignCorrectionId })

    $validDeferred = Copy-Record -Record $bundled
    $validDeferred.changeTriggerReviewResult = 'BLOCKED_PENDING_DECISION'
    $validDeferred.decisionBoundaries = @([ordered]@{
            id = 'BOUNDARY-SECURITY-001'
            type = 'SECURITY_DECISION'
            cause = 'A security owner decision is required.'
            evidence = 'fixture evidence'
            blocking = $true
            owner = 'security owner'
            nextAction = 'Decide the security policy.'
        })
    $validDeferred.review.deferredFindings = @(
        New-Finding -Id 'FIX-DEFERRED-001' -Disposition 'DEFERRED_WITH_BOUNDARY' -BoundaryId 'BOUNDARY-SECURITY-001' -BoundaryType 'SECURITY_DECISION'
    )
    [void]$cases.Add([pscustomobject]@{ Name = 'positive-deferred-boundary'; ExpectedExit = 0; ChangedPaths = @('Governance/change-trigger-catalog.json'); Record = $validDeferred })

    $nonBlockingDeferred = Copy-Record -Record $validDeferred
    $nonBlockingDeferred.decisionBoundaries[0].blocking = $false
    [void]$cases.Add([pscustomobject]@{ Name = 'negative-deferred-nonblocking'; ExpectedExit = 1; ChangedPaths = @('Governance/change-trigger-catalog.json'); Record = $nonBlockingDeferred })

    $duplicateDeferredBoundary = Copy-Record -Record $validDeferred
    $duplicateDeferredBoundary.decisionBoundaries = @(
        $duplicateDeferredBoundary.decisionBoundaries[0],
        $duplicateDeferredBoundary.decisionBoundaries[0]
    )
    [void]$cases.Add([pscustomobject]@{ Name = 'negative-deferred-ambiguous-boundary'; ExpectedExit = 1; ChangedPaths = @('Governance/change-trigger-catalog.json'); Record = $duplicateDeferredBoundary })

    $emptyDeferredNextAction = Copy-Record -Record $validDeferred
    $emptyDeferredNextAction.decisionBoundaries[0].nextAction = ''
    [void]$cases.Add([pscustomobject]@{ Name = 'negative-deferred-empty-next-action'; ExpectedExit = 1; ChangedPaths = @('Governance/change-trigger-catalog.json'); Record = $emptyDeferredNextAction })

    $mismatchedDeferredClass = Copy-Record -Record $validDeferred
    $mismatchedDeferredClass.review.deferredFindings[0].boundaryType = 'ARCHITECTURE_DECISION'
    [void]$cases.Add([pscustomobject]@{ Name = 'negative-deferred-class-mismatch'; ExpectedExit = 1; ChangedPaths = @('Governance/change-trigger-catalog.json'); Record = $mismatchedDeferredClass })

    $focusedUnrelatedPath = Copy-Record -Record $delta
    [void]$cases.Add([pscustomobject]@{
            Name = 'negative-focused-unrelated-path'
            ExpectedExit = 1
            ChangedPaths = @('Governance/change-trigger-catalog.json', 'README.md')
            Record = $focusedUnrelatedPath
        })

    $focusedUnchangedDeclared = Copy-Record -Record $delta
    $focusedUnchangedDeclared.focusedDelta.correctionOnlyPaths += 'README.md'
    $focusedUnchangedDeclared.focusedDelta.allowedDeltaPaths += 'README.md'
    [void]$cases.Add([pscustomobject]@{ Name = 'negative-focused-unchanged-declared'; ExpectedExit = 1; ChangedPaths = @('Governance/change-trigger-catalog.json'); Record = $focusedUnchangedDeclared })

    $focusedMissingFinding = Copy-Record -Record $delta
    $focusedMissingFinding.focusedDelta.reviewedFindingIds = @()
    [void]$cases.Add([pscustomobject]@{ Name = 'negative-focused-missing-finding'; ExpectedExit = 1; ChangedPaths = @('Governance/change-trigger-catalog.json'); Record = $focusedMissingFinding })

    $focusedWrongBaseline = Copy-Record -Record $delta
    [void]$cases.Add([pscustomobject]@{
            Name = 'negative-focused-wrong-baseline-hash'
            ExpectedExit = 1
            ChangedPaths = @('Governance/change-trigger-catalog.json')
            Record = $focusedWrongBaseline
            ExpectedPriorReviewBaselineSha256 = ('d' * 64)
        })

    $focusedWrongPatch = Copy-Record -Record $delta
    [void]$cases.Add([pscustomobject]@{
            Name = 'negative-focused-wrong-patch-hash'
            ExpectedExit = 1
            ChangedPaths = @('Governance/change-trigger-catalog.json')
            Record = $focusedWrongPatch
            ExpectedCorrectionPatchSha256 = ('d' * 64)
        })

    $focusedWrongCurrent = Copy-Record -Record $delta
    [void]$cases.Add([pscustomobject]@{
            Name = 'negative-focused-wrong-current-delta-hash'
            ExpectedExit = 1
            ChangedPaths = @('Governance/change-trigger-catalog.json')
            Record = $focusedWrongCurrent
            ExpectedCurrentDeltaSha256 = ('d' * 64)
        })

    $focusedScopeExpansion = Copy-Record -Record $delta
    $focusedScopeExpansion.focusedDelta.allowedDeltaPaths += 'README.md'
    [void]$cases.Add([pscustomobject]@{ Name = 'negative-focused-scope-expansion'; ExpectedExit = 1; ChangedPaths = @('Governance/change-trigger-catalog.json'); Record = $focusedScopeExpansion })

    foreach ($provenanceCase in @(
            [pscustomobject]@{ Name = 'repository'; Property = 'ExpectedRepository'; Value = 'https://github.com/example/other.git' },
            [pscustomobject]@{ Name = 'baseline'; Property = 'ExpectedBaselineCommit'; Value = ('a' * 40) },
            [pscustomobject]@{ Name = 'head'; Property = 'ExpectedCurrentCommit'; Value = ('a' * 40) }
        )) {
        $caseProperties = [ordered]@{
            Name = 'negative-provenance-' + $provenanceCase.Name
            ExpectedExit = 1
            ChangedPaths = @('Governance/change-trigger-catalog.json')
            Record = $bundled
        }
        $caseProperties[$provenanceCase.Property] = $provenanceCase.Value
        [void]$cases.Add([pscustomobject]$caseProperties)
    }

    foreach ($workflowProvenanceCase in @(
            [pscustomobject]@{ Name = 'workflow-commit'; Property = 'ExpectedWorkflowCommit'; Value = ('a' * 40) },
            [pscustomobject]@{ Name = 'run-id'; Property = 'ExpectedRunId'; Value = '999' },
            [pscustomobject]@{ Name = 'run-attempt'; Property = 'ExpectedRunAttempt'; Value = 2 },
            [pscustomobject]@{ Name = 'event'; Property = 'ExpectedEvent'; Value = 'push' },
            [pscustomobject]@{ Name = 'ref'; Property = 'ExpectedRef'; Value = 'refs/heads/main' },
            [pscustomobject]@{ Name = 'head-sha'; Property = 'ExpectedHeadSha'; Value = ('a' * 40) }
        )) {
        $caseProperties = [ordered]@{
            Name = 'negative-provenance-' + $workflowProvenanceCase.Name
            ExpectedExit = 1
            ChangedPaths = @()
            Record = $release
            ExpectedWorkflowCommit = '537ea1c1660cddfde5aace1888242d80a6be77bf'
            ExpectedRunId = '123'
            ExpectedRunAttempt = 1
            ExpectedEvent = 'workflow_dispatch'
            ExpectedRef = 'refs/tags/v1.0.0'
            ExpectedHeadSha = $actualHead
        }
        $caseProperties[$workflowProvenanceCase.Property] = $workflowProvenanceCase.Value
        [void]$cases.Add([pscustomobject]$caseProperties)
    }

    foreach ($reportMutation in @(
            'VALID',
            'MISSING_FIELD',
            'EXTRA_FIELD',
            'WRONG_TYPE',
            'INVALID_ENUM',
            'INVALID_NESTED',
            'ASSIGNMENT_CONTRADICTION',
            'SCOPE_CONTRADICTION',
            'INCOMPLETE_FINDING',
            'READINESS_CONTRADICTION'
        )) {
        [void]$cases.Add([pscustomobject]@{
                Name = 'report-contract-' + $reportMutation.ToLowerInvariant().Replace('_', '-')
                ExpectedExit = if ($reportMutation -ceq 'VALID') { 0 } else { 1 }
                ChangedPaths = @('Governance/change-trigger-catalog.json')
                Record = $bundled
                CompletionMutation = $reportMutation
                Tags = if ($reportMutation -ceq 'VALID') { @('post-bl230-native', 'completion-contract') } else { @('completion-contract') }
            })
    }

    $expectedFailedCheckByMutation = @{
        HANDOFF_VISIBLE_PENDING_MISMATCH = 'RECORD-HANDOFF-VISIBLE-CONTRACT-PARITY'
        HANDOFF_VISIBLE_COMMIT_PREP_MISMATCH = 'RECORD-HANDOFF-VISIBLE-CONTRACT-PARITY'
        HANDOFF_VISIBLE_COMMIT_AUTH_MISMATCH = 'RECORD-HANDOFF-VISIBLE-CONTRACT-PARITY'
        HANDOFF_VISIBLE_TARGET_COUNT_MISMATCH = 'RECORD-HANDOFF-VISIBLE-CONTRACT-PARITY'
        HANDOFF_VISIBLE_CORRECTED_COUNT_MISMATCH = 'RECORD-HANDOFF-VISIBLE-CONTRACT-PARITY'
        HANDOFF_VISIBLE_PENDING_COUNT_MISMATCH = 'RECORD-HANDOFF-VISIBLE-CONTRACT-PARITY'
        HANDOFF_VISIBLE_CLOSED_COUNT_MISMATCH = 'RECORD-HANDOFF-VISIBLE-CONTRACT-PARITY'
        HANDOFF_VISIBLE_OPEN_COUNT_MISMATCH = 'RECORD-HANDOFF-VISIBLE-CONTRACT-PARITY'
        HANDOFF_VISIBLE_DUPLICATE_KEY_SAME = 'RECORD-HANDOFF-VISIBLE-KEYS'
        HANDOFF_VISIBLE_DUPLICATE_KEY_CONFLICT = 'RECORD-HANDOFF-VISIBLE-KEYS'
        HANDOFF_VISIBLE_UNKNOWN_KEY = 'RECORD-HANDOFF-VISIBLE-KEYS'
        HANDOFF_RESERVED_KEY_OUTSIDE = 'RECORD-HANDOFF-RESERVED-CONTROL-LINES'
        HANDOFF_EXTRA_STATUS_BEGIN = 'RECORD-HANDOFF-MARKER-COUNTS'
        HANDOFF_EXTRA_STATUS_END = 'RECORD-HANDOFF-MARKER-COUNTS'
        HANDOFF_EXTRA_CONTRACT_BEGIN = 'RECORD-HANDOFF-MARKER-COUNTS'
        HANDOFF_EXTRA_CONTRACT_END = 'RECORD-HANDOFF-MARKER-COUNTS'
        HANDOFF_REVERSED_STATUS_MARKERS = 'RECORD-HANDOFF-MARKER-COUNTS'
        HANDOFF_REVERSED_CONTRACT_MARKERS = 'RECORD-HANDOFF-MARKER-COUNTS'
    }
    foreach ($packageMutation in @(
            'MISSING_FILE',
            'WRONG_HASH',
            'WRONG_SIZE',
            'SCOPE_INVENTORY_WRONG_SCOPE',
            'WRONG_SCOPE',
            'DUPLICATE_SCOPE',
            'PATH_SCOPE_SWAP',
            'EXTRA_SCOPE',
            'PATH_CASE_DUPLICATE',
            'HANDOFF_STALE_STATUS',
            'HANDOFF_TARGET_MISMATCH',
            'HANDOFF_CLOSED_MISMATCH',
            'HANDOFF_RUN_MISMATCH',
            'HANDOFF_COMMIT_PREP_MISMATCH',
            'HANDOFF_COMMIT_AUTH_MISMATCH',
            'HANDOFF_REVIEW_MODE_MISMATCH',
            'HANDOFF_NEXT_ACTION_MISMATCH',
            'HANDOFF_NARRATIVE_CONTRACT_MISMATCH',
            'HANDOFF_MISSING_BLOCK',
            'HANDOFF_DUPLICATE_BLOCK',
            'HANDOFF_INVALID_JSON',
            'HANDOFF_VISIBLE_PENDING_MISMATCH',
            'HANDOFF_VISIBLE_COMMIT_PREP_MISMATCH',
            'HANDOFF_VISIBLE_COMMIT_AUTH_MISMATCH',
            'HANDOFF_VISIBLE_TARGET_COUNT_MISMATCH',
            'HANDOFF_VISIBLE_CORRECTED_COUNT_MISMATCH',
            'HANDOFF_VISIBLE_PENDING_COUNT_MISMATCH',
            'HANDOFF_VISIBLE_CLOSED_COUNT_MISMATCH',
            'HANDOFF_VISIBLE_OPEN_COUNT_MISMATCH',
            'HANDOFF_VISIBLE_DUPLICATE_KEY_SAME',
            'HANDOFF_VISIBLE_DUPLICATE_KEY_CONFLICT',
            'HANDOFF_VISIBLE_UNKNOWN_KEY',
            'HANDOFF_RESERVED_KEY_OUTSIDE',
            'HANDOFF_EXTRA_STATUS_BEGIN',
            'HANDOFF_EXTRA_STATUS_END',
            'HANDOFF_EXTRA_CONTRACT_BEGIN',
            'HANDOFF_EXTRA_CONTRACT_END',
            'HANDOFF_REVERSED_STATUS_MARKERS',
            'HANDOFF_REVERSED_CONTRACT_MARKERS',
            'MISSING_ASSIGNMENT',
            'MISSING_REPORT',
            'MISSING_EXTERNAL',
            'EXTRA_MEMBER',
            'PATCH_BYTES_STALE_MANIFEST',
            'PATCH_BYTES_REMANIFESTED',
            'PATCH_BYTES_RECORDS_REHASHED',
            'CURRENT_DELTA_BYTES_REMANIFESTED',
            'PATCHES_SWAPPED',
            'TRUSTED_WRONG_PATCH_HASH',
            'FOCUSED_WRONG_PATCH_HASH',
            'FOCUSED_WRONG_CURRENT_HASH',
            'FOCUSED_MISSING_FINDING',
            'FOCUSED_EXTRA_PATH',
            'FOCUSED_MISSING_INTERFACE',
            'FOCUSED_WRONG_REFERENCE',
            'CORRECTION_MISSING_FINDING',
            'CORRECTION_EXTRA_FINDING',
            'CORRECTION_DUPLICATE_FINDING',
            'CORRECTION_MISSING_PATH',
            'CORRECTION_FOREIGN_PATH',
            'CORRECTION_MISSING_TEST',
            'REGRESSION_UNKNOWN_FINDING',
            'REGRESSION_UNKNOWN_TEST',
            'COMPLETION_MISSING_FINDING',
            'COMPLETION_WRONG_STATUS',
            'COMPLETION_WRONG_SEVERITY',
            'COMPLETION_EMPTY_FINDINGS',
            'COMPLETION_WRONG_PATH',
            'COMPLETION_FOREIGN_TEST',
            'COMPLETION_EMPTY_EVIDENCE',
            'REPORT_MISSING_REPOSITORY_PATH',
            'REPORT_EXTRA_REPOSITORY_PATH',
            'REPORT_MISSING_EXTERNAL_PATH',
            'REPORT_EXTRA_EXTERNAL_PATH',
            'REPORT_STALE_STATUS',
            'REPORT_CONFLICTING_QUEUE',
            'REPORT_MISSING_BLOCK',
            'REPORT_DOUBLE_BLOCK',
            'REPORT_DUMMY_JSON',
            'LOCAL_REGISTER_STALE_STATUS'
        )) {
        $mutatedPackage = New-MutatedFixturePackage -Root $temporaryRoot `
            -SourcePackage $classicFixture.PackagePath -Mutation $packageMutation
        $packageCase = [ordered]@{
                Name = 'negative-real-package-' + $packageMutation.ToLowerInvariant().Replace('_', '-')
                ExpectedExit = 1
                ChangedPaths = @('BACKLOG.md')
                Record = $preCommit
                RecordPath = $mutatedPackage.AssignmentPath
                CompletionReportPath = $mutatedPackage.CompletionReportPath
                ScopeInventoryPath = $mutatedPackage.ScopeInventoryPath
                HandoffPackagePath = $mutatedPackage.PackagePath
                CanonicalArtifactValidatorPath = $CanonicalArtifactValidatorPath
                ExpectedCanonicalArtifactValidatorSha256 = $classicFixture.ValidatorHash
                ExpectedCorrectionPatchSha256 = $classicFixture.CorrectionPatchHash
                ExpectedCurrentDeltaSha256 = $classicFixture.CurrentDeltaHash
        }
        if ($expectedFailedCheckByMutation.ContainsKey($packageMutation)) {
            $packageCase.ExpectedFailedCheck =
                [string]$expectedFailedCheckByMutation[$packageMutation]
        }
        [void]$cases.Add([pscustomobject]$packageCase)
    }

    $openBoundaryPackageRecord = Copy-Record -Record $preCommit
    $openBoundaryPackageRecord.changeTriggerReviewResult = 'BLOCKED_PENDING_DECISION'
    $openBoundaryPackageRecord.decisionBoundaries = @([ordered]@{
            id = 'BOUNDARY-PACKAGE-001'
            type = 'SCOPE_DECISION'
            cause = 'A scope decision remains open.'
            evidence = 'fixture evidence'
            blocking = $true
            owner = 'project owner'
            nextAction = 'Decide the package scope.'
        })
    [void]$cases.Add([pscustomobject]@{
            Name = 'negative-real-package-open-boundary'
            ExpectedExit = 1
            ChangedPaths = @('BACKLOG.md')
            Record = $openBoundaryPackageRecord
            HandoffPackagePath = $classicFixture.PackagePath
            CanonicalArtifactValidatorPath = $CanonicalArtifactValidatorPath
            ExpectedCanonicalArtifactValidatorSha256 = $classicFixture.ValidatorHash
        })

    $repositoryInternalPackagePath = Join-Path $resolvedRepositoryRoot (
        '.governance-fixture-' + [guid]::NewGuid().ToString('N') + '.zip'
    )
    Copy-Item -LiteralPath $classicFixture.PackagePath -Destination $repositoryInternalPackagePath
    $repositoryInternalPackageRecord = Copy-Record -Record $preCommit
    $repositoryInternalPackageRecord.handoff.package = $repositoryInternalPackagePath
    [void]$cases.Add([pscustomobject]@{
            Name = 'negative-real-package-repository-internal'
            ExpectedExit = 1
            ChangedPaths = @('BACKLOG.md')
            Record = $repositoryInternalPackageRecord
            HandoffPackagePath = $repositoryInternalPackagePath
            CanonicalArtifactValidatorPath = $CanonicalArtifactValidatorPath
            ExpectedCanonicalArtifactValidatorSha256 = $classicFixture.ValidatorHash
        })

    $catalog = Get-Content -LiteralPath (Join-Path $resolvedRepositoryRoot 'Governance/change-trigger-catalog.json') -Raw -Encoding UTF8 |
        ConvertFrom-Json -Depth 100 -DateKind String
    $actualTrackedPaths = @(& git -C $resolvedRepositoryRoot ls-files)
    foreach ($trigger in @($catalog.triggers | Where-Object { 'DIFF' -in @($_.sources) })) {
        $representativePath = @(
            $actualTrackedPaths | Where-Object {
                $trackedCandidate = $_
                @($trigger.pathPatterns | Where-Object {
                        $trackedCandidate -match (Convert-GlobToRegex -Pattern ([string]$_)
                        )
                    }).Count -gt 0
            }
        ) | Select-Object -First 1
        if ([string]::IsNullOrWhiteSpace([string]$representativePath)) {
            continue
        }
        $representativeTriggers = @(
            $catalog.triggers | Where-Object {
                $candidateTrigger = $_
                'DIFF' -in @($candidateTrigger.sources) -and
                @($candidateTrigger.pathPatterns | Where-Object {
                        $representativePath -match (Convert-GlobToRegex -Pattern ([string]$_)
                        )
                    }).Count -gt 0
            }
        )
        $representativeRecord = New-BaseRecord `
            -Mode BUNDLED_CORRECTION `
            -Checkpoint MATERIAL_SCOPE_CHANGE `
            -ObservedTriggers @($representativeTriggers | ForEach-Object { [string]$_.id }) `
            -TriggeredDomains @($representativeTriggers | ForEach-Object { [string]$_.domain } | Sort-Object -Unique) `
            -AffectedGates @($representativeTriggers | ForEach-Object { @($_.continuousGates) } | Sort-Object -Unique)
        [void]$cases.Add([pscustomobject]@{
                Name = 'positive-domain-' + ([string]$trigger.id).ToLowerInvariant().Replace('_', '-')
                ExpectedExit = 0
                ChangedPaths = @([string]$representativePath)
                Record = $representativeRecord
            })
    }

    foreach ($modeId in @('INDEPENDENT_REVIEW', 'BUNDLED_CORRECTION', 'FOCUSED_INDEPENDENT_DELTA_REVIEW', 'COMMIT_PREPARATION')) {
        foreach ($modeFlag in @(
                'independent',
                'repositoryMutationAllowed',
                'externalMutationAllowed',
                'reviewScope',
                'commitAllowed',
                'correctionAllowed',
                'requiresPriorReview',
                'requiresFocusedDeltaReview'
            )) {
            $mutatedCatalog = $catalog | ConvertTo-Json -Depth 100 | ConvertFrom-Json -Depth 100 -DateKind String
            $modeEntry = @($mutatedCatalog.modes | Where-Object id -ceq $modeId)[0]
            $modeEntry.$modeFlag = if ($modeFlag -ceq 'reviewScope') {
                'UNSAFE_SCOPE'
            }
            else {
                -not [bool]$modeEntry.$modeFlag
            }
            $mutatedCatalogPath = Join-Path $temporaryRoot (
                'catalog-' + $modeId.ToLowerInvariant() + '-' + $modeFlag.ToLowerInvariant() + '.json'
            )
            [System.IO.File]::WriteAllText(
                $mutatedCatalogPath,
                ($mutatedCatalog | ConvertTo-Json -Depth 100),
                [System.Text.UTF8Encoding]::new($false)
            )
            [void]$cases.Add([pscustomobject]@{
                    Name = 'negative-mode-' + $modeId.ToLowerInvariant().Replace('_', '-') + '-' + $modeFlag.ToLowerInvariant()
                    ExpectedExit = 1
                    ChangedPaths = @('README.md')
                    Record = $bundled
                    CatalogPath = $mutatedCatalogPath
                })
        }
    }

    foreach ($policyName in @('remediationPolicy', 'activityGatePolicy', 'validationExecutionPolicy', 'classicHandoffPolicy')) {
        foreach ($propertyName in @($catalog.$policyName.PSObject.Properties.Name)) {
            $mutatedCatalog = $catalog | ConvertTo-Json -Depth 100 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $currentValue = $mutatedCatalog.$policyName.$propertyName
            $mutatedCatalog.$policyName.$propertyName = if ($currentValue -is [System.Array]) {
                @($currentValue | Select-Object -Skip 1)
            }
            elseif ($currentValue -is [bool]) {
                -not [bool]$currentValue
            }
            elseif ($currentValue -is [long] -or $currentValue -is [int]) {
                [int]$currentValue + 1
            }
            else {
                'UNSAFE_POLICY_VALUE'
            }
            $mutatedCatalogPath = Join-Path $temporaryRoot (
                'catalog-policy-' + $policyName.ToLowerInvariant() + '-' +
                $propertyName.ToLowerInvariant() + '.json'
            )
            [System.IO.File]::WriteAllText(
                $mutatedCatalogPath,
                ($mutatedCatalog | ConvertTo-Json -Depth 100),
                [System.Text.UTF8Encoding]::new($false)
            )
            [void]$cases.Add([pscustomobject]@{
                    Name = 'negative-policy-' + $policyName.ToLowerInvariant() + '-' +
                        $propertyName.ToLowerInvariant()
                    ExpectedExit = 1
                    ChangedPaths = @('Governance/change-trigger-catalog.json')
                    Record = $bundled
                    CatalogPath = $mutatedCatalogPath
                })
        }
    }

    $coveragePolicyCases = @(
        [pscustomobject]@{ Name = 'negative-selection-error-before-fixture-start'; Property = 'selectionResolvedBeforeRunnerStart'; Unsafe = $false },
        [pscustomobject]@{ Name = 'negative-explicit-source-worktree-binding'; Property = 'explicitSourceAndWorktreeParametersRequired'; Unsafe = $false },
        [pscustomobject]@{ Name = 'negative-helper-command-shadowing'; Property = 'helperCommandShadowingAllowed'; Unsafe = $true },
        [pscustomobject]@{ Name = 'negative-detached-head-detection'; Property = 'detachedHeadDetection'; Unsafe = 'AMBIGUOUS_BRANCH_TEXT' },
        [pscustomobject]@{ Name = 'negative-direct-exitcode-evaluation'; Property = 'directExitCodeEvaluationRequired'; Unsafe = $false },
        [pscustomobject]@{ Name = 'negative-scope-overrun-fails-closed'; Property = 'scopeOverrunFailsClosed'; Unsafe = $false }
    )
    foreach ($coverageCase in $coveragePolicyCases) {
        $mutatedCatalog = $catalog | ConvertTo-Json -Depth 100 |
            ConvertFrom-Json -Depth 100 -DateKind String
        $mutatedCatalog.validationExecutionPolicy.($coverageCase.Property) = $coverageCase.Unsafe
        $mutatedCatalogPath = Join-Path $temporaryRoot ('catalog-' + $coverageCase.Name + '.json')
        [System.IO.File]::WriteAllText(
            $mutatedCatalogPath,
            ($mutatedCatalog | ConvertTo-Json -Depth 100),
            [System.Text.UTF8Encoding]::new($false)
        )
        [void]$cases.Add([pscustomobject]@{
                Name = $coverageCase.Name
                ExpectedExit = 1
                ExpectedFailedCheck = 'CATALOG-POLICY-validationExecutionPolicy-' + $coverageCase.Property
                ChangedPaths = @('Governance/change-trigger-catalog.json')
                Record = $bundled
                CatalogPath = $mutatedCatalogPath
            })
    }

    $orchestrationCatalogCases = @(
        [pscustomobject]@{
            Name = 'negative-orchestration-profile-duplicate'
            FailedCheck = 'CATALOG-ORCHESTRATION-PROFILES'
            Mutate = {
                param($candidate)
                $candidate.orchestrationPolicy.profiles = @(
                    @($candidate.orchestrationPolicy.profiles) +
                    @($candidate.orchestrationPolicy.profiles[0])
                )
            }
        },
        [pscustomobject]@{
            Name = 'negative-cheap-gate-order'
            FailedCheck = 'CATALOG-CHEAP-GATE-ORDER'
            Mutate = {
                param($candidate)
                $candidate.orchestrationPolicy.cheapGateOrder = @(
                    'text-policy',
                    'parser-syntax',
                    'git-diff-check',
                    'external-input-binding',
                    'toolchain-context-binding',
                    'source-worktree-selector-binding'
                )
            }
        },
        [pscustomobject]@{
            Name = 'negative-efficiency-counter-omitted'
            FailedCheck = 'CATALOG-EFFICIENCY-COUNTERS'
            Mutate = {
                param($candidate)
                $candidate.orchestrationPolicy.counterFields = @(
                    $candidate.orchestrationPolicy.counterFields |
                        Where-Object { [string]$_ -cne 'readOnlyProbeCount' }
                )
            }
        },
        [pscustomobject]@{
            Name = 'negative-full-completion-owner-duplicate'
            FailedCheck = 'CATALOG-FULL-COMPLETION-UNIQUE'
            Mutate = {
                param($candidate)
                $candidate.orchestrationPolicy.profiles[0].fullMatrix = $true
            }
        }
    )
    foreach ($orchestrationCase in $orchestrationCatalogCases) {
        $mutatedCatalog = $catalog | ConvertTo-Json -Depth 100 |
            ConvertFrom-Json -Depth 100 -DateKind String
        & $orchestrationCase.Mutate $mutatedCatalog
        $mutatedCatalogPath = Join-Path $temporaryRoot ('catalog-' + $orchestrationCase.Name + '.json')
        [System.IO.File]::WriteAllText(
            $mutatedCatalogPath,
            ($mutatedCatalog | ConvertTo-Json -Depth 100),
            [System.Text.UTF8Encoding]::new($false)
        )
        [void]$cases.Add([pscustomobject]@{
                Name = $orchestrationCase.Name
                ExpectedExit = 1
                ExpectedFailedCheck = $orchestrationCase.FailedCheck
                ChangedPaths = @('Governance/change-trigger-catalog.json')
                Record = $bundled
                CatalogPath = $mutatedCatalogPath
                Tags = @('bl-339-phase-a', 'orchestration-policy')
            })
    }

    $caseFixtureDescriptors = @(
        foreach ($case in $cases) {
            $caseGroup = if ('Group' -in @($case.PSObject.Properties.Name)) {
                [string]$case.Group
            }
            else {
                $null
            }
            New-CanonicalFixtureDescriptor -CaseId ([string]$case.Name) -Group $caseGroup `
                -Tags (Get-CaseStringArrayProperty -Case $case -PropertyName 'Tags') `
                -SupportedPlatforms (Get-CaseStringArrayProperty -Case $case -PropertyName 'SupportedPlatforms' -Default @('windows', 'linux')) `
                -RequiredCapabilities (Get-CaseStringArrayProperty -Case $case -PropertyName 'RequiredCapabilities' -Default @('git', 'powershell-7.6.4')) `
                -WindowsOnlyDependencies (Get-CaseStringArrayProperty -Case $case -PropertyName 'WindowsOnlyDependencies')
        }
    )
    $artifactPolicyFixtureDescriptors = @(
        New-CanonicalFixtureDescriptor -CaseId 'positive-package-single-file-direct' -Group 'handoff-package'
        New-CanonicalFixtureDescriptor -CaseId 'positive-package-full-rebuild-after-change' -Group 'handoff-package'
        New-CanonicalFixtureDescriptor -CaseId 'negative-package-duplicate-zip-path' -Group 'handoff-package'
        New-CanonicalFixtureDescriptor -CaseId 'negative-package-case-colliding-zip-path' -Group 'handoff-package'
        New-CanonicalFixtureDescriptor -CaseId 'negative-package-absolute-zip-path' -Group 'handoff-package'
        New-CanonicalFixtureDescriptor -CaseId 'negative-package-traversal-zip-path' -Group 'handoff-package'
        New-CanonicalFixtureDescriptor -CaseId 'negative-package-link-junction-reparse-entry' -Group 'handoff-package'
        New-CanonicalFixtureDescriptor -CaseId 'negative-package-separate-member-transfer' -Group 'handoff-package'
    )
    $workflowFixtureDescriptors = @(
        New-CanonicalFixtureDescriptor -CaseId 'positive-real-ci-workflow-binding' -Group 'workflow-binding' -Tags @('post-bl230-native', 'workflow-binding')
        New-CanonicalFixtureDescriptor -CaseId 'positive-real-release-workflow-binding' -Group 'workflow-binding' -Tags @('post-bl230-native', 'workflow-binding')
        New-CanonicalFixtureDescriptor -CaseId 'negative-real-workflow-semantic-diagnostic' -Group 'workflow-binding' -Tags @('post-bl230-native', 'workflow-binding', 'diagnostic-contract')
    )
    $supplementalFixtureDescriptors = @(
        @($artifactPolicyFixtureDescriptors)
        New-CanonicalFixtureDescriptor -CaseId 'positive-runtime-hosted-ci-array' -Group 'runtime-contract'
        New-CanonicalFixtureDescriptor -CaseId 'positive-complete-tracked-path-coverage' -Group 'scope-inventory' -Tags @('post-bl230-native', 'inventory-contract')
        New-CanonicalFixtureDescriptor -CaseId 'negative-unclassified-tracked-path' -Group 'scope-inventory'
        @($workflowFixtureDescriptors)
    )
    $artifactPolicyFixtureNames = @($artifactPolicyFixtureDescriptors | ForEach-Object { [string]$_.CaseId })
    $workflowFixtureNames = @($workflowFixtureDescriptors | ForEach-Object { [string]$_.CaseId })
    $canonicalFixtureInventory = @($caseFixtureDescriptors) + @($supplementalFixtureDescriptors)
    $canonicalFixtureNames = @($canonicalFixtureInventory | ForEach-Object { [string]$_.CaseId })
    $duplicateInventoryNames = @(
        $canonicalFixtureNames |
            Group-Object -CaseSensitive |
            Where-Object Count -gt 1 |
            ForEach-Object { [string]$_.Name }
    )
    if ($canonicalFixtureNames.Count -eq 0 -or $duplicateInventoryNames.Count -gt 0) {
        throw (
            'Canonical fixture inventory must be non-empty and contain unique case IDs. ' +
            "duplicates=$($duplicateInventoryNames -join ', ')"
        )
    }
    $fixtureInventoryBytes = [System.Text.UTF8Encoding]::new($false).GetBytes(
        (($canonicalFixtureInventory | ConvertTo-Json -Depth 8 -Compress) + "`n")
    )
    $fixtureInventorySHA256 = [Convert]::ToHexString(
        [System.Security.Cryptography.SHA256]::HashData($fixtureInventoryBytes)
    ).ToLowerInvariant()
    $canonicalFixtureCount = $canonicalFixtureNames.Count

    $duplicateSelectedNames = @(
        @($CaseName) |
            Group-Object -CaseSensitive |
            Where-Object Count -gt 1 |
            ForEach-Object { [string]$_.Name }
    )
    $duplicateSelectedTags = @(
        @($Tag) |
            Group-Object -CaseSensitive |
            Where-Object Count -gt 1 |
            ForEach-Object { [string]$_.Name }
    )
    $unknownSelectedNames = @(
        @($CaseName) | Where-Object { $_ -cnotin $canonicalFixtureNames }
    )
    $canonicalTags = @($canonicalFixtureInventory.Tags | Sort-Object -Unique -CaseSensitive)
    $unknownSelectedTags = @(
        @($Tag) | Where-Object { $_ -cnotin $canonicalTags }
    )
    $ambiguousSelectors = @(
        if (@($CaseName).Count -gt 0 -and @($Tag).Count -gt 0) {
            'CaseName+Tag'
        }
    )
    if (
        $duplicateSelectedNames.Count -gt 0 -or
        $duplicateSelectedTags.Count -gt 0 -or
        $unknownSelectedNames.Count -gt 0 -or
        $unknownSelectedTags.Count -gt 0 -or
        $ambiguousSelectors.Count -gt 0
    ) {
        throw (
            'Fixture selection must resolve exactly once against the canonical inventory. ' +
            "duplicates=$($duplicateSelectedNames -join ', '); " +
            "duplicateTags=$($duplicateSelectedTags -join ', '); " +
            "unknown=$($unknownSelectedNames -join ', '); " +
            "unknownTags=$($unknownSelectedTags -join ', '); " +
            "ambiguous=$($ambiguousSelectors -join ', ')"
        )
    }
    $selectionIsFullInventory = @($CaseName).Count -eq 0 -and @($Tag).Count -eq 0
    $selectedFixtureMetadata = if (@($CaseName).Count -gt 0) {
        @($canonicalFixtureInventory | Where-Object { $_.CaseId -cin @($CaseName) })
    }
    elseif (@($Tag).Count -gt 0) {
        @($canonicalFixtureInventory | Where-Object {
                $fixtureTags = @($_.Tags)
                @($Tag | Where-Object { $_ -cnotin $fixtureTags }).Count -eq 0
            })
    }
    else {
        @($canonicalFixtureInventory)
    }
    if ($selectedFixtureMetadata.Count -eq 0) {
        throw 'Fixture selection resolved to zero canonical cases.'
    }
    $selectedFixtureNames = @($selectedFixtureMetadata | ForEach-Object { [string]$_.CaseId })

    if (-not [string]::IsNullOrWhiteSpace($TargetPlatform)) {
        $normalizedTargetPlatform = $TargetPlatform.ToLowerInvariant()
        if ($normalizedTargetPlatform -cnotin @('windows', 'linux')) {
            throw "Unsupported target platform selector: $TargetPlatform"
        }
        $platformIncompatible = @($selectedFixtureMetadata | Where-Object {
                $normalizedTargetPlatform -cnotin @($_.SupportedPlatforms) -or
                ($normalizedTargetPlatform -ceq 'linux' -and @($_.WindowsOnlyDependencies).Count -gt 0)
            } | ForEach-Object { [string]$_.CaseId })
        $missingCapabilities = @(
            foreach ($fixture in $selectedFixtureMetadata) {
                $missingForFixture = @($fixture.RequiredCapabilities | Where-Object { $_ -cnotin @($AvailableCapability) })
                if ($missingForFixture.Count -gt 0) {
                    "$($fixture.CaseId):$($missingForFixture -join '+')"
                }
            }
        )
        if ($platformIncompatible.Count -gt 0 -or $missingCapabilities.Count -gt 0) {
            throw (
                'Fixture selection is not valid for the requested platform and capabilities. ' +
                "platform=$normalizedTargetPlatform; incompatible=$($platformIncompatible -join ', '); " +
                "missingCapabilities=$($missingCapabilities -join ', ')"
            )
        }
        $TargetPlatform = $normalizedTargetPlatform
    }
    $selectionResolved = $true

    $selectedCases = @($cases | Where-Object { $_.Name -cin $selectedFixtureNames })
    foreach ($case in $selectedCases) {
        $recordPath = if ('RecordPath' -in @($case.PSObject.Properties.Name)) {
            [string]$case.RecordPath
        }
        else {
            Join-Path $temporaryRoot ($case.Name + '.json')
        }
        $reportPath = Join-Path $temporaryRoot ($case.Name + '-report.json')
        if ('RecordPath' -notin @($case.PSObject.Properties.Name)) {
            [System.IO.File]::WriteAllText($recordPath, ($case.Record | ConvertTo-Json -Depth 100), [System.Text.UTF8Encoding]::new($false))
        }
        $completionFixture = if ('CompletionMutation' -in @($case.PSObject.Properties.Name)) {
            New-CompletionFixture -Root $temporaryRoot -Name $case.Name -Record $case.Record `
                -RecordPath $recordPath -ChangedPaths @($case.ChangedPaths) `
                -Mutation ([string]$case.CompletionMutation)
        }
        else {
            $null
        }

        $commandParts = [System.Collections.Generic.List[string]]::new()
        [void]$commandParts.Add('& ' + (ConvertTo-PowerShellSingleQuotedLiteral -Value $validatorPath))
        [void]$commandParts.Add('-RepositoryRoot ' + (ConvertTo-PowerShellSingleQuotedLiteral -Value $resolvedRepositoryRoot))
        if ('CatalogPath' -in @($case.PSObject.Properties.Name)) {
            [void]$commandParts.Add('-CatalogPath ' + (
                    ConvertTo-PowerShellSingleQuotedLiteral -Value ([string]$case.CatalogPath)
                ))
        }
        [void]$commandParts.Add('-AssignmentRecordPath ' + (ConvertTo-PowerShellSingleQuotedLiteral -Value $recordPath))
        $changedPathLiteral = if ('ChangedPathExpression' -in @($case.PSObject.Properties.Name)) {
            [string]$case.ChangedPathExpression
        }
        else {
            ConvertTo-PowerShellArrayLiteral -Value @($case.ChangedPaths)
        }
        [void]$commandParts.Add('-ChangedPath ' + $changedPathLiteral)
        $fixtureTrackedPaths = if (@($case.ChangedPaths).Count -gt 0) {
            @($case.ChangedPaths)
        }
        else {
            @('README.md')
        }
        $trackedPathLiteral = if ('ChangedPathExpression' -in @($case.PSObject.Properties.Name)) {
            [string]$case.ChangedPathExpression
        }
        else {
            ConvertTo-PowerShellArrayLiteral -Value $fixtureTrackedPaths
        }
        [void]$commandParts.Add('-TrackedPath ' + $trackedPathLiteral)
        $expectedRepository = if ('ExpectedRepository' -in @($case.PSObject.Properties.Name)) {
            [string]$case.ExpectedRepository
        }
        else {
            'https://github.com/thomasweidner/flashgate-mcp.git'
        }
        $expectedBaselineCommit = if ('ExpectedBaselineCommit' -in @($case.PSObject.Properties.Name)) {
            [string]$case.ExpectedBaselineCommit
        }
        else {
            $actualHead
        }
        $expectedCurrentCommit = if ('ExpectedCurrentCommit' -in @($case.PSObject.Properties.Name)) {
            [string]$case.ExpectedCurrentCommit
        }
        else {
            $actualHead
        }
        [void]$commandParts.Add('-ExpectedRepository ' + (ConvertTo-PowerShellSingleQuotedLiteral -Value $expectedRepository))
        [void]$commandParts.Add('-ExpectedBaselineCommit ' + (ConvertTo-PowerShellSingleQuotedLiteral -Value $expectedBaselineCommit))
        [void]$commandParts.Add('-ExpectedCurrentCommit ' + (ConvertTo-PowerShellSingleQuotedLiteral -Value $expectedCurrentCommit))
        [void]$commandParts.Add('-ReportPath ' + (ConvertTo-PowerShellSingleQuotedLiteral -Value $reportPath))
        if ($null -ne $completionFixture) {
            [void]$commandParts.Add('-CompletionReportPath ' + (
                    ConvertTo-PowerShellSingleQuotedLiteral -Value $completionFixture.ReportPath
                ))
            [void]$commandParts.Add('-ScopeInventoryPath ' + (
                    ConvertTo-PowerShellSingleQuotedLiteral -Value $completionFixture.ScopePath
                ))
        }

        foreach ($propertyName in @(
                'CompletionReportPath',
                'ScopeInventoryPath',
                'HandoffPackagePath',
                'CanonicalArtifactValidatorPath',
                'ExpectedCanonicalArtifactValidatorSha256',
                'ExpectedWorkflowCommit',
                'ExpectedRunId',
                'ExpectedEvent',
                'ExpectedRef',
                'ExpectedHeadSha',
                'ExpectedPriorReviewBaselineSha256',
                'ExpectedCorrectionPatchSha256',
                'ExpectedCurrentDeltaSha256',
                'ExpectedCorrectionStartCommit'
            )) {
            if ($propertyName -in @($case.PSObject.Properties.Name)) {
                [void]$commandParts.Add("-$propertyName " + (
                        ConvertTo-PowerShellSingleQuotedLiteral -Value ([string]$case.$propertyName)
                    ))
            }
        }
        if ('ExpectedRunAttempt' -in @($case.PSObject.Properties.Name)) {
            [void]$commandParts.Add('-ExpectedRunAttempt ' + [string]$case.ExpectedRunAttempt)
        }

        $command = $commandParts -join ' '
        $output = @(& $pwsh -NoLogo -NoProfile -Command $command 2>&1)
        $actualExit = $LASTEXITCODE
        $expectedFailedCheckPass = $true
        if ('ExpectedFailedCheck' -in @($case.PSObject.Properties.Name)) {
            $expectedFailedCheckPass = $false
            if (Test-Path -LiteralPath $reportPath -PathType Leaf) {
                $fixtureReport = Get-Content -LiteralPath $reportPath -Raw -Encoding UTF8 |
                    ConvertFrom-Json -Depth 100 -DateKind String
                $expectedFailedCheckPass = @(
                    $fixtureReport.checks | Where-Object {
                        [string]$_.Id -ceq [string]$case.ExpectedFailedCheck -and
                        [string]$_.Result -ceq 'FAIL'
                    }
                ).Count -eq 1
            }
        }
        $passed = (
            $actualExit -eq [int]$case.ExpectedExit -and
            $expectedFailedCheckPass
        )
        [void]$results.Add([pscustomobject]@{
            Name         = $case.Name
            ExpectedExit = [int]$case.ExpectedExit
            ActualExit   = $actualExit
            Result       = if ($passed) { 'PASS' } else { 'FAIL' }
            Diagnostic   = if ($passed) {
                ''
            }
            else {
                "ExpectedFailedCheckPass=$expectedFailedCheckPass`n" +
                    ($output -join [Environment]::NewLine)
            }
        })
        $lastCompletedFixture = $case.Name
        if (-not [string]::IsNullOrWhiteSpace($ProgressPath)) {
            [System.IO.File]::AppendAllText(
                $ProgressPath,
                (([ordered]@{
                    CompletedAt = [DateTimeOffset]::Now.ToString('o')
                    FixtureNumber = $results.Count
                    Name = $case.Name
                    Result = $results[$results.Count - 1].Result
                } | ConvertTo-Json -Compress) + [Environment]::NewLine),
                [System.Text.UTF8Encoding]::new($false)
            )
        }
    }

    foreach ($fixtureName in $artifactPolicyFixtureNames) {
        if ($fixtureName -cnotin $selectedFixtureNames) {
            continue
        }

        $expectedExit = if ($fixtureName.StartsWith('positive-', [System.StringComparison]::Ordinal)) {
            0
        }
        else {
            1
        }
        $actualExit = 1
        $output = @()
        switch ($fixtureName) {
            'positive-package-single-file-direct' {
                $artifactPath = Join-Path $temporaryRoot 'BL-335-single-review-file.md'
                [System.IO.File]::WriteAllText(
                    $artifactPath,
                    "ClassicReviewReady: true`n",
                    [System.Text.UTF8Encoding]::new($false)
                )
                $output = @(
                    & $pwsh -NoLogo -NoProfile -File $CanonicalArtifactValidatorPath `
                        -ArtifactPath $artifactPath -ReadinessRequirement RequireTrue 2>&1
                )
                $actualExit = $LASTEXITCODE
            }
            'positive-package-full-rebuild-after-change' {
                $firstPackage = New-MinimalClassicReviewPackage -Root $temporaryRoot `
                    -Name 'BL-335-package-before-change' `
                    -ReadmeText "ClassicReviewReady: true`nVersion: one`n"
                $rebuiltPackage = New-MinimalClassicReviewPackage -Root $temporaryRoot `
                    -Name 'BL-335-package-after-change' `
                    -ReadmeText "ClassicReviewReady: true`nVersion: two`n"
                $firstOutput = @(
                    & $pwsh -NoLogo -NoProfile -File $CanonicalArtifactValidatorPath `
                        -ArtifactPath $firstPackage.PackagePath `
                        -ReadinessRequirement RequireTrue 2>&1
                )
                $firstExit = $LASTEXITCODE
                $secondOutput = @(
                    & $pwsh -NoLogo -NoProfile -File $CanonicalArtifactValidatorPath `
                        -ArtifactPath $rebuiltPackage.PackagePath `
                        -ReadinessRequirement RequireTrue 2>&1
                )
                $secondExit = $LASTEXITCODE
                $rebuildChanged = (
                    $firstPackage.PackageSha256 -cne $rebuiltPackage.PackageSha256 -and
                    $firstPackage.ManifestJsonSha256 -cne $rebuiltPackage.ManifestJsonSha256
                )
                $actualExit = if ($firstExit -eq 0 -and $secondExit -eq 0 -and $rebuildChanged) {
                    0
                }
                else {
                    1
                }
                $output = @($firstOutput + $secondOutput + "rebuildChanged=$rebuildChanged")
            }
            'negative-package-separate-member-transfer' {
                $planPassed = Test-ClassicTransferPlan -RequiredFileCount 3 `
                    -TransferFileCount 3 -TransferFileName 'README.md' `
                    -Instruction 'Upload README.md, report.md, and handoff.json separately.'
                $actualExit = if ($planPassed) { 0 } else { 1 }
                $output = @("transferPlanPassed=$planPassed")
            }
            default {
                $mutation = switch ($fixtureName) {
                    'negative-package-duplicate-zip-path' { 'DUPLICATE_PATH' }
                    'negative-package-case-colliding-zip-path' { 'CASE_COLLISION' }
                    'negative-package-absolute-zip-path' { 'ABSOLUTE_PATH' }
                    'negative-package-traversal-zip-path' { 'TRAVERSAL_PATH' }
                    'negative-package-link-junction-reparse-entry' { 'REPARSE_ENTRY' }
                }
                $mutatedPackage = New-MinimalClassicReviewPackage -Root $temporaryRoot `
                    -Name $fixtureName -ReadmeText "ClassicReviewReady: true`n" `
                    -Mutation $mutation
                $output = @(
                    & $pwsh -NoLogo -NoProfile -File $CanonicalArtifactValidatorPath `
                        -ArtifactPath $mutatedPackage.PackagePath `
                        -ReadinessRequirement RequireTrue 2>&1
                )
                $actualExit = $LASTEXITCODE
            }
        }

        $passed = $actualExit -eq $expectedExit
        [void]$results.Add([pscustomobject]@{
                Name = $fixtureName
                ExpectedExit = $expectedExit
                ActualExit = $actualExit
                Result = if ($passed) { 'PASS' } else { 'FAIL' }
                Diagnostic = if ($passed) { '' } else { $output -join [Environment]::NewLine }
            })
        $lastCompletedFixture = $fixtureName
        if (-not [string]::IsNullOrWhiteSpace($ProgressPath)) {
            [System.IO.File]::AppendAllText(
                $ProgressPath,
                (([ordered]@{
                    CompletedAt = [DateTimeOffset]::Now.ToString('o')
                    FixtureNumber = $results.Count
                    Name = $fixtureName
                    Result = $results[$results.Count - 1].Result
                } | ConvertTo-Json -Compress) + [Environment]::NewLine),
                [System.Text.UTF8Encoding]::new($false)
            )
        }
    }

    if ('positive-runtime-hosted-ci-array' -cin $selectedFixtureNames) {
        $runtimeRecordPath = Join-Path $temporaryRoot 'runtime-release-record.json'
        $runtimePackagePath = Join-Path $temporaryRoot 'PowerShell-7.6.4-win-x64.zip'
        [System.IO.File]::WriteAllText(
            $runtimeRecordPath,
            ($release | ConvertTo-Json -Depth 100),
            [System.Text.UTF8Encoding]::new($false)
        )
        [System.IO.File]::WriteAllBytes(
            $runtimePackagePath,
            [System.Text.Encoding]::UTF8.GetBytes('PowerShell package hash fixture')
        )
        $runtimePackageSha256 = (
            Get-FileHash -LiteralPath $runtimePackagePath -Algorithm SHA256
        ).Hash
        $runtimeCommandParts = @(
            '& ' + (ConvertTo-PowerShellSingleQuotedLiteral -Value $validatorPath),
            '-RepositoryRoot ' + (ConvertTo-PowerShellSingleQuotedLiteral -Value $resolvedRepositoryRoot),
            '-AssignmentRecordPath ' + (ConvertTo-PowerShellSingleQuotedLiteral -Value $runtimeRecordPath),
            '-ChangedPath @()',
            '-TrackedPath @(''README.md'')',
            '-RuntimeCheckpoint RELEASE_CANDIDATE',
            '-HostedCISource ' + (ConvertTo-PowerShellArrayLiteral -Value @(
                    "thomasweidner/flashgate-mcp@$actualHead",
                    'github-actions-run:123'
                )),
            '-ExpectedRepository ' + (ConvertTo-PowerShellSingleQuotedLiteral -Value 'https://github.com/thomasweidner/flashgate-mcp.git'),
            "-ExpectedBaselineCommit $actualHead",
            "-ExpectedCurrentCommit $actualHead",
            '-ExpectedWorkflowCommit 537ea1c1660cddfde5aace1888242d80a6be77bf',
            '-ExpectedRunId 123',
            '-ExpectedRunAttempt 1',
            '-ExpectedEvent workflow_dispatch',
            '-ExpectedRef refs/tags/v1.0.0',
            "-ExpectedHeadSha $actualHead",
            '-PowerShellPackagePath ' + (
                ConvertTo-PowerShellSingleQuotedLiteral -Value $runtimePackagePath
            )
        )

        $runtimeCommand = @(
            $runtimeCommandParts
            "-ExpectedPowerShellVersion $requiredPowerShellVersion"
            "-ExpectedPowerShellPackageSha256 $runtimePackageSha256"
        ) -join ' '
        $runtimeOutput = @(& $pwsh -NoLogo -NoProfile -Command $runtimeCommand 2>&1)
        $runtimeExit = $LASTEXITCODE

        $wrongVersionReportPath = Join-Path $temporaryRoot 'runtime-wrong-version-report.json'
        $wrongVersionCommand = @(
            $runtimeCommandParts
            '-ExpectedPowerShellVersion 0.0.0'
            "-ExpectedPowerShellPackageSha256 $runtimePackageSha256"
            '-ReportPath ' + (
                ConvertTo-PowerShellSingleQuotedLiteral -Value $wrongVersionReportPath
            )
        ) -join ' '
        $wrongVersionOutput = @(& $pwsh -NoLogo -NoProfile -Command $wrongVersionCommand 2>&1)
        $wrongVersionExit = $LASTEXITCODE
        $wrongVersionCheckPass = $false
        if (Test-Path -LiteralPath $wrongVersionReportPath -PathType Leaf) {
            $wrongVersionReport = Get-Content -LiteralPath $wrongVersionReportPath -Raw -Encoding UTF8 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $wrongVersionCheckPass = @(
                $wrongVersionReport.checks | Where-Object {
                    [string]$_.Id -ceq 'POWERSHELL-VERSION' -and
                    [string]$_.Result -ceq 'FAIL'
                }
            ).Count -eq 1
        }

        $wrongHashReportPath = Join-Path $temporaryRoot 'runtime-wrong-hash-report.json'
        $wrongHashCommand = @(
            $runtimeCommandParts
            "-ExpectedPowerShellVersion $requiredPowerShellVersion"
            "-ExpectedPowerShellPackageSha256 $('0' * 64)"
            '-ReportPath ' + (
                ConvertTo-PowerShellSingleQuotedLiteral -Value $wrongHashReportPath
            )
        ) -join ' '
        $wrongHashOutput = @(& $pwsh -NoLogo -NoProfile -Command $wrongHashCommand 2>&1)
        $wrongHashExit = $LASTEXITCODE
        $wrongHashCheckPass = $false
        if (Test-Path -LiteralPath $wrongHashReportPath -PathType Leaf) {
            $wrongHashReport = Get-Content -LiteralPath $wrongHashReportPath -Raw -Encoding UTF8 |
                ConvertFrom-Json -Depth 100 -DateKind String
            $wrongHashCheckPass = @(
                $wrongHashReport.checks | Where-Object {
                    [string]$_.Id -ceq 'POWERSHELL-PACKAGE-SHA256' -and
                    [string]$_.Result -ceq 'FAIL'
                }
            ).Count -eq 1
        }

        $runtimePassed = (
            $runtimeExit -eq 0 -and
            $wrongVersionExit -eq 1 -and
            $wrongVersionCheckPass -and
            $wrongHashExit -eq 1 -and
            $wrongHashCheckPass
        )
        [void]$results.Add([pscustomobject]@{
            Name         = 'positive-runtime-hosted-ci-array'
            ExpectedExit = 0
            ActualExit   = if ($runtimePassed) { 0 } else { 1 }
            Result       = if ($runtimePassed) { 'PASS' } else { 'FAIL' }
            Diagnostic   = if ($runtimePassed) {
                ''
            }
            else {
                (
                    "runtimeExit=$runtimeExit; wrongVersionExit=$wrongVersionExit; " +
                    "wrongVersionCheckPass=$wrongVersionCheckPass; wrongHashExit=$wrongHashExit; " +
                    "wrongHashCheckPass=$wrongHashCheckPass`n" +
                    "runtime=$($runtimeOutput -join [Environment]::NewLine)`n" +
                    "wrongVersion=$($wrongVersionOutput -join [Environment]::NewLine)`n" +
                    "wrongHash=$($wrongHashOutput -join [Environment]::NewLine)"
                )
            }
        })
        $lastCompletedFixture = 'positive-runtime-hosted-ci-array'
        if (-not [string]::IsNullOrWhiteSpace($ProgressPath)) {
            [System.IO.File]::AppendAllText(
                $ProgressPath,
                (([ordered]@{
                    CompletedAt = [DateTimeOffset]::Now.ToString('o')
                    FixtureNumber = $results.Count
                    Name = $lastCompletedFixture
                    Result = $results[$results.Count - 1].Result
                } | ConvertTo-Json -Compress) + [Environment]::NewLine),
                [System.Text.UTF8Encoding]::new($false)
            )
        }
    }

    if ('positive-complete-tracked-path-coverage' -cin $selectedFixtureNames) {
    $coverageOutput = @(
        & $pwsh -NoLogo -NoProfile -File $validatorPath `
            -RepositoryRoot $resolvedRepositoryRoot 2>&1
    )
    $coverageExit = $LASTEXITCODE
    [void]$results.Add([pscustomobject]@{
        Name = 'positive-complete-tracked-path-coverage'
        ExpectedExit = 0
        ActualExit = $coverageExit
        Result = if ($coverageExit -eq 0) { 'PASS' } else { 'FAIL' }
        Diagnostic = if ($coverageExit -eq 0) { '' } else { ($coverageOutput -join [Environment]::NewLine) }
    })
    $lastCompletedFixture = 'positive-complete-tracked-path-coverage'
    if (-not [string]::IsNullOrWhiteSpace($ProgressPath)) {
        [System.IO.File]::AppendAllText(
            $ProgressPath,
            (([ordered]@{
                CompletedAt = [DateTimeOffset]::Now.ToString('o')
                FixtureNumber = $results.Count
                Name = $lastCompletedFixture
                Result = $results[$results.Count - 1].Result
            } | ConvertTo-Json -Compress) + [Environment]::NewLine),
            [System.Text.UTF8Encoding]::new($false)
        )
    }
    }

    if ('negative-unclassified-tracked-path' -cin $selectedFixtureNames) {
        $unknownCoverageOutput = @(
            & $pwsh -NoLogo -NoProfile -File $validatorPath `
                -RepositoryRoot $resolvedRepositoryRoot `
                -TrackedPath 'unclassified-production/new.go' 2>&1
        )
        $unknownCoverageExit = $LASTEXITCODE
        [void]$results.Add([pscustomobject]@{
            Name = 'negative-unclassified-tracked-path'
            ExpectedExit = 1
            ActualExit = $unknownCoverageExit
            Result = if ($unknownCoverageExit -eq 1) { 'PASS' } else { 'FAIL' }
            Diagnostic = if ($unknownCoverageExit -eq 1) { '' } else { ($unknownCoverageOutput -join [Environment]::NewLine) }
        })
        $lastCompletedFixture = 'negative-unclassified-tracked-path'
        if (-not [string]::IsNullOrWhiteSpace($ProgressPath)) {
            [System.IO.File]::AppendAllText(
                $ProgressPath,
                (([ordered]@{
                    CompletedAt = [DateTimeOffset]::Now.ToString('o')
                    FixtureNumber = $results.Count
                    Name = $lastCompletedFixture
                    Result = $results[$results.Count - 1].Result
                } | ConvertTo-Json -Compress) + [Environment]::NewLine),
                [System.Text.UTF8Encoding]::new($false)
            )
        }
    }

    if (@($workflowFixtureNames | Where-Object { $_ -cin $selectedFixtureNames }).Count -gt 0) {
        $generatorPath = Join-Path $resolvedRepositoryRoot 'scripts/New-GovernanceWorkflowRecord.ps1'
        $workflowDefinitions = @(
            [pscustomobject]@{
                Name = 'positive-real-ci-workflow-binding'
                WorkflowPath = Join-Path $resolvedRepositoryRoot '.github/workflows/ci.yml'
                ChangedPath = '.github/workflows/ci.yml'
                Checkpoint = 'SPRINT_CLOSE'
                Event = 'pull_request'
                Ref = 'refs/pull/1/merge'
                IsCI = $true
                ExpectedExit = 0
                InjectInvalidRepeatedChecks = $false
            },
            [pscustomobject]@{
                Name = 'positive-real-release-workflow-binding'
                WorkflowPath = Join-Path $resolvedRepositoryRoot '.github/workflows/release-build.yml'
                ChangedPath = '.github/workflows/release-build.yml'
                Checkpoint = 'RELEASE_CANDIDATE'
                Event = 'workflow_dispatch'
                Ref = 'refs/tags/v1.0.0'
                IsCI = $false
                ExpectedExit = 0
                InjectInvalidRepeatedChecks = $false
            },
            [pscustomobject]@{
                Name = 'negative-real-workflow-semantic-diagnostic'
                WorkflowPath = Join-Path $resolvedRepositoryRoot '.github/workflows/ci.yml'
                ChangedPath = '.github/workflows/ci.yml'
                Checkpoint = 'SPRINT_CLOSE'
                Event = 'pull_request'
                Ref = 'refs/pull/1/merge'
                IsCI = $true
                ExpectedExit = 1
                InjectInvalidRepeatedChecks = $true
            }
        )
        foreach ($workflowDefinition in $workflowDefinitions) {
            if ($workflowDefinition.Name -cnotin $selectedFixtureNames) {
                continue
            }
            $workflowText = Get-Content -LiteralPath $workflowDefinition.WorkflowPath -Raw -Encoding UTF8
            $requiredWorkflowTokens = @(
                'New-GovernanceWorkflowRecord.ps1',
                '-AssignmentRecordPath $recordPath',
                '-ChangedPath $changedPaths',
                "-RuntimeCheckpoint $($workflowDefinition.Checkpoint)"
            )
            if ([bool]$workflowDefinition.IsCI) {
                $requiredWorkflowTokens += @(
                    'name: Governance (exact PR head)',
                    'PULL_REQUEST_HEAD: ${{ github.event.pull_request.head.sha }}',
                    'PUSH_HEAD: ${{ github.sha }}',
                    'ref: ${{ steps.exact_head.outputs.expected_head }}',
                    'https://github.com/PowerShell/PowerShell/releases/download/v7.6.4/PowerShell-7.6.4-win-x64.zip',
                    '80832551C52809301E6071C8BAC977BEB5A2F1EC953EB4DB9F94DEB953333793',
                    'shell: pwsh',
                    'EXPECTED_PWSH_PATH: ${{ steps.pwsh764.outputs.pwsh_path }}',
                    '$installRoot | Out-File -FilePath $env:GITHUB_PATH -Encoding utf8 -Append',
                    '"PowerShellVersion: $powerShellVersion"',
                    '"ExpectedPowerShellPath: $expectedPwshPath"',
                    '"ActualPowerShellPath: $resolvedActualPwshPath"',
                    '[System.StringComparison]::OrdinalIgnoreCase',
                    'if (-not $powerShellProcessPathParity)',
                    '"Governance shell does not use the verified PowerShell executable."',
                    "'PowerShellProcessPathParityPassed: true'",
                    '"ExpectedPullRequestHead: $expectedHead"',
                    '"CheckedOutCommit: $checkedOutCommit"',
                    '"CurrentCommit: $checkedOutCommit"',
                    '"ExpectedCurrentCommit: $expectedHead"',
                    '"ExpectedHeadSha: $expectedHead"',
                    "'WorkflowSourceParityPassed: true'",
                    '-ExpectedRepository $repository',
                    '-ExpectedBaselineCommit $baselineCommit',
                    '-ExpectedCurrentCommit $expectedHead',
                    '-ExpectedWorkflowCommit $workflowCommit',
                    '-ExpectedRunId $env:GITHUB_RUN_ID_VALUE',
                    '-ExpectedRunAttempt $env:GITHUB_RUN_ATTEMPT_VALUE',
                    '-ExpectedEvent $env:EVENT_NAME',
                    '-ExpectedRef $env:GITHUB_REF_VALUE',
                    '-ExpectedHeadSha $expectedHead',
                    "-ExpectedPowerShellVersion '7.6.4'",
                    '-PowerShellPackagePath $env:PWSH_PACKAGE_PATH',
                    '-ExpectedPowerShellPackageSha256 $env:PWSH_PACKAGE_SHA256',
                    "Join-Path `$PWD 'scripts\Test-ClassicReviewArtifact.ps1'"
                )
            }
            else {
                $requiredWorkflowTokens += @(
                    '-ExpectedRepository $repository',
                    '-ExpectedBaselineCommit $baselineCommit',
                    '-ExpectedCurrentCommit $currentCommit',
                    '-ExpectedWorkflowCommit $workflowCommit',
                    "-ExpectedRunId '`${{ github.run_id }}'",
                    '-ExpectedRunAttempt ${{ github.run_attempt }}',
                    "-ExpectedEvent '`${{ github.event_name }}'",
                    "-ExpectedRef '`${{ github.ref }}'",
                    "-ExpectedHeadSha '`${{ github.sha }}'"
                )
            }
            $workflowContractFailures = @()
            if ([bool]$workflowDefinition.IsCI) {
                $getNamedWorkflowStep = {
                    param(
                        [Parameter(Mandatory)][string]$Text,
                        [Parameter(Mandatory)][string]$Name
                    )

                    $stepPattern = (
                        '(?ms)^      - name: ' + [regex]::Escape($Name) +
                        '\r?\n.*?(?=^      - name: |^  [A-Za-z0-9_-]+:\s*$|\z)'
                    )
                    $stepMatches = [regex]::Matches($Text, $stepPattern)
                    if ($stepMatches.Count -eq 1) {
                        return $stepMatches[0].Value
                    }
                    return ''
                }
                $testCIWorkflowContract = {
                    param([Parameter(Mandatory)][string]$Text)

                    $prepareStep = & $getNamedWorkflowStep `
                        -Text $Text `
                        -Name 'Prepare PowerShell 7.6.4'
                    $governanceStep = & $getNamedWorkflowStep `
                        -Text $Text `
                        -Name 'Bind governance to exact commit and PowerShell 7.6.4'
                    $pathWriteToken = '$installRoot | Out-File -FilePath $env:GITHUB_PATH -Encoding utf8 -Append'
                    $expectedPathToken = 'EXPECTED_PWSH_PATH: ${{ steps.pwsh764.outputs.pwsh_path }}'
                    $oldDynamicShellToken = 'shell: ${{ steps.pwsh764.outputs.pwsh_path }}'
                    $legacyPathComparisonPattern = (
                        '\$resolvedActualPwshPath\s+-(?:c)?ne\s+\$expectedPwshPath'
                    )
                    $pathWriteIndex = $prepareStep.IndexOf(
                        $pathWriteToken,
                        [System.StringComparison]::Ordinal
                    )
                    $hashGateIndex = $prepareStep.IndexOf(
                        'PowerShell package SHA-256 mismatch',
                        [System.StringComparison]::Ordinal
                    )
                    $extractIndex = $prepareStep.IndexOf(
                        'Expand-Archive',
                        [System.StringComparison]::Ordinal
                    )
                    $executableGateIndex = $prepareStep.IndexOf(
                        'Extracted PowerShell executable does not exist',
                        [System.StringComparison]::Ordinal
                    )
                    $preparePass = (
                        -not [string]::IsNullOrWhiteSpace($prepareStep) -and
                        $pathWriteIndex -gt $hashGateIndex -and
                        $pathWriteIndex -gt $extractIndex -and
                        $pathWriteIndex -gt $executableGateIndex
                    )
                    $governancePass = (
                        -not [string]::IsNullOrWhiteSpace($governanceStep) -and
                        [regex]::IsMatch($governanceStep, '(?m)^        shell: pwsh\s*$') -and
                        $governanceStep.Contains(
                            $expectedPathToken,
                            [System.StringComparison]::Ordinal
                        ) -and
                        $governanceStep.Contains(
                            '"PowerShellVersion: $powerShellVersion"',
                            [System.StringComparison]::Ordinal
                        ) -and
                        $governanceStep.Contains(
                            '"ExpectedPowerShellPath: $expectedPwshPath"',
                            [System.StringComparison]::Ordinal
                        ) -and
                        $governanceStep.Contains(
                            '"ActualPowerShellPath: $resolvedActualPwshPath"',
                            [System.StringComparison]::Ordinal
                        ) -and
                        $governanceStep.Contains(
                            '$expectedPwshPath = [System.IO.Path]::GetFullPath($env:EXPECTED_PWSH_PATH)',
                            [System.StringComparison]::Ordinal
                        ) -and
                        $governanceStep.Contains(
                            '$resolvedActualPwshPath = [System.IO.Path]::GetFullPath($actualPwshPath)',
                            [System.StringComparison]::Ordinal
                        ) -and
                        $governanceStep.Contains(
                            '[System.StringComparison]::OrdinalIgnoreCase',
                            [System.StringComparison]::Ordinal
                        ) -and
                        $governanceStep.Contains(
                            'if (-not $powerShellProcessPathParity)',
                            [System.StringComparison]::Ordinal
                        ) -and
                        $governanceStep.Contains(
                            '"Governance shell does not use the verified PowerShell executable."',
                            [System.StringComparison]::Ordinal
                        ) -and
                        $governanceStep.Contains(
                            "'PowerShellProcessPathParityPassed: true'",
                            [System.StringComparison]::Ordinal
                        ) -and
                        -not [regex]::IsMatch(
                            $governanceStep,
                            $legacyPathComparisonPattern
                        )
                    )
                    $oldDynamicShellAbsent = -not $Text.Contains(
                        $oldDynamicShellToken,
                        [System.StringComparison]::Ordinal
                    )
                    return [pscustomobject]@{
                        Passed = $preparePass -and $governancePass -and $oldDynamicShellAbsent
                        PrepareStep = $prepareStep
                        GovernanceStep = $governanceStep
                    }
                }
                $testWindowsSemanticPathParity = {
                    param(
                        [AllowEmptyString()][string]$ExpectedPath,
                        [AllowEmptyString()][string]$ActualPath
                    )

                    $absoluteWindowsPathPattern = (
                        '^[A-Za-z]:\\[^\\/:*?"<>|\r\n]+' +
                        '(?:\\[^\\/:*?"<>|\r\n]+)*$'
                    )
                    if ([string]::IsNullOrWhiteSpace($ExpectedPath) -or
                        [string]::IsNullOrWhiteSpace($ActualPath) -or
                        -not [regex]::IsMatch($ExpectedPath, $absoluteWindowsPathPattern) -or
                        -not [regex]::IsMatch($ActualPath, $absoluteWindowsPathPattern)) {
                        return $false
                    }
                    $expectedSegments = @($ExpectedPath.Substring(3).Split('\'))
                    $actualSegments = @($ActualPath.Substring(3).Split('\'))
                    if (
                        @($expectedSegments | Where-Object { $_ -in @('.', '..') }).Count -gt 0 -or
                        @($actualSegments | Where-Object { $_ -in @('.', '..') }).Count -gt 0
                    ) {
                        return $false
                    }
                    return [string]::Equals(
                        $ActualPath,
                        $ExpectedPath,
                        [System.StringComparison]::OrdinalIgnoreCase
                    )
                }

                $positiveWorkflowContract = & $testCIWorkflowContract -Text $workflowText
                if (-not $positiveWorkflowContract.Passed) {
                    $workflowContractFailures += 'corrected-workflow-contract'
                }
                $expectedWindowsPwshPath = 'D:\a\_temp\PowerShell-7.6.4-win-x64\pwsh.exe'
                if (-not (& $testWindowsSemanticPathParity `
                        -ExpectedPath $expectedWindowsPwshPath `
                        -ActualPath 'D:\a\_temp\PowerShell-7.6.4-win-x64\pwsh.EXE')) {
                    $workflowContractFailures += 'case-only-path-difference-not-accepted'
                }
                if (& $testWindowsSemanticPathParity `
                        -ExpectedPath $expectedWindowsPwshPath `
                        -ActualPath 'D:\a\_temp\PowerShell-7.6.4-other\pwsh.exe') {
                    $workflowContractFailures += 'different-directory-not-rejected'
                }
                if (& $testWindowsSemanticPathParity `
                        -ExpectedPath $expectedWindowsPwshPath `
                        -ActualPath 'D:\a\_temp\PowerShell-7.6.4-win-x64\powershell.exe') {
                    $workflowContractFailures += 'different-filename-not-rejected'
                }
                if (& $testWindowsSemanticPathParity `
                        -ExpectedPath $expectedWindowsPwshPath `
                        -ActualPath 'E:\a\_temp\PowerShell-7.6.4-win-x64\pwsh.exe') {
                    $workflowContractFailures += 'different-drive-not-rejected'
                }
                if (& $testWindowsSemanticPathParity `
                        -ExpectedPath $expectedWindowsPwshPath `
                        -ActualPath 'D:\a\_temp\PowerShell-7.6.4-win-x64\subdir\pwsh.exe') {
                    $workflowContractFailures += 'different-subdirectory-not-rejected'
                }
                if (& $testWindowsSemanticPathParity `
                        -ExpectedPath $expectedWindowsPwshPath `
                        -ActualPath '.\pwsh.exe') {
                    $workflowContractFailures += 'relative-path-not-rejected'
                }
                if (& $testWindowsSemanticPathParity `
                        -ExpectedPath $expectedWindowsPwshPath `
                        -ActualPath '') {
                    $workflowContractFailures += 'empty-path-not-rejected'
                }

                $missingPathWriteCandidate = $workflowText.Replace(
                    '$installRoot | Out-File -FilePath $env:GITHUB_PATH -Encoding utf8 -Append',
                    '$installRoot | Out-Null'
                )
                if ((& $testCIWorkflowContract -Text $missingPathWriteCandidate).Passed) {
                    $workflowContractFailures += 'missing-github-path-not-rejected'
                }

                $missingExpectedPathCandidate = $workflowText.Replace(
                    'EXPECTED_PWSH_PATH: ${{ steps.pwsh764.outputs.pwsh_path }}',
                    'REMOVED_EXPECTED_PWSH_PATH: true'
                )
                if ((& $testCIWorkflowContract -Text $missingExpectedPathCandidate).Passed) {
                    $workflowContractFailures += 'missing-expected-pwsh-path-not-rejected'
                }

                $dynamicGovernanceStep = $positiveWorkflowContract.GovernanceStep.Replace(
                    '        shell: pwsh',
                    '        shell: ${{ steps.pwsh764.outputs.pwsh_path }} -NoLogo -NoProfile -NonInteractive -File {0}'
                )
                $dynamicShellCandidate = $workflowText.Replace(
                    $positiveWorkflowContract.GovernanceStep,
                    $dynamicGovernanceStep
                )
                if ((& $testCIWorkflowContract -Text $dynamicShellCandidate).Passed) {
                    $workflowContractFailures += 'dynamic-shell-not-rejected'
                }

                $ordinalComparisonCandidate = $workflowText.Replace(
                    '[System.StringComparison]::OrdinalIgnoreCase',
                    '[System.StringComparison]::Ordinal'
                )
                if ((& $testCIWorkflowContract -Text $ordinalComparisonCandidate).Passed) {
                    $workflowContractFailures += 'case-sensitive-ordinal-not-rejected'
                }

                $legacyComparisonCandidate = $workflowText.Replace(
                    "'PowerShellProcessPathParityPassed: true'",
                    (
                        "'PowerShellProcessPathParityPassed: true'`r`n" +
                        '            # $resolvedActualPwshPath -cne $expectedPwshPath'
                    )
                )
                if ((& $testCIWorkflowContract -Text $legacyComparisonCandidate).Passed) {
                    $workflowContractFailures += 'case-sensitive-legacy-comparison-not-rejected'
                }
            }
            $missingWorkflowTokens = @($requiredWorkflowTokens | Where-Object {
                    -not $workflowText.Contains($_, [System.StringComparison]::Ordinal)
                })
            $workflowRecordPath = Join-Path $temporaryRoot ($workflowDefinition.Name + '.json')
            $workflowValidatorReportPath = Join-Path $temporaryRoot ($workflowDefinition.Name + '-validator-report.json')
            $generatorStderrPath = Join-Path $temporaryRoot ($workflowDefinition.Name + '-generator-stderr.log')
            $validatorStderrPath = Join-Path $temporaryRoot ($workflowDefinition.Name + '-validator-stderr.log')
            $hostedSources = @(
                "thomasweidner/flashgate-mcp@$actualHead",
                'github-actions-run:123'
            )
            $generatorOutput = @()
            $generatorStderr = ''
            $generatorExit = 1
            $validatorOutput = @()
            $validatorStderr = ''
            $validatorReport = $null
            $validatorReportReadFailure = $null
            $failedCheckDiagnostics = @()
            $workflowExit = 1
            if ($missingWorkflowTokens.Count -eq 0 -and $workflowContractFailures.Count -eq 0) {
                $generatorOutput = @(
                    & $generatorPath `
                        -OutputPath $workflowRecordPath `
                        -Checkpoint $workflowDefinition.Checkpoint `
                        -ChangedPath @($workflowDefinition.ChangedPath) `
                        -Repository 'https://github.com/thomasweidner/flashgate-mcp.git' `
                        -BaselineCommit $actualHead `
                        -CurrentCommit $actualHead `
                        -WorkflowCommit $actualHead `
                        -RunId 123 `
                        -RunAttempt 1 `
                        -Event $workflowDefinition.Event `
                        -Ref $workflowDefinition.Ref `
                        -HeadSha $actualHead 2> $generatorStderrPath |
                        Out-String -Stream
                )
                $generatorExit = $LASTEXITCODE
                if (Test-Path -LiteralPath $generatorStderrPath -PathType Leaf) {
                    $generatorStderr = [System.IO.File]::ReadAllText($generatorStderrPath)
                }
                if ($generatorExit -eq 0) {
                    if ([bool]$workflowDefinition.InjectInvalidRepeatedChecks) {
                        $invalidRecord = Get-Content -LiteralPath $workflowRecordPath -Raw -Encoding UTF8 |
                            ConvertFrom-Json -Depth 100 -DateKind String
                        $invalidRecord.repeatedChecks = $null
                        [System.IO.File]::WriteAllText(
                            $workflowRecordPath,
                            ($invalidRecord | ConvertTo-Json -Depth 100),
                            [System.Text.UTF8Encoding]::new($false)
                        )
                    }
                    $validatorOutput = @(
                        & $validatorPath `
                            -RepositoryRoot $resolvedRepositoryRoot `
                            -AssignmentRecordPath $workflowRecordPath `
                            -ChangedPath @($workflowDefinition.ChangedPath) `
                            -TrackedPath @($workflowDefinition.ChangedPath) `
                            -RuntimeCheckpoint $workflowDefinition.Checkpoint `
                            -HostedCISource $hostedSources `
                            -ExpectedRepository 'https://github.com/thomasweidner/flashgate-mcp.git' `
                            -ExpectedBaselineCommit $actualHead `
                            -ExpectedCurrentCommit $actualHead `
                            -ExpectedWorkflowCommit $actualHead `
                            -ExpectedRunId 123 `
                            -ExpectedRunAttempt 1 `
                            -ExpectedEvent $workflowDefinition.Event `
                            -ExpectedRef $workflowDefinition.Ref `
                            -ExpectedHeadSha $actualHead `
                            -ReportPath $workflowValidatorReportPath 2> $validatorStderrPath |
                            Out-String -Stream
                    )
                    $workflowExit = $LASTEXITCODE
                    if (Test-Path -LiteralPath $validatorStderrPath -PathType Leaf) {
                        $validatorStderr = [System.IO.File]::ReadAllText($validatorStderrPath)
                    }
                    if (Test-Path -LiteralPath $workflowValidatorReportPath -PathType Leaf) {
                        try {
                            $validatorReport = Get-Content -LiteralPath $workflowValidatorReportPath -Raw -Encoding UTF8 |
                                ConvertFrom-Json -Depth 100 -DateKind String
                            $failedCheckDiagnostics = @(
                                $validatorReport.checks |
                                    Where-Object Result -CEQ 'FAIL' |
                                    ForEach-Object {
                                        [ordered]@{
                                            CheckId = [string]$_.Id
                                            Condition = [string]$_.Message
                                            Evidence = [string]$_.Evidence
                                        }
                                    }
                            )
                        }
                        catch {
                            $validatorReportReadFailure = $_.Exception.Message
                        }
                    }
                }
            }
            $formatDataTypeNamePresent = (
                (($generatorOutput -join [Environment]::NewLine) -match 'Microsoft\.PowerShell\.Commands\.Internal\.Format') -or
                (($validatorOutput -join [Environment]::NewLine) -match 'Microsoft\.PowerShell\.Commands\.Internal\.Format')
            )
            $positiveSemanticResult = (
                [int]$workflowDefinition.ExpectedExit -eq 0 -and
                $generatorExit -eq 0 -and
                $workflowExit -eq 0 -and
                $null -ne $validatorReport -and
                [string]$validatorReport.status -ceq 'PASS' -and
                [int]$validatorReport.errorCount -eq 0 -and
                $failedCheckDiagnostics.Count -eq 0 -and
                -not $formatDataTypeNamePresent
            )
            $negativeDiagnosticResult = (
                [int]$workflowDefinition.ExpectedExit -eq 1 -and
                $generatorExit -eq 0 -and
                $workflowExit -eq 1 -and
                $null -ne $validatorReport -and
                [string]$validatorReport.status -ceq 'FAIL' -and
                [int]$validatorReport.errorCount -gt 0 -and
                'RECORD-JSON-SCHEMA' -cin @($failedCheckDiagnostics | ForEach-Object { [string]$_.CheckId }) -and
                'RECORD-repeatedChecks' -cin @($failedCheckDiagnostics | ForEach-Object { [string]$_.CheckId }) -and
                @($failedCheckDiagnostics | Where-Object { [string]::IsNullOrWhiteSpace([string]$_.Condition) }).Count -eq 0 -and
                @($failedCheckDiagnostics | Where-Object {
                        [string]$_.CheckId -ceq 'RECORD-JSON-SCHEMA' -and
                        [string]$_.Evidence -match '/repeatedChecks'
                    }).Count -eq 1 -and
                -not $formatDataTypeNamePresent
            )
            $workflowCasePassed = (
                $missingWorkflowTokens.Count -eq 0 -and
                $workflowContractFailures.Count -eq 0 -and
                [string]::IsNullOrWhiteSpace($validatorReportReadFailure) -and
                ($positiveSemanticResult -or $negativeDiagnosticResult)
            )
            $semanticDiagnosticPayload = [ordered]@{
                ExpectedExit = [int]$workflowDefinition.ExpectedExit
                ActualExit = $workflowExit
                GeneratorExit = $generatorExit
                ValidatorStatus = if ($null -ne $validatorReport) { [string]$validatorReport.status } else { $null }
                ValidatorErrorCount = if ($null -ne $validatorReport) { [int]$validatorReport.errorCount } else { $null }
                FailedChecks = @($failedCheckDiagnostics)
                GeneratorStdout = @($generatorOutput)
                GeneratorStderr = $generatorStderr
                ValidatorStdout = @($validatorOutput)
                ValidatorStderr = $validatorStderr
                ValidatorReportReadFailure = $validatorReportReadFailure
                FormatDataTypeNamePresent = $formatDataTypeNamePresent
                MissingTokens = @($missingWorkflowTokens)
                ContractFailures = @($workflowContractFailures)
            }
            [void]$results.Add([pscustomobject]@{
                Name = $workflowDefinition.Name
                ExpectedExit = [int]$workflowDefinition.ExpectedExit
                ActualExit = $workflowExit
                Result = if ($workflowCasePassed) { 'PASS' } else { 'FAIL' }
                Diagnostic = if ($workflowCasePassed -and [int]$workflowDefinition.ExpectedExit -eq 0) {
                    ''
                }
                else {
                    $semanticDiagnosticPayload | ConvertTo-Json -Depth 10 -Compress
                }
            })
            $lastCompletedFixture = $workflowDefinition.Name
            if (-not [string]::IsNullOrWhiteSpace($ProgressPath)) {
                [System.IO.File]::AppendAllText(
                    $ProgressPath,
                    (([ordered]@{
                        CompletedAt = [DateTimeOffset]::Now.ToString('o')
                        FixtureNumber = $results.Count
                        Name = $workflowDefinition.Name
                        Result = $results[$results.Count - 1].Result
                    } | ConvertTo-Json -Compress) + [Environment]::NewLine),
                    [System.Text.UTF8Encoding]::new($false)
                )
            }
        }
    }

    $completedFixtureNames = @($results | ForEach-Object { [string]$_.Name })
    if (($completedFixtureNames -join "`n") -cne ($selectedFixtureNames -join "`n")) {
        throw (
            'Completed fixture IDs do not match the canonical resolved selection. ' +
            "selected=$($selectedFixtureNames.Count); completed=$($completedFixtureNames.Count); " +
            "inventorySHA256=$fixtureInventorySHA256"
        )
    }
    if ($selectionIsFullInventory -and $results.Count -ne $canonicalFixtureCount) {
        throw (
            "Full fixture matrix produced $($results.Count) results; canonical inventory contains " +
            "$canonicalFixtureCount cases with SHA-256 $fixtureInventorySHA256."
        )
    }
    $status = if (@($results | Where-Object Result -eq 'FAIL').Count -eq 0) { 'PASS' } else { 'FAIL' }
}
catch {
    $status = 'FAIL'
    $failureMessage = $_.Exception.Message
}
finally {
    try {
        if ($null -ne $repositoryInternalPackagePath -and
            $null -ne $resolvedRepositoryRoot -and
            (Test-Path -LiteralPath $repositoryInternalPackagePath -PathType Leaf) -and
            [System.IO.Path]::GetDirectoryName([System.IO.Path]::GetFullPath($repositoryInternalPackagePath)) -ceq
                [System.IO.Path]::GetFullPath($resolvedRepositoryRoot).TrimEnd('\')) {
            Remove-Item -LiteralPath $repositoryInternalPackagePath -Force
        }
    }
    catch {
        $cleanupErrors.Add("Repository package cleanup failed: $($_.Exception.Message)")
    }
    try {
        if ($null -ne $temporaryRoot -and (Test-Path -LiteralPath $temporaryRoot -PathType Container)) {
            $resolvedTemporaryRoot = [System.IO.Path]::GetFullPath($temporaryRoot)
            $systemTemporaryRoot = [System.IO.Path]::GetFullPath([System.IO.Path]::GetTempPath())
            if ($resolvedTemporaryRoot.StartsWith($systemTemporaryRoot, [System.StringComparison]::OrdinalIgnoreCase) -and
                [System.IO.Path]::GetFileName($resolvedTemporaryRoot).StartsWith('flashgate-governance-fixtures-', [System.StringComparison]::Ordinal)) {
                Remove-Item -LiteralPath $resolvedTemporaryRoot -Recurse -Force
            }
        }
    }
    catch {
        $cleanupErrors.Add("Temporary root cleanup failed: $($_.Exception.Message)")
    }
    $cleanupStatus = if ($cleanupErrors.Count -eq 0) { 'PASS' } else { 'FAIL' }
}

if ($null -ne $resolvedRepositoryRoot -and $null -ne $repositoryStatusBefore) {
    $repositoryStatusAfter = @(& git -C $resolvedRepositoryRoot status --porcelain=v1 --untracked-files=all)
    if ($LASTEXITCODE -ne 0) {
        $repositoryMutationDetected = $true
        $cleanupErrors.Add('Cannot capture the repository status after fixture execution.')
    }
    else {
        $repositoryMutationDetected = (($repositoryStatusBefore -join "`n") -cne ($repositoryStatusAfter -join "`n"))
    }
}
if ($cleanupErrors.Count -gt 0 -or $repositoryMutationDetected) {
    $status = 'FAIL'
}

$completedAt = [DateTimeOffset]::Now
$resultFailureCount = @($results | Where-Object Result -eq 'FAIL').Count
$structuralFailureCount = if ([string]::IsNullOrWhiteSpace($failureMessage)) { 0 } else { 1 }
$warningCount = 0
$failureCount = $resultFailureCount + $structuralFailureCount + $cleanupErrors.Count + [int]$repositoryMutationDetected
$skippedCount = if ($selectionIsFullInventory) { [Math]::Max(0, $canonicalFixtureCount - $results.Count) } else { 0 }
$progressRecordCount = 0
$progressSHA256 = $null
if (-not [string]::IsNullOrWhiteSpace($ProgressPath) -and
    (Test-Path -LiteralPath $ProgressPath -PathType Leaf)) {
    $progressRecordCount = [System.IO.File]::ReadAllLines($ProgressPath).Count
    $progressSHA256 = (Get-FileHash -LiteralPath $ProgressPath -Algorithm SHA256).Hash.ToLowerInvariant()
}

$finalResult = [pscustomobject]@{
    Status       = $status
    StartedAt    = $startedAt.ToString('o')
    CompletedAt  = $completedAt.ToString('o')
    DurationSeconds = [Math]::Round(($completedAt - $startedAt).TotalSeconds, 3)
    CanonicalFixtureCount = $canonicalFixtureCount
    FixtureInventorySHA256 = $fixtureInventorySHA256
    FixtureInventory = @($canonicalFixtureNames)
    FixtureInventoryMetadata = @($canonicalFixtureInventory)
    SelectedFixtureCount = @($selectedFixtureNames).Count
    SelectedFixtureNames = @($selectedFixtureNames)
    SelectedFixtureMetadata = @($selectedFixtureMetadata)
    SelectionResolved = $selectionResolved
    FixtureProcessStarted = ($results.Count -gt 0)
    TargetPlatform = $TargetPlatform
    AvailableCapabilities = @($AvailableCapability | Sort-Object -Unique -CaseSensitive)
    ExpectedFixtureCount = @($selectedFixtureNames).Count
    ExecutedFixtureCount = $results.Count
    FixtureCount = $results.Count
    PassedCount  = @($results | Where-Object Result -eq 'PASS').Count
    FailedCount  = $resultFailureCount
    SkippedCount = $skippedCount
    WarningCount = $warningCount
    FailureCount = $failureCount
    ValidationExecutionCount = 1
    InfrastructureOrInvocationFailureCount = $structuralFailureCount
    FullMatrixRunCount = 0
    PackageWriteAttemptCount = 0
    GeneratedTaskControllerFileCount = 0
    GeneratedTaskControllerLineCount = 0
    ReadOnlyProbeCount = 0
    CleanupStatus = $cleanupStatus
    CleanupErrors = $cleanupErrors -join [Environment]::NewLine
    RepositoryMutationDetected = $repositoryMutationDetected
    LastCompletedFixture = $lastCompletedFixture
    ProgressPath = $ProgressPath
    ProgressRecordCount = $progressRecordCount
    ProgressSHA256 = $progressSHA256
    Failures     = @($results | Where-Object Result -eq 'FAIL' | ForEach-Object { "$($_.Name): expected $($_.ExpectedExit), actual $($_.ActualExit); $($_.Diagnostic)" }) -join [Environment]::NewLine
    FailureMessage = $failureMessage
    NextAction   = if ($status -eq 'PASS') { 'Use the governance validator in CI and commit preparation.' } else { 'Correct the failing fixture cases and rerun the matrix.' }
}

if (-not [string]::IsNullOrWhiteSpace($ResultPath)) {
    $temporaryResultPath = "$ResultPath.$PID.$([guid]::NewGuid().ToString('N')).tmp"
    try {
        [System.IO.File]::WriteAllText(
            $temporaryResultPath,
            (($finalResult | ConvertTo-Json -Depth 8) + [Environment]::NewLine),
            [System.Text.UTF8Encoding]::new($false)
        )
        [System.IO.File]::Move($temporaryResultPath, $ResultPath)
    }
    finally {
        if (Test-Path -LiteralPath $temporaryResultPath -PathType Leaf) {
            Remove-Item -LiteralPath $temporaryResultPath -Force
        }
    }
}

$finalResult | Format-List

if ($status -eq 'PASS') {
    exit 0
}
exit 1
