# Spec Delta

## Purpose

Defines how a pull request's breaking-change declaration is held to the commit titles that
git-cliff reads, given that pull requests land by rebase merge.

## ADDED Requirements

### Requirement: BMC-1 A declared breaking change needs a marked commit

`hack/check-breaking-marker.sh <base> <head>` SHALL exit non-zero when the PR title (`PR_TITLE`)
has a conventional breaking marker (`type!:` or `type(scope)!:`, after an optional gitmoji
shortcode), or the PR body (`PR_BODY`) has a `BREAKING CHANGE:` or `BREAKING-CHANGE:` footer line
outside a fenced code block, and no commit title in `<base>..<head>` (merges excluded) has a
breaking marker. The message SHALL name which declaration triggered the failure. When a commit
title carries the marker, the script SHALL pass.

#### Scenario: Marked PR title, plain commits

- **GIVEN** the PR title is `:lock: fix(sink)!: honour tls.caSecretRef`
- **AND** no commit title has `!`
- **WHEN** the script runs
- **THEN** it exits 1 and names the PR title

#### Scenario: Body footer, plain commits

- **GIVEN** the PR title has no marker and the body has a line starting `BREAKING CHANGE:`
- **AND** no commit title has `!`
- **WHEN** the script runs
- **THEN** it exits 1 and names the PR body

#### Scenario: Declared and marked

- **GIVEN** the PR title has `!` and one commit title is `:sparkles: feat(sink)!: …`
- **WHEN** the script runs
- **THEN** it exits 0

#### Scenario: Mentions are not declarations

- **GIVEN** the body mentions `BREAKING CHANGE:` mid-line, or inside a fenced code block, and the
  title has a `!` that is not the conventional marker (for example `fix: handle a!: prefix`)
- **WHEN** the script runs with plain commits
- **THEN** it SHALL NOT fail

### Requirement: BMC-2 An undeclared marked commit is a warning

When a commit title in the range has a breaking marker and the PR title has none, the script SHALL
print a GitHub `::warning::` naming the commit and SHALL exit 0.

#### Scenario: Marked commit under a plain title

- **GIVEN** the PR title is `:sparkles: feat(sink): x` and a commit title is `feat(sink)!: x`
- **WHEN** the script runs
- **THEN** it prints `::warning::` with the commit title
- **AND** it SHALL NOT exit non-zero

### Requirement: BMC-3 The script fails closed on bad input

The script SHALL exit 2, and SHALL NOT report success, when it gets the wrong number of arguments,
an unknown revision, or no `PR_TITLE` in its environment. An empty range with no declaration
passes.

#### Scenario: Unknown base

- **WHEN** the base revision does not exist
- **THEN** the script exits 2

### Requirement: BMC-4 The workflow is fork-safe and least-privilege

`.github/workflows/breaking-marker.yaml` SHALL trigger on `pull_request` with the types `opened`,
`edited`, `synchronize` and `reopened`, and SHALL NOT trigger on `pull_request_target`. Its
permissions SHALL be `contents: read` only. Its job `breaking-marker` SHALL use only SHA-pinned
actions, check out with `persist-credentials: false`, pass the PR title and body to the script
through step `env:`, and SHALL NOT reference `github.event.pull_request` inside any `run:` body.
The job SHALL NOT be listed in the required checks of `hack/release/verify-eligibility.sh`.

#### Scenario: PR text inlined into shell

- **WHEN** a `run:` body references `${{ github.event.pull_request.title }}`
- **THEN** `hack/test/breaking_marker_test.sh` fails naming the injection

### Requirement: BMC-5 The behaviour is guarded in a required job

`hack/test/breaking_marker_test.sh` SHALL run the BMC-1 to BMC-3 fixture cases against the script
on throwaway repositories and check BMC-4 statically. Its `--self-test` SHALL show one mutant per
clause turning its own case red with its own message, plus a no-op control that stays green. Both
modes SHALL run in the required `lint` job, enforced by CFC-6 in
`hack/test/ci_workflow_security_test.sh`.

#### Scenario: Self-test mode is unwired

- **GIVEN** the `--self-test` step is removed from `ci.yaml`
- **WHEN** `hack/test/ci_workflow_security_test.sh` runs
- **THEN** it fails naming `breaking_marker_test.sh`
