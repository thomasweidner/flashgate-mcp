# Documentation style

## Audience and purpose

Public FlashGate documentation serves users, operators, integrators, and
contributors. It must be sufficient to understand, use, build, test, release,
and contribute to the product without private workflow files or infrastructure.

Retain current product contracts, security boundaries, operating instructions,
reproducible test and build guidance, architecture decisions, and accepted
product plans. Distinguish implemented behavior from accepted targets.

Personal work logs, individual review runs, vacation or mobile work plans,
transient PR queues, and historical backlog or sprint renumbering reports do
not belong in the active public documentation tree. Preserve their evidence
outside the public tree before removal. Do not create a public evidence archive
as a substitute for this separation.

Removing a report does not resolve its findings. Preserve every still-relevant
requirement in its canonical product document, and retain unresolved work under
its existing backlog owner. Never change task status solely because a report
has been removed.

## Language

Write all maintained prose in English, including headings, tables, captions,
example explanations, and comments in documentation examples.

Do not translate public API identifiers, command names, field names, literal
configuration values, proper names, or attribution text. Preserve licenses and
third-party notices as required. Non-English and Unicode data may remain when
it is necessary to demonstrate or test actual product behavior; label that
purpose explicitly.

A language scan identifies candidates for review; it cannot prove that every
sentence is English. Combine automated detection with editorial inspection.
Do not substitute an ASCII-only or umlaut prohibition for language review.

## Filenames

Use short English topic names in `lower-kebab-case.md`. Use directories for
subject grouping. Do not encode a creation date, PR number, sprint number,
review run, authoring client, or personal working mode in the filename.

Keep stable decision identifiers such as `ADR-005` and `DEC-FP-002` in document
headings and references, not as filename prefixes. Do not renumber a decision
when renaming its file. Necessary dates and version applicability belong in
the content.

The explicit conventional-name exceptions are:

- `README.md` at repository or directory entry points;
- root `CHANGELOG.md`, `CONTRIBUTING.md`, `AGENTS.md`, and `BACKLOG.md`;
- root `THIRD-PARTY-NOTICES.md` and `LICENSE`.

This rule applies to maintained documentation, not to technical source-code
naming, externally specified formats, generated release artifacts, or third-party
files. Do not rename technical identifiers or fixtures to enforce prose rules.

Choose the shortest name that remains unambiguous. Do not add unexplained
abbreviations or impose an arbitrary character limit.

## Structure and authority

The documentation entry point identifies user guidance, reference material,
architecture decisions, contributor guidance, and accepted planning separately.
Use one canonical location for each contract and link to it instead of copying
requirements into multiple competing authorities.

`BACKLOG.md` owns task identifiers, ownership, status, and milestone. Keep
assigned identifiers stable and retain concise completed entries without
turning the backlog into an execution-report archive.

`CHANGELOG.md` remains the sole manually maintained narrative release-notes
source. Preserve release history. Include user-relevant migration guidance when
released behavior changes; do not confuse product migration with internal task
renumbering.

## Safe changes

Before moving or removing a document, identify incoming links, section anchors,
plain path references, tests, scripts, CI consumers, and generator inputs.
Preserve decision identifiers and machine-readable section markers. Update all
affected active references together.

Do not erase an architecture decision solely because it contains historical
context. Retain rationale that still explains the product. Resolve contradictory
implementation statements against current code and accepted decisions, without
silently changing security, compatibility, or release contracts.

Archive original evidence byte-for-byte outside the public tree before deleting
its active copy. Record the disposition and source identity in the internal
report. An old Git commit remains history; removing a file from the current tree
does not rewrite Git history.

## Validation

Review every maintained document, not only a fixed list of core files. Check
English prose, naming, local links and anchors, canonical ownership, retained
contracts, and absence of personal workflow dependencies. Classify third-party
and intentional non-English data explicitly rather than ignoring whole active
documents.

Run the documentation consistency gate and directly affected regression tests.
Changes to executable validators also require the applicable shell and platform
gates in the testing documentation. A passing language or link scan is not
proof that implementation and status claims are correct.
