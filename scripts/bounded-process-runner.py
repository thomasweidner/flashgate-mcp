#!/usr/bin/env python3
"""Run one command with bounded output, timeout, and process-group cleanup."""

from __future__ import annotations

import argparse
import os
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


def terminate_tree(process: subprocess.Popen[bytes]) -> None:
    if process.poll() is not None:
        return
    try:
        if os.name == "posix":
            os.killpg(process.pid, signal.SIGKILL)
        else:
            process.kill()
    except (OSError, ProcessLookupError):
        try:
            process.kill()
        except (OSError, ProcessLookupError):
            pass


def capture_stream(
    stream: object,
    capture: Capture,
    maximum_bytes: int,
    process: subprocess.Popen[bytes],
    limit_event: threading.Event,
    stream_name: str,
) -> None:
    try:
        while True:
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
                terminate_tree(process)
                return
    except Exception as exception:  # fail closed; type only, no untrusted text
        capture.failure = (
            f"{stream_name}ReadFailure:{type(exception).__name__}"
        )
        limit_event.set()
        terminate_tree(process)


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
    timed_wait_deadline: float | None = None

    def activate_timed_wait() -> None:
        nonlocal timed_wait_deadline
        if timed_wait_deadline is not None:
            raise RuntimeError("timed wait was activated more than once")
        timed_wait_deadline = time.monotonic() + args.timeout_seconds

    try:
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
        stdout_thread = threading.Thread(
            target=capture_stream,
            args=(
                process.stdout,
                stdout_capture,
                args.maximum_bytes,
                process,
                limit_event,
                "Stdout",
            ),
            daemon=True,
        )
        stderr_thread = threading.Thread(
            target=capture_stream,
            args=(
                process.stderr,
                stderr_capture,
                args.maximum_bytes,
                process,
                limit_event,
                "Stderr",
            ),
            daemon=True,
        )
        stdout_thread.start()
        stderr_thread.start()

        if before_timed_wait is not None:
            barrier_failure = before_timed_wait(process)
            if barrier_failure:
                failure_reason = barrier_failure
                terminate_tree(process)

        if not failure_reason and at_timed_wait_activation is not None:
            barrier_failure = at_timed_wait_activation(
                process,
                activate_timed_wait,
            )
            if barrier_failure:
                failure_reason = barrier_failure
                terminate_tree(process)

        if not failure_reason:
            if timed_wait_deadline is None:
                activate_timed_wait()
            while process.poll() is None:
                if limit_event.wait(timeout=0.02):
                    break
                if time.monotonic() >= timed_wait_deadline:
                    timed_out = True
                    failure_reason = "Timeout"
                    terminate_tree(process)
                    break

        try:
            process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            terminate_tree(process)
            failure_reason = failure_reason or "ProcessCleanupFailure"
            process.wait(timeout=5)

        stdout_thread.join(timeout=5)
        stderr_thread.join(timeout=5)
        if stdout_thread.is_alive() or stderr_thread.is_alive():
            failure_reason = failure_reason or "StreamCleanupFailure"
            terminate_tree(process)

        exit_code = str(process.returncode)
        output_limit_exceeded = (
            stdout_capture.exceeded or stderr_capture.exceeded
        )
        failure_reason = (
            failure_reason
            or stdout_capture.failure
            or stderr_capture.failure
        )
        if (
            not timed_out
            and not output_limit_exceeded
            and not failure_reason
            and process.returncode == 0
        ):
            status = "PASS"
        elif not failure_reason:
            failure_reason = "NonZeroExit"
    except Exception as exception:  # fail closed; type only, no path/output
        failure_reason = f"StartOrProcessFailure:{type(exception).__name__}"
        if process is not None:
            terminate_tree(process)
    finally:
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
