#!/usr/bin/env python3
"""Run one command with bounded output, timeout, and process-group cleanup."""

from __future__ import annotations

import argparse
import os
import select
import signal
import subprocess
import sys
import threading
import time
from collections.abc import Callable
from dataclasses import dataclass
from pathlib import Path


@dataclass
class Capture:
    data: bytearray
    exceeded: bool = False
    failure: str = ""


@dataclass(frozen=True)
class LinuxIdentity:
    pid: int
    parent: int
    group: int
    session: int
    start: int
    state: str


def linux_identity(pid: int) -> LinuxIdentity:
    with open(f"/proc/{pid}/stat", "rb") as stream:
        data = stream.read(4097)
    if len(data) > 4096:
        raise RuntimeError("ProcessIdentityUnknown")
    fields = data[data.rfind(b")") + 2 :].split()
    return LinuxIdentity(
        pid, int(fields[1]), int(fields[2]), int(fields[3]),
        int(fields[19]), fields[0].decode("ascii"),
    )


class OwnedLinuxGroup:
    """Keep our session leader waitable until group cleanup is confirmed.

    WNOWAIT observes exit without releasing the leader's PID. Together with
    start time, direct parenthood and setsid's session/group identity, this
    prevents a stale numeric PGID from becoming signal authority. Only this
    coordinator reaps the leader; stream threads never poll, wait or signal.
    """

    def __init__(self, process: subprocess.Popen[bytes]) -> None:
        self.process = process
        self.identity = linux_identity(process.pid)
        identity = self.identity
        if (
            identity.parent != os.getpid()
            or identity.group != process.pid
            or identity.session != process.pid
        ):
            raise RuntimeError("ProcessGroupIdentityUnknown")
        self.validate()

    def validate(self) -> None:
        if self.process.returncode is not None:
            raise RuntimeError("ProcessGroupLeaderReaped")
        current = linux_identity(self.process.pid)
        if (
            current.pid, current.parent, current.group,
            current.session, current.start,
        ) != (
            self.identity.pid, self.identity.parent, self.identity.group,
            self.identity.session, self.identity.start,
        ):
            raise RuntimeError("ProcessGroupIdentityChanged")
        # ECHILD also rejects a leader reaped outside this coordinator, even
        # if a numeric PID has subsequently been reused.
        os.waitid(
            os.P_PID, self.identity.pid,
            os.WEXITED | os.WNOHANG | os.WNOWAIT,
        )

    def exited(self) -> bool:
        self.validate()
        return os.waitid(
            os.P_PID, self.identity.pid,
            os.WEXITED | os.WNOHANG | os.WNOWAIT,
        ) is not None

    def live_members(self, deadline: float) -> list[LinuxIdentity]:
        self.validate()
        members = []
        # This bounded readback does not establish ownership by searching for
        # PIDs or ancestry. Ownership is the still-pinned session above. Read
        # only identities in that session; never signal a discovered foreign
        # group, even when a fixture has moved into another group.
        with os.scandir("/proc") as entries:
            inspected = 0
            for entry in entries:
                if time.monotonic() >= deadline or inspected >= 65536:
                    raise RuntimeError("ProcessGroupReadbackLimit")
                if not entry.name.isascii() or not entry.name.isdecimal():
                    continue
                inspected += 1
                try:
                    identity = linux_identity(int(entry.name))
                except (FileNotFoundError, ProcessLookupError):
                    continue
                if identity.session != self.identity.session:
                    continue
                if identity.group != self.identity.group:
                    raise RuntimeError("ProcessGroupMembershipChanged")
                if identity.state not in {"Z", "X"}:
                    members.append(identity)
        self.validate()
        return members

    def terminate(self, deadline: float) -> str:
        try:
            self.validate()
            # The unreaped, identity-checked leader pins the session/group
            # number even after exit and reparenting of its children.
            os.killpg(self.identity.group, signal.SIGKILL)
            while time.monotonic() < deadline:
                if not self.live_members(deadline):
                    return ""
                time.sleep(0.01)
            return "ProcessGroupCleanupUnconfirmed"
        except (OSError, ValueError, IndexError, RuntimeError):
            return "ProcessGroupCleanupUnknown"


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(add_help=False)
    parser.add_argument("--timeout-seconds", type=float, required=True)
    parser.add_argument("--maximum-bytes", type=int, required=True)
    parser.add_argument("--stdout-file", required=True)
    parser.add_argument("--stderr-file", required=True)
    parser.add_argument("--working-directory", required=True)
    parser.add_argument("command", nargs=argparse.REMAINDER)
    args = parser.parse_args()
    if args.command and args.command[0] == "--":
        args.command = args.command[1:]
    if not args.command:
        parser.error("a command is required after --")
    if not 0.1 <= args.timeout_seconds <= 300:
        parser.error("timeout must be between 0.1 and 300 seconds")
    if not 1024 <= args.maximum_bytes <= 1048576:
        parser.error("maximum bytes must be between 1024 and 1048576")
    return args


def capture_stream(
    stream: object,
    capture: Capture,
    maximum_bytes: int,
    limit_event: threading.Event,
    stream_name: str,
    stop_event: threading.Event,
) -> None:
    try:
        native_linux = sys.platform == "linux"
        if native_linux:
            os.set_blocking(stream.fileno(), False)
        while not stop_event.is_set():
            if native_linux:
                if not select.select([stream], [], [], 0.02)[0]:
                    continue
                try:
                    chunk = os.read(stream.fileno(), 4096)
                except BlockingIOError:
                    continue
            else:
                chunk = stream.read(4096)
            if not chunk:
                return
            remaining = maximum_bytes - len(capture.data)
            if remaining > 0:
                capture.data.extend(chunk[:remaining])
            if len(chunk) > remaining:
                capture.exceeded = True
                capture.failure = f"{stream_name}LimitExceeded"
                limit_event.set()
                return
    except Exception as exception:  # fail closed; type only, no untrusted text
        capture.failure = (
            f"{stream_name}ReadFailure:{type(exception).__name__}"
        )
        limit_event.set()
    finally:
        stream.close()


def write_capture(path: str, data: bytearray) -> None:
    Path(path).write_bytes(bytes(data))


def _run_bounded_process(
    args: argparse.Namespace,
    before_timed_wait: Callable[[subprocess.Popen[bytes]], str] | None = None,
    at_timed_wait_activation: (
        Callable[
            [subprocess.Popen[bytes], Callable[[], None]],
            str,
        ]
        | None
    ) = None,
) -> tuple[str, str, str, str, str, str]:
    stdout_capture = Capture(bytearray())
    stderr_capture = Capture(bytearray())
    attempted = True
    status = "FAIL"
    exit_code = ""
    timed_out = False
    output_limit_exceeded = False
    failure_reason = ""
    process: subprocess.Popen[bytes] | None = None
    stdout_thread: threading.Thread | None = None
    stderr_thread: threading.Thread | None = None
    limit_event = threading.Event()
    stop_event = threading.Event()
    owned_group: OwnedLinuxGroup | None = None
    timed_wait_deadline: float | None = None

    def activate_timed_wait() -> None:
        nonlocal timed_wait_deadline
        if timed_wait_deadline is not None:
            raise RuntimeError("timed wait was activated more than once")
        timed_wait_deadline = time.monotonic() + args.timeout_seconds

    try:
        if os.name == "posix" and (
            sys.platform != "linux"
            or signal.getsignal(signal.SIGCHLD) != signal.SIG_DFL
        ):
            raise RuntimeError("WaitableLinuxLeaderRequired")
        process = subprocess.Popen(
            args.command,
            cwd=args.working_directory,
            stdin=subprocess.DEVNULL,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            start_new_session=(os.name == "posix"),
        )
        assert process.stdout is not None
        assert process.stderr is not None
        if sys.platform == "linux":
            owned_group = OwnedLinuxGroup(process)
        stdout_thread = threading.Thread(
            target=capture_stream,
            args=(
                process.stdout,
                stdout_capture,
                args.maximum_bytes,
                limit_event,
                "Stdout",
                stop_event,
            ),
            daemon=True,
        )
        stderr_thread = threading.Thread(
            target=capture_stream,
            args=(
                process.stderr,
                stderr_capture,
                args.maximum_bytes,
                limit_event,
                "Stderr",
                stop_event,
            ),
            daemon=True,
        )
        stdout_thread.start()
        stderr_thread.start()

        if before_timed_wait is not None:
            barrier_failure = before_timed_wait(process)
            if barrier_failure:
                failure_reason = barrier_failure

        if not failure_reason and at_timed_wait_activation is not None:
            barrier_failure = at_timed_wait_activation(
                process,
                activate_timed_wait,
            )
            if barrier_failure:
                failure_reason = barrier_failure

        if not failure_reason:
            if timed_wait_deadline is None:
                activate_timed_wait()
            while not (
                owned_group.exited() if owned_group is not None
                else process.poll() is not None
            ):
                if limit_event.wait(timeout=0.02):
                    break
                if time.monotonic() >= timed_wait_deadline:
                    timed_out = True
                    failure_reason = "Timeout"
                    break
    except Exception as exception:  # fail closed; type only, no path/output
        failure_reason = failure_reason or (
            f"StartOrProcessFailure:{type(exception).__name__}"
        )
    finally:
        cleanup_failures = []
        if process is not None:
            deadline = time.monotonic() + 3.0
            if owned_group is not None:
                cleanup_reason = owned_group.terminate(deadline)
                if cleanup_reason:
                    cleanup_failures.append(cleanup_reason)
            elif sys.platform == "linux":
                cleanup_failures.append("ProcessGroupIdentityUnknown")
            elif process.poll() is None:
                process.kill()
            try:
                process.wait(timeout=max(0.001, deadline - time.monotonic()))
            except subprocess.TimeoutExpired:
                cleanup_failures.append("ProcessCleanupFailure")
            exit_code = (
                str(process.returncode) if process.returncode is not None else ""
            )
            for thread in (stdout_thread, stderr_thread):
                if thread is not None:
                    thread.join(timeout=max(0, deadline - time.monotonic() - 0.1))
            stop_event.set()
            for thread in (stdout_thread, stderr_thread):
                if thread is not None and thread.is_alive():
                    thread.join(timeout=0.1)
                    cleanup_failures.append("StreamCleanupFailure")
            for stream, thread in (
                (process.stdout, stdout_thread), (process.stderr, stderr_thread),
            ):
                if stream is not None and (thread is None or not thread.is_alive()):
                    stream.close()
        output_limit_exceeded = stdout_capture.exceeded or stderr_capture.exceeded
        failure_reason = (
            failure_reason or stdout_capture.failure or stderr_capture.failure
        )
        if process is not None and process.returncode not in (None, 0):
            failure_reason = failure_reason or "NonZeroExit"
        if cleanup_failures:
            failure_reason = ";".join(filter(None, (
                failure_reason, *dict.fromkeys(cleanup_failures),
            )))
        if process is not None and process.returncode == 0 and not failure_reason:
            status = "PASS"
        write_capture(args.stdout_file, stdout_capture.data)
        write_capture(args.stderr_file, stderr_capture.data)

    fields = (
        str(attempted).lower(),
        status,
        exit_code or "none",
        str(timed_out).lower(),
        str(output_limit_exceeded).lower(),
        failure_reason or "None",
    )
    return fields


def main() -> int:
    args = parse_args()
    fields = _run_bounded_process(args)
    sys.stdout.write("\t".join(fields) + "\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
