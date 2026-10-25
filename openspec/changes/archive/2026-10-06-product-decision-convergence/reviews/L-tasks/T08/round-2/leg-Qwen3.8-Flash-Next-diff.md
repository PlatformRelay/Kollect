## Verdict: CONCERNS

## Findings

- [WARNING] The round-1 rejection of finding #2 rests on a false fact — "No backend nils a field its Export dereferences — none nil fields at all" (`evidence/T08.md:122-123`): nats does, and the tombstone-discard path turns "may fail against it" into "succeeds and leaks a connection" — `internal/sink/nats/backend.go:77-86` (Close nils `b.nc`/`b.js`), `internal/sink/backend_pool.go:175-177` (discard returns the just-Closed backend as the live one), `internal/sink/nats/backend.go:92,115-155` (Export → `jsProvider`/`jetStream` re-dials whenever `js == nil`)
  Failure: event-sink (nats) deleted exactly while an acquire-build is in flight → `acquireBackend` Closes the built backend (`backend_pool.go:156-158`), hands it back as the working one; `RunExportEnvelope` exports to the deleted sink **successfully** (re-dial on a fresh TCP connection, `nats/backend.go:137`), release is a no-op (`export.go:179`) and the backend is never pooled again → that NATS connection is never Closed by anyone; one live leak per occurrence, and the `storePooledBackend` comment ("the export it backs may fail against it", `backend_pool.go:167-168`) is wrong for nats (succeeds) and equally wrong for s3/gcs/local, which implement no `Close()` at all (`export.go:321-330` makes them no-ops) — the "fails against a closed backend" shape the whole rejection argues only holds for postgres/mongodb/kafka.
  Fix: smallest change — in `storePooledBackend`'s tombstone branch, have `acquireBackend` return `kollecterrors.Terminal("sink deleted mid-build")` instead of the discarded backend; there is no legitimate export to a tombstoned key, so no backend-state assumptions remain on the return path. Otherwise correct the rejection rationale and the `backend_pool.go:165-168` comment to name nats' reconnect.
  Confidence: 85 (code path read end to end; occurrence window is narrow)

- [NOTE] The delete-only half of the wiring is pinned by review, not by a sensor: nothing fails if a future edit adds an `UpdateFunc` (or `GenericFunc`) eviction to the `handler.Funcs` — live sinks would be evicted+Closed on every event — and the envtest spec only exercises Delete (`internal/controller/family_sink_controller.go:106-110`, `evidence/T08.md:28` row 10 is `independent-review | pending`).
  Failure: `UpdateFunc: evict` regression → every status update on a live sink Closes its pooled mid-cadence backend; all 54 specs + unit tests stay green.
  Fix: in the existing envtest spec, one `Consistently(spy.closes == 0)` across a no-op update of the sink object before the Delete step.
  Confidence: 90 (absence verified by grep of the two new test files)

- [NOTE] Sync Close without a deadline on the delete hook: `evictBackendPoolOnSinkDelete` runs `closeBackend` (`backend_pool_log.go:8-12`) on the informer listener goroutine; a kafka `writer.Close()` flush against a dead broker delays every *subsequent* sink delete of that kind (reconcile is unaffected — client-go buffers per-listener). `internal/controller/family_sink_controller.go:87-93`.
  Fix: none required for merge; if delete-latency ever matters, Close asynchronously.
  Confidence: 70 (bounded-delay claim from client-go's buffered per-listener queue, not re-run under fault)

## Could not check
- Ran nothing: build/vet/lint/`task lint`, both `-race` sink runs, the envtest 54/54 and the mutation check are taken from the recorded matrix (`evidence/T08.md`); plan mode is read-only.
- `task verify`, `task test-integration`, gitleaks at commit — not reproducible here; no Docker.
- The claimed round-1 register dispositions beyond what the diff embeds verbatim in `evidence/T08.md:194-219`; `reviews/L-tasks/T08/register.md` and the round-1 leg files were deliberately not opened (out-of-bounds prior reviews).
- s3/gcs/local/bigquery backend internals beyond the presence/absence of `Close()`; postgres/mongodb mid-Export behaviour taken from the round-1 disposition.
- HA/leader-election topology: pool is per-process, assumes only the leader exports — not verified against deployment docs.
