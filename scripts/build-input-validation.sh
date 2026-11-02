#!/usr/bin/env bash

FLASHGATE_MAX_SOURCE_DATE_EPOCH=253402300799
FLASHGATE_SEMVER_MAJOR=""
FLASHGATE_SEMVER_MINOR=""
FLASHGATE_SEMVER_PATCH=""
FLASHGATE_FILE_VERSION=""
FLASHGATE_SOURCE_DATE_EPOCH=""
FLASHGATE_REPOSITORY_VERSION=""

flashgate_read_repository_version() {
    local LC_ALL=C
    local root_path="$1"
    local version_path="$root_path/VERSION"
    local value
    local byte_count
    local byte_hex
    [[ -f "$version_path" && ! -L "$version_path" ]] || return 1
    byte_count="$(wc -c < "$version_path")" || return 1
    ((byte_count > 0 && byte_count <= 128)) || return 1
    byte_hex="$(od -An -tx1 "$version_path")" || return 1
    # Inspect hex bytes before read: Bash variables cannot retain NUL bytes.
    [[ ! " $byte_hex " =~ [[:space:]](00|0d)[[:space:]] ]] || return 1
    IFS= read -r value < "$version_path" || :
    # A single extra byte is allowed only when its actual final byte is LF.
    if ((byte_count != ${#value} && byte_count != ${#value} + 1)); then
        return 1
    fi
    if ((byte_count == ${#value} + 1)) &&
        [[ ! "$byte_hex" =~ (^|[[:space:]])0a$ ]]; then
        return 1
    fi
    flashgate_validate_semver "$value" || return 1
    FLASHGATE_REPOSITORY_VERSION="$value"
}

flashgate_decimal_le() {
    local value="$1"
    local maximum="$2"

    ((${#value} < ${#maximum})) && return 0
    ((${#value} > ${#maximum})) && return 1
    [[ "$value" == "$maximum" || "$value" < "$maximum" ]]
}

flashgate_validate_semver() {
    local value="$1"
    local pattern
    local prerelease
    local identifier

    pattern='^(0|[1-9][0-9]*)\.'
    pattern+='(0|[1-9][0-9]*)\.'
    pattern+='(0|[1-9][0-9]*)'
    pattern+='(-([0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*))?'
    pattern+='(\+([0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*))?$'
    [[ "$value" =~ $pattern ]] || return 1

    FLASHGATE_SEMVER_MAJOR="${BASH_REMATCH[1]}"
    FLASHGATE_SEMVER_MINOR="${BASH_REMATCH[2]}"
    FLASHGATE_SEMVER_PATCH="${BASH_REMATCH[3]}"
    prerelease="${BASH_REMATCH[5]:-}"

    for identifier in ${prerelease//./ }; do
        if [[ "$identifier" =~ ^[0-9]+$ && ${#identifier} -gt 1 && "$identifier" == 0* ]]; then
            return 1
        fi
    done

    flashgate_decimal_le "$FLASHGATE_SEMVER_MAJOR" 65535 || return 1
    flashgate_decimal_le "$FLASHGATE_SEMVER_MINOR" 65535 || return 1
    flashgate_decimal_le "$FLASHGATE_SEMVER_PATCH" 65535 || return 1

    FLASHGATE_FILE_VERSION="${FLASHGATE_SEMVER_MAJOR}.${FLASHGATE_SEMVER_MINOR}.${FLASHGATE_SEMVER_PATCH}.0"
}

flashgate_validate_source_date_epoch() {
    local value="$1"
    local normalized

    [[ "$value" =~ ^[0-9]+$ ]] || return 1
    normalized="$value"
    while [[ ${#normalized} -gt 1 && "$normalized" == 0* ]]; do
        normalized="${normalized#0}"
    done
    flashgate_decimal_le "$normalized" "$FLASHGATE_MAX_SOURCE_DATE_EPOCH" ||
        return 1
    FLASHGATE_SOURCE_DATE_EPOCH="$normalized"
}

flashgate_source_time_from_epoch() {
    local LC_ALL=C
    local value="$1"
    local date_tool
    local source_time
    local timestamp_pattern
    local probe
    local compatible

    flashgate_validate_source_date_epoch "$value" || return 1
    # Some date implementations cannot represent the full accepted epoch range.
    # Prefer GNU's separate executable when installed, then probe the host date.
    for date_tool in gnudate date; do
        command -v "$date_tool" > /dev/null 2>&1 || continue
        compatible=true
        for probe in '0|1970-01-01T00:00:00Z' \
            '1704067200|2024-01-01T00:00:00Z' \
            '253402300799|9999-12-31T23:59:59Z'; do
            if ! source_time="$("$date_tool" -u -d "@${probe%%|*}" \
                '+%Y-%m-%dT%H:%M:%SZ' 2> /dev/null)" ||
                [[ "$source_time" != "${probe#*|}" ]]; then
                compatible=false
                break
            fi
        done
        [[ "$compatible" == true ]] || continue

        if ! source_time="$("$date_tool" -u -d "@$FLASHGATE_SOURCE_DATE_EPOCH" \
            '+%Y-%m-%dT%H:%M:%SZ' 2> /dev/null)"; then
            printf 'Source epoch conversion failed with %s\n' "$date_tool" >&2
            return 1
        fi
        timestamp_pattern='^(19[7-9][0-9]|[2-9][0-9]{3})-(0[1-9]|1[0-2])-'
        timestamp_pattern+='(0[1-9]|[12][0-9]|3[01])T([01][0-9]|2[0-3]):'
        timestamp_pattern+='[0-5][0-9]:[0-5][0-9]Z$'
        if [[ ! "$source_time" =~ $timestamp_pattern ]]; then
            printf 'Unexpected source epoch output from %s\n' "$date_tool" >&2
            return 1
        fi
        printf '%s\n' "$source_time"
        return 0
    done
    printf 'No compatible gnudate or date for the accepted source epoch range\n' >&2
    return 1
}
