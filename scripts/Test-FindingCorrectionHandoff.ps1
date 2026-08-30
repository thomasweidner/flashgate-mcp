#requires -Version 7.6
[CmdletBinding()]
param(
    [Parameter(Mandatory)][string]$PackagePath,
    [Parameter(Mandatory)][string]$RepositoryRoot,
    [Parameter(Mandatory)][string]$AuthoritativeRepositoryRoot,
    [string]$IndependentReviewOutcomePath,
    [string]$ExpectedIndependentReviewOutcomeSha256,
    [switch]$ReturnInsteadOfExit
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$status = 'FAIL'
$failureMessage = $null
$checks = [System.Collections.Generic.List[object]]::new()
$context = $null
$archive = $null
$archiveStream = $null
$lifecycleState = 'UNKNOWN'
$independentReviewOutcomeValidation = 'NOT_APPLICABLE'
$independentReviewOutcomeSha256 = $null
$immediatePreviousReviewPackageSha256 = $null
$transitiveFullReviewBaselineSha256 = $null
$findingDispositionBindingResult = 'NOT_APPLICABLE'

. (Join-Path $PSScriptRoot 'GenericGovernanceGitEvidence.ps1')
Import-Module (Join-Path $PSScriptRoot 'GovernanceValidationOrchestration.psm1') -Force

function Add-Result {
    param([string]$Id, [bool]$Passed, [string]$Evidence = '')
    [void]$checks.Add([pscustomobject][ordered]@{
        Id = $Id
        Result = if ($Passed) { 'PASS' } else { 'FAIL' }
        Evidence = $Evidence
    })
    if (-not $Passed) { throw "[$Id] $Evidence" }
}

function Get-Hash {
    param([Parameter(Mandatory)][AllowEmptyCollection()][byte[]]$Bytes)
    return [Convert]::ToHexString(
        [System.Security.Cryptography.SHA256]::HashData($Bytes)
    ).ToLowerInvariant()
}

function Read-JsonBytes {
    param(
        [Parameter(Mandatory)][byte[]]$Bytes,
        [Parameter(Mandatory)][string]$Name,
        [string]$SchemaName,
        [int]$ExpectedSchemaVersion = 0
    )
    $arguments = @{
        Bytes = $Bytes
        Label = $Name
    }
    if (-not [string]::IsNullOrWhiteSpace($SchemaName)) {
        $arguments.SchemaPath = Join-Path $RepositoryRoot "Governance/$SchemaName"
    }
    if ($ExpectedSchemaVersion -gt 0) {
        $arguments.ExpectedSchemaVersion = $ExpectedSchemaVersion
    }
    return Read-GovernanceJsonContract @arguments
}

function Read-IndependentReviewOutcome {
    param(
        [Parameter(Mandatory)][string]$LiteralPath,
        [Parameter(Mandatory)][string]$ExpectedSha256,
        [Parameter(Mandatory)][string]$ExpectedTaskId
    )

    if ([string]::IsNullOrWhiteSpace($LiteralPath) -or
        [string]::IsNullOrWhiteSpace($ExpectedSha256)) {
        throw '[CORRECTION-INDEPENDENT-OUTCOME-INPUT] Focused-to-focused validation requires path and expected SHA-256.'
    }
    $resolvedPath = [System.IO.Path]::GetFullPath($LiteralPath)
    Add-Result -Id 'CORRECTION-INDEPENDENT-OUTCOME-EXISTS' -Passed (
        Test-Path -LiteralPath $resolvedPath -PathType Leaf
    ) -Evidence $resolvedPath
    $item = Get-Item -LiteralPath $resolvedPath -Force
    Add-Result -Id 'CORRECTION-INDEPENDENT-OUTCOME-REGULAR-FILE' -Passed (
        -not $item.PSIsContainer -and
        ($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -eq 0 -and
        [string]::IsNullOrWhiteSpace([string]$item.LinkType)
    )
    $bytes = [System.IO.File]::ReadAllBytes($resolvedPath)
    Add-Result -Id 'CORRECTION-INDEPENDENT-OUTCOME-NO-BOM' -Passed (
        $bytes.Length -lt 3 -or
        -not ($bytes[0] -eq 0xef -and $bytes[1] -eq 0xbb -and $bytes[2] -eq 0xbf)
    )
    $actualSha256 = (Get-Hash -Bytes $bytes).ToUpperInvariant()
    Add-Result -Id 'CORRECTION-INDEPENDENT-OUTCOME-HASH' -Passed (
        $actualSha256 -ceq $ExpectedSha256.ToUpperInvariant()
    ) -Evidence "actual=$actualSha256;expected=$($ExpectedSha256.ToUpperInvariant())"
    $contract = Read-JsonBytes -Bytes $bytes -Name 'independent review outcome' `
        -SchemaName 'generic-independent-review-evidence.schema.json' -ExpectedSchemaVersion 1
    Add-Result -Id 'CORRECTION-INDEPENDENT-OUTCOME-IDENTITY' -Passed (
        [string]$contract.artifactType -ceq 'INDEPENDENT_FOCUSED_DELTA_REVIEW_OUTCOME' -and
        [string]$contract.taskId -ceq $ExpectedTaskId -and
        [string]$contract.reviewMode -ceq 'FOCUSED_INDEPENDENT_DELTA_REVIEW' -and
        [string]$contract.reviewerRole -ceq 'INDEPENDENT_REVIEWER'
    )
    return [pscustomobject][ordered]@{
        Path = $resolvedPath
        Bytes = $bytes
        Sha256 = $actualSha256
        Contract = $contract
    }
}

function Read-EmbeddedJsonContract {
    param(
        [Parameter(Mandatory)][byte[]]$Bytes,
        [Parameter(Mandatory)][string]$Name,
        [Parameter(Mandatory)][string]$BeginMarker,
        [Parameter(Mandatory)][string]$EndMarker,
        [string]$SchemaName,
        [int]$ExpectedSchemaVersion = 0
    )

    $text = [System.Text.UTF8Encoding]::new($false, $true).GetString($Bytes)
    if ($text.Contains([char]0xfffd) -or $text.IndexOf([char]0) -ge 0) {
        throw "[$Name] Invalid strict UTF-8 text."
    }
    if ([regex]::Matches($text, [regex]::Escape($BeginMarker)).Count -ne 1 -or
        [regex]::Matches($text, [regex]::Escape($EndMarker)).Count -ne 1) {
        throw "[$Name] Expected exactly one embedded contract."
    }
    $pattern = '(?s)' + [regex]::Escape($BeginMarker) + '\r?\n(?<json>.*?)\r?\n' + [regex]::Escape($EndMarker)
    $match = [regex]::Match($text, $pattern)
    if (-not $match.Success) {
        throw "[$Name] Embedded contract markers are not canonical."
    }
    $jsonBytes = [System.Text.UTF8Encoding]::new($false, $true).GetBytes($match.Groups['json'].Value)
    return Read-JsonBytes -Bytes $jsonBytes -Name "$Name embedded contract" `
        -SchemaName $SchemaName -ExpectedSchemaVersion $ExpectedSchemaVersion
}

function Get-CanonicalSet {
    param([object[]]$Value, [string]$Label)
    $items = @($Value | ForEach-Object { [string]$_ })
    $ordinal = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::Ordinal)
    $ignoreCase = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::OrdinalIgnoreCase)
    foreach ($item in $items) {
        if ([string]::IsNullOrWhiteSpace($item) -or -not $ordinal.Add($item) -or -not $ignoreCase.Add($item)) {
            throw "[$Label] Empty, duplicate, or case-colliding value: $item"
        }
    }
    [array]::Sort($items, [System.StringComparer]::Ordinal)
    return $items
}

function Assert-EqualSet {
    param([object[]]$Left, [object[]]$Right, [string]$Label)
    $leftSet = @(Get-CanonicalSet -Value $Left -Label "$Label-left")
    $rightSet = @(Get-CanonicalSet -Value $Right -Label "$Label-right")
    Add-Result -Id $Label -Passed (($leftSet -join "`n") -ceq ($rightSet -join "`n")) `
        -Evidence "left=$($leftSet.Count);right=$($rightSet.Count)"
}

function Assert-Subset {
    param([object[]]$Subset, [object[]]$Superset, [string]$Label)
    $left = @(Get-CanonicalSet -Value $Subset -Label "$Label-subset")
    $right = @(Get-CanonicalSet -Value $Superset -Label "$Label-superset")
    $rightSet = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::Ordinal)
    foreach ($item in $right) { [void]$rightSet.Add($item) }
    Add-Result -Id $Label -Passed (@($left | Where-Object { -not $rightSet.Contains($_) }).Count -eq 0)
}

function Read-ReviewZipEntries {
    param([Parameter(Mandatory)][string]$LiteralPath, [Parameter(Mandatory)][string]$Label)

    $resolvedPath = [System.IO.Path]::GetFullPath($LiteralPath)
    Add-Result -Id "$Label-EXISTS" -Passed (Test-Path -LiteralPath $resolvedPath -PathType Leaf) `
        -Evidence $resolvedPath
    $entries = [System.Collections.Generic.Dictionary[string, byte[]]]::new(
        [System.StringComparer]::Ordinal
    )
    $names = [System.Collections.Generic.HashSet[string]]::new(
        [System.StringComparer]::OrdinalIgnoreCase
    )
    $zip = [System.IO.Compression.ZipFile]::OpenRead($resolvedPath)
    try {
        foreach ($entry in $zip.Entries) {
            $name = [string]$entry.FullName
            if ($name -notmatch '^[A-Za-z0-9][A-Za-z0-9._-]*$' -or
                -not $names.Add($name) -or $entries.ContainsKey($name)) {
                throw "[$Label] Unsafe, duplicate, or case-colliding member: $name"
            }
            $unixType = ([uint32]$entry.ExternalAttributes -shr 16) -band 0xf000
            if ($unixType -eq 0xa000 -or ([uint32]$entry.ExternalAttributes -band 0x400) -ne 0) {
                throw "[$Label] Link or reparse ZIP semantics: $name"
            }
            $memory = [System.IO.MemoryStream]::new()
            try {
                $stream = $entry.Open()
                try { $stream.CopyTo($memory) } finally { $stream.Dispose() }
                $entries.Add($name, $memory.ToArray())
            }
            finally { $memory.Dispose() }
        }
    }
    finally { $zip.Dispose() }
    return $entries
}

function Assert-ManifestInventoryCoverage {
    param(
        [Parameter(Mandatory)]$Entries,
        [Parameter(Mandatory)]$Inventory,
        [Parameter(Mandatory)][string]$Label
    )

    $inventoryEntries = @($Inventory.entries)
    $inventoryNames = @($Entries.Keys | Where-Object { $_ -notin @('MANIFEST.sha256', 'package-inventory.json') })
    Assert-EqualSet @($inventoryEntries | ForEach-Object path) $inventoryNames "$Label-INVENTORY-COVERAGE"
    foreach ($inventoryEntry in $inventoryEntries) {
        $name = [string]$inventoryEntry.path
        Add-Result -Id "$Label-INVENTORY-$name" -Passed (
            $Entries.ContainsKey($name) -and
            [string]$inventoryEntry.sha256 -ceq (Get-Hash $Entries[$name]) -and
            [int64]$inventoryEntry.length -eq $Entries[$name].LongLength
        )
    }

    $manifestText = [System.Text.UTF8Encoding]::new($false, $true).GetString($Entries['MANIFEST.sha256'])
    $manifestNames = @($manifestText.TrimEnd("`r", "`n") -split "`n" | ForEach-Object {
            $match = [regex]::Match(
                $_,
                '^(?<hash>[0-9a-f]{64})  (?<length>[0-9]+)  (?<path>[A-Za-z0-9][A-Za-z0-9._-]*)$'
            )
            if (-not $match.Success) { throw "[$Label] Invalid manifest line: $_" }
            $name = $match.Groups['path'].Value
            if (-not $Entries.ContainsKey($name) -or
                $match.Groups['hash'].Value -cne (Get-Hash $Entries[$name]) -or
                [int64]$match.Groups['length'].Value -ne $Entries[$name].LongLength) {
                throw "[$Label] Manifest mismatch: $name"
            }
            $name
        })
    Assert-EqualSet $manifestNames @($Entries.Keys | Where-Object { $_ -cne 'MANIFEST.sha256' }) `
        "$Label-MANIFEST-COVERAGE"
}

function Get-IndependentOutcomeState {
    param([Parameter(Mandatory)]$Outcome, [Parameter(Mandatory)][string]$Label)

    $isGeneralized = $null -ne $Outcome.PSObject.Properties['targetFindingOutcomes']
    if ($isGeneralized) {
        $targetOutcomes = @($Outcome.targetFindingOutcomes)
        $newFindings = @($Outcome.newFindings)
        $inheritedClosed = @(Get-CanonicalSet @($Outcome.inheritedClosedFindingIds) "$Label-INHERITED")
        $targetIds = @(Get-CanonicalSet @($targetOutcomes | ForEach-Object id) "$Label-TARGETS")
        $newIds = @(Get-CanonicalSet @($newFindings | ForEach-Object id) "$Label-NEW")
        $targetClosed = @($targetOutcomes | Where-Object {
                [string]$_.disposition -ceq 'CLOSED_BY_INDEPENDENT_DELTA_REVIEW'
            } | ForEach-Object id)
        $targetOpen = @($targetOutcomes | Where-Object {
                [string]$_.disposition -ceq 'OPEN_INCOMPLETE_CORRECTION'
            } | ForEach-Object id)
        Add-Result -Id "$Label-TARGET-DISPOSITIONS" -Passed (
            $targetClosed.Count + $targetOpen.Count -eq $targetOutcomes.Count
        )
        Add-Result -Id "$Label-NEW-DISPOSITIONS" -Passed (
            @($newFindings | Where-Object { [string]$_.disposition -cne 'OPEN' }).Count -eq 0
        )
        $previousIds = @(Get-CanonicalSet @($inheritedClosed + $targetIds) "$Label-PREVIOUS-UNIVERSE")
        $openIds = @(Get-CanonicalSet @($targetOpen + $newIds) "$Label-DERIVED-OPEN")
        $closedIds = @(Get-CanonicalSet @($inheritedClosed + $targetClosed) "$Label-DERIVED-CLOSED")
        $authoritativeIds = @(Get-CanonicalSet @($previousIds + $newIds) "$Label-AUTHORITATIVE-UNIVERSE")
        $resultValid = switch ([string]$Outcome.reviewResult) {
            'PASS' {
                $openIds.Count -eq 0 -and $newIds.Count -eq 0 -and
                $targetClosed.Count -eq $targetOutcomes.Count
            }
            'FAIL_WITH_FINDINGS' { $openIds.Count -gt 0 }
            default { $false }
        }
    }
    else {
        $targetOutcomes = @($Outcome.findingOutcomes)
        $targetIds = @(Get-CanonicalSet @($targetOutcomes | ForEach-Object id) "$Label-LEGACY-TARGETS")
        $newIds = @()
        $openIds = @($targetOutcomes | Where-Object {
                [string]$_.disposition -ceq 'OPEN_INCOMPLETE_CORRECTION'
            } | ForEach-Object id)
        $closedIds = @($targetOutcomes | Where-Object {
                [string]$_.disposition -ceq 'CLOSED_BY_INDEPENDENT_DELTA_REVIEW'
            } | ForEach-Object id)
        Add-Result -Id "$Label-LEGACY-DISPOSITIONS" -Passed (
            $openIds.Count + $closedIds.Count -eq $targetOutcomes.Count
        )
        $previousIds = $targetIds
        $authoritativeIds = $targetIds
        $resultValid = [string]$Outcome.reviewResult -ceq 'FAIL_WITH_FINDING' -and $openIds.Count -gt 0
    }

    Assert-EqualSet @($Outcome.openFindingIds) $openIds "$Label-OPEN-SET"
    Assert-EqualSet @($Outcome.closedFindingIds) $closedIds "$Label-CLOSED-SET"
    $declaredOpen = @(Get-CanonicalSet @($Outcome.openFindingIds) "$Label-DECLARED-OPEN")
    $declaredClosed = @(Get-CanonicalSet @($Outcome.closedFindingIds) "$Label-DECLARED-CLOSED")
    $closedSet = [System.Collections.Generic.HashSet[string]]::new(
        [System.StringComparer]::OrdinalIgnoreCase
    )
    foreach ($id in $declaredClosed) { [void]$closedSet.Add($id) }
    Add-Result -Id "$Label-OPEN-CLOSED-DISJOINT" -Passed (
        @($declaredOpen | Where-Object { $closedSet.Contains($_) }).Count -eq 0
    )
    Assert-EqualSet @($declaredOpen + $declaredClosed) $authoritativeIds "$Label-STATE-COMPLETE"
    Add-Result -Id "$Label-REVIEW-RESULT" -Passed $resultValid

    $directOutcomes = @($Outcome.directInterfaceOutcomes)
    $directIds = @(Get-CanonicalSet @($directOutcomes | ForEach-Object id) "$Label-DIRECT-INTERFACES")
    Add-Result -Id "$Label-DIRECT-DISPOSITIONS" -Passed (
        @($directOutcomes | Where-Object { [string]$_.disposition -cne 'CLOSED' }).Count -eq 0 -and
        $directIds.Count -eq $directOutcomes.Count
    )

    return [pscustomobject][ordered]@{
        IsGeneralized = $isGeneralized
        TargetFindingIds = $targetIds
        NewFindingIds = $newIds
        PreviousFindingIds = $previousIds
        OpenFindingIds = $openIds
        ClosedFindingIds = $closedIds
        AuthoritativeFindingIds = $authoritativeIds
    }
}

function Get-FocusedPackageContract {
    param(
        [Parameter(Mandatory)]$Entries,
        [Parameter(Mandatory)][string]$Label,
        [Parameter(Mandatory)][string]$ExpectedTaskId
    )

    $assignment = Read-JsonBytes $Entries['assignment-record.json'] "$Label assignment" `
        'finding-correction-assignment.schema.json' 2
    $completion = Read-JsonBytes $Entries['completion-report.json'] "$Label completion" `
        'finding-correction-completion.schema.json' 2
    $inventory = Read-JsonBytes $Entries['package-inventory.json'] "$Label inventory" '' 1
    $focused = Read-JsonBytes $Entries['focused-delta-review-record.json'] "$Label focused record" `
        'focused-delta-review-record.schema.json' 3

    Add-Result -Id "$Label-PROFILE" -Passed (
        [string]$assignment.profile -ceq 'FINDING_CORRECTION' -and
        [string]$completion.profile -ceq 'FINDING_CORRECTION' -and
        [string]$inventory.profile -ceq 'FINDING_CORRECTION' -and
        [string]$assignment.transitionType -ceq 'BUNDLED_CORRECTION_TO_FOCUSED_DELTA_REVIEW' -and
        [string]$completion.transitionType -ceq 'BUNDLED_CORRECTION_TO_FOCUSED_DELTA_REVIEW' -and
        [string]$inventory.transitionType -ceq 'BUNDLED_CORRECTION_TO_FOCUSED_DELTA_REVIEW'
    )
    Add-Result -Id "$Label-IDENTITY" -Passed (
        [string]$assignment.taskId -ceq $ExpectedTaskId -and
        [string]$completion.taskId -ceq $ExpectedTaskId -and
        [string]$inventory.taskId -ceq $ExpectedTaskId
    )

    $requiredNames = @(
        'HANDOFF.md', 'assignment-record.json', 'completion-report.json',
        'correction-only.patch', 'correction-scope-inventory.json',
        'current-delta.patch', 'external-governance-manifest.json',
        'finding-correction-matrix.json', 'finding-ledger.json',
        'finding-regression-matrix.json', 'focused-delta-review-record.json',
        'MANIFEST.sha256', 'package-inventory.json', 'previous-review-binding.json',
        'readiness-evidence.json', 'report.md', 'scope-inventory.json',
        'trusted-expected-hashes.json', 'validation-summary.json'
    )
    $outcomeBindings = @(
        $assignment.PSObject.Properties['previousIndependentReviewOutcomeSha256'],
        $completion.PSObject.Properties['previousIndependentReviewOutcomeSha256'],
        $focused.PSObject.Properties['previousIndependentReviewOutcomeSha256']
    )
    $outcomeBindingCount = @($outcomeBindings | Where-Object { $null -ne $_ }).Count
    Add-Result -Id "$Label-OUTCOME-BINDING-SHAPE" -Passed ($outcomeBindingCount -in @(0, 3))
    $outcomeRequired = $outcomeBindingCount -eq 3
    if ($outcomeRequired) { $requiredNames += 'previous-independent-review-outcome.json' }

    $publicationRequired = $null -ne $focused.PSObject.Properties['publicationRegressionEvidence']
    if ($publicationRequired) {
        $requiredNames += @('publication-regression-evidence.json', 'publication-regression-result.json')
    }
    $focusedSourceRequired = $Entries.ContainsKey('focused-validation-result.json')
    if ($focusedSourceRequired) {
        $requiredNames += 'focused-validation-result.json'
    }
    Assert-EqualSet @($Entries.Keys) $requiredNames "$Label-PROFILE-DRIVEN-PACKAGE-SHAPE"
    Assert-ManifestInventoryCoverage -Entries $Entries -Inventory $inventory -Label $Label

    if ($focusedSourceRequired) {
        $regression = Read-JsonBytes $Entries['finding-regression-matrix.json'] `
            "$Label finding regression" 'finding-regression-matrix.schema.json' 2
        $sourceResult = Read-JsonBytes $Entries['focused-validation-result.json'] `
            "$Label focused validation source" '' 2
        $sourceBinding = $regression.finalFocusedValidationEvidence
        $sourceRows = @($sourceResult.results)
        $regressionRows = @($regression.findings | ForEach-Object regressionTests)
        Add-Result -Id "$Label-FOCUSED-SOURCE-BYTE-BINDING" -Passed (
            [string]$sourceBinding.sourceArtifact -ceq 'focused-validation-result.json' -and
            [int64]$sourceBinding.sourceEvidenceLength -eq
                $Entries['focused-validation-result.json'].LongLength -and
            [string]$sourceBinding.sourceEvidenceSha256 -ceq
                (Get-Hash $Entries['focused-validation-result.json']) -and
            [string]$sourceResult.status -ceq 'PASS' -and
            [int]$sourceResult.selected -eq $sourceRows.Count -and
            [int]$sourceResult.passed -eq @($sourceRows | Where-Object result -ceq 'PASS').Count -and
            [int]$sourceResult.failed -eq 0
        )
        Assert-EqualSet -Left @($sourceRows | ForEach-Object id) `
            -Right @($regressionRows | ForEach-Object id) `
            -Label "$Label-FOCUSED-SOURCE-RESULT-SET"
    }

    $embeddedOutcome = $null
    $embeddedOutcomeHash = $null
    if ($outcomeRequired) {
        $embeddedOutcomeHash = (Get-Hash $Entries['previous-independent-review-outcome.json']).ToUpperInvariant()
        Add-Result -Id "$Label-OUTCOME-HASH-PARITY" -Passed (
            [string]$assignment.previousIndependentReviewOutcomeSha256 -ceq $embeddedOutcomeHash -and
            [string]$completion.previousIndependentReviewOutcomeSha256 -ceq $embeddedOutcomeHash -and
            [string]$focused.previousIndependentReviewOutcomeSha256 -ceq $embeddedOutcomeHash
        )
        $embeddedOutcome = Read-JsonBytes $Entries['previous-independent-review-outcome.json'] `
            "$Label embedded independent outcome" 'generic-independent-review-evidence.schema.json' 1
    }

    return [pscustomobject][ordered]@{
        Assignment = $assignment
        Completion = $completion
        Inventory = $inventory
        Focused = $focused
        EmbeddedOutcome = $embeddedOutcome
        EmbeddedOutcomeHash = $embeddedOutcomeHash
        CurrentDeltaSha256 = (Get-Hash $Entries['current-delta.patch']).ToUpperInvariant()
        CorrectionPatchSha256 = (Get-Hash $Entries['correction-only.patch']).ToUpperInvariant()
    }
}

function Assert-OutcomeReviewsFocusedPackage {
    param(
        [Parameter(Mandatory)]$Outcome,
        [Parameter(Mandatory)]$ReviewedNode,
        [Parameter(Mandatory)][string]$TransitiveFullReviewSha256,
        [Parameter(Mandatory)][string]$Label
    )

    $state = Get-IndependentOutcomeState -Outcome $Outcome -Label $Label
    $reviewedHashProperty = if ($state.IsGeneralized) {
        [string]$Outcome.reviewedPackageSha256
    }
    else { [string]$Outcome.previousReviewPackageSha256 }
    Add-Result -Id "$Label-REVIEWED-PACKAGE" -Passed (
        $reviewedHashProperty -ceq [string]$ReviewedNode.PackageSha256
    )
    Add-Result -Id "$Label-REVIEWED-DELTA" -Passed (
        [string]$Outcome.reviewedCurrentDeltaSha256 -ceq [string]$ReviewedNode.Contract.CurrentDeltaSha256
    )
    Assert-EqualSet $state.TargetFindingIds @($ReviewedNode.Contract.Focused.reviewedFindingIds) `
        "$Label-PRODUCER-TARGETS"

    if ($state.IsGeneralized) {
        Add-Result -Id "$Label-REVIEWED-CORRECTION" -Passed (
            [string]$Outcome.reviewedCorrectionPatchSha256 -ceq
                [string]$ReviewedNode.Contract.CorrectionPatchSha256
        )
        Add-Result -Id "$Label-IMMEDIATE-PREVIOUS" -Passed (
            [string]$Outcome.immediatePreviousReviewPackageSha256 -ceq
                ([string]$ReviewedNode.Contract.Focused.previousReviewSha256).ToUpperInvariant()
        )
        $expectedPriorOutcomeHash = [string]$ReviewedNode.Contract.EmbeddedOutcomeHash
        $hasPriorOutcomeBinding = $null -ne $Outcome.PSObject.Properties['previousIndependentReviewOutcomeSha256']
        Add-Result -Id "$Label-PREVIOUS-OUTCOME-PRESENCE" -Passed (
            $hasPriorOutcomeBinding -eq (-not [string]::IsNullOrWhiteSpace($expectedPriorOutcomeHash))
        )
        if ($hasPriorOutcomeBinding) {
            Add-Result -Id "$Label-PREVIOUS-OUTCOME-HASH" -Passed (
                [string]$Outcome.previousIndependentReviewOutcomeSha256 -ceq $expectedPriorOutcomeHash
            )
            $priorState = Get-IndependentOutcomeState `
                -Outcome $ReviewedNode.Contract.EmbeddedOutcome -Label "$Label-PREVIOUS-STATE"
            Assert-EqualSet $state.PreviousFindingIds $priorState.AuthoritativeFindingIds `
                "$Label-PREVIOUS-STATE-PARITY"
        }
    }
    Add-Result -Id "$Label-TRANSITIVE-FULL" -Passed (
        [string]$Outcome.transitiveFullReviewBaselineSha256 -ceq $TransitiveFullReviewSha256
    )
    return $state
}

function Resolve-ReviewPackageChain {
    param(
        [Parameter(Mandatory)][string]$InitialPackagePath,
        [Parameter(Mandatory)][string]$ExpectedInitialSha256,
        [Parameter(Mandatory)][string]$ExpectedTaskId
    )

    $nodes = [System.Collections.Generic.List[object]]::new()
    $seen = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::OrdinalIgnoreCase)
    $path = [System.IO.Path]::GetFullPath($InitialPackagePath)
    $expectedHash = $ExpectedInitialSha256.ToUpperInvariant()
    $transitiveFullHash = $null
    for ($depth = 0; $depth -lt 16; $depth++) {
        $label = "CORRECTION-RECURSIVE-CHAIN-$depth"
        Add-Result -Id "$label-NO-CYCLE" -Passed $seen.Add($path) -Evidence $path
        $actualHash = (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToUpperInvariant()
        Add-Result -Id "$label-PACKAGE-HASH" -Passed ($actualHash -ceq $expectedHash)
        $entries = Read-ReviewZipEntries -LiteralPath $path -Label $label
        $discriminatorNames = @('assignment-record.json', 'completion-report.json', 'package-inventory.json')
        $discriminatorCount = @(
            $discriminatorNames | Where-Object { $entries.ContainsKey($_) }
        ).Count
        if ($discriminatorCount -eq 0) {
            Assert-EqualSet @($entries.Keys) `
                @('MANIFEST.sha256', 'current-delta.patch', 'scope-inventory.json') `
                "$label-LEGACY-FULL-PACKAGE-SHAPE"
            $legacyScope = Read-JsonBytes $entries['scope-inventory.json'] `
                "$label legacy full scope" '' 1
            Add-Result -Id "$label-LEGACY-FULL-SCOPE" -Passed (
                $null -ne $legacyScope.PSObject.Properties['entries'] -and
                $null -ne $legacyScope.PSObject.Properties['pathCount'] -and
                [int]$legacyScope.pathCount -eq @($legacyScope.entries).Count
            )
            $legacyManifestText = [System.Text.UTF8Encoding]::new($false, $true).GetString(
                $entries['MANIFEST.sha256']
            )
            $legacyManifestNames = @($legacyManifestText.TrimEnd("`r", "`n") -split "`n" | ForEach-Object {
                    $match = [regex]::Match(
                        $_,
                        '^(?<hash>[0-9a-f]{64})  (?<length>[0-9]+)  (?<path>[A-Za-z0-9][A-Za-z0-9._-]*)$'
                    )
                    if (-not $match.Success) { throw "[$label] Invalid legacy manifest line: $_" }
                    $name = $match.Groups['path'].Value
                    if (-not $entries.ContainsKey($name) -or
                        $match.Groups['hash'].Value -cne (Get-Hash $entries[$name]) -or
                        [int64]$match.Groups['length'].Value -ne $entries[$name].LongLength) {
                        throw "[$label] Legacy manifest mismatch: $name"
                    }
                    $name
                })
            Assert-EqualSet $legacyManifestNames `
                @($entries.Keys | Where-Object { $_ -cne 'MANIFEST.sha256' }) `
                "$label-LEGACY-FULL-MANIFEST-COVERAGE"
            $transitiveFullHash = $actualHash
            break
        }
        Add-Result -Id "$label-DISCRIMINATORS" -Passed (
            $discriminatorCount -eq $discriminatorNames.Count
        )
        $assignmentDiscriminatorText = [System.Text.UTF8Encoding]::new($false, $true).GetString(
            $entries['assignment-record.json']
        )
        if ($assignmentDiscriminatorText.Contains([char]0xfffd) -or
            $assignmentDiscriminatorText.IndexOf([char]0) -ge 0) {
            throw "[$label] Invalid strict UTF-8 assignment discriminator."
        }
        $assignmentDiscriminator = $assignmentDiscriminatorText | ConvertFrom-Json -Depth 100
        if ([string]$assignmentDiscriminator.profile -ceq 'FINDING_CORRECTION') {
            $contract = Get-FocusedPackageContract -Entries $entries -Label $label `
                -ExpectedTaskId $ExpectedTaskId
            $node = [pscustomobject][ordered]@{
                PackagePath = $path
                PackageSha256 = $actualHash
                Profile = 'FINDING_CORRECTION'
                Entries = $entries
                Contract = $contract
            }
            [void]$nodes.Add($node)
            $path = [System.IO.Path]::GetFullPath([string]$contract.Focused.previousReviewPackage)
            $expectedHash = ([string]$contract.Focused.previousReviewSha256).ToUpperInvariant()
            continue
        }

        $expectedProfile = 'IMPLEMENTATION_TO_INDEPENDENT_FULL_REVIEW'
        Add-Result -Id "$label-FULL-PROFILE" -Passed (
            [string]$assignmentDiscriminator.profile -ceq $expectedProfile
        )
        $requiredNames = @(
            'HANDOFF.md', 'assignment-record.json', 'completion-report.json',
            'current-delta.patch', 'MANIFEST.sha256', 'package-inventory.json',
            'pre-review-validation-evidence.json', 'report.md', 'scope-inventory.json',
            'task.patch', 'validation-summary.json'
        )
        Assert-EqualSet @($entries.Keys) $requiredNames "$label-FULL-PACKAGE-SHAPE"
        $inventory = Read-JsonBytes $entries['package-inventory.json'] "$label full inventory" `
            'generic-package-inventory.schema.json' 1
        Assert-ManifestInventoryCoverage -Entries $entries -Inventory $inventory -Label $label
        $transitiveFullHash = $actualHash
        break
    }
    Add-Result -Id 'CORRECTION-RECURSIVE-CHAIN-TERMINATED' -Passed (
        -not [string]::IsNullOrWhiteSpace($transitiveFullHash)
    )
    for ($index = 0; $index -lt $nodes.Count; $index++) {
        $node = $nodes[$index]
        if ($null -ne $node.Contract.EmbeddedOutcome) {
            Add-Result -Id "CORRECTION-RECURSIVE-CHAIN-$index-HAS-NEXT" -Passed (
                $index + 1 -lt $nodes.Count
            )
            [void](Assert-OutcomeReviewsFocusedPackage -Outcome $node.Contract.EmbeddedOutcome `
                    -ReviewedNode $nodes[$index + 1] -TransitiveFullReviewSha256 $transitiveFullHash `
                    -Label "CORRECTION-RECURSIVE-CHAIN-$index-OUTCOME")
        }
    }
    return [pscustomobject][ordered]@{
        Nodes = @($nodes)
        TransitiveFullReviewSha256 = $transitiveFullHash
    }
}

function Get-PublicationBindingSignature {
    param(
        [Parameter(Mandatory)][string]$FindingId,
        [Parameter(Mandatory)][object]$Binding,
        [Parameter(Mandatory)][string]$Label
    )
    $caseIds = @(Get-CanonicalSet -Value @($Binding.caseIds) -Label "$Label-cases")
    return ('{0}|{1}|{2}|{3}|{4}' -f $FindingId, [string]$Binding.artifact,
        [string]$Binding.sha256, [string]$Binding.matrixId, ($caseIds -join ','))
}

function Get-FindingPublicationBindingSignatures {
    param([Parameter(Mandatory)][object[]]$Findings, [Parameter(Mandatory)][string]$Label)
    return @(foreach ($finding in $Findings) {
            if ($finding.PSObject.Properties['publicationRegressionEvidence']) {
                Get-PublicationBindingSignature -FindingId ([string]$finding.id) `
                    -Binding $finding.publicationRegressionEvidence -Label "$Label-$([string]$finding.id)"
            }
        })
}

function Get-TopLevelPublicationBindingSignatures {
    param([Parameter(Mandatory)][object]$Contract, [Parameter(Mandatory)][string]$Label)
    if (-not $Contract.PSObject.Properties['publicationRegressionEvidence']) { return @() }
    return @(foreach ($binding in @($Contract.publicationRegressionEvidence)) {
            Get-PublicationBindingSignature -FindingId ([string]$binding.findingId) `
                -Binding $binding -Label "$Label-$([string]$binding.findingId)"
        })
}

function Get-ExpandedScopePaths {
    param([Parameter(Mandatory)][object[]]$Entries)
    return @(foreach ($entry in $Entries) {
            if ([string]$entry.gitStatus -ceq 'TRACKED_RENAMED') {
                [string]$entry.previousPath
            }
            [string]$entry.path
        })
}

function Test-FindingIdTaskBinding {
    param([Parameter(Mandatory)][string]$TaskId, [Parameter(Mandatory)][string]$FindingId)
    $reviewIndex = $FindingId.LastIndexOf('-REV-', [System.StringComparison]::Ordinal)
    if ($reviewIndex -le 0) { return $false }
    $prefix = $FindingId.Substring(0, $reviewIndex)
    $components = @([regex]::Matches($prefix, '(?:^|-)(?<task>BL-(?:00[1-9]|0[1-9][0-9]|[1-9][0-9]{2,}))(?=-|$)') | ForEach-Object {
            $_.Groups['task'].Value
        })
    return $TaskId -cin $components
}

function Assert-FindingSetForTask {
    param([Parameter(Mandatory)][string]$TaskId, [object[]]$FindingIds, [string]$Label)
    $canonical = @(Get-CanonicalSet -Value $FindingIds -Label $Label)
    foreach ($findingId in $canonical) {
        Add-Result -Id ("$Label-TASK-" + $findingId) `
            -Passed (Test-FindingIdTaskBinding -TaskId $TaskId -FindingId $findingId) `
            -Evidence "task=$TaskId;finding=$findingId"
    }
    return $canonical
}

function Invoke-IsolatedGitText {
    param([string[]]$Argument)
    $result = Invoke-GenericGitBytes -Root $AuthoritativeRepositoryRoot -Argument $Argument `
        -Environment $context.Environment
    return (ConvertFrom-GenericStrictUtf8 -Bytes $result.Bytes -Label 'isolated Git output').Trim()
}

function Get-TreePostimage {
    param([string]$Tree, [string]$Path)
    Assert-GenericRepositoryPath -Path $Path | Out-Null
    $result = Invoke-GenericGitBytes -Root $AuthoritativeRepositoryRoot `
        -Argument @('ls-tree', '-z', $Tree, '--', $Path) `
        -Environment $context.Environment -RepositoryPaths
    if ($result.Bytes.Length -eq 0) {
        return [pscustomobject][ordered]@{ path = $Path; presence = 'ABSENT' }
    }
    $text = ConvertFrom-GenericStrictUtf8 -Bytes $result.Bytes[0..($result.Bytes.Length - 2)] -Label "tree entry $Path"
    $match = [regex]::Match($text, '^(?<mode>[0-7]{6}) blob (?<oid>[0-9a-f]{40})\t(?<path>.+)$')
    if (-not $match.Success -or $match.Groups['path'].Value -cne $Path) {
        throw "[TREE-POSTIMAGE] Missing or non-blob tree entry: $Path"
    }
    $blob = Invoke-GenericGitBytes -Root $AuthoritativeRepositoryRoot `
        -Argument @('cat-file', 'blob', $match.Groups['oid'].Value) `
        -Environment $context.Environment
    return [pscustomobject][ordered]@{
        path = $Path
        presence = 'PRESENT'
        mode = $match.Groups['mode'].Value
        length = [int64]$blob.Bytes.Length
        sha256 = Get-Hash -Bytes $blob.Bytes
    }
}

function Convert-LegacyPreviousState {
    param([Parameter(Mandatory)][object]$Previous)
    return [pscustomobject][ordered]@{
        type = 'IMMUTABLE_REVIEW_PACKAGE'
        historicalPackagePath = [string]$Previous.historicalPackagePath
        historicalPackageSha256 = [string]$Previous.historicalPackageSha256
        historicalManifestSha256 = [string]$Previous.historicalManifestSha256
        historicalPatchSha256 = [string]$Previous.historicalPatchSha256
        historicalScopeInventorySha256 = [string]$Previous.historicalScopeInventorySha256
        previousReviewedBaselineCommit = [string]$Previous.previousReviewedBaselineCommit
        previousReviewedTree = [string]$Previous.previousReviewedTree
        previousReviewedPathCount = [int]$Previous.previousReviewedPathCount
        previousReviewedPostimages = @($Previous.previousReviewedPostimages)
    }
}

function Assert-CorrectionEntryBinding {
    param([Parameter(Mandatory)][object]$Entry, [string]$PreviousTree, [string]$CurrentTree)
    $preimagePath = if ([string]$Entry.gitStatus -ceq 'TRACKED_RENAMED') {
        [string]$Entry.previousPath
    }
    else { [string]$Entry.path }
    $preimage = Get-TreePostimage -Tree $PreviousTree -Path $preimagePath
    $postimage = Get-TreePostimage -Tree $CurrentTree -Path ([string]$Entry.path)
    $expectedPreHash = if ($preimage.presence -ceq 'PRESENT') { [string]$preimage.sha256 } else { $null }
    $expectedPostHash = if ($postimage.presence -ceq 'PRESENT') { [string]$postimage.sha256 } else { $null }
    $expectedPreLength = if ($preimage.presence -ceq 'PRESENT') { [int64]$preimage.length } else { $null }
    $expectedPostLength = if ($postimage.presence -ceq 'PRESENT') { [int64]$postimage.length } else { $null }
    $expectedPreMode = if ($preimage.presence -ceq 'PRESENT') { [string]$preimage.mode } else { $null }
    $expectedPostMode = if ($postimage.presence -ceq 'PRESENT') { [string]$postimage.mode } else { $null }
    $bindingPassed = (
        $Entry.previousReviewedSha256 -eq $expectedPreHash -and
        $Entry.currentCorrectedSha256 -eq $expectedPostHash -and
        $Entry.previousReviewedLength -eq $expectedPreLength -and
        $Entry.currentCorrectedLength -eq $expectedPostLength -and
        $Entry.previousReviewedMode -eq $expectedPreMode -and
        $Entry.currentCorrectedMode -eq $expectedPostMode
    )
    Add-Result -Id ('CORRECTION-BEFORE-AFTER-' + [string]$Entry.path) -Passed $bindingPassed -Evidence (
        "preHash={0}/{1};postHash={2}/{3};preLength={4}/{5};postLength={6}/{7};preMode={8}/{9};postMode={10}/{11}" -f
        $Entry.previousReviewedSha256,$expectedPreHash,$Entry.currentCorrectedSha256,$expectedPostHash,
        $Entry.previousReviewedLength,$expectedPreLength,$Entry.currentCorrectedLength,$expectedPostLength,
        $Entry.previousReviewedMode,$expectedPreMode,$Entry.currentCorrectedMode,$expectedPostMode
    )
}

function Get-SharedIsolationPatchDeltaEvidence {
    param(
        [Parameter(Mandatory)][string]$BaselineTree,
        [Parameter(Mandatory)][string]$PatchPath,
        [Parameter(Mandatory)][object[]]$IncludedEntry
    )
    Invoke-IsolatedGitText @('read-tree', $BaselineTree) | Out-Null
    Invoke-IsolatedGitText @('apply', '--cached', '--binary', '--whitespace=nowarn', '--', $PatchPath) | Out-Null
    $nameStatus = Invoke-GenericGitBytes -Root $AuthoritativeRepositoryRoot `
        -Argument @('diff', '--cached', '--name-status', '-z', '--find-renames', $BaselineTree, '--') `
        -Environment $context.Environment
    $actualEntries = @(ConvertFrom-GenericNameStatusZ -Bytes $nameStatus.Bytes)
    $expectedKeys = @(Get-GenericExpectedDeltaInventoryKeys -IncludedEntry $IncludedEntry)
    $actualKeys = @($actualEntries | ForEach-Object key | Sort-Object)
    return [pscustomobject][ordered]@{
        ActualDeltaInventory = $actualEntries
        ExpectedDeltaInventoryKeys = $expectedKeys
        ActualDeltaInventoryParity = (($actualKeys -join "`n") -ceq ($expectedKeys -join "`n"))
        ExcludedDeltaPathProhibition = $true
        RealObjectDatabaseImmutable = $true
        TemporaryArtifactsRemoved = $true
    }
}

try {
    $resolvedPackagePath = [System.IO.Path]::GetFullPath($PackagePath)
    $RepositoryRoot = [System.IO.Path]::GetFullPath($RepositoryRoot).TrimEnd('\', '/')
    $AuthoritativeRepositoryRoot = [System.IO.Path]::GetFullPath($AuthoritativeRepositoryRoot).TrimEnd('\', '/')
    $isDirectory = Test-Path -LiteralPath $resolvedPackagePath -PathType Container
    $isZip = Test-Path -LiteralPath $resolvedPackagePath -PathType Leaf
    Add-Result -Id 'CORRECTION-ARTIFACT-KIND' -Passed ($isDirectory -or $isZip) -Evidence $resolvedPackagePath
    Add-Result -Id 'CORRECTION-PACKAGE-OUTSIDE-REPOSITORY' -Passed (-not $resolvedPackagePath.StartsWith(
            $RepositoryRoot + [System.IO.Path]::DirectorySeparatorChar,
            [System.StringComparison]::OrdinalIgnoreCase
        ))

    $entryBytes = [System.Collections.Generic.Dictionary[string, byte[]]]::new([System.StringComparer]::Ordinal)
    $entryNamesIgnoreCase = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::OrdinalIgnoreCase)
    if ($isDirectory) {
        $rootItem = Get-Item -LiteralPath $resolvedPackagePath -Force
        Add-Result -Id 'CORRECTION-DIRECTORY-NO-REPARSE' -Passed (($rootItem.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -eq 0)
        Add-Result -Id 'CORRECTION-DIRECTORY-FLAT' -Passed (@(Get-ChildItem -LiteralPath $resolvedPackagePath -Directory -Force).Count -eq 0)
        foreach ($file in @(Get-ChildItem -LiteralPath $resolvedPackagePath -File -Force)) {
            if (($file.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) { throw "Reparse member: $($file.Name)" }
            if (-not $entryNamesIgnoreCase.Add($file.Name)) { throw "Duplicate or case-colliding member: $($file.Name)" }
            $entryBytes.Add($file.Name, [System.IO.File]::ReadAllBytes($file.FullName))
        }
    }
    else {
        Add-Type -AssemblyName System.IO.Compression
        $archiveStream = [System.IO.File]::OpenRead($resolvedPackagePath)
        $archive = [System.IO.Compression.ZipArchive]::new($archiveStream, [System.IO.Compression.ZipArchiveMode]::Read, $false)
        foreach ($entry in $archive.Entries) {
            if ($entry.FullName -notmatch '^[A-Za-z0-9][A-Za-z0-9._-]*$') { throw "Unsafe ZIP member: $($entry.FullName)" }
            if (-not $entryNamesIgnoreCase.Add($entry.FullName)) { throw "Duplicate or case-colliding ZIP member: $($entry.FullName)" }
            $unixType = ([uint32]$entry.ExternalAttributes -shr 16) -band 0xf000
            if ($unixType -eq 0xa000 -or ([uint32]$entry.ExternalAttributes -band 0x400) -ne 0) {
                throw "Link or reparse ZIP semantics: $($entry.FullName)"
            }
            $memory = [System.IO.MemoryStream]::new()
            try {
                $stream = $entry.Open()
                try { $stream.CopyTo($memory) } finally { $stream.Dispose() }
                $entryBytes.Add($entry.FullName, $memory.ToArray())
            }
            finally { $memory.Dispose() }
        }
    }

    $requiredNames = @(
        'HANDOFF.md', 'assignment-record.json', 'completion-report.json',
        'correction-only.patch', 'correction-scope-inventory.json', 'current-delta.patch',
        'external-governance-manifest.json', 'finding-correction-matrix.json',
        'finding-ledger.json', 'finding-regression-matrix.json',
        'focused-delta-review-record.json', 'focused-validation-result.json',
        'MANIFEST.sha256', 'package-inventory.json',
        'previous-review-binding.json', 'readiness-evidence.json', 'report.md',
        'scope-inventory.json', 'trusted-expected-hashes.json', 'validation-summary.json'
    )
    $hasIndependentReviewOutcomeMember = $entryBytes.ContainsKey('previous-independent-review-outcome.json')
    if ($hasIndependentReviewOutcomeMember) {
        $requiredNames += 'previous-independent-review-outcome.json'
    }
    $hasPublicationEvidence = $entryBytes.ContainsKey('publication-regression-evidence.json')
    $hasPublicationResult = $entryBytes.ContainsKey('publication-regression-result.json')
    if ($hasPublicationEvidence -or $hasPublicationResult) {
        $requiredNames += @('publication-regression-evidence.json', 'publication-regression-result.json')
    }
    Assert-EqualSet -Left @($entryBytes.Keys) -Right $requiredNames -Label 'CORRECTION-PACKAGE-SHAPE'
    foreach ($name in $requiredNames) {
        Add-Result -Id ('CORRECTION-NONEMPTY-' + $name) -Passed ($entryBytes[$name].Length -gt 0)
    }

    $assignment = Read-JsonBytes $entryBytes['assignment-record.json'] 'assignment-record.json' 'finding-correction-assignment.schema.json' 2
    $completion = Read-JsonBytes $entryBytes['completion-report.json'] 'completion-report.json' 'finding-correction-completion.schema.json' 2
    $currentScope = Read-JsonBytes $entryBytes['scope-inventory.json'] 'scope-inventory.json' '' 1
    $correctionScope = Read-JsonBytes $entryBytes['correction-scope-inventory.json'] 'correction-scope-inventory.json' '' 1
    $correctionMatrix = Read-JsonBytes $entryBytes['finding-correction-matrix.json'] 'finding-correction-matrix.json' 'finding-correction-matrix.schema.json' 2
    $regressionMatrix = Read-JsonBytes $entryBytes['finding-regression-matrix.json'] 'finding-regression-matrix.json' 'finding-regression-matrix.schema.json' 2
    $focusedValidationResult = Read-JsonBytes $entryBytes['focused-validation-result.json'] `
        'focused-validation-result.json' '' 2
    $ledger = Read-JsonBytes $entryBytes['finding-ledger.json'] 'finding-ledger.json' 'finding-ledger.schema.json' 1
    $focused = Read-JsonBytes $entryBytes['focused-delta-review-record.json'] 'focused-delta-review-record.json' 'focused-delta-review-record.schema.json' 3
    $previous = Read-JsonBytes $entryBytes['previous-review-binding.json'] 'previous-review-binding.json' 'previous-review-binding.schema.json' 3
    $inventory = Read-JsonBytes $entryBytes['package-inventory.json'] 'package-inventory.json' '' 1
    $trusted = Read-JsonBytes $entryBytes['trusted-expected-hashes.json'] 'trusted-expected-hashes.json' '' 1
    $readiness = Read-JsonBytes $entryBytes['readiness-evidence.json'] 'readiness-evidence.json' '' 2
    $validationSummary = Read-JsonBytes $entryBytes['validation-summary.json'] 'validation-summary.json' '' 1
    $externalManifest = Read-JsonBytes $entryBytes['external-governance-manifest.json'] 'external-governance-manifest.json' '' 1
    $publicationEvidence = if ($hasPublicationEvidence) {
        Read-JsonBytes $entryBytes['publication-regression-evidence.json'] `
            'publication-regression-evidence.json' 'publication-regression-evidence.schema.json' 2
    }
    else { $null }
    $publicationResult = if ($hasPublicationResult) {
        Read-JsonBytes $entryBytes['publication-regression-result.json'] `
            'publication-regression-result.json' 'publication-regression-result.schema.json' 2
    }
    else { $null }
    Add-Result -Id 'CORRECTION-CANONICAL-JSON-CONTRACTS' -Passed ($null -ne $externalManifest)
    $reportContract = Read-EmbeddedJsonContract -Bytes $entryBytes['report.md'] -Name 'report.md' `
        -BeginMarker '<!-- BEGIN FINDING-CORRECTION-REPORT-CONTRACT -->' `
        -EndMarker '<!-- END FINDING-CORRECTION-REPORT-CONTRACT -->' `
        -SchemaName 'finding-correction-report-contract.schema.json' -ExpectedSchemaVersion 1
    $handoffContract = Read-EmbeddedJsonContract -Bytes $entryBytes['HANDOFF.md'] -Name 'HANDOFF.md' `
        -BeginMarker '<!-- BEGIN GOVERNANCE-HANDOFF-CONTRACT -->' `
        -EndMarker '<!-- END GOVERNANCE-HANDOFF-CONTRACT -->' -ExpectedSchemaVersion 2

    Add-Result -Id 'CORRECTION-EXPLICIT-DISCRIMINATORS' -Passed (
        [string]$assignment.profile -ceq 'FINDING_CORRECTION' -and
        [string]$assignment.transitionType -ceq 'BUNDLED_CORRECTION_TO_FOCUSED_DELTA_REVIEW' -and
        [string]$completion.taskId -ceq [string]$assignment.taskId -and
        [string]$reportContract.taskId -ceq [string]$assignment.taskId -and
        [string]$reportContract.profile -ceq [string]$assignment.profile -and
        [string]$reportContract.transitionType -ceq [string]$assignment.transitionType -and
        [string]$handoffContract.taskId -ceq [string]$assignment.taskId -and
        [string]$handoffContract.profile -ceq [string]$assignment.profile -and
        [string]$handoffContract.transitionType -ceq [string]$assignment.transitionType
    )
    $lifecycleState = if ([int]$completion.schemaVersion -eq 1) {
        'ZIP_FREE_READY_TO_EXECUTE'
    }
    else { [string]$completion.artifactLifecycleState }
    $assignmentLifecycle = if ([int]$assignment.schemaVersion -eq 1) { 'ZIP_FREE_READY_TO_EXECUTE' } else { [string]$assignment.artifactLifecycleState }
    $readinessLifecycle = if ([int]$readiness.schemaVersion -eq 1) { 'ZIP_FREE_READY_TO_EXECUTE' } else { [string]$readiness.artifactLifecycleState }
    Add-Result -Id 'CORRECTION-LIFECYCLE-PARITY' -Passed (
        $lifecycleState -ceq $assignmentLifecycle -and $lifecycleState -ceq $readinessLifecycle -and
        $lifecycleState -ceq [string]$reportContract.artifactLifecycleState -and
        $lifecycleState -ceq [string]$handoffContract.artifactLifecycleState
    )
    $handoffText = [System.Text.UTF8Encoding]::new($false, $true).GetString($entryBytes['HANDOFF.md'])
    $reportText = [System.Text.UTF8Encoding]::new($false, $true).GetString($entryBytes['report.md'])
    if ($lifecycleState -ceq 'ZIP_FREE_READY_TO_EXECUTE') {
        Add-Result -Id 'CORRECTION-PREFLIGHT-SEMANTICS' -Passed (
            [bool]$completion.readyToExecute -and -not [bool]$completion.classicReviewReady -and
            [int]$completion.packageWriteAttemptCount -eq 0 -and [bool]$readiness.readyToExecute -and
            -not [bool]$readiness.classicReviewReady -and [int]$readiness.packageWriteAttemptCount -eq 0 -and
            [string]$completion.nextAction -ceq 'READY_FOR_SINGLE_DELTA_REVIEW_PACKAGE_WRITE_AUTHORIZATION' -and
            [string]$assignment.nextAction -ceq 'READY_FOR_SINGLE_DELTA_REVIEW_PACKAGE_WRITE_AUTHORIZATION' -and
            [bool]$reportContract.readyToExecute -and -not [bool]$reportContract.classicReviewReady -and
            [int]$reportContract.packageWriteAttemptCount -eq 0 -and
            [string]$reportContract.nextAction -ceq 'READY_FOR_SINGLE_DELTA_REVIEW_PACKAGE_WRITE_AUTHORIZATION' -and
            [bool]$handoffContract.readyToExecute -and -not [bool]$handoffContract.classicReviewReady -and
            [int]$handoffContract.packageWriteAttemptCount -eq 0 -and
            [string]$handoffContract.nextAction -ceq 'READY_FOR_SINGLE_DELTA_REVIEW_PACKAGE_WRITE_AUTHORIZATION' -and
            $handoffText.Contains('Status: ZIP_FREE_READY_TO_EXECUTE') -and
            $reportText.Contains('Status: ZIP_FREE_READY_TO_EXECUTE')
        )
        Add-Result -Id 'CORRECTION-PREFLIGHT-DIRECTORY-ONLY' -Passed $isDirectory
    }
    elseif ($lifecycleState -ceq 'FINAL_REVIEW_PACKAGE') {
        Add-Result -Id 'CORRECTION-FINAL-CONTENT-SEMANTICS' -Passed (
            -not [bool]$completion.readyToExecute -and [bool]$completion.classicReviewReady -and
            [int]$completion.packageWriteAttemptCount -eq 1 -and -not [bool]$readiness.readyToExecute -and
            [bool]$readiness.classicReviewReady -and [int]$readiness.packageWriteAttemptCount -eq 1 -and
            [string]$completion.nextAction -ceq 'FOCUSED_INDEPENDENT_DELTA_REVIEW' -and
            [string]$assignment.nextAction -ceq 'FOCUSED_INDEPENDENT_DELTA_REVIEW' -and
            -not [bool]$reportContract.readyToExecute -and [bool]$reportContract.classicReviewReady -and
            [int]$reportContract.packageWriteAttemptCount -eq 1 -and
            [string]$reportContract.nextAction -ceq 'FOCUSED_INDEPENDENT_DELTA_REVIEW' -and
            -not [bool]$handoffContract.readyToExecute -and [bool]$handoffContract.classicReviewReady -and
            [int]$handoffContract.packageWriteAttemptCount -eq 1 -and
            [string]$handoffContract.nextAction -ceq 'FOCUSED_INDEPENDENT_DELTA_REVIEW' -and
            $handoffText.Contains('ArtifactLifecycleState: FINAL_REVIEW_PACKAGE') -and
            $handoffText.Contains('ClassicReviewReady: true') -and
            $reportText.Contains('ArtifactLifecycleState: FINAL_REVIEW_PACKAGE') -and
            $reportText.Contains('PackageWriteAttemptCount: 1')
        )
    }
    else { throw "Unsupported artifact lifecycle state: $lifecycleState" }

    $findingSets = @(
        @($assignment.findingIds), @($completion.findingIds),
        @($correctionMatrix.findings | ForEach-Object id),
        @($regressionMatrix.findings | ForEach-Object id),
        @($focused.reviewedFindingIds), @($ledger.findings | ForEach-Object id),
        @($reportContract.findingIds)
    )
    for ($index = 0; $index -lt $findingSets.Count; $index++) {
        [void](Assert-FindingSetForTask -TaskId ([string]$assignment.taskId) -FindingIds $findingSets[$index] -Label "CORRECTION-FINDING-SET-$index")
        if ($index -gt 0) {
            Assert-EqualSet -Left $findingSets[0] -Right $findingSets[$index] -Label "CORRECTION-FINDING-PARITY-$index"
        }
    }
    Add-Result -Id 'CORRECTION-FINDING-COUNTS' -Passed (
        [int]$correctionMatrix.correctedFindingCount -eq @($findingSets[0]).Count -and
        [int]$ledger.findingCount -eq @($findingSets[0]).Count -and
        @($reportContract.findingDispositions).Count -eq @($findingSets[0]).Count
    )
    foreach ($findingId in @($findingSets[0])) {
        $correctionFindings = @($correctionMatrix.findings | Where-Object id -CEQ $findingId)
        $regressionFindings = @($regressionMatrix.findings | Where-Object id -CEQ $findingId)
        $ledgerFindings = @($ledger.findings | Where-Object id -CEQ $findingId)
        $reportFindings = @($reportContract.findingDispositions | Where-Object id -CEQ $findingId)
        Add-Result -Id ('CORRECTION-PER-FINDING-UNIQUE-' + $findingId) -Passed (
            $correctionFindings.Count -eq 1 -and $regressionFindings.Count -eq 1 -and
            $ledgerFindings.Count -eq 1 -and $reportFindings.Count -eq 1
        )
        $correctionFinding = $correctionFindings[0]
        $regressionFinding = $regressionFindings[0]
        $ledgerFinding = $ledgerFindings[0]
        $reportFinding = $reportFindings[0]

        foreach ($field in @('severity', 'previousStatus', 'status', 'disposition', 'correction')) {
            $expectedValue = [string]$correctionFinding.$field
            Add-Result -Id ("CORRECTION-PER-FINDING-{0}-{1}" -f $field.ToUpperInvariant(), $findingId) -Passed (
                [string]$regressionFinding.$field -ceq $expectedValue -and
                [string]$ledgerFinding.$field -ceq $expectedValue -and
                [string]$reportFinding.$field -ceq $expectedValue
            ) -Evidence ("correction={0};regression={1};ledger={2};report={3}" -f
                $expectedValue, [string]$regressionFinding.$field,
                [string]$ledgerFinding.$field, [string]$reportFinding.$field)
        }
        Add-Result -Id ('CORRECTION-PER-FINDING-STATUS-MAPPING-' + $findingId) -Passed (
            [string]$correctionFinding.status -ceq 'CORRECTED_PENDING_DELTA' -and
            [string]$ledgerFinding.producerStatus -ceq 'CORRECTED_PENDING_DELTA_REVIEW' -and
            [string]$reportFinding.producerStatus -ceq [string]$ledgerFinding.producerStatus -and
            [string]$ledgerFinding.reviewerStatus -ceq 'OPEN_PENDING_FOCUSED_INDEPENDENT_DELTA_REVIEW' -and
            [string]$reportFinding.reviewerStatus -ceq [string]$ledgerFinding.reviewerStatus
        )
        Assert-EqualSet @($correctionFinding.affectedPaths) @($regressionFinding.affectedPaths) `
            ('CORRECTION-PER-FINDING-AFFECTED-REGRESSION-' + $findingId)
        Assert-EqualSet @($correctionFinding.affectedPaths) @($ledgerFinding.correctionPaths) `
            ('CORRECTION-PER-FINDING-AFFECTED-LEDGER-' + $findingId)
        Assert-EqualSet @($correctionFinding.affectedPaths) @($reportFinding.affectedPaths) `
            ('CORRECTION-PER-FINDING-AFFECTED-REPORT-' + $findingId)

        $regressionFindingTestIds = @($regressionFinding.regressionTests | ForEach-Object id)
        [void](Get-CanonicalSet $regressionFindingTestIds ('CORRECTION-PER-FINDING-REGRESSION-TEST-IDS-' + $findingId))
        Assert-EqualSet @($correctionFinding.regressionTestIds) $regressionFindingTestIds `
            ('CORRECTION-PER-FINDING-TESTS-REGRESSION-' + $findingId)
        Assert-EqualSet @($correctionFinding.regressionTestIds) @($ledgerFinding.permanentRegressions) `
            ('CORRECTION-PER-FINDING-TESTS-LEDGER-' + $findingId)
        Assert-EqualSet @($correctionFinding.regressionTestIds) @($reportFinding.regressionTestIds) `
            ('CORRECTION-PER-FINDING-TESTS-REPORT-' + $findingId)

        Assert-EqualSet @($correctionFinding.evidenceReferences) @($regressionFinding.evidenceReferences) `
            ('CORRECTION-PER-FINDING-EVIDENCE-REGRESSION-' + $findingId)
        Assert-EqualSet @($correctionFinding.evidenceReferences) @($ledgerFinding.evidenceReferences) `
            ('CORRECTION-PER-FINDING-EVIDENCE-LEDGER-' + $findingId)
        Assert-EqualSet @($correctionFinding.evidenceReferences) @($reportFinding.evidenceReferences) `
            ('CORRECTION-PER-FINDING-EVIDENCE-REPORT-' + $findingId)
    }

    $ledgerPublicationBindings = @(Get-FindingPublicationBindingSignatures `
            -Findings @($ledger.findings) -Label 'CORRECTION-PUBLICATION-LEDGER')
    $regressionPublicationBindings = @(Get-FindingPublicationBindingSignatures `
            -Findings @($regressionMatrix.findings) -Label 'CORRECTION-PUBLICATION-REGRESSION')
    $focusedPublicationBindings = @(Get-TopLevelPublicationBindingSignatures `
            -Contract $focused -Label 'CORRECTION-PUBLICATION-FOCUSED')
    $reportPublicationBindings = @(Get-TopLevelPublicationBindingSignatures `
            -Contract $reportContract -Label 'CORRECTION-PUBLICATION-REPORT')
    $hasPublicationDeclarations = ($ledgerPublicationBindings.Count + $regressionPublicationBindings.Count +
        $focusedPublicationBindings.Count + $reportPublicationBindings.Count) -gt 0
    Add-Result -Id 'CORRECTION-PUBLICATION-EVIDENCE-PRESENCE' -Passed (
        $hasPublicationEvidence -eq $hasPublicationDeclarations -and
        $hasPublicationResult -eq $hasPublicationDeclarations
    )
    if (-not $hasPublicationEvidence) {
        Add-Result -Id 'CORRECTION-PUBLICATION-EVIDENCE-OPTIONAL-COMPATIBILITY' -Passed (
            -not $hasPublicationDeclarations
        )
    }
    else {
        Assert-EqualSet $ledgerPublicationBindings $regressionPublicationBindings `
            'CORRECTION-PUBLICATION-LEDGER-REGRESSION-PARITY'
        Assert-EqualSet $ledgerPublicationBindings $focusedPublicationBindings `
            'CORRECTION-PUBLICATION-LEDGER-FOCUSED-PARITY'
        Assert-EqualSet $ledgerPublicationBindings $reportPublicationBindings `
            'CORRECTION-PUBLICATION-LEDGER-REPORT-PARITY'

        $matrixDefinitionPath = Join-Path $RepositoryRoot 'Governance/publication-regression-matrix-catalog.json'
        $matrixDefinitionBytes = [System.IO.File]::ReadAllBytes($matrixDefinitionPath)
        $matrixCatalog = Read-JsonBytes $matrixDefinitionBytes `
            'Governance/publication-regression-matrix-catalog.json' `
            'publication-regression-matrix-catalog.schema.json' 1
        $matrixRecords = @($matrixCatalog.matrices | Where-Object matrixId -CEQ ([string]$publicationEvidence.matrixId))
        Add-Result -Id 'CORRECTION-PUBLICATION-MATRIX-DEFINITION-UNIQUE' -Passed ($matrixRecords.Count -eq 1)
        $matrixDefinition = $matrixRecords[0]
        Add-Result -Id 'CORRECTION-PUBLICATION-MATRIX-DEFINITION-BINDING' -Passed (
            [string]$publicationEvidence.matrixDefinitionArtifact -ceq 'Governance/publication-regression-matrix-catalog.json' -and
            [string]$publicationEvidence.matrixDefinitionSha256 -ceq (Get-Hash $matrixDefinitionBytes) -and
            [string]$matrixDefinition.resultSchema -ceq 'Governance/publication-regression-result.schema.json'
        )
        $executionInputBinding = $publicationResult.executionInputBinding
        Add-Result -Id 'CORRECTION-PUBLICATION-RESULT-MATRIX-BINDING' -Passed (
            [string]$executionInputBinding.matrixDefinitionArtifact -ceq
                [string]$publicationEvidence.matrixDefinitionArtifact -and
            [string]$executionInputBinding.matrixDefinitionArtifact -ceq
                'Governance/publication-regression-matrix-catalog.json' -and
            [string]$executionInputBinding.matrixDefinitionSha256 -ceq
                [string]$publicationEvidence.matrixDefinitionSha256 -and
            [string]$executionInputBinding.matrixDefinitionSha256 -ceq (Get-Hash $matrixDefinitionBytes)
        )
        Add-Result -Id 'CORRECTION-PUBLICATION-RESULT-SHA-BINDING' -Passed (
            [string]$publicationEvidence.resultArtifact -ceq 'publication-regression-result.json' -and
            [string]$publicationEvidence.resultSha256 -ceq (Get-Hash $entryBytes['publication-regression-result.json']) -and
            [string]$publicationEvidence.matrixId -ceq [string]$publicationResult.matrixId
        )

        $resultRows = @($publicationResult.results)
        $resultCaseIds = @($resultRows | ForEach-Object id)
        $resultPassed = @($resultRows | Where-Object result -CEQ 'PASS').Count
        $resultFailed = @($resultRows | Where-Object result -CEQ 'FAIL').Count
        [void](Get-CanonicalSet -Value $resultCaseIds -Label 'CORRECTION-PUBLICATION-RESULT-CASE-IDS')
        Add-Result -Id 'CORRECTION-PUBLICATION-RESULT-INVARIANTS' -Passed (
            [string]$publicationResult.status -ceq 'PASS' -and
            [int]$publicationResult.selected -eq $resultRows.Count -and
            [int]$publicationResult.passed -eq $resultPassed -and
            [int]$publicationResult.failed -eq $resultFailed -and
            $resultFailed -eq 0 -and $resultPassed -eq $resultRows.Count
        ) -Evidence ("status={0};selected={1}/{2};passed={3}/{4};failed={5}/{6}" -f
            $publicationResult.status, $publicationResult.selected, $resultRows.Count,
            $publicationResult.passed, $resultPassed, $publicationResult.failed, $resultFailed)
        Assert-EqualSet $resultCaseIds @($matrixDefinition.requiredCaseIds) `
            'CORRECTION-PUBLICATION-CANONICAL-CASE-SET'

        $evidenceAssignments = @($publicationEvidence.findingAssignments)
        $evidenceAssignmentIds = @($evidenceAssignments | ForEach-Object findingId)
        $declaredFindingIds = @($ledger.findings | Where-Object {
                $_.PSObject.Properties['publicationRegressionEvidence']
            } | ForEach-Object id)
        Assert-EqualSet $declaredFindingIds $evidenceAssignmentIds `
            'CORRECTION-PUBLICATION-FINDING-PARITY'
        [void](Assert-FindingSetForTask -TaskId ([string]$assignment.taskId) `
                -FindingIds $evidenceAssignmentIds -Label 'CORRECTION-PUBLICATION-FINDING-TASK')
        Add-Result -Id 'CORRECTION-PUBLICATION-TASK-BINDING' -Passed (
            [string]$publicationEvidence.taskId -ceq [string]$assignment.taskId
        )

        $evidenceHash = Get-Hash $entryBytes['publication-regression-evidence.json']
        $assignedCaseIds = [System.Collections.Generic.List[string]]::new()
        foreach ($assignmentBinding in $evidenceAssignments) {
            $findingId = [string]$assignmentBinding.findingId
            $declaredFinding = @($ledger.findings | Where-Object id -CEQ $findingId)
            Add-Result -Id ('CORRECTION-PUBLICATION-UNIQUE-FINDING-' + $findingId) `
                -Passed ($declaredFinding.Count -eq 1)
            $declaredBinding = $declaredFinding[0].publicationRegressionEvidence
            Assert-EqualSet @($declaredBinding.caseIds) @($assignmentBinding.caseIds) `
                ('CORRECTION-PUBLICATION-CASE-PARITY-' + $findingId)
            Add-Result -Id ('CORRECTION-PUBLICATION-BINDING-' + $findingId) -Passed (
                [string]$declaredBinding.artifact -ceq 'publication-regression-evidence.json' -and
                [string]$declaredBinding.sha256 -ceq $evidenceHash -and
                [string]$declaredBinding.matrixId -ceq [string]$publicationEvidence.matrixId
            )
            foreach ($caseId in @($assignmentBinding.caseIds)) { [void]$assignedCaseIds.Add([string]$caseId) }
        }
        Assert-EqualSet @($assignedCaseIds) $resultCaseIds `
            'CORRECTION-PUBLICATION-EXECUTED-CASE-COVERAGE'

        Assert-EqualSet @($publicationEvidence.sourceBindings | ForEach-Object path) `
            @($matrixDefinition.sourcePaths) 'CORRECTION-PUBLICATION-CANONICAL-SOURCE-SET'
        Assert-EqualSet @($publicationEvidence.dependencyBindings | ForEach-Object path) `
            @($matrixDefinition.dependencyPaths) 'CORRECTION-PUBLICATION-CANONICAL-DEPENDENCY-SET'
        Assert-EqualSet @($executionInputBinding.sourceBindings | ForEach-Object path) `
            @($matrixDefinition.sourcePaths) 'CORRECTION-PUBLICATION-RESULT-CANONICAL-SOURCE-SET'
        Assert-EqualSet @($executionInputBinding.dependencyBindings | ForEach-Object path) `
            @($matrixDefinition.dependencyPaths) 'CORRECTION-PUBLICATION-RESULT-CANONICAL-DEPENDENCY-SET'

        $resultFileBindings = [System.Collections.Generic.Dictionary[string, string]]::new(
            [System.StringComparer]::OrdinalIgnoreCase
        )
        foreach ($resultFileBinding in @($executionInputBinding.sourceBindings) +
            @($executionInputBinding.dependencyBindings)) {
            $resultRelativePath = [string]$resultFileBinding.path
            Add-Result -Id ('CORRECTION-PUBLICATION-RESULT-UNIQUE-FILE-' + $resultRelativePath) `
                -Passed $resultFileBindings.TryAdd($resultRelativePath, [string]$resultFileBinding.sha256)
        }

        $fileBindingPaths = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::OrdinalIgnoreCase)
        foreach ($fileBinding in @($publicationEvidence.sourceBindings) + @($publicationEvidence.dependencyBindings)) {
            $relativePath = [string]$fileBinding.path
            Assert-GenericRepositoryPath -Path $relativePath | Out-Null
            Add-Result -Id ('CORRECTION-PUBLICATION-UNIQUE-FILE-' + $relativePath) `
                -Passed $fileBindingPaths.Add($relativePath)
            $fullPath = [System.IO.Path]::GetFullPath((Join-Path $RepositoryRoot $relativePath))
            $insideRoot = $fullPath.StartsWith(
                $RepositoryRoot + [System.IO.Path]::DirectorySeparatorChar,
                [System.StringComparison]::OrdinalIgnoreCase
            )
            $actualHash = if ($insideRoot -and (Test-Path -LiteralPath $fullPath -PathType Leaf)) {
                (Get-FileHash -LiteralPath $fullPath -Algorithm SHA256).Hash.ToLowerInvariant()
            }
            else { '' }
            Add-Result -Id ('CORRECTION-PUBLICATION-FRESHNESS-' + $relativePath) -Passed (
                $insideRoot -and $resultFileBindings.ContainsKey($relativePath) -and
                [string]$resultFileBindings[$relativePath] -ceq [string]$fileBinding.sha256 -and
                $actualHash -ceq [string]$fileBinding.sha256
            ) -Evidence ("result={0};evidence={1};actual={2}" -f
                [string]$resultFileBindings[$relativePath], [string]$fileBinding.sha256, $actualHash)
        }
    }

    $currentEntries = @($currentScope.entries)
    $correctionEntries = @($correctionScope.entries)
    $currentPaths = @(Get-ExpandedScopePaths -Entries $currentEntries)
    $correctionPaths = @(Get-ExpandedScopePaths -Entries $correctionEntries)
    $referenceOnlyPaths = @(Get-CanonicalSet -Value @($focused.referenceOnlyPaths) -Label 'CORRECTION-REFERENCE-ONLY')
    $directInterfacePaths = @(Get-CanonicalSet -Value @($focused.directInterfacePaths) -Label 'CORRECTION-DIRECT-INTERFACE')
    Assert-EqualSet $currentPaths $assignment.allowedDeltaPaths 'CORRECTION-CURRENT-SCOPE-ASSIGNMENT'
    Assert-EqualSet $currentPaths $focused.allowedDeltaPaths 'CORRECTION-CURRENT-SCOPE-FOCUSED'
    Assert-EqualSet $correctionPaths $assignment.correctionOnlyPaths 'CORRECTION-ONLY-SCOPE-ASSIGNMENT'
    Assert-EqualSet $correctionPaths $focused.correctionOnlyPaths 'CORRECTION-ONLY-SCOPE-FOCUSED'
    Assert-EqualSet $correctionPaths $correctionMatrix.repositoryCorrectionPaths 'CORRECTION-MATRIX-SCOPE'
    Assert-Subset $directInterfacePaths $correctionPaths 'CORRECTION-DIRECT-INTERFACE-COVERAGE'
    Assert-Subset $referenceOnlyPaths $currentPaths 'CORRECTION-REFERENCE-ONLY-CURRENT-SCOPE'
    Assert-EqualSet $correctionPaths $reportContract.scopeSemantics.correctionOnlyPaths 'CORRECTION-REPORT-CORRECTION-SEMANTICS'
    Assert-EqualSet $directInterfacePaths $reportContract.scopeSemantics.directInterfacePaths 'CORRECTION-REPORT-DIRECT-INTERFACE-SEMANTICS'
    Assert-EqualSet $referenceOnlyPaths $reportContract.scopeSemantics.referenceOnlyPaths 'CORRECTION-REPORT-REFERENCE-ONLY-SEMANTICS'
    Add-Result -Id 'CORRECTION-REPORT-SCOPE-PATH-COUNTS' -Passed (
        [int]$currentScope.pathCount -eq $currentEntries.Count -and
        [int]$correctionScope.pathCount -eq $correctionEntries.Count -and
        [int]$reportContract.currentFeaturePatch.pathCount -eq @(Get-CanonicalSet $currentPaths 'CORRECTION-REPORT-CURRENT-PATHS').Count -and
        [int]$reportContract.correctionOnlyPatch.pathCount -eq @(Get-CanonicalSet $correctionPaths 'CORRECTION-REPORT-CORRECTION-PATHS').Count
    )
    Assert-EqualSet $reportContract.permanentRegressionEvidence.testIds $focused.regressionTestIds 'CORRECTION-REPORT-REGRESSION-EVIDENCE'
    $correctionRegressionIds = @($correctionMatrix.findings | ForEach-Object regressionTestIds | Select-Object -Unique)
    $regressionMatrixIds = @($regressionMatrix.findings | ForEach-Object regressionTests | ForEach-Object id | Select-Object -Unique)
    $ledgerRegressionIds = @($ledger.findings | ForEach-Object permanentRegressions | Select-Object -Unique)
    Assert-EqualSet $reportContract.permanentRegressionEvidence.testIds $correctionRegressionIds `
        'CORRECTION-REPORT-CORRECTION-MATRIX-REGRESSION-UNION'
    Assert-EqualSet $reportContract.permanentRegressionEvidence.testIds $regressionMatrixIds `
        'CORRECTION-REPORT-REGRESSION-MATRIX-UNION'
    Assert-EqualSet $reportContract.permanentRegressionEvidence.testIds $ledgerRegressionIds `
        'CORRECTION-REPORT-LEDGER-REGRESSION-UNION'
    $finalFocusedValidationProperty = $regressionMatrix.PSObject.Properties['finalFocusedValidationEvidence']
    if ($null -eq $finalFocusedValidationProperty) {
        throw 'Finding regression evidence does not bind the final focused validation result.'
    }
    $finalFocusedValidation = $finalFocusedValidationProperty.Value
    $regressionRows = @($regressionMatrix.findings | ForEach-Object regressionTests)
    $regressionRowIds = @($regressionRows | ForEach-Object id)
    $sourceResultRows = @($focusedValidationResult.results)
    $sourceResultIds = @($sourceResultRows | ForEach-Object { [string]$_.id })
    $sourceEvidenceBytes = $entryBytes['focused-validation-result.json']
    Add-Result -Id 'CORRECTION-FOCUSED-SOURCE-EVIDENCE-BYTE-BINDING' -Passed (
        [string]$finalFocusedValidation.sourceArtifact -ceq 'focused-validation-result.json' -and
        [int64]$finalFocusedValidation.sourceEvidenceLength -eq $sourceEvidenceBytes.LongLength -and
        [string]$finalFocusedValidation.sourceEvidenceSha256 -ceq (Get-Hash -Bytes $sourceEvidenceBytes)
    )
    Add-Result -Id 'CORRECTION-FOCUSED-SOURCE-EVIDENCE-RESULT' -Passed (
        [string]$focusedValidationResult.status -ceq 'PASS' -and
        [int]$focusedValidationResult.selected -eq $sourceResultRows.Count -and
        [int]$focusedValidationResult.passed -eq @($sourceResultRows | Where-Object result -ceq 'PASS').Count -and
        [int]$focusedValidationResult.failed -eq @($sourceResultRows | Where-Object result -ceq 'FAIL').Count -and
        [int]$focusedValidationResult.failed -eq 0 -and
        [int]$focusedValidationResult.selected -eq [int]$focusedValidationResult.passed
    )
    Assert-EqualSet -Left $sourceResultIds -Right $regressionRowIds `
        -Label 'CORRECTION-FOCUSED-SOURCE-EVIDENCE-RESULT-SET'
    Add-Result -Id 'CORRECTION-FINAL-FOCUSED-VALIDATION-EVIDENCE' -Passed (
        [string]$finalFocusedValidation.status -ceq 'PASS' -and
        [int]$finalFocusedValidation.selected -eq [int]$focusedValidationResult.selected -and
        [int]$finalFocusedValidation.passed -eq [int]$focusedValidationResult.passed -and
        [int]$finalFocusedValidation.failed -eq [int]$focusedValidationResult.failed -and
        [int]$finalFocusedValidation.selected -eq $regressionRowIds.Count -and
        [int]$finalFocusedValidation.passed -eq @($regressionRows | Where-Object status -ceq 'PASS').Count -and
        [int]$finalFocusedValidation.failed -eq 0 -and
        [int]$regressionMatrix.fixtureCount -eq [int]$finalFocusedValidation.selected -and
        @($regressionRowIds | Sort-Object -Unique).Count -eq $regressionRowIds.Count
    )
    Add-Result -Id 'CORRECTION-REPORT-FOCUSED-VALIDATION' -Passed (
        [string]$reportContract.permanentRegressionEvidence.result -ceq 'PASS' -and
        [string]$reportContract.focusedValidationResult.result -ceq [string]$validationSummary.focusedFixtureResult -and
        [int]$reportContract.focusedValidationResult.selected -eq [int]$finalFocusedValidation.selected -and
        [int]$reportContract.focusedValidationResult.passed -eq [int]$finalFocusedValidation.passed -and
        [int]$validationSummary.focusedFixtureCount -eq [int]$finalFocusedValidation.selected -and
        [int]$validationSummary.focusedFixtureSelectedCount -eq [int]$finalFocusedValidation.selected -and
        [int]$validationSummary.focusedFixturePassedCount -eq [int]$finalFocusedValidation.passed -and
        [string]$validationSummary.focusedFixtureEvidenceSha256 -ceq [string]$finalFocusedValidation.sourceEvidenceSha256 -and
        [int64]$validationSummary.focusedFixtureEvidenceLength -eq [int64]$finalFocusedValidation.sourceEvidenceLength -and
        [bool]$reportContract.independentDeltaReviewRequired
    )

    $currentPatchEvidence = Get-GenericPatchDeltaEvidence -Root $AuthoritativeRepositoryRoot `
        -BaselineCommit ([string]$assignment.baselineCommit) -PatchBytes $entryBytes['current-delta.patch'] `
        -IncludedEntry $currentEntries -ExcludedEntry @()
    Add-Result -Id 'CORRECTION-CURRENT-PATCH-GIT-INVENTORY' -Passed (
        [bool]$currentPatchEvidence.ActualDeltaInventoryParity -and
        [bool]$currentPatchEvidence.ExcludedDeltaPathProhibition -and
        [bool]$currentPatchEvidence.RealObjectDatabaseImmutable -and
        [bool]$currentPatchEvidence.TemporaryArtifactsRemoved
    ) -Evidence ("parity={0};excluded={1};objects={2};cleanup={3};expected={4};actual={5}" -f
        $currentPatchEvidence.ActualDeltaInventoryParity,
        $currentPatchEvidence.ExcludedDeltaPathProhibition,
        $currentPatchEvidence.RealObjectDatabaseImmutable,
        $currentPatchEvidence.TemporaryArtifactsRemoved,
        (@($currentPatchEvidence.ExpectedDeltaInventoryKeys) -join ','),
        (@($currentPatchEvidence.ActualDeltaInventory | ForEach-Object key | Sort-Object) -join ','))

    $hashBindings = [ordered]@{
        scopeInventorySha256 = Get-Hash $entryBytes['scope-inventory.json']
        correctionScopeInventorySha256 = Get-Hash $entryBytes['correction-scope-inventory.json']
        currentDeltaSha256 = Get-Hash $entryBytes['current-delta.patch']
        correctionPatchSha256 = Get-Hash $entryBytes['correction-only.patch']
        findingLedgerSha256 = Get-Hash $entryBytes['finding-ledger.json']
        previousReviewBindingSha256 = Get-Hash $entryBytes['previous-review-binding.json']
    }
    foreach ($property in $hashBindings.Keys) {
        Add-Result -Id ('CORRECTION-HASH-' + $property) -Passed ([string]$assignment.$property -ceq [string]$hashBindings[$property])
    }
    Add-Result -Id 'CORRECTION-PATCH-HASH-PARITY' -Passed (
        [string]$focused.currentDeltaSha256 -ceq $hashBindings.currentDeltaSha256 -and
        [string]$focused.correctionPatchSha256 -ceq $hashBindings.correctionPatchSha256 -and
        [string]$trusted.currentDeltaSha256 -ceq $hashBindings.currentDeltaSha256 -and
        [string]$trusted.correctionPatchSha256 -ceq $hashBindings.correctionPatchSha256
    )
    Add-Result -Id 'CORRECTION-REPORT-PATCH-BINDING' -Passed (
        [string]$reportContract.currentFeaturePatch.artifact -ceq 'current-delta.patch' -and
        [string]$reportContract.currentFeaturePatch.sha256 -ceq $hashBindings.currentDeltaSha256 -and
        [string]$reportContract.correctionOnlyPatch.artifact -ceq 'correction-only.patch' -and
        [string]$reportContract.correctionOnlyPatch.sha256 -ceq $hashBindings.correctionPatchSha256
    )
    Add-Result -Id 'CORRECTION-REPORT-PREVIOUS-BINDING-HASH' -Passed (
        [string]$reportContract.previousReview.bindingArtifact -ceq 'previous-review-binding.json' -and
        [string]$reportContract.previousReview.bindingSha256 -ceq $hashBindings.previousReviewBindingSha256
    )
    if ($hasIndependentReviewOutcomeMember) {
        $memberOutcomeHash = (Get-Hash $entryBytes['previous-independent-review-outcome.json']).ToUpperInvariant()
        Add-Result -Id 'CORRECTION-INDEPENDENT-OUTCOME-CONTRACT-HASH-PARITY' -Passed (
            [string]$assignment.previousIndependentReviewOutcomeSha256 -ceq $memberOutcomeHash -and
            [string]$completion.previousIndependentReviewOutcomeSha256 -ceq $memberOutcomeHash -and
            [string]$focused.previousIndependentReviewOutcomeSha256 -ceq $memberOutcomeHash
        ) -Evidence "member=$memberOutcomeHash"
    }

    $previousState = if ([int]$previous.schemaVersion -eq 2) {
        Convert-LegacyPreviousState -Previous $previous
    }
    else { $previous.previousReviewState }
    $focusedState = if ([int]$focused.schemaVersion -eq 2) {
        [pscustomobject]@{ type = 'COMMIT'; correctionStartCommit = [string]$focused.correctionStartCommit }
    }
    else { $focused.previousReviewState }
    Add-Result -Id 'CORRECTION-PREVIOUS-STATE-DISCRIMINATOR' -Passed (
        [string]$previousState.type -in @('COMMIT', 'IMMUTABLE_REVIEW_PACKAGE') -and
        [string]$previousState.type -ceq [string]$focusedState.type -and
        [string]$reportContract.previousReview.stateType -ceq [string]$previousState.type
    )
    Add-Result -Id 'CORRECTION-REPORT-PREVIOUS-REVIEW-SHA' -Passed (
        [string]$reportContract.previousReview.reviewStateSha256 -ceq [string]$focused.previousReviewSha256 -and
        [string]$reportContract.previousReview.reviewStateSha256 -ceq [string]$trusted.previousReviewSha256
    )

    $context = New-GenericGitIsolationContext -Root $AuthoritativeRepositoryRoot
    $currentPatchPath = Join-Path $context.Root 'current.patch'
    $correctionPatchPath = Join-Path $context.Root 'correction.patch'
    [System.IO.File]::WriteAllBytes($currentPatchPath, $entryBytes['current-delta.patch'])
    [System.IO.File]::WriteAllBytes($correctionPatchPath, $entryBytes['correction-only.patch'])
    $previousTree = $null
    if ([string]$previousState.type -ceq 'COMMIT') {
        Add-Result -Id 'CORRECTION-COMMIT-STATE-PARITY' -Passed (
            [string]$previousState.correctionStartCommit -ceq [string]$focusedState.correctionStartCommit
        )
        $commitCheck = Invoke-GenericGitBytes -Root $AuthoritativeRepositoryRoot `
            -Argument @('cat-file', '-e', ([string]$previousState.correctionStartCommit + '^{commit}')) `
            -Environment $context.Environment -AllowedExitCode @(0, 1, 128)
        Add-Result -Id 'CORRECTION-COMMIT-OBJECT-EXISTS' -Passed ($commitCheck.ExitCode -eq 0)
        $previousTree = Invoke-IsolatedGitText @('rev-parse', ([string]$previousState.correctionStartCommit + '^{tree}'))
        Add-Result -Id 'CORRECTION-COMMIT-TREE-BINDING' -Passed ($previousTree -ceq [string]$previousState.previousReviewedTree)
    }
    else {
        foreach ($property in @('historicalPackageSha256', 'historicalManifestSha256', 'historicalPatchSha256', 'historicalScopeInventorySha256', 'previousReviewedBaselineCommit', 'previousReviewedTree', 'previousReviewedPathCount')) {
            Add-Result -Id ('CORRECTION-PREVIOUS-PARITY-' + $property) -Passed (
                [string]$focusedState.$property -ceq [string]$previousState.$property
            )
        }
        Assert-EqualSet @($focusedState.previousReviewedPostimages | ForEach-Object { "$(($_ | ConvertTo-Json -Compress))" }) `
            @($previousState.previousReviewedPostimages | ForEach-Object { "$(($_ | ConvertTo-Json -Compress))" }) 'CORRECTION-PREVIOUS-POSTIMAGE-PARITY'
        $historicalPackagePath = [System.IO.Path]::GetFullPath([string]$previousState.historicalPackagePath)
        Add-Result -Id 'CORRECTION-HISTORICAL-PACKAGE-EXISTS' -Passed (Test-Path -LiteralPath $historicalPackagePath -PathType Leaf)
        Add-Result -Id 'CORRECTION-HISTORICAL-PACKAGE-HASH' -Passed ((Get-FileHash -LiteralPath $historicalPackagePath -Algorithm SHA256).Hash.ToLowerInvariant() -ceq [string]$previousState.historicalPackageSha256)
        $historicalEntries = [System.Collections.Generic.Dictionary[string, byte[]]]::new(
            [System.StringComparer]::Ordinal
        )
        $historicalEntryNamesIgnoreCase = [System.Collections.Generic.HashSet[string]]::new(
            [System.StringComparer]::OrdinalIgnoreCase
        )
        Add-Type -AssemblyName System.IO.Compression
        $historicalArchive = [System.IO.Compression.ZipFile]::OpenRead($historicalPackagePath)
        try {
            foreach ($historicalEntry in $historicalArchive.Entries) {
                $name = [string]$historicalEntry.FullName
                if ($name -notmatch '^[A-Za-z0-9][A-Za-z0-9._-]*$' -or
                    -not $historicalEntryNamesIgnoreCase.Add($name) -or
                    $historicalEntries.ContainsKey($name)) {
                    throw "Historical package contains an unsafe, duplicate, or case-colliding member: $name"
                }
                $memory = [System.IO.MemoryStream]::new()
                try {
                    $stream = $historicalEntry.Open()
                    try { $stream.CopyTo($memory) } finally { $stream.Dispose() }
                    $historicalEntries.Add($name, $memory.ToArray())
                }
                finally { $memory.Dispose() }
            }
        }
        finally { $historicalArchive.Dispose() }
        Add-Result -Id 'CORRECTION-HISTORICAL-MEMBER-HASHES' -Passed (
            (Get-Hash $historicalEntries['MANIFEST.sha256']) -ceq [string]$previousState.historicalManifestSha256 -and
            (Get-Hash $historicalEntries['current-delta.patch']) -ceq [string]$previousState.historicalPatchSha256 -and
            (Get-Hash $historicalEntries['scope-inventory.json']) -ceq [string]$previousState.historicalScopeInventorySha256
        )

        $genericDiscriminatorNames = @(
            'assignment-record.json', 'completion-report.json', 'package-inventory.json'
        )
        $genericDiscriminatorCount = @(
            $genericDiscriminatorNames | Where-Object { $historicalEntries.ContainsKey($_) }
        ).Count
        $historicalAssignmentDiscriminator = if ($historicalEntries.ContainsKey('assignment-record.json')) {
            $historicalDiscriminatorText = [System.Text.UTF8Encoding]::new($false, $true).GetString(
                $historicalEntries['assignment-record.json']
            )
            $historicalDiscriminatorText | ConvertFrom-Json -Depth 100
        }
        else { $null }
        $historicalIsFindingCorrection = (
            $genericDiscriminatorCount -eq $genericDiscriminatorNames.Count -and
            [string]$historicalAssignmentDiscriminator.profile -ceq 'FINDING_CORRECTION'
        )
        $historicalPackageProfile = $null
        $historicalAllEntries = @()
        $historicalEntriesList = @()
        $historicalSemanticEntryCount = 0
        if ($genericDiscriminatorCount -eq 0) {
            Assert-EqualSet -Left @($historicalEntries.Keys) `
                -Right @('MANIFEST.sha256', 'current-delta.patch', 'scope-inventory.json') `
                -Label 'CORRECTION-HISTORICAL-LEGACY-PACKAGE-SHAPE'
            $historicalScope = Read-JsonBytes $historicalEntries['scope-inventory.json'] `
                'historical legacy scope-inventory.json' '' 1
            Add-Result -Id 'CORRECTION-HISTORICAL-LEGACY-SCOPE-DISCRIMINATOR' -Passed (
                $null -ne $historicalScope.PSObject.Properties['pathCount'] -and
                $null -ne $historicalScope.PSObject.Properties['entries'] -and
                $null -eq $historicalScope.PSObject.Properties['profile']
            )
            $historicalPackageProfile = 'LEGACY_FINDING_CORRECTION_SNAPSHOT'
            $historicalAllEntries = @($historicalScope.entries)
            $historicalEntriesList = @($historicalAllEntries)
            $historicalSemanticEntryCount = $historicalEntriesList.Count
            Add-Result -Id 'CORRECTION-HISTORICAL-LEGACY-PATHCOUNT' -Passed (
                [int]$historicalScope.pathCount -eq $historicalSemanticEntryCount
            ) -Evidence "declared=$($historicalScope.pathCount);entries=$historicalSemanticEntryCount"
        }
        elseif ($historicalIsFindingCorrection) {
            $historicalAssignment = Read-JsonBytes $historicalEntries['assignment-record.json'] `
                'historical focused assignment-record.json' 'finding-correction-assignment.schema.json' 2
            $historicalCompletion = Read-JsonBytes $historicalEntries['completion-report.json'] `
                'historical focused completion-report.json' 'finding-correction-completion.schema.json' 2
            $historicalInventory = Read-JsonBytes $historicalEntries['package-inventory.json'] `
                'historical focused package-inventory.json' '' 1
            $historicalScope = Read-JsonBytes $historicalEntries['scope-inventory.json'] `
                'historical focused scope-inventory.json' '' 1
            $historicalFocused = Read-JsonBytes $historicalEntries['focused-delta-review-record.json'] `
                'historical focused focused-delta-review-record.json' 'focused-delta-review-record.schema.json' 3
            $historicalPrevious = Read-JsonBytes $historicalEntries['previous-review-binding.json'] `
                'historical focused previous-review-binding.json' 'previous-review-binding.schema.json' 3
            $historicalRequiredNames = @(
                'HANDOFF.md', 'assignment-record.json', 'completion-report.json',
                'correction-only.patch', 'correction-scope-inventory.json',
                'current-delta.patch', 'external-governance-manifest.json',
                'finding-correction-matrix.json', 'finding-ledger.json',
                'finding-regression-matrix.json', 'focused-delta-review-record.json',
                'MANIFEST.sha256', 'package-inventory.json', 'previous-review-binding.json',
                'readiness-evidence.json', 'report.md', 'scope-inventory.json',
                'trusted-expected-hashes.json', 'validation-summary.json'
            )
            $historicalOutcomeBindings = @(
                $historicalAssignment.PSObject.Properties['previousIndependentReviewOutcomeSha256'],
                $historicalCompletion.PSObject.Properties['previousIndependentReviewOutcomeSha256'],
                $historicalFocused.PSObject.Properties['previousIndependentReviewOutcomeSha256']
            )
            $historicalOutcomeBindingCount = @(
                $historicalOutcomeBindings | Where-Object { $null -ne $_ }
            ).Count
            Add-Result -Id 'CORRECTION-HISTORICAL-FOCUSED-OUTCOME-BINDING-SHAPE' -Passed (
                $historicalOutcomeBindingCount -in @(0, 3)
            )
            $historicalHasIndependentOutcome = $historicalOutcomeBindingCount -eq 3
            if ($historicalHasIndependentOutcome) {
                $historicalRequiredNames += 'previous-independent-review-outcome.json'
            }
            $historicalHasPublicationEvidence = $historicalEntries.ContainsKey('publication-regression-evidence.json')
            $historicalHasPublicationResult = $historicalEntries.ContainsKey('publication-regression-result.json')
            $historicalPublicationRequired = (
                $null -ne $historicalFocused.PSObject.Properties['publicationRegressionEvidence']
            )
            Add-Result -Id 'CORRECTION-HISTORICAL-FOCUSED-PUBLICATION-PAIR' -Passed (
                $historicalHasPublicationEvidence -eq $historicalHasPublicationResult -and
                $historicalHasPublicationEvidence -eq $historicalPublicationRequired
            )
            if ($historicalPublicationRequired) {
                $historicalRequiredNames += @('publication-regression-evidence.json', 'publication-regression-result.json')
            }
            $historicalHasFocusedSourceEvidence = $historicalEntries.ContainsKey('focused-validation-result.json')
            if ($historicalHasFocusedSourceEvidence) {
                $historicalRequiredNames += 'focused-validation-result.json'
            }
            Assert-EqualSet -Left @($historicalEntries.Keys) -Right $historicalRequiredNames `
                -Label 'CORRECTION-HISTORICAL-FOCUSED-PROFILE-DRIVEN-PACKAGE-SHAPE'
            $historicalPackageProfile = 'FINDING_CORRECTION'

            if ($historicalHasFocusedSourceEvidence) {
                $historicalRegression = Read-JsonBytes $historicalEntries['finding-regression-matrix.json'] `
                    'historical focused finding-regression-matrix.json' `
                    'finding-regression-matrix.schema.json' 2
                $historicalSourceResult = Read-JsonBytes $historicalEntries['focused-validation-result.json'] `
                    'historical focused focused-validation-result.json' '' 2
                $historicalSourceBinding = $historicalRegression.finalFocusedValidationEvidence
                $historicalSourceRows = @($historicalSourceResult.results)
                $historicalRegressionRows = @($historicalRegression.findings | ForEach-Object regressionTests)
                Add-Result -Id 'CORRECTION-HISTORICAL-FOCUSED-SOURCE-BYTE-BINDING' -Passed (
                    [string]$historicalSourceBinding.sourceArtifact -ceq 'focused-validation-result.json' -and
                    [int64]$historicalSourceBinding.sourceEvidenceLength -eq
                        $historicalEntries['focused-validation-result.json'].LongLength -and
                    [string]$historicalSourceBinding.sourceEvidenceSha256 -ceq
                        (Get-Hash $historicalEntries['focused-validation-result.json']) -and
                    [string]$historicalSourceResult.status -ceq 'PASS' -and
                    [int]$historicalSourceResult.selected -eq $historicalSourceRows.Count -and
                    [int]$historicalSourceResult.passed -eq
                        @($historicalSourceRows | Where-Object result -ceq 'PASS').Count -and
                    [int]$historicalSourceResult.failed -eq 0
                )
                Assert-EqualSet -Left @($historicalSourceRows | ForEach-Object id) `
                    -Right @($historicalRegressionRows | ForEach-Object id) `
                    -Label 'CORRECTION-HISTORICAL-FOCUSED-SOURCE-RESULT-SET'
            }

            Add-Result -Id 'CORRECTION-HISTORICAL-FOCUSED-PROFILE' -Passed (
                [string]$historicalAssignment.profile -ceq 'FINDING_CORRECTION' -and
                [string]$historicalCompletion.profile -ceq 'FINDING_CORRECTION' -and
                [string]$historicalInventory.profile -ceq 'FINDING_CORRECTION' -and
                [string]$historicalAssignment.transitionType -ceq 'BUNDLED_CORRECTION_TO_FOCUSED_DELTA_REVIEW' -and
                [string]$historicalCompletion.transitionType -ceq 'BUNDLED_CORRECTION_TO_FOCUSED_DELTA_REVIEW' -and
                [string]$historicalInventory.transitionType -ceq 'BUNDLED_CORRECTION_TO_FOCUSED_DELTA_REVIEW'
            )
            Add-Result -Id 'CORRECTION-HISTORICAL-FOCUSED-IDENTITY' -Passed (
                [string]$historicalAssignment.taskId -ceq [string]$assignment.taskId -and
                [string]$historicalCompletion.taskId -ceq [string]$assignment.taskId -and
                [string]$historicalScope.taskId -ceq [string]$assignment.taskId -and
                [string]$historicalInventory.taskId -ceq [string]$assignment.taskId
            )
            $historicalPatchHash = Get-Hash $historicalEntries['current-delta.patch']
            $historicalScopeHash = Get-Hash $historicalEntries['scope-inventory.json']
            Add-Result -Id 'CORRECTION-HISTORICAL-FOCUSED-HASH-PARITY' -Passed (
                [string]$historicalAssignment.currentDeltaSha256 -ceq $historicalPatchHash -and
                [string]$historicalFocused.currentDeltaSha256 -ceq $historicalPatchHash -and
                [string]$historicalAssignment.scopeInventorySha256 -ceq $historicalScopeHash
            )

            $historicalInventoryEntries = @($historicalInventory.entries)
            $historicalInventoryNames = @(
                $historicalEntries.Keys | Where-Object { $_ -notin @('MANIFEST.sha256', 'package-inventory.json') }
            )
            Assert-EqualSet @($historicalInventoryEntries | ForEach-Object path) `
                $historicalInventoryNames 'CORRECTION-HISTORICAL-FOCUSED-INVENTORY-COVERAGE'
            foreach ($historicalInventoryEntry in $historicalInventoryEntries) {
                $inventoryName = [string]$historicalInventoryEntry.path
                Add-Result -Id ('CORRECTION-HISTORICAL-FOCUSED-INVENTORY-' + $inventoryName) -Passed (
                    [string]$historicalInventoryEntry.sha256 -ceq (Get-Hash $historicalEntries[$inventoryName]) -and
                    [int64]$historicalInventoryEntry.length -eq $historicalEntries[$inventoryName].LongLength
                )
            }
            $historicalManifestText = [System.Text.UTF8Encoding]::new($false, $true).GetString(
                $historicalEntries['MANIFEST.sha256']
            )
            $historicalManifestNames = @($historicalManifestText.TrimEnd("`r", "`n") -split "`n" | ForEach-Object {
                    $manifestMatch = [regex]::Match(
                        $_,
                        '^(?<hash>[0-9a-f]{64})  (?<length>[0-9]+)  (?<path>[A-Za-z0-9][A-Za-z0-9._-]*)$'
                    )
                    if (-not $manifestMatch.Success) { throw "Invalid historical focused manifest line: $_" }
                    $manifestName = $manifestMatch.Groups['path'].Value
                    if (-not $historicalEntries.ContainsKey($manifestName) -or
                        $manifestMatch.Groups['hash'].Value -cne (Get-Hash $historicalEntries[$manifestName]) -or
                        [int64]$manifestMatch.Groups['length'].Value -ne $historicalEntries[$manifestName].LongLength) {
                        throw "Historical focused manifest mismatch: $manifestName"
                    }
                    $manifestName
                })
            Assert-EqualSet $historicalManifestNames `
                @($historicalEntries.Keys | Where-Object { $_ -cne 'MANIFEST.sha256' }) `
                'CORRECTION-HISTORICAL-FOCUSED-MANIFEST-COVERAGE'

            $historicalEmbeddedOutcome = $null
            $historicalEmbeddedOutcomeHash = $null
            if ($historicalHasIndependentOutcome) {
                $historicalEmbeddedOutcomeHash = (
                    Get-Hash $historicalEntries['previous-independent-review-outcome.json']
                ).ToUpperInvariant()
                Add-Result -Id 'CORRECTION-HISTORICAL-FOCUSED-OUTCOME-HASH-PARITY' -Passed (
                    [string]$historicalAssignment.previousIndependentReviewOutcomeSha256 -ceq
                        $historicalEmbeddedOutcomeHash -and
                    [string]$historicalCompletion.previousIndependentReviewOutcomeSha256 -ceq
                        $historicalEmbeddedOutcomeHash -and
                    [string]$historicalFocused.previousIndependentReviewOutcomeSha256 -ceq
                        $historicalEmbeddedOutcomeHash
                )
                $historicalEmbeddedOutcome = Read-JsonBytes `
                    $historicalEntries['previous-independent-review-outcome.json'] `
                    'historical focused previous-independent-review-outcome.json' `
                    'generic-independent-review-evidence.schema.json' 1
            }

            $outcomeEvidence = Read-IndependentReviewOutcome `
                -LiteralPath $IndependentReviewOutcomePath `
                -ExpectedSha256 $ExpectedIndependentReviewOutcomeSha256 `
                -ExpectedTaskId ([string]$assignment.taskId)
            $outcome = $outcomeEvidence.Contract
            $independentReviewOutcomeSha256 = $outcomeEvidence.Sha256
            $immediatePreviousReviewPackageSha256 = (
                Get-FileHash -LiteralPath $historicalPackagePath -Algorithm SHA256
            ).Hash.ToUpperInvariant()
            Add-Result -Id 'CORRECTION-HISTORICAL-FOCUSED-TRANSITIVE-STATE' -Passed (
                [string]$historicalFocused.previousReviewState.type -ceq 'IMMUTABLE_REVIEW_PACKAGE' -and
                [string]$historicalPrevious.previousReviewState.type -ceq 'IMMUTABLE_REVIEW_PACKAGE' -and
                [string]$historicalFocused.previousReviewSha256 -ceq
                    [string]$historicalFocused.previousReviewState.historicalPackageSha256 -and
                [string]$historicalFocused.previousReviewSha256 -ceq
                    [string]$historicalPrevious.previousReviewState.historicalPackageSha256
            )
            $recursiveChain = Resolve-ReviewPackageChain `
                -InitialPackagePath ([string]$historicalFocused.previousReviewPackage) `
                -ExpectedInitialSha256 ([string]$historicalFocused.previousReviewSha256) `
                -ExpectedTaskId ([string]$assignment.taskId)
            $transitiveFullReviewBaselineSha256 = $recursiveChain.TransitiveFullReviewSha256
            if ($historicalHasIndependentOutcome) {
                Add-Result -Id 'CORRECTION-HISTORICAL-FOCUSED-OUTCOME-HAS-REVIEWED-NODE' -Passed (
                    @($recursiveChain.Nodes).Count -gt 0
                )
                [void](Assert-OutcomeReviewsFocusedPackage -Outcome $historicalEmbeddedOutcome `
                        -ReviewedNode @($recursiveChain.Nodes)[0] `
                        -TransitiveFullReviewSha256 $transitiveFullReviewBaselineSha256 `
                        -Label 'CORRECTION-HISTORICAL-EMBEDDED-OUTCOME')
            }

            $historicalFindingIds = @(Get-CanonicalSet -Value @($historicalFocused.reviewedFindingIds) `
                -Label 'CORRECTION-HISTORICAL-FOCUSED-FINDINGS')
            Assert-EqualSet $historicalAssignment.findingIds $historicalFindingIds `
                'CORRECTION-HISTORICAL-FOCUSED-FINDING-UNIVERSE'
            $historicalNode = [pscustomobject][ordered]@{
                PackagePath = $historicalPackagePath
                PackageSha256 = $immediatePreviousReviewPackageSha256
                Profile = 'FINDING_CORRECTION'
                Entries = $historicalEntries
                Contract = [pscustomobject][ordered]@{
                    Assignment = $historicalAssignment
                    Completion = $historicalCompletion
                    Inventory = $historicalInventory
                    Focused = $historicalFocused
                    EmbeddedOutcome = $historicalEmbeddedOutcome
                    EmbeddedOutcomeHash = $historicalEmbeddedOutcomeHash
                    CurrentDeltaSha256 = $historicalPatchHash.ToUpperInvariant()
                    CorrectionPatchSha256 = (
                        Get-Hash $historicalEntries['correction-only.patch']
                    ).ToUpperInvariant()
                }
            }
            $outcomeState = Assert-OutcomeReviewsFocusedPackage -Outcome $outcome `
                -ReviewedNode $historicalNode `
                -TransitiveFullReviewSha256 $transitiveFullReviewBaselineSha256 `
                -Label 'CORRECTION-INDEPENDENT-OUTCOME'
            Assert-EqualSet $assignment.findingIds $outcomeState.OpenFindingIds `
                'CORRECTION-INDEPENDENT-OUTCOME-TARGET-FINDINGS'
            Add-Result -Id 'CORRECTION-INDEPENDENT-OUTCOME-MEMBER-PRESENT' -Passed $hasIndependentReviewOutcomeMember
            Add-Result -Id 'CORRECTION-INDEPENDENT-OUTCOME-MEMBER-BYTES' -Passed (
                $hasIndependentReviewOutcomeMember -and
                (Get-Hash $entryBytes['previous-independent-review-outcome.json']) -ceq
                    (Get-Hash $outcomeEvidence.Bytes)
            )
            $independentReviewOutcomeValidation = 'PASS'
            $findingDispositionBindingResult = 'PASS'

            $historicalAllEntries = @($historicalScope.entries)
            $historicalEntriesList = @($historicalAllEntries)
            $historicalSemanticEntryCount = $historicalEntriesList.Count
            Add-Result -Id 'CORRECTION-HISTORICAL-FOCUSED-PATHCOUNT' -Passed (
                [int]$historicalScope.pathCount -eq $historicalSemanticEntryCount
            )
        }
        elseif ($genericDiscriminatorCount -eq $genericDiscriminatorNames.Count) {
            $genericExpectedNames = @(
                'HANDOFF.md', 'assignment-record.json', 'completion-report.json',
                'current-delta.patch', 'MANIFEST.sha256', 'package-inventory.json',
                'pre-review-validation-evidence.json', 'report.md', 'scope-inventory.json',
                'task.patch', 'validation-summary.json'
            )
            Assert-EqualSet -Left @($historicalEntries.Keys) -Right $genericExpectedNames `
                -Label 'CORRECTION-HISTORICAL-GENERIC-PACKAGE-SHAPE'
            $historicalAssignment = Read-JsonBytes $historicalEntries['assignment-record.json'] `
                'historical assignment-record.json' 'generic-assignment-record.schema.json' 1
            $historicalCompletion = Read-JsonBytes $historicalEntries['completion-report.json'] `
                'historical completion-report.json' 'generic-completion-report.schema.json' 1
            $historicalInventory = Read-JsonBytes $historicalEntries['package-inventory.json'] `
                'historical package-inventory.json' 'generic-package-inventory.schema.json' 1
            $historicalScope = Read-JsonBytes $historicalEntries['scope-inventory.json'] `
                'historical scope-inventory.json' 'generic-scope-inventory.schema.json' 1
            $historicalPackageProfile = [string]$historicalAssignment.profile
            $expectedHistoricalProfile = 'IMPLEMENTATION_TO_INDEPENDENT_FULL_REVIEW'
            $expectedHistoricalTransition = 'IMPLEMENTATION_TO_INDEPENDENT_FULL_REVIEW'
            Add-Result -Id 'CORRECTION-HISTORICAL-GENERIC-PROFILE' -Passed (
                $historicalPackageProfile -ceq $expectedHistoricalProfile -and
                [string]$historicalCompletion.profile -ceq $expectedHistoricalProfile -and
                [string]$historicalInventory.profile -ceq $expectedHistoricalProfile -and
                [string]$historicalScope.profile -ceq $expectedHistoricalProfile
            )
            Add-Result -Id 'CORRECTION-HISTORICAL-GENERIC-TRANSITION' -Passed (
                [string]$historicalAssignment.transitionType -ceq $expectedHistoricalTransition -and
                [string]$historicalCompletion.transitionType -ceq $expectedHistoricalTransition -and
                [string]$historicalInventory.transitionType -ceq $expectedHistoricalTransition
            )
            Add-Result -Id 'CORRECTION-HISTORICAL-GENERIC-IDENTITY-PARITY' -Passed (
                [string]$historicalAssignment.taskId -ceq [string]$historicalCompletion.taskId -and
                [string]$historicalAssignment.taskId -ceq [string]$historicalScope.taskId -and
                [string]$historicalAssignment.taskId -ceq [string]$historicalInventory.taskId -and
                [string]$historicalAssignment.repository -ceq [string]$historicalCompletion.repository -and
                [string]$historicalAssignment.repository -ceq [string]$historicalScope.repository -and
                [string]$historicalAssignment.baselineCommit -ceq [string]$historicalCompletion.baselineCommit -and
                [string]$historicalAssignment.baselineCommit -ceq [string]$historicalScope.baselineCommit -and
                [string]$historicalAssignment.currentCommit -ceq [string]$historicalCompletion.currentCommit -and
                [string]$historicalAssignment.currentCommit -ceq [string]$historicalScope.currentCommit -and
                [string]$historicalAssignment.branch -ceq [string]$historicalCompletion.branch -and
                [string]$historicalAssignment.branch -ceq [string]$historicalScope.branch
            )
            $historicalScopeHash = Get-Hash $historicalEntries['scope-inventory.json']
            $historicalPatchHash = Get-Hash $historicalEntries['current-delta.patch']
            Add-Result -Id 'CORRECTION-HISTORICAL-GENERIC-HASH-PARITY' -Passed (
                [string]$historicalAssignment.scopeInventorySha256 -ceq $historicalScopeHash -and
                [string]$historicalCompletion.scopeInventorySha256 -ceq $historicalScopeHash -and
                [string]$historicalAssignment.taskPatchSha256 -ceq (Get-Hash $historicalEntries['task.patch']) -and
                [string]$historicalCompletion.taskPatchSha256 -ceq (Get-Hash $historicalEntries['task.patch']) -and
                [string]$historicalAssignment.currentDeltaSha256 -ceq $historicalPatchHash -and
                [string]$historicalCompletion.currentDeltaSha256 -ceq $historicalPatchHash -and
                (Get-Hash $historicalEntries['task.patch']) -ceq $historicalPatchHash
            )
            Add-Result -Id 'CORRECTION-HISTORICAL-GENERIC-REVIEW-STATUS' -Passed (
                [bool]$historicalAssignment.classicReviewReady -and
                [bool]$historicalCompletion.classicReviewReady -and
                [string]$historicalCompletion.status -ceq 'CLASSIC_REVIEW_READY' -and
                [bool]$historicalCompletion.zipFreeReadinessPassed -and
                [string]$historicalCompletion.independentReviewStatus -ceq 'NOT_PERFORMED' -and
                -not [bool]$historicalAssignment.commitAuthorized -and
                -not [bool]$historicalCompletion.commitAuthorized
            )
            Assert-EqualSet $historicalAssignment.allowedDeltaPaths $historicalScope.allowedDeltaPaths `
                'CORRECTION-HISTORICAL-GENERIC-ASSIGNMENT-ALLOWED'
            Assert-EqualSet $historicalCompletion.allowedDeltaPaths $historicalScope.allowedDeltaPaths `
                'CORRECTION-HISTORICAL-GENERIC-COMPLETION-ALLOWED'
            Assert-EqualSet $historicalAssignment.excludedDeltaPaths $historicalScope.excludedDeltaPaths `
                'CORRECTION-HISTORICAL-GENERIC-ASSIGNMENT-EXCLUDED'
            Assert-EqualSet $historicalCompletion.excludedDeltaPaths $historicalScope.excludedDeltaPaths `
                'CORRECTION-HISTORICAL-GENERIC-COMPLETION-EXCLUDED'

            $historicalInventoryEntries = @($historicalInventory.entries)
            $historicalInventoryNames = @(
                $historicalEntries.Keys | Where-Object { $_ -notin @('MANIFEST.sha256', 'package-inventory.json') }
            )
            Assert-EqualSet @($historicalInventoryEntries | ForEach-Object path) `
                $historicalInventoryNames 'CORRECTION-HISTORICAL-GENERIC-INVENTORY-COVERAGE'
            foreach ($historicalInventoryEntry in $historicalInventoryEntries) {
                $inventoryName = [string]$historicalInventoryEntry.path
                Add-Result -Id ('CORRECTION-HISTORICAL-GENERIC-INVENTORY-' + $inventoryName) -Passed (
                    [string]$historicalInventoryEntry.sha256 -ceq (Get-Hash $historicalEntries[$inventoryName]) -and
                    [int64]$historicalInventoryEntry.length -eq $historicalEntries[$inventoryName].LongLength
                )
            }

            $historicalManifestText = [System.Text.UTF8Encoding]::new($false, $true).GetString(
                $historicalEntries['MANIFEST.sha256']
            )
            $historicalManifestLines = @($historicalManifestText.TrimEnd("`r", "`n") -split "`n")
            $historicalManifestNames = @($historicalManifestLines | ForEach-Object {
                    $manifestMatch = [regex]::Match(
                        $_,
                        '^(?<hash>[0-9a-f]{64})  (?<length>[0-9]+)  (?<path>[A-Za-z0-9][A-Za-z0-9._-]*)$'
                    )
                    if (-not $manifestMatch.Success) { throw "Invalid historical manifest line: $_" }
                    $manifestName = $manifestMatch.Groups['path'].Value
                    if (-not $historicalEntries.ContainsKey($manifestName) -or
                        $manifestMatch.Groups['hash'].Value -cne (Get-Hash $historicalEntries[$manifestName]) -or
                        [int64]$manifestMatch.Groups['length'].Value -ne $historicalEntries[$manifestName].LongLength) {
                        throw "Historical manifest mismatch: $manifestName"
                    }
                    $manifestName
                })
            Assert-EqualSet $historicalManifestNames `
                @($historicalEntries.Keys | Where-Object { $_ -cne 'MANIFEST.sha256' }) `
                'CORRECTION-HISTORICAL-GENERIC-MANIFEST-COVERAGE'

            $historicalAllEntries = @($historicalScope.entries)
            $historicalEntriesList = @(
                $historicalAllEntries | Where-Object { [string]$_.inclusionDecision -ceq 'INCLUDE' }
            )
            $historicalExcludedEntries = @(
                $historicalAllEntries | Where-Object { [string]$_.inclusionDecision -ceq 'EXCLUDE' }
            )
            [object[]]$historicalIncludedPaths = @()
            if ($historicalEntriesList.Count -gt 0) {
                $historicalIncludedPaths = @(Get-ExpandedScopePaths -Entries $historicalEntriesList)
            }
            [object[]]$historicalExcludedPaths = @()
            if ($historicalExcludedEntries.Count -gt 0) {
                $historicalExcludedPaths = @(Get-ExpandedScopePaths -Entries $historicalExcludedEntries)
            }
            Assert-EqualSet $historicalIncludedPaths `
                $historicalScope.allowedDeltaPaths 'CORRECTION-HISTORICAL-GENERIC-INCLUDED-SCOPE'
            $declaredHistoricalExcludedPaths = @($historicalScope.excludedDeltaPaths)
            if ($historicalExcludedPaths.Count -eq 0 -and $declaredHistoricalExcludedPaths.Count -eq 0) {
                Add-Result -Id 'CORRECTION-HISTORICAL-GENERIC-EXCLUDED-SCOPE' -Passed $true
            }
            elseif ($historicalExcludedPaths.Count -eq 0 -or $declaredHistoricalExcludedPaths.Count -eq 0) {
                Add-Result -Id 'CORRECTION-HISTORICAL-GENERIC-EXCLUDED-SCOPE' -Passed $false `
                    -Evidence "entries=$($historicalExcludedPaths.Count);declared=$($declaredHistoricalExcludedPaths.Count)"
            }
            else {
                Assert-EqualSet $historicalExcludedPaths $declaredHistoricalExcludedPaths `
                    'CORRECTION-HISTORICAL-GENERIC-EXCLUDED-SCOPE'
            }
            $historicalSemanticEntryCount = $historicalEntriesList.Count
        }
        else {
            throw 'Historical previous-review package has an incomplete or unknown profile discriminator.'
        }
        Add-Result -Id 'CORRECTION-HISTORICAL-PACKAGE-PROFILE-TYPED' -Passed (
            $historicalPackageProfile -in @(
                'IMPLEMENTATION_TO_INDEPENDENT_FULL_REVIEW',
                'FINDING_CORRECTION',
                'LEGACY_FINDING_CORRECTION_SNAPSHOT'
            )
        ) -Evidence "profile=$historicalPackageProfile"

        foreach ($historicalScopeEntry in $historicalAllEntries) {
            $historicalPath = [string]$historicalScopeEntry.path
            $historicalPreviousPath = if ($historicalScopeEntry.PSObject.Properties['previousPath']) {
                [string]$historicalScopeEntry.previousPath
            }
            else { '' }
            $historicalStatus = [string]$historicalScopeEntry.gitStatus
            Add-Result -Id ('CORRECTION-HISTORICAL-SCOPE-ENTRY-' + $historicalPath) -Passed (
                -not [string]::IsNullOrWhiteSpace($historicalPath) -and
                (($historicalStatus -ceq 'TRACKED_RENAMED' -and
                    -not [string]::IsNullOrWhiteSpace($historicalPreviousPath) -and
                    $historicalPreviousPath -cne $historicalPath) -or
                 ($historicalStatus -cne 'TRACKED_RENAMED' -and
                    [string]::IsNullOrWhiteSpace($historicalPreviousPath)))
            ) -Evidence "status=$historicalStatus;source=$historicalPreviousPath;target=$historicalPath"
        }
        $historicalPaths = @(Get-ExpandedScopePaths -Entries $historicalEntriesList)
        $historicalCanonicalPaths = @(Get-CanonicalSet -Value $historicalPaths -Label 'CORRECTION-HISTORICAL-SCOPE-PATHS')
        $historicalPathCountProperty = $historicalScope.PSObject.Properties['pathCount']
        $historicalUsesDeclaredPathCount = $null -ne $historicalPathCountProperty
        $historicalResolvedPathCount = $historicalEntriesList.Count
        $historicalGenericScopePass = $false
        if (-not $historicalUsesDeclaredPathCount) {
            $historicalProfileProperty = $historicalScope.PSObject.Properties['profile']
            $historicalAllowedPathsProperty = $historicalScope.PSObject.Properties['allowedDeltaPaths']
            if ($null -ne $historicalProfileProperty -and $null -ne $historicalAllowedPathsProperty) {
                $historicalAllowedPaths = @(Get-CanonicalSet -Value @($historicalScope.allowedDeltaPaths) `
                        -Label 'CORRECTION-HISTORICAL-GENERIC-ALLOWED-PATHS')
                $historicalGenericScopePass = (
                    [string]$historicalScope.profile -ceq 'IMPLEMENTATION_TO_INDEPENDENT_FULL_REVIEW' -and
                    ($historicalAllowedPaths -join "`n") -ceq ($historicalCanonicalPaths -join "`n")
                )
            }
        }
        Add-Result -Id 'CORRECTION-HISTORICAL-SCOPE-INTERNAL-PATHCOUNT' -Passed (
            ($historicalUsesDeclaredPathCount -and [int]$historicalScope.pathCount -eq $historicalEntriesList.Count) -or
            (-not $historicalUsesDeclaredPathCount -and $historicalGenericScopePass)
        ) -Evidence "declared=$(if($historicalUsesDeclaredPathCount){$historicalScope.pathCount}else{'ABSENT_GENERIC'});entries=$($historicalEntriesList.Count);paths=$($historicalCanonicalPaths.Count);generic=$historicalGenericScopePass"
        Add-Result -Id 'CORRECTION-HISTORICAL-SCOPE-PREVIOUS-COUNT' -Passed (
            [int]$previousState.previousReviewedPathCount -eq $historicalSemanticEntryCount
        ) -Evidence "previous=$($previousState.previousReviewedPathCount);semanticEntries=$historicalSemanticEntryCount;paths=$($historicalCanonicalPaths.Count);profile=$historicalPackageProfile;resolvedEntries=$historicalResolvedPathCount;generic=$historicalGenericScopePass"
        $historicalPatchPath = Join-Path $context.Root 'historical.patch'
        [System.IO.File]::WriteAllBytes($historicalPatchPath, $historicalEntries['current-delta.patch'])
        Invoke-IsolatedGitText @('read-tree', [string]$previousState.previousReviewedBaselineCommit) | Out-Null
        Invoke-IsolatedGitText @('apply', '--cached', '--binary', '--whitespace=nowarn', '--', $historicalPatchPath) | Out-Null
        $previousTree = Invoke-IsolatedGitText @('write-tree')
        Add-Result -Id 'CORRECTION-HISTORICAL-RECONSTRUCTED-TREE' -Passed ($previousTree -ceq [string]$previousState.previousReviewedTree)
        $historicalInventoryBytes = Invoke-GenericGitBytes -Root $AuthoritativeRepositoryRoot `
            -Argument @('diff-tree', '--no-commit-id', '--name-status', '-r', '-z', '--find-renames', `
                [string]$previousState.previousReviewedBaselineCommit, $previousTree, '--') `
            -Environment $context.Environment
        $historicalInventoryEntries = @(ConvertFrom-GenericNameStatusZ -Bytes $historicalInventoryBytes.Bytes)
        $historicalInventoryPaths = @(foreach ($historicalInventoryEntry in $historicalInventoryEntries) {
                if ([string]$historicalInventoryEntry.kind -ceq 'RENAME') { [string]$historicalInventoryEntry.previousPath }
                [string]$historicalInventoryEntry.path
            })
        Assert-EqualSet $historicalCanonicalPaths $historicalInventoryPaths 'CORRECTION-HISTORICAL-SCOPE-PATCH-INVENTORY'
        $boundPostimages = @($previousState.previousReviewedPostimages)
        Assert-EqualSet @($boundPostimages | ForEach-Object path) $historicalCanonicalPaths 'CORRECTION-PREVIOUS-POSTIMAGE-COVERAGE'
        foreach ($bound in $boundPostimages) {
            $actual = Get-TreePostimage -Tree $previousTree -Path ([string]$bound.path)
            Add-Result -Id ('CORRECTION-PREVIOUS-POSTIMAGE-' + [string]$bound.path) -Passed (
                (($actual | ConvertTo-Json -Compress) -ceq ($bound | ConvertTo-Json -Compress))
            )
        }
    }

    $correctionPatchEvidence = if ([string]$previousState.type -ceq 'IMMUTABLE_REVIEW_PACKAGE') {
        Get-SharedIsolationPatchDeltaEvidence -BaselineTree $previousTree `
            -PatchPath $correctionPatchPath -IncludedEntry $correctionEntries
    }
    else {
        Get-GenericPatchDeltaEvidence -Root $AuthoritativeRepositoryRoot `
            -BaselineCommit $previousTree -PatchBytes $entryBytes['correction-only.patch'] `
            -IncludedEntry $correctionEntries -ExcludedEntry @()
    }
    Add-Result -Id 'CORRECTION-ONLY-PATCH-GIT-INVENTORY' -Passed (
        [bool]$correctionPatchEvidence.ActualDeltaInventoryParity -and
        [bool]$correctionPatchEvidence.ExcludedDeltaPathProhibition -and
        [bool]$correctionPatchEvidence.RealObjectDatabaseImmutable -and
        [bool]$correctionPatchEvidence.TemporaryArtifactsRemoved
    ) -Evidence ("parity={0};excluded={1};objects={2};cleanup={3};expected={4};actual={5}" -f
        $correctionPatchEvidence.ActualDeltaInventoryParity,
        $correctionPatchEvidence.ExcludedDeltaPathProhibition,
        $correctionPatchEvidence.RealObjectDatabaseImmutable,
        $correctionPatchEvidence.TemporaryArtifactsRemoved,
        (@($correctionPatchEvidence.ExpectedDeltaInventoryKeys) -join ','),
        (@($correctionPatchEvidence.ActualDeltaInventory | ForEach-Object key | Sort-Object) -join ','))
    $actualCorrectionPaths = @(foreach ($entry in @($correctionPatchEvidence.ActualDeltaInventory)) {
            if ([string]$entry.kind -ceq 'RENAME') { [string]$entry.previousPath }
            [string]$entry.path
        })
    Assert-EqualSet $actualCorrectionPaths $correctionPaths 'CORRECTION-ACTUAL-SCOPE-PARITY'
    $referenceOverlap = @($actualCorrectionPaths | Where-Object { $_ -cin $referenceOnlyPaths })
    Add-Result -Id 'CORRECTION-REFERENCE-ONLY-CHANGED-PATH-EXCLUSION' -Passed ($referenceOverlap.Count -eq 0) `
        -Evidence ($referenceOverlap -join ',')
    $coveredCorrectionPaths = @($assignment.correctionOnlyPaths + $assignment.directInterfacePaths | Sort-Object -Unique)
    Assert-Subset $actualCorrectionPaths $coveredCorrectionPaths 'CORRECTION-ACTUAL-PATH-COVERAGE'

    Invoke-IsolatedGitText @('read-tree', [string]$assignment.baselineCommit) | Out-Null
    Invoke-IsolatedGitText @('apply', '--cached', '--binary', '--whitespace=nowarn', '--', $currentPatchPath) | Out-Null
    $currentTree = Invoke-IsolatedGitText @('write-tree')
    Invoke-IsolatedGitText @('read-tree', $previousTree) | Out-Null
    Invoke-IsolatedGitText @('apply', '--cached', '--binary', '--whitespace=nowarn', '--', $correctionPatchPath) | Out-Null
    $correctedTree = Invoke-IsolatedGitText @('write-tree')
    Add-Result -Id 'CORRECTION-PREVIOUS-PLUS-CORRECTION-EQUALS-CURRENT' -Passed ($correctedTree -ceq $currentTree)
    Add-Result -Id 'CORRECTION-PREVIOUS-TREE-BINDING' -Passed ([string]$correctionScope.previousReviewedTree -ceq $previousTree)
    Add-Result -Id 'CORRECTION-CURRENT-TREE-BINDING' -Passed ([string]$correctionScope.currentCorrectedTree -ceq $currentTree)
    foreach ($entry in $correctionEntries) {
        Assert-CorrectionEntryBinding -Entry $entry -PreviousTree $previousTree -CurrentTree $currentTree
    }

    $inventoryEntries = @($inventory.entries)
    $inventoryNames = @($entryBytes.Keys | Where-Object { $_ -notin @('package-inventory.json', 'MANIFEST.sha256') })
    Assert-EqualSet @($inventoryEntries | ForEach-Object path) $inventoryNames 'CORRECTION-INVENTORY-COVERAGE'
    foreach ($item in $inventoryEntries) {
        Add-Result -Id ('CORRECTION-INVENTORY-' + [string]$item.path) -Passed (
            [string]$item.sha256 -ceq (Get-Hash $entryBytes[[string]$item.path]) -and
            [int64]$item.length -eq $entryBytes[[string]$item.path].Length
        )
    }
    $manifestText = [System.Text.UTF8Encoding]::new($false, $true).GetString($entryBytes['MANIFEST.sha256'])
    $manifestLines = @($manifestText.TrimEnd("`r", "`n") -split "`n")
    $expectedManifestNames = @($entryBytes.Keys | Where-Object { $_ -cne 'MANIFEST.sha256' })
    $manifestNames = @($manifestLines | ForEach-Object {
            $match = [regex]::Match($_, '^(?<hash>[0-9a-f]{64})  (?<length>[0-9]+)  (?<path>[A-Za-z0-9][A-Za-z0-9._-]*)$')
            if (-not $match.Success) { throw "Invalid manifest line: $_" }
            $name = $match.Groups['path'].Value
            if (-not $entryBytes.ContainsKey($name) -or $match.Groups['hash'].Value -cne (Get-Hash $entryBytes[$name]) -or [int64]$match.Groups['length'].Value -ne $entryBytes[$name].Length) { throw "Manifest mismatch: $name" }
            $name
        })
    Assert-EqualSet $manifestNames $expectedManifestNames 'CORRECTION-MANIFEST-COVERAGE'

    $status = 'PASS'
}
catch {
    $failureMessage = $_.Exception.Message
}
finally {
    if ($null -ne $context) {
        try { Remove-GenericGitIsolationContext -Context $context }
        catch { if ([string]::IsNullOrWhiteSpace($failureMessage)) { $failureMessage = $_.Exception.Message; $status = 'FAIL' } }
    }
    if ($null -ne $archive) { $archive.Dispose() }
    if ($null -ne $archiveStream) { $archiveStream.Dispose() }
    $result = [pscustomobject][ordered]@{
        Status = $status
        Profile = 'FINDING_CORRECTION'
        TransitionType = 'BUNDLED_CORRECTION_TO_FOCUSED_DELTA_REVIEW'
        ArtifactKind = if (Test-Path -LiteralPath $PackagePath -PathType Container) { 'DIRECTORY' } else { 'ZIP' }
        ArtifactLifecycleState = if ($status -ceq 'PASS') { $lifecycleState } else { 'UNKNOWN' }
        CheckCount = $checks.Count
        PassedCount = @($checks | Where-Object Result -ceq 'PASS').Count
        FailureCount = if ($status -ceq 'PASS') { 0 } else { 1 }
        ReadyToExecute = $status -ceq 'PASS'
        PackageWriteAttemptCount = 0
        IndependentReviewOutcomeValidation = $independentReviewOutcomeValidation
        IndependentReviewOutcomeSHA256 = $independentReviewOutcomeSha256
        ImmediatePreviousReviewPackageSHA256 = $immediatePreviousReviewPackageSha256
        TransitiveFullReviewBaselineSHA256 = $transitiveFullReviewBaselineSha256
        FindingDispositionBindingResult = $findingDispositionBindingResult
        FailureMessage = $failureMessage
    }
    $result | Format-List
}

if ($ReturnInsteadOfExit) {
    $global:LASTEXITCODE = if ($status -ceq 'PASS') { 0 } else { 1 }
    return
}
if ($status -ceq 'PASS') { exit 0 }
exit 1
