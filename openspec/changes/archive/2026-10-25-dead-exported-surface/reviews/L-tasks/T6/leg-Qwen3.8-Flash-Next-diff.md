## Verdict: CONCERNS

Code change verified sound; the evidence record for its own key sensor is not.

## Findings
- [WARNING] The "unabridged" probe record in the evidence is truncated and miscounted — `openspec/changes/dead-exported-surface/evidence/T6.md:31` claims "exit 0, 16 hits, full output"; the block lists 15 lines, and re-running the same grep at `3db9bcb3` yields **17** hits. The record omits `kollectclusterinventory_dedupe_test.go:348-349` (keepAllPreservesCrossTargetRows) — sites the diff itself proves were present and migrated.
  Failure: matrix row 1 is a machine-class "pass" resting on a record that contradicts the measured output; a downstream reader of "probe-listed test sites" (tasks 6.1/6.2 wording) gets an incomplete basis. The migration itself is complete (I re-ran the post-deletion grep: zero hits repo-wide outside the change dir; both suites green on the final tree, collect 9.1s, controller 56.0s).
  Fix: replace the probe block with the real 17-line output and correct the hit count in row 1 and the header line 31.
  Confidence: 95
- [NOTE] Wrong line cite for the inert-seed argument — `evidence/T6.md:19,71` cite `kollecttarget_controller.go:80` as the KollectTarget delete path; :80 is the suspend branch (:179 is degradeScopeDenied). The path the test reaches is `target_finalizer.go:51` (finalizeTargetDeletion → UnregisterTarget), and `engine.go:394` removes the store row unconditionally, so the deleted `target_finalizer_test.go` seed is genuinely inert — conclusion holds, cite is wrong.
  Fix: cite `target_finalizer.go:51`.
  Confidence: 90
- [NOTE] The helper's `t.Cleanup(cancel)` stops nothing — `kollectclusterinventory_helpers_test.go:41-42`: `startInformer` derives the informer context from `informerContext()` (`engine.go:582-589` → `context.Background()` since `Start` is never called), and the real cancel lands in `e.informerCancels` which no `Engine` method ever invokes (no Stop/Close exists). One fake-client reflector goroutine per helper call lives to process exit. Harmless in a short-lived test binary (empty fake client, no events, no dispatch workers), but the cleanup reads as lifecycle management it isn't.
  Fix: drop the WithCancel or note "engine has no stop path" in the doc comment.
  Confidence: 80
- [NOTE] The sort-order pin for `NamespacesForClusterTarget` (`engine.go:419`) is gone with no replacement anywhere; honestly recorded at `evidence/T6.md:118`, and I verified no order-dependent consumer (`composeNamespaceRollup` re-sorts itself). Smallest ratchet if wanted: a 3-line sorted-assert inside the existing `composeNamespaceRollup` unit test's engine fixture.
  Confidence: 85

Checked: full diff vs `syncEngineTargets` shape (`kollectclustertarget_controller.go:193-247` matches the helper, incl. `LabelMetadataName` selector); RegisterTarget new-key path does not touch the store, so the pre-registered `store.Upsert` seeding in both finalizer tests survives registration and is still removed by the delete path; `err :=` fix in cluster_target_finalizer_test.go compiles, no shadow; map-iteration order in the helper is sorted; no import cycle added (controller→collect pre-existing); build/vet/tests re-run green on the final tree; zero surviving references, Go and non-Go.

## Could not check
- Envtest suites (`kollectclustertarget_controller_test.go`, `kollectclusterinventory_controller_test.go`) — row 11/12 reach-ability claims: no binaries run; only verified those files contain no reference to the deleted symbol.
- `task arch-lint` / full `task check` — relied on the evidence's recorded run (27.6s, OK).
- Whether 17-vs-15 miscount originated in the probe tool run or the transcription into T6.md (no raw log left).
