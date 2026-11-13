#!/usr/bin/env python3
"""Validate the explicit native-Linux corpus parent for authoritative benchmarks."""

from __future__ import annotations

import os
import stat
from pathlib import Path


def _unescape_mount_path(value: str) -> str:
    for encoded, decoded in (("\\040", " "), ("\\011", "\t"), ("\\012", "\n"), ("\\134", "\\")):
        value = value.replace(encoded, decoded)
    return value


def _filesystem_type(path: Path, mountinfo: str) -> str:
    matches: list[tuple[int, str]] = []
    for line in mountinfo.splitlines():
        fields = line.split()
        try:
            separator = fields.index("-")
        except ValueError:
            continue
        if separator + 1 >= len(fields) or len(fields) < 5:
            continue
        mount = Path(_unescape_mount_path(fields[4]))
        try:
            path.relative_to(mount)
        except ValueError:
            continue
        matches.append((len(mount.parts), fields[separator + 1]))
    if not matches:
        raise RuntimeError("corpus parent filesystem could not be resolved")
    return max(matches)[1]


def validate_linux_corpus_parent(raw: str | None, mountinfo: str | None = None) -> Path:
    """Return a canonical ext4 directory strictly below /home, or fail closed."""

    if not raw:
        raise RuntimeError("FLASHGATE_BENCHMARK_CORPUS_PARENT is required")
    candidate = Path(raw)
    if not candidate.is_absolute():
        raise RuntimeError("benchmark corpus parent must be an absolute path")
    try:
        resolved = candidate.resolve(strict=True)
    except OSError as exc:
        raise RuntimeError("benchmark corpus parent must be an existing directory") from exc
    if not resolved.is_dir() or resolved == Path("/home") or Path("/home") not in resolved.parents:
        raise RuntimeError("benchmark corpus parent must be strictly below /home")
    if os.path.normpath(str(candidate)) != str(resolved):
        raise RuntimeError("benchmark corpus parent must be canonical and contain no links")
    current = Path(resolved.anchor)
    for component in resolved.parts[1:]:
        current /= component
        info = current.lstat()
        if stat.S_ISLNK(info.st_mode):
            raise RuntimeError("benchmark corpus parent path crosses a symbolic link")
    if mountinfo is None:
        try:
            mountinfo = Path("/proc/self/mountinfo").read_text(encoding="utf-8")
        except OSError as exc:
            raise RuntimeError("benchmark corpus parent filesystem could not be inspected") from exc
    filesystem = _filesystem_type(resolved, mountinfo)
    if filesystem != "ext4":
        raise RuntimeError(f"benchmark corpus parent must be on native ext4, found {filesystem}")
    return resolved


if __name__ == "__main__":
    print(validate_linux_corpus_parent(os.environ.get("FLASHGATE_BENCHMARK_CORPUS_PARENT")))
