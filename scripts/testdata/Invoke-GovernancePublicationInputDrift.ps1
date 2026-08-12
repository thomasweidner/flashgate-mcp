#requires -Version 7.6
[CmdletBinding()]
param(
    [Parameter(Mandatory)][string]$TargetPath,
    [Parameter(Mandatory)][string]$ContinuePath,
    [Parameter(Mandatory)][string]$CompletionPath
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$utf8 = [System.Text.UTF8Encoding]::new($false)
$status = 'FAIL'
$failureMessage = $null

try {
    $resolvedTarget = [System.IO.Path]::GetFullPath($TargetPath)
    if (-not (Test-Path -LiteralPath $resolvedTarget -PathType Leaf)) { throw "Drift target is missing: $resolvedTarget" }
    [System.IO.File]::AppendAllText($resolvedTarget, "`n# deterministic execution-input drift`n", $utf8)
    [void][System.IO.Directory]::CreateDirectory((Split-Path -Parent ([System.IO.Path]::GetFullPath($ContinuePath))))
    [System.IO.File]::WriteAllText([System.IO.Path]::GetFullPath($ContinuePath), "CONTINUE`n", $utf8)
    $status = 'PASS'
}
catch { $failureMessage = $_.Exception.Message }
finally {
    [void][System.IO.Directory]::CreateDirectory((Split-Path -Parent ([System.IO.Path]::GetFullPath($CompletionPath))))
    $record = [ordered]@{ status = $status; targetPath = $TargetPath; failureMessage = $failureMessage }
    [System.IO.File]::WriteAllText(
        [System.IO.Path]::GetFullPath($CompletionPath),
        (($record | ConvertTo-Json -Depth 10) + "`n"),
        $utf8
    )
}

if ($status -ceq 'PASS') { exit 0 }
exit 1
