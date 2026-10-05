# Tasks

## 1. Test first (PAC-1 to PAC-5)

- [ ] 1.1 Write `hack/test/agent_contract_test.sh` with mutants: missing file, unknown task, long pointer, missing pointer target, stale checklist path, scrub string, home path, harness path, ignored again; watch it fail on the missing-file assertion

## 2. Contract

- [ ] 2.1 Derive the three checklists by dry-run (change one CRD field, one metric, stub one sink on a scratch branch) and record which gates fired; reconcile with the proposal's lists
- [ ] 2.2 Write `AGENTS.md` (commands that exist, CI-enforced rules, conventions, checklists); do not restate CONTRIBUTING.md
- [ ] 2.3 Add the two pointers; adjust `.gitignore`; add the pointer line to `CONTRIBUTING.md`
- [ ] 2.4 Wire the test into `lint`; run `bash hack/scrub.sh` with the files staged as well

## 3. Review

- [ ] 3.1 Read the committed text as a stranger: every sentence true on a fresh clone; no private gotcha leaked (independent reviewer checks)

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| PAC-1 | test file-present and task-exists checks; `git ls-files AGENTS.md` in a fresh clone | green; mutant red | not-run | |
| PAC-2 | pointer length and target mutants | red | not-run | |
| PAC-3 | stale-path mutant; dry-run record from 2.1 | red; no unnamed gate fired | not-run | |
| PAC-4 | scrub, home-path, harness-path and gitignore mutants; `task scrub` with files staged | each mutant red; scrub ok | not-run | |
| PAC-5 | self-test plus no-op copy, exit status | mutants red, no-op green | not-run | |
| all | CI on the PR head; independent review | green; APPROVE | not-run | |
