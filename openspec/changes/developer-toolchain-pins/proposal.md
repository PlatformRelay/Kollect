# Proposal

## Why

The operator wants the same tool versions across their repositories. Re-measured on 2026-10-16
against `main` at 966e17e10 (change 4, cross-file-consistency-gates, merged):

| Tool | Now | Where it is stated |
| --- | --- | --- |
| Task | 3.51.1 | 18 `version:` inputs (17 in `.github/workflows/*`, 1 in `.github/actions/kind-e2e-setup/action.yml`), plus `mise.toml:54` (`task = "3.51.1"`) |
| govulncheck | v1.9.0 | inline in `Taskfile.yml:204`, no variable, no Renovate annotation |
| gitleaks | 8.30.1 | `ci.yaml:164` env, `hack/install-gitleaks.sh:14`, `.pre-commit-config.yaml:5` |
| git-cliff | v2.13.1 | `Taskfile.yml:22` (annotated for Renovate) |
| golangci-lint | v2.14.0 | `Makefile:184`, `hack/tooling/.custom-gcl.yml:6` |

The Renovate side is partly dead. Two custom managers match a `releases/download` URL that
`ci.yaml` does not contain: the gitleaks one (`renovate.json:37-47`; the real pins are the env at
`ci.yaml:164`, `hack/install-gitleaks.sh:14` and `.pre-commit-config.yaml:5`) and the shellcheck
one (`renovate.json:48-58`; CI pins shellcheck through the env at `ci.yaml:299`). Nothing matches
`Makefile:184` or `hack/tooling/.custom-gcl.yml:6`, and Renovate's `pre-commit` manager is off by
default. A manager that matches nothing is silent and looks like "nothing to update".

`mise.toml` comments restate values that live elsewhere: helm v3.21.4 at `mise.toml:28` while
`Taskfile.yml:31` pins v3.22.0, git-cliff and helm-docs versions at `:24`, `go 1.27.2` at `:39`,
and "all 17 occurrences in .github/workflows/" at `:52`, a count that goes stale the first time a
workflow adds or drops a setup-task step.

Two installers download a binary and verify no checksum: `hack/install-git-cliff.sh:40-43` says so,
and `hack/install-helm-docs.sh:40` points at that note. `hack/test/ci_fetch_lib_hardening_test.sh`
(~766) records both in `UNVERIFIED_BY_DESIGN`. `changelog-sync.yaml` runs git-cliff in a job that
holds a ruleset-bypass token, so an unverified download there is a supply-chain hole that the
auto-merge change must not widen.

## What Changes

- Task to 3.52.0 at every site; govulncheck behind one Taskfile variable `GOVULNCHECK_VERSION`
  (Renovate-annotated, value unchanged unless a bot moves it); gitleaks kept in agreement at its
  floor, git-cliff at its floor.
- **Pin ownership (decision A, 2026-10-16, pending operator validation):** golangci-lint and
  govulncheck get no floor and no agreement check here. Change 4's CFC-2
  (`hack/test/consistency_tool_pins_test.sh`) already smoke-runs each analysis tool under
  `go.mod`'s Go and checks `.custom-gcl.yml` against `Makefile`; it owns whether those pins are
  good enough. This change only moves govulncheck into a variable and re-anchors CFC-2's mutants
  that edited the inline literal.
- The drift checks assert that all sites of a tool agree AND are at least the floor, never exact
  values (bot bumps that move every site together stay green).
- Renovate managers: replace the dead gitleaks and shellcheck managers with ones that match the
  real pins; add managers for `Makefile`, `.custom-gcl.yml`, `.pre-commit-config.yaml` and
  `hack/install-gitleaks.sh`; a check fails when a manager matches no file or a pin site is matched
  by none.
- `mise.toml`: Go stays out of `[tools]` (already true and already checked); its comments stop
  restating versions and site counts.
- `install-git-cliff.sh` verifies the per-asset `.sha512` with `sha512sum`; `install-helm-docs.sh`
  verifies against `checksums.txt`. Both leave `UNVERIFIED_BY_DESIGN`, which only shrinks.
- **Guard shape (decision C, 2026-10-16, pending operator validation):** the checks live in a new
  `hack/test/consistency_toolchain_pins_test.sh` (plain and `--self-test`). CFC-6 in
  `hack/test/ci_workflow_security_test.sh` enrols every `consistency_*_test.sh` and requires both
  modes in a required job, so the wiring is enforced without a self-wiring check.
  `hack/test/dev_mise_pin_drift_test.sh` (DEV-MISE-01) is folded into it, mutant for mutant, and
  deleted together with its `ci.yaml` step.

## Capabilities

### New Capabilities

- `developer-toolchain`: the pinned tool floors, where each version lives, how sites are kept in
  agreement, and installer verification.

### Modified Capabilities

None.

## Impact

- Entry points: `Taskfile.yml`, `mise.toml`, `.github/workflows/*`, `.github/actions/*`,
  `renovate.json`, `hack/install-git-cliff.sh`, `hack/install-helm-docs.sh`, the new guard run in
  the `lint` job, and `task check`.
- Test changes on guards this change does not own: CFC-2's govulncheck mutants in
  `hack/test/consistency_tool_pins_test.sh` (change 4), the checksum scan and
  `UNVERIFIED_BY_DESIGN` in `hack/test/ci_fetch_lib_hardening_test.sh`, and the CWS-6 mutant at
  `hack/test/ci_workflow_security_test.sh:1145` that names the deleted guard.

## Dependencies

Landing order across the ten proposed changes: (1) ci-workflow-hardening, (2) task-check-entrypoint,
(3) golangci-lint-bump, (4) cross-file-consistency-gates, (5) developer-toolchain-pins,
(6) dependency-update-automation, (7) ci-failure-reporting, (8) test-depth-signals,
(9) mutation-testing-signal, (10) public-agent-contract.

Change 4 is merged (#463) and this branch starts from it. This change must not land before it:
both review legs raised that decision A leaves golangci-lint and govulncheck to CFC-2 and decision C
relies on CFC-6's enrolment, so without change 4 both tools would be unguarded and the new guard
unwired. Changes 2 and 3 are archived. The installer checksums are a hard prerequisite of change 6.

## Non-goals

- The dependency bot: it is Renovate (operator ruling 2026-10-05, recorded in
  `agent-context/decisions.md`); the managers here are Renovate regex managers.
- Floors or agreement for golangci-lint and govulncheck (CFC-2 owns them), the Go version
  (change 4), the `task check` entry point (change 2).
- Tool versions not listed: helm, shellcheck, polaris, kubeaudit and kind keep their pins
  (shellcheck only gets a live manager).

## Assumptions

- Task 3.52.0 and gitleaks 8.30.1 exist (operator baseline).
- Checksums probed 2026-10-16 for v2.13.1 and v1.14.2: git-cliff publishes
  `<asset>.sha512` (HTTP 200) and no `<asset>.sha256` (HTTP 404); helm-docs publishes
  `checksums.txt` (HTTP 200). Task 4.1 re-probes every platform the installers support.
- `renovate-config-validator --strict` and a local dry run can show which files and which
  dependencies a custom manager matches. Probe in task 3.1.
