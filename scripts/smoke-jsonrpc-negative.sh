#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/.." && pwd)"

BINARY_PATH="${REPO_ROOT}/build/flashgate-mcp"
STAMP="$$-$(date +%s%N)"
REQUEST_PATH="${REPO_ROOT}/build/smoke-jsonrpc-negative-${STAMP}-request.jsonl"
RESPONSE_PATH="${REPO_ROOT}/build/smoke-jsonrpc-negative-${STAMP}-response.jsonl"

cleanup() {
  rm -f "${REQUEST_PATH}" "${RESPONSE_PATH}"
}
trap cleanup EXIT

if [[ ! -x "${BINARY_PATH}" ]]; then
  echo "Binary not found or not executable: ${BINARY_PATH}" >&2
  echo "Run: go build -o build/flashgate-mcp ./cmd/server" >&2
  exit 1
fi

export MCP_ROOT="${REPO_ROOT}"

mkdir -p "${REPO_ROOT}/build"

cat > "${REQUEST_PATH}" <<'JSONRPC'
{"jsonrpc":"2.0","id":1,"method":
{"jsonrpc":"2.0","id":2,"method":"unknown/method","params":{}}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":123,"arguments":{}}}
{"jsonrpc":"2.0","method":"tools/call","params":{"name":"list_directory","arguments":{"path":"."}}}
{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"list_files","arguments":{}}}
{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"rename_path","arguments":{}}}
{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"stat_path","arguments":{}}}
{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"exists_path","arguments":{}}}
{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"mkdir","arguments":{}}}
{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"read_file","arguments":{"path":"../outside.txt"}}}
JSONRPC

"${BINARY_PATH}" < "${REQUEST_PATH}" > "${RESPONSE_PATH}"

python3 - "${RESPONSE_PATH}" <<'PY'
import json
import sys

response_path = sys.argv[1]

with open(response_path, "r", encoding="utf-8") as handle:
    responses = [json.loads(line) for line in handle if line.strip()]

if len(responses) != 9:
    raise SystemExit(f"Expected 9 JSON-RPC responses, got {len(responses)}. Response file: {response_path}")

parse_error = responses[0]
method_not_found = responses[1]
invalid_params = responses[2]
removed_tools = responses[3:8]

if parse_error.get("id") is not None:
    raise SystemExit(f"Expected parse error id null, got {parse_error.get('id')}")

if parse_error.get("error", {}).get("code") != -32700:
    raise SystemExit(f"Expected parse error code -32700, got {parse_error.get('error', {}).get('code')}")

if parse_error.get("error", {}).get("message") != "parse error":
    raise SystemExit(f"Expected parse error message, got {parse_error.get('error', {}).get('message')}")

if method_not_found.get("id") != 2:
    raise SystemExit(f"Expected unknown method response id 2, got {method_not_found.get('id')}")

if method_not_found.get("error", {}).get("code") != -32601:
    raise SystemExit(
        f"Expected method-not-found code -32601, got {method_not_found.get('error', {}).get('code')}"
    )

if method_not_found.get("error", {}).get("message") != "method not found":
    raise SystemExit(
        f"Expected generic method-not-found message, got {method_not_found.get('error', {}).get('message')}"
    )

if invalid_params.get("id") != 3:
    raise SystemExit(f"Expected invalid params response id 3, got {invalid_params.get('id')}")

if invalid_params.get("error", {}).get("code") != -32602:
    raise SystemExit(f"Expected invalid params code -32602, got {invalid_params.get('error', {}).get('code')}")

if invalid_params.get("error", {}).get("message") != "invalid params":
    raise SystemExit(f"Expected invalid params message, got {invalid_params.get('error', {}).get('message')}")

for removed_tool in removed_tools:
    if removed_tool.get("error", {}).get("code") != -32602:
        raise SystemExit("Expected removed tool name to return Invalid params")
    if removed_tool.get("error", {}).get("message") != "invalid params":
        raise SystemExit("Expected generic Invalid params message for removed tool")

tool_error = responses[8]
result = tool_error.get("result", {})
if "error" in tool_error or result.get("isError") is not True or "structuredContent" in result:
    raise SystemExit("Expected an MCP error result without structuredContent")
if set(result) != {"content", "isError"} or len(result.get("content", [])) != 1:
    raise SystemExit("Unexpected MCP error result fields or block count")
block = result["content"][0]
payload = json.loads(block["text"])
if block.get("type") != "text" or payload != {"category": "invalid_path", "message": "filesystem error: invalid path"}:
    raise SystemExit("Unexpected safe path-error payload")
if block["text"] != json.dumps(payload, separators=(",", ":")):
    raise SystemExit("Tool-error text must be compact deterministic JSON")
if __import__("os").environ["MCP_ROOT"] in json.dumps(tool_error):
    raise SystemExit("Root path leaked")
print("Negative JSON-RPC smoke test passed.")
PY
