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

version="${FG_VERSION:-1.2.3-rc.1}"
NATIVE_SAFETY_HELPER="$FG_NATIVE_SAFETY_HELPER"
# shellcheck source=native-validation-safety.sh
source "$FG_NATIVE_SAFETY"
flashgate_prepare_work_root

native_root="$NATIVE_WORK_ROOT"
repo_dir="$native_root/repository"
snapshot_dir="$native_root/verified-snapshot"
output_dir="$native_root/output"
logs_dir="$native_root/logs"
summary_path="$FG_OUTPUT_DIR/native-summary.env"
failure_log="$FG_OUTPUT_DIR/native-failure.log"

mkdir -p "$FG_OUTPUT_DIR"
mkdir -p "$output_dir" "$logs_dir"

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

for command_name in git tar file readelf objcopy strings sha256sum date find sort xargs awk grep hostname; do
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
        printf 'native validation repository must not be under a Windows mount: %s\n' "$PWD" >&2
        exit 1
        ;;
esac

chmod +x \
    scripts/build.sh \
    scripts/build-input-validation.sh \
    scripts/Test-LinuxMetadata.sh \
    scripts/test-verifier-process.sh \
    scripts/verifier-process.sh \
    scripts/new-release-artifact.sh \
    scripts/test-release-artifact.sh \
    scripts/test-build-input-validation.sh \
    scripts/test-build-repository.sh \
    scripts/test-native-validation-safety.sh

head_commit="$(git rev-parse HEAD)"
[[ "$head_commit" =~ ^[0-9a-f]{40}$ ]]
[[ "$head_commit" == "$FG_HEAD_COMMIT" ]]

commit_time="$(git show -s --format=%cI HEAD)"
source_time="$(date -u -d "$commit_time" '+%Y-%m-%dT%H:%M:%SZ')"
modified="false"
if [[ -n "$(git status --porcelain=v1 --untracked-files=normal)" ]]; then
    modified="true"
fi
file_version="${version%%[-+]*}.0"

bash -n scripts/build.sh
bash -n scripts/Test-LinuxMetadata.sh
bash -n scripts/test-verifier-process.sh
bash -n scripts/verifier-process.sh
bash -n scripts/new-release-artifact.sh
bash -n scripts/test-release-artifact.sh
bash -n scripts/build-input-validation.sh
bash -n scripts/test-build-input-validation.sh
bash -n scripts/test-build-repository.sh
bash -n scripts/test-native-validation-safety.sh
python3 -c \
    'import ast, pathlib; ast.parse(pathlib.Path("scripts/bounded-process-runner.py").read_text(encoding="utf-8"))'
python3 -c \
    'import ast, pathlib; ast.parse(pathlib.Path("scripts/testdata/verifier-process-helper.py").read_text(encoding="utf-8"))'

bash scripts/test-build-input-validation.sh \
    >"$logs_dir/build-input-validation.log" 2>&1
bash scripts/test-build-repository.sh \
    >"$logs_dir/build-repository.log" 2>&1
bash scripts/test-native-validation-safety.sh \
    >"$logs_dir/native-root-safety.log" 2>&1
python3 scripts/test-snapshot-security.py \
    >"$logs_dir/snapshot-security.log" 2>&1

go test -mod=vendor ./... >"$logs_dir/go-test.log" 2>&1
go test -race -mod=vendor ./... >"$logs_dir/go-test-race.log" 2>&1
go vet -mod=vendor ./... >"$logs_dir/go-vet.log" 2>&1
go list -mod=vendor ./... >"$logs_dir/go-list.log" 2>&1
git diff --check >"$logs_dir/git-diff-check.log" 2>&1

find . \
    -path './vendor' -prune -o \
    -type f -name '*.go' -print0 |
    xargs -0 gofmt -l >"$logs_dir/gofmt.log"
[[ ! -s "$logs_dir/gofmt.log" ]] || {
    printf 'gofmt reported nonconforming project files\n' >&2
    exit 1
}

coverage_dir="$output_dir/coverage-linux"
mkdir -p "$coverage_dir"
go test \
    -mod=vendor \
    -covermode=atomic \
    -coverpkg=./... \
    -coverprofile="$coverage_dir/coverage.out" \
    ./... >"$coverage_dir/test.log" 2>&1
go tool cover \
    -func="$coverage_dir/coverage.out" >"$coverage_dir/coverage.txt"
go tool cover \
    -html="$coverage_dir/coverage.out" \
    -o "$coverage_dir/coverage.html"
coverage_total="$(
    awk '/^total:/ { gsub(/%/, "", $3); print $3 }' \
        "$coverage_dir/coverage.txt"
)"
[[ -n "$coverage_total" ]] || {
    printf 'Linux total coverage could not be resolved\n' >&2
    exit 1
}
awk -v value="$coverage_total" \
    'BEGIN { exit !(value >= 70.6) }' || {
    printf 'Linux coverage %s is below 70.6\n' "$coverage_total" >&2
    exit 1
}

x64_a="$output_dir/flashgate-mcp_${version}_linux_x64_a"
x64_b="$output_dir/flashgate-mcp_${version}_linux_x64_b"
arm64="$output_dir/flashgate-mcp_${version}_linux_arm64"
benchmark_binary="$output_dir/flashgate-benchmark"

bash scripts/build.sh \
    --goos linux \
    --goarch amd64 \
    --version "$version" \
    --output "$x64_a" >"$logs_dir/build-linux-x64-a.log" 2>&1

bash scripts/build.sh \
    --goos linux \
    --goarch amd64 \
    --version "$version" \
    --output "$x64_b" >"$logs_dir/build-linux-x64-b.log" 2>&1

bash scripts/build.sh \
    --goos linux \
    --goarch arm64 \
    --version "$version" \
    --output "$arm64" >"$logs_dir/build-linux-arm64.log" 2>&1

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
        -mod=vendor \
        -trimpath \
        -o "$benchmark_binary" \
        ./cmd/benchmark >"$logs_dir/build-benchmark.log" 2>&1

bash scripts/Test-LinuxMetadata.sh \
    --binary "$x64_a" \
    --expected-product-version "$version" \
    --expected-file-version "$file_version" \
    --expected-public-arch x64 \
    --expected-goarch amd64 \
    --expected-commit "$head_commit" \
    --expected-source-time "$source_time" \
    --expected-modified "$modified" >"$logs_dir/metadata-linux-x64.log" 2>&1

bash scripts/Test-LinuxMetadata.sh \
    --binary "$arm64" \
    --expected-product-version "$version" \
    --expected-file-version "$file_version" \
    --expected-public-arch arm64 \
    --expected-goarch arm64 \
    --expected-commit "$head_commit" \
    --expected-source-time "$source_time" \
    --expected-modified "$modified" \
    --skip-execution >"$logs_dir/metadata-linux-arm64.log" 2>&1

bash scripts/test-verifier-process.sh \
    --real-linux-x64 "$x64_a" \
    --real-linux-arm64 "$arm64" \
    --expected-product-version "$version" \
    --expected-file-version "$file_version" \
    --expected-commit "$head_commit" \
    --expected-source-time "$source_time" \
    --expected-modified "$modified" \
    >"$logs_dir/verifier-process-contracts.log" 2>&1

compact_output="$("$x64_a" --version)"
verbose_output="$("$x64_a" --version --verbose)"
go_build_info_x64="$(go version -m "$x64_a")"
go_build_info_arm64="$(go version -m "$arm64")"
elf_notes_x64="$(readelf -n "$x64_a")"
elf_notes_arm64="$(readelf -n "$arm64")"
build_id_x64="$(go tool buildid "$x64_a")"
build_id_arm64="$(go tool buildid "$arm64")"

sha_x64_a="$(sha256sum "$x64_a" | awk '{print $1}')"
sha_x64_b="$(sha256sum "$x64_b" | awk '{print $1}')"
sha_arm64="$(sha256sum "$arm64" | awk '{print $1}')"
[[ "$sha_x64_a" == "$sha_x64_b" ]] || {
    printf 'reproducibility comparison failed: %s != %s\n' "$sha_x64_a" "$sha_x64_b" >&2
    exit 1
}

wrong_version_log="$logs_dir/negative-wrong-version.log"
if bash scripts/Test-LinuxMetadata.sh \
    --binary "$x64_a" \
    --expected-product-version 9.9.9 \
    --expected-file-version 9.9.9.0 \
    --expected-public-arch x64 \
    --expected-goarch amd64 \
    --expected-commit "$head_commit" \
    --expected-source-time "$source_time" \
    --expected-modified "$modified" >"$wrong_version_log" 2>&1; then
    printf 'negative wrong-version test unexpectedly passed\n' >&2
    exit 1
fi

linker_prefix='github.com/thomasweidner/flashgate-mcp/internal/version'
no_vcs="$output_dir/flashgate-mcp-no-vcs"
no_vcs_ldflags="-s -w"
no_vcs_ldflags+=" -X ${linker_prefix}.version=${version}"
no_vcs_ldflags+=" -X ${linker_prefix}.fileVersion=${file_version}"
no_vcs_ldflags+=" -X ${linker_prefix}.commit=${head_commit}"
no_vcs_ldflags+=" -X ${linker_prefix}.date=${source_time}"
no_vcs_ldflags+=" -X ${linker_prefix}.modified=${modified}"
no_vcs_manifest="FLASHGATE_BUILD_MANIFEST_V1"
no_vcs_manifest+="|version=${version}"
no_vcs_manifest+="|fileVersion=${file_version}"
no_vcs_manifest+="|commit=${head_commit}"
no_vcs_manifest+="|sourceTime=${source_time}"
no_vcs_manifest+="|modified=${modified}"
no_vcs_manifest+="|goos=linux"
no_vcs_manifest+="|goarch=amd64"
no_vcs_manifest+="|publicArch=x64"
no_vcs_manifest+="|END_FLASHGATE_BUILD_MANIFEST_V1"
no_vcs_ldflags+=" -X ${linker_prefix}.buildManifest=${no_vcs_manifest}"

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
        -mod=vendor \
        -trimpath \
        -buildvcs=false \
        -ldflags "$no_vcs_ldflags" \
        -o "$no_vcs" \
        ./cmd/server

missing_vcs_log="$logs_dir/negative-missing-vcs.log"
if bash scripts/Test-LinuxMetadata.sh \
    --binary "$no_vcs" \
    --expected-product-version "$version" \
    --expected-file-version "$file_version" \
    --expected-public-arch x64 \
    --expected-goarch amd64 \
    --expected-commit "$head_commit" \
    --expected-source-time "$source_time" \
    --expected-modified "$modified" >"$missing_vcs_log" 2>&1; then
    printf 'negative missing-VCS test unexpectedly passed\n' >&2
    exit 1
fi

no_note="$output_dir/flashgate-mcp-no-go-note"
objcopy --remove-section=.note.go.buildid "$x64_a" "$no_note"

missing_note_log="$logs_dir/negative-missing-go-note.log"
if bash scripts/Test-LinuxMetadata.sh \
    --binary "$no_note" \
    --expected-product-version "$version" \
    --expected-file-version "$file_version" \
    --expected-public-arch x64 \
    --expected-goarch amd64 \
    --expected-commit "$head_commit" \
    --expected-source-time "$source_time" \
    --expected-modified "$modified" >"$missing_note_log" 2>&1; then
    printf 'negative missing-Go-note test unexpectedly passed\n' >&2
    exit 1
fi

strings "$x64_a" >"$logs_dir/linux-x64-strings.log"
if grep -E -i \
    'C:\\Users\\ThomasW|/mnt/c/Users/ThomasW|OneDrive - VOXTRONIC' \
    "$logs_dir/linux-x64-strings.log" >"$logs_dir/path-leak-findings.log"; then
    printf 'local Windows path or user data found in Linux binary strings\n' >&2
    exit 1
fi

if grep -F "$native_root" \
    "$logs_dir/linux-x64-strings.log" >>"$logs_dir/path-leak-findings.log"; then
    printf 'native validation path found in Linux binary strings\n' >&2
    exit 1
fi

release_a="$native_root/release-a"
release_b="$native_root/release-b"
mkdir -p "$release_a" "$release_b"

bash scripts/new-release-artifact.sh \
    --version "$version" \
    --goarch amd64 \
    --output-directory "$release_a" >"$logs_dir/release-linux-x64-a.log" 2>&1
bash scripts/new-release-artifact.sh \
    --version "$version" \
    --goarch arm64 \
    --output-directory "$release_a" >"$logs_dir/release-linux-arm64-a.log" 2>&1
bash scripts/new-release-artifact.sh \
    --version "$version" \
    --goarch amd64 \
    --output-directory "$release_b" >"$logs_dir/release-linux-x64-b.log" 2>&1
bash scripts/new-release-artifact.sh \
    --version "$version" \
    --goarch arm64 \
    --output-directory "$release_b" >"$logs_dir/release-linux-arm64-b.log" 2>&1

linux_x64_archive="$release_a/flashgate-mcp_${version}_linux_x64.tar.gz"
linux_arm64_archive="$release_a/flashgate-mcp_${version}_linux_arm64.tar.gz"
linux_x64_checksum="${linux_x64_archive}.sha256"
linux_arm64_checksum="${linux_arm64_archive}.sha256"
linux_x64_archive_b="$release_b/flashgate-mcp_${version}_linux_x64.tar.gz"
linux_arm64_archive_b="$release_b/flashgate-mcp_${version}_linux_arm64.tar.gz"
linux_x64_checksum_b="${linux_x64_archive_b}.sha256"
linux_arm64_checksum_b="${linux_arm64_archive_b}.sha256"

bash scripts/test-release-artifact.sh \
    --archive "$linux_x64_archive" \
    --checksum "$linux_x64_checksum" \
    --expected-version "$version" \
    --expected-public-arch x64 \
    --expected-goarch amd64 \
    --expected-commit "$head_commit" \
    --expected-source-time "$source_time" \
    --expected-modified "$modified" >"$logs_dir/test-release-linux-x64.log" 2>&1
bash scripts/test-release-artifact.sh \
    --archive "$linux_arm64_archive" \
    --checksum "$linux_arm64_checksum" \
    --expected-version "$version" \
    --expected-public-arch arm64 \
    --expected-goarch arm64 \
    --expected-commit "$head_commit" \
    --expected-source-time "$source_time" \
    --expected-modified "$modified" >"$logs_dir/test-release-linux-arm64.log" 2>&1

linux_x64_archive_sha="$(sha256sum "$linux_x64_archive" | awk '{print $1}')"
linux_arm64_archive_sha="$(sha256sum "$linux_arm64_archive" | awk '{print $1}')"
linux_x64_archive_b_sha="$(sha256sum "$linux_x64_archive_b" | awk '{print $1}')"
linux_arm64_archive_b_sha="$(sha256sum "$linux_arm64_archive_b" | awk '{print $1}')"
[[ "$linux_x64_archive_sha" == "$linux_x64_archive_b_sha" ]]
[[ "$linux_arm64_archive_sha" == "$linux_arm64_archive_b_sha" ]]

go -C "$repo_dir" run -mod=vendor ./cmd/releaseaudit compare \
    --artifact-a "$linux_x64_archive" \
    --checksum-a "$linux_x64_checksum" \
    --artifact-b "$linux_x64_archive_b" \
    --checksum-b "$linux_x64_checksum_b" \
    --binary-suffix /flashgate-mcp \
    --report "$logs_dir/repro-linux-x64.json"
go -C "$repo_dir" run -mod=vendor ./cmd/releaseaudit compare \
    --artifact-a "$linux_arm64_archive" \
    --checksum-a "$linux_arm64_checksum" \
    --artifact-b "$linux_arm64_archive_b" \
    --checksum-b "$linux_arm64_checksum_b" \
    --binary-suffix /flashgate-mcp \
    --report "$logs_dir/repro-linux-arm64.json"

host_name="$(hostname)"
leak_arguments=(
    --forbidden "$native_root"
    --forbidden "$HOME"
    --forbidden "$host_name"
    --forbidden 'C:\Users\ThomasW'
    --forbidden '/mnt/c/Users/ThomasW'
    --forbidden 'OneDrive - VOXTRONIC'
)
go -C "$repo_dir" run -mod=vendor ./cmd/releaseaudit scan \
    --artifact "$linux_x64_archive" \
    --checksum "$linux_x64_checksum" \
    --report "$logs_dir/leak-linux-x64.json" \
    "${leak_arguments[@]}"
go -C "$repo_dir" run -mod=vendor ./cmd/releaseaudit scan \
    --artifact "$linux_arm64_archive" \
    --checksum "$linux_arm64_checksum" \
    --report "$logs_dir/leak-linux-arm64.json" \
    "${leak_arguments[@]}"

printf '%s\n' "$compact_output" >"$logs_dir/cli-compact-linux-x64.log"
printf '%s\n' "$verbose_output" >"$logs_dir/cli-verbose-linux-x64.log"
printf '%s\n' "$go_build_info_x64" >"$logs_dir/go-version-m-linux-x64.log"
printf '%s\n' "$go_build_info_arm64" >"$logs_dir/go-version-m-linux-arm64.log"
printf '%s\n' "$elf_notes_x64" >"$logs_dir/readelf-notes-linux-x64.log"
printf '%s\n' "$elf_notes_arm64" >"$logs_dir/readelf-notes-linux-arm64.log"
printf '%s\n' "$build_id_x64" >"$logs_dir/go-buildid-linux-x64.log"
printf '%s\n' "$build_id_arm64" >"$logs_dir/go-buildid-linux-arm64.log"

cp "$x64_a" "$FG_OUTPUT_DIR/flashgate-mcp_${version}_linux_x64"
cp "$arm64" "$FG_OUTPUT_DIR/flashgate-mcp_${version}_linux_arm64"
cp "$linux_x64_archive" "$linux_x64_checksum" "$FG_OUTPUT_DIR/"
cp "$linux_arm64_archive" "$linux_arm64_checksum" "$FG_OUTPUT_DIR/"
cp -R "$coverage_dir" "$FG_OUTPUT_DIR/coverage-linux"
cp -R "$logs_dir" "$FG_OUTPUT_DIR/logs"

{
    printf 'STATUS=PASS\n'
    printf 'DISTRO_NAME=%s\n' "$FG_DISTRO_NAME"
    printf 'UBUNTU_DESCRIPTION=%s\n' "$ubuntu_description"
    printf 'KERNEL=%s\n' "$kernel"
    printf 'GO_VERSION=%s\n' "$go_version"
    printf 'NATIVE_ROOT=%s\n' "$native_root"
    printf 'HEAD_COMMIT=%s\n' "$head_commit"
    printf 'SOURCE_TIME=%s\n' "$source_time"
    printf 'MODIFIED=%s\n' "$modified"
    printf 'VERSION=%s\n' "$version"
    printf 'FILE_VERSION=%s\n' "$file_version"
    printf 'LINUX_X64_SHA256=%s\n' "$sha_x64_a"
    printf 'LINUX_ARM64_SHA256=%s\n' "$sha_arm64"
    printf 'LINUX_X64_BUILD_ID=%s\n' "$build_id_x64"
    printf 'LINUX_ARM64_BUILD_ID=%s\n' "$build_id_arm64"
    printf 'LINUX_COVERAGE=%s\n' "$coverage_total"
    printf 'LINUX_X64_ARCHIVE_SHA256=%s\n' "$linux_x64_archive_sha"
    printf 'LINUX_ARM64_ARCHIVE_SHA256=%s\n' "$linux_arm64_archive_sha"
    printf 'COMPACT_OUTPUT=%s\n' "$compact_output"
    printf 'NEGATIVE_WRONG_VERSION=PASS\n'
    printf 'NEGATIVE_MISSING_VCS=PASS\n'
    printf 'NEGATIVE_MISSING_GO_NOTE=PASS\n'
    printf 'REPRODUCIBLE_X64=PASS\n'
    printf 'REPRODUCIBLE_ARCHIVES=PASS\n'
    printf 'PATH_LEAK_SCAN=PASS\n'
    printf 'SECRET_LEAK_SCAN=PASS\n'
} >"$summary_path"

trap - ERR
