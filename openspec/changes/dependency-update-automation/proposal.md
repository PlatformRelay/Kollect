# Proposal

## Why

Self-hosted Renovate runs weekly (`.github/workflows/renovate.yaml`, `cron: "0 4 * * 1"`) with
`secrets.RENOVATE_TOKEN` when set and `github.token` otherwise (last line of the file). No
`RENOVATE_TOKEN` exists, so Renovate pushes `renovate/*` branches, cannot open pull requests the
way CI expects, and any PR it does open by that token does not start workflows (GitHub does not
trigger workflows for events caused by `GITHUB_TOKEN`). The result is branches nobody merges and
dependency drift that only the manual security sweep catches. `renovate.json` has no
`automerge`, no `minimumReleaseAge` and no schedule; every update is manual.

## What Changes

- `renovate.yaml` mints a short-lived installation token with `actions/create-github-app-token`
  (already pinned in `changelog-sync.yaml:74`) from a NEW GitHub App dedicated to Renovate, and
  passes it to the Renovate action. The `github.token` fallback is removed.
- `renovate.json` enables auto-merge for patch and minor updates of Go modules, the `github-actions`
  manager (digest pins and minor/patch) and the pinned-tools group, with
  `minimumReleaseAge: "7 days"`, and with `automergeStrategy: "rebase"`. Major updates and the
  Kubernetes module group are never auto-merged.
- A guard (`hack/test/renovate_automerge_test.sh`) proves the config cannot select a squash or
  merge-commit strategy, scopes auto-merge as specified, and keeps the token step wired.

## Capabilities

### New Capabilities

- `dependency-updates`: how dependency update PRs are created, tested and merged.

### Modified Capabilities

None.

## Impact

- Entry points: `.github/workflows/renovate.yaml` (token step), `renovate.json` (rules), and the
  repository's required checks, which gate every bot PR exactly as human PRs.
- Operator-owned prerequisites are listed in tasks section 4 (App creation, secrets, repo setting
  "Allow auto-merge", rebase-merge enabled, ruleset review).

## Non-goals

- Replacing Renovate with Dependabot (open question in `developer-toolchain-consistency`).
- Auto-merging major updates, Kubernetes modules, Dockerfile base images or the Go toolchain.
- Changing which checks are required.
- Reusing the changelog-sync App (see design.md).

## Assumptions

- Pull requests and pushes made with a GitHub App installation token DO trigger workflows (the
  exception is `GITHUB_TOKEN`). Source: GitHub docs, "Triggering a workflow from a workflow".
  Probe in task 3.1: the first Renovate PR shows a CI run.
- Renovate's `platformAutomerge: true` with `automergeStrategy: "rebase"` enables GitHub
  auto-merge using the rebase method; it needs "Allow auto-merge" on the repository and merges only
  after required checks pass. Source: Renovate docs, "automerge" and "automergeStrategy". Probe in
  task 3.2 on the first eligible PR.
- Renovate editing files under `.github/workflows/` needs the App to hold the `workflows`
  permission; without it the push is rejected. Probe in task 3.1.
- The existing App behind `changelog-sync.yaml` is on the `protect-main` bypass list with
  `bypass_mode: always` (`changelog-sync.yaml:18-19`). Reusing it would give the bot a path
  around required checks.
