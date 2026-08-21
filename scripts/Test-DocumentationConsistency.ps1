[CmdletBinding()]
param(
    [string]$RepositoryRoot = (Split-Path -Parent $PSScriptRoot),
    [string]$ReportPath
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$script:Checks = [System.Collections.Generic.List[object]]::new()
$script:Warnings = [System.Collections.Generic.List[object]]::new()
$script:Errors = [System.Collections.Generic.List[object]]::new()

function Add-DocumentationCheck {
    param(
        [Parameter(Mandatory)]
        [string]$Id,

        [Parameter(Mandatory)]
        [string]$Category,

        [Parameter(Mandatory)]
        [bool]$Passed,

        [Parameter(Mandatory)]
        [ValidateSet('Error', 'Warning')]
        [string]$Severity,

        [Parameter(Mandatory)]
        [string]$Message,

        [string]$Evidence = ''
    )

    $entry = [pscustomobject]@{
        Id       = $Id
        Result   = if ($Passed) { 'PASS' } else { 'FAIL' }
        Category = $Category
        Severity = $Severity
        Message  = $Message
        Evidence = $Evidence
    }

    [void]$script:Checks.Add($entry)

    if (-not $Passed) {
        if ($Severity -eq 'Warning') {
            [void]$script:Warnings.Add($entry)
        }
        else {
            [void]$script:Errors.Add($entry)
        }
    }
}

function Convert-ToMarkdownCell {
    param([AllowNull()][object]$Value)

    if ($null -eq $Value) {
        return ''
    }

    return ([string]$Value).Replace('|', '\|').Replace("`r", ' ').Replace("`n", ' ')
}

function Expand-BacklogReferences {
    param([Parameter(Mandatory)][string]$Text)

    $references = [System.Collections.Generic.List[string]]::new()
    $matches = [regex]::Matches($Text, 'BL-(?<start>\d{3})(?:\s*[–-]\s*BL-(?<end>\d{3}))?')

    foreach ($match in $matches) {
        $start = [int]$match.Groups['start'].Value
        $endGroup = $match.Groups['end']

        if (-not $endGroup.Success) {
            [void]$references.Add(('BL-{0:D3}' -f $start))
            continue
        }

        $end = [int]$endGroup.Value
        if ($end -lt $start) {
            throw "Invalid descending backlog range: $($match.Value)"
        }

        for ($number = $start; $number -le $end; $number++) {
            [void]$references.Add(('BL-{0:D3}' -f $number))
        }
    }

    return @($references)
}

function Get-SectionBody {
    param(
        [Parameter(Mandatory)][string]$Content,
        [Parameter(Mandatory)][string]$Heading
    )

    $pattern = '(?ms)^##\s+' + [regex]::Escape($Heading) + '\s*$\r?\n(?<body>.*?)(?=^##\s+|\z)'
    $match = [regex]::Match($Content, $pattern)
    if (-not $match.Success) {
        return $null
    }

    return $match.Groups['body'].Value
}

function Get-RelativeRepositoryPath {
    param(
        [Parameter(Mandatory)][string]$Root,
        [Parameter(Mandatory)][string]$Path
    )

    return ([System.IO.Path]::GetRelativePath($Root, $Path) -replace '\\', '/')
}

$status = 'FAIL'
$exitCode = 1
$resolvedRepositoryRoot = $null
$resolvedReportPath = $null
$markdownFiles = @()
$reportWritten = $false
$failureMessage = $null
$failureStack = $null
$nextAction = 'Correct the reported documentation errors and rerun the gate.'

try {
    $resolvedRepositoryRoot = [System.IO.Path]::GetFullPath($RepositoryRoot)

    if (-not (Test-Path -LiteralPath $resolvedRepositoryRoot -PathType Container)) {
        throw "Repository root does not exist: $resolvedRepositoryRoot"
    }

    foreach ($requiredPath in @('README.md', 'BACKLOG.md', 'docs', 'scripts')) {
        $candidate = Join-Path $resolvedRepositoryRoot $requiredPath
        Add-DocumentationCheck -Id "ROOT-$requiredPath" -Category 'RepositoryStructure' -Passed (Test-Path -LiteralPath $candidate) -Severity Error -Message "Required repository path exists: $requiredPath" -Evidence $candidate
    }

    if ([string]::IsNullOrWhiteSpace($ReportPath)) {
        $resolvedReportPath = Join-Path $resolvedRepositoryRoot 'build/reports/documentation-consistency.md'
    }
    elseif ([System.IO.Path]::IsPathRooted($ReportPath)) {
        $resolvedReportPath = [System.IO.Path]::GetFullPath($ReportPath)
    }
    else {
        $resolvedReportPath = [System.IO.Path]::GetFullPath((Join-Path $resolvedRepositoryRoot $ReportPath))
    }

    $markdownFiles = @(Get-ChildItem -LiteralPath $resolvedRepositoryRoot -Recurse -File -Filter '*.md' |
        Where-Object { $_.FullName -notmatch '[\\/]build[\\/]' } |
        Sort-Object FullName)

    Add-DocumentationCheck -Id 'DOC-001' -Category 'Inventory' -Passed ($markdownFiles.Count -gt 0) -Severity Error -Message 'At least one Markdown document exists.' -Evidence $markdownFiles.Count

    $strictUtf8 = [System.Text.UTF8Encoding]::new($false, $true)
    $contentByPath = @{}
    $utf8Failures = [System.Collections.Generic.List[string]]::new()

    foreach ($file in $markdownFiles) {
        try {
            $contentByPath[$file.FullName] = [System.IO.File]::ReadAllText($file.FullName, $strictUtf8)
        }
        catch {
            [void]$utf8Failures.Add((Get-RelativeRepositoryPath -Root $resolvedRepositoryRoot -Path $file.FullName))
        }
    }

    Add-DocumentationCheck -Id 'DOC-002' -Category 'Encoding' -Passed ($utf8Failures.Count -eq 0) -Severity Error -Message 'All Markdown files are valid UTF-8.' -Evidence ($utf8Failures -join ' | ')

    $hashGroups = $markdownFiles |
        Group-Object -Property { (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash }
    $duplicateGroups = @($hashGroups | Where-Object Count -gt 1)
    $duplicateEvidence = @($duplicateGroups | ForEach-Object {
            ($_.Group | ForEach-Object { Get-RelativeRepositoryPath -Root $resolvedRepositoryRoot -Path $_.FullName }) -join ' | '
        }) -join '; '

    Add-DocumentationCheck -Id 'DOC-003' -Category 'Duplication' -Passed ($duplicateGroups.Count -eq 0) -Severity Error -Message 'No Markdown files have identical content.' -Evidence $duplicateEvidence

    $brokenLinks = [System.Collections.Generic.List[string]]::new()
    $checkedLinkCount = 0
    $rootPrefix = $resolvedRepositoryRoot.TrimEnd([System.IO.Path]::DirectorySeparatorChar, [System.IO.Path]::AltDirectorySeparatorChar) + [System.IO.Path]::DirectorySeparatorChar
    $pathComparison = if ([System.OperatingSystem]::IsWindows()) { [System.StringComparison]::OrdinalIgnoreCase } else { [System.StringComparison]::Ordinal }

    foreach ($file in $markdownFiles) {
        if (-not $contentByPath.ContainsKey($file.FullName)) {
            continue
        }

        $content = $contentByPath[$file.FullName]
        $scanContent = [regex]::Replace($content, '(?ms)```.*?```', '')
        $linkMatches = [regex]::Matches($scanContent, '!?\[[^\]]*\]\((?<target>[^)\r\n]+)\)')

        foreach ($linkMatch in $linkMatches) {
            $rawTarget = $linkMatch.Groups['target'].Value.Trim()
            if ([string]::IsNullOrWhiteSpace($rawTarget)) {
                continue
            }

            if ($rawTarget.StartsWith('<')) {
                $angleMatch = [regex]::Match($rawTarget, '^<(?<target>[^>]+)>')
                if (-not $angleMatch.Success) {
                    continue
                }
                $target = $angleMatch.Groups['target'].Value
            }
            else {
                $target = ($rawTarget -split '\s+', 2)[0]
            }

            if ($target -match '^(?:https?|mailto|data):' -or $target.StartsWith('#')) {
                continue
            }

            $targetWithoutFragment = ($target -split '[?#]', 2)[0]
            if ([string]::IsNullOrWhiteSpace($targetWithoutFragment)) {
                continue
            }

            $targetWithoutFragment = [System.Uri]::UnescapeDataString($targetWithoutFragment)
            $candidatePath = [System.IO.Path]::GetFullPath((Join-Path $file.DirectoryName $targetWithoutFragment))
            $checkedLinkCount++

            $insideRepository = $candidatePath.Equals($resolvedRepositoryRoot, $pathComparison) -or $candidatePath.StartsWith($rootPrefix, $pathComparison)
            if (-not $insideRepository -or -not (Test-Path -LiteralPath $candidatePath)) {
                $relativeSource = Get-RelativeRepositoryPath -Root $resolvedRepositoryRoot -Path $file.FullName
                [void]$brokenLinks.Add(('{0} -> {1}' -f $relativeSource, $target))
            }
        }
    }

    Add-DocumentationCheck -Id 'LINK-001' -Category 'Links' -Passed ($brokenLinks.Count -eq 0) -Severity Error -Message 'All relative inline Markdown links resolve inside the repository.' -Evidence ($brokenLinks -join '; ')
    Add-DocumentationCheck -Id 'LINK-002' -Category 'Links' -Passed ($checkedLinkCount -gt 0) -Severity Warning -Message 'At least one relative inline Markdown link was checked.' -Evidence $checkedLinkCount

    $backlogPath = Join-Path $resolvedRepositoryRoot 'BACKLOG.md'
    $backlogText = if ($contentByPath.ContainsKey($backlogPath)) { $contentByPath[$backlogPath] } else { [System.IO.File]::ReadAllText($backlogPath, $strictUtf8) }
    $catalogMarker = '## Canonical task catalog'
    $catalogIndex = $backlogText.IndexOf($catalogMarker, [System.StringComparison]::Ordinal)
    Add-DocumentationCheck -Id 'BL-001' -Category 'BacklogStructure' -Passed ($catalogIndex -ge 0) -Severity Error -Message 'BACKLOG.md contains the canonical task catalog marker.' -Evidence $catalogMarker

    if ($catalogIndex -ge 0) {
        $catalogText = $backlogText.Substring($catalogIndex)
        $taskMatches = [regex]::Matches($catalogText, '(?m)^\|\s*(?<id>BL-\d{3})\s*\|\s*(?<status>Done|In Progress|Planned|Later|Ready|Blocked)\s*\|')
        $taskDefinitions = @($taskMatches | ForEach-Object {
                [pscustomobject]@{
                    Id     = $_.Groups['id'].Value
                    Number = [int]$_.Groups['id'].Value.Substring(3)
                    Status = $_.Groups['status'].Value
                }
            })

        $duplicateTaskIds = @($taskDefinitions | Group-Object Id | Where-Object Count -gt 1 | ForEach-Object Name)
        Add-DocumentationCheck -Id 'BL-002' -Category 'BacklogCatalog' -Passed ($taskDefinitions.Count -gt 0) -Severity Error -Message 'The canonical task catalog contains task definitions.' -Evidence $taskDefinitions.Count
        Add-DocumentationCheck -Id 'BL-003' -Category 'BacklogCatalog' -Passed ($duplicateTaskIds.Count -eq 0) -Severity Error -Message 'Every canonical backlog ID is defined exactly once.' -Evidence ($duplicateTaskIds -join ' | ')

        $taskNumbers = @($taskDefinitions.Number | Sort-Object -Unique)
        $missingTaskIds = [System.Collections.Generic.List[string]]::new()
        if ($taskNumbers.Count -gt 0) {
            $maximumTaskNumber = ($taskNumbers | Measure-Object -Maximum).Maximum
            for ($number = 1; $number -le $maximumTaskNumber; $number++) {
                if ($number -notin $taskNumbers) {
                    [void]$missingTaskIds.Add(('BL-{0:D3}' -f $number))
                }
            }
        }
        else {
            $maximumTaskNumber = 0
        }

        Add-DocumentationCheck -Id 'BL-004' -Category 'BacklogCatalog' -Passed ($missingTaskIds.Count -eq 0) -Severity Error -Message 'Canonical backlog IDs form a continuous range beginning at BL-001.' -Evidence ($missingTaskIds -join ' | ')

        $taskStatus = @{}
        foreach ($definition in $taskDefinitions) {
            $taskStatus[$definition.Id] = $definition.Status
        }

        $sprintRowCandidates = [regex]::Matches($backlogText, '(?m)^\|\s*(?<sprint>SPR-[^|]+?)\s*\|\s*(?<status>Done|Planned|In progress)\s*\|\s*(?<ids>[^|]+?)\s*\|')
        $sprintMatches = [regex]::Matches($backlogText, '(?m)^\|\s*(?<sprint>SPR-(?<number>[1-9]\d*))\s*\|\s*(?<status>Done|Planned|In progress)\s*\|\s*(?<ids>[^|]+?)\s*\|')
        $invalidSprintIds = @($sprintRowCandidates | Where-Object { $_.Groups['sprint'].Value.Trim() -notmatch '^SPR-[1-9]\d*$' } | ForEach-Object { $_.Groups['sprint'].Value.Trim() })
        $duplicateSprintIds = @($sprintMatches | Group-Object { $_.Groups['sprint'].Value.Trim() } | Where-Object Count -gt 1 | ForEach-Object Name)
        $legacySprintRows = @([regex]::Matches($backlogText, '(?m)^\|\s*Sprint\s+3\.[^|]+\|') | ForEach-Object { $_.Value.Trim() })
        $assignments = [System.Collections.Generic.List[object]]::new()

        foreach ($sprintMatch in $sprintMatches) {
            $references = Expand-BacklogReferences -Text $sprintMatch.Groups['ids'].Value
            foreach ($reference in $references) {
                [void]$assignments.Add([pscustomobject]@{
                        Id           = $reference
                        Sprint       = $sprintMatch.Groups['sprint'].Value.Trim()
                        SprintStatus = $sprintMatch.Groups['status'].Value
                    })
            }
        }

        $duplicateAssignments = @($assignments | Group-Object Id | Where-Object Count -gt 1 | ForEach-Object {
                '{0}: {1}' -f $_.Name, (($_.Group.Sprint | Sort-Object -Unique) -join ' | ')
            })
        $unknownAssignments = @($assignments | Where-Object { -not $taskStatus.ContainsKey($_.Id) } | Select-Object -ExpandProperty Id -Unique | Sort-Object)
        $assignedIds = @($assignments.Id | Sort-Object -Unique)
        $unassignedPlanned = @($taskDefinitions | Where-Object { $_.Status -eq 'Planned' -and $_.Id -notin $assignedIds } | Select-Object -ExpandProperty Id | Sort-Object)
        $assignedLater = @($taskDefinitions | Where-Object { $_.Status -eq 'Later' -and $_.Id -in $assignedIds } | Select-Object -ExpandProperty Id | Sort-Object)
        $doneInPlannedSprints = @($assignments | Where-Object { $_.SprintStatus -eq 'Planned' -and $taskStatus.ContainsKey($_.Id) -and $taskStatus[$_.Id] -eq 'Done' } | ForEach-Object { "$($_.Id) in $($_.Sprint)" })
        $nonDoneInDoneSprints = @($assignments | Where-Object { $_.SprintStatus -eq 'Done' -and $taskStatus.ContainsKey($_.Id) -and $taskStatus[$_.Id] -ne 'Done' } | ForEach-Object { "$($_.Id) in $($_.Sprint)" })

        Add-DocumentationCheck -Id 'BL-005' -Category 'SprintAssignments' -Passed ($sprintMatches.Count -gt 0) -Severity Error -Message 'The sprint sequence contains status-bearing sprint rows.' -Evidence $sprintMatches.Count
        Add-DocumentationCheck -Id 'BL-006' -Category 'SprintAssignments' -Passed ($duplicateAssignments.Count -eq 0) -Severity Error -Message 'Every sprint-assigned backlog task appears in exactly one sprint row.' -Evidence ($duplicateAssignments -join '; ')
        Add-DocumentationCheck -Id 'BL-007' -Category 'SprintAssignments' -Passed ($unknownAssignments.Count -eq 0) -Severity Error -Message 'Every sprint reference resolves to a canonical backlog task.' -Evidence ($unknownAssignments -join ' | ')
        Add-DocumentationCheck -Id 'BL-008' -Category 'SprintAssignments' -Passed ($unassignedPlanned.Count -eq 0) -Severity Error -Message 'Every Planned task is assigned to a Version 1.0 sprint.' -Evidence ($unassignedPlanned -join ' | ')
        Add-DocumentationCheck -Id 'BL-009' -Category 'SprintAssignments' -Passed ($assignedLater.Count -eq 0) -Severity Error -Message 'No Later task is assigned to a Version 1.0 sprint.' -Evidence ($assignedLater -join ' | ')
        Add-DocumentationCheck -Id 'BL-010' -Category 'SprintStatus' -Passed $true -Severity Warning -Message 'Done tasks in Planned sprint rows are reported for semantic review because cross-cutting tasks may complete before the containing sprint.' -Evidence ($doneInPlannedSprints -join '; ')
        Add-DocumentationCheck -Id 'BL-011' -Category 'SprintStatus' -Passed ($nonDoneInDoneSprints.Count -eq 0) -Severity Error -Message 'Done sprint rows contain only Done tasks.' -Evidence ($nonDoneInDoneSprints -join '; ')
        Add-DocumentationCheck -Id 'BL-012' -Category 'BacklogCatalog' -Passed ($maximumTaskNumber -eq $taskDefinitions.Count) -Severity Error -Message 'The maximum backlog number equals the number of unique canonical tasks.' -Evidence "Maximum=$maximumTaskNumber; Definitions=$($taskDefinitions.Count)"
        Add-DocumentationCheck -Id 'SPR-001' -Category 'SprintIdentifiers' -Passed ($invalidSprintIds.Count -eq 0) -Severity Error -Message 'Every current sprint row uses SPR-<positive integer> without suffixes or decimals.' -Evidence ($invalidSprintIds -join ' | ')
        Add-DocumentationCheck -Id 'SPR-002' -Category 'SprintIdentifiers' -Passed ($duplicateSprintIds.Count -eq 0) -Severity Error -Message 'Every canonical sprint identifier appears in exactly one status row.' -Evidence ($duplicateSprintIds -join ' | ')
        Add-DocumentationCheck -Id 'SPR-003' -Category 'SprintIdentifiers' -Passed ($legacySprintRows.Count -eq 0) -Severity Error -Message 'The current backlog contains no legacy Sprint 3.x status rows.' -Evidence ($legacySprintRows -join '; ')
    }

    $expectedRepository = 'thomasweidner/flashgate-mcp'
    $expectedModule = 'github.com/thomasweidner/flashgate-mcp'
    $historicalOwner = 'blacksheepkhan/flashgate-mcp'
    $historicalModule = 'github.com/blacksheepkhan/flashgate-mcp'
    $activeIdentityDocuments = @('README.md', 'BACKLOG.md', 'docs/architecture.md', 'docs/project-identity.md')

    foreach ($relativePath in $activeIdentityDocuments) {
        $fullPath = Join-Path $resolvedRepositoryRoot $relativePath
        $content = if ($contentByPath.ContainsKey($fullPath)) { $contentByPath[$fullPath] } else { [System.IO.File]::ReadAllText($fullPath, $strictUtf8) }
        $safeId = ($relativePath -replace '[^A-Za-z0-9]+', '-').Trim('-').ToUpperInvariant()

        Add-DocumentationCheck -Id ('ID-{0}-REPO' -f $safeId) -Category 'ProjectIdentity' -Passed $content.Contains($expectedRepository, [System.StringComparison]::Ordinal) -Severity Error -Message ('{0} contains the current repository identity.' -f $relativePath) -Evidence $expectedRepository
        Add-DocumentationCheck -Id ('ID-{0}-MODULE' -f $safeId) -Category 'ProjectIdentity' -Passed $content.Contains($expectedModule, [System.StringComparison]::Ordinal) -Severity Error -Message ('{0} contains the current Go module identity.' -f $relativePath) -Evidence $expectedModule
        $containsHistoricalCurrentIdentity = $content.Contains($historicalOwner, [System.StringComparison]::Ordinal) -or $content.Contains($historicalModule, [System.StringComparison]::Ordinal)
        Add-DocumentationCheck -Id ('ID-{0}-HISTORICAL' -f $safeId) -Category 'ProjectIdentity' -Passed (-not $containsHistoricalCurrentIdentity) -Severity Error -Message ('{0} does not present the former owner identity in an active identity document.' -f $relativePath) -Evidence $historicalOwner
    }

    $architecturePath = Join-Path $resolvedRepositoryRoot 'docs/architecture.md'
    $architectureText = if ($contentByPath.ContainsKey($architecturePath)) { $contentByPath[$architecturePath] } else { [System.IO.File]::ReadAllText($architecturePath, $strictUtf8) }
    $currentState = Get-SectionBody -Content $architectureText -Heading 'Current state'
    Add-DocumentationCheck -Id 'CUR-001' -Category 'CurrentStateBoundary' -Passed ($null -ne $currentState) -Severity Error -Message 'Architecture contains a dedicated Current state section.'

    if ($null -ne $currentState) {
        Add-DocumentationCheck -Id 'CUR-002' -Category 'CurrentStateBoundary' -Passed $currentState.Contains('eight filesystem tools', [System.StringComparison]::OrdinalIgnoreCase) -Severity Error -Message 'Current state identifies the eight implemented filesystem tools.' -Evidence 'eight filesystem tools'
        Add-DocumentationCheck -Id 'CUR-003' -Category 'CurrentStateBoundary' -Passed $currentState.Contains('Not yet implemented:', [System.StringComparison]::Ordinal) -Severity Error -Message 'Current state contains an explicit Not yet implemented boundary.' -Evidence 'Not yet implemented:'

        foreach ($plannedMarker in @('multiple named roots', 'general profiles/capabilities', 'search', 'Operations/Job Manager', 'process observation/management', 'typed command execution', 'system-information tools')) {
            $markerId = ($plannedMarker -replace '[^A-Za-z0-9]+', '-').Trim('-').ToUpperInvariant()
            $listedAsNotImplemented = [regex]::IsMatch($currentState, '(?ms)Not yet implemented:.*?-\s*' + [regex]::Escape($plannedMarker) + '\s*;')
            Add-DocumentationCheck -Id ('CUR-{0}' -f $markerId) -Category 'CurrentStateBoundary' -Passed $listedAsNotImplemented -Severity Error -Message ("Architecture explicitly lists '{0}' as not yet implemented." -f $plannedMarker) -Evidence $plannedMarker
        }
    }

    $expectedTools = @('list_directory', 'read_file', 'get_path_info', 'write_file', 'create_directory', 'delete_path', 'copy_path', 'move_path')
    $catalogPath = Join-Path $resolvedRepositoryRoot 'docs/mcp-tool-catalog.json'
    $catalog = Get-Content -LiteralPath $catalogPath -Raw -Encoding UTF8 | ConvertFrom-Json -Depth 100
    $catalogTools = @($catalog.tools | ForEach-Object { $_.name })
    Add-DocumentationCheck -Id 'TOOL-001' -Category 'ToolCatalog' -Passed (@(Compare-Object -ReferenceObject $expectedTools -DifferenceObject $catalogTools -SyncWindow 0).Count -eq 0) -Severity Error -Message 'The machine-readable catalog contains the exact eight implemented tools in canonical order.' -Evidence ($catalogTools | ConvertTo-Json -Compress)

    $readmePath = Join-Path $resolvedRepositoryRoot 'README.md'
    $readmeText = if ($contentByPath.ContainsKey($readmePath)) { $contentByPath[$readmePath] } else { [System.IO.File]::ReadAllText($readmePath, $strictUtf8) }
    $implementedToolsSection = Get-SectionBody -Content $readmeText -Heading 'Implemented MCP Tools'
    $readmeTools = @()
    if ($null -ne $implementedToolsSection) {
        $readmeTools = @([regex]::Matches($implementedToolsSection, '(?m)^\|\s*`(?<tool>[a-z0-9_]+)`\s*\|') | ForEach-Object { $_.Groups['tool'].Value })
    }
    Add-DocumentationCheck -Id 'TOOL-002' -Category 'ToolCatalog' -Passed (@(Compare-Object -ReferenceObject $expectedTools -DifferenceObject $readmeTools -SyncWindow 0).Count -eq 0) -Severity Error -Message 'README lists the exact eight implemented tools in canonical order.' -Evidence ($readmeTools | ConvertTo-Json -Compress)

    $toolsDocumentPath = Join-Path $resolvedRepositoryRoot 'docs/tools.md'
    $toolsDocumentText = if ($contentByPath.ContainsKey($toolsDocumentPath)) { $contentByPath[$toolsDocumentPath] } else { [System.IO.File]::ReadAllText($toolsDocumentPath, $strictUtf8) }
    $toolsBlockMatch = [regex]::Match($toolsDocumentText, '(?ms)exposes eight filesystem tools.*?```text\s*(?<tools>.*?)```')
    $documentedTools = @()
    if ($toolsBlockMatch.Success) {
        $documentedTools = @($toolsBlockMatch.Groups['tools'].Value -split '\r?\n' | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | ForEach-Object { $_.Trim() })
    }
    Add-DocumentationCheck -Id 'TOOL-003' -Category 'ToolCatalog' -Passed (@(Compare-Object -ReferenceObject $expectedTools -DifferenceObject $documentedTools -SyncWindow 0).Count -eq 0) -Severity Error -Message 'docs/tools.md lists the exact eight implemented tools in canonical order.' -Evidence ($documentedTools | ConvertTo-Json -Compress)

    $governanceSources = @(
        'Governance/CHANGE-TRIGGER-REVIEW-AND-BACKLOG-STANDARD.md',
        'Governance/FINDING-REMEDIATION-AND-REVIEW-MODE-STANDARD.md',
        'Governance/HANDOFF-ARTIFACT-AND-CLASSIC-READINESS-STANDARD.md',
        'Governance/change-trigger-catalog.json',
        'Governance/assignment-governance-record.schema.json',
        'Governance/completion-report.schema.json',
        'Governance/finding-correction-matrix.schema.json',
        'Governance/finding-regression-matrix.schema.json',
        'Governance/publication-regression-evidence.schema.json',
        'Governance/publication-regression-expected-execution-input-binding.schema.json',
        'Governance/publication-regression-result.schema.json',
        'Governance/publication-regression-result-v1.schema.json',
        'Governance/publication-regression-matrix-catalog.schema.json',
        'Governance/publication-regression-matrix-catalog.json',
        'Governance/focused-delta-review-record.schema.json',
        'Governance/governance-handoff-contract.schema.json',
        'Governance/governance-report-contract.schema.json',
        'scripts/New-GovernanceHandoff.ps1',
        'scripts/New-GovernanceWorkflowRecord.ps1',
        'scripts/Test-GovernanceConsistency.ps1',
        'scripts/Test-GovernanceConsistencyFixtures.ps1'
    )
    foreach ($relativePath in $governanceSources) {
        $governancePath = Join-Path $resolvedRepositoryRoot $relativePath
        $governanceId = ($relativePath -replace '[^A-Za-z0-9]+', '-').Trim('-').ToUpperInvariant()
        Add-DocumentationCheck -Id ('GOV-{0}' -f $governanceId) -Category 'Governance' -Passed (Test-Path -LiteralPath $governancePath -PathType Leaf) -Severity Error -Message ('Required governance source exists: {0}' -f $relativePath) -Evidence $relativePath
    }

    $governanceCatalogPath = Join-Path $resolvedRepositoryRoot 'Governance/change-trigger-catalog.json'
    $governanceSchemaPath = Join-Path $resolvedRepositoryRoot 'Governance/assignment-governance-record.schema.json'
    $governanceCatalog = Get-Content -LiteralPath $governanceCatalogPath -Raw -Encoding UTF8 | ConvertFrom-Json -Depth 100
    $governanceSchema = Get-Content -LiteralPath $governanceSchemaPath -Raw -Encoding UTF8 | ConvertFrom-Json -Depth 100
    Add-DocumentationCheck -Id 'GOV-CATALOG-TRIGGERS' -Category 'Governance' -Passed (@($governanceCatalog.triggers).Count -ge 14) -Severity Error -Message 'Governance catalog contains the complete current trigger set.' -Evidence @($governanceCatalog.triggers).Count
    Add-DocumentationCheck -Id 'GOV-SCHEMA-STRICT' -Category 'Governance' -Passed ($governanceSchema.additionalProperties -eq $false) -Severity Error -Message 'Assignment governance schema rejects unknown top-level properties.'
    $completionSchemaPath = Join-Path $resolvedRepositoryRoot 'Governance/completion-report.schema.json'
    $completionSchema = Get-Content -LiteralPath $completionSchemaPath -Raw -Encoding UTF8 | ConvertFrom-Json -Depth 100
    Add-DocumentationCheck -Id 'GOV-COMPLETION-SCHEMA-STRICT' -Category 'Governance' -Passed ($completionSchema.additionalProperties -eq $false) -Severity Error -Message 'Completion-report schema rejects unknown top-level properties.'
    foreach ($contractSchemaName in @(
            'finding-correction-matrix.schema.json',
            'finding-regression-matrix.schema.json',
            'publication-regression-evidence.schema.json',
            'publication-regression-expected-execution-input-binding.schema.json',
            'publication-regression-result.schema.json',
            'publication-regression-result-v1.schema.json',
            'publication-regression-matrix-catalog.schema.json',
            'focused-delta-review-record.schema.json',
            'governance-handoff-contract.schema.json',
            'governance-report-contract.schema.json'
        )) {
        $contractSchemaPath = Join-Path $resolvedRepositoryRoot (
            'Governance/' + $contractSchemaName
        )
        $contractSchema = Get-Content -LiteralPath $contractSchemaPath -Raw -Encoding UTF8 |
            ConvertFrom-Json -Depth 100
        Add-DocumentationCheck -Id (
            'GOV-' + $contractSchemaName.Replace('.schema.json', '').Replace('-', '-').ToUpperInvariant() + '-STRICT'
        ) -Category 'Governance' -Passed ($contractSchema.additionalProperties -eq $false) `
            -Severity Error -Message "$contractSchemaName rejects unknown top-level properties."
    }

    $handoffStandardPath = Join-Path $resolvedRepositoryRoot 'Governance/HANDOFF-ARTIFACT-AND-CLASSIC-READINESS-STANDARD.md'
    $handoffStandard = [System.IO.File]::ReadAllText($handoffStandardPath, $strictUtf8)
    $canonicalArtifactValidator = 'C:\Users\ThomasW\OneDrive - VOXTRONIC\Desktop\Voxtronic\Codex-Work\Scripts\Test-ClassicReviewArtifact.ps1'
    foreach ($bindingText in @(
            $canonicalArtifactValidator,
            '-ArtifactPath',
            '-ReadinessRequirement RequireTrue',
            'PowerShell 7.6.5',
            'Exit code `0` is PASS',
            'prerequisite static/semantic gate',
            'it is not an alternative'
        )) {
        $bindingId = ($bindingText -replace '[^A-Za-z0-9]+', '-').Trim('-').ToUpperInvariant()
        Add-DocumentationCheck -Id ('GOV-CANONICAL-ARTIFACT-{0}' -f $bindingId) -Category 'Governance' -Passed (
            $handoffStandard.Contains($bindingText, [System.StringComparison]::Ordinal)
        ) -Severity Error -Message "Handoff standard binds canonical artifact-validator contract: $bindingText"
    }

    foreach ($relativePath in @('AGENTS.md', 'README.md', 'CONTRIBUTING.md', 'docs/testing.md', 'docs/documentation-quality-gate.md')) {
        $fullPath = Join-Path $resolvedRepositoryRoot $relativePath
        $sourceText = if ($contentByPath.ContainsKey($fullPath)) { $contentByPath[$fullPath] } else { [System.IO.File]::ReadAllText($fullPath, $strictUtf8) }
        $governanceId = ($relativePath -replace '[^A-Za-z0-9]+', '-').Trim('-').ToUpperInvariant()
        Add-DocumentationCheck -Id ('GOV-REFERENCE-{0}' -f $governanceId) -Category 'Governance' -Passed $sourceText.Contains('Test-GovernanceConsistency', [System.StringComparison]::Ordinal) -Severity Error -Message ('{0} references the governance enforcement gate.' -f $relativePath)
    }

    $status = if ($script:Errors.Count -gt 0) { 'FAIL' } elseif ($script:Warnings.Count -gt 0) { 'PASS_WITH_WARNINGS' } else { 'PASS' }
    $exitCode = if ($script:Errors.Count -gt 0) { 1 } else { 0 }
    $nextAction = if ($script:Errors.Count -gt 0) {
        'Correct the reported documentation errors and rerun the gate.'
    }
    elseif ($script:Warnings.Count -gt 0) {
        'Review the warnings, complete the manual checklist, and record the result in the pull request.'
    }
    else {
        'Complete the manual checklist and record the PASS result in the pull request.'
    }

    $reportDirectory = Split-Path -Parent $resolvedReportPath
    [void][System.IO.Directory]::CreateDirectory($reportDirectory)

    $reportLines = [System.Collections.Generic.List[string]]::new()
    [void]$reportLines.Add('# FlashGate documentation consistency report')
    [void]$reportLines.Add('')
    [void]$reportLines.Add("- Generated: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss zzz')")
    [void]$reportLines.Add(('- Repository root: `{0}`' -f (Convert-ToMarkdownCell $resolvedRepositoryRoot)))
    [void]$reportLines.Add("- Status: **$status**")
    [void]$reportLines.Add("- Markdown files: $($markdownFiles.Count)")
    [void]$reportLines.Add("- Checks: $($script:Checks.Count)")
    [void]$reportLines.Add("- Warnings: $($script:Warnings.Count)")
    [void]$reportLines.Add("- Errors: $($script:Errors.Count)")
    [void]$reportLines.Add('')
    [void]$reportLines.Add('## Check matrix')
    [void]$reportLines.Add('')
    [void]$reportLines.Add('| ID | Result | Category | Severity | Message | Evidence |')
    [void]$reportLines.Add('|---|---|---|---|---|---|')

    foreach ($check in $script:Checks) {
        $checkLine = '| {0} | {1} | {2} | {3} | {4} | {5} |' -f @(
            (Convert-ToMarkdownCell $check.Id),
            (Convert-ToMarkdownCell $check.Result),
            (Convert-ToMarkdownCell $check.Category),
            (Convert-ToMarkdownCell $check.Severity),
            (Convert-ToMarkdownCell $check.Message),
            (Convert-ToMarkdownCell $check.Evidence)
        )
        [void]$reportLines.Add($checkLine)
    }

    [void]$reportLines.Add('')
    [void]$reportLines.Add('## Manual review required')
    [void]$reportLines.Add('')
    [void]$reportLines.Add('The automated gate does not replace review of semantic claims. Confirm current versus planned behavior, historical-document immutability, task status against code/CI evidence, and all documents affected by the change.')

    [System.IO.File]::WriteAllLines($resolvedReportPath, $reportLines, [System.Text.UTF8Encoding]::new($false))
    $reportWritten = $true
}
catch {
    $status = 'FAIL'
    $exitCode = 2
    $failureMessage = $_.Exception.Message
    $failureStack = $_.ScriptStackTrace
    $nextAction = 'Resolve the infrastructure or script failure before evaluating documentation consistency.'
    [void]$script:Errors.Add([pscustomobject]@{
            Id       = 'INF-001'
            Result   = 'FAIL'
            Category = 'Infrastructure'
            Severity = 'Error'
            Message  = $failureMessage
            Evidence = $failureStack
        })

    if ($null -ne $resolvedReportPath) {
        try {
            $reportDirectory = Split-Path -Parent $resolvedReportPath
            [void][System.IO.Directory]::CreateDirectory($reportDirectory)
            $failureReport = @(
                '# FlashGate documentation consistency report'
                ''
                "- Generated: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss zzz')"
                '- Status: **FAIL**'
                '- Failure type: Infrastructure or script execution'
                ''
                '## Failure'
                ''
                ('- Message: `{0}`' -f (Convert-ToMarkdownCell $failureMessage))
                ('- Stack: `{0}`' -f (Convert-ToMarkdownCell $failureStack))
            )
            [System.IO.File]::WriteAllLines($resolvedReportPath, $failureReport, [System.Text.UTF8Encoding]::new($false))
            $reportWritten = $true
        }
        catch {
            $reportWritten = $false
        }
    }
}
finally {
    # No temporary state is retained. The report is written only to the requested build/report path.
}

[pscustomobject]@{
    Status        = $status
    RepositoryRoot = $resolvedRepositoryRoot
    MarkdownFiles = $markdownFiles.Count
    Checks        = $script:Checks.Count
    WarningCount  = $script:Warnings.Count
    ErrorCount     = $script:Errors.Count
    FailureMessage = $failureMessage
    ReportPath    = if ($reportWritten) { $resolvedReportPath } else { $null }
    NextAction    = $nextAction
} | Format-List

exit $exitCode
