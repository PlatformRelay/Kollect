## Verdict: CONCERNS

## Findings
- [WARNING] The new `Watches(...)` DeleteFunc wiring itself has no test — `internal/controller/family_sink_controller.go:104-111`.
  Failure: the two new hook tests call `evictBackendPoolOnSinkDelete` directly (`family_sink_delete_watch_test.go:120,172`), and the COV-90-S07 wiring spec only asserts `SetupWithManager` returns nil without ever starting the manager (`setupwithmanager_envtest_test.go:42-81`). Deleting the entire `Watches(...)` clause from the builder keeps every recorded sensor green — the change's central claim (delete event → seam) is pinned only by reading the chain. The red phase tested the stubbed hook body, not the wiring.
  Fix: an envtest spec that creates a sink, stops it being deleted... smallest form: a spec that starts the manager, creates+deletes a KollectSnapshotSink, and asserts the seam fires (e.g. via an exported test hook or pool observation); failing that, downgrade matrix rows 10-11 from "wired" to "read-only review" in the evidence.
  Confidence: 85
- [NOTE] Delete-time eviction `Close()`s a backend a concurrent in-flight export may still be using — `internal/sink/backend_pool.go:291-302`.
  Failure: export acquires the pooled backend by reference (`backend_pool.go:136-141`, release is a no-op); a sink delete mid-export closes the backend under it → export fails with a "closed"-class error instead of completing. Inherent to BEP-1 without refcounting; the tombstone guards the store path only. Worth one sentence naming this accepted window in the spec/comment.
  Confidence: 60 (real window; likely accepted design, found no explicit statement of it).
- [NOTE] Tombstones age out only inside `pruneStaleEntriesLocked`, which runs only on `acquireBackend` (`backend_pool.go:230-234,135`) — `internal/sink/backend_pool.go:217`.
  Failure: a controller that deletes sinks but stops exporting never prunes; the tombstone map grows per deleted sink with no TTL relief. Same pre-existing flaw as entries, but this diff adds a second such map, now written on every delete event even with no pooled entry (`:297`).
  Fix: acceptable as-is (small fixed-size records, TTL comment covers it); a cheap ratchet is also pruning on the periodic reconcile of any kind if one is added later.
  Confidence: 70
- [NOTE] Comment cites the wrong source path for the tombstone-unwrap claim — `internal/controller/family_sink_controller.go:84`.
  Failure: I verified the behavior against the pinned module — `EventHandler.OnDelete` does unwrap `DeletedFinalStateUnknown` and set `DeleteStateUnknown` (`$GOMODCACHE/sigs.k8s.io/controller-runtime@v0.24.1/pkg/internal/source/event_handler.go:123-146`) — but the path is `pkg/internal/source/event_handler.go`, not `internal/source/event_handler.go`.
  Fix: correct the path in the comment.
  Confidence: 95
- [NOTE] tasks.md T08 is still `- [ ]` (`openspec/changes/2026-10-06-product-decision-convergence/tasks.md:60`) and the diff does not touch tasks.md, though the evidence's own scope line promises a "tasks.md tick" (T08.md:6). Defensible while status is SENSING → REVIEW; drop the promise or tick at close.
  Confidence: 90
- [NOTE] Verified-clean items (so "no issues" isn't implied laziness): delete-only handler is genuinely safe — nil `Create/Update/GenericFunc` are no-ops in pinned v0.24.1 (`pkg/handler/eventhandler.go:125-144`), matching matrix row 10's pending claim; `For`+`Watches` on the same kind registers without `ErrWatchOverridden` (`pkg/internal/controller/controller.go:236-248`); UID-key symmetry holds — production acquires pass `resolved.UID` (`kollectinventory_controller.go:414`, `kollectclusterinventory_finalizer/sink_cleanup.go`), as does the hook, and k8s never reuses UIDs, so no tombstone can poison a live sink's key; finalizer cleanup necessarily precedes the informer Delete event, so delete-time retraction can never run against a tombstoned key; store/evict race is fully serialized under `globalBackendPool.mu` with no leak window; `ResetBackendPoolForTest` covers tombstones; the two hook test runs and the four T03 reds pass here (`go test -count=1` both packages, exit 0); commit message matches the diff; no new module dependency, no allow-list/baseline widening; `backend_pool_delete_hook_test.go` diff is comment-only as claimed.
  Confidence: n/a (evidence list)

## Could not check
- Full `-race -count=1/-count=2 ./internal/sink`, `./internal/controller` envtest suite, `task lint`, golangci/go-arch-lint — accepted the orchestrator's re-stated gate exits on faith (targeted `-run` subsets re-run green, no race re-triggered in those).
- The `breakerRegistry` pre-existing race signature — only as documented in the evidence, not reproduced.
- `task verify` / `task test-integration` / gitleaks — not run here (evidence records them as not-run/pending); no generated artifact in `git diff --stat` matches the claim.
- Anything under `../_workbench/` or prior reviews (out of bounds by instruction).
