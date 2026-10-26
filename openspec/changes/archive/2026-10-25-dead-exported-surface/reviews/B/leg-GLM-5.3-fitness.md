## Verdict: CLEAN
Fitness functions all hold and none were widened. Independently verified: `.golangci.yaml`, `.go-arch-lint.yml`, `codecov.yml`, `sonar-project.properties`, `Taskfile.yml` (`COVERAGE_MIN=90`) and CI configs are untouched in `3ee21266..HEAD` (empty config diff); no `//nolint`, no tag-dodging, no new production exported symbol (only exported test funcs in `_test.go`). Tip coverage re-derived from the artifact itself: `go tool cover -func=coverage.out` → **91.3%** (production `.go` identical `f5736ec7`↔HEAD — T9 touched 2 test files only; T8's recorded run matches). Live-path coverage kept: `exportThroughBreaker` 100.0%, `RunExportEnvelope` 89.4%; 3 production dispatch call sites intact. All 11 deleted symbols grep to zero live references at HEAD. The one gate the branch tripped — govet `shadow` ×2 from its own test edits — was caught by the repo's own lint fitness function, classified, and fixed without weakening it (`2906c7eb`: rename + `:=`→`=`, diff verified). Characteristics moved: exported surface down (809→797 grep-counted top-level symbols), dead complexity down (~800 lines), layering/coupling unchanged (arch-lint OK, no new cross-component imports), coverage and determinism held exactly.

## Findings
- [WARNING] No fitness function measures exported-surface size — the exact characteristic this change improves is unguarded going forward — `.golangci.yaml:32` (`unused` sees only unexported)
  Failure: a future PR re-adds dead exported aliases/runners; `unused`, arch-lint and the 90% floor all stay green, so the Sweep-2 surface regrows silently (this branch: 809 base → 797 HEAD).
  Fix: smallest ratchet from today's measurement — `hack/test/exported_surface_ratchet_test.sh` in the repo's existing regression-guard pattern: grep count of exported top-level decls (`^(func|type|var|const) [A-Z]` + `^func \([^)]*\) [A-Z]` over `api/ internal/`, excluding `_test.go`/`zz_generated*`), fail on count > committed baseline 797, wire into ci.yaml's lint job. Deterministic; undercounts block-declared consts and generics (`func F[T any]`) — same bias both sides, only-increase rule, never false-red.
  Confidence: 90
- [NOTE] Base-side 91.3% parity rests on T8's recorded detached-worktree run, not on an in-repo artifact — `openspec/changes/dead-exported-surface/evidence/T8.md:55`
  Failure: none for this branch (tip side verified independently); if the parity claim is reused later, the base profile cannot be re-derived from the repo (the on-disk `cover.out` is a different-shaped run that includes `api/` blocks).
  Fix: none required for this branch; keep the base `coverage.out` beside the record if the "holds exactly" claim will be cited again.
  Confidence: 25
- [NOTE] golangci-lint runs only at the final task by loop design, so T1/T6 test-file debt accumulated to T8 — self-reported in `evidence/T8.md:215`; CI would have caught it at PR time regardless.
  Failure: none here (caught, classified, fixed); cost is one blocked final gate per accumulated-debt branch.
  Fix: adopt the already-proposed per-task package-scoped `golangci-lint run` when a task rewrites test files (owner-level instruction change).
  Confidence: 85

## Could not check
- Base-tree coverage re-measurement (ran in a detached worktree outside this repo; I ran no tests — read-only review).
- `task lint`/`task test`/`task arch-lint` re-executed at HEAD by me (toolchain runs write caches); verified via recorded artifacts, T9's diff and config-untouched checks instead.
- External Go consumers of the deleted `api/v1alpha1` constants (accepted assumption, `proposal.md:108-113`; the compile cannot prove the external half).
- External source report `data/kollect-xconsol-final/report.md` (outside this repository).
