[CmdletBinding()]
param(
    [string]$RepositoryRoot = (Split-Path -Parent $PSScriptRoot),
    [string]$WorkingPath = [System.IO.Path]::GetTempPath(),
    [string]$ResultPath
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Write-StrictJson {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][string]$Path,
        [Parameter(Mandatory)][object]$Value
    )

    [System.IO.File]::WriteAllText(
        $Path,
        (($Value | ConvertTo-Json -Depth 30) + [Environment]::NewLine),
        [System.Text.UTF8Encoding]::new($false)
    )
}

function Copy-JsonValue {
    [CmdletBinding()]
    param([Parameter(Mandatory)][object]$Value)

    return $Value | ConvertTo-Json -Depth 30 | ConvertFrom-Json -Depth 30 -DateKind String
}

function Test-SelectionFailureGate {
    [CmdletBinding()]
    param([Parameter(Mandatory)][object]$Selection)

    return (
        [string]$Selection.SelectorResolutionResult -ceq 'FAIL' -and
        -not [bool]$Selection.ReadyToExecute -and
        [int]$Selection.RunnerProcessStartCount -eq 0 -and
        [int]$Selection.ValidationExecutionCount -eq 0
    )
}

$status = 'FAIL'
$failureMessage = $null
$temporaryRoot = $null
$cleanupStatus = 'NOT_RUN'
$cleanupErrors = [System.Collections.Generic.List[string]]::new()
$results = [System.Collections.Generic.List[object]]::new()
$startedAt = [DateTimeOffset]::Now

try {
    $resolvedRepositoryRoot = [System.IO.Path]::GetFullPath($RepositoryRoot)
    $resolvedWorkingPath = [System.IO.Path]::GetFullPath($WorkingPath)
    if (-not (Test-Path -LiteralPath $resolvedWorkingPath -PathType Container)) {
        throw 'WorkingPath must be an existing directory.'
    }
    $temporaryRoot = Join-Path $resolvedWorkingPath (
        'governance-case-selection-' + [guid]::NewGuid().ToString('N')
    )
    [System.IO.Directory]::CreateDirectory($temporaryRoot) | Out-Null

    $modulePath = Join-Path $resolvedRepositoryRoot 'scripts/GovernanceCaseSelection.psm1'
    $metadataPath = Join-Path $resolvedRepositoryRoot 'Governance/governance-case-metadata.json'
    $schemaPath = Join-Path $resolvedRepositoryRoot 'Governance/governance-case-metadata.schema.json'
    Import-Module -Name $modulePath -Force

    $metadata = Read-GovernanceCaseMetadata -Path $metadataPath -SchemaPath $schemaPath
    $available = @('git', 'powershell-7.6.5')
    $single = Resolve-GovernanceCaseSelection -Metadata $metadata `
        -CaseName 'positive-bundled-start' -TargetPlatform windows `
        -AvailableCapability $available
    $results.Add([pscustomobject]@{
            Id = 'BL338-POS-001-SINGLE-CASENAME'
            Passed = $single.ReadyToExecute -and $single.ResolvedCaseCount -eq 1
        })

    $multiple = Resolve-GovernanceCaseSelection -Metadata $metadata `
        -CaseName @('positive-bundled-start', 'positive-independent-review') `
        -TargetPlatform windows -AvailableCapability $available
    $results.Add([pscustomobject]@{
            Id = 'BL338-POS-002-MULTIPLE-CASENAME'
            Passed = $multiple.ReadyToExecute -and $multiple.ResolvedCaseCount -eq 2
        })

    $group = Resolve-GovernanceCaseSelection -Metadata $metadata `
        -Group 'workflow-binding' -TargetPlatform windows -AvailableCapability $available
    $results.Add([pscustomobject]@{
            Id = 'BL338-POS-003-GROUP'
            Passed = $group.ReadyToExecute -and $group.ResolvedCaseCount -gt 0
        })

    $tag = Resolve-GovernanceCaseSelection -Metadata $metadata `
        -Tag 'legacy-compatibility' -TargetPlatform windows -AvailableCapability $available
    $results.Add([pscustomobject]@{
            Id = 'BL338-POS-004-TAG'
            Passed = $tag.ReadyToExecute -and $tag.ResolvedCaseCount -gt 0
        })

    $sameClassCombination = Resolve-GovernanceCaseSelection -Metadata $metadata `
        -Tag @('legacy-compatibility', 'positive') -TargetPlatform windows `
        -AvailableCapability $available
    $results.Add([pscustomobject]@{
            Id = 'BL338-POS-005-VALID-SAME-CLASS-COMBINATION'
            Passed = $sameClassCombination.ReadyToExecute -and
                $sameClassCombination.ResolvedCaseCount -gt 0
        })

    $groups = Get-GovernanceCaseList -Metadata $metadata -Kind Groups
    $tags = Get-GovernanceCaseList -Metadata $metadata -Kind Tags
    $cases = Get-GovernanceCaseList -Metadata $metadata -Kind Cases
    $results.Add([pscustomobject]@{
            Id = 'BL338-POS-006-LIST-GROUPS'
            Passed = $groups.ListResult -ceq 'PASS' -and $groups.Values.Count -gt 0
        })
    $results.Add([pscustomobject]@{
            Id = 'BL338-POS-007-LIST-TAGS'
            Passed = $tags.ListResult -ceq 'PASS' -and $tags.Values.Count -gt 0
        })
    $results.Add([pscustomobject]@{
            Id = 'BL338-POS-008-LIST-CASES'
            Passed = $cases.ListResult -ceq 'PASS' -and
                $cases.Values.Count -eq $metadata.Cases.Count
        })

    $windowsCase = Resolve-GovernanceCaseSelection -Metadata $metadata `
        -CaseName 'positive-commit-preparation' -TargetPlatform windows `
        -AvailableCapability $available
    $results.Add([pscustomobject]@{
            Id = 'BL338-POS-009-WINDOWS-COMPATIBLE'
            Passed = $windowsCase.ReadyToExecute
        })
    $linuxCase = Resolve-GovernanceCaseSelection -Metadata $metadata `
        -CaseName 'positive-bundled-start' -TargetPlatform linux `
        -AvailableCapability @('git', 'powershell-7.6.4')
    $results.Add([pscustomobject]@{
            Id = 'BL338-POS-010-PLATFORM-CAPABILITIES-COMPLETE'
            Passed = $windowsCase.CapabilityIncompleteSelectorCount -eq 0 -and
                $linuxCase.ReadyToExecute -and
                $linuxCase.CapabilityIncompleteSelectorCount -eq 0
        })

    $singleAgain = Resolve-GovernanceCaseSelection -Metadata $metadata `
        -CaseName 'positive-bundled-start' -TargetPlatform windows `
        -AvailableCapability $available
    $results.Add([pscustomobject]@{
            Id = 'BL338-POS-011-DETERMINISTIC-RE-RESOLUTION'
            Passed = $single.ResolvedCaseSetSHA256 -ceq $singleAgain.ResolvedCaseSetSHA256 -and
                ($single.ResolvedCaseIds -join "`n") -ceq ($singleAgain.ResolvedCaseIds -join "`n")
        })
    $results.Add([pscustomobject]@{
            Id = 'BL338-POS-012-CASENAME-COMPATIBILITY'
            Passed = $single.ResolvedCaseIds.Count -eq 1 -and
                [string]$single.ResolvedCaseIds[0] -ceq 'positive-bundled-start'
        })

    $unknownCase = Resolve-GovernanceCaseSelection -Metadata $metadata `
        -CaseName 'unknown-case' -TargetPlatform windows -AvailableCapability $available
    $zeroSelection = Resolve-GovernanceCaseSelection -Metadata $metadata `
        -Tag @('artifact-policy', 'completion-contract') `
        -TargetPlatform windows -AvailableCapability $available
    $results.Add([pscustomobject]@{
            Id = 'BL338-NEG-013-NONEMPTY-REQUEST-ZERO-RESOLUTION'
            Passed = (Test-SelectionFailureGate -Selection $zeroSelection) -and
                $zeroSelection.RequestedSelectorCount -gt 0 -and
                $zeroSelection.ResolvedCaseCount -eq 0 -and
                'ZERO_SELECTION' -cin @($zeroSelection.ErrorDiagnostics.ErrorClass)
        })
    $results.Add([pscustomobject]@{
            Id = 'BL338-NEG-014-UNKNOWN-CASE'
            Passed = $unknownCase.UnresolvedSelectorCount -eq 1 -and
                'UNRESOLVED_SELECTOR' -cin @($unknownCase.ErrorDiagnostics.ErrorClass)
        })

    $unknownGroup = Resolve-GovernanceCaseSelection -Metadata $metadata `
        -Group 'unknown-group' -TargetPlatform windows -AvailableCapability $available
    $results.Add([pscustomobject]@{
            Id = 'BL338-NEG-015-UNKNOWN-GROUP'
            Passed = (Test-SelectionFailureGate -Selection $unknownGroup) -and
                $unknownGroup.UnresolvedSelectorCount -eq 1
        })

    $unknownTag = Resolve-GovernanceCaseSelection -Metadata $metadata `
        -Tag 'unknown-tag' -TargetPlatform windows -AvailableCapability $available
    $results.Add([pscustomobject]@{
            Id = 'BL338-NEG-016-UNKNOWN-TAG'
            Passed = (Test-SelectionFailureGate -Selection $unknownTag) -and
                $unknownTag.UnresolvedSelectorCount -eq 1
        })

    $duplicate = Resolve-GovernanceCaseSelection -Metadata $metadata `
        -CaseName @('positive-bundled-start', 'positive-bundled-start') `
        -TargetPlatform windows -AvailableCapability $available
    $results.Add([pscustomobject]@{
            Id = 'BL338-NEG-017-DUPLICATE-SELECTOR'
            Passed = (Test-SelectionFailureGate -Selection $duplicate) -and
                $duplicate.DuplicateSelectorCount -eq 1
        })

    $ambiguous = Resolve-GovernanceCaseSelection -Metadata $metadata `
        -CaseName 'positive-bundled-start' -Tag 'positive' `
        -TargetPlatform windows -AvailableCapability $available
    $results.Add([pscustomobject]@{
            Id = 'BL338-NEG-018-AMBIGUOUS-SELECTOR'
            Passed = (Test-SelectionFailureGate -Selection $ambiguous) -and
                $ambiguous.AmbiguousSelectorCount -gt 0
        })

    $platformFailure = Resolve-GovernanceCaseSelection -Metadata $metadata `
        -CaseName 'positive-commit-preparation' -TargetPlatform linux `
        -AvailableCapability @('git', 'powershell-7.6.4')
    $results.Add([pscustomobject]@{
            Id = 'BL338-NEG-019-PLATFORM-INCOMPATIBLE'
            Passed = (Test-SelectionFailureGate -Selection $platformFailure) -and
                $platformFailure.PlatformIncompatibleSelectorCount -eq 1
        })

    $capabilityFailure = Resolve-GovernanceCaseSelection -Metadata $metadata `
        -CaseName 'positive-bundled-start' -TargetPlatform windows `
        -AvailableCapability @('git')
    $results.Add([pscustomobject]@{
            Id = 'BL338-NEG-020-CAPABILITY-INCOMPLETE'
            Passed = (Test-SelectionFailureGate -Selection $capabilityFailure) -and
                $capabilityFailure.CapabilityIncompleteSelectorCount -eq 1
        })

    $sourceCatalog = [System.IO.File]::ReadAllText(
        $metadataPath,
        [System.Text.UTF8Encoding]::new($false, $true)
    ) | ConvertFrom-Json -Depth 30 -DateKind String

    $invalidMutations = @(
        [pscustomobject]@{
            Id = 'BL338-NEG-021-DUPLICATE-CASEID'
            Mutate = { param($value) $value.Cases[1].CaseId = $value.Cases[0].CaseId }
        },
        [pscustomobject]@{
            Id = 'BL338-NEG-022-INVALID-PLATFORM'
            Mutate = { param($value) $value.Cases[0].SupportedPlatforms = @('plan9') }
        },
        [pscustomobject]@{
            Id = 'BL338-NEG-023-INVALID-CAPABILITY'
            Mutate = { param($value) $value.Cases[0].RequiredCapabilities = @('unknown-capability') }
        },
        [pscustomobject]@{
            Id = 'BL338-NEG-031-MISSING-CASEID'
            Mutate = { param($value) $value.Cases[0].PSObject.Properties.Remove('CaseId') }
        },
        [pscustomobject]@{
            Id = 'BL338-NEG-032-EMPTY-CASEID'
            Mutate = { param($value) $value.Cases[0].CaseId = '' }
        },
        [pscustomobject]@{
            Id = 'BL338-NEG-033-INVALID-GROUP'
            Mutate = { param($value) $value.Cases[0].Group = 'Invalid Group' }
        },
        [pscustomobject]@{
            Id = 'BL338-NEG-034-DUPLICATE-TAG'
            Mutate = {
                param($value)
                $value.Cases[0].Tags = @($value.Cases[0].Tags) + @($value.Cases[0].Tags[0])
            }
        },
        [pscustomobject]@{
            Id = 'BL338-NEG-035-UNKNOWN-WINDOWS-DEPENDENCY'
            Mutate = {
                param($value)
                $value.Cases[4].WindowsOnlyDependency = @('unknown-dependency')
            }
        },
        [pscustomobject]@{
            Id = 'BL338-NEG-036-NONDETERMINISTIC-ORDER'
            Mutate = { param($value) $value.Cases[1].Order = 1 }
        },
        [pscustomobject]@{
            Id = 'BL338-NEG-037-SCHEMA-DATATYPE'
            Mutate = { param($value) $value.Cases[0].Tags = 'positive' }
        }
    )
    foreach ($invalidMutation in $invalidMutations) {
        $candidate = Copy-JsonValue -Value $sourceCatalog
        & $invalidMutation.Mutate $candidate
        $candidatePath = Join-Path $temporaryRoot ($invalidMutation.Id + '.json')
        Write-StrictJson -Path $candidatePath -Value $candidate
        $invalidMetadata = Read-GovernanceCaseMetadata -Path $candidatePath -SchemaPath $schemaPath
        $results.Add([pscustomobject]@{
                Id = [string]$invalidMutation.Id
                Passed = $invalidMetadata.MetadataResult -ceq 'FAIL' -and
                    -not $invalidMetadata.ReadyToResolveSelectors -and
                    $invalidMetadata.RunnerProcessStartCount -eq 0 -and
                    $invalidMetadata.ValidationExecutionCount -eq 0
            })
    }

    $malformedPath = Join-Path $temporaryRoot 'malformed.json'
    [System.IO.File]::WriteAllText(
        $malformedPath,
        '{not-json',
        [System.Text.UTF8Encoding]::new($false)
    )
    $malformed = Read-GovernanceCaseMetadata -Path $malformedPath -SchemaPath $schemaPath
    $results.Add([pscustomobject]@{
            Id = 'BL338-NEG-024-MALFORMED-METADATA'
            Passed = $malformed.MetadataResult -ceq 'FAIL' -and
                $malformed.RunnerProcessStartCount -eq 0 -and
                $malformed.ValidationExecutionCount -eq 0
        })

    $mixed = Resolve-GovernanceCaseSelection -Metadata $metadata `
        -CaseName @('positive-bundled-start', 'unknown-case') `
        -TargetPlatform windows -AvailableCapability $available
    $results.Add([pscustomobject]@{
            Id = 'BL338-NEG-025-MIXED-VALID-INVALID-FAILS-WHOLE-SELECTION'
            Passed = (Test-SelectionFailureGate -Selection $mixed) -and
                $mixed.ResolvedCaseCount -eq 0
        })
    $results.Add([pscustomobject]@{
            Id = 'BL338-NEG-026-SELECTOR-FAILURE-NO-RUNNER'
            Passed = $mixed.RunnerProcessStartCount -eq 0
        })
    $results.Add([pscustomobject]@{
            Id = 'BL338-NEG-027-SELECTOR-FAILURE-NO-VALIDATION'
            Passed = $mixed.ValidationExecutionCount -eq 0
        })

    $driftCatalog = Copy-JsonValue -Value $sourceCatalog
    $driftCatalog.Cases[0].Tags = @('aaa-drift') + @($driftCatalog.Cases[0].Tags)
    $driftPath = Join-Path $temporaryRoot 'metadata-drift.json'
    Write-StrictJson -Path $driftPath -Value $driftCatalog
    $driftMetadata = Read-GovernanceCaseMetadata -Path $driftPath -SchemaPath $schemaPath
    $results.Add([pscustomobject]@{
            Id = 'BL338-POS-028-METADATA-HASH-DRIFT'
            Passed = $driftMetadata.MetadataResult -ceq 'PASS' -and
                $driftMetadata.MetadataInventorySHA256 -cne $metadata.MetadataInventorySHA256
        })

    $results.Add([pscustomobject]@{
            Id = 'BL338-POS-029-RESOLVED-CASESET-HASH-DRIFT'
            Passed = $single.ResolvedCaseSetSHA256 -cne $multiple.ResolvedCaseSetSHA256
        })
    $results.Add([pscustomobject]@{
            Id = 'BL338-POS-030-SAME-INPUT-SAME-HASH-AND-ORDER'
            Passed = $single.ResolvedCaseSetSHA256 -ceq $singleAgain.ResolvedCaseSetSHA256 -and
                ($single.ResolvedCaseIds -join "`n") -ceq ($singleAgain.ResolvedCaseIds -join "`n")
        })

    $missingSource = Read-GovernanceCaseMetadata `
        -Path (Join-Path $temporaryRoot 'absent.json') -SchemaPath $schemaPath
    $results.Add([pscustomobject]@{
            Id = 'BL338-NEG-038-UNREADABLE-METADATA-SOURCE'
            Passed = $missingSource.MetadataResult -ceq 'FAIL' -and
                $missingSource.RunnerProcessStartCount -eq 0
        })

    $failed = @($results | Where-Object { -not [bool]$_.Passed })
    $status = if ($failed.Count -eq 0) { 'PASS' } else { 'FAIL' }
}
catch {
    $failureMessage = $_.Exception.Message
    $status = 'FAIL'
}
finally {
    try {
        if ($null -ne $temporaryRoot -and
            (Test-Path -LiteralPath $temporaryRoot -PathType Container)) {
            $resolvedTemporaryRoot = [System.IO.Path]::GetFullPath($temporaryRoot)
            $resolvedWorkingPath = [System.IO.Path]::GetFullPath($WorkingPath)
            if ($resolvedTemporaryRoot.StartsWith(
                    $resolvedWorkingPath.TrimEnd([System.IO.Path]::DirectorySeparatorChar) +
                        [System.IO.Path]::DirectorySeparatorChar,
                    [System.StringComparison]::OrdinalIgnoreCase
                ) -and
                [System.IO.Path]::GetFileName($resolvedTemporaryRoot).StartsWith(
                    'governance-case-selection-',
                    [System.StringComparison]::Ordinal
                )) {
                Remove-Item -LiteralPath $resolvedTemporaryRoot -Recurse -Force
            }
            else {
                throw 'Temporary fixture root failed the bounded cleanup check.'
            }
        }
    }
    catch {
        $cleanupErrors.Add($_.Exception.Message)
    }
    $cleanupStatus = if ($cleanupErrors.Count -eq 0) { 'PASS' } else { 'FAIL' }
}

if ($cleanupStatus -cne 'PASS') {
    $status = 'FAIL'
}
$completedAt = [DateTimeOffset]::Now
$failedResults = @($results | Where-Object { -not [bool]$_.Passed })
$failureCount = $failedResults.Count
if (-not [string]::IsNullOrWhiteSpace($failureMessage)) {
    $failureCount++
}
if ($cleanupErrors.Count -gt 0) {
    $failureCount += $cleanupErrors.Count
}
$finalResult = [pscustomobject][ordered]@{
    Status = $status
    StartedAt = $startedAt.ToString('o')
    CompletedAt = $completedAt.ToString('o')
    FixtureCount = $results.Count
    PassedCount = @($results | Where-Object { [bool]$_.Passed }).Count
    FailedCount = $failedResults.Count
    WarningCount = 0
    FailureCount = $failureCount
    RunnerProcessStartCount = 0
    ValidationExecutionCount = 0
    CleanupStatus = $cleanupStatus
    CleanupErrors = @($cleanupErrors)
    FailureMessage = $failureMessage
    FailedFixtureIds = @($failedResults | ForEach-Object { [string]$_.Id })
    Results = @($results)
}

$json = ($finalResult | ConvertTo-Json -Depth 12 -Compress) + [Environment]::NewLine
if (-not [string]::IsNullOrWhiteSpace($ResultPath)) {
    $resolvedResultPath = [System.IO.Path]::GetFullPath($ResultPath)
    if (Test-Path -LiteralPath $resolvedResultPath) {
        throw 'ResultPath must not exist before fixture execution.'
    }
    [System.IO.File]::WriteAllText(
        $resolvedResultPath,
        $json,
        [System.Text.UTF8Encoding]::new($false)
    )
}
$json

if ($status -ceq 'PASS') {
    exit 0
}
exit 1
