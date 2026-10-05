# Spec Delta

## Purpose

Defines how dependency update pull requests are created, how they get CI, and under which narrow
conditions they merge without a human, so dependencies stay current without weakening the merge
gates.

## ADDED Requirements

### Requirement: DUA-1 Renovate pull requests start CI

`renovate.yaml` SHALL authenticate Renovate with an installation token minted by
`actions/create-github-app-token` from App credentials in repository secrets, and SHALL NOT fall
back to `github.token`.

#### Scenario: Token step present

- **WHEN** the Renovate workflow runs
- **THEN** the token passed to the Renovate action comes from the `create-github-app-token` step output

#### Scenario: Secrets missing

- **WHEN** the App secrets are not set
- **THEN** the job SHALL fail with a message naming the missing secret; it SHALL NOT proceed with `github.token`

#### Scenario: Bot PR gets CI

- **WHEN** Renovate opens a pull request
- **THEN** the required checks run on it

### Requirement: DUA-2 Bot merges use rebase only

Automerge of Renovate pull requests SHALL use `automergeStrategy: "rebase"`. No configuration in
the repository SHALL select squash or a merge commit for a bot PR.

#### Scenario: Eligible PR merges

- **WHEN** an eligible bot PR has all required checks green
- **THEN** it is rebase-merged and the commit history on `main` stays linear

#### Scenario: Squash or merge strategy configured

- **WHEN** `renovate.json` sets `automergeStrategy` to `squash` or `merge-commit`, or a workflow runs `gh pr merge --squash` or `--merge`
- **THEN** `renovate_automerge_test.sh` SHALL fail

#### Scenario: Checks red or pending

- **WHEN** a required check is red or pending
- **THEN** the PR SHALL NOT merge

### Requirement: DUA-3 Auto-merge is narrow

Auto-merge SHALL apply only to patch and minor updates from the `gomod`, `github-actions` and
`custom.regex` managers, and SHALL NOT apply to major updates, to `k8s.io/` or `sigs.k8s.io/`
modules, to the Dockerfile manager, or to the Go toolchain.

#### Scenario: Patch bump of a Go module

- **WHEN** Renovate proposes a patch update of a non-Kubernetes Go module older than the cooldown
- **THEN** the PR is marked for auto-merge

#### Scenario: Major bump

- **WHEN** Renovate proposes a major update of any dependency
- **THEN** the PR is NOT marked for auto-merge

#### Scenario: Kubernetes group

- **WHEN** Renovate updates `k8s.io/api`
- **THEN** the PR is NOT marked for auto-merge

#### Scenario: Rule order cannot widen scope

- **WHEN** a new `packageRules` entry matches `k8s.io/` after the automerge rule
- **THEN** the guard evaluates the final rule set and fails if `automerge` resolves true for a Kubernetes package or a major update

### Requirement: DUA-4 New releases wait seven days

Updates eligible for auto-merge SHALL set `minimumReleaseAge` to at least `7 days`.

#### Scenario: Release one day old

- **WHEN** a new version was published one day ago
- **THEN** Renovate does not create or merge the update yet

#### Scenario: Setting removed

- **WHEN** `minimumReleaseAge` is absent or below 7 days on an automerge rule
- **THEN** the guard SHALL fail

### Requirement: DUA-5 Bot credentials carry no bypass

The Renovate GitHub App SHALL NOT be on the `protect-main` ruleset bypass list and SHALL NOT be the
App used by `changelog-sync.yaml`.

#### Scenario: Same App id

- **WHEN** the Renovate workflow and `changelog-sync.yaml` reference the same secret names
- **THEN** the guard SHALL fail

#### Scenario: Bot PR with a failing check

- **WHEN** a bot PR has a failing required check
- **THEN** the merge is blocked exactly as for a human PR (no bypass path exists)
