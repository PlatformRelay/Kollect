# Spec Delta

## Purpose

Defines how the linter version is raised without weakening what it checks.

## ADDED Requirements

### Requirement: LTB-1 One linter version in both places

`GOLANGCI_LINT_VERSION` in `Makefile` and `version:` in `hack/tooling/.custom-gcl.yml` SHALL be
equal and at least v2.13.1.

#### Scenario: Custom build file lags

- **WHEN** `Makefile` says v2.13.1 and `.custom-gcl.yml` says v2.11.4
- **THEN** the change's pin-equality check (LTB-1: grep both files, diff) SHALL fail; the two
  never differ in a merged tree. A drift test red-flagging any mismatch is change 5's
  (developer-toolchain-pins, DTP-3, which names both sites), not this change's.

### Requirement: LTB-2 A bump does not loosen the linter

The bump SHALL NOT add a disabled linter or a blanket exclusion to `.golangci.yaml`. Each new
`//nolint` or exclusion SHALL carry a reason on the same or preceding line.

#### Scenario: Finding absorbed by disabling

- **WHEN** the diff disables a linter to make `task lint` pass
- **THEN** review SHALL reject it (gate-weakening needs its own justification line)

#### Scenario: Findings fixed

- **WHEN** v2.13.1 reports new findings
- **THEN** each is fixed or justified in commits separate from the version-only commit
  (grouped when one rule produces many findings); every new `//nolint` or exclusion carries a
  reason on the same or preceding line

### Requirement: LTB-3 The bump is observable

The version-only commit SHALL leave `task lint` runnable — the linter binary starts and reports
its findings (exit may be non-zero until task 1.3 lands) — and the review record SHALL state the
findings count before and after, with the number fixed and the number justified, and the version
the executed binary reports.

#### Scenario: Findings count recorded

- **WHEN** the change is reviewed
- **THEN** the record shows the count under v2.13.1 and the number fixed or justified, and the
  executed binary's reported version matches the pin

### Requirement: LTB-4 The plugin does not float

The logcheck plugin's `version:` in `hack/tooling/.custom-gcl.yml` SHALL be a pinned release
(not `latest`), recorded in the probe evidence, so the checker set does not drift between
builds. The pin moves with the linter bump.

#### Scenario: Plugin floats

- **WHEN** the plugin block names `version: latest`
- **THEN** the change's check SHALL fail; the resolved version is in the evidence
