I'll review this branch adversarially. Let me start by loading the skill and reading the target.
Let me read the tasks, loop record, and the actual code diff.
## Verdict: CLEAN

## Findings

- [NOTE] Test helper claims to mirror production but omits three production inputs — `internal/controller/kollectclusterinventory_helpers_test.go:29`
  Failure: `newEngineWithBoundClusterTargets`'s doc says it mirrors `syncEngineTargets`, but it omits `ScopeCeiling`, `CollectionFilterSpec` and the `RefreshNamespaces` call that `kollectclustertarget_controller.go:206-232` makes. A future maintainer extending these tests to cover collection (not just the reader/rollup path they currently seed by hand) will trust the fixture as production-shaped when it is not, and a namespace-filtering regression would not surface. Current tests are unaffected because they seed `store` rows directly and only read `NamespacesForClusterTarget`.
  Fix: one clause in the comment naming the three omitted inputs, or add them to the helper.
  Confidence: 70 (divergence is certain; impact is latent, not present).

- [NOTE] Removal of exported `api/v1alpha1` constants is an external API break — `api/v1alpha1/constants.go:10-11`
  Failure: `ConditionConnected`/`ConditionCredentialsVerified` are deleted from an importable package; any out-of-repo consumer fails to compile. Unverifiable in-repo.
  Fix: none required if pre-1.0 policy holds; the `refactor(api)!` + `BREAKING CHANGE:` footer and proposal assumption already record it. Do not pretend the compile proves the external half.
  Confidence: 90 that the break exists; 20 that it matters given zero in-repo refs and no status write.

- [NOTE] Coverage parity reported to one decimal can mask a sub-0.05% drop — `openspec/.../evidence/T8.md:18`
  Failure: 91.3% → 91.3% is consistent with a small real drop hidden by rounding; the floor (90) is the only hard guard.
  Fix: none — floor passes; record the raw float if parity is used as evidence.
  Confidence: 60.

## What each persona tried, and what failed to land

- **Saboteur**: checked for a second shard-deletion path that would make the rejected version-monotonicity guard still reachable — none (`store.go` has no `delete(s.shards, …)` after `RemoveCluster` removal; versions map never pruned). Checked the auth-cache key covers everything the dropped `user` field contributed — key includes token hash, verb, namespace, name, resource; `allowed` is the only output (`auth.go:113-145`), so identity was never load-bearing. Checked tagged test files (`export_integration_test.go`, `export_forgejo_integration_test.go`) were migrated, not skipped — all 10 `Export(` sites and 3 `ExportMemory` sites re-pointed to test-local helpers. Checked concurrent breaker tests — the `ResetBreakersForTest` parallel flake is pre-existing, recorded, out of scope.
- **New Hire**: verified the reworded controller comment no longer names the deleted runner and still reads correctly; verified `store.go` rationale comments no longer cite `RemoveCluster` and the monotonicity rationale is retained.
- **Security Auditor**: no untrusted input reaches a new sink; no secret touched; the only security-relevant surface is the auth cache, which is behaviour-preserving and still covered by `TestAuthMiddlewareCacheHitsTokenReviewOnce`.
- **Budget Holder**: the change is pure subtraction (846 deletions vs 305 additions, all additions in tests/records); no dependency, config knob or abstraction added; no allow-list, exclusion or coverage baseline widened (diff touches no `.yml`/`.toml`/`Taskfile`/`go.mod`).

Repo-wide `git grep` for all 14 removed identifiers returns zero live references outside review records and the three `TestExportMemory*` name-only lines. Every live test cited in T1's coverage-accounting table exists and asserts what is claimed.

## Could not check
- Did not run `go build`/`go vet`/`task test`/`task coverage`/`task lint` (read-only review); compile-clean and gate-green claims rest on the recorded evidence files, not on my own execution.
- Did not read all 9 evidence files in full, nor the full review registers under `reviews/`.
- External consumers of `api/v1alpha1` are unverifiable in-repo; the sibling `data/kollect-xconsol-final/report.md` is outside this repository and was not read.
