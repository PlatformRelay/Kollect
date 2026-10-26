## Unified verdict: BLOCK (legs ok: 2/2)
| # | Sev | Claim (one line) | Where | Models | Legs | Conf |
|---|---|---|---|---|---|---|
| 1 | CRITICAL | New `Watches` DeleteFunc wiring is never exercised end-to-end: hook tests call `evictBackendPoolOnSinkDelete` directly and the envtest spec only asserts `SetupWithManager` returns nil, so deleting the entire `Watches(...)` clause keeps every gate green — the task's central claim is unpinned | `internal/controller/family_sink_controller.go:104-111` | 2 | 2 | 100 |
| 2 | WARNING | Delete path can `Close()` a backend still in use: a sink delete mid-export closes the backend under a concurrent export, and the tombstone path returns a closed backend whose `Export` may then panic (inherent to BEP-1 without refcounting; spec sanctions "may fail", not panic) | `internal/sink/backend_pool.go:157,160,176,291-302`; `internal/sink/export.go:170-189` | 2 | 2 | 75 |
| 3 | WARNING | Tombstone TTL-aging branch has no sensor: matrix rows 1–15 never cover `pruneStaleEntriesLocked` dropping a stale tombstone vs keeping a fresh one, so a regression dropping the aging loop or aging too early passes every listed gate | `internal/sink/backend_pool.go:230-234` | 1 | 1 | 90 |

## Disagreements
- Tombstone aging: DeepSeek demands a test pinning the aging branch (WARNING, conf 90); Qwen found the neighbouring growth window (prune only runs on `acquireBackend`) but judged it "acceptable as-is" (NOTE) — kept as separate defects, so no promotion fired.
- Closed-backend use: DeepSeek flags possible `Export` panic after `Close` (fix: verify Close-safety or return a terminal error); Qwen calls it an inherent, likely-accepted BEP-1 window needing only a spec sentence — neither leg resolved acceptability.

## Nobody could check
- Full gate matrix unverified by any leg: `-race -count=1/-count=2` full suites, envtest, `task lint`, golangci/go-arch-lint, `task verify`/`test-integration`, gitleaks — every exit code in `evidence/T08.md` taken on faith (Qwen re-ran only targeted `-run` subsets).
- `breakerRegistry` pre-existing race signature: documented in evidence, never reproduced by either leg.
- `Backend` implementations' `Export` Close-safety: no `Export`/`Close` methods found in `internal/sink/*.go` root, so finding 2's panic risk is unconfirmed.

Dropped as single-leg NOTEs: matrix row 11's sensor over-claim (DeepSeek), wrong comment path for the unwrap claim (Qwen), `tasks.md` tick promise (Qwen), "only the delete hook records tombstones" untested (DeepSeek), tombstone growth on delete-only controllers (Qwen).
