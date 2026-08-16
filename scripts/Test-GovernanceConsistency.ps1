[CmdletBinding()]
param(
    [string]$RepositoryRoot = (Split-Path -Parent $PSScriptRoot),
    [string]$CatalogPath,
    [string]$AssignmentRecordPath,
    [string]$CompletionReportPath,
    [string]$ScopeInventoryPath,
    [string[]]$ChangedPath = @(),
    [string[]]$TrackedPath = @(),
    [ValidateSet('', 'ASSIGNMENT_START', 'MATERIAL_SCOPE_CHANGE', 'PRE_COMMIT', 'SPRINT_CLOSE', 'RELEASE_CANDIDATE', 'STABLE_RELEASE')]
    [string]$RuntimeCheckpoint = '',
    [string[]]$HostedCISource = @(),
    [string]$ExpectedRepository,
    [string]$ExpectedBaselineCommit,
    [string]$ExpectedCurrentCommit,
    [string]$ExpectedWorkflowCommit,
    [string]$ExpectedRunId,
    [int]$ExpectedRunAttempt,
    [string]$ExpectedEvent,
    [string]$ExpectedRef,
    [string]$ExpectedHeadSha,
    [string]$ExpectedPowerShellVersion,
    [string]$PowerShellPackagePath,
    [string]$ExpectedPowerShellPackageSha256,
    [string]$ExpectedPriorReviewBaselineSha256,
    [string]$ExpectedCorrectionPatchSha256,
    [string]$ExpectedCurrentDeltaSha256,
    [string]$ExpectedCorrectionStartCommit,
    [string]$HandoffPackagePath,
    [string]$GenericHandoffPackagePath,
    [string]$CanonicalArtifactValidatorPath,
    [string]$ExpectedCanonicalArtifactValidatorSha256,
    [string]$ReportPath
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$script:Checks = [System.Collections.Generic.List[object]]::new()
$script:Errors = [System.Collections.Generic.List[object]]::new()

function Add-GovernanceCheck {
    param(
        [Parameter(Mandatory)][string]$Id,
        [Parameter(Mandatory)][bool]$Passed,
        [Parameter(Mandatory)][string]$Message,
        [AllowEmptyString()][string]$Evidence = ''
    )

    $entry = [pscustomobject]@{
        Id       = $Id
        Result   = if ($Passed) { 'PASS' } else { 'FAIL' }
        Message  = $Message
        Evidence = $Evidence
    }
    [void]$script:Checks.Add($entry)
    if (-not $Passed) {
        [void]$script:Errors.Add($entry)
    }
}

function Read-StrictJson {
    param([Parameter(Mandatory)][string]$LiteralPath)

    $encoding = [System.Text.UTF8Encoding]::new($false, $true)
    $text = [System.IO.File]::ReadAllText($LiteralPath, $encoding)
    if ($text.Contains([char]0xFFFD)) {
        throw "Stored U+FFFD is not allowed: $LiteralPath"
    }

    return $text | ConvertFrom-Json -Depth 100 -DateKind String
}

function Get-LowerSha256 {
    param([Parameter(Mandatory)][byte[]]$Bytes)

    return [Convert]::ToHexString(
        [System.Security.Cryptography.SHA256]::HashData($Bytes)
    ).ToLowerInvariant()
}

function Test-OrdinalSetEqual {
    param(
        [AllowNull()][object[]]$Left,
        [AllowNull()][object[]]$Right
    )

    [string[]]$leftValues = @($Left | ForEach-Object { [string]$_ })
    [string[]]$rightValues = @($Right | ForEach-Object { [string]$_ })
    [array]::Sort($leftValues, [System.StringComparer]::Ordinal)
    [array]::Sort($rightValues, [System.StringComparer]::Ordinal)
    return [string]::Equals(
        ($leftValues -join "`n"),
        ($rightValues -join "`n"),
        [System.StringComparison]::Ordinal
    )
}

function Test-OrdinalSequenceEqual {
    param(
        [AllowNull()][object[]]$Left,
        [AllowNull()][object[]]$Right
    )

    [string[]]$leftValues = @($Left | ForEach-Object { [string]$_ })
    [string[]]$rightValues = @($Right | ForEach-Object { [string]$_ })
    return [string]::Equals(
        ($leftValues -join "`u{0000}"),
        ($rightValues -join "`u{0000}"),
        [System.StringComparison]::Ordinal
    )
}

function Get-NormalizedWindowsGovernancePath {
    param([AllowNull()][string]$Path)

    if ([string]::IsNullOrWhiteSpace($Path) -or
        -not [System.IO.Path]::IsPathFullyQualified($Path) -or
        $Path.StartsWith('\\', [System.StringComparison]::Ordinal) -or
        $Path.Contains('/', [System.StringComparison]::Ordinal) -or
        $Path -match '[\x00-\x1f\x7f]') {
        return $null
    }
    try {
        return [System.IO.Path]::GetFullPath($Path).
            TrimEnd([System.IO.Path]::DirectorySeparatorChar).
            ToUpperInvariant()
    }
    catch {
        return $null
    }
}

function Get-ExpectedExternalGovernanceMappings {
    return @(
        [pscustomobject]@{
            ActivePath = 'C:\Users\ThomasW\.codex\AGENTS.md'
            Scope = 'GLOBAL_CODEX_RULES'
            PayloadRoot = 'external/global-codex-agents'
        },
        [pscustomobject]@{
            ActivePath = 'C:\Voxtronic\AGENTS.md'
            Scope = 'VOXTRONIC_WORKSPACE_RULES'
            PayloadRoot = 'external/workspace-agents'
        },
        [pscustomobject]@{
            ActivePath = 'C:\Users\ThomasW\OneDrive - VOXTRONIC\Desktop\Voxtronic\Codex-Work\PROJECT-WIDE-REVIEW-AND-VALIDATION-STANDARD.md'
            Scope = 'PROJECT_WIDE_ROOT_STANDARD'
            PayloadRoot = 'external/project-wide-root-standard'
        },
        [pscustomobject]@{
            ActivePath = 'C:\Users\ThomasW\OneDrive - VOXTRONIC\Desktop\Voxtronic\Codex-Work\Governance\PROJECT-WIDE-REVIEW-AND-VALIDATION-STANDARD.md'
            Scope = 'PROJECT_WIDE_GOVERNANCE_STANDARD'
            PayloadRoot = 'external/project-wide-governance-standard'
        },
        [pscustomobject]@{
            ActivePath = 'C:\Voxtronic\MCP\flashgate-mcp-local-work-register.md'
            Scope = 'FLASHGATE_LOCAL_WORK_REGISTER'
            PayloadRoot = 'external/local-work-register'
        }
    )
}

function Test-HandoffDocumentContracts {
    param(
        [Parameter(Mandatory)][string]$Text,
        [Parameter(Mandatory)][string]$ContractSchemaPath
    )

    $statusBeginText = '<!-- BEGIN GOVERNANCE-HANDOFF-STATUS -->'
    $statusEndText = '<!-- END GOVERNANCE-HANDOFF-STATUS -->'
    $contractBeginText = '<!-- BEGIN GOVERNANCE-HANDOFF-CONTRACT -->'
    $contractEndText = '<!-- END GOVERNANCE-HANDOFF-CONTRACT -->'
    $statusBegin = @([regex]::Matches($Text, '(?m)^' + [regex]::Escape($statusBeginText) + '\r?$'))
    $statusEnd = @([regex]::Matches($Text, '(?m)^' + [regex]::Escape($statusEndText) + '\r?$'))
    $contractBegin = @([regex]::Matches($Text, '(?m)^' + [regex]::Escape($contractBeginText) + '\r?$'))
    $contractEnd = @([regex]::Matches($Text, '(?m)^' + [regex]::Escape($contractEndText) + '\r?$'))
    $markerCountsPass = (
        $statusBegin.Count -eq 1 -and
        $statusEnd.Count -eq 1 -and
        $contractBegin.Count -eq 1 -and
        $contractEnd.Count -eq 1
    )
    $markerOrderPass = (
        $markerCountsPass -and
        $statusBegin[0].Index -lt $statusEnd[0].Index -and
        $statusEnd[0].Index -lt $contractBegin[0].Index -and
        $contractBegin[0].Index -lt $contractEnd[0].Index
    )
    $statusBlockMatch = [regex]::Match(
        $Text,
        '(?ms)^' + [regex]::Escape($statusBeginText) +
            '\r?\n(?<body>.*?)\r?\n' + [regex]::Escape($statusEndText) + '\r?$'
    )
    $contractBlockMatch = [regex]::Match(
        $Text,
        '(?ms)^' + [regex]::Escape($contractBeginText) +
            '\r?\n(?<json>\{.*?\})\r?\n' + [regex]::Escape($contractEndText) + '\r?$'
    )
    $markerGatePassed = (
        $markerCountsPass -and
        $markerOrderPass -and
        $statusBlockMatch.Success -and
        $contractBlockMatch.Success
    )
    Add-GovernanceCheck -Id 'RECORD-HANDOFF-MARKER-COUNTS' `
        -Passed $markerGatePassed `
        -Message 'Status and contract start/end markers occur exactly once, are correctly ordered, and form non-overlapping pairs.' `
        -Evidence "statusBegin=$($statusBegin.Count); statusEnd=$($statusEnd.Count); contractBegin=$($contractBegin.Count); contractEnd=$($contractEnd.Count)"

    $requiredKeys = @(
        'Status',
        'CorrectionMode',
        'TargetFindingCount',
        'CorrectedFindingCount',
        'PendingDeltaFindingCount',
        'ClosedFindingCount',
        'OpenFindingCount',
        'ClassicReviewReady',
        'TargetFindings',
        'PendingFindings',
        'ClosedFindings',
        'Run007Status',
        'CommitPreparationApproved',
        'CommitAuthorized',
        'RequiredReviewMode',
        'NextAction'
    )
    $integerKeys = @(
        'TargetFindingCount',
        'CorrectedFindingCount',
        'PendingDeltaFindingCount',
        'ClosedFindingCount',
        'OpenFindingCount'
    )
    $booleanKeys = @(
        'ClassicReviewReady',
        'CommitPreparationApproved',
        'CommitAuthorized'
    )
    $listKeys = @('TargetFindings', 'PendingFindings', 'ClosedFindings')
    $visibleValues = [ordered]@{}
    $keyGatePassed = $markerGatePassed
    if ($statusBlockMatch.Success) {
        $statusLines = @($statusBlockMatch.Groups['body'].Value -split '\r?\n')
        $keyGatePassed = $keyGatePassed -and $statusLines.Count -eq $requiredKeys.Count
        foreach ($line in $statusLines) {
            $lineMatch = [regex]::Match($line, '^(?<key>[A-Za-z][A-Za-z0-9]*): (?<value>.+)$')
            if (-not $lineMatch.Success) {
                $keyGatePassed = $false
                continue
            }
            $key = $lineMatch.Groups['key'].Value
            $value = $lineMatch.Groups['value'].Value
            if ($key -notin $requiredKeys -or $visibleValues.Contains($key)) {
                $keyGatePassed = $false
                continue
            }
            if ($key -in $integerKeys) {
                if ($value -notmatch '^(?:0|[1-9][0-9]*)$') {
                    $keyGatePassed = $false
                    continue
                }
                $visibleValues[$key] = [int]$value
            }
            elseif ($key -in $booleanKeys) {
                if ($value -cnotin @('true', 'false')) {
                    $keyGatePassed = $false
                    continue
                }
                $visibleValues[$key] = $value -ceq 'true'
            }
            elseif ($key -in $listKeys) {
                $items = @($value.Split(','))
                if ($items.Count -eq 0 -or
                    @($items | Where-Object { [string]::IsNullOrEmpty($_) }).Count -gt 0 -or
                    @($items | Sort-Object -Unique).Count -ne $items.Count) {
                    $keyGatePassed = $false
                    continue
                }
                $visibleValues[$key] = $items
            }
            else {
                $visibleValues[$key] = $value
            }
        }
        $keyGatePassed = (
            $keyGatePassed -and
            (Test-OrdinalSequenceEqual -Left @($visibleValues.Keys) -Right $requiredKeys)
        )
    }
    Add-GovernanceCheck -Id 'RECORD-HANDOFF-VISIBLE-KEYS' `
        -Passed $keyGatePassed `
        -Message 'Visible status block contains exactly the 16 canonical keys once, in order, with strict syntax and typed non-empty values.'

    $outsideStatusText = if ($statusBlockMatch.Success) {
        $Text.Remove($statusBlockMatch.Index, $statusBlockMatch.Length)
    }
    else {
        $Text
    }
    $reservedPattern = '(?m)^(?:' + (($requiredKeys | ForEach-Object {
                    [regex]::Escape($_)
                }) -join '|') + '):'
    $outsideReservedLines = @([regex]::Matches($outsideStatusText, $reservedPattern))
    $reservedControlLineGatePassed = $outsideReservedLines.Count -eq 0
    Add-GovernanceCheck -Id 'RECORD-HANDOFF-RESERVED-CONTROL-LINES' `
        -Passed $reservedControlLineGatePassed `
        -Message 'No reserved key-value control line occurs outside the bounded visible status block.' `
        -Evidence "outsideReservedLineCount=$($outsideReservedLines.Count)"

    $contract = $null
    $contractSchemaPass = $false
    if ($markerGatePassed -and $contractBlockMatch.Success) {
        $contractText = $contractBlockMatch.Groups['json'].Value
        $contractErrors = @()
        $contractSchemaPass = $contractText |
            Test-Json -SchemaFile $ContractSchemaPath `
                -ErrorVariable contractErrors -ErrorAction SilentlyContinue
        Add-GovernanceCheck -Id 'RECORD-HANDOFF-CONTRACT-SCHEMA' `
            -Passed $contractSchemaPass `
            -Message 'HANDOFF contract passes its strict schema.' `
            -Evidence (@($contractErrors | ForEach-Object ToString) -join ' | ')
        if ($contractSchemaPass) {
            $contract = $contractText | ConvertFrom-Json -Depth 100 -DateKind String
        }
    }
    else {
        Add-GovernanceCheck -Id 'RECORD-HANDOFF-CONTRACT-SCHEMA' `
            -Passed $false `
            -Message 'HANDOFF contract passes its strict schema.'
    }

    $visibleParityGatePassed = $keyGatePassed -and $null -ne $contract
    if ($visibleParityGatePassed) {
        $visibleParityGatePassed = (
            [string]$visibleValues.Status -ceq [string]$contract.status -and
            [string]$visibleValues.CorrectionMode -ceq [string]$contract.correctionMode -and
            [int]$visibleValues.TargetFindingCount -eq [int]$contract.targetFindingCount -and
            [int]$visibleValues.TargetFindingCount -eq @($contract.targetFindings).Count -and
            [int]$visibleValues.CorrectedFindingCount -eq [int]$contract.correctedFindingCount -and
            [int]$visibleValues.PendingDeltaFindingCount -eq [int]$contract.pendingDeltaFindingCount -and
            [int]$visibleValues.PendingDeltaFindingCount -eq @($contract.pendingFindings).Count -and
            [int]$visibleValues.ClosedFindingCount -eq [int]$contract.closedFindingCount -and
            [int]$visibleValues.ClosedFindingCount -eq @($contract.closedFindings).Count -and
            [int]$visibleValues.OpenFindingCount -eq [int]$contract.openFindingCount -and
            [bool]$visibleValues.ClassicReviewReady -eq [bool]$contract.classicReviewReady -and
            (Test-OrdinalSequenceEqual $visibleValues.TargetFindings $contract.targetFindings) -and
            (Test-OrdinalSequenceEqual $visibleValues.PendingFindings $contract.pendingFindings) -and
            (Test-OrdinalSequenceEqual $visibleValues.ClosedFindings $contract.closedFindings) -and
            [string]$visibleValues.Run007Status -ceq [string]$contract.run007Status -and
            [bool]$visibleValues.CommitPreparationApproved -eq
                [bool]$contract.commitPreparationApproved -and
            [bool]$visibleValues.CommitAuthorized -eq [bool]$contract.commitAuthorized -and
            [string]$visibleValues.RequiredReviewMode -ceq
                [string]$contract.requiredReviewMode -and
            [string]$visibleValues.NextAction -ceq [string]$contract.nextAction
        )
    }
    Add-GovernanceCheck -Id 'RECORD-HANDOFF-VISIBLE-CONTRACT-PARITY' `
        -Passed $visibleParityGatePassed `
        -Message 'Every typed visible HANDOFF value exactly equals its JSON contract field or bounded derivation.'

    return [pscustomobject]@{
        Contract = $contract
        ContractSchemaPass = $contractSchemaPass
        MarkerCountGatePassed = $markerGatePassed
        VisibleKeyGatePassed = $keyGatePassed
        VisibleParityGatePassed = $visibleParityGatePassed
        ReservedControlLineGatePassed = $reservedControlLineGatePassed
        VisibleValues = $visibleValues
    }
}

function Get-PatchPaths {
    param([Parameter(Mandatory)][string]$PatchText)

    return @(
        [regex]::Matches(
            $PatchText,
            '(?m)^diff --git a/(?<path>[^\r\n]+) b/\k<path>$'
        ) |
            ForEach-Object { $_.Groups['path'].Value } |
            Sort-Object -Unique
    )
}

function Get-PropertyNames {
    param([Parameter(Mandatory)][object]$Value)

    return @($Value.PSObject.Properties.Name)
}

function Test-CanonicalRepositoryPath {
    param([AllowNull()][string]$Path)

    if ([string]::IsNullOrWhiteSpace($Path)) {
        return $false
    }
    if (-not [string]::Equals(
            $Path,
            $Path.Normalize([System.Text.NormalizationForm]::FormC),
            [System.StringComparison]::Ordinal
        )) {
        return $false
    }
    if ($Path.Contains('\') -or $Path.StartsWith('/', [System.StringComparison]::Ordinal) -or
        $Path -match '^[A-Za-z]:' -or $Path.StartsWith('//', [System.StringComparison]::Ordinal) -or
        [System.IO.Path]::IsPathRooted($Path)) {
        return $false
    }
    if (@($Path.ToCharArray() | Where-Object { [int]$_ -lt 0x20 -or [int]$_ -eq 0x7F }).Count -gt 0) {
        return $false
    }

    $segments = @($Path.Split('/'))
    if ($segments.Count -eq 0 -or
        @($segments | Where-Object { $_ -in @('', '.', '..') }).Count -gt 0) {
        return $false
    }

    return [string]::Equals(
        ($segments -join '/'),
        $Path,
        [System.StringComparison]::Ordinal
    )
}

function Get-CanonicalModeDefinitions {
    return [ordered]@{
        INDEPENDENT_REVIEW = [ordered]@{
            independent = $true
            repositoryMutationAllowed = $false
            externalMutationAllowed = $false
            reviewScope = 'FULL_INTEGRATION'
            commitAllowed = $false
            correctionAllowed = $false
            requiresPriorReview = $false
            requiresFocusedDeltaReview = $false
        }
        BUNDLED_CORRECTION = [ordered]@{
            independent = $false
            repositoryMutationAllowed = $true
            externalMutationAllowed = $true
            reviewScope = 'APPROVED_CORRECTION_SCOPE'
            commitAllowed = $false
            correctionAllowed = $true
            requiresPriorReview = $true
            requiresFocusedDeltaReview = $true
        }
        FOCUSED_INDEPENDENT_DELTA_REVIEW = [ordered]@{
            independent = $true
            repositoryMutationAllowed = $false
            externalMutationAllowed = $false
            reviewScope = 'CORRECTION_DELTA'
            commitAllowed = $false
            correctionAllowed = $false
            requiresPriorReview = $true
            requiresFocusedDeltaReview = $false
        }
        COMMIT_PREPARATION = [ordered]@{
            independent = $false
            repositoryMutationAllowed = $false
            externalMutationAllowed = $false
            reviewScope = 'COMMIT_SCOPE'
            commitAllowed = $false
            correctionAllowed = $false
            requiresPriorReview = $true
            requiresFocusedDeltaReview = $true
        }
    }
}

function Get-CanonicalOperatingPolicies {
    return [ordered]@{
        remediationPolicy = [ordered]@{
            newOrMateriallyRebuiltArtifactCycleBudget = 12
            establishedValidatedArtifactCycleBudget = 6
            automaticRetryBudgetAfterFirstProductiveWrite = 0
            mutationAttemptBoundary = 'BEFORE_FIRST_PRODUCTIVE_WRITE_CAPABLE_OPERATION'
            directlyCorrectableDefectClasses = @(
                'HARNESS',
                'FIXTURE',
                'PARSER',
                'INSTRUMENTATION',
                'DIAGNOSTIC',
                'CLASSIFICATION'
            )
            classicReturnRequiredOnlyAtDecisionAuthorizationScopeOrBudgetBoundary = $true
        }
        activityGatePolicy = [ordered]@{
            finalGateRequiredImmediatelyBeforeFirstProductiveWrite = $true
            externalMonitorMustBeStoppedOrPlaintextRedacted = $true
            monitoredPlaintextPathsAndObjectNamesForbiddenInConcurrentControlState = $true
            timeVaryingChecksAllowedBetweenFinalGateAndWrite = $false
        }
        validationExecutionPolicy = [ordered]@{
            resultStatuses = @('PASS', 'FAIL', 'SKIPPED', 'BLOCKED', 'CANCELLED', 'PENDING', 'NOT_RUN')
            progressSemantics = 'completed/selected unit - Phase: phase'
            warningInvariant = 'observedWarningCount = resolvedWarningCount + openWarningCount; warningCount = openWarningCount'
            counterFields = @(
                'materialCorrectionCycleCount',
                'validationExecutionCount',
                'infrastructureOrInvocationFailureCount',
                'fullMatrixRunCount',
                'packageWriteAttemptCount',
                'generatedTaskControllerFileCount',
                'generatedTaskControllerLineCount',
                'readOnlyProbeCount'
            )
            requiredPreflightBindings = @('repositoryIdentity', 'commitAndBranch', 'completeStatus', 'scopeAndIds', 'parallelWorktrees')
            finalFullValidationRunLimit = 1
            resumeRequiresHashBoundState = $true
            failureStatuses = @('FAIL')
            blockedIncrementsFailureCount = $false
            terminalProgressEventPerCaseRequired = $true
            minimumProgressEventFields = @('sequence', 'caseId', 'eventType', 'status', 'completed', 'selected', 'unit', 'phase', 'elapsedMilliseconds')
            identicalEventSuppressionRequired = $true
            heartbeatIntervalRequired = $true
            heartbeatExplicitlyTyped = $true
            allowedProgressEmissionReasons = @('PROGRESS', 'PHASE_CHANGE', 'STATUS_CHANGE', 'HEARTBEAT')
            missingProgressInstrumentationCreatesFinding = $true
            selectionResolvedBeforeRunnerStart = $true
            selectorResolutionCardinality = 1
            unknownDuplicateAmbiguousSelectionFailsClosed = $true
            canonicalCaseInventoryFormat = 'MACHINE_READABLE_NON_SHELL'
            singleCaseMetadataSourceRequired = $true
            structuredSelectionDiagnosticIdsRequired = $true
            explicitSourceAndWorktreeParametersRequired = $true
            hardcodedMainWorktreeAllowed = $false
            requiredSourceBindings = @(
                'branch',
                'commit',
                'tree',
                'expectedStatusSha256',
                'scopePaths',
                'fileHashes',
                'protectedWorktrees'
            )
            nativeEvidenceMustMatchDeltaWorktree = $true
            standardProbeClasses = @('GIT', 'POWERSHELL')
            helperCommandShadowingAllowed = $false
            detachedHeadDetection = 'SYMBOLIC_REF_EXIT_CODE'
            directExitCodeEvaluationRequired = $true
            timeoutBudgetFromProbeCountAndMeasuredRuntimeRequired = $true
            knownNormalUserContextSkipsSandboxAttempt = $true
            blockedAndFailSemanticallyDistinct = $true
            historicalAndCurrentContractVersionsSeparated = $true
            scopeOverrunFailsClosed = $true
        }
        classicHandoffPolicy = [ordered]@{
            singleRequiredFileDirectTransferAllowed = $true
            multipleRequiredFilesRequireExactlyOneZip = $true
            separatePackageMemberTransferAllowed = $false
            freshFullRebuildRequiredAfterArtifactChange = $true
            manifestCoverageRequired = $true
            sha256ValidationRequired = $true
            rejectUnsafeDuplicateOrCaseCollidingPaths = $true
            rejectLinkJunctionOrReparseTargets = $true
            classicReviewReadyRequiresCompletePackage = $true
            zipFreeReadinessRequired = $true
            inPlaceFinalZipRepairAllowed = $false
            invalidFinalZipDisposition = 'DISCARD'
            correctedPackageRequiresFreshStaging = $true
        }
    }
}

function Test-RequiredProperties {
    param(
        [Parameter(Mandatory)][object]$Value,
        [Parameter(Mandatory)][string[]]$Required,
        [Parameter(Mandatory)][string]$Prefix
    )

    $names = @(Get-PropertyNames -Value $Value)
    foreach ($name in $Required) {
        Add-GovernanceCheck -Id "$Prefix-$name" -Passed ($name -in $names) -Message "Required property exists: $name"
    }
}

function Test-UniqueScalarArray {
    param(
        [AllowNull()][object[]]$Value,
        [Parameter(Mandatory)][string]$Id
    )

    $items = @($Value)
    $valid = @($items | Where-Object { $_ -is [string] -and -not [string]::IsNullOrWhiteSpace([string]$_) }).Count -eq $items.Count
    $unique = @($items | Sort-Object -Unique).Count -eq $items.Count
    Add-GovernanceCheck -Id $Id -Passed ($valid -and $unique) -Message 'Array contains unique non-empty scalar strings.' -Evidence ($items -join ', ')
}

function Convert-GlobToRegex {
    param([Parameter(Mandatory)][string]$Pattern)

    $normalized = $Pattern.Replace('\', '/')
    $escaped = [regex]::Escape($normalized)
    $escaped = $escaped.Replace('\*\*', '.*')
    $escaped = $escaped.Replace('\*', '[^/]*')
    return '^' + $escaped + '$'
}

function Get-DerivedTriggers {
    param(
        [Parameter(Mandatory)][object]$Catalog,
        [Parameter(Mandatory)][AllowEmptyCollection()][string[]]$Paths
    )

    $derived = [System.Collections.Generic.List[string]]::new()
    foreach ($rawPath in $Paths) {
        $pathId = ([Convert]::ToHexString(
                [System.Security.Cryptography.SHA256]::HashData(
                    [System.Text.Encoding]::UTF8.GetBytes([string]$rawPath)
                )
            )).Substring(0, 12)
        $pathValid = Test-CanonicalRepositoryPath -Path $rawPath
        Add-GovernanceCheck -Id "PATH-CANONICAL-$pathId" -Passed $pathValid `
            -Message 'Changed path is a canonical repository-relative path.' -Evidence ([string]$rawPath)
        if (-not $pathValid) {
            continue
        }
        $path = [string]$rawPath
        foreach ($trigger in @($Catalog.triggers)) {
            if ('DIFF' -notin @($trigger.sources)) {
                continue
            }
            foreach ($pattern in @($trigger.pathPatterns)) {
                if ($path -match (Convert-GlobToRegex -Pattern ([string]$pattern))) {
                    if ([string]$trigger.id -notin $derived) {
                        [void]$derived.Add([string]$trigger.id)
                    }
                    break
                }
            }
        }
    }
    return @($derived | Sort-Object)
}

function Test-TrackedPathCoverage {
    param(
        [Parameter(Mandatory)][object]$Catalog,
        [Parameter(Mandatory)][AllowEmptyCollection()][string[]]$Paths
    )

    $exclusions = @($Catalog.trackedPathExclusions)
    foreach ($exclusion in $exclusions) {
        Test-RequiredProperties -Value $exclusion -Required @('pattern', 'reason') -Prefix 'CATALOG-EXCLUSION'
        Add-GovernanceCheck -Id ('CATALOG-EXCLUSION-REASON-' + [string]$exclusion.pattern) `
            -Passed (-not [string]::IsNullOrWhiteSpace([string]$exclusion.reason)) `
            -Message 'Tracked-path exclusion has a non-empty reason.'
    }

    foreach ($path in @($Paths | Sort-Object -Unique)) {
        $pathValid = Test-CanonicalRepositoryPath -Path $path
        if (-not $pathValid) {
            Add-GovernanceCheck -Id 'TRACKED-PATH-CANONICAL' -Passed $false `
                -Message 'Tracked path is canonical.' -Evidence $path
            continue
        }
        $triggerIdsForPath = @(Get-DerivedTriggers -Catalog $Catalog -Paths @($path))
        $matchingExclusions = @($exclusions | Where-Object {
                $path -match (Convert-GlobToRegex -Pattern ([string]$_.pattern))
            })
        $covered = $triggerIdsForPath.Count -gt 0 -or $matchingExclusions.Count -eq 1
        Add-GovernanceCheck -Id ('TRACKED-PATH-' + ([Convert]::ToHexString(
                    [System.Security.Cryptography.SHA256]::HashData(
                        [System.Text.Encoding]::UTF8.GetBytes($path)
                    )
                )).Substring(0, 12)) -Passed $covered `
            -Message 'Tracked path maps to a material trigger or one explicit exclusion.' `
            -Evidence ('{0} => {1}' -f $path, (@($triggerIdsForPath) | ConvertTo-Json -Compress))
    }
}

function Test-ExternalDeltaPayload {
    param([Parameter(Mandatory)][string]$PackagePath)

    Add-Type -AssemblyName System.IO.Compression
    $stream = [System.IO.File]::OpenRead($PackagePath)
    try {
        $archive = [System.IO.Compression.ZipArchive]::new(
            $stream,
            [System.IO.Compression.ZipArchiveMode]::Read,
            $false
        )
        try {
            $entries = @($archive.Entries)
            $manifestEntries = @($entries | Where-Object FullName -ceq 'external-governance-manifest.json')
            Add-GovernanceCheck -Id 'EXTERNAL-DELTA-MANIFEST-UNIQUE' `
                -Passed ($manifestEntries.Count -eq 1) `
                -Message 'Package contains exactly one external-governance manifest.'
            if ($manifestEntries.Count -ne 1) {
                return
            }

            $reader = [System.IO.StreamReader]::new(
                $manifestEntries[0].Open(),
                [System.Text.UTF8Encoding]::new($false, $true)
            )
            try {
                $manifest = $reader.ReadToEnd() | ConvertFrom-Json -Depth 100 -DateKind String
            }
            finally {
                $reader.Dispose()
            }

            Test-RequiredProperties -Value $manifest -Required @('schemaVersion', 'changes') `
                -Prefix 'EXTERNAL-DELTA-MANIFEST'
            Add-GovernanceCheck -Id 'EXTERNAL-DELTA-MANIFEST-NONEMPTY' `
                -Passed (@($manifest.changes).Count -gt 0) `
                -Message 'External-governance manifest contains reviewable changes.'

            $expectedMappings = @(Get-ExpectedExternalGovernanceMappings)
            $actualChanges = @($manifest.changes)
            $normalizedExpectedPaths = @(
                $expectedMappings | ForEach-Object {
                    Get-NormalizedWindowsGovernancePath -Path $_.ActivePath
                }
            )
            $normalizedActualPaths = @(
                $actualChanges | ForEach-Object {
                    Get-NormalizedWindowsGovernancePath -Path ([string]$_.activePath)
                }
            )
            $actualScopes = @($actualChanges | ForEach-Object { [string]$_.scope })
            $expectedScopes = @($expectedMappings | ForEach-Object { [string]$_.Scope })
            $exactPathSetPass = Test-OrdinalSetEqual `
                -Left @($actualChanges | ForEach-Object { [string]$_.activePath }) `
                -Right @($expectedMappings | ForEach-Object { [string]$_.ActivePath })
            $normalizedPathSetPass = Test-OrdinalSetEqual `
                -Left $normalizedActualPaths -Right $normalizedExpectedPaths
            $scopeSetPass = Test-OrdinalSetEqual -Left $actualScopes -Right $expectedScopes
            $uniqueNormalizedPaths = (
                @($normalizedActualPaths | Where-Object { $null -ne $_ } |
                        Sort-Object -Unique).Count -eq $actualChanges.Count
            )
            $uniqueScopes = @($actualScopes | Sort-Object -Unique).Count -eq $actualChanges.Count
            $mappingPass = $actualChanges.Count -eq 5
            foreach ($expectedMapping in $expectedMappings) {
                $matches = @($actualChanges | Where-Object {
                        (Get-NormalizedWindowsGovernancePath -Path ([string]$_.activePath)) -ceq
                            (Get-NormalizedWindowsGovernancePath -Path $expectedMapping.ActivePath) -and
                        [string]$_.activePath -ceq $expectedMapping.ActivePath -and
                        [string]$_.scope -ceq $expectedMapping.Scope -and
                        [string]$_.beforePayload -ceq ($expectedMapping.PayloadRoot + '/before.txt') -and
                        [string]$_.afterPayload -ceq ($expectedMapping.PayloadRoot + '/after.txt') -and
                        [string]$_.diffPayload -ceq ($expectedMapping.PayloadRoot + '/change.patch')
                    })
                $mappingPass = $mappingPass -and $matches.Count -eq 1
            }
            Add-GovernanceCheck -Id 'EXTERNAL-DELTA-EXACT-COUNT' `
                -Passed ($actualChanges.Count -eq 5) `
                -Message 'External-governance manifest contains exactly five entries.'
            Add-GovernanceCheck -Id 'EXTERNAL-DELTA-EXACT-PATH-SET' `
                -Passed ($exactPathSetPass -and $normalizedPathSetPass -and $uniqueNormalizedPaths) `
                -Message 'External paths are the exact canonical Windows path set with case-insensitive uniqueness.'
            Add-GovernanceCheck -Id 'EXTERNAL-DELTA-EXACT-SCOPE-SET' `
                -Passed ($scopeSetPass -and $uniqueScopes) `
                -Message 'External scopes are the exact unique five-category set.'
            Add-GovernanceCheck -Id 'EXTERNAL-DELTA-PATH-SCOPE-MAPPING' `
                -Passed $mappingPass `
                -Message 'Each canonical external path maps one-to-one to its required scope and payload root.'

            $declaredPayloadPaths = [System.Collections.Generic.List[string]]::new()
            foreach ($change in $actualChanges) {
                Test-RequiredProperties -Value $change -Required @(
                    'activePath',
                    'backupPath',
                    'scope',
                    'beforeSha256',
                    'afterSha256',
                    'beforeSize',
                    'afterSize',
                    'beforePayload',
                    'afterPayload',
                    'diffPayload'
                ) -Prefix 'EXTERNAL-DELTA-CHANGE'

                foreach ($payloadProperty in @('beforePayload', 'afterPayload', 'diffPayload')) {
                    $payloadPath = [string]$change.$payloadProperty
                    [void]$declaredPayloadPaths.Add($payloadPath)
                    Add-GovernanceCheck -Id ('EXTERNAL-DELTA-PATH-' + $payloadProperty + '-' + [Math]::Abs($payloadPath.GetHashCode())) `
                        -Passed (Test-CanonicalRepositoryPath -Path $payloadPath) `
                        -Message 'External delta payload path is a safe canonical relative path.'
                }

                $beforeEntry = @($entries | Where-Object FullName -ceq ([string]$change.beforePayload))
                $afterEntry = @($entries | Where-Object FullName -ceq ([string]$change.afterPayload))
                $diffEntry = @($entries | Where-Object FullName -ceq ([string]$change.diffPayload))
                $entriesPass = (
                    $beforeEntry.Count -eq 1 -and
                    $afterEntry.Count -eq 1 -and
                    $diffEntry.Count -eq 1
                )
                Add-GovernanceCheck -Id ('EXTERNAL-DELTA-ENTRIES-' + [Math]::Abs(([string]$change.activePath).GetHashCode())) `
                    -Passed $entriesPass `
                    -Message 'Every registered external change has before, after, and unified-diff payloads.'
                if (-not $entriesPass) {
                    continue
                }

                $beforeStream = $beforeEntry[0].Open()
                try {
                    $beforeBytes = [byte[]]::new($beforeEntry[0].Length)
                    $beforeRead = 0
                    while ($beforeRead -lt $beforeBytes.Length) {
                        $count = $beforeStream.Read($beforeBytes, $beforeRead, $beforeBytes.Length - $beforeRead)
                        if ($count -eq 0) { break }
                        $beforeRead += $count
                    }
                }
                finally {
                    $beforeStream.Dispose()
                }
                $afterStream = $afterEntry[0].Open()
                try {
                    $afterBytes = [byte[]]::new($afterEntry[0].Length)
                    $afterRead = 0
                    while ($afterRead -lt $afterBytes.Length) {
                        $count = $afterStream.Read($afterBytes, $afterRead, $afterBytes.Length - $afterRead)
                        if ($count -eq 0) { break }
                        $afterRead += $count
                    }
                }
                finally {
                    $afterStream.Dispose()
                }
                $beforeHash = [Convert]::ToHexString(
                    [System.Security.Cryptography.SHA256]::HashData($beforeBytes)
                ).ToLowerInvariant()
                $afterHash = [Convert]::ToHexString(
                    [System.Security.Cryptography.SHA256]::HashData($afterBytes)
                ).ToLowerInvariant()
                $hashAndSizePass = (
                    $beforeHash -ceq [string]$change.beforeSha256 -and
                    $afterHash -ceq [string]$change.afterSha256 -and
                    $beforeBytes.LongLength -eq [long]$change.beforeSize -and
                    $afterBytes.LongLength -eq [long]$change.afterSize
                )
                Add-GovernanceCheck -Id ('EXTERNAL-DELTA-HASH-SIZE-' + [Math]::Abs(([string]$change.activePath).GetHashCode())) `
                    -Passed $hashAndSizePass `
                    -Message 'External before/after payload hashes and sizes match the manifest.'
                $backupPass = (
                    -not [string]::IsNullOrWhiteSpace([string]$change.backupPath) -and
                    $null -ne (
                        Get-NormalizedWindowsGovernancePath -Path ([string]$change.backupPath)
                    ) -and
                    (Get-NormalizedWindowsGovernancePath -Path ([string]$change.backupPath)) -cne
                        (Get-NormalizedWindowsGovernancePath -Path ([string]$change.activePath))
                )
                Add-GovernanceCheck -Id ('EXTERNAL-DELTA-BACKUP-' + [Math]::Abs(([string]$change.activePath).GetHashCode())) `
                    -Passed $backupPass `
                    -Message 'External change declares a distinct absolute canonical Windows backup path.'
            }
            Test-UniqueScalarArray -Value @($declaredPayloadPaths) -Id 'EXTERNAL-DELTA-PAYLOAD-PATHS'
        }
        finally {
            $archive.Dispose()
        }
    }
    finally {
        $stream.Dispose()
    }
}

function Get-FindingDisposition {
    param([AllowNull()][object[]]$Findings)

    return @($Findings | ForEach-Object { [string]$_.disposition })
}

$status = 'FAIL'
$exitCode = 1
$failureMessage = $null
$resolvedRepositoryRoot = $null
$resolvedReportPath = $null
$recordValidated = $false
$derivedTriggers = @()

try {
    $resolvedRepositoryRoot = [System.IO.Path]::GetFullPath($RepositoryRoot)
    if (-not (Test-Path -LiteralPath $resolvedRepositoryRoot -PathType Container)) {
        throw "Repository root does not exist: $resolvedRepositoryRoot"
    }

    if (-not [string]::IsNullOrWhiteSpace($ExpectedPowerShellVersion)) {
        $actualPowerShellVersion = $PSVersionTable.PSVersion.ToString()
        Add-GovernanceCheck -Id 'POWERSHELL-VERSION' `
            -Passed ($actualPowerShellVersion -ceq $ExpectedPowerShellVersion) `
            -Message "PowerShell version is exactly $ExpectedPowerShellVersion (actual: $actualPowerShellVersion)."
    }

    $powerShellPackageValidationRequested = (
        -not [string]::IsNullOrWhiteSpace($PowerShellPackagePath) -or
        -not [string]::IsNullOrWhiteSpace($ExpectedPowerShellPackageSha256)
    )
    if ($powerShellPackageValidationRequested) {
        $powerShellPackageInputsComplete = (
            -not [string]::IsNullOrWhiteSpace($PowerShellPackagePath) -and
            -not [string]::IsNullOrWhiteSpace($ExpectedPowerShellPackageSha256)
        )
        Add-GovernanceCheck -Id 'POWERSHELL-PACKAGE-INPUTS' `
            -Passed $powerShellPackageInputsComplete `
            -Message 'PowerShell package validation requires both the package path and expected SHA-256.'

        if ($powerShellPackageInputsComplete) {
            $powerShellPackageExists = Test-Path -LiteralPath $PowerShellPackagePath -PathType Leaf
            Add-GovernanceCheck -Id 'POWERSHELL-PACKAGE-PATH' `
                -Passed $powerShellPackageExists `
                -Message "PowerShell package exists at the bound path: $PowerShellPackagePath"

            $expectedPowerShellPackageHashValid = (
                $ExpectedPowerShellPackageSha256 -match '^[0-9A-Fa-f]{64}$'
            )
            Add-GovernanceCheck -Id 'POWERSHELL-PACKAGE-EXPECTED-SHA256' `
                -Passed $expectedPowerShellPackageHashValid `
                -Message 'Expected PowerShell package SHA-256 is a full hexadecimal digest.'

            if ($powerShellPackageExists -and $expectedPowerShellPackageHashValid) {
                $actualPowerShellPackageSha256 = (
                    Get-FileHash -LiteralPath $PowerShellPackagePath -Algorithm SHA256
                ).Hash
                Add-GovernanceCheck -Id 'POWERSHELL-PACKAGE-SHA256' `
                    -Passed (
                        $actualPowerShellPackageSha256 -ceq
                        $ExpectedPowerShellPackageSha256.ToUpperInvariant()
                    ) `
                    -Message (
                        'PowerShell package SHA-256 matches the expected digest ' +
                        "(actual: $actualPowerShellPackageSha256)."
                    )
            }
        }
    }

    $governanceRoot = Join-Path $resolvedRepositoryRoot 'Governance'
    $catalogPath = if ([string]::IsNullOrWhiteSpace($CatalogPath)) {
        Join-Path $governanceRoot 'change-trigger-catalog.json'
    }
    elseif ([System.IO.Path]::IsPathRooted($CatalogPath)) {
        [System.IO.Path]::GetFullPath($CatalogPath)
    }
    else {
        [System.IO.Path]::GetFullPath((Join-Path $resolvedRepositoryRoot $CatalogPath))
    }
    $schemaPath = Join-Path $governanceRoot 'assignment-governance-record.schema.json'
    $completionSchemaPath = Join-Path $governanceRoot 'completion-report.schema.json'
    $correctionMatrixSchemaPath = Join-Path $governanceRoot 'finding-correction-matrix.schema.json'
    $regressionMatrixSchemaPath = Join-Path $governanceRoot 'finding-regression-matrix.schema.json'
    $focusedRecordSchemaPath = Join-Path $governanceRoot 'focused-delta-review-record.schema.json'
    $reportContractSchemaPath = Join-Path $governanceRoot 'governance-report-contract.schema.json'
    $handoffContractSchemaPath = Join-Path $governanceRoot 'governance-handoff-contract.schema.json'
    $genericSchemaPaths = @(
        'generic-assignment-record.schema.json',
        'generic-completion-report.schema.json',
        'generic-handoff-contract.schema.json',
        'generic-independent-review-evidence.schema.json',
        'generic-package-inventory.schema.json',
        'generic-pre-review-validation-evidence.schema.json',
        'generic-report-contract.schema.json',
        'generic-scope-inventory.schema.json',
        'generic-validation-summary.schema.json'
    ) | ForEach-Object { Join-Path $governanceRoot $_ }
    $orchestrationSchemaPaths = @(
        'governance-validation-request.schema.json',
        'governance-validation-result.schema.json'
    ) | ForEach-Object { Join-Path $governanceRoot $_ }
    $orchestrationImplementationPaths = @(
        (Join-Path $resolvedRepositoryRoot 'scripts/GovernanceValidationOrchestration.psm1'),
        (Join-Path $resolvedRepositoryRoot 'scripts/Invoke-GovernanceValidation.ps1'),
        (Join-Path $resolvedRepositoryRoot 'scripts/Test-GovernanceValidationOrchestration.ps1')
    )
    $standardPaths = @(
        (Join-Path $governanceRoot 'CHANGE-TRIGGER-REVIEW-AND-BACKLOG-STANDARD.md'),
        (Join-Path $governanceRoot 'FINDING-REMEDIATION-AND-REVIEW-MODE-STANDARD.md'),
        (Join-Path $governanceRoot 'HANDOFF-ARTIFACT-AND-CLASSIC-READINESS-STANDARD.md')
    )

    foreach ($path in @(
            $catalogPath,
            $schemaPath,
            $completionSchemaPath,
            $correctionMatrixSchemaPath,
            $regressionMatrixSchemaPath,
            $focusedRecordSchemaPath,
            $reportContractSchemaPath
        ) + $genericSchemaPaths + $orchestrationSchemaPaths + $orchestrationImplementationPaths + $standardPaths) {
        Add-GovernanceCheck -Id ('SOURCE-' + [System.IO.Path]::GetFileName($path)) -Passed (Test-Path -LiteralPath $path -PathType Leaf) -Message 'Required governance source exists.' -Evidence $path
    }
    if ($script:Errors.Count -gt 0) {
        throw 'One or more required governance sources are missing.'
    }

    $catalog = Read-StrictJson -LiteralPath $catalogPath
    $schema = Read-StrictJson -LiteralPath $schemaPath
    $completionSchema = Read-StrictJson -LiteralPath $completionSchemaPath
    $canonicalModes = Get-CanonicalModeDefinitions
    $canonicalOperatingPolicies = Get-CanonicalOperatingPolicies

    $modeIds = @($catalog.modes | ForEach-Object { [string]$_.id })
    $triggerIds = @($catalog.triggers | ForEach-Object { [string]$_.id })
    $checkpointIds = @($catalog.checkpoints | ForEach-Object { [string]$_ })
    $boundaryIds = @($catalog.decisionBoundaries | ForEach-Object { [string]$_ })
    $resultIds = @($catalog.changeTriggerResults | ForEach-Object { [string]$_ })
    $findingDispositions = @($catalog.findingDispositions | ForEach-Object { [string]$_ })
    $orchestrationProfileIds = @($catalog.orchestrationPolicy.profiles | ForEach-Object { [string]$_.id })

    Test-UniqueScalarArray -Value $modeIds -Id 'CATALOG-MODES'
    Test-UniqueScalarArray -Value $triggerIds -Id 'CATALOG-TRIGGERS'
    Test-UniqueScalarArray -Value $checkpointIds -Id 'CATALOG-CHECKPOINTS'
    Test-UniqueScalarArray -Value $boundaryIds -Id 'CATALOG-BOUNDARIES'
    Test-UniqueScalarArray -Value $resultIds -Id 'CATALOG-RESULTS'
    Test-UniqueScalarArray -Value $findingDispositions -Id 'CATALOG-FINDINGS'
    Test-UniqueScalarArray -Value $orchestrationProfileIds -Id 'CATALOG-ORCHESTRATION-PROFILES'
    Add-GovernanceCheck -Id 'CATALOG-MODE-COUNT' -Passed (@($catalog.modes).Count -eq 4) `
        -Message 'Catalog contains exactly the four canonical modes.'
    $expectedOrchestrationProfiles = @(
        'documentation-registration',
        'governance-schema-change',
        'fixture-harness-change',
        'finding-correction',
        'commit-preparation',
        'focused-revalidation',
        'evidence-only-focused-review',
        'post-merge-closure',
        'full-completion'
    )
    $expectedCheapGateOrder = @(
        'parser-syntax',
        'text-policy',
        'git-diff-check',
        'external-input-binding',
        'toolchain-context-binding',
        'source-worktree-selector-binding'
    )
    $expectedEfficiencyCounters = @(
        'validationExecutionCount',
        'infrastructureOrInvocationFailureCount',
        'fullMatrixRunCount',
        'packageWriteAttemptCount',
        'generatedTaskControllerFileCount',
        'generatedTaskControllerLineCount',
        'readOnlyProbeCount'
    )
    Add-GovernanceCheck -Id 'CATALOG-ORCHESTRATION-PROFILE-COUNT' `
        -Passed (@($catalog.orchestrationPolicy.profiles).Count -eq 9) `
        -Message 'Catalog contains exactly nine permanent orchestration profiles.'
    Add-GovernanceCheck -Id 'CATALOG-ORCHESTRATION-PROFILE-IDS' `
        -Passed (Test-OrdinalSequenceEqual -Left $orchestrationProfileIds -Right $expectedOrchestrationProfiles) `
        -Message 'Orchestration profile IDs and order are canonical.'
    Add-GovernanceCheck -Id 'CATALOG-CHEAP-GATE-ORDER' `
        -Passed (Test-OrdinalSequenceEqual -Left @($catalog.orchestrationPolicy.cheapGateOrder) -Right $expectedCheapGateOrder) `
        -Message 'Cheap gates have the canonical fail-fast order.'
    Add-GovernanceCheck -Id 'CATALOG-EFFICIENCY-COUNTERS' `
        -Passed (Test-OrdinalSequenceEqual -Left @($catalog.orchestrationPolicy.counterFields) -Right $expectedEfficiencyCounters) `
        -Message 'Orchestration policy contains the seven canonical efficiency counters.'
    Add-GovernanceCheck -Id 'CATALOG-DATA-FIRST-TASKS' `
        -Passed (
            [string]$catalog.orchestrationPolicy.taskDefinitionMode -ceq 'DATA_FIRST' -and
            [string]$catalog.orchestrationPolicy.generatedControllerPolicy -ceq 'EXCEPTION_ONLY_AND_TELEMETRY_REQUIRED'
        ) `
        -Message 'Task-specific workflow definitions are data-first and generated controllers are exceptional.'
    Add-GovernanceCheck -Id 'CATALOG-ORCHESTRATION-SOURCE-PATHS' `
        -Passed (
            [string]$catalog.orchestrationPolicy.requestSchema -ceq 'Governance/governance-validation-request.schema.json' -and
            [string]$catalog.orchestrationPolicy.resultSchema -ceq 'Governance/governance-validation-result.schema.json' -and
            [string]$catalog.orchestrationPolicy.runner -ceq 'scripts/Invoke-GovernanceValidation.ps1' -and
            [string]$catalog.orchestrationPolicy.module -ceq 'scripts/GovernanceValidationOrchestration.psm1' -and
            [string]$catalog.orchestrationPolicy.typedResultReader -ceq 'Read-GovernanceTypedResult'
        ) `
        -Message 'Catalog binds the canonical request, result, runner, module, and typed result reader.'
    Add-GovernanceCheck -Id 'CATALOG-DIRECTORY-BEFORE-ZIP' `
        -Passed (Test-OrdinalSequenceEqual -Left @($catalog.orchestrationPolicy.packageSequence) -Right @(
                'FRESH_STAGING',
                'DIRECTORY_VALIDATION',
                'INVENTORY_MANIFEST_PASS',
                'ONE_FINAL_ZIP_WRITE',
                'ZIP_REOPEN_SHA_PATH_INVENTORY_VALIDATION'
            )) `
        -Message 'Directory validation and manifest/inventory PASS precede one final ZIP write.'
    foreach ($profile in @($catalog.orchestrationPolicy.profiles)) {
        Add-GovernanceCheck -Id "CATALOG-ORCHESTRATION-$($profile.id)-STAGES" `
            -Passed (@($profile.requiredSubordinateStages).Count -gt 0) `
            -Message 'Every orchestration profile composes at least one permanent subordinate stage.'
    }
    Add-GovernanceCheck -Id 'CATALOG-FULL-COMPLETION-UNIQUE' `
        -Passed (
            @($catalog.orchestrationPolicy.profiles | Where-Object { [bool]$_.fullMatrix }).Count -eq 1 -and
            [bool](@($catalog.orchestrationPolicy.profiles | Where-Object id -ceq 'full-completion')[0].fullMatrix)
        ) `
        -Message 'Exactly one profile owns the final full matrix.'

    foreach ($modeName in @($canonicalModes.Keys)) {
        $catalogMode = @($catalog.modes | Where-Object id -ceq $modeName)
        Add-GovernanceCheck -Id "MODE-$modeName-UNIQUE" -Passed ($catalogMode.Count -eq 1) `
            -Message 'Canonical mode occurs exactly once.'
        if ($catalogMode.Count -eq 1) {
            foreach ($propertyName in @($canonicalModes[$modeName].Keys)) {
                $expectedValue = $canonicalModes[$modeName][$propertyName]
                $actualValue = $catalogMode[0].$propertyName
                Add-GovernanceCheck -Id "MODE-$modeName-$propertyName" `
                    -Passed ($actualValue -ceq $expectedValue) `
                    -Message 'Catalog mode property matches immutable validator semantics.' `
                    -Evidence "expected=$expectedValue; actual=$actualValue"
            }
        }
    }

    foreach ($policyName in @($canonicalOperatingPolicies.Keys)) {
        $expectedPolicy = $canonicalOperatingPolicies[$policyName]
        $actualPolicy = $catalog.$policyName
        $actualPolicyExists = $null -ne $actualPolicy
        Add-GovernanceCheck -Id "CATALOG-POLICY-$policyName-EXISTS" `
            -Passed $actualPolicyExists `
            -Message 'Catalog contains the canonical operating policy.'
        if (-not $actualPolicyExists) {
            continue
        }

        $expectedPropertyNames = @($expectedPolicy.Keys)
        $actualPropertyNames = @(Get-PropertyNames -Value $actualPolicy)
        Add-GovernanceCheck -Id "CATALOG-POLICY-$policyName-SHAPE" `
            -Passed (Test-OrdinalSetEqual -Left $actualPropertyNames -Right $expectedPropertyNames) `
            -Message 'Operating policy contains exactly the canonical properties.'

        foreach ($propertyName in $expectedPropertyNames) {
            $expectedValue = $expectedPolicy[$propertyName]
            $actualValue = $actualPolicy.$propertyName
            $propertyMatches = if ($expectedValue -is [System.Array]) {
                Test-OrdinalSequenceEqual -Left @($actualValue) -Right @($expectedValue)
            }
            else {
                $actualValue -ceq $expectedValue
            }
            Add-GovernanceCheck -Id "CATALOG-POLICY-$policyName-$propertyName" `
                -Passed $propertyMatches `
                -Message 'Catalog operating-policy value matches immutable validator semantics.' `
                -Evidence "expected=$($expectedValue -join ','); actual=$($actualValue -join ',')"
        }
    }

    foreach ($trigger in @($catalog.triggers)) {
        $triggerName = [string]$trigger.id
        Test-RequiredProperties -Value $trigger -Required @('id', 'domain', 'sources', 'pathPatterns', 'continuousGates') -Prefix "TRIGGER-$triggerName"
        Test-UniqueScalarArray -Value @($trigger.sources) -Id "TRIGGER-$triggerName-SOURCES"
        Test-UniqueScalarArray -Value @($trigger.continuousGates) -Id "TRIGGER-$triggerName-GATES"
        $validSources = @($trigger.sources | Where-Object { $_ -notin @('DIFF', 'EVENT', 'ASSIGNMENT') })
        Add-GovernanceCheck -Id "TRIGGER-$triggerName-SOURCE-VALUES" -Passed ($validSources.Count -eq 0) -Message 'Trigger sources use only supported values.' -Evidence ($validSources -join ', ')
        $diffShapeValid = if ('DIFF' -in @($trigger.sources)) { @($trigger.pathPatterns).Count -gt 0 } else { $true }
        Add-GovernanceCheck -Id "TRIGGER-$triggerName-DIFF-SHAPE" -Passed $diffShapeValid -Message 'Every diff-derived trigger has at least one path pattern.'
        Test-UniqueScalarArray -Value @($trigger.pathPatterns) -Id "TRIGGER-$triggerName-PATTERNS"
        foreach ($pattern in @($trigger.pathPatterns)) {
            Add-GovernanceCheck -Id "TRIGGER-$triggerName-PATTERN-$([Math]::Abs(([string]$pattern).GetHashCode()))" `
                -Passed (Test-CanonicalRepositoryPath -Path ([string]$pattern -replace '\*', 'x')) `
                -Message 'Trigger pattern is repository-relative and canonical.' -Evidence ([string]$pattern)
        }
    }

    $schemaModes = @($schema.properties.executionMode.enum | ForEach-Object { [string]$_ })
    $schemaCheckpoints = @($schema.properties.checkpoint.enum | ForEach-Object { [string]$_ })
    $schemaResults = @($schema.properties.changeTriggerReviewResult.enum | ForEach-Object { [string]$_ })
    $schemaTriggers = @($schema.properties.observedTriggers.items.enum | ForEach-Object { [string]$_ })
    $schemaBoundaries = @($schema.'$defs'.decisionBoundary.properties.type.enum | ForEach-Object { [string]$_ })
    $schemaFindingDispositions = @($schema.'$defs'.finding.properties.disposition.enum | ForEach-Object { [string]$_ })

    Add-GovernanceCheck -Id 'SCHEMA-MODES' -Passed ((($modeIds | Sort-Object) -join ',') -eq (($schemaModes | Sort-Object) -join ',')) -Message 'Schema and catalog modes agree.'
    Add-GovernanceCheck -Id 'SCHEMA-CHECKPOINTS' -Passed ((($checkpointIds | Sort-Object) -join ',') -eq (($schemaCheckpoints | Sort-Object) -join ',')) -Message 'Schema and catalog checkpoints agree.'
    Add-GovernanceCheck -Id 'SCHEMA-RESULTS' -Passed ((($resultIds | Sort-Object) -join ',') -eq (($schemaResults | Sort-Object) -join ',')) -Message 'Schema and catalog trigger results agree.'
    Add-GovernanceCheck -Id 'SCHEMA-TRIGGERS' -Passed ((($triggerIds | Sort-Object) -join ',') -eq (($schemaTriggers | Sort-Object) -join ',')) -Message 'Schema and catalog trigger IDs agree.'
    Add-GovernanceCheck -Id 'SCHEMA-BOUNDARIES' -Passed ((($boundaryIds | Sort-Object) -join ',') -eq (($schemaBoundaries | Sort-Object) -join ',')) -Message 'Schema and catalog decision boundaries agree.'
    Add-GovernanceCheck -Id 'SCHEMA-FINDINGS' -Passed ((($findingDispositions | Sort-Object) -join "`n") -eq (($schemaFindingDispositions | Sort-Object) -join "`n")) -Message 'Schema and catalog finding dispositions agree.'

    $standardsText = ($standardPaths | ForEach-Object { [System.IO.File]::ReadAllText($_, [System.Text.UTF8Encoding]::new($false, $true)) }) -join "`n"
    foreach ($value in @($modeIds + $checkpointIds + $boundaryIds + $resultIds | Sort-Object -Unique)) {
        Add-GovernanceCheck -Id ('STANDARD-{0}' -f [string]$value) -Passed $standardsText.Contains([string]$value, [System.StringComparison]::Ordinal) -Message 'Binding standards reference every catalog control value.' -Evidence $value
    }

    $derivedTriggers = Get-DerivedTriggers -Catalog $catalog -Paths $ChangedPath

    $trackedPathsToValidate = if (@($TrackedPath).Count -gt 0) {
        @($TrackedPath)
    }
    else {
        $gitTracked = @(& git -C $resolvedRepositoryRoot ls-files)
        if ($LASTEXITCODE -ne 0) {
            throw 'Unable to enumerate tracked repository paths.'
        }
        @($gitTracked)
    }
    $trackedPathsToValidate = @(@($trackedPathsToValidate) + @($ChangedPath) | Sort-Object -Unique)
    Test-TrackedPathCoverage -Catalog $catalog -Paths $trackedPathsToValidate

    $backlogPath = Join-Path $resolvedRepositoryRoot 'BACKLOG.md'
    $backlogText = [System.IO.File]::ReadAllText(
        $backlogPath,
        [System.Text.UTF8Encoding]::new($false, $true)
    )
    $backlogTaskMatches = @(
        [regex]::Matches(
            $backlogText,
            '(?m)^\| (BL-(?<number>[0-9]{3})) \| (?:Ready|Planned|Later|Blocked|Done|In Progress) \|'
        )
    )
    $backlogTaskIds = @($backlogTaskMatches | ForEach-Object { $_.Groups[1].Value })
    $backlogTaskNumbers = @($backlogTaskMatches | ForEach-Object { [int]$_.Groups['number'].Value } | Sort-Object)
    $duplicateBacklogIds = @($backlogTaskIds | Group-Object | Where-Object Count -ne 1)
    $maxBacklogId = if ($backlogTaskNumbers.Count -gt 0) { $backlogTaskNumbers[-1] } else { 0 }
    $missingBacklogNumbers = @(1..$maxBacklogId | Where-Object { $_ -notin $backlogTaskNumbers })
    $duplicateBacklogIdText = [string]::Join("`n", [string[]]@($duplicateBacklogIds | ForEach-Object Name))
    $missingBacklogNumberText = [string]::Join("`n", [string[]]$missingBacklogNumbers)
    Add-GovernanceCheck -Id 'BACKLOG-CONTINUITY' `
        -Passed ($maxBacklogId -ge 335 -and $duplicateBacklogIds.Count -eq 0 -and $missingBacklogNumbers.Count -eq 0) `
        -Message 'Backlog IDs are continuous through BL-335 and unique.' `
        -Evidence ('max={0}; duplicates={1}; missing={2}' -f $maxBacklogId, $duplicateBacklogIdText, $missingBacklogNumberText)
    Add-GovernanceCheck -Id 'BACKLOG-QUEUE' `
        -Passed $backlogText.Contains(
            'schedule BL-340 independently in SPR-61 -> final documentation convergence -> Local Work Register dissolution audit -> separately authorized Local Work Register removal',
            [System.StringComparison]::Ordinal
        ) `
        -Message 'Backlog records the exact post-BL-324 queue through separate Local Work Register removal.'

    if (-not [string]::IsNullOrWhiteSpace($ExpectedBaselineCommit)) {
        $baselineBacklog = @(& git -C $resolvedRepositoryRoot show "${ExpectedBaselineCommit}:BACKLOG.md")
        $baselineBacklogExit = $LASTEXITCODE
        foreach ($protectedId in @('BL-340')) {
            $currentLine = @($backlogText -split '\r?\n' | Where-Object { $_ -match "^\| $protectedId \|" })
            $baselineLine = @($baselineBacklog | Where-Object { $_ -match "^\| $protectedId \|" })
            Add-GovernanceCheck -Id "BACKLOG-PROTECTED-$protectedId" `
                -Passed (
                    $baselineBacklogExit -eq 0 -and
                    $currentLine.Count -eq 1 -and
                    $baselineLine.Count -eq 1 -and
                    $currentLine[0] -ceq $baselineLine[0]
                ) `
                -Message "$protectedId canonical acceptance text is unchanged from the trusted baseline."
        }
    }

    if (-not [string]::IsNullOrWhiteSpace($RuntimeCheckpoint)) {
        $repositorySources = @($HostedCISource | Where-Object { $_ -match '^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+@[0-9a-fA-F]{40}$' })
        $runSources = @($HostedCISource | Where-Object { $_ -match '^github-actions-run:[1-9][0-9]*$' })
        Add-GovernanceCheck -Id 'RUNTIME-CHECKPOINT' -Passed ($RuntimeCheckpoint -in $checkpointIds) -Message 'Runtime checkpoint is canonical.' -Evidence $RuntimeCheckpoint
        Add-GovernanceCheck -Id 'RUNTIME-ASSIGNMENT-REQUIRED' `
            -Passed (-not [string]::IsNullOrWhiteSpace($AssignmentRecordPath)) `
            -Message 'A runtime checkpoint requires an assignment record.'
        if ($RuntimeCheckpoint -in @('RELEASE_CANDIDATE', 'STABLE_RELEASE')) {
            Add-GovernanceCheck -Id 'RUNTIME-HOSTED-CI-REPOSITORY' -Passed ($repositorySources.Count -eq 1) -Message 'Hosted CI evidence contains exactly one repository and immutable 40-character commit source.' -Evidence ($repositorySources -join ', ')
            Add-GovernanceCheck -Id 'RUNTIME-HOSTED-CI-RUN' -Passed ($runSources.Count -eq 1) -Message 'Hosted CI evidence contains exactly one GitHub Actions run identity.' -Evidence ($runSources -join ', ')
        }
        Test-UniqueScalarArray -Value @($HostedCISource) -Id 'RUNTIME-HOSTED-CI-SOURCES'
    }

    if (-not [string]::IsNullOrWhiteSpace($AssignmentRecordPath)) {
        $resolvedRecordPath = if ([System.IO.Path]::IsPathRooted($AssignmentRecordPath)) {
            [System.IO.Path]::GetFullPath($AssignmentRecordPath)
        }
        else {
            [System.IO.Path]::GetFullPath((Join-Path $resolvedRepositoryRoot $AssignmentRecordPath))
        }
        Add-GovernanceCheck -Id 'RECORD-EXISTS' -Passed (Test-Path -LiteralPath $resolvedRecordPath -PathType Leaf) -Message 'Assignment governance record exists.' -Evidence $resolvedRecordPath
        if (-not (Test-Path -LiteralPath $resolvedRecordPath -PathType Leaf)) {
            throw 'Assignment governance record is missing.'
        }

        $record = Read-StrictJson -LiteralPath $resolvedRecordPath
        $schemaErrors = @()
        $schemaValid = Test-Json -LiteralPath $resolvedRecordPath -SchemaFile $schemaPath -ErrorVariable schemaErrors -ErrorAction SilentlyContinue
        Add-GovernanceCheck -Id 'RECORD-JSON-SCHEMA' -Passed $schemaValid -Message 'Assignment record passes the canonical JSON Schema including nested types and unknown-property rejection.' -Evidence (@($schemaErrors | ForEach-Object { $_.ToString() }) -join ' | ')

        $requiredTopLevel = @($schema.required | ForEach-Object { [string]$_ })
        Test-RequiredProperties -Value $record -Required $requiredTopLevel -Prefix 'RECORD'

        $recordProperties = @(Get-PropertyNames -Value $record)
        $allowedProperties = @(Get-PropertyNames -Value $schema.properties)
        $unexpectedProperties = @($recordProperties | Where-Object { $_ -notin $allowedProperties })
        Add-GovernanceCheck -Id 'RECORD-ADDITIONAL-PROPERTIES' -Passed ($unexpectedProperties.Count -eq 0) -Message 'Assignment record has no unknown top-level properties.' -Evidence ($unexpectedProperties -join ', ')

        Add-GovernanceCheck -Id 'RECORD-SCHEMA-VERSION' -Passed ([int]$record.schemaVersion -eq 1) -Message 'Assignment record uses schema version 1.'
        $recordedAt = [datetimeoffset]::MinValue
        $recordedAtText = [string]$record.recordedAt
        $recordedAtValid = (
            $recordedAtText -match '^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:Z|[+-]\d{2}:\d{2})$' -and
            [datetimeoffset]::TryParse(
                $recordedAtText,
                [System.Globalization.CultureInfo]::InvariantCulture,
                [System.Globalization.DateTimeStyles]::None,
                [ref]$recordedAt
            )
        )
        Add-GovernanceCheck -Id 'RECORD-TIMESTAMP' -Passed $recordedAtValid -Message 'Assignment record has an ISO-8601 second-precision timestamp.' -Evidence ([string]$record.recordedAt)
        Add-GovernanceCheck -Id 'RECORD-TASK-ID' -Passed ([string]$record.taskId -match '^(?:BL-\d{3})(?:/BL-\d{3})?$') -Message 'Task ID is canonical.'
        Add-GovernanceCheck -Id 'RECORD-REPOSITORY' -Passed ([string]$record.repository -eq 'https://github.com/thomasweidner/flashgate-mcp.git') -Message 'Assignment record names the authoritative repository.'
        Add-GovernanceCheck -Id 'RECORD-BASELINE' -Passed ([string]$record.baselineCommit -match '^[0-9a-f]{40}$') -Message 'Assignment record names an exact lowercase baseline commit.'
        Add-GovernanceCheck -Id 'RECORD-CURRENT-COMMIT' -Passed ([string]$record.currentCommit -match '^[0-9a-f]{40}$') -Message 'Assignment record names an exact lowercase current commit.'
        Add-GovernanceCheck -Id 'RECORD-BRANCH' -Passed (-not [string]::IsNullOrWhiteSpace([string]$record.branch)) -Message 'Assignment record names the working branch.'
        Add-GovernanceCheck -Id 'RECORD-MODE' -Passed ([string]$record.executionMode -in $modeIds) -Message 'Execution mode is cataloged.' -Evidence ([string]$record.executionMode)
        Add-GovernanceCheck -Id 'RECORD-CHECKPOINT' -Passed ([string]$record.checkpoint -in $checkpointIds) -Message 'Checkpoint is cataloged.' -Evidence ([string]$record.checkpoint)
        Add-GovernanceCheck -Id 'RECORD-RESULT' -Passed ([string]$record.changeTriggerReviewResult -in $resultIds) -Message 'Change-trigger result is cataloged.' -Evidence ([string]$record.changeTriggerReviewResult)

        $hasCurrentStateGate = 'currentStateGate' -in $recordProperties
        $currentStateReadinessClaimed = (
            [string]$record.documentationConsistencyResult -in @('PASS', 'PASS_WITH_WARNINGS') -or
            [string]$record.review.focusedValidationResult -ceq 'PASS' -or
            [bool]$record.handoff.required -or
            [bool]$record.handoff.classicReviewReady -or
            [bool]$record.commitPreparation.allFindingsClosed -or
            [bool]$record.commitPreparation.independentDeltaReviewComplete -or
            [bool]$record.commitPreparation.scopeVerified -or
            [bool]$record.commitPreparation.validationPassed -or
            [bool]$record.commitPreparation.commitAuthorized
        )
        $currentReadinessClassPass = (
            -not $currentStateReadinessClaimed -or
            ('recordReadinessClass' -in $recordProperties -and
                [string]$record.recordReadinessClass -ceq 'CURRENT')
        )
        Add-GovernanceCheck -Id 'RECORD-CURRENT-READINESS-CLASS' `
            -Passed $currentReadinessClassPass `
            -Message (
                'Historical schema-version-1 records remain readable, but every current-state, ' +
                'validation, checkpoint, handoff, or commit-readiness claim requires recordReadinessClass=CURRENT.'
            )
        Add-GovernanceCheck -Id 'RECORD-CURRENT-STATE-GATE-PRESENCE' `
            -Passed (-not $currentStateReadinessClaimed -or $hasCurrentStateGate) `
            -Message (
                'Current-state, validation, handoff, or commit-readiness claims require the ' +
                'typed current-state gate; pending workflow-generated schema-version-1 records remain compatible.'
            )
        if ($hasCurrentStateGate) {
            Test-RequiredProperties -Value $record.currentStateGate -Required @(
                'result',
                'repositoryIdentityBound',
                'commitAndBranchBound',
                'completeStatusBound',
                'scopeAndIdsBound',
                'parallelWorktreesBound'
            ) -Prefix 'RECORD-CURRENT-STATE-GATE'
            $currentStateGatePassed = (
                [string]$record.currentStateGate.result -ceq 'PASS' -and
                [bool]$record.currentStateGate.repositoryIdentityBound -and
                [bool]$record.currentStateGate.commitAndBranchBound -and
                [bool]$record.currentStateGate.completeStatusBound -and
                [bool]$record.currentStateGate.scopeAndIdsBound -and
                [bool]$record.currentStateGate.parallelWorktreesBound
            )
            Add-GovernanceCheck -Id 'RECORD-CURRENT-STATE-GATE' `
                -Passed $currentStateGatePassed `
                -Message 'The supplied current-state gate is a complete fail-closed PASS binding.'
        }

        $trustedProvenanceProvided = (
            -not [string]::IsNullOrWhiteSpace($ExpectedRepository) -and
            -not [string]::IsNullOrWhiteSpace($ExpectedBaselineCommit) -and
            -not [string]::IsNullOrWhiteSpace($ExpectedCurrentCommit)
        )
        Add-GovernanceCheck -Id 'RECORD-TRUSTED-PROVENANCE' -Passed $trustedProvenanceProvided `
            -Message 'Record validation receives trusted expected repository, baseline, and current commit.'
        if ($trustedProvenanceProvided) {
            Add-GovernanceCheck -Id 'RECORD-EXPECTED-REPOSITORY' `
                -Passed ([string]$record.repository -ceq $ExpectedRepository) `
                -Message 'Record repository matches the trusted expected repository.'
            Add-GovernanceCheck -Id 'RECORD-EXPECTED-BASELINE' `
                -Passed ([string]$record.baselineCommit -ceq $ExpectedBaselineCommit) `
                -Message 'Record baseline matches the trusted expected baseline.'
            Add-GovernanceCheck -Id 'RECORD-EXPECTED-CURRENT' `
                -Passed ([string]$record.currentCommit -ceq $ExpectedCurrentCommit) `
                -Message 'Record current commit matches the trusted expected commit.'

            $actualHead = [string](& git -C $resolvedRepositoryRoot rev-parse HEAD)
            Add-GovernanceCheck -Id 'RECORD-ACTUAL-HEAD' `
                -Passed ($LASTEXITCODE -eq 0 -and $actualHead -ceq $ExpectedCurrentCommit) `
                -Message 'Trusted current commit matches the actual repository HEAD.' `
                -Evidence "expected=$ExpectedCurrentCommit; actual=$actualHead"
            & git -C $resolvedRepositoryRoot cat-file -e "$ExpectedBaselineCommit`^{commit}" 2>$null
            Add-GovernanceCheck -Id 'RECORD-ACTUAL-BASELINE' `
                -Passed ($LASTEXITCODE -eq 0) `
                -Message 'Trusted baseline resolves to an actual commit in the repository.' `
                -Evidence $ExpectedBaselineCommit
        }
        if (-not [string]::IsNullOrWhiteSpace($RuntimeCheckpoint)) {
            Add-GovernanceCheck -Id 'RECORD-RUNTIME-CHECKPOINT' `
                -Passed ([string]$record.checkpoint -ceq $RuntimeCheckpoint) `
                -Message 'Record checkpoint matches the trusted runtime checkpoint.'
        }

        foreach ($arrayProperty in @(
                'triggeredDomains',
                'observedTriggers',
                'affectedContinuousGates',
                'existingBacklogCoverage',
                'repeatedChecks',
                'checksNotRequired',
                'newBacklogItems',
                'updatedBacklogOrRegisterEntries',
                'deferredTriggerItems'
            )) {
            Test-UniqueScalarArray -Value @($record.$arrayProperty) -Id "RECORD-$arrayProperty"
        }

        $observedTriggers = @($record.observedTriggers | ForEach-Object { [string]$_ })
        $unknownObservedTriggers = @($observedTriggers | Where-Object { $_ -notin $triggerIds })
        Add-GovernanceCheck -Id 'RECORD-OBSERVED-TRIGGERS' -Passed ($unknownObservedTriggers.Count -eq 0) -Message 'All observed triggers are cataloged.' -Evidence ($unknownObservedTriggers -join ', ')
        $missingDerivedTriggers = @($derivedTriggers | Where-Object { $_ -notin $observedTriggers })
        Add-GovernanceCheck -Id 'RECORD-DIFF-TRIGGERS' -Passed ($missingDerivedTriggers.Count -eq 0) -Message 'Record includes every diff-derived material trigger.' -Evidence ($missingDerivedTriggers -join ', ')

        $requiredDomains = @($catalog.triggers | Where-Object { $_.id -in $observedTriggers } | ForEach-Object { [string]$_.domain } | Sort-Object -Unique)
        $missingDomains = @($requiredDomains | Where-Object { $_ -notin @($record.triggeredDomains) })
        Add-GovernanceCheck -Id 'RECORD-DOMAINS' -Passed ($missingDomains.Count -eq 0) -Message 'Triggered domains cover every observed trigger.' -Evidence ($missingDomains -join ', ')

        $requiredGates = @($catalog.triggers | Where-Object { $_.id -in $observedTriggers } | ForEach-Object { @($_.continuousGates) } | Sort-Object -Unique)
        $missingGates = @($requiredGates | Where-Object { $_ -notin @($record.affectedContinuousGates) })
        Add-GovernanceCheck -Id 'RECORD-GATES' -Passed ($missingGates.Count -eq 0) -Message 'Affected continuous gates cover every observed trigger.' -Evidence ($missingGates -join ', ')

        Test-RequiredProperties -Value $record.duplicateSearch -Required @('performed', 'sources', 'result') -Prefix 'RECORD-DUPLICATE'
        Test-UniqueScalarArray -Value @($record.duplicateSearch.sources) -Id 'RECORD-DUPLICATE-SOURCES'
        $duplicateSearchRequired = $observedTriggers.Count -gt 0
        Add-GovernanceCheck -Id 'RECORD-DUPLICATE-PERFORMED' -Passed (-not $duplicateSearchRequired -or [bool]$record.duplicateSearch.performed) -Message 'Material triggers require a duplicate search.'
        Add-GovernanceCheck -Id 'RECORD-DUPLICATE-BACKLOG' -Passed (-not $duplicateSearchRequired -or 'BACKLOG.md' -in @($record.duplicateSearch.sources)) -Message 'Duplicate search includes the canonical backlog.'

        $boundaries = @($record.decisionBoundaries)
        Test-UniqueScalarArray -Value @($boundaries | ForEach-Object { [string]$_.id }) -Id 'RECORD-BOUNDARY-IDS'
        foreach ($boundary in $boundaries) {
            Test-RequiredProperties -Value $boundary -Required @('id', 'type', 'cause', 'evidence', 'blocking', 'owner', 'nextAction') -Prefix 'RECORD-BOUNDARY'
            Add-GovernanceCheck -Id ('RECORD-BOUNDARY-' + [string]$boundary.type) -Passed ([string]$boundary.type -in $boundaryIds) -Message 'Decision boundary is cataloged.'
        }
        $blockingBoundaries = @($boundaries | Where-Object { [bool]$_.blocking })
        Add-GovernanceCheck -Id 'RECORD-BOUNDARY-RESULT' -Passed ($blockingBoundaries.Count -eq 0 -or [string]$record.changeTriggerReviewResult -eq 'BLOCKED_PENDING_DECISION') -Message 'Blocking boundaries force the blocked trigger result.'

        switch ([string]$record.changeTriggerReviewResult) {
            'NO_TRIGGER' {
                Add-GovernanceCheck -Id 'RECORD-RESULT-NO-TRIGGER' `
                    -Passed ($observedTriggers.Count -eq 0 -and @($ChangedPath).Count -eq 0) `
                    -Message 'NO_TRIGGER is valid only without observed triggers and with an empty changed-path set.'
            }
            'EXISTING_BACKLOG_UPDATED' {
                Add-GovernanceCheck -Id 'RECORD-RESULT-UPDATED' -Passed (@($record.updatedBacklogOrRegisterEntries).Count -gt 0) -Message 'An updated-backlog result names the updated entries.'
            }
            'NEW_BACKLOG_REGISTERED' {
                Add-GovernanceCheck -Id 'RECORD-RESULT-NEW' -Passed (@($record.newBacklogItems).Count -gt 0) -Message 'A new-backlog result names registered items.'
            }
            'BLOCKED_PENDING_DECISION' {
                Add-GovernanceCheck -Id 'RECORD-RESULT-BLOCKED' -Passed ($blockingBoundaries.Count -gt 0) -Message 'A blocked result names at least one blocking boundary.'
            }
        }

        Test-RequiredProperties -Value $record.review -Required @(
            'repositoryMutationAllowed',
            'findingFixesPerformed',
            'reviewerIndependencePreserved',
            'originalFindings',
            'discoveredInRunFindings',
            'correctedInRunFindings',
            'deferredFindings',
            'stopConditionsEncountered',
            'selfReviewIterations',
            'permanentRegressionEvidence',
            'focusedValidationResult',
            'independentDeltaReviewRequired'
        ) -Prefix 'RECORD-REVIEW'
        Test-UniqueScalarArray -Value @($record.review.permanentRegressionEvidence) `
            -Id 'RECORD-PERMANENT-REGRESSION-EVIDENCE'

        $allFindingCollections = @(
            @($record.review.originalFindings),
            @($record.review.discoveredInRunFindings),
            @($record.review.correctedInRunFindings),
            @($record.review.deferredFindings)
        )
        $allFindingIds = [System.Collections.Generic.List[string]]::new()
        foreach ($collection in $allFindingCollections) {
            foreach ($finding in $collection) {
                Test-RequiredProperties -Value $finding -Required @(
                    'id',
                    'disposition',
                    'evidence',
                    'cause',
                    'correction',
                    'regressionEvidenceIds',
                    'affectedPaths',
                    'nonFileBoundary',
                    'boundaryId',
                    'boundaryType'
                ) -Prefix 'RECORD-FINDING'
                Add-GovernanceCheck -Id ('RECORD-FINDING-' + [string]$finding.id) -Passed ([string]$finding.disposition -in $findingDispositions) -Message 'Finding disposition is cataloged.' -Evidence ([string]$finding.disposition)
                [void]$allFindingIds.Add([string]$finding.id)
                $findingLocationValid = (
                    @($finding.affectedPaths).Count -gt 0 -or
                    -not [string]::IsNullOrWhiteSpace([string]$finding.nonFileBoundary)
                )
                Add-GovernanceCheck -Id ('RECORD-FINDING-LOCATION-' + [string]$finding.id) `
                    -Passed $findingLocationValid `
                    -Message 'Finding identifies affected paths or an explicit non-file boundary.'
            }
        }

        $mode = [string]$record.executionMode
        $modeDefinition = $canonicalModes[$mode]
        Add-GovernanceCheck -Id 'RECORD-MUTATION-MODE' -Passed ([bool]$record.review.repositoryMutationAllowed -eq [bool]$modeDefinition.repositoryMutationAllowed) -Message 'Repository-mutation declaration matches immutable mode semantics.'
        Add-GovernanceCheck -Id 'RECORD-EXTERNAL-MUTATION-MODE' -Passed ([bool]$record.review.externalMutationAllowed -eq [bool]$modeDefinition.externalMutationAllowed) -Message 'External-mutation declaration matches immutable mode semantics.'
        if ([bool]$modeDefinition.independent) {
            Add-GovernanceCheck -Id 'RECORD-INDEPENDENT-NO-FIX' -Passed (-not [bool]$record.review.findingFixesPerformed) -Message 'Independent review does not fix findings.'
            Add-GovernanceCheck -Id 'RECORD-INDEPENDENT-PRESERVED' -Passed ([bool]$record.review.reviewerIndependencePreserved) -Message 'Independent reviewer boundary is preserved.'
            Add-GovernanceCheck -Id 'RECORD-INDEPENDENT-NO-CORRECTIONS' `
                -Passed (@($record.review.discoveredInRunFindings).Count -eq 0 -and @($record.review.correctedInRunFindings).Count -eq 0) `
                -Message 'Independent modes contain no correction collections.'
        }
        if ($mode -eq 'BUNDLED_CORRECTION') {
            $discovered = @($record.review.discoveredInRunFindings)
            $corrected = @($record.review.correctedInRunFindings)
            Test-UniqueScalarArray -Value @($discovered | ForEach-Object { [string]$_.id }) -Id 'RECORD-SAME-RUN-DISCOVERED-IDS'
            Test-UniqueScalarArray -Value @($corrected | ForEach-Object { [string]$_.id }) -Id 'RECORD-SAME-RUN-CORRECTED-IDS'
            foreach ($finding in $discovered) {
                $partners = @($corrected | Where-Object { [string]$_.id -ceq [string]$finding.id })
                $discoveryEvidencePermanent = @(
                    $finding.regressionEvidenceIds | Where-Object {
                        [string]$_ -notin @($record.review.permanentRegressionEvidence)
                    }
                ).Count -eq 0
                $completeDiscovery = (
                    [string]$finding.disposition -ceq 'DISCOVERED_AND_CORRECTED_IN_RUN' -and
                    -not [string]::IsNullOrWhiteSpace([string]$finding.cause) -and
                    -not [string]::IsNullOrWhiteSpace([string]$finding.correction) -and
                    @($finding.regressionEvidenceIds).Count -gt 0 -and
                    $discoveryEvidencePermanent -and
                    (
                        @($finding.affectedPaths).Count -gt 0 -or
                        -not [string]::IsNullOrWhiteSpace([string]$finding.nonFileBoundary)
                    )
                )
                Add-GovernanceCheck -Id ('RECORD-SAME-RUN-DISCOVERY-' + [string]$finding.id) `
                    -Passed ($completeDiscovery -and $partners.Count -eq 1) `
                    -Message 'Same-run discovery is complete and has exactly one ID-identical correction partner.'
            }
            foreach ($finding in $corrected) {
                $partners = @($discovered | Where-Object { [string]$_.id -ceq [string]$finding.id })
                $correctionEvidencePermanent = @(
                    $finding.regressionEvidenceIds | Where-Object {
                        [string]$_ -notin @($record.review.permanentRegressionEvidence)
                    }
                ).Count -eq 0
                $completeCorrection = (
                    [string]$finding.disposition -ceq 'CORRECTED' -and
                    -not [string]::IsNullOrWhiteSpace([string]$finding.correction) -and
                    @($finding.regressionEvidenceIds).Count -gt 0 -and
                    $correctionEvidencePermanent
                )
                Add-GovernanceCheck -Id ('RECORD-SAME-RUN-CORRECTION-' + [string]$finding.id) `
                    -Passed ($completeCorrection -and $partners.Count -eq 1) `
                    -Message 'Same-run correction is complete and has exactly one ID-identical discovery partner.'
            }
        }
        foreach ($finding in @($record.review.deferredFindings)) {
            $matchingBoundaries = @($boundaries | Where-Object {
                    [string]$_.id -ceq [string]$finding.boundaryId
                })
            $deferredPass = (
                [string]$finding.disposition -ceq 'DEFERRED_WITH_BOUNDARY' -and
                $matchingBoundaries.Count -eq 1 -and
                [bool]$matchingBoundaries[0].blocking -and
                [string]$matchingBoundaries[0].type -ceq [string]$finding.boundaryType -and
                -not [string]::IsNullOrWhiteSpace([string]$matchingBoundaries[0].cause) -and
                -not [string]::IsNullOrWhiteSpace([string]$matchingBoundaries[0].nextAction)
            )
            Add-GovernanceCheck -Id ('RECORD-DEFERRED-BOUNDARY-' + [string]$finding.id) `
                -Passed $deferredPass `
                -Message 'Deferred finding maps to exactly one matching blocking decision boundary.'
        }

        Test-RequiredProperties -Value $record.handoff -Required @(
            'required',
            'package',
            'artifacts',
            'scopeInventoryResult',
            'patchCompletenessResult',
            'manifestResult',
            'missingArtifacts',
            'classicReviewReady',
            'canonicalValidatorPath',
            'canonicalValidatorSha256',
            'canonicalValidatorExitCode',
            'canonicalValidatorResult'
        ) -Prefix 'RECORD-HANDOFF'
        Test-UniqueScalarArray -Value @($record.handoff.artifacts) -Id 'RECORD-HANDOFF-ARTIFACTS'
        Test-UniqueScalarArray -Value @($record.handoff.missingArtifacts) -Id 'RECORD-HANDOFF-MISSING'

        if ([bool]$record.handoff.classicReviewReady) {
            $requiredArtifacts = @(
                'HANDOFF.md',
                'completion-report.json',
                'correction-only.patch',
                'current-delta.patch',
                'finding-correction-matrix.json',
                'finding-regression-matrix.json',
                'focused-delta-review-record.json',
                'report.md',
                'scope-inventory.json',
                'assignment-record.json',
                'external-governance-manifest.json',
                'trusted-expected-hashes.json',
                'validation-summary.json',
                'MANIFEST.sha256'
            )
            $missingNamedArtifacts = @($requiredArtifacts | Where-Object { $_ -notin @($record.handoff.artifacts) })
            $resolvedHandoffPackage = if ([string]::IsNullOrWhiteSpace($HandoffPackagePath)) {
                $null
            }
            else {
                [System.IO.Path]::GetFullPath($HandoffPackagePath)
            }
            $packageExists = (
                $null -ne $resolvedHandoffPackage -and
                (Test-Path -LiteralPath $resolvedHandoffPackage -PathType Leaf)
            )
            $packageOutsideRepository = (
                $packageExists -and
                -not $resolvedHandoffPackage.StartsWith(
                    $resolvedRepositoryRoot.TrimEnd('\', '/') + [System.IO.Path]::DirectorySeparatorChar,
                    [System.StringComparison]::OrdinalIgnoreCase
                )
            )
            $recordPackageMatches = (
                $packageExists -and
                [System.IO.Path]::GetFullPath([string]$record.handoff.package) -ceq $resolvedHandoffPackage
            )
            Add-GovernanceCheck -Id 'RECORD-HANDOFF-PACKAGE-EXISTS' -Passed $packageExists `
                -Message 'Classic-ready handoff package exists.'
            Add-GovernanceCheck -Id 'RECORD-HANDOFF-PACKAGE-OUTSIDE-REPOSITORY' -Passed $packageOutsideRepository `
                -Message 'Classic-ready handoff package is outside the repository.'
            Add-GovernanceCheck -Id 'RECORD-HANDOFF-PACKAGE-MATCH' -Passed $recordPackageMatches `
                -Message 'Record package path matches the trusted handoff package path.'

            $validatorExists = (
                -not [string]::IsNullOrWhiteSpace($CanonicalArtifactValidatorPath) -and
                (Test-Path -LiteralPath $CanonicalArtifactValidatorPath -PathType Leaf)
            )
            $actualValidatorHash = if ($validatorExists) {
                (Get-FileHash -LiteralPath $CanonicalArtifactValidatorPath -Algorithm SHA256).Hash.ToLowerInvariant()
            }
            else {
                ''
            }
            $validatorBindingPass = (
                $validatorExists -and
                [System.IO.Path]::GetFullPath([string]$record.handoff.canonicalValidatorPath) -ceq
                    [System.IO.Path]::GetFullPath($CanonicalArtifactValidatorPath) -and
                [string]$record.handoff.canonicalValidatorSha256 -ceq $actualValidatorHash -and
                (
                    [string]::IsNullOrWhiteSpace($ExpectedCanonicalArtifactValidatorSha256) -or
                    $actualValidatorHash -ceq $ExpectedCanonicalArtifactValidatorSha256.ToLowerInvariant()
                )
            )
            Add-GovernanceCheck -Id 'RECORD-HANDOFF-CANONICAL-VALIDATOR' -Passed $validatorBindingPass `
                -Message 'Handoff binds the existing canonical external artifact validator and exact hash.' `
                -Evidence $actualValidatorHash

            $artifactValidatorExitCode = 2
            $artifactValidatorStatus = 'ERROR'
            $artifactValidatorOutput = @()
            if ($packageExists -and $validatorBindingPass) {
                $pwshPath = (Get-Command pwsh -ErrorAction Stop).Source
                $validatorLiteral = $CanonicalArtifactValidatorPath.Replace("'", "''")
                $packageLiteral = $resolvedHandoffPackage.Replace("'", "''")
                $artifactValidatorCommand = (
                    "& '$validatorLiteral' -ArtifactPath '$packageLiteral' " +
                    "-ReadinessRequirement RequireTrue | ConvertTo-Json -Depth 10 -Compress"
                )
                $artifactValidatorOutput = @(
                    & $pwshPath -NoLogo -NoProfile -Command $artifactValidatorCommand
                )
                $artifactValidatorExitCode = $LASTEXITCODE
                $artifactValidatorStatus = if ($artifactValidatorExitCode -eq 0) { 'PASS' } elseif ($artifactValidatorExitCode -eq 1) { 'FAIL' } else { 'ERROR' }
            }
            $artifactValidatorPass = (
                $artifactValidatorExitCode -eq 0 -and
                $artifactValidatorStatus -ceq 'PASS' -and
                [int]$record.handoff.canonicalValidatorExitCode -eq 0 -and
                [string]$record.handoff.canonicalValidatorResult -ceq 'PASS'
            )
            $artifactValidatorEvidence = if ($artifactValidatorOutput) {
                @($artifactValidatorOutput | ForEach-Object {
                        if ($_ -is [string]) { $_ } else { $_ | ConvertTo-Json -Depth 8 -Compress }
                    }) -join ' | '
            }
            else {
                ''
            }
            Add-GovernanceCheck -Id 'RECORD-HANDOFF-ARTIFACT-VALIDATOR-RESULT' -Passed $artifactValidatorPass `
                -Message 'Authoritative external artifact validator passed the actual package.' `
                -Evidence "exit=$artifactValidatorExitCode; status=$artifactValidatorStatus; output=$artifactValidatorEvidence"

            $packageEntries = @()
            $packageEntryBytes = @{}
            if ($packageExists) {
                Add-Type -AssemblyName System.IO.Compression
                $packageStream = [System.IO.File]::OpenRead($resolvedHandoffPackage)
                try {
                    $packageArchive = [System.IO.Compression.ZipArchive]::new(
                        $packageStream,
                        [System.IO.Compression.ZipArchiveMode]::Read,
                        $false
                    )
                    try {
                        $packageEntries = @($packageArchive.Entries | ForEach-Object { $_.FullName })
                        foreach ($packageEntry in @($packageArchive.Entries | Where-Object {
                                    $_.FullName -like 'external/*/after.txt' -or
                                    $_.FullName -in @(
                                        'HANDOFF.md',
                                        'assignment-record.json',
                                        'completion-report.json',
                                        'scope-inventory.json',
                                        'correction-only.patch',
                                        'current-delta.patch',
                                        'external-governance-manifest.json',
                                        'finding-correction-matrix.json',
                                        'finding-regression-matrix.json',
                                        'focused-delta-review-record.json',
                                        'readiness-evidence.json',
                                        'report.md',
                                        'MANIFEST.sha256',
                                        'trusted-expected-hashes.json',
                                        'validation-summary.json'
                                    )
                                })) {
                            $entryStream = $packageEntry.Open()
                            try {
                                $entryMemory = [System.IO.MemoryStream]::new()
                                try {
                                    $entryStream.CopyTo($entryMemory)
                                    $packageEntryBytes[$packageEntry.FullName] = $entryMemory.ToArray()
                                }
                                finally {
                                    $entryMemory.Dispose()
                                }
                            }
                            finally {
                                $entryStream.Dispose()
                            }
                        }
                    }
                    finally {
                        $packageArchive.Dispose()
                    }
                }
                finally {
                    $packageStream.Dispose()
                }
            }
            $missingActualArtifacts = @($record.handoff.artifacts | Where-Object { $_ -notin $packageEntries })
            Add-GovernanceCheck -Id 'RECORD-HANDOFF-ACTUAL-ARTIFACTS' `
                -Passed ($missingActualArtifacts.Count -eq 0) `
                -Message 'Every declared handoff artifact exists in the actual ZIP.' `
                -Evidence ($missingActualArtifacts -join ', ')
            if ($packageExists) {
                Test-ExternalDeltaPayload -PackagePath $resolvedHandoffPackage
            }
            $packageContractEntriesPresent = @(
                @(
                    'assignment-record.json',
                    'HANDOFF.md',
                    'completion-report.json',
                    'scope-inventory.json',
                    'correction-only.patch',
                    'current-delta.patch',
                    'external-governance-manifest.json',
                    'finding-correction-matrix.json',
                    'finding-regression-matrix.json',
                    'focused-delta-review-record.json',
                    'readiness-evidence.json',
                    'report.md',
                    'MANIFEST.sha256',
                    'trusted-expected-hashes.json',
                    'validation-summary.json'
                ) | Where-Object { -not $packageEntryBytes.ContainsKey($_) }
            )
            Add-GovernanceCheck -Id 'RECORD-HANDOFF-CONTRACT-ENTRIES' `
                -Passed ($packageContractEntriesPresent.Count -eq 0) `
                -Message 'Package contains every embedded machine-readable governance contract.' `
                -Evidence ($packageContractEntriesPresent -join ', ')

            $embeddedContractPass = $false
            if ($packageContractEntriesPresent.Count -eq 0) {
                $strictUtf8 = [System.Text.UTF8Encoding]::new($false, $true)
                $embeddedAssignmentBytes = [byte[]]$packageEntryBytes['assignment-record.json']
                $embeddedHandoffBytes = [byte[]]$packageEntryBytes['HANDOFF.md']
                $embeddedCompletionBytes = [byte[]]$packageEntryBytes['completion-report.json']
                $embeddedScopeBytes = [byte[]]$packageEntryBytes['scope-inventory.json']
                $embeddedPatchBytes = [byte[]]$packageEntryBytes['correction-only.patch']
                $embeddedCurrentDeltaBytes = [byte[]]$packageEntryBytes['current-delta.patch']
                $embeddedExternalBytes = [byte[]]$packageEntryBytes['external-governance-manifest.json']
                $embeddedCorrectionMatrixBytes = [byte[]]$packageEntryBytes['finding-correction-matrix.json']
                $embeddedRegressionMatrixBytes = [byte[]]$packageEntryBytes['finding-regression-matrix.json']
                $embeddedFocusedRecordBytes = [byte[]]$packageEntryBytes['focused-delta-review-record.json']
                $embeddedReadinessBytes = [byte[]]$packageEntryBytes['readiness-evidence.json']
                $embeddedReportBytes = [byte[]]$packageEntryBytes['report.md']
                $embeddedManifestBytes = [byte[]]$packageEntryBytes['MANIFEST.sha256']
                $embeddedTrustedHashesBytes = [byte[]]$packageEntryBytes['trusted-expected-hashes.json']
                $embeddedValidationBytes = [byte[]]$packageEntryBytes['validation-summary.json']
                $embeddedAssignmentHash = Get-LowerSha256 -Bytes $embeddedAssignmentBytes
                $trustedAssignmentHash = (
                    Get-FileHash -LiteralPath $resolvedRecordPath -Algorithm SHA256
                ).Hash.ToLowerInvariant()
                $assignmentBindingPass = $embeddedAssignmentHash -ceq $trustedAssignmentHash
                Add-GovernanceCheck -Id 'RECORD-HANDOFF-ASSIGNMENT-BINDING' `
                    -Passed $assignmentBindingPass `
                    -Message 'Embedded assignment record is byte-identical to the trusted validated record.'

                $embeddedCompletionText = $strictUtf8.GetString($embeddedCompletionBytes)
                $embeddedHandoffText = $strictUtf8.GetString($embeddedHandoffBytes)
                $embeddedCorrectionMatrixText = $strictUtf8.GetString($embeddedCorrectionMatrixBytes)
                $embeddedRegressionMatrixText = $strictUtf8.GetString($embeddedRegressionMatrixBytes)
                $embeddedFocusedRecordText = $strictUtf8.GetString($embeddedFocusedRecordBytes)
                $embeddedReadinessText = $strictUtf8.GetString($embeddedReadinessBytes)
                $embeddedReportText = $strictUtf8.GetString($embeddedReportBytes)
                $embeddedValidationText = $strictUtf8.GetString($embeddedValidationBytes)
                $contractSchemas = @(
                    [pscustomobject]@{
                        Id = 'COMPLETION'
                        Text = $embeddedCompletionText
                        Schema = $completionSchemaPath
                    },
                    [pscustomobject]@{
                        Id = 'CORRECTION-MATRIX'
                        Text = $embeddedCorrectionMatrixText
                        Schema = $correctionMatrixSchemaPath
                    },
                    [pscustomobject]@{
                        Id = 'REGRESSION-MATRIX'
                        Text = $embeddedRegressionMatrixText
                        Schema = $regressionMatrixSchemaPath
                    },
                    [pscustomobject]@{
                        Id = 'FOCUSED-RECORD'
                        Text = $embeddedFocusedRecordText
                        Schema = $focusedRecordSchemaPath
                    }
                )
                $embeddedSchemaPass = $true
                foreach ($contractSchema in $contractSchemas) {
                    $contractErrors = @()
                    $contractPass = $contractSchema.Text |
                        Test-Json -SchemaFile $contractSchema.Schema `
                            -ErrorVariable contractErrors -ErrorAction SilentlyContinue
                    Add-GovernanceCheck -Id ('RECORD-HANDOFF-' + $contractSchema.Id + '-SCHEMA') `
                        -Passed $contractPass `
                        -Message 'Embedded machine-readable contract passes its strict schema.' `
                        -Evidence (@($contractErrors | ForEach-Object ToString) -join ' | ')
                    $embeddedSchemaPass = $embeddedSchemaPass -and $contractPass
                }
                $embeddedCompletion = $embeddedCompletionText |
                    ConvertFrom-Json -Depth 100 -DateKind String
                $embeddedScopeText = $strictUtf8.GetString($embeddedScopeBytes)
                $embeddedScope = $embeddedScopeText |
                    ConvertFrom-Json -Depth 100 -DateKind String
                $embeddedExternal = $strictUtf8.GetString($embeddedExternalBytes) |
                    ConvertFrom-Json -Depth 100 -DateKind String
                $embeddedCorrectionMatrix = $embeddedCorrectionMatrixText |
                    ConvertFrom-Json -Depth 100 -DateKind String
                $embeddedRegressionMatrix = $embeddedRegressionMatrixText |
                    ConvertFrom-Json -Depth 100 -DateKind String
                $embeddedFocusedRecord = $embeddedFocusedRecordText |
                    ConvertFrom-Json -Depth 100 -DateKind String
                $embeddedTrustedHashesText = $strictUtf8.GetString($embeddedTrustedHashesBytes)
                $embeddedTrustedHashes = $embeddedTrustedHashesText |
                    ConvertFrom-Json -Depth 100 -DateKind String

                $reportContractMatches = @(
                    [regex]::Matches(
                        $embeddedReportText,
                        '(?ms)^BEGIN_GOVERNANCE_REPORT_CONTRACT\r?\n(?<json>\{.*?\})\r?\nEND_GOVERNANCE_REPORT_CONTRACT$'
                    )
                )
                $reportContractUnique = $reportContractMatches.Count -eq 1
                Add-GovernanceCheck -Id 'RECORD-HANDOFF-REPORT-CONTRACT-UNIQUE' `
                    -Passed $reportContractUnique `
                    -Message 'Narrative report contains exactly one bounded governance contract.'
                $reportContractSchemaPass = $false
                $reportContract = $null
                if ($reportContractUnique) {
                    $reportContractText = $reportContractMatches[0].Groups['json'].Value
                    $reportContractErrors = @()
                    $reportContractSchemaPass = $reportContractText |
                        Test-Json -SchemaFile $reportContractSchemaPath `
                            -ErrorVariable reportContractErrors -ErrorAction SilentlyContinue
                    Add-GovernanceCheck -Id 'RECORD-HANDOFF-REPORT-CONTRACT-SCHEMA' `
                        -Passed $reportContractSchemaPass `
                        -Message 'Narrative report contract passes its strict schema.' `
                        -Evidence (@($reportContractErrors | ForEach-Object ToString) -join ' | ')
                    if ($reportContractSchemaPass) {
                        $reportContract = $reportContractText |
                            ConvertFrom-Json -Depth 100 -DateKind String
                    }
                }

                $expectedTargetFindings = @(
                    'BL333-BL334-REV-013',
                    'BL333-BL334-REV-015'
                )
                $expectedClosedFindings = @(
                    'BL333-BL334-REV-007',
                    'BL333-BL334-REV-008',
                    'BL333-BL334-REV-010'
                )
                $expectedHandoffStatus =
                    'FOURTH_BUNDLED_CORRECTION_COMPLETE_AWAITING_FOCUSED_DELTA_REVIEW'
                $expectedNextAction =
                    'Perform only the focused independent delta review of BL333-BL334-REV-013 and BL333-BL334-REV-015 with the new verified review package.'
                $handoffDocumentResult = Test-HandoffDocumentContracts `
                    -Text $embeddedHandoffText `
                    -ContractSchemaPath $handoffContractSchemaPath
                $handoffContract = $handoffDocumentResult.Contract
                $handoffContractSchemaPass =
                    [bool]$handoffDocumentResult.ContractSchemaPass
                $handoffMarkerCountGatePassed =
                    [bool]$handoffDocumentResult.MarkerCountGatePassed
                $visibleHandoffKeyGatePassed =
                    [bool]$handoffDocumentResult.VisibleKeyGatePassed
                $handoffVisibleParityPass =
                    [bool]$handoffDocumentResult.VisibleParityGatePassed
                $reservedControlLineGatePassed =
                    [bool]$handoffDocumentResult.ReservedControlLineGatePassed
                $readinessEvidence = $embeddedReadinessText |
                    ConvertFrom-Json -Depth 100 -DateKind String
                $validationSummary = $embeddedValidationText |
                    ConvertFrom-Json -Depth 100 -DateKind String

                $manifestText = $strictUtf8.GetString($embeddedManifestBytes)
                $manifestRecords = @(
                    $manifestText -split '\r?\n' |
                        Where-Object { -not [string]::IsNullOrWhiteSpace($_) } |
                        ForEach-Object {
                            if ($_ -notmatch '^(?<sha>[0-9a-f]{64})  (?<size>[0-9]+)  (?<path>.+)$') {
                                throw 'Package manifest contains an invalid record.'
                            }
                            [pscustomobject]@{
                                Sha256 = $Matches['sha']
                                Size = [long]$Matches['size']
                                Path = $Matches['path']
                            }
                        }
                )
                $manifestCorrection = @(
                    $manifestRecords | Where-Object Path -ceq 'correction-only.patch'
                )
                $manifestCurrentDelta = @(
                    $manifestRecords | Where-Object Path -ceq 'current-delta.patch'
                )

                $actualCorrectionPatchHash = Get-LowerSha256 -Bytes $embeddedPatchBytes
                $actualCurrentDeltaHash = Get-LowerSha256 -Bytes $embeddedCurrentDeltaBytes
                $trustedHashProperties = @(
                    $embeddedTrustedHashes.PSObject.Properties.Name
                )
                $trustedHashShapePass = Test-OrdinalSetEqual -Left $trustedHashProperties -Right @(
                    'schemaVersion',
                    'previousReviewPackage',
                    'previousReviewSha256',
                    'correctionPatchArtifact',
                    'correctionPatchSha256',
                    'currentDeltaArtifact',
                    'currentDeltaSha256'
                )
                $patchHashBindingPass = (
                    $manifestCorrection.Count -eq 1 -and
                    $manifestCorrection[0].Sha256 -ceq $actualCorrectionPatchHash -and
                    $manifestCorrection[0].Size -eq $embeddedPatchBytes.LongLength -and
                    -not [string]::IsNullOrWhiteSpace($ExpectedCorrectionPatchSha256) -and
                    $actualCorrectionPatchHash -ceq $ExpectedCorrectionPatchSha256.ToLowerInvariant() -and
                    [string]$record.focusedDelta.correctionPatchArtifact -ceq 'correction-only.patch' -and
                    [string]$record.focusedDelta.correctionPatchSha256 -ceq $actualCorrectionPatchHash -and
                    [string]$embeddedFocusedRecord.correctionPatchArtifact -ceq 'correction-only.patch' -and
                    [string]$embeddedFocusedRecord.correctionPatchSha256 -ceq $actualCorrectionPatchHash -and
                    [string]$embeddedCompletion.correctionPatchArtifact -ceq 'correction-only.patch' -and
                    [string]$embeddedCompletion.correctionPatchSha256 -ceq $actualCorrectionPatchHash -and
                    $trustedHashShapePass -and
                    [int]$embeddedTrustedHashes.schemaVersion -eq 1 -and
                    [string]$embeddedTrustedHashes.correctionPatchArtifact -ceq 'correction-only.patch' -and
                    [string]$embeddedTrustedHashes.correctionPatchSha256 -ceq $actualCorrectionPatchHash
                )
                Add-GovernanceCheck -Id 'RECORD-HANDOFF-CORRECTION-PATCH-BYTE-HASH' `
                    -Passed $patchHashBindingPass `
                    -Message 'Actual correction-patch bytes bind manifest, trusted expected hash, assignment, focused record, completion report, and trusted-hash record.' `
                    -Evidence $actualCorrectionPatchHash

                $currentDeltaHashBindingPass = (
                    $manifestCurrentDelta.Count -eq 1 -and
                    $manifestCurrentDelta[0].Sha256 -ceq $actualCurrentDeltaHash -and
                    $manifestCurrentDelta[0].Size -eq $embeddedCurrentDeltaBytes.LongLength -and
                    -not [string]::IsNullOrWhiteSpace($ExpectedCurrentDeltaSha256) -and
                    $actualCurrentDeltaHash -ceq $ExpectedCurrentDeltaSha256.ToLowerInvariant() -and
                    [string]$record.focusedDelta.currentDeltaArtifact -ceq 'current-delta.patch' -and
                    [string]$record.focusedDelta.currentDeltaSha256 -ceq $actualCurrentDeltaHash -and
                    [string]$embeddedFocusedRecord.currentDeltaArtifact -ceq 'current-delta.patch' -and
                    [string]$embeddedFocusedRecord.currentDeltaSha256 -ceq $actualCurrentDeltaHash -and
                    [string]$embeddedCompletion.currentDeltaArtifact -ceq 'current-delta.patch' -and
                    [string]$embeddedCompletion.currentDeltaSha256 -ceq $actualCurrentDeltaHash -and
                    $trustedHashShapePass -and
                    [string]$embeddedTrustedHashes.currentDeltaArtifact -ceq 'current-delta.patch' -and
                    [string]$embeddedTrustedHashes.currentDeltaSha256 -ceq $actualCurrentDeltaHash
                )
                Add-GovernanceCheck -Id 'RECORD-HANDOFF-CURRENT-DELTA-BYTE-HASH' `
                    -Passed $currentDeltaHashBindingPass `
                    -Message 'Actual current-delta bytes bind manifest, trusted expected hash, assignment, focused record, completion report, and trusted-hash record.' `
                    -Evidence $actualCurrentDeltaHash

                $previousReviewBindingPass = (
                    -not [string]::IsNullOrWhiteSpace([string]$record.focusedDelta.previousReviewPackage) -and
                    (Test-Path -LiteralPath ([string]$record.focusedDelta.previousReviewPackage) -PathType Leaf) -and
                    [string]$record.focusedDelta.previousReviewPackage -ceq
                        [string]$embeddedFocusedRecord.previousReviewPackage -and
                    [string]$record.focusedDelta.previousReviewPackage -ceq
                        [string]$embeddedTrustedHashes.previousReviewPackage -and
                    [string]$record.focusedDelta.priorReviewBaselineSha256 -ceq
                        [string]$embeddedFocusedRecord.previousReviewSha256 -and
                    [string]$record.focusedDelta.priorReviewBaselineSha256 -ceq
                        [string]$embeddedTrustedHashes.previousReviewSha256 -and
                    (
                        Get-FileHash -LiteralPath ([string]$record.focusedDelta.previousReviewPackage) `
                            -Algorithm SHA256
                    ).Hash.ToLowerInvariant() -ceq [string]$record.focusedDelta.priorReviewBaselineSha256
                )
                Add-GovernanceCheck -Id 'RECORD-HANDOFF-PREVIOUS-REVIEW-BINDING' `
                    -Passed $previousReviewBindingPass `
                    -Message 'Focused evidence binds the actual previous review package and SHA-256.'

                $embeddedScopePaths = @(
                    $embeddedScope.entries | ForEach-Object { [string]$_.path }
                )
                $embeddedReferencePaths = @(
                    $embeddedScope.referenceOnlyPaths | ForEach-Object { [string]$_ }
                )
                $trustedChangedPaths = @($ChangedPath | Sort-Object -Unique)
                $reportedRepositoryPaths = @(
                    $embeddedCompletion.repositoryArtifacts | ForEach-Object { [string]$_ }
                )
                $reportedExternalPaths = @(
                    $embeddedCompletion.externalGovernanceChanges | ForEach-Object { [string]$_ }
                )
                $manifestExternalPaths = @(
                    $embeddedExternal.changes | ForEach-Object { [string]$_.activePath }
                )
                $matrixAffectedPaths = @(
                    $embeddedCorrectionMatrix.findings |
                        ForEach-Object { @($_.affectedPaths) } |
                        Sort-Object -Unique
                )
                $patchPaths = Get-PatchPaths -PatchText (
                    $strictUtf8.GetString($embeddedPatchBytes)
                )
                $currentDeltaPaths = Get-PatchPaths -PatchText (
                    $strictUtf8.GetString($embeddedCurrentDeltaBytes)
                )
                $expectedCurrentDeltaPaths = @(
                    @($trustedChangedPaths) + @($embeddedReferencePaths) |
                        Sort-Object -Unique
                )
                $reportRepositoryPaths = if ($null -ne $reportContract) {
                    @($reportContract.repositoryPaths)
                }
                else {
                    @()
                }
                $reportReferencePaths = if ($null -ne $reportContract) {
                    @($reportContract.referenceOnlyPaths)
                }
                else {
                    @()
                }
                $repositoryPathParityPass = (
                    (Test-OrdinalSetEqual $embeddedScopePaths $trustedChangedPaths) -and
                    (Test-OrdinalSetEqual $reportedRepositoryPaths $trustedChangedPaths) -and
                    (Test-OrdinalSetEqual $record.focusedDelta.correctionOnlyPaths $trustedChangedPaths) -and
                    (Test-OrdinalSetEqual $record.focusedDelta.allowedDeltaPaths $trustedChangedPaths) -and
                    (Test-OrdinalSetEqual $embeddedFocusedRecord.correctionOnlyPaths $trustedChangedPaths) -and
                    (Test-OrdinalSetEqual $embeddedFocusedRecord.allowedDeltaPaths $trustedChangedPaths) -and
                    (Test-OrdinalSetEqual $embeddedCorrectionMatrix.repositoryCorrectionPaths $trustedChangedPaths) -and
                    (Test-OrdinalSetEqual $matrixAffectedPaths $trustedChangedPaths) -and
                    (Test-OrdinalSetEqual $reportRepositoryPaths $trustedChangedPaths) -and
                    (Test-OrdinalSetEqual $patchPaths $trustedChangedPaths) -and
                    (Test-OrdinalSetEqual $currentDeltaPaths $expectedCurrentDeltaPaths) -and
                    (Test-OrdinalSetEqual $record.focusedDelta.referenceOnlyPaths $embeddedReferencePaths) -and
                    (Test-OrdinalSetEqual $embeddedFocusedRecord.referenceOnlyPaths $embeddedReferencePaths) -and
                    (Test-OrdinalSetEqual $reportReferencePaths $embeddedReferencePaths) -and
                    @($trustedChangedPaths | Where-Object {
                            -not (Test-CanonicalRepositoryPath -Path $_)
                        }).Count -eq 0
                )
                Add-GovernanceCheck -Id 'RECORD-HANDOFF-REPOSITORY-PATH-PARITY' `
                    -Passed $repositoryPathParityPass `
                    -Message 'Patch, current delta, scope, assignment, focused record, completion report, correction matrix, and report contract have exact repository/reference path parity.'

                $reportDirectInterfacePaths = if ($null -ne $reportContract) {
                    @($reportContract.directInterfacePaths)
                }
                else {
                    @()
                }
                $directInterfaceParityPass = (
                    (Test-OrdinalSetEqual $record.focusedDelta.directInterfacePaths $embeddedFocusedRecord.directInterfacePaths) -and
                    (Test-OrdinalSetEqual $record.focusedDelta.directInterfacePaths $reportDirectInterfacePaths) -and
                    @($record.focusedDelta.directInterfacePaths).Count -gt 0 -and
                    @($record.focusedDelta.directInterfacePaths | Where-Object {
                            $_ -notin @($trustedChangedPaths)
                        }).Count -eq 0
                )
                Add-GovernanceCheck -Id 'RECORD-HANDOFF-DIRECT-INTERFACE-PARITY' `
                    -Passed $directInterfaceParityPass `
                    -Message 'Assignment, focused record, and report contract declare the same non-empty direct-interface subset.'

                $reportExternalPaths = if ($null -ne $reportContract) {
                    @($reportContract.externalPaths)
                }
                else {
                    @()
                }
                $expectedExternalMappings = @(Get-ExpectedExternalGovernanceMappings)
                $manifestExternalScopes = @(
                    $embeddedExternal.changes | ForEach-Object { [string]$_.scope }
                )
                $expectedExternalPaths = @(
                    $expectedExternalMappings | ForEach-Object { [string]$_.ActivePath }
                )
                $expectedExternalScopes = @(
                    $expectedExternalMappings | ForEach-Object { [string]$_.Scope }
                )
                $externalMappingParityPass = @($embeddedExternal.changes).Count -eq 5
                foreach ($expectedExternalMapping in $expectedExternalMappings) {
                    $externalMappingParityPass = $externalMappingParityPass -and @(
                        $embeddedExternal.changes | Where-Object {
                            [string]$_.activePath -ceq $expectedExternalMapping.ActivePath -and
                            [string]$_.scope -ceq $expectedExternalMapping.Scope
                        }
                    ).Count -eq 1
                }
                $externalPathParityPass = (
                    (Test-OrdinalSetEqual $reportedExternalPaths $manifestExternalPaths) -and
                    (Test-OrdinalSetEqual $reportExternalPaths $manifestExternalPaths) -and
                    (Test-OrdinalSetEqual $manifestExternalPaths $expectedExternalPaths) -and
                    (Test-OrdinalSetEqual $manifestExternalScopes $expectedExternalScopes) -and
                    $externalMappingParityPass
                )
                Add-GovernanceCheck -Id 'RECORD-HANDOFF-EXTERNAL-PATH-PARITY' `
                    -Passed $externalPathParityPass `
                    -Message 'External manifest, deltas, completion, and report have exact path, category, and one-to-one mapping parity.'

                $embeddedScopeHash = Get-LowerSha256 -Bytes $embeddedScopeBytes
                $embeddedReportBindingPass = (
                    [string]$embeddedCompletion.assignmentRecordSha256 -ceq $embeddedAssignmentHash -and
                    [string]$embeddedCompletion.scopeInventorySha256 -ceq $embeddedScopeHash
                )
                Add-GovernanceCheck -Id 'RECORD-HANDOFF-REPORT-HASH-BINDING' `
                    -Passed $embeddedReportBindingPass `
                    -Message 'Embedded completion report binds exact assignment and scope hashes.'

                $assignmentFindings = @($record.review.originalFindings)
                $completionFindings = @($embeddedCompletion.findingStatus)
                $correctionFindings = @($embeddedCorrectionMatrix.findings)
                $regressionFindings = @($embeddedRegressionMatrix.findings)
                $focusedFindingIds = @($embeddedFocusedRecord.reviewedFindingIds)
                $reportFindingIds = if ($null -ne $reportContract) {
                    @($reportContract.findingIds)
                }
                else {
                    @()
                }
                $findingIdParityPass = (
                    (Test-OrdinalSetEqual $assignmentFindings.id $expectedTargetFindings) -and
                    (Test-OrdinalSetEqual $completionFindings.id $expectedTargetFindings) -and
                    (Test-OrdinalSetEqual $correctionFindings.id $expectedTargetFindings) -and
                    (Test-OrdinalSetEqual $regressionFindings.id $expectedTargetFindings) -and
                    (Test-OrdinalSetEqual $focusedFindingIds $expectedTargetFindings) -and
                    (Test-OrdinalSetEqual $reportFindingIds $expectedTargetFindings) -and
                    [int]$embeddedCorrectionMatrix.correctedFindingCount -eq 2 -and
                    @($assignmentFindings.id | Sort-Object -Unique).Count -eq
                        $assignmentFindings.Count -and
                    @($completionFindings.id | Sort-Object -Unique).Count -eq
                        $completionFindings.Count -and
                    @($correctionFindings.id | Sort-Object -Unique).Count -eq
                        $correctionFindings.Count -and
                    @($regressionFindings.id | Sort-Object -Unique).Count -eq
                        $regressionFindings.Count
                )
                Add-GovernanceCheck -Id 'RECORD-HANDOFF-FINDING-ID-PARITY' `
                    -Passed $findingIdParityPass `
                    -Message 'All current correction/review contracts contain exactly REV-013 and REV-015.'

                $allRegressionTests = @(
                    $regressionFindings |
                        ForEach-Object { @($_.regressionTests) }
                )
                $allRegressionTestIds = @(
                    $allRegressionTests | ForEach-Object { [string]$_.id }
                )
                $productiveValidatorRelativePath = 'scripts/Test-GovernanceConsistency.ps1'
                $regressionShapePass = (
                    @($allRegressionTestIds | Sort-Object -Unique).Count -eq
                        $allRegressionTestIds.Count -and
                    @($allRegressionTests | Where-Object {
                            [string]$_.status -cne 'PASS' -or
                            [string]$_.validatorPath -cne $productiveValidatorRelativePath -or
                            [string]::IsNullOrWhiteSpace([string]$_.evidence)
                        }).Count -eq 0 -and
                    [string]$embeddedRegressionMatrix.validatorPath -ceq
                        $productiveValidatorRelativePath -and
                    (Test-OrdinalSetEqual $record.review.permanentRegressionEvidence $allRegressionTestIds) -and
                    (Test-OrdinalSetEqual $record.focusedDelta.regressionEvidenceIds $allRegressionTestIds) -and
                    (Test-OrdinalSetEqual $embeddedFocusedRecord.regressionTestIds $allRegressionTestIds) -and
                    (
                        $null -ne $reportContract -and
                        (Test-OrdinalSetEqual $reportContract.regressionTestIds $allRegressionTestIds)
                    )
                )
                Add-GovernanceCheck -Id 'RECORD-HANDOFF-REGRESSION-EVIDENCE-PARITY' `
                    -Passed $regressionShapePass `
                    -Message 'Every finding has unique passing permanent evidence from the productive validator path.'

                $findingSemanticPass = $findingIdParityPass
                foreach ($findingId in $expectedTargetFindings) {
                    $assignmentFinding = @($assignmentFindings | Where-Object id -ceq $findingId)
                    $completionFinding = @($completionFindings | Where-Object id -ceq $findingId)
                    $correctionFinding = @($correctionFindings | Where-Object id -ceq $findingId)
                    $regressionFinding = @($regressionFindings | Where-Object id -ceq $findingId)
                    $entryPass = (
                        $assignmentFinding.Count -eq 1 -and
                        $completionFinding.Count -eq 1 -and
                        $correctionFinding.Count -eq 1 -and
                        $regressionFinding.Count -eq 1
                    )
                    if ($entryPass) {
                        $findingRegressionIds = @(
                            $regressionFinding[0].regressionTests |
                                ForEach-Object { [string]$_.id }
                        )
                        $entryPass = (
                            [string]$assignmentFinding[0].severity -ceq
                                [string]$completionFinding[0].severity -and
                            [string]$assignmentFinding[0].severity -ceq
                                [string]$correctionFinding[0].severity -and
                            [string]$assignmentFinding[0].disposition -ceq 'CORRECTED' -and
                            [string]$completionFinding[0].disposition -ceq 'CORRECTED' -and
                            [string]$correctionFinding[0].disposition -ceq 'CORRECTED' -and
                            [string]$completionFinding[0].status -ceq
                                'CORRECTED_PENDING_DELTA' -and
                            [string]$correctionFinding[0].status -ceq
                                'CORRECTED_PENDING_DELTA' -and
                            [string]$correctionFinding[0].previousStatus -ceq
                                'PARTIALLY_CLOSED_CORRECTION_REQUIRED' -and
                            [string]$assignmentFinding[0].correction -ceq
                                [string]$completionFinding[0].correctionEvidence -and
                            [string]$assignmentFinding[0].correction -ceq
                                [string]$correctionFinding[0].correction -and
                            (Test-OrdinalSetEqual $assignmentFinding[0].affectedPaths $completionFinding[0].affectedPaths) -and
                            (Test-OrdinalSetEqual $assignmentFinding[0].affectedPaths $correctionFinding[0].affectedPaths) -and
                            (Test-OrdinalSetEqual $assignmentFinding[0].regressionEvidenceIds $completionFinding[0].regressionTestIds) -and
                            (Test-OrdinalSetEqual $assignmentFinding[0].regressionEvidenceIds $correctionFinding[0].regressionTestIds) -and
                            (Test-OrdinalSetEqual $assignmentFinding[0].regressionEvidenceIds $findingRegressionIds) -and
                            (Test-OrdinalSetEqual $completionFinding[0].evidenceReferences $correctionFinding[0].evidenceReferences) -and
                            [string]$assignmentFinding[0].evidence -in
                                @($correctionFinding[0].evidenceReferences)
                        )
                    }
                    Add-GovernanceCheck -Id ('RECORD-HANDOFF-FINDING-PARITY-' + $findingId) `
                        -Passed $entryPass `
                        -Message 'Finding severity, status, disposition, correction, paths, tests, and evidence agree across contracts.'
                    $findingSemanticPass = $findingSemanticPass -and $entryPass
                }

                $reportNarrative = if ($reportContractUnique) {
                    $embeddedReportText.Remove(
                        $reportContractMatches[0].Index,
                        $reportContractMatches[0].Length
                    )
                }
                else {
                    $embeddedReportText
                }
                $narrativeRequiredPaths = @(
                    @($trustedChangedPaths) + @($manifestExternalPaths)
                )
                $narrativePathMatch = [regex]::Match(
                    $reportNarrative,
                    '(?ms)^Repository and external correction paths:\r?\n(?<paths>.*?)^Commit Preparation remains blocked\.\r?$'
                )
                $narrativeDeclaredPaths = if ($narrativePathMatch.Success) {
                    @(
                        $narrativePathMatch.Groups['paths'].Value -split '\r?\n' |
                            Where-Object { -not [string]::IsNullOrWhiteSpace($_) }
                    )
                }
                else {
                    @()
                }
                $narrativePathPass = (
                    $narrativePathMatch.Success -and
                    (Test-OrdinalSetEqual $narrativeDeclaredPaths $narrativeRequiredPaths)
                )
                $narrativeStatusPass = (
                    $reportNarrative.Contains(
                        $expectedHandoffStatus,
                        [System.StringComparison]::Ordinal
                    ) -and
                    -not $reportNarrative.Contains(
                        'Commit Preparation approved',
                        [System.StringComparison]::OrdinalIgnoreCase
                    ) -and
                    -not $reportNarrative.Contains(
                        'independent Full Review, possible bundled correction and focused Delta',
                        [System.StringComparison]::OrdinalIgnoreCase
                    )
                )
                Add-GovernanceCheck -Id 'RECORD-HANDOFF-REPORT-NARRATIVE-PARITY' `
                    -Passed ($narrativePathPass -and $narrativeStatusPass) `
                    -Message 'Narrative report has an exact repository/external path set and contains no stale status statement.'

                $localRegisterChanges = @(
                    $embeddedExternal.changes |
                        Where-Object scope -ceq 'FLASHGATE_LOCAL_WORK_REGISTER'
                )
                $localRegisterStatusPass = $false
                if ($localRegisterChanges.Count -eq 1) {
                    $localRegisterAfterPath = [string]$localRegisterChanges[0].afterPayload
                    if ($packageEntryBytes.ContainsKey($localRegisterAfterPath)) {
                        $localRegisterAfter = $strictUtf8.GetString(
                            [byte[]]$packageEntryBytes[$localRegisterAfterPath]
                        )
                        $localRegisterStatusPass = (
                            $localRegisterAfter.Contains(
                                $expectedHandoffStatus,
                                [System.StringComparison]::Ordinal
                            ) -and
                            $localRegisterAfter.Contains(
                                [string]$embeddedCompletion.queue,
                                [System.StringComparison]::Ordinal
                            ) -and
                            $localRegisterAfter.Contains(
                                'Commit Preparation remains blocked',
                                [System.StringComparison]::Ordinal
                            ) -and
                            $localRegisterAfter.Contains(
                                'BL-335',
                                [System.StringComparison]::Ordinal
                            ) -and
                            $localRegisterAfter.Contains(
                                'ClosedFindingCount: 3',
                                [System.StringComparison]::Ordinal
                            ) -and
                            $localRegisterAfter.Contains(
                                'PendingDeltaFindingCount: 2',
                                [System.StringComparison]::Ordinal
                            ) -and
                            $localRegisterAfter.Contains(
                                'BL333-BL334-REV-007,BL333-BL334-REV-008,BL333-BL334-REV-010',
                                [System.StringComparison]::Ordinal
                            ) -and
                            $localRegisterAfter.Contains(
                                'BL333-BL334-REV-013,BL333-BL334-REV-015',
                                [System.StringComparison]::Ordinal
                            ) -and
                            -not $localRegisterAfter.Contains(
                                'independent Full Review, possible bundled correction and focused Delta',
                                [System.StringComparison]::OrdinalIgnoreCase
                            )
                        )
                    }
                }
                $statusCountParityPass = (
                    $null -ne $handoffContract -and
                    [string]$handoffContract.status -ceq $expectedHandoffStatus -and
                    (Test-OrdinalSetEqual $handoffContract.targetFindings $expectedTargetFindings) -and
                    (Test-OrdinalSetEqual $handoffContract.pendingFindings $expectedTargetFindings) -and
                    (Test-OrdinalSetEqual $handoffContract.closedFindings $expectedClosedFindings) -and
                    [int]$handoffContract.targetFindingCount -eq 2 -and
                    [int]$handoffContract.correctedFindingCount -eq 2 -and
                    [int]$handoffContract.pendingDeltaFindingCount -eq 2 -and
                    [int]$handoffContract.closedFindingCount -eq 3 -and
                    [int]$handoffContract.openFindingCount -eq 0 -and
                    [int]$embeddedCorrectionMatrix.correctedFindingCount -eq 2 -and
                    [int]$embeddedRegressionMatrix.fixtureCount -eq
                        [int]$validationSummary.fixtureCount -and
                    [string]$validationSummary.status -ceq 'PASS' -and
                    [bool]$validationSummary.handoffContractGatePassed -and
                    [bool]$validationSummary.visibleHandoffKeyGatePassed -and
                    [bool]$validationSummary.visibleHandoffParityGatePassed -and
                    [bool]$validationSummary.handoffMarkerCountGatePassed -and
                    [bool]$validationSummary.reservedControlLineGatePassed -and
                    [bool]$validationSummary.externalPathScopeMappingGatePassed -and
                    [bool]$readinessEvidence.classicReviewReady -and
                    -not [bool]$readinessEvidence.commitPreparationApproved -and
                    -not [bool]$readinessEvidence.commitAuthorized -and
                    [string]$readinessEvidence.status -ceq $expectedHandoffStatus -and
                    (Test-OrdinalSetEqual $readinessEvidence.targetFindings $expectedTargetFindings) -and
                    (Test-OrdinalSetEqual $readinessEvidence.closedFindings $expectedClosedFindings) -and
                    $null -ne $reportContract -and
                    [string]$reportContract.status -ceq $expectedHandoffStatus -and
                    (Test-OrdinalSetEqual $reportContract.findingIds $expectedTargetFindings) -and
                    (Test-OrdinalSetEqual $reportContract.closedFindings $expectedClosedFindings) -and
                    [int]$reportContract.targetFindingCount -eq 2 -and
                    [int]$reportContract.correctedFindingCount -eq 2 -and
                    [int]$reportContract.pendingDeltaFindingCount -eq 2 -and
                    [int]$reportContract.closedFindingCount -eq 3 -and
                    [int]$reportContract.openFindingCount -eq 0 -and
                    [string]$reportContract.nextAction -ceq $expectedNextAction
                )
                Add-GovernanceCheck -Id 'RECORD-HANDOFF-STATUS-COUNT-PARITY' `
                    -Passed $statusCountParityPass `
                    -Message 'Handoff, report, matrices, readiness, and validation evidence have exact status/count parity.'
                $reportStatusParityPass = (
                    $null -ne $reportContract -and
                    $null -ne $handoffContract -and
                    [string]$reportContract.executionMode -ceq
                        [string]$record.executionMode -and
                    [string]$handoffContract.correctionMode -ceq
                        [string]$record.executionMode -and
                    [string]$reportContract.reviewStatus -ceq
                        [string]$embeddedCompletion.reviewStatus -and
                    [string]$reportContract.run007Status -ceq
                        [string]$embeddedCompletion.run007Status -and
                    [string]$embeddedCompletion.run007Status -ceq
                        'CORRECTED_PENDING_DELTA' -and
                    [string]$reportContract.queue -ceq
                        [string]$embeddedCompletion.queue -and
                    [string]$handoffContract.run007Status -ceq
                        [string]$embeddedCompletion.run007Status -and
                    [string]$handoffContract.requiredReviewMode -ceq
                        [string]$embeddedFocusedRecord.mode -and
                    -not [bool]$embeddedFocusedRecord.fullReviewRepeatAuthorized -and
                    -not [bool]$embeddedFocusedRecord.commitPreparationApproved -and
                    -not [bool]$embeddedFocusedRecord.commitAuthorized -and
                    (Test-OrdinalSetEqual $handoffContract.targetFindings $record.focusedDelta.reviewedFindingIds) -and
                    (Test-OrdinalSetEqual $handoffContract.targetFindings $embeddedFocusedRecord.reviewedFindingIds) -and
                    [string]$handoffContract.nextAction -ceq
                        [string]$embeddedCompletion.nextAction -and
                    -not [bool]$record.commitPreparation.commitAuthorized -and
                    -not [bool]$record.commitPreparation.independentDeltaReviewComplete -and
                    -not [bool]$reportContract.commitPreparationApproved -and
                    -not [bool]$reportContract.commitAuthorized -and
                    -not [bool]$handoffContract.commitPreparationApproved -and
                    -not [bool]$handoffContract.commitAuthorized -and
                    [bool]$handoffContract.classicReviewReady -eq
                        [bool]$record.handoff.classicReviewReady -and
                    $localRegisterStatusPass
                )
                Add-GovernanceCheck -Id 'RECORD-HANDOFF-STATUS-QUEUE-PARITY' `
                    -Passed $reportStatusParityPass `
                    -Message 'Assignment, completion, report contract, and Local Work Register agree on status, queue, Run-007, and commit-preparation boundary.'

                $embeddedContractPass = (
                    $assignmentBindingPass -and
                    $embeddedSchemaPass -and
                    $reportContractSchemaPass -and
                    $handoffContractSchemaPass -and
                    $handoffMarkerCountGatePassed -and
                    $visibleHandoffKeyGatePassed -and
                    $handoffVisibleParityPass -and
                    $reservedControlLineGatePassed -and
                    $statusCountParityPass -and
                    $patchHashBindingPass -and
                    $currentDeltaHashBindingPass -and
                    $previousReviewBindingPass -and
                    $repositoryPathParityPass -and
                    $directInterfaceParityPass -and
                    $externalPathParityPass -and
                    $embeddedReportBindingPass -and
                    $findingSemanticPass -and
                    $regressionShapePass -and
                    $narrativePathPass -and
                    $narrativeStatusPass -and
                    $reportStatusParityPass
                )
            }
            $openFindingsForHandoff = @(
                $allFindingCollections |
                    ForEach-Object { @($_) } |
                    Where-Object { [string]$_.disposition -in @('OPEN', 'DEFERRED_WITH_BOUNDARY') }
            )

            $handoffResultsPass = (
                $currentReadinessClassPass -and
                [bool]$record.handoff.required -and
                $packageExists -and
                $packageOutsideRepository -and
                $recordPackageMatches -and
                $validatorBindingPass -and
                $artifactValidatorPass -and
                [string]$record.handoff.scopeInventoryResult -eq 'PASS' -and
                [string]$record.handoff.patchCompletenessResult -eq 'PASS' -and
                [string]$record.handoff.manifestResult -eq 'PASS' -and
                @($record.handoff.missingArtifacts).Count -eq 0 -and
                $missingNamedArtifacts.Count -eq 0 -and
                $missingActualArtifacts.Count -eq 0 -and
                $embeddedContractPass -and
                $blockingBoundaries.Count -eq 0 -and
                $openFindingsForHandoff.Count -eq 0
            )
            Add-GovernanceCheck -Id 'RECORD-CLASSIC-READY' -Passed $handoffResultsPass -Message 'ClassicReviewReady requires a complete passing handoff.' -Evidence ($missingNamedArtifacts -join ', ')
        }

        Test-RequiredProperties -Value $record.hostedCI -Required @(
            'required',
            'sourceVerified',
            'sources',
            'workflowCommit',
            'runId',
            'runAttempt',
            'event',
            'ref',
            'headSha'
        ) -Prefix 'RECORD-HOSTED-CI'
        Test-UniqueScalarArray -Value @($record.hostedCI.sources) -Id 'RECORD-HOSTED-CI-SOURCES'
        $expectedRunAttemptText = if ($ExpectedRunAttempt -ge 1) {
            [string]$ExpectedRunAttempt
        }
        else {
            ''
        }
        $expectedWorkflowValues = @(
            $ExpectedWorkflowCommit,
            $ExpectedRunId,
            $expectedRunAttemptText,
            $ExpectedEvent,
            $ExpectedRef,
            $ExpectedHeadSha
        )
        $trustedWorkflowProvenanceRequested = @(
            $expectedWorkflowValues | Where-Object { -not [string]::IsNullOrWhiteSpace([string]$_) }
        ).Count -gt 0
        if ($trustedWorkflowProvenanceRequested) {
            $trustedWorkflowProvenanceComplete = @(
                $expectedWorkflowValues | Where-Object { [string]::IsNullOrWhiteSpace([string]$_) }
            ).Count -eq 0
            Add-GovernanceCheck -Id 'RECORD-WORKFLOW-PROVENANCE-COMPLETE' `
                -Passed $trustedWorkflowProvenanceComplete `
                -Message 'Trusted workflow provenance is supplied as one complete atomic set.'
            if ($trustedWorkflowProvenanceComplete) {
                Add-GovernanceCheck -Id 'RECORD-EXPECTED-WORKFLOW-COMMIT' `
                    -Passed ([string]$record.hostedCI.workflowCommit -ceq $ExpectedWorkflowCommit) `
                    -Message 'Record workflow commit matches trusted workflow context.'
                Add-GovernanceCheck -Id 'RECORD-EXPECTED-RUN-ID' `
                    -Passed ([string]$record.hostedCI.runId -ceq $ExpectedRunId) `
                    -Message 'Record run ID matches trusted workflow context.'
                Add-GovernanceCheck -Id 'RECORD-EXPECTED-RUN-ATTEMPT' `
                    -Passed ([int]$record.hostedCI.runAttempt -eq $ExpectedRunAttempt) `
                    -Message 'Record run attempt matches trusted workflow context.'
                Add-GovernanceCheck -Id 'RECORD-EXPECTED-EVENT' `
                    -Passed ([string]$record.hostedCI.event -ceq $ExpectedEvent) `
                    -Message 'Record event matches trusted workflow context.'
                Add-GovernanceCheck -Id 'RECORD-EXPECTED-REF' `
                    -Passed ([string]$record.hostedCI.ref -ceq $ExpectedRef) `
                    -Message 'Record ref matches trusted workflow context.'
                Add-GovernanceCheck -Id 'RECORD-EXPECTED-HEAD-SHA' `
                    -Passed (
                        [string]$record.hostedCI.headSha -ceq $ExpectedHeadSha -and
                        [string]$record.currentCommit -ceq $ExpectedHeadSha
                    ) `
                    -Message 'Record head SHA and current commit match trusted workflow context.'
            }
        }
        if ([string]$record.checkpoint -in @('RELEASE_CANDIDATE', 'STABLE_RELEASE')) {
            $recordRepositorySources = @($record.hostedCI.sources | Where-Object { $_ -match '^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+@[0-9a-fA-F]{40}$' })
            $recordRunSources = @($record.hostedCI.sources | Where-Object { $_ -match '^github-actions-run:[1-9][0-9]*$' })
            $hostedCIPass = (
                [bool]$record.hostedCI.required -and
                [bool]$record.hostedCI.sourceVerified -and
                $recordRepositorySources.Count -eq 1 -and
                $recordRunSources.Count -eq 1 -and
                @($record.hostedCI.sources).Count -eq 2 -and
                -not [string]::IsNullOrWhiteSpace($ExpectedWorkflowCommit) -and
                -not [string]::IsNullOrWhiteSpace($ExpectedRunId) -and
                $ExpectedRunAttempt -ge 1 -and
                -not [string]::IsNullOrWhiteSpace($ExpectedEvent) -and
                -not [string]::IsNullOrWhiteSpace($ExpectedRef) -and
                -not [string]::IsNullOrWhiteSpace($ExpectedHeadSha) -and
                [string]$record.hostedCI.workflowCommit -ceq $ExpectedWorkflowCommit -and
                [string]$record.hostedCI.runId -ceq $ExpectedRunId -and
                [int]$record.hostedCI.runAttempt -eq $ExpectedRunAttempt -and
                [string]$record.hostedCI.event -ceq $ExpectedEvent -and
                [string]$record.hostedCI.ref -ceq $ExpectedRef -and
                [string]$record.hostedCI.headSha -ceq $ExpectedHeadSha -and
                [string]$record.currentCommit -ceq $ExpectedHeadSha -and
                $recordRepositorySources[0] -ceq (
                    ($ExpectedRepository -replace '^https://github\.com/', '' -replace '\.git$', '') +
                    '@' + $ExpectedHeadSha
                ) -and
                $recordRunSources[0] -ceq ('github-actions-run:' + $ExpectedRunId)
            )
            Add-GovernanceCheck -Id 'RECORD-RELEASE-HOSTED-CI' -Passed $hostedCIPass -Message 'Release checkpoints require verified authoritative Hosted CI sources.'
        }

        Test-RequiredProperties -Value $record.commitPreparation -Required @('allFindingsClosed', 'independentDeltaReviewComplete', 'scopeVerified', 'validationPassed', 'commitAuthorized') -Prefix 'RECORD-COMMIT'
        Add-GovernanceCheck -Id 'RECORD-COMMIT-NOT-AUTHORIZED' `
            -Passed (-not [bool]$record.commitPreparation.commitAuthorized) `
            -Message 'Governance records never authorize a commit.'
        if ([string]$record.checkpoint -eq 'PRE_COMMIT' -or $mode -eq 'COMMIT_PREPARATION') {
            $openDispositions = @('OPEN', 'DEFERRED_WITH_BOUNDARY')
            $allDispositions = @($allFindingCollections | ForEach-Object { Get-FindingDisposition -Findings $_ })
            $hasOpenFinding = @($allDispositions | Where-Object { $_ -in $openDispositions }).Count -gt 0
            $commitPass = (
                [bool]$record.commitPreparation.allFindingsClosed -and
                [bool]$record.commitPreparation.independentDeltaReviewComplete -and
                [bool]$record.commitPreparation.scopeVerified -and
                [bool]$record.commitPreparation.validationPassed -and
                [bool]$record.handoff.classicReviewReady -and
                -not $hasOpenFinding -and
                $blockingBoundaries.Count -eq 0
            )
            Add-GovernanceCheck -Id 'RECORD-COMMIT-PREPARATION' -Passed $commitPass -Message 'Commit preparation requires closed findings, completed independent delta review, Classic-ready handoff, verified scope, and passing validation.'
        }

        Test-RequiredProperties -Value $record.focusedDelta -Required @(
            'previousReviewPackage',
            'priorReviewBaselineSha256',
            'correctionStartCommit',
            'correctionPatchArtifact',
            'correctionPatchSha256',
            'currentDeltaArtifact',
            'currentDeltaSha256',
            'correctionOnlyPaths',
            'reviewedFindingIds',
            'directInterfacePaths',
            'regressionEvidenceIds',
            'allowedDeltaPaths',
            'referenceOnlyPaths'
        ) -Prefix 'RECORD-FOCUSED-DELTA'
        foreach ($pathProperty in @('correctionOnlyPaths', 'directInterfacePaths', 'allowedDeltaPaths', 'referenceOnlyPaths')) {
            Test-UniqueScalarArray -Value @($record.focusedDelta.$pathProperty) -Id "RECORD-FOCUSED-$pathProperty"
            foreach ($pathValue in @($record.focusedDelta.$pathProperty)) {
                Add-GovernanceCheck -Id ('RECORD-FOCUSED-PATH-' + [Math]::Abs(([string]$pathValue).GetHashCode())) `
                    -Passed (Test-CanonicalRepositoryPath -Path ([string]$pathValue)) `
                    -Message 'Focused-delta path is canonical repository-relative.' `
                    -Evidence ([string]$pathValue)
            }
        }
        Test-UniqueScalarArray -Value @($record.focusedDelta.reviewedFindingIds) -Id 'RECORD-FOCUSED-FINDING-IDS'
        Test-UniqueScalarArray -Value @($record.focusedDelta.regressionEvidenceIds) -Id 'RECORD-FOCUSED-EVIDENCE-IDS'

        if ($mode -eq 'FOCUSED_INDEPENDENT_DELTA_REVIEW') {
            $allowedDeltaPaths = @(
                @($record.focusedDelta.correctionOnlyPaths) +
                @($record.focusedDelta.directInterfacePaths) |
                    Sort-Object -Unique
            )
            $declaredAllowedDeltaPaths = @($record.focusedDelta.allowedDeltaPaths | Sort-Object -Unique)
            $unexpectedDeltaPaths = @($ChangedPath | Where-Object {
                    $_ -notin $allowedDeltaPaths -or $_ -notin $declaredAllowedDeltaPaths
                })
            $declaredButUnchangedPaths = @($allowedDeltaPaths | Where-Object { $_ -notin @($ChangedPath) })
            $undeclaredCorrectionPaths = @($declaredAllowedDeltaPaths | Where-Object { $_ -notin $allowedDeltaPaths })
            $referencePathMutations = @($ChangedPath | Where-Object { $_ -in @($record.focusedDelta.referenceOnlyPaths) })
            $correctedFindingIds = @(
                @($record.review.originalFindings) +
                @($record.review.correctedInRunFindings) |
                    Where-Object { [string]$_.disposition -in @('CORRECTED', 'CLOSED_BY_INDEPENDENT_REVIEW') } |
                    ForEach-Object { [string]$_.id } |
                    Sort-Object -Unique
            )
            $missingReviewedFindings = @($correctedFindingIds | Where-Object {
                    $_ -notin @($record.focusedDelta.reviewedFindingIds)
                })
            $focusedHashesPass = (
                [string]$record.focusedDelta.priorReviewBaselineSha256 -match '^[0-9a-f]{64}$' -and
                [string]$record.focusedDelta.correctionStartCommit -match '^[0-9a-f]{40}$' -and
                [string]$record.focusedDelta.correctionPatchSha256 -match '^[0-9a-f]{64}$' -and
                [string]$record.focusedDelta.currentDeltaSha256 -match '^[0-9a-f]{64}$' -and
                (
                    [string]::IsNullOrWhiteSpace($ExpectedCorrectionStartCommit) -or
                    [string]$record.focusedDelta.correctionStartCommit -ceq $ExpectedCorrectionStartCommit
                ) -and
                (
                    [string]::IsNullOrWhiteSpace($ExpectedPriorReviewBaselineSha256) -or
                    [string]$record.focusedDelta.priorReviewBaselineSha256 -ceq $ExpectedPriorReviewBaselineSha256
                ) -and
                (
                    [string]::IsNullOrWhiteSpace($ExpectedCorrectionPatchSha256) -or
                    [string]$record.focusedDelta.correctionPatchSha256 -ceq $ExpectedCorrectionPatchSha256
                ) -and
                (
                    [string]::IsNullOrWhiteSpace($ExpectedCurrentDeltaSha256) -or
                    [string]$record.focusedDelta.currentDeltaSha256 -ceq $ExpectedCurrentDeltaSha256
                )
            )
            Add-GovernanceCheck -Id 'RECORD-FOCUSED-DELTA-HASHES' -Passed $focusedHashesPass `
                -Message 'Focused delta binds the prior review, correction start, correction patch, and current delta hashes.'
            Add-GovernanceCheck -Id 'RECORD-FOCUSED-DELTA-SCOPE' `
                -Passed (
                    $unexpectedDeltaPaths.Count -eq 0 -and
                    $declaredButUnchangedPaths.Count -eq 0 -and
                    $undeclaredCorrectionPaths.Count -eq 0 -and
                    $referencePathMutations.Count -eq 0
                ) `
                -Message 'Changed paths exactly match declared correction/direct-interface paths and the allowed delta set.' `
                -Evidence (
                    @(
                        $unexpectedDeltaPaths
                        $declaredButUnchangedPaths
                        $undeclaredCorrectionPaths
                        $referencePathMutations
                    ) -join '; '
                )
            Add-GovernanceCheck -Id 'RECORD-FOCUSED-DELTA-FINDINGS' `
                -Passed ($missingReviewedFindings.Count -eq 0) `
                -Message 'Focused delta includes every corrected finding.' `
                -Evidence ($missingReviewedFindings -join '; ')
        }

        $completionReportValidated = $false
        if (-not [string]::IsNullOrWhiteSpace($CompletionReportPath)) {
            $resolvedCompletionReportPath = if ([System.IO.Path]::IsPathRooted($CompletionReportPath)) {
                [System.IO.Path]::GetFullPath($CompletionReportPath)
            }
            else {
                [System.IO.Path]::GetFullPath((Join-Path $resolvedRepositoryRoot $CompletionReportPath))
            }
            $completionReportExists = Test-Path -LiteralPath $resolvedCompletionReportPath -PathType Leaf
            Add-GovernanceCheck -Id 'COMPLETION-REPORT-EXISTS' -Passed $completionReportExists `
                -Message 'Machine-readable completion report exists.'
            if ($completionReportExists) {
                $completionSchemaErrors = @()
                $completionSchemaValid = Test-Json -LiteralPath $resolvedCompletionReportPath `
                    -SchemaFile $completionSchemaPath -ErrorVariable completionSchemaErrors `
                    -ErrorAction SilentlyContinue
                Add-GovernanceCheck -Id 'COMPLETION-REPORT-SCHEMA' -Passed $completionSchemaValid `
                    -Message 'Completion report passes the strict canonical JSON Schema.' `
                    -Evidence (@($completionSchemaErrors | ForEach-Object { $_.ToString() }) -join ' | ')
                $completionReport = Read-StrictJson -LiteralPath $resolvedCompletionReportPath
                $completionConsistencyPass = (
                    [string]$completionReport.taskId -ceq [string]$record.taskId -and
                    [string]$completionReport.repository -ceq [string]$record.repository -and
                    [string]$completionReport.baselineCommit -ceq [string]$record.baselineCommit -and
                    [string]$completionReport.currentCommit -ceq [string]$record.currentCommit -and
                    [string]$completionReport.branch -ceq [string]$record.branch -and
                    [string]$completionReport.executionMode -ceq [string]$record.executionMode -and
                    [string]$completionReport.checkpoint -ceq [string]$record.checkpoint -and
                    [bool]$completionReport.handoff.classicReviewReady -eq [bool]$record.handoff.classicReviewReady
                )
                Add-GovernanceCheck -Id 'COMPLETION-REPORT-ASSIGNMENT-CONSISTENCY' `
                    -Passed $completionConsistencyPass `
                    -Message 'Completion report agrees with assignment identity, provenance, mode, checkpoint, and readiness.'

                $completionTelemetryPass = (
                    [int]$completionReport.observedWarningCount -eq
                        ([int]$completionReport.resolvedWarningCount + [int]$completionReport.openWarningCount) -and
                    [int]$completionReport.warningCount -eq [int]$completionReport.openWarningCount -and
                    [int]$completionReport.validationExecutionCount -ge 1
                )
                Add-GovernanceCheck -Id 'COMPLETION-REPORT-TELEMETRY' `
                    -Passed $completionTelemetryPass `
                    -Message 'Completion report warning and execution counters satisfy the canonical invariants.'
                $completionPackagePass = (
                    [bool]$completionReport.packageGeneration.freshStaging -and
                    [int]$completionReport.packageGeneration.finalZipWriteCount -eq 1 -and
                    -not [bool]$completionReport.packageGeneration.inPlaceRepairPerformed -and
                    (-not [bool]$completionReport.handoff.classicReviewReady -or (
                        [bool]$completionReport.zipFreeReadinessPassed -and
                        [int]$completionReport.openWarningCount -eq 0 -and
                        [int]$completionReport.failureCount -eq 0
                    ))
                )
                Add-GovernanceCheck -Id 'COMPLETION-REPORT-ZIP-FREE-READINESS' `
                    -Passed $completionPackagePass `
                    -Message 'Completion report proves ZIP-free readiness and exactly one fresh final package write.'

                $assignmentHash = (Get-FileHash -LiteralPath $resolvedRecordPath -Algorithm SHA256).Hash.ToLowerInvariant()
                Add-GovernanceCheck -Id 'COMPLETION-REPORT-ASSIGNMENT-HASH' `
                    -Passed ([string]$completionReport.assignmentRecordSha256 -ceq $assignmentHash) `
                    -Message 'Completion report binds the exact assignment record.' `
                    -Evidence $assignmentHash

                if (-not [string]::IsNullOrWhiteSpace($ScopeInventoryPath)) {
                    $resolvedScopeInventoryPath = if ([System.IO.Path]::IsPathRooted($ScopeInventoryPath)) {
                        [System.IO.Path]::GetFullPath($ScopeInventoryPath)
                    }
                    else {
                        [System.IO.Path]::GetFullPath((Join-Path $resolvedRepositoryRoot $ScopeInventoryPath))
                    }
                    $scopeExists = Test-Path -LiteralPath $resolvedScopeInventoryPath -PathType Leaf
                    Add-GovernanceCheck -Id 'SCOPE-INVENTORY-EXISTS' -Passed $scopeExists `
                        -Message 'Scope inventory exists.'
                    if ($scopeExists) {
                        $scopeInventory = Read-StrictJson -LiteralPath $resolvedScopeInventoryPath
                        $scopeHash = (Get-FileHash -LiteralPath $resolvedScopeInventoryPath -Algorithm SHA256).Hash.ToLowerInvariant()
                        $scopePaths = @($scopeInventory.entries | ForEach-Object { [string]$_.path } | Sort-Object)
                        $reportPaths = @($completionReport.repositoryArtifacts | ForEach-Object { [string]$_ } | Sort-Object)
                        $changedPathsSorted = @($ChangedPath | Sort-Object)
                        $scopeParityPass = (
                            ($scopePaths -join "`n") -ceq ($reportPaths -join "`n") -and
                            ($scopePaths -join "`n") -ceq ($changedPathsSorted -join "`n") -and
                            [string]$completionReport.scopeInventorySha256 -ceq $scopeHash
                        )
                        Add-GovernanceCheck -Id 'COMPLETION-REPORT-SCOPE-PARITY' `
                            -Passed $scopeParityPass `
                            -Message 'Completion report, scope inventory, and changed-path sets agree exactly.' `
                            -Evidence "scopeHash=$scopeHash"
                    }
                }
                elseif ([bool]$record.handoff.classicReviewReady) {
                    Add-GovernanceCheck -Id 'SCOPE-INVENTORY-REQUIRED' -Passed $false `
                        -Message 'Classic-ready validation requires a scope inventory.'
                }
                $completionReportValidated = $completionSchemaValid -and $completionConsistencyPass -and
                    $completionTelemetryPass -and $completionPackagePass
            }
        }
        elseif ([bool]$record.handoff.classicReviewReady) {
            Add-GovernanceCheck -Id 'COMPLETION-REPORT-REQUIRED' -Passed $false `
                -Message 'Classic-ready validation requires a machine-readable completion report.'
        }

        $recordValidated = $true
    }

    if (-not [string]::IsNullOrWhiteSpace($GenericHandoffPackagePath)) {
        $genericValidatorPath = Join-Path $PSScriptRoot 'Test-GenericGovernanceHandoff.ps1'
        $genericValidatorExists = Test-Path -LiteralPath $genericValidatorPath -PathType Leaf
        Add-GovernanceCheck -Id 'GENERIC-HANDOFF-VALIDATOR-EXISTS' `
            -Passed $genericValidatorExists `
            -Message 'The task-neutral generic handoff validator exists.' `
            -Evidence $genericValidatorPath
        if ($genericValidatorExists) {
            $pwshPath = (Get-Command pwsh -ErrorAction Stop).Source
            $genericOutput = @(
                & $pwshPath -NoLogo -NoProfile -File $genericValidatorPath `
                    -PackagePath $GenericHandoffPackagePath `
                    -RepositoryRoot $resolvedRepositoryRoot
            )
            $genericExitCode = $LASTEXITCODE
            Add-GovernanceCheck -Id 'GENERIC-HANDOFF-PACKAGE' `
                -Passed ($genericExitCode -eq 0) `
                -Message 'The explicit generic handoff package passes all profile-specific gates.' `
                -Evidence ($genericOutput -join ' | ')
        }
    }

    $status = if ($script:Errors.Count -eq 0) { 'PASS' } else { 'FAIL' }
    $exitCode = if ($script:Errors.Count -eq 0) { 0 } else { 1 }
}
catch {
    $status = 'FAIL'
    $exitCode = 2
    $failureMessage = $_.Exception.Message
    Add-GovernanceCheck -Id 'INFRASTRUCTURE' -Passed $false -Message 'Governance validator could not complete.' -Evidence $failureMessage
}
finally {
    if (-not [string]::IsNullOrWhiteSpace($ReportPath)) {
        try {
            $resolvedReportPath = if ([System.IO.Path]::IsPathRooted($ReportPath)) {
                [System.IO.Path]::GetFullPath($ReportPath)
            }
            else {
                [System.IO.Path]::GetFullPath((Join-Path $resolvedRepositoryRoot $ReportPath))
            }
            $reportDirectory = Split-Path -Parent $resolvedReportPath
            [void][System.IO.Directory]::CreateDirectory($reportDirectory)
            $report = [ordered]@{
                schemaVersion  = 1
                status         = $status
                repositoryRoot = $resolvedRepositoryRoot
                recordValidated = $recordValidated
                changedPaths   = @($ChangedPath)
                runtimeCheckpoint = $RuntimeCheckpoint
                hostedCISources = @($HostedCISource)
                derivedTriggers = @($derivedTriggers)
                checkCount     = $script:Checks.Count
                errorCount     = $script:Errors.Count
                validationExecutionCount = 1
                infrastructureOrInvocationFailureCount = [int]($exitCode -eq 2)
                fullMatrixRunCount = 0
                packageWriteAttemptCount = 0
                generatedTaskControllerFileCount = 0
                generatedTaskControllerLineCount = 0
                readOnlyProbeCount = $script:Checks.Count
                failureMessage = $failureMessage
                checks         = @($script:Checks)
            }
            [System.IO.File]::WriteAllText(
                $resolvedReportPath,
                ($report | ConvertTo-Json -Depth 100),
                [System.Text.UTF8Encoding]::new($false)
            )
        }
        catch {
            $status = 'FAIL'
            $exitCode = 2
            $failureMessage = "Unable to write governance report: $($_.Exception.Message)"
        }
    }
}

[pscustomobject]@{
    Status          = $status
    RecordValidated = $recordValidated
    ChangedPathCount = @($ChangedPath).Count
    DerivedTriggers = @($derivedTriggers) -join ', '
    CheckCount      = $script:Checks.Count
    ErrorCount      = $script:Errors.Count
    ValidationExecutionCount = 1
    InfrastructureOrInvocationFailureCount = [int]($exitCode -eq 2)
    FullMatrixRunCount = 0
    PackageWriteAttemptCount = 0
    GeneratedTaskControllerFileCount = 0
    GeneratedTaskControllerLineCount = 0
    ReadOnlyProbeCount = $script:Checks.Count
    FailedChecks    = @($script:Errors | ForEach-Object { "$($_.Id): $($_.Evidence)" }) -join '; '
    ReportPath      = $resolvedReportPath
    FailureMessage  = $failureMessage
    NextAction      = if ($status -eq 'PASS') { 'Proceed to the next authorized governance checkpoint.' } else { 'Correct the failed governance checks and rerun the validator.' }
} | Format-List

exit $exitCode
