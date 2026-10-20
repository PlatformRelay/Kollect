# Spec Delta

## ADDED Requirements

### Requirement: SDC-1 An up-to-date debounced sink reports Synced=True

For `KollectInventory` and `KollectClusterInventory`, when the operator skips a bound sink because
its payload checksum, spec generation and `kollect.dev/requestedAt` value equal those of the
sink's last successful export and its effective export interval has not elapsed (or is `0s`), the
sink's entry in `status.sinkExports[]` SHALL carry the condition `Synced` with status `True` and
reason `Debounced`. Its message SHALL name the effective interval and SHALL NOT contain a value
that changes between two reconciles while the sink stays debounced (no countdown).

#### Scenario: Second reconcile with an unchanged payload

- **GIVEN** an inventory with one sink and an export interval of 15 minutes, which exported successfully
- **WHEN** the inventory reconciles again with the same payload, generation and `requestedAt` value
- **THEN** no export runs for the sink
- **AND** its per-sink condition is `Synced=True`, reason `Debounced`

#### Scenario: Debounced sink is not reported as not synced

- **GIVEN** a sink that is debounced as above
- **WHEN** the per-sink condition is read
- **THEN** it SHALL NOT have status `False`

#### Scenario: Message is stable while debounced

- **GIVEN** a debounced sink whose per-sink condition was written on one reconcile
- **WHEN** the inventory reconciles again at least one second later and the sink is still debounced
- **THEN** the per-sink condition's message is unchanged

#### Scenario: Zero interval binding

- **GIVEN** a sink bound with `exportMinInterval: 0s` that exported successfully
- **WHEN** the inventory reconciles with an unchanged payload
- **THEN** the per-sink condition is `Synced=True`, reason `Debounced`, and its message says the binding exports only on change

### Requirement: SDC-2 The aggregate Synced is True when no sink failed

The aggregate `Synced` condition of `KollectInventory` and `KollectClusterInventory` SHALL be
`True` with reason `Exported` whenever at least one sink is bound and no bound sink failed in the
pass, whether each sink exported in the pass or was debounced (SDC-1). Its message SHALL be the
same for an export pass and a debounced pass over the same set of sinks, and SHALL count the
up-to-date sinks out of the bound sinks. The `ExportSucceeded` condition of
`KollectClusterInventory` SHALL use the same count in its message. The aggregate SHALL NOT use
reason `PartiallySynced` when no sink failed.

#### Scenario: All sinks debounced

- **GIVEN** an inventory with two sinks that both exported successfully
- **WHEN** a reconcile debounces both
- **THEN** the aggregate condition is `Synced=True`, reason `Exported`, message "2/2 sink(s) up to date"
- **AND** its reason is not `PartiallySynced`

#### Scenario: One sink exported, one debounced

- **GIVEN** an inventory whose sink A has a 15-minute interval and sink B a `0s` interval, both exported successfully
- **WHEN** a reconcile exports A (forced by a `requestedAt` change) and debounces B
- **THEN** the aggregate condition is `Synced=True`, reason `Exported`

#### Scenario: Cluster inventory, all sinks debounced

- **GIVEN** a `KollectClusterInventory` whose sinks all exported successfully
- **WHEN** a reconcile debounces every sink
- **THEN** its aggregate `Synced` is `True`, reason `Exported`
- **AND** its `ExportSucceeded` message counts the debounced sinks as up to date and SHALL NOT say "to 0 sink(s)"

### Requirement: SDC-3 A failed sink keeps Synced=False

When any bound sink failed in a pass, the aggregate `Synced` condition SHALL be `False`, and the
failed sink's per-sink condition SHALL be `Synced=False` with its failure reason
(`ExportFailed`, `SpillRequired`), as before this change. The aggregate reason SHALL be
`PartiallySynced` when another sink exported in the pass, and otherwise the reason the reconciler
sets today (`Progressing`, `ExportTerminal` or `SpillRequired`). A debounced sink in the same
pass SHALL keep `Synced=True`, reason `Debounced`. `Ready` and `Degraded` SHALL behave as before
this change.

#### Scenario: One sink exported, one failed

- **GIVEN** an inventory with two sinks
- **WHEN** a pass exports one and the other fails
- **THEN** the aggregate is `Synced=False`, reason `PartiallySynced`
- **AND** the failed sink's condition is `Synced=False`, reason `ExportFailed`

#### Scenario: One sink debounced, one failed

- **GIVEN** an inventory whose sink A exported successfully and whose sink B is failing
- **WHEN** a pass debounces A and B fails again
- **THEN** the aggregate is `Synced=False` and SHALL NOT be `True`
- **AND** A's condition is `Synced=True`, reason `Debounced`, and B's is `Synced=False`
- **AND** `Ready` is `False`, as before this change

#### Scenario: Failure, then recovery

- **GIVEN** an inventory whose only sink is up to date (`Synced=True`)
- **WHEN** the payload changes and the export fails, then the sink recovers and the next export succeeds
- **THEN** the aggregate goes `True` → `False` → `True`, and `lastTransitionTime` changes on each of those two transitions only

### Requirement: SDC-4 A pending change is never reported as debounced

A sink whose payload checksum, spec generation or `kollect.dev/requestedAt` value differs from
its last successful export SHALL be exported in the same reconcile and SHALL NOT be reported with
reason `Debounced`, whatever its interval. A failed export SHALL clear the sink's debounce
baseline, so the sink is not reported as debounced until an export to it succeeds again.

#### Scenario: Changed payload inside the interval

- **GIVEN** a sink with a 15-minute interval that exported checksum C1 one second ago
- **WHEN** the payload changes to C2 and the inventory reconciles
- **THEN** the sink is exported in that reconcile
- **AND** its condition SHALL NOT have reason `Debounced`

#### Scenario: Payload returns to the last exported checksum after a failed export

- **GIVEN** a sink that exported checksum C1 successfully, with a 15-minute interval
- **WHEN** the payload changes to C2 and that export fails, and the payload then returns to C1 and the inventory reconciles with the sink healthy
- **THEN** the sink is exported in that reconcile
- **AND** it SHALL NOT be reported with reason `Debounced` before that export succeeds

### Requirement: SDC-5 No condition churn in the steady state

While the payload, generation, `requestedAt` value and sink health do not change, a reconcile in
which every bound sink is debounced SHALL NOT change the inventory's status (its
`resourceVersion` stays the same after the reconcile) and SHALL NOT emit an Event. The aggregate
`Synced` condition (status, reason, message and `lastTransitionTime`) SHALL be identical before
and after a pass that re-exports an unchanged payload. `lastTransitionTime` of the aggregate and
per-sink `Synced` conditions SHALL change only when their status changes. A sink's
`status.sinkExports[]` entry SHALL keep its `lastExportTime`, `lastChecksum` and the
`lastTransitionTime` of its `Synced` condition across a pass that debounces the sink or in which
its export fails; only a successful export to that sink SHALL replace `lastExportTime` and
`lastChecksum`.

#### Scenario: Repeated debounced reconciles

- **GIVEN** an inventory whose sinks are all debounced and whose status was written by the previous reconcile
- **WHEN** it reconciles again, one second or more later, with nothing changed
- **THEN** its `resourceVersion` is unchanged and no Event was recorded

#### Scenario: Export pass and debounce pass look the same in the aggregate

- **GIVEN** an inventory with one sink, up to date
- **WHEN** a pass re-exports the unchanged payload (forced by a `requestedAt` change) and the next pass debounces it
- **THEN** the aggregate `Synced` condition after each pass has the same status, reason, message and `lastTransitionTime`
- **AND** the per-sink `Synced` status is `True` after both, with an unchanged `lastTransitionTime`

#### Scenario: Per-sink export record survives a debounced pass

- **GIVEN** a sink whose last successful export set `lastExportTime` T1 and `lastChecksum` C1
- **WHEN** a later pass debounces the sink, and a pass after that fails to export a changed payload to it
- **THEN** its entry still has `lastExportTime` T1 and `lastChecksum` C1 after each of those passes
- **AND** the read API still reports T1 as that sink's last export time

### Requirement: SDC-6 The read API still reports debounced sinks

The read API's per-sink status SHALL report `debounced` for a sink whose `Synced` condition has
reason `Debounced`, whether its status is `True` or `False`, and `ok` for any other `True`
condition.

#### Scenario: Debounced with status True

- **GIVEN** an inventory whose per-sink condition is `Synced=True`, reason `Debounced`
- **WHEN** the read API builds its export status
- **THEN** that sink's status is `debounced`, not `ok`

### Requirement: SDC-7 Documentation matches the conditions

`docs/reference/conditions.md`, `docs/operator-manual/troubleshooting.md`,
`docs/crds/kollectinventory.md` and `docs/crds/kollectclusterinventory.md` SHALL describe a
debounced sink as `Synced=True`, reason `Debounced`, the aggregate as `Synced=True` when no sink
failed, and `PartiallySynced` as "some sinks exported, some failed". ADR-0413 §6 SHALL carry a
note that this change amends its aggregate rule. The upgrade guide SHALL state, under its
v0.22.0 behaviour changes, that a debounce no longer sets `Synced=False` or `PartiallySynced`.

#### Scenario: No doc says a debounce sets Synced=False

- **GIVEN** the documentation after this change
- **WHEN** it is searched for rows or sentences that pair a debounce with `Synced=False` or `PartiallySynced`
- **THEN** none is found outside the upgrade note and the ADR amendment note
