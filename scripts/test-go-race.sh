#!/usr/bin/env bash
set -Eeuo pipefail

repository_root="$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"

cd -- "$repository_root"
exec go test -race -mod=vendor ./...
