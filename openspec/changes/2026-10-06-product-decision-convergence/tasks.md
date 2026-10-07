# Tasks

Change: product-decision-convergence (review §7 union; design.md D1–D7). Every behavioural task
goes red first through the production path, then green; `-race` runs on the changed packages
(stateful rule); `task verify` must stay green after codegen. Commit per task, Conventional
Commits with `Signed-off-by`.

## 1. Behavioural tests first

- [x] T01 Write the annotation tests before any production change:
  (a) namespaced + cluster controllers: changing `kollect.dev/requestedAt` re-exports unchanged
  content that would otherwise debounce, and an unchanged value keeps the debounce
  (ERA-1); absence→present and present→absence count as changes; the cluster path behaves like
  the namespaced path; a preview rendered after the change does not report the bindings as
  debounced. Red on the assertions (the annotation is simply not read yet, so the debounce
  fires): no production symbol is referenced that does not exist. (b) `PruneResource`:
  Resource-mode copy is stamped with the source generation; the stamp survives a profile prune
  of `metadata.annotations`; no stamp when include excludes metadata; Attributes mode
  byte-identical (ERA-2). Red on the missing stamp. — closed 2026-10-07, evidence: evidence/T01.md
- [x] T02 Add the `KollectClusterTargetStatus` fields as scaffolding (spec-shaped: pointers,
  json names, printer-column markers — the signature the spec fixes) and write the parity test:
  reconcile persists `status.collectedCount` + `collectedCountUpdatedAt` on Ready; a changed
  count is persisted even when the condition payload is byte-identical; steady count keeps its
  timestamp; Degraded keeps the last count (TSP-1). Red on the persistence behaviour (fields
  stay unset — the controller does not write them yet). — closed 2026-10-07, evidence: evidence/T02.md
- [x] T03 Write the eviction test before the watch change, against a no-op seam stub: define
  the hook the controller will call (spec-shaped signature) with a no-op body; the test pools a
  backend under a sink UID via the production acquire path, fires the delete-hook seam, and
  asserts the entry is gone and `Close` ran; an acquire-build in flight at eviction is
  discarded, not re-pooled (delete-tombstone); a delete without a pooled entry is a no-op; a
  spec-update is not an eviction (BEP-1, BEP-2). Red because the seam is a no-op. — closed
  2026-10-07, evidence: evidence/T03.md
- [x] T04 Write the engine-convergence tests before the API change. Red assertions: admission
  validation rejects `engine: cli` naming `go-git`; backend config rejects it independently;
  the generated CRD schema enum for `spec.git.engine` lists only `go-git`; the go-git KEX offer
  contains `mlkem768x25519-sha256` and `diffie-hellman-group16-sha512` with the existing eight
  names in their current relative order. Green-by-construction regressions (assert unchanged
  behaviour, expected to pass before and after): `file://` routing and `ls-remote` probe tests
  (GTE-1, GTE-2). Note: 19 existing `internal/sink/git` test fixtures construct
  `Config{Engine: GitEngineCLI}`; T09 migrates them rather than T04. — closed 2026-10-07,
  evidence: evidence/T04.md

## 2. Implementation

- [x] T05 Implement `requestedAt`: API constant, tracker third axis (`shouldSkip`/`record`),
  both inventory controllers (export + preview paths), narrow the `ANNOTATIONS-LABELS.md` row
  to the two inventory kinds and state the implemented semantics (ERA-1). Makes T01(a) green.
  — closed 2026-10-07, evidence: evidence/T05.md
- [x] T06 Implement `collectedGeneration` stamp in `PruneResource` after prune/scrub; clarify
  the `ANNOTATIONS-LABELS.md` row to the Resource-mode embedded copy with implemented
  semantics (ERA-2). Makes T01(b) green.
  — closed 2026-10-07, evidence: evidence/T06.md
- [x] T07 Implement cluster-target parity: reuse the namespaced sync semantics via a shared
  helper, wire the count into the ONE existing cluster escape-hatch write
  (`persistFilterStatusIfSkipped` extended to `filterChanged || countChanged`), regenerate CRDs
  (`make generate manifests`), update `docs/crds/kollectclustertarget.md` (and
  `docs/crds/kollecttarget.md` only if it restates fields), regen glossary (TSP-1). Makes T02
  green.
  — closed 2026-10-07, evidence: evidence/T07.md
- [x] T08 Implement evict-on-delete: family-sink controller delete watch per kind calling the
  seam (UID eviction; ns/name fallback only when the event object carries no UID), pool-side
  delete-tombstone discarding in-flight re-stores, pool comment truth-up (the "no caller"
  claim goes away), TTL comment stays as backstop (BEP-1, BEP-2). Makes T03 green.
  — closed 2026-10-07, evidence: evidence/T08.md
- [x] T09 Implement the git-engine convergence: CRD enum marker becomes `go-git` only, remove
  the exported `GitEngineCLI` API constant, admission validation and backend config reject
  `cli` naming `go-git`, remove the engine branch points and the internal `GitEngine` type and
  `Config.Engine` field, migrate the 19 existing `Config{Engine: GitEngineCLI}` test fixtures
  (file:// fixtures route on the URL; cli_env tests target the machinery directly), extend the
  KEX pin, regenerate CRDs, truth up every `engine: cli` doc site (CRD reference, chart README
  template + `task helm-docs` regen, operator manual engine table, coding-standards MUST line,
  security-architecture engine sections, Dockerfile / Dockerfile.pipeline comments, ADR-0415
  sentence), upgrade note in `docs/operator-manual/upgrading.md` naming the persisted-sink
  behaviour (an existing object still carrying `engine: cli` is rejected at construction, not
  retroactively deleted), ADR-0803 (notes 0802 reserved by the pipeline-CLI comments)
  (GTE-1..GTE-3). Makes T04 green.
  — closed 2026-10-07, evidence: evidence/T09.md
- [x] T10 Sweep: `ANNOTATIONS-LABELS.md` rows state implemented semantics; `task spec:validate`;
  full local gate subset (`task lint`, focused `-race` on changed packages, `task verify`);
  fill the Verification table below with executed evidence; record the independent review row.
  — closed 2026-10-07, evidence: evidence/T10.md

## 3. Verification

Rev: implementation commits on `fm/kollect-product-decisions-impl`, based on `3ee21266`.
Local: Go 1.26.6 (darwin/arm64), no Docker in this environment (integration rows below).

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| ERA-1 | T01(a) controller tests | changed requestedAt re-exports; unchanged debounces; both paths; preview not debounced | pass | rev `81bea0b0`: `KUBEBUILDER_ASSETS=<abs> go test -race -count=1 -v -run 'TestKollectInventoryReconciler_requestedAt\|TestKollectClusterInventoryReconciler_requestedAt\|TestKollectInventoryReconciler_preview_requestedAtChangeNotDebounced' ./internal/controller` — 7 named tests green (2026-10-07); red-first record in evidence/T01.md, green at T05 (evidence/T05.md) |
| ERA-2 | T01(b) PruneResource tests | stamped copy; stamp survives annotation prune; no stamp without metadata; Attributes byte-identical | pass | rev `81bea0b0`: `go test -race -count=2 -v -run 'TestPruneResource\|TestProcessDispatch' ./internal/collect` — 8 named tests green ×2 iterations (2026-10-07); red-first record in evidence/T01.md, green at T06 (evidence/T06.md) |
| TSP-1 | T02 controller test | count + timestamp persisted; escape hatch; steady timestamp; Degraded keeps last | pass | rev `81bea0b0`: same controller run — `TestClusterTargetSetReady_persistsCollectedCount`, `persistsCountWhenConditionUnchanged`, `persistsMeasuredZeroWhenConditionUnchanged`, `steadyCountKeepsTimestamp`, `persistsCountAndFilterInOneWrite`, `TestClusterTargetSetDegraded_keepsLastCount` green (2026-10-07); red-first in evidence/T02.md, green at T07 (evidence/T07.md) |
| BEP-1 | T03 pool + delete-hook tests | entry gone + Close ran; in-flight build discarded; no-op when absent; spec-update not evicting | pass | rev `81bea0b0`: `go test -race -count=2 -v -run 'TestEvictBackendPoolForSink\|TestAcquireBackend_tombstone' ./internal/sink` — 8 named tests green ×2 iterations (2026-10-07); red-first in evidence/T03.md, green at T08 (evidence/T08.md) |
| BEP-2 | T03 + existing pool tests | TTL pruning unchanged | pass | same sink run incl. `TestAcquireBackend_prunesStaleEntries` green ×2 (2026-10-07); TTL backstop semantics unchanged by the delete hook |
| GTE-1 | T04 validation/config/schema tests | cli rejected with go-git named; go-git/default accepted; file:// unchanged | pass | rev `81bea0b0` (2026-10-07): `go test -race -count=2 -v -run 'TestValidateGitSpec\|TestKollectSnapshotSinkGitEngineEnum' ./internal/validation ./test/schema` and `go test -race -count=2 -v -run 'TestFileRemote\|TestConfigFromSpec' ./internal/sink/git` — reject/accept + schema enum + 3 `file://` routing tests green; red-first in evidence/T04.md, green at T09 (evidence/T09.md) |
| GTE-2 | T04 KEX list test | mlkem768x25519-sha256 + group16-sha512 present; existing order kept | pass | rev `81bea0b0`: `TestSSHKeyExchangeOffer_carriesModernAlgorithms` + `TestSSHKeyExchangeOffer_existingEightKeepRelativeOrder` green in the sink/git run (2026-10-07) |
| GTE-3 | docs diff review | all engine: cli sites truthed up; no SSH/KEX cli claim; ADR-0803 linked | pass | T09 diff review (2 free legs, 1 HIGH wording finding fixed, evidence/T09.md) + T10 sweep re-checked 2026-10-07: ADR-0803 linked from CRD ref/coding-standards/ADR-0415; upgrading note names persisted-sink behaviour; B whole-branch review still pending |
| — | `task spec:validate` | strict pass | pass | rev `81bea0b0`, exit 0, 3.3s, 14/14 items (2026-10-07) |
| — | `task verify` | no generated-artifact drift | pass | rev `81bea0b0`, exit 0, 10.6s, tree clean after (2026-10-07) |
| — | `task lint` | pass | pass | rev `81bea0b0`, exit 0, 50.0s; golangci `0 issues` + go-arch-lint clean (2026-10-07) |
| — | `-race` on changed packages | pass, -count=2 | pass | rev `81bea0b0` (2026-10-07), no DATA RACE anywhere: `-count=2` on internal/validation, test/schema, api/v1alpha1 (6.3s), internal/sink (69.9s), internal/sink/git (505.3s); `-count=1` where the repo's own race gate (`hack/coverage.sh`) does: internal/controller (Ginkgo rejects `-count>1`, 42.9s), internal/collect (10.6s — `-count=2` blocked by the pre-existing `TestRecordLabeledMetricSeries_CapsCardinalityDeterministically` non-idempotency, reproduced alone at base `3ee21266`; owner task in loop.md) |
| — | `task test-integration` | executed, 0 skipped | not-run | needs Docker; CI owns on PR |

Independent review (per task): T10 — fanout diff review, free-model legs only, verdict and register at
`reviews/L-tasks/T10/` (see evidence/T10.md for legs, rounds and disposition).
