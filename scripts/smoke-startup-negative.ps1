$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot 'WindowsConfigFixture.ps1')

$repoRoot = (Resolve-Path "$PSScriptRoot\..").Path
$binaryPath = Join-Path $repoRoot "build\flashgate-mcp.exe"
[string]$stamp = "{0}-{1}" -f ([DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()), $PID
$rootFile = Join-Path $repoRoot "build\startup-root-file-$stamp.txt"
$missingRoot = Join-Path $repoRoot "build\startup-missing-root-$stamp"

if (-not (Test-Path -LiteralPath $binaryPath)) {
    throw "Binary not found: $binaryPath. Run: go build -o build/flashgate-mcp.exe ./cmd/server"
}

function Assert-NoHostPathLeak {
    param([string]$Value)

    if ($Value -match '[A-Za-z]:[\\/]' -or $Value -match '\\\\[^\\]+' -or $Value -match '/(home|etc|private|tmp)/') {
        throw "Startup diagnostics leaked a host path: $Value"
    }
}

function Invoke-StartupCase {
    param(
        [string]$Name,
        [AllowNull()][string]$Document,
        [int]$ExpectedExitCode,
        [string]$ExpectedStderr
    )

    $startInfo = [System.Diagnostics.ProcessStartInfo]::new()
    $startInfo.FileName = $binaryPath
    $startInfo.WorkingDirectory = $repoRoot
    $startInfo.UseShellExecute = $false
    $startInfo.RedirectStandardInput = $true
    $startInfo.RedirectStandardOutput = $true
    $startInfo.RedirectStandardError = $true
    $startInfo.CreateNoWindow = $true

    $fixtureId = [Guid]::NewGuid().ToString('N')
    $configPath = Join-Path ([IO.Path]::GetTempPath()) "flashgate-startup-config-$stamp-$fixtureId.toml"
    [void]$startInfo.ArgumentList.Add('--config')
    [void]$startInfo.ArgumentList.Add($configPath)
    $startInfo.Environment['MCP_ROOT'] = $repoRoot
    $startInfo.Environment['MCP_READ_ONLY'] = 'true'

    $process = [System.Diagnostics.Process]::new()
    $process.StartInfo = $startInfo
    $started = $false
    try {
        if ($Document -ne 'ABSENT') {
            New-WindowsConfigFixture -Path $configPath -Document $Document
        }
        $started = $process.Start()
        if (-not $started) {
            throw "Failed to start case '$Name'"
        }
        $process.StandardInput.Close()
        $stdout = $process.StandardOutput.ReadToEnd()
        $stderr = $process.StandardError.ReadToEnd()
        $process.WaitForExit()

        if ($process.ExitCode -ne $ExpectedExitCode) {
            throw "Case '$Name': expected exit code $ExpectedExitCode, got $($process.ExitCode). stderr=$stderr"
        }
        if ($stdout.Length -ne 0) {
            throw "Case '$Name': expected empty stdout, got $stdout"
        }

        [string]$normalizedStderr = $stderr -replace "`r`n", "`n"
        if ($normalizedStderr -ne $ExpectedStderr) {
            throw "Case '$Name': expected stderr '$ExpectedStderr', got '$normalizedStderr'"
        }
        Assert-NoHostPathLeak -Value $normalizedStderr
    } finally {
        if ($started -and -not $process.HasExited) {
            $process.Kill($true)
            $process.WaitForExit()
        }
        $process.Dispose()
        Remove-Item -LiteralPath $configPath -Force -ErrorAction SilentlyContinue
    }
}

New-Item -ItemType Directory -Path (Split-Path $rootFile) -Force | Out-Null
[System.IO.File]::WriteAllText($rootFile, "not a directory", [System.Text.UTF8Encoding]::new($false))

try {
    $header = "[files]`ndefault_id = 'work'`n"
    $quotedRoot = ConvertTo-Json -InputObject $repoRoot -Compress
    $quotedMissing = ConvertTo-Json -InputObject $missingRoot -Compress
    $quotedFile = ConvertTo-Json -InputObject $rootFile -Compress
    $absolute = "$($header)roots.work.path = $quotedRoot`n"
    $missing = "$($header)roots.work.path = $quotedMissing`n"
    $file = "$($header)roots.work.path = $quotedFile`n"
    $cases = @(
        @{ Name = 'missing config'; Document = 'ABSENT'; Exit = 3; Category = 'missing_config' }
        @{ Name = 'missing root'; Document = $header; Exit = 3; Category = 'missing_root' }
        @{ Name = 'missing default'; Document = "[files.roots.work]`npath = $quotedRoot"; Exit = 3; Category = 'missing_root' }
        @{ Name = 'unknown default'; Document = $absolute.Replace("'work'", "'unknown'"); Exit = 3; Category = 'invalid_root' }
        @{ Name = 'empty root'; Document = "$($header)roots.work.path = ''"; Exit = 3; Category = 'invalid_root' }
        @{ Name = 'whitespace root'; Document = "$($header)roots.work.path = '   '"; Exit = 3; Category = 'invalid_root' }
        @{ Name = 'relative root'; Document = "$($header)roots.work.path = 'subdir'"; Exit = 3; Category = 'invalid_root' }
        @{ Name = 'parent root'; Document = "$($header)roots.work.path = '..'"; Exit = 3; Category = 'invalid_root' }
        @{ Name = 'dot root'; Document = "$($header)roots.work.path = '.'"; Exit = 3; Category = 'invalid_root' }
        @{ Name = 'legacy group'; Document = "[filesystem]`nroot = $quotedRoot"; Exit = 3; Category = 'invalid_config' }
        @{ Name = 'legacy development option'; Document = "$($absolute)allow_cwd_root = true"; Exit = 3; Category = 'invalid_config' }
        @{ Name = 'missing filesystem root'; Document = $missing; Exit = 3; Category = 'root_not_found' }
        @{ Name = 'file root'; Document = $file; Exit = 3; Category = 'root_not_directory' }
        @{ Name = 'missing secondary root'; Document = "$($absolute)roots.archive.path = $quotedMissing"; Exit = 3; Category = 'root_not_found' }
        @{ Name = 'retired read-only option'; Document = "$($absolute)read_only = true"; Exit = 3; Category = 'invalid_config' }
        @{ Name = 'unknown profile'; Document = "$($absolute)[profiles]`nactive = 'unknown'"; Exit = 3; Category = 'invalid_profile' }
        @{ Name = 'valid absolute root'; Document = $absolute; Exit = 0; Stderr = '' }
        @{ Name = 'valid overlapping roots'; Document = "$($absolute)roots.archive.path = $quotedRoot"; Exit = 0; Stderr = '' }
    )

    foreach ($case in $cases) {
        $expected = if ($case.Exit -eq 0) { $case.Stderr } else { "flashgate-mcp: startup failed ($($case.Category))`n" }
        Invoke-StartupCase -Name $case.Name -Document $case.Document -ExpectedExitCode $case.Exit -ExpectedStderr $expected
    }

    Write-Host "Startup negative smoke test passed."
} finally {
    Remove-Item -LiteralPath $rootFile -Force -ErrorAction SilentlyContinue
}
