#requires -Version 7.6
[CmdletBinding()]
param(
    [string]$ResultPath,
    [string]$ExpectedExecutionInputBindingPath,
    [string]$ExpectedExecutionInputBindingSha256,
    [string]$PhaseHandshakeDirectory,
    [string]$PhaseContinueDirectory,
    [ValidateSet('', 'PARENT_BINDING_CAPTURED', 'RUNNER_IMPORTS_COMPLETED')]
    [string]$PausePhase = ''
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$matrixId = 'GOVERNANCE_HANDOFF_PUBLICATION_FIXTURES'
$results = [System.Collections.Generic.List[object]]::new()
$status = 'FAIL'
$failureMessage = $null
$root = $null
$utf8 = [System.Text.UTF8Encoding]::new($false)
$utf8Strict = [System.Text.UTF8Encoding]::new($false, $true)
$executionInputBinding = $null
$caseExecutionAuthorized = $false

function Get-IndependentFileSha256 {
    param([Parameter(Mandatory)][string]$LiteralPath)
    $bytes = [System.IO.File]::ReadAllBytes($LiteralPath)
    return [Convert]::ToHexString(
        [System.Security.Cryptography.SHA256]::HashData($bytes)
    ).ToLowerInvariant()
}

function Assert-NoDuplicateJsonProperties {
    param(
        [Parameter(Mandatory)][System.Text.Json.JsonElement]$Element,
        [Parameter(Mandatory)][string]$JsonPath
    )
    if ($Element.ValueKind -eq [System.Text.Json.JsonValueKind]::Object) {
        $names = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::Ordinal)
        foreach ($property in $Element.EnumerateObject()) {
            if (-not $names.Add($property.Name)) { throw "Duplicate JSON property at $JsonPath/$($property.Name)." }
            Assert-NoDuplicateJsonProperties -Element $property.Value -JsonPath "$JsonPath/$($property.Name)"
        }
    }
    elseif ($Element.ValueKind -eq [System.Text.Json.JsonValueKind]::Array) {
        $index = 0
        foreach ($item in $Element.EnumerateArray()) {
            Assert-NoDuplicateJsonProperties -Element $item -JsonPath "$JsonPath/$index"
            $index++
        }
    }
}

function Read-IndependentStrictJson {
    param([Parameter(Mandatory)][string]$LiteralPath)
    $bytes = [System.IO.File]::ReadAllBytes($LiteralPath)
    if ($bytes.Length -ge 3 -and $bytes[0] -eq 0xef -and $bytes[1] -eq 0xbb -and $bytes[2] -eq 0xbf) {
        throw "JSON BOM is forbidden: $LiteralPath"
    }
    $text = $utf8Strict.GetString($bytes)
    $options = [System.Text.Json.JsonDocumentOptions]::new()
    $options.AllowTrailingCommas = $false
    $options.CommentHandling = [System.Text.Json.JsonCommentHandling]::Disallow
    $document = [System.Text.Json.JsonDocument]::Parse($text, $options)
    try { Assert-NoDuplicateJsonProperties -Element $document.RootElement -JsonPath '$' }
    finally { $document.Dispose() }
    return $text | ConvertFrom-Json -Depth 100 -DateKind String
}

function Assert-ExactPropertySet {
    param([Parameter(Mandatory)][object]$Value, [Parameter(Mandatory)][string[]]$Expected, [string]$Label)
    [string[]]$actual = @($Value.PSObject.Properties.Name)
    [array]::Sort($actual, [System.StringComparer]::Ordinal)
    [string[]]$expectedSorted = @($Expected)
    [array]::Sort($expectedSorted, [System.StringComparer]::Ordinal)
    if (($actual -join "`n") -cne ($expectedSorted -join "`n")) { throw "$Label property set is not canonical." }
}

function New-IndependentExecutionInputBinding {
    $productRoot = Split-Path -Parent $PSScriptRoot
    $catalogRelativePath = 'Governance/publication-regression-matrix-catalog.json'
    $catalogPath = Join-Path $productRoot $catalogRelativePath
    $catalog = Read-IndependentStrictJson -LiteralPath $catalogPath
    $matrixRecords = @($catalog.matrices | Where-Object matrixId -CEQ $matrixId)
    if ($matrixRecords.Count -ne 1) { throw "Publication matrix must be unique: $matrixId" }
    $matrix = $matrixRecords[0]
    if ([string]$matrix.runner -cne 'scripts/Test-GovernanceHandoffPublicationFixtures.ps1') {
        throw 'Publication matrix runner binding is not canonical.'
    }
    $bindingPaths = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::OrdinalIgnoreCase)
    $getBindings = {
        param([object[]]$RelativePaths, [string]$Kind)
        return @($RelativePaths | ForEach-Object {
                $relativePath = [string]$_
                if ([string]::IsNullOrWhiteSpace($relativePath) -or $relativePath -match '^(?:[A-Za-z]:|/|\\|.*(?:^|/)\.\.(?:/|$))' -or
                    -not $bindingPaths.Add($relativePath)) {
                    throw "Invalid, duplicate, or case-colliding publication $Kind path: $relativePath"
                }
                $fullPath = [System.IO.Path]::GetFullPath((Join-Path $productRoot $relativePath))
                if (-not $fullPath.StartsWith($productRoot + [System.IO.Path]::DirectorySeparatorChar,
                        [System.StringComparison]::OrdinalIgnoreCase) -or
                    -not (Test-Path -LiteralPath $fullPath -PathType Leaf)) {
                    throw "Publication $Kind input is outside the product root or missing: $relativePath"
                }
                [ordered]@{ path = $relativePath; sha256 = Get-IndependentFileSha256 -LiteralPath $fullPath }
            })
    }
    return [ordered]@{
        matrixDefinitionArtifact = $catalogRelativePath
        matrixDefinitionSha256 = Get-IndependentFileSha256 -LiteralPath $catalogPath
        sourceBindings = @(& $getBindings -RelativePaths @($matrix.sourcePaths) -Kind 'source')
        dependencyBindings = @(& $getBindings -RelativePaths @($matrix.dependencyPaths) -Kind 'dependency')
    }
}

function Read-ExpectedExecutionInputBinding {
    param([Parameter(Mandatory)][string]$LiteralPath)
    if ([string]::IsNullOrWhiteSpace($ExpectedExecutionInputBindingSha256) -or
        $ExpectedExecutionInputBindingSha256 -cnotmatch '^[0-9a-f]{64}$' -or
        (Get-IndependentFileSha256 -LiteralPath $LiteralPath) -cne $ExpectedExecutionInputBindingSha256) {
        throw 'Expected execution input binding artifact identity mismatch.'
    }
    $contract = Read-IndependentStrictJson -LiteralPath $LiteralPath
    Assert-ExactPropertySet $contract @('schemaVersion', 'matrixId', 'executionInputBinding') 'Expected binding'
    if ([int]$contract.schemaVersion -ne 1 -or [string]$contract.matrixId -cne $matrixId) {
        throw 'Expected execution input binding discriminator mismatch.'
    }
    Assert-ExactPropertySet $contract.executionInputBinding `
        @('matrixDefinitionArtifact', 'matrixDefinitionSha256', 'sourceBindings', 'dependencyBindings') `
        'Expected execution input binding'
    foreach ($binding in @($contract.executionInputBinding.sourceBindings) + @($contract.executionInputBinding.dependencyBindings)) {
        Assert-ExactPropertySet $binding @('path', 'sha256') 'Expected file binding'
        if ([string]$binding.sha256 -cnotmatch '^[0-9a-f]{64}$') { throw "Invalid expected SHA-256: $($binding.path)" }
    }
    return $contract
}

function Assert-ExpectedExecutionInputBinding {
    param([Parameter(Mandatory)][string]$LiteralPath)
    $expected = Read-ExpectedExecutionInputBinding -LiteralPath $LiteralPath
    $actual = New-IndependentExecutionInputBinding
    $expectedJson = $expected.executionInputBinding | ConvertTo-Json -Depth 20 -Compress
    $actualJson = $actual | ConvertTo-Json -Depth 20 -Compress
    if ($expectedJson -cne $actualJson) { throw 'Expected execution input binding does not match current catalog/source/dependency bytes.' }
    return $expected.executionInputBinding
}

function Invoke-BindingPhase {
    param([Parameter(Mandatory)][string]$Phase)
    if ([string]::IsNullOrWhiteSpace($PhaseHandshakeDirectory)) { return }
    [void][System.IO.Directory]::CreateDirectory($PhaseHandshakeDirectory)
    $readyPath = Join-Path $PhaseHandshakeDirectory ($Phase + '.ready')
    [System.IO.File]::WriteAllText($readyPath, "$PID`n", $utf8)
    if ($PausePhase -cne $Phase) { return }
    if ([string]::IsNullOrWhiteSpace($PhaseContinueDirectory)) { throw "Missing continue directory for phase $Phase." }
    $continuePath = Join-Path $PhaseContinueDirectory ($Phase + '.continue')
    $stopwatch = [System.Diagnostics.Stopwatch]::StartNew()
    while (-not (Test-Path -LiteralPath $continuePath -PathType Leaf)) {
        if ($stopwatch.Elapsed.TotalSeconds -ge 30) { throw "Timed out waiting for phase continuation: $Phase" }
        [System.Threading.Thread]::Sleep(50)
    }
}

function Add-Case {
    param([string]$Id, [bool]$Passed, [string]$Evidence = '')
    if (-not $caseExecutionAuthorized -or $null -eq $executionInputBinding) {
        throw "[$Id] Execution input binding was not verified before case execution."
    }
    [void](Assert-ExpectedExecutionInputBinding -LiteralPath $ExpectedExecutionInputBindingPath)
    [void]$results.Add([pscustomobject]@{ id=$Id; result=if($Passed){'PASS'}else{'FAIL'}; evidence=$Evidence })
    if (-not $Passed) { throw "[$Id] $Evidence" }
}

function Invoke-ExpectedFailure {
    param([scriptblock]$Operation)
    try { & $Operation; return $false } catch { return $true }
}

function Test-ZipCandidate {
    param([string]$CandidatePath)
    $archive = [System.IO.Compression.ZipFile]::OpenRead($CandidatePath)
    try { if (@($archive.Entries).Count -ne 1) { throw 'Synthetic candidate member count mismatch.' } }
    finally { $archive.Dispose() }
}

function Invoke-CandidateDriftProcess {
    param([string]$CandidatePath, [ValidateSet('MUTATE','REPLACE')][string]$Mode, [string]$CaseRoot)
    $handshake = Join-Path $CaseRoot ("drift-$($Mode.ToLowerInvariant()).txt")
    $arguments = @(
        '-NoLogo','-NoProfile','-File',(Join-Path $PSScriptRoot 'testdata/Invoke-GovernanceHandoffCandidateDrift.ps1'),
        '-CandidatePath',$CandidatePath,'-Mode',$Mode,'-HandshakePath',$handshake
    )
    $process = Start-Process -FilePath (Get-Process -Id $PID).Path -ArgumentList $arguments `
        -PassThru -Wait -WindowStyle Hidden
    if ($process.ExitCode -ne 0 -or -not (Test-Path -LiteralPath $handshake -PathType Leaf) -or
        [System.IO.File]::ReadAllText($handshake, $utf8) -cne $Mode) {
        throw "Candidate drift process did not complete deterministically: $Mode"
    }
}

function Invoke-InterruptCase {
    param([string]$Phase, [bool]$ExpectFinal)
    $caseRoot = Join-Path $root ('interrupt-' + $Phase.ToLowerInvariant())
    $staging = Join-Path $caseRoot 'staging'
    $final = Join-Path $caseRoot 'final.zip'
    $handshake = Join-Path $caseRoot 'handshake.txt'
    [void][System.IO.Directory]::CreateDirectory($staging)
    [System.IO.File]::WriteAllText((Join-Path $staging 'payload.txt'), 'synthetic', $utf8)
    $eventName = 'Local\FlashGatePublicationFixture-' + [guid]::NewGuid().ToString('N')
    $created = $false
    $waitHandle = [System.Threading.EventWaitHandle]::new($false, [System.Threading.EventResetMode]::ManualReset, $eventName, [ref]$created)
    $watcher = [System.IO.FileSystemWatcher]::new($caseRoot, 'handshake.txt')
    $watcher.EnableRaisingEvents = $true
    $process = $null
    try {
        $arguments = @(
            '-NoLogo','-NoProfile','-File',(Join-Path $PSScriptRoot 'testdata/Invoke-GovernanceHandoffPublicationFixtureChild.ps1'),
            '-ModulePath',(Join-Path $PSScriptRoot 'GovernanceHandoffPublication.psm1'),
            '-StagingDirectory',$staging,'-FinalPath',$final,'-HandshakePath',$handshake,
            '-WaitHandleName',$eventName,'-InterruptPhase',$Phase
        )
        $process = Start-Process -FilePath (Get-Process -Id $PID).Path -ArgumentList $arguments `
            -PassThru -WindowStyle Hidden
        $change = $watcher.WaitForChanged([System.IO.WatcherChangeTypes]::Created, 15000)
        if ($change.TimedOut -or -not (Test-Path -LiteralPath $handshake -PathType Leaf)) {
            throw "No deterministic handshake for $Phase."
        }
        Add-Case ("GHP-$Phase-HANDSHAKE") $true
        if ($ExpectFinal) {
            Add-Case ("GHP-$Phase-FINAL-VALID") ((Test-Path -LiteralPath $final -PathType Leaf) -and ({ Test-ZipCandidate $final; $true }.Invoke()))
        }
        else {
            Add-Case ("GHP-$Phase-FINAL-ABSENT") (-not (Test-Path -LiteralPath $final))
        }
        Stop-Process -Id $process.Id -Force
        $process.WaitForExit()
        Add-Case ("GHP-$Phase-POST-TERMINATION") ($(if($ExpectFinal){Test-Path -LiteralPath $final -PathType Leaf}else{-not(Test-Path -LiteralPath $final)}))
    }
    finally {
        if ($null -ne $process -and -not $process.HasExited) { Stop-Process -Id $process.Id -Force }
        $watcher.Dispose()
        $waitHandle.Dispose()
    }
}

if ([string]::IsNullOrWhiteSpace($ExpectedExecutionInputBindingPath)) {
    $parentBinding = New-IndependentExecutionInputBinding
    $bindingContract = [ordered]@{
        schemaVersion = 1
        matrixId = $matrixId
        executionInputBinding = $parentBinding
    }
    $bindingParent = if ([string]::IsNullOrWhiteSpace($ResultPath)) {
        [System.IO.Path]::GetTempPath()
    }
    else { Split-Path -Parent ([System.IO.Path]::GetFullPath($ResultPath)) }
    [void][System.IO.Directory]::CreateDirectory($bindingParent)
    $bindingPath = Join-Path $bindingParent ('publication-expected-input-' + [guid]::NewGuid().ToString('N') + '.json')
    $childResultPath = if ([string]::IsNullOrWhiteSpace($ResultPath)) {
        Join-Path $bindingParent ('publication-result-' + [guid]::NewGuid().ToString('N') + '.json')
    }
    else { [System.IO.Path]::GetFullPath($ResultPath) }
    $temporaryChildResult = [string]::IsNullOrWhiteSpace($ResultPath)
    try {
        [System.IO.File]::WriteAllText($bindingPath, (($bindingContract | ConvertTo-Json -Depth 20) + "`n"), $utf8)
        $bindingSha256 = Get-IndependentFileSha256 -LiteralPath $bindingPath
        Invoke-BindingPhase 'PARENT_BINDING_CAPTURED'
        $startInfo = [System.Diagnostics.ProcessStartInfo]::new()
        $startInfo.FileName = (Get-Process -Id $PID).Path
        $startInfo.UseShellExecute = $false
        $startInfo.RedirectStandardOutput = $true
        $startInfo.RedirectStandardError = $true
        foreach ($argument in @(
                '-NoLogo', '-NoProfile', '-File', $PSCommandPath,
                '-ResultPath', $childResultPath,
                '-ExpectedExecutionInputBindingPath', $bindingPath,
                '-ExpectedExecutionInputBindingSha256', $bindingSha256
            )) { [void]$startInfo.ArgumentList.Add($argument) }
        if (-not [string]::IsNullOrWhiteSpace($PhaseHandshakeDirectory)) {
            [void]$startInfo.ArgumentList.Add('-PhaseHandshakeDirectory')
            [void]$startInfo.ArgumentList.Add($PhaseHandshakeDirectory)
        }
        if (-not [string]::IsNullOrWhiteSpace($PhaseContinueDirectory)) {
            [void]$startInfo.ArgumentList.Add('-PhaseContinueDirectory')
            [void]$startInfo.ArgumentList.Add($PhaseContinueDirectory)
        }
        if (-not [string]::IsNullOrWhiteSpace($PausePhase)) {
            [void]$startInfo.ArgumentList.Add('-PausePhase')
            [void]$startInfo.ArgumentList.Add($PausePhase)
        }
        $child = [System.Diagnostics.Process]::new()
        $child.StartInfo = $startInfo
        try {
            [void]$child.Start()
            $stdoutTask = $child.StandardOutput.ReadToEndAsync()
            $stderrTask = $child.StandardError.ReadToEndAsync()
            $child.WaitForExit()
            $stdout = $stdoutTask.GetAwaiter().GetResult()
            $stderr = $stderrTask.GetAwaiter().GetResult()
            if (-not [string]::IsNullOrWhiteSpace($stdout)) { $stdout.TrimEnd() }
            if (-not [string]::IsNullOrWhiteSpace($stderr)) { [Console]::Error.Write($stderr) }
            $childExitCode = $child.ExitCode
        }
        finally { $child.Dispose() }
    }
    finally {
        if (Test-Path -LiteralPath $bindingPath -PathType Leaf) { Remove-Item -LiteralPath $bindingPath -Force }
        if ($temporaryChildResult -and (Test-Path -LiteralPath $childResultPath -PathType Leaf)) {
            Remove-Item -LiteralPath $childResultPath -Force
        }
    }
    exit $childExitCode
}

try {
    $expectedContract = Read-ExpectedExecutionInputBinding -LiteralPath $ExpectedExecutionInputBindingPath
    $executionInputBinding = $expectedContract.executionInputBinding
    [void](Assert-ExpectedExecutionInputBinding -LiteralPath $ExpectedExecutionInputBindingPath)
    Invoke-BindingPhase 'RUNNER_INITIAL_BINDING_VERIFIED'
    [void](Assert-ExpectedExecutionInputBinding -LiteralPath $ExpectedExecutionInputBindingPath)
    Import-Module (Join-Path $PSScriptRoot 'GovernanceValidationOrchestration.psm1') -Force
    [void](Assert-ExpectedExecutionInputBinding -LiteralPath $ExpectedExecutionInputBindingPath)
    Import-Module (Join-Path $PSScriptRoot 'GovernanceHandoffPublication.psm1') -Force
    Invoke-BindingPhase 'RUNNER_IMPORTS_COMPLETED'
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $root = Join-Path ([System.IO.Path]::GetTempPath()) ('flashgate-publication-product-' + [guid]::NewGuid().ToString('N'))
    $staging = Join-Path $root 'staging'
    [void][System.IO.Directory]::CreateDirectory($staging)
    [System.IO.File]::WriteAllText((Join-Path $staging 'payload.txt'), 'synthetic', $utf8)

    [void](Assert-ExpectedExecutionInputBinding -LiteralPath $ExpectedExecutionInputBindingPath)
    $caseExecutionAuthorized = $true
    $normalFinal = Join-Path $root 'normal.zip'
    $attempt = 0
    $normalCandidateNames = [System.Collections.Generic.List[string]]::new()
    $validator = { param($candidate) if(Test-Path -LiteralPath $normalFinal){throw 'Final path existed during candidate validation.'}; Test-ZipCandidate $candidate }
    $normal = Publish-GovernanceHandoffPackage $staging $normalFinal $validator ([ref]$attempt) -PhaseObserver {
        param($state)
        if ($state.Phase -ceq 'BEFORE_CREATE') {
            $normalCandidateNames.Add([System.IO.Path]::GetFileName([string]$state.CandidatePath))
        }
    }
    Add-Case 'GHP-NORMAL-PUBLICATION-PASS' (
        $attempt -eq 1 -and
        $normal.SerializationCount -eq 1 -and
        (Test-Path -LiteralPath $normalFinal) -and
        $normalCandidateNames.Count -eq 1 -and
        -not $normalCandidateNames[0].StartsWith('.', [System.StringComparison]::Ordinal) -and
        $normalCandidateNames[0].EndsWith('.normal.zip.pending', [System.StringComparison]::Ordinal)
    ) 'candidate basename is provider-visible on Windows and Linux'

    $validatorFinal = Join-Path $root 'validator-failure.zip'; $attempt = 0
    $failed = Invoke-ExpectedFailure { Publish-GovernanceHandoffPackage $staging $validatorFinal { throw 'synthetic validator failure' } ([ref]$attempt) | Out-Null }
    Add-Case 'GHP-VALIDATOR-FAILURE-FINAL-ABSENT' ($failed -and $attempt -eq 1 -and -not(Test-Path $validatorFinal))

    $writeFinal = Join-Path $root 'write-failure.zip'; $attempt = 0
    $failed = Invoke-ExpectedFailure { Publish-GovernanceHandoffPackage $staging $writeFinal { param($p) } ([ref]$attempt) -CandidateSerializer { param($s,$stream) throw 'synthetic write failure' } | Out-Null }
    Add-Case 'GHP-WRITE-FAILURE-FINAL-ABSENT' ($failed -and $attempt -eq 1 -and -not(Test-Path $writeFinal))

    $createFinal = Join-Path $root 'create-failure.zip'; $attempt = 0
    $failed = Invoke-ExpectedFailure { Publish-GovernanceHandoffPackage $staging $createFinal { param($p) } ([ref]$attempt) -PhaseObserver { param($state) if($state.Phase -ceq 'BEFORE_CREATE'){[System.IO.File]::WriteAllText($state.CandidatePath,'occupied',$utf8)} } | Out-Null }
    Add-Case 'GHP-CREATENEW-FAILURE-COUNTED-ONCE' ($failed -and $attempt -eq 1 -and -not(Test-Path $createFinal))

    $ioFinal = Join-Path $root 'publication-io-failure.zip'; $attempt = 0
    $failed = Invoke-ExpectedFailure { Publish-GovernanceHandoffPackage $staging $ioFinal ${function:Test-ZipCandidate} ([ref]$attempt) -PublicationOperation { throw 'synthetic publication I/O failure' } | Out-Null }
    Add-Case 'GHP-PUBLICATION-IO-FAILURE-NO-RETRY' ($failed -and $attempt -eq 1 -and -not(Test-Path $ioFinal))

    $raceFinal = Join-Path $root 'race.zip'; $attempt = 0
    $failed = Invoke-ExpectedFailure { Publish-GovernanceHandoffPackage $staging $raceFinal ${function:Test-ZipCandidate} ([ref]$attempt) -PhaseObserver { param($state) if($state.Phase -ceq 'CANDIDATE_VALIDATED'){[System.IO.File]::WriteAllText($state.FinalPath,'race',$utf8)} } | Out-Null }
    Add-Case 'GHP-FINAL-PATH-RACE-NO-OVERWRITE' ($failed -and $attempt -eq 1 -and ([System.IO.File]::ReadAllText($raceFinal,$utf8) -ceq 'race'))

    $wrongParent = Join-Path $root 'other'; [void][System.IO.Directory]::CreateDirectory($wrongParent); $attempt = 0
    $failed = Invoke-ExpectedFailure { Publish-GovernanceHandoffPackage $staging (Join-Path $root 'wrong-parent.zip') ${function:Test-ZipCandidate} ([ref]$attempt) -CandidateDirectory $wrongParent | Out-Null }
    Add-Case 'GHP-WRONG-CANDIDATE-DIRECTORY-PREWRITE-REJECTED' ($failed -and $attempt -eq 0)

    foreach ($driftMode in @('MUTATE','REPLACE')) {
        $driftRoot = Join-Path $root ('drift-' + $driftMode.ToLowerInvariant())
        [void][System.IO.Directory]::CreateDirectory($driftRoot)
        $driftFinal = Join-Path $driftRoot 'final.zip'; $attempt = 0
        $failed = Invoke-ExpectedFailure {
            Publish-GovernanceHandoffPackage $staging $driftFinal ${function:Test-ZipCandidate} `
                ([ref]$attempt) -PhaseObserver {
                    param($state)
                    if ($state.Phase -ceq 'CANDIDATE_VALIDATED') {
                        Invoke-CandidateDriftProcess $state.CandidatePath $driftMode $driftRoot
                    }
                } | Out-Null
        }
        Add-Case ("GHP-CANDIDATE-$driftMode-AFTER-VALIDATION-REJECTED") `
            ($failed -and $attempt -eq 1 -and -not(Test-Path -LiteralPath $driftFinal))
    }

    $boundDriftRoot = Join-Path $root 'drift-after-identity-binding'
    [void][System.IO.Directory]::CreateDirectory($boundDriftRoot)
    $boundDriftFinal = Join-Path $boundDriftRoot 'final.zip'; $attempt = 0
    $failed = Invoke-ExpectedFailure {
        Publish-GovernanceHandoffPackage $staging $boundDriftFinal {
            param($candidate, $boundIdentity)
            Test-ZipCandidate $candidate
            if ([string]::IsNullOrWhiteSpace([string]$boundIdentity.Sha256)) { throw 'Candidate identity was not bound.' }
            Invoke-CandidateDriftProcess $candidate 'MUTATE' $boundDriftRoot
        } ([ref]$attempt) | Out-Null
    }
    Add-Case 'GHP-CANDIDATE-DRIFT-AFTER-IDENTITY-BINDING-REJECTED' `
        ($failed -and $attempt -eq 1 -and -not(Test-Path -LiteralPath $boundDriftFinal))

    Add-Case 'GHP-AUTOMATIC-PACKAGE-RETRY-COUNT-ZERO' ($attempt -eq 1) 'one attempt; zero retry'

    Invoke-InterruptCase 'CANDIDATE_VALIDATION_STARTED' $false
    Invoke-InterruptCase 'CANDIDATE_VALIDATED' $false
    Invoke-InterruptCase 'PUBLISHED' $true
    $status = 'PASS'
}
catch { $failureMessage = $_.Exception.Message }
finally {
    if ($null -ne $root -and (Test-Path -LiteralPath $root)) { Remove-Item -LiteralPath $root -Recurse -Force }
    $result = [ordered]@{ schemaVersion=2; matrixId=$matrixId; executionInputBinding=$executionInputBinding; status=$status; selected=$results.Count; passed=@($results|Where-Object result -ceq 'PASS').Count; failed=@($results|Where-Object result -ceq 'FAIL').Count; results=@($results); failureMessage=$failureMessage }
    if(-not[string]::IsNullOrWhiteSpace($ResultPath)){[System.IO.File]::WriteAllText([System.IO.Path]::GetFullPath($ResultPath),(($result|ConvertTo-Json -Depth 20)+"`n"),$utf8)}
    [pscustomobject]$result | Format-List
}
if($status -ceq 'PASS'){exit 0}
exit 1
