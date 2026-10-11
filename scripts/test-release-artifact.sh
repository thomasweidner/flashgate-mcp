#!/usr/bin/env bash
set -uo pipefail

root_path="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
# shellcheck source=build-input-validation.sh
source "$root_path/scripts/build-input-validation.sh"
archive_path=""
checksum_path=""
expected_version=""
expected_public_arch=""
expected_goarch=""
expected_commit=""
expected_source_time=""
expected_modified=""
errors=()
warnings=()
extraction_root=""
inventory_report=""
archive_sha256=""

add_error() { errors+=("$1"); }

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
cleanup() { [[ -z "$extraction_root" || ! -d "$extraction_root" ]] || rm -rf -- "$extraction_root"; }
trap cleanup EXIT

while (($# > 0)); do
    case "$1" in
        --archive) archive_path="$2"; shift 2 ;;
        --checksum) checksum_path="$2"; shift 2 ;;
        --expected-version) expected_version="$2"; shift 2 ;;
        --expected-public-arch) expected_public_arch="$2"; shift 2 ;;
        --expected-goarch) expected_goarch="$2"; shift 2 ;;
        --expected-commit) expected_commit="$2"; shift 2 ;;
        --expected-source-time) expected_source_time="$2"; shift 2 ;;
        --expected-modified) expected_modified="$2"; shift 2 ;;
        --help|-h)
            printf 'Usage: scripts/test-release-artifact.sh --archive PATH --checksum PATH --expected-version VERSION --expected-public-arch x64|arm64 --expected-goarch amd64|arm64 --expected-commit SHA40 --expected-source-time RFC3339 --expected-modified true|false\n'
            exit 0 ;;
        *) add_error "unknown argument: $1"; shift ;;
    esac
done

for value in "$archive_path" "$checksum_path" "$expected_version" "$expected_public_arch" "$expected_goarch" "$expected_commit" "$expected_source_time" "$expected_modified"; do
    [[ -n "$value" ]] || add_error "one or more required values are missing"
done
[[ "$expected_public_arch" == "x64" || "$expected_public_arch" == "arm64" ]] || add_error "invalid public architecture"
[[ "$expected_goarch" == "amd64" || "$expected_goarch" == "arm64" ]] || add_error "invalid GOARCH"
[[ "$expected_modified" == "true" || "$expected_modified" == "false" ]] || add_error "invalid modified value"
flashgate_validate_semver "$expected_version" || add_error "invalid expected version"

if ! resolve_go_toolchain; then
    add_error "required command not found: go"
fi

for command_name in go tar gzip sha256sum mktemp python3 cp; do
    command -v "$command_name" >/dev/null 2>&1 || add_error "required command not found: $command_name"
done
[[ -f "$archive_path" ]] || add_error "archive not found: $archive_path"
[[ -f "$checksum_path" ]] || add_error "checksum not found: $checksum_path"
[[ -x "$root_path/scripts/Test-LinuxMetadata.sh" || -f "$root_path/scripts/Test-LinuxMetadata.sh" ]] || add_error "Linux metadata validator not found"

expected_base_name="flashgate-mcp_${expected_version}_linux_${expected_public_arch}"
expected_archive_name="${expected_base_name}.tar.gz"
[[ "$(basename "$archive_path")" == "$expected_archive_name" ]] || add_error "unexpected archive filename"
[[ "$(basename "$checksum_path")" == "${expected_archive_name}.sha256" ]] ||
    add_error "unexpected checksum filename"

if ((${#errors[@]} == 0)); then
    extraction_root="$(mktemp -d)"
    validated_archive="$extraction_root/$expected_archive_name"
    validated_checksum="$extraction_root/${expected_archive_name}.sha256"
    cp -- "$archive_path" "$validated_archive" ||
        add_error "unable to copy archive into the controlled validation directory"
    cp -- "$checksum_path" "$validated_checksum" ||
        add_error "unable to copy checksum into the controlled validation directory"
fi

if ((${#errors[@]} == 0)); then
    if ! (
        cd "$extraction_root"
        sha256sum -c "$(basename "$validated_checksum")" >/dev/null
    ); then
        add_error "checksum verification failed"
    fi
fi

actual_entries=""
if ((${#errors[@]} == 0)); then
    if ! archive_digest="$(sha256sum -- "$validated_archive")"; then
        add_error "unable to hash the controlled validation archive"
    else
        archive_sha256="${archive_digest%% *}"
        if [[ ! "$archive_sha256" =~ ^[0-9a-f]{64}$ ]]; then
            archive_sha256=""
            add_error "invalid SHA-256 for the controlled validation archive"
        fi
    fi
fi

if ((${#errors[@]} == 0)); then
    if ! actual_entries="$(tar -tzf "$validated_archive")"; then
        add_error "unable to list archive"
    fi
fi

if [[ -n "$actual_entries" ]]; then
    while IFS= read -r entry; do
        [[ "$entry" != /* && "$entry" != *".."* && "$entry" != *'\\'* ]] || add_error "unsafe archive entry: $entry"
    done <<<"$actual_entries"

    expected_entries="$(printf '%s\n' \
        "$expected_base_name/" \
        "$expected_base_name/LICENSE" \
        "$expected_base_name/README.md" \
        "$expected_base_name/THIRD-PARTY-NOTICES.md" \
        "$expected_base_name/flashgate-mcp" | sort)"
    sorted_actual_entries="$(printf '%s\n' "$actual_entries" | sort)"
    [[ "$sorted_actual_entries" == "$expected_entries" ]] || add_error "unexpected archive content"
fi

if ((${#errors[@]} == 0)); then
    inventory_report="$extraction_root/inventory.json"
    if ! go -C "$root_path" run -mod=vendor ./cmd/releaseaudit inventory \
        --artifact "$validated_archive" \
        --report "$inventory_report"; then
        add_error "archive type and path audit failed"
    elif ! python3 - "$inventory_report" "$expected_base_name" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as stream:
    report = json.load(stream)
base = sys.argv[2]
expected = {
    base: "directory",
    f"{base}/LICENSE": "file",
    f"{base}/README.md": "file",
    f"{base}/THIRD-PARTY-NOTICES.md": "file",
    f"{base}/flashgate-mcp": "file",
}
actual = {entry["path"]: entry["type"] for entry in report["entries"]}
if report["status"] != "PASS" or actual != expected:
    raise SystemExit("release inventory or entry types differ")
PY
    then
        add_error "archive entry types do not match the release contract"
    fi
fi

if ((${#errors[@]} == 0)); then
    mkdir -p "$extraction_root/extracted"
    tar \
        --extract \
        --gzip \
        --file "$validated_archive" \
        --directory "$extraction_root/extracted" \
        --no-same-owner \
        --no-same-permissions ||
        add_error "archive extraction failed"
    artifact_root="$extraction_root/extracted/$expected_base_name"
    binary_path="$artifact_root/flashgate-mcp"

    for required_file in LICENSE README.md THIRD-PARTY-NOTICES.md flashgate-mcp; do
        [[ -s "$artifact_root/$required_file" ]] || add_error "extracted file is missing or empty: $required_file"
    done

    metadata_arguments=(
        --binary "$binary_path"
        --expected-product-version "$expected_version"
        --expected-file-version "$FLASHGATE_FILE_VERSION"
        --expected-public-arch "$expected_public_arch"
        --expected-goarch "$expected_goarch"
        --expected-commit "$expected_commit"
        --expected-source-time "$expected_source_time"
        --expected-modified "$expected_modified"
    )
    [[ "$expected_goarch" == "arm64" ]] && metadata_arguments+=(--skip-execution)

    if ! bash "$root_path/scripts/Test-LinuxMetadata.sh" "${metadata_arguments[@]}"; then
        add_error "Linux metadata validation failed"
    fi
fi

status="PASS"
((${#errors[@]} == 0)) || status="FAIL"
printf 'Status: %s\n' "$status"
printf 'ArchivePath: %s\n' "$archive_path"
printf 'ChecksumPath: %s\n' "$checksum_path"
printf 'Version: %s\n' "$expected_version"
printf 'PublicArch: %s\n' "$expected_public_arch"
printf 'Sha256: %s\n' "$archive_sha256"
printf 'WarningCount: %d\n' "${#warnings[@]}"
printf 'ErrorCount: %d\n' "${#errors[@]}"
printf 'Warnings: %s\n' "$(IFS='; '; echo "${warnings[*]:-}")"
printf 'Errors: %s\n' "$(IFS='; '; echo "${errors[*]:-}")"
[[ "$status" == "PASS" ]]
