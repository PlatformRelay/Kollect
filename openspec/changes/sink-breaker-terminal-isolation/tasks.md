# Tasks

Red first. Each group's test task comes before its implementation and records its red in
`evidence/G<N>.md`: the named test runs (`=== RUN <name>`) and fails on the target assertion, not on
a compile error or a skip. "Red on base" means red on `origin/main` for G1 and on the G1 commit for
G2. A stub with a `not implemented` (no-op) body is in scope so tests compile. Each group closes in
one commit (tests and implementation), so the tree is green at every commit. A case marked *control*
is expected green on its base and is recorded as a control, not a red.

Each case reads: given / when / then · the failure it exposes.

Test rules for every group:

- Breaker unit tests live in `internal/sink/circuit_breaker_test.go` and use `t.Parallel()` with a
  key unique per test invocation (`-count` safe). No parallel test calls `ResetBreakersForTest`. The
  only timeout seam is the `newBreaker(key, timeout)` constructor; no test sleeps the production 30 s
  timeout and none writes a package variable. See `design.md` *Testing approach*.
- Each group records `go test -race -count=3 ./internal/sink/ ./internal/controller/` (G2 adds
  `-skip TestControllers` six times, see 2.1) and `task lint`.
- Mutations are judged by the test command's exit status, with one no-op control that must survive;
  mutant trees live outside the repository.

## 0. Probe

- [ ] 0.1 Verify the load-bearing claims in `proposal.md` and `design.md` on `origin/main` and record
  each with file:line or command output in `evidence/T0.1.md`: the breaker settings and key; that
  gobreaker v2.4.0 `IsExcluded` neither counts nor resets `ConsecutiveFailures` and frees the
  half-open slot; the three zones in `design.md` *Context* — for each Terminal site cited there, is
  it before, at or after the breaker; that Git auth failures are classified only after the breaker;
  every caller of `ResetBreakersForTest` and whether its test is parallel; RACE-COUNT-BREAKER-ISOLATION
  on base (`go test -race -count=3 -skip TestControllers ./internal/controller/`, ten runs, exit
  status each, failing test names). A flake that does not reproduce in ten runs is recorded as "not
  reproduced", not as "mechanism disproved". A claim that does not hold stops the loop with a
  decision request.

## 1. Breaker excludes Terminal errors (SCB-1, SCB-2, SCB-3)

- [ ] 1.1 Tests (red on base unless marked *control*):
  - SCB-1a: given a fresh key, when `exportThroughBreaker` returns a Terminal error 10 times, then an
    11th call with a succeeding function runs it and returns nil · exposes Terminal errors counting
    as failures.
  - SCB-1b (boundary): given a stub backend that fails Terminal for inventory path A and succeeds for
    path B on one sink, when `RunExportEnvelope` runs A 10 times, then B's export reaches the backend
    and succeeds · exposes the cross-inventory blast radius at the real call site.
  - SCB-1c: given a fresh key, when calls fail T, X, T, T, T (T Transient, X Terminal), then the
    sixth call still runs its function (four counted failures); when that sixth call fails T, the
    seventh does not run and returns a Transient error containing "circuit breaker open" · on base
    the breaker opens after the fifth call (X counted), so the sixth call's assertion is the red;
    after the fix it exposes an implementation that resets the streak on exclusion (`IsSuccessful`
    instead of `IsExcluded`), which would let the seventh call run.
  - SCB-2a (*control*): 5 Transient failures, then the 6th call's function does not run and the
    error is Transient with "circuit breaker open" · guards against over-excluding.
  - SCB-2b (*control*): same with an unclassified `errors.New` error · guards against excluding
    everything that is not Transient.
  - SCB-2c (*control*): same with `kollecterrors.Forbidden(...)` · guards against excluding Forbidden.
  - SCB-2d (*control*): same with a raw error whose text a later classifier would call a Git auth
    failure (e.g. `errors.New("authentication required")`) · guards against classifying at the
    breaker with the after-breaker classifier.
  - SCB-1d: an unwrapped `apierrors.NewNotFound(...)` returned 10 times leaves the breaker closed (an
    11th succeeding call runs) · documents that `ClassOf`-inferred Terminal is excluded (red on base).
  - SCB-3a: given a short-timeout breaker (constructor seam) opened by 5 Transient failures and its
    timeout elapsed, when the trial returns Terminal, then `State()` is half-open; when the next call
    succeeds, its function ran and `State()` is closed · exposes an excluded trial closing the
    breaker (state after the trial) or wedging the slot (next call not run).
  - SCB-3b: as 3a, but the call after the Terminal trial fails Transient, then `State()` is open and
    the following call does not run · exposes an excluded trial closing the breaker.
  - SCB-3c: half-open breaker; a trial blocks on a channel; a second concurrent call does not run its
    function and returns an error (gobreaker `ErrTooManyRequests`); release the trial with a Terminal
    result; a third call runs · exposes a Terminal trial wedging the slot. *Control* for the
    concurrency half on base.
- [ ] 1.2 Add `newBreaker(key, timeout)` and set `IsExcluded: kollecterrors.IsTerminal` in it;
  `breakerFor` passes `circuitBreakerTimeout`. Make `TestResetBreakersForTest_clearsOpenBreaker`
  non-parallel and fix any other parallel caller of `ResetBreakersForTest` found in 0.1. Mutations:
  drop `IsExcluded`; replace it with `IsSuccessful`; exclude every non-nil error; exclude
  `!IsTransient` (would exclude Forbidden). Each must be killed by a named 1.1 case.

## 2. Reconciler tests stop sharing a tripped breaker (RACE-COUNT-BREAKER-ISOLATION)

- [ ] 2.1 On the G1 commit run `go test -race -count=3 -skip TestControllers ./internal/controller/`
  ten times and record each exit status, against T0.1's base count. Any red → name the failing tests
  and the breaker key they share; that red is the group's red. All ten green is weak evidence (the
  Transient-failure test still shares `default/postgres-demo`), so 2.2 runs either way.
- [ ] 2.2 Isolate the reconciler tests built on `newExportFailureReconciler` and the other tests that
  share the `default/postgres-demo` key from each other's breaker state, without changing what they
  assert (a per-invocation sink name where the assertion does not name the sink; otherwise a
  non-parallel test with `t.Cleanup(sink.ResetBreakersForTest)`), never a retry. Ten green runs of
  the 2.1 command afterwards.

## 3. Docs

- [ ] 3.1 `docs/operator-manual/troubleshooting.md:214`: the breaker counts retryable export errors
  (Transient, Forbidden, unclassified), not Terminal ones; a Terminal failure stays in that
  inventory's status. Replace "self-closes after timeout": after 30 s one trial export is admitted,
  success closes it, a counted failure re-opens it, a Terminal result leaves it half-open.
  `docs/adr/0602-error-taxonomy.md:38`: add that Terminal errors are excluded (dated note, ADR not
  re-decided). `task lint:markdown` 0 issues; `rg -n 'circuit breaker' docs` reviewed for any other
  claim the change contradicts, the list recorded in `evidence/G3.md`.

## 4. Integration

- [ ] 4.1 Integrator (orchestrator): branch review (stage B), PR with the release-note line, merge on
  review + green CI, post-merge evidence, archive the change, update BACKLOG (SINK-BREAKER-TENANT-01,
  RACE-COUNT-BREAKER-ISOLATION).
