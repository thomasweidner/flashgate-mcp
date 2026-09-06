#!/usr/bin/env bash
set -Eeuo pipefail

root_path="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
safety_library="$root_path/scripts/native-validation-safety.sh"
safety_helper="$root_path/scripts/safe-work-root.py"
python_safety_test="$root_path/scripts/test-safe-work-root.py"
test_root="$(mktemp -d /tmp/flashgate-native-safety.XXXXXXXX)"
sentinel="$test_root/outside-sentinel"
errors=()

cleanup() {
    if [[ "$test_root" == /tmp/flashgate-native-safety.* && -d "$test_root" ]]; then
        rm -rf -- "$test_root"
    fi
}
trap cleanup EXIT

printf 'preserve' >"$sentinel"

run_prepare() {
    local home_path="$1"
    local run_id="$2"

    HOME="$home_path" \
    FG_RUN_ID="$run_id" \
    NATIVE_SAFETY_HELPER="$safety_helper" \
    SAFETY_LIBRARY="$safety_library" \
        bash -Eeuo pipefail -c '
            source "$SAFETY_LIBRARY"
            flashgate_prepare_work_root
            trap safe_remove_work_root EXIT
        '
}

mkdir -p "$test_root/home"
for value in \
    '../../..' \
    '..' \
    '.' \
    '/tmp/x' \
    'a/b' \
    'a\b' \
    '' \
    ' ' \
    $'white\tspace' \
    "$(printf 'a%.0s' {1..81})"; do
    if run_prepare "$test_root/home" "$value" >/dev/null 2>&1; then
        errors+=("invalid FG_RUN_ID unexpectedly passed: ${value@Q}")
    fi
done

symlink_home="$test_root/symlink-base-home"
mkdir -p "$symlink_home/.cache" "$test_root/outside-base"
ln -s "$test_root/outside-base" \
    "$symlink_home/.cache/flashgate-mcp-validation"
if run_prepare "$symlink_home" safe-run >/dev/null 2>&1; then
    errors+=("symlink validation base unexpectedly passed")
fi

run_symlink_home="$test_root/run-symlink-home"
mkdir -p \
    "$run_symlink_home/.cache/flashgate-mcp-validation" \
    "$test_root/outside-run"
chmod 700 "$run_symlink_home/.cache/flashgate-mcp-validation"
ln -s "$test_root/outside-run" \
    "$run_symlink_home/.cache/flashgate-mcp-validation/safe-run"
if run_prepare "$run_symlink_home" safe-run >/dev/null 2>&1; then
    errors+=("symlink run directory unexpectedly passed")
fi

exchange_home="$test_root/exchange-home"
exchange_base="$exchange_home/.cache/flashgate-mcp-validation"
mkdir -p "$exchange_base/exchange-run/component" "$test_root/exchange-outside"
chmod 700 "$exchange_base"
rm -rf -- "$exchange_base/exchange-run/component"
ln -s "$test_root/exchange-outside" \
    "$exchange_base/exchange-run/component"
if python3 "$safety_helper" remove \
    --base "$exchange_base" \
    --run-id exchange-run >/dev/null 2>&1; then
    errors+=("exchanged target component unexpectedly passed")
fi

python_work_root="$test_root/python-work"
mkdir -m 700 -- "$python_work_root"
if ! FLASHGATE_TASK_ROOT="$test_root" \
    FLASHGATE_WORK_ROOT="$python_work_root" \
    TEMP="$python_work_root" \
    TMP="$python_work_root" \
    TMPDIR="$python_work_root" \
    python3 "$python_safety_test" >/dev/null 2>&1; then
    errors+=("descriptor-bound base-component exchange test failed")
fi

valid_home="$test_root/valid-home"
mkdir -p "$valid_home"
if ! run_prepare "$valid_home" valid-run >/dev/null 2>&1; then
    errors+=("valid direct-child work root failed")
fi

[[ "$(cat "$sentinel")" == "preserve" ]] ||
    errors+=("path outside the validation base was modified")

status="PASS"
((${#errors[@]} == 0)) || status="FAIL"
printf 'Status: %s\n' "$status"
printf 'TestRoot: %s\n' "$test_root"
printf 'WarningCount: 0\n'
printf 'ErrorCount: %d\n' "${#errors[@]}"
printf 'Warnings:\n'
printf 'Errors: %s\n' "$(IFS='; '; echo "${errors[*]:-}")"
[[ "$status" == "PASS" ]]
