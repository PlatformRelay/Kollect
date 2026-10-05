# Tasks

## 1. Bump

- [ ] 1.1 Probe: `.custom-gcl.yml` mismatch behaviour; run the old config under v2.13.1 and record the findings count (before any fix)
- [ ] 1.2 Version-only commit: `Makefile` and `.custom-gcl.yml` together
- [ ] 1.3 Fix or justify each new finding, one commit per group, no linter disabled; `task lint` clean

## 2. Land

- [ ] 2.1 Archive the change as the last commit of the PR, after review and green CI; the review record names the reviewed and the archive revision

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| LTB-1 | grep both files; `task lint` real run | equal, at least v2.13.1; lint clean | not-run | |
| LTB-2 | diff review of `.golangci.yaml` | no disabled linter or blanket exclusion; nolint with reasons | not-run | |
| LTB-3 | findings count before and after in the review record | recorded | not-run | |
| all | independent review; CI on the PR head | APPROVE, green; both revisions recorded | not-run | |
