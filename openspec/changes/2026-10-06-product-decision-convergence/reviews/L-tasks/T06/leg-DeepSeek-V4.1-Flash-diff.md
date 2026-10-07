## Verdict: CONCERNS

## Findings
- [WARNING] ERA-2's headline requirement is only partially met: with the default `include`, a Resource-mode copy carries no stamp — `openspec/.../specs/export-annotations/spec.md:45` vs `internal/collect/prune.go:98`
  Failure: profile `export: {mode: Resource}` (no `include`) → `IncludeOrDefault()` returns `SpecAndStatus` (`api/v1alpha1/export_spec_types.go:109`, CRD default at `:51`), `selectIncludeSections` keeps only spec+status (`prune.go:139-142`), so `root["metadata"]` is absent and `stampCollectedGeneration` returns at `prune.go:98-101`. A generation-42 object exports with **no** `collectedGeneration`, so the staleness signal the annotation exists for is silently inert for every user who does not explicitly set `include: All`. The requirement is unconditional ("SHALL carry"); only the `StatusOnly` scenario sanctions a missing stamp, and D2 frames that as a deliberate exclusion, not the default.
  Fix: smallest is to make the divergence explicit — the doc row (`docs/ANNOTATIONS-LABELS.md:100`) and the constant comment should state that the default `SpecAndStatus` drops metadata and therefore produces no stamp; if the feature is meant to work by default, retain metadata in Resource mode or stamp before `selectIncludeSections` instead.
  Confidence: 85 that the behaviour is as described (code-read, not executed); ~70 that it is a defect rather than intended D2 semantics.
- [NOTE] Unreachable nil guard in the new helper — `internal/collect/prune.go:94`
  Failure: `stampCollectedGeneration` is called only from `prune.go:79`, after the `obj == nil` early return at `prune.go:45-47`, so `src == nil` can never hold; the branch is dead and is the one uncovered path in the function.
  Fix: delete the guard, or cover it with a direct `stampCollectedGeneration` unit test.
  Confidence: 95.
- [NOTE] TEST-class correction of the two no-op tests is justified — `internal/collect/prune_branches_test.go:66,190`
  Failure: none. The tests' exact `copy == input` deep-equality is genuinely stale against D2 (the stamp is written whenever metadata survives), and the shared `stripCollectedGenerationStamp` helper asserts the stamp is present before deleting it, so the invalid-pointer / unsupported-JSONPath no-op guarantees stay fully pinned — it is strengthened, not weakened. The helper mutates only `got` (a `PruneResource` deep copy), never `want`.
  Fix: none.
  Confidence: 90.
- [NOTE] No ratchet exists for the headline requirement — `internal/collect/prune_collected_generation_test.go:54`
  Failure: every ERA-2 test sets `Include: All` or `StatusOnly`; nothing exercises the default include, which is exactly where the requirement fails. An existing sensor would not have caught the gap.
  Fix: add one test asserting a Resource-mode copy with `Include` unset still carries the stamp (it fails today, forcing the D2-vs-requirement decision).
  Confidence: 90.

## Could not check
- Did not run `task verify` or `task test-integration` (no Docker; `verify` not applicable per evidence) — I ran only the focused ERA-2/no-op tests (`go test ./internal/collect`, green).
- Did not exercise the CRD default in a live cluster; the `SpecAndStatus` default is taken from `+kubebuilder:default` and `IncludeOrDefault`.
- Did not read the other review legs (`reviews/L-tasks/T06/`), by design, nor any path outside this repo.
- Did not confirm whether any generated artifact enumerates annotation constants (searched `api/v1alpha1`, `internal/validation`, docs — found none).
