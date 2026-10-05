# Proposal

## Why

The operator wants the same tool versions across their repositories. Probed on this tree:

| Tool | Now | Where it is stated |
| --- | --- | --- |
| Task | 3.51.1 | 18 `version:` inputs (17 in `.github/workflows/*`, 1 in `.github/actions/kind-e2e-setup/action.yml`), plus `mise.toml` (`task = "3.51.1"`) |
| govulncheck | v1.1.4 | inline in `Taskfile.yml:199`, no variable, no Renovate annotation |
| gitleaks | 8.30.1 | `ci.yaml` env, `hack/install-gitleaks.sh:14`, `.pre-commit-config.yaml:5` |
| git-cliff | v2.13.1 | `Taskfile.yml:22` (annotated for Renovate) |

The Renovate side is partly dead: `renovate.json:40-46` matches a gitleaks `releases/download` URL
that `ci.yaml` does not contain (the real pins are the env at `ci.yaml:152`,
`hack/install-gitleaks.sh:14` and `.pre-commit-config.yaml:5`), nothing matches `Makefile:184` or
`hack/tooling/.custom-gcl.yml:6`, and Renovate's `pre-commit` manager is off by default. A manager
that matches nothing is silent and looks like "nothing to update". `mise.toml` carries a comment
naming helm v3.21.4 while `Taskfile.yml:31` pins v3.22.0.

Two installers download a binary and verify no checksum: `hack/install-git-cliff.sh:39` says so,
and `hack/install-helm-docs.sh` has no checksum step. `changelog-sync.yaml` runs git-cliff in a job
that holds a ruleset-bypass token, so an unverified download there is a supply-chain hole that
the auto-merge change must not widen.

## What Changes

- Task to 3.52.0 at every site; govulncheck behind one Taskfile variable `GOVULNCHECK_VERSION`
  (Renovate-annotated, at least v1.6.0, exact pin chosen at implementation); golangci-lint,
  gitleaks and git-cliff kept in agreement at their floors.
- The drift test asserts that all sites of a tool agree AND are at least the floor, never exact
  values (bot bumps that move every site together stay green).
- Renovate managers: fix or replace the dead gitleaks one; add managers for `Makefile`,
  `.custom-gcl.yml`, `.pre-commit-config.yaml` and `hack/install-gitleaks.sh`; a check fails when
  a manager matches no file or a pin site is matched by none.
- `mise.toml`: Go stays out of `[tools]` (read from `go.mod` through
  `idiomatic_version_file_enable_tools`), and its version comments shrink to pointers.
- `install-git-cliff.sh` and `install-helm-docs.sh` verify a published checksum.

## Capabilities

### New Capabilities

- `developer-toolchain`: the pinned tool floors, where each version lives, how sites are kept in
  agreement, and installer verification.

### Modified Capabilities

None.

## Impact

- Entry points: `Taskfile.yml`, `mise.toml`, `.github/workflows/*`, `.github/actions/*`,
  `renovate.json`, `hack/install-*.sh`, and the drift tests run in the `lint` job and `task check`.

## Dependencies

Landing order across the ten proposed changes: (1) ci-workflow-hardening, (2) task-check-entrypoint,
(3) golangci-lint-bump, (4) cross-file-consistency-gates, (5) developer-toolchain-pins,
(6) dependency-update-automation, (7) ci-failure-reporting, (8) test-depth-signals,
(9) mutation-testing-signal, (10) public-agent-contract. Needs 3 (the golangci-lint floor is the
bumped version). The installer checksums are a hard prerequisite of change 6.

## Non-goals

- Choosing the dependency bot: an operator question recorded in the change that would move off
  Renovate; nothing here depends on it except that the managers are Renovate regex managers.
- The Go version (change 4), the linter bump (change 3), the `task check` entry point (change 2).
- Tools not listed (helm, shellcheck, polaris, kubeaudit, kind keep their pins).

## Assumptions

- Task 3.52.0, golangci-lint v2.13.1, gitleaks 8.30.1 and govulncheck v1.6.0 exist (operator
  baseline). A newer govulncheck may be chosen at implementation.
- Upstream publishes checksum files for git-cliff and helm-docs releases; probe in task 4.1 and
  stop if one is missing.
- `renovate-config-validator --strict` and a local dry run can show which files a custom manager
  matches. Probe in task 3.1.
