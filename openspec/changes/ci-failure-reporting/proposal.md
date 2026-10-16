# Proposal

## Why

`e2e-nightly.yaml` runs the L4 kind matrix, the race suite and the extractor budget every night
(`cron: "0 3 * * *"`, `e2e-nightly.yaml:6`). A red night produces a red run in the Actions tab and
nothing else: no issue, no record that a later night recovered. `go-patch-lag.yaml` (CFC-5,
`cron: "23 6 * * 1"`, `go-patch-lag.yaml:10`) goes red on purpose every Monday that go.mod's Go
lags the latest patch of its minor (`hack/ci/go-patch-lag.sh:110` exits 1), and that red is just
as invisible. Later changes add more scheduled signals (a shuffle run, a mutation report) that are
useless unless a failure reaches a person. attune closes the loop with one deduplicated issue per
failing workflow and a parsed body (`scripts/nightly-failure-issue-body.sh` in
github.com/attune-io/attune).

## What Changes

- `hack/ci/ci-failure-issue-body.sh` builds a Markdown body from a run's job list and failed-step
  logs: a bot marker on the first line, workflow, run link, failed jobs, `FAIL` lines. It never
  evaluates log text and keeps excerpts Markdown-inert (fence longer than any backtick run inside,
  `@` neutralised outside fences).
- `hack/ci/ci-failure-report.sh` treats the `workflow_run` payload as a trigger only. It acts on
  the latest completed scheduled run of the workflow (`gh run list --workflow <path> --event
  schedule --status completed --limit 1`) and decides `open`, `comment`, `close` or `none` from
  that run's conclusion, its ordering key `(run_number, run_attempt)` and the key in the bot marker
  of the newest bot issue for the workflow, open or closed (spec.md CFR-1 decision table). It reads
  the run's conclusion only, never the conclusion of a single job. Full reconciliation of every
  missed run is rejected: the latest run is the state worth reporting.
- `.github/workflows/ci-failure-report.yaml`, triggered by `workflow_run` (completed) for a listed
  set of scheduled workflows, runs it. One open issue per reported workflow, label `ci-failure`.
  The workflow creates the label idempotently (`gh label create ci-failure --force`) before the
  reporter runs, so the first report does not depend on an operator setting. A separate workflow,
  because a job inside the run has no conclusion yet.
- `hack/test/ci_failure_report_test.sh` (plain and `--self-test`) locks the decision table, the
  body builder and the workflow's wiring, including that every listed workflow name equals the
  `name:` of a file under `.github/workflows/`.

## Capabilities

### New Capabilities

- `ci-failure-reporting`: how scheduled-workflow failures become one tracked issue and how
  recovery is recorded.

### Modified Capabilities

None.

## Impact

- Entry point: `.github/workflows/ci-failure-report.yaml` (workflow default `contents: read`; the
  job holds exactly `contents: read`, `actions: read`, `issues: write`, because job permissions
  replace the workflow map; precedent `codeql.yaml:28-31`). It checks out the default branch only, never the triggering
  run's ref or repository.
- The reported set is `E2E nightly` (daily) and `Go patch lag` (weekly, Mondays). While Go lags,
  Go patch lag keeps one issue open and adds one comment per red Monday; the first green Monday
  after a Go bump closes it. Changes 8 and 9 add their own workflows to the list. Noisy signals
  (shuffle, mutation) live in their own workflows so that they get their own issue and cannot mask
  a regression of the nightly suite.
- `workflow_run` trips zizmor's `dangerous-triggers` at high severity (measured with zizmor 1.30.1
  at the gate's `--min-severity=high` on a scratch workflow). The suppression goes in
  `.github/zizmor.yml` with its reason (no checkout of the triggering run, no log text evaluated,
  same-repo guard), as `rules:` > `dangerous-triggers:` > `ignore:` > `- ci-failure-report.yaml`,
  with a comment directly above the rule key and another directly above the entry (CWS-2 checks
  both). No comment under `.github/` may contain the literal `zizmor: ignore`, which CWS-2's inline
  grep matches anywhere. Inline `# zizmor: ignore[...]` is not an option: CWS-2 rejects it
  (`hack/test/ci_workflow_security_test.sh:303-308`). The config's header comment, which says
  there are no suppressions (`.github/zizmor.yml:10-14`), changes with it.

## Dependencies

No technical dependency on another open change. What this change builds on is on `main`: the
zizmor gate and CWS-2 (change 1, ci-workflow-hardening) and the Go patch lag workflow (change 4,
cross-file-consistency-gates, CFC-5). The new workflow must satisfy the zizmor gate in its own
tasks.

## Non-goals

- Paging, chat notifications, auto-retry of failed jobs.
- Reporting PR or push runs, or `workflow_dispatch` runs.
- Reporting a job that fails under `continue-on-error`. The nightly `race` job
  (`e2e-nightly.yaml:127-131`, `continue-on-error: true`) cannot change the run's conclusion, so a
  race failure is not reported. This is a known gap until SINK-BREAKER-RACE-01 makes the race job
  blocking: its acceptance includes removing `continue-on-error`, after which race failures reach
  the run conclusion and this reporter with no change here. The workflow list carries a comment
  naming the gap.
- Reporting `scale-envtest-10k`: it runs only on `workflow_dispatch` with `run_scale_jobs`
  (`e2e-nightly.yaml:220`) and is skipped on every scheduled run.
- Reporting `codeql`, `scorecard`, `renovate` (own notification paths); the workflow list is
  data, so a later change can add them.
- Reporting the reporter's own failures: they are visible only in the Actions tab, because nothing
  reports a `workflow_run` run.

## Assumptions

- `workflow_run` payloads carry `workflow_run.event`, `.conclusion`, `.html_url`, `.id`,
  `.run_number`, `.run_attempt`, `.name`, `.path`, `.workflow_id` and
  `.head_repository.full_name`. Source: GitHub
  docs, "workflow_run" event; probe in task 1.1 with a recorded payload.
- `on.workflow_run.workflows` matches the triggering workflow's `name:`, not its file name, so a
  rename disconnects the reporter silently unless a test pins the names.
- `workflow_run` fires only for workflow files on the default branch, so live evidence exists only
  after merge (post-merge rows below).
- `gh run view <id> --json jobs` lists one entry per job instance. A scheduled nightly lists 11:
  7 job definitions, the `e2e-scenario` matrix expanding to 5 entries, and `scale-envtest-10k` as
  `skipped` (run 38871014363). A cancelled run whose jobs never started shows jobs as skipped or
  not started. A `workflow_dispatch` that replaces a pending run shows that run as `cancelled`.
  Probe in task 1.1.
- `gh run view --log-failed` prints logs only for a completed run (probe in task 1.1).
- `gh label create <name> --force` succeeds when the label exists: "Update the label color and
  description if label already exists" (`gh label create --help`, gh 2.98.0, read 2026-10-16;
  manual https://cli.github.com/manual/gh_label_create; source `pkg/cmd/label/create.go` in
  github.com/cli/cli). That it needs only `issues: write` is proven live by the post-merge row.
- `gh run list --workflow .github/workflows/e2e-nightly.yaml --event schedule --status completed
  --limit 1` accepts the path form and returned run 38871014363 (verified 2026-10-16, gh 2.98.0,
  read-only). Runs list newest first by creation, so a re-run of an older run is not the latest
  (believed; probe in task 1.1).
- `gh run view` defaults to the latest attempt; `--attempt` selects one (`gh run view --help`).
- The bot-author spelling for `gh issue list` (`--author app/<bot>` or `--app <bot>`) is recorded
  read-only against an existing bot-authored issue in task 1.1; the list API is used, never
  `--search`, whose index lags.
- A concurrency group keeps at most one pending run, so a third event for one workflow while one
  report runs and one waits replaces the waiting one. Harmless: the payload only triggers the
  reporter, which reads the latest completed scheduled run itself.
- GitHub issue bodies cap at 65536 characters.
