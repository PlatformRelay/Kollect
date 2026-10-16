# Proposal

## Why

`.github/workflows/changelog-sync.yaml` mints an installation token of the App that sits on the
`protect-main` bypass list (`bypass_mode: always`, header lines 18-19; minting step at line 74) and
hands that token to `actions/checkout` (`token: ${{ steps.app-token.outputs.token }}`, line 87).
The checkout only fetches a public repository, which the job's default `GITHUB_TOKEN` does as
well. The push authenticates itself: the "Commit & push if drifted" step reads the token from its
own `APP_TOKEN` env (line 120) and passes it to that one `git push` through `GIT_CONFIG_*`
extraheader keys (lines 132-138). `persist-credentials: false` (line 92) already keeps the token
out of `.git/config`, so the `token:` input gives the most privileged credential in the repository
to a third-party action that does not need it.

This was DUA-6 of `dependency-update-automation` (change 6, issue #443). Re-measure on
2026-10-16 found the `persist-credentials: false` half already on `main` and the rest of change 6
blocked until the operator creates a Renovate GitHub App, so the 2026-10-16 decision (outcome B,
recorded in the maintainer's decision log) split the remaining half out into this change.

## What Changes

- The changelog-sync checkout drops its `token:` input and keeps `persist-credentials: false`; it
  fetches with the default `GITHUB_TOKEN`. The App token is referenced only by the push step.
- `hack/test/changelog_sync_release_guard_test.sh` (already in the `lint` job, `ci.yaml:359`) gains
  structural checks for the checkout's credentials and self-test mutants for them.

## Capabilities

### New Capabilities

- `changelog-sync`: how the changelog self-healing workflow holds and uses its privileged token.

### Modified Capabilities

None.

## Impact

- Entry point: `.github/workflows/changelog-sync.yaml`, job `sync` (runs on every push to `main`).
- Guard: `hack/test/changelog_sync_release_guard_test.sh`, run by the `lint` job.

## Non-goals

- Renovate, its token, automerge and everything else in `dependency-update-automation`.
- Replacing the extraheader push with another mechanism, or changing the App or its bypass.
- Who may push to `main`; the ruleset is unchanged.

## Assumptions

- Nothing after the checkout uses the checkout's credentials. Probe: task 1.1, `evidence/T1.1.md`
  (the push sets its own extraheader from `APP_TOKEN`; `persist-credentials: false` already removed
  the checkout's credentials before any later step ran).
- The default `GITHUB_TOKEN` can fetch full history and tags of this public repository with
  `contents: write` granted to the job (`changelog-sync.yaml:71-72`). Source: actions/checkout
  README (`token` defaults to `${{ github.token }}`); confirmed after merge by the changelog-sync
  run on `main` (post-merge row in `tasks.md`).
