Set-StrictMode -Version Latest

if (-not ('FlashGateSnapshot.NativeFile' -as [type])) {
    Add-Type -TypeDefinition @'
using System;
using System.ComponentModel;
using System.IO;
using System.Runtime.InteropServices;
using Microsoft.Win32.SafeHandles;

namespace FlashGateSnapshot
{
    public static class NativeFile
    {
        [StructLayout(LayoutKind.Sequential)]
        private struct ByHandleFileInformation
        {
            public uint FileAttributes;
            public System.Runtime.InteropServices.ComTypes.FILETIME CreationTime;
            public System.Runtime.InteropServices.ComTypes.FILETIME LastAccessTime;
            public System.Runtime.InteropServices.ComTypes.FILETIME LastWriteTime;
            public uint VolumeSerialNumber;
            public uint FileSizeHigh;
            public uint FileSizeLow;
            public uint NumberOfLinks;
            public uint FileIndexHigh;
            public uint FileIndexLow;
        }

        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern bool GetFileInformationByHandle(
            SafeFileHandle file,
            out ByHandleFileInformation information);

        [DllImport("kernel32.dll", SetLastError = true, CharSet = CharSet.Unicode)]
        private static extern uint GetFinalPathNameByHandle(
            SafeFileHandle file,
            System.Text.StringBuilder path,
            uint length,
            uint flags);

        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern uint GetFileType(SafeFileHandle file);

        public static uint LinkCount(SafeFileHandle file)
        {
            if (!GetFileInformationByHandle(file, out var information))
            {
                throw new Win32Exception(Marshal.GetLastWin32Error());
            }
            return information.NumberOfLinks;
        }

        public static string FinalPath(SafeFileHandle file)
        {
            var buffer = new System.Text.StringBuilder(32768);
            var length = GetFinalPathNameByHandle(
                file,
                buffer,
                (uint)buffer.Capacity,
                0);
            if (length == 0 || length >= buffer.Capacity)
            {
                throw new Win32Exception(Marshal.GetLastWin32Error());
            }
            var value = buffer.ToString();
            return value.StartsWith(@"\\?\", StringComparison.Ordinal)
                ? value.Substring(4)
                : value;
        }

        public static void RequireDiskFile(SafeFileHandle file)
        {
            const uint FileTypeDisk = 0x0001;
            if (GetFileType(file) != FileTypeDisk)
            {
                throw new InvalidOperationException("Snapshot input is not a disk file.");
            }
        }
    }
}
'@
}

function Invoke-GitNullList {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string] $RootPath,

        [Parameter(Mandatory)]
        [string[]] $Arguments
    )

    $StartInfo = [Diagnostics.ProcessStartInfo]::new()
    $StartInfo.FileName = 'git.exe'
    $StartInfo.UseShellExecute = $false
    $StartInfo.RedirectStandardOutput = $true
    $StartInfo.RedirectStandardError = $true
    $StartInfo.CreateNoWindow = $true
    $null = $StartInfo.ArgumentList.Add('-C')
    $null = $StartInfo.ArgumentList.Add($RootPath)
    foreach ($Argument in $Arguments) {
        $null = $StartInfo.ArgumentList.Add($Argument)
    }

    $Process = [Diagnostics.Process]::new()
    $Process.StartInfo = $StartInfo
    $Output = [IO.MemoryStream]::new()
    try {
        if (-not $Process.Start()) {
            throw 'Unable to start git.exe.'
        }
        $ErrorTask = $Process.StandardError.ReadToEndAsync()
        $Process.StandardOutput.BaseStream.CopyTo($Output)
        $Process.WaitForExit()
        $ErrorText = $ErrorTask.GetAwaiter().GetResult()
        if ($Process.ExitCode -ne 0) {
            throw ('git {0} failed: {1}' -f ($Arguments -join ' '), $ErrorText)
        }
    }
    finally {
        $Process.Dispose()
    }

    $Text = [Text.Encoding]::UTF8.GetString($Output.ToArray())
    @(
        $Text.Split(
            [char]0,
            [StringSplitOptions]::RemoveEmptyEntries
        )
    )
}

function Test-FlashGateSensitiveSnapshotName {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string] $RelativePath
    )

    $Name = [IO.Path]::GetFileName($RelativePath).ToLowerInvariant()
    if (
        $Name -eq '.env' -or
        $Name.StartsWith('.env.') -or
        $Name -match '\.(pem|key|pfx|p12)$' -or
        $Name.StartsWith('id_rsa') -or
        $Name.StartsWith('id_ed25519') -or
        $Name.StartsWith('credentials') -or
        $Name.StartsWith('secrets') -or
        $Name -in @('auth.json', '.npmrc', '.pypirc', 'token', 'tokens.json') -or
        $Name -match '(^|[._-])(github|gitlab|npm|pypi|api)[._-]?token([._-]|$)'
    ) {
        return $true
    }
    return $false
}

function New-FlashGateGitSnapshot {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string] $RootPath,

        [Parameter(Mandatory)]
        [string] $SnapshotPath,

        [Parameter(Mandatory)]
        [string] $ManifestPath
    )

    $RootPath = [IO.Path]::GetFullPath($RootPath).TrimEnd('\')
    $SnapshotPath = [IO.Path]::GetFullPath($SnapshotPath)
    $ManifestPath = [IO.Path]::GetFullPath($ManifestPath)
    $RootPrefix = "$RootPath\"
    foreach ($OutputPath in @($SnapshotPath, $ManifestPath)) {
        if ($OutputPath.StartsWith(
            $RootPrefix,
            [StringComparison]::OrdinalIgnoreCase
        )) {
            throw "Snapshot outputs must be outside the repository: $OutputPath"
        }
    }

    $Tracked = Invoke-GitNullList `
        -RootPath $RootPath `
        -Arguments @('ls-files', '-z')
    $Untracked = Invoke-GitNullList `
        -RootPath $RootPath `
        -Arguments @('ls-files', '--others', '--exclude-standard', '-z')
    $Ignored = [Collections.Generic.HashSet[string]]::new(
        [StringComparer]::Ordinal
    )
    foreach (
        $Path in Invoke-GitNullList `
            -RootPath $RootPath `
            -Arguments @('ls-files', '-ci', '--exclude-standard', '-z')
    ) {
        $null = $Ignored.Add($Path)
    }
    foreach (
        $Path in Invoke-GitNullList `
            -RootPath $RootPath `
            -Arguments @(
                'ls-files'
                '--others'
                '--ignored'
                '--exclude-standard'
                '-z'
            )
    ) {
        $null = $Ignored.Add($Path)
    }

    foreach ($IgnoredPath in $Ignored) {
        if (
            Test-FlashGateSensitiveSnapshotName `
                -RelativePath $IgnoredPath
        ) {
            throw (
                'Sensitive filename exists in the worktree even though Git ' +
                "ignores it: $IgnoredPath"
            )
        }
    }

    $TrackedPaths = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
    foreach ($TrackedPath in @($Tracked)) {
        [void]$TrackedPaths.Add($TrackedPath)
    }
    $Paths = @(
        @($Tracked) + @($Untracked) |
            # A local exclusion must not remove committed public source.
            # Sensitive names remain rejected independently below.
            Where-Object { $TrackedPaths.Contains($_) -or -not $Ignored.Contains($_) } |
            Sort-Object -Unique
    )
    if ($Paths.Count -eq 0) {
        throw 'Git snapshot inventory is empty.'
    }

    $Streams = [Collections.Generic.List[IO.FileStream]]::new()
    $Entries = [Collections.Generic.List[object]]::new()
    try {
        foreach ($RelativePath in $Paths) {
            if (
                [string]::IsNullOrWhiteSpace($RelativePath) -or
                [IO.Path]::IsPathRooted($RelativePath) -or
                $RelativePath.Contains('\') -or
                $RelativePath -match '(^|/)\.\.?(/|$)' -or
                $RelativePath.IndexOfAny([char[]]@(0, 13, 10)) -ge 0
            ) {
                throw "Unsafe Git inventory path: $RelativePath"
            }
            if (
                Test-FlashGateSensitiveSnapshotName `
                    -RelativePath $RelativePath
            ) {
                throw "Sensitive filename is not allowed in a snapshot: $RelativePath"
            }

            $CandidatePath = [IO.Path]::GetFullPath(
                (Join-Path $RootPath $RelativePath)
            )
            if (-not $CandidatePath.StartsWith(
                $RootPrefix,
                [StringComparison]::OrdinalIgnoreCase
            )) {
                throw "Snapshot input escapes the repository: $RelativePath"
            }

            $Item = Get-Item -LiteralPath $CandidatePath -Force
            if (
                -not $Item.PSIsContainer -and
                ($Item.Attributes -band [IO.FileAttributes]::ReparsePoint) -eq 0 -and
                ($Item.Attributes -band [IO.FileAttributes]::Device) -eq 0
            ) {
                $Stream = [IO.File]::Open(
                    $CandidatePath,
                    [IO.FileMode]::Open,
                    [IO.FileAccess]::Read,
                    (
                        [IO.FileShare]::Read -bor
                        [IO.FileShare]::Delete
                    )
                )
            }
            else {
                throw "Snapshot input is not a regular link-free file: $RelativePath"
            }

            try {
                [FlashGateSnapshot.NativeFile]::RequireDiskFile(
                    $Stream.SafeFileHandle
                )
                $FinalPath = [IO.Path]::GetFullPath(
                    [FlashGateSnapshot.NativeFile]::FinalPath(
                        $Stream.SafeFileHandle
                    )
                )
                if ($FinalPath -cne $CandidatePath) {
                    throw "Snapshot handle resolved unexpectedly: $RelativePath"
                }
                if (
                    [FlashGateSnapshot.NativeFile]::LinkCount(
                        $Stream.SafeFileHandle
                    ) -ne 1
                ) {
                    throw "Hard-linked snapshot input rejected: $RelativePath"
                }

                $HashAlgorithm = [Security.Cryptography.SHA256]::Create()
                try {
                    $Hash = [Convert]::ToHexString(
                        $HashAlgorithm.ComputeHash($Stream)
                    ).ToLowerInvariant()
                }
                finally {
                    $HashAlgorithm.Dispose()
                }
                $Stream.Position = 0

                $Streams.Add($Stream)
                $Entries.Add([pscustomobject]@{
                    Path   = $RelativePath
                    Type   = 'file'
                    Length = $Stream.Length
                    SHA256 = $Hash
                })
                $Stream = $null
            }
            finally {
                if ($null -ne $Stream) {
                    $Stream.Dispose()
                }
            }
        }

        $Manifest = [ordered]@{
            schema = 'flashgate-git-snapshot-manifest/v1'
            files  = @($Entries)
        }
        [IO.Directory]::CreateDirectory(
            [IO.Path]::GetDirectoryName($ManifestPath)
        ) | Out-Null
        [IO.File]::WriteAllText(
            $ManifestPath,
            ($Manifest | ConvertTo-Json -Depth 5) + "`n",
            [Text.UTF8Encoding]::new($false)
        )

        Add-Type -AssemblyName System.Formats.Tar
        $TarStream = [IO.File]::Open(
            $SnapshotPath,
            [IO.FileMode]::CreateNew,
            [IO.FileAccess]::Write,
            [IO.FileShare]::None
        )
        try {
            $Writer = [System.Formats.Tar.TarWriter]::new(
                $TarStream,
                [System.Formats.Tar.TarEntryFormat]::Ustar,
                $false
            )
            try {
                for ($Index = 0; $Index -lt $Entries.Count; $Index++) {
                    $Entry = [System.Formats.Tar.UstarTarEntry]::new(
                        [System.Formats.Tar.TarEntryType]::RegularFile,
                        $Entries[$Index].Path
                    )
                    $Entry.DataStream = $Streams[$Index]
                    $Entry.ModificationTime = [DateTimeOffset]::UnixEpoch
                    $Entry.Uid = 0
                    $Entry.Gid = 0
                    $Entry.UserName = ''
                    $Entry.GroupName = ''
                    $Entry.Mode = [Convert]::ToInt32('644', 8)
                    $Writer.WriteEntry($Entry)
                }
            }
            finally {
                $Writer.Dispose()
            }
        }
        finally {
            $TarStream.Dispose()
        }

        [pscustomobject]@{
            SnapshotPath = $SnapshotPath
            ManifestPath = $ManifestPath
            FileCount    = $Entries.Count
            SnapshotSHA256 = (
                Get-FileHash -LiteralPath $SnapshotPath -Algorithm SHA256
            ).Hash.ToLowerInvariant()
        }
    }
    catch {
        foreach ($PartialPath in @($SnapshotPath, $ManifestPath)) {
            if (Test-Path -LiteralPath $PartialPath -PathType Leaf) {
                Remove-Item -LiteralPath $PartialPath -Force
            }
        }
        throw
    }
    finally {
        foreach ($Stream in $Streams) {
            $Stream.Dispose()
        }
    }
}
