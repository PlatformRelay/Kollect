# Cold review brief: task group G5 (5.1-5.6) of OpenSpec change `test-depth-signals` (kollect)

You are reviewing one task group, independently of whoever implemented it. You have read-only
access to this repository (cwd is the repo root, branch `feat/test-depth-signals`, HEAD
`5c0196072`). You have no network and cannot write `/tmp` or the tree. Read the files; do not
guess. Change nothing.

## Inputs

1. TASK: `openspec/changes/test-depth-signals/tasks.md`, section "5. Stage-B fixes (round 1)",
   tasks 5.1-5.6. They fix the findings of the stage-B branch review recorded in
   `openspec/changes/test-depth-signals/reviews/B-branch/round-1/verification.md` (A1-A4, O1-O3,
   and the table "Deferred to stage B"). Spec: `openspec/changes/test-depth-signals/specs/test-depth/spec.md`
   (TDS-1 fuzz runner, TDS-2 nightly shuffle, TDS-3 README quick start); 5.6 edits TDS-3's text.
2. MATRIX: `openspec/changes/test-depth-signals/evidence/G5.md` (one section per task, then the
   gate run). The gate section is in the working tree, not yet committed.
3. DIFF: `git diff 31747e9c0..5c0196072` (`git log --oneline 31747e9c0..5c0196072`, six commits,
   one per task). Files: `hack/ci/fuzz-retry.sh`, `hack/ci/test-shuffle.sh`,
   `hack/test/fuzz_retry_test.sh`, `hack/test/test_shuffle_test.sh`,
   `hack/test/readme_quickstart_test.sh`, the spec, tasks.md, loop.md, evidence/G5.md.
4. SENSORS: the table "Gate run on the final G5 tree" in evidence/G5.md (all exit 0).
5. RULES: `AGENTS.md` and `docs/development/spec-workflow.md` (no constitution). Guards follow
   `hack/test/ci_workflow_security_test.sh`: plain mode checks the real tree, `--self-test`
   mutates a throwaway copy per mutant and asserts the named case reds, plus a no-op control.

## Judge, in this order

a. INTENT: does each commit do what its task asks, all of it, and nothing else? In particular:
   - 5.1: is every `kubectl get` operand's type checked once any operand is `TYPE/NAME`, and do
     plain NAME operands after a type list stay unchecked names?
   - 5.2: can `if:` or `continue-on-error:` still skip or soften the fuzz runner step, the
     download step, the fuzz job, the shuffle job or any of its steps without a red case?
   - 5.3: is a retry now possible only after an attempt that exited 1?
   - 5.4: does S13 fail if a failed Ginkgo run is swallowed or stops the other runs?
   - 5.5: does a failing `make setup-envtest` print the script's own `::error::` line; is an
     `internal: true` task or include refused; do F0 (empty pkg) and F18 (crash fixture) kill
     the mutants the evidence names?
   - 5.6: does the amended TDS-3 text match what `readme_quickstart_test.sh` does, no more and
     no less?
b. MATRIX: for each row, is the sensor real, and would it fail if the claim were false? You
   cannot write `/tmp`; reason about mutants from the code, and say which you could not run.
c. RULES: anything in AGENTS.md the diff violates (commit format, nothing private, no attribution).
d. AI-code failure modes: tautological cases, weakened assertions, a fake that cannot fail, prose
   claiming more than the code does, a dropped half of a task.

For each P0/P1 you are about to file, argue the opposite for one line; keep only what survives.
An APPROVE with no findings is valid when substantiated.

RETURN at most 15 lines, nothing else:

```
## Verdict
verdict: APPROVE | REQUEST_CHANGES | BLOCK
findings:
  - [P0|P1|P2] <one line, with path:line>
matrix_disputes:
  - <row claim> — <why the sensor does not prove it>
could_not_check:
  - <what you could not verify>
summary: <one line>
```

Severity: P0 (security, invariant broken, vacuous gate presented as proof) → BLOCK; P1
(correctness gap, untested new behaviour, dishonest evidence, missing half of a task) →
REQUEST_CHANGES; P2 → note only.
