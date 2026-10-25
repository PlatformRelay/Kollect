# mutation-testing Specification

## Purpose
Defines the nightly mutation measurement of kollect's most correctness-critical pure packages and
the rule that it measures and does not gate.

## Requirements

### Requirement: MUT-1 Mutation results are measured nightly, in their own workflow

A nightly workflow of its own, `.github/workflows/mutation.yaml` with `name: Mutation testing` and
a `schedule:` cron of `30 1 * * *`, SHALL run `task mutation` and nothing else that executes
code besides the pinned checkout, Go cache, Task setup and artifact upload actions. gremlins SHALL
be installed only by a root `Taskfile.yml` task running
`go install github.com/go-gremlins/gremlins/cmd/gremlins@{{.GREMLINS_VERSION}}` into a
version-named path, where `GREMLINS_VERSION` is one full version (v0.6.0 today), a top-level
`vars:` entry directly under `# renovate: datasource=go depName=github.com/go-gremlins/gremlins`.
Neither task SHALL carry anything that can skip or soften it (`status:`, `sources:`,
`generates:`, `method:`, `preconditions:`, `ignore_error:` at task or command level, `env:`,
`dotenv:`). No other executable or configuration file (the `Makefile`, `Taskfile.yml` and every
Taskfile it includes, everything under `.github/`, `hack/` outside `hack/test/`) SHALL install or
invoke gremlins, or reach the install task, the `mutation` task or `hack/ci/mutation.sh`; only the
`mutation.yaml` run step runs `task mutation`, and only `hack/ci/mutation.sh` invokes gremlins. Evidence and
fixture provenance may name the version.

`hack/ci/mutation.sh` SHALL invoke gremlins once per package on `internal/aggregate`,
`internal/redact` and `internal/pathvalidate`, each writing its own JSON report into a directory
created fresh for the run, with every mutator flag of the pinned version passed explicitly as
enabled, `--config=<repository root>/hack/ci/gremlins.yaml` as an absolute path (the fixture
call runs from the fixture's directory) and `--timeout-coefficient=10` (at the default 3 a loaded
runner times whole packages out). Before its first `unleash` call it SHALL read the pinned
binary's mutator flags from `gremlins unleash --help` and fail unless they are exactly the flags
it passes, so a version bump cannot leave a new mutator silently off. When `$GITHUB_OUTPUT` is set, the runner SHALL write
`reports=<that directory>` to it before its first gremlins call and SHALL leave the directory in
place; the workflow's upload step SHALL take its `path:` from that output, with
`if-no-files-found: error`. `hack/ci/mutation-summary.sh` SHALL take exactly one
`<package>=<report>` argument per measured package, reject a missing, duplicate or unknown
package, validate every report before it prints anything, and write, per package, the killed,
lived, not covered, timed out and not viable counts, counted from the report's mutation statuses,
to `$GITHUB_STEP_SUMMARY` (stdout outside Actions). Diagnostics are `::error::mutation:` lines on
stderr.

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

#### Scenario: Installed or invoked outside the pin

- **WHEN** a workflow, the Makefile, an included Taskfile or a `hack/` script outside `hack/test/`
  installs, downloads or invokes gremlins, or the `# renovate:` line above `GREMLINS_VERSION` is
  missing
- **THEN** the guard SHALL fail and name the file

### Requirement: MUT-2 A low score is not a failure, a missing measurement is, and there is no gate

The workflow SHALL NOT fail on a low mutation score and SHALL fail when the tool cannot produce a
fresh report. No threshold SHALL be set by any source gremlins reads:

- no threshold option on any gremlins command line in `mutation.yaml`, `Taskfile.yml` or
  `hack/ci/mutation*.sh`, in any spelling gremlins accepts (it normalises `_` and `.` in flag names
  to `-`, so the guard matches `--threshold[-_.]`, case insensitive);
- no tracked file named `.gremlins` or `.gremlins.<anything>` anywhere in the tree (gremlins reads
  one from the module root under any extension, and the fixture module is a module root too), and
  `hack/ci/gremlins.yaml`, the only config the run reads, holds an empty map (`{}`) and comments
  only;
- no environment variable whose name matches `GREMLINS_` and contains `THRESHOLD` (case
  insensitive) at workflow, job or step `env:` level of `mutation.yaml`, in `Taskfile.yml` `env:`,
  or as text anywhere in those files and `hack/ci/mutation*.sh` (which covers a write to
  `$GITHUB_ENV` and an inline `VAR=… gremlins`). At run time the runner SHALL refuse to start when
  any `GREMLINS_` variable is set, which covers sources the scan cannot see (an `env:` built from a
  `${{ }}` expression, a Taskfile `dotenv:`).

The guard reads those files; the formatter never sees the arguments or the environment. The
guard reads YAML semantically (anchors and aliases expanded, flow and block forms alike, quoted
keys alike). Every read, `git ls-files` call and YAML parse the guard makes SHALL fail the guard
when it errors, never read as "nothing found". The guard SHALL collect every failure before it
exits, so one defect never hides another's message. The required `lint` job SHALL run the guard plain and with
`--self-test` and SHALL NOT install or run gremlins.

#### Scenario: Surviving mutants

- **WHEN** a package has surviving mutants
- **THEN** the run stays green

#### Scenario: Tool broken

- **WHEN** gremlins cannot run (build failure, panic, non-zero exit, no report, an empty report,
  a report with no mutants, a status that is not a measurement such as RUNNABLE or SKIPPED, or a
  package none of whose mutants was killed or lived, because each one timed out, was not viable
  or is not covered)
- **THEN** the run SHALL fail with an `::error::mutation:` line and no counts table, because a
  missing measurement must not look like a clean one

#### Scenario: Gate added on the command line

- **WHEN** a threshold option such as `--threshold-efficacy` or `--threshold_efficacy` appears in
  the workflow, the Taskfile or the runner
- **THEN** the guard SHALL fail and name the file and line

#### Scenario: Gate added through a config file

- **WHEN** a `.gremlins`, `.gremlins.yaml` or `.gremlins.json` is tracked anywhere in the tree, or
  `hack/ci/gremlins.yaml` holds any key
- **THEN** the guard SHALL fail and name the path

#### Scenario: Gate added through the environment

- **WHEN** `GREMLINS_UNLEASH_THRESHOLD_EFFICACY` is set at workflow, job or step level, or written
  to `$GITHUB_ENV`
- **THEN** the guard SHALL fail and name the file and line

#### Scenario: Environment set where the scan cannot see it

- **WHEN** any `GREMLINS_` variable is present in the runner's environment
- **THEN** the runner SHALL fail before its first gremlins call and name the variable, whatever
  the variable's name after the prefix and whether or not its value is empty

### Requirement: MUT-3 The measurement is proven able to see a surviving mutant

A fixture module under `hack/test/testdata/mutation/`, with its own `go.mod`, SHALL contain a
function whose test executes it without asserting on its result. Its control mutant is identified
by file, line, column and mutator type, recorded in task 1.1. Each nightly run SHALL run gremlins
on the fixture before the real packages and SHALL fail unless the report holds that control
exactly once, with status LIVED. Code that no test executes SHALL be reported as NOT COVERED, and
the control SHALL NOT rely on it. The control runs in the nightly only; pull requests run the
static guard of MUT-2, not gremlins.

#### Scenario: Executed but unasserted

- **WHEN** gremlins runs on the fixture
- **THEN** the control mutant is LIVED and the run continues to the real packages

#### Scenario: Control reported as not covered

- **WHEN** the control mutant is reported NOT COVERED
- **THEN** the run SHALL fail before the real packages (the fixture no longer executes the
  function)

#### Scenario: Control killed or missing

- **WHEN** the control mutant is reported KILLED, or is absent from the fixture report while
  another mutant is LIVED
- **THEN** the run SHALL fail before the real packages (the fixture asserts on the result, or the
  mutator that makes the control is gone)

### Requirement: MUT-4 Success to tool failure to recovery is visible

A night where the tool fails SHALL be distinguishable in the run from a night with surviving
mutants: a tool failure is a red run with an `::error::mutation:` line naming the failure and no
counts table; a night with survivors is green with its counts.

#### Scenario: Night 1 ok, night 2 tool failure, night 3 ok

- **WHEN** the three nights run
- **THEN** night 2 is red with the tool error, nights 1 and 3 are green, each with its own run's
  report

#### Scenario: Forced failure on a scheduled run

- **WHEN** the repository variable `MUTATION_FORCE_FAILURE` is `true` for a scheduled run
- **THEN** that run SHALL fail with an `::error::mutation:` line naming the forced failure, and the
  next scheduled run with the variable unset SHALL be judged on its measurement alone; the
  variable SHALL have no effect other than failing the run
