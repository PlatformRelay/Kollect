# breaking-change-marking Specification

## Purpose
Defines how a pull request's breaking-change declaration is held to the commit titles that
git-cliff reads, given that pull requests land by rebase merge.

## Requirements

### Requirement: BMC-1 A declared breaking change needs a marked commit

`hack/check-breaking-marker.sh <base> <head>` SHALL exit non-zero when the PR title (`PR_TITLE`)
has a conventional breaking marker (`type!:` or `type(scope)!:`, after any gitmoji shortcode), or
the PR body (`PR_BODY`) has a `BREAKING CHANGE:` or `BREAKING-CHANGE:` footer line outside a fenced
code block (a fence closes only with the character that opened it), and no commit title in
`<base>..<head>` (merges excluded, since a rebase merge drops them) has a breaking marker.

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

#### Scenario: Mentions are not declarations

- **GIVEN** the body mentions `BREAKING CHANGE:` mid-line, or inside a fenced code block, and the
  title has a `!` that is not the conventional marker (for example `fix: handle a!: prefix`)
- **WHEN** the script runs with plain commits
- **THEN** it SHALL NOT fail

### Requirement: BMC-1.2 A marked commit passes, read as git-cliff reads it, and failures name the trigger

On a commit title the optional shortcode must be written exactly as git-cliff's preprocessor
strips it (`^:[a-z0-9_]+:` plus one space); a title it cannot parse does not count. The message
SHALL name which declaration triggered the failure. When a commit title carries the marker, the
script SHALL pass.

#### Scenario: Declared and marked

- **GIVEN** the PR title has `!` and one commit title is `:sparkles: feat(sink)!: …`
- **WHEN** the script runs
- **THEN** it exits 0

#### Scenario: Commit marker git-cliff cannot parse

- **GIVEN** the PR title is `feat(api)!: drop the engine field`
- **AND** the only marked commit titles are `:sparkles:  feat(api)!: …` (two spaces after the shortcode) and `:+1: fix(api)!: …`
- **WHEN** the script runs
- **THEN** it exits 1 and names the PR title

### Requirement: BMC-2 An undeclared marked commit is a warning

When a commit title in the range has a breaking marker and the PR title has none, the script SHALL
print a GitHub `::warning::` naming the commit, with `%`, CR and LF encoded as workflow-command
data, and SHALL exit 0. When the PR title has the marker, it SHALL NOT warn.

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
permissions SHALL be `contents: read` only.

#### Scenario: Fork-unsafe trigger or write permission

- **WHEN** the workflow triggers on `pull_request_target`, or its permissions grant `contents: write`
- **THEN** `hack/test/breaking_marker_test.sh` fails

### Requirement: BMC-4.2 The job uses only a pinned checkout, no secrets and no overrides

The `breaking-marker` job of `.github/workflows/breaking-marker.yaml` SHALL use no action but a
SHA-pinned `actions/checkout`, reference no secrets in any spelling, declare no `if`,
`continue-on-error`, `permissions` or `env` on the job or its steps beyond the check step's
`PR_TITLE` and `PR_BODY`, and check out with `persist-credentials: false` and no `ref` or
`repository` override.

#### Scenario: Unpinned checkout, persisted credentials or a secret

- **WHEN** the checkout step is not pinned to a commit SHA, or sets `persist-credentials: true`, or the job references `secrets`
- **THEN** `hack/test/breaking_marker_test.sh` fails

### Requirement: BMC-4.3 The job runs only the script and passes PR text through env

The `breaking-marker` job SHALL run on `ubuntu-latest` with exactly two steps whose check runs
exactly `bash hack/check-breaking-marker.sh HEAD^1 HEAD^2`, pass the PR title and body to the
script through step `env:`, and SHALL NOT reference `github.event.pull_request` inside any `run:`
body. The job SHALL NOT be listed in the required checks of `hack/release/verify-eligibility.sh`.

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
