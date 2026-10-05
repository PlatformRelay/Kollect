# Proposal

## Why

The operator wants the same tool versions and the same entry-point names across their projects.
Probed on this tree, kollect's baseline is mixed:

| Tool | Now | Where it is stated |
| --- | --- | --- |
| Go | `go 1.26.6` | `go.mod`; every setup reads it (`go-version-file: go.mod` in `.github/actions/go-cache/action.yml`) |
| Task | 3.51.1 | 17 `version:` inputs in `.github/workflows/*` and `.github/actions/kind-e2e-setup/action.yml`, plus `mise.toml` (`task = "3.51.1"`) |
| golangci-lint | v2.11.4 | `Makefile:184` AND `hack/tooling/.custom-gcl.yml:6` (two places, no check) |
| govulncheck | v1.1.4 | inline in `Taskfile.yml:199` (no variable, no Renovate annotation) |
| gitleaks | 8.30.1 | `ci.yaml` env, `hack/install-gitleaks.sh:14`, `.pre-commit-config.yaml:5` |
| git-cliff | v2.13.1 | `Taskfile.yml:22` (annotated for Renovate) |

`task verify` means "generated artifacts are fresh" (`Taskfile.yml:111`), not "run every gate", and
there is no `task check`.

## What Changes

- Raise Task to 3.52.0 and golangci-lint to v2.13.1 everywhere they appear; govulncheck to the
  latest release (exact pin chosen at implementation, at least v1.6.0); keep gitleaks 8.30.1 and
  git-cliff v2.13.1.
- Give govulncheck a `GOVULNCHECK_VERSION` variable in `Taskfile.yml` with the same Renovate
  annotation as the others.
- Extend `hack/test/dev_mise_pin_drift_test.sh` (or a sibling) so each tool's multiple sites must
  agree and each equals the baseline, and a guard keeps `go-version-file: go.mod`.
- Add `task check` as an alias of the full local gate matrix that CI runs; `task verify` stays.

## Capabilities

### New Capabilities

- `developer-toolchain`: the pinned tool baseline, where each version lives, and the standard task
  entry points.

### Modified Capabilities

None.

## Impact

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
- Bumping Go itself.

## Assumptions

- Task 3.52.0, golangci-lint v2.13.1 and gitleaks 8.30.1 exist as releases. Stated by the operator
  as the cross-repo baseline; probe in task 1.1 (`gh release view` on each, or `go list -m -versions`
  for the module-based tools) before any edit, and stop if one is missing.
- govulncheck's latest version could not be resolved offline when this proposal was written.
  Requirement DTC-3 therefore says ">= v1.6.0, exact pin chosen at implementation"; probe with
  `go list -m -versions golang.org/x/vuln` in task 1.1.
- `.golangci.yaml` is `version: "2"`; v2.13.1 may add default-enabled behaviour or deprecate
  linter options. Probe: run `make golangci-lint lint` after the bump in task 3.1.
- `hack/tooling/.custom-gcl.yml` `version:` must equal the golangci-lint version it builds on;
  that is why two places must move together (the custom build fails or misbehaves on mismatch;
  probe in task 3.1).
