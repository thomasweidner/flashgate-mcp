#!/usr/bin/env python3
"""Focused tests for the native-Linux authoritative corpus-parent gate."""

from __future__ import annotations

import importlib.util
import tempfile
import unittest
from pathlib import Path


PATH = Path(__file__).with_name("benchmark-corpus-parent.py")
SPEC = importlib.util.spec_from_file_location("benchmark_corpus_parent", PATH)
if SPEC is None or SPEC.loader is None:
    raise RuntimeError(f"unable to load {PATH}")
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class CorpusParentTests(unittest.TestCase):
    def test_required_and_absolute(self) -> None:
        for value in (None, "", "relative"):
            with self.subTest(value=value), self.assertRaises(RuntimeError):
                MODULE.validate_linux_corpus_parent(value, "")

    def test_must_be_below_home(self) -> None:
        with self.assertRaises(RuntimeError):
            MODULE.validate_linux_corpus_parent("/home", "")
        with tempfile.TemporaryDirectory(dir="/tmp") as parent:
            with self.assertRaises(RuntimeError):
                MODULE.validate_linux_corpus_parent(parent, "")

    def test_rejects_non_ext4_and_accepts_ext4(self) -> None:
        with tempfile.TemporaryDirectory(dir="/home") as parent:
            path = Path(parent)
            mount = str(path).replace(" ", "\\040")
            prefix = f"1 0 0:1 / {mount} rw -"
            with self.assertRaisesRegex(RuntimeError, "native ext4"):
                MODULE.validate_linux_corpus_parent(str(path), f"{prefix} tmpfs tmpfs rw")
            self.assertEqual(
                MODULE.validate_linux_corpus_parent(str(path), f"{prefix} ext4 /dev/test rw"),
                path,
            )

    def test_rejects_link_alias(self) -> None:
        with tempfile.TemporaryDirectory(dir="/home") as parent:
            root = Path(parent)
            target = root / "target"
            alias = root / "alias"
            target.mkdir()
            alias.symlink_to(target, target_is_directory=True)
            with self.assertRaises(RuntimeError):
                MODULE.validate_linux_corpus_parent(str(alias), "")


if __name__ == "__main__":
    unittest.main()
