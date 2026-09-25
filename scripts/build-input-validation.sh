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
    local value="$1"

    flashgate_validate_source_date_epoch "$value" || return 1
    date -u -d "@$FLASHGATE_SOURCE_DATE_EPOCH" '+%Y-%m-%dT%H:%M:%SZ'
}
