#requires -Version 7.6

[CmdletBinding()]
param(
    [Parameter(Mandatory, Position = 0)]
    [ValidateNotNullOrEmpty()]
    [string]$ArtifactPath,

    [string]$ManifestPath,

    [ValidateSet('RequireTrue', 'Ignore')]
    [string]$ReadinessRequirement = 'RequireTrue',

    [string[]]$AllowedLiteralPlaceholder = @(),

    [string]$ReportPath
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$Failures = [System.Collections.Generic.List[object]]::new()
$Warnings = [System.Collections.Generic.List[object]]::new()
$Entries = [System.Collections.Generic.List[object]]::new()
$ReadinessValues = [System.Collections.Generic.List[string]]::new()
$AllowlistMatches = [System.Collections.Generic.List[object]]::new()
$StrictUtf8 = [System.Text.UTF8Encoding]::new($false, $true)
$ExitCode = 1
$InvocationProblem = $false
$ArtifactType = 'UNKNOWN'
$ResolvedArtifactPath = $ArtifactPath
$StrictUtf8Result = 'NOT_RUN'
$ReplacementCharacterResult = 'NOT_RUN'
$ControlCharacterResult = 'NOT_RUN'
$PlaceholderResult = 'NOT_RUN'
$ObjectInterpolationResult = 'NOT_RUN'
$ManifestResult = 'NOT_APPLICABLE'
$ZipReopenResult = 'NOT_APPLICABLE'
$ZipPathSafetyResult = 'NOT_APPLICABLE'
$InventoryConsistencyResult = 'NOT_APPLICABLE'
$ClassicReviewReadyObserved = 'Missing'
$ClassicReviewReadyAllowed = $false

$TextExtensions = [System.Collections.Generic.HashSet[string]]::new(
    [System.StringComparer]::OrdinalIgnoreCase
)
foreach ($Extension in @(
        '.csv', '.diff', '.json', '.log', '.md', '.patch', '.ps1', '.psd1',
        '.psm1', '.sha256', '.tsv', '.txt', '.xml', '.yaml', '.yml'
    )) {
    [void]$TextExtensions.Add($Extension)
}

$SemanticTextExtensions = [System.Collections.Generic.HashSet[string]]::new(
    [System.StringComparer]::OrdinalIgnoreCase
)
foreach ($Extension in @('.log', '.md', '.txt')) {
    [void]$SemanticTextExtensions.Add($Extension)
}

$Allowlist = [System.Collections.Generic.HashSet[string]]::new(
    [System.StringComparer]::Ordinal
)
foreach ($AllowedValue in $AllowedLiteralPlaceholder) {
    if ([string]::IsNullOrWhiteSpace($AllowedValue)) {
        $InvocationProblem = $true
        [void]$Failures.Add([pscustomobject]@{
            Code   = 'INVALID_ALLOWLIST_VALUE'
            Entry  = $null
            Offset = $null
            Detail = 'Allowlist values must be non-empty exact literals.'
        })
    }
    else {
        [void]$Allowlist.Add($AllowedValue)
    }
}

function Add-Failure {
    param(
        [Parameter(Mandatory)][string]$Code,
        [string]$Entry,
        [Nullable[int]]$Offset,
        [string]$Detail
    )

    [void]$script:Failures.Add([pscustomobject]@{
        Code   = $Code
        Entry  = $Entry
        Offset = $Offset
        Detail = $Detail
    })
}

function Get-Sha256Hex {
    param([Parameter(Mandatory)][byte[]]$Bytes)

    $Algorithm = [System.Security.Cryptography.SHA256]::Create()
    try {
        $HashBytes = $Algorithm.ComputeHash($Bytes)
        return [System.Convert]::ToHexString($HashBytes).ToLowerInvariant()
    }
    finally {
        $Algorithm.Dispose()
    }
}

function Test-SafeRelativePath {
    param([Parameter(Mandatory)][string]$Path)

    if ([string]::IsNullOrWhiteSpace($Path)) {
        return $false
    }

    if ($Path.Contains('\') -or
        $Path.StartsWith('/', [System.StringComparison]::Ordinal) -or
        $Path.StartsWith('\', [System.StringComparison]::Ordinal) -or
        $Path -match '^[A-Za-z]:' -or
        $Path.EndsWith('/', [System.StringComparison]::Ordinal)) {
        return $false
    }

    $Segments = $Path.Split('/')
    if ($Segments.Count -eq 0) {
        return $false
    }

    foreach ($Segment in $Segments) {
        if ([string]::IsNullOrEmpty($Segment) -or $Segment -eq '.' -or $Segment -eq '..') {
            return $false
        }
    }

    return $true
}

function Remove-InlineCode {
    param(
        [Parameter(Mandatory)]
        [AllowEmptyString()]
        [string]$Line
    )

    $Builder = [System.Text.StringBuilder]::new($Line.Length)
    $Index = 0
    while ($Index -lt $Line.Length) {
        if ([int][char]$Line[$Index] -ne 96) {
            [void]$Builder.Append($Line[$Index])
            $Index++
            continue
        }

        $RunLength = 1
        while (($Index + $RunLength) -lt $Line.Length -and
            [int][char]$Line[$Index + $RunLength] -eq 96) {
            $RunLength++
        }

        $Delimiter = [string]::new([char]96, $RunLength)
        $ClosingIndex = $Line.IndexOf(
            $Delimiter,
            $Index + $RunLength,
            [System.StringComparison]::Ordinal
        )

        if ($ClosingIndex -lt 0) {
            [void]$Builder.Append($Delimiter)
            $Index += $RunLength
            continue
        }

        $MaskedLength = $ClosingIndex + $RunLength - $Index
        [void]$Builder.Append(' ', $MaskedLength)
        $Index += $MaskedLength
    }

    return $Builder.ToString()
}

function Get-MarkdownOutsideCode {
    param([Parameter(Mandatory)][string]$Text)

    $Builder = [System.Text.StringBuilder]::new($Text.Length)
    $FenceCharacter = $null
    $FenceLength = 0

    foreach ($RawLine in $Text.Split([char]10)) {
        $Line = $RawLine.TrimEnd([char]13)

        if ($null -eq $FenceCharacter) {
            $OpeningMatch = [regex]::Match($Line, '^\s*(?<marker>`{3,}|~{3,})')
            if ($OpeningMatch.Success) {
                $Marker = $OpeningMatch.Groups['marker'].Value
                $FenceCharacter = $Marker[0]
                $FenceLength = $Marker.Length
                [void]$Builder.AppendLine()
                continue
            }

            [void]$Builder.AppendLine((Remove-InlineCode -Line $Line))
            continue
        }

        $ClosingPattern = '^\s*' +
            [regex]::Escape([string]$FenceCharacter) +
            '{' + $FenceLength + ',}\s*$'
        if ([regex]::IsMatch($Line, $ClosingPattern)) {
            $FenceCharacter = $null
            $FenceLength = 0
        }
        [void]$Builder.AppendLine()
    }

    return $Builder.ToString()
}

function Test-AllowlistedMatch {
    param(
        [Parameter(Mandatory)][string]$Value,
        [Parameter(Mandatory)][string]$Entry,
        [Parameter(Mandatory)][int]$Offset
    )

    if (-not $script:Allowlist.Contains($Value)) {
        return $false
    }

    [void]$script:AllowlistMatches.Add([pscustomobject]@{
        Entry   = $Entry
        Offset  = $Offset
        Literal = $Value
    })
    return $true
}

function Get-EntryText {
    param([Parameter(Mandatory)]$Entry)

    try {
        $Text = $script:StrictUtf8.GetString([byte[]]$Entry.Bytes)
        return [pscustomobject]@{
            Success = $true
            Text    = $Text
        }
    }
    catch [System.Text.DecoderFallbackException] {
        Add-Failure -Code 'STRICT_UTF8_INVALID' -Entry $Entry.Path -Offset $null `
            -Detail 'The entry is not valid strict UTF-8.'
        return [pscustomobject]@{
            Success = $false
            Text    = $null
        }
    }
}

function Test-TextEntry {
    param([Parameter(Mandatory)]$Entry)

    $Decoded = Get-EntryText -Entry $Entry
    if (-not $Decoded.Success) {
        return
    }

    $Text = [string]$Decoded.Text
    if ($Text.Length -gt 0 -and [int][char]$Text[0] -eq 0xFEFF) {
        $Text = $Text.Substring(1)
    }

    $ReplacementIndex = $Text.IndexOf([char]0xFFFD)
    if ($ReplacementIndex -ge 0) {
        Add-Failure -Code 'REPLACEMENT_CHARACTER_PRESENT' -Entry $Entry.Path `
            -Offset $ReplacementIndex -Detail 'Stored U+FFFD replacement character is forbidden.'
    }

    for ($Index = 0; $Index -lt $Text.Length; $Index++) {
        $CodePoint = [int][char]$Text[$Index]
        if ($CodePoint -eq 10) {
            continue
        }
        if ($CodePoint -eq 13) {
            if (($Index + 1) -lt $Text.Length -and [int][char]$Text[$Index + 1] -eq 10) {
                continue
            }
            Add-Failure -Code 'ISOLATED_CARRIAGE_RETURN' -Entry $Entry.Path -Offset $Index `
                -Detail 'CR is allowed only as part of CRLF.'
            continue
        }
        if ($CodePoint -eq 9) {
            Add-Failure -Code 'UNEXPECTED_TAB' -Entry $Entry.Path -Offset $Index `
                -Detail 'Tab is not allowed in review text.'
            continue
        }
        if ($CodePoint -lt 32 -or ($CodePoint -ge 127 -and $CodePoint -le 159)) {
            Add-Failure -Code 'FORBIDDEN_CONTROL_CHARACTER' -Entry $Entry.Path -Offset $Index `
                -Detail ('Forbidden control code U+{0:X4}.' -f $CodePoint)
        }
    }

    $Extension = [System.IO.Path]::GetExtension($Entry.Path)
    if (-not $script:SemanticTextExtensions.Contains($Extension)) {
        return
    }

    $OutsideCode = Get-MarkdownOutsideCode -Text $Text

    foreach ($Match in [regex]::Matches(
            $OutsideCode,
            '\$(?:\{[A-Za-z_][A-Za-z0-9_:.-]*\}|[A-Za-z_][A-Za-z0-9_:.-]*)'
        )) {
        if (-not (Test-AllowlistedMatch -Value $Match.Value -Entry $Entry.Path -Offset $Match.Index)) {
            Add-Failure -Code 'UNRESOLVED_POWERSHELL_VARIABLE' -Entry $Entry.Path `
                -Offset $Match.Index -Detail 'Unresolved PowerShell variable outside Markdown code.'
        }
    }

    foreach ($Match in [regex]::Matches($OutsideCode, '\$\(')) {
        if (-not (Test-AllowlistedMatch -Value $Match.Value -Entry $Entry.Path -Offset $Match.Index)) {
            Add-Failure -Code 'UNRESOLVED_SUBEXPRESSION' -Entry $Entry.Path `
                -Offset $Match.Index -Detail 'Unresolved PowerShell subexpression outside Markdown code.'
        }
    }

    foreach ($Pattern in @(
            '\{\{[A-Z][A-Z0-9_]{1,63}\}\}',
            '__[A-Z][A-Z0-9_]{1,63}__',
            '<[A-Z][A-Z0-9_]{1,63}>'
        )) {
        foreach ($Match in [regex]::Matches($OutsideCode, $Pattern)) {
            if (-not (Test-AllowlistedMatch -Value $Match.Value -Entry $Entry.Path -Offset $Match.Index)) {
                Add-Failure -Code 'UNRESOLVED_TEMPLATE_TOKEN' -Entry $Entry.Path `
                    -Offset $Match.Index -Detail 'Unresolved template token outside Markdown code.'
            }
        }
    }

    foreach ($Match in [regex]::Matches($OutsideCode, 'System\.Object\[\]')) {
        Add-Failure -Code 'NON_SCALAR_OBJECT_ARRAY' -Entry $Entry.Path -Offset $Match.Index `
            -Detail 'Implicit array interpolation output is forbidden.'
    }

    foreach ($Match in [regex]::Matches(
            $OutsideCode,
            '@\{[^{}\r\n]*[A-Za-z_][A-Za-z0-9_]*\s*=[^{}\r\n]+(?:;[^{}\r\n]+)*\}'
        )) {
        Add-Failure -Code 'NON_SCALAR_HASHTABLE_DUMP' -Entry $Entry.Path -Offset $Match.Index `
            -Detail 'Implicit hashtable/object dump is forbidden.'
    }

    foreach ($Match in [regex]::Matches($OutsideCode, '\$\(@\{[^\r\n]{0,2000}\}')) {
        Add-Failure -Code 'NON_SCALAR_SUBEXPRESSION' -Entry $Entry.Path -Offset $Match.Index `
            -Detail 'Hashtable/object subexpression interpolation is forbidden.'
    }

    foreach ($Match in [regex]::Matches(
            $OutsideCode,
            '(?im)\bClassic\s*Review\s*Ready\s*(?:\|\s*|[:=]\s*)(?<value>true|false)\b'
        )) {
        [void]$script:ReadinessValues.Add($Match.Groups['value'].Value.ToLowerInvariant())
    }
}

function Read-ZipEntries {
    param([Parameter(Mandatory)][string]$Path)

    $FileStream = $null
    $Archive = $null
    try {
        $FileStream = [System.IO.File]::Open(
            $Path,
            [System.IO.FileMode]::Open,
            [System.IO.FileAccess]::Read,
            [System.IO.FileShare]::Read
        )
        $Archive = [System.IO.Compression.ZipArchive]::new(
            $FileStream,
            [System.IO.Compression.ZipArchiveMode]::Read,
            $false
        )

        foreach ($ZipEntry in $Archive.Entries) {
            $Memory = [System.IO.MemoryStream]::new()
            $EntryStream = $null
            try {
                $EntryStream = $ZipEntry.Open()
                $EntryStream.CopyTo($Memory)
                $Bytes = $Memory.ToArray()
            }
            finally {
                if ($null -ne $EntryStream) {
                    $EntryStream.Dispose()
                }
                $Memory.Dispose()
            }

            [void]$script:Entries.Add([pscustomobject]@{
                Path               = $ZipEntry.FullName
                Bytes              = $Bytes
                Length             = [long]$Bytes.LongLength
                ExternalAttributes = [int]$ZipEntry.ExternalAttributes
            })
        }

        $script:ZipReopenResult = 'PASS'
    }
    catch {
        $script:ZipReopenResult = 'FAIL'
        Add-Failure -Code 'ZIP_REOPEN_FAILED' -Entry $null -Offset $null `
            -Detail 'ZIP could not be opened and read completely.'
    }
    finally {
        if ($null -ne $Archive) {
            $Archive.Dispose()
        }
        if ($null -ne $FileStream) {
            $FileStream.Dispose()
        }
    }
}

function Test-ZipPaths {
    $Seen = [System.Collections.Generic.HashSet[string]]::new(
        [System.StringComparer]::OrdinalIgnoreCase
    )
    $Failed = $false

    foreach ($Entry in $script:Entries) {
        if (-not (Test-SafeRelativePath -Path $Entry.Path)) {
            $Failed = $true
            Add-Failure -Code 'ZIP_UNSAFE_PATH' -Entry $Entry.Path -Offset $null `
                -Detail 'ZIP entry path is not a safe canonical relative path.'
        }
        if (-not $Seen.Add($Entry.Path)) {
            $Failed = $true
            Add-Failure -Code 'ZIP_DUPLICATE_PATH' -Entry $Entry.Path -Offset $null `
                -Detail 'ZIP entry paths must be unique under Windows path comparison.'
        }

        $UnsignedAttributes = [System.BitConverter]::ToUInt32(
            [System.BitConverter]::GetBytes([int]$Entry.ExternalAttributes),
            0
        )
        $UnixMode = ($UnsignedAttributes -shr 16) -band 0xF000
        $WindowsAttributes = $UnsignedAttributes -band 0xFFFF
        if ($UnixMode -eq 0xA000 -or
            ($WindowsAttributes -band [uint32][System.IO.FileAttributes]::ReparsePoint) -ne 0) {
            $Failed = $true
            Add-Failure -Code 'ZIP_LINK_OR_REPARSE_ENTRY' -Entry $Entry.Path -Offset $null `
                -Detail 'Symlink/reparse-like ZIP entries are forbidden.'
        }
    }

    $script:ZipPathSafetyResult = if ($Failed) { 'FAIL' } else { 'PASS' }
}

function Test-Manifest {
    param(
        [byte[]]$ManifestBytes,
        [string]$ManifestEntryPath,
        [bool]$Required
    )

    if ($null -eq $ManifestBytes) {
        if ($Required) {
            $script:ManifestResult = 'FAIL'
            $script:InventoryConsistencyResult = 'FAIL'
            Add-Failure -Code 'MANIFEST_MISSING' -Entry $ManifestEntryPath -Offset $null `
                -Detail 'A root MANIFEST.sha256 is required for directory and ZIP artifacts.'
        }
        return
    }

    $ManifestTextEntry = [pscustomobject]@{
        Path   = if ($ManifestEntryPath) { $ManifestEntryPath } else { 'MANIFEST.sha256' }
        Bytes  = $ManifestBytes
        Length = [long]$ManifestBytes.LongLength
    }
    $Decoded = Get-EntryText -Entry $ManifestTextEntry
    if (-not $Decoded.Success) {
        $script:ManifestResult = 'FAIL'
        $script:InventoryConsistencyResult = 'FAIL'
        return
    }

    $ManifestText = [string]$Decoded.Text
    if ($ManifestText.Length -gt 0 -and [int][char]$ManifestText[0] -eq 0xFEFF) {
        $ManifestText = $ManifestText.Substring(1)
    }

    $ManifestRecords = [System.Collections.Generic.List[object]]::new()
    $ManifestPaths = [System.Collections.Generic.HashSet[string]]::new(
        [System.StringComparer]::OrdinalIgnoreCase
    )
    $LineNumber = 0
    foreach ($RawLine in $ManifestText.Split([char]10)) {
        $LineNumber++
        $Line = $RawLine.TrimEnd([char]13)
        if ($Line.Length -eq 0 -and $LineNumber -eq $ManifestText.Split([char]10).Count) {
            continue
        }

        $Match = [regex]::Match(
            $Line,
            '^(?<hash>[0-9a-f]{64})  (?<size>0|[1-9][0-9]*)  (?<path>.+)$'
        )
        if (-not $Match.Success) {
            Add-Failure -Code 'MANIFEST_FORMAT_INVALID' -Entry $ManifestTextEntry.Path `
                -Offset $LineNumber -Detail 'Expected: lowercase-sha256, two spaces, size, two spaces, path.'
            continue
        }

        $RelativePath = $Match.Groups['path'].Value
        if (-not (Test-SafeRelativePath -Path $RelativePath)) {
            Add-Failure -Code 'MANIFEST_UNSAFE_PATH' -Entry $RelativePath -Offset $LineNumber `
                -Detail 'Manifest path is not a safe canonical relative path.'
            continue
        }
        if (-not $ManifestPaths.Add($RelativePath)) {
            Add-Failure -Code 'MANIFEST_DUPLICATE_PATH' -Entry $RelativePath -Offset $LineNumber `
                -Detail 'Manifest paths must be unique.'
            continue
        }

        [void]$ManifestRecords.Add([pscustomobject]@{
            Hash = $Match.Groups['hash'].Value
            Size = [long]::Parse(
                $Match.Groups['size'].Value,
                [System.Globalization.CultureInfo]::InvariantCulture
            )
            Path = $RelativePath
        })
    }

    $RecordPaths = @($ManifestRecords | ForEach-Object Path)
    $SortedPaths = [string[]]@($RecordPaths)
    [array]::Sort($SortedPaths, [System.StringComparer]::Ordinal)
    for ($Index = 0; $Index -lt $RecordPaths.Count; $Index++) {
        if ($RecordPaths[$Index] -cne $SortedPaths[$Index]) {
            Add-Failure -Code 'MANIFEST_ORDER_NONDETERMINISTIC' -Entry $ManifestTextEntry.Path `
                -Offset ($Index + 1) -Detail 'Manifest paths must be ordinally sorted.'
            break
        }
    }

    $ExpectedEntries = @(
        $script:Entries |
            Where-Object {
                -not $ManifestEntryPath -or
                $_.Path -cne $ManifestEntryPath
            }
    )
    $ExpectedLookup = [System.Collections.Generic.Dictionary[string, object]]::new(
        [System.StringComparer]::OrdinalIgnoreCase
    )
    foreach ($Entry in $ExpectedEntries) {
        if ($ExpectedLookup.ContainsKey($Entry.Path)) {
            Add-Failure -Code 'INVENTORY_DUPLICATE_PATH' -Entry $Entry.Path -Offset $null `
                -Detail 'Artifact inventory contains a duplicate path.'
            continue
        }
        $ExpectedLookup.Add($Entry.Path, $Entry)
    }

    foreach ($Record in $ManifestRecords) {
        if (-not $ExpectedLookup.ContainsKey($Record.Path)) {
            Add-Failure -Code 'MANIFEST_EXTRA_ENTRY' -Entry $Record.Path -Offset $null `
                -Detail 'Manifest references a path absent from the artifact.'
            continue
        }

        $ActualEntry = $ExpectedLookup[$Record.Path]
        if ([long]$ActualEntry.Length -ne [long]$Record.Size) {
            Add-Failure -Code 'MANIFEST_SIZE_MISMATCH' -Entry $Record.Path -Offset $null `
                -Detail 'Manifest size does not match artifact bytes.'
        }
        $ActualHash = Get-Sha256Hex -Bytes ([byte[]]$ActualEntry.Bytes)
        if ($ActualHash -cne $Record.Hash) {
            Add-Failure -Code 'MANIFEST_HASH_MISMATCH' -Entry $Record.Path -Offset $null `
                -Detail 'Manifest SHA-256 does not match artifact bytes.'
        }
    }

    foreach ($ExpectedEntry in $ExpectedEntries) {
        if (-not $ManifestPaths.Contains($ExpectedEntry.Path)) {
            Add-Failure -Code 'MANIFEST_ENTRY_MISSING' -Entry $ExpectedEntry.Path -Offset $null `
                -Detail 'Artifact content is not covered exactly once by the manifest.'
        }
    }

    $ManifestFailureCodes = @(
        $script:Failures |
            Where-Object {
                $_.Code -like 'MANIFEST_*' -or $_.Code -eq 'INVENTORY_DUPLICATE_PATH'
            }
    )
    $script:ManifestResult = if ($ManifestFailureCodes.Count -eq 0) { 'PASS' } else { 'FAIL' }
    $script:InventoryConsistencyResult = $script:ManifestResult
}

function New-ResultObject {
    [pscustomobject][ordered]@{
        Status                         = if ($script:InvocationProblem) {
            'ERROR'
        }
        elseif ($script:Failures.Count -eq 0) {
            'PASS'
        }
        else {
            'FAIL'
        }
        ArtifactPath                   = $script:ResolvedArtifactPath
        ArtifactType                   = $script:ArtifactType
        StrictUtf8Result               = $script:StrictUtf8Result
        ReplacementCharacterResult     = $script:ReplacementCharacterResult
        ControlCharacterResult         = $script:ControlCharacterResult
        PlaceholderResult              = $script:PlaceholderResult
        ObjectInterpolationResult      = $script:ObjectInterpolationResult
        ManifestResult                 = $script:ManifestResult
        ZipReopenResult                = $script:ZipReopenResult
        ZipPathSafetyResult            = $script:ZipPathSafetyResult
        InventoryConsistencyResult     = $script:InventoryConsistencyResult
        ClassicReviewReadyObserved     = $script:ClassicReviewReadyObserved
        ClassicReviewReadyAllowed      = $script:ClassicReviewReadyAllowed
        ReadinessRequirement           = $ReadinessRequirement
        AllowedLiteralPlaceholder      = @($script:Allowlist)
        AllowlistMatchCount            = $script:AllowlistMatches.Count
        AllowlistMatches               = @($script:AllowlistMatches)
        WarningCount                   = $script:Warnings.Count
        FailureCount                   = $script:Failures.Count
        Failures                       = @($script:Failures)
        Warnings                       = @($script:Warnings)
    }
}

try {
    if ($InvocationProblem) {
        throw [System.ArgumentException]::new('Invalid exact allowlist configuration.')
    }

    $ArtifactItem = Get-Item -LiteralPath $ArtifactPath -ErrorAction Stop
    $ResolvedArtifactPath = $ArtifactItem.FullName

    if ($ArtifactItem.PSIsContainer) {
        $ArtifactType = 'Directory'
    }
    elseif ($ArtifactItem.Extension -ieq '.zip') {
        $ArtifactType = 'Zip'
    }
    else {
        $ArtifactType = 'File'
    }

    if ($ReportPath) {
        $ResolvedReportPath = [System.IO.Path]::GetFullPath($ReportPath)
        if ($ArtifactType -eq 'Directory') {
            $DirectoryPrefix = $ResolvedArtifactPath.TrimEnd('\', '/') +
                [System.IO.Path]::DirectorySeparatorChar
            if ($ResolvedReportPath.StartsWith(
                    $DirectoryPrefix,
                    [System.StringComparison]::OrdinalIgnoreCase
                )) {
                throw [System.ArgumentException]::new(
                    'ReportPath must be outside the artifact directory.'
                )
            }
        }
        elseif ($ResolvedReportPath -eq $ResolvedArtifactPath) {
            throw [System.ArgumentException]::new('ReportPath must not overwrite the artifact.')
        }
    }

    if ($ArtifactType -eq 'Zip' -and $ManifestPath) {
        throw [System.ArgumentException]::new(
            'ZIP artifacts use the root internal MANIFEST.sha256; external ManifestPath is unsupported.'
        )
    }

    if ($ArtifactType -eq 'Directory') {
        foreach ($File in Get-ChildItem -LiteralPath $ResolvedArtifactPath -File -Recurse) {
            if (($File.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
                Add-Failure -Code 'DIRECTORY_REPARSE_POINT' -Entry $File.FullName -Offset $null `
                    -Detail 'Reparse-point files are forbidden in artifact directories.'
            }
            $RelativePath = [System.IO.Path]::GetRelativePath(
                $ResolvedArtifactPath,
                $File.FullName
            ).Replace('\', '/')
            [void]$Entries.Add([pscustomobject]@{
                Path               = $RelativePath
                Bytes              = [System.IO.File]::ReadAllBytes($File.FullName)
                Length             = [long]$File.Length
                ExternalAttributes = 0
            })
        }
    }
    elseif ($ArtifactType -eq 'Zip') {
        Read-ZipEntries -Path $ResolvedArtifactPath
        if ($ZipReopenResult -eq 'PASS') {
            Test-ZipPaths
        }
    }
    else {
        [void]$Entries.Add([pscustomobject]@{
            Path               = $ArtifactItem.Name
            Bytes              = [System.IO.File]::ReadAllBytes($ResolvedArtifactPath)
            Length             = [long]$ArtifactItem.Length
            ExternalAttributes = 0
        })
    }

    $Utf8FailureCountBefore = @($Failures | Where-Object Code -eq 'STRICT_UTF8_INVALID').Count
    $ReplacementFailureCountBefore = @(
        $Failures | Where-Object Code -eq 'REPLACEMENT_CHARACTER_PRESENT'
    ).Count
    $ControlFailureCountBefore = @(
        $Failures |
            Where-Object Code -in @(
                'ISOLATED_CARRIAGE_RETURN',
                'UNEXPECTED_TAB',
                'FORBIDDEN_CONTROL_CHARACTER'
            )
    ).Count
    $PlaceholderFailureCountBefore = @(
        $Failures |
            Where-Object Code -in @(
                'UNRESOLVED_POWERSHELL_VARIABLE',
                'UNRESOLVED_SUBEXPRESSION',
                'UNRESOLVED_TEMPLATE_TOKEN'
            )
    ).Count
    $ObjectFailureCountBefore = @(
        $Failures |
            Where-Object Code -in @(
                'NON_SCALAR_OBJECT_ARRAY',
                'NON_SCALAR_HASHTABLE_DUMP',
                'NON_SCALAR_SUBEXPRESSION'
            )
    ).Count

    foreach ($Entry in $Entries) {
        $Extension = [System.IO.Path]::GetExtension($Entry.Path)
        if ($TextExtensions.Contains($Extension)) {
            Test-TextEntry -Entry $Entry
        }
    }

    $StrictUtf8Result = if (
        @($Failures | Where-Object Code -eq 'STRICT_UTF8_INVALID').Count -eq
        $Utf8FailureCountBefore
    ) { 'PASS' } else { 'FAIL' }
    $ReplacementCharacterResult = if (
        @($Failures | Where-Object Code -eq 'REPLACEMENT_CHARACTER_PRESENT').Count -eq
        $ReplacementFailureCountBefore
    ) { 'PASS' } else { 'FAIL' }
    $ControlCharacterResult = if (
        @(
            $Failures |
                Where-Object Code -in @(
                    'ISOLATED_CARRIAGE_RETURN',
                    'UNEXPECTED_TAB',
                    'FORBIDDEN_CONTROL_CHARACTER'
                )
        ).Count -eq $ControlFailureCountBefore
    ) { 'PASS' } else { 'FAIL' }
    $PlaceholderResult = if (
        @(
            $Failures |
                Where-Object Code -in @(
                    'UNRESOLVED_POWERSHELL_VARIABLE',
                    'UNRESOLVED_SUBEXPRESSION',
                    'UNRESOLVED_TEMPLATE_TOKEN'
                )
        ).Count -eq $PlaceholderFailureCountBefore
    ) { 'PASS' } else { 'FAIL' }
    $ObjectInterpolationResult = if (
        @(
            $Failures |
                Where-Object Code -in @(
                    'NON_SCALAR_OBJECT_ARRAY',
                    'NON_SCALAR_HASHTABLE_DUMP',
                    'NON_SCALAR_SUBEXPRESSION'
                )
        ).Count -eq $ObjectFailureCountBefore
    ) { 'PASS' } else { 'FAIL' }

    $ManifestBytes = $null
    $ManifestEntryPath = $null
    $ManifestRequired = $ArtifactType -in @('Directory', 'Zip')

    if ($ArtifactType -eq 'Zip') {
        $ManifestCandidates = @($Entries | Where-Object Path -ceq 'MANIFEST.sha256')
        if ($ManifestCandidates.Count -eq 1) {
            $ManifestBytes = [byte[]]$ManifestCandidates[0].Bytes
            $ManifestEntryPath = 'MANIFEST.sha256'
        }
        elseif ($ManifestCandidates.Count -gt 1) {
            Add-Failure -Code 'MANIFEST_DUPLICATE_PATH' -Entry 'MANIFEST.sha256' -Offset $null `
                -Detail 'ZIP contains more than one root manifest.'
        }
    }
    elseif ($ArtifactType -eq 'Directory') {
        if ($ManifestPath) {
            $ManifestItem = Get-Item -LiteralPath $ManifestPath -ErrorAction Stop
            if ($ManifestItem.PSIsContainer) {
                throw [System.ArgumentException]::new('ManifestPath must be a file.')
            }
            $ManifestBytes = [System.IO.File]::ReadAllBytes($ManifestItem.FullName)
            if ($ManifestItem.FullName.StartsWith(
                    $ResolvedArtifactPath.TrimEnd('\', '/') +
                        [System.IO.Path]::DirectorySeparatorChar,
                    [System.StringComparison]::OrdinalIgnoreCase
                )) {
                $ManifestEntryPath = [System.IO.Path]::GetRelativePath(
                    $ResolvedArtifactPath,
                    $ManifestItem.FullName
                ).Replace('\', '/')
            }
        }
        else {
            $DefaultManifestPath = Join-Path $ResolvedArtifactPath 'MANIFEST.sha256'
            if (Test-Path -LiteralPath $DefaultManifestPath -PathType Leaf) {
                $ManifestBytes = [System.IO.File]::ReadAllBytes($DefaultManifestPath)
                $ManifestEntryPath = 'MANIFEST.sha256'
            }
        }
    }
    elseif ($ManifestPath) {
        $ManifestItem = Get-Item -LiteralPath $ManifestPath -ErrorAction Stop
        if ($ManifestItem.PSIsContainer) {
            throw [System.ArgumentException]::new('ManifestPath must be a file.')
        }
        $ManifestBytes = [System.IO.File]::ReadAllBytes($ManifestItem.FullName)
        $ManifestRequired = $true
    }

    Test-Manifest -ManifestBytes $ManifestBytes -ManifestEntryPath $ManifestEntryPath `
        -Required $ManifestRequired

    $DistinctReadinessValues = @($ReadinessValues | Sort-Object -Unique)
    if ($DistinctReadinessValues.Count -eq 1) {
        $ClassicReviewReadyObserved = if ($DistinctReadinessValues[0] -eq 'true') {
            'True'
        }
        else {
            'False'
        }
    }
    elseif ($DistinctReadinessValues.Count -gt 1) {
        $ClassicReviewReadyObserved = 'Conflict'
        Add-Failure -Code 'READINESS_CONFLICT' -Entry $null -Offset $null `
            -Detail 'Artifact contains conflicting ClassicReviewReady values.'
    }

    $GateFailuresBeforeReadiness = $Failures.Count
    $ClassicReviewReadyAllowed = (
        $ClassicReviewReadyObserved -eq 'True' -and
        $GateFailuresBeforeReadiness -eq 0
    )

    if ($ReadinessRequirement -eq 'RequireTrue' -and
        $ClassicReviewReadyObserved -ne 'True') {
        Add-Failure -Code 'READINESS_NOT_TRUE' -Entry $null -Offset $null `
            -Detail 'ClassicReviewReady=True is required for review packages.'
    }
    elseif ($ClassicReviewReadyObserved -eq 'True' -and
        $GateFailuresBeforeReadiness -gt 0) {
        Add-Failure -Code 'READINESS_FAIL_CLOSED' -Entry $null -Offset $null `
            -Detail 'Observed readiness cannot be allowed while any mandatory gate fails.'
    }

    if ($Failures.Count -eq 0) {
        $ExitCode = 0
    }
}
catch [System.ArgumentException] {
    $InvocationProblem = $true
    $ExitCode = 2
    if (@($Failures | Where-Object Code -like 'INVALID_*').Count -eq 0) {
        Add-Failure -Code 'INVOCATION_ERROR' -Entry $null -Offset $null `
            -Detail $_.Exception.Message
    }
}
catch [System.Management.Automation.ItemNotFoundException] {
    $InvocationProblem = $true
    $ExitCode = 2
    Add-Failure -Code 'PATH_NOT_FOUND' -Entry $null -Offset $null `
        -Detail 'ArtifactPath or ManifestPath does not exist.'
}
catch {
    $ExitCode = 1
    Add-Failure -Code 'VALIDATOR_RUNTIME_ERROR' -Entry $null -Offset $null `
        -Detail $_.Exception.GetType().FullName
}
finally {
    $Result = New-ResultObject

    if ($ReportPath) {
        try {
            $ResolvedReportPath = [System.IO.Path]::GetFullPath($ReportPath)
            $ReportDirectory = [System.IO.Path]::GetDirectoryName($ResolvedReportPath)
            if (-not [string]::IsNullOrEmpty($ReportDirectory)) {
                [void][System.IO.Directory]::CreateDirectory($ReportDirectory)
            }
            $Json = $Result | ConvertTo-Json -Depth 8
            [System.IO.File]::WriteAllText(
                $ResolvedReportPath,
                $Json + [System.Environment]::NewLine,
                [System.Text.UTF8Encoding]::new($false, $true)
            )
        }
        catch {
            $InvocationProblem = $true
            $ExitCode = 2
            Add-Failure -Code 'REPORT_WRITE_FAILED' -Entry $null -Offset $null `
                -Detail $_.Exception.GetType().FullName
            $Result = New-ResultObject
        }
    }

    $Result
    exit $ExitCode
}
