#!/usr/bin/env python3
"""Resolve a fail-closed FlashGate validation work root."""

from __future__ import annotations

import os
import stat
from pathlib import Path
from typing import Mapping


def _path_key(path: Path) -> str:
    return os.path.normcase(os.path.normpath(str(path)))


def _safe_absolute_directory(raw: str | None, name: str) -> Path:
    if not raw:
        raise RuntimeError(f"{name} is required")
    candidate = Path(raw)
    if not candidate.is_absolute():
        raise RuntimeError(f"{name} must be an absolute path")
    try:
        resolved = candidate.resolve(strict=True)
    except OSError as exc:
        raise RuntimeError(f"{name} must be an existing directory") from exc
    if not resolved.is_dir():
        raise RuntimeError(f"{name} must be an existing directory")
    if resolved.parent == resolved:
        raise RuntimeError(f"{name} must not be a filesystem root")
    if _path_key(resolved) != _path_key(candidate):
        raise RuntimeError(f"{name} must be canonical and contain no links")

    current = Path(resolved.anchor)
    reparse_flag = getattr(stat, "FILE_ATTRIBUTE_REPARSE_POINT", 0)
    for component in resolved.parts[1:]:
        current /= component
        info = current.lstat()
        attributes = getattr(info, "st_file_attributes", 0)
        if stat.S_ISLNK(info.st_mode) or (reparse_flag and attributes & reparse_flag):
            raise RuntimeError(f"{name} must be canonical and contain no links")
    return resolved


def resolve_validation_work_root(
    environ: Mapping[str, str] | None = None,
) -> Path:
    """Return the validated work root from an explicit process environment."""

    values = os.environ if environ is None else environ
    work_root = _safe_absolute_directory(
        values.get("FLASHGATE_WORK_ROOT"), "FLASHGATE_WORK_ROOT"
    )
    task_raw = values.get("FLASHGATE_TASK_ROOT")
    if task_raw:
        task_root = _safe_absolute_directory(task_raw, "FLASHGATE_TASK_ROOT")
        if work_root == task_root or task_root not in work_root.parents:
            raise RuntimeError(
                "FLASHGATE_WORK_ROOT must be below FLASHGATE_TASK_ROOT"
            )
    return work_root
