# Tasks

## 1. Probe (assumption in proposal.md)

- [x] 1.1 Read `.github/workflows/changelog-sync.yaml` end to end and record, with line numbers, how
  the `sync` job pushes and whether any step after the checkout relies on the checkout's
  credentials. If removing `token:` would break the push, stop: the change is BLOCKED.
  Evidence: `evidence/T1.1.md`. — closed 2026-10-16: not blocked; the push sets its own
  extraheader from `APP_TOKEN` and no later step uses checkout credentials

## 2. Guard first (CST-1, CST-2)

- [x] 2.1 Extend `hack/test/changelog_sync_release_guard_test.sh`: section 6 checks the checkout's
  credentials and that the App token output is read only by the push step; section 8 adds mutants.
  Run it on today's `main` and record it red on the token assertion (not on a parse or usage
  error). Cases, each on a structural copy of the real workflow:
  - CST-1 given the checkout with `token:`, when the guard runs on today's tree, then it fails
    naming the checkout's `token` input.
  - CST-2 given the fixed workflow, when a mutant re-adds `token: ${{ steps.app-token.outputs.token }}`
    to the checkout, then the check fails naming `token`.
  - CST-2 given the fixed workflow, when a mutant sets `persist-credentials: true`, then the check
    fails naming `persist-credentials`.
  - CST-2 given the fixed workflow, when a mutant removes `persist-credentials`, then the check
    fails naming `persist-credentials` (the action's default is `true`).
  - CST-2 given the fixed workflow, when a mutant puts the App token into the guard step's `env`,
    then the check fails naming the App token and that step.
  - CST-2 (review round 1) given the fixed workflow, when a mutant reads the token as
    `steps['app-token']['outputs']['token']`, reads `toJSON(steps)`, or adds a step that references
    `secrets.CHANGELOG_SYNC_GITHUB_APP_PRIVATE_KEY`, then the check fails naming that step.
  - CST-2 (review round 2) given the fixed workflow, when a mutant reads `toJSON(steps.app-token)`,
    `toJSON(secrets)` or `secrets[format(...)]` in the guard step, or puts the App private key in
    the job-level `env:`, then the check fails naming that step or scope.
  - CST-2 no-op controls: the identity round-trip passes; `persist-credentials: "false"` (a string)
    passes.

  Closed 2026-10-16: red on `main` on the checkout `token` assertion, then green with 3.1;
  evidence: `evidence/T2.1.md`.

## 3. Implement (CST-1)

- [x] 3.1 Remove `token:` from the changelog-sync checkout and correct its comment. Then: the guard
  is green; `hack/test/ci_workflow_security_test.sh` (zizmor) green in plain and `--self-test`;
  `hack/test/sonar_ko_08_workflow_permissions_test.sh` green; `task lint:shell` clean. — closed
  2026-10-16, evidence: `evidence/T2.1.md`

## 4. Land

- [x] 4.1 Independent review of the diff (CI and security posture); PR with green CI on the head SHA;
  rebase merge. — closed 2026-10-16: PR #467 (head f75c1bb12, merge 743a536fc), 38 checks green,
  2 skipped; reviews in `reviews/T/` rounds 1–2
- [x] 4.2 After the merge, find the changelog-sync run on `main` by the merge head SHA and record its
  conclusion (whether it pushed or found nothing to sync). Archive in a follow-up evidence PR. —
  closed 2026-10-16: run 38926231432 on headSha 743a536fc, success, pushed `743a536f..0ce077f6`
  and logged "Pushed regenerated CHANGELOG.md to main"; archived in a follow-up evidence PR

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| CST-1 | probe of the push path | push needs no checkout credentials | pass | evidence/T1.1.md (read of `a9913eda9`, runs 38889827962, 38551429146) |
| CST-1 | guard on `main` before 3.1 | red on the checkout `token` assertion | pass | evidence/T2.1.md, base `950743808`, exit 1 |
| CST-1 | guard after 3.1 | green | pass | evidence/T2.1.md, exit 0 |
| CST-1 | changelog-sync run on `main` after the merge | success, and the push step logs "Pushed regenerated CHANGELOG.md" (the push soft-fails, so a green run alone is not proof); verdict `in-sync` means no push was tried and the row stays open | pass | changelog-sync run 38926231432 on `main` (headSha 743a536fc): success; the push step pushed `743a536f..0ce077f6 HEAD -> main` and logged "Pushed regenerated CHANGELOG.md to main." |
| CST-2 | token, persist-true, persist-removed, other-step, bracket-ref, step-object, `toJSON(steps)`, `toJSON(secrets)`, computed-index, job-env and App-secret mutants by exit status; identity and string-`"false"` controls | mutants rejected for the named reason; controls pass | pass | evidence/T2.1.md: 11 self-test mutants rejected, 2 controls accepted; real-file token and persist mutants exit 1, restored exit 0; each new check switched off in a scratch copy reds the self-test |
| all | zizmor gate both modes, `task lint:shell`, `task spec:validate` | green | pass | evidence/T2.1.md; zizmor 1.30.1 no findings |
| all | independent review; CI on the PR head | APPROVE or verified findings fixed; green | pass | PR #467 head f75c1bb12: 38 checks green, 2 skipped; main post-merge CI by headSha 743a536fc: all 7 workflows success (CI, Preflight, E2E smoke, CodeQL, CodeQL - Code Quality, OpenSSF Scorecard, changelog-sync); round 1 on `bcd248619`: astra REQUEST_CHANGES (P1 bracket spelling, fixed), claude/opus CLEAN (notes; two fixed); round 2 on `2e3640092`: astra APPROVE, claude/opus CONCERNS (2 WARNING, both fixed after the round, no third round); reviews/T/ |
