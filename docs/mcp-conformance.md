# MCP conformance validation

Native Go tests cover production revision `2025-11-25` and the compiled
`2026-07-28` candidate under an injected test policy, each with default and
read-only profiles. The candidate remains disabled in production. The
[protocol matrix](protocol.md) and [ADR-013](adr/mcp-compatibility.md) retain
their existing activation gates.

## Run and report

From the repository root:

```bash
go test -mod=vendor ./cmd/server ./cmd/mcpcheck -run 'Test(MCPConformance|Report|Stateless|ProtocolRevision|Filesystem.*Wire|ToolsListWire)' -count=1
go run -mod=vendor ./cmd/mcpcheck -report /absolute/output-directory/mcp-conformance.md
```

For the report, supply an existing caller-controlled directory outside the
source tree; Windows callers use an absolute Windows path. Both the Markdown
output and adjacent `.tests.jsonl` log paths must be new. The tool runs focused
server tests and requires passing evidence for every mandatory family and
every snapshot revision/profile row. It rejects failures, skips, missing
evidence, and source drift. A failed run can leave its actual JSON log.
Source must remain quiescent during validation.

The Windows and Linux CI test jobs run this command as a blocking step for
pushes and pull requests to `main`. Each runner writes the report and log
under `RUNNER_TEMP` and retains available evidence for 14 days. CI compares
snapshot bytes and never supplies the explicit update flag.

The report binds HEAD, a deterministic hash of tracked and nonignored source
bytes, snapshot hashes, individual tests and official requirement mappings.
It does not replace repository-wide gates, native Linux validation or
independent review. BL-369 may consume this focused evidence later; this is
not a general release-report generator.

## Snapshots and completeness

Four snapshots under `cmd/server/testdata/mcp` bind actual startup-registry
definitions, including name, title, description, complete input/output schemas
and every explicit boolean annotation. They also bind actual `tools/list`
wire results, initialization/discovery capabilities, revision metadata, and
successful calls for every registered result variant and a denied call for
each exposed tool in each profile.

The wire negatives also exercise `read_file` encoding in every
revision/profile combination. Valid and empty UTF-8 round-trip byte for byte
and malformed UTF-8 returns the safe `invalid_text_encoding` result.

Definitions are sorted by name; wire lists preserve their actual stable
registration order. Object keys are canonical, UTF-8 and indented, with LF
and a final newline. Fixtures use fixed content, relative names and fixed
server identity; no host paths, timestamps or secrets are recorded. Ordinary
tests only compare bytes. Deliberate updates require an explicit flag:

```bash
go test -mod=vendor ./cmd/server -run '^TestMCPConformanceSnapshots$' -count=1 -args -update-mcp-snapshots
git diff -- cmd/server/testdata/mcp
```

Review the complete diff, including new untracked snapshots, then rerun normal
tests without the flag. Tool owners add executable cases for every declared
result variant and a denial case to `mcpToolCases`, assert result placement
and new or changed parameter semantics, and
update the catalog and affected snapshots. Capability, profile, and revision
owners extend the existing discovery, revision/profile matrix, and wire
contracts, including a snapshot for every supported combination. The CI entry
point stays the same. Runtime and test declarations establish what the gate
can verify. Feature specifications and independent review establish whether
the declarations and assertions cover the intended behavior.

An intentional snapshot update requires independent review of the diff.
A technical PASS records executed coverage and accepted byte parity.
`docs/mcp-tool-catalog.json` remains the product catalog; snapshots are test
baselines of runtime/wire behavior, not another product catalog.

Mutation tests register an additional tool and reject missing coverage,
missing result variants, and snapshot drift. They also reject changed input/output schemas or descriptive
metadata, missing explicit-false annotations, missing tools, changed profile
membership, activated/announced revisions and unknown capabilities/members.
New capabilities require explicit tests or justified non-applicability.

## JSON Schema Draft 2020-12

The snapshot run compiles every registered tool's input and output schema in
each matrix entry, validates the functional success and security inputs and wire
`structuredContent`, and checks complete runtime/catalog parity in the default
profile. Input roots must be closed objects. Output roots describe success
objects. Error responses follow the separate tool and JSON-RPC error tests.
Future effective profiles extend the shared matrix under BL-110.

The vendored test compiler pins Draft 2020-12 and rejects other dialect
declarations. Published schemas omit `$schema`. Reference resolution uses
the registered schema and embedded standard metaschemas. Missing external
resources fail without host file or network access. Put local reference targets
at standard schema positions such as `$defs`. The default dialect
treats `format` as annotation. Patterns use Go's regular expression engine
and its supported syntax. Schema validation does not model filesystem guards,
configured limits, Go integer decoding, or authorization.

`TestMCPConformanceSchema202012` exercises invalid metaschemas, dialect and
reference failures, Draft 2020-12 keywords, strict arguments and output
instance failures, and catalog mutations. `mcpcheck` requires this family and
the snapshot and completeness families in its test evidence.

## Wire coverage

The same gate checks [catalog and entry budgets](planning/efficiency.md#tool-catalog-and-initialization-budgets)
on captured executable responses. It reports catalog text and schema costs
and checks instruction presence and size separately from each complete frame.

The [wire efficiency gate](../benchmarks/README.md#deterministic-wire-efficiency)
reuses the registry, revision/profile matrix, executable cases and reviewed
snapshots. It captures complete frames with framing intact. Add the matching
semantic references and wire budget binding when a result or path changes.

The real parser, STDIO transport, router and serializers process requests.
Child-process tests also run real startup/environment loading under both
profiles, enforce protocol-only stdout, verify write authorization and reject
production candidate requests. No candidate CLI/environment switch is added.

Native contracts cover negotiation, notifications, unknown methods, malformed
envelopes, strict arguments, result/error shape, traversal and read-only denial.
BL-371 adds native `2025-11-25` ping tests for exact empty results, request IDs,
optional metadata, invalid parameters, message limits, both tool profiles and
tool-registry independence. The injected `2026-07-28` policy rejects the method
even when the handler is registered.
Candidate tests cover `server/discover`, calls before discovery, per-request
metadata, `UnsupportedProtocolVersion`, `serverInfo`, `resultType`, conservative
`ttlMs:0`/`cacheScope:"private"` hints and cross-revision behavior. Client
metadata and extensions grant no authorization.

## Official evaluation and limits

The [official suite](https://github.com/modelcontextprotocol/conformance/tree/c37eec888e1c6ff140af79987a40008548b7cc5f)
is pinned to `c37eec888e1c6ff140af79987a40008548b7cc5f`
(`0.2.0-alpha.12`). Its [2025-11-25 requirements](https://github.com/modelcontextprotocol/conformance/blob/c37eec888e1c6ff140af79987a40008548b7cc5f/requirements/2025-11-25.yaml)
and [2026-07-28 requirements](https://github.com/modelcontextprotocol/conformance/blob/c37eec888e1c6ff140af79987a40008548b7cc5f/requirements/2026-07-28.yaml)
use anchor `0.2.0-alpha.10`. The older set was reconstructed at that anchor;
the newer set was frozen at release. Each revision needs a separate run.

`official-requirements.json` maps all 83 server entries, preserving scored and
not-scored membership and upstream reasons. Client-leg scenarios are not
applicable to this server. Applicability, official execution and native subset
evidence remain distinct. Every official scenario is `NOT_RUN`: the pinned
runner connects to HTTP and expects named fixture behavior. No installation,
runtime dependency or HTTP bridge is used. Native PASS is never an official
suite PASS. Full MCP conformance is not claimed.

Absent resources, prompts, completion, Tasks, media, MRTR and HTTP/SSE behavior
is explicitly classified. The `2025-11-25` ping mapping points to BL-371's native
STDIO test. Its official HTTP scenario retains `NOT_RUN`. Fingerprints/cache semantics remain BL-219,
general release reporting BL-369 and HTTP transport BL-370. Snapshot success
does not activate the candidate or close those owners.
