## Verdict: CONCERNS

The R1-1 fix is correct and tested on the path that had the defect. What remains are small gaps: one untested path and two stale documents. None of them can produce a wrong write.

## Findings
- [NOTE] The jobs shape check is tested only on a cancelled run. The failure-class path that shares the same `need_json` call has no malformed-jobs case. — `hack/test/ci_failure_report_test.sh:1426-1430`, `hack/ci/ci-failure-report.sh:112`
  Failure: suppose a later edit moves line 112 inside `if [[ "${class}" == cancelled ]]`. A failure run whose jobs output is empty or `{"jobs":"x"}` then goes on to the `--log-failed` fetch. It only dies later, in the body builder ("the issue body could not be built"), with no `need_json` message. All 16 `gh-output-*` cases still pass, so that regression goes unnoticed. No write happens, so the harm is limited to a misleading error and one extra gh call.
  Fix: add one `jobs` case with `run_listed 162 1 failure`, expecting `expect_stop … jobs` plus the shape message.
  Confidence: 85
- [NOTE] tasks.md 7.1 still describes three reads: "each gh read (run list, issue list, comments)". The code and evidence now cover four, so the task text no longer matches what was built. — `openspec/changes/ci-failure-reporting/tasks.md:234`
  Failure: a later spec or compliance reviewer reads the jobs check as going beyond the spec, or a later edit removes it as out of scope.
  Fix: add "jobs" to the 7.1 list (or cite R1-1 next to it) when the task is ticked.
  Confidence: 70
- [NOTE] The test header's "What that part locks" list (`:24-34`) has no item for the B1 rule: empty, malformed or multi-document gh output with exit 0 is an error. The rule is stated only at `:1412-1417`. — `hack/test/ci_failure_report_test.sh:31-34`
  Fix: add one bullet to the header.
  Confidence: 60

What I checked and found sound:
- **Placement of the check.** The `need_json` filter `type == "object" and (.jobs | type == "array")` at `ci-failure-report.sh:112` runs before the cancelled-class `started` count. That count was the silent no-op: an empty result gives `started=""`, and `[[ "" -gt 0 ]]` is false, so the run was classed as noop.
- **The header comment** (`:23-24`) is now true for every non-log read.
- **The two new mutants.**
  - `gh-output-jobs-empty` replaces the call with `:`, so empty output gives `started=""` and the run becomes noop (exit 0): red.
  - `gh-output-jobs-wrong` drops the array clause, so `.jobs[]?` on a string gives 0 and the run becomes noop: red.
  - Each is judged by its own case. The literals occur once in the reporter, and M11's literal (`:343`) is unchanged.
- **`raw jobs` in the fake gh** sits after `inject jobs`, matching the pattern used for the other three reads.
- **Recorded red** for jobs empty, two and wrong is plausible from the code. Before the fix, two documents make `started="0\n0"`; `[[ … -gt 0 ]]` then fails inside the `if`, so the run became noop with exit 0.
- **R1-3:** the comment now says 1.6 MB, which matches 30000 filler lines.
- **R1-2 comment in check.sh:** it now states its dependence on GNU grep and names the 1.6 MB test that would turn red if grep behaved otherwise. I accept it, given the recorded probe (`0 0` versus `141 0`).
- **Evidence matrix counts:** 16 cases, 7 B1 mutants (`:397-417`), and 7.1 through 7.4 mapped to sensors.

## Could not check
- I ran no sensors (read-only leg). The recorded results (152 `ok` plain, 97 `ok` self-test, `task_check_test.sh` both modes, `task lint:shell`) are taken from the evidence, not reproduced.
- The `task check` run whose log shows the three guards' `--self-test` lines (7.4) is still pending (FINAL).
- I did not verify that real `gh run view --json jobs` emits `"jobs": []` rather than `null` for a run with no jobs. If it emitted `null`, the new check would turn a legitimate noop into exit 1.
- Whether GNU grep's /dev/null behaviour holds on GitHub's ubuntu runner image; I rely on the recorded host probe only.
- I did not re-review the round-1 parts of `0f7455c73..a824dd398` (workflow concurrency move, CWS CFC-6 list) beyond what the round-2 diff touches.
