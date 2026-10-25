## Verdict: CONCERNS

## Findings
- [WARNING] BEP-1's normative clause "no new acquire SHALL rebuild an entry for that sink's UID" (`specs/backend-pool/spec.md:15`) has no test and no matrix row, and no task owns it — T08's text (`tasks.md:55-58`) names only "discard in-flight re-stores". This is the same class of gap round 1 accepted for the fallback clause (evidence rows split only task-text clauses, not the delta's SHALLs).
  Failure: T08 ships a tombstone that GCs immediately or keys only the in-flight race; a post-eviction acquire silently re-pools the deleted sink's UID; nothing fails.
  Fix: add one red test (acquire uid → evict → acquire same uid → assert no entry, mirroring `TestEvictBackendPoolForSink_evictsEntryAndCloses`), or record in evidence/design why the clause is deliberately untested and who owns it.
  Confidence: 70
- [WARNING] The change ships intermittent `-race` CI red: evidence measures the pre-existing `breakerRegistry` race (parallel `circuit_breaker_test.go:78` writes the global map vs `circuit_breaker_test.go:21` reads via `exportThroughBreaker`) at 0/4 runs on base vs 2/5 on this tree — the new sequential tests perturb parallel-batch scheduling. Merging makes branch CI flaky even though the defect lives in another file; "recorded as harness gap" still lands the flake with this PR.
  Failure: `go test -race ./internal/sink` on CI fails ~2/5 with `race detected`, red unrelated to the task, blocking every subsequent task's gate.
  Fix: drop `t.Parallel()` at `internal/sink/circuit_breaker_test.go:78` so the map-replacing test runs in the sequential phase (one line, same package).
  Confidence: 60 (measurements are from the evidence log; I could not re-run)
- [NOTE] `TestEvictBackendPoolForSink_inFlightBuildDiscardedNotRepooled` never asserts what the racy acquire *returns* — it reads the `acquired` channel but discards the value (`backend_pool_delete_hook_test.go:129-131`). Spec sanctions it (`spec.md:14`: in-flight holders "may fail against it"), so a T08 that returns the same closed backend to a live caller passes this test with `built.closes==1`.
  Failure: not a current failure — a pinned-in-advance ambiguity for T08 (return nil vs return closed backend).
  Fix: assert the returned Backend is non-nil (or add a comment stating nil-is-acceptable).
  Confidence: 75
- [NOTE] `TestEvictBackendPoolForSink_noPooledEntryIsNoOp` (`backend_pool_delete_hook_test.go:205-220`) checks only `len(entries)` before/after; an implemented seam that wrongly closed a *different* key's backend while leaving the map size unchanged passes. Weak-but-green guard.
  Fix: pool a sentinel under an unrelated key and assert its `closes==0` after the absent-key eviction.
  Confidence: 60
- [NOTE] Tests set the pool flag via `backendPoolDisabled.Store(false)` (`backend_pool_delete_hook_test.go:29` etc.) instead of the package's own `EnableBackendPoolForTest()` used by every other test; behaviourally identical, inconsistent convention only.
  Confidence: 90 (style, not defect)

Checked in full: both new files line-by-line; `backend_pool.go` (pool machinery, `storePooledBackend:159-187`, `evictPoolKey:253-263`, pruning); the BEP-1/BEP-2 delta; both commits of `33c6812d..0632218d` (second commit adds only the fallback test, no production change); red/green classification of all five tests traced by hand against the no-op stub (3 red + 2 green guards — matches the task text and matrix); "production acquire path" claim verified at `export.go:170`/`cleanup.go:184`; test-ordering and global-state leakage between the new file's tests and sibling tests (unique keys, cleanup resets pool; flag left at default); license headers and commit style.

## Could not check
- Did not run `go test`, `go vet`, `golangci-lint` or `task` targets (plan-mode read-only); all pass/fail/race-clean claims are taken from `evidence/T03.md` logs, not reproduced.
- The breaker-race reproduction rate (0/4 vs 2/5) is the evidence's measurement; unverified.
- Other reviewers' T03 registers under `reviews/L-tasks/T03/` — out of bounds by brief; did not read.
- The `proposal.md:129-130` "spec-fixed shape" citation vs the chosen signature beyond `spec.md` itself; T08's implementation choices (tombstone GC, return-on-race) do not exist yet.
