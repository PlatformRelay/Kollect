# Spec Delta

## Purpose

Defines how dependency update pull requests are created, how they get CI, and under which narrow
conditions they merge without a human, so dependencies stay current without weakening the merge
gates or exposing privileged credentials.

## ADDED Requirements

### Requirement: DUA-1 Renovate pull requests start CI and run daily

`.github/workflows/renovate.yaml` SHALL authenticate Renovate with an installation token minted by
`actions/create-github-app-token` from App credentials in repository secrets, SHALL NOT fall back to
`github.token`, and SHALL run at least daily.

#### Scenario: Token step present

- **WHEN** the Renovate workflow runs
- **THEN** the token passed to the Renovate action comes from the `create-github-app-token` step output

#### Scenario: Secrets missing

- **WHEN** the App secrets are not set
- **THEN** the job SHALL fail naming the missing secret; it SHALL NOT proceed with `github.token`

#### Scenario: Schedule weakened

- **WHEN** the cron is changed to run less often than daily
- **THEN** the guard SHALL fail

### Requirement: DUA-2 Bot merges use rebase only, after every check is green

Renovate SHALL merge with `automergeStrategy: "rebase"` and `platformAutomerge: false`. No
configuration SHALL select squash or a merge commit for a bot PR.

#### Scenario: Eligible PR merges

- **WHEN** an eligible bot PR has every check run on its head SHA green and is up to date
- **THEN** it is rebase-merged

#### Scenario: Any check red or pending

- **WHEN** any check run on the head SHA is red or pending, required or not
- **THEN** the PR SHALL NOT merge

#### Scenario: Squash or merge strategy configured

- **WHEN** `renovate.json` sets `automergeStrategy` to `squash` or `merge-commit`, or a workflow runs `gh pr merge --squash` or `--merge`
- **THEN** the guard SHALL fail

#### Scenario: PR behind main

- **WHEN** a bot PR is behind `main` under the up-to-date rule
- **THEN** it is not merged until Renovate rebases it at a later run, and the rebased head must pass checks again

### Requirement: DUA-3 Auto-merge is narrow

Auto-merge SHALL apply only to `patch` and `minor` updates of Go modules from the `gomod` manager,
and SHALL NOT apply to `major` updates, `k8s.io/` and `sigs.k8s.io/` modules, the `go` directive,
the Dockerfile manager, any `github-actions` update, any `custom.regex` pinned tool, or any
`digest` or `pinDigest` update. The deny rules SHALL be the last entries of `packageRules`.

#### Scenario: Patch bump of a Go module

- **WHEN** Renovate proposes a patch update of a non-Kubernetes Go module older than the cooldown
- **THEN** the PR is eligible for auto-merge

#### Scenario: Action or tool bump

- **WHEN** Renovate proposes any update of a GitHub Action or a pinned tool (Task, git-cliff, Helm, golangci-lint, gitleaks)
- **THEN** the PR is NOT eligible, because those run in jobs holding privileged tokens or in release workflows

#### Scenario: Major bump, Kubernetes group, digest

- **WHEN** Renovate proposes a major update, a `k8s.io/` or `sigs.k8s.io/` update, or a digest update
- **THEN** the PR is NOT eligible

#### Scenario: Rule appended after the deny rules

- **WHEN** a `packageRules` entry is added after the deny rules
- **THEN** the `jq` guard SHALL fail

### Requirement: DUA-4 New releases wait seven days

Updates eligible for auto-merge SHALL set `minimumReleaseAge` to at least `7 days`. No eligible
update SHALL be exempt from the age rule.

#### Scenario: Release one day old

- **WHEN** a new version was published one day ago
- **THEN** Renovate does not create or merge the update yet

#### Scenario: Setting removed

- **WHEN** `minimumReleaseAge` is absent or below 7 days on an automerge rule
- **THEN** the guard SHALL fail

### Requirement: DUA-5 Bot credentials carry no bypass and are not the changelog App

The Renovate App SHALL NOT be on the `protect-main` bypass list, and SHALL be a different App from
the one used by `changelog-sync.yaml`, compared by App slug.

#### Scenario: Same App slug

- **WHEN** the token's `app-slug` equals the changelog-sync App slug variable
- **THEN** the Renovate workflow SHALL fail before running Renovate

#### Scenario: Same secret names

- **WHEN** both workflows reference the same secret names
- **THEN** the guard SHALL fail

#### Scenario: Bot PR with a failing check

- **WHEN** a bot PR has a failing required check
- **THEN** the merge is blocked as for a human PR; no bypass path exists

### Requirement: DUA-6 The privileged token is isolated in changelog-sync

In `changelog-sync.yaml` the checkout SHALL set `persist-credentials: false`, and the App token SHALL
be passed only to the step that pushes.

#### Scenario: Credentials persist

- **WHEN** the checkout step omits `persist-credentials: false` or receives the App token
- **THEN** the guard SHALL fail

#### Scenario: Token reaches an earlier step

- **WHEN** a step before the push references the App token output
- **THEN** the guard SHALL fail

### Requirement: DUA-7 Auto-merge pauses on a red main and has no pull_request_target variant

The Renovate run SHALL set `automerge: false` when the latest push run of `ci.yaml` on `main` is not
`success`. No workflow SHALL merge bot PRs from a `pull_request_target` trigger.

#### Scenario: Main red

- **WHEN** the latest push CI on `main` failed or is in progress
- **THEN** the Renovate run creates and updates PRs but merges none

#### Scenario: Main recovers

- **WHEN** main goes red, then green
- **THEN** the next run merges eligible PRs again (transition red to green)

#### Scenario: Lookup fails

- **WHEN** the run lookup errors
- **THEN** the run behaves as red (no merges)

#### Scenario: pull_request_target merge workflow added

- **WHEN** a workflow combines `pull_request_target` with a merge command
- **THEN** the guard SHALL fail

### Requirement: DUA-8 Hard prerequisites are met before auto-merge is enabled

`automerge` SHALL NOT be enabled in `renovate.json` until the ruleset requires `lint`, `vulncheck`,
`workflow-security` and `dependency-review`, and `install-git-cliff.sh` verifies a checksum.

#### Scenario: Enabled early

- **WHEN** the change that enables `automerge` is reviewed
- **THEN** the review record cites the operator's evidence of the ruleset and the merged installer change
