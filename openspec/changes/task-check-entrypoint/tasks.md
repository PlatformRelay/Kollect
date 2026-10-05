# Tasks

## 1. Guard first (TCE-1 to TCE-4)

- [ ] 1.1 Probe: list the guards under `hack/test/` and how each declares its modes; grep for existing `check` references
- [ ] 1.2 Write `hack/test/task_check_test.sh` (plain and `--self-test`) with mutants: gate removed, `verify` redefined, exclusion without reason, new-guard-not-run; watch it fail on the missing task
- [ ] 1.3 Wire it into the `lint` job, plain and `--self-test` as separate steps

## 2. Implement

- [ ] 2.1 Add `check` (iterates the guard glob, aggregates failures, prints exclusions with reasons); guard passes
- [ ] 2.2 Update `CONTRIBUTING.md` and `docs/COMMAND-REFERENCE.md`

## 3. Land

- [ ] 3.1 Archive the change as the last commit of the PR, after review and green CI; the review record names the reviewed and the archive revision

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| TCE-1 | `task check` on a machine without Docker; new-guard and failing-gate cases | runs all runnable gates; exits non-zero on failure | not-run | |
| TCE-2 | guard mutant redefining `verify` | red | not-run | |
| TCE-3 | guard mutants: unreachable job, reasonless exclusion | red | not-run | |
| TCE-4 | self-test mutants plus no-op, by exit status | mutants red, no-op green | not-run | |
| all | independent review; CI on the PR head | APPROVE, green; both revisions recorded | not-run | |
