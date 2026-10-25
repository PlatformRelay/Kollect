## Verdict: CONCERNS

The branch delivers the five product decisions and the T11 fixes hold. My concerns are the deferred lifecycle items (6, 8, 11), which I judge real but correctly named as residuals — not missed.

## Findings

- [WARNING] Cluster `collectedCount` can stay stale indefinitely: the cluster target never requeues and does not watch the resources it collects, so its count only refreshes on target/Namespace/Profile/Scope events — unlike the namespaced path's liveness requeue. — `internal/controller/kollectclustertarget_controller.go:400` (SetupWithManager, no `RequeueAfter`) vs `internal/controller/kollecttarget_controller.go:359`.
  Failure: add/remove objects in a matched namespace → `kctgt` `status.collectedCount` (and the Ready prose) freeze at the last event-triggered value; a GitOps reader polls a number that never moves.
  Fix: requeue `ctrl.Result{RequeueAfter: r.Options.targetCountResync()}` on the Ready path (mirror the namespaced controller), or state the difference in TSP-1.
  Confidence: 85 (register #6; deferred as owner decision, not refuted).

- [WARNING] Eviction `Close()` runs inline on the controller-runtime informer dispatch goroutine, so a slow backend `Close` stalls every sink event of that kind, including live sinks' reconciles. — `internal/controller/family_sink_controller.go:106` → `internal/sink/backend_pool.go:328`.
  Failure: a `Close` that blocks on network I/O (nats/kafka) delays delivery of all subsequent delete/update events for the kind for its duration.
  Fix: hand the evicted backend to a short-lived goroutine (or a bounded closer) instead of closing on the handler goroutine.
  Confidence: 70 (register #8).

- [WARNING] Evict-during-use leaks a redialled NATS connection: a pooled backend held by an in-flight export is Closed, then `jetStream()` redials and stores a fresh `nc`, which nothing ever Closes (the pooled-hit release is a no-op). — `internal/sink/nats/backend.go:116-157` + `internal/sink/backend_pool.go:141`.
  Failure: delete a NATS sink while one of its exports is mid-`Export` → fresh connection + goroutines leak for process lifetime.
  Fix: make the eviction path mark the backend dead before/under its mutex, or have the pooled-hit release observe a tombstone and Close.
  Confidence: 70 (register #11; deferred).

- [NOTE] T11 terminal classification is correct and stronger than the finding required: `ConfigFromSpec` is pure, so `Terminal` at `internal/sink/git/backend.go:33` is right; dropping `ClassifyAPI` at `internal/sink/export.go:185`/`cleanup.go:193` also stops the pre-existing demotion of `registry.go:89`'s Terminal "unknown sink type" to transient. `ResolveSecret` rewrites NotFound (`credentials.go:45`), and `ClassOf` derives the same class for unclassified API errors, so no unclassified-acquire regression. Verified by reading, not by running.
  Confidence: 90.

- [NOTE] T11 tombstone bounding holds: `pruneExpiredTombstonesLocked` (`internal/sink/backend_pool.go:254`) runs on both the acquire cycle and the delete hook (`:326`), so an export-less manager is bounded; pruning after `backendPoolTTL` is safe because UIDs are unique. Register #7 (deferred-close instead of discard) is spec-compliant: BEP-1 requires "not pooled", not "closed synchronously".
  Confidence: 90.

- [NOTE] ERA-1/ERA-2/GTE-1..3 mechanisms match the spec on reading: `requestedAt` third axis checked before the zero-interval return (`per_sink_export.go:82`) and threaded to both inventory kinds and the preview; the stamp runs after prune/scrub (`prune.go:79`) and only when `metadata` survives; CRD/golden/docs admit `go-git` only. Not executed.

## Could not check
- Did not run `task verify`, `task lint`, coverage, `-race`, or any test (plan mode is read-only; those regenerate files). All Verification-table greens remain claims.
- Did not reproduce the `internal/collect` `-count=2` non-idempotency at base `3ee21266`.
- Integration tier (`task test-integration`) — needs Docker.
- Mutation testing of the new red tests, and base-vs-branch coverage.
- The sibling knowledge-base source of the five decisions (`data/kollect-xconsol-*`) is outside this repo.
