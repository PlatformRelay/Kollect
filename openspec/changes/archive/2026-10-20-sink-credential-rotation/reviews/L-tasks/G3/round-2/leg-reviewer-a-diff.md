## Verdict
verdict: BLOCK
findings:
  - [P0] internal/sink/backend_pool.go:319 — A reads v6 while a concurrent builder stores B5; lookup requests a re-read, but a timeout leaves B5 pooled and available to delayed v5 acquires, violating SCR-3 despite the successful first observation.
  - [P1] internal/sink/backend_pool.go:306 — The forced second read can itself become stale: read v6, concurrently store B7, then resume; force bypasses the generation guard and retires B7. The deferred G2 defect remains.
matrix_disputes:
  - Rows 11–12b cover neither a failed re-read nor another store during that re-read.
  - M14 explanation (evidence/G3.md:130) — pooled entries already have retired=false; removing the assignment preserves the late-release assertion.
  - Rejected generation-pruning test — “only one extra GET” overlooks that the extra GET can fail, affecting retirement.
could_not_check:
  - Scratch mutations/base execution: filesystem policy is read-only; base-green concurrency claim is consistent with code inspection.
  - Full suite remains pending as declared.
summary: Focused tests pass, including race; holders and store fencing look sound, but the re-read fix still violates retirement guarantees.
