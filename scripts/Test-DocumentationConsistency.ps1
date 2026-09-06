#Requires -Version 7.6

[CmdletBinding()]
param(
    [Parameter()]
    [string]$RepositoryRoot = (Split-Path -Parent $PSScriptRoot)
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$resolvedRoot = (Resolve-Path -LiteralPath $RepositoryRoot -ErrorAction Stop).Path
if (-not (Test-Path -LiteralPath $resolvedRoot -PathType Container)) {
    throw 'RepositoryRoot must identify an existing directory.'
}

$checks = [System.Collections.Generic.List[object]]::new()

function Add-Check {
    param(
        [Parameter(Mandatory)]
        [string]$Id,

        [Parameter(Mandatory)]
        [bool]$Passed,

        [Parameter(Mandatory)]
        [string]$Message
    )

    $checks.Add([pscustomobject]@{
            id      = $Id
            passed  = $Passed
            message = $Message
        })
}

function Read-StrictUtf8 {
    param(
        [Parameter(Mandatory)]
        [string]$RelativePath
    )

    $fullPath = Join-Path $resolvedRoot $RelativePath
    if (-not (Test-Path -LiteralPath $fullPath -PathType Leaf)) {
        Add-Check -Id ('FILE-{0}' -f $RelativePath) -Passed $false -Message 'Required file is missing.'
        return $null
    }

    try {
        $bytes = [System.IO.File]::ReadAllBytes($fullPath)
        $encoding = [System.Text.UTF8Encoding]::new($false, $true)
        $text = $encoding.GetString($bytes)
        Add-Check -Id ('FILE-{0}' -f $RelativePath) -Passed (-not $text.Contains([char]0)) -Message 'Strict UTF-8 and NUL-free.'
        return $text
    }
    catch {
        Add-Check -Id ('FILE-{0}' -f $RelativePath) -Passed $false -Message $_.Exception.Message
        return $null
    }
}

function Get-BacklogHighestIdentifier {
    param(
        [Parameter(Mandatory)]
        [string]$BacklogText
    )

    $diagnostics = [System.Collections.Generic.List[string]]::new()
    $workItemMatches = [regex]::Matches(
        $BacklogText,
        '(?m)^\| BL-(?<Number>[0-9]{3}) \| (?<Status>[^|\r\n]+) \| (?<Task>[^|\r\n]+) \|'
    )
    $workItemIdentifiers = [string[]]@(
        $workItemMatches | ForEach-Object {
            'BL-{0:D3}' -f [int]$_.Groups['Number'].Value
        }
    )
    $duplicateIdentifiers = [string[]]@(
        $workItemIdentifiers |
            Group-Object |
            Where-Object { $_.Count -gt 1 } |
            ForEach-Object { $_.Name }
    )

    $computedIdentifier = $null
    if ($workItemIdentifiers.Count -eq 0) {
        $diagnostics.Add('No canonical backlog work-item rows were found.')
    }
    else {
        $highestNumber = [int](
            $workItemMatches |
                ForEach-Object { [int]$_.Groups['Number'].Value } |
                Measure-Object -Maximum
        ).Maximum
        $computedIdentifier = 'BL-{0:D3}' -f $highestNumber
    }

    if ($duplicateIdentifiers.Count -ne 0) {
        $diagnostics.Add('Duplicate canonical work-item identifiers: {0}.' -f ($duplicateIdentifiers -join ', '))
    }

    $declarationMatches = [regex]::Matches(
        $BacklogText,
        '(?im)^The highest assigned backlog identifier is (?<Token>[^.\r\n]+)\.'
    )
    $declaredIdentifier = $null
    if ($declarationMatches.Count -eq 0) {
        $diagnostics.Add('The highest assigned backlog identifier declaration is missing.')
    }
    elseif ($declarationMatches.Count -gt 1) {
        $diagnostics.Add('Multiple highest assigned backlog identifier declarations were found.')
    }
    else {
        $declaredToken = $declarationMatches[0].Groups['Token'].Value.Trim()
        $declaredMatch = [regex]::Match($declaredToken, '^`(?<Identifier>BL-[0-9]{3})`$')
        if (-not $declaredMatch.Success) {
            $diagnostics.Add('The declared highest assigned backlog identifier has an invalid format.')
        }
        else {
            $declaredIdentifier = $declaredMatch.Groups['Identifier'].Value
        }
    }

    if ($null -ne $computedIdentifier -and
        $null -ne $declaredIdentifier -and
        $computedIdentifier -ne $declaredIdentifier) {
        $diagnostics.Add((
                'Declared identifier {0} does not match computed identifier {1}.' -f
                $declaredIdentifier,
                $computedIdentifier
            ))
    }

    [pscustomobject]@{
        Passed                                = $diagnostics.Count -eq 0
        ComputedHighestAssignedBacklogIdentifier = $computedIdentifier
        DeclaredHighestAssignedBacklogIdentifier = $declaredIdentifier
        Diagnostics                           = [string[]]$diagnostics
    }
}

$requiredFiles = [string[]]@(
    'BACKLOG.md',
    'CHANGELOG.md',
    'CONTRIBUTING.md',
    'README.md',
    'Governance/CHANGE-TRIGGER-REVIEW-AND-BACKLOG-STANDARD.md',
    'docs/documentation-quality-gate.md',
    'docs/testing.md',
    'benchmarks/README.md',
    '.github/workflows/ci.yml',
    '.github/workflows/metadata-regression.yml',
    '.github/workflows/release-build.yml'
)

$documents = @{}
foreach ($relativePath in $requiredFiles) {
    $documents[$relativePath] = Read-StrictUtf8 -RelativePath $relativePath
}

$backlog = [string]$documents['BACKLOG.md']
$adapter = [string]$documents['Governance/CHANGE-TRIGGER-REVIEW-AND-BACKLOG-STANDARD.md']
$ci = [string]$documents['.github/workflows/ci.yml']
$release = [string]$documents['.github/workflows/release-build.yml']
$combinedGuidance = [string]::Join("`n", [string[]]@(
        $documents['README.md'],
        $documents['CONTRIBUTING.md'],
        $documents['docs/documentation-quality-gate.md'],
        $documents['docs/testing.md'],
        $documents['benchmarks/README.md']
    ))

Add-Check -Id 'BACKLOG-BL343' -Passed ($backlog -match '(?m)^\| BL-343 \| Done \|') -Message 'BL-343 is registered once with terminal Done status.'
Add-Check -Id 'BACKLOG-BL337' -Passed ($backlog -match '(?m)^\| BL-337 \| Done \|' -and $backlog.Contains('Superseded by BL-343')) -Message 'BL-337 is terminally superseded without removing its ID.'
Add-Check -Id 'BACKLOG-BL330' -Passed ($backlog -match '(?m)^\| BL-330 \| Planned \|') -Message 'BL-330 retains the small project-specific status contract scope.'
$backlogHighest = Get-BacklogHighestIdentifier -BacklogText $backlog
Add-Check -Id 'BACKLOG-HIGHEST' -Passed $backlogHighest.Passed -Message (
    'Computed={0}; Declared={1}; Diagnostics={2}' -f
    $backlogHighest.ComputedHighestAssignedBacklogIdentifier,
    $backlogHighest.DeclaredHighestAssignedBacklogIdentifier,
    ([string[]]$backlogHighest.Diagnostics -join ' ')
)
Add-Check -Id 'BACKLOG-BL342-HISTORY' -Passed ($backlog -match '(?m)^\| BL-342 \| Done \|') -Message 'BL-342 remains historical Done work.'
Add-Check -Id 'INF168-DISPOSITION' -Passed ($backlog.Contains('four current documentation semantics are `ALREADY_PRESENT_EQUIVALENT`') -and $backlog.Contains('six Heavy artifacts are `SUPERSEDED_BY_SLIM_GOVERNANCE`')) -Message 'All ten INF168-REV-007 paths have a complete 4/6 disposition.'

Add-Check -Id 'ADAPTER-SLIM' -Passed ($adapter.Contains('SLIM_PROJECT_ADAPTER') -and $adapter.Contains('DIRECTLY_AFFECTED_FIRST')) -Message 'The project adapter inherits the Slim Governance model.'
Add-Check -Id 'ADAPTER-REGISTRATION-TRUTH' -Passed ($adapter.Contains('NEW_WORK_REGISTERED') -and $adapter.Contains('canonical `BACKLOG.md` write')) -Message 'Registration truth requires canonical write and readback.'
Add-Check -Id 'ADAPTER-EXTERNAL-BOUNDARY' -Passed ($adapter.Contains('Push, PR or merge writes') -and $adapter.Contains('separately authorized')) -Message 'External and credential boundaries remain fail closed.'
Add-Check -Id 'ADAPTER-NO-HEAVY-SCHEMA' -Passed (-not ($adapter -match 'assignment-governance-record\.schema|completion-report\.schema|change-trigger-catalog\.json')) -Message 'The thin adapter does not copy Heavy machine contracts.'

Add-Check -Id 'CI-HEAVY-DISABLED' -Passed ($ci.Contains('Legacy governance orchestration (disabled by Slim Governance)') -and $ci.Contains('if: ${{ false }}')) -Message 'Legacy governance orchestration cannot block normal Product CI.'
Add-Check -Id 'CI-PRODUCT-GATES' -Passed ($ci.Contains('go vet ./...') -and $ci.Contains('Test-GoCoverage.ps1') -and $ci.Contains('golangci-lint run') -and $ci.Contains('go build')) -Message 'Go vet, coverage, lint and build remain active.'
Add-Check -Id 'CI-SCRIPT-GATES' -Passed ($ci.Contains('Test-DocumentationConsistency.ps1') -and $ci.Contains('Test-ShellScripts.Tests.ps1')) -Message 'Focused documentation and shell gates remain active.'
Add-Check -Id 'RELEASE-NO-HEAVY' -Passed (-not ($release -match 'New-GovernanceWorkflowRecord|Test-GovernanceConsistency')) -Message 'Release preparation no longer consumes Heavy governance orchestration.'
Add-Check -Id 'RELEASE-TECHNICAL' -Passed ($release.Contains('Resolve and validate release values') -and $release.Contains('Test-ReleaseArtifact.ps1')) -Message 'Technical release validation remains active.'

$staleBenchmarkPath = 'C:\Voxtronic\Codex\Temp\Benchmarks'
Add-Check -Id 'PORTABILITY-BENCHMARK-PATH' -Passed (-not $combinedGuidance.Contains($staleBenchmarkPath)) -Message 'Active guidance contains no stale personal benchmark root.'
Add-Check -Id 'PORTABILITY-CODEX-ROOT' -Passed (-not ($combinedGuidance -match 'C:\\Users\\[^\\]+\\.*Codex-Work')) -Message 'Active repository guidance contains no contributor-local Codex-Work path.'
Add-Check -Id 'LEGACY-EXPLICIT' -Passed ($combinedGuidance.Contains('LEGACY_COMPATIBILITY_ONLY')) -Message 'Historical Heavy material is explicitly nonblocking compatibility content.'

$failedChecks = [object[]]@($checks | Where-Object { -not $_.passed })
$result = [pscustomobject]@{
    schemaVersion = 1
    status        = if ($failedChecks.Count -eq 0) { 'PASS' } else { 'FAIL' }
    checkCount    = $checks.Count
    passedCount   = $checks.Count - $failedChecks.Count
    failureCount  = $failedChecks.Count
    computedHighestAssignedBacklogIdentifier = $backlogHighest.ComputedHighestAssignedBacklogIdentifier
    declaredHighestAssignedBacklogIdentifier = $backlogHighest.DeclaredHighestAssignedBacklogIdentifier
    checks        = [object[]]$checks
}

$result | ConvertTo-Json -Depth 5
if ($failedChecks.Count -ne 0) {
    exit 1
}
