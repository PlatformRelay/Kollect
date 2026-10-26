## Verdict: BLOCK
## Findings
- [CRITICAL] Debounce does not establish that the current sink destination received the payload — `internal/controller/per_sink_export.go:73`
  Failure: Change a sink’s database/table or recreate its CR between reconciles; inventory generation, checksum and requestedAt remain unchanged → both reconcilers can report True/Debounced against an unpopulated destination, indefinitely at 0s.
  Fix: Include sink UID and effective export configuration in baseline validity, including preview checks; add replacement and configuration-change scenarios.
  Confidence: 100
- [CRITICAL] The planned resolve-failure reset misses the production entry path — `internal/controller/kollectinventory_controller.go:154`
  Failure: Delete the sink and reconcile → reachability returns early, bypassing exportToSinks and its proposed forget call; recreation retains the baseline. Cluster reconciliation similarly returns at line 149. Preview also ignores resolution errors at line 490.
  Fix: Invalidate affected baselines on reachability failures, reject failed preview resolutions, and explicitly reconcile during deletion in the G3 test.
  Confidence: 100
- [CRITICAL] Total failures discard the per-sink outcome, so carry-over alone cannot deliver SDC-3 — `internal/controller/kollectinventory_controller.go:251`
  Failure: A single previously successful sink fails → setInventoryDegraded persists the old per-sink True condition; cluster has the same defect at line 211. G2’s claimed “base: nil” is wrong: the old record survives.
  Fix: Persist outcome.SinkExports before both total-failure status writes; correct the root-cause claim and make the red assert the stale True condition.
  Confidence: 100
- [CRITICAL] The proposed read-API change cannot reach real controller entries — `internal/inventory/status.go:160`
  Failure: Controllers write names such as database/warehouse, but the API compares against warehouse → reason-first mapping is bypassed, reporting aggregate ok instead of debounced and potentially another sink’s export time.
  Fix: Match canonical family/name keys and test controller-shaped entries, including duplicate names across families and distinct export times.
  Confidence: 100
- [WARNING] SDC-5’s no-Event guarantee fails on the snapshot path — `internal/controller/export_spill.go:87`
  Failure: An unchanged snapshot inventory with at least 1800 rows falls through preview; noteExportShardWarning returns true every pass → the reconciler emits a Warning even when every sink debounces.
  Fix: Emit the warning only when its condition meaningfully changes; add a snapshot no-churn case above the threshold.
  Confidence: 100
- [WARNING] SDC-5 contradicts both itself and the design — `openspec/changes/synced-debounce-condition/specs/inventory-sync-status/spec.md:123`
  Failure: The first debounce must change Exported to Debounced despite the blanket no-status-change rule; a successful-to-failed transition must change lastTransitionTime despite lines 129–131 requiring preservation across failure.
  Fix: Restrict no-write guarantees to repeated debounced passes; preserve export time/checksum across failure, but transition time only while condition status remains unchanged.
  Confidence: 100
- [WARNING] The mixed exported/debounced scenario cannot occur as specified — `openspec/changes/synced-debounce-condition/specs/inventory-sync-status/spec.md:59`
  Failure: Changing inventory requestedAt invalidates every binding, including B’s 0s binding → both export, rather than A exporting and B debouncing.
  Fix: Make A’s interval expire while B remains change-only, using controlled tracker timestamps; assert actual backend calls.
  Confidence: 100
- [WARNING] Timestamp-based reds and controls are nondeterministic — `openspec/changes/synced-debounce-condition/tasks.md:115`
  Failure: Consecutive export/debounce or failure/recovery calls within one second serialize identical metav1.Time values → the intended red can pass and the “green” transition control can fail.
  Fix: Separate compared transitions across second boundaries or control their clock; apply this to probe 0.1 and G2, and compare freshly fetched objects.
  Confidence: 100
- [WARNING] Existing message assertions omitted from G1 will keep its required suite red — `internal/controller/requested_at_annotation_test.go:257`
  Failure: The new aggregate message breaks this assertion and its cluster counterpart at line 402, although the proposal lists only the aggregate and read-API tests as needing changes.
  Fix: Add both requestedAt helper assertions to G1’s explicit test changes, preserving their forced-export checks.
  Confidence: 100
- [WARNING] SDC-7’s document list and grep miss remaining obsolete guidance — `docs/operator-manual/metrics.md:87`
  Failure: Metrics documentation still recommends aggregate PartiallySynced to distinguish debounce; its explanation spans lines and escapes the proposed grep. api/v1alpha1/export_interval_types.go:16 also defines PartiallySynced as debounce.
  Fix: Update both locations and inspect multiline matches; exclude unrelated “debounced pass” descriptions such as docs/REQUIREMENTS.md:89 from the negative check.
  Confidence: 100
- [WARNING] The verification matrix does not prove the full production-path requirements — `openspec/changes/synced-debounce-condition/tasks.md:96`
  Failure: G2/G3 exercise namespaced database sinks only → cluster carry-over/reset defects and snapshot full-path churn can survive; helper-only 0s coverage does not establish effective scope-floor behavior.
  Fix: Parameterize relevant transition tests across both reconcilers and snapshot/database paths, and cover effective intervals, partition failures, restart, generation and requestedAt invalidation.
  Confidence: 98
- [NOTE] Several source ranges omit the decisive statements they claim to establish — `openspec/changes/synced-debounce-condition/proposal.md:17`
  Failure: shouldSkip :59–97 omits its interval return at 98; record :99–120 omits requestedAt at 121 (function is 101–122); preview :463–517 omits its success return at 518. Design line 92 also incorrectly promises nextDue=1s for 0s bindings.
  Fix: Correct ranges to 59–99, 101–122 and 463–519, including tasks’ repeated shouldSkip citation; document the zero-interval watchdog exception.
  Confidence: 100
## Could not check
- No tests, envtest probes, mutations or live cadence reproduction ran. Direct Reconcile calls can inspect resourceVersion and FakeRecorder output, but cannot prove automatic watch re-enqueueing. Pinned module sources support condition timestamp preservation and unchanged-object storage short-circuiting.
