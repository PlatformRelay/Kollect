# Tasks

## 1. Body builder and probes (CFR-5)

- [ ] 1.1 Probe: record a real `workflow_run` payload, `gh run view --json jobs` for a completed and a cancelled-before-start run, and `--log-failed` on an in-progress run; save as fixtures
- [ ] 1.2 Write body-builder tests (two failed jobs, huge log, timeout, hostile text); watch them fail on the assertion
- [ ] 1.3 Implement `hack/ci/ci-failure-issue-body.sh`; tests pass

## 2. Decision logic (CFR-1 to CFR-4, CFR-6, CFR-7)

- [ ] 2.1 Write the decision-table test with a fake `gh` on `PATH`: fail+none, fail+open, pass+open, pass+none, timed_out, cancelled with and without started jobs, dispatch event, fork, stale event, same run number new attempt, spoofed title, concurrent reports, `gh` error; sequence test pass-fail-fail-pass-fail; watch it fail
- [ ] 2.2 Implement `hack/ci/ci-failure-report.sh`; tests pass
- [ ] 2.3 Defect controls: remove the recovery branch, the label-and-author lookup, the ordering check and the event guard in turn; each turns a case red; a no-op control survives

## 3. Workflow

- [ ] 3.1 Add `.github/workflows/ci-failure-report.yaml` (`workflow_run` for `E2E nightly` and `Go patch lag`, `completed`; guards; concurrency per reported workflow, `cancel-in-progress: false`; `issues: write`, `actions: read`; no checkout of the triggering ref); extend the test to assert wiring, guards and default `contents: read`
- [ ] 3.2 Add the `dangerous-triggers` suppression with its reason to `.github/zizmor.yml`; the zizmor gate passes
- [ ] 3.3 Grep `hack/test/` for step-index assertions on workflows touched; run the test in `lint` (plain and `--self-test`)

## 4. Land

- [ ] 4.1 Merge with the `post-merge` row open; archive in a follow-up evidence PR once it passes

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| CFR-1 | `ci_failure_report_test.sh` fail+none, timed_out, cancelled cases, concurrent reports | one issue; never two; unstarted cancel no-op | not-run | |
| CFR-2 | same, fail+open | comment only | not-run | |
| CFR-3 | pass-fail-pass sequence; re-run attempt; stale event; fail after close | recovery closes; stale writes nothing | not-run | |
| CFR-4 | spoofed-title case | not touched | not-run | |
| CFR-5 | body fixtures incl. hostile text | listed, capped, inert | not-run | |
| CFR-6 | dispatch, fork, non-schedule, `gh` error | no action; error visible | not-run | |
| CFR-7 | mutants judged by exit status plus no-op; zizmor gate on the workflow | each mutant red; gate green | not-run | |
| all | first real scheduled night(s) after merge | issue opens on a failure, closes on recovery | post-merge | follow-up evidence PR |
| all | independent review; CI on the PR head | APPROVE, green; reviewed and archive revisions recorded | not-run | |
