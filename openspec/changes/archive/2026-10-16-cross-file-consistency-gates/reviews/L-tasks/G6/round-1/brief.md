You are reviewing one task group, independently of whoever implemented it. Read-only: do not modify files in the repository; for mutation checks copy the tracked tree to a scratch dir under /tmp (mktemp -d) and run guards there with `bash <copy>/hack/test/<guard> <copy-root>`.

Repository: the current directory (kollect, branch feat/cross-file-consistency-gates).

1. TASK — openspec change `cross-file-consistency-gates`, task 6.1 (tasks.md): "Run all guards in `lint` (plain and `--self-test` as separate steps) and, by the glob, in `task check`". The guards: hack/test/consistency_go_toolchain_test.sh, consistency_tool_pins_test.sh, consistency_rbac_test.sh, consistency_metrics_test.sh, go_patch_lag_test.sh. Spec: `openspec/changes/cross-file-consistency-gates/specs/cross-file-consistency/spec.md` CFC-6 ("SHALL run on pull_request in a required job, plain and --self-test as separate steps"). Dispatch notes: `lint` is a REQUIRED check and runs unconditionally on every PR, docs-only included, and must stay so; each guard's tools (Go exactly go.mod's, helm, yq, PyYAML, network) must be present in the job, pinned the way the job already pins things; the tool-pins self-test (~7 min) gets its own step; deferred findings to fix or record: (G2) the tool-pins self-test deletes a fixture from the shared Go module cache, parallel-unsafe — make sure no other step races it in CI or give it a private GOMODCACHE; (G4) keep the go-patch-lag curl retry budget inside the step's timeout (`timeout-minutes` on the step). Never loosen an existing guard to make the wiring pass. Task 6.2 is owner/post-merge and out of scope.

2. MATRIX — `openspec/changes/cross-file-consistency-gates/evidence/G6.md` (verification matrix, tool table, deferred findings, runtimes, proposal, gaps).

3. DIFF — `git diff c47170e5b..300801927` (one commit): `.github/workflows/ci.yaml` (lint job), `.github/workflows/go-patch-lag.yaml`, `hack/ci/go-patch-lag.sh` (comment), `hack/test/go_patch_lag_test.sh` (new budget assertion + 5 mutants), evidence/G6.md.

4. SENSORS — recorded in G6.md "Sensors run" and "Runtimes": all five guards in both modes exit 0 (tool-pins self-test 416 s, on the base tree; it is re-run on the final tree in FINAL); the CI-reading guards (ci_workflow_security both modes, ci_docs_gate, dist_ci_wiring, repo_root_links_wiring, sonar_ko_ci_wiring, docs_map_wiring, ci_fetch_lib_hardening, ci_install_helm_hardening, dev_mise_pin_drift, docs_coverage_floor_drift, ui_removal_reference, task_check both modes) exit 0 on the diff; zizmor offline at high: no findings; `task lint:shell` 0. Everything except the tool-pins self-test runs in under ~1 minute; rerun what you dispute.

5. RULES — repo AGENTS.md: guards have a plain mode (real tree) and `--self-test` (mutants on mktemp copies asserting non-zero with the targeted message + a no-op copy that passes); never widen an allow-list or exclusion list to get green; SHA-pinned actions; zizmor clean at high; nothing private ships.

Judge four things, in this order:
a. INTENT — does the diff do what the task asks, all of it, and nothing it did not ask for? Is every guard wired in both modes as separate steps of the `lint` job, and does `lint` stay unconditional? Does `task check` (hack/check.sh, glob sweep) pick up all five in both modes?
b. MATRIX — for each row: is the sensor real, is the evidence class honest, would the sensor fail if the claim were false? Check the tool table against the guards' prerequisite lines and the lint job's install steps (order matters: a tool must be installed before the step that needs it). Check the G2 reasoning (sequential steps, no backgrounded command) and the G4 fix: mutation-verify the new go_patch_lag_test.sh assertion (step timeout, job timeout, `--max-time`/`--retry-max-time` parsing from code lines, the arithmetic) in a scratch copy. Check the runtime/wall-time claims are labelled believed vs verified honestly.
c. RULES — any rule above the diff touches or violates (look for a weakened guard, an unpinned action, a new zizmor finding).
d. AI-code failure modes — prose that claims more than the code does (ci.yaml comments included), dropped halves, a step whose command could not fail the job.

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
