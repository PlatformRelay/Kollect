# Spec Delta

## Purpose

Defines the standard task entry points a contributor can rely on and what each promises.

## ADDED Requirements

### Requirement: TCE-1 `task check` is the full local gate

`task check` SHALL run every required CI gate that can run on a developer machine, SHALL run
every `hack/test` guard in every mode its header declares (including `--self-test`), and SHALL
print the required gates it does not run, each with its reason.

#### Scenario: Everything runnable runs

- **WHEN** a contributor runs `task check` on a machine without Docker
- **THEN** it runs verify, lint, vulncheck, unit tests, scrub, the shell and markdown lints, spec validation and every guard, and prints the Docker-dependent gates as not run

#### Scenario: A new guard appears

- **WHEN** a new `hack/test/*_test.sh` is added
- **THEN** `task check` runs it without an edit to the Taskfile

#### Scenario: A gate fails

- **WHEN** one gate fails
- **THEN** `task check` exits non-zero after reporting which gate failed; it SHALL NOT report success because the last gate passed

### Requirement: TCE-2 `task verify` keeps its meaning

`task verify` SHALL remain the generated-artifact drift check and SHALL NOT be redefined as a
subset or a superset of `check`.

#### Scenario: Redefinition attempted

- **WHEN** `verify` is removed or changed to run lint or tests
- **THEN** the guard SHALL fail

### Requirement: TCE-3 No required CI gate is silently missing locally

Every job in `verify-eligibility.sh` `required_checks` SHALL be reachable from `task check` or
listed as an exclusion with a reason.

#### Scenario: Gate without a local equivalent

- **WHEN** a required job is neither reachable from `check` nor listed as an exclusion
- **THEN** the guard SHALL fail and name the job

#### Scenario: Exclusion without reason

- **WHEN** an exclusion has no reason
- **THEN** the guard SHALL fail

### Requirement: TCE-4 The guard can fail

`hack/test/task_check_test.sh` SHALL carry throwaway-copy mutants for TCE-1 to TCE-3 and a no-op
copy that passes, and SHALL run on `pull_request` in a required job.

#### Scenario: Mutant survives

- **WHEN** removing a gate from `check` does not make the guard fail
- **THEN** the self-test SHALL fail
