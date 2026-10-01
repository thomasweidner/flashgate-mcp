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
$documents = [System.Collections.Generic.Dictionary[string, string]]::new(
    [System.StringComparer]::Ordinal
)

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
    $identifiers = [string[]]@(
        $workItemMatches | ForEach-Object {
            'BL-{0:D3}' -f [int]$_.Groups['Number'].Value
        }
    )
    $duplicates = [string[]]@(
        $identifiers |
            Group-Object |
            Where-Object { $_.Count -gt 1 } |
            ForEach-Object { $_.Name }
    )

    $computedIdentifier = $null
    if ($identifiers.Count -eq 0) {
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

    if ($duplicates.Count -ne 0) {
        $duplicateJson = ConvertTo-Json -InputObject ([string[]]$duplicates) -Compress
        $diagnostics.Add('Duplicate canonical work-item identifiers: {0}.' -f $duplicateJson)
    }

    $declarationMatches = [regex]::Matches(
        $BacklogText,
        '(?im)^The highest assigned backlog identifier is (?<Token>[^.\r\n]+)\.'
    )
    $declaredIdentifier = $null
    if ($declarationMatches.Count -eq 1) {
        $declaredMatch = [regex]::Match(
            $declarationMatches[0].Groups['Token'].Value.Trim(),
            '^`(?<Identifier>BL-[0-9]{3})`$'
        )
        if ($declaredMatch.Success) {
            $declaredIdentifier = $declaredMatch.Groups['Identifier'].Value
        }
        else {
            $diagnostics.Add('The declared highest assigned backlog identifier has an invalid format.')
        }
    }
    elseif ($declarationMatches.Count -eq 0) {
        $diagnostics.Add('The highest assigned backlog identifier declaration is missing.')
    }
    else {
        $diagnostics.Add('Multiple highest assigned backlog identifier declarations were found.')
    }

    if ($null -ne $computedIdentifier -and
        $null -ne $declaredIdentifier -and
        $computedIdentifier -cne $declaredIdentifier) {
        $diagnostics.Add((
                'Declared identifier {0} does not match computed identifier {1}.' -f
                $declaredIdentifier,
                $computedIdentifier
            ))
    }

    [pscustomobject]@{
        Passed                                   = $diagnostics.Count -eq 0
        ComputedHighestAssignedBacklogIdentifier = $computedIdentifier
        DeclaredHighestAssignedBacklogIdentifier = $declaredIdentifier
        Diagnostics                              = [string[]]$diagnostics
    }
}

function Get-InternalReferenceFindings {
    param(
        [Parameter(Mandatory)]
        [System.Collections.Generic.Dictionary[string, string]]$TextByPath,

        [Parameter(Mandatory)]
        [object[]]$Patterns
    )

    $findings = [System.Collections.Generic.List[object]]::new()
    foreach ($path in [string[]]@($TextByPath.Keys | Sort-Object -CaseSensitive)) {
        $text = $TextByPath[$path]
        foreach ($pattern in $Patterns) {
            $matches = [regex]::Matches($text, [string]$pattern.expression)
            if ($matches.Count -ne 0) {
                $findings.Add([pscustomobject]@{
                        path       = $path
                        patternId  = [string]$pattern.id
                        matchCount = $matches.Count
                    })
            }
        }
    }

    return [object[]]$findings
}

function Test-HumanStatusModel {
    param(
        [Parameter(Mandatory)] [string]$BacklogText,
        [Parameter(Mandatory)] [AllowEmptyCollection()] [string[]]$ExpectedPostIds
    )

    $allowed = [string[]]@('Planned', 'In Progress', 'Blocked', 'Completed', 'Rejected', 'Superseded')
    $findings = [System.Collections.Generic.List[string]]::new()
    $backlogRows = [regex]::Matches($BacklogText, '(?m)^\| (?<Id>BL-[0-9]{3}) \| (?<Status>[^|\r\n]+) \|')
    $sprintRows = [regex]::Matches($BacklogText, '(?m)^\| (?<Id>SPR-[0-9]{3}) \| (?<Status>[^|\r\n]+) \|')
    $backlogById = [System.Collections.Generic.Dictionary[string, string]]::new([System.StringComparer]::Ordinal)
    foreach ($row in $backlogRows) {
        $id = $row.Groups['Id'].Value
        $status = $row.Groups['Status'].Value.Trim()
        if ($status -cnotin $allowed) { $findings.Add(('Invalid backlog status: {0}:{1}' -f $id, $status)) }
        if ($backlogById.ContainsKey($id)) { $findings.Add('Duplicate backlog ID: {0}' -f $id) }
        else { $backlogById.Add($id, $status) }
    }
    $sprintIds = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::Ordinal)
    foreach ($row in $sprintRows) {
        $id = $row.Groups['Id'].Value
        $status = $row.Groups['Status'].Value.Trim()
        if ($status -cnotin $allowed) { $findings.Add(('Invalid sprint status: {0}:{1}' -f $id, $status)) }
        if (-not $sprintIds.Add($id)) { $findings.Add('Duplicate sprint ID: {0}' -f $id) }
    }

    $postIds = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::Ordinal)
    $postTable = [regex]::Match($BacklogText, '(?s)\| Post-1\.0 workstream \| Backlog IDs \| Direction \|\r?\n\|---\|---\|---\|\r?\n(?<Rows>(?:\|[^\r\n]*\|\r?\n)*)')
    if (-not $postTable.Success) {
        $findings.Add('Post-1.0 workstream table is missing or malformed.')
    }
    else {
        $postRows = [regex]::Matches($postTable.Groups['Rows'].Value, '(?m)^\| [^|]+ \| (?<Ids>[^|]+) \| [^|]+ \|$')
        foreach ($row in $postRows) {
            foreach ($token in ($row.Groups['Ids'].Value -split ',')) {
                $trimmed = $token.Trim()
                $idMatch = [regex]::Match($trimmed, '^BL-(?<Start>[0-9]{3})(?:[–-]BL-(?<End>[0-9]{3}))?$')
                if (-not $idMatch.Success) {
                    $findings.Add('Malformed Post-1.0 reference: {0}' -f $trimmed)
                    continue
                }
                $start = [int]$idMatch.Groups['Start'].Value
                $end = if ($idMatch.Groups['End'].Success) { [int]$idMatch.Groups['End'].Value } else { $start }
                if ($end -lt $start) { $findings.Add('Descending Post-1.0 range: {0}' -f $trimmed); continue }
                for ($number = $start; $number -le $end; $number++) {
                    $id = 'BL-{0:D3}' -f $number
                    if (-not $postIds.Add($id)) { $findings.Add('Duplicate Post-1.0 reference: {0}' -f $id) }
                    if (-not $backlogById.ContainsKey($id)) { $findings.Add('Missing canonical Post-1.0 item: {0}' -f $id) }
                }
            }
        }
    }
    foreach ($id in $ExpectedPostIds) {
        if (-not $postIds.Contains($id)) { $findings.Add('Missing expected Post-1.0 placement: {0}' -f $id) }
    }
    foreach ($id in $postIds) {
        if ($id -cnotin $ExpectedPostIds) { $findings.Add('Unexpected Post-1.0 placement: {0}' -f $id) }
    }
    [pscustomobject]@{
        Passed = $findings.Count -eq 0
        Findings = [string[]]$findings
        BacklogCount = $backlogRows.Count
        SprintCount = $sprintRows.Count
        PostCount = $postIds.Count
        BacklogIds = [string[]]@($backlogById.Keys)
        SprintIds = [string[]]@($sprintIds)
    }
}

$requiredPublicPaths = [string[]]@(
    'AGENTS.md',
    'docs/README.md',
    'docs/adr/README.md',
    'docs/adr/product-metadata.md',
    'docs/documentation-style.md',
    'docs/migration.md',
    'BACKLOG.md',
    'CHANGELOG.md',
    'CONTRIBUTING.md',
    'README.md',
    'benchmarks/README.md',
    'docs/architecture.md',
    'docs/client-setup.md',
    'docs/development/code-coverage.md',
    'docs/documentation-quality-gate.md',
    'docs/planning/efficiency.md',
    'docs/planning/README.md',
    'docs/planning/tool-adapters.md',
    'docs/project-identity.md',
    'docs/protocol.md',
    'docs/roadmap.md',
    'docs/security.md',
    'docs/specification.md',
    'docs/testing.md',
    'docs/tool-conventions.md',
    'docs/tools.md',
    'docs/planning/release-scope.md'
)
$supportingPaths = [string[]]@(
    '.github/workflows/ci.yml',
    '.github/workflows/metadata-regression.yml',
    '.github/workflows/release-build.yml'
)
# The native, standard-library-only checker inventories every maintained
# Markdown file, including nonignored new files. The explicit required list
# prevents deletion of a canonical document from shrinking the acceptance set.
$documentationRules = $null
$documentationRulePaths = [string[]]@()
$priorNativePreference = $PSNativeCommandUseErrorActionPreference
$priorGoToolchain = $env:GOTOOLCHAIN
try {
    $PSNativeCommandUseErrorActionPreference = $false
    $env:GOTOOLCHAIN = 'local'
    $ruleOutput = @(& go -C $resolvedRoot run ./cmd/doccheck -root $resolvedRoot)
    $ruleExitCode = $LASTEXITCODE
    $documentationRules = ($ruleOutput -join [Environment]::NewLine) | ConvertFrom-Json -ErrorAction Stop
    if ($documentationRules.schemaVersion -ne 1 -or
        $documentationRules.documentCount -le 0 -or
        @($documentationRules.documentPaths).Count -ne $documentationRules.documentCount) {
        throw 'Documentation checker returned an invalid inventory contract.'
    }
    $documentationRulePaths = [string[]]@($documentationRules.documentPaths)
    Add-Check -Id 'DOCUMENTATION-RULES' -Passed (
        $ruleExitCode -eq 0 -and $documentationRules.status -ceq 'PASS' -and
        @($documentationRules.findings).Count -eq 0
    ) -Message (ConvertTo-Json -InputObject $documentationRules -Depth 6 -Compress)
}
catch {
    Add-Check -Id 'DOCUMENTATION-RULES' -Passed $false -Message $_.Exception.Message
}
finally {
    $PSNativeCommandUseErrorActionPreference = $priorNativePreference
    $env:GOTOOLCHAIN = $priorGoToolchain
}
$publicAuthorityPaths = [string[]]@(
    @($requiredPublicPaths + $documentationRulePaths) | Sort-Object -Unique -CaseSensitive
)
$requiredFiles = [string[]]@($publicAuthorityPaths + $supportingPaths)

foreach ($relativePath in $requiredFiles) {
    $text = Read-StrictUtf8 -RelativePath $relativePath
    if ($null -ne $text) {
        $documents.Add($relativePath, [string]$text)
    }
}

$backlog = [string]$documents['BACKLOG.md']
$ci = [string]$documents['.github/workflows/ci.yml']
$release = [string]$documents['.github/workflows/release-build.yml']

Add-Check -Id 'BACKLOG-BL343' -Passed ($backlog -match '(?m)^\| BL-343 \| Completed \|') -Message 'BL-343 remains historical Completed work.'
Add-Check -Id 'BACKLOG-BL344' -Passed ($backlog -match '(?m)^\| BL-344 \| Completed \|') -Message 'BL-344 remains historical Completed work.'
$bl344ContradictoryStatusCount = [regex]::Matches(
    $backlog,
    '(?i)\bBL-344\s+is\s+`?In Progress`?'
).Count
$bl344CompletedSummaryCount = [regex]::Matches(
    $backlog,
    '(?i)\bBL-344\s+(?:is|are)\s+`?Completed`?'
).Count
Add-Check -Id 'BACKLOG-BL344-STATUS-PARITY' -Passed (
    $bl344ContradictoryStatusCount -eq 0 -and
    $bl344CompletedSummaryCount -gt 0
) -Message (
    'CompletedSummaryCount={0}; ContradictoryStatusCount={1}.' -f
    $bl344CompletedSummaryCount,
    $bl344ContradictoryStatusCount
)
$backlogHighest = Get-BacklogHighestIdentifier -BacklogText $backlog
$backlogDiagnosticsJson = ConvertTo-Json -InputObject ([string[]]$backlogHighest.Diagnostics) -Compress
Add-Check -Id 'BACKLOG-HIGHEST' -Passed $backlogHighest.Passed -Message (
    'Computed={0}; Declared={1}; Diagnostics={2}' -f
    $backlogHighest.ComputedHighestAssignedBacklogIdentifier,
    $backlogHighest.DeclaredHighestAssignedBacklogIdentifier,
    $backlogDiagnosticsJson
)

$expectedPostIds = [string[]]@(
    'BL-081', 'BL-083', 'BL-112', 'BL-127', 'BL-128', 'BL-150', 'BL-158',
    'BL-169', 'BL-176', 'BL-180', 'BL-181', 'BL-182', 'BL-183', 'BL-184',
    'BL-185', 'BL-186', 'BL-187', 'BL-188', 'BL-217', 'BL-232', 'BL-240',
    'BL-313', 'BL-345', 'BL-347', 'BL-348', 'BL-349', 'BL-350', 'BL-351',
    'BL-354', 'BL-355', 'BL-356', 'BL-357', 'BL-358', 'BL-359', 'BL-360',
    'BL-361', 'BL-362'
)
$statusModel = Test-HumanStatusModel -BacklogText $backlog -ExpectedPostIds $expectedPostIds
Add-Check -Id 'BACKLOG-HUMAN-STATUS-AND-POST-PLACEMENT' -Passed $statusModel.Passed -Message (
    'BL={0}; Sprints={1}; Post={2}; Findings={3}' -f
    $statusModel.BacklogCount, $statusModel.SprintCount, $statusModel.PostCount,
    (ConvertTo-Json -InputObject ([string[]]$statusModel.Findings) -Compress)
)
$missingStableIds = [string[]]@(
    @(1..362 | ForEach-Object { 'BL-{0:D3}' -f $_ } | Where-Object { $_ -cnotin $statusModel.BacklogIds }) +
    @(41..61 | ForEach-Object { 'SPR-{0:D3}' -f $_ } | Where-Object { $_ -cnotin $statusModel.SprintIds })
)
Add-Check -Id 'BACKLOG-STABLE-IDS' -Passed ($missingStableIds.Count -eq 0) -Message (
    'Missing={0}.' -f (ConvertTo-Json -InputObject $missingStableIds -Compress)
)

$fixtureHeader = [string[]]@('| Post-1.0 workstream | Backlog IDs | Direction |', '|---|---|---|')
foreach ($status in [string[]]@('Planned', 'In Progress', 'Blocked', 'Completed', 'Rejected', 'Superseded')) {
    $fixture = ([string[]]@(('| BL-001 | {0} | Fixture | Scope |' -f $status), '| SPR-001 | Planned | BL-001 | Fixture |') + $fixtureHeader) -join "`n"
    $actual = Test-HumanStatusModel -BacklogText ($fixture + "`n") -ExpectedPostIds ([string[]]@())
    Add-Check -Id ('STATUS-FIXTURE-ACCEPT-{0}' -f ($status -replace ' ', '-')) -Passed $actual.Passed -Message (ConvertTo-Json -InputObject ([string[]]$actual.Findings) -Compress)
}
foreach ($status in [string[]]@('Ready', 'Done', 'Later', 'Unknown', 'Planned (Post-1.0)')) {
    foreach ($kind in [string[]]@('BL', 'SPR')) {
        $blStatus = if ($kind -eq 'BL') { $status } else { 'Planned' }
        $sprintStatus = if ($kind -eq 'SPR') { $status } else { 'Planned' }
        $fixture = ([string[]]@(('| BL-001 | {0} | Fixture | Scope |' -f $blStatus), ('| SPR-001 | {0} | BL-001 | Fixture |' -f $sprintStatus)) + $fixtureHeader) -join "`n"
        $actual = Test-HumanStatusModel -BacklogText ($fixture + "`n") -ExpectedPostIds ([string[]]@())
        $diagnosticPrefix = if ($kind -eq 'BL') { 'Invalid backlog status: BL-001:' } else { 'Invalid sprint status: SPR-001:' }
        $specificFinding = [string[]]@($actual.Findings | Where-Object { $_.StartsWith($diagnosticPrefix, [System.StringComparison]::Ordinal) })
        Add-Check -Id ('STATUS-FIXTURE-REJECT-{0}-{1}' -f $kind, ($status -replace '[^A-Za-z0-9]', '-')) -Passed (-not $actual.Passed -and $specificFinding.Count -eq 1) -Message (ConvertTo-Json -InputObject ([string[]]$actual.Findings) -Compress)
    }
}
$postFixture = ([string[]]@('| BL-001 | Planned | Fixture | Scope |', '| SPR-001 | Planned | BL-001 | Fixture |') + $fixtureHeader + '| Future | BL-001 | Fixture |') -join "`n"
$postResult = Test-HumanStatusModel -BacklogText ($postFixture + "`n") -ExpectedPostIds ([string[]]@('BL-001'))
Add-Check -Id 'STATUS-FIXTURE-ACCEPT-POST-PLANNED' -Passed $postResult.Passed -Message (ConvertTo-Json -InputObject ([string[]]$postResult.Findings) -Compress)
$driftResult = Test-HumanStatusModel -BacklogText ($postFixture + "`n") -ExpectedPostIds ([string[]]@('BL-002'))
Add-Check -Id 'STATUS-FIXTURE-REJECT-POST-DRIFT' -Passed (-not $driftResult.Passed -and 'Missing expected Post-1.0 placement: BL-002' -cin $driftResult.Findings) -Message (ConvertTo-Json -InputObject ([string[]]$driftResult.Findings) -Compress)
$missingReferenceFixture = $postFixture.Replace('| Future | BL-001 |', '| Future | BL-002 |')
$missingReferenceResult = Test-HumanStatusModel -BacklogText ($missingReferenceFixture + "`n") -ExpectedPostIds ([string[]]@('BL-002'))
Add-Check -Id 'STATUS-FIXTURE-REJECT-MISSING-BL' -Passed (-not $missingReferenceResult.Passed -and 'Missing canonical Post-1.0 item: BL-002' -cin $missingReferenceResult.Findings) -Message (ConvertTo-Json -InputObject ([string[]]$missingReferenceResult.Findings) -Compress)

$readme = [string]$documents['README.md']
$architecture = [string]$documents['docs/architecture.md']
$identity = [string]$documents['docs/project-identity.md']
$protocol = [string]$documents['docs/protocol.md']
$scope = [string]$documents['docs/planning/release-scope.md']
$security = [string]$documents['docs/security.md']
$matrixPath = Join-Path $resolvedRoot 'docs/mcp-protocol-matrix.json'
$advertised2025Only = $false
if (Test-Path -LiteralPath $matrixPath -PathType Leaf) {
    try {
        $utf8 = [System.Text.UTF8Encoding]::new($false, $true)
        $matrixText = $utf8.GetString([System.IO.File]::ReadAllBytes($matrixPath))
        $matrix = ConvertFrom-Json -InputObject $matrixText -AsHashtable -Depth 8 -ErrorAction Stop
        $revisions = @($matrix['revisions'])
        $advertised2025Only = (
            $matrix['schemaVersion'] -eq 'flashgate-mcp-protocol-matrix/v1' -and
            $revisions.Count -eq 1 -and
            $revisions[0]['protocolVersion'] -eq '2025-11-25' -and
            $revisions[0]['opening'] -eq 'initialize' -and
            @($revisions[0]['extensions']).Count -eq 0
        )
    }
    catch {
        $advertised2025Only = $false
    }
}
$publicMcpDocuments = [string[]]@(
    $readme, $architecture, $protocol, $scope, $security,
    [string]$documents['docs/adr/mcp-compatibility.md']
)
$obsoleteMcpClaims = [string[]]@(
    $publicMcpDocuments | Where-Object {
        $_.Contains('Current implementation remains only `2025-11-25`') -or
        $_.Contains('The implemented protocol remains MCP `2025-11-25`') -or
        $_.Contains('The implemented revision remains `2025-11-25`') -or
        $_.Contains('not a claim that `2026-07-28` is implemented today') -or
        $_.Contains('The planned `2026-07-28` path') -or
        $_.Contains('Those behaviors are target architecture only until BL-207/208')
    }
)
Add-Check -Id 'CURRENT-AND-V1-SCOPE-PARITY' -Passed (
    $readme.Contains('Today it provides secure, root-confined filesystem access') -and
    $readme.Contains('Version 1.0 extends that foundation') -and
    $architecture.Contains('Its current implementation provides root-confined filesystem access') -and
    $identity.Contains('Today it provides secure, root-confined filesystem access') -and
    $scope.Contains('stateless adapter is implemented as a compiled-in candidate') -and
    $scope.Contains('The Version 1.0 release matrix targets both exact')
) -Message 'Current filesystem implementation and the Version 1.0 candidate/activation distinction agree.'
Add-Check -Id 'MCP-REVISION-PARITY' -Passed (
    $advertised2025Only -and
    $obsoleteMcpClaims.Count -eq 0 -and
    $architecture.Contains('The production policy enables and advertises only `2025-11-25`') -and
    $architecture.Contains('`2026-07-28` stateless adapter is compiled in') -and
    $protocol.Contains('canonical list of currently advertised revisions') -and
    $protocol.Contains('adapter is compiled in and tested with an injected internal support policy') -and
    $protocol.Contains('remains disabled by the production policy and absent from the matrix until') -and
    $protocol.Contains('advertises MCP revision `2025-11-25`') -and
    $scope.Contains('Public activation remains dependent on') -and
    $security.Contains('It remains disabled and unadvertised until BL-204, BL-212, and') -and
    $architecture.Contains('BL-204, BL-212, and BL-219') -and
    $protocol.Contains('BL-204, BL-212, and BL-219') -and
    $scope.Contains('BL-204, BL-212, and BL-219')
) -Message ('Implemented candidate and sole advertised 2025 revision remain distinct; ObsoleteClaimDocumentCount={0}; MatrixParity={1}.' -f $obsoleteMcpClaims.Count, $advertised2025Only)

$storagePlan = [string]$documents['docs/planning/tool-adapters.md']
$storageNames = [string[]]@(
    'compression_supported', 'compression_enabled', 'compression_inherited_default',
    'encryption_supported', 'encryption_enabled', 'encryption_inherited_default'
)
$storageTermsAgree = $true
foreach ($name in $storageNames) {
    if (-not $backlog.Contains($name) -or -not $storagePlan.Contains($name)) {
        $storageTermsAgree = $false
    }
}
Add-Check -Id 'STORAGE-PLANNING-TERMINOLOGY' -Passed (
    $storageTermsAgree -and
    $backlog.Contains('An inherited default describes directory/child behavior, not the enabled state of the current path.') -and
    $storagePlan.Contains('An inherited default describes directory/child behavior, not the enabled state of the current path.')
) -Message 'Planned storage names and inherited-default semantics agree.'

Add-Check -Id 'CI-PRODUCT-GATES' -Passed (
    $ci.Contains('go vet ./...') -and
    $ci.Contains('Test-GoCoverage.ps1') -and
    $ci.Contains('golangci-lint run') -and
    $ci.Contains('go build')
) -Message 'Go vet, coverage, lint, and build remain active.'
Add-Check -Id 'CI-SCRIPT-GATES' -Passed (
    $ci.Contains('Test-DocumentationConsistency.ps1') -and
    $ci.Contains('Test-ShellScripts.Tests.ps1')
) -Message 'Documentation and shell gates remain active.'
Add-Check -Id 'CI-NO-INTERNAL-ORCHESTRATION' -Passed (
    -not ($ci -match 'Legacy governance orchestration|New-GovernanceWorkflowRecord|Test-GovernanceConsistency|Test-ClassicReviewArtifact')
) -Message 'Product CI has no private workflow-orchestration block.'
Add-Check -Id 'RELEASE-TECHNICAL' -Passed (
    $release.Contains('Resolve and validate release values') -and
    $release.Contains('Test-ReleaseArtifact.ps1')
) -Message 'Technical release validation remains active.'

$obsoleteTopLevelPaths = [string[]]@('Governance', 'MOBILE.md')
$obsoletePresent = [string[]]@(
    $obsoleteTopLevelPaths |
        Where-Object { Test-Path -LiteralPath (Join-Path $resolvedRoot $_) }
)
Add-Check -Id 'BOUNDARY-OBSOLETE-TOP-LEVEL' -Passed ($obsoletePresent.Count -eq 0) -Message (
    'Present={0}.' -f (ConvertTo-Json -InputObject ([string[]]$obsoletePresent) -Compress)
)

$operativePaths = $publicAuthorityPaths
$operativeDocuments = [System.Collections.Generic.Dictionary[string, string]]::new(
    [System.StringComparer]::Ordinal
)
foreach ($path in $operativePaths) {
    if ($documents.ContainsKey($path)) {
        $operativeDocuments.Add($path, $documents[$path])
    }
}
$publicAuthorityMissing = [string[]]@(
    $publicAuthorityPaths | Where-Object { -not $documents.ContainsKey($_) }
)
Add-Check -Id 'PUBLIC-AUTHORITY-COVERAGE' -Passed (
    $publicAuthorityMissing.Count -eq 0 -and
    $operativeDocuments.Count -eq $publicAuthorityPaths.Count
) -Message (
    'Authorities={0}; ActiveWholeDocuments={1}; Missing={2}.' -f
    $publicAuthorityPaths.Count,
    $operativeDocuments.Count,
    (ConvertTo-Json -InputObject ([string[]]$publicAuthorityMissing) -Compress)
)

# All remaining documentation is active public guidance. Historical execution
# reports were removed after their applicable contracts were consolidated.
$operativeDocuments['BACKLOG.md'] = $backlog
$planningIndex = [string]$documents['docs/planning/README.md']

$privateHostPathPatterns = [object[]]@(
    [pscustomobject]@{ id = 'PRIVATE_USER_PATH'; expression = '(?i)C:\\Users\\[^\\\r\n]+' },
    [pscustomobject]@{ id = 'PRIVATE_WORK_ROOT'; expression = '(?i)Codex-Work|<CodexTempRoot>' },
    [pscustomobject]@{ id = 'PRIVATE_SYNC_PATH'; expression = '(?i)OneDrive\s+-\s+[^\\/\r\n]+(?:\\[^\\\r\n]+)?' }
)
$forbiddenPatterns = [object[]]@(
    $privateHostPathPatterns + [object[]]@(
        [pscustomobject]@{ id = 'INTERNAL_TASK_ID'; expression = '(?i)\bINF-?[0-9]{3}(?:-[A-Z0-9]+)?\b' },
        [pscustomobject]@{ id = 'RETIRED_WORKFLOW_PATH'; expression = '(?-i:Governance[/\\])|(?i:MOBILE\.md|(?:New|Invoke|Test)-(?:Generic)?Governance|Test-ClassicReviewArtifact|FindingCorrectionHandoff|FINDING_CORRECTION)' },
        [pscustomobject]@{ id = 'PRIVATE_REVIEW_WORKFLOW'; expression = '(?i)ChatGPT Classic|Classic Review|ClassicReviewReady|Classic independent review' },
        [pscustomobject]@{ id = 'PRIVATE_CONTROL_PLANE_DEPENDENCY'; expression = '(?i)\b(?:requires?|depends?\s+on)\s+(?:access\s+to\s+)?(?:a\s+|the\s+)?(?:private|internal)\s+(?:development\s+)?control[- ]plane\b' }
    )
)

$privateHostPathFindings = [object[]]@(
    Get-InternalReferenceFindings -TextByPath $operativeDocuments -Patterns $privateHostPathPatterns
)
$privateActiveHostPathCount = 0
foreach ($finding in $privateHostPathFindings) {
    $privateActiveHostPathCount += [int]$finding.matchCount
}
Add-Check -Id 'BOUNDARY-PRIVATE-ACTIVE-HOST-PATHS' -Passed (
    $privateActiveHostPathCount -eq 0
) -Message ('PrivateActiveHostPathCount={0}.' -f $privateActiveHostPathCount)

$operativeFindings = [object[]]@(
    Get-InternalReferenceFindings -TextByPath $operativeDocuments -Patterns $forbiddenPatterns
)
$operativeFindingMessage = if ($operativeFindings.Count -eq 0) {
    'OperativeInternalReferenceCount=0.'
}
else {
    $operativeFindings | ConvertTo-Json -Compress -Depth 4
}
Add-Check -Id 'BOUNDARY-OPERATIVE-INTERNAL-REFERENCES' -Passed (
    $operativeFindings.Count -eq 0
) -Message $operativeFindingMessage

$runtimeFiles = [System.Collections.Generic.Dictionary[string, string]]::new(
    [System.StringComparer]::Ordinal
)
foreach ($relativeRoot in [string[]]@('cmd', 'internal')) {
    $fullRoot = Join-Path $resolvedRoot $relativeRoot
    foreach ($file in Get-ChildItem -LiteralPath $fullRoot -File -Recurse -Filter '*.go') {
        if ($file.Name.EndsWith('_test.go', [System.StringComparison]::Ordinal)) {
            continue
        }
        $relativePath = [System.IO.Path]::GetRelativePath($resolvedRoot, $file.FullName).Replace('\', '/')
        $runtimeFiles.Add($relativePath, [System.IO.File]::ReadAllText($file.FullName))
    }
}
$runtimeFindings = [object[]]@(
    Get-InternalReferenceFindings -TextByPath $runtimeFiles -Patterns $forbiddenPatterns
)
Add-Check -Id 'BOUNDARY-RUNTIME-DEPENDENCY' -Passed ($runtimeFindings.Count -eq 0) -Message (
    'RuntimeInternalReferenceCount={0}.' -f $runtimeFindings.Count
)

$documentationStyle = [string]$documents['docs/documentation-style.md']
$evidenceBoundaryValid =
    $documentationStyle.Contains('Removing a report does not resolve its findings.') -and
    $documentationStyle.Contains('outside the public tree before removal.') -and
    $planningIndex.Contains('BACKLOG.md') -and
    $planningIndex.Contains('tool-adapters.md')
Add-Check -Id 'DOCUMENTATION-EVIDENCE-BOUNDARY' -Passed (
    $evidenceBoundaryValid -and
    (Test-Path -LiteralPath (Join-Path $resolvedRoot 'CHANGELOG.md') -PathType Leaf)
) -Message 'Public product contracts and CHANGELOG remain; personal execution evidence is external.'

$failedChecks = [object[]]@($checks | Where-Object { -not $_.passed })
$result = [pscustomobject]@{
    schemaVersion                            = 3
    status                                   = if ($failedChecks.Count -eq 0) { 'PASS' } else { 'FAIL' }
    checkCount                               = $checks.Count
    passedCount                              = $checks.Count - $failedChecks.Count
    failureCount                             = $failedChecks.Count
    operativeInternalReferenceCount          = $operativeFindings.Count
    buildDependencyOnInternalInfrastructure  = $operativeFindings.Count -ne 0
    runtimeDependencyOnInternalInfrastructure = $runtimeFindings.Count -ne 0
    releaseDependencyOnInternalInfrastructure = $operativeFindings.Count -ne 0
    contributorDependencyOnInternalInfrastructure = $operativeFindings.Count -ne 0
    historicalClassification                 = 'EXTERNAL_EXECUTION_EVIDENCE'
    documentationRules                       = $documentationRules
    computedHighestAssignedBacklogIdentifier = $backlogHighest.ComputedHighestAssignedBacklogIdentifier
    declaredHighestAssignedBacklogIdentifier = $backlogHighest.DeclaredHighestAssignedBacklogIdentifier
    bl344ContradictoryStatusCount             = $bl344ContradictoryStatusCount
    bl344CompletedSummaryCount                = $bl344CompletedSummaryCount
    humanStatusModel                          = $statusModel
    privateActiveHostPathCount                = $privateActiveHostPathCount
    privateHostPathFindings                   = [object[]]$privateHostPathFindings
    operativeFindings                        = [object[]]$operativeFindings
    runtimeFindings                          = [object[]]$runtimeFindings
    checks                                   = [object[]]$checks
}

$result | ConvertTo-Json -Depth 6
if ($failedChecks.Count -ne 0) {
    exit 1
}
