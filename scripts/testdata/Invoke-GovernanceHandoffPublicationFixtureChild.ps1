#requires -Version 7.6
[CmdletBinding()]
param(
    [Parameter(Mandatory)][string]$ModulePath,
    [Parameter(Mandatory)][string]$StagingDirectory,
    [Parameter(Mandatory)][string]$FinalPath,
    [Parameter(Mandatory)][string]$HandshakePath,
    [Parameter(Mandatory)][string]$WaitHandleName,
    [Parameter(Mandatory)]
    [ValidateSet('CANDIDATE_VALIDATION_STARTED','CANDIDATE_VALIDATED','PUBLISHED')][string]$InterruptPhase
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$attemptCount = 0
$waitHandle = [System.Threading.EventWaitHandle]::OpenExisting($WaitHandleName)
try {
    Import-Module $ModulePath -Force
    $validator = {
        param($candidatePath)
        if ($InterruptPhase -ceq 'CANDIDATE_VALIDATION_STARTED') {
            [System.IO.File]::WriteAllText($HandshakePath, $candidatePath, [System.Text.UTF8Encoding]::new($false))
            [void]$waitHandle.WaitOne()
        }
        $archive = [System.IO.Compression.ZipFile]::OpenRead($candidatePath)
        try {
            if (@($archive.Entries).Count -lt 1) { throw 'Synthetic candidate is empty.' }
        }
        finally { $archive.Dispose() }
    }
    $observer = {
        param($state)
        if ([string]$state.Phase -ceq $InterruptPhase) {
            [System.IO.File]::WriteAllText($HandshakePath, [string]$state.CandidatePath, [System.Text.UTF8Encoding]::new($false))
            [void]$waitHandle.WaitOne()
        }
    }
    Publish-GovernanceHandoffPackage -StagingDirectory $StagingDirectory -FinalPath $FinalPath `
        -CandidateValidator $validator -PackageWriteAttemptCount ([ref]$attemptCount) `
        -PhaseObserver $observer | Out-Null
}
finally {
    $waitHandle.Dispose()
}
