# Tasks

Every task: run its zero-caller probe first and record it, delete, then prove with
`go build ./...` and `go vet ./...` plus the affected package suite. No behavioural red exists
for this change (unreachable surface): the probe record plus post-deletion compile plus the
package suites are the per-task evidence, per task 1.1 of each task below. Files behind a
build tag are additionally compiled with that tag (`go vet -tags integration ./...` for the
integration files) because the default build skips them.

## 1. Dead sink runner (DR-1)

- [ ] 1.1 Probe (record the full list, do not truncate): `grep -rn "RunExportItems\|ExportItemsRequest" --include='*.go' internal/ cmd/ api/ test/ hack/` → production hits only in `internal/sink/export.go`; test hits only in `internal/sink/export_test.go` and `internal/sink/circuit_breaker_test.go`; plus the stale comment in `kollectclusterinventory_controller.go`
- [ ] 1.2 Delete `RunExportItems` + `ExportItemsRequest` from `internal/sink/export.go`, and `sinkNamespaceForExport` with them (its sole caller is the dead runner — leaving it trips the enabled `unused` linter); reword the stale comment in `kollectclusterinventory_controller.go` so it no longer names the deleted runner
- [ ] 1.3 Delete the `TestRunExportItems_*` tests in `internal/sink/export_test.go` (they exercise only the unreachable path) and migrate the two breaker tests in `internal/sink/circuit_breaker_test.go` (`TestRunExportItems_circuitBreakerTripsAfterRepeatedFailures`, `TestResetBreakersForTest_clearsOpenBreaker`) to drive the live `RunExportEnvelope` path instead, which is where production calls `exportThroughBreaker` (the `exportThroughBreaker` call inside `RunExportEnvelope`); the trip-at-N and reset semantics must be asserted as before. Then record, per deleted `TestRunExportItems_*` test, which live test or suite covers the same behaviour (or that the behaviour is runner-specific) — the coverage accounting goes into the task's evidence file
- [ ] 1.4 `go build ./...` and `go vet ./...` compile clean; `go test ./internal/sink/... ./internal/controller/...` green

## 2. Zero-reference deletions (DR-2, DR-3)

- [ ] 2.1 Probe then delete `MergeRequestAPI` from `internal/sink/gitlab/client.go` (zero refs anywhere, tests included)
- [ ] 2.2 Probe: `grep -rn "ConditionConnected\|ConditionCredentialsVerified" --include='*.go' .` → only `api/v1alpha1/constants.go:10-11`; delete both constants. The deletion commit's body names the removed exported constants (the changelog is commit-derived, no manual CHANGELOG.md edits)
- [ ] 2.3 Compile clean; `go test ./internal/sink/gitlab/... ./api/...` green; grep confirms no docs/CRD text names the constants

## 3. Superseded store methods (DR-4)

- [ ] 3.1 Probe: `Store.RemoveCluster` and `Store.MarshalTargetJSON` referenced only by tests; delete both
- [ ] 3.2 Delete `TestStoreRemoveCluster` and the `RemoveCluster`-based version-monotonicity test (shard deletion is reachable only through the dead method — production `RemoveTarget` never deletes shards); adapt every remaining test that used `MarshalTargetJSON` to inspect envelopes — `engine_extract_failure_test.go` and `TestStoreSubscribeAndMarshal` in `store_test.go` — via `SnapshotTarget` or the envelope the subscriber actually delivers
- [ ] 3.3 Update `store.go` comments that cite `RemoveCluster` (:43, :174, :179) so no dangling name remains; keep the version-monotonicity rationale (it documents `bumpNamespaceVersion` behaviour, not the deleted method)
- [ ] 3.4 Compile clean; `go test ./internal/collect/...` green

## 4. Superseded git entry points (DR-5)

- [ ] 4.1 Probe (record the full list): package-level `git.Export`/`ExportMemory` have zero production callers (`Backend.Export` calls `ExportWithBranch` directly); test callers live in `export_test.go`, `export_integration_test.go` (`//go:build integration`) and `export_forgejo_integration_test.go` (`//go:build integration`)
- [ ] 4.2 Delete `git.Export`; migrate every `Export(` call site in those three test files to `ExportWithBranch` through one test-local helper that replicates the deleted wrapper's commit-context derivation; move the in-memory commit builder (`ExportMemory`) into the test file as an unexported helper — no production file keeps either symbol
- [ ] 4.3 Compile clean; `go test ./internal/sink/git/...` green (unit); the tagged test files compile with their tag on: `go vet -tags integration ./internal/sink/git/`

## 5. Test-only capability aliases (DR-6)

- [ ] 5.1 Probe (record the full reference list, every file and package): the four aliases have zero production references; every test reference found by the probe migrates — expect multiple packages outside `internal/sink/`, each gaining a `cap` import
- [ ] 5.2 Delete the aliases and their four self-tests; migrate every probe-listed test reference to `cap.SnapshotStore()`/`cap.ObjectStoreSnapshot()`/`cap.StreamEmitter()`/`cap.RelationalStore()`
- [ ] 5.3 Compile clean; `go test ./internal/sink/...` green plus every other package whose tests the probe listed

## 6. Superseded engine binding (DR-7)

- [ ] 6.1 Probe: `BindClusterTargetNamespaces` referenced only by tests; record that production binds via `RegisterTarget` synthetic objects (`kollectclustertarget_controller.go` `syncEngineTargets`)
- [ ] 6.2 Delete the method; migrate the probe-listed test sites (controller-package tests and collect-package tests, ~8 files): production binds cluster targets via `RegisterTarget` with a synthetic `KollectTarget` (as `kollectclustertarget_controller.go` `syncEngineTargets` does — profile object plus synthetic object fixture needed), so prefer that shape where the test needs a populated target state; where a test only needs the reader to see a name, the narrowest same-package equivalent is acceptable, recorded in the evidence. Share one fixture helper across the controller tests rather than duplicating per file; delete tests that exist only to exercise the deleted writer
- [ ] 6.3 Compile clean; `go test ./internal/collect/... ./internal/controller/...` green; confirm every `NamespacesForClusterTarget` production call site keeps a test that reaches it

## 7. Auth cache without dead identity (DR-8)

- [ ] 7.1 Delete the `user` field from `authCacheEntry`; `get` returns `(allowed, ok)`, `set` takes `allowed` only; drop the `_ = user` line and the discarded binding in `auth.go`; drop the then-unused import
- [ ] 7.2 Compile clean; `go test ./internal/inventory/...` green, including the cache tests

## 8. Final gates and record (DR-8, DR-9)

- [ ] 8.1 One full suite on the final tree: `task test`; plus `task lint`, `task coverage` (floor 90% holds), `task spec:validate`, `go vet ./...`
- [ ] 8.2 Sweep exclusions recorded in the proposal's Non-goals remain true; record the `EvictBackendPool*` and `AutoMerge` exclusion reasons in the PR description; the commit bodies name every removed exported symbol so the commit-derived changelog records the API changes
- [ ] 8.3 Zero dangling references: the per-symbol probe greps return no hits outside review records

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| DR-1 | probe 1.1 then `go build ./...` after 1.2 | probe shows definition+tests only; build clean with symbols gone | not-run | |
| DR-2 | probe 2.1/2.2 then compile | zero refs; gitlab+api packages compile | not-run | |
| DR-3 | probe 2.2 then compile + `grep` docs/CRDs | no text reference survives | not-run | |
| DR-4 | probe 3.1 then compile + collect suite | store compiles without the methods; suite green | not-run | |
| DR-4b | review: deleted tests guarded no reachable path | recorded reasoning, reviewer-checked | not-run | |
| DR-5 | probe 4.1 then compile + git unit suite + tagged-file compile | pipeline coverage unchanged via `ExportWithBranch` | not-run | |
| DR-6 | probe 5.1 then compile + sink suites | tests use `cap.*` | not-run | |
| DR-7 | probe 6.1 then compile + controller suite; reader call sites still covered | engine compiles without the method; tests still reach `NamespacesForClusterTarget` | not-run | |
| DR-8 | compile + inventory suite | `_ = user` gone; cache tests green | not-run | |
| DR-9 | `task test`, `task lint`, `task coverage`, `task spec:validate` on final tree | all green, floor holds | not-run | |
| DR-10 | re-run per-symbol greps | no hits outside review records | not-run | |
