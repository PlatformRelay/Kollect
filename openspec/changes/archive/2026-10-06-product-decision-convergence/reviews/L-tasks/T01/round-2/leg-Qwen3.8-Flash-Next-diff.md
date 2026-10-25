## Verdict: CONCERNS

## Findings
- [WARNING] The round-2 fix `assertSyncedAsForAnyExport` is unreachable dead code today and unverified for its green path — `internal/controller/requested_at_annotation_test.go:242`
  Failure: the helper is only called *after* a `t.Fatalf` on the forced-export assertion (lines 222/282 — reproduced by running the tests), so its `updateStatus` call never executes at HEAD; when T05 makes it run, the sync assertion can fail for harness reasons (e.g. `Status().Update` on a fake client with no `WithStatusSubresource(inv)`, unlike every existing updateStatus test — `kcollectinventory_update_status_test.go:35`) and T05 inherits a correction T01 could have de-risked.
  Fix: add one always-green test now that drives `updateStatus` with a plain (non-forced) `perSinkExportOutcome` through the same harness and asserts the identical Synced/message/RequeueAfter expectations, proving the fixture and the expected literals.
  Confidence: 90
- [WARNING] Cluster-path parity is claimed but not locked — the cluster tests never assert Synced/RequeueAfter after a forced export, and the cluster `updateStatus` has a different signature (5-arg, `kollectclusterinventory_controller.go:153`) so no reusable helper exists; T05 can mislabel the cluster condition and pass T01. — `internal/controller/requested_at_annotation_test.go:332-368`
  Failure: a T05 that writes the forced export's cluster status with a special reason/zero requeue keeps both cluster tests green, contradicting the row-5 parity claim.
  Fix: mirror the namespaced helper for `KollectClusterInventoryReconciler` (the cluster harness already has `WithStatusSubresource`, so it is runnable) and call it after the third/fourth cluster exports.
  Confidence: 85
- [NOTE] Matrix header is stale after round-2 edits — `openspec/changes/2026-10-06-product-decision-convergence/evidence/T01.md:12`
  Failure: "rows 1–10 must be RED at SENSE; rows 11–12 are green" contradicts the table itself (row 10 is a pass, only 11 rows exist). The round-1 F2 fix also left row 3's phrasing (+helper) as the only marker of the new second-round sensor; row 9 correctly added. Reader miscount will miscall a green as a missed red.
  Fix: reword to "rows 1–9 red; rows 10–11 green guards".
  Confidence: 100
- [NOTE] `Prune.ScrubKeys` set in the scrub test has no effect on `PruneResource` — `internal/collect/prune_collected_generation_test.go:109` vs `internal/collect/prune.go:40-76` (only the passed `*Scrubber` scrubs; `PruneSpec.ScrubKeys` is consumed solely by the engine at `engine.go:266`). Harmless (engine covers both channels) but it documents a false lever for T06.
  Fix: drop the field or comment that it is exercised only via `newResourceExportEngine`.
  Confidence: 90
- [NOTE] Cluster `presenceTransitions` folds two named scenarios into one test (row 5 cites `*_presenceTransitions` alongside the sibling's separately-run transitions), so a red in the second transition aborts before the first is recorded separately.
  Fix: split into absence→present and present→absence tests to match the namespaced-path granularity the matrix implies.
  Confidence: 60

What I checked: ran all 6 controller reds and all collect tests — each fails at exactly the line and message recorded in the evidence (183/222/282/321/360/397; 67/90/124), guards `noStampWithoutMetadata` and `attributesModeUntouchedByStamp` pass; no production symbol referenced (test-local keys; `updateStatus`/`perSinkExportOutcome`/`recordingBackend`/`clusterRollupScheme`/`newResourceExportEngine` all exist at base); round-1 F1 fix verified concrete against `scrub.go:121-135` suffix/denylist match and `redactedValue()` map shape (the map-type branch catches a pre-scrub stamp); F3 fix verified (whole-blob JSON scan); expected `True/Exported/"exported to 1 sink(s)"` matches production `aggregateInventorySync` default branch, and 5m matches `exportDebounce(inv)` with the harness's `ExportMinInterval`.

## Could not check
- Green-phase behaviour of `assertSyncedAsForAnyExport` against the namespaced fake client (blocked by the red, as above) — controller-runtime v0.24.1 fake `Status().Update` without `WithStatusSubresource` not executed here.
- `-race` and `-count=2` runs, `task lint`, `task verify` — trusted the evidence table; not re-run.
- Round-1 register `reviews/L-tasks/T01/register.md` content and T05/T06 task texts — cited verbatim in the evidence file but not opened independently.
