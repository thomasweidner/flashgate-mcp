#requires -Version 7.6
[CmdletBinding()]
param(
    [Parameter(Mandatory)][string]$PackagePath,
    [Parameter(Mandatory)][string]$RepositoryRoot,
    [Parameter(Mandatory)][string]$AuthoritativeRepositoryRoot,
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
    $taskComponent = $TaskId.Replace('-', '')
    $reviewIndex = $FindingId.LastIndexOf('-REV-', [System.StringComparison]::Ordinal)
    if ($reviewIndex -le 0) { return $false }
    $prefix = $FindingId.Substring(0, $reviewIndex)
    $components = @([regex]::Matches($prefix, '(?:^|-)(?<task>BL[0-9]{3})(?=-|$)') | ForEach-Object {
            $_.Groups['task'].Value
        })
    return $taskComponent -cin $components
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
        'focused-delta-review-record.json', 'MANIFEST.sha256', 'package-inventory.json',
        'previous-review-binding.json', 'readiness-evidence.json', 'report.md',
        'scope-inventory.json', 'trusted-expected-hashes.json', 'validation-summary.json'
    )
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
    Add-Result -Id 'CORRECTION-REPORT-FOCUSED-VALIDATION' -Passed (
        [string]$reportContract.permanentRegressionEvidence.result -ceq 'PASS' -and
        [string]$reportContract.focusedValidationResult.result -ceq [string]$validationSummary.focusedFixtureResult -and
        [int]$reportContract.focusedValidationResult.selected -eq [int]$validationSummary.focusedFixtureCount -and
        [int]$reportContract.focusedValidationResult.passed -eq [int]$validationSummary.focusedFixtureCount -and
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
        $historicalPath = [System.IO.Path]::GetFullPath([string]$previousState.historicalPackagePath)
        Add-Result -Id 'CORRECTION-HISTORICAL-PACKAGE-EXISTS' -Passed (Test-Path -LiteralPath $historicalPath -PathType Leaf)
        Add-Result -Id 'CORRECTION-HISTORICAL-PACKAGE-HASH' -Passed ((Get-FileHash -LiteralPath $historicalPath -Algorithm SHA256).Hash.ToLowerInvariant() -ceq [string]$previousState.historicalPackageSha256)
        $historicalEntries = @{}
        Add-Type -AssemblyName System.IO.Compression
        $historicalArchive = [System.IO.Compression.ZipFile]::OpenRead($historicalPath)
        try {
            foreach ($name in @('MANIFEST.sha256', 'current-delta.patch', 'scope-inventory.json')) {
                $historicalEntry = @($historicalArchive.Entries | Where-Object FullName -ceq $name)
                if ($historicalEntry.Count -ne 1) { throw "Historical member missing: $name" }
                $memory = [System.IO.MemoryStream]::new()
                try {
                    $stream = $historicalEntry[0].Open()
                    try { $stream.CopyTo($memory) } finally { $stream.Dispose() }
                    $historicalEntries[$name] = $memory.ToArray()
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
        $historicalScope = Read-JsonBytes $historicalEntries['scope-inventory.json'] `
            'historical scope-inventory.json' '' 1
        $historicalEntriesList = @($historicalScope.entries)
        foreach ($historicalScopeEntry in $historicalEntriesList) {
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
        Add-Result -Id 'CORRECTION-HISTORICAL-SCOPE-INTERNAL-PATHCOUNT' -Passed (
            [int]$historicalScope.pathCount -eq $historicalEntriesList.Count
        ) -Evidence "declared=$($historicalScope.pathCount);entries=$($historicalEntriesList.Count);paths=$($historicalCanonicalPaths.Count)"
        Add-Result -Id 'CORRECTION-HISTORICAL-SCOPE-PREVIOUS-COUNT' -Passed (
            [int]$previousState.previousReviewedPathCount -eq [int]$historicalScope.pathCount
        ) -Evidence "previous=$($previousState.previousReviewedPathCount);historical=$($historicalScope.pathCount)"
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
        Assert-EqualSet @($boundPostimages | ForEach-Object path) $currentPaths 'CORRECTION-PREVIOUS-POSTIMAGE-COVERAGE'
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
