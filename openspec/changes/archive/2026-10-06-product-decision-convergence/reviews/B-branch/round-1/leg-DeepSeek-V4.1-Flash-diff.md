## Verdict: CONCERNS

## Findings

Requirement map (spec-lens, verified against code): ERA-1 **holds** (`per_sink_export.go:82`, both controllers read `requestedAt`, preview agrees). TSP-1 **holds** (`collected_count.go:34`, escape hatch `kollectclustertarget_controller.go:338`). BEP-1/BEP-2 **hold** (`backend_pool.go:289-313`, TTL unchanged). GTE-1/GTE-2/GTE-3 **hold** (validation+config+schema reject `cli`; `x/crypto v0.57.0` registers both KEX at `ssh/kex.go:409,448`; docs truthed). ERA-2 **partial** — see below.

- [WARNING] ERA-2's unconditional "SHALL carry" is not met for the default profile — `internal/collect/prune.go:95-108`, `api/v1alpha1/export_spec_types.go:107-110`
  Failure: a Resource-mode profile that omits `export.include` defaults to `SpecAndStatus`, which drops `metadata` (`prune.go:137-139`); `stampCollectedGeneration` returns early, so the exported copy has no `kollect.dev/collectedGeneration` — the exact headline case ERA-2 ("Resource-mode copy is stamped", generation 42) describes. Only `include: All|MetadataOnly` stamps.
  Fix: qualify ERA-2's headline to "when metadata survives the include section", or make `ResourceExportEnabled` imply keeping `metadata` (stamp a minimal block). The no-stamp carve-out is a separate scenario, so the requirement currently reads as a stronger guarantee than the code gives.
  Confidence: 80 (intentional and documented in design D2 / `ANNOTATIONS-LABELS.md`, but the requirement text still overstates).

- [NOTE] New unbounded growth vector: tombstones are added on every delete and only aged out on acquire — `internal/sink/backend_pool.go:302-313` vs `:241-245`
  Failure: a long-lived manager that deletes many sinks but performs no further exports (no `acquireBackend`) never runs `pruneStaleEntriesLocked`, so the tombstone map grows by one entry per deleted sink for the process lifetime. Entries shared this property, but tombstones are recorded even when no entry existed, so churn without exports now leaks.
  Fix: prune tombstones opportunistically in `evictPoolKeyForDelete` (or bound the map to N).
  Confidence: 65.

- [NOTE] Delete hook runs backend `Close()` synchronously on the informer dispatch goroutine — `internal/controller/family_sink_controller.go:105` → `internal/sink/backend_pool.go:311`
  Failure: a backend whose `Close` blocks (network flush) stalls delivery of that kind's create/update/delete events for the whole controller until it returns; reconciles for the kind are delayed. `closeBackendLogged` is called outside the pool mutex but still inline.
  Fix: dispatch the close to a small bounded worker, or document why inline close is acceptable.
  Confidence: 60.

- [NOTE] `.gitignore:97` edit is a no-op — the added comment "fanout-review raw leg transcripts (never committed)" has no glob after it.
  Fix: add the intended pattern (e.g. `**/reviews/**/*.log`/`.md`) or drop the comment. Confidence: 95.

- [NOTE] Target does more than any requirement asks: an `Age` printer column added to `KollectClusterTarget` (`api/v1alpha1/kollectclustertarget_types.go:63`); the tombstoned build is handed to the caller open and closed by its release (`backend_pool.go:159-163`) rather than discarded immediately as BEP-1's word "discarded" implies; `EvictBackendPool`/`EvictBackendPoolByUID` retained as test-only utilities. All benign, but the tombstone behaviour is a deliberate softening of the requirement wording. Confidence: 90.

Fitness: the convergence **reduces** coupling/complexity (removes `GitEngine` type, `Config.Engine`, engine branch points, 9 CLI-mirror regression tests whose surface is now unreachable — `delete_mirror_regression_test.go`). No allow-list/exclusion/baseline widened (`.go-arch-lint.yml`, `.golangci.yaml`, `codecov.yml`, `Taskfile.yml` COVERAGE_MIN unchanged); `controller → sink` was already permitted. Ratchets that would catch regressions exist and were exercised: `task verify` (CRD drift), `test/schema/engine_enum_test.go`, `TestSSHKeyExchangeOffer_*`, arch-lint. I ran `go build ./...`, `go test ./test/schema ./internal/validation ./internal/sink/git -run 'Engine|Kex|FileRemote|ValidateGit'` — all green.

## Could not check
- envtest/integration rows (`family_sink_delete_watch_envtest_test.go`, `task test-integration`): no Docker/KUBEBUILDER_ASSETS here; took the wiring claim from source read of `controller-runtime@v0.24.1` `internal/source/event_handler.go:123-156` (tombstone unwrapped; `handler.Funcs` nil-safe at `handler/eventhandler.go:130+`).
- Did not run `task verify`, `task lint`, or `task coverage:race`; did not read the per-task evidence files, review registers, or `openspec/.../evidence/*` (trusted the Verification table's SHAs only as claims).
- Did not re-verify the `docs/adr/0415` / `0407` / chart-README truth-up line-by-line beyond the diff.
- Did not inspect the pipeline (`internal/pipeline`) export path for `file://` handling after the convergence.
