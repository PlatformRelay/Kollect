Spec set under review: a deletion proposal for dead exported symbols. The load-bearing premise is "every listed symbol is dead at this HEAD" — I'll verify that against the actual code, not the spec's own prose. Running the probes myself.
Dead-runner premise holds so far, but note `circuit_breaker_test.go:20` — a circuit-breaker test reaching production state through `RunExportItems`. Checking whether the breaker has any other route to coverage, plus the remaining symbols.
`circuit_breaker_test.go` also names `TestRunExportItems_*` — the probe's stated expected result (tasks.md:9) misses it, and the breaker may be live state. Reading the sink export code and breaker wiring.
Breaker is live state: `RunExportEnvelope` → `exportThroughBreaker` (export.go:267), and the only trip-assertions live in `circuit_breaker_test.go` via `RunExportItems` — which task 1.2 deletes wholesale and the probe expectation omits. Now verifying the store claims, the `cap` package's own tests, git entry points, and the auth cache.
The consolidated report the proposal cites (`data/kollect-xconsol-final/report.md`) doesn't exist in this repo — noted. The `cap` package has its own test file; checking whether it already covers the four constructors' invariants, and verifying the remaining claims.
`cap` tests cover three constructors but the `RelationalStore()`/`SupportsDelete` invariant may live only in the alias self-test. Checking that, plus the auth cache and engine binding paths.
`SupportsDelete` is transitively covered (postgres/mongodb/bigquery backend tests assert `cap.RelationalStore()`), so DR-6's self-test deletion is safe. Auth-cache premise holds (`authorizeResource` runs pre-cache; only `allowed` is needed). Final sweep: repo conventions, task targets, integration test tags, and the `RunExportEnvelope` caller count.
One parallel-review record surfaced in a grep (`reviews/R/leg-*.md` inside the change dir) — I won't open or use it; my findings stand on the code I read myself. Final checks: the Taskfile gates the change claims, and the forgejo integration test's calls to the deleted wrapper.
Last verification batch before writing the report: DR-7's production-binding claim, `layout.VerifySet` existence, and whether the deleted constants are named anywhere outside Go.
## Verdict: BLOCK

Load-bearing premise checked on the real tree, not the spec's prose: every listed symbol is production-dead at this HEAD (verified by repo-wide greps: `RunExportItems`/`ExportItemsRequest` defined only at `internal/sink/export.go:58-146` with test/comment callers; `MergeRequestAPI` zero refs; constants only `api/v1alpha1/constants.go:10-11`, no docs/CRD/chart hits; `RemoveCluster`/`MarshalTargetJSON` test-only; `git.Export`/`git.ExportMemory` zero callers; aliases test-only; `BindClusterTargetNamespaces` test-only with production binding at `kollectclustertarget_controller.go:193→230`; `_ = user` at `auth.go:122` safe because SAR runs pre-cache, `auth.go:138,146`). Non-goals verified: `EvictBackendPool` ~30 test sites, `AutoMerge` at `kollectsink_types.go:253-255`, `layout.VerifySet` real (`layout/manifest.go:129`). Cap-alias self-tests are redundant (cap's own tests + postgres/mongodb/bigquery assert `SupportsDelete`). Gates exist (`Taskfile.yml:13` floor 90). One requirement's justification is contradicted by the code, one gate as written cannot go green.

## Findings

- [CRITICAL] Task 1.2 deletes the only tests exercising the live circuit breaker, and as written cannot compile — `openspec/changes/dead-exported-surface/tasks.md:10`
  Failure: `TestResetBreakersForTest_clearsOpenBreaker` (`internal/sink/circuit_breaker_test.go:77`) calls `RunExportItems` (:118,128) but doesn't match the `TestRunExportItems_*` deletion glob, so package compile fails and gate 1.3 is unreachable; if deleted wholesale instead, the only trip-at-5/reset assertions of `exportThroughBreaker` — live production state, called from `RunExportEnvelope` (`internal/sink/export.go:267`) — vanish while every gate stays green. Proposal assumption "deleted tests exercise only the unreachable `RunExportItems` path" (`proposal.md:85-87`) is false for this file, and probe 1.1's stated expected output (`tasks.md:9`) omits it.
  Fix: task 1.2 migrates both breaker tests to drive `RunExportEnvelope` (or `exportThroughBreaker`) instead of deleting; correct task 1.1's expected file list and the Assumption sentence.
  Confidence: 90
- [WARNING] DR-4 deletes the only guard of the version-monotonicity invariant and task 3.3 rewords away its recorded rationale — `openspec/changes/dead-exported-surface/tasks.md:22-23`
  Failure: `RemoveTarget` never deletes shards (`internal/collect/store.go:194-203`), so the premise holds today; but `TestStoreNamespaceVersion_MonotonicAcrossRemoveClusterAndShardRecreation` (`store_test.go:103-149`) guards the invariant itself, not the entry point, and the rationale lives in `store.go:42-47`. Test deleted + comments reworded → a future change reintroducing shard deletion (or moving the counter onto the shard) resets namespace versions with no failing test: a reused version makes a stale cache entry hit.
  Fix: keep the test, driving shard deletion in-package (`delete(s.shards, ns)` — same package), or require task 3.3 to preserve the invariant rationale in prose even after the name goes.
  Confidence: 70
- [NOTE] Task enumerations miss test call sites the deletions force adapting — `openspec/changes/dead-exported-surface/tasks.md:22,29`
  Failure: `store_test.go:252` uses `MarshalTargetJSON` (task 3.2 names only `engine_extract_failure_test.go`); `export_integration_test.go:37` (tagged integration) calls `git.Export` (task 4.2 names only `export_test.go` and the forgejo suite). An implementer following the listed files literally hits broken builds outside them.
  Fix: name both files in the tasks (the compile gates would catch it, but tasks.md is the record).
  Confidence: 95
- [NOTE] Task 8.3's zero-dangling-refs gate hits immutable history and cannot pass as written — `openspec/changes/dead-exported-surface/tasks.md:53`
  Failure: `CHANGELOG.md:1249` names `RunExportItems` (git-cliff output) and `openspec/changes/archive/2026-10-05-inventory-export-identity/{design.md:23,tasks.md:27,proposal.md:81-82}` name `RunExportItems`/`ExportItemsRequest`; neither is a review record, so the gate is ungreenable without improvising exclusions.
  Fix: define the allowed-hit set in 8.3 (CHANGELOG, openspec archive, reviews).
  Confidence: 95
- [NOTE] Primary evidence citation is unresolvable in-repo — `openspec/changes/dead-exported-surface/proposal.md:5`
  Failure: `data/kollect-xconsol-final/report.md` (cited for Sweep 2 and §7 exclusions) does not exist under that path in this repo (glob empty); a reviewer cannot check the cited source or the §7 rationale it anchors.
  Fix: inline the cited sections into the change dir or repoint to an in-repo path.
  Confidence: 90
- [NOTE] The compile-is-the-proof argument covers in-repo callers only — `openspec/changes/dead-exported-surface/proposal.md:17`
  Failure: `ConditionConnected`/`ConditionCredentialsVerified` and `RunExportItems` are exported from module-public paths (`api/v1alpha1`, `internal/sink`); any external module consumer breaks on `go build ./...` in *their* tree, which no in-repo gate observes. Internal packages under `internal/` are safe by construction.
  Fix: one line in the proposal acknowledging the proof is in-repo-only, or confirm the module has no external consumers.
  Confidence: 60

Personas that came up empty, with attempts made: Saboteur (no live caller found for any symbol; per-symbol probes re-run; no ordering hazards — no CRD/chart/deployment objects touched); Security Auditor (auth-cache removal traced end to end — SAR decision computed before caching, only `allowed` persisted; no new dependencies; deletion-only change shrinks surface).

## Could not check

- Parallel review records exist at `openspec/changes/dead-exported-surface/reviews/R/`; I did not open any of them, though file paths and incidental grep fragments surfaced in tool output. Every finding above rests on my own reads of the code, made before those fragments appeared.
- Nothing executed: `go build`/`go vet`/`go test`, the forgejo-tagged suite, and any mutation test of the breaker tests were not run (read-only review).
- Included Taskfiles (`hack/task/Taskfile.test.yml`) not read, so `task lint`/`task spec:validate` target existence is unverified; `COVERAGE_MIN: "90"` verified at `Taskfile.yml:13`.
- External consumers of `github.com/platformrelay/kollect` — uncheckable from here.
- `kollect-xconsol-final/report.md` content — absent in-repo (see finding 5).
