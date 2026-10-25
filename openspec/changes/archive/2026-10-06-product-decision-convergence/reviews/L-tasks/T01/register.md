## Unified verdict: CONCERNS  (legs ok: 2/2)
| # | Sev | Claim (one line) | Where | Models | Legs | Conf |
|---|---|---|---|---|---|---|
| 1 | ERROR | ERA-2 "scrub cannot remove the stamp" is pinned by no test: a scrub denylist on `collectedGeneration` redacts the stamp while the whole T01 matrix stays green (stamp-before-scrub order unguarded) | internal/collect/prune_collected_generation_test.go:54-123 | 2 | 2 | 100 |
| 2 | ERROR | ERA-1 "Absence is a value": Synced condition reason/message and `RequeueAfter` after the forced re-export are unasserted — a T05 that mislabels the condition passes T01 | internal/controller/requested_at_annotation_test.go:229-256 | 2 | 2 | 95 |
| 3 | WARNING | `TestPruneResource_noStampWithoutMetadata` is green-by-construction: asserts only metadata-removed/non-nil, so a stamp under any other top-level key passes | internal/collect/prune_collected_generation_test.go:103-119 | 2 | 2 | 100 |

## Disagreements
- Finding 1: DS rated NOTE (abstract gap), QW WARNING with a concrete mechanism (`scrub.go:129-135` suffix-matches `collectedGeneration`); unified ERROR via dual-model promotion.
- Finding 2: DS WARNING conf 80 vs QW NOTE conf 70; unified ERROR/95 via promotion rule.
- Verification depth differs (DS executed tests/vet/lint, QW static-only) but no claim of one leg contradicts the other.

## Nobody could check
- No `-race` run; the `-count=1` race-gate deviation rests on an unopened `hack/coverage.sh` claim.
- CI/CDS gate results for `d67309bf`; the base-flake repro at `4ae93088` was not re-verified.
- `task verify`/codegen drift, `task test-integration`, full `task lint` incl. go-arch-lint.
- T05/T06 future doc edits to `docs/ANNOTATIONS-LABELS.md:100-101` — untestable until those tasks land.
