# Tasks

## 1. Guard first (DTP-1 to DTP-7)

- [ ] 1.1 Extend `hack/test/dev_mise_pin_drift_test.sh` (or add `dev_toolchain_pin_drift_test.sh`) with floors, agreement, empty-scan, inline-govulncheck, stale-comment and manager-liveness checks and throwaway-tree mutants; watch it fail on the 3.51.1 / inline govulncheck / dead gitleaks manager assertions

## 2. Pins (DTP-1 to DTP-3, DTP-5)

- [ ] 2.1 Task at every site including `mise.toml`
- [ ] 2.2 `GOVULNCHECK_VERSION` variable with Renovate annotation; `task vulncheck` uses it
- [ ] 2.3 Replace the stale helm comment in `mise.toml` with a pointer; confirm Go absent from `[tools]`

## 3. Managers (DTP-4)

- [ ] 3.1 Probe which files each custom manager matches (validator plus dry run)
- [ ] 3.2 Fix or replace the dead gitleaks manager; add managers for `Makefile`, `hack/tooling/.custom-gcl.yml`, `.pre-commit-config.yaml`, `hack/install-gitleaks.sh`; liveness guard passes

## 4. Installers (DTP-6)

- [ ] 4.1 Probe the upstream checksum files for git-cliff and helm-docs
- [ ] 4.2 Write a test with a tampered tarball and a missing checksum for each installer (fake `fetch_to`); watch it fail; add verification through `hack/lib/fetch.sh` and update `hack/test/ci_fetch_lib_hardening_test.sh`, which records the git-cliff gap

## 5. Land

- [ ] 5.1 Archive the change as the last commit of the PR, after review and green CI; the review record names the reviewed and the archive revision

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| DTP-1 | drift test: one-site and below-floor mutants red; newer set passes | as stated | not-run | |
| DTP-2 | inline-govulncheck mutant; `task vulncheck` real run | red; run result recorded | not-run | |
| DTP-3 | pre-commit-rev and empty-scan mutants | red | not-run | |
| DTP-4 | dead-manager and uncovered-site fixtures | red | not-run | |
| DTP-5 | go-in-tools and stale-comment mutants | red | not-run | |
| DTP-6 | tampered and missing-checksum cases per installer | installer fails, nothing installed | not-run | |
| DTP-7 | mutants plus no-op, by exit status | mutants red, no-op green | not-run | |
| all | independent review; CI on the PR head | APPROVE, green; both revisions recorded | not-run | |
