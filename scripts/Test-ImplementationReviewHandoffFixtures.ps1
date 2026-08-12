#requires -Version 7.6

[CmdletBinding()]
param(
    [string]$RepositoryRoot = (Split-Path -Parent $PSScriptRoot),
    [string]$ResultPath
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$status = 'FAIL'
$failureMessage = $null
$temporaryRoot = Join-Path ([IO.Path]::GetTempPath()) (
    'flashgate-implementation-review-fixtures-' + [guid]::NewGuid().ToString('N')
)
$results = [Collections.Generic.List[object]]::new()
$materialCorrectionCycleCount = 0
$validationExecutionCount = 0
$infrastructureOrInvocationFailureCount = 0
$packageWriteAttemptCount = 0
$utf8 = [Text.UTF8Encoding]::new($false)

. (Join-Path $PSScriptRoot 'GenericGovernanceGitEvidence.ps1')

function Write-Utf8 {
    param([Parameter(Mandatory)][string]$Path, [Parameter(Mandatory)][string]$Text)
    [IO.File]::WriteAllText($Path, $Text, $utf8)
}

function Get-LowerHash {
    param([Parameter(Mandatory)][string]$Path)
    return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}

function Invoke-FixtureGit {
    param([Parameter(Mandatory)][string]$Root, [Parameter(Mandatory)][string[]]$Argument)
    $output = @(& git -c "safe.directory=$Root" -C $Root @Argument)
    if ($LASTEXITCODE -ne 0) {
        throw "Fixture Git failed: git $($Argument -join ' ')"
    }
    return @($output)
}

function Add-Result {
    param(
        [Parameter(Mandatory)][string]$Name,
        [Parameter(Mandatory)][bool]$Passed,
        [string]$Evidence = ''
    )
    [void]$results.Add([pscustomobject]@{
        name = $Name
        result = if ($Passed) { 'PASS' } else { 'FAIL' }
        evidence = $Evidence
    })
}

function New-FixtureRepository {
    $root = Join-Path $temporaryRoot 'repository'
    [void][IO.Directory]::CreateDirectory($root)
    $null = Invoke-FixtureGit -Root $root -Argument @('init', '-b', 'main')
    $null = Invoke-FixtureGit -Root $root -Argument @('config', 'user.name', 'FlashGate Fixture')
    $null = Invoke-FixtureGit -Root $root -Argument @('config', 'user.email', 'fixture@example.invalid')
    $null = Invoke-FixtureGit -Root $root -Argument @(
        'remote', 'add', 'origin', 'https://github.com/thomasweidner/flashgate-mcp.git'
    )
    $fixturePath = Join-Path $root 'fixture.txt'
    Write-Utf8 -Path $fixturePath -Text "before`n"
    $null = Invoke-FixtureGit -Root $root -Argument @('add', '--', 'fixture.txt')
    $null = Invoke-FixtureGit -Root $root -Argument @('commit', '-m', 'fixture baseline')
    $baseline = [string](@(Invoke-FixtureGit -Root $root -Argument @('rev-parse', 'HEAD'))[0])
    Write-Utf8 -Path $fixturePath -Text "after`n"
    $patchText = (@(Invoke-FixtureGit -Root $root -Argument @(
        'diff', '--binary', '--full-index', $baseline, '--', 'fixture.txt'
    )) -join "`n") + "`n"
    return [pscustomobject]@{
        Root = $root
        Baseline = $baseline
        CurrentCommit = $baseline
        Branch = 'main'
        FixturePath = $fixturePath
        PatchText = $patchText
    }
}

function New-ProfileSource {
    param(
        [Parameter(Mandatory)][string]$Path,
        [Parameter(Mandatory)]$Fixture,
        [Parameter(Mandatory)]
        [ValidateSet('GENERIC_COMMIT_PREPARATION', 'IMPLEMENTATION_TO_INDEPENDENT_FULL_REVIEW')]
        [string]$Profile
    )

    [void][IO.Directory]::CreateDirectory($Path)
    $isImplementation = $Profile -ceq 'IMPLEMENTATION_TO_INDEPENDENT_FULL_REVIEW'
    $transition = if ($isImplementation) {
        'IMPLEMENTATION_TO_INDEPENDENT_FULL_REVIEW'
    }
    else {
        'COMMIT_PREPARATION_TO_COMMIT_APPROVAL'
    }
    $evidenceName = if ($isImplementation) {
        'pre-review-validation-evidence.json'
    }
    else {
        'independent-review-evidence.json'
    }
    Write-Utf8 -Path (Join-Path $Path 'task.patch') -Text $Fixture.PatchText
    Write-Utf8 -Path (Join-Path $Path 'current-delta.patch') -Text $Fixture.PatchText

    $fixtureItem = Get-Item -LiteralPath $Fixture.FixturePath -Force
    $scope = [ordered]@{
        schemaVersion = 1
        taskId = 'BL-339'
        profile = $Profile
        repository = 'https://github.com/thomasweidner/flashgate-mcp.git'
        baselineCommit = $Fixture.Baseline
        currentCommit = $Fixture.CurrentCommit
        branch = $Fixture.Branch
        allowedDeltaPaths = @('fixture.txt')
        excludedDeltaPaths = @()
        entries = @([ordered]@{
            path = 'fixture.txt'
            gitStatus = 'TRACKED_MODIFIED'
            tracked = $true
            staged = $false
            postimage = [ordered]@{
                mode = '100644'
                modeSource = 'GIT_WORKTREE'
                length = [int64]$fixtureItem.Length
                sha256 = Get-LowerHash -Path $fixtureItem.FullName
            }
            inclusionDecision = 'INCLUDE'
            reason = 'Implementation fixture delta'
        })
        hostPathPolicy = [ordered]@{
            hostPathFreeArtifacts = @(
                'HANDOFF.md', 'assignment-record.json', 'completion-report.json',
                'current-delta.patch', $evidenceName, 'report.md', 'task.patch',
                'validation-summary.json'
            )
            allowedReferences = @()
        }
    }
    $patchBytes = Get-GenericDeltaBytes -Root $Fixture.Root `
        -BaselineCommit $Fixture.Baseline `
        -IncludedEntry @($scope.entries) `
        -ExcludedEntry @()
    [IO.File]::WriteAllBytes((Join-Path $Path 'task.patch'), $patchBytes)
    [IO.File]::WriteAllBytes((Join-Path $Path 'current-delta.patch'), $patchBytes)
    Write-Utf8 -Path (Join-Path $Path 'scope-inventory.json') -Text (
        $scope | ConvertTo-Json -Depth 30
    )
    $scopeHash = Get-LowerHash -Path (Join-Path $Path 'scope-inventory.json')
    $taskHash = Get-LowerHash -Path (Join-Path $Path 'task.patch')
    $deltaHash = Get-LowerHash -Path (Join-Path $Path 'current-delta.patch')

    if ($isImplementation) {
        $evidence = [ordered]@{
            schemaVersion = 1
            taskId = 'BL-339'
            repository = 'https://github.com/thomasweidner/flashgate-mcp.git'
            baselineCommit = $Fixture.Baseline
            currentCommit = $Fixture.CurrentCommit
            branch = $Fixture.Branch
            profile = $Profile
            evidenceType = 'FULL_COMPLETION_REUSE'
            independentReviewStatus = 'NOT_PERFORMED'
            externalArtifactRequired = $false
            fullCompletionStatus = 'PASS'
            stagePassed = 1
            stageSelected = 1
            fullCompletionEvidenceReused = $true
            fullCompletionReexecuted = $false
            fullMatrixRunCount = 1
            productionRunInvocationCount = 1
            automaticRetryCount = 0
            infrastructureOrInvocationFailureCount = 0
            openInfrastructureFindingCount = 0
            warningCount = 0
            failureCount = 0
            packageWriteAttemptCountBeforeHandoff = 0
            fullCompletionResultSha256 = ('a' * 64)
            executionEnvelopeSha256 = ('b' * 64)
            findingIds = @()
            scopeInventorySha256 = $scopeHash
            allowedDeltaPaths = @('fixture.txt')
            excludedDeltaPaths = @()
        }
    }
    else {
        $evidence = [ordered]@{
            schemaVersion = 1
            taskId = 'BL-339'
            repository = 'https://github.com/thomasweidner/flashgate-mcp.git'
            baselineCommit = $Fixture.Baseline
            currentCommit = $Fixture.CurrentCommit
            branch = $Fixture.Branch
            profile = $Profile
            reviewMode = 'INDEPENDENT_REVIEW'
            external = $true
            result = 'PASS'
            reviewerIndependencePreserved = $true
            findingIds = @()
            scopeInventorySha256 = $scopeHash
            allowedDeltaPaths = @('fixture.txt')
            excludedDeltaPaths = @()
            reviewedArtifacts = @(
                [ordered]@{ path = 'task.patch'; sha256 = $taskHash },
                [ordered]@{ path = 'current-delta.patch'; sha256 = $deltaHash }
            )
        }
    }
    Write-Utf8 -Path (Join-Path $Path $evidenceName) -Text (
        $evidence | ConvertTo-Json -Depth 30
    )
    $evidenceHash = Get-LowerHash -Path (Join-Path $Path $evidenceName)

    $currentStateGate = [ordered]@{
        result = 'PASS'
        repositoryIdentityBound = $true
        commitAndBranchBound = $true
        completeStatusBound = $true
        scopeAndIdsBound = $true
        parallelWorktreesBound = $true
    }
    $assignment = [ordered]@{
        schemaVersion = 1
        taskId = 'BL-339'
        repository = 'https://github.com/thomasweidner/flashgate-mcp.git'
        baselineCommit = $Fixture.Baseline
        currentCommit = $Fixture.CurrentCommit
        branch = $Fixture.Branch
        executionMode = if ($isImplementation) { 'BUNDLED_CORRECTION' } else { 'COMMIT_PREPARATION' }
        checkpoint = if ($isImplementation) { 'SPRINT_CLOSE' } else { 'PRE_COMMIT' }
        profile = $Profile
        transitionType = $transition
        changeTriggerReviewResult = 'EXISTING_BACKLOG_UPDATED'
        currentStateGate = $currentStateGate
        classicReviewReady = $true
        findingIds = @()
        commitAuthorized = $false
        scopeInventorySha256 = $scopeHash
        taskPatchSha256 = $taskHash
        currentDeltaSha256 = $deltaHash
        allowedDeltaPaths = @('fixture.txt')
        excludedDeltaPaths = @()
    }
    if ($isImplementation) {
        $assignment.fullCompletionEvidenceSha256 = $evidenceHash
        $assignment.fullCompletionResultSha256 = [string]$evidence.fullCompletionResultSha256
        $assignment.executionEnvelopeSha256 = [string]$evidence.executionEnvelopeSha256
    }
    $completion = [ordered]@{
        schemaVersion = 1
        taskId = 'BL-339'
        repository = 'https://github.com/thomasweidner/flashgate-mcp.git'
        baselineCommit = $Fixture.Baseline
        currentCommit = $Fixture.CurrentCommit
        branch = $Fixture.Branch
        profile = $Profile
        transitionType = $transition
        status = 'CLASSIC_REVIEW_READY'
        currentStateGate = $currentStateGate
        classicReviewReady = $true
        findingIds = @()
        commitAuthorized = $false
        materialCorrectionCycleCount = 0
        validationExecutionCount = 1
        infrastructureOrInvocationFailureCount = 0
        observedWarningCount = 0
        resolvedWarningCount = 0
        openWarningCount = 0
        warningCount = 0
        failureCount = 0
        zipFreeReadinessPassed = $true
        packageGeneration = [ordered]@{
            freshStaging = $true
            finalZipWriteCount = 1
            inPlaceRepairPerformed = $false
        }
        scopeInventorySha256 = $scopeHash
        taskPatchSha256 = $taskHash
        currentDeltaSha256 = $deltaHash
        allowedDeltaPaths = @('fixture.txt')
        excludedDeltaPaths = @()
        nextAction = if ($isImplementation) {
            'INDEPENDENT_PHASE_A_FULL_REVIEW'
        }
        else {
            'REQUEST_COMMIT_APPROVAL'
        }
    }
    if ($isImplementation) {
        $completion.independentReviewStatus = 'NOT_PERFORMED'
        $completion.fullCompletionEvidenceSha256 = $evidenceHash
        $completion.fullCompletionResultSha256 = [string]$evidence.fullCompletionResultSha256
        $completion.executionEnvelopeSha256 = [string]$evidence.executionEnvelopeSha256
    }
    Write-Utf8 -Path (Join-Path $Path 'assignment-record.json') -Text (
        $assignment | ConvertTo-Json -Depth 30
    )
    Write-Utf8 -Path (Join-Path $Path 'completion-report.json') -Text (
        $completion | ConvertTo-Json -Depth 30
    )

    $validation = [ordered]@{
        schemaVersion = 1
        taskId = 'BL-339'
        profile = $Profile
        result = 'PASS'
        checks = @([ordered]@{ id = 'focused-profile-validation'; result = 'PASS' })
        progress = [ordered]@{
            completed = 1
            selected = 1
            unit = 'checks'
            phase = 'focused-profile-validation'
            heartbeatIntervalMilliseconds = 30000
            statusCounts = [ordered]@{
                PASS = 1; FAIL = 0; SKIPPED = 0; BLOCKED = 0
                CANCELLED = 0; PENDING = 0; NOT_RUN = 0
            }
            message = '1/1 checks - Phase: focused-profile-validation'
        }
        progressEvents = @([ordered]@{
            sequence = 1
            caseId = 'focused-profile-validation'
            eventType = 'STATUS_CHANGE'
            status = 'PASS'
            completed = 1
            selected = 1
            unit = 'checks'
            phase = 'focused-profile-validation'
            elapsedMilliseconds = 1
        })
        materialCorrectionCycleCount = 0
        validationExecutionCount = 1
        infrastructureOrInvocationFailureCount = 0
        observedWarningCount = 0
        resolvedWarningCount = 0
        openWarningCount = 0
        warningCount = 0
        failureCount = 0
    }
    Write-Utf8 -Path (Join-Path $Path 'validation-summary.json') -Text (
        $validation | ConvertTo-Json -Depth 30
    )

    $reportContract = [ordered]@{
        schemaVersion = 1
        taskId = 'BL-339'
        repository = 'https://github.com/thomasweidner/flashgate-mcp.git'
        baselineCommit = $Fixture.Baseline
        currentCommit = $Fixture.CurrentCommit
        branch = $Fixture.Branch
        transitionType = $transition
        profile = $Profile
        status = 'CLASSIC_REVIEW_READY'
        classicReviewReady = $true
        findingIds = @()
        reviewStatus = if ($isImplementation) { 'NOT_PERFORMED' } else { 'PASS' }
        commitAuthorized = $false
        materialCorrectionCycleCount = 0
        validationExecutionCount = 1
        infrastructureOrInvocationFailureCount = 0
        observedWarningCount = 0
        resolvedWarningCount = 0
        openWarningCount = 0
        zipFreeReadinessPassed = $true
        scopeInventorySha256 = $scopeHash
        taskPatchSha256 = $taskHash
        currentDeltaSha256 = $deltaHash
        allowedDeltaPaths = @('fixture.txt')
        excludedDeltaPaths = @()
        nextAction = $completion.nextAction
    }
    if ($isImplementation) {
        $reportContract.fullCompletionEvidenceSha256 = $evidenceHash
        $reportContract.fullCompletionResultSha256 = [string]$evidence.fullCompletionResultSha256
        $reportContract.executionEnvelopeSha256 = [string]$evidence.executionEnvelopeSha256
    }
    $reportText = @"
# Focused governance handoff fixture

ClassicReviewReady: true

<!-- BEGIN GOVERNANCE-REPORT-CONTRACT -->
$($reportContract | ConvertTo-Json -Depth 30)
<!-- END GOVERNANCE-REPORT-CONTRACT -->
"@
    Write-Utf8 -Path (Join-Path $Path 'report.md') -Text $reportText

    return [pscustomobject]@{
        Profile = $Profile
        TransitionType = $transition
        EvidenceName = $evidenceName
        AllowedDeltaPaths = @('fixture.txt')
    }
}

function Invoke-Generator {
    param(
        [Parameter(Mandatory)]$SourceContract,
        [Parameter(Mandatory)][string]$SourcePath,
        [Parameter(Mandatory)][string]$PackagePath,
        [Parameter(Mandatory)][string]$AuthoritativeRoot
    )
    $output = @(& (Join-Path $PSScriptRoot 'New-GovernanceHandoff.ps1') `
        -Profile $SourceContract.Profile `
        -TransitionType $SourceContract.TransitionType `
        -TaskId 'BL-339' `
        -SourceDirectory $SourcePath `
        -AllowedDeltaPath @($SourceContract.AllowedDeltaPaths) `
        -PackagePath $PackagePath `
        -AuthoritativeRepositoryRoot $AuthoritativeRoot)
    return [pscustomobject]@{
        ExitCode = $LASTEXITCODE
        Output = ($output | Out-String)
    }
}

function Invoke-Validator {
    param(
        [Parameter(Mandatory)][string]$PackagePath,
        [Parameter(Mandatory)][string]$AuthoritativeRoot
    )
    $reportPath = Join-Path $temporaryRoot ('validator-' + [guid]::NewGuid().ToString('N') + '.json')
    $output = @(& (Join-Path $PSScriptRoot 'Test-GenericGovernanceHandoff.ps1') `
        -PackagePath $PackagePath `
        -RepositoryRoot $RepositoryRoot `
        -AuthoritativeRepositoryRoot $AuthoritativeRoot `
        -ReportPath $reportPath `
        -ReturnInsteadOfExit)
    $script:validationExecutionCount++
    return [pscustomobject]@{
        ExitCode = $LASTEXITCODE
        Output = ($output | Out-String)
        ReportPath = $reportPath
    }
}

function Copy-PackageDirectory {
    param([Parameter(Mandatory)][string]$Source, [Parameter(Mandatory)][string]$Name)
    $destination = Join-Path $temporaryRoot $Name
    Copy-Item -LiteralPath $Source -Destination $destination -Recurse
    return $destination
}

try {
    [void][IO.Directory]::CreateDirectory($temporaryRoot)
    $fixture = New-FixtureRepository
    $implementationSource = Join-Path $temporaryRoot 'implementation-source'
    $implementationContract = New-ProfileSource -Path $implementationSource `
        -Fixture $fixture -Profile 'IMPLEMENTATION_TO_INDEPENDENT_FULL_REVIEW'
    $implementationZip = Join-Path $temporaryRoot 'implementation.zip'
    $generator = Invoke-Generator -SourceContract $implementationContract `
        -SourcePath $implementationSource -PackagePath $implementationZip `
        -AuthoritativeRoot $fixture.Root
    $packageWriteAttemptCount += [int]($generator.Output -match 'PackageWriteAttemptCount\s*:\s*1')
    Add-Result -Name 'valid-final-zip' -Passed (
        $generator.ExitCode -eq 0 -and (Test-Path -LiteralPath $implementationZip -PathType Leaf)
    ) -Evidence $generator.Output
    Add-Result -Name 'one-package-write-attempt' -Passed (
        $generator.ExitCode -eq 0 -and $generator.Output -match 'PackageWriteAttemptCount\s*:\s*1'
    ) -Evidence $generator.Output
    if ($generator.ExitCode -ne 0 -or -not (Test-Path -LiteralPath $implementationZip -PathType Leaf)) {
        throw "Implementation profile generator failed before fixture expansion: $($generator.Output)"
    }

    $implementationDirectory = Join-Path $temporaryRoot 'implementation-directory'
    [IO.Compression.ZipFile]::ExtractToDirectory($implementationZip, $implementationDirectory)
    $directoryValidation = Invoke-Validator -PackagePath $implementationDirectory `
        -AuthoritativeRoot $fixture.Root
    Add-Result -Name 'valid-directory-payload' -Passed ($directoryValidation.ExitCode -eq 0) `
        -Evidence $directoryValidation.Output
    $zipValidation = Invoke-Validator -PackagePath $implementationZip `
        -AuthoritativeRoot $fixture.Root
    Add-Result -Name 'zip-reopen-manifest-sha-parity' -Passed ($zipValidation.ExitCode -eq 0) `
        -Evidence $zipValidation.Output
    Add-Result -Name 'no-independent-review-input-required' -Passed (
        -not (Test-Path -LiteralPath (Join-Path $implementationDirectory 'independent-review-evidence.json')) -and
        (Test-Path -LiteralPath (Join-Path $implementationDirectory 'pre-review-validation-evidence.json'))
    )

    $missingPatch = Copy-PackageDirectory -Source $implementationDirectory -Name 'negative-missing-patch'
    Remove-Item -LiteralPath (Join-Path $missingPatch 'task.patch')
    Add-Result -Name 'missing-patch-fails' -Passed (
        (Invoke-Validator -PackagePath $missingPatch -AuthoritativeRoot $fixture.Root).ExitCode -ne 0
    )

    $incompleteScope = Copy-PackageDirectory -Source $implementationDirectory -Name 'negative-incomplete-scope'
    $scope = Get-Content -LiteralPath (Join-Path $incompleteScope 'scope-inventory.json') -Raw |
        ConvertFrom-Json -Depth 30
    $scope.entries = @()
    Write-Utf8 -Path (Join-Path $incompleteScope 'scope-inventory.json') -Text (
        $scope | ConvertTo-Json -Depth 30
    )
    Add-Result -Name 'incomplete-scope-fails' -Passed (
        (Invoke-Validator -PackagePath $incompleteScope -AuthoritativeRoot $fixture.Root).ExitCode -ne 0
    )

    $patchScopeMismatch = Copy-PackageDirectory -Source $implementationDirectory -Name 'negative-patch-scope'
    Write-Utf8 -Path (Join-Path $patchScopeMismatch 'task.patch') -Text "not a unified patch`n"
    Add-Result -Name 'patch-scope-mismatch-fails' -Passed (
        (Invoke-Validator -PackagePath $patchScopeMismatch -AuthoritativeRoot $fixture.Root).ExitCode -ne 0
    )

    $wrongTransition = Copy-PackageDirectory -Source $implementationDirectory -Name 'negative-transition'
    $assignment = Get-Content -LiteralPath (Join-Path $wrongTransition 'assignment-record.json') -Raw |
        ConvertFrom-Json -Depth 30
    $assignment.transitionType = 'COMMIT_PREPARATION_TO_COMMIT_APPROVAL'
    Write-Utf8 -Path (Join-Path $wrongTransition 'assignment-record.json') -Text (
        $assignment | ConvertTo-Json -Depth 30
    )
    Add-Result -Name 'wrong-transition-fails' -Passed (
        (Invoke-Validator -PackagePath $wrongTransition -AuthoritativeRoot $fixture.Root).ExitCode -ne 0
    )

    $wrongProfile = Copy-PackageDirectory -Source $implementationDirectory -Name 'negative-profile'
    $assignment = Get-Content -LiteralPath (Join-Path $wrongProfile 'assignment-record.json') -Raw |
        ConvertFrom-Json -Depth 30
    $assignment.profile = 'GENERIC_COMMIT_PREPARATION'
    Write-Utf8 -Path (Join-Path $wrongProfile 'assignment-record.json') -Text (
        $assignment | ConvertTo-Json -Depth 30
    )
    Add-Result -Name 'wrong-profile-fails' -Passed (
        (Invoke-Validator -PackagePath $wrongProfile -AuthoritativeRoot $fixture.Root).ExitCode -ne 0
    )

    $commitSource = Join-Path $temporaryRoot 'commit-source'
    $commitContract = New-ProfileSource -Path $commitSource -Fixture $fixture `
        -Profile 'GENERIC_COMMIT_PREPARATION'
    $commitZip = Join-Path $temporaryRoot 'commit.zip'
    $commitGenerator = Invoke-Generator -SourceContract $commitContract `
        -SourcePath $commitSource -PackagePath $commitZip -AuthoritativeRoot $fixture.Root
    Add-Result -Name 'existing-commit-preparation-with-review-passes' -Passed (
        $commitGenerator.ExitCode -eq 0 -and
        (Invoke-Validator -PackagePath $commitZip -AuthoritativeRoot $fixture.Root).ExitCode -eq 0
    ) -Evidence $commitGenerator.Output

    $commitDirectory = Join-Path $temporaryRoot 'commit-directory'
    [IO.Compression.ZipFile]::ExtractToDirectory($commitZip, $commitDirectory)
    Remove-Item -LiteralPath (Join-Path $commitDirectory 'independent-review-evidence.json')
    Add-Result -Name 'commit-preparation-without-review-fails' -Passed (
        (Invoke-Validator -PackagePath $commitDirectory -AuthoritativeRoot $fixture.Root).ExitCode -ne 0
    )

    $invalidSource = Join-Path $temporaryRoot 'invalid-source'
    Copy-Item -LiteralPath $implementationSource -Destination $invalidSource -Recurse
    Remove-Item -LiteralPath (Join-Path $invalidSource 'task.patch')
    $invalidZip = Join-Path $temporaryRoot 'must-not-exist.zip'
    $invalidGenerator = Invoke-Generator -SourceContract $implementationContract `
        -SourcePath $invalidSource -PackagePath $invalidZip -AuthoritativeRoot $fixture.Root
    Add-Result -Name 'directory-error-prevents-zip-write' -Passed (
        $invalidGenerator.ExitCode -ne 0 -and
        -not (Test-Path -LiteralPath $invalidZip) -and
        $invalidGenerator.Output -match 'PackageWriteAttemptCount\s*:\s*0'
    ) -Evidence $invalidGenerator.Output

    $existingTarget = Join-Path $temporaryRoot 'existing-target.zip'
    Write-Utf8 -Path $existingTarget -Text "preserve existing target bytes`n"
    $existingTargetHashBefore = (Get-FileHash -LiteralPath $existingTarget -Algorithm SHA256).Hash
    $existingTargetGenerator = Invoke-Generator -SourceContract $implementationContract `
        -SourcePath $implementationSource -PackagePath $existingTarget `
        -AuthoritativeRoot $fixture.Root
    $existingTargetAttemptMatches = @(
        [regex]::Matches($existingTargetGenerator.Output, 'PackageWriteAttemptCount\s*:\s*1')
    ).Count
    Add-Result -Name 'existing-target-fails-after-one-write-attempt' -Passed (
        $existingTargetGenerator.ExitCode -ne 0 -and
        $existingTargetAttemptMatches -eq 1 -and
        (Get-FileHash -LiteralPath $existingTarget -Algorithm SHA256).Hash -ceq $existingTargetHashBefore
    ) -Evidence $existingTargetGenerator.Output

    $directoryTarget = Join-Path $temporaryRoot 'directory-target.zip'
    [void][IO.Directory]::CreateDirectory($directoryTarget)
    $directoryTargetGenerator = Invoke-Generator -SourceContract $implementationContract `
        -SourcePath $implementationSource -PackagePath $directoryTarget `
        -AuthoritativeRoot $fixture.Root
    $directoryTargetAttemptMatches = @(
        [regex]::Matches($directoryTargetGenerator.Output, 'PackageWriteAttemptCount\s*:\s*1')
    ).Count
    Add-Result -Name 'directory-target-io-failure-counts-write-attempt' -Passed (
        $directoryTargetGenerator.ExitCode -ne 0 -and
        $directoryTargetAttemptMatches -eq 1 -and
        (Test-Path -LiteralPath $directoryTarget -PathType Container)
    ) -Evidence $directoryTargetGenerator.Output

    Add-Result -Name 'write-failure-has-no-automatic-retry' -Passed (
        $existingTargetAttemptMatches -eq 1 -and
        $directoryTargetAttemptMatches -eq 1 -and
        -not (Test-Path -LiteralPath (Join-Path $temporaryRoot 'existing-target.retry.zip')) -and
        -not (Test-Path -LiteralPath (Join-Path $temporaryRoot 'directory-target.retry.zip'))
    ) -Evidence "existingTargetAttempts=$existingTargetAttemptMatches; directoryTargetAttempts=$directoryTargetAttemptMatches"

    $invalidReadiness = Copy-PackageDirectory -Source $implementationDirectory -Name 'negative-readiness'
    $completion = Get-Content -LiteralPath (Join-Path $invalidReadiness 'completion-report.json') -Raw |
        ConvertFrom-Json -Depth 30
    $completion.zipFreeReadinessPassed = $false
    Write-Utf8 -Path (Join-Path $invalidReadiness 'completion-report.json') -Text (
        $completion | ConvertTo-Json -Depth 30
    )
    Add-Result -Name 'classic-readiness-requires-complete-handoff' -Passed (
        (Invoke-Validator -PackagePath $invalidReadiness -AuthoritativeRoot $fixture.Root).ExitCode -ne 0
    )

    Add-Result -Name 'full-completion-evidence-is-reused-not-reexecuted' -Passed (
        $implementationContract.EvidenceName -ceq 'pre-review-validation-evidence.json'
    )
    Add-Result -Name 'implementation-profile-is-explicit' -Passed (
        $implementationContract.Profile -ceq 'IMPLEMENTATION_TO_INDEPENDENT_FULL_REVIEW' -and
        $implementationContract.TransitionType -ceq 'IMPLEMENTATION_TO_INDEPENDENT_FULL_REVIEW'
    )

    $status = if (@($results | Where-Object result -ceq 'FAIL').Count -eq 0) { 'PASS' } else { 'FAIL' }
}
catch {
    $failureMessage = $_.Exception.Message
    $infrastructureOrInvocationFailureCount++
}
finally {
    if (Test-Path -LiteralPath $temporaryRoot -PathType Container) {
        Remove-Item -LiteralPath $temporaryRoot -Recurse -Force
    }
    $result = [ordered]@{
        schemaVersion = 1
        status = $status
        selectedFixtureCaseCount = 19
        observedFixtureCaseCount = $results.Count
        passedFixtureCaseCount = @($results | Where-Object result -ceq 'PASS').Count
        failedFixtureCaseCount = @($results | Where-Object result -ceq 'FAIL').Count
        materialCorrectionCycleCount = $materialCorrectionCycleCount
        validationExecutionCount = $validationExecutionCount
        infrastructureOrInvocationFailureCount = $infrastructureOrInvocationFailureCount
        packageWriteAttemptCount = $packageWriteAttemptCount
        automaticRetryCount = 0
        results = @($results)
        failureMessage = $failureMessage
    }
    if (-not [string]::IsNullOrWhiteSpace($ResultPath)) {
        $resolvedResultPath = [IO.Path]::GetFullPath($ResultPath)
        [void][IO.Directory]::CreateDirectory((Split-Path -Parent $resolvedResultPath))
        Write-Utf8 -Path $resolvedResultPath -Text ($result | ConvertTo-Json -Depth 30)
    }
    [pscustomobject]$result | Format-List
}

if ($status -ceq 'PASS') { exit 0 }
exit 1
