# Tasks

## 1. Meta-test first (CWS-1 to CWS-7)

- [ ] 1.1 Probe `runs-on` values; grep `hack/test/` for step-index assertions (`steps[0]`, `steps | length`, `steps[N]`) that adding jobs or steps could break; write `hack/test/ci_workflow_security_test.sh` with the checks and mutants; run it and watch it fail on the missing wiring (assertion named)
- [ ] 1.2 Wire it into the `lint` job next to `ci_docs_gate_test.sh`, plain and `--self-test` as separate steps

## 2. Concurrency (CWS-4, CWS-5)

- [ ] 2.1 Add the `concurrency:` block to `ci.yaml` and fix the group in `e2e-smoke.yaml`; test passes

## 3. zizmor (CWS-1, CWS-2)

- [ ] 3.1 Add `hack/install-zizmor.sh` (pinned release, checksum verified through `hack/lib/fetch.sh`, same shape as `install-gitleaks.sh`); probe `--offline`; choose and record `--min-severity`
- [ ] 3.2 Run zizmor on the tree; fix every fixable finding in its own commit (a fix that changes workflow behaviour says so); suppress only what cannot be fixed, each with a reason in `.github/zizmor.yml`
- [ ] 3.3 Add the `workflow-security` job; add it to `verify-eligibility.sh` `required_checks` and `hack/release/test-verify-eligibility.sh`
- [ ] 3.4 Every later change that adds a workflow must satisfy this gate in its own tasks (recorded in each such change)

## 4. dependency-review (CWS-3)

- [ ] 4.1 Add the `dependency-review` job (PR only, default threshold, `allow-licenses` list derived from `go-licenses` or the current dependency set, with reasons for each per-module exception)
- [ ] 4.2 Probe on a throwaway PR with a known-vulnerable module and one disallowed licence: job fails both times

## 5. Operator-owned (hard prerequisite of the later changes)

- [ ] 5.1 (operator) Probe that a skipped required job counts as passing, then make `lint`, `vulncheck`, `workflow-security` and `dependency-review` required checks in `protect-main`
- [ ] 5.2 (operator) Read the ruleset JSON for `require_extra_approval_for_unattributed_changes` and record whether it affects bot PRs

## 6. Land

- [ ] 6.1 Merge with the `post-merge` rows below open; the archive commit lands in a small follow-up evidence PR once they pass

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| CWS-1 | `workflow-security` on the PR head; template-injection fixture; meta-test mutants (no `--offline`, skip switch) | green on tree, red on fixture and mutants | not-run | |
| CWS-2 | meta-test bare-suppression mutant; review of each suppression | red on mutant; each justified | not-run | |
| CWS-3 | `dependency-review` on throwaway PRs (vulnerable module, disallowed licence); meta-test mutants | job red twice; mutants red | not-run | |
| CWS-4 | meta-test group check (ref-keyed, unprefixed mutants); two pushes to a PR | first run cancelled | not-run | |
| CWS-4 | three quick merges to main each get a run | each SHA has a run | post-merge | follow-up evidence PR |
| CWS-5 | meta-test with `push` removed (mutant) | mutant red | not-run | |
| CWS-6 | meta-test: guard in non-required job, unpinned mode | red | not-run | |
| CWS-6 | ruleset lists the four jobs as required | listed | post-merge | operator, task 5.1 |
| CWS-7 | self-test mutants plus no-op control, by exit status | each mutant red, no-op green | not-run | |
| all | independent review; CI on the PR head | APPROVE, green; reviewed and archive revisions recorded | not-run | |
