# Tasks

## 1. Probe (assumption in proposal.md)

- [ ] 1.1 Read `.github/workflows/changelog-sync.yaml` end to end and record, with line numbers, how
  the `sync` job pushes and whether any step after the checkout relies on the checkout's
  credentials. If removing `token:` would break the push, stop: the change is BLOCKED.
  Evidence: `evidence/T1.1.md`.

## 2. Guard first (CST-1, CST-2)

- [ ] 2.1 Extend `hack/test/changelog_sync_release_guard_test.sh`: section 6 checks the checkout's
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
  - CST-2 no-op controls: the identity round-trip passes; `persist-credentials: "false"` (a string)
    passes.

## 3. Implement (CST-1)

- [ ] 3.1 Remove `token:` from the changelog-sync checkout and correct its comment. Then: the guard
  is green; `hack/test/ci_workflow_security_test.sh` (zizmor) green in plain and `--self-test`;
  `hack/test/sonar_ko_08_workflow_permissions_test.sh` green; `task lint:shell` clean.

## 4. Land

- [ ] 4.1 Independent review of the diff (CI and security posture); PR with green CI on the head SHA;
  rebase merge.
- [ ] 4.2 After the merge, find the changelog-sync run on `main` by the merge head SHA and record its
  conclusion (whether it pushed or found nothing to sync). Archive in a follow-up evidence PR.

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| CST-1 | probe of the push path | push needs no checkout credentials | not-run | evidence/T1.1.md |
| CST-1 | guard on `main` before 3.1 | red on the checkout `token` assertion | not-run | |
| CST-1 | guard after 3.1 | green | not-run | |
| CST-1 | changelog-sync run on `main` after the merge | success; no auth failure | post-merge | issue #443 owns the row until the follow-up evidence PR |
| CST-2 | token, persist-true, persist-removed and other-step mutants by exit status; identity and string-`"false"` controls | mutants rejected for the named reason; controls pass | not-run | |
| all | zizmor gate both modes, `task lint:shell`, `task spec:validate` | green | not-run | |
| all | independent review; CI on the PR head | APPROVE or verified findings fixed; green | not-run | |
