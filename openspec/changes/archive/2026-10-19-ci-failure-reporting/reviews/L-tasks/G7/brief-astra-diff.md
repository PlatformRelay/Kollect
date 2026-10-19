# Review brief — G7 (tasks 7.1–7.4), diff only

You are reviewing one task group, independently of whoever implemented it. Read-only: do not edit
files, do not run git commands that change state, do not call the real `gh`. Work from the
repository root.

## 1. TASK

Feature: `openspec/changes/ci-failure-reporting/` (spec: `specs/ci-failure-reporting/spec.md`,
CFR-1..CFR-7). Group G7 fixes stage-B review findings (described in
`reviews/B-branch/round-1/verification.md`, rows B1, B2, B3, H1):

- 7.1 B1: the reporter (`hack/ci/ci-failure-report.sh`) requires exactly one JSON array from each
  gh read (run list, issue list, comments); malformed or empty output with gh exit 0 is an error
  (exit non-zero, no write). Fake-gh cases for empty and malformed output on each read, red first.
- 7.2 B2: add `hack/test/ci_failure_report_test.sh` to the CFC-6 fixed guard list in
  `hack/test/ci_workflow_security_test.sh` (plain and --self-test), with a mutant deleting both
  lint steps that CWS-6/CFC-6 catches.
- 7.3 B3: move `concurrency:` from workflow level to the `report` job in
  `.github/workflows/ci-failure-report.yaml`; update the wiring assertion and its mutant.
- 7.4 H1 (pre-existing on main): `hack/check.sh` detects a guard's `--self-test` mode with
  `grep -v | grep -q` under pipefail, which SIGPIPEs on large guards and silently skips their
  self-test. Use `grep … >/dev/null`; add a `task_check_test.sh` case with a guard larger than the
  pipe buffer whose self-test must be run.

## 2. MATRIX

`openspec/changes/ci-failure-reporting/evidence/G7.md`, section *Verification matrix* (and the
per-task sections with the recorded reds, *Test changes*, *Sensors (pre-review)*).

## 3. DIFF

`git diff 0f7455c73..a824dd398` (four commits, one per task). The evidence file is uncommitted in
the working tree; judge it as part of the matrix, not as code.

## 4. SENSORS

As recorded in G7.md *Sensors (pre-review)*: reporter test plain 0 and `--self-test` 0 (3:37);
`ci_workflow_security_test.sh` plain 0 and `--self-test` 0; `task_check_test.sh` both modes 0;
`task lint:shell` 0; zizmor gate 0. The full `task check` is pending (runs once after review).
Trust these exit codes; you may rerun anything under a minute (the two `task_check_test.sh`
modes take seconds; `CFR_SELFTEST_ONLY='gh-output-.*' bash hack/test/ci_failure_report_test.sh`
runs only the 7.1 cases). If you need scratch space and cannot write /tmp, say so in
could_not_check rather than guessing.

## 5. RULES

No constitution. Repo rules: guards follow the plain-mode plus `--self-test` (mutated copies, one
mutant per clause, judged by the message, a no-op control) pattern; never weaken a test to get
green; this workflow is privileged (`workflow_run`, `issues: write`): no event data in `run:`, no
checkout of the triggering run.

Judge, in this order:

a. INTENT — does the diff do what each task asks, all of it, and nothing the task did not ask
   for?
b. MATRIX — for each row: is the sensor real, does it exercise the claim, is the evidence class
   honest, would it fail if the claim were false? Check each new mutant actually isolates its
   clause. Dispute vacuous or over-classified rows.
c. RULES — anything the diff touches or violates.
d. AI-code failure modes — hallucinated flags or APIs (jq `-s`/`-e` semantics, yq expression
   semantics, GitHub job-level `concurrency` semantics), tautological tests, weakened assertions,
   a dropped half of a task, prose claiming more than the code does.

For each P0/P1 you are about to file, argue the opposite for one line; keep only what survives.
An APPROVE with no findings is valid when substantiated.

RETURN ≤15 lines, nothing else, under a heading `## Verdict`:

```text
verdict: APPROVE | REQUEST_CHANGES | BLOCK
findings:
  - [P0|P1|P2] <one line, with path:line>
matrix_disputes:
  - <row claim> — <why the sensor/class does not prove it>
could_not_check:
  - <what you could not verify>
summary: <one line>
```
