#!/usr/bin/env python3
"""Controlled subprocess behavior for verifier contract tests only."""

from __future__ import annotations

import argparse
import importlib.util
import os
import re
import subprocess
import sys
import time
from collections.abc import Callable
from dataclasses import dataclass
from pathlib import Path


@dataclass(frozen=True)
class ProcessIdentity:
    process_id: int
    state: str
    parent_process_id: int
    process_group_id: int
    session_id: int
    start_time_ticks: int


def write_atomic_record(path: str, value: bytes) -> None:
    temporary = Path(f"{path}.tmp")
    temporary.write_bytes(value)
    temporary.replace(path)


def read_process_identity(process_id: int) -> ProcessIdentity | None:
    try:
        stat = Path(f"/proc/{process_id}/stat").read_text(encoding="ascii")
        closing_parenthesis = stat.rfind(")")
        if closing_parenthesis < 0:
            return None
        fields = stat[closing_parenthesis + 2 :].split()
        if len(fields) < 20:
            return None
        return ProcessIdentity(
            process_id=process_id,
            state=fields[0],
            parent_process_id=int(fields[1]),
            process_group_id=int(fields[2]),
            session_id=int(fields[3]),
            start_time_ticks=int(fields[19]),
        )
    except (OSError, UnicodeError, ValueError):
        return None


def is_descendant_of(child_process_id: int, ancestor_process_id: int) -> bool:
    current_process_id = child_process_id
    visited: set[int] = set()
    for _ in range(64):
        if current_process_id in visited:
            return False
        visited.add(current_process_id)
        identity = read_process_identity(current_process_id)
        if identity is None:
            return False
        if identity.parent_process_id == ancestor_process_id:
            return True
        if (
            identity.parent_process_id <= 0
            or identity.parent_process_id == current_process_id
        ):
            return False
        current_process_id = identity.parent_process_id
    return False


def process_is_alive(process_id: int) -> bool:
    identity = read_process_identity(process_id)
    return identity is not None and identity.state != "Z"


def validate_controlled_child(
    parent_process_id: int,
    child_process_id: int,
    expected_child: ProcessIdentity | None,
    expected_parent: ProcessIdentity | None,
) -> tuple[str, ProcessIdentity | None, ProcessIdentity | None]:
    parent_identity = read_process_identity(parent_process_id)
    if parent_identity is None or parent_identity.state == "Z":
        return "ReadyParentNotAlive", None, None
    if (
        expected_parent is not None
        and parent_identity.start_time_ticks != expected_parent.start_time_ticks
    ):
        return "ReadyParentIdentityMismatch", None, None
    child_identity = read_process_identity(child_process_id)
    if child_identity is None or child_identity.state == "Z":
        return "ReadyChildNotAlive", None, parent_identity
    if (
        expected_child is not None
        and (
            child_identity.process_id != expected_child.process_id
            or child_identity.start_time_ticks != expected_child.start_time_ticks
        )
    ):
        return "ReadyChildIdentityMismatch", None, parent_identity
    if (
        child_identity.process_group_id != parent_process_id
        or child_identity.session_id != parent_process_id
        or not is_descendant_of(child_process_id, parent_process_id)
    ):
        return "ReadyChildIdentityMismatch", None, parent_identity
    return "", child_identity, parent_identity


def read_pid_record(path: str, record_name: str) -> tuple[str, int | None]:
    try:
        with Path(path).open("rb") as stream:
            value = stream.read(65)
    except OSError:
        return f"{record_name}ReadFailure", None
    if len(value) > 64:
        return f"{record_name}PidInvalid", None
    try:
        text = value.decode("ascii", errors="strict")
    except UnicodeError:
        return f"{record_name}PidInvalid", None
    match = re.fullmatch(
        rf"{record_name.upper()}:([1-9][0-9]*)",
        text,
        flags=re.ASCII,
    )
    if match is None:
        return f"{record_name}PidInvalid", None
    process_id = int(match.group(1))
    if process_id > 2147483647:
        return f"{record_name}PidInvalid", None
    return "", process_id


def control_record_bytes(
    record_name: str,
    process_id: int,
    variant: str,
) -> bytes:
    prefix = f"{record_name}:"
    valid = f"{prefix}{process_id}"
    values = {
        "overflow": f"{prefix}2147483648",
        "exact-64": prefix + ("9" * (64 - len(prefix))),
        "exact-65": prefix + ("9" * (65 - len(prefix))),
        "leading-space": f" {valid}",
        "trailing-space": f"{valid} ",
        "plus": f"{prefix}+{process_id}",
        "leading-zero": f"{prefix}0{process_id}",
        "space-after-colon": f"{prefix} {process_id}",
        "tab-after-colon": f"{prefix}\t{process_id}",
        "newline": f"{valid}\n",
        "extra-record": f"{valid}\n{valid}",
        "suffix": f"{valid}x",
        "empty": "",
        "nonnumeric": f"{prefix}not-a-pid",
        "zero": f"{prefix}0",
        "negative": f"{prefix}-1",
        "wrong-pid": f"{prefix}{2 if process_id == 1 else 1}",
    }
    if variant == "invalid-encoding":
        return prefix.encode("ascii") + b"\xff"
    return values.get(variant, valid).encode("ascii")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "mode",
        choices=(
            "valid-version",
            "valid-help",
            "empty",
            "partial-help",
            "nonzero",
            "hang",
            "stdout-flood",
            "stderr-flood",
            "spawn-child",
            "delayed-marker",
            "ready-barrier-runner",
        ),
    )
    parser.add_argument("--marker")
    parser.add_argument("--ready")
    parser.add_argument("--release")
    parser.add_argument("--released")
    parser.add_argument("--child-pid")
    parser.add_argument("--activation-probe")
    parser.add_argument("--activated")
    parser.add_argument("--final-check")
    parser.add_argument("--foreign-pid", type=int)
    parser.add_argument("--parent-pid", type=int)
    parser.add_argument(
        "--ready-mode",
        choices=(
            "valid",
            "missing",
            "late",
            "invalid",
            "leading-space",
            "trailing-space",
            "newline",
            "plus",
            "leading-zero",
            "space-after-colon",
            "tab-after-colon",
            "extra-record",
            "suffix",
            "empty",
            "zero",
            "negative",
            "oversize",
            "dead-pid",
            "foreign-pid",
            "zombie-pid",
            "exit-before-release",
            "exit-after-release",
            "exit-after-released",
            "exit-after-activated",
            "exit-after-final-check",
            "child-identity-mismatch-after-ready",
            "parent-identity-mismatch-after-ready",
            "parent-exit-before-release",
            "parent-exit-after-released",
            "parent-exit-before-timeout",
        ),
        default="valid",
    )
    parser.add_argument(
        "--record-target",
        choices=("ready", "released", "activated"),
        default="ready",
    )
    parser.add_argument(
        "--record-variant",
        choices=(
            "valid",
            "invalid-encoding",
            "overflow",
            "exact-64",
            "exact-65",
            "leading-space",
            "trailing-space",
            "plus",
            "leading-zero",
            "space-after-colon",
            "tab-after-colon",
            "newline",
            "extra-record",
            "suffix",
            "empty",
            "nonnumeric",
            "zero",
            "negative",
            "wrong-pid",
        ),
        default="valid",
    )
    parser.add_argument("--runner")
    parser.add_argument("--working-directory")
    parser.add_argument("--start-timeout", type=float, default=2.0)
    parser.add_argument("--execution-timeout", type=float, default=0.3)
    parser.add_argument("--maximum-bytes", type=int, default=4096)
    parser.add_argument("--stdout-file")
    parser.add_argument("--stderr-file")
    args = parser.parse_args()

    if args.mode == "valid-version":
        sys.stdout.write("flashgate-mcp 1.2.3-rc.1")
    elif args.mode == "valid-help":
        sys.stdout.write(
            "\n".join(
                (
                    "flashgate-mcp",
                    "Usage:",
                    "flashgate-mcp --version",
                    "flashgate-mcp --version --verbose",
                    "flashgate-mcp --help",
                    "MCP_ROOT",
                    "MCP_READ_ONLY",
                    "MCP_ALLOW_CWD_ROOT",
                )
            )
        )
    elif args.mode == "empty":
        pass
    elif args.mode == "partial-help":
        sys.stdout.write("flashgate-mcp\nUsage:")
    elif args.mode == "nonzero":
        sys.stderr.write("controlled failure")
        return 7
    elif args.mode == "hang":
        time.sleep(30)
    elif args.mode == "stdout-flood":
        sys.stdout.write("x" * 1048576)
    elif args.mode == "stderr-flood":
        sys.stderr.write("x" * 1048576)
    elif args.mode == "spawn-child":
        if any(
            not value
            for value in (
                args.marker,
                args.ready,
                args.release,
                args.released,
                args.child_pid,
                args.activation_probe,
                args.activated,
                args.final_check,
            )
        ):
            parser.error(
                "marker, ready, release, released, child-pid, and "
                "activation-probe and activated are required for spawn-child"
            )
        controlled_child = subprocess.Popen(
            [
                sys.executable,
                __file__,
                "delayed-marker",
                "--marker",
                args.marker,
                "--ready",
                args.ready,
                "--release",
                args.release,
                "--released",
                args.released,
                "--child-pid",
                args.child_pid,
                "--activation-probe",
                args.activation_probe,
                "--activated",
                args.activated,
                "--final-check",
                args.final_check,
                "--ready-mode",
                args.ready_mode,
                "--record-target",
                args.record_target,
                "--record-variant",
                args.record_variant,
                "--parent-pid",
                str(os.getpid()),
                "--foreign-pid",
                str(args.foreign_pid or 0),
            ]
        )
        write_atomic_record(
            args.child_pid,
            f"CHILD:{controlled_child.pid}".encode("ascii"),
        )
        parent_exit_path = {
            "parent-exit-before-release": args.ready,
            "parent-exit-after-released": args.activation_probe,
            "parent-exit-before-timeout": args.final_check,
        }.get(args.ready_mode)
        if parent_exit_path:
            parent_exit_deadline = time.monotonic() + 10
            while time.monotonic() < parent_exit_deadline:
                if Path(parent_exit_path).is_file():
                    return {
                        "parent-exit-before-release": 31,
                        "parent-exit-after-released": 32,
                        "parent-exit-before-timeout": 33,
                    }[args.ready_mode]
                time.sleep(0.005)
            return 34
        time.sleep(30)
    elif args.mode == "delayed-marker":
        if any(
            not value
            for value in (
                args.marker,
                args.ready,
                args.release,
                args.released,
                args.child_pid,
                args.activation_probe,
                args.activated,
                args.final_check,
            )
        ):
            parser.error(
                "marker, ready, release, released, child-pid, and "
                "activation-probe and activated are required for delayed-marker"
            )
        if args.ready_mode == "late":
            time.sleep(4)
        if args.ready_mode == "missing":
            time.sleep(30)
            return 0
        ready_pid = os.getpid()
        zombie_process: subprocess.Popen[bytes] | None = None
        if args.ready_mode == "dead-pid":
            dead_process = subprocess.Popen(
                [sys.executable, "-c", "pass"],
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
            )
            ready_pid = dead_process.pid
            dead_process.wait(timeout=2)
        elif args.ready_mode == "zombie-pid":
            zombie_process = subprocess.Popen(
                [sys.executable, "-c", "pass"],
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
            )
            ready_pid = zombie_process.pid
            zombie_deadline = time.monotonic() + 2
            while time.monotonic() < zombie_deadline:
                identity = read_process_identity(ready_pid)
                if identity is not None and identity.state == "Z":
                    break
                time.sleep(0.01)
            else:
                return 25
        elif args.ready_mode == "foreign-pid":
            if not args.foreign_pid or args.foreign_pid <= 0:
                parser.error(
                    "--foreign-pid must be positive for foreign-pid mode"
                )
            ready_pid = args.foreign_pid
        legacy_variants = {
            "invalid": "nonnumeric",
            "oversize": "exact-65",
        }
        ready_variant = "valid"
        if args.record_target == "ready":
            ready_variant = (
                args.record_variant
                if args.record_variant != "valid"
                else legacy_variants.get(args.ready_mode, args.ready_mode)
            )
            if ready_variant not in {
                "invalid-encoding",
                "overflow",
                "exact-64",
                "exact-65",
                "leading-space",
                "trailing-space",
                "plus",
                "leading-zero",
                "space-after-colon",
                "tab-after-colon",
                "newline",
                "extra-record",
                "suffix",
                "empty",
                "nonnumeric",
                "zero",
                "negative",
                "wrong-pid",
            }:
                ready_variant = "valid"
        ready_value = control_record_bytes("READY", ready_pid, ready_variant)
        write_atomic_record(args.ready, ready_value)
        if args.ready_mode == "exit-before-release":
            return 21
        release_deadline = time.monotonic() + 10
        while time.monotonic() < release_deadline:
            if args.parent_pid and not process_is_alive(args.parent_pid):
                return 40
            if Path(args.release).is_file():
                break
            time.sleep(0.025)
        else:
            return 8
        if args.ready_mode == "exit-after-release":
            return 22
        write_atomic_record(
            args.released,
            control_record_bytes(
                "RELEASED",
                os.getpid(),
                (
                    args.record_variant
                    if args.record_target == "released"
                    else "valid"
                ),
            ),
        )
        activation_deadline = time.monotonic() + 10
        while time.monotonic() < activation_deadline:
            if args.parent_pid and not process_is_alive(args.parent_pid):
                return 40
            if Path(args.activation_probe).is_file():
                if args.ready_mode == "exit-after-released":
                    return 23
                write_atomic_record(
                    args.activated,
                    control_record_bytes(
                        "ACTIVATED",
                        os.getpid(),
                        (
                            args.record_variant
                            if args.record_target == "activated"
                            else "valid"
                        ),
                    ),
                )
                if args.ready_mode == "exit-after-activated":
                    return 26
                break
            time.sleep(0.01)
        else:
            return 24
        if args.parent_pid and not process_is_alive(args.parent_pid):
            return 40
        if args.ready_mode == "exit-after-final-check":
            final_check_deadline = time.monotonic() + 10
            while time.monotonic() < final_check_deadline:
                if Path(args.final_check).is_file():
                    return 27
                time.sleep(0.005)
            return 28
        survivor_deadline = time.monotonic() + 2
        while time.monotonic() < survivor_deadline:
            if args.parent_pid and not process_is_alive(args.parent_pid):
                return 40
            time.sleep(0.01)
        Path(args.marker).write_text("child-survived", encoding="utf-8")
    elif args.mode == "ready-barrier-runner":
        required = (
            args.marker,
            args.ready,
            args.release,
            args.released,
            args.child_pid,
            args.activation_probe,
            args.activated,
            args.final_check,
            args.runner,
            args.working_directory,
            args.stdout_file,
            args.stderr_file,
        )
        if any(not value for value in required):
            parser.error(
                "the READY-barrier runner requires marker, ready, release, "
                "released, child-pid, activation-probe, runner, "
                "activated, final-check, "
                "working-directory, stdout-file, and stderr-file"
            )

        module_name = "_flashgate_bounded_process_runner_testonly"
        previous_dont_write_bytecode = sys.dont_write_bytecode
        try:
            sys.dont_write_bytecode = True
            spec = importlib.util.spec_from_file_location(
                module_name,
                args.runner,
            )
            if spec is None or spec.loader is None:
                raise RuntimeError("unable to load the bounded runner")
            runner_module = importlib.util.module_from_spec(spec)
            sys.modules[module_name] = runner_module
            spec.loader.exec_module(runner_module)
        finally:
            sys.dont_write_bytecode = previous_dont_write_bytecode

        state: dict[str, object] = {
            "ready_observed": False,
            "ready_pid": None,
            "released_observed": False,
            "released_pid": None,
            "activated_observed": False,
            "activated_pid": None,
            "child_alive": False,
            "timeout_started_after_ready": False,
            "ready_elapsed_ms": 0,
            "barrier_failure": "",
        }
        child_identity: ProcessIdentity | None = None
        parent_identity: ProcessIdentity | None = None

        def before_timed_wait(
            parent: subprocess.Popen[bytes],
        ) -> str:
            nonlocal child_identity, parent_identity
            started = time.monotonic()
            deadline = started + args.start_timeout
            ready_path = Path(args.ready)
            while time.monotonic() < deadline:
                if ready_path.is_file():
                    record_failure, child_pid = read_pid_record(
                        args.ready,
                        "Ready",
                    )
                    if record_failure or child_pid is None:
                        state["barrier_failure"] = record_failure
                        return record_failure
                    state["ready_observed"] = True
                    state["ready_pid"] = child_pid
                    state["ready_elapsed_ms"] = int(
                        (time.monotonic() - started) * 1000
                    )
                    (
                        identity_failure,
                        child_identity,
                        parent_identity,
                    ) = validate_controlled_child(
                        parent.pid,
                        child_pid,
                        None,
                        None,
                    )
                    if identity_failure:
                        state["barrier_failure"] = identity_failure
                        return identity_failure
                    if (
                        args.ready_mode
                        == "child-identity-mismatch-after-ready"
                        and child_identity is not None
                    ):
                        child_identity = ProcessIdentity(
                            process_id=child_identity.process_id,
                            state=child_identity.state,
                            parent_process_id=child_identity.parent_process_id,
                            process_group_id=child_identity.process_group_id,
                            session_id=child_identity.session_id,
                            start_time_ticks=child_identity.start_time_ticks + 1,
                        )
                    if (
                        args.ready_mode
                        == "parent-identity-mismatch-after-ready"
                        and parent_identity is not None
                    ):
                        parent_identity = ProcessIdentity(
                            process_id=parent_identity.process_id,
                            state=parent_identity.state,
                            parent_process_id=parent_identity.parent_process_id,
                            process_group_id=parent_identity.process_group_id,
                            session_id=parent_identity.session_id,
                            start_time_ticks=parent_identity.start_time_ticks + 1,
                        )

                    time.sleep(0.075)
                    (
                        identity_failure,
                        _,
                        _,
                    ) = validate_controlled_child(
                        parent.pid,
                        child_pid,
                        child_identity,
                        parent_identity,
                    )
                    if identity_failure:
                        state["barrier_failure"] = identity_failure
                        return identity_failure
                    try:
                        Path(args.release).write_text(
                            "RELEASE", encoding="utf-8"
                        )
                    except OSError:
                        state["barrier_failure"] = "ReleaseWriteFailure"
                        return "ReleaseWriteFailure"

                    released_deadline = time.monotonic() + args.start_timeout
                    while time.monotonic() < released_deadline:
                        (
                            identity_failure,
                            _,
                            _,
                        ) = validate_controlled_child(
                            parent.pid,
                            child_pid,
                            child_identity,
                            parent_identity,
                        )
                        if identity_failure:
                            failure = (
                                "ReadyChildNotAliveAfterRelease"
                                if identity_failure == "ReadyChildNotAlive"
                                else identity_failure
                            )
                            state["barrier_failure"] = failure
                            return failure
                        if Path(args.released).is_file():
                            released_failure, released_pid = read_pid_record(
                                args.released,
                                "Released",
                            )
                            if (
                                released_failure
                                or released_pid is None
                                or released_pid != child_pid
                            ):
                                failure = (
                                    released_failure
                                    or "ReleasedPidInvalid"
                                )
                                state["barrier_failure"] = failure
                                return failure
                            state["released_observed"] = True
                            state["released_pid"] = released_pid
                            return ""
                        time.sleep(0.01)
                    state["barrier_failure"] = "ReleasedTimeout"
                    return "ReleasedTimeout"
                time.sleep(0.01)
            state["ready_elapsed_ms"] = int(
                (time.monotonic() - started) * 1000
            )
            state["barrier_failure"] = "ReadyTimeout"
            return "ReadyTimeout"

        def at_timed_wait_activation(
            parent: subprocess.Popen[bytes],
            activate_timed_wait: Callable[[], None],
        ) -> str:
            try:
                Path(args.activation_probe).write_text(
                    "ACTIVATE",
                    encoding="ascii",
                )
            except OSError:
                state["barrier_failure"] = "ActivationProbeWriteFailure"
                return "ActivationProbeWriteFailure"
            if args.ready_mode == "parent-exit-after-released":
                parent_exit_deadline = (
                    time.monotonic() + args.start_timeout
                )
                while time.monotonic() < parent_exit_deadline:
                    if not process_is_alive(parent.pid):
                        break
                    time.sleep(0.005)
            if args.ready_mode == "exit-after-released":
                child_exit_deadline = (
                    time.monotonic() + args.start_timeout
                )
                ready_pid = int(state["ready_pid"] or 0)
                while time.monotonic() < child_exit_deadline:
                    if not process_is_alive(ready_pid):
                        break
                    time.sleep(0.005)
            activated_deadline = time.monotonic() + args.start_timeout
            while time.monotonic() < activated_deadline:
                identity_failure, _, _ = validate_controlled_child(
                    parent.pid,
                    int(state["ready_pid"] or 0),
                    child_identity,
                    parent_identity,
                )
                if identity_failure:
                    failure = (
                        "ReadyChildNotAliveBeforeTimeout"
                        if identity_failure == "ReadyChildNotAlive"
                        else identity_failure
                    )
                    state["barrier_failure"] = failure
                    return failure
                if Path(args.activated).is_file():
                    activated_failure, activated_pid = read_pid_record(
                        args.activated,
                        "Activated",
                    )
                    if (
                        activated_failure
                        or activated_pid is None
                        or activated_pid != state["ready_pid"]
                    ):
                        failure = (
                            activated_failure or "ActivatedPidInvalid"
                        )
                        state["barrier_failure"] = failure
                        return failure
                    identity_failure, _, _ = validate_controlled_child(
                        parent.pid,
                        activated_pid,
                        child_identity,
                        parent_identity,
                    )
                    if identity_failure:
                        failure = (
                            "ReadyChildNotAliveAfterActivated"
                            if identity_failure == "ReadyChildNotAlive"
                            else identity_failure
                        )
                        state["barrier_failure"] = failure
                        return failure
                    try:
                        Path(args.final_check).write_text(
                            "FINAL_CHECK",
                            encoding="ascii",
                        )
                    except OSError:
                        state["barrier_failure"] = (
                            "FinalCheckProbeWriteFailure"
                        )
                        return "FinalCheckProbeWriteFailure"
                    if args.ready_mode in {
                        "exit-after-activated",
                        "exit-after-final-check",
                    }:
                        exit_deadline = time.monotonic() + args.start_timeout
                        while time.monotonic() < exit_deadline:
                            if not process_is_alive(activated_pid):
                                break
                            time.sleep(0.005)
                    if args.ready_mode == "parent-exit-before-timeout":
                        parent_exit_deadline = (
                            time.monotonic() + args.start_timeout
                        )
                        while time.monotonic() < parent_exit_deadline:
                            if not process_is_alive(parent.pid):
                                break
                            time.sleep(0.005)
                    identity_failure, _, _ = validate_controlled_child(
                        parent.pid,
                        activated_pid,
                        child_identity,
                        parent_identity,
                    )
                    if identity_failure:
                        failure = (
                            "ReadyChildNotAliveBeforeTimeout"
                            if identity_failure == "ReadyChildNotAlive"
                            else identity_failure
                        )
                        state["barrier_failure"] = failure
                        return failure
                    state["activated_observed"] = True
                    state["activated_pid"] = activated_pid
                    state["child_alive"] = True
                    state["timeout_started_after_ready"] = True
                    activate_timed_wait()
                    return ""
                time.sleep(0.01)
            state["barrier_failure"] = "ActivatedTimeout"
            return "ActivatedTimeout"

        runner_args = argparse.Namespace(
            command=[
                sys.executable,
                __file__,
                "spawn-child",
                "--marker",
                args.marker,
                "--ready",
                args.ready,
                "--release",
                args.release,
                "--released",
                args.released,
                "--child-pid",
                args.child_pid,
                "--activation-probe",
                args.activation_probe,
                "--activated",
                args.activated,
                "--final-check",
                args.final_check,
                "--ready-mode",
                args.ready_mode,
                "--record-target",
                args.record_target,
                "--record-variant",
                args.record_variant,
                "--foreign-pid",
                str(os.getpid()),
            ],
            working_directory=args.working_directory,
            timeout_seconds=args.execution_timeout,
            maximum_bytes=args.maximum_bytes,
            stdout_file=args.stdout_file,
            stderr_file=args.stderr_file,
        )
        fields = runner_module._run_bounded_process(
            runner_args,
            before_timed_wait=before_timed_wait,
            at_timed_wait_activation=at_timed_wait_activation,
        )
        barrier_fields = (
            str(state["ready_observed"]).lower(),
            str(state["ready_pid"] or "none"),
            str(state["released_observed"]).lower(),
            str(state["released_pid"] or "none"),
            str(state["activated_observed"]).lower(),
            str(state["activated_pid"] or "none"),
            str(state["child_alive"]).lower(),
            str(state["timeout_started_after_ready"]).lower(),
            str(state["ready_elapsed_ms"]),
            str(state["barrier_failure"] or "None"),
        )
        sys.stdout.write("\t".join((*fields, *barrier_fields)) + "\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
