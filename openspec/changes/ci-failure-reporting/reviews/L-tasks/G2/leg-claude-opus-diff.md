## Verdict: CONCERNS

## Findings
- [WARNING] The test header says no marker lookalike can appear outside a fence "through log text or job names", but only the log-text half is tested — `hack/test/ci_failure_report_test.sh:17`, `hack/ci/ci-failure-issue-body.sh:80-81`
  Failure: the `&`, `<` and `>` escaping and the `[[:cntrl:]]` replacement in `safe` are not covered by any case or mutant (M1–M13 in G2.md). Delete `gsub("<"; "&lt;")`, or the control-character `gsub`, and all 15 checks still pass. A job or step name `x <!-- ci-failure-report v1 … -->`, or a name containing `\n<!--`, would then put a second marker outside the fence. Today job names come from the workflow file, which the repo controls, so this is a gap in the tests and not an exploit.
  Fix: in case 8, also rename one job to a marker lookalike containing a newline. Assert that the outside-fence marker count is 1 and that no line outside the fence starts with `<!--` except line 1.
  Confidence: 85
- [NOTE] The UTF-8-safe 4000-byte cut has no test, and only ASCII lines are cut — `hack/ci/ci-failure-issue-body.sh:137-141`
  Failure: the `sed 's/[\xC0-\xFF][\x80-\xBF]*$//'` step can be removed, or broken by a sed without `\xHH` support, and nothing notices. Case 4 cuts only `x` padding. The sed also drops a *complete* trailing multibyte character, which does no harm.
  Fix: in case 4, pad one line with `é` so that byte 4000 falls in the middle of the character, and assert that the excerpt is valid UTF-8 (`iconv -f UTF-8 -t UTF-8`).
  Confidence: 70
- [NOTE] A log file that exists but cannot be read yields "No line containing FAIL" instead of "logs unavailable" — `hack/ci/ci-failure-issue-body.sh:125-131`
  Failure: `-s` is true, `grep` exits 2, `|| true` hides the error, and the body makes a false negative claim about the log.
  Fix: test `-r` alongside `-s` at line 125.
  Confidence: 60
- [NOTE] The truncation note says "60000 characters", but the cap is counted in bytes — `hack/ci/ci-failure-issue-body.sh:117`
  Fix: write "bytes", or "under 60000 bytes (GitHub limit 65536 characters)".
  Confidence: 90
- [NOTE] The die path for a job list that does not fit the cap is untested. G2.md:53 admits this and marks it **believed** — `hack/ci/ci-failure-issue-body.sh:119-120`
  Confidence: 95

## Checked and holding
- **Fence length:** the `awk` pass measures the longest run over all cut lines. The fence is that length + 1, minimum 3, and at most 4001 because runs are ≤ 4000 after the cut. The ` [line cut]` suffix adds no backticks. Mutant M8 checks this directly through the case-7 assertion `open_len > 5`.
- **`@` neutralised outside the fence:** it is escaped after `&`, so the entity it adds is not escaped again. Marker inputs are regex-validated and contain no `@`, backtick, `<` or newline. `RUN_URL` and `WF_PATH` (in a code span) cannot carry backticks or `@`.
- **50-line bound:** `grep -a -m 50 -F FAIL` (M3).
- **Cap arithmetic:** total = head + 2·(fence+1) + excerpt + note + 2 ≤ 59999 bytes. The precheck `head + note + 3·4016 < 60000` leaves a budget of at least ~4041 bytes, enough for one maximal 4012-byte line, so the excerpt never comes out empty. The fence is printed before the note (M4, M5). `LC_ALL=C` is exported before any `${#…}`, so lengths are in bytes.
- **No evaluation:** log text passes only through `printf '%s'`, `grep`, `awk` and `sed` as data. M7 proves case 6 can go red.
- **Jobs:** with the `workflow_run` trigger the run is complete, so no in-progress `""` conclusions reach the builder. Cancelled jobs with `steps: []` give "no failed step recorded". `first` of an empty array returns null, and that case is handled.
- **Vacuity:** the stub red run fails each case on its own assertion (G2.md:84-99), and 13/13 hand mutants are killed. The pipelines are `! a | grep -q` under `pipefail`, and the bodies are smaller than the pipe buffer, so they are not flaky in practice.
- **CI wiring:** one plain-mode step has been added. CWS-6 is recorded as passing.

## Could not check
- I could not run the test, the mutant runner `/tmp/g2cfr/mutants.sh` or ShellCheck (read-only, git-only Bash). I relied on the sensor results recorded in G2.md.
- I did not read the fixture JSON files under `hack/test/testdata/ci-failure-report/`, the full CFR-4 lookup logic (whether it reads only line 1 or the whole body), or the context of the `lint` job around `ci.yaml:573`.
- I did not check how GitHub actually renders `@&#8203;user`; I took the zero-width-space approach from the design.
