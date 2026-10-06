# Tasks

Every task: run its zero-caller probe first and record it, delete, then prove with
`go build ./...` and `go vet ./...` plus the affected package suite. No behavioural red exists
for this change (unreachable surface); the probe + compile pair is the per-task evidence.

## 1. Dead sink runner (DR-1)

- [ ] 1.1 Probe: `grep -rn "RunExportItems\|ExportItemsRequest" --include='*.go' internal/ cmd/ api/ test/ hack/` shows only `internal/sink/export.go`, `internal/sink/export_test.go` and the stale comment in `kollectclusterinventory_controller.go`
- [ ] 1.2 Delete `RunExportItems` + `ExportItemsRequest` from `internal/sink/export.go`; delete the `TestRunExportItems_*` tests (they exercise only the unreachable path); reword the stale comment in `kollectclusterinventory_controller.go` so it no longer names the deleted runner
- [ ] 1.3 `go build ./...` and `go vet ./...` compile clean; `go test ./internal/sink/... ./internal/controller/...` green

## 2. Zero-reference deletions (DR-2, DR-3)

- [ ] 2.1 Probe then delete `MergeRequestAPI` from `internal/sink/gitlab/client.go` (zero refs anywhere, tests included)
- [ ] 2.2 Probe: `grep -rn "ConditionConnected\|ConditionCredentialsVerified" --include='*.go' .` → only `api/v1alpha1/constants.go:10-11`; delete both constants
- [ ] 2.3 Compile clean; `go test ./internal/sink/gitlab/... ./api/...` green; grep confirms no docs/CRD text names the constants

## 3. Superseded store methods (DR-4)

- [ ] 3.1 Probe: `Store.RemoveCluster` and `Store.MarshalTargetJSON` referenced only by tests; delete both
- [ ] 3.2 Delete `TestStoreRemoveCluster` and the `RemoveCluster`-based version-monotonicity test (shard deletion is reachable only through the dead method — production `RemoveTarget` never deletes shards); adapt `engine_extract_failure_test.go` where it used `MarshalTargetJSON` to inspect envelopes
- [ ] 3.3 Update `store.go` comments that cite `RemoveCluster` (:43, :174, :179) so no dangling name remains
- [ ] 3.4 Compile clean; `go test ./internal/collect/...` green

## 4. Superseded git entry points (DR-5)

- [ ] 4.1 Probe: package-level `git.Export`/`ExportMemory` have zero production callers (`Backend.Export` calls `ExportWithBranch` directly)
- [ ] 4.2 Delete `git.Export`; migrate `export_test.go` and `export_forgejo_integration_test.go` call sites to `ExportWithBranch` through one test-local helper that replicates the deleted wrapper's commit-context derivation; move the in-memory commit builder (`ExportMemory`) into the test file as an unexported helper — no production file keeps either symbol
- [ ] 4.3 Compile clean; `go test ./internal/sink/git/...` green (unit); the integration-tagged forgejo suite compiles (`go vet -tags integration ./internal/sink/git/` or build the tagged files)

## 5. Test-only capability aliases (DR-6)

- [ ] 5.1 Probe: the four aliases have zero production references; tests reference them across ~19 sites
- [ ] 5.2 Delete the aliases and their four self-tests; migrate test references to `cap.SnapshotStore()`/`cap.ObjectStoreSnapshot()`/`cap.StreamEmitter()`/`cap.RelationalStore()`
- [ ] 5.3 Compile clean; `go test ./internal/sink/...` green

## 6. Superseded engine binding (DR-7)

- [ ] 6.1 Probe: `BindClusterTargetNamespaces` referenced only by tests; record that production binds via `RegisterTarget` synthetic objects (`kollectclustertarget_controller.go` `syncEngineTargets`)
- [ ] 6.2 Delete the method; migrate the ~14 test sites in 8 files: seed through the production-shaped `RegisterTarget` path (or the narrowest same-package equivalent) so the `NamespacesForClusterTarget` reader paths stay covered; delete tests that exist only to exercise the deleted writer
- [ ] 6.3 Compile clean; `go test ./internal/collect/... ./internal/controller/...` green; confirm every `NamespacesForClusterTarget` production call site keeps a test that reaches it

## 7. Auth cache without dead identity (DR-8)

- [ ] 7.1 Delete the `user` field from `authCacheEntry`; `get` returns `(allowed, ok)`, `set` takes `allowed` only; drop the `_ = user` line and the discarded binding in `auth.go`; drop the then-unused import
- [ ] 7.2 Compile clean; `go test ./internal/inventory/...` green, including the cache tests

## 8. Final gates and record (DR-8, DR-9)

- [ ] 8.1 One full suite on the final tree: `task test`; plus `task lint`, `task coverage` (floor 90% holds), `task spec:validate`, `go vet ./...`
- [ ] 8.2 Sweep exclusions recorded in the proposal's Non-goals remain true; record the `EvictBackendPool*` and `AutoMerge` exclusion reasons in the PR description
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
