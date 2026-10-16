## Verdict: CONCERNS

## Findings
- [WARNING] The B1 hardening leaves out the fourth gh read (`gh run view --json jobs`), and the script header now claims it covers every read — `hack/ci/ci-failure-report.sh:23-24`, `:108-114`
  Failure: a scheduled run concludes `cancelled` after a job started, and `gh run view --json jobs` exits 0 with empty output, `{}`, or two documents. With empty output, `jq '[.jobs[]? …] | length'` prints nothing and exits 0, so `started=""`, `[[ "" -gt 0 ]]` is false, the class becomes `noop`, and the script exits 0 without a report. That is the "empty read means nothing found" failure that B1 calls an error, here as a missed issue instead of a duplicate one. The header (":23-24 so does a read that exits 0 without exactly one JSON document") reads as covering this read too. Evidence G7.md:72-75 tags the gap as **believed** harmless, but a silent missed report is not harmless under CFR-1's decision table (cancelled with a started job counts as failure).
  Fix: add `need_json "${WORK}/jobs.json" 'type == "object" and (.jobs | type == "array")' "gh run view --json jobs"` after `:108`, plus a `raw-jobs` case in the fake gh. Otherwise, narrow the header comment to "the run list, issue list and comments reads".
  Confidence: 75
- [NOTE] The H1 fix depends on how GNU grep happens to behave, not on anything grep guarantees — `hack/check.sh:83-85`
  Failure: GNU grep with stdout on `/dev/null` also stops matching at the first hit (`done_on_match`). It avoids SIGPIPE only because `finalize_input` then drains the rest of stdin from the pipe. A grep without that drain (BSD/macOS grep, busybox) brings back the silent skip. `task_check_test.sh` would catch that on such a host, so it can't regress unnoticed, but the comment's explanation is incomplete.
  Fix: use one process with no pipe, e.g. `awk '!/^[[:space:]]*#/ && /--self-test/ { f = 1 } END { exit !f }' "$1"`.
  Confidence: 55
- [NOTE] The test comment gives the file size as "About 1.2 MB", but the evidence records 1638958 bytes (about 1.6 MB) — `hack/test/task_check_test.sh:144`
  Failure: the comment is misleading only. The 64 KiB margin is the point and still holds.
  Fix: change the comment to "about 1.6 MB".
  Confidence: 90
- [NOTE] Fixing H1 turns on self-tests that `task check` silently skipped before: `ci_failure_report` (~3.5 min), `ci_workflow_security` (~2 min), `consistency_toolchain_pins` (~8 min per CWS comment `:617`) and, intermittently, others. Local `task check` wall time goes up by several minutes. G7.md:180 and :240 still show the run that proves these self-tests now execute and pass as "pending: FINAL" — `hack/check.sh:100`
  Failure: if one of these self-tests has quietly broken while it was being skipped, `task check` goes red for the first time in FINAL, outside this group's evidence.
  Fix: record the FINAL `task check` log lines before ticking 7.4.
  Confidence: 70

Checked and found sound:
- `need_json` correctly rejects empty, garbage, two-document and wrong-type output. `jq -s` on empty input gives `[]`, which fails `length == 1`, and the `$2` interpolations are constant literals.
- `need_json` sits after each `|| die`, so gh-error tests are unaffected. The M17 mutant was rewritten as the function `m17`, which is valid because `judge` runs `"$@"` in a subshell that inherits shell functions (`ci_failure_report_test.sh:190`).
- The five B1 mutants each remove one clause.
- The B2 list entry is held in both modes by the existing loop at `ci_workflow_security_test.sh:607-613`. The new mutant deletes both steps and checks that the deletion applied.
- In B3, the wiring assertion now also requires `.concurrency == null` at workflow level. The W4 mutant was retargeted and the B3 mutant added. zizmor still passes at `--min-severity=high`.
- The new EXIT trap in `task_check_test.sh:143` runs in check mode, which has no other trap (the self-test trap is at `:338`), so it overwrites nothing. All three sed mutants check that they applied.

## Could not check
- GitHub's runtime ordering of a job's `if:` versus job-level `concurrency`: whether a skipped job ever takes a slot in the group. B3's safety depends on this, and G7.md:177 only believes it. It can't be observed before merge.
- I ran no sensors (read-only leg). All exit codes and red/green outputs above come from G7.md and were not reproduced.
- `hack/ci/ci-failure-issue-body.sh`: whether it really exits 2 on an empty jobs file for the failure class (G7.md:73). Not read.
- The `lit`/`judge` mismatch handling for a chained `m17` whose first `lit` fails. I read only the dispatch at `:190`.
