#!/usr/bin/env python3
"""Focused descriptor-binding tests for safe-work-root.py."""

from __future__ import annotations

import importlib.util
import os
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock


HELPER_PATH = Path(__file__).with_name("safe-work-root.py")
WORK_ROOT_HELPER_PATH = Path(__file__).with_name("task-bound-work-root.py")
sys.dont_write_bytecode = True
SPEC = importlib.util.spec_from_file_location("flashgate_safe_work_root", HELPER_PATH)
if SPEC is None or SPEC.loader is None:
    raise RuntimeError(f"unable to load helper: {HELPER_PATH}")
HELPER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(HELPER)
WORK_ROOT_SPEC = importlib.util.spec_from_file_location(
    "flashgate_task_bound_work_root", WORK_ROOT_HELPER_PATH
)
if WORK_ROOT_SPEC is None or WORK_ROOT_SPEC.loader is None:
    raise RuntimeError(f"unable to load helper: {WORK_ROOT_HELPER_PATH}")
WORK_ROOT_HELPER = importlib.util.module_from_spec(WORK_ROOT_SPEC)
WORK_ROOT_SPEC.loader.exec_module(WORK_ROOT_HELPER)


@unittest.skipUnless(
    os.name == "posix"
    and hasattr(os, "O_DIRECTORY")
    and hasattr(os, "O_NOFOLLOW"),
    "descriptor-relative POSIX directory operations are required",
)
class SafeWorkRootTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary = tempfile.TemporaryDirectory(
            prefix="flashgate-safe-work-root-",
            dir=WORK_ROOT_HELPER.resolve_validation_work_root(),
        )
        self.root = Path(self.temporary.name)
        self.home = self.root / "home"
        self.cache = self.home / ".cache"
        self.base = self.cache / "flashgate-mcp-validation"
        self.base.mkdir(parents=True, mode=0o700)
        self.base.chmod(0o700)
        self.sentinel = self.root / "outside-sentinel"
        self.sentinel.write_text("preserve", encoding="utf-8")

    def tearDown(self) -> None:
        self.assertEqual(self.sentinel.read_text(encoding="utf-8"), "preserve")
        self.temporary.cleanup()

    def test_create_and_remove_direct_child(self) -> None:
        expected = self.base / "direct-child"
        created = HELPER.create_run_directory(self.base, "direct-child")
        self.assertEqual(created, expected)
        self.assertTrue(expected.is_dir())
        (expected / "nested").mkdir()
        (expected / "nested" / "file.txt").write_text(
            "content",
            encoding="utf-8",
        )

        HELPER.remove_run_directory(self.base, "direct-child")

        self.assertFalse(expected.exists())

    def test_base_component_exchange_stays_on_bound_descriptor(self) -> None:
        run_id = "exchange-run"
        original_run = self.base / run_id
        original_run.mkdir()
        (original_run / "owned.txt").write_text("remove", encoding="utf-8")

        foreign_cache = self.root / "foreign-cache"
        foreign_base = foreign_cache / "flashgate-mcp-validation"
        foreign_run = foreign_base / run_id
        foreign_run.mkdir(parents=True)
        foreign_victim = foreign_run / "foreign.txt"
        foreign_victim.write_text("preserve", encoding="utf-8")

        bound_cache = self.home / ".cache-bound"
        original_open = HELPER.open_bound_base_directory
        captured_fd = -1

        def bind_then_exchange(base: Path) -> int:
            nonlocal captured_fd
            captured_fd = original_open(base)
            self.cache.rename(bound_cache)
            self.cache.symlink_to(foreign_cache, target_is_directory=True)
            return captured_fd

        with mock.patch.object(
            HELPER,
            "open_bound_base_directory",
            side_effect=bind_then_exchange,
        ):
            HELPER.remove_run_directory(self.base, run_id)

        self.assertFalse(
            (bound_cache / "flashgate-mcp-validation" / run_id).exists()
        )
        self.assertEqual(foreign_victim.read_text(encoding="utf-8"), "preserve")
        self.assertEqual(self.sentinel.read_text(encoding="utf-8"), "preserve")
        with self.assertRaises(OSError):
            os.fstat(captured_fd)


if __name__ == "__main__":
    unittest.main(verbosity=2)
