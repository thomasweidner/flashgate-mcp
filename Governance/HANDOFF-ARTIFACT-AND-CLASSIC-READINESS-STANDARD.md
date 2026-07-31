# Handoff Artifact and Classic Review Readiness Standard

**Status:** Binding
**Tasks:** BL-333 foundation and BL-334 enforcement

## Principle

A handoff is complete only when the next actor's required inputs exist,
identify one authoritative source state, and pass integrity and scope checks.
A report that names an uncreated or inaccessible artifact is incomplete.

The cross-project storage rule is authoritative. FlashGate produces exactly
one handoff file below `<CodexTempRoot>` or an explicitly authorized
assignment-specific override. When exactly one payload file is required, that
file may be transferred directly without a ZIP. When more than one payload
file is required, the handoff is exactly one ZIP archive. Package members are
never requested, uploaded, or transferred separately. Task-related names
begin with the identifier confirmed in the leading Local Work Register.

## Required payload by transition

| Transition | Required payload |
|---|---|
| Implementation to independent full review | Handoff README, report, complete feature patch, scope inventory, focused evidence, and manifest. |
| Bundled correction to focused delta review | Handoff README, correction report, byte-exact `correction-only.patch` and `current-delta.patch`, separately trusted expected hashes, assignment/completion records, strict correction and regression matrices, strict focused-delta record, bounded report contract, external before/after deltas, scope inventory, validation evidence, and manifest. |
| Commit preparation to commit approval | Exact staged/unstaged scope, commit-preparation report, patch, assignment record, validation evidence, and manifest. |
| Exact commit to push approval | Commit SHA, commit patch, exact-commit validation, scope inventory, and manifest. |
| Push/PR to Hosted CI review | PR identity, head SHA, changed paths, workflow/run identities, and authoritative source URLs or immutable identifiers. |

Ignored build output and unrelated preparation workspaces are excluded unless
the next step explicitly requires them.

## Patch and scope requirements

The patch must:

- use unified Git format and the declared baseline;
- include tracked changes, intended untracked source, modes, renames, and
  binary-safe data when applicable;
- exclude unintended generated output and secrets;
- parse and preferably pass `git apply --check` against an isolated baseline.

The inventory records repository, baseline, branch, HEAD, path, Git status,
tracked/staged state, mode, byte size, SHA-256, inclusion decision, and reason.
Repository status, report paths, patch paths, inventory paths, and manifest
paths must agree.

## Manifest and package validation

The root `MANIFEST.sha256` is ordinally sorted and covers every payload file
exactly once while excluding itself. Each line contains lowercase SHA-256,
decimal byte size, and safe relative path. The final ZIP is reopened and
validated independently. Absolute/traversing ZIP paths, duplicate entries,
case-colliding paths, symlink/junction/reparse entries or source targets,
invalid UTF-8, control characters, unresolved placeholders, secret material,
and unintended host/user paths fail closed.

Any payload-byte, required-file, path, manifest, readiness, or instruction
change invalidates the previous package. Rebuild the complete package from a
fresh staging root, regenerate the full manifest, reopen the resulting ZIP,
and validate every entry hash and length. Incremental archive updates and a
manifest carried forward from older payload bytes are forbidden. Extra
unmanifested objects and missing required objects fail closed.

The authoritative external artifact gate is:

```text
C:\Users\ThomasW\OneDrive - VOXTRONIC\Desktop\Voxtronic\Codex-Work\Scripts\Test-ClassicReviewArtifact.ps1
```

Binding invocation under PowerShell 7.6.4:

```powershell
& {
    $result = & 'C:\Users\ThomasW\OneDrive - VOXTRONIC\Desktop\Voxtronic\Codex-Work\Scripts\Test-ClassicReviewArtifact.ps1' `
        -ArtifactPath '<absolute-package-path>' `
        -ReadinessRequirement RequireTrue
    $exitCode = $LASTEXITCODE
}
```

Exit code `0` is PASS, `1` is a content or readiness failure, and `2` is an
invocation or infrastructure error. The validator path and SHA-256 used by the
producer are recorded in the assignment and completion records. The repository
validator is a prerequisite static/semantic gate; it is not an alternative to
this authoritative external artifact gate.

`HANDOFF.md` contains exactly one visible status block between
`<!-- BEGIN GOVERNANCE-HANDOFF-STATUS -->` and
`<!-- END GOVERNANCE-HANDOFF-STATUS -->`, followed by exactly one strict JSON
contract between `<!-- BEGIN GOVERNANCE-HANDOFF-CONTRACT -->` and
`<!-- END GOVERNANCE-HANDOFF-CONTRACT -->`. The productive handoff producer
generates both blocks from one typed status source.

The validator counts each of the four marker lines independently and requires
exactly one of each in strict status-before-contract order. The visible block
contains exactly these case-sensitive keys, once each and in canonical order:
`Status`, `CorrectionMode`, `TargetFindingCount`, `CorrectedFindingCount`,
`PendingDeltaFindingCount`, `ClosedFindingCount`, `OpenFindingCount`,
`ClassicReviewReady`, `TargetFindings`, `PendingFindings`, `ClosedFindings`,
`Run007Status`, `CommitPreparationApproved`, `CommitAuthorized`,
`RequiredReviewMode`, and `NextAction`. Unknown keys, duplicate keys, empty or
mistyped values, non-canonical lists, and reserved `Key: value` control lines
outside the visible block fail closed. Every visible value is parsed by type
and compared exactly with the JSON field or its bounded array-derived count;
substring checks are not parity evidence.

The JSON contract is validated by
`Governance/governance-handoff-contract.schema.json` and must agree with the
assignment, completion report, correction and regression matrices, focused
record, readiness evidence, report contract, validation summary, Local Work
Register delta, and every parsed visible handoff value. Status parity is true
only when the contract, visible-key, visible-value, independent-marker,
reserved-control-line, finding, count, queue, and package-contract gates all
pass.

## ClassicReviewReady boundary

`ClassicReviewReady=true` is permitted only when:

- every transition-required artifact exists;
- the actual correction/current-delta entry bytes match separately supplied
  expected hashes and every manifest and record declaration;
- patch and inventory scope agree exactly with assignment, focused record,
  completion report, correction matrix, report contract, and repository status;
- all current correction/review semantic contracts contain the same unique
  target findings, per-finding
  status/evidence, direct interfaces, regression tests, and no unknown fields;
- external manifest, payloads, completion report, report contract, and
  narrative path section contain exactly five canonical external paths and
  scopes with the required one-to-one mapping. Windows-normalized,
  case-insensitive duplicate paths, scope swaps, missing/extra scopes, and
  alternate spellings fail closed;
- manifest and ZIP reopen checks pass;
- the assignment governance record passes;
- the strict completion-report contract passes;
- every registered external change has a manifest-bound, hash-matching review
  delta;
- no open decision boundary or required artifact remains; and
- the package remains outside the active repository.

False, absent, mistyped, or conflicting readiness is never promoted to true.
An instruction to transfer individual members of a multi-file package also
fails readiness, even if the ZIP itself is technically valid.

## Required report fields

- `HandoffRequired`
- `HandoffPackage`
- `HandoffArtifacts`
- `ScopeInventoryResult`
- `PatchCompletenessResult`
- `HandoffManifestResult`
- `HostedCISourceVerification`
- `MissingHandoffArtifacts`
- `ClassicReviewReady`

`NextAction` must not instruct the user to pass an artifact that the producing
assignment did not create.
