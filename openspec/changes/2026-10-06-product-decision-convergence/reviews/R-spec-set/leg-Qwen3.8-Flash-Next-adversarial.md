## Verdict: CONCERNS

## Findings
- [WARNING] The change's only traceability anchor — `data/kollect-xconsol-final/report.md` §7 (and pass B) — does not exist in this repository: no `data/` directory, empty `git log --all -- data/`. — `openspec/changes/2026-10-06-product-decision-convergence/proposal.md:5`
  Failure: the question "do the requirements honour the five decisions" is only checkable against the proposal's own paraphrase; a decision as originally worded could be silently restated (D7 already re-frames one as "no defined wiring target") with no falsifiable record.
  Fix: commit the §7 excerpt (or a decision-extract table: decision → requirement id) into the change dir.
  Confidence: 95

- [WARNING] T02 is unsatisfiable as written: a Go test referencing `status.collectedCount` on `KollectClusterTargetStatus` cannot compile until T07 adds the fields (grep: zero hits in `api/v1alpha1/kollectclustertarget_types.go`; namespaced equivalent at `api/v1alpha1/kollecttarget_types.go:96,107`), and a non-compiling test file in `internal/controller` also kills the red runs of T01/T03 in the same package. — `openspec/changes/2026-10-06-product-decision-convergence/tasks.md:18-21`
  Failure: executing tasks in order → `go test ./internal/controller/...` fails to build, not "red on assertions"; D6's red-first contract stalls at T02.
  Fix: split T07 into T07a (API fields + codegen only), run T02 after it.
  Confidence: 85

- [WARNING] BEP-1's unbounded property "no connection to the sink's backend endpoint outlives the deletion beyond the eviction call" is violated by the re-store race: `RunExportEnvelope` → `acquireBackend` (`internal/sink/export.go:170`) → `storePooledBackend` (`internal/sink/backend_pool.go:148`) re-adds a freshly built backend under the UID key *after* the DeleteFunc evicted it; nothing closes it until the 48 h TTL (`backend_pool.go:41` = 2×`validation.MaxExportInterval`, `internal/validation/export_interval.go:22`). — `openspec/changes/2026-10-06-product-decision-convergence/specs/backend-pool/spec.md:16-17`
  Failure: delete the sink while an export is in flight → credentials+connections held for 48 h — the exact defect the decision exists to close; T03's sequential scenario cannot catch it.
  Fix: in the store path, skip-and-close when the sink Get returns NotFound; else scope the scenario's AND-clause to the sequential order and name the race.
  Confidence: 80

- [NOTE] `EvictBackendPool(ns,name)` in the delete hook is likely unreachable in production: the pool keys `ns:` only when UID is empty (`internal/sink/backend_pool.go:79-85`) and every production caller passes the resolved UID (`internal/sink/export.go:138`, `internal/sink/cleanup.go:75`) — D4/T08 add a call for a key form nothing writes. — `design.md:57`
  Fix: drop the ns/name call or state which path writes `ns:` keys. Confidence: 70

- [NOTE] T09 doesn't say whether the exported API constant `GitEngineCLI` / `Engine` field Enum marker (`api/v1alpha1/kollectsink_types.go:130-134,181`) are removed or kept — "internal `GitEngine` type/field" reads as a different symbol. — `tasks.md:48-51`
  Fix: name the api/v1alpha1 symbols in T09. Confidence: 75

- [NOTE] `docs/ANNOTATIONS-LABELS.md:101` promises `requestedAt` on "Reconciled Kollect CRs"; D1 deliberately honours it only on the two inventory kinds (`design.md:21-22`) — T05/T10 only say "docs rows" / "implemented semantics", leaving the table still over-promising for e.g. `KollectTarget`.
  Fix: name the two kinds in T05's docs row. Confidence: 80

- [NOTE] Commit a93f6862 adds a duplicate `**/reviews/**/*.err` to `.gitignore` (line already present above). — `.gitignore:96-98`. Confidence: 95

Checked and true at HEAD: annotations have zero Go reads; `docs/crds/kollectsnapshotsink.md:22` carries the stale "cli required for SSH/KEX" claim; `backend_pool.go:33-35` "no caller" comment; cluster-target lacks the fields the namespaced one has (printer cols `kollecttarget_types.go:115-116`); exactly three `shouldSkip` sites (`kollectinventory_controller.go:356,487`, `kollectclusterinventory_controller.go:305`); `PruneResource` is the single choke point, deep-copying (`engine.go:957`, `prune.go:40`); x/crypto v0.57.0 registers both KEXes (module cache `ssh/kex.go:409,~448`, `ssh/common.go:54,62`), sntrup761 only in testdata; `Dockerfile.pipeline:31-35` ships no git; ADR-0104:114-115 confirms CLI fail-open host keys; PERF-FIX-05 escape hatch present (`kollecttarget_controller.go:330,355`); `setClusterTargetCondition` returns `written`; KEX pin has exactly 8 names; ADR-0803 number free; `task verify` covers chart CRDs (loop.md:21); no allow-list/baseline widened by the commit.

## Could not check
- `data/kollect-xconsol-final/report.md` §7 and pass B — absent from the repo; the five decisions' original wording is unverifiable here (may live in a workbench path out of bounds).
- No commands executed beyond reads (plan mode): `task verify`, `task spec:validate`, `task lint`, `-race` suites not run.
- Helm `mode` value/schema in `charts/kollect` (D7 parked) — values schema entry not opened.
- `KollectClusterTargetReconciler.collectedCount` body (`kollectclustertarget_controller.go:267`) and the degraded-clearing path in depth; envtest tiers and CI behaviour on the eventual PR.
