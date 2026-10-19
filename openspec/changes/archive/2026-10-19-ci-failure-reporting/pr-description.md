# PR description — ci-failure-reporting

**Create at:** <https://github.com/platformrelay/kollect/compare/main...feat/ci-failure-reporting?expand=1>

- **Source:** `feat/ci-failure-reporting`
- **Target:** `main`
- **Title:** `:sparkles: feat(ci): report failing scheduled workflows as deduplicated issues`
- **Opens as:** ready

Paste everything below the line.

---

Closes #444

## TL;DR

- **What:** a new `workflow_run` workflow, `ci-failure-report.yaml`, keeps one `ci-failure` issue
  per scheduled workflow (`E2E nightly`, `Go patch lag`): it opens the issue on the first red
  scheduled run, comments on later red runs, and comments and closes on the first green one.
- **Why:** a red nightly or weekly run is visible only to someone who opens the Actions tab;
  nobody gets told.
- **Risk:** `workflow_run` is a privileged trigger with `issues: write`; it is guarded (schedule
  event, same repository, default-branch code only) and zizmor's `dangerous-triggers` is
  suppressed for this one file.
- **Reviewer action:** check the security model of `.github/workflows/ci-failure-report.yaml` and
  the zizmor exception; approving also accepts `task check` at ~30 min and the lint job at ~16 min.

The reporter acts on the latest completed scheduled run of the workflow that triggered it and
decides `open`, `comment`, `close` or `none` from that run's conclusion and the run key stored in
the newest bot issue for that workflow. The branch also fixes a silent skip in `task check`: the
`--self-test` of five large guards never ran.

## Most important things

- **What it reports, and what it does not.** It reads the run's conclusion only, never a
  single job's. The nightly `race` job runs with `continue-on-error`, so a race failure leaves the
  run green and is **not reported** until SINK-BREAKER-RACE-01 makes that job blocking (a
  documented non-goal; the workflow header says so). Only `schedule` events count: a manual
  `workflow_dispatch` of a reported workflow is ignored by design. The reporter's own failures are
  visible **only** in this workflow's runs in the Actions tab; nothing reports the reporter.
- **Security model.** `workflow_run` runs with the base repository's token. The job runs only
  when `workflow_run.event == 'schedule'` and the head repository is this repository. The checkout
  has no `ref:` or `repository:`, so it is always the default branch, never the triggering run's
  code, and keeps no credentials. No `run:` step interpolates event data: the reporter reads the
  payload from `GITHUB_EVENT_PATH` with `jq` and gets everything else from `env:`. Log excerpts go
  into issue bodies through `--body-file`, built by `hack/ci/ci-failure-issue-body.sh`, which caps
  size and neutralises mentions; nothing from a log is evaluated. Permissions: workflow
  `contents: read`, job adds `actions: read` and `issues: write`.
- **zizmor file exception.** `.github/zizmor.yml` goes from `rules: {}` to one
  `dangerous-triggers` ignore for `ci-failure-report.yaml` only, with the reasons in a comment.
  Any other `workflow_run` workflow is still a finding.
- **Deduplication.** Job-level concurrency per reported workflow, queued and never cancelled, so
  two reports cannot both open an issue. It sits after the job's `if:`, so a skipped
  (non-scheduled) run cannot displace a pending scheduled report.
- **`task check` self-test fix.** `hack/check.sh` detected `--self-test` support with
  `grep -v | grep -q` under `pipefail`; on guards larger than the pipe buffer the upstream grep died
  of SIGPIPE and the pipeline read as "no", so five guards' self-tests were silently skipped
  (`ci_failure_report`, `ci_workflow_security`, `consistency_toolchain_pins` always;
  `consistency_go_toolchain` and `consistency_tool_pins` intermittently). They now run, and pass.
  Cost: `task check` goes from ~18 to **~30 min** locally. `task_check_test.sh` pins the fix with a
  1.6 MB guard.
- **Lint job ~12 → ~16 min.** `ci.yaml`'s lint job gains the reporter test, plain and
  `--self-test` (~3.5 min). Still below `tool-pins-selftest`, the slowest required job.
- **Scope decision (operator-accepted, 2026-10-16, decision A):** report the run conclusion only,
  keep `Go patch lag` in the reported set (one open issue and a comment per red Monday while Go
  lags), and let the workflow create the `ci-failure` label itself (`gh label create --force`)
  instead of depending on an operator setting.

## Risk

- **What can break:** a reporter bug opens duplicate issues or comments, or closes an issue
  early; a `gh` output shape change makes the reporter exit 1 (it refuses empty or malformed
  `gh` output rather than writing). A future edit that adds `ref:` to the checkout or
  interpolates event data in `run:` would reopen the privileged-trigger hole.
- **Blast radius:** issues and the `ci-failure` label in this repository only. No product code,
  no release path, no required check changes behaviour except the lint job's duration.
- **Rollback:** revert the PR, or disable the `CI failure report` workflow in the Actions tab.
  Issues it opened stay and can be closed by hand.
- **Residual risk accepted:** race failures unreported until SINK-BREAKER-RACE-01; reporter
  failures visible only in the Actions tab; the job-level concurrency's skip-before-queue
  ordering is documented GitHub behaviour, not something observable before merge.

## Files to look at first

- `.github/workflows/ci-failure-report.yaml` — the privileged workflow: three steps, the `if:`
  guard, job-level concurrency, checkout without a ref.
- `.github/zizmor.yml` — the commented `dangerous-triggers` file exception.
- `hack/ci/ci-failure-report.sh`, `hack/ci/ci-failure-issue-body.sh` — the reporter's decision
  table and the hardened issue body builder.
- `hack/check.sh` — `declares_self_test`: `task check` now really runs the `--self-test` of five
  large guards.
- `hack/test/ci_workflow_security_test.sh` — the CFC-6 list gains `ci_failure_report_test.sh`;
  the CWS-2 self-test mutants are re-anchored on `rules:` (a test change, same rule and message).
- `.github/workflows/ci.yaml` — the lint job gains the reporter test steps.

**Review this commit-by-commit, not as one flattened diff.** Each `ci:` and `test:` commit is one
task of the OpenSpec change with its own evidence under
`openspec/changes/ci-failure-reporting/evidence/`; most of the flattened diff is recorded `gh`
fixtures and review evidence.

## Testing protocol

- [x] `bash hack/test/ci_failure_report_test.sh` and `--self-test` exit 0.
- [x] `task check` on `ebe224388`: every gate and guard green, the five self-tests now run
  (`check: guard … --self-test ok`); one ENVIRONMENT red (`demo_04`, mise kubectl shim unset in
  the worktree), green on rerun with a real kubectl.
- [x] `zizmor --offline --no-progress --min-severity=high --config .github/zizmor.yml .github/`:
  no findings.
- [ ] Every check on the PR head green, then rebase-merge.
- [ ] After merge, wait for the next scheduled `E2E nightly` (cron `0 3 * * *` UTC). Then
  `gh run list --workflow ci-failure-report.yaml --branch main --limit 5` shows a run whose
  `report` job ran (not skipped) and concluded `success`. Failure signature: the job is `failure`,
  or it is skipped for a `schedule` run.
- [ ] If that nightly was red: `gh issue list --label ci-failure --state open` shows exactly one
  issue for `E2E nightly`. If green: no new issue, and the `report` step log ends in `action none` or `nothing to do`.
- [ ] Do not test with `gh workflow run e2e-nightly.yaml`: a dispatch is not a `schedule` event,
  and the reporter skips it by design.

---

## Optional reading — the long version

*Nothing above depends on this. Read it if you want the full story, the road not taken, or the
review trail — skip it if the top half already told you what you needed.*

**Why a separate workflow.** A job inside the reported run has no run conclusion yet, so the
reporter has to run after the run completes: `workflow_run: completed` is the only trigger that
gives that.

**Why the latest run, not every missed run.** The payload is treated as a trigger only. If two
nightlies complete while a report is queued, the reporter acts on the newest completed scheduled
run; the ordering key `(run_number, run_attempt)` stored in the bot marker keeps a rerun or a late
event from reopening or re-commenting. Full reconciliation of every missed run was rejected: the
latest state is what is worth reporting.

**Issue lookup.** The reporter lists issues authored by `github-actions[bot]` and filters the
`ci-failure` label client-side, because `gh issue list --label` routes to the search API, which the
reporter never uses (probe F1 in `evidence/T1.1.md`). The lookup ceiling (500) is shared with every issue the bot
opens; recorded as a harness gap in `evidence/G3.md`.

**Review trail.** Spec set: two rounds by two independent reviewers, no CRITICAL left. Each task
group: a diff review, plus a second one wherever CI wiring or the privileged workflow was touched.
Branch review: REQUEST_CHANGES with two WARNINGs (empty or malformed `gh` output with exit 0 could
duplicate an issue; deleting both lint steps went undetected outside the test), both fixed as
tasks 7.1 and 7.2; the `task check` SIGPIPE skip was found by the holistic run at that stage and
fixed as 7.4. Everything is under `openspec/changes/ci-failure-reporting/reviews/`.

**Test changes** (tests whose expectation changed, each with evidence): the CWS-2 self-test
mutants re-anchored from the literal `rules: {}`; the CFC-6 guard list gains one entry and one
mutant; one reporter self-test mutant (M17) made non-equivalent again; the concurrency wiring
assertion follows the move to job level. Details in `loop.md` *Test changes*.

**Post-merge.** The archive of this OpenSpec change waits for the first real reporter run after
a scheduled E2E nightly.
