# Spec Delta

## Purpose

Defines refusals that go beyond "the tests passed once, in source order": a fuzz runner that
cannot hide a crash or a no-op, shuffled repeated unit runs, and a README quick start that names
things that exist.

## ADDED Requirements

### Requirement: TDS-1 Fuzz runs once per target, retries only the known flake, and cannot be empty

`hack/ci/fuzz-retry.sh` SHALL run each target for 30 seconds. It SHALL retry at most once, and
only when the failed attempt's output contains `--- FAIL: <target>` followed by a
`context deadline exceeded` line, contains none of `panic:`, `fatal error:`,
`Failing input written`, `signal: killed`, and `git status --porcelain` shows no new file under
`testdata/fuzz/`. Every other failure SHALL fail the job at once (fail closed): when the failure
matches neither the deadline flake nor a deny marker, the error message SHALL contain
`unrecognised failure text`, so a change in Go's wording is diagnosable from the log. A run that
exits 0 with `no fuzz tests to fuzz` in its output and no `fuzz: elapsed` line SHALL fail as zero
targets run. A run that exits 0 with no `fuzz: elapsed` line and without that warning SHALL also
fail (fail closed, `unrecognised` in the message): a pass that fuzzed nothing is never green.

#### Scenario: Pure deadline artifact

- **WHEN** the first attempt fails with `--- FAIL: FuzzX` and `context deadline exceeded`, with
  none of the deny markers and no new corpus file, and the second passes
- **THEN** the job passes and logs a warning naming golang/go#75804

#### Scenario: Panic is never retried

- **WHEN** the first attempt prints `panic:`, even alongside `context deadline exceeded`
- **THEN** the job SHALL fail without a second attempt

#### Scenario: Killed process

- **WHEN** the output contains `signal: killed`
- **THEN** the job SHALL fail without a second attempt

#### Scenario: Crash corpus written

- **WHEN** the first attempt fails with the deadline text and a new file appears under
  `testdata/fuzz/`
- **THEN** the job SHALL fail without a second attempt

#### Scenario: Deadline twice

- **WHEN** both attempts fail with the deadline artifact
- **THEN** the job SHALL fail

#### Scenario: Unknown failure text

- **WHEN** the output is empty or matches none of the known patterns
- **THEN** the job SHALL fail without a retry, and the message SHALL contain
  `unrecognised failure text`

#### Scenario: Zero fuzz targets

- **WHEN** `go test` exits 0, prints `no fuzz tests to fuzz` (a renamed target, a wrong package)
  and no `fuzz: elapsed` line
- **THEN** the job SHALL fail and say that no fuzz target ran; a matrix leg that fuzzes nothing
  never passes

### Requirement: TDS-2 Unit tests do not depend on order

`task test:shuffle` (`shuffle:` in `hack/task/Taskfile.test.yml`) SHALL choose a seed, print it
before any test runs, and run the unit packages with `-shuffle=<seed> -count=3`. The unit packages
are `go list ./...` minus an explicit exclusion list in which every entry carries a reason; the
list SHALL hold only the e2e packages. Envtest-backed packages SHALL run, with envtest assets
provisioned as the CI test jobs do (`.github/actions/go-cache` with `envtest: true`, then
`make setup-envtest`, as `hack/coverage.sh:23` does). A nightly workflow `test-shuffle.yaml`
SHALL run it on `schedule:`, SHALL be a workflow of its own, and SHALL be listed under
`on.workflow_run.workflows` of `ci-failure-report.yaml`, which reports scheduled runs only.

`-shuffle` reorders top-level `Test*` functions only, not Ginkgo specs; a pass of this workflow is
not evidence that the Ginkgo suites are order-independent (see proposal *Non-goals*).

#### Scenario: Order-dependent test

- **WHEN** a unit test passes only after another test has run
- **THEN** the run SHALL fail on some seed and the output names the seed

#### Scenario: Seed reproducible

- **WHEN** a shuffled run fails with seed N at `-count=3`
- **THEN** `go test -shuffle=N -count=3` on the same package reproduces the failure (one RNG stream
  serves all iterations, so `-count` must match)

#### Scenario: Exclusion without a reason

- **WHEN** a package is added to the exclusion list with no reason
- **THEN** the guard test SHALL fail

#### Scenario: Exclusion beyond e2e

- **WHEN** a non-e2e package (for example an envtest-backed one) is added to the exclusion list
- **THEN** the guard test SHALL fail

#### Scenario: Shuffle failure does not hide the nightly

- **WHEN** the scheduled shuffle run and the scheduled E2E nightly both fail
- **THEN** two separate issues exist

### Requirement: TDS-3 The README quick start names things that exist

`hack/test/readme_quickstart_test.sh` SHALL take the README section whose `##` heading starts with
`Quick start` (today `## Quick start (MVP)`) and extract the commands of its fenced `sh` blocks,
including commands in `#` comments (`# or: task dev-up` offers `task dev-up` to users, so it
counts); prose outside the fences is not extracted. It SHALL fail when a `task <name>` is not a task
of `Taskfile.yml` or of a Taskfile it includes (an included task is `<namespace>:<name>`), a
`kubectl apply -k <path>` path does not exist, or a short name in `kubectl get` is not a CRD
`shortNames` entry in `config/crd/bases`. It SHALL NOT duplicate the delegation checks of
`demo_task_aliases_test.sh`.

#### Scenario: Task renamed

- **WHEN** `demo-up` is renamed and the README is not updated
- **THEN** the test SHALL fail and name the task

#### Scenario: Commented alternative renamed

- **WHEN** `dev-up` is renamed and the README comment `# or: task dev-up` is not updated
- **THEN** the test SHALL fail and name `dev-up`

#### Scenario: Unknown short name

- **WHEN** the README says `kubectl get kinv,foo`
- **THEN** the test SHALL fail and name `foo`

#### Scenario: Missing apply path

- **WHEN** the README says `kubectl apply -k config/samples/nosuch/`
- **THEN** the test SHALL fail and name the path

#### Scenario: Section missing

- **WHEN** the README has no `##` heading starting with "Quick start"
- **THEN** the test SHALL fail; an empty extraction never passes
