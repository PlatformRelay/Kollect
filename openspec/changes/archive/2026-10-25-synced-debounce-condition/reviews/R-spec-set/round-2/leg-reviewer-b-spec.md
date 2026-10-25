## Verdict: CONCERNS

## Findings
- [WARNING] The SDC-7 doc check in task 4.1 passes on docs that still say a debounce means `Synced=False`/`PartiallySynced`. The check is `rg -U 'debounc[^|]{0,200}(Synced=False|PartiallySynced)|…'`, and `[^|]` cannot cross a table cell. Most of the stale wording pairs the two across cells. — `docs/crds/kollectinventory.md:148`, `docs/crds/kollectclusterinventory.md:132`, `docs/operator-manual/troubleshooting.md:32`, `:222`
  Failure: an implementer edits only `troubleshooting.md:223` and ADR-0413 `:80`. The check then reports "only the two notes", the Verification row turns green, and the four rows above still say a debounce sets `Synced=False`/`PartiallySynced`.
  Fix: make the check per line (`[^\n]` instead of `[^|]`), and keep the multiline variant only for prose; or list these four rows as named edits in 4.1.
  Confidence: 85
- [WARNING] SDC-6 is only partly met by the design: the read API still falls back to the inventory-level `lastExportTime` when a sink's entry exists but has no `lastExportTime`. — `internal/inventory/status.go:147-166`
  Failure: a sink that never exported successfully (or, between the G1 and G2 commits, a debounced entry) is reported with another sink's export time, which contradicts "the entry's `lastExportTime` as the sink's last export time". The design changes only the lookup key and the mapping order, and no planned row covers a matched entry with a nil time.
  Fix: when an entry matches and its time is nil, report an empty time. Add that row to `TestExportStatusFromInventory_…`.
  Confidence: 70
- [NOTE] G1 temporarily drops the failed sink's `lastExportTime`/`lastChecksum` on total failure. G1 stores `outcome.SinkExports` on that path, but those entries are built from empty with paths-only carry-over until G2 lands. On base, the stale entry kept T1/C1. The design's phrase "built with the carry-over below" is only true from G2 on. — `internal/controller/kollectinventory_controller.go:300,323`, `kollectclusterinventory_controller.go:271-272`
  Fix: say in tasks.md that G1 regresses T1/C1 on total failure until G2, or have G1 seed `LastExportTime`/`LastChecksum`.
  Confidence: 75
- [NOTE] Existing tests that will change are missing from the proposal's *Test changes* list. `TestCarryOverLastExportPaths` (its "fresh paths kept" case conflicts with seeding the entry before the export decision unless seeding is conditional), plus 43 direct `shouldSkip`/`record`/`nextDue`/`carryOverLastExportPaths` calls in 6 test files that break when the G2/G3 signatures change. — `internal/controller/per_sink_export_test.go:386-416`
  Fix: add a *Test changes* row for the carry-over test, and say the signature-only edits are mechanical.
  Confidence: 60
- [NOTE] The namespaced "writes no status on a repeated debounced reconcile" envtest only goes through the preview path, because Postgres sinks use the raw fingerprint. The reported case (a Git snapshot sink) always falls through to the full path, since the preview compares the raw fingerprint and snapshot sinks use `PartitionsChecksum`. So no-churn on the namespaced full path is not tested. — `kollectinventory_controller.go:360-363` vs `:498`
  Fix: add one snapshot-family (or mixed) row to that envtest.
  Confidence: 65
- [NOTE] "Known to be up to date" (SDC-4) claims more than the code can know. A Secret rotation that repoints the sink, or drift on the backend side, bumps neither the sink UID nor `metadata.generation`, so the sink stays debounced. — `internal/sink/resolver.go:63,78,93`
  Fix: narrow the wording in SDC-4 to "unchanged sink object", or name Secret changes in the non-goals.
  Confidence: 50

Checked and holding:
- **SDC-1–4 reds:** each planned red is real on its stated base.
  - G1: aggregate (0,2,0)/(1,1,0) gives `PartiallySynced` (`per_sink_export.go:350-355`); per-sink `False/Debounced` (`:363-371`, cluster `:312-320`); the total-failure path skips `SinkExports` (`kollectinventory_controller.go:251-270`, cluster `:211-231`); the bare-name lookup (`status.go:160`).
  - G2: entries rebuilt from empty and a fresh `lastTransitionTime` each pass (`:300`, `setSinkExportSynced` `per_sink_export.go:216`).
  - G3: no identity check and no reset (`per_sink_export.go:59-122`); preview falls back to the default interval (`:490`).
  - Sinks carry no finalizer in envtest, so the "recreated sink" scenario can run.
- **No-churn:** every other field written on a debounced pass is computed from stable inputs: Ready/ExportSucceeded messages, `SinkReachable`, `itemCount`, `namespaceShards`. No Event fires when `ExportedCount==0`.
- **Citations:** current. Small offsets only: `status.go:159-182` is the loop, not the whole function (125-195), and `carryOverLastExportPaths` starts at `:279` with its doc comment at `:272`.
- **Readers:** no reader of `Synced`/`PartiallySynced` outside the Impact list (`hack/`, `charts/`, `test/`, `cmd/`).

## Could not check
- Envtest behaviour: whether a no-op `Status().Update` keeps `resourceVersion`, and whether managedFields timestamps change on it (that depends on the envtest apiserver version, which probe 0.1 is meant to pin). The `apimeta.SetStatusCondition` source in the pinned module.
- `loop.md` and `reviews/` were out of scope, so prior rounds were not read. The decision brief `agent-context/decisions/2026-10-20-synced-debounce-reason.md` was not read.
- Read-only mode, so no tests or `rg` runs were executed; nothing was observed live.
