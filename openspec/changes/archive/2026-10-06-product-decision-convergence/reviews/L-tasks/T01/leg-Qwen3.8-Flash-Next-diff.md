## Verdict: CONCERNS

## Findings
- [WARNING] ERA-2's spec clause "scrub rules cannot remove [the stamp]" is pinned by no test — if T06 applies the stamp *before* scrubbing, the whole T01 matrix goes green anyway. — `internal/collect/prune_collected_generation_test.go:54-123` (all three stamp tests use `NewScrubber(nil)`) vs `openspec/.../specs/export-annotations/spec.md` ("applied after pruning and scrubbing").
  Failure: operator/profile scrub denylist entry `collectedGeneration` (`api/v1alpha1/export_spec_types.go:84`; `internal/collect/scrub.go:129-135` matches suffixes case-insensitively) → the stamp is replaced by `{redacted:true}` in the embedded copy, contradicting the spec, with every T01 test passing. The prune-axis test (`:75`) pins prune-ordering but not scrub-ordering.
  Fix: add one red test — `PruneResource(deploymentWithGeneration(42), exportAll, NewScrubber([]string{"collectedGeneration"}))` asserting the stamp survives.
  Confidence: 85
- [NOTE] ERA-1 "Absence is a value" scenario's `AND the Synced condition message and requeue cadence stay as for any other export` is unasserted — the tests check only `ExportedCount/DebouncedCount` and backend call counts. — `internal/controller/requested_at_annotation_test.go:229-256`; a T05 that forces the export but mislabels the condition reason or requeue would pass T01.
  Fix: in one transition test, assert `outcome.SinkExports[0]` Synced reason "Exported" and `RequeueAfter` after the forced export.
  Confidence: 70
- [NOTE] `TestPruneResource_noStampWithoutMetadata` is named for stamp-absence but asserts only that the `metadata` key is gone — a stamp emitted under a different top-level key passes it. — `internal/collect/prune_collected_generation_test.go:103-119`.
  Fix: also marshal `got` and assert it does not contain the annotation key, as the Attributes-mode test already does (`:203-209`).
  Confidence: 65
- [NOTE] Cluster tests use invKey literal `"cluster/platform-rollup"` while production passes `req.String()` (`internal/controller/kollectclusterinventory_controller.go:192-206`), which for a cluster-scoped object is the bare name. No test effect (tracker key is self-consistent), but it misstates the production key shape; if T05 ever derives debounce identity from a production helper, the tests won't notice a divergence.
  Fix: use `"platform-rollup"` to mirror production.
  Confidence: 80

## Checked (not just "looks fine")
Read both test files, `T01.md`, the ERA-1/ERA-2 spec delta, `kollectinventory_controller.go` (`exportToSinks`, `previewAllSinksDebounced`), `kollectclusterinventory_controller.go`, `per_sink_export.go`, `prune.go`, `scrub.go`, and every referenced fixture (`recordingBackend`, `noopLogger`, `clusterRollupScheme`, `newResourceExportEngine`, `sampleDeployment`, dual-sink harness). Verified: all symbols exist at base (no compile-red); `perSinkCoalesceTracker` is a per-reconciler value with lazy maps, so struct-literal reconcilers and `t.Parallel()` are safe; 5-minute interval makes steady-state debounce deterministic; the namespaced invKey matches production `req.String()`; the cluster reconciler has no debounce-preview, so the preview test's namespaced-only scope is correct, not a gap; evidence's recorded failure lines (182/221/247/285/324/361, 67/90/161) match the committed file exactly; every red is an assertion failure, including the prune-ordering red whose "no annotations map" cause is the missing stamp. T01's tick in `tasks.md:10` is unchecked — declared as pending close in the evidence, so process state, not a defect.

## Could not check
- Did not execute `go build/vet/test`, `task lint`, or golangci-lint myself (read-only review); red-at-base rests on static reading plus the evidence log, which I cross-checked line-by-line against the committed tests.
- Evidence iteration 6's throwaway-worktree base-flake reproduction (`TestRecordLabeledMetricSeries_CapsCardinalityDeterministically` at `4ae93088`) — not re-verified.
- CI/CDS gate results for commit `d67309bf`; the `-count=1` race-gate deviation is accepted per the repo's `hack/coverage.sh` claim, which I did not open.
