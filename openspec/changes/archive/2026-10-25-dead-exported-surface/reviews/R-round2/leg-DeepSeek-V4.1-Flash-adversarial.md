## Verdict: CONCERNS

## Findings
- [WARNING] Task 1.3's line anchor `export.go:267` is invalidated by its own preceding task — `internal/sink/export.go:95`
  Failure: task 1.2 deletes `RunExportItems` (lines 95–146); 1.3 then runs and "export.go:267" has shifted up ~50 lines, so the implementer anchors on the wrong call.
  Fix: anchor by symbol (`exportThroughBreaker` inside `RunExportEnvelope`), never a line number a prior step deletes.
  Confidence: 90
- [WARNING] Proposal claims `go build ./...` is the per-symbol proof no caller was missed — `proposal.md:17`
  Failure: `go build` does not compile `_test.go` files, yet most deleted symbols' only callers are tests (`ExportItemsRequest`, the aliases, `RemoveCluster`, `BindClusterTargetNamespaces`). A missed test caller passes the stated proof.
  Fix: state the proof as `go vet ./...` (compiles tests) + tag-on vet; the tasks already do this, the proposal sentence contradicts them.
  Confidence: 88
- [WARNING] Deleting the monotonicity test removes the only guard for the `NamespaceVersion` invariant — `internal/collect/store_test.go:111`
  Failure: `store.go:173-180` documents that the version counter lives outside the shard and is never re-issued "including across RemoveCluster". `store_test.go:44` covers only Upsert/Remove, not shard recreation, so after deletion a refactor moving the counter onto `storeShard` passes all tests.
  Fix: keep a monotonicity test that does not require the dead method, or delete the "across RemoveCluster" clause from the doc comment so the invariant is no longer claimed.
  Confidence: 65
- [WARNING] Removing exported constants from the importable `api/v1alpha1` is an external API break no in-repo check can verify — `api/v1alpha1/constants.go:10-11`
  Failure: a downstream consumer of `github.com/platformrelay/kollect/api/v1alpha1` referencing `ConditionConnected` fails to compile; the proposal's own assumption (`proposal.md:96-99`) admits the compile cannot prove the external half.
  Fix: deprecate-in-place (keep with `// Deprecated:`) or grep the module cache / dependents before deleting; record the check as evidence.
  Confidence: 55
- [NOTE] Task 6.2 understates the migration cost — `internal/collect/engine.go:406` vs `internal/controller/kollectclustertarget_controller.go:230`
  Failure: `BindClusterTargetNamespaces` writes a bare `targetState` (no profile/opts); `RegisterTarget` needs a profile and synthetic object. Controller tests cannot reach a collect-internal equivalent, so all ~14 sites need real profile fixtures, not a mechanical rename.
  Fix: name the fixture construction in 6.2, or keep a collect-internal test-only seeding helper behind the reader paths.
  Confidence: 60
- [NOTE] Proposal's "6 production references" to `RunExportEnvelope` counts comments and the `RunExportItems` body this change deletes — `proposal.md:91`
  Failure: the true post-change production call sites are 3 (`cleanup.go:364`, `kollectinventory_controller.go:404`, `kollectclusterinventory_controller.go:331`); the inflated count overstates retained coverage.
  Fix: restate as call sites, excluding comments and the deleted wrapper.
  Confidence: 85
- [NOTE] Impact list omits the alias-migration test files — `proposal.md:56-60`
  Failure: `internal/pipeline/wire_test.go:34` and ~8 `internal/controller/*_test.go` files gain a `cap` import and change; §Impact names controller only for the comment/engine work, so the blast radius reads smaller than it is (task 5.1 does anticipate it).
  Fix: add the alias-migration files to §Impact.
  Confidence: 80

Verified as sound (checked, not rubber-stamped): every deleted symbol really has zero production callers at HEAD (`RunExportItems`/`ExportItemsRequest` only in `export.go` + two test files; `MergeRequestAPI`, both conditions, `RemoveCluster`, `MarshalTargetJSON`, `git.Export`/`ExportMemory`, the four aliases, `BindClusterTargetNamespaces` — all test/comment-only); `delete(s.shards…)` occurs only in `RemoveCluster` (`store.go:212`), so the shard-recreation trap is genuinely production-unreachable; no `//go:build`-tagged file anywhere references any deleted symbol, and `git.Export`'s tagged callers are exactly the three files task 4.2 lists; `cap` sits inside the `sink` arch-lint component, so `pipeline`/`controller` importing it is allowed; the four alias self-tests are backed by `internal/sink/cap/capabilities_test.go`, so deleting them does not orphan those constructors; the breaker tests already run through `RunExportEnvelope`→`exportThroughBreaker` (`export.go:267`), so 1.3's migration is feasible.

## Could not check
- The round-1/round-2 peer reviews under `reviews/` (peer output, out of bounds) and the external consolidating report `data/kollect-xconsol-final/report.md` (outside the repo).
- No build, vet, test or coverage run — line-shift and coverage claims are reasoned from source, not executed.
- External module consumers of `api/v1alpha1` (not resolvable from this checkout).
