Reviewed commit `de1e7df12` against repository code and cached gobreaker v2.4.0 source. Round 1’s reset isolation, immutable timeout constructor, Forbidden/inferred-Terminal coverage, and half-open concurrency fixes are sound in principle. The classification-boundary fix remains factually incorrect.

1. **CRITICAL — Git authentication failures already reach the breaker as Terminal.**  
   Evidence: `design.md:22–24` says they become Terminal only afterwards; `tasks.md:29–34` requires verifying that claim and stopping if false. But `internal/sink/git/export.go:128–149` calls `ClassifyExportError` before returning from the backend. `git/backend.go:65,77` and `layout_export.go:151,217` connect those returns to the export plan inside the breaker. Authentication failures become Terminal at `git/errors.go:35–36`.

   Executing task 0.1 correctly therefore stops the plan. Proceeding regardless would ship a broader behavior change than documented: Git authentication and protected-branch failures also stop tripping the breaker.  
   **Fix:** correct the classification inventory, proposal non-goals, and impact statement. Explicitly accept exclusion of these sink-wide Git failures and add coverage using the actual Git classification path. Retain the raw-error scenario as a separate synthetic boundary case.

2. **WARNING — SCB-2d does not protect the real classification boundary.**  
   Evidence: `design.md:50–52` specifies direct breaker tests except for SCB-1b; `tasks.md:56–58` claims SCB-2d guards against moving classification before the breaker. A mutation changing `export.go:201` to return `git.ClassifyExportError(plan.run(exportCtx))` passes that direct test. SCB-1b also passes because its backend already returns Terminal.

   **Fix:** run SCB-2d through `RunExportEnvelope` with a backend returning raw `"authentication required"`. Assert five backend invocations return Terminal to callers, while the sixth is rejected as breaker-open. Require this test to kill the classification-movement mutation.

3. **WARNING — The normative trip requirement omits the existing interval reset.**  
   Evidence: `specs/sink-circuit-breaker/spec.md:39–41` unconditionally requires opening after five consecutive counted failures. Production sets `Interval: 60s` at `internal/sink/circuit_breaker.go:54`; gobreaker’s `gobreaker.go:304–309,348–359` clears the closed-state generation when that interval expires. Five consecutive failures spanning that reset need not open it.

   **Fix:** qualify the requirement and streak scenarios with “within the same closed-state counting interval,” and state that interval expiration resets counts independently of exclusions. Preserve the existing 60-second behavior.

4. **WARNING — Short-timeout state assertions can conceal reopening or introduce flakes.**  
   Evidence: `tasks.md:61–70` observes `State()` and immediate subsequent admission using an unspecified short timeout. Gobreaker’s `State()` advances time-dependent state (`gobreaker.go:195–201`); an incorrectly reopened breaker becomes half-open again after timeout (`:312–315`). Thus SCB-3a/3c can miss reopening if assertions run late, while SCB-3b can fail correctly implemented code after its second timeout expires.

   **Fix:** specify a positive timeout and bounded timing protocol, and capture state transitions through a per-instance callback configured at construction. Assert that a Terminal trial causes no transition to open or closed. Keep callback configuration immutable and avoid global seams.

5. **NOTE — G2’s evidence rules contradict its intentional unconditional isolation.**  
   Evidence: `tasks.md:3–8` requires each group’s failing target assertion, but `:79–86` permits ten green G1 runs and still mandates G2. The proposal also still describes isolation as conditional (`proposal.md:70–71`).

   **Fix:** explicitly allow G2 to record “no reproduction; preventive isolation,” without manufacturing a red. Align the proposal with unconditional isolation and correct “six times” at `tasks.md:19` to ten.

Runtime tests were not run: the configured Go launcher attempted to install Go 1.27.3 and failed under the read-only filesystem.

## Verdict

REQUEST_CHANGES
