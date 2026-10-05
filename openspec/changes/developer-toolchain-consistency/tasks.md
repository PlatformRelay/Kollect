# Tasks

## 1. Probe and guard first (DTC-1 to DTC-6)

- [ ] 1.1 Resolve the latest govulncheck at or above v1.6.0 (`go list -m -versions golang.org/x/vuln` if network works); probe `go build ./... && go vet ./...` and `task test` under Go 1.27.1
- [ ] 1.2 Extend the drift test (new `hack/test/dev_toolchain_pin_drift_test.sh` or additions to `dev_mise_pin_drift_test.sh`) for DTC-1 to DTC-4, DTC-6 and DTC-7 with throwaway-tree mutants (floor, agreement, dead manager, uncovered site, stale comment); watch it fail on the 3.51.1 / v2.11.4 / inline govulncheck / go 1.26.6 vs 1.27.1 / dead gitleaks manager assertions

## 2. Bumps without behaviour change (DTC-1 to DTC-4, DTC-6, DTC-7)

- [ ] 2.0 Probe Renovate: does `gomod` bump the `go` directive at all (`golang` depType, `rangeStrategy`), and which group wins; the golang-image group rule must come after the existing "go minor and patch dependencies" and "docker images" `groupName` rules to take effect. Probe CodeQL's Go extractor on 1.27 (`codeql.yaml`).
- [ ] 2.0b After 3.1 (so golangci-lint v2.13.1, not v2.11.4, runs under Go 1.27.1): bump `go.mod` to `go 1.27.1` (one commit, version only); `task test`, `task lint`, `task vulncheck` recorded; add the `go`-directive + golang-image Renovate group

- [ ] 2.1 Task 3.52.0 at every site including `mise.toml`; guard passes for DTC-2
- [ ] 2.2 `GOVULNCHECK_VERSION` variable with Renovate annotation; `task vulncheck` uses it; guard passes
- [ ] 2.3 gitleaks and git-cliff already at baseline: guard only
- [ ] 2.4 Renovate managers: fix or replace the dead gitleaks manager (`renovate.json:40-46`), add managers for `Makefile` `GOLANGCI_LINT_VERSION`, `hack/tooling/.custom-gcl.yml`, `.pre-commit-config.yaml` and `hack/install-gitleaks.sh`; manager-liveness guard passes
- [ ] 2.5 Replace the stale helm comment in `mise.toml` with a pointer to `Taskfile.yml`

## 3. golangci-lint (DTC-3)

- [ ] 3.1 Bump `Makefile` and `.custom-gcl.yml` together (version-only commit); run `task lint`, record the findings count
- [ ] 3.2 Fix each new finding or justify it in `.golangci.yaml` with a reason, in separate commits; no linter disabled to absorb the bump

## 4. Entry points (DTC-5)

- [ ] 4.1 Write a test that `task --list-all` lists `check` and `verify` and that every Docker-free required CI job has a local task reachable from `check` (exceptions list with reasons); watch it fail
- [ ] 4.2 Add `check`; update CONTRIBUTING.md and `docs/COMMAND-REFERENCE.md`

## 5. Operator

- [ ] 5.1 (operator) Answer open questions 1 to 3 in design.md; record in the INBOX and `decisions.md`
- [ ] 5.2 (operator, optional) Make `vulncheck` a required merge check once green on main

## 6. Land

- [ ] 6.1 Archive the change (`openspec archive`) as the last commit of the PR, after review and green CI; the review record names the reviewed and the archive revision

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| DTC-1 | drift test Go-literal mutant; `go.mod` vs Dockerfile; Renovate group check; `task test`, `task vulncheck` under 1.27.1 | red on mutants; tree green; vulncheck result recorded | not-run | |
| DTC-2 | drift test: one-site and below-floor mutants red; whole-set newer passes | as stated | not-run | |
| DTC-3 | drift test, lagging `.custom-gcl.yml` and inline-govulncheck mutants; `task lint` and `task vulncheck` real runs | mutants red; lint clean; vulncheck result recorded (red advisories are a finding, not a pass) | not-run | |
| DTC-4 | drift test, pre-commit rev mutant | red | not-run | |
| DTC-5 | `task --list-all`; reachability test | both names listed; uncovered job named | not-run | |
| DTC-6 | empty-scan, dead-manager and uncovered-site fixtures; no-op control | each red; no-op green | not-run | |
| DTC-7 | stale-comment mutant | red | not-run | |
| all | CI on the PR head; independent review | green; APPROVE; both revisions recorded | not-run | |
