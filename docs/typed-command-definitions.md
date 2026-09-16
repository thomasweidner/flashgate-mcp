# Typed Command Definitions

**Status:** Version 1.0 domain contract

**Owners:** BL-137 (registry), BL-138 (typed invocation construction), BL-140
(timeout policy)

FlashGate resolves a public `command_id` exclusively through an immutable,
server-owned registry. A command definition references an `executable_id`; a
separate executable definition maps that ID to a clean absolute native-program
path and may pin the binary with a lowercase SHA-256 digest. Neither path nor
digest is public result data. Unknown, duplicate, malformed, or cross-referenced
IDs make catalog construction or lookup fail closed.

Each command definition contains, in deterministic order:

- fixed arguments, including any fixed subcommand;
- a closed list of named argument rules;
- a positive default timeout not exceeding a positive timeout maximum;
- positive stdout-byte and stderr-byte maxima; and
- an explicit `allowed` or `denied` network policy.

An argument rule has one of five closed value kinds: boolean, bounded integer,
bounded string, enumeration, or root-bound path. Enumerations list every
accepted value. Integer rules provide both bounds. String and path rules have a
positive length bound, and path rules list one or more named roots. A rule may
associate its value with one exact flag or define a positional value. Flag
rules precede positional rules, and names and flags are unique within a
definition.

Registry construction defensively copies all definitions, constraints, and
optional identity pins. Resolution also returns copies. Configuration owners
and callers therefore cannot mutate validated policy after startup.

Invocation construction accepts only a closed object keyed by the definition's
argument names. Values must have the exact configured boolean, integer, string,
enumeration, or root-relative path type; missing required fields, unknown
fields, nulls, numeric coercion, excess bounds, absolute paths, and traversal
fail closed. Definition order, not request-map order, determines `argv` order.
The executable path and argument array remain separate, so no shell command
string is representable and shell metacharacters stay literal argument data.

Response-file syntax and value-led option injection are rejected. Registry
construction also rejects fixed arguments or rule flags that select generic
configuration, hook, plugin, loader, interpreter, or alternate-executable
surfaces. Path values carry an approved root ID and clean relative path; a
platform-aware resolver must bind them to a clean absolute path and perform
effective containment and link/reparse checks at the process-creation boundary.

Every constructed invocation carries an effective timeout. When the request
omits a timeout, the server-owned command default applies. An explicit timeout
must be positive and no greater than the command maximum; invalid values are
rejected rather than silently clamped. The later Managed Process Engine must
turn this effective value into a server-controlled deadline and retain control
of cancellation and process-tree cleanup.

This contract does not hash or open binaries, implement those platform path
checks, enforce platform isolation, or launch processes. Those operations
remain with the subsequent filesystem-security and Managed Process Engine
backlog owners described by the
[Command Execution Threat Model](command-execution-threat-model.md).
