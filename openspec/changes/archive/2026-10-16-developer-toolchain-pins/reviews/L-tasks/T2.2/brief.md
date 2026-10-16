# Review brief — task 2.2 of developer-toolchain-pins (diff-only, independent)

You are reviewing one task, independently of whoever implemented it. Read-only: do not edit files.
Repo root: the current directory (kollect, branch `feat/developer-toolchain-pins`).

1. TASK — `openspec/changes/developer-toolchain-pins/tasks.md` item 2.2 (full text also in the
   evidence file under `## Task text`): add the DTP-2 check (Taskfile.yml defines
   `GOVULNCHECK_VERSION` under `# renovate: datasource=go depName=golang.org/x/vuln`, and
   `task vulncheck` uses it), with self-test cases: literal `govulncheck@v1.9.0` in vulncheck → red;
   annotation deleted → red; vulncheck reads another variable with the same value while the
   annotated one stays defined → red naming the missing `GOVULNCHECK_VERSION` reference; definition
   removed → red with a missing-definition message. Then move the pin. Re-anchor change 4's two
   CFC-2 mutants in `hack/test/consistency_tool_pins_test.sh` that `sed` the literal
   `govulncheck@v…` onto the new variable, and keep the two version-free ones applying and red.
   Confirm CFC-2's Taskfile variable lookup (~:162) resolves `{{.GOVULNCHECK_VERSION}}`. Record a
   real `task vulncheck` run. One commit.
   Spec: `openspec/changes/developer-toolchain-pins/specs/developer-toolchain/spec.md` (DTP-2, DTP-7).
   Repo rules: AGENTS.md, docs/development/spec-workflow.md.
2. MATRIX and SENSORS — `openspec/changes/developer-toolchain-pins/evidence/T2.2.md` (matrix, red
   before the move, CFC-2 mutant table, Renovate dry-run, `task vulncheck`, sensors, hand mutants,
   test changes, deviations).
3. DIFF — the working tree against base 9fab7adae: `git diff 9fab7adae` (nothing committed yet; the
   evidence file and this brief are intent-to-add). Files: `Taskfile.yml`, `.github/workflows/ci.yaml`
   (one step name and its comment), `hack/test/consistency_toolchain_pins_test.sh`,
   `hack/test/consistency_tool_pins_test.sh`.
4. RULES — guards: plain mode checks the real tree, `--self-test` corrupts throwaway copies per
   mutant and asserts non-zero WITH the clause's own message and no other clause tripped, plus a
   no-op copy that must pass. Tests owned by another change (change 4's CFC-2 self-test) change only
   with evidence from the spec, listed under `## Test changes`.

Judge, in order:
a. INTENT — all of 2.2 and nothing else? Does `check_govulncheck` enforce each DTP-2 scenario, and
   does its reading of the `vulncheck:` task block (awk in `vulncheck_lines`) end where the task
   ends? Is the Renovate annotation in the form the Taskfile custom manager in `renovate.json`
   (~:27-36) extracts?
b. MATRIX — is each sensor real, would it fail if the claim were false? Mutation-verify in a scratch
   copy (mktemp, never the repo) any clause you doubt: `bash <guard copy> --self-test <repo>` runs
   in about 70 s. Does each probe prove its mutant landed? Are the two re-anchored CFC-2 mutants
   still testing what their labels say (do not rerun that self-test: it takes ~7 min; its result is
   in the evidence)?
c. RULES — CI wiring (the lint step was renamed; CWS-6 / CFC-6 must still hold the guard in both
   modes); workflow security (only one step name and comment changed in YAML).
d. AI-code failure modes — tautological probes, weakened assertions, dropped halves, comments or
   evidence that claim more than the code does.

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
