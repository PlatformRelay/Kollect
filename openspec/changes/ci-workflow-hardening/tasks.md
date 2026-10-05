# Tasks

## 1. Meta-test first (CWS-1 to CWS-7)

- [ ] 1.1 Probe `runs-on` values, and list which jobs `ci_docs_gate_test.sh` pins to an exact step count (the exemption list);  write `hack/test/ci_workflow_security_test.sh` with the checks and mutants for CWS-1 to CWS-6; run it and watch it fail on the missing wiring (assertion output named)
- [ ] 1.2 Wire it into the `lint` job next to `ci_docs_gate_test.sh`

## 2. harden-runner and concurrency (CWS-1, CWS-5, CWS-6)

- [ ] 2.1 Add the `concurrency:` block to `ci.yaml` and fix the group in `e2e-smoke.yaml`; test passes for CWS-5
- [ ] 2.2 Add harden-runner (audit, digest-pinned) as first step of every job; test passes for CWS-1

## 3. zizmor (CWS-2, CWS-3)

- [ ] 3.1 Add `hack/install-zizmor.sh` (pinned release, checksum via `hack/lib/fetch.sh`, same shape as `install-gitleaks.sh`); probe `--offline` on the pinned release
- [ ] 3.2 Run zizmor on the tree; fix or justify every finding in `.github/zizmor.yml` (a fix that changes a workflow's behaviour is its own commit)
- [ ] 3.3 Add the `workflow-security` job; add it to `verify-eligibility.sh` `required_checks` and its test (`hack/release/test-verify-eligibility.sh`)

## 4. dependency-review (CWS-4)

- [ ] 4.1 Add the `dependency-review` job (PR only, `fail-on-severity: high`)
- [ ] 4.2 Probe on a throwaway PR that adds a known-vulnerable module: job fails

## 5. Operator-owned

- [ ] 5.1 (operator) Make `workflow-security` a required check in the `protect-main` ruleset once it has been green on `main` for a week
- [ ] 5.2 (operator) Decide whether to schedule the block-mode follow-up after reviewing audit data

## 6. Land

- [ ] 6.1 Archive the change (`openspec archive`) as the last commit of the PR, after review and green CI; the review record names the reviewed revision and the archive revision

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| CWS-1 | `ci_workflow_security_test.sh` harden-runner check + mutant (remove from one job) | passes on tree, fails on mutant | not-run | |
| CWS-2 | `workflow-security` job on the PR head; template-injection fixture | green on tree, red on fixture | not-run | |
| CWS-3 | meta-test, bare-suppression mutant | fails on mutant | not-run | |
| CWS-4 | `dependency-review` on a throwaway PR with a vulnerable module | job red | not-run | |
| CWS-5 | meta-test group/expression check (ref-keyed mutant); two pushes to a PR; three quick merges to main | first PR run cancelled; every main SHA has a run | not-run | |
| CWS-6 | meta-test with `push` removed (mutant); push run on merge SHA | mutant fails; push run reports | not-run | |
| CWS-7 | self-test mutants plus a no-op control, judged by exit status | each mutant red, no-op green | not-run | |
| all | independent review, CI on PR head | APPROVE, green; record both the reviewed and the archive revision | not-run | |
