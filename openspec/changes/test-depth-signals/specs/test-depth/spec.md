# Spec Delta

## Purpose

Defines measurements and refusals that go beyond "the tests passed once": a fuzz retry that cannot hide a crash, repeated runs of golden tests, and a README that matches what
the smoke test runs.

## ADDED Requirements

### Requirement: TDS-1 Only the known fuzz-engine deadline artifact is retried

`hack/ci/fuzz-retry.sh` SHALL retry a failed fuzz run once only when its output contains
`context deadline exceeded` and contains no `panic:`, no `fatal error:`, no `Failing input written`,
no `signal: killed`, and `git status` shows no new file under `testdata/fuzz/`.
Every other failure SHALL fail the job at once.

#### Scenario: Pure deadline artifact

- **WHEN** the first attempt fails with `context deadline exceeded` and the usual `--- FAIL: FuzzX` line, with none of the deny patterns, and the second passes
- **THEN** the job passes and logs a warning naming golang/go#75804

#### Scenario: Panic is never retried

- **WHEN** the first attempt prints `panic:` (even with `context deadline exceeded` also present)
- **THEN** the job SHALL fail without a second attempt

#### Scenario: Killed process

- **WHEN** the output contains `signal: killed` (out-of-memory)
- **THEN** the job SHALL fail without a second attempt

#### Scenario: Deadline twice

- **WHEN** both attempts fail with the deadline artifact
- **THEN** the job SHALL fail

#### Scenario: Unknown failure text

- **WHEN** the output is empty or matches none of the known patterns
- **THEN** the job SHALL fail without a retry

### Requirement: TDS-2 Unit tests do not depend on order

`task test:shuffle` SHALL run the unit packages with `-shuffle=on -count=3`, SHALL print the seed,
and CI SHALL run it in the nightly (`e2e-nightly.yaml`), not in the PR `test-suite`, because it
multiplies unit-test time by three; the measured cost is recorded in task 2.1. Packages that need Docker, envtest or kind are excluded by an explicit list
with reasons.

#### Scenario: Order-dependent test

- **WHEN** a unit test passes only after another test has run
- **THEN** `task test:shuffle` SHALL fail on some seed, and the output names the seed to reproduce

#### Scenario: Seed reproducible

- **WHEN** a shuffled run fails with seed N
- **THEN** `go test -shuffle=N` on the same package reproduces the failure

#### Scenario: Exclusion without a reason

- **WHEN** a package is excluded from the list with no reason
- **THEN** the guard test SHALL fail

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
