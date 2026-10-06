# Tasks

## 1. Bump

- [ ] 1.1 Probe (evidence: `evidence/probe.md`, raw output and exit codes for every command —
  do not rely on `task format:check`'s stderr-swallowed view): FIRST run `task lint` at the
  current pin (v2.11.4, `bin/golangci-lint` already built) and record its findings count as
  the before baseline; then in the working tree (uncommitted) set BOTH pins —
  `GOLANGCI_LINT_VERSION` (`Makefile`) and `version:` (`hack/tooling/.custom-gcl.yml`) — to
  v2.13.1, `rm -f bin/golangci-lint*` (make skips the stale file target — verified with
  `make -n`), `make golangci-lint` (custom build with plugins; a vanilla binary fails config
  validation on the custom logcheck linter — record that failure as probe evidence), then
  `task lint` and `task format:check`; record the findings count, any formatter drift, and
  `bin/golangci-lint version`; record the resolved `sigs.k8s.io/logtools` version the plugin
  build used; revert both pins when done
- [ ] 1.2 Version-only commit: `Makefile` and `.custom-gcl.yml` together — the linter version
  in both sites, and the logcheck plugin pin (`version: latest` becomes the resolved version
  recorded by the probe)
- [ ] 1.3 Fix or justify each new finding, one commit per group, no linter disabled; every new
  `//nolint` directive or exclusion carries a reason on the same or preceding line;
  `task lint` and `task format:check` clean; record in `evidence/1.3.md`: findings count
  after, number fixed, number justified, `bin/golangci-lint version` (must report the pin).
  A clean `task lint` at the pin structurally proves the custom build: a vanilla binary
  cannot pass this config (logcheck is unregistered), established by the 1.1 probe evidence

## 2. Land

- [ ] 2.1 Archive the change as the last commit of the PR, after review and green CI; the review record names the reviewed and the archive revision

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| LTB-1 | grep both files; `task lint` + `task format:check` real runs | equal, at least v2.13.1; both clean | not-run | |
| LTB-2 | diff review of `.golangci.yaml` and every `//nolint` in the diff | no disabled linter or blanket exclusion; every new nolint/exclusion has a reason | not-run | |
| LTB-3 | findings count before and after in the review record | recorded, with the fixed/justified numbers and the executed binary's version | not-run | |
| LTB-4 | grep `.custom-gcl.yml` plugin block | a pinned version, not `latest` | not-run | |
| all | independent review; CI on the PR head | APPROVE, green; both revisions recorded | not-run | |
