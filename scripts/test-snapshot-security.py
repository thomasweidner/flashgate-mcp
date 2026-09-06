#!/usr/bin/env python3
"""Negative fixtures for the native snapshot validator."""

from __future__ import annotations

import hashlib
import importlib.util
import io
import json
import os
import subprocess
import sys
import tarfile
import tempfile
import unittest
from pathlib import Path


VALIDATOR = Path(__file__).with_name("validate-snapshot.py")
sys.dont_write_bytecode = True
WORK_ROOT_HELPER_PATH = Path(__file__).with_name("task-bound-work-root.py")
WORK_ROOT_SPEC = importlib.util.spec_from_file_location(
    "flashgate_task_bound_work_root", WORK_ROOT_HELPER_PATH
)
if WORK_ROOT_SPEC is None or WORK_ROOT_SPEC.loader is None:
    raise RuntimeError(f"unable to load helper: {WORK_ROOT_HELPER_PATH}")
WORK_ROOT_HELPER = importlib.util.module_from_spec(WORK_ROOT_SPEC)
WORK_ROOT_SPEC.loader.exec_module(WORK_ROOT_HELPER)


class SnapshotSecurityTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary = tempfile.TemporaryDirectory(
            prefix="flashgate-snapshot-security-",
            dir=WORK_ROOT_HELPER.resolve_validation_work_root(),
        )
        self.root = Path(self.temporary.name)
        self.sentinel = self.root / "outside-sentinel"
        self.sentinel.write_text("preserve", encoding="utf-8")

    def tearDown(self) -> None:
        self.assertEqual(self.sentinel.read_text(encoding="utf-8"), "preserve")
        self.temporary.cleanup()

    def write_manifest(
        self,
        files: dict[str, bytes],
        *,
        length_delta: int = 0,
        wrong_hash: bool = False,
    ) -> Path:
        records = []
        for name, payload in files.items():
            digest = hashlib.sha256(payload).hexdigest()
            if wrong_hash:
                digest = "0" * 64
            records.append(
                {
                    "Path": name,
                    "Type": "file",
                    "Length": len(payload) + length_delta,
                    "SHA256": digest,
                }
            )
        path = self.root / "manifest.json"
        path.write_text(
            json.dumps(
                {
                    "schema": "flashgate-git-snapshot-manifest/v1",
                    "files": records,
                }
            ),
            encoding="utf-8",
        )
        return path

    def write_tar(self, members: list[tuple[tarfile.TarInfo, bytes]]) -> Path:
        path = self.root / "snapshot.tar"
        with tarfile.open(path, "w") as archive:
            for information, payload in members:
                archive.addfile(information, io.BytesIO(payload))
        return path

    @staticmethod
    def regular(name: str, payload: bytes = b"content") -> tuple[tarfile.TarInfo, bytes]:
        information = tarfile.TarInfo(name)
        information.size = len(payload)
        information.mode = 0o600
        return information, payload

    def run_validator(
        self,
        archive: Path,
        manifest: Path,
        *,
        should_pass: bool,
    ) -> None:
        extraction = self.root / f"extract-{os.urandom(4).hex()}"
        result = subprocess.run(
            [
                sys.executable,
                str(VALIDATOR),
                "--archive",
                str(archive),
                "--manifest",
                str(manifest),
                "--extract-root",
                str(extraction),
            ],
            check=False,
            capture_output=True,
            text=True,
        )
        if should_pass:
            self.assertEqual(result.returncode, 0, result.stderr)
        else:
            self.assertNotEqual(result.returncode, 0, result.stdout)

    def test_valid_snapshot(self) -> None:
        files = {"dir/file.txt": b"content"}
        archive = self.write_tar([self.regular("dir/file.txt")])
        self.run_validator(archive, self.write_manifest(files), should_pass=True)

    def test_path_and_type_attacks_are_rejected(self) -> None:
        fixtures: list[tuple[str, tarfile.TarInfo]] = []
        for label, name in (
            ("parent", "../escape"),
            ("absolute", "/tmp/escape"),
            ("backslash", r"..\escape"),
            ("normalization", "dir/../file.txt"),
        ):
            information = tarfile.TarInfo(name)
            information.size = 7
            fixtures.append((label, information))

        for label, entry_type in (
            ("symlink", tarfile.SYMTYPE),
            ("hardlink", tarfile.LNKTYPE),
            ("fifo", tarfile.FIFOTYPE),
            ("device", tarfile.CHRTYPE),
            ("socket", b"s"),
        ):
            information = tarfile.TarInfo("file.txt")
            information.type = entry_type
            information.linkname = "target"
            information.devmajor = 1
            information.devminor = 3
            fixtures.append((label, information))

        for label, information in fixtures:
            with self.subTest(label=label):
                archive = self.write_tar([(information, b"content")])
                manifest = self.write_manifest({"file.txt": b"content"})
                self.run_validator(archive, manifest, should_pass=False)

    def test_duplicate_path_is_rejected(self) -> None:
        archive = self.write_tar(
            [self.regular("file.txt"), self.regular("file.txt")]
        )
        self.run_validator(
            archive,
            self.write_manifest({"file.txt": b"content"}),
            should_pass=False,
        )

    def test_unexpected_and_missing_files_are_rejected(self) -> None:
        with self.subTest(case="unexpected"):
            archive = self.write_tar(
                [self.regular("file.txt"), self.regular("extra.txt")]
            )
            self.run_validator(
                archive,
                self.write_manifest({"file.txt": b"content"}),
                should_pass=False,
            )
        with self.subTest(case="missing"):
            archive = self.write_tar([self.regular("file.txt")])
            self.run_validator(
                archive,
                self.write_manifest(
                    {"file.txt": b"content", "missing.txt": b"content"}
                ),
                should_pass=False,
            )

    def test_length_and_hash_mismatch_are_rejected(self) -> None:
        archive = self.write_tar([self.regular("file.txt")])
        with self.subTest(case="length"):
            self.run_validator(
                archive,
                self.write_manifest(
                    {"file.txt": b"content"},
                    length_delta=1,
                ),
                should_pass=False,
            )
        with self.subTest(case="hash"):
            self.run_validator(
                archive,
                self.write_manifest(
                    {"file.txt": b"content"},
                    wrong_hash=True,
                ),
                should_pass=False,
            )


if __name__ == "__main__":
    unittest.main(verbosity=2)
