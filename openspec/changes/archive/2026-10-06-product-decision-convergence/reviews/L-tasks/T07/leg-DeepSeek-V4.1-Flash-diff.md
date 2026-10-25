## Verdict: CONCERNS

## Findings
- [WARNING] The cluster count has no refresh cadence, unlike the namespaced kind it is supposed to mirror — `internal/controller/kollectclustertarget_controller.go:413-429` vs `internal/controller/kollecttarget_controller.go:357-359`
  Failure: after a `KollectClusterTarget` reaches Ready, adding/removing a collected resource in a matched namespace enqueues nothing (the controller watches only Namespace/Profile/ClusterScope, not the collected objects), so `status.collectedCount` stays frozen at its last value until an unrelated namespace/profile/scope event fires. The namespaced kind requeues itself (`RequeueAfter: r.Options.targetCountResync()`) precisely because "objects entering or leaving the matched set do not enqueue their target". TSP-1's "at its last healthy refresh" does not fix a cadence, so this is not a spec violation — but it is the exact F-05 parity the task title claims, and the count can silently go stale on a healthy Ready target. The evidence (T07.md:46-49) defers it knowingly.
  Fix: either add the same `RequeueAfter: r.Options.targetCountResync()` return from cluster `setReady`, or state in the CRD/doc prose that the cluster count is event-driven, not polled — do not leave the divergence undocumented.
  Confidence: 70 (divergence is certain; whether it counts as a defect for this task is a judgement call, hence not CRITICAL).
- [NOTE] Cluster measured-zero upgrade (nil count → derived `0`) is not directly exercised — `internal/controller/kollectclustertarget_collected_count_test.go:146-187` uses 17.
  Failure: none demonstrated — the shared helper is identical to the namespaced one whose zero case is pinned (`kollecttarget_collected_count_test.go:221-242`), and the wiring path (`countChanged && !written` → hatch) is covered by the 17 case. Coverage is adequate by construction, not by test.
  Fix: none required; if the helpers ever fork, the cluster zero case goes uncovered (the evidence's own "Harder next" note).
  Confidence: 85.
- [NOTE] `docs/crds/kollecttarget.md` still documents no status fields, so the two CRD pages now have unequal depth — `docs/crds/kollectclustertarget.md:116-125` vs `docs/crds/kollecttarget.md:121`.
  Failure: cosmetic only; the task text conditions the namespaced edit on "only if it restates fields", and it restates none.
  Fix: none in T07; deferred (T07.md:146-149).
  Confidence: 90.

## What I verified as correct
- The refactor is behaviour-preserving: `syncCollectedCountFields` (`internal/controller/collected_count.go:20-36`) is byte-for-byte the old namespaced body; the wrapper keeps the signature, so namespaced tests are untouched. Build/vet claim consistent (no dangling `metav1` import in `kollecttarget_controller.go`; single `Status().Update` site in the cluster controller at :355).
- The "one write" claim holds: condition write (`written`) or the single hatch, never both; `filterChanged || countChanged` is exactly D3.
- Degraded keeps the count on both paths (`setDegraded` never touches the fields on either controller) — parity holds.
- Printer columns: types markers (`kollectclustertarget_types.go:61-63`), both CRD sources (`config/crd/bases` and `charts/kollect/crds`) carry Collected/Updated/Age on the same JSONPaths; the mirrored test reads both. Shortname `kctgt` in the doc matches the CRD.
- `kollecttarget.md` genuinely restates no count fields (grep clean).

## Could not check
- Did not run `go build`, `task lint`, `task verify`, or any test (read-only review); gate claims rest on `evidence/T07.md`.
- Did not verify against a live API server that non-empty `additionalPrinterColumns` suppresses the default Age column (the evidence flags this as believed, not verified).
- Did not read `hack/verify.sh` end-to-end to independently confirm the exit-201 is solely the GTE-1 schema red, nor the chart-CRD diff byte-for-byte.
