# Managed Process CPU and Memory Limit Strategy

## Status and scope

This document began as the `BL-132` design contract for CPU and memory limits
on FlashGate-managed process trees. `BL-134` now provides platform adapters for
the selected mechanisms. Native Windows and delegated-cgroup validation,
lifecycle/race/restart coverage, and canonical completion remain required by
`BL-135` and Windows finalization.

The limits are policy output, never tool input. They complement, rather than
replace, the existing process-count, runtime, and output bounds. External PID
control and general host resource management remain outside this contract.

## Required policy outcome

A policy may require either or both of these limits for a managed launch:

- a positive CPU bandwidth ceiling, expressed by the platform-neutral policy
  as a rate and enforced over a bounded scheduler period; and
- a positive memory ceiling in bytes for the complete managed process tree.

The platform-neutral `CPURate` is expressed in basis points of host CPU
capacity (`1..10000`). Linux uses a one-second cgroup period and rejects rates
below the kernel's representable one-millisecond quota rather than rounding to
a weaker limit.

Zero means that the particular CPU or memory control is not requested by that
policy. Negative, overflowed, unrepresentable, or platform-invalid values are
configuration errors and must fail before process creation. Tool requests
cannot disable, raise, or select these limits.

The protected unit is the entire process tree, not only the initial PID. A
successful start means that untrusted child code cannot run before membership
in the platform resource container is established. Adapter initialization,
limit configuration, process creation, container assignment, or verification
failure denies the launch and cleans up the not-yet-running process and its
container.

CPU bandwidth is distinct from wall-clock runtime: exhausting a runtime limit
terminates the process, while reaching a CPU-rate ceiling throttles it. The
memory ceiling is a hard admission/use boundary. If the platform reports a
memory-limit termination, FlashGate retains the existing terminal `failed`
outcome and a bounded internal resource-limit reason; a new public status or
wire error is not introduced by this design task.

## Platform mechanisms

| Platform | CPU mechanism | Memory mechanism | Containment and setup |
|---|---|---|---|
| Windows | A private Job Object configured with `JOBOBJECT_CPU_RATE_CONTROL_INFORMATION`, `ENABLE`, and `HARD_CAP`; convert the policy rate to the supported 1–10,000 scale and reject values that cannot be represented. | The same Job Object uses `JOB_OBJECT_LIMIT_JOB_MEMORY` for the aggregate committed-memory ceiling. `JOB_OBJECT_LIMIT_PROCESS_MEMORY` may additionally enforce a policy-defined per-process ceiling later, but it is not a substitute for the tree aggregate. | Create the child suspended, assign it to the fully configured Job Object, verify assignment, then resume it. Disable breakaway and use kill-on-job-close for bounded cleanup. Reject a host/nested-job situation that cannot preserve all required controls. |
| Linux | A private cgroup v2 leaf uses `cpu.max` with an explicit bounded period and quota derived from the policy rate. | The same cgroup uses `memory.max`; set `memory.oom.group=1` so an out-of-memory event is handled as a workload-tree failure rather than leaving an arbitrary subset alive. | Create and configure the delegated cgroup before launch, place the child in it before untrusted execution, and keep descendants in that subtree. The adapter must not depend on invoking `systemd-run` or another shell command at runtime. |

Linux implementation may use a direct cgroup v2 API/filesystem adapter or a
service-manager API that proves equivalent delegation and pre-execution
membership. Merely writing a running PID to `cgroup.procs` after normal launch
does not meet the no-uncontained-execution requirement. Windows implementation
must similarly avoid ordinary start followed by late Job assignment.

Windows Job committed-memory accounting and Linux cgroup memory accounting are
not byte-for-byte identical. The portable contract is the hard, tree-scoped
ceiling and fail-closed admission, not identical kernel counters. Diagnostics
may report a bounded platform mechanism and outcome but must not expose command
lines, environments, host paths, or raw platform errors.

## Capability detection and fallbacks

Capability detection occurs before admitting a launch that requires a limit.
It must distinguish at least CPU control, memory control, process-tree
containment, and pre-execution assignment. Detection is cached only for a
stable adapter/service generation and is revalidated after a generation or
execution-backend change.

There is no silent best-effort degradation:

1. If policy requires a control and the selected backend cannot prove it, deny
   the launch before untrusted code executes.
2. Do not substitute wall-clock timeout, process counts, priority/nice values,
   monitoring-and-kill loops, Linux `RLIMIT_CPU`, or address-space
   `RLIMIT_AS`; none provides the selected CPU-bandwidth and tree-memory
   contract.
3. A policy that explicitly requests neither CPU nor memory enforcement may
   run on an otherwise supported backend, still subject to concurrency,
   runtime, output, authorization, and lifecycle bounds.
4. Configuration may choose a stricter representable value through an
   explicit validation/normalization rule, but it must never round to a weaker
   limit. If no safe representation exists, validation fails.

## Adapter implementation and validation obligations

The engine accepts one trusted `ProcessAdapter`; resource limits are policy
output carried in `Launch` and are never copied from `StartRequest`. The Linux
adapter requires an explicitly configured absolute delegated cgroup v2 root,
creates a private random leaf, configures `cpu.max`, `memory.max`, and
`memory.oom.group`, and uses Go's `UseCgroupFD` clone path for pre-execution
membership. The Windows adapter creates and configures a private Job Object,
starts the process suspended, assigns and verifies Job membership, and only
then resumes it. Both adapters deny a requested control on setup failure and
bind tree termination to container cleanup.

`BL-134` owns the platform-specific implementation behind one
platform-neutral managed-process adapter. `BL-135` owns lifecycle, race, and
restart coverage. Those tasks must demonstrate:

- invalid and unsupported requirements fail before launch;
- the initial process and descendants share the resource container;
- CPU saturation is throttled without being confused with runtime expiry;
- aggregate memory pressure cannot escape the configured tree ceiling;
- explicit stop, runtime timeout, normal exit, startup failure, server
  shutdown, and resource-limit failure each clean up the container exactly
  once;
- concurrent exit and limit notifications preserve the first terminal outcome;
- a service/backend generation change cannot reuse stale capability evidence;
- supported Windows and native Linux tests exercise the real kernel mechanisms
  and record any environment-specific skip as missing evidence, never success.

Cross-compilation and test doubles can validate build boundaries and failure
mapping, but they cannot establish native resource-enforcement evidence.

## Primary platform references

- Microsoft, [Job Objects](https://learn.microsoft.com/windows/win32/procthread/job-objects)
- Microsoft, [Job Object Limits](https://learn.microsoft.com/windows/win32/procthread/job-objects#job-object-limits)
- Linux kernel, [Control Group v2](https://docs.kernel.org/admin-guide/cgroup-v2.html)
- systemd, [Resource Control](https://www.freedesktop.org/software/systemd/man/latest/systemd.resource-control.html)
