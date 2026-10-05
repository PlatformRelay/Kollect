# Design

## Context

See proposal.md. Two choices belong to the operator and are written here as questions.

## Open question 1: which dependency bot

Context: kollect runs self-hosted Renovate (`.github/workflows/renovate.yaml`, `renovate.json`),
whose regex managers keep Taskfile, mise and workflow tool pins moving together. Other projects the
operator maintains may use Dependabot. The version-consistency goal needs the pins to move
together; that is a Renovate strength and a Dependabot gap.

Options:

- **A. Renovate everywhere, with a GitHub App token.** Regex managers cover Taskfile, mise,
  Makefile and install-script pins; grouping keeps one tool in one PR; one config language.
  Cost: self-hosting and an App per repository (or one org App) to operate.
- **B. Dependabot for ecosystems it handles (gomod, github-actions, docker, npm), Renovate only
  for the regex-manager pins** (the approach of the reference project attune). Less to host for
  the common case; two systems and two PR streams, and Dependabot cannot group a tool across
  Taskfile + workflow + mise.
- **C. Keep as is** (Renovate here, whatever each repo has elsewhere). No work; the cross-repo
  baseline then depends on each repo's own bot.

Recommendation: **A**, because version consistency is exactly the grouping Renovate does and
Dependabot cannot, and `dependency-update-automation` already fixes the token gap. Decision field:
operator answer to be recorded in the INBOX and decisions log, not in this change.

## Open question 2: `task check` versus `task verify`

Context: the baseline wants the same entry-point names in every project. Here `verify` already
means generated-artifact drift.

Options:

- **A. Add `check` as an alias for the full local gate matrix; keep `verify`.** No churn; two names
  for different things (`verify` narrow, `check` broad). Recommended.
- **B. Rename `verify` to `check` and make `verify` the broad one.** Matches other repos if they use
  `verify` broadly; renames a name used in CONTRIBUTING.md, ADR checklists and CI (`ci.yaml`,
  `verify` is a required check name via the job, not the task).
- **C. Do nothing.**

Recommendation: A. The spec (DTC-5) requires the alias and states the meaning.

## Open question 3: should `vulncheck` be a required status check

`vulncheck` already appears in `hack/release/verify-eligibility.sh` `required_checks` (so a red
`vulncheck` blocks a release) but is not necessarily a ruleset-required check for merges.
Options: A. Require it for merges once green on main for a week (recommended, then stdlib
advisories block merges until the toolchain bump lands, which is the point); B. keep release-only;
C. leave as is. Operator-owned ruleset change; not part of the tasks.

## Decisions

- **Where each version lives.** Task: workflows are the authority (a `uses:` input cannot read a
  file) and `mise.toml` mirrors them (existing rule). golangci-lint: `Makefile` is the authority;
  `.custom-gcl.yml` mirrors it. govulncheck: `Taskfile.yml` variable. gitleaks: the install script
  default is the authority; the CI env and pre-commit `rev` mirror it. Mirrors are enforced by a
  drift test, as `dev_mise_pin_drift_test.sh` does for Task.
- **The baseline values are asserted in the drift test**, not just consistency, so a bot PR that
  moves all sites together to a different version still reds the test and forces a deliberate
  baseline change (a one-line edit that reviewers see). Trade-off: every legitimate bump edits the
  test too; Renovate grouping plus the test message make that a mechanical step.
- **Bump and findings are separate commits** (golangci-lint): the bump commit changes versions
  only; new findings are fixed or justified in later commits, because a gate that is loosened to
  absorb a bump is a weaker gate (the repo's gate-weakening rule).

## Risks / Trade-offs

- [v2.13.1 reveals many findings] -> task 3.2 is open-ended on purpose; the bump does not merge
  until the tree is clean without new `//nolint` unless each carries its reason.
- [Asserting baseline values makes bot bumps need a test edit] -> see above; mechanical.
