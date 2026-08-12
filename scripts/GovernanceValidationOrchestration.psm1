#requires -Version 7.6

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$script:TerminalStatuses = @('PASS', 'FAIL', 'SKIPPED', 'BLOCKED', 'CANCELLED', 'PENDING', 'NOT_RUN')
$script:CheapGateOrder = @(
    'parser-syntax',
    'text-policy',
    'git-diff-check',
    'external-input-binding',
    'toolchain-context-binding',
    'source-worktree-selector-binding'
)

function Get-GovernanceLowerSha256 {
    [CmdletBinding()]
    param([Parameter(Mandatory)][string]$LiteralPath)

    return (Get-FileHash -LiteralPath $LiteralPath -Algorithm SHA256).Hash.ToLowerInvariant()
}

function Get-GovernanceObjectPropertyNames {
    param([Parameter(Mandatory)][object]$Value)

    return @($Value.PSObject.Properties | ForEach-Object Name)
}

function Assert-GovernanceJsonElementUniqueProperties {
    param(
        [Parameter(Mandatory)][System.Text.Json.JsonElement]$Element,
        [string]$Location = '$'
    )

    if ($Element.ValueKind -eq [System.Text.Json.JsonValueKind]::Object) {
        $names = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::Ordinal)
        foreach ($property in $Element.EnumerateObject()) {
            if (-not $names.Add($property.Name)) {
                throw "Duplicate JSON property at ${Location}: $($property.Name)"
            }
            Assert-GovernanceJsonElementUniqueProperties `
                -Element $property.Value `
                -Location ($Location + '.' + $property.Name)
        }
    }
    elseif ($Element.ValueKind -eq [System.Text.Json.JsonValueKind]::Array) {
        $index = 0
        foreach ($item in $Element.EnumerateArray()) {
            Assert-GovernanceJsonElementUniqueProperties -Element $item -Location "${Location}[$index]"
            $index++
        }
    }
}

function Read-GovernanceJsonContract {
    [CmdletBinding(DefaultParameterSetName = 'Path')]
    param(
        [Parameter(Mandatory, ParameterSetName = 'Path')][string]$LiteralPath,
        [Parameter(Mandatory, ParameterSetName = 'Bytes')][byte[]]$Bytes,
        [Parameter(Mandatory, ParameterSetName = 'Bytes')][string]$Label,
        [string]$SchemaPath,
        [int]$ExpectedSchemaVersion = 1,
        [string]$ExpectedProfile
    )

    $resolvedPath = if ($PSCmdlet.ParameterSetName -ceq 'Path') {
        [System.IO.Path]::GetFullPath($LiteralPath)
    }
    else {
        $Label
    }
    $resolvedSchemaPath = if ([string]::IsNullOrWhiteSpace($SchemaPath)) {
        $null
    }
    else {
        [System.IO.Path]::GetFullPath($SchemaPath)
    }
    if ($PSCmdlet.ParameterSetName -ceq 'Path' -and -not (Test-Path -LiteralPath $resolvedPath -PathType Leaf)) {
        throw "JSON contract does not exist: $resolvedPath"
    }
    if ($null -ne $resolvedSchemaPath -and -not (Test-Path -LiteralPath $resolvedSchemaPath -PathType Leaf)) {
        throw "JSON schema does not exist: $resolvedSchemaPath"
    }

    if ($PSCmdlet.ParameterSetName -ceq 'Path') {
        $Bytes = [System.IO.File]::ReadAllBytes($resolvedPath)
    }
    if ($bytes.Length -ge 3 -and $bytes[0] -eq 0xEF -and $bytes[1] -eq 0xBB -and $bytes[2] -eq 0xBF) {
        throw "UTF-8 BOM is not allowed: $resolvedPath"
    }
    $strictUtf8 = [System.Text.UTF8Encoding]::new($false, $true)
    $text = $strictUtf8.GetString($bytes)
    if ($text.Contains([char]0xFFFD)) {
        throw "Stored U+FFFD is not allowed: $resolvedPath"
    }

    $document = [System.Text.Json.JsonDocument]::Parse(
        $text,
        [System.Text.Json.JsonDocumentOptions]@{
            AllowTrailingCommas = $false
            CommentHandling = [System.Text.Json.JsonCommentHandling]::Disallow
        }
    )
    try {
        Assert-GovernanceJsonElementUniqueProperties -Element $document.RootElement
    }
    finally {
        $document.Dispose()
    }

    if ($null -ne $resolvedSchemaPath) {
        $schemaErrors = @()
        $schemaValid = $text | Test-Json `
            -SchemaFile $resolvedSchemaPath `
            -ErrorVariable schemaErrors `
            -ErrorAction SilentlyContinue
        if (-not $schemaValid) {
            throw "JSON schema validation failed: $resolvedPath; $(@($schemaErrors | ForEach-Object ToString) -join ' | ')"
        }
    }

    $value = $text | ConvertFrom-Json -Depth 100 -DateKind String
    if ('schemaVersion' -notin (Get-GovernanceObjectPropertyNames -Value $value) -or
        [int]$value.schemaVersion -ne $ExpectedSchemaVersion) {
        throw "Unexpected schema version in $resolvedPath"
    }
    if (-not [string]::IsNullOrWhiteSpace($ExpectedProfile)) {
        if ('profile' -notin (Get-GovernanceObjectPropertyNames -Value $value) -or
            [string]$value.profile -cne $ExpectedProfile) {
            throw "Unexpected profile in $resolvedPath"
        }
    }
    return $value
}

function Read-GovernanceTypedResult {
    [CmdletBinding(DefaultParameterSetName = 'Path')]
    param(
        [Parameter(Mandatory, ParameterSetName = 'Path')][string]$LiteralPath,
        [Parameter(Mandatory, ParameterSetName = 'Bytes')][byte[]]$Bytes,
        [Parameter(Mandatory, ParameterSetName = 'Bytes')][string]$Label,
        [Parameter(Mandatory)][string]$SchemaPath,
        [int]$ExpectedSchemaVersion = 1,
        [string]$ExpectedProfile,
        [string[]]$AllowedTerminalStatus = $script:TerminalStatuses
    )

    $readArguments = @{
        SchemaPath = $SchemaPath
        ExpectedSchemaVersion = $ExpectedSchemaVersion
        ExpectedProfile = $ExpectedProfile
    }
    if ($PSCmdlet.ParameterSetName -ceq 'Path') {
        $readArguments.LiteralPath = $LiteralPath
        $resultLabel = $LiteralPath
    }
    else {
        $readArguments.Bytes = $Bytes
        $readArguments.Label = $Label
        $resultLabel = $Label
    }
    $value = Read-GovernanceJsonContract @readArguments
    $names = Get-GovernanceObjectPropertyNames -Value $value
    $statusProperty = if ('status' -in $names) { 'status' } elseif ('result' -in $names) { 'result' } else { $null }
    if ($null -eq $statusProperty -or [string]$value.$statusProperty -notin $AllowedTerminalStatus) {
        throw "Typed result does not contain an allowed terminal status: $resultLabel"
    }

    foreach ($counterName in @(
            'validationExecutionCount',
            'infrastructureOrInvocationFailureCount',
            'fullMatrixRunCount',
            'packageWriteAttemptCount',
            'generatedTaskControllerFileCount',
            'generatedTaskControllerLineCount',
            'readOnlyProbeCount',
            'failureCount'
        )) {
        if ($counterName -in $names -and [int64]$value.$counterName -lt 0) {
            throw "Typed result contains a negative counter: $counterName"
        }
    }
    if ('fullMatrixRunCount' -in $names -and [int]$value.fullMatrixRunCount -gt 1) {
        throw 'A typed result may record at most one full matrix run.'
    }
    if ('packageWriteAttemptCount' -in $names -and [int]$value.packageWriteAttemptCount -gt 1) {
        throw 'A typed result may record at most one package write attempt.'
    }
    if ('status' -in $names -and [string]$value.status -ceq 'BLOCKED' -and
        'failureCount' -in $names -and [int]$value.failureCount -ne 0) {
        throw 'BLOCKED must not increment failureCount.'
    }
    return $value
}

function Test-GovernanceCanonicalRepositoryPath {
    param([Parameter(Mandatory)][string]$Path)

    if ([string]::IsNullOrWhiteSpace($Path) -or
        $Path.Contains('\') -or
        $Path.StartsWith('/', [System.StringComparison]::Ordinal) -or
        $Path.Contains('//', [System.StringComparison]::Ordinal) -or
        $Path -match '^[A-Za-z]:' -or
        $Path -match '[\x00-\x1f\x7f]') {
        return $false
    }
    $segments = @($Path.Split('/'))
    return (
        @($segments | Where-Object { $_ -in @('', '.', '..') }).Count -eq 0 -and
        [string]::Equals(($segments -join '/'), $Path, [System.StringComparison]::Ordinal) -and
        [string]::Equals($Path.Normalize(), $Path, [System.StringComparison]::Ordinal)
    )
}

function Invoke-GovernanceGitText {
    param(
        [Parameter(Mandatory)][string]$RepositoryRoot,
        [Parameter(Mandatory)][string[]]$Argument,
        [int[]]$AllowedExitCode = @(0)
    )

    $output = @(& git -C $RepositoryRoot @Argument 2>&1)
    $exitCode = $LASTEXITCODE
    if ($exitCode -notin $AllowedExitCode) {
        throw "Git command failed ($exitCode): git $($Argument -join ' '); $($output -join ' | ')"
    }
    return [pscustomobject]@{
        ExitCode = $exitCode
        Text = ($output -join "`n")
    }
}

function Invoke-GovernanceGitBytes {
    param(
        [Parameter(Mandatory)][string]$RepositoryRoot,
        [Parameter(Mandatory)][string[]]$Argument,
        [int[]]$AllowedExitCode = @(0)
    )

    $gitPath = [System.IO.Path]::GetFullPath(
        (Get-Command git -CommandType Application -ErrorAction Stop | Select-Object -First 1).Source
    )
    $startInfo = [System.Diagnostics.ProcessStartInfo]::new()
    $startInfo.FileName = $gitPath
    $startInfo.UseShellExecute = $false
    $startInfo.CreateNoWindow = $true
    $startInfo.RedirectStandardOutput = $true
    $startInfo.RedirectStandardError = $true
    [void]$startInfo.ArgumentList.Add('-C')
    [void]$startInfo.ArgumentList.Add([System.IO.Path]::GetFullPath($RepositoryRoot))
    foreach ($argumentValue in $Argument) {
        [void]$startInfo.ArgumentList.Add($argumentValue)
    }
    $startInfo.Environment['GIT_OPTIONAL_LOCKS'] = '0'

    $process = [System.Diagnostics.Process]::new()
    $process.StartInfo = $startInfo
    $standardOutput = [System.IO.MemoryStream]::new()
    try {
        if (-not $process.Start()) {
            throw 'Git process did not start.'
        }
        $standardOutputCopy = $process.StandardOutput.BaseStream.CopyToAsync($standardOutput)
        $standardErrorRead = $process.StandardError.ReadToEndAsync()
        $process.WaitForExit()
        [void]$standardOutputCopy.GetAwaiter().GetResult()
        $standardError = $standardErrorRead.GetAwaiter().GetResult()
        if ($process.ExitCode -notin $AllowedExitCode) {
            throw "Git command failed ($($process.ExitCode)): git $($Argument -join ' '); $standardError"
        }
        return [pscustomobject]@{
            ExitCode = $process.ExitCode
            Bytes = $standardOutput.ToArray()
            StandardError = $standardError
        }
    }
    finally {
        $standardOutput.Dispose()
        $process.Dispose()
    }
}

function Get-GovernanceByteSha256 {
    param([Parameter(Mandatory)][AllowEmptyCollection()][byte[]]$Bytes)

    return [Convert]::ToHexString(
        [System.Security.Cryptography.SHA256]::HashData($Bytes)
    ).ToLowerInvariant()
}

function Get-GovernanceWorkingTreeStatusBinding {
    [CmdletBinding()]
    param([Parameter(Mandatory)][string]$RepositoryRoot)

    $status = Invoke-GovernanceGitBytes `
        -RepositoryRoot $RepositoryRoot `
        -Argument @('status', '--porcelain=v2', '--untracked-files=all')
    return [pscustomobject]@{
        Sha256 = Get-GovernanceByteSha256 -Bytes ([byte[]]$status.Bytes)
        ByteLength = ([byte[]]$status.Bytes).Length
    }
}

function Test-GovernanceTextPolicy {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][string]$RepositoryRoot,
        [Parameter(Mandatory)][string[]]$RepositoryPath
    )

    $diagnostics = [System.Collections.Generic.List[string]]::new()
    $checkedCount = 0
    $canonicalPaths = @($RepositoryPath | Where-Object {
            Test-GovernanceCanonicalRepositoryPath -Path ([string]$_)
        })
    if ($canonicalPaths.Count -ne @($RepositoryPath).Count) {
        $diagnostics.Add('One or more text-policy paths are non-canonical.')
    }
    $attributeResult = Invoke-GovernanceGitText `
        -RepositoryRoot $RepositoryRoot `
        -Argument (@('check-attr', 'text', 'eol', '--') + $canonicalPaths)
    $textUnset = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::Ordinal)
    foreach ($attributeLine in @($attributeResult.Text -split "`n")) {
        if ($attributeLine -match '^(?<path>.+): text: unset$') {
            [void]$textUnset.Add($Matches.path)
        }
    }
    foreach ($relativePath in @($RepositoryPath)) {
        if (-not (Test-GovernanceCanonicalRepositoryPath -Path $relativePath)) {
            continue
        }
        $fullPath = Join-Path $RepositoryRoot ($relativePath.Replace('/', [System.IO.Path]::DirectorySeparatorChar))
        if (-not (Test-Path -LiteralPath $fullPath -PathType Leaf)) {
            continue
        }
        $item = Get-Item -LiteralPath $fullPath -Force
        if (($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
            $diagnostics.Add("Text-policy target is a reparse point: $relativePath")
            continue
        }
        if ($textUnset.Contains($relativePath)) {
            continue
        }
        $bytes = [System.IO.File]::ReadAllBytes($fullPath)
        if ($bytes -contains 0) {
            continue
        }
        try {
            $text = [System.Text.UTF8Encoding]::new($false, $true).GetString($bytes)
        }
        catch {
            $diagnostics.Add("Declared text is not strict UTF-8: $relativePath")
            continue
        }
        $checkedCount++
        if ($text.Contains("`r", [System.StringComparison]::Ordinal)) {
            $diagnostics.Add("Text must use LF: $relativePath")
        }
        if (-not $text.EndsWith("`n", [System.StringComparison]::Ordinal)) {
            $diagnostics.Add("Text must end with exactly one newline: $relativePath")
        }
        elseif ($text.EndsWith("`n`n", [System.StringComparison]::Ordinal)) {
            $diagnostics.Add("Text contains an extra EOF blank line: $relativePath")
        }
        $lineNumber = 0
        foreach ($line in @($text -split "`n")) {
            $lineNumber++
            if ($line -match '[ \t]+$') {
                $diagnostics.Add("Trailing whitespace: ${relativePath}:$lineNumber")
            }
        }
    }
    return [pscustomobject]@{
        Status = if ($diagnostics.Count -eq 0) { 'PASS' } else { 'FAIL' }
        CheckedFileCount = $checkedCount
        Diagnostics = @($diagnostics)
    }
}

function Test-GovernanceExternalInputBinding {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][string]$SourceRepositoryRoot,
        [AllowEmptyCollection()][object[]]$ExternalInput = @()
    )

    $diagnostics = [System.Collections.Generic.List[string]]::new()
    $bound = [System.Collections.Generic.List[object]]::new()
    $paths = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::OrdinalIgnoreCase)
    foreach ($entry in @($ExternalInput)) {
        $path = [System.IO.Path]::GetFullPath([string]$entry.path)
        if (-not $paths.Add($path)) {
            $diagnostics.Add("Duplicate input path: $path")
            continue
        }
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
            $diagnostics.Add("Required input is missing: $path")
            continue
        }
        $actualHash = Get-GovernanceLowerSha256 -LiteralPath $path
        if ($actualHash -cne [string]$entry.sha256) {
            $diagnostics.Add("Input hash mismatch: $path")
            continue
        }
        if ([string]$entry.kind -ceq 'IGNORED') {
            $ignore = Invoke-GovernanceGitText `
                -RepositoryRoot $SourceRepositoryRoot `
                -Argument @('check-ignore', '--', $path) `
                -AllowedExitCode @(0, 1)
            if ($ignore.ExitCode -ne 0) {
                $diagnostics.Add("IGNORED input is not ignored by Git: $path")
                continue
            }
        }
        [void]$bound.Add([pscustomobject]@{
            Id = [string]$entry.id
            Path = $path
            Kind = [string]$entry.kind
            Purpose = [string]$entry.purpose
            Sha256 = $actualHash
        })
    }
    return [pscustomobject]@{
        Status = if ($diagnostics.Count -eq 0) { 'PASS' } else { 'FAIL' }
        BoundInputs = @($bound)
        Diagnostics = @($diagnostics)
        ReadOnlyProbeCount = @($ExternalInput).Count
    }
}

function Test-GovernanceToolchainBinding {
    [CmdletBinding()]
    param([Parameter(Mandatory)][object]$Toolchain)

    $diagnostics = [System.Collections.Generic.List[string]]::new()
    $powerShellPath = [System.IO.Path]::GetFullPath([string]$Toolchain.powerShellPath)
    if (-not (Test-Path -LiteralPath $powerShellPath -PathType Leaf)) {
        $diagnostics.Add("Bound PowerShell does not exist: $powerShellPath")
    }
    else {
        if ((Get-GovernanceLowerSha256 -LiteralPath $powerShellPath) -cne [string]$Toolchain.powerShellSha256) {
            $diagnostics.Add('PowerShell hash mismatch.')
        }
        if ($PSVersionTable.PSVersion.ToString() -cne [string]$Toolchain.powerShellVersion) {
            $diagnostics.Add('PowerShell version mismatch.')
        }
        $currentProcessPath = [System.IO.Path]::GetFullPath((Get-Process -Id $PID).Path)
        if (-not [string]::Equals($currentProcessPath, $powerShellPath, [System.StringComparison]::OrdinalIgnoreCase)) {
            $diagnostics.Add('The current process is not the bound PowerShell executable.')
        }
    }

    $gitPath = [System.IO.Path]::GetFullPath([string]$Toolchain.gitPath)
    if (-not (Test-Path -LiteralPath $gitPath -PathType Leaf)) {
        $diagnostics.Add("Bound Git does not exist: $gitPath")
    }
    elseif ((Get-GovernanceLowerSha256 -LiteralPath $gitPath) -cne [string]$Toolchain.gitSha256) {
        $diagnostics.Add('Git hash mismatch.')
    }
    if ([string]$Toolchain.executionContext -notin @('CURRENT_PROCESS', 'KNOWN_NORMAL_USER', 'NATIVE_LINUX_LOGIN')) {
        $diagnostics.Add('Unknown execution context.')
    }
    return [pscustomobject]@{
        Status = if ($diagnostics.Count -eq 0) { 'PASS' } else { 'FAIL' }
        PowerShellPath = $powerShellPath
        GitPath = $gitPath
        ExecutionContext = [string]$Toolchain.executionContext
        Diagnostics = @($diagnostics)
        ReadOnlyProbeCount = 4
    }
}

function Test-GovernanceSourceBinding {
    [CmdletBinding()]
    param([Parameter(Mandatory)][object]$Request)

    $diagnostics = [System.Collections.Generic.List[string]]::new()
    $probeCount = 0
    $actualStatusHash = $null
    $protectedBindings = [System.Collections.Generic.List[object]]::new()
    $sourceRoot = [System.IO.Path]::GetFullPath([string]$Request.sourceRepositoryRoot).TrimEnd('\', '/')
    $worktreeRoot = [System.IO.Path]::GetFullPath([string]$Request.worktreeRoot).TrimEnd('\', '/')
    foreach ($root in @($sourceRoot, $worktreeRoot)) {
        if (-not (Test-Path -LiteralPath $root -PathType Container)) {
            $diagnostics.Add("Repository root does not exist: $root")
            continue
        }
        $item = Get-Item -LiteralPath $root -Force
        if (($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
            $diagnostics.Add("Repository root is a reparse point: $root")
        }
        $top = Invoke-GovernanceGitText -RepositoryRoot $root -Argument @('rev-parse', '--show-toplevel')
        $probeCount++
        if (-not [string]::Equals(
                [System.IO.Path]::GetFullPath($top.Text.Trim()).TrimEnd('\', '/'),
                $root,
                [System.StringComparison]::OrdinalIgnoreCase
            )) {
            $diagnostics.Add("Root is not the Git top-level: $root")
        }
    }
    if ($diagnostics.Count -eq 0) {
        $head = Invoke-GovernanceGitText -RepositoryRoot $worktreeRoot -Argument @('rev-parse', 'HEAD')
        $tree = Invoke-GovernanceGitText -RepositoryRoot $worktreeRoot -Argument @('rev-parse', 'HEAD^{tree}')
        $branch = Invoke-GovernanceGitText `
            -RepositoryRoot $worktreeRoot `
            -Argument @('symbolic-ref', '--quiet', '--short', 'HEAD') `
            -AllowedExitCode @(0, 1)
        $statusBinding = Get-GovernanceWorkingTreeStatusBinding -RepositoryRoot $worktreeRoot
        $worktrees = Invoke-GovernanceGitText -RepositoryRoot $sourceRoot -Argument @('worktree', 'list', '--porcelain')
        $probeCount += 5
        if ($head.Text.Trim() -cne [string]$Request.currentCommit) {
            $diagnostics.Add('Worktree HEAD mismatch.')
        }
        if ($tree.Text.Trim() -cne [string]$Request.expectedTree) {
            $diagnostics.Add('Worktree tree mismatch.')
        }
        $actualDetached = $branch.ExitCode -eq 1
        if ($actualDetached -ne [bool]$Request.detachedHead) {
            $diagnostics.Add('Detached-HEAD binding mismatch.')
        }
        if (-not $actualDetached -and $branch.Text.Trim() -cne [string]$Request.expectedBranch) {
            $diagnostics.Add('Branch binding mismatch.')
        }
        $normalizedWorktree = $worktreeRoot.Replace('\', '/')
        if (-not $worktrees.Text.Replace('\', '/').Contains("worktree $normalizedWorktree", [System.StringComparison]::OrdinalIgnoreCase)) {
            $diagnostics.Add('Bound worktree is absent from the source repository worktree inventory.')
        }
        $actualStatusHash = [string]$statusBinding.Sha256
        if ($actualStatusHash -cne [string]$Request.expectedStatusSha256) {
            $diagnostics.Add('Working-tree status hash mismatch.')
        }

        $scopePaths = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::Ordinal)
        $scopePathsWindows = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::OrdinalIgnoreCase)
        foreach ($scopePathValue in @($Request.scopePaths)) {
            $scopePath = [string]$scopePathValue
            if (-not $scopePaths.Add($scopePath)) {
                $diagnostics.Add("Duplicate scope path: $scopePath")
                continue
            }
            if (-not $scopePathsWindows.Add($scopePath)) {
                $diagnostics.Add("Case-colliding scope path: $scopePath")
            }
            if (-not (Test-GovernanceCanonicalRepositoryPath -Path $scopePath)) {
                $diagnostics.Add("Non-canonical scope path: $scopePath")
            }
        }

        $boundFilePaths = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::Ordinal)
        $boundFilePathsWindows = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::OrdinalIgnoreCase)
        foreach ($fileBinding in @($Request.fileHashes)) {
            $relativePath = [string]$fileBinding.path
            if (-not $boundFilePaths.Add($relativePath)) {
                $diagnostics.Add("Duplicate file-hash path: $relativePath")
                continue
            }
            if (-not $boundFilePathsWindows.Add($relativePath)) {
                $diagnostics.Add("Case-colliding file-hash path: $relativePath")
            }
            if (-not (Test-GovernanceCanonicalRepositoryPath -Path $relativePath)) {
                $diagnostics.Add("Non-canonical file-hash path: $relativePath")
                continue
            }
            $fullPath = Join-Path $worktreeRoot ($relativePath.Replace('/', [System.IO.Path]::DirectorySeparatorChar))
            if (-not (Test-Path -LiteralPath $fullPath -PathType Leaf)) {
                $diagnostics.Add("Hash-bound source file is missing: $relativePath")
                continue
            }
            if ((Get-GovernanceLowerSha256 -LiteralPath $fullPath) -cne [string]$fileBinding.sha256) {
                $diagnostics.Add("Hash-bound source file changed: $relativePath")
            }
            $probeCount++
        }
        foreach ($scopePath in $scopePaths) {
            if (-not $boundFilePaths.Contains($scopePath)) {
                $diagnostics.Add("Scope path has no file-hash binding: $scopePath")
            }
        }
        foreach ($filePath in $boundFilePaths) {
            if (-not $scopePaths.Contains($filePath)) {
                $diagnostics.Add("File-hash path is outside scope: $filePath")
            }
        }

        $protectedIds = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::Ordinal)
        $protectedRoots = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::OrdinalIgnoreCase)
        foreach ($protected in @($Request.protectedWorktrees)) {
            $protectedId = [string]$protected.id
            $protectedRoot = [System.IO.Path]::GetFullPath([string]$protected.root).TrimEnd('\', '/')
            $protectedStatus = 'PASS'
            $protectedActualStatusHash = $null
            if (-not $protectedIds.Add($protectedId)) {
                $diagnostics.Add("Duplicate protected-worktree ID: $protectedId")
                $protectedStatus = 'FAIL'
            }
            if (-not $protectedRoots.Add($protectedRoot)) {
                $diagnostics.Add("Duplicate protected-worktree root: $protectedRoot")
                $protectedStatus = 'FAIL'
            }
            try {
                if (-not (Test-Path -LiteralPath $protectedRoot -PathType Container)) {
                    throw 'Protected worktree does not exist.'
                }
                $protectedItem = Get-Item -LiteralPath $protectedRoot -Force
                if (($protectedItem.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
                    throw 'Protected worktree is a reparse point.'
                }
                $protectedTop = Invoke-GovernanceGitText -RepositoryRoot $protectedRoot -Argument @('rev-parse', '--show-toplevel')
                $protectedHead = Invoke-GovernanceGitText -RepositoryRoot $protectedRoot -Argument @('rev-parse', 'HEAD')
                $protectedTree = Invoke-GovernanceGitText -RepositoryRoot $protectedRoot -Argument @('rev-parse', 'HEAD^{tree}')
                $protectedBranch = Invoke-GovernanceGitText `
                    -RepositoryRoot $protectedRoot `
                    -Argument @('symbolic-ref', '--quiet', '--short', 'HEAD') `
                    -AllowedExitCode @(0, 1)
                $protectedStatusBinding = Get-GovernanceWorkingTreeStatusBinding -RepositoryRoot $protectedRoot
                $probeCount += 5
                $protectedActualStatusHash = [string]$protectedStatusBinding.Sha256
                $protectedDetached = $protectedBranch.ExitCode -eq 1
                if (-not [string]::Equals(
                        [System.IO.Path]::GetFullPath($protectedTop.Text.Trim()).TrimEnd('\', '/'),
                        $protectedRoot,
                        [System.StringComparison]::OrdinalIgnoreCase
                    ) -or
                    $protectedHead.Text.Trim() -cne [string]$protected.currentCommit -or
                    $protectedTree.Text.Trim() -cne [string]$protected.expectedTree -or
                    $protectedDetached -ne [bool]$protected.detachedHead -or
                    (-not $protectedDetached -and $protectedBranch.Text.Trim() -cne [string]$protected.expectedBranch) -or
                    $protectedActualStatusHash -cne [string]$protected.expectedStatusSha256) {
                    throw 'Protected worktree identity or status binding mismatch.'
                }
            }
            catch {
                $diagnostics.Add("Protected worktree binding failed (${protectedId}): $($_.Exception.Message)")
                $protectedStatus = 'FAIL'
            }
            $protectedBindings.Add([pscustomobject]@{
                Id = $protectedId
                Root = $protectedRoot
                ExpectedStatusSha256 = [string]$protected.expectedStatusSha256
                ActualStatusSha256 = $protectedActualStatusHash
                Status = $protectedStatus
            })
        }
    }
    else {
        $actualDetached = $null
    }
    $selectorText = $Request.selectors | ConvertTo-Json -Compress -Depth 20
    $selectorHash = [Convert]::ToHexString(
        [System.Security.Cryptography.SHA256]::HashData(
            [System.Text.UTF8Encoding]::new($false).GetBytes($selectorText)
        )
    ).ToLowerInvariant()
    return [pscustomobject]@{
        Status = if ($diagnostics.Count -eq 0) { 'PASS' } else { 'FAIL' }
        SourceRepositoryRoot = $sourceRoot
        WorktreeRoot = $worktreeRoot
        CurrentCommit = [string]$Request.currentCommit
        Tree = [string]$Request.expectedTree
        DetachedHead = $actualDetached
        ExpectedStatusSha256 = [string]$Request.expectedStatusSha256
        ActualStatusSha256 = $actualStatusHash
        ScopePathCount = @($Request.scopePaths).Count
        FileHashPathCount = @($Request.fileHashes).Count
        ProtectedWorktrees = @($protectedBindings)
        SelectorInventorySha256 = [string]$Request.selectors.inventorySha256
        SelectionSha256 = $selectorHash
        Diagnostics = @($diagnostics)
        ReadOnlyProbeCount = $probeCount
    }
}

function Get-GovernanceEvidenceDisposition {
    [CmdletBinding()]
    param(
        [AllowEmptyCollection()][object[]]$PriorEvidence = @(),
        [AllowEmptyCollection()][object[]]$CurrentDependency = @()
    )

    $dependencyMap = [System.Collections.Generic.Dictionary[string, string]]::new([System.StringComparer]::Ordinal)
    foreach ($dependency in @($CurrentDependency)) {
        if ($dependencyMap.ContainsKey([string]$dependency.id)) {
            throw "Duplicate current dependency ID: $($dependency.id)"
        }
        $dependencyMap.Add([string]$dependency.id, [string]$dependency.sha256)
    }
    $reused = [System.Collections.Generic.List[string]]::new()
    $invalidated = [System.Collections.Generic.List[object]]::new()
    foreach ($evidence in @($PriorEvidence)) {
        $reasons = [System.Collections.Generic.List[string]]::new()
        if ([string]$evidence.status -cne 'PASS') {
            $reasons.Add('PRIOR_STATUS_NOT_PASS')
        }
        if (-not (Test-Path -LiteralPath ([string]$evidence.path) -PathType Leaf)) {
            $reasons.Add('EVIDENCE_MISSING')
        }
        elseif ((Get-GovernanceLowerSha256 -LiteralPath ([string]$evidence.path)) -cne [string]$evidence.sha256) {
            $reasons.Add('EVIDENCE_HASH_CHANGED')
        }
        foreach ($dependency in @($evidence.dependencies)) {
            if (-not $dependencyMap.ContainsKey([string]$dependency.id)) {
                $reasons.Add("DEPENDENCY_MISSING:$($dependency.id)")
            }
            elseif ($dependencyMap[[string]$dependency.id] -cne [string]$dependency.sha256) {
                $reasons.Add("DEPENDENCY_CHANGED:$($dependency.id)")
            }
        }
        if ($reasons.Count -eq 0) {
            $reused.Add([string]$evidence.id)
        }
        else {
            $invalidated.Add([pscustomobject]@{ Id = [string]$evidence.id; Reasons = @($reasons) })
        }
    }
    return [pscustomobject]@{
        ReusedIds = @($reused)
        Invalidated = @($invalidated)
    }
}

function Merge-GovernanceOptimisticText {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][string]$BaseText,
        [Parameter(Mandatory)][string]$DesiredText,
        [Parameter(Mandatory)][string]$CurrentText
    )

    if ($CurrentText -ceq $BaseText) {
        return [pscustomobject]@{ Status = 'PASS'; Text = $DesiredText; ForeignDeltaPreserved = $false }
    }
    $baseLines = @($BaseText -split "`n")
    $desiredLines = @($DesiredText -split "`n")
    $currentLines = @($CurrentText -split "`n")
    if ($baseLines.Count -ne $desiredLines.Count -or $baseLines.Count -ne $currentLines.Count) {
        return [pscustomobject]@{ Status = 'BLOCKED'; Text = $null; ForeignDeltaPreserved = $false }
    }
    $merged = [string[]]@($currentLines)
    for ($index = 0; $index -lt $baseLines.Count; $index++) {
        $desiredChanged = $desiredLines[$index] -cne $baseLines[$index]
        $currentChanged = $currentLines[$index] -cne $baseLines[$index]
        if ($desiredChanged -and $currentChanged -and $desiredLines[$index] -cne $currentLines[$index]) {
            return [pscustomobject]@{ Status = 'BLOCKED'; Text = $null; ForeignDeltaPreserved = $false }
        }
        if ($desiredChanged) {
            $merged[$index] = $desiredLines[$index]
        }
    }
    return [pscustomobject]@{
        Status = 'PASS'
        Text = $merged -join "`n"
        ForeignDeltaPreserved = $true
    }
}

function Set-GovernanceExternalTextOptimistic {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][string]$LiteralPath,
        [Parameter(Mandatory)][string]$ExpectedSha256,
        [Parameter(Mandatory)][string]$BaseText,
        [Parameter(Mandatory)][string]$DesiredText
    )

    $resolvedPath = [System.IO.Path]::GetFullPath($LiteralPath)
    $temporaryPath = "$resolvedPath.$PID.$([guid]::NewGuid().ToString('N')).tmp"
    $status = 'FAIL'
    $mergedForeignDelta = $false
    $failureMessage = $null
    try {
        $currentHash = Get-GovernanceLowerSha256 -LiteralPath $resolvedPath
        $currentText = [System.IO.File]::ReadAllText($resolvedPath, [System.Text.UTF8Encoding]::new($false, $true))
        if ($currentHash -ceq $ExpectedSha256) {
            $nextText = $DesiredText
        }
        else {
            $merge = Merge-GovernanceOptimisticText -BaseText $BaseText -DesiredText $DesiredText -CurrentText $currentText
            if ($merge.Status -cne 'PASS') {
                throw 'Optimistic concurrency detected an overlapping external change.'
            }
            $nextText = [string]$merge.Text
            $mergedForeignDelta = [bool]$merge.ForeignDeltaPreserved
        }
        [System.IO.File]::WriteAllText($temporaryPath, $nextText, [System.Text.UTF8Encoding]::new($false))
        $lastMomentHash = Get-GovernanceLowerSha256 -LiteralPath $resolvedPath
        if ($lastMomentHash -cne $currentHash) {
            throw 'External document changed after the optimistic-concurrency gate.'
        }
        [System.IO.File]::Move($temporaryPath, $resolvedPath, $true)
        $status = 'PASS'
    }
    catch {
        $failureMessage = $_.Exception.Message
    }
    finally {
        if (Test-Path -LiteralPath $temporaryPath -PathType Leaf) {
            Remove-Item -LiteralPath $temporaryPath -Force
        }
    }
    return [pscustomobject]@{
        Status = $status
        Path = $resolvedPath
        Sha256 = if ($status -ceq 'PASS') { Get-GovernanceLowerSha256 -LiteralPath $resolvedPath } else { $null }
        ForeignDeltaPreserved = $mergedForeignDelta
        FailureMessage = $failureMessage
    }
}

function Invoke-GovernanceValidationOrchestration {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][object]$Request,
        [Parameter(Mandatory)][string]$RepositoryRoot
    )

    $catalogPath = Join-Path $RepositoryRoot 'Governance/change-trigger-catalog.json'
    $catalog = Read-GovernanceJsonContract -LiteralPath $catalogPath -ExpectedSchemaVersion 1
    $profile = @($catalog.orchestrationPolicy.profiles | Where-Object {
            [string]$_.id -ceq [string]$Request.profile
        })
    if ($profile.Count -ne 1) {
        throw "Unknown or duplicate orchestration profile: $($Request.profile)"
    }
    if ((@($catalog.orchestrationPolicy.cheapGateOrder) -join "`n") -cne ($script:CheapGateOrder -join "`n")) {
        throw 'Catalog cheap-gate order does not match the canonical orchestrator.'
    }

    $stageResults = [System.Collections.Generic.List[object]]::new()
    $readOnlyProbeCount = 0
    $failureCount = 0
    $infrastructureOrInvocationFailureCount = 0
    $blocked = $false
    $sourceResult = $null

    foreach ($gateId in $script:CheapGateOrder) {
        $gateStatus = 'PASS'
        $diagnostics = @()
        try {
            switch ($gateId) {
                'parser-syntax' {
                    foreach ($path in @($Request.scopePaths | Where-Object { $_ -match '\.ps(?:1|m1)$' })) {
                        $tokens = $null
                        $parserErrors = $null
                        $fullPath = Join-Path ([string]$Request.worktreeRoot) ($path.Replace('/', [System.IO.Path]::DirectorySeparatorChar))
                        $null = [System.Management.Automation.Language.Parser]::ParseFile(
                            $fullPath,
                            [ref]$tokens,
                            [ref]$parserErrors
                        )
                        if (@($parserErrors).Count -gt 0) {
                            $gateStatus = 'FAIL'
                            $diagnostics += @($parserErrors | ForEach-Object { "${path}:$($_.Extent.StartLineNumber): $($_.Message)" })
                        }
                    }
                }
                'text-policy' {
                    $textResult = Test-GovernanceTextPolicy `
                        -RepositoryRoot ([string]$Request.worktreeRoot) `
                        -RepositoryPath @($Request.scopePaths)
                    $gateStatus = [string]$textResult.Status
                    $diagnostics = @($textResult.Diagnostics)
                    $readOnlyProbeCount += [int]$textResult.CheckedFileCount
                }
                'git-diff-check' {
                    $diffCheck = Invoke-GovernanceGitText `
                        -RepositoryRoot ([string]$Request.worktreeRoot) `
                        -Argument @('diff', '--check', [string]$Request.baselineCommit, '--') `
                        -AllowedExitCode @(0, 2)
                    $readOnlyProbeCount++
                    if ($diffCheck.ExitCode -ne 0) {
                        $gateStatus = 'FAIL'
                        $diagnostics = @($diffCheck.Text)
                    }
                }
                'external-input-binding' {
                    $inputResult = Test-GovernanceExternalInputBinding `
                        -SourceRepositoryRoot ([string]$Request.sourceRepositoryRoot) `
                        -ExternalInput @($Request.externalInputs)
                    $gateStatus = [string]$inputResult.Status
                    $diagnostics = @($inputResult.Diagnostics)
                    $readOnlyProbeCount += [int]$inputResult.ReadOnlyProbeCount
                }
                'toolchain-context-binding' {
                    $toolchainResult = Test-GovernanceToolchainBinding -Toolchain $Request.toolchain
                    $gateStatus = [string]$toolchainResult.Status
                    $diagnostics = @($toolchainResult.Diagnostics)
                    $readOnlyProbeCount += [int]$toolchainResult.ReadOnlyProbeCount
                }
                'source-worktree-selector-binding' {
                    $sourceResult = Test-GovernanceSourceBinding -Request $Request
                    $gateStatus = [string]$sourceResult.Status
                    $diagnostics = @($sourceResult.Diagnostics)
                    $readOnlyProbeCount += [int]$sourceResult.ReadOnlyProbeCount
                }
            }
        }
        catch {
            $gateStatus = 'FAIL'
            $diagnostics = @($_.Exception.Message)
            $infrastructureOrInvocationFailureCount++
        }
        if ($gateStatus -ceq 'FAIL') {
            $failureCount++
        }
        $stageResults.Add([pscustomobject]@{
            Id = $gateId
            Phase = 'CHEAP'
            Status = $gateStatus
            Diagnostics = @($diagnostics)
        })
        if ($gateStatus -ne 'PASS') {
            break
        }
    }
    if (@($stageResults | Where-Object { $_.Phase -ceq 'CHEAP' }).Count -lt $script:CheapGateOrder.Count) {
        foreach ($remainingId in $script:CheapGateOrder[@($stageResults).Count..($script:CheapGateOrder.Count - 1)]) {
            $stageResults.Add([pscustomobject]@{
                Id = $remainingId
                Phase = 'CHEAP'
                Status = 'NOT_RUN'
                Diagnostics = @('Stopped after an earlier cheap-gate failure.')
            })
        }
    }

    $evidenceDisposition = Get-GovernanceEvidenceDisposition `
        -PriorEvidence @($Request.priorEvidence) `
        -CurrentDependency @($Request.currentDependencies)
    if ($failureCount -eq 0) {
        foreach ($requiredStage in @($profile[0].requiredSubordinateStages)) {
            $declared = @($Request.subordinateResults | Where-Object {
                    [string]$_.id -ceq [string]$requiredStage
                })
            if ($declared.Count -ne 1) {
                $blocked = $true
                $stageResults.Add([pscustomobject]@{
                    Id = [string]$requiredStage
                    Phase = 'SUBORDINATE'
                    Status = 'BLOCKED'
                    Diagnostics = @('Required typed subordinate result is missing or duplicated.')
                })
                continue
            }
            try {
                if ((Get-GovernanceLowerSha256 -LiteralPath ([string]$declared[0].path)) -cne [string]$declared[0].sha256) {
                    throw 'Subordinate result hash mismatch.'
                }
                $typed = Read-GovernanceTypedResult `
                    -LiteralPath ([string]$declared[0].path) `
                    -SchemaPath ([string]$declared[0].schemaPath) `
                    -ExpectedProfile ([string]$declared[0].expectedProfile)
                $typedNames = Get-GovernanceObjectPropertyNames -Value $typed
                $typedStatus = if ('status' -in $typedNames) { [string]$typed.status } else { [string]$typed.result }
                if ($typedStatus -ceq 'FAIL') { $failureCount++ }
                elseif ($typedStatus -ceq 'BLOCKED') { $blocked = $true }
                $stageResults.Add([pscustomobject]@{
                    Id = [string]$requiredStage
                    Phase = 'SUBORDINATE'
                    Status = $typedStatus
                    Diagnostics = @()
                })
            }
            catch {
                $failureCount++
                $infrastructureOrInvocationFailureCount++
                $stageResults.Add([pscustomobject]@{
                    Id = [string]$requiredStage
                    Phase = 'SUBORDINATE'
                    Status = 'FAIL'
                    Diagnostics = @($_.Exception.Message)
                })
            }
        }
    }

    $status = if ($failureCount -gt 0) { 'FAIL' } elseif ($blocked) { 'BLOCKED' } else { 'PASS' }
    $fullMatrixRunCount = @($stageResults | Where-Object { $_.Id -ceq 'full-completion' -and $_.Status -ceq 'PASS' }).Count
    return [pscustomobject][ordered]@{
        schemaVersion = 1
        taskId = [string]$Request.taskId
        profile = [string]$Request.profile
        status = $status
        bindings = [ordered]@{
            repository = [string]$Request.repository
            sourceRepositoryRoot = [System.IO.Path]::GetFullPath([string]$Request.sourceRepositoryRoot)
            worktreeRoot = [System.IO.Path]::GetFullPath([string]$Request.worktreeRoot)
            baselineCommit = [string]$Request.baselineCommit
            currentCommit = [string]$Request.currentCommit
            expectedTree = [string]$Request.expectedTree
            expectedStatusSha256 = [string]$Request.expectedStatusSha256
            actualStatusSha256 = if ($null -eq $sourceResult) { $null } else { [string]$sourceResult.ActualStatusSha256 }
            protectedWorktrees = [object[]]$(if ($null -eq $sourceResult) {
                @()
            }
            else {
                @($sourceResult.ProtectedWorktrees | ForEach-Object {
                        [ordered]@{
                            id = [string]$_.Id
                            root = [string]$_.Root
                            expectedStatusSha256 = [string]$_.ExpectedStatusSha256
                            actualStatusSha256 = if ($null -eq $_.ActualStatusSha256) { $null } else { [string]$_.ActualStatusSha256 }
                            status = [string]$_.Status
                        }
                    })
            })
            selectorInventorySha256 = [string]$Request.selectors.inventorySha256
        }
        stageResults = @($stageResults)
        evidenceReuse = [ordered]@{
            reusedIds = @($evidenceDisposition.ReusedIds)
            invalidated = @($evidenceDisposition.Invalidated)
        }
        validationExecutionCount = 1
        infrastructureOrInvocationFailureCount = $infrastructureOrInvocationFailureCount
        fullMatrixRunCount = $fullMatrixRunCount
        packageWriteAttemptCount = 0
        generatedTaskControllerFileCount = 0
        generatedTaskControllerLineCount = 0
        readOnlyProbeCount = $readOnlyProbeCount
        observedWarningCount = 0
        resolvedWarningCount = 0
        openWarningCount = 0
        warningCount = 0
        failureCount = $failureCount
    }
}

Export-ModuleMember -Function @(
    'Get-GovernanceLowerSha256',
    'Read-GovernanceJsonContract',
    'Read-GovernanceTypedResult',
    'Test-GovernanceTextPolicy',
    'Test-GovernanceExternalInputBinding',
    'Test-GovernanceToolchainBinding',
    'Get-GovernanceWorkingTreeStatusBinding',
    'Test-GovernanceSourceBinding',
    'Get-GovernanceEvidenceDisposition',
    'Merge-GovernanceOptimisticText',
    'Set-GovernanceExternalTextOptimistic',
    'Invoke-GovernanceValidationOrchestration'
)
