#requires -Version 7.6

[CmdletBinding(PositionalBinding = $false)]
param(
    [string]$RepositoryRoot = (Split-Path -Parent $PSScriptRoot),
    [string]$WorkingPath = [System.IO.Path]::GetTempPath(),
    [string]$ResultPath
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$utf8 = [System.Text.UTF8Encoding]::new($false, $true)

function Invoke-HarnessProbe {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][string]$PowerShellPath,
        [Parameter(Mandatory)][string]$HarnessPath,
        [Parameter(Mandatory)][string]$Repository,
        [Parameter(Mandatory)][string]$ProbeResultPath,
        [Parameter(Mandatory)][string[]]$ProbeArgument,
        [Parameter(Mandatory)][int]$ExpectedExitCode
    )

    $startInfo = [System.Diagnostics.ProcessStartInfo]::new()
    $startInfo.FileName = $PowerShellPath
    $startInfo.WorkingDirectory = $Repository
    $startInfo.UseShellExecute = $false
    $startInfo.CreateNoWindow = $true
    $startInfo.RedirectStandardOutput = $true
    $startInfo.RedirectStandardError = $true
    $startInfo.StandardOutputEncoding = $utf8
    $startInfo.StandardErrorEncoding = $utf8
    foreach ($argument in @(
            '-NoLogo', '-NoProfile', '-NonInteractive', '-File', $HarnessPath,
            '-RepositoryRoot', $Repository, '-ResultPath', $ProbeResultPath
        ) + $ProbeArgument) {
        [void]$startInfo.ArgumentList.Add($argument)
    }
    $startInfo.Environment['POWERSHELL_TELEMETRY_OPTOUT'] = '1'

    $process = [System.Diagnostics.Process]::new()
    $process.StartInfo = $startInfo
    try {
        if (-not $process.Start()) {
            throw 'Hosted-CI portability probe process did not start.'
        }
        $stdoutTask = $process.StandardOutput.ReadToEndAsync()
        $stderrTask = $process.StandardError.ReadToEndAsync()
        if (-not $process.WaitForExit(300000)) {
            try { $process.Kill($true) } catch { }
            throw 'Hosted-CI portability probe timed out.'
        }
        $stdout = $stdoutTask.GetAwaiter().GetResult()
        $stderr = $stderrTask.GetAwaiter().GetResult()
        if ($process.ExitCode -ne $ExpectedExitCode) {
            throw "Hosted-CI portability probe exit mismatch: expected=$ExpectedExitCode actual=$($process.ExitCode) stderr=$stderr"
        }
        if (-not (Test-Path -LiteralPath $ProbeResultPath -PathType Leaf)) {
            throw 'Hosted-CI portability probe did not create its result.'
        }
        $resultText = [System.IO.File]::ReadAllText($ProbeResultPath, $utf8)
        $result = $resultText | ConvertFrom-Json -Depth 100 -DateKind String
        return [pscustomobject]@{
            ExitCode = $process.ExitCode
            StdOut = $stdout
            StdErr = $stderr
            Result = $result
        }
    }
    finally {
        $process.Dispose()
    }
}

$status = 'FAIL'
$failureMessage = $null
$temporaryRoot = $null
$cleanupResult = 'NOT_RUN'
$listGroupsResult = 'NOT_RUN'
$listTagsResult = 'NOT_RUN'
$listCasesResult = 'NOT_RUN'
$canonicalCaseSelectionResult = 'NOT_RUN'
$fullFixturePreflightResult = 'NOT_RUN'
$invalidSelectorFailClosedResult = 'NOT_RUN'
$localCodexWorkDependencyCount = -1
$hardcodedContributorInfrastructurePathCount = -1

try {
    $repository = [System.IO.Path]::GetFullPath($RepositoryRoot)
    $working = [System.IO.Path]::GetFullPath($WorkingPath)
    if (-not (Test-Path -LiteralPath $working -PathType Container)) {
        throw 'WorkingPath must be an existing directory.'
    }
    $temporaryRoot = Join-Path $working ('hosted-ci-portability-' + [guid]::NewGuid().ToString('N'))
    [void][System.IO.Directory]::CreateDirectory($temporaryRoot)
    $harnessPath = Join-Path $repository 'scripts/Test-GovernanceConsistencyFixtures.ps1'
    $validatorPath = Join-Path $repository 'scripts/Test-ClassicReviewArtifact.ps1'
    $workflowPath = Join-Path $repository '.github/workflows/ci.yml'
    $pwsh = Join-Path $PSHOME $(if ($IsWindows) { 'pwsh.exe' } else { 'pwsh' })
    foreach ($path in @($harnessPath, $validatorPath, $workflowPath, $pwsh)) {
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
            throw "Required hosted-portability input does not exist: $path"
        }
    }

    $harnessSource = [System.IO.File]::ReadAllText($harnessPath, $utf8)
    $runtimeMarker = "`$status = 'FAIL'"
    $runtimeStart = $harnessSource.IndexOf($runtimeMarker, [System.StringComparison]::Ordinal)
    if ($runtimeStart -lt 0) { throw 'Harness runtime marker was not found.' }
    $runtimeSource = $harnessSource.Substring($runtimeStart)
    $localCodexWorkDependencyCount = [regex]::Matches(
        $harnessSource,
        'ExecutionRoutingRoot|SandboxExecutionRouting|WINDOWS-SANDBOX-EXECUTION|externalCanonicalArtifactValidatorPath|external-windows-governance-backups'
    ).Count
    $hardcodedContributorInfrastructurePathCount = [regex]::Matches(
        $runtimeSource,
        'C:\\(?:Users\\ThomasW|Voxtronic)|OneDrive\s+-\s+VOXTRONIC|Codex-Work'
    ).Count

    $workflowSource = [System.IO.File]::ReadAllText($workflowPath, $utf8)
    if (-not $workflowSource.Contains('.\scripts\Test-GovernanceConsistencyFixtures.ps1') -or
        -not $workflowSource.Contains("-CanonicalArtifactValidatorPath (Join-Path `$PWD 'scripts\Test-ClassicReviewArtifact.ps1')") -or
        $workflowSource.Contains('-ExecutionRoutingRoot')) {
        throw 'Hosted-CI workflow invocation contract is not portable.'
    }

    $groups = Invoke-HarnessProbe -PowerShellPath $pwsh -HarnessPath $harnessPath `
        -Repository $repository -ProbeResultPath (Join-Path $temporaryRoot 'groups.json') `
        -ProbeArgument @('-ListGroups') -ExpectedExitCode 0
    $listGroupsResult = if ($groups.Result.ListResult -ceq 'PASS' -and
        [int]$groups.Result.RunnerProcessStartCount -eq 0 -and
        [int]$groups.Result.ValidationExecutionCount -eq 0) { 'PASS' } else { 'FAIL' }

    $tags = Invoke-HarnessProbe -PowerShellPath $pwsh -HarnessPath $harnessPath `
        -Repository $repository -ProbeResultPath (Join-Path $temporaryRoot 'tags.json') `
        -ProbeArgument @('-ListTags') -ExpectedExitCode 0
    $listTagsResult = if ($tags.Result.ListResult -ceq 'PASS' -and
        [int]$tags.Result.RunnerProcessStartCount -eq 0 -and
        [int]$tags.Result.ValidationExecutionCount -eq 0) { 'PASS' } else { 'FAIL' }

    $cases = Invoke-HarnessProbe -PowerShellPath $pwsh -HarnessPath $harnessPath `
        -Repository $repository -ProbeResultPath (Join-Path $temporaryRoot 'cases.json') `
        -ProbeArgument @('-ListCases') -ExpectedExitCode 0
    $listCasesResult = if ($cases.Result.ListResult -ceq 'PASS' -and
        [int]$cases.Result.RunnerProcessStartCount -eq 0 -and
        [int]$cases.Result.ValidationExecutionCount -eq 0) { 'PASS' } else { 'FAIL' }

    $invalid = Invoke-HarnessProbe -PowerShellPath $pwsh -HarnessPath $harnessPath `
        -Repository $repository -ProbeResultPath (Join-Path $temporaryRoot 'invalid.json') `
        -ProbeArgument @('-CaseName', 'hosted-ci-unknown-case', '-TargetPlatform', 'windows') `
        -ExpectedExitCode 1
    $invalidSelectorFailClosedResult = if (
        $invalid.Result.SelectorResolutionResult -ceq 'FAIL' -and
        [int]$invalid.Result.UnresolvedSelectorCount -eq 1 -and
        [int]$invalid.Result.RunnerProcessStartCount -eq 0 -and
        [int]$invalid.Result.ValidationExecutionCount -eq 0
    ) { 'PASS' } else { 'FAIL' }

    $canonical = Invoke-HarnessProbe -PowerShellPath $pwsh -HarnessPath $harnessPath `
        -Repository $repository -ProbeResultPath (Join-Path $temporaryRoot 'canonical.json') `
        -ProbeArgument @(
            '-CaseName', 'positive-commit-preparation',
            '-TargetPlatform', 'windows',
            '-CanonicalArtifactValidatorPath', $validatorPath
        ) -ExpectedExitCode 0
    $canonicalCaseSelectionResult = if (
        $canonical.Result.SelectorResolutionResult -ceq 'PASS' -and
        [int]$canonical.Result.SelectedFixtureCount -eq 1 -and
        [string]$canonical.Result.SelectedFixtureNames[0] -ceq 'positive-commit-preparation'
    ) { 'PASS' } else { 'FAIL' }
    $fullFixturePreflightResult = [string]$canonical.Result.FullFixturePreflightResult

    $requiredPass = @(
        $listGroupsResult,
        $listTagsResult,
        $listCasesResult,
        $canonicalCaseSelectionResult,
        $fullFixturePreflightResult,
        $invalidSelectorFailClosedResult
    )
    if ($requiredPass -contains 'FAIL' -or
        $localCodexWorkDependencyCount -ne 0 -or
        $hardcodedContributorInfrastructurePathCount -ne 0) {
        throw 'Hosted-CI portability acceptance contract failed.'
    }
    $status = 'PASS'
}
catch {
    $failureMessage = $_.Exception.Message
}
finally {
    try {
        if ($null -ne $temporaryRoot -and (Test-Path -LiteralPath $temporaryRoot -PathType Container)) {
            $resolvedTemporaryRoot = [System.IO.Path]::GetFullPath($temporaryRoot)
            $resolvedWorking = [System.IO.Path]::GetFullPath($WorkingPath)
            if (-not $resolvedTemporaryRoot.StartsWith(
                    $resolvedWorking.TrimEnd([System.IO.Path]::DirectorySeparatorChar) +
                        [System.IO.Path]::DirectorySeparatorChar,
                    [System.StringComparison]::OrdinalIgnoreCase
                ) -or
                -not [System.IO.Path]::GetFileName($resolvedTemporaryRoot).StartsWith(
                    'hosted-ci-portability-',
                    [System.StringComparison]::Ordinal
                )) {
                throw 'Temporary portability root failed its bounded cleanup check.'
            }
            Remove-Item -LiteralPath $resolvedTemporaryRoot -Recurse -Force
        }
        $cleanupResult = 'PASS'
    }
    catch {
        $cleanupResult = 'FAIL'
        if ([string]::IsNullOrWhiteSpace($failureMessage)) {
            $failureMessage = $_.Exception.Message
        }
    }
}

if ($cleanupResult -cne 'PASS') { $status = 'FAIL' }
$failureCount = if ($status -ceq 'PASS') { 0 } else { 1 }
$result = [pscustomobject][ordered]@{
    Status = $status
    HostedCiPortableHarnessResult = $status
    ListGroupsResult = $listGroupsResult
    ListTagsResult = $listTagsResult
    ListCasesResult = $listCasesResult
    CanonicalCaseSelectionResult = $canonicalCaseSelectionResult
    FullFixturePreflightResult = $fullFixturePreflightResult
    InvalidSelectorFailClosedResult = $invalidSelectorFailClosedResult
    LocalCodexWorkDependencyCount = $localCodexWorkDependencyCount
    HardcodedContributorInfrastructurePathCount = $hardcodedContributorInfrastructurePathCount
    RunnerProcessStartCountForInvalidSelector = 0
    ValidationExecutionCountForInvalidSelector = 0
    CleanupResult = $cleanupResult
    WarningCount = 0
    FailureCount = $failureCount
    FailureMessage = $failureMessage
}
$json = ($result | ConvertTo-Json -Depth 12 -Compress) + [Environment]::NewLine
if (-not [string]::IsNullOrWhiteSpace($ResultPath)) {
    $resolvedResultPath = [System.IO.Path]::GetFullPath($ResultPath)
    if (Test-Path -LiteralPath $resolvedResultPath) {
        throw 'ResultPath must be new.'
    }
    [System.IO.File]::WriteAllText($resolvedResultPath, $json, [System.Text.UTF8Encoding]::new($false))
}
$json

if ($status -ceq 'PASS') { exit 0 }
exit 1
