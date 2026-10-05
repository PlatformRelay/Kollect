# Tasks

## 1. Guard first (DUA-1 to DUA-5)

- [ ] 1.1 Write `hack/test/renovate_automerge_test.sh` over `renovate.json` and `renovate.yaml` (token step, no `github.token` fallback, strategy, scope, age, distinct secret names) with mutants for each scenario; watch it fail on the assertions
- [ ] 1.2 Probe how to evaluate the final rule set (for example `renovate-config-validator --strict` plus a dry run `LOG_LEVEL=debug renovate --platform=local`); record what the test uses

## 2. Config (DUA-1 to DUA-4)

- [ ] 2.1 Change `renovate.yaml` to mint the App token, drop the fallback and fail on missing secrets
- [ ] 2.2 Add automerge, `automergeStrategy: "rebase"`, `platformAutomerge`, `minimumReleaseAge: "7 days"` and the exclusions to `renovate.json`; guard passes
- [ ] 2.3 Wire the guard into `lint`; validate the config with the Renovate validator

## 3. Live probes

- [ ] 3.1 Manual `workflow_dispatch` of Renovate: a PR opens and CI starts on it (records the run link); a workflow-file bump is pushed without a permission error
- [ ] 3.2 First eligible patch PR auto-merges with rebase after green checks, or stays open when a check is red (record both if available)

## 4. Operator-owned

- [ ] 4.1 (operator) Create the Renovate GitHub App (contents, pull-requests, workflows write; metadata read), install on the repo, no ruleset bypass
- [ ] 4.2 (operator) Add `RENOVATE_GITHUB_APP_ID` and `RENOVATE_GITHUB_APP_PRIVATE_KEY` repository secrets
- [ ] 4.3 (operator) Enable "Allow auto-merge" and confirm rebase merging is allowed; confirm `protect-main` lists no bypass for the new App
- [ ] 4.4 (operator) Delete the stale `renovate/*` branches that never got PRs, or let Renovate adopt them

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| DUA-1 | guard token-step and no-fallback checks; live dispatch | mutant red; PR has CI | not-run | needs operator tasks 4.1-4.2 |
| DUA-2 | guard strategy mutants (squash, merge); live eligible PR | mutants red; rebase merge observed | not-run | needs 4.3 |
| DUA-3 | guard evaluation of the final rule set on patch, major, k8s.io fixtures | automerge only for the patch fixture | not-run | |
| DUA-4 | guard age mutant | red when removed or under 7 days | not-run | |
| DUA-5 | guard distinct-secret check; ruleset inspected by the operator | mutant red; no bypass listed | not-run | operator evidence |
| all | CI on the PR head; independent review | green; APPROVE | not-run | |
