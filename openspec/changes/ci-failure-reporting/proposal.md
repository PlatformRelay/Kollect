# Proposal

## Why

`e2e-nightly.yaml` runs the L4 kind matrix, the race suite and the extractor budget every night
(`cron: "0 3 * * *"`). A red night produces a red run in the Actions tab and nothing else: no
issue, no record that a later night recovered. Later changes add more scheduled signals (a Go
patch-lag sensor, a shuffle run, a mutation report) that are useless unless a failure reaches a
person. attune closes the loop with one deduplicated issue per failing workflow and a parsed body
(`scripts/nightly-failure-issue-body.sh` in github.com/attune-io/attune).

## What Changes

- `hack/ci/ci-failure-issue-body.sh` builds a Markdown body from a run's job list and failed-step
  logs: workflow, run link, failed jobs, `FAIL` lines. It never evaluates log text.
- `hack/ci/ci-failure-report.sh` decides `open`, `comment`, `close` or `none` from the run's
  conclusion, its ordering key `(run_number, run_attempt)` and the existing open issue, and calls
  `gh`.
- `.github/workflows/ci-failure-report.yaml`, triggered by `workflow_run` (completed) for a listed
  set of scheduled workflows, runs it. One open issue per reported workflow, label `ci-failure`.
  A separate workflow, because a job inside the run has no conclusion yet.

## Capabilities

### New Capabilities

- `ci-failure-reporting`: how scheduled-workflow failures become one tracked issue and how
  recovery is recorded.

### Modified Capabilities

None.

## Impact

- Entry point: `.github/workflows/ci-failure-report.yaml` (`issues: write`, `actions: read`,
  default `contents: read`). It checks out nothing from the triggering run.
- The reported set starts with `E2E nightly` and `Go patch lag` (change 4); changes 8 and 9 add
  their own workflows to it. Noisy signals (shuffle, mutation) live in their own workflows so
  that they get their own issue and cannot mask a regression of the nightly suite.
- `workflow_run` trips zizmor's `dangerous-triggers`; the suppression in `.github/zizmor.yml` carries
  its reason (no checkout of the triggering run, no log text evaluated, same-repo guard).

## Dependencies

Landing order across the ten proposed changes: (1) ci-workflow-hardening, (2) task-check-entrypoint,
(3) golangci-lint-bump, (4) cross-file-consistency-gates, (5) developer-toolchain-pins,
(6) dependency-update-automation, (7) ci-failure-reporting, (8) test-depth-signals,
(9) mutation-testing-signal, (10) public-agent-contract. The new workflow must satisfy the
zizmor gate from change 1 in its own tasks.

## Non-goals

- Paging, chat notifications, auto-retry of failed jobs.
- Reporting PR or push runs, or `workflow_dispatch` runs.
- Reporting `codeql`, `scorecard`, `renovate` (own notification paths); the workflow list is
  data, so a later change can add them.

## Assumptions

- `workflow_run` payloads carry `workflow_run.event`, `.conclusion`, `.html_url`, `.id`,
  `.run_number`, `.run_attempt` and `.head_repository.full_name`. Source: GitHub docs,
  "workflow_run" event; probe in task 1.1 with a recorded payload.
- `workflow_run` fires only for workflow files on the default branch, so live evidence exists only
  after merge (post-merge rows below).
- `gh run view <id> --json jobs` lists jobs with `name`, `conclusion` and steps; a cancelled run
  whose jobs never started shows jobs as skipped or not started. A `workflow_dispatch` that
  replaces a pending run shows that run as `cancelled`. Probe in task 1.1.
- `gh run view --log-failed` prints logs only for a completed run (probe in task 1.1).
- GitHub issue bodies cap at 65536 characters.
