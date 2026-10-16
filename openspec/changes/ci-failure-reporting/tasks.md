# Tasks

Red first. Every test task comes before the implementation it drives and records its red in
evidence, failing on the target assertion (not on a missing file, a parse error or a usage error).
`task check` runs every `hack/test/*_test.sh` (`hack/check.sh:79`), so a red test cannot be
committed alone: each test task closes in one commit with the implementation task after it (2.1
with 2.2, 3.1 with 3.2, 4.1 with 4.2), and the tree is green at every commit. The test's plain
mode joins the required `lint` job (`ci.yaml:279`) in the first of those commits; its
`--self-test` mode joins in 5.1, when it exists, so CWS-6 sees every mode it recognises wired.

Fixtures live under `hack/test/testdata/ci-failure-report/`. Evidence under
`openspec/changes/ci-failure-reporting/evidence/` is Markdown only
(`hack/test/openspec_evidence_markdown_only_test.sh`).

## 1. Probe

- [ ] 1.1 Record fixtures and check the assumptions in proposal.md, each with its source:
  - a `workflow_run` payload (GitHub docs example, trimmed to the fields CFR-1 to CFR-6 read);
  - `gh run view --json jobs` for scheduled nightly run 38871014363 (11 job entries, all
    `success`, `scale-envtest-10k` `skipped`), edited into a two-failed copy and a timed-out copy;
  - a cancelled-before-start run (`gh run list --status cancelled` on any workflow), or the
    documented shape if none exists, marked as such;
  - `gh run view --log-failed` on an in-progress run (expected: refused or empty);
  - the job conclusion and run conclusion of a failed `continue-on-error` job, from any repository
    run, to confirm the run stays `success`;
  - `gh label create <scratch> --force` twice on a scratch label in a fork or a throwaway repo,
    never on this repository's labels.
  If a payload field CFR-6 relies on is absent, stop: the change is BLOCKED. Evidence:
  `evidence/T1.1.md`.

## 2. Body builder (CFR-5)

- [ ] 2.1 Write the body-builder cases in `hack/test/ci_failure_report_test.sh` and record them
  red against a stub `hack/ci/ci-failure-issue-body.sh` that prints nothing:
  - given the nightly job list with two failed entries, when the body is built, then it lists
    exactly those two by name (exposes a builder that lists all jobs, or only the first);
  - given a failed log over the cap, when the body is built, then it is under 60000 characters,
    ends the log part with a visible truncation marker and still lists every failed job (exposes
    truncation that drops jobs, or a body GitHub rejects at 65536);
  - given a timed-out job with no `FAIL` line, when the body is built, then the job is listed with
    its failed step name (exposes a builder keyed on `FAIL` lines only);
  - given a log line `$(touch pwned)` and one with backticks, when the body is built in a scratch
    directory, then both appear verbatim and no `pwned` file exists (exposes `eval` or an unquoted
    expansion of log text);
  - given a job list with no failed job, when the body is built, then it says so and lists none
    (exposes a builder that invents a failure).
- [ ] 2.2 Implement `hack/ci/ci-failure-issue-body.sh`; 2.1 green; plain mode wired into `lint`.
  One commit with 2.1.

## 3. Decision logic (CFR-1 to CFR-4, CFR-6)

- [ ] 3.1 Write the decision-table cases with a fake `gh` on `PATH` that records every call, and
  record them red against a stub `hack/ci/ci-failure-report.sh` that exits 0 and does nothing.
  Each case asserts the exact recorded writes:
  - given a `failure` and no open issue, then one `issue create` with label `ci-failure` (CFR-1;
    exposes no report at all);
  - given `failure` and an open bot issue for that workflow, then one comment and no create
    (CFR-2; exposes a duplicate issue per night);
  - given `success` and an open bot issue with an older recorded key, then a recovery comment and
    a close (CFR-3; exposes an issue that never closes);
  - given `success` and no open issue, then no write (CFR-3; exposes noise on every green night);
  - given `timed_out`, and `cancelled` with a started job, then treated as `failure` (CFR-1;
    exposes a timeout read as green);
  - given `cancelled` with no started job, then no write (CFR-1; exposes an issue for a replaced
    pending run);
  - given a run whose conclusion is `success` while one job failed under `continue-on-error`, then
    no write (CFR-1, decision 2026-10-16 A; exposes job-level reporting this change excludes);
  - given event `workflow_dispatch`, or `push`, or a head repository other than this one, then no
    write (CFR-6; exposes reports from manual or fork runs);
  - given an event older by `(run_number, run_attempt)` than the key recorded on the open issue,
    then no write (CFR-3; exposes a late event closing or reopening out of order);
  - given run N attempt 1 recorded as failed and run N attempt 2 `success`, then close (CFR-3;
    exposes ordering on `run_number` alone);
  - given only a user-authored open issue with the label and the reporter's title, then a new bot
    issue and no write to the user's (CFR-4; exposes a title or label-only lookup);
  - given a closed bot issue and a new `failure`, then a new issue and no reopen (CFR-3);
  - given `gh` exiting non-zero on the lookup, then the script exits non-zero with the `gh` error
    on stderr (CFR-6; exposes a swallowed error that leaves the run green);
  - sequence pass, fail, fail, pass, fail on one fake issue store: none, create, comment, close,
    create;
  - sequence for `Go patch lag`: fail, fail, fail, pass: create, comment, comment, close.
- [ ] 3.2 Implement `hack/ci/ci-failure-report.sh`; 3.1 green. One commit with 3.1.

## 4. Workflow (CFR-1, CFR-6, CFR-7)

- [ ] 4.1 Write the wiring and guard assertions on `.github/workflows/ci-failure-report.yaml` and
  record them red on today's tree (the file is absent, so each assertion must name what it found
  missing, not stop at the first):
  - every name under `on.workflow_run.workflows` equals the `name:` of exactly one file under
    `.github/workflows/`, and the list is exactly `E2E nightly` (`e2e-nightly.yaml:2`) and
    `Go patch lag` (`go-patch-lag.yaml:6`) (exposes a rename that stops `workflow_run` firing);
  - `types: [completed]` only;
  - the job's `if:` requires `github.event.workflow_run.event == 'schedule'` and
    `github.event.workflow_run.head_repository.full_name == github.repository` (CFR-6);
  - concurrency group keyed on the reported workflow, `cancel-in-progress: false` (CFR-1);
  - workflow `permissions: contents: read`; job `issues: write` and `actions: read`, nothing else;
  - every `actions/checkout` sets `persist-credentials: false` and no `ref:` or `repository:`
    (exposes a checkout of the triggering run);
  - a step running `gh label create ci-failure --force` precedes the step that runs
    `hack/ci/ci-failure-report.sh` (exposes a first report failing on a missing label);
  - no step's `run:` interpolates `github.event.workflow_run.*` directly; values reach scripts
    through `env:` (exposes template injection through a branch or title);
  - a comment on the workflow list names the `race` gap and SINK-BREAKER-RACE-01.
- [ ] 4.2 Add `.github/workflows/ci-failure-report.yaml` and the `dangerous-triggers` rule in
  `.github/zizmor.yml`, block style, scoped to this file, with its reason in a comment directly
  above it (no checkout of the triggering run, no log text evaluated, same-repo and schedule
  guard), and update the config's header comment (`.github/zizmor.yml:10-14`). Then: 4.1 green;
  zizmor at the gate's command (`zizmor --offline --no-progress --min-severity=high --config
  .github/zizmor.yml .github/`) has no findings; `hack/test/ci_workflow_security_test.sh` green in
  both modes, CWS-2 (`:236`) included; `hack/test/sonar_ko_08_workflow_permissions_test.sh`
  green. One commit with 4.1.

## 5. Proof and wiring (CFR-7)

- [ ] 5.1 Add `--self-test` to `hack/test/ci_failure_report_test.sh`, mutating copies, each judged
  by exit status and the message of the assertion it targets, plus a no-op control:
  - recovery branch removed: the pass-after-fail case reds;
  - label-and-author lookup reduced to title or label: the spoofed-issue case reds;
  - ordering check removed: the stale-event case reds;
  - event or repository guard removed (script and workflow `if:` separately): the dispatch and
    fork cases red;
  - conclusion widened to job-level failures: the `continue-on-error` case reds;
  - `name: E2E nightly` changed in a copy of `e2e-nightly.yaml`: the wiring check reds naming
    `E2E nightly`;
  - label-create step removed, and moved after the report step: the wiring check reds;
  - `ref: ${{ github.event.workflow_run.head_sha }}` added to the checkout: the wiring check reds;
  - `dangerous-triggers` comment removed in a copy of `.github/zizmor.yml`: CWS-2 reds (proves
    CFR-7's suppression scenario against the existing rule, not a new one).
  Wire `--self-test` into `lint` next to the plain step. Grep `hack/test/` for step-index or
  step-count assertions on `ci.yaml`'s `lint` job and update any that the two new steps move.
  `hack/test/ci_workflow_security_test.sh` both modes and `task lint:shell` green.

## 6. Land

- [ ] 6.1 Independent review; PR with green CI on the head SHA; rebase merge with the `post-merge`
  rows open; archive in a follow-up evidence PR once they pass.

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| CFR-1 | `ci_failure_report_test.sh` fail+none, timed_out, cancelled cases, `continue-on-error` case; label-create step present and first; concurrency group | one issue; never two; unstarted cancel and job-level failure no-op | not-run | |
| CFR-2 | same, fail+open; Go patch lag sequence | comment only | not-run | |
| CFR-3 | pass-fail-pass sequence; re-run attempt; stale event; fail after close | recovery closes; stale writes nothing | not-run | |
| CFR-4 | spoofed-title case | not touched | not-run | |
| CFR-5 | body fixtures incl. hostile text | listed, capped, inert | not-run | |
| CFR-6 | dispatch, fork, non-schedule, `gh` error; workflow `if:` and checkout assertions | no action; error visible | not-run | |
| CFR-7 | mutants judged by exit status plus no-op; rename mutant; zizmor gate on the workflow; CWS-2 on the suppression | each mutant red; gate green | not-run | |
| all | `gh label list` after the first scheduled run following the merge | `ci-failure` exists | post-merge | follow-up evidence PR |
| all | first real scheduled night(s) and Monday(s) after merge | issue opens on a failure, closes on recovery; a green run makes no write | post-merge | follow-up evidence PR |
| all | independent review; CI on the PR head | APPROVE, green; reviewed and archive revisions recorded | not-run | |
