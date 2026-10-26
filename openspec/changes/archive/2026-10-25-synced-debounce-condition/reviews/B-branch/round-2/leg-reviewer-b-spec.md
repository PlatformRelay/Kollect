## Verdict: CLEAN

## Findings
- [NOTE] SDC-1 holds for both kinds: a debounced sink gets `True`/`Debounced`, and the message names the interval with no countdown. — `internal/controller/per_sink_export.go:346-357`, `kollectinventory_controller.go:370,504`, `kollectclusterinventory_controller.go:318`
  All four scenarios hold. The zero-interval scenario holds because the message reads "exports only on change (interval 0s)" (`per_sink_export.go:353`).
  Confidence: 90
- [NOTE] SDC-2 holds: when no sink failed the aggregate is `True`/`Exported` with "N/N sink(s) up to date". Cluster `ExportSucceeded` uses the same count. — `per_sink_export.go:454-467`, `kollectclusterinventory_controller.go:759-760`
  The denominator is exported+debounced, not the bound-sink count. Round-1 B4 rejected this, and the rejection holds: every binding ends in fail, debounce or export on every branch (`kollectinventory_controller.go:328-381,439-453`; cluster `:277-372`; preview `:515`).
  Confidence: 85
- [NOTE] SDC-3 holds. A debounced+failed pass gives `Progressing`, which is the reason before this change. Total failure now persists the per-sink entries. The carve-out holds because the pre-export gates call `setInventoryDegraded`/`setDegraded` without touching `SinkExports`. — `per_sink_export.go:447-453`, `kollectinventory_controller.go:265,150-158,217-220`, `kollectclusterinventory_controller.go:227,190-206`
  `Ready` and `Degraded` branches are unchanged (`kollectinventory_controller.go:563-609`). Aggregate `lastTransitionTime` only moves on a status change, via `SetStatusCondition` (`sink_status.go:179`).
  Confidence: 85
- [NOTE] SDC-4 holds. The inputs are checked in this order: generation, identity (inventory UID, sink UID, sink generation), checksum, requestedAt, interval. — `per_sink_export.go:100-129`
  - Every failure branch goes through `failSinkExport`, which calls `forget` (`:259-270`).
  - The preview path refuses a sink it cannot resolve (`kollectinventory_controller.go:490-494`).
  - The cluster path gets `Generation` through the same resolver (`internal/sink/resolver.go:72,88,104`).
  Confidence: 90
- [NOTE] SDC-5 holds, including the new status-write sentence. — `kollectinventory_controller.go:612-613`, `kollectclusterinventory_controller.go:834-835`, `per_sink_export.go:229-243`
  - `forget` runs before the conflict/error split, so it covers both cases. `markExported` is called only after `record`, under `mu` on the parallel path (`kollectinventory_controller.go:446-447`).
  - The only write paths after an export are `updateStatus` or the total-failure branch, and in the total-failure branch `ExportedCount==0`, so no baseline is left to drop.
  - Carry-over keeps T1/C1 and the condition's `lastTransitionTime` (`per_sink_export.go:383-404`).
  Confidence: 85
- [NOTE] The SDC-5 test does not go through `Reconcile`, so the namespaced preview path after a lost write is untested. — `internal/controller/per_sink_export_persist_test.go:104-115`
  Failure: none today. Preview calls `shouldSkip`, which returns false when the state is nil (`per_sink_export.go:100`), so the full path re-exports. A future cache in the preview path could still debounce, and no test would catch it.
  Fix: optionally add an envtest case that drives two `Reconcile` calls with an injected status-update conflict.
  Confidence: 70
- [NOTE] Dropping the baseline after a failed status write means a full re-send to every sink exported in that pass. On a frequently conflicting object with an event-family sink (Kafka, NATS), each lost write re-emits the whole payload. — `kollectinventory_controller.go:613`
  Failure: an inventory under sustained status conflicts emits the same payload to its event stream every time a write is lost. The spec requires this, and the upgrade note (`upgrading.md:425-459`) does not mention it.
  Fix: add one line to the upgrade note, or accept as designed.
  Confidence: 55
- [NOTE] On an all-debounced pass, cluster `ExportSucceeded` says "exported N item(s); N/N sink(s) up to date" although nothing was exported in that pass. — `kollectclusterinventory_controller.go:759`
  The spec allows this; it only forbids "to 0 sink(s)". The namespaced `Ready` message "exported %d item(s) across" (`kollectinventory_controller.go:574`) has the same wording, and that one predates this branch.
  Confidence: 60
- [NOTE] SDC-6 holds. Entries are keyed by `<family>/<name>` (the same key as `sinkExportKey`, `inventory_sink_helpers.go:81-82`), the reason is mapped before the status, and a matched entry with no time reports an empty time. — `internal/inventory/status.go:161-175,193-206`
  Confidence: 90
- [NOTE] SDC-7 holds. A search of `docs/` for debounce paired with `Synced=False`/`PartiallySynced` finds only `upgrading.md:429` and `adr/0413-export-interval-scheduling.md:80`, which is the amended line and is covered by the note at `:83-91`. — `docs/operator-manual/upgrading.md:408`
  The section heading matches the spec's name. FR-EXP-9 is reworded (`docs/REQUIREMENTS.md:89`).
  Confidence: 85
- Round-1 dispositions:
  - All hold: A1 (fix verified above), A2 (the `b74f2c678` footer names the read API and the per-sink message), B1 (spec lines 77-79 plus code paths above), B2, B3.
  - Rejections B4 and B6 still stand: no write path produces an entry without a `Synced` condition.
  - B5 is deferred: `status.go:161` still rebuilds the key by hand.
- Code behaviour that no requirement mentions:
  - The preview path stops at the first sink it cannot resolve, even after debouncing earlier sinks.
  - A new `ResolvedSink.Generation` field.
  - The comment on `ReasonPartiallySynced` changed (`api/v1alpha1/export_interval_types.go:16`).
  - A debounced+failed pass uses `Progressing` even when the failure is terminal or a spill.
  - `forget` is also triggered by a resolve error.
  - `exportedSinks` bookkeeping on the outcome.

## Could not check
- I ran no tests, envtest or `task check`. I relied on the round-1 sensor records in `reviews/B-branch/round-1/verification.md` and did not re-verify them.
- I did not read in full the envtest files (`*_debounce_identity_envtest_test.go`, `*_sink_export_record_envtest_test.go`, `*_synced_debounce_envtest_test.go`, the cluster export envtest), so I have not checked their scenario coverage line by line.
- I did not trace whether the `noteExportShardWarning` path (excluded by SDC-5) or the namespace item count differs between preview and full passes in a way that changes `Ready`.
