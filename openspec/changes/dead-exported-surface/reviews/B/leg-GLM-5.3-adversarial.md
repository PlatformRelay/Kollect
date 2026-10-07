Spec read. Now the full code diff (excluding the openspec records) and the tasks/loop records.
Now the core code diffs, in parallel batches.
Deletions look faithful so far. Now adversarial verification: dangling references to every deleted symbol across the whole tree, plus the remaining context files.
Records and greps line up. Now the deeper adversarial checks: the migrated breaker tests' live path, the auth middleware after `user` removal, shard-deletion comment truthfulness, and the gitlab interface's concrete method.
T8's record is solid — coverage measured on both trees. Final check: T9's fix and green gates, then one last scan for tagged files.
## Verdict: CLEAN

Checked: full code diff `3ee21266..HEAD` (37 code files + records), spec set, tasks.md, loop.md, evidence T1/T8/T9; tree-wide greps for every deleted symbol (textual probes cover `//go:build integration` files too, since greps are tag-blind); the migrated breaker path re-derived in `export.go`; the auth middleware re-read; all 10 live tests cited in T1's coverage accounting grep-verified to exist. No CRITICAL or WARNING survived attack-defend-revise; note-only findings below.

## Findings

- [NOTE] Shard-recreation version-monotonicity is now guarded by comments only — `internal/collect/store.go:43`
  Failure: if a future change re-introduces shard deletion (the deleted `RemoveCluster` pattern), a recreated shard could re-issue a version a stale cache entry holds; the white-box test guarding this was deleted. Attack failed today: `delete(s.shards` has zero hits, so the scenario is unreachable — and the rejection is recorded twice (loop.md R1#7, R2#9).
  Fix: none now; if shard deletion ever returns, re-add the monotonicity test (git history preserves `store_cluster_test.go`).
  Confidence: 85 (real but conditional risk, disposition already recorded)
- [NOTE] Breaker tests still trip a process-global breaker registry under `t.Parallel()` — `internal/sink/circuit_breaker_test.go:25,94`
  Failure: parallel scheduling can clear one test's breaker mid-trip → flake. Pre-existing shape (old tests identical); recorded in T1's review and loop.md lessons; not introduced by this branch.
  Fix: scoped reset or serial flag (already proposed to the owner).
  Confidence: 90
- [NOTE] Three test names keep the deleted identifier's spelling (`TestExportMemory*`) — `internal/sink/git/export_test.go:23,36,48`
  Failure: a future zero-reference grep for `ExportMemory` hits these name lines and must be re-triaged (T8 rows 22/146 dispositioned it twice).
  Fix: optional rename (e.g. `TestMemoryCommitHelper*`).
  Confidence: 95
- [NOTE] Duplicated "B branch review" stage row (running + pending) — `openspec/changes/dead-exported-surface/loop.md:18-19`
  Failure: reader cannot tell B's state.
  Fix: drop the pending row when this review lands.
  Confidence: 100

Spec-lens: every "What Changes" bullet HOLDS against code — `RunExportItems`/`ExportItemsRequest`/`sinkNamespaceForExport`, `MergeRequestAPI`, both condition constants, `Store.RemoveCluster`/`MarshalTargetJSON`, `git.Export`/`ExportMemory` (test helpers `exportForTest`/`exportMemory` are body-identical to the deleted wrappers), the four capability aliases, `BindClusterTargetNamespaces`, and the auth-cache `user` field: zero live references tree-wide; reworded comments truth-checked (e.g. "no current mutation path deletes one" — verified); exclusions (`EvictBackendPool*`, `AutoMerge`, `layout.VerifySet`) intact. Stronger-than-spec: `sinkNamespaceForExport` deletion — unexported, sole-caller rule, recorded in task 1.2 and commit body; harmless. Unmentioned-but-recorded work: store.go comment rewrites (3.3), shared `RegisterTarget` fixture (6.2), T9 lint fix (authorised micro-task). Personas: Saboteur re-derived the breaker key (`export.go:186`, ns/name; `SinkUID` was empty in the old path too) and confirmed `checkInventoryIdentity` admits zero identity (`export.go:209`); New Hire truth-checked comments and the fixture's `Start`/`informerContext` subtlety; Security Auditor verified the auth decision flow is unchanged (`authenticate` still runs on miss, `auth.go:127`; cache key unchanged; no secrets, logs, or deps touched); Budget Holder found the code delta is −809/+268 with records as the harness convention and no invention.

## Could not check

- Did not run `go build`/`go vet`/`task test`/`task lint`/`task coverage` (read-only constraint): gates stand on the recorded exit codes with sha+seconds (T8/T9) plus my own tree-wide identifier greps, the strongest static equivalent.
- External module consumers of `api/v1alpha1` (constants deletion) — unverifiable in-repo; the change's own recorded accepted assumption (R1#5), mitigated by the `refactor(api)!:` + `BREAKING CHANGE:` commit body (verified).
- The out-of-repo source report `data/kollect-xconsol-final/report.md` — out of bounds; the proposal quotes its Sweep 2 excerpt.
- CHANGELOG rendering of the breaking commit — not run (loop.md records that local cliff runs overwrite `CHANGELOG.md`).
- Integration/e2e gates — Docker absent on this host (T8 recorded); not part of the 8.1 gate set.
