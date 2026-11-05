# R round 1 — verification and triage

Spec set `a505029da`. Reviewer A (spec lens): REQUEST_CHANGES. Reviewer B (spec lens): CONCERNS.
Each finding was checked against the code by the orchestrator on 2026-11-05 before triage.

| # | Finding | Sev | Reviewers | Verified | Disposition |
|---|---|---|---|---|---|
| 1 | A parallel test (`TestResetBreakersForTest_clearsOpenBreaker`, `circuit_breaker_test.go:95-96`) calls `ResetBreakersForTest` (`:155`), which clears every key; unique keys do not make new tests safe; `-count` reuses keys | CRITICAL (A) / WARNING (B) | A, B | yes: `t.Parallel()` at `:96`, reset at `:155`, `breakerRegistry.Clear()` at `circuit_breaker.go:69` | fix: design *Testing approach* isolation rules; tasks test rules; 1.2 makes the test non-parallel; 0.1 lists every caller |
| 2 | Mutable package-variable timeout seam races under `-race` and leaks into other breakers | WARNING | A, B | yes (design allowed it) | fix: constructor `newBreaker(key, timeout)` only |
| 3 | Identity, payload, layout, acquire and CA/TLS errors return before the breaker | WARNING | A, B | yes: `export.go:86-190` return before `:200` | fix: proposal *Why*/*Impact*, design *Context* (three zones); 0.1 classifies each site |
| 4 | Classification boundary unspecified; Forbidden and `ClassOf`-inferred Terminal not specified | WARNING (A) / NOTE (B) | A, B | yes: `errors.go:86-108` | fix: SCB-1 boundary text and two scenarios; SCB-2 names Forbidden; cases SCB-1d, SCB-2c, SCB-2d |
| 5 | SCB-3a does not prove state transitions; no concurrent-trial case | WARNING | A | yes: two runs pass if an excluded trial closes the breaker | fix: `State()` assertions in SCB-3a/3b; SCB-3c concurrent probe; spec scenario added |
| 6 | Flake root cause overstated: the Terminal test runs once per count; the Transient and success tests share the key | WARNING | A, B | yes: `newExportFailureReconciler` (`kollectinventory_export_test.go:339-375`) used by both | fix: proposal reworded; 2.1 ten runs; 2.2 isolates regardless |
| 7 | Troubleshooting "self-closes after timeout" is wrong for gobreaker | NOTE | A | yes: half-open then trial | fix: task 3.1 wording |
| 8 | Terminal trial can keep a recovering sink half-open | NOTE | B | yes (follows from `validRequests`) | fix: stated as accepted consequence in SCB-3 and proposal *Impact* |

The CRITICAL (1) has a mechanical fix both reviewers named and that the code confirms; it was fixed
in the spec set and goes to a second and last spec-set round instead of stopping (spec-loop R rule).
