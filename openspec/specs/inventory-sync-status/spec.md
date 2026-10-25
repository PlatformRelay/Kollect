# inventory-sync-status Specification

## Purpose
How `KollectInventory` and `KollectClusterInventory` report export state: the per-sink
`Synced` condition in `status.sinkExports[]`, the aggregate `Synced` condition, when a sink counts
as up to date (debounced) rather than due for export, and what the read API reports for each sink.

## Requirements

### Requirement: SDC-1 An up-to-date debounced sink reports Synced=True

For `KollectInventory` and `KollectClusterInventory`, when the operator skips a bound sink because
it is up to date (SDC-4 defines when), the sink's entry in `status.sinkExports[]` SHALL carry the
condition `Synced` with status `True` and reason `Debounced`. Its message SHALL name the effective
interval and SHALL NOT contain a value that changes between two reconciles while the sink stays
debounced (no countdown).

#### Scenario: Second reconcile with an unchanged payload

- **GIVEN** an inventory with one sink and an export interval of 15 minutes, which exported successfully
- **WHEN** the inventory reconciles again with the same payload, generation and `requestedAt` value, against the same sink object
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
`True` with reason `Exported` after an export pass in which at least one sink is bound and no
bound sink failed, whether each sink exported in the pass or was debounced (SDC-1). The aggregate
SHALL NOT use reason `PartiallySynced` when no sink failed.

#### Scenario: All sinks debounced

- **GIVEN** an inventory with two sinks that both exported successfully
- **WHEN** a reconcile debounces both
- **THEN** the aggregate condition is `Synced=True`, reason `Exported`, message "2/2 sink(s) up to date"
- **AND** its reason is not `PartiallySynced`

#### Scenario: One sink exported, one debounced

- **GIVEN** an inventory whose sink A has a 1-second interval and sink B a 15-minute interval, both exported successfully
- **WHEN** more than one second later the inventory reconciles with an unchanged payload, so A is re-exported and B is debounced
- **THEN** the aggregate condition is `Synced=True`, reason `Exported`, message "2/2 sink(s) up to date"

### Requirement: SDC-2.2 The aggregate message counts the up-to-date sinks

After an export pass in which no bound sink failed, the aggregate `Synced` message SHALL count
the up-to-date sinks out of the bound sinks and SHALL be the same for an export pass and a
debounced pass over the same set of sinks. The `ExportSucceeded` condition of
`KollectClusterInventory` SHALL use the same count in its message.

#### Scenario: Cluster inventory, all sinks debounced

- **GIVEN** a `KollectClusterInventory` whose sinks all exported successfully
- **WHEN** a reconcile debounces every sink
- **THEN** its aggregate `Synced` is `True`, reason `Exported`
- **AND** its `ExportSucceeded` message counts the debounced sinks as up to date and SHALL NOT say "to 0 sink(s)"

### Requirement: SDC-3 A failed sink reports Synced=False

When any bound sink failed in an export pass, the aggregate `Synced` condition SHALL be `False`
and the failed sink's per-sink condition SHALL be `Synced=False` with its failure reason
(`ExportFailed`, `SpillRequired`), also when every sink failed (the total-failure path, which
today leaves the per-sink entries as they were).

#### Scenario: The only sink fails

- **GIVEN** an inventory whose only sink exported successfully
- **WHEN** the payload changes and the export to that sink fails
- **THEN** the sink's per-sink condition is `Synced=False`, reason `ExportFailed`
- **AND** it SHALL NOT keep `Synced=True` from the earlier export

#### Scenario: Failure, then recovery

- **GIVEN** an inventory whose only sink is up to date (`Synced=True`)
- **WHEN** the payload changes and the export fails, then, at least one second later, the sink recovers and the next export succeeds
- **THEN** the aggregate goes `True` → `False` → `True`, and its `lastTransitionTime` changes at each of those two transitions and at no other pass

### Requirement: SDC-3.2 The aggregate reason names the failure

When a bound sink failed in an export pass, the aggregate reason SHALL be `PartiallySynced` when
another sink exported in the pass, and otherwise the reason the reconciler sets today
(`Progressing`, `ExportTerminal` or `SpillRequired`).

#### Scenario: One sink exported, one failed

- **GIVEN** an inventory with two sinks
- **WHEN** a pass exports one and the other fails
- **THEN** the aggregate is `Synced=False`, reason `PartiallySynced`
- **AND** the failed sink's condition is `Synced=False`, reason `ExportFailed`

### Requirement: SDC-3.3 A failure leaves debounced sinks, Ready and unexported entries as they were

A debounced sink in the same pass as a failed sink SHALL keep `Synced=True`, reason `Debounced`.
`Ready` and `Degraded` SHALL behave as before this change.
Only export passes write per-sink entries: a pass that stops before it exports (scope denial,
unreachable or missing sink, spill gate, registry missing) leaves them as the last export pass
wrote them, and the aggregate `Synced=False` with the blocking reason reports the problem.

#### Scenario: One sink debounced, one failed

- **GIVEN** an inventory whose sink A exported successfully and whose sink B is failing
- **WHEN** a pass debounces A and B fails again
- **THEN** the aggregate is `Synced=False` and SHALL NOT be `True`
- **AND** A's condition is `Synced=True`, reason `Debounced`, and B's is `Synced=False`
- **AND** `Ready` is `False`, as before this change

### Requirement: SDC-4 A sink is debounced only when its export inputs are unchanged

A sink SHALL be debounced (skipped and reported with reason `Debounced`) only when its last
export succeeded and, since that export, none of these changed: the payload checksum, the
inventory's UID and generation, the `kollect.dev/requestedAt` value, and the UID and
`metadata.generation` of the family sink object the binding resolves to; and the effective
export interval has not elapsed, or is `0s`.

#### Scenario: Sink spec changed

- **GIVEN** a database sink that exported successfully, with a 15-minute interval
- **WHEN** the sink's spec is changed (for example its table), which bumps the sink's `metadata.generation` but not the inventory's, and the inventory reconciles with an unchanged payload
- **THEN** the sink is exported in that reconcile, not debounced

#### Scenario: Inventory replaced

- **GIVEN** an inventory that exported successfully, with a 15-minute interval
- **WHEN** it is deleted (its cleanup runs) and recreated with the same name and spec, and the new object reconciles with an unchanged payload
- **THEN** the sink is exported in that reconcile, not debounced

#### Scenario: Sink object replaced

- **GIVEN** a sink that exported successfully, with a 15-minute interval
- **WHEN** the sink object is deleted and recreated with the same name and spec, and the inventory reconciles with an unchanged payload
- **THEN** the sink is exported in that reconcile, not debounced

### Requirement: SDC-4.2 Every other case exports in the same reconcile

In every case SDC-4 does not debounce, the sink SHALL be exported in the same reconcile and SHALL
NOT be reported with reason `Debounced`. Changes outside the inputs SDC-4 lists (a referenced
Secret that points the sink elsewhere, data removed on the backend) are not detected: for an
interval > 0 the periodic re-export bounds them, for `0s` nothing does, as the
`sink-secret-rotation` capability already states for Secret changes.

#### Scenario: Changed payload inside the interval

- **GIVEN** a sink with a 15-minute interval that exported checksum C1 one second ago
- **WHEN** the payload changes to C2 and the inventory reconciles
- **THEN** the sink is exported in that reconcile
- **AND** its condition SHALL NOT have reason `Debounced`

### Requirement: SDC-4.3 A failed or unresolved sink is not debounced

A failed export SHALL clear the sink's debounce state, so the sink is not debounced until an
export to it succeeds again. A binding whose sink cannot be resolved SHALL NOT be counted as
debounced on any path, including the namespaced preview.

#### Scenario: Payload returns to the last exported checksum after a failed export

- **GIVEN** a sink that exported checksum C1 successfully, with a 15-minute interval
- **WHEN** the payload changes to C2 and that export fails, and the payload then returns to C1 and the inventory reconciles with the sink healthy
- **THEN** the sink is exported in that reconcile
- **AND** it SHALL NOT be reported with reason `Debounced` before that export succeeds

### Requirement: SDC-5 No condition churn in the steady state

While the inputs that SDC-4 lists and the sinks' health do not change, a debounced reconcile that
follows another debounced reconcile SHALL NOT change the inventory's status (its
`resourceVersion` stays the same) and SHALL NOT emit an Event, for both inventory kinds, for
inventories below the export-shard warning threshold, on the preview path and on the full export
path (snapshot sinks always take the full path).

#### Scenario: Repeated debounced reconciles

- **GIVEN** an inventory whose sinks are all debounced and whose status was written by the previous, debounced, reconcile
- **WHEN** it reconciles again, at least one second later, with nothing changed
- **THEN** its `resourceVersion` is unchanged and no Event was recorded

### Requirement: SDC-5.2 The aggregate is identical across a re-export of an unchanged payload

The aggregate `Synced` condition (status, reason, message and `lastTransitionTime`) SHALL be
identical before and after a pass that re-exports an unchanged payload. The `lastTransitionTime`
of the aggregate and per-sink `Synced` conditions SHALL change only when that condition's status
changes.

#### Scenario: Export pass and debounce pass look the same in the aggregate

- **GIVEN** an inventory with one sink, up to date
- **WHEN** a pass re-exports the unchanged payload and, at least one second later, the next pass debounces it
- **THEN** the aggregate `Synced` condition after each pass has the same status, reason, message and `lastTransitionTime`
- **AND** the per-sink `Synced` status is `True` after both, with the same `lastTransitionTime`

### Requirement: SDC-5.3 Only a successful export replaces a sink's export record

A sink's `status.sinkExports[]` entry SHALL keep its `lastExportTime` and `lastChecksum` across a
pass that debounces the sink or in which its export fails; only a successful export to that sink
SHALL replace them.

#### Scenario: Per-sink export record survives debounced and failed passes

- **GIVEN** a sink whose last successful export set `lastExportTime` T1 and `lastChecksum` C1
- **WHEN** a later pass debounces the sink, and a pass after that fails to export a changed payload to it
- **THEN** its entry still has `lastExportTime` T1 and `lastChecksum` C1 after each of those passes

### Requirement: SDC-5.4 A lost status write forces a re-export

When the status write that records a successful export fails, the next pass SHALL export that
sink again rather than debounce it, so a debounce never rests on an export that the persisted
entry does not record.

#### Scenario: Status write lost after an export

- **GIVEN** an inventory whose only sink, with interval `0s`, exported payload C2 in a pass
- **WHEN** that pass's status write fails (for example on a conflict) and the next pass sees C2 unchanged
- **THEN** the next pass exports the sink again and SHALL NOT debounce it, for both inventory kinds

### Requirement: SDC-6 The read API reports debounced sinks

The read API's per-sink export status SHALL be taken from the `status.sinkExports[]` entry whose
name is the binding's `<family>/<name>` key, the name the reconcilers write. The inventory-level
status and time SHALL be used only for a binding that has no entry.

#### Scenario: Debounced with status True, controller-shaped entry

- **GIVEN** an inventory with a snapshot sink `git` and a database sink `git`, whose entries `snapshot/git` and `database/git` carry `Synced=True` reason `Debounced` and `Synced=True` reason `Exported`, with different `lastExportTime` values
- **WHEN** the read API builds its export status
- **THEN** the snapshot binding reports `debounced` with its own `lastExportTime`, and the database binding reports `ok` with its own

### Requirement: SDC-6.2 The read API maps each entry's condition and time

The read API SHALL report `debounced` for an entry whose `Synced` condition has reason
`Debounced`, whether its status is `True` or `False`, `ok` for any other `True` condition, and the
entry's `lastExportTime` as the sink's last export time, empty when the entry has none.

#### Scenario: Entry without an export time

- **GIVEN** an inventory whose sink A exported and whose sink B has an entry with no `lastExportTime` (it never exported successfully)
- **WHEN** the read API builds its export status
- **THEN** B's last export time is empty and SHALL NOT be A's time or the inventory's

### Requirement: SDC-7 Documentation matches the conditions

`docs/reference/conditions.md`, `docs/operator-manual/troubleshooting.md`,
`docs/operator-manual/metrics.md`, `docs/crds/kollectinventory.md` and
`docs/crds/kollectclusterinventory.md` SHALL describe a debounced sink as `Synced=True`, reason
`Debounced`, the aggregate as `Synced=True` when no sink failed, and `PartiallySynced` as "some
sinks exported, some failed".

#### Scenario: No doc says a debounce sets Synced=False

- **GIVEN** the documentation after this change
- **WHEN** it is searched, across line breaks, for text that pairs a debounce with `Synced=False` or with `PartiallySynced`
- **THEN** nothing is found outside the upgrade note and the ADR amendment note

### Requirement: SDC-7.2 The ADR and the upgrade guide record the change

ADR-0413 §6 SHALL carry a note that this change amends its aggregate rule. The upgrade guide
SHALL state, in its behaviour changes for the release after v0.21.0 (*Smaller behaviour changes
(after v0.21.0)*), that a debounce no longer sets `Synced=False` or `PartiallySynced`, and list
the new messages.

#### Scenario: ADR note and upgrade note present

- **GIVEN** the documentation after this change
- **WHEN** ADR-0413 §6 and the upgrade guide's *Smaller behaviour changes (after v0.21.0)* section are read
- **THEN** ADR-0413 §6 carries a note that this change amends its aggregate rule
- **AND** the upgrade guide states that a debounce no longer sets `Synced=False` or `PartiallySynced` and lists the new messages
