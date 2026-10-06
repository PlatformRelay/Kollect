## Verdict: BLOCK

## Findings
- [CRITICAL] Probe 1.1 is false at this HEAD and task 1.2's deletion glob silently removes the only behavioural test of the live circuit breaker — `openspec/changes/dead-exported-surface/tasks.md:9-10` vs `internal/sink/circuit_breaker_test.go:20,77`
  Failure: `internal/sink/circuit_breaker_test.go` calls `RunExportItems`/`ExportItemsRequest` (lines 51–68, 108–128), yet probe 1.1 expects hits only in `export.go`/`export_test.go`/the controller comment, and proposal.md:9 claims "re-verified at this HEAD". `TestRunExportItems_circuitBreakerTripsAfterRepeatedFailures` matches the `TestRunExportItems_*` deletion glob but is the only test of `exportThroughBreaker`, which wraps every production export (`internal/sink/export.go:267` in `RunExportEnvelope`) — sink-hammer/repeated-credential-failure isolation. Deleting it compiles and passes every listed gate (90% floor won't move: breaker code stays covered by other `RunExportEnvelope` suites), so coverage of a fault-isolation control is lost silently. `TestResetBreakersForTest_clearsOpenBreaker` (:77) doesn't match the glob and will break the build instead.
  Fix: add a subtask migrating both circuit-breaker tests' call sites to `RunExportEnvelope` directly (same breaker path, `SinkSpec` set), and correct probe 1.1's expected output.
  Confidence: 95
- [WARNING] Task 4.2's migration list misses a tagged caller; the "compile is the backstop" claim does not hold for build-tagged files — `openspec/changes/dead-exported-surface/tasks.md:29` vs `internal/sink/git/export_integration_test.go:1,37`
  Failure: `export_integration_test.go` (`//go:build integration`) calls package-level `Export`; `go build ./...`/`go vet ./...` skip tagged files, so proposal.md:17's per-symbol compile proof is void here. Task 4.3's `go vet -tags integration` catches it, so it cannot ship broken, but the task list understates the work.
  Fix: name `export_integration_test.go` in 4.2.
  Confidence: 90
- [NOTE] DR-3 is the only deletion on the importable public surface; everything else is under `internal/` — `api/v1alpha1/constants.go:10-11`
  Failure: `api/v1alpha1` is outside `internal/`, so an external consumer importing the module breaks on `ConditionConnected`/`ConditionCredentialsVerified` removal; no in-repo, CRD, docs or yaml reference exists (verified by repo-wide `*.go`/`*.yaml`/`*.md` grep), so risk is external-only and low.
  Fix: note external-consumer impact in the PR description alongside the AutoMerge exclusion.
  Confidence: 70

## Checked (security pass)
- DR-8 auth cache deletion is safe and slightly positive: the cached decision is bound to the caller's token hash in the key (`internal/inventory/auth_cache.go:66-73`, `auth.go:112-113`), so no decision can leak across identities by dropping `user` (`auth_cache.go:16`), which was only ever `_ = user` on hit (`auth.go:122`); after the change no TokenReview identity (username/UID/groups) is retained in process memory for the 30 s TTL (`cmd/startup_flags.go:128`). The 30 s post-RBAC-change decision staleness is pre-existing and not widened.
- `bearerToken` errors echoed to clients (`auth.go:106`) leak only header-shape messages (`bearer.go:12-25`), no token material — unchanged by the spec.
- Path-traversal evidence survives DR-5: `../` rejection on the live path is tested independently of `ExportMemory` (`validate_test.go:390,425` via `ExportWithBranch`/`ExportFilesWithBranch` → `validateObjectPath`, `validate.go:16,75`); moving `ExportMemory` (`export.go:587-623`) to a test helper loses nothing if it keeps calling `validateObjectPath`.
- Zero-caller claims re-verified by grep for `MergeRequestAPI` (def only, `gitlab/client.go:25-26`), the two condition constants, the four capability aliases (zero non-test refs), `RemoveCluster`/`MarshalTargetJSON` (a one-line wrapper over `MarshalTargetExport`, `store.go:249-252`), and `BindClusterTargetNamespaces` (def only; the reader `NamespacesForClusterTarget` has 3 production call sites, `kollectclustertarget_controller.go:254`, `kollectclusterinventory_controller.go:392,426,654` — task 6.2's "reader stays covered" requirement is the right guard).
- `RemoveCluster` deletes in-memory shards only (`store.go:205-217`) — no on-disk retention implication.
- No new dependencies, no secrets in the spec files, no CRD/RBAC/chart touch.

## Could not check
- Did not run `go build`/`go vet -tags integration`/`task coverage` (read-only run); all greps are static at this HEAD.
- Did not verify the referenced review record `data/kollect-xconsol-final/report.md` §4 exists or matches the item list.
- Did not check external importers of `api/v1alpha1` (no network/module-graph lookup).
