#!/usr/bin/env bash
set -Eeuo pipefail

root_path="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
validation_script="$root_path/scripts/build-input-validation.sh"
goos="linux"
goarch="amd64"
version=""
output_path=""
release=false
repository_preflight_only=false

status="FAIL"
file_version=""
commit=""
source_time=""
modified="false"
public_arch=""
error_message=""

print_result() {
    printf 'Status: %s\n' "$status"
    printf 'RootPath: %s\n' "$root_path"
    printf 'GOOS: %s\n' "$goos"
    printf 'GOARCH: %s\n' "$goarch"
    printf 'PublicArch: %s\n' "$public_arch"
    printf 'Version: %s\n' "$version"
    printf 'FileVersion: %s\n' "$file_version"
    printf 'Commit: %s\n' "$commit"
    printf 'SourceTime: %s\n' "$source_time"
    printf 'Modified: %s\n' "$modified"
    printf 'OutputPath: %s\n' "$output_path"
    printf 'WarningCount: 0\n'
    if [[ "$status" == "PASS" ]]; then
        printf 'ErrorCount: 0\n'
        printf 'Warnings:\n'
        printf 'Errors:\n'
    else
        printf 'ErrorCount: 1\n'
        printf 'Warnings:\n'
        printf 'Errors: %s\n' "$error_message"
    fi
}

fail() {
    error_message="$1"
    print_result
    exit 1
}

on_error() {
    local exit_code="$1"
    local line_number="$2"
    if [[ -z "$error_message" ]]; then
        error_message="command failed at line ${line_number} with exit code ${exit_code}"
    fi
    print_result
    exit "$exit_code"
}
trap 'on_error "$?" "$LINENO"' ERR

usage() {
    cat <<'EOF'
Usage:
  scripts/build.sh [options]

Options:
  --goos linux
  --goarch amd64|arm64
  --version SEMVER
  --output PATH
  --release
  --repository-preflight-only
  --help
EOF
}

while (($# > 0)); do
    case "$1" in
        --goos)
            (($# >= 2)) || fail "--goos requires a value"
            goos="$2"
            shift 2
            ;;
        --goarch)
            (($# >= 2)) || fail "--goarch requires a value"
            goarch="$2"
            shift 2
            ;;
        --version)
            (($# >= 2)) || fail "--version requires a value"
            version="$2"
            shift 2
            ;;
        --output)
            (($# >= 2)) || fail "--output requires a value"
            output_path="$2"
            shift 2
            ;;
        --release)
            release=true
            shift
            ;;
        --repository-preflight-only)
            repository_preflight_only=true
            shift
            ;;
        --help|-h)
            usage
            exit 0
            ;;
        *)
            fail "unknown argument: $1"
            ;;
    esac
done

[[ "$goos" == "linux" ]] || fail "unsupported GOOS: $goos"
[[ "$goarch" == "amd64" || "$goarch" == "arm64" ]] || fail "unsupported GOARCH: $goarch"

public_arch="$goarch"
if [[ "$goarch" == "amd64" ]]; then
    public_arch="x64"
fi

for command_name in git go date; do
    command -v "$command_name" >/dev/null 2>&1 || fail "required command not found: $command_name"
done

[[ -f "$validation_script" ]] || fail "input validation helper not found: $validation_script"
# shellcheck source=build-input-validation.sh
source "$validation_script"

inside_work_tree="$(
    git -C "$root_path" rev-parse --is-inside-work-tree 2>/dev/null
)" || fail "Git repository not found or invalid: $root_path"
[[ "$inside_work_tree" == "true" ]] ||
    fail "path is not inside a Git working tree: $root_path"

repository_top="$(
    git -C "$root_path" rev-parse --path-format=absolute --show-toplevel 2>/dev/null
)" || fail "unable to resolve Git working-tree root: $root_path"
repository_top="$(cd "$repository_top" && pwd -P)"
[[ "$repository_top" == "$root_path" ]] ||
    fail "build script must run from the repository root: $repository_top"

git_common_directory="$(
    git -C "$root_path" rev-parse --path-format=absolute --git-common-dir 2>/dev/null
)" || fail "unable to resolve Git common directory"
[[ -d "$git_common_directory" ]] ||
    fail "Git common directory not found: $git_common_directory"

git_directory="$(
    git -C "$root_path" rev-parse --path-format=absolute --git-dir 2>/dev/null
)" || fail "unable to resolve Git directory"
[[ -d "$git_directory" ]] || fail "Git directory not found: $git_directory"

if [[ "$repository_preflight_only" == true ]]; then
    status="PASS"
    output_path="repository-preflight-only"
    trap - ERR
    print_result
    exit 0
fi

[[ -d "$root_path/vendor" ]] || fail "vendor directory not found: $root_path/vendor"
[[ -d "$root_path/cmd/server" ]] || fail "server package not found: $root_path/cmd/server"

mapfile -t existing_resources < <(
    find "$root_path/cmd/server" -maxdepth 1 -type f -name 'resource_windows_*.syso' -print
)
((${#existing_resources[@]} == 0)) ||
    fail "refusing to build with pre-existing Windows resource files: ${existing_resources[*]}"

flashgate_read_repository_version "$root_path" ||
    fail "invalid or missing repository VERSION"
repository_version="$FLASHGATE_REPOSITORY_VERSION"
if [[ -z "$version" ]]; then
    version="$repository_version"
fi

flashgate_validate_semver "$version" || fail "invalid semantic version: $version"
file_version="$FLASHGATE_FILE_VERSION"

if [[ "$release" == true ]]; then
    [[ "$version" == "$repository_version" ]] ||
        fail "release version assertion differs from repository VERSION"
    expected_tag="v$repository_version"
    git -C "$root_path" tag --points-at HEAD | grep -Fxq -- "$expected_tag" ||
        fail "release builds require exact tag '$expected_tag' on HEAD"
    go -C "$root_path" run -mod=vendor ./cmd/releaseaudit source \
        --root "$root_path" --release >/dev/null ||
        fail "release notes source validation failed"
fi

commit="$(git -C "$root_path" rev-parse HEAD)"
[[ "$commit" =~ ^[0-9a-f]{40}$ ]] || fail "unexpected Git commit format: $commit"

if [[ -n "${SOURCE_DATE_EPOCH:-}" ]]; then
    flashgate_validate_source_date_epoch "$SOURCE_DATE_EPOCH" ||
        fail "SOURCE_DATE_EPOCH is invalid or outside the supported range"
    source_time="$(flashgate_source_time_from_epoch "$SOURCE_DATE_EPOCH")"
else
    commit_time="$(git -C "$root_path" show -s --format=%cI HEAD)"
    source_time="$(date -u -d "$commit_time" '+%Y-%m-%dT%H:%M:%SZ')"
fi

if [[ -n "$(git -C "$root_path" status --porcelain=v1 --untracked-files=all)" ]]; then
    modified="true"
fi

if [[ "$release" == true && "$modified" == true ]]; then
    fail "release builds require a clean working tree"
fi

if [[ -z "$output_path" ]]; then
    output_path="$root_path/build/${goos}_${public_arch}/flashgate-mcp"
elif [[ "$output_path" != /* ]]; then
    output_path="$root_path/$output_path"
fi

mkdir -p "$(dirname "$output_path")"

linker_prefix='github.com/thomasweidner/flashgate-mcp/internal/version'
build_manifest="FLASHGATE_BUILD_MANIFEST_V1"
build_manifest+="|version=${version}"
build_manifest+="|fileVersion=${file_version}"
build_manifest+="|commit=${commit}"
build_manifest+="|sourceTime=${source_time}"
build_manifest+="|modified=${modified}"
build_manifest+="|goos=${goos}"
build_manifest+="|goarch=${goarch}"
build_manifest+="|publicArch=${public_arch}"
build_manifest+="|END_FLASHGATE_BUILD_MANIFEST_V1"
ldflags=(
    "-s"
    "-w"
    "-X" "${linker_prefix}.version=${version}"
    "-X" "${linker_prefix}.fileVersion=${file_version}"
    "-X" "${linker_prefix}.commit=${commit}"
    "-X" "${linker_prefix}.date=${source_time}"
    "-X" "${linker_prefix}.modified=${modified}"
    "-X" "${linker_prefix}.buildManifest=${build_manifest}"
)

CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go -C "$root_path" build \
        -mod=vendor \
        -trimpath \
        -buildvcs=true \
        -ldflags "${ldflags[*]}" \
        -o "$output_path" \
        ./cmd/server >/dev/null

[[ -f "$output_path" ]] || fail "build output was not created: $output_path"

status="PASS"
trap - ERR
print_result
