# Spec Delta

## Purpose

Defines measurements and refusals that go beyond "the tests passed once": a mutation report, a
fuzz retry that cannot hide a crash, repeated runs of golden tests, and a README that matches what
the smoke test runs.

## ADDED Requirements

### Requirement: TDS-1 Mutation results are measured nightly and not gated

A nightly job SHALL run `gremlins` on `internal/aggregate`, `internal/redact` and
`internal/pathvalidate`, upload the report, and write killed, lived and not-covered counts per
package to the job summary. The job SHALL NOT fail on a low mutation score.

#### Scenario: Report produced

- **WHEN** the nightly runs
- **THEN** the summary shows per-package counts and the report artifact exists

#### Scenario: Low score is not a failure

- **WHEN** a package has surviving mutants
- **THEN** the job stays green

#### Scenario: Tool broken is a failure

- **WHEN** gremlins cannot run (build failure, panic, no report)
- **THEN** the job SHALL fail, because a missing measurement must not look like a clean one

### Requirement: TDS-2 Only the known fuzz-engine deadline artifact is retried

`hack/ci/fuzz-retry.sh` SHALL retry a failed fuzz run once only when its output contains
`context deadline exceeded` and contains no `panic:`, no `fatal error:`, no `Failing input written`,
no `--- FAIL`, no `signal: killed`, and `git status` shows no new file under `testdata/fuzz/`.
Every other failure SHALL fail the job at once.

#### Scenario: Pure deadline artifact

- **WHEN** the first attempt fails with only `fuzz: ... context deadline exceeded` and the second passes
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

### Requirement: TDS-3 Golden tests are run repeatedly

`task test:determinism` SHALL run the named golden tests with `-count=2`, and CI SHALL run it. The
list SHALL be in `Taskfile.yml` and SHALL include `TestMarshalExportEnvelopeGolden` and
`TestMarshalEventEnvelopeGolden`.

#### Scenario: Output depends on map order or time

- **WHEN** a golden test's output varies between two runs in one process
- **THEN** `task test:determinism` SHALL fail

#### Scenario: A named test does not run

- **WHEN** a listed test name matches nothing (renamed or deleted)
- **THEN** the task SHALL fail; `go test -v` must show `=== RUN` for every listed name

### Requirement: TDS-4 The README quick start names things that exist

`hack/test/readme_quickstart_test.sh` SHALL extract the commands of the README "Quick start"
section and fail when a `task <name>` is not defined in `Taskfile.yml`, a `kubectl apply -k <path>`
path does not exist, a short name in `kubectl get` is not a CRD `shortNames` entry in
`config/crd/bases`, or the first task's resolved script is not the one `hero-demo-smoke` runs.

#### Scenario: Task renamed

- **WHEN** `demo-up` is renamed and the README is not updated
- **THEN** the test SHALL fail and name the task

#### Scenario: Unknown short name

- **WHEN** the README says `kubectl get kinv,foo`
- **THEN** the test SHALL fail and name `foo`

#### Scenario: Section missing

- **WHEN** the README has no "Quick start" section
- **THEN** the test SHALL fail; an empty extraction never passes

#### Scenario: Smoke diverges from the README

- **WHEN** `hack/demo/hero/smoke.sh` stops calling the script that `task demo-up` resolves to
- **THEN** the test SHALL fail
