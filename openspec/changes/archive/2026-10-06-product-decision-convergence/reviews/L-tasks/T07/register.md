## Unified verdict: BLOCK   (legs ok: 2/2)

| # | Sev | Claim (one line) | Where | Models | Legs | Conf |
|---|---|---|---|---|---|---|
| 1 | CRITICAL | Cluster count has no refresh cadence: `collectedCount`/`UpdatedAt` freeze while Ready (the exact F-05 parity the namespaced kind solves via `RequeueAfter`) | `internal/controller/kollectclustertarget_controller.go:413-429` vs `internal/controller/kollecttarget_controller.go:357-359` | DeepSeek, Qwen | 2 | 85 |
| 2 | WARNING | Cluster nil-count→0 measured-zero path has no direct test (collected-count test uses 17; skipsWrite fixture narrowed, covered only by a sibling test) | `kollectclustertarget_collected_count_test.go:146-187`, `TestClusterTargetSetReady_skipsWriteWhenNothingChanged` | DeepSeek, Qwen | 2 | 100 |
| 3 | WARNING | Mirrored printer test hardcodes its own `want` map instead of comparing the two CRDs' column sets — mirroring is never actually asserted | `test/schema/printer_columns_test.go:56-62` | Qwen | 1 | 85 |

## Disagreements
- #1 severity is a promotion artifact: DeepSeek rated WARNING and explicitly capped it ("judgement call, hence not CRITICAL"); Qwen rated NOTE and dispositioned it as accepted/out-of-scope (evidence row 17). No leg rated it CRITICAL and both legs' own verdicts were CONCERNS.
- #2 emphasis: DeepSeek says the zero case is never directly exercised; Qwen says the skipsWrite fixture seeds measured zero and the pair covers the nil-as-unchanged regression — compatible in detail (derived nil→0 vs seeded 0, different tests), opposite conclusion.

## Nobody could check
- Nothing was executed (both legs, read-only): build/vet, `task lint`, `task verify`, tests (incl. `-race`), gitleaks, glossary regen — all gate claims rest on `evidence/T07.md` only.
- Live-API behaviour of non-empty `additionalPrinterColumns` suppressing the default Age column (evidence flags it as believed, not verified).
- `hack/verify.sh` end-to-end (whether exit-201 is solely the GTE-1 schema red) and the chart-CRD byte-for-byte diff.
- `hack/gen-glossary.py` internals (spec-fields-only renderer claim, row 11).
- `zz_generated.deepcopy.go` completeness for the cluster status beyond the two CollectedCount hunks (T02 commit).
