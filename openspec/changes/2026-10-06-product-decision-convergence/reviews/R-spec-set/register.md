## Unified verdict: BLOCK  (legs ok: 4/6 — both DeepSeek legs timed out, exit 143)
| # | Sev | Claim (one line) | Where | Models | Legs | Conf |
|---|---|---|---|---|---|---|
| 1 | CRIT | Proposal promises 11 requirement IDs; deltas define only 8 — ERA-3, TSP-2, BEP-3, GTE-4 (auth-mode parity) exist nowhere, so tasks silently ship fewer requirements than claimed | proposal.md:52-55 | GLM-5.3, Qwen-NVFP4, Qwen-FlashNext | 3 | 100 |
| 2 | CRIT | Engine convergence makes 4 doc sites false (charts README.md.gotmpl, operator-manual/index.md, coding-standards.md, Dockerfile.pipeline) that no task fixes; helm-docs verify stays green because the claim lives in the template | charts/kollect/README.md.gotmpl:16 | Qwen-NVFP4, Qwen-FlashNext | 2 | 100 |
| 3 | CRIT | BEP-1 is unbounded on delete mid-export: eviction Closes a backend a live export is using, and the re-store race re-adds an evicted backend that then sits to the 48h TTL with credentials | specs/backend-pool/spec.md:16-17; internal/sink/backend_pool.go:148 | Qwen-FlashNext, Qwen-NVFP4 | 3 | 95 |
| 4 | WARN | ANNOTATIONS-LABELS.md still promises requestedAt "On: Reconciled Kollect CRs" though D1 honours it only on the two inventory kinds; no task pins the narrowed row | docs/ANNOTATIONS-LABELS.md:101 | GLM-5.3, Qwen-FlashNext | 2 | 95 |
| 5 | WARN | The five product decisions are untraceable: data/kollect-xconsol-final/report.md §7 is on neither disk nor git history, so "honours §7" is the proposal paraphrasing itself | proposal.md:5 | Qwen-FlashNext | 2 | 95 |
| 6 | WARN | T02 cannot compile as ordered (needs status fields T07 adds), and the broken package also kills the red-first runs of T01/T03 in the same package | tasks.md:18-21 | Qwen-FlashNext | 1 | 85 |
| 7 | WARN | GTE-1 self-contradicts on file:// ("any engine value") and no scenario defines a persisted `engine: cli` sink's fate at upgrade, since admission never re-runs on it | specs/git-engine/spec.md:13-27 | Qwen-FlashNext | 1 | 80 |
| 8 | WARN | "Delete event carries the last known state" cites the wrong controller-runtime file; DeleteStateUnknown can deliver a nil/non-sink object and T03/T08 cover no guard branch | proposal.md:102-103 | Qwen-FlashNext | 1 | 70 |

## Disagreements
- Deltas define 8 requirement IDs (both Qwen legs) vs 7 (GLM-5.3) — the dangling-ID substance is agreed by all three models; the count discrepancy is unexplained.
- GLM-5.3's verification says the pool is "UID- and ns/name-keyed as BEP-1 needs"; FlashNext-adversarial says no production caller ever writes the `ns:` key, making T08's `EvictBackendPool(ns,name)` unreachable (its finding was a one-leg NOTE, dropped from the register).
- KEX pin cited at ssh_auth.go:23-33 (GLM-5.3) vs :27-36 (FlashNext-spec); the later read is likely the correct one.

## Nobody could check
- data/kollect-xconsol-final/report.md §7 and pass B — absent from disk and git history in all four legs; the five decisions' original wording unverifiable.
- Any execution: task verify / spec:validate / tests / codegen / -race not run (read-only brief).
- x/crypto v0.57.0 KEX behaviour — verified only via the Go module cache, never in-repo.
- controller-runtime delete-event/tombstone depth, and backend Close internals (whether Close aborts a live git push mid-write).
