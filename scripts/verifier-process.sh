#!/usr/bin/env bash

flashgate_normalize_architecture() {
    case "${1,,}" in
        x64|amd64|x86_64)
            printf 'x64\n'
            ;;
        arm64|aarch64)
            printf 'arm64\n'
            ;;
        *)
            return 1
            ;;
    esac
}

flashgate_execution_decision() {
    local host_architecture="$1"
    local target_architecture="$2"
    local caller_requested_skip="$3"

    FG_HOST_ARCHITECTURE="$(
        flashgate_normalize_architecture "$host_architecture"
    )" || {
        FG_HOST_ARCHITECTURE="unsupported"
        FG_TARGET_ARCHITECTURE="$target_architecture"
        FG_NATIVE_EXECUTION_ELIGIBLE=false
        FG_EXECUTION_SKIP_REASON=UnsupportedHostArchitecture
        return 1
    }
    FG_TARGET_ARCHITECTURE="$target_architecture"

    if [[ "$FG_HOST_ARCHITECTURE" != "$target_architecture" ]]; then
        FG_NATIVE_EXECUTION_ELIGIBLE=false
        FG_EXECUTION_SKIP_REASON=NonNativeTarget
    elif [[ "$caller_requested_skip" == true ]]; then
        FG_NATIVE_EXECUTION_ELIGIBLE=false
        FG_EXECUTION_SKIP_REASON=CallerRequestedSkip
    else
        FG_NATIVE_EXECUTION_ELIGIBLE=true
        FG_EXECUTION_SKIP_REASON=""
    fi
}

flashgate_execution_state() {
    local attempted="$1"
    local process_status="$2"
    if [[ "$attempted" != true ]]; then
        printf 'SKIPPED\n'
    elif [[ "$process_status" == PASS ]]; then
        printf 'PASS\n'
    else
        printf 'FAIL\n'
    fi
}

flashgate_missing_help_lines() {
    local output="$1"
    local expected_line
    for expected_line in \
        "flashgate-mcp" \
        "Usage:" \
        "flashgate-mcp --version" \
        "flashgate-mcp --version --verbose" \
        "flashgate-mcp --help" \
        "MCP_ROOT" \
        "MCP_READ_ONLY" \
        "MCP_ALLOW_CWD_ROOT"; do
        [[ "$output" == *"$expected_line"* ]] ||
            printf '%s\n' "$expected_line"
    done
}

run_flashgate_bounded_process() {
    local working_directory="$1"
    local timeout_seconds="$2"
    local maximum_bytes="$3"
    shift 3

    local helper_path="$FG_VERIFIER_ROOT/scripts/bounded-process-runner.py"
    local temporary_directory=""
    local stdout_file=""
    local stderr_file=""
    local runner_metadata=""
    local runner_exit=0

    FG_PROCESS_ATTEMPTED=true
    FG_PROCESS_STATUS=FAIL
    FG_PROCESS_EXIT_CODE=""
    FG_PROCESS_TIMED_OUT=false
    FG_PROCESS_OUTPUT_LIMIT_EXCEEDED=false
    FG_PROCESS_STDOUT=""
    FG_PROCESS_STDERR=""
    FG_PROCESS_FAILURE_REASON=""

    if [[ ! -f "$helper_path" ]]; then
        FG_PROCESS_FAILURE_REASON=RunnerNotFound
        return 0
    fi
    if ! command -v python3 >/dev/null 2>&1; then
        FG_PROCESS_FAILURE_REASON=Python3NotFound
        return 0
    fi
    if ! temporary_directory="$(mktemp -d)"; then
        FG_PROCESS_FAILURE_REASON=TemporaryDirectoryFailure
        return 0
    fi
    stdout_file="$temporary_directory/stdout.bin"
    stderr_file="$temporary_directory/stderr.bin"

    runner_metadata="$(
        python3 "$helper_path" \
            --timeout-seconds "$timeout_seconds" \
            --maximum-bytes "$maximum_bytes" \
            --stdout-file "$stdout_file" \
            --stderr-file "$stderr_file" \
            --working-directory "$working_directory" \
            -- "$@"
    )" || runner_exit=$?

    if ((runner_exit != 0)); then
        FG_PROCESS_FAILURE_REASON=RunnerFailure
    elif ! IFS=$'\t' read -r \
        FG_PROCESS_ATTEMPTED \
        FG_PROCESS_STATUS \
        FG_PROCESS_EXIT_CODE \
        FG_PROCESS_TIMED_OUT \
        FG_PROCESS_OUTPUT_LIMIT_EXCEEDED \
        FG_PROCESS_FAILURE_REASON <<<"$runner_metadata"; then
        FG_PROCESS_FAILURE_REASON=RunnerProtocolFailure
    fi
    if [[ "$FG_PROCESS_EXIT_CODE" == none ]]; then
        FG_PROCESS_EXIT_CODE=""
    fi
    if [[ "$FG_PROCESS_FAILURE_REASON" == None ]]; then
        FG_PROCESS_FAILURE_REASON=""
    fi

    if [[ -f "$stdout_file" ]]; then
        FG_PROCESS_STDOUT="$(<"$stdout_file")"
    fi
    if [[ -f "$stderr_file" ]]; then
        FG_PROCESS_STDERR="$(<"$stderr_file")"
    fi
    rm -f -- "$stdout_file" "$stderr_file"
    rmdir -- "$temporary_directory"
}
