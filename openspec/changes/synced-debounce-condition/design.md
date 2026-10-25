# Design

## Context

See `proposal.md` *Why*. Three defects produce the symptom; four more become visible, or start
to matter, once "debounced" means "synced":

1. The aggregate maps a debounce to `Synced=False/PartiallySynced`
   (`internal/controller/per_sink_export.go:336-358`).
2. The per-sink condition is `False/Debounced`, and its message carries a countdown.
3. `status.sinkExports[]` is rebuilt from empty entries every export pass, so a debounced pass
   erases `lastExportTime` and `lastChecksum` and recreates the condition with a new
   `lastTransitionTime`.
4. The coalesce baseline ignores the inventory object (UID) and the sink object (UID,
   `metadata.generation`), and is never cleared on failure or on inventory deletion.
5. The preview path treats an unresolvable sink as debounced with the default interval
   (`kollectinventory_controller.go:490`).
6. The total-failure path never writes per-sink entries.
7. The read API looks entries up by the bare sink name and never finds them.

## Option chosen

Decision brief `agent-context/decisions/2026-10-20-synced-debounce-reason.md` (local), run through
two independent legs; both chose A. Pending operator validation.

| Option | Aggregate when no sink failed | Churn | Surface |
| --- | --- | --- | --- |
| **A (chosen)** | `True/Exported`, one message for export and debounce passes | none in the aggregate | none new |
| B | `True/Exported` or `True/UpToDate` | reason alternates each interval | new reason |
| C | unchanged, docs only | flip stays | none |
| D | `True/Exported` plus a new `ExportDebounced` condition | the new condition flips each interval | new condition type |

A loses one bit in the aggregate: whether the last pass exported or skipped. The per-sink reason
and `lastExportTime` keep it.

## Behaviour

### Per-sink condition (SDC-1, SDC-3)

| Sink outcome in the pass | `Synced` | Reason | Message |
| --- | --- | --- | --- |
| exported | `True` | `Exported` | `export completed` (unchanged) |
| debounced, interval > 0 | `True` | `Debounced` | `payload unchanged since the last export; re-exported every <interval>` |
| debounced, interval `0s` | `True` | `Debounced` | `payload unchanged since the last export; exports only on change (interval 0s)` |
| failed | `False` | `ExportFailed` / `SpillRequired` | unchanged |

`<interval>` is the effective interval as `time.Duration.String()`. Nothing time-dependent goes
into the message. One helper, `setSinkExportDebounced(status, generation, interval)`, writes the
debounced row in all three call sites (both reconcilers and `previewAllSinksDebounced`).

### Aggregate (SDC-2, SDC-3)

`aggregateInventorySync(conditions, generation, exported, debounced, failed)` keeps its
signature. Let `upToDate = exported + debounced`, `bound = upToDate + failed`.

| Case | `Synced` | Reason | Message |
| --- | --- | --- | --- |
| `failed > 0 && exported > 0` | `False` | `PartiallySynced` | unchanged (`%d/%d sinks exported; %d failed`) |
| `failed > 0` | `False` | `Progressing` | unchanged (`%d sink(s) failed export`) |
| otherwise | `True` | `Exported` | `%d/%d sink(s) up to date` (`upToDate`, `bound`) |

The total-failure branches (`applyInventoryExportOutcome` and the cluster reconciler's
equivalent at `kollectclusterinventory_controller.go:211-231`), which set `ExportTerminal`,
`SpillRequired` or `Progressing`, keep their aggregate reasons. From G2 on (after the carry-over
below exists, so the entries keep T1/C1) they also store `outcome.SinkExports` before calling `setInventoryDegraded` /
`setDegraded`, so the failed sink's entry reads `False` with its reason and keeps its last
successful `lastExportTime` and `lastChecksum` (SDC-3, SDC-5). `setInventoryDegraded` and
`setDegraded` themselves do not touch `SinkExports`, so the paths that stop before an export pass
(scope, reachability, spill gate) leave the entries as the last export pass wrote them (proposal
non-goals). `Ready` and `Degraded` are unchanged.

The `KollectClusterInventory` `ExportSucceeded` message in the no-failure branch becomes
`exported %d item(s); %d/%d sink(s) up to date`. Its reason and status are unchanged.

### Per-sink entry carry-over (SDC-5)

`carryOverLastExportPaths` becomes `carryOverSinkExport(status, previous, exportKey)`: before the
pass decides anything for the sink, the new entry is seeded from the previous entry with the same
name: `LastExportTime`, `LastChecksum`, `LastExportPaths` and `Conditions` (deep-copied). Then:

- debounced: the `Synced` condition is set as above; `SetStatusCondition` keeps
  `lastTransitionTime` because the status stays `True`;
- exported: `LastExportTime`, `LastChecksum`, `LastExportPaths` are replaced as today;
- failed: only the condition changes (to `False`, which moves `lastTransitionTime`).

Seeding happens in the sequential loop over bindings, before any job goroutine starts; each
goroutine then writes only its own entry under the existing mutex, as today. An entry for a sink
no longer bound is dropped as today (entries are built from the current bindings).
`recordExportPaths` keeps its "empty write keeps the previous paths" rule, so the deletion
cleanup's retraction evidence is unchanged. The previous entry comes from
`inv.Status.SinkExports` of the object being reconciled, which `carryOverLastExportPaths` already
reads.

### When a sink counts as up to date (SDC-4)

The tracker key stays `(inventory, exportKey)`. `sinkCoalesceState` gains `inventoryUID`,
`sinkUID` and `sinkGeneration`; `record` stores them from the reconciled inventory and the
`ResolvedSink` of the export that succeeded, and `shouldSkip` returns false when any differs from
the current pass. The inventory UID covers an inventory deleted (with its backend cleanup) and
recreated under the same name, which starts again at generation 1.
`ResolvedSink` gains `Generation int64`, filled from the family sink object in
`internal/sink/resolver.go:63`, `:78`, `:93`.

`shouldSkip` and `record` take the resolved identity as one small struct argument, so all call
sites change together. The preview path (`previewAllSinksDebounced`) needs the resolved sink for
that argument; when `loadResolvedSink` fails it returns "not all debounced" and lets the full
path report the failure, instead of debouncing with the default interval.

`perSinkCoalesceTracker` gains `forget(invKey, exportKey)`, which deletes the entry. One helper,
`failSinkExport(tracker, outcome, status, invKey, exportKey, generation, err)`, replaces the
`addSinkFailure` + `setSinkExportSynced` pairs in every per-sink failure branch of an export
pass (the job failure, `kollectinventory_controller.go:437-443`, cluster
`kollectclusterinventory_controller.go:359-364`; the resolve failure, `:324-328`; the partition
failure, `:350-354`; and their cluster counterparts) and calls `forget`. A unit test covers the
helper, and the rule "no `outcome.addSinkFailure` call outside `failSinkExport`" is checked by
`rg`, so no branch can skip the reset; the job-failure branch also has an envtest. With no entry, `shouldSkip` returns false, so the next reconcile exports. The
requeue after a failure is unchanged (the reconciler's failure requeue; for a `0s` interval the
`ZeroIntervalWatchdog`, `nextDue` returning the watchdog, not 1 s).

The identity check covers the sink replaced or edited while the inventory reconciler was not
looking; a reachability failure in between (`kollectinventory_controller.go:153-159`) needs no
`forget`, because a recreated sink has a new UID. A pod restart starts with an empty tracker, so
the first pass exports, as today.

The reset changes behaviour only when the inputs return to those of the last successful export
after a failed one (payload back at C1, or a `requestedAt` value set back). Every other failure is
already retried, because its checksum, generation or `requestedAt` value differs from the
baseline.

`record` runs when an export succeeds, before the status write that persists its entry. When that
write fails (a conflict, or any other error), `updateStatus` calls `forgetUnpersistedExports`,
which drops the baseline of every sink exported in the pass (`perSinkExportOutcome.markExported`
lists them). Without it a `0s` binding would debounce, for as long as its payload stays the same,
against an export whose time and checksum the persisted entry never recorded (branch review
round 1). The cost is one repeated export of the same payload per failed status write; a
conflict loop repeats it on each retry (conflict requeue), and event sinks receive the payload
again each time (upgrade guide entry).

### Steady state without writes (SDC-5)

With these changes, a debounced pass that follows a debounced pass, with nothing changed,
produces a status equal to the stored one: aggregate condition unchanged, per-sink entries equal
(seeded from the previous ones, same message, same `lastTransitionTime`), `itemCount`,
`observedGeneration` and `lastExportTime` unchanged, `SinkReachable` and `Ready` messages
unchanged. That holds on the namespaced preview path and on the full path that snapshot sinks
always take (the preview compares their partition checksum with the raw fingerprint, so it never
short-circuits them); G2 tests both. For the cluster inventory the pass also rewrites `namespaceShards`, `targetCount` and
the `ExportSucceeded` and `Ready` messages from the same inputs, so they are equal too; G2 tests
this rather than assuming it. `Status().Update` of an unchanged object is not persisted, so
`resourceVersion` stays and the inventory's own watch does not fire again. No Event is recorded
on a pass with `ExportedCount == 0` below the shard-warning threshold. The reconciler does not
compare before updating; equality is left to the API server, which keeps the change small. Probe
0.1 checks the no-op behaviour in envtest.

The first debounced pass after an export does write: the per-sink reason goes `Exported` →
`Debounced`. The periodic re-export pass writes `lastExportTime` and the reason back, and emits
`ExportSucceeded` (proposal non-goals). That is two writes per interval, down from writes on
most reconciles.

### Read API (SDC-6)

`exportStatusFromInventory` (`internal/inventory/status.go:159-182`) looks up the entry named
`<family>/<name>` for each binding (the read API's bindings carry the family). Mapping order:
reason `Debounced` → `debounced` for either status; then `True` → `ok`, `False` → `degraded`,
other → `unknown`. A matched entry's `lastExportTime` is reported as is, empty when nil. A
binding with no entry keeps the aggregate status and time, as today.

### Future pending state

No state holds back a changed payload (ADR-0413 §2). A later change that adds one (rate
limiting, a schedule) must report it with its own per-sink reason and decide its aggregate status
explicitly; SDC-4's scenario "Changed payload inside the interval" guards that today's code
exports instead.

## Compatibility

| Observer | Before | After |
| --- | --- | --- |
| Aggregate `Synced`, all sinks debounced | `False/PartiallySynced` "N sink(s) debounced" | `True/Exported` "N/N sink(s) up to date" |
| Aggregate `Synced`, some exported, some debounced | `False/PartiallySynced` "x/y sinks debounced" | `True/Exported` |
| Aggregate `Synced`, all exported | `True/Exported` "exported to N sink(s)" | `True/Exported` "N/N sink(s) up to date" (message text changes) |
| Aggregate `Synced`, any failure | `False`, reasons as today | unchanged |
| Per-sink `Synced`, debounced | `False/Debounced`, countdown message | `True/Debounced`, static message |
| Per-sink entry after a total failure | stale `True/Exported` | `False/ExportFailed` (or `SpillRequired`) |
| Per-sink `lastExportTime`, `lastChecksum` after a debounced or failed pass | erased | kept |
| Sink spec edited or sink recreated, payload unchanged | debounced until the interval | exported on the next pass |
| Inventory deleted and recreated under the same name, payload unchanged | debounced until the interval | exported on the first pass |
| Read API per-sink status | aggregate status and time (entries never matched) | per-sink status and time (`ok`, `debounced`, `degraded`; empty time if none) |
| `kollect_export_debounced_total` | counts skips | unchanged |
| `Ready`, `Degraded`, Events | as today | unchanged |

An alert on `Synced=False` or on reason `PartiallySynced` stops firing for a debounce. Anything
that detected debouncing from the aggregate must read the per-sink reason or
`kollect_export_debounced_total`. No CRD schema change: reasons and messages are free strings.

## Risks

- A message text change can break a script matching "exported to N sink(s)". Mitigation: the
  upgrade note lists the new messages.
- Deep-copying the previous conditions carries forward a condition type that a future change
  stops setting. Only `Synced` is set per sink today.
- A sink's status-only write does not bump `metadata.generation` (status subresource), so the
  identity check does not cause extra exports on sink status updates.
