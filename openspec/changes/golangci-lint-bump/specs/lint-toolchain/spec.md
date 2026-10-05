# Spec Delta

## Purpose

Defines how the linter version is raised without weakening what it checks.

## ADDED Requirements

### Requirement: LTB-1 One linter version in both places

`GOLANGCI_LINT_VERSION` in `Makefile` and `version:` in `hack/tooling/.custom-gcl.yml` SHALL be
equal and at least v2.13.1.

#### Scenario: Custom build file lags

- **WHEN** `Makefile` says v2.13.1 and `.custom-gcl.yml` says v2.11.4
- **THEN** `task lint` SHALL fail or the change's check SHALL fail; the two never differ in a merged tree

### Requirement: LTB-2 A bump does not loosen the linter

The bump SHALL NOT add a disabled linter or a blanket exclusion to `.golangci.yaml`. Each new
`//nolint` or exclusion SHALL carry a reason on the same or preceding line.

#### Scenario: Finding absorbed by disabling

- **WHEN** the diff disables a linter to make `task lint` pass
- **THEN** review SHALL reject it (gate-weakening needs its own justification line)

#### Scenario: Findings fixed

- **WHEN** v2.13.1 reports new findings
- **THEN** each is fixed or justified in its own commit, separate from the version-only commit

### Requirement: LTB-3 The bump is observable

The version-only commit SHALL leave `task lint` runnable, and the review record SHALL state the
findings count before and after.

#### Scenario: Findings count recorded

- **WHEN** the change is reviewed
- **THEN** the record shows the count under v2.13.1 and the number fixed or justified
