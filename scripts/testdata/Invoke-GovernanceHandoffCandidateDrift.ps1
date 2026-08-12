#requires -Version 7.6
[CmdletBinding()]
param(
    [Parameter(Mandatory)][string]$CandidatePath,
    [Parameter(Mandatory)][ValidateSet('MUTATE','REPLACE')][string]$Mode,
    [Parameter(Mandatory)][string]$HandshakePath
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$utf8 = [System.Text.UTF8Encoding]::new($false)
try {
    if ($Mode -ceq 'MUTATE') {
        $stream = [System.IO.FileStream]::new(
            $CandidatePath, [System.IO.FileMode]::Append,
            [System.IO.FileAccess]::Write, [System.IO.FileShare]::Read
        )
        try {
            $bytes = $utf8.GetBytes('candidate-drift')
            $stream.Write($bytes, 0, $bytes.Length)
        }
        finally { $stream.Dispose() }
    }
    else {
        $replacedPath = $CandidatePath + '.replaced'
        [System.IO.File]::Move($CandidatePath, $replacedPath, $false)
        [System.IO.File]::WriteAllText($CandidatePath, 'replacement-object', $utf8)
    }
    [System.IO.File]::WriteAllText($HandshakePath, $Mode, $utf8)
}
catch {
    [System.IO.File]::WriteAllText($HandshakePath, 'FAIL: ' + $_.Exception.Message, $utf8)
    throw
}
