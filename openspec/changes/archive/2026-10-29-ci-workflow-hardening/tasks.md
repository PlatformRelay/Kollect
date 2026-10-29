# Tasks

## 1. Meta-test first (CWS-1 to CWS-7)

- [x] 1.1 Probe `runs-on` values; grep `hack/test/` for step-index assertions (`steps[0]`, `steps | length`, `steps[N]`) that adding jobs or steps could break; write `hack/test/ci_workflow_security_test.sh` with the checks and mutants; run it and watch it fail on the missing wiring (assertion named)
  - Probes: `dist_ci_wiring_test.sh` reds on the new top-level `concurrency` key until `WORKFLOW_KEY_ALLOWLIST` gains it (done, with the CWS-4 justification); `ci_docs_gate_test.sh` pins step counts only for the `test`/`kind-smoke`/`changes` jobs, which the new jobs do not touch; no `hack/test/` script asserts step indices of `ci.yaml`'s new jobs.
- [x] 1.2 Wire it into the `lint` job next to `ci_docs_gate_test.sh`, plain and `--self-test` as separate steps

## 2. Concurrency (CWS-4, CWS-5)

- [x] 2.1 Add the `concurrency:` block to `ci.yaml` and fix the group in `e2e-smoke.yaml`; test passes

## 3. zizmor (CWS-1, CWS-2)

- [x] 3.1 Add `hack/install-zizmor.sh` (pinned release, checksum verified through `hack/lib/fetch.sh`, same shape as `install-gitleaks.sh`); probe `--offline`; choose and record `--min-severity`
- [x] 3.2 Run zizmor on the tree; fix every fixable finding in its own commit (a fix that changes workflow behaviour says so); suppress only what cannot be fixed, each with a reason in `.github/zizmor.yml`
- [x] 3.3 Add the `workflow-security` job; add it to `verify-eligibility.sh` `required_checks` and `hack/release/test-verify-eligibility.sh`
- [x] 3.4 Every later change that adds a workflow must satisfy this gate in its own tasks (recorded in each such change) — rule adopted; `hack/test/ci_workflow_security_test.sh` is the enforcer the later changes reference

## 4. dependency-review (CWS-3)

- [x] 4.1 Add the `dependency-review` job (PR only, default threshold, `allow-licenses` list derived from `go-licenses` or the current dependency set, with reasons for each per-module exception)
- [x] 4.2 Probe on a throwaway PR with a known-vulnerable module and one disallowed licence: job fails both times (needs two real throwaway PRs against the repo; the config-level mutants are covered by the meta-test's CWS-3 mutants) — closed 2026-10-25, evidence: evidence/post-merge.md

## 5. Operator-owned (hard prerequisite of the later changes)

- [x] 5.1 (operator) Probe that a skipped required job counts as passing, then make `lint`, `vulncheck`, `workflow-security` and `dependency-review` required checks in `protect-main`
  - 2026-10-25: the four are required (ruleset versions 53247538 and 53474785). The skip probe is still open: `vulncheck` is skipped on every docs-only PR, and no docs-only PR has yet reached a state that shows whether the skipped job counts as passing (#489 is also red on `dependency-review`). Evidence: evidence/post-merge.md
  - 2026-10-29: closed. Dependabot #489 (docs only) merged as `3772071` with `vulncheck` SKIPPED and every other required context SUCCESS; rule suite 4802732069 `required_status_checks` `pass`, not `bypass`. Evidence: evidence/post-merge.md
- [x] 5.2 (operator) Read the ruleset JSON for `require_extra_approval_for_unattributed_changes` and record whether it affects bot PRs — closed 2026-10-25, evidence: evidence/post-merge.md (`true`; Renovate #475/#476 and Dependabot #457 merged with rule suite `pass`)

## 6. Land

- [x] 6.1 Merge with the `post-merge` rows below open; the archive commit lands in a small follow-up evidence PR once they pass
  - 2026-10-25: merged as #450; every post-merge row passes except the CWS-6 skip probe (task 5.1), so the archive waits for it
  - 2026-10-29: the skip probe passes (task 5.1); archived in the follow-up evidence PR together with dependency-update-automation

## zizmor threshold decision (measured 2026-10-05, zizmor 1.30.1)

`--min-severity=high` is the pinned threshold because it catches the audits that matter
structurally (template injection, app-token scoping, cache poisoning) while the remaining
findings are all cosmetic:

- **At the gate threshold (`high`): 0 findings.** 27 ignored (12 informational + 15 low,
  every one of them `self-repository` — the "use `${{ github.repository }}` self-repository
  syntax" style audit), 25 suppressed (Pedantic-persona findings that zizmor filters by
  default persona; the committed `.github/zizmor.yml` adds no suppression of its own,
  `rules: {}`).
- **At `medium`: 0 findings.** The only medium finding on the pre-fix tree was `artipacked`
  on changelog-sync.yaml, which is FIXED, not suppressed: `persist-credentials: false` on the
  checkout plus a per-push `GIT_CONFIG_*` extraheader (the same pattern actions/checkout
  itself uses), so nothing needs persisted credentials.
- **Suppressed ≠ suppressed here:** the 25 suppressed findings are zizmor's default persona
  filtering, not entries in `.github/zizmor.yml`. If the threshold ever drops, each of the
  15 `self-repository` findings is fixable mechanically (origin → explicit repository URL),
  so the threshold stays a visible decision with a number behind it.

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| CWS-1 | `workflow-security` on the PR head; template-injection fixture; meta-test mutants (no `--offline`, skip switch) | green on tree, red on fixture and mutants | pass: mutants red locally (12 mutants); green on PR heads #494 (run 40223387529) and #495 (run 40227161572) | evidence/evidence.md, evidence/post-merge.md |
| CWS-2 | meta-test bare-suppression mutant; review of each suppression | red on mutant; each justified | red on mutant; zero suppressions exist (`rules: {}`) | evidence/evidence.md |
| CWS-3 | `dependency-review` on throwaway PRs (vulnerable module, disallowed licence); meta-test mutants | job red twice; mutants red | pass: mutants red; licence red on #470 (run 39170622155) and #489 (run 39987119032); vulnerability red on throwaway #496 (run 40231428226) | evidence/evidence.md, evidence/post-merge.md |
| CWS-4 | meta-test group check (ref-keyed, unprefixed mutants); two pushes to a PR | first run cancelled | pass: mutants red locally; #495 runs 40224677510 and 40225572307 cancelled by the next push | evidence/evidence.md, evidence/post-merge.md |
| CWS-4 | three quick merges to main each get a run | each SHA has a run | pass: five 2026-10-25 merges, one push run each | evidence/post-merge.md |
| CWS-5 | meta-test with `push` removed (mutant) | mutant red | red locally | evidence/evidence.md |
| CWS-6 | meta-test: guard in non-required job, unpinned mode | red | red locally (19 pre-existing orphans wired into `lint`: 4 e2e-side + 15 docs-side) | evidence/evidence.md |
| CWS-6 | ruleset lists the four jobs as required | listed; skipped required job counts as passing | pass: listed (10 contexts); skipped `vulncheck` counted as passing on #489 (rule suite 4802732069 `pass`) | evidence/post-merge.md, task 5.1 |
| CWS-7 | self-test mutants plus no-op control, by exit status | each mutant red, no-op green | green locally (34 mutants + no-op) | evidence/evidence.md |
| all | independent review; CI on the PR head | APPROVE, green; reviewed and archive revisions recorded | pass: two approval rounds before merge (#450, `3ee212669` closes the second round's findings), CI green on its head; archive revision: the follow-up evidence PR that also archives dependency-update-automation | evidence/evidence.md, evidence/post-merge.md |
