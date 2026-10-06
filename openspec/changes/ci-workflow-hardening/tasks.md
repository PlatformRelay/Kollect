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
- [ ] 4.2 Probe on a throwaway PR with a known-vulnerable module and one disallowed licence: job fails both times (needs two real throwaway PRs against the repo; the config-level mutants are covered by the meta-test's CWS-3 mutants)

## 5. Operator-owned (hard prerequisite of the later changes)

- [ ] 5.1 (operator) Probe that a skipped required job counts as passing, then make `lint`, `vulncheck`, `workflow-security` and `dependency-review` required checks in `protect-main`
- [ ] 5.2 (operator) Read the ruleset JSON for `require_extra_approval_for_unattributed_changes` and record whether it affects bot PRs

## 6. Land

- [ ] 6.1 Merge with the `post-merge` rows below open; the archive commit lands in a small follow-up evidence PR once they pass

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
| CWS-1 | `workflow-security` on the PR head; template-injection fixture; meta-test mutants (no `--offline`, skip switch) | green on tree, red on fixture and mutants | mutants red locally (12 mutants); PR-head run is post-merge | evidence/evidence.md |
| CWS-2 | meta-test bare-suppression mutant; review of each suppression | red on mutant; each justified | red on mutant; zero suppressions exist (`rules: {}`) | evidence/evidence.md |
| CWS-3 | `dependency-review` on throwaway PRs (vulnerable module, disallowed licence); meta-test mutants | job red twice; mutants red | mutants red; throwaway-PR probe post-merge (task 4.2) | evidence/evidence.md |
| CWS-4 | meta-test group check (ref-keyed, unprefixed mutants); two pushes to a PR | first run cancelled | mutants red locally; two-push cancellation is live post-merge | evidence/evidence.md |
| CWS-4 | three quick merges to main each get a run | each SHA has a run | post-merge | follow-up evidence PR |
| CWS-5 | meta-test with `push` removed (mutant) | mutant red | red locally | evidence/evidence.md |
| CWS-6 | meta-test: guard in non-required job, unpinned mode | red | red locally (19 pre-existing orphans wired into `lint`: 4 e2e-side + 15 docs-side) | evidence/evidence.md |
| CWS-6 | ruleset lists the four jobs as required | listed | post-merge | operator, task 5.1 |
| CWS-7 | self-test mutants plus no-op control, by exit status | each mutant red, no-op green | green locally (34 mutants + no-op) | evidence/evidence.md |
| all | independent review; CI on the PR head | APPROVE, green; reviewed and archive revisions recorded | not-run | this PR |
