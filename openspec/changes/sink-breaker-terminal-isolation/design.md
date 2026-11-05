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
- **At the breaker** (`:200`): whatever `plan.run` returns. Backends classify inside the plan: the
  Git backend returns `ClassifyExportError(...)` (`git/export.go:128-149`), so Git auth (401/403),
  protected-branch and pre-receive rejections arrive Terminal (`git/errors.go:35-42`); BigQuery and
  MongoDB classify some write errors Terminal (`bigquery/errors.go`, `mongodb/errors.go`). The
  ownership/prune rejection arrives Terminal through `invalidPrune` (`git/prune_owned.go:254-256`,
  25 call sites).
- **After the breaker** (`:207-212`): `git.ClassifyExportError` again and `classifyExportFailure`,
  for errors the plan returned unclassified.

Inventories reconcile on collected-resource events, and a failed export is retried on the next
reconcile, so a sink-wide failure repeats across every inventory on the sink regardless of class.

## Option chosen

B: mark inventory-scoped Terminal results and exclude only those. Rejected: A (exclude every Terminal;
drops load-shedding for sink-wide Terminal failures such as a revoked Git token, which repeat on every
event), C (breaker per sink and inventory; a dead backend is hit 5×N times before any back-off and the
registry needs eviction). A was chosen first on a false premise and reversed; decision record
`agent-context/decisions.md` 2026-11-05 `sink-breaker-terminal`.

## Marker

In `internal/errors`:

- `var ErrInventoryScoped = errors.New("inventory-scoped")`.
- `func InventoryScoped(err error) error`: nil for nil; otherwise an error that is Terminal
  (`ClassOf` = Terminal), satisfies `errors.Is(_, ErrInventoryScoped)`, keeps `errors.Is`/`errors.As`
  to everything `err` wrapped, and has exactly `err.Error()` as its text (status messages and logs do
  not change).
- `func IsInventoryScoped(err error) bool { return errors.Is(err, ErrInventoryScoped) }`.

At the raise sites: T0.1 classifies the 25 `invalidPrune` calls and every other Terminal raised inside
an export plan as inventory-scoped (about the inventory's own paths or its ownership claim) or
sink-wide/shared (e.g. corrupt or unreadable prune metadata, repository state). G1 routes the
inventory-scoped ones through a new helper (e.g. `ownershipConflict(format, args...)` returning
`kollecterrors.InventoryScoped(...)`) and leaves the rest on `invalidPrune`. A site whose class T0.1
cannot settle stays unmarked (falls back to today's behaviour) and is listed in the evidence.

Breaker: `IsExcluded: kollecterrors.IsInventoryScoped`.

## Behaviour (gobreaker v2.4.0, read in `gobreaker.go:237-302`, `counter.go:16-41`)

- **Closed.** An excluded result increments `TotalExclusions` only. `ConsecutiveFailures` is neither
  incremented nor reset, so T, X, T, T, T, T (T Transient, X marked) trips on the fifth T. A success
  still resets the streak. The closed-state counts reset when the 60 s `Interval` expires
  (`gobreaker.go:304-309`), independent of exclusions; unchanged.
- **Open.** Unchanged: requests fail fast with `ErrOpenState`, mapped to a Transient "sink circuit
  breaker open" error (`circuit_breaker.go:39-41`). The export function does not run.
- **Half-open** (`MaxRequests: 1`). `beforeRequest` admits a request while `Requests -
  TotalExclusions < MaxRequests`. An excluded trial leaves the state half-open and frees the slot; the
  next request is admitted as the trial. A success closes the breaker; a counted failure re-opens it.
- `ErrTooManyRequests` (a second concurrent request in half-open) is returned unwrapped from
  `exportThroughBreaker`, passes `git.ClassifyExportError` (for every sink type, `export.go:208`),
  where it is wrapped Transient as "git transport: too many requests", then `classifyExportFailure`.
  Pre-existing and out of scope (misleading message only; class is Transient); deferred to the
  backlog.

## Testing approach

Unit tests drive `exportThroughBreaker` directly; boundary tests drive `RunExportEnvelope` with a stub
backend (as `TestRunExportEnvelope_circuitBreakerTripsAfterRepeatedFailures`,
`circuit_breaker_test.go:26`; the existing `stubBackend` returns one error for every path, so SCB-1b
needs a stub that branches on the object path), and one test drives a real Git ownership rejection
through `RunExportEnvelope` with the local-repository fixtures the Git ownership tests already use
(`internal/sink/inventory_identity_test.go`, `internal/sink/git/*owned*_test.go`; T0.1 names the
fixture).

Isolation. The registry is process-wide and `ResetBreakersForTest` clears every key
(`circuit_breaker.go:68-70`). Rules: (1) a key is unique per test *invocation*, not per test name,
because `-count=N` reruns a test against the breakers its earlier run left behind (e.g. `t.Name()`
plus a package-level atomic counter); (2) no `t.Parallel()` test calls `ResetBreakersForTest`; the
existing `TestResetBreakersForTest_clearsOpenBreaker` (`circuit_breaker_test.go:95-96`, parallel,
resets at `:155`) becomes non-parallel in G1; the other callers are non-parallel (round-2 review,
verified again in T0.1); (3) a non-parallel test that depends on a clean breaker resets at its start
and in `t.Cleanup`.

Constructor seam. `newBreaker(key string, timeout time.Duration) *gobreaker.CircuitBreaker[struct{}]`
holds all settings; `breakerFor` calls it with `circuitBreakerTimeout`. G1 extracts it first as a pure
refactor (settings unchanged), so the SCB-3 tests compile on a base that behaves like `origin/main`
and fail on their target assertion. A test builds a short-timeout breaker and stores it under its own
key with `breakerRegistry.LoadOrStore` before the first call. No mutable package variable.

Timing protocol for half-open tests: timeout 1 s; sleep 1.1 s once to reach half-open; every later
assertion runs without sleeping, well inside the next 1 s window, so a wrongly re-opened breaker
cannot drift back to half-open before `State()` is read, and a correctly re-opened one cannot either.
State is read with `breakerFor(key).State()` immediately after each call.

## Risks

- The marker is lost by a re-wrap without `Unwrap` between the raise site and the breaker: covered by
  SCB-1e through the real Git backend (CLI engine) and B1.1 (per-site table and the go-git
  return path).
- A shared-state rejection marked as inventory-scoped would stop shedding load for a sink-wide fault:
  T0.1's per-site classification, reviewed in G1.
- A test that relies on ownership rejections tripping the breaker would now fail; none is known
  (`rg -n 'circuit breaker open|ErrOpenState' internal` at G1 records the search).
