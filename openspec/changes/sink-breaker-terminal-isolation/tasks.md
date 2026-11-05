# Tasks

Red first. Each group's test task comes before its implementation and records its red in
`evidence/G<N>.md`: the named test runs (`=== RUN <name>`) and fails on the target assertion, not on
a compile error or a skip. "Red on base" means red on the group's refactor base: for G1, `origin/main`
plus the pure `newBreaker` extraction and the marker API stubs named in 1.1; for G2, the G1 commit. A
stub with the signature from `design.md` and a `not implemented` body (a no-op that returns its input
unmarked) is in scope so tests compile. Each group closes in one commit (tests and implementation),
so the tree is green at every commit. A case marked *control* is expected green on its base and is
recorded as a control, not a red. G2 may close without a red when its target is preventive (see 2.1);
it says so in its evidence.

Each case reads: given / when / then · the failure it exposes.

Test rules for every group:

- Breaker unit tests live in `internal/sink/circuit_breaker_test.go` and use `t.Parallel()` with a
  key unique per test invocation (`-count` safe). No parallel test calls `ResetBreakersForTest`. The
  only timeout seam is the `newBreaker(key, timeout)` constructor; half-open tests follow the timing
  protocol in `design.md` *Testing approach*; no test writes a package variable.
- Each group records `go test -race -count=3 ./internal/errors/ ./internal/sink/... ./internal/controller/`
  and `task lint`.
- Mutations are judged by the test command's exit status, with one no-op control that must survive;
  mutant trees live outside the repository.

## 0. Probe

- [x] 0.1 Verify the load-bearing claims in `proposal.md` and `design.md` on `origin/main` and record
  each with file:line or command output in `evidence/T0.1.md`: the breaker settings and key; that
  gobreaker v2.4.0 `IsExcluded` neither counts nor resets `ConsecutiveFailures`, frees the half-open
  slot, and that the 60 s `Interval` resets closed-state counts; the three zones in `design.md`
  *Context*; that Git auth/protected-branch errors reach the breaker Terminal; that a failed export is
  attempted again on the next event-driven reconcile (cite the code path). Then the two tables
  design.md *Marker* depends on: (a) every one of the 25 `invalidPrune` call sites and every other
  Terminal raised inside an export plan (Git, GitLab, object-store and database backends), each
  classified inventory-scoped / sink-wide / undecided with a one-line reason; (b) every caller of
  `ResetBreakersForTest` and whether its test is parallel. Name the local-repository fixture that
  produces a real Git ownership rejection through `RunExportEnvelope`. Finally
  RACE-COUNT-BREAKER-ISOLATION on base (`go test -race -count=3 -skip TestControllers
  ./internal/controller/`, ten runs, exit status each, failing test names); a flake that does not
  reproduce in ten runs is recorded as "not reproduced", not as "mechanism disproved". A claim that
  does not hold stops the loop with a decision request. — closed 2026-11-05, evidence: evidence/T0.1.md

## 1. Inventory-scoped results are excluded from the breaker (SCB-1, SCB-2, SCB-3)

- [x] 1.1 Refactor base first, in the same group: extract `newBreaker(key, timeout)` from `breakerFor`
  with today's settings (pure refactor, existing tests green), and add `ErrInventoryScoped`,
  `InventoryScoped` and `IsInventoryScoped` with no-op bodies (`InventoryScoped` returns its input).
  Then the tests (red on that base unless marked *control*):
  - M-1 (`internal/errors`): `InventoryScoped(e)` is Terminal, `IsInventoryScoped` true,
    `errors.Is(_, e)` true, `Error()` equals `e.Error()`; `InventoryScoped(nil)` is nil; a plain
    `Terminal(e)` is not inventory-scoped · exposes a marker that drops the class, the text or the
    wrapped chain.
  - SCB-1a: given a fresh key, when `exportThroughBreaker` returns `InventoryScoped(...)` 10 times,
    then an 11th call with a succeeding function runs it and returns nil · exposes marked results
    counting as failures.
  - SCB-1b (boundary): given a stub backend that branches on object path, failing
    `InventoryScoped(...)` for inventory A and succeeding for B on one sink, when `RunExportEnvelope`
    runs A 10 times, then B's export reaches the backend and succeeds · exposes the cross-inventory
    blast radius at the real call site.
  - SCB-1c: given a fresh key, when calls fail T, X, T, T, T (T Transient, X marked), then the sixth
    call still runs its function; when it fails T, the seventh does not run and returns a Transient
    error containing "circuit breaker open" · red on base because X counts; after the fix it exposes
    an implementation that resets the streak on exclusion (`IsSuccessful`).
  - SCB-1e (boundary, real Git backend, not parallel): with the fixture T0.1 names (`identityRemote`), calling `RunExportEnvelope` directly with a per-invocation sink name (its `export` helper resets every breaker), an ownership rejection returned
    by `RunExportEnvelope` is Terminal and inventory-scoped, and after 6 such rejections a different
    inventory's export to the same sink runs · exposes a marker lost between `invalidPrune`'s
    replacement and the breaker, or a site left unmarked.
  - SCB-2a (*control*): 5 Transient failures, then the 6th call's function does not run and the
    error is Transient with "circuit breaker open" · guards against over-excluding.
  - SCB-2b/2c/2f (*control*): same with an unclassified `errors.New`, with `kollecterrors.Forbidden`,
    and with an unwrapped `apierrors.NewNotFound` (Terminal by `ClassOf`, unmarked) · guard against
    excluding by class instead of by marker.
  - SCB-2d (*control*, boundary): `RunExportEnvelope` with a stub backend returning
    `git.ClassifyExportError(errors.New("authentication required"))` (Terminal, sink-wide): the
    sixth export does not reach the backend · guards against excluding every Terminal (option A).
  - SCB-3a: short-timeout breaker opened by 5 Transient failures, timeout elapsed; the trial returns
    `InventoryScoped(...)`; `State()` is half-open; the next call succeeds, its function ran, and
    `State()` is closed · exposes an excluded trial closing the breaker or wedging the slot.
  - SCB-3b: as 3a, but the call after the marked trial fails Transient; `State()` is open and the next
    call does not run · exposes an excluded trial closing the breaker.
  - SCB-3c: half-open breaker; a trial blocks on a channel; a second concurrent call does not run its
    function and returns an error; release the trial with a marked result; a third call runs ·
    exposes a marked trial wedging the slot. *Control* for the concurrency half on base.
- [x] 1.2 Implement the marker (`design.md` *Marker*), route exactly the 5 inventory-scoped sites from T0.1
  table (a) through the new helper (undecided sites stay unmarked), set `IsExcluded: kollecterrors.IsInventoryScoped` in
  `newBreaker`, and make `TestResetBreakersForTest_clearsOpenBreaker` non-parallel. Mutations, each
  killed by a named case: drop `IsExcluded` (SCB-1a); `IsSuccessful` instead (SCB-1c, SCB-3a);
  exclude `IsTerminal` (SCB-2d, SCB-2f); exclude every non-nil error (SCB-2a); marker drops `Unwrap`
  (M-1); one routed site reverted to `invalidPrune` (SCB-1e). — 1.1 and 1.2 closed 2026-11-05, evidence: evidence/G1.md

## 2. Reconciler tests stop sharing a tripped breaker (RACE-COUNT-BREAKER-ISOLATION)

- [x] 2.1 On the G1 commit run `go test -race -count=3 -skip TestControllers ./internal/controller/`
  ten times and record each exit status, against T0.1's base count. Any red → name the failing tests
  and the breaker key they share; that red is the group's red. All ten green → record "not
  reproduced; isolation is preventive" (the tests still share `default/postgres-demo`); 2.2 runs
  either way.
- [x] 2.2 Isolate the reconciler tests built on `newExportFailureReconciler` and the other tests that
  share the `default/postgres-demo` key from each other's breaker state, without changing what they
  assert: a per-invocation sink name where no assertion names the sink; otherwise a non-parallel test
  that calls `sink.ResetBreakersForTest()` at its start and in `t.Cleanup`. Never a retry. Also make
  `TestClusterInventory_DebouncedExportIncrementsMetric` non-parallel with a comment (it reads a
  process-global counter as a delta; precedent `terminalCleanupIncrementsMetric`; added after G2
  round 1, decision in loop.md *Triage*). Ten green runs of the 2.1 command afterwards. — 2.1 and 2.2 closed 2026-11-05, evidence: evidence/G2.md

## 3. Docs

- [x] 3.1 `docs/operator-manual/troubleshooting.md:214`: the breaker opens after 5 consecutive counted
  export failures per sink key (Transient, Forbidden, sink-wide Terminal such as Git auth); a failure
  that is about one inventory (ownership conflict) does not count and stays in that inventory's status.
  Replace "self-closes after timeout": after 30 s one trial export is admitted, success closes it, a
  counted failure re-opens it, an inventory-scoped result leaves it half-open.
  `docs/adr/0602-error-taxonomy.md:38`: add a dated note on what counts (ADR not re-decided). `task
  lint:markdown` 0 issues; `rg -n 'circuit breaker' docs` reviewed for any other claim the change
  contradicts, the list recorded in `evidence/G3.md`. — closed 2026-11-05, evidence: evidence/G3.md

## 4. Integration

- [ ] 4.1 Integrator (orchestrator): branch review (stage B), PR with the release-note line, merge on
  review + green CI, post-merge evidence, archive the change, update BACKLOG (SINK-BREAKER-TENANT-01,
  RACE-COUNT-BREAKER-ISOLATION).
