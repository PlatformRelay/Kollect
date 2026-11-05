## Verdict: BLOCK

## Findings
- [CRITICAL] The design says Git auth failures become Terminal only after the breaker. The code does not support that: the Git backend classifies its own errors inside `plan.run`. — `internal/sink/git/export.go:128-149`, `internal/sink/git/backend.go:65,77`
  Failure: `Backend.Export` and `Backend.ExportFiles` both return `ClassifyExportError(...)`. That call turns "authentication required", 401/403, "protected branch" and "pre-receive hook declined" into Terminal before `exportThroughBreaker` returns (`errors.go:35-42`). With `IsExcluded: IsTerminal`, a revoked Git token or a protected branch, both of which fail the whole sink, will never open the breaker. Every inventory then does a full clone and push attempt before failing. Three places describe the opposite: design *Context* (`design.md:22-24`), the SCB-2 wording "errors that become Terminal only after the breaker", and the "authentication failure" example in the SCB-1 scenario. The proposal *Impact* names BigQuery as the only sink-wide Terminal case. Task 0.1 checks exactly this claim and says a claim that fails stops the loop, so as written the loop stops at 0.1. Round 2 is the last spec-set round.
  Fix: rewrite the three zones in the design and the proposal *Impact* so they say Git auth and push rejections are Terminal at the breaker and will be excluded. Get an operator decision on whether that is acceptable (option A as chosen) or whether these errors need a marker so they keep counting (option B narrowed). Change the SCB-1 scenario example to one that really is classified only after the breaker, or drop the example. Fix the 3.1 docs line to match.
  Confidence: 90
- [WARNING] SCB-3b can be flaky under `-race -count=3`: gobreaker's `State()` moves from open to half-open on its own once the timeout has passed. — `tasks.md:65-66`, `design.md:63-68`
  Failure: with a short constructor timeout (for example 10 ms), a scheduling delay between the trial's Transient failure and the `State()` read or the "following call does not run" check lets the timeout pass. The next call is then admitted as a new trial and runs. The test fails intermittently in the ten-run gate that 2.x relies on.
  Fix: say how far the timeout must exceed the assertion window (for example a timeout of 1 s or more, sleeping just past it once), or assert `open` straight after the failing trial and drop the "following call does not run" check.
  Confidence: 70
- [WARNING] On base, the SCB-3a/b/c reds either fail to compile or fail on a nil breaker, not on the target assertion. — `tasks.md:3-6,61-72`
  Failure: the SCB-3 cases need `newBreaker(key, timeout)`, which does not exist on `origin/main`, and "red on base" is defined as `origin/main`. The permitted "not implemented (no-op)" stub returns nil, so `Execute` panics. That breaks the stated rule that a red must fail on the target assertion.
  Fix: in 1.1, extract `newBreaker` as a pure refactor that keeps the current settings. Record the SCB-3 reds against that refactor, and leave only `IsExcluded` to 1.2.
  Confidence: 65
- [NOTE] With `-count=3`, the reset in 2.2's non-parallel option runs only at cleanup, not at start. — `tasks.md:85-86`
  Failure: a serial test in run 2 executes after the parallel tests of run 1. If any parallel test is still on `default/postgres-demo`, the serial test inherits a breaker that run 1 tripped.
  Fix: call `sink.ResetBreakersForTest()` at the start as well as in `t.Cleanup`, or require that no parallel test keeps the shared key.
  Confidence: 45
- [NOTE] Round-1 fixes that hold against the code:
  - Fix #1 holds. The other callers of `ResetBreakersForTest` are non-parallel: `inventory_identity_test.go`, `release_on_delete_test.go`, the fuzz harness and `circuit_breaker_test.go:167`. Only `:95-96` is parallel, and 1.2 makes it serial.
  - Fix #3 holds. `export.go:79-190` returns before the breaker call at `:200`.
  - Fix #4 holds. `errors.go:86-108`: `ClassOf(nil)` is Transient, so a success is never excluded. An unwrapped NotFound is Terminal.
  - Fix #6 holds. `kollectinventory_export_test.go:339-454`: both tests are parallel and share `default/postgres-demo`.
  - Mutation coverage holds. Dropping `IsExcluded` is killed by SCB-1a, `IsSuccessful` by SCB-1c and SCB-3a, exclude-all by SCB-2a, and `!IsTransient` by SCB-2c.
  - The SCB-2 open-breaker message "sink circuit breaker open for %q" holds (`circuit_breaker.go:40`).
  - SCB-1b needs a new stub that branches on path, because `stubBackend` returns one error for every path (`export_test.go:31-35`). The task implies this but does not say it.
  Confidence: 80

## Could not check
- gobreaker v2.4.0 source (`gobreaker.go:237-302`, `counter.go`): it is in the Go module cache, outside the repository, so I did not read it. The `IsExcluded` claims (no count, no reset, the half-open slot released through `Requests - TotalExclusions`) are unverified here. SCB-3 depends on them, and so does T0.1.
- I did not read `plan.run` and `resolveSnapshotExport`, so I have not confirmed that every Git layout goes through `Backend.Export` or `Backend.ExportFiles`.
- The other sink backends' Terminal sites (`s3/backend.go`, `mongodb/errors.go`, `bigquery/errors.go`) were not checked for further sink-wide Terminal errors at the breaker.
- I ran no tests and found no recorded sensor results for this round.
