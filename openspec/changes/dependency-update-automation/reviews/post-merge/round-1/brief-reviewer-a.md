# Review brief: one-commit CI fix

Cold, read-only reviewer. Repository: the current directory. Review `git show HEAD` (a change to
`.github/workflows/e2e-extended.yaml`) against its commit message.

Context: the repository's self-hosted Renovate auto-merges Docker digest PRs only when every
check run on the head SHA is green (Renovate 44.179.1 `getBranchStatus`: any check run whose
conclusion is not success/skipped/neutral makes the branch pending; the GitHub check-runs API
returns the latest run per check name). Renovate opens a PR and then adds the `dependencies`
label, which fired a second `pull_request` `labeled` run of this workflow in the same concurrency
group; it cancelled the first, leaving cancelled check runs on the head SHA (seen on #503 and #504).

Check: does the fix stop that cancellation for any label but `e2e/full` while keeping the
existing behaviour (an `e2e/full` label and new pushes still supersede; main never cancels; the
gate still runs the heavy jobs for an `e2e/full` label and for relevant file changes)? Is the
expression in `concurrency.group` valid GitHub Actions syntax and safe (no injection: label names
flow only into a group key and an env var)? Any case where the labeled run now produces a failed
or cancelled check that would still block Renovate? Any guard under `hack/test/` that this breaks?

Output a findings table (ID, severity CRITICAL / WARNING / NIT, file:line, finding, fix), then
`## Verdict` with exactly one of APPROVE, CONCERNS, BLOCK.
