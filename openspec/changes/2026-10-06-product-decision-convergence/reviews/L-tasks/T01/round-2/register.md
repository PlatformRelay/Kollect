## Unified verdict: CONCERNS (legs ok: 2/2)

| # | Sev | Claim (one line) | Where | Models | Legs | Conf |
|---|---|---|---|---|---|---|
| 1 | CRITICAL | `newRequestedAtNamespacedHarness` omits `.WithStatusSubresource(inv)`, so the moment T05 turns the forced export green, `updateStatus` → `Status().Update` returns NotFound (controller-runtime v0.24.1 `versioned_tracker.go:250-252`) and `assertSyncedAsForAnyExport` fatals before asserting | `internal/controller/requested_at_annotation_test.go:92` (helper `:242`; pattern exists at `kollectinventory_update_status_test.go:34,72`) | 2 | 2 | 100 |
| 2 | WARNING | `Prune.ScrubKeys` in the scrub-test fixture is a false lever: `PruneResource` never reads it (only the `*Scrubber` arg scrubs; merge lives at `engine.go:266/258-278`) — misleads the T06 implementer | `internal/collect/prune_collected_generation_test.go:109-116` | 2 | 2 | 100 |
| 3 | WARNING | Evidence `T01.md` sensor table/header stale and self-contradictory after round-2 edits: header "rows 1–10 RED" contradicts table (row 10 pass, 11 rows), rows 42-43 mix pre/post-fix run counts unlabelled, row 3's "+helper" is the only marker of the round-2 sensor | `openspec/changes/2026-10-06-product-decision-convergence/evidence/T01.md:12,43` | 2 | 2 | 100 |
| 4 | WARNING | Cluster-path parity claimed but not locked: no Synced/RequeueAfter assertion after forced export on the cluster path (5-arg `updateStatus`, `kollectclusterinventory_controller.go:153`, no reusable helper); a T05 writing a special reason/zero requeue passes both cluster tests | `internal/controller/requested_at_annotation_test.go:332-368` | 1 | 1 | 85 |

## Disagreements
- Entry 1 severity: GLM-5.3 graded CRITICAL claiming the NotFound failure is source-verified; Qwen graded WARNING framing the green path as merely unverified — same defect, no factual conflict.
- Execution vs source proof: Qwen listed the fake-client `Status().Update` behaviour as "could not check"; GLM resolved it by reading the pinned dependency source — neither executed it.

## Nobody could check
- `-race` (and `-count=2`) on the final tree — both legs; evidence's "no DATA RACE" row trusted, not re-run.
- `task lint` full gate incl. go-arch-lint, `task verify`, and CI results for round-2 commit `066038ee`.
- Runtime execution proof of the entry-1 NotFound path (needs a runnable harness change; source-verified only).
- Pre-existing `-count=2` base flake repro at `4ae93088`; round-1 `register.md` content and T05/T06 task texts (cited verbatim in evidence, not opened independently).
