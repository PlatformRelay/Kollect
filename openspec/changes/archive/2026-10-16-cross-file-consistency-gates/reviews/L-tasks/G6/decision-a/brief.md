You are reviewing one follow-up change, independently of whoever implemented it. Read-only: do not modify files in the repository; for mutation checks copy the tracked tree to a scratch dir (mktemp -d; if /tmp is read-only for you, say so under could_not_check) and run guards there.

Repository: the current directory (kollect, branch feat/cross-file-consistency-gates).

1. TASK — openspec change `cross-file-consistency-gates`, task 6.1 follow-up for an operator decision (2026-10-16): move `bash hack/test/consistency_tool_pins_test.sh --self-test` out of the required `lint` job (.github/workflows/ci.yaml) into its own unconditional job `tool-pins-selftest` (runs on every PR, docs-only included, like lint), with its own `timeout-minutes`, added to `hack/release/verify-eligibility.sh` `required_checks`; `lint` keeps the tool-pins PLAIN step; keep `hack/test/task_check_test.sh` green (check.sh may need a coverage/exclusion entry for the new context); update every guard that pins job lists or required contexts WITHOUT loosening any — the CFC-6 rule in `hack/test/ci_workflow_security_test.sh` must require the self-test in that job. The protect-main ruleset is deliberately NOT touched (added after merge). Second item (G2 deferral): the tool-pins self-test's fixture module in the shared Go module cache gets a unique name per run so parallel runs cannot delete each other's fixture. Spec: `openspec/changes/cross-file-consistency-gates/specs/cross-file-consistency/spec.md` CFC-6 ("SHALL run on pull_request in a required job, plain and --self-test as separate steps").

2. MATRIX — `openspec/changes/cross-file-consistency-gates/evidence/G6.md`, section "Operator decision A (2026-10-16)" (rows A1–A11, sensors, iteration A-1).

3. DIFF — `git diff 366dc0840..b94b69066` (two commits: d770c56f3 the job split + guards + evidence; b94b69066 the fixture name).

4. SENSORS — recorded in that section: 24 CI-reading guard commands exit 0 (both modes where supported), `test-verify-eligibility.sh` 0, `task_check_test.sh` both modes 0, zizmor offline high no findings, `task lint:shell` 0, 4/4 gate mutants on the new clauses killed, tool-pins plain 49 s and `--self-test` 1002 s exit 0. Rerun anything under a minute that you dispute.

5. RULES — repo AGENTS.md: guards have a plain mode and `--self-test` (mutants on copies asserting non-zero with the targeted message + a no-op copy); never widen an allow-list, exclusion list or baseline to get green; SHA-pinned actions; zizmor clean at high; temp files cleaned in an EXIT trap; nothing private ships.

Judge four things, in this order:
a. INTENT — does the diff do what the decision asks, all of it, and nothing more? Is the new job unconditional (no if/needs/continue-on-error), required, bounded, with the tools the self-test needs (Go = go.mod's via ./.github/actions/go-cache, make, git, jq, zip, network)? Does lint keep the plain step and lose the self-test?
b. MATRIX — is each sensor real and would it fail if the claim were false? Check the new CWS CFC-6 clause (awk on required_rows column 3 = job name), the ci_docs_gate `check_tool_pins_job` and its mutants, the check.sh coverage mapping against task_check_test.sh's TCE-3 logic, the re-anchored TCE-3 mutant in task_check_test.sh (is its meaning unchanged?), and the fixture-name change (module path validity, cleanup scope with `${FIXTURE_NAME:?}`, the sentinel/probe still matching, nothing else in the guard still hard-coding `newgo`).
c. RULES — any rule above the diff touches or violates (a loosened guard, an unpinned action, a widened list).
d. AI-code failure modes — prose (comments, evidence) claiming more than the code does, dropped halves, a step that cannot fail the job.

For each P0/P1 you are about to file, argue the opposite for one line; keep only what survives.

An APPROVE with an empty findings list is valid when substantiated.

RETURN, as the last section of your output, a block headed `## Verdict` of at most 15 lines, nothing else after it:
## Verdict
verdict: APPROVE | REQUEST_CHANGES | BLOCK
findings:
  - [P0|P1|P2] <one line, with path:line>
matrix_disputes:
  - <row claim> — <why the sensor/class does not prove it>
could_not_check:
  - <what you could not verify>
summary: <one line>

Severity → verdict: P0 (security, data loss, invariant broken, vacuous gate presented as proof) → BLOCK. P1 (correctness gap, untested new behaviour, dishonest evidence class, missing half of the task) → REQUEST_CHANGES. P2/P3 → note only, APPROVE allowed.
