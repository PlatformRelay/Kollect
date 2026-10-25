Dead symbols fully gone from the tree; auth logic unchanged in substance. Now check store.go lock claims and the remaining guards.
Let me verify the tagged-file compile claim myself — cheapest independent sensor.
## Verdict: CLEAN

Checked: full code diff `3ee21266..HEAD` (1,967 lines) read in full; every deleted symbol re-probed at HEAD (zero refs incl. docs/CRDs, and zero refs at base outside `constants.go` for the condition constants); auth middleware diffed and read end-to-end — cache-hit authz path is byte-equivalent, no behaviour moved; `delete(s.shards` confirmed gone, so the rewritten `NamespaceVersion` comments (store.go:174-186) are accurate and the deleted monotonicity test's scenario is genuinely unreachable; migrated breaker tests re-run on the live `RunExportEnvelope` path (green, and they do exercise `exportThroughBreaker` — the round-1 CRITICAL fix holds); `go vet -tags integration` on `./internal/sink/... ./internal/collect/... ./api/...` run myself: clean; T9 shadow fix verified minimal; no lint/arch/config or allow-list file touched; `Memfs`/`memory` deps remain production-used elsewhere, so the test-only copy introduces no dep change; BREAKING CHANGE footer present on the api commit.

## Findings
- [NOTE] A test comment now cites line numbers the deletion itself invalidated — `internal/sink/export_test.go:74`
  Failure: `// nil registry → terminal error (line 125-127)` (also :87) pointed at `RunExportEnvelope`'s guards in the pre-T1 `export.go`; T1 deleted ~90 lines above them, so the guards now live at `internal/sink/export.go:74-85` and the cited lines name unrelated code — a comment describing where the guard is that is now wrong.
  Fix: replace the two line-range cites with the symbol anchor (`RunExportEnvelope guards above`).
  Confidence: 90
- [NOTE] T1 coverage-accounting table cites stale line anchors — `openspec/changes/dead-exported-surface/evidence/T1.md:86`
  Failure: rows cite `export_test.go:389/463/517`; post-branch those tests sit at :70/:126/:180 — a maintainer following the evidence to confirm "behaviour still covered" lands on the wrong function or a blank region.
  Fix: re-anchor the table to test names (T6/round-2 already learned this lesson for line anchors).
  Confidence: 85
- [NOTE] `git.ExportMemory` was renamed into the test file verbatim, not deleted — `internal/sink/git/export_test.go:431` (`func exportMemory`)
  Failure: the three `TestExportMemory*` tests now exercise a 30-line copy of a function production no longer has — nothing production is under test, and the copy can drift from `ExportWithBranch`'s path-validation semantics unnoticed. Pre-branch the same vacuity existed behind an exported name; the branch shrinks the API but keeps the corpse under a lowercase name.
  Fix: acceptable as-is (deliberate, stated in the proposal); if it is ever touched again, drop the copy and the three tests rather than maintaining it.
  Confidence: 80
- [NOTE] Sortedness of `NamespacesForClusterTarget` lost its only assertion — `internal/collect/engine.go:419`
  Failure: `slices.Sort` is now guarded by nothing — the deleted `TestEngineSetScrubKeysAndBindClusterTargetNamespaces` asserted sort order; surviving tests use `ConsistOf` (order-insensitive). Deleting the `slices.Sort` line today passes CI (the remaining helpers sort their own inputs).
  Fix: one same-package test asserting `NamespacesForClusterTarget` returns sorted namespaces via a `newEngineWithBoundClusterTargets`-style binding.
  Confidence: 70
- [NOTE] The per-task gate matrix missed the lint class the final gate caught — `openspec/changes/dead-exported-surface/evidence/T8.md`
  Failure: T6 and T1's test additions carried two govet `shadow` findings that only surfaced at T8's final `task lint`, costing a BLOCKED→T9 round-trip; no per-task sensor would have caught it had the branch stopped at the task loop.
  Fix: add `golangci-lint run ./<touched-pkg>/...` (config untouched) to the per-task sensor set so shadow/unused classes fail where they are introduced.
  Confidence: 75

## Could not check
- Did not run `task coverage`, `task test` (47 pkgs, envtest, ~8 min) or `task lint` myself; the 91.3% floor parity, full-suite green and "0 issues" are as recorded in evidence/T8.md and T9.md. What I ran: focused breaker tests, tag-on `go vet`, symbol greps — all green.
- External consumers of the deleted `api/v1alpha1` constants — unverifiable in-repo (proposal records this as an accepted pre-1.0 assumption; the compile proves only the in-repo half).
- The consolidating report `data/kollect-xconsol-final/report.md` lives outside this repo (out of bounds); relied on its verbatim Sweep-2 quote in the proposal.
- `integration`-tagged git suites (`TestExportBareRepoIntegration`, forgejo tests) compile-checked only — they need a live forgejo/bare-remote env; not executed.
- Did not mutate sources to mutation-check the migrated breaker tests or the surviving `TestRunExportEnvelope_*` battery (plan mode, read-only); judged load-bearingness by reading `RunExportEnvelope`'s failure paths instead.
- Race behaviour of the new `newEngineWithBoundClusterTargets` informers under `-race` beyond the recorded suite runs.
