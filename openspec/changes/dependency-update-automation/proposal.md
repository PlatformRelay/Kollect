# Proposal

## Why

Self-hosted Renovate runs weekly (`.github/workflows/renovate.yaml`, `cron: "0 4 * * 1"`) with
`secrets.RENOVATE_TOKEN` when set and `github.token` otherwise (last line of the file).
Re-measured 2026-10-16: `RENOVATE_TOKEN` has been set since 2026-08-19 to the maintainer's admin
PAT, and Renovate pull requests (#374 to #461) get full CI under it. The admin role is a
`protect-main` bypass actor (`bypass_mode: pull_request`), so a Renovate merge under that PAT could
bypass required checks; automerge under it would break DUA-5 ("no bypass path exists"). The
dedicated App below is therefore still needed; the operator will create it (2026-10-16), and this
change stays parked until then. `renovate.json` has no `automerge`, no `minimumReleaseAge` and no
schedule; every update is manual.

Auto-merging bot PRs is only safe if the gates can block them, and if nothing a bot merges runs
with a privileged token. Probed on this tree:

- The `protect-main` ruleset (re-measured 2026-10-16) requires `preflight`, `test`, `kind-smoke`,
  `Analyze (Go)`, `lint`, `workflow-security`, `dependency-review`, `tool-pins-selftest` and
  `vulncheck`. The original finding (only the first four required) is resolved.
- `changelog-sync.yaml` mints the token of an App on the `protect-main` bypass list
  (`bypass_mode: always`, lines 18-19) and runs `go-task/setup-task` and git-cliff in the same job.
  Its checkout already sets `persist-credentials: false` on `main`; the remaining isolation (the
  checkout's unneeded `token:` input, formerly DUA-6) moved to the change
  `changelog-sync-checkout-token`. A merged Renovate bump of an action or tool in that job still
  runs next to a token that can push to `main`, which is why DUA-3 never auto-merges them.

## What Changes

- `.github/workflows/renovate.yaml` mints a short-lived installation token with
  `actions/create-github-app-token` (pattern at `changelog-sync.yaml:74`) from a NEW App with no
  ruleset bypass; the `github.token` fallback is removed. It runs daily.
- `renovate.json` (the repo config; `.github/renovate-config.json` only points at it) auto-merges
  only `patch` and `minor` updates of non-Kubernetes Go modules, with `automergeStrategy:
  "rebase"`, `platformAutomerge: false` (Renovate merges itself and so waits for every check on
  the head SHA, not just the required ones) and `minimumReleaseAge: "7 days"`. Everything else is
  never auto-merged: majors, `k8s.io/` and `sigs.k8s.io/`, the `go` directive, Dockerfiles,
  GitHub Actions of every kind (they run in jobs that hold tokens), pinned tools (`custom.regex`),
  and `digest`/`pinDigest` updates (no release timestamp, cannot be aged).
- A step before Renovate sets `automerge: false` for the run when the latest push CI on `main`
  is not green.
- `changelog-sync.yaml` is out of scope: its token isolation (formerly DUA-6) is the change
  `changelog-sync-checkout-token`.
- `hack/test/renovate_automerge_test.sh` guards all of this structurally.

## Capabilities

### New Capabilities

- `dependency-updates`: how update PRs are created, tested and merged, and what a bot can merge.

### Modified Capabilities

None.

## Impact

- Entry points: `.github/workflows/renovate.yaml`, `renovate.json`, the `lint` job (required)
  running the guard.

## Dependencies

Landing order across the ten proposed changes: (1) ci-workflow-hardening, (2) task-check-entrypoint,
(3) golangci-lint-bump, (4) cross-file-consistency-gates, (5) developer-toolchain-pins,
(6) dependency-update-automation, (7) ci-failure-reporting, (8) test-depth-signals,
(9) mutation-testing-signal, (10) public-agent-contract.

Hard prerequisites, checked in tasks section 0 before any other task:
the four jobs `lint`, `vulncheck`, `workflow-security`, `dependency-review` are required checks
(change 1, operator step; done, task 0.1); `install-git-cliff.sh` verifies a checksum (change 5). The operator also
chooses the dependency bot; this change assumes Renovate stays (the choice is the operator's and
is recorded elsewhere).

## Non-goals

- Auto-merging anything beyond Go module patch/minor; widening is a later reviewed change.
- Dependabot, `pull_request_target` workflows (forbidden here), changing which checks are required
  beyond the prerequisite.
- Reusing the changelog-sync App.

## Assumptions

- Events made with an App installation token DO trigger workflows (`GITHUB_TOKEN` is the
  exception). Source: GitHub docs, "Triggering a workflow from a workflow"; post-merge row below.
- With `platformAutomerge: false` Renovate merges through the API after all status checks on the
  branch pass, using `automergeStrategy`; a PR behind `main` under a strict up-to-date rule is
  refused until Renovate rebases it (`rebaseWhen: "behind-base-branch"`), which happens at its
  next run; daily runs bound that delay. Source: Renovate docs ("automerge", "automergeStrategy",
  "platformAutomerge", "rebaseWhen"); probe in task 3.3. Rebase merging is already the only merge
  method the repository allows.
- `RENOVATE_FORCE` can carry `{"automerge":false}` for one run (the workflow already uses it for
  `schedule`). Probe in task 2.3.
- Renovate editing files under `.github/workflows/` needs the App's `workflows` permission.
- The ruleset may contain `require_extra_approval_for_unattributed_changes`. Task 0.3 result
  (operator, 2026-10-16): no observed effect on bot-authored merges.
