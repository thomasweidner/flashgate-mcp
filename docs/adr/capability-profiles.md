# ADR-009: Capability Profiles and Tool Exposure

## Status

Accepted

## Context

As host functionality expands, exposing every tool in every deployment would increase attack surface, `tools/list` size, and operational ambiguity.

## Decision

The effective catalog uses functional rights, profile policy, and independently
verified technical support. BL-159 owns the authoritative execution decision.

Possible future profiles include `safe-read`, `filesystem-write`, and `admin`.
BL-103 owns profile names, risk classifications such as `high-risk`,
`destructive`, and `interactive`, and the conditions for activation. BL-108
owns per-root capability mapping. The existing `files.read_only` TOML
setting remains the current restricted-profile control.

Server-side checks remain authoritative. MCP tool annotations are not authorization.

The target supports multiple named roots addressed by explicit root ID and relative path, based on authoritative FlashGate server configuration. Each root may have read/write, size, result, file-type, capability, symlink/reparse, and process-working-directory rules. Deprecated MCP Roots is not an architectural dependency and may be evaluated only as optional legacy-client compatibility.

### Functional capability contract

BL-100 defines the following stable basis in `internal/capability`.
These definitions are available to Go consumers. Activation requires an
approved operation mapping, effective grants, and all applicable policy gates.

| ID | Functional scope |
|---|---|
| `filesystem.read` | Inspect filesystem metadata and read filesystem content. |
| `filesystem.write` | Create, modify, move or remove filesystem data. |
| `search.execute` | Execute a bounded search. |
| `process.observe` | Inspect permitted host process information. |
| `process.manage` | Operate on server-managed processes. |
| `process.control.external` | Control processes outside the server-managed lifecycle, subject to separate high-risk policy. |
| `command.execute` | Execute a server-approved typed command. |
| `system.read` | Read explicitly permitted system information. |

An ID has at least two dot-separated segments. Each segment starts with an
ASCII lowercase letter and continues with ASCII lowercase letters or digits.
The full ID is at most 128 bytes. The first segment identifies a functional
namespace. Matching is exact and case-sensitive. Reject malformed input as-is.
There are no aliases, wildcards, implicit prefixes, or inherited rights. An ID
retains its reviewed meaning across catalog snapshots. New meaning requires a
new ID.

Each definition has one ID and a nonempty description of at most 256 UTF-8
bytes, with no leading or trailing whitespace or control characters.
Descriptions are review metadata. A validated immutable catalog contains one
definition per ID and at least one definition. Equal duplicate definitions and
conflicting descriptions for the same ID are distinct errors. For a given
input, validation groups by ID in lexical order. Within a group, an invalid
definition takes precedence over a conflict, and a conflict over a duplicate.
Any validation error rejects the entire input.

An explicit grant set contains zero or more unique IDs known to one catalog
snapshot. An operation or operation variant declares one or more unique IDs
from that snapshot as its requirement. Malformed, unknown, and duplicate IDs
are construction errors. IDs in sets are exposed in ascending lexical order
via copies. Union, intersection, equality and exact membership operate on
explicit IDs. Binary set operations require the same snapshot. Rebuilding a
catalog requires rebuilding its sets and requirements from validated inputs.
No partially validated value can satisfy a requirement.

Functional evaluation succeeds exactly when a nonempty validated requirement
and validated grants share the same catalog snapshot and every required ID
is explicitly granted. A zero-value requirement or grant denies. An empty
grant satisfies no operation requirement. For example, if an operation reads
one resource and writes another, its declaration can require both read and
write, with BL-159 applying each operand's applicable root policy.

Profile and risk policy, root and resource scopes, authenticated principal,
effective execution identity, backend availability and enforcement strength
remain independent decision inputs. Protocol capabilities, extension
negotiation, tool and format support, quotas, audit, state and lifecycle
cannot supply a grant. A functional success is one input to BL-159.

Future owners declare reviewed definitions and operation requirements under
their own backlog acceptance. The same evaluator handles every valid ID.
Definition, explicit grant and activation are separate steps. The reviewed
catalog for an effective configuration rejects ID collisions atomically,
including collisions contributed by providers. Provider identifiers,
provenance, installation, and format support convey no grants. The provider
contract and trust model remain with BL-184 and BL-188.

## Rationale

Least-privilege tool exposure reduces risk and schema/token overhead while giving operators understandable configurations.

## Consequences

- `tools/list` varies by effective profile and capabilities.
- Negative capability tests are required.
- Direct tool calls must still be authorized server-side.
- Profile migration must cover configurations with one or more named roots.

## Security Impact

Capabilities, profiles, and roots form a server-enforced gate. Hidden tools and annotations alone never provide enforcement.

## Implementation Guidance

BL-110 uses the shared functional contract to derive effective catalog
visibility. BL-159 evaluates the same operation requirements during execution.
Preserve generic unresolved-tool errors and keep internal validation detail
out of caller-facing responses.

## Deferred Decisions

- final profile names and profile field definitions in the [central configuration model](runtime-modes.md#configuration-contract)
- whether a supported legacy client justifies optional deprecated MCP Roots compatibility

## Current restricted profile

Set `files.read_only = true` explicitly to expose only `list_directory`,
`read_file`, and `get_path_info`. The five mutation names and five removed
names return the same generic Invalid params contract in STDIO smoke tests.
This is a restricted baseline, not a general profile framework or an implicit
read-only default. See [client setup](../client-setup.md).
