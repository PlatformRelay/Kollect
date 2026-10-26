## Unified verdict: CONCERNS  (legs ok: 2/2)

| # | Sev | Claim (one line) | Where | Models | Legs | Conf |
|---|---|---|---|---|---|---|
| 1 | CRITICAL | New test helper's `t.Cleanup(cancel)` is inert: `Engine.Start` is never called so informers derive from `informerContext()`=`context.Background()` (engine.go:582-589) and the real cancels sit uninvoked in `e.informerCancels` — per-call informer/goroutine leak for the test binary | internal/controller/kollectclusterinventory_helpers_test.go:40-43 | DeepSeek, Qwen | 2 | 100 |
| 2 | WARNING | Evidence probe record claims "exit 0, 16 hits, unabridged" but lists 15 lines and a re-run at 3db9bcb3 yields 17 — `kollectclusterinventory_dedupe_test.go:348-349` omitted; matrix row 1 rests on a record contradicting measured output | openspec/changes/dead-exported-surface/evidence/T6.md:31 | Qwen | 1 | 95 |
| 3 | WARNING | Deleted `TestEngineSetScrubKeysAndBindClusterTargetNamespaces` was the only order-assertion for `NamespacesForClusterTarget`'s sorted output; no migrated test pins it (both legs verified no order-dependent consumer today) | internal/collect/engine.go:419 | DeepSeek, Qwen | 2 | 100 |

## Disagreements
- Finding 1 impact/fix: DeepSeek — real leak, fix by calling `engine.Start(ctx)` in the helper (makes `runCtx==ctx`, cleanup then stops informers and dispatch workers); Qwen — harmless in a short-lived fake-client test binary (no events, no workers), fix by dropping the `WithCancel` or annotating "engine has no stop path". Both agree the cleanup is a no-op.
- Finding 1 grading: DeepSeek WARNING vs Qwen NOTE — consequence of the impact disagreement above, not of the mechanism.

## Nobody could check
- `task arch-lint` / full `task check` not re-run by either leg; both relied on the evidence's recorded exit-0 run (27.6s OK); DeepSeek did not inspect `.go-arch-lint.yml` in full.
- Envtest reach-ability claims (evidence rows 11/12 for the two controller test files) — no envtest binaries executed; only absence of references to the deleted symbol was verified.
- Origin of the 17-vs-15 miscount (probe run vs transcription into T6.md — no raw log left); no leak-detector run to observe the informer/goroutine count from finding 1.

Dropped (NOTE, single leg): DeepSeek's "helper doc comment overstates fidelity"; Qwen's "wrong line cite in evidence (target_finalizer.go:51 vs :80)".
