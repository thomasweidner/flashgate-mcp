#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/.." && pwd)"
BINARY_PATH="${REPO_ROOT}/build/flashgate-mcp"
STAMP="$-$(date +%s%N)"
STDOUT_PATH="${REPO_ROOT}/build/startup-smoke-${STAMP}-stdout.txt"
STDERR_PATH="${REPO_ROOT}/build/startup-smoke-${STAMP}-stderr.txt"
ROOT_FILE="${REPO_ROOT}/build/startup-root-file-${STAMP}.txt"
MISSING_ROOT="${REPO_ROOT}/build/startup-missing-root-${STAMP}"

cleanup() {
  rm -f "${CONFIG_PATH}" "${STDOUT_PATH}" "${STDERR_PATH}" "${ROOT_FILE}"
  rmdir -- "${CONFIG_DIR}"
}
CONFIG_DIR="$(mktemp -d -- "${TMPDIR:-/tmp}/flashgate-config.XXXXXXXX")"
CONFIG_PATH="${CONFIG_DIR}/flashgate_config.toml"
trap cleanup EXIT

if [[ ! -x "${BINARY_PATH}" ]]; then
  echo "Binary not found or not executable: ${BINARY_PATH}" >&2
  echo "Run: go build -o build/flashgate-mcp ./cmd/server" >&2
  exit 1
fi

mkdir -p "${REPO_ROOT}/build"
printf '%s' 'not a directory' > "${ROOT_FILE}"

assert_no_host_path_leak() {
  local value="$1"
  if grep -Eq '([A-Za-z]:[\\/]|\\\\[^\\]+|/(home|etc|private|tmp)/)' <<<"${value}"; then
    echo "Startup diagnostics leaked a host path: ${value}" >&2
    exit 1
  fi
}

run_case() {
  local name="$1"
  local expected_exit="$2"
  local expected_stderr="$3"
  local document="$4"

  : > "${STDOUT_PATH}"
  : > "${STDERR_PATH}"

  set +e
  (
    cd "${REPO_ROOT}"
    if [[ "$document" != "ABSENT" ]]; then
      (umask 077; printf '%s\n' "$document" > "$CONFIG_PATH")
    fi
    MCP_ROOT="${REPO_ROOT}" MCP_READ_ONLY=true \
      "${BINARY_PATH}" --config "$CONFIG_PATH" < /dev/null > "${STDOUT_PATH}" 2> "${STDERR_PATH}"
  )
  local exit_code=$?
  set -e

  if [[ ${exit_code} -ne ${expected_exit} ]]; then
    echo "Case '${name}': expected exit ${expected_exit}, got ${exit_code}" >&2
    cat "${STDERR_PATH}" >&2
    exit 1
  fi
  if [[ -s "${STDOUT_PATH}" ]]; then
    echo "Case '${name}': expected empty stdout" >&2
    cat "${STDOUT_PATH}" >&2
    exit 1
  fi

  local stderr_value
  stderr_value="$(cat "${STDERR_PATH}")"
  if [[ "${stderr_value}" != "${expected_stderr}" ]]; then
    echo "Case '${name}': expected stderr '${expected_stderr}', got '${stderr_value}'" >&2
    exit 1
  fi
  assert_no_host_path_leak "${stderr_value}"
}

root_toml() {
  python3 -c 'import json,sys; print("[filesystem]\nroot = " + json.dumps(sys.argv[1], ensure_ascii=False))' "$1"
}

run_case "missing config" 3 "flashgate-mcp: startup failed (missing_config)" ABSENT
run_case "missing root" 3 "flashgate-mcp: startup failed (missing_root)" '[filesystem]'
run_case "empty root" 3 "flashgate-mcp: startup failed (invalid_root)" "$(root_toml '')"
run_case "whitespace root" 3 "flashgate-mcp: startup failed (invalid_root)" "$(root_toml '   ')"
run_case "relative root" 3 "flashgate-mcp: startup failed (invalid_root)" "$(root_toml subdir)"
run_case "parent root" 3 "flashgate-mcp: startup failed (invalid_root)" "$(root_toml ..)"
run_case "dot without opt-in" 3 "flashgate-mcp: startup failed (invalid_root)" "$(root_toml .)"
run_case "opt-in without root" 3 "flashgate-mcp: startup failed (missing_root)" $'[filesystem]\nallow_cwd_root = true'
run_case "invalid development option" 3 "flashgate-mcp: startup failed (invalid_config)" "$(root_toml "${REPO_ROOT}")"$'\nallow_cwd_root = "TRUE"'
run_case "missing filesystem root" 3 "flashgate-mcp: startup failed (root_not_found)" "$(root_toml "${MISSING_ROOT}")"
run_case "file root" 3 "flashgate-mcp: startup failed (root_not_directory)" "$(root_toml "${ROOT_FILE}")"
run_case "invalid read-only option" 3 "flashgate-mcp: startup failed (invalid_config)" "$(root_toml "${REPO_ROOT}")"$'\nread_only = "invalid"'
run_case "valid absolute root" 0 "" "$(root_toml "${REPO_ROOT}")"$'\nread_only = true'
run_case "development CWD" 0 "flashgate-mcp: warning: development CWD root enabled" "$(root_toml .)"$'\nallow_cwd_root = true\nread_only = true'

echo "Startup negative smoke test passed."
