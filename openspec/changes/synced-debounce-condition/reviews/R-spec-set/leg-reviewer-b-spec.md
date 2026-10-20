## Verdict: BLOCK

## Findings
- [CRITICAL] SDC-6 cannot hold in production. The read API matches `sinkExports[].name` against the bare `ref.Name`, but the controller names each entry `<family>/<name>`. The per-sink override therefore never fires on a real object — `internal/inventory/status.go:160` vs `internal/controller/inventory_sink_helpers.go:81-83`, `kollectinventory_controller.go:320-322`
  Failure: a real inventory whose sink is debounced reports the aggregate status. Today that is `degraded` (aggregate `False`); after SDC-2 it becomes `ok`, never `debounced`. `cov_s06_test.go:54-89` uses bare names like `git-ok`, so the planned 1.1 row goes green and hides this.
  Fix: match on `sinkExportKey`, or strip the family prefix. Add a test whose entry names come from `sinkExportKey`. Note that proposal SDC-6 ("keeps reporting") is false today.
  Confidence: 85
- [CRITICAL] A total-failure pass never writes `outcome.SinkExports`, so the SDC-3 per-sink clause and one G2 red rest on a false premise — `kollectinventory_controller.go:251-270` → `setInventoryDegraded` `:627-659`; cluster `kollectclusterinventory_controller.go:211-231`
  Failure: with one sink (or every sink failing, none debounced), the stored entry keeps its old `True/Exported` condition, and T1/C1 are kept already on base. Task 2.1 "keeps the last successful export record across a failed pass" claims "base: nil", which is wrong. Its `False/ExportFailed` assertion stays red after the designed `carryOverSinkExport`, which never runs on this path. SDC-3 says the failed sink is `Synced=False` "as before this change", but before this change it was not. Proposal :27-28 ("a debounced or failed pass erases") holds only for partial failure.
  Fix: either write the per-sink entries in the total-failure branch (spec it as new behaviour), or scope SDC-3 and the test to partial failure. Fix the base claim.
  Confidence: 85
- [WARNING] G3 envtest "reports a resolve failure, then exports on recovery" cannot go green the way the design says. A deleted sink CR is caught by the reachability gate before `exportToSinks`, so the resolve-failure branch where `forget` would be called (`:324-328`) is unreachable in this scenario — `kollectinventory_controller.go:153-159`, `sink_status.go:79-88`
  Failure: delete → `setInventoryDegraded`, with no `forget` and the baseline intact. Recreate → the preview (`:192`, Postgres raw checksum equals the recorded one) debounces and reports `True/Debounced` for a sink that may now be a fresh backend. Mutation M2 can never be killed.
  Fix: call `forget` for the bindings in the reachability/degraded branch (and the cluster equivalent), or drop the resolve-branch claim and the M2 mutation.
  Confidence: 80
- [WARNING] The SDC-2 scenario "One sink exported, one debounced" cannot happen as written. A `requestedAt` change invalidates every binding, including the `0s` one: the check runs before the zero-interval return — `internal/controller/per_sink_export.go:86-92`
  Failure: both A and B export, so the scenario never exercises the mixed case.
  Fix: create the mix with an elapsed interval on A (short interval plus wait, or an injected clock), not with `requestedAt`.
  Confidence: 90
- [WARNING] Tests not in the plan break under SDC-2's new message — `internal/controller/requested_at_annotation_test.go:257`, `:402`
  Failure: both assert `"exported to 1 sink(s)"`, which becomes `"1/1 sink(s) up to date"`. Proposal Impact (:114-119) and task 1.1 list only `TestAggregateInventorySync` and `cov_s06`, so G1 goes red outside its named reds.
  Fix: add *Test changes* rows citing SDC-2 for both.
  Confidence: 95
- [NOTE] SDC-5 covers both kinds, but no cluster test checks that `resourceVersion` stays unchanged. The cluster path has no preview and rewrites `NamespaceShards`/`ExportSucceeded` on every pass — `kollectclusterinventory_controller.go:729-770`
  Fix: add a cluster envtest for "writes no status on a repeated debounced reconcile", or scope SDC-5 to namespaced.
  Confidence: 60
- [NOTE] Stale citations: `shouldSkip` is `per_sink_export.go:59-99` (cited 59-97); `record` is `:101-122` (cited 99-120). I checked these and found them accurate: 300, 363-371, 437-443, 446, 463-517, 498-506, 558, 563-577, 696-697; cluster 312-320, 359-365, 744; `status.go:167-180`; `export_interval.go:246-251`; `suite_test.go:31`; `conditions.md:10`; `troubleshooting.md:222`; `kollectinventory.md:160`; the three `ReasonDebounced` test lines.
  Confidence: 95
- [NOTE] What holds against the code: the SDC-4 first clause (`per_sink_export.go:73-88`); "no Event when `ExportedCount==0`" (`kollectinventory_controller.go:565`); `Ready`/`SinkReachable` messages stable across debounced passes (`:569-576`, `sink_status.go:142-145`); the per-sink `Exported`↔`Debounced` alternation keeps `lastTransitionTime` because status stays `True`; the 1.1 mixed debounced+failed envtest goes through `updateStatus`, because `isTotalExportFailure` needs `DebouncedCount==0` (`per_sink_export.go:210-212`).

## Could not check
- `reviews/R-spec-set/*` and `loop.md`: the other reviewers' output and the loop state, deliberately not read.
- Whether the API server skips a no-op status Update for this CRD, including the managedFields timestamp. Probe 0.1 owns this and I ran nothing.
- The pinned `k8s.io/apimachinery` `SetStatusCondition` source (module cache is outside the repo).
- `docs/adr/0413-export-interval-scheduling.md` §2/§6, `upgrading.md`, `kollectclusterinventory.md`: not read.
- The cluster reconciler outside lines 150-240, 270-320 and 725-835.
