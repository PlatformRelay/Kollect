## Verdict: CLEAN

## Findings
- [NOTE] T7.md status header is stale — says `Status: FRAMED` / `Next action: implement 7.1` while 7.1/7.2 are ticked, the matrix is all-pass, and sensors were run — `openspec/changes/dead-exported-surface/evidence/T7.md:7-8`
  Failure: a reader of the evidence file cannot tell from its own header whether the task ran; only the matrix and ticks say so.
  Fix: update the header to `Status: CLOSED (pending independent review)` when the review block is filled.
  Confidence: 95
- [NOTE] Matrix row 11 claims the commit "additionally carries the evidence file, the tasks tick and the review register" — the register is not in the commit; `openspec/changes/dead-exported-surface/reviews/L-tasks/T7/` is untracked at HEAD — `git status` (2026-10-07)
  Failure: scope claim over-states the commit contents by one artifact; if it is meant to be landed by T8, the row should say so.
  Fix: reword row 11 to "evidence file + tasks tick (register landed separately/by 8.x)" or commit the register with the record task.
  Confidence: 80
- [NOTE] Nil-cache `get` collapse is a semantic no-op but drops the distinction between "nil cache" and "expired entry" that the old 3-tuple also erased anyway; no caller can observe the difference — `internal/inventory/auth_cache.go:32-46`
  Failure: none found — hit path (`auth.go:115`) only branches on `ok`, and both nil-cache and expired now return `(false,false)` exactly as before.
  Fix: none.
  Confidence: 90

What I checked (not "no issues found" by assertion): full diff 7ce8ab57..HEAD (4 files); post-change `auth.go` and `auth_cache.go` in full — hit path serves on `allowed` iff `RequireInventoryGet` gating, miss path's `user` still feeds `authorizeResource` at `auth.go:137`, `set` carries only `allowed`; repo-wide grep at HEAD: no `_ = user`, no other caller of `authCache.get/set` (the `ref_cache_test.go` hits are a different package's type), `authenticationv1` import remains only where TokenReview/SAR need it; probe record cross-checked against `git grep 7ce8ab57` — all counts (1 Go `_ = user`, 2 cache call sites, 4 entry refs, 7 import refs, 1 test ref) reproduce; tests: `TestAuthMiddlewareCacheHitsTokenReviewOnce` guards the positive hit path, and the cached-denial test at `internal/inventory/auth_test.go:263-286` guards the 403-on-hit branch, so both changed branches are covered without test edits; re-ran `go build ./internal/inventory/...`, `go vet ./internal/inventory/...`, `go test ./internal/inventory/...` — all green. Behaviour is genuinely identical: with `RequireInventoryGet=false` the miss path always sets `allowed=true`, so the removed `user` value could never influence any branch.

## Could not check
- Repo-wide `go build ./...` / `go vet ./...`, `task arch-lint`, `task test/lint/coverage` — ran only the package-scoped trio (T8 is the final-gates task; evidence claims exit 0 for the full set).
- Whether the untracked `reviews/L-tasks/T7/` register is deliberately deferred to task 8.x — orchestrator intent not in-repo.
