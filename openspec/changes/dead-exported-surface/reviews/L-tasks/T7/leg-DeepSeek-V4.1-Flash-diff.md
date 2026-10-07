## Verdict: CLEAN

## Findings
- [NOTE] Evidence state fields contradict the committed completion: `Status: FRAMED` and `Next action: implement 7.1, then task-tier sensors`, while `tasks.md` ticks 7.1/7.2 and DR-8 `done`, and commit `34be68c8` has already landed — `openspec/changes/dead-exported-surface/evidence/T7.md:7-8`
  Failure: a reader of the committed change sees the task claimed done in tasks.md but the evidence file still saying "FRAMED / implement"; every sibling T1–T6 finalises these to `CLOSED`. (T3 flagged and fixed exactly this class.)
  Fix: finalise Status/Next-action and fill Independent review + Verdict at CLOSE, as T1–T6 do.
  Confidence: 90 (real; harmless to behaviour, and the loop expects finalisation at close)
- [NOTE] Two evidence line anchors drift from the code they cite — `evidence/T7.md:17,82`
  Failure: line 82 cites `authorizeResource :178`; the function signature is `auth.go:176` (`:178` is the `user authenticationv1.UserInfo` parameter line). Matrix row 4 cites the `authorizeResource` call at `auth.go:138`; it is at `auth.go:137`.
  Fix: correct the two anchors to `:176`/`:137`.
  Confidence: 95 (verified against the read file)

## Could not check
- Did not run the holistic gates DR-9 defers to task 8 (`task test`, `task lint`, `task coverage`, `task spec:validate`) — out of scope for a single task; `go build`/`go vet`/`go test ./internal/inventory/...` I did run green (cached).
- External consumers of the change: all touched symbols (`authCacheEntry`, `get`, `set`) are unexported, so none are reachable outside the module — verified by `git grep`, not by a downstream build.
- The pre-decided loop rows (tasks.md preamble; R1 #6; R2 #9) were treated as settled and not re-litigated.

What I actively checked and found correct: `_ = user` and the discarded `user` binding are gone repo-wide in Go (`git grep`); `auth_cache.go` has zero `authenticationv1` refs and the import is dropped; `auth.go` retains import `:16` and uses it at `:157,158,159,166,170,178`; the only `.cache.get(`/`.cache.set(` call sites are `auth.go:115,145`; no test constructs an `authCacheEntry` or calls `get`/`set` directly (`auth_cache_test.go` only calls `newAuthCache(0)`); the cache-hit decision logic (`RequireInventoryGet && !allowed` → 403, else serve) is byte-identical to the pre-sha form, and the miss path is untouched, so the refactor is behaviour-preserving; commit message matches the change.
