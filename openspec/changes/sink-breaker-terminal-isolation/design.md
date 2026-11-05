# Design

## Context

`exportThroughBreaker(key, fn)` (`internal/sink/circuit_breaker.go:26`) wraps the export plan run in
`RunExportEnvelope` (`internal/sink/export.go:200`). The key is `req.SinkNamespace + "/" +
req.SinkName`. `breakerFor` lazily creates one `gobreaker.CircuitBreaker[struct{}]` per key in a
process-wide `sync.Map` with `MaxRequests: 1`, `Interval: 60s`, `Timeout: 30s` and `ReadyToTrip:
ConsecutiveFailures >= 5`.

Errors reaching the breaker are whatever `plan.run` returns. Some are already classified
`kollecterrors.Terminal` inside the export (ownership, identity, payload, CA/TLS configuration,
backend-specific Terminal classification in `bigquery/errors.go`, `mongodb/errors.go`). Others are
classified only after the breaker by `git.ClassifyExportError` and `classifyExportFailure`
(`export.go:207-212`), e.g. Git auth failures; those count as failures today and keep counting.

## Option chosen

A: `Settings.IsExcluded = kollecterrors.IsTerminal`. Rejected: B (tag only inventory-scoped Terminal
errors with a new marker; a forgotten tag fails back to today's defect and protects nothing extra),
C (breaker per sink and inventory; a dead backend is then hit 5×N times before any back-off and the
registry needs eviction). Decision record: `agent-context/decisions.md` 2026-11-05.

## Behaviour (gobreaker v2.4.0, read in `gobreaker.go:237-302`, `counter.go:16-41`)

- **Closed.** An excluded error increments `TotalExclusions` only. `ConsecutiveFailures` is neither
  incremented nor reset, so T, X, T, T, T, T (T Transient, X Terminal) trips on the fifth T. A success
  still resets the streak.
- **Open.** Unchanged: requests fail fast with `ErrOpenState`, mapped to a Transient "sink circuit
  breaker open" error (`circuit_breaker.go:39-41`). The export function does not run, so no
  classification happens.
- **Half-open** (`MaxRequests: 1`). `beforeRequest` admits a request while `Requests -
  TotalExclusions < MaxRequests`. An excluded trial result leaves the state half-open and frees the
  slot; the next request is admitted as the trial. A success closes the breaker; a counted failure
  re-opens it.
- `ErrTooManyRequests` (a second concurrent request in half-open) is returned unwrapped from
  `exportThroughBreaker` and becomes Transient in `classifyExportFailure`. Unchanged; not in scope.

## Testing approach

Unit tests drive `exportThroughBreaker` directly with unique keys per test (the registry is
process-wide; unique keys keep `t.Parallel()` safe) and one boundary test drives `RunExportEnvelope`
with a stub backend, as `TestRunExportEnvelope_circuitBreakerTripsAfterRepeatedFailures` does
(`circuit_breaker_test.go:26`). Half-open cases need the 30 s timeout to elapse; the test must not
sleep 30 s. The implementation may make the timeout overridable for tests through an unexported
package variable or a constructor parameter, read once at breaker creation; it must not change the
production value.

## Risks

- Mislabelled Terminal errors hide an outage from the breaker (see proposal *Assumptions*).
- A test that relies on Terminal errors tripping the breaker would now fail; none is known
  (`rg -n 'circuit breaker open|ErrOpenState' internal` at G1 records the search).
