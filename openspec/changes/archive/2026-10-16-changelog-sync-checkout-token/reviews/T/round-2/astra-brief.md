You are reviewing one task, independently of whoever implemented it. You are in the git worktree of
the kollect repository (read-only). You have five inputs:

1. TASK — OpenSpec change `openspec/changes/changelog-sync-checkout-token/`: read `proposal.md`,
   `specs/changelog-sync/spec.md` (CST-1, CST-2) and `tasks.md` (tasks 1.1, 2.1, 3.1). The same
   range also edits `openspec/changes/dependency-update-automation/` to remove DUA-6 from it and
   apply re-measure facts; check that DUA-6 is fully removed there and that the remaining text is
   consistent (no dangling reference to changelog-sync token isolation as in-scope work).
2. MATRIX — the `## Verification` table in that `tasks.md`, and `evidence/T1.1.md` (probe) and
   `evidence/T2.1.md` (guard red/green, mutants, sensors).
3. DIFF — `git diff a9913eda9...2e3640092` (four commits). This is ROUND 2: round 1 reviewed `bcd248619` (reports in `openspec/changes/changelog-sync-checkout-token/reviews/T/round-1/`); commit `2e3640092` claims to fix the round-1 P1 (bracket spelling `steps['id']['outputs']`) and adds `toJSON(steps)` and App-secret checks. Verify each round-1 finding is closed and that the fix introduced no new defect. The code under judgment is
   `.github/workflows/changelog-sync.yaml` (the checkout's `token:` removed) and
   `hack/test/changelog_sync_release_guard_test.sh` (new `credential_errors` in the embedded
   `syncwf.py`, wired into section 6 `static_wiring_check`; new mutants in section 8; a
   `|| return 0` change in `mutant_rejected`/`mutant_accepted`).
4. SENSORS (reported by the implementer, exit codes): guard `bash hack/test/changelog_sync_release_guard_test.sh`
   1 on the pre-fix workflow, 0 after; `hack/test/sonar_ko_08_workflow_permissions_test.sh` 0;
   `hack/test/ci_workflow_security_test.sh` 0 and `--self-test` 0; zizmor 1.30.1 `--offline
   --min-severity=high --config .github/zizmor.yml .github/` no findings; `task lint:shell` 0;
   `task spec:validate` 0; `task lint:markdown` 0.
5. RULES — the repository's AGENTS.md / docs/development/spec-workflow.md: TDD (red before green on
   the named assertion), mutation controls judged by exit status with a no-op control, evidence
   names revision and limitations, nothing private ships.

The load-bearing premise: removing `token:` from the checkout does not break the push to `main`.
Check it against the workflow itself (how the push authenticates, whether any later step or tool
relies on credentials the checkout set), not against the evidence prose.

Judge four things, in this order:
a. INTENT — does the diff do what the tasks ask, all of it, and nothing they did not ask for?
b. MATRIX — for each row: is the sensor real, does it exercise the claim, would it fail if the
   claim were false? You may run the guard (it takes seconds; it needs python3 with PyYAML) and
   mutate a scratch COPY of the workflow outside the repo if your sandbox allows writes there;
   otherwise reason from the code. Dispute rows that are vacuous or over-classified.
c. RULES — any rule the diff touches or violates.
d. AI-code failure modes — hallucinated APIs or action inputs, tautological tests, weakened
   assertions, silently dropped halves, prose that claims more than the code does.

For each P0/P1 you are about to file, argue the opposite for one line; keep only what survives.
An APPROVE with no findings is valid when substantiated.

RETURN ≤15 lines, nothing else:
verdict: APPROVE | REQUEST_CHANGES | BLOCK
findings:
  - [P0|P1|P2] <one line, with path:line>
matrix_disputes:
  - <row claim> — <why the sensor/class does not prove it>
could_not_check:
  - <what you could not verify>
summary: <one line>

Severity: P0 (security, invariant broken, vacuous gate presented as proof) → BLOCK. P1
(correctness gap, untested new behaviour, dishonest evidence, missing half) → REQUEST_CHANGES.
P2/P3 → note only.
