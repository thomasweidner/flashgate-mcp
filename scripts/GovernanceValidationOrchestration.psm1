#requires -Version 7.6

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

. (Join-Path $PSScriptRoot 'GenericGovernanceGitEvidence.ps1')

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
            throw ('JSON schema validation failed: {0}; {1}' -f $resolvedPath, (@($schemaErrors | ForEach-Object ToString) -join ' | '))
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
        throw ('Git command failed ({0}): git {1}; {2}' -f $exitCode, ($Argument -join ' '), ($output -join ' | '))
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
        [AllowEmptyCollection()][byte[]]$StandardInputBytes = @(),
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
    $startInfo.RedirectStandardInput = $StandardInputBytes.Count -gt 0
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
        if ($StandardInputBytes.Count -gt 0) {
            $process.StandardInput.BaseStream.Write($StandardInputBytes, 0, $StandardInputBytes.Length)
            $process.StandardInput.Close()
        }
        $standardOutputCopy = $process.StandardOutput.BaseStream.CopyToAsync($standardOutput)
        $standardErrorRead = $process.StandardError.ReadToEndAsync()
        $process.WaitForExit()
        [void]$standardOutputCopy.GetAwaiter().GetResult()
        $standardError = $standardErrorRead.GetAwaiter().GetResult()
        if ($process.ExitCode -notin $AllowedExitCode) {
            throw ('Git command failed ({0}): git {1}; {2}' -f $process.ExitCode, ($Argument -join ' '), $standardError)
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
    $pathComparer = if ($IsWindows) { [System.StringComparer]::OrdinalIgnoreCase } else { [System.StringComparer]::Ordinal }
    $pathComparison = if ($IsWindows) { [System.StringComparison]::OrdinalIgnoreCase } else { [System.StringComparison]::Ordinal }
    $paths = [System.Collections.Generic.HashSet[string]]::new($pathComparer)
    $ids = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::Ordinal)
    $probeCount = 0
    $normalizePath = {
        param([string]$Value)
        $full = [System.IO.Path]::GetFullPath($Value)
        $root = [System.IO.Path]::GetPathRoot($full)
        if ([string]::Equals($full, $root, $pathComparison)) { return $full }
        return $full.TrimEnd('\', '/')
    }
    $repositoryRoot = & $normalizePath $SourceRepositoryRoot
    foreach ($entry in @($ExternalInput)) {
        $id = [string]$entry.id
        $path = & $normalizePath ([string]$entry.path)
        $sourceRoot = & $normalizePath ([string]$entry.sourceRoot)
        if (-not $ids.Add($id)) {
            $diagnostics.Add("Duplicate input ID: $id")
            continue
        }
        if (-not $paths.Add($path)) {
            $diagnostics.Add("Duplicate input path: $path")
            continue
        }
        if (-not (Test-Path -LiteralPath $sourceRoot -PathType Container)) {
            $diagnostics.Add("Input source root is missing or not a directory: $sourceRoot")
            continue
        }
        $probeCount++
        $sourceRootItem = Get-Item -LiteralPath $sourceRoot -Force
        if (($sourceRootItem.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0 -or
            $null -ne $sourceRootItem.LinkType) {
            $diagnostics.Add("Input source root is a link or reparse point: $sourceRoot")
            continue
        }
        $rootPrefix = if ($sourceRoot.EndsWith([System.IO.Path]::DirectorySeparatorChar)) {
            $sourceRoot
        }
        else {
            $sourceRoot + [System.IO.Path]::DirectorySeparatorChar
        }
        if (-not $path.StartsWith($rootPrefix, $pathComparison)) {
            $diagnostics.Add("Input is outside its declared source root: $path")
            continue
        }
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
            $diagnostics.Add("Required input is missing: $path")
            continue
        }
        $probeCount++
        $cursor = $path
        $unsafeLink = $false
        while ($true) {
            $item = Get-Item -LiteralPath $cursor -Force
            if (($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0 -or
                $null -ne $item.LinkType) {
                $unsafeLink = $true
                break
            }
            if ([string]::Equals($cursor, $sourceRoot, $pathComparison)) { break }
            $cursor = Split-Path -Parent $cursor
            if ([string]::IsNullOrWhiteSpace($cursor) -or
                -not $cursor.StartsWith($rootPrefix, $pathComparison) -and
                -not [string]::Equals($cursor, $sourceRoot, $pathComparison)) {
                $unsafeLink = $true
                break
            }
        }
        if ($unsafeLink) {
            $diagnostics.Add("Input traverses a link or reparse point: $path")
            continue
        }
        $actualHash = Get-GovernanceLowerSha256 -LiteralPath $path
        $probeCount++
        if ($actualHash -cne [string]$entry.sha256) {
            $diagnostics.Add("Input hash mismatch: $path")
            continue
        }
        $kind = [string]$entry.kind
        $repositoryPrefix = if ($repositoryRoot.EndsWith([System.IO.Path]::DirectorySeparatorChar)) {
            $repositoryRoot
        }
        else {
            $repositoryRoot + [System.IO.Path]::DirectorySeparatorChar
        }
        $insideRepository = $path.StartsWith($repositoryPrefix, $pathComparison)
        if ($kind -ceq 'EXTERNAL') {
            if ($insideRepository) {
                $diagnostics.Add("EXTERNAL input is inside the repository: $path")
                continue
            }
        }
        else {
            if (-not $insideRepository -or -not [string]::Equals($sourceRoot, $repositoryRoot, $pathComparison)) {
                $diagnostics.Add("Git-classified input is not rooted in the source repository: $path")
                continue
            }
            $relativePath = [System.IO.Path]::GetRelativePath($repositoryRoot, $path).Replace('\', '/')
            $tracked = Invoke-GovernanceGitText -RepositoryRoot $repositoryRoot `
                -Argument @('ls-files', '--error-unmatch', '--', $relativePath) -AllowedExitCode @(0, 1)
            $ignoreInput = [System.Text.UTF8Encoding]::new($false).GetBytes($relativePath + [char]0)
            $ignore = Invoke-GovernanceGitBytes -RepositoryRoot $repositoryRoot `
                -Argument @('check-ignore', '-v', '-z', '--no-index', '--stdin') `
                -StandardInputBytes $ignoreInput `
                -AllowedExitCode @(0, 1)
            $gitInfoExclude = Invoke-GovernanceGitText -RepositoryRoot $repositoryRoot `
                -Argument @('rev-parse', '--git-path', 'info/exclude')
            $coreExcludes = Invoke-GovernanceGitText -RepositoryRoot $repositoryRoot `
                -Argument @('config', '--path', '--get', 'core.excludesFile') -AllowedExitCode @(0, 1)
            $probeCount += 4
            $gitInfoExcludePath = if ([System.IO.Path]::IsPathRooted($gitInfoExclude.Text.Trim())) {
                & $normalizePath $gitInfoExclude.Text.Trim()
            }
            else {
                & $normalizePath (Join-Path $repositoryRoot $gitInfoExclude.Text.Trim())
            }
            $coreExcludesPath = $null
            if ($coreExcludes.ExitCode -eq 0 -and -not [string]::IsNullOrWhiteSpace($coreExcludes.Text)) {
                $coreExcludesPath = if ([System.IO.Path]::IsPathRooted($coreExcludes.Text.Trim())) {
                    & $normalizePath $coreExcludes.Text.Trim()
                }
                else {
                    & $normalizePath (Join-Path $repositoryRoot $coreExcludes.Text.Trim())
                }
            }
            $ignoreSourcePath = $null
            if ($ignore.ExitCode -eq 0) {
                $ignoreText = [System.Text.UTF8Encoding]::new($false, $true).GetString([byte[]]$ignore.Bytes)
                $ignoreFields = @($ignoreText.Split([char]0))
                if ($ignoreFields.Count -ge 4 -and -not [string]::IsNullOrWhiteSpace($ignoreFields[0])) {
                    $ignoreSourcePath = if ([System.IO.Path]::IsPathRooted($ignoreFields[0])) {
                        & $normalizePath $ignoreFields[0]
                    }
                    else {
                        & $normalizePath (Join-Path $repositoryRoot $ignoreFields[0])
                    }
                }
            }
            $gitExcluded = $null -ne $ignoreSourcePath -and (
                [string]::Equals($ignoreSourcePath, $gitInfoExcludePath, $pathComparison) -or
                ($null -ne $coreExcludesPath -and
                    [string]::Equals($ignoreSourcePath, $coreExcludesPath, $pathComparison))
            )
            $classified = if ($tracked.ExitCode -eq 0) {
                'VERSIONED'
            }
            elseif ($ignore.ExitCode -eq 0 -and $gitExcluded) {
                'GIT_EXCLUDED'
            }
            elseif ($ignore.ExitCode -eq 0) {
                'IGNORED'
            }
            else {
                'UNCLASSIFIED'
            }
            if ($classified -cne $kind) {
                $diagnostics.Add("Git classification mismatch for ${path}: expected $kind, actual $classified")
                continue
            }
        }
        [void]$bound.Add([pscustomobject]@{
            Id = $id
            Path = $path
            SourceRoot = $sourceRoot
            Kind = $kind
            Purpose = [string]$entry.purpose
            Sha256 = $actualHash
        })
    }
    return [pscustomobject]@{
        Status = if ($diagnostics.Count -eq 0) { 'PASS' } else { 'FAIL' }
        BoundInputs = @($bound)
        Diagnostics = @($diagnostics)
        ReadOnlyProbeCount = $probeCount
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
    $actualCommit = $null
    $actualTree = $null
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
        $actualCommit = $head.Text.Trim()
        $actualTree = $tree.Text.Trim()
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
    $selectorHash = $null
    $resolvedCaseCount = 0
    try {
        $governanceRoot = [System.IO.Path]::GetFullPath((Split-Path -Parent $PSScriptRoot))
        $selectorModulePath = Join-Path $governanceRoot 'scripts/GovernanceCaseSelection.psm1'
        $metadataPath = Join-Path $governanceRoot 'Governance/governance-case-metadata.json'
        $metadataSchemaPath = Join-Path $governanceRoot 'Governance/governance-case-metadata.schema.json'
        if ([string]$Request.selectors.interface -cne 'scripts/GovernanceCaseSelection.psm1' -or
            [string]$Request.selectors.metadataPath -cne 'Governance/governance-case-metadata.json' -or
            [string]$Request.selectors.schemaPath -cne 'Governance/governance-case-metadata.schema.json') {
            throw 'Selector request does not bind the canonical BL-338 source, schema, and resolver.'
        }
        Import-Module -Name $selectorModulePath -Force -ErrorAction Stop
        $metadata = Read-GovernanceCaseMetadata -Path $metadataPath -SchemaPath $metadataSchemaPath
        if ([string]$metadata.MetadataInventorySHA256 -cne [string]$Request.selectors.inventorySha256) {
            throw 'Canonical selector inventory hash mismatch.'
        }
        $selection = Resolve-GovernanceCaseSelection -Metadata $metadata `
            -CaseName @($Request.selectors.caseNames) `
            -Group @($Request.selectors.groups) `
            -Tag @($Request.selectors.tags) `
            -TargetPlatform ([string]$Request.selectors.targetPlatform) `
            -AvailableCapability @($Request.selectors.availableCapabilities)
        $selectorHash = [string]$selection.ResolvedCaseSetSHA256
        $resolvedCaseCount = [int]$selection.ResolvedCaseCount
        if (-not [bool]$selection.ReadyToExecute -or [string]$selection.SelectorResolutionResult -cne 'PASS') {
            foreach ($selectorDiagnostic in @($selection.ErrorDiagnostics)) {
                $diagnostics.Add("Selector resolution failed: $($selectorDiagnostic | ConvertTo-Json -Compress -Depth 10)")
            }
            if (@($selection.ErrorDiagnostics).Count -eq 0) {
                $diagnostics.Add('Selector resolution failed without executable cases.')
            }
        }
    }
    catch {
        $diagnostics.Add("Canonical selector binding failed: $($_.Exception.Message)")
    }
    return [pscustomobject]@{
        Status = if ($diagnostics.Count -eq 0) { 'PASS' } else { 'FAIL' }
        SourceRepositoryRoot = $sourceRoot
        WorktreeRoot = $worktreeRoot
        CurrentCommit = $actualCommit
        Tree = $actualTree
        DetachedHead = $actualDetached
        ExpectedStatusSha256 = [string]$Request.expectedStatusSha256
        ActualStatusSha256 = $actualStatusHash
        ScopePathCount = @($Request.scopePaths).Count
        FileHashPathCount = @($Request.fileHashes).Count
        ProtectedWorktrees = @($protectedBindings)
        SelectorInventorySha256 = [string]$Request.selectors.inventorySha256
        SelectionSha256 = $selectorHash
        ResolvedCaseCount = $resolvedCaseCount
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

function Get-GovernanceCanonicalDigest {
    [CmdletBinding()]
    param([Parameter(Mandatory)][AllowNull()][object]$Value)

    $json = if ($null -eq $Value) {
        'null'
    }
    elseif ($Value -is [array] -and @($Value).Count -eq 0) {
        '[]'
    }
    else {
        $Value | ConvertTo-Json -Compress -Depth 100
    }
    $bytes = [System.Text.UTF8Encoding]::new($false).GetBytes($json)
    return Get-GovernanceByteSha256 -Bytes $bytes
}

function Get-GovernanceCurrentComponentHashes {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][object]$Request,
        [AllowNull()][object]$SourceResult,
        [AllowNull()][object]$ExternalResult,
        [Parameter(Mandatory)][object]$EvidenceDisposition
    )

    $hashes = [ordered]@{
        commit = $null
        tree = $null
        'working-status' = $null
        scope = $null
        selector = $null
        package = $null
        'external-inputs' = $null
        evidence = $null
    }
    if ($null -ne $SourceResult) {
        if (-not [string]::IsNullOrWhiteSpace([string]$SourceResult.CurrentCommit)) {
            $hashes.commit = Get-GovernanceCanonicalDigest -Value ([ordered]@{
                    component = 'commit'; value = [string]$SourceResult.CurrentCommit
                })
        }
        if (-not [string]::IsNullOrWhiteSpace([string]$SourceResult.Tree)) {
            $hashes.tree = Get-GovernanceCanonicalDigest -Value ([ordered]@{
                    component = 'tree'; value = [string]$SourceResult.Tree
                })
        }
        if (-not [string]::IsNullOrWhiteSpace([string]$SourceResult.ActualStatusSha256)) {
            $hashes['working-status'] = [string]$SourceResult.ActualStatusSha256
        }
        $actualScope = @(
            $Request.fileHashes | Sort-Object { [string]$_.path } | ForEach-Object {
                $relativePath = [string]$_.path
                $fullPath = Join-Path ([string]$Request.worktreeRoot) ($relativePath.Replace('/', [System.IO.Path]::DirectorySeparatorChar))
                [ordered]@{
                    path = $relativePath
                    sha256 = if (Test-Path -LiteralPath $fullPath -PathType Leaf) {
                        Get-GovernanceLowerSha256 -LiteralPath $fullPath
                    }
                    else { $null }
                }
            }
        )
        $hashes.scope = Get-GovernanceCanonicalDigest -Value ([ordered]@{
                scopePaths = @($Request.scopePaths | Sort-Object)
                files = $actualScope
            })
        if (-not [string]::IsNullOrWhiteSpace([string]$SourceResult.SelectionSha256)) {
            $hashes.selector = [string]$SourceResult.SelectionSha256
        }
    }
    if ($null -ne $ExternalResult) {
        $externalState = @($ExternalResult.BoundInputs | Sort-Object Id | ForEach-Object {
                [ordered]@{
                    id = [string]$_.Id
                    path = [string]$_.Path
                    sourceRoot = [string]$_.SourceRoot
                    kind = [string]$_.Kind
                    sha256 = [string]$_.Sha256
                }
            })
        $hashes['external-inputs'] = Get-GovernanceCanonicalDigest -Value $externalState
    }
    $packageInputs = @($Request.subordinateResults | Sort-Object { [string]$_.id } | ForEach-Object {
            $resultPath = [string]$_.path
            [ordered]@{
                id = [string]$_.id
                sha256 = if (Test-Path -LiteralPath $resultPath -PathType Leaf) {
                    Get-GovernanceLowerSha256 -LiteralPath $resultPath
                }
                else { $null }
            }
        })
    $hashes.package = Get-GovernanceCanonicalDigest -Value ([ordered]@{
            profile = [string]$Request.profile
            exactCommit = $Request.exactCommit
            subordinateResults = $packageInputs
        })
    $priorEvidenceState = @($Request.priorEvidence | Sort-Object { [string]$_.id } | ForEach-Object {
            $evidencePath = [string]$_.path
            [ordered]@{
                id = [string]$_.id
                sha256 = if (Test-Path -LiteralPath $evidencePath -PathType Leaf) {
                    Get-GovernanceLowerSha256 -LiteralPath $evidencePath
                }
                else { $null }
            }
        })
    $hashes.evidence = Get-GovernanceCanonicalDigest -Value ([ordered]@{
            currentDependencies = @($Request.currentDependencies | Sort-Object { [string]$_.id })
            priorEvidence = $priorEvidenceState
            reusedIds = @($EvidenceDisposition.ReusedIds | Sort-Object)
            invalidated = @($EvidenceDisposition.Invalidated | Sort-Object Id)
        })
    return $hashes
}

function Get-GovernanceStateTransitionMap {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][object[]]$StateComponent,
        [Parameter(Mandatory)][System.Collections.IDictionary]$CurrentComponentHash
    )

    $required = @('commit', 'tree', 'working-status', 'scope', 'selector', 'package', 'external-inputs', 'evidence')
    $actual = @($StateComponent | ForEach-Object { [string]$_.component })
    if ((@($actual | Sort-Object -Unique).Count -ne $required.Count) -or
        (($actual | Sort-Object) -join "`n") -cne (($required | Sort-Object) -join "`n")) {
        throw 'State transition input must contain each canonical component exactly once.'
    }
    return @($required | ForEach-Object {
            $component = [string]$_
            $entry = @($StateComponent | Where-Object { [string]$_.component -ceq $component })[0]
            $currentHash = $CurrentComponentHash[$component]
            $reused = $null -ne $currentHash -and
                [string]$entry.priorSha256 -ceq [string]$currentHash
            [pscustomobject][ordered]@{
                Component = $component
                PriorSha256 = [string]$entry.priorSha256
                CurrentSha256 = if ($null -eq $currentHash) { $null } else { [string]$currentHash }
                Disposition = if ($reused) { 'REUSED' } else { 'INVALIDATED' }
                Reason = if ($reused) {
                    'ACTUAL_HASH_BOUND_STATE_UNCHANGED'
                }
                elseif ($null -eq $currentHash) {
                    'ACTUAL_CURRENT_STATE_NOT_AVAILABLE'
                }
                else {
                    "ACTUAL_${component}_HASH_CHANGED".ToUpperInvariant()
                }
            }
        })
}

function Test-GovernanceTaskControllerInventory {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][string]$WorktreeRoot,
        [AllowEmptyCollection()][object[]]$TaskController = @()
    )

    $diagnostics = [System.Collections.Generic.List[string]]::new()
    $pathComparer = if ($IsWindows) { [System.StringComparer]::OrdinalIgnoreCase } else { [System.StringComparer]::Ordinal }
    $pathComparison = if ($IsWindows) { [System.StringComparison]::OrdinalIgnoreCase } else { [System.StringComparison]::Ordinal }
    $resolvedWorktreeRoot = [System.IO.Path]::GetFullPath($WorktreeRoot).TrimEnd('\', '/')
    $worktreePrefix = $resolvedWorktreeRoot + [System.IO.Path]::DirectorySeparatorChar
    $permanentRoot = Join-Path $resolvedWorktreeRoot 'scripts'
    $permanentPrefix = $permanentRoot + [System.IO.Path]::DirectorySeparatorChar
    $actualInventory = [System.Collections.Generic.Dictionary[string, object]]::new($pathComparer)
    $trackedPaths = [System.Collections.Generic.HashSet[string]]::new($pathComparer)
    $inventoryConventionRoot = Join-Path $resolvedWorktreeRoot '.governance-task-controllers'
    $inventoryConventionPrefix = $inventoryConventionRoot + [System.IO.Path]::DirectorySeparatorChar

    if (-not (Test-Path -LiteralPath $resolvedWorktreeRoot -PathType Container)) {
        $diagnostics.Add("Validated worktree root is not a directory: $resolvedWorktreeRoot")
    }
    else {
        $rootItem = Get-Item -LiteralPath $resolvedWorktreeRoot -Force
        if (($rootItem.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0 -or
            $null -ne $rootItem.LinkType) {
            $diagnostics.Add("Validated worktree root is a link or reparse point: $resolvedWorktreeRoot")
        }
        else {
            $trackedResult = Invoke-GovernanceGitText -RepositoryRoot $resolvedWorktreeRoot `
                -Argument @('ls-files', '-z')
            foreach ($trackedPath in @($trackedResult.Text.Split([char]0))) {
                if (-not [string]::IsNullOrEmpty($trackedPath)) {
                    [void]$trackedPaths.Add($trackedPath.Replace('\', '/'))
                }
            }

            $pendingDirectories = [System.Collections.Generic.Stack[string]]::new()
            $pendingDirectories.Push($resolvedWorktreeRoot)
            while ($pendingDirectories.Count -gt 0) {
                $directory = $pendingDirectories.Pop()
                foreach ($item in @(Get-ChildItem -LiteralPath $directory -Force -ErrorAction Stop)) {
                    if ([string]::Equals([string]$item.Name, '.git', $pathComparison)) {
                        continue
                    }
                    if (($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0 -or
                        $null -ne $item.LinkType) {
                        $diagnostics.Add("Controller inventory contains a link or reparse point: $($item.FullName)")
                        continue
                    }
                    if ($item.PSIsContainer) {
                        $pendingDirectories.Push([System.IO.Path]::GetFullPath($item.FullName))
                        continue
                    }
                    $isPowerShellArtifact =
                        [string]::Equals([string]$item.Extension, '.ps1', $pathComparison) -or
                        [string]::Equals([string]$item.Extension, '.psm1', $pathComparison)
                    $actualPath = [System.IO.Path]::GetFullPath($item.FullName)
                    if (-not $isPowerShellArtifact) {
                        if ($actualPath.StartsWith($inventoryConventionPrefix, $pathComparison)) {
                            $diagnostics.Add("Task-controller inventory convention contains an unsupported executable artifact: $actualPath")
                        }
                        continue
                    }
                    $relativePath = [System.IO.Path]::GetRelativePath($resolvedWorktreeRoot, $actualPath).Replace('\', '/')
                    $isPermanentProfile =
                        $actualPath.StartsWith($permanentPrefix, $pathComparison) -and
                        $trackedPaths.Contains($relativePath)
                    if ($isPermanentProfile) {
                        continue
                    }
                    if ($actualInventory.ContainsKey($actualPath)) {
                        $diagnostics.Add("Duplicate or case-aliased actual task controller: $actualPath")
                        continue
                    }
                    $actualInventory.Add($actualPath, [pscustomobject]@{
                            Path = $actualPath
                            LineCount = @([System.IO.File]::ReadAllLines($actualPath)).Count
                        })
                }
            }
        }
    }

    $taskSpecificFileCount = $actualInventory.Count
    $taskSpecificLineCount = if ($actualInventory.Count -eq 0) {
        0
    }
    else {
        [int]($actualInventory.Values | Measure-Object -Property LineCount -Sum).Sum
    }
    $declaredPaths = [System.Collections.Generic.HashSet[string]]::new($pathComparer)
    $declaredTaskPaths = [System.Collections.Generic.HashSet[string]]::new($pathComparer)
    foreach ($controller in @($TaskController)) {
        try {
            $path = [System.IO.Path]::GetFullPath([string]$controller.path)
        }
        catch {
            $diagnostics.Add("Invalid controller declaration path: $([string]$controller.path)")
            continue
        }
        if (-not $declaredPaths.Add($path)) {
            $diagnostics.Add("Duplicate or case-aliased controller declaration path: $path")
            continue
        }
        $insideValidatedWorktree = $path.StartsWith($worktreePrefix, $pathComparison)
        if (-not $insideValidatedWorktree) {
            $diagnostics.Add("Controller declaration is outside the validated worktree: $path")
        }
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
            $diagnostics.Add("Controller inventory path is missing: $path")
            continue
        }
        $controllerItem = Get-Item -LiteralPath $path -Force
        if (($controllerItem.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0 -or
            $null -ne $controllerItem.LinkType) {
            $diagnostics.Add("Controller inventory path is a link or reparse point: $path")
            continue
        }
        $actualLineCount = @([System.IO.File]::ReadAllLines($path)).Count
        if ($actualLineCount -ne [int]$controller.lineCount) {
            $diagnostics.Add("Controller line-count inventory drift: $path")
        }
        if ([string]$controller.classification -ceq 'TASK_SPECIFIC_EXECUTABLE') {
            [void]$declaredTaskPaths.Add($path)
            if ($insideValidatedWorktree -and -not $actualInventory.ContainsKey($path)) {
                $diagnostics.Add("Declared task-specific controller is absent from the authoritative inventory: $path")
            }
            $diagnostics.Add("Executable task-specific controller is prohibited: $path")
            if (-not [string]::IsNullOrWhiteSpace([string]$controller.exceptionId)) {
                $diagnostics.Add("Unknown task-controller exception: $($controller.exceptionId)")
            }
        }
        elseif ([string]$controller.classification -ceq 'PERMANENT_PROFILE') {
            if (-not $path.StartsWith($permanentPrefix, $pathComparison)) {
                $diagnostics.Add("Permanent profile is outside the canonical scripts root: $path")
            }
            else {
                $relativePath = [System.IO.Path]::GetRelativePath($resolvedWorktreeRoot, $path).Replace('\', '/')
                if (-not $trackedPaths.Contains($relativePath)) {
                    $diagnostics.Add("Permanent profile is not a versioned existing helper: $path")
                }
            }
            if ($null -ne $controller.exceptionId) {
                $diagnostics.Add("Permanent profiles cannot declare a task-controller exception: $path")
            }
        }
        else {
            $diagnostics.Add("Unknown controller classification: $($controller.classification)")
        }
    }
    foreach ($actualPath in $actualInventory.Keys) {
        if (-not $declaredTaskPaths.Contains($actualPath)) {
            $diagnostics.Add("Undeclared actual task-specific controller: $actualPath")
        }
    }
    foreach ($declaredTaskPath in $declaredTaskPaths) {
        if (-not $actualInventory.ContainsKey($declaredTaskPath)) {
            $diagnostics.Add("Controller declaration/actual inventory mismatch: $declaredTaskPath")
        }
    }
    return [pscustomobject][ordered]@{
        Status = if ($diagnostics.Count -eq 0) { 'PASS' } else { 'FAIL' }
        AuthoritativeInventoryRoot = $resolvedWorktreeRoot
        GeneratedTaskControllerFileCount = $taskSpecificFileCount
        GeneratedTaskControllerLineCount = $taskSpecificLineCount
        Diagnostics = @($diagnostics)
    }
}

function Test-GovernanceExactCommitProfile {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][object]$Request,
        [Parameter(Mandatory)][object]$SourceResult
    )

    $notRun = [pscustomobject][ordered]@{
        IntendedBaseResult = 'NOT_RUN'
        MergeBaseResult = 'NOT_RUN'
        EffectivePRScopeResult = 'NOT_RUN'
        EffectivePRPatchHash = $null
        IntegrationProjectionResult = 'NOT_RUN'
        IntegrationProjectionHash = $null
        AuthorizedWriteSetResult = 'NOT_RUN'
        ForeignProtectedStateResult = 'NOT_RUN'
        Diagnostics = @()
    }
    if ($null -eq $Request.exactCommit) {
        if ([string]$Request.profile -ceq 'commit-preparation') {
            $notRun.IntendedBaseResult = 'FAIL'
            $notRun.Diagnostics = @('commit-preparation requires the exactCommit contract.')
        }
        return $notRun
    }

    $contract = $Request.exactCommit
    $diagnostics = [System.Collections.Generic.List[string]]::new()
    $root = [string]$Request.worktreeRoot
    $intendedBasePass = [string]$Request.baselineCommit -ceq [string]$contract.intendedBase
    try {
        $null = Invoke-GovernanceGitText -RepositoryRoot $root -Argument @('cat-file', '-e', "$($contract.intendedBase)^{commit}")
    }
    catch { $intendedBasePass = $false }
    if (-not $intendedBasePass) { $diagnostics.Add('Intended base is absent or does not match the bound baseline.') }

    $mergeBase = Invoke-GovernanceGitText -RepositoryRoot $root -Argument @('merge-base', [string]$Request.currentCommit, [string]$contract.targetRef)
    $mergeBasePass = $mergeBase.Text.Trim() -ceq [string]$contract.expectedMergeBase
    if (-not $mergeBasePass) { $diagnostics.Add('Merge base does not match the expected intended-base projection.') }

    $entries = @(Get-GenericStatusEvidence -Root $root -BaselineCommit ([string]$contract.intendedBase))
    $authorized = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::Ordinal)
    foreach ($path in @($contract.authorizedWriteSet)) { [void]$authorized.Add([string]$path) }
    $included = @($entries | Where-Object {
            $authorized.Contains([string]$_.Path) -and
            ($null -eq $_.PreviousPath -or $authorized.Contains([string]$_.PreviousPath))
        })
    $excluded = @($entries | Where-Object { $_ -notin $included })
    $staged = @($entries | Where-Object { [bool]$_.Staged })
    $effectiveScope = @(Get-GenericScopePaths -Entry $entries | Sort-Object -Unique)
    $scopePass = (($effectiveScope -join "`n") -ceq (@($contract.expectedEffectivePRScope | Sort-Object) -join "`n"))
    $authorizedPass = $excluded.Count -eq 0 -and $staged.Count -eq 0
    if (-not $scopePass) { $diagnostics.Add('Effective PR scope differs from the expected scope.') }
    if ($excluded.Count -gt 0) { $diagnostics.Add('Effective delta contains a path outside the authorized write set.') }
    if ($staged.Count -gt 0) { $diagnostics.Add('Staged input is prohibited for exact commit preparation.') }

    $patchHash = $null
    $projectionHash = $null
    $patchPass = $false
    $projectionPass = $false
    if ($included.Count -gt 0 -and $authorizedPass) {
        $delta = Get-GenericDeltaEvidence -Root $root -BaselineCommit ([string]$contract.intendedBase) `
            -IncludedEntry $included -ExcludedEntry $excluded
        $patchHash = Get-GovernanceByteSha256 -Bytes ([byte[]]$delta.Bytes)
        $patchPass = $patchHash -ceq [string]$contract.expectedEffectivePRPatchSha256 -and
            [bool]$delta.ActualDeltaInventoryParity -and [bool]$delta.ExcludedDeltaPathProhibition
        $projection = Get-GenericIntegrationProjectionEvidence -Root $root `
            -BaselineCommit ([string]$contract.intendedBase) -IncludedEntry $included -ExcludedEntry $excluded
        $projectionHash = [string]$projection.Tree
        $projectionPass = $projectionHash -ceq [string]$contract.expectedIntegrationProjection -and
            [bool]$projection.ExcludedPathProhibition
    }
    if (-not $patchPass) { $diagnostics.Add('Effective PR patch hash or actual delta inventory is not reproducible.') }
    if (-not $projectionPass) { $diagnostics.Add('Integration projection is not reproducible.') }

    $protected = @($SourceResult.ProtectedWorktrees)
    $protectedPassCount = @($protected | Where-Object { [string]$_.Status -ceq 'PASS' }).Count
    $foreignResult = if ($protectedPassCount -eq $protected.Count) { 'PASS_{0}_OF_{1}' -f $protectedPassCount, $protected.Count } else { 'FAIL' }
    return [pscustomobject][ordered]@{
        IntendedBaseResult = if ($intendedBasePass) { 'PASS' } else { 'FAIL' }
        MergeBaseResult = if ($mergeBasePass) { 'PASS' } else { 'FAIL' }
        EffectivePRScopeResult = if ($scopePass -and $patchPass) { 'PASS' } else { 'FAIL' }
        EffectivePRPatchHash = $patchHash
        IntegrationProjectionResult = if ($projectionPass) { 'PASS' } else { 'FAIL' }
        IntegrationProjectionHash = $projectionHash
        AuthorizedWriteSetResult = if ($authorizedPass) { 'PASS' } else { 'FAIL' }
        ForeignProtectedStateResult = $foreignResult
        Diagnostics = @($diagnostics)
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
    $inputResult = $null
    $validationExecutionCount = 0
    $controllerResult = [pscustomobject][ordered]@{
        Status = 'NOT_RUN'
        AuthoritativeInventoryRoot = $null
        GeneratedTaskControllerFileCount = 0
        GeneratedTaskControllerLineCount = 0
        Diagnostics = @('Controller inventory requires a successful source-worktree binding.')
    }
    $profileResults = [pscustomobject][ordered]@{
        IntendedBaseResult = 'NOT_RUN'
        MergeBaseResult = 'NOT_RUN'
        EffectivePRScopeResult = 'NOT_RUN'
        EffectivePRPatchHash = $null
        IntegrationProjectionResult = 'NOT_RUN'
        IntegrationProjectionHash = $null
        AuthorizedWriteSetResult = 'NOT_RUN'
        ForeignProtectedStateResult = 'NOT_RUN'
        Diagnostics = @()
    }

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
                    if ($gateStatus -ceq 'PASS') {
                        $controllerResult = Test-GovernanceTaskControllerInventory `
                            -WorktreeRoot ([string]$sourceResult.WorktreeRoot) `
                            -TaskController @($Request.taskControllers)
                        if ([string]$controllerResult.Status -ne 'PASS') {
                            $gateStatus = 'FAIL'
                            $diagnostics += @($controllerResult.Diagnostics)
                        }
                    }
                    if ($gateStatus -ceq 'PASS') {
                        $profileResults = Test-GovernanceExactCommitProfile -Request $Request -SourceResult $sourceResult
                        if (@(
                                $profileResults.IntendedBaseResult,
                                $profileResults.MergeBaseResult,
                                $profileResults.EffectivePRScopeResult,
                                $profileResults.IntegrationProjectionResult,
                                $profileResults.AuthorizedWriteSetResult
                            ) -contains 'FAIL' -or [string]$profileResults.ForeignProtectedStateResult -ceq 'FAIL') {
                            $gateStatus = 'FAIL'
                            $diagnostics += @($profileResults.Diagnostics)
                        }
                    }
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
    $currentComponentHashes = Get-GovernanceCurrentComponentHashes `
        -Request $Request `
        -SourceResult $sourceResult `
        -ExternalResult $inputResult `
        -EvidenceDisposition $evidenceDisposition
    $stateTransitionMap = @(Get-GovernanceStateTransitionMap `
            -StateComponent @($Request.stateComponents) `
            -CurrentComponentHash $currentComponentHashes)
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
                $validationExecutionCount++
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
            worktreeRoot = if ($null -eq $sourceResult) {
                [System.IO.Path]::GetFullPath([string]$Request.worktreeRoot)
            }
            else {
                [string]$sourceResult.WorktreeRoot
            }
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
            selectionSha256 = if ($null -eq $sourceResult -or [string]::IsNullOrWhiteSpace([string]$sourceResult.SelectionSha256)) { $null } else { [string]$sourceResult.SelectionSha256 }
            resolvedCaseCount = if ($null -eq $sourceResult) { 0 } else { [int]$sourceResult.ResolvedCaseCount }
        }
        stageResults = @($stageResults)
        evidenceReuse = [ordered]@{
            reusedIds = @($evidenceDisposition.ReusedIds)
            invalidated = @($evidenceDisposition.Invalidated)
        }
        stateTransitionMap = @($stateTransitionMap)
        profileResults = [ordered]@{
            IntendedBaseResult = [string]$profileResults.IntendedBaseResult
            MergeBaseResult = [string]$profileResults.MergeBaseResult
            EffectivePRScopeResult = [string]$profileResults.EffectivePRScopeResult
            EffectivePRPatchHash = $profileResults.EffectivePRPatchHash
            IntegrationProjectionResult = [string]$profileResults.IntegrationProjectionResult
            IntegrationProjectionHash = $profileResults.IntegrationProjectionHash
            AuthorizedWriteSetResult = [string]$profileResults.AuthorizedWriteSetResult
            ForeignProtectedStateResult = [string]$profileResults.ForeignProtectedStateResult
        }
        runnerProcessStartCount = 0
        validationExecutionCount = $validationExecutionCount
        infrastructureOrInvocationFailureCount = $infrastructureOrInvocationFailureCount
        fullMatrixRunCount = $fullMatrixRunCount
        packageWriteAttemptCount = 0
        generatedTaskControllerFileCount = [int]$controllerResult.GeneratedTaskControllerFileCount
        generatedTaskControllerLineCount = [int]$controllerResult.GeneratedTaskControllerLineCount
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
