[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [ValidateSet(
        'valid-version',
        'valid-help',
        'empty',
        'partial-help',
        'nonzero',
        'hang',
        'stdout-flood',
        'stderr-flood',
        'spawn-child',
        'delayed-marker',
        'termination-seam-target'
    )]
    [string] $Mode,

    [string] $MarkerPath,

    [string] $ReadyPath,

    [string] $ReleasePath,

    [string] $ReleasedPath,

    [string] $ChildPidPath,

    [string] $TimeoutActivationProbePath,

    [string] $TimeoutActivatedPath,

    [string] $TimeoutFinalCheckPath,

    [int] $ForeignPid,

    [int] $ParentPid,

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
    [string] $ReadyMode = 'valid'
)

$ErrorActionPreference = 'Stop'

function Write-AtomicUtf8Record {
    param(
        [Parameter(Mandatory)]
        [string] $Path,

        [AllowEmptyString()]
        [Parameter(Mandatory)]
        [string] $Value
    )

    $TemporaryPath = "$Path.tmp"
    [IO.File]::WriteAllText(
        $TemporaryPath,
        $Value,
        [Text.UTF8Encoding]::new($false)
    )
    [IO.File]::Move($TemporaryPath, $Path)
}

function Write-AtomicBytes {
    param(
        [Parameter(Mandatory)]
        [string] $Path,

        [Parameter(Mandatory)]
        [AllowEmptyCollection()]
        [byte[]] $Value
    )

    $TemporaryPath = "$Path.tmp"
    [IO.File]::WriteAllBytes($TemporaryPath, $Value)
    [IO.File]::Move($TemporaryPath, $Path)
}

function Get-ControlRecordBytes {
    param(
        [Parameter(Mandatory)]
        [ValidateSet('READY', 'RELEASED', 'ACTIVATED')]
        [string] $RecordName,

        [Parameter(Mandatory)]
        [int] $ProcessId,

        [Parameter(Mandatory)]
        [string] $Variant
    )

    $Prefix = "${RecordName}:"
    $Valid = "$Prefix$ProcessId"
    $Text = switch ($Variant) {
        'overflow' { "${Prefix}2147483648" }
        'exact-64' { $Prefix + ('9' * (64 - $Prefix.Length)) }
        'exact-65' { $Prefix + ('9' * (65 - $Prefix.Length)) }
        'leading-space' { " $Valid" }
        'trailing-space' { "$Valid " }
        'plus' { "${Prefix}+$ProcessId" }
        'leading-zero' { "${Prefix}0$ProcessId" }
        'space-after-colon' { "$Prefix $ProcessId" }
        'tab-after-colon' { "$Prefix`t$ProcessId" }
        'newline' { "$Valid`n" }
        'extra-record' { "$Valid`n$Valid" }
        'suffix' { "${Valid}x" }
        'empty' { '' }
        'nonnumeric' { "${Prefix}not-a-pid" }
        'zero' { "${Prefix}0" }
        'negative' { "${Prefix}-1" }
        'wrong-pid' {
            $WrongProcessId = if ($ProcessId -eq 1) { 2 } else { 1 }
            "$Prefix$WrongProcessId"
        }
        default { $Valid }
    }
    $Bytes = [Text.Encoding]::ASCII.GetBytes($Text)
    if ($Variant -ceq 'invalid-encoding') {
        $Bytes = [byte[]](
            [Text.Encoding]::ASCII.GetBytes($Prefix) +
            [byte]0xFF
        )
    }
    return ,$Bytes
}

function Test-ControlledProcessAlive {
    param(
        [Parameter(Mandatory)]
        [int] $ProcessId
    )

    try {
        $Process = [Diagnostics.Process]::GetProcessById($ProcessId)
        try {
            return -not $Process.HasExited
        }
        finally {
            $Process.Dispose()
        }
    }
    catch {
        return $false
    }
}

function Wait-ControlledFile {
    param(
        [Parameter(Mandatory)]
        [string] $Path,

        [int] $TimeoutMilliseconds = 10000
    )

    $Clock = [Diagnostics.Stopwatch]::StartNew()
    while ($Clock.ElapsedMilliseconds -lt $TimeoutMilliseconds) {
        if (Test-Path -LiteralPath $Path -PathType Leaf) {
            return $true
        }
        Start-Sleep -Milliseconds 5
    }
    return $false
}

switch ($Mode) {
    'valid-version' {
        [Console]::Out.Write('flashgate-mcp 1.2.3-rc.1')
    }
    'valid-help' {
        [Console]::Out.Write(@'
flashgate-mcp
Usage:
flashgate-mcp --version
flashgate-mcp --version --verbose
flashgate-mcp --help
MCP_ROOT
MCP_READ_ONLY
MCP_ALLOW_CWD_ROOT
'@)
    }
    'empty' {
        return
    }
    'partial-help' {
        [Console]::Out.Write("flashgate-mcp`nUsage:")
    }
    'nonzero' {
        [Console]::Error.Write('controlled failure')
        exit 7
    }
    'hang' {
        Start-Sleep -Seconds 30
    }
    'stdout-flood' {
        [Console]::Out.Write('x' * 1048576)
    }
    'stderr-flood' {
        [Console]::Error.Write('x' * 1048576)
    }
    'spawn-child' {
        if (
            [string]::IsNullOrWhiteSpace($MarkerPath) -or
            [string]::IsNullOrWhiteSpace($ReadyPath) -or
            [string]::IsNullOrWhiteSpace($ReleasePath) -or
            [string]::IsNullOrWhiteSpace($ReleasedPath) -or
            [string]::IsNullOrWhiteSpace($ChildPidPath) -or
            [string]::IsNullOrWhiteSpace($TimeoutActivationProbePath) -or
            [string]::IsNullOrWhiteSpace($TimeoutActivatedPath) -or
            [string]::IsNullOrWhiteSpace($TimeoutFinalCheckPath)
        ) {
            throw (
                'MarkerPath, ReadyPath, ReleasePath, ReleasedPath, ' +
                'ChildPidPath, TimeoutActivationProbePath, and ' +
                'TimeoutActivatedPath are required ' +
                'for spawn-child.'
            )
        }
        $StartInfo = [Diagnostics.ProcessStartInfo]::new()
        $StartInfo.FileName = Join-Path $PSHOME 'pwsh.exe'
        $StartInfo.UseShellExecute = $false
        $StartInfo.CreateNoWindow = $true
        foreach ($Argument in @(
            '-NoProfile'
            '-File'
            $PSCommandPath
            '-Mode'
            'delayed-marker'
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
            '-ParentPid'
            [string]$PID
            '-ForeignPid'
            [string]$ForeignPid
        )) {
            $null = $StartInfo.ArgumentList.Add($Argument)
        }
        $ControlledChild = [Diagnostics.Process]::Start($StartInfo)
        Write-AtomicUtf8Record `
            -Path $ChildPidPath `
            -Value "CHILD:$($ControlledChild.Id)"
        $ControlledChild.Dispose()
        if ($ReadyMode -ceq 'parent-exit-before-release') {
            $null = Wait-ControlledFile -Path $ReadyPath
            exit 31
        }
        if ($ReadyMode -ceq 'parent-exit-after-released') {
            $null = Wait-ControlledFile -Path $TimeoutActivationProbePath
            exit 32
        }
        if ($ReadyMode -ceq 'parent-exit-before-timeout') {
            $null = Wait-ControlledFile -Path $TimeoutFinalCheckPath
            exit 33
        }
        Start-Sleep -Seconds 30
    }
    'delayed-marker' {
        if (
            [string]::IsNullOrWhiteSpace($MarkerPath) -or
            [string]::IsNullOrWhiteSpace($ReadyPath) -or
            [string]::IsNullOrWhiteSpace($ReleasePath) -or
            [string]::IsNullOrWhiteSpace($ReleasedPath) -or
            [string]::IsNullOrWhiteSpace($ChildPidPath) -or
            [string]::IsNullOrWhiteSpace($TimeoutActivationProbePath) -or
            [string]::IsNullOrWhiteSpace($TimeoutActivatedPath) -or
            [string]::IsNullOrWhiteSpace($TimeoutFinalCheckPath)
        ) {
            throw (
                'MarkerPath, ReadyPath, ReleasePath, ReleasedPath, ' +
                'ChildPidPath, TimeoutActivationProbePath, and ' +
                'TimeoutActivatedPath are required ' +
                'for delayed-marker.'
            )
        }
        if ($ReadyMode -ceq 'late') {
            Start-Sleep -Seconds 4
        }
        if ($ReadyMode -ceq 'missing') {
            Start-Sleep -Seconds 30
            break
        }
        $ReadyPid = $PID
        if ($ReadyMode -ceq 'dead-pid') {
            $DeadStartInfo = [Diagnostics.ProcessStartInfo]::new()
            $DeadStartInfo.FileName = Join-Path $PSHOME 'pwsh.exe'
            $DeadStartInfo.UseShellExecute = $false
            $DeadStartInfo.CreateNoWindow = $true
            $DeadStartInfo.ArgumentList.Add('-NoProfile')
            $DeadStartInfo.ArgumentList.Add('-Command')
            $DeadStartInfo.ArgumentList.Add('exit 0')
            $DeadProcess = [Diagnostics.Process]::Start($DeadStartInfo)
            $ReadyPid = $DeadProcess.Id
            if (-not $DeadProcess.WaitForExit(5000)) {
                $DeadProcess.Kill()
                throw 'The controlled dead-PID process did not exit.'
            }
            $DeadProcess.Dispose()
        }
        elseif ($ReadyMode -ceq 'foreign-pid') {
            if ($ForeignPid -le 0) {
                throw 'ForeignPid must be positive for foreign-pid mode.'
            }
            $ReadyPid = $ForeignPid
        }
        $LegacyVariant = switch ($ReadyMode) {
            'invalid' { 'nonnumeric' }
            'oversize' { 'exact-65' }
            default { $ReadyMode }
        }
        $ReadyVariant = if ($RecordTarget -ceq 'ready') {
            if ($RecordVariant -cne 'valid') {
                $RecordVariant
            }
            elseif ($LegacyVariant -in @(
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
                'nonnumeric',
                'exact-65'
            )) {
                $LegacyVariant
            }
            else {
                'valid'
            }
        }
        else {
            'valid'
        }
        Write-AtomicBytes `
            -Path $ReadyPath `
            -Value (Get-ControlRecordBytes `
                -RecordName READY `
                -ProcessId $ReadyPid `
                -Variant $ReadyVariant)
        if ($ReadyMode -ceq 'exit-before-release') {
            exit 21
        }
        $ReleaseClock = [Diagnostics.Stopwatch]::StartNew()
        while ($ReleaseClock.ElapsedMilliseconds -lt 10000) {
            if (
                $ParentPid -gt 0 -and
                -not (Test-ControlledProcessAlive -ProcessId $ParentPid)
            ) {
                exit 40
            }
            if (Test-Path -LiteralPath $ReleasePath -PathType Leaf) {
                break
            }
            Start-Sleep -Milliseconds 25
        }
        if (-not (Test-Path -LiteralPath $ReleasePath -PathType Leaf)) {
            exit 8
        }
        if ($ReadyMode -ceq 'exit-after-release') {
            exit 22
        }
        $ReleasedVariant = if ($RecordTarget -ceq 'released') {
            $RecordVariant
        }
        else {
            'valid'
        }
        Write-AtomicBytes `
            -Path $ReleasedPath `
            -Value (Get-ControlRecordBytes `
                -RecordName RELEASED `
                -ProcessId $PID `
                -Variant $ReleasedVariant)
        $ActivationClock = [Diagnostics.Stopwatch]::StartNew()
        while ($ActivationClock.ElapsedMilliseconds -lt 10000) {
            if (
                $ParentPid -gt 0 -and
                -not (Test-ControlledProcessAlive -ProcessId $ParentPid)
            ) {
                exit 40
            }
            if (
                Test-Path `
                    -LiteralPath $TimeoutActivationProbePath `
                    -PathType Leaf
            ) {
                if ($ReadyMode -ceq 'exit-after-released') {
                    exit 23
                }
                Write-AtomicBytes `
                    -Path $TimeoutActivatedPath `
                    -Value (Get-ControlRecordBytes `
                        -RecordName ACTIVATED `
                        -ProcessId $PID `
                        -Variant $(if ($RecordTarget -ceq 'activated') {
                            $RecordVariant
                        }
                        else {
                            'valid'
                        }))
                if ($ReadyMode -ceq 'exit-after-activated') {
                    exit 26
                }
                break
            }
            Start-Sleep -Milliseconds 10
        }
        if (
            -not (Test-Path `
                -LiteralPath $TimeoutActivatedPath `
                -PathType Leaf)
        ) {
            exit 24
        }
        if (
            $ParentPid -gt 0 -and
            -not (Test-ControlledProcessAlive -ProcessId $ParentPid)
        ) {
            exit 40
        }
        if ($ReadyMode -ceq 'exit-after-final-check') {
            $FinalCheckClock = [Diagnostics.Stopwatch]::StartNew()
            while ($FinalCheckClock.ElapsedMilliseconds -lt 10000) {
                if (
                    Test-Path `
                        -LiteralPath $TimeoutFinalCheckPath `
                        -PathType Leaf
                ) {
                    exit 27
                }
                Start-Sleep -Milliseconds 5
            }
            exit 28
        }
        $SurvivorClock = [Diagnostics.Stopwatch]::StartNew()
        while ($SurvivorClock.ElapsedMilliseconds -lt 2000) {
            if (
                $ParentPid -gt 0 -and
                -not (Test-ControlledProcessAlive -ProcessId $ParentPid)
            ) {
                exit 40
            }
            Start-Sleep -Milliseconds 10
        }
        [IO.File]::WriteAllText(
            $MarkerPath,
            'child-survived',
            [Text.UTF8Encoding]::new($false)
        )
    }
    'termination-seam-target' {
        Start-Sleep -Seconds 2
    }
}
