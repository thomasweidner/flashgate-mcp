#requires -Version 7.6

Set-StrictMode -Version Latest

function Invoke-GenericGitBytes {
    param(
        [Parameter(Mandatory)][string]$Root,
        [Parameter(Mandatory)][string[]]$Argument,
        [int[]]$AllowedExitCode = @(0),
        [hashtable]$Environment = @{},
        [switch]$RepositoryPaths,
        [AllowEmptyCollection()][byte[]]$InputBytes
    )
    $boundEnvironment = @{}
    foreach ($name in $Environment.Keys) { $boundEnvironment[[string]$name] = [string]$Environment[$name] }
    if ($RepositoryPaths) {
        $separatorIndex = [Array]::IndexOf($Argument, '--')
        if ($separatorIndex -lt 0 -or $separatorIndex -ge $Argument.Count - 1) {
            throw '[GENERIC-LITERAL-PATHSPEC-BINDING] Repository paths must be separate arguments after --.'
        }
        foreach ($path in @($Argument[($separatorIndex + 1)..($Argument.Count - 1)])) {
            $null = Assert-GenericRepositoryPath -Path ([string]$path)
        }
        if ($boundEnvironment.ContainsKey('GIT_LITERAL_PATHSPECS') -and [string]$boundEnvironment.GIT_LITERAL_PATHSPECS -cne '1') {
            throw '[GENERIC-LITERAL-PATHSPEC-BINDING] A path-bound Git call attempted to disable literal pathspecs.'
        }
        $boundEnvironment.GIT_LITERAL_PATHSPECS = '1'
    }
    if ($boundEnvironment.ContainsKey('GIT_INDEX_FILE')) {
        Assert-GenericIsolatedGitEnvironment -Root $Root -Environment $boundEnvironment
    }
    $startInfo = [System.Diagnostics.ProcessStartInfo]::new()
    $startInfo.FileName = (Get-Command git -ErrorAction Stop).Source
    $startInfo.UseShellExecute = $false
    $startInfo.RedirectStandardOutput = $true
    $startInfo.RedirectStandardError = $true
    $startInfo.RedirectStandardInput = $PSBoundParameters.ContainsKey('InputBytes')
    [void]$startInfo.ArgumentList.Add('-C')
    [void]$startInfo.ArgumentList.Add($Root)
    foreach ($item in $Argument) { [void]$startInfo.ArgumentList.Add($item) }
    foreach ($name in $boundEnvironment.Keys) { $startInfo.Environment[[string]$name] = [string]$boundEnvironment[$name] }
    $process = [System.Diagnostics.Process]::new()
    $process.StartInfo = $startInfo
    $memory = [System.IO.MemoryStream]::new()
    try {
        [void]$process.Start()
        $stderrTask = $process.StandardError.ReadToEndAsync()
        if ($PSBoundParameters.ContainsKey('InputBytes')) {
            $process.StandardInput.BaseStream.Write($InputBytes,0,$InputBytes.Length)
            $process.StandardInput.Close()
        }
        $process.StandardOutput.BaseStream.CopyTo($memory)
        $process.WaitForExit()
        $stderr = $stderrTask.GetAwaiter().GetResult()
        if ($process.ExitCode -notin $AllowedExitCode) { throw "Git query failed with exit code $($process.ExitCode): $stderr" }
        return [pscustomobject]@{ ExitCode=$process.ExitCode; Bytes=$memory.ToArray(); StandardError=$stderr }
    }
    finally { $memory.Dispose(); $process.Dispose() }
}

function ConvertFrom-GenericStrictUtf8 {
    param([Parameter(Mandatory)][AllowEmptyCollection()][byte[]]$Bytes, [string]$Label='Git output')
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

function Assert-GenericRepositoryPath {
    param([Parameter(Mandatory)][AllowEmptyString()][string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path) -or $Path.Contains([char]0) -or $Path.Contains("`r") -or
        $Path.Contains("`n") -or $Path.Contains('\') -or $Path.StartsWith('/', [System.StringComparison]::Ordinal) -or
        $Path -match '^[A-Za-z]:' -or $Path.Contains('//')) {
        throw "[GENERIC-LITERAL-PATHSPEC-BINDING] Unsafe repository path: $Path"
    }
    $segments = @($Path.Split('/'))
    if ($segments.Count -eq 0 -or @($segments | Where-Object { $_ -in @('', '.', '..') }).Count -gt 0) {
        throw "[GENERIC-LITERAL-PATHSPEC-BINDING] Unsafe repository path segment: $Path"
    }
    return $Path
}

function Get-GenericRealObjectDirectory {
    param([Parameter(Mandatory)][string]$Root)
    $commonValue = (ConvertFrom-GenericStrictUtf8 -Bytes (
        Invoke-GenericGitBytes -Root $Root -Argument @('rev-parse','--git-common-dir')
    ).Bytes -Label 'git common directory').Trim()
    $objectValue = (ConvertFrom-GenericStrictUtf8 -Bytes (
        Invoke-GenericGitBytes -Root $Root -Argument @('rev-parse','--git-path','objects')
    ).Bytes -Label 'git object directory').Trim()
    if ([string]::IsNullOrWhiteSpace($commonValue) -or [string]::IsNullOrWhiteSpace($objectValue)) {
        throw '[GENERIC-REAL-OBJECT-DATABASE-IMMUTABILITY] Git did not resolve its common or object directory.'
    }
    $commonDirectory = if ([System.IO.Path]::IsPathRooted($commonValue)) {
        [System.IO.Path]::GetFullPath($commonValue)
    } else { [System.IO.Path]::GetFullPath((Join-Path $Root $commonValue)) }
    $objectDirectory = if ([System.IO.Path]::IsPathRooted($objectValue)) {
        [System.IO.Path]::GetFullPath($objectValue)
    } else { [System.IO.Path]::GetFullPath((Join-Path $Root $objectValue)) }
    $commonDirectory = $commonDirectory.TrimEnd('\','/')
    $objectDirectory = $objectDirectory.TrimEnd('\','/')
    if (-not [System.IO.Directory]::Exists($objectDirectory) -or
        -not $objectDirectory.StartsWith($commonDirectory + [System.IO.Path]::DirectorySeparatorChar, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw '[GENERIC-REAL-OBJECT-DATABASE-IMMUTABILITY] Git object directory is not a child of the common Git directory.'
    }
    return $objectDirectory
}

function Get-GenericObjectDirectoryInventory {
    param([Parameter(Mandatory)][string]$ObjectDirectory)
    $resolved = [System.IO.Path]::GetFullPath($ObjectDirectory).TrimEnd('\','/')
    if (-not [System.IO.Directory]::Exists($resolved)) {
        throw "[GENERIC-REAL-OBJECT-DATABASE-IMMUTABILITY] Object directory is missing: $resolved"
    }
    $entries = [System.Collections.Generic.List[object]]::new()
    foreach ($item in @(Get-ChildItem -LiteralPath $resolved -Force -Recurse | Sort-Object FullName)) {
        if (($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
            throw "[GENERIC-REAL-OBJECT-DATABASE-IMMUTABILITY] Reparse point in object directory: $($item.FullName)"
        }
        $relative = [System.IO.Path]::GetRelativePath($resolved,$item.FullName).Replace('\','/')
        if ($item.PSIsContainer) {
            [void]$entries.Add([pscustomobject][ordered]@{path=$relative;type='DIRECTORY';length=$null;sha256=$null})
        } else {
            [void]$entries.Add([pscustomobject][ordered]@{
                path=$relative;type='FILE';length=[int64]$item.Length
                sha256=(Get-FileHash -LiteralPath $item.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
            })
        }
    }
    $lines = @(foreach ($entry in $entries) {
        if ($entry.type -ceq 'DIRECTORY') { "D`t$($entry.path)" }
        else { "F`t$($entry.path)`t$($entry.length)`t$($entry.sha256)" }
    })
    $canonicalText = ($lines -join "`n") + "`n"
    return [pscustomobject][ordered]@{
        objectDirectory=$resolved;entryCount=$entries.Count
        fileCount=@($entries|Where-Object type -ceq 'FILE').Count
        directoryCount=@($entries|Where-Object type -ceq 'DIRECTORY').Count
        sha256=Get-GenericByteSha256 -Bytes ([System.Text.UTF8Encoding]::new($false).GetBytes($canonicalText))
        canonicalText=$canonicalText;entries=@($entries)
    }
}

function Get-GenericRealObjectInventory {
    param([Parameter(Mandatory)][string]$Root)
    return Get-GenericObjectDirectoryInventory -ObjectDirectory (Get-GenericRealObjectDirectory -Root $Root)
}

function Test-GenericObjectInventoryEqual {
    param([Parameter(Mandatory)][object]$Before,[Parameter(Mandatory)][object]$After)
    return [string]$Before.objectDirectory -ceq [string]$After.objectDirectory -and
        [string]$Before.sha256 -ceq [string]$After.sha256 -and
        [string]$Before.canonicalText -ceq [string]$After.canonicalText
}

function Assert-GenericIsolatedGitEnvironment {
    param([Parameter(Mandatory)][string]$Root,[Parameter(Mandatory)][hashtable]$Environment)
    foreach ($name in @('GIT_INDEX_FILE','GIT_OBJECT_DIRECTORY','GIT_LITERAL_PATHSPECS')) {
        if (-not $Environment.ContainsKey($name) -or [string]::IsNullOrWhiteSpace([string]$Environment[$name])) {
            throw "[GENERIC-REAL-OBJECT-DATABASE-IMMUTABILITY] Missing isolated Git environment value: $name"
        }
    }
    if ([string]$Environment.GIT_LITERAL_PATHSPECS -cne '1') {
        throw '[GENERIC-LITERAL-PATHSPEC-BINDING] GIT_LITERAL_PATHSPECS must equal 1.'
    }
    $realObjects = Get-GenericRealObjectDirectory -Root $Root
    $temporaryObjects = [System.IO.Path]::GetFullPath([string]$Environment.GIT_OBJECT_DIRECTORY).TrimEnd('\','/')
    if ($temporaryObjects -ceq $realObjects) {
        throw '[GENERIC-REAL-OBJECT-DATABASE-IMMUTABILITY] Temporary and real object directories must differ.'
    }
    $alternate = Join-Path (Join-Path $temporaryObjects 'info') 'alternates'
    if (-not [System.IO.File]::Exists($alternate)) {
        throw '[GENERIC-REAL-OBJECT-DATABASE-IMMUTABILITY] The temporary object database has no alternate binding.'
    }
    $alternateText = [System.Text.UTF8Encoding]::new($false,$true).GetString([System.IO.File]::ReadAllBytes($alternate))
    if ($alternateText -cne ($realObjects.Replace('\','/') + "`n")) {
        throw '[GENERIC-REAL-OBJECT-DATABASE-IMMUTABILITY] The temporary object database alternate is not bound to the real object directory.'
    }
}

function New-GenericGitIsolationContext {
    param([Parameter(Mandatory)][string]$Root)
    $temporaryRoot = Join-Path ([System.IO.Path]::GetTempPath()) ('flashgate-governance-git-'+[guid]::NewGuid().ToString('N'))
    try {
        $objects = Join-Path $temporaryRoot 'objects'
        $information = Join-Path $objects 'info'
        [System.IO.Directory]::CreateDirectory($information)|Out-Null
        $realObjects = Get-GenericRealObjectDirectory -Root $Root
        $alternate = Join-Path $information 'alternates'
        [System.IO.File]::WriteAllText($alternate,$realObjects.Replace('\','/')+"`n",[System.Text.UTF8Encoding]::new($false))
        $context = [pscustomobject][ordered]@{
            Root=$temporaryRoot;IndexPath=Join-Path $temporaryRoot 'index';ObjectDirectory=$objects
            RealObjectDirectory=$realObjects;AlternatePath=$alternate
            Environment=@{GIT_INDEX_FILE=Join-Path $temporaryRoot 'index';GIT_OBJECT_DIRECTORY=$objects;GIT_LITERAL_PATHSPECS='1';GIT_OPTIONAL_LOCKS='0'}
        }
        Assert-GenericIsolatedGitEnvironment -Root $Root -Environment $context.Environment
        return $context
    } catch {
        if ([System.IO.Directory]::Exists($temporaryRoot)) { Remove-Item -LiteralPath $temporaryRoot -Recurse -Force }
        throw
    }
}

function Remove-GenericGitIsolationContext {
    param([Parameter(Mandatory)][object]$Context)
    $resolved = [System.IO.Path]::GetFullPath([string]$Context.Root).TrimEnd('\','/')
    $parent = [System.IO.Path]::GetFullPath([System.IO.Path]::GetTempPath()).TrimEnd('\','/')+[System.IO.Path]::DirectorySeparatorChar
    if (-not $resolved.StartsWith($parent,[System.StringComparison]::OrdinalIgnoreCase) -or
        -not [System.IO.Path]::GetFileName($resolved).StartsWith('flashgate-governance-git-',[System.StringComparison]::Ordinal)) {
        throw "[GENERIC-REAL-OBJECT-DATABASE-IMMUTABILITY] Refusing unsafe isolation cleanup: $resolved"
    }
    if ([System.IO.Directory]::Exists($resolved)) {
        $item=Get-Item -LiteralPath $resolved -Force
        if (($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
            throw "[GENERIC-REAL-OBJECT-DATABASE-IMMUTABILITY] Refusing reparse-point cleanup: $resolved"
        }
        Remove-Item -LiteralPath $resolved -Recurse -Force
    }
    Assert-GenericGitIsolationRemoved -Context $Context
}

function Assert-GenericGitIsolationRemoved {
    param([Parameter(Mandatory)][object]$Context)
    $resolved=[System.IO.Path]::GetFullPath([string]$Context.Root).TrimEnd('\','/')
    if([System.IO.Directory]::Exists($resolved)-or[System.IO.File]::Exists($resolved)){
        throw "[GENERIC-REAL-OBJECT-DATABASE-IMMUTABILITY] Temporary Git isolation cleanup failed: $resolved"
    }
}

function Get-GenericStatusPathCandidates {
    param([Parameter(Mandatory)][string]$Root)
    $records=@(Split-GenericNulUtf8 -Bytes (Invoke-GenericGitBytes -Root $Root -Argument @('status','--porcelain=v2','-z','--untracked-files=all')).Bytes -Label 'git status path candidates')
    $paths=[System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::Ordinal)
    for($index=0;$index-lt$records.Count;$index++){
        $record=$records[$index]
        if([string]::IsNullOrEmpty($record)-or$record.StartsWith('! ',[System.StringComparison]::Ordinal)){continue}
        if($record.StartsWith('? ',[System.StringComparison]::Ordinal)){[void]$paths.Add((Assert-GenericRepositoryPath $record.Substring(2).Replace('\','/')));continue}
        if($record.StartsWith('1 ',[System.StringComparison]::Ordinal)){$fields=$record.Split(' ',9,[System.StringSplitOptions]::None);if($fields.Count-ne 9){throw 'Unsupported ordinary porcelain-v2 record.'};[void]$paths.Add((Assert-GenericRepositoryPath $fields[8].Replace('\','/')));continue}
        if($record.StartsWith('2 ',[System.StringComparison]::Ordinal)){$fields=$record.Split(' ',10,[System.StringSplitOptions]::None);if($fields.Count-ne 10){throw 'Unsupported rename/copy porcelain-v2 record.'};[void]$paths.Add((Assert-GenericRepositoryPath $fields[9].Replace('\','/')));$index++;if($index-ge$records.Count-or[string]::IsNullOrEmpty($records[$index])){throw 'Rename source record is missing.'};[void]$paths.Add((Assert-GenericRepositoryPath $records[$index].Replace('\','/')));continue}
        throw 'Unsupported porcelain-v2 record type.'
    }
    return @($paths|Sort-Object)
}

function ConvertFrom-GenericNameStatusZ {
    param([Parameter(Mandatory)][AllowEmptyCollection()][byte[]]$Bytes)
    $records=@(Split-GenericNulUtf8 -Bytes $Bytes -Label 'actual NUL-separated delta inventory')
    $entries=[System.Collections.Generic.List[object]]::new();$keys=[System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::Ordinal)
    for($index=0;$index-lt$records.Count;$index++){
        $status=$records[$index];if([string]::IsNullOrEmpty($status)){continue};$previousPath=$null;$path=$null
        if($status-match'^R[0-9]{1,3}$'){if($index+2-ge$records.Count){throw '[GENERIC-ACTUAL-DELTA-INVENTORY-PARITY] Malformed rename delta inventory.'};$previousPath=Assert-GenericRepositoryPath $records[++$index].Replace('\','/');$path=Assert-GenericRepositoryPath $records[++$index].Replace('\','/');$kind='RENAME';$key="R`t$previousPath`t$path"}
        elseif($status-in@('A','M','D','T')){if($index+1-ge$records.Count){throw '[GENERIC-ACTUAL-DELTA-INVENTORY-PARITY] Missing ordinary delta path.'};$path=Assert-GenericRepositoryPath $records[++$index].Replace('\','/');$kind=@{A='ADDED';M='MODIFIED';D='DELETED';T='TYPE_CHANGED'}[$status];$key="$status`t$path"}
        else{throw "[GENERIC-ACTUAL-DELTA-INVENTORY-PARITY] Unsupported delta status code: $status"}
        if(-not$keys.Add($key)){throw "[GENERIC-ACTUAL-DELTA-INVENTORY-PARITY] Duplicate delta inventory entry: $key"}
        [void]$entries.Add([pscustomobject][ordered]@{status=$status;kind=$kind;previousPath=$previousPath;path=$path;key=$key})
    }
    return @($entries)
}

function Get-GenericExpectedDeltaInventoryKeys {
    param([Parameter(Mandatory)][object[]]$IncludedEntry)
    $keys=[System.Collections.Generic.List[string]]::new()
    foreach($entry in $IncludedEntry){$path=Assert-GenericRepositoryPath ([string]$entry.path);$key=switch([string]$entry.gitStatus){'TRACKED_RENAMED'{$previous=Assert-GenericRepositoryPath ([string]$entry.previousPath);"R`t$previous`t$path"}'TRACKED_DELETED'{"D`t$path"}'TRACKED_ADDED'{"A`t$path"}'UNTRACKED'{"A`t$path"}'TRACKED_MODE_CHANGED'{"M`t$path"}'TRACKED_MODIFIED'{"M`t$path"}default{throw "[GENERIC-ACTUAL-DELTA-INVENTORY-PARITY] Unsupported scope status: $($entry.gitStatus)"}};[void]$keys.Add($key)}
    if(@($keys|Sort-Object -Unique).Count-ne$keys.Count){throw '[GENERIC-ACTUAL-DELTA-INVENTORY-PARITY] Duplicate expected delta inventory entry.'}
    return @($keys|Sort-Object)
}

function Get-GenericBaselineBlobEvidence {
    param([Parameter(Mandatory)][string]$Root, [Parameter(Mandatory)][string]$Commit, [Parameter(Mandatory)][string]$Path)
    $null = Assert-GenericRepositoryPath -Path $Path
    $tree = Invoke-GenericGitBytes -Root $Root -Argument @('ls-tree','-z','--full-tree',$Commit,'--',$Path) -RepositoryPaths
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
    $null = Assert-GenericRepositoryPath -Path $Path
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
    $paths=@(Get-GenericStatusPathCandidates -Root $Root)
    $beforeInventory=Get-GenericRealObjectInventory -Root $Root
    $context=New-GenericGitIsolationContext -Root $Root
    $pairs=[System.Collections.Generic.List[object]]::new()
    $operationError=$null
    try {
        $null=Invoke-GenericGitBytes -Root $Root -Argument @('read-tree',$BaselineCommit) -Environment $context.Environment
        if($paths.Count-gt 0){$null=Invoke-GenericGitBytes -Root $Root -Argument (@('add','-A','--')+$paths) -Environment $context.Environment -RepositoryPaths}
        $diff=Invoke-GenericGitBytes -Root $Root -Argument @('diff','--cached','--name-status','-z','--find-renames','--diff-filter=R',$BaselineCommit,'--') -Environment $context.Environment
        $records = @(Split-GenericNulUtf8 -Bytes $diff.Bytes -Label 'temporary-index rename diff')
        for ($index=0; $index -lt $records.Count; $index++) {
            if ([string]::IsNullOrEmpty($records[$index])) { continue }
            $status = $records[$index]
            if ($status -notmatch '^R[0-9]{1,3}$' -or $index + 2 -ge $records.Count) { throw 'Malformed NUL-separated rename diff.' }
            $previousPath=Assert-GenericRepositoryPath -Path $records[++$index].Replace('\','/')
            $path=Assert-GenericRepositoryPath -Path $records[++$index].Replace('\','/')
            $stage = Invoke-GenericGitBytes -Root $Root -Argument @('ls-files','--stage','-z','--',$path) -Environment $context.Environment -RepositoryPaths
            $stageRecords=@(Split-GenericNulUtf8 -Bytes $stage.Bytes -Label 'temporary-index stage entry'|Where-Object{$_ -ne ''})
            if($stageRecords.Count -ne 1){throw "Temporary rename target has no unique stage entry: $path"}
            $match=[regex]::Match($stageRecords[0],'^(?<mode>[0-7]{6}) [0-9a-f]{40} 0\t(?<path>.+)$')
            if(-not $match.Success -or $match.Groups['path'].Value -cne $path){throw "Malformed temporary stage entry: $path"}
            [void]$pairs.Add([pscustomobject]@{PreviousPath=$previousPath;Path=$path;Mode=$match.Groups['mode'].Value})
        }
    }
    catch{$operationError=$_}
    finally {
        $afterInventory=Get-GenericRealObjectInventory -Root $Root
        try{Remove-GenericGitIsolationContext -Context $context}catch{if($null-eq$operationError){$operationError=$_}}
    }
    if($null-ne$operationError){throw $operationError}
    if(-not(Test-GenericObjectInventoryEqual -Before $beforeInventory -After $afterInventory)){throw '[GENERIC-REAL-OBJECT-DATABASE-IMMUTABILITY] Rename evidence mutated the real Git object database.'}
    return @($pairs)
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
        $path=Assert-GenericRepositoryPath -Path $path.Replace('\','/'); if($null -ne $previousPath){$previousPath=Assert-GenericRepositoryPath -Path $previousPath.Replace('\','/')}
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

function Get-GenericDeltaEvidence {
    param(
        [Parameter(Mandatory)][string]$Root,
        [Parameter(Mandatory)][string]$BaselineCommit,
        [Parameter(Mandatory)][object[]]$IncludedEntry,
        [AllowEmptyCollection()][object[]]$ExcludedEntry=@()
    )
    $paths=@(Get-GenericScopePaths -Entry $IncludedEntry|Sort-Object -Unique)
    if($paths.Count-eq 0){throw '[GENERIC-ACTUAL-DELTA-INVENTORY-PARITY] At least one included repository path is required.'}
    $excludedPaths=@(Get-GenericScopePaths -Entry $ExcludedEntry|Sort-Object -Unique)
    $expectedKeys=@(Get-GenericExpectedDeltaInventoryKeys -IncludedEntry $IncludedEntry)
    $beforeInventory=Get-GenericRealObjectInventory -Root $Root
    $context=New-GenericGitIsolationContext -Root $Root
    $operationError=$null;$deltaBytes=[byte[]]::new(0);$actualEntries=@();$temporaryObjectFileCount=0;$cleanupPassed=$false
    try{
        $null=Invoke-GenericGitBytes -Root $Root -Argument @('read-tree',$BaselineCommit) -Environment $context.Environment
        $null=Invoke-GenericGitBytes -Root $Root -Argument (@('add','-A','--')+$paths) -Environment $context.Environment -RepositoryPaths
        $nameStatus=Invoke-GenericGitBytes -Root $Root -Argument (@('diff','--cached','--name-status','-z','--find-renames',$BaselineCommit,'--')+$paths) -Environment $context.Environment -RepositoryPaths
        $actualEntries=@(ConvertFrom-GenericNameStatusZ -Bytes $nameStatus.Bytes)
        $delta=Invoke-GenericGitBytes -Root $Root -Argument (@('diff','--cached','--binary','--find-renames',$BaselineCommit,'--')+$paths) -Environment $context.Environment -RepositoryPaths
        $deltaBytes=[byte[]]$delta.Bytes
        $temporaryInventory=Get-GenericObjectDirectoryInventory -ObjectDirectory $context.ObjectDirectory
        $temporaryObjectFileCount=@($temporaryInventory.entries|Where-Object{$_.type-ceq'FILE'-and$_.path-cne'info/alternates'}).Count
    }
    catch{$operationError=$_}
    finally{
        $afterInventory=Get-GenericRealObjectInventory -Root $Root
        try{Remove-GenericGitIsolationContext -Context $context;$cleanupPassed=$true}catch{if($null-eq$operationError){$operationError=$_}}
    }
    if($null-ne$operationError){throw $operationError}
    $objectImmutable=Test-GenericObjectInventoryEqual -Before $beforeInventory -After $afterInventory
    if(-not$objectImmutable){throw '[GENERIC-REAL-OBJECT-DATABASE-IMMUTABILITY] Authoritative delta generation mutated the real Git object database.'}
    if(-not$cleanupPassed){throw '[GENERIC-REAL-OBJECT-DATABASE-IMMUTABILITY] Temporary Git isolation cleanup did not complete.'}
    $actualKeys=@($actualEntries|ForEach-Object key|Sort-Object)
    $actualParity=($actualKeys-join"`n")-ceq($expectedKeys-join"`n")
    $actualPaths=@(foreach($entry in $actualEntries){if($entry.kind-ceq'RENAME'){[string]$entry.previousPath};[string]$entry.path})
    $excludedDeltaPaths=@($actualPaths|Where-Object{$_-cin$excludedPaths}|Sort-Object -Unique)
    return [pscustomobject][ordered]@{
        Bytes=$deltaBytes;ActualDeltaInventory=@($actualEntries);ExpectedDeltaInventoryKeys=$expectedKeys
        ActualDeltaInventoryParity=$actualParity;ExcludedDeltaPaths=$excludedDeltaPaths
        ExcludedDeltaPathProhibition=$excludedDeltaPaths.Count-eq 0;LiteralPathspecBinding=$true
        RealObjectDatabaseImmutable=$objectImmutable;RealObjectInventoryBeforeSha256=[string]$beforeInventory.sha256
        RealObjectInventoryAfterSha256=[string]$afterInventory.sha256;RealObjectInventoryEntryCount=[int]$beforeInventory.entryCount
        TemporaryObjectFileCount=$temporaryObjectFileCount;TemporaryArtifactsRemoved=$cleanupPassed
    }
}

function Get-GenericPatchDeltaEvidence {
    param(
        [Parameter(Mandatory)][string]$Root,
        [Parameter(Mandatory)][string]$BaselineCommit,
        [Parameter(Mandatory)][AllowEmptyCollection()][byte[]]$PatchBytes,
        [Parameter(Mandatory)][object[]]$IncludedEntry,
        [AllowEmptyCollection()][object[]]$ExcludedEntry=@()
    )
    if($PatchBytes.Length-eq 0){throw '[GENERIC-ACTUAL-DELTA-INVENTORY-PARITY] Package patch is empty.'}
    $expectedKeys=@(Get-GenericExpectedDeltaInventoryKeys -IncludedEntry $IncludedEntry)
    $excludedPaths=@(Get-GenericScopePaths -Entry $ExcludedEntry|Sort-Object -Unique)
    $beforeInventory=Get-GenericRealObjectInventory -Root $Root
    $context=New-GenericGitIsolationContext -Root $Root
    $operationError=$null;$actualEntries=@();$temporaryObjectFileCount=0;$cleanupPassed=$false
    try{
        $null=Invoke-GenericGitBytes -Root $Root -Argument @('read-tree',$BaselineCommit) -Environment $context.Environment
        $null=Invoke-GenericGitBytes -Root $Root -Argument @('apply','--cached','--whitespace=nowarn','-') -Environment $context.Environment -InputBytes $PatchBytes
        $nameStatus=Invoke-GenericGitBytes -Root $Root -Argument @('diff','--cached','--name-status','-z','--find-renames',$BaselineCommit,'--') -Environment $context.Environment
        $actualEntries=@(ConvertFrom-GenericNameStatusZ -Bytes $nameStatus.Bytes)
        $temporaryInventory=Get-GenericObjectDirectoryInventory -ObjectDirectory $context.ObjectDirectory
        $temporaryObjectFileCount=@($temporaryInventory.entries|Where-Object{$_.type-ceq'FILE'-and$_.path-cne'info/alternates'}).Count
    }
    catch{$operationError=$_}
    finally{
        $afterInventory=Get-GenericRealObjectInventory -Root $Root
        try{Remove-GenericGitIsolationContext -Context $context;$cleanupPassed=$true}catch{if($null-eq$operationError){$operationError=$_}}
    }
    if($null-ne$operationError){throw $operationError}
    $objectImmutable=Test-GenericObjectInventoryEqual -Before $beforeInventory -After $afterInventory
    if(-not$objectImmutable){throw '[GENERIC-REAL-OBJECT-DATABASE-IMMUTABILITY] Package patch inspection mutated the real Git object database.'}
    $actualKeys=@($actualEntries|ForEach-Object key|Sort-Object)
    $actualPaths=@(foreach($entry in $actualEntries){if($entry.kind-ceq'RENAME'){[string]$entry.previousPath};[string]$entry.path})
    $excludedDeltaPaths=@($actualPaths|Where-Object{$_-cin$excludedPaths}|Sort-Object -Unique)
    return [pscustomobject][ordered]@{
        ActualDeltaInventory=@($actualEntries);ExpectedDeltaInventoryKeys=$expectedKeys
        ActualDeltaInventoryParity=($actualKeys-join"`n")-ceq($expectedKeys-join"`n")
        ExcludedDeltaPaths=$excludedDeltaPaths;ExcludedDeltaPathProhibition=$excludedDeltaPaths.Count-eq 0
        LiteralPathspecBinding=$true;RealObjectDatabaseImmutable=$objectImmutable
        RealObjectInventoryBeforeSha256=[string]$beforeInventory.sha256
        RealObjectInventoryAfterSha256=[string]$afterInventory.sha256
        TemporaryObjectFileCount=$temporaryObjectFileCount;TemporaryArtifactsRemoved=$cleanupPassed
    }
}

function Get-GenericDeltaBytes {
    param(
        [Parameter(Mandatory)][string]$Root,
        [Parameter(Mandatory)][string]$BaselineCommit,
        [Parameter(Mandatory)][object[]]$IncludedEntry,
        [AllowEmptyCollection()][object[]]$ExcludedEntry=@()
    )
    $evidence=Get-GenericDeltaEvidence -Root $Root -BaselineCommit $BaselineCommit -IncludedEntry $IncludedEntry -ExcludedEntry $ExcludedEntry
    return ,([byte[]]$evidence.Bytes)
}
