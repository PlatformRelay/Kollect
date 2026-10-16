# Review brief — task 1.2 of developer-toolchain-pins (diff-only, independent)

You are reviewing one task, independently of whoever implemented it. Read-only: do not edit files.
Repo root: the current directory (kollect, branch `feat/developer-toolchain-pins`).

1. TASK — `openspec/changes/developer-toolchain-pins/tasks.md` item 1.2 (fold
   `hack/test/dev_mise_pin_drift_test.sh` into `hack/test/consistency_toolchain_pins_test.sh`: port
   every old mutant one-to-one into `--self-test`, show each red on the new guard, list the mapping,
   then delete the old guard and its `ci.yaml` step and update every live reference, including the
   CWS-6 composite-action mutant in `hack/test/ci_workflow_security_test.sh` and `renovate.json:74`).
   Spec: `openspec/changes/developer-toolchain-pins/specs/developer-toolchain/spec.md` (DTP-5 Go half
   and DTP-7). Repo rules: AGENTS.md, docs/development/spec-workflow.md.
2. MATRIX and SENSORS — `openspec/changes/developer-toolchain-pins/evidence/T1.2.md` (verification
   matrix, mapping table, sensors run, hand mutants, harness gaps).
3. DIFF — the staged change: `git diff --cached 75586033c` (base 75586033c; nothing committed yet).
   The old guard is at `git show 75586033c:hack/test/dev_mise_pin_drift_test.sh`.
4. RULES — guards: plain mode checks the real tree, `--self-test` corrupts throwaway copies per
   mutant and asserts non-zero with the clause's own message, plus a no-op copy that must pass.
   A test may not be weakened to get green; coverage must not be lost in the migration.

Judge, in order:
a. INTENT — does the diff do all of 1.2 and nothing else? Is every old case really ported (same
   failure, same trigger), and does the new plain check reject everything the old one did (compare
   regexes and control flow with the old file)? Is any live reference to the old name left
   (`grep -rn dev_mise_pin_drift`; records under openspec/changes/archive, change 4's records and
   review files are allowed)?
b. MATRIX — is each sensor real, would it fail if the claim were false? Mutation-verify in a
   scratch copy (mktemp, never the repo) any clause you doubt: `bash <guard copy> --self-test <repo>`
   runs in about 60 s. Does each probe prove its mutant landed?
c. RULES — CI wiring (CWS-6 / CFC-6 still hold the guard in a required job, both modes).
d. AI-code failure modes — tautological probes, weakened assertions, dropped halves, prose that
   claims more than the code does.

For each P0/P1, argue the opposite for one line; keep only what survives. An APPROVE with no
findings is valid when substantiated.

RETURN at most 15 lines, nothing else, under a `## Verdict` heading:
verdict: APPROVE | REQUEST_CHANGES | BLOCK
findings:
  - [P0|P1|P2] <one line, with path:line>
matrix_disputes:
  - <row> — <why>
could_not_check:
  - <what>
summary: <one line>
