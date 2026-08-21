Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function New-GovernanceCaseDiagnostic {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][string]$ErrorClass,
        [string]$AffectedSelector,
        [string]$AffectedCaseId,
        [Parameter(Mandatory)][string]$Reason,
        [string]$RequestedValue,
        [AllowEmptyCollection()][string[]]$CanonicalCandidates = @(),
        [string]$RequiredPlatform,
        [string]$ActualPlatform,
        [AllowEmptyCollection()][string[]]$MissingCapabilities = @()
    )

    return [pscustomobject][ordered]@{
        ErrorClass = $ErrorClass
        AffectedSelector = $AffectedSelector
        AffectedCaseId = $AffectedCaseId
        Reason = $Reason
        RequestedValue = $RequestedValue
        CanonicalCandidates = @($CanonicalCandidates)
        RequiredPlatform = $RequiredPlatform
        ActualPlatform = $ActualPlatform
        MissingCapabilities = @($MissingCapabilities)
    }
}

function Get-GovernanceOrdinalSortedUniqueStrings {
    [CmdletBinding()]
    param([AllowEmptyCollection()][object[]]$Value = @())

    $set = [System.Collections.Generic.SortedSet[string]]::new(
        [System.StringComparer]::Ordinal
    )
    foreach ($item in @($Value)) {
        [void]$set.Add([string]$item)
    }
    return @($set)
}

function Get-GovernanceObjectPropertyNames {
    [CmdletBinding()]
    param([Parameter(Mandatory)][object]$Value)

    return @(Get-GovernanceOrdinalSortedUniqueStrings -Value $Value.PSObject.Properties.Name)
}

function Test-GovernanceExactProperties {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][object]$Value,
        [Parameter(Mandatory)][string[]]$Expected
    )

    $actual = @(Get-GovernanceObjectPropertyNames -Value $Value)
    $normalizedExpected = @(Get-GovernanceOrdinalSortedUniqueStrings -Value $Expected)
    return ($actual -join "`n") -ceq ($normalizedExpected -join "`n")
}

function Test-GovernanceIdentifier {
    [CmdletBinding()]
    param([AllowNull()][object]$Value)

    return $Value -is [string] -and $Value -cmatch '^[a-z0-9]+(?:-[a-z0-9]+)*$'
}

function Test-GovernanceCapabilityIdentifier {
    [CmdletBinding()]
    param([AllowNull()][object]$Value)

    return $Value -is [string] -and $Value -cmatch '^[a-z0-9]+(?:[.-][a-z0-9]+)*$'
}

function Test-GovernanceOrdinalStringArray {
    [CmdletBinding()]
    param(
        [AllowNull()][object]$Value,
        [switch]$RequireNonEmpty
    )

    if ($null -eq $Value -or $Value -is [string]) {
        return $false
    }
    $items = @($Value)
    if ($RequireNonEmpty -and $items.Count -eq 0) {
        return $false
    }
    if (@($items | Where-Object { -not (Test-GovernanceIdentifier -Value $_) }).Count -gt 0) {
        return $false
    }
    $sorted = @(Get-GovernanceOrdinalSortedUniqueStrings -Value $items)
    return $sorted.Count -eq $items.Count -and ($sorted -join "`n") -ceq ($items -join "`n")
}

function Get-GovernanceCanonicalJsonBytes {
    [CmdletBinding()]
    param([Parameter(Mandatory)][object]$Value)

    $json = ($Value | ConvertTo-Json -Depth 20 -Compress) + "`n"
    return [System.Text.UTF8Encoding]::new($false).GetBytes($json)
}

function Get-GovernanceSha256 {
    [CmdletBinding()]
    param([Parameter(Mandatory)][byte[]]$Bytes)

    return [Convert]::ToHexString(
        [System.Security.Cryptography.SHA256]::HashData($Bytes)
    ).ToLowerInvariant()
}

function Read-GovernanceCaseMetadata {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][string]$Path,
        [string]$SchemaPath = (Join-Path (Split-Path -Parent $PSScriptRoot) 'Governance/governance-case-metadata.schema.json')
    )

    $diagnostics = [System.Collections.Generic.List[object]]::new()
    $catalog = $null
    $canonicalCatalog = $null
    $metadataHash = $null
    try {
        $resolvedPath = [System.IO.Path]::GetFullPath($Path)
        $resolvedSchemaPath = [System.IO.Path]::GetFullPath($SchemaPath)
        if (-not (Test-Path -LiteralPath $resolvedPath -PathType Leaf)) {
            throw 'Metadata source is not readable.'
        }
        if (-not (Test-Path -LiteralPath $resolvedSchemaPath -PathType Leaf)) {
            throw 'Metadata schema is not readable.'
        }
        $bytes = [System.IO.File]::ReadAllBytes($resolvedPath)
        if ($bytes.Length -ge 3 -and $bytes[0] -eq 0xef -and $bytes[1] -eq 0xbb -and $bytes[2] -eq 0xbf) {
            throw 'Metadata source must be strict UTF-8 without a BOM.'
        }
        $text = [System.Text.UTF8Encoding]::new($false, $true).GetString($bytes)
        if (-not (Test-Json -Json $text -SchemaFile $resolvedSchemaPath -ErrorAction Stop)) {
            throw 'Metadata source does not satisfy its JSON schema.'
        }
        $catalog = $text | ConvertFrom-Json -Depth 30 -DateKind String
        if (-not (Test-GovernanceExactProperties -Value $catalog -Expected @(
                    'CapabilityCatalog', 'Cases', 'SchemaVersion', 'WindowsOnlyDependencyCatalog'
                ))) {
            throw 'Metadata root contains unknown or missing properties.'
        }

        $capabilityIds = [System.Collections.Generic.HashSet[string]]::new(
            [System.StringComparer]::Ordinal
        )
        $canonicalCapabilities = [System.Collections.Generic.List[object]]::new()
        foreach ($capability in @($catalog.CapabilityCatalog)) {
            if (-not (Test-GovernanceExactProperties -Value $capability -Expected @(
                        'CapabilityId', 'SupportedPlatforms'
                    ))) {
                throw 'Capability metadata contains unknown or missing properties.'
            }
            if (-not (Test-GovernanceCapabilityIdentifier -Value $capability.CapabilityId)) {
                throw 'Capability metadata contains an invalid capability ID.'
            }
            if (-not $capabilityIds.Add([string]$capability.CapabilityId)) {
                throw 'Capability metadata contains a duplicate capability ID.'
            }
            if (-not (Test-GovernanceOrdinalStringArray -Value $capability.SupportedPlatforms -RequireNonEmpty) -or
                @($capability.SupportedPlatforms | Where-Object { $_ -cnotin @('linux', 'windows') }).Count -gt 0) {
                throw 'Capability metadata contains an invalid platform set.'
            }
            $canonicalCapabilities.Add([ordered]@{
                    CapabilityId = [string]$capability.CapabilityId
                    SupportedPlatforms = @($capability.SupportedPlatforms)
                })
        }
        $sortedCapabilityIds = @(Get-GovernanceOrdinalSortedUniqueStrings -Value @($capabilityIds))
        if (($sortedCapabilityIds -join "`n") -cne (@($canonicalCapabilities.CapabilityId) -join "`n")) {
            throw 'Capability metadata is not in deterministic ordinal order.'
        }

        $dependencyIds = [System.Collections.Generic.HashSet[string]]::new(
            [System.StringComparer]::Ordinal
        )
        $dependencyCapability = [System.Collections.Generic.Dictionary[string, string]]::new(
            [System.StringComparer]::Ordinal
        )
        $canonicalDependencies = [System.Collections.Generic.List[object]]::new()
        foreach ($dependency in @(
                $catalog.WindowsOnlyDependencyCatalog | Where-Object { $null -ne $_ }
            )) {
            if (-not (Test-GovernanceExactProperties -Value $dependency -Expected @(
                        'DependencyId', 'RequiredCapability'
                    ))) {
                throw 'Windows-only dependency metadata contains unknown or missing properties.'
            }
            if (-not (Test-GovernanceIdentifier -Value $dependency.DependencyId) -or
                -not (Test-GovernanceCapabilityIdentifier -Value $dependency.RequiredCapability)) {
                throw 'Windows-only dependency metadata contains an invalid identifier.'
            }
            if (-not $dependencyIds.Add([string]$dependency.DependencyId)) {
                throw 'Windows-only dependency metadata contains a duplicate dependency ID.'
            }
            if (-not $capabilityIds.Contains([string]$dependency.RequiredCapability)) {
                throw 'Windows-only dependency metadata references an unknown capability.'
            }
            $dependencyCapability.Add(
                [string]$dependency.DependencyId,
                [string]$dependency.RequiredCapability
            )
            $canonicalDependencies.Add([ordered]@{
                    DependencyId = [string]$dependency.DependencyId
                    RequiredCapability = [string]$dependency.RequiredCapability
                })
        }
        $sortedDependencyIds = @(Get-GovernanceOrdinalSortedUniqueStrings -Value @($dependencyIds))
        $canonicalDependencyIds = @(
            $canonicalDependencies | ForEach-Object { [string]$_.DependencyId }
        )
        if (($sortedDependencyIds -join "`n") -cne ($canonicalDependencyIds -join "`n")) {
            throw 'Windows-only dependency metadata is not in deterministic ordinal order.'
        }

        $caseIds = [System.Collections.Generic.HashSet[string]]::new(
            [System.StringComparer]::Ordinal
        )
        $canonicalCases = [System.Collections.Generic.List[object]]::new()
        $expectedOrder = 0
        foreach ($case in @($catalog.Cases)) {
            $expectedOrder++
            if (-not (Test-GovernanceExactProperties -Value $case -Expected @(
                        'CaseId', 'Group', 'Order', 'RequiredCapabilities', 'SupportedPlatforms',
                        'Tags', 'WindowsOnlyDependency'
                    ))) {
                throw 'Case metadata contains unknown or missing properties.'
            }
            if ($case.Order -isnot [long] -and $case.Order -isnot [int]) {
                throw 'Case metadata Order must be an integer.'
            }
            if ([int]$case.Order -ne $expectedOrder) {
                throw 'Case metadata Order must be unique, contiguous, and deterministic.'
            }
            if (-not (Test-GovernanceIdentifier -Value $case.CaseId)) {
                throw 'Case metadata contains a missing, empty, or invalid CaseId.'
            }
            if (-not $caseIds.Add([string]$case.CaseId)) {
                throw 'Case metadata contains a duplicate CaseId.'
            }
            if (-not (Test-GovernanceIdentifier -Value $case.Group)) {
                throw 'Case metadata contains an invalid Group.'
            }
            if (-not (Test-GovernanceOrdinalStringArray -Value $case.Tags)) {
                throw 'Case metadata Tags must be unique and ordinally sorted.'
            }
            if (-not (Test-GovernanceOrdinalStringArray -Value $case.SupportedPlatforms -RequireNonEmpty) -or
                @($case.SupportedPlatforms | Where-Object { $_ -cnotin @('linux', 'windows') }).Count -gt 0) {
                throw 'Case metadata contains an unknown or contradictory platform set.'
            }
            $requiredCapabilities = @($case.RequiredCapabilities)
            $sortedRequiredCapabilities = @(
                Get-GovernanceOrdinalSortedUniqueStrings -Value $requiredCapabilities
            )
            if ($requiredCapabilities.Count -eq 0 -or
                @($requiredCapabilities | Where-Object {
                        -not (Test-GovernanceCapabilityIdentifier -Value $_)
                    }).Count -gt 0 -or
                $sortedRequiredCapabilities.Count -ne $requiredCapabilities.Count -or
                ($sortedRequiredCapabilities -join "`n") -cne ($requiredCapabilities -join "`n") -or
                @($case.RequiredCapabilities | Where-Object { -not $capabilityIds.Contains([string]$_) }).Count -gt 0) {
                throw 'Case metadata contains an unknown capability.'
            }
            if (-not (Test-GovernanceOrdinalStringArray -Value $case.WindowsOnlyDependency) -or
                @($case.WindowsOnlyDependency | Where-Object { -not $dependencyIds.Contains([string]$_) }).Count -gt 0) {
                throw 'Case metadata contains an unresolvable Windows-only dependency.'
            }
            if (@($case.WindowsOnlyDependency).Count -gt 0) {
                if (@($case.SupportedPlatforms).Count -ne 1 -or
                    [string]$case.SupportedPlatforms[0] -cne 'windows') {
                    throw 'Windows-only dependency metadata contradicts SupportedPlatforms.'
                }
                foreach ($dependencyId in @($case.WindowsOnlyDependency)) {
                    $requiredCapability = $dependencyCapability[[string]$dependencyId]
                    if ($requiredCapability -cnotin @($case.RequiredCapabilities)) {
                        throw 'Windows-only dependency capability is absent from RequiredCapabilities.'
                    }
                }
            }
            foreach ($requiredCapability in @($case.RequiredCapabilities)) {
                $capability = @($canonicalCapabilities | Where-Object {
                        [string]$_.CapabilityId -ceq [string]$requiredCapability
                    })[0]
                if (@($case.SupportedPlatforms | Where-Object {
                            $_ -cin @($capability.SupportedPlatforms)
                        }).Count -eq 0) {
                    throw 'Case platform metadata has no overlap with a required capability platform.'
                }
            }
            $canonicalCases.Add([ordered]@{
                    Order = [int]$case.Order
                    CaseId = [string]$case.CaseId
                    Group = [string]$case.Group
                    Tags = @($case.Tags)
                    SupportedPlatforms = @($case.SupportedPlatforms)
                    RequiredCapabilities = @($case.RequiredCapabilities)
                    WindowsOnlyDependency = @($case.WindowsOnlyDependency)
                })
        }
        if ($canonicalCases.Count -eq 0) {
            throw 'Canonical case metadata must contain at least one case.'
        }
        $canonicalCatalog = [ordered]@{
            SchemaVersion = 1
            CapabilityCatalog = @($canonicalCapabilities)
            WindowsOnlyDependencyCatalog = @($canonicalDependencies)
            Cases = @($canonicalCases)
        }
        $canonicalBytes = Get-GovernanceCanonicalJsonBytes -Value $canonicalCatalog
        $secondCanonicalBytes = Get-GovernanceCanonicalJsonBytes -Value $canonicalCatalog
        $firstCanonicalHash = Get-GovernanceSha256 -Bytes $canonicalBytes
        $secondCanonicalHash = Get-GovernanceSha256 -Bytes $secondCanonicalBytes
        if ($firstCanonicalHash -cne $secondCanonicalHash) {
            throw 'Metadata canonicalization is not deterministic.'
        }
        $metadataHash = $firstCanonicalHash
    }
    catch {
        $diagnostics.Add((New-GovernanceCaseDiagnostic `
                    -ErrorClass 'INVALID_METADATA' `
                    -Reason $_.Exception.Message))
    }

    $passed = $diagnostics.Count -eq 0
    return [pscustomobject][ordered]@{
        MetadataResult = if ($passed) { 'PASS' } else { 'FAIL' }
        ReadyToResolveSelectors = $passed
        RunnerProcessStartCount = 0
        ValidationExecutionCount = 0
        MetadataInventorySHA256 = $metadataHash
        CanonicalCatalog = $canonicalCatalog
        Cases = if ($passed) { @($canonicalCatalog.Cases) } else { @() }
        Groups = if ($passed) {
            @(Get-GovernanceOrdinalSortedUniqueStrings -Value $canonicalCatalog.Cases.Group)
        }
        else { @() }
        Tags = if ($passed) {
            @(Get-GovernanceOrdinalSortedUniqueStrings -Value $canonicalCatalog.Cases.Tags)
        }
        else { @() }
        Diagnostics = @($diagnostics)
    }
}

function Get-GovernanceDuplicateValues {
    [CmdletBinding()]
    param([AllowEmptyCollection()][string[]]$Value = @())

    $counts = [System.Collections.Generic.Dictionary[string, int]]::new(
        [System.StringComparer]::Ordinal
    )
    foreach ($item in @($Value)) {
        if ($counts.ContainsKey([string]$item)) {
            $counts[[string]$item]++
        }
        else {
            $counts.Add([string]$item, 1)
        }
    }
    return @(
        Get-GovernanceOrdinalSortedUniqueStrings -Value @(
            $counts.Keys | Where-Object { $counts[$_] -gt 1 }
        )
    )
}

function Resolve-GovernanceCaseSelection {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][object]$Metadata,
        [AllowEmptyCollection()][string[]]$CaseName = @(),
        [AllowEmptyCollection()][string[]]$Group = @(),
        [AllowEmptyCollection()][string[]]$Tag = @(),
        [Parameter(Mandatory)][ValidateSet('linux', 'windows')][string]$TargetPlatform,
        [AllowEmptyCollection()][string[]]$AvailableCapability = @()
    )

    $diagnostics = [System.Collections.Generic.List[object]]::new()
    $resolvedCases = @()
    $requestedCount = @($CaseName).Count + @($Group).Count + @($Tag).Count
    $unresolvedCount = 0
    $duplicateCount = 0
    $ambiguousCount = 0
    $platformIncompatibleCount = 0
    $capabilityIncompleteCount = 0

    if ([string]$Metadata.MetadataResult -cne 'PASS' -or
        -not [bool]$Metadata.ReadyToResolveSelectors) {
        foreach ($metadataDiagnostic in @($Metadata.Diagnostics)) {
            $diagnostics.Add($metadataDiagnostic)
        }
    }
    else {
        $selectorClasses = @(
            if (@($CaseName).Count -gt 0) { 'CaseName' }
            if (@($Group).Count -gt 0) { 'Group' }
            if (@($Tag).Count -gt 0) { 'Tag' }
        )
        if ($selectorClasses.Count -gt 1) {
            $ambiguousCount = $selectorClasses.Count
            $diagnostics.Add((New-GovernanceCaseDiagnostic `
                        -ErrorClass 'AMBIGUOUS_SELECTOR' `
                        -AffectedSelector 'SelectorClass' `
                        -Reason 'Selectors from more than one selector class cannot be combined.' `
                        -CanonicalCandidates $selectorClasses))
        }

        foreach ($selectorSet in @(
                [pscustomobject]@{ Name = 'CaseName'; Values = @($CaseName) },
                [pscustomobject]@{ Name = 'Group'; Values = @($Group) },
                [pscustomobject]@{ Name = 'Tag'; Values = @($Tag) }
            )) {
            $duplicates = @(Get-GovernanceDuplicateValues -Value $selectorSet.Values)
            $duplicateCount += $duplicates.Count
            foreach ($duplicate in $duplicates) {
                $diagnostics.Add((New-GovernanceCaseDiagnostic `
                            -ErrorClass 'DUPLICATE_SELECTOR' `
                            -AffectedSelector ([string]$selectorSet.Name) `
                            -Reason 'Selector value was requested more than once.' `
                            -RequestedValue $duplicate))
            }
        }

        $caseIds = @($Metadata.Cases.CaseId)
        $groups = @($Metadata.Groups)
        $tags = @($Metadata.Tags)
        foreach ($requestedCase in @($CaseName)) {
            if ($requestedCase -cnotin $caseIds) {
                $unresolvedCount++
                $diagnostics.Add((New-GovernanceCaseDiagnostic `
                            -ErrorClass 'UNRESOLVED_SELECTOR' `
                            -AffectedSelector 'CaseName' `
                            -Reason 'CaseName does not resolve to a canonical case.' `
                            -RequestedValue $requestedCase))
            }
        }
        foreach ($requestedGroup in @($Group)) {
            if ($requestedGroup -cnotin $groups) {
                $unresolvedCount++
                $diagnostics.Add((New-GovernanceCaseDiagnostic `
                            -ErrorClass 'UNRESOLVED_SELECTOR' `
                            -AffectedSelector 'Group' `
                            -Reason 'Group does not resolve to a canonical group.' `
                            -RequestedValue $requestedGroup))
            }
        }
        foreach ($requestedTag in @($Tag)) {
            if ($requestedTag -cnotin $tags) {
                $unresolvedCount++
                $diagnostics.Add((New-GovernanceCaseDiagnostic `
                            -ErrorClass 'UNRESOLVED_SELECTOR' `
                            -AffectedSelector 'Tag' `
                            -Reason 'Tag does not resolve to a canonical tag.' `
                            -RequestedValue $requestedTag))
            }
        }

        if ($diagnostics.Count -eq 0) {
            if (@($CaseName).Count -gt 0) {
                $resolvedCases = @($Metadata.Cases | Where-Object {
                        [string]$_.CaseId -cin @($CaseName)
                    })
            }
            elseif (@($Group).Count -gt 0) {
                $resolvedCases = @($Metadata.Cases | Where-Object {
                        [string]$_.Group -cin @($Group)
                    })
            }
            elseif (@($Tag).Count -gt 0) {
                $resolvedCases = @($Metadata.Cases | Where-Object {
                        $caseTags = @($_.Tags)
                        @($Tag | Where-Object { $_ -cnotin $caseTags }).Count -eq 0
                    })
            }
            else {
                $resolvedCases = @($Metadata.Cases)
            }

            if ($requestedCount -gt 0 -and $resolvedCases.Count -eq 0) {
                $diagnostics.Add((New-GovernanceCaseDiagnostic `
                            -ErrorClass 'ZERO_SELECTION' `
                            -AffectedSelector $selectorClasses[0] `
                            -Reason 'A non-empty selector request resolved to zero canonical cases.'))
            }

            foreach ($case in @($resolvedCases)) {
                if ($TargetPlatform -cnotin @($case.SupportedPlatforms) -or
                    ($TargetPlatform -ceq 'linux' -and @($case.WindowsOnlyDependency).Count -gt 0)) {
                    $platformIncompatibleCount++
                    $diagnostics.Add((New-GovernanceCaseDiagnostic `
                                -ErrorClass 'PLATFORM_INCOMPATIBLE' `
                                -AffectedCaseId ([string]$case.CaseId) `
                                -Reason 'Selected case does not support the actual platform.' `
                                -RequiredPlatform (@($case.SupportedPlatforms) -join '|') `
                                -ActualPlatform $TargetPlatform))
                }
                $applicableCapabilities = @($case.RequiredCapabilities | Where-Object {
                        $requiredCapability = $_
                        $capability = @($Metadata.CanonicalCatalog.CapabilityCatalog | Where-Object {
                                [string]$_.CapabilityId -ceq [string]$requiredCapability
                            })[0]
                        $TargetPlatform -cin @($capability.SupportedPlatforms)
                    })
                $missingCapabilities = @($applicableCapabilities | Where-Object {
                        $_ -cnotin @($AvailableCapability)
                    })
                if ($missingCapabilities.Count -gt 0) {
                    $capabilityIncompleteCount++
                    $diagnostics.Add((New-GovernanceCaseDiagnostic `
                                -ErrorClass 'CAPABILITY_INCOMPLETE' `
                                -AffectedCaseId ([string]$case.CaseId) `
                                -Reason 'Selected case requires unavailable capabilities.' `
                                -MissingCapabilities $missingCapabilities))
                }
            }
        }
    }

    $ready = $diagnostics.Count -eq 0 -and $resolvedCases.Count -gt 0
    $resolvedCaseHash = $null
    if ($resolvedCases.Count -gt 0) {
        $resolvedCaseHash = Get-GovernanceSha256 -Bytes (
            Get-GovernanceCanonicalJsonBytes -Value @($resolvedCases)
        )
    }
    return [pscustomobject][ordered]@{
        RequestedSelectorCount = $requestedCount
        ResolvedCaseCount = @($resolvedCases).Count
        UnresolvedSelectorCount = $unresolvedCount
        DuplicateSelectorCount = $duplicateCount
        AmbiguousSelectorCount = $ambiguousCount
        PlatformIncompatibleSelectorCount = $platformIncompatibleCount
        CapabilityIncompleteSelectorCount = $capabilityIncompleteCount
        ResolvedCaseIds = @($resolvedCases | ForEach-Object { [string]$_.CaseId })
        ResolvedCases = @($resolvedCases)
        ResolvedCaseSetSHA256 = $resolvedCaseHash
        MetadataInventorySHA256 = $Metadata.MetadataInventorySHA256
        SelectorResolutionResult = if ($ready) { 'PASS' } else { 'FAIL' }
        ReadyToExecute = $ready
        RunnerProcessStartCount = 0
        ValidationExecutionCount = 0
        ErrorDiagnostics = @($diagnostics)
    }
}

function Get-GovernanceCaseList {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][object]$Metadata,
        [Parameter(Mandatory)][ValidateSet('Cases', 'Groups', 'Tags')][string]$Kind
    )

    if ([string]$Metadata.MetadataResult -cne 'PASS') {
        return [pscustomobject][ordered]@{
            ListResult = 'FAIL'
            Kind = $Kind
            Values = @()
            MetadataInventorySHA256 = $Metadata.MetadataInventorySHA256
            RunnerProcessStartCount = 0
            ValidationExecutionCount = 0
            ErrorDiagnostics = @($Metadata.Diagnostics)
        }
    }
    $values = switch ($Kind) {
        'Cases' { @($Metadata.Cases.CaseId) }
        'Groups' { @($Metadata.Groups) }
        'Tags' { @($Metadata.Tags) }
    }
    return [pscustomobject][ordered]@{
        ListResult = 'PASS'
        Kind = $Kind
        Values = @($values)
        MetadataInventorySHA256 = $Metadata.MetadataInventorySHA256
        RunnerProcessStartCount = 0
        ValidationExecutionCount = 0
        ErrorDiagnostics = @()
    }
}

Export-ModuleMember -Function @(
    'Read-GovernanceCaseMetadata',
    'Resolve-GovernanceCaseSelection',
    'Get-GovernanceCaseList'
)
