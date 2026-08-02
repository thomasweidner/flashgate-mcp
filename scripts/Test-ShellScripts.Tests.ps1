#requires -Version 7.6
[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$modulePath = Join-Path $PSScriptRoot 'ShellValidation.psm1'
$gitBashPath = 'C:\Program Files\Git\bin\bash.exe'
$testRoot = Join-Path ([IO.Path]::GetTempPath()) ("BL-251 shell validation tests {0}" -f [guid]::NewGuid().ToString('N'))
$failures = [System.Collections.Generic.List[string]]::new()
$passCount = 0
$testCount = 0

function Add-TestResult {
    param(
        [Parameter(Mandatory)][string]$Name,
        [Parameter(Mandatory)][bool]$Passed,
        [string]$Detail = ''
    )
    $script:testCount++
    if ($Passed) {
        $script:passCount++
    }
    else {
        $script:failures.Add("$Name`: $Detail")
    }
}

function New-FixtureRepository {
    param(
        [Parameter(Mandatory)][string]$Name,
        [string]$PowerShellContent = "'ok'`n",
        [string]$BashContent = "#!/usr/bin/env bash`nset -euo pipefail`nprintf 'ok\\n'`n",
        [switch]$NoShellFiles
    )

    $root = Join-Path $testRoot $Name
    [void][IO.Directory]::CreateDirectory((Join-Path $root 'scripts'))
    & git -C $root init --quiet
    if ($LASTEXITCODE -ne 0) {
        throw "git init failed for fixture: $Name"
    }
    if ($NoShellFiles) {
        Set-Content -LiteralPath (Join-Path $root 'README.md') -Value "fixture`n" -NoNewline -Encoding utf8NoBOM
    }
    else {
        Set-Content -LiteralPath (Join-Path $root 'scripts\fixture.ps1') -Value $PowerShellContent -NoNewline -Encoding utf8NoBOM
        Set-Content -LiteralPath (Join-Path $root 'scripts\fixture.sh') -Value $BashContent -NoNewline -Encoding utf8NoBOM
    }
    & git -C $root add --all
    if ($LASTEXITCODE -ne 0) {
        throw "git add failed for fixture: $Name"
    }
    return $root
}

try {
    Import-Module $modulePath -Force
    [void][IO.Directory]::CreateDirectory($testRoot)

    $successRoot = New-FixtureRepository -Name 'success path with spaces'
    $successBefore = (& git -C $successRoot status --porcelain=v1 --untracked-files=all) -join "`n"
    $success = Invoke-FlashGateShellValidation -RepositoryRoot $successRoot -GitBashPath $gitBashPath
    $successAfter = (& git -C $successRoot status --porcelain=v1 --untracked-files=all) -join "`n"
    Add-TestResult -Name 'success-and-space-path' -Passed (
        $success.Status -eq 'PASS' -and
        $success.InventoryCount -eq 2 -and
        $success.PowerShellScriptCount -eq 1 -and
        $success.BashScriptCount -eq 1
    ) -Detail (($success.Diagnostics | ForEach-Object { $_.Message }) -join ' | ')
    Add-TestResult -Name 'success-does-not-mutate-repository' -Passed (
        [string]::Equals($successBefore, $successAfter, [StringComparison]::Ordinal)
    ) -Detail 'Repository status changed.'

    $deterministic = Invoke-FlashGateShellValidation -RepositoryRoot $successRoot -GitBashPath $gitBashPath
    Add-TestResult -Name 'deterministic-inventory' -Passed (
        (@($success.InventoryPaths) -join "`n") -ceq (@($deterministic.InventoryPaths) -join "`n")
    ) -Detail 'Repeated inventory order differs.'

    $emptyRoot = New-FixtureRepository -Name 'empty' -NoShellFiles
    $empty = Invoke-FlashGateShellValidation -RepositoryRoot $emptyRoot -GitBashPath $gitBashPath
    Add-TestResult -Name 'empty-inventory-fails' -Passed ($empty.Status -eq 'FAIL') -Detail 'Empty inventory passed.'

    $missingRoot = New-FixtureRepository -Name 'missing'
    Remove-Item -LiteralPath (Join-Path $missingRoot 'scripts\fixture.ps1')
    $missing = Invoke-FlashGateShellValidation -RepositoryRoot $missingRoot -GitBashPath $gitBashPath
    Add-TestResult -Name 'missing-tracked-file-fails' -Passed ($missing.Status -eq 'FAIL') -Detail 'Missing tracked file passed.'

    $psSyntaxRoot = New-FixtureRepository -Name 'powershell syntax' -PowerShellContent "if (`$true) {`n"
    $psSyntax = Invoke-FlashGateShellValidation -RepositoryRoot $psSyntaxRoot -GitBashPath $gitBashPath
    Add-TestResult -Name 'powershell-syntax-fails-with-location' -Passed (
        $psSyntax.Status -eq 'FAIL' -and
        @($psSyntax.Diagnostics | Where-Object {
            $_.Code -eq 'PowerShellParser' -and $_.Line -gt 0 -and $_.Column -gt 0
        }).Count -gt 0
    ) -Detail 'PowerShell parser location was not reported.'

    $bashSyntaxRoot = New-FixtureRepository -Name 'bash syntax' -BashContent "#!/usr/bin/env bash`nif then`n"
    $bashSyntax = Invoke-FlashGateShellValidation -RepositoryRoot $bashSyntaxRoot -GitBashPath $gitBashPath
    Add-TestResult -Name 'git-bash-syntax-fails' -Passed ($bashSyntax.Status -eq 'FAIL') -Detail 'Invalid Bash passed.'

    $wrongShell = Invoke-FlashGateShellValidation -RepositoryRoot $successRoot -GitBashPath 'C:\Windows\System32\cmd.exe'
    Add-TestResult -Name 'wrong-git-bash-path-fails' -Passed ($wrongShell.Status -eq 'FAIL') -Detail 'Wrong shell path passed.'

    Add-TestResult -Name 'wrong-powershell-version-fails' -Passed (
        -not (Test-RequiredPowerShellVersion -ActualVersion '7.6.3')
    ) -Detail 'PowerShell 7.6.3 was accepted.'
    Add-TestResult -Name 'required-powershell-version-passes' -Passed (
        Test-RequiredPowerShellVersion -ActualVersion '7.6.4'
    ) -Detail 'PowerShell 7.6.4 was rejected.'

    $invalidRoot = Invoke-FlashGateShellValidation -RepositoryRoot (Join-Path $testRoot 'not-present') -GitBashPath $gitBashPath
    Add-TestResult -Name 'invalid-repository-root-fails' -Passed ($invalidRoot.Status -eq 'FAIL') -Detail 'Invalid root passed.'

    $boundedSuccess = Invoke-BoundedProcess `
        -FilePath 'C:\Program Files\PowerShell\7\pwsh.exe' `
        -ArgumentList @('-NoLogo', '-NoProfile', '-Command', "[Console]::Out.Write('ok')") `
        -TimeoutSeconds 5
    Add-TestResult -Name 'bounded-subprocess-exit-zero' -Passed (
        $boundedSuccess.ExitCode -eq 0 -and
        -not $boundedSuccess.TimedOut -and
        $boundedSuccess.FailureClassification -eq 'None' -and
        $boundedSuccess.StreamStatus -eq 'PASS' -and
        $boundedSuccess.StandardOutput -eq 'ok'
    ) -Detail 'Successful bounded process was not classified deterministically.'

    $boundedFailure = Invoke-BoundedProcess `
        -FilePath 'C:\Program Files\PowerShell\7\pwsh.exe' `
        -ArgumentList @('-NoLogo', '-NoProfile', '-Command', 'exit 7') `
        -TimeoutSeconds 5
    Add-TestResult -Name 'bounded-subprocess-nonzero-exit' -Passed (
        $boundedFailure.ExitCode -eq 7 -and
        -not $boundedFailure.TimedOut -and
        $boundedFailure.FailureClassification -eq 'None' -and
        $boundedFailure.StreamStatus -eq 'PASS'
    ) -Detail 'Nonzero bounded process was not returned without reclassification.'

    $timeoutPidPath = Join-Path $testRoot 'bounded-timeout.pid'
    $escapedTimeoutPidPath = $timeoutPidPath.Replace("'", "''")
    $timeout = Invoke-BoundedProcess `
        -FilePath 'C:\Program Files\PowerShell\7\pwsh.exe' `
        -ArgumentList @(
            '-NoLogo', '-NoProfile', '-Command',
            "Start-Sleep -Seconds 30; [IO.File]::WriteAllText('$escapedTimeoutPidPath', [string]`$PID)"
        ) `
        -TimeoutSeconds 1
    Add-TestResult -Name 'bounded-subprocess-timeout' -Passed (
        $timeout.TimedOut -and
        $timeout.ExitCode -eq 124 -and
        $timeout.ProcessId -gt 0 -and
        $timeout.TerminationStatus -eq 'PASS' -and
        $timeout.StreamStatus -eq 'PASS' -and
        $timeout.FailureClassification -eq 'ProcessTimedOut'
    ) -Detail 'Timeout was not bounded and classified.'
    $timeoutProcessId = $timeout.ProcessId
    Add-TestResult -Name 'bounded-timeout-leaves-no-process' -Passed (
        $null -eq (Get-Process -Id $timeoutProcessId -ErrorAction SilentlyContinue)
    ) -Detail "Timed-out process remains active: $timeoutProcessId"
    Add-TestResult -Name 'bounded-timeout-does-not-require-child-pid-file' -Passed (
        -not (Test-Path -LiteralPath $timeoutPidPath)
    ) -Detail 'The timed-out child unexpectedly reached the optional PID-file write.'

    $module = Get-Module -Name ShellValidation
    $terminationFailureWatch = [Diagnostics.Stopwatch]::StartNew()
    $terminationFailure = & $module {
        Complete-BoundedProcessResult `
            -Process $null `
            -ProcessId $PID `
            -StandardOutputTask $null `
            -StandardErrorTask $null `
            -TimedOut $true `
            -TerminationConfirmed $false `
            -TerminationStatus 'FAIL' `
            -StreamWaitMilliseconds 5000
    }
    $terminationFailureWatch.Stop()
    Add-TestResult -Name 'termination-failure-is-fail-closed' -Passed (
        $terminationFailure.ExitCode -eq 124 -and
        $terminationFailure.ProcessId -eq $PID -and
        $terminationFailure.TimedOut -and
        $terminationFailure.TerminationStatus -eq 'FAIL' -and
        $terminationFailure.StreamStatus -eq 'NOT_READ' -and
        $terminationFailure.FailureClassification -eq 'ProcessTerminationFailed'
    ) -Detail 'Termination failure did not return the deterministic fail-closed result.'
    Add-TestResult -Name 'termination-failure-does-not-wait-for-streams' -Passed (
        $terminationFailureWatch.Elapsed.TotalSeconds -lt 1 -and
        $terminationFailure.StandardOutput -eq ''
    ) -Detail "Termination failure waited for stream completion: $($terminationFailureWatch.Elapsed.TotalSeconds)s"
    $secondTerminationFailure = & $module {
        Complete-BoundedProcessResult `
            -Process $null `
            -ProcessId $PID `
            -StandardOutputTask $null `
            -StandardErrorTask $null `
            -TimedOut $true `
            -TerminationConfirmed $false `
            -TerminationStatus 'FAIL'
    }
    Add-TestResult -Name 'termination-failure-classification-is-deterministic' -Passed (
        $secondTerminationFailure.FailureClassification -eq $terminationFailure.FailureClassification -and
        $secondTerminationFailure.ProcessId -eq $terminationFailure.ProcessId -and
        $secondTerminationFailure.TerminationStatus -eq $terminationFailure.TerminationStatus -and
        $secondTerminationFailure.StreamStatus -eq $terminationFailure.StreamStatus
    ) -Detail 'Repeated simulated termination failure changed classification.'

    Add-TestResult -Name 'repository-mutation-comparison-negative' -Passed (
        -not (Test-RepositoryStateUnchanged -BeforeStatus 'a' -AfterStatus 'b' -BeforeHashes 'c' -AfterHashes 'c')
    ) -Detail 'Changed repository status was accepted.'
    Add-TestResult -Name 'repository-mutation-comparison-positive' -Passed (
        Test-RepositoryStateUnchanged -BeforeStatus 'a' -AfterStatus 'a' -BeforeHashes 'c' -AfterHashes 'c'
    ) -Detail 'Unchanged repository state was rejected.'
}
catch {
    $failures.Add("Harness: $($_.Exception.Message)")
}
finally {
    $cleanupFailure = $null
    try {
        $resolvedTestRoot = [IO.Path]::GetFullPath($testRoot)
        $resolvedTempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
        if (-not $resolvedTestRoot.StartsWith($resolvedTempRoot, [StringComparison]::OrdinalIgnoreCase)) {
            throw "Test root is outside the system temp root: $resolvedTestRoot"
        }
        if (Test-Path -LiteralPath $resolvedTestRoot) {
            $item = Get-Item -LiteralPath $resolvedTestRoot -Force
            if ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) {
                throw "Test root is a reparse point: $resolvedTestRoot"
            }
            Remove-Item -LiteralPath $resolvedTestRoot -Recurse -Force
        }
    }
    catch {
        $cleanupFailure = $_.Exception.Message
        $failures.Add("Cleanup: $cleanupFailure")
    }

    $failureCount = $failures.Count
    [pscustomobject]@{
        Status = if ($failureCount -eq 0) { 'PASS' } else { 'FAIL' }
        TestCount = $testCount
        PassCount = $passCount
        Cleanup = if ($null -eq $cleanupFailure) { 'PASS' } else { 'FAIL' }
        Diagnostics = if ($failureCount -eq 0) { 'NONE' } else { $failures -join ' | ' }
        WarningCount = 0
        FailureCount = $failureCount
    } | Format-List
}

if ($failures.Count -gt 0) {
    exit 1
}
