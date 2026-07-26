#!/usr/bin/env bash
set -uo pipefail

root_path="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
# shellcheck source=verifier-process.sh
source "$root_path/scripts/verifier-process.sh"
FG_VERIFIER_ROOT="$root_path"

real_linux_x64=""
real_linux_arm64=""
expected_product_version=""
expected_file_version=""
expected_commit=""
expected_source_time=""
expected_modified=""

while (($# > 0)); do
    case "$1" in
        --real-linux-x64) real_linux_x64="$2"; shift 2 ;;
        --real-linux-arm64) real_linux_arm64="$2"; shift 2 ;;
        --expected-product-version) expected_product_version="$2"; shift 2 ;;
        --expected-file-version) expected_file_version="$2"; shift 2 ;;
        --expected-commit) expected_commit="$2"; shift 2 ;;
        --expected-source-time) expected_source_time="$2"; shift 2 ;;
        --expected-modified) expected_modified="$2"; shift 2 ;;
        *)
            printf 'Unknown argument: %s\n' "$1" >&2
            exit 2
            ;;
    esac
done

failures=()
cases=()
barrier_files=()
barrier_directories=()
temporary_directory="$(mktemp -d)"
helper="$root_path/scripts/testdata/verifier-process-helper.py"

cleanup() {
    local file=""
    for file in \
        "$temporary_directory/child-survived.txt" \
        "$temporary_directory/child-ready.txt" \
        "$temporary_directory/static-launch.txt" \
        "$temporary_directory/positive.log" \
        "$temporary_directory/static.log" \
        "$temporary_directory/missing.log" \
        "$temporary_directory/arm64.log" \
        "${barrier_files[@]}"; do
        if [[ -f "$file" || -L "$file" ]]; then
            rm -f -- "$file"
        fi
    done
    local directory=""
    for directory in "${barrier_directories[@]}"; do
        rmdir -- "$directory"
    done
    rmdir -- "$temporary_directory"
}
trap cleanup EXIT

check() {
    local condition="$1"
    local name="$2"
    cases+=("$name")
    if [[ "$condition" != true && "$condition" != 1 ]]; then
        failures+=("$name")
    fi
}

wait_for_file() {
    local path="$1"
    local timeout_seconds="$2"
    local deadline=$((SECONDS + timeout_seconds))
    while ((SECONDS < deadline)); do
        [[ -f "$path" ]] && return 0
        sleep 0.05
    done
    [[ -f "$path" ]]
}

wait_for_process_exit() {
    local process_id="$1"
    local timeout_seconds="$2"
    local deadline=$((SECONDS + timeout_seconds))
    while ((SECONDS < deadline)); do
        kill -0 "$process_id" 2>/dev/null || return 0
        sleep 0.05
    done
    ! kill -0 "$process_id" 2>/dev/null
}

marker_remains_absent() {
    local path="$1"
    local timeout_seconds="$2"
    local deadline=$((SECONDS + timeout_seconds))
    while ((SECONDS < deadline)); do
        [[ ! -e "$path" ]] || return 1
        sleep 0.1
    done
    [[ ! -e "$path" ]]
}

run_helper() {
    local mode="$1"
    local timeout_seconds="${2:-2}"
    local maximum_bytes="${3:-4096}"
    shift 3 || true
    run_flashgate_bounded_process \
        "$root_path" \
        "$timeout_seconds" \
        "$maximum_bytes" \
        python3 "$helper" "$mode" "$@"
}

run_ready_barrier() {
    local name="$1"
    local ready_mode="$2"
    local start_timeout="${3:-5}"
    local execution_timeout="${4:-0.3}"
    local release_write_failure="${5:-false}"
    local record_target="${6:-ready}"
    local record_variant="${7:-valid}"
    local metadata=""
    local runner_exit=0
    local started_seconds="$SECONDS"

    FG_BARRIER_MARKER="$temporary_directory/${name}-survivor.txt"
    FG_BARRIER_READY="$temporary_directory/${name}-ready.txt"
    FG_BARRIER_RELEASE="$temporary_directory/${name}-release.txt"
    FG_BARRIER_RELEASED="$temporary_directory/${name}-released.txt"
    FG_BARRIER_CHILD_PID_PATH="$temporary_directory/${name}-child.txt"
    FG_BARRIER_ACTIVATION="$temporary_directory/${name}-activation.txt"
    FG_BARRIER_ACTIVATED="$temporary_directory/${name}-activated.txt"
    FG_BARRIER_FINAL_CHECK="$temporary_directory/${name}-final-check.txt"
    local stdout_file="$temporary_directory/${name}-stdout.bin"
    local stderr_file="$temporary_directory/${name}-stderr.bin"
    barrier_files+=(
        "$FG_BARRIER_MARKER"
        "$FG_BARRIER_READY"
        "$FG_BARRIER_READY.tmp"
        "$FG_BARRIER_RELEASE"
        "$FG_BARRIER_RELEASED"
        "$FG_BARRIER_RELEASED.tmp"
        "$FG_BARRIER_CHILD_PID_PATH"
        "$FG_BARRIER_CHILD_PID_PATH.tmp"
        "$FG_BARRIER_ACTIVATION"
        "$FG_BARRIER_ACTIVATED"
        "$FG_BARRIER_ACTIVATED.tmp"
        "$FG_BARRIER_FINAL_CHECK"
        "$stdout_file"
        "$stderr_file"
    )
    if [[ "$release_write_failure" == true ]]; then
        mkdir -- "$FG_BARRIER_RELEASE"
        barrier_directories+=("$FG_BARRIER_RELEASE")
    fi

    metadata="$(
        python3 "$helper" ready-barrier-runner \
            --marker "$FG_BARRIER_MARKER" \
            --ready "$FG_BARRIER_READY" \
            --release "$FG_BARRIER_RELEASE" \
            --released "$FG_BARRIER_RELEASED" \
            --child-pid "$FG_BARRIER_CHILD_PID_PATH" \
            --activation-probe "$FG_BARRIER_ACTIVATION" \
            --activated "$FG_BARRIER_ACTIVATED" \
            --final-check "$FG_BARRIER_FINAL_CHECK" \
            --ready-mode "$ready_mode" \
            --record-target "$record_target" \
            --record-variant "$record_variant" \
            --runner "$root_path/scripts/bounded-process-runner.py" \
            --working-directory "$root_path" \
            --start-timeout "$start_timeout" \
            --execution-timeout "$execution_timeout" \
            --maximum-bytes 4096 \
            --stdout-file "$stdout_file" \
            --stderr-file "$stderr_file"
    )" || runner_exit=$?
    if ((runner_exit != 0)); then
        return "$runner_exit"
    fi
    IFS=$'\t' read -r \
        FG_PROCESS_ATTEMPTED \
        FG_PROCESS_STATUS \
        FG_PROCESS_EXIT_CODE \
        FG_PROCESS_TIMED_OUT \
        FG_PROCESS_OUTPUT_LIMIT_EXCEEDED \
        FG_PROCESS_FAILURE_REASON \
        FG_BARRIER_READY_OBSERVED \
        FG_BARRIER_READY_PID \
        FG_BARRIER_RELEASED_OBSERVED \
        FG_BARRIER_RELEASED_PID \
        FG_BARRIER_ACTIVATED_OBSERVED \
        FG_BARRIER_ACTIVATED_PID \
        FG_BARRIER_CHILD_ALIVE \
        FG_BARRIER_TIMEOUT_AFTER_READY \
        FG_BARRIER_READY_ELAPSED_MS \
        FG_BARRIER_FAILURE_REASON <<<"$metadata"
    FG_BARRIER_ELAPSED_SECONDS=$((SECONDS - started_seconds))
    FG_BARRIER_CONTROLLED_CHILD_PID=none
    if [[ -f "$FG_BARRIER_CHILD_PID_PATH" ]]; then
        local child_record=""
        child_record="$(<"$FG_BARRIER_CHILD_PID_PATH")"
        if [[ "$child_record" =~ ^CHILD:([1-9][0-9]*)$ ]]; then
            FG_BARRIER_CONTROLLED_CHILD_PID="${BASH_REMATCH[1]}"
        fi
    fi
}

run_helper valid-version 2 4096
check "$([[ "$FG_PROCESS_STATUS" == PASS ]] && printf true || printf false)" \
    "native positive bounded execution"

run_helper nonzero 2 4096
check "$(
    [[ "$FG_PROCESS_STATUS" == FAIL &&
        "$FG_PROCESS_EXIT_CODE" == 7 &&
        "$FG_PROCESS_FAILURE_REASON" == NonZeroExit ]] &&
        printf true || printf false
)" "invalid version exit code"
check "$(
    [[ "$(flashgate_execution_state \
        "$FG_PROCESS_ATTEMPTED" \
        "$FG_PROCESS_STATUS")" == FAIL ]] &&
        printf true || printf false
)" "invalid help exit becomes FAIL"

run_helper empty 2 4096
mapfile -t empty_missing < <(
    flashgate_missing_help_lines "$FG_PROCESS_STDOUT"
)
check "$(( ${#empty_missing[@]} == 8 ))" "empty help is incomplete"

run_helper partial-help 2 4096
mapfile -t partial_missing < <(
    flashgate_missing_help_lines "$FG_PROCESS_STDOUT"
)
check "$(( ${#partial_missing[@]} > 0 ))" "partial help is incomplete"

run_helper hang 0.3 4096
check "$(
    [[ "$FG_PROCESS_STATUS" == FAIL &&
        "$FG_PROCESS_TIMED_OUT" == true &&
        "$FG_PROCESS_FAILURE_REASON" == Timeout ]] &&
        printf true || printf false
)" "timeout fails closed"

run_helper stdout-flood 2 1024
check "$(
    [[ "$FG_PROCESS_STATUS" == FAIL &&
        "$FG_PROCESS_OUTPUT_LIMIT_EXCEEDED" == true &&
        ${#FG_PROCESS_STDOUT} -eq 1024 ]] &&
        printf true || printf false
)" "stdout limit fails closed"

run_helper stderr-flood 2 1024
check "$(
    [[ "$FG_PROCESS_STATUS" == FAIL &&
        "$FG_PROCESS_OUTPUT_LIMIT_EXCEEDED" == true &&
        ${#FG_PROCESS_STDERR} -eq 1024 ]] &&
        printf true || printf false
)" "stderr limit fails without deadlock"

for ready_iteration in 1 2 3; do
    run_ready_barrier "ready-${ready_iteration}" valid 10 0.3
    child_exited=false
    marker_absent=false
    if [[ "$FG_BARRIER_READY_PID" =~ ^[1-9][0-9]*$ ]]; then
        wait_for_process_exit "$FG_BARRIER_READY_PID" 3 &&
            child_exited=true
    fi
    marker_remains_absent "$FG_BARRIER_MARKER" 3 &&
        marker_absent=true
    check "$(
        [[ "$FG_PROCESS_ATTEMPTED" == true &&
            "$FG_PROCESS_STATUS" == FAIL &&
            "$FG_PROCESS_TIMED_OUT" == true &&
            "$FG_PROCESS_FAILURE_REASON" == Timeout &&
            "$FG_BARRIER_READY_OBSERVED" == true &&
            "$FG_BARRIER_READY_PID" =~ ^[1-9][0-9]*$ &&
            "$FG_BARRIER_RELEASED_OBSERVED" == true &&
            "$FG_BARRIER_RELEASED_PID" == "$FG_BARRIER_READY_PID" &&
            "$FG_BARRIER_ACTIVATED_OBSERVED" == true &&
            "$FG_BARRIER_ACTIVATED_PID" == "$FG_BARRIER_READY_PID" &&
            "$FG_BARRIER_CONTROLLED_CHILD_PID" == "$FG_BARRIER_READY_PID" &&
            "$FG_BARRIER_CHILD_ALIVE" == true &&
            "$FG_BARRIER_TIMEOUT_AFTER_READY" == true &&
            "$FG_BARRIER_READY_ELAPSED_MS" =~ ^[0-9]+$ &&
            "$FG_BARRIER_READY_ELAPSED_MS" -lt 10000 &&
            -f "$FG_BARRIER_RELEASE" &&
            -f "$FG_BARRIER_RELEASED" &&
            -f "$FG_BARRIER_ACTIVATION" &&
            -f "$FG_BARRIER_ACTIVATED" &&
            -f "$FG_BARRIER_FINAL_CHECK" &&
            "$FG_BARRIER_ELAPSED_SECONDS" -lt 15 &&
            "$child_exited" == true &&
            "$marker_absent" == true ]] &&
            printf true || printf false
    )" "READY precedes timeout and child cleanup iteration ${ready_iteration}"
done

negative_ready_specs=()
for record_target in ready released activated; do
    case "$record_target" in
        ready) record_reason_prefix=Ready ;;
        released) record_reason_prefix=Released ;;
        activated) record_reason_prefix=Activated ;;
    esac
    for record_variant in \
        invalid-encoding \
        overflow \
        exact-64 \
        exact-65 \
        leading-space \
        trailing-space \
        plus \
        leading-zero \
        space-after-colon \
        tab-after-colon \
        newline \
        extra-record \
        suffix \
        empty \
        nonnumeric \
        zero \
        negative; do
        negative_ready_specs+=(
            "${record_target}-record-${record_variant}|valid|${record_reason_prefix}PidInvalid|15|false|${record_target}|${record_variant}"
        )
    done
done
negative_ready_specs+=(
    "wrong-released-pid|valid|ReleasedPidInvalid|15|false|released|wrong-pid"
    "wrong-activated-pid|valid|ActivatedPidInvalid|15|false|activated|wrong-pid"
    "ready-missing|missing|ReadyTimeout|2|false|ready|valid"
    "ready-late|late|ReadyTimeout|2|false|ready|valid"
    "ready-dead-pid|dead-pid|ReadyChildNotAlive|15|false|ready|valid"
    "ready-foreign-pid|foreign-pid|ReadyChildIdentityMismatch|15|false|ready|valid"
    "ready-zombie-pid|zombie-pid|ReadyChildNotAlive|15|false|ready|valid"
    "release-write-failure|valid|ReleaseWriteFailure|15|true|ready|valid"
    "child-identity-mismatch-after-ready|child-identity-mismatch-after-ready|ReadyChildIdentityMismatch|15|false|ready|valid"
    "parent-identity-mismatch-after-ready|parent-identity-mismatch-after-ready|ReadyParentIdentityMismatch|15|false|ready|valid"
)
for race_iteration in 1 2 3; do
    negative_ready_specs+=(
        "exit-before-release-${race_iteration}|exit-before-release|ReadyChildNotAlive|15|false|ready|valid"
        "exit-after-release-${race_iteration}|exit-after-release|ReadyChildNotAliveAfterRelease|15|false|ready|valid"
        "exit-after-released-${race_iteration}|exit-after-released|ReadyChildNotAliveBeforeTimeout|15|false|ready|valid"
        "exit-after-activated-${race_iteration}|exit-after-activated|ReadyChildNotAliveBeforeTimeout|15|false|ready|valid"
        "exit-after-final-check-${race_iteration}|exit-after-final-check|ReadyChildNotAliveBeforeTimeout|15|false|ready|valid"
        "parent-exit-before-release-${race_iteration}|parent-exit-before-release|ReadyParentNotAlive|15|false|ready|valid"
        "parent-exit-after-released-${race_iteration}|parent-exit-after-released|ReadyParentNotAlive|15|false|ready|valid"
        "parent-exit-before-timeout-${race_iteration}|parent-exit-before-timeout|ReadyParentNotAlive|15|false|ready|valid"
    )
done

negative_marker_names=()
negative_marker_paths=()
for negative_ready_spec in "${negative_ready_specs[@]}"; do
    IFS='|' read -r \
        ready_name \
        ready_mode \
        expected_ready_reason \
        ready_start_timeout \
        release_write_failure \
        record_target \
        record_variant <<<"$negative_ready_spec"
    run_ready_barrier \
        "$ready_name" \
        "$ready_mode" \
        "$ready_start_timeout" \
        0.3 \
        "$release_write_failure" \
        "$record_target" \
        "$record_variant"
    controlled_child_exited=false
    maximum_elapsed_seconds=$((ready_start_timeout + 3))
    if [[ "$FG_BARRIER_CONTROLLED_CHILD_PID" =~ ^[1-9][0-9]*$ ]] &&
        wait_for_process_exit "$FG_BARRIER_CONTROLLED_CHILD_PID" 3; then
        controlled_child_exited=true
    elif [[
        "$ready_mode" == missing || "$ready_mode" == late
    ]] &&
        [[ "$FG_BARRIER_READY_OBSERVED" == false ]] &&
        [[ ! -e "$FG_BARRIER_RELEASE" ]] &&
        [[ ! -e "$FG_BARRIER_RELEASED" ]]; then
        controlled_child_exited=true
    fi
    negative_condition="$(
        [[ "$FG_PROCESS_ATTEMPTED" == true &&
            "$FG_PROCESS_STATUS" == FAIL &&
            "$FG_PROCESS_TIMED_OUT" == false &&
            "$FG_PROCESS_FAILURE_REASON" == "$expected_ready_reason" &&
            "$FG_BARRIER_TIMEOUT_AFTER_READY" == false &&
            "$FG_BARRIER_ELAPSED_SECONDS" -lt "$maximum_elapsed_seconds" &&
            "$controlled_child_exited" == true ]] &&
            printf true || printf false
    )"
    negative_case_name="${ready_name} fails bounded before timeout activation"
    if [[ "$negative_condition" != true ]]; then
        negative_case_name+=" [Status=${FG_PROCESS_STATUS};TimedOut=${FG_PROCESS_TIMED_OUT};Reason=${FG_PROCESS_FAILURE_REASON};Expected=${expected_ready_reason};AfterReady=${FG_BARRIER_TIMEOUT_AFTER_READY};Elapsed=${FG_BARRIER_ELAPSED_SECONDS};ChildPid=${FG_BARRIER_CONTROLLED_CHILD_PID};ChildExited=${controlled_child_exited}]"
    fi
    check "$negative_condition" "$negative_case_name"
    negative_marker_names+=("$ready_name")
    negative_marker_paths+=("$FG_BARRIER_MARKER")
done

sleep 2.3
for negative_index in "${!negative_marker_names[@]}"; do
    check "$(
        [[ ! -e "${negative_marker_paths[$negative_index]}" ]] &&
            printf true || printf false
    )" "${negative_marker_names[$negative_index]} leaves no survivor marker"
done

run_flashgate_bounded_process \
    "$root_path" \
    1 \
    4096 \
    "$temporary_directory/missing-command" --help
check "$(
    [[ "$FG_PROCESS_ATTEMPTED" == true &&
        "$FG_PROCESS_STATUS" == FAIL &&
        "$FG_PROCESS_FAILURE_REASON" == StartOrProcessFailure:* ]] &&
        printf true || printf false
)" "missing executable start fails closed"

flashgate_execution_decision x86_64 x64 false
check "$FG_NATIVE_EXECUTION_ELIGIBLE" \
    "x64 host and x64 target are native"
flashgate_execution_decision x86_64 arm64 false
check "$(
    [[ "$FG_NATIVE_EXECUTION_ELIGIBLE" == false &&
        "$FG_EXECUTION_SKIP_REASON" == NonNativeTarget ]] &&
        printf true || printf false
)" "x64 host and ARM64 target are skipped"
flashgate_execution_decision aarch64 arm64 false
check "$FG_NATIVE_EXECUTION_ELIGIBLE" \
    "ARM64 host and ARM64 target are native"
flashgate_execution_decision x86_64 arm64 false
check "$(
    [[ "$FG_NATIVE_EXECUTION_ELIGIBLE" == false &&
        "$FG_EXECUTION_SKIP_REASON" == NonNativeTarget ]] &&
        printf true || printf false
)" "binfmt availability cannot override x64 host architecture"
flashgate_execution_decision x86_64 arm64 true
check "$(
    [[ "$FG_NATIVE_EXECUTION_ELIGIBLE" == false &&
        "$FG_EXECUTION_SKIP_REASON" == NonNativeTarget ]] &&
        printf true || printf false
)" "caller skip cannot obscure intrinsic nonnative target"
flashgate_execution_decision x86_64 x64 true
check "$(
    [[ "$FG_NATIVE_EXECUTION_ELIGIBLE" == false &&
        "$FG_EXECUTION_SKIP_REASON" == CallerRequestedSkip ]] &&
        printf true || printf false
)" "caller skip is only an additional restriction"
check "$(
    [[ "$(flashgate_execution_state false FAIL)" == SKIPPED &&
        "$(flashgate_execution_state true FAIL)" == FAIL ]] &&
        printf true || printf false
)" "structured execution state distinguishes skipped and failed"

real_arguments_complete=false
if [[ -n "$real_linux_x64" &&
    -n "$expected_product_version" &&
    -n "$expected_file_version" &&
    -n "$expected_commit" &&
    -n "$expected_source_time" &&
    -n "$expected_modified" ]]; then
    real_arguments_complete=true
fi

if [[ "$real_arguments_complete" == true ]]; then
    common_arguments=(
        --expected-product-version "$expected_product_version"
        --expected-file-version "$expected_file_version"
        --expected-commit "$expected_commit"
        --expected-source-time "$expected_source_time"
        --expected-modified "$expected_modified"
        --artifact-timeout-seconds 10
    )

    bash "$root_path/scripts/Test-LinuxMetadata.sh" \
        --binary "$real_linux_x64" \
        --expected-public-arch x64 \
        --expected-goarch amd64 \
        "${common_arguments[@]}" \
        >"$temporary_directory/positive.log" 2>&1
    positive_exit=$?
    check "$(
        [[ $positive_exit -eq 0 ]] &&
            grep -q '^RuntimeExecution: PASS$' \
                "$temporary_directory/positive.log" &&
            grep -q '^HelpContract: PASS$' \
                "$temporary_directory/positive.log" &&
            printf true || printf false
    )" "real Linux x64 positive verifier"

    # Source-only test seam: direct script execution always reinstalls the
    # production runtime wrapper before calling its main function.
    source "$root_path/scripts/Test-LinuxMetadata.sh"

    controlled_mode=""
    set_controlled_process_result() {
        FG_PROCESS_ATTEMPTED=true
        FG_PROCESS_STATUS=FAIL
        FG_PROCESS_EXIT_CODE="${1:-}"
        FG_PROCESS_TIMED_OUT="${2:-false}"
        FG_PROCESS_OUTPUT_LIMIT_EXCEEDED="${3:-false}"
        FG_PROCESS_STDOUT="${4:-}"
        FG_PROCESS_STDERR="${5:-}"
        FG_PROCESS_FAILURE_REASON="${6:-}"
    }

    flashgate_run_runtime_process() {
        local working_directory="$1"
        local timeout_seconds="$2"
        local maximum_bytes="$3"
        shift 3
        local key="${*:2}"

        case "$controlled_mode:$key" in
            nonzero-compact:--version)
                set_controlled_process_result \
                    7 false false "" "controlled failure" NonZeroExit
                return 0
                ;;
            nonzero-verbose:--version\ --verbose)
                set_controlled_process_result \
                    7 false false "" "controlled failure" NonZeroExit
                return 0
                ;;
            nonzero-help:--help)
                set_controlled_process_result \
                    7 false false "" "controlled failure" NonZeroExit
                return 0
                ;;
            timeout:--version)
                set_controlled_process_result \
                    "" true false "" "" Timeout
                return 0
                ;;
            stdout-limit:--version)
                set_controlled_process_result \
                    "" false true \
                    "$(printf '%1024s' '' | tr ' ' x)" \
                    "" \
                    StdoutLimitExceeded
                return 0
                ;;
            stderr-limit:--version)
                set_controlled_process_result \
                    "" false true \
                    "" \
                    "$(printf '%1024s' '' | tr ' ' x)" \
                    StderrLimitExceeded
                return 0
                ;;
            start-failure:--version)
                set_controlled_process_result \
                    "" false false "" "" \
                    StartOrProcessFailure:FileNotFoundError
                return 0
                ;;
            cleanup-failure:--version)
                set_controlled_process_result \
                    "" false false "" "" ProcessCleanupFailure
                return 0
                ;;
        esac

        run_flashgate_bounded_process \
            "$working_directory" \
            "$timeout_seconds" \
            "$maximum_bytes" \
            "$@"
        if [[ "$controlled_mode" == empty-help && "$key" == --help ]]; then
            FG_PROCESS_STDOUT=""
        elif [[
            "$controlled_mode" == partial-help &&
            "$key" == --help
        ]]; then
            FG_PROCESS_STDOUT="${FG_PROCESS_STDOUT%%$'\n'*}"
        fi
    }

    run_full_linux_case() {
        local mode="$1"
        local expected_runtime="$2"
        local expected_help="$3"
        local expected_runtime_reason="$4"
        local expected_help_reason="$5"
        local expected_help_skip="$6"
        local output=""
        local verifier_exit=0

        controlled_mode="$mode"
        output="$(
            flashgate_linux_metadata_main \
                --binary "$real_linux_x64" \
                --expected-public-arch x64 \
                --expected-goarch amd64 \
                "${common_arguments[@]}"
        )" || verifier_exit=$?

        [[ $verifier_exit -ne 0 ]] || return 1
        grep -q '^Status: FAIL$' <<<"$output" || return 1
        grep -q "^RuntimeExecution: $expected_runtime$" \
            <<<"$output" || return 1
        grep -q "^HelpContract: $expected_help$" \
            <<<"$output" || return 1
        if [[ -n "$expected_runtime_reason" ]]; then
            grep -q \
                "^RuntimeFailureReason: $expected_runtime_reason$" \
                <<<"$output" || return 1
        fi
        if [[ -n "$expected_help_reason" ]]; then
            grep -q "^HelpFailureReason: $expected_help_reason$" \
                <<<"$output" || return 1
        fi
        if [[ -n "$expected_help_skip" ]]; then
            grep -q "^HelpSkipReason: $expected_help_skip$" \
                <<<"$output" || return 1
        fi
        grep -Eq '^ErrorCount: [1-9][0-9]*$' <<<"$output" || return 1
        grep -Eq '^Errors: .+$' <<<"$output"
    }

    for full_case in \
        "nonzero-compact|FAIL|SKIPPED|NonZeroExit||RuntimeValidationFailed" \
        "nonzero-verbose|FAIL|SKIPPED|NonZeroExit||RuntimeValidationFailed" \
        "nonzero-help|PASS|FAIL||NonZeroExit|" \
        "empty-help|PASS|FAIL||HelpContractMismatch|" \
        "partial-help|PASS|FAIL||HelpContractMismatch|" \
        "timeout|FAIL|SKIPPED|Timeout||RuntimeValidationFailed" \
        "stdout-limit|FAIL|SKIPPED|StdoutLimitExceeded||RuntimeValidationFailed" \
        "stderr-limit|FAIL|SKIPPED|StderrLimitExceeded||RuntimeValidationFailed" \
        "start-failure|FAIL|SKIPPED|StartOrProcessFailure:FileNotFoundError||RuntimeValidationFailed" \
        "cleanup-failure|FAIL|SKIPPED|ProcessCleanupFailure||RuntimeValidationFailed"; do
        IFS='|' read -r \
            mode \
            expected_runtime \
            expected_help \
            expected_runtime_reason \
            expected_help_reason \
            expected_help_skip <<<"$full_case"
        if run_full_linux_case \
            "$mode" \
            "$expected_runtime" \
            "$expected_help" \
            "$expected_runtime_reason" \
            "$expected_help_reason" \
            "$expected_help_skip"; then
            full_case_result=true
        else
            full_case_result=false
        fi
        check "$full_case_result" "full Linux verifier aggregates $mode"
    done

    static_marker="$temporary_directory/static-launch.txt"
    FLASHGATE_VERIFIER_TEST_MODE=1 \
        bash "$root_path/scripts/Test-LinuxMetadata.sh" \
            --binary "$real_linux_x64" \
            --expected-public-arch x64 \
            --expected-goarch amd64 \
            --expected-product-version 9.9.9 \
            --expected-file-version 9.9.9.0 \
            --expected-commit "$expected_commit" \
            --expected-source-time "$expected_source_time" \
            --expected-modified "$expected_modified" \
            --test-launch-marker "$static_marker" \
            >"$temporary_directory/static.log" 2>&1
    static_exit=$?
    check "$(
        [[ $static_exit -ne 0 && ! -e "$static_marker" ]] &&
            grep -q '^Status: FAIL$' \
                "$temporary_directory/static.log" &&
            grep -q '^RuntimeExecution: SKIPPED$' \
                "$temporary_directory/static.log" &&
            grep -q '^ExecutionSkipReason: StaticValidationFailed$' \
                "$temporary_directory/static.log" &&
            grep -q '^HelpContract: SKIPPED$' \
                "$temporary_directory/static.log" &&
            grep -q '^HelpSkipReason: StaticValidationFailed$' \
                "$temporary_directory/static.log" &&
            printf true || printf false
    )" "static failure prevents launch"

    bash "$root_path/scripts/Test-LinuxMetadata.sh" \
        --binary "$temporary_directory/missing-artifact" \
        --expected-public-arch x64 \
        --expected-goarch amd64 \
        "${common_arguments[@]}" \
        >"$temporary_directory/missing.log" 2>&1
    missing_exit=$?
    check "$(
        [[ $missing_exit -ne 0 ]] &&
            grep -q '^RuntimeExecution: SKIPPED$' \
                "$temporary_directory/missing.log" &&
            printf true || printf false
    )" "missing artifact fails without execution"

    if [[ -n "$real_linux_arm64" ]]; then
        bash "$root_path/scripts/Test-LinuxMetadata.sh" \
            --binary "$real_linux_arm64" \
            --expected-public-arch arm64 \
            --expected-goarch arm64 \
            "${common_arguments[@]}" \
            >"$temporary_directory/arm64.log" 2>&1
        arm64_exit=$?
        check "$(
            [[ $arm64_exit -eq 0 ]] &&
                grep -q '^RuntimeExecution: SKIPPED$' \
                    "$temporary_directory/arm64.log" &&
                grep -q '^ExecutionSkipReason: NonNativeTarget$' \
                    "$temporary_directory/arm64.log" &&
                printf true || printf false
        )" "real Linux ARM64 static verification is skipped on x64"
    fi
fi

status=PASS
if ((${#failures[@]} > 0)); then
    status=FAIL
fi

printf 'Status: %s\n' "$status"
printf 'CaseCount: %d\n' "${#cases[@]}"
printf 'FailureCount: %d\n' "${#failures[@]}"
printf 'Failures: %s\n' "$(IFS='; '; echo "${failures[*]:-}")"

[[ "$status" == PASS ]]
