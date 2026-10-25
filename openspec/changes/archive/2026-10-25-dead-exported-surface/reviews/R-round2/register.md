## Unified verdict: CONCERNS   (legs ok: 6/6)

| # | Sev | Claim (one line) | Where | Models | Legs | Conf |
|---|---|---|---|---|---|---|
| 1 | HIGH | Task 1.3 premise false: deleted `RunExportItems_*` delegation tests assert live-path behaviour (git-sink happy path, NaN fail-closed, backend pooling, close-error tolerance, capability skip); only the 2 breaker tests are migrated | proposal.md:90-92, tasks.md:14 | GLM, DeepSeek | G-adv, G-sec, D-spec | 100 |
| 2 | HIGH | "6 production references" to `RunExportEnvelope` unreproducible — 3 live call sites post-change (4th is the runner being deleted); comments never yield 6 | proposal.md:91-92 | GLM, DeepSeek | G-adv, G-sec, D-adv, D-spec | 100 |
| 3 | HIGH | Sole external anchor `data/kollect-xconsol-final/report.md` does not exist in the repo; task 8.2 exclusion claims unverifiable from checkout | proposal.md:5,74,76 | GLM, DeepSeek | G-adv, G-sec, D-spec | 100 |
| 4 | HIGH | Removing `ConditionConnected`/`ConditionCredentialsVerified` breaks external `api/v1alpha1` consumers; disclosed as accepted limit but no CHANGELOG/API-compat note in any task | api/v1alpha1/constants.go:10-11 | DeepSeek, Qwen | D-adv, D-spec, Q-sec | 95 |
| 5 | WARNING | Task 6.2 understates migration: `RegisterTarget` needs profile + synthetic-object fixtures at all 14 controller test sites; no shared helper named | tasks.md:45 | GLM, DeepSeek | G-adv, D-adv | 80 |
| 6 | WARNING | Task 1.3 anchors `export.go:267` by line number that shifts ~50 lines up once task 1.2 deletes `RunExportItems` (export.go:95-146) | tasks.md (1.3), export.go:95 | DeepSeek | D-adv | 90 |
| 7 | WARNING | Proposal states `go build ./...` as the missed-caller proof, but it does not compile `_test.go` — most deleted symbols' only callers are tests | proposal.md:17 | DeepSeek | D-adv | 88 |
| 8 | WARNING | Deleting `RunExportItems` orphans `sinkNamespaceForExport` → `unused` (enabled in .golangci.yaml:32) fails `task lint`, a stated final gate | export.go:302 (sole caller :136) | DeepSeek | D-spec | 85 |
| 9 | WARNING | Deleting the version-monotonicity test leaves the "counter never re-issued, including across RemoveCluster" doc-comment invariant unguarded | store.go:173-180, store_test.go:111 | DeepSeek | D-adv | 65 |

## Disagreements
- Qwen-spec graded DR-1 "holds" with migration "faithful" while 3 legs found it incomplete — Qwen verified only the breaker pair, not the other five delegation tests.
- Verdict split: Qwen-security CLEAN; other five CONCERNS.
- Qwen-sec downgraded the API break (pre-1.0, zero writes/doc/CRD refs for the literals) to conf 60 NOTE; D-adv held it at WARNING — same fact, opposite weight.
- Qwen-spec confirmed `export.go:267` is the exact `exportThroughBreaker` call at HEAD; D-adv's objection is its position *after* task 1.2 runs — different time points, not a factual conflict.

## Nobody could check
- Nothing executed anywhere: no `go build/vet/test`, no `task test/lint/coverage/spec:validate`, no `go vet -tags integration`; the 90% coverage floor on the final tree is asserted, not measured.
- External (out-of-module) consumers of `api/v1alpha1` — compile cannot prove the external half; the proposal itself records this limit.
- The cited consolidated report (`data/kollect-xconsol-final/report.md`) and PR #451 — outside the repo, content unread by every leg.
- Whether a controller-level test asserts the YAML doc export via a differently-named recorder (residual on #1's existence, not its confidence).
