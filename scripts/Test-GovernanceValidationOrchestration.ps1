#requires -Version 7.6
[CmdletBinding()]
param(
    [string]$RepositoryRoot = (Split-Path -Parent $PSScriptRoot),
    [string]$ResultPath
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$results = [System.Collections.Generic.List[object]]::new()
$temporaryRoot = $null
$failureMessage = $null
$status = 'FAIL'

function Add-CaseResult {
    param(
        [Parameter(Mandatory)][string]$Name,
        [Parameter(Mandatory)][bool]$Passed,
        [string]$Evidence = ''
    )

    [void]$results.Add([pscustomobject]@{
        Name = $Name
        Result = if ($Passed) { 'PASS' } else { 'FAIL' }
        Evidence = $Evidence
    })
}

function Write-StrictUtf8Text {
    param([string]$Path, [string]$Text)

    [System.IO.File]::WriteAllText($Path, $Text, [System.Text.UTF8Encoding]::new($false))
}

function Invoke-RequiredGit {
    param([string]$Root, [string[]]$Argument)

    $output = @(& git -C $Root @Argument 2>&1)
    if ($LASTEXITCODE -ne 0) {
        throw "Git fixture command failed: git $($Argument -join ' '); $($output -join ' | ')"
    }
    return $output
}

function Get-FileBinding {
    param([string]$Root, [string]$Path)

    return [ordered]@{
        path = $Path
        sha256 = (Get-FileHash -LiteralPath (Join-Path $Root $Path) -Algorithm SHA256).Hash.ToLowerInvariant()
    }
}

function New-PassResult {
    param(
        [string]$TaskId,
        [string]$Profile,
        [string]$Repository,
        [string]$SourceRoot,
        [string]$Commit,
        [string]$Tree,
        [string]$SelectorHash,
        [string]$StatusHash
    )

    $stageResults = @(
        foreach ($id in @('one', 'two', 'three', 'four', 'five', 'six')) {
            [ordered]@{ Id = $id; Phase = 'CHEAP'; Status = 'PASS'; Diagnostics = @() }
        }
    )
    return [ordered]@{
        schemaVersion = 1
        taskId = $TaskId
        profile = $Profile
        status = 'PASS'
        bindings = [ordered]@{
            repository = $Repository
            sourceRepositoryRoot = $SourceRoot
            worktreeRoot = $SourceRoot
            baselineCommit = $Commit
            currentCommit = $Commit
            expectedTree = $Tree
            expectedStatusSha256 = $StatusHash
            actualStatusSha256 = $StatusHash
            protectedWorktrees = @()
            selectorInventorySha256 = $SelectorHash
        }
        stageResults = $stageResults
        evidenceReuse = [ordered]@{ reusedIds = @(); invalidated = @() }
        validationExecutionCount = 1
        infrastructureOrInvocationFailureCount = 0
        fullMatrixRunCount = 0
        packageWriteAttemptCount = 0
        generatedTaskControllerFileCount = 0
        generatedTaskControllerLineCount = 0
        readOnlyProbeCount = 1
        observedWarningCount = 0
        resolvedWarningCount = 0
        openWarningCount = 0
        warningCount = 0
        failureCount = 0
    }
}

try {
    $resolvedRepositoryRoot = [System.IO.Path]::GetFullPath($RepositoryRoot)
    Import-Module (Join-Path $PSScriptRoot 'GovernanceValidationOrchestration.psm1') -Force
    $temporaryRoot = Join-Path ([System.IO.Path]::GetTempPath()) (
        'flashgate-governance-orchestration-' + [guid]::NewGuid().ToString('N')
    )
    [void][System.IO.Directory]::CreateDirectory($temporaryRoot)
    $fixtureRoot = Join-Path $temporaryRoot 'repository'
    [void][System.IO.Directory]::CreateDirectory($fixtureRoot)

    Write-StrictUtf8Text -Path (Join-Path $fixtureRoot '.gitattributes') -Text "* text=auto`n*.ps1 text eol=lf`n"
    Write-StrictUtf8Text -Path (Join-Path $fixtureRoot '.editorconfig') -Text "root = true`n`n[*]`ncharset = utf-8`nend_of_line = lf`ninsert_final_newline = true`ntrim_trailing_whitespace = true`n"
    Write-StrictUtf8Text -Path (Join-Path $fixtureRoot '.gitignore') -Text "ignored.txt`n"
    Write-StrictUtf8Text -Path (Join-Path $fixtureRoot 'valid.ps1') -Text "#requires -Version 7.6`n'fixture' | Out-Null`n"
    Write-StrictUtf8Text -Path (Join-Path $fixtureRoot 'fixture.txt') -Text "fixture`n"
    Write-StrictUtf8Text -Path (Join-Path $fixtureRoot 'ignored.txt') -Text "ignored input`n"

    $null = Invoke-RequiredGit -Root $fixtureRoot -Argument @('init', '-b', 'main')
    $null = Invoke-RequiredGit -Root $fixtureRoot -Argument @(
        'add', '--', '.gitattributes', '.editorconfig', '.gitignore', 'valid.ps1', 'fixture.txt'
    )
    $null = Invoke-RequiredGit -Root $fixtureRoot -Argument @(
        '-c', 'user.name=FlashGate Fixture', '-c', 'user.email=fixture@example.invalid',
        'commit', '-m', 'fixture baseline'
    )
    $commit = [string]@(Invoke-RequiredGit -Root $fixtureRoot -Argument @('rev-parse', 'HEAD'))[0]
    $tree = [string]@(Invoke-RequiredGit -Root $fixtureRoot -Argument @('rev-parse', 'HEAD^{tree}'))[0]
    $protectedRoot = Join-Path $temporaryRoot 'protected-worktree'
    $null = Invoke-RequiredGit -Root $fixtureRoot -Argument @('worktree', 'add', '--detach', $protectedRoot, $commit)
    $primaryStatusHash = [string](Get-GovernanceWorkingTreeStatusBinding -RepositoryRoot $fixtureRoot).Sha256
    $protectedStatusHash = [string](Get-GovernanceWorkingTreeStatusBinding -RepositoryRoot $protectedRoot).Sha256
    $repository = 'https://github.com/thomasweidner/flashgate-mcp.git'
    $selectorHash = 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
    $powerShellPath = [System.IO.Path]::GetFullPath((Get-Process -Id $PID).Path)
    $gitPath = [System.IO.Path]::GetFullPath((Get-Command git -CommandType Application | Select-Object -First 1).Source)
    $resultSchema = Join-Path $resolvedRepositoryRoot 'Governance/governance-validation-result.schema.json'

    $subordinatePath = Join-Path $temporaryRoot 'documentation-result.json'
    $subordinate = New-PassResult `
        -TaskId 'BL-339' `
        -Profile 'documentation-registration' `
        -Repository $repository `
        -SourceRoot $fixtureRoot `
        -Commit $commit `
        -Tree $tree `
        -SelectorHash $selectorHash `
        -StatusHash $primaryStatusHash
    Write-StrictUtf8Text -Path $subordinatePath -Text (($subordinate | ConvertTo-Json -Depth 20) + "`n")

    $evidencePath = Join-Path $temporaryRoot 'prior-pass.json'
    Write-StrictUtf8Text -Path $evidencePath -Text "{`"result`":`"PASS`"}`n"
    $dependencyHash = 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb'
    $scopePaths = @('.editorconfig', '.gitattributes', '.gitignore', 'fixture.txt', 'valid.ps1')
    $request = [ordered]@{
        schemaVersion = 1
        taskId = 'BL-339'
        repository = $repository
        sourceRepositoryRoot = $fixtureRoot
        worktreeRoot = $fixtureRoot
        baselineCommit = $commit
        currentCommit = $commit
        expectedTree = $tree
        expectedBranch = 'main'
        detachedHead = $false
        expectedStatusSha256 = $primaryStatusHash
        scopePaths = $scopePaths
        profile = 'documentation-registration'
        fileHashes = @($scopePaths | ForEach-Object { Get-FileBinding -Root $fixtureRoot -Path $_ })
        protectedWorktrees = @(
            [ordered]@{
                id = 'protected-fixture'
                root = $protectedRoot
                currentCommit = $commit
                expectedTree = $tree
                expectedBranch = ''
                detachedHead = $true
                expectedStatusSha256 = $protectedStatusHash
            }
        )
        externalInputs = @(
            [ordered]@{
                id = 'ignored-agents-equivalent'
                path = Join-Path $fixtureRoot 'ignored.txt'
                purpose = 'Ignored governance input fixture'
                kind = 'IGNORED'
                sha256 = (Get-FileHash -LiteralPath (Join-Path $fixtureRoot 'ignored.txt') -Algorithm SHA256).Hash.ToLowerInvariant()
            }
        )
        toolchain = [ordered]@{
            powerShellPath = $powerShellPath
            powerShellVersion = $PSVersionTable.PSVersion.ToString()
            powerShellSha256 = (Get-FileHash -LiteralPath $powerShellPath -Algorithm SHA256).Hash.ToLowerInvariant()
            gitPath = $gitPath
            gitSha256 = (Get-FileHash -LiteralPath $gitPath -Algorithm SHA256).Hash.ToLowerInvariant()
            executionContext = 'CURRENT_PROCESS'
        }
        selectors = [ordered]@{
            interface = 'scripts/Test-GovernanceConsistencyFixtures.ps1'
            inventorySha256 = $selectorHash
            caseNames = @('positive-bundled-start')
            tags = @('bl-339-phase-a')
            targetPlatform = if ($IsWindows) { 'windows' } else { 'linux' }
            availableCapabilities = @('git', 'powershell-7.6.4')
        }
        currentDependencies = @([ordered]@{ id = 'source'; sha256 = $dependencyHash })
        priorEvidence = @(
            [ordered]@{
                id = 'prior-pass'
                status = 'PASS'
                path = $evidencePath
                sha256 = (Get-FileHash -LiteralPath $evidencePath -Algorithm SHA256).Hash.ToLowerInvariant()
                dependencies = @([ordered]@{ id = 'source'; sha256 = $dependencyHash })
            }
        )
        subordinateResults = @(
            [ordered]@{
                id = 'documentation-consistency'
                path = $subordinatePath
                schemaPath = $resultSchema
                expectedProfile = 'documentation-registration'
                sha256 = (Get-FileHash -LiteralPath $subordinatePath -Algorithm SHA256).Hash.ToLowerInvariant()
            }
        )
    }

    $requestPath = Join-Path $temporaryRoot 'request.json'
    Write-StrictUtf8Text -Path $requestPath -Text (($request | ConvertTo-Json -Depth 30) + "`n")
    $requestSchema = Join-Path $resolvedRepositoryRoot 'Governance/governance-validation-request.schema.json'
    $typedRequest = Read-GovernanceJsonContract -LiteralPath $requestPath -SchemaPath $requestSchema
    $directResult = Invoke-GovernanceValidationOrchestration -Request $typedRequest -RepositoryRoot $resolvedRepositoryRoot
    $expectedCheapOrder = @(
        'parser-syntax', 'text-policy', 'git-diff-check', 'external-input-binding',
        'toolchain-context-binding', 'source-worktree-selector-binding'
    )
    Add-CaseResult -Name 'positive-cheap-order-and-typed-subordinate' -Passed (
        [string]$directResult.status -ceq 'PASS' -and
        ((@($directResult.stageResults | Select-Object -First 6 | ForEach-Object Id) -join "`n") -ceq ($expectedCheapOrder -join "`n")) -and
        [int]$directResult.generatedTaskControllerFileCount -eq 0 -and
        [int]$directResult.generatedTaskControllerLineCount -eq 0
    ) -Evidence ($directResult | ConvertTo-Json -Compress -Depth 10)

    $runnerResultPath = Join-Path $temporaryRoot 'runner-result.json'
    $runnerOutput = @(
        & $powerShellPath -NoLogo -NoProfile -File (Join-Path $PSScriptRoot 'Invoke-GovernanceValidation.ps1') `
            -RequestPath $requestPath `
            -ResultPath $runnerResultPath `
            -RepositoryRoot $resolvedRepositoryRoot
    )
    $runnerExitCode = $LASTEXITCODE
    $runnerResult = Read-GovernanceTypedResult `
        -LiteralPath $runnerResultPath `
        -SchemaPath $resultSchema `
        -ExpectedProfile 'documentation-registration'
    Add-CaseResult -Name 'positive-thin-runner' -Passed (
        $runnerExitCode -eq 0 -and [string]$runnerResult.status -ceq 'PASS'
    ) -Evidence ($runnerOutput -join ' | ')

    $badExternalRequest = $request | ConvertTo-Json -Depth 30 | ConvertFrom-Json -Depth 30
    $badExternalEntry = @($badExternalRequest.externalInputs)[0]
    $badExternalEntry.sha256 = 'cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc'
    $directBadExternal = Test-GovernanceExternalInputBinding `
        -SourceRepositoryRoot $fixtureRoot `
        -ExternalInput @($badExternalRequest.externalInputs)
    $badExternalResult = Invoke-GovernanceValidationOrchestration `
        -Request $badExternalRequest `
        -RepositoryRoot $resolvedRepositoryRoot
    Add-CaseResult -Name 'negative-external-hash-stops-expensive-stages' -Passed (
        [string]$directBadExternal.Status -ceq 'FAIL' -and
        [string]$badExternalResult.status -ceq 'FAIL' -and
        [string](@($badExternalResult.stageResults | Where-Object Id -ceq 'external-input-binding')[0].Status) -ceq 'FAIL' -and
        [string](@($badExternalResult.stageResults | Where-Object Id -ceq 'toolchain-context-binding')[0].Status) -ceq 'NOT_RUN' -and
        @($badExternalResult.stageResults | Where-Object Phase -ceq 'SUBORDINATE').Count -eq 0
    ) -Evidence ([pscustomobject]@{
            RequestedSha256 = [string]$badExternalEntry.sha256
            Direct = $directBadExternal
            Orchestration = $badExternalResult
        } | ConvertTo-Json -Compress -Depth 10)

    $badSourceRequest = $request | ConvertTo-Json -Depth 30 | ConvertFrom-Json -Depth 30
    $badSourceRequest.expectedTree = 'dddddddddddddddddddddddddddddddddddddddd'
    $badSourceResult = Invoke-GovernanceValidationOrchestration `
        -Request $badSourceRequest `
        -RepositoryRoot $resolvedRepositoryRoot
    Add-CaseResult -Name 'negative-source-tree-binding' -Passed (
        [string]$badSourceResult.status -ceq 'FAIL' -and
        [string](@($badSourceResult.stageResults | Where-Object Id -ceq 'source-worktree-selector-binding')[0].Status) -ceq 'FAIL'
    )

    $exactSourceBinding = Test-GovernanceSourceBinding -Request $request
    Add-CaseResult -Name 'positive-exact-complete-status-and-scope-hash-set' -Passed (
        [string]$exactSourceBinding.Status -ceq 'PASS' -and
        [string]$exactSourceBinding.ExpectedStatusSha256 -ceq $primaryStatusHash -and
        [string]$exactSourceBinding.ActualStatusSha256 -ceq $primaryStatusHash -and
        [int]$exactSourceBinding.ScopePathCount -eq [int]$exactSourceBinding.FileHashPathCount
    ) -Evidence ($exactSourceBinding | ConvertTo-Json -Compress -Depth 8)

    Write-StrictUtf8Text -Path (Join-Path $fixtureRoot 'fixture.txt') -Text "modified`n"
    $modifiedTrackedBinding = Test-GovernanceSourceBinding -Request $request
    Add-CaseResult -Name 'negative-complete-status-additional-modified-tracked-path' -Passed (
        [string]$modifiedTrackedBinding.Status -ceq 'FAIL' -and
        @($modifiedTrackedBinding.Diagnostics | Where-Object { $_ -ceq 'Working-tree status hash mismatch.' }).Count -eq 1
    )
    Write-StrictUtf8Text -Path (Join-Path $fixtureRoot 'fixture.txt') -Text "fixture`n"

    Write-StrictUtf8Text -Path (Join-Path $fixtureRoot 'unexpected.txt') -Text "unexpected`n"
    $untrackedBinding = Test-GovernanceSourceBinding -Request $request
    Add-CaseResult -Name 'negative-complete-status-additional-untracked-path' -Passed (
        [string]$untrackedBinding.Status -ceq 'FAIL' -and
        @($untrackedBinding.Diagnostics | Where-Object { $_ -ceq 'Working-tree status hash mismatch.' }).Count -eq 1
    )
    Remove-Item -LiteralPath (Join-Path $fixtureRoot 'unexpected.txt') -Force

    Remove-Item -LiteralPath (Join-Path $fixtureRoot 'valid.ps1') -Force
    $missingPathRequest = $request | ConvertTo-Json -Depth 30 | ConvertFrom-Json -Depth 30
    $missingPathRequest.expectedStatusSha256 = [string](Get-GovernanceWorkingTreeStatusBinding -RepositoryRoot $fixtureRoot).Sha256
    $missingPathBinding = Test-GovernanceSourceBinding -Request $missingPathRequest
    Add-CaseResult -Name 'negative-expected-hash-bound-path-missing' -Passed (
        [string]$missingPathBinding.Status -ceq 'FAIL' -and
        @($missingPathBinding.Diagnostics | Where-Object { $_ -ceq 'Hash-bound source file is missing: valid.ps1' }).Count -eq 1 -and
        @($missingPathBinding.Diagnostics | Where-Object { $_ -ceq 'Working-tree status hash mismatch.' }).Count -eq 0
    )
    Write-StrictUtf8Text -Path (Join-Path $fixtureRoot 'valid.ps1') -Text "#requires -Version 7.6`n'fixture' | Out-Null`n"

    Write-StrictUtf8Text -Path (Join-Path $fixtureRoot 'fixture.txt') -Text "status-code-source`n"
    $statusCodeRequest = $request | ConvertTo-Json -Depth 30 | ConvertFrom-Json -Depth 30
    $statusCodeRequest.expectedStatusSha256 = [string](Get-GovernanceWorkingTreeStatusBinding -RepositoryRoot $fixtureRoot).Sha256
    @($statusCodeRequest.fileHashes | Where-Object { [string]$_.path -ceq 'fixture.txt' })[0].sha256 = `
        (Get-FileHash -LiteralPath (Join-Path $fixtureRoot 'fixture.txt') -Algorithm SHA256).Hash.ToLowerInvariant()
    Remove-Item -LiteralPath (Join-Path $fixtureRoot 'fixture.txt') -Force
    $statusCodeBinding = Test-GovernanceSourceBinding -Request $statusCodeRequest
    Add-CaseResult -Name 'negative-known-path-status-code-changed' -Passed (
        [string]$statusCodeBinding.Status -ceq 'FAIL' -and
        @($statusCodeBinding.Diagnostics | Where-Object { $_ -ceq 'Working-tree status hash mismatch.' }).Count -eq 1
    )
    Write-StrictUtf8Text -Path (Join-Path $fixtureRoot 'fixture.txt') -Text "fixture`n"

    Write-StrictUtf8Text -Path (Join-Path $fixtureRoot 'fixture.txt') -Text "different-content`n"
    $contentHashRequest = $request | ConvertTo-Json -Depth 30 | ConvertFrom-Json -Depth 30
    $contentHashRequest.expectedStatusSha256 = [string](Get-GovernanceWorkingTreeStatusBinding -RepositoryRoot $fixtureRoot).Sha256
    $contentHashBinding = Test-GovernanceSourceBinding -Request $contentHashRequest
    Add-CaseResult -Name 'negative-same-path-different-content-hash' -Passed (
        [string]$contentHashBinding.Status -ceq 'FAIL' -and
        @($contentHashBinding.Diagnostics | Where-Object { $_ -ceq 'Hash-bound source file changed: fixture.txt' }).Count -eq 1 -and
        @($contentHashBinding.Diagnostics | Where-Object { $_ -ceq 'Working-tree status hash mismatch.' }).Count -eq 0
    )
    Write-StrictUtf8Text -Path (Join-Path $fixtureRoot 'fixture.txt') -Text "fixture`n"

    $duplicateScopeRequest = $request | ConvertTo-Json -Depth 30 | ConvertFrom-Json -Depth 30
    $duplicateScopeRequest.scopePaths = @($duplicateScopeRequest.scopePaths) + @('fixture.txt')
    $duplicateScopeBinding = Test-GovernanceSourceBinding -Request $duplicateScopeRequest
    Add-CaseResult -Name 'negative-scope-path-duplicate' -Passed (
        [string]$duplicateScopeBinding.Status -ceq 'FAIL' -and
        @($duplicateScopeBinding.Diagnostics | Where-Object { $_ -ceq 'Duplicate scope path: fixture.txt' }).Count -eq 1
    )

    $duplicateFileHashRequest = $request | ConvertTo-Json -Depth 30 | ConvertFrom-Json -Depth 30
    $duplicateFileHashRequest.fileHashes = @($duplicateFileHashRequest.fileHashes) + @(
        [ordered]@{
            path = 'fixture.txt'
            sha256 = (Get-FileHash -LiteralPath (Join-Path $fixtureRoot 'fixture.txt') -Algorithm SHA256).Hash.ToLowerInvariant()
        }
    )
    $duplicateFileHashBinding = Test-GovernanceSourceBinding -Request $duplicateFileHashRequest
    Add-CaseResult -Name 'negative-file-hash-path-duplicate' -Passed (
        [string]$duplicateFileHashBinding.Status -ceq 'FAIL' -and
        @($duplicateFileHashBinding.Diagnostics | Where-Object { $_ -ceq 'Duplicate file-hash path: fixture.txt' }).Count -eq 1
    )

    $setMismatchRequest = $request | ConvertTo-Json -Depth 30 | ConvertFrom-Json -Depth 30
    $setMismatchRequest.fileHashes = @($setMismatchRequest.fileHashes | Where-Object { [string]$_.path -cne 'valid.ps1' })
    $setMismatchBinding = Test-GovernanceSourceBinding -Request $setMismatchRequest
    Add-CaseResult -Name 'negative-scope-file-hash-set-mismatch' -Passed (
        [string]$setMismatchBinding.Status -ceq 'FAIL' -and
        @($setMismatchBinding.Diagnostics | Where-Object { $_ -ceq 'Scope path has no file-hash binding: valid.ps1' }).Count -eq 1
    )

    $caseCollisionRequest = $request | ConvertTo-Json -Depth 30 | ConvertFrom-Json -Depth 30
    $caseCollisionRequest.scopePaths = @($caseCollisionRequest.scopePaths) + @('Fixture.txt')
    $caseCollisionRequest.fileHashes = @($caseCollisionRequest.fileHashes) + @(
        [ordered]@{
            path = 'Fixture.txt'
            sha256 = (Get-FileHash -LiteralPath (Join-Path $fixtureRoot 'fixture.txt') -Algorithm SHA256).Hash.ToLowerInvariant()
        }
    )
    $caseCollisionBinding = Test-GovernanceSourceBinding -Request $caseCollisionRequest
    Add-CaseResult -Name 'negative-windows-case-collision' -Passed (
        [string]$caseCollisionBinding.Status -ceq 'FAIL' -and
        @($caseCollisionBinding.Diagnostics | Where-Object { $_ -ceq 'Case-colliding scope path: Fixture.txt' }).Count -eq 1 -and
        @($caseCollisionBinding.Diagnostics | Where-Object { $_ -ceq 'Case-colliding file-hash path: Fixture.txt' }).Count -eq 1
    )

    Add-CaseResult -Name 'positive-empty-status-is-explicitly-bound' -Passed (
        $primaryStatusHash -ceq 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855' -and
        [string]$exactSourceBinding.Status -ceq 'PASS'
    )

    Write-StrictUtf8Text -Path (Join-Path $protectedRoot 'foreign-only.txt') -Text "protected foreign delta`n"
    $protectedDeltaRequest = $request | ConvertTo-Json -Depth 30 | ConvertFrom-Json -Depth 30
    @($protectedDeltaRequest.protectedWorktrees)[0].expectedStatusSha256 = `
        [string](Get-GovernanceWorkingTreeStatusBinding -RepositoryRoot $protectedRoot).Sha256
    $protectedDeltaBinding = Test-GovernanceSourceBinding -Request $protectedDeltaRequest
    Add-CaseResult -Name 'positive-separately-bound-protected-worktree-delta' -Passed (
        [string]$protectedDeltaBinding.Status -ceq 'PASS' -and
        [string]$protectedDeltaBinding.ActualStatusSha256 -ceq $primaryStatusHash -and
        [string](@($protectedDeltaBinding.ProtectedWorktrees)[0].Status) -ceq 'PASS'
    ) -Evidence ($protectedDeltaBinding | ConvertTo-Json -Compress -Depth 8)
    Remove-Item -LiteralPath (Join-Path $protectedRoot 'foreign-only.txt') -Force

    $duplicateResultPath = Join-Path $temporaryRoot 'duplicate-result.json'
    Write-StrictUtf8Text -Path $duplicateResultPath -Text "{`"schemaVersion`":1,`"schemaVersion`":1}`n"
    $duplicateRejected = $false
    try {
        $null = Read-GovernanceJsonContract -LiteralPath $duplicateResultPath -SchemaPath $resultSchema
    }
    catch {
        $duplicateRejected = $_.Exception.Message -like 'Duplicate JSON property*'
    }
    Add-CaseResult -Name 'negative-typed-reader-duplicate-property' -Passed $duplicateRejected

    $reuse = Get-GovernanceEvidenceDisposition `
        -PriorEvidence @($request.priorEvidence) `
        -CurrentDependency @($request.currentDependencies)
    $changedDependency = @([ordered]@{ id = 'source'; sha256 = 'eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee' })
    $invalidation = Get-GovernanceEvidenceDisposition `
        -PriorEvidence @($request.priorEvidence) `
        -CurrentDependency $changedDependency
    Add-CaseResult -Name 'positive-evidence-reuse-and-selective-invalidation' -Passed (
        @($reuse.ReusedIds).Count -eq 1 -and
        @($invalidation.Invalidated).Count -eq 1 -and
        [string]$invalidation.Invalidated[0].Reasons[0] -ceq 'DEPENDENCY_CHANGED:source'
    )

    $nonOverlap = Merge-GovernanceOptimisticText `
        -BaseText "A`nB`n" `
        -DesiredText "A-task`nB`n" `
        -CurrentText "A`nB-foreign`n"
    $overlap = Merge-GovernanceOptimisticText `
        -BaseText "A`nB`n" `
        -DesiredText "A-task`nB`n" `
        -CurrentText "A-foreign`nB`n"
    Add-CaseResult -Name 'positive-optimistic-concurrency-non-overlap' -Passed (
        [string]$nonOverlap.Status -ceq 'PASS' -and
        [string]$nonOverlap.Text -ceq "A-task`nB-foreign`n" -and
        [string]$overlap.Status -ceq 'BLOCKED'
    ) -Evidence ([pscustomobject]@{ NonOverlap = $nonOverlap; Overlap = $overlap } | ConvertTo-Json -Compress -Depth 5)

    Write-StrictUtf8Text -Path (Join-Path $fixtureRoot 'fixture.txt') -Text "fixture `n"
    $textFailure = Test-GovernanceTextPolicy -RepositoryRoot $fixtureRoot -RepositoryPath @('fixture.txt')
    Add-CaseResult -Name 'negative-text-trailing-whitespace' -Passed ([string]$textFailure.Status -ceq 'FAIL')
    Write-StrictUtf8Text -Path (Join-Path $fixtureRoot 'fixture.txt') -Text "fixture`n"

    $externalPath = Join-Path $temporaryRoot 'external-register.md'
    $baseExternal = "A`nB`n"
    Write-StrictUtf8Text -Path $externalPath -Text $baseExternal
    $expectedExternalHash = (Get-FileHash -LiteralPath $externalPath -Algorithm SHA256).Hash.ToLowerInvariant()
    Write-StrictUtf8Text -Path $externalPath -Text "A`nB-foreign`n"
    $externalWrite = Set-GovernanceExternalTextOptimistic `
        -LiteralPath $externalPath `
        -ExpectedSha256 $expectedExternalHash `
        -BaseText $baseExternal `
        -DesiredText "A-task`nB`n"
    Add-CaseResult -Name 'positive-external-write-preserves-non-overlap' -Passed (
        [string]$externalWrite.Status -ceq 'PASS' -and
        [bool]$externalWrite.ForeignDeltaPreserved -and
        [System.IO.File]::ReadAllText($externalPath) -ceq "A-task`nB-foreign`n"
    ) -Evidence ($externalWrite | ConvertTo-Json -Compress -Depth 5)

    $status = if (@($results | Where-Object Result -ceq 'FAIL').Count -eq 0) { 'PASS' } else { 'FAIL' }
}
catch {
    $failureMessage = $_.Exception.Message
}
finally {
    if ($null -ne $temporaryRoot -and (Test-Path -LiteralPath $temporaryRoot -PathType Container)) {
        Remove-Item -LiteralPath $temporaryRoot -Recurse -Force
    }
}

$failureCount = @($results | Where-Object Result -ceq 'FAIL').Count + [int](-not [string]::IsNullOrWhiteSpace($failureMessage))
$finalResult = [pscustomobject]@{
    Status = if ($failureCount -eq 0 -and $status -ceq 'PASS') { 'PASS' } else { 'FAIL' }
    CaseCount = $results.Count
    PassedCount = @($results | Where-Object Result -ceq 'PASS').Count
    FailedCount = @($results | Where-Object Result -ceq 'FAIL').Count
    WarningCount = 0
    FailureCount = $failureCount
    Results = @($results)
    FailureMessage = $failureMessage
    NextAction = if ($failureCount -eq 0) {
        'Use the permanent orchestration module and data request schema.'
    }
    else {
        'Correct the failed orchestration contract case and rerun the focused matrix.'
    }
}

if (-not [string]::IsNullOrWhiteSpace($ResultPath)) {
    [System.IO.File]::WriteAllText(
        [System.IO.Path]::GetFullPath($ResultPath),
        (($finalResult | ConvertTo-Json -Depth 20) + "`n"),
        [System.Text.UTF8Encoding]::new($false)
    )
}
$finalResult | Format-List
if ($failureCount -eq 0) { exit 0 }
exit 1
