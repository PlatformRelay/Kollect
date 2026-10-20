# Tasks

## 1. Test first (OFS-1 to OFS-5)

- [x] 1.1 Write `hack/test/operatorhub_fork_sync_test.sh` (fake `gh`, `git`, `curl`, `make` on
  PATH; cases behind->synced, identical, diverged, ahead, wrong parent, stuck sync, dry run,
  dry run diverged, offline dry run); run it against the script on `main` and record the red —
  evidence: evidence/T1.md

## 2. Implement (OFS-1 to OFS-4)

- [x] 2.1 `hack/operatorhub-pr.sh`: `preflight_forks` (read-only, all forks first), `sync_forks`
  (`merge-upstream` for a behind fork, then re-compare), DRY_RUN plan output; never force-push a
  fork's default branch, stop on a merge commit from GitHub — evidence: evidence/T1.md
- [x] 2.2 `hack/test/dist_operatorhub_pr_test.sh`: run its dry run with `GH_TOKEN` unset so it
  stays offline — evidence: evidence/T1.md

## 3. Guard and docs (OFS-5)

- [x] 3.1 `--self-test`: one mutant per clause plus a no-op control; wire plain and `--self-test`
  into the `lint` job; add the guard to `cfc_guards` — evidence: evidence/T1.md
- [x] 3.2 `docs/RELEASE.md`: the sync step, the diverged-fork failure and remedy, the dry run's
  output — evidence: evidence/T1.md

## 4. Proof and land

- [x] 4.1 Dry run against the real forks, read-only, with `gh` and `git` wrappers that block any
  write — evidence: evidence/T1.md
- [x] 4.2 Independent review (two legs, then a fresh round 2); PR with every check green on the
  head SHA; archive as the last commit — evidence: evidence/T1.md (Review)

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| OFS-1 | `case_behind_synced`, `case_identical_no_sync`, `case_stuck_sync_fails`, `case_merge_commit_fails`, `case_sync_refused_fails`; mutants `no-sync`, `sync-after-clone`, `no-recompare`, `merge-accepted`, `refusal-unchecked` | sync before clone, re-compare, only the version branch pushed; no sync when identical; stuck, refused or merge-commit sync stops before push | pass | evidence/T1.md |
| OFS-2 | `case_diverged_fails`, `case_ahead_fails`, `case_wrong_parent_fails`, `case_fork_unreadable_fails`, `case_compare_unreadable_fails`, `case_unknown_status_fails`; mutants `diverged-accepted`, `ahead-accepted`, `parent-unchecked`, `unknown-accepted` (killed by writes), `fork-read-unchecked`, `compare-unchecked` (killed by message: the next check still fails closed) | non-zero, named message, zero writes, no git | pass | evidence/T1.md |
| OFS-3 | `case_diverged_fails` (first fork behind, second diverged) | the behind fork is not synced | pass | evidence/T1.md |
| OFS-4 | `case_dry_run_plan`, `case_dry_run_diverged`, `case_dry_run_offline`; mutants `dry-run-syncs`, `dry-run-offline-calls`; real-fork dry run | plan printed, zero writes; diverged fails; offline makes no call | pass | evidence/T1.md |
| OFS-5 | `--self-test` (8 mutants + no-op); CWS-6/CFC-6 with the `--self-test` step removed | each mutant reds its own case; unwired mode reds | pass | evidence/T1.md |
| OFS-1 | live `merge-upstream` on a real fork | fast-forward | N/A | a write to the third-party forks is outside this change (operator-owned, first release after merge); the response shape is taken from the GitHub REST docs (proposal, Assumptions) and the fake `gh` returns that shape |
| all | independent review; CI on the PR head | APPROVE, green | pass | review: round 1 two legs on `93011938f` APPROVE (P2s fixed in `3611626cf`), round 2 fresh reviewer on `3611626cf` APPROVE (wording fixed in `a9b508cb4`), evidence/T1.md (Review); PR #482 head `a9b508cb4`: 39 checks success, 3 skipped (conditional jobs), runs CI 39357586513, Preflight 39357586561, Docs 39357586419, E2E smoke 39357586365, E2E extended 39357586517, CodeQL 39357586701; archive revision: the last commit of PR #482 |
