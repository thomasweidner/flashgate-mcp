#requires -Version 7.6

Set-StrictMode -Version Latest

function Invoke-GenericGitBytes {
    param([Parameter(Mandatory)][string]$Root, [Parameter(Mandatory)][string[]]$Argument, [int[]]$AllowedExitCode = @(0), [hashtable]$Environment = @{})
    $startInfo = [System.Diagnostics.ProcessStartInfo]::new()
    $startInfo.FileName = (Get-Command git -ErrorAction Stop).Source
    $startInfo.UseShellExecute = $false
    $startInfo.RedirectStandardOutput = $true
    $startInfo.RedirectStandardError = $true
    [void]$startInfo.ArgumentList.Add('-C')
    [void]$startInfo.ArgumentList.Add($Root)
    foreach ($item in $Argument) { [void]$startInfo.ArgumentList.Add($item) }
    foreach ($name in $Environment.Keys) { $startInfo.Environment[[string]$name] = [string]$Environment[$name] }
    $process = [System.Diagnostics.Process]::new()
    $process.StartInfo = $startInfo
    $memory = [System.IO.MemoryStream]::new()
    try {
        [void]$process.Start()
        $stderrTask = $process.StandardError.ReadToEndAsync()
        $process.StandardOutput.BaseStream.CopyTo($memory)
        $process.WaitForExit()
        $stderr = $stderrTask.GetAwaiter().GetResult()
        if ($process.ExitCode -notin $AllowedExitCode) { throw "Git query failed with exit code $($process.ExitCode): $stderr" }
        return [pscustomobject]@{ ExitCode=$process.ExitCode; Bytes=$memory.ToArray(); StandardError=$stderr }
    }
    finally { $memory.Dispose(); $process.Dispose() }
}

function ConvertFrom-GenericStrictUtf8 {
    param([Parameter(Mandatory)][byte[]]$Bytes, [string]$Label='Git output')
    try { return [System.Text.UTF8Encoding]::new($false, $true).GetString($Bytes) }
    catch { throw "$Label is not strict UTF-8." }
}

function Split-GenericNulUtf8 {
    param([Parameter(Mandatory)][AllowEmptyCollection()][byte[]]$Bytes, [string]$Label)
    $records = [System.Collections.Generic.List[string]]::new()
    $start = 0
    for ($index=0; $index -lt $Bytes.Length; $index++) {
        if ($Bytes[$index] -ne 0) { continue }
        $length = $index - $start
        $segment = [byte[]]::new($length)
        if ($length -gt 0) { [Array]::Copy($Bytes, $start, $segment, 0, $length) }
        [void]$records.Add((ConvertFrom-GenericStrictUtf8 -Bytes $segment -Label $Label))
        $start = $index + 1
    }
    if ($start -ne $Bytes.Length) { throw "$Label is not NUL terminated." }
    return @($records)
}

function Get-GenericByteSha256 {
    param([Parameter(Mandatory)][byte[]]$Bytes)
    return [Convert]::ToHexString([System.Security.Cryptography.SHA256]::HashData($Bytes)).ToLowerInvariant()
}

function Get-GenericBaselineBlobEvidence {
    param([Parameter(Mandatory)][string]$Root, [Parameter(Mandatory)][string]$Commit, [Parameter(Mandatory)][string]$Path)
    $tree = Invoke-GenericGitBytes -Root $Root -Argument @('ls-tree','-z','--full-tree',$Commit,'--',$Path)
    $records = @(Split-GenericNulUtf8 -Bytes $tree.Bytes -Label 'git ls-tree output' | Where-Object { $_ -ne '' })
    if ($records.Count -ne 1) { throw "Baseline path is not exactly one Git tree entry: $Path" }
    $match = [regex]::Match($records[0], '^(?<mode>[0-7]{6}) blob (?<oid>[0-9a-f]{40})\t(?<path>.+)$')
    if (-not $match.Success -or $match.Groups['path'].Value -cne $Path) { throw "Baseline path is not a regular Git blob: $Path" }
    $mode = $match.Groups['mode'].Value
    if ($mode -notin @('100644','100755')) { throw "Unsupported baseline file mode for ${Path}: $mode" }
    $blob = Invoke-GenericGitBytes -Root $Root -Argument @('cat-file','blob',$match.Groups['oid'].Value)
    return [ordered]@{ commit=$Commit; mode=$mode; modeSource='BASELINE_TREE'; length=[int64]$blob.Bytes.Length; sha256=Get-GenericByteSha256 -Bytes $blob.Bytes }
}

function Get-GenericPostimageEvidence {
    param([Parameter(Mandatory)][string]$Root, [Parameter(Mandatory)][string]$Path, [string]$GitMode, [switch]$Untracked)
    $fullPath = Join-Path $Root ($Path.Replace('/', [System.IO.Path]::DirectorySeparatorChar))
    if (-not (Test-Path -LiteralPath $fullPath -PathType Leaf)) { throw "Current postimage is not a regular file: $Path" }
    $item = Get-Item -LiteralPath $fullPath -Force
    if (($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) { throw "Current postimage is a reparse point or symbolic link: $Path" }
    $bytes = [System.IO.File]::ReadAllBytes($fullPath)
    if ($Untracked) {
        if ($IsWindows) { $mode='100644'; $modeSource='WINDOWS_REGULAR_FILE_NORMALIZED' }
        else {
            $unixMode = [System.IO.File]::GetUnixFileMode($fullPath)
            $mask = [System.IO.UnixFileMode]::UserExecute -bor [System.IO.UnixFileMode]::GroupExecute -bor [System.IO.UnixFileMode]::OtherExecute
            $mode = if (($unixMode -band $mask) -ne 0) { '100755' } else { '100644' }
            $modeSource = 'UNIX_EXECUTABLE_BIT_NORMALIZED'
        }
    }
    else {
        if ($GitMode -notin @('100644','100755')) { throw "Unsupported current Git mode for ${Path}: $GitMode" }
        $mode=$GitMode; $modeSource='GIT_WORKTREE'
    }
    return [ordered]@{ mode=$mode; modeSource=$modeSource; length=[int64]$bytes.Length; sha256=Get-GenericByteSha256 -Bytes $bytes }
}

function Get-GenericUnstagedRenamePairs {
    param([Parameter(Mandatory)][string]$Root, [Parameter(Mandatory)][string]$BaselineCommit)
    $indexPath = Join-Path ([System.IO.Path]::GetTempPath()) ('flashgate-governance-index-' + [guid]::NewGuid().ToString('N'))
    $environment = @{ GIT_INDEX_FILE = $indexPath }
    try {
        $null = Invoke-GenericGitBytes -Root $Root -Argument @('read-tree',$BaselineCommit) -Environment $environment
        $null = Invoke-GenericGitBytes -Root $Root -Argument @('add','-A','--','.') -Environment $environment
        $diff = Invoke-GenericGitBytes -Root $Root -Argument @('diff','--cached','--name-status','-z','--find-renames','--diff-filter=R',$BaselineCommit,'--') -Environment $environment
        $records = @(Split-GenericNulUtf8 -Bytes $diff.Bytes -Label 'temporary-index rename diff')
        $pairs = [System.Collections.Generic.List[object]]::new()
        for ($index=0; $index -lt $records.Count; $index++) {
            if ([string]::IsNullOrEmpty($records[$index])) { continue }
            $status = $records[$index]
            if (-not $status.StartsWith('R',[System.StringComparison]::Ordinal) -or $index + 2 -ge $records.Count) { throw 'Malformed NUL-separated rename diff.' }
            $previousPath=$records[++$index].Replace('\','/')
            $path=$records[++$index].Replace('\','/')
            $stage = Invoke-GenericGitBytes -Root $Root -Argument @('ls-files','--stage','-z','--',$path) -Environment $environment
            $stageRecords=@(Split-GenericNulUtf8 -Bytes $stage.Bytes -Label 'temporary-index stage entry'|Where-Object{$_ -ne ''})
            if($stageRecords.Count -ne 1){throw "Temporary rename target has no unique stage entry: $path"}
            $match=[regex]::Match($stageRecords[0],'^(?<mode>[0-7]{6}) [0-9a-f]{40} 0\t(?<path>.+)$')
            if(-not $match.Success -or $match.Groups['path'].Value -cne $path){throw "Malformed temporary stage entry: $path"}
            [void]$pairs.Add([pscustomobject]@{PreviousPath=$previousPath;Path=$path;Mode=$match.Groups['mode'].Value})
        }
        return @($pairs)
    }
    finally {
        foreach($candidate in @($indexPath,$indexPath+'.lock')){if([System.IO.File]::Exists($candidate)){[System.IO.File]::Delete($candidate)}}
    }
}

function Get-GenericStatusEvidence {
    param([Parameter(Mandatory)][string]$Root, [Parameter(Mandatory)][string]$BaselineCommit)
    $renamePairs=@(Get-GenericUnstagedRenamePairs -Root $Root -BaselineCommit $BaselineCommit)
    $renameByTarget=[System.Collections.Generic.Dictionary[string,object]]::new([System.StringComparer]::Ordinal)
    $renameSources=[System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::Ordinal)
    foreach($pair in $renamePairs){$renameByTarget.Add([string]$pair.Path,$pair);[void]$renameSources.Add([string]$pair.PreviousPath)}
    $consumedRenameTargets=[System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::Ordinal)
    $status = Invoke-GenericGitBytes -Root $Root -Argument @('status','--porcelain=v2','-z','--untracked-files=all')
    $records = @(Split-GenericNulUtf8 -Bytes $status.Bytes -Label 'git status output')
    $entries = [System.Collections.Generic.List[object]]::new()
    for ($index=0; $index -lt $records.Count; $index++) {
        $record=$records[$index]
        if ([string]::IsNullOrEmpty($record)) { continue }
        $path=$null; $previousPath=$null; $gitStatus=$null; $tracked=$true; $staged=$false; $mode=$null
        if ($record.StartsWith('? ',[System.StringComparison]::Ordinal)) {
            $path=$record.Substring(2).Replace('\','/')
            if($renameByTarget.ContainsKey($path)){$pair=$renameByTarget[$path];$previousPath=[string]$pair.PreviousPath;$gitStatus='TRACKED_RENAMED';$tracked=$true;$mode=[string]$pair.Mode;[void]$consumedRenameTargets.Add($path)}
            else{$gitStatus='UNTRACKED';$tracked=$false}
        }
        elseif ($record.StartsWith('1 ',[System.StringComparison]::Ordinal)) {
            $fields=$record.Split(' ',9,[System.StringSplitOptions]::None)
            if ($fields.Count -ne 9) { throw 'Unsupported ordinary porcelain-v2 record.' }
            $xy=$fields[1]; $path=$fields[8]; $mode=$fields[5]; $staged=$xy[0] -cne '.'
            $gitStatus = if($xy.Contains('D')){'TRACKED_DELETED'}elseif($xy.Contains('A')){'TRACKED_ADDED'}elseif($xy.Contains('T')){'TRACKED_MODE_CHANGED'}else{'TRACKED_MODIFIED'}
        }
        elseif ($record.StartsWith('2 ',[System.StringComparison]::Ordinal)) {
            $fields=$record.Split(' ',10,[System.StringSplitOptions]::None)
            if ($fields.Count -ne 10 -or -not $fields[8].StartsWith('R',[System.StringComparison]::Ordinal)) { throw 'Unsupported rename/copy porcelain-v2 record.' }
            $xy=$fields[1]; $path=$fields[9]; $mode=$fields[5]; $staged=$xy[0] -cne '.'; $gitStatus='TRACKED_RENAMED'
            $index++
            if ($index -ge $records.Count -or [string]::IsNullOrEmpty($records[$index])) { throw 'Rename source record is missing.' }
            $previousPath=$records[$index]
        }
        elseif ($record.StartsWith('! ',[System.StringComparison]::Ordinal)) { continue }
        else { throw 'Unsupported porcelain-v2 record type.' }
        $path=$path.Replace('\','/'); if($null -ne $previousPath){$previousPath=$previousPath.Replace('\','/')}
        if($gitStatus -ceq 'TRACKED_DELETED' -and $renameSources.Contains($path)){continue}
        $fullPath=Join-Path $Root ($path.Replace('/',[System.IO.Path]::DirectorySeparatorChar))
        $entry=[ordered]@{ Path=$path; PreviousPath=$previousPath; GitStatus=$gitStatus; Tracked=$tracked; Staged=$staged; Preimage=$null; Postimage=$null; PostimageAbsent=$false }
        switch($gitStatus){
            'TRACKED_DELETED' { if(Test-Path -LiteralPath $fullPath){throw "Deleted path is still present: $path"}; $entry.Preimage=Get-GenericBaselineBlobEvidence -Root $Root -Commit $BaselineCommit -Path $path; $entry.PostimageAbsent=$true }
            'TRACKED_RENAMED' { if($previousPath -ceq $path){throw 'Rename paths must differ.'}; $entry.Preimage=Get-GenericBaselineBlobEvidence -Root $Root -Commit $BaselineCommit -Path $previousPath; $entry.Postimage=Get-GenericPostimageEvidence -Root $Root -Path $path -GitMode $mode }
            'UNTRACKED' { $entry.Postimage=Get-GenericPostimageEvidence -Root $Root -Path $path -Untracked }
            default { $entry.Postimage=Get-GenericPostimageEvidence -Root $Root -Path $path -GitMode $mode }
        }
        [void]$entries.Add([pscustomobject]$entry)
    }
    if($consumedRenameTargets.Count -ne $renamePairs.Count){throw 'Temporary rename pairs did not match the complete real Porcelain-v2 delete/untracked state.'}
    return @($entries)
}

function Get-GenericScopePaths {
    param([Parameter(Mandatory)][AllowEmptyCollection()][object[]]$Entry)
    return @(foreach($item in $Entry){if([string]$item.gitStatus -ceq 'TRACKED_RENAMED'){[string]$item.previousPath};[string]$item.path})
}

function Get-GenericDeltaBytes {
    param([Parameter(Mandatory)][string]$Root, [Parameter(Mandatory)][string]$BaselineCommit, [Parameter(Mandatory)][object[]]$IncludedEntry)
    $paths=@(Get-GenericScopePaths -Entry $IncludedEntry|Sort-Object -Unique)
    $indexPath=Join-Path ([System.IO.Path]::GetTempPath()) ('flashgate-governance-delta-index-'+[guid]::NewGuid().ToString('N'))
    $environment=@{GIT_INDEX_FILE=$indexPath}
    try{
        $null=Invoke-GenericGitBytes -Root $Root -Argument @('read-tree',$BaselineCommit) -Environment $environment
        $null=Invoke-GenericGitBytes -Root $Root -Argument @('add','-A','--','.') -Environment $environment
        return (Invoke-GenericGitBytes -Root $Root -Argument (@('diff','--cached','--binary','--find-renames',$BaselineCommit,'--')+$paths) -Environment $environment).Bytes
    }
    finally{foreach($candidate in @($indexPath,$indexPath+'.lock')){if([System.IO.File]::Exists($candidate)){[System.IO.File]::Delete($candidate)}}}
}
