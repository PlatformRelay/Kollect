# Tasks

## 1. Body builder (NFR-4)

- [ ] 1.1 Record a real `gh run view --json jobs` sample and a failed-log excerpt as fixtures; write body-builder tests (two failed jobs, huge log, timeout without FAIL line); watch them fail on the assertion
- [ ] 1.2 Implement `hack/ci/nightly-failure-issue-body.sh`; tests pass

## 2. Decision logic (NFR-1, NFR-2, NFR-3, NFR-5, NFR-6)

- [ ] 2.1 Write the decision-table test with a fake `gh` on `PATH`: fail+none, fail+open, pass+open, pass+none, dispatch event, `gh` error; sequence test pass-fail-fail-pass-fail; watch it fail
- [ ] 2.2 Implement `hack/ci/nightly-report.sh`; tests pass
- [ ] 2.3 Defect controls: remove the recovery branch, the dedupe lookup and the event guard in turn; each turns the test red; one no-op control survives

## 3. Wiring

- [ ] 3.1 Add the `report` job to `e2e-nightly.yaml` (`needs` every job, `if: always()`, `issues: write` on that job only); extend the test to assert the wiring and that the workflow default permission stays `contents: read`
- [ ] 3.2 Run the test in the `lint` job

## 4. Live

- [ ] 4.1 `workflow_dispatch` a scheduled-style run on a branch with a deliberately failing job via a throwaway workflow copy: issue opens, second run comments, passing run closes (record the three run links)

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| NFR-1 | `nightly_failure_report_test.sh` fail+none, re-run case | one issue created, never two | not-run | |
| NFR-2 | same, fail+open | comment, no new issue | not-run | |
| NFR-3 | same, pass-fail-pass sequence; fail after closed | recovery comment and close; new issue after close | not-run | |
| NFR-4 | body fixtures | failed jobs listed, cap respected, marker on truncation | not-run | |
| NFR-5 | dispatch event case; `gh` error case | no action; nightly state unchanged | not-run | |
| NFR-6 | mutants judged by exit status plus no-op control | each mutant red | not-run | |
| all | live throwaway run (4.1) | open, comment, close observed | not-run | |
| all | independent review, CI on PR head | APPROVE, green | not-run | |
