#!/usr/bin/env python3
"""Permanent negative and positive tests for task-bound Python scratch roots."""

from __future__ import annotations

import importlib.util
import os
import sys
import tempfile
import unittest
from pathlib import Path


HELPER_PATH = Path(__file__).with_name("task-bound-work-root.py")
sys.dont_write_bytecode = True
SPEC = importlib.util.spec_from_file_location("flashgate_task_bound_work_root", HELPER_PATH)
if SPEC is None or SPEC.loader is None:
    raise RuntimeError(f"unable to load helper: {HELPER_PATH}")
HELPER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(HELPER)


class TaskBoundWorkRootTests(unittest.TestCase):
    def setUp(self) -> None:
        self.environ = dict(os.environ)
        self.work_root = HELPER.resolve_validation_work_root(self.environ)

    def test_valid_bound_root(self) -> None:
        self.assertEqual(HELPER.resolve_validation_work_root(self.environ), self.work_root)

    def test_missing_work_root_rejected(self) -> None:
        values = dict(self.environ)
        values.pop("FLASHGATE_WORK_ROOT", None)
        with self.assertRaises(RuntimeError):
            HELPER.resolve_validation_work_root(values)

    def test_relative_work_root_rejected(self) -> None:
        values = dict(self.environ, FLASHGATE_WORK_ROOT="relative-work-root")
        with self.assertRaises(RuntimeError):
            HELPER.resolve_validation_work_root(values)

    def test_filesystem_work_root_rejected(self) -> None:
        values = dict(self.environ, FLASHGATE_WORK_ROOT=self.work_root.anchor)
        values.pop("FLASHGATE_TASK_ROOT", None)
        with self.assertRaises(RuntimeError):
            HELPER.resolve_validation_work_root(values)

    def test_relative_task_root_rejected(self) -> None:
        values = dict(self.environ, FLASHGATE_TASK_ROOT="relative-task-root")
        with self.assertRaises(RuntimeError):
            HELPER.resolve_validation_work_root(values)

    def test_filesystem_task_root_rejected(self) -> None:
        values = dict(self.environ, FLASHGATE_TASK_ROOT=self.work_root.anchor)
        with self.assertRaises(RuntimeError):
            HELPER.resolve_validation_work_root(values)

    def test_work_root_must_be_below_task_root(self) -> None:
        values = dict(self.environ, FLASHGATE_TASK_ROOT=str(self.work_root))
        with self.assertRaises(RuntimeError):
            HELPER.resolve_validation_work_root(values)

    def test_foreign_task_root_rejected(self) -> None:
        with tempfile.TemporaryDirectory(
            prefix="flashgate-foreign-task-contract-", dir=self.work_root
        ) as temporary:
            foreign_task_root = Path(temporary) / "foreign-task"
            foreign_task_root.mkdir()
            values = dict(self.environ, FLASHGATE_TASK_ROOT=str(foreign_task_root))
            with self.assertRaises(RuntimeError):
                HELPER.resolve_validation_work_root(values)

    def test_link_work_root_rejected(self) -> None:
        with tempfile.TemporaryDirectory(
            prefix="flashgate-task-root-contract-", dir=self.work_root
        ) as temporary:
            parent = Path(temporary)
            target = parent / "target"
            link = parent / "link"
            target.mkdir()
            try:
                link.symlink_to(target, target_is_directory=True)
            except OSError as exc:
                self.skipTest(f"directory symlink unavailable: {exc}")
            values = dict(self.environ, FLASHGATE_WORK_ROOT=str(link))
            with self.assertRaises(RuntimeError):
                HELPER.resolve_validation_work_root(values)


if __name__ == "__main__":
    unittest.main()
