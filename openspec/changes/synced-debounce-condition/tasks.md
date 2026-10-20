# Tasks

Red first. In each group the test task comes before the implementation and records its red in
`evidence/G<N>.md`: the named test runs (`=== RUN <name>`, or the Ginkgo spec line) and fails on
the target assertion, not on a compile error or a skip. "Red on base" means red on the revision
the group starts from: `origin/main` for G1, the G1 commit for G2, the G2 commit for G3. A stub
with the signature from `design.md` and a `not implemented` body (a no-op) is in scope so the
tests compile. Each group closes in one commit (test and implementation), so the tree is green at
every commit and *Known red* stays empty unless a group is split. A case marked *control* is
expected green on its base and is recorded as a control, not a red.

Each case below reads: given / when / then · the failure it exposes.

Test rules for every group:

- Unit tests live in `internal/controller` and `internal/inventory` and are table tests where the
  case says "rows".
- Envtest cases are Ginkgo `It`s in `internal/controller`, next to
  `kollectinventory_per_sink_envtest_test.go` and `kollectclusterinventory_export_envtest_test.go`.
  They call `reconciler.Reconcile` directly against the envtest API server with Postgres family
  sinks from `createPostgresSinkFixtures`, a `recordingBackend`, and
  `newPostgresRecordingRegistryWithSelector`; a sink fails while a test-owned `atomic.Bool` is
  set, by returning `&failingBackend{err: kollecterrors.Transient(...)}` (as in the EC-P1-02
  spec). The backend pool is off in this suite (`suite_test.go:31`), so the selector runs on every
  export. Each spec gives the reconciler a `record.NewFakeRecorder(32)` and drains its channel
  before the step whose Events it asserts.
- Intervals are 15 minutes unless the case says `0s`, so a second reconcile is always inside the
  window. A forced export pass is made by setting a new `kollect.dev/requestedAt` annotation value
  and reconciling.
- Run envtest with `go test -v -run TestControllers ./internal/controller/ -ginkgo.focus '<text>'`
  and record the spec lines. A `BeforeSuite` failure is a real failure.
- Each group records `go test -race -count=3 ./internal/controller/ ./internal/inventory/`.
- Mutations are judged by the test command's exit status, with one no-op control that must
  survive; mutant trees live outside the repository.

## 0. Probe

- [ ] 0.1 Verify the load-bearing claims in `proposal.md` and `design.md` and record each as
  verified (command, versions, date) in `evidence/T0.1.md`; a failed one stops the loop as a spec
  finding:
  - on `origin/main`, an envtest run of export → reconcile → reconcile on one sink shows the
    aggregate `Synced` `True/Exported` after the first and `False/PartiallySynced` after the
    second, and the per-sink entry has no `lastExportTime` and a new `lastTransitionTime` after
    the second (a throwaway spec outside the repository, or `-ginkgo.focus` on a spec that is not
    committed);
  - the API server keeps `resourceVersion` on a status update of an unchanged object (envtest);
  - `apimeta.SetStatusCondition` keeps `lastTransitionTime` when the status is unchanged (cite
    file:line in the pinned `k8s.io/apimachinery`);
  - `shouldSkip` never returns true for a changed checksum, generation or `requestedAt` value
    (`internal/controller/per_sink_export.go:59-97`);
  - every reader of the per-sink or aggregate `Synced` reason: `rg -n 'ReasonDebounced|PartiallySynced|ConditionSynced|conditionSynced'`
    over `internal`, `cmd`, `api`, `charts`, `hack`, `test`, listed with what each does with it.

## 1. Debounced is synced (SDC-1, SDC-2, SDC-3, SDC-6)

- [ ] 1.1 Tests (red on `origin/main`):
  - `TestAggregateInventorySync` (changed; a *Test changes* row citing SDC-2), rows
    (exported, debounced, failed): (2,0,0), (0,2,0), (1,1,0) → `True/Exported`, message
    "2/2 sink(s) up to date" in all three; (1,0,1) → `False/PartiallySynced`; (0,1,1) and (0,0,1)
    → `False/Progressing` · exposes the debounce-to-`False` mapping (base: rows 2-3 are
    `False/PartiallySynced`, row 1's message differs).
  - `TestSetSinkExportDebounced` (new helper `setSinkExportDebounced(status, generation,
    interval)`), rows 15m and 0s: given an empty entry / when the helper runs twice with a
    second's sleep between / then `Synced=True`, reason `Debounced`, message contains `15m0s` or
    "only on change", identical after both calls · exposes a countdown or a `False` status (stub:
    no condition set).
  - envtest "reports debounced sinks as synced": given two database sinks that exported / when
    the inventory reconciles again / then each per-sink `Synced` is `True/Debounced`, the
    aggregate is `True/Exported` and its reason is not `PartiallySynced` · exposes the reported
    symptom (base: `False/PartiallySynced`).
  - envtest "reports a debounced cluster inventory as synced": the same for a
    `KollectClusterInventory`, plus `ExportSucceeded` message does not contain "to 0 sink(s)" ·
    exposes the cluster path (base: `False`).
  - envtest "keeps Synced=False when a debounced sink sits next to a failed one": given sinks A
    and B that both exported C1 / when the payload changes to C2 and the inventory reconciles with
    B failing (A exports C2, B fails), then reconciles again unchanged (A is debounced; B differs
    from its baseline C1, is retried and fails again) / then the aggregate is `False`,
    A is `True/Debounced`, B is `False/ExportFailed`, `Ready` is `False` · exposes a fix that
    treats "no exports" as success (base: A is `False`; the aggregate row is a *control*).
  - *control* the EC-P1-02 spec "marks PartiallySynced when one sink fails and one succeeds"
    stays green unchanged.
  - `TestExportStatusFromInventory_perSinkConditionsOverrideInventoryStatus` (changed; a *Test
    changes* row citing SDC-6): add a sink whose condition is `True/Debounced` → `debounced`,
    keep `False/Debounced` → `debounced` · exposes a read API that checks status before reason
    (base: `ok`).
- [ ] 1.2 Implement: `setSinkExportDebounced` used by the three debounced branches
  (`kollectinventory_controller.go:363-371`, `:498-506`; `kollectclusterinventory_controller.go:312-320`);
  `aggregateInventorySync` per design *Aggregate*; the `ExportSucceeded` message of the cluster
  reconciler; the reason-first mapping in `internal/inventory/status.go`.
  — Mutations: M1 the aggregate's no-failure case returns `False` when `debounced > 0`
  (`TestAggregateInventorySync` and the first envtest fail); M2 the read API checks status before
  reason (its test fails); M0 a no-op edit (all survive).

## 2. No churn (SDC-5, SDC-3 transitions)

- [ ] 2.1 Tests (red on the G1 commit):
  - `TestCarryOverSinkExport` (new `carryOverSinkExport(status, previous, exportKey)`): given a
    previous entry with `LastExportTime` T1, `LastChecksum` C1, two paths and a `Synced`
    condition with `lastTransitionTime` L1 / when it seeds an empty entry of the same name / then
    the entry has all four, and changing the new entry's condition leaves the previous one
    unchanged; no previous entry of that name → the entry stays empty · exposes the paths-only
    carry-over and a shallow copy (stub: copies paths only, as today).
  - envtest "keeps the per-sink export record across a debounced pass": given one sink that
    exported (T1, C1) / when the inventory reconciles again / then the entry still has T1, C1, and
    its `Synced` `lastTransitionTime` is unchanged · exposes the rebuild from empty (base:
    `lastExportTime` nil).
  - envtest "keeps the last successful export record across a failed pass": given one sink that
    exported (T1, C1) / when the payload changes and the export fails / then the entry has T1 and
    C1, and its condition is `False/ExportFailed` · exposes the erase on failure (base: nil).
  - envtest "writes no status on a repeated debounced reconcile": given one sink that exported,
    then a debounced reconcile / when the inventory reconciles again after at least one second /
    then `resourceVersion` equals the value after the debounced reconcile and the FakeRecorder
    has no Event · exposes a per-pass `lastTransitionTime` or message change (base: the per-sink
    `lastTransitionTime` moves, so `resourceVersion` changes).
  - envtest "export pass and debounce pass give the same aggregate": given one up-to-date sink /
    when a forced export pass runs and then a debounced pass / then the aggregate `Synced` has the
    same status, reason, message and `lastTransitionTime` after both, and the per-sink status is
    `True` after both with the same `lastTransitionTime` · exposes the steady-state flip (base:
    per-sink `lastTransitionTime` differs).
  - *control* envtest "moves lastTransitionTime only on a status change": given one up-to-date
    sink / when the payload changes and the export fails, then the sink recovers and the next
    pass exports, then a debounced pass / then the aggregate `lastTransitionTime` changes at the
    failure and at the recovery and not at the debounced pass (green on the G1 commit; it guards
    G2's carry-over).
- [ ] 2.2 Implement `carryOverSinkExport` per design *Per-sink entry carry-over*, replacing
  `carryOverLastExportPaths` at its call sites in both reconcilers and the preview path.
  — Mutations: M1 carry over paths only (the three envtest reds fail again); M2 copy
  `Conditions` without a deep copy (`TestCarryOverSinkExport` fails); M0 no-op.

## 3. A failed export clears the baseline (SDC-4)

- [ ] 3.1 Tests (red on the G2 commit):
  - `TestPerSinkCoalesceTracker_forget`: given a recorded export of C1 / when `forget` runs /
    then `shouldSkip` with C1 is false, and `forget` of an unknown key is a no-op · exposes a
    missing reset (stub: no-op).
  - envtest "re-exports after a failed export when the payload returns": given one sink that
    exported C1 / when the payload changes to C2 and the export fails, then the payload returns
    to C1 with the sink healthy and the inventory reconciles / then the backend recorded one
    export of C1 in that reconcile and the per-sink condition is `True/Exported` · exposes a
    stale baseline (base: no export, `True/Debounced`).
  - envtest "reports a resolve failure, then exports on recovery": given one sink that exported
    / when its sink CR is deleted (resolve fails) and recreated with the same spec / then the
    next reconcile exports · exposes a reset only on the job failure branch (base: debounced).
  - *control* envtest "exports a changed payload inside the interval": given one sink that
    exported C1 a moment ago / when the payload changes to C2 / then the reconcile exports and the
    condition is `True/Exported`, never `Debounced` (green on base; guards SDC-4's first clause).
- [ ] 3.2 Implement `perSinkCoalesceTracker.forget` and call it on every per-sink failure branch
  of both reconcilers (design *Baseline reset on failure*).
  — Mutations: M1 drop the call in the job failure branch (the first envtest fails); M2 drop it
  in the resolve failure branch (the second fails); M0 no-op.

## 4. Documentation (SDC-7)

- [ ] 4.1 Update `docs/reference/conditions.md` (`Synced` row; `ExportSucceeded` row),
  `docs/operator-manual/troubleshooting.md` (condition table row for `PartiallySynced`, the
  per-sink table, the two debounce rows, the partial-failure rows),
  `docs/crds/kollectinventory.md` and `docs/crds/kollectclusterinventory.md` (status condition
  tables, per-sink `conditions[]` row), an amendment note on `docs/adr/0413-export-interval-scheduling.md`
  §6, and an entry under *Smaller behaviour changes (after v0.21.0)* in
  `docs/operator-manual/upgrading.md` (aggregate `True` on a debounce, per-sink `True/Debounced`,
  `PartiallySynced` only for partial failure, the new messages, per-sink export record kept).
  Check: `rg -n -i 'debounc' docs | rg 'Synced=False|PartiallySynced'` lists only the upgrade
  note and the ADR note; `task lint:markdown` reports `0 issues`; the `hack/test/docs_*` guards
  pass.

## 5. Land

- [ ] 5.1 Integrator: branch review, PR, green required CI, then `openspec archive
  synced-debounce-condition` as the last commit.

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| SDC-1 | `TestSetSinkExportDebounced` (15m, 0s); envtest "reports debounced sinks as synced" | `True/Debounced`, stable message | not-run | evidence/G1.md |
| SDC-2 | `TestAggregateInventorySync`; envtest namespaced and cluster "reports … as synced" | `True/Exported` "N/N sink(s) up to date"; no `PartiallySynced`; no "to 0 sink(s)" | not-run | evidence/G1.md |
| SDC-3 | envtest "keeps Synced=False when a debounced sink sits next to a failed one"; *control* EC-P1-02 spec | aggregate `False`; failed sink `False`; `Ready=False` | not-run | evidence/G1.md |
| SDC-3 | *control* envtest "moves lastTransitionTime only on a status change" | moves at failure and recovery only | not-run | evidence/G2.md |
| SDC-4 | `TestPerSinkCoalesceTracker_forget`; envtests "re-exports after a failed export when the payload returns", "reports a resolve failure, then exports on recovery" | export in the recovering reconcile | not-run | evidence/G3.md |
| SDC-4 | *control* envtest "exports a changed payload inside the interval" | exported, never `Debounced` | not-run | evidence/G3.md |
| SDC-5 | `TestCarryOverSinkExport`; envtests "keeps the per-sink export record …" (debounced, failed) | T1, C1 and `lastTransitionTime` kept | not-run | evidence/G2.md |
| SDC-5 | envtests "writes no status on a repeated debounced reconcile", "export pass and debounce pass give the same aggregate" | `resourceVersion` unchanged, no Event; identical aggregate | not-run | evidence/G2.md |
| SDC-6 | `TestExportStatusFromInventory_perSinkConditionsOverrideInventoryStatus` | `True/Debounced` → `debounced` | not-run | evidence/G1.md |
| SDC-1–6 | mutations G1 M1-M2, G2 M1-M2, G3 M1-M2, each with M0 | each target test fails; M0 survives | not-run | evidence/G1.md, G2.md, G3.md |
| SDC-1–6 | `go test -race -count=3 ./internal/controller/ ./internal/inventory/` | pass, N=3 | not-run | evidence/G1.md, G2.md, G3.md |
| SDC-7 | `rg` doc check; `task lint:markdown`; `hack/test/docs_*` guards | only the two notes match; `0 issues`; guards pass | not-run | evidence/G4.md |
| all | independent review of the spec set and the branch | recorded verdicts | not-run | `reviews/R-spec-set/`, `reviews/B-branch/` |
