#requires -Version 7.6
[CmdletBinding(DefaultParameterSetName = 'LegacyDocument')]
param(
    [Parameter(Mandatory, ParameterSetName = 'LegacyDocument')][string]$OutputPath,
    [Parameter(Mandatory, ParameterSetName = 'LegacyDocument')][string]$StatusSourcePath,

    [Parameter(Mandatory, ParameterSetName = 'GenericPackage')]
    [ValidateSet('GENERIC_COMMIT_PREPARATION')][string]$Profile,
    [Parameter(Mandatory, ParameterSetName = 'GenericPackage')]
    [ValidateSet('COMMIT_PREPARATION_TO_COMMIT_APPROVAL')][string]$TransitionType,
    [Parameter(Mandatory, ParameterSetName = 'GenericPackage')]
    [ValidatePattern('^BL-[0-9]{3}$')][string]$TaskId,
    [Parameter(Mandatory, ParameterSetName = 'GenericPackage')][string]$SourceDirectory,
    [Parameter(Mandatory, ParameterSetName = 'GenericPackage')][string[]]$AllowedDeltaPath,
    [Parameter(Mandatory, ParameterSetName = 'GenericPackage')][string]$PackagePath
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$status = 'FAIL'
$resolvedOutputPath = $null
$outputHash = $null
$failureMessage = $null
$stagingRoot = $null

function Read-StrictUtf8Json {
    param([Parameter(Mandatory)][string]$LiteralPath)

    $strictUtf8 = [System.Text.UTF8Encoding]::new($false, $true)
    $text = [System.IO.File]::ReadAllText($LiteralPath, $strictUtf8)
    if ($text.Contains([char]0xFFFD)) {
        throw "Stored U+FFFD is not allowed: $LiteralPath"
    }
    return $text | ConvertFrom-Json -Depth 100 -DateKind String
}

function Get-LowerSha256 {
    param([Parameter(Mandatory)][string]$LiteralPath)
    return (Get-FileHash -LiteralPath $LiteralPath -Algorithm SHA256).Hash.ToLowerInvariant()
}

function Assert-JsonSchema {
    param(
        [Parameter(Mandatory)][string]$LiteralPath,
        [Parameter(Mandatory)][string]$SchemaPath
    )

    $text = [System.IO.File]::ReadAllText($LiteralPath, [System.Text.UTF8Encoding]::new($false, $true))
    if (-not ($text | Test-Json -SchemaFile $SchemaPath -ErrorAction Stop)) {
        throw "Schema validation failed: $([System.IO.Path]::GetFileName($LiteralPath))"
    }
}

try {
    $utf8 = [System.Text.UTF8Encoding]::new($false)

    if ($PSCmdlet.ParameterSetName -ceq 'LegacyDocument') {
        $source = Read-StrictUtf8Json -LiteralPath ([System.IO.Path]::GetFullPath($StatusSourcePath))
        $requiredProperties = @(
            'schemaVersion', 'taskId', 'correctionMode', 'status',
            'classicReviewReady', 'targetFindings', 'pendingFindings',
            'closedFindings', 'run007Status', 'commitPreparationApproved',
            'commitAuthorized', 'requiredReviewMode', 'targetFindingCount',
            'correctedFindingCount', 'pendingDeltaFindingCount',
            'closedFindingCount', 'openFindingCount', 'nextAction'
        )
        $missing = @($requiredProperties | Where-Object {
                $_ -notin @($source.PSObject.Properties.Name)
            })
        if ($missing.Count -gt 0) {
            throw "Status source is missing required properties: $($missing -join ', ')"
        }

        $contract = [ordered]@{}
        foreach ($property in $requiredProperties) {
            $contract[$property] = $source.$property
        }
        $contractJson = $contract | ConvertTo-Json -Depth 20
        $handoff = @"
# BL-333/BL-334 fourth bundled correction handoff

<!-- BEGIN GOVERNANCE-HANDOFF-STATUS -->
Status: $($contract.status)
CorrectionMode: $($contract.correctionMode)
TargetFindingCount: $($contract.targetFindingCount)
CorrectedFindingCount: $($contract.correctedFindingCount)
PendingDeltaFindingCount: $($contract.pendingDeltaFindingCount)
ClosedFindingCount: $($contract.closedFindingCount)
OpenFindingCount: $($contract.openFindingCount)
ClassicReviewReady: $(([string]$contract.classicReviewReady).ToLowerInvariant())
TargetFindings: $(@($contract.targetFindings) -join ',')
PendingFindings: $(@($contract.pendingFindings) -join ',')
ClosedFindings: $(@($contract.closedFindings) -join ',')
Run007Status: $($contract.run007Status)
CommitPreparationApproved: $(([string]$contract.commitPreparationApproved).ToLowerInvariant())
CommitAuthorized: $(([string]$contract.commitAuthorized).ToLowerInvariant())
RequiredReviewMode: $($contract.requiredReviewMode)
NextAction: $($contract.nextAction)
<!-- END GOVERNANCE-HANDOFF-STATUS -->

<!-- BEGIN GOVERNANCE-HANDOFF-CONTRACT -->
$contractJson
<!-- END GOVERNANCE-HANDOFF-CONTRACT -->
"@
        $resolvedOutputPath = [System.IO.Path]::GetFullPath($OutputPath)
        $outputDirectory = Split-Path -Parent $resolvedOutputPath
        if (-not (Test-Path -LiteralPath $outputDirectory -PathType Container)) {
            [void][System.IO.Directory]::CreateDirectory($outputDirectory)
        }
        [System.IO.File]::WriteAllText($resolvedOutputPath, $handoff, $utf8)
    }
    else {
        $resolvedSourceDirectory = [System.IO.Path]::GetFullPath($SourceDirectory)
        $resolvedOutputPath = [System.IO.Path]::GetFullPath($PackagePath)
        if (-not (Test-Path -LiteralPath $resolvedSourceDirectory -PathType Container)) {
            throw "Source directory does not exist: $resolvedSourceDirectory"
        }
        if (Test-Path -LiteralPath $resolvedOutputPath) {
            throw "Package path already exists; productive package writes are single-attempt: $resolvedOutputPath"
        }

        $requiredSourceNames = @(
            'assignment-record.json', 'completion-report.json',
            'current-delta.patch', 'independent-review-evidence.json',
            'report.md', 'scope-inventory.json', 'task.patch',
            'validation-summary.json'
        )
        $actualSourceNames = @(
            Get-ChildItem -LiteralPath $resolvedSourceDirectory -File |
                ForEach-Object Name | Sort-Object
        )
        if (($actualSourceNames -join "`n") -cne (($requiredSourceNames | Sort-Object) -join "`n")) {
            throw 'Generic source directory must contain exactly the eight canonical source artifacts.'
        }

        $governanceRoot = Join-Path (Split-Path -Parent $PSScriptRoot) 'Governance'
        $schemaMap = [ordered]@{
            'assignment-record.json' = 'generic-assignment-record.schema.json'
            'completion-report.json' = 'generic-completion-report.schema.json'
            'independent-review-evidence.json' = 'generic-independent-review-evidence.schema.json'
            'scope-inventory.json' = 'generic-scope-inventory.schema.json'
            'validation-summary.json' = 'generic-validation-summary.schema.json'
        }
        foreach ($name in $schemaMap.Keys) {
            $sourcePath = Join-Path $resolvedSourceDirectory $name
            Assert-JsonSchema -LiteralPath $sourcePath -SchemaPath (Join-Path $governanceRoot $schemaMap[$name])
            $typedSource = Read-StrictUtf8Json -LiteralPath $sourcePath
            if ([string]$typedSource.taskId -cne $TaskId -or
                [string]$typedSource.profile -cne $Profile) {
                throw "Task/profile mismatch in $name"
            }
        }

        $assignment = Read-StrictUtf8Json -LiteralPath (Join-Path $resolvedSourceDirectory 'assignment-record.json')
        $completion = Read-StrictUtf8Json -LiteralPath (Join-Path $resolvedSourceDirectory 'completion-report.json')
        $review = Read-StrictUtf8Json -LiteralPath (Join-Path $resolvedSourceDirectory 'independent-review-evidence.json')
        $scope = Read-StrictUtf8Json -LiteralPath (Join-Path $resolvedSourceDirectory 'scope-inventory.json')
        $scopeInventorySha256 = Get-LowerSha256 -LiteralPath (Join-Path $resolvedSourceDirectory 'scope-inventory.json')
        $taskPatchSha256 = Get-LowerSha256 -LiteralPath (Join-Path $resolvedSourceDirectory 'task.patch')
        $currentDeltaSha256 = Get-LowerSha256 -LiteralPath (Join-Path $resolvedSourceDirectory 'current-delta.patch')
        if ([string]$assignment.transitionType -cne $TransitionType -or
            [string]$completion.transitionType -cne $TransitionType) {
            throw 'Transition discriminator mismatch.'
        }
        $scopePaths = @(
            $scope.entries |
                Where-Object { [string]$_.inclusionDecision -ceq 'INCLUDE' } |
                ForEach-Object { [string]$_.path }
        )
        if ((($scopePaths | Sort-Object) -join "`n") -cne
            (($AllowedDeltaPath | Sort-Object) -join "`n")) {
            throw 'Allowed delta paths do not match the scope inventory.'
        }
        $bindingSources = @($assignment, $completion, $review)
        foreach ($binding in $bindingSources) {
            if ([string]$binding.repository -cne [string]$scope.repository -or
                [string]$binding.baselineCommit -cne [string]$scope.baselineCommit -or
                [string]$binding.currentCommit -cne [string]$scope.currentCommit -or
                [string]$binding.branch -cne [string]$scope.branch -or
                [string]$binding.scopeInventorySha256 -cne $scopeInventorySha256 -or
                ((@($binding.allowedDeltaPaths | Sort-Object) -join "`n") -cne (@($scope.allowedDeltaPaths | Sort-Object) -join "`n")) -or
                ((@($binding.excludedDeltaPaths | Sort-Object) -join "`n") -cne (@($scope.excludedDeltaPaths | Sort-Object) -join "`n"))) {
                throw 'Repository, current-state, scope-hash, or delta-path binding mismatch.'
            }
        }
        foreach ($binding in @($assignment, $completion)) {
            if ([string]$binding.taskPatchSha256 -cne $taskPatchSha256 -or
                [string]$binding.currentDeltaSha256 -cne $currentDeltaSha256) {
                throw 'Patch hash binding mismatch.'
            }
        }
        foreach ($reviewed in @($review.reviewedArtifacts)) {
            $expectedHash = if ([string]$reviewed.path -ceq 'task.patch') {
                $taskPatchSha256
            }
            else {
                $currentDeltaSha256
            }
            if ([string]$reviewed.sha256 -cne $expectedHash) {
                throw 'Independent-review patch hash binding mismatch.'
            }
        }
        $assignmentFindingJson = @($assignment.findingIds) | ConvertTo-Json -Compress
        $completionFindingJson = @($completion.findingIds) | ConvertTo-Json -Compress
        $reviewFindingJson = @($review.findingIds) | ConvertTo-Json -Compress
        if ($assignmentFindingJson -cne $completionFindingJson -or
            $assignmentFindingJson -cne $reviewFindingJson) {
            throw 'Finding IDs do not agree across generic evidence.'
        }

        $contract = [ordered]@{
            schemaVersion = 1
            taskId = $TaskId
            repository = [string]$assignment.repository
            baselineCommit = [string]$assignment.baselineCommit
            currentCommit = [string]$assignment.currentCommit
            branch = [string]$assignment.branch
            transitionType = $TransitionType
            profile = $Profile
            status = [string]$completion.status
            classicReviewReady = [bool]$completion.classicReviewReady
            findingIds = @($completion.findingIds)
            reviewStatus = [string]$review.result
            commitAuthorized = $false
            scopeInventorySha256 = $scopeInventorySha256
            taskPatchSha256 = $taskPatchSha256
            currentDeltaSha256 = $currentDeltaSha256
            allowedDeltaPaths = @($AllowedDeltaPath)
            excludedDeltaPaths = @($scope.excludedDeltaPaths)
            nextAction = [string]$completion.nextAction
        }
        $contractJson = $contract | ConvertTo-Json -Depth 20
        if (-not ($contractJson | Test-Json -SchemaFile (Join-Path $governanceRoot 'generic-handoff-contract.schema.json'))) {
            throw 'Generated generic handoff contract failed its schema.'
        }

        $stagingRoot = Join-Path ([System.IO.Path]::GetTempPath()) ('flashgate-generic-handoff-' + [guid]::NewGuid().ToString('N'))
        [void][System.IO.Directory]::CreateDirectory($stagingRoot)
        foreach ($name in $requiredSourceNames) {
            [System.IO.File]::Copy((Join-Path $resolvedSourceDirectory $name), (Join-Path $stagingRoot $name), $false)
        }

        $handoff = @"
# $TaskId generic commit-preparation handoff

<!-- BEGIN GOVERNANCE-HANDOFF-STATUS -->
TaskId: $TaskId
TransitionType: $TransitionType
Profile: $Profile
Status: $($contract.status)
ClassicReviewReady: $(([string]$contract.classicReviewReady).ToLowerInvariant())
FindingCount: $(@($contract.findingIds).Count)
ReviewStatus: $($review.result)
CommitAuthorized: false
AllowedDeltaPaths: $(@($contract.allowedDeltaPaths) -join ',')
NextAction: $($completion.nextAction)
<!-- END GOVERNANCE-HANDOFF-STATUS -->

<!-- BEGIN GOVERNANCE-HANDOFF-CONTRACT -->
$contractJson
<!-- END GOVERNANCE-HANDOFF-CONTRACT -->
"@
        [System.IO.File]::WriteAllText((Join-Path $stagingRoot 'HANDOFF.md'), $handoff, $utf8)

        $inventoryEntries = @(
            Get-ChildItem -LiteralPath $stagingRoot -File |
                Sort-Object Name |
                ForEach-Object {
                    [ordered]@{
                        path = $_.Name
                        sha256 = Get-LowerSha256 -LiteralPath $_.FullName
                        length = [int64]$_.Length
                    }
                }
        )
        $inventory = [ordered]@{
            schemaVersion = 1
            taskId = $TaskId
            profile = $Profile
            transitionType = $TransitionType
            entries = $inventoryEntries
        }
        $inventoryPath = Join-Path $stagingRoot 'package-inventory.json'
        [System.IO.File]::WriteAllText($inventoryPath, ($inventory | ConvertTo-Json -Depth 20), $utf8)
        Assert-JsonSchema -LiteralPath $inventoryPath -SchemaPath (Join-Path $governanceRoot 'generic-package-inventory.schema.json')

        [string[]]$manifestNames = @(
            Get-ChildItem -LiteralPath $stagingRoot -File | ForEach-Object Name
        )
        [array]::Sort($manifestNames, [System.StringComparer]::Ordinal)
        $manifestLines = @(
            $manifestNames | ForEach-Object {
                $manifestFile = Get-Item -LiteralPath (Join-Path $stagingRoot $_)
                "$(Get-LowerSha256 -LiteralPath $manifestFile.FullName)  $($manifestFile.Length)  $($manifestFile.Name)"
            }
        )
        [System.IO.File]::WriteAllText(
            (Join-Path $stagingRoot 'MANIFEST.sha256'),
            (($manifestLines -join "`n") + "`n"),
            $utf8
        )

        $outputDirectory = Split-Path -Parent $resolvedOutputPath
        if (-not (Test-Path -LiteralPath $outputDirectory -PathType Container)) {
            [void][System.IO.Directory]::CreateDirectory($outputDirectory)
        }
        Add-Type -AssemblyName System.IO.Compression
        Add-Type -AssemblyName System.IO.Compression.FileSystem
        $zipStream = [System.IO.FileStream]::new(
            $resolvedOutputPath,
            [System.IO.FileMode]::CreateNew,
            [System.IO.FileAccess]::Write,
            [System.IO.FileShare]::None
        )
        try {
            $archive = [System.IO.Compression.ZipArchive]::new(
                $zipStream,
                [System.IO.Compression.ZipArchiveMode]::Create,
                $false
            )
            try {
                foreach ($file in @(Get-ChildItem -LiteralPath $stagingRoot -File | Sort-Object Name)) {
                    $entry = $archive.CreateEntry($file.Name, [System.IO.Compression.CompressionLevel]::Optimal)
                    $entry.LastWriteTime = [datetimeoffset]::new(2000, 1, 1, 0, 0, 0, [timespan]::Zero)
                    $entryStream = $entry.Open()
                    try {
                        $bytes = [System.IO.File]::ReadAllBytes($file.FullName)
                        $entryStream.Write($bytes, 0, $bytes.Length)
                    }
                    finally {
                        $entryStream.Dispose()
                    }
                }
            }
            finally {
                $archive.Dispose()
            }
        }
        finally {
            $zipStream.Dispose()
        }
    }

    $outputHash = Get-LowerSha256 -LiteralPath $resolvedOutputPath
    $status = 'PASS'
}
catch {
    $failureMessage = $_.Exception.Message
}
finally {
    if ($null -ne $stagingRoot -and (Test-Path -LiteralPath $stagingRoot -PathType Container)) {
        Remove-Item -LiteralPath $stagingRoot -Recurse -Force
    }
    [pscustomobject]@{
        Status = $status
        HandoffPath = $resolvedOutputPath
        HandoffSHA256 = $outputHash
        FailureMessage = $failureMessage
        NextAction = if ($status -ceq 'PASS') {
            'Validate the generated handoff with Test-GovernanceConsistency.ps1.'
        }
        else {
            'Correct the typed source artifacts and generate one new package path.'
        }
    } | Format-List
}

if ($status -ceq 'PASS') { exit 0 }
exit 1
