#requires -Version 7.6
[CmdletBinding()]
param(
    [string]$ResultPath,
    [switch]$PublicationEvidenceOnly,
    [switch]$SkipPublicationExecutionInputDrift
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$results = [System.Collections.Generic.List[object]]::new()
$status = 'FAIL'
$failureMessage = $null
$fixtureRoot = $null
$utf8 = [System.Text.UTF8Encoding]::new($false)

. (Join-Path $PSScriptRoot 'GenericGovernanceGitEvidence.ps1')

function Add-Case {
    param([string]$Id, [bool]$Passed, [string]$Evidence = '')
    [void]$results.Add([pscustomobject][ordered]@{
            id = $Id
            result = if ($Passed) { 'PASS' } else { 'FAIL' }
            evidence = $Evidence
        })
    if (-not $Passed) { throw ('[{0}] {1}' -f $Id, [string]$Evidence) }
}

function Write-Utf8 {
    param([string]$LiteralPath, [string]$Text)
    [System.IO.File]::WriteAllText($LiteralPath, $Text, $utf8)
}

function Write-Json {
    param([string]$LiteralPath, [object]$Value)
    Write-Utf8 -LiteralPath $LiteralPath -Text (($Value | ConvertTo-Json -Depth 100) + "`n")
}

function Read-Json {
    param([string]$LiteralPath)
    return Get-Content -LiteralPath $LiteralPath -Raw | ConvertFrom-Json -Depth 100
}

function Get-Hash {
    param([string]$LiteralPath)
    return (Get-FileHash -LiteralPath $LiteralPath -Algorithm SHA256).Hash.ToLowerInvariant()
}

function Get-BytesHash {
    param([Parameter(Mandatory)][AllowEmptyCollection()][byte[]]$Bytes)
    return [Convert]::ToHexString([System.Security.Cryptography.SHA256]::HashData($Bytes)).ToLowerInvariant()
}

function Get-StringHash {
    param([Parameter(Mandatory)][string]$Value)
    return Get-BytesHash -Bytes $utf8.GetBytes($Value)
}

function Read-ReportContract {
    param([Parameter(Mandatory)][string]$LiteralPath)
    $text = [System.IO.File]::ReadAllText($LiteralPath, [System.Text.UTF8Encoding]::new($false, $true))
    $beginMarker = '<!-- BEGIN FINDING-CORRECTION-REPORT-CONTRACT -->'
    $endMarker = '<!-- END FINDING-CORRECTION-REPORT-CONTRACT -->'
    $pattern = '(?s)' + [regex]::Escape($beginMarker) + '\r?\n(?<json>.*?)\r?\n' + [regex]::Escape($endMarker)
    $match = [regex]::Match($text, $pattern)
    if (-not $match.Success) { throw "Missing report contract: $LiteralPath" }
    return $match.Groups['json'].Value | ConvertFrom-Json -Depth 100
}

function Write-ReportContract {
    param([Parameter(Mandatory)][string]$LiteralPath, [Parameter(Mandatory)][object]$Contract)
    $text = [System.IO.File]::ReadAllText($LiteralPath, [System.Text.UTF8Encoding]::new($false, $true))
    $beginMarker = '<!-- BEGIN FINDING-CORRECTION-REPORT-CONTRACT -->'
    $endMarker = '<!-- END FINDING-CORRECTION-REPORT-CONTRACT -->'
    $pattern = '(?s)' + [regex]::Escape($beginMarker) + '\r?\n(?<json>.*?)\r?\n' + [regex]::Escape($endMarker)
    if ([regex]::Matches($text, $pattern).Count -ne 1) { throw "Non-canonical report contract: $LiteralPath" }
    $replacement = $beginMarker + "`n" + ($Contract | ConvertTo-Json -Depth 100) + "`n" + $endMarker
    $updated = [regex]::Replace(
        $text,
        $pattern,
        [System.Text.RegularExpressions.MatchEvaluator]{ param($match) $replacement },
        1
    )
    Write-Utf8 -LiteralPath $LiteralPath -Text $updated
}

function Get-ReportSubjectJson {
    param([Parameter(Mandatory)][object]$Contract)
    $copy = ($Contract | ConvertTo-Json -Depth 100) | ConvertFrom-Json -Depth 100
    foreach ($name in @('artifactLifecycleState', 'status', 'readyToExecute', 'classicReviewReady', 'packageWriteAttemptCount', 'nextAction')) {
        $copy.PSObject.Properties.Remove($name)
    }
    return $copy | ConvertTo-Json -Depth 100 -Compress
}

function Invoke-GitText {
    param([string]$Root, [string[]]$Argument)
    $output = @(& git -C $Root @Argument 2>&1)
    if ($LASTEXITCODE -ne 0) {
        throw ('git {0} failed: {1}' -f ([string]::Join(' ', [string[]]$Argument)), (($output | Out-String).Trim()))
    }
    return ($output -join "`n").Trim()
}

function Convert-ScopeEntry {
    param([object]$Entry, [string]$Reason)
    return [ordered]@{
        path = [string]$Entry.Path
        previousPath = if ($null -eq $Entry.PreviousPath) { $null } else { [string]$Entry.PreviousPath }
        gitStatus = [string]$Entry.GitStatus
        tracked = [bool]$Entry.Tracked
        staged = [bool]$Entry.Staged
        mode = if ($null -eq $Entry.Postimage) { $null } else { [string]$Entry.Postimage.mode }
        length = if ($null -eq $Entry.Postimage) { $null } else { [int64]$Entry.Postimage.length }
        sha256 = if ($null -eq $Entry.Postimage) { $null } else { [string]$Entry.Postimage.sha256 }
        inclusionDecision = 'INCLUDE'
        reason = $Reason
    }
}

function Convert-CorrectionEntry {
    param([object]$Entry, [string]$FindingId)
    $pre = $Entry.Preimage
    $post = $Entry.Postimage
    return [ordered]@{
        path = [string]$Entry.Path
        previousPath = if ($null -eq $Entry.PreviousPath) { $null } else { [string]$Entry.PreviousPath }
        gitStatus = [string]$Entry.GitStatus
        tracked = [bool]$Entry.Tracked
        staged = [bool]$Entry.Staged
        previousReviewedSha256 = if ($null -eq $pre) { $null } else { [string]$pre.sha256 }
        currentCorrectedSha256 = if ($null -eq $post) { $null } else { [string]$post.sha256 }
        previousReviewedLength = if ($null -eq $pre) { $null } else { [int64]$pre.length }
        currentCorrectedLength = if ($null -eq $post) { $null } else { [int64]$post.length }
        previousReviewedMode = if ($null -eq $pre) { $null } else { [string]$pre.mode }
        currentCorrectedMode = if ($null -eq $post) { $null } else { [string]$post.mode }
        correctionClassification = if ($null -eq $pre) { 'ADDED_AFTER_REVIEW' } elseif ($null -eq $post) { 'REMOVED_AFTER_REVIEW' } else { 'MODIFIED_AFTER_REVIEW' }
        findingIds = @($FindingId)
        correctionReason = 'Synthetic productive finding-correction regression.'
    }
}

function Get-TreePostimage {
    param([string]$Root, [string]$Tree, [string]$Path)
    $entry = Invoke-GenericGitBytes -Root $Root -Argument @('ls-tree', '-z', $Tree, '--', $Path) -RepositoryPaths
    if ($entry.Bytes.Length -eq 0) { return [ordered]@{ path = $Path; presence = 'ABSENT' } }
    $text = ConvertFrom-GenericStrictUtf8 -Bytes $entry.Bytes[0..($entry.Bytes.Length - 2)] -Label "tree $Path"
    $match = [regex]::Match($text, '^(?<mode>[0-7]{6}) blob (?<oid>[0-9a-f]{40})\t(?<path>.+)$')
    if (-not $match.Success -or $match.Groups['path'].Value -cne $Path) { throw "Invalid tree postimage: $Path" }
    $blob = Invoke-GenericGitBytes -Root $Root -Argument @('cat-file', 'blob', $match.Groups['oid'].Value)
    return [ordered]@{ path = $Path; presence = 'PRESENT'; mode = $match.Groups['mode'].Value; length = [int64]$blob.Bytes.Length; sha256 = Get-BytesHash $blob.Bytes }
}

function Get-AppliedTree {
    param([string]$Root, [string]$Baseline, [byte[]]$PatchBytes)
    $context = New-GenericGitIsolationContext -Root $Root
    try {
        $null = Invoke-GenericGitBytes -Root $Root -Argument @('read-tree', $Baseline) -Environment $context.Environment
        $null = Invoke-GenericGitBytes -Root $Root -Argument @('apply', '--cached', '--binary', '--whitespace=nowarn', '-') -Environment $context.Environment -InputBytes $PatchBytes
        return (ConvertFrom-GenericStrictUtf8 -Bytes (Invoke-GenericGitBytes -Root $Root -Argument @('write-tree') -Environment $context.Environment).Bytes -Label 'tree').Trim()
    }
    finally { Remove-GenericGitIsolationContext -Context $context }
}

function New-ZipFromDirectory {
    param([string]$Directory, [string]$ZipPath)
    Add-Type -AssemblyName System.IO.Compression
    $stream = [System.IO.FileStream]::new($ZipPath, [System.IO.FileMode]::CreateNew, [System.IO.FileAccess]::Write, [System.IO.FileShare]::None)
    try {
        $archive = [System.IO.Compression.ZipArchive]::new($stream, [System.IO.Compression.ZipArchiveMode]::Create, $false)
        try {
            foreach ($file in @(Get-ChildItem -LiteralPath $Directory -File | Sort-Object Name)) {
                $entry = $archive.CreateEntry($file.Name, [System.IO.Compression.CompressionLevel]::Optimal)
                $entry.LastWriteTime = [datetimeoffset]::new(2000, 1, 1, 0, 0, 0, [timespan]::Zero)
                $entryStream = $entry.Open()
                try { $bytes = [System.IO.File]::ReadAllBytes($file.FullName); $entryStream.Write($bytes, 0, $bytes.Length) }
                finally { $entryStream.Dispose() }
            }
        }
        finally { $archive.Dispose() }
    }
    finally { $stream.Dispose() }
}

function New-HistoricalPackageVariant {
    param([string]$Name, [string]$HistoricalPatchPath, [object]$Scope)
    $directory = Join-Path $fixtureRoot ('historical-' + $Name)
    [void][System.IO.Directory]::CreateDirectory($directory)
    [System.IO.File]::Copy($HistoricalPatchPath, (Join-Path $directory 'current-delta.patch'), $false)
    Write-Json (Join-Path $directory 'scope-inventory.json') $Scope
    $manifestLines = @('current-delta.patch','scope-inventory.json' | ForEach-Object {
            $file = Get-Item (Join-Path $directory $_)
            "$(Get-Hash $file.FullName)  $($file.Length)  $_"
        })
    Write-Utf8 (Join-Path $directory 'MANIFEST.sha256') (($manifestLines -join "`n") + "`n")
    $zip = Join-Path $fixtureRoot ('historical-' + $Name + '.zip')
    New-ZipFromDirectory $directory $zip
    return [pscustomobject]@{
        Directory = $directory
        Zip = $zip
        PackageSha256 = Get-Hash $zip
        ManifestSha256 = Get-Hash (Join-Path $directory 'MANIFEST.sha256')
        PatchSha256 = Get-Hash (Join-Path $directory 'current-delta.patch')
        ScopeSha256 = Get-Hash (Join-Path $directory 'scope-inventory.json')
    }
}

function New-GenericHistoricalPackageVariant {
    param(
        [string]$Name,
        [string]$HistoricalPatchPath,
        [object]$Scope,
        [string]$BaselineCommit,
        [string]$CurrentCommit,
        [scriptblock]$Mutate
    )

    $directory = Join-Path $fixtureRoot ('generic-historical-' + $Name)
    [void][System.IO.Directory]::CreateDirectory($directory)
    [System.IO.File]::Copy($HistoricalPatchPath, (Join-Path $directory 'current-delta.patch'), $false)
    [System.IO.File]::Copy($HistoricalPatchPath, (Join-Path $directory 'task.patch'), $false)
    Write-Json (Join-Path $directory 'scope-inventory.json') $Scope
    Write-Utf8 (Join-Path $directory 'HANDOFF.md') "# Synthetic generic implementation review handoff`n"
    Write-Json (Join-Path $directory 'pre-review-validation-evidence.json') ([ordered]@{ schemaVersion=1; status='PASS' })
    Write-Utf8 (Join-Path $directory 'report.md') "# Synthetic generic implementation review report`n"
    Write-Json (Join-Path $directory 'validation-summary.json') ([ordered]@{ schemaVersion=1; status='PASS' })

    $scopeHash = Get-Hash (Join-Path $directory 'scope-inventory.json')
    $patchHash = Get-Hash (Join-Path $directory 'current-delta.patch')
    $currentStateGate = [ordered]@{
        result='PASS'; repositoryIdentityBound=$true; commitAndBranchBound=$true
        completeStatusBound=$true; scopeAndIdsBound=$true; parallelWorktreesBound=$true
    }
    $allowedPaths = @($Scope.allowedDeltaPaths)
    $excludedPaths = @($Scope.excludedDeltaPaths)
    Write-Json (Join-Path $directory 'assignment-record.json') ([ordered]@{
            schemaVersion=1; taskId='BL-339'; repository='https://github.com/thomasweidner/flashgate-mcp.git'
            baselineCommit=$BaselineCommit; currentCommit=$CurrentCommit; branch='fixture'
            executionMode='BUNDLED_CORRECTION'; checkpoint='SPRINT_CLOSE'
            profile='IMPLEMENTATION_TO_INDEPENDENT_FULL_REVIEW'
            transitionType='IMPLEMENTATION_TO_INDEPENDENT_FULL_REVIEW'
            changeTriggerReviewResult='EXISTING_GATES_REQUIRED'; currentStateGate=$currentStateGate
            classicReviewReady=$true; findingIds=@(); commitAuthorized=$false
            scopeInventorySha256=$scopeHash; taskPatchSha256=$patchHash; currentDeltaSha256=$patchHash
            allowedDeltaPaths=$allowedPaths; excludedDeltaPaths=$excludedPaths
            fullCompletionEvidenceSha256=('a'*64); fullCompletionResultSha256=('b'*64)
            executionEnvelopeSha256=('c'*64)
        })
    Write-Json (Join-Path $directory 'completion-report.json') ([ordered]@{
            schemaVersion=1; taskId='BL-339'; repository='https://github.com/thomasweidner/flashgate-mcp.git'
            baselineCommit=$BaselineCommit; currentCommit=$CurrentCommit; branch='fixture'
            profile='IMPLEMENTATION_TO_INDEPENDENT_FULL_REVIEW'
            transitionType='IMPLEMENTATION_TO_INDEPENDENT_FULL_REVIEW'
            status='CLASSIC_REVIEW_READY'; currentStateGate=$currentStateGate
            classicReviewReady=$true; findingIds=@(); commitAuthorized=$false
            materialCorrectionCycleCount=0; validationExecutionCount=1
            infrastructureOrInvocationFailureCount=0; observedWarningCount=0
            resolvedWarningCount=0; openWarningCount=0; warningCount=0; failureCount=0
            zipFreeReadinessPassed=$true
            packageGeneration=[ordered]@{ freshStaging=$true; finalZipWriteCount=1; inPlaceRepairPerformed=$false }
            scopeInventorySha256=$scopeHash; taskPatchSha256=$patchHash; currentDeltaSha256=$patchHash
            allowedDeltaPaths=$allowedPaths; excludedDeltaPaths=$excludedPaths
            nextAction='INDEPENDENT_FULL_REVIEW'; independentReviewStatus='NOT_PERFORMED'
            fullCompletionEvidenceSha256=('a'*64); fullCompletionResultSha256=('b'*64)
            executionEnvelopeSha256=('c'*64)
        })

    if ($null -ne $Mutate) {
        & $Mutate $directory
    }

    $assignment = Read-Json (Join-Path $directory 'assignment-record.json')
    $inventoryEntries = @(Get-ChildItem -LiteralPath $directory -File |
        Where-Object Name -notin @('MANIFEST.sha256', 'package-inventory.json') |
        Sort-Object Name | ForEach-Object {
            [ordered]@{ path=$_.Name; sha256=Get-Hash $_.FullName; length=[int64]$_.Length }
        })
    Write-Json (Join-Path $directory 'package-inventory.json') ([ordered]@{
            schemaVersion=1; taskId=[string]$assignment.taskId
            profile=[string]$assignment.profile; transitionType=[string]$assignment.transitionType
            entries=$inventoryEntries
        })
    [string[]]$manifestNames = @(Get-ChildItem -LiteralPath $directory -File |
        Where-Object Name -cne 'MANIFEST.sha256' | ForEach-Object Name)
    [array]::Sort($manifestNames, [System.StringComparer]::Ordinal)
    $manifestLines = @($manifestNames | ForEach-Object {
            $file = Get-Item -LiteralPath (Join-Path $directory $_)
            "$(Get-Hash $file.FullName)  $($file.Length)  $_"
        })
    Write-Utf8 (Join-Path $directory 'MANIFEST.sha256') (($manifestLines -join "`n") + "`n")

    $zip = Join-Path $fixtureRoot ('generic-historical-' + $Name + '.zip')
    New-ZipFromDirectory $directory $zip
    return [pscustomobject]@{
        Directory = $directory
        Zip = $zip
        PackageSha256 = Get-Hash $zip
        ManifestSha256 = Get-Hash (Join-Path $directory 'MANIFEST.sha256')
        PatchSha256 = Get-Hash (Join-Path $directory 'current-delta.patch')
        ScopeSha256 = Get-Hash (Join-Path $directory 'scope-inventory.json')
    }
}

function Update-PackageMetadata {
    param([string]$Directory)
    $inventoryEntries = @(Get-ChildItem -LiteralPath $Directory -File | Where-Object Name -notin @('MANIFEST.sha256', 'package-inventory.json') | Sort-Object Name | ForEach-Object {
            [ordered]@{ path = $_.Name; sha256 = Get-Hash $_.FullName; length = [int64]$_.Length }
        })
    Write-Json (Join-Path $Directory 'package-inventory.json') ([ordered]@{ schemaVersion = 1; taskId = (Read-Json (Join-Path $Directory 'assignment-record.json')).taskId; profile = 'FINDING_CORRECTION'; transitionType = 'BUNDLED_CORRECTION_TO_FOCUSED_DELTA_REVIEW'; entries = $inventoryEntries })
    [string[]]$names = @(Get-ChildItem -LiteralPath $Directory -File | Where-Object Name -cne 'MANIFEST.sha256' | ForEach-Object Name)
    [array]::Sort($names, [System.StringComparer]::Ordinal)
    $lines = @($names | ForEach-Object { $file = Get-Item -LiteralPath (Join-Path $Directory $_); "$(Get-Hash $file.FullName)  $($file.Length)  $($_)" })
    Write-Utf8 (Join-Path $Directory 'MANIFEST.sha256') (($lines -join "`n") + "`n")
}

function Sync-FocusedValidationSourceBinding {
    param(
        [Parameter(Mandatory)][string]$Directory,
        [switch]$Counts
    )
    $sourcePath = Join-Path $Directory 'focused-validation-result.json'
    $sourceItem = Get-Item -LiteralPath $sourcePath
    $sourceHash = Get-Hash $sourcePath
    $source = Read-Json $sourcePath
    $regression = Read-Json (Join-Path $Directory 'finding-regression-matrix.json')
    $regression.finalFocusedValidationEvidence.sourceEvidenceSha256 = $sourceHash
    $regression.finalFocusedValidationEvidence.sourceEvidenceLength = [int64]$sourceItem.Length
    if ($Counts) {
        $regression.finalFocusedValidationEvidence.status = [string]$source.status
        $regression.finalFocusedValidationEvidence.selected = [int]$source.selected
        $regression.finalFocusedValidationEvidence.passed = [int]$source.passed
        $regression.finalFocusedValidationEvidence.failed = [int]$source.failed
    }
    Write-Json (Join-Path $Directory 'finding-regression-matrix.json') $regression
    $summary = Read-Json (Join-Path $Directory 'validation-summary.json')
    $summary.focusedFixtureEvidenceSha256 = $sourceHash
    $summary.focusedFixtureEvidenceLength = [int64]$sourceItem.Length
    if ($Counts) {
        $summary.focusedFixtureCount = [int]$source.selected
        $summary.focusedFixtureSelectedCount = [int]$source.selected
        $summary.focusedFixturePassedCount = [int]$source.passed
    }
    Write-Json (Join-Path $Directory 'validation-summary.json') $summary
}

function New-IndependentReviewOutcome {
    param(
        [Parameter(Mandatory)][string]$LiteralPath,
        [Parameter(Mandatory)][string]$TaskId,
        [Parameter(Mandatory)][string]$PreviousReviewPackageSha256,
        [Parameter(Mandatory)][string]$TransitiveFullReviewBaselineSha256,
        [Parameter(Mandatory)][string]$ReviewedCurrentDeltaSha256,
        [Parameter(Mandatory)][string[]]$FindingIds,
        [Parameter(Mandatory)][string[]]$OpenFindingIds
    )
    $openSet = [System.Collections.Generic.HashSet[string]]::new(
        $OpenFindingIds, [System.StringComparer]::Ordinal
    )
    $closedFindingIds = @($FindingIds | Where-Object { -not $openSet.Contains($_) })
    $findingOutcomes = @($FindingIds | ForEach-Object {
            if ($openSet.Contains($_)) {
                [ordered]@{
                    id = $_
                    disposition = 'OPEN_INCOMPLETE_CORRECTION'
                    reviewFinding = 'Synthetic focused-to-focused correction finding.'
                }
            }
            else {
                [ordered]@{ id = $_; disposition = 'CLOSED_BY_INDEPENDENT_DELTA_REVIEW' }
            }
        })
    $taskComponent = $TaskId.Replace('-', '')
    $nextFinding = $OpenFindingIds[0].Replace('-', '_')
    Write-Json $LiteralPath ([ordered]@{
            schemaVersion = 1
            artifactType = 'INDEPENDENT_FOCUSED_DELTA_REVIEW_OUTCOME'
            taskId = $TaskId
            reviewMode = 'FOCUSED_INDEPENDENT_DELTA_REVIEW'
            reviewerRole = 'INDEPENDENT_REVIEWER'
            previousReviewPackageSha256 = $PreviousReviewPackageSha256.ToUpperInvariant()
            transitiveFullReviewBaselineSha256 = $TransitiveFullReviewBaselineSha256.ToUpperInvariant()
            reviewedCurrentDeltaSha256 = $ReviewedCurrentDeltaSha256.ToUpperInvariant()
            reviewResult = 'FAIL_WITH_FINDING'
            findingOutcomes = $findingOutcomes
            directInterfaceOutcomes = @([ordered]@{ id = "$taskComponent-CORR-001"; disposition = 'CLOSED' })
            openFindingIds = $OpenFindingIds
            closedFindingIds = $closedFindingIds
            nextAction = "CORRECT_$nextFinding`_AND_REQUEST_FOCUSED_INDEPENDENT_DELTA_REVIEW"
        })
    return (Get-Hash $LiteralPath).ToUpperInvariant()
}

function New-GeneralizedIndependentReviewOutcome {
    param(
        [Parameter(Mandatory)][string]$LiteralPath,
        [Parameter(Mandatory)][string]$TaskId,
        [Parameter(Mandatory)][string]$ReviewedPackageSha256,
        [Parameter(Mandatory)][string]$ImmediatePreviousReviewPackageSha256,
        [string]$PreviousIndependentReviewOutcomeSha256,
        [Parameter(Mandatory)][string]$TransitiveFullReviewBaselineSha256,
        [Parameter(Mandatory)][string]$ReviewedCurrentDeltaSha256,
        [Parameter(Mandatory)][string]$ReviewedCorrectionPatchSha256,
        [Parameter(Mandatory)][string[]]$TargetFindingIds,
        [string[]]$OpenTargetFindingIds = @(),
        [string[]]$NewFindingIds = @(),
        [string[]]$InheritedClosedFindingIds = @(),
        [object[]]$DirectInterfaceOutcomes = @(),
        [ValidateSet('PASS', 'FAIL_WITH_FINDINGS')][string]$ReviewResult = 'FAIL_WITH_FINDINGS'
    )
    $openTargetSet = [System.Collections.Generic.HashSet[string]]::new(
        $OpenTargetFindingIds, [System.StringComparer]::Ordinal
    )
    $targetOutcomes = @($TargetFindingIds | ForEach-Object {
            if ($openTargetSet.Contains($_)) {
                [ordered]@{
                    id = $_
                    disposition = 'OPEN_INCOMPLETE_CORRECTION'
                    reviewFinding = 'Synthetic incomplete focused correction.'
                }
            }
            else {
                [ordered]@{ id = $_; disposition = 'CLOSED_BY_INDEPENDENT_DELTA_REVIEW' }
            }
        })
    $newFindings = @($NewFindingIds | ForEach-Object {
            [ordered]@{
                id = $_
                severity = 'MAJOR'
                disposition = 'OPEN'
                summary = 'Synthetic independently discovered finding.'
                evidence = 'Synthetic immutable reviewer evidence.'
                requiredCorrection = 'Correct and request focused independent delta review.'
            }
        })
    $openFindingIds = @($OpenTargetFindingIds + $NewFindingIds)
    $closedFindingIds = @($InheritedClosedFindingIds + @(
            $TargetFindingIds | Where-Object { -not $openTargetSet.Contains($_) }
        ))
    $contract = [ordered]@{
        schemaVersion = 1
        artifactType = 'INDEPENDENT_FOCUSED_DELTA_REVIEW_OUTCOME'
        taskId = $TaskId
        reviewMode = 'FOCUSED_INDEPENDENT_DELTA_REVIEW'
        reviewerRole = 'INDEPENDENT_REVIEWER'
        reviewedPackageSha256 = $ReviewedPackageSha256.ToUpperInvariant()
        immediatePreviousReviewPackageSha256 = $ImmediatePreviousReviewPackageSha256.ToUpperInvariant()
        transitiveFullReviewBaselineSha256 = $TransitiveFullReviewBaselineSha256.ToUpperInvariant()
        reviewedCurrentDeltaSha256 = $ReviewedCurrentDeltaSha256.ToUpperInvariant()
        reviewedCorrectionPatchSha256 = $ReviewedCorrectionPatchSha256.ToUpperInvariant()
        packageIntegrityResult = 'PASS'
        reviewResult = $ReviewResult
        targetFindingOutcomes = $targetOutcomes
        newFindings = $newFindings
        inheritedClosedFindingIds = $InheritedClosedFindingIds
        directInterfaceOutcomes = $DirectInterfaceOutcomes
        closedFindingIds = $closedFindingIds
        openFindingIds = $openFindingIds
        nextAction = if ($ReviewResult -ceq 'PASS') {
            'NO_FURTHER_CORRECTION_REQUIRED'
        }
        else { 'CORRECT_OPEN_FINDINGS_AND_REQUEST_FOCUSED_INDEPENDENT_DELTA_REVIEW' }
    }
    if (-not [string]::IsNullOrWhiteSpace($PreviousIndependentReviewOutcomeSha256)) {
        $ordered = [ordered]@{}
        foreach ($key in @($contract.Keys)) {
            $ordered[$key] = $contract[$key]
            if ($key -ceq 'immediatePreviousReviewPackageSha256') {
                $ordered.previousIndependentReviewOutcomeSha256 = `
                    $PreviousIndependentReviewOutcomeSha256.ToUpperInvariant()
            }
        }
        $contract = $ordered
    }
    Write-Json $LiteralPath $contract
    return (Get-Hash $LiteralPath).ToUpperInvariant()
}

function Test-IndependentReviewOutcomeSchema {
    param([Parameter(Mandatory)][string]$LiteralPath)
    return Test-Json -Json (Get-Content -LiteralPath $LiteralPath -Raw -Encoding utf8) `
        -SchemaFile (Join-Path $PSScriptRoot '../Governance/generic-independent-review-evidence.schema.json') `
        -ErrorAction Stop
}

function Add-IndependentReviewOutcomeToSource {
    param(
        [Parameter(Mandatory)][string]$Directory,
        [Parameter(Mandatory)][string]$OutcomePath,
        [Parameter(Mandatory)][string]$OutcomeSha256
    )
    [System.IO.File]::Copy(
        $OutcomePath,
        (Join-Path $Directory 'previous-independent-review-outcome.json'),
        $false
    )
    $handoffPath = Join-Path $Directory 'HANDOFF.md'
    if (-not (Test-Path -LiteralPath $handoffPath -PathType Leaf)) {
        $completion = Read-Json (Join-Path $Directory 'completion-report.json')
        $handoffContract = [ordered]@{
            schemaVersion = 2
            taskId = [string]$completion.taskId
            profile = [string]$completion.profile
            transitionType = [string]$completion.transitionType
            artifactLifecycleState = [string]$completion.artifactLifecycleState
            status = [string]$completion.status
            readyToExecute = [bool]$completion.readyToExecute
            classicReviewReady = [bool]$completion.classicReviewReady
            findingIds = @($completion.findingIds)
            reviewStatus = [string]$completion.reviewStatus
            commitAuthorized = $false
            packageWriteAttemptCount = [int]$completion.packageWriteAttemptCount
            nextAction = [string]$completion.nextAction
        }
        $handoffLines = @(
            "# $($completion.taskId) finding-correction handoff",
            '',
            '<!-- BEGIN GOVERNANCE-HANDOFF-STATUS -->',
            "TaskId: $($completion.taskId)",
            "TransitionType: $($completion.transitionType)",
            "Profile: $($completion.profile)",
            "ArtifactLifecycleState: $($completion.artifactLifecycleState)",
            "Status: $($completion.status)",
            "ReadyToExecute: $(([string][bool]$completion.readyToExecute).ToLowerInvariant())",
            "ClassicReviewReady: $(([string][bool]$completion.classicReviewReady).ToLowerInvariant())",
            "FindingCount: $(@($completion.findingIds).Count)",
            "ReviewStatus: $($completion.reviewStatus)",
            "PackageWriteAttemptCount: $([int]$completion.packageWriteAttemptCount)",
            'CommitAuthorized: false',
            "NextAction: $($completion.nextAction)",
            '<!-- END GOVERNANCE-HANDOFF-STATUS -->',
            '',
            '<!-- BEGIN GOVERNANCE-HANDOFF-CONTRACT -->',
            ($handoffContract | ConvertTo-Json -Depth 20),
            '<!-- END GOVERNANCE-HANDOFF-CONTRACT -->',
            ''
        )
        Write-Utf8 $handoffPath ($handoffLines -join "`n")
    }
    foreach ($name in @('assignment-record.json', 'completion-report.json', 'focused-delta-review-record.json')) {
        $contract = Read-Json (Join-Path $Directory $name)
        $contract | Add-Member -NotePropertyName previousIndependentReviewOutcomeSha256 `
            -NotePropertyValue $OutcomeSha256 -Force
        Write-Json (Join-Path $Directory $name) $contract
    }
    Update-PackageMetadata $Directory
}

function Set-SingleSourceFindingId {
    param(
        [Parameter(Mandatory)][string]$Directory,
        [Parameter(Mandatory)][string]$FindingId
    )
    foreach ($name in @('completion-report.json', 'readiness-evidence.json')) {
        $contract = Read-Json (Join-Path $Directory $name)
        $contract.findingIds = @($FindingId)
        Write-Json (Join-Path $Directory $name) $contract
    }
    $correction = Read-Json (Join-Path $Directory 'finding-correction-matrix.json')
    $correction.findings[0].id = $FindingId
    Write-Json (Join-Path $Directory 'finding-correction-matrix.json') $correction
    $regression = Read-Json (Join-Path $Directory 'finding-regression-matrix.json')
    $regression.findings[0].id = $FindingId
    Write-Json (Join-Path $Directory 'finding-regression-matrix.json') $regression
    $ledger = Read-Json (Join-Path $Directory 'finding-ledger.json')
    $ledger.findings[0].id = $FindingId
    Write-Json (Join-Path $Directory 'finding-ledger.json') $ledger
    $scope = Read-Json (Join-Path $Directory 'correction-scope-inventory.json')
    foreach ($entry in @($scope.entries)) { $entry.findingIds = @($FindingId) }
    Write-Json (Join-Path $Directory 'correction-scope-inventory.json') $scope
    $focused = Read-Json (Join-Path $Directory 'focused-delta-review-record.json')
    $focused.reviewedFindingIds = @($FindingId)
    Write-Json (Join-Path $Directory 'focused-delta-review-record.json') $focused
    $report = Read-ReportContract (Join-Path $Directory 'report.md')
    $report.findingIds = @($FindingId)
    $report.findingDispositions[0].id = $FindingId
    Write-ReportContract (Join-Path $Directory 'report.md') $report
    $assignment = Read-Json (Join-Path $Directory 'assignment-record.json')
    $assignment.findingIds = @($FindingId)
    $assignment.findingLedgerSha256 = Get-Hash (Join-Path $Directory 'finding-ledger.json')
    $assignment.correctionScopeInventorySha256 = Get-Hash (
        Join-Path $Directory 'correction-scope-inventory.json'
    )
    Write-Json (Join-Path $Directory 'assignment-record.json') $assignment
    Update-PackageMetadata $Directory
}

function Set-HistoricalPackageBinding {
    param(
        [Parameter(Mandatory)][string]$Directory,
        [Parameter(Mandatory)][string]$PackagePath,
        [Parameter(Mandatory)][string]$PackageContentDirectory
    )
    $packageHash = Get-Hash $PackagePath
    $binding = Read-Json (Join-Path $Directory 'previous-review-binding.json')
    $binding.previousReviewState.historicalPackagePath = $PackagePath
    $binding.previousReviewState.historicalPackageSha256 = $packageHash
    $binding.previousReviewState.historicalManifestSha256 = Get-Hash (
        Join-Path $PackageContentDirectory 'MANIFEST.sha256'
    )
    $binding.previousReviewState.historicalPatchSha256 = Get-Hash (
        Join-Path $PackageContentDirectory 'current-delta.patch'
    )
    $binding.previousReviewState.historicalScopeInventorySha256 = Get-Hash (
        Join-Path $PackageContentDirectory 'scope-inventory.json'
    )
    Write-Json (Join-Path $Directory 'previous-review-binding.json') $binding

    $focused = Read-Json (Join-Path $Directory 'focused-delta-review-record.json')
    $focused.previousReviewPackage = $PackagePath
    $focused.previousReviewSha256 = $packageHash
    $focused.previousReviewState = $binding.previousReviewState
    Write-Json (Join-Path $Directory 'focused-delta-review-record.json') $focused

    $assignment = Read-Json (Join-Path $Directory 'assignment-record.json')
    $assignment.previousReviewBindingSha256 = Get-Hash (
        Join-Path $Directory 'previous-review-binding.json'
    )
    Write-Json (Join-Path $Directory 'assignment-record.json') $assignment
    Update-PackageMetadata $Directory
}

function Copy-Artifact {
    param([string]$Source, [string]$Name)
    $target = Join-Path $fixtureRoot $Name
    [void][System.IO.Directory]::CreateDirectory($target)
    Copy-Item -Path (Join-Path $Source '*') -Destination $target -Recurse -Force
    return $target
}

function Set-PublicationEvidenceHashBindings {
    param([string]$Directory, [string]$Sha256)
    $ledger = Read-Json (Join-Path $Directory 'finding-ledger.json')
    $ledger.findings[0].publicationRegressionEvidence.sha256 = $Sha256
    Write-Json (Join-Path $Directory 'finding-ledger.json') $ledger
    $regression = Read-Json (Join-Path $Directory 'finding-regression-matrix.json')
    $regression.findings[0].publicationRegressionEvidence.sha256 = $Sha256
    Write-Json (Join-Path $Directory 'finding-regression-matrix.json') $regression
    $focused = Read-Json (Join-Path $Directory 'focused-delta-review-record.json')
    $focused.publicationRegressionEvidence[0].sha256 = $Sha256
    Write-Json (Join-Path $Directory 'focused-delta-review-record.json') $focused
    $report = Read-ReportContract (Join-Path $Directory 'report.md')
    $report.publicationRegressionEvidence[0].sha256 = $Sha256
    Write-ReportContract (Join-Path $Directory 'report.md') $report
    $assignment = Read-Json (Join-Path $Directory 'assignment-record.json')
    $assignment.findingLedgerSha256 = Get-Hash (Join-Path $Directory 'finding-ledger.json')
    Write-Json (Join-Path $Directory 'assignment-record.json') $assignment
}

function Sync-PublicationResultBinding {
    param([string]$Directory)
    $evidence = Read-Json (Join-Path $Directory 'publication-regression-evidence.json')
    $evidence.resultSha256 = Get-Hash (Join-Path $Directory 'publication-regression-result.json')
    Write-Json (Join-Path $Directory 'publication-regression-evidence.json') $evidence
    Set-PublicationEvidenceHashBindings $Directory (Get-Hash (Join-Path $Directory 'publication-regression-evidence.json'))
}

function Set-AllPublicationCaseIds {
    param([string]$Directory, [string[]]$CaseIds)
    $evidence = Read-Json (Join-Path $Directory 'publication-regression-evidence.json')
    $evidence.findingAssignments[0].caseIds = $CaseIds
    Write-Json (Join-Path $Directory 'publication-regression-evidence.json') $evidence
    $ledger = Read-Json (Join-Path $Directory 'finding-ledger.json')
    $ledger.findings[0].publicationRegressionEvidence.caseIds = $CaseIds
    Write-Json (Join-Path $Directory 'finding-ledger.json') $ledger
    $regression = Read-Json (Join-Path $Directory 'finding-regression-matrix.json')
    $regression.findings[0].publicationRegressionEvidence.caseIds = $CaseIds
    Write-Json (Join-Path $Directory 'finding-regression-matrix.json') $regression
    $focused = Read-Json (Join-Path $Directory 'focused-delta-review-record.json')
    $focused.publicationRegressionEvidence[0].caseIds = $CaseIds
    Write-Json (Join-Path $Directory 'focused-delta-review-record.json') $focused
    $report = Read-ReportContract (Join-Path $Directory 'report.md')
    $report.publicationRegressionEvidence[0].caseIds = $CaseIds
    Write-ReportContract (Join-Path $Directory 'report.md') $report
    Set-PublicationEvidenceHashBindings $Directory (Get-Hash (Join-Path $Directory 'publication-regression-evidence.json'))
}

function Add-PublicationEvidenceToSource {
    param([string]$Directory, [string]$FindingId)
    $productRoot = Split-Path -Parent $PSScriptRoot
    $catalogPath = Join-Path $productRoot 'Governance/publication-regression-matrix-catalog.json'
    $catalog = Read-Json $catalogPath
    $matrix = @($catalog.matrices | Where-Object matrixId -CEQ 'GOVERNANCE_HANDOFF_PUBLICATION_FIXTURES')[0]
    $caseIds = @($matrix.requiredCaseIds)
    $sourceBindings = @($matrix.sourcePaths | ForEach-Object {
            [ordered]@{ path = $_; sha256 = Get-Hash (Join-Path $productRoot $_) }
        })
    $dependencyBindings = @($matrix.dependencyPaths | ForEach-Object {
            [ordered]@{ path = $_; sha256 = Get-Hash (Join-Path $productRoot $_) }
        })
    $result = [ordered]@{
        schemaVersion = 2; matrixId = [string]$matrix.matrixId
        executionInputBinding = [ordered]@{
            matrixDefinitionArtifact = 'Governance/publication-regression-matrix-catalog.json'
            matrixDefinitionSha256 = Get-Hash $catalogPath
            sourceBindings = $sourceBindings
            dependencyBindings = $dependencyBindings
        }
        status = 'PASS'
        selected = $caseIds.Count; passed = $caseIds.Count; failed = 0
        results = @($caseIds | ForEach-Object {
                [ordered]@{ id = $_; result = 'PASS'; evidence = 'Synthetic product-contract fixture; not BL-339 execution evidence.' }
            })
        failureMessage = $null
    }
    $resultPath = Join-Path $Directory 'publication-regression-result.json'
    Write-Json $resultPath $result
    $evidence = [ordered]@{
        schemaVersion = 2; taskId = 'BL-339'; matrixId = [string]$matrix.matrixId
        resultArtifact = 'publication-regression-result.json'; resultSha256 = Get-Hash $resultPath
        matrixDefinitionArtifact = 'Governance/publication-regression-matrix-catalog.json'
        matrixDefinitionSha256 = Get-Hash $catalogPath
        findingAssignments = @([ordered]@{ findingId = $FindingId; caseIds = $caseIds })
        sourceBindings = $sourceBindings
        dependencyBindings = $dependencyBindings
    }
    $evidencePath = Join-Path $Directory 'publication-regression-evidence.json'
    Write-Json $evidencePath $evidence
    $binding = [pscustomobject][ordered]@{ artifact='publication-regression-evidence.json'; sha256=Get-Hash($evidencePath); matrixId=$evidence.matrixId; caseIds=$caseIds }
    $ledger = Read-Json (Join-Path $Directory 'finding-ledger.json')
    $ledger.findings[0] | Add-Member -NotePropertyName publicationRegressionEvidence -NotePropertyValue $binding
    Write-Json (Join-Path $Directory 'finding-ledger.json') $ledger
    $regression = Read-Json (Join-Path $Directory 'finding-regression-matrix.json')
    $regression.findings[0] | Add-Member -NotePropertyName publicationRegressionEvidence -NotePropertyValue $binding
    Write-Json (Join-Path $Directory 'finding-regression-matrix.json') $regression
    $topBinding = [pscustomobject][ordered]@{ findingId=$FindingId; artifact=$binding.artifact; sha256=$binding.sha256; matrixId=$binding.matrixId; caseIds=$caseIds }
    $focused = Read-Json (Join-Path $Directory 'focused-delta-review-record.json')
    $focused | Add-Member -NotePropertyName publicationRegressionEvidence -NotePropertyValue @($topBinding)
    Write-Json (Join-Path $Directory 'focused-delta-review-record.json') $focused
    $report = Read-ReportContract (Join-Path $Directory 'report.md')
    $report | Add-Member -NotePropertyName publicationRegressionEvidence -NotePropertyValue @($topBinding)
    Write-ReportContract (Join-Path $Directory 'report.md') $report
    $assignment = Read-Json (Join-Path $Directory 'assignment-record.json')
    $assignment.findingLedgerSha256 = Get-Hash (Join-Path $Directory 'finding-ledger.json')
    Write-Json (Join-Path $Directory 'assignment-record.json') $assignment
}

function Invoke-ProductValidator {
    param(
        [string]$Artifact,
        [string]$RepositoryRoot,
        [string]$ContractRoot = (Split-Path -Parent $PSScriptRoot),
        [string]$IndependentReviewOutcomePath,
        [string]$ExpectedIndependentReviewOutcomeSha256
    )
    $parameters = @{
        PackagePath = $Artifact
        RepositoryRoot = $ContractRoot
        AuthoritativeRepositoryRoot = $RepositoryRoot
        ReturnInsteadOfExit = $true
    }
    if (-not [string]::IsNullOrWhiteSpace($IndependentReviewOutcomePath)) {
        $parameters.IndependentReviewOutcomePath = $IndependentReviewOutcomePath
    }
    if (-not [string]::IsNullOrWhiteSpace($ExpectedIndependentReviewOutcomeSha256)) {
        $parameters.ExpectedIndependentReviewOutcomeSha256 = $ExpectedIndependentReviewOutcomeSha256
    }
    $output = @(& (Join-Path $PSScriptRoot 'Test-FindingCorrectionHandoff.ps1') @parameters)
    return [pscustomobject]@{ ExitCode = $LASTEXITCODE; Output = (($output | Out-String).Trim()) }
}

function New-PublicationContractRoot {
    param([string]$Name)
    $productRoot = Split-Path -Parent $PSScriptRoot
    $contractRoot = Join-Path $fixtureRoot $Name
    [void][System.IO.Directory]::CreateDirectory($contractRoot)
    Copy-Item -LiteralPath (Join-Path $productRoot 'Governance') -Destination $contractRoot -Recurse
    $catalog = Read-Json (Join-Path $productRoot 'Governance/publication-regression-matrix-catalog.json')
    $matrix = @($catalog.matrices | Where-Object matrixId -CEQ 'GOVERNANCE_HANDOFF_PUBLICATION_FIXTURES')[0]
    foreach ($relativePath in @($matrix.sourcePaths) + @($matrix.dependencyPaths)) {
        $targetPath = Join-Path $contractRoot ([string]$relativePath)
        [void][System.IO.Directory]::CreateDirectory((Split-Path -Parent $targetPath))
        Copy-Item -LiteralPath (Join-Path $productRoot ([string]$relativePath)) -Destination $targetPath
    }
    return $contractRoot
}

function Refresh-PublicationEvidenceForContractRoot {
    param([string]$Directory, [string]$ContractRoot)
    $catalogPath = Join-Path $ContractRoot 'Governance/publication-regression-matrix-catalog.json'
    $catalog = Read-Json $catalogPath
    $matrix = @($catalog.matrices | Where-Object matrixId -CEQ 'GOVERNANCE_HANDOFF_PUBLICATION_FIXTURES')[0]
    $evidence = Read-Json (Join-Path $Directory 'publication-regression-evidence.json')
    $evidence.matrixDefinitionSha256 = Get-Hash $catalogPath
    $evidence.sourceBindings = @($matrix.sourcePaths | ForEach-Object {
            [ordered]@{ path = $_; sha256 = Get-Hash (Join-Path $ContractRoot $_) }
        })
    $evidence.dependencyBindings = @($matrix.dependencyPaths | ForEach-Object {
            [ordered]@{ path = $_; sha256 = Get-Hash (Join-Path $ContractRoot $_) }
        })
    Write-Json (Join-Path $Directory 'publication-regression-evidence.json') $evidence
    Set-PublicationEvidenceHashBindings $Directory (Get-Hash (Join-Path $Directory 'publication-regression-evidence.json'))
}

function Invoke-PublicationExecutionInputDriftCase {
    param(
        [Parameter(Mandatory)][string]$Id,
        [Parameter(Mandatory)][ValidateSet('PARENT_BINDING_CAPTURED', 'RUNNER_IMPORTS_COMPLETED')][string]$PausePhase,
        [Parameter(Mandatory)][ValidateSet('RUNNER', 'DEPENDENCY')][string]$TargetKind
    )
    $contractRoot = New-PublicationContractRoot ('execution-input-' + $Id.ToLowerInvariant())
    $catalog = Read-Json (Join-Path $contractRoot 'Governance/publication-regression-matrix-catalog.json')
    $matrix = @($catalog.matrices | Where-Object matrixId -CEQ 'GOVERNANCE_HANDOFF_PUBLICATION_FIXTURES')[0]
    $runnerPath = Join-Path $contractRoot ([string]$matrix.runner)
    $targetRelativePath = if ($TargetKind -ceq 'RUNNER') {
        [string]$matrix.runner
    }
    else { 'scripts/GovernanceHandoffPublication.psm1' }
    $targetPath = Join-Path $contractRoot $targetRelativePath
    $caseRoot = Join-Path $fixtureRoot ('handshake-' + $Id.ToLowerInvariant())
    $readyRoot = Join-Path $caseRoot 'ready'
    $continueRoot = Join-Path $caseRoot 'continue'
    [void][System.IO.Directory]::CreateDirectory($readyRoot)
    [void][System.IO.Directory]::CreateDirectory($continueRoot)
    $readyPath = Join-Path $readyRoot ($PausePhase + '.ready')
    $continuePath = Join-Path $continueRoot ($PausePhase + '.continue')
    $completionPath = Join-Path $caseRoot 'drift-completion.json'
    $resultPath = Join-Path $caseRoot 'publication-result.json'
    $startInfo = [System.Diagnostics.ProcessStartInfo]::new()
    $startInfo.FileName = (Get-Process -Id $PID).Path
    $startInfo.UseShellExecute = $false
    $startInfo.RedirectStandardOutput = $true
    $startInfo.RedirectStandardError = $true
    foreach ($argument in @(
            '-NoLogo', '-NoProfile', '-File', $runnerPath,
            '-ResultPath', $resultPath,
            '-PhaseHandshakeDirectory', $readyRoot,
            '-PhaseContinueDirectory', $continueRoot,
            '-PausePhase', $PausePhase
        )) { [void]$startInfo.ArgumentList.Add($argument) }
    $runnerProcess = [System.Diagnostics.Process]::new()
    $runnerProcess.StartInfo = $startInfo
    try {
        [void]$runnerProcess.Start()
        $stdoutTask = $runnerProcess.StandardOutput.ReadToEndAsync()
        $stderrTask = $runnerProcess.StandardError.ReadToEndAsync()
        $stopwatch = [System.Diagnostics.Stopwatch]::StartNew()
        while (-not (Test-Path -LiteralPath $readyPath -PathType Leaf)) {
            if ($runnerProcess.HasExited) { throw "Runner exited before handshake: $Id" }
            if ($stopwatch.Elapsed.TotalSeconds -ge 20) { throw "Runner handshake timed out: $Id" }
            [System.Threading.Thread]::Sleep(50)
        }
        $driftArguments = @(
            '-NoLogo', '-NoProfile', '-File', (Join-Path $PSScriptRoot 'testdata/Invoke-GovernancePublicationInputDrift.ps1'),
            '-TargetPath', $targetPath, '-ContinuePath', $continuePath, '-CompletionPath', $completionPath
        )
        $driftProcess = Start-Process -FilePath (Get-Process -Id $PID).Path -ArgumentList $driftArguments `
            -PassThru -Wait -WindowStyle Hidden
        if ($driftProcess.ExitCode -ne 0 -or -not (Test-Path -LiteralPath $completionPath -PathType Leaf)) {
            throw "Input drift process failed: $Id"
        }
        [void]$runnerProcess.WaitForExit(30000)
        if (-not $runnerProcess.HasExited) { throw "Runner did not terminate after drift: $Id" }
        $stdout = $stdoutTask.GetAwaiter().GetResult()
        $stderr = $stderrTask.GetAwaiter().GetResult()
        if (-not (Test-Path -LiteralPath $resultPath -PathType Leaf)) {
            throw "Runner did not persist a fail-closed result: $Id; stdout=$stdout; stderr=$stderr"
        }
        $result = Read-Json $resultPath
        return [pscustomobject]@{
            Passed = (
                $runnerProcess.ExitCode -ne 0 -and [int]$result.schemaVersion -eq 2 -and
                [string]$result.status -ceq 'FAIL' -and [int]$result.selected -eq 0 -and
                [int]$result.passed -eq 0 -and [int]$result.failed -eq 0 -and
                @($result.results).Count -eq 0 -and $null -ne $result.executionInputBinding
            )
            Evidence = ('phase={0};target={1};exit={2};selected={3};passed={4}' -f
                $PausePhase, $targetRelativePath, $runnerProcess.ExitCode,
                [int]$result.selected, [int]$result.passed)
        }
    }
    finally {
        if (-not $runnerProcess.HasExited) { Stop-Process -Id $runnerProcess.Id -Force }
        $runnerProcess.Dispose()
    }
}

function New-SyntheticSource {
    param(
        [string]$Directory, [string]$RepositoryRoot, [string]$TaskId, [string]$FindingId,
        [string]$BaselineCommit, [string]$PreviousCommit, [string]$PreviousTree,
        [byte[]]$CurrentPatch, [byte[]]$CorrectionPatch, [object[]]$CurrentEntries,
        [object[]]$CorrectionEntries, [string]$CurrentTree, [ValidateSet('COMMIT','IMMUTABLE_REVIEW_PACKAGE')][string]$PreviousMode,
        [string]$HistoricalPackagePath, [object]$HistoricalBinding
    )
    [void][System.IO.Directory]::CreateDirectory($Directory)
    [System.IO.File]::WriteAllBytes((Join-Path $Directory 'current-delta.patch'), $CurrentPatch)
    [System.IO.File]::WriteAllBytes((Join-Path $Directory 'correction-only.patch'), $CorrectionPatch)
    $currentScopeEntries = @($CurrentEntries | ForEach-Object { Convert-ScopeEntry $_ 'Synthetic full feature scope.' })
    $correctionRows = @($CorrectionEntries | ForEach-Object { Convert-CorrectionEntry $_ $FindingId })
    $currentPaths = @(Get-GenericScopePaths -Entry $CurrentEntries | Sort-Object -Unique)
    $correctionPaths = @(Get-GenericScopePaths -Entry $CorrectionEntries | Sort-Object -Unique)
    $currentPatchHash = Get-Hash (Join-Path $Directory 'current-delta.patch')
    $correctionPatchHash = Get-Hash (Join-Path $Directory 'correction-only.patch')
    $previousReviewHash = if ($PreviousMode -ceq 'COMMIT') {
        Get-StringHash -Value $PreviousCommit
    }
    else { [string]$HistoricalBinding.historicalPackageSha256 }
    Write-Json (Join-Path $Directory 'scope-inventory.json') ([ordered]@{ schemaVersion=1; taskId=$TaskId; baselineCommit=$BaselineCommit; detachedHead=$false; stagedPathCount=@($CurrentEntries|Where-Object Staged).Count; expectedStatusSha256=('a'*64); actualStatusSha256=('a'*64); currentFeaturePatch='current-feature.patch'; currentFeaturePatchSha256=$currentPatchHash; pathCount=$currentScopeEntries.Count; entries=$currentScopeEntries })
    Write-Json (Join-Path $Directory 'correction-scope-inventory.json') ([ordered]@{ schemaVersion=1; taskId=$TaskId; previousReviewBinding='previous-review-binding.json'; previousReviewedTree=$PreviousTree; currentCorrectedTree=$CurrentTree; correctionOnlyPatch='correction-only.patch'; correctionOnlyPatchSha256=$correctionPatchHash; pathCount=$correctionRows.Count; forwardApplyValidation='PASS'; fullFeatureTreeParity='PASS'; entries=$correctionRows })
    $previousState = if ($PreviousMode -ceq 'COMMIT') {
        [ordered]@{ type='COMMIT'; correctionStartCommit=$PreviousCommit; previousReviewedTree=$PreviousTree }
    } else { $HistoricalBinding }
    Write-Json (Join-Path $Directory 'previous-review-binding.json') ([ordered]@{ schemaVersion=3; taskId=$TaskId; previousReviewState=$previousState })
    $tests = @('FCH-PRODUCTIVE-SYNTHETIC')
    $findingSemantics = [ordered]@{
        id=$FindingId; severity='MAJOR'; previousStatus='OPEN'; status='CORRECTED_PENDING_DELTA'
        disposition='CORRECTED'; correction='Synthetic correction.'
        affectedPaths=$correctionPaths; regressionTestIds=$tests; evidenceReferences=@('validation-summary.json')
    }
    Write-Json (Join-Path $Directory 'finding-ledger.json') ([ordered]@{ schemaVersion=1; taskId=$TaskId; findingCount=1; findings=@([ordered]@{ id=$FindingId; severity=$findingSemantics.severity; previousStatus=$findingSemantics.previousStatus; status=$findingSemantics.status; disposition=$findingSemantics.disposition; correction=$findingSemantics.correction; correctionPaths=$correctionPaths; permanentRegressions=$tests; evidenceReferences=$findingSemantics.evidenceReferences; producerStatus='CORRECTED_PENDING_DELTA_REVIEW'; reviewerStatus='OPEN_PENDING_FOCUSED_INDEPENDENT_DELTA_REVIEW' }) })
    Write-Json (Join-Path $Directory 'finding-correction-matrix.json') ([ordered]@{ schemaVersion=2; mode='BUNDLED_CORRECTION'; previousReviewPackage=if($PreviousMode -ceq 'COMMIT'){'COMMIT'}else{$HistoricalPackagePath}; previousReviewSha256=$previousReviewHash; correctedFindingCount=1; repositoryCorrectionPaths=$correctionPaths; findings=@($findingSemantics) })
    $focusedValidationResultPath = Join-Path $Directory 'focused-validation-result.json'
    Write-Json $focusedValidationResultPath ([ordered]@{ schemaVersion=2; status='PASS'; selected=1; passed=1; failed=0; results=@([ordered]@{ id=$tests[0]; result='PASS'; evidence='Synthetic execution.' }); failureMessage=$null })
    $focusedValidationResultItem = Get-Item -LiteralPath $focusedValidationResultPath
    $focusedValidationResultHash = Get-Hash $focusedValidationResultPath
    Write-Json (Join-Path $Directory 'finding-regression-matrix.json') ([ordered]@{ schemaVersion=2; fixtureCount=1; fixtureResult='PASS'; validatorPath='scripts/Test-FindingCorrectionHandoff.ps1'; finalFocusedValidationEvidence=[ordered]@{ sourceArtifact='focused-validation-result.json'; sourceEvidenceSha256=$focusedValidationResultHash; sourceEvidenceLength=[int64]$focusedValidationResultItem.Length; status='PASS'; selected=1; passed=1; failed=0 }; findings=@([ordered]@{ id=$FindingId; severity=$findingSemantics.severity; previousStatus=$findingSemantics.previousStatus; status=$findingSemantics.status; disposition=$findingSemantics.disposition; correction=$findingSemantics.correction; affectedPaths=$correctionPaths; evidenceReferences=$findingSemantics.evidenceReferences; regressionTests=@([ordered]@{ id=$tests[0]; status='PASS'; validatorPath='scripts/Test-FindingCorrectionHandoff.ps1'; evidence='Synthetic execution.' }) }) })
    $focusedState = if ($PreviousMode -ceq 'COMMIT') { [ordered]@{ type='COMMIT'; correctionStartCommit=$PreviousCommit } } else { $HistoricalBinding.PSObject.Copy(); $HistoricalBinding }
    if ($PreviousMode -ceq 'IMMUTABLE_REVIEW_PACKAGE') { $focusedState = [ordered]@{}; foreach($property in $HistoricalBinding.PSObject.Properties){ if($property.Name -cne 'historicalPackagePath'){ $focusedState[$property.Name]=$property.Value } } }
    Write-Json (Join-Path $Directory 'focused-delta-review-record.json') ([ordered]@{ schemaVersion=3; mode='FOCUSED_INDEPENDENT_DELTA_REVIEW'; previousReviewPackage=if($PreviousMode -ceq 'COMMIT'){'COMMIT'}else{$HistoricalPackagePath}; previousReviewSha256=$previousReviewHash; previousReviewState=$focusedState; correctionPatchArtifact='correction-only.patch'; correctionPatchSha256=$correctionPatchHash; currentDeltaArtifact='current-delta.patch'; currentDeltaSha256=$currentPatchHash; correctionOnlyPaths=$correctionPaths; reviewedFindingIds=@($FindingId); directInterfacePaths=$correctionPaths; regressionTestIds=$tests; allowedDeltaPaths=$currentPaths; referenceOnlyPaths=@('reference.txt'); fullReviewRepeatAuthorized=$false; commitPreparationApproved=$false; commitAuthorized=$false })
    Write-Json (Join-Path $Directory 'external-governance-manifest.json') ([ordered]@{ schemaVersion=1; changes=@() })
    Write-Json (Join-Path $Directory 'trusted-expected-hashes.json') ([ordered]@{ schemaVersion=1; previousReviewPackage=if($PreviousMode -ceq 'COMMIT'){'COMMIT'}else{$HistoricalPackagePath}; previousReviewSha256=$previousReviewHash; correctionPatchArtifact='correction-only.patch'; correctionPatchSha256=$correctionPatchHash; currentDeltaArtifact='current-delta.patch'; currentDeltaSha256=$currentPatchHash })
    Write-Json (Join-Path $Directory 'readiness-evidence.json') ([ordered]@{ schemaVersion=2; artifactLifecycleState='ZIP_FREE_READY_TO_EXECUTE'; status='PASS'; readyToExecute=$true; classicReviewReady=$false; packageWriteAttemptCount=0; directoryValidation='PASS'; findingIds=@($FindingId) })
    Write-Json (Join-Path $Directory 'validation-summary.json') ([ordered]@{ schemaVersion=1; taskId=$TaskId; profile='FINDING_CORRECTION'; result='PASS'; focusedFixtureCount=1; focusedFixtureSelectedCount=1; focusedFixturePassedCount=1; focusedFixtureEvidenceSha256=$focusedValidationResultHash; focusedFixtureEvidenceLength=[int64]$focusedValidationResultItem.Length; focusedFixtureEvidenceArtifact='focused-validation-result.json'; focusedFixtureResult='PASS'; previousReviewBinding='PASS'; currentFeaturePatch='PASS'; correctionOnlyPatch='PASS'; materialCorrectionCycleCount=1; validationExecutionCount=1; infrastructureOrInvocationFailureCount=0; warningCount=0; failureCount=0 })
    $reportContract = [ordered]@{
        schemaVersion = 1; taskId = $TaskId; profile = 'FINDING_CORRECTION'
        transitionType = 'BUNDLED_CORRECTION_TO_FOCUSED_DELTA_REVIEW'
        artifactLifecycleState = 'ZIP_FREE_READY_TO_EXECUTE'; status = 'ZIP_FREE_READY_TO_EXECUTE'; readyToExecute = $true
        previousReview = [ordered]@{
            stateType = $PreviousMode; reviewStateSha256 = $previousReviewHash
            bindingArtifact = 'previous-review-binding.json'; bindingSha256 = Get-Hash (Join-Path $Directory 'previous-review-binding.json')
        }
        currentFeaturePatch = [ordered]@{ artifact = 'current-delta.patch'; sha256 = $currentPatchHash; pathCount = $currentPaths.Count }
        correctionOnlyPatch = [ordered]@{ artifact = 'correction-only.patch'; sha256 = $correctionPatchHash; pathCount = $correctionPaths.Count }
        findingIds = @($FindingId)
        findingDispositions = @([ordered]@{ id=$FindingId; severity=$findingSemantics.severity; previousStatus=$findingSemantics.previousStatus; status=$findingSemantics.status; disposition=$findingSemantics.disposition; correction=$findingSemantics.correction; affectedPaths=$correctionPaths; regressionTestIds=$tests; evidenceReferences=$findingSemantics.evidenceReferences; producerStatus='CORRECTED_PENDING_DELTA_REVIEW'; reviewerStatus='OPEN_PENDING_FOCUSED_INDEPENDENT_DELTA_REVIEW' })
        scopeSemantics = [ordered]@{ correctionOnlyPaths = $correctionPaths; directInterfacePaths = $correctionPaths; referenceOnlyPaths = @('reference.txt') }
        permanentRegressionEvidence = [ordered]@{ testIds = $tests; result = 'PASS' }
        focusedValidationResult = [ordered]@{ result = 'PASS'; selected = 1; passed = 1 }
        independentDeltaReviewRequired = $true; classicReviewReady = $false
        packageWriteAttemptCount = 0; nextAction = 'READY_FOR_SINGLE_DELTA_REVIEW_PACKAGE_WRITE_AUTHORIZATION'
    }
    $reportJson = $reportContract | ConvertTo-Json -Depth 100
    $reportText = @"
# $TaskId finding-correction focused delta handoff

ArtifactLifecycleState: ZIP_FREE_READY_TO_EXECUTE
Status: ZIP_FREE_READY_TO_EXECUTE
Profile: FINDING_CORRECTION
TransitionType: BUNDLED_CORRECTION_TO_FOCUSED_DELTA_REVIEW
CurrentFeaturePathCount: $($currentPaths.Count)
CorrectionOnlyPathCount: $($correctionPaths.Count)
FindingIds: $FindingId
FindingStatus: CORRECTED_PENDING_DELTA_REVIEW
ReadyToExecute: true
ClassicReviewReady: false
PackageWriteAttemptCount: 0
IndependentDeltaReviewRequired: true
NextAction: READY_FOR_SINGLE_DELTA_REVIEW_PACKAGE_WRITE_AUTHORIZATION

<!-- BEGIN FINDING-CORRECTION-REPORT-CONTRACT -->
$reportJson
<!-- END FINDING-CORRECTION-REPORT-CONTRACT -->
"@
    Write-Utf8 (Join-Path $Directory 'report.md') $reportText
    Write-Json (Join-Path $Directory 'assignment-record.json') ([ordered]@{ schemaVersion=2; artifactLifecycleState='ZIP_FREE_READY_TO_EXECUTE'; readyToExecute=$true; taskId=$TaskId; repository='https://github.com/thomasweidner/flashgate-mcp.git'; baselineCommit=$BaselineCommit; currentCommit=$PreviousCommit; branch='fixture'; executionMode='BUNDLED_CORRECTION'; checkpoint='MATERIAL_SCOPE_CHANGE'; profile='FINDING_CORRECTION'; transitionType='BUNDLED_CORRECTION_TO_FOCUSED_DELTA_REVIEW'; currentStateGate=[ordered]@{ result='PASS'; repositoryIdentityBound=$true; commitAndBranchBound=$true; completeStatusBound=$true; scopeAndIdsBound=$true; parallelWorktreesBound=$true }; findingIds=@($FindingId); classicReviewReady=$false; commitAuthorized=$false; scopeInventorySha256=Get-Hash(Join-Path $Directory 'scope-inventory.json'); correctionScopeInventorySha256=Get-Hash(Join-Path $Directory 'correction-scope-inventory.json'); currentDeltaSha256=$currentPatchHash; correctionPatchSha256=$correctionPatchHash; findingLedgerSha256=Get-Hash(Join-Path $Directory 'finding-ledger.json'); previousReviewBindingSha256=Get-Hash(Join-Path $Directory 'previous-review-binding.json'); allowedDeltaPaths=$currentPaths; correctionOnlyPaths=$correctionPaths; directInterfacePaths=$correctionPaths; nextAction='READY_FOR_SINGLE_DELTA_REVIEW_PACKAGE_WRITE_AUTHORIZATION' })
    Write-Json (Join-Path $Directory 'completion-report.json') ([ordered]@{ schemaVersion=2; artifactLifecycleState='ZIP_FREE_READY_TO_EXECUTE'; taskId=$TaskId; profile='FINDING_CORRECTION'; transitionType='BUNDLED_CORRECTION_TO_FOCUSED_DELTA_REVIEW'; status='ZIP_FREE_READY_TO_EXECUTE'; findingIds=@($FindingId); reviewStatus='AWAITING_FOCUSED_INDEPENDENT_DELTA_REVIEW'; readyToExecute=$true; classicReviewReady=$false; commitAuthorized=$false; fullCompletionReexecuted=$false; fullMatrixRunCount=1; productionRunInvocationCount=1; automaticRetryCount=0; packageWriteAttemptCount=0; materialCorrectionCycleCount=1; validationExecutionCount=1; infrastructureOrInvocationFailureCount=0; observedWarningCount=0; resolvedWarningCount=0; openWarningCount=0; warningCount=0; failureCount=0; nextAction='READY_FOR_SINGLE_DELTA_REVIEW_PACKAGE_WRITE_AUTHORIZATION' })
    return [pscustomobject]@{ CurrentPaths=$currentPaths; CorrectionPaths=$correctionPaths }
}

function Invoke-Generator {
    param([string]$Source, [string]$RepositoryRoot, [string]$TaskId, [string[]]$AllowedPaths, [string]$OutputPath, [ValidateSet('Preflight','FinalContent','Zip')][string]$Kind)
    $parameters = @{
        Profile = 'FINDING_CORRECTION'
        TransitionType = 'BUNDLED_CORRECTION_TO_FOCUSED_DELTA_REVIEW'
        TaskId = $TaskId
        SourceDirectory = $Source
        AllowedDeltaPath = $AllowedPaths
        PackagePath = $OutputPath
        AuthoritativeRepositoryRoot = $RepositoryRoot
    }
    if ($Kind -ceq 'Preflight') { $parameters.PreflightOnly = $true; $parameters.StagingDirectory = $OutputPath }
    elseif ($Kind -ceq 'FinalContent') { $parameters.FinalPackageContentOnly = $true; $parameters.StagingDirectory = $OutputPath }
    $output = @(& (Join-Path $PSScriptRoot 'New-GovernanceHandoff.ps1') @parameters)
    return [pscustomobject]@{ ExitCode=$LASTEXITCODE; Output=(($output|Out-String).Trim()) }
}

function Convert-ToTwoFindingParityFixture {
    param(
        [Parameter(Mandatory)][string]$Directory,
        [string[]]$FindingIds = @('BL339-REV-003', 'BL339-REV-004'),
        [string[]]$TestIds = @('FCH-PER-FINDING-CONTRACT-A', 'FCH-PER-FINDING-CONTRACT-B')
    )

    if ($FindingIds.Count -ne 2 -or $TestIds.Count -ne 2) {
        throw 'Two-finding fixture requires exactly two finding IDs and two test IDs.'
    }
    $clone = {
        param([object]$Value)
        return $Value | ConvertTo-Json -Depth 100 | ConvertFrom-Json -Depth 100
    }

    $correction = Read-Json (Join-Path $Directory 'finding-correction-matrix.json')
    $correctionFirst = & $clone $correction.findings[0]
    $correctionSecond = & $clone $correction.findings[0]
    $correctionFirst.id = $findingIds[0]
    $correctionFirst.regressionTestIds = @($testIds[0])
    $correctionSecond.id = $findingIds[1]
    $correctionSecond.regressionTestIds = @($testIds[1])
    $correction.correctedFindingCount = 2
    $correction.findings = @($correctionFirst, $correctionSecond)
    Write-Json (Join-Path $Directory 'finding-correction-matrix.json') $correction

    $focusedValidationResultPath = Join-Path $Directory 'focused-validation-result.json'
    $focusedValidationResult = Read-Json $focusedValidationResultPath
    $focusedValidationResult.selected = 2
    $focusedValidationResult.passed = 2
    $focusedValidationResult.results = @(
        [ordered]@{ id=$testIds[0]; result='PASS'; evidence='Synthetic execution A.' },
        [ordered]@{ id=$testIds[1]; result='PASS'; evidence='Synthetic execution B.' }
    )
    Write-Json $focusedValidationResultPath $focusedValidationResult
    $focusedValidationResultItem = Get-Item -LiteralPath $focusedValidationResultPath
    $focusedValidationResultHash = Get-Hash $focusedValidationResultPath

    $regression = Read-Json (Join-Path $Directory 'finding-regression-matrix.json')
    $regressionFirst = & $clone $regression.findings[0]
    $regressionSecond = & $clone $regression.findings[0]
    $regressionFirst.id = $findingIds[0]
    $regressionFirst.regressionTests[0].id = $testIds[0]
    $regressionSecond.id = $findingIds[1]
    $regressionSecond.regressionTests[0].id = $testIds[1]
    $regression.fixtureCount = 2
    $regression.finalFocusedValidationEvidence.selected = 2
    $regression.finalFocusedValidationEvidence.passed = 2
    $regression.finalFocusedValidationEvidence.sourceEvidenceSha256 = $focusedValidationResultHash
    $regression.finalFocusedValidationEvidence.sourceEvidenceLength = [int64]$focusedValidationResultItem.Length
    $regression.findings = @($regressionFirst, $regressionSecond)
    Write-Json (Join-Path $Directory 'finding-regression-matrix.json') $regression

    $ledger = Read-Json (Join-Path $Directory 'finding-ledger.json')
    $ledgerFirst = & $clone $ledger.findings[0]
    $ledgerSecond = & $clone $ledger.findings[0]
    $ledgerFirst.id = $findingIds[0]
    $ledgerFirst.permanentRegressions = @($testIds[0])
    $ledgerSecond.id = $findingIds[1]
    $ledgerSecond.permanentRegressions = @($testIds[1])
    $ledger.findingCount = 2
    $ledger.findings = @($ledgerFirst, $ledgerSecond)
    Write-Json (Join-Path $Directory 'finding-ledger.json') $ledger

    $scope = Read-Json (Join-Path $Directory 'correction-scope-inventory.json')
    foreach ($entry in @($scope.entries)) { $entry.findingIds = $findingIds }
    Write-Json (Join-Path $Directory 'correction-scope-inventory.json') $scope

    $assignment = Read-Json (Join-Path $Directory 'assignment-record.json')
    $assignment.findingIds = $findingIds
    $assignment.findingLedgerSha256 = Get-Hash (Join-Path $Directory 'finding-ledger.json')
    $assignment.correctionScopeInventorySha256 = Get-Hash (Join-Path $Directory 'correction-scope-inventory.json')
    Write-Json (Join-Path $Directory 'assignment-record.json') $assignment
    foreach ($name in @('completion-report.json', 'readiness-evidence.json')) {
        $contract = Read-Json (Join-Path $Directory $name)
        $contract.findingIds = $findingIds
        Write-Json (Join-Path $Directory $name) $contract
    }

    $focused = Read-Json (Join-Path $Directory 'focused-delta-review-record.json')
    $focused.reviewedFindingIds = $findingIds
    $focused.regressionTestIds = $testIds
    Write-Json (Join-Path $Directory 'focused-delta-review-record.json') $focused

    $summary = Read-Json (Join-Path $Directory 'validation-summary.json')
    $summary.focusedFixtureCount = 2
    $summary.focusedFixtureSelectedCount = 2
    $summary.focusedFixturePassedCount = 2
    $summary.focusedFixtureEvidenceSha256 = $focusedValidationResultHash
    $summary.focusedFixtureEvidenceLength = [int64]$focusedValidationResultItem.Length
    Write-Json (Join-Path $Directory 'validation-summary.json') $summary

    $report = Read-ReportContract (Join-Path $Directory 'report.md')
    $reportFirst = & $clone $report.findingDispositions[0]
    $reportSecond = & $clone $report.findingDispositions[0]
    $reportFirst.id = $findingIds[0]
    $reportFirst.regressionTestIds = @($testIds[0])
    $reportSecond.id = $findingIds[1]
    $reportSecond.regressionTestIds = @($testIds[1])
    $report.findingIds = $findingIds
    $report.findingDispositions = @($reportFirst, $reportSecond)
    $report.permanentRegressionEvidence.testIds = $testIds
    $report.focusedValidationResult.selected = 2
    $report.focusedValidationResult.passed = 2
    Write-ReportContract (Join-Path $Directory 'report.md') $report
    Update-PackageMetadata $Directory
}

try {
    $fixtureRoot = Join-Path ([System.IO.Path]::GetTempPath()) ('flashgate-finding-correction-product-' + [guid]::NewGuid().ToString('N'))
    $repo = Join-Path $fixtureRoot 'repo'
    [void][System.IO.Directory]::CreateDirectory($repo)
    $null = Invoke-GitText $repo @('init','--quiet')
    $null = Invoke-GitText $repo @('config','user.name','FlashGate Fixture')
    $null = Invoke-GitText $repo @('config','user.email','fixture@example.invalid')
    Write-Utf8 (Join-Path $repo 'reference.txt') "reference v1`n"
    Write-Utf8 (Join-Path $repo 'modify.txt') "before`n"
    Write-Utf8 (Join-Path $repo 'delete.txt') "delete`n"
    Write-Utf8 (Join-Path $repo 'rename-unchanged.txt') "unchanged rename`n"
    Write-Utf8 (Join-Path $repo 'rename-modified.txt') ((1..20 | ForEach-Object { "stable line $_" }) -join "`n")
    Write-Utf8 (Join-Path $repo 'mode.sh') "#!/bin/sh`necho fixture`n"
    Write-Utf8 (Join-Path $repo 'historical-source.txt') "historical rename`n"
    $null = Invoke-GitText $repo @('add','-A')
    $null = Invoke-GitText $repo @('commit','--quiet','-m','baseline')
    $baseline = Invoke-GitText $repo @('rev-parse','HEAD')
    Write-Utf8 (Join-Path $repo 'reference.txt') "reference v2`n"
    Move-Item -LiteralPath (Join-Path $repo 'historical-source.txt') `
        -Destination (Join-Path $repo 'historical-target.txt')
    $null = Invoke-GitText $repo @('add','-A')
    $null = Invoke-GitText $repo @('commit','--quiet','-m','previous review')
    $previousCommit = Invoke-GitText $repo @('rev-parse','HEAD')
    $previousTree = Invoke-GitText $repo @('rev-parse','HEAD^{tree}')
    Write-Utf8 (Join-Path $repo 'modify.txt') "after`n"
    Write-Utf8 (Join-Path $repo 'added.txt') "added`n"
    Remove-Item -LiteralPath (Join-Path $repo 'delete.txt')
    Move-Item -LiteralPath (Join-Path $repo 'rename-unchanged.txt') -Destination (Join-Path $repo 'renamed-unchanged.txt')
    Move-Item -LiteralPath (Join-Path $repo 'rename-modified.txt') -Destination (Join-Path $repo 'renamed-modified.txt')
    Add-Content -LiteralPath (Join-Path $repo 'renamed-modified.txt') -Value 'corrected line' -Encoding utf8NoBOM
    $null = Invoke-GitText $repo @('update-index','--chmod=+x','mode.sh')

    $currentEntries = @(Get-GenericStatusEvidence -Root $repo -BaselineCommit $baseline)
    $referencePostimage = Get-GenericPostimageEvidence -Root $repo -Path 'reference.txt' -GitMode '100644'
    $currentEntries += [pscustomobject][ordered]@{
        Path='reference.txt'; PreviousPath=$null; GitStatus='TRACKED_MODIFIED'; Tracked=$true; Staged=$false
        Preimage=Get-GenericBaselineBlobEvidence -Root $repo -Commit $baseline -Path 'reference.txt'
        Postimage=$referencePostimage; PostimageAbsent=$false
    }
    $historicalRenamePostimage = Get-GenericPostimageEvidence -Root $repo -Path 'historical-target.txt' -GitMode '100644'
    $currentEntries += [pscustomobject][ordered]@{
        Path='historical-target.txt'; PreviousPath='historical-source.txt'; GitStatus='TRACKED_RENAMED'; Tracked=$true; Staged=$false
        Preimage=Get-GenericBaselineBlobEvidence -Root $repo -Commit $baseline -Path 'historical-source.txt'
        Postimage=$historicalRenamePostimage; PostimageAbsent=$false
    }
    $currentEntries = @($currentEntries | Sort-Object Path)
    $correctionEntries = @(Get-GenericStatusEvidence -Root $repo -BaselineCommit $previousCommit | Sort-Object Path)
    foreach ($entry in @($correctionEntries | Where-Object { $null -eq $_.Preimage -and $_.GitStatus -notin @('UNTRACKED','TRACKED_ADDED') })) {
        $preimagePath = if ($entry.GitStatus -ceq 'TRACKED_RENAMED') { [string]$entry.PreviousPath } else { [string]$entry.Path }
        $entry.Preimage = Get-GenericBaselineBlobEvidence -Root $repo -Commit $previousCommit -Path $preimagePath
    }
    foreach ($entry in @($currentEntries + $correctionEntries | Where-Object Path -ceq 'mode.sh')) {
        $entry.GitStatus = 'TRACKED_MODE_CHANGED'
        $entry.Postimage.mode = '100755'
    }
    $currentEvidence = Get-GenericDeltaEvidence -Root $repo -BaselineCommit $baseline -IncludedEntry $currentEntries -ExcludedEntry @()
    $correctionEvidence = Get-GenericDeltaEvidence -Root $repo -BaselineCommit $previousCommit -IncludedEntry $correctionEntries -ExcludedEntry @()
    $modePatch = $utf8.GetBytes("diff --git a/mode.sh b/mode.sh`nold mode 100644`nnew mode 100755`n")
    $currentPatchBytes = [byte[]]($currentEvidence.Bytes + $modePatch)
    $correctionPatchBytes = [byte[]]($correctionEvidence.Bytes + $modePatch)
    $currentTree = Get-AppliedTree -Root $repo -Baseline $baseline -PatchBytes $currentPatchBytes
    $correctedTree = Get-AppliedTree -Root $repo -Baseline $previousCommit -PatchBytes $correctionPatchBytes
    Add-Case 'FCH-CURRENT-AND-CORRECTION-TREE-PARITY' ($currentTree -ceq $correctedTree)
    $statuses = @($correctionEntries | ForEach-Object GitStatus)
    Add-Case 'FCH-TRACKED-MODIFICATION-PRESENT' ('TRACKED_MODIFIED' -cin $statuses)
    Add-Case 'FCH-ADDED-PATH-PRESENT' (@($correctionEntries | Where-Object { $_.GitStatus -in @('UNTRACKED','TRACKED_ADDED') }).Count -gt 0)
    Add-Case 'FCH-DELETED-PATH-PRESENT' ('TRACKED_DELETED' -cin $statuses)
    Add-Case 'FCH-MODE-CHANGE-PRESENT' ($modePatch.Length -gt 0)
    Add-Case 'FCH-RENAME-UNCHANGED-PRESENT' (@($correctionEntries | Where-Object { $_.GitStatus -ceq 'TRACKED_RENAMED' -and $_.Path -ceq 'renamed-unchanged.txt' }).Count -eq 1)
    Add-Case 'FCH-RENAME-MODIFIED-PRESENT' (@($correctionEntries | Where-Object { $_.GitStatus -ceq 'TRACKED_RENAMED' -and $_.Path -ceq 'renamed-modified.txt' }).Count -eq 1)

    $historicalDir = Join-Path $fixtureRoot 'historical-source'
    [void][System.IO.Directory]::CreateDirectory($historicalDir)
    $historicalPatch = (Invoke-GenericGitBytes -Root $repo -Argument @('diff','--binary','--full-index',$baseline,$previousCommit,'--')).Bytes
    [System.IO.File]::WriteAllBytes((Join-Path $historicalDir 'current-delta.patch'), $historicalPatch)
    $historicalScopeValue = [ordered]@{
        schemaVersion=1; taskId='BL-339'; baselineCommit=$baseline; pathCount=2
        entries=@(
            [ordered]@{ path='reference.txt'; previousPath=$null; gitStatus='TRACKED_MODIFIED' },
            [ordered]@{ path='historical-target.txt'; previousPath='historical-source.txt'; gitStatus='TRACKED_RENAMED' }
        )
    }
    Write-Json (Join-Path $historicalDir 'scope-inventory.json') $historicalScopeValue
    $historicalManifestLines = @('current-delta.patch','scope-inventory.json' | ForEach-Object { $f=Get-Item(Join-Path $historicalDir $_); "$(Get-Hash $f.FullName)  $($f.Length)  $_" })
    Write-Utf8 (Join-Path $historicalDir 'MANIFEST.sha256') (($historicalManifestLines -join "`n")+"`n")
    $historicalZip = Join-Path $fixtureRoot 'historical.zip'
    New-ZipFromDirectory $historicalDir $historicalZip
    $currentPaths = @(Get-GenericScopePaths -Entry $currentEntries | Sort-Object -Unique)
    $historicalBoundPaths = @(Get-GenericScopePaths -Entry @($historicalScopeValue.entries) | Sort-Object -Unique)
    $postimages = @($historicalBoundPaths | ForEach-Object { Get-TreePostimage -Root $repo -Tree $previousTree -Path $_ })
    $historicalBinding = [pscustomobject][ordered]@{ type='IMMUTABLE_REVIEW_PACKAGE'; historicalPackagePath=$historicalZip; historicalPackageSha256=Get-Hash($historicalZip); historicalManifestSha256=Get-Hash(Join-Path $historicalDir 'MANIFEST.sha256'); historicalPatchSha256=Get-Hash(Join-Path $historicalDir 'current-delta.patch'); historicalScopeInventorySha256=Get-Hash(Join-Path $historicalDir 'scope-inventory.json'); previousReviewedBaselineCommit=$baseline; previousReviewedTree=$previousTree; previousReviewedPathCount=2; previousReviewedPostimages=$postimages }

    $commitSource = Join-Path $fixtureRoot 'commit-source'
    $commitContract = New-SyntheticSource -Directory $commitSource -RepositoryRoot $repo -TaskId 'BL-339' -FindingId 'BL339-REV-003' -BaselineCommit $baseline -PreviousCommit $previousCommit -PreviousTree $previousTree -CurrentPatch $currentPatchBytes -CorrectionPatch $correctionPatchBytes -CurrentEntries $currentEntries -CorrectionEntries $correctionEntries -CurrentTree $currentTree -PreviousMode COMMIT -HistoricalPackagePath $historicalZip -HistoricalBinding $historicalBinding
    $preflight = Join-Path $fixtureRoot 'commit-preflight'
    $preflightRun = Invoke-Generator $commitSource $repo 'BL-339' $commitContract.CurrentPaths $preflight Preflight
    if (-not $PublicationEvidenceOnly) {
        Add-Case 'FCH-COMMIT-PRODUCT-VALIDATOR-PASS' ($preflightRun.ExitCode -eq 0) $preflightRun.Output
        Add-Case 'FCH-PREFLIGHT-LIFECYCLE-PASS' ((Read-Json (Join-Path $preflight 'completion-report.json')).artifactLifecycleState -ceq 'ZIP_FREE_READY_TO_EXECUTE')
    }
    $preflightReportContract = Read-ReportContract (Join-Path $preflight 'report.md')
    if (-not $PublicationEvidenceOnly) {
        $preflightValidationSummary = Read-Json (Join-Path $preflight 'validation-summary.json')
        $preflightRegressionMatrix = Read-Json (Join-Path $preflight 'finding-regression-matrix.json')

        $staleFocusedCount = Copy-Artifact $preflight 'mutant-stale-focused-validation-count'
        $staleSummary = Read-Json (Join-Path $staleFocusedCount 'validation-summary.json')
        $staleSummary.focusedFixtureCount = 139
        $staleSummary.focusedFixtureSelectedCount = 139
        $staleSummary.focusedFixturePassedCount = 139
        Write-Json (Join-Path $staleFocusedCount 'validation-summary.json') $staleSummary
        $staleReport = Read-ReportContract (Join-Path $staleFocusedCount 'report.md')
        $staleReport.focusedValidationResult.selected = 139
        $staleReport.focusedValidationResult.passed = 139
        Write-ReportContract (Join-Path $staleFocusedCount 'report.md') $staleReport
        Update-PackageMetadata $staleFocusedCount
        $staleFocusedValidation = Invoke-ProductValidator $staleFocusedCount $repo

        $selectedPassedMismatch = Copy-Artifact $preflight 'mutant-focused-selected-passed-mismatch'
        $mismatchReport = Read-ReportContract (Join-Path $selectedPassedMismatch 'report.md')
        $mismatchReport.focusedValidationResult.passed = 2
        Write-ReportContract (Join-Path $selectedPassedMismatch 'report.md') $mismatchReport
        Update-PackageMetadata $selectedPassedMismatch
        $selectedPassedValidation = Invoke-ProductValidator $selectedPassedMismatch $repo

        $actualFocusedMismatch = Copy-Artifact $preflight 'mutant-final-focused-result-count-mismatch'
        $actualRegression = Read-Json (Join-Path $actualFocusedMismatch 'finding-regression-matrix.json')
        $actualRegression.fixtureCount = 2
        $actualRegression.finalFocusedValidationEvidence.selected = 2
        $actualRegression.finalFocusedValidationEvidence.passed = 2
        Write-Json (Join-Path $actualFocusedMismatch 'finding-regression-matrix.json') $actualRegression
        $actualSummary = Read-Json (Join-Path $actualFocusedMismatch 'validation-summary.json')
        $actualSummary.focusedFixtureCount = 2
        $actualSummary.focusedFixtureSelectedCount = 2
        $actualSummary.focusedFixturePassedCount = 2
        Write-Json (Join-Path $actualFocusedMismatch 'validation-summary.json') $actualSummary
        $actualReport = Read-ReportContract (Join-Path $actualFocusedMismatch 'report.md')
        $actualReport.focusedValidationResult.selected = 2
        $actualReport.focusedValidationResult.passed = 2
        Write-ReportContract (Join-Path $actualFocusedMismatch 'report.md') $actualReport
        Update-PackageMetadata $actualFocusedMismatch
        $actualFocusedValidation = Invoke-ProductValidator $actualFocusedMismatch $repo

        $missingSourceEvidence = Copy-Artifact $preflight 'mutant-focused-source-missing'
        Remove-Item -LiteralPath (Join-Path $missingSourceEvidence 'focused-validation-result.json')
        Update-PackageMetadata $missingSourceEvidence
        $missingSourceValidation = Invoke-ProductValidator $missingSourceEvidence $repo

        $wrongSourceHash = Copy-Artifact $preflight 'mutant-focused-source-wrong-hash'
        $wrongHashRegression = Read-Json (Join-Path $wrongSourceHash 'finding-regression-matrix.json')
        $wrongHashRegression.finalFocusedValidationEvidence.sourceEvidenceSha256 = 'f' * 64
        Write-Json (Join-Path $wrongSourceHash 'finding-regression-matrix.json') $wrongHashRegression
        $wrongHashSummary = Read-Json (Join-Path $wrongSourceHash 'validation-summary.json')
        $wrongHashSummary.focusedFixtureEvidenceSha256 = 'f' * 64
        Write-Json (Join-Path $wrongSourceHash 'validation-summary.json') $wrongHashSummary
        Update-PackageMetadata $wrongSourceHash
        $wrongSourceHashValidation = Invoke-ProductValidator $wrongSourceHash $repo

        $wrongSourceLength = Copy-Artifact $preflight 'mutant-focused-source-wrong-length'
        $wrongLengthRegression = Read-Json (Join-Path $wrongSourceLength 'finding-regression-matrix.json')
        $wrongLengthRegression.finalFocusedValidationEvidence.sourceEvidenceLength++
        Write-Json (Join-Path $wrongSourceLength 'finding-regression-matrix.json') $wrongLengthRegression
        $wrongLengthSummary = Read-Json (Join-Path $wrongSourceLength 'validation-summary.json')
        $wrongLengthSummary.focusedFixtureEvidenceLength++
        Write-Json (Join-Path $wrongSourceLength 'validation-summary.json') $wrongLengthSummary
        Update-PackageMetadata $wrongSourceLength
        $wrongSourceLengthValidation = Invoke-ProductValidator $wrongSourceLength $repo

        $tamperedSource = Copy-Artifact $preflight 'mutant-focused-source-content-tampered'
        Add-Content -LiteralPath (Join-Path $tamperedSource 'focused-validation-result.json') `
            -Value ' ' -Encoding utf8NoBOM
        Update-PackageMetadata $tamperedSource
        $tamperedSourceValidation = Invoke-ProductValidator $tamperedSource $repo

        $sourceLink = Copy-Artifact $preflight 'mutant-focused-source-link'
        $sourceLinkPath = Join-Path $sourceLink 'focused-validation-result.json'
        $sourceLinkTarget = Join-Path $fixtureRoot 'focused-source-link-target.json'
        Copy-Item -LiteralPath $sourceLinkPath -Destination $sourceLinkTarget
        Remove-Item -LiteralPath $sourceLinkPath
        $sourceLinkValidation = $null
        try {
            [void](New-Item -ItemType SymbolicLink -Path $sourceLinkPath -Target $sourceLinkTarget)
            $sourceLinkValidation = Invoke-ProductValidator $sourceLink $repo
        }
        catch {
            $sourceLinkZip = Join-Path $fixtureRoot 'mutant-focused-source-link.zip'
            New-ZipFromDirectory $preflight $sourceLinkZip
            Add-Type -AssemblyName System.IO.Compression
            $sourceLinkArchive = [System.IO.Compression.ZipFile]::Open(
                $sourceLinkZip,
                [System.IO.Compression.ZipArchiveMode]::Update
            )
            try {
                $sourceLinkEntry = $sourceLinkArchive.GetEntry('focused-validation-result.json')
                $sourceLinkEntry.ExternalAttributes = -1610612736
            }
            finally { $sourceLinkArchive.Dispose() }
            $sourceLinkValidation = Invoke-ProductValidator $sourceLinkZip $repo
        }

        $sourceSelectedMismatch = Copy-Artifact $preflight 'mutant-focused-source-selected-mismatch'
        $selectedSource = Read-Json (Join-Path $sourceSelectedMismatch 'focused-validation-result.json')
        $selectedSource.selected = 2
        Write-Json (Join-Path $sourceSelectedMismatch 'focused-validation-result.json') $selectedSource
        Sync-FocusedValidationSourceBinding $sourceSelectedMismatch
        Update-PackageMetadata $sourceSelectedMismatch
        $sourceSelectedValidation = Invoke-ProductValidator $sourceSelectedMismatch $repo

        $sourcePassedMismatch = Copy-Artifact $preflight 'mutant-focused-source-passed-mismatch'
        $passedSource = Read-Json (Join-Path $sourcePassedMismatch 'focused-validation-result.json')
        $passedSource.passed = 0
        Write-Json (Join-Path $sourcePassedMismatch 'focused-validation-result.json') $passedSource
        Sync-FocusedValidationSourceBinding $sourcePassedMismatch
        Update-PackageMetadata $sourcePassedMismatch
        $sourcePassedValidation = Invoke-ProductValidator $sourcePassedMismatch $repo

        $sourceFailedNonzero = Copy-Artifact $preflight 'mutant-focused-source-failed-nonzero'
        $failedSource = Read-Json (Join-Path $sourceFailedNonzero 'focused-validation-result.json')
        $failedSource.failed = 1
        Write-Json (Join-Path $sourceFailedNonzero 'focused-validation-result.json') $failedSource
        Sync-FocusedValidationSourceBinding $sourceFailedNonzero
        Update-PackageMetadata $sourceFailedNonzero
        $sourceFailedValidation = Invoke-ProductValidator $sourceFailedNonzero $repo

        $sourceIdOmittedFromMatrix = Copy-Artifact $preflight 'mutant-focused-source-id-omitted-from-matrix'
        $omittedSource = Read-Json (Join-Path $sourceIdOmittedFromMatrix 'focused-validation-result.json')
        $omittedSource.results = @($omittedSource.results + [pscustomobject]@{
                id='FCH-SOURCE-ONLY-OMITTED'; result='PASS'; evidence='Source-only regression row.'
            })
        $omittedSource.selected = 2
        $omittedSource.passed = 2
        Write-Json (Join-Path $sourceIdOmittedFromMatrix 'focused-validation-result.json') $omittedSource
        Sync-FocusedValidationSourceBinding $sourceIdOmittedFromMatrix -Counts
        $omittedReport = Read-ReportContract (Join-Path $sourceIdOmittedFromMatrix 'report.md')
        $omittedReport.focusedValidationResult.selected = 2
        $omittedReport.focusedValidationResult.passed = 2
        Write-ReportContract (Join-Path $sourceIdOmittedFromMatrix 'report.md') $omittedReport
        Update-PackageMetadata $sourceIdOmittedFromMatrix
        $sourceIdOmittedValidation = Invoke-ProductValidator $sourceIdOmittedFromMatrix $repo

        $regressionIdMissingFromSource = Copy-Artifact $preflight 'mutant-regression-id-missing-from-source'
        $missingIdSource = Read-Json (Join-Path $regressionIdMissingFromSource 'focused-validation-result.json')
        $missingIdSource.results[0].id = 'FCH-STALE-SOURCE-ID'
        Write-Json (Join-Path $regressionIdMissingFromSource 'focused-validation-result.json') $missingIdSource
        Sync-FocusedValidationSourceBinding $regressionIdMissingFromSource
        Update-PackageMetadata $regressionIdMissingFromSource
        $regressionIdMissingValidation = Invoke-ProductValidator $regressionIdMissingFromSource $repo

        $duplicateSourceId = Copy-Artifact $preflight 'mutant-focused-source-duplicate-id'
        $duplicateSource = Read-Json (Join-Path $duplicateSourceId 'focused-validation-result.json')
        $duplicateSource.results = @($duplicateSource.results + $duplicateSource.results[0])
        $duplicateSource.selected = 2
        $duplicateSource.passed = 2
        Write-Json (Join-Path $duplicateSourceId 'focused-validation-result.json') $duplicateSource
        Sync-FocusedValidationSourceBinding $duplicateSourceId
        Update-PackageMetadata $duplicateSourceId
        $duplicateSourceValidation = Invoke-ProductValidator $duplicateSourceId $repo

        $staleSourceEvidence = Copy-Artifact $preflight 'mutant-stale-prior-focused-source'
        $staleSource = Read-Json (Join-Path $staleSourceEvidence 'focused-validation-result.json')
        $staleSource.results[0].id = 'FCH-STALE-PRIOR-ROUND'
        $staleSource.results[0].evidence = 'Stale prior-round source evidence.'
        Write-Json (Join-Path $staleSourceEvidence 'focused-validation-result.json') $staleSource
        Sync-FocusedValidationSourceBinding $staleSourceEvidence
        Update-PackageMetadata $staleSourceEvidence
        $staleSourceValidation = Invoke-ProductValidator $staleSourceEvidence $repo

        Add-Case 'FCH-FINAL-VALIDATION-EVIDENCE-PARITY-CONTRACT-PASS' (
            [string]$preflightReportContract.artifactLifecycleState -ceq 'ZIP_FREE_READY_TO_EXECUTE' -and
            [string]$preflightReportContract.currentFeaturePatch.sha256 -ceq (Get-Hash (Join-Path $preflight 'current-delta.patch')) -and
            [string]$preflightReportContract.correctionOnlyPatch.sha256 -ceq (Get-Hash (Join-Path $preflight 'correction-only.patch')) -and
            [int]$preflightValidationSummary.focusedFixtureCount -eq 1 -and
            [int]$preflightValidationSummary.focusedFixtureSelectedCount -eq 1 -and
            [int]$preflightValidationSummary.focusedFixturePassedCount -eq 1 -and
            [int]$preflightReportContract.focusedValidationResult.selected -eq 1 -and
            [int]$preflightReportContract.focusedValidationResult.passed -eq 1 -and
            [int]$preflightRegressionMatrix.finalFocusedValidationEvidence.selected -eq 1 -and
            [int]$preflightRegressionMatrix.finalFocusedValidationEvidence.passed -eq 1 -and
            $staleFocusedValidation.ExitCode -ne 0 -and
            $selectedPassedValidation.ExitCode -ne 0 -and
            $actualFocusedValidation.ExitCode -ne 0 -and
            $missingSourceValidation.ExitCode -ne 0 -and
            $wrongSourceHashValidation.ExitCode -ne 0 -and
            $wrongSourceLengthValidation.ExitCode -ne 0 -and
            $tamperedSourceValidation.ExitCode -ne 0 -and
            $sourceLinkValidation.ExitCode -ne 0 -and
            $sourceSelectedValidation.ExitCode -ne 0 -and
            $sourcePassedValidation.ExitCode -ne 0 -and
            $sourceFailedValidation.ExitCode -ne 0 -and
            $sourceIdOmittedValidation.ExitCode -ne 0 -and
            $regressionIdMissingValidation.ExitCode -ne 0 -and
            $duplicateSourceValidation.ExitCode -ne 0 -and
            $staleSourceValidation.ExitCode -ne 0
        ) (($staleFocusedValidation.Output, $selectedPassedValidation.Output,
                $actualFocusedValidation.Output, $missingSourceValidation.Output,
                $wrongSourceHashValidation.Output, $wrongSourceLengthValidation.Output,
                $tamperedSourceValidation.Output, $sourceLinkValidation.Output,
                $sourceSelectedValidation.Output, $sourcePassedValidation.Output,
                $sourceFailedValidation.Output, $sourceIdOmittedValidation.Output,
                $regressionIdMissingValidation.Output, $duplicateSourceValidation.Output,
                $staleSourceValidation.Output) -join "`n")

        Add-Case 'FCH-REFERENCE-ONLY-UNCHANGED-PASS' ((Invoke-ProductValidator $preflight $repo).ExitCode -eq 0)
        Add-Case 'FCH-REV001-CURRENT-STATE-REGRESSION-PRESERVED' ($preflightRun.ExitCode -eq 0)

        $twoFinding = Copy-Artifact $preflight 'two-finding-parity-positive'
        Convert-ToTwoFindingParityFixture $twoFinding
        $twoFindingValidation = Invoke-ProductValidator $twoFinding $repo
        Add-Case 'FCH-PER-FINDING-TWO-FINDING-CONTRACT-PASS' `
            ($twoFindingValidation.ExitCode -eq 0) $twoFindingValidation.Output

        $unionPreservingSwap = Copy-Artifact $twoFinding 'mutant-per-finding-union-preserving-swap'
        $swapCorrection = Read-Json (Join-Path $unionPreservingSwap 'finding-correction-matrix.json')
        $swapCorrection.findings[0].regressionTestIds = @('FCH-PER-FINDING-CONTRACT-B')
        $swapCorrection.findings[1].regressionTestIds = @('FCH-PER-FINDING-CONTRACT-A')
        Write-Json (Join-Path $unionPreservingSwap 'finding-correction-matrix.json') $swapCorrection
        Update-PackageMetadata $unionPreservingSwap
        $unionPreservingSwapValidation = Invoke-ProductValidator $unionPreservingSwap $repo
        Add-Case 'FCH-PER-FINDING-UNION-PRESERVING-REGRESSION-SWAP-REJECTED' `
            ($unionPreservingSwapValidation.ExitCode -ne 0) $unionPreservingSwapValidation.Output
    }
    Add-Case 'FCH-PUBLICATION-EVIDENCE-OPTIONAL-COMPATIBILITY-PASS' ($preflightRun.ExitCode -eq 0)

    $publicationSource = Copy-Artifact $commitSource 'publication-source'
    Add-PublicationEvidenceToSource $publicationSource 'BL339-REV-003'
    $publicationPreflight = Join-Path $fixtureRoot 'publication-preflight'
    $publicationRun = Invoke-Generator $publicationSource $repo 'BL-339' $commitContract.CurrentPaths $publicationPreflight Preflight
    Add-Case 'FCH-PUBLICATION-EVIDENCE-VALID-PASS' ($publicationRun.ExitCode -eq 0) $publicationRun.Output

    foreach ($publicationCase in @(
            [ordered]@{ Id='FCH-PUBLICATION-RESULT-MISSING-REJECTED'; Mutate={ param($d) Remove-Item -LiteralPath (Join-Path $d 'publication-regression-result.json') } },
            [ordered]@{ Id='FCH-PUBLICATION-EVIDENCE-MISSING-REJECTED'; Mutate={ param($d) Remove-Item -LiteralPath (Join-Path $d 'publication-regression-evidence.json') } },
            [ordered]@{ Id='FCH-PUBLICATION-RESULT-WRONG-SHA-REJECTED'; Mutate={ param($d) $e=Read-Json(Join-Path $d 'publication-regression-evidence.json');$e.resultSha256='f'*64;Write-Json(Join-Path $d 'publication-regression-evidence.json')$e;Set-PublicationEvidenceHashBindings $d (Get-Hash(Join-Path $d 'publication-regression-evidence.json')) } },
            [ordered]@{ Id='FCH-PUBLICATION-RESULT-FAIL-REJECTED'; Mutate={ param($d) $r=Read-Json(Join-Path $d 'publication-regression-result.json');$r.status='FAIL';$r.results[0].result='FAIL';$r.passed=$r.selected-1;$r.failed=1;Write-Json(Join-Path $d 'publication-regression-result.json')$r;Sync-PublicationResultBinding $d } },
            [ordered]@{ Id='FCH-PUBLICATION-RESULT-SELECTED-COUNT-REJECTED'; Mutate={ param($d) $r=Read-Json(Join-Path $d 'publication-regression-result.json');$r.selected=$r.selected-1;Write-Json(Join-Path $d 'publication-regression-result.json')$r;Sync-PublicationResultBinding $d } },
            [ordered]@{ Id='FCH-PUBLICATION-RESULT-PASSED-COUNT-REJECTED'; Mutate={ param($d) $r=Read-Json(Join-Path $d 'publication-regression-result.json');$r.passed=$r.passed-1;Write-Json(Join-Path $d 'publication-regression-result.json')$r;Sync-PublicationResultBinding $d } },
            [ordered]@{ Id='FCH-PUBLICATION-RESULT-FAILED-COUNT-REJECTED'; Mutate={ param($d) $r=Read-Json(Join-Path $d 'publication-regression-result.json');$r.failed=1;Write-Json(Join-Path $d 'publication-regression-result.json')$r;Sync-PublicationResultBinding $d } },
            [ordered]@{ Id='FCH-PUBLICATION-RESULT-DUPLICATE-CASE-REJECTED'; Mutate={ param($d) $r=Read-Json(Join-Path $d 'publication-regression-result.json');$r.results[1].id=$r.results[0].id;Write-Json(Join-Path $d 'publication-regression-result.json')$r;Sync-PublicationResultBinding $d } },
            [ordered]@{ Id='FCH-PUBLICATION-RESULT-MISSING-CANONICAL-CASE-REJECTED'; Mutate={ param($d) $r=Read-Json(Join-Path $d 'publication-regression-result.json');$r.results=@($r.results|Select-Object -Skip 1);$r.selected=$r.results.Count;$r.passed=$r.results.Count;Write-Json(Join-Path $d 'publication-regression-result.json')$r;Sync-PublicationResultBinding $d } },
            [ordered]@{ Id='FCH-PUBLICATION-RESULT-FOREIGN-CASE-REJECTED'; Mutate={ param($d) $r=Read-Json(Join-Path $d 'publication-regression-result.json');$r.results=@($r.results+[pscustomobject]@{id='GHP-FOREIGN-CASE';result='PASS';evidence='foreign'});$r.selected=$r.results.Count;$r.passed=$r.results.Count;Write-Json(Join-Path $d 'publication-regression-result.json')$r;Sync-PublicationResultBinding $d } },
            [ordered]@{ Id='FCH-PUBLICATION-CONSISTENTLY-REDUCED-CASE-SET-REJECTED'; Mutate={ param($d) $r=Read-Json(Join-Path $d 'publication-regression-result.json');$r.results=@($r.results|Select-Object -Skip 1);$r.selected=$r.results.Count;$r.passed=$r.results.Count;Write-Json(Join-Path $d 'publication-regression-result.json')$r;Set-AllPublicationCaseIds $d @($r.results.id);Sync-PublicationResultBinding $d } },
            [ordered]@{ Id='FCH-PUBLICATION-MISSING-CANONICAL-SOURCE-REJECTED'; Mutate={ param($d) $e=Read-Json(Join-Path $d 'publication-regression-evidence.json');$e.sourceBindings=@($e.sourceBindings|Select-Object -Skip 1);Write-Json(Join-Path $d 'publication-regression-evidence.json')$e;Set-PublicationEvidenceHashBindings $d (Get-Hash(Join-Path $d 'publication-regression-evidence.json')) } },
            [ordered]@{ Id='FCH-PUBLICATION-MISSING-CANONICAL-DEPENDENCY-REJECTED'; Mutate={ param($d) $e=Read-Json(Join-Path $d 'publication-regression-evidence.json');$e.dependencyBindings=@($e.dependencyBindings|Select-Object -Skip 1);Write-Json(Join-Path $d 'publication-regression-evidence.json')$e;Set-PublicationEvidenceHashBindings $d (Get-Hash(Join-Path $d 'publication-regression-evidence.json')) } },
            [ordered]@{ Id='FCH-PUBLICATION-EXTRA-SOURCE-REJECTED'; Mutate={ param($d) $e=Read-Json(Join-Path $d 'publication-regression-evidence.json');$e.sourceBindings=@($e.sourceBindings+[pscustomobject]@{path='BACKLOG.md';sha256=Get-Hash(Join-Path (Split-Path -Parent $PSScriptRoot) 'BACKLOG.md')});Write-Json(Join-Path $d 'publication-regression-evidence.json')$e;Set-PublicationEvidenceHashBindings $d (Get-Hash(Join-Path $d 'publication-regression-evidence.json')) } },
            [ordered]@{ Id='FCH-PUBLICATION-STALE-SOURCE-SHA-REJECTED'; Mutate={ param($d) $e=Read-Json(Join-Path $d 'publication-regression-evidence.json');$e.sourceBindings[0].sha256='f'*64;Write-Json(Join-Path $d 'publication-regression-evidence.json')$e;Set-PublicationEvidenceHashBindings $d (Get-Hash(Join-Path $d 'publication-regression-evidence.json')) } },
            [ordered]@{ Id='FCH-PUBLICATION-STALE-DEPENDENCY-SHA-REJECTED'; Mutate={ param($d) $e=Read-Json(Join-Path $d 'publication-regression-evidence.json');$e.dependencyBindings[0].sha256='f'*64;Write-Json(Join-Path $d 'publication-regression-evidence.json')$e;Set-PublicationEvidenceHashBindings $d (Get-Hash(Join-Path $d 'publication-regression-evidence.json')) } },
            [ordered]@{ Id='FCH-PUBLICATION-MATRIX-DEFINITION-SHA-REJECTED'; Mutate={ param($d) $e=Read-Json(Join-Path $d 'publication-regression-evidence.json');$e.matrixDefinitionSha256='f'*64;Write-Json(Join-Path $d 'publication-regression-evidence.json')$e;Set-PublicationEvidenceHashBindings $d (Get-Hash(Join-Path $d 'publication-regression-evidence.json')) } },
            [ordered]@{ Id='FCH-PUBLICATION-WRONG-MATRIX-ID-REJECTED'; Mutate={ param($d) $e=Read-Json(Join-Path $d 'publication-regression-evidence.json');$e.matrixId='FOREIGN_MATRIX';Write-Json(Join-Path $d 'publication-regression-evidence.json')$e;Set-PublicationEvidenceHashBindings $d (Get-Hash(Join-Path $d 'publication-regression-evidence.json')) } },
            [ordered]@{ Id='FCH-PUBLICATION-RESULT-DUPLICATE-PROPERTY-REJECTED'; Mutate={ param($d) $p=Join-Path $d 'publication-regression-result.json';$t=[IO.File]::ReadAllText($p,$utf8);$t=$t -replace '"schemaVersion": 2,',('"schemaVersion": 2,'+"`n"+'  "schemaVersion": 2,');Write-Utf8 $p $t;Sync-PublicationResultBinding $d } },
            [ordered]@{ Id='FCH-PUBLICATION-RESULT-BOM-REJECTED'; Mutate={ param($d) $p=Join-Path $d 'publication-regression-result.json';$b=[IO.File]::ReadAllBytes($p);[IO.File]::WriteAllBytes($p,[byte[]](0xEF,0xBB,0xBF)+$b);Sync-PublicationResultBinding $d } },
            [ordered]@{ Id='FCH-PUBLICATION-RESULT-INVALID-UTF8-REJECTED'; Mutate={ param($d) $p=Join-Path $d 'publication-regression-result.json';[IO.File]::WriteAllBytes($p,[byte[]](0x7B,0x22,0x78,0x22,0x3A,0x22,0xFF,0x22,0x7D));Sync-PublicationResultBinding $d } },
            [ordered]@{ Id='FCH-PUBLICATION-RESULT-TRAILING-JSON-REJECTED'; Mutate={ param($d) $p=Join-Path $d 'publication-regression-result.json';$t=[IO.File]::ReadAllText($p,$utf8);Write-Utf8 $p ($t+'{}');Sync-PublicationResultBinding $d } },
            [ordered]@{ Id='FCH-PUBLICATION-EVIDENCE-LEADING-CONTRACT-PARITY-REJECTED'; Mutate={ param($d) $r=Read-ReportContract(Join-Path $d 'report.md');$r.publicationRegressionEvidence[0].matrixId='FOREIGN_MATRIX';Write-ReportContract(Join-Path $d 'report.md')$r } }
        )) {
        $publicationMutant = Copy-Artifact $publicationPreflight ('mutant-' + $publicationCase.Id.ToLowerInvariant())
        & $publicationCase.Mutate $publicationMutant
        Update-PackageMetadata $publicationMutant
        $publicationValidation = Invoke-ProductValidator $publicationMutant $repo
        Add-Case $publicationCase.Id ($publicationValidation.ExitCode -ne 0) $publicationValidation.Output
    }

    foreach ($staleResultCase in @(
            [ordered]@{ Id='OLD_PASS_RESULT_PLUS_CHANGED_SOURCE_WITH_FRESH_EVIDENCE_REJECTED'; Kind='sourcePaths' },
            [ordered]@{ Id='OLD_PASS_RESULT_PLUS_CHANGED_DEPENDENCY_WITH_FRESH_EVIDENCE_REJECTED'; Kind='dependencyPaths' },
            [ordered]@{ Id='OLD_PASS_RESULT_PLUS_CHANGED_CATALOG_WITH_FRESH_EVIDENCE_REJECTED'; Kind='catalog' }
        )) {
        $contractRoot = New-PublicationContractRoot ('contract-' + $staleResultCase.Kind.ToLowerInvariant())
        $staleMutant = Copy-Artifact $publicationPreflight ('mutant-' + $staleResultCase.Id.ToLowerInvariant())
        $unchangedResultHash = Get-Hash (Join-Path $staleMutant 'publication-regression-result.json')
        $catalog = Read-Json (Join-Path $contractRoot 'Governance/publication-regression-matrix-catalog.json')
        $matrix = @($catalog.matrices | Where-Object matrixId -CEQ 'GOVERNANCE_HANDOFF_PUBLICATION_FIXTURES')[0]
        if ($staleResultCase.Kind -ceq 'catalog') {
            $targetPath = Join-Path $contractRoot 'Governance/publication-regression-matrix-catalog.json'
            Add-Content -LiteralPath $targetPath -Value '' -Encoding utf8NoBOM
        }
        else {
            $relativePath = [string]@($matrix.($staleResultCase.Kind))[0]
            Add-Content -LiteralPath (Join-Path $contractRoot $relativePath) `
                -Value "# deterministic post-result input drift" -Encoding utf8NoBOM
        }
        Refresh-PublicationEvidenceForContractRoot $staleMutant $contractRoot
        Update-PackageMetadata $staleMutant
        $staleValidation = Invoke-ProductValidator $staleMutant $repo $contractRoot
        Add-Case $staleResultCase.Id (
            $staleValidation.ExitCode -ne 0 -and
            (Get-Hash (Join-Path $staleMutant 'publication-regression-result.json')) -ceq $unchangedResultHash
        ) $staleValidation.Output
    }

    Add-Case 'CURRENT_RESULT_PLUS_CURRENT_EVIDENCE_AND_INPUTS_PASS' `
        ((Invoke-ProductValidator $publicationPreflight $repo).ExitCode -eq 0)

    if (-not $SkipPublicationExecutionInputDrift) {
        foreach ($bindingDriftCase in @(
                [ordered]@{ Id='FCH-PARENT-BOUND-RUNNER-CHANGED-BEFORE-RUNNER-START-REJECTED'; Phase='PARENT_BINDING_CAPTURED'; Target='RUNNER' },
                [ordered]@{ Id='FCH-PARENT-BOUND-DEPENDENCY-CHANGED-BEFORE-RUNNER-INITIAL-CHECK-REJECTED'; Phase='PARENT_BINDING_CAPTURED'; Target='DEPENDENCY' },
                [ordered]@{ Id='FCH-RUNNER-CHANGED-AFTER-IMPORT-BEFORE-CASE1-REJECTED'; Phase='RUNNER_IMPORTS_COMPLETED'; Target='RUNNER' },
                [ordered]@{ Id='FCH-DEPENDENCY-CHANGED-AFTER-IMPORT-BEFORE-CASE1-REJECTED'; Phase='RUNNER_IMPORTS_COMPLETED'; Target='DEPENDENCY' }
            )) {
            $bindingDrift = Invoke-PublicationExecutionInputDriftCase `
                -Id $bindingDriftCase.Id -PausePhase $bindingDriftCase.Phase -TargetKind $bindingDriftCase.Target
            Add-Case $bindingDriftCase.Id $bindingDrift.Passed $bindingDrift.Evidence
        }
    }

    if ($PublicationEvidenceOnly) {
        $status = 'PASS'
        return
    }

    $finalContent = Join-Path $fixtureRoot 'commit-final-content'
    $finalRun = Invoke-Generator $commitSource $repo 'BL-339' $commitContract.CurrentPaths $finalContent FinalContent
    Add-Case 'FCH-FINAL-CONTENT-PRODUCT-VALIDATOR-PASS' ($finalRun.ExitCode -eq 0) $finalRun.Output
    $finalCompletion = Read-Json (Join-Path $finalContent 'completion-report.json')
    $finalReportContract = Read-ReportContract (Join-Path $finalContent 'report.md')
    Add-Case 'FCH-COMPLETE-FINAL-REPORT-CONTRACT-PASS' (
        [string]$finalReportContract.artifactLifecycleState -ceq 'FINAL_REVIEW_PACKAGE' -and
        [bool]$finalReportContract.classicReviewReady -and
        [int]$finalReportContract.packageWriteAttemptCount -eq 1
    )
    Add-Case 'FCH-FINAL-REPORT-SUBJECT-DATA-PRESERVED' (
        (Get-ReportSubjectJson $preflightReportContract) -ceq (Get-ReportSubjectJson $finalReportContract)
    )
    Add-Case 'FCH-FINAL-CONTENT-SEMANTICS-PASS' ([bool]$finalCompletion.classicReviewReady -and [int]$finalCompletion.packageWriteAttemptCount -eq 1 -and [string]$finalCompletion.nextAction -ceq 'FOCUSED_INDEPENDENT_DELTA_REVIEW')

    $syntheticZip = Join-Path $fixtureRoot 'synthetic-final.zip'
    $zipRun = Invoke-Generator $commitSource $repo 'BL-339' $commitContract.CurrentPaths $syntheticZip Zip
    Add-Case 'FCH-SYNTHETIC-FINAL-ZIP-PASS' ($zipRun.ExitCode -eq 0 -and (Test-Path -LiteralPath $syntheticZip)) $zipRun.Output
    $zipValidation = Invoke-ProductValidator $syntheticZip $repo
    Add-Case 'FCH-SYNTHETIC-FINAL-ZIP-REOPEN-PASS' ($zipValidation.ExitCode -eq 0) $zipValidation.Output
    Add-Case 'FCH-SYNTHETIC-FINAL-ZIP-FULL-REPORT-CONTRACT-PASS' ($zipValidation.ExitCode -eq 0) $zipValidation.Output
    Add-Case 'FCH-REV002-FIRST-WRITE-ATTEMPT-REGRESSION-PRESERVED' ($zipRun.Output -match 'PackageWriteAttemptCount\s*:\s*1') $zipRun.Output
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $zipArchive = [System.IO.Compression.ZipFile]::OpenRead($syntheticZip)
    try { $entry=$zipArchive.GetEntry('completion-report.json'); $reader=[System.IO.StreamReader]::new($entry.Open(),$utf8); try{$zipCompletion=$reader.ReadToEnd()|ConvertFrom-Json}finally{$reader.Dispose()} }
    finally { $zipArchive.Dispose() }
    Add-Case 'FCH-SYNTHETIC-ZIP-INTERNAL-FINAL-STATE' ([string]$zipCompletion.artifactLifecycleState -ceq 'FINAL_REVIEW_PACKAGE' -and [bool]$zipCompletion.classicReviewReady)

    $snapshotSource = Join-Path $fixtureRoot 'snapshot-source'
    $snapshotContract = New-SyntheticSource -Directory $snapshotSource -RepositoryRoot $repo -TaskId 'BL-339' -FindingId 'BL339-REV-004' -BaselineCommit $baseline -PreviousCommit $previousCommit -PreviousTree $previousTree -CurrentPatch $currentPatchBytes -CorrectionPatch $correctionPatchBytes -CurrentEntries $currentEntries -CorrectionEntries $correctionEntries -CurrentTree $currentTree -PreviousMode IMMUTABLE_REVIEW_PACKAGE -HistoricalPackagePath $historicalZip -HistoricalBinding $historicalBinding
    $snapshotPreflight = Join-Path $fixtureRoot 'snapshot-preflight'
    $snapshotRun = Invoke-Generator $snapshotSource $repo 'BL-339' $snapshotContract.CurrentPaths $snapshotPreflight Preflight
    Add-Case 'FCH-IMMUTABLE-PACKAGE-PRODUCT-VALIDATOR-PASS' ($snapshotRun.ExitCode -eq 0) $snapshotRun.Output
    Add-Case 'FCH-HISTORICAL-RENAME-SNAPSHOT-PASS' ($snapshotRun.ExitCode -eq 0) $snapshotRun.Output
    Add-Case 'FCH-HISTORICAL-MULTI-ENTRY-PLUS-RENAME-PASS' ($snapshotRun.ExitCode -eq 0) $snapshotRun.Output
    Add-Case 'FCH-EXPANDED-CORRECTION-SCOPE-OVER-HISTORICAL-REVIEW-PASS' `
        ($snapshotRun.ExitCode -eq 0 -and $currentPaths.Count -gt $historicalBoundPaths.Count) $snapshotRun.Output

    $referencePreviousPostimage = Get-TreePostimage -Root $repo -Tree $previousTree -Path 'reference.txt'
    $renamePreviousPostimage = Get-TreePostimage -Root $repo -Tree $previousTree -Path 'historical-target.txt'
    $renameBaselinePreimage = Get-TreePostimage -Root $repo -Tree $baseline -Path 'historical-source.txt'
    $genericHistoricalScopeValue = [ordered]@{
        schemaVersion=1; taskId='BL-339'; profile='IMPLEMENTATION_TO_INDEPENDENT_FULL_REVIEW'
        repository='https://github.com/thomasweidner/flashgate-mcp.git'
        baselineCommit=$baseline; currentCommit=$previousCommit; branch='fixture'
        allowedDeltaPaths=@('historical-source.txt', 'historical-target.txt', 'reference.txt')
        excludedDeltaPaths=@()
        entries=@(
            [ordered]@{
                path='reference.txt'; gitStatus='TRACKED_MODIFIED'; tracked=$true; staged=$false
                postimage=[ordered]@{
                    mode=[string]$referencePreviousPostimage.mode; modeSource='GIT_WORKTREE'
                    length=[int64]$referencePreviousPostimage.length; sha256=[string]$referencePreviousPostimage.sha256
                }
                inclusionDecision='INCLUDE'; reason='Synthetic generic previous-review modification.'
            },
            [ordered]@{
                previousPath='historical-source.txt'; path='historical-target.txt'
                gitStatus='TRACKED_RENAMED'; tracked=$true; staged=$false
                preimage=[ordered]@{
                    commit=$baseline; mode=[string]$renameBaselinePreimage.mode; modeSource='BASELINE_TREE'
                    length=[int64]$renameBaselinePreimage.length; sha256=[string]$renameBaselinePreimage.sha256
                }
                postimage=[ordered]@{
                    mode=[string]$renamePreviousPostimage.mode; modeSource='GIT_WORKTREE'
                    length=[int64]$renamePreviousPostimage.length; sha256=[string]$renamePreviousPostimage.sha256
                }
                inclusionDecision='INCLUDE'; reason='Synthetic generic previous-review rename.'
            }
        )
        hostPathPolicy=[ordered]@{ hostPathFreeArtifacts=@(); allowedReferences=@() }
    }
    $genericHistorical = New-GenericHistoricalPackageVariant -Name 'valid-no-pathcount' `
        -HistoricalPatchPath (Join-Path $historicalDir 'current-delta.patch') `
        -Scope $genericHistoricalScopeValue -BaselineCommit $baseline -CurrentCommit $previousCommit
    $genericHistoricalBinding = [pscustomobject][ordered]@{
        type='IMMUTABLE_REVIEW_PACKAGE'; historicalPackagePath=$genericHistorical.Zip
        historicalPackageSha256=$genericHistorical.PackageSha256
        historicalManifestSha256=$genericHistorical.ManifestSha256
        historicalPatchSha256=$genericHistorical.PatchSha256
        historicalScopeInventorySha256=$genericHistorical.ScopeSha256
        previousReviewedBaselineCommit=$baseline; previousReviewedTree=$previousTree
        previousReviewedPathCount=2; previousReviewedPostimages=$postimages
    }
    $genericHistoricalSource = Join-Path $fixtureRoot 'generic-historical-valid-source'
    $genericHistoricalContract = New-SyntheticSource -Directory $genericHistoricalSource `
        -RepositoryRoot $repo -TaskId 'BL-339' -FindingId 'BL339-REV-004' `
        -BaselineCommit $baseline -PreviousCommit $previousCommit -PreviousTree $previousTree `
        -CurrentPatch $currentPatchBytes -CorrectionPatch $correctionPatchBytes `
        -CurrentEntries $currentEntries -CorrectionEntries $correctionEntries -CurrentTree $currentTree `
        -PreviousMode IMMUTABLE_REVIEW_PACKAGE -HistoricalPackagePath $genericHistorical.Zip `
        -HistoricalBinding $genericHistoricalBinding
    $genericHistoricalPreflight = Join-Path $fixtureRoot 'generic-historical-valid-preflight'
    $genericHistoricalRun = Invoke-Generator $genericHistoricalSource $repo 'BL-339' `
        $genericHistoricalContract.CurrentPaths $genericHistoricalPreflight Preflight
    Add-Case 'FCH-GENERIC-IMPLEMENTATION-PREVIOUS-WITHOUT-PATHCOUNT-PASS' `
        ($genericHistoricalRun.ExitCode -eq 0) $genericHistoricalRun.Output
    Add-Case 'FCH-BL340-GENERIC-SCOPE-INVENTORY-FORM-PASS' `
        ($genericHistoricalRun.ExitCode -eq 0) $genericHistoricalRun.Output
    Add-Case 'FCH-STRICTMODE-PATHCOUNT-ABSENCE-REGRESSION-CLOSED' `
        ($genericHistoricalRun.ExitCode -eq 0) $genericHistoricalRun.Output

    $genericHistoricalCases = @(
        [ordered]@{
            Id='FCH-GENERIC-PREVIOUS-MISSING-SEMANTIC-INVENTORY-REJECTED'
            Mutate={ param($directory)
                $scope=Read-Json(Join-Path $directory 'scope-inventory.json')
                $scope.PSObject.Properties.Remove('entries')
                Write-Json (Join-Path $directory 'scope-inventory.json') $scope
                $scopeHash=Get-Hash(Join-Path $directory 'scope-inventory.json')
                foreach($name in @('assignment-record.json','completion-report.json')){
                    $contract=Read-Json(Join-Path $directory $name);$contract.scopeInventorySha256=$scopeHash
                    Write-Json (Join-Path $directory $name) $contract
                }
            }
        },
        [ordered]@{
            Id='FCH-GENERIC-PREVIOUS-WRONG-PROFILE-REJECTED'
            Mutate={ param($directory)
                $assignment=Read-Json(Join-Path $directory 'assignment-record.json')
                $assignment.profile='GENERIC_COMMIT_PREPARATION'
                $assignment.transitionType='COMMIT_PREPARATION_TO_COMMIT_APPROVAL'
                $assignment.executionMode='COMMIT_PREPARATION';$assignment.checkpoint='PRE_COMMIT'
                foreach($name in @('fullCompletionEvidenceSha256','fullCompletionResultSha256','executionEnvelopeSha256')){$assignment.PSObject.Properties.Remove($name)}
                Write-Json (Join-Path $directory 'assignment-record.json') $assignment
                $completion=Read-Json(Join-Path $directory 'completion-report.json')
                $completion.profile='GENERIC_COMMIT_PREPARATION'
                $completion.transitionType='COMMIT_PREPARATION_TO_COMMIT_APPROVAL'
                foreach($name in @('independentReviewStatus','fullCompletionEvidenceSha256','fullCompletionResultSha256','executionEnvelopeSha256')){$completion.PSObject.Properties.Remove($name)}
                Write-Json (Join-Path $directory 'completion-report.json') $completion
                $scope=Read-Json(Join-Path $directory 'scope-inventory.json')
                $scope.profile='GENERIC_COMMIT_PREPARATION';Write-Json(Join-Path $directory 'scope-inventory.json')$scope
            }
        },
        [ordered]@{
            Id='FCH-GENERIC-PREVIOUS-UNKNOWN-PROFILE-REJECTED'
            Mutate={ param($directory)
                $assignment=Read-Json(Join-Path $directory 'assignment-record.json')
                $assignment.profile='UNKNOWN_PROFILE';Write-Json(Join-Path $directory 'assignment-record.json')$assignment
            }
        },
        [ordered]@{
            Id='FCH-GENERIC-PREVIOUS-ENTRIES-ALLOWED-MISMATCH-REJECTED'
            Mutate={ param($directory)
                $scope=Read-Json(Join-Path $directory 'scope-inventory.json')
                $scope.allowedDeltaPaths=@('reference.txt');Write-Json(Join-Path $directory 'scope-inventory.json')$scope
                $scopeHash=Get-Hash(Join-Path $directory 'scope-inventory.json')
                foreach($name in @('assignment-record.json','completion-report.json')){
                    $contract=Read-Json(Join-Path $directory $name);$contract.allowedDeltaPaths=@('reference.txt')
                    $contract.scopeInventorySha256=$scopeHash;Write-Json(Join-Path $directory $name)$contract
                }
            }
        },
        [ordered]@{
            Id='FCH-GENERIC-PREVIOUS-SCOPE-HASH-TAMPERING-REJECTED'
            Mutate={ param($directory)
                $scope=Read-Json(Join-Path $directory 'scope-inventory.json')
                $scope.entries[0].reason='Tampered after assignment binding.'
                Write-Json (Join-Path $directory 'scope-inventory.json') $scope
            }
        },
        [ordered]@{
            Id='FCH-GENERIC-PREVIOUS-PACKAGE-TAMPERING-REJECTED'
            Mutate={ param($directory)
                $bytes=[System.IO.File]::ReadAllBytes((Join-Path $directory 'current-delta.patch'))
                [System.IO.File]::WriteAllBytes((Join-Path $directory 'current-delta.patch'),[byte[]]($bytes+0x0A))
                [System.IO.File]::WriteAllBytes((Join-Path $directory 'task.patch'),[byte[]]($bytes+0x0A))
            }
        },
        [ordered]@{
            Id='FCH-GENERIC-PREVIOUS-CASE-COLLISION-REJECTED'
            Mutate={ param($directory)
                $scope=Read-Json(Join-Path $directory 'scope-inventory.json')
                $scope.allowedDeltaPaths=@($scope.allowedDeltaPaths+'REFERENCE.TXT')
                Write-Json(Join-Path $directory 'scope-inventory.json')$scope
                $scopeHash=Get-Hash(Join-Path $directory 'scope-inventory.json')
                foreach($name in @('assignment-record.json','completion-report.json')){
                    $contract=Read-Json(Join-Path $directory $name)
                    $contract.allowedDeltaPaths=@($contract.allowedDeltaPaths+'REFERENCE.TXT')
                    $contract.scopeInventorySha256=$scopeHash;Write-Json(Join-Path $directory $name)$contract
                }
            }
        }
    )
    foreach ($genericHistoricalCase in $genericHistoricalCases) {
        $scopeVariant = ($genericHistoricalScopeValue | ConvertTo-Json -Depth 30) | ConvertFrom-Json -Depth 30
        $variant = New-GenericHistoricalPackageVariant `
            -Name $genericHistoricalCase.Id.ToLowerInvariant() `
            -HistoricalPatchPath (Join-Path $historicalDir 'current-delta.patch') `
            -Scope $scopeVariant -BaselineCommit $baseline -CurrentCommit $previousCommit `
            -Mutate $genericHistoricalCase.Mutate
        $variantBinding = [pscustomobject][ordered]@{
            type='IMMUTABLE_REVIEW_PACKAGE'; historicalPackagePath=$variant.Zip
            historicalPackageSha256=$variant.PackageSha256; historicalManifestSha256=$variant.ManifestSha256
            historicalPatchSha256=$variant.PatchSha256; historicalScopeInventorySha256=$variant.ScopeSha256
            previousReviewedBaselineCommit=$baseline; previousReviewedTree=$previousTree
            previousReviewedPathCount=2; previousReviewedPostimages=$postimages
        }
        $variantSource = Join-Path $fixtureRoot ('source-' + $genericHistoricalCase.Id.ToLowerInvariant())
        $variantContract = New-SyntheticSource -Directory $variantSource -RepositoryRoot $repo `
            -TaskId 'BL-339' -FindingId 'BL339-REV-004' -BaselineCommit $baseline `
            -PreviousCommit $previousCommit -PreviousTree $previousTree -CurrentPatch $currentPatchBytes `
            -CorrectionPatch $correctionPatchBytes -CurrentEntries $currentEntries `
            -CorrectionEntries $correctionEntries -CurrentTree $currentTree `
            -PreviousMode IMMUTABLE_REVIEW_PACKAGE -HistoricalPackagePath $variant.Zip `
            -HistoricalBinding $variantBinding
        $variantPreflight = Join-Path $fixtureRoot ('preflight-' + $genericHistoricalCase.Id.ToLowerInvariant())
        $variantRun = Invoke-Generator $variantSource $repo 'BL-339' `
            $variantContract.CurrentPaths $variantPreflight Preflight
        Add-Case $genericHistoricalCase.Id ($variantRun.ExitCode -ne 0) $variantRun.Output
    }

    foreach ($historicalFailureCase in @(
            [ordered]@{ Id='FCH-HISTORICAL-DECLARED-ENTRY-COUNT-REJECTED'; Mutate={ param($scope) $scope.pathCount=3 } },
            [ordered]@{ Id='FCH-HISTORICAL-RENAME-SOURCE-SEMANTICS-REJECTED'; Mutate={ param($scope) $scope.entries[1].previousPath='wrong-source.txt' } },
            [ordered]@{ Id='FCH-HISTORICAL-RENAME-TARGET-SEMANTICS-REJECTED'; Mutate={ param($scope) $scope.entries[1].path='wrong-target.txt' } }
        )) {
        $scopeVariant = ($historicalScopeValue | ConvertTo-Json -Depth 20) | ConvertFrom-Json -Depth 20
        & $historicalFailureCase.Mutate $scopeVariant
        $variant = New-HistoricalPackageVariant -Name $historicalFailureCase.Id.ToLowerInvariant() `
            -HistoricalPatchPath (Join-Path $historicalDir 'current-delta.patch') -Scope $scopeVariant
        $variantBinding = [pscustomobject][ordered]@{
            type='IMMUTABLE_REVIEW_PACKAGE'; historicalPackagePath=$variant.Zip
            historicalPackageSha256=$variant.PackageSha256; historicalManifestSha256=$variant.ManifestSha256
            historicalPatchSha256=$variant.PatchSha256; historicalScopeInventorySha256=$variant.ScopeSha256
            previousReviewedBaselineCommit=$baseline; previousReviewedTree=$previousTree
            previousReviewedPathCount=[int]$scopeVariant.pathCount; previousReviewedPostimages=$postimages
        }
        $variantSource = Join-Path $fixtureRoot ('source-' + $historicalFailureCase.Id.ToLowerInvariant())
        $variantContract = New-SyntheticSource -Directory $variantSource -RepositoryRoot $repo -TaskId 'BL-339' `
            -FindingId 'BL339-REV-004' -BaselineCommit $baseline -PreviousCommit $previousCommit `
            -PreviousTree $previousTree -CurrentPatch $currentPatchBytes -CorrectionPatch $correctionPatchBytes `
            -CurrentEntries $currentEntries -CorrectionEntries $correctionEntries -CurrentTree $currentTree `
            -PreviousMode IMMUTABLE_REVIEW_PACKAGE -HistoricalPackagePath $variant.Zip -HistoricalBinding $variantBinding
        $variantPreflight = Join-Path $fixtureRoot ('preflight-' + $historicalFailureCase.Id.ToLowerInvariant())
        $variantRun = Invoke-Generator $variantSource $repo 'BL-339' $variantContract.CurrentPaths $variantPreflight Preflight
        Add-Case $historicalFailureCase.Id ($variantRun.ExitCode -ne 0) $variantRun.Output
    }

    foreach ($countCase in @(
            [ordered]@{ Id='FCH-HISTORICAL-SCOPE-JOINTLY-WRONG-COUNT-REJECTED'; Count=4 },
            [ordered]@{ Id='FCH-HISTORICAL-SCOPE-COUNT-TOO-HIGH-REJECTED'; Count=3 },
            [ordered]@{ Id='FCH-HISTORICAL-SCOPE-COUNT-TOO-LOW-REJECTED'; Count=1 }
        )) {
        $countMutant = Copy-Artifact $snapshotPreflight ('mutant-' + $countCase.Id.ToLowerInvariant())
        $previousJson = Read-Json (Join-Path $countMutant 'previous-review-binding.json')
        $previousJson.previousReviewState.previousReviewedPathCount = $countCase.Count
        Write-Json (Join-Path $countMutant 'previous-review-binding.json') $previousJson
        $focusedJson = Read-Json (Join-Path $countMutant 'focused-delta-review-record.json')
        $focusedJson.previousReviewState.previousReviewedPathCount = $countCase.Count
        Write-Json (Join-Path $countMutant 'focused-delta-review-record.json') $focusedJson
        $bindingHash = Get-Hash (Join-Path $countMutant 'previous-review-binding.json')
        $assignmentJson = Read-Json (Join-Path $countMutant 'assignment-record.json')
        $assignmentJson.previousReviewBindingSha256 = $bindingHash
        Write-Json (Join-Path $countMutant 'assignment-record.json') $assignmentJson
        $reportJson = Read-ReportContract (Join-Path $countMutant 'report.md')
        $reportJson.previousReview.bindingSha256 = $bindingHash
        Write-ReportContract (Join-Path $countMutant 'report.md') $reportJson
        Update-PackageMetadata $countMutant
        $countValidation = Invoke-ProductValidator $countMutant $repo
        Add-Case $countCase.Id ($countValidation.ExitCode -ne 0) $countValidation.Output
    }

    $historicalSource = Join-Path $fixtureRoot 'historical-commit-source'
    $historicalContract = New-SyntheticSource -Directory $historicalSource -RepositoryRoot $repo -TaskId 'BL-334' -FindingId 'BL333-BL334-REV-013' -BaselineCommit $baseline -PreviousCommit $previousCommit -PreviousTree $previousTree -CurrentPatch $currentPatchBytes -CorrectionPatch $correctionPatchBytes -CurrentEntries $currentEntries -CorrectionEntries $correctionEntries -CurrentTree $currentTree -PreviousMode COMMIT -HistoricalPackagePath $historicalZip -HistoricalBinding $historicalBinding
    $historicalPreflight = Join-Path $fixtureRoot 'historical-commit-preflight'
    $historicalRun = Invoke-Generator $historicalSource $repo 'BL-334' $historicalContract.CurrentPaths $historicalPreflight Preflight
    Add-Case 'FCH-HISTORICAL-BL333-BL334-COMMIT-PRODUCT-PASS' ($historicalRun.ExitCode -eq 0) $historicalRun.Output

    foreach ($case in @(
        [ordered]@{ Id='FCH-MISSING-REPORT-CONTRACT-REJECTED'; Base=$preflight; Mutate={ param($d) Write-Utf8 (Join-Path $d 'report.md') "# Missing embedded report contract`n" } },
        [ordered]@{ Id='FCH-REPORT-WRONG-CURRENT-PATCH-SHA-REJECTED'; Base=$preflight; Mutate={ param($d) $j=Read-ReportContract(Join-Path $d 'report.md');$j.currentFeaturePatch.sha256='f'*64;Write-ReportContract(Join-Path $d 'report.md')$j } },
        [ordered]@{ Id='FCH-REPORT-WRONG-CORRECTION-PATCH-SHA-REJECTED'; Base=$preflight; Mutate={ param($d) $j=Read-ReportContract(Join-Path $d 'report.md');$j.correctionOnlyPatch.sha256='f'*64;Write-ReportContract(Join-Path $d 'report.md')$j } },
        [ordered]@{ Id='FCH-REPORT-MISSING-FINDING-REJECTED'; Base=$preflight; Mutate={ param($d) $j=Read-ReportContract(Join-Path $d 'report.md');$j.findingIds=@();$j.findingDispositions=@();Write-ReportContract(Join-Path $d 'report.md')$j } },
        [ordered]@{ Id='FCH-REPORT-FOREIGN-FINDING-REJECTED'; Base=$preflight; Mutate={ param($d) $j=Read-ReportContract(Join-Path $d 'report.md');$j.findingIds=@($j.findingIds+'BL340-REV-001');$base=$j.findingDispositions[0];$j.findingDispositions=@($j.findingDispositions+[pscustomobject]@{id='BL340-REV-001';severity=$base.severity;previousStatus=$base.previousStatus;status=$base.status;disposition=$base.disposition;correction=$base.correction;affectedPaths=$base.affectedPaths;regressionTestIds=$base.regressionTestIds;evidenceReferences=$base.evidenceReferences;producerStatus='CORRECTED_PENDING_DELTA_REVIEW';reviewerStatus='OPEN_PENDING_FOCUSED_INDEPENDENT_DELTA_REVIEW'});Write-ReportContract(Join-Path $d 'report.md')$j } },
        [ordered]@{ Id='FCH-PER-FINDING-REGRESSION-CORRECTION-MATRIX-MISMATCH-REJECTED'; Base=$preflight; Mutate={ param($d) $j=Read-Json(Join-Path $d 'finding-correction-matrix.json');$j.findings[0].regressionTestIds=@($j.findings[0].regressionTestIds+'GHP-FOREIGN-PER-FINDING');Write-Json(Join-Path $d 'finding-correction-matrix.json')$j } },
        [ordered]@{ Id='FCH-PER-FINDING-REGRESSION-MATRIX-MISMATCH-REJECTED'; Base=$preflight; Mutate={ param($d) $j=Read-Json(Join-Path $d 'finding-regression-matrix.json');$j.findings[0].regressionTests[0].id='FCH-FOREIGN-PER-FINDING';Write-Json(Join-Path $d 'finding-regression-matrix.json')$j } },
        [ordered]@{ Id='FCH-PER-FINDING-REGRESSION-LEDGER-MISMATCH-REJECTED'; Base=$preflight; Mutate={ param($d) $j=Read-Json(Join-Path $d 'finding-ledger.json');$j.findings[0].permanentRegressions=@($j.findings[0].permanentRegressions+'FCH-FOREIGN-PER-FINDING');Write-Json(Join-Path $d 'finding-ledger.json')$j } },
        [ordered]@{ Id='FCH-PER-FINDING-SEVERITY-MISMATCH-REJECTED'; Base=$preflight; Mutate={ param($d) $j=Read-Json(Join-Path $d 'finding-regression-matrix.json');$j.findings[0].severity='MINOR';Write-Json(Join-Path $d 'finding-regression-matrix.json')$j } },
        [ordered]@{ Id='FCH-PER-FINDING-PREVIOUS-STATUS-MISMATCH-REJECTED'; Base=$preflight; Mutate={ param($d) $j=Read-Json(Join-Path $d 'finding-ledger.json');$j.findings[0].previousStatus='PARTIALLY_CLOSED_CORRECTION_REQUIRED';Write-Json(Join-Path $d 'finding-ledger.json')$j } },
        [ordered]@{ Id='FCH-PER-FINDING-STATUS-MISMATCH-REJECTED'; Base=$preflight; Mutate={ param($d) $j=Read-Json(Join-Path $d 'finding-regression-matrix.json');$j.findings[0].status='OPEN';Write-Json(Join-Path $d 'finding-regression-matrix.json')$j } },
        [ordered]@{ Id='FCH-PER-FINDING-DISPOSITION-MISMATCH-REJECTED'; Base=$preflight; Mutate={ param($d) $j=Read-Json(Join-Path $d 'finding-ledger.json');$j.findings[0].disposition='DEFERRED';Write-Json(Join-Path $d 'finding-ledger.json')$j } },
        [ordered]@{ Id='FCH-PER-FINDING-CORRECTION-TEXT-MISMATCH-REJECTED'; Base=$preflight; Mutate={ param($d) $j=Read-Json(Join-Path $d 'finding-regression-matrix.json');$j.findings[0].correction='Foreign correction text.';Write-Json(Join-Path $d 'finding-regression-matrix.json')$j } },
        [ordered]@{ Id='FCH-PER-FINDING-AFFECTED-PATH-MISMATCH-REJECTED'; Base=$preflight; Mutate={ param($d) $j=Read-Json(Join-Path $d 'finding-ledger.json');$j.findings[0].correctionPaths=@('reference.txt');Write-Json(Join-Path $d 'finding-ledger.json')$j } },
        [ordered]@{ Id='FCH-PER-FINDING-EVIDENCE-MISMATCH-REJECTED'; Base=$preflight; Mutate={ param($d) $j=Read-ReportContract(Join-Path $d 'report.md');$j.findingDispositions[0].evidenceReferences=@('foreign-evidence.json');Write-ReportContract(Join-Path $d 'report.md')$j } },
        [ordered]@{ Id='FCH-PER-FINDING-PRODUCER-STATUS-MISMATCH-REJECTED'; Base=$preflight; Mutate={ param($d) $j=Read-ReportContract(Join-Path $d 'report.md');$j.findingDispositions[0].producerStatus='OPEN';Write-ReportContract(Join-Path $d 'report.md')$j } },
        [ordered]@{ Id='FCH-REPORT-WRONG-PREVIOUS-BINDING-REJECTED'; Base=$preflight; Mutate={ param($d) $j=Read-ReportContract(Join-Path $d 'report.md');$j.previousReview.bindingSha256='f'*64;Write-ReportContract(Join-Path $d 'report.md')$j } },
        [ordered]@{ Id='FCH-REPORT-PREFLIGHT-LIFECYCLE-IN-FINAL-REJECTED'; Base=$finalContent; Mutate={ param($d) $j=Read-ReportContract(Join-Path $d 'report.md');$j.artifactLifecycleState='ZIP_FREE_READY_TO_EXECUTE';$j.status='ZIP_FREE_READY_TO_EXECUTE';$j.readyToExecute=$true;$j.classicReviewReady=$false;$j.packageWriteAttemptCount=0;$j.nextAction='READY_FOR_SINGLE_DELTA_REVIEW_PACKAGE_WRITE_AUTHORIZATION';Write-ReportContract(Join-Path $d 'report.md')$j } },
        [ordered]@{ Id='FCH-REPORT-FINAL-LIFECYCLE-IN-PREFLIGHT-REJECTED'; Base=$preflight; Mutate={ param($d) $j=Read-ReportContract(Join-Path $d 'report.md');$j.artifactLifecycleState='FINAL_REVIEW_PACKAGE';$j.status='FINAL_REVIEW_PACKAGE';$j.readyToExecute=$false;$j.classicReviewReady=$true;$j.packageWriteAttemptCount=1;$j.nextAction='FOCUSED_INDEPENDENT_DELTA_REVIEW';Write-ReportContract(Join-Path $d 'report.md')$j } },
        [ordered]@{ Id='FCH-FINAL-CLASSIC-FALSE-REJECTED'; Base=$finalContent; Mutate={ param($d) $j=Read-Json(Join-Path $d 'completion-report.json');$j.classicReviewReady=$false;Write-Json(Join-Path $d 'completion-report.json')$j } },
        [ordered]@{ Id='FCH-FINAL-ATTEMPT-ZERO-REJECTED'; Base=$finalContent; Mutate={ param($d) $j=Read-Json(Join-Path $d 'completion-report.json');$j.packageWriteAttemptCount=0;Write-Json(Join-Path $d 'completion-report.json')$j } },
        [ordered]@{ Id='FCH-FINAL-PACKAGE-AUTH-NEXT-REJECTED'; Base=$finalContent; Mutate={ param($d) $j=Read-Json(Join-Path $d 'completion-report.json');$j.nextAction='READY_FOR_SINGLE_DELTA_REVIEW_PACKAGE_WRITE_AUTHORIZATION';Write-Json(Join-Path $d 'completion-report.json')$j } },
        [ordered]@{ Id='FCH-REFERENCE-ONLY-CHANGED-REJECTED'; Base=$preflight; Mutate={ param($d) $j=Read-Json(Join-Path $d 'focused-delta-review-record.json');$j.referenceOnlyPaths=@($j.correctionOnlyPaths[0]);Write-Json(Join-Path $d 'focused-delta-review-record.json')$j } },
        [ordered]@{ Id='FCH-REFERENCE-DUPLICATE-REJECTED'; Base=$preflight; Mutate={ param($d) $j=Read-Json(Join-Path $d 'focused-delta-review-record.json');$j.referenceOnlyPaths=@('reference.txt','reference.txt');Write-Json(Join-Path $d 'focused-delta-review-record.json')$j } },
        [ordered]@{ Id='FCH-REFERENCE-CASE-COLLISION-REJECTED'; Base=$preflight; Mutate={ param($d) $j=Read-Json(Join-Path $d 'focused-delta-review-record.json');$j.referenceOnlyPaths=@('reference.txt','REFERENCE.TXT');Write-Json(Join-Path $d 'focused-delta-review-record.json')$j } },
        [ordered]@{ Id='FCH-FOREIGN-CORRECTION-PATH-REJECTED'; Base=$preflight; Mutate={ param($d) $j=Read-Json(Join-Path $d 'focused-delta-review-record.json');$j.correctionOnlyPaths=@($j.correctionOnlyPaths+'reference.txt');Write-Json(Join-Path $d 'focused-delta-review-record.json')$j } },
        [ordered]@{ Id='FCH-COMMIT-WRONG-COMMIT-REJECTED'; Base=$preflight; Mutate={ param($d) $bad='f'*40;$p=Read-Json(Join-Path $d 'previous-review-binding.json');$p.previousReviewState.correctionStartCommit=$bad;$p.previousReviewState.previousReviewedTree=$bad;Write-Json(Join-Path $d 'previous-review-binding.json')$p;$f=Read-Json(Join-Path $d 'focused-delta-review-record.json');$f.previousReviewState.correctionStartCommit=$bad;Write-Json(Join-Path $d 'focused-delta-review-record.json')$f } },
        [ordered]@{ Id='FCH-COMMIT-AND-SNAPSHOT-REJECTED'; Base=$preflight; Mutate={ param($d) $p=Read-Json(Join-Path $d 'previous-review-binding.json');$p|Add-Member historicalPackagePath 'forbidden.zip';Write-Json(Join-Path $d 'previous-review-binding.json')$p } },
        [ordered]@{ Id='FCH-NEITHER-PREVIOUS-MODE-REJECTED'; Base=$preflight; Mutate={ param($d) $p=Read-Json(Join-Path $d 'previous-review-binding.json');$p.PSObject.Properties.Remove('previousReviewState');Write-Json(Join-Path $d 'previous-review-binding.json')$p } },
        [ordered]@{ Id='FCH-SNAPSHOT-INCOMPLETE-REJECTED'; Base=$snapshotPreflight; Mutate={ param($d) $p=Read-Json(Join-Path $d 'previous-review-binding.json');$p.previousReviewState.PSObject.Properties.Remove('previousReviewedPostimages');Write-Json(Join-Path $d 'previous-review-binding.json')$p } },
        [ordered]@{ Id='FCH-RENAME-SOURCE-BINDING-REJECTED'; Base=$preflight; Mutate={ param($d) $j=Read-Json(Join-Path $d 'correction-scope-inventory.json');$r=@($j.entries|Where-Object gitStatus -ceq 'TRACKED_RENAMED')[0];$r.previousPath='modify.txt';Write-Json(Join-Path $d 'correction-scope-inventory.json')$j } },
        [ordered]@{ Id='FCH-RENAME-TARGET-BINDING-REJECTED'; Base=$preflight; Mutate={ param($d) $j=Read-Json(Join-Path $d 'correction-scope-inventory.json');$r=@($j.entries|Where-Object gitStatus -ceq 'TRACKED_RENAMED')[0];$r.path='wrong-target.txt';Write-Json(Join-Path $d 'correction-scope-inventory.json')$j } },
        [ordered]@{ Id='FCH-PREPOST-HASH-BINDING-REJECTED'; Base=$preflight; Mutate={ param($d) $j=Read-Json(Join-Path $d 'correction-scope-inventory.json');$j.entries[0].currentCorrectedSha256='f'*64;Write-Json(Join-Path $d 'correction-scope-inventory.json')$j } }
        [ordered]@{ Id='FCH-DUPLICATE-TOP-LEVEL-JSON-REJECTED'; Base=$preflight; Mutate={ param($d) $p=Join-Path $d 'validation-summary.json';$t=[System.IO.File]::ReadAllText($p,$utf8);$t=$t -replace '"schemaVersion": 1,',('"schemaVersion": 1,'+"`n"+'  "schemaVersion": 1,');Write-Utf8 $p $t } },
        [ordered]@{ Id='FCH-DUPLICATE-NESTED-JSON-REJECTED'; Base=$preflight; Mutate={ param($d) $p=Join-Path $d 'previous-review-binding.json';$t=[System.IO.File]::ReadAllText($p,$utf8);$t=$t -replace '"type": "COMMIT",',('"type": "COMMIT",'+"`n"+'    "type": "COMMIT",');Write-Utf8 $p $t } },
        [ordered]@{ Id='FCH-DUPLICATE-FINDING-OBJECT-REJECTED'; Base=$preflight; Mutate={ param($d) $p=Join-Path $d 'finding-ledger.json';$t=[System.IO.File]::ReadAllText($p,$utf8);$t=$t -replace '"id": "BL339-REV-003",',('"id": "BL339-REV-003",'+"`n"+'      "id": "BL339-REV-003",');Write-Utf8 $p $t } },
        [ordered]@{ Id='FCH-DUPLICATE-REPORT-TOP-LEVEL-REJECTED'; Base=$preflight; Mutate={ param($d) $p=Join-Path $d 'report.md';$t=[System.IO.File]::ReadAllText($p,$utf8);$t=$t -replace '"schemaVersion": 1,',('"schemaVersion": 1,'+"`n"+'  "schemaVersion": 1,');Write-Utf8 $p $t } },
        [ordered]@{ Id='FCH-DUPLICATE-REPORT-NESTED-REJECTED'; Base=$preflight; Mutate={ param($d) $p=Join-Path $d 'report.md';$t=[System.IO.File]::ReadAllText($p,$utf8);$t=$t -replace '"stateType": "COMMIT",',('"stateType": "COMMIT",'+"`n"+'    "stateType": "COMMIT",');Write-Utf8 $p $t } },
        [ordered]@{ Id='FCH-TRAILING-JSON-PAYLOAD-REJECTED'; Base=$preflight; Mutate={ param($d) $p=Join-Path $d 'validation-summary.json';$t=[System.IO.File]::ReadAllText($p,$utf8);Write-Utf8 $p ($t+'{}') } },
        [ordered]@{ Id='FCH-UTF8-BOM-JSON-REJECTED'; Base=$preflight; Mutate={ param($d) $p=Join-Path $d 'external-governance-manifest.json';$b=[System.IO.File]::ReadAllBytes($p);[System.IO.File]::WriteAllBytes($p,[byte[]](0xEF,0xBB,0xBF)+$b) } },
        [ordered]@{ Id='FCH-INVALID-UTF8-JSON-REJECTED'; Base=$preflight; Mutate={ param($d) $p=Join-Path $d 'external-governance-manifest.json';[System.IO.File]::WriteAllBytes($p,[byte[]](0x7B,0x22,0x78,0x22,0x3A,0x22,0xFF,0x22,0x7D)) } }
    )) {
        $mutant = Copy-Artifact $case.Base ('mutant-' + $case.Id.ToLowerInvariant())
        & $case.Mutate $mutant
        Update-PackageMetadata $mutant
        $validation = Invoke-ProductValidator $mutant $repo
        Add-Case $case.Id ($validation.ExitCode -ne 0) $validation.Output
    }

    $foreign = Copy-Artifact $preflight 'mutant-foreign-task-finding'
    $foreignId = 'BL340-REV-001'
    $j=Read-Json(Join-Path $foreign 'assignment-record.json');$j.findingIds=@($foreignId);Write-Json(Join-Path $foreign 'assignment-record.json')$j
    $j=Read-Json(Join-Path $foreign 'completion-report.json');$j.findingIds=@($foreignId);Write-Json(Join-Path $foreign 'completion-report.json')$j
    $j=Read-Json(Join-Path $foreign 'finding-correction-matrix.json');$j.findings[0].id=$foreignId;Write-Json(Join-Path $foreign 'finding-correction-matrix.json')$j
    $j=Read-Json(Join-Path $foreign 'finding-regression-matrix.json');$j.findings[0].id=$foreignId;Write-Json(Join-Path $foreign 'finding-regression-matrix.json')$j
    $j=Read-Json(Join-Path $foreign 'focused-delta-review-record.json');$j.reviewedFindingIds=@($foreignId);Write-Json(Join-Path $foreign 'focused-delta-review-record.json')$j
    $j=Read-Json(Join-Path $foreign 'finding-ledger.json');$j.findings[0].id=$foreignId;Write-Json(Join-Path $foreign 'finding-ledger.json')$j
    Update-PackageMetadata $foreign
    $foreignValidation=Invoke-ProductValidator $foreign $repo
    Add-Case 'FCH-FOREIGN-TASK-FINDING-ALL-SIX-PRODUCT-REJECTED' ($foreignValidation.ExitCode -ne 0) $foreignValidation.Output

    $finalInput = Join-Path $fixtureRoot 'invalid-final-input'
    [void][System.IO.Directory]::CreateDirectory($finalInput)
    Copy-Item -Path (Join-Path $commitSource '*') -Destination $finalInput -Recurse -Force
    foreach($name in @('assignment-record.json','completion-report.json','readiness-evidence.json')){ $j=Read-Json(Join-Path $finalInput $name);$j.artifactLifecycleState='FINAL_REVIEW_PACKAGE';$j.readyToExecute=$false;$j.classicReviewReady=$true;if($name -cne 'assignment-record.json'){$j.packageWriteAttemptCount=1};if($j.PSObject.Properties['nextAction']){$j.nextAction='FOCUSED_INDEPENDENT_DELTA_REVIEW'};Write-Json(Join-Path $finalInput $name)$j }
    $invalidOutput = Join-Path $fixtureRoot 'invalid-final-input-output'
    $invalidRun = Invoke-Generator $finalInput $repo 'BL-339' $commitContract.CurrentPaths $invalidOutput Preflight
    Add-Case 'FCH-PREFLIGHT-INPUT-FINAL-SEMANTICS-REJECTED' ($invalidRun.ExitCode -ne 0) $invalidRun.Output

    $focusedPreviousSource = Copy-Artifact $snapshotSource 'focused-to-focused-previous-source'
    Convert-ToTwoFindingParityFixture $focusedPreviousSource
    foreach ($packageMetadataName in @('MANIFEST.sha256', 'package-inventory.json')) {
        Remove-Item -LiteralPath (Join-Path $focusedPreviousSource $packageMetadataName)
    }
    $focusedPreviousContent = Join-Path $fixtureRoot 'focused-to-focused-previous-content'
    $focusedPreviousRun = Invoke-Generator $focusedPreviousSource $repo 'BL-339' `
        $snapshotContract.CurrentPaths $focusedPreviousContent FinalContent
    Add-Case 'FCH-FOCUSED-TO-FOCUSED-PREVIOUS-PACKAGE-PASS' `
        ($focusedPreviousRun.ExitCode -eq 0) $focusedPreviousRun.Output
    $focusedPreviousZip = Join-Path $fixtureRoot 'focused-to-focused-previous.zip'
    New-ZipFromDirectory $focusedPreviousContent $focusedPreviousZip
    $focusedPreviousHash = Get-Hash $focusedPreviousZip
    $transitiveFullHash = Get-Hash $historicalZip

    $outcomePath = Join-Path $fixtureRoot 'focused-independent-review-outcome.json'
    $outcomeHash = New-IndependentReviewOutcome -LiteralPath $outcomePath -TaskId 'BL-339' `
        -PreviousReviewPackageSha256 $focusedPreviousHash `
        -TransitiveFullReviewBaselineSha256 $transitiveFullHash `
        -ReviewedCurrentDeltaSha256 (Get-Hash (Join-Path $focusedPreviousContent 'current-delta.patch')) `
        -FindingIds @('BL339-REV-003', 'BL339-REV-004') -OpenFindingIds @('BL339-REV-004')

    $passOutcomePath = Join-Path $fixtureRoot 'generalized-pass-outcome.json'
    $passOutcomeHash = New-GeneralizedIndependentReviewOutcome -LiteralPath $passOutcomePath `
        -TaskId 'BL-339' -ReviewedPackageSha256 $focusedPreviousHash `
        -ImmediatePreviousReviewPackageSha256 $transitiveFullHash `
        -TransitiveFullReviewBaselineSha256 $transitiveFullHash `
        -ReviewedCurrentDeltaSha256 (Get-Hash (Join-Path $focusedPreviousContent 'current-delta.patch')) `
        -ReviewedCorrectionPatchSha256 (Get-Hash (Join-Path $focusedPreviousContent 'correction-only.patch')) `
        -TargetFindingIds @('BL339-REV-003', 'BL339-REV-004') `
        -InheritedClosedFindingIds @() -DirectInterfaceOutcomes @() -ReviewResult PASS
    Add-Case 'FCH-GENERALIZED-PASS-ZERO-OPEN-OUTCOME-SCHEMA-PASS' `
        (Test-IndependentReviewOutcomeSchema $passOutcomePath) $passOutcomeHash

    $oneFindingOutcomePath = Join-Path $fixtureRoot 'generalized-one-finding-outcome.json'
    $null = New-GeneralizedIndependentReviewOutcome -LiteralPath $oneFindingOutcomePath `
        -TaskId 'BL-339' -ReviewedPackageSha256 $focusedPreviousHash `
        -ImmediatePreviousReviewPackageSha256 $transitiveFullHash `
        -TransitiveFullReviewBaselineSha256 $transitiveFullHash `
        -ReviewedCurrentDeltaSha256 (Get-Hash (Join-Path $focusedPreviousContent 'current-delta.patch')) `
        -ReviewedCorrectionPatchSha256 (Get-Hash (Join-Path $focusedPreviousContent 'correction-only.patch')) `
        -TargetFindingIds @('BL339-REV-003', 'BL339-REV-004') `
        -OpenTargetFindingIds @('BL339-REV-004') -InheritedClosedFindingIds @() `
        -DirectInterfaceOutcomes @() -ReviewResult FAIL_WITH_FINDINGS
    Add-Case 'FCH-GENERALIZED-FAIL-WITH-ONE-FINDING-SCHEMA-PASS' `
        (Test-IndependentReviewOutcomeSchema $oneFindingOutcomePath)

    $multiFindingOutcomePath = Join-Path $fixtureRoot 'generalized-multiple-finding-outcome.json'
    $null = New-GeneralizedIndependentReviewOutcome -LiteralPath $multiFindingOutcomePath `
        -TaskId 'BL-339' -ReviewedPackageSha256 $focusedPreviousHash `
        -ImmediatePreviousReviewPackageSha256 $transitiveFullHash `
        -TransitiveFullReviewBaselineSha256 $transitiveFullHash `
        -ReviewedCurrentDeltaSha256 (Get-Hash (Join-Path $focusedPreviousContent 'current-delta.patch')) `
        -ReviewedCorrectionPatchSha256 (Get-Hash (Join-Path $focusedPreviousContent 'correction-only.patch')) `
        -TargetFindingIds @('BL339-REV-003', 'BL339-REV-004') `
        -OpenTargetFindingIds @('BL339-REV-004') -NewFindingIds @('BL339-REV-006') `
        -InheritedClosedFindingIds @() -DirectInterfaceOutcomes @() `
        -ReviewResult FAIL_WITH_FINDINGS
    Add-Case 'FCH-GENERALIZED-FAIL-WITH-MULTIPLE-FINDINGS-SCHEMA-PASS' `
        (Test-IndependentReviewOutcomeSchema $multiFindingOutcomePath)
    Add-Case 'FCH-GENERALIZED-DIRECT-INTERFACE-OUTCOMES-EMPTY-PASS' `
        (Test-IndependentReviewOutcomeSchema $multiFindingOutcomePath)

    $directOutcomePath = Join-Path $fixtureRoot 'generalized-direct-interface-outcome.json'
    $null = New-GeneralizedIndependentReviewOutcome -LiteralPath $directOutcomePath `
        -TaskId 'BL-339' -ReviewedPackageSha256 $focusedPreviousHash `
        -ImmediatePreviousReviewPackageSha256 $transitiveFullHash `
        -TransitiveFullReviewBaselineSha256 $transitiveFullHash `
        -ReviewedCurrentDeltaSha256 (Get-Hash (Join-Path $focusedPreviousContent 'current-delta.patch')) `
        -ReviewedCorrectionPatchSha256 (Get-Hash (Join-Path $focusedPreviousContent 'correction-only.patch')) `
        -TargetFindingIds @('BL339-REV-003', 'BL339-REV-004') `
        -OpenTargetFindingIds @('BL339-REV-004') -InheritedClosedFindingIds @() `
        -DirectInterfaceOutcomes @([ordered]@{ id='BL339-CORR-001'; disposition='CLOSED' }) `
        -ReviewResult FAIL_WITH_FINDINGS
    Add-Case 'FCH-GENERALIZED-DIRECT-INTERFACE-OUTCOME-PRESENT-PASS' `
        (Test-IndependentReviewOutcomeSchema $directOutcomePath)

    $null = Invoke-GitText $repo @('add', '-A')
    $null = Invoke-GitText $repo @('commit', '--quiet', '-m', 'focused review fixture state')
    $focusedPreviousTree = Invoke-GitText $repo @('rev-parse', 'HEAD^{tree}')
    Add-Case 'FCH-FOCUSED-PREVIOUS-TREE-PERSISTED-PASS' ($focusedPreviousTree -ceq $currentTree)
    Write-Utf8 (Join-Path $repo 'modify.txt') "after`noutcome correction`n"
    $nextCurrentEntries = @(
        $currentEntries | ConvertTo-Json -Depth 100 | ConvertFrom-Json -Depth 100
    )
    $nextModifyEntry = @($nextCurrentEntries | Where-Object Path -ceq 'modify.txt')[0]
    $nextModifyEntry.Postimage = Get-GenericPostimageEvidence -Root $repo -Path 'modify.txt' -GitMode '100644'
    $nextCorrectionEntries = @([pscustomobject][ordered]@{
            Path = 'modify.txt'; PreviousPath = $null; GitStatus = 'TRACKED_MODIFIED'
            Tracked = $true; Staged = $false
            Preimage = Get-GenericBaselineBlobEvidence -Root $repo -Commit $focusedPreviousTree -Path 'modify.txt'
            Postimage = Get-GenericPostimageEvidence -Root $repo -Path 'modify.txt' -GitMode '100644'
            PostimageAbsent = $false
        })
    $nextCurrentEvidence = Get-GenericDeltaEvidence -Root $repo -BaselineCommit $baseline `
        -IncludedEntry $nextCurrentEntries -ExcludedEntry @()
    $nextCurrentPatchBytes = [byte[]]($nextCurrentEvidence.Bytes + $modePatch)
    $nextCorrectionEvidence = Get-GenericDeltaEvidence -Root $repo -BaselineCommit $focusedPreviousTree `
        -IncludedEntry $nextCorrectionEntries -ExcludedEntry @()
    $nextCorrectionPatchBytes = [byte[]]$nextCorrectionEvidence.Bytes
    $nextCurrentTree = Get-AppliedTree -Root $repo -Baseline $baseline -PatchBytes $nextCurrentPatchBytes
    $nextCurrentPaths = @(Get-GenericScopePaths -Entry $nextCurrentEntries | Sort-Object -Unique)
    $nextPreviousPostimages = @($nextCurrentPaths | ForEach-Object {
            Get-TreePostimage -Root $repo -Tree $focusedPreviousTree -Path $_
        })
    $focusedPreviousScope = Read-Json (Join-Path $focusedPreviousContent 'scope-inventory.json')
    $focusedPreviousBinding = [pscustomobject][ordered]@{
        type = 'IMMUTABLE_REVIEW_PACKAGE'
        historicalPackagePath = $focusedPreviousZip
        historicalPackageSha256 = $focusedPreviousHash
        historicalManifestSha256 = Get-Hash (Join-Path $focusedPreviousContent 'MANIFEST.sha256')
        historicalPatchSha256 = Get-Hash (Join-Path $focusedPreviousContent 'current-delta.patch')
        historicalScopeInventorySha256 = Get-Hash (Join-Path $focusedPreviousContent 'scope-inventory.json')
        previousReviewedBaselineCommit = $baseline
        previousReviewedTree = $focusedPreviousTree
        previousReviewedPathCount = [int]$focusedPreviousScope.pathCount
        previousReviewedPostimages = $nextPreviousPostimages
    }
    $focusedToFocusedSource = Join-Path $fixtureRoot 'focused-to-focused-source'
    $focusedToFocusedContract = New-SyntheticSource -Directory $focusedToFocusedSource `
        -RepositoryRoot $repo -TaskId 'BL-339' -FindingId 'BL339-REV-004' `
        -BaselineCommit $baseline -PreviousCommit $previousCommit -PreviousTree $focusedPreviousTree `
        -CurrentPatch $nextCurrentPatchBytes -CorrectionPatch $nextCorrectionPatchBytes `
        -CurrentEntries $nextCurrentEntries -CorrectionEntries $nextCorrectionEntries `
        -CurrentTree $nextCurrentTree -PreviousMode IMMUTABLE_REVIEW_PACKAGE `
        -HistoricalPackagePath $focusedPreviousZip -HistoricalBinding $focusedPreviousBinding
    Add-IndependentReviewOutcomeToSource -Directory $focusedToFocusedSource `
        -OutcomePath $outcomePath -OutcomeSha256 $outcomeHash
    $focusedToFocusedValidation = Invoke-ProductValidator $focusedToFocusedSource $repo `
        -IndependentReviewOutcomePath $outcomePath `
        -ExpectedIndependentReviewOutcomeSha256 $outcomeHash
    Add-Case 'FCH-FOCUSED-PACKAGE-WITH-INDEPENDENT-OUTCOME-PASS' `
        ($focusedToFocusedValidation.ExitCode -eq 0) $focusedToFocusedValidation.Output
    Add-Case 'FCH-ONLY-OUTCOME-OPEN-FINDING-TARGETED-PASS' `
        ($focusedToFocusedValidation.ExitCode -eq 0) $focusedToFocusedValidation.Output
    Add-Case 'FCH-TRANSITIVE-FULL-REVIEW-BOUND-PASS' `
        ($focusedToFocusedValidation.ExitCode -eq 0 -and
         $focusedToFocusedValidation.Output -match 'TransitiveFullReviewBaselineSHA256\s*:\s*[0-9A-F]{64}') `
        $focusedToFocusedValidation.Output

    $focusedSecondZip = Join-Path $fixtureRoot 'focused-second-with-prior-outcome.zip'
    New-ZipFromDirectory $focusedToFocusedSource $focusedSecondZip
    $focusedSecondHash = Get-Hash $focusedSecondZip
    Add-Case 'FCH-FOCUSED-SECOND-PACKAGE-WITH-PRIOR-OUTCOME-CREATED' `
        ((Test-Path -LiteralPath $focusedSecondZip -PathType Leaf) -and
         (Get-Hash $focusedSecondZip) -ceq $focusedSecondHash)

    $null = Invoke-GitText $repo @('add', '-A')
    $null = Invoke-GitText $repo @('commit', '--quiet', '-m', 'second focused fixture state')
    $focusedSecondTree = Invoke-GitText $repo @('rev-parse', 'HEAD^{tree}')
    Write-Utf8 (Join-Path $repo 'modify.txt') "after`noutcome correction`nrecursive correction`n"
    $thirdCurrentEntries = @(
        $nextCurrentEntries | ConvertTo-Json -Depth 100 | ConvertFrom-Json -Depth 100
    )
    $thirdModifyEntry = @($thirdCurrentEntries | Where-Object Path -ceq 'modify.txt')[0]
    $thirdModifyEntry.Postimage = Get-GenericPostimageEvidence -Root $repo -Path 'modify.txt' `
        -GitMode '100644'
    $thirdCorrectionEntries = @([pscustomobject][ordered]@{
            Path = 'modify.txt'; PreviousPath = $null; GitStatus = 'TRACKED_MODIFIED'
            Tracked = $true; Staged = $false
            Preimage = Get-GenericBaselineBlobEvidence -Root $repo -Commit $focusedSecondTree `
                -Path 'modify.txt'
            Postimage = Get-GenericPostimageEvidence -Root $repo -Path 'modify.txt' -GitMode '100644'
            PostimageAbsent = $false
        })
    $thirdCurrentEvidence = Get-GenericDeltaEvidence -Root $repo -BaselineCommit $baseline `
        -IncludedEntry $thirdCurrentEntries -ExcludedEntry @()
    $thirdCurrentPatchBytes = [byte[]]($thirdCurrentEvidence.Bytes + $modePatch)
    $thirdCorrectionEvidence = Get-GenericDeltaEvidence -Root $repo -BaselineCommit $focusedSecondTree `
        -IncludedEntry $thirdCorrectionEntries -ExcludedEntry @()
    $thirdCorrectionPatchBytes = [byte[]]$thirdCorrectionEvidence.Bytes
    $thirdCurrentTree = Get-AppliedTree -Root $repo -Baseline $baseline `
        -PatchBytes $thirdCurrentPatchBytes
    $thirdCurrentPaths = @(Get-GenericScopePaths -Entry $thirdCurrentEntries | Sort-Object -Unique)
    $thirdPreviousPostimages = @($thirdCurrentPaths | ForEach-Object {
            Get-TreePostimage -Root $repo -Tree $focusedSecondTree -Path $_
        })
    $focusedSecondScope = Read-Json (Join-Path $focusedToFocusedSource 'scope-inventory.json')
    $focusedSecondBinding = [pscustomobject][ordered]@{
        type = 'IMMUTABLE_REVIEW_PACKAGE'
        historicalPackagePath = $focusedSecondZip
        historicalPackageSha256 = $focusedSecondHash
        historicalManifestSha256 = Get-Hash (Join-Path $focusedToFocusedSource 'MANIFEST.sha256')
        historicalPatchSha256 = Get-Hash (Join-Path $focusedToFocusedSource 'current-delta.patch')
        historicalScopeInventorySha256 = Get-Hash (Join-Path $focusedToFocusedSource 'scope-inventory.json')
        previousReviewedBaselineCommit = $baseline
        previousReviewedTree = $focusedSecondTree
        previousReviewedPathCount = [int]$focusedSecondScope.pathCount
        previousReviewedPostimages = $thirdPreviousPostimages
    }
    $thirdCorrectionSource = Join-Path $fixtureRoot 'third-correction-source'
    $null = New-SyntheticSource -Directory $thirdCorrectionSource -RepositoryRoot $repo `
        -TaskId 'BL-339' -FindingId 'BL339-REV-006' -BaselineCommit $baseline `
        -PreviousCommit $previousCommit -PreviousTree $focusedSecondTree `
        -CurrentPatch $thirdCurrentPatchBytes -CorrectionPatch $thirdCorrectionPatchBytes `
        -CurrentEntries $thirdCurrentEntries -CorrectionEntries $thirdCorrectionEntries `
        -CurrentTree $thirdCurrentTree -PreviousMode IMMUTABLE_REVIEW_PACKAGE `
        -HistoricalPackagePath $focusedSecondZip -HistoricalBinding $focusedSecondBinding
    Convert-ToTwoFindingParityFixture -Directory $thirdCorrectionSource `
        -FindingIds @('BL339-REV-006', 'BL339-REV-007') `
        -TestIds @('FCH-RECURSIVE-REV006', 'FCH-RECURSIVE-REV007')

    $secondOutcomePath = Join-Path $fixtureRoot 'focused-second-independent-outcome.json'
    $secondOutcomeHash = New-GeneralizedIndependentReviewOutcome `
        -LiteralPath $secondOutcomePath -TaskId 'BL-339' `
        -ReviewedPackageSha256 $focusedSecondHash `
        -ImmediatePreviousReviewPackageSha256 $focusedPreviousHash `
        -PreviousIndependentReviewOutcomeSha256 $outcomeHash `
        -TransitiveFullReviewBaselineSha256 $transitiveFullHash `
        -ReviewedCurrentDeltaSha256 (Get-Hash (Join-Path $focusedToFocusedSource 'current-delta.patch')) `
        -ReviewedCorrectionPatchSha256 (Get-Hash (Join-Path $focusedToFocusedSource 'correction-only.patch')) `
        -TargetFindingIds @('BL339-REV-004') `
        -NewFindingIds @('BL339-REV-006', 'BL339-REV-007') `
        -InheritedClosedFindingIds @('BL339-REV-003') -DirectInterfaceOutcomes @() `
        -ReviewResult FAIL_WITH_FINDINGS
    Add-IndependentReviewOutcomeToSource -Directory $thirdCorrectionSource `
        -OutcomePath $secondOutcomePath -OutcomeSha256 $secondOutcomeHash
    $thirdCorrectionValidation = Invoke-ProductValidator $thirdCorrectionSource $repo `
        -IndependentReviewOutcomePath $secondOutcomePath `
        -ExpectedIndependentReviewOutcomeSha256 $secondOutcomeHash
    Add-Case 'FCH-FOCUSED-SECOND-TO-THIRD-CORRECTION-PASS' `
        ($thirdCorrectionValidation.ExitCode -eq 0) $thirdCorrectionValidation.Output
    Add-Case 'FCH-RECURSIVE-FOCUSED-THIRD-CHAIN-PASS' `
        ($thirdCorrectionValidation.ExitCode -eq 0 -and
         $thirdCorrectionValidation.Output -match 'FindingDispositionBindingResult\s*:\s*PASS') `
        $thirdCorrectionValidation.Output
    Add-Case 'FCH-NEW-REVIEWER-FINDINGS-OUTSIDE-PRODUCER-TARGET-PASS' `
        ($thirdCorrectionValidation.ExitCode -eq 0) $thirdCorrectionValidation.Output
    Add-Case 'FCH-GENERALIZED-MULTI-FINDING-OUTCOME-PASS' `
        ($thirdCorrectionValidation.ExitCode -eq 0) $thirdCorrectionValidation.Output

    $producerForgedSource = Copy-Artifact $thirdCorrectionSource 'producer-forged-reviewer-finding'
    Convert-ToTwoFindingParityFixture -Directory $producerForgedSource `
        -FindingIds @('BL339-REV-006', 'BL339-REV-008') `
        -TestIds @('FCH-RECURSIVE-REV006', 'FCH-PRODUCER-FORGED-REV008')
    $producerForgedValidation = Invoke-ProductValidator $producerForgedSource $repo `
        -IndependentReviewOutcomePath $secondOutcomePath `
        -ExpectedIndependentReviewOutcomeSha256 $secondOutcomeHash
    Add-Case 'FCH-PRODUCER-CANNOT-FORGE-REVIEWER-FINDING-REJECTED' `
        ($producerForgedValidation.ExitCode -ne 0) $producerForgedValidation.Output

    $omittedOpenSource = Copy-Artifact $thirdCorrectionSource 'producer-omitted-open-finding'
    Set-SingleSourceFindingId $omittedOpenSource 'BL339-REV-006'
    $omittedOpenValidation = Invoke-ProductValidator $omittedOpenSource $repo `
        -IndependentReviewOutcomePath $secondOutcomePath `
        -ExpectedIndependentReviewOutcomeSha256 $secondOutcomeHash
    Add-Case 'FCH-GENERALIZED-OPEN-FINDING-OMITTED-REJECTED' `
        ($omittedOpenValidation.ExitCode -ne 0) $omittedOpenValidation.Output

    $generalizedMutationCases = @(
        [ordered]@{
            Id='FCH-GENERALIZED-OPEN-CLOSED-OVERLAP-REJECTED'
            Mutate={ param($j) $j.openFindingIds=@($j.openFindingIds + 'BL339-REV-004') }
        },
        [ordered]@{
            Id='FCH-GENERALIZED-UNKNOWN-DISPOSITION-REJECTED'
            Mutate={ param($j) $j.targetFindingOutcomes[0].disposition='UNKNOWN' }
        },
        [ordered]@{
            Id='FCH-GENERALIZED-PRODUCER-DISPOSITION-CONTRADICTION-REJECTED'
            Mutate={ param($j) $j.targetFindingOutcomes[0].disposition='OPEN_INCOMPLETE_CORRECTION';$j.openFindingIds=@($j.openFindingIds+'BL339-REV-004');$j.closedFindingIds=@($j.closedFindingIds|Where-Object{$_ -cne 'BL339-REV-004'}) }
        }
    )
    foreach ($generalizedCase in $generalizedMutationCases) {
        $mutantPath = Join-Path $fixtureRoot ($generalizedCase.Id.ToLowerInvariant() + '.json')
        $mutant = Read-Json $secondOutcomePath
        & $generalizedCase.Mutate $mutant
        Write-Json $mutantPath $mutant
        $mutantHash = (Get-Hash $mutantPath).ToUpperInvariant()
        $validation = Invoke-ProductValidator $thirdCorrectionSource $repo `
            -IndependentReviewOutcomePath $mutantPath `
            -ExpectedIndependentReviewOutcomeSha256 $mutantHash
        Add-Case $generalizedCase.Id ($validation.ExitCode -ne 0) $validation.Output
    }

    $directPresentOutcomePath = Join-Path $fixtureRoot 'focused-second-direct-present-outcome.json'
    $directPresentOutcome = Read-Json $secondOutcomePath
    $directPresentOutcome.directInterfaceOutcomes = @(
        [pscustomobject][ordered]@{ id='BL339-CORR-001'; disposition='CLOSED' }
    )
    Write-Json $directPresentOutcomePath $directPresentOutcome
    $directPresentOutcomeHash = (Get-Hash $directPresentOutcomePath).ToUpperInvariant()
    $directPresentSource = Copy-Artifact $thirdCorrectionSource 'direct-interface-present-source'
    Remove-Item -LiteralPath (Join-Path $directPresentSource 'previous-independent-review-outcome.json')
    Add-IndependentReviewOutcomeToSource -Directory $directPresentSource `
        -OutcomePath $directPresentOutcomePath -OutcomeSha256 $directPresentOutcomeHash
    $directPresentValidation = Invoke-ProductValidator $directPresentSource $repo `
        -IndependentReviewOutcomePath $directPresentOutcomePath `
        -ExpectedIndependentReviewOutcomeSha256 $directPresentOutcomeHash
    Add-Case 'FCH-GENERALIZED-DIRECT-INTERFACE-PRESENT-VALIDATOR-PASS' `
        ($directPresentValidation.ExitCode -eq 0) $directPresentValidation.Output

    $missingPriorContent = Copy-Artifact $focusedToFocusedSource 'recursive-missing-prior-outcome-content'
    Remove-Item -LiteralPath (Join-Path $missingPriorContent 'previous-independent-review-outcome.json')
    Update-PackageMetadata $missingPriorContent
    $missingPriorZip = Join-Path $fixtureRoot 'recursive-missing-prior-outcome.zip'
    New-ZipFromDirectory $missingPriorContent $missingPriorZip
    $missingPriorSource = Copy-Artifact $thirdCorrectionSource 'recursive-missing-prior-outcome-source'
    Set-HistoricalPackageBinding -Directory $missingPriorSource -PackagePath $missingPriorZip `
        -PackageContentDirectory $missingPriorContent
    $missingPriorValidation = Invoke-ProductValidator $missingPriorSource $repo `
        -IndependentReviewOutcomePath $secondOutcomePath `
        -ExpectedIndependentReviewOutcomeSha256 $secondOutcomeHash
    Add-Case 'FCH-RECURSIVE-MISSING-PRIOR-OUTCOME-REJECTED' `
        ($missingPriorValidation.ExitCode -ne 0) $missingPriorValidation.Output

    $wrongPriorContent = Copy-Artifact $focusedToFocusedSource 'recursive-wrong-prior-outcome-hash-content'
    foreach ($name in @('assignment-record.json','completion-report.json','focused-delta-review-record.json')) {
        $contract = Read-Json (Join-Path $wrongPriorContent $name)
        $contract.previousIndependentReviewOutcomeSha256 = 'F' * 64
        Write-Json (Join-Path $wrongPriorContent $name) $contract
    }
    Update-PackageMetadata $wrongPriorContent
    $wrongPriorZip = Join-Path $fixtureRoot 'recursive-wrong-prior-outcome-hash.zip'
    New-ZipFromDirectory $wrongPriorContent $wrongPriorZip
    $wrongPriorSource = Copy-Artifact $thirdCorrectionSource 'recursive-wrong-prior-outcome-hash-source'
    Set-HistoricalPackageBinding -Directory $wrongPriorSource -PackagePath $wrongPriorZip `
        -PackageContentDirectory $wrongPriorContent
    $wrongPriorValidation = Invoke-ProductValidator $wrongPriorSource $repo `
        -IndependentReviewOutcomePath $secondOutcomePath `
        -ExpectedIndependentReviewOutcomeSha256 $secondOutcomeHash
    Add-Case 'FCH-RECURSIVE-WRONG-PRIOR-OUTCOME-HASH-REJECTED' `
        ($wrongPriorValidation.ExitCode -ne 0) $wrongPriorValidation.Output

    $brokenChainContent = Copy-Artifact $focusedToFocusedSource 'recursive-broken-chain-content'
    $brokenChainFocused = Read-Json (Join-Path $brokenChainContent 'focused-delta-review-record.json')
    $brokenChainFocused.previousReviewSha256 = 'f' * 64
    $brokenChainFocused.previousReviewState.historicalPackageSha256 = 'f' * 64
    Write-Json (Join-Path $brokenChainContent 'focused-delta-review-record.json') $brokenChainFocused
    Update-PackageMetadata $brokenChainContent
    $brokenChainZip = Join-Path $fixtureRoot 'recursive-broken-chain.zip'
    New-ZipFromDirectory $brokenChainContent $brokenChainZip
    $brokenChainSource = Copy-Artifact $thirdCorrectionSource 'recursive-broken-chain-source'
    Set-HistoricalPackageBinding -Directory $brokenChainSource -PackagePath $brokenChainZip `
        -PackageContentDirectory $brokenChainContent
    $brokenChainValidation = Invoke-ProductValidator $brokenChainSource $repo `
        -IndependentReviewOutcomePath $secondOutcomePath `
        -ExpectedIndependentReviewOutcomeSha256 $secondOutcomeHash
    Add-Case 'FCH-RECURSIVE-TRANSITIVE-CHAIN-BROKEN-REJECTED' `
        ($brokenChainValidation.ExitCode -ne 0) $brokenChainValidation.Output

    $missingOutcomeValidation = Invoke-ProductValidator $focusedToFocusedSource $repo
    Add-Case 'FCH-INDEPENDENT-OUTCOME-MISSING-REJECTED' `
        ($missingOutcomeValidation.ExitCode -ne 0) $missingOutcomeValidation.Output
    $wrongHashValidation = Invoke-ProductValidator $focusedToFocusedSource $repo `
        -IndependentReviewOutcomePath $outcomePath `
        -ExpectedIndependentReviewOutcomeSha256 ('F' * 64)
    Add-Case 'FCH-INDEPENDENT-OUTCOME-WRONG-HASH-REJECTED' `
        ($wrongHashValidation.ExitCode -ne 0) $wrongHashValidation.Output

    $outcomeMutationCases = @(
        [ordered]@{ Id='FCH-OUTCOME-PREVIOUS-PACKAGE-SHA-REJECTED'; Mutate={ param($j) $j.previousReviewPackageSha256='F'*64 } },
        [ordered]@{ Id='FCH-OUTCOME-REVIEWED-CURRENT-DELTA-SHA-REJECTED'; Mutate={ param($j) $j.reviewedCurrentDeltaSha256='F'*64 } },
        [ordered]@{ Id='FCH-OUTCOME-TRANSITIVE-FULL-SHA-REJECTED'; Mutate={ param($j) $j.transitiveFullReviewBaselineSha256='F'*64 } },
        [ordered]@{ Id='FCH-OUTCOME-FINDING-UNIVERSE-MISMATCH-REJECTED'; Mutate={ param($j) $j.findingOutcomes[0].id='BL339-REV-099';$j.closedFindingIds[0]='BL339-REV-099' } },
        [ordered]@{ Id='FCH-OUTCOME-UNKNOWN-DISPOSITION-REJECTED'; Mutate={ param($j) $j.findingOutcomes[0].disposition='UNKNOWN' } },
        [ordered]@{ Id='FCH-OUTCOME-DUPLICATE-FINDING-ID-REJECTED'; Mutate={ param($j) $duplicate=($j.findingOutcomes[0]|ConvertTo-Json|ConvertFrom-Json);$duplicate|Add-Member -NotePropertyName reviewFinding -NotePropertyValue 'duplicate id' -Force;$j.findingOutcomes=@($j.findingOutcomes+$duplicate) } },
        [ordered]@{ Id='FCH-OUTCOME-OPEN-CLOSED-SET-MISMATCH-REJECTED'; Mutate={ param($j) $j.openFindingIds=@('BL339-REV-003') } },
        [ordered]@{ Id='FCH-OUTCOME-REVIEWER-NOT-INDEPENDENT-REJECTED'; Mutate={ param($j) $j.reviewerRole='PRODUCER' } },
        [ordered]@{ Id='FCH-OUTCOME-WRONG-ARTIFACT-TYPE-REJECTED'; Mutate={ param($j) $j.artifactType='PRODUCER_REVIEW_OUTCOME' } }
    )
    foreach ($outcomeCase in $outcomeMutationCases) {
        $mutantOutcomePath = Join-Path $fixtureRoot ($outcomeCase.Id.ToLowerInvariant() + '.json')
        $mutantOutcome = Read-Json $outcomePath
        & $outcomeCase.Mutate $mutantOutcome
        Write-Json $mutantOutcomePath $mutantOutcome
        $mutantOutcomeHash = (Get-Hash $mutantOutcomePath).ToUpperInvariant()
        $mutantOutcomeValidation = Invoke-ProductValidator $focusedToFocusedSource $repo `
            -IndependentReviewOutcomePath $mutantOutcomePath `
            -ExpectedIndependentReviewOutcomeSha256 $mutantOutcomeHash
        Add-Case $outcomeCase.Id ($mutantOutcomeValidation.ExitCode -ne 0) `
            $mutantOutcomeValidation.Output
    }

    $closedRetarget = Copy-Artifact $focusedToFocusedSource 'focused-to-focused-closed-retarget'
    Set-SingleSourceFindingId $closedRetarget 'BL339-REV-003'
    $closedRetargetValidation = Invoke-ProductValidator $closedRetarget $repo `
        -IndependentReviewOutcomePath $outcomePath `
        -ExpectedIndependentReviewOutcomeSha256 $outcomeHash
    Add-Case 'FCH-CLOSED-FINDING-RETARGETED-REJECTED' `
        ($closedRetargetValidation.ExitCode -ne 0) $closedRetargetValidation.Output
    Add-Case 'FCH-OPEN-FINDING-OMITTED-REJECTED' `
        ($closedRetargetValidation.ExitCode -ne 0) $closedRetargetValidation.Output

    $producerClosed = Copy-Artifact $focusedToFocusedSource 'focused-to-focused-producer-closed'
    $producerLedger = Read-Json (Join-Path $producerClosed 'finding-ledger.json')
    $producerLedger.findings[0].producerStatus = 'CLOSED'
    Write-Json (Join-Path $producerClosed 'finding-ledger.json') $producerLedger
    Update-PackageMetadata $producerClosed
    $producerClosedValidation = Invoke-ProductValidator $producerClosed $repo `
        -IndependentReviewOutcomePath $outcomePath `
        -ExpectedIndependentReviewOutcomeSha256 $outcomeHash
    Add-Case 'FCH-PRODUCER-CLOSED-OUTCOME-OPEN-REJECTED' `
        ($producerClosedValidation.ExitCode -ne 0) $producerClosedValidation.Output

    $tamperedOutcomePath = Join-Path $fixtureRoot 'tampered-after-hash-binding.json'
    [System.IO.File]::WriteAllBytes($tamperedOutcomePath, [byte[]]([System.IO.File]::ReadAllBytes($outcomePath) + 0x20))
    $tamperedOutcomeValidation = Invoke-ProductValidator $focusedToFocusedSource $repo `
        -IndependentReviewOutcomePath $tamperedOutcomePath `
        -ExpectedIndependentReviewOutcomeSha256 $outcomeHash
    Add-Case 'FCH-OUTCOME-TAMPERED-AFTER-HASH-BINDING-REJECTED' `
        ($tamperedOutcomeValidation.ExitCode -ne 0) $tamperedOutcomeValidation.Output

    $tamperedPreviousZip = Join-Path $fixtureRoot 'focused-to-focused-previous-tampered.zip'
    Copy-Item -LiteralPath $focusedPreviousZip -Destination $tamperedPreviousZip
    Add-Content -LiteralPath $tamperedPreviousZip -Value 'tampered' -Encoding ascii
    $tamperedPreviousSource = Copy-Artifact $focusedToFocusedSource 'focused-to-focused-previous-tampered-source'
    $tamperedPreviousBinding = Read-Json (Join-Path $tamperedPreviousSource 'previous-review-binding.json')
    $tamperedPreviousBinding.previousReviewState.historicalPackagePath = $tamperedPreviousZip
    Write-Json (Join-Path $tamperedPreviousSource 'previous-review-binding.json') $tamperedPreviousBinding
    $tamperedPreviousFocused = Read-Json (Join-Path $tamperedPreviousSource 'focused-delta-review-record.json')
    $tamperedPreviousFocused.previousReviewPackage = $tamperedPreviousZip
    Write-Json (Join-Path $tamperedPreviousSource 'focused-delta-review-record.json') $tamperedPreviousFocused
    $tamperedPreviousAssignment = Read-Json (Join-Path $tamperedPreviousSource 'assignment-record.json')
    $tamperedPreviousAssignment.previousReviewBindingSha256 = Get-Hash (
        Join-Path $tamperedPreviousSource 'previous-review-binding.json'
    )
    Write-Json (Join-Path $tamperedPreviousSource 'assignment-record.json') $tamperedPreviousAssignment
    Update-PackageMetadata $tamperedPreviousSource
    $tamperedPreviousValidation = Invoke-ProductValidator $tamperedPreviousSource $repo `
        -IndependentReviewOutcomePath $outcomePath `
        -ExpectedIndependentReviewOutcomeSha256 $outcomeHash
    Add-Case 'FCH-PREVIOUS-FOCUSED-PACKAGE-TAMPERED-REJECTED' `
        ($tamperedPreviousValidation.ExitCode -ne 0) $tamperedPreviousValidation.Output
    Add-Case 'FCH-FULL-REVIEW-PROVENANCE-BROKEN-REJECTED' `
        (@($results | Where-Object id -ceq 'FCH-OUTCOME-TRANSITIVE-FULL-SHA-REJECTED' | `
                Where-Object result -ceq 'PASS').Count -eq 1)

    $unexpectedMemberSource = Copy-Artifact $focusedToFocusedSource 'focused-to-focused-unexpected-outcome-member'
    Move-Item -LiteralPath (Join-Path $unexpectedMemberSource 'previous-independent-review-outcome.json') `
        -Destination (Join-Path $unexpectedMemberSource 'unexpected-independent-review-outcome.json')
    Update-PackageMetadata $unexpectedMemberSource
    $unexpectedMemberValidation = Invoke-ProductValidator $unexpectedMemberSource $repo `
        -IndependentReviewOutcomePath $outcomePath `
        -ExpectedIndependentReviewOutcomeSha256 $outcomeHash
    Add-Case 'FCH-UNEXPECTED-OUTCOME-MEMBER-PATH-REJECTED' `
        ($unexpectedMemberValidation.ExitCode -ne 0) $unexpectedMemberValidation.Output

    $outcomeLinkPath = Join-Path $fixtureRoot 'focused-independent-review-outcome-link.json'
    try {
        $null = New-Item -ItemType SymbolicLink -Path $outcomeLinkPath -Target $outcomePath
    }
    catch {
        $null = New-Item -ItemType Junction -Path $outcomeLinkPath -Target $fixtureRoot
    }
    $outcomeLinkValidation = Invoke-ProductValidator $focusedToFocusedSource $repo `
        -IndependentReviewOutcomePath $outcomeLinkPath `
        -ExpectedIndependentReviewOutcomeSha256 $outcomeHash
    Add-Case 'FCH-OUTCOME-LINK-REPARSE-REJECTED' `
        ($outcomeLinkValidation.ExitCode -ne 0) $outcomeLinkValidation.Output

    $status = 'PASS'
}
catch { $failureMessage = $_.Exception.Message }
finally {
    if ($null -ne $fixtureRoot -and (Test-Path -LiteralPath $fixtureRoot)) {
        Remove-Item -LiteralPath $fixtureRoot -Recurse -Force
    }
    $result = [ordered]@{ schemaVersion=2; status=$status; selected=$results.Count; passed=@($results|Where-Object result -ceq 'PASS').Count; failed=@($results|Where-Object result -ceq 'FAIL').Count; results=@($results); failureMessage=$failureMessage }
    if (-not [string]::IsNullOrWhiteSpace($ResultPath)) { Write-Utf8 -LiteralPath ([System.IO.Path]::GetFullPath($ResultPath)) -Text (($result|ConvertTo-Json -Depth 100)+"`n") }
    [pscustomobject]$result | Format-List
}
if ($status -ceq 'PASS') { exit 0 }
exit 1
