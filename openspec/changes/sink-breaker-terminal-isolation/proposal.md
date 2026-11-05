# Proposal

## Why

Every export to a sink runs through one process-wide circuit breaker keyed
`<sinkNamespace>/<sinkName>` (`internal/sink/export.go:200`, `internal/sink/circuit_breaker.go:26-62`,
sony/gobreaker v2.4.0). It trips after 5 consecutive failures (`circuitBreakerTripAt`) within its
60 s closed-state counting interval, stays open 30 s (`circuitBreakerTimeout`), and while open every
export to that sink, from every inventory, fails fast with a Transient "sink circuit breaker open"
error.

The breaker counts **every** non-nil result of the export plan as a failure (gobreaker's default
`IsSuccessful`; `Settings` sets neither `IsSuccessful` nor `IsExcluded`). Some of those results are
about **one inventory**: the Git ownership/prune rejection (`internal/sink/git/prune_owned.go:254-256`,
`invalidPrune`), raised inside the export plan. Five such rejections from one misconfigured inventory
open the breaker for every healthy inventory on the same sink for 30 s (backlog
SINK-BREAKER-TENANT-01, found by the #412 review 2026-10-04). Identity, payload, layout and
backend-acquire errors are inventory-scoped too but return before the breaker
(`internal/sink/export.go:86-190`) and never reach it.

Other Terminal results are **sink-wide** and must keep counting. The Git backend classifies its own
errors before returning to the breaker (`internal/sink/git/export.go:128-149` →
`ClassifyExportError`, `internal/sink/git/errors.go:35-42`), so a revoked token (401/403), a protected
branch or a pre-receive rejection reaches the breaker already Terminal; BigQuery and MongoDB classify
some write errors Terminal the same way. Inventories are event-driven: a collected-resource change
triggers a reconcile and a failed export is attempted again on the next one, so a sink-wide Terminal
failure repeats across every inventory on the sink. Today the breaker sheds that load (fail fast for
30 s after 5); it should keep doing so.

A related flake: `go test -race -count=3 -skip TestControllers ./internal/controller/` fails in four
reconciler tests (backlog RACE-COUNT-BREAKER-ISOLATION, P3; 4 of 6 base runs failed 2026-10, no DATA
RACE, not yet re-measured). Those tests share the `default/postgres-demo` breaker through
`newExportFailureReconciler` (`internal/controller/kollectinventory_export_test.go:339-375`): the
Terminal-failure test, the Transient-failure test and successful exports all record against one key,
so whether it opens depends on the mix and order of outcomes. The breaker change does not remove that
sharing (the Terminal there is not inventory-scoped), so this change isolates those tests.

## What Changes

- A new error marker, `kollecterrors.ErrInventoryScoped`, says "this Terminal failure is about one
  inventory, not the sink". It keeps the Terminal class and the error text.
- The ownership rejections inside the Git export plan carry the marker; T0.1 lists every Terminal
  raised inside the export plan and which of them are inventory-scoped.
- The sink circuit breaker **excludes** marked results (gobreaker `IsExcluded`): they count neither as
  failures nor as successes. Every other non-nil result keeps counting, sink-wide Terminal included.
- An excluded result during half-open neither closes nor re-opens the breaker and releases the trial
  slot.
- The reconciler tests that share `default/postgres-demo` are isolated from each other's breaker
  state, not retried.
- Docs say which errors count.

Decision: option B of three (exclude all Terminal / mark inventory-scoped Terminal only / key per
inventory). A first run chose A on the premise that Terminal exports never repeat; the spec-set review
showed that premise false (event-driven re-export, Git sink-wide Terminal at the breaker); the
corrected brief was re-run and both legs chose B (`agent-context/decisions.md`, 2026-11-05
`sink-breaker-terminal`, pending operator validation).

## Capabilities

### New Capabilities

- `sink-circuit-breaker`: which export errors a sink's shared circuit breaker counts, and how it
  behaves closed, open and half-open.

### Modified Capabilities

None.

## Impact

- Code: `internal/errors/errors.go` (marker, constructor, predicate), `internal/sink/git/prune_owned.go` and
  `prune_target.go` (marked ownership rejections, the 5 sites in `evidence/T0.1.md` table a), `internal/sink/circuit_breaker.go` (`newBreaker`, `IsExcluded`).
- Tests: `internal/errors`, `internal/sink/circuit_breaker_test.go`, a Git-backend boundary test, and
  the `internal/controller` tests named in RACE-COUNT-BREAKER-ISOLATION.
- Docs: `docs/operator-manual/troubleshooting.md:214`, `docs/adr/0602-error-taxonomy.md:38`
  (clarifying note, not a new ADR).
- Behaviour visible to users: one inventory's ownership conflict no longer opens the breaker for the
  other inventories on its sink. Sink-wide failures, Terminal or not, still open it. Export error
  metrics are recorded after the breaker (`export.go:203-212`) and are unchanged. A marked result on
  the half-open trial leaves the breaker half-open; the sink admits one export at a time until a trial
  succeeds or fails with a counted error (stated in SCB-3).
- Release note: next release, under fixes.

## Non-goals

- Changing the breaker key, trip threshold, interval or timeout.
- Reclassifying any error between Transient and Terminal.
- A breaker-state metric or condition.

## Assumptions

- The marker reaches the breaker: no layer between `invalidPrune` and `exportThroughBreaker` re-wraps
  the error without `Unwrap` (`ClassError` unwraps, `internal/errors/errors.go:42-48`). The boundary
  test in 1.1 (SCB-1e) checks this through the real Git backend, not only at the raise site (main
  risk named by a decision leg).
- A rejection is marked only when the condition is about the inventory's own paths; a rejection caused
  by shared state in the sink (e.g. corrupt prune metadata) stays unmarked so it keeps counting.
