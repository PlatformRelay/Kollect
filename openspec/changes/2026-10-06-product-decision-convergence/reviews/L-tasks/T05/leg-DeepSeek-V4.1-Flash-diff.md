## Verdict: CONCERNS

## Findings
- [WARNING] The one behaviour the new code comment singles out — that the `requestedAt` check runs *before* the `interval == 0` early return so zero-interval bindings still re-export — is untested — `internal/controller/per_sink_export.go:82-87`
  Failure: if a later refactor moves the `if state.lastRequestedAt != requestedAt` block below `if interval == 0 { return true }`, a sink with `exportMinInterval: 0` silently stops honouring `requestedAt` (the forced re-export becomes a no-op); every existing test still passes because `TestPerSinkCoalesceTracker_requestedAtAxis` uses `interval = time.Minute` (`per_sink_export_test.go:184`) and the controller tests use 5 min.
  Fix: add one `interval = 0` case to `TestPerSinkCoalesceTracker_requestedAtAxis` (record with a value, assert a changed value returns `false`).
  Confidence: 90
- [NOTE] An explicit empty-string annotation is indistinguishable from absence, so `requestedAt: ""` is not a trigger — `api/v1alpha1/constants.go:44-48`, `docs/ANNOTATIONS-LABELS.md:101`
  Failure: the spec says "its appearance on an object that had none" is a change; a user who sets the key to `""` (e.g. a templated Helm value rendering empty) gets no export and no error. The doc's "any non-empty string works" is technically correct but the spec wording invites the mistake.
  Fix: none required; the doc already says non-empty. Optionally one clause in the constant comment.
  Confidence: 70
- [NOTE] The two `assertSyncedAsForAnyExport` helpers now drive `updateStatus` on a `DeepCopy` — `internal/controller/requested_at_annotation_test.go:247,392`
  Failure: this is a legitimate test-class fix (verified: the fake client's status-subresource update restores stored metadata onto the passed object, resurrecting `inv.Annotations = nil`), and it does not weaken the production assertions. Flagging so the reviewer confirms the claim rather than the code. I reproduced a green run of the six ERA-1 tests and the tracker axis test.
  Fix: none.
  Confidence: 85

## What I verified
- Read the full diff `81cef0c1..HEAD` and all four production call sites; no `.shouldSkip`/`.record` caller was missed (`rg` shows exactly the four, all passing `requestedAt`). No literal `kollect.dev/requestedAt` remains under `internal/`.
- Traced the feature end to end: annotation change → watch (no `GenerationChangedPredicate` in either `SetupWithManager`) → namespaced `previewAllSinksDebounced` gate (`kollectinventory_controller.go:189`) now sees the change and lets the reconcile through to `exportToSinks`; cluster path calls `exportClusterToSinks` directly. "One export then steady state" holds: `record` pins the value, the next preview/export skips.
- Ran `go test -count=1 ./internal/controller ./api/v1alpha1`: 53/53 Ginkgo specs pass, `api/v1alpha1` ok, and the only failures are the four T02 known reds (`TestClusterTargetSetReady_*`, `kollectclustertarget_collected_count_test.go`). Matches the evidence.
- No new dependency, no arch-lint/allow-list/baseline touched, no secret or untrusted-input sink (value only string-compared, never parsed/logged/written).

## Could not check
- Did not run `task lint`, `task verify`, gitleaks, or the `-race` tiers; those restate the evidence's records.
- Did not run `hack/test/docs_launch_truth_test.sh` or `task lint:markdown`; did not confirm whether any lint requires an ADR link on an annotation-table row (the row dropped its ADR-0201 citation).
- Integration/envtest tier (Docker) not run here.
