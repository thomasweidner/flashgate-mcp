#requires -Version 7.6
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Get-PublicationSha256 {
    param([Parameter(Mandatory)][string]$LiteralPath)
    return (Get-FileHash -LiteralPath $LiteralPath -Algorithm SHA256).Hash.ToLowerInvariant()
}

function Get-PublicationStreamIdentity {
    param([Parameter(Mandatory)][System.IO.Stream]$Stream)
    $position = $Stream.Position
    try {
        $Stream.Position = 0
        $hash = [System.Security.Cryptography.SHA256]::HashData($Stream)
        return [pscustomobject]@{
            Sha256 = [Convert]::ToHexString($hash).ToLowerInvariant()
            Length = [int64]$Stream.Length
        }
    }
    finally { $Stream.Position = $position }
}

function New-PublicationHardLink {
    param([Parameter(Mandatory)][string]$LinkPath, [Parameter(Mandatory)][string]$ExistingPath)
    if (-not ('GovernanceHandoffPublication.NativeHardLink' -as [type])) {
        Add-Type -TypeDefinition @'
using System;
using System.ComponentModel;
using System.Runtime.InteropServices;
namespace GovernanceHandoffPublication {
    public static class NativeHardLink {
        [DllImport("Kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
        private static extern bool CreateHardLinkW(string linkPath, string existingPath, IntPtr securityAttributes);
        [DllImport("libc", EntryPoint = "link", SetLastError = true)]
        private static extern int CreateUnixHardLink(string existingPath, string linkPath);
        public static void Create(string linkPath, string existingPath) {
            bool ok;
            if (RuntimeInformation.IsOSPlatform(OSPlatform.Windows)) {
                ok = CreateHardLinkW(linkPath, existingPath, IntPtr.Zero);
            } else {
                ok = CreateUnixHardLink(existingPath, linkPath) == 0;
            }
            if (!ok) throw new Win32Exception(Marshal.GetLastWin32Error(), "Atomic hard-link publication failed");
        }
    }
}
'@
    }
    [GovernanceHandoffPublication.NativeHardLink]::Create($LinkPath, $ExistingPath)
}

function Invoke-PublicationPhaseObserver {
    param(
        [scriptblock]$Observer,
        [Parameter(Mandatory)][string]$Phase,
        [Parameter(Mandatory)][string]$CandidatePath,
        [Parameter(Mandatory)][string]$FinalPath
    )
    if ($null -ne $Observer) {
        & $Observer ([pscustomobject]@{
                Phase = $Phase
                CandidatePath = $CandidatePath
                FinalPath = $FinalPath
            })
    }
}

function Publish-GovernanceHandoffPackage {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][string]$StagingDirectory,
        [Parameter(Mandatory)][string]$FinalPath,
        [Parameter(Mandatory)][scriptblock]$CandidateValidator,
        [Parameter(Mandatory)][ref]$PackageWriteAttemptCount,
        [string]$CandidateDirectory,
        [scriptblock]$PhaseObserver,
        [scriptblock]$CandidateSerializer,
        [scriptblock]$PublicationOperation
    )

    $resolvedStaging = [System.IO.Path]::GetFullPath($StagingDirectory)
    $resolvedFinal = [System.IO.Path]::GetFullPath($FinalPath)
    $finalDirectory = [System.IO.Path]::GetFullPath((Split-Path -Parent $resolvedFinal))
    $resolvedCandidateDirectory = if ([string]::IsNullOrWhiteSpace($CandidateDirectory)) {
        $finalDirectory
    }
    else {
        [System.IO.Path]::GetFullPath($CandidateDirectory)
    }
    if (-not (Test-Path -LiteralPath $resolvedStaging -PathType Container)) {
        throw "Publication staging directory does not exist: $resolvedStaging"
    }
    if (-not (Test-Path -LiteralPath $finalDirectory -PathType Container)) {
        throw "Publication target directory does not exist: $finalDirectory"
    }
    if ($resolvedCandidateDirectory -cne $finalDirectory) {
        throw 'Candidate and final package must use the same canonical parent directory.'
    }
    if ([System.IO.Path]::GetPathRoot($resolvedCandidateDirectory) -cne [System.IO.Path]::GetPathRoot($resolvedFinal)) {
        throw 'Candidate and final package must use the same volume.'
    }
    if (Test-Path -LiteralPath $resolvedFinal) {
        throw "Canonical final package path must be absent before publication: $resolvedFinal"
    }
    if ($PackageWriteAttemptCount.Value -ne 0) {
        throw 'Package write attempt count must be zero before the sole publication attempt.'
    }

    $candidateName = '{0}.{1}.pending' -f [guid]::NewGuid().ToString('N'), ([System.IO.Path]::GetFileName($resolvedFinal))
    $candidatePath = Join-Path $resolvedCandidateDirectory $candidateName
    $candidateSha256 = $null
    $candidateLength = 0L
    $candidateMemberCount = 0
    $serializationCount = 0

    Invoke-PublicationPhaseObserver -Observer $PhaseObserver -Phase 'BEFORE_CREATE' `
        -CandidatePath $candidatePath -FinalPath $resolvedFinal
    $PackageWriteAttemptCount.Value = 1
    $candidateStream = [System.IO.FileStream]::new(
        $candidatePath,
        [System.IO.FileMode]::CreateNew,
        [System.IO.FileAccess]::Write,
        [System.IO.FileShare]::None
    )
    try {
        $serializationCount++
        if ($null -ne $CandidateSerializer) {
            & $CandidateSerializer $resolvedStaging $candidateStream
        }
        else {
            Add-Type -AssemblyName System.IO.Compression
            $archive = [System.IO.Compression.ZipArchive]::new(
                $candidateStream,
                [System.IO.Compression.ZipArchiveMode]::Create,
                $true
            )
            try {
                foreach ($file in @(Get-ChildItem -LiteralPath $resolvedStaging -File | Sort-Object Name)) {
                    $entry = $archive.CreateEntry($file.Name, [System.IO.Compression.CompressionLevel]::Optimal)
                    $entry.LastWriteTime = [datetimeoffset]::new(2000, 1, 1, 0, 0, 0, [timespan]::Zero)
                    $entryStream = $entry.Open()
                    try {
                        $bytes = [System.IO.File]::ReadAllBytes($file.FullName)
                        $entryStream.Write($bytes, 0, $bytes.Length)
                    }
                    finally { $entryStream.Dispose() }
                }
            }
            finally { $archive.Dispose() }
        }
    }
    finally { $candidateStream.Dispose() }

    if ($serializationCount -ne 1) {
        throw 'The package candidate must be serialized exactly once.'
    }
    Invoke-PublicationPhaseObserver -Observer $PhaseObserver -Phase 'CANDIDATE_CLOSED' `
        -CandidatePath $candidatePath -FinalPath $resolvedFinal
    $candidateIdentity = [pscustomobject]@{
        Sha256 = Get-PublicationSha256 -LiteralPath $candidatePath
        Length = [int64][System.IO.FileInfo]::new($candidatePath).Length
    }
    & $CandidateValidator $candidatePath $candidateIdentity
    Invoke-PublicationPhaseObserver -Observer $PhaseObserver -Phase 'CANDIDATE_VALIDATED' `
        -CandidatePath $candidatePath -FinalPath $resolvedFinal
    $candidateLease = [System.IO.FileStream]::new(
        $candidatePath, [System.IO.FileMode]::Open, [System.IO.FileAccess]::Read, [System.IO.FileShare]::Read
    )
    $published = $false
    $publicationFailure = $null
    try {
        $leasedIdentity = Get-PublicationStreamIdentity -Stream $candidateLease
        if ($leasedIdentity.Sha256 -cne $candidateIdentity.Sha256 -or
            $leasedIdentity.Length -ne $candidateIdentity.Length) {
            throw 'Package candidate identity drifted after product validation and before publication.'
        }
        $candidateSha256 = $leasedIdentity.Sha256
        $candidateLength = $leasedIdentity.Length
        Add-Type -AssemblyName System.IO.Compression.FileSystem
        $candidateArchive = [System.IO.Compression.ZipFile]::OpenRead($candidatePath)
        try { $candidateMemberCount = @($candidateArchive.Entries).Count }
        finally { $candidateArchive.Dispose() }
        if (Test-Path -LiteralPath $resolvedFinal) {
            throw 'Canonical final package path appeared before atomic publication.'
        }
        if ($null -ne $PublicationOperation) {
            & $PublicationOperation $candidatePath $resolvedFinal
        }
        else {
            New-PublicationHardLink -LinkPath $resolvedFinal -ExistingPath $candidatePath
        }
        $published = $true
        Invoke-PublicationPhaseObserver -Observer $PhaseObserver -Phase 'PUBLISHED' `
            -CandidatePath $candidatePath -FinalPath $resolvedFinal

        $finalSha256 = Get-PublicationSha256 -LiteralPath $resolvedFinal
        $finalLength = (Get-Item -LiteralPath $resolvedFinal).Length
        $finalArchive = [System.IO.Compression.ZipFile]::OpenRead($resolvedFinal)
        try { $finalMemberCount = @($finalArchive.Entries).Count }
        finally { $finalArchive.Dispose() }
        if ($finalSha256 -cne $candidateSha256 -or $finalLength -ne $candidateLength -or
            $finalMemberCount -ne $candidateMemberCount) {
            throw 'Published package identity differs from the validated candidate identity.'
        }
    }
    catch {
        $publicationFailure = $_
    }
    finally { $candidateLease.Dispose() }

    if ($null -ne $publicationFailure) {
        if ($published -and (Test-Path -LiteralPath $resolvedFinal -PathType Leaf)) {
            [System.IO.File]::Delete($resolvedFinal)
        }
        throw $publicationFailure
    }

    if (Test-Path -LiteralPath $candidatePath -PathType Leaf) {
        [System.IO.File]::Delete($candidatePath)
    }

    return [pscustomobject][ordered]@{
        CandidatePath = $candidatePath
        FinalPath = $resolvedFinal
        Sha256 = $finalSha256
        Length = $finalLength
        MemberCount = $finalMemberCount
        PackageWriteAttemptCount = $PackageWriteAttemptCount.Value
        SerializationCount = $serializationCount
        CandidateValidationStatus = 'PASS'
        PublicationStatus = 'PASS'
        IdentityValidationStatus = 'PASS'
    }
}

Export-ModuleMember -Function Publish-GovernanceHandoffPackage
