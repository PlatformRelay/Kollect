# Design

## Context

`exportThroughBreaker(key, fn)` (`internal/sink/circuit_breaker.go:26`) wraps the export plan run in
`RunExportEnvelope` (`internal/sink/export.go:200`). The key is `req.SinkNamespace + "/" +
req.SinkName`. `breakerFor` lazily creates one `gobreaker.CircuitBreaker[struct{}]` per key in a
process-wide `sync.Map` with `MaxRequests: 1`, `Interval: 60s`, `Timeout: 30s` and `ReadyToTrip:
ConsecutiveFailures >= 5`.

Three zones (`internal/sink/export.go`):

- **Before the breaker** (`:79-190`): registry/spec checks, inventory identity (`:86`), backend
  acquire incl. construction and CA/TLS config (`:92-110`), payload decode, size and layout. These
  return early and never touch the breaker. Unchanged.
- **At the breaker** (`:200`): whatever `plan.run` returns. The class is
  `kollecterrors.ClassOf(err)` (`internal/errors/errors.go:86-108`): an explicit `ClassError`, else a
  Kubernetes API status (NotFound/Invalid/BadRequest → Terminal, Forbidden → Forbidden), else
  Transient. Terminal results here include the Git ownership/prune rejection
  (`git/prune_owned.go:255`) and backend-classified write errors (`bigquery/errors.go`,
  `mongodb/errors.go`).
- **After the breaker** (`:207-212`): `git.ClassifyExportError` and `classifyExportFailure`. Errors
  that become Terminal only here (e.g. Git auth failures, `git/errors.go:35-36`) were not Terminal at
  the breaker and keep counting.

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

Unit tests drive `exportThroughBreaker` directly and one boundary test drives `RunExportEnvelope`
with a stub backend, as `TestRunExportEnvelope_circuitBreakerTripsAfterRepeatedFailures` does
(`circuit_breaker_test.go:26`).

Isolation. The registry is process-wide and `ResetBreakersForTest` clears every key
(`circuit_breaker.go:68-70`). Rules: (1) a key is unique per test *invocation*, not per test name,
because `-count=N` reruns a test against the breakers its earlier run left behind (e.g. `t.Name()`
plus a package-level atomic counter); (2) no `t.Parallel()` test calls `ResetBreakersForTest`, so
the existing `TestResetBreakersForTest_clearsOpenBreaker` (`circuit_breaker_test.go:95-96`, parallel,
resets at `:155`) becomes non-parallel in G1. The other callers (`release_on_delete_test.go`,
`inventory_identity_test.go`, `git/owned_prune_fuzz_test.go`) are checked in G1 for the same rule and
listed in the evidence.

Timeout seam. Half-open cases must not sleep 30 s. The only allowed seam is a constructor:
`newBreaker(key string, timeout time.Duration)`, which `breakerFor` calls with
`circuitBreakerTimeout`; a test builds a short-timeout breaker and stores it under its own unique key
with `breakerRegistry.LoadOrStore` before the first call. No mutable package variable (a write from
one parallel test is a data race with `breakerFor` in another and would leak into their breakers).
State assertions read `breakerFor(key).State()`.

## Risks

- Mislabelled Terminal errors hide an outage from the breaker (see proposal *Assumptions*).
- A test that relies on Terminal errors tripping the breaker would now fail; none is known
  (`rg -n 'circuit breaker open|ErrOpenState' internal` at G1 records the search).
