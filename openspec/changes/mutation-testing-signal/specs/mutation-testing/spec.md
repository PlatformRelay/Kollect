# Spec Delta

## Purpose

Defines the nightly mutation measurement of kollect's most correctness-critical pure packages and
the rule that it measures and does not gate.

## ADDED Requirements

### Requirement: MUT-1 Mutation results are measured nightly, in their own workflow

A nightly workflow of its own, `.github/workflows/mutation.yaml` with `name: Mutation testing` and
a `schedule:` cron of `30 1 * * *`, SHALL run `task mutation`. The task SHALL install gremlins only
through a root `Taskfile.yml` task that runs
`go install github.com/go-gremlins/gremlins/cmd/gremlins@{{.GREMLINS_VERSION}}`, where
`GREMLINS_VERSION` is one full version (v0.6.0 today) directly under
`# renovate: datasource=go depName=github.com/go-gremlins/gremlins`; no other file SHALL name a
gremlins version. The run SHALL invoke gremlins once per package on `internal/aggregate`,
`internal/redact` and `internal/pathvalidate`, each writing its own JSON report, with every mutator
that the pinned version ships switched off enabled by an explicit flag. It SHALL upload the reports
and write, per package, the killed, lived, not covered, timed out and not viable counts to the job
summary, counted from each report's mutation statuses.

The workflow SHALL declare `permissions: contents: read` only, check out with
`persist-credentials: false`, and use only actions already pinned elsewhere in the repository, at
those SHAs. `Mutation testing` SHALL be listed in `.github/workflows/ci-failure-report.yaml`
`on.workflow_run.workflows`.

#### Scenario: Report produced

- **WHEN** the nightly runs
- **THEN** the summary shows the five counts for each of the three packages and the report
  artifact exists

#### Scenario: Not part of the nightly suite

- **WHEN** a scheduled mutation run fails
- **THEN** its failure is reported as its own `CI failure: Mutation testing` issue, separate from
  the `E2E nightly` issue

#### Scenario: Version restated outside the pin

- **WHEN** a workflow, an included Taskfile or a script names `gremlins@` with a version, or the
  `# renovate:` line above `GREMLINS_VERSION` is missing
- **THEN** the guard SHALL fail and name the file

### Requirement: MUT-2 A low score is not a failure, a missing measurement is, and there is no gate

The workflow SHALL NOT fail on a low mutation score and SHALL fail when the tool cannot produce a
report. No threshold SHALL be set by any source gremlins reads from the repository or the run:

- no `--threshold-` option on any gremlins command line in `mutation.yaml`, `Taskfile.yml` or
  `hack/ci/mutation*.sh`;
- no tracked `.gremlins.yaml` or `.gremlins.yml` anywhere in the tree (gremlins reads one from the
  module root, and the fixture module is a module root too);
- no environment variable whose name matches `GREMLINS_` and contains `THRESHOLD` (case
  insensitive) at workflow, job or step `env:` level of `mutation.yaml`, in `Taskfile.yml` `env:`,
  or as text anywhere in those files and `hack/ci/mutation*.sh` (which covers a write to
  `$GITHUB_ENV` and an inline `VAR=… gremlins`).

The guard reads those files; the formatter never sees the arguments or the environment. The
required `lint` job SHALL run the guard plain and with `--self-test` and SHALL NOT install gremlins.

#### Scenario: Surviving mutants

- **WHEN** a package has surviving mutants
- **THEN** the run stays green

#### Scenario: Tool broken

- **WHEN** gremlins cannot run (build failure, panic, non-zero exit, no report, an empty report,
  a report with no mutants, or a status that is not a measurement such as RUNNABLE or SKIPPED)
- **THEN** the run SHALL fail, because a missing measurement must not look like a clean one

#### Scenario: Gate added on the command line

- **WHEN** a threshold option such as `--threshold-efficacy` appears in the workflow, the Taskfile
  or the runner
- **THEN** the guard SHALL fail and name the file and line

#### Scenario: Gate added through a config file

- **WHEN** a `.gremlins.yaml` or `.gremlins.yml` is tracked anywhere in the tree
- **THEN** the guard SHALL fail and name the path

#### Scenario: Gate added through the environment

- **WHEN** `GREMLINS_UNLEASH_THRESHOLD_EFFICACY` is set at workflow, job or step level, or written
  to `$GITHUB_ENV`
- **THEN** the guard SHALL fail and name the file and line

### Requirement: MUT-3 The measurement is proven able to see a surviving mutant

A fixture module under `hack/test/testdata/mutation/`, with its own `go.mod`, SHALL contain a
function whose test executes it without asserting on its result. Each nightly run SHALL run
gremlins on the fixture before the real packages and SHALL fail unless the control mutant is
reported LIVED. Code that no test executes SHALL be reported as NOT COVERED, and the control SHALL
NOT rely on it. The control runs in the nightly only; pull requests run the static guard of MUT-2,
not gremlins.

#### Scenario: Executed but unasserted

- **WHEN** gremlins runs on the fixture
- **THEN** the control mutant is LIVED and the run continues to the real packages

#### Scenario: Control reported as not covered

- **WHEN** the control mutant is reported NOT COVERED
- **THEN** the run SHALL fail before the real packages (the fixture no longer executes the
  function)

#### Scenario: Control killed or missing

- **WHEN** the control mutant is reported KILLED, or is absent from the fixture report
- **THEN** the run SHALL fail before the real packages (the fixture asserts on the result, or the
  mutator that makes the control is gone)

### Requirement: MUT-4 Success to tool failure to recovery is visible

A night where the tool fails SHALL be distinguishable in the run from a night with surviving
mutants: a tool failure is a red run with an `::error::` line naming the failure and no counts
table; a night with survivors is green with its counts.

#### Scenario: Night 1 ok, night 2 tool failure, night 3 ok

- **WHEN** the three nights run
- **THEN** night 2 is red with the tool error, nights 1 and 3 are green, each with its own run's
  report

#### Scenario: Forced failure on a scheduled run

- **WHEN** the repository variable `MUTATION_FORCE_FAILURE` is `true` for a scheduled run
- **THEN** that run SHALL fail with an `::error::` line naming the forced failure, and the next
  scheduled run with the variable unset SHALL be judged on its measurement alone; the variable
  SHALL have no effect other than failing the run
