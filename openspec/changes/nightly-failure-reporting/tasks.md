# Tasks

## 1. Body builder (NFR-4)

- [ ] 1.1 Probe `gh run view --log-failed` on an in-progress run (expected to refuse) and on a completed one; record a real `gh run view --json jobs` sample and a failed-log excerpt as fixtures; write body-builder tests (two failed jobs, huge log, timeout without FAIL line); watch them fail on the assertion
- [ ] 1.2 Implement `hack/ci/nightly-failure-issue-body.sh`; tests pass

## 2. Decision logic (NFR-1, NFR-2, NFR-3, NFR-5, NFR-6)

- [ ] 2.1 Write the decision-table test with a fake `gh` on `PATH`: fail+none, fail+open, pass+open, pass+none, dispatch event, `gh` error; sequence test pass-fail-fail-pass-fail, conclusions `timed_out` and `cancelled`; watch it fail
- [ ] 2.2 Implement `hack/ci/nightly-report.sh`; tests pass
- [ ] 2.3 Defect controls: remove the recovery branch, the dedupe lookup and the event guard in turn; each turns the test red; one no-op control survives

## 3. Wiring

- [ ] 3.1 Add `.github/workflows/nightly-report.yaml` (`workflow_run` on `E2E nightly`, `completed`; same-repo and `event == 'schedule'` guards; concurrency per reported workflow, `cancel-in-progress: false`; `issues: write`, `actions: read`; no checkout of the triggering ref); extend the test to assert the wiring, the guards, and that the workflow default permission stays `contents: read`
- [ ] 3.2 Run the test in the `lint` job

## 4. Live

- [ ] 4.1 `workflow_run` fires only for workflow files on the default branch, and the `schedule` guard filters dispatch runs, so a pre-merge live probe is impossible here. Evidence is the first real scheduled night(s) after merge (a failing night opens an issue, a later green night closes it), or a throwaway repository with the same two workflows; the row stays not-run until then

## 5. Land

- [ ] 5.1 Archive the change (`openspec archive`) as the last commit of the PR, after review and green CI; the review record names the reviewed and the archive revision

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| NFR-1 | `nightly_failure_report_test.sh` fail+none, timed_out, cancelled, concurrent-report case | one issue created, never two | not-run | |
| NFR-2 | same, fail+open | comment, no new issue | not-run | |
| NFR-3 | same, pass-fail-pass sequence; fail after closed | recovery comment and close; new issue after close | not-run | |
| NFR-4 | body fixtures | failed jobs listed, cap respected, marker on truncation | not-run | |
| NFR-5 | dispatch, fork and non-schedule event cases; `gh` error case | no action; error visible in the report run | not-run | |
| NFR-6 | mutants judged by exit status plus no-op control | each mutant red | not-run | |
| all | first scheduled nights after merge, or a throwaway repository (4.1) | open, comment, close observed | not-run | |
| all | independent review, CI on PR head | APPROVE, green; reviewed and archive revisions recorded | not-run | |
