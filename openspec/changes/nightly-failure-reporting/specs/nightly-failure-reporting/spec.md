# Spec Delta

## Purpose

Defines how a failing scheduled workflow becomes exactly one open, up-to-date GitHub issue, and
how a recovery is recorded, so a red night is noticed and a green night is not mistaken for
"never failed".

## ADDED Requirements

### Requirement: NFR-1 A failed nightly opens one issue

When a scheduled run of a reported workflow completes with conclusion `failure`, `timed_out` or
`cancelled` and no open issue with label `ci/nightly-failure` and the workflow's title key exists,
the report workflow SHALL open one.

#### Scenario: Timed out or cancelled run

- **WHEN** the nightly completes with `timed_out` or `cancelled`
- **THEN** an issue is opened (or commented) naming the conclusion; `cancelled` and `timed_out` are never treated as success

#### Scenario: First failure

- **WHEN** the nightly fails and no matching open issue exists
- **THEN** one issue is created with the label and a body listing the failed jobs

#### Scenario: Two issues never exist

- **WHEN** two reports run at the same time for the same workflow (a re-run completing while the first report is running)
- **THEN** there SHALL NOT be two open issues; the report workflow's concurrency group is per reported workflow with `cancel-in-progress: false`, so reports serialize

### Requirement: NFR-2 Repeated failures update the same issue

When a matching open issue exists and the run fails, the report job SHALL add one comment with the
new body instead of opening another issue.

#### Scenario: Second red night

- **WHEN** the nightly fails on consecutive nights
- **THEN** the existing issue gets a comment with that night's run link and failed jobs, and no new issue is created

### Requirement: NFR-3 Recovery is recorded

When a scheduled run succeeds and a matching open issue exists, the report job SHALL comment
that the run recovered, with its link, and close the issue.

#### Scenario: Success after failure after success (A to B to A)

- **WHEN** night 1 passes, night 2 fails, night 3 passes
- **THEN** night 1 touches no issue, night 2 opens one, night 3 comments recovery and closes it

#### Scenario: Failure after recovery reopens as new

- **WHEN** a closed issue exists and the next night fails
- **THEN** a new issue is opened; the closed one is not reopened

#### Scenario: Green night with no open issue

- **WHEN** the nightly passes and no matching open issue exists
- **THEN** nothing is created, commented or closed

### Requirement: NFR-4 The body names what failed

`hack/ci/nightly-failure-issue-body.sh` SHALL emit the workflow name, the run URL, each failed or
timed-out job by name, and up to a bounded number of `FAIL` lines from the failed logs, and SHALL
keep the result under 60000 characters.

#### Scenario: Two failed jobs

- **WHEN** the job list has `e2e-scenario` (one matrix leg) and `race` failed and the other jobs passed
- **THEN** the body lists exactly those two jobs (fixture: a recorded nightly job list of its 7 jobs, 2 failed)

#### Scenario: Huge log

- **WHEN** the failed log exceeds the cap
- **THEN** the body is truncated with a visible marker, and still lists every failed job

#### Scenario: No FAIL lines

- **WHEN** a job failed with no `FAIL` line (for example a timeout)
- **THEN** the job is still listed, with its failed step name

### Requirement: NFR-5 Only scheduled runs of this repository report

The report workflow SHALL act only when `workflow_run.event == 'schedule'` and
`workflow_run.head_repository.full_name == github.repository`, SHALL NOT check out or run code
from the triggering run, and SHALL surface a reporting error as its own failed run.

#### Scenario: Manual dispatch fails

- **WHEN** a `workflow_dispatch` run fails
- **THEN** no issue is opened or commented

#### Scenario: Run from a fork or a pull request

- **WHEN** a `workflow_run` event comes from another repository or a non-schedule event
- **THEN** the report workflow does nothing

#### Scenario: Reporting breaks

- **WHEN** `gh` returns an error in the report workflow
- **THEN** that workflow's own run fails visibly; the nightly's conclusion is unaffected (it already completed)

### Requirement: NFR-6 The decision logic is tested

`hack/test/nightly_failure_report_test.sh` SHALL cover every row of the decision table
(conclusion x open-issue-exists) and the body builder on recorded fixtures, using a fake `gh`.

#### Scenario: Decision table row removed

- **WHEN** the script's recovery branch is removed (mutant)
- **THEN** the test SHALL fail
