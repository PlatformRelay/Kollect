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

Recommendation: A. The spec (DTC-5) requires the alias and states the meaning (the Docker-free gates).

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
- **The drift test asserts agreement between sites plus a floor**: every site equals the others
  and is at least the baseline (3.52.0, v2.13.1, 8.30.1, v2.13.1, govulncheck v1.6.0). It does
  not assert exact values: `dependency-update-automation` auto-merges patch/minor bumps of the
  pinned tools, and an exact-value test would make every such bot PR red until a human edited the
  test. The floor stops a downgrade or a lagging site; Renovate's grouping moves the sites together.
- **Go: one version in `go.mod` and both Dockerfiles, 1.27.1.** The image currently ships Go 1.27.1
  while tests and govulncheck run 1.26.6, so the shipped toolchain is never scanned. This change
  bumps `go.mod` (the Dockerfiles already say 1.27.1) and adds a Renovate group so the `go`
  directive and the `golang` image always move in one PR. The equality check itself belongs to
  `cross-file-consistency-gates` (CFC-1), which lands after this change.
- **Renovate managers need a liveness check.** A custom regex manager that matches no file is
  silent and looks like "nothing to update" (`renovate.json:40-46` is one today). The test runs
  each `customManagers` entry's `managerFilePatterns` and `matchStrings` against the tree and fails
  if one matches nothing; new managers cover `Makefile` and `hack/tooling/.custom-gcl.yml`
  (golangci-lint, one group) and the real gitleaks sites. Renovate's `pre-commit` manager is off by
  default, so the `.pre-commit-config.yaml` rev needs a regex manager too.
- **`task check` is defined as the gates that run without Docker or kind**, not "everything CI
  requires": `kind-smoke`, `test-integration` and `docker-build` need a container runtime and would
  make the alias mostly exceptions. They stay separate tasks; the alias lists what it runs.
- **Bump and findings are separate commits** (golangci-lint): the bump commit changes versions
  only; new findings are fixed or justified in later commits, because a gate that is loosened to
  absorb a bump is a weaker gate (the repo's gate-weakening rule).

## Risks / Trade-offs

- [v2.13.1 reveals many findings] -> task 3.2 is open-ended on purpose; the bump does not merge
  until the tree is clean without new `//nolint` unless each carries its reason.
- [A floor does not catch an unwanted upgrade] -> upgrades are what the bot is for; the floor
  catches the dangerous direction (a stale site).
