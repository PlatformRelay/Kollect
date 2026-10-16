You are reviewing one task group, independently of whoever implemented it. Read-only: do not modify files in the repository; for mutation checks copy files to a scratch dir under /tmp (mktemp -d) and run there, or use `bash hack/test/consistency_metrics_test.sh <copy-root>` on a copy.

Repository: the current directory (kollect, branch feat/cross-file-consistency-gates).

1. TASK — openspec change `cross-file-consistency-gates`, tasks 5.1 and 5.2 (tasks.md):
   - 5.1 Probe histogram suffix use and the literal scan (excluding `metrics_catalog.go`)
   - 5.2 Write `consistency_metrics_test.sh` with renamed-metric, undocumented, catalog-only, stale-mirror, suffix and empty-scan cases; watch them fail; implement; fix real drift in its own commit
   Spec: `openspec/changes/cross-file-consistency-gates/specs/cross-file-consistency/spec.md`, requirements CFC-4 (all scenarios) and CFC-6 (self-test with mutants + no-op copy; the CI-wiring half is task 6.1, out of scope here). Dispatch notes: registered names = `"kollect_[a-z0-9_]+"` string literals in non-test files of internal/metrics EXCLUDING metrics_catalog.go; strip `_bucket|_sum|_count` only where the base is a registered histogram; `registeredMetricNames` (internal/metrics/metrics_catalog_test.go:11) must equal the source set; every registered metric must appear in metrics.md; empty scan fails; real drift is fixed in docs/rules/mirror in its own commit, never by renaming a shipped metric.

2. MATRIX — `openspec/changes/cross-file-consistency-gates/evidence/G5.md` (verification matrix, probe, design decisions, sensors).

3. DIFF — `git diff 65a1dfffd..05842fe59` (two commits: 0dbc6302a docs drift fix; 05842fe59 guard + evidence + `evidence/G5-gate-mutants.sh`).

4. SENSORS — recorded in G5.md "Sensors run": plain run exit 1 before the docs fix (5 undocumented), exit 0 after; `--self-test` exit 0 (no-op + 18 mutants + 2 controls); `evidence/G5-gate-mutants.sh` 14/14 killed; `task lint:shell` 0; markdownlint 0; `go test ./internal/metrics/` ok. All run in under a minute; rerun any you dispute.

5. RULES — repo AGENTS.md: guards follow `hack/test/consistency_rbac_test.sh` shape (plain mode checks the real tree, `--self-test` corrupts a mktemp copy per mutant and asserts non-zero with the targeted message, plus a no-op copy that must pass); temp dirs cleaned in an EXIT trap; nothing private ships; never rename a shipped metric to fix drift.

Judge four things, in this order:
a. INTENT — does the diff do what the tasks ask, all of it, and nothing they did not ask for? (The guard also fails on a Namespace/Subsystem field and on Prometheus client imports outside internal/metrics; judge whether that is justified or scope creep.)
b. MATRIX — for each row: is the sensor real, is the evidence class honest, would the sensor fail if the claim were false? Mutation-verify every clause of the guard in a scratch copy: remove the logic it guards, confirm the self-test goes red, restore. Look especially at the Go masker (`mask_go`: comments, raw strings, rune literals, escapes), histogram detection (`HistogramOpts` block + Name resolution), the docs/rules name regex and suffix logic, mirror parsing, and whether any input makes the gate pass when a CFC-4 scenario is violated. Check the probe claims in G5.md against the code. Check the docs rows added in 0dbc6302a against `internal/metrics/metrics.go` (type, labels).
c. RULES — any rule above the diff touches or violates.
d. AI-code failure modes — tautological tests, weakened assertions, dropped halves, prose that claims more than the code does.

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
