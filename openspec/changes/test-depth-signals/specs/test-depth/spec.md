# Spec Delta

## Purpose

Defines refusals that go beyond "the tests passed once, in source order": a fuzz runner that
cannot hide a crash or a no-op, shuffled repeated unit runs, and a README quick start that names
things that exist.

## ADDED Requirements

### Requirement: TDS-1 Fuzz runs once per target, retries only the known flake, and cannot be empty

`hack/ci/fuzz-retry.sh` SHALL run each target for 30 seconds, SHALL retry at most once and only when
the output contains `context deadline exceeded` and none of `panic:`, `fatal error:`,
`Failing input written`, `signal: killed`, and `git status` shows no new file under
`testdata/fuzz/`. Every other failure SHALL fail the job at once. A run in which no fuzz target ran
SHALL fail.

#### Scenario: Pure deadline artifact

- **WHEN** the first attempt fails with `context deadline exceeded` and the usual `--- FAIL: FuzzX` line, with none of the deny markers, and the second passes
- **THEN** the job passes and logs a warning naming golang/go#75804

#### Scenario: Panic is never retried

- **WHEN** the first attempt prints `panic:`, even alongside `context deadline exceeded`
- **THEN** the job SHALL fail without a second attempt

#### Scenario: Killed process

- **WHEN** the output contains `signal: killed`
- **THEN** the job SHALL fail without a second attempt

#### Scenario: Deadline twice

- **WHEN** both attempts fail with the deadline artifact
- **THEN** the job SHALL fail

#### Scenario: Unknown failure text

- **WHEN** the output is empty or matches none of the known patterns
- **THEN** the job SHALL fail without a retry

#### Scenario: Zero fuzz targets

- **WHEN** the run reports that no fuzz test was found or ran (a renamed target, a wrong package)
- **THEN** the job SHALL fail; a matrix leg that fuzzes nothing never passes

### Requirement: TDS-2 Unit tests do not depend on order

A nightly workflow `test-shuffle.yaml` SHALL run the unit packages with `-shuffle=on -count=3`,
SHALL print the seed, and SHALL be a workflow of its own listed among those the CI-failure reporter
reports. Packages that need Docker, envtest or kind are excluded by an explicit list with reasons.

#### Scenario: Order-dependent test

- **WHEN** a unit test passes only after another test has run
- **THEN** the run SHALL fail on some seed and the output names the seed

#### Scenario: Seed reproducible

- **WHEN** a shuffled run fails with seed N
- **THEN** `go test -shuffle=N` on the same package reproduces the failure

#### Scenario: Exclusion without a reason

- **WHEN** a package is excluded from the list with no reason
- **THEN** the guard test SHALL fail

#### Scenario: Shuffle failure does not hide the nightly

- **WHEN** the shuffle workflow and the nightly both fail
- **THEN** two separate issues exist

### Requirement: TDS-3 The README quick start names things that exist

`hack/test/readme_quickstart_test.sh` SHALL extract the commands of the README "Quick start"
section and fail when a `task <name>` is not defined in `Taskfile.yml`, a `kubectl apply -k <path>`
path does not exist, or a short name in `kubectl get` is not a CRD `shortNames` entry in
`config/crd/bases`. It SHALL NOT duplicate the delegation checks of `demo_task_aliases_test.sh`.

#### Scenario: Task renamed

- **WHEN** `demo-up` is renamed and the README is not updated
- **THEN** the test SHALL fail and name the task

#### Scenario: Unknown short name

- **WHEN** the README says `kubectl get kinv,foo`
- **THEN** the test SHALL fail and name `foo`

#### Scenario: Section missing

- **WHEN** the README has no "Quick start" section
- **THEN** the test SHALL fail; an empty extraction never passes
