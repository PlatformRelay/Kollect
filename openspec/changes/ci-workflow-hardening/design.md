# Design

## Context

See proposal.md. The reference project removes CI on push to `main`.

## Decisions

- **Push-to-main CI stays, a deliberate counterpoint to the reference project.** attune
  squash-merges onto up-to-date branches, so a green PR head is the merge result and a second run
  adds nothing. Kollect rebase-merges (linear history), so `main` can hold commits whose tree no PR
  run tested, and `verify-eligibility.sh` needs exact-SHA green checks on the release commit.
  CWS-5 makes removal a forbidden outcome.
- **Group by workflow, then PR number or SHA.** Groups are repository-global, so the group is
  `${{ github.workflow }}-${{ github.event.pull_request.number || github.sha }}` with
  `cancel-in-progress: ${{ github.event_name == 'pull_request' }}`. A group keeps one running and
  one pending run, and a newer pending run replaces the older one even without cancel; keyed by
  `github.ref`, three quick merges to `main` would drop the middle SHA's run and
  `verify-eligibility.sh` would fail on it. `e2e-smoke.yaml:31` has the `github.ref` key today.
- **Required checks through the ruleset, not through `test`'s `needs`.** The alternative, adding
  `lint`, `vulncheck`, `workflow-security` and `dependency-review` to `test`'s `needs`, is in-repo
  and testable, but `ci_docs_gate_test.sh` pins `test` to a single step and a specific docs-only
  decision table; widening it risks the CI-DOCSGATE-01 bypass the table exists to prevent. The
  ruleset edit is operator-owned and is a hard prerequisite of every later change that adds a
  guard to `lint` (and of auto-merge). The meta-test cannot see the ruleset, so it asserts what
  it can: each guard script is invoked by a step of a job in the declared required set
  (`required_checks` plus `workflow-security`/`dependency-review`), with each mode (for example
  `--self-test`) as its own step.
- **zizmor offline, suppressions in a file.** Offline avoids a token and network flakiness;
  network-only audits are out of reach (accepted). Existing high findings are fixed (for example a
  `setup-go` cache in a tag or publish workflow becomes `cache: false`). No skip switch (an
  environment variable that disables the job) may appear in any workflow; the meta-test greps.
- **dependency-review default severity.** Loosening needs its own `# why:` line after a measured
  noisy trial. Licences by allow-list, because a deny-list silently admits any new licence.
  Unknown licences are reported, not failed, to avoid blocking on missing metadata.
- **Auto-merged workflow edits need guards.** The step-index grep rule in proposal.md applies.

## Risks / Trade-offs

- [zizmor first run reports many findings] -> task 3.2 fixes or justifies each before the job is
  added, so main never goes red.
- [Making `lint` required blocks merges on lint flakes] -> that is the point; a lint job that can
  be red on `main` and block a release is worse.
- [Cancelling superseded PR runs hides a flaky older run] -> the newest head is what merges.
