Now let me look at the round-2 revision commit and the project conventions.
The auth-cache claims check out so far. Now let me verify the zero-caller probe claims for the other symbols.
Probes largely confirm the spec. Now verifying the deeper claims: `git.Export`/`Backend.Export`, `RemoveTarget` shard behaviour, and the production `RegisterTarget` binding path.
Now checking the remaining cross-references: `git.Export` callers repo-wide, the `cap` package import path, `exportThroughBreaker`, and the gitlab client.
A few remaining verifications: build-tagged files among probe hits (round 1's miss class), the only-shard-deleter claim, and the engine reader paths.
One more check — whether the pool "no-close-per-export / close-on-evict" property asserted by the deleted `TestRunExportItems_poolsBackendUntilEvict` is covered elsewhere on the live path.
## Verdict: CONCERNS

Security lens over the round-2 spec set (proposal.md + tasks.md at 92360d72). I re-ran every zero-caller claim against HEAD rather than trusting the text: `RunExportItems`/`ExportItemsRequest` (probe accurate, incl. the stale comment at kollectclusterinventory_controller.go:278), `MergeRequestAPI` (client.go:25-26, zero refs), `ConditionConnected`/`ConditionCredentialsVerified` (constants.go:10-11, zero Go and zero non-Go text refs), `RemoveCluster`/`MarshalTargetJSON` (test-only; `RemoveTarget` confirmed shard-preserving at store.go:194-203, and store.go:212 is the only shard deleter), package-level `git.Export`/`ExportMemory` (backend.go:60 calls `ExportWithBranch` directly; test callers are exactly the three files tasks.md names, two of them tagged), the four `*Capabilities` aliases (all refs are tests in internal/sink, internal/controller, internal/pipeline), and `BindClusterTargetNamespaces` (14 test sites in 8 files — count matches; production binding is `RegisterTarget` at kollectclustertarget_controller.go:230). DR-8 verified at the auth boundary: auth.go:115-122 discards the cached `user` (`_ = user`), the cache key stays per-token-hash (auth.go:113), so dropping the field is behaviour-neutral and strictly reduces retained UserInfo; SAR still receives a fresh `user` (auth.go:138). No new dependencies, no secret/log surface touched, traversal guards survive (`TestExportMemory_rejectsTraversal` moves with the helper; `TestExportWithBranch_rejectsMaliciousObjectPath` untouched). The round-2 fixes (breaker-test migration, `export_integration_test.go`, external-consumer assumption) all landed coherently.

## Findings

- [WARNING] Task 1.3's migration list is incomplete: two deleted `TestRunExportItems_*` tests assert reachable pool behaviour, contradicting the assumption "Every other deleted test exercises only the unreachable `RunExportItems` path" — `openspec/changes/dead-exported-surface/tasks.md:14`
  Failure: `TestRunExportItems_poolsBackendUntilEvict` (export_test.go:306-348) is the only test asserting a pooled backend is NOT closed after a completed export (no other `.closed`-after-export assertion exists — verified via grep across sink/controller tests), and `TestRunExportItems_closeErrorDoesNotFailExport` (export_test.go:263-304) is the only test that an erroring-Close backend still exports successfully through the pipeline. Deleting them lets a future `acquireBackend` regression that closes pooled backends per export pass CI.
  Fix: extend task 1.3 to migrate these two tests to the live `RunExportEnvelope` path (same pattern as the breaker tests), or record in the assumptions why backend_pool_test.go's evict/race tests are deemed sufficient.
  Confidence: 85
- [NOTE] The Why and Non-goals cite `data/kollect-xconsol-final/report.md` (§4, §7), which does not exist in this repository — `openspec/changes/dead-exported-surface/proposal.md:5`
  Failure: implementers and CI cannot follow the citation to the sweep's source of truth; only the in-repo probes make it non-load-bearing.
  Fix: replace the report citation with the recorded probe evidence (tasks.md already carries it) or move the report into the repo.
  Confidence: 100
- [NOTE] The assumption "production dispatches through `RunExportEnvelope` (6 production references)" does not reproduce under any clean counting — `openspec/changes/dead-exported-surface/proposal.md:92`
  Failure: after the deletion, 3 production call sites remain (internal/sink/cleanup.go:364, internal/controller/kollectclusterinventory_controller.go:331, internal/controller/kollectinventory_controller.go:404) plus definition/comments; "6" matches no grep I ran.
  Fix: restate as "3 production call sites: cleanup + two controllers".
  Confidence: 90

## Could not check

- The round-1/round-2 review records under `reviews/R/` and `reviews/R-round2/` — deliberately unread to keep this leg independent.
- External (out-of-module) consumers of `api/v1alpha1` — the accepted limit the proposal itself records; in-repo I verified zero references.
- `task coverage` / `task test` / `task spec:validate` outcomes — read-only session, nothing executed.
- The content of the cited consolidated report — path does not exist inside this repository.
