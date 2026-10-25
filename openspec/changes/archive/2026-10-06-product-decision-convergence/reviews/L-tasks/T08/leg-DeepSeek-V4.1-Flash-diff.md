## Verdict: CONCERNS

## Findings
- [WARNING] The delete watch is never exercised end-to-end — the only functional tests call `evictBackendPoolOnSinkDelete` directly (`internal/controller/family_sink_delete_watch_test.go:120,172`), and the envtest "wiring" spec only asserts `SetupWithManager` returns nil (`internal/controller/setupwithmanager_envtest_test.go:47-49`).
  Failure: if `Watches(PT(&t), …)` ever fails to attach the `DeleteFunc` (framework change, or the handler being shadowed by `For` on the same kind), every BEP-1 eviction silently stops and no gate fails — the task's central claim is unpinned.
  Fix: one envtest that creates then deletes a `KollectSnapshotSink` through the manager and asserts the pooled backend's Close ran.
  Confidence: 75

- [WARNING] Tombstone TTL aging is new behaviour with no sensor — `internal/sink/backend_pool.go:230-234`. Matrix rows 1–15 cover eviction, discard, fallback, no-op, spec-swap, entry-TTL, race and lint, but not this branch.
  Failure: a regression that drops the aging loop (unbounded `tombstones` growth) or ages tombstones too early (a deleted sink's build gets re-pooled) passes every listed gate.
  Fix: pin `pruneStaleEntriesLocked` drops a tombstone older than `backendPoolTTL` and keeps a fresh one.
  Confidence: 90

- [NOTE] "Only the delete hook records tombstones" is asserted in a comment, not a test — `internal/sink/backend_pool.go:286-290`; the alternative evict path `evictPoolKey` is `backend_pool.go:304`.
  Failure: a refactor that makes `EvictBackendPool`/`EvictBackendPoolByUID` share the tombstone write would permanently discard a *live* sink's next acquire, and no test fails.
  Fix: a test asserting `EvictBackendPool` leaves the key acquirable afterwards.
  Confidence: 80

- [NOTE] The tombstone path hands a closed backend back to the caller for use — `storePooledBackend` returns `built, built` (`internal/sink/backend_pool.go:176`), `acquireBackend` closes it (`:157`) then returns it (`:160`), and `RunExportEnvelope` calls `backend.Export` on it (`internal/sink/export.go:170-189`).
  Failure: the spec sanctions "may fail against it", but a backend whose `Export` panics after `Close` converts an accepted failure into a controller-goroutine panic.
  Fix: verify each registered Backend's `Export` is Close-safe, or return a terminal error on the tombstone path instead of a closed backend.
  Confidence: 55

- [NOTE] Matrix row 11's cited sensor over-claims — it says the wiring spec "registers for all three family-sink kinds", but `setupwithmanager_envtest_test.go:47` registers only `KollectSnapshotSink`. The claim holds by generic instantiation plus production wiring (`cmd/main.go:276-303`), not by the sensor named.
  Fix: cite `cmd/main.go`, or register all three in the spec.
  Confidence: 95

## Could not check
- Ran no gate (read-only session): every exit code in `evidence/T08.md` is unverified, including the four T03 reds→green, the `breakerRegistry` race, `task lint`, and gofmt.
- Did not trace `Backend` implementations: no `Export`/`Close` methods exist in `internal/sink/*.go` root, so finding 4's panic risk is unconfirmed.
- Did not read `.golangci.yaml` depguard or `.go-arch-lint.yml` to independently confirm no allow-list/exclusion widening (the evidence claims the controller→sink edge is pre-existing).
- Did not read the `run-task.sh` T08 prompt or the untracked `reviews/L-tasks/T08/` directory.
