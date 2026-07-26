[CmdletBinding()]
param(
    [string] $RealWindowsX64Binary,
    [string] $RealWindowsArm64Binary,
    [string] $ExpectedProductVersion,
    [string] $ExpectedFileVersion,
    [ValidatePattern('^[0-9a-f]{40}$')]
    [string] $ExpectedCommit,
    [string] $ExpectedSourceTime,
    [ValidateSet('', 'true', 'false')]
    [string] $ExpectedModified = '',
    [ValidateRange(1970, 9999)]
    [int] $ExpectedCopyrightYear = 1970
)

$ErrorActionPreference = 'Stop'
$RootPath = Split-Path -Parent $PSScriptRoot
$ProcessLibrary = Join-Path $PSScriptRoot 'VerifierProcess.ps1'
$ProcessHelper = Join-Path `
    $PSScriptRoot `
    'testdata\verifier-process-helper.ps1'
$Verifier = Join-Path $PSScriptRoot 'Test-WindowsMetadata.ps1'
$Failures = [System.Collections.Generic.List[string]]::new()
$Cases = [System.Collections.Generic.List[string]]::new()
$BarrierFiles = [System.Collections.Generic.List[string]]::new()
$BarrierDirectories = [System.Collections.Generic.List[string]]::new()
$ExitCode = 1
$TempRoot = Join-Path `
    ([IO.Path]::GetTempPath()) `
    ('flashgate-verifier-contract-' + [Guid]::NewGuid().ToString('N'))
$PreviousTestMode = $env:FLASHGATE_VERIFIER_TEST_MODE

function Test-Condition {
    param(
        [Parameter(Mandatory)]
        [bool] $Condition,

        [Parameter(Mandatory)]
        [string] $Name
    )

    $Cases.Add($Name)
    if (-not $Condition) {
        $Failures.Add($Name)
    }
}

function Invoke-ControlledHelper {
    param(
        [Parameter(Mandatory)]
        [string] $Mode,

        [int] $TimeoutMilliseconds = 10000,

        [int] $MaximumCharacters = 4096,

        [string] $MarkerPath,

        [string] $ReadyPath
    )

    $Arguments = @(
        '-NoProfile'
        '-File'
        $ProcessHelper
        '-Mode'
        $Mode
    )
    if (-not [string]::IsNullOrWhiteSpace($MarkerPath)) {
        $Arguments += @('-MarkerPath', $MarkerPath)
    }
    if (-not [string]::IsNullOrWhiteSpace($ReadyPath)) {
        $Arguments += @('-ReadyPath', $ReadyPath)
    }
    return Invoke-FlashGateBoundedProcess `
        -FilePath (Join-Path $PSHOME 'pwsh.exe') `
        -Arguments $Arguments `
        -WorkingDirectory $RootPath `
        -TimeoutMilliseconds $TimeoutMilliseconds `
        -MaximumCharactersPerStream $MaximumCharacters
}

function Wait-ForFile {
    param(
        [Parameter(Mandatory)]
        [string] $Path,

        [Parameter(Mandatory)]
        [int] $TimeoutMilliseconds
    )

    $Deadline = [Diagnostics.Stopwatch]::StartNew()
    while ($Deadline.ElapsedMilliseconds -lt $TimeoutMilliseconds) {
        if (Test-Path -LiteralPath $Path -PathType Leaf) {
            return $true
        }
        Start-Sleep -Milliseconds 50
    }
    return (Test-Path -LiteralPath $Path -PathType Leaf)
}

function Wait-ForProcessExit {
    param(
        [Parameter(Mandatory)]
        [int] $ProcessId,

        [Parameter(Mandatory)]
        [int] $TimeoutMilliseconds
    )

    $Deadline = [Diagnostics.Stopwatch]::StartNew()
    while ($Deadline.ElapsedMilliseconds -lt $TimeoutMilliseconds) {
        if ($null -eq (Get-Process -Id $ProcessId -ErrorAction SilentlyContinue)) {
            return $true
        }
        Start-Sleep -Milliseconds 50
    }
    return (
        $null -eq (Get-Process -Id $ProcessId -ErrorAction SilentlyContinue)
    )
}

function New-ControlledProcessResult {
    param(
        [bool] $Attempted = $true,
        [string] $Status = 'FAIL',
        [Nullable[int]] $ExitCode = $null,
        [bool] $TimedOut = $false,
        [bool] $OutputLimitExceeded = $false,
        [string] $Stdout = '',
        [string] $Stderr = '',
        [string] $FailureReason = ''
    )

    return [pscustomobject]@{
        Attempted           = $Attempted
        Status              = $Status
        ExitCode            = $ExitCode
        TimedOut            = $TimedOut
        OutputLimitExceeded = $OutputLimitExceeded
        Stdout              = $Stdout
        Stderr              = $Stderr
        FailureReason       = $FailureReason
    }
}

function Invoke-ReadyBarrierTest {
    param(
        [Parameter(Mandatory)]
        [string] $Name,

        [Parameter(Mandatory)]
        [ValidateSet(
            'valid',
            'missing',
            'late',
            'invalid',
            'leading-space',
            'trailing-space',
            'newline',
            'plus',
            'leading-zero',
            'space-after-colon',
            'tab-after-colon',
            'extra-record',
            'suffix',
            'empty',
            'zero',
            'negative',
            'oversize',
            'dead-pid',
            'foreign-pid',
            'exit-before-release',
            'exit-after-release',
            'exit-after-released',
            'exit-after-activated',
            'exit-after-final-check',
            'child-identity-mismatch-after-ready',
            'parent-identity-mismatch-after-ready',
            'parent-exit-before-release',
            'parent-exit-after-released',
            'parent-exit-before-timeout'
        )]
        [string] $ReadyMode,

        [ValidateSet('ready', 'released', 'activated')]
        [string] $RecordTarget = 'ready',

        [ValidateSet(
            'valid',
            'invalid-encoding',
            'overflow',
            'exact-64',
            'exact-65',
            'leading-space',
            'trailing-space',
            'plus',
            'leading-zero',
            'space-after-colon',
            'tab-after-colon',
            'newline',
            'extra-record',
            'suffix',
            'empty',
            'nonnumeric',
            'zero',
            'negative',
            'wrong-pid'
        )]
        [string] $RecordVariant = 'valid',

        [int] $ReadyTimeoutMilliseconds = 10000,

        [int] $ExecutionTimeoutMilliseconds = 300,

        [switch] $ReleaseWriteFailure
    )

    $MarkerPath = Join-Path $TempRoot "$Name-survivor.txt"
    $ReadyPath = Join-Path $TempRoot "$Name-ready.txt"
    $ReleasePath = Join-Path $TempRoot "$Name-release.txt"
    $ReleasedPath = Join-Path $TempRoot "$Name-released.txt"
    $ChildPidPath = Join-Path $TempRoot "$Name-child.txt"
    $TimeoutActivationProbePath =
        Join-Path $TempRoot "$Name-timeout-activation.txt"
    $TimeoutActivatedPath =
        Join-Path $TempRoot "$Name-timeout-activated.txt"
    $TimeoutFinalCheckPath =
        Join-Path $TempRoot "$Name-timeout-final-check.txt"
    foreach ($Path in @(
        $MarkerPath
        $ReadyPath
        "$ReadyPath.tmp"
        $ReleasePath
        $ReleasedPath
        "$ReleasedPath.tmp"
        $ChildPidPath
        "$ChildPidPath.tmp"
        $TimeoutActivationProbePath
        $TimeoutActivatedPath
        "$TimeoutActivatedPath.tmp"
        $TimeoutFinalCheckPath
    )) {
        $BarrierFiles.Add($Path)
    }
    if ($ReleaseWriteFailure) {
        [IO.Directory]::CreateDirectory($ReleasePath) | Out-Null
        $BarrierDirectories.Add($ReleasePath)
    }
    $Arguments = [string[]]@(
        '-NoProfile'
        '-File'
        $ProcessHelper
        '-Mode'
        'spawn-child'
        '-MarkerPath'
        $MarkerPath
        '-ReadyPath'
        $ReadyPath
        '-ReleasePath'
        $ReleasePath
        '-ReleasedPath'
        $ReleasedPath
        '-ChildPidPath'
        $ChildPidPath
        '-TimeoutActivationProbePath'
        $TimeoutActivationProbePath
        '-TimeoutActivatedPath'
        $TimeoutActivatedPath
        '-TimeoutFinalCheckPath'
        $TimeoutFinalCheckPath
        '-ReadyMode'
        $ReadyMode
        '-RecordTarget'
        $RecordTarget
        '-RecordVariant'
        $RecordVariant
        '-ForeignPid'
        [string]$PID
    )
    $Method = [FlashGate.Validation.BoundedProcessRunner].GetMethod(
        'RunReadyBarrierTestOnly',
        [Reflection.BindingFlags]'Static,NonPublic'
    )
    if ($null -eq $Method) {
        throw 'The internal READY-barrier test method was not found.'
    }
    $MethodArguments = [object[]]::new(14)
    $MethodArguments[0] = [string](Join-Path $PSHOME 'pwsh.exe')
    $MethodArguments[1] = $Arguments
    $MethodArguments[2] = [string]$RootPath
    $MethodArguments[3] = [int]$ExecutionTimeoutMilliseconds
    $MethodArguments[4] = [int]4096
    $MethodArguments[5] = [int]2000
    $MethodArguments[6] = [string]$ReadyPath
    $MethodArguments[7] = [string]$ReleasePath
    $MethodArguments[8] = [string]$ReleasedPath
    $MethodArguments[9] = [string]$TimeoutActivationProbePath
    $MethodArguments[10] = [string]$TimeoutActivatedPath
    $MethodArguments[11] = [string]$TimeoutFinalCheckPath
    $MethodArguments[12] = [int]$ReadyTimeoutMilliseconds
    $MethodArguments[13] = [string]$ReadyMode
    $InvocationClock = [Diagnostics.Stopwatch]::StartNew()
    $Barrier = $Method.Invoke($null, $MethodArguments)
    $InvocationClock.Stop()
    $ControlledChildPid = 0
    if (Test-Path -LiteralPath $ChildPidPath -PathType Leaf) {
        $ChildRecord = [IO.File]::ReadAllText($ChildPidPath)
        if ($ChildRecord -cmatch '\ACHILD:([1-9][0-9]*)\z') {
            $ControlledChildPid = [int]$Matches[1]
        }
    }
    return [pscustomobject]@{
        Barrier = $Barrier
        MarkerPath = $MarkerPath
        ReadyPath = $ReadyPath
        ReleasePath = $ReleasePath
        ReleasedPath = $ReleasedPath
        ChildPidPath = $ChildPidPath
        TimeoutActivationProbePath = $TimeoutActivationProbePath
        TimeoutActivatedPath = $TimeoutActivatedPath
        TimeoutFinalCheckPath = $TimeoutFinalCheckPath
        ControlledChildPid = $ControlledChildPid
        ElapsedMilliseconds = $InvocationClock.ElapsedMilliseconds
    }
}

try {
    [IO.Directory]::CreateDirectory($TempRoot) | Out-Null
    . $ProcessLibrary

    $Positive = Invoke-ControlledHelper -Mode valid-version
    Test-Condition `
        ($Positive.Attempted -and $Positive.Status -ceq 'PASS') `
        'native positive bounded execution'

    $NonzeroVersion = Invoke-ControlledHelper -Mode nonzero
    Test-Condition `
        (
            $NonzeroVersion.Status -ceq 'FAIL' -and
            $NonzeroVersion.ExitCode -eq 7 -and
            $NonzeroVersion.FailureReason -ceq 'NonZeroExit'
        ) `
        'invalid version exit code'

    $NonzeroHelp = Invoke-ControlledHelper -Mode nonzero
    Test-Condition `
        (
            (ConvertTo-FlashGateExecutionState `
                -Attempted $NonzeroHelp.Attempted `
                -ProcessStatus $NonzeroHelp.Status) -ceq 'FAIL'
        ) `
        'invalid help exit becomes FAIL'

    $EmptyHelp = Invoke-ControlledHelper -Mode empty
    Test-Condition `
        (@(Get-FlashGateMissingHelpLines -Output $EmptyHelp.Stdout).Count -eq 8) `
        'empty help is incomplete'

    $PartialHelp = Invoke-ControlledHelper -Mode partial-help
    Test-Condition `
        (@(Get-FlashGateMissingHelpLines -Output $PartialHelp.Stdout).Count -gt 0) `
        'partial help is incomplete'

    $Timeout = Invoke-ControlledHelper `
        -Mode hang `
        -TimeoutMilliseconds 300
    Test-Condition `
        (
            $Timeout.Status -ceq 'FAIL' -and
            $Timeout.TimedOut -and
            $Timeout.FailureReason -ceq 'Timeout'
        ) `
        'timeout fails closed'

    $StdoutLimit = Invoke-ControlledHelper `
        -Mode stdout-flood `
        -MaximumCharacters 1024
    Test-Condition `
        (
            $StdoutLimit.Status -ceq 'FAIL' -and
            $StdoutLimit.OutputLimitExceeded -and
            $StdoutLimit.Stdout.Length -eq 1024
        ) `
        'stdout limit fails closed'

    $StderrLimit = Invoke-ControlledHelper `
        -Mode stderr-flood `
        -MaximumCharacters 1024
    Test-Condition `
        (
            $StderrLimit.Status -ceq 'FAIL' -and
            $StderrLimit.OutputLimitExceeded -and
            $StderrLimit.Stderr.Length -eq 1024
        ) `
        'stderr limit fails without deadlock'

    foreach ($Iteration in 1..3) {
        $ReadyCase = Invoke-ReadyBarrierTest `
            -Name "ready-$Iteration" `
            -ReadyMode valid
        $ReadyBarrier = $ReadyCase.Barrier
        $ReadyProcess = $ReadyBarrier.ProcessResult
        $ReadyChildExited = (
            $ReadyBarrier.ReadyProcessId -gt 0 -and
            (Wait-ForProcessExit `
                -ProcessId $ReadyBarrier.ReadyProcessId `
                -TimeoutMilliseconds 3000)
        )
        $MarkerDeadline = [Diagnostics.Stopwatch]::StartNew()
        $MarkerAbsent = $true
        while ($MarkerDeadline.ElapsedMilliseconds -lt 3000) {
            if (
                Test-Path `
                    -LiteralPath $ReadyCase.MarkerPath `
                    -PathType Leaf
            ) {
                $MarkerAbsent = $false
                break
            }
            Start-Sleep -Milliseconds 50
        }
        $ReadyCondition = (
                $ReadyProcess.Attempted -and
                $ReadyProcess.Status -ceq 'FAIL' -and
                $ReadyProcess.TimedOut -and
                $ReadyProcess.FailureReason -ceq 'Timeout' -and
                $ReadyBarrier.ReadyObserved -and
                $ReadyBarrier.ReadyProcessId -gt 0 -and
                $ReadyBarrier.ReleasedObserved -and
                $ReadyBarrier.ReleasedProcessId -eq
                    $ReadyBarrier.ReadyProcessId -and
                $ReadyBarrier.ActivatedObserved -and
                $ReadyBarrier.ActivatedProcessId -eq
                    $ReadyBarrier.ReadyProcessId -and
                $ReadyCase.ControlledChildPid -eq
                    $ReadyBarrier.ReadyProcessId -and
                $ReadyBarrier.ChildAliveBeforeTimeout -and
                $ReadyBarrier.TimeoutStartedAfterReady -and
                $ReadyBarrier.ReadyElapsedMilliseconds -lt 10000 -and
                (Test-Path -LiteralPath $ReadyCase.ReleasePath -PathType Leaf) -and
                (Test-Path -LiteralPath $ReadyCase.ReleasedPath -PathType Leaf) -and
                (Test-Path `
                    -LiteralPath $ReadyCase.TimeoutActivationProbePath `
                    -PathType Leaf) -and
                (Test-Path `
                    -LiteralPath $ReadyCase.TimeoutActivatedPath `
                    -PathType Leaf) -and
                (Test-Path `
                    -LiteralPath $ReadyCase.TimeoutFinalCheckPath `
                    -PathType Leaf) -and
                $ReadyCase.ElapsedMilliseconds -lt 15000 -and
                $ReadyChildExited -and
                $MarkerAbsent
            )
        $ReadyName =
            "READY precedes timeout and child cleanup iteration $Iteration"
        if (-not $ReadyCondition) {
            $ReadyName += (
                " [Status=$($ReadyProcess.Status);" +
                "TimedOut=$($ReadyProcess.TimedOut);" +
                "Reason=$($ReadyProcess.FailureReason);" +
                "Ready=$($ReadyBarrier.ReadyObserved);" +
                "Pid=$($ReadyBarrier.ReadyProcessId);" +
                "Alive=$($ReadyBarrier.ChildAliveBeforeTimeout);" +
                "AfterReady=$($ReadyBarrier.TimeoutStartedAfterReady);" +
                "ReadyMs=$($ReadyBarrier.ReadyElapsedMilliseconds);" +
                "Release=$(Test-Path -LiteralPath $ReadyCase.ReleasePath);" +
                "Released=$($ReadyBarrier.ReleasedObserved);" +
                "Exited=$ReadyChildExited;MarkerAbsent=$MarkerAbsent]"
            )
        }
        Test-Condition $ReadyCondition $ReadyName
    }

    $NegativeSpecifications = [System.Collections.Generic.List[object]]::new()
    foreach ($RecordTarget in @('ready', 'released', 'activated')) {
        foreach ($RecordVariant in @(
            'invalid-encoding'
            'overflow'
            'exact-64'
            'exact-65'
            'leading-space'
            'trailing-space'
            'plus'
            'leading-zero'
            'space-after-colon'
            'tab-after-colon'
            'newline'
            'extra-record'
            'suffix'
            'empty'
            'nonnumeric'
            'zero'
            'negative'
        )) {
            $RecordReasonPrefix = switch ($RecordTarget) {
                'ready' { 'Ready' }
                'released' { 'Released' }
                'activated' { 'Activated' }
            }
            $NegativeSpecifications.Add([pscustomobject]@{
                Name = "$RecordTarget-record-$RecordVariant"
                Mode = 'valid'
                RecordTarget = $RecordTarget
                RecordVariant = $RecordVariant
                ExpectedReason = "${RecordReasonPrefix}PidInvalid"
                ReadyTimeout = 15000
                ReleaseWriteFailure = $false
            })
        }
    }
    foreach ($WrongRecordTarget in @('released', 'activated')) {
        $WrongReasonPrefix = if ($WrongRecordTarget -ceq 'released') {
            'Released'
        }
        else {
            'Activated'
        }
        $NegativeSpecifications.Add([pscustomobject]@{
            Name = "wrong-$WrongRecordTarget-pid"
            Mode = 'valid'
            RecordTarget = $WrongRecordTarget
            RecordVariant = 'wrong-pid'
            ExpectedReason = "${WrongReasonPrefix}PidInvalid"
            ReadyTimeout = 15000
            ReleaseWriteFailure = $false
        })
    }
    foreach ($Specification in @(
        [pscustomobject]@{
            Name = 'ready-missing'
            Mode = 'missing'
            ExpectedReason = 'ReadyTimeout'
            RecordTarget = 'ready'
            RecordVariant = 'valid'
            ReadyTimeout = 2000
            ReleaseWriteFailure = $false
        }
        [pscustomobject]@{
            Name = 'ready-late'
            Mode = 'late'
            ExpectedReason = 'ReadyTimeout'
            RecordTarget = 'ready'
            RecordVariant = 'valid'
            ReadyTimeout = 2000
            ReleaseWriteFailure = $false
        }
        [pscustomobject]@{
            Name = 'ready-dead-pid'
            Mode = 'dead-pid'
            ExpectedReason = 'ReadyChildNotAlive'
            RecordTarget = 'ready'
            RecordVariant = 'valid'
            ReadyTimeout = 15000
            ReleaseWriteFailure = $false
        }
        [pscustomobject]@{
            Name = 'ready-foreign-pid'
            Mode = 'foreign-pid'
            ExpectedReason = 'ReadyChildIdentityMismatch'
            RecordTarget = 'ready'
            RecordVariant = 'valid'
            ReadyTimeout = 15000
            ReleaseWriteFailure = $false
        }
        [pscustomobject]@{
            Name = 'release-write-failure'
            Mode = 'valid'
            ExpectedReason = 'ReleaseWriteFailure'
            RecordTarget = 'ready'
            RecordVariant = 'valid'
            ReadyTimeout = 15000
            ReleaseWriteFailure = $true
        }
        [pscustomobject]@{
            Name = 'child-identity-mismatch-after-ready'
            Mode = 'child-identity-mismatch-after-ready'
            RecordTarget = 'ready'
            RecordVariant = 'valid'
            ExpectedReason = 'ReadyChildIdentityMismatch'
            ReadyTimeout = 15000
            ReleaseWriteFailure = $false
        }
        [pscustomobject]@{
            Name = 'parent-identity-mismatch-after-ready'
            Mode = 'parent-identity-mismatch-after-ready'
            RecordTarget = 'ready'
            RecordVariant = 'valid'
            ExpectedReason = 'ReadyParentIdentityMismatch'
            ReadyTimeout = 15000
            ReleaseWriteFailure = $false
        }
    )) {
        $NegativeSpecifications.Add($Specification)
    }
    foreach ($RaceIteration in 1..3) {
        foreach ($RaceMode in @(
            [pscustomobject]@{
                Mode = 'exit-before-release'
                ExpectedReason = 'ReadyChildNotAlive'
            }
            [pscustomobject]@{
                Mode = 'exit-after-release'
                ExpectedReason = 'ReadyChildNotAliveAfterRelease'
            }
            [pscustomobject]@{
                Mode = 'exit-after-released'
                ExpectedReason = 'ReadyChildNotAliveBeforeTimeout'
            }
            [pscustomobject]@{
                Mode = 'exit-after-activated'
                ExpectedReason = 'ReadyChildNotAliveBeforeTimeout'
            }
            [pscustomobject]@{
                Mode = 'exit-after-final-check'
                ExpectedReason = 'ReadyChildNotAliveBeforeTimeout'
            }
            [pscustomobject]@{
                Mode = 'parent-exit-before-release'
                ExpectedReason = 'ReadyParentNotAlive'
            }
            [pscustomobject]@{
                Mode = 'parent-exit-after-released'
                ExpectedReason = 'ReadyParentNotAlive'
            }
            [pscustomobject]@{
                Mode = 'parent-exit-before-timeout'
                ExpectedReason = 'ReadyParentNotAlive'
            }
        )) {
            $NegativeSpecifications.Add([pscustomobject]@{
                Name = "$($RaceMode.Mode)-$RaceIteration"
                Mode = $RaceMode.Mode
                ExpectedReason = $RaceMode.ExpectedReason
                RecordTarget = 'ready'
                RecordVariant = 'valid'
                ReadyTimeout = 15000
                ReleaseWriteFailure = $false
            })
        }
    }

    $NegativeResults = [System.Collections.Generic.List[object]]::new()
    foreach ($Specification in $NegativeSpecifications) {
        $InvokeArguments = @{
            Name = $Specification.Name
            ReadyMode = $Specification.Mode
            ReadyTimeoutMilliseconds = $Specification.ReadyTimeout
            RecordTarget = $Specification.RecordTarget
            RecordVariant = $Specification.RecordVariant
        }
        if ($Specification.ReleaseWriteFailure) {
            $InvokeArguments.ReleaseWriteFailure = $true
        }
        $BarrierCase = Invoke-ReadyBarrierTest @InvokeArguments
        $Barrier = $BarrierCase.Barrier
        $ProcessResult = $Barrier.ProcessResult
        $MaximumElapsedMilliseconds =
            $Specification.ReadyTimeout + 3000
        $ControlledChildExited = if (
            $BarrierCase.ControlledChildPid -gt 0
        ) {
            Wait-ForProcessExit `
                -ProcessId $BarrierCase.ControlledChildPid `
                -TimeoutMilliseconds 3000
        }
        else {
            (
                $Specification.Mode -in @('missing', 'late') -and
                -not $Barrier.ReadyObserved -and
                -not (Test-Path `
                    -LiteralPath $BarrierCase.ReleasePath `
                    -PathType Leaf) -and
                -not (Test-Path `
                    -LiteralPath $BarrierCase.ReleasedPath `
                    -PathType Leaf)
            )
        }
        $NegativeCondition = (
            $ProcessResult.Attempted -and
            $ProcessResult.Status -ceq 'FAIL' -and
            -not $ProcessResult.TimedOut -and
            $ProcessResult.FailureReason -ceq
                $Specification.ExpectedReason -and
            -not $Barrier.TimeoutStartedAfterReady -and
            $BarrierCase.ElapsedMilliseconds -lt
                $MaximumElapsedMilliseconds -and
            $ControlledChildExited
        )
        $NegativeName =
            "$($Specification.Name) fails bounded before timeout activation"
        if (-not $NegativeCondition) {
            $NegativeName += (
                " [Status=$($ProcessResult.Status);" +
                "TimedOut=$($ProcessResult.TimedOut);" +
                "Reason=$($ProcessResult.FailureReason);" +
                "Expected=$($Specification.ExpectedReason);" +
                "AfterReady=$($Barrier.TimeoutStartedAfterReady);" +
                "ElapsedMs=$($BarrierCase.ElapsedMilliseconds);" +
                "ChildPid=$($BarrierCase.ControlledChildPid);" +
                "ChildExited=$ControlledChildExited;" +
                "Stderr=$($ProcessResult.Stderr)]"
            )
        }
        Test-Condition `
            $NegativeCondition `
            $NegativeName
        $NegativeResults.Add([pscustomobject]@{
            Name = $Specification.Name
            MarkerPath = $BarrierCase.MarkerPath
        })
    }

    Start-Sleep -Milliseconds 2300
    foreach ($NegativeResult in $NegativeResults) {
        Test-Condition `
            (-not (Test-Path `
                -LiteralPath $NegativeResult.MarkerPath `
                -PathType Leaf)) `
            "$($NegativeResult.Name) leaves no survivor marker"
    }

    $TerminationArguments = [string[]]@(
        '-NoProfile'
        '-File'
        $ProcessHelper
        '-Mode'
        'termination-seam-target'
    )
    $TerminationClock = [Diagnostics.Stopwatch]::StartNew()
    $TerminationTestMethod =
        [FlashGate.Validation.BoundedProcessRunner].GetMethod(
            'RunTerminationFailureTestOnly',
            [Reflection.BindingFlags]'Static,NonPublic'
        )
    $TerminationTestArguments = [object[]]::new(6)
    $TerminationTestArguments[0] = [string](Join-Path $PSHOME 'pwsh.exe')
    $TerminationTestArguments[1] = [string[]]$TerminationArguments
    $TerminationTestArguments[2] = [string]$RootPath
    $TerminationTestArguments[3] = [int]200
    $TerminationTestArguments[4] = [int]1024
    $TerminationTestArguments[5] = [int]300
    $TerminationFailure = $TerminationTestMethod.Invoke(
        $null,
        $TerminationTestArguments
    )
    $TerminationClock.Stop()
    Test-Condition `
        (
            $TerminationFailure.Attempted -and
            $TerminationFailure.Status -ceq 'FAIL' -and
            $TerminationFailure.TimedOut -and
            $TerminationFailure.FailureReason -ceq 'TerminationFailed' -and
            $TerminationFailure.Stdout.Length -le 1024 -and
            $TerminationFailure.Stderr.Length -le 1024 -and
            $TerminationClock.ElapsedMilliseconds -lt 1500
        ) `
        'double kill failure returns within cleanup deadline'

    $StartFailure = Invoke-FlashGateBoundedProcess `
        -FilePath (Join-Path $TempRoot 'missing-command.exe') `
        -Arguments @('--help') `
        -WorkingDirectory $RootPath `
        -TimeoutMilliseconds 1000 `
        -MaximumCharactersPerStream 4096
    Test-Condition `
        (
            $StartFailure.Attempted -and
            $StartFailure.Status -ceq 'FAIL' -and
            $StartFailure.FailureReason -like 'StartOrProcessFailure:*'
        ) `
        'missing executable start fails closed'

    $X64Native = Get-FlashGateNativeExecutionDecision `
        -HostArchitecture x64 `
        -TargetArchitecture x64
    $X64Cross = Get-FlashGateNativeExecutionDecision `
        -HostArchitecture x64 `
        -TargetArchitecture arm64
    $ArmNative = Get-FlashGateNativeExecutionDecision `
        -HostArchitecture arm64 `
        -TargetArchitecture arm64
    $EmulatedX64Process = Get-FlashGateNativeExecutionDecision `
        -HostArchitecture arm64 `
        -TargetArchitecture x64
    $CrossWithCallerSkip = Get-FlashGateNativeExecutionDecision `
        -HostArchitecture x64 `
        -TargetArchitecture arm64 `
        -CallerRequestedSkip
    Test-Condition `
        $X64Native.NativeExecutionEligible `
        'x64 host and x64 target are native'
    Test-Condition `
        (
            -not $X64Cross.NativeExecutionEligible -and
            $X64Cross.SkipReason -ceq 'NonNativeTarget'
        ) `
        'x64 host and ARM64 target are skipped'
    Test-Condition `
        $ArmNative.NativeExecutionEligible `
        'ARM64 host and ARM64 target are native'
    Test-Condition `
        (
            -not $EmulatedX64Process.NativeExecutionEligible -and
            $EmulatedX64Process.SkipReason -ceq 'NonNativeTarget'
        ) `
        'emulated x64 process does not override ARM64 OS architecture'
    Test-Condition `
        (
            -not $CrossWithCallerSkip.NativeExecutionEligible -and
            $CrossWithCallerSkip.SkipReason -ceq 'NonNativeTarget'
        ) `
        'caller skip cannot obscure intrinsic nonnative target'
    Test-Condition `
        (
            (ConvertTo-FlashGateExecutionState `
                -Attempted $false `
                -ProcessStatus FAIL) -ceq 'SKIPPED' -and
            (ConvertTo-FlashGateExecutionState `
                -Attempted $true `
                -ProcessStatus FAIL) -ceq 'FAIL'
        ) `
        'structured execution state distinguishes skipped and failed'

    $RealArgumentsComplete = (
        -not [string]::IsNullOrWhiteSpace($RealWindowsX64Binary) -and
        -not [string]::IsNullOrWhiteSpace($ExpectedProductVersion) -and
        -not [string]::IsNullOrWhiteSpace($ExpectedFileVersion) -and
        -not [string]::IsNullOrWhiteSpace($ExpectedCommit) -and
        -not [string]::IsNullOrWhiteSpace($ExpectedSourceTime) -and
        -not [string]::IsNullOrWhiteSpace($ExpectedModified) -and
        $ExpectedCopyrightYear -ne 1970
    )
    if ($RealArgumentsComplete) {
        $CommonArguments = @(
            '-ExpectedProductVersion'
            $ExpectedProductVersion
            '-ExpectedFileVersion'
            $ExpectedFileVersion
            '-ExpectedCommit'
            $ExpectedCommit
            '-ExpectedSourceTime'
            $ExpectedSourceTime
            '-ExpectedModified'
            $ExpectedModified
            '-ExpectedCopyrightYear'
            [string]$ExpectedCopyrightYear
            '-ArtifactTimeoutSeconds'
            '10'
        )

        $PositiveOutput = & (Join-Path $PSHOME 'pwsh.exe') `
            -NoProfile `
            -File $Verifier `
            -BinaryPath $RealWindowsX64Binary `
            -ExpectedPublicArch x64 `
            -ExpectedGOARCH amd64 `
            @CommonArguments 2>&1
        $PositiveExit = $LASTEXITCODE
        $PositiveText = ($PositiveOutput | Out-String) -replace `
            "`e\[[0-9;]*m", `
            ''
        Test-Condition `
            (
                $PositiveExit -eq 0 -and
                $PositiveText -match 'RuntimeExecution\s+:\s+PASS' -and
                $PositiveText -match 'HelpContract\s+:\s+PASS'
            ) `
            'real Windows x64 positive verifier'

        . $Verifier `
            -BinaryPath $RealWindowsX64Binary `
            -ExpectedPublicArch x64 `
            -ExpectedGOARCH amd64 `
            -ExpectedProductVersion $ExpectedProductVersion `
            -ExpectedFileVersion $ExpectedFileVersion `
            -ExpectedCommit $ExpectedCommit `
            -ExpectedSourceTime $ExpectedSourceTime `
            -ExpectedModified $ExpectedModified `
            -ExpectedCopyrightYear $ExpectedCopyrightYear

        function Invoke-ControlledFullWindowsVerifier {
            param(
                [Parameter(Mandatory)]
                [string] $Mode
            )

            $ControlledMode = $Mode
            $ResultFactory = ${function:New-ControlledProcessResult}
            $ProductionInvoker = ${function:Invoke-FlashGateBoundedProcess}
            $Invoker = {
                param(
                    [string] $FilePath,
                    [string[]] $Arguments,
                    [string] $WorkingDirectory,
                    [int] $TimeoutMilliseconds,
                    [int] $MaximumCharacters
                )

                $Key = $Arguments -join ' '
                $FailureResult = switch ($ControlledMode) {
                    'nonzero-compact' {
                        if ($Key -ceq '--version') {
                            & $ResultFactory `
                                -ExitCode 7 `
                                -Stderr 'controlled failure' `
                                -FailureReason NonZeroExit
                        }
                    }
                    'nonzero-verbose' {
                        if ($Key -ceq '--version --verbose') {
                            & $ResultFactory `
                                -ExitCode 7 `
                                -Stderr 'controlled failure' `
                                -FailureReason NonZeroExit
                        }
                    }
                    'nonzero-help' {
                        if ($Key -ceq '--help') {
                            & $ResultFactory `
                                -ExitCode 7 `
                                -Stderr 'controlled failure' `
                                -FailureReason NonZeroExit
                        }
                    }
                    'timeout' {
                        if ($Key -ceq '--version') {
                            & $ResultFactory `
                                -TimedOut $true `
                                -FailureReason Timeout
                        }
                    }
                    'stdout-limit' {
                        if ($Key -ceq '--version') {
                            & $ResultFactory `
                                -OutputLimitExceeded $true `
                                -Stdout ('x' * 1024) `
                                -FailureReason StdoutLimitExceeded
                        }
                    }
                    'stderr-limit' {
                        if ($Key -ceq '--version') {
                            & $ResultFactory `
                                -OutputLimitExceeded $true `
                                -Stderr ('x' * 1024) `
                                -FailureReason StderrLimitExceeded
                        }
                    }
                    'start-failure' {
                        if ($Key -ceq '--version') {
                            & $ResultFactory `
                                -FailureReason `
                                    'StartOrProcessFailure:Win32Exception'
                        }
                    }
                    'termination-failure' {
                        if ($Key -ceq '--version') {
                            & $ResultFactory `
                                -TimedOut $true `
                                -FailureReason TerminationFailed
                        }
                    }
                }
                if ($null -ne $FailureResult) {
                    return $FailureResult
                }

                $RealResult = & $ProductionInvoker `
                    -FilePath $FilePath `
                    -Arguments $Arguments `
                    -WorkingDirectory $WorkingDirectory `
                    -TimeoutMilliseconds $TimeoutMilliseconds `
                    -MaximumCharactersPerStream $MaximumCharacters
                if (
                    $ControlledMode -ceq 'empty-help' -and
                    $Key -ceq '--help'
                ) {
                    $RealResult.Stdout = ''
                }
                elseif (
                    $ControlledMode -ceq 'partial-help' -and
                    $Key -ceq '--help'
                ) {
                    $RealResult.Stdout =
                        ($RealResult.Stdout -split '\r?\n')[0]
                }
                return $RealResult
            }.GetNewClosure()

            return Invoke-FlashGateWindowsMetadataMain `
                -BinaryPath $RealWindowsX64Binary `
                -ExpectedPublicArch x64 `
                -ExpectedGOARCH amd64 `
                -ExpectedProductVersion $ExpectedProductVersion `
                -ExpectedFileVersion $ExpectedFileVersion `
                -ExpectedCommit $ExpectedCommit `
                -ExpectedSourceTime $ExpectedSourceTime `
                -ExpectedModified $ExpectedModified `
                -ExpectedCopyrightYear $ExpectedCopyrightYear `
                -RuntimeProcessInvoker $Invoker
        }

        foreach ($Mode in @(
            'nonzero-compact'
            'nonzero-verbose'
            'nonzero-help'
            'empty-help'
            'partial-help'
            'timeout'
            'stdout-limit'
            'stderr-limit'
            'start-failure'
            'termination-failure'
        )) {
            $FullResult = Invoke-ControlledFullWindowsVerifier -Mode $Mode
            $VerifierResult = $FullResult.Result
            $ExpectedRuntime = if (
                $Mode -in @('nonzero-help', 'empty-help', 'partial-help')
            ) {
                'PASS'
            }
            else {
                'FAIL'
            }
            $ExpectedHelp = if (
                $Mode -in @('nonzero-help', 'empty-help', 'partial-help')
            ) {
                'FAIL'
            }
            else {
                'SKIPPED'
            }
            $ExpectedRuntimeReason = switch ($Mode) {
                'nonzero-compact' { 'NonZeroExit' }
                'nonzero-verbose' { 'NonZeroExit' }
                'timeout' { 'Timeout' }
                'stdout-limit' { 'StdoutLimitExceeded' }
                'stderr-limit' { 'StderrLimitExceeded' }
                'start-failure' {
                    'StartOrProcessFailure:Win32Exception'
                }
                'termination-failure' { 'TerminationFailed' }
                default { $null }
            }
            $ExpectedHelpReason = switch ($Mode) {
                'nonzero-help' { 'NonZeroExit' }
                'empty-help' { 'HelpContractMismatch' }
                'partial-help' { 'HelpContractMismatch' }
                default { $null }
            }
            $FullCondition = (
                    $FullResult.ExitCode -ne 0 -and
                    $VerifierResult.Status -ceq 'FAIL' -and
                    $VerifierResult.RuntimeExecution -ceq $ExpectedRuntime -and
                    $VerifierResult.HelpContract -ceq $ExpectedHelp -and
                    (
                        $null -eq $ExpectedRuntimeReason -or
                        $VerifierResult.RuntimeFailureReason -ceq
                            $ExpectedRuntimeReason
                    ) -and
                    (
                        $null -eq $ExpectedHelpReason -or
                        $VerifierResult.HelpFailureReason -ceq
                            $ExpectedHelpReason
                    ) -and
                    (
                        $ExpectedHelp -cne 'SKIPPED' -or
                        $VerifierResult.HelpSkipReason -ceq
                            'RuntimeValidationFailed'
                    ) -and
                    $VerifierResult.ErrorCount -gt 0 -and
                    -not [string]::IsNullOrWhiteSpace(
                        $VerifierResult.Errors
                    )
                )
            $FullCaseName = "full Windows verifier aggregates $Mode"
            if (-not $FullCondition) {
                $FullCaseName += (
                    " [Exit=$($FullResult.ExitCode);" +
                    "Status=$($VerifierResult.Status);" +
                    "Runtime=$($VerifierResult.RuntimeExecution);" +
                    "RuntimeReason=$($VerifierResult.RuntimeFailureReason);" +
                    "Help=$($VerifierResult.HelpContract);" +
                    "HelpReason=$($VerifierResult.HelpFailureReason);" +
                    "HelpSkip=$($VerifierResult.HelpSkipReason);" +
                    "Errors=$($VerifierResult.ErrorCount)]"
                )
            }
            Test-Condition $FullCondition $FullCaseName
        }

        $StaticMarker = Join-Path $TempRoot 'static-launch.txt'
        $env:FLASHGATE_VERIFIER_TEST_MODE = '1'
        $StaticOutput = & (Join-Path $PSHOME 'pwsh.exe') `
            -NoProfile `
            -File $Verifier `
            -BinaryPath $RealWindowsX64Binary `
            -ExpectedPublicArch x64 `
            -ExpectedGOARCH amd64 `
            -ExpectedProductVersion $ExpectedProductVersion `
            -ExpectedFileVersion $ExpectedFileVersion `
            -ExpectedCommit $ExpectedCommit `
            -ExpectedSourceTime $ExpectedSourceTime `
            -ExpectedModified $ExpectedModified `
            -ExpectedCopyrightYear ($ExpectedCopyrightYear - 1) `
            -TestLaunchMarkerPath $StaticMarker 2>&1
        $StaticExit = $LASTEXITCODE
        $StaticText = ($StaticOutput | Out-String) -replace `
            "`e\[[0-9;]*m", `
            ''
        Test-Condition `
            (
                $StaticExit -ne 0 -and
                $StaticText -match 'Status\s+:\s+FAIL' -and
                $StaticText -match 'RuntimeExecution\s+:\s+SKIPPED' -and
                $StaticText -match 'ExecutionSkipReason\s+:\s+StaticValidationFailed' -and
                $StaticText -match 'HelpContract\s+:\s+SKIPPED' -and
                $StaticText -match 'HelpSkipReason\s+:\s+StaticValidationFailed' -and
                -not (Test-Path -LiteralPath $StaticMarker)
            ) `
            'static failure prevents launch'

        $MissingOutput = & (Join-Path $PSHOME 'pwsh.exe') `
            -NoProfile `
            -File $Verifier `
            -BinaryPath (Join-Path $TempRoot 'missing.exe') `
            -ExpectedPublicArch x64 `
            -ExpectedGOARCH amd64 `
            @CommonArguments 2>&1
        $MissingExit = $LASTEXITCODE
        $MissingText = ($MissingOutput | Out-String) -replace `
            "`e\[[0-9;]*m", `
            ''
        Test-Condition `
            (
                $MissingExit -ne 0 -and
                $MissingText -match 'RuntimeExecution\s+:\s+SKIPPED'
            ) `
            'missing artifact fails without execution'

        if (-not [string]::IsNullOrWhiteSpace($RealWindowsArm64Binary)) {
            $ArmOutput = & (Join-Path $PSHOME 'pwsh.exe') `
                -NoProfile `
                -File $Verifier `
                -BinaryPath $RealWindowsArm64Binary `
                -ExpectedPublicArch arm64 `
                -ExpectedGOARCH arm64 `
                @CommonArguments 2>&1
            $ArmExit = $LASTEXITCODE
            $ArmText = ($ArmOutput | Out-String) -replace `
                "`e\[[0-9;]*m", `
                ''
            Test-Condition `
                (
                    $ArmExit -eq 0 -and
                    $ArmText -match 'RuntimeExecution\s+:\s+SKIPPED' -and
                    $ArmText -match 'ExecutionSkipReason\s+:\s+NonNativeTarget'
                ) `
                'real Windows ARM64 static verification is skipped on x64'
        }
    }

    if ($Failures.Count -eq 0) {
        $ExitCode = 0
    }
}
catch {
    $Failures.Add("Unhandled test failure: $($_.Exception.Message)")
}
finally {
    if ($null -eq $PreviousTestMode) {
        Remove-Item Env:FLASHGATE_VERIFIER_TEST_MODE -ErrorAction SilentlyContinue
    }
    else {
        $env:FLASHGATE_VERIFIER_TEST_MODE = $PreviousTestMode
    }
    $KnownFiles = @(
        (Join-Path $TempRoot 'static-launch.txt')
    ) + @($BarrierFiles)
    foreach ($KnownFile in $KnownFiles) {
        if (Test-Path -LiteralPath $KnownFile -PathType Leaf) {
            Remove-Item -LiteralPath $KnownFile -Force
        }
    }
    foreach ($KnownDirectory in $BarrierDirectories) {
        if (Test-Path -LiteralPath $KnownDirectory -PathType Container) {
            Remove-Item -LiteralPath $KnownDirectory -Force
        }
    }
    if (Test-Path -LiteralPath $TempRoot -PathType Container) {
        Remove-Item -LiteralPath $TempRoot -Force
    }

    [pscustomobject]@{
        Status       = if ($Failures.Count -eq 0) { 'PASS' } else { 'FAIL' }
        CaseCount    = $Cases.Count
        FailureCount = $Failures.Count
        Failures     = if ($Failures.Count -gt 0) {
            $Failures -join [Environment]::NewLine
        }
        else {
            $null
        }
    } | Format-List
}

exit $ExitCode
