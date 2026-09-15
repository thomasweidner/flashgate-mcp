#!/usr/bin/env bash
set -Eeuo pipefail

# shellcheck source=native-leak-markers.sh
source "$(dirname "$0")/native-leak-markers.sh"

export FG_FORBIDDEN_WINDOWS_USER='D:\Users\RuntimeContributor'
export FG_FORBIDDEN_WSL_USER='/mnt/d/Users/RuntimeContributor'
export FG_FORBIDDEN_SYNC_ROOTS=$'D:\Sync One\Project\nE:\Sync Two'
mapfile -t markers < <(flashgate_native_leak_markers)

expected=(
    'D:\Users\RuntimeContributor'
    '/mnt/d/Users/RuntimeContributor'
    'D:\Sync One\Project'
    'E:\Sync Two'
    'C:\Users\FlashGateLeakFixture'
    '/mnt/c/Users/FlashGateLeakFixture'
    'FlashGate Sync Fixture'
)
[[ "${markers[*]}" == "${expected[*]}" ]]

unset FG_FORBIDDEN_WINDOWS_USER
if (flashgate_native_leak_markers >/dev/null 2>&1); then
    printf 'missing caller-derived Windows marker unexpectedly passed\n' >&2
    exit 1
fi
