## Unified verdict: CONCERNS   (legs ok: 3/3)

| # | Sev | Claim (one line) | Where | Models | Legs | Conf |
|---|---|---|---|---|---|---|
| 1 | CRITICAL | Evict-during-use NATS leak: pool closes a backend mid-export; `jetStream()` redials and caches a fresh `nc` nothing ever Closes — process-lifetime connection/goroutine leak per delete-mid-export (tracked as T12 owner-gated residual, so CRITICAL = confirmation, not a new miss) | `internal/sink/backend_pool.go:141` + `internal/sink/nats/backend.go:116-157` | 2 (DeepSeek-V4.1-Flash, Qwen3.8-Flash-Next) | diff, adversarial | 100 |
| 2 | WARNING | Cluster target never requeues or watches collected resources → `status.collectedCount`/Ready prose freeze between target/Namespace/Profile/Scope events, unlike the namespaced path's liveness requeue | `internal/controller/kollectclustertarget_controller.go:400` vs `kollecttarget_controller.go:359` | 1 | diff | 85 |
| 3 | WARNING | Evicted backend `Close()` runs inline on the informer dispatch goroutine; a slow backend Close stalls subsequent sink events of that kind | `internal/controller/family_sink_controller.go:106` → `internal/sink/backend_pool.go:328` | 1 | diff | 70 |

Dropped as single-leg NOTEs: stale Verification-table rev `81bea0b0` (tasks.md:384-396), `configFaultsAreTerminal` broader than GTE-1, ns/name-tombstone fallback can discard legit builds (D4), loop.md:12 still `<pending>`.

## Disagreements
- #6 (entry 2): spec leg ran the suites at HEAD and judged the cadence spec-silent — count was already event-refreshed pre-branch; diff leg alone calls it a WARNING.
- #8 (entry 3): adversarial leg argues the blast radius is overstated (delete-watch listener's `add` is non-blocking and enqueues nothing, so the `For()` reconcile path is not stalled); deferral sound, only its paper trail missing.

## Nobody could check
- `task verify` / `task lint` / coverage floor / `-race` at HEAD — no leg executed the gates; all greens are implementer records (Qwen-2.4T did re-run unit suites at `3bc38bdc`).
- Integration tier (`task test-integration`, Docker) — no leg ran it.
- `-count=2` non-idempotency repro at base `3ee21266` — "pre-existing" still unconfirmed by anyone.
- Whether x/crypto v0.57.0 actually registers the two added KEX names — module cache out of bounds; if wrong, the widened offer is a silent no-op.
- Mutation pass "at B" (new sensors red-on-invert); sibling knowledge-base decision sources (`data/kollect-xconsol-*`, out of repo).
