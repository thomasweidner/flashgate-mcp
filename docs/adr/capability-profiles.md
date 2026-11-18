# ADR-009: Capability Profiles and Tool Exposure

## Status

Accepted

## Context

As host functionality expands, exposing every tool in every deployment would increase attack surface, `tools/list` size, and operational ambiguity.

## Decision

The effective catalog uses functional rights, profile policy, and independently
verified technical support. BL-159 owns the authoritative execution decision.

BL-103 defines the default `read` profile with `files.read` and the explicit
`write` profile with `files.read` and `files.write`. Custom profile names
carry only declared grants. Profile definitions are fully validated even
when inactive. An explicit base may select additional named grant sets
without recursive inheritance. BL-108 owns per-root capability mapping.

Each reviewed capability definition carries a risk floor. An operation takes
the highest floor of its requirements and may declare a higher floor. The
configuration ceiling can restrict eligible operations. Both the complete
functional grant and the independent risk check must succeed. External
process control has a high-risk floor and retains its separate policy gates.
The default ceiling is `mutation`. The generic profile resolver accepts an
explicit `high` ceiling while host admission still requires a measured
executable view and the applicable operation policies.

STDIO startup admits only effective grant and risk combinations covered by
the executable wire and catalog gates. Newly defined rights and technically
available implementations require their own activation bindings.

Server-side checks remain authoritative. MCP tool annotations are not authorization.

The server supports multiple named roots addressed by explicit root ID and
relative path. BL-104 resolves [named policies](../configuration.md#named-policies)
and per-root read/write overrides from the typed configuration. The registry
enforces these rights independently of functional grants, risk and catalog
exposure. BL-105 limits and BL-106 file-type restrictions use the same immutable
root snapshots and grant no functional rights. Later owners extend the model
for capabilities, symlink/reparse and working-directory policy. Deprecated MCP
Roots may be evaluated as optional legacy-client compatibility.

### Functional capability contract

BL-100 supplies the evaluator in `internal/capability`. BL-103 sets the
following public IDs for the pre-1.0 cutover.
These definitions are available to Go consumers. Activation requires an
approved operation mapping, effective grants, and all applicable policy gates.

| ID | Functional scope |
|---|---|
| `files.read` | Inspect filesystem metadata and read filesystem content. |
| `files.write` | Create, modify, move or remove filesystem data. |
| `search.run` | Run a bounded search. |
| `processes.read` | Inspect permitted host process information. |
| `processes.manage` | Operate on server-managed processes. |
| `processes.control` | Control processes outside the server-managed lifecycle, subject to separate high-risk policy. |
| `commands.run` | Run a server-approved typed command. |
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
- Profile admission covers configurations with one or more named roots.

## Security Impact

Capabilities, profiles, and roots form a server-enforced gate. Hidden tools and annotations alone never provide enforcement.

## Implementation Guidance

BL-110 uses the shared functional contract to derive effective catalog
visibility. BL-159 evaluates the same operation requirements during execution.
Preserve generic unresolved-tool errors and keep internal validation detail
out of caller-facing responses.

## Deferred Decisions

- whether a supported legacy client justifies optional deprecated MCP Roots compatibility

## Current STDIO profiles

The default `read` profile exposes `list_directory`, `read_file`, and
`get_path_info`. The five mutation names return generic Invalid params in
that profile. See the [configuration reference](../configuration.md#profiles)
and [client setup](../client-setup.md).
