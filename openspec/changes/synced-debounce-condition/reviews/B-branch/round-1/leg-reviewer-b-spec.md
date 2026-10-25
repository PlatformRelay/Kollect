## Verdict: CONCERNS

The code meets SDC-1 to SDC-6, checked against the code itself rather than the evidence files. The concerns are a carve-out the proposal makes but the spec leaves out, and documentation wording that SDC-7 states literally. Nothing blocks.

**SDC-1 holds.** `per_sink_export.go:325-336`: every debounce site sets `True`/`Debounced` with a message that names the interval and has no countdown (`kollectinventory_controller.go:370,503`, cluster `:318`). The `0s` message is at `:331-332`. Scenarios: second reconcile, not `False`, stable message and zero interval all hold. The zero-interval case is tested only as a unit (`per_sink_export_test.go:137`).
**SDC-2 holds.** `per_sink_export.go:433-446`: the aggregate reads `N/N sink(s) up to date` on both export and debounce passes. Cluster `ExportSucceeded` uses the same count (`kollectclusterinventory_controller.go:758-759`), so it no longer says "to 0 sink(s)".
**SDC-3 holds for export passes.** `PartiallySynced` only when exported>0 and failed>0 (`per_sink_export.go:427`). Debounced plus failed gives `False`/`Progressing` (`:430`). On the total-failure path the per-sink entries are now stored (`kollectinventory_controller.go:265`, cluster `:227`). `Ready`/`Degraded` switches are unchanged (`:562-608`). The `True`→`False`→`True` `lastTransitionTime` behaviour is tested (`…sink_export_record_envtest_test.go:299`).
**SDC-4 holds.** The identity check is at `per_sink_export.go:105`. The generation is filled in `sink/resolver.go:72,88,104`, and every `ResolvedSink` is built there. `forget` runs on every failure path through `failSinkExport` (`:238-249`), including resolve and partition failures. The preview returns "not all debounced" for an unresolvable sink (`kollectinventory_controller.go:490-493`). All five scenarios have envtests (`…debounce_identity_envtest_test.go:43-149`).
**SDC-5 holds.** `carryOverSinkExport` deep-copies the record and conditions (`per_sink_export.go:376-380`). `SetStatusCondition` keeps `lastTransitionTime`. No-write and no-Event behaviour is tested on the preview path, the full path and the cluster path (`:171,:189`, cluster `:481`).
**SDC-6 holds.** Entries are matched by `<family>/<name>` (`inventory/status.go:161`). The status is mapped by reason first (`:193-206`). A matched entry reports its own time, or empty (`:168-171`).
**SDC-7 is partial.** The required documentation edits are present. The literal search scenario has one hit outside the two allowed notes (finding 2).

## Findings
- [WARNING] The spec does not include the proposal's non-goal: a pre-export degrade leaves per-sink entries at `Synced=True`/`Debounced` — `internal/controller/kollectinventory_controller.go:217-220,156-158`
  Failure: the only sink is up to date, then the payload grows past the inline cap with no spill sink. The spill gate calls `setInventoryDegraded` with no export pass. The aggregate is `False`/`SpillRequired`, but the sink's entry stays `True`/`Debounced` and the read API reports `debounced`. That conflicts with how SDC-3's "only sink fails … SHALL NOT keep Synced=True" reads to someone who has only the spec. `proposal.md:150-153` excludes this case; the spec, which is the published contract, does not.
  Fix: add the carve-out (scope, reachability, spill gate, registry missing) to the SDC-3 text, or reset the entries on those paths.
  Confidence: 70
- [NOTE] The SDC-7 scenario allows exceptions only in the upgrade note and the ADR note. The evidence widened that to `docs/REQUIREMENTS.md:89` (FR-EXP-9: "one debounced pass … PartiallySynced") — `openspec/changes/synced-debounce-condition/evidence/G4.md:203`
  Failure: a literal run of the scenario search finds a hit the spec does not allow, so the scenario is not met as written.
  Fix: reword FR-EXP-9 (for example "in one pass"), or name it in the spec scenario.
  Confidence: 85
- [NOTE] SDC-7 asks for an entry under the upgrade guide's "v0.22.0 behaviour changes". The entry is under "Smaller behaviour changes (after v0.21.0)" — `docs/operator-manual/upgrading.md:408`
  Failure: following the file's heading convention is reasonable, but the spec text does not match the file. G4.md:195 records this as a deliberate choice.
  Fix: change the spec wording to "after v0.21.0".
  Confidence: 60
- [NOTE] The aggregate message's denominator is `exported+debounced`, not the bound-sink count — `internal/controller/per_sink_export.go:442-445`
  Failure: today every binding ends up exported, debounced or failed, so the result is correct. A future `continue` that counts none of the three would still print `N/N` and hide the missing sink. SDC-2 says "out of the bound sinks".
  Fix: pass `totalInventorySinkRefs` / `totalClusterInventorySinkRefs` as the denominator.
  Confidence: 40
- [NOTE] The read API rebuilds the entry key by hand instead of sharing `sinkExportKey` — `internal/inventory/status.go:161` vs `internal/controller/inventory_sink_helpers.go:82`
  Failure: if one side changes the key format, every binding quietly falls back to the inventory-level status, which is exactly the bug SDC-6 fixes.
  Fix: move the key builder into `api/v1alpha1`, next to `InventorySinkBinding`.
  Confidence: 45
- [NOTE] An entry with no `Synced` condition keeps the inventory-level status and message but reports the entry's own (possibly empty) time — `internal/inventory/status.go:172-175`
  Failure: SDC-6 says the inventory-level values apply only to a binding with no entry. No current write path produces such an entry: the preview's partial entries are thrown away at `kollectinventory_controller.go:514-515`.
  Fix: report `unknown` for a matched entry that has no condition.
  Confidence: 30

**Behaviour no requirement mentions:**
- The `ReasonPartiallySynced` comment changed.
- The cluster `ExportSucceeded` message gained a prefix (`exported N item(s); `).
- The preview now runs `carryOverSinkExport` for each binding before resolving it.
- The read API message now comes from the per-sink condition.

None of these causes a defect.

## Could not check
- I ran no tests or gates (read-only role). I relied on the recorded results in the evidence files. G4 has no Go test run, and its `task docs:verify` failed on an environment problem (a missing kubectl shim), after which the remaining steps were rerun by hand.
- I did not read G1–G3/T0.1 evidence in full, the deletion and cleanup code behind `deleteInventoryAndFinalize`, or the helpers in `cov_s06_test.go`.
- Whether controller watches actually trigger a reconcile after a sink spec edit or recreate. The scenarios assume the reconcile happens.
