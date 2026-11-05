# Proposal

## Why

Every export to a sink runs through one process-wide circuit breaker keyed
`<sinkNamespace>/<sinkName>` (`internal/sink/export.go:200`, `internal/sink/circuit_breaker.go:26-62`,
sony/gobreaker v2.4.0). It trips after 5 consecutive failures (`circuitBreakerTripAt`), stays open
30 s (`circuitBreakerTimeout`), and while open every export to that sink, from every inventory, fails
fast with a Transient "sink circuit breaker open" error.

The breaker counts **every** non-nil error from the export as a failure, Terminal ones included
(gobreaker's default `IsSuccessful` returns false for any non-nil error; `Settings` sets neither
`IsSuccessful` nor `IsExcluded`). Terminal errors are raised inside the export for problems that
belong to one inventory, most visibly the Git ownership/prune rejection
(`internal/sink/git/prune_owned.go:255`), raised from inside the export plan. (Identity, payload,
layout and backend-acquire errors return before the breaker, `internal/sink/export.go:86-190`, and
never reach it; only errors returned by `plan.run` at `export.go:200` do.) Five such
rejections from one misconfigured inventory open the breaker for every healthy inventory on the same
sink for 30 s (backlog SINK-BREAKER-TENANT-01, found by the #412 review 2026-10-04).

This contradicts the documented design: ADR-0602 says the breaker is "per sink/doc backend for
`ErrTransient` storms" (`docs/adr/0602-error-taxonomy.md:38`), and the troubleshooting page says it
opens after "5 consecutive transient failures per sink key"
(`docs/operator-manual/troubleshooting.md:214`).

A related flake: `go test -race -count=3 -skip TestControllers ./internal/controller/` fails in four
reconciler tests (backlog RACE-COUNT-BREAKER-ISOLATION, P3; 4 of 6 base runs failed 2026-10, no DATA
RACE, not yet re-measured). Those tests share the `default/postgres-demo` breaker through
`newExportFailureReconciler` (`internal/controller/kollectinventory_export_test.go:339-375`): the
Terminal-failure test, the Transient-failure test and successful exports all record against one key,
so whether it opens depends on the mix and order of outcomes. Excluding Terminal results removes one
contributor; the Transient test still contributes, so this change re-measures the flake and isolates
the tests if it persists (tasks group 2).

A Terminal error is never retried, so counting it protects no backend; it only widens one
inventory's failure to the whole sink.

## What Changes

- The sink circuit breaker **excludes** errors classified Terminal (`kollecterrors.IsTerminal`) at
  the breaker boundary: they count neither as failures nor as successes (gobreaker `IsExcluded`).
  Transient and unclassified errors keep counting, so the breaker still trips after 5 consecutive
  retryable failures.
- An excluded error during half-open neither closes nor re-opens the breaker and releases the trial
  slot, so the next export is admitted as the trial.
- The four reconciler tests named in RACE-COUNT-BREAKER-ISOLATION pass under `-race -count=3`;
  where a test still shares a breaker key through Transient failures, it is isolated (its own sink
  key or a breaker reset), not retried.
- Docs say which errors count.

Decision: option A of three (exclude all Terminal / tag inventory-scoped Terminal only / key per
inventory), chosen by two independent legs, both A (`agent-context/decisions.md`, 2026-11-05
`sink-breaker-terminal`, pending operator validation).

## Capabilities

### New Capabilities

- `sink-circuit-breaker`: which export errors a sink's shared circuit breaker counts, and how it
  behaves closed, open and half-open.

### Modified Capabilities

None.

## Impact

- Code: `internal/sink/circuit_breaker.go` (breaker settings). No change to `export.go`'s
  classification after the breaker.
- Tests: `internal/sink/circuit_breaker_test.go` (new cases), `internal/controller` tests named in
  RACE-COUNT-BREAKER-ISOLATION (isolation only if the fix alone does not make them pass).
- Docs: `docs/operator-manual/troubleshooting.md:214`, `docs/adr/0602-error-taxonomy.md:38`
  (clarifying note, not a new ADR).
- Behaviour visible to users: a misconfigured inventory no longer opens the breaker for other
  inventories on its sink. A sink-wide Terminal error returned during the export (e.g. a BigQuery
  authorization failure classified Terminal in `internal/sink/bigquery/errors.go`) no longer opens
  the breaker either; it was never retried, and it stays visible in each inventory's status. Export
  error metrics are recorded after the breaker (`export.go:203-212`) and are unchanged. A Terminal
  result on the half-open trial leaves the breaker half-open, so one misconfigured inventory can keep
  a recovering sink admitting one trial at a time until another export's trial succeeds or fails
  retryably; bounded and stated in SCB-3.
- Release note: v0.22.x / next minor, under fixes.

## Non-goals

- Changing the breaker key, trip threshold, interval or timeout.
- Reclassifying any error (e.g. making Git auth failures Terminal before the breaker).
- A breaker-state metric or condition.

## Assumptions

- Terminal classification at the breaker boundary is correct for the backends: an error labelled
  Terminal is not a symptom of an outage. A backend that mislabels an outage as Terminal makes the
  breaker blind to it; those exports are still not requeued, so the load on the backend stays bounded
  (main risk named by both decision legs).
