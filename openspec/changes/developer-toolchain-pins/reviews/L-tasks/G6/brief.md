# Review brief: task group G6 (6.1 to 6.8) of openspec change developer-toolchain-pins

You are reviewing one task group, independently of whoever implemented it. Read-only: do not edit
files in the repository (scratch copies under /tmp for mutation checks are fine; free each copy,
/tmp inodes are scarce). Repo root: the current directory.

Inputs:

1. TASK: `openspec/changes/developer-toolchain-pins/tasks.md` section "6. Stage-B fixes (round 1)"
   (6.1 to 6.8; also quoted in the evidence). Spec: `openspec/changes/developer-toolchain-pins/specs/developer-toolchain/spec.md`
   (DTP-1 to DTP-7). The findings being fixed: `openspec/changes/developer-toolchain-pins/reviews/B-branch/round-1/verification.md`.
2. MATRIX and SENSORS: `openspec/changes/developer-toolchain-pins/evidence/G6.md` (matrix, red runs,
   hand mutants, gate runs with exit codes and times).
3. DIFF: `git diff b1f5a4369..851c1aa76` (seven commits; the evidence file is uncommitted and not
   under review as code).
4. RULES: AGENTS.md (repo), docs/development/spec-workflow.md. Guards: plain mode checks the tree,
   --self-test corrupts throwaway copies and asserts each check reds with its own message, plus a
   no-op copy that passes. A self-test value must never be a plausible next release unless read
   from the tree. One clause per mutant.

Key points to judge:

- 6.1: is the all-pins bump control a faithful model of a Renovate group PR (every extracted site,
  consistent values, shellcheck digests), does its probe prove the bump landed, and is any other
  self-test sentinel/probe still tied to a current pin value? Recursion guard `KOLLECT_TP_NESTED`.
- 6.2: CI wiring in `.github/workflows/ci.yaml` (`tool-pins-selftest` job) and CFC-6 in
  `hack/test/ci_workflow_security_test.sh`; are the changed change-4 mutants still precise?
- 6.3/6.4: `hack/install-git-cliff.sh`, `hack/lib/verify-sha256.sh`, `hack/install-helm-docs.sh`
  and the installer cases in `hack/test/consistency_toolchain_pins_test.sh` (`hashless_path`).
- 6.5: `hack/test/consistency_tool_pins_test.sh` Taskfile-var resolution.
- 6.6/6.7: case-insensitive `uses:` reader, `ALONE` groups, `renovate.json`.

Judge four things, in this order:
a. INTENT: does the diff do what each task asks, all of it, and nothing it did not ask for?
b. MATRIX: for each row, is the sensor real, would it fail if the claim were false, is the class
   honest? Mutation-verify clauses you doubt in a scratch copy (anything under a minute; trust the
   multi-minute self-test exit codes as reported).
c. RULES: any rule the diff touches or violates (security posture of the installers included).
d. AI-code failure modes: tautological tests, weakened assertions, dropped halves, prose claiming
   more than the code does.

For each P0/P1 you are about to file, argue the opposite for one line; keep only what survives.
An APPROVE with no findings is valid when substantiated.

RETURN at most 15 lines, nothing else:

```text
## Verdict
verdict: APPROVE | REQUEST_CHANGES | BLOCK
findings:
  - [P0|P1|P2] <one line, with path:line>
matrix_disputes:
  - <row> -- <why>
could_not_check:
  - <item>
summary: <one line>
```
