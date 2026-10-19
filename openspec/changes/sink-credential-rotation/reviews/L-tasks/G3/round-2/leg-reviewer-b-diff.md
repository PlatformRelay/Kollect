## Verdict: CLEAN

## Findings
- [NOTE] The forced second read can still retire an entry stored after that read began, which costs one extra rebuild but does not leave a stale backend in use — `internal/sink/backend_pool.go:306`, `:366-373`
  Failure: acquire A's first lookup sees the generation changed and reads again. A third acquire stores E2 between A's second `poolGenerationOf` and A's second lookup. With `force` set, A retires E2 even though its read may be older than E2. The rebuild then runs `fetchSecrets` after the retire, so A's backend is at least as new as E2 and E2's holders drain. No correctness loss, only an extra Close and rebuild under heavy churn.
  Fix: none needed. Optionally say in the `poolReadAttempts` comment that the bound accepts this.
  Confidence: 70
- [NOTE] With two concurrent builds at different versions on a key with no entry, the first store wins, not the last — `internal/sink/backend_pool.go:479-494`
  Failure: A misses at v5 and B misses at v6. Both record the same generation. If A stores first, B's v6 build goes back under `owningRelease` and the pool holds v5 until the next acquire's read retires it (gen unchanged → `default` branch). This converges, but the evidence calls it "last writer wins" (G1 b-N2, row 5), and row 5 tests only the order where the newer build stores first.
  Fix: reword row 5 to "first store wins; the next acquire's read retires an older one". Optionally add the reverse-order assertion.
  Confidence: 75
- [NOTE] A generation is not refreshed when it is read, so it can age out while a build is in flight. The build is then not pooled once — `internal/sink/backend_pool.go:340-344`, `:545-547`
  Failure: a key with no entry whose last bump was just under `backendPoolTTL` ago. An acquire records that value, and while it builds, another sink's prune drops the generation. `storePooledBackend` reads 0 ≠ recorded, so the build is returned unpooled. This is safe (the fence errs closed). The extra cost is one rebuild.
  Fix: none required. Optionally refresh `changed` in `generationLocked`.
  Confidence: 60
- [NOTE] Release does not refresh `lastUsed`, so a hold longer than the TTL is pruned on the first prune after its release — `internal/sink/backend_pool.go:441-451`, `:517`
  Failure: only an export held for more than 48 h. After the release, the entry is closed once and rebuilt. No double Close.
  Fix: none needed. Optionally set `entry.lastUsed = timeNow()` in the release.
  Confidence: 55

Checked, and found to hold:
- **Close exactly once on every path.**
  - Hit and share: `holdLocked`.
  - Store: fenced under the generation.
  - Tombstone and superseded builds: `owningRelease` with `sync.Once`.
  - Retire with and without holders, TTL, delete eviction, test evictions: all through `retireLocked`, which deletes from the map, so an entry is never held after it is retired.
  - Reset followed by a late release: `retired=false`.
- **No Close under the mutex** on any production path; `ResetBackendPoolForTest` is test-only and disclosed in row 14.
- **No deadlock:** the release takes the mutex only on its own, and acquire never waits on holders.
- **Fence order:** tombstone, then same-key share, then unchanged generation (never 0), then the owning release.
- **Generations:** counter-based and never reissued. Reset clears the map but not the counter.
- **Production callers** `defer release()` (`internal/sink/export.go:112`); production passes a `SecretReader`.
- **Tests:** the reds are on their target assertions. `concurrentAcquiresAfterRotation` cannot re-read: nothing is pooled after the first retire, so the barrier count holds. The task's mutation list (M1, M2, M3, no-op control) is all present.
- **Round-1 dispositions:** the P0 fix (re-read, last attempt acts) and the P1 fix (nonzero fresh generation) both hold. The rejection of the `!pooled` NOTE holds too: a pooled key's generation persists, and dropping it only forces a re-read.
- **Secrets:** no Secret value reaches a log, error or pool entry. Log reasons are static strings, and test values are `sentinel-*`.

## Could not check
- I did not run any tests, the race run, the mutation gate or lint. I relied on the exit codes recorded in `evidence/G3.md`. The FINAL full suite and envtest specs are still pending there.
- I did not read `tasks.md` 3.1 past line 239 in full, `design.md` *Pool entries with holders* / step 4, or `specs/sink-secret-rotation/spec.md` (SCR-3). Intent was judged against the brief, the evidence and the task lines I read.
- I did not read the helpers (`mustAcquire`, `acquireErr`, `enableRotationPool`, `pooledEntryFor`, `setSecretData`), `cleanup.go:194-207`, or the `pruneRelationalExport` nested-acquire path.
- The round-1 review records under `reviews/L-tasks/G3/` were not opened.
