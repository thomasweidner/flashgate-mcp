# Structured audit lifecycle

Each independently configured server owns one persistent audit store and a
single managed-file byte budget. A server instance is one running process.
A client instance communicates through a transport session, which may carry
many requests. Multiple direct STDIO instances for the same configuration
coordinate one store. Restart creates a new process namespace for event IDs
while preserving storage and the managed-file budget.

FlashGate uses the trusted role and canonical configuration path as the store
binding. The standard configuration name uses an `audit` directory beside the
file. Other names in the same directory use a distinct `audit-<binding>`
directory. A configured absolute override must identify a local store with an
existing parent. A store persists its server and storage policy binding.
Another configuration cannot open it. Conflicting limits for the same
configuration take effect only after previous policy holders exit and the new
policy is admitted under native coordination. A content edit or restart does
not change the configuration identity.

## Event and correlation

Audit observes declared operations at the core decision, native effect
admission and outcome boundaries. Each event has an immutable event ID formed
from a random process namespace and a monotonic number. The request reference
uses the existing trusted generation and request sequence. A parent reference
names a specific event across namespaces. A later independently authorized
action may carry the original request reference. IDs and retained references
give no permission.

One versioned UTF-8 JSON object occupies each line. Events have bounded
operation, version, UTC time, phase, reason and correlation fields, with optional
typed declaration, ordered operands, identity references and counters. An
allowed decision records authorized logical IDs. Only a positively bound native
intent or outcome can include an eligible normalized relative path. Caller and
effective execution references have distinct fields. Decisions made before
effective execution omit the effective identity. A denied catalog request
carries an opaque denial.

Sensitive fields are excluded before encoding and queue admission. Native
paths, raw requests or errors, credentials, arguments and file contents stay
out of events. Optional paths are omitted if their safe form cannot be
established. Counts distinguish requested bytes from native bytes read and
written. An observed zero is recorded explicitly. Unobserved counts are omitted,
including when a failure occurs before the corresponding I/O.
Control characters, ambiguous relative paths and unsafe values have no record
boundary or field-name authority. Mandatory projection failures enter the
configured audit loss and availability policy.

## Availability and lifecycle

Audit defaults to `operation_first`. A bounded asynchronous recorder owns one
writer per process. A slow, full or unavailable store produces explicit
degradation and bounded loss reporting. Ordinary operations remain governed by
their existing authorization and native safety checks.

When configured as `audit_required`, selected declarations require durable
intent before the protected native effect. The native adapter completes its
proof first and revalidates after the bounded wait. The file sink confirms
complete bytes and synchronizes the file before acknowledging. Linux also
synchronizes newly created directory entries. Windows confirms file data
through the platform flush contract. Filesystem, controller and power-loss
guarantees remain those of the native platform. A failed or late confirmation
prevents the selected effect. The actual outcome is captured separately.

The default queue caps both event count and encoded bytes per process.
The regular 100 ms batch interval limits the scheduled wait under light
load. Required intent wakes the writer immediately. Its configurable
two-second wait uses the same starting allowance as the writer I/O
budget. The two deadlines are independent, and queue or store failure
can refuse the effect earlier. The 256 MiB store cap can shorten the
nominal 30-day retention under sustained event volume. These defaults
bound resource use and waiting. BL-244 owns quantitative comparison and
tuning across native platforms and runtime modes.

Critical P0 records outrank High P1, Normal P2 and Low P3. When either queue
limit is reached, the oldest eligible event in the lowest priority class loses,
including the incoming event if it ranks below all eligible records. Ordinary
P0 events compete by age when only P0 remains. Durable intents and in-flight
records are protected from eviction. A bounded counters snapshot reports
observable losses after recovery. The shutdown deadline bounds the host wait.
An unclean shutdown can leave an observable intent without a known outcome.
Recovery never invents a terminal result.

## Storage and access

The sink appends bounded batches to UTC day partitions such as
`audit/2026/11/25/audit-000001.jsonl`. Each record uses its observation day,
including when it waits across midnight in the queue. Late records can open a
new segment in an earlier partition. A segment is closed at the configured
size, day boundary or an unrecoverable tail. A partial terminal record can be
truncated to the last validated boundary under the same native lock. Other
unexpected file states degrade the store.

Native file locks serialize append, rotation, retention and budget admission
across direct instances. A second lease keeps the store policy stable while
processes use it. The store refreshes its inventory on a structural change or
recovery and checks the current segment identity and size before each append.
The byte budget includes segment headers and coordination files. Every segment
written by a durable batch stays protected until confirmation under the writer
lock. If the full batch cannot fit, required intents receive a failure.
Filesystem metadata and block allocation are outside the managed-file byte count.

Retention checks age on startup, on writes and at one-minute maintenance
intervals subject to writer scheduling and native lock availability. It enforces
the byte budget before writing. The start of the
observation day is the conservative age bound for a partition. Records in the
same partition can therefore expire together. A shared UTC-minute checkpoint
preserves observed progress across backward clock steps and restarts. Unknown
objects and unproved links stop safe storage or retention. FlashGate neither deletes
foreign files nor repairs existing filesystem ownership or permissions.

Native OS permissions govern direct access. On Linux store nodes belong to the
server user and allow no group or other access. On Windows the store receives a
private inheritable security descriptor at creation. The runtime verifies
existing nodes before use. Ordinary FlashGate tools cannot read, list, copy,
write, move or delete the reserved audit tree. Local actors with effective OS
access to the server account or privileged filesystem administration remain
within the OS trust boundary. Audit and technical diagnostics have separate
purposes and neither writes to MCP stdout.

BL-361 owns administrative queries, field and scope authorization, and any
integrity or indexing extension. Future producers reuse the central audit
contract and specify their own decision, correlation, field and release
acceptance. See [Configuration](configuration.md#audit),
[Testing](testing.md#audit-and-failure-path-tests), and the
[implementation contract](planning/implementation-contract.md#bl-166-structured-audit-and-correlation).
