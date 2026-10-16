# Tasks

## 0. Hard prerequisites (check before any other task)

- [x] 0.1 (operator) `lint`, `vulncheck`, `workflow-security`, `dependency-review` are required checks in `protect-main` (change 1, task 5.1); record the ruleset JSON — done 2026-10-16: required checks are `preflight`, `test`, `kind-smoke`, `Analyze (Go)`, `lint`, `workflow-security`, `dependency-review`, `tool-pins-selftest`, `vulncheck`
- [ ] 0.2 `install-git-cliff.sh` verifies a checksum (change 5, DTP-6) and is merged
- [x] 0.3 (operator) Probe whether `require_extra_approval_for_unattributed_changes` in the ruleset blocks bot-authored merges; record the result — operator: no observed effect (2026-10-16)

## 1. Guard first (DUA-1 to DUA-5, DUA-7)

- [ ] 1.1 Write `hack/test/renovate_automerge_test.sh` (plain and `--self-test`) over `renovate.json` and `renovate.yaml`: token step, no fallback, daily cron, strategy, `platformAutomerge`, scope, deny rules last, age, secret names, no `pull_request_target` merge; mutants for each scenario; watch it fail on the assertions
- [ ] 1.2 Wire it into `lint` (separate steps for plain and `--self-test`); grep `hack/test/` for step-index assertions on `renovate.yaml` before editing it

## 2. Config

- [ ] 2.1 `renovate.yaml`: App token, no fallback, fail on missing secrets, slug check, daily cron
- [ ] 2.2 `renovate.json`: automerge for gomod patch/minor non-Kubernetes, rebase, `platformAutomerge: false`, `minimumReleaseAge: "7 days"`, `rebaseWhen`, `prCreation`, groups, deny rules last
- [ ] 2.3 `hack/ci/renovate-automerge-gate.sh`: latest push run of `ci.yaml` on `main`; fake-`gh` tests for green, red, in progress, lookup error, red-then-green; emits the `RENOVATE_FORCE` value; wire into the workflow; probe that `RENOVATE_FORCE` carries `automerge: false`
- ~~2.4 `changelog-sync.yaml` token isolation~~ — struck 2026-10-16: `persist-credentials: false` is already on `main`; the `token:` half (formerly DUA-6) moved to the change `changelog-sync-checkout-token`
- [ ] 2.5 Validate the config with `renovate-config-validator --strict`

## 3. Operator-owned and live

- [ ] 3.1 (operator; the operator will create it, 2026-10-16) Create the Renovate App (contents, pull-requests, workflows write; no ruleset bypass); add `RENOVATE_GITHUB_APP_ID` and `RENOVATE_GITHUB_APP_PRIVATE_KEY` secrets and the changelog-sync App slug variable
- ~~3.2 (operator) Enable rebase merging~~ — struck 2026-10-16: rebase merging is already the only allowed merge method; the behind-base probe moves to 3.3
- [ ] 3.3 Dispatch Renovate: a PR opens and CI starts; first eligible patch PR auto-merges with rebase after all checks are green; a behind-base PR is rebased and merged at a later run

## 4. Land

- [ ] 4.1 Merge with the `post-merge` rows open; archive in a follow-up evidence PR once they pass

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| DUA-1 | guard token-step, no-fallback and cron mutants | mutants red | not-run | |
| DUA-1 | live dispatch: PR opens and CI starts | CI run on the bot PR | post-merge | needs 3.1; follow-up evidence PR |
| DUA-2 | guard strategy mutants; live eligible PR | mutants red | not-run | |
| DUA-2 | rebase merge observed; red-check PR stays open | observed | post-merge | follow-up evidence PR |
| DUA-3 | `jq` structural guard: deny rules last; mutant appending a rule; action, tool, digest fixtures | guard green on tree, red on mutants | not-run | |
| DUA-4 | guard age mutant | red | not-run | |
| DUA-5 | slug-equal and same-secret mutants | red | not-run | |
| DUA-5 | ruleset lists no bypass for the Renovate App | none listed | post-merge | operator evidence |
| DUA-6 | moved to the change `changelog-sync-checkout-token` | — | N/A | split out 2026-10-16 |
| DUA-7 | gate-script tests (green, red, in progress, error, red-then-green); `pull_request_target` mutant | as specified | not-run | |
| DUA-8 | review record cites tasks 0.1 and 0.2 evidence | cited | not-run | |
| all | independent review; CI on the PR head | APPROVE, green; reviewed and archive revisions recorded | not-run | |
