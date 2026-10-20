## Verdict: CLEAN

## Findings

**Round-1 findings**
- H1 is closed. `k8s.io/utils/ptr` is gone from every file and the three tests use `boolPtr` (`internal/controller/secret_reader_wiring_test.go:95`, `secret_retire_envtest_test.go:89`, `secret_rotation_envtest_test.go:43`). `go.mod:224` still lists it as indirect.
- A1 is closed. When the full read, or the CA-rules read behind it, finds a Secret missing or empty, it now retires the entry with `force=true` (`internal/sink/backend_pool.go:205-210`). `retireLocked` bumps the store generation even when nothing is pooled (`:449-450`), so the fence works. The three tests in `backend_pool_full_read_test.go` pin both interleavings and the CA path.
- A2 is closed. A `0s` skip sets `lastSkipped` (`internal/controller/per_sink_export.go:89-92`) and the prune uses the later of export and skip time (`:132`). `TestPerSinkCoalesceTracker_zeroIntervalSkipsKeepStateAlive` runs to twice the TTL with a sweep at every step.
- B1 is kept as an accepted residual and pinned by `backend_pool_reread_residual_test.go`; the wording is in the design and in SCR-2. See the note below on how loosely the test pins it.
- B3 is closed. The spec now says the fence starts at the pool miss (`spec.md:149-150`). The code matches: the generation is read under the lock at the miss (`backend_pool.go:394`).
- B4 is closed. `TestClusterReconcile_zeroIntervalTerminalNoRequeue` (`zero_interval_requeue_test.go:176`) checks that the degraded reason is `ExportTerminal` and that `RequeueAfter` is 0. The code that decides it is `kollectclusterinventory_controller.go:227-229`.

**SCR rows the fixes touch**
- SCR-2 (retire on not-found or empty, whichever read sees it): **holds**. The metadata read decides at `backend_pool.go:326`, the full read and CA-rules read at `:206`.
- SCR-3 (fence and drain): **holds**.
  - A build is pooled only if nothing was stored or retired since its miss (`storePooledBackend`, `:517`).
  - A held entry is never closed (`retireLocked`, `:458`).
  - The amended last sentence is true against the code.
- SCR-7, unchanged `0s` inventory never re-exported: **holds** for the watchdog requeue. The watchdog keeps calling `shouldSkip` every 30 s through both reconcilers (`kollectinventory_controller.go:363` and `:498`, `kollectclusterinventory_controller.go:312`), and each skip renews `lastSkipped`.
- SCR-7, terminal failure not requeued: **holds** in both reconcilers (`kollectinventory_controller.go:265`, `kollectclusterinventory_controller.go:227`).
- `lastSkipped` cannot keep state alive that should age out. It is only set after the generation, checksum and `requestedAt` all match and the interval is 0. A deleted inventory or sink, or one whose generation changed, stops renewing it, so it still ages out 48 h after its last skip.
- Design check: the two new paragraphs in `design.md:88-101` match the code.

**New items from the fixes**
- [NOTE] SCR-2's new clause calls the earlier read's result "a change … observed", but in the B1 ordering the first read saw no change. It was only older than the pooled entry — `spec.md:86-87`
  Failure: rv5 is read; the Secret rotates and B6 is stored; the re-read times out; B6 is retired (`backend_pool.go:332-333`). The Secret did not change after B6 was built, so the spec wording overstates what happened. The behaviour is the accepted residual and the design states it correctly.
  Fix: change the wording to "a mismatch with the pooled entry that an earlier successful read of the same acquire saw still retires that entry".
  Confidence: 70
- [NOTE] The B1 test passes whether or not the newer entry is retired. Its `next == b6` branch accepts the outcome where no retire happened, so it does not pin the retire that the design's *Stale reads* paragraph claims — `internal/sink/backend_pool_reread_residual_test.go:54-60`
  Failure: deleting `retireEntryWithKey` at `backend_pool.go:333` leaves this test green. `_failedReReadKeepsObservedChange` still covers the older-entry case.
  Fix: either assert `next != b6` and one close after `release6`, or say in tasks 6.5 that the test accepts both outcomes.
  Confidence: 85
- [NOTE] The full-read retire can also hit an entry built from a Secret that was recreated after the read came back not-found. The cost is one drain and one rebuild, never stale use. The design's "an entry another acquire stored meanwhile is retired" covers it — `backend_pool.go:209`
  Confidence: 60 that it is worth mentioning, not a defect.
- [NOTE] Two cases outside A2 are not covered. The tracker is in memory, so a controller restart re-exports each unchanged `0s` binding once. A `0s` inventory that sits on a non-exporting degraded path (for example suspended) for more than 48 h also loses its state. Neither is caused by a timer, so neither is inside SCR-7's wording, and neither was introduced by these fixes. `troubleshooting.md:397` ("identical payloads never re-sent") may overstate this — `per_sink_export.go:130-136`
  Confidence: 50

## Could not check
- I ran no tests, mutants, `task check` or `go mod tidy -diff`. Red and green results and mutant kills are taken from `tasks.md` §6, which records them; I did not verify them.
- I did not read `pruneStaleEntriesLocked`, whether a TTL removal moves the generation, or how the generations map is pruned. `troubleshooting.md` was not re-read.
- I did not read the other round-2 reviews or the untracked `reviews/B-branch/round-2/` contents.
