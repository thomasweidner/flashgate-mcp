[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$RootPath = Split-Path -Parent $PSScriptRoot
$ValidationScript = Join-Path $PSScriptRoot 'Build-InputValidation.ps1'
$FixturePath = Join-Path `
    $RootPath `
    'internal\version\testdata\build-input-validation-fixtures.json'
$Errors = [System.Collections.Generic.List[string]]::new()
$ExitCode = 1

try {
    . $ValidationScript
    $Fixtures = Get-Content -LiteralPath $FixturePath -Raw |
        ConvertFrom-Json -Depth 10 -DateKind String
    if (
        $Fixtures.schema -cne
        'flashgate-build-input-validation-fixtures/v1'
    ) {
        throw "Unexpected fixture schema: $($Fixtures.schema)"
    }

    foreach ($Fixture in $Fixtures.semanticVersions.valid) {
        try {
            $Result = Get-FlashGateSemanticVersion -Value $Fixture.value
            if ($Result.FileVersion -cne $Fixture.fileVersion) {
                throw (
                    "Expected file version '$($Fixture.fileVersion)'; found " +
                    "'$($Result.FileVersion)'."
                )
            }
        }
        catch {
            $Errors.Add(
                "Valid SemVer '$($Fixture.value)' failed: $($_.Exception.Message)"
            )
        }
    }
    foreach ($Value in $Fixtures.semanticVersions.invalid) {
        try {
            $null = Get-FlashGateSemanticVersion -Value ([string]$Value)
            $Errors.Add("Invalid SemVer '$Value' unexpectedly passed.")
        }
        catch {
        }
    }

    $WorkRoot = $env:FLASHGATE_WORK_ROOT
    if ([string]::IsNullOrWhiteSpace($WorkRoot) -or
        -not (Test-Path -LiteralPath $WorkRoot -PathType Container)) {
        throw 'FLASHGATE_WORK_ROOT must name an existing work directory.'
    }
    $FixtureRoot = Join-Path $WorkRoot ('flashgate-version-' + [guid]::NewGuid().ToString('N'))
    $null = New-Item -ItemType Directory -Path $FixtureRoot -ErrorAction Stop
    $VersionPath = Join-Path $FixtureRoot 'VERSION'
    try {
        try {
            $null = Get-FlashGateRepositoryVersion -RootPath $FixtureRoot
            $Errors.Add('Missing repository VERSION unexpectedly passed.')
        }
        catch {
        }
        $Utf8 = [Text.UTF8Encoding]::new($false)
        $VersionCases = @(
            @{ Name = 'stable without LF'; Bytes = $Utf8.GetBytes('0.1.0'); Valid = $true }
            @{ Name = 'stable with LF'; Bytes = $Utf8.GetBytes("0.1.0`n"); Valid = $true }
            @{ Name = 'prerelease'; Bytes = $Utf8.GetBytes('1.2.3-rc.1'); Valid = $true }
            @{ Name = 'empty'; Bytes = [byte[]]@(); Valid = $false }
            @{ Name = 'whitespace'; Bytes = $Utf8.GetBytes(" 0.1.0`n"); Valid = $false }
            @{ Name = 'BOM'; Bytes = [byte[]]@(0xef, 0xbb, 0xbf) + $Utf8.GetBytes("0.1.0`n"); Valid = $false }
            @{ Name = 'leading v'; Bytes = $Utf8.GetBytes("v0.1.0`n"); Valid = $false }
            @{ Name = 'multiline'; Bytes = $Utf8.GetBytes("0.1.0`n1.0.0`n"); Valid = $false }
            @{ Name = 'CRLF'; Bytes = $Utf8.GetBytes("0.1.0`r`n"); Valid = $false }
            @{ Name = 'CR'; Bytes = $Utf8.GetBytes("0.1.0`r"); Valid = $false }
            @{ Name = 'trailing whitespace'; Bytes = $Utf8.GetBytes("0.1.0 `n"); Valid = $false }
            @{ Name = 'NUL only'; Bytes = [byte[]]@(0); Valid = $false }
            @{ Name = 'NUL final'; Bytes = $Utf8.GetBytes('0.1.0') + [byte[]]@(0); Valid = $false }
            @{ Name = 'NUL internal'; Bytes = $Utf8.GetBytes('0.1') + [byte[]]@(0) + $Utf8.GetBytes('.0'); Valid = $false }
            @{ Name = 'NUL before LF'; Bytes = $Utf8.GetBytes('0.1.0') + [byte[]]@(0, 10); Valid = $false }
            @{ Name = 'NUL leading'; Bytes = [byte[]]@(0) + $Utf8.GetBytes(".1.0`n"); Valid = $false }
            @{ Name = 'invalid'; Bytes = $Utf8.GetBytes("1.2`n"); Valid = $false }
            @{ Name = 'overflow'; Bytes = $Utf8.GetBytes("65536.0.0`n"); Valid = $false }
            @{ Name = 'leading zero'; Bytes = $Utf8.GetBytes("1.2.3-rc.01`n"); Valid = $false }
        )
        foreach ($Case in $VersionCases) {
            [IO.File]::WriteAllBytes($VersionPath, [byte[]]$Case.Bytes)
            try {
                $Actual = Get-FlashGateRepositoryVersion -RootPath $FixtureRoot
                if (-not $Case.Valid) {
                    $Errors.Add("Invalid repository VERSION '$($Case.Name)' passed.")
                }
                elseif ($Actual -cne ([Text.Encoding]::UTF8.GetString($Case.Bytes)).TrimEnd("`n")) {
                    $Errors.Add("Repository VERSION '$($Case.Name)' changed value.")
                }
            }
            catch {
                if ($Case.Valid) {
                    $Errors.Add("Valid repository VERSION '$($Case.Name)' failed: $($_.Exception.Message)")
                }
            }
        }
    }
    finally {
        if (Test-Path -LiteralPath $VersionPath) {
            Remove-Item -LiteralPath $VersionPath -Force
        }
        Remove-Item -LiteralPath $FixtureRoot -Force
    }

    foreach ($Fixture in $Fixtures.sourceDateEpoch.valid) {
        try {
            $Actual = ConvertFrom-FlashGateSourceDateEpoch `
                -Value $Fixture.value
            if ($Actual -cne $Fixture.sourceTime) {
                throw (
                    "Expected source time '$($Fixture.sourceTime)'; found " +
                    "'$Actual'."
                )
            }
        }
        catch {
            $Errors.Add(
                "Valid epoch '$($Fixture.value)' failed: $($_.Exception.Message)"
            )
        }
    }
    foreach ($Value in $Fixtures.sourceDateEpoch.invalid) {
        try {
            $null = ConvertFrom-FlashGateSourceDateEpoch -Value ([string]$Value)
            $Errors.Add("Invalid epoch '$Value' unexpectedly passed.")
        }
        catch {
        }
    }

    if ($Errors.Count -eq 0) {
        $ExitCode = 0
    }
}
catch {
    $Errors.Add($_.Exception.Message)
}
finally {
    [pscustomobject]@{
        Status       = if ($Errors.Count -eq 0) { 'PASS' } else { 'FAIL' }
        FixturePath  = $FixturePath
        WarningCount = 0
        ErrorCount   = $Errors.Count
        Warnings     = $null
        Errors       = if ($Errors.Count -gt 0) {
            $Errors -join [Environment]::NewLine
        } else {
            $null
        }
    } | Format-List
}

exit $ExitCode
