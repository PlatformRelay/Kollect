# Spec Delta

## Purpose

Defines what the CI workflows must guarantee about their own supply chain (runner egress
visibility, static analysis of the workflows, dependency review on pull requests, superseded-run
cancellation) and which CI triggers must be preserved so that release eligibility keeps working.

## ADDED Requirements

### Requirement: CWS-1 Every job audits runner egress

Every job that has `steps:` in `.github/workflows/*.yaml` SHALL have `step-security/harden-runner`,
pinned by commit digest, as its first step with `egress-policy: audit`.

#### Scenario: Job with harden-runner first

- **WHEN** the meta-test scans the workflows
- **THEN** every job's first step uses `step-security/harden-runner` with `egress-policy: audit`

#### Scenario: A job without it

- **WHEN** a job is added whose first step is `actions/checkout`
- **THEN** the meta-test SHALL fail and name the workflow and job

#### Scenario: Block mode is not introduced here

- **WHEN** a job sets `egress-policy: block`
- **THEN** the meta-test SHALL fail until a later change specifies the allowlist

### Requirement: CWS-2 Workflows are statically analysed offline

`ci.yaml` SHALL run `zizmor --offline` over `.github/` in a job named `workflow-security`, using
the committed `.github/zizmor.yml`, and the job SHALL fail on any unsuppressed finding.

#### Scenario: Clean tree

- **WHEN** no workflow has an unsuppressed finding
- **THEN** `workflow-security` passes

#### Scenario: New unsafe pattern

- **WHEN** a workflow interpolates `${{ github.event.pull_request.title }}` into a `run:` step
- **THEN** `workflow-security` SHALL fail

#### Scenario: Gate cannot be silenced by omission

- **WHEN** the `workflow-security` job loses `--offline`, the config path, or is removed
- **THEN** the meta-test SHALL fail

### Requirement: CWS-3 Every zizmor suppression is justified

Each entry in `.github/zizmor.yml` that suppresses a finding SHALL be preceded by a comment that
states why it is safe.

#### Scenario: Suppression with a reason

- **WHEN** an ignore entry has a preceding `#` comment
- **THEN** the meta-test accepts it

#### Scenario: Bare suppression

- **WHEN** an ignore entry has no comment
- **THEN** the meta-test SHALL fail and name the entry

### Requirement: CWS-4 Pull requests are dependency-reviewed

`ci.yaml` SHALL run `actions/dependency-review-action` in a job named `dependency-review` on
`pull_request` only, with `fail-on-severity: high`.

#### Scenario: PR adds a vulnerable high-severity dependency

- **WHEN** a PR adds a Go module with a known high-severity advisory
- **THEN** `dependency-review` SHALL fail

#### Scenario: Not a release check

- **WHEN** `verify-eligibility.sh` lists required checks
- **THEN** `dependency-review` SHALL NOT be among them, because it does not run on `main`

### Requirement: CWS-5 Superseded pull-request runs are cancelled, main runs are not

`ci.yaml` SHALL declare `concurrency:` with a per-PR group and
`cancel-in-progress: ${{ github.ref != 'refs/heads/main' }}`.

#### Scenario: Two pushes to one PR

- **WHEN** a second commit is pushed while the first run is in progress
- **THEN** the first run is cancelled

#### Scenario: Two merges to main in quick succession

- **WHEN** a second commit lands on `main` while the first run is in progress
- **THEN** the first run SHALL NOT be cancelled

### Requirement: CWS-6 CI keeps running on push to main

`ci.yaml` SHALL keep `push: branches: [main]` as a trigger, and `workflow-security` SHALL be in
`required_checks` of `hack/release/verify-eligibility.sh`.

#### Scenario: Trigger removed

- **WHEN** a change removes the `push` trigger or its `main` branch from `ci.yaml`
- **THEN** the meta-test SHALL fail with a message pointing to `verify-eligibility.sh`

#### Scenario: Transition PR green, then main

- **WHEN** a PR is green, is rebase-merged, and the push run on `main` starts
- **THEN** the push run executes the same jobs and reports on the exact merge SHA

### Requirement: CWS-7 Each new gate is proven able to fail

`hack/test/ci_workflow_security_test.sh` SHALL include a self-test that mutates a copy of the
tree to break each of CWS-1 to CWS-6 and asserts the check fails, plus one no-op mutation that
must still pass.

#### Scenario: Mutation survives

- **WHEN** removing harden-runner from one job does not make the meta-test fail
- **THEN** the self-test SHALL fail
