You are reviewing one task group of the OpenSpec change `synced-debounce-condition`,
independently of whoever implemented it. Read-only: do not modify any file. Work from the
repository root (branch `feat/synced-debounce-condition`).

Inputs:

1. TASK — `openspec/changes/synced-debounce-condition/tasks.md`, items 3.1 (tests, red on the G2
   commit `e90da31ac`) and 3.2 (implementation), group G3. They cite SDC-4 in
   `openspec/changes/synced-debounce-condition/specs/inventory-sync-status/spec.md` and
   `design.md` *When a sink counts as up to date* in the same directory. Docs are G4, not G3.
2. MATRIX — `openspec/changes/synced-debounce-condition/evidence/G3.md` (verification matrix,
   recorded reds, mutation results, test changes).
3. DIFF — the staged diff `git diff --cached e90da31ac` (nothing committed yet). Production:
   `internal/controller/per_sink_export.go`, `kollectinventory_controller.go`,
   `kollectclusterinventory_controller.go`, `internal/sink/resolver.go`. Tests: new
   `per_sink_export_identity_test.go`, new `kollectinventory_debounce_identity_envtest_test.go`,
   `kollectclusterinventory_export_envtest_test.go`, `kollectinventory_debounce_test.go`, and
   mechanical identity-argument edits in five other test files. Plus evidence and tasks.md.
4. SENSORS — the *Sensors run* table in the evidence (exit codes as reported). The full package
   suite and the race run are recorded as pending: they run once after review. The unit tests
   run in under a second
   (`go test -count=1 -run 'TestPerSinkCoalesceTracker|TestFailSinkExport|previewAllSinksDebounced' ./internal/controller/`);
   rerun them if the sandbox lets you. Envtest needs a kube-apiserver binary; trust its reported
   results.
5. RULES — tests written first and red on the target assertion (not a compile error) against the
   stub design.md implies (identity argument ignored; `forget` a no-op; `failSinkExport` without
   the reset; preview fall-back kept); changed existing tests only as *Test changes* rows citing
   the spec (the mechanical identity-argument edits, one row, and
   `TestKollectInventoryReconciler_previewAllSinksDebounced_allDebounced`, whose fixture gains the
   two sink objects with its assertion unchanged); any other existing test turning red is a
   finding; no CRD schema, RBAC, metric or dependency change; no private company strings,
   secrets, or model/tool names in committed files.

Judge, in this order:

a. INTENT — does the diff do all of 3.1 and 3.2 and nothing else? Check each 3.1 bullet against a
   test, including "both kinds" where tasks.md says so (returning payload, recreated inventory)
   and the two controls. Check each 3.2 clause: `ResolvedSink.Generation` in all three
   resolvers; identity in `sinkCoalesceState`, `shouldSkip` and `record` at every call site
   (both reconcilers and the preview); `failSinkExport` with `forget` on EVERY per-sink failure
   branch of both reconcilers (resolve, partition, export job); the preview's resolve-error
   branch no longer debounces.
b. MATRIX — is each sensor real, does it exercise the claim, would it fail if the claim were
   false, is the class honest? Are the recorded reds on the target assertion? Are the controls
   honestly labelled? Is anything SDC-4 requires left untested?
c. RULES — as above. Are the mechanical edits really mechanical (same zero identity on both sides,
   no assertion changed)?
d. AI-code failure modes — tautological tests, weakened assertions, a dropped half of the task,
   prose that claims more than the code does. Check in particular: can the identity check cause
   a re-export on every pass in some steady state (an identity field that differs between two
   passes over the same objects, e.g. a value read from a cache, a nil resolved sink, or the
   preview and the full path computing different identities for the same binding)? Does
   `forget` from the namespaced export goroutines race with the tracker or deadlock with the
   outcome mutex? Does the preview's early return change anything for snapshot sinks or for the
   requeue interval? Does the failure reason or message of the resolve and partition branches
   change compared to before?

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
