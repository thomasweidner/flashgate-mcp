#requires -Version 7.6
[CmdletBinding()]
param(
    [string]$RepositoryRoot = (Split-Path -Parent $PSScriptRoot),
    [string]$ValidatorPath = (Join-Path $PSScriptRoot 'Test-GenericGovernanceHandoff.ps1'),
    [string]$ResultPath,
    [string[]]$CaseName = @()
)

enum SyntheticHostPathClass {
    WINDOWS_PRIVATE_USER
    WINDOWS_PRIVATE_UNC
    UNIX_PRIVATE_HOME
    MACOS_PRIVATE_USER
    UNIX_PRIVATE_TEMP
    UNDECLARED_WINDOWS_ABSOLUTE
    UNDECLARED_UNIX_ABSOLUTE
}

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$status = 'FAIL'
$failureMessage = $null
$temporaryRoot = $null
$authoritativeRoot = $null
$fixtureAllowedDeltaPaths = @()
$fixtureIncludedPaths = @()
$results = [System.Collections.Generic.List[object]]::new()
$expectedFixtureCount = 72

. (Join-Path $PSScriptRoot 'GenericGovernanceGitEvidence.ps1')

function Write-Utf8 {
    param([string]$Path, [string]$Text)
    [System.IO.File]::WriteAllText($Path, $Text, [System.Text.UTF8Encoding]::new($false))
}

function Get-LowerHash {
    param([string]$Path)
    return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}

function Test-CaseSelected {
    param([string]$Name)
    return @($CaseName).Count -eq 0 -or $Name -in $CaseName
}

function New-SyntheticHostPath {
    [OutputType([string])]
    param(
        [Parameter(Mandatory)]
        [SyntheticHostPathClass]$PathClass
    )

    switch ($PathClass.ToString()) {
        'WINDOWS_PRIVATE_USER' {
            return 'C:' + '\' + (@('Users', 'SyntheticPrivateUser', 'secret.txt') -join '\')
        }
        'WINDOWS_PRIVATE_UNC' {
            return '\' + '\' + (@('synthetic-private-server', 'SyntheticPrivateShare', 'secret.txt') -join '\')
        }
        'UNIX_PRIVATE_HOME' {
            return '/' + (@('home', 'SyntheticPrivateUser', 'secret.txt') -join '/')
        }
        'MACOS_PRIVATE_USER' {
            return '/' + (@('Users', 'SyntheticPrivateUser', 'secret.txt') -join '/')
        }
        'UNIX_PRIVATE_TEMP' {
            return '/' + (@('tmp', 'synthetic-private-secret.txt') -join '/')
        }
        'UNDECLARED_WINDOWS_ABSOLUTE' {
            return 'Z:' + '\' + (@('synthetic-undeclared', 'secret.txt') -join '\')
        }
        'UNDECLARED_UNIX_ABSOLUTE' {
            return '/' + (@('var', 'synthetic-undeclared', 'secret.txt') -join '/')
        }
    }

    throw "Unsupported synthetic host-path class: $PathClass"
}

function Invoke-SyntheticHostPathFactoryCase {
    $name = 'positive-typed-synthetic-host-path-factory'
    if (-not (Test-CaseSelected -Name $name)) {
        return
    }

    $expected = [ordered]@{
        WINDOWS_PRIVATE_USER = 'C:' + '\' + (@('Users', 'SyntheticPrivateUser', 'secret.txt') -join '\')
        WINDOWS_PRIVATE_UNC = '\' + '\' + (@('synthetic-private-server', 'SyntheticPrivateShare', 'secret.txt') -join '\')
        UNIX_PRIVATE_HOME = '/' + (@('home', 'SyntheticPrivateUser', 'secret.txt') -join '/')
        MACOS_PRIVATE_USER = '/' + (@('Users', 'SyntheticPrivateUser', 'secret.txt') -join '/')
        UNIX_PRIVATE_TEMP = '/' + (@('tmp', 'synthetic-private-secret.txt') -join '/')
        UNDECLARED_WINDOWS_ABSOLUTE = 'Z:' + '\' + (@('synthetic-undeclared', 'secret.txt') -join '\')
        UNDECLARED_UNIX_ABSOLUTE = '/' + (@('var', 'synthetic-undeclared', 'secret.txt') -join '/')
    }
    $enumNames = @([enum]::GetNames([SyntheticHostPathClass]))
    $observed = @(
        foreach ($enumName in $enumNames) {
            New-SyntheticHostPath -PathClass ([SyntheticHostPathClass]::$enumName)
        }
    )
    $mappingPass = $enumNames.Count -eq $expected.Count
    foreach ($enumName in $enumNames) {
        $mappingPass = $mappingPass -and (
            (New-SyntheticHostPath -PathClass ([SyntheticHostPathClass]::$enumName)) -ceq [string]$expected[$enumName]
        )
    }
    $arbitraryRejected = $false
    try {
        $null = New-SyntheticHostPath -PathClass 'ARBITRARY_HOST_PATH_CLASS' -ErrorAction Stop
    }
    catch {
        $arbitraryRejected = $true
    }
    $passed = $mappingPass -and $arbitraryRejected -and @($observed | Sort-Object -Unique).Count -eq $expected.Count
    [void]$results.Add([pscustomobject]@{
        name = $name
        result = if ($passed) { 'PASS' } else { 'FAIL' }
        expectedExit = 0
        actualExit = if ($passed) { 0 } else { 1 }
        expectedFailedCheckId = ''
        evidence = if ($passed) { 'Seven closed path classes produced deterministic, unique values; an arbitrary class was rejected.' } else { 'Typed synthetic host-path factory contract failed.' }
    })
}

function Invoke-FixtureGitText {
    param([Parameter(Mandatory)][string[]]$Argument)

    $startInfo = [System.Diagnostics.ProcessStartInfo]::new()
    $startInfo.FileName = (Get-Command git -ErrorAction Stop).Source
    $startInfo.UseShellExecute = $false
    $startInfo.RedirectStandardOutput = $true
    $startInfo.RedirectStandardError = $true
    [void]$startInfo.ArgumentList.Add('-C')
    [void]$startInfo.ArgumentList.Add($authoritativeRoot)
    foreach ($item in $Argument) { [void]$startInfo.ArgumentList.Add($item) }
    $process = [System.Diagnostics.Process]::new()
    $process.StartInfo = $startInfo
    try {
        [void]$process.Start()
        $standardOutput = $process.StandardOutput.ReadToEnd()
        $standardError = $process.StandardError.ReadToEnd()
        $process.WaitForExit()
        if ($process.ExitCode -ne 0) {
            throw "Read-only fixture Git query failed with exit code $($process.ExitCode)."
        }
        return $standardOutput
    }
    finally { $process.Dispose() }
}

function Get-AuthoritativeFixtureEntries {
    param([Parameter(Mandatory)][string]$Root, [Parameter(Mandatory)][string]$BaselineCommit, [Parameter(Mandatory)][string[]]$IncludedPath)
    $entries = [System.Collections.Generic.List[object]]::new()
    foreach ($actual in @(Get-GenericStatusEvidence -Root $Root -BaselineCommit $BaselineCommit)) {
        if ([bool]$actual.Staged) { throw 'Fixture worktree must not contain staged changes.' }
        $expanded = if ([string]$actual.GitStatus -ceq 'TRACKED_RENAMED') { @([string]$actual.PreviousPath,[string]$actual.Path) } else { @([string]$actual.Path) }
        $decision = if (@($expanded | Where-Object { $_ -in $IncludedPath }).Count -gt 0) { 'INCLUDE' } else { 'EXCLUDE' }
        $entry = [ordered]@{ path=[string]$actual.Path; gitStatus=[string]$actual.GitStatus; tracked=[bool]$actual.Tracked; staged=$false }
        if ([string]$actual.GitStatus -ceq 'TRACKED_RENAMED') { $entry.previousPath=[string]$actual.PreviousPath }
        if ($null -ne $actual.Preimage) { $entry.preimage=$actual.Preimage }
        if ($null -ne $actual.Postimage) { $entry.postimage=$actual.Postimage }
        if ([bool]$actual.PostimageAbsent) { $entry.postimageAbsent=$true }
        $entry.inclusionDecision=$decision
        $entry.reason=if($decision -ceq 'INCLUDE'){'Selected for the authoritative fixture delta.'}else{'Relevant fixture path explicitly excluded.'}
        [void]$entries.Add($entry)
    }
    return @($entries | Sort-Object path)
}

function New-GenericSource {
    param(
        [string]$Path,
        [string]$TaskId = 'BL-230',
        [string[]]$FindingIds = @(),
        [bool]$ClassicReviewReady = $true,
        [string]$PatchNarrative = '',
        [string]$ReportNarrative = '',
        [object[]]$AllowedHostReferences = @(),
        [string]$FixtureRepositoryRoot = $authoritativeRoot,
        [string[]]$IncludedPath = @('BACKLOG.md')
    )
    [void][System.IO.Directory]::CreateDirectory($Path)
    if (-not [string]::IsNullOrWhiteSpace($PatchNarrative)) {
        throw 'Authoritative generic fixture patches cannot contain narrative prefixes.'
    }
    $script:authoritativeRoot = [System.IO.Path]::GetFullPath($FixtureRepositoryRoot)
    $repository = (Invoke-FixtureGitText -Argument @('config', '--get', 'remote.origin.url')).Trim()
    $baselineCommit = (Invoke-FixtureGitText -Argument @('rev-parse', 'HEAD')).Trim()
    $currentCommit = $baselineCommit
    $branch = (Invoke-FixtureGitText -Argument @('branch', '--show-current')).Trim()
    $profile = 'GENERIC_COMMIT_PREPARATION'
    $transition = 'COMMIT_PREPARATION_TO_COMMIT_APPROVAL'
    $scopeEntries = @(Get-AuthoritativeFixtureEntries -Root $script:authoritativeRoot -BaselineCommit $baselineCommit -IncludedPath $IncludedPath)
    $includedEntries = @($scopeEntries | Where-Object inclusionDecision -CEQ 'INCLUDE')
    $excludedEntries = @($scopeEntries | Where-Object inclusionDecision -CEQ 'EXCLUDE')
    if ($includedEntries.Count -eq 0) {
        throw "Fixture produced no INCLUDE entry. Root=$script:authoritativeRoot Requested=$($IncludedPath -join ',') Observed=$(@($scopeEntries | ForEach-Object path) -join ',')"
    }
    $allowedDeltaPaths = @(Get-GenericScopePaths -Entry $includedEntries | Sort-Object)
    $excludedDeltaPaths = @(Get-GenericScopePaths -Entry $excludedEntries | Sort-Object)
    $patchBytes = Get-GenericDeltaBytes -Root $script:authoritativeRoot -BaselineCommit $baselineCommit -IncludedEntry $includedEntries
    if ($patchBytes.Length -eq 0) { throw 'Authoritative fixture delta must not be empty.' }
    [System.IO.File]::WriteAllBytes((Join-Path $Path 'task.patch'), $patchBytes)
    [System.IO.File]::WriteAllBytes((Join-Path $Path 'current-delta.patch'), $patchBytes)
    $taskHash = Get-LowerHash -Path (Join-Path $Path 'task.patch')
    $deltaHash = Get-LowerHash -Path (Join-Path $Path 'current-delta.patch')
    $script:fixtureAllowedDeltaPaths = $allowedDeltaPaths
    $scannedArtifacts = @(
        'HANDOFF.md', 'assignment-record.json', 'completion-report.json',
        'current-delta.patch', 'independent-review-evidence.json',
        'report.md', 'task.patch', 'validation-summary.json'
    )
    $artifactsWithAllowedReferences = @($AllowedHostReferences | ForEach-Object { [string]$_.artifact } | Sort-Object -Unique)
    $scope = [ordered]@{
        schemaVersion = 1; taskId = $TaskId; profile = $profile
        repository = $repository; baselineCommit = $baselineCommit
        currentCommit = $currentCommit; branch = $branch
        allowedDeltaPaths = $allowedDeltaPaths
        excludedDeltaPaths = $excludedDeltaPaths
        entries = $scopeEntries
        hostPathPolicy = [ordered]@{
            hostPathFreeArtifacts = @($scannedArtifacts | Where-Object { $_ -notin $artifactsWithAllowedReferences })
            allowedReferences = @($AllowedHostReferences)
        }
    }
    Write-Utf8 -Path (Join-Path $Path 'scope-inventory.json') -Text ($scope | ConvertTo-Json -Depth 30)
    $scopeHash = Get-LowerHash -Path (Join-Path $Path 'scope-inventory.json')
    $readinessStatus = if ($ClassicReviewReady) { 'CLASSIC_REVIEW_READY' } else { 'NOT_CLASSIC_REVIEW_READY' }
    $reviewResult = if ($ClassicReviewReady) { 'PASS' } else { 'FAIL' }
    $warningCount = if ($ClassicReviewReady) { 0 } else { 1 }
    $failureCount = if ($ClassicReviewReady) { 0 } else { 1 }
    $assignment = [ordered]@{
        schemaVersion = 1; taskId = $TaskId
        repository = $repository; baselineCommit = $baselineCommit
        currentCommit = $currentCommit; branch = $branch
        executionMode = 'COMMIT_PREPARATION'; checkpoint = 'PRE_COMMIT'
        profile = $profile; transitionType = $transition
        changeTriggerReviewResult = 'NEW_BACKLOG_REGISTERED'
        classicReviewReady = $ClassicReviewReady; findingIds = @($FindingIds)
        commitAuthorized = $false; scopeInventorySha256 = $scopeHash
        taskPatchSha256 = $taskHash; currentDeltaSha256 = $deltaHash
        allowedDeltaPaths = $allowedDeltaPaths; excludedDeltaPaths = $excludedDeltaPaths
    }
    $completion = [ordered]@{
        schemaVersion = 1; taskId = $TaskId; repository = $repository
        baselineCommit = $baselineCommit; currentCommit = $currentCommit; branch = $branch; profile = $profile
        transitionType = $transition; status = $readinessStatus
        classicReviewReady = $ClassicReviewReady; findingIds = @($FindingIds)
        commitAuthorized = $false; warningCount = $warningCount
        failureCount = $failureCount; scopeInventorySha256 = $scopeHash
        taskPatchSha256 = $taskHash; currentDeltaSha256 = $deltaHash
        allowedDeltaPaths = $allowedDeltaPaths; excludedDeltaPaths = $excludedDeltaPaths
        nextAction = 'Obtain explicit commit approval after independent Classic review.'
    }
    $review = [ordered]@{
        schemaVersion = 1; taskId = $TaskId; repository = $repository
        baselineCommit = $baselineCommit; currentCommit = $currentCommit; branch = $branch; profile = $profile
        reviewMode = 'INDEPENDENT_REVIEW'; external = $true; result = $reviewResult
        reviewerIndependencePreserved = $true; findingIds = @($FindingIds)
        scopeInventorySha256 = $scopeHash
        allowedDeltaPaths = $allowedDeltaPaths; excludedDeltaPaths = $excludedDeltaPaths
        reviewedArtifacts = @(
            [ordered]@{ path = 'task.patch'; sha256 = $taskHash },
            [ordered]@{ path = 'current-delta.patch'; sha256 = $deltaHash }
        )
    }
    $validation = [ordered]@{
        schemaVersion = 1; taskId = $TaskId; profile = $profile
        result = $reviewResult
        checks = @([ordered]@{ id = 'fixture-validation'; result = $reviewResult })
        warningCount = $warningCount; failureCount = $failureCount
    }
    $reportContract = [ordered]@{
        schemaVersion = 1; taskId = $TaskId; repository = $repository
        baselineCommit = $baselineCommit; currentCommit = $currentCommit; branch = $branch
        transitionType = $transition
        profile = $profile; status = $readinessStatus
        classicReviewReady = $ClassicReviewReady; findingIds = @($FindingIds)
        reviewStatus = $reviewResult; commitAuthorized = $false
        scopeInventorySha256 = $scopeHash; taskPatchSha256 = $taskHash
        currentDeltaSha256 = $deltaHash; allowedDeltaPaths = $allowedDeltaPaths
        excludedDeltaPaths = $excludedDeltaPaths
        nextAction = $completion.nextAction
    }
    Write-Utf8 -Path (Join-Path $Path 'assignment-record.json') -Text ($assignment | ConvertTo-Json -Depth 20)
    Write-Utf8 -Path (Join-Path $Path 'completion-report.json') -Text ($completion | ConvertTo-Json -Depth 20)
    Write-Utf8 -Path (Join-Path $Path 'independent-review-evidence.json') -Text ($review | ConvertTo-Json -Depth 20)
    Write-Utf8 -Path (Join-Path $Path 'validation-summary.json') -Text ($validation | ConvertTo-Json -Depth 20)
    Write-Utf8 -Path (Join-Path $Path 'report.md') -Text @"
# Generic governance fixture report

$ReportNarrative

<!-- BEGIN GOVERNANCE-REPORT-CONTRACT -->
$($reportContract | ConvertTo-Json -Depth 20)
<!-- END GOVERNANCE-REPORT-CONTRACT -->
"@
}

function Invoke-Generator {
    param([string]$Source, [string]$Package, [string]$TaskId = 'BL-230')
    $pwsh = (Get-Command pwsh -ErrorAction Stop).Source
    $quote = { param([string]$Value) "'" + $Value.Replace("'", "''") + "'" }
    $scriptLiteral = & $quote (Join-Path $PSScriptRoot 'New-GovernanceHandoff.ps1')
    $taskLiteral = & $quote $TaskId
    $sourceLiteral = & $quote $Source
    $packageLiteral = & $quote $Package
    $allowedLiteral = '@(' + (@($fixtureAllowedDeltaPaths | ForEach-Object { & $quote ([string]$_) }) -join ',') + ')'
    $command = "& $scriptLiteral -Profile GENERIC_COMMIT_PREPARATION -TransitionType COMMIT_PREPARATION_TO_COMMIT_APPROVAL -TaskId $taskLiteral -SourceDirectory $sourceLiteral -AllowedDeltaPath $allowedLiteral -PackagePath $packageLiteral"
    $encodedCommand = [Convert]::ToBase64String([System.Text.Encoding]::Unicode.GetBytes($command))
    $output = @(& $pwsh -NoLogo -NoProfile -EncodedCommand $encodedCommand)
    if ($LASTEXITCODE -ne 0) { throw "Generator failed: $($output -join ' | ')" }
}

function Invoke-ValidationCase {
    param(
        [string]$Name,
        [string]$Package,
        [int]$ExpectedExit,
        [string]$ExpectedFailedCheckId,
        [string]$CaseAuthoritativeRoot = $authoritativeRoot
    )
    if (-not (Test-CaseSelected -Name $Name)) {
        return
    }
    $pwsh = (Get-Command pwsh -ErrorAction Stop).Source
    $caseReportPath = Join-Path $temporaryRoot ('validator-' + $Name + '.json')
    $output = @(& $pwsh -NoLogo -NoProfile -File $ValidatorPath `
        -PackagePath $Package -RepositoryRoot $RepositoryRoot `
        -AuthoritativeRepositoryRoot $CaseAuthoritativeRoot -ReportPath $caseReportPath)
    $actualExit = $LASTEXITCODE
    $specificCheckPassed = $true
    if (-not [string]::IsNullOrWhiteSpace($ExpectedFailedCheckId)) {
        if (-not (Test-Path -LiteralPath $caseReportPath -PathType Leaf)) {
            $specificCheckPassed = $false
        }
        else {
            $caseReport = Get-Content -LiteralPath $caseReportPath -Raw | ConvertFrom-Json -Depth 30
            $failedChecks = @($caseReport.checks | Where-Object { $_.Result -ceq 'FAIL' } | ForEach-Object Id)
            $specificCheckPassed = (
                $ExpectedFailedCheckId -in $failedChecks -and
                @($failedChecks | Where-Object { $_ -ceq $ExpectedFailedCheckId }).Count -eq 1
            )
        }
    }
    $passed = $actualExit -eq $ExpectedExit -and $specificCheckPassed
    [void]$results.Add([pscustomobject]@{
        name = $Name; result = if ($passed) { 'PASS' } else { 'FAIL' }
        expectedExit = $ExpectedExit; actualExit = $actualExit
        expectedFailedCheckId = $ExpectedFailedCheckId
        evidence = if ($passed) { '' } else { $output -join ' | ' }
    })
}

function Copy-ZipToDirectory {
    param([string]$Zip, [string]$Directory)
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [System.IO.Compression.ZipFile]::ExtractToDirectory($Zip, $Directory)
}

function New-ZipFromDirectory {
    param([string]$Directory, [string]$Zip)
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [System.IO.Compression.ZipFile]::CreateFromDirectory($Directory, $Zip)
}

function New-MutatedZip {
    param(
        [string]$Name,
        [string]$BaselineZip,
        [scriptblock]$Mutation,
        [object[]]$MutationArgument = @(),
        [switch]$Resign
    )
    $directory = Join-Path $temporaryRoot ('mut-' + $Name)
    [void][System.IO.Directory]::CreateDirectory($directory)
    Copy-ZipToDirectory -Zip $BaselineZip -Directory $directory
    & $Mutation $directory @MutationArgument
    if ($Resign) {
        Repair-SemanticPackageSignatures -Directory $directory
    }
    $zip = Join-Path $temporaryRoot ($Name + '.zip')
    New-ZipFromDirectory -Directory $directory -Zip $zip
    return $zip
}

function Sync-BindingObject {
    param([object]$Binding, [object]$Scope, [string]$ScopeHash, [string]$TaskPatchHash, [string]$CurrentDeltaHash)

    foreach ($propertyName in @('repository', 'baselineCommit', 'currentCommit', 'branch')) {
        if ($propertyName -in $Binding.PSObject.Properties.Name) {
            $Binding.$propertyName = $Scope.$propertyName
        }
    }
    if ('scopeInventorySha256' -in $Binding.PSObject.Properties.Name) {
        $Binding.scopeInventorySha256 = $ScopeHash
    }
    if ('taskPatchSha256' -in $Binding.PSObject.Properties.Name) { $Binding.taskPatchSha256 = $TaskPatchHash }
    if ('currentDeltaSha256' -in $Binding.PSObject.Properties.Name) { $Binding.currentDeltaSha256 = $CurrentDeltaHash }
    if ('allowedDeltaPaths' -in $Binding.PSObject.Properties.Name) {
        $Binding.allowedDeltaPaths = @($Scope.allowedDeltaPaths)
    }
    if ('excludedDeltaPaths' -in $Binding.PSObject.Properties.Name) {
        $Binding.excludedDeltaPaths = @($Scope.excludedDeltaPaths)
    }
}

function Sync-EmbeddedContract {
    param([string]$Path, [string]$Kind, [object]$Scope, [string]$ScopeHash, [string]$TaskPatchHash, [string]$CurrentDeltaHash)

    $text = Get-Content -LiteralPath $Path -Raw
    $begin = "<!-- BEGIN GOVERNANCE-$Kind-CONTRACT -->"
    $end = "<!-- END GOVERNANCE-$Kind-CONTRACT -->"
    $pattern = [regex]::Escape($begin) + '\s*(?<json>\{.*?\})\s*' + [regex]::Escape($end)
    $match = [regex]::Match($text, $pattern, [System.Text.RegularExpressions.RegexOptions]::Singleline)
    if (-not $match.Success) { throw "Unable to locate $Kind fixture contract." }
    $contract = $match.Groups['json'].Value | ConvertFrom-Json -Depth 50
    Sync-BindingObject -Binding $contract -Scope $Scope -ScopeHash $ScopeHash -TaskPatchHash $TaskPatchHash -CurrentDeltaHash $CurrentDeltaHash
    $replacement = "$begin`n$($contract | ConvertTo-Json -Depth 50)`n$end"
    $updated = $text.Substring(0, $match.Index) + $replacement + $text.Substring($match.Index + $match.Length)
    Write-Utf8 -Path $Path -Text $updated
}

function Repair-SemanticPackageSignatures {
    param([Parameter(Mandatory)][string]$Directory)

    $scopePath = Join-Path $Directory 'scope-inventory.json'
    $scope = Get-Content -LiteralPath $scopePath -Raw | ConvertFrom-Json -Depth 50
    $scopeHash = Get-LowerHash -Path $scopePath
    $taskPatchHash = Get-LowerHash -Path (Join-Path $Directory 'task.patch')
    $currentDeltaHash = Get-LowerHash -Path (Join-Path $Directory 'current-delta.patch')
    foreach ($name in @('assignment-record.json', 'completion-report.json', 'independent-review-evidence.json')) {
        $path = Join-Path $Directory $name
        $binding = Get-Content -LiteralPath $path -Raw | ConvertFrom-Json -Depth 50
        Sync-BindingObject -Binding $binding -Scope $scope -ScopeHash $scopeHash -TaskPatchHash $taskPatchHash -CurrentDeltaHash $currentDeltaHash
        if ($name -ceq 'independent-review-evidence.json') {
            foreach ($reviewed in @($binding.reviewedArtifacts)) {
                $reviewed.sha256 = if ([string]$reviewed.path -ceq 'task.patch') { $taskPatchHash } else { $currentDeltaHash }
            }
        }
        Write-Utf8 -Path $path -Text ($binding | ConvertTo-Json -Depth 50)
    }
    Sync-EmbeddedContract -Path (Join-Path $Directory 'HANDOFF.md') -Kind 'HANDOFF' -Scope $scope -ScopeHash $scopeHash -TaskPatchHash $taskPatchHash -CurrentDeltaHash $currentDeltaHash
    Sync-EmbeddedContract -Path (Join-Path $Directory 'report.md') -Kind 'REPORT' -Scope $scope -ScopeHash $scopeHash -TaskPatchHash $taskPatchHash -CurrentDeltaHash $currentDeltaHash

    $inventoryPath = Join-Path $Directory 'package-inventory.json'
    $inventory = Get-Content -LiteralPath $inventoryPath -Raw | ConvertFrom-Json -Depth 30
    $inventory.entries = @(
        Get-ChildItem -LiteralPath $Directory -File |
            Where-Object Name -NotIn @('package-inventory.json', 'MANIFEST.sha256') |
            Sort-Object Name |
            ForEach-Object {
                [ordered]@{
                    path = $_.Name
                    sha256 = Get-LowerHash -Path $_.FullName
                    length = [int64]$_.Length
                }
            }
    )
    Write-Utf8 -Path $inventoryPath -Text ($inventory | ConvertTo-Json -Depth 30)

    $manifestLines = @(
        Get-ChildItem -LiteralPath $Directory -File |
            Where-Object Name -CNE 'MANIFEST.sha256' |
            Sort-Object Name |
            ForEach-Object {
                "$(Get-LowerHash -Path $_.FullName)  $($_.Length)  $($_.Name)"
            }
    )
    Write-Utf8 -Path (Join-Path $Directory 'MANIFEST.sha256') -Text (($manifestLines -join "`n") + "`n")
}

function Invoke-TemporaryGit {
    param([Parameter(Mandatory)][string]$Root, [Parameter(Mandatory)][string[]]$Argument)
    $output = @(& git -C $Root @Argument 2>&1)
    if ($LASTEXITCODE -ne 0) { throw "Temporary fixture Git command failed: git $($Argument -join ' ') | $($output -join ' | ')" }
    return @($output)
}

function New-StateFixtureRepository {
    param(
        [Parameter(Mandatory)][string]$Name,
        [Parameter(Mandatory)][ValidateSet('MODIFIED','DELETED','RENAMED','RENAMED_MODIFIED','UNTRACKED')][string]$State,
        [switch]$Executable
    )
    $root = Join-Path $temporaryRoot ('repo-' + $Name)
    [void][System.IO.Directory]::CreateDirectory($root)
    $null = Invoke-TemporaryGit -Root $root -Argument @('init','-b','codex/fixture')
    $null = Invoke-TemporaryGit -Root $root -Argument @('remote','add','origin','https://github.com/thomasweidner/flashgate-mcp.git')
    Write-Utf8 -Path (Join-Path $root 'README.md') -Text "fixture baseline`n"
    switch ($State) {
        'MODIFIED' {
            Write-Utf8 -Path (Join-Path $root 'BACKLOG.md') -Text "baseline`n"
            Write-Utf8 -Path (Join-Path $root 'EXCLUDED.md') -Text "excluded baseline`n"
        }
        'DELETED' { Write-Utf8 -Path (Join-Path $root 'deleted.txt') -Text "deleted baseline bytes`n" }
        { $_ -in @('RENAMED','RENAMED_MODIFIED') } {
            Write-Utf8 -Path (Join-Path $root 'previous.txt') -Text ((1..40 | ForEach-Object { "stable rename line $_" }) -join "`n")
        }
    }
    $null = Invoke-TemporaryGit -Root $root -Argument @('add','--all')
    $null = Invoke-TemporaryGit -Root $root -Argument @('-c','user.name=FlashGate Fixture','-c','user.email=fixture@example.invalid','commit','-m','fixture baseline')
    switch ($State) {
        'MODIFIED' {
            Write-Utf8 -Path (Join-Path $root 'BACKLOG.md') -Text "baseline`nmodified`n"
            Write-Utf8 -Path (Join-Path $root 'EXCLUDED.md') -Text "excluded baseline`nmodified`n"
            $include = @('BACKLOG.md')
        }
        'DELETED' { Remove-Item -LiteralPath (Join-Path $root 'deleted.txt'); $include=@('deleted.txt') }
        'RENAMED' { Move-Item -LiteralPath (Join-Path $root 'previous.txt') -Destination (Join-Path $root 'current.txt'); $include=@('previous.txt','current.txt') }
        'RENAMED_MODIFIED' {
            Move-Item -LiteralPath (Join-Path $root 'previous.txt') -Destination (Join-Path $root 'current.txt')
            Add-Content -LiteralPath (Join-Path $root 'current.txt') -Value "changed after rename"
            $include=@('previous.txt','current.txt')
        }
        'UNTRACKED' {
            Write-Utf8 -Path (Join-Path $root 'untracked.sh') -Text "#!/bin/sh`nprintf fixture`n"
            if ($Executable -and -not $IsWindows) {
                [System.IO.File]::SetUnixFileMode((Join-Path $root 'untracked.sh'),
                    [System.IO.UnixFileMode]::UserRead -bor [System.IO.UnixFileMode]::UserWrite -bor
                    [System.IO.UnixFileMode]::UserExecute -bor [System.IO.UnixFileMode]::GroupRead -bor
                    [System.IO.UnixFileMode]::GroupExecute -bor [System.IO.UnixFileMode]::OtherRead -bor
                    [System.IO.UnixFileMode]::OtherExecute)
            }
            $include=@('untracked.sh')
        }
    }
    return [pscustomobject]@{ Root=$root; IncludedPath=$include }
}

function New-StatePackage {
    param([Parameter(Mandatory)][string]$Name, [Parameter(Mandatory)][object]$Fixture)
    $source=Join-Path $temporaryRoot ('source-' + $Name)
    New-GenericSource -Path $source -FixtureRepositoryRoot $Fixture.Root -IncludedPath $Fixture.IncludedPath
    $zip=Join-Path $temporaryRoot ($Name + '.zip')
    Invoke-Generator -Source $source -Package $zip
    return $zip
}

try {
    $resolvedRepositoryRoot = [System.IO.Path]::GetFullPath($RepositoryRoot)
    $authoritativeRoot = $resolvedRepositoryRoot
    $resolvedValidatorPath = [System.IO.Path]::GetFullPath($ValidatorPath)
    if (-not (Test-Path -LiteralPath $resolvedValidatorPath -PathType Leaf)) { throw 'Generic validator not found.' }
    $temporaryRoot = Join-Path ([System.IO.Path]::GetTempPath()) ('flashgate-generic-fixtures-' + [guid]::NewGuid().ToString('N'))
    [void][System.IO.Directory]::CreateDirectory($temporaryRoot)

    $baseFixture = New-StateFixtureRepository -Name 'base-modified' -State MODIFIED
    $authoritativeRoot = $baseFixture.Root

    Invoke-SyntheticHostPathFactoryCase

    $baseSource = Join-Path $temporaryRoot 'source-base'
    New-GenericSource -Path $baseSource -FixtureRepositoryRoot $baseFixture.Root -IncludedPath $baseFixture.IncludedPath
    $baseZip = Join-Path $temporaryRoot 'positive-finding-free.zip'
    Invoke-Generator -Source $baseSource -Package $baseZip
    Invoke-ValidationCase -Name 'positive-finding-free-bl230' -Package $baseZip -ExpectedExit 0

    $findingSource = Join-Path $temporaryRoot 'source-finding'
    New-GenericSource -Path $findingSource -TaskId 'BL-336' -FindingIds @('BL336-REAL-001')
    $findingZip = Join-Path $temporaryRoot 'positive-real-finding.zip'
    Invoke-Generator -Source $findingSource -Package $findingZip -TaskId 'BL-336'
    Invoke-ValidationCase -Name 'positive-real-finding' -Package $findingZip -ExpectedExit 0

    $historicalTextSource = Join-Path $temporaryRoot 'source-historical-text'
    $historicalText = 'Historical evidence names BL333-BL334-REV-013, correction-only.patch, and finding-correction-matrix.json without declaring correction-profile data.'
    New-GenericSource -Path $historicalTextSource -ReportNarrative $historicalText
    $historicalTextZip = Join-Path $temporaryRoot 'positive-historical-text-mentions.zip'
    Invoke-Generator -Source $historicalTextSource -Package $historicalTextZip
    Invoke-ValidationCase -Name 'positive-historical-correction-text-mentions' -Package $historicalTextZip -ExpectedExit 0

    $allowedPathCases = @(
        [pscustomobject]@{ Name = 'positive-canonical-windows-path'; Path = 'C:\Voxtronic\Codex-Work\Reports'; Classification = 'CANONICAL_INFRASTRUCTURE' },
        [pscustomobject]@{ Name = 'positive-canonical-unix-path'; Path = '/opt/flashgate/governance'; Classification = 'CANONICAL_INFRASTRUCTURE' },
        [pscustomobject]@{ Name = 'positive-synthetic-fixture-path'; Path = 'X:\synthetic\fixture.txt'; Classification = 'SYNTHETIC_FIXTURE' },
        [pscustomobject]@{ Name = 'positive-documented-example-path'; Path = 'D:\examples\governance.md'; Classification = 'DOCUMENTED_EXAMPLE' }
    )
    foreach ($allowedCase in $allowedPathCases) {
        $source = Join-Path $temporaryRoot ('source-' + $allowedCase.Name)
        $reference = [ordered]@{
            artifact = 'report.md'; path = $allowedCase.Path
            classification = $allowedCase.Classification
            reason = 'Explicitly classified positive fixture reference.'
        }
        New-GenericSource -Path $source -ReportNarrative ("Documented path: " + $allowedCase.Path) -AllowedHostReferences @($reference)
        $zip = Join-Path $temporaryRoot ($allowedCase.Name + '.zip')
        Invoke-Generator -Source $source -Package $zip
        Invoke-ValidationCase -Name $allowedCase.Name -Package $zip -ExpectedExit 0
    }

    $deleteFixture = New-StateFixtureRepository -Name 'delete' -State DELETED
    $deleteZip = New-StatePackage -Name 'positive-tracked-deletion' -Fixture $deleteFixture
    Invoke-ValidationCase -Name 'positive-tracked-deletion' -Package $deleteZip -ExpectedExit 0 -CaseAuthoritativeRoot $deleteFixture.Root

    $renameFixture = New-StateFixtureRepository -Name 'rename-unchanged' -State RENAMED
    $renameZip = New-StatePackage -Name 'positive-tracked-rename-unchanged' -Fixture $renameFixture
    Invoke-ValidationCase -Name 'positive-tracked-rename-unchanged' -Package $renameZip -ExpectedExit 0 -CaseAuthoritativeRoot $renameFixture.Root

    $renameModifiedFixture = New-StateFixtureRepository -Name 'rename-modified' -State RENAMED_MODIFIED
    $renameModifiedZip = New-StatePackage -Name 'positive-tracked-rename-modified' -Fixture $renameModifiedFixture
    Invoke-ValidationCase -Name 'positive-tracked-rename-modified' -Package $renameModifiedZip -ExpectedExit 0 -CaseAuthoritativeRoot $renameModifiedFixture.Root

    $untrackedFixture = New-StateFixtureRepository -Name 'untracked-regular' -State UNTRACKED
    $untrackedZip = New-StatePackage -Name 'positive-untracked-nonexecutable' -Fixture $untrackedFixture
    Invoke-ValidationCase -Name 'positive-untracked-nonexecutable' -Package $untrackedZip -ExpectedExit 0 -CaseAuthoritativeRoot $untrackedFixture.Root

    if ($IsWindows) {
        Invoke-ValidationCase -Name 'positive-untracked-windows-normalization' -Package $untrackedZip -ExpectedExit 0 -CaseAuthoritativeRoot $untrackedFixture.Root
        if (Test-CaseSelected -Name 'positive-untracked-unix-executable') {
            [void]$results.Add([pscustomobject]@{name='positive-untracked-unix-executable';result='PASS';expectedExit=0;actualExit=0;expectedFailedCheckId='';evidence='Platform-gated: the executable-bit end-to-end package runs on Unix/Linux; Windows rejects the Unix classification.'})
        }
    }
    else {
        $unixExecutableFixture = New-StateFixtureRepository -Name 'untracked-executable' -State UNTRACKED -Executable
        $unixExecutableZip = New-StatePackage -Name 'positive-untracked-unix-executable' -Fixture $unixExecutableFixture
        Invoke-ValidationCase -Name 'positive-untracked-unix-executable' -Package $unixExecutableZip -ExpectedExit 0 -CaseAuthoritativeRoot $unixExecutableFixture.Root
        if (Test-CaseSelected -Name 'positive-untracked-windows-normalization') {
            [void]$results.Add([pscustomobject]@{name='positive-untracked-windows-normalization';result='PASS';expectedExit=0;actualExit=0;expectedFailedCheckId='';evidence='Platform-gated: the Windows normalization end-to-end package runs on Windows; Unix rejects the Windows classification.'})
        }
    }
    $authoritativeRoot = $baseFixture.Root

    $legacySourcePath = Join-Path $temporaryRoot 'legacy-source.json'
    $legacyOutputPath = Join-Path $temporaryRoot 'legacy-HANDOFF.md'
    $legacy = [ordered]@{
        schemaVersion=1; taskId='BL-333/BL-334'; correctionMode='BUNDLED_CORRECTION'
        status='FOURTH_BUNDLED_CORRECTION_COMPLETE_AWAITING_FOCUSED_DELTA_REVIEW'
        classicReviewReady=$true; targetFindings=@('BL333-BL334-REV-013','BL333-BL334-REV-015')
        pendingFindings=@('BL333-BL334-REV-013','BL333-BL334-REV-015')
        closedFindings=@('BL333-BL334-REV-007','BL333-BL334-REV-008','BL333-BL334-REV-010')
        run007Status='CORRECTED_PENDING_DELTA'; commitPreparationApproved=$false
        commitAuthorized=$false; requiredReviewMode='FOCUSED_INDEPENDENT_DELTA_REVIEW'
        targetFindingCount=2; correctedFindingCount=2; pendingDeltaFindingCount=2
        closedFindingCount=3; openFindingCount=0
        nextAction='Perform only the focused independent delta review of BL333-BL334-REV-013 and BL333-BL334-REV-015 with the new verified review package.'
    }
    Write-Utf8 -Path $legacySourcePath -Text ($legacy | ConvertTo-Json -Depth 20)
    $pwsh = (Get-Command pwsh -ErrorAction Stop).Source
    $null = & $pwsh -NoLogo -NoProfile -File (Join-Path $PSScriptRoot 'New-GovernanceHandoff.ps1') -OutputPath $legacyOutputPath -StatusSourcePath $legacySourcePath
    if (Test-CaseSelected -Name 'positive-legacy-correction-generator') {
        [void]$results.Add([pscustomobject]@{ name='positive-legacy-correction-generator'; result=if($LASTEXITCODE -eq 0){'PASS'}else{'FAIL'}; expectedExit=0; actualExit=$LASTEXITCODE; evidence='' })
    }

    Invoke-ValidationCase -Name 'positive-external-independent-review' -Package $baseZip -ExpectedExit 0
    $falseSource = Join-Path $temporaryRoot 'source-not-ready'
    New-GenericSource -Path $falseSource -ClassicReviewReady $false
    $falseZip = Join-Path $temporaryRoot 'positive-not-classic-ready.zip'
    Invoke-Generator -Source $falseSource -Package $falseZip
    Invoke-ValidationCase -Name 'positive-classic-readiness-false' -Package $falseZip -ExpectedExit 0
    Invoke-ValidationCase -Name 'positive-classic-ready-commit-unauthorized' -Package $baseZip -ExpectedExit 0

    $authoritativeRoot = $baseFixture.Root
    $negativeCases = @(
        @{ Name='negative-missing-profile'; Mutation={param($d) $j=Get-Content (Join-Path $d 'assignment-record.json') -Raw|ConvertFrom-Json; $j.PSObject.Properties.Remove('profile'); Write-Utf8 (Join-Path $d 'assignment-record.json') ($j|ConvertTo-Json -Depth 20)}},
        @{ Name='negative-unknown-profile'; Mutation={param($d) $j=Get-Content (Join-Path $d 'assignment-record.json') -Raw|ConvertFrom-Json; $j.profile='UNKNOWN'; Write-Utf8 (Join-Path $d 'assignment-record.json') ($j|ConvertTo-Json -Depth 20)}},
        @{ Name='negative-mixed-profile-artifact'; Mutation={param($d) Write-Utf8 (Join-Path $d 'correction-only.patch') 'forbidden'}},
        @{ Name='negative-fabricated-correction-matrix'; Mutation={param($d) Write-Utf8 (Join-Path $d 'finding-correction-matrix.json') '{}'}},
        @{ Name='negative-missing-patch'; Mutation={param($d) Remove-Item -LiteralPath (Join-Path $d 'task.patch')}},
        @{ Name='negative-extra-delta-path'; Mutation={param($d) Add-Content -LiteralPath (Join-Path $d 'current-delta.patch') -Value "diff --git a/README.md b/README.md`n--- a/README.md`n+++ b/README.md`n@@ -1 +1 @@`n-a`n+b"}},
        @{ Name='negative-missing-completion-report'; Mutation={param($d) Remove-Item -LiteralPath (Join-Path $d 'completion-report.json')}},
        @{ Name='negative-missing-validation-evidence'; Mutation={param($d) Remove-Item -LiteralPath (Join-Path $d 'validation-summary.json')}},
        @{ Name='negative-missing-manifest'; Mutation={param($d) Remove-Item -LiteralPath (Join-Path $d 'MANIFEST.sha256')}},
        @{ Name='negative-wrong-sha256'; Mutation={param($d) $j=Get-Content (Join-Path $d 'independent-review-evidence.json') -Raw|ConvertFrom-Json; $j.reviewedArtifacts[0].sha256='1'*64; Write-Utf8 (Join-Path $d 'independent-review-evidence.json') ($j|ConvertTo-Json -Depth 20)}},
        @{ Name='negative-absolute-scope-path'; Mutation={param($d) $j=Get-Content (Join-Path $d 'scope-inventory.json') -Raw|ConvertFrom-Json; $j.entries[0].path='C:\unsafe.txt'; Write-Utf8 (Join-Path $d 'scope-inventory.json') ($j|ConvertTo-Json -Depth 20)}},
        @{ Name='negative-review-path-drift'; Mutation={param($d) $j=Get-Content (Join-Path $d 'independent-review-evidence.json') -Raw|ConvertFrom-Json; $j.reviewedArtifacts[0].path='current-delta.patch'; Write-Utf8 (Join-Path $d 'independent-review-evidence.json') ($j|ConvertTo-Json -Depth 20)}},
        @{ Name='negative-classic-ready-without-external-review'; Mutation={param($d) $j=Get-Content (Join-Path $d 'independent-review-evidence.json') -Raw|ConvertFrom-Json; $j.external=$false; Write-Utf8 (Join-Path $d 'independent-review-evidence.json') ($j|ConvertTo-Json -Depth 20)}},
        @{ Name='negative-unauthorized-commit'; Mutation={param($d) $j=Get-Content (Join-Path $d 'completion-report.json') -Raw|ConvertFrom-Json; $j.commitAuthorized=$true; Write-Utf8 (Join-Path $d 'completion-report.json') ($j|ConvertTo-Json -Depth 20)}},
        @{ Name='negative-legacy-under-generic-profile'; Mutation={param($d) $j=Get-Content (Join-Path $d 'completion-report.json') -Raw|ConvertFrom-Json; Add-Member -InputObject $j -NotePropertyName correctionMode -NotePropertyValue BUNDLED_CORRECTION; Write-Utf8 (Join-Path $d 'completion-report.json') ($j|ConvertTo-Json -Depth 20)}},
        @{ Name='negative-fixed-legacy-finding-id'; Mutation={param($d) $j=Get-Content (Join-Path $d 'completion-report.json') -Raw|ConvertFrom-Json; $j.findingIds=@('BL333-BL334-REV-013'); Write-Utf8 (Join-Path $d 'completion-report.json') ($j|ConvertTo-Json -Depth 20)}},
        @{ Name='negative-empty-correction-matrix'; Mutation={param($d) Write-Utf8 (Join-Path $d 'finding-correction-matrix.json') '{}'}},
        @{ Name='negative-unknown-json-property'; Mutation={param($d) $j=Get-Content (Join-Path $d 'completion-report.json') -Raw|ConvertFrom-Json; Add-Member -InputObject $j -NotePropertyName unknown -NotePropertyValue true; Write-Utf8 (Join-Path $d 'completion-report.json') ($j|ConvertTo-Json -Depth 20)}},
        @{ Name='negative-invalid-utf8'; Mutation={param($d) [System.IO.File]::WriteAllBytes((Join-Path $d 'report.md'),[byte[]](0xC3,0x28))}},
        @{ Name='negative-zip-inventory-manifest-divergence'; Mutation={param($d) Write-Utf8 (Join-Path $d 'unexpected.txt') 'unexpected'} },
        @{ Name='negative-actual-finding-regression-matrix'; Mutation={param($d) Write-Utf8 (Join-Path $d 'finding-regression-matrix.json') '{}'} },
        @{ Name='negative-mixed-profile-discriminator'; Mutation={param($d) $j=Get-Content (Join-Path $d 'assignment-record.json') -Raw|ConvertFrom-Json; $j.profile='FINDING_CORRECTION'; Write-Utf8 (Join-Path $d 'assignment-record.json') ($j|ConvertTo-Json -Depth 30)} },
        @{ Name='negative-baseline-mismatch'; Semantic=$true; ExpectedCheck='GENERIC-AUTHORITATIVE-REPOSITORY-IDENTITY'; Mutation={param($d) $j=Get-Content (Join-Path $d 'scope-inventory.json') -Raw|ConvertFrom-Json; $j.baselineCommit='a'*40; Write-Utf8 (Join-Path $d 'scope-inventory.json') ($j|ConvertTo-Json -Depth 30)} },
        @{ Name='negative-current-commit-mismatch'; Semantic=$true; ExpectedCheck='GENERIC-AUTHORITATIVE-REPOSITORY-IDENTITY'; Mutation={param($d) $j=Get-Content (Join-Path $d 'scope-inventory.json') -Raw|ConvertFrom-Json; $j.currentCommit='a'*40; Write-Utf8 (Join-Path $d 'scope-inventory.json') ($j|ConvertTo-Json -Depth 30)} },
        @{ Name='negative-branch-mismatch'; Semantic=$true; ExpectedCheck='GENERIC-AUTHORITATIVE-REPOSITORY-IDENTITY'; Mutation={param($d) $j=Get-Content (Join-Path $d 'scope-inventory.json') -Raw|ConvertFrom-Json; $j.branch='codex/wrong-branch'; Write-Utf8 (Join-Path $d 'scope-inventory.json') ($j|ConvertTo-Json -Depth 30)} },
        @{ Name='negative-repository-mismatch'; Semantic=$true; ExpectedCheck='GENERIC-TRUSTED-REPOSITORY'; Mutation={param($d) $j=Get-Content (Join-Path $d 'scope-inventory.json') -Raw|ConvertFrom-Json; $j.repository='https://github.com/example/other.git'; Write-Utf8 (Join-Path $d 'scope-inventory.json') ($j|ConvertTo-Json -Depth 30)} },
        @{ Name='negative-staged-target-path'; Semantic=$true; ExpectedCheck='GENERIC-STAGED-SCOPE-PROHIBITION'; Mutation={param($d) $j=Get-Content (Join-Path $d 'scope-inventory.json') -Raw|ConvertFrom-Json; $j.entries[0].staged=$true; Write-Utf8 (Join-Path $d 'scope-inventory.json') ($j|ConvertTo-Json -Depth 30)} },
        @{ Name='negative-tracked-status-mismatch'; Semantic=$true; ExpectedCheck='GENERIC-AUTHORITATIVE-SCOPE-BINDING'; Mutation={param($d) $j=Get-Content (Join-Path $d 'scope-inventory.json') -Raw|ConvertFrom-Json; $j.entries[0].gitStatus='TRACKED_MODE_CHANGED'; Write-Utf8 (Join-Path $d 'scope-inventory.json') ($j|ConvertTo-Json -Depth 30)} },
        @{ Name='negative-scope-length-mismatch'; Semantic=$true; ExpectedCheck='GENERIC-AUTHORITATIVE-SCOPE-BINDING'; Mutation={param($d) $j=Get-Content (Join-Path $d 'scope-inventory.json') -Raw|ConvertFrom-Json; $j.entries[0].postimage.length=[int64]$j.entries[0].postimage.length+1; Write-Utf8 (Join-Path $d 'scope-inventory.json') ($j|ConvertTo-Json -Depth 30)} },
        @{ Name='negative-scope-mode-mismatch'; Semantic=$true; ExpectedCheck='GENERIC-AUTHORITATIVE-SCOPE-BINDING'; Mutation={param($d) $j=Get-Content (Join-Path $d 'scope-inventory.json') -Raw|ConvertFrom-Json; $j.entries[0].postimage.mode=if([string]$j.entries[0].postimage.mode -ceq '100755'){'100644'}else{'100755'}; Write-Utf8 (Join-Path $d 'scope-inventory.json') ($j|ConvertTo-Json -Depth 30)} },
        @{ Name='negative-scope-hash-mismatch'; Semantic=$true; ExpectedCheck='GENERIC-AUTHORITATIVE-SCOPE-BINDING'; Mutation={param($d) $j=Get-Content (Join-Path $d 'scope-inventory.json') -Raw|ConvertFrom-Json; $j.entries[0].postimage.sha256='2'*64; Write-Utf8 (Join-Path $d 'scope-inventory.json') ($j|ConvertTo-Json -Depth 30)} },
        @{ Name='negative-inclusion-decision-mismatch'; Mutation={param($d) $j=Get-Content (Join-Path $d 'scope-inventory.json') -Raw|ConvertFrom-Json; $j.entries[0].inclusionDecision='EXCLUDE'; Write-Utf8 (Join-Path $d 'scope-inventory.json') ($j|ConvertTo-Json -Depth 30)} },
        @{ Name='negative-missing-inclusion-reason'; Mutation={param($d) $j=Get-Content (Join-Path $d 'scope-inventory.json') -Raw|ConvertFrom-Json; $j.entries[0].PSObject.Properties.Remove('reason'); Write-Utf8 (Join-Path $d 'scope-inventory.json') ($j|ConvertTo-Json -Depth 30)} }
    )
    foreach ($case in $negativeCases) {
        $isSemantic = $case.ContainsKey('Semantic') -and [bool]$case.Semantic
        $expectedCheck = if ($case.ContainsKey('ExpectedCheck')) { [string]$case.ExpectedCheck } else { '' }
        $zip = New-MutatedZip -Name $case.Name -BaselineZip $baseZip -Mutation $case.Mutation -Resign:$isSemantic
        Invoke-ValidationCase -Name $case.Name -Package $zip -ExpectedExit 1 -ExpectedFailedCheckId $expectedCheck
    }

    $stateNegativeCases = @(
        @{ Name='negative-delete-preimage-hash'; Zip=$deleteZip; Root=$deleteFixture.Root; Check='GENERIC-AUTHORITATIVE-SCOPE-BINDING'; Mutation={param($d) $j=Get-Content (Join-Path $d 'scope-inventory.json') -Raw|ConvertFrom-Json; $j.entries[0].preimage.sha256='3'*64; Write-Utf8 (Join-Path $d 'scope-inventory.json') ($j|ConvertTo-Json -Depth 30)} },
        @{ Name='negative-delete-preimage-length'; Zip=$deleteZip; Root=$deleteFixture.Root; Check='GENERIC-AUTHORITATIVE-SCOPE-BINDING'; Mutation={param($d) $j=Get-Content (Join-Path $d 'scope-inventory.json') -Raw|ConvertFrom-Json; $j.entries[0].preimage.length=[int64]$j.entries[0].preimage.length+1; Write-Utf8 (Join-Path $d 'scope-inventory.json') ($j|ConvertTo-Json -Depth 30)} },
        @{ Name='negative-delete-preimage-mode'; Zip=$deleteZip; Root=$deleteFixture.Root; Check='GENERIC-AUTHORITATIVE-SCOPE-BINDING'; Mutation={param($d) $j=Get-Content (Join-Path $d 'scope-inventory.json') -Raw|ConvertFrom-Json; $j.entries[0].preimage.mode=if([string]$j.entries[0].preimage.mode -ceq '100755'){'100644'}else{'100755'}; Write-Utf8 (Join-Path $d 'scope-inventory.json') ($j|ConvertTo-Json -Depth 30)} },
        @{ Name='negative-rename-source-path'; Zip=$renameZip; Root=$renameFixture.Root; Check='GENERIC-PATCH-SCOPE-PARITY'; Mutation={param($d) $j=Get-Content (Join-Path $d 'scope-inventory.json') -Raw|ConvertFrom-Json; $j.entries[0].previousPath='wrong-source.txt'; Write-Utf8 (Join-Path $d 'scope-inventory.json') ($j|ConvertTo-Json -Depth 30)} },
        @{ Name='negative-rename-target-path'; Zip=$renameZip; Root=$renameFixture.Root; Check='GENERIC-PATCH-SCOPE-PARITY'; Mutation={param($d) $j=Get-Content (Join-Path $d 'scope-inventory.json') -Raw|ConvertFrom-Json; $j.entries[0].path='wrong-target.txt'; Write-Utf8 (Join-Path $d 'scope-inventory.json') ($j|ConvertTo-Json -Depth 30)} },
        @{ Name='negative-rename-paths-swapped'; Zip=$renameZip; Root=$renameFixture.Root; Check='GENERIC-AUTHORITATIVE-SCOPE-BINDING'; Mutation={param($d) $j=Get-Content (Join-Path $d 'scope-inventory.json') -Raw|ConvertFrom-Json; $source=[string]$j.entries[0].previousPath; $j.entries[0].previousPath=[string]$j.entries[0].path; $j.entries[0].path=$source; Write-Utf8 (Join-Path $d 'scope-inventory.json') ($j|ConvertTo-Json -Depth 30)} },
        @{ Name='negative-rename-preimage-hash'; Zip=$renameZip; Root=$renameFixture.Root; Check='GENERIC-AUTHORITATIVE-SCOPE-BINDING'; Mutation={param($d) $j=Get-Content (Join-Path $d 'scope-inventory.json') -Raw|ConvertFrom-Json; $j.entries[0].preimage.sha256='4'*64; Write-Utf8 (Join-Path $d 'scope-inventory.json') ($j|ConvertTo-Json -Depth 30)} },
        @{ Name='negative-rename-postimage-hash'; Zip=$renameZip; Root=$renameFixture.Root; Check='GENERIC-AUTHORITATIVE-SCOPE-BINDING'; Mutation={param($d) $j=Get-Content (Join-Path $d 'scope-inventory.json') -Raw|ConvertFrom-Json; $j.entries[0].postimage.sha256='5'*64; Write-Utf8 (Join-Path $d 'scope-inventory.json') ($j|ConvertTo-Json -Depth 30)} },
        @{ Name='negative-untracked-mode'; Zip=$untrackedZip; Root=$untrackedFixture.Root; Check='GENERIC-AUTHORITATIVE-SCOPE-BINDING'; Mutation={param($d) $j=Get-Content (Join-Path $d 'scope-inventory.json') -Raw|ConvertFrom-Json; $j.entries[0].postimage.mode=if([string]$j.entries[0].postimage.mode -ceq '100755'){'100644'}else{'100755'}; Write-Utf8 (Join-Path $d 'scope-inventory.json') ($j|ConvertTo-Json -Depth 30)} },
        @{ Name='negative-untracked-mode-source'; Zip=$untrackedZip; Root=$untrackedFixture.Root; Check='GENERIC-AUTHORITATIVE-SCOPE-BINDING'; Mutation={param($d) $j=Get-Content (Join-Path $d 'scope-inventory.json') -Raw|ConvertFrom-Json; $j.entries[0].postimage.modeSource=if($IsWindows){'UNIX_EXECUTABLE_BIT_NORMALIZED'}else{'WINDOWS_REGULAR_FILE_NORMALIZED'}; Write-Utf8 (Join-Path $d 'scope-inventory.json') ($j|ConvertTo-Json -Depth 30)} },
        @{ Name='negative-platform-mode-classification'; Zip=$untrackedZip; Root=$untrackedFixture.Root; Check='GENERIC-AUTHORITATIVE-SCOPE-BINDING'; Mutation={param($d) $j=Get-Content (Join-Path $d 'scope-inventory.json') -Raw|ConvertFrom-Json; if($IsWindows){$j.entries[0].postimage.mode='100755';$j.entries[0].postimage.modeSource='UNIX_EXECUTABLE_BIT_NORMALIZED'}else{$j.entries[0].postimage.mode='100644';$j.entries[0].postimage.modeSource='WINDOWS_REGULAR_FILE_NORMALIZED'}; Write-Utf8 (Join-Path $d 'scope-inventory.json') ($j|ConvertTo-Json -Depth 30)} },
        @{ Name='negative-rename-single-side-patch'; Zip=$renameZip; Root=$renameFixture.Root; Check='GENERIC-PATCH-SCOPE-PARITY'; Mutation={param($d) foreach($name in @('task.patch','current-delta.patch')){$p=Join-Path $d $name;$t=[System.IO.File]::ReadAllText($p,[System.Text.UTF8Encoding]::new($false,$true));$t=$t.Replace('rename from previous.txt','rename from wrong-source.txt');Write-Utf8 $p $t}} }
    )
    foreach ($case in $stateNegativeCases) {
        $zip=New-MutatedZip -Name $case.Name -BaselineZip $case.Zip -Mutation $case.Mutation -Resign
        Invoke-ValidationCase -Name $case.Name -Package $zip -ExpectedExit 1 -ExpectedFailedCheckId $case.Check -CaseAuthoritativeRoot $case.Root
    }

    Write-Utf8 -Path (Join-Path $deleteFixture.Root 'deleted.txt') -Text "deleted baseline bytes`n"
    Invoke-ValidationCase -Name 'negative-deleted-path-still-present' -Package $deleteZip -ExpectedExit 1 -ExpectedFailedCheckId 'GENERIC-PATCH-SCOPE-PARITY' -CaseAuthoritativeRoot $deleteFixture.Root

    $excludedPatch = ConvertFrom-GenericStrictUtf8 -Bytes (Invoke-GenericGitBytes -Root $baseFixture.Root -Argument @('diff','--binary','--','EXCLUDED.md')).Bytes
    $extraExcludedMutation = { param($d,$extra) foreach($name in @('task.patch','current-delta.patch')){$p=Join-Path $d $name;$t=[System.IO.File]::ReadAllText($p,[System.Text.UTF8Encoding]::new($false,$true));Write-Utf8 $p ($t+$extra)} }
    $extraExcludedZip=New-MutatedZip -Name 'negative-extra-excluded-path-in-patch' -BaselineZip $baseZip -Mutation $extraExcludedMutation -MutationArgument @($excludedPatch) -Resign
    Invoke-ValidationCase -Name 'negative-extra-excluded-path-in-patch' -Package $extraExcludedZip -ExpectedExit 1 -ExpectedFailedCheckId 'GENERIC-PATCH-SCOPE-PARITY' -CaseAuthoritativeRoot $baseFixture.Root

    $hostPathCases = @(
        [pscustomobject]@{ Name='negative-private-windows-user-path'; PathClass=[SyntheticHostPathClass]::WINDOWS_PRIVATE_USER; Label='Private path' },
        [pscustomobject]@{ Name='negative-private-unc-path'; PathClass=[SyntheticHostPathClass]::WINDOWS_PRIVATE_UNC; Label='Private UNC' },
        [pscustomobject]@{ Name='negative-private-linux-home-path'; PathClass=[SyntheticHostPathClass]::UNIX_PRIVATE_HOME; Label='Private path' },
        [pscustomobject]@{ Name='negative-private-macos-user-path'; PathClass=[SyntheticHostPathClass]::MACOS_PRIVATE_USER; Label='Private path' },
        [pscustomobject]@{ Name='negative-private-unix-temp-path'; PathClass=[SyntheticHostPathClass]::UNIX_PRIVATE_TEMP; Label='Private path' },
        [pscustomobject]@{ Name='negative-undeclared-windows-absolute-host-path'; PathClass=[SyntheticHostPathClass]::UNDECLARED_WINDOWS_ABSOLUTE; Label='Undeclared path' },
        [pscustomobject]@{ Name='negative-undeclared-absolute-host-path'; PathClass=[SyntheticHostPathClass]::UNDECLARED_UNIX_ABSOLUTE; Label='Undeclared path' }
    )
    foreach ($hostPathCase in $hostPathCases) {
        $syntheticHostPath = New-SyntheticHostPath -PathClass $hostPathCase.PathClass
        $label = $hostPathCase.Label
        $mutation = {
            param($directory, $pathLabel, $pathValue)
            $reportPath = Join-Path $directory 'report.md'
            Write-Utf8 -Path $reportPath -Text ((Get-Content -LiteralPath $reportPath -Raw) + "`n${pathLabel}: $pathValue")
        }
        $zip = New-MutatedZip -Name $hostPathCase.Name -BaselineZip $baseZip -Mutation $mutation -MutationArgument @($label, $syntheticHostPath) -Resign
        Invoke-ValidationCase -Name $hostPathCase.Name -Package $zip -ExpectedExit 1 -ExpectedFailedCheckId 'GENERIC-CLASSIFIED-HOST-PATH-POLICY'
    }

    $expectedObservedCount = if (@($CaseName).Count -eq 0) { $expectedFixtureCount } else { @($CaseName | Sort-Object -Unique).Count }
    if ($results.Count -ne $expectedObservedCount) { throw "Expected $expectedObservedCount fixtures, observed $($results.Count)." }
    if (@($results | Where-Object result -ceq 'FAIL').Count -gt 0) { throw 'One or more generic handoff fixtures failed.' }
    $status = 'PASS'
}
catch {
    $failureMessage = $_.Exception.Message
}
finally {
    if ($null -ne $temporaryRoot -and (Test-Path -LiteralPath $temporaryRoot -PathType Container)) {
        Remove-Item -LiteralPath $temporaryRoot -Recurse -Force
    }
    if (-not [string]::IsNullOrWhiteSpace($ResultPath)) {
        $resolvedResultPath = [System.IO.Path]::GetFullPath($ResultPath)
        [void][System.IO.Directory]::CreateDirectory((Split-Path -Parent $resolvedResultPath))
        Write-Utf8 -Path $resolvedResultPath -Text ([ordered]@{ schemaVersion=1; status=$status; fixtureCount=$results.Count; expectedFixtureCount=$expectedFixtureCount; failureMessage=$failureMessage; results=@($results) } | ConvertTo-Json -Depth 20)
    }
    [pscustomobject]@{
        Status = $status; FixtureCount = $results.Count
        PassedCount = @($results | Where-Object result -ceq 'PASS').Count
        FailureCount = @($results | Where-Object result -ceq 'FAIL').Count
        FailureMessage = $failureMessage; ResultPath = $ResultPath
        NextAction = if($status -ceq 'PASS'){'Run the complete governance and documentation gates.'}else{'Correct the failing fixture and rerun the matrix.'}
    } | Format-List
}

if ($status -ceq 'PASS') { exit 0 }
exit 1
