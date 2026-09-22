#!/usr/bin/env bash
set -Eeuo pipefail

export LC_ALL=C

mode='normal'
if (($# > 0)); then
  if (($# == 1)) && [[ $1 == '--cleanup-negative-probe' || $1 == '--cleanup-escape-probe' ]]; then
    mode=${1#--}
  else
    printf '%s\n' \
      'Status: FAIL' \
      'TestCount: 0' \
      'PassCount: 0' \
      'Cleanup: PASS' \
      'Diagnostics: Harness: unsupported argument' \
      'WarningCount: 0' \
      'FailureCount: 1'
    exit 2
  fi
fi

root_path=$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
validator="$root_path/scripts/test-shell-scripts.sh"
script_path="$root_path/scripts/test-shell-scripts.tests.sh"

fail_preflight() {
  printf '%s\n' \
    'Status: FAIL' \
    'TestCount: 0' \
    'PassCount: 0' \
    'Cleanup: PASS' \
    "Diagnostics: Harness: $1" \
    'WarningCount: 0' \
    'FailureCount: 1'
  exit 2
}

if [[ ! -v FLASHGATE_WORK_ROOT ]]; then
  fail_preflight 'FLASHGATE_WORK_ROOT must be explicitly set'
fi
if [[ -z $FLASHGATE_WORK_ROOT ]]; then
  fail_preflight 'FLASHGATE_WORK_ROOT must not be empty'
fi
if [[ $FLASHGATE_WORK_ROOT != /* ]]; then
  fail_preflight 'FLASHGATE_WORK_ROOT must be an absolute path'
fi
if [[ ! -d $FLASHGATE_WORK_ROOT || -L $FLASHGATE_WORK_ROOT ]]; then
  fail_preflight 'FLASHGATE_WORK_ROOT must identify an existing non-symlink directory'
fi
if ! work_root=$(cd -P -- "$FLASHGATE_WORK_ROOT" && pwd); then
  fail_preflight 'FLASHGATE_WORK_ROOT cannot be resolved'
fi
if [[ $work_root == "$root_path" || $work_root == "$root_path/"* ]]; then
  fail_preflight 'FLASHGATE_WORK_ROOT must be outside the repository source tree'
fi

if ! test_root=$(mktemp -d -- "$work_root/BL-251-shell-validation-tests.XXXXXXXX"); then
  fail_preflight 'unique scratch directory creation failed'
fi
failures=()
test_count=0
pass_count=0
force_cleanup_failure=false

cleanup() {
  local exit_code=$?
  local cleanup_status='PASS'
  if [[ -d "$test_root" && ! -L "$test_root" && "$test_root" == "$work_root"/BL-251-shell-validation-tests.* ]]; then
    if [[ "$force_cleanup_failure" == true ]]; then
      failures+=("Cleanup: simulated deletion failure for controlled fixture root: $test_root")
      cleanup_status='FAIL'
    fi
    if ! rm -rf -- "$test_root"; then
      failures+=("Cleanup: deletion failed for controlled fixture root: $test_root")
      cleanup_status='FAIL'
    fi
  else
    failures+=("Cleanup: unsafe or missing test root: $test_root")
    cleanup_status='FAIL'
    exit_code=1
  fi

  local status='PASS' diagnostics='NONE'
  if ((${#failures[@]} > 0)); then
    status='FAIL'
    diagnostics=$(IFS=' | '; printf '%s' "${failures[*]}")
    exit_code=1
  fi
  printf '%s\n' \
    "Status: $status" \
    "TestCount: $test_count" \
    "PassCount: $pass_count" \
    "Cleanup: $cleanup_status" \
    "Diagnostics: $diagnostics" \
    'WarningCount: 0' \
    "FailureCount: ${#failures[@]}"
  exit "$exit_code"
}
trap cleanup EXIT

if [[ "$mode" == 'cleanup-negative-probe' ]]; then
  force_cleanup_failure=true
  exit 0
fi
if [[ "$mode" == 'cleanup-escape-probe' ]]; then
  test_root=$work_root
  exit 0
fi

record() {
  local name=$1 result=$2 detail=${3:-}
  ((test_count += 1))
  if [[ "$result" == 'true' ]]; then
    ((pass_count += 1))
  else
    failures+=("$name: $detail")
  fi
}

new_fixture() {
  local name=$1 mode=${2:-valid}
  local fixture="$test_root/$name"
  mkdir -p -- "$fixture/scripts"
  git -C "$fixture" init --quiet
  case "$mode" in
    valid)
      printf '%s\n' '#!/usr/bin/env bash' 'set -euo pipefail' "printf 'ok\\n'" > "$fixture/scripts/fixture.sh"
      ;;
    invalid)
      printf '%s\n' '#!/usr/bin/env bash' 'if then' > "$fixture/scripts/fixture.sh"
      ;;
    empty)
      printf '%s\n' 'fixture' > "$fixture/README.md"
      ;;
  esac
  git -C "$fixture" add --all
  printf '%s' "$fixture"
}

success_root=$(new_fixture 'success path with spaces')
before_status=$(git -C "$success_root" status --porcelain=v1 --untracked-files=all)
if success_output=$(/usr/bin/bash "$validator" --repository-root "$success_root" 2>&1); then
  record 'success-and-space-path' true
else
  record 'success-and-space-path' false "$success_output"
fi
after_status=$(git -C "$success_root" status --porcelain=v1 --untracked-files=all)
record 'success-does-not-mutate-repository' "$([[ "$before_status" == "$after_status" ]] && printf true || printf false)" 'Repository status changed.'

second_output=$(/usr/bin/bash "$validator" --repository-root "$success_root" 2>&1)
record 'deterministic-output' "$([[ "$success_output" == "$second_output" ]] && printf true || printf false)" 'Repeated output differs.'

empty_root=$(new_fixture 'empty' empty)
if empty_output=$(/usr/bin/bash "$validator" --repository-root "$empty_root" 2>&1); then
  record 'empty-inventory-fails' false "$empty_output"
else
  record 'empty-inventory-fails' true
fi

missing_root=$(new_fixture 'missing')
rm -- "$missing_root/scripts/fixture.sh"
if missing_output=$(/usr/bin/bash "$validator" --repository-root "$missing_root" 2>&1); then
  record 'missing-tracked-file-fails' false "$missing_output"
else
  record 'missing-tracked-file-fails' true
fi

syntax_root=$(new_fixture 'syntax' invalid)
if syntax_output=$(/usr/bin/bash "$validator" --repository-root "$syntax_root" 2>&1); then
  record 'bash-syntax-fails' false "$syntax_output"
else
  record 'bash-syntax-fails' true
fi

if wrong_shell_output=$(/usr/bin/bash "$validator" --repository-root "$success_root" --bash-path /bin/bash 2>&1); then
  record 'wrong-shell-path-fails' false "$wrong_shell_output"
else
  record 'wrong-shell-path-fails' true
fi

if invalid_arg_output=$(/usr/bin/bash "$validator" --bogus 2>&1); then
  record 'invalid-argument-fails' false "$invalid_arg_output"
else
  record 'invalid-argument-fails' true
fi

if cleanup_negative_output=$(FLASHGATE_WORK_ROOT="$work_root" /usr/bin/bash "$script_path" --cleanup-negative-probe 2>&1); then
  record 'cleanup-negative-probe-fails-closed' false "$cleanup_negative_output"
else
  cleanup_status_count=$(grep -c '^Status:' <<< "$cleanup_negative_output" || true)
  cleanup_failure_count=$(awk -F ': ' '$1 == "FailureCount" { print $2 }' <<< "$cleanup_negative_output")
  record 'cleanup-negative-probe-fails-closed' "$(
    [[ "$cleanup_status_count" == 1 ]] &&
      grep -qx 'Status: FAIL' <<< "$cleanup_negative_output" &&
      grep -qx 'Cleanup: FAIL' <<< "$cleanup_negative_output" &&
      [[ "${cleanup_failure_count:-0}" -gt 0 ]] &&
      printf true || printf false
  )" "$cleanup_negative_output"
fi

if work_root_unset_output=$(env -u FLASHGATE_WORK_ROOT /usr/bin/bash "$script_path" 2>&1); then
  record 'work-root-unset-fails' false "$work_root_unset_output"
else
  record 'work-root-unset-fails' "$(grep -qx 'Status: FAIL' <<< "$work_root_unset_output" && printf true || printf false)" "$work_root_unset_output"
fi

if work_root_empty_output=$(FLASHGATE_WORK_ROOT='' /usr/bin/bash "$script_path" 2>&1); then
  record 'work-root-empty-fails' false "$work_root_empty_output"
else
  record 'work-root-empty-fails' "$(grep -qx 'Status: FAIL' <<< "$work_root_empty_output" && printf true || printf false)" "$work_root_empty_output"
fi

if work_root_relative_output=$(FLASHGATE_WORK_ROOT='relative-work-root' /usr/bin/bash "$script_path" 2>&1); then
  record 'work-root-relative-fails' false "$work_root_relative_output"
else
  record 'work-root-relative-fails' "$(grep -qx 'Status: FAIL' <<< "$work_root_relative_output" && printf true || printf false)" "$work_root_relative_output"
fi

if work_root_source_output=$(FLASHGATE_WORK_ROOT="$root_path" /usr/bin/bash "$script_path" 2>&1); then
  record 'work-root-source-tree-fails' false "$work_root_source_output"
else
  record 'work-root-source-tree-fails' "$(grep -qx 'Status: FAIL' <<< "$work_root_source_output" && printf true || printf false)" "$work_root_source_output"
fi

if cleanup_escape_output=$(FLASHGATE_WORK_ROOT="$test_root" /usr/bin/bash "$script_path" --cleanup-escape-probe 2>&1); then
  record 'cleanup-escape-fails-closed' false "$cleanup_escape_output"
else
  record 'cleanup-escape-fails-closed' "$(
    grep -qx 'Status: FAIL' <<< "$cleanup_escape_output" &&
      grep -qx 'Cleanup: FAIL' <<< "$cleanup_escape_output" &&
      printf true || printf false
  )" "$cleanup_escape_output"
fi

link_root=$(new_fixture 'link' empty)
ln -s /etc/passwd "$link_root/scripts/escape.sh"
git -C "$link_root" add --all
if link_output=$(/usr/bin/bash "$validator" --repository-root "$link_root" 2>&1); then
  record 'symlink-entrypoint-fails' false "$link_output"
else
  record 'symlink-entrypoint-fails' true
fi
