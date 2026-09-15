# GitHub Actions pinning policy

FlashGate pins every third-party GitHub Action used by a repository workflow to
an exact, lowercase 40-character commit SHA. A nearby `# vN` or `# vN.N.N`
comment records the reviewed upstream release for humans and Dependabot. Tags
and branches are mutable and therefore are not accepted as execution identities.

Repository-local actions (`./...`) are bound by the checked-out repository
revision. Docker actions, if introduced, require a separate reviewed image
digest policy; `docker://` references are not silently treated as satisfying
the GitHub Action commit policy.

## Updating an action

1. Use the normal Dependabot pull request or open a focused dependency update.
2. Resolve the proposed upstream release tag to its commit through the official
   upstream repository and record both the immutable SHA and readable release
   comment.
3. Review the upstream release notes and action source between the old and new
   commits, including runtime changes, permissions, inputs, outputs, and
   transitive download or execution behavior.
4. Keep workflow permissions least-privileged and do not add credentials or
   broaden event triggers as part of a routine pin update.
5. Run the workflow pinning test and the directly affected workflow/shell gates.
   Hosted CI remains the authoritative execution check for GitHub-hosted jobs.

Security updates use the same review and immutable pin requirements; urgency
does not permit a floating tag. Rollback restores a previously reviewed exact
SHA in a new reviewed commit. Automated dependency proposals never merge
without review, and no workflow downloads an action by an unreviewed branch.

The permanent repository gate is `TestWorkflowsPinExternalActions` in
`internal/workflowpolicy`. It scans all YAML files under
`.github/workflows`, rejects floating or abbreviated external references, and
requires the human-readable reviewed-version comment.
