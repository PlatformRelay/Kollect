# Spec Delta

## Purpose

Defines how a failing scheduled workflow becomes exactly one open, up-to-date issue, how
recovery is recorded, and how the reporter resists spoofing and stale events.

## ADDED Requirements

### Requirement: CFR-1 A failed scheduled run opens one issue

The `workflow_run` payload SHALL only trigger the reporter. The reporter SHALL act on the latest
completed scheduled run of the reported workflow
(`gh run list --workflow <workflow_run.path> --event schedule --status completed --limit 1`), with
that run's attempt, so an event dropped by the concurrency group or delivered late is harmless.
When that run is in the failure class of the decision table below and no bot issue for the
workflow is open, the reporter SHALL open one. The reporter SHALL judge the run by its conclusion
only: a job that fails under `continue-on-error` does not change the run's conclusion and is not
reported. Before the reporter runs, the workflow SHALL create the `ci-failure` label idempotently
(`gh label create ci-failure --force`).

Ordering key: `(run_number, run_attempt)` of the run acted on, compared lexicographically.
Recorded key: the key in the last bot marker (CFR-4) of the newest bot issue for the workflow,
open or closed. "None" means no such issue exists.

| Conclusion of the run acted on | Newest bot issue | Key vs recorded | Action |
| --- | --- | --- | --- |
| failure class: `failure`, `timed_out`, `startup_failure`, `cancelled` with a started job | none | - | open an issue (marker `event=failure`) |
| failure class | open | newer | one comment (marker `event=failure`) |
| failure class | open | equal or older | no write |
| failure class | closed | newer | open a new issue; never reopen |
| failure class | closed | equal or older | no write |
| `success` | none | - | no write |
| `success` | open | newer | recovery comment (marker `event=recovery`), then close |
| `success` | open | equal, last marker `event=recovery` | close only (retry of a failed close) |
| `success` | open | equal with last marker `event=failure`, or older | no write |
| `success` | closed | any | no write |
| no-op class: `cancelled` with no started job, `action_required`, `neutral`, `skipped`, `stale` | any | any | no write |

#### Scenario: First failure

- **WHEN** the nightly fails and no bot issue for it exists
- **THEN** one issue is created with the label, the marker and a body listing the failed jobs

#### Scenario: Label missing

- **WHEN** the repository has no `ci-failure` label and a reported run fails
- **THEN** the label is created and the issue is opened with it; when the label already exists the create step succeeds and changes nothing the reporter relies on

#### Scenario: Job failed under continue-on-error

- **WHEN** the nightly `race` job (`continue-on-error: true`) fails and every other job passes, so the run concludes `success`
- **THEN** the run is treated as a success; no issue is opened (known gap until SINK-BREAKER-RACE-01 makes the job blocking)

#### Scenario: Cancelled without a started job

- **WHEN** a run is `cancelled` and none of its jobs started (a dispatch replaced a pending run)
- **THEN** the reporter does nothing

#### Scenario: Timed out, startup failure or cancelled mid-run

- **WHEN** a run completes `timed_out` or `startup_failure`, or `cancelled` with started jobs
- **THEN** it is treated as a failure and never as success

#### Scenario: Other conclusions

- **WHEN** a run completes `action_required`, `neutral`, `skipped` or `stale`
- **THEN** the reporter makes no write

#### Scenario: Dropped or late event

- **WHEN** the concurrency group drops a pending event, or an event arrives after a newer scheduled run completed
- **THEN** the next report acts on the latest completed scheduled run, so the issue state follows that run

#### Scenario: Concurrent reports

- **WHEN** two reports run at the same time for one workflow
- **THEN** there SHALL NOT be two open issues; the reporter's concurrency group is `ci-failure-report-` plus `workflow_run.workflow_id`, with `cancel-in-progress: false`

### Requirement: CFR-2 Repeated failures update the same issue

When the newest bot issue for the workflow is open and the run acted on fails with a newer key,
the reporter SHALL add one comment instead of opening another issue. An equal key SHALL make no
write, so a re-run of the reporter does not duplicate the comment.

#### Scenario: Second red night

- **WHEN** the workflow fails on consecutive nights
- **THEN** the existing issue gets a comment with that run's link and failed jobs, and no new issue is created

#### Scenario: Reporter re-run

- **WHEN** the reporter runs twice for the same run and attempt
- **THEN** the second run makes no write

#### Scenario: Go lags for several weeks

- **WHEN** `Go patch lag` (weekly, `23 6 * * 1`) exits 1 on three Mondays in a row and passes on the fourth after a Go bump
- **THEN** one issue is opened on the first Monday, commented on the second and third, and closed on the fourth

### Requirement: CFR-3 Recovery is recorded, in order

When the run acted on succeeds and the newest bot issue for the workflow is open, the reporter
SHALL comment the recovery with the run's link and close the issue, provided the run's key is
newer than the recorded key. An equal key SHALL make no write, except that an equal key whose last
marker is `event=recovery` on a still-open issue SHALL retry the close. The recorded key SHALL be
read from the newest bot issue for the workflow whether it is open or closed.

#### Scenario: Success after failure after success

- **WHEN** night 1 passes, night 2 fails, night 3 passes
- **THEN** night 1 touches no issue, night 2 opens one, night 3 comments recovery and closes it

#### Scenario: Successful re-run of the same run number

- **WHEN** run N attempt 1 failed and attempt 2 succeeds
- **THEN** attempt 2 closes the issue

#### Scenario: Stale event

- **WHEN** the run acted on has an older `(run_number, run_attempt)` than the recorded key
- **THEN** it makes no write

#### Scenario: Stale failure after the issue closed

- **WHEN** the newest bot issue is closed with recorded key (N+1, 1) and the run acted on is (N, 2) with `failure`
- **THEN** it makes no write

#### Scenario: Close failed after the recovery comment

- **WHEN** the recovery comment was written, the close failed, and the reporter runs again for the same run
- **THEN** it closes the issue and writes no second comment

#### Scenario: Failure after recovery

- **WHEN** the newest bot issue is closed and a newer run fails
- **THEN** a new issue is opened; the closed one is not reopened

#### Scenario: Green with no issue

- **WHEN** a run passes and no bot issue for the workflow exists
- **THEN** nothing is created, commented or closed

### Requirement: CFR-4 The issue is found by label, bot author and workflow marker, never by title

The reporter SHALL find the newest bot issue for a workflow by label `ci-failure` AND bot
authorship AND a bot marker whose workflow key equals `workflow_run.path`, through the issue list
API (never `--search`), and SHALL NOT trust a title match. The bot marker is the first line of a
bot-authored issue body or comment:
`<!-- ci-failure-report v1 workflow=<path> run_id=<id> key=<run_number>.<run_attempt> event=<failure|recovery> -->`.
The reporter SHALL read markers only from the first line of bot-authored bodies and comments; a
marker anywhere else, or in text by any other author, SHALL be ignored.

#### Scenario: Spoofed issue

- **WHEN** a user opens an issue titled like the reporter's with the label
- **THEN** the reporter does not comment on or close it; it opens its own

#### Scenario: Spoofed marker in a user comment

- **WHEN** a user comments on the bot issue with a marker carrying a higher key
- **THEN** the marker is ignored and the next newer run is reported normally

#### Scenario: Both workflows have an open issue

- **WHEN** the `E2E nightly` and `Go patch lag` issues are both open and the nightly passes
- **THEN** only the nightly's issue gets the recovery comment and closes

### Requirement: CFR-5 The body names what failed and cannot execute or inject log text

`hack/ci/ci-failure-issue-body.sh` SHALL emit the marker as its first line, then the workflow,
the run URL, each failed or timed-out job by name and at most 50 `FAIL` lines, and SHALL keep the
whole body, marker included, under 60000 characters. It SHALL NOT evaluate or expand any text
taken from logs. It SHALL fence each excerpt with a backtick run longer than any backtick run
inside it, and SHALL neutralise `@` in run-supplied text outside a fence so that it cannot mention
a user. When the failed logs cannot be fetched, the body SHALL still list the failed jobs and say
"logs unavailable".

#### Scenario: Two failed jobs

- **WHEN** the recorded job list of a scheduled nightly (7 job definitions, 11 job entries once the 5-scenario matrix expands, `scale-envtest-10k` skipped) has two failed
- **THEN** the body lists exactly those two

#### Scenario: Huge log

- **WHEN** the failed log exceeds the cap
- **THEN** the body is truncated with a visible marker and still lists every failed job

#### Scenario: No FAIL lines

- **WHEN** a job failed with no `FAIL` line (a timeout)
- **THEN** the job is still listed with its failed step name

#### Scenario: Hostile log text

- **WHEN** a log line contains `$(touch pwned)` or backticks
- **THEN** it appears as text and nothing runs

#### Scenario: Markdown injection

- **WHEN** a log line contains a run of backticks, `@user` and a marker lookalike
- **THEN** the excerpt fence stays closed around them, no `@user` mention appears outside a fence, and the only marker on the body's first line is the real one

### Requirement: CFR-6 Only scheduled runs of this repository report

The reporter SHALL act only when `workflow_run.event == 'schedule'` and
`workflow_run.head_repository.full_name == github.repository`, SHALL NOT check out or run code from
the triggering run, and SHALL surface its own errors as its own failed run. Every `gh` call SHALL
take its values as quoted arguments and bodies through `--body-file`, and SHALL use a numeric id
only after checking that it is numeric. Its own failures are visible only in the Actions tab.

#### Scenario: Dispatch, fork or non-schedule event

- **WHEN** the event is `workflow_dispatch`, comes from another repository, or is not `schedule`
- **THEN** the reporter does nothing

#### Scenario: Reporting breaks

- **WHEN** any `gh` call other than the log fetch errors
- **THEN** the reporter exits non-zero with the `gh` error on stderr and makes no write after the failing call; the reported run's conclusion is unaffected

#### Scenario: Log fetch fails

- **WHEN** fetching the failed logs errors or returns nothing (logs expired)
- **THEN** the reporter still writes the issue or comment with the job names and "logs unavailable", then exits non-zero

### Requirement: CFR-7 The decision logic is tested and the workflow satisfies the zizmor gate

`hack/test/ci_failure_report_test.sh` SHALL cover every row of CFR-1's decision table and the body
builder on recorded fixtures with a fake `gh`, with mutants, and SHALL run in both modes (plain
and `--self-test`) in the required `lint` job on `pull_request`, asserting that wiring itself. It
SHALL assert the workflow's wiring: every name under `on.workflow_run.workflows` equals the `name:`
of exactly one file under `.github/workflows/`, the event and repository guards, the concurrency
group, the permissions (job: exactly `contents: read`, `actions: read`, `issues: write`), no
checkout of the triggering run, and the label-create step before the report step. The workflow's
`dangerous-triggers` suppression SHALL live in `.github/zizmor.yml`, with a reason comment above
the rule key and another above the ignore entry.

#### Scenario: Decision row removed

- **WHEN** the recovery branch, the ordering check, the label-and-author lookup or the workflow filter is removed (mutants)
- **THEN** the test SHALL fail

#### Scenario: Reported workflow renamed

- **WHEN** a listed workflow's `name:` changes (for example `E2E nightly` to `E2E Nightly`) and the list does not
- **THEN** the test SHALL fail naming the listed name that matches no workflow file, because `workflow_run` matches by name and would stop firing silently

#### Scenario: Label step missing or late

- **WHEN** the `gh label create ci-failure --force` step is removed or moved after the report step
- **THEN** the test SHALL fail

#### Scenario: Test unwired

- **WHEN** both `lint` steps that run the test are deleted
- **THEN** the test's own wiring assertion SHALL fail

#### Scenario: Suppression without reason

- **WHEN** the comment above the `dangerous-triggers` rule key, or the comment above its `- ci-failure-report.yaml` ignore entry, is removed from `.github/zizmor.yml`
- **THEN** the existing CWS-2 rule (`cws2_suppressions`, `hack/test/ci_workflow_security_test.sh:236`) SHALL fail; an inline `# zizmor: ignore[dangerous-triggers]` fails the same rule (`:303-308`)
