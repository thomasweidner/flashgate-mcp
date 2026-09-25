#!/usr/bin/env bash
set -Eeuo pipefail

root_path="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
validation_script="$root_path/scripts/build-input-validation.sh"
version=""
goarch=""
output_directory=""
release=false
status="FAIL"
archive_path=""
checksum_path=""
sha256=""
error_message=""
work_directory=""

print_result() {
    printf 'Status: %s\n' "$status"
    printf 'Version: %s\n' "$version"
    printf 'GOARCH: %s\n' "$goarch"
    printf 'ArchivePath: %s\n' "$archive_path"
    printf 'ChecksumPath: %s\n' "$checksum_path"
    printf 'Sha256: %s\n' "$sha256"
    printf 'WarningCount: 0\n'
    if [[ "$status" == "PASS" ]]; then
        printf 'ErrorCount: 0\nWarnings:\nErrors:\n'
    else
        printf 'ErrorCount: 1\nWarnings:\nErrors: %s\n' "$error_message"
    fi
}

fail() {
    error_message="$1"
    print_result
    exit 1
}

resolve_go_toolchain() {
    if command -v go >/dev/null 2>&1; then
        return 0
    fi

    local candidate
    local candidate_directory
    local -a candidates=()

    shopt -s nullglob
    candidates+=(
        "$HOME"/.local/go-*/bin/go
        "$HOME"/.local/go/bin/go
        /usr/local/go/bin/go
    )
    shopt -u nullglob

    for candidate in "${candidates[@]}"; do
        if [[ -x "$candidate" ]]; then
            candidate_directory="$(cd "$(dirname "$candidate")" && pwd -P)"
            export PATH="$candidate_directory:$PATH"
        fi
    done

    command -v go >/dev/null 2>&1
}

cleanup() {
    if [[ -n "$work_directory" && -d "$work_directory" ]]; then
        rm -rf -- "$work_directory"
    fi
}
trap cleanup EXIT
trap 'fail "command failed at line $LINENO with exit code $?"' ERR

while (($# > 0)); do
    case "$1" in
        --version)
            (($# >= 2)) || fail "--version requires a value"
            version="$2"; shift 2 ;;
        --goarch)
            (($# >= 2)) || fail "--goarch requires a value"
            goarch="$2"; shift 2 ;;
        --output-directory)
            (($# >= 2)) || fail "--output-directory requires a value"
            output_directory="$2"; shift 2 ;;
        --release)
            release=true; shift ;;
        --help|-h)
            printf 'Usage: scripts/new-release-artifact.sh --version SEMVER --goarch amd64|arm64 --output-directory PATH [--release]\n'
            exit 0 ;;
        *) fail "unknown argument: $1" ;;
    esac
done

[[ -f "$validation_script" ]] || fail "input validation helper not found: $validation_script"
# shellcheck source=build-input-validation.sh
source "$validation_script"
flashgate_validate_semver "$version" || fail "invalid semantic version: $version"
if [[ "$release" == true ]]; then
    flashgate_read_repository_version "$root_path" ||
        fail "invalid or missing repository VERSION"
    [[ "$version" == "$FLASHGATE_REPOSITORY_VERSION" ]] ||
        fail "release version assertion differs from repository VERSION"
fi
[[ "$goarch" == "amd64" || "$goarch" == "arm64" ]] || fail "unsupported GOARCH: $goarch"
[[ -n "$output_directory" ]] || fail "--output-directory is required"

resolve_go_toolchain || fail "required command not found: go"

for command_name in go git tar gzip sha256sum date; do
    command -v "$command_name" >/dev/null 2>&1 || fail "required command not found: $command_name"
done

for required_file in LICENSE README.md THIRD-PARTY-NOTICES.md scripts/build.sh; do
    [[ -s "$root_path/$required_file" ]] || fail "required release file is missing or empty: $root_path/$required_file"
done

if [[ "$output_directory" != /* ]]; then
    output_directory="$root_path/$output_directory"
fi
mkdir -p -- "$output_directory"
output_directory="$(cd "$output_directory" && pwd -P)"

public_arch="$goarch"
[[ "$goarch" == "amd64" ]] && public_arch="x64"
artifact_base_name="flashgate-mcp_${version}_linux_${public_arch}"
work_directory="$output_directory/.${artifact_base_name}.work"
stage_directory="$work_directory/$artifact_base_name"
archive_path="$output_directory/${artifact_base_name}.tar.gz"
checksum_path="${archive_path}.sha256"

rm -rf -- "$work_directory"
rm -f -- "$archive_path" "$checksum_path"
mkdir -p -- "$stage_directory"

source_epoch="${SOURCE_DATE_EPOCH:-}"
if [[ -z "$source_epoch" ]]; then
    source_epoch="$(git -C "$root_path" show -s --format=%ct HEAD)"
fi
flashgate_validate_source_date_epoch "$source_epoch" ||
    fail "invalid or unsupported source epoch"

build_arguments=(
    --goos linux
    --goarch "$goarch"
    --version "$version"
    --output "$stage_directory/flashgate-mcp"
)
[[ "$release" == true ]] && build_arguments+=(--release)

SOURCE_DATE_EPOCH="$source_epoch" \
    bash "$root_path/scripts/build.sh" "${build_arguments[@]}"

for required_file in LICENSE README.md THIRD-PARTY-NOTICES.md; do
    cp -- "$root_path/$required_file" "$stage_directory/$required_file"
done

find "$stage_directory" -type f -exec touch -d "@$source_epoch" {} +
(
    cd "$work_directory"
    tar \
        --sort=name \
        --mtime="@$source_epoch" \
        --owner=0 \
        --group=0 \
        --numeric-owner \
        --format=ustar \
        -cf - \
        "$artifact_base_name" |
        gzip -n >"$archive_path"
)

(
    cd "$output_directory"
    sha256sum "$(basename "$archive_path")" >"$(basename "$checksum_path")"
)
sha256="$(sha256sum "$archive_path" | awk '{print $1}')"

status="PASS"
trap - ERR
print_result
