#!/usr/bin/env bash
set -uo pipefail

root_path="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
# shellcheck source=build-input-validation.sh
source "$root_path/scripts/build-input-validation.sh"
# shellcheck source=verifier-process.sh
source "$root_path/scripts/verifier-process.sh"
FG_VERIFIER_ROOT="$root_path"

flashgate_run_runtime_process() {
    run_flashgate_bounded_process "$@"
}

flashgate_linux_metadata_main() {
binary_path=""
expected_product_version=""
expected_file_version=""
expected_public_arch=""
expected_goarch=""
expected_commit=""
expected_source_time=""
expected_modified=""
skip_execution=false
require_vcs=true
require_go_note=true
artifact_timeout_seconds=10
tool_timeout_seconds=120
maximum_output_bytes=65536
test_launch_marker=""

errors=()
warnings=()

usage() {
    cat <<'EOF'
Usage:
  scripts/Test-LinuxMetadata.sh [options]

Required:
  --binary PATH
  --expected-product-version VERSION
  --expected-file-version VERSION
  --expected-public-arch x64|arm64
  --expected-goarch amd64|arm64
  --expected-commit SHA40
  --expected-source-time RFC3339
  --expected-modified true|false

Optional:
  --skip-execution
  --allow-missing-vcs
  --allow-missing-go-note
  --artifact-timeout-seconds 1..60
  --tool-timeout-seconds 10..300
  --maximum-output-bytes 1024..1048576
  --help
EOF
}

add_error() {
    errors+=("$1")
}

run_static_command() {
    local output_variable="$1"
    local description="$2"
    shift 2

    run_flashgate_bounded_process \
        "$root_path" \
        "$tool_timeout_seconds" \
        "$maximum_output_bytes" \
        "$@"
    if [[ "$FG_PROCESS_STATUS" != PASS ]]; then
        add_error \
            "$description failed safely: $FG_PROCESS_FAILURE_REASON"
        printf -v "$output_variable" '%s' ""
    else
        printf -v "$output_variable" '%s' "$FG_PROCESS_STDOUT"
    fi
}

while (($# > 0)); do
    case "$1" in
        --binary)
            (($# >= 2)) || { add_error "--binary requires a value"; break; }
            binary_path="$2"
            shift 2
            ;;
        --expected-product-version)
            (($# >= 2)) || { add_error "--expected-product-version requires a value"; break; }
            expected_product_version="$2"
            shift 2
            ;;
        --expected-file-version)
            (($# >= 2)) || { add_error "--expected-file-version requires a value"; break; }
            expected_file_version="$2"
            shift 2
            ;;
        --expected-public-arch)
            (($# >= 2)) || { add_error "--expected-public-arch requires a value"; break; }
            expected_public_arch="$2"
            shift 2
            ;;
        --expected-goarch)
            (($# >= 2)) || { add_error "--expected-goarch requires a value"; break; }
            expected_goarch="$2"
            shift 2
            ;;
        --expected-commit)
            (($# >= 2)) || { add_error "--expected-commit requires a value"; break; }
            expected_commit="$2"
            shift 2
            ;;
        --expected-source-time)
            (($# >= 2)) || { add_error "--expected-source-time requires a value"; break; }
            expected_source_time="$2"
            shift 2
            ;;
        --expected-modified)
            (($# >= 2)) || { add_error "--expected-modified requires a value"; break; }
            expected_modified="$2"
            shift 2
            ;;
        --skip-execution)
            skip_execution=true
            shift
            ;;
        --allow-missing-vcs)
            require_vcs=false
            shift
            ;;
        --allow-missing-go-note)
            require_go_note=false
            shift
            ;;
        --artifact-timeout-seconds)
            (($# >= 2)) || { add_error "--artifact-timeout-seconds requires a value"; break; }
            artifact_timeout_seconds="$2"
            shift 2
            ;;
        --tool-timeout-seconds)
            (($# >= 2)) || { add_error "--tool-timeout-seconds requires a value"; break; }
            tool_timeout_seconds="$2"
            shift 2
            ;;
        --maximum-output-bytes)
            (($# >= 2)) || { add_error "--maximum-output-bytes requires a value"; break; }
            maximum_output_bytes="$2"
            shift 2
            ;;
        --test-launch-marker)
            (($# >= 2)) || { add_error "--test-launch-marker requires a value"; break; }
            test_launch_marker="$2"
            shift 2
            ;;
        --help|-h)
            usage
            return 0
            ;;
        *)
            add_error "unknown argument: $1"
            shift
            ;;
    esac
done

for required_value in \
    "$binary_path" \
    "$expected_product_version" \
    "$expected_file_version" \
    "$expected_public_arch" \
    "$expected_goarch" \
    "$expected_commit" \
    "$expected_source_time" \
    "$expected_modified"; do
    [[ -n "$required_value" ]] || add_error "one or more required values are missing"
done

for command_name in file readelf go python3 uname; do
    command -v "$command_name" >/dev/null 2>&1 ||
        add_error "required command not found: $command_name"
done

[[ -f "$binary_path" ]] || add_error "binary not found: $binary_path"
[[ "$expected_goarch" == "amd64" || "$expected_goarch" == "arm64" ]] ||
    add_error "unsupported expected GOARCH: $expected_goarch"
[[ "$expected_public_arch" == "x64" || "$expected_public_arch" == "arm64" ]] ||
    add_error "unsupported expected public architecture: $expected_public_arch"
[[ "$expected_modified" == "true" || "$expected_modified" == "false" ]] ||
    add_error "expected modified value must be true or false"
[[ "$artifact_timeout_seconds" =~ ^[0-9]+$ ]] &&
    ((artifact_timeout_seconds >= 1 && artifact_timeout_seconds <= 60)) ||
    add_error "artifact timeout must be an integer from 1 through 60"
[[ "$tool_timeout_seconds" =~ ^[0-9]+$ ]] &&
    ((tool_timeout_seconds >= 10 && tool_timeout_seconds <= 300)) ||
    add_error "tool timeout must be an integer from 10 through 300"
[[ "$maximum_output_bytes" =~ ^[0-9]+$ ]] &&
    ((maximum_output_bytes >= 1024 && maximum_output_bytes <= 1048576)) ||
    add_error "maximum output bytes must be from 1024 through 1048576"
if [[ -n "$test_launch_marker" &&
    "${FLASHGATE_VERIFIER_TEST_MODE:-}" != 1 ]]; then
    add_error "the launch marker is available only in verifier test mode"
fi
if ! flashgate_validate_semver "$expected_product_version"; then
    add_error "expected product version is invalid"
elif [[ "$FLASHGATE_FILE_VERSION" != "$expected_file_version" ]]; then
    add_error "expected product/file version mapping is inconsistent"
fi

file_output=""
elf_header=""
elf_notes=""
go_build_info=""
go_build_id=""
static_manifest_output=""
compact_output=""
verbose_output=""
help_output=""
runtime_execution="SKIPPED"
runtime_failure_reason=""
help_contract="SKIPPED"
help_failure_reason=""
host_architecture="unsupported"
target_architecture="$expected_public_arch"
native_execution_eligible=false
execution_skip_reason="NotEvaluated"
help_skip_reason="NotEvaluated"

if ((${#errors[@]} == 0)); then
    run_static_command file_output "file inspection" \
        file -b "$binary_path"
    run_static_command elf_header "ELF header inspection" \
        readelf -h "$binary_path"
    run_static_command elf_notes "ELF note inspection" \
        readelf -n "$binary_path"
    run_static_command go_build_info "Go build-information inspection" \
        go version -m "$binary_path"
    run_static_command go_build_id "Go build-ID inspection" \
        go tool buildid "$binary_path"
    run_static_command static_manifest_output \
        "static build-manifest validation" \
        go -C "$root_path" run -mod=vendor ./cmd/versionmanifest \
            --binary "$binary_path" \
            --expected-version "$expected_product_version" \
            --expected-file-version "$expected_file_version" \
            --expected-commit "$expected_commit" \
            --expected-source-time "$expected_source_time" \
            --expected-modified "$expected_modified" \
            --expected-goos linux \
            --expected-goarch "$expected_goarch" \
            --expected-public-arch "$expected_public_arch"
fi

if [[ -n "$file_output" ]]; then
    [[ "$file_output" == *"ELF 64-bit"* ]] ||
        add_error "binary is not a 64-bit ELF file: $file_output"
    [[ "$file_output" == *"statically linked"* ]] ||
        add_error "binary is not statically linked: $file_output"
fi

case "$expected_goarch" in
    amd64)
        [[ "$elf_header" == *"Advanced Micro Devices X86-64"* ]] ||
            add_error "ELF machine is not x86-64"
        ;;
    arm64)
        [[ "$elf_header" == *"AArch64"* ]] ||
            add_error "ELF machine is not AArch64"
        ;;
esac

if [[ "$require_go_note" == true ]]; then
    [[ "$elf_notes" == *".note.go.buildid"* ]] ||
        add_error "ELF .note.go.buildid section is missing"
    [[ "$elf_notes" == *"GO BUILDID"* ]] ||
        add_error "ELF Go build-ID note is missing"
    [[ -n "$go_build_id" ]] ||
        add_error "Go build ID is empty"
fi

for expected_build_setting in \
    "path"$'\t'"github.com/thomasweidner/flashgate-mcp/cmd/server" \
    "build"$'\t'"GOOS=linux" \
    "build"$'\t'"GOARCH=${expected_goarch}" \
    "build"$'\t'"CGO_ENABLED=0" \
    "build"$'\t'"-trimpath=true"; do
    [[ "$go_build_info" == *"$expected_build_setting"* ]] ||
        add_error "go version -m is missing: $expected_build_setting"
done

if [[ "$require_vcs" == true ]]; then
    for expected_vcs_setting in \
        "build"$'\t'"vcs=git" \
        "build"$'\t'"vcs.revision=${expected_commit}" \
        "build"$'\t'"vcs.time=${expected_source_time}" \
        "build"$'\t'"vcs.modified=${expected_modified}"; do
        [[ "$go_build_info" == *"$expected_vcs_setting"* ]] ||
            add_error "go version -m is missing VCS setting: $expected_vcs_setting"
    done
fi

raw_host_architecture=""
if command -v uname >/dev/null 2>&1; then
    run_static_command raw_host_architecture "host architecture detection" \
        uname -m
fi
if flashgate_execution_decision \
    "$raw_host_architecture" \
    "$expected_public_arch" \
    "$skip_execution"; then
    host_architecture="$FG_HOST_ARCHITECTURE"
    target_architecture="$FG_TARGET_ARCHITECTURE"
    native_execution_eligible="$FG_NATIVE_EXECUTION_ELIGIBLE"
    execution_skip_reason="$FG_EXECUTION_SKIP_REASON"
else
    host_architecture="$FG_HOST_ARCHITECTURE"
    target_architecture="$FG_TARGET_ARCHITECTURE"
    native_execution_eligible=false
    execution_skip_reason="$FG_EXECUTION_SKIP_REASON"
    add_error "the Linux host architecture is unsupported"
fi

if ((${#errors[@]} > 0)); then
    execution_skip_reason=StaticValidationFailed
    help_skip_reason=StaticValidationFailed
elif [[ "$native_execution_eligible" != true ]]; then
    help_skip_reason="$execution_skip_reason"
else
    execution_skip_reason=""
    help_skip_reason=""
    runtime_error_count=${#errors[@]}
    runtime_execution=FAIL
    if [[ -n "$test_launch_marker" ]]; then
        printf 'runtime-attempted' >"$test_launch_marker"
    fi
    flashgate_run_runtime_process \
        "$root_path" \
        "$artifact_timeout_seconds" \
        "$maximum_output_bytes" \
        "$binary_path" --version
    runtime_execution="$(
        flashgate_execution_state \
            "$FG_PROCESS_ATTEMPTED" \
            "$FG_PROCESS_STATUS"
    )"
    if [[ "$FG_PROCESS_STATUS" != PASS ]]; then
        runtime_failure_reason="$FG_PROCESS_FAILURE_REASON"
        add_error \
            "compact version execution failed safely: $FG_PROCESS_FAILURE_REASON"
    else
        compact_output="$FG_PROCESS_STDOUT"
        [[ "$compact_output" == "flashgate-mcp $expected_product_version" ]] || {
            runtime_failure_reason=CompactOutputMismatch
            add_error "unexpected compact version output"
        }
    fi

    if ((${#errors[@]} == runtime_error_count)); then
        flashgate_run_runtime_process \
            "$root_path" \
            "$artifact_timeout_seconds" \
            "$maximum_output_bytes" \
            "$binary_path" --version --verbose
        if [[ "$FG_PROCESS_STATUS" != PASS ]]; then
            runtime_failure_reason="$FG_PROCESS_FAILURE_REASON"
            add_error \
                "verbose version execution failed safely: $FG_PROCESS_FAILURE_REASON"
        else
            verbose_output="$FG_PROCESS_STDOUT"
            for expected_line in \
                "Product:      FlashGate MCP" \
                "Version:      $expected_product_version" \
                "File version: $expected_file_version" \
                "Commit:       $expected_commit" \
                "Source time:  $expected_source_time" \
                "Modified:     $expected_modified" \
                "Platform:     linux/$expected_public_arch" \
                "Go target:    linux/$expected_goarch"; do
                [[ "$verbose_output" == *"$expected_line"* ]] ||
                {
                    runtime_failure_reason=VerboseOutputMismatch
                    add_error "verbose output is missing: $expected_line"
                }
            done
        fi
    fi
    if ((${#errors[@]} == runtime_error_count)); then
        runtime_execution="PASS"
    else
        runtime_execution="FAIL"
    fi

    if [[ "$runtime_execution" == PASS ]]; then
        help_error_count=${#errors[@]}
        help_contract=FAIL
        flashgate_run_runtime_process \
            "$root_path" \
            "$artifact_timeout_seconds" \
            "$maximum_output_bytes" \
            "$binary_path" --help
        help_contract="$(
            flashgate_execution_state \
                "$FG_PROCESS_ATTEMPTED" \
                "$FG_PROCESS_STATUS"
        )"
        if [[ "$FG_PROCESS_STATUS" != PASS ]]; then
            help_failure_reason="$FG_PROCESS_FAILURE_REASON"
            add_error \
                "help execution failed safely: $FG_PROCESS_FAILURE_REASON"
        else
            help_output="$FG_PROCESS_STDOUT"
            mapfile -t missing_help_lines < <(
                flashgate_missing_help_lines "$help_output"
            )
            for missing_help_line in "${missing_help_lines[@]}"; do
                help_failure_reason=HelpContractMismatch
                add_error "help output is missing: $missing_help_line"
            done
        fi
        if ((${#errors[@]} == help_error_count)); then
            help_contract="PASS"
        else
            help_contract="FAIL"
        fi
    else
        help_skip_reason=RuntimeValidationFailed
    fi
fi

if ((${#errors[@]} == 0)); then
    status="PASS"
else
    status="FAIL"
fi

printf 'Status: %s\n' "$status"
printf 'BinaryPath: %s\n' "$binary_path"
printf 'ExpectedProductVersion: %s\n' "$expected_product_version"
printf 'ExpectedFileVersion: %s\n' "$expected_file_version"
printf 'ExpectedPublicArch: %s\n' "$expected_public_arch"
printf 'ExpectedGOARCH: %s\n' "$expected_goarch"
printf 'ExpectedCommit: %s\n' "$expected_commit"
printf 'ExpectedSourceTime: %s\n' "$expected_source_time"
printf 'ExpectedModified: %s\n' "$expected_modified"
printf 'FileDescription: %s\n' "$file_output"
printf 'GoBuildID: %s\n' "$go_build_id"
printf 'HostArchitecture: %s\n' "$host_architecture"
printf 'TargetArchitecture: %s\n' "$target_architecture"
printf 'NativeExecutionEligible: %s\n' "$native_execution_eligible"
printf 'ExecutionSkipReason: %s\n' "$execution_skip_reason"
printf 'RuntimeExecution: %s\n' "$runtime_execution"
printf 'RuntimeFailureReason: %s\n' "$runtime_failure_reason"
printf 'HelpContract: %s\n' "$help_contract"
printf 'HelpSkipReason: %s\n' "$help_skip_reason"
printf 'HelpFailureReason: %s\n' "$help_failure_reason"
printf 'WarningCount: %d\n' "${#warnings[@]}"
printf 'ErrorCount: %d\n' "${#errors[@]}"
printf 'Warnings: %s\n' "$(IFS='; '; echo "${warnings[*]:-}")"
printf 'Errors: %s\n' "$(IFS='; '; echo "${errors[*]:-}")"

if [[ "$status" != "PASS" ]]; then
    return 1
fi
return 0
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    flashgate_linux_metadata_main "$@"
fi
