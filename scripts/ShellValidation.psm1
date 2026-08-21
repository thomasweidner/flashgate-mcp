Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$script:RequiredPowerShellVersion = '7.6.5'
$script:RequiredGitBashPath = 'C:\Program Files\Git\bin\bash.exe'
$script:ProcessTerminationWaitMilliseconds = 5000
$script:StreamDrainWaitMilliseconds = 5000

function New-BoundedProcessResult {
    param(
        [Parameter(Mandatory)][int]$ExitCode,
        [Parameter(Mandatory)][ValidateRange(1, 2147483647)][int]$ProcessId,
        [Parameter(Mandatory)][bool]$TimedOut,
        [Parameter(Mandatory)][string]$TerminationStatus,
        [Parameter(Mandatory)][string]$StreamStatus,
        [Parameter(Mandatory)][string]$FailureClassification,
        [AllowEmptyString()][string]$StandardOutput = '',
        [AllowEmptyString()][string]$StandardError = ''
    )

    [pscustomobject]@{
        ExitCode = $ExitCode
        ProcessId = $ProcessId
        TimedOut = $TimedOut
        TerminationStatus = $TerminationStatus
        StreamStatus = $StreamStatus
        FailureClassification = $FailureClassification
        StandardOutput = $StandardOutput
        StandardError = $StandardError
    }
}

function Complete-BoundedProcessResult {
    param(
        [AllowNull()][Diagnostics.Process]$Process,
        [Parameter(Mandatory)][ValidateRange(1, 2147483647)][int]$ProcessId,
        [AllowNull()][Threading.Tasks.Task[string]]$StandardOutputTask,
        [AllowNull()][Threading.Tasks.Task[string]]$StandardErrorTask,
        [Parameter(Mandatory)][bool]$TimedOut,
        [Parameter(Mandatory)][bool]$TerminationConfirmed,
        [Parameter(Mandatory)][string]$TerminationStatus,
        [ValidateRange(1, 30000)][int]$StreamWaitMilliseconds = $script:StreamDrainWaitMilliseconds
    )

    if (-not $TerminationConfirmed) {
        return New-BoundedProcessResult `
            -ExitCode 124 `
            -ProcessId $ProcessId `
            -TimedOut $TimedOut `
            -TerminationStatus 'FAIL' `
            -StreamStatus 'NOT_READ' `
            -FailureClassification 'ProcessTerminationFailed' `
            -StandardError 'Process did not exit within the bounded termination window.'
    }

    if ($null -eq $Process -or $null -eq $StandardOutputTask -or $null -eq $StandardErrorTask) {
        return New-BoundedProcessResult `
            -ExitCode 124 `
            -ProcessId $ProcessId `
            -TimedOut $TimedOut `
            -TerminationStatus $TerminationStatus `
            -StreamStatus 'FAIL' `
            -FailureClassification 'StreamReadFailed' `
            -StandardError 'Required process or stream state was unavailable.'
    }

    try {
        $streamTasks = [Threading.Tasks.Task[]]@($StandardOutputTask, $StandardErrorTask)
        $streamsCompleted = [Threading.Tasks.Task]::WaitAll($streamTasks, $StreamWaitMilliseconds)
        if (-not $streamsCompleted) {
            return New-BoundedProcessResult `
                -ExitCode 124 `
                -ProcessId $ProcessId `
                -TimedOut $TimedOut `
                -TerminationStatus $TerminationStatus `
                -StreamStatus 'FAIL' `
                -FailureClassification 'StreamDrainTimedOut' `
                -StandardError 'Process streams did not complete within the bounded drain window.'
        }

        return New-BoundedProcessResult `
            -ExitCode $(if ($TimedOut) { 124 } else { $Process.ExitCode }) `
            -ProcessId $ProcessId `
            -TimedOut $TimedOut `
            -TerminationStatus $TerminationStatus `
            -StreamStatus 'PASS' `
            -FailureClassification $(if ($TimedOut) { 'ProcessTimedOut' } else { 'None' }) `
            -StandardOutput $StandardOutputTask.GetAwaiter().GetResult() `
            -StandardError $StandardErrorTask.GetAwaiter().GetResult()
    }
    catch {
        return New-BoundedProcessResult `
            -ExitCode 124 `
            -ProcessId $ProcessId `
            -TimedOut $TimedOut `
            -TerminationStatus $TerminationStatus `
            -StreamStatus 'FAIL' `
            -FailureClassification 'StreamReadFailed' `
            -StandardError "Bounded stream read failed: $($_.Exception.Message)"
    }
}

function Invoke-BoundedProcess {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][string]$FilePath,
        [AllowEmptyCollection()][string[]]$ArgumentList = @(),
        [ValidateRange(1, 300)][int]$TimeoutSeconds = 30
    )

    $process = $null
    $processId = 0
    $stdoutTask = $null
    $stderrTask = $null
    $timedOut = $false
    $terminationConfirmed = $false
    $terminationStatus = 'NOT_REQUIRED'
    try {
        $startInfo = [Diagnostics.ProcessStartInfo]::new()
        $startInfo.FileName = $FilePath
        $startInfo.UseShellExecute = $false
        $startInfo.RedirectStandardOutput = $true
        $startInfo.RedirectStandardError = $true
        $startInfo.CreateNoWindow = $true
        foreach ($argument in $ArgumentList) {
            [void]$startInfo.ArgumentList.Add($argument)
        }

        $process = [Diagnostics.Process]::new()
        $process.StartInfo = $startInfo
        [void]$process.Start()
        $processId = $process.Id
        $stdoutTask = $process.StandardOutput.ReadToEndAsync()
        $stderrTask = $process.StandardError.ReadToEndAsync()
        $terminationConfirmed = $process.WaitForExit($TimeoutSeconds * 1000)
        if (-not $terminationConfirmed) {
            $timedOut = $true
            $terminationStatus = 'FAIL'
            try {
                $process.Kill($true)
            }
            catch {
                # The bounded completion result remains fail-closed.
            }
            $terminationConfirmed = $process.WaitForExit($script:ProcessTerminationWaitMilliseconds)
            if ($terminationConfirmed) {
                $terminationStatus = 'PASS'
            }
        }

        return Complete-BoundedProcessResult `
            -Process $process `
            -ProcessId $processId `
            -StandardOutputTask $stdoutTask `
            -StandardErrorTask $stderrTask `
            -TimedOut $timedOut `
            -TerminationConfirmed $terminationConfirmed `
            -TerminationStatus $terminationStatus
    }
    finally {
        if ($null -ne $process) {
            if ($null -ne $process.StandardOutput) {
                $process.StandardOutput.Dispose()
            }
            if ($null -ne $process.StandardError) {
                $process.StandardError.Dispose()
            }
            $process.Dispose()
        }
    }
}

function Test-RequiredPowerShellVersion {
    [CmdletBinding()]
    param([Parameter(Mandatory)][string]$ActualVersion)

    return [string]::Equals(
        $ActualVersion,
        $script:RequiredPowerShellVersion,
        [StringComparison]::Ordinal
    )
}

function Test-RepositoryStateUnchanged {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][AllowEmptyString()][string]$BeforeStatus,
        [Parameter(Mandatory)][AllowEmptyString()][string]$AfterStatus,
        [Parameter(Mandatory)][AllowEmptyString()][string]$BeforeHashes,
        [Parameter(Mandatory)][AllowEmptyString()][string]$AfterHashes
    )

    return (
        [string]::Equals($BeforeStatus, $AfterStatus, [StringComparison]::Ordinal) -and
        [string]::Equals($BeforeHashes, $AfterHashes, [StringComparison]::Ordinal)
    )
}

function Test-CanonicalRepositoryPath {
    param([Parameter(Mandatory)][string]$Path)

    if ([string]::IsNullOrWhiteSpace($Path) -or
        $Path.Contains('\') -or
        $Path.StartsWith('/', [StringComparison]::Ordinal) -or
        $Path.Contains('//', [StringComparison]::Ordinal) -or
        $Path -match '^[A-Za-z]:' -or
        $Path -match '[\x00-\x1f\x7f]') {
        return $false
    }
    $segments = @($Path.Split('/'))
    return @($segments | Where-Object { $_ -in @('', '.', '..') }).Count -eq 0
}

function Get-OrdinalSortedStrings {
    param([AllowEmptyCollection()][string[]]$Values = @())

    $result = [string[]]@($Values)
    [Array]::Sort($result, [StringComparer]::Ordinal)
    return $result
}

function Invoke-RequiredGit {
    param(
        [Parameter(Mandatory)][string]$GitPath,
        [Parameter(Mandatory)][string]$RepositoryRoot,
        [Parameter(Mandatory)][string[]]$Arguments
    )

    $result = Invoke-BoundedProcess -FilePath $GitPath -ArgumentList (@('-C', $RepositoryRoot) + $Arguments)
    if ($result.FailureClassification -notin @('None', 'ProcessTimedOut')) {
        throw ('Git command failed closed ({0}): {1}; {2}' -f $result.FailureClassification, [string]::Join(' ', [string[]]$Arguments), $result.StandardError)
    }
    if ($result.TimedOut) {
        throw ('Git command timed out: {0}' -f [string]::Join(' ', [string[]]$Arguments))
    }
    if ($result.ExitCode -ne 0) {
        $detail = $result.StandardError.Trim()
        throw ('Git command failed ({0}): {1}; {2}' -f $result.ExitCode, [string]::Join(' ', [string[]]$Arguments), $detail)
    }
    return $result.StandardOutput
}

function Get-RepositoryPaths {
    param(
        [Parameter(Mandatory)][string]$GitPath,
        [Parameter(Mandatory)][string]$RepositoryRoot
    )

    $raw = Invoke-RequiredGit -GitPath $GitPath -RepositoryRoot $RepositoryRoot -Arguments @(
        '-c', 'core.quotePath=false', 'ls-files', '--cached', '--others', '--exclude-standard', '-z'
    )
    $paths = @($raw.Split([char]0, [StringSplitOptions]::RemoveEmptyEntries))
    foreach ($path in $paths) {
        if (-not (Test-CanonicalRepositoryPath -Path $path)) {
            throw "Git returned a non-canonical tracked path: $path"
        }
    }
    return Get-OrdinalSortedStrings -Values $paths
}

function Get-ShellInventory {
    param(
        [Parameter(Mandatory)][string]$RepositoryRoot,
        [Parameter(Mandatory)][string[]]$TrackedPaths
    )

    $selected = [System.Collections.Generic.List[string]]::new()
    foreach ($path in $TrackedPaths) {
        $extension = [IO.Path]::GetExtension($path).ToLowerInvariant()
        if ($extension -in @('.ps1', '.psm1', '.sh')) {
            $selected.Add($path)
            continue
        }

        $fullPath = Join-Path $RepositoryRoot ($path.Replace('/', '\'))
        if (-not (Test-Path -LiteralPath $fullPath -PathType Leaf)) {
            continue
        }
        $firstLine = Get-Content -LiteralPath $fullPath -TotalCount 1 -ErrorAction Stop
        if ($firstLine -match '^#!.*\b(?:bash|pwsh|powershell)(?:\s|$)') {
            $selected.Add($path)
        }
    }
    return Get-OrdinalSortedStrings -Values @($selected)
}

function Get-FileValidationFacts {
    param(
        [Parameter(Mandatory)][string]$FullPath,
        [Parameter(Mandatory)][string]$RelativePath
    )

    $item = Get-Item -LiteralPath $FullPath -Force -ErrorAction Stop
    if (-not $item.PSIsContainer -and ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
        throw "Tracked shell entry point is a reparse point: $RelativePath"
    }
    if ($item.PSIsContainer) {
        throw "Tracked shell entry point is not a regular file: $RelativePath"
    }

    $bytes = [IO.File]::ReadAllBytes($FullPath)
    $strictUtf8 = [Text.UTF8Encoding]::new($false, $true)
    try {
        $text = $strictUtf8.GetString($bytes)
    }
    catch {
        throw "Tracked shell entry point is not strict UTF-8: $RelativePath"
    }
    if ($bytes -contains 0) {
        throw "Tracked shell entry point contains a NUL byte: $RelativePath"
    }

    $crlfCount = [regex]::Matches($text, "`r`n").Count
    $lfCount = [regex]::Matches($text, '(?<!\r)\n').Count
    $crCount = [regex]::Matches($text, '\r(?!\n)').Count
    [pscustomobject]@{
        Text = $text
        Sha256 = (Get-FileHash -LiteralPath $FullPath -Algorithm SHA256).Hash.ToLowerInvariant()
        CrlfCount = $crlfCount
        LfCount = $lfCount
        CrCount = $crCount
        MixedLineEndings = (($crlfCount -gt 0 -and $lfCount -gt 0) -or $crCount -gt 0)
        HasFinalNewline = $text.Length -gt 0 -and $text.EndsWith("`n", [StringComparison]::Ordinal)
    }
}

function Get-ShellHashSnapshot {
    param(
        [Parameter(Mandatory)][string]$RepositoryRoot,
        [Parameter(Mandatory)][string[]]$Inventory
    )

    $lines = foreach ($path in $Inventory) {
        $fullPath = Join-Path $RepositoryRoot ($path.Replace('/', '\'))
        if (-not (Test-Path -LiteralPath $fullPath -PathType Leaf)) {
            "$path=<missing>"
        }
        else {
            $hash = (Get-FileHash -LiteralPath $fullPath -Algorithm SHA256).Hash.ToLowerInvariant()
            "$path=$hash"
        }
    }
    return $lines -join "`n"
}

function Invoke-FlashGateShellValidation {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][string]$RepositoryRoot,
        [string]$GitBashPath = $script:RequiredGitBashPath
    )

    $failures = [System.Collections.Generic.List[object]]::new()
    $inventory = @()
    $powerShellCount = 0
    $bashCount = 0
    $gitPath = $null
    $resolvedRoot = $null
    $beforeStatus = $null
    $beforeHashes = $null
    $repositoryMutationDetected = $false

    try {
        $actualPowerShellVersion = $PSVersionTable.PSVersion.ToString()
        if (-not (Test-RequiredPowerShellVersion -ActualVersion $actualPowerShellVersion)) {
            throw "PowerShell $($script:RequiredPowerShellVersion) is required; actual=$actualPowerShellVersion"
        }

        $resolvedRoot = [IO.Path]::GetFullPath($RepositoryRoot)
        if (-not (Test-Path -LiteralPath $resolvedRoot -PathType Container)) {
            throw "Repository root does not exist: $resolvedRoot"
        }
        $rootItem = Get-Item -LiteralPath $resolvedRoot -Force
        if ($rootItem.Attributes -band [IO.FileAttributes]::ReparsePoint) {
            throw "Repository root must not be a reparse point: $resolvedRoot"
        }

        $resolvedGitBash = [IO.Path]::GetFullPath($GitBashPath)
        $requiredGitBash = [IO.Path]::GetFullPath($script:RequiredGitBashPath)
        if (-not [string]::Equals($resolvedGitBash, $requiredGitBash, [StringComparison]::OrdinalIgnoreCase)) {
            throw "Git Bash path must be exactly $requiredGitBash"
        }
        if (-not (Test-Path -LiteralPath $resolvedGitBash -PathType Leaf)) {
            throw "Required Git Bash executable does not exist: $resolvedGitBash"
        }

        $gitCommand = Get-Command git.exe -CommandType Application -ErrorAction Stop |
            Select-Object -First 1
        $gitPath = [IO.Path]::GetFullPath($gitCommand.Source)
        $topLevel = (Invoke-RequiredGit -GitPath $gitPath -RepositoryRoot $resolvedRoot -Arguments @(
            'rev-parse', '--show-toplevel'
        )).Trim()
        $resolvedTopLevel = [IO.Path]::GetFullPath($topLevel)
        if (-not [string]::Equals($resolvedTopLevel, $resolvedRoot, [StringComparison]::OrdinalIgnoreCase)) {
            throw "RepositoryRoot is not the actual Git top-level: root=$resolvedRoot; git=$resolvedTopLevel"
        }

        $beforeStatus = Invoke-RequiredGit -GitPath $gitPath -RepositoryRoot $resolvedRoot -Arguments @(
            'status', '--porcelain=v1', '-z', '--untracked-files=all'
        )
        $repositoryPaths = Get-RepositoryPaths -GitPath $gitPath -RepositoryRoot $resolvedRoot
        $inventory = @(Get-ShellInventory -RepositoryRoot $resolvedRoot -TrackedPaths $repositoryPaths)
        $powerShellCount = @($inventory | Where-Object { [IO.Path]::GetExtension($_) -in @('.ps1', '.psm1') }).Count
        $bashCount = @($inventory | Where-Object { [IO.Path]::GetExtension($_) -eq '.sh' }).Count
        if ($inventory.Count -eq 0 -or $powerShellCount -eq 0 -or $bashCount -eq 0) {
            throw ('Shell inventory is incomplete: total={0}; PowerShell={1}; Bash={2}' -f $inventory.Count, $powerShellCount, $bashCount)
        }
        $beforeHashes = Get-ShellHashSnapshot -RepositoryRoot $resolvedRoot -Inventory $inventory

        foreach ($path in $inventory) {
            $fullPath = Join-Path $resolvedRoot ($path.Replace('/', '\'))
            if (-not (Test-Path -LiteralPath $fullPath -PathType Leaf)) {
                $failures.Add([pscustomobject]@{ Code = 'MissingFile'; Path = $path; Line = 0; Column = 0; Message = 'Tracked shell entry point does not exist.' })
                continue
            }

            try {
                $facts = Get-FileValidationFacts -FullPath $fullPath -RelativePath $path
                if ($facts.MixedLineEndings) {
                    throw "Mixed or lone-CR line endings are not allowed: $path"
                }
                if (-not $facts.HasFinalNewline) {
                    throw "Shell entry point must end with a newline: $path"
                }

                $extension = [IO.Path]::GetExtension($path).ToLowerInvariant()
                if ($extension -in @('.ps1', '.psm1')) {
                    $tokens = $null
                    $parserErrors = $null
                    $null = [System.Management.Automation.Language.Parser]::ParseFile(
                        $fullPath,
                        [ref]$tokens,
                        [ref]$parserErrors
                    )
                    foreach ($parserError in @($parserErrors)) {
                        $failures.Add([pscustomobject]@{
                            Code = 'PowerShellParser'
                            Path = $path
                            Line = $parserError.Extent.StartLineNumber
                            Column = $parserError.Extent.StartColumnNumber
                            Message = $parserError.Message
                        })
                    }
                }
                elseif ($extension -eq '.sh') {
                    if ($facts.CrlfCount -gt 0 -or $facts.CrCount -gt 0) {
                        throw "Bash entry point must use LF line endings: $path"
                    }
                    $firstLine = ($facts.Text -split "`n", 2)[0]
                    if ($firstLine -notmatch '^#!/usr/bin/env bash$' -and $firstLine -notmatch '^#!/bin/bash$') {
                        throw "Bash entry point has no supported Bash shebang: $path"
                    }
                    $syntax = Invoke-BoundedProcess -FilePath $resolvedGitBash -ArgumentList @('-n', '--', $fullPath)
                    if ($syntax.FailureClassification -notin @('None', 'ProcessTimedOut')) {
                        throw ('Git Bash syntax validation failed closed ({0}): {1}; {2}' -f $syntax.FailureClassification, $path, $syntax.StandardError)
                    }
                    if ($syntax.TimedOut) {
                        throw "Git Bash syntax validation timed out: $path"
                    }
                    if ($syntax.ExitCode -ne 0) {
                        $detail = ($syntax.StandardError + $syntax.StandardOutput).Trim()
                        throw ('Git Bash syntax validation failed ({0}): {1}; {2}' -f $syntax.ExitCode, $path, $detail)
                    }
                }
                else {
                    throw "Shell shebang entry point uses an unsupported file extension: $path"
                }
            }
            catch {
                $failures.Add([pscustomobject]@{
                    Code = 'FileValidation'
                    Path = $path
                    Line = 0
                    Column = 0
                    Message = $_.Exception.Message
                })
            }
        }

        $afterStatus = Invoke-RequiredGit -GitPath $gitPath -RepositoryRoot $resolvedRoot -Arguments @(
            'status', '--porcelain=v1', '-z', '--untracked-files=all'
        )
        $afterHashes = Get-ShellHashSnapshot -RepositoryRoot $resolvedRoot -Inventory $inventory
        $repositoryMutationDetected = -not (Test-RepositoryStateUnchanged `
            -BeforeStatus $beforeStatus `
            -AfterStatus $afterStatus `
            -BeforeHashes $beforeHashes `
            -AfterHashes $afterHashes)
        if ($repositoryMutationDetected) {
            $failures.Add([pscustomobject]@{
                Code = 'RepositoryMutation'
                Path = ''
                Line = 0
                Column = 0
                Message = 'Repository state or tracked shell bytes changed during validation.'
            })
        }
    }
    catch {
        $failures.Add([pscustomobject]@{
            Code = 'Preflight'
            Path = ''
            Line = 0
            Column = 0
            Message = $_.Exception.Message
        })
    }

    [pscustomobject]@{
        Status = if ($failures.Count -eq 0) { 'PASS' } else { 'FAIL' }
        RepositoryRoot = $resolvedRoot
        PowerShellPath = (Get-Process -Id $PID).Path
        PowerShellVersion = $PSVersionTable.PSVersion.ToString()
        GitPath = $gitPath
        GitBashPath = if ($GitBashPath) { [IO.Path]::GetFullPath($GitBashPath) } else { $null }
        InventoryCount = $inventory.Count
        PowerShellScriptCount = $powerShellCount
        BashScriptCount = $bashCount
        InventoryPaths = @($inventory)
        ParserFailureCount = @($failures | Where-Object Code -eq 'PowerShellParser').Count
        BashSyntaxFailureCount = @($failures | Where-Object { $_.Message -like 'Git Bash syntax validation failed*' }).Count
        RepositoryMutationDetected = $repositoryMutationDetected
        Diagnostics = @($failures)
        WarningCount = 0
        FailureCount = $failures.Count
    }
}

Export-ModuleMember -Function @(
    'Invoke-BoundedProcess',
    'Invoke-FlashGateShellValidation',
    'Test-RepositoryStateUnchanged',
    'Test-RequiredPowerShellVersion'
)
