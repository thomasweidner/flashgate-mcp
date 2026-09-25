$script:FlashGateMaximumSourceDateEpoch = [int64]253402300799

function Get-FlashGateRepositoryVersion {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string] $RootPath
    )

    $VersionPath = Join-Path $RootPath 'VERSION'
    if (-not (Test-Path -LiteralPath $VersionPath -PathType Leaf)) {
        throw "Repository VERSION file not found: $VersionPath"
    }
    $Attributes = [IO.File]::GetAttributes($VersionPath)
    if (($Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw 'Repository VERSION must be a regular file, not a link.'
    }
    $Bytes = [IO.File]::ReadAllBytes($VersionPath)
    if ($Bytes.Length -eq 0 -or $Bytes.Length -gt 128) {
        throw 'Repository VERSION must contain one bounded SemVer value.'
    }
    if ($Bytes.Length -ge 3 -and $Bytes[0] -eq 0xEF -and
        $Bytes[1] -eq 0xBB -and $Bytes[2] -eq 0xBF) {
        throw 'Repository VERSION must not have a UTF-8 BOM.'
    }
    $Utf8 = [Text.UTF8Encoding]::new($false, $true)
    $Text = $Utf8.GetString($Bytes)
    if ($Text.EndsWith("`n", [StringComparison]::Ordinal)) {
        $Text = $Text.Substring(0, $Text.Length - 1)
    }
    if ($Text.Contains("`n") -or $Text.Contains("`r")) {
        throw 'Repository VERSION must contain exactly one line with LF ending.'
    }
    $null = Get-FlashGateSemanticVersion -Value $Text
    return $Text
}

function Get-FlashGateSemanticVersion {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [AllowEmptyString()]
        [string] $Value
    )

    if (
        [string]::IsNullOrEmpty($Value) -or
        $Value -cne $Value.Trim() -or
        $Value.IndexOfAny([char[]]"`r`n`t") -ge 0
    ) {
        throw "Invalid semantic version: $Value"
    }

    $Pattern =
        '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)' +
        '(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?' +
        '(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?\z'
    $Match = [regex]::Match(
        $Value,
        $Pattern,
        [Text.RegularExpressions.RegexOptions]::CultureInvariant
    )
    if (-not $Match.Success) {
        throw "Invalid semantic version: $Value"
    }

    if ($Match.Groups[4].Success) {
        foreach ($Identifier in $Match.Groups[4].Value.Split('.')) {
            if (
                $Identifier -match '^[0-9]+$' -and
                $Identifier.Length -gt 1 -and
                $Identifier[0] -eq '0'
            ) {
                throw "Invalid numeric prerelease identifier: $Value"
            }
        }
    }

    $Components = foreach ($Index in 1..3) {
        $Parsed = [uint16]0
        if (
            -not [uint16]::TryParse(
                $Match.Groups[$Index].Value,
                [Globalization.NumberStyles]::None,
                [Globalization.CultureInfo]::InvariantCulture,
                [ref]$Parsed
            )
        ) {
            throw "Version component exceeds the Windows 16-bit range: $Value"
        }
        $Parsed
    }

    [pscustomobject]@{
        Value       = $Value
        Major       = $Components[0]
        Minor       = $Components[1]
        Patch       = $Components[2]
        FileVersion = '{0}.{1}.{2}.0' -f $Components
    }
}

function ConvertFrom-FlashGateSourceDateEpoch {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [AllowEmptyString()]
        [string] $Value
    )

    if ($Value -notmatch '^[0-9]+$') {
        throw 'SOURCE_DATE_EPOCH must contain nonnegative decimal digits only.'
    }

    $Epoch = [int64]0
    if (
        -not [int64]::TryParse(
            $Value,
            [Globalization.NumberStyles]::None,
            [Globalization.CultureInfo]::InvariantCulture,
            [ref]$Epoch
        ) -or
        $Epoch -gt $script:FlashGateMaximumSourceDateEpoch
    ) {
        throw 'SOURCE_DATE_EPOCH is outside the supported range.'
    }

    [DateTimeOffset]::FromUnixTimeSeconds($Epoch).UtcDateTime.ToString(
        'yyyy-MM-ddTHH:mm:ssZ',
        [Globalization.CultureInfo]::InvariantCulture
    )
}
