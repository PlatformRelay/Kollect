## Verdict: CLEAN

## Findings
- [NOTE] Evidence cites stale pre-deletion line numbers for two post-fix claims — `openspec/changes/dead-exported-surface/evidence/T6.md:130` cites `engine.go:439` for the reader's `slices.Sort` (actual `internal/collect/engine.go:419`) and `engine.go:622-624` for the nil-kube no-op (actual `internal/collect/engine.go:601-606`). Same class as the round-1 single-leg cite finding dropped in the register.
  Failure: a maintainer jumping to the cited lines lands in unrelated code.
  Fix: re-cite against d48bda9a.
  Confidence: 95
- [NOTE] Helper doc says bindings "mirror production (`syncEngineTargets`)" but omits two things production always does: `RefreshNamespaces` before the loop and a `ScopeCeiling` on the options (`internal/controller/kollectclustertarget_controller.go:206,231` vs helper at `kollectclusterinventory_helpers_test.go:31-88`). Harmless for every current call site (nil kube makes the refresh a no-op; none of these tests exercise ceiling enforcement), but "mirror" will mislead the next test author who assumes a ceiling is bound.
  Failure: no concrete current failure — a future ceiling-dependent test built on the helper gets scopeEnforced=false silently.
  Fix: one doc-comment clause ("no ceiling bound; effective set supplied directly").
  Confidence: 85
- [NOTE] Round-1 accepted finding 3 leaves the sort ratchet unadded in the last round: `NamespacesForClusterTarget`'s `slices.Sort` (`internal/collect/engine.go:419`) now has no order-pinning test — the two remaining controller assertions use `ConsistOf` (`kollectclustertarget_controller_test.go:155,234`, order-insensitive). The Harness-gaps note says a pin "belongs in the controller suite" if a consumer ever depends on order; it names the condition, not the ratchet. Accepted by loop decision; recording, not reopening.
  Confidence: 100 that the pin is absent; accepted.

## What I checked (not rubber-stamp)
- Full diff 3db9bcb3..d48bda9a read line by line; deletion site, all 9 migrated call sites, the helper, evidence, tasks ticks, round-1 register.
- Round-1 fix 1 verified mechanically: helper calls `engine.Start(ctx)` before any `RegisterTarget`; `startInformer` derives `informerCtx` from `informerContext()`=`runCtx`=`ctx` (`engine.go:580-586,704-705`), so `t.Cleanup(cancel)` now stops factories; Start's watcher calls `stopFn`, ending dispatch workers — no leak path left on the helper's happy or Fatalf path.
- Round-1 fix 2 verified: `git grep -n BindClusterTargetNamespaces 3db9bcb3 -- '*.go'` = 17 lines; diffed mechanically against the T6.md record — exact match, 14 call sites in 8 files, zero non-test.
- Post-deletion grep at d48bda9a: symbol survives only in openspec records. Reader `NamespacesForClusterTarget` has 4 production call sites, each with a reaching test per the row-11 map (spot-checked cluster_target_finalizer, unit-suspend, envtest ConsistOf sites).
- target_finalizer_test deletion verified inert: `KollectTargetReconciler` calls `UnregisterTarget(ns,name)` directly (`kollecttarget_controller.go:80`), which clears the store unconditionally.
- Helper fidelity: synthetic shape matches `syncEngineTargets` (`kollectclustertarget_controller.go:213-234`), incl. `LabelMetadataName` selector; EffectiveNamespaces skips the namespace LIST; nil kube safe at `engine.go:601-606`.
- New imports in the helper (`dynamicfake`, `slices`, `corev1`, `schema`) all used; no production import edge (test-only file, go-arch-lint excludes tests — matches its recorded exit 0); no metrics-based test could be disturbed by the now-live `InformerClusterWideScope` sets (zero test references).
- No allow-list, exclusion or baseline touched anywhere in the diff.
- Test-fidelity direction: every migrated site binds through the one production writer (`RegisterTarget`), so post-migration fixtures are stronger, not weaker; bare-state semantics only mattered to the deleted method itself.

## Could not check
- Sensors (build/vet/test/gofmt/arch-lint) not re-run by me — took the orchestrator's exit-0 stack on faith; plan mode, read-only.
- No `-race` or goroutine-leak-detector run on the fixed tree, so the cleanup fix is verified by code reading, not observation.
- Envtest suites behind row-11's :155/:234/:145-164 claims not executed (no binaries run).
- `.go-arch-lint.yml` rules not read in full.
