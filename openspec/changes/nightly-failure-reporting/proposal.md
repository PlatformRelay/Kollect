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
- `e2e-nightly.yaml` gains a final `report` job (`if: always()`, scheduled runs only) that runs it.

## Capabilities

### New Capabilities

- `nightly-failure-reporting`: how scheduled-workflow failures become one tracked issue and how
  recovery is recorded.

### Modified Capabilities

None.

## Impact

- Entry point: the `report` job of `.github/workflows/e2e-nightly.yaml`, which needs
  `issues: write` for that job only (the workflow default stays `contents: read`).
- A one-time label `ci/nightly-failure` must exist (created by the script when absent).
- Tested by `hack/test/nightly_failure_report_test.sh` in the same style as the other
  `hack/test/*_test.sh` files, using a fake `gh` on `PATH`.

## Non-goals

- Reporting for `codeql`, `scorecard`, `renovate` (weekly, own notification paths). The scripts
  take the workflow name as an argument so a later change can opt them in.
- Paging, Slack, or auto-retry of failed nightly jobs.
- Reporting failures of PR or push runs.

## Assumptions

- `gh run view <id> --json jobs` lists jobs with `name`, `conclusion`, and steps with
  `conclusion`. Probe: run it against a recent nightly in task 1.1 and record the sample as a
  test fixture.
- `gh run view --log-failed` prints failed-step logs; its size is bounded by the body builder
  (GitHub issue bodies cap at 65536 characters).
- `workflow_dispatch` runs of the nightly must not open or close issues (they are manual probes);
  only `schedule` events report. Source: `e2e-nightly.yaml` `on:` block.
