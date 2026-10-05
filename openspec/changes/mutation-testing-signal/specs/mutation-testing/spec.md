# Spec Delta

## Purpose

Defines the nightly mutation measurement of kollect's most correctness-critical pure packages and
the rule that it measures and does not gate.

## ADDED Requirements

### Requirement: MUT-1 Mutation results are measured nightly

A nightly job SHALL run `gremlins` on `internal/aggregate`, `internal/redact` and
`internal/pathvalidate`, upload the report, and write killed, lived and not-covered counts per
package to the job summary.

#### Scenario: Report produced

- **WHEN** the nightly runs
- **THEN** the summary shows per-package counts and the report artifact exists

### Requirement: MUT-2 A low score is not a failure, a missing measurement is

The job SHALL NOT fail on a low mutation score, and SHALL fail when the tool cannot produce a
report.

#### Scenario: Surviving mutants

- **WHEN** a package has surviving mutants
- **THEN** the job stays green

#### Scenario: Tool broken

- **WHEN** gremlins cannot run (build failure, panic, no report)
- **THEN** the job SHALL fail, because a missing measurement must not look like a clean one

#### Scenario: Gate added by accident

- **WHEN** a threshold option (`--threshold-efficacy` or similar) appears on the gremlins command line
- **THEN** the formatter's test SHALL fail

### Requirement: MUT-3 Success to tool failure to recovery is visible

A night where the tool fails SHALL be distinguishable in the run from a night with surviving mutants.

#### Scenario: Night 1 ok, night 2 tool failure, night 3 ok

- **WHEN** the three nights run
- **THEN** night 2 is red with the tool error, nights 1 and 3 are green, each with its own run's report
