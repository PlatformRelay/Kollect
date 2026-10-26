I'll review this target systematically. Let me start by reading the evidence and diff.
## Verdict: CONCERNS

## Findings
- [WARNING] New test helper never calls `Engine.Start`, so its `t.Cleanup(cancel)` cannot stop the informer it starts — `internal/controller/kollectclusterinventory_helpers_test.go:40-43`, `internal/collect/engine.go:582-588`
  Failure: helper does `ctx, cancel := context.WithCancel(...)` / `t.Cleanup(cancel)` then `NewEngine(dyn,…)` and `RegisterTarget(ctx,…)`. `RegisterTarget` starts the informer via `e.startInformer(e.informerContext(), …)` (engine.go:369); `informerContext()` returns `context.Background()` because `runCtx` is only set by `Start` (engine.go:510). The factory's stop channel therefore derives from Background, not from `ctx`, so cancelling on cleanup never closes it: each of the 9 migrated call sites leaks an informer (`factory.Start(informerCtx.Done())`, engine.go:704) plus its client-go goroutines for the life of the test binary. Contrast the established fixture pattern at `internal/controller/kollectclustertarget_nsrefresh_test.go:59-61`, which does call `engine.Start(engineCtx)`.
  Fix: in the helper, call `engine.Start(ctx)` after `NewEngine` (before the registration loop). This makes `runCtx == ctx`, so `t.Cleanup(cancel)` stops the informers and dispatch workers.
  Confidence: 85 (traced the context chain; did not run with a leak detector to observe the count).
- [NOTE] The reader's sorted-output behaviour loses its only order-asserting test — `internal/collect/engine.go:419`, `internal/collect/engine_setters_test.go:35-50`
  Failure: the deleted `TestEngineSetScrubKeysAndBindClusterTargetNamespaces` was the sole assertion that `NamespacesForClusterTarget` returns `[team-a, team-b]`. No migrated site asserts order (finalizer/suspend loops are order-insensitive; `composeNamespaceRollup` sorts its own names at kollectclusterinventory_controller.go:455). A future regression dropping `slices.Sort` would be invisible.
  Fix: assert sorted order once in a migrated test, or keep a tiny same-package reader test seeded via `RegisterTarget`.
  Confidence: 90 (the loss is real; impact is low — the evidence's harness-gap note already records it).
- [NOTE] Helper doc comment overstates fidelity to production — `internal/controller/kollectclusterinventory_helpers_test.go:25-30`
  Failure: it claims to mirror `syncEngineTargets` but omits the `RefreshNamespaces` call (kollectclustertarget_controller.go:206) and `ScopeCeiling`, and drops `CollectionFilterSpec`. Harmless for the reader path (only `Name`/`Namespace` are read), but the comment could mislead a maintainer into trusting it as a general production fixture.
  Fix: narrow the comment to "reproduces the RegisterTarget binding shape needed by `NamespacesForClusterTarget`".
  Confidence: 80.

## Could not check
- Did not execute the sensors (build/vet/tests/arch-lint) — plan mode is read-only; I relied on the recorded exit-0 runs in `evidence/T6.md`.
- Did not inspect `.go-arch-lint.yml` in full; accepted evidence row 14 that controller→collect and the new vendor test imports add no component edge.
- Did not read any other review leg (out of bounds), nor the wider `openspec/changes/dead-exported-surface/` review directory.
