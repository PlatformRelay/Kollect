## Verdict: CLEAN

## Findings
I checked every requirement against the code itself (`hack/ci/ci-failure-report.sh`, `hack/ci/ci-failure-issue-body.sh`, `.github/workflows/ci-failure-report.yaml`, `.github/zizmor.yml`, `ci.yaml` lint steps, CWS-2/CWS-6 in `ci_workflow_security_test.sh`, and the test's cases and mutants), not against the evidence files.

**Compliance table**
- CFR-1 **holds.**
  - Event as trigger only, latest completed scheduled run at its attempt: `ci-failure-report.sh:75-88`.
  - Empty run list exits 0 with no write: `:77-80`.
  - Class from the conclusion only, with cancelled split by whether any job has non-empty `steps`: `:93-107`.
  - The 11 table rows: `:164-177`. I traced each row and all match, including closed+newer (open, never reopen) and the equal-key recovery retry (close only).
  - Label step uses `--force` with a fixed colour and description, before the report step: `ci-failure-report.yaml:55-59`.
  - Concurrency group and `cancel-in-progress: false`: `:33-35`.
- CFR-2 **holds.** Open+newer comments, open+equal makes no write: `:168`. The recorded key comes from the last bot comment: `:144`.
- CFR-3 **holds.** Recovery comment then close: `:204-208`. Close retry: `:174-175`. The key is read whether the issue is open or closed: `:131-149`.
- CFR-4 **holds.**
  - Lookup lists by author only, with `--state all` and `--limit 500`, and stops when the result count reaches the limit: `:114-118`.
  - `is_bot`, login and label are checked client-side: `:126-127`.
  - Markers are read only from the first line and must match the workflow path: `:121-124`.
  - Comments count only when the author login is `github-actions`: `:144`.
- CFR-5 **holds.**
  - Marker is line 1: `ci-failure-issue-body.sh:98`.
  - At most 50 FAIL lines: `:132`.
  - Byte-counted cap under 60000 with a visible truncation note: `:163-178`.
  - Fence is one backtick longer than the longest run inside: `:156-160`.
  - `@`, `<` and `&` are escaped outside the fence: `:81-82`.
  - "logs unavailable" body still lists the jobs: `:124-136`.
- CFR-6 **holds.**
  - Guard in the workflow `if:` (`ci-failure-report.yaml:40`) and again in the script (`ci-failure-report.sh:60-65`).
  - Checkout is the default branch only: `yaml:49-51`.
  - Every body goes through `--body-file`; ids are checked as integers before use (`need_int`, `:85-87`, `:137`).
  - A gh error stops the script (`die`). A failed or empty log fetch still writes, then exits 1: `:186-193`, `:215`.
- CFR-7 **holds**, with one small gap (see NOTE 3).
  - Every table row has a case, and every named mutant exists: recovery branch removed (`ci_failure_report_test.sh:318`), ordering check removed (`:327`), lookup reduced to the label (`:320`), workflow filter dropped (`:325`), workflow renamed (`:392`), label step removed or moved after the report step (`:416-421`), test unwired (`:466`), CWS-2 comments removed (`:454-457`).
  - Workflow permissions are pinned exactly: `:1549-1560`.
  - The `zizmor.yml` comments sit above both the rule key and the ignore entry: `zizmor.yml:16,20`.

**What the code does that no requirement asks for** (each is stronger than the spec, but none breaks a legitimate caller today)
- A conclusion outside the table makes the script exit 1: `ci-failure-report.sh:98`.
- A workflow path that does not match `.github/workflows/[A-Za-z0-9._-]+.ya?ml` makes the script exit 1. A future workflow file with a space or other character in its name would fail closed: `:67`.
- The issue title is cut at 200 bytes: `:68-69`.
- The test enforces more than the spec lists: `workflow_run` as the only trigger, exactly three steps, no `${{ }}` in any `run:` body (comment lines included), a race-gap comment above the workflow list, and a `zizmor.yml` ignore list of exactly one entry: `test:1508-1665`.
- The builder lists cancelled jobs as well as failed and timed-out ones: `body.sh:89`.
- `gh label create --force` resets any manual edits to the label: `yaml:54`.

**Notes**
- [NOTE] A job list too large for the cap gets no issue at all, only a failed reporter run: `ci-failure-issue-body.sh:121-122`
  Failure: a workflow with roughly 250 or more failed jobs with long names → the builder exits 2 → the reporter stops at `ci-failure-report.sh:196`. The failure then shows only in the Actions tab, even though CFR-5 says every failed job is still listed. This can't happen with today's 11 jobs.
  Fix: none needed now. If it is ever reachable, cap the job list with a visible "and N more" line.
  Confidence: 70
- [NOTE] The concurrency group is set at workflow level, so non-schedule completions also queue in it: `ci-failure-report.yaml:33-35`
  Failure: a dispatched E2E nightly that finishes while a scheduled report is queued in the same group can replace that pending schedule event. The dispatched run's job is then skipped by its `if:`, so the scheduled result is not reported until the next scheduled completion. For Go patch lag that is a week. The spec accepts dropped events, but this adds a way to drop them.
  Fix: move `concurrency:` to the `report` job, where it applies only after the `if:` passes.
  Confidence: 45
- [NOTE] The self-wiring check pins "some line of `ci.yaml`", not the `lint` job that CFR-7 names: `ci_failure_report_test.sh:1674-1686`
  Failure: moving both steps to another required job still passes. CWS-6's general loop (`ci_workflow_security_test.sh:588-594`) does require a required job on `pull_request`, so the check is weaker than the spec's wording but no safety guarantee is lost.
  Fix: none, or record that "a required job" is the intended reading.
  Confidence: 60

## Could not check
- I did not run either test mode (`ci_failure_report_test.sh` plain or `--self-test`), CWS-2, or zizmor. No sensor results were read.
- Real `gh` behaviour I took from the T1.1 evidence:
  - that `--author` uses the list API rather than search;
  - that `gh issue view --json comments` returns every comment, beyond 100;
  - that `--workflow <path>` is accepted;
  - the issue author login `app/github-actions` and comment author login `github-actions`.
- I did not look at the many other openspec changes and workflow edits in this range (toolchain pins, renovate, consistency gates). They are outside the CFR spec.
- The `jobs-timed-out.json` fixture was edited by hand; T1.1 F7 says its exact live shape was never confirmed.
