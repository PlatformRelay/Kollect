You are reviewing one task group of the OpenSpec change `synced-debounce-condition`,
independently of whoever implemented it. Read-only: do not modify any file. Work from the
repository root (branch `feat/synced-debounce-condition`).

Inputs:

1. TASK — `openspec/changes/synced-debounce-condition/tasks.md`, items 1.1 (tests, red on
   `origin/main`) and 1.2 (implementation), group G1. They cite SDC-1, SDC-2, SDC-3 and SDC-6 in
   `openspec/changes/synced-debounce-condition/specs/inventory-sync-status/spec.md`, and
   `design.md` *Per-sink condition*, *Aggregate* and *Read API* in the same directory. The
   total-failure per-sink write (SDC-3 "also when every sink failed"), carry-over and churn
   (SDC-5) and the debounce identity (SDC-4) are later groups (G2, G3), not G1; docs are G4.
2. MATRIX — `openspec/changes/synced-debounce-condition/evidence/G1.md` (verification matrix,
   recorded reds, mutation results, test changes).
3. DIFF — the staged diff `git diff --cached 6944e2dcf` (nothing committed yet; base `6944e2dcf`
   has the same code as `origin/main`). Production: `internal/controller/per_sink_export.go`,
   `kollectinventory_controller.go`, `kollectclusterinventory_controller.go`,
   `internal/inventory/status.go`, `api/v1alpha1/export_interval_types.go`. Tests:
   `per_sink_export_test.go`, `requested_at_annotation_test.go`,
   `kollectclusterinventory_export_envtest_test.go`, new
   `kollectinventory_synced_debounce_envtest_test.go`, `internal/inventory/cov_s06_test.go`.
4. SENSORS — the *Sensors run* table in the evidence (exit codes as reported). The full package
   suite and the race run are recorded as pending: they run once after review. Unit tests run in
   about a second (`go test -count=1 -run 'TestAggregateInventorySync|TestSetSinkExportDebounced' ./internal/controller/`,
   `go test -count=1 -run TestExportStatusFromInventory ./internal/inventory/`); rerun any you dispute
   if the sandbox lets you. Envtest needs a kube-apiserver binary; trust its reported results.
5. RULES — tests written first and red on the target assertion (not a compile error) against the
   named stub; changed existing tests only as *Test changes* rows citing the spec; any other
   existing test turning red is a finding; no CRD schema, RBAC, metric or dependency change; no
   private company strings, secrets, or model/tool names in committed files.

Judge, in this order:

a. INTENT — does the diff do all of 1.1 and 1.2 and nothing else? Check each 1.1 bullet against a
   test, and each 1.2 clause (helper in the three debounced branches; aggregate per design
   table; cluster `ExportSucceeded` message `exported %d item(s); %d/%d sink(s) up to date`;
   `<family>/<name>` lookup, reason-first mapping and per-entry time in the read API; the
   `ReasonPartiallySynced` comment).
b. MATRIX — is each sensor real, does it exercise the claim, would it fail if the claim were
   false, is the class honest? Are the recorded reds on the target assertion? Is anything the
   spec requires for SDC-1/2/3/6 in G1's scope left untested?
c. RULES — as above.
d. AI-code failure modes — tautological tests, weakened assertions, a dropped half of the task,
   prose that claims more than the code does. Check in particular that the read API still uses
   the aggregate for a binding with no entry, and that the message helper's "N/N" is correct for
   every no-failure pass (can a binding be neither exported, debounced nor failed in a pass?).

For each P0/P1 you are about to file, argue the opposite for one line; keep only what survives.
An APPROVE with no findings is valid when substantiated.

Write your result, and nothing else, in this shape (at most 15 lines):

```text
## Verdict
verdict: APPROVE | REQUEST_CHANGES | BLOCK
findings:
  - [P0|P1|P2] <one line, with path:line>
matrix_disputes:
  - <row claim> — <why the sensor/class does not prove it>
could_not_check:
  - <what you could not verify>
summary: <one line>
```

Severity: P0 (invariant broken, vacuous gate presented as proof) → BLOCK; P1 (correctness gap,
untested new behaviour, dishonest evidence class, missing half of the task) → REQUEST_CHANGES;
P2 → note only.
