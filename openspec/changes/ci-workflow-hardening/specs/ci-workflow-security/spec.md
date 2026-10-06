# Spec Delta

## Purpose

Defines what the CI workflows must guarantee about their own supply chain, which gates must be
able to block a merge, and which CI triggers must be preserved so that release eligibility keeps
working.

## ADDED Requirements

### Requirement: CWS-1 Workflows are statically analysed offline

`ci.yaml` SHALL run `zizmor --offline` over `.github/` in a job named `workflow-security`, with a
pinned zizmor version, a pinned `--min-severity`, and the committed `.github/zizmor.yml`, and the
job SHALL fail on any unsuppressed finding.

#### Scenario: Clean tree

- **WHEN** no workflow has an unsuppressed finding
- **THEN** `workflow-security` passes

#### Scenario: New unsafe pattern

- **WHEN** a workflow interpolates `${{ github.event.pull_request.title }}` into a `run:` step
- **THEN** `workflow-security` SHALL fail

#### Scenario: Gate cannot be silenced by omission

- **WHEN** the job loses `--offline`, the version pin, `--min-severity`, the config path, or is removed
- **THEN** the meta-test SHALL fail

#### Scenario: Skip switch

- **WHEN** any workflow sets an environment variable or input that skips or disables the `workflow-security` job
- **THEN** the meta-test SHALL fail

### Requirement: CWS-2 Every zizmor suppression is justified, existing findings are fixed

Each suppression in `.github/zizmor.yml` SHALL be preceded by a comment stating why it is safe.
Findings that can be fixed in the workflow SHALL be fixed, not suppressed.

#### Scenario: Bare suppression

- **WHEN** a suppression has no comment
- **THEN** the meta-test SHALL fail and name the entry

#### Scenario: Fixable finding suppressed

- **WHEN** a suppression covers a pattern the workflow could simply stop using (for example a language cache in a publish workflow)
- **THEN** the reviewer rejects it; the review record notes each suppression's justification

### Requirement: CWS-3 Pull requests are dependency-reviewed

`ci.yaml` SHALL run `actions/dependency-review-action` in a job named `dependency-review` on
`pull_request` only, with the action's default severity threshold, a licence policy expressed
with `allow-licenses` (and `allow-dependencies-licenses` entries each with a reason), and unknown
licences reported without failing.

#### Scenario: PR adds a vulnerable dependency

- **WHEN** a PR adds a Go module with a known advisory at or above the default threshold
- **THEN** `dependency-review` SHALL fail

#### Scenario: Disallowed licence

- **WHEN** a PR adds a dependency whose licence is not in `allow-licenses`
- **THEN** `dependency-review` SHALL fail

#### Scenario: Threshold loosened

- **WHEN** `fail-on-severity` is set above the default without a `# why:` line citing a measured trial
- **THEN** the meta-test SHALL fail

#### Scenario: Deprecated input

- **WHEN** `deny-licenses` is used
- **THEN** the meta-test SHALL fail

#### Scenario: Not a release check

- **WHEN** `verify-eligibility.sh` lists required checks
- **THEN** `dependency-review` SHALL NOT be among them, because it does not run on `main`

### Requirement: CWS-4 Superseded pull-request runs are cancelled, every main push keeps its own run

`ci.yaml` and `e2e-smoke.yaml` SHALL declare `concurrency:` with group
`${{ github.workflow }}-${{ github.event.pull_request.number || github.sha }}` and
`cancel-in-progress: ${{ github.event_name == 'pull_request' }}`.

#### Scenario: Two pushes to one PR

- **WHEN** a second commit is pushed while the first run is in progress
- **THEN** the first run is cancelled

#### Scenario: Two workflows on one PR

- **WHEN** `ci.yaml` and `e2e-smoke.yaml` run for the same PR
- **THEN** neither cancels the other; a group without the workflow prefix SHALL fail the meta-test

#### Scenario: Three quick merges to main

- **WHEN** three commits land on `main` within one run's duration
- **THEN** each commit gets its own complete run (live evidence is post-merge)

#### Scenario: Group keyed by ref

- **WHEN** a group expression uses `github.ref` for push events in either workflow
- **THEN** the meta-test SHALL fail

### Requirement: CWS-5 CI keeps running on push to main

`ci.yaml` SHALL keep `push: branches: [main]` as a trigger, and `workflow-security` SHALL be in
`required_checks` of `hack/release/verify-eligibility.sh`.

#### Scenario: Trigger removed

- **WHEN** a change removes the `push` trigger or its `main` branch from `ci.yaml`
- **THEN** the meta-test SHALL fail with a message pointing to `verify-eligibility.sh`

#### Scenario: Documentation-only merge

- **WHEN** a merge touches only paths in the push `paths-ignore` list
- **THEN** no push run is expected, and the meta-test SHALL NOT require one

### Requirement: CWS-6 A guard that cannot block a merge is not a gate

Every guard script under `hack/test/` that CI runs, in every mode (plain, `--self-test`, and
any other mode its code accepts), SHALL be invoked on `pull_request` by a step of a job that
is a required check. The required set is `lint`, `vulncheck`, `workflow-security`,
`dependency-review` and the existing required contexts.

A guard reached only through a wrapper script (for example `hack/docs/verify.sh` via
`task docs:verify`) counts as invoked by CI and must ALSO be pinned by a required-job step.
Several guards may share one step (the `sonar_ko_*`/`dist_*` glob steps and the docs-side
group do); the requirement is the required-job invocation, not one step per script.

The repo's recognised flag mode is `--self-test`; `hack/test/ci_workflow_security_test.sh`
enforces its wiring. A guard adopting a NEW flag mode is a re-review event for the meta-test,
not something the mode detector can parse generically (a generic flag parser false-positives
on guard fixtures that embed foreign command lines).

#### Scenario: Guard in a non-required job

- **WHEN** a guard script is invoked only from a job outside the required set
- **THEN** the meta-test SHALL fail and name the script

#### Scenario: Mode not pinned

- **WHEN** a guard's `--self-test` mode is not run by any step
- **THEN** the meta-test SHALL fail

#### Scenario: Required set shrinks

- **WHEN** the declared required set in the meta-test no longer lists `lint`
- **THEN** the meta-test SHALL fail; the ruleset itself is checked by the operator (post-merge evidence)

### Requirement: CWS-7 Each new gate is proven able to fail

`hack/test/ci_workflow_security_test.sh` SHALL include a self-test that mutates a copy of the tree
to break each of CWS-1 to CWS-6 and asserts the check fails, plus one no-op mutation that must pass.

#### Scenario: Mutation survives

- **WHEN** removing `--offline` from the job does not make the meta-test fail
- **THEN** the self-test SHALL fail
