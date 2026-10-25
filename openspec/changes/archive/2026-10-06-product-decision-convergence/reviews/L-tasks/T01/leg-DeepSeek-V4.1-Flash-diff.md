## Verdict: CONCERNS

## Findings
- [WARNING] ERA-1's "Absence is a value" scenario also requires "the `Synced` condition message and requeue cadence stay as for any other export", which no test asserts — `internal/controller/requested_at_annotation_test.go:203,229`
  Failure: a T05 implementation that re-exports on an absence transition but sets a wrong `Reason`/message or a bogus `RequeueAfter` passes all T01 tests; the spec clause is unguarded.
  Fix: assert the exported status `Reason`/message and `outcome.RequeueAfter` in the absence→present test, as the spec's scenario demands.
  Confidence: 80 (the clause is in `specs/export-annotations/spec.md:41`; task text T01 does not enumerate it, so it may be deliberately deferred).
- [NOTE] ERA-2 says scrub rules must not be able to remove the stamp; only a prune path is tested — `internal/collect/prune_collected_generation_test.go:72`
  Failure: T06 that applies the stamp before `scrubber.Scrub` would be caught only if a scrub key targets `metadata.annotations`; no such test exists.
  Fix: add a `PruneSpec.ScrubKeys` case (or reuse an annotation-targeting scrub) to the survive test.
  Confidence: 70.
- [NOTE] "Attributes mode byte-identical" is a structural check, not a byte comparison — `internal/collect/prune_collected_generation_test.go:196`
  Failure: a change to an existing attribute value, or a new non-`Attributes` field on `collect.Item`, would not be caught; the test only checks `len(item.Attributes)==1` and absence of the stamp substring.
  Fix: compare a marshalled golden payload, or at minimum pin the full attribute map.
  Confidence: 75.
- [NOTE] `TestPruneResource_noStampWithoutMetadata` asserts only that `metadata` is gone and the result is non-nil — `internal/collect/prune_collected_generation_test.go:99`
  Failure: it is green regardless of most T06 errors; weak as a guard (acknowledged in evidence as "green-by-construction").
  Fix: additionally assert the surviving sections equal today's `StatusOnly` output.
  Confidence: 90 (by design).

## What I verified (machine)
- All 9 reds fail exactly for the stated reasons: 6 controller (`requested_at_annotation_test.go:182,221,247,285,324,361`) and 3 collect (`prune_collected_generation_test.go:67,90,161`). Reproduced with `go test -count=1 -run ... ./internal/controller ./internal/collect`.
- No production symbol referenced that does not exist: both packages compile; the 2 guards pass (`TestPruneResource_noStampWithoutMetadata`, `TestProcessDispatch_attributesModeUntouchedByStamp`).
- `go vet ./internal/controller ./internal/collect` → exit 0; `bin/golangci-lint run ./internal/controller/... ./internal/collect/...` → "0 issues".
- Fidelity to task: the enumerated T01(a)/(b) claims (changed-value re-export, unchanged debounce, both transitions, cluster parity, preview honesty, stamp, prune survival, no-metadata, Attributes untouched) are each covered by a named test; the commit message's "nine red tests" matches the matrix.
- Diff is test-only plus evidence; no production file touched; annotation keys are test-local literals, so T05's tracker signature change cannot break compilation.
- Correctness hazard checked: `DefaultsEnabled()` returns true for a nil `Prune` (`api/v1alpha1/export_spec_types.go:125`), so the nil-prune stamp tests do apply the builtin `/metadata/generation` prune — a T06 that read generation post-prune would stay red. No gap there.

## Could not check
- `-race` runs, `task lint` (full, incl. go-arch-lint), `task verify`/codegen drift, `task test-integration` — not run here.
- The two other T01 review legs under `openspec/.../reviews/L-tasks/T01/` and `task-prompt.md` — out of bounds (other reviewers' artefacts), not read.
- Whether T05/T06's eventual doc edits to `docs/ANNOTATIONS-LABELS.md:100-101` satisfy ERA-1/ERA-2 doc clauses — future work, no test possible.
