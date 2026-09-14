# MCP profile catalog and initialization budgets

This document defines the Version 1.0 size ceilings owned by `BL-215`. The
machine-readable source is
[`mcp-profile-budgets.json`](mcp-profile-budgets.json), and the server contract
tests enforce it against the catalog and initialization responses produced by
the current checkout.

## Scope and accounting

The budgets are deterministic UTF-8 byte ceilings, not latency or memory
budgets. `tools/list` response bytes include the complete JSON-RPC envelope and
the STDIO newline. Schema bytes are the sum of the compact JSON encodings of
every input and output schema in the profile. Description bytes are counted as
UTF-8 bytes, both individually and in aggregate. Initialization-instruction
bytes count the UTF-8 `instructions` value only; an omitted value counts as
zero. Approximate tokens remain the documented orientation
`ceil(UTF-8 bytes / 4)` and are therefore derivable rather than an independent
acceptance limit.

The limits apply independently to the current `read_only` and `default`
profiles. A future named capability profile must add an explicit keyed budget
before it can become a Version 1.0 release profile. The limits do not authorize
tools, make catalog metadata an authorization input, or permit a caller to
select capabilities that server policy has not granted.

## Version 1.0 ceilings

| Profile | Tools | `tools/list` response | Schema bytes | Description/tool | Description total | Instructions |
|---|---:|---:|---:|---:|---:|---:|
| `read_only` | 16 | 32 KiB | 24 KiB | 1 KiB | 8 KiB | 1 KiB |
| `default` | 64 | 128 KiB | 96 KiB | 1 KiB | 32 KiB | 1 KiB |

These ceilings deliberately leave bounded expansion room above the current
eight-tool/three-tool implementation while preventing the Version 1.0 catalog
from growing without review. The read-only profile remains substantially
smaller than the default profile. The 1 KiB instruction ceiling accommodates
compact profile guidance without turning initialization into general
documentation. Each ceiling is a maximum: shrinking a response does not
require lowering the contract, while raising a ceiling requires review of this
file, the machine-readable artifact, and the permanent contract test.

## Release and change gate

Every registered tool still needs its own accepted backlog scope, schema,
security review, and profile authorization. If a catalog change exceeds a
ceiling, the change must reduce its wire footprint, split optional tools into a
separately budgeted profile, or explicitly revise this Version 1.0 contract.
Changing these numbers solely to make a failing test pass is prohibited.

The test gate validates the live server-generated responses rather than the
static catalog alone. Windows finalization must rerun the same deterministic
gate with PowerShell 7.6.5 and the repository's native validation profile; no
platform-specific byte variance is expected from these JSON responses.
