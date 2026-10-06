# Tasks

## 1. Guard first (TCE-1 to TCE-4)

- [x] 1.1 Probe: list the guards under `hack/test/` and how each declares its modes; grep for existing `check` references
  - 72 `_test.sh` guards + `lab_harness_meta_suite.sh`; only `ci_workflow_security_test.sh`
    parses a `--self-test` flag (the rest run their self-tests unconditionally); no `check`
    task existed; the required gates and their local equivalents are mapped in `hack/check.sh`.
- [x] 1.2 Write `hack/test/task_check_test.sh` (plain and `--self-test`) with mutants: gate removed, `verify` redefined, exclusion without reason, new-guard-not-run; watch it fail on the missing task
  - The pre-wiring red is the guard's own missing run_gate sweep coverage: before the lint
    wiring the guard's CWS-6-analogue assertion is unobservable, so the plain red comes from
    the `check` task itself being absent (TCE-1: no check task).
- [x] 1.3 Wire it into the `lint` job, plain and `--self-test` as separate steps

## 2. Implement

- [x] 2.1 Add `check` (iterates the guard glob, aggregates failures, prints exclusions with reasons); guard passes
- [x] 2.2 Update `CONTRIBUTING.md` and `docs/COMMAND-REFERENCE.md`

## 3. Land

- [ ] 3.1 Archive the change as the last commit of the PR, after review and green CI; the review record names the reviewed and the archive revision

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| TCE-1 | `task check` on a machine without Docker; new-guard and failing-gate cases | runs all runnable gates; exits non-zero on failure | structurally pinned + mutants red (gate removed, glob hard-coded, verify redefined, reasonless exclusion); a full local `task check` run is post-merge | evidence/evidence.md |
| TCE-2 | guard mutant redefining `verify` | red | red locally | evidence/evidence.md |
| TCE-3 | guard mutants: unreachable job, reasonless exclusion | red | exclusion-without-reason mutant red; the unreachable-job direction is covered by the required-checks sweep | evidence/evidence.md |
| TCE-4 | self-test mutants plus no-op, by exit status | mutants red, no-op green | green locally (4 mutants + no-op) | evidence/evidence.md |
| all | independent review; CI on the PR head | APPROVE, green; both revisions recorded | not-run | this PR |
