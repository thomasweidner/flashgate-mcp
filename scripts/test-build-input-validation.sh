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
    if [[ -d "$version_fixture_root/date tools" ]]; then
        rm -f -- "$version_fixture_root/date tools/date" \
            "$version_fixture_root/date tools/gnudate"
        rmdir -- "$version_fixture_root/date tools"
    fi
    rm -f -- "$version_fixture_root/date-double" \
        "$version_fixture_root/date-calls" "$version_fixture_root/date-error"
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

# Keep host fixtures above intact; these doubles exercise tool selection without
# requiring a second installed implementation or changing the system PATH.
date_tool_root="$version_fixture_root/date tools"
mkdir -- "$date_tool_root"
cat > "$version_fixture_root/date-double" <<'BASH'
#!/usr/bin/bash
set -eu
tool="${0##*/}"
printf '%s:%s\n' "$tool" "$*" >> "$FLASHGATE_DATE_TEST_LOG"
[[ $# == 4 && "$1" == '-u' && "$2" == '-d' &&
    "$4" == '+%Y-%m-%dT%H:%M:%SZ' ]] || exit 90
case "$tool" in
    gnudate) mode="$FLASHGATE_GNUDATE_TEST_MODE" ;;
    date) mode="$FLASHGATE_DATE_TEST_MODE" ;;
    *) exit 91 ;;
esac
case "$mode" in
    nonzero) exit 42 ;;
    malformed) printf 'not-a-timestamp\n'; exit 0 ;;
    unexpected) printf '2000-01-01T00:00:00Z\n'; exit 0 ;;
    incompatible)
        [[ "$3" != '@253402300799' ]] || exit 1
        ;;
    actual-nonzero)
        [[ "$3" != '@1' ]] || exit 42
        ;;
    actual-malformed)
        if [[ "$3" == '@1' ]]; then
            printf '1970-01-01T00:00:01Z extra\n'
            exit 0
        fi
        ;;
esac
case "$3" in
    '@0') printf '1970-01-01T00:00:00Z\n' ;;
    '@1') printf '1970-01-01T00:00:01Z\n' ;;
    '@1704067200') printf '2024-01-01T00:00:00Z\n' ;;
    '@253402300799') printf '9999-12-31T23:59:59Z\n' ;;
    *) exit 92 ;;
esac
[[ "$mode" != 'output-nonzero' ]] || exit 42
BASH
cp -- "$version_fixture_root/date-double" "$date_tool_root/date"
chmod +x -- "$date_tool_root/date"
date_test_count=0
date_case() {
    local label="$1" value="$2" expected="$3" preferred="${4:-}"
    local actual diagnostic
    : > "$version_fixture_root/date-calls"
    : > "$version_fixture_root/date-error"
    date_test_count=$((date_test_count + 1))
    if actual="$(PATH="$date_tool_root" \
        FLASHGATE_DATE_TEST_LOG="$version_fixture_root/date-calls" \
        FLASHGATE_GNUDATE_TEST_MODE="$gnudate_mode" \
        FLASHGATE_DATE_TEST_MODE="$date_mode" \
        flashgate_source_time_from_epoch "$value" \
        2> "$version_fixture_root/date-error")"; then
        if [[ -z "$expected" || "$actual" != "$expected" ]]; then
            errors+=("date tool case failed: $label")
        fi
    elif [[ -n "$expected" ]]; then
        errors+=("date tool case rejected valid epoch: $label")
    else
        # Failure must never leak a fabricated source timestamp to stdout.
        [[ -z "$actual" ]] || errors+=("date tool failure emitted timestamp: $label")
        diagnostic="$(< "$version_fixture_root/date-error")"
        [[ -n "$diagnostic" ]] || errors+=("date tool failure was silent: $label")
    fi
    if [[ -n "$preferred" ]]; then
        while IFS= read -r actual; do
            [[ "$actual" == "$preferred:"* ]] ||
                errors+=("wrong date tool used: $label")
        done < "$version_fixture_root/date-calls"
    fi
}
gnudate_mode=correct
date_mode=incompatible
cp -- "$version_fixture_root/date-double" "$date_tool_root/gnudate"
chmod +x -- "$date_tool_root/gnudate"
date_case 'prefer gnudate over incompatible date' 253402300799 \
    '9999-12-31T23:59:59Z' gnudate
date_mode=correct
date_case 'prefer gnudate over compatible date' 1704067200 \
    '2024-01-01T00:00:00Z' gnudate
for gnudate_mode in nonzero output-nonzero malformed unexpected incompatible; do
    date_case "fallback from $gnudate_mode gnudate" 1 '1970-01-01T00:00:01Z'
done
rm -f -- "$date_tool_root/gnudate"
gnudate_mode=correct
for record in '0|1970-01-01T00:00:00Z' '1|1970-01-01T00:00:01Z' \
    '0000000001|1970-01-01T00:00:01Z' '1704067200|2024-01-01T00:00:00Z' \
    '253402300799|9999-12-31T23:59:59Z'; do
    date_case 'compatible date without gnudate' "${record%%|*}" "${record#*|}" date
done
for date_mode in incompatible nonzero output-nonzero malformed unexpected actual-nonzero actual-malformed; do
    date_case "reject $date_mode date without gnudate" 1 ''
done
rm -f -- "$date_tool_root/date"
date_case 'no date implementation' 0 ''
# Invalid input must be rejected before any tool, including capability probes.
cp -- "$version_fixture_root/date-double" "$date_tool_root/date"
chmod +x -- "$date_tool_root/date"
date_mode=correct
for value in '253402300800' '-1' 'abc' '' ' 1' '1 ' '1.0' $'1\n' '+1'; do
    : > "$version_fixture_root/date-calls"
    date_test_count=$((date_test_count + 1))
    if PATH="$date_tool_root" \
        FLASHGATE_DATE_TEST_LOG="$version_fixture_root/date-calls" \
        FLASHGATE_GNUDATE_TEST_MODE="$gnudate_mode" \
        FLASHGATE_DATE_TEST_MODE="$date_mode" \
        flashgate_source_time_from_epoch "$value" > /dev/null 2>&1; then
        errors+=("invalid epoch reached date conversion")
    fi
    [[ ! -s "$version_fixture_root/date-calls" ]] ||
        errors+=("date tool called before epoch validation")
done

status="PASS"
((${#errors[@]} == 0)) || status="FAIL"
printf 'Status: %s\n' "$status"
printf 'FixturePath: %s\n' "$fixture_path"
printf 'DateToolCaseCount: %d\n' "$date_test_count"
printf 'WarningCount: 0\n'
printf 'ErrorCount: %d\n' "${#errors[@]}"
printf 'Warnings:\n'
printf 'Errors: %s\n' "$(IFS='; '; echo "${errors[*]:-}")"
[[ "$status" == "PASS" ]]
