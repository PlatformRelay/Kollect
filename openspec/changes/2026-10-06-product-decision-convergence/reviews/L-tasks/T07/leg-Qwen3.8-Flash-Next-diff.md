## Verdict: CONCERNS

## Findings
- [WARNING] The mirrored printer test does not test mirroring — `test/schema/printer_columns_test.go:56-62` hardcodes its own `want` map instead of comparing the two CRDs' column sets.
  Failure: a future change edits the namespaced columns (or reverts one) and both CRDs diverge; neither `TestKollectTargetPrinterColumns` nor the new cluster test fails — the "mirrors the namespaced target" half of TSP-1 (`specs/target-status/spec.md:9`) is only pinned by two independently-maintained literals plus a comment.
  Fix: add one subtest reading both CRD manifests and asserting `printerColumns(namespaced) == printerColumns(cluster)`; keep the absolute maps for path regression.
  Confidence: 85
- [NOTE] Cluster `collectedCount` goes stale between events; the namespaced controller requeues itself (`kollecttarget_controller.go:357-359`, F-05) because objects entering/leaving don't enqueue the target — the same argument applies to the cluster path, whose only triggers are namespace/profile/scope watches (`kollectclustertarget_controller.go:413-427`). A cluster target whose engine items churn silently holds `collectedCount`/`collectedCountUpdatedAt` frozen while Ready restates the old number. Explicitly recorded and justified as out-of-scope in evidence row 17 / out-of-scope list, and the field contract ("at its last healthy refresh") doesn't fix a cadence — accept, but the two kinds now behave differently under the one doc'd contract, which is exactly what TSP-1 exists to prevent.
  Confidence: 70
- [NOTE] `syncCollectedCountFields` short-circuits on count equality without checking `updatedAtField != nil` — `collected_count.go:27-29`. A hand-patched or drifted object with `collectedCount` set and `collectedCountUpdatedAt` null keeps a permanently empty Updated column. Pre-existing namespaced semantics carried over verbatim (correct for parity); flag only because the cluster CRD is newly writable and someone will hand-edit it.
  Confidence: 60
- [NOTE] `TestClusterTargetSetReady_skipsWriteWhenNothingChanged`'s churn claim is now narrower than before: with count seeded at the measured zero it can no longer catch a regression where nil-count-on-Ready is treated as unchanged — but that regression is caught by `persistsCountWhenConditionUnchanged`, so the pair covers it. Fixture fix is honest, matches the namespaced anchor (`kollecttarget_collected_count_test.go:221-242`) semantics.
  Confidence: 80
- [NOTE] `int(count)` cast at `kollectclustertarget_controller.go:322` is provably lossless (helper derives int64 from the same int read), correctly documented; no aliasing bug in the double-pointer helper — `next`/`now` are call-local, no shared pointers across objects.
  Confidence: 90

Verified against code, not evidence: all five TSP-1 scenarios have concrete tests that pass by inspection of the write-skip logic (`conditions.go:96-125` redact-then-skip, escape at `kollectclustertarget_controller.go:338`); the ONE-write-site claim holds — cluster path has exactly the shared writer (`conditions.go:125`) plus the one hatch (`kollectclustertarget_controller.go:355`); Degraded path correctly never syncs count (line 283-301); namespaced wrapper is a pure delegate with unchanged signature (`kollecttarget_controller.go:294-301`); CRD base and chart copy are the same blob (`d0563c2e`); deepcopy already covered the fields at base (T02), so the no-op claim is real; `docs/crds/kollecttarget.md` indeed restates no status fields; no secrets introduced; Age suppression claim matches namespaced markers (`kollecttarget_types.go:115-117`).

## Could not check
- Ran nothing (read-only review): build, vet, `task lint`, `task verify`, `-race` tests, gitleaks, glossary regen — all accepted as recorded in `evidence/T07.md` gates table; test-green claims are logical inference from the code paths, not execution.
- `hack/gen-glossary.py` internals (spec-fields-only renderer claim, row 11).
- `zz_generated.deepcopy.go` completeness for the cluster status at T02 commit beyond the two CollectedCount hunks spotted by grep.
