[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [ValidateSet('windows', 'linux')]
    [string]$PlatformName,

    [Parameter()]
    [ValidateRange(0.0, 100.0)]
    [double]$MinimumCoverage = 0.0,

    [Parameter()]
    [ValidateNotNullOrEmpty()]
    [string]$OutputRoot = 'build/coverage'
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $false

$status = 'FAIL'
$warningCount = 0
$failureCount = 1
$exitCode = 1
$totalCoverage = $null
$coveredStatements = $null
$totalStatements = $null
$errorMessage = $null
$coverageProfile = $null
$textReport = $null
$htmlReport = $null
$testLog = $null
$summaryJson = $null
$packageInventory = $null
$productPackageCount = 0
$productRoot = './cmd/server'
$coverageScope = 'server-module-dependency-closure'

function Resolve-OutputPath {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string]$Path
    )

    if ([IO.Path]::IsPathFullyQualified($Path)) {
        return [IO.Path]::GetFullPath($Path)
    }

    return [IO.Path]::GetFullPath(
        (Join-Path -Path (Get-Location) -ChildPath $Path)
    )
}

function Get-OutputTail {
    [CmdletBinding()]
    param(
        [Parameter()]
        [object[]]$Output,

        [Parameter()]
        [ValidateRange(1, 100)]
        [int]$LineCount = 25
    )

    $lines = @($Output | ForEach-Object { $_.ToString() })
    if ($lines.Count -eq 0) {
        return '<no output>'
    }

    return (($lines | Select-Object -Last $LineCount) -join [Environment]::NewLine)
}

try {
    $platformOutputRoot = Resolve-OutputPath -Path (
        Join-Path -Path $OutputRoot -ChildPath $PlatformName
    )

    [IO.Directory]::CreateDirectory($platformOutputRoot) | Out-Null

    $coverageProfile = Join-Path $platformOutputRoot 'coverage.out'
    $textReport = Join-Path $platformOutputRoot 'coverage.txt'
    $htmlReport = Join-Path $platformOutputRoot 'coverage.html'
    $testLog = Join-Path $platformOutputRoot 'test.log'
    $summaryJson = Join-Path $platformOutputRoot 'summary.json'
    $packageInventory = Join-Path $platformOutputRoot 'product-packages.txt'

    foreach ($file in @(
        $coverageProfile,
        $textReport,
        $htmlReport,
        $testLog,
        $summaryJson,
        $packageInventory
    )) {
        if ([IO.File]::Exists($file)) {
            [IO.File]::Delete($file)
        }
    }

    $moduleOutput = @(& go list -m -f '{{.Path}}' 2>&1)
    $moduleExitCode = $LASTEXITCODE
    if ($moduleExitCode -ne 0 -or $moduleOutput.Count -ne 1) {
        $tail = Get-OutputTail -Output $moduleOutput
        throw "go list -m could not identify the current module unambiguously (exit code $moduleExitCode).$([Environment]::NewLine)$tail"
    }

    $modulePath = $moduleOutput[0].ToString().Trim()
    if ([string]::IsNullOrWhiteSpace($modulePath)) {
        throw 'go list -m lieferte einen leeren Modulpfad.'
    }

    $dependencyOutput = @(& go list -deps -f '{{if .Module}}{{.ImportPath}}|{{.Module.Path}}{{end}}' $productRoot 2>&1)
    $dependencyExitCode = $LASTEXITCODE
    if ($dependencyExitCode -ne 0) {
        $tail = Get-OutputTail -Output $dependencyOutput
        throw "go list -deps failed with exit code $dependencyExitCode.$([Environment]::NewLine)$tail"
    }

    $productPackages = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
    foreach ($entry in $dependencyOutput) {
        $line = $entry.ToString().Trim()
        if (-not $line) {
            continue
        }

        $parts = $line.Split('|')
        if ($parts.Count -ne 2 -or -not $parts[0] -or -not $parts[1]) {
            throw "Unerwarteter go-list-Dependency-Eintrag: $line"
        }

        if ($parts[1] -eq $modulePath) {
            if ($parts[0] -ne $modulePath -and
                -not $parts[0].StartsWith("$modulePath/", [StringComparison]::Ordinal)) {
                throw "Fremdes Package im Produktmodul: $line"
            }
            [void]$productPackages.Add($parts[0])
        }
    }

    $serverPackage = "$modulePath/cmd/server"
    if ($productPackages.Count -eq 0 -or -not $productPackages.Contains($serverPackage)) {
        throw 'The product graph is empty or does not contain ./cmd/server.'
    }

    [string[]]$sortedPackages = [string[]]::new($productPackages.Count)
    $productPackages.CopyTo($sortedPackages)
    [Array]::Sort($sortedPackages, [StringComparer]::Ordinal)
    $productPackageCount = $sortedPackages.Count
    [IO.File]::WriteAllLines($packageInventory, $sortedPackages, [Text.UTF8Encoding]::new($false))

    $coveragePackages = $sortedPackages -join ','
    $testOutput = @(
        & go test `
            -covermode=atomic `
            "-coverpkg=$coveragePackages" `
            "-coverprofile=$coverageProfile" `
            ./... 2>&1
    )
    $testExitCode = $LASTEXITCODE

    [IO.File]::WriteAllText(
        $testLog,
        (($testOutput | ForEach-Object { $_.ToString() }) -join [Environment]::NewLine) +
            [Environment]::NewLine,
        [Text.UTF8Encoding]::new($false)
    )

    if ($testExitCode -ne 0) {
        $tail = Get-OutputTail -Output $testOutput
        throw "go test failed with exit code $testExitCode.$([Environment]::NewLine)$tail"
    }

    $coverageOutput = @(
        & go tool cover "-func=$coverageProfile" 2>&1
    )
    $coverageExitCode = $LASTEXITCODE

    if ($coverageExitCode -ne 0) {
        $tail = Get-OutputTail -Output $coverageOutput
        throw "go tool cover -func failed with exit code $coverageExitCode.$([Environment]::NewLine)$tail"
    }

    $coverageText = (
        $coverageOutput |
            ForEach-Object { $_.ToString() }
    ) -join [Environment]::NewLine

    [IO.File]::WriteAllText(
        $textReport,
        $coverageText + [Environment]::NewLine,
        [Text.UTF8Encoding]::new($false)
    )

    $totalLine = $coverageOutput |
        ForEach-Object { $_.ToString() } |
        Where-Object {
            $_ -match '^total:\s+\(statements\)\s+\d+(?:\.\d+)?%$'
        } |
        Select-Object -Last 1

    if (-not $totalLine) {
        throw 'Total coverage could not be determined from go tool cover.'
    }

    $coverageMatch = [regex]::Match(
        $totalLine,
        '^total:\s+\(statements\)\s+(?<coverage>\d+(?:\.\d+)?)%$'
    )

    if (-not $coverageMatch.Success) {
        throw "Total coverage could not be parsed: $totalLine"
    }

    $reportedCoverage = [double]::Parse(
        $coverageMatch.Groups['coverage'].Value,
        [Globalization.CultureInfo]::InvariantCulture
    )

    # A repository-wide test run repeats instrumented product blocks in the
    # profile for each test package. Count each source block once, as cover does,
    # and use the unrounded ratio for the hard minimum.
    $coverageBlocks = [Collections.Generic.Dictionary[string, long[]]]::new([StringComparer]::Ordinal)
    foreach ($line in [IO.File]::ReadLines($coverageProfile)) {
        if ($line -eq 'mode: atomic') {
            continue
        }

        $blockMatch = [regex]::Match(
            $line,
            '^(?<file>.+/[^/]+\.go):(?<range>\d+\.\d+,\d+\.\d+)\s+(?<statements>\d+)\s+(?<count>\d+)$'
        )
        if (-not $blockMatch.Success) {
            throw "Unerwarteter Coverage-Profil-Eintrag: $line"
        }

        $file = $blockMatch.Groups['file'].Value
        $package = $file.Substring(0, $file.LastIndexOf('/'))
        if (-not $productPackages.Contains($package)) {
            throw "Fremdes Package im Coverage-Profil: $package"
        }

        $key = "$file`:$($blockMatch.Groups['range'].Value)"
        $statements = [long]::Parse($blockMatch.Groups['statements'].Value)
        $count = [long]::Parse($blockMatch.Groups['count'].Value)
        if ($coverageBlocks.ContainsKey($key)) {
            if ($coverageBlocks[$key][0] -ne $statements) {
                throw "Inconsistent statement count in the coverage profile: $key"
            }
            if ($count -gt 0) {
                $coverageBlocks[$key][1] = 1
            }
        }
        else {
            $coverageBlocks.Add($key, [long[]]@($statements, [long]($count -gt 0)))
        }
    }

    [long]$totalStatements = 0
    [long]$coveredStatements = 0
    foreach ($block in $coverageBlocks.Values) {
        $totalStatements += $block[0]
        if ($block[1] -gt 0) {
            $coveredStatements += $block[0]
        }
    }
    if ($totalStatements -eq 0) {
        throw 'The coverage profile contains no product statements.'
    }

    $totalCoverage = 100.0 * $coveredStatements / $totalStatements
    if ([Math]::Abs($totalCoverage - $reportedCoverage) -gt 0.051) {
        throw "Coverage profile and go tool cover disagree: $totalCoverage / $reportedCoverage"
    }

    $htmlOutput = @(
        & go tool cover "-html=$coverageProfile" "-o=$htmlReport" 2>&1
    )
    $htmlExitCode = $LASTEXITCODE

    if ($htmlExitCode -ne 0) {
        $tail = Get-OutputTail -Output $htmlOutput
        throw "go tool cover -html failed with exit code $htmlExitCode.$([Environment]::NewLine)$tail"
    }

    if ($env:GITHUB_STEP_SUMMARY) {
        $summaryMarkdown = @"
## Go Code Coverage — $PlatformName

| Metric | Value |
|---|---:|
| Gesamt-Coverage | $($totalCoverage.ToString('0.0', [Globalization.CultureInfo]::InvariantCulture)) % |
| Minimum | $($MinimumCoverage.ToString('0.0', [Globalization.CultureInfo]::InvariantCulture)) % |
"@

        [IO.File]::AppendAllText(
            $env:GITHUB_STEP_SUMMARY,
            $summaryMarkdown + [Environment]::NewLine,
            [Text.UTF8Encoding]::new($false)
        )
    }

    if ($totalCoverage -lt $MinimumCoverage) {
        throw (
            'Coverage {0:0.0} % is below the minimum {1:0.0} %.' -f
            $totalCoverage,
            $MinimumCoverage
        )
    }

    $status = 'PASS'
    $failureCount = 0
    $exitCode = 0
}
catch {
    $errorMessage = $_.Exception.Message
}
finally {
    if ($summaryJson) {
        $summary = [ordered]@{
            status = $status
            platform = $PlatformName
            coverage_scope = $coverageScope
            product_root = $productRoot
            product_package_count = $productPackageCount
            product_package_inventory = $packageInventory
            total_coverage_percent = $totalCoverage
            covered_statements = $coveredStatements
            total_statements = $totalStatements
            minimum_coverage_percent = $MinimumCoverage
            coverage_profile = $coverageProfile
            text_report = $textReport
            html_report = $htmlReport
            test_log = $testLog
            error = $errorMessage
        }

        try {
            [IO.File]::WriteAllText(
                $summaryJson,
                ($summary | ConvertTo-Json -Depth 4) + [Environment]::NewLine,
                [Text.UTF8Encoding]::new($false)
            )
        }
        catch {
            $summaryWriteError = $_.Exception.Message
            $status = 'FAIL'
            $failureCount = 1
            $exitCode = 1

            if ($errorMessage) {
                $errorMessage += " Summary could not be written: $summaryWriteError"
            }
            else {
                $errorMessage = "Summary could not be written: $summaryWriteError"
            }
        }
    }

    [pscustomobject]@{
        Status          = $status
        Platform        = $PlatformName
        TotalCoverage   = if ($null -eq $totalCoverage) {
            $null
        }
        else {
            '{0:0.0} %' -f $totalCoverage
        }
        MinimumCoverage = '{0:0.0} %' -f $MinimumCoverage
        ReportPath      = Join-Path $OutputRoot $PlatformName
        WarningCount    = $warningCount
        FailureCount    = $failureCount
        NextAction      = if ($status -eq 'PASS') {
            'Document the baseline and define the platform-specific minimum.'
        }
        else {
            'Correct the errors and repeat the coverage run.'
        }
        Error           = $errorMessage
    } | Format-List
}

exit $exitCode
