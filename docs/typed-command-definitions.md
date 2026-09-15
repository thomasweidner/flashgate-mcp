# Typed Command Definitions

**Status:** Version 1.0 domain contract

**Owner:** BL-137

FlashGate resolves a public `command_id` exclusively through an immutable,
server-owned registry. A command definition references an `executable_id`; a
separate executable definition maps that ID to a clean absolute native-program
path and may pin the binary with a lowercase SHA-256 digest. Neither path nor
digest is public result data. Unknown, duplicate, malformed, or cross-referenced
IDs make catalog construction or lookup fail closed.

Each command definition contains, in deterministic order:

- fixed arguments, including any fixed subcommand;
- a closed list of named argument rules;
- positive timeout, stdout-byte, and stderr-byte maxima; and
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

This contract does not parse requests, construct `argv`, resolve filesystem
paths below named roots, hash or open binaries, enforce platform isolation, or
launch processes. Those operations remain with the subsequent command,
filesystem-security, and Managed Process Engine backlog owners. In particular,
the argument-to-`argv` implementation must still reject response files and
configuration, hook, plugin, loader, and interpreter injection as required by
the [Command Execution Threat Model](command-execution-threat-model.md).
