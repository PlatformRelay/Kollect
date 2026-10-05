# Spec Delta

## Purpose

Defines how a failing scheduled workflow becomes exactly one open, up-to-date issue, how
recovery is recorded, and how the reporter resists spoofing and stale events.

## ADDED Requirements

### Requirement: CFR-1 A failed scheduled run opens one issue

When a reported workflow completes with conclusion `failure` or `timed_out`, or with `cancelled`
after at least one job started, and no open issue with label `ci-failure` authored by the
workflow's bot exists for that workflow, the reporter SHALL open one.

#### Scenario: First failure

- **WHEN** the nightly fails and no matching open issue exists
- **THEN** one issue is created with the label and a body listing the failed jobs

#### Scenario: Cancelled without a started job

- **WHEN** a run is `cancelled` and none of its jobs started (a dispatch replaced a pending run)
- **THEN** the reporter does nothing

#### Scenario: Timed out or cancelled mid-run

- **WHEN** a run completes `timed_out`, or `cancelled` with started jobs
- **THEN** it is treated as a failure and never as success

#### Scenario: Concurrent reports

- **WHEN** two reports run at the same time for one workflow
- **THEN** there SHALL NOT be two open issues; the reporter's concurrency group is per reported workflow with `cancel-in-progress: false`

### Requirement: CFR-2 Repeated failures update the same issue

When a matching open issue exists and a run fails, the reporter SHALL add one comment instead of
opening another issue.

#### Scenario: Second red night

- **WHEN** the workflow fails on consecutive nights
- **THEN** the existing issue gets a comment with that run's link and failed jobs, and no new issue is created

### Requirement: CFR-3 Recovery is recorded, in order

When a run succeeds and a matching open issue exists, the reporter SHALL comment the recovery with
the run's link and close the issue, provided the run is newer than the last recorded event by
`(run_number, run_attempt)`.

#### Scenario: Success after failure after success

- **WHEN** night 1 passes, night 2 fails, night 3 passes
- **THEN** night 1 touches no issue, night 2 opens one, night 3 comments recovery and closes it

#### Scenario: Successful re-run of the same run number

- **WHEN** run N attempt 1 failed and attempt 2 succeeds
- **THEN** attempt 2 closes the issue

#### Scenario: Stale event

- **WHEN** an event for an older `(run_number, run_attempt)` than the last recorded one is delivered late
- **THEN** it makes no write

#### Scenario: Failure after recovery

- **WHEN** a closed issue exists and the next run fails
- **THEN** a new issue is opened; the closed one is not reopened

#### Scenario: Green with no open issue

- **WHEN** a run passes and no matching open issue exists
- **THEN** nothing is created, commented or closed

### Requirement: CFR-4 The issue is found by label and bot author, never by title

The reporter SHALL find the open issue by label `ci-failure` AND the bot's authorship AND the
workflow's key, and SHALL NOT trust a title match alone.

#### Scenario: Spoofed issue

- **WHEN** a user opens an issue titled like the reporter's with the label
- **THEN** the reporter does not comment on or close it; it opens its own

### Requirement: CFR-5 The body names what failed and cannot execute log text

`hack/ci/ci-failure-issue-body.sh` SHALL emit the workflow, run URL, each failed or timed-out job
by name, a bounded number of `FAIL` lines, and SHALL keep the result under 60000 characters. It
SHALL NOT evaluate or expand any text taken from logs.

#### Scenario: Two failed jobs

- **WHEN** the recorded job list of the nightly (7 jobs) has two failed
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

### Requirement: CFR-6 Only scheduled runs of this repository report

The reporter SHALL act only when `workflow_run.event == 'schedule'` and
`workflow_run.head_repository.full_name == github.repository`, SHALL NOT check out or run code from
the triggering run, and SHALL surface its own errors as its own failed run.

#### Scenario: Dispatch, fork or non-schedule event

- **WHEN** the event is `workflow_dispatch`, comes from another repository, or is not `schedule`
- **THEN** the reporter does nothing

#### Scenario: Reporting breaks

- **WHEN** `gh` errors
- **THEN** the reporter's own run fails visibly; the reported run's conclusion is unaffected

### Requirement: CFR-7 The decision logic is tested and the workflow satisfies the zizmor gate

`hack/test/ci_failure_report_test.sh` SHALL cover every row of (conclusion x open-issue x ordering)
and the body builder on recorded fixtures with a fake `gh`, with mutants, and SHALL run on
`pull_request` in a required job. The workflow's `dangerous-triggers` suppression SHALL carry its
reason.

#### Scenario: Decision row removed

- **WHEN** the recovery branch, the ordering check or the label-and-author lookup is removed (mutants)
- **THEN** the test SHALL fail

#### Scenario: Suppression without reason

- **WHEN** the `dangerous-triggers` entry has no comment
- **THEN** the zizmor meta-test SHALL fail
