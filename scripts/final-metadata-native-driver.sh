#!/usr/bin/env bash
set -Eeuo pipefail

: "${FG_WINDOWS_REPO:?FG_WINDOWS_REPO is required}"
: "${FG_GIT_COMMON_DIR:?FG_GIT_COMMON_DIR is required}"
: "${FG_HEAD_COMMIT:?FG_HEAD_COMMIT is required}"
: "${FG_SNAPSHOT_TAR:?FG_SNAPSHOT_TAR is required}"
: "${FG_SNAPSHOT_MANIFEST:?FG_SNAPSHOT_MANIFEST is required}"
: "${FG_SNAPSHOT_VALIDATOR:?FG_SNAPSHOT_VALIDATOR is required}"
: "${FG_NATIVE_SAFETY:?FG_NATIVE_SAFETY is required}"
: "${FG_NATIVE_SAFETY_HELPER:?FG_NATIVE_SAFETY_HELPER is required}"
: "${FG_OUTPUT_DIR:?FG_OUTPUT_DIR is required}"
: "${FG_RUN_ID:?FG_RUN_ID is required}"
: "${FG_DISTRO_NAME:?FG_DISTRO_NAME is required}"

NATIVE_SAFETY_HELPER="$FG_NATIVE_SAFETY_HELPER"
# shellcheck source=native-validation-safety.sh
source "$FG_NATIVE_SAFETY"
flashgate_prepare_work_root

native_root="$NATIVE_WORK_ROOT"
repo_dir="$native_root/repository"
snapshot_dir="$native_root/verified-snapshot"
logs_dir="$native_root/logs"
summary_path="$FG_OUTPUT_DIR/native-final-summary.env"
failure_log="$FG_OUTPUT_DIR/native-final-failure.log"

mkdir -p "$FG_OUTPUT_DIR"
mkdir -p "$logs_dir"

write_failure() {
    local exit_code="$1"
    local line_number="$2"
    {
        printf 'STATUS=FAIL\n'
        printf 'DISTRO_NAME=%s\n' "$FG_DISTRO_NAME"
        printf 'NATIVE_ROOT=%s\n' "$native_root"
        printf 'ERROR=command failed at line %s with exit code %s\n' "$line_number" "$exit_code"
    } >"$summary_path"
    printf 'command failed at line %s with exit code %s\n' "$line_number" "$exit_code" >"$failure_log"
}
trap 'write_failure "$?" "$LINENO"' ERR
trap safe_remove_work_root EXIT

for command_name in git python3 date; do
    command -v "$command_name" >/dev/null 2>&1 || {
        printf 'required command not found: %s\n' "$command_name" >&2
        exit 1
    }
done

go_binary="$(command -v go || true)"
if [[ -z "$go_binary" ]]; then
    go_binary="$(
        find "$HOME/.local" -maxdepth 4 -type f -path '*/bin/go' 2>/dev/null |
            sort -V |
            tail -n 1
    )"
fi
[[ -n "$go_binary" && -x "$go_binary" ]] || {
    printf 'Go executable not found in PATH or under $HOME/.local\n' >&2
    exit 1
}

export PATH="$(dirname "$go_binary"):$PATH"
export GOPROXY=off
export GOWORK=off
export NO_COLOR=1

go_version="$(go version)"
kernel="$(uname -srmo)"
ubuntu_description="$(
    . /etc/os-release
    printf '%s' "${PRETTY_NAME:-unknown}"
)"

git clone \
    --no-hardlinks \
    --quiet \
    "$FG_GIT_COMMON_DIR" \
    "$repo_dir"
git -C "$repo_dir" checkout --detach --quiet "$FG_HEAD_COMMIT"

python3 "$FG_SNAPSHOT_VALIDATOR" \
    --archive "$FG_SNAPSHOT_TAR" \
    --manifest "$FG_SNAPSHOT_MANIFEST" \
    --extract-root "$snapshot_dir" \
    --overlay-root "$repo_dir"
cd "$repo_dir"

case "$PWD" in
    /mnt/*)
        printf 'native smoke repository must not be under a Windows mount: %s\n' "$PWD" >&2
        exit 1
        ;;
esac

chmod +x \
    scripts/build.sh \
    scripts/Test-LinuxMetadata.sh \
    scripts/smoke-jsonrpc.sh \
    scripts/smoke-jsonrpc-negative.sh \
    scripts/smoke-startup-negative.sh

bash -n scripts/build.sh
bash -n scripts/Test-LinuxMetadata.sh
bash -n scripts/smoke-jsonrpc.sh
bash -n scripts/smoke-jsonrpc-negative.sh
bash -n scripts/smoke-startup-negative.sh

go test -mod=vendor ./... >"$logs_dir/go-test.log" 2>&1
go vet -mod=vendor ./... >"$logs_dir/go-vet.log" 2>&1
git diff --check >"$logs_dir/git-diff-check.log" 2>&1

clear_inherited_mcp_environment() {
    local variable_name

    while IFS='=' read -r variable_name _; do
        case "$variable_name" in
            MCP_*)
                unset "$variable_name"
                ;;
        esac
    done < <(env)
}

clear_inherited_mcp_environment

mkdir -p build
bash scripts/build.sh \
    --goos linux \
    --goarch amd64 \
    --version 1.2.3-rc.1 \
    --output build/flashgate-mcp >"$logs_dir/build-linux-x64.log" 2>&1

bash scripts/smoke-jsonrpc.sh >"$logs_dir/smoke-default.log" 2>&1
bash scripts/smoke-jsonrpc.sh --read-only >"$logs_dir/smoke-readonly.log" 2>&1
bash scripts/smoke-jsonrpc-negative.sh >"$logs_dir/smoke-negative.log" 2>&1
bash scripts/smoke-startup-negative.sh >"$logs_dir/smoke-startup-negative.log" 2>&1

fixture_count="$(
    python3 - <<'PY'
import json
from pathlib import Path
data = json.loads(Path("internal/version/testdata/build-metadata-fixtures.json").read_text(encoding="utf-8"))
print(len(data["fixtures"]))
PY
)"
[[ "$fixture_count" == "3" ]] || {
    printf 'expected 3 build metadata fixtures, got %s\n' "$fixture_count" >&2
    exit 1
}

cp -R "$logs_dir" "$FG_OUTPUT_DIR/native-final-logs"

{
    printf 'STATUS=PASS\n'
    printf 'DISTRO_NAME=%s\n' "$FG_DISTRO_NAME"
    printf 'UBUNTU_DESCRIPTION=%s\n' "$ubuntu_description"
    printf 'KERNEL=%s\n' "$kernel"
    printf 'GO_VERSION=%s\n' "$go_version"
    printf 'NATIVE_ROOT=%s\n' "$native_root"
    printf 'FIXTURE_COUNT=%s\n' "$fixture_count"
    printf 'SMOKE_DEFAULT=PASS\n'
    printf 'SMOKE_READONLY=PASS\n'
    printf 'SMOKE_NEGATIVE=PASS\n'
    printf 'SMOKE_STARTUP_NEGATIVE=PASS\n'
} >"$summary_path"

trap - ERR
