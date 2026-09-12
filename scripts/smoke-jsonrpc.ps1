$ErrorActionPreference = "Stop"

$repoRoot = Resolve-Path "$PSScriptRoot\.."
$binaryPath = Join-Path $repoRoot "build\flashgate-mcp.exe"
$buildDir = Join-Path $repoRoot "build"
$stamp = "{0}-{1}" -f ([DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()), $PID
$requestPath = Join-Path $buildDir "smoke-jsonrpc-$stamp-request.jsonl"
$responsePath = Join-Path $buildDir "smoke-jsonrpc-$stamp-response.jsonl"
$moveSourceRelative = "build/smoke-move-$stamp-source.txt"
$moveTargetRelative = "build/smoke-move-$stamp-target.txt"
$moveSourcePath = Join-Path $repoRoot $moveSourceRelative
$moveTargetPath = Join-Path $repoRoot $moveTargetRelative
$readOnlyWriteRelative = "build/smoke-readonly-$stamp-write.txt"
$readOnlyCreateRelative = "build/smoke-readonly-$stamp-directory"
$readOnlyDeleteRelative = "build/smoke-readonly-$stamp-delete.txt"
$readOnlyCopySourceRelative = "build/smoke-readonly-$stamp-copy-source.txt"
$readOnlyCopyTargetRelative = "build/smoke-readonly-$stamp-copy-target.txt"
$readOnlyMoveSourceRelative = "build/smoke-readonly-$stamp-move-source.txt"
$readOnlyMoveTargetRelative = "build/smoke-readonly-$stamp-move-target.txt"
$readOnlyWritePath = Join-Path $repoRoot $readOnlyWriteRelative
$readOnlyCreatePath = Join-Path $repoRoot $readOnlyCreateRelative
$readOnlyDeletePath = Join-Path $repoRoot $readOnlyDeleteRelative
$readOnlyCopySourcePath = Join-Path $repoRoot $readOnlyCopySourceRelative
$readOnlyCopyTargetPath = Join-Path $repoRoot $readOnlyCopyTargetRelative
$readOnlyMoveSourcePath = Join-Path $repoRoot $readOnlyMoveSourceRelative
$readOnlyMoveTargetPath = Join-Path $repoRoot $readOnlyMoveTargetRelative

if (-not (Test-Path $binaryPath)) {
    throw "Binary not found: $binaryPath. Run: go build -o build/flashgate-mcp.exe ./cmd/server"
}

$env:MCP_ROOT = $repoRoot

New-Item -ItemType Directory -Path $buildDir -Force | Out-Null

function Get-CallToolStructuredContent {
    param(
        [object]$Response,
        [int]$ExpectedId
    )

    if ($Response.id -ne $ExpectedId -or
        $Response.PSObject.Properties.Name -contains "error" -or
        -not ($Response.result -is [PSCustomObject])) {
        throw "Expected successful tools/call response id $ExpectedId"
    }

    $allowedFields = @("content", "structuredContent", "isError")
    foreach ($field in $Response.result.PSObject.Properties.Name) {
        if ($allowedFields -notcontains $field) {
            throw "tools/call result id $ExpectedId contains unexpected top-level field '$field'"
        }
    }
    if ($Response.result.PSObject.Properties.Name -notcontains "content" -or -not ($Response.result.content -is [System.Array])) {
        throw "tools/call result id $ExpectedId content must be an array"
    }
    $blocks = @($Response.result.content)
    if ($blocks.Count -ne 1) {
        throw "tools/call result id $ExpectedId content must contain exactly one block"
    }
    foreach ($block in $blocks) {
        $blockFields = @($block.PSObject.Properties.Name)
        if (-not ($block -is [PSCustomObject]) -or
            $blockFields.Count -ne 2 -or
            $blockFields -notcontains "type" -or
            $blockFields -notcontains "text" -or
            $block.type -cne "text" -or
            -not ($block.text -is [string])) {
            throw "tools/call result id $ExpectedId contains an invalid text block"
        }
    }
    if ($Response.result.PSObject.Properties.Name -notcontains "structuredContent" -or
        -not ($Response.result.structuredContent -is [PSCustomObject])) {
        throw "tools/call result id $ExpectedId structuredContent must be an object"
    }
    if ($Response.result.PSObject.Properties.Name -contains "isError") {
        if (-not ($Response.result.isError -is [bool]) -or $Response.result.isError) {
            throw "tools/call result id $ExpectedId contains an invalid success isError value"
        }
    }

    $textValue = $blocks[0].text | ConvertFrom-Json
    if (-not ($textValue -is [PSCustomObject])) {
        throw "tools/call result id $ExpectedId text must encode an object"
    }
    $textJSON = $textValue | ConvertTo-Json -Compress -Depth 100
    $structuredJSON = $Response.result.structuredContent | ConvertTo-Json -Compress -Depth 100
    if ($textJSON -cne $structuredJSON) {
        throw "tools/call result id $ExpectedId text and structuredContent differ"
    }

    return $Response.result.structuredContent
}

try {
    if ($env:MCP_READ_ONLY -eq "true") {
        foreach ($fixturePath in @($readOnlyDeletePath, $readOnlyCopySourcePath, $readOnlyMoveSourcePath)) {
            [System.IO.File]::WriteAllText($fixturePath, "read-only-smoke-fixture", [System.Text.UTF8Encoding]::new($false))
        }
    }

    $requests = @(
        '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"smoke-test","version":"dev"}}}'
        '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
        '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_directory","arguments":{}}}'
        '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"read_file","arguments":{"path":"README.md"}}}'
        '{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"get_path_info","arguments":{"path":"README.md"}}}'
        '{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"get_path_info","arguments":{"path":"smoke-missing-path"}}}'
    )

    if ($env:MCP_READ_ONLY -ne "true") {
        [System.IO.File]::WriteAllText($moveSourcePath, "move-smoke", [System.Text.UTF8Encoding]::new($false))
        $requests += '{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"move_path","arguments":{"source":"' + $moveSourceRelative + '","target":"' + $moveTargetRelative + '"}}}'
    } else {
        $requests += @(
            ('{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"write_file","arguments":{"path":"' + $readOnlyWriteRelative + '","content":"blocked"}}}')
            ('{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"create_directory","arguments":{"path":"' + $readOnlyCreateRelative + '"}}}')
            ('{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"delete_path","arguments":{"path":"' + $readOnlyDeleteRelative + '"}}}')
            ('{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"copy_path","arguments":{"source":"' + $readOnlyCopySourceRelative + '","target":"' + $readOnlyCopyTargetRelative + '"}}}')
            ('{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"move_path","arguments":{"source":"' + $readOnlyMoveSourceRelative + '","target":"' + $readOnlyMoveTargetRelative + '"}}}')
        )
    }

    [System.IO.File]::WriteAllText($requestPath, (($requests -join "`n") + "`n"), [System.Text.UTF8Encoding]::new($false))

    cmd /c "`"$binaryPath`" < `"$requestPath`" > `"$responsePath`""

    if ($LASTEXITCODE -ne 0) {
        throw "Smoke test process failed with exit code $LASTEXITCODE"
    }

    $responses = Get-Content $responsePath | Where-Object { $_.Trim().Length -gt 0 }

    $expectedResponseCount = if ($env:MCP_READ_ONLY -eq "true") { 11 } else { 7 }
    if ($responses.Count -ne $expectedResponseCount) {
        throw "Expected $expectedResponseCount JSON-RPC responses, got $($responses.Count). Response file: $responsePath"
    }

    $initialize = $responses[0] | ConvertFrom-Json
    $toolsList = $responses[1] | ConvertFrom-Json
    $listDirectory = $responses[2] | ConvertFrom-Json
    $readFile = $responses[3] | ConvertFrom-Json
    $existingPathInfo = $responses[4] | ConvertFrom-Json
    $missingPathInfo = $responses[5] | ConvertFrom-Json

    if ($initialize.id -ne 1) {
        throw "Expected initialize response id 1, got $($initialize.id)"
    }

    if (-not $initialize.result.protocolVersion) {
        throw "Initialize response does not contain protocolVersion"
    }

    if ($initialize.result.serverInfo.name -ne "flashgate") {
        throw "Expected serverInfo.name flashgate, got $($initialize.result.serverInfo.name)"
    }

    if ($toolsList.id -ne 2) {
        throw "Expected tools/list response id 2, got $($toolsList.id)"
    }

    if (-not $toolsList.result.tools) {
        throw "tools/list response does not contain tools"
    }

    $toolNames = @($toolsList.result.tools | ForEach-Object { $_.name })

    $expectedTools = @(
        "list_directory",
        "read_file",
        "get_path_info",
        "system_info"
    )

    if ($env:MCP_READ_ONLY -ne "true") {
        $expectedTools += @(
            "write_file",
            "create_directory",
            "delete_path",
            "copy_path",
            "move_path"
        )
    }

    if (($toolNames -join "`0") -cne ($expectedTools -join "`0")) {
        throw "Unexpected tool order. Expected: $($expectedTools -join ', '). Actual: $($toolNames -join ', ')"
    }

    foreach ($expectedTool in $expectedTools) {
        if ($toolNames -notcontains $expectedTool) {
            throw "Expected tool '$expectedTool' was not listed. Actual tools: $($toolNames -join ', ')"
        }
    }

    foreach ($toolName in $toolNames) {
        if ($expectedTools -notcontains $toolName) {
            throw "Unexpected tool '$toolName' was listed. Expected tools: $($expectedTools -join ', ')"
        }
    }

    $listResult = Get-CallToolStructuredContent -Response $listDirectory -ExpectedId 3
    $readResult = Get-CallToolStructuredContent -Response $readFile -ExpectedId 4
    $existingResult = Get-CallToolStructuredContent -Response $existingPathInfo -ExpectedId 5
    $missingResult = Get-CallToolStructuredContent -Response $missingPathInfo -ExpectedId 6
    if ($null -eq $listResult.entries) {
        throw "list_directory did not return entries"
    }
    if (-not ($readResult.content -is [string]) -or $readResult.size -le 0) {
        throw "read_file did not return content and size"
    }
    if (-not $existingResult.exists -or $existingResult.path -ne "README.md") {
        throw "get_path_info did not report README.md as existing"
    }
    if ($missingResult.exists -or $missingResult.path -ne "smoke-missing-path" -or @($missingResult.PSObject.Properties).Count -ne 2) {
        throw "get_path_info did not report the missing path correctly"
    }
    if ($env:MCP_READ_ONLY -ne "true") {
        $moveResult = $responses[6] | ConvertFrom-Json
        $moveStructured = Get-CallToolStructuredContent -Response $moveResult -ExpectedId 7
        if (-not $moveStructured.moved -or -not (Test-Path -LiteralPath $moveTargetPath)) {
            throw "move_path did not perform rename semantics"
        }
    } else {
        $writeToolResponses = @($responses[6..10] | ForEach-Object { $_ | ConvertFrom-Json })
        for ($index = 0; $index -lt $writeToolResponses.Count; $index++) {
            $response = $writeToolResponses[$index]
            $expectedId = $index + 7
            if ($response.id -ne $expectedId -or $response.error.code -ne -32602 -or $response.error.message -ne "invalid params") {
                throw "Expected read-only-gated write tool id $expectedId to return generic Invalid params"
            }
        }
        foreach ($expectedFixture in @($readOnlyDeletePath, $readOnlyCopySourcePath, $readOnlyMoveSourcePath)) {
            if (-not (Test-Path -LiteralPath $expectedFixture -PathType Leaf)) {
                throw "Read-only smoke fixture was modified or removed: $expectedFixture"
            }
        }
        foreach ($unexpectedPath in @($readOnlyWritePath, $readOnlyCreatePath, $readOnlyCopyTargetPath, $readOnlyMoveTargetPath)) {
            if (Test-Path -LiteralPath $unexpectedPath) {
                throw "Read-only smoke created an unexpected path: $unexpectedPath"
            }
        }
    }

    Write-Host "JSON-RPC smoke test passed."
    Write-Host "Protocol version: $($initialize.result.protocolVersion)"
    Write-Host "Tools: $($toolNames -join ', ')"
} finally {
    Remove-Item -LiteralPath $requestPath, $responsePath, $moveSourcePath, $moveTargetPath, $readOnlyWritePath, $readOnlyDeletePath, $readOnlyCopySourcePath, $readOnlyCopyTargetPath, $readOnlyMoveSourcePath, $readOnlyMoveTargetPath -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $readOnlyCreatePath -Recurse -Force -ErrorAction SilentlyContinue
}
