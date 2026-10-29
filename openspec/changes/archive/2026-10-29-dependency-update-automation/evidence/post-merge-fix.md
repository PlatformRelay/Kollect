# Post-merge fix: a bot label cancelled the extended e2e run

Status: CLOSED (follow-up PR to #502). Found 2026-10-28 while checking the DUA-7 post-merge row.

## What happened (verified 2026-10-28)

- The first run on `main` after #502 (dispatch, run 40707869797) opened #503 (`docker images`:
  debian and distroless digests) and #504 (`go toolchain`: golang digest), both with
  "Automerge: Enabled", and the Dependency Dashboard issue #505.
- Renovate labels a PR (`dependencies`) right after opening it. E2E extended triggers on
  `labeled`, and its concurrency group was shared per PR with `cancel-in-progress` off `main`, so
  the labeled run (40708456967, success) cancelled the opened run (40708456104, cancelled) on the
  same head SHA `285e9706`.
- `gh api repos/PlatformRelay/Kollect/commits/<head>/check-runs` then lists the cancelled
  `gate`, `kind-webhook-smoke`, `e2e-${{ matrix.scenario }}` and three `*-meta` check runs.
  Renovate 44.179.1 `getBranchStatus` (evidence/0.4.md (e)) maps any conclusion other than
  success, skipped or neutral to pending, so the PR never reaches green and never auto-merges.
- The same pattern is in older Renovate PR history (e2e-extended runs cancelled on 2026-08-31,
  09-07, 09-14, 09-21 and 10-05); it did not matter while every bot PR was merged by hand.

## Fix

- `.github/workflows/e2e-extended.yaml`: a run started by the `dependencies` label gets the
  suffix `-label-<run id>`, a group of its own; it no longer cancels the opened run and is never
  cancelled by it. It decides like any other run (gate unchanged), so its checks are real. Any
  other label still supersedes as before.
- `hack/test/renovate_policy_test.sh` T18: the group is exactly the shared PR group plus that
  suffix. Written first and red on `main` (`FAIL: T18: ... does not end with the label-run
  suffix`), green after the fix; five mutants (file missing, no group, no suffix, a fixed
  `-label` suffix, a run-id prefix) each red with T18's own message; self-test 13 controls green,
  152 mutants red.
- `docs/development/tooling-setup.md`: a human label on a bot PR still cancels; rebase to clear.

## Sensors (fix branch, 2026-10-28)

| Command | Exit |
| --- | --- |
| `bash hack/test/renovate_policy_test.sh` | 0 (red on `main` at T18 before the fix) |
| `bash hack/test/renovate_policy_test.sh --self-test` | 0 |
| `bash hack/test/ci_workflow_security_test.sh` and `--self-test` | 0, 0 |
| `bash hack/test/ci_docs_gate_test.sh`, `finalizer_cleanup_e2e_test.sh`, `e2e_mt_repro_harden_test.sh`, `e2e_webhook_existing_cluster_test.sh` | 0 |
| `zizmor --offline --min-severity=high` (1.30.1), `actionlint` on the workflow | 0, 0 |
| `task lint:shell`, `task lint:markdown`, `task spec:validate` | 0 |

## Review

`reviews/post-merge/`: round 1 (an earlier version: shared `-other-label` group plus a gate
no-op) reviewer A CONCERNS, reviewer B CONCERNS: the no-op run could hide a real result under the
same check name, two labels still cancelled each other, no guard. Redesigned. Round 2 (last):
reviewer A APPROVE, reviewer B CONCERNS: duplicate heavy runs for every non-`e2e/full` label
(fixed: only the `dependencies` label is isolated), T18 checked only the suffix (fixed: exact
group, plus a run-id prefix mutant), an `e2e/full` label on a bot PR still leaves cancelled checks
(documented), flake doubling on a labeled bot PR (accepted: fails safe, watched on the next
digest PRs). Round-2 fixes are unreviewed (last round).

## Residual cost

A Renovate PR whose diff matches the gate's patterns (any Dockerfile digest PR) runs the heavy E2E
jobs twice on one head; either run's failure holds the auto-merge. Accepted: fails safe.
