# Proposal

## Why

`e2e-nightly.yaml` runs the L4 kind matrix, the race suite and the extractor budget every night
(`cron: "0 3 * * *"`, line 6). A red night produces a red run in the Actions tab and nothing else:
no issue, no notification someone reads, no record that the next night recovered. The repo has
already paid for this (the comment in `hack/test/core_events_rbac_test.sh` cites a nightly run that
caught a regression after it landed). attune closes the loop with one deduplicated issue per
failing workflow and a parsed body (`scripts/nightly-failure-issue-body.sh` in
github.com/attune-io/attune).

## What Changes

- `hack/ci/nightly-failure-issue-body.sh` turns a run's job list and failed-step logs into a
  Markdown body: workflow, run link, failed jobs, and the `FAIL`/`--- FAIL` lines.
- `hack/ci/nightly-report.sh` decides one of `open`, `comment`, `close` or `none` from
  (run conclusion, existing open issue) and calls `gh`. There is at most one open issue per
  workflow, found by label `ci/nightly-failure` plus a title key.
- A separate workflow `.github/workflows/nightly-report.yaml`, triggered by
  `workflow_run: workflows: ["E2E nightly"], types: [completed]`, runs it. It is a separate
  workflow because a job inside the nightly runs before the run has a conclusion: `gh run view`
  cannot report `cancelled` or `timed_out` for a run still in progress, and its failed-step logs
  may not be available yet.

## Capabilities

### New Capabilities

- `nightly-failure-reporting`: how scheduled-workflow failures become one tracked issue and how
  recovery is recorded.

### Modified Capabilities

None.

## Impact

- Entry point: `.github/workflows/nightly-report.yaml`, whose job has `issues: write` and
  `actions: read`; the workflow default stays `contents: read`. It does not check out any ref
  other than the default branch and runs no code from the triggering run.
- A one-time label `ci/nightly-failure` must exist (created by the script when absent).
- Tested by `hack/test/nightly_failure_report_test.sh` in the same style as the other
  `hack/test/*_test.sh` files, using a fake `gh` on `PATH`.

## Dependencies

Landing order across the seven proposed changes: developer-toolchain-consistency (with its Go
bump), cross-file-consistency-gates, ci-workflow-hardening, dependency-update-automation,
nightly-failure-reporting, test-depth-signals, public-agent-contract.

## Non-goals

- Reporting for `codeql`, `scorecard`, `renovate` (weekly, own notification paths). The scripts
  take the workflow name as an argument so a later change can opt them in.
- Paging, Slack, or auto-retry of failed nightly jobs.
- Reporting failures of PR or push runs.

## Assumptions

- `gh run view <id> --json jobs` lists jobs with `name`, `conclusion`, and steps with
  `conclusion`. Probe: run it against a recent nightly in task 1.1 and record the sample as a
  test fixture.
- `gh run view --log-failed` prints failed-step logs only for a completed run (probe in task 1.1
  on an in-progress run to confirm it refuses; that probe is the reason for the separate
  workflow); its size is bounded by the body builder (GitHub issue bodies cap at 65536
  characters).
- `workflow_run` payloads carry `workflow_run.event`, `.conclusion`, `.html_url`, `.id` and
  `.head_repository.full_name`. Source: GitHub docs, "workflow_run" event.
- `workflow_dispatch` runs of the nightly must not open or close issues (they are manual probes);
  only runs with `workflow_run.event == 'schedule'` report. Source: `e2e-nightly.yaml` `on:` block.
