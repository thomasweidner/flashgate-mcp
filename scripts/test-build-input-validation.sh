#!/usr/bin/env bash
set -Eeuo pipefail

root_path="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fixture_path="$root_path/internal/version/testdata/build-input-validation-fixtures.json"

# shellcheck source=build-input-validation.sh
source "$root_path/scripts/build-input-validation.sh"

[[ -n "${FLASHGATE_WORK_ROOT:-}" && -d "$FLASHGATE_WORK_ROOT" ]] || {
    printf 'FLASHGATE_WORK_ROOT must name an existing work directory\n' >&2
    exit 1
}
version_fixture_root="$(mktemp -d -- "$FLASHGATE_WORK_ROOT/flashgate-version.XXXXXXXX")"
cleanup_version_fixture() {
    rm -f -- "$version_fixture_root/VERSION"
    rmdir -- "$version_fixture_root"
}
trap cleanup_version_fixture EXIT

errors=()
if flashgate_read_repository_version "$version_fixture_root"; then
    errors+=("missing repository VERSION passed")
fi
valid_versions=('0.1.0' $'0.1.0\n' '1.2.3-rc.1')
invalid_versions=(
    '' $' 0.1.0\n' $'\xEF\xBB\xBF0.1.0\n' $'v0.1.0\n'
    $'0.1.0\n1.0.0\n' $'0.1.0\r\n' $'0.1.0\r' $'0.1.0 \n' $'1.2\n'
    $'65536.0.0\n' $'1.2.3-rc.01\n'
)
for value in "${valid_versions[@]}"; do
    printf '%s' "$value" > "$version_fixture_root/VERSION"
    if ! flashgate_read_repository_version "$version_fixture_root"; then
        errors+=("valid repository VERSION failed")
    fi
done
for index in "${!invalid_versions[@]}"; do
    value="${invalid_versions[index]}"
    if [[ "$index" == 5 ]]; then
        python3 -c 'import pathlib,sys; pathlib.Path(sys.argv[1]).write_bytes(b"0.1.0\r\n")' \
            "$version_fixture_root/VERSION"
    elif [[ "$index" == 6 ]]; then
        python3 -c 'import pathlib,sys; pathlib.Path(sys.argv[1]).write_bytes(b"0.1.0\r")' \
            "$version_fixture_root/VERSION"
    else
        printf '%s' "$value" > "$version_fixture_root/VERSION"
    fi
    if flashgate_read_repository_version "$version_fixture_root"; then
        printf -v displayed_value '%q' "$value"
        byte_hex="$(od -An -tx1 "$version_fixture_root/VERSION")"
        errors+=("invalid repository VERSION index $index passed: $displayed_value bytes=$byte_hex")
    fi
done
# Escaped payloads stay NUL-free in Bash variables; printf writes the bytes.
invalid_version_bytes=(
    '\0' '0.1.0\0' '0.1\0.0' '0.1.0\0\n' '\0.1.0\n'
)
for encoded in "${invalid_version_bytes[@]}"; do
    printf '%b' "$encoded" > "$version_fixture_root/VERSION"
    if flashgate_read_repository_version "$version_fixture_root"; then
        errors+=("NUL repository VERSION passed: $encoded")
    fi
done
fixture_stream_complete=false
while IFS= read -r -d '' kind; do
    if ! IFS= read -r -d '' value ||
        ! IFS= read -r -d '' expected; then
        errors+=("fixture stream ended with an incomplete record")
        break
    fi
    printf -v displayed_value '%q' "$value"
    case "$kind" in
        semver-valid)
            if ! flashgate_validate_semver "$value"; then
                errors+=("valid SemVer failed: $displayed_value")
            elif [[ "$FLASHGATE_FILE_VERSION" != "$expected" ]]; then
                errors+=("file version mismatch for $displayed_value")
            fi
            ;;
        semver-invalid)
            if flashgate_validate_semver "$value"; then
                errors+=("invalid SemVer passed: $displayed_value")
            fi
            ;;
        epoch-valid)
            if ! actual="$(flashgate_source_time_from_epoch "$value")"; then
                errors+=("valid epoch failed: $displayed_value")
            elif [[ "$actual" != "$expected" ]]; then
                errors+=("source time mismatch for $displayed_value")
            fi
            ;;
        epoch-invalid)
            if flashgate_validate_source_date_epoch "$value"; then
                errors+=("invalid epoch passed: $displayed_value")
            fi
            ;;
        stream-end)
            fixture_stream_complete=true
            ;;
        *)
            errors+=("unknown fixture record kind: $kind")
            ;;
    esac
done < <(
    python3 - "$fixture_path" <<'PY'
import json
import sys

def emit(kind: str, value: str, expected: str) -> None:
    for field in (kind, value, expected):
        sys.stdout.buffer.write(field.encode("utf-8"))
        sys.stdout.buffer.write(b"\0")

with open(sys.argv[1], encoding="utf-8") as stream:
    data = json.load(stream)
if data.get("schema") != "flashgate-build-input-validation-fixtures/v1":
    raise SystemExit("unexpected fixture schema")
for item in data["semanticVersions"]["valid"]:
    emit("semver-valid", item["value"], item["fileVersion"])
for value in data["semanticVersions"]["invalid"]:
    emit("semver-invalid", value, "")
for item in data["sourceDateEpoch"]["valid"]:
    emit("epoch-valid", item["value"], item["sourceTime"])
for value in data["sourceDateEpoch"]["invalid"]:
    emit("epoch-invalid", value, "")
emit("stream-end", "", "")
PY
)
if [[ "$fixture_stream_complete" != true ]]; then
    errors+=("fixture stream did not complete")
fi

status="PASS"
((${#errors[@]} == 0)) || status="FAIL"
printf 'Status: %s\n' "$status"
printf 'FixturePath: %s\n' "$fixture_path"
printf 'WarningCount: 0\n'
printf 'ErrorCount: %d\n' "${#errors[@]}"
printf 'Warnings:\n'
printf 'Errors: %s\n' "$(IFS='; '; echo "${errors[*]:-}")"
[[ "$status" == "PASS" ]]
