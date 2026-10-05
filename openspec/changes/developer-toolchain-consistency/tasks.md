# Tasks

## 1. Probe and guard first (DTC-1 to DTC-6)

- [ ] 1.1 Probe: confirm Task 3.52.0, golangci-lint v2.13.1, gitleaks 8.30.1 exist; resolve the latest govulncheck (`go list -m -versions golang.org/x/vuln`); stop and report if a pinned version does not exist
- [ ] 1.2 Extend the drift test (new `hack/test/dev_toolchain_pin_drift_test.sh` or additions to `dev_mise_pin_drift_test.sh`) for DTC-1 to DTC-4 and DTC-6 with throwaway-tree mutants; watch it fail on the 3.51.1 / v2.11.4 / inline govulncheck assertions

## 2. Bumps without behaviour change (DTC-2, DTC-3, DTC-4)

- [ ] 2.1 Task 3.52.0 at every site including `mise.toml`; guard passes for DTC-2
- [ ] 2.2 `GOVULNCHECK_VERSION` variable with Renovate annotation; `task vulncheck` uses it; guard passes
- [ ] 2.3 gitleaks and git-cliff already at baseline: guard only

## 3. golangci-lint (DTC-3)

- [ ] 3.1 Bump `Makefile` and `.custom-gcl.yml` together (version-only commit); run `task lint`, record the findings count
- [ ] 3.2 Fix each new finding or justify it in `.golangci.yaml` with a reason, in separate commits; no linter disabled to absorb the bump

## 4. Entry points (DTC-5)

- [ ] 4.1 Write a test that `task --list-all` lists `check` and `verify` and that every required CI job has a local task reachable from `check` (with an exceptions list stating why, for example kind jobs needing Docker); watch it fail
- [ ] 4.2 Add `check`; update CONTRIBUTING.md and `docs/COMMAND-REFERENCE.md`

## 5. Operator

- [ ] 5.1 (operator) Answer open questions 1 to 3 in design.md; record in the INBOX and `decisions.md`
- [ ] 5.2 (operator, optional) Make `vulncheck` a required merge check once green on main

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| DTC-1 | drift test Go-literal mutant | red on literal, green on tree | not-run | |
| DTC-2 | drift test, one-site and whole-set mutants | both red | not-run | |
| DTC-3 | drift test, lagging `.custom-gcl.yml` and inline-govulncheck mutants; `task lint` and `task vulncheck` real runs | mutants red; lint clean; vulncheck result recorded (red advisories are a finding, not a pass) | not-run | |
| DTC-4 | drift test, pre-commit rev mutant | red | not-run | |
| DTC-5 | `task --list-all`; reachability test | both names listed; uncovered job named | not-run | |
| DTC-6 | empty-scan fixture; no-op control | empty scan red; no-op green | not-run | |
| all | CI on the PR head; independent review | green; APPROVE | not-run | |
