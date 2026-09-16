# ADR-011: Managed Process and Command Execution

## Status

Accepted

## Context

Process observation and program execution are useful host operations but expose command lines, environments, identities, and powerful control surfaces.

## Decision

Use a Managed Process Registry. Every server-started process receives an opaque process handle used for status, output, waiting, and stopping. PIDs may be diagnostic fields but are not the sole security identity; PID reuse must not cause false association.

Stopping defaults to server-managed processes. External PID control requires a distinct functional capability such as `process.control.external` plus a high-risk policy classification and is excluded from standard profiles. Process data, command lines, and environments are filtered and redacted.

Command execution uses configured executable IDs resolved server-side to allowed absolute program paths. Arguments are separate arrays. Working directories must be within allowed roots. Environment propagation is allowlisted or explicitly defined. stdout and stderr are separate and bounded. Runtime, output, and concurrency are limited.

CPU and memory controls apply to the complete managed process tree. Windows
uses a private Job Object with hard CPU-rate and aggregate job-memory limits;
Linux uses a delegated cgroup v2 leaf with CPU bandwidth and hard memory
limits. A process requiring either control is admitted only when the adapter
can configure and establish containment before untrusted execution. There is no
best-effort fallback to polling, priority, runtime, or per-process resource
limits. A policy may explicitly omit CPU or memory control while retaining all
other mandatory bounds. The detailed platform and validation contract is the
[managed process CPU and memory limit strategy](../process-resource-limit-strategy.md).

A future `run_command` is a synchronous wrapper over the Managed Process Engine. No second execution engine is permitted. Free shell strings and interactive shells are disabled by default.

## Rationale

Opaque handles provide stronger lifecycle identity than reusable PIDs. A single execution engine prevents divergent policy and cleanup behavior.

## Consequences

- Process observation and control use separate capabilities.
- Windows and Linux adapters must implement equivalent policy outcomes.
- Output needs bounded ring buffers or equivalent streaming storage.
- Server restart behavior must be defined.

## Security Impact

Executable allowlisting, argument separation, root-confined working directories, environment allowlisting, output limits, redaction, and least privilege are mandatory boundaries.

## Implementation Guidance

Threat-model observation and execution separately. Implement registry identity and cleanup before control tools. Test PID reuse assumptions, races, timeouts, redaction, and platform isolation.

## Decision Gates

- external PID control remains separately capable, high-risk classified, and opt-in
- interactive input requires a later risk decision
- interactive shell support requires separate interactive/high-risk policy decisions
- CPU/RAM isolation uses Windows Job Objects and Linux cgroup v2 as specified
  by the managed-process resource-limit strategy; native implementation and
  platform proof remain `BL-134/BL-135`
