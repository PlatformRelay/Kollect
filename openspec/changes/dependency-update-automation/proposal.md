# Proposal

## Why

Dependency updates come from one bot: the self-hosted Renovate in
`.github/workflows/renovate.yaml`, authenticated with the `RENOVATE_TOKEN` secret (set since
2026-08-19). Its pull requests get full CI and merge by hand (#374 to #476). The hosted Renovate
App installed on the organisation has never opened a pull request here; it is unused, and the
maintainer removes it from the repository. This change no longer waits for a new bot App: it makes
the self-hosted Renovate that already works automated and trustworthy.

Re-measured on `main` @ `023313519` (2026-10-26):

- Schedule: weekly (`cron: "0 4 * * 1"`); GitHub started the last eight runs between 04:39 and
  11:37 UTC. Under the strict up-to-date rule a bot PR waits a week for each rebase.
- Token: `secrets.RENOVATE_TOKEN` with a silent `github.token` fallback. PRs opened under the
  fallback get no CI, so they can never pass the required checks. The job grants `contents: write`
  and `pull-requests: write` to `GITHUB_TOKEN` only for that fallback.
- The Renovate that runs is the floating `ghcr.io/renovatebot/renovate:44` tag (the action
  default, run log 39230872281), in a job that holds the maintainer's token.
- `renovate.json` extends `:disableDependencyDashboard`, so pending, aged and failed updates are
  invisible.
- Nothing in CI validates `renovate.json`; `docs/development/tooling-setup.md` gives a hand
  command with an unpinned `npx renovate`.
- No `automerge`. Most bot PRs are Docker image digest bumps (#448, #460, #461, #475), merged by
  hand days late.
- The `helm` group rule matches `depName` `helm/helm`, but Renovate extracts `azure/setup-helm`
  in `release.yaml` as `depName` `helm` (`packageName` `helm/helm`). So #385 moved every Helm pin
  except `release.yaml`.

## What Changes

- `renovate.yaml`: runs daily with a `concurrency` group; uses `RENOVATE_TOKEN` only and fails
  before Renovate starts when it is missing; the job's `GITHUB_TOKEN` drops to `contents: read`;
  Renovate runs from a tag-plus-digest image reference that Renovate updates itself; the
  `RENOVATE_FORCE` override that dropped every schedule on a manual run is removed.
- `renovate.json`: removes `:disableDependencyDashboard`; auto-merges only Docker image `digest`
  updates of the `dockerfile` and `devcontainer` managers, merged by Renovate after every check is
  green, rebase only; puts the Renovate image in a weekly group of its own, never auto-merged; the
  Helm rule matches on `packageName`.
- `hack/ci/renovate-config-validate.sh` runs `renovate-config-validator --strict` from the same
  image on `renovate.json` (repo mode) and `.github/renovate-config.json` (global mode) in the
  required `lint` job.
- `hack/test/renovate_policy_test.sh` (plain and `--self-test`, both in `lint`) guards all of it.
- `docs/development/tooling-setup.md` gets a maintainer section on how the bot works.

## Capabilities

### New Capabilities

- `dependency-updates`: how update PRs are created, validated and tested, and what the bot may
  merge on its own.

### Modified Capabilities

None.

## Impact

- Entry points: `.github/workflows/renovate.yaml`, `renovate.json`,
  `.github/renovate-config.json`, the `lint` job in `.github/workflows/ci.yaml`.
- The bot's identity and its secret are unchanged. The maintainer owns the secret and the token
  behind it.

## Dependencies

Change 6 of ten in the landing order. Its earlier prerequisites are met on `main`: `lint`,
`vulncheck`, `workflow-security` and `dependency-review` are required (ruleset 18802410, ten
required checks); `install-git-cliff.sh` verifies a checksum (change 5, archived
2026-10-16-developer-toolchain-pins); the changelog-sync checkout token is removed (formerly
DUA-6, archived 2026-10-16-changelog-sync-checkout-token).

## Non-goals

- A new GitHub App or token, or any change to secrets, rulesets or repository settings.
- Auto-merging Go modules, the Go toolchain version, GitHub Actions, pinned tools, docs packages or
  the Renovate image. Widening needs a later reviewed change, ideally once a bot identity without
  a ruleset bypass exists.
- GitHub native auto-merge (`platformAutomerge: true`).
- A "pause when `main` is red" step: a bot PR is rebased on `main` and runs the same required
  checks, so a red `main` already keeps it red (see design.md).
- Signed or re-attributed bot commits (`platformCommit`, `gitAuthor`).
- Removing the hosted App: that is the maintainer's action.

## Assumptions

Each item is believed until the task named for it records a probe.

- With `automerge: true` and `platformAutomerge: false`, Renovate merges a PR itself during a run,
  only when every status and check run on the head SHA is green. It treats a PR with no checks as
  pending, not green. It rebases a branch that is behind before merging it. It ends the run after
  one automerge. Sources: Renovate docs "automerge", "platformAutomerge", "rebaseWhen". Probe:
  task 0.4 (e), from the pinned Renovate's source.
- For a branch, `automerge` holds only when every upgrade in it is allowed to automerge. So a
  `go toolchain` group PR that also moves the `go` directive is not merged. Probe: task 0.4 (c), (e).
- Renovate's `github-actions` manager extracts `renovatebot/github-action`'s `renovate-image`
  input as a Docker dependency, with its tag and digest (Renovate 44.167.7,
  `known-actions/docker-dynamic.js`). The action uses a `renovate-image` that holds a `:` as the
  whole image reference from v46.3.10 on; v46.3.2, pinned today, does not (action `src/docker.ts`
  at both tags). Probe: task 0.4 (a), (d).
- `renovate-config-validator` ships in the Renovate image and `--strict` fails on a config that
  needs migration. It validates files named on the command line as global config unless
  `--no-global` is given (Renovate 44.167.7 `config-validator.js`). Probe: task 0.5.
- The dependency dashboard needs the token to write issues. The token is a maintainer token with
  admin rights; the first dashboard issue proves it. Post-merge row.
