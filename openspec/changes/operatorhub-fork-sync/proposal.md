# Proposal

## Why

`hack/operatorhub-pr.sh`, run by the `OperatorHub PR` job of `.github/workflows/release.yaml`,
cuts the submission branch `kollect-v<version>` from `upstream/main` and pushes it to the
`platformrelay` fork of `k8s-operatorhub/community-operators` and of
`redhat-openshift-ecosystem/community-operators-prod`. It never looks at the fork's own default
branch. When the fork lags upstream, the pushed branch carries every upstream commit the fork
lacks, `.github/workflows/*` among them, and the push or the PR fails or rests on a stale base. The
forks were synced by hand on 2026-10-16 (`gh repo sync`); on 2026-10-20 they were behind again,
by 15 and 18 commits (read-only probe in `evidence/T1.md`).

## What Changes

- Before any write, the script reads each fork (`GET repos/<fork>`: its parent must be the
  upstream, case-insensitively; its default branch) and compares it with upstream
  (`GET repos/<upstream>/compare/main...<owner>:<repo>:<branch>`). Identical or behind passes;
  ahead, diverged or an unreadable answer fails closed for both forks, before anything is written.
- A fork that is behind is fast-forwarded with `POST repos/<fork>/merge-upstream` (the endpoint
  behind `gh repo sync`), then compared again; anything but `identical` stops the run before the
  first push. The default branch is never force-pushed. `merge-upstream` takes no expected SHA, so
  a fork that moves between the check and the sync can still get a merge commit from GitHub; the
  script detects `merge_type=merge` and stops before pushing.
- `DRY_RUN=1` keeps making no remote writes. With `GH_TOKEN` set it runs the read-only checks and
  prints, per fork, what it would sync, branch, push and open; a diverged fork fails the dry run
  too. Without `GH_TOKEN` it makes no API call and says so.
- `hack/test/operatorhub_fork_sync_test.sh` runs the script against fake `gh`, `git`, `curl` and
  `make`, plain and `--self-test`, wired into the required `lint` job and named in CFC-6.
- `docs/RELEASE.md` describes the sync step, the diverged-fork failure and the dry run's output.

## Capabilities

### New Capabilities

- `operatorhub-submission`: how the release workflow's OperatorHub submission keeps its forks
  current and what it refuses to do.

### Modified Capabilities

None.

## Impact

- Entry point: the `OperatorHub PR` job of `.github/workflows/release.yaml`, step "Generate OLM
  bundle and create OperatorHub PRs", which runs `hack/operatorhub-pr.sh` with
  `GH_TOKEN: ${{ secrets.OPERATORHUB_PAT }}`, and the manual fallback in `docs/RELEASE.md`.
- Credential: no new scope. `OPERATORHUB_PAT` is a classic PAT with `public_repo` and `workflow`
  (script header, `docs/RELEASE.md`). `merge-upstream` needs push access to the fork, which
  `public_repo` grants; an upstream range that touches `.github/workflows/*` needs `workflow`, the
  same scope the branch push already needs. The job's `permissions: contents: read` applies to
  `GITHUB_TOKEN` only and does not limit the PAT.
- The job stays `continue-on-error`; a diverged fork surfaces through the existing warning and
  job-summary line, and the release itself is unaffected.

## Assumptions

- `GET repos/{owner}/{repo}/compare/{base}...{head}` accepts a cross-fork head as
  `owner:repo:branch` and returns `status` (`identical`, `behind`, `ahead`, `diverged`),
  `ahead_by` and `behind_by`. Probed read-only on 2026-10-20 against both forks (`evidence/T1.md`).
- `POST repos/{owner}/{repo}/merge-upstream` with `branch` syncs a fork's branch with the upstream
  default branch and returns `merge_type` (`none`, `fast-forward` or `merge`). Source: GitHub REST
  docs, "Sync a fork branch with the upstream repository"
  (docs.github.com/en/rest/branches/branches#sync-a-fork-branch-with-the-upstream-repository).
  Not probed: a probe is a write to the forks, which this change does not make.

## Non-goals

- Reconciling a diverged fork automatically (reset, merge or force-push).
- Changing the PR body, the bundle, the target repositories or the job's soft-fail behaviour.
- A live submission. The first real sync happens at the next release, run by the operator.
