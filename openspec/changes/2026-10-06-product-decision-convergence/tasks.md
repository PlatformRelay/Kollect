# Tasks

Change: product-decision-convergence (review §7 union; design.md D1–D7). Every behavioural task
goes red first through the production path, then green; `-race` runs on the changed packages
(stateful rule); `task verify` must stay green after codegen. Commit per task, Conventional
Commits with `Signed-off-by`.

## 1. Behavioural tests first

- [ ] T01 Write the annotation tests before any production change:
  (a) namespaced + cluster controllers: changing `kollect.dev/requestedAt` re-exports unchanged
  content that would otherwise debounce, and an unchanged value keeps the debounce
  (ERA-1); absence→present and present→absence count as changes; the cluster path behaves like
  the namespaced path. (b) `PruneResource`: Resource-mode copy is stamped with the source
  generation; the stamp survives a profile prune of `metadata.annotations`; no stamp when
  include excludes metadata; Attributes mode byte-identical (ERA-2). Run: red on the assertions
  (not compile errors).
- [ ] T02 Write the cluster-target parity test before the API change: envtest/unit reconcile
  persists `status.collectedCount` + `collectedCountUpdatedAt` on Ready; a changed count is
  persisted even when the condition payload is byte-identical; steady count keeps its
  timestamp; Degraded keeps the last count (TSP-1). Red on the missing fields/behaviour.
- [ ] T03 Write the eviction test before the watch change: pool a backend under a sink UID via
  the production acquire path, fire the family-sink delete handler, and assert the entry is
  gone and `Close` ran; a delete without a pooled entry is a no-op; a spec-update is not an
  eviction (BEP-1, BEP-2). Red because no delete hook exists.
- [ ] T04 Write the engine-convergence tests before the API change: admission validation
  rejects `engine: cli` naming `go-git`; backend config rejects it independently; CRD schema
  enum lists only `go-git`; the go-git KEX offer contains `mlkem768x25519-sha256` and
  `diffie-hellman-group16-sha512` with the existing eight names in their current relative
  order; `file://` routing and `ls-remote` probe tests keep passing (GTE-1, GTE-2). Red on the
  assertions.

## 2. Implementation

- [ ] T05 Implement `requestedAt`: API constant, tracker third axis (`shouldSkip`/`record`),
  both inventory controllers (export + preview paths), docs rows in `ANNOTATIONS-LABELS.md`
  (ERA-1). Makes T01(a) green.
- [ ] T06 Implement `collectedGeneration` stamp in `PruneResource` after prune/scrub; narrow the
  `ANNOTATIONS-LABELS.md` row to the Resource-mode copy (ERA-2). Makes T01(b) green.
- [ ] T07 Implement cluster-target parity: API fields + printer columns, reuse the
  namespaced sync semantics via a shared helper, wire the `countChanged && !written` escape
  hatch on the cluster path, regenerate CRDs (`make generate manifests`), update
  `docs/crds/kollecttarget.md` if it restates namespaced fields and
  `docs/crds/kollectclustertarget.md`, regen glossary (TSP-1). Makes T02 green.
- [ ] T08 Implement evict-on-delete: family-sink controller delete watch per kind, pool comment
  truth-up (no "no caller" claim anymore), keep the TTL comment as backstop (BEP-1, BEP-2).
  Makes T03 green.
- [ ] T09 Implement the git-engine convergence: CRD enum, admission validation, backend config
  rejection, remove engine branch points + internal `GitEngine` type/field, extend the KEX
  pin, regenerate CRDs, update `docs/crds/kollectsnapshotsink.md`, upgrade note, ADR-0803
  (GTE-1..GTE-3). Makes T04 green.
- [ ] T10 Sweep: `ANNOTATIONS-LABELS.md` `requestedAt`/`collectedGeneration` rows state
  implemented semantics; `task spec:validate`; full local gate subset (`task lint`,
  focused `-race` on changed packages, `task verify`); fill the Verification table below with
  executed evidence; record the independent review row.

## 3. Verification

Rev: implementation commits on `fm/kollect-product-decisions-impl`, based on `3ee21266`.
Local: Go 1.26.6 (darwin/arm64), no Docker in this environment (integration rows below).

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| ERA-1 | T01(a) controller tests | changed requestedAt re-exports; unchanged debounces; both paths | not-run | planned |
| ERA-2 | T01(b) PruneResource tests | stamped copy; stamp survives annotation prune; no stamp without metadata; Attributes byte-identical | not-run | planned |
| TSP-1 | T02 controller test | count + timestamp persisted; escape hatch; steady timestamp; Degraded keeps last | not-run | planned |
| BEP-1 | T03 pool + delete-handler tests | entry gone + Close ran; no-op when absent; spec-update not evicting | not-run | planned |
| BEP-2 | T03 + existing pool tests | TTL pruning unchanged | not-run | planned |
| GTE-1 | T04 validation/config/schema tests | cli rejected with go-git named; go-git/default accepted; file:// unchanged | not-run | planned |
| GTE-2 | T04 KEX list test | mlkem768x25519-sha256 + group16-sha512 present; existing order kept | not-run | planned |
| GTE-3 | docs diff review | CRD ref names only go-git; no SSH/KEX cli claim; ADR-0803 linked | not-run | planned |
| — | `task spec:validate` | strict pass | not-run | planned |
| — | `task verify` | no generated-artifact drift | not-run | planned |
| — | `task lint` | pass | not-run | planned |
| — | `-race` on changed packages | pass, -count=2 | not-run | planned |
| — | `task test-integration` | executed, 0 skipped | not-run | needs Docker; CI owns on PR |
