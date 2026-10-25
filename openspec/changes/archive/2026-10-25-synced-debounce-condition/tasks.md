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

- Unit tests live in `internal/controller` and `internal/inventory`, as table tests where the
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
- Envtest cases marked *both kinds* run once for `KollectInventory` and once for
  `KollectClusterInventory` (a `DescribeTable` or two `It`s); the others are namespaced.
- Every assertion reads the object freshly with `k8sClient.Get` after the reconcile.
- `lastTransitionTime` and `lastExportTime` have one-second precision. Any two passes whose
  timestamps a case compares, or whose difference makes the red, are separated by
  `time.Sleep(1100 * time.Millisecond)`. A forced export pass uses a new `kollect.dev/requestedAt`
  value; a pass where one sink re-exports and another is debounced uses a 1 s interval on the
  first sink and a sleep over it.
- Intervals are 15 minutes unless the case says otherwise.
- Run envtest with `go test -v -run TestControllers ./internal/controller/ -ginkgo.focus '<text>'`
  and record the spec lines. A `BeforeSuite` failure is a real failure.
- Each group records `go test -race -count=3 ./internal/controller/ ./internal/inventory/` (G3 adds
  `./internal/sink/`).
- Mutations are judged by the test command's exit status, with one no-op control that must
  survive; mutant trees live outside the repository.

## 0. Probe

- [x] 0.1 Verify the load-bearing claims in `proposal.md` and `design.md` and record each as
  verified (command, versions, date) in `evidence/T0.1.md`; a failed one stops the loop as a spec
  finding:
  - on `origin/main`, an envtest run (a throwaway spec kept outside the repository, run through
    a scratch copy) of export → sleep → reconcile on one sink: the aggregate `Synced` is
    `True/Exported` after the first pass and `False/PartiallySynced` after the second, and the
    per-sink entry has no `lastExportTime` and a newer `lastTransitionTime` after the second;
  - on `origin/main`, a total failure of the only sink leaves its per-sink condition
    `True/Exported`; the read API's per-sink override does not fire for a `<family>/<name>` entry;
  - the API server keeps `resourceVersion` on a status update of an unchanged object (envtest);
  - `apimeta.SetStatusCondition` keeps `lastTransitionTime` when the status is unchanged (cite
    file:line in the pinned `k8s.io/apimachinery`);
  - a family sink spec update bumps `metadata.generation` and a status update does not (envtest);
  - `shouldSkip` never returns true for a changed checksum, generation or `requestedAt` value
    (`internal/controller/per_sink_export.go:59-99`);
  - every reader of the per-sink or aggregate `Synced` reason or message:
    `rg -n 'ReasonDebounced|PartiallySynced|ConditionSynced|conditionSynced|sink\(s\)'` over
    `internal`, `cmd`, `api`, `charts`, `hack`, `test`, listed with what each does with it; any
    reader the proposal's *Impact* misses is a spec finding.

## 1. Debounced is synced (SDC-1, SDC-2, SDC-3, SDC-6)

- [x] 1.1 Tests (red on `origin/main`):
  - `TestAggregateInventorySync` (changed; *Test changes* row citing SDC-2), rows
    (exported, debounced, failed): (2,0,0), (0,2,0), (1,1,0) → `True/Exported`, message
    "2/2 sink(s) up to date" in all three; (1,0,1) → `False/PartiallySynced`; (0,1,1) and (0,0,1)
    → `False/Progressing` · exposes the debounce-to-`False` mapping (base: rows 2-3 are
    `False/PartiallySynced`, row 1's message differs).
  - `requested_at_annotation_test.go:257` and `:402` (changed; *Test changes* rows citing SDC-2):
    the expected message becomes "1/1 sink(s) up to date"; the forced-export assertions stay.
  - `TestSetSinkExportDebounced` (new helper `setSinkExportDebounced(status, generation,
    interval)`), rows 15m and 0s: given an empty entry / when the helper runs, then runs again
    after a 1.1 s sleep / then `Synced=True`, reason `Debounced`, message contains `15m0s` or
    "only on change", identical after both calls · exposes a countdown or a `False` status (stub:
    no condition set).
  - envtest *both kinds* "reports debounced sinks as synced": given two sinks that exported /
    when the inventory reconciles again / then each per-sink `Synced` is `True/Debounced`, the
    aggregate is `True/Exported` "2/2 sink(s) up to date"; for the cluster kind the
    `ExportSucceeded` message does not contain "to 0 sink(s)" · exposes the reported symptom
    (base: `False/PartiallySynced`).
  - envtest "reports one exported and one debounced sink as synced": given sink A with a 1 s
    interval and sink B with 15 minutes, both exported / when, after a 1.1 s sleep, the inventory
    reconciles with an unchanged payload / then the backend recorded one export (A), A is
    `True/Exported`, B is `True/Debounced`, the aggregate is `True/Exported` "2/2 sink(s) up to
    date" · exposes the mixed case (base: `False/PartiallySynced`).
  - envtest "keeps Synced=False when a debounced sink sits next to a failed one": given sinks A
    and B that both exported C1 / when the payload changes to C2 and the inventory reconciles with
    B failing (A exports C2, B fails), then reconciles again unchanged (A is debounced; B differs
    from its baseline C1, is retried and fails again) / then the aggregate is `False`, A is
    `True/Debounced`, B is `False/ExportFailed`, `Ready` is `False` · exposes a fix that treats
    "no exports" as success (base: A is `False`; the aggregate row is a *control*).
  - *control* the EC-P1-02 spec "marks PartiallySynced when one sink fails and one succeeds"
    stays green unchanged.
  - `TestExportStatusFromInventory_perSinkConditionsOverrideInventoryStatus` (changed; *Test
    changes* row citing SDC-6): entry names become `<family>/<name>`; add a snapshot sink `git`
    and a database sink `git` whose entries are `True/Debounced` and `True/Exported` with
    different `lastExportTime` values → `debounced` and `ok`, each with its own time; keep
    `False/Debounced` → `debounced`; the entry with no `lastExportTime` (`:124`) now reports an
    empty time instead of the inventory's · exposes the bare-name lookup, a status-first mapping
    and the time fallback (base: the aggregate status and time for every binding).
- [x] 1.2 Implement: `setSinkExportDebounced` used by the three debounced branches
  (`kollectinventory_controller.go:363-371`, `:498-506`; `kollectclusterinventory_controller.go:312-320`);
  `aggregateInventorySync` per design *Aggregate*; the `ExportSucceeded` message of the cluster
  reconciler; the `<family>/<name>` lookup, reason-first mapping and per-entry time in
  `internal/inventory/status.go`; the
  `ReasonPartiallySynced` comment in `api/v1alpha1/export_interval_types.go:16`.
  — Mutations: M1 the aggregate's no-failure case returns `False` when `debounced > 0`
  (`TestAggregateInventorySync` and the first envtest fail); M2 the read API checks status
  before reason (its test fails); M3 the read API falls back to the inventory time for a
  matched entry (its test fails); M0 a no-op edit (all survive).

## 2. No churn (SDC-5, SDC-3 transitions)

- [x] 2.1 Tests (red on the G1 commit):
  - `TestCarryOverSinkExport` (new `carryOverSinkExport(status, previous, exportKey)`; replaces
    `TestCarryOverLastExportPaths`, a *Test changes* row citing SDC-5): given a
    previous entry with `LastExportTime` T1, `LastChecksum` C1, two paths and a `Synced`
    condition with `lastTransitionTime` L1 / when it seeds an empty entry of the same name / then
    the entry has all four, and changing the new entry's condition leaves the previous one
    unchanged; no previous entry of that name → the entry stays empty · exposes the paths-only
    carry-over and a shallow copy (stub: copies paths only, as today).
  - envtest *both kinds* "keeps the per-sink export record across a debounced pass": given one
    sink that exported (T1, C1) / when, after a 1.1 s sleep, the inventory reconciles again / then
    the entry still has T1 and C1, and its `Synced` `lastTransitionTime` is unchanged · exposes the
    rebuild from empty (base: `lastExportTime` nil).
  - envtest "keeps the last successful export record across a partly failed pass": given sinks A
    and B that exported (B: T1, C1) / when the payload changes and B's export fails while A
    exports / then B's entry has T1 and C1 and its condition is `False/ExportFailed` · exposes the
    erase on a partial failure (base: B's `lastExportTime` nil).
  - envtest *both kinds* "marks the only failing sink as not synced": given one sink that
    exported (T1, C1) / when the payload changes and the export fails / then the per-sink
    condition is `False/ExportFailed`, the entry keeps T1 and C1, and the aggregate `Synced` is
    `False` · exposes the total-failure path that skips the per-sink write (base: per-sink
    `True/Exported`).
  - envtest *both kinds* "writes no status on a repeated debounced reconcile": given one sink
    that exported, then a debounced reconcile / when, after a 1.1 s sleep, the inventory
    reconciles again / then `resourceVersion` equals the value after the first debounced
    reconcile and the FakeRecorder has no Event · exposes a per-pass `lastTransitionTime` or
    message change (base: the per-sink `lastTransitionTime` moves, so `resourceVersion`
    changes). The namespaced kind runs twice: with a database sink (preview path) and with a
    snapshot sink (full path), the latter built as in `secret_rotation_envtest_test.go`
    (`newSecretRecordingRegistry`).
  - envtest "export pass and debounce pass give the same aggregate": given one up-to-date sink /
    when a forced export pass runs and, after a 1.1 s sleep, a debounced pass / then the aggregate
    `Synced` has the same status, reason, message and `lastTransitionTime` after both, and the
    per-sink status is `True` after both with the same `lastTransitionTime` · exposes the
    steady-state flip (base: per-sink `lastTransitionTime` differs).
  - envtest "moves lastTransitionTime only on a status change": given one up-to-date sink / when,
    with 1.1 s sleeps between passes, the payload changes and the export fails, then the sink
    recovers and the next pass exports, then a debounced pass runs / then the aggregate
    `lastTransitionTime` changes at the failure and at the recovery and not at the debounced
    pass, and the per-sink one likewise · *control* for the aggregate (green on the G1 commit);
    red for the per-sink part (base: it moves at the debounced pass).
- [x] 2.2 Implement `carryOverSinkExport` per design *Per-sink entry carry-over*, replacing
  `carryOverLastExportPaths` at its call sites in both reconcilers and the preview path; store
  `outcome.SinkExports` in both total-failure branches (design *Aggregate*).
  — Mutations: M1 carry over paths only (the envtest reds fail again); M2 copy `Conditions`
  without a deep copy (`TestCarryOverSinkExport` fails); M3 drop the per-sink write in the
  total-failure branch ("marks the only failing sink" fails); M0 no-op.

## 3. Debounced only when up to date (SDC-4)

- [x] 3.1 Tests (red on the G2 commit):
  - `TestPerSinkCoalesceTracker_identity`, rows: same identity → skip; other inventory UID → no
    skip; other sink UID → no skip; other sink generation → no skip · exposes a baseline that
    ignores the objects (stub: identity ignored). Existing tests that call `shouldSkip`, `record`
    or `nextDue` get the identity argument only (mechanical; listed in the evidence, one *Test
    changes* row). `TestKollectInventoryReconciler_previewAllSinksDebounced_allDebounced`
    (`kollectinventory_debounce_test.go:205`) also gets the two `KollectDatabaseSink` objects in
    its fake client, assertion unchanged (its own *Test changes* row citing SDC-4; T0.1 finding 8).
    G1 and G2 touch `previewAllSinksDebounced` without changing its resolve-error branch, so the
    test stays green until G3.
  - `TestFailSinkExport`: given a recorded export / when `failSinkExport` runs / then the outcome
    counts the failure, the entry is `False` with the error's reason, and `shouldSkip` is false ·
    exposes a failure helper without the reset (stub: no `forget`). Check: `rg -n
    'outcome\.addSinkFailure' internal/controller/*_controller.go` finds nothing (every branch
    goes through the helper).
  - `TestPerSinkCoalesceTracker_forget`: given a recorded export of C1 / when `forget` runs /
    then `shouldSkip` with C1 is false, and `forget` of an unknown key is a no-op · exposes a
    missing reset (stub: no-op).
  - envtest *both kinds* "re-exports after a failed export when the payload returns": given one
    sink that exported C1 / when the payload changes to C2 and the export fails, then the payload
    returns to C1 with the sink healthy and the inventory reconciles / then the backend recorded
    one export of C1 in that reconcile and the per-sink condition is `True/Exported` · exposes a
    stale baseline (base: no export, `True/Debounced`).
  - envtest "re-exports after a sink spec change": given a database sink that exported / when
    its `postgres.table` is changed and the inventory reconciles with an unchanged payload / then
    the backend recorded one export · exposes a baseline without the sink generation (base:
    debounced).
  - envtest *both kinds* "re-exports to a recreated inventory": given an inventory that exported
    / when it is deleted, its cleanup completes (finalizer removed by the reconciler), and it is
    recreated with the same name and spec / then the new object's first reconcile exports ·
    exposes a baseline without the inventory UID (base: debounced).
  - envtest "re-exports to a recreated sink": given a sink that exported / when the sink object
    is deleted, the inventory reconciles (reachability fails), the sink is recreated with the same
    name and spec, and the inventory reconciles again / then the second reconcile exports ·
    exposes a baseline without the sink UID (base: debounced).
  - `TestKollectInventoryReconciler_previewAllSinksDebounced_unresolvedSink`: given a recorded
    export for a binding whose sink object does not exist / when the preview runs / then it
    returns "not all debounced" · exposes the preview's resolve-error fall-back (base: true).
  - *control* envtest "exports a changed payload inside the interval": given one sink that
    exported C1 / when the payload changes to C2 / then the reconcile exports and the condition is
    `True/Exported`, never `Debounced` (green on base; guards SDC-4's first clause).
  - *control* envtest "exports on the first pass of a new reconciler": given a sink that exported
    through reconciler R1 / when a new reconciler R2 (empty tracker, as after a restart)
    reconciles the unchanged inventory / then it exports (green on base).
- [x] 3.2 Implement per design *When a sink counts as up to date*: `ResolvedSink.Generation`;
  identity in `sinkCoalesceState`, `shouldSkip` and `record`; `forget` on every per-sink failure
  branch of both reconcilers; the preview's resolve-error branch.
  — Mutations: M1 drop the sink UID comparison (recreated-sink envtest fails); M2 drop the sink
  generation comparison (spec-change envtest fails); M3 drop the inventory UID comparison
  (recreated-inventory envtest fails); M4 drop `forget` from `failSinkExport` (`TestFailSinkExport`
  and the returning-payload envtest fail); M5 restore the preview's default-interval fall-back
  (preview test fails); M0 no-op.

## 4. Documentation (SDC-7)

- [x] 4.1 Update `docs/reference/conditions.md` (`Synced` row; `ExportSucceeded` row),
  `docs/operator-manual/troubleshooting.md` (condition table row for `PartiallySynced`, the
  per-sink table and its read-API line, the two debounce rows, the partial-failure rows),
  `docs/operator-manual/metrics.md` (line 87 onward),
  `docs/crds/kollectinventory.md` and `docs/crds/kollectclusterinventory.md` (status condition
  tables, per-sink `conditions[]` row), an amendment note on `docs/adr/0413-export-interval-scheduling.md`
  §6, and an entry under *Smaller behaviour changes (after v0.21.0)* in
  `docs/operator-manual/upgrading.md` (aggregate `True` on a debounce, per-sink `True/Debounced`,
  `PartiallySynced` only for partial failure, the new messages, the per-sink record kept, a sink
  edit or replacement re-exports once, the read API's per-sink status).
  Named edits, each rewritten and quoted before/after in the evidence:
  `docs/crds/kollectinventory.md:147-149` and `:163`, `docs/crds/kollectclusterinventory.md:131-132`,
  `docs/operator-manual/troubleshooting.md:32`, `:116-122`, `:222-223`, `:259-260`,
  `docs/operator-manual/metrics.md:87`, `docs/reference/conditions.md:10-11`,
  `docs/adr/0413-export-interval-scheduling.md:80`.
  Check, per line: `rg -n -i 'debounc' docs | rg 'Synced=False|PartiallySynced'` lists only the
  upgrade note, the ADR note and `docs/REQUIREMENTS.md:89` (FR-EXP-9 says partial failure
  degrades to `PartiallySynced`, which stays true; reviewed and allowed); prose that breaks the
  pair across lines is covered by the named edits. `task lint:markdown` reports `0 issues`; the
  `hack/test/docs_*` guards pass.

## B. Branch review fixes

- [x] B1.1 Tests (red on `df2c9e501`): `TestKollectInventory_StatusWriteFailureAfterExport_ReExports`
  and `TestKollectClusterInventory_StatusWriteFailureAfterExport_ReExports` in
  `internal/controller/per_sink_export_persist_test.go`: an export pass whose status write
  conflicts is followed by a pass that exports again (`0s` interval, unchanged payload); a control
  case whose write succeeds debounces (SDC-5 *Status write lost after an export*).
- [x] B1.2 Implement: `perSinkExportOutcome.markExported` at both record sites and
  `forgetUnpersistedExports` on the failed status write in both `updateStatus` methods (design
  *When a sink counts as up to date*). Spec wording: SDC-3 pre-export carve-out, SDC-7 upgrade
  section name; `docs/REQUIREMENTS.md` FR-EXP-9 no longer pairs a debounce with
  `PartiallySynced`.

## 5. Land

- [x] 5.1 Integrator: branch review, PR, green required CI, then `openspec archive
  synced-debounce-condition` as the last commit; the reviewer re-checks the archive commit (the
  moved change and the living spec) and the record names both revisions; required CI green on
  the final head before merge (`docs/development/spec-workflow.md` *Land*). File the follow-up
  backlog item SHARD-WARN-EVENT-CHURN (proposal non-goals).

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| SDC-1 | `TestSetSinkExportDebounced` (15m, 0s); envtest "reports debounced sinks as synced" (both kinds) | `True/Debounced`, stable message | pass | evidence/G1.md |
| SDC-2 | `TestAggregateInventorySync`; `requested_at_annotation_test.go` message rows; envtests "reports debounced sinks as synced" (both kinds), "reports one exported and one debounced sink as synced" | `True/Exported` "N/N sink(s) up to date"; no `PartiallySynced`; no "to 0 sink(s)" | pass | evidence/G1.md |
| SDC-3 | envtest "keeps Synced=False when a debounced sink sits next to a failed one"; *control* EC-P1-02 spec | aggregate `False`; failed sink `False`; `Ready=False` | pass | evidence/G1.md |
| SDC-3 | envtest "marks the only failing sink as not synced" (both kinds) | per-sink `False/ExportFailed`, T1/C1 kept | pass | evidence/G2.md |
| SDC-3, SDC-5 | envtest "moves lastTransitionTime only on a status change" | moves at failure and recovery only | pass | evidence/G2.md |
| SDC-4 | `TestPerSinkCoalesceTracker_identity`, `_forget`; `TestFailSinkExport` and its `rg` check; `TestKollectInventoryReconciler_previewAllSinksDebounced_unresolvedSink`; envtests "re-exports after a failed export when the payload returns" (both kinds), "re-exports to a recreated inventory" (both kinds), "re-exports after a sink spec change", "re-exports to a recreated sink" | export in the recovering or changed pass; preview not debounced | pass | evidence/G3.md |
| SDC-4 | *controls* envtests "exports a changed payload inside the interval", "exports on the first pass of a new reconciler" | exported, never `Debounced` | pass | evidence/G3.md |
| SDC-5 | `TestCarryOverSinkExport`; envtests "keeps the per-sink export record across a debounced pass" (both kinds), "keeps the last successful export record across a partly failed pass" | T1, C1 and `lastTransitionTime` kept | pass | evidence/G2.md |
| SDC-5 | envtests "writes no status on a repeated debounced reconcile" (both kinds; namespaced with a database and a snapshot sink), "export pass and debounce pass give the same aggregate" | `resourceVersion` unchanged, no Event; identical aggregate | pass | evidence/G2.md |
| SDC-5 | `TestKollectInventory_StatusWriteFailureAfterExport_ReExports`, `TestKollectClusterInventory_StatusWriteFailureAfterExport_ReExports` (each with a control case); mutations B1 M1-M5, M0 | next pass exports after a failed status write; control debounces | pass | reviews/B-branch/round-1/verification.md |
| SDC-6 | `TestExportStatusFromInventory_perSinkConditionsOverrideInventoryStatus` | `<family>/<name>` entries; `True/Debounced` → `debounced` with its own time; nil time → empty | pass | evidence/G1.md |
| SDC-1–6 | mutations G1 M1-M3, G2 M1-M3, G3 M1-M5, each group with M0 | each target test fails; M0 survives | pass | evidence/G1.md, G2.md, G3.md |
| SDC-1–6 | `go test -race -count=3` on the changed packages | pass, N=3 | pass | evidence/G1.md, G2.md, G3.md |
| SDC-7 | named edits; per-line `rg` doc check; `task lint:markdown`; `hack/test/docs_*` guards | edits quoted; only the two notes match (FR-EXP-9 reworded in B1.2); `0 issues`; guards pass | pass | evidence/G4.md |
| all | independent review of the spec set and the branch | recorded verdicts | pass | `reviews/R-spec-set/`, `reviews/B-branch/` |
