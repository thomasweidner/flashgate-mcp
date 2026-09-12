#!/usr/bin/env bash

# Emits caller-derived host markers plus stable synthetic fixtures. The fixtures
# keep the leak gate deterministic without embedding a contributor identity.
flashgate_native_leak_markers() {
    : "${FG_FORBIDDEN_WINDOWS_USER:?FG_FORBIDDEN_WINDOWS_USER is required}"
    : "${FG_FORBIDDEN_WSL_USER:?FG_FORBIDDEN_WSL_USER is required}"

    printf '%s\n' "$FG_FORBIDDEN_WINDOWS_USER" "$FG_FORBIDDEN_WSL_USER"
    if [[ -n "${FG_FORBIDDEN_SYNC_ROOTS:-}" ]]; then
        printf '%s\n' "$FG_FORBIDDEN_SYNC_ROOTS"
    fi
    printf '%s\n' 'C:\Users\FlashGateLeakFixture' '/mnt/c/Users/FlashGateLeakFixture' 'FlashGate Sync Fixture'
}
