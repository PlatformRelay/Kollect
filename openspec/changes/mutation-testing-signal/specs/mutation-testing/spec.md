# Spec Delta

## Purpose

Defines the nightly mutation measurement of kollect's most correctness-critical pure packages and
the rule that it measures and does not gate.

## ADDED Requirements

### Requirement: MUT-1 Mutation results are measured nightly, in their own workflow

A nightly workflow of its own SHALL run `gremlins` (one pinned version that supports the `go.mod`
Go version) on `internal/aggregate`, `internal/redact` and `internal/pathvalidate`, upload the
report, and write killed, lived and not-covered counts per package to the job summary.

#### Scenario: Report produced

- **WHEN** the nightly runs
- **THEN** the summary shows per-package counts and the report artifact exists

#### Scenario: Not part of the nightly suite

- **WHEN** the mutation workflow fails
- **THEN** its failure is reported as its own issue, separate from the `E2E nightly` issue

### Requirement: MUT-2 A low score is not a failure, a missing measurement is, and there is no gate

The workflow SHALL NOT fail on a low mutation score and SHALL fail when the tool cannot produce a
report. No threshold option SHALL appear on the gremlins command line in the workflow or the Taskfile.

#### Scenario: Surviving mutants

- **WHEN** a package has surviving mutants
- **THEN** the run stays green

#### Scenario: Tool broken

- **WHEN** gremlins cannot run (build failure, panic, no report)
- **THEN** the run SHALL fail, because a missing measurement must not look like a clean one

#### Scenario: Gate added by accident

- **WHEN** a threshold option such as `--threshold-efficacy` appears in the workflow or Taskfile command line
- **THEN** the guard test, which reads those files (the formatter never sees the arguments), SHALL fail

### Requirement: MUT-3 The measurement is proven able to see a surviving mutant

A fixture module SHALL contain a function whose test executes it without asserting on its result,
and the run on the fixture SHALL report that mutant as LIVED. Code that no test executes SHALL be
reported as NOT COVERED, and the control SHALL NOT rely on it.

#### Scenario: Executed but unasserted

- **WHEN** gremlins runs on the fixture
- **THEN** the control mutant is LIVED

#### Scenario: Control reported as not covered

- **WHEN** the control mutant is reported NOT COVERED
- **THEN** the control test SHALL fail (the fixture no longer executes the function)

### Requirement: MUT-4 Success to tool failure to recovery is visible

A night where the tool fails SHALL be distinguishable in the run from a night with surviving mutants.

#### Scenario: Night 1 ok, night 2 tool failure, night 3 ok

- **WHEN** the three nights run
- **THEN** night 2 is red with the tool error, nights 1 and 3 are green, each with its own run's report
