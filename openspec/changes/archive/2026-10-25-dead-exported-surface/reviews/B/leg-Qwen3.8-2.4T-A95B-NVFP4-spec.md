All deleted symbols verified absent from live Go code. Now checking non-goals remain and the migrations.
## Verdict: CLEAN

## Findings

- [NOTE] `RelationalStore()` lost its only direct struct pin; the deferred cap-package ratchet does not exist in this tree — `internal/sink/cap/capabilities_test.go:8`
  Failure: the deleted `TestRelationalStoreCapabilities` asserted `SupportsDelete==true` and no stray flags; `TestCapabilityConstructors` pins the other three constructors but not `RelationalStore()`. If someone flips `SupportsDelete` to false (`internal/sink/cap/capabilities.go:41`), relational stale-row pruning (`internal/sink/cleanup.go:197`) silently degrades. Mitigation verified: the semantics are still reached behaviourally via `TestExportPayload` relational cases (`capabilities.go:53` branch) and backend pins (`internal/sink/postgres/capabilities_test.go:20`, bigquery:20, mongodb:31); deferral is recorded in loop.md T5.
  Fix: add `RelationalStore()` to the equality table in `TestCapabilityConstructors` (one line) when the ratchet lands.
  Confidence: 85 that the pin gap is real; 30 that it matters before the ratchet task runs.

- [NOTE] Shadowing binding left by construction: helper's `cancel` shadows package-level `cancel` — `internal/controller/kollectclusterinventory_helpers_test.go:37` (outer at `suite_test.go:40`)
  Failure: none observable — govet shadow does not flag it; task 9.1's authorised scope was exactly the two flagged findings, and T9 evidence records the observation and the decision to leave it. Listed only so it is not mistaken for an oversight.
  Fix: none required; rename alongside `ctx` if a future lint run flags it.
  Confidence: 95 that this is deliberate and recorded (evidence/T9.md register note).

Spec verification performed (each item code-checked, not document-trusted):
- All nine deletions hold with zero live references (repo-wide grep; hits only in change records): `RunExportItems`/`ExportItemsRequest`/`sinkNamespaceForExport` (`internal/sink/export.go`), `MergeRequestAPI` (`internal/sink/gitlab/client.go`), both condition constants (`api/v1alpha1/constants.go`), `Store.RemoveCluster`/`MarshalTargetJSON` (`internal/collect/store.go`), `git.Export`/`git.ExportMemory` (`internal/sink/git/export.go`), four capability aliases (`internal/sink/export.go`), `Engine.BindClusterTargetNamespaces` (`internal/collect/engine.go`), cache `user` field (`internal/inventory/auth_cache.go`, `_ = user` gone from `auth.go`; `user` still legitimately used at `auth.go:127,137`).
- Non-goals intact (would-be over-deletion checked): `EvictBackendPool*`/`AutoMerge` (38 refs), `layout.VerifySet` (`internal/sink/layout/manifest.go:129`), `s3.NewBackendWithClient` (`internal/sink/s3/backend.go:55`), probe.go wrappers (9 in `internal/sink/probe.go`), `Capabilities` alias and `RESTClient` kept.
- Migrations as specified: breaker pair drives `RunExportEnvelope` with trip-at-N/reset intact (`internal/sink/circuit_breaker_test.go:20,108`); `exportForTest` (`internal/sink/git/export_test.go:426`) replicates the deleted wrapper's commit-context derivation byte-for-byte; `exportMemory` unexported at :435; integration-tagged files re-pointed (forgejo:48,98,130; integration:37); shared production-shaped helper `newEngineWithBoundClusterTargets` with `engine.Start(ctx)` (`kollectclusterinventory_helpers_test.go:26`); all 4 `NamespacesForClusterTarget` reader sites keep reaching tests; stale comment reworded (`kollectclusterinventory_controller.go:278`); store.go comments updated, monotonicity rationale kept (`store.go:43,174`).
- Proposal claim verified: exactly 3 production `RunExportEnvelope` call sites (`cleanup.go:364`, `kollectinventory_controller.go:404`, `kollectclusterinventory_controller.go:331`).
- Stronger-than-spec check: production diffs are deletion-only + comment reword; `git diff --name-only` outside `*.go`/`openspec/` is empty — no lint config, allow-list, CI, chart or Taskfile change; nothing deleted beyond the Sweep-2 list.
- Task 8.2: all 7 deletion commit bodies name the removed exported symbols; `04a16ff1` carries `refactor(api)!:` + `BREAKING CHANGE:` footer. Task 9: both shadow fixes land exactly where specified.

## Could not check

- Gate re-execution (`task test`, `task lint`, `task coverage` 91.3%, `task spec:validate`, `go vet -tags integration`): read-only review session, envtest suite is ~8 min; relied on evidence/T8.md + evidence/T9.md records, which I read but did not reproduce.
- The source report `data/kollect-xconsol-final/report.md` (lives outside this repo per loop.md); the proposal quotes its Sweep 2 excerpt verbatim, which I checked against the implemented list.
- External consumers of `api/v1alpha1` (recorded pre-1.0 assumption in proposal.md:108).
- Per-deleted-test coverage accounting (evidence/T1.md) exists and is specific; I verified its structure, not its claims by re-running the suites.
