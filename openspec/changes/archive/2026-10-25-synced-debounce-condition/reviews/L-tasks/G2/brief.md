You are reviewing one task group of the OpenSpec change `synced-debounce-condition`,
independently of whoever implemented it. Read-only: do not modify any file. Work from the
repository root (branch `feat/synced-debounce-condition`).

Inputs:

1. TASK — `openspec/changes/synced-debounce-condition/tasks.md`, items 2.1 (tests, red on the G1
   commit `fddd8e761`) and 2.2 (implementation), group G2. They cite SDC-5 and SDC-3 in
   `openspec/changes/synced-debounce-condition/specs/inventory-sync-status/spec.md`, and
   `design.md` *Per-sink entry carry-over*, *Aggregate* (the total-failure branches) and
   *Steady state without writes* in the same directory. The debounce identity, `forget` and
   `failSinkExport` (SDC-4) are G3, not G2; docs are G4.
2. MATRIX — `openspec/changes/synced-debounce-condition/evidence/G2.md` (verification matrix,
   recorded reds, mutation results, test changes).
3. DIFF — the staged diff `git diff --cached fddd8e761` (nothing committed yet). Production:
   `internal/controller/per_sink_export.go`, `kollectinventory_controller.go`,
   `kollectclusterinventory_controller.go`. Tests: `per_sink_export_test.go`,
   `kollectclusterinventory_export_envtest_test.go`, new
   `kollectinventory_sink_export_record_envtest_test.go`. Plus the evidence and `loop.md`.
4. SENSORS — the *Sensors run* table in the evidence (exit codes as reported). The full package
   suite and the race run are recorded as pending: they run once after review. The unit test
   runs in under a second (`go test -count=1 -run 'TestCarryOverSinkExport' ./internal/controller/`);
   rerun it if the sandbox lets you. Envtest needs a kube-apiserver binary; trust its reported
   results.
5. RULES — tests written first and red on the target assertion (not a compile error) against the
   named stub (`carryOverSinkExport` copying paths only, as before); changed existing tests only
   as *Test changes* rows citing the spec (`TestCarryOverLastExportPaths` is the one named);
   any other existing test turning red is a finding; no CRD schema, RBAC, metric or dependency
   change; no private company strings, secrets, or model/tool names in committed files.

Judge, in this order:

a. INTENT — does the diff do all of 2.1 and 2.2 and nothing else? Check each 2.1 bullet against a
   test, including "both kinds" (namespaced and cluster) where tasks.md says so and the two
   namespaced variants of "writes no status" (database sink on the preview path, snapshot sink on
   the full path). Check each 2.2 clause: the helper at every former `carryOverLastExportPaths`
   call site (both reconcilers and the preview path), deep copy of conditions, and
   `outcome.SinkExports` stored in BOTH total-failure branches before the degraded write.
b. MATRIX — is each sensor real, does it exercise the claim, would it fail if the claim were
   false, is the class honest? Are the recorded reds on the target assertion? Are the controls
   (aggregate parts of the last two specs) honestly labelled? Is anything SDC-5 or the SDC-3
   transitions require in G2's scope left untested?
c. RULES — as above.
d. AI-code failure modes — tautological tests, weakened assertions, a dropped half of the task,
   prose that claims more than the code does. Check in particular: can seeding the conditions
   carry a stale `Synced` condition into an entry whose pass never sets one (every binding in a
   pass is exported, debounced or failed — is that true on every path that stores
   `SinkExports`, including the preview path)? Does storing `SinkExports` on a total failure
   change what the deletion cleanup (`exportEvidenceBySink`, `inventory_finalizer.go`) sees in a
   way the spec does not intend? Is there a data race between the seeding and the export
   goroutines?

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
  - <what you had no access, time, or evidence to verify>
summary: <one line>
```
