# Tasks

## 1. Bump

- [ ] 1.1 Probe: in the working tree (uncommitted), set BOTH pins — `GOLANGCI_LINT_VERSION`
  (`Makefile`) and `version:` (`hack/tooling/.custom-gcl.yml`) — to v2.13.1, run
  `make golangci-lint` (the custom build with plugins; a vanilla binary fails config
  validation on the custom logcheck linter), then `task lint` and `task format:check`; record
  the findings count, any formatter drift, and `bin/golangci-lint version` before any fix;
  revert both pins when done
- [ ] 1.2 Version-only commit: `Makefile` and `.custom-gcl.yml` together
- [ ] 1.3 Fix or justify each new finding, one commit per group, no linter disabled; every new
  `//nolint` directive or exclusion carries a reason on the same or preceding line;
  `task lint` and `task format:check` clean; record in the review record: findings count
  after, number fixed, number justified, `bin/golangci-lint version` (must report the pin)

## 2. Land

- [ ] 2.1 Archive the change as the last commit of the PR, after review and green CI; the review record names the reviewed and the archive revision

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| LTB-1 | grep both files; `task lint` + `task format:check` real runs | equal, at least v2.13.1; both clean | not-run | |
| LTB-2 | diff review of `.golangci.yaml` and every `//nolint` in the diff | no disabled linter or blanket exclusion; every new nolint/exclusion has a reason | not-run | |
| LTB-3 | findings count before and after in the review record | recorded, with the fixed/justified numbers and the executed binary's version | not-run | |
| all | independent review; CI on the PR head | APPROVE, green; both revisions recorded | not-run | |
