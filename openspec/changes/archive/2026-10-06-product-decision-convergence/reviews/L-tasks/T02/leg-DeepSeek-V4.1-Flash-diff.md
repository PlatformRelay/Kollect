## Verdict: CONCERNS

## Findings

- [WARNING] The new status fields and printer columns land without regenerated CRD manifests, so the committed tree fails `task verify` (codegen drift) and a schema-pruning API server would drop `status.collectedCount` — the fake-client tests cannot detect a T07 that forgets `make manifests`. — `config/crd/bases/kollect.dev_kollectclustertargets.yaml` (no `collectedCount`/`collectedCountUpdatedAt`, verified by grep) vs `api/v1alpha1/kollectclustertarget_types.go:41-52,61-62`
  Failure: T07 wires the write, T02's tests go green (fake client keeps unknown status fields), but the shipped CRD schema still omits the fields → kubectl shows nothing and the API server prunes them.
  Fix: none in T02 if the deferral stands (loop.md:21 scopes drift to T07/T09); T07 MUST run `make generate manifests`, and a post-T07 `task verify` is the only guard.
  Confidence: 90 (the stale manifest is verified; the deferral is documented and sanctioned, so this is a risk, not a contract breach).

- [NOTE] Printer-column set diverges from the namespaced target: `KollectTarget` declares an explicit `Age` column, `KollectClusterTarget` does not. Spec TSP-1 requires only Collected/Updated, so this is not a defect, only an incomplete "mirror". — `api/v1alpha1/kollectclustertarget_types.go:61-62` vs `kollecttarget_types.go:115-117`
  Failure: operators see a different column set between the two kinds; harmless.
  Fix: add the `Age` marker if full parity is intended; otherwise ignore.
  Confidence: 60.

- [NOTE] The steady test drives both reconciles through the same in-memory `live` object rather than re-reading it from the client, so it verifies the sync helper's decision, not that the timestamp survived an API round-trip on the second pass. — `internal/controller/kollectclustertarget_collected_count_test.go:214`
  Failure: a T07 bug that recomputes the timestamp in memory but persists a stale copy would be masked; the first-pass re-fetch at line 206 does validate first-write persistence, so the exposure is small.
  Fix: re-fetch `live` between the two `setReady` calls.
  Confidence: 55.

- [NOTE] The Degraded guard seeds the count on a nil-Engine reconciler, so it pins "`setDegraded` must not clear the fields" but never exercises the realistic sequence (Ready derives a count with an Engine, a later Degraded reconcile keeps it). — `internal/controller/kollectclustertarget_collected_count_test.go:294-337`
  Failure: a T07 that clears the count only on the Engine-present Degraded branch would slip past.
  Fix: set `r.Engine` and derive the count via `setReady` before `setDegraded`.
  Confidence: 45.

## Could not check
- Did not run `go test`/`go vet` (read-only plan mode), so I verified the red/green classification by reading the fixtures and production seams, not by executing them; the evidence's exit-1 failure set is unconfirmed.
- Did not read `openspec/changes/.../reviews/` (prior review rounds) or the branch-wide review, per the stay-in-scope rule.
- Did not inspect `redact.Text` to prove it is a no-op on the Ready message; the test's own byte-identical assertion (line 180) would catch a divergence, so the risk is contained.
