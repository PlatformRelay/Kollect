## Verdict: CLEAN

## Findings
- [NOTE] The new ci.yaml comment says the test "checks it is wired here", but the self-wiring check matches the two lines anywhere in ci.yaml, not in the `lint` job. — `.github/workflows/ci.yaml:576-578`, `hack/test/ci_failure_report_test.sh:1672-1686`
  Failure: someone moves both steps into a non-required job. The self-wiring check still passes, and only CWS-6 catches it (evidence *Harness gaps*). The comment tells a reader that this test guards the placement, and it does not.
  Fix: change the wording to "checks ci.yaml runs it in both modes; CWS-6 checks the job is required".
  Confidence: 70
- [NOTE] `names no case` catches only a filter that matches nothing. A typo in one alternative of a multi-case filter is not caught, and that matters in controls. — `hack/test/ci_failure_report_test.sh:57-64`, bump control at about `:262`
  Failure: a later edit turns the bump control's `"wiring|self-wiring"` into `"wiring|self-wirng"`. `SELECTED` stays above 0, so the control passes with the self-wiring case silently dropped. Mutants are safe from this, because `judge` needs every expected message, so a lost case reds.
  Fix: in `control`, give a filter as a list of names and assert each one is selected, or have `select_case` record matched names so the run can check each alternative matched at least once.
  Confidence: 55
- [NOTE] G2 M4 and G2 M5 are judged by the same message, `cap: expected a body of 1..59999 bytes`. That message heads one check made of seven chained clauses. — `hack/test/ci_failure_report_test.sh:602-611` (case), mutants at about `:276-281`
  Failure: the message match cannot tell which clause went red, so "the message of the assertion it targets" holds only per check, not per clause. Evidence row 14 admits this for W7. For M5 it is still sound, because only the truncated-outside-fence clause can change.
  Fix: none needed now. If tighter targeting is wanted later, split the cap check into separate `bad` lines for size, fence order and outside marker.
  Confidence: 40

What I checked, with nothing found:
- **Judging logic:** `judge` and `control` refuse a failed or no-op edit, need a non-zero exit plus a FAIL line per message, and free the copy on every path.
- **Edit helpers:** `lit` replaces exactly once and handles a trailing newline. In `gone` the `grep -v | grep -F` runs under pipefail but `grep -F` reads all its input, so no SIGPIPE. `drop_comment_above` flushes its buffer on non-comment lines.
- **Plain-mode skipping:** `select_case` and the `SKIP` early returns are inert when `CFR_SELFTEST_ONLY` is unset. Checks between cases (the `lookup: limit case not run` branch, the guard and `expect_nocall` loops) take the right case's `SKIP`.
- **Mutant coverage:** each tasks.md 5.1 mutant has a matching `judge` line: recovery, author/label lookup, workflow filter, ordering, the script and `if:` guards, conclusion widened, rename, label step removed and reordered, head-sha `ref:`, both CWS-2 comments, both ci.yaml steps deleted.
- **Case 12b:** the 4000-character cut on an ASCII line equals 4000 bytes.
- **ci.yaml:** the new step is the exact `-Fx` form the self-wiring check needs. The cws gate has its own unmutated-copy control. The round-1 findings are addressed in the diff.

## Could not check
- I did not run the plain mode, `--self-test`, CWS, `dist_ci_wiring_test.sh` or `task lint:shell`. I relied on the results recorded in `evidence/T5.1.md`: 89 ok, exit 0, 182 s.
- I did not read `dist_ci_wiring_test.sh`'s STEP_KEY_ALLOWLIST or CWS-6's required-job logic. The evidence cites them to cover the gap in the first finding, and I have not verified that.
- I did not check that the `lint` runner has `yq`, `iconv` and `sed -E -i` at the versions used locally, or the lint wall-time figures, which the evidence marks as believed.
- I did not read the untracked `reviews/L-tasks/T5.1/round-2/` and `legs.log`, which are out of lens.
