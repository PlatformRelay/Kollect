## Verdict: CLEAN

## Findings
- [NOTE] The push still authenticates without the checkout token, so the load-bearing premise holds — `.github/workflows/changelog-sync.yaml:117-142`
  Failure: none. The push step reads `APP_TOKEN` from its own `env` (line ~120) and builds the extraheader through `GIT_CONFIG_*` for that one process. Before this change the checkout already had `persist-credentials: false`, so the App token never reached the push through `.git/config`. Removing `token:` changes only the fetch, which now uses the default GITHUB_TOKEN. That is enough for a public repo with `fetch-depth: 0` and tags.
  Fix: none. The live proof is still the post-merge run (tasks.md 4.2), and the change records it as such.
  Confidence: 90
- [NOTE] The job-level `contents: write` is now unused privilege — `.github/workflows/changelog-sync.yaml:71-72`
  Failure: the App token does every write. The GITHUB_TOKEN only fetches, yet it is still minted with write scope, and any action in the job (setup-task, git-cliff installer) can use it through `github.token` to push non-protected refs or tags. The comment "commits & pushes ... back to main" is now inaccurate. This was already true before the change and is not a regression; this diff just makes it visible.
  Fix: a follow-up that sets `contents: read` on the job, after checking `hack/test/sonar_ko_08_workflow_permissions_test.sh` for an assertion on it.
  Confidence: 70
- [NOTE] The token-isolation check has blind spots — `hack/test/changelog_sync_release_guard_test.sh` (`credential_errors`, `_output_ref`)
  Failure: `${{ toJSON(steps) }}` in an earlier step, or a second step that mints its own token from `secrets.CHANGELOG_SYNC_GITHUB_APP_PRIVATE_KEY`, would expose a bypass-capable token and the guard would still pass. T2.1 "Limitations" admits the first case. The re-mint case is not mentioned anywhere.
  Fix: in `credential_errors`, also reject `toJSON(steps)` / `steps.*` and any reference to the two App secrets outside the create-github-app-token step. Add one mutant for each.
  Confidence: 75 (that it is a gap; it is beyond what CST-1 literally asks)
- [NOTE] Stricter than the spec requires (still within its text): `"token" in given` also rejects a harmless `token: ${{ github.token }}`, and `persist-credentials: ${{ false }}` is rejected — `changelog_sync_release_guard_test.sh` (`credential_errors`)
  Failure: an honest rewrite fails CI. The spec does say "any value", so the token rejection is intended.
  Fix: none needed. Optionally, accept `${{ false }}` the way `falsy_if` normalises literals.
  Confidence: 60
- [NOTE] Writing `persist-credentials: "false"` into the real workflow makes the guard fail, because the `checkout-persist-string-false` variant becomes a no-op against the baseline — section 8 `mutant_accepted`
  Failure: the honest spelling turns the suite red. T2.1 documents this as by design, and the same is true of `push-spelled-plain`.
  Fix: none required. If the variant is made relative to the current spelling (for example, flip to the other honest spelling), it stays meaningful.
  Confidence: 80
- [NOTE] The `|| return 0` change in `mutant_rejected`/`mutant_accepted` correctly fixes a real `set -e` early exit, and nothing in the commit subject mentions it — `changelog_sync_release_guard_test.sh:~948,966`
  Failure: before the fix, the first no-op mutant ended the run silently, before the remaining cases and the summary. It is correct now, and `fail()` still increments `failures`.
  Fix: none. It is noted in T2.1. Ideally it would be its own `fix(test)` commit, per the one-logical-change rule.
  Confidence: 85

Checked:
- **Workflow:** the whole of `changelog-sync.yaml` at the head (step order, push env, header comments).
- **Credential checks (CST-1):**
  - `credential_errors`, `checkout_indices`, `app_token_ids` and `_output_ref`, and how they plug into `static_wiring_check`.
  - `pushes` is in scope where `credential_errors` is called (line ~514).
  - A token-reading step that also pushes to main is still caught by the "exactly one push step" error.
- **Mutants (CST-2):**
  - Each mutant builds correctly: the guard step had no `env`, so `setdefault` works, and `checkout_with` defaults correctly.
  - Each expected-reason string matches the error text.
- **Spec and CI:**
  - Every CST-1 and CST-2 scenario maps to an assertion or a mutant. The push-auth scenario is post-merge only.
  - The DUA-6 removal is consistent across proposal, design, tasks and spec.
  - `ci.yaml:359` runs the guard in `lint`.
- **Docs:** the T1.1 and T2.1 evidence contain no secret values. The proposal says RENOVATE_TOKEN is an admin PAT but gives no value.

## Could not check
- I did not run the guard, zizmor, `task lint:shell` or `task spec:validate`. All results above come from the recorded evidence in T2.1.md.
- I could not verify the cited run logs (38551429146, 38889827962) or the live `protect-main` ruleset. The post-merge changelog-sync run (task 4.2) has not happened yet.
- I did not read `hack/test/sonar_ko_08_workflow_permissions_test.sh` to see whether it pins `contents: write` on this job.
- I did not read `openspec/changes/changelog-sync-checkout-token/proposal.md` or the untracked `reviews/` directory.
