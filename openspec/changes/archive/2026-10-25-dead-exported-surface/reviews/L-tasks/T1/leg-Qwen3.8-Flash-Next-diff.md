## Verdict: CLEAN

## Findings
- [NOTE] Evidence undercounts deleted tests: "Deleted 8 `TestRunExportItems_*` tests" contradicts its own matrix row 5 ("all nine") and the tree — `openspec/changes/dead-exported-surface/evidence/T1.md:123`
  Failure: `git show b1a2554e:internal/sink/export_test.go` has exactly 9 `TestRunExportItems_*` funcs, all deleted; a future audit trusting "8" mis-accounts coverage.
  Fix: change "8" to "9" in the Test changes section.
  Confidence: 95
- [NOTE] Maintainability item 7 claims "the only additions are two comments" — false: the migration adds ~20 lines of `export.MarshalEnvelope` envelope construction in both breaker tests — `internal/sink/circuit_breaker_test.go:55-61,124-130`
  Failure: record inaccuracy only; the added code is the legitimate `ExportEnvelopeRequest` shape, not sensor-satisfying debt. Two identical envelope-construction blocks are mildly duplicated but below any helper threshold.
  Fix: amend item 7 wording; no code change.
  Confidence: 90

## What I checked (diff lens)
- Diff read in full (`b1a2554e..HEAD`, commit `dfcef3ca`): exactly the 4 in-scope files + evidence; no scope creep. Commit message matches content.
- Deletion is truly dead at HEAD: grep for `RunExportItems|ExportItemsRequest|sinkNamespaceForExport` across all `*.go` → zero hits; `go vet ./internal/sink/... ./internal/controller/...` clean (my own run, not the record's).
- `sinkNamespaceForExport` was only a wrapper — its callee `SinkNamespaceForResolved` retains 4 production callers (e.g. `internal/controller/kollectclusterinventory_controller.go:335`), so only the wrapper went; correct.
- Migrated breaker pair: both pass on my run (`-count=1`, 1.7s). The trip test genuinely exercises `exportThroughBreaker` via `RunExportEnvelope` (`internal/sink/export.go:198`) — a `run()` failure that never reached the breaker would fail the open-breaker assertion at `circuit_breaker_test.go:84-90`; zero-`Inventory` request passes `checkInventoryIdentity` (`export.go:221`) as intended. Trip-at-N and reset assertions preserved verbatim.
- Reworded comment (`kollectclusterinventory_controller.go:278`) is factually right: the deleted runner applied no ceiling and re-marshalled per binding (`export.MarshalEnvelope` call in the removed body).
- Import pruning in `export_test.go` (`math`, `corev1`, `metav1`, `runtime`, `fake`) consistent with deleted tests; orphaned `closableStubBackend` fully removed, no residue.
- Coverage accounting: all 14 cited live-test names exist in the tree (verified by grep, 15 matches incl. the `_reusesPooledInstanceByUID` prefix). Pre-existing parallel-flake risk (global `ResetBreakersForTest` vs `t.Parallel`) unchanged by this diff.

## Could not check
- Did not re-run `go build ./...` or the full `go test ./internal/sink/... ./internal/controller/...` (10m run recorded in evidence; only vet and the focused breaker pair run locally).
- Cannot see sibling repos / other reviewers / orchestrator records outside this checkout.
- Did not verify each coverage-accounting row's *semantic* equivalence beyond reading the cited test names' existence — the "runner-specific" classifications are judgement, accepted as recorded.
