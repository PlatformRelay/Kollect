Reviewed commit `a505029da` and the local gobreaker v2.4.0 source. The proposed `IsExcluded` setting has the intended closed/half-open behavior. I did not execute tests in this read-only review; the historical “4 of 6” reproduction remains unverified.

1. **CRITICAL — Parallel tests remain vulnerable to global breaker resets.**  
   `tasks.md:14–16` and `design.md:41–42` assume unique keys make parallel breaker tests safe. However, `internal/sink/circuit_breaker_test.go:95–96,155` runs a parallel test that calls `ResetBreakersForTest()`, which clears **all** keys (`internal/sink/circuit_breaker.go:68–69`). It can erase a new test’s breaker between failures, or between opening and probing it. This introduces intermittent failures into the required validation and can invalidate mutation evidence.  
   **Fix:** Make the existing global-reset test nonparallel, audit other reset callers, and require keys unique per test **invocation**, including `-count` repetitions.

2. **WARNING — The permitted timeout seam can race and affect unrelated breakers.**  
   `design.md:45–47` permits a mutable package variable while `tasks.md:14–16` requires parallel tests. Reading that variable once during construction does not synchronize it with test writes; unrelated breakers can also inherit the shortened timeout.  
   **Fix:** Require a constructor parameter with production explicitly supplying 30 seconds. Install the shortened-timeout breaker only under the current test’s unique key. Prohibit a mutable global override.

3. **WARNING — Several claimed examples never reach the breaker.**  
   `proposal.md:11–17` and `design.md:11–15` include identity, payload and CA/TLS configuration errors among failures counted by the breaker. Identity validation returns at `internal/sink/export.go:86–89`; payload failures return at lines 115–120 and 168–177; backend construction failures return at lines 92–110. For example, Git configuration faults are Terminal during construction (`internal/sink/git/backend.go:20–33`). The breaker starts only at `export.go:200`.  
   **Fix:** Separate errors occurring before, inside and after the breaker. Use ownership rejection and backend write classification as affected examples. Replace the CA example in `proposal.md:70` with an actual sink-wide Terminal returned during export, such as BigQuery authorization failure (`internal/sink/bigquery/errors.go:67–71`).

4. **WARNING — The requirements omit the classification boundary and leave Forbidden behavior unspecified.**  
   `specs/sink-circuit-breaker/spec.md:7–8,25–27` does not limit Terminal classification to the breaker boundary. Git auth errors become Terminal afterwards (`internal/sink/export.go:208–212`; `internal/sink/git/errors.go:35–36`) and will still trip it. Also, `IsTerminal` derives Terminal from raw Kubernetes status errors; an error need not carry an explicit class (`internal/errors/errors.go:99–104,115–117`). Forbidden errors remain counted, but SCB-2 does not say so.  
   **Fix:** Define exclusion as `IsTerminal(err)` evaluated on `plan.run`’s result; count every other non-nil result. Add controls for Forbidden, inferred Terminal, sink-wide Terminal, and an error classified Terminal only after the breaker.

5. **WARNING — SCB-3a does not prove the state transitions it claims.**  
   `tasks.md:51–53` says admitting two subsequent successful calls exposes premature closing and proves closure. Both calls also run if the Terminal trial incorrectly closes the breaker. They can also run if successful trials incorrectly leave it half-open while releasing slots. SCB-3b covers premature closing, but neither specified assertion proves successful recovery closes it.  
   **Fix:** Assert `StateHalfOpen` immediately after the Terminal trial and `StateClosed` after success. Add a channel-controlled concurrent probe: while a trial is blocked, a second call must not execute and must return `ErrTooManyRequests`; after Terminal completion, one new trial must be admitted.

6. **WARNING — The claimed controller root cause overstates what the named test can accumulate.**  
   `proposal.md:25–28` attributes opening to repeated runs of the Terminal test. That test performs one reconciliation (`internal/controller/kollectinventory_export_test.go:405–413`), so `-count=3` contributes only three failures. The Transient test shares its `default/postgres-demo` key through the same helper (`:362,439–443`), contributing additional failures; successful tests also share the key and can reset the streak (`:263,291–312`). The mechanism is mixed outcomes and scheduling, not the Terminal test alone.  
   **Fix:** Describe the shared-key interaction accurately. In task 0.1, distinguish failure to reproduce a probabilistic flake from disproving the mechanism; record the historical reproduction separately from current evidence.

7. **NOTE — Correct the recovery documentation while editing it.**  
   `docs/operator-manual/troubleshooting.md:214`, already targeted by task 3.1, says the breaker “self-closes after timeout.” gobreaker instead enters half-open and closes after a successful trial (`gobreaker.go:276–284,312–315`); excluded Terminal trials can keep it half-open indefinitely.  
   **Fix:** State that timeout permits a trial, Terminal releases the slot without changing state, success closes, and counted failure reopens. Existing export-error metrics remain recorded after exclusion (`internal/sink/export.go:203–212`).

## Verdict

REQUEST_CHANGES
