#requires -Version 7.6
[CmdletBinding()]
param(
    [string]$RepositoryRoot = (Split-Path -Parent $PSScriptRoot),
    [string]$GitBashPath = 'C:\Program Files\Git\bin\bash.exe'
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$result = $null
$exitCode = 1
try {
    Import-Module (Join-Path $PSScriptRoot 'ShellValidation.psm1') -Force
    $result = Invoke-FlashGateShellValidation `
        -RepositoryRoot $RepositoryRoot `
        -GitBashPath $GitBashPath
    $exitCode = if ($result.Status -eq 'PASS') { 0 } else { 1 }
}
catch {
    $result = [pscustomobject]@{
        Status = 'FAIL'
        RepositoryRoot = $RepositoryRoot
        PowerShellPath = (Get-Process -Id $PID).Path
        PowerShellVersion = $PSVersionTable.PSVersion.ToString()
        GitPath = $null
        GitBashPath = $GitBashPath
        InventoryCount = 0
        PowerShellScriptCount = 0
        BashScriptCount = 0
        ParserFailureCount = 0
        BashSyntaxFailureCount = 0
        RepositoryMutationDetected = $false
        Diagnostics = @($_.Exception.Message)
        WarningCount = 0
        FailureCount = 1
    }
}
finally {
    $diagnosticText = if (@($result.Diagnostics).Count -eq 0) {
        'NONE'
    }
    else {
        @($result.Diagnostics | ForEach-Object {
            if ($_ -is [string]) { $_ } else {
                "$($_.Code):$($_.Path):$($_.Line):$($_.Column):$($_.Message)"
            }
        }) -join ' | '
    }
    [pscustomobject]@{
        Status = $result.Status
        RepositoryRoot = $result.RepositoryRoot
        PowerShellPath = $result.PowerShellPath
        PowerShellVersion = $result.PowerShellVersion
        GitPath = $result.GitPath
        GitBashPath = $result.GitBashPath
        InventoryCount = $result.InventoryCount
        PowerShellScriptCount = $result.PowerShellScriptCount
        BashScriptCount = $result.BashScriptCount
        ParserFailureCount = $result.ParserFailureCount
        BashSyntaxFailureCount = $result.BashSyntaxFailureCount
        RepositoryMutationDetected = $result.RepositoryMutationDetected
        Diagnostics = $diagnosticText
        WarningCount = $result.WarningCount
        FailureCount = $result.FailureCount
    } | Format-List
}

exit $exitCode
