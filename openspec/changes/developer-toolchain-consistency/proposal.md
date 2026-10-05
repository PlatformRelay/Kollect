# Proposal

## Why

The operator wants the same tool versions and the same entry-point names across their projects.
Probed on this tree, kollect's baseline is mixed:

| Tool | Now | Where it is stated |
| --- | --- | --- |
| Go | `go 1.26.6` in `go.mod`, `golang:1.27.1` in `Dockerfile:2` and `Dockerfile.pipeline:4` | `go.mod` (every setup reads it via `go-version-file: go.mod`) and two Dockerfiles that disagree with it |
| Task | 3.51.1 | 18 `version:` inputs (17 in `.github/workflows/*`, 1 in `.github/actions/kind-e2e-setup/action.yml`), plus `mise.toml` (`task = "3.51.1"`) |
| golangci-lint | v2.11.4 | `Makefile:184` AND `hack/tooling/.custom-gcl.yml:6` (two places, no check) |
| govulncheck | v1.1.4 | inline in `Taskfile.yml:199` (no variable, no Renovate annotation) |
| gitleaks | 8.30.1 | `ci.yaml` env, `hack/install-gitleaks.sh:14`, `.pre-commit-config.yaml:5` |
| git-cliff | v2.13.1 | `Taskfile.yml:22` (annotated for Renovate) |

`task verify` means "generated artifacts are fresh" (`Taskfile.yml:111`), not "run every gate", and
there is no `task check`.

## What Changes

- Go: bump `go.mod` to 1.27.1 so it equals the Dockerfiles (the shipped image is then built and
  tested with one toolchain; `Taskfile.yml:14` takes `GOTOOLCHAIN` from `go.mod`, so govulncheck
  scans it too). Renovate groups the `go` directive and the golang image so they move together.
- Raise Task to 3.52.0 and golangci-lint to v2.13.1 everywhere they appear; govulncheck to
  v1.6.0 or later (a newer pin may be chosen at implementation); keep gitleaks 8.30.1 and
  git-cliff v2.13.1.
- Give govulncheck a `GOVULNCHECK_VERSION` variable in `Taskfile.yml` with the same Renovate
  annotation as the others.
- Extend `hack/test/dev_mise_pin_drift_test.sh` (or a sibling) so each tool's multiple sites must
  agree and be at least the baseline (a floor, not an exact value, so Renovate bumps that move
  every site together stay green), and a guard keeps `go-version-file: go.mod`.
- Fix the Renovate regex managers: `renovate.json:40-46` matches a gitleaks `releases/download`
  URL that `ci.yaml` does not contain (the real pins are the env at `ci.yaml:152`,
  `hack/install-gitleaks.sh:14` and `.pre-commit-config.yaml:5`), and nothing matches
  `Makefile:184` or `.custom-gcl.yml:6`. A check fails when a custom manager matches no file.
- Fix the stale `mise.toml` comment that says helm v3.21.4 (`Taskfile.yml:31` is v3.22.0) and guard
  it by reducing the comment to a pointer.
- Add `task check` as an alias of the full local gate matrix that CI runs; `task verify` stays.

## Capabilities

### New Capabilities

- `developer-toolchain`: the pinned tool baseline, where each version lives, and the standard task
  entry points.

### Modified Capabilities

None.

## Impact

- Landing order across the seven proposed changes: developer-toolchain-consistency (this one, with
  the Go bump) first, then cross-file-consistency-gates, ci-workflow-hardening,
  dependency-update-automation, nightly-failure-reporting, test-depth-signals,
  mutation-testing-signal, public-agent-contract. This change and ci-workflow-hardening both edit
  every job and both meta-tests parse the workflows, so they land sequentially.
- Entry points: `Taskfile.yml`, `Makefile`, `hack/tooling/.custom-gcl.yml`, `mise.toml`,
  `.github/workflows/*`, `.github/actions/*`, `renovate.json` (regex managers), and the drift tests
  run in the `lint` job.
- golangci-lint v2.13.1 may report new findings; that is planned as its own task (3.2), not folded
  into the bump.
- The vulncheck state on main: the harness notes it was red on main for stdlib advisories. See the
  open question in design.md.

## Non-goals

- Choosing the dependency bot (open question A/B/C in design.md; not decided here).
- Renaming `task verify`.
- Changing any tool not listed (helm, shellcheck, polaris, kubeaudit, kind keep their pins).

## Assumptions

- Task 3.52.0, golangci-lint v2.13.1, gitleaks 8.30.1 and govulncheck v1.6.0 are known to exist
  (the operator's cross-repo baseline); no existence probe is planned.
- govulncheck v1.6.0 is the floor; a newer release may be pinned at implementation
  (`go list -m -versions golang.org/x/vuln` in task 1.1 if network works).
- Go 1.27.1 builds the module and the dependencies at their current versions; probe with
  `go build ./... && go vet ./...` in task 2.0 before the bump lands (the bump must not break CI).
- `.golangci.yaml` is `version: "2"`; v2.13.1 may add default-enabled behaviour or deprecate
  linter options. Probe: run `make golangci-lint lint` after the bump in task 3.1.
- `hack/tooling/.custom-gcl.yml` `version:` must equal the golangci-lint version it builds on;
  that is why two places must move together (the custom build fails or misbehaves on mismatch;
  probe in task 3.1).
