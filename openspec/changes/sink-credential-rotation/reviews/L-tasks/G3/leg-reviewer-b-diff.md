## Verdict: CLEAN

## Findings
- [NOTE] The generation age-out's `!pooled` condition is not covered by any test — `internal/sink/backend_pool.go:499`
  Failure: Suppose a mutant drops `!pooled`, or uses the entry's age instead of `generation.changed`. The generation of a busy key is then deleted one TTL after its store and reads as 0. An acquire that recorded the older value skips a retire it was due to make, so the stale entry is retired one acquire later. None of the 12 new tests and none of the mutants M1–M10 exercise this sweep, so the mutant would most likely survive. Hits still need matching metadata, so SCR-2 holds and the cost is only a later retire.
  Fix: Add a test that keeps an entry pooled and touched past `backendPoolTTL` after its store, then checks that `generations[key]` is still present. Also add the matching mutant to the gate.
  Confidence: 60
- [NOTE] When the stale-read guard skips, the pool keeps the stale entry and does not store the correct build — `internal/sink/backend_pool.go:193-194`, `:349`
  Failure: Acquire A records g0. A builder whose read predates the rotation stores v6 at g1. The Secret then moves to v7, and A reads v7. Because the generation changed, A sets `mayStore=false` and does not retire. A builds v7 under an owning release, so the v7 build is closed after one export and v6 stays pooled until the next acquire retires it and rebuilds. This is not a SCR-2/SCR-3 breach. A hit always needs a fresh metadata read that matches the entry key (`:186`), so no read that observed v7 can be handed v6. The guard only changes when a retire happens, never whether a stale entry can be hit. I checked this for the key-change path and the not-found path (`:233`). The cost is one extra build per such interleaving.
  Fix: None required. Optionally, add one sentence to the evidence noting that the guard is sound because hit eligibility never depends on the generation.
  Confidence: 85
- [NOTE] Two paths in the brief's Close-exactly-once list have no direct test — `internal/sink/backend_pool.go:100-108`, `:573-579`
  Failure: First, `ResetBackendPoolForTest` closes held entries and resets `retired`. A later release should then close nothing; this relies on the `retired=false` line, which no test covers, so removing it would double-Close with no test failing. Second, `evictPoolKey` (explicit eviction) with an active holder is untested. It goes through the shared `retireLocked`, so the risk is low.
  Fix: Add a test that acquires, calls Reset, releases, and asserts the backend was closed once. Add one explicit-eviction drain case.
  Confidence: 70

Checked and found to hold:
- **Close-once paths.** Each path closes once, outside `mu`: hit (`holdLocked`, `sync.Once`), share (`:338`, surplus closed once by the caller at `:246`), fenced build and tombstone build (`owningRelease`), retire with and without holders, TTL (held entries are skipped at `:470`), and delete or explicit eviction.
- **Release reads after Unlock.** The release's read of `retireReason` and `backend` after Unlock is ordered by the mutex. An entry is retired only once, because it leaves the map in the same step.
- **Callers.** Both production callers `defer release()` (`export.go:112`, `cleanup.go:208`). A nested acquire under an outer hold just takes a second hold, so there is no deadlock.
- **Fence order.** The store fence checks tombstone, then same-key share, then unchanged generation and `mayStore`, then falls back to the owning release. This matches the design. Generation values come from one counter and are never reissued. A value that ages out back to 0 fails closed: the build is not pooled.
- **Concurrency test on base.** The claim that `_concurrentAcquiresAfterRotation` passes on base is plausible. On base a hit returned a no-op release and nothing held B1, so the test is load-bearing only through mutants M5 and M9.
- **Secrets.** The tests use only sentinel values. No Secret value reaches a log line or an error message added by this change.

## Could not check
- I did not run any tests, the race run or the mutation gate (my access is read-only, git only). The mutation results and the line numbers of the red runs are taken from `evidence/G3.md` as recorded.
- I did not read `design.md`, `tasks.md` or the SCR spec text, so intent was judged from the brief and the evidence. I also did not read the helpers in the rotation test files (`mustAcquire`, `enableRotationPool`, `setSecretData`, `pooledEntryFor`).
- The envtest and integration pool specs (FINAL row 19) are still pending and were not run.
