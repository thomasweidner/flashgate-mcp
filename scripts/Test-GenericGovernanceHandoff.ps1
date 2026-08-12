#requires -Version 7.6
[CmdletBinding()]
param(
    [Parameter(Mandatory)][string]$PackagePath,
    [string]$RepositoryRoot = (Split-Path -Parent $PSScriptRoot),
    [string]$AuthoritativeRepositoryRoot = $RepositoryRoot,
    [string]$ReportPath,
    [switch]$ReturnInsteadOfExit
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$status = 'FAIL'
$failureMessage = $null
$resolvedPackagePath = $null
$resolvedReportPath = $null
$checks = [System.Collections.Generic.List[object]]::new()

function Add-Check {
    param([string]$Id, [bool]$Passed, [string]$Evidence = '')
    [void]$checks.Add([pscustomobject]@{ Id = $Id; Result = if ($Passed) { 'PASS' } else { 'FAIL' }; Evidence = $Evidence })
}

function Get-ByteSha256 {
    param([byte[]]$Bytes)
    return [Convert]::ToHexString([System.Security.Cryptography.SHA256]::HashData($Bytes)).ToLowerInvariant()
}

function Get-StrictText {
    param([byte[]]$Bytes, [string]$Name)
    try {
        $text = [System.Text.UTF8Encoding]::new($false, $true).GetString($Bytes)
    }
    catch {
        throw "Invalid UTF-8 in $Name"
    }
    if ($text.Contains([char]0xFFFD) -or ($Bytes.Length -ge 3 -and
        $Bytes[0] -eq 0xEF -and $Bytes[1] -eq 0xBB -and $Bytes[2] -eq 0xBF)) {
        throw "Non-canonical UTF-8 in $Name"
    }
    return $text
}

function Assert-Schema {
    param([string]$Text, [string]$SchemaPath, [string]$Name)
    if (-not ($Text | Test-Json -SchemaFile $SchemaPath -ErrorAction Stop)) {
        throw "Schema validation failed: $Name"
    }
}

function Get-SingleContract {
    param([string]$Text, [string]$Kind)
    $begin = "<!-- BEGIN GOVERNANCE-$Kind-CONTRACT -->"
    $end = "<!-- END GOVERNANCE-$Kind-CONTRACT -->"
    if (([regex]::Matches($Text, [regex]::Escape($begin))).Count -ne 1 -or
        ([regex]::Matches($Text, [regex]::Escape($end))).Count -ne 1) {
        throw "$Kind contract markers must occur exactly once."
    }
    $pattern = [regex]::Escape($begin) + '\s*(?<json>\{.*?\})\s*' + [regex]::Escape($end)
    $match = [regex]::Match($Text, $pattern, [System.Text.RegularExpressions.RegexOptions]::Singleline)
    if (-not $match.Success) { throw "$Kind contract markers are malformed." }
    return $match.Groups['json'].Value
}

function Get-HostPathCandidates {
    param([string]$Text)

    $patterns = @(
        '(?<![A-Za-z0-9])(?<path>[A-Za-z]:\\[^\s<>"''`|]+)',
        '(?<!\\)(?<path>\\\\[^\s<>"''`|]+)',
        '(?<![A-Za-z0-9:/])(?<path>/(?:[A-Za-z0-9._~-]+/)+[A-Za-z0-9._~+%-]+)'
    )
    $candidates = [System.Collections.Generic.List[string]]::new()
    foreach ($pattern in $patterns) {
        foreach ($match in [regex]::Matches($Text, $pattern)) {
            $candidate = $match.Groups['path'].Value.TrimEnd('.', ',', ';', ':', ')', ']', '}')
            if ($candidate -ceq '/dev/null') { continue }
            $lineStart = $Text.LastIndexOf("`n", [Math]::Max(0, $match.Index - 1)) + 1
            $linePrefix = $Text.Substring($lineStart, $match.Index - $lineStart)
            if ($linePrefix -cin @('#!', '+#!', '-#!')) { continue }
            if (-not [string]::IsNullOrWhiteSpace($candidate) -and $candidate -notin $candidates) {
                [void]$candidates.Add($candidate)
            }
        }
    }
    return @($candidates)
}

function Invoke-GitText {
    param(
        [Parameter(Mandatory)][string]$Root,
        [Parameter(Mandatory)][string[]]$Argument,
        [switch]$AllowFailure
    )

    $allowed=if($AllowFailure){@(0,1)}else{@(0)}
    $result=Invoke-GenericGitBytes -Root $Root -Argument $Argument -AllowedExitCode $allowed
    return [pscustomobject]@{ExitCode=$result.ExitCode;StandardOutput=ConvertFrom-GenericStrictUtf8 -Bytes $result.Bytes -Label 'Git text output';StandardError=$result.StandardError}
}

function ConvertTo-CanonicalRepositoryIdentity {
    param([string]$Repository)

    $candidate = $Repository.Trim()
    if ($candidate -match '^git@github\.com:(?<path>.+)$') {
        $candidate = 'https://github.com/' + $Matches.path
    }
    elseif ($candidate -match '^ssh://git@github\.com/(?<path>.+)$') {
        $candidate = 'https://github.com/' + $Matches.path
    }
    return $candidate.TrimEnd('/')
}

function Invoke-GitBytes {
    param(
        [Parameter(Mandatory)][string]$Root,
        [Parameter(Mandatory)][string[]]$Argument,
        [int[]]$AllowedExitCode = @(0),
        [hashtable]$Environment = @{},
        [switch]$RepositoryPaths
    )
    return Invoke-GenericGitBytes -Root $Root -Argument $Argument -AllowedExitCode $AllowedExitCode -Environment $Environment -RepositoryPaths:$RepositoryPaths
}

function ConvertFrom-StrictUtf8Bytes {
    param([Parameter(Mandatory)][byte[]]$Bytes, [string]$Label = 'Git output')
    try {
        return [System.Text.UTF8Encoding]::new($false, $true).GetString($Bytes)
    }
    catch {
        throw "$Label is not strict UTF-8."
    }
}

function Split-NulTerminatedUtf8 {
    param([Parameter(Mandatory)][AllowEmptyCollection()][byte[]]$Bytes, [string]$Label)

    $records = [System.Collections.Generic.List[string]]::new()
    $start = 0
    for ($index = 0; $index -lt $Bytes.Length; $index++) {
        if ($Bytes[$index] -ne 0) { continue }
        $length = $index - $start
        $segment = [byte[]]::new($length)
        if ($length -gt 0) { [Array]::Copy($Bytes, $start, $segment, 0, $length) }
        [void]$records.Add((ConvertFrom-StrictUtf8Bytes -Bytes $segment -Label $Label))
        $start = $index + 1
    }
    if ($start -ne $Bytes.Length) { throw "$Label is not NUL terminated." }
    return @($records)
}

function Get-BaselineBlobEvidence {
    param(
        [Parameter(Mandatory)][string]$Root,
        [Parameter(Mandatory)][string]$Commit,
        [Parameter(Mandatory)][string]$Path
    )

    $tree = Invoke-GitBytes -Root $Root -Argument @('ls-tree', '-z', '--full-tree', $Commit, '--', $Path)
    $records = @(Split-NulTerminatedUtf8 -Bytes $tree.Bytes -Label 'git ls-tree output' | Where-Object { $_ -ne '' })
    if ($records.Count -ne 1) { throw "Baseline path is not exactly one Git tree entry: $Path" }
    $match = [regex]::Match($records[0], '^(?<mode>[0-7]{6}) blob (?<oid>[0-9a-f]{40})\t(?<path>.+)$')
    if (-not $match.Success -or $match.Groups['path'].Value -cne $Path) {
        throw "Baseline path is not a regular Git blob: $Path"
    }
    $mode = $match.Groups['mode'].Value
    if ($mode -notin @('100644', '100755')) { throw "Unsupported baseline file mode for ${Path}: $mode" }
    $blob = Invoke-GitBytes -Root $Root -Argument @('cat-file', 'blob', $match.Groups['oid'].Value)
    return [ordered]@{
        commit = $Commit
        mode = $mode
        modeSource = 'BASELINE_TREE'
        length = [int64]$blob.Bytes.Length
        sha256 = Get-ByteSha256 -Bytes $blob.Bytes
    }
}

function Get-CurrentPostimageEvidence {
    param(
        [Parameter(Mandatory)][string]$Root,
        [Parameter(Mandatory)][string]$Path,
        [string]$GitMode,
        [switch]$Untracked
    )

    $fullPath = Join-Path $Root ($Path.Replace('/', [System.IO.Path]::DirectorySeparatorChar))
    if (-not (Test-Path -LiteralPath $fullPath -PathType Leaf)) {
        throw "Current postimage is not a regular file: $Path"
    }
    $item = Get-Item -LiteralPath $fullPath -Force
    if (($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw "Current postimage is a reparse point or symbolic link: $Path"
    }
    $bytes = [System.IO.File]::ReadAllBytes($fullPath)
    if ($Untracked) {
        if ($IsWindows) {
            $mode = '100644'
            $modeSource = 'WINDOWS_REGULAR_FILE_NORMALIZED'
        }
        else {
            $unixMode = [System.IO.File]::GetUnixFileMode($fullPath)
            $executeMask = [System.IO.UnixFileMode]::UserExecute -bor
                [System.IO.UnixFileMode]::GroupExecute -bor [System.IO.UnixFileMode]::OtherExecute
            $mode = if (($unixMode -band $executeMask) -ne 0) { '100755' } else { '100644' }
            $modeSource = 'UNIX_EXECUTABLE_BIT_NORMALIZED'
        }
    }
    else {
        if ($GitMode -notin @('100644', '100755')) {
            throw "Unsupported current Git worktree mode for ${Path}: $GitMode"
        }
        $mode = $GitMode
        $modeSource = 'GIT_WORKTREE'
    }
    return [ordered]@{
        mode = $mode
        modeSource = $modeSource
        length = [int64]$bytes.Length
        sha256 = Get-ByteSha256 -Bytes $bytes
    }
}

function Get-UnstagedRenamePairs {
    param([Parameter(Mandatory)][string]$Root, [Parameter(Mandatory)][string]$BaselineCommit)
    return @(Get-GenericUnstagedRenamePairs -Root $Root -BaselineCommit $BaselineCommit)
}

function Get-AuthoritativeStatusEntries {
    param(
        [Parameter(Mandatory)][string]$Root,
        [Parameter(Mandatory)][string]$BaselineCommit
    )

    $renamePairs=@(Get-UnstagedRenamePairs -Root $Root -BaselineCommit $BaselineCommit)
    $renameByTarget=[System.Collections.Generic.Dictionary[string,object]]::new([System.StringComparer]::Ordinal)
    $renameSources=[System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::Ordinal)
    foreach($pair in $renamePairs){$renameByTarget.Add([string]$pair.Path,$pair);[void]$renameSources.Add([string]$pair.PreviousPath)}
    $consumedRenameTargets=[System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::Ordinal)
    $statusResult = Invoke-GitBytes -Root $Root -Argument @(
        'status', '--porcelain=v2', '-z', '--untracked-files=all'
    )
    $records = @(Split-NulTerminatedUtf8 -Bytes $statusResult.Bytes -Label 'git status --porcelain=v2 output')
    $entries = [System.Collections.Generic.List[object]]::new()
    for ($recordIndex = 0; $recordIndex -lt $records.Count; $recordIndex++) {
        $record = $records[$recordIndex]
        if ([string]::IsNullOrEmpty($record)) { continue }

        $path = $null
        $previousPath = $null
        $gitStatus = $null
        $tracked = $true
        $staged = $false
        $mode = $null
        if ($record.StartsWith('? ', [System.StringComparison]::Ordinal)) {
            $path = $record.Substring(2).Replace('\','/')
            if($renameByTarget.ContainsKey($path)){$pair=$renameByTarget[$path];$previousPath=[string]$pair.PreviousPath;$gitStatus='TRACKED_RENAMED';$tracked=$true;$mode=[string]$pair.Mode;[void]$consumedRenameTargets.Add($path)}
            else{$gitStatus='UNTRACKED';$tracked=$false}
        }
        elseif ($record.StartsWith('1 ', [System.StringComparison]::Ordinal)) {
            $fields = $record.Split(' ', 9, [System.StringSplitOptions]::None)
            if ($fields.Count -ne 9) { throw 'Unsupported ordinary porcelain-v2 status record.' }
            $xy = $fields[1]
            $path = $fields[8]
            $mode = $fields[5]
            $staged = $xy[0] -cne '.'
            $gitStatus = if ($xy.Contains('D')) { 'TRACKED_DELETED' }
                elseif ($xy.Contains('A')) { 'TRACKED_ADDED' }
                elseif ($xy.Contains('T')) { 'TRACKED_MODE_CHANGED' }
                else { 'TRACKED_MODIFIED' }
        }
        elseif ($record.StartsWith('2 ', [System.StringComparison]::Ordinal)) {
            $fields = $record.Split(' ', 10, [System.StringSplitOptions]::None)
            if ($fields.Count -ne 10) { throw 'Unsupported renamed porcelain-v2 status record.' }
            if (-not $fields[8].StartsWith('R', [System.StringComparison]::Ordinal)) {
                throw 'Porcelain-v2 copy records are not supported as renames.'
            }
            $xy = $fields[1]
            $path = $fields[9]
            $mode = $fields[5]
            $staged = $xy[0] -cne '.'
            $gitStatus = 'TRACKED_RENAMED'
            $recordIndex++
            if ($recordIndex -ge $records.Count -or [string]::IsNullOrEmpty($records[$recordIndex])) {
                throw 'Porcelain-v2 rename source record is missing.'
            }
            $previousPath = $records[$recordIndex]
        }
        elseif ($record.StartsWith('! ', [System.StringComparison]::Ordinal)) {
            continue
        }
        else {
            throw 'Unsupported porcelain-v2 status record type.'
        }

        $normalizedPath = $path.Replace('\', '/')
        $normalizedPreviousPath = if ($null -ne $previousPath) { $previousPath.Replace('\', '/') } else { $null }
        if($gitStatus -ceq 'TRACKED_DELETED' -and $renameSources.Contains($normalizedPath)){continue}
        $fullPath = Join-Path $Root ($normalizedPath.Replace('/', [System.IO.Path]::DirectorySeparatorChar))
        $entry = [ordered]@{
            Path = $normalizedPath
            GitStatus = $gitStatus
            Tracked = $tracked
            Staged = $staged
            PreviousPath = $normalizedPreviousPath
            Preimage = $null
            Postimage = $null
            PostimageAbsent = $false
        }
        switch ($gitStatus) {
            'TRACKED_DELETED' {
                if (Test-Path -LiteralPath $fullPath) { throw "Deleted path is still present: $normalizedPath" }
                $entry.Preimage = Get-BaselineBlobEvidence -Root $Root -Commit $BaselineCommit -Path $normalizedPath
                $entry.PostimageAbsent = $true
            }
            'TRACKED_RENAMED' {
                if ($normalizedPreviousPath -ceq $normalizedPath) { throw 'Rename source and target must differ.' }
                $entry.Preimage = Get-BaselineBlobEvidence -Root $Root -Commit $BaselineCommit -Path $normalizedPreviousPath
                $entry.Postimage = Get-CurrentPostimageEvidence -Root $Root -Path $normalizedPath -GitMode $mode
            }
            'UNTRACKED' {
                $entry.Postimage = Get-CurrentPostimageEvidence -Root $Root -Path $normalizedPath -Untracked
            }
            default {
                $entry.Postimage = Get-CurrentPostimageEvidence -Root $Root -Path $normalizedPath -GitMode $mode
            }
        }
        [void]$entries.Add([pscustomobject]$entry)
    }
    if($consumedRenameTargets.Count -ne $renamePairs.Count){throw 'Temporary rename pairs did not match the complete real Porcelain-v2 delete/untracked state.'}
    return @($entries)
}

function Get-ScopePathList {
    param([Parameter(Mandatory)][AllowEmptyCollection()][object[]]$Entry)

    return @(
        foreach ($item in $Entry) {
            if ([string]$item.gitStatus -ceq 'TRACKED_RENAMED') {
                [string]$item.previousPath
            }
            [string]$item.path
        }
    )
}

function Get-AuthoritativeDeltaBytes {
    param(
        [Parameter(Mandatory)][string]$Root,
        [Parameter(Mandatory)][string]$BaselineCommit,
        [Parameter(Mandatory)][object[]]$IncludedEntry,
        [AllowEmptyCollection()][object[]]$ExcludedEntry=@()
    )
    $script:lastGenericDeltaEvidence=Get-GenericDeltaEvidence -Root $Root -BaselineCommit $BaselineCommit -IncludedEntry $IncludedEntry -ExcludedEntry $ExcludedEntry
    return ,([byte[]]$script:lastGenericDeltaEvidence.Bytes)
}

# The shared helper is authoritative for every generic Git-evidence operation.
# These late-bound adapters replace the historical in-file implementations above
# without changing the validator's public call surface.
. (Join-Path $PSScriptRoot 'GenericGovernanceGitEvidence.ps1')
$script:lastGenericDeltaEvidence = $null

function Invoke-GitText {
    param([Parameter(Mandatory)][string]$Root,[Parameter(Mandatory)][string[]]$Argument,[switch]$AllowFailure)
    $allowed=if($AllowFailure){@(0,1,128)}else{@(0)}
    $result=Invoke-GenericGitBytes -Root $Root -Argument $Argument -AllowedExitCode $allowed
    return [pscustomobject]@{ExitCode=$result.ExitCode;StandardOutput=ConvertFrom-GenericStrictUtf8 -Bytes $result.Bytes -Label 'Git text output';StandardError=$result.StandardError}
}

function Invoke-GitBytes {
    param([Parameter(Mandatory)][string]$Root,[Parameter(Mandatory)][string[]]$Argument,[int[]]$AllowedExitCode=@(0),[hashtable]$Environment=@{},[switch]$RepositoryPaths)
    return Invoke-GenericGitBytes -Root $Root -Argument $Argument -AllowedExitCode $AllowedExitCode -Environment $Environment -RepositoryPaths:$RepositoryPaths
}

function Get-BaselineBlobEvidence {
    param([Parameter(Mandatory)][string]$Root,[Parameter(Mandatory)][string]$Commit,[Parameter(Mandatory)][string]$Path)
    return Get-GenericBaselineBlobEvidence -Root $Root -Commit $Commit -Path $Path
}

function Get-CurrentPostimageEvidence {
    param([Parameter(Mandatory)][string]$Root,[Parameter(Mandatory)][string]$Path,[string]$GitMode,[switch]$Untracked)
    return Get-GenericPostimageEvidence -Root $Root -Path $Path -GitMode $GitMode -Untracked:$Untracked
}

function Get-UnstagedRenamePairs {
    param([Parameter(Mandatory)][string]$Root,[Parameter(Mandatory)][string]$BaselineCommit)
    return @(Get-GenericUnstagedRenamePairs -Root $Root -BaselineCommit $BaselineCommit)
}

function Get-AuthoritativeStatusEntries {
    param([Parameter(Mandatory)][string]$Root,[Parameter(Mandatory)][string]$BaselineCommit)
    return @(Get-GenericStatusEvidence -Root $Root -BaselineCommit $BaselineCommit)
}

function Get-ScopePathList {
    param([Parameter(Mandatory)][AllowEmptyCollection()][object[]]$Entry)
    return @(Get-GenericScopePaths -Entry $Entry)
}

function Get-AuthoritativeDeltaBytes {
    param(
        [Parameter(Mandatory)][string]$Root,
        [Parameter(Mandatory)][string]$BaselineCommit,
        [Parameter(Mandatory)][object[]]$IncludedEntry,
        [AllowEmptyCollection()][object[]]$ExcludedEntry=@()
    )
    $script:lastGenericDeltaEvidence=Get-GenericDeltaEvidence -Root $Root -BaselineCommit $BaselineCommit -IncludedEntry $IncludedEntry -ExcludedEntry $ExcludedEntry
    return ,([byte[]]$script:lastGenericDeltaEvidence.Bytes)
}

try {
    Import-Module (Join-Path $PSScriptRoot 'GovernanceValidationOrchestration.psm1') -Force
    $resolvedRepositoryRoot = [System.IO.Path]::GetFullPath($RepositoryRoot).TrimEnd('\', '/')
    $resolvedAuthoritativeRepositoryRoot = [System.IO.Path]::GetFullPath($AuthoritativeRepositoryRoot).TrimEnd('\', '/')
    $resolvedPackagePath = [System.IO.Path]::GetFullPath($PackagePath)
    $artifactIsDirectory = Test-Path -LiteralPath $resolvedPackagePath -PathType Container
    $artifactIsZip = Test-Path -LiteralPath $resolvedPackagePath -PathType Leaf
    if (-not $artifactIsDirectory -and -not $artifactIsZip) {
        throw "Package or staging directory does not exist: $resolvedPackagePath"
    }
    $outsideRepository = -not $resolvedPackagePath.StartsWith(
        $resolvedRepositoryRoot + [System.IO.Path]::DirectorySeparatorChar,
        [System.StringComparison]::OrdinalIgnoreCase
    )
    Add-Check -Id 'GENERIC-PACKAGE-OUTSIDE-REPOSITORY' -Passed $outsideRepository
    if (-not $outsideRepository) { throw 'Generic handoff package must be outside the repository.' }

    Add-Type -AssemblyName System.IO.Compression
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $requiredNames = @()
    $entryBytes = @{}
    $entryText = @{}
    if ($artifactIsDirectory) {
        $rootItem = Get-Item -LiteralPath $resolvedPackagePath -Force
        if (($rootItem.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
            throw 'Staging directory must not be a reparse point.'
        }
        $files = @(Get-ChildItem -LiteralPath $resolvedPackagePath -File -Force)
        $directories = @(Get-ChildItem -LiteralPath $resolvedPackagePath -Directory -Force)
        if ($directories.Count -ne 0) {
            throw 'Staging directory must contain only canonical root files.'
        }
        $names = @($files | ForEach-Object Name)
        foreach ($file in $files) {
            if (($file.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
                throw "Staging member is a reparse point: $($file.Name)"
            }
            $bytes = [System.IO.File]::ReadAllBytes($file.FullName)
            if ($bytes.Length -eq 0) { throw "Empty staging member: $($file.Name)" }
            $entryBytes[$file.Name] = $bytes
            $entryText[$file.Name] = Get-StrictText -Bytes $bytes -Name $file.Name
        }
    }
    else {
        $stream = [System.IO.File]::OpenRead($resolvedPackagePath)
        try {
            $archive = [System.IO.Compression.ZipArchive]::new($stream, [System.IO.Compression.ZipArchiveMode]::Read, $false)
            try {
            $names = @($archive.Entries | ForEach-Object FullName)
            foreach ($entry in $archive.Entries) {
                $entryStream = $entry.Open()
                try {
                    $memory = [System.IO.MemoryStream]::new()
                    try {
                        $entryStream.CopyTo($memory)
                        $bytes = $memory.ToArray()
                    }
                    finally { $memory.Dispose() }
                }
                finally { $entryStream.Dispose() }
                if ($bytes.Length -eq 0) { throw "Empty package member: $($entry.FullName)" }
                $entryBytes[$entry.FullName] = $bytes
                $entryText[$entry.FullName] = Get-StrictText -Bytes $bytes -Name $entry.FullName
            }
            }
            finally { $archive.Dispose() }
        }
        finally { $stream.Dispose() }
    }

    if (@($names | Where-Object { $_ -ceq 'assignment-record.json' }).Count -ne 1) {
        throw 'Artifact must contain exactly one assignment-record.json discriminator.'
    }
    $assignmentDiscriminator = $entryText['assignment-record.json'] |
        ConvertFrom-Json -Depth 20 -DateKind String
    $profile = [string]$assignmentDiscriminator.profile
    $transitionType = [string]$assignmentDiscriminator.transitionType
    $isImplementationReview = $profile -ceq 'IMPLEMENTATION_TO_INDEPENDENT_FULL_REVIEW'
    if ($profile -ceq 'GENERIC_COMMIT_PREPARATION' -and
        $transitionType -ceq 'COMMIT_PREPARATION_TO_COMMIT_APPROVAL') {
        $evidenceName = 'independent-review-evidence.json'
        $evidenceSchemaName = 'generic-independent-review-evidence.schema.json'
    }
    elseif ($isImplementationReview -and
        $transitionType -ceq 'IMPLEMENTATION_TO_INDEPENDENT_FULL_REVIEW') {
        $evidenceName = 'pre-review-validation-evidence.json'
        $evidenceSchemaName = 'generic-pre-review-validation-evidence.schema.json'
    }
    else {
        throw 'Unknown or mismatched explicit handoff profile and transition type.'
    }
    $requiredNames = @(
        'HANDOFF.md', 'assignment-record.json', 'completion-report.json',
        'current-delta.patch', $evidenceName,
        'MANIFEST.sha256', 'package-inventory.json', 'report.md',
        'scope-inventory.json', 'task.patch', 'validation-summary.json'
    )

    $safeNames = @($names | Where-Object {
            $_ -match '^[A-Za-z0-9][A-Za-z0-9._-]*$' -and
            -not $_.Contains('/') -and -not $_.Contains('\')
        })
    $shapePass = (
        $names.Count -eq $requiredNames.Count -and
        ($names | Sort-Object -Unique).Count -eq $names.Count -and
        (($names | Sort-Object) -join "`n") -ceq (($requiredNames | Sort-Object) -join "`n") -and
        $safeNames.Count -eq $names.Count
    )
    Add-Check -Id 'GENERIC-PACKAGE-SHAPE' -Passed $shapePass -Evidence ($names -join ',')
    if (-not $shapePass) { throw 'Artifact entries are missing, extra, duplicated, or unsafe.' }
    Add-Check -Id 'GENERIC-ARTIFACT-KIND' -Passed $true -Evidence $(if ($artifactIsDirectory) { 'DIRECTORY' } else { 'ZIP' })

            $correctionOnlyMembers = @(
                'correction-only.patch', 'finding-correction-matrix.json',
                'finding-regression-matrix.json', 'focused-delta-review-record.json'
            )
            $profileMemberIsolationPass = @($names | Where-Object { $_ -in $correctionOnlyMembers }).Count -eq 0
            Add-Check -Id 'GENERIC-PROFILE-MEMBER-ISOLATION' -Passed $profileMemberIsolationPass
            if (-not $profileMemberIsolationPass) {
                throw 'Generic package contains a correction-profile package member.'
            }

            $governanceRoot = Join-Path $resolvedRepositoryRoot 'Governance'
            $assignment = Read-GovernanceJsonContract `
                -Bytes ([byte[]]$entryBytes['assignment-record.json']) `
                -Label 'assignment-record.json' `
                -SchemaPath (Join-Path $governanceRoot 'generic-assignment-record.schema.json') `
                -ExpectedProfile $profile
            $completion = Read-GovernanceJsonContract `
                -Bytes ([byte[]]$entryBytes['completion-report.json']) `
                -Label 'completion-report.json' `
                -SchemaPath (Join-Path $governanceRoot 'generic-completion-report.schema.json') `
                -ExpectedProfile $profile
            $review = Read-GovernanceJsonContract `
                -Bytes ([byte[]]$entryBytes[$evidenceName]) `
                -Label $evidenceName `
                -SchemaPath (Join-Path $governanceRoot $evidenceSchemaName) `
                -ExpectedProfile $profile
            $validation = Read-GovernanceTypedResult `
                -Bytes ([byte[]]$entryBytes['validation-summary.json']) `
                -Label 'validation-summary.json' `
                -SchemaPath (Join-Path $governanceRoot 'generic-validation-summary.schema.json') `
                -ExpectedProfile $profile
            $scope = Read-GovernanceJsonContract `
                -Bytes ([byte[]]$entryBytes['scope-inventory.json']) `
                -Label 'scope-inventory.json' `
                -SchemaPath (Join-Path $governanceRoot 'generic-scope-inventory.schema.json') `
                -ExpectedProfile $profile
            $inventory = Read-GovernanceJsonContract `
                -Bytes ([byte[]]$entryBytes['package-inventory.json']) `
                -Label 'package-inventory.json' `
                -SchemaPath (Join-Path $governanceRoot 'generic-package-inventory.schema.json') `
                -ExpectedProfile $profile

            $trustedRepository = 'https://github.com/thomasweidner/flashgate-mcp.git'
            $trustedRepositoryPass = @($assignment, $completion, $review, $scope | Where-Object {
                    [string]$_.repository -cne $trustedRepository
                }).Count -eq 0
            Add-Check -Id 'GENERIC-TRUSTED-REPOSITORY' -Passed $trustedRepositoryPass
            if (-not $trustedRepositoryPass) {
                throw 'Generic package is not bound to the trusted repository identity.'
            }
            $stagedScopeProhibitionPass = @($scope.entries | Where-Object { [bool]$_.staged }).Count -eq 0
            Add-Check -Id 'GENERIC-STAGED-SCOPE-PROHIBITION' -Passed $stagedScopeProhibitionPass
            if (-not $stagedScopeProhibitionPass) {
                throw 'Generic package declares a staged scope path.'
            }

            $schemaMap = [ordered]@{
                'assignment-record.json' = 'generic-assignment-record.schema.json'
                'completion-report.json' = 'generic-completion-report.schema.json'
                $evidenceName = $evidenceSchemaName
                'scope-inventory.json' = 'generic-scope-inventory.schema.json'
                'validation-summary.json' = 'generic-validation-summary.schema.json'
                'package-inventory.json' = 'generic-package-inventory.schema.json'
            }
            foreach ($name in $schemaMap.Keys) {
                Assert-Schema -Text $entryText[$name] -SchemaPath (Join-Path $governanceRoot $schemaMap[$name]) -Name $name
            }
            Add-Check -Id 'GENERIC-JSON-SCHEMAS' -Passed $true

            $correctionOnlyFields = @(
                'correctionMode', 'targetFindings', 'pendingFindings',
                'closedFindings', 'correctionPatchArtifact',
                'findingCorrectionMatrix', 'findingRegressionMatrix',
                'focusedDeltaReviewRecord'
            )
            $profileTypedIsolationPass = (
                [string]$assignment.profile -ceq $profile -and
                [string]$assignment.transitionType -ceq $transitionType -and
                [string]$completion.profile -ceq $profile -and
                [string]$completion.transitionType -ceq $transitionType
            )
            foreach ($typedContract in @($assignment, $completion, $review, $scope, $validation, $inventory)) {
                $profileTypedIsolationPass = $profileTypedIsolationPass -and
                    @($typedContract.PSObject.Properties.Name | Where-Object { $_ -in $correctionOnlyFields }).Count -eq 0
            }
            Add-Check -Id 'GENERIC-PROFILE-TYPED-ISOLATION' -Passed $profileTypedIsolationPass
            if (-not $profileTypedIsolationPass) {
                throw 'Generic package contains a correction-profile discriminator or typed field.'
            }

            $scannedArtifacts = @(
                'HANDOFF.md', 'assignment-record.json', 'completion-report.json',
                'current-delta.patch', $evidenceName,
                'report.md', 'task.patch', 'validation-summary.json'
            )
            $hostPathFreeArtifacts = @($scope.hostPathPolicy.hostPathFreeArtifacts | ForEach-Object { [string]$_ })
            $allowedHostReferences = @($scope.hostPathPolicy.allowedReferences)
            $hostPathPolicyPass = $true
            $candidateMap = @{}
            foreach ($artifactName in $scannedArtifacts) {
                $artifactReferences = @($allowedHostReferences | Where-Object { [string]$_.artifact -ceq $artifactName })
                $isHostPathFree = $artifactName -in $hostPathFreeArtifacts
                if ($isHostPathFree -eq ($artifactReferences.Count -gt 0)) {
                    $hostPathPolicyPass = $false
                }
                $candidates = @(Get-HostPathCandidates -Text $entryText[$artifactName])
                $candidateMap[$artifactName] = $candidates
                if ($isHostPathFree -and $candidates.Count -gt 0) {
                    $hostPathPolicyPass = $false
                }
                foreach ($candidate in $candidates) {
                    $privateUserPath = (
                        $candidate -match '(?i)^[A-Z]:\\Users\\[^\\]+(?:\\|$)' -or
                        $candidate -match '^/home/[^/]+(?:/|$)' -or
                        $candidate -match '^/Users/[^/]+(?:/|$)' -or
                        $candidate -match '^/tmp(?:/|$)'
                    )
                    $matchingReferences = @($artifactReferences | Where-Object { [string]$_.path -ceq $candidate })
                    if ($privateUserPath -or $matchingReferences.Count -ne 1) {
                        $hostPathPolicyPass = $false
                    }
                }
            }
            foreach ($reference in $allowedHostReferences) {
                $artifactCandidates = @($candidateMap[[string]$reference.artifact])
                $referenceAllowed = (
                    [string]$reference.path -in $artifactCandidates -and
                    -not ([string]$reference.path -match '(?i)^[A-Z]:\\Users\\[^\\]+(?:\\|$)') -and
                    -not ([string]$reference.path -match '^/home/[^/]+(?:/|$)') -and
                    -not ([string]$reference.path -match '^/Users/[^/]+(?:/|$)') -and
                    -not ([string]$reference.path -match '^/tmp(?:/|$)')
                )
                if (-not $referenceAllowed) {
                    $hostPathPolicyPass = $false
                }
            }
            Add-Check -Id 'GENERIC-CLASSIFIED-HOST-PATH-POLICY' -Passed $hostPathPolicyPass
            if (-not $hostPathPolicyPass) {
                throw 'Classified host-path policy failed.'
            }

            $scopeInventoryHash = Get-ByteSha256 -Bytes $entryBytes['scope-inventory.json']
            $taskPatchHash = Get-ByteSha256 -Bytes $entryBytes['task.patch']
            $currentDeltaHash = Get-ByteSha256 -Bytes $entryBytes['current-delta.patch']
            $identityPass = (
                [string]$assignment.taskId -ceq [string]$completion.taskId -and
                [string]$assignment.taskId -ceq [string]$review.taskId -and
                [string]$assignment.taskId -ceq [string]$validation.taskId -and
                [string]$assignment.taskId -ceq [string]$scope.taskId -and
                [string]$assignment.taskId -ceq [string]$inventory.taskId -and
                [string]$assignment.profile -ceq $profile -and
                [string]$assignment.transitionType -ceq $transitionType
            )
            $currentStatePass = $true
            foreach ($binding in @($assignment, $completion, $review)) {
                $currentStatePass = $currentStatePass -and
                    [string]$binding.repository -ceq [string]$scope.repository -and
                    [string]$binding.baselineCommit -ceq [string]$scope.baselineCommit -and
                    [string]$binding.currentCommit -ceq [string]$scope.currentCommit -and
                    [string]$binding.branch -ceq [string]$scope.branch -and
                    [string]$binding.scopeInventorySha256 -ceq $scopeInventoryHash -and
                    ((@($binding.allowedDeltaPaths | Sort-Object) -join "`n") -ceq (@($scope.allowedDeltaPaths | Sort-Object) -join "`n")) -and
                    ((@($binding.excludedDeltaPaths | Sort-Object) -join "`n") -ceq (@($scope.excludedDeltaPaths | Sort-Object) -join "`n"))
            }
            foreach ($binding in @($assignment, $completion)) {
                $currentStatePass = $currentStatePass -and
                    [string]$binding.taskPatchSha256 -ceq $taskPatchHash -and
                    [string]$binding.currentDeltaSha256 -ceq $currentDeltaHash
            }
            $findingJson = @($assignment.findingIds) | ConvertTo-Json -Compress
            $findingParityPass = (
                $findingJson -ceq (@($completion.findingIds) | ConvertTo-Json -Compress) -and
                $findingJson -ceq (@($review.findingIds) | ConvertTo-Json -Compress)
            )
            Add-Check -Id 'GENERIC-IDENTITY-PARITY' -Passed $identityPass
            Add-Check -Id 'GENERIC-CURRENT-STATE-BINDING' -Passed $currentStatePass
            Add-Check -Id 'GENERIC-FINDING-PARITY' -Passed $findingParityPass
            if (-not $identityPass -or -not $currentStatePass -or -not $findingParityPass) {
                throw 'Generic identity, current-state, or finding parity failed.'
            }

            $authoritativeTopLevel = (Invoke-GitText -Root $resolvedAuthoritativeRepositoryRoot -Argument @(
                    'rev-parse', '--show-toplevel'
                )).StandardOutput.Trim()
            $authoritativeHead = (Invoke-GitText -Root $resolvedAuthoritativeRepositoryRoot -Argument @(
                    'rev-parse', 'HEAD'
                )).StandardOutput.Trim()
            $authoritativeBranch = (Invoke-GitText -Root $resolvedAuthoritativeRepositoryRoot -Argument @(
                    'branch', '--show-current'
                )).StandardOutput.Trim()
            $authoritativeOrigin = (Invoke-GitText -Root $resolvedAuthoritativeRepositoryRoot -Argument @(
                    'config', '--get', 'remote.origin.url'
                )).StandardOutput.Trim()
            $baselineObject = Invoke-GitText -Root $resolvedAuthoritativeRepositoryRoot -Argument @(
                'cat-file', '-e', "$($scope.baselineCommit)^{commit}"
            ) -AllowFailure
            $authoritativeIdentityPass = (
                [System.IO.Path]::GetFullPath($authoritativeTopLevel).TrimEnd('\', '/') -ceq $resolvedAuthoritativeRepositoryRoot -and
                $authoritativeHead -ceq [string]$scope.currentCommit -and
                $authoritativeBranch -ceq [string]$scope.branch -and
                (ConvertTo-CanonicalRepositoryIdentity -Repository $authoritativeOrigin) -ceq
                    (ConvertTo-CanonicalRepositoryIdentity -Repository ([string]$scope.repository)) -and
                $baselineObject.ExitCode -eq 0
            )
            Add-Check -Id 'GENERIC-AUTHORITATIVE-REPOSITORY-IDENTITY' -Passed $authoritativeIdentityPass
            if (-not $authoritativeIdentityPass) {
                throw 'Authoritative repository, baseline, current commit, or branch identity failed.'
            }

            $includedEntries = @($scope.entries | Where-Object { [string]$_.inclusionDecision -ceq 'INCLUDE' })
            $excludedEntries = @($scope.entries | Where-Object { [string]$_.inclusionDecision -ceq 'EXCLUDE' })
            $scopeEntryPaths = @($scope.entries | ForEach-Object { [string]$_.path })
            $allScopePaths = @(Get-ScopePathList -Entry @($scope.entries))
            $includedScopePaths = @(Get-ScopePathList -Entry $includedEntries | Sort-Object)
            $excludedScopePaths = @(Get-ScopePathList -Entry $excludedEntries | Sort-Object)
            $scopeMetadataPass = (
                @($scopeEntryPaths | Sort-Object -Unique).Count -eq $scopeEntryPaths.Count -and
                @($allScopePaths | Sort-Object -Unique).Count -eq $allScopePaths.Count -and
                (($includedScopePaths -join "`n") -ceq (@($scope.allowedDeltaPaths | Sort-Object) -join "`n")) -and
                (($excludedScopePaths -join "`n") -ceq (@($scope.excludedDeltaPaths | Sort-Object) -join "`n"))
            )
            foreach ($scopeEntry in @($scope.entries)) {
                $statusTrackedParity = if ([string]$scopeEntry.gitStatus -ceq 'UNTRACKED') {
                    -not [bool]$scopeEntry.tracked
                }
                else {
                    [bool]$scopeEntry.tracked
                }
                $scopeMetadataPass = $scopeMetadataPass -and
                    -not [bool]$scopeEntry.staged -and
                    $statusTrackedParity -and
                    -not [string]::IsNullOrWhiteSpace([string]$scopeEntry.reason) -and
                    ([string]$scopeEntry.gitStatus -cne 'TRACKED_RENAMED' -or
                        [string]$scopeEntry.previousPath -cne [string]$scopeEntry.path)
            }
            $emptyGitEvidence = [pscustomobject]@{
                RealObjectDatabaseImmutable = $false
                TemporaryArtifactsRemoved = $true
                RealObjectInventoryBeforeSha256 = ''
                RealObjectInventoryAfterSha256 = ''
                LiteralPathspecBinding = $false
                ActualDeltaInventoryParity = $false
                ExcludedDeltaPathProhibition = $false
            }
            $authoritativePatchBytes = [byte[]]@()
            $script:lastGenericDeltaEvidence = $emptyGitEvidence
            try {
                $authoritativePatchBytes = Get-AuthoritativeDeltaBytes `
                    -Root $resolvedAuthoritativeRepositoryRoot -BaselineCommit ([string]$scope.baselineCommit) `
                    -IncludedEntry $includedEntries -ExcludedEntry $excludedEntries
            }
            catch {
                $script:lastGenericDeltaEvidence = $emptyGitEvidence
            }
            try {
                $packagePatchEvidence = Get-GenericPatchDeltaEvidence `
                    -Root $resolvedAuthoritativeRepositoryRoot -BaselineCommit ([string]$scope.baselineCommit) `
                    -PatchBytes ([byte[]]$entryBytes['current-delta.patch']) `
                    -IncludedEntry $includedEntries -ExcludedEntry $excludedEntries
            }
            catch {
                $packagePatchEvidence = $emptyGitEvidence
            }
            $realObjectDatabasePass = [bool]$script:lastGenericDeltaEvidence.RealObjectDatabaseImmutable -and
                [bool]$script:lastGenericDeltaEvidence.TemporaryArtifactsRemoved -and
                [string]$script:lastGenericDeltaEvidence.RealObjectInventoryBeforeSha256 -ceq
                    [string]$script:lastGenericDeltaEvidence.RealObjectInventoryAfterSha256 -and
                [bool]$packagePatchEvidence.RealObjectDatabaseImmutable -and
                [bool]$packagePatchEvidence.TemporaryArtifactsRemoved -and
                [string]$packagePatchEvidence.RealObjectInventoryBeforeSha256 -ceq
                    [string]$packagePatchEvidence.RealObjectInventoryAfterSha256
            $literalPathspecPass = [bool]$script:lastGenericDeltaEvidence.LiteralPathspecBinding -and
                [bool]$packagePatchEvidence.LiteralPathspecBinding
            $actualDeltaInventoryPass = [bool]$script:lastGenericDeltaEvidence.ActualDeltaInventoryParity -and
                [bool]$packagePatchEvidence.ActualDeltaInventoryParity
            $excludedDeltaPathPass = [bool]$script:lastGenericDeltaEvidence.ExcludedDeltaPathProhibition -and
                [bool]$packagePatchEvidence.ExcludedDeltaPathProhibition
            Add-Check -Id 'GENERIC-REAL-OBJECT-DATABASE-IMMUTABILITY' -Passed $realObjectDatabasePass
            Add-Check -Id 'GENERIC-LITERAL-PATHSPEC-BINDING' -Passed $literalPathspecPass
            Add-Check -Id 'GENERIC-EXCLUDED-DELTA-PATH-PROHIBITION' -Passed $excludedDeltaPathPass
            Add-Check -Id 'GENERIC-ACTUAL-DELTA-INVENTORY-PARITY' -Passed $actualDeltaInventoryPass
            $packagePatchesEqual = [System.Linq.Enumerable]::SequenceEqual(
                [byte[]]$entryBytes['task.patch'], [byte[]]$entryBytes['current-delta.patch']
            )
            $authoritativePatchEqual = [System.Linq.Enumerable]::SequenceEqual(
                [byte[]]$entryBytes['current-delta.patch'], [byte[]]$authoritativePatchBytes
            )
            $scopePass = (
                $packagePatchesEqual -and $authoritativePatchEqual -and
                $authoritativePatchBytes.Length -gt 0 -and $includedScopePaths.Count -gt 0 -and
                @($includedScopePaths | Where-Object { $_ -in $excludedScopePaths }).Count -eq 0 -and
                $scopeMetadataPass
            )
            Add-Check -Id 'GENERIC-SCOPE-METADATA-PARITY' -Passed $scopeMetadataPass
            Add-Check -Id 'GENERIC-PATCH-SCOPE-PARITY' -Passed $scopePass

            $authoritativeEntries = @(Get-AuthoritativeStatusEntries `
                -Root $resolvedAuthoritativeRepositoryRoot -BaselineCommit ([string]$scope.baselineCommit))
            $authoritativePaths = @($authoritativeEntries | ForEach-Object Path | Sort-Object)
            $authoritativeScopePass = (
                @($authoritativePaths | Sort-Object -Unique).Count -eq $authoritativePaths.Count -and
                ($authoritativePaths -join "`n") -ceq (@($scopeEntryPaths | Sort-Object) -join "`n")
            )
            foreach ($scopeEntry in @($scope.entries)) {
                $matches = @($authoritativeEntries | Where-Object Path -CEQ ([string]$scopeEntry.path))
                if ($matches.Count -ne 1) {
                    $authoritativeScopePass = $false
                    continue
                }
                $actual = $matches[0]
                $authoritativeScopePass = $authoritativeScopePass -and
                    [string]$scopeEntry.gitStatus -ceq [string]$actual.GitStatus -and
                    [bool]$scopeEntry.tracked -eq [bool]$actual.Tracked -and
                    [bool]$scopeEntry.staged -eq [bool]$actual.Staged
                switch ([string]$scopeEntry.gitStatus) {
                    'TRACKED_DELETED' {
                        $authoritativeScopePass = $authoritativeScopePass -and
                            [bool]$scopeEntry.postimageAbsent -and [bool]$actual.PostimageAbsent -and
                            [string]$scopeEntry.preimage.commit -ceq [string]$actual.Preimage.commit -and
                            [string]$scopeEntry.preimage.mode -ceq [string]$actual.Preimage.mode -and
                            [string]$scopeEntry.preimage.modeSource -ceq [string]$actual.Preimage.modeSource -and
                            [int64]$scopeEntry.preimage.length -eq [int64]$actual.Preimage.length -and
                            [string]$scopeEntry.preimage.sha256 -ceq [string]$actual.Preimage.sha256
                    }
                    'TRACKED_RENAMED' {
                        $authoritativeScopePass = $authoritativeScopePass -and
                            [string]$scopeEntry.previousPath -ceq [string]$actual.PreviousPath -and
                            [string]$scopeEntry.preimage.commit -ceq [string]$actual.Preimage.commit -and
                            [string]$scopeEntry.preimage.mode -ceq [string]$actual.Preimage.mode -and
                            [string]$scopeEntry.preimage.modeSource -ceq [string]$actual.Preimage.modeSource -and
                            [int64]$scopeEntry.preimage.length -eq [int64]$actual.Preimage.length -and
                            [string]$scopeEntry.preimage.sha256 -ceq [string]$actual.Preimage.sha256 -and
                            [string]$scopeEntry.postimage.mode -ceq [string]$actual.Postimage.mode -and
                            [string]$scopeEntry.postimage.modeSource -ceq [string]$actual.Postimage.modeSource -and
                            [int64]$scopeEntry.postimage.length -eq [int64]$actual.Postimage.length -and
                            [string]$scopeEntry.postimage.sha256 -ceq [string]$actual.Postimage.sha256
                    }
                    default {
                        $authoritativeScopePass = $authoritativeScopePass -and
                            [string]$scopeEntry.postimage.mode -ceq [string]$actual.Postimage.mode -and
                            [string]$scopeEntry.postimage.modeSource -ceq [string]$actual.Postimage.modeSource -and
                            [int64]$scopeEntry.postimage.length -eq [int64]$actual.Postimage.length -and
                            [string]$scopeEntry.postimage.sha256 -ceq [string]$actual.Postimage.sha256
                    }
                }
            }
            Add-Check -Id 'GENERIC-AUTHORITATIVE-SCOPE-BINDING' -Passed $authoritativeScopePass
            if (-not $realObjectDatabasePass -or -not $literalPathspecPass -or
                -not $actualDeltaInventoryPass -or -not $excludedDeltaPathPass -or
                -not $scopePass -or -not $authoritativeScopePass) {
                throw 'Authoritative Git isolation, literal path, patch scope, actual delta inventory, or worktree binding failed.'
            }

            if ($isImplementationReview) {
                $fullCompletionEvidenceHash = Get-ByteSha256 -Bytes $entryBytes[$evidenceName]
                $reviewHashPass = (
                    [string]$review.independentReviewStatus -ceq 'NOT_PERFORMED' -and
                    [string]$review.fullCompletionStatus -ceq 'PASS' -and
                    [bool]$review.fullCompletionEvidenceReused -and
                    -not [bool]$review.fullCompletionReexecuted -and
                    -not [bool]$review.externalArtifactRequired -and
                    [int]$review.stagePassed -eq [int]$review.stageSelected -and
                    [int]$review.stageSelected -gt 0 -and
                    [int]$review.packageWriteAttemptCountBeforeHandoff -eq 0 -and
                    [string]$assignment.fullCompletionEvidenceSha256 -ceq $fullCompletionEvidenceHash -and
                    [string]$completion.fullCompletionEvidenceSha256 -ceq $fullCompletionEvidenceHash -and
                    [string]$assignment.fullCompletionResultSha256 -ceq [string]$review.fullCompletionResultSha256 -and
                    [string]$completion.fullCompletionResultSha256 -ceq [string]$review.fullCompletionResultSha256 -and
                    [string]$assignment.executionEnvelopeSha256 -ceq [string]$review.executionEnvelopeSha256 -and
                    [string]$completion.executionEnvelopeSha256 -ceq [string]$review.executionEnvelopeSha256
                )
                Add-Check -Id 'GENERIC-PRE-REVIEW-VALIDATION-EVIDENCE' -Passed $reviewHashPass
                if (-not $reviewHashPass) { throw 'Pre-review validation evidence parity failed.' }
            }
            else {
                $reviewedNames = @($review.reviewedArtifacts | ForEach-Object { [string]$_.path })
                $reviewHashPass = (
                    ($reviewedNames | Sort-Object -Unique).Count -eq 2 -and
                    'task.patch' -in $reviewedNames -and 'current-delta.patch' -in $reviewedNames
                )
                foreach ($reviewed in @($review.reviewedArtifacts)) {
                    $reviewHashPass = $reviewHashPass -and
                        [string]$reviewed.sha256 -ceq (Get-ByteSha256 -Bytes $entryBytes[[string]$reviewed.path])
                }
                Add-Check -Id 'GENERIC-INDEPENDENT-REVIEW-HASHES' -Passed $reviewHashPass
                if (-not $reviewHashPass) { throw 'Independent-review path or hash parity failed.' }
            }

            $inventoryExpectedNames = @($requiredNames | Where-Object { $_ -notin @('package-inventory.json', 'MANIFEST.sha256') } | Sort-Object)
            $inventoryNames = @($inventory.entries | ForEach-Object { [string]$_.path } | Sort-Object)
            $inventoryPass = ($inventoryNames -join "`n") -ceq ($inventoryExpectedNames -join "`n")
            foreach ($item in @($inventory.entries)) {
                $inventoryPass = $inventoryPass -and
                    [string]$item.sha256 -ceq (Get-ByteSha256 -Bytes $entryBytes[[string]$item.path]) -and
                    [int64]$item.length -eq $entryBytes[[string]$item.path].Length
            }
            Add-Check -Id 'GENERIC-INVENTORY-PARITY' -Passed $inventoryPass
            if (-not $inventoryPass) { throw 'Package inventory parity failed.' }

            $manifestRecords = @{}
            foreach ($line in @($entryText['MANIFEST.sha256'] -split "`n" | Where-Object { $_ -ne '' })) {
                if ($line -notmatch '^(?<hash>[0-9a-f]{64})  (?<size>0|[1-9][0-9]*)  (?<path>[A-Za-z0-9][A-Za-z0-9._-]*)$' -or
                    $manifestRecords.ContainsKey($Matches.path)) {
                    throw 'Manifest syntax or uniqueness failed.'
                }
                $manifestRecords[$Matches.path] = [pscustomobject]@{
                    Hash = $Matches.hash
                    Size = [int64]$Matches.size
                }
            }
            $manifestExpectedNames = @($requiredNames | Where-Object { $_ -cne 'MANIFEST.sha256' } | Sort-Object)
            $manifestPass = (($manifestRecords.Keys | Sort-Object) -join "`n") -ceq ($manifestExpectedNames -join "`n")
            foreach ($name in $manifestExpectedNames) {
                $manifestPass = $manifestPass -and
                    [string]$manifestRecords[$name].Hash -ceq (Get-ByteSha256 -Bytes $entryBytes[$name]) -and
                    [int64]$manifestRecords[$name].Size -eq $entryBytes[$name].LongLength
            }
            Add-Check -Id 'GENERIC-MANIFEST-PARITY' -Passed $manifestPass
            if (-not $manifestPass) { throw 'Manifest parity failed.' }

            $handoffJson = Get-SingleContract -Text $entryText['HANDOFF.md'] -Kind 'HANDOFF'
            Assert-Schema -Text $handoffJson -SchemaPath (Join-Path $governanceRoot 'generic-handoff-contract.schema.json') -Name 'HANDOFF contract'
            $handoff = $handoffJson | ConvertFrom-Json -Depth 100 -DateKind String
            $reportJson = Get-SingleContract -Text $entryText['report.md'] -Kind 'REPORT'
            Assert-Schema -Text $reportJson -SchemaPath (Join-Path $governanceRoot 'generic-report-contract.schema.json') -Name 'report contract'
            $report = $reportJson | ConvertFrom-Json -Depth 100 -DateKind String
            $reviewStatus = if ($isImplementationReview) {
                [string]$review.independentReviewStatus
            }
            else {
                [string]$review.result
            }
            $warningInvariantPass = (
                [int]$validation.observedWarningCount -eq ([int]$validation.resolvedWarningCount + [int]$validation.openWarningCount) -and
                [int]$completion.observedWarningCount -eq ([int]$completion.resolvedWarningCount + [int]$completion.openWarningCount) -and
                [int]$validation.warningCount -eq [int]$validation.openWarningCount -and
                [int]$completion.warningCount -eq [int]$completion.openWarningCount -and
                [int]$report.observedWarningCount -eq [int]$completion.observedWarningCount -and
                [int]$report.resolvedWarningCount -eq [int]$completion.resolvedWarningCount -and
                [int]$report.openWarningCount -eq [int]$completion.openWarningCount
            )
            $executionCounterParityPass = (
                [int]$validation.materialCorrectionCycleCount -eq [int]$completion.materialCorrectionCycleCount -and
                [int]$report.materialCorrectionCycleCount -eq [int]$completion.materialCorrectionCycleCount -and
                [int]$validation.validationExecutionCount -eq [int]$completion.validationExecutionCount -and
                [int]$report.validationExecutionCount -eq [int]$completion.validationExecutionCount -and
                [int]$validation.infrastructureOrInvocationFailureCount -eq [int]$completion.infrastructureOrInvocationFailureCount -and
                [int]$report.infrastructureOrInvocationFailureCount -eq [int]$completion.infrastructureOrInvocationFailureCount
            )
            $progressStatusTotal = @(
                'PASS', 'FAIL', 'SKIPPED', 'BLOCKED', 'CANCELLED', 'PENDING', 'NOT_RUN' |
                    ForEach-Object { [int]$validation.progress.statusCounts.$_ }
            ) | Measure-Object -Sum | Select-Object -ExpandProperty Sum
            $progressCheckCounts = @{}
            foreach ($statusName in @('PASS', 'FAIL', 'SKIPPED', 'BLOCKED', 'CANCELLED', 'PENDING', 'NOT_RUN')) {
                $progressCheckCounts[$statusName] = @($validation.checks | Where-Object { [string]$_.result -ceq $statusName }).Count
            }
            $progressInvariantPass = (
                [int]$validation.progress.completed -le [int]$validation.progress.selected -and
                [int]$validation.progress.completed -eq [int]$progressStatusTotal -and
                @('PASS', 'FAIL', 'SKIPPED', 'BLOCKED', 'CANCELLED', 'PENDING', 'NOT_RUN' | Where-Object {
                    [int]$validation.progress.statusCounts.$_ -ne [int]$progressCheckCounts[$_]
                }).Count -eq 0 -and
                [string]$validation.progress.message -clike "$($validation.progress.completed)/$($validation.progress.selected) $($validation.progress.unit)*"
            )
            $failedCheckCount = @($validation.checks | Where-Object { [string]$_.result -ceq 'FAIL' }).Count
            $failureCountInvariantPass = (
                [int]$validation.failureCount -eq $failedCheckCount -and
                ([string]$validation.result -cne 'BLOCKED' -or [int]$validation.failureCount -eq 0)
            )
            $progressEvents = @($validation.progressEvents)
            $progressEventInvariantPass = $progressEvents.Count -gt 0
            $terminalStatuses = @('PASS', 'FAIL', 'SKIPPED', 'BLOCKED', 'CANCELLED')
            for ($eventIndex = 0; $eventIndex -lt $progressEvents.Count; $eventIndex++) {
                $event = $progressEvents[$eventIndex]
                $progressEventInvariantPass = $progressEventInvariantPass -and
                    [int]$event.sequence -eq ($eventIndex + 1) -and
                    [int]$event.completed -le [int]$event.selected -and
                    [int]$event.selected -eq [int]$validation.progress.selected -and
                    [string]$event.unit -ceq [string]$validation.progress.unit
                if ($eventIndex -gt 0) {
                    $previousEvent = $progressEvents[$eventIndex - 1]
                    $currentSignature = @(
                        $event.caseId, $event.eventType, $event.status, $event.completed,
                        $event.selected, $event.unit, $event.phase, $event.elapsedMilliseconds
                    ) -join "`n"
                    $previousSignature = @(
                        $previousEvent.caseId, $previousEvent.eventType, $previousEvent.status, $previousEvent.completed,
                        $previousEvent.selected, $previousEvent.unit, $previousEvent.phase, $previousEvent.elapsedMilliseconds
                    ) -join "`n"
                    $progressEventInvariantPass = $progressEventInvariantPass -and $currentSignature -cne $previousSignature
                    switch ([string]$event.eventType) {
                        'PROGRESS' {
                            $progressEventInvariantPass = $progressEventInvariantPass -and [int]$event.completed -gt [int]$previousEvent.completed
                        }
                        'PHASE_CHANGE' {
                            $progressEventInvariantPass = $progressEventInvariantPass -and [string]$event.phase -cne [string]$previousEvent.phase
                        }
                        'STATUS_CHANGE' {
                            $progressEventInvariantPass = $progressEventInvariantPass -and (
                                [string]$event.status -cne [string]$previousEvent.status -or
                                [string]$event.caseId -cne [string]$previousEvent.caseId
                            )
                        }
                        'HEARTBEAT' {
                            $progressEventInvariantPass = $progressEventInvariantPass -and
                                ([int64]$event.elapsedMilliseconds - [int64]$previousEvent.elapsedMilliseconds) -ge
                                    [int64]$validation.progress.heartbeatIntervalMilliseconds
                        }
                        default { $progressEventInvariantPass = $false }
                    }
                }
            }
            foreach ($validationCheck in @($validation.checks)) {
                $terminalEventCount = @($progressEvents | Where-Object {
                    [string]$_.caseId -ceq [string]$validationCheck.id -and
                    [string]$_.eventType -ceq 'STATUS_CHANGE' -and
                    [string]$_.status -in $terminalStatuses
                }).Count
                if ([string]$validationCheck.result -in $terminalStatuses) {
                    $progressEventInvariantPass = $progressEventInvariantPass -and
                        $terminalEventCount -eq 1 -and
                        @($progressEvents | Where-Object {
                            [string]$_.caseId -ceq [string]$validationCheck.id -and
                            [string]$_.eventType -ceq 'STATUS_CHANGE' -and
                            [string]$_.status -ceq [string]$validationCheck.result
                        }).Count -eq 1
                }
                else {
                    $progressEventInvariantPass = $progressEventInvariantPass -and $terminalEventCount -eq 0
                }
            }
            $zipFreeReadinessPass = (
                [bool]$completion.packageGeneration.freshStaging -and
                [int]$completion.packageGeneration.finalZipWriteCount -eq 1 -and
                -not [bool]$completion.packageGeneration.inPlaceRepairPerformed -and
                [bool]$report.zipFreeReadinessPassed -eq [bool]$completion.zipFreeReadinessPassed -and
                (-not [bool]$completion.classicReviewReady -or [bool]$completion.zipFreeReadinessPassed)
            )
            Add-Check -Id 'GENERIC-WARNING-INVARIANT' -Passed $warningInvariantPass
            Add-Check -Id 'GENERIC-EXECUTION-COUNTER-PARITY' -Passed $executionCounterParityPass
            Add-Check -Id 'GENERIC-PROGRESS-INVARIANT' -Passed $progressInvariantPass
            Add-Check -Id 'GENERIC-FAILURE-COUNT-INVARIANT' -Passed $failureCountInvariantPass
            Add-Check -Id 'GENERIC-PROGRESS-EVENT-INVARIANT' -Passed $progressEventInvariantPass
            Add-Check -Id 'GENERIC-ZIP-FREE-READINESS' -Passed $zipFreeReadinessPass
            if (-not $warningInvariantPass -or -not $executionCounterParityPass -or -not $progressInvariantPass -or
                -not $failureCountInvariantPass -or -not $progressEventInvariantPass -or -not $zipFreeReadinessPass) {
                throw 'Telemetry, progress, warning, or ZIP-free readiness invariants failed.'
            }
            $contractParityPass = (
                [string]$handoff.taskId -ceq [string]$assignment.taskId -and
                [string]$report.taskId -ceq [string]$assignment.taskId -and
                [string]$handoff.repository -ceq [string]$assignment.repository -and
                [string]$report.repository -ceq [string]$assignment.repository -and
                [string]$handoff.baselineCommit -ceq [string]$assignment.baselineCommit -and
                [string]$report.baselineCommit -ceq [string]$assignment.baselineCommit -and
                [string]$handoff.currentCommit -ceq [string]$assignment.currentCommit -and
                [string]$report.currentCommit -ceq [string]$assignment.currentCommit -and
                [string]$handoff.branch -ceq [string]$assignment.branch -and
                [string]$report.branch -ceq [string]$assignment.branch -and
                [string]$handoff.status -ceq [string]$completion.status -and
                [string]$report.status -ceq [string]$completion.status -and
                [bool]$handoff.classicReviewReady -eq [bool]$completion.classicReviewReady -and
                [bool]$report.classicReviewReady -eq [bool]$completion.classicReviewReady -and
                [string]$handoff.reviewStatus -ceq $reviewStatus -and
                [string]$report.reviewStatus -ceq $reviewStatus -and
                [int]$report.materialCorrectionCycleCount -eq [int]$completion.materialCorrectionCycleCount -and
                [int]$report.validationExecutionCount -eq [int]$completion.validationExecutionCount -and
                [int]$report.infrastructureOrInvocationFailureCount -eq [int]$completion.infrastructureOrInvocationFailureCount -and
                [string]$handoff.scopeInventorySha256 -ceq $scopeInventoryHash -and
                [string]$report.scopeInventorySha256 -ceq $scopeInventoryHash -and
                [string]$handoff.taskPatchSha256 -ceq $taskPatchHash -and
                [string]$report.taskPatchSha256 -ceq $taskPatchHash -and
                [string]$handoff.currentDeltaSha256 -ceq $currentDeltaHash -and
                [string]$report.currentDeltaSha256 -ceq $currentDeltaHash -and
                ((@($handoff.allowedDeltaPaths | Sort-Object) -join "`n") -ceq (@($scope.allowedDeltaPaths | Sort-Object) -join "`n")) -and
                ((@($report.allowedDeltaPaths | Sort-Object) -join "`n") -ceq (@($scope.allowedDeltaPaths | Sort-Object) -join "`n")) -and
                ((@($handoff.excludedDeltaPaths | Sort-Object) -join "`n") -ceq (@($scope.excludedDeltaPaths | Sort-Object) -join "`n")) -and
                ((@($report.excludedDeltaPaths | Sort-Object) -join "`n") -ceq (@($scope.excludedDeltaPaths | Sort-Object) -join "`n")) -and
                -not [bool]$handoff.commitAuthorized -and -not [bool]$report.commitAuthorized
            )
            if ($isImplementationReview) {
                $contractParityPass = $contractParityPass -and
                    [string]$handoff.fullCompletionEvidenceSha256 -ceq $fullCompletionEvidenceHash -and
                    [string]$report.fullCompletionEvidenceSha256 -ceq $fullCompletionEvidenceHash -and
                    [string]$handoff.fullCompletionResultSha256 -ceq [string]$review.fullCompletionResultSha256 -and
                    [string]$report.fullCompletionResultSha256 -ceq [string]$review.fullCompletionResultSha256 -and
                    [string]$handoff.executionEnvelopeSha256 -ceq [string]$review.executionEnvelopeSha256 -and
                    [string]$report.executionEnvelopeSha256 -ceq [string]$review.executionEnvelopeSha256
            }
            $readinessPass = if ([bool]$completion.classicReviewReady) {
                ($isImplementationReview -or [string]$review.result -ceq 'PASS') -and
                (-not $isImplementationReview -or (
                    [string]$review.independentReviewStatus -ceq 'NOT_PERFORMED' -and
                    [string]$review.fullCompletionStatus -ceq 'PASS' -and
                    [bool]$review.fullCompletionEvidenceReused -and
                    -not [bool]$review.fullCompletionReexecuted
                )) -and
                [string]$validation.result -ceq 'PASS' -and
                [bool]$completion.zipFreeReadinessPassed -and
                [bool]$report.zipFreeReadinessPassed -and
                [int]$completion.openWarningCount -eq 0 -and
                [int]$validation.openWarningCount -eq 0 -and
                [int]$completion.warningCount -eq 0 -and [int]$completion.failureCount -eq 0 -and
                [int]$validation.warningCount -eq 0 -and [int]$validation.failureCount -eq 0
            }
            else { $true }
            Add-Check -Id 'GENERIC-CONTRACT-PARITY' -Passed $contractParityPass
            Add-Check -Id 'GENERIC-CLASSIC-READINESS' -Passed $readinessPass
            if (-not $contractParityPass -or -not $readinessPass) { throw 'Contract or readiness parity failed.' }

            $visibleLines = @(
                "TaskId: $($handoff.taskId)"
                "TransitionType: $($handoff.transitionType)"
                "Profile: $($handoff.profile)"
                "Status: $($handoff.status)"
                "ClassicReviewReady: $(([string]$handoff.classicReviewReady).ToLowerInvariant())"
                "FindingCount: $(@($handoff.findingIds).Count)"
                "ReviewStatus: $($handoff.reviewStatus)"
                'CommitAuthorized: false'
                "AllowedDeltaPaths: $(@($handoff.allowedDeltaPaths) -join ',')"
                "NextAction: $($handoff.nextAction)"
            )
            $normalizedHandoff = $entryText['HANDOFF.md'].Replace("`r`n", "`n")
            $expectedVisible = (
                @('<!-- BEGIN GOVERNANCE-HANDOFF-STATUS -->') +
                @($visibleLines) +
                @('<!-- END GOVERNANCE-HANDOFF-STATUS -->')
            ) -join "`n"
            $visiblePass = (
                ([regex]::Matches($normalizedHandoff, [regex]::Escape('<!-- BEGIN GOVERNANCE-HANDOFF-STATUS -->'))).Count -eq 1 -and
                ([regex]::Matches($normalizedHandoff, [regex]::Escape('<!-- END GOVERNANCE-HANDOFF-STATUS -->'))).Count -eq 1 -and
                $normalizedHandoff.Contains($expectedVisible, [System.StringComparison]::Ordinal)
            )
            Add-Check -Id 'GENERIC-HANDOFF-VISIBLE-PARITY' -Passed $visiblePass
            if (-not $visiblePass) { throw 'Visible HANDOFF parity failed.' }

            $status = 'PASS'
}
catch {
    $failureMessage = $_.Exception.Message
}
finally {
    if (-not [string]::IsNullOrWhiteSpace($ReportPath)) {
        try {
            $resolvedReportPath = [System.IO.Path]::GetFullPath($ReportPath)
            $reportDirectory = Split-Path -Parent $resolvedReportPath
            if (-not (Test-Path -LiteralPath $reportDirectory -PathType Container)) {
                [void][System.IO.Directory]::CreateDirectory($reportDirectory)
            }
            [System.IO.File]::WriteAllText(
                $resolvedReportPath,
                ([ordered]@{
                        schemaVersion = 1
                        status = $status
                        packagePath = $resolvedPackagePath
                        packageSha256 = if (Test-Path -LiteralPath $resolvedPackagePath -PathType Leaf) { (Get-FileHash -LiteralPath $resolvedPackagePath -Algorithm SHA256).Hash.ToLowerInvariant() } else { $null }
                        validationExecutionCount = 1
                        infrastructureOrInvocationFailureCount = [int](-not [string]::IsNullOrWhiteSpace($failureMessage))
                        fullMatrixRunCount = 0
                        packageWriteAttemptCount = 0
                        generatedTaskControllerFileCount = 0
                        generatedTaskControllerLineCount = 0
                        readOnlyProbeCount = $checks.Count
                        checks = @($checks)
                        failureMessage = $failureMessage
                    } | ConvertTo-Json -Depth 20),
                [System.Text.UTF8Encoding]::new($false)
            )
        }
        catch {
            $status = 'FAIL'
            $failureMessage = "Unable to write report: $($_.Exception.Message)"
        }
    }
    [pscustomobject]@{
        Status = $status
        PackagePath = $resolvedPackagePath
        CheckCount = $checks.Count
        FailureCount = @($checks | Where-Object Result -ceq 'FAIL').Count
        FailedChecks = @($checks | Where-Object Result -ceq 'FAIL' | ForEach-Object {
                "$($_.Id): $($_.Evidence)"
            }) -join '; '
        ValidationExecutionCount = 1
        InfrastructureOrInvocationFailureCount = [int](-not [string]::IsNullOrWhiteSpace($failureMessage))
        FullMatrixRunCount = 0
        PackageWriteAttemptCount = 0
        GeneratedTaskControllerFileCount = 0
        GeneratedTaskControllerLineCount = 0
        ReadOnlyProbeCount = $checks.Count
        ReportPath = $resolvedReportPath
        FailureMessage = $failureMessage
        NextAction = if ($status -ceq 'PASS') { 'Use the validated package at the authorized next checkpoint.' } else { 'Correct the failed generic handoff gate and generate a fresh package.' }
    } | Format-List
}

$terminalExitCode = if ($status -ceq 'PASS') { 0 } else { 1 }
if ($ReturnInsteadOfExit) {
    $global:LASTEXITCODE = $terminalExitCode
    return
}
exit $terminalExitCode
