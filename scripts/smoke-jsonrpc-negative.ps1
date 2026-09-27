$ErrorActionPreference = "Stop"

$repoRoot = Resolve-Path "$PSScriptRoot\.."
$binaryPath = Join-Path $repoRoot "build\flashgate-mcp.exe"
$buildDir = Join-Path $repoRoot "build"
[string]$stamp = "{0}-{1}" -f ([DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()), $PID
$requestPath = Join-Path $buildDir "smoke-jsonrpc-negative-$stamp-request.jsonl"
$responsePath = Join-Path $buildDir "smoke-jsonrpc-negative-$stamp-response.jsonl"

if (-not (Test-Path $binaryPath)) {
    throw "Binary not found: $binaryPath. Run: go build -o build/flashgate-mcp.exe ./cmd/server"
}

$env:MCP_ROOT = $repoRoot

New-Item -ItemType Directory -Path $buildDir -Force | Out-Null

try {
    $requests = @(
        '{"jsonrpc":"2.0","id":1,"method":'
        '{"jsonrpc":"2.0","id":2,"method":"unknown/method","params":{}}'
        '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":123,"arguments":{}}}'
        '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"list_directory","arguments":{"path":"."}}}'
        '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"list_files","arguments":{}}}'
        '{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"rename_path","arguments":{}}}'
        '{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"stat_path","arguments":{}}}'
        '{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"exists_path","arguments":{}}}'
        '{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"mkdir","arguments":{}}}'
        '{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"read_file","arguments":{"path":"../outside.txt"}}}'
    )

    [System.IO.File]::WriteAllText($requestPath, (($requests -join "`n") + "`n"), [System.Text.UTF8Encoding]::new($false))

    cmd /c "`"$binaryPath`" < `"$requestPath`" > `"$responsePath`""

    if ($LASTEXITCODE -ne 0) {
        throw "Negative smoke test process failed with exit code $LASTEXITCODE"
    }

    $responses = Get-Content $responsePath | Where-Object { $_.Trim().Length -gt 0 }

    if ($responses.Count -ne 9) {
        throw "Expected 9 JSON-RPC responses, got $($responses.Count). Response file: $responsePath"
    }

    $parseError = $responses[0] | ConvertFrom-Json
    $methodNotFound = $responses[1] | ConvertFrom-Json
    $invalidParams = $responses[2] | ConvertFrom-Json
    $removedListFiles = $responses[3] | ConvertFrom-Json
    $removedRenamePath = $responses[4] | ConvertFrom-Json
	$removedStatPath = $responses[5] | ConvertFrom-Json
	$removedExistsPath = $responses[6] | ConvertFrom-Json
	$removedMkdir = $responses[7] | ConvertFrom-Json

    if ($null -ne $parseError.id) {
        throw "Expected parse error id null, got $($parseError.id)"
    }

    if ($parseError.error.code -ne -32700) {
        throw "Expected parse error code -32700, got $($parseError.error.code)"
    }

    if ($parseError.error.message -ne "parse error") {
        throw "Expected parse error message, got $($parseError.error.message)"
    }

    if ($methodNotFound.id -ne 2) {
        throw "Expected unknown method response id 2, got $($methodNotFound.id)"
    }

    if ($methodNotFound.error.code -ne -32601) {
        throw "Expected method-not-found code -32601, got $($methodNotFound.error.code)"
    }

    if ($methodNotFound.error.message -ne "method not found") {
        throw "Expected generic method-not-found message, got $($methodNotFound.error.message)"
    }

    if ($invalidParams.id -ne 3) {
        throw "Expected invalid params response id 3, got $($invalidParams.id)"
    }

    if ($invalidParams.error.code -ne -32602) {
        throw "Expected invalid params code -32602, got $($invalidParams.error.code)"
    }

    if ($invalidParams.error.message -ne "invalid params") {
        throw "Expected invalid params message, got $($invalidParams.error.message)"
    }

    foreach ($removedToolResponse in @($removedListFiles, $removedRenamePath, $removedStatPath, $removedExistsPath, $removedMkdir)) {
        if ($removedToolResponse.error.code -ne -32602 -or $removedToolResponse.error.message -ne "invalid params") {
            throw "Expected removed tool name to return generic Invalid params"
        }
    }

    $toolError = $responses[8] | ConvertFrom-Json -AsHashtable
    $result = $toolError['result']
    if ($toolError.ContainsKey('error') -or $result['isError'] -isnot [bool] -or $result['isError'] -ne $true -or $result.ContainsKey('structuredContent')) {
        throw 'Expected an MCP error result without structuredContent'
    }
    if ($result.Count -ne 2 -or @($result['content']).Count -ne 1) {
        throw 'Unexpected MCP error fields or block count'
    }
    $block = $result['content'][0]
    if ($block.Count -ne 2 -or $block['type'] -ne 'text' -or $block['text'] -cne '{"category":"invalid_path","message":"filesystem error: invalid path"}') {
        throw 'Unexpected safe compact path-error payload'
    }
    if ($responses[8].Contains([string]$repoRoot) -or $responses[8].Contains(([string]$repoRoot | ConvertTo-Json -Compress).Trim('"'))) {
        throw 'Root path leaked'
    }
    Write-Host "Negative JSON-RPC smoke test passed."
} finally {
    Remove-Item -LiteralPath $requestPath, $responsePath -Force -ErrorAction SilentlyContinue
}
