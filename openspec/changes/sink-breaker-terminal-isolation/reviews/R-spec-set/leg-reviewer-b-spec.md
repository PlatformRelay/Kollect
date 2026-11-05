## Verdict: CONCERNS

## Findings
- [WARNING] The test rules say parallel breaker tests with unique keys are safe. They are not, because a sibling parallel test clears every key. — `internal/sink/circuit_breaker_test.go:96,155`, `internal/sink/circuit_breaker.go:68-70`, `openspec/changes/sink-breaker-terminal-isolation/tasks.md:14-15`, `design.md:41-42`
  Failure: `TestResetBreakersForTest_clearsOpenBreaker` is `t.Parallel()` and calls `ResetBreakersForTest()`, which runs `breakerRegistry.Clear()` on all keys. If that reset lands between SCB-2a's fifth Transient failure and its sixth call, or after SCB-3's trip, the "does not run" or "re-opened" assertion fails at random. The file says as much at lines 165-166 ("must not wipe the breakers of sibling parallel tests mid-trip"). This is the same flake class that group 2 is meant to remove.
  Fix: make `TestResetBreakersForTest_clearsOpenBreaker` non-parallel, or replace it with a per-key reset helper, and add a test rule: "no `t.Parallel()` test may call `ResetBreakersForTest`".
  Confidence: 80
- [WARNING] The design lets the timeout test seam be an unexported package variable. With the required `t.Parallel()`, that is a data race under `-race`. — `design.md:45-47`, `tasks.md:14,17,56`
  Failure: SCB-3a and 3b write the variable to shorten the timeout while other parallel tests read it in `breakerFor`. `go test -race` reports DATA RACE, and both groups record their gates under `-race`. Restoring the value in `t.Cleanup` also changes the timeout for breakers that other tests create at the same moment.
  Fix: allow only the constructor-parameter form, e.g. a `newBreaker(key, timeout)` used by `breakerFor` with the const, and have tests register a breaker for their own key. Or make the half-open tests serial.
  Confidence: 75
- [WARNING] Some of the proposal's examples of Terminal errors that reach the breaker are wrong: they return before `exportThroughBreaker` runs. — `proposal.md:14-15`, `design.md:12-13`, `internal/sink/export.go:86,117-190` versus `:200`
  Failure: the identity/path check (line 86), the oversized-payload error (173), payload decode/marshal (117/136/161) and layout resolve (186) all return early, so they never count today. Only errors from `plan.run` reach the breaker: the ownership/prune rejection at `git/prune_owned.go:255` and backend-classified Terminal errors. The proposal's Impact line about CA errors is probably wrong the same way (backend acquire happens at ~:107, also before the breaker). Probe 0.1 checks only the ownership claim, so the overstated motivation would pass it.
  Fix: in 0.1, list for each cited Terminal site whether it is inside `plan.run`, and correct the proposal and design.
  Confidence: 85
- [NOTE] SCB-2 says only "Transient and unclassified" errors count. Forbidden-classified errors also count under `IsExcluded = IsTerminal`, and the spec says nothing about them. — `internal/errors/errors.go:86-108`, `spec.md:25`
  Failure: a backend error classified Forbidden, or a raw K8s 403, still trips the breaker. That holds in the code, but it is neither required nor forbidden, so a later change could exclude it without breaking any test.
  Fix: one sentence in SCB-2 that says Forbidden counts, or that it is excluded, plus one test case for it.
  Confidence: 60
- [NOTE] "Unclassified" in the spec is not the same as `ClassOf`. A bare `apierrors` NotFound/Invalid/BadRequest carries no `ClassError`, yet `IsTerminal` treats it as Terminal, so it is excluded. — `internal/errors/errors.go:99-101`, `spec.md:39-40`
  Failure: an error that "carries no class" per the spec's wording, returned by `plan.run`, would not count. SCB-2b uses only `errors.New`, so this goes unnoticed.
  Fix: say in the spec that classification means `kollecterrors.ClassOf`.
  Confidence: 55

**Requirement status:**
- **SCB-1 holds** if the tasks are executed. In gobreaker v2.4.0, `afterRequest` checks `isExcluded` before `isSuccessful`/`isFailure` (`gobreaker.go:264-267`), and `onExclusion` only increments `TotalExclusions` (`counter.go:32-34`), so the streak is neither reset nor incremented. The 1.1 cases (1a, 1b, 1c) cover the scenarios.
- **SCB-2 holds.** The message "sink circuit breaker open for %q" contains "circuit breaker open" (`circuit_breaker.go:40`). SCB-2a and 2b are valid controls.
- **SCB-3 holds.** Half-open admits a request while `validRequests() < maxRequests`, with `validRequests = Requests - TotalExclusions` (`gobreaker.go:246`, `counter.go:36-41`), so an excluded trial frees the slot. Success closes the breaker (`:282`) and a counted failure re-opens it (`:296`). The design's line citations match the module source.
- **The three mutations in 1.2 are each killed.** Dropping `IsExcluded` is caught by SCB-1a, swapping in `IsSuccessful` by SCB-1c, and excluding every error by SCB-2a/2b.
- **Behaviour no requirement mentions:** a Terminal result from the trial request leaves the breaker half-open, so one misconfigured inventory can keep a recovering sink half-open, serialising exports one at a time, for as long as its requests keep winning the trial. This is bounded but not stated. Also, `TestKollectInventoryReconciler_transientTotalExportFailureRequeues` (`internal/controller/kollectinventory_export_test.go:439`) still sends Transient failures to the shared `default/postgres-demo` key. Task 2.1's six-run sample (base fails 4 of 6) gives weak evidence that the flake is gone.

## Could not check
- The breaker results of the four tests named in backlog RACE-COUNT-BREAKER-ISOLATION; the backlog and `agent-context/decisions.md` are outside the change.
- `git.ClassifyExportError` and the backend Terminal mappings (`bigquery/errors.go`, `mongodb/errors.go`), to confirm whether the claim that Git auth failures are classified only after the breaker is true.
- `loop.md` and the untracked `reviews/` directory. No tests were run.
