# Design

## Context

See proposal.md. The reference project attune removes CI on push to `main`.

## Decisions

- **Push-to-main CI stays. This is a deliberate counterpoint to the reference project.** attune
  squash-merges onto up-to-date branches, so a green PR head is the same tree as the merge
  result and a second run on `main` adds nothing. Kollect rebase-merges (workspace merge policy:
  linear history, no squash), so `main` can contain commits whose tree no PR run ever tested, and
  `hack/release/verify-eligibility.sh` requires exact-SHA green checks on the release commit.
  Dropping the trigger would make every release fail that check or force a manual re-run.
  The spec makes removal a forbidden outcome (CWS-6).
- **`cancel-in-progress` uses the existing expression** `${{ github.ref != 'refs/heads/main' }}`
  (HY-06 in `codeql.yaml`, `e2e-*.yaml`), so a run on `main` is never cancelled (a cancelled
  main run leaves the SHA without a verdict) while superseded PR runs are. Group key:
  `ci-${{ github.event.pull_request.number || github.ref }}`.
- **harden-runner audit first.** Audit mode records egress and cannot break a job. A later
  block-mode change builds its allowlist from the audit data.
- **zizmor offline, suppressions in a file.** `--offline` avoids a token and network flakiness.
  Findings that need the network (for example impostor-commit checks) are out of reach; that is
  accepted. A suppression lives in `.github/zizmor.yml` with a comment naming the audit, the
  workflow and why it is safe, never as an inline ignore without a reason.
- **`dependency-review` is PR-only.** It compares base and head manifests, which does not exist
  on a push. It is therefore not an exact-SHA release check.

## Risks / Trade-offs

- [harden-runner is a third-party action with a network agent] -> pinned by digest like every
  other action (Renovate `github-actions` manager), audit mode only.
- [zizmor first run reports many findings] -> task 3.2 fixes or justifies each; the gate is not
  switched on until the tree is clean, so main never goes red.
- [Cancelling superseded PR runs hides a flaky failure of the older run] -> acceptable; the
  newest head is what merges.
