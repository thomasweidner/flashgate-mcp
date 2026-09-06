[CmdletBinding()]
param([string]$WorkingPath)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
Import-Module (Join-Path $PSScriptRoot 'TaskBoundWorkRoot.psm1') -Force
$WorkRoot = Resolve-FlashGateWorkRoot -WorkingPath $WorkingPath
. (Join-Path $PSScriptRoot 'benchmark-window.ps1')

$Failures = [System.Collections.Generic.List[string]]::new()
$CheckCount = 0
$TemporaryDirectory = $null

function Test-Condition {
    param([bool]$Condition, [string]$Name)
    $script:CheckCount++
    if (-not $Condition) { $script:Failures.Add($Name) }
}

try {
    $Cases = @(
        @('2026-01-17T02:59:00Z', $true, '03:59 blocked'),
        @('2026-01-17T03:00:00Z', $false, '04:00 allowed'),
        @('2026-01-17T03:15:00Z', $false, '04:15 allowed'),
        @('2026-07-17T16:45:00Z', $false, '18:45 allowed'),
        @('2026-07-17T16:59:00Z', $false, '18:59 allowed'),
        @('2026-07-17T17:00:00Z', $true, '19:00 blocked'),
        @('2026-07-17T21:59:00Z', $true, '23:59 blocked')
    )
    foreach ($Case in $Cases) {
        $Status = Get-EuropeViennaMeasurementWindowStatus -UtcInstant ([DateTimeOffset]::Parse($Case[0]))
        Test-Condition ($Status.IsBlocked -eq $Case[1]) $Case[2]
    }

    $Winter = Get-EuropeViennaMeasurementWindowStatus -UtcInstant ([DateTimeOffset]::Parse('2026-01-17T12:00:00Z'))
    $Summer = Get-EuropeViennaMeasurementWindowStatus -UtcInstant ([DateTimeOffset]::Parse('2026-07-17T12:00:00Z'))
    Test-Condition ($Winter.LocalTime.Offset -eq [TimeSpan]::FromHours(1)) 'winter UTC offset'
    Test-Condition ($Summer.LocalTime.Offset -eq [TimeSpan]::FromHours(2)) 'summer UTC offset'
    if ($IsWindows) {
        Test-Condition ($Summer.TimeZoneId -eq 'W. Europe Standard Time') 'Windows time-zone ID'
    }

    $BlockedRejected = $false
    $BlockedMessageComplete = $false
    try {
        Assert-BaselineMeasurementWindow -UtcInstant ([DateTimeOffset]::Parse('2026-07-17T17:00:00Z')) | Out-Null
    }
    catch {
        $BlockedRejected = $_.Exception.Message -match 'No baseline was written or replaced'
        $BlockedMessageComplete = $_.Exception.Message -match '2026-07-17 19:00:00 \+02:00' -and $_.Exception.Message -match '19:00 inclusive to 04:00 exclusive' -and $_.Exception.Message -match '2026-07-18 04:00 Europe/Vienna'
    }
    Test-Condition $BlockedRejected 'blocked record precheck'
    Test-Condition $BlockedMessageComplete 'blocked message contains current time, window, and next window'

    $AllowedAccepted = $true
    try {
        Assert-BaselineMeasurementWindow -UtcInstant ([DateTimeOffset]::Parse('2026-07-17T02:15:00Z')) | Out-Null
    }
    catch {
        $AllowedAccepted = $false
    }
    Test-Condition $AllowedAccepted 'allowed record precheck'

    $TemporaryDirectory = New-FlashGateScratchDirectory -WorkingPath $WorkRoot -Prefix 'benchmark-window-test'
    $Candidate = Join-Path $TemporaryDirectory 'candidate.json'
    $Existing = Join-Path $TemporaryDirectory 'baseline.json'
    [IO.File]::WriteAllText($Candidate, 'candidate')
    [IO.File]::WriteAllText($Existing, 'existing')
    $PublishRejected = $false
    try {
        Publish-BenchmarkBaselineCandidate -CandidatePath $Candidate -DestinationPath $Existing -UtcInstant ([DateTimeOffset]::Parse('2026-07-17T17:00:00Z'))
    }
    catch {
        $PublishRejected = $true
    }
    Test-Condition $PublishRejected 'publication recheck blocks at 19:00'
    Test-Condition (Test-Path -LiteralPath $Candidate -PathType Leaf) 'blocked candidate retained for caller cleanup'
    Test-Condition (([IO.File]::ReadAllText($Existing)) -eq 'existing') 'existing baseline not replaced'
    Test-Condition ((Get-ContaminatedPerformanceWarning) -eq 'Performance values are contaminated by the scheduled host-load window and are not valid baseline evidence.') 'normal-run contamination warning'
}
catch {
    $Failures.Add("unexpected test error: $($_.Exception.Message)")
}
finally {
    if ($null -ne $TemporaryDirectory -and (Test-Path -LiteralPath $TemporaryDirectory -PathType Container)) {
        Remove-FlashGateScratchDirectory -Path $TemporaryDirectory -WorkingPath $WorkRoot -Prefix 'benchmark-window-test'
    }
    [pscustomobject]@{
        Status       = $(if ($Failures.Count -eq 0) { 'PASS' } else { 'FAIL' })
        CheckCount   = $CheckCount
        FailureCount = $Failures.Count
        Failures     = ($Failures -join '; ')
    } | Format-List
}

if ($Failures.Count -gt 0) { exit 1 }
