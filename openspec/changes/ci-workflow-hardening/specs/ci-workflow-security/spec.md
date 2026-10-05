# Spec Delta

## Purpose

Defines what the CI workflows must guarantee about their own supply chain (runner egress
visibility, static analysis of the workflows, dependency review on pull requests, superseded-run
cancellation) and which CI triggers must be preserved so that release eligibility keeps working.

## ADDED Requirements

### Requirement: CWS-1 Every job audits runner egress

Every job that has `steps:` in `.github/workflows/*.yaml` SHALL have `step-security/harden-runner`,
pinned by commit digest, as its first step with `egress-policy: audit`, except the reporter and
classifier jobs that `hack/test/ci_docs_gate_test.sh` pins to an exact step count (`test` and
`changes` in `ci.yaml` and `e2e-smoke.yaml`, plus any other job that test pins; task 1.1 lists
them). The exemption list lives in the meta-test with that reason, because a third-party step in
those jobs could write `$GITHUB_ENV` and report a required context green.

#### Scenario: Job with harden-runner first

- **WHEN** the meta-test scans the workflows
- **THEN** every job's first step uses `step-security/harden-runner` with `egress-policy: audit`

#### Scenario: A job without it

- **WHEN** a job is added whose first step is `actions/checkout`
- **THEN** the meta-test SHALL fail and name the workflow and job

#### Scenario: Exempt job gets the action

- **WHEN** harden-runner is added to the `test` reporter job
- **THEN** both the meta-test and `ci_docs_gate_test.sh` SHALL fail

#### Scenario: Exemption list widened without reason

- **WHEN** a job name is added to the exemption list without a reason on the same line
- **THEN** the meta-test SHALL fail

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

### Requirement: CWS-5 Superseded pull-request runs are cancelled, every main push keeps its own run

`ci.yaml` and `e2e-smoke.yaml` SHALL declare `concurrency:` whose group is the workflow name (`${{ github.workflow }}-`) followed by the pull-request number
for `pull_request` events and the commit SHA otherwise, with
`cancel-in-progress: ${{ github.event_name == 'pull_request' }}`.

#### Scenario: Two pushes to one PR

- **WHEN** a second commit is pushed while the first run is in progress
- **THEN** the first run is cancelled

#### Scenario: Three quick merges to main

- **WHEN** three commits land on `main` within one run's duration
- **THEN** each commit gets its own complete run; none is cancelled and none is dropped as a superseded pending run

#### Scenario: Two workflows on one PR

- **WHEN** `ci.yaml` and `e2e-smoke.yaml` run for the same PR
- **THEN** neither cancels the other; a group expression without a per-workflow prefix (bare `PR number || sha`) SHALL fail the meta-test

#### Scenario: Group keyed by ref

- **WHEN** a group expression uses `github.ref` for push events in either workflow
- **THEN** the meta-test SHALL fail

### Requirement: CWS-6 CI keeps running on push to main

`ci.yaml` SHALL keep `push: branches: [main]` as a trigger, and `workflow-security` SHALL be in
`required_checks` of `hack/release/verify-eligibility.sh`.

#### Scenario: Trigger removed

- **WHEN** a change removes the `push` trigger or its `main` branch from `ci.yaml`
- **THEN** the meta-test SHALL fail with a message pointing to `verify-eligibility.sh`

#### Scenario: Transition PR green, then main

- **WHEN** a PR that changes code is green, is rebase-merged, and the push run on `main` starts
- **THEN** the push run executes the same jobs and reports on the exact merge SHA

#### Scenario: Documentation-only merge

- **WHEN** a merge touches only paths in the push `paths-ignore` list (`ci.yaml:33-42`)
- **THEN** no push run is expected; release eligibility applies to code commits, and the meta-test SHALL NOT require a push run for them

### Requirement: CWS-7 Each new gate is proven able to fail

`hack/test/ci_workflow_security_test.sh` SHALL include a self-test that mutates a copy of the
tree to break each of CWS-1 to CWS-6 and asserts the check fails, plus one no-op mutation that
must still pass.

#### Scenario: Mutation survives

- **WHEN** removing harden-runner from one job does not make the meta-test fail
- **THEN** the self-test SHALL fail
