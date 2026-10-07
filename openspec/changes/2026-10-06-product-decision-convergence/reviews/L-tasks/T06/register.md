## Unified verdict: CONCERNS  (legs ok: 2/2)

| # | Sev | Claim (one line) | Where | Models | Legs | Conf |
|---|---|---|---|---|---|---|
| 1 | MAJOR | Default-include (`SpecAndStatus`) Resource-mode copy drops metadata and carries no `collectedGeneration` stamp, leaving ERA-2's unconditional "SHALL carry" unmet for the most likely config, and no test pins that combination | internal/collect/prune.go:98,98-101 + api/v1alpha1/export_spec_types.go:107-113 vs specs/export-annotations/spec.md:45 | 2 (DeepSeek, Qwen) | 2 | 100 |
| 2 | MINOR | Both legs independently confirm the TEST-class correction of the two no-op tests strengthens coverage (no `got`/`want` aliasing; stamp presence still asserted before stripping) — not a weakening | internal/collect/prune_branches_test.go:66,190/193 | 2 (DeepSeek, Qwen) | 2 | 100 |

## Disagreements
- Entry 1's behaviour is agreed by both models (code-read), but DeepSeek judges it a WARNING defect (requirement is unconditional; only StatusOnly sanctions a missing stamp) while Qwen judges it D2-sanctioned and doc-row-accurate, flagging only the missing test pin — hence CONCERNS (DeepSeek) vs CLEAN (Qwen); MAJOR severity rests on that promotion.
- Nothing else contradicted: both legs' verified-positive lists (stamp-after-prune ordering, source-generation read, generation-0 handling, attributes-mode untouched, scrub-then-stamp order) are consistent.

## Nobody could check
- Full `task verify` / `test-integration` (no Docker / read-only): DeepSeek ran only `go test ./internal/collect` (green); Qwen ran nothing — all evidence-matrix exit codes (build/vet/test/-race/lint/gitleaks) trusted as recorded, not re-measured.
- CRD `SpecAndStatus` default not exercised in a live cluster (taken from `+kubebuilder:default` + `IncludeOrDefault`).
- Sink-side content-hash/debounce impact of the stamp churn (underpins the dropped single-leg churn NOTE).
- Project-service / pk-dist twin-module sync named in the dispatch — repos outside this checkout.
- Whether any generated artifact enumerates annotation constants.
- Whether the post-review close commit will actually tick tasks.md:49, advance loop.md and commit evidence/T06.md (future state; currently all absent from the reviewed range).
