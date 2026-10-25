## Verdict: CLEAN

The change does what it claims: `RunExportItems`/`ExportItemsRequest`/`sinkNamespaceForExport` had zero production callers at the pre-task sha (verified: `git grep` at `b1a2554e` finds them only in `export.go` itself, the two test files, and the controller comment; no `_test.go`-excluded or build-tagged caller exists). Post-change the whole repo greps clean, `go build ./...` and `go vet ./internal/sink/... ./internal/controller/...` exit 0, and the two migrated breaker tests pass. The reworded controller comment is accurate for the per-binding ceiling it documents (`kollectclusterinventory_controller.go:275-279`). No config, allow-list, exclusion or coverage baseline was touched.

## Findings
- [NOTE] Evidence's own test-count is inconsistent — `openspec/changes/dead-exported-surface/evidence/T1.md:123`
  Failure: "Deleted 8 `TestRunExportItems_*` tests" contradicts matrix row 5 ("nine") and the coverage table; `git diff` shows exactly 9 deleted. Reader cannot tell which number is true.
  Fix: change "8" to "9" in the Test changes section.
  Confidence: 95
- [NOTE] Coverage-accounting line numbers are stale (pre-deletion), not as claimed "verified" — `evidence/T1.md:86-90`
  Failure: cites `export_test.go:389/463/517` for `TestRunExportEnvelope_guards`/`_acquireBackendFailure`/`_relationalRemashals...`; those tests now sit at `:70/:126/:180`. The claims are about the right tests, but the anchors mislead a reader who greps them.
  Fix: re-anchor to current line numbers (or drop line numbers, keep test names).
  Confidence: 90
- [NOTE] Migrated trip test cannot prove the breaker opened — `internal/sink/circuit_breaker_test.go:84-90`
  Failure: after the loop the assertion only requires a non-nil *transient* error, but the underlying `exportErr` ("network down") is itself classified transient (`export.go:238`), so the test passes even if `exportThroughBreaker` never trips. Only `TestResetBreakersForTest_clearsOpenBreaker:149-157` actually distinguishes open-state (it is the sole assertion that proves the breaker engaged). Pre-existing, unchanged by the migration.
  Fix (optional): assert the breaker-open message/`key` on the post-trip call, e.g. require the error to name `sink circuit breaker open`.
  Confidence: 85
- [NOTE] `ResetBreakersForTest()` clears the process-global `breakerRegistry` while both tests run `t.Parallel()` — `circuit_breaker_test.go:93,153`
  Failure: if the reset runs mid-loop of the trip test, that test's 5 consecutive failures are wiped and the trip assertion flakes. Pre-existing (identical structure before this diff), not introduced here.
  Fix: drop `t.Parallel()` from the reset test, or give breakers per-test keys and reset by key.
  Confidence: 60

## Could not check
- Full `go test ./internal/sink/... ./internal/controller/...` (evidence claims 603 s green; I ran only the focused breaker pair plus build/vet, not the full package suites).
- Coverage floor `≥90%` (`task coverage`) and integration build-tagged files (`go vet -tags integration`); task 8 owns the holistic gates. Deleting covered dead lines plus their tests leaves the ratio unverified at this tier.
- The untracked `openspec/changes/dead-exported-surface/reviews/L-tasks/` register was out of scope for this leg (I am one independent reviewer, not the orchestrator).
