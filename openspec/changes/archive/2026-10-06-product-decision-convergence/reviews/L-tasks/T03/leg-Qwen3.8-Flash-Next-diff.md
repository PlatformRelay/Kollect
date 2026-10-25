## Verdict: CONCERNS

## Findings
- [WARNING] The BEP-1 empty-UID fallback branch (`ns/name` key when the delete event carries no UID) has no test in the red set or any recorded plan — `internal/sink/backend_pool_delete_hook_test.go:39` (all four tests use non-empty UIDs); the seam's own doc at `internal/sink/backend_pool.go:244-249` promises the fallback, T03.md's matrix has no row for it, and T08 (`tasks.md:55-58`) names no test task for it.
  Failure: `acquireBackend(ctx, cl, reg, ns, name, "", spec)` pools under `ns:team-a/x` (`backend_pool.go:84`); a T08 seam body that only ever evicts `uid:` keys ships green against all four T03 tests while BEP-1's "SHALL fall back" clause is silently dead.
  Fix: add a fifth test now — pool with `uid=""`, call `EvictBackendPoolForSink("", ns, name)`, assert entry gone + Close ran; it is red on the no-op stub exactly like the required two, so it costs nothing at this phase.
  Confidence: 85
- [NOTE] The commit-range evidence is not in git at `f0e82035`: `evidence/T03.md` and `reviews/…/T03/` are untracked (`??` in `git status`), and the T03 tick in `tasks.md:26` is unticked — consistent with T02's two-commit closeout pattern, but the verification matrix the commit's claims rest on is only on the worktree, not the branch, until the follow-up docs commit lands.
  Fix: ensure the T02-pattern closeout commit (evidence + tick) is part of the same PR, not left behind on close.
  Confidence: 90
- [NOTE] Three eviction entry points now coexist (`EvictBackendPool` `backend_pool.go:227`, `EvictBackendPoolByUID` `:236`, and the exported do-nothing `EvictBackendPoolForSink` `:250`); a caller wiring the seam into a watch before T08 fills it gets silent no-eviction with no compile or lint signal. Accepted cost of the requested TDD stub — call it out in T08's diff so the stub window cannot outlive the change.
  Confidence: 70
- [NOTE] The diff measurably raises exposure of a real, pre-existing `-race` failure: `ResetBreakersForTest` reassigns the global `breakerRegistry` (`circuit_breaker.go:67`) inside `t.Parallel()` tests (`circuit_breaker_test.go:21,78`), and the four new sequential tests shift parallel-batch scheduling (evidence: 2/5 red runs here vs 0/4 at base). Correctly classified as out of T03's scope, but the package's race sensor is now flaky on every PR until a harness task fixes it.
  Fix: `TestResetBreakersForTest_clearsOpenBreaker` non-parallel, or clear the map in place.
  Confidence: 80

## What I checked
- Full diff `33c6812d..f0e82035` (2 files, +247) against `internal/sink/backend_pool.go` call/store/evict machinery, `poolKeyForSink`, `storePooledBackend` swap semantics, and the `closeCountBackend` fixture (`backend_pool_test.go:31-44`).
- Ran `go test -count=1 -run TestEvictBackendPoolForSink ./internal/sink`: fails at exactly the two claimed lines (`:66`, `:141`) with the evidence-quoted messages; the two guard tests genuinely pass (verified logic: absent-key no-op; spec-hash swap closes v1, keeps v2 open — matches `storePooledBackend:174-186`).
- Signature vs BEP-1 spec delta (`specs/backend-pool/spec.md`): shape matches, void return correctly enforces "SHALL NOT fail"; task text clauses all map to a test except the fallback above.
- Test hygiene: no `t.Parallel()` in the new file (no interleaving hazard), buffered goroutine channels, 5s timeouts, `t.Cleanup(ResetBackendPoolForTest)`; no test-order dependency found.

## Could not check
- T03.md's full-package `-race -count=2`, `task lint`, and `go vet` runs (slow; I ran only the targeted test and read the claims — the base-vs-tree flake-rate measurement of 0/4 vs 2/5 is taken on trust).
- Whether T08 or any later task records a plan to cover the empty-UID fallback beyond the seam's doc comment — no such task exists in `tasks.md` as of this tree.
- CI/integration and `task verify` (not run here; evidence records them as not-run with reasons).
