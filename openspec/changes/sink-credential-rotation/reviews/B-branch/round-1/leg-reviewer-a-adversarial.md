## Verdict: BLOCK

## Findings

- [CRITICAL] F1: Missing/empty Secrets discovered during the full read bypass retirement and the store fence, violating SCR-2 — `internal/sink/backend_pool.go:203`
  Failure: A and B miss an empty pool at the same generation; B reads valid credentials and pauses building; the Secret is deleted; A’s full read returns NotFound without bumping the generation; B then pools its obsolete backend at `internal/sink/backend_pool.go:510`. Alternatively, B stores before A fails and remains pooled.
  Counterargument: Metadata failures already retire correctly, but this full-read error branch bypasses that protection.
  Fix: Apply generation-aware missing/empty retirement and fence invalidation to full-read failures, including errors discovered by `caRulesFirst`; add both interleaving tests.
  Confidence: 98

- [CRITICAL] F2: An unchanged successful `0s` inventory can export periodically after coalescing-state expiry, violating SCR-7 — `internal/controller/per_sink_export.go:124`
  Failure: A exports successfully, then remains unchanged for over 48 hours. Another inventory’s successful export prunes A’s state; A’s next watchdog reconcile sees no state at line 70 and exports again. Both reconcilers share this tracker.
  Counterargument: This TTL predates the branch, but SCR-7 explicitly promises no unchanged re-export at any later time; the new documentation repeats that promise.
  Fix: Track observation activity separately from successful exports and preserve actively reconciled `0s` state; test unchanged reconciliation beyond the TTL while another inventory exports.
  Confidence: 100

## Answers

- Rotation/deletion: holds for fresh next-export acquires through uncached metadata checks; missing/empty retirement is broken (F1) — `internal/sink/backend_pool.go:190`, `internal/sink/secret_refs.go:137`.
- Backend lifetime/cleanup: holds in production, including nested prune, fenced builds, surplus disposal and TTL; test-only reset deliberately closes held backends. Stream and non-retracting object-store cleanup skip Secrets — `internal/sink/backend_pool.go:461`, `internal/sink/backend_pool.go:493`, `internal/sink/backend_pool.go:541`, `internal/sink/backend_pool.go:105`, `internal/sink/cleanup.go:175`, `internal/sink/cleanup.go:208`.
- Stale reads/build ordering: newer entries can be retired by forced or failed re-reads, but holders drain; successful rebuilds read values after retirement. No newer-entry overwrite found; full-read failures leave a fencing gap (F1) — `internal/sink/backend_pool.go:325`, `internal/sink/backend_pool.go:379`, `internal/sink/backend_pool.go:510`.
- Credential disclosure: holds for inspected SCR-4 additions: keys retain metadata, annotations are discarded, and new naming errors contain no values. Third-party constructor errors remain passed through, as explicitly excluded — `internal/sink/secret_refs.go:145`, `internal/sink/secret_refs.go:196`, `internal/sink/backend_pool.go:214`, `internal/sink/export.go:107`.
- `0s` scheduling: broken (F2). Total terminal export failures still suppress requeue in both reconcilers; partial terminal failures can retry alongside successful/debounced bindings — `internal/controller/kollectinventory_controller.go:265`, `internal/controller/kollectclusterinventory_controller.go:227`, `internal/controller/per_sink_export.go:204`.
- CLI parity: holds for empty effective values and fresh construction on subsequent runs — `internal/pipeline/wire.go:177`, `internal/pipeline/wire.go:205`, `internal/pipeline/wire.go:531`.
- Documentation: broken by F1’s “backend is dropped” and F2’s “identical payloads never re-sent” claims; inspected cleanup rules match the code — `docs/crds/kollectdatabasesink.md:91`, `docs/operator-manual/troubleshooting.md:397`, `internal/sink/cleanup.go:188`.

## Could not check

- Did not rerun full gates, race suites, envtest or external-backend integration tests; accepted recorded exit codes.
- Focused pool acquire/release and controller coalescing/zero-interval tests passed. F1’s interleavings and F2’s combined TTL/watchdog scenario were traced statically; no files were modified.
