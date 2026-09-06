[CmdletBinding()]
param([string]$WorkingPath)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
Import-Module (Join-Path $PSScriptRoot 'TaskBoundWorkRoot.psm1') -Force
$WorkRoot = Resolve-FlashGateWorkRoot -WorkingPath $WorkingPath

$Failures = [System.Collections.Generic.List[string]]::new()
$Notes = [System.Collections.Generic.List[string]]::new()
$Junctions = [System.Collections.Generic.List[string]]::new()
$CheckCount = 0
$TemporaryDirectory = $null
$OriginalPath = $env:PATH

function Test-Condition {
    param([bool]$Condition, [string]$Name)
    $script:CheckCount++
    if (-not $Condition) { $script:Failures.Add($Name) }
}

function Invoke-TestWrapper {
    param([string]$WrapperPath, [string[]]$Arguments)

    $ProcessInfo = [Diagnostics.ProcessStartInfo]::new()
    $ProcessInfo.FileName = (Get-Process -Id $PID).Path
    $ProcessInfo.UseShellExecute = $false
    $ProcessInfo.RedirectStandardOutput = $true
    $ProcessInfo.RedirectStandardError = $true
    foreach ($Argument in @('-NoProfile', '-File', $WrapperPath) + $Arguments) {
        [void]$ProcessInfo.ArgumentList.Add($Argument)
    }
    $Process = [Diagnostics.Process]::Start($ProcessInfo)
    try {
        $StandardOutput = $Process.StandardOutput.ReadToEnd()
        $StandardError = $Process.StandardError.ReadToEnd()
        $Process.WaitForExit()
        return [pscustomobject]@{ ExitCode = $Process.ExitCode; StandardOutput = $StandardOutput; StandardError = $StandardError }
    }
    finally {
        $Process.Dispose()
    }
}

function Remove-TestMarkers {
    foreach ($Path in @($script:GitMarker, $script:GoMarker)) {
        Remove-Item -LiteralPath $Path -Force -ErrorAction SilentlyContinue
    }
}

function Test-BlockedCanonicalPath {
    param(
        [Parameter(Mandatory)][string]$Name,
        [Parameter(Mandatory)][string]$Path
    )

    Remove-TestMarkers
    $Result = Invoke-TestWrapper -WrapperPath $script:Wrapper -Arguments @('-OutputPath', $Path)
    $CombinedOutput = $Result.StandardOutput + "`n" + $Result.StandardError
    Test-Condition ($Result.ExitCode -ne 0) "$Name exits nonzero"
    Test-Condition ($CombinedOutput -match 'Non-authoritative benchmark runs must not') "$Name emits policy error"
    Test-Condition (-not (Test-Path -LiteralPath $script:GitMarker)) "$Name does not invoke git"
    Test-Condition (-not (Test-Path -LiteralPath $script:GoMarker)) "$Name does not invoke go"
    Test-Condition (-not (Test-Path -LiteralPath (Join-Path $script:FakeRepository 'build'))) "$Name does not create build directory"
    Test-Condition (-not (Test-Path -LiteralPath $Path -PathType Leaf)) "$Name does not write output"
    Test-Condition ($Result.StandardOutput -notmatch 'Status\s*:\s*PASS') "$Name does not emit success output"
}

try {
    $TemporaryDirectory = New-FlashGateScratchDirectory -WorkingPath $WorkRoot -Prefix 'record-policy-test'
    $FakeRepository = Join-Path $TemporaryDirectory 'repo'
    $FakeScripts = Join-Path $FakeRepository 'scripts'
    $FakeBenchmarks = Join-Path $FakeRepository 'benchmarks'
    $AliasRoot = Join-Path $TemporaryDirectory 'aliases'
    $StubDirectory = Join-Path $TemporaryDirectory 'stub'
    $DiagnosticDirectory = Join-Path $TemporaryDirectory 'diagnostics'
    New-Item -ItemType Directory -Path $FakeScripts, $FakeBenchmarks, $AliasRoot, $StubDirectory, $DiagnosticDirectory | Out-Null

    $Wrapper = Join-Path $FakeScripts 'benchmark.ps1'
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'benchmark.ps1') -Destination $Wrapper
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'benchmark-window.ps1') -Destination (Join-Path $FakeScripts 'benchmark-window.ps1')
    $GitMarker = Join-Path $TemporaryDirectory 'git-invoked.marker'
    $GoMarker = Join-Path $TemporaryDirectory 'go-invoked.marker'
    $GoStub = Join-Path $StubDirectory 'go.cmd'
    $GitStub = Join-Path $StubDirectory 'git.cmd'
    @('@echo off', ('type nul > "{0}"' -f $GoMarker), 'exit /b 97') | Set-Content -LiteralPath $GoStub -Encoding ascii
    @('@echo off', ('type nul > "{0}"' -f $GitMarker), 'exit /b 97') | Set-Content -LiteralPath $GitStub -Encoding ascii
    $env:PATH = $StubDirectory + [IO.Path]::PathSeparator + $OriginalPath

    $RecordOutput = Join-Path $TemporaryDirectory 'baseline.windows-amd64.json'
    $RecordResult = Invoke-TestWrapper -WrapperPath $Wrapper -Arguments @('-RecordBaseline', '-OutputPath', $RecordOutput)
    Test-Condition ($RecordResult.ExitCode -ne 0) 'record mode exits nonzero'
    Test-Condition ($RecordResult.StandardError -match 'Authoritative baseline recording is not supported by scripts/benchmark.ps1') 'record mode emits policy error on stderr'
    Test-Condition (-not (Test-Path -LiteralPath $GitMarker)) 'record mode does not invoke git'
    Test-Condition (-not (Test-Path -LiteralPath $GoMarker)) 'record mode does not invoke go'
    Test-Condition (-not (Test-Path -LiteralPath $RecordOutput)) 'record mode does not write baseline output'
    Test-Condition (-not (Test-Path -LiteralPath (Join-Path $FakeRepository 'build'))) 'record mode does not create build directory'
    Test-Condition ([string]::IsNullOrWhiteSpace($RecordResult.StandardOutput)) 'record mode does not emit success output'

    Test-BlockedCanonicalPath -Name 'direct canonical path' -Path (Join-Path $FakeBenchmarks 'baseline.windows-amd64.json')
    $ExtendedCanonicalPath = '\\?\' + (Join-Path $FakeBenchmarks 'baseline.windows-amd64.json')
    Test-BlockedCanonicalPath -Name 'extended-length canonical path' -Path $ExtendedCanonicalPath

    $DirectJunction = Join-Path $AliasRoot 'alias-benchmarks'
    New-Item -ItemType Junction -Path $DirectJunction -Target $FakeBenchmarks | Out-Null
    $Junctions.Add($DirectJunction)
    Test-BlockedCanonicalPath -Name 'direct benchmark junction' -Path (Join-Path $DirectJunction 'baseline.windows-amd64.json')
    Test-BlockedCanonicalPath -Name 'noncanonical benchmark junction' -Path (Join-Path $DirectJunction 'benchmark-current.windows-amd64.json')

    $RepositoryJunction = Join-Path $AliasRoot 'alias-repository'
    New-Item -ItemType Junction -Path $RepositoryJunction -Target $FakeRepository | Out-Null
    $Junctions.Add($RepositoryJunction)
    Test-BlockedCanonicalPath -Name 'repository parent junction' -Path (Join-Path $RepositoryJunction 'benchmarks\baseline.windows-amd64.json')

    $ChainTarget = Join-Path $AliasRoot 'chain-target'
    $ChainEntry = Join-Path $AliasRoot 'chain-entry'
    New-Item -ItemType Junction -Path $ChainTarget -Target $FakeBenchmarks | Out-Null
    $Junctions.Add($ChainTarget)
    New-Item -ItemType Junction -Path $ChainEntry -Target $ChainTarget | Out-Null
    $Junctions.Add($ChainEntry)
    Test-BlockedCanonicalPath -Name 'two-level junction chain' -Path (Join-Path $ChainEntry 'baseline.windows-amd64.json')

    $DotPath = Join-Path $DirectJunction '.\unused\..\baseline.windows-amd64.json'
    Test-BlockedCanonicalPath -Name 'junction with dot segments' -Path $DotPath
    $MixedPath = ($DirectJunction -replace '\\', '/') + '\baseline.windows-amd64.json'
    Test-BlockedCanonicalPath -Name 'junction with mixed separators' -Path $MixedPath

    $MissingParent = Join-Path $TemporaryDirectory 'missing\parent\baseline.windows-amd64.json'
    Test-BlockedCanonicalPath -Name 'unresolved canonical parent' -Path $MissingParent

    $BrokenTarget = Join-Path $TemporaryDirectory 'broken-target'
    $BrokenJunction = Join-Path $AliasRoot 'broken-junction'
    New-Item -ItemType Directory -Path $BrokenTarget | Out-Null
    New-Item -ItemType Junction -Path $BrokenJunction -Target $BrokenTarget | Out-Null
    $Junctions.Add($BrokenJunction)
    Remove-Item -LiteralPath $BrokenTarget
    Test-BlockedCanonicalPath -Name 'unresolved junction target' -Path (Join-Path $BrokenJunction 'baseline.windows-amd64.json')

    $DirectorySymlink = Join-Path $AliasRoot 'symlink-benchmarks'
    try {
        New-Item -ItemType SymbolicLink -Path $DirectorySymlink -Target $FakeBenchmarks -ErrorAction Stop | Out-Null
        $Junctions.Add($DirectorySymlink)
        Test-BlockedCanonicalPath -Name 'directory symlink' -Path (Join-Path $DirectorySymlink 'baseline.windows-amd64.json')
    }
    catch {
        $Notes.Add('Directory symlink creation is not supported by the current Windows configuration; mandatory junction coverage executed.')
    }

    $ExistingBaselineTarget = Join-Path $FakeBenchmarks 'baseline.existing-amd64.json'
    [IO.File]::WriteAllText($ExistingBaselineTarget, 'existing')
    $FileSymlink = Join-Path $DiagnosticDirectory 'baseline.existing-amd64.json'
    try {
        New-Item -ItemType SymbolicLink -Path $FileSymlink -Target $ExistingBaselineTarget -ErrorAction Stop | Out-Null
        $Junctions.Add($FileSymlink)
        Remove-TestMarkers
        $FileLinkResult = Invoke-TestWrapper -WrapperPath $Wrapper -Arguments @('-OutputPath', $FileSymlink)
        $FileLinkCombined = $FileLinkResult.StandardOutput + "`n" + $FileLinkResult.StandardError
        Test-Condition ($FileLinkResult.ExitCode -ne 0) 'existing file symlink exits nonzero'
        Test-Condition ($FileLinkCombined -match 'Non-authoritative benchmark runs must not') 'existing file symlink emits policy error'
        Test-Condition (-not (Test-Path -LiteralPath $GitMarker)) 'existing file symlink does not invoke git'
        Test-Condition (-not (Test-Path -LiteralPath $GoMarker)) 'existing file symlink does not invoke go'
        Test-Condition (([IO.File]::ReadAllText($ExistingBaselineTarget)) -eq 'existing') 'existing file symlink target remains unchanged'

        $NoncanonicalFileSymlink = Join-Path $DiagnosticDirectory 'benchmark-current.windows-amd64.json'
        New-Item -ItemType SymbolicLink -Path $NoncanonicalFileSymlink -Target $ExistingBaselineTarget -ErrorAction Stop | Out-Null
        $Junctions.Add($NoncanonicalFileSymlink)
        Remove-TestMarkers
        $NoncanonicalLinkResult = Invoke-TestWrapper -WrapperPath $Wrapper -Arguments @('-OutputPath', $NoncanonicalFileSymlink)
        $NoncanonicalLinkCombined = $NoncanonicalLinkResult.StandardOutput + "`n" + $NoncanonicalLinkResult.StandardError
        Test-Condition ($NoncanonicalLinkResult.ExitCode -ne 0) 'noncanonical file symlink exits nonzero'
        Test-Condition ($NoncanonicalLinkCombined -match 'final symbolic link or reparse-point') 'noncanonical file symlink emits policy error'
        Test-Condition (-not (Test-Path -LiteralPath $GitMarker)) 'noncanonical file symlink does not invoke git'
        Test-Condition (-not (Test-Path -LiteralPath $GoMarker)) 'noncanonical file symlink does not invoke go'
        Test-Condition (([IO.File]::ReadAllText($ExistingBaselineTarget)) -eq 'existing') 'noncanonical file symlink target remains unchanged'
        Remove-Item -LiteralPath $NoncanonicalFileSymlink -Force

        $BrokenFileTarget = Join-Path $DiagnosticDirectory 'broken-target.json'
        $BrokenFileSymlink = Join-Path $DiagnosticDirectory 'benchmark-broken.windows-amd64.json'
        [IO.File]::WriteAllText($BrokenFileTarget, 'temporary')
        New-Item -ItemType SymbolicLink -Path $BrokenFileSymlink -Target $BrokenFileTarget -ErrorAction Stop | Out-Null
        $Junctions.Add($BrokenFileSymlink)
        Remove-Item -LiteralPath $BrokenFileTarget
        Remove-TestMarkers
        $BrokenFileResult = Invoke-TestWrapper -WrapperPath $Wrapper -Arguments @('-OutputPath', $BrokenFileSymlink)
        $BrokenFileCombined = $BrokenFileResult.StandardOutput + "`n" + $BrokenFileResult.StandardError
        Test-Condition ($BrokenFileResult.ExitCode -ne 0) 'broken final file symlink exits nonzero'
        Test-Condition ($BrokenFileCombined -match 'final symbolic link or reparse-point') 'broken final file symlink emits policy error'
        Test-Condition (-not (Test-Path -LiteralPath $GitMarker)) 'broken final file symlink does not invoke git'
        Test-Condition (-not (Test-Path -LiteralPath $GoMarker)) 'broken final file symlink does not invoke go'
        Test-Condition (([IO.File]::ReadAllText($ExistingBaselineTarget)) -eq 'existing') 'broken final file symlink leaves baseline unchanged'
    }
    catch {
        $Notes.Add('File symlink creation is not supported by the current Windows configuration; directory reparse coverage executed.')
    }

    Remove-TestMarkers
    $DiagnosticOutput = Join-Path $DiagnosticDirectory 'benchmark-current.windows-amd64.json'
    $DiagnosticResult = Invoke-TestWrapper -WrapperPath $Wrapper -Arguments @('-Quick', '-OutputPath', $DiagnosticOutput)
    $DiagnosticCombined = $DiagnosticResult.StandardOutput + "`n" + $DiagnosticResult.StandardError
    Test-Condition ($DiagnosticResult.ExitCode -ne 0) 'diagnostic stub run exits nonzero'
    Test-Condition ($DiagnosticCombined -notmatch 'Non-authoritative benchmark runs must not') 'regular diagnostic output passes path policy'
    Test-Condition (Test-Path -LiteralPath $GitMarker -PathType Leaf) 'regular diagnostic output reaches git'
    Test-Condition (-not (Test-Path -LiteralPath $GoMarker)) 'git stub stops diagnostic run before go'
    Test-Condition (-not (Test-Path -LiteralPath $DiagnosticOutput)) 'diagnostic stub run does not write output'

    Remove-TestMarkers
    $MutableJunction = Join-Path $AliasRoot 'mutable-output'
    New-Item -ItemType Junction -Path $MutableJunction -Target $DiagnosticDirectory | Out-Null
    $Junctions.Add($MutableJunction)
    @(
        '@echo off',
        ('type nul > "{0}"' -f $GitMarker),
        ('rmdir "{0}"' -f $MutableJunction),
        ('mklink /J "{0}" "{1}" > nul' -f $MutableJunction, $FakeBenchmarks),
        'if "%1"=="rev-parse" echo da1175752064ed91dc3bca517e6148e711ba0ca1',
        'exit /b 0'
    ) | Set-Content -LiteralPath $GitStub -Encoding ascii
    @('@echo off', ('type nul > "{0}"' -f $GoMarker), 'exit /b 0') | Set-Content -LiteralPath $GoStub -Encoding ascii
    $ChangedOutput = Join-Path $MutableJunction 'benchmark-current.windows-amd64.json'
    $ChangedResult = Invoke-TestWrapper -WrapperPath $Wrapper -Arguments @('-Quick', '-OutputPath', $ChangedOutput)
    $ChangedCombined = $ChangedResult.StandardOutput + "`n" + $ChangedResult.StandardError
    Test-Condition ($ChangedResult.ExitCode -ne 0) 'TOCTOU reparse change exits nonzero'
    Test-Condition ($ChangedCombined -match 'Non-authoritative benchmark runs must not') 'TOCTOU reparse change emits policy error'
    Test-Condition (Test-Path -LiteralPath $GitMarker -PathType Leaf) 'TOCTOU test changes target after initial validation'
    Test-Condition (Test-Path -LiteralPath $GoMarker -PathType Leaf) 'TOCTOU test reaches completed preparation before second check'
    Test-Condition (-not (Test-Path -LiteralPath $ChangedOutput -PathType Leaf)) 'TOCTOU second check prevents output'
}
catch {
    $Failures.Add("unexpected test error: $($_.Exception.Message)")
}
finally {
    $env:PATH = $OriginalPath
    for ($Index = $Junctions.Count - 1; $Index -ge 0; $Index--) {
        $LinkPath = $Junctions[$Index]
        if ($null -ne (Get-Item -LiteralPath $LinkPath -Force -ErrorAction SilentlyContinue)) {
            Remove-Item -LiteralPath $LinkPath -Force
        }
    }
    if ($null -ne $TemporaryDirectory -and (Test-Path -LiteralPath $TemporaryDirectory -PathType Container)) {
        Remove-FlashGateScratchDirectory -Path $TemporaryDirectory -WorkingPath $WorkRoot -Prefix 'record-policy-test'
    }
}

Test-Condition ($null -eq $TemporaryDirectory -or -not (Test-Path -LiteralPath $TemporaryDirectory)) 'temporary test root removed'

[pscustomobject]@{
    Status       = $(if ($Failures.Count -eq 0) { 'PASS' } else { 'FAIL' })
    CheckCount   = $CheckCount
    FailureCount = $Failures.Count
    NoteCount    = $Notes.Count
    Notes        = ($Notes -join '; ')
    Failures     = ($Failures -join '; ')
} | Format-List

if ($Failures.Count -gt 0) { exit 1 }
