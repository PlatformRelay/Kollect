## Unified verdict: CONCERNS   (legs ok: 6/6)

| # | Sev | Claim (one line) | Where | Models | Legs | Conf |
|---|---|---|---|---|---|---|
| 1 | HIGH | ERA-2's unconditional "SHALL carry stamp" is false for the default profile: `SpecAndStatus` drops `metadata`, so a minimal Resource-mode copy is unstamped | specs/export-annotations/spec.md scenario 1; internal/collect/prune.go:96-99 | GLM, DS, Q24T (3) | GLM-spec, DS-diff, Q24T-spec (3) | 100 |
| 2 | HIGH | Live per-release template still advertises `spec.git.engine: cli`, falsifying ADR-0803's "every site truthed up" claim | .github/release-notes-install.md:7,29 | GLM, QFN (2) | GLM-spec, QFN-adv (2) | 100 |
| 3 | HIGH | Upgrade note says persisted `engine: cli` fails terminally; code classifies it Transient → deterministic failure requeues forever | docs/operator-manual/upgrading.md:250-253; internal/sink/git/config.go:169-171 | QFN, Q24T (2) | QFN-adv, Q24T-adv (2) | 100 |
| 4 | WARNING | Tombstones added on every delete but pruned only inside `acquireBackend`; an export-less manager leaks one entry per delete for process lifetime | internal/sink/backend_pool.go:241-245,302-313 | GLM, DS, QFN (3) | GLM-spec, DS-diff, QFN-adv (3) | 100 |
| 5 | WARNING | `.gitignore` ends with a comment promising an exclusion that has no pattern; raw leg transcripts stay tracked-eligible | .gitignore:97 | GLM, DS, Q24T (3) | GLM-fit, DS-diff, Q24T-adv (3) | 100 |
| 6 | WARNING | TSP-1 cadence asymmetry: cluster target recounts every reconcile, namespaced path keeps its own `targetCountResync` cadence; owner decision parked in loop.md | internal/controller/kollectclustertarget_controller.go:316 vs kollecttarget_controller.go:359 | QFN, Q24T (2) | QFN-adv, Q24T-spec, Q24T-adv (3) | 100 |
| 7 | WARNING | BEP-1's "discarded" is implemented as deferred-close: tombstoned in-flight build handed open to caller, Closed on its release | internal/sink/backend_pool.go:159-163 | DS, Q24T (2) | DS-diff, Q24T-spec (2) | 100 |
| 8 | WARNING | Backend `Close()` runs inline on the informer dispatch goroutine; a blocking Close stalls all sink events for the kind | internal/controller/family_sink_controller.go:105 | DS, QFN (2) | DS-diff, QFN-adv (2) | 75 |
| 9 | WARNING | Determinism ratchet fails on `internal/collect` (`-count=2` metrics test, reproduced); branch gates that package at `-count=1` | internal/collect/metrics_snapshot_test.go:73 | GLM (1) | GLM-fit (1) | 90 |
| 10 | WARNING | Coverage headroom thin: 91.3% vs 90% floor; new low spots (EvictBackendPoolForSink 66.7%, writeTempKnownHosts 58.3%) | hack/coverage.sh:14 | GLM (1) | GLM-fit (1) | 85 |
| 11 | WARNING | Evict-during-use NATS leak: pooled backend Closed mid-export, `jetStream()` redials, fresh connection never closed (same class aa967f23 fixed for builds) | internal/sink/backend_pool.go:136-141 | Q24T (1) | Q24T-adv (1) | 70 |

## Disagreements
- GTE-3 completeness: Q24T-spec concluded "no surviving `engine: cli` claims; GTE-3 holds" while GLM-spec and QFN-adv both found `.github/release-notes-install.md` still advertising it.
- Tombstone growth framing: DS-diff calls it a "new unbounded growth vector"; GLM-spec and QFN-adv call it bounded with no correctness impact.
- ERA-2 requirement verdict: DS-diff says "partial", GLM-spec and Q24T-spec say "holds (modulo the stamp caveat)" — same code facts, different requirement-level judgement.
- Severity of evict/Close paths: Q24T-spec executed the tombstone/evict tests and called BEP-1/2 clean, while Q24T-adv (same model, other lens) found the #11 leak in the pooled-backend path its sibling did not test.

## Nobody could check
- Full `task verify`, `task lint`, coverage-gate and full `-race` never executed by any leg (they regenerate files); all Verification-table greens at `81bea0b0` are claims (GLM-fit re-ran only arch-lint, scoped golangci, spec:validate; Q24T-adv ran build/vet + focused units + envtest wiring spec).
- Integration tier `task test-integration` (needs Docker) — not run by any leg; CI-owned.
- Base-tree repro at `3ee21266` of the `-count=2` metrics failure — the "pre-existing" claim was confirmed by nobody.
- Mutation testing of the new tests, and base-branch coverage before/after comparison.
- Sibling knowledge-base review artefacts (`data/kollect-xconsol-*`, outside the repo) — the five product decisions were taken on the proposal's relay only.
- One-leg residuals: `internal/pipeline` `file://` export path post-convergence (DS); whether any sntrup761-only SSH server exists (GLM).
