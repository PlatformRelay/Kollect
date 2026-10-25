Round-1 register verbatim is already in the evidence file. Now the production code the tests claim to drive, plus the round-1 register:
`Prune.ScrubKeys` is not consumed inside `prune.go`. Checking whether anything wires it, and reading the controller paths the helper depends on:
Now the Synced condition aggregation and the engine's scrub-key merge — the two things the round-2 fixes assert against:
Assertions match production (`per_sink_export.go:338-339` → `true, "Exported", "exported to %d sink(s)"`). Now the requeue math and the fake-client status-subresource question:
The established pattern registers `.WithStatusSubresource(inv)` before calling `updateStatus` (kollectinventory_update_status_test.go:34,72) — the new harness does not. Checking the requeue math and then what the fake client actually does on a status update of a non-registered object:
That previous grep searched outside the repo by accident — disregarding it and staying in bounds. Checking the fake client's status-update behaviour in the pinned dependency source (module cache, not a sibling checkout):
The dependency source confirms it: `versioned_tracker.go:250-252` returns `apierrors.NewNotFound` when a status update hits an object whose GVK is not in `withStatusSubresource`. Now verify the evidence's claimed failure lines against the actual files, then run the tests myself:
Line numbers match the evidence. Now the compile check and the actual red/green verification:
Vet is clean — no non-existent production symbol referenced. Running the red tests and the two guards:
All reds verified against my own runs, both guards green. Two last cross-checks — the Synced condition type used by `setSyncedCondition`, and what the round-2 commit actually touched:
Round-2 commit touches only the three fix areas. One last gate — lint on the final tree:
## Verdict: CONCERNS

Round-1 fixes F1/F3 verified sound and landed; F2's helper is correctly asserted against production but sits on a harness that will make it fail with the wrong error the first time it actually runs. All other claims verified by running the tests.

## Findings

- [CRITICAL] `newRequestedAtNamespacedHarness` does not register the status subresource, so `assertSyncedAsForAnyExport` will fail on `Status().Update` with NotFound the moment T05 turns the forced export green — `internal/controller/requested_at_annotation_test.go:92`
  Failure: at T05 green, `updateStatus` → `r.Status().Update(ctx, inv)` (kollectinventory_controller.go:600) → controller-runtime v0.24.1 `versioned_tracker.go:250-252` returns `NewNotFound` for a GVK not in `withStatusSubresource` (KollectInventory is a CRD, not auto-registered) → the helper's `t.Fatalf("updateStatus: %v", err)` fires before the condition assertions it was added to make.
  Fix: add `.WithStatusSubresource(inv)` to the namespaced builder — the repo's own `kollectinventory_update_status_test.go:34,72` and the cluster builder in this same new file (`:142-145`) already do exactly this.
  Confidence: 92 (verified against the pinned dependency's source and the repo's own pattern; not executed, since the helper is unreachable in the red state).

- [NOTE] Evidence sensor table is stale for the round-2 additions: rows for the `-v` runs (evidence/T01.md:42-43) predate `TestPruneResource_stampSurvivesScrubRule`, yet matrix row 9 and Known-red (`:80`) cite it — `openspec/changes/2026-10-06-product-decision-convergence/evidence/T01.md:43`
  Failure: a reviewer trusting the table sees no recorded run for the new known red; the `-race` rows (46-47) do say "4 known reds" (post-fix) while rows 42-43 say 3, so the table mixes pre- and post-fix runs unlabelled.
  Fix: mark rows 42-43 as round-1 runs and add the post-fix `-run` row (I re-ran it: the scrub test fails at line 124 for the stated reason, guards pass).
  Confidence: 100.

- [NOTE] `TestPruneResource_stampSurvivesScrubRule`'s `Prune.ScrubKeys` field is dead in a direct `PruneResource` call — the effective denylist is the `NewScrubber` argument; `prune.go` never reads `ScrubKeys` (the merge lives in `engine.go:258-278`) — `internal/collect/prune_collected_generation_test.go:110-116`
  Failure: none today — same `isSensitiveKey` path either way, and the profile merge is separately pinned by the existing `TestProcessDispatch_resourceExportMergesProfileScrubKeys`; risk is only a T06 implementer misled about where `ScrubKeys` is consulted.
  Fix: one comment line pointing at `scrubberForProfile`, or drop the field from the fixture.
  Confidence: 95.

## What I checked (all on `4ae93088..HEAD`, fresh runs)

- Ran the 6 controller reds and 4 collect reds: each fails at the line and for the reason the evidence quotes (`:183/:222/:282/:321/:360/:397` and `:67/:90/:124/:197`) — including the round-2 scrub test. Both guards pass (`noStampWithoutMetadata`, `attributesModeUntouchedByStamp`).
- `assertSyncedAsForAnyExport`'s values match production: Synced/True/"Exported"/"exported to 1 sink(s)" is `aggregateInventorySync`'s default branch (`per_sink_export.go:338-339` via `sink_status.go:173`), RequeueAfter 5m is `mergeRequeueAfter(5m, 5m)` from `exportDebounce` → updateStatus (`kollectinventory_controller.go:292,445-446,608-613`).
- Scrub-redaction mechanism real: `isSensitiveKey` exact/suffix match on the normalized stamp key redacts to a `map[string]any` sentinel (`scrub.go:121-144,34-39`), which the test's first check catches.
- `go vet` on both packages (0) — no non-existent production symbol referenced, no constant collision; `bin/golangci-lint` on both packages: 0 issues.
- Round-2 commit `066038ee` touches only the three fix areas (+36/+36/evidence).

## Could not check

- `-race` on the final tree (per-package ~45s not run here; evidence claims no DATA RACE) — my runs were `-count=1` non-race.
- `task lint` full gate incl. go-arch-lint, `task verify`, CI results for `066038ee`; the pre-existing `-count=2` base flake repro at `4ae93088`.
- Execution proof of the NotFound path (would need a runnable harness change; source-verified instead — see finding 1).
