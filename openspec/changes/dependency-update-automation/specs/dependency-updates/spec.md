# Spec Delta

## Purpose

Defines how dependency update pull requests are created, validated and tested, and the narrow
case in which the bot merges one without a human, so dependencies stay current without weakening
the merge gates. DUA-6 (the changelog-sync checkout token) was split out and is archived in
`2026-10-16-changelog-sync-checkout-token`; the ID is not reused.

## ADDED Requirements

### Requirement: DUA-1 One self-hosted bot, its token only, fail closed

`.github/workflows/renovate.yaml` SHALL be the only dependency-update bot workflow. It SHALL pass
`secrets.RENOVATE_TOKEN` to Renovate and nothing else, SHALL fail before Renovate starts with an
error naming the secret when it is empty, and its job SHALL grant `GITHUB_TOKEN` no more than
`contents: read`. The maintainer docs SHALL record that the hosted Renovate App is unused.

#### Scenario: Token present

- **WHEN** the Renovate workflow runs with `RENOVATE_TOKEN` set
- **THEN** the Renovate step receives that secret as its token

#### Scenario: Token missing

- **WHEN** `RENOVATE_TOKEN` is empty
- **THEN** the job fails before the Renovate step with a message naming `RENOVATE_TOKEN`, and SHALL NOT run Renovate with `github.token`

#### Scenario: Fallback or write grant reintroduced

- **WHEN** the workflow references `github.token` or `secrets.GITHUB_TOKEN` for Renovate, or grants `GITHUB_TOKEN` any write permission
- **THEN** the guard SHALL fail

### Requirement: DUA-2 Daily runs, one at a time

The Renovate workflow SHALL run on a schedule at least once a day and on `workflow_dispatch`, and
SHALL declare a `concurrency` group with `cancel-in-progress: false`.

#### Scenario: Daily schedule

- **WHEN** the workflow's `schedule` is read
- **THEN** some cron entry fires every day

#### Scenario: Schedule weakened or overlap allowed

- **WHEN** every cron entry fires less than daily, or the concurrency group is removed or set to cancel in progress
- **THEN** the guard SHALL fail

### Requirement: DUA-3 The Renovate that runs is pinned and never self-merged

The Renovate step SHALL use `renovatebot/github-action` at v46.3.10 or later with a
`renovate-image` of the form `ghcr.io/renovatebot/renovate:<version>@sha256:<64 hex>`.
`renovate.json` SHALL group that image alone on a weekly schedule and SHALL NOT auto-merge it.

#### Scenario: Pinned image

- **WHEN** the workflow is read
- **THEN** `renovate-image` carries an exact version and a digest, and no `renovate-version` input is set

#### Scenario: Floating or unpinned image

- **WHEN** `renovate-image` is removed, has no digest, or uses a floating tag such as `44` or `latest`
- **THEN** the guard SHALL fail

#### Scenario: Image bump PR

- **WHEN** Renovate proposes a new Renovate image
- **THEN** it opens its own PR at most weekly, and that PR is not auto-merged

### Requirement: DUA-4 The dependency dashboard is on

`renovate.json` SHALL NOT extend `:disableDependencyDashboard` or set `dependencyDashboard` to
false, so Renovate keeps one dashboard issue listing pending, rate-limited, awaiting-age and
errored updates.

#### Scenario: Dashboard issue after a run

- **WHEN** Renovate completes a run on `main`
- **THEN** the repository has an open "Dependency Dashboard" issue authored by the token's user

#### Scenario: Dashboard disabled again

- **WHEN** the preset or the setting that disables the dashboard is added
- **THEN** the guard SHALL fail

### Requirement: DUA-5 The Renovate config is validated in a required job

The required `lint` job SHALL run `hack/ci/renovate-config-validate.sh`. The script SHALL take the
image reference from `renovate.yaml`, SHALL refuse one without a digest, and SHALL run
`renovate-config-validator --strict` from that image on `renovate.json` and
`.github/renovate-config.json`, failing when the validator does.

#### Scenario: Valid config

- **WHEN** both config files are valid and need no migration
- **THEN** the step passes

#### Scenario: Invalid or outdated config

- **WHEN** `renovate.json` has an unknown option, a wrong type, or a setting the validator would migrate
- **THEN** the `lint` job fails

#### Scenario: Validation unwired or unpinned

- **WHEN** the `lint` step is removed, made conditional or allowed to fail, or the script would run an image without a digest
- **THEN** the guard SHALL fail

### Requirement: DUA-7 Only Docker image digests auto-merge

Exactly one `packageRules` entry SHALL set `automerge: true`. It SHALL match only `matchManagers`
`dockerfile` and `devcontainer` and `matchUpdateTypes` `digest`. No other rule, top-level
setting or `extends` preset SHALL enable automerge. A branch holding any other update SHALL NOT
auto-merge.

#### Scenario: Base image digest bump

- **WHEN** Renovate proposes only a new digest for a Dockerfile or devcontainer image
- **THEN** the PR is eligible for auto-merge

#### Scenario: Anything else

- **WHEN** Renovate proposes a Go module, the Go toolchain version, a GitHub Action, a pinned tool, a docs package, the Renovate image or a Docker tag change, or a group mixing one of those with a digest
- **THEN** the PR is NOT auto-merged

#### Scenario: Scope widened

- **WHEN** a second rule sets `automerge: true`, the automerge rule gains a manager or update type, or an automerge preset or top-level `automerge` is added
- **THEN** the guard SHALL fail

### Requirement: DUA-8 Bot merges wait for every check and rebase only

`renovate.json` SHALL set `platformAutomerge: false`, `automergeType: "pr"`,
`automergeStrategy: "rebase"` and `rebaseWhen: "behind-base-branch"`, and SHALL NOT set
`ignoreTests`. No workflow SHALL merge bot PRs, and none SHALL combine `pull_request_target` with
a merge command.

#### Scenario: Eligible PR all green

- **WHEN** an eligible PR is up to date and every check on its head SHA is green
- **THEN** Renovate rebase-merges it during a run

#### Scenario: Check red, pending or behind

- **WHEN** any check on the head SHA is red or pending, required or not, or the PR is behind `main`
- **THEN** it is not merged in that run; a behind PR is rebased and must pass again

#### Scenario: Setting weakened

- **WHEN** `platformAutomerge` is true or absent, the strategy is not `rebase`, `ignoreTests` is set, or a workflow merges bot PRs
- **THEN** the guard SHALL fail

### Requirement: DUA-9 Every Helm pin moves in one group

The Helm group rule in `renovate.json` SHALL match `matchPackageNames` `helm/helm`, so the
custom-manager pins and the `azure/setup-helm` input in `release.yaml` land in one PR.

#### Scenario: Helm release

- **WHEN** Renovate finds a new `helm/helm` release
- **THEN** one branch, `renovate/helm`, updates every Helm pin including `release.yaml`

#### Scenario: Rule matches depName only

- **WHEN** the rule matches by `matchDepNames` `helm/helm` alone
- **THEN** the guard SHALL fail

### Requirement: DUA-10 Maintainer docs describe the bot

`docs/development/tooling-setup.md` SHALL describe the schedule, the token and its owner, the
dashboard, what auto-merges and why, how to pause the bot, how to rebase a stuck PR, how to
validate the config, that the hosted Renovate App is unused, and that the maintainer reviews bot
merges since the last tag before approving a release.

#### Scenario: Docs match the config

- **WHEN** the docs section is read against `renovate.yaml` and `renovate.json`
- **THEN** each statement about schedule, token, scope and dashboard matches them

#### Scenario: Docs drift

- **WHEN** the docs section stops naming the auto-merge scope or the schedule
- **THEN** the guard SHALL fail
