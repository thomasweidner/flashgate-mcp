#!/usr/bin/env python3
"""Linux security regressions for the existing artifact-verifier runner."""

from __future__ import annotations

import argparse
import ctypes
import dataclasses
import importlib.util
import json
import os
import signal
import subprocess
import sys
import tempfile
import threading
import time
import unittest
from pathlib import Path
from unittest import mock


def wait_for(predicate, seconds=2.0):
    deadline = time.monotonic() + seconds
    while time.monotonic() < deadline:
        if predicate():
            return
        time.sleep(0.005)
    raise AssertionError("bounded fixture wait expired")


def fixture(mode, directory):
    if mode == "child":
        (directory / "ready").write_text(str(os.getpid()), encoding="ascii")
        # Deliberately ignore parent death, unlike the older cooperative helper.
        while True:
            if (directory / "release").exists():
                (directory / "released").touch()
            if (directory / "activate").exists():
                (directory / "activated").touch()
            time.sleep(0.005)
    child = subprocess.Popen([
        sys.executable, "-B", __file__, "--fixture", "child", str(directory),
    ], start_new_session=(mode == "foreign-stream"))
    (directory / "child").write_text(str(child.pid), encoding="ascii")
    wait_for(lambda: (directory / "ready").exists())
    print("stdout before parent exit", flush=True)
    print("stderr before parent exit", file=sys.stderr, flush=True)
    wait_for(lambda: (directory / "exit").exists(), 10)
    return 31


class GroupSecurityTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(dir=WORK_ROOT)
        self.directory = Path(self.temporary.name)
        self.fds = []
        self.children = []
        self.threads = set(threading.enumerate())

    def bind(self, pid):
        before = RUNNER.linux_identity(pid)
        fd = os.pidfd_open(pid)
        after = RUNNER.linux_identity(pid)
        self.assertEqual(before.start, after.start)
        self.fds.append(fd)
        return before

    def tearDown(self):
        # pidfds target only this test's bound fixtures, never a reused PID/PGID.
        for fd in self.fds:
            try:
                signal.pidfd_send_signal(fd, signal.SIGKILL)
            except ProcessLookupError:
                pass
            finally:
                os.close(fd)
        for pid in self.children:
            def reaped():
                try:
                    return os.waitpid(pid, os.WNOHANG)[0] == pid
                except ChildProcessError:
                    return True
            wait_for(reaped)
        self.assertEqual(set(threading.enumerate()), self.threads)
        self.temporary.cleanup()

    def run_case(self, command, before=None, timeout=0.2):
        args = argparse.Namespace(
            command=command, working_directory=str(self.directory),
            timeout_seconds=timeout, maximum_bytes=1024,
            stdout_file=str(self.directory / "stdout"),
            stderr_file=str(self.directory / "stderr"),
        )
        start = time.monotonic()
        result = RUNNER._run_bounded_process(args, before_timed_wait=before)
        elapsed = time.monotonic() - start
        self.assertLess(elapsed, timeout + 4)
        print("RunnerEvidence: " + json.dumps({
            "result": result, "elapsedSeconds": elapsed,
            "captureThreadsRemaining": len(set(threading.enumerate()) - self.threads),
        }), flush=True)
        return result

    def barrier(self, parent, boundary):
        parent_identity = self.bind(parent.pid)
        self.assertEqual(parent_identity.group, parent.pid)
        self.assertEqual(parent_identity.session, parent.pid)
        wait_for(lambda: (self.directory / "child").exists()
                 and (self.directory / "ready").exists())
        pid = int((self.directory / "child").read_text(encoding="ascii"))
        child = self.bind(pid)
        self.children.append(pid)
        self.assertEqual((child.parent, child.group, child.session),
                         (parent.pid, parent.pid, parent.pid))
        if boundary != "ready":
            (self.directory / "release").touch()
            wait_for(lambda: (self.directory / "released").exists())
        if boundary == "activated":
            (self.directory / "activate").touch()
            wait_for(lambda: (self.directory / "activated").exists())
        (self.directory / "exit").touch()
        # Observe exit without reaping: the runner must retain its identity pin.
        wait_for(lambda: os.waitid(os.P_PID, parent.pid,
                 os.WEXITED | os.WNOHANG | os.WNOWAIT) is not None)
        self.assertIsNone(parent.returncode)
        current = RUNNER.linux_identity(pid)
        self.assertNotIn(current.state, {"Z", "X"})
        self.assertEqual((current.start, current.group, current.session),
                         (child.start, parent.pid, parent.pid))
        self.assertNotEqual(current.parent, parent.pid)
        print("OwnedIdentityEvidence: " + json.dumps({
            "boundary": boundary, "parent": dataclasses.asdict(parent_identity),
            "childBefore": dataclasses.asdict(child),
            "childAfterParentExit": dataclasses.asdict(current),
            "leaderStillWaitable": True, "childInitiallyAlive": True,
        }), flush=True)
        return "ReadyParentNotAlive"

    def test_uncooperative_child_after_parent_exit(self):
        for boundary in ("ready", "released", "activated"):
            with self.subTest(boundary=boundary):
                case = self.directory / boundary
                case.mkdir()
                previous = self.directory
                self.directory = case
                try:
                    result = self.run_case([
                        sys.executable, "-B", __file__, "--fixture", "parent", str(case),
                    ], lambda parent: self.barrier(parent, boundary))
                    self.assertEqual(result[1:],
                                     ("FAIL", "31", "false", "false", "ReadyParentNotAlive"))
                    pid = int((case / "child").read_text(encoding="ascii"))
                    try:
                        self.assertIn(RUNNER.linux_identity(pid).state, {"Z", "X"})
                    except FileNotFoundError:
                        pass
                    self.assertIn(b"stdout before parent exit", (case / "stdout").read_bytes())
                    self.assertIn(b"stderr before parent exit", (case / "stderr").read_bytes())
                finally:
                    self.directory = previous

    def test_positive_nonzero_timeout_and_output_limits(self):
        cases = [
            ("print('ok')", "PASS", "None"),
            ("raise SystemExit(7)", "FAIL", "NonZeroExit"),
            ("import time;time.sleep(10)", "FAIL", "Timeout"),
            ("print('x'*8192)", "FAIL", "StdoutLimitExceeded"),
            ("import sys;sys.stderr.write('x'*8192)", "FAIL", "StderrLimitExceeded"),
        ]
        for code, status, reason in cases:
            with self.subTest(reason=reason):
                result = self.run_case([sys.executable, "-c", code])
                self.assertEqual(result[1], status)
                self.assertEqual(result[5], reason)
                self.assertLessEqual((self.directory / "stdout").stat().st_size, 1024)
                self.assertLessEqual((self.directory / "stderr").stat().st_size, 1024)

    def test_identity_changes_never_signal(self):
        process = subprocess.Popen([sys.executable, "-c", "import time;time.sleep(10)"],
                                   start_new_session=True)
        self.bind(process.pid)
        try:
            group = RUNNER.OwnedLinuxGroup(process)
            original = group.identity
            for field in ("pid", "parent", "group", "session", "start"):
                with self.subTest(field=field):
                    group.identity = dataclasses.replace(original, **{field: getattr(original, field) + 1})
                    with mock.patch.object(RUNNER.os, "killpg") as kill:
                        self.assertEqual(group.terminate(time.monotonic() + 0.2),
                                         "ProcessGroupCleanupUnknown")
                        kill.assert_not_called()
            group.identity = original
            with mock.patch.object(RUNNER, "linux_identity", side_effect=PermissionError):
                with mock.patch.object(RUNNER.os, "killpg") as kill:
                    self.assertEqual(group.terminate(time.monotonic() + 0.2),
                                     "ProcessGroupCleanupUnknown")
                    kill.assert_not_called()
        finally:
            signal.pidfd_send_signal(self.fds[-1], signal.SIGKILL)
            process.wait(timeout=2)

    def test_foreign_group_and_reaped_leader_never_signal(self):
        foreign = subprocess.Popen([sys.executable, "-c", "import time;time.sleep(10)"],
                                   start_new_session=True)
        self.bind(foreign.pid)
        own = subprocess.Popen([sys.executable, "-c", "pass"], start_new_session=True)
        self.bind(own.pid)
        try:
            group = RUNNER.OwnedLinuxGroup(own)
            group.identity = RUNNER.linux_identity(foreign.pid)
            with mock.patch.object(RUNNER.os, "killpg") as kill:
                self.assertEqual(group.terminate(time.monotonic() + 0.2),
                                 "ProcessGroupCleanupUnknown")
                kill.assert_not_called()
            self.assertIsNone(foreign.poll())
            group.identity = RUNNER.linux_identity(own.pid)
            own.wait(timeout=2)
            with mock.patch.object(RUNNER.os, "killpg") as kill:
                self.assertEqual(group.terminate(time.monotonic() + 0.2),
                                 "ProcessGroupCleanupUnknown")
                kill.assert_not_called()
            self.assertIsNone(foreign.poll())
        finally:
            signal.pidfd_send_signal(self.fds[0], signal.SIGKILL)
            foreign.wait(timeout=2)
            own.wait(timeout=2)

    def test_unconfirmed_cleanup_cannot_pass(self):
        with mock.patch.object(RUNNER.OwnedLinuxGroup, "terminate",
                               return_value="ProcessGroupCleanupUnconfirmed"):
            result = self.run_case([sys.executable, "-c", "pass"])
        self.assertEqual(result[1], "FAIL")
        self.assertIn("ProcessGroupCleanupUnconfirmed", result[5])

    def test_signal_failure_and_readback_deadline(self):
        process = subprocess.Popen([sys.executable, "-c", "import time;time.sleep(10)"],
                                   start_new_session=True)
        self.bind(process.pid)
        try:
            group = RUNNER.OwnedLinuxGroup(process)
            with mock.patch.object(RUNNER.os, "killpg", side_effect=PermissionError):
                self.assertEqual(group.terminate(time.monotonic() + 0.2),
                                 "ProcessGroupCleanupUnknown")
            with self.assertRaises(RuntimeError):
                group.live_members(time.monotonic() - 1)
            self.assertIsNone(process.poll())
        finally:
            signal.pidfd_send_signal(self.fds[-1], signal.SIGKILL)
            process.wait(timeout=2)

    def test_foreign_session_pipe_writer_is_not_signalled(self):
        def before(parent):
            self.bind(parent.pid)
            wait_for(lambda: (self.directory / "child").exists()
                     and (self.directory / "ready").exists())
            pid = int((self.directory / "child").read_text(encoding="ascii"))
            child = self.bind(pid)
            self.children.append(pid)
            self.assertNotEqual(child.session, parent.pid)
            (self.directory / "exit").touch()
            return "FixtureBarrierFailure"
        result = self.run_case([
            sys.executable, "-B", __file__, "--fixture", "foreign-stream", str(self.directory),
        ], before)
        self.assertEqual(result[1], "FAIL")
        self.assertTrue(result[5].startswith("FixtureBarrierFailure;"))
        self.assertIn("StreamCleanupFailure", result[5])
        child = RUNNER.linux_identity(self.children[-1])
        self.assertNotIn(child.state, {"Z", "X"})
        print("ForeignSessionEvidence: " + json.dumps({
            "child": dataclasses.asdict(child), "aliveAfterRunner": True,
            "streamFailureRetained": True,
        }), flush=True)

    def test_missing_command_fails_closed(self):
        result = self.run_case([str(self.directory / "missing-command")])
        self.assertEqual(result[1], "FAIL")
        self.assertEqual(result[5], "StartOrProcessFailure:FileNotFoundError")


if __name__ == "__main__":
    if len(sys.argv) > 1 and sys.argv[1] == "--fixture":
        raise SystemExit(fixture(sys.argv[2], Path(sys.argv[3])))
    parser = argparse.ArgumentParser()
    parser.add_argument("--runner", required=True)
    parser.add_argument("--work-root", required=True)
    options, remaining = parser.parse_known_args()
    if sys.platform != "linux":
        parser.error("native Linux is required")
    WORK_ROOT = Path(options.work_root)
    spec = importlib.util.spec_from_file_location("_verifier_runner_security", options.runner)
    RUNNER = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = RUNNER
    previous_bytecode = sys.dont_write_bytecode
    try:
        sys.dont_write_bytecode = True
        spec.loader.exec_module(RUNNER)
    finally:
        sys.dont_write_bytecode = previous_bytecode
    # Test-local adoption allows exact orphan reaping; the production runner
    # neither installs a subreaper nor starts a background cleanup component.
    libc = ctypes.CDLL(None, use_errno=True)
    previous = ctypes.c_int()
    if libc.prctl(37, ctypes.byref(previous), 0, 0, 0) != 0:
        raise RuntimeError("fixture subreaper read failed")
    if libc.prctl(36, 1, 0, 0, 0) != 0:
        raise RuntimeError("fixture subreaper setup failed")
    try:
        unittest.main(argv=[sys.argv[0], *remaining], verbosity=2)
    finally:
        libc.prctl(36, previous.value, 0, 0, 0)
