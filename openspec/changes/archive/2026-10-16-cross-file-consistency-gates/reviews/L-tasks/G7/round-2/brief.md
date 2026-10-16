You are reviewing one task group of an OpenSpec change, independently of whoever implemented it.
Repository: the current directory (kollect). Read-only: do not modify files; use scratch copies under a
temp dir for any mutation checks.

1. TASK — tasks 7.1–7.5 of openspec/changes/cross-file-consistency-gates/tasks.md (stage-B round-1 fixes):
   - 7.1 Fix markdownlint (MD032, MD038) in this change's loop.md, task-prompt.md and evidence/*.md so `task lint:markdown` is green (H1). Constraint: no content change in loop.md/task-prompt.md, tables not broken.
   - 7.2 CFC-1: fail on a `FROM` whose image is a variable (or resolve it from its `ARG` default); fixture + mutant. Must cover `FROM ${B}`, `FROM $B`, `FROM ${B:-golang:1.27.2}`.
   - 7.3 CFC-4: add a Go test that gathers the metrics registry and compares the registered names with `registeredMetricNames`, so dropping a collector from `MustRegister` goes red; mutant.
   - 7.4 consistency_go_toolchain_test.sh: call `new_copy` in the parent shell so the EXIT trap removes every copy (no temp-dir leak).
   - 7.5 consistency_rbac_test.sh: reject a malformed exception line (class token without colon) with a clear FAIL instead of a Python traceback.
   Spec: openspec/changes/cross-file-consistency-gates/specs/cross-file-consistency/spec.md (CFC-1, CFC-4, CFC-6).
   Repo rules: AGENTS.md, docs/development/spec-workflow.md. Guards follow hack/test/dev_mise_pin_drift_test.sh: plain mode checks the real tree, `--self-test` corrupts throwaway copies and asserts the named failure, plus a no-op control.
2. MATRIX — openspec/changes/cross-file-consistency-gates/evidence/G7.md, section "Verification matrix" (and "Design decisions", "Red first", "Sensors run").
3. DIFF — `git diff 028bbc44c..HEAD` (6 commits) — the only code you judge. The evidence file G7.md is uncommitted in the working tree.
   This is review round 2 (the last). Round 1 (a different reviewer) found two P1s, both fixed: (i) `FROM localhost:5000/${B}:1.99.0` bypassed the variable-image check (fix 6dd85621f); (ii) the metrics test never called Register(), so Register() pointing at another registry stayed green (fix 7fe5b7708). Check the fixes hold and introduced no regression; judge the whole diff, but only file what you verify.
4. SENSORS — as recorded in G7.md "Sensors run" (exit codes trusted; the final-tree row is pending and runs after review).
5. RULES — no constitution; AGENTS.md: TDD, fail-closed guards, no weakened tests, never widen an allow-list to get green.

Judge four things, in this order:
a. INTENT — does the diff do what TASK asks, all of it, and nothing the task did not ask for?
b. MATRIX — for each row: is the sensor real, is the evidence class honest, would the sensor fail if the claim were false? Mutation-verify each new clause in a scratch copy (each guard takes a tree root or `--self-test <root>`; `go test ./internal/metrics/` takes ~2 s). Dispute vacuous or over-classified rows.
c. RULES — any rule the diff touches or violates.
d. AI-code failure modes — hallucinated APIs, tautological tests, weakened assertions, silently dropped halves of a multi-part task, scope widened to make a sensor pass, prose that claims more than the code does. In particular: can a variable image still slip past 7.2 (other spellings)? Does the 7.3 test really see what Register() registers into controller-runtime's registry?

For each P0/P1 you are about to file, argue the opposite for one line; keep only what survives.
An APPROVE with an empty findings list is valid when substantiated.

RETURN ≤15 lines, nothing else, starting with a `## Verdict` heading:
## Verdict
verdict: APPROVE | REQUEST_CHANGES | BLOCK
findings:
  - [P0|P1|P2] <one line, with path:line>
matrix_disputes:
  - <row claim> — <why the sensor/class does not prove it>
could_not_check:
  - <what you did not verify>
summary: <one line>
