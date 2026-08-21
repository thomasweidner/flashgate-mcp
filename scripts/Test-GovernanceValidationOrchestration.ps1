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
        throw ('Git fixture command failed: git {0}; {1}' -f [string]::Join(' ', [string[]]$Argument), [string]::Join(' | ', [string[]]$output))
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
            selectionSha256 = $SelectorHash
            resolvedCaseCount = 1
        }
        stageResults = $stageResults
        evidenceReuse = [ordered]@{ reusedIds = @(); invalidated = @() }
        stateTransitionMap = @(
            foreach ($component in @('commit', 'tree', 'working-status', 'scope', 'selector', 'package', 'external-inputs', 'evidence')) {
                [ordered]@{
                    Component = $component
                    PriorSha256 = $SelectorHash
                    CurrentSha256 = $SelectorHash
                    Disposition = 'REUSED'
                    Reason = 'ACTUAL_HASH_BOUND_STATE_UNCHANGED'
                }
            }
        )
        profileResults = [ordered]@{
            IntendedBaseResult = 'NOT_RUN'; MergeBaseResult = 'NOT_RUN'
            EffectivePRScopeResult = 'NOT_RUN'; EffectivePRPatchHash = $null
            IntegrationProjectionResult = 'NOT_RUN'; IntegrationProjectionHash = $null
            AuthorizedWriteSetResult = 'NOT_RUN'; ForeignProtectedStateResult = 'NOT_RUN'
        }
        runnerProcessStartCount = 0
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
    . (Join-Path $PSScriptRoot 'GenericGovernanceGitEvidence.ps1')
    $temporaryRoot = Join-Path ([System.IO.Path]::GetTempPath()) (
        'flashgate-governance-orchestration-' + [guid]::NewGuid().ToString('N')
    )
    [void][System.IO.Directory]::CreateDirectory($temporaryRoot)
    $fixtureRoot = Join-Path $temporaryRoot 'repository'
    [void][System.IO.Directory]::CreateDirectory($fixtureRoot)

    Write-StrictUtf8Text -Path (Join-Path $fixtureRoot '.gitattributes') -Text "* text=auto`n*.ps1 text eol=lf`n"
    Write-StrictUtf8Text -Path (Join-Path $fixtureRoot '.editorconfig') -Text "root = true`n`n[*]`ncharset = utf-8`nend_of_line = lf`ninsert_final_newline = true`ntrim_trailing_whitespace = true`n"
    Write-StrictUtf8Text -Path (Join-Path $fixtureRoot '.gitignore') -Text "ignored.txt`n"
    Write-StrictUtf8Text -Path (Join-Path $fixtureRoot 'fixture.txt') -Text "fixture`n"
    Write-StrictUtf8Text -Path (Join-Path $fixtureRoot 'ignored.txt') -Text "ignored input`n"
    [void][System.IO.Directory]::CreateDirectory((Join-Path $fixtureRoot 'scripts'))
    Write-StrictUtf8Text -Path (Join-Path $fixtureRoot 'scripts/valid.ps1') -Text "#requires -Version 7.6`n'fixture' | Out-Null`n"
    Write-StrictUtf8Text -Path (Join-Path $fixtureRoot 'scripts/permanent-profile.ps1') `
        -Text "#requires -Version 7.6`n'permanent profile' | Out-Null`n"

    $null = Invoke-RequiredGit -Root $fixtureRoot -Argument @('init', '-b', 'main')
    $null = Invoke-RequiredGit -Root $fixtureRoot -Argument @(
        'add', '--', '.gitattributes', '.editorconfig', '.gitignore', 'scripts/valid.ps1', 'fixture.txt',
        'scripts/permanent-profile.ps1'
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
    Import-Module (Join-Path $PSScriptRoot 'GovernanceCaseSelection.psm1') -Force
    $selectorMetadata = Read-GovernanceCaseMetadata `
        -Path (Join-Path $resolvedRepositoryRoot 'Governance/governance-case-metadata.json') `
        -SchemaPath (Join-Path $resolvedRepositoryRoot 'Governance/governance-case-metadata.schema.json')
    $selectorHash = [string]$selectorMetadata.MetadataInventorySHA256
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
    $scopePaths = @('.editorconfig', '.gitattributes', '.gitignore', 'fixture.txt', 'scripts/permanent-profile.ps1', 'scripts/valid.ps1')
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
                sourceRoot = $fixtureRoot
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
            interface = 'scripts/GovernanceCaseSelection.psm1'
            metadataPath = 'Governance/governance-case-metadata.json'
            schemaPath = 'Governance/governance-case-metadata.schema.json'
            inventorySha256 = $selectorHash
            caseNames = @('positive-bundled-start')
            groups = @()
            tags = @()
            targetPlatform = if ($IsWindows) { 'windows' } else { 'linux' }
            availableCapabilities = @('git', $(if ($IsWindows) { 'powershell-7.6.5' } else { 'powershell-7.6.4' }))
        }
        exactCommit = $null
        stateComponents = @(
            foreach ($component in @('commit', 'tree', 'working-status', 'scope', 'selector', 'package', 'external-inputs', 'evidence')) {
                [ordered]@{ component = $component; priorSha256 = $dependencyHash; currentSha256 = $dependencyHash }
            }
        )
        taskControllers = @()
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
    foreach ($stateComponent in @($request.stateComponents)) {
        $actualTransition = @($directResult.stateTransitionMap | Where-Object {
                [string]$_.Component -ceq [string]$stateComponent.component
            })[0]
        $stateComponent.priorSha256 = [string]$actualTransition.CurrentSha256
    }

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

    $implementationHead = [string]@(Invoke-RequiredGit -Root $resolvedRepositoryRoot -Argument @('rev-parse', 'HEAD'))[0]
    $currentRecordPath = Join-Path $temporaryRoot 'current-workflow-record.json'
    $null = @(& (Join-Path $PSScriptRoot 'New-GovernanceWorkflowRecord.ps1') `
            -OutputPath $currentRecordPath `
            -RepositoryRoot $resolvedRepositoryRoot `
            -Checkpoint 'ASSIGNMENT_START' `
            -Repository $repository `
            -BaselineCommit $implementationHead `
            -CurrentCommit $implementationHead `
            -WorkflowCommit $implementationHead `
            -RunId '1' `
            -RunAttempt 1 `
            -Event 'fixture' `
            -Ref 'refs/heads/main' `
            -HeadSha $implementationHead `
            -TaskId 'BL-339')
    $assignmentSchemaPath = Join-Path $resolvedRepositoryRoot 'Governance/assignment-governance-record.schema.json'
    $currentRecord = Get-Content -LiteralPath $currentRecordPath -Raw | ConvertFrom-Json -Depth 100
    $historicalRecord = $currentRecord | ConvertTo-Json -Depth 100 | ConvertFrom-Json -Depth 100
    $historicalRecord.PSObject.Properties.Remove('recordReadinessClass')
    $historicalRecordPath = Join-Path $temporaryRoot 'historical-workflow-record.json'
    Write-StrictUtf8Text -Path $historicalRecordPath -Text (($historicalRecord | ConvertTo-Json -Depth 100) + "`n")
    $historicalSchemaReadable = (Get-Content -LiteralPath $historicalRecordPath -Raw) | Test-Json -SchemaFile $assignmentSchemaPath
    $historicalReportPath = Join-Path $temporaryRoot 'historical-consistency-result.json'
    $null = @(& (Join-Path $PSScriptRoot 'Test-GovernanceConsistency.ps1') `
            -RepositoryRoot $resolvedRepositoryRoot `
            -AssignmentRecordPath $historicalRecordPath `
            -RuntimeCheckpoint 'ASSIGNMENT_START' `
            -ExpectedRepository $repository `
            -ExpectedBaselineCommit $implementationHead `
            -ExpectedCurrentCommit $implementationHead `
            -ReportPath $historicalReportPath)
    $historicalReport = Get-Content -LiteralPath $historicalReportPath -Raw | ConvertFrom-Json -Depth 100
    $historicalClassCheck = @($historicalReport.checks | Where-Object Id -ceq 'RECORD-CURRENT-READINESS-CLASS')[0]
    Add-CaseResult -Name 'historical-v1-readable-without-readiness-class' -Passed (
        $historicalSchemaReadable -and [string]$historicalClassCheck.Result -ceq 'PASS'
    ) -Evidence ($historicalClassCheck | ConvertTo-Json -Compress)

    $historicalReadyRecord = $historicalRecord | ConvertTo-Json -Depth 100 | ConvertFrom-Json -Depth 100
    $historicalReadyRecord.documentationConsistencyResult = 'PASS'
    $historicalReadyPath = Join-Path $temporaryRoot 'historical-current-readiness-claim.json'
    Write-StrictUtf8Text -Path $historicalReadyPath -Text (($historicalReadyRecord | ConvertTo-Json -Depth 100) + "`n")
    $historicalReadyReportPath = Join-Path $temporaryRoot 'historical-current-readiness-result.json'
    $null = @(& (Join-Path $PSScriptRoot 'Test-GovernanceConsistency.ps1') `
            -RepositoryRoot $resolvedRepositoryRoot `
            -AssignmentRecordPath $historicalReadyPath `
            -RuntimeCheckpoint 'ASSIGNMENT_START' `
            -ExpectedRepository $repository `
            -ExpectedBaselineCommit $implementationHead `
            -ExpectedCurrentCommit $implementationHead `
            -ReportPath $historicalReadyReportPath)
    $historicalReadyReport = Get-Content -LiteralPath $historicalReadyReportPath -Raw | ConvertFrom-Json -Depth 100
    $historicalReadyClassCheck = @($historicalReadyReport.checks | Where-Object Id -ceq 'RECORD-CURRENT-READINESS-CLASS')[0]
    Add-CaseResult -Name 'historical-v1-cannot-satisfy-current-readiness' -Passed (
        [string]$historicalReadyClassCheck.Result -ceq 'FAIL'
    ) -Evidence ($historicalReadyClassCheck | ConvertTo-Json -Compress)

    $currentReadyRecord = $currentRecord | ConvertTo-Json -Depth 100 | ConvertFrom-Json -Depth 100
    $currentReadyRecord.documentationConsistencyResult = 'PASS'
    $currentReadyPath = Join-Path $temporaryRoot 'current-readiness-claim.json'
    Write-StrictUtf8Text -Path $currentReadyPath -Text (($currentReadyRecord | ConvertTo-Json -Depth 100) + "`n")
    $currentReadyReportPath = Join-Path $temporaryRoot 'current-readiness-result.json'
    $null = @(& (Join-Path $PSScriptRoot 'Test-GovernanceConsistency.ps1') `
            -RepositoryRoot $resolvedRepositoryRoot `
            -AssignmentRecordPath $currentReadyPath `
            -RuntimeCheckpoint 'ASSIGNMENT_START' `
            -ExpectedRepository $repository `
            -ExpectedBaselineCommit $implementationHead `
            -ExpectedCurrentCommit $implementationHead `
            -ReportPath $currentReadyReportPath)
    $currentReadyReport = Get-Content -LiteralPath $currentReadyReportPath -Raw | ConvertFrom-Json -Depth 100
    $currentReadyClassCheck = @($currentReadyReport.checks | Where-Object Id -ceq 'RECORD-CURRENT-READINESS-CLASS')[0]
    Add-CaseResult -Name 'current-readiness-class-passes-current-consumer' -Passed (
        [string]$currentReadyClassCheck.Result -ceq 'PASS'
    ) -Evidence ($currentReadyClassCheck | ConvertTo-Json -Compress)

    $invalidClassRecord = $currentRecord | ConvertTo-Json -Depth 100 | ConvertFrom-Json -Depth 100
    $invalidClassRecord.recordReadinessClass = 'HISTORICAL'
    $invalidClassText = ($invalidClassRecord | ConvertTo-Json -Depth 100)
    Add-CaseResult -Name 'invalid-readiness-class-fails-schema' -Passed (
        -not ($invalidClassText | Test-Json -SchemaFile $assignmentSchemaPath -ErrorAction SilentlyContinue)
    )

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
        @($badExternalResult.stageResults | Where-Object Phase -ceq 'SUBORDINATE').Count -eq 0 -and
        [int]$badExternalResult.runnerProcessStartCount -eq 0 -and
        [int]$badExternalResult.validationExecutionCount -eq 0
    ) -Evidence ([pscustomobject]@{
            RequestedSha256 = [string]$badExternalEntry.sha256
            Direct = $directBadExternal
            Orchestration = $badExternalResult
        } | ConvertTo-Json -Compress -Depth 10)

    $infoExcludedPath = Join-Path $fixtureRoot 'info-excluded.txt'
    Write-StrictUtf8Text -Path $infoExcludedPath -Text "info excluded`n"
    $gitInfoExcludePath = Join-Path $fixtureRoot '.git/info/exclude'
    $gitInfoExcludeText = [System.IO.File]::ReadAllText($gitInfoExcludePath)
    Write-StrictUtf8Text -Path $gitInfoExcludePath -Text ($gitInfoExcludeText.TrimEnd("`r", "`n") + "`ninfo-excluded.txt`n")
    $infoExcludedBinding = Test-GovernanceExternalInputBinding -SourceRepositoryRoot $fixtureRoot -ExternalInput @(
        [ordered]@{
            id = 'info-excluded'; path = $infoExcludedPath; sourceRoot = $fixtureRoot
            purpose = 'Git info exclude provenance'; kind = 'GIT_EXCLUDED'
            sha256 = (Get-FileHash -LiteralPath $infoExcludedPath -Algorithm SHA256).Hash.ToLowerInvariant()
        }
    )
    Add-CaseResult -Name 'positive-git-info-exclude-actual-provenance' -Passed (
        [string]$infoExcludedBinding.Status -ceq 'PASS' -and
        [string]$infoExcludedBinding.BoundInputs[0].Kind -ceq 'GIT_EXCLUDED'
    ) -Evidence ($infoExcludedBinding | ConvertTo-Json -Compress -Depth 8)

    $coreExcludedPath = Join-Path $fixtureRoot 'core-excluded.txt'
    $coreExcludeFile = Join-Path $temporaryRoot 'global-excludes.txt'
    Write-StrictUtf8Text -Path $coreExcludedPath -Text "core excluded`n"
    Write-StrictUtf8Text -Path $coreExcludeFile -Text "core-excluded.txt`n"
    $savedGitConfigCount = $env:GIT_CONFIG_COUNT
    $savedGitConfigKey = $env:GIT_CONFIG_KEY_0
    $savedGitConfigValue = $env:GIT_CONFIG_VALUE_0
    try {
        $env:GIT_CONFIG_COUNT = '1'
        $env:GIT_CONFIG_KEY_0 = 'core.excludesFile'
        $env:GIT_CONFIG_VALUE_0 = $coreExcludeFile
        $coreExcludedBinding = Test-GovernanceExternalInputBinding -SourceRepositoryRoot $fixtureRoot -ExternalInput @(
            [ordered]@{
                id = 'core-excluded'; path = $coreExcludedPath; sourceRoot = $fixtureRoot
                purpose = 'Resolved core excludes file provenance'; kind = 'GIT_EXCLUDED'
                sha256 = (Get-FileHash -LiteralPath $coreExcludedPath -Algorithm SHA256).Hash.ToLowerInvariant()
            }
        )
    }
    finally {
        $env:GIT_CONFIG_COUNT = $savedGitConfigCount
        $env:GIT_CONFIG_KEY_0 = $savedGitConfigKey
        $env:GIT_CONFIG_VALUE_0 = $savedGitConfigValue
    }
    Add-CaseResult -Name 'positive-core-excludes-file-resolved-path-provenance' -Passed (
        [string]$coreExcludedBinding.Status -ceq 'PASS' -and
        [string]$coreExcludedBinding.BoundInputs[0].Kind -ceq 'GIT_EXCLUDED'
    ) -Evidence ($coreExcludedBinding | ConvertTo-Json -Compress -Depth 8)
    Remove-Item -LiteralPath $coreExcludedPath -Force

    $caseRoot = if ($IsWindows) { $fixtureRoot.ToUpperInvariant() } else { $fixtureRoot + '-CASE-DIFFERENT' }
    $caseRootBinding = Test-GovernanceExternalInputBinding -SourceRepositoryRoot $fixtureRoot -ExternalInput @(
        [ordered]@{
            id = 'platform-case-root'; path = Join-Path $fixtureRoot 'ignored.txt'; sourceRoot = $caseRoot
            purpose = 'Platform path-comparison semantics'; kind = 'IGNORED'
            sha256 = (Get-FileHash -LiteralPath (Join-Path $fixtureRoot 'ignored.txt') -Algorithm SHA256).Hash.ToLowerInvariant()
        }
    )
    Add-CaseResult -Name 'platform-case-sensitive-root-semantics' -Passed (
        ($IsWindows -and [string]$caseRootBinding.Status -ceq 'PASS') -or
        (-not $IsWindows -and [string]$caseRootBinding.Status -ceq 'FAIL')
    ) -Evidence ($caseRootBinding | ConvertTo-Json -Compress -Depth 8)

    $linkTarget = Join-Path $temporaryRoot 'external-link-target'
    [void][System.IO.Directory]::CreateDirectory($linkTarget)
    Write-StrictUtf8Text -Path (Join-Path $linkTarget 'linked-input.txt') -Text "linked input`n"
    $linkRoot = Join-Path $temporaryRoot 'external-link-root'
    $linkType = if ($IsWindows) { 'Junction' } else { 'SymbolicLink' }
    $null = New-Item -ItemType $linkType -Path $linkRoot -Target $linkTarget
    $linkRootRequest = $request | ConvertTo-Json -Depth 40 | ConvertFrom-Json -Depth 40
    $linkRootRequest.externalInputs = @([ordered]@{
            id = 'linked-root'; path = Join-Path $linkRoot 'linked-input.txt'; sourceRoot = $linkRoot
            purpose = 'Reject linked source root'; kind = 'EXTERNAL'
            sha256 = (Get-FileHash -LiteralPath (Join-Path $linkTarget 'linked-input.txt') -Algorithm SHA256).Hash.ToLowerInvariant()
        })
    $linkRootResult = Invoke-GovernanceValidationOrchestration -Request $linkRootRequest -RepositoryRoot $resolvedRepositoryRoot
    Add-CaseResult -Name 'negative-source-root-link-rejected-before-runner' -Passed (
        [string]$linkRootResult.status -ceq 'FAIL' -and
        [int]$linkRootResult.runnerProcessStartCount -eq 0 -and
        [int]$linkRootResult.validationExecutionCount -eq 0
    ) -Evidence ($linkRootResult | ConvertTo-Json -Compress -Depth 8)
    Remove-Item -LiteralPath $linkRoot -Force

    $outsideInputPath = Join-Path $temporaryRoot 'outside-root-input.txt'
    Write-StrictUtf8Text -Path $outsideInputPath -Text "outside`n"
    $outsideInputRequest = $request | ConvertTo-Json -Depth 40 | ConvertFrom-Json -Depth 40
    $outsideInputRequest.externalInputs = @([ordered]@{
            id = 'outside-root'; path = $outsideInputPath; sourceRoot = $fixtureRoot
            purpose = 'Reject source-root escape'; kind = 'EXTERNAL'
            sha256 = (Get-FileHash -LiteralPath $outsideInputPath -Algorithm SHA256).Hash.ToLowerInvariant()
        })
    $outsideInputResult = Invoke-GovernanceValidationOrchestration -Request $outsideInputRequest -RepositoryRoot $resolvedRepositoryRoot
    Add-CaseResult -Name 'negative-source-root-escape-rejected-before-runner' -Passed (
        [string]$outsideInputResult.status -ceq 'FAIL' -and
        [int]$outsideInputResult.runnerProcessStartCount -eq 0 -and
        [int]$outsideInputResult.validationExecutionCount -eq 0
    ) -Evidence ($outsideInputResult | ConvertTo-Json -Compress -Depth 8)

    $wrongClassificationRequest = $request | ConvertTo-Json -Depth 40 | ConvertFrom-Json -Depth 40
    $wrongClassificationRequest.externalInputs[0].kind = 'VERSIONED'
    $wrongClassificationResult = Invoke-GovernanceValidationOrchestration -Request $wrongClassificationRequest -RepositoryRoot $resolvedRepositoryRoot
    Add-CaseResult -Name 'negative-wrong-git-classification-rejected-before-runner' -Passed (
        [string]$wrongClassificationResult.status -ceq 'FAIL' -and
        [int]$wrongClassificationResult.runnerProcessStartCount -eq 0 -and
        [int]$wrongClassificationResult.validationExecutionCount -eq 0
    ) -Evidence ($wrongClassificationResult | ConvertTo-Json -Compress -Depth 8)

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

    Remove-Item -LiteralPath (Join-Path $fixtureRoot 'scripts/valid.ps1') -Force
    $missingPathRequest = $request | ConvertTo-Json -Depth 30 | ConvertFrom-Json -Depth 30
    $missingPathRequest.expectedStatusSha256 = [string](Get-GovernanceWorkingTreeStatusBinding -RepositoryRoot $fixtureRoot).Sha256
    $missingPathBinding = Test-GovernanceSourceBinding -Request $missingPathRequest
    Add-CaseResult -Name 'negative-expected-hash-bound-path-missing' -Passed (
        [string]$missingPathBinding.Status -ceq 'FAIL' -and
        @($missingPathBinding.Diagnostics | Where-Object { $_ -ceq 'Hash-bound source file is missing: scripts/valid.ps1' }).Count -eq 1 -and
        @($missingPathBinding.Diagnostics | Where-Object { $_ -ceq 'Working-tree status hash mismatch.' }).Count -eq 0
    )
    Write-StrictUtf8Text -Path (Join-Path $fixtureRoot 'scripts/valid.ps1') -Text "#requires -Version 7.6`n'fixture' | Out-Null`n"

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
    $setMismatchRequest.fileHashes = @($setMismatchRequest.fileHashes | Where-Object { [string]$_.path -cne 'scripts/valid.ps1' })
    $setMismatchBinding = Test-GovernanceSourceBinding -Request $setMismatchRequest
    Add-CaseResult -Name 'negative-scope-file-hash-set-mismatch' -Passed (
        [string]$setMismatchBinding.Status -ceq 'FAIL' -and
        @($setMismatchBinding.Diagnostics | Where-Object { $_ -ceq 'Scope path has no file-hash binding: scripts/valid.ps1' }).Count -eq 1
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

    $zeroSelectorRequest = $request | ConvertTo-Json -Depth 40 | ConvertFrom-Json -Depth 40
    $zeroSelectorRequest.selectors.caseNames = @()
    $zeroSelectorRequest.selectors.groups = @()
    $zeroSelectorRequest.selectors.tags = @('artifact-policy', 'bl-339-phase-a')
    $zeroSelectorResult = Invoke-GovernanceValidationOrchestration -Request $zeroSelectorRequest `
        -RepositoryRoot $resolvedRepositoryRoot
    Add-CaseResult -Name 'negative-canonical-zero-selection-stops-before-runner' -Passed (
        [string]$zeroSelectorResult.status -ceq 'FAIL' -and
        [int]$zeroSelectorResult.runnerProcessStartCount -eq 0 -and
        [int]$zeroSelectorResult.validationExecutionCount -eq 0 -and
        [int]$zeroSelectorResult.bindings.resolvedCaseCount -eq 0
    ) -Evidence ($zeroSelectorResult | ConvertTo-Json -Compress -Depth 12)

    $controllerRoot = Join-Path $fixtureRoot '.governance-task-controllers'
    [void][System.IO.Directory]::CreateDirectory($controllerRoot)
    $controllerPath = Join-Path $controllerRoot 'synthetic-task-controller.ps1'
    Write-StrictUtf8Text -Path $controllerPath -Text "#requires -Version 7.6`n'controller' | Out-Null`n"
    $controllerRequest = $request | ConvertTo-Json -Depth 40 | ConvertFrom-Json -Depth 40
    $controllerRequest.expectedStatusSha256 = [string](Get-GovernanceWorkingTreeStatusBinding -RepositoryRoot $fixtureRoot).Sha256
    $controllerRequest.taskControllers = @([ordered]@{
            path = $controllerPath; lineCount = 2
            classification = 'TASK_SPECIFIC_EXECUTABLE'; exceptionId = 'UNKNOWN'
        })
    $controllerResult = Invoke-GovernanceValidationOrchestration -Request $controllerRequest `
        -RepositoryRoot $resolvedRepositoryRoot
    Add-CaseResult -Name 'negative-task-controller-and-exception-fail-closed' -Passed (
        [string]$controllerResult.status -ceq 'FAIL' -and
        [int]$controllerResult.generatedTaskControllerFileCount -eq 1 -and
        [int]$controllerResult.generatedTaskControllerLineCount -eq 2 -and
        [int]$controllerResult.runnerProcessStartCount -eq 0 -and
        [int]$controllerResult.validationExecutionCount -eq 0
    ) -Evidence ($controllerResult | ConvertTo-Json -Compress -Depth 12)

    $undeclaredControllerRequest = $controllerRequest | ConvertTo-Json -Depth 40 | ConvertFrom-Json -Depth 40
    $undeclaredControllerRequest.taskControllers = @()
    $undeclaredControllerResult = Invoke-GovernanceValidationOrchestration -Request $undeclaredControllerRequest `
        -RepositoryRoot $resolvedRepositoryRoot
    Add-CaseResult -Name 'negative-undeclared-actual-controller-fails-authoritative-inventory' -Passed (
        [string]$undeclaredControllerResult.status -ceq 'FAIL' -and
        [int]$undeclaredControllerResult.generatedTaskControllerFileCount -eq 1 -and
        [int]$undeclaredControllerResult.generatedTaskControllerLineCount -eq 2 -and
        [string]$undeclaredControllerResult.bindings.actualStatusSha256 -ceq [string]$undeclaredControllerRequest.expectedStatusSha256 -and
        [int]$undeclaredControllerResult.runnerProcessStartCount -eq 0 -and
        [int]$undeclaredControllerResult.validationExecutionCount -eq 0
    ) -Evidence ($undeclaredControllerResult | ConvertTo-Json -Compress -Depth 10)

    $lineDriftRequest = $controllerRequest | ConvertTo-Json -Depth 40 | ConvertFrom-Json -Depth 40
    $lineDriftRequest.taskControllers[0].lineCount = 3
    $lineDriftResult = Invoke-GovernanceValidationOrchestration -Request $lineDriftRequest -RepositoryRoot $resolvedRepositoryRoot
    Add-CaseResult -Name 'negative-controller-line-count-drift' -Passed (
        [string]$lineDriftResult.status -ceq 'FAIL' -and
        [int]$lineDriftResult.runnerProcessStartCount -eq 0 -and
        [int]$lineDriftResult.validationExecutionCount -eq 0
    )

    $duplicateControllerRequest = $controllerRequest | ConvertTo-Json -Depth 40 | ConvertFrom-Json -Depth 40
    $duplicateControllerRequest.taskControllers = @($duplicateControllerRequest.taskControllers) + @($duplicateControllerRequest.taskControllers[0])
    $duplicateControllerResult = Invoke-GovernanceValidationOrchestration -Request $duplicateControllerRequest -RepositoryRoot $resolvedRepositoryRoot
    Add-CaseResult -Name 'negative-controller-duplicate-or-case-alias' -Passed (
        [string]$duplicateControllerResult.status -ceq 'FAIL' -and
        [int]$duplicateControllerResult.runnerProcessStartCount -eq 0 -and
        [int]$duplicateControllerResult.validationExecutionCount -eq 0
    )

    Remove-Item -LiteralPath $controllerRoot -Recurse -Force

    $otherControllerRoot = Join-Path $fixtureRoot 'other'
    [void][System.IO.Directory]::CreateDirectory($otherControllerRoot)
    $otherControllerPath = Join-Path $otherControllerRoot 'controller.ps1'
    Write-StrictUtf8Text -Path $otherControllerPath -Text "#requires -Version 7.6`n'other controller' | Out-Null`n"
    $otherControllerRequest = $request | ConvertTo-Json -Depth 40 | ConvertFrom-Json -Depth 40
    $otherControllerRequest.expectedStatusSha256 = [string](Get-GovernanceWorkingTreeStatusBinding -RepositoryRoot $fixtureRoot).Sha256
    $otherControllerResult = Invoke-GovernanceValidationOrchestration -Request $otherControllerRequest -RepositoryRoot $resolvedRepositoryRoot
    Add-CaseResult -Name 'negative-undeclared-controller-outside-legacy-inventory-root' -Passed (
        [string]$otherControllerResult.status -ceq 'FAIL' -and
        [int]$otherControllerResult.generatedTaskControllerFileCount -eq 1 -and
        [int]$otherControllerResult.generatedTaskControllerLineCount -eq 2 -and
        [string]$otherControllerResult.bindings.actualStatusSha256 -ceq [string]$otherControllerRequest.expectedStatusSha256 -and
        [int]$otherControllerResult.runnerProcessStartCount -eq 0 -and
        [int]$otherControllerResult.validationExecutionCount -eq 0
    ) -Evidence ($otherControllerResult | ConvertTo-Json -Compress -Depth 10)
    Remove-Item -LiteralPath $otherControllerRoot -Recurse -Force

    $rootControllerPath = Join-Path $fixtureRoot 'controller.ps1'
    Write-StrictUtf8Text -Path $rootControllerPath -Text "#requires -Version 7.6`n'root controller' | Out-Null`n"
    $rootControllerRequest = $request | ConvertTo-Json -Depth 40 | ConvertFrom-Json -Depth 40
    $rootControllerRequest.expectedStatusSha256 = [string](Get-GovernanceWorkingTreeStatusBinding -RepositoryRoot $fixtureRoot).Sha256
    $rootControllerResult = Invoke-GovernanceValidationOrchestration -Request $rootControllerRequest -RepositoryRoot $resolvedRepositoryRoot
    Add-CaseResult -Name 'negative-undeclared-controller-at-worktree-root' -Passed (
        [string]$rootControllerResult.status -ceq 'FAIL' -and
        [int]$rootControllerResult.generatedTaskControllerFileCount -eq 1 -and
        [int]$rootControllerResult.generatedTaskControllerLineCount -eq 2 -and
        [int]$rootControllerResult.runnerProcessStartCount -eq 0 -and
        [int]$rootControllerResult.validationExecutionCount -eq 0
    ) -Evidence ($rootControllerResult | ConvertTo-Json -Compress -Depth 10)
    Remove-Item -LiteralPath $rootControllerPath -Force

    $untrackedScriptsControllerPath = Join-Path $fixtureRoot 'scripts/temporary-controller.ps1'
    Write-StrictUtf8Text -Path $untrackedScriptsControllerPath -Text "#requires -Version 7.6`n'temporary controller' | Out-Null`n"
    $untrackedScriptsRequest = $request | ConvertTo-Json -Depth 40 | ConvertFrom-Json -Depth 40
    $untrackedScriptsRequest.expectedStatusSha256 = [string](Get-GovernanceWorkingTreeStatusBinding -RepositoryRoot $fixtureRoot).Sha256
    $untrackedScriptsResult = Invoke-GovernanceValidationOrchestration -Request $untrackedScriptsRequest -RepositoryRoot $resolvedRepositoryRoot
    Add-CaseResult -Name 'negative-untracked-controller-under-scripts' -Passed (
        [string]$untrackedScriptsResult.status -ceq 'FAIL' -and
        [int]$untrackedScriptsResult.generatedTaskControllerFileCount -eq 1 -and
        [int]$untrackedScriptsResult.generatedTaskControllerLineCount -eq 2 -and
        [int]$untrackedScriptsResult.runnerProcessStartCount -eq 0 -and
        [int]$untrackedScriptsResult.validationExecutionCount -eq 0
    ) -Evidence ($untrackedScriptsResult | ConvertTo-Json -Compress -Depth 10)

    $untrackedPermanentRequest = $untrackedScriptsRequest | ConvertTo-Json -Depth 40 | ConvertFrom-Json -Depth 40
    $untrackedPermanentRequest.taskControllers = @([ordered]@{
            path = $untrackedScriptsControllerPath; lineCount = 2
            classification = 'PERMANENT_PROFILE'; exceptionId = $null
        })
    $untrackedPermanentResult = Invoke-GovernanceValidationOrchestration -Request $untrackedPermanentRequest -RepositoryRoot $resolvedRepositoryRoot
    Add-CaseResult -Name 'negative-permanent-profile-must-be-git-tracked' -Passed (
        [string]$untrackedPermanentResult.status -ceq 'FAIL' -and
        [int]$untrackedPermanentResult.generatedTaskControllerFileCount -eq 1 -and
        [int]$untrackedPermanentResult.runnerProcessStartCount -eq 0 -and
        [int]$untrackedPermanentResult.validationExecutionCount -eq 0
    ) -Evidence ($untrackedPermanentResult | ConvertTo-Json -Compress -Depth 10)
    Remove-Item -LiteralPath $untrackedScriptsControllerPath -Force

    $outsideControllerPath = Join-Path $temporaryRoot 'outside-controller.ps1'
    Write-StrictUtf8Text -Path $outsideControllerPath -Text "#requires -Version 7.6`n'outside' | Out-Null`n"
    $outsideControllerRequest = $request | ConvertTo-Json -Depth 40 | ConvertFrom-Json -Depth 40
    $outsideControllerRequest.expectedStatusSha256 = [string](Get-GovernanceWorkingTreeStatusBinding -RepositoryRoot $fixtureRoot).Sha256
    $outsideControllerRequest.taskControllers = @([ordered]@{
            path = $outsideControllerPath; lineCount = 2
            classification = 'TASK_SPECIFIC_EXECUTABLE'; exceptionId = $null
        })
    $outsideControllerResult = Invoke-GovernanceValidationOrchestration -Request $outsideControllerRequest -RepositoryRoot $resolvedRepositoryRoot
    Add-CaseResult -Name 'negative-controller-outside-validated-worktree' -Passed (
        [string]$outsideControllerResult.status -ceq 'FAIL' -and
        [int]$outsideControllerResult.runnerProcessStartCount -eq 0 -and
        [int]$outsideControllerResult.validationExecutionCount -eq 0
    ) -Evidence ($outsideControllerResult | ConvertTo-Json -Compress -Depth 10)

    $missingControllerRequest = $request | ConvertTo-Json -Depth 40 | ConvertFrom-Json -Depth 40
    $missingControllerRequest.expectedStatusSha256 = [string](Get-GovernanceWorkingTreeStatusBinding -RepositoryRoot $fixtureRoot).Sha256
    $missingControllerRequest.taskControllers = @([ordered]@{
            path = Join-Path $fixtureRoot 'missing-controller.ps1'; lineCount = 2
            classification = 'TASK_SPECIFIC_EXECUTABLE'; exceptionId = $null
        })
    $missingControllerResult = Invoke-GovernanceValidationOrchestration -Request $missingControllerRequest -RepositoryRoot $resolvedRepositoryRoot
    Add-CaseResult -Name 'negative-declared-task-controller-absent' -Passed (
        [string]$missingControllerResult.status -ceq 'FAIL' -and
        [int]$missingControllerResult.runnerProcessStartCount -eq 0 -and
        [int]$missingControllerResult.validationExecutionCount -eq 0
    ) -Evidence ($missingControllerResult | ConvertTo-Json -Compress -Depth 10)

    $reparseTarget = Join-Path $temporaryRoot 'reparse-controller-target'
    [void][System.IO.Directory]::CreateDirectory($reparseTarget)
    Write-StrictUtf8Text -Path (Join-Path $reparseTarget 'controller.ps1') -Text "#requires -Version 7.6`n'reparse controller' | Out-Null`n"
    $reparseCandidateRoot = Join-Path $fixtureRoot 'linked-task-controllers'
    $reparseType = if ($IsWindows) { 'Junction' } else { 'SymbolicLink' }
    $null = New-Item -ItemType $reparseType -Path $reparseCandidateRoot -Target $reparseTarget
    $reparseCandidateRequest = $request | ConvertTo-Json -Depth 40 | ConvertFrom-Json -Depth 40
    $reparseCandidateRequest.expectedStatusSha256 = [string](Get-GovernanceWorkingTreeStatusBinding -RepositoryRoot $fixtureRoot).Sha256
    $reparseCandidateResult = Invoke-GovernanceValidationOrchestration -Request $reparseCandidateRequest -RepositoryRoot $resolvedRepositoryRoot
    Add-CaseResult -Name 'negative-reparse-controller-candidate-no-descent' -Passed (
        [string]$reparseCandidateResult.status -ceq 'FAIL' -and
        [int]$reparseCandidateResult.runnerProcessStartCount -eq 0 -and
        [int]$reparseCandidateResult.validationExecutionCount -eq 0
    ) -Evidence ($reparseCandidateResult | ConvertTo-Json -Compress -Depth 10)
    Remove-Item -LiteralPath $reparseCandidateRoot -Force

    $reparseWorktreeRoot = Join-Path $temporaryRoot 'reparse-worktree-root'
    $null = New-Item -ItemType $reparseType -Path $reparseWorktreeRoot -Target $fixtureRoot
    $reparseRootRequest = $request | ConvertTo-Json -Depth 40 | ConvertFrom-Json -Depth 40
    $reparseRootRequest.worktreeRoot = $reparseWorktreeRoot
    $reparseRootResult = Invoke-GovernanceValidationOrchestration -Request $reparseRootRequest -RepositoryRoot $resolvedRepositoryRoot
    Add-CaseResult -Name 'negative-reparse-worktree-root-stops-before-controller-inventory' -Passed (
        [string]$reparseRootResult.status -ceq 'FAIL' -and
        [int]$reparseRootResult.runnerProcessStartCount -eq 0 -and
        [int]$reparseRootResult.validationExecutionCount -eq 0 -and
        [int]$reparseRootResult.generatedTaskControllerFileCount -eq 0
    ) -Evidence ($reparseRootResult | ConvertTo-Json -Compress -Depth 10)
    Remove-Item -LiteralPath $reparseWorktreeRoot -Force

    $permanentProfileRequest = $request | ConvertTo-Json -Depth 40 | ConvertFrom-Json -Depth 40
    $permanentProfileRequest.expectedStatusSha256 = [string](Get-GovernanceWorkingTreeStatusBinding -RepositoryRoot $fixtureRoot).Sha256
    $permanentProfileRequest.taskControllers = @([ordered]@{
            path = Join-Path $fixtureRoot 'scripts/permanent-profile.ps1'; lineCount = 2
            classification = 'PERMANENT_PROFILE'; exceptionId = $null
        })
    $permanentProfileResult = Invoke-GovernanceValidationOrchestration -Request $permanentProfileRequest -RepositoryRoot $resolvedRepositoryRoot
    Add-CaseResult -Name 'positive-versioned-permanent-profile-binding' -Passed (
        [string]$permanentProfileResult.status -ceq 'PASS' -and
        [int]$permanentProfileResult.generatedTaskControllerFileCount -eq 0 -and
        [int]$permanentProfileResult.generatedTaskControllerLineCount -eq 0
    ) -Evidence ($permanentProfileResult | ConvertTo-Json -Compress -Depth 8)

    $falseCallerCurrentRequest = $request | ConvertTo-Json -Depth 40 | ConvertFrom-Json -Depth 40
    foreach ($component in @($falseCallerCurrentRequest.stateComponents)) { $component.currentSha256 = ('0' * 64) }
    $falseCallerCurrentResult = Invoke-GovernanceValidationOrchestration -Request $falseCallerCurrentRequest -RepositoryRoot $resolvedRepositoryRoot
    Add-CaseResult -Name 'caller-current-hash-cannot-defeat-actual-reuse-binding' -Passed (
        [string]$falseCallerCurrentResult.status -ceq 'PASS' -and
        @($falseCallerCurrentResult.stateTransitionMap | Where-Object Disposition -cne 'REUSED').Count -eq 0
    ) -Evidence ($falseCallerCurrentResult.stateTransitionMap | ConvertTo-Json -Compress -Depth 5)

    $packageDriftPath = Join-Path $temporaryRoot 'documentation-result-package-drift.json'
    $packageDrift = $subordinate | ConvertTo-Json -Depth 30 | ConvertFrom-Json -Depth 30
    $packageDrift.readOnlyProbeCount = 2
    Write-StrictUtf8Text -Path $packageDriftPath -Text (($packageDrift | ConvertTo-Json -Depth 30) + "`n")
    $packageDriftRequest = $request | ConvertTo-Json -Depth 40 | ConvertFrom-Json -Depth 40
    $packageDriftRequest.subordinateResults[0].path = $packageDriftPath
    $packageDriftRequest.subordinateResults[0].sha256 = (Get-FileHash -LiteralPath $packageDriftPath -Algorithm SHA256).Hash.ToLowerInvariant()
    $packageDriftResult = Invoke-GovernanceValidationOrchestration -Request $packageDriftRequest -RepositoryRoot $resolvedRepositoryRoot
    $packageInvalidated = @($packageDriftResult.stateTransitionMap | Where-Object Disposition -ceq 'INVALIDATED')
    Add-CaseResult -Name 'actual-package-drift-invalidates-only-package' -Passed (
        [string]$packageDriftResult.status -ceq 'PASS' -and $packageInvalidated.Count -eq 1 -and
        [string]$packageInvalidated[0].Component -ceq 'package'
    ) -Evidence ($packageDriftResult.stateTransitionMap | ConvertTo-Json -Compress -Depth 5)

    $evidenceDriftPath = Join-Path $temporaryRoot 'prior-pass-drift.json'
    Write-StrictUtf8Text -Path $evidenceDriftPath -Text "{`"result`":`"PASS`",`"refresh`":true}`n"
    $evidenceDriftRequest = $request | ConvertTo-Json -Depth 40 | ConvertFrom-Json -Depth 40
    $evidenceDriftRequest.priorEvidence[0].path = $evidenceDriftPath
    $evidenceDriftRequest.priorEvidence[0].sha256 = (Get-FileHash -LiteralPath $evidenceDriftPath -Algorithm SHA256).Hash.ToLowerInvariant()
    $evidenceDriftResult = Invoke-GovernanceValidationOrchestration -Request $evidenceDriftRequest -RepositoryRoot $resolvedRepositoryRoot
    $evidenceInvalidated = @($evidenceDriftResult.stateTransitionMap | Where-Object Disposition -ceq 'INVALIDATED')
    Add-CaseResult -Name 'actual-evidence-drift-invalidates-only-evidence' -Passed (
        [string]$evidenceDriftResult.status -ceq 'PASS' -and $evidenceInvalidated.Count -eq 1 -and
        [string]$evidenceInvalidated[0].Component -ceq 'evidence'
    ) -Evidence ($evidenceDriftResult.stateTransitionMap | ConvertTo-Json -Compress -Depth 5)

    Write-StrictUtf8Text -Path (Join-Path $fixtureRoot 'ignored.txt') -Text "ignored input drift`n"
    $externalDriftRequest = $request | ConvertTo-Json -Depth 40 | ConvertFrom-Json -Depth 40
    $externalDriftRequest.externalInputs[0].sha256 = (Get-FileHash -LiteralPath (Join-Path $fixtureRoot 'ignored.txt') -Algorithm SHA256).Hash.ToLowerInvariant()
    $externalDriftResult = Invoke-GovernanceValidationOrchestration -Request $externalDriftRequest -RepositoryRoot $resolvedRepositoryRoot
    $externalInvalidated = @($externalDriftResult.stateTransitionMap | Where-Object Disposition -ceq 'INVALIDATED')
    Add-CaseResult -Name 'actual-external-input-drift-invalidates-only-external-inputs' -Passed (
        [string]$externalDriftResult.status -ceq 'PASS' -and $externalInvalidated.Count -eq 1 -and
        [string]$externalInvalidated[0].Component -ceq 'external-inputs'
    ) -Evidence ($externalDriftResult.stateTransitionMap | ConvertTo-Json -Compress -Depth 5)
    Write-StrictUtf8Text -Path (Join-Path $fixtureRoot 'ignored.txt') -Text "ignored input`n"

    Write-StrictUtf8Text -Path (Join-Path $fixtureRoot 'fixture.txt') -Text "actual scope drift`n"
    $scopeDriftRequest = $request | ConvertTo-Json -Depth 40 | ConvertFrom-Json -Depth 40
    $scopeDriftRequest.expectedStatusSha256 = [string](Get-GovernanceWorkingTreeStatusBinding -RepositoryRoot $fixtureRoot).Sha256
    @($scopeDriftRequest.fileHashes | Where-Object path -ceq 'fixture.txt')[0].sha256 = `
        (Get-FileHash -LiteralPath (Join-Path $fixtureRoot 'fixture.txt') -Algorithm SHA256).Hash.ToLowerInvariant()
    $scopeDriftResult = Invoke-GovernanceValidationOrchestration -Request $scopeDriftRequest -RepositoryRoot $resolvedRepositoryRoot
    $scopeInvalidatedNames = @($scopeDriftResult.stateTransitionMap | Where-Object Disposition -ceq 'INVALIDATED' | ForEach-Object Component | Sort-Object)
    Add-CaseResult -Name 'actual-working-status-and-scope-drift-are-component-specific' -Passed (
        [string]$scopeDriftResult.status -ceq 'PASS' -and
        ($scopeInvalidatedNames -join ',') -ceq 'scope,working-status'
    ) -Evidence ($scopeDriftResult.stateTransitionMap | ConvertTo-Json -Compress -Depth 5)
    Write-StrictUtf8Text -Path (Join-Path $fixtureRoot 'fixture.txt') -Text "fixture`n"

    $selectorDriftRequest = $request | ConvertTo-Json -Depth 40 | ConvertFrom-Json -Depth 40
    $selectorDriftRequest.selectors.caseNames = @('positive-current-state-gate')
    $selectorDriftResult = Invoke-GovernanceValidationOrchestration -Request $selectorDriftRequest -RepositoryRoot $resolvedRepositoryRoot
    $selectorInvalidated = @($selectorDriftResult.stateTransitionMap | Where-Object Disposition -ceq 'INVALIDATED')
    Add-CaseResult -Name 'actual-selector-drift-invalidates-only-selector' -Passed (
        [string]$selectorDriftResult.status -ceq 'PASS' -and $selectorInvalidated.Count -eq 1 -and
        [string]$selectorInvalidated[0].Component -ceq 'selector'
    ) -Evidence ($selectorDriftResult.stateTransitionMap | ConvertTo-Json -Compress -Depth 5)

    Write-StrictUtf8Text -Path (Join-Path $fixtureRoot 'scripts/valid.ps1') -Text "#requires -Version 7.6`n'exact commit profile' | Out-Null`n"
    $exactRequest = $request | ConvertTo-Json -Depth 50 | ConvertFrom-Json -Depth 50
    $exactRequest.profile = 'commit-preparation'
    $exactRequest.expectedStatusSha256 = [string](Get-GovernanceWorkingTreeStatusBinding -RepositoryRoot $fixtureRoot).Sha256
    @($exactRequest.fileHashes | Where-Object path -ceq 'scripts/valid.ps1')[0].sha256 = `
        (Get-FileHash -LiteralPath (Join-Path $fixtureRoot 'scripts/valid.ps1') -Algorithm SHA256).Hash.ToLowerInvariant()
    $exactRequest.subordinateResults[0].id = 'scope-patch-handoff'
    $exactEntries = @(Get-GenericStatusEvidence -Root $fixtureRoot -BaselineCommit $commit)
    $exactDelta = Get-GenericDeltaEvidence -Root $fixtureRoot -BaselineCommit $commit `
        -IncludedEntry $exactEntries -ExcludedEntry @()
    $exactProjection = Get-GenericIntegrationProjectionEvidence -Root $fixtureRoot `
        -BaselineCommit $commit -IncludedEntry $exactEntries -ExcludedEntry @()
    $exactRequest.exactCommit = [ordered]@{
        intendedBase = $commit; targetRef = $commit; expectedMergeBase = $commit
        authorizedWriteSet = @('scripts/valid.ps1'); expectedEffectivePRScope = @('scripts/valid.ps1')
        expectedEffectivePRPatchSha256 = [Convert]::ToHexString(
            [Security.Cryptography.SHA256]::HashData([byte[]]$exactDelta.Bytes)
        ).ToLowerInvariant()
        expectedIntegrationProjection = [string]$exactProjection.Tree
    }
    $exactResult = Invoke-GovernanceValidationOrchestration -Request $exactRequest `
        -RepositoryRoot $resolvedRepositoryRoot
    Add-CaseResult -Name 'positive-exact-commit-intended-base-push-scope-projection' -Passed (
        [string]$exactResult.status -ceq 'PASS' -and
        @($exactResult.profileResults.PSObject.Properties.Value | Where-Object { $_ -ceq 'FAIL' }).Count -eq 0 -and
        [string]$exactResult.profileResults.IntendedBaseResult -ceq 'PASS' -and
        [string]$exactResult.profileResults.ForeignProtectedStateResult -ceq 'PASS_1_OF_1'
    ) -Evidence ($exactResult.profileResults | ConvertTo-Json -Compress -Depth 8)

    $null = Invoke-RequiredGit -Root $fixtureRoot -Argument @('add', '--', 'scripts/valid.ps1')
    $stagedRequest = $exactRequest | ConvertTo-Json -Depth 50 | ConvertFrom-Json -Depth 50
    $stagedRequest.expectedStatusSha256 = [string](Get-GovernanceWorkingTreeStatusBinding -RepositoryRoot $fixtureRoot).Sha256
    $stagedResult = Invoke-GovernanceValidationOrchestration -Request $stagedRequest `
        -RepositoryRoot $resolvedRepositoryRoot
    Add-CaseResult -Name 'negative-exact-commit-staged-input-fails-closed' -Passed (
        [string]$stagedResult.status -ceq 'FAIL' -and
        [string]$stagedResult.profileResults.AuthorizedWriteSetResult -ceq 'FAIL' -and
        [int]$stagedResult.runnerProcessStartCount -eq 0 -and
        [int]$stagedResult.validationExecutionCount -eq 0
    ) -Evidence ($stagedResult.profileResults | ConvertTo-Json -Compress -Depth 8)
    $null = Invoke-RequiredGit -Root $fixtureRoot -Argument @('reset', '--', 'scripts/valid.ps1')
    Write-StrictUtf8Text -Path (Join-Path $fixtureRoot 'scripts/valid.ps1') -Text "#requires -Version 7.6`n'fixture' | Out-Null`n"

    $status = if (@($results | Where-Object Result -ceq 'FAIL').Count -eq 0) { 'PASS' } else { 'FAIL' }
}
catch {
    $failureMessage = "$($_.Exception.Message) | $($_.ScriptStackTrace)"
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
