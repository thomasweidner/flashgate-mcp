# Native OS adapter and no-interpreter policy

This document is the normative Version 1.0 contract for choosing how the
FlashGate product runtime reaches operating-system functionality. The
machine-readable companion is
[`native-os-adapter-policy.json`](native-os-adapter-policy.json).

## Product-runtime boundary

The product runtime comprises non-test Go sources below `cmd/server` and
`internal`, except packages explicitly classified as development-only in the
machine-readable policy. Build, test, benchmark, release, installation, smoke,
and administrator scripts are outside this boundary; they must not become a
hidden prerequisite of an MCP tool, the server core, proxy, or service host.

## Selection order

Implement an OS operation using the first sufficient option:

1. the Go standard library;
2. a small build-tagged Windows or Linux Go adapter behind a narrow interface;
3. a direct OS API or documented stable interface such as `/proc` or `/sys`;
4. a native OS program started directly with a fixed executable identity and a
   constructed argument vector.

Convenience alone does not justify advancing to a later option. A direct OS API
must document supported platform/version behavior, privilege and identity
effects, cancellation, bounded resources, error normalization, and tests. A
new library, native API binding, or program remains subject to the repository's
dependency, architecture, security, and release decision boundaries.

## External native-program admission gate

An external program is excluded until its owning backlog task supplies all of
the following in one reviewable change:

- measured need and a benchmark against the preceding native options;
- an exact executable identity and resolution rule that cannot fall back to
  `PATH`, file associations, or a shell;
- a closed typed argument contract, with response files, hooks, plugins,
  loaders, configuration injection, and shell metacharacter interpretation
  disabled;
- fixed or allowlisted environment, root-confined working directory and path
  arguments, timeout, cancellation and process-tree cleanup;
- bounded stdin, stdout, stderr, concurrency, CPU/memory and retained results;
- capability, caller, root, profile and execution-backend authorization before
  launch, plus safe error mapping, redaction and audit behavior;
- Windows and Linux behavior, packaging/SBOM/provenance impact, negative
  injection tests, and the required native-host finalization evidence.

After approval, the source file and executable definition are added to
`externalNativePrograms` in the machine-readable policy. An empty list means
that no product-runtime source may import `os/exec`.

## No-interpreter rule

The Version 1.0 product runtime must not invoke or embed Bash, `cmd.exe`, Java,
Node.js, PHP, PowerShell, Python, `sh`, or another command/script interpreter.
Renaming an interpreter, wrapping it in a native launcher, or invoking it
indirectly does not change this classification. Arbitrary script, expression,
template, workflow, or shell strings are also excluded.

Platform administration assets may use `pwsh` or the target system shell only
when they remain explicit operator-invoked installation, removal, validation,
or troubleshooting tools. They are not fallback implementations for a failed
or unavailable product-runtime operation.

## Permanent gate

`internal/nativeadapter/policy_test.go` strictly decodes the policy, verifies
its closed Version 1 contract, and scans the declared product-runtime roots.
It rejects undeclared development exclusions and every non-test `os/exec`
import that is not bound to an admitted external-native-program entry. This is
a source gate; native Windows/Linux validation must still prove actual binary,
identity, process-tree, permission, and packaging behavior.
